import test from 'node:test';
import assert from 'node:assert/strict';
import {createMemoryMeasurements, waitForMemoryStartup} from './memory.mjs';

const targets = (url = 'http://127.0.0.1:18766/theme-lab', type = 'page') => [{targetId: 'page-1', type, url}, {targetId: 'frame-1', type: 'iframe', url: 'http://127.0.0.1:18766/theme-lab/preview?specimen=dashboard'}];

test('startup waits for teardown and retains every unknown target', async () => {
  let clock = 0, calls = 0;
  const observations = [], raw = [{targetId: 'closing', type: 'other', url: 'about:blank'}];
  await waitForMemoryStartup({guard: {send: async () => ({targetInfos: calls++ ? [] : raw})}, now: () => clock, observations, pauseFn: async ms => { clock += ms; }});
  assert.equal(calls, 2); assert.deepEqual(observations[0].raw_target_infos, raw); assert.deepEqual(observations[1].raw_target_infos, []);
});
test('startup never silently excludes a persistent unknown target', async () => {
  let clock = 0; const observations = [];
  await assert.rejects(waitForMemoryStartup({guard: {send: async () => ({targetInfos: [{targetId: 'unknown', type: 'other'}]})}, now: () => clock, observations, timeoutMs: 200, pauseFn: async ms => { clock += ms; }}), /did not finish teardown/);
  assert.equal(clock, 200); assert.equal(observations.length, 3); assert.match(observations.at(-1).error, /teardown/);
});
test('startup retains a protocol read failure', async () => {
  const observations = [];
  await assert.rejects(waitForMemoryStartup({guard: {send: async () => { throw new Error('synthetic target read'); }}, now: () => 0, observations}), /synthetic target read/);
  assert.match(observations[0].error, /synthetic target read/);
});
function fixture({targetInfos = targets(), failGC = false} = {}) {
  const sessions = [{targetId: 'page-1', sessionId: 'session-page', appliedPolicy: true, resumed: true, detached: false}, {targetId: 'frame-1', sessionId: 'session-frame', appliedPolicy: true, resumed: true, detached: false}];
  const guard = {coverage: {allowedOrigin: 'http://127.0.0.1:18766', sessions, rawNetworkEvents: []}, async send(method, _params, sessionId) {
    if (method === 'Target.getTargets') return {targetInfos};
    if (method === 'Runtime.getIsolateId') return {id: sessionId === 'session-page' ? 'isolate-page' : 'isolate-frame'};
    if (method === 'HeapProfiler.collectGarbage') { if (failGC) throw new Error('synthetic GC failure'); return {}; }
    if (method === 'Runtime.getHeapUsage') return {usedSize: 10, totalSize: 20};
    if (method === 'Memory.getDOMCounters') return {documents: 1, nodes: 2, jsEventListeners: 0};
    throw new Error(`unexpected ${method}`);
  }};
  return guard;
}
const make = (guard, overrides = {}) => {
  let clock = 0;
  return createMemoryMeasurements({guard, browserPid: 1, serverPid: 2, now: () => clock, wallNow: (() => { let n = 0; return () => `2026-09-17T00:00:${String(n++).padStart(2, '0')}.000Z`; })(), sampleFn: async () => ({completed_ms: clock, rows: [{pid: 1, start_identity: 'browser'}, {pid: 2, start_identity: 'server'}], errors: []}), pauseFn: async ms => { clock += ms; }, ...overrides});
};
async function failedEndpoint(guard) { const memory = await make(guard); const lane = await memory.begin('test', 0); try { await assert.rejects(() => memory.settledEndpoint(lane)); return lane; } finally { await memory.end(lane); } }

async function endpointWithIdleWakes(wakes) {
  let clock = 0, released = false, postIdleSampleCatchup = false;
  const samplerWaiters = [], idlePauseRequests = [];
  const releaseSampler = () => {
    released = true;
    for (const resolve of samplerWaiters.splice(0)) resolve();
  };
  const pauseFn = async ms => {
    // Sampling subtracts elapsed acquisition time, so its delay can be fractional
    // or zero. Only the exact remaining idle deadline belongs to the idle task.
    if (!released && ms >= 0 && ms <= 100 && ms !== 5000 - clock &&
        !(postIdleSampleCatchup && ms === 10)) {
      return new Promise(resolve => samplerWaiters.push(resolve));
    }
    if (postIdleSampleCatchup && ms === 10) {
      clock += ms;
      for (const resolve of samplerWaiters.splice(0)) resolve();
      await Promise.resolve();
      return;
    }
    idlePauseRequests.push(ms);
    const advance = wakes.shift();
    assert.notEqual(advance, undefined, `unexpected idle pause request ${ms}`);
    clock += advance;
    for (const resolve of samplerWaiters.splice(0)) resolve();
    await Promise.resolve();
  };
  const sampleFn = async () => ({completed_ms: clock, rows: [
    {pid: 1, start_identity: 'browser'}, {pid: 2, start_identity: 'server'},
  ], errors: []});
  const guard = fixture(), send = guard.send.bind(guard);
  guard.send = async (...args) => {
    if (args[0] === 'Runtime.getHeapUsage') postIdleSampleCatchup = true;
    return send(...args);
  };
  const memory = await createMemoryMeasurements({guard, browserPid: 1, serverPid: 2,
    now: () => clock, wallNow: () => '2026-09-17T00:00:00.000Z', sampleFn, pauseFn});
  const lane = await memory.begin('idle-deadline', 0);
  try {
    const endpoint = await memory.settledEndpoint(lane);
    assert.deepEqual(lane.errors, [], 'fake sampler must remain live');
    return {endpoint, idlePauseRequests, unusedWakes: [...wakes]};
  } finally {
    releaseSampler();
    await memory.end(lane);
  }
}

test('refuses near-match paths and retains exact target observation', async () => { const lane = await failedEndpoint(fixture({targetInfos: targets('http://127.0.0.1:18766/theme-lab-evil')})); assert.match(lane.endpoint_attempts[0].error, /Out-of-scope/); assert.deepEqual(lane.topology_observations[0].raw_target_infos, targets('http://127.0.0.1:18766/theme-lab-evil')); });
test('refuses unknown target types while retaining raw Target.getTargets', async () => { const raw = targets(); raw[0].type = 'other'; const lane = await failedEndpoint(fixture({targetInfos: raw})); assert.match(lane.endpoint_attempts[0].error, /Unexpected live memory target/); assert.deepEqual(lane.topology_observations[0].raw_target_infos, raw); });
test('retains partial endpoint and failed GC observation', async () => { const lane = await failedEndpoint(fixture({failGC: true})); assert.equal(lane.endpoint_attempts.length, 1); assert.equal(lane.endpoint_attempts[0].gc.length, 1); assert.equal(lane.endpoint_attempts[0].gc[0].ok, false); assert.match(lane.endpoint_attempts[0].error, /synthetic GC failure/); });
test('valid endpoint uses injected sampler and records wall boundaries', async () => { const memory = await make(fixture()); const lane = await memory.begin('test', 0); try { const endpoint = await memory.settledEndpoint(lane); assert.equal(endpoint.error, undefined); assert.equal(endpoint.targets.length, 2); assert.ok(endpoint.idle.completed_ms - endpoint.idle.started_ms >= 5000); assert.match(endpoint.wall_started_at, /T/); assert.match(endpoint.wall_completed_at, /T/); assert.equal(lane.endpoint_attempts[0], endpoint); } finally { await memory.end(lane); } });

test('settled endpoint waits again when a five-second pause wakes early', async () => {
  const early = 4999.730292;
  const {endpoint, idlePauseRequests, unusedWakes} = await endpointWithIdleWakes([early, 5000 - early]);
  assert.ok(endpoint.idle.completed_ms - endpoint.idle.started_ms >= 5000);
  assert.deepEqual(idlePauseRequests, [5000, 5000 - early]);
  assert.deepEqual(unusedWakes, []);
});

test('settled endpoint accepts exact-deadline and oversleep wakeups without another wait', async t => {
  for (const elapsed of [5000, 5000.5]) await t.test(String(elapsed), async () => {
    const {endpoint, idlePauseRequests, unusedWakes} = await endpointWithIdleWakes([elapsed]);
    assert.equal(endpoint.idle.completed_ms - endpoint.idle.started_ms, elapsed);
    assert.deepEqual(idlePauseRequests, [5000]);
    assert.deepEqual(unusedWakes, []);
  });
});
