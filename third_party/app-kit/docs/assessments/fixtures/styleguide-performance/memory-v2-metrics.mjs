// Pure, partial metric evaluator. It neither acquires evidence nor accepts a
// complete browser-performance contract. v1 thresholds and receipts are intact.
import {parsePS} from './memory-check.mjs';

const MIB = 1024 * 1024;
const HEAP_LIMIT = 4 * MIB;
const MAX_GAP = 200;
const ORIGIN_PATH = {page: '/theme-lab', iframe: '/theme-lab/preview'};
const DOMAINS = ['usedSize', 'embedderHeapUsedSize', 'backingStorageSize'];
const validByte = x => Number.isSafeInteger(x) && x >= 0;
const validTime = x => typeof x === 'number' && Number.isFinite(x) && x >= 0;
const validID = x => typeof x === 'string' && x.length > 0;
const sameSet = (a, b) => a.length === b.length && new Set(a).size === a.length && new Set(b).size === b.length && a.every(x => b.includes(x));
const emptyDomain = () => ({baseline_bytes: null, final_bytes: null, growth_bytes: null});

export function evaluateMemoryV2Metrics(lane, {browser, server, origin} = {}) {
  const errors = [];
  const check = (ok, label) => { if (!ok) errors.push(label); return Boolean(ok); };
  const bytes = (value, label) => check(validByte(value), label) ? value : null;
  const time = (value, label) => check(validTime(value), label) ? value : null;
  const result = {
    schema: 'appkit-theme-guide-memory-v2-metrics', metrics_valid: false,
    budgets_passed: null, errors, memory_complete: false, full_contract_complete: false,
    attribution: 'not-verified',
    heap: Object.fromEntries(DOMAINS.map(key => [key, emptyDomain()])),
    dom: {baseline: null, final: null},
    browser_cost: {
      sampling_target_ms: 100, max_allowed_gap_ms: MAX_GAP, max_observed_gap_ms: null,
      samples: null,
      total_rss_bytes: {baseline: null, final: null, growth: null, max: null},
      renderer_rss_bytes: {baseline: null, final: null, growth: null, max: null},
      nonrenderer_rss_bytes: {baseline: null, final: null, growth: null, max: null},
      server_rss_bytes: {baseline: null, final: null, growth: null, max: null},
      meaning: 'Summed sampled process RSS, not physical memory or application allocations; no RSS budget or blank subtraction.',
    },
    limitations: [
      'Pure metrics only: no collector, task closure, browser ownership or independent acquisition is certified.',
      'Heap domains are separate and may overlap; never add them together.',
      'DOM values are per session diagnostics, not an aggregate budget.',
      'RSS is sampled process RSS; peaks between samples and physical memory are not established.',
    ],
  };
  if (!lane || typeof lane !== 'object') {
    errors.push('missing lane');
    return result;
  }
  check(lane.contract_id === 'appkit-theme-guide-memory-v2', 'contract_id');
  check(lane.id !== undefined && lane.id !== null, 'lane id');
  check(Array.isArray(lane.errors) && lane.errors.length === 0, 'lane errors');
  for (const [name, owner] of Object.entries({browser, server})) {
    check(owner && validByte(owner.pid) && owner.pid > 0 && validID(owner.start_identity), `${name} owner identity`);
  }
  check(browser?.pid !== server?.pid, 'owner PID collision');
  let allowedOrigin = null;
  try { const u = new URL(origin); if (origin === u.origin && u.protocol === 'http:' && ['127.0.0.1', '[::1]'].includes(u.hostname) && u.port) allowedOrigin = u.origin; }
  catch { /* reported below */ }
  check(allowedOrigin !== null, 'exact origin');

  const samples = Array.isArray(lane.samples) ? lane.samples : [];
  check(samples.length > 0, 'samples');
  const pidStarts = new Map();
  const measured = [];
  let maxGap = 0;
  for (const [i, sample] of samples.entries()) {
    const label = `sample ${i}`;
    const start = time(sample?.started_ms, `${label} start`);
    const end = time(sample?.completed_ms, `${label} completion`);
    if (start !== null && end !== null) {
      check(start <= end, `${label} order`);
      check(end - start <= MAX_GAP, `${label} acquisition gap`);
      maxGap = Math.max(maxGap, end - start);
      if (i > 0 && validTime(samples[i - 1]?.completed_ms)) {
        const gap = end - samples[i - 1].completed_ms;
        check(gap >= 0 && gap <= MAX_GAP, `${label} inter-completion gap`);
        maxGap = Math.max(maxGap, gap);
      }
    }
    check(Array.isArray(sample?.errors) && sample.errors.length === 0, `${label} errors`);
    let parsed = null;
    try { parsed = parsePS(sample?.raw_ps); } catch { errors.push(`${label} unparseable raw_ps`); }
    const rows = Array.isArray(sample?.rows) ? sample.rows : [];
    check(Array.isArray(sample?.rows) && rows.length > 0, `${label} rows`);
    if (parsed) check(JSON.stringify(rows.map(r => r && ({pid:r.pid,ppid:r.ppid,rss_kib:r.rss_kib,start_identity:r.start_identity,command:r.command}))) === JSON.stringify(parsed), `${label} raw_ps/rows mismatch`);
    const rowMap = new Map();
    for (const row of rows) {
      if (!check(row && validByte(row.pid) && row.pid > 0 && validByte(row.ppid) && validByte(row.rss_kib) && validID(row.start_identity) && validID(row.command), `${label} invalid ps row`)) continue;
      check(!rowMap.has(row.pid), `${label} duplicate ps PID`);
      rowMap.set(row.pid, row);
      const prior = pidStarts.get(row.pid);
      check(prior === undefined || prior === row.start_identity, `${label} reused PID`);
      if (prior === undefined) pidStarts.set(row.pid, row.start_identity);
    }
    const info = Array.isArray(sample?.process_info) ? sample.process_info : [];
    check(Array.isArray(sample?.process_info), `${label} process_info`);
    const infoMap = new Map();
    for (const item of info) {
      if (!check(item && validByte(item.id) && item.id > 0 && validID(item.type) && typeof item.cpuTime === 'number' && Number.isFinite(item.cpuTime) && item.cpuTime >= 0, `${label} invalid process_info`)) continue;
      check(!infoMap.has(item.id), `${label} duplicate SystemInfo PID`);
      infoMap.set(item.id, item);
      check(rowMap.has(item.id), `${label} SystemInfo PID missing ps row`);
    }
    check(infoMap.get(browser?.pid)?.type === 'browser', `${label} browser SystemInfo identity`);
    check(!infoMap.has(server?.pid), `${label} server unexpectedly in SystemInfo`);
    check(rowMap.get(browser?.pid)?.start_identity === browser?.start_identity, `${label} browser ps identity`);
    check(rowMap.get(server?.pid)?.start_identity === server?.start_identity, `${label} server ps identity`);
    let renderer = 0, nonrenderer = 0, serverRSS = null, renderers = 0;
    for (const row of rowMap.values()) {
      const rss = row.rss_kib * 1024;
      check(validByte(rss), `${label} RSS overflow`);
      check(!/^\([^)]*\)$/.test(row.command), `${label} process command unavailable`);
      if (row.pid === server?.pid) { serverRSS = rss; continue; }
      const kind = infoMap.get(row.pid)?.type;
      check(validID(kind), `${label} unknown ps process`);
      if (row.pid !== browser?.pid) check(kind !== 'browser', `${label} duplicate browser classification`);
      if (row.pid !== browser?.pid) {
        const seen = new Set([row.pid]); let parent = row.ppid, owned = false;
        while (rowMap.has(parent) && !seen.has(parent)) {
          if (parent === browser?.pid) { owned = true; break; }
          seen.add(parent); parent = rowMap.get(parent).ppid;
        }
        check(owned, `${label} nonowned process`);
      }
      const processTypes = [...row.command.matchAll(/(?:^|\s)--type=([^\s]+)/g)];
      if (row.pid !== browser?.pid) {
        check(processTypes.length === 1, `${label} process type unavailable`);
      }
      const commandRenderer = processTypes.some(match => match[1] === 'renderer');
      check((kind === 'renderer') === commandRenderer, `${label} renderer classification`);
      if (kind === 'renderer') { renderer += rss; renderers++; } else { nonrenderer += rss; }
    }
    check(renderers > 0, `${label} missing renderer`);
    check(validByte(renderer) && validByte(nonrenderer) && validByte(serverRSS) && validByte(renderer + nonrenderer), `${label} RSS totals`);
    measured.push({renderer, nonrenderer, server: serverRSS, total: renderer + nonrenderer});
  }
  result.browser_cost.samples = samples.length;
  result.browser_cost.max_observed_gap_ms = maxGap;

  const interval = (start, end, label) => {
    if (!validTime(start) || !validTime(end) || start > end) { errors.push(`${label} interval`); return; }
    const selected = samples.filter(s => validTime(s?.completed_ms) && s.completed_ms >= start && s.completed_ms <= end);
    if (!selected.length) { errors.push(`${label} sample coverage`); return; }
    const gaps = [selected[0].completed_ms - start, end - selected.at(-1).completed_ms];
    for (let i = 1; i < selected.length; i++) gaps.push(selected[i].completed_ms - selected[i-1].completed_ms);
    for (const gap of gaps) { maxGap = Math.max(maxGap, gap); check(gap >= 0 && gap <= MAX_GAP, `${label} sample gap`); }
    result.browser_cost.max_observed_gap_ms = maxGap;
  };
  time(lane.work_started_ms, 'work start');
  time(lane.work_completed_ms, 'work completion');
  check(validTime(lane.work_started_ms) && validTime(lane.work_completed_ms) && lane.work_started_ms <= lane.work_completed_ms, 'work order');
  const cycles = Array.isArray(lane.cycles) ? lane.cycles : [];
  check(cycles.length === 20, 'exactly 20 cycles');
  let previous = lane.work_started_ms;
  for (const [i, cycle] of cycles.entries()) {
    check(cycle?.index === i + 1, `cycle ${i+1} index`);
    check(validTime(cycle?.started_ms) && validTime(cycle?.completed_ms) && cycle.started_ms >= previous && cycle.completed_ms >= cycle.started_ms && cycle.completed_ms <= lane.work_completed_ms, `cycle ${i+1} non-overlapping order`);
    check(cycle?.ok === true, `cycle ${i+1} failed`);
    previous = cycle?.completed_ms;
  }
  interval(lane.work_started_ms, lane.work_completed_ms, 'work');

  const endpoint = (value, label) => {
    if (!value || typeof value !== 'object') { errors.push(`${label} endpoint`); return null; }
    const start = time(value.started_ms, `${label} start`), end = time(value.completed_ms, `${label} completion`);
    check(start !== null && end !== null && start <= end, `${label} time order`);
    const idle = value.idle ?? {};
    check(validTime(idle.started_ms) && validTime(idle.completed_ms) && start !== null && end !== null && idle.started_ms >= start && idle.completed_ms - idle.started_ms >= 5000 && idle.completed_ms <= end, `${label} five second idle`);
    check(validByte(idle.network_before) && idle.network_before === idle.network_after, `${label} idle network stable`);
    const targets = Array.isArray(value.targets) ? value.targets : [];
    check(targets.length === 2 && targets.filter(t => t?.type === 'page').length === 1 && targets.filter(t => t?.type === 'iframe').length === 1, `${label} guide topology`);
    const targetIDs = targets.map(t => t?.target_id), sessions = targets.map(t => t?.session_id);
    check(targetIDs.every(validID) && new Set(targetIDs).size === targetIDs.length, `${label} unique targets`);
    check(sessions.every(validID) && new Set(sessions).size === sessions.length, `${label} unique sessions`);
    check(Array.isArray(idle.targets_before) && Array.isArray(idle.targets_after) && sameSet(idle.targets_before, targetIDs) && sameSet(idle.targets_after, targetIDs), `${label} idle targets stable`);
    for (const target of targets) {
      check(validID(target?.isolate_id), `${label} target isolate`);
      let validURL = false;
      try { const u = new URL(target?.url); validURL = u.origin === allowedOrigin && u.pathname === ORIGIN_PATH[target?.type] && u.hash === ''; } catch { /* reported below */ }
      check(validURL, `${label} target origin/path/type`);
    }
    const isolates = Array.isArray(value.isolates) ? value.isolates : [];
    const isolateIDs = isolates.map(x => x?.id);
    check(isolates.length > 0 && isolateIDs.every(validID) && new Set(isolateIDs).size === isolateIDs.length, `${label} unique isolates`);
    const assigned = isolates.flatMap(x => Array.isArray(x?.session_ids) ? x.session_ids : []);
    check(sameSet(assigned, sessions), `${label} isolate session coverage`);
    for (const target of targets) check(isolates.some(x => x?.id === target?.isolate_id && Array.isArray(x?.session_ids) && x.session_ids.includes(target?.session_id)), `${label} target/isolate binding`);
    const sums = Object.fromEntries(DOMAINS.map(key => [key, 0]));
    for (const isolate of isolates) {
      check(Array.isArray(isolate?.session_ids) && isolate.session_ids.length > 0 && isolate.session_ids.every(validID) && new Set(isolate.session_ids).size === isolate.session_ids.length, `${label} isolate sessions`);
      for (const key of DOMAINS) {
        const value = bytes(isolate?.raw_heap?.[key], `${label} raw ${key}`);
        if (value !== null) sums[key] += value;
      }
      check(isolate?.raw_heap?.usedSize === isolate?.used_bytes && isolate?.raw_heap?.totalSize === isolate?.total_bytes && validByte(isolate?.total_bytes) && isolate?.total_bytes >= isolate?.used_bytes, `${label} raw heap binding`);
    }
    for (const key of DOMAINS) check(validByte(sums[key]), `${label} ${key} sum overflow`);
    const gc = Array.isArray(value.gc) ? value.gc : [];
    check(gc.length === isolates.length && sameSet(gc.map(x => x?.isolate_id), isolateIDs), `${label} GC each dedup isolate`);
    for (const item of gc) check(isolates.some(x => x?.id === item?.isolate_id && Array.isArray(x?.session_ids) && x.session_ids.includes(item?.session_id)) && validTime(item?.started_ms) && validTime(item?.completed_ms) && start !== null && item.started_ms >= start && item.started_ms <= item.completed_ms && item.completed_ms <= idle.started_ms && item.ok === true, `${label} GC result/order`);
    const dom = Array.isArray(value.dom) ? value.dom : [];
    check(dom.length === sessions.length && sameSet(dom.map(x => x?.session_id), sessions), `${label} DOM sessions`);
    for (const item of dom) for (const key of ['documents','nodes','jsEventListeners']) bytes(item?.[key], `${label} DOM ${key}`);
    check(validByte(value.sample_index) && value.sample_index < samples.length, `${label} sample index`);
    const sample = samples[value.sample_index];
    check(sample && validTime(sample.completed_ms) && sample.completed_ms >= idle.completed_ms && end !== null && sample.completed_ms <= end && end-sample.completed_ms <= MAX_GAP, `${label} endpoint sample timing`);
    return {sums, dom, sample_index:value.sample_index, topology:targets.map(t => `${t?.type}\0${t?.url}`).sort()};
  };
  const baseline = endpoint(lane.baseline, 'baseline');
  const final = endpoint(lane.final, 'final');
  if (baseline && final) {
    check(JSON.stringify(baseline.topology) === JSON.stringify(final.topology), 'baseline/final topology');
    check(lane.baseline.completed_ms <= lane.work_started_ms && lane.work_completed_ms <= lane.final.started_ms, 'endpoint/work order');
    interval(lane.baseline.started_ms, lane.final.completed_ms, 'full acquisition');
    for (const key of DOMAINS) {
      result.heap[key] = {baseline_bytes:baseline.sums[key], final_bytes:final.sums[key], growth_bytes:final.sums[key] - baseline.sums[key]};
    }
    result.dom = {baseline:baseline.dom, final:final.dom};
    for (const [name, field] of Object.entries({total_rss_bytes:'total',renderer_rss_bytes:'renderer',nonrenderer_rss_bytes:'nonrenderer',server_rss_bytes:'server'})) {
      const before = measured[baseline.sample_index]?.[field] ?? null;
      const after = measured[final.sample_index]?.[field] ?? null;
      result.browser_cost[name] = {baseline:before, final:after, growth:before === null || after === null ? null : after-before, max:measured.length ? Math.max(...measured.map(x => x[field] ?? NaN)) : null};
    }
  }
  result.metrics_valid = errors.length === 0;
  if (!result.metrics_valid) {
    for (const key of DOMAINS) result.heap[key] = emptyDomain();
    result.dom = {baseline:null, final:null};
    result.browser_cost.samples = null;
    result.browser_cost.max_observed_gap_ms = null;
    for (const name of ['total_rss_bytes','renderer_rss_bytes','nonrenderer_rss_bytes','server_rss_bytes']) result.browser_cost[name] = {baseline:null,final:null,growth:null,max:null};
  } else {
    result.budgets_passed = DOMAINS.every(key => result.heap[key].growth_bytes <= HEAP_LIMIT);
    if (!result.budgets_passed) errors.push('retained heap domain growth budget');
  }
  return result;
}
