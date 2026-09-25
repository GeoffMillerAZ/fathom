// Opt-in, descriptive first slice of P10 payload/readiness collection.
// It neither starts nor restarts a server, installs anything, changes a consumer,
// writes source bodies, or supplies a performance threshold/verdict.
import crypto from 'node:crypto';
import {execFileSync} from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {performance} from 'node:perf_hooks';
import {verifyHardenedReference} from '../catalyst-hardened/reference.mjs';
import {
  applyBody, applyFinished, applyResponse, assertAllowedURL, exactLoopbackOrigin,
  newResource, networkFailureFacts, sampleDisposition, summarizeResources, warmCacheObservation,
} from './payload-checks.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const SOURCE_ROOT = '/tmp/appkit-catalyst-hardened.DdYTWU';
const SOURCE_PACKET_NAME = 'p10-source-ohWeRq';
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const CHROME_VERSION = '152.0.7977.84';
const PLAYWRIGHT_VERSION = '1.63.0';
const SAMPLES = 30;
const SOURCE_MANIFEST_SHA256 = 'f8de11343773dbb8dbeb53bb713d684fee23fcd36604ddff921b07da57450ee2';
const SOURCE_PACKET_REFERENCE_SHA256 = '94d3ab4c145e33bf29af750918429833d833fff3f9ac8820e1314c7f2c67f53d';
const MATRIX = Object.freeze([{width: 390, theme: 'dark'}, {width: 1440, theme: 'light'}]);
const PROFILES = Object.freeze([
  {id: 'source', route: '/settings', javaScriptEnabled: true, accounting: 'source-full-react-page-with-shell'},
  {id: 'native', route: theme => `/${theme}/settings`, javaScriptEnabled: false, accounting: 'target-go-form-only'},
  {id: 'enhanced', route: theme => `/${theme}/enhanced/settings`, javaScriptEnabled: true, accounting: 'target-go-form-only-plus-local-controller'},
]);
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const check = (condition, message) => { if (!condition) throw Error(message); };

function regularAbsolute(value, label, kind = 'file') {
  check(path.isAbsolute(value), `${label} must be absolute`);
  const stat = fs.lstatSync(value);
  check(!stat.isSymbolicLink() && (kind === 'file' ? stat.isFile() : stat.isDirectory()), `${label} must be an existing non-symlink ${kind}`);
  return value;
}

function readJSON(file, label) {
  const bytes = fs.readFileSync(regularAbsolute(file, label));
  try { return {value: JSON.parse(bytes), sha256: hash(bytes), bytes: bytes.length}; }
  catch { throw Error(`${label} must be valid JSON`); }
}

export function fileIdentity(file, label) {
  const selected = regularAbsolute(file, label);
  const bytes = fs.readFileSync(selected);
  return {path: selected, sha256: hash(bytes), bytes: bytes.length};
}

function packetFile(packet, pin, label) {
  check(pin && typeof pin === 'object' && typeof pin.name === 'string' && /^[A-Za-z0-9_.-]+$/.test(pin.name) &&
    typeof pin.sha256 === 'string' && /^[a-f0-9]{64}$/.test(pin.sha256) && Number.isInteger(pin.bytes) && pin.bytes > 0 && pin.bytes <= 16 * 1024 * 1024, `${label} lacks a bounded file pin`);
  const selected = path.join(packet, pin.name);
  check(path.dirname(selected) === packet, `${label} escapes source packet`);
  const observed = fileIdentity(selected, label);
  check(observed.bytes === pin.bytes && observed.sha256 === pin.sha256, `${label} drift`);
  return {name: pin.name, sha256: observed.sha256, bytes: observed.bytes};
}

function sourceAssetFile(sourceRoot, asset, label) {
  check(asset && typeof asset.url === 'string' && /^\/[^?#]+$/.test(asset.url), `${label} lacks a safe source asset URL`);
  const relative = asset.url.startsWith('/_next/') ? path.join('.next', asset.url.slice('/_next/'.length)) : path.join('public', asset.url.slice(1));
  const selected = path.resolve(sourceRoot, relative);
  check(selected.startsWith(`${sourceRoot}${path.sep}`), `${label} escapes selected source root`);
  const observed = fileIdentity(selected, label);
  check(observed.bytes === asset.file.bytes && observed.sha256 === asset.file.sha256, `${label} source build asset drift`);
  return {url: asset.url, sha256: observed.sha256, bytes: observed.bytes};
}

export function privatePacket(packet, parent, sourceRoot) {
  regularAbsolute(packet, 'SOURCE_PACKET', 'directory');
  check(path.basename(packet) === SOURCE_PACKET_NAME, `SOURCE_PACKET must select ${SOURCE_PACKET_NAME}`);
  check(path.dirname(packet) === parent, 'SOURCE_PACKET must be directly inside PRIVATE_PARENT');
  const reference = readJSON(path.join(packet, 'reference.json'), 'source packet reference.json');
  check(reference.sha256 === SOURCE_PACKET_REFERENCE_SHA256, 'source packet reference drift');
  check(reference.value?.id === 'p10-hardened-rendered-reference-v1' && reference.value.source_manifest_sha256 === SOURCE_MANIFEST_SHA256, 'source packet lineage mismatch');
  const pins = [
    ['plan', reference.value.plan], ['default_html', reference.value.default_html], ['error_html', reference.value.error_html],
    ['countries', reference.value.countries], ['defaults', reference.value.defaults], ['currencies', reference.value.currencies],
    ...(reference.value.assets ?? []).map((asset, index) => [`asset-${index}`, asset?.file]),
    ...(reference.value.licenses ?? []).map((license, index) => [`license-${index}`, license]),
    ...(reference.value.frames ?? []).map((frame, index) => [`frame-${index}`, frame?.screenshot]),
  ];
  check(Array.isArray(reference.value.assets) && reference.value.assets.length > 0 && Array.isArray(reference.value.frames) && reference.value.frames.length === 4, 'source packet assets or frames unavailable');
  const seen = new Set();
  const verified = pins.map(([label, pin]) => {
    const result = packetFile(packet, pin, `source packet ${label}`);
    check(!seen.has(result.name), `duplicate source packet pin: ${result.name}`); seen.add(result.name);
    return result;
  });
  const sourceAssets = reference.value.assets.map((asset, index) => sourceAssetFile(sourceRoot, asset, `source build asset-${index}`));
  const source = verifyHardenedReference(sourceRoot);
  check(source.source_manifest_sha256 === reference.value.source_manifest_sha256, 'selected source root does not match packet source manifest');
  return {path: packet, reference_sha256: reference.sha256, reference_bytes: reference.bytes,
    source_manifest_sha256: source.source_manifest_sha256, pinned_files: verified, source_build_assets: sourceAssets};
}

export function processIdentity(pid, label) {
  check(typeof pid === 'string' && /^[1-9][0-9]*$/.test(pid), `${label}_PID must be a positive decimal PID`);
  let command;
  try { command = execFileSync('/bin/ps', ['-p', pid, '-o', 'command='], {encoding: 'utf8', timeout: 2_000}).trim(); }
  catch { throw Error(`${label}_PID is not a live inspectable process`); }
  check(command, `${label}_PID has no command identity`);
  return {pid: Number(pid), command};
}

function lsofPaths(pid, args, label) {
  let output;
  try { output = execFileSync('/usr/sbin/lsof', ['-nP', '-a', '-p', String(pid), ...args, '-Fn'], {encoding: 'utf8', timeout: 2_000}); }
  catch { throw Error(`${label} cannot be attested with lsof`); }
  return output.split('\n').filter(line => line.startsWith('n')).map(line => line.slice(1));
}

export function attestServer(process, origin, {root, binary, label, sourceCommandSuffix}) {
  const url = new URL(origin);
  const cwd = lsofPaths(process.pid, ['-d', 'cwd'], label)[0];
  check(cwd, `${label} working directory unavailable`);
  if (sourceCommandSuffix) check(process.command.endsWith(sourceCommandSuffix), `${label} command does not identify selected transport/root/packet`);
  else if (root) check([root, fs.realpathSync(root)].some(value => process.command.endsWith(` --serve ${value}`)), `${label} command does not identify selected source root`);
  if (binary) {
    const textMappings = lsofPaths(process.pid, ['-d', 'txt'], label);
    check(textMappings.includes(fs.realpathSync(binary)), `${label} does not map the selected target binary`);
  }
  const listeners = lsofPaths(process.pid, [`-iTCP:${url.port}`, '-sTCP:LISTEN'], label);
  check(listeners.some(value => value === `${url.hostname}:${url.port}`), `${label} does not listen on selected loopback origin`);
  return {...process, cwd, listening_origin: origin};
}

export function approvedPlaywright() {
  const modulePath = process.env.APPKIT_PLAYWRIGHT_MODULE;
  check(modulePath, 'Select APPKIT_PLAYWRIGHT_MODULE; no dependency is installed');
  regularAbsolute(modulePath, 'APPKIT_PLAYWRIGHT_MODULE');
  const packageFile = path.join(path.dirname(modulePath), 'package.json');
  const pkg = readJSON(packageFile, 'Playwright package.json');
  check(pkg.value.name === 'playwright' && pkg.value.version === PLAYWRIGHT_VERSION, `Approved Playwright ${PLAYWRIGHT_VERSION} required`);
  return {module: fileIdentity(modulePath, 'APPKIT_PLAYWRIGHT_MODULE'), package: {path: packageFile, sha256: pkg.sha256, version: pkg.value.version}};
}

export function hardware() {
  return {
    platform: process.platform, release: os.release(), arch: process.arch, node: process.version,
    cpu: os.cpus().map(cpu => ({model: cpu.model, speed_mhz: cpu.speed})), total_memory_bytes: os.totalmem(),
  };
}

export function put(directory, name, value) {
  const bytes = Buffer.from(JSON.stringify(value, null, 2) + '\n');
  fs.writeFileSync(path.join(directory, name), bytes, {flag: 'wx', mode: 0o600});
  return {name, sha256: hash(bytes), bytes: bytes.length};
}

function profileRoute(profile, origin, theme) { return origin + (typeof profile.route === 'function' ? profile.route(theme) : profile.route); }

function sourceReadiness() {
  return `(() => {
    const form=document.querySelector('form'), fieldset=form?.querySelector('fieldset');
    const controls=[...form?.querySelectorAll('input:not([type=hidden]),textarea,select,[role=checkbox],[role=combobox],button[aria-haspopup=listbox]')||[]]
      .filter(node=>!node.disabled && !node.closest('[hidden]') && node.getBoundingClientRect().width>0 && node.getBoundingClientRect().height>0);
    const named=new Set(controls.map(node=>node.name||node.getAttribute('aria-label')||node.getAttribute('role')||node.tagName));
    const expected=['name','bio','email','checkbox','address','city','Region','postal_code','Country','currency'];
    return {complete:!!form && (!fieldset || !fieldset.disabled) && controls.length===10 && named.size===10 && expected.every(name=>named.has(name)),
      form_present:!!form, editable_control_observation:controls.length,
      control_names:controls.map(node=>node.name||node.getAttribute('aria-label')||node.getAttribute('role')||node.tagName),
      javascript_disabled:false, application_script_elements:document.querySelectorAll('script').length,
      enhanced_ready:false};
  })()`;
}

function targetReadiness(profile) {
  return `(() => {
    const form=document.querySelector('form[data-p10-settings]');
    const controls=[...form?.querySelectorAll('input:not([type=hidden]),textarea,select')||[]]
      .filter(node=>node.form===form && !!node.name && !node.disabled);
    const names=controls.map(node=>node.name);
    const expected=['name','bio','email','email_is_public','address','city','region','postal_code','country[code]','currency'];
    const native=controls.length===10 && new Set(names).size===10 && expected.every(name=>names.includes(name));
    const scripts=document.querySelectorAll('script').length;
    const enhanced=form?.getAttribute('data-p10-enhanced')==='true';
    return {complete:!!form && native, form_present:!!form, native_editable_controls:controls.length,
      native_control_names:names, javascript_disabled:${profile === 'native'}, application_script_elements:scripts,
      enhanced_ready:${profile === 'enhanced'} ? enhanced && native : false};
  })()`;
}

async function formReadiness(page, profile) {
  await page.waitForLoadState('domcontentloaded');
  const probe = profile === 'source' ? sourceReadiness() : targetReadiness(profile);
  await page.waitForFunction(`(${probe}).complete`, null, {timeout:10000});
  const beforeFonts = await page.evaluate(() => ({navigation: performance.getEntriesByType('navigation')[0]?.toJSON() ?? null, dom_observed_ms: performance.now()}));
  const font = await page.evaluate(async () => { await document.fonts.ready; return {fonts_status: document.fonts.status, font_observed_ms: performance.now(), navigation: performance.getEntriesByType('navigation')[0]?.toJSON() ?? null}; });
  await page.waitForLoadState('load');
  const readiness = await page.evaluate(profile === 'source' ? sourceReadiness() : targetReadiness(profile));
  return {
    ...readiness,
    navigation_timing: font.navigation ?? beforeFonts.navigation,
    dom_observed_ms: beforeFonts.dom_observed_ms,
    fonts_status: font.fonts_status,
    fonts_observed_ms: font.font_observed_ms,
    interval_kind: 'automation-observed navigation/dom/fonts/form readiness; not paint, FUV, or input latency',
  };
}

export async function installCDPCollection(cdp, {origins, profile, phase, failures, policyFailures = failures}) {
  const resources = new Map(), pending = new Set(), timeline = [], listeners = [];
  const id = crypto.randomUUID();
  let lastActivity=performance.now(), currentPhase=phase, sequence=0;
  // CDP timestamps use the browser's monotonic clock; node_monotonic_ms uses this
  // collector's clock. They are retained side by side, never subtracted.
  const record = (kind, event = {}, extra = {}) => {
    const fact = {
      sequence: ++sequence, kind, request_id: event.requestId ?? null,
      cdp_timestamp: Number.isFinite(event.timestamp) ? event.timestamp : null,
      node_monotonic_ms: performance.now(), phase: currentPhase, ...extra,
    };
    timeline.push(fact);
    return fact;
  };
  const marker = (kind, nextPhase = currentPhase) => {
    currentPhase = nextPhase;
    record(kind, {}, {marker: true});
  };
  const later = task => { pending.add(task); task.catch(error => failures.push(String(error))).finally(() => pending.delete(task)); };
  const waitForPending = async (limitMs, boundary) => {
    const deadline=performance.now()+limitMs;
    while (pending.size) {
      const remaining=Math.max(0, deadline-performance.now());
      if (!remaining) {
        failures.push(`async CDP event join timed out after ${limitMs}ms at ${boundary}; ${pending.size} task(s) still pending`);
        return false;
      }
      let timeout;
      const settled=await Promise.race([
        Promise.allSettled([...pending]).then(() => true),
        new Promise(resolve => { timeout=setTimeout(() => resolve(false), remaining); }),
      ]);
      clearTimeout(timeout);
      if (!settled) {
        failures.push(`async CDP event join timed out after ${limitMs}ms at ${boundary}; ${pending.size} task(s) still pending`);
        return false;
      }
    }
    return true;
  };
  const on = (name, handler) => { listeners.push([name, handler]); cdp.on(name, handler); };
  const lineageString = value => typeof value === 'string' && value.length <= 256 ? value : null;
  const newLineage = (event, requestSequence, requestPhase) => ({
    collection_id: lineageString(id), phase: lineageString(requestPhase),
    frame_id: lineageString(event.frameId), loader_id: lineageString(event.loaderId),
    request_sequence: requestSequence ?? null, response_sequence: null,
    cache_event_sequence: null, finished_sequence: null,
  });
  const cacheFlags = response => ({
    from_service_worker: typeof response.fromServiceWorker === 'boolean' ? response.fromServiceWorker : null,
    from_prefetch_cache: typeof response.fromPrefetchCache === 'boolean' ? response.fromPrefetchCache : null,
    from_disk_cache: typeof response.fromDiskCache === 'boolean' ? response.fromDiskCache : null,
  });
  const errorFact = (value, limit) => typeof value === 'string'
    ? {value: value.slice(0, limit), truncated: value.length > limit} : {value: null, truncated: false};
  const bodyError = error => {
    const hasCode = error !== null && typeof error === 'object' && Object.hasOwn(error, 'code');
    const code = hasCode ? error.code : null;
    return {
      source: 'playwright-cdp', command: 'Network.getResponseBody',
      name: errorFact(error?.name, 64), message: errorFact(error?.message, 512),
      code: !hasCode ? {availability: 'absent', value: null}
        : Number.isFinite(code) && Number.isInteger(code) ? {availability: 'observed', value: code}
          : {availability: 'malformed', value: null},
    };
  };
  const detach = () => {
    marker('cdp-listener-detach-start');
    const remove = typeof cdp.off === 'function' ? cdp.off.bind(cdp) : typeof cdp.removeListener === 'function' ? cdp.removeListener.bind(cdp) : null;
    if (!remove) failures.push('CDP listener detach unavailable; post-close receipt boundary cannot claim a detached event source');
    else for (const [name, handler] of listeners) remove(name, handler);
    marker('cdp-listener-detach-end');
  };
  await cdp.send('Network.enable');
  await cdp.send('Fetch.enable', {patterns: [{urlPattern: '*', requestStage: 'Request'}]});
  on('Fetch.requestPaused', event => later((async () => {
    let url;
    try { url = assertAllowedURL(event.request.url, origins); }
    catch (error) { policyFailures.push(String(error)); await cdp.send('Fetch.failRequest', {requestId: event.requestId, errorReason: 'BlockedByClient'}); return; }
    if (event.resourceType === 'WebSocket' || !['GET', 'HEAD'].includes(event.request.method)) {
      policyFailures.push(`Blocked ${event.resourceType || 'request'} ${url.href}`);
      await cdp.send('Fetch.failRequest', {requestId: event.requestId, errorReason: 'BlockedByClient'}); return;
    }
    await cdp.send('Fetch.continueRequest', {requestId: event.requestId});
  })()));
  on('Network.requestWillBeSent', event => {
    const eventFact = record('Network.requestWillBeSent', event); lastActivity=performance.now();
    later((async () => {
    try {
      if(event.redirectResponse){
        // These fixed fixture routes do not redirect; retain/refuse instead of
        // silently overwriting an earlier response sharing the request ID.
        const prior=resources.get(event.requestId);
        if(prior){prior.failure='redirect outside frozen fixture route';resources.set(`${event.requestId}:redirect:${resources.size}`,prior);}
        failures.push(`Unexpected redirect ${event.redirectResponse.url}`);
      }
      const resource = newResource(event, origins, profile);
      resource.lineage = newLineage(event, eventFact.sequence, eventFact.phase);
      resource.cache_flags = cacheFlags({});
      resources.set(event.requestId, resource);
    }
    catch (error) { failures.push(String(error)); }
    })());
  });
  on('Network.requestServedFromCache', event => {
    const eventFact = record('Network.requestServedFromCache', event); lastActivity=performance.now();
    const resource = resources.get(event.requestId); if (resource) { resource.served_from_cache_event = true; resource.lineage.cache_event_sequence = eventFact.sequence; }
  });
  on('Network.responseReceived', event => {
    const eventFact = record('Network.responseReceived', event); lastActivity=performance.now();
    later((async () => {
    const resource = resources.get(event.requestId);
    if (resource) {
      resource.lineage.response_sequence = eventFact.sequence;
      resource.cache_flags = cacheFlags(event.response);
      applyResponse(resource, event.response, !!resource.served_from_cache_event);
    }
    })());
  });
  on('Network.loadingFinished', event => {
    const eventFact = record('Network.loadingFinished', event); lastActivity=performance.now();
    later((async () => {
    const resource = resources.get(event.requestId);
    if (!resource) return;
    resource.lineage.finished_sequence = eventFact.sequence;
    applyFinished(resource, event);
    if(resource.status===204||resource.method==='HEAD'){
      // These HTTP responses have no message body; zero is a protocol fact,
      // not a fabricated digest for an unavailable nonempty body.
      applyBody(resource, Buffer.alloc(0));resource.body.basis='HTTP bodyless response';return;
    }
    try {
      const body = await cdp.send('Network.getResponseBody', {requestId: event.requestId});
      const bytes = body.base64Encoded ? Buffer.from(body.body, 'base64') : Buffer.from(body.body, 'utf8');
      applyBody(resource, bytes);
    } catch (error) {
      resource.body_failure = String(error);
      resource.body_error = bodyError(error);
      // Preserve the resource and the missing body truth; a body failure fails this sample.
      failures.push(`Response body unavailable for ${resource.url}: ${error}`);
    }
    })());
  });
  on('Network.loadingFailed', event => {
    const details = networkFailureFacts(event);
    record('Network.loadingFailed', event, {failure_details: details}); lastActivity=performance.now();
    const resource = resources.get(event.requestId);
    if (resource) { resource.settled = true; resource.failure = event.errorText || 'unknown'; resource.failure_details = details; }
    failures.push(`Network load failed ${event.errorText || 'unknown'} (${event.requestId})`);
  });
  return {
    id,
    resources,
    timeline,
    marker,
    setPhase(nextPhase, label = `${nextPhase}-start`) { marker(label, nextPhase); },
    async finish(boundary = 'observation') {
      // Browser-owned favicon requests may start after load/fonts readiness.
      // Collect a named finite quiet window; this is NOT a readiness latency.
      marker('resource-drain-start');
      const deadline=performance.now()+5000;
      let quiet=false;
      while(performance.now()<deadline){
        const unsettled=[...resources.values()].some(resource=>!resource.settled&&!resource.failure);
        if(!unsettled&&pending.size===0&&performance.now()-lastActivity>=200){quiet=true;break;}
        await new Promise(resolve=>setTimeout(resolve,10));
      }
      if(quiet) marker('resource-quiet-achieved');
      else { marker('resource-quiet-timed-out'); failures.push(`resource drain exceeded5s or never quiet for200ms at ${boundary}`); }
      await waitForPending(1000, `${boundary}-drain`);
      for (const resource of resources.values()) {
        if (!resource.settled && !resource.failure) {
          resource.failure = 'request did not settle before observation boundary';
          failures.push(`Network request did not settle (${resource.request_id})`);
        }
        if (resource.status === null || resource.status >= 400) failures.push(`HTTP response unusable ${resource.status ?? 'missing'} (${resource.url})`);
      }
      marker('resource-drain-end');
      return this.snapshot().resources;
    },
    resetResources() { resources.clear(); marker('warm-prime-resources-cleared'); },
    async finalizeAfterClose() {
      detach();
      await waitForPending(1000, 'post-context-close');
      for (const resource of resources.values()) if (!resource.settled && !resource.failure) {
        resource.failure = 'request did not settle before post-close receipt boundary';
        failures.push(`Network request did not settle before post-close receipt (${resource.request_id})`);
      }
      marker('post-close-receipt-finalized');
      return {...this.snapshot(), failures: [...failures]};
    },
    snapshot() {
      // Resources may still receive CDP mutations while the context closes. A receipt
      // is a value snapshot, not a pre-close array of mutable resource objects.
      const observed = [...resources.values()].map(resource => structuredClone(resource)).sort((a, b) => a.url.localeCompare(b.url));
      return {collection_id: id, resources: observed, resource_totals: summarizeResources(observed), timeline: timeline.map(event => ({...event}))};
    },
  };
}

export async function closeContextWithCollection(context, collection, failures) {
  collection.marker('context-close-start', 'context-close');
  try { await context.close(); }
  catch (error) { failures.push(`Context close: ${error}`); }
  finally { collection.marker('context-close-end'); }
  return collection.finalizeAfterClose();
}

export async function oneObservation(browser, input, {kind, ordinal}) {
  const {profile, lane, sourceOrigin, targetOrigin} = input;
  const context = await browser.newContext({viewport: {width: lane.width, height: 1200}, colorScheme: lane.theme, reducedMotion: 'reduce', serviceWorkers: 'block', javaScriptEnabled: profile.javaScriptEnabled, acceptDownloads: false});
  const pageErrors = [], blocked = [], collectorErrors = [];
  let page, collection, observation, readiness=null, route, warmPrimeObservation=null;
  const taskStarted = {wall_clock: new Date().toISOString(), node_monotonic_ms: performance.now()};
  try {
    if (input.streamCancelObserver) await context.addInitScript(input.streamCancelObserver.installStreamCancelObserver, {capacity: 64, stackLimit: 1024});
    await context.routeWebSocket('**/*', socket => {
      blocked.push(`WebSocket refused: ${socket.url()}`);
      socket.close();
    });
    page = await context.newPage();
    page.setDefaultTimeout(10000);page.setDefaultNavigationTimeout(10000);
    const cdp = await context.newCDPSession(page);
    const allowedOrigins = [profile.id === 'source' ? sourceOrigin : targetOrigin];
    collection = await installCDPCollection(cdp, {origins: allowedOrigins, profile: profile.id, phase: kind, failures: collectorErrors, policyFailures: blocked});
    page.on('pageerror', error => pageErrors.push(String(error)));
    route = profileRoute(profile, profile.id === 'source' ? sourceOrigin : targetOrigin, lane.theme);
    if (kind === 'warm') {
      // This is intentionally a same-context prime then reload. A successful reload
      // is not called warm unless CDP reports cache provenance in the reload slice.
      collection.setPhase('warm-prime');
      await page.goto(route, {waitUntil: 'load'});
      await formReadiness(page, profile.id);
      const primeResources=await collection.finish('warm-prime');
      warmPrimeObservation={
        collection_id: collection.id, resources: primeResources, resource_totals: summarizeResources(primeResources), cache: warmCacheObservation(primeResources),
        boundary: 'same-context warm prime; retained separately and excluded from warm reload totals',
      };
      collection.resetResources();
      collection.setPhase('warm-reload');
      taskStarted.wall_clock = new Date().toISOString(); taskStarted.node_monotonic_ms = performance.now();
      await page.reload({waitUntil: 'domcontentloaded'});
    } else {
      collection.setPhase('cold-navigation');
      await page.goto(route, {waitUntil: 'domcontentloaded'});
    }
    readiness = await formReadiness(page, profile.id);
    collection.marker('readiness-complete');
    const resources = await collection.finish('observation');
    const cache = warmCacheObservation(resources);
    const sourceDocument = profile.id === 'source' ? resources.find(resource => resource.group === 'document') : null;
    observation = {
      collection_id: collection.id, profile: profile.id, accounting: profile.accounting, kind, ordinal, lane, route, task_started: taskStarted,
      readiness, cache, resources, resource_totals: summarizeResources(resources),
      source_dynamic_html_observation: sourceDocument ? {status: sourceDocument.status, sha256: sourceDocument.body.sha256, decoded_body_bytes_observed: sourceDocument.decoded_body_bytes_observed, availability: sourceDocument.body.availability} : null,
      warm_prime_observation: warmPrimeObservation, blocked, page_errors: pageErrors, ok: true, failures: kind === 'warm' && cache.status !== 'observed-cache-hit' ? ['warm-cache-not-observed'] : [],
    };
  } catch (error) {
    const resources=collection?[...collection.resources.values()]:[];
    try{if(page&&!page.isClosed())readiness=await page.evaluate(profile.id==='source'?sourceReadiness():targetReadiness(profile.id));}catch{}
    observation = {collection_id: collection?.id ?? null, profile: profile.id, accounting: profile.accounting, kind, ordinal, lane, task_started: taskStarted, readiness, cache: warmCacheObservation(resources), resources, resource_totals: summarizeResources(resources), source_dynamic_html_observation: null, warm_prime_observation: warmPrimeObservation, blocked, page_errors: pageErrors, ok: false, failures: ['collector-exception', String(error)]};
  } finally {
    if (input.streamCancelObserver && observation && page && !page.isClosed()) {
      try {
        collection?.marker('stream-cancel-snapshot-start');
        observation.stream_cancel_diagnostic = await page.evaluate(input.streamCancelObserver.streamCancelObserverSnapshot);
        if (!observation.stream_cancel_diagnostic.available) collectorErrors.push('stream cancel observer unavailable');
        collection?.marker('stream-cancel-snapshot-end');
      } catch (error) { collectorErrors.push(`Stream cancel snapshot: ${error}`); }
    }
    const receipt = await (collection ? closeContextWithCollection(context, collection, collectorErrors) : (() => {
      // Collection setup can fail before CDP is available; retain that distinct gap.
      return context.close().then(() => null, error => { collectorErrors.push(`Context close: ${error}`); return null; });
    })());
    if (observation && receipt) {
      observation.collection_id=receipt.collection_id;
      observation.resources=receipt.resources;
      observation.resource_totals=receipt.resource_totals;
      observation.timeline=receipt.timeline;
      observation.timeline_clock_note='cdp_timestamp is browser-clock data and node_monotonic_ms is collector-clock data; these values are not subtracted or compared.';
      observation.cache=warmCacheObservation(receipt.resources);
      const sourceDocument = profile.id === 'source' ? receipt.resources.find(resource => resource.group === 'document') : null;
      observation.source_dynamic_html_observation=sourceDocument ? {status: sourceDocument.status, sha256: sourceDocument.body.sha256, decoded_body_bytes_observed: sourceDocument.decoded_body_bytes_observed, availability: sourceDocument.body.availability} : null;
      observation.blocked=[...blocked];
      observation.collector_errors=receipt.failures;
      observation.page_errors=[...pageErrors];
      const disposition=sampleDisposition({blocked, collectorErrors: receipt.failures, pageErrors, readiness: observation.readiness, resources: receipt.resources, profile: profile.id});
      observation.failures=[...new Set([...observation.failures, ...disposition.failures])];
      observation.ok=observation.failures.length===0;
    } else if (observation && (blocked.length || collectorErrors.length || pageErrors.length)) {
      observation.blocked=[...blocked]; observation.collector_errors=[...collectorErrors]; observation.page_errors=[...pageErrors];
      observation.ok=false; observation.failures.push('error-through-context-close');
    }
  }
  return observation;
}

async function main() {
  const [sourceOriginInput, sourceRoot, targetOriginInput, targetBinary, sourcePacket, privateParent, sourcePID, targetPID] = process.argv.slice(2);
  const diagnosticDesktop = process.argv[10] === '--diagnostic-desktop';
  const diagnosticStreamCancel = process.argv[10] === '--diagnostic-stream-cancel';
  check(process.argv.length === (diagnosticDesktop || diagnosticStreamCancel ? 11 : 10), 'Use payload-browser.mjs SOURCE_ORIGIN SOURCE_ROOT TARGET_ORIGIN TARGET_BINARY SOURCE_PACKET PRIVATE_PARENT SOURCE_PID TARGET_PID [--diagnostic-desktop|--diagnostic-stream-cancel]');
  const sourceOrigin = exactLoopbackOrigin(sourceOriginInput, 'SOURCE_ORIGIN');
  const targetOrigin = exactLoopbackOrigin(targetOriginInput, 'TARGET_ORIGIN');
  check(sourceOrigin !== targetOrigin, 'SOURCE_ORIGIN and TARGET_ORIGIN must be distinct');
  check(sourceRoot === SOURCE_ROOT, `SOURCE_ROOT must be the selected ${SOURCE_ROOT}`);
  regularAbsolute(sourceRoot, 'SOURCE_ROOT', 'directory');
  regularAbsolute(privateParent, 'PRIVATE_PARENT', 'directory');
  const sourceProcess = attestServer(processIdentity(sourcePID, 'SOURCE'), sourceOrigin, {root: sourceRoot, label: 'source server'});
  const targetBinaryIdentity = fileIdentity(targetBinary, 'TARGET_BINARY');
  const targetProcess = attestServer(processIdentity(targetPID, 'TARGET'), targetOrigin, {binary: targetBinary, label: 'target server'});
  const pins = {source_root: sourceRoot, source_process: sourceProcess, source_packet: privatePacket(sourcePacket, privateParent, sourceRoot), target_binary: targetBinaryIdentity, target_process: targetProcess, runner: fileIdentity(fileURLToPath(import.meta.url), 'collector runner'), helper: fileIdentity(path.join(here, 'payload-checks.mjs'), 'collector helper'), playwright: approvedPlaywright(), hardware: hardware()};
  const streamCancelObserver = diagnosticStreamCancel ? await import('./stream-cancel-observer.mjs') : null;
  if (streamCancelObserver) pins.stream_cancel_observer = fileIdentity(path.join(here, 'stream-cancel-observer.mjs'), 'stream cancel observer');
  regularAbsolute(CHROME, 'installed Chrome');
  const out = fs.mkdtempSync(path.join(privateParent, 'p10-payload-')); fs.chmodSync(out, 0o700);
  const run = diagnosticStreamCancel ? Object.freeze({
    id: 'p10-stream-cancel-calls-diagnostic-v1', mode: 'instrumented-stream-calls-diagnostic-only', matrix: [MATRIX[1]], profiles: [PROFILES[0]], sampleCount: 5, kinds: ['cold'],
    lanes: {cold: 'Up to five cold source1440/light observations; stop at first failure. Wrapped stream cancel methods alter native function identity and overhead; no per-request correlation or performance sample.'},
  }) : diagnosticDesktop ? Object.freeze({
    id: 'p10-payload-close-diagnostic-desktop-v1', mode: 'diagnostic-only-not-a-360-benchmark', matrix: [MATRIX[1]], profiles: [PROFILES[0]], sampleCount: 5, kinds: ['cold'],
    lanes: {cold: 'Five serial fresh-context source observations at 1440/light only; stop at first failure. Diagnostic-only, not the 360-sample benchmark.'},
  }) : Object.freeze({
    id: 'p10-payload-readiness-descriptive-v1', mode: 'default-descriptive-matrix', matrix: MATRIX, profiles: PROFILES, sampleCount: SAMPLES, kinds: ['cold', 'warm'],
    lanes: {cold: 'All six cold lanes first:30 serial fresh-context navigations each', warm: 'Then30 serial same-context prime-and-reload observations per lane; only CDP cache provenance establishes warm cache'},
  });
  const plan = {schema_version: 1, id: run.id, mode: run.mode, created_at: new Date().toISOString(), pins,
    matrix: run.matrix, profiles: run.profiles.map(({id, accounting, javaScriptEnabled}) => ({id, accounting, javaScriptEnabled})), sample_count: run.sampleCount,
    lanes: run.lanes,
    stopping:'Preserve every sample; stop at first failure and leave all unvisited samples explicitly not run. No completed360-sample claim from a partial collection.',
    error_channels:'blocked lists explicit origin/method/WebSocket policy refusals; collector_errors lists transport/body/drain/close failures. Neither channel is ignored or relabeled as a pass.',
    observation_boundary:'After form/font readiness, include all admitted requests through a200ms network-quiet window, bounded at5s, then record context-close CDP events through a separately marked post-close receipt boundary. Pending event joins are bounded and timeouts remain failures. These waits are not latency measurements; later events after receipt finalization are outside this finite observation.',
    safety: {origins: [sourceOrigin, targetOrigin], request_methods: ['GET', 'HEAD'], websocket: 'refused through CDP Fetch/Network', server_lifecycle: 'external only; collector never starts or restarts either server'},
    exclusions: ['no latency, paint, FUV, input, heap, long-task, compression calculation, or relative-speed claim', 'no shell-equivalent target comparison', 'no full performance gate or acceptance threshold', 'no source response bodies in output'], verification: 'not-verified'};
  const planFile = put(out, 'plan.json', plan);
  console.log(JSON.stringify({out, plan_sha256: planFile.sha256, mode: run.mode, status: 'descriptive collection starting; no threshold or comparison verdict'}));
  const {chromium} = await import(pathToFileURL(pins.playwright.module.path).href);
  const report = {schema_version: 1, plan: planFile, started_at: new Date().toISOString(), samples: [], limits: plan.exclusions, status: 'incomplete'};
  let browser;
  try {
    browser = await chromium.launch({executablePath: CHROME, headless: true, args: ['--disable-background-networking', '--no-proxy-server', '--host-resolver-rules=MAP * 0.0.0.0, EXCLUDE 127.0.0.1']});
    check(browser.version() === CHROME_VERSION, `Approved Chrome ${CHROME_VERSION} required; got ${browser.version()}`);
    report.chrome = browser.version();
    collectionLoop: for (const kind of run.kinds) for (const lane of run.matrix) for (const profile of run.profiles) {
      for (let ordinal = 1; ordinal <= run.sampleCount; ordinal++) {
        // Serial by construction: this loop awaits every context before opening the next.
        const sample=await oneObservation(browser, {profile, lane, sourceOrigin, targetOrigin, streamCancelObserver}, {kind, ordinal});
        report.samples.push(sample);
        put(out, `sample-${String(report.samples.length).padStart(3,'0')}.json`, sample);
        if(!sample.ok||ordinal===run.sampleCount)console.log(JSON.stringify({out,profile:profile.id,lane,kind,ordinal,ok:sample.ok,failures:sample.failures}));
        if(!sample.ok)break collectionLoop;
      }
    }
    report.status = report.samples.every(sample => sample.ok)
      ? diagnosticStreamCancel ? 'diagnostic-stream-cancel-observations-complete-not-a-performance-sample'
        : diagnosticDesktop ? 'diagnostic-desktop-observations-complete-not-a-360-benchmark' : 'descriptive-observations-complete-not-a-performance-gate'
      : diagnosticStreamCancel ? 'failed-partial-diagnostic-stream-cancel-observations'
        : diagnosticDesktop ? 'failed-partial-diagnostic-desktop-observations' : 'failed-partial-descriptive-observations';
  } catch (error) {
    report.status = 'failed-before-or-during-descriptive-observations'; report.failure = String(error);
  } finally {
    if (browser) try { await browser.close(); } catch (error) {
      report.browser_close_failure = String(error);
      report.status = 'failed-browser-close-during-descriptive-observations';
    }
    report.finished_at = new Date().toISOString();
    report.planned_samples=run.matrix.length*run.profiles.length*run.kinds.length*run.sampleCount;
    report.not_run_samples=report.planned_samples-report.samples.length;
    report.failed_samples = report.samples.filter(sample => !sample.ok).length;
    put(out, 'receipt.json', report);
    console.log(JSON.stringify({out, status: report.status, samples: report.samples.length, failed_samples: report.failed_samples}));
    if (!['descriptive-observations-complete-not-a-performance-gate', 'diagnostic-desktop-observations-complete-not-a-360-benchmark', 'diagnostic-stream-cancel-observations-complete-not-a-performance-sample'].includes(report.status)) process.exitCode = 1;
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await main();
