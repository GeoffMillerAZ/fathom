import assert from 'node:assert/strict';
import test from 'node:test';

import {connectGuard} from './cdp_guard.mjs';
import {captureGuideLifecycle} from './lifecycle.mjs';
import {joinGuideLifecycle} from './lifecycle-check.mjs';

const origin = 'http://127.0.0.1:18080';
const contextId = 'owned-guide-context';
const targetInfos = [
  {targetId: 'guide', type: 'page', url: origin + '/theme-lab', browserContextId: contextId},
  {targetId: 'preview', type: 'iframe', url: origin + '/theme-lab/preview?specimen=dashboard&scheme=light', browserContextId: contextId},
  {targetId: 'internal', type: 'browser_ui', url: 'chrome://internal'},
];

class FakeWebSocket {
  static OPEN = 1;
  static instances = [];

  constructor() {
    this.readyState = FakeWebSocket.OPEN;
    this.listeners = new Map();
    FakeWebSocket.instances.push(this);
    queueMicrotask(() => this.emit('open', {}));
  }

  addEventListener(name, listener, options = {}) {
    const wrapped = options.once ? event => { this.removeEventListener(name, wrapped); listener(event); } : listener;
    this.listeners.set(name, [...(this.listeners.get(name) ?? []), wrapped]);
  }

  removeEventListener(name, listener) {
    this.listeners.set(name, (this.listeners.get(name) ?? []).filter(item => item !== listener));
  }

  emit(name, event) {
    for (const listener of [...(this.listeners.get(name) ?? [])]) listener(event);
  }

  send(raw) {
    const command = JSON.parse(raw);
    let result = {};
    const frame = {
      id: command.sessionId + '-frame', loaderId: command.sessionId + '-loader',
      url: origin + (command.sessionId === 'preview-session' ? '/theme-lab/preview?specimen=dashboard&scheme=light' : '/theme-lab'),
      mimeType: 'text/html', securityOrigin: origin,
      ...(command.sessionId === 'preview-session' ? {parentId: 'guide-session-frame'} : {}),
    };
    if (command.method === 'Target.setDiscoverTargets') {
      for (const targetInfo of targetInfos) queueMicrotask(() => this.message({
        method: 'Target.targetCreated', params: {targetInfo},
      }));
    }
    if (command.method === 'Page.enable') queueMicrotask(() => this.message({
      method: 'Page.frameNavigated', sessionId: command.sessionId, params: {frame, type: 'Navigation'},
    }));
    if (command.method === 'Runtime.enable') queueMicrotask(() => this.message({
      method: 'Runtime.executionContextCreated', sessionId: command.sessionId,
      params: {context: {id: 1, uniqueId: command.sessionId + '-context',
        origin: command.sessionId === 'preview-session' ? '://' : origin, name: '',
        auxData: {isDefault: true, type: 'default', frameId: frame.id}}},
    }));
    if (command.method === 'Target.getTargets') result = {targetInfos};
    else if (command.method === 'Runtime.getIsolateId') result = {id: command.sessionId + '-isolate'};
    else if (command.method === 'Page.getFrameTree') result = {frameTree: {frame}};
    queueMicrotask(() => this.message({id: command.id, result}));
  }

  message(message) {
    this.emit('message', {data: JSON.stringify(message)});
  }

  close(code = 1000, reason = '') {
    this.readyState = 3;
    queueMicrotask(() => this.emit('close', {code, reason}));
  }
}

async function settle(predicate) {
  for (let attempt = 0; attempt < 50; attempt++) {
    if (predicate()) return;
    await new Promise(resolve => setTimeout(resolve, 0));
  }
  assert.fail('guard did not settle');
}

async function guardedGuide(lifecycle) {
  globalThis.WebSocket = FakeWebSocket;
  const events = [], protocolEvents = [], lifecycleEvents = [];
  const guard = await connectGuard('ws://127.0.0.1:9222/devtools/browser/integration', {
    allowedOrigin: origin, events, protocolEvents, shaping: false,
    ...(lifecycle ? {lifecycleEvents} : {}),
  });
  await guard.ready;
  const socket = FakeWebSocket.instances.at(-1);
  for (const [sessionId, targetInfo] of [
    ['guide-session', targetInfos[0]],
    ['preview-session', targetInfos[1]],
  ]) socket.message({method: 'Target.attachedToTarget', params: {sessionId, waitingForDebugger: true, targetInfo}});
  await settle(() => guard.coverage.sessions.length === 2 && guard.coverage.sessions.every(session => session.resumed));
  return {guard, lifecycleEvents};
}

test('guard lifecycle trace drives one bounded guide checkpoint without promoting acceptance', async () => {
  FakeWebSocket.instances.length = 0;
  const {guard, lifecycleEvents} = await guardedGuide(true);
  const checkpoints = [];
  const result = await captureGuideLifecycle({
    guard, lifecycleEvents, contextId, label: 'synthetic-guide', checkpoints,
  });

  assert.equal(result.status, 'recorded');
  assert.equal(result.reads.length, 10);
  assert.equal(result.targets.length, 2);
  const page = result.targets.find(target => target.type === 'page');
  const iframe = result.targets.find(target => target.type === 'iframe');
  assert.equal(page.browser_context_id, contextId);
  assert.equal(page.frame_id, 'guide-session-frame');
  assert.equal(page.loader_id, 'guide-session-loader');
  assert.equal(page.isolate_id, 'guide-session-isolate');
  assert.equal(iframe.parent_frame_id, 'guide-session-frame');
  assert.equal(new Set(result.reads.map(row => row.command_id)).size, 10);
  for (const row of result.reads) {
    const command = lifecycleEvents.find(event => event.seq === row.command_seq);
    const response = lifecycleEvents.find(event => event.seq === row.response_seq);
    assert.equal(command.kind, 'command');
    assert.equal(response.kind, 'response');
    assert.equal(command.id, row.command_id);
    assert.equal(response.id, row.command_id);
    assert.ok(row.command_seq < row.response_seq);
  }
  assert.equal(result.attribution, 'not-verified');
  assert.equal(result.memory_complete, false);
  assert.equal(result.full_contract_complete, false);
  await guard.close();

  const missing = await guardedGuide(false);
  const refused = [];
  await assert.rejects(captureGuideLifecycle({
    guard: missing.guard, lifecycleEvents: undefined, contextId, label: 'missing-trace', checkpoints: refused,
  }), /Missing opt-in lifecycle trace/);
  assert.equal(refused[0].status, 'failed');
  assert.equal(refused[0].full_contract_complete, false);
  await missing.guard.close();
});

test('independent join checks raw events from real guard and checkpoint modules', async () => {
  const {guard, lifecycleEvents} = await guardedGuide(true);
  try {
    const checkpoints = [];
    for (const label of ['baseline', 'final']) await captureGuideLifecycle({
      guard, lifecycleEvents, contextId, label, checkpoints,
    });
    const inputs = {events: lifecycleEvents, checkpoints, contextId, origin};
    const original = JSON.stringify(inputs);
    const result = joinGuideLifecycle(inputs);
    assert.equal(result.evidence_valid, true, JSON.stringify(result));
    assert.equal(result.live_membership_complete, true, JSON.stringify(result));
    assert.equal(result.attribution, 'not-verified');
    assert.equal(result.memory_complete, false);
    assert.equal(result.full_contract_complete, false);
    assert.equal(JSON.stringify(inputs), original, 'checker must not mutate raw evidence');

    const corrupt = structuredClone(inputs);
    const isolateRead = corrupt.checkpoints[0].reads.find(row => row.method === 'Runtime.getIsolateId');
    corrupt.events.find(event => event.id === isolateRead.command_id && event.kind === 'response').result.id = 'forged-isolate';
    const refused = joinGuideLifecycle(corrupt);
    assert.equal(refused.evidence_valid && refused.live_membership_complete, false,
      'inconsistent raw isolate lineage cannot qualify via producer summaries');
    assert.equal(refused.memory_complete, false);

    const unrelatedContext = structuredClone(inputs);
    unrelatedContext.events.find(event => event.method === 'Runtime.executionContextCreated' &&
      event.sessionId === 'preview-session').params.context.auxData.frameId = 'unobserved-frame';
    const unqualified = joinGuideLifecycle(unrelatedContext);
    assert.equal(unqualified.evidence_valid && unqualified.live_membership_complete, false,
      'matching URLs and producer targets do not establish context/frame ownership');
    assert.equal(unqualified.attribution, 'not-verified');
  } finally { await guard.close(); }
});

test('independent join refuses replayed checkpoints and unaccounted live contexts', async () => {
  const {guard, lifecycleEvents} = await guardedGuide(true);
  try {
    const checkpoints = [];
    for (const label of ['baseline', 'final']) await captureGuideLifecycle({
      guard, lifecycleEvents, contextId, label, checkpoints,
    });
    const inputs = {events: lifecycleEvents, checkpoints, contextId, origin};
    const cases = [
      ['replayed checkpoint', value => { value.checkpoints[1] = structuredClone(value.checkpoints[0]); }],
      ['reversed checkpoints', value => { value.checkpoints.reverse(); }],
      ['policy before attachment', value => {
        const attachment = value.events.find(event => event.method === 'Target.attachedToTarget');
        const session = attachment.params.sessionId;
        const resume = value.events.find(event => event.kind === 'command' && event.method === 'Runtime.runIfWaitingForDebugger' && event.sessionId === session);
        const setup = value.events.filter(event => ['command', 'response'].includes(event.kind) &&
          event.sessionId === session && event.seq > attachment.seq && event.seq < resume.seq);
        value.events = value.events.filter(event => !setup.includes(event));
        value.events.splice(value.events.indexOf(attachment), 0, ...setup);
        value.events.forEach((event, i) => { event.seq = i + 1; event.monotonic_ms = i + 1; });
      }],
      ['duplicate global context ID', value => {
        const contexts = value.events.filter(event => event.method === 'Runtime.executionContextCreated');
        contexts[1].params.context.uniqueId = contexts[0].params.context.uniqueId;
      }],
      ['unaccounted child frame', value => {
        for (const checkpoint of value.checkpoints) for (const read of checkpoint.reads) {
          if (read.method !== 'Page.getFrameTree' || read.session_id !== 'guide-session') continue;
          read.result.frameTree.childFrames = [{frame: {id: 'unaccounted-child', parentId: 'guide-session-frame',
            loaderId: 'unaccounted-loader', url: origin + '/theme-lab/preview'}}];
          value.events.find(event => event.kind === 'response' && event.id === read.command_id).result = structuredClone(read.result);
        }
      }],
      ['unaccounted context', value => {
        const index = value.events.findIndex(event => event.method === 'Runtime.executionContextCreated');
        const extra = structuredClone(value.events[index]);
        Object.assign(extra.params.context, {id: 99, uniqueId: 'unaccounted-context'});
        extra.params.context.auxData.frameId = 'unaccounted-frame';
        value.events.splice(index + 1, 0, extra);
        value.events.forEach((event, i) => { event.seq = i + 1; event.monotonic_ms = i + 1; });
        for (const checkpoint of value.checkpoints) {
          checkpoint.trace_start_seq++; checkpoint.trace_end_seq++;
          for (const read of checkpoint.reads) { read.command_seq++; read.response_seq++; }
        }
      }],
    ];
    const failures = [];
    for (const [name, mutate] of cases) {
      const value = structuredClone(inputs); mutate(value);
      const result = joinGuideLifecycle(value);
      if (result.evidence_valid) failures.push(name);
    }
    assert.deepEqual(failures, [], 'independent structural checker must refuse each ambiguous trace');
  } finally { await guard.close(); }
});
