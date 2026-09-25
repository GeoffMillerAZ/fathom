import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {fileURLToPath} from 'node:url';

import {runCLI, validateHandoffPath, validateLaunchURL, writeHandoff} from './handoff.mjs';

const token = 'A'.repeat(42) + '_';
const validURL = `http://127.0.0.1:49152/auth/launch?token=${token}`;

function privateRoot(t) {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'appkit-browser-handoff-')));
  fs.chmodSync(root, 0o700);
  t.after(() => fs.rmSync(root, {recursive: true, force: true}));
  return root;
}

test('launch URL is one exact canonical loopback capability URL', () => {
  assert.deepEqual(validateLaunchURL(validURL), {
    origin: 'http://127.0.0.1:49152',
    launchURL: validURL,
  });
  for (const value of [
    '', 'https://127.0.0.1:49152/auth/launch?token=' + token,
    'http://localhost:49152/auth/launch?token=' + token,
    'http://user@127.0.0.1:49152/auth/launch?token=' + token,
    'http://127.0.0.1:0/auth/launch?token=' + token,
    'http://127.0.0.1:65536/auth/launch?token=' + token,
    'http://127.0.0.1:049152/auth/launch?token=' + token,
    'http://127.0.0.1:49152/auth/launch?token=' + 'A'.repeat(42),
    'http://127.0.0.1:49152/auth/launch?token=' + token + '&extra=1',
    'http://127.0.0.1:49152/auth/launch?extra=1&token=' + token,
    'http://127.0.0.1:49152/auth/launch?token=' + token + '#fragment',
    'http://127.0.0.1:49152/auth//launch?token=' + token,
  ]) assert.throws(() => validateLaunchURL(value), /invalid launch URL/, value);
});

test('output authority is a new leaf below a canonical private owned directory', t => {
  const root = privateRoot(t);
  const output = path.join(root, 'handoff.json');
  assert.equal(validateHandoffPath(output), output);

  const existing = path.join(root, 'existing.json');
  fs.writeFileSync(existing, 'preserve');
  assert.throws(() => validateHandoffPath(existing), /invalid output authority/);

  const publicRoot = path.join(root, 'public');
  fs.mkdirSync(publicRoot, {mode: 0o755});
  fs.chmodSync(publicRoot, 0o755);
  assert.throws(() => validateHandoffPath(path.join(publicRoot, 'out')), /invalid output authority/);
  assert.throws(() => validateHandoffPath(path.join(root, 'missing', 'out')), /invalid output authority/);
  assert.throws(() => validateHandoffPath('relative.json'), /invalid output authority/);

  const link = path.join(root, 'linked');
  fs.symlinkSync(root, link);
  assert.throws(() => validateHandoffPath(path.join(link, 'out')), /invalid output authority/);
});

test('exclusive handoff is exact JSON at mode 0600 and never clobbers', t => {
  const root = privateRoot(t);
  const output = path.join(root, 'handoff.json');
  assert.equal(writeHandoff([validURL], {APPKIT_BROWSER_HANDOFF: output}), undefined);
  assert.equal(fs.lstatSync(output).mode & 0o777, 0o600);
  assert.deepEqual(JSON.parse(fs.readFileSync(output, 'utf8')), {
    origin: 'http://127.0.0.1:49152', launchURL: validURL,
  });
  assert.throws(() => writeHandoff([validURL], {APPKIT_BROWSER_HANDOFF: output}), /invalid output authority/);
  assert.equal(JSON.parse(fs.readFileSync(output, 'utf8')).launchURL, validURL);
});

test('completed inode is published atomically and immediate consumer unlink is safe', t => {
  const root = privateRoot(t);
  const output = path.join(root, 'handoff.json');
  let openedFinal = false;
  let consumed;
  let consumedMode;
  const fsAPI = new Proxy(fs, {get(target, property) {
    if (property === 'openSync') return (file, ...args) => {
      const descriptor = target.openSync(file, ...args);
      if (file === output) {
        // This is the original race: visibility at open lets the consumer read
        // and unlink an incomplete final pathname while the writer holds its fd.
        openedFinal = true;
        consumed = target.readFileSync(output, 'utf8');
        target.unlinkSync(output);
      }
      return descriptor;
    };
    if (property === 'linkSync') return (staging, destination) => {
      assert.equal(destination, output);
      assert.equal(fs.existsSync(output), false);
      const staged = fs.lstatSync(staging);
      assert.equal(staged.mode & 0o777, 0o600);
      target.linkSync(staging, destination);
      consumed = target.readFileSync(destination, 'utf8');
      consumedMode = target.lstatSync(destination).mode & 0o777;
      target.unlinkSync(destination);
    };
    return Reflect.get(target, property);
  }});
  assert.equal(writeHandoff([validURL], {APPKIT_BROWSER_HANDOFF: output}, fsAPI), undefined);
  assert.equal(openedFinal, false);
  assert.equal(consumedMode, 0o600);
  assert.deepEqual(JSON.parse(consumed), {
    origin: 'http://127.0.0.1:49152', launchURL: validURL,
  });
  assert.equal(fs.existsSync(output), false);
  assert.deepEqual(fs.readdirSync(root), []);
});

test('publication collision preserves competitor and removes only staging inode', t => {
  const root = privateRoot(t);
  const output = path.join(root, 'handoff.json');
  let inserted = false;
  const fsAPI = new Proxy(fs, {get(target, property) {
    if (property === 'linkSync') return (staging, destination) => {
      target.writeFileSync(destination, 'competitor', {flag: 'wx', mode: 0o600});
      inserted = true;
      return target.linkSync(staging, destination);
    };
    return Reflect.get(target, property);
  }});
  assert.throws(() => writeHandoff([validURL], {APPKIT_BROWSER_HANDOFF: output}, fsAPI), /output write failed/);
  assert.equal(inserted, true);
  assert.equal(fs.readFileSync(output, 'utf8'), 'competitor');
  assert.deepEqual(fs.readdirSync(root), ['handoff.json']);
});

test('runner requires one argument and explicit output environment', t => {
  const root = privateRoot(t);
  assert.throws(() => writeHandoff([], {APPKIT_BROWSER_HANDOFF: path.join(root, 'a')}), /wrong argument count/);
  assert.throws(() => writeHandoff([validURL, validURL], {APPKIT_BROWSER_HANDOFF: path.join(root, 'a')}), /wrong argument count/);
  assert.throws(() => writeHandoff([validURL], {}), /invalid output authority/);
  let diagnostic = '';
  assert.equal(runCLI([], {}, fs, {write(value) { diagnostic += value; }}), 1);
  assert.equal(diagnostic, 'browser handoff refused\n');
});

test('CLI refusal diagnostics disclose neither capability nor destination', t => {
  const root = privateRoot(t);
  const output = path.join(root, 'handoff.json');
  fs.writeFileSync(output, 'collision', {mode: 0o600});
  const helper = fileURLToPath(new URL('./handoff.mjs', import.meta.url));
  const result = spawnSync(process.execPath, [helper, validURL], {
    env: {...process.env, APPKIT_BROWSER_HANDOFF: output}, encoding: 'utf8', timeout: 5000,
  });
  assert.equal(result.status, 1);
  assert.equal(result.stdout, '');
  assert.equal(result.stderr, 'browser handoff refused\n');
  assert(!result.stderr.includes(token));
  assert(!result.stderr.includes(output));
  assert.equal(fs.readFileSync(output, 'utf8'), 'collision');
});
