import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {
  lstatSync,
  mkdirSync,
  mkdtempSync,
  realpathSync,
  rmSync,
  symlinkSync,
} from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {
  browserInitOptions,
  consoleErrorClass,
  consoleMultisetDifference,
  requestClass,
  writeViewportCapture,
} from './workflow_browser.mjs';

const origin = 'http://127.0.0.1:18780';
const token = 'A'.repeat(43);
const launch = `${origin}/auth/launch?token=${token}`;

test('browser init options default headless and admit only canonical existing capture directories', t => {
  const root = realpathSync(mkdtempSync(path.join(os.tmpdir(), 'appkit-workflow-browser-')));
  t.after(() => rmSync(root, {recursive: true, force: true}));
  const captures = path.join(root, 'captures');
  mkdirSync(captures);
  assert.deepEqual(browserInitOptions({}), {headed: false, captureDir: ''});
  assert.deepEqual(browserInitOptions({headed: true, captureDir: captures}), {
    headed: true,
    captureDir: captures,
  });
  for (const headed of [0, 'true', null]) {
    assert.throws(() => browserInitOptions({headed}), /state/);
  }
  assert.throws(() => browserInitOptions({captureDir: 'captures'}), /state/);
  assert.throws(() => browserInitOptions({captureDir: path.join(root, 'missing')}), /state/);
  const linked = path.join(root, 'linked');
  symlinkSync(captures, linked);
  assert.throws(() => browserInitOptions({captureDir: linked}), /state/);
});

test('viewport capture is bounded, traversal-safe, hashed, and never overwrites', t => {
  const root = realpathSync(mkdtempSync(path.join(os.tmpdir(), 'appkit-workflow-capture-')));
  t.after(() => rmSync(root, {recursive: true, force: true}));
  const png = Buffer.from('synthetic viewport png');
  const result = JSON.parse(writeViewportCapture(root, 'handoff', png, {width: 1440, height: 1000}));
  assert.deepEqual(result, {
    file: 'handoff.png',
    sha256: createHash('sha256').update(png).digest('hex'),
    width: 1440,
    height: 1000,
  });
  assert.equal(lstatSync(path.join(root, result.file)).mode & 0o777, 0o600);
  assert.throws(() => writeViewportCapture(root, 'handoff', png, {width: 1, height: 1}));
  assert.throws(() => writeViewportCapture(root, '../escape', png, {width: 1, height: 1}), /state/);
  assert.throws(() => writeViewportCapture(root, 'too-large', Buffer.alloc(4 * 1024 * 1024 + 1), {width: 1, height: 1}), /state/);
});

test('request classifier admits only exact synthetic Gallery transport', () => {
  const launches = new Set([launch]);
  assert.equal(requestClass(origin, 'GET', launch, launches), 'launch');
  for (const path of ['/shell', '/shell/home', '/shell/profile', '/shell/activity',
    '/shell/settings', '/shell/help', '/shell/about', '/shell/events',
    '/static/app.css', '/static/shell-demo.js', '/static/shell-live-entry.js',
    '/static/shell-live.js', '/static/datastar-v1.0.3.js']) {
    assert.equal(requestClass(origin, 'GET', origin + path, launches), 'read', path);
    assert.equal(requestClass(origin, 'HEAD', origin + path, launches), 'read', `HEAD ${path}`);
  }
  assert.equal(requestClass(origin, 'POST', `${origin}/shell/action`, launches), 'action');
  for (const [method, url] of [
    ['GET', `${origin}/favicon.ico`],
    ['GET', `${origin}/shell/profile?token=secret`],
    ['POST', `${origin}/shell/profile`],
    ['PUT', `${origin}/shell/action`],
    ['GET', `http://127.0.0.1:18781/shell/profile`],
    ['GET', 'https://example.com/static/app.css'],
  ]) assert.equal(requestClass(origin, method, url, launches), 'blocked', `${method} ${url}`);
});

test('console classifier accepts only the deliberate workflow response multiset', () => {
  const observed = new Map();
  for (const [status, reason, path, key] of [
    [422, 'Unprocessable Entity', '/shell/action', 'POST /shell/action 422'],
    [409, 'Conflict', '/shell/action', 'POST /shell/action 409'],
    [503, 'Service Unavailable', '/shell/action', 'POST /shell/action 503'],
    [401, 'Unauthorized', '/shell/profile', 'GET /shell/profile 401'],
  ]) {
    const text = `Failed to load resource: the server responded with a status of ${status} (${reason})`;
    assert.equal(consoleErrorClass(origin, text, origin + path), key);
    observed.set(key, 1);
  }
  assert.equal(consoleMultisetDifference(observed), 0);
  observed.set('POST /shell/action 422', 2);
  assert.equal(consoleMultisetDifference(observed), 1, 'duplicates fail the exact multiset');
  observed.delete('POST /shell/action 503');
  assert.equal(consoleMultisetDifference(observed), 2, 'missing and duplicate observations both count');
});

test('console classifier refuses wrong status, reason, path, origin, or message shape', () => {
  for (const [text, url] of [
    ['Failed to load resource: the server responded with a status of 500 (Internal Server Error)', `${origin}/shell/action`],
    ['Failed to load resource: the server responded with a status of 422 (wrong)', `${origin}/shell/action`],
    ['Failed to load resource: the server responded with a status of 401 (Unauthorized)', `${origin}/shell/action`],
    ['Failed to load resource: the server responded with a status of 409 (Conflict)', `${origin}/shell/profile`],
    ['Failed to load resource: the server responded with a status of 422 (Unprocessable Entity)', 'http://127.0.0.1:18781/shell/action'],
    ['Uncaught Error: synthetic', `${origin}/shell/action`],
  ]) assert.equal(consoleErrorClass(origin, text, url), 'unexpected');
});
