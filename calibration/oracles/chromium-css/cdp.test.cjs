'use strict';

const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const { PassThrough } = require('node:stream');
const { mkdtempSync, rmSync } = require('node:fs');
const { tmpdir } = require('node:os');
const path = require('node:path');
const test = require('node:test');
const { CDPConnection, startBrowser } = require('./cdp.cjs');

function frames(buffer) {
  return buffer.toString('utf8').split('\0').filter(Boolean).map(JSON.parse);
}

test('split and coalesced pipe frames correlate replies around events and expose errors', async () => {
  const toBrowser = new PassThrough();
  const fromBrowser = new PassThrough();
  const conn = new CDPConnection({ write: toBrowser, read: fromBrowser, timeoutMs: 1000 });
  const sent = [];
  const events = [];
  toBrowser.on('data', chunk => sent.push(chunk));
  conn.on('event', event => events.push(event));
  const first = conn.send('Target.createTarget', { url: 'about:blank' });
  const second = conn.send('Runtime.evaluate', { expression: '2' }, 'session-1');
  const [a, b] = frames(Buffer.concat(sent));
  assert.equal(a.id + 1, b.id);
  assert.equal(b.sessionId, 'session-1');

  const payload = Buffer.from(JSON.stringify({ method: 'Runtime.consoleAPICalled', sessionId: 'session-1', params: { type: 'log' } }) + '\0' +
    JSON.stringify({ id: b.id, error: { code: -32000, message: 'evaluation blocked' } }) + '\0' +
    JSON.stringify({ id: a.id, result: { targetId: 'target-1' } }) + '\0');
  fromBrowser.write(payload.subarray(0, 7));
  fromBrowser.write(payload.subarray(7, 23));
  fromBrowser.write(payload.subarray(23));
  assert.deepEqual(await first, { targetId: 'target-1' });
  await assert.rejects(second, /Runtime\.evaluate.*evaluation blocked/);
  assert.equal(events.length, 1);
  assert.equal(events[0].sessionId, 'session-1');
  await conn.close();
});

test('browser launch uses pipe only; timeout and close reject pending work and reap child', async () => {
  const profileDir = mkdtempSync(path.join(tmpdir(), 'cdp-test-'));
  const stderr = new PassThrough();
  const toBrowser = new PassThrough();
  const fromBrowser = new PassThrough();
  const child = new EventEmitter();
  child.stdio = [null, null, stderr, toBrowser, fromBrowser];
  child.exitCode = null;
  child.kill = signal => {
    child.exitCode = 0;
    queueMicrotask(() => child.emit('exit', 0, signal));
    return true;
  };
  let launch;
  try {
    const browser = startBrowser('/fake/headless-shell', {
      profileDir, timeoutMs: 20,
      spawnImpl: (binary, args, options) => {
        launch = { binary, args, options };
        return child;
      },
    });
    assert.ok(launch.args.includes('--remote-debugging-pipe'));
    assert.ok(launch.args.includes('--disable-background-networking'));
    assert.ok(launch.args.includes('--disable-component-update'));
    assert.ok(launch.args.includes('--disable-sync'));
    assert.ok(launch.args.includes('--no-first-run'));
    assert.ok(launch.args.includes('about:blank'));
    assert.ok(launch.args.some(arg => arg === `--user-data-dir=${profileDir}`));
    assert.ok(!launch.args.some(arg => arg.includes('--remote-debugging-port')));
    assert.deepEqual(launch.options.stdio, ['ignore', 'ignore', 'pipe', 'pipe', 'pipe']);
    assert.equal(stderr.listenerCount('data') > 0 || stderr.readableFlowing, true, 'stderr must be drained');
    const drained = await Promise.race([
      new Promise(resolve => stderr.write(Buffer.alloc(1024 * 1024, 120), () => resolve(true))),
      new Promise(resolve => setTimeout(() => resolve(false), 100)),
    ]);
    assert.equal(drained, true, 'diagnostics must not backpressure the browser');
    await assert.rejects(browser.send('Never.replies'), /timed out/i);
    const pending = browser.send('Also.pending');
    await browser.close();
    await assert.rejects(pending, /closed/i);
    assert.equal(child.exitCode, 0);
  } finally {
    rmSync(profileDir, { recursive: true, force: true });
  }
});

test('close waits for exit after SIGKILL rather than reporting cleanup when kill is sent', async () => {
  const toBrowser = new PassThrough();
  const fromBrowser = new PassThrough();
  const child = new EventEmitter();
  child.exitCode = null;
  child.signalCode = null;
  const signals = [];
  child.kill = signal => { signals.push(signal); return true; };
  const conn = new CDPConnection({ write: toBrowser, read: fromBrowser, child, timeoutMs: 100 });
  let settled = false;
  const closing = conn.close({ graceMs: 5, killWaitMs: 100 }).then(() => { settled = true; });
  await new Promise(resolve => setTimeout(resolve, 20));
  assert.deepEqual(signals, ['SIGTERM', 'SIGKILL']);
  assert.equal(settled, false, 'SIGKILL is not child exit');
  child.signalCode = 'SIGKILL';
  child.emit('exit', null, 'SIGKILL');
  await closing;
  assert.equal(settled, true);
});

test('close reports failure if child never exits after SIGKILL', async () => {
  const child = new EventEmitter();
  child.exitCode = null;
  child.signalCode = null;
  const signals = [];
  child.kill = signal => { signals.push(signal); return true; };
  const conn = new CDPConnection({ write: new PassThrough(), read: new PassThrough(), child });
  await assert.rejects(conn.close({ graceMs: 5, killWaitMs: 10 }), /did not exit after SIGKILL/);
  assert.deepEqual(signals, ['SIGTERM', 'SIGKILL']);
});

test('target attachment is flattened and a child exit rejects an in-flight request', async () => {
  const toBrowser = new PassThrough();
  const fromBrowser = new PassThrough();
  const child = new EventEmitter();
  child.exitCode = null;
  child.signalCode = null;
  const sent = [];
  toBrowser.on('data', chunk => sent.push(chunk));
  const conn = new CDPConnection({ write: toBrowser, read: fromBrowser, child, timeoutMs: 1000 });
  const session = conn.createTargetSession();
  let request = frames(Buffer.concat(sent))[0];
  assert.equal(request.method, 'Target.createTarget');
  fromBrowser.write(JSON.stringify({ id: request.id, result: { targetId: 't-1' } }) + '\0');
  await new Promise(resolve => setImmediate(resolve));
  request = frames(Buffer.concat(sent))[1];
  assert.equal(request.method, 'Target.attachToTarget');
  assert.deepEqual(request.params, { targetId: 't-1', flatten: true });
  fromBrowser.write(JSON.stringify({ id: request.id, result: { sessionId: 's-1' } }) + '\0');
  const attached = await session;
  assert.equal(attached.sessionId, 's-1');
  const pending = attached.send('CSS.enable');
  assert.equal(frames(Buffer.concat(sent))[2].sessionId, 's-1');
  child.exitCode = 9;
  child.emit('exit', 9, null);
  await assert.rejects(pending, /browser exited: code=9/);
  await conn.close();
});
