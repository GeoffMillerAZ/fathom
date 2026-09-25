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
    queueMicrotask(() => this.message(error ? {id: message.id, error} : {id: message.id, result: {}}));
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

async function fixture() {
  FakeWebSocket.instances.length = 0;
  globalThis.WebSocket = FakeWebSocket;
  const events = [], protocolEvents = [];
  const guard = await connectGuard(endpoint, {allowedOrigin: origin, events, protocolEvents, shaping: false});
  await guard.ready;
  return {guard, socket: FakeWebSocket.instances[0], events, protocolEvents};
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
