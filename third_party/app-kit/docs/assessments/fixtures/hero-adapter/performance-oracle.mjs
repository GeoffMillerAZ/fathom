const requiredContract = {
  schema: 1,
  id: 'p01-enhanced-local-performance-v1',
  widths: [390, 1440],
  schemes: ['light', 'dark'],
  repetitions: 2,
  phases: ['cold', 'warm'],
  viewportHeight: 1000,
  network: {latencyMs:80, downloadBytesPerSecond:1250000, uploadBytesPerSecond:625000},
  cpuSlowdown: 4,
  budgets: {
    cold: {bodyBytes:524288, requests:7, fcpMs:1000, lcpMs:1800, usableMs:2000, heapBytes:16777216},
    warm: {bodyBytes:16384, requests:7, fcpMs:600, lcpMs:1000, usableMs:1200, heapBytes:16777216},
  },
};

const metricNames = ['bodyBytes', 'requests', 'fcpMs', 'lcpMs', 'usableMs', 'heapBytes'];
const expectedLinks = width => width === 1440
  ? ['/announcement', '/start', '/learn']
  : ['/start', '/learn'];
const same = (left, right) => JSON.stringify(left) === JSON.stringify(right);
const failure = (code, detail) => ({code, detail});
const nonnegative = value => typeof value === 'number' && Number.isFinite(value) && value >= 0;
const positive = value => nonnegative(value) && value > 0;

function contractFailures(contract) {
  const failures = [];
  if (!contract || typeof contract !== 'object' || Array.isArray(contract)) {
    return [failure('contract.type', 'contract must be an object')];
  }
  for (const key of ['schema', 'id', 'widths', 'schemes', 'repetitions', 'phases', 'viewportHeight', 'network', 'cpuSlowdown']) {
    if (!same(contract[key], requiredContract[key])) {
      failures.push(failure(`contract.${key}`, `expected ${JSON.stringify(requiredContract[key])}`));
    }
  }
  for (const phase of requiredContract.phases) {
    const budget = contract.budgets?.[phase];
    if (!budget || typeof budget !== 'object' || Array.isArray(budget)) {
      failures.push(failure(`contract.budgets.${phase}`, 'budget must be an object'));
      continue;
    }
    for (const metric of metricNames) {
      const expected=requiredContract.budgets[phase][metric];
      if (!nonnegative(budget[metric]) || budget[metric] !== expected) {
        failures.push(failure(`contract.budgets.${phase}.${metric}`, `expected ${expected}`));
      }
    }
  }
  return failures;
}

function coordinate(contract, sample, failures) {
  if (!Number.isInteger(sample.repeat) || sample.repeat < 1 || sample.repeat > contract.repetitions) {
    failures.push(failure('sample.repeat', 'repeat must be 1 or 2'));
  }
  if (!contract.widths.includes(sample.width)) failures.push(failure('sample.width', 'unknown width'));
  if (!contract.schemes.includes(sample.scheme)) failures.push(failure('sample.scheme', 'unknown scheme'));
  if (!contract.phases.includes(sample.phase)) failures.push(failure('sample.phase', 'unknown phase'));
}

function arrayCheck(sample, name, failures) {
  if (!Array.isArray(sample[name])) {
    failures.push(failure(`sample.${name}`, `${name} must be an array`));
    return false;
  }
  return true;
}

export function evaluateSample(contract, sample) {
  const failures = contractFailures(contract);
  if (failures.length) return failures;
  if (!sample || typeof sample !== 'object' || Array.isArray(sample)) {
    return [failure('sample.type', 'sample must be an object')];
  }

  coordinate(contract, sample, failures);
  for (const metric of ['bodyBytes', 'requests', 'cacheHits']) {
    if (!nonnegative(sample[metric])) failures.push(failure(`sample.${metric}`, `${metric} must be finite and nonnegative`));
  }
  for (const metric of ['fcpMs', 'lcpMs', 'usableMs', 'heapBytes']) {
    if (!positive(sample[metric])) failures.push(failure(`sample.${metric}`, `${metric} must be finite and greater than zero`));
  }
  if (!Number.isInteger(sample.requests)) failures.push(failure('sample.requests.integer', 'requests must be an integer'));
  if (!Number.isInteger(sample.cacheHits)) failures.push(failure('sample.cacheHits.integer', 'cacheHits must be an integer'));
  if (!Number.isInteger(sample.bodyBytes)) failures.push(failure('sample.bodyBytes.integer', 'bodyBytes must be an integer'));
  if (!Number.isInteger(sample.heapBytes)) failures.push(failure('sample.heapBytes.integer', 'heapBytes must be an integer'));
  if (Number.isInteger(sample.cacheHits) && Number.isInteger(sample.requests) && sample.cacheHits > sample.requests) {
    failures.push(failure('sample.cacheHits.range', 'cacheHits cannot exceed requests'));
  }
  if (sample.navigationType !== 'navigate') failures.push(failure('navigation.type', 'navigationType must equal navigate'));
  if (sample.documentCache !== false) failures.push(failure('navigation.documentCache', 'documentCache must be boolean false'));
  if (sample.serverDocumentRequests !== 1) failures.push(failure('navigation.serverDocumentRequests', 'serverDocumentRequests must equal 1'));

  const budget = contract.budgets[sample.phase];
  if (budget) {
    for (const metric of ['bodyBytes', 'fcpMs', 'lcpMs', 'usableMs', 'heapBytes']) {
      if (typeof sample[metric] === 'number' && Number.isFinite(sample[metric]) && sample[metric] > budget[metric]) {
        failures.push(failure(`budget.${metric}`, `${sample[metric]} exceeds ${sample.phase} budget ${budget[metric]}`));
      }
    }
    if (Number.isInteger(sample.requests) && sample.requests !== budget.requests) {
      failures.push(failure('count.requests', `${sample.requests} does not equal required count ${budget.requests}`));
    }
  }

  if (sample.scripts !== 0) failures.push(failure('count.scripts', 'scripts must equal 0'));
  if (sample.fontReady !== true) failures.push(failure('state.fontReady', 'fontReady must be boolean true'));
  if (sample.imagesDecoded !== 3) failures.push(failure('count.imagesDecoded', 'imagesDecoded must equal 3'));
  if (sample.visibleImages !== 2) failures.push(failure('count.visibleImages', 'visibleImages must equal 2'));
  if (sample.phase === 'warm' && (!Number.isInteger(sample.cacheHits) || sample.cacheHits < 1)) {
    failures.push(failure('state.warmCache', 'warm samples require at least one cache hit'));
  }
  if (sample.phase === 'cold' && sample.cacheHits !== 0) {
    failures.push(failure('state.coldCache', 'cold samples require zero cache hits'));
  }

  if (arrayCheck(sample, 'assetFailures', failures) && sample.assetFailures.length !== 0) {
    failures.push(failure('assets.failed', `asset failures: ${JSON.stringify(sample.assetFailures)}`));
  }
  if (arrayCheck(sample, 'unexpectedRequests', failures) && sample.unexpectedRequests.length !== 0) {
    failures.push(failure('requests.unexpected', `unexpected requests: ${JSON.stringify(sample.unexpectedRequests)}`));
  }
  const links = expectedLinks(sample.width);
  if (!arrayCheck(sample, 'visibleLinks', failures) || !same(sample.visibleLinks, links)) {
    failures.push(failure('links.visible', `expected ${JSON.stringify(links)}`));
  }
  if (!arrayCheck(sample, 'keyboardLinks', failures) || !same(sample.keyboardLinks, links)) {
    failures.push(failure('links.keyboard', `expected ${JSON.stringify(links)}`));
  }
  return failures;
}

const canaryTargets = {
  'missing-font': '/assets/inter.woff2',
  'delayed-css': '/source.css',
  'oversize-image': '/assets/hero.jpg',
};
const canaryPaths = ['/enhanced','/source.css','/hero.css','/assets/logo-light.svg','/assets/logo-dark.svg','/assets/hero.jpg','/assets/inter.woff2'];
const digest = value => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);

export function evaluateCanary(contract, name, sample) {
  const failures=evaluateSample(contract,sample);
  if (!Object.hasOwn(canaryTargets,name)) {
    failures.push(failure('canary.name','unknown canary'));
    return {detected:false,failures};
  }
  if (!sample || typeof sample !== 'object' || Array.isArray(sample)) {
    failures.push(failure('canary.sample','canary sample must be an object'));
    return {detected:false,failures};
  }
  if (sample.repeat!==1 || sample.width!==390 || sample.scheme!=='light' || sample.phase!=='cold') {
    failures.push(failure('canary.coordinate','canary requires coordinate 1/390/light/cold'));
  }
  if (!Array.isArray(sample.requestGraph) || sample.requestGraph.length!==7 || sample.requestGraph.length!==sample.requests) {
    failures.push(failure('canary.requestGraph','requestGraph must contain the exact seven requests'));
  }
  const graph=Array.isArray(sample.requestGraph)?sample.requestGraph:[];
  const paths=new Set();
  for(const entry of graph) {
    const valid=entry && typeof entry==='object' && typeof entry.path==='string' &&
      Number.isInteger(entry.status) && typeof entry.finished==='boolean' && typeof entry.cache==='boolean' &&
      Number.isInteger(entry.bodyBytes) && entry.bodyBytes>=0 && Number.isInteger(entry.expectedBodyBytes) && entry.expectedBodyBytes>=0 &&
      digest(entry.sha256) && digest(entry.expectedSHA256) && positive(entry.durationMs) && nonnegative(entry.serverDelayMs);
    if(!valid) failures.push(failure('canary.requestGraph.entry',`malformed requestGraph entry ${JSON.stringify(entry)}`));
    if(entry && typeof entry.path==='string') {
      if(paths.has(entry.path)) failures.push(failure('canary.requestGraph.duplicate',entry.path));
      paths.add(entry.path);
    }
  }
  if(!same([...paths].sort(),[...canaryPaths].sort())) failures.push(failure('canary.requestGraph.paths','requestGraph paths must equal the seven-resource allowlist'));
  const targetPath=canaryTargets[name],targets=graph.filter(entry=>entry?.path===targetPath),target=targets[0];
  if(targets.length!==1) failures.push(failure('canary.target',`expected one ${targetPath} request`));
  for(const entry of graph) {
    if(entry?.path===targetPath) continue;
    if(entry?.status!==200 || entry?.finished!==true || entry?.error || entry?.cache!==false || entry?.bodyBytes!==entry?.expectedBodyBytes || entry?.sha256!==entry?.expectedSHA256) {
      failures.push(failure('canary.unrelatedRequest',`unrelated request failed: ${entry?.path}`));
    }
  }
  const assetFailures=Array.isArray(sample.assetFailures)?sample.assetFailures:[];
  const targetAssetFailure=assetFailures.some(item=>item && typeof item==='object' && item.path===targetPath);
  if(name==='missing-font') {
    const completed=target?.finished===true&&!target.error;
    const aborted=target?.finished===false&&target.error==='net::ERR_ABORTED'&&target.bodyBytes===0&&target.sha256==='e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855';
    if(!target || target.status!==404 || (!completed&&!aborted) || target.cache!==false || sample.fontReady!==false || !targetAssetFailure) failures.push(failure('canary.missing-font','font must return an uncached 404 (completed or exact empty ERR_ABORTED), fail readiness and produce its asset failure'));
  } else if(name==='delayed-css') {
    if(!target || target.status!==200 || target.finished!==true || target.error || target.cache!==false || target.bodyBytes!==target.expectedBodyBytes || target.sha256!==target.expectedSHA256 || target.durationMs<2500 || target.serverDelayMs!==2500 || !(sample.fcpMs>contract.budgets.cold.fcpMs) || !(sample.usableMs>contract.budgets.cold.usableMs)) {
      failures.push(failure('canary.delayed-css','CSS delay and specific FCP/usable overruns were not observed'));
    }
  } else if(name==='oversize-image') {
    if(!target || target.status!==200 || target.finished!==true || target.error || target.cache!==false || target.bodyBytes!==target.expectedBodyBytes+614400 || target.sha256===target.expectedSHA256 || !targetAssetFailure) {
      failures.push(failure('canary.oversize-image','image must grow by 614400 bytes, change identity and produce its asset failure'));
    }
  }
  if(name!=='delayed-css' && assetFailures.some(item=>!item || typeof item!=='object' || item.path!==targetPath)) failures.push(failure('canary.unrelatedAssetFailure','unrelated asset failure observed'));
  if(name==='delayed-css' && assetFailures.length) failures.push(failure('canary.unrelatedAssetFailure','delayed CSS must not create asset failures'));

  const allowed=new Set(['budget.bodyBytes','budget.fcpMs','budget.lcpMs','budget.usableMs','budget.heapBytes']);
  if(name==='missing-font') {allowed.add('state.fontReady');allowed.add('assets.failed')}
  if(name==='oversize-image') allowed.add('assets.failed');
  const unrelated=failures.filter(item=>!allowed.has(item.code) && !item.code.startsWith('canary.'));
  if(unrelated.length) failures.push(failure('canary.unrelatedSampleFailure',unrelated.map(item=>item.code).join(',')));
  const canaryFailures=failures.filter(item=>item.code.startsWith('canary.'));
  return {detected:canaryFailures.length===0,failures};
}

export function evaluateMatrix(contract, samples) {
  const failures = contractFailures(contract);
  if (failures.length) return failures;
  if (!Array.isArray(samples)) return [failure('matrix.type', 'samples must be an array')];

  const expected = new Set();
  for (let repeat=1; repeat<=contract.repetitions; repeat++) {
    for (const width of contract.widths) for (const scheme of contract.schemes) for (const phase of contract.phases) {
      expected.add(`${repeat}/${width}/${scheme}/${phase}`);
    }
  }
  const seen = new Set();
  samples.forEach((sample, index) => {
    for (const item of evaluateSample(contract, sample)) {
      failures.push(failure(`matrix.sample.${index}.${item.code}`, item.detail));
    }
    if (!sample || typeof sample !== 'object' || Array.isArray(sample)) return;
    const key=`${sample.repeat}/${sample.width}/${sample.scheme}/${sample.phase}`;
    if (!expected.has(key)) failures.push(failure('matrix.coordinate.unknown', `sample ${index}: ${key}`));
    else if (seen.has(key)) failures.push(failure('matrix.coordinate.duplicate', `sample ${index}: ${key}`));
    else seen.add(key);
  });
  for (const key of expected) {
    if (!seen.has(key)) failures.push(failure('matrix.coordinate.missing', key));
  }
  if (samples.length !== expected.size) failures.push(failure('matrix.count', `got ${samples.length}, want ${expected.size}`));
  return failures;
}
