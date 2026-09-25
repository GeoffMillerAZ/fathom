import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const env = { PATH: '/usr/bin:/bin', LANG: 'C', LC_ALL: 'C', TMPDIR: os.tmpdir() };
const skip = process.platform === 'darwin' ? false : 'unsupported: requires Darwin xattr semantics';
const sha = body => crypto.createHash('sha256').update(body).digest('hex');

function runnerPath() {
  const selected = process.env.APPKIT_BUNDLE_PROBE_TEST_SCRIPT || fileURLToPath(new URL('./bundle_check_probe.mjs', import.meta.url));
  assert(path.isAbsolute(selected), 'runner path must be absolute');
  const stat = fs.lstatSync(selected);
  assert(stat.isFile(), 'runner path must be a regular file');
  assert.equal(fs.realpathSync(selected), selected, 'runner path must not use symlinks');
  return selected;
}

function executable(root, name) {
  const file = path.join(root, name);
  fs.writeFileSync(file, '#!/bin/sh\nexit 97\n', { mode: 0o700, flag: 'wx' });
  return file;
}

function fixture() {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'appkit-bundle-probe-xattr-')));
  fs.chmodSync(root, 0o700);
  const helpers = path.join(root, 'helpers');
  const output = path.join(root, 'output');
  fs.mkdirSync(helpers, { mode: 0o700 });
  fs.mkdirSync(output, { mode: 0o700 });
  return { root, helpers, output, preparer: executable(helpers, 'prepare-bundle'), launcher: executable(helpers, 'bundle-launcher') };
}

function quarantine(target) {
  const result = spawnSync('/usr/bin/xattr', ['-w', 'com.apple.quarantine', '0081;synthetic;appkit;', target],
    { encoding: 'utf8', timeout: 5_000, maxBuffer: 64 * 1024, env });
  assert.equal(result.error, undefined, `xattr setup failed: ${result.error?.message}`);
  assert.equal(result.status, 0, `xattr setup failed: ${result.stderr}`);
}

function assertRefusal(kind, setup, t) {
  const selectedRunner = runnerPath();
  const f = fixture();
  t.diagnostic(`retained fixture and output: ${f.root}`);
  setup(f);
  const result = spawnSync(process.execPath, [selectedRunner, f.preparer, f.launcher, f.output],
    { encoding: 'utf8', timeout: 15_000, maxBuffer: 1024 * 1024, env });
  assert.equal(result.error, undefined, `runner invocation failed: ${result.error?.message}`);
  assert.equal(result.status, 1, `runner did not refuse ${kind}: ${result.stdout}\n${result.stderr}`);

  const receiptPath = path.join(f.output, 'receipt.json');
  const configPath = path.join(f.output, 'evidence', 'config.json');
  const harnessPath = path.join(f.output, 'evidence', 'bundle_check_probe.mjs');
  for (const retained of [receiptPath, configPath, harnessPath]) assert(fs.lstatSync(retained).isFile(), `missing retained ${retained}`);
  assert.equal(sha(fs.readFileSync(harnessPath)), sha(fs.readFileSync(selectedRunner)), 'retained harness differs');

  const receipt = JSON.parse(fs.readFileSync(receiptPath, 'utf8'));
  const config = JSON.parse(fs.readFileSync(configPath, 'utf8'));
  assert.equal(receipt.completed, false);
  assert.match(receipt.failure, /quarantin/i);
  assert.deepEqual(receipt.commands, [], 'prepare/check helper command unexpectedly ran');
  assert(receipt.inspections.some(entry => entry.quarantine_present === true), 'quarantine observation missing');
  assert(receipt.assertions.some(entry => entry.passed === false && /quarantin/i.test(entry.name)), 'quarantine refusal assertion missing');
  assert.equal(config.preparer.path, f.preparer);
  assert.equal(config.launcher.path, f.launcher);
  assert.deepEqual(fs.readdirSync(path.join(f.output, 'inputs')), [], 'helper copy exists after refusal');
  for (const dir of ['assemblies', 'relocated', 'failures']) assert.deepEqual(fs.readdirSync(path.join(f.output, dir)), [], `${dir} is not empty`);
  assert.match(result.stderr, /quarantin/i);
  return f.output;
}

test('leaf-quarantined supplied helper is refused before copy or execution', { skip }, t => {
  const output = assertRefusal('leaf quarantine', f => quarantine(f.preparer), t);
  t.diagnostic(`retained refusal: ${output}`);
});

test('ancestor-quarantined supplied helper is refused before copy or execution', { skip }, t => {
  const output = assertRefusal('ancestor quarantine', f => quarantine(f.helpers), t);
  t.diagnostic(`retained refusal: ${output}`);
});
