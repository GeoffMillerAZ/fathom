import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import test from 'node:test';

import {bounded, closeContext, launchManaged} from './lifecycle.mjs';

function fixture(overrides = {}) {
  const child = new EventEmitter();
  child.exitCode = null;
  child.signalCode = null;
  child.kills = [];
  child.kill = signal => {
    child.kills.push(signal);
    child.signalCode = signal;
    queueMicrotask(() => child.emit('exit', null, signal));
    return true;
  };
  const calls = [];
  const browser = {
    async close() { calls.push('browser.close'); },
    ...overrides.browser,
  };
  const server = {
    process() { return child; },
    wsEndpoint() { return 'ws://private.invalid/secret-endpoint'; },
    async close() {
      calls.push('server.close');
      child.exitCode = 0;
      child.emit('exit', 0, null);
    },
    async kill() {
      calls.push('server.kill');
      child.signalCode = 'SIGKILL';
      child.emit('exit', null, 'SIGKILL');
    },
    ...overrides.server,
  };
  const chromium = {
    async launchServer(options) { calls.push(['launchServer', options]); return server; },
    async connect(endpoint, options) { calls.push(['connect', endpoint, options]); return browser; },
    ...overrides.chromium,
  };
  return {browser, calls, child, chromium, server};
}

test('managed launch uses fixed deadlines and closes the exact owned server', async () => {
  const f = fixture();
  const managed = await launchManaged(f.chromium, {headless: false, executablePath: '/approved/chrome'});
  assert.equal(managed.browser, f.browser);
  assert.deepEqual(f.calls[0], ['launchServer', {headless: false, executablePath: '/approved/chrome', timeout: 15000}]);
  assert.deepEqual(f.calls[1], ['connect', 'ws://private.invalid/secret-endpoint', {timeout: 15000}]);
  await managed.close();
  await managed.close();
  assert.deepEqual(f.calls.slice(2), ['browser.close', 'server.close']);
  assert.deepEqual(f.child.kills, []);
});

test('connect refusal cleans up the owned server without disclosing endpoint', async () => {
  const f = fixture({chromium: {
    async connect() { throw new Error('ws://private.invalid/secret-endpoint'); },
  }});
  await assert.rejects(() => launchManaged(f.chromium, {headless: false}), error => {
    assert.equal(error.message, 'browser-lifecycle-connect-failed');
    assert(!error.message.includes('secret-endpoint'));
    return true;
  });
  assert.deepEqual(f.calls, [['launchServer', {headless: false, timeout: 15000}], 'server.close']);
  assert.equal(f.child.exitCode, 0);
});

test('launch refusal is sanitized', async () => {
  const f = fixture({chromium: {
    async launchServer() { throw new Error('private executable argv'); },
  }});
  await assert.rejects(() => launchManaged(f.chromium, {headless: false}), error => {
    assert.equal(error.message, 'browser-lifecycle-launch-failed');
    assert(!error.message.includes('private'));
    return true;
  });
});

test('launch timeout retains ownership and disposes a late BrowserServer', async t => {
  t.mock.timers.enable({apis: ['setTimeout']});
  const f = fixture();
  let resolveLaunch;
  f.chromium.launchServer = options => {
    f.calls.push(['launchServer', options]);
    return new Promise(resolve => { resolveLaunch = resolve; });
  };
  f.server.close = async () => { f.calls.push('server.close'); throw new Error('late close failed'); };
  f.server.kill = async () => { f.calls.push('server.kill'); throw new Error('late kill failed'); };
  const pending = launchManaged(f.chromium, {headless: false});
  await Promise.resolve();
  t.mock.timers.tick(15_000);
  await assert.rejects(pending, /browser-lifecycle-launch-failed/);
  resolveLaunch(f.server);
  await new Promise(setImmediate);
  assert.deepEqual(f.calls, [
    ['launchServer', {headless: false, timeout: 15000}],
    'server.close',
    'server.kill',
  ]);
  assert.deepEqual(f.child.kills, ['SIGKILL']);
  assert.equal(f.child.signalCode, 'SIGKILL');
});

test('connect refusal force-kills only its exact process when graceful close fails', async () => {
  const f = fixture({
    chromium: {async connect() { throw new Error('refused with private argv'); }},
    server: {async close() { f.calls.push('server.close'); throw new Error('close failed'); }},
  });
  await assert.rejects(() => launchManaged(f.chromium, {headless: false}), /browser-lifecycle-connect-failed/);
  assert.deepEqual(f.calls, [['launchServer', {headless: false, timeout: 15000}], 'server.close', 'server.kill']);
  assert.equal(f.child.signalCode, 'SIGKILL');
});

test('BrowserServer kill refusal falls back to its exact child handle', async () => {
  const f = fixture({
    chromium: {async connect() { throw new Error('refused'); }},
    server: {
      async close() { f.calls.push('server.close'); throw new Error('close failed'); },
      async kill() { f.calls.push('server.kill'); throw new Error('kill failed'); },
    },
  });
  await assert.rejects(() => launchManaged(f.chromium, {headless: false}), /browser-lifecycle-connect-failed/);
  assert.deepEqual(f.calls, [['launchServer', {headless: false, timeout: 15000}], 'server.close', 'server.kill']);
  assert.deepEqual(f.child.kills, ['SIGKILL']);
});

test('a graceful cleanup failure stays a failure after forced cleanup', async () => {
  const f = fixture({browser: {
    async close() { f.calls.push('browser.close'); throw new Error('private browser failure'); },
  }});
  const managed = await launchManaged(f.chromium, {headless: false});
  await assert.rejects(() => managed.close(), error => {
    assert.equal(error.message, 'browser-lifecycle-graceful-cleanup-failed');
    assert(!error.message.includes('private'));
    return true;
  });
  assert.deepEqual(f.calls.slice(2), ['browser.close', 'server.close']);
});

test('context close and generic deadline failures are bounded and sanitized', async () => {
  let closed = 0;
  await closeContext({async close() { closed++; }});
  assert.equal(closed, 1);
  await assert.rejects(() => closeContext(null), /browser-context-cleanup-failed/);
  await assert.rejects(() => bounded(new Promise(() => {}), 5, 'fixed-timeout'), error => {
    assert.equal(error.message, 'fixed-timeout');
    return true;
  });
  await assert.rejects(() => bounded(Promise.reject(new Error('secret endpoint')), 50, 'fixed-refusal'), error => {
    assert.equal(error.message, 'fixed-refusal');
    return true;
  });
});
