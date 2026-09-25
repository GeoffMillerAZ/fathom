import assert from 'node:assert/strict';
import test from 'node:test';
import {captureGuideLifecycle} from './lifecycle.mjs';

function fixture() {
  const origin = 'http://127.0.0.1:12345', contextId = 'owned-context';
  const infos = [
    {targetId: 'guide', type: 'page', url: origin + '/theme-lab', browserContextId: contextId},
    {targetId: 'preview', type: 'iframe', url: origin + '/theme-lab/preview?specimen=dashboard&scheme=light', browserContextId: contextId},
    {targetId: 'internal', type: 'browser_ui', url: 'chrome://internal'},
  ];
  const lifecycleEvents = [{seq: 1, monotonic_ms: performance.now(), kind: 'event', method: 'Target.targetCreated'}];
  const checkpoints = [], calls = [], overrides = new Map();
  let commandId = 0;
  const sessions = infos.slice(0, 2).map(info => ({targetId: info.targetId, sessionId: info.targetId + '-session', type: info.type, appliedPolicy: true, resumed: true, detached: false}));
  const guard = {lifecycleEvents, coverage: {ready: true, failed: false, closed: false, allowedOrigin: origin, sessions}, send: async (method, params, sessionId) => {
    calls.push({method, params, sessionId});
    const id = ++commandId;
    lifecycleEvents.push({seq: lifecycleEvents.length + 1, monotonic_ms: performance.now(), id, kind: 'command', method, params, sessionId});
    let result;
    if (overrides.has(method)) result = await overrides.get(method)(sessionId);
    else if (method === 'Target.getTargets') result = {targetInfos: infos};
    else if (method === 'Runtime.getIsolateId') result = {id: sessionId + '-isolate'};
    else if (method === 'Page.getFrameTree') result = {frameTree: {frame: {id: sessionId + '-frame', loaderId: sessionId + '-loader', ...(sessionId === 'preview-session' ? {parentId: 'guide-session-frame'} : {})}}};
    else throw Error('Unexpected method: ' + method);
    lifecycleEvents.push({seq: lifecycleEvents.length + 1, monotonic_ms: performance.now(), id, kind: 'response', method, sessionId, result: structuredClone(result)});
    return result;
  }};
  const options = {guard, lifecycleEvents, contextId, label: 'baseline', checkpoints};
  return {infos, lifecycleEvents, checkpoints, calls, overrides, guard, options, run: changes => captureGuideLifecycle({...options, ...changes})};
}

test('retains complete raw reads, internal targets and IDs without attribution claims', async () => {
  const f = fixture(), result = await f.run();
  assert.equal(result.status, 'recorded');
  assert.equal(result.reads.length, 10);
  assert.equal(result.raw_targets_before.length, 3);
  assert.equal(result.raw_targets_after.length, 3);
  assert.equal(result.targets[1].parent_frame_id, 'guide-session-frame');
  assert.equal(result.targets[1].isolate_id, 'preview-session-isolate');
  assert.equal(result.trace_start_seq, 1);
  assert.equal(result.trace_end_seq, 21);
  assert.deepEqual(result.errors, []);
  assert.equal(result.attribution, 'not-verified');
  assert.equal(result.memory_complete, false);
  assert.equal(result.full_contract_complete, false);
  assert.equal(f.calls.some(c => /evaluate|navigate|runIf|set|collect/i.test(c.method)), false);
  assert.ok(result.completed_ms >= result.started_ms);
  f.infos[0].targetId = 'mutated';
  assert.equal(result.raw_targets_before[0].targetId, 'guide');
});

test('shared isolate retained per session, not silently discarded or double-summed', async () => {
  const f = fixture(); f.overrides.set('Runtime.getIsolateId', () => ({id: 'shared-isolate'}));
  const result = await f.run();
  assert.deepEqual(result.targets.map(t => t.isolate_id), ['shared-isolate', 'shared-isolate']);
  assert.equal(result.reads.filter(r => r.method === 'Runtime.getIsolateId').length, 4);
  assert.equal(Object.hasOwn(result, 'heap_bytes'), false);
});

const refusals = [
  ['unknown target', f => f.infos.push({targetId: 'unknown', type: 'other', browserContextId: 'owned-context'}), /Unknown application target/],
  ['worker', f => f.infos.push({targetId: 'worker', type: 'worker', browserContextId: 'owned-context'}), /Unknown application target/],
  ['foreign context with identical URL', f => f.infos[1].browserContextId = 'foreign', /outside the owned browser context/],
  ['missing context', f => delete f.infos[1].browserContextId, /outside the owned browser context/],
  ['duplicate target', f => f.infos.push({...f.infos[0]}), /duplicate target/],
  ['missing preview', f => f.infos.splice(1, 1), /one guide page/],
  ['extra same-origin page', f => f.infos.push({...f.infos[0], targetId: 'extra'}), /exactly one live guarded session/],
  ['remote origin', f => f.infos[1].url = 'https://example.invalid/theme-lab/preview', /outside the guide route/],
  ['wrong route', f => f.infos[1].url = 'http://127.0.0.1:12345/settings', /outside the guide route/],
  ['ambiguous sessions', f => f.guard.coverage.sessions.push({...f.guard.coverage.sessions[0], sessionId: 'duplicate'}), /exactly one live guarded session/],
  ['detached session', f => f.guard.coverage.sessions[1].detached = true, /exactly one live guarded session/],
  ['unguarded session', f => f.guard.coverage.sessions[1].appliedPolicy = false, /exactly one live guarded session/],
  ['unresumed session', f => f.guard.coverage.sessions[1].resumed = false, /exactly one live guarded session/],
  ['wrong session type', f => f.guard.coverage.sessions[1].type = 'page', /types differ/],
  ['session collision', f => f.guard.coverage.sessions[1].sessionId = f.guard.coverage.sessions[0].sessionId, /multiple targets/],
  ['unlisted live session', f => f.guard.coverage.sessions.push({...f.guard.coverage.sessions[0], targetId: 'unlisted', sessionId: 'unlisted-session'}), /Live guarded sessions disagree/],
  ['closed guard', f => f.guard.coverage.closed = true, /live ready guard/],
  ['failed guard', f => f.guard.coverage.failed = true, /live ready guard/],
  ['different trace', f => f.options.lifecycleEvents = [], /does not belong/],
  ['missing opt-in trace', f => {delete f.guard.lifecycleEvents; delete f.options.lifecycleEvents;}, /Missing opt-in/],
  ['trace sequence gap', f => f.lifecycleEvents[0].seq = 2, /sequence or clock/],
  ['trace NaN time', f => f.lifecycleEvents[0].monotonic_ms = NaN, /sequence or clock/],
  ['missing raw inventory', f => f.overrides.set('Target.getTargets', () => ({})), /Missing raw target inventory/],
  ['missing frame identity', f => f.overrides.set('Page.getFrameTree', () => ({frameTree: {frame: {loaderId: 'loader'}}})), /frame\/loader identity/],
  ['missing loader identity', f => f.overrides.set('Page.getFrameTree', () => ({frameTree: {frame: {id: 'frame'}}})), /frame\/loader identity/],
  ['missing isolate', f => f.overrides.set('Runtime.getIsolateId', () => ({})), /isolate identity/],
];
for (const [name, mutate, pattern] of refusals) test(`retains refusal: ${name}`, async () => {
  const f = fixture(); mutate(f);
  await assert.rejects(f.run(), pattern);
  assert.equal(f.checkpoints.length, 1);
  assert.equal(f.checkpoints[0].status, 'failed');
  assert.equal(f.checkpoints[0].errors.length, 1);
  assert.ok(f.checkpoints[0].completed_ms >= f.checkpoints[0].started_ms);
});

for (const domain of ['target', 'session', 'isolate', 'loader']) test(`refuses mid-checkpoint ${domain} replacement`, async () => {
  const f = fixture(); let calls = 0;
  const method = domain === 'loader' ? 'Page.getFrameTree' : domain === 'isolate' ? 'Runtime.getIsolateId' : 'Target.getTargets';
  f.overrides.set(method, sessionId => {
    calls++;
    if (method === 'Target.getTargets') {
      if (calls > 1 && domain === 'target') {f.infos[0].targetId = 'replacement'; f.guard.coverage.sessions[0].targetId = 'replacement';}
      if (calls > 1 && domain === 'session') f.guard.coverage.sessions[0].sessionId = 'replacement-session';
      return {targetInfos: f.infos};
    }
    if (method === 'Runtime.getIsolateId') return {id: calls > 2 ? 'new-isolate' : sessionId + '-isolate'};
    return {frameTree: {frame: {id: sessionId + '-frame', loaderId: calls > 2 ? 'new-loader' : sessionId + '-loader'}}};
  });
  await assert.rejects(f.run(), /changed during checkpoint/);
});

test('failed command retains partial observations and original error', async () => {
  const f = fixture(); f.overrides.set('Page.getFrameTree', () => {throw Error('fixture frame lookup refused');});
  await assert.rejects(f.run(), /fixture frame lookup refused/);
  assert.equal(f.checkpoints[0].reads.length, 3);
  assert.match(f.checkpoints[0].reads.at(-1).error, /fixture frame lookup refused/);
  assert.equal(f.checkpoints[0].raw_targets_before.length, 3);
});

test('bounded timeout retains failure; late read cannot mutate its receipt', async () => {
  const f = fixture(); let release;
  f.overrides.set('Target.getTargets', () => new Promise(resolve => {release = resolve;}));
  await assert.rejects(f.run({timeoutMs: 25}), /deadline exceeded/);
  const before = structuredClone(f.checkpoints[0]);
  release({targetInfos: f.infos});
  await new Promise(resolve => setTimeout(resolve, 0));
  assert.deepEqual(f.checkpoints[0], before);
});

test('guard failure during read is not hidden by successful CDP reply', async () => {
  const f = fixture(); f.overrides.set('Target.getTargets', () => {f.guard.coverage.failed = true; return {targetInfos: f.infos};});
  await assert.rejects(f.run(), /Guard failed during/);
});

test('checkpoint rejects invalid deadline before any reads', async () => {
  const f = fixture(); await assert.rejects(f.run({timeoutMs: 10000}), /Invalid lifecycle checkpoint boundary/);
  assert.equal(f.calls.length, 0);
});

test('entry-time deadline also refuses excessive synchronous trace processing', async () => {
  const f = fixture(); let elapsed = 0;
  Object.defineProperty(f.lifecycleEvents[0], 'seq', {get() {elapsed = 5001; return 1;}});
  await assert.rejects(f.run({now: () => elapsed}), /deadline exceeded/);
  assert.equal(f.calls.length, 0);
});

for (const kind of ['command', 'response']) test(`successful CDP read with substituted ${kind} trace is refused`, async () => {
  const f = fixture(), original = f.guard.send;
  f.guard.send = async (...args) => {
    const result = await original(...args);
    const row = f.lifecycleEvents.find(e => e.kind === kind);
    row.kind = 'event'; row.method = 'Page.lifecycleEvent';
    return result;
  };
  await assert.rejects(f.run(), kind === 'command' ? /command missing/ : /response missing/);
});

test('mismatched response bytes cannot certify a successfully returned read', async () => {
  const f = fixture(), original = f.guard.send;
  f.guard.send = async (...args) => {
    const result = await original(...args);
    f.lifecycleEvents.at(-1).result = {targetInfos: []};
    return result;
  };
  await assert.rejects(f.run(), /response missing or mismatched/);
});

test('complete snapshots may change loader/isolate between separate native task endpoints', async () => {
  const f = fixture(); await f.run();
  f.overrides.set('Runtime.getIsolateId', sessionId => ({id: 'new-' + sessionId}));
  f.overrides.set('Page.getFrameTree', sessionId => ({frameTree: {frame: {id: 'new-' + sessionId, loaderId: 'new-loader-' + sessionId}}}));
  const final = await f.run({label: 'final'});
  assert.equal(final.status, 'recorded');
  assert.notEqual(final.targets[0].isolate_id, f.checkpoints[0].targets[0].isolate_id);
  assert.equal(final.attribution, 'not-verified');
});
