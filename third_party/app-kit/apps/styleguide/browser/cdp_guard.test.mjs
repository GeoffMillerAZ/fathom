import assert from 'node:assert/strict';
import test from 'node:test';

import {connectGuard} from './cdp_guard.mjs';

class FakeWebSocket {
  static OPEN = 1;
  static instances = [];

  constructor(url) {
    this.url = url;
    this.readyState = FakeWebSocket.OPEN;
    this.listeners = new Map();
    this.sent = [];
    this.failures = new Map();
    this.results = new Map();
    FakeWebSocket.instances.push(this);
    queueMicrotask(() => this.emit('open', {}));
  }

  addEventListener(name, listener, options = {}) {
    const wrapped = options.once ? event => { this.removeEventListener(name, wrapped); listener(event); } : listener;
    const listeners = this.listeners.get(name) ?? [];
    listeners.push(wrapped);
    this.listeners.set(name, listeners);
  }

  removeEventListener(name, listener) {
    this.listeners.set(name, (this.listeners.get(name) ?? []).filter(item => item !== listener));
  }

  emit(name, event) {
    for (const listener of [...(this.listeners.get(name) ?? [])]) listener(event);
  }

  send(raw) {
    const message = JSON.parse(raw);
    this.sent.push(message);
    const error = this.failures.get(message.method);
    const selected = this.results.get(message.method);
    const result = typeof selected === 'function' ? selected(message) : selected ?? {};
    queueMicrotask(() => this.message(error ? {id: message.id, error} : {id: message.id, result}));
  }

  message(message) {
    this.emit('message', {data: JSON.stringify(message)});
  }

  close(code = 1000, reason = '') {
    this.readyState = 3;
    queueMicrotask(() => this.emit('close', {code, reason}));
  }
}

const endpoint = 'ws://127.0.0.1:9222/devtools/browser/test';
const origin = 'http://127.0.0.1:8080';

async function fixture({lifecycle = false} = {}) {
  FakeWebSocket.instances.length = 0;
  globalThis.WebSocket = FakeWebSocket;
  const events = [], protocolEvents = [], lifecycleEvents = [];
  const guard = await connectGuard(endpoint, {allowedOrigin: origin, events, protocolEvents, shaping: false,
    ...(lifecycle ? {lifecycleEvents} : {})});
  await guard.ready;
  return {guard, socket: FakeWebSocket.instances[0], events, protocolEvents, lifecycleEvents};
}

async function settle(predicate) {
  for (let attempt = 0; attempt < 20; attempt++) {
    if (predicate()) return;
    await new Promise(resolve => setTimeout(resolve, 0));
  }
  assert.fail('guard did not settle');
}

test('retains ordered Target command, response, and complete event payloads', async () => {
  const {guard, socket, protocolEvents} = await fixture();
  const targetInfo = {
    targetId: 'target-1', type: 'browser_ui', title: 'Omnibox Popup',
    url: 'chrome://omnibox-popup.top-chrome/', attached: false,
    canAccessOpener: false, browserContextId: 'context-1',
  };
  socket.message({method: 'Target.targetCreated', params: {targetInfo}});

  assert.deepEqual(protocolEvents.map(item => [item.seq, item.kind, item.method]), [
    [1, 'command', 'Target.setAutoAttach'],
    [2, 'response', 'Target.setAutoAttach'],
    [3, 'event', 'Target.targetCreated'],
  ]);
  assert.deepEqual(protocolEvents[2].params.targetInfo, targetInfo);
  assert.equal(protocolEvents[2].params.targetInfo.browserContextId, 'context-1');
  assert.equal(protocolEvents[2].params.targetInfo.title, 'Omnibox Popup');
  await guard.close();
});

test('retains full failed Target command responses', async () => {
  const {guard, socket, protocolEvents} = await fixture();
  const error = {code: -32000, message: 'Target lookup failed', data: 'retained detail'};
  socket.failures.set('Target.getTargets', error);

  await assert.rejects(guard.send('Target.getTargets', {filter: [{type: 'page'}]}), /Target lookup failed/);
  const command = protocolEvents.find(item => item.kind === 'command' && item.method === 'Target.getTargets');
  const response = protocolEvents.find(item => item.kind === 'response' && item.id === command.id);
  assert.deepEqual(command.params, {filter: [{type: 'page'}]});
  assert.deepEqual(response.error, error);
  assert.ok(response.received_at);
  await guard.close();
});

test('unknown target types still fail closed and are never resumed', async () => {
  const {guard, socket, protocolEvents} = await fixture();
  socket.message({
    method: 'Target.attachedToTarget',
    params: {
      sessionId: 'unknown-session', waitingForDebugger: true,
      targetInfo: {targetId: 'unknown-target', type: 'other', title: 'about:blank', url: 'about:blank', browserContextId: 'default-context'},
    },
  });
  await new Promise(resolve => setTimeout(resolve, 0));

  const target = guard.coverage.sessions.find(item => item.sessionId === 'unknown-session');
  assert.equal(guard.coverage.failed, true);
  assert.match(target.error, /Unsupported auto-attached target type: other/);
  assert.equal(target.appliedPolicy, false);
  assert.equal(target.resumed, false);
  assert.equal(socket.sent.some(item => item.sessionId === 'unknown-session' && item.method === 'Runtime.runIfWaitingForDebugger'), false);
  assert.equal(protocolEvents.find(item => item.method === 'Target.attachedToTarget').params.targetInfo.browserContextId, 'default-context');

  socket.message({method: 'Target.detachedFromTarget', params: {sessionId: 'unknown-session', targetId: 'unknown-target'}});
  await guard.close();
});

test('lifecycle option requires a distinct caller-owned initially empty array', async () => {
  const invalid = [null, {}, ['not-empty']];
  for (const lifecycleEvents of invalid) {
    await assert.rejects(connectGuard(endpoint, {allowedOrigin: origin, events: [], lifecycleEvents}), /lifecycleEvents/);
  }
  const shared = [];
  await assert.rejects(connectGuard(endpoint, {allowedOrigin: origin, events: [], protocolEvents: shared, lifecycleEvents: shared}), /distinct/);
  await assert.rejects(connectGuard(endpoint, {allowedOrigin: origin, events: shared, lifecycleEvents: shared}), /distinct/);
});

test('default mode sends no lifecycle instrumentation and preserves Target-only trace', async () => {
  const {guard, socket, protocolEvents, lifecycleEvents} = await fixture();
  assert.equal(guard.lifecycleEvents, undefined);
  assert.deepEqual(socket.sent.map(item => item.method), ['Target.setAutoAttach']);
  assert.deepEqual(protocolEvents.map(item => item.method), ['Target.setAutoAttach', 'Target.setAutoAttach']);
  assert.deepEqual(lifecycleEvents, []);
  socket.message({method: 'Target.attachedToTarget', params: {sessionId: 'page-session', waitingForDebugger: true,
    targetInfo: {targetId: 'page-target', type: 'page', title: 'guide', url: origin + '/theme-lab', browserContextId: 'context-1'}}});
  await settle(() => guard.coverage.sessions.some(item => item.sessionId === 'page-session' && item.resumed));
  const pageMethods = socket.sent.filter(item => item.sessionId === 'page-session').map(item => item.method);
  assert.equal(pageMethods.includes('Page.enable'), false);
  assert.equal(pageMethods.includes('Runtime.enable'), false);
  await guard.close();
});

test('opt-in records ordered raw page iframe lifecycle and stale identities', async () => {
  const {guard, socket, lifecycleEvents} = await fixture({lifecycle: true});
  assert.equal(guard.lifecycleEvents, lifecycleEvents);
  assert.deepEqual(socket.sent.filter(item => !item.sessionId).map(item => item.method),
    ['Target.setDiscoverTargets', 'Target.setAutoAttach']);
  socket.results.set('Page.getFrameTree', message => ({frameTree: {frame: {id: `frame-${message.sessionId}`, loaderId: `loader-${message.sessionId}`, url: origin + '/theme-lab'}}}));
  socket.results.set('Runtime.getIsolateId', message => ({id: `isolate-${message.sessionId}`}));
  for (const [sessionId, targetId, type, url] of [
    ['page-session', 'page-target', 'page', origin + '/theme-lab'],
    ['frame-session', 'frame-target', 'iframe', origin + '/theme-lab/preview'],
  ]) socket.message({method: 'Target.attachedToTarget', params: {sessionId, waitingForDebugger: true,
    targetInfo: {targetId, type, title: type, url, browserContextId: 'context-1'}}});
  await settle(() => guard.coverage.sessions.filter(item => ['page', 'iframe'].includes(item.type)).every(item => item.resumed));
  socket.message({sessionId: 'frame-session', method: 'Page.frameNavigated', params: {frame: {id: 'stale-frame', parentId: 'old-parent', loaderId: 'stale-loader', url: origin + '/old'}}});
  socket.message({sessionId: 'frame-session', method: 'Page.lifecycleEvent', params: {frameId: 'stale-frame', loaderId: 'stale-loader', name: 'networkIdle', timestamp: 42.5}});
  socket.message({sessionId: 'frame-session', method: 'Runtime.executionContextCreated', params: {context: {id: 77, uniqueId: 'stale-context', auxData: {frameId: 'stale-frame'}}}});
  socket.message({sessionId: 'frame-session', method: 'Runtime.executionContextsCleared', params: {}});
  socket.message({method: 'Target.targetDestroyed', params: {targetId: 'unknown-destroyed-target'}});
  socket.message({method: 'Target.detachedFromTarget', params: {sessionId: 'frame-session', targetId: 'frame-target'}});
  await settle(() => lifecycleEvents.some(item => item.method === 'Runtime.executionContextCreated'));

  assert.deepEqual(lifecycleEvents.map(item => item.seq), lifecycleEvents.map((_, index) => index + 1));
  assert.ok(lifecycleEvents.every((item, index) => Number.isFinite(item.monotonic_ms) && (index === 0 || item.monotonic_ms >= lifecycleEvents[index - 1].monotonic_ms)));
  const pageMethods = socket.sent.filter(item => item.sessionId === 'page-session').map(item => item.method);
  assert.deepEqual(pageMethods, ['Target.setAutoAttach', 'Network.enable', 'Network.setBlockedURLs', 'Page.enable', 'Runtime.enable', 'Page.setLifecycleEventsEnabled', 'Page.getFrameTree', 'Runtime.getIsolateId', 'Runtime.runIfWaitingForDebugger']);
  const frameTree = lifecycleEvents.find(item => item.kind === 'response' && item.method === 'Page.getFrameTree' && item.sessionId === 'frame-session');
  assert.equal(frameTree.result.frameTree.frame.loaderId, 'loader-frame-session');
  const isolate = lifecycleEvents.find(item => item.kind === 'response' && item.method === 'Runtime.getIsolateId' && item.sessionId === 'frame-session');
  assert.equal(isolate.result.id, 'isolate-frame-session');
  assert.equal(lifecycleEvents.find(item => item.method === 'Page.frameNavigated').params.frame.loaderId, 'stale-loader');
  assert.equal(lifecycleEvents.find(item => item.method === 'Page.lifecycleEvent').params.timestamp, 42.5);
  assert.equal(lifecycleEvents.find(item => item.method === 'Runtime.executionContextCreated').params.context.uniqueId, 'stale-context');
  assert.deepEqual(lifecycleEvents.filter(item => ['Target.targetDestroyed', 'Target.detachedFromTarget', 'Runtime.executionContextsCleared'].includes(item.method)).map(item => item.method),
    ['Runtime.executionContextsCleared', 'Target.targetDestroyed', 'Target.detachedFromTarget']);
  assert.equal(lifecycleEvents.find(item => item.method === 'Target.targetDestroyed').params.targetId, 'unknown-destroyed-target');
  await guard.send('HeapProfiler.collectGarbage', {}, 'page-session');
  assert.ok(lifecycleEvents.some(item => item.kind === 'response' && item.method === 'HeapProfiler.collectGarbage'));
  await guard.close();
});

test('failed lifecycle command is retained and target is never resumed', async () => {
  const {guard, socket, lifecycleEvents} = await fixture({lifecycle: true});
  socket.failures.set('Page.getFrameTree', {code: -32000, message: 'frame tree failed', data: 'raw failure'});
  socket.message({method: 'Target.attachedToTarget', params: {sessionId: 'page-session', waitingForDebugger: true,
    targetInfo: {targetId: 'page-target', type: 'page', title: 'guide', url: origin + '/theme-lab', browserContextId: 'context-1'}}});
  await settle(() => guard.coverage.failed);
  const target = guard.coverage.sessions.find(item => item.sessionId === 'page-session');
  assert.equal(target.resumed, false);
  assert.equal(socket.sent.some(item => item.sessionId === 'page-session' && item.method === 'Runtime.runIfWaitingForDebugger'), false);
  const failed = lifecycleEvents.find(item => item.kind === 'response' && item.method === 'Page.getFrameTree');
  assert.deepEqual(failed.error, {code: -32000, message: 'frame tree failed', data: 'raw failure'});
  socket.message({method: 'Target.detachedFromTarget', params: {sessionId: 'page-session', targetId: 'page-target'}});
  await guard.close();
});

test('malformed attachments are retained, fail closed and never send session commands', async () => {
  const valid = {sessionId: 'session', waitingForDebugger: true, targetInfo: {targetId: 'target', type: 'page', url: origin + '/theme-lab'}};
  const malformed = [null, {}, {...valid, sessionId: ''}, {...valid, targetInfo: null}, {sessionId: valid.sessionId, targetInfo: valid.targetInfo}, {...valid, targetInfo: {...valid.targetInfo, targetId: ''}}];
  for (const params of malformed) {
    const {guard, socket, lifecycleEvents} = await fixture({lifecycle: true});
    const priorCommands = socket.sent.length;
    socket.message({method: 'Target.attachedToTarget', params});
    await settle(() => guard.coverage.failed);
    assert.equal(socket.sent.length, priorCommands);
    assert.equal(guard.coverage.sessions.length, 0);
    assert.equal(guard.coverage.unaccountedAttachments.length, 1);
    assert.deepEqual(guard.coverage.unaccountedAttachments[0].params, params);
    assert.match(guard.coverage.errors[0].error, /Malformed/);
    assert.deepEqual(lifecycleEvents.at(-1).params, params);
    await assert.rejects(guard.close(), /unaccounted auto-attachments/);
    socket.close(); // Simulate the owner terminating Chrome, not a guard detach.
    await settle(() => guard.coverage.closed);
  }
});

for (const duplicate of ['session', 'target']) test(`duplicate ${duplicate} identity cannot overwrite or resume pending attachments`, async () => {
  const {guard, socket} = await fixture({lifecycle: true});
  const first = {sessionId: 'session', waitingForDebugger: true, targetInfo: {targetId: 'target', type: 'page', url: origin + '/theme-lab'}};
  socket.message({method: 'Target.attachedToTarget', params: first});
  socket.message({method: 'Target.attachedToTarget', params: {...first, sessionId: duplicate === 'session' ? 'session' : 'session-2', targetInfo: {...first.targetInfo, targetId: duplicate === 'target' ? 'target' : 'target-2'}}});
  await settle(() => guard.coverage.failed && guard.coverage.sessions[0]?.error);
  assert.equal(guard.coverage.sessions.length, 1);
  assert.equal(guard.coverage.sessions[0].targetId, 'target');
  assert.equal(guard.coverage.unaccountedAttachments.length, 1);
  assert.equal(socket.sent.some(item => item.method === 'Runtime.runIfWaitingForDebugger'), false);
  assert.match(guard.coverage.unaccountedAttachments[0].error, /Duplicate/);
  await assert.rejects(guard.close(), /unaccounted auto-attachments/);
  socket.close();
  await settle(() => guard.coverage.closed);
});
