// One synthetic instrument control, never the actual Gallery memory workload.
// node memory-control.mjs ABS_FROZEN_MANIFEST NEW_OUTPUT positive|retention|startup|shutdown
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import crypto from 'node:crypto';
import path from 'node:path';
import {createReadStream} from 'node:fs';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {createControlServer} from './memory-control-fixture.mjs';
import {memoryControlFlags, memoryControlModes} from './memory-control-profile.mjs';
import {validateStartupWindow} from './startup-window.mjs';
import {trackOwnedChild, terminateOwned} from './owned_process.mjs';
import {disposeOwnedContext} from './owned_context.mjs';
import {connectGuard} from './cdp_guard.mjs';
import {captureGuideLifecycle} from './lifecycle.mjs';
import {joinGuideLifecycle} from './lifecycle-check.mjs';
import {createMemoryMeasurements} from './memory.mjs';
import {evaluateMemoryV2Metrics} from '../../../docs/assessments/fixtures/styleguide-performance/memory-v2-metrics.mjs';

const [manifestFile, output, mode] = process.argv.slice(2);
assert.equal(process.argv.length, 5);
assert.ok(path.isAbsolute(manifestFile) && path.isAbsolute(output));
assert.ok(['positive', 'retention', 'startup', 'shutdown'].includes(mode));
const manifestBytes = await fs.readFile(manifestFile), manifest = JSON.parse(manifestBytes);
assert.equal(manifest.schema, 'appkit-memory-control-inputs-v1');
assert.equal(manifest.tools.node.path, process.execPath);
assert.equal(manifest.owner, fileURLToPath(import.meta.url));
const profileName = manifest.startup_profile ?? 'baseline';
assert.deepEqual(manifest.chrome_flags, memoryControlFlags(profileName), 'unreviewed Chrome arguments');
const admittedModes = memoryControlModes(profileName);
assert.deepEqual(manifest.allowed_modes ?? ['positive', 'retention'], admittedModes);
assert.ok(admittedModes.includes(mode), 'mode is outside this frozen profile contract');
await fs.mkdir(output, {mode: 0o700});
await fs.writeFile(path.join(output, 'inputs.json'), manifestBytes, {flag: 'wx'});
const now = () => performance.now();
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
const digest = async file => {
  const hash = crypto.createHash('sha256');
  for await (const chunk of createReadStream(file)) hash.update(chunk);
  return hash.digest('hex');
};
const report = {schema: 'appkit-memory-control-v1', mode, started_at: new Date().toISOString(),
  owner_pid: process.pid, startup_profile: profileName,
  passed: false, errors: [], actions: [], bootstrap: [], lifecycle: [], checkpoints: [], guard_events: [],
  attribution: 'not-verified', memory_complete: false, full_contract_complete: false,
  limits: ['Synthetic instrument control, not Gallery or consumer performance.',
    'CDP application guard is not OS-wide containment.', 'Summed Chrome RSS is not app allocation.',
    'Detached/native/GPU reclamation and complete memory acceptance remain unproven.']};
let chrome, server, guard, memory, lane, bootstrapSocket, origin, endpoint, contextId, timeout, aborted = false;
const chromeCapture = {stdout: '', stderr: '', truncated: false};
async function verifyInputs() {
  for (const input of [...manifest.sources, ...Object.values(manifest.tools)])
    assert.equal(await digest(input.path), input.sha256, 'input drift: ' + input.path);
  assert.equal(await digest(manifestFile), crypto.createHash('sha256').update(manifestBytes).digest('hex'), 'manifest drift');
}
async function bounded(work, milliseconds) {
  let timer;
  try { return await Promise.race([work(), new Promise((_, reject) => {
    timer = setTimeout(() => reject(Error('bounded operation timeout')), milliseconds);
  })]); } finally { clearTimeout(timer); }
}
async function until(read, milliseconds = 10000) {
  const end = now() + milliseconds;
  while (!aborted && now() < end) {
    assert.ok(!guard?.coverage.failed, 'application guard failed');
    const result = await read();
    if (result) return result;
    await pause(50);
  }
  throw Error('control condition deadline');
}
// Only bootstrap commands are allowed; application commands go through the guard.
async function bootstrap(url) {
  bootstrapSocket = new WebSocket(url);
  const socket = bootstrapSocket, pending = new Map(); let id = 0;
  await bounded(() => new Promise((resolve, reject) => {
    socket.addEventListener('open', resolve, {once: true});
    socket.addEventListener('error', () => reject(Error('bootstrap socket error')), {once: true});
  }), 5000);
  socket.addEventListener('message', event => {
    const value = JSON.parse(event.data), waiting = pending.get(value.id);
    if (waiting) { pending.delete(value.id); report.bootstrap.push({at_ms: now(), response: value});
      value.error ? waiting.reject(Error(JSON.stringify(value.error))) : waiting.resolve(value.result); }
  });
  const send = (method, params = {}) => bounded(() => new Promise((resolve, reject) => {
    assert.ok(['Browser.getVersion', 'Target.getTargets', 'Target.closeTarget'].includes(method));
    const message = {id: ++id, method, params}; pending.set(id, {resolve, reject});
    report.bootstrap.push({at_ms: now(), command: message}); socket.send(JSON.stringify(message));
  }), 5000);
  try {
    report.browser_version = await send('Browser.getVersion');
    assert.equal(report.browser_version.product, 'Chrome/' + manifest.chrome_version);
    const {targetInfos} = await send('Target.getTargets');
    const closingTargets = new Map();
    for (const target of targetInfos) {
      if (['browser', 'tab', 'browser_ui'].includes(target.type)) continue;
      assert.equal(target.type, 'page'); assert.equal(target.url, 'about:blank');
      closingTargets.set(target.targetId, target.browserContextId);
      assert.equal((await send('Target.closeTarget', {targetId: target.targetId})).success, true);
    }
    await until(async () => {
      const remaining = (await send('Target.getTargets')).targetInfos.filter(t => !['browser', 'tab', 'browser_ui'].includes(t.type));
      if (profileName !== 'baseline') assert.ok(remaining.every(t => t.type === 'page' && t.url === 'about:blank' &&
        closingTargets.has(t.targetId) && closingTargets.get(t.targetId) === t.browserContextId), 'unexpected target during blank teardown');
      return remaining.length === 0;
    });
    if (profileName !== 'baseline') {
      const window = report.startup_window = {started_ms: now(), duration_ms: 5000,
        max_gap_ms: 500, min_samples: 10, samples: []};
      do {
        assert.ok(!aborted);
        const sample = {started_ms: now()}; window.samples.push(sample);
        const observed = await send('Target.getTargets');
        sample.completed_ms = now(); sample.target_infos = observed.targetInfos;
        assert.ok(observed.targetInfos.every(t => ['browser', 'tab', 'browser_ui'].includes(t.type)), 'unexpected target during startup window');
        const previous = window.samples.at(-2)?.completed_ms ?? window.started_ms;
        assert.ok(sample.completed_ms - previous <= window.max_gap_ms, 'startup observation gap exceeded');
        if (sample.completed_ms - window.started_ms >= window.duration_ms) break;
        await pause(100);
      } while (!aborted);
      assert.ok(!aborted);
      window.completed_ms = window.samples.at(-1).completed_ms;
      report.startup_coverage = validateStartupWindow(window);
    }
  } finally {
    await bounded(() => new Promise(resolve => { socket.addEventListener('close', resolve, {once: true}); socket.close(); }), 5000);
    bootstrapSocket = null;
  }
}
async function send(method, params = {}, sessionId) {
  assert.ok(!aborted && !guard.coverage.failed, 'control guard unavailable');
  const row = {method, params, session_id: sessionId ?? null, started_ms: now()}; report.actions.push(row);
  try { row.result = await guard.send(method, params, sessionId); return row.result; }
  catch (error) { row.error = String(error); throw error; }
  finally { row.completed_ms = now(); }
}
const session = type => guard.coverage.sessions.find(s => s.type === type && !s.detached && s.resumed && s.appliedPolicy);
async function node(selector, type = 'page') {
  const target = session(type); if (!target) return null;
  const {root} = await send('DOM.getDocument', {}, target.sessionId);
  const {nodeId} = await send('DOM.querySelector', {nodeId: root.nodeId, selector}, target.sessionId);
  return nodeId ? {nodeId, sessionId: target.sessionId} : null;
}
async function markup(selector, type) {
  const found = await node(selector, type);
  return found ? (await send('DOM.getOuterHTML', {nodeId: found.nodeId}, found.sessionId)).outerHTML : null;
}
async function click(selector) {
  const found = await until(() => node(selector));
  const {model} = await send('DOM.getBoxModel', {nodeId: found.nodeId}, found.sessionId);
  const [x1, y1, x2, y2, x3, y3, x4, y4] = model.content;
  const x = (x1 + x2 + x3 + x4) / 4, y = (y1 + y2 + y3 + y4) / 4;
  for (const type of ['mousePressed', 'mouseReleased'])
    await send('Input.dispatchMouseEvent', {type, x, y, button: 'left', clickCount: 1}, found.sessionId);
}
async function navigatePreview() {
  const before = await until(() => markup('#generation', 'iframe'));
  const traceStart = report.lifecycle.length;
  await click('#next');
  await until(async () => report.lifecycle.slice(traceStart).some(event =>
    event.method === 'Page.frameStoppedLoading' && event.sessionId === session('iframe')?.sessionId));
  const after = await until(async () => { const html = await markup('#generation', 'iframe'); return html && html !== before ? html : null; });
  return {before, after};
}
async function checkpoint(label) {
  await captureGuideLifecycle({guard, lifecycleEvents: report.lifecycle, contextId, label, checkpoints: report.checkpoints});
  const result = joinGuideLifecycle({events: report.lifecycle, checkpoints: report.checkpoints, contextId, origin});
  report.lifecycle_result = result;
  assert.ok(result.evidence_valid && result.live_membership_complete, JSON.stringify(result.unqualified_reasons));
}
async function run() {
  await verifyInputs(); report.inputs_verified_before = true;
  if (mode !== 'startup') {
    server = createControlServer({mode: mode === 'shutdown' ? 'positive' : mode});
    await bounded(() => new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); }), 3000);
    origin = `http://127.0.0.1:${server.address().port}`; report.origin = origin;
    report.server_pid = process.pid; // The tiny fixture is this owned Node process, not Gallery.
  }
  const profile = path.join(output, 'chrome-profile'); await fs.mkdir(profile, {mode: 0o700});
  const args = manifest.chrome_flags.map(flag => flag.replace('{PROFILE}', profile));
  report.chrome_command = {executable: manifest.tools.chrome.path, args};
  chrome = trackOwnedChild(spawn(manifest.tools.chrome.path, args, {stdio: ['ignore', 'pipe', 'pipe']}));
  report.chrome_pid = chrome.child.pid;
  report.chrome_state = chrome.state; report.chrome_capture = chromeCapture;
  for (const name of ['stdout', 'stderr']) chrome.child[name].on('data', bytes => {
    if (Buffer.byteLength(chromeCapture[name]) + bytes.length > 1024 * 1024) chromeCapture.truncated = true;
    else chromeCapture[name] += bytes.toString();
  });
  endpoint = await until(async () => {
    assert.ok(!chrome.state.exit && !chrome.state.error && !chromeCapture.truncated, 'Chrome startup failed');
    return chromeCapture.stderr.match(/DevTools listening on (ws:\/\/127\.0\.0\.1:\d+\/devtools\/browser\/[^\s]+)/)?.[1];
  });
  report.endpoint = endpoint;
  await bootstrap(endpoint);
  if (mode === 'startup') { report.observation_complete = true; return; }
  guard = await connectGuard(endpoint, {allowedOrigin: origin, events: report.guard_events, lifecycleEvents: report.lifecycle});
  report.guard = guard.coverage; await guard.ready;
  const created = await send('Target.createBrowserContext', {disposeOnDetach: true});
  contextId = created.browserContextId; report.context_id = contextId;
  const target = await send('Target.createTarget', {url: 'about:blank', browserContextId: contextId});
  const page = await until(async () => session('page'));
  assert.equal(page.targetId, target.targetId);
  await send('Page.navigate', {url: origin + '/theme-lab'}, page.sessionId);
  report.warm_action = await navigatePreview();
  if (mode === 'shutdown') {
    // Isolate the cleanup hypothesis; this does not collect or qualify memory.
    await checkpoint('shutdown-before-work');
    report.shutdown_cycles = [];
    for (let index = 1; index <= 20; index++) {
      const cycle = {index, started_ms: now(), ok: false}; report.shutdown_cycles.push(cycle);
      Object.assign(cycle, await navigatePreview(), {completed_ms: now(), ok: true});
    }
    await checkpoint('shutdown-after-work');
    report.observation_complete = true;
    return;
  }
  await checkpoint('before-baseline');
  memory = await createMemoryMeasurements({guard, now, browserPid: chrome.child.pid, serverPid: process.pid});
  report.owners = {...memory.owners, origin}; report.initial_sample = memory.initial;
  lane = await memory.begin(mode, 0); report.lane = lane;
  lane.contract_id = 'appkit-theme-guide-memory-v2';
  lane.baseline = await memory.settledEndpoint(lane);
  await checkpoint('after-baseline');
  lane.work_started_ms = now();
  if (mode === 'retention') {
    await click('#allocate');
    report.allocation_status = await until(async () => {
      const html = await markup('#allocation-state', 'page'); return html?.includes('8388608') ? html : null;
    });
  }
  for (let index = 1; index <= 20; index++) {
    const cycle = {index, started_ms: now(), ok: false}; lane.cycles.push(cycle);
    Object.assign(cycle, await navigatePreview(), {completed_ms: now(), ok: true});
  }
  lane.work_completed_ms = now();
  await checkpoint('before-final');
  lane.final = await memory.settledEndpoint(lane);
  await checkpoint('after-final');
  await memory.end(lane);
  report.metrics = evaluateMemoryV2Metrics(lane, report.owners);
  assert.ok(report.metrics.metrics_valid, JSON.stringify(report.metrics.errors));
  if (mode === 'positive') assert.equal(report.metrics.budgets_passed, true);
  else {
    assert.equal(report.metrics.budgets_passed, false);
    assert.ok(report.metrics.heap.backingStorageSize.growth_bytes > 4 * 1024 * 1024);
  }
  report.observation_complete = true;
}
try {
  await Promise.race([run(), new Promise((_, reject) => { timeout = setTimeout(() => {
    aborted = true; reject(Error('120-second control deadline'));
  }, 120000); })]);
} catch (error) { report.errors.push(String(error.stack || error)); }
finally {
  clearTimeout(timeout);
  if (memory && lane) try { await bounded(() => memory.end(lane), 2000); } catch (e) { report.errors.push('sampler cleanup: ' + e); }
  if (guard && contextId) {
    report.context_cleanup_events = [];
    try {
      report.context_cleanup = await bounded(() => disposeOwnedContext({guard, contextId,
        events: report.context_cleanup_events}), 26000);
    } catch (e) { report.errors.push('owned context cleanup: ' + e); }
  }
  if (guard) try { await bounded(() => guard.close(), 6000); } catch (e) { report.errors.push('guard cleanup: ' + e); }
  if (guard?.coverage.failed) report.errors.push('guard failed during control cleanup');
  if (bootstrapSocket) bootstrapSocket.close();
  if (chrome) {
    try {
      report.chrome_cleanup = await terminateOwned(chrome, {graceMs: 2000, killMs: 2000, drainMs: 2000});
      if (!report.chrome_cleanup.ok || report.chrome_cleanup.forced || chromeCapture.truncated) report.errors.push('Chrome cleanup/capture failed');
    } catch (e) { report.errors.push('Chrome cleanup: ' + e); }
  }
  if (server) try {
    server.closeAllConnections(); await bounded(() => new Promise((resolve, reject) => server.close(e => e ? reject(e) : resolve())), 2000);
  } catch (e) { report.errors.push('fixture cleanup: ' + e); }
  report.ports = [];
  for (const url of [origin && origin + '/theme-lab', endpoint && 'http://' + new URL(endpoint).host + '/json/version'].filter(Boolean)) {
    const item = {url, refused: false}; report.ports.push(item);
    try { await fetch(url, {signal: AbortSignal.timeout(1500), redirect: 'error'}); }
    catch (e) { item.refused = e.cause?.code === 'ECONNREFUSED'; item.error = String(e); }
    if (!item.refused) report.errors.push('owned port closure unproven: ' + url);
  }
  try { await verifyInputs(); report.inputs_verified_after = true; } catch (e) { report.errors.push(String(e)); }
  report.finished_at = new Date().toISOString();
  report.passed = report.observation_complete === true && !aborted && report.errors.length === 0;
  await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2), {flag: 'wx'});
  console.log(JSON.stringify({output, passed: report.passed, cycles: lane?.cycles.length ?? report.shutdown_cycles?.length ?? 0, errors: report.errors}));
  if (!report.passed) process.exitCode = 1;
}
