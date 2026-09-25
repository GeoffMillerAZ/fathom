#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const MAX_OUTPUT = 1024 * 1024;
const MAX_FILE = 256 * 1024 * 1024;
const TIMEOUT_MS = 15_000;
const XATTR_TIMEOUT_MS = 5_000;
const [preparerArg, launcherArg, outputArg] = process.argv.slice(2);
if (!preparerArg || !launcherArg || !outputArg || process.argv.length !== 5) {
  console.error('usage: node bundle_check_probe.mjs ABS_PREPARER ABS_LAUNCHER EXISTING_EMPTY_PRIVATE_OUTPUT');
  process.exit(2);
}

const sha = body => crypto.createHash('sha256').update(body).digest('hex');
function identity(p) {
  const before = fs.lstatSync(p);
  if (!before.isFile() || before.size > MAX_FILE) throw new Error(`bounded regular file required: ${p}`);
  const fd = fs.openSync(p, 'r'), hash = crypto.createHash('sha256'), chunk = Buffer.allocUnsafe(1024 * 1024);
  try {
    const opened = fs.fstatSync(fd);
    if (opened.dev !== before.dev || opened.ino !== before.ino || opened.size !== before.size) throw new Error(`file changed while opening: ${p}`);
    let total = 0, read;
    while ((read = fs.readSync(fd, chunk, 0, chunk.length, null)) > 0) { total += read; if (total > MAX_FILE) throw new Error(`file exceeds bound: ${p}`); hash.update(chunk.subarray(0, read)); }
    if (total !== opened.size) throw new Error(`file changed while hashing: ${p}`);
    return { path: p, sha256: hash.digest('hex'), bytes: total, mode: opened.mode & 0o777 };
  } finally { fs.closeSync(fd); }
}
function realExecutable(p, label) {
  if (!path.isAbsolute(p) || path.normalize(p) !== p) throw new Error(`${label} must be a clean absolute path`);
  const stat = fs.lstatSync(p);
  if (!stat.isFile() || fs.realpathSync(p) !== p || !(stat.mode & 0o100) || (stat.mode & 0o022)) throw new Error(`${label} must be a real private owner-executable file`);
  return identity(p);
}
function mkdir(p) { fs.mkdirSync(p, { mode: 0o700 }); }
function write(p, body, mode = 0o600) { fs.writeFileSync(p, body, { mode, flag: 'wx' }); }
function copyFile(src, dst) { fs.copyFileSync(src, dst, fs.constants.COPYFILE_EXCL); fs.chmodSync(dst, fs.statSync(src).mode & 0o777); }
function copyTree(src, dst) {
  const stat = fs.lstatSync(src);
  if (stat.isSymbolicLink()) throw new Error(`symlink refused while copying ${src}`);
  if (stat.isDirectory()) {
    fs.mkdirSync(dst, { mode: stat.mode & 0o777 });
    for (const name of fs.readdirSync(src).sort()) copyTree(path.join(src, name), path.join(dst, name));
  } else if (stat.isFile()) copyFile(src, dst);
  else throw new Error(`non-file refused while copying ${src}`);
}
function inventory(root) {
  const rows = [];
  function visit(current, rel = '') {
    const stat = fs.lstatSync(current);
    if (stat.isSymbolicLink()) throw new Error(`symlink refused in ${root}`);
    rows.push({ path: rel || '.', type: stat.isDirectory() ? 'directory' : 'file', mode: stat.mode & 0o777,
      ...(stat.isFile() ? { bytes: stat.size, sha256: identity(current).sha256 } : {}) });
    if (stat.isDirectory()) for (const name of fs.readdirSync(current).sort()) visit(path.join(current, name), rel ? `${rel}/${name}` : name);
  }
  visit(root);
  return rows;
}

let receiptPath;
const receipt = { schema_version: 1, completed: false, commands: [], assertions: [], artifacts: {} };
const save = () => { if (receiptPath) fs.writeFileSync(receiptPath, `${JSON.stringify(receipt, null, 2)}\n`, { mode: 0o600 }); };
function check(condition, name, observed) {
  receipt.assertions.push({ name, passed: Boolean(condition), observed }); save();
  if (!condition) throw new Error(`assertion failed: ${name}`);
}
function run(label, executable, args, expected) {
  const result = spawnSync(executable, args, { cwd: outputArg, encoding: 'utf8', timeout: TIMEOUT_MS, maxBuffer: MAX_OUTPUT,
    env: { PATH: '/usr/bin:/bin', LANG: 'C', LC_ALL: 'C', TMPDIR: path.join(outputArg, 'tmp') } });
  const record = { label, executable, args, expected_exit: expected, exit: result.status, signal: result.signal,
    error: result.error?.message || '', stdout: result.stdout || '', stderr: result.stderr || '' };
  receipt.commands.push(record); save();
  check(!result.error && result.status === expected && !result.signal, `${label} exit`, { exit: result.status, signal: result.signal, error: record.error });
  return record;
}
function noCheckSuccess(command, label) {
  let parsed;
  try { parsed = JSON.parse(command.stdout); } catch {}
  check(!(parsed && parsed.checked_only === true), `${label} emitted no check success`, { stdout_bytes: Buffer.byteLength(command.stdout) });
}
function ancestors(p) {
  const paths = [];
  for (let current = p; ; current = path.dirname(current)) { paths.push(current); if (path.dirname(current) === current) break; }
  return paths;
}
function inspectXattrs(label, target) {
  const result = spawnSync('/usr/bin/xattr', [target], { cwd: outputArg, encoding: 'utf8', timeout: XATTR_TIMEOUT_MS, maxBuffer: 64 * 1024,
    env: { PATH: '/usr/bin:/bin', LANG: 'C', LC_ALL: 'C', TMPDIR: path.join(outputArg, 'tmp') } });
  const stdout = result.stdout || '', stderr = result.stderr || '';
  const names = stdout.split('\n').map(value => value.trim()).filter(Boolean);
  const record = { label, path: target, exit: result.status, signal: result.signal, error: result.error?.message || '',
    attribute_name_count: names.length, quarantine_present: names.includes('com.apple.quarantine'), stdout_bytes: Buffer.byteLength(stdout), stderr_bytes: Buffer.byteLength(stderr) };
  receipt.inspections.push(record); save();
  check(!result.error && result.status === 0 && !result.signal && stderr === '', `${label} xattr names inspected`, { path: target, exit: result.status, signal: result.signal, stderr_bytes: record.stderr_bytes });
  check(!record.quarantine_present, `${label} is not quarantined`, { path: target, attribute_name_count: names.length });
}

try {
  const preparer = realExecutable(preparerArg, 'preparer');
  const launcher = realExecutable(launcherArg, 'launcher');
  if (!path.isAbsolute(outputArg) || path.normalize(outputArg) !== outputArg) throw new Error('output must be a clean absolute path');
  const outputStat = fs.lstatSync(outputArg);
  if (!outputStat.isDirectory() || fs.realpathSync(outputArg) !== outputArg || (outputStat.mode & 0o077) || fs.readdirSync(outputArg).length) throw new Error('output must be an existing empty real private directory');

  for (const name of ['evidence', 'inputs', 'fixtures', 'assemblies', 'relocated', 'failures', 'tmp']) mkdir(path.join(outputArg, name));
  receiptPath = path.join(outputArg, 'receipt.json'); save();
  const harnessSource = fileURLToPath(import.meta.url);
  const retainedHarness = path.join(outputArg, 'evidence', 'bundle_check_probe.mjs');
  const retainedPreparer = path.join(outputArg, 'inputs', 'prepare-bundle');
  const retainedLauncher = path.join(outputArg, 'inputs', 'bundle-launcher');
  copyFile(harnessSource, retainedHarness);
  const server = path.join(outputArg, 'fixtures', 'gallery-server');
  const native = path.join(outputArg, 'fixtures', 'gallery-native');
  write(server, '#!/bin/sh\n# synthetic server role; must never execute\nexit 97\n', 0o500);
  write(native, '#!/bin/sh\n# synthetic native role; must never execute\nexit 97\n', 0o500);
  const fixtureServer = identity(server), fixtureNative = identity(native);
  const kitRevision = sha(Buffer.from(`${preparer.sha256}\n${launcher.sha256}\n`));
  const config = { preparer, launcher, retained_harness: identity(retainedHarness), kit_revision: kitRevision,
    kit_revision_basis: 'sha256(preparer SHA256 hex string + newline + launcher SHA256 hex string + newline); authored artifact-set identity, not source provenance',
    min_macos: '12.0', min_macos_scope: 'declaration only; not linked-target or runtime proof', server: fixtureServer, native: fixtureNative };
  const configPath = path.join(outputArg, 'evidence', 'config.json');
  write(configPath, `${JSON.stringify(config, null, 2)}\n`);
  receipt.inputs = config; receipt.config = identity(configPath); receipt.inspections = []; save();
  check(process.platform === 'darwin', 'quarantine inspection platform is supported', { platform: process.platform, supported: process.platform === 'darwin' });
  receipt.inspector = realExecutable('/usr/bin/xattr', 'xattr inspector'); save();
  const sourceInspection = new Map();
  for (const [role, helper] of [['preparer source', preparerArg], ['launcher source', launcherArg]]) for (const target of ancestors(helper)) {
    if (!sourceInspection.has(target)) sourceInspection.set(target, []);
    sourceInspection.get(target).push(role);
  }
  for (const [target, roles] of sourceInspection) inspectXattrs(`${roles.join(' and ')}${roles.length > 1 ? ' ancestors' : ''}`, target);
  copyFile(preparerArg, retainedPreparer); copyFile(launcherArg, retainedLauncher);
  const retainedHelpers = { preparer: identity(retainedPreparer), launcher: identity(retainedLauncher) };
  receipt.retained_helpers = retainedHelpers; save();
  const copiedInspection = new Map();
  for (const [role, helper] of [['retained preparer', retainedPreparer], ['retained launcher', retainedLauncher]]) for (const target of ancestors(helper)) {
    if (!copiedInspection.has(target)) copiedInspection.set(target, []);
    copiedInspection.get(target).push(role);
  }
  for (const [target, roles] of copiedInspection) inspectXattrs(`${roles.join(' and ')}${roles.length > 1 ? ' ancestors' : ''}`, target);
  check(preparer.sha256 === retainedHelpers.preparer.sha256 && launcher.sha256 === retainedHelpers.launcher.sha256 && preparer.mode === retainedHelpers.preparer.mode && launcher.mode === retainedHelpers.launcher.mode,
    'retained supplied binaries are byte-and-mode exact', { preparer: preparer.sha256, launcher: launcher.sha256 });

  const prepareArgs = out => ['-out', out, '-launcher', retainedLauncher, '-launcher-sha256', launcher.sha256,
    '-server', server, '-server-sha256', fixtureServer.sha256, '-native', native, '-native-sha256', fixtureNative.sha256,
    '-kit-revision', kitRevision, '-min-macos', config.min_macos];
  const first = path.join(outputArg, 'assemblies', 'KnownGood.app');
  const second = path.join(outputArg, 'assemblies', 'RelocateMe.app');
  for (const [label, app] of [['prepare-first', first], ['prepare-second', second]]) {
    const command = run(label, retainedPreparer, prepareArgs(app), 0);
    check(command.stderr === '' && Boolean(JSON.parse(command.stdout).files), `${label} returned clean manifest JSON`, { stdout_sha256: sha(Buffer.from(command.stdout)), stderr_bytes: Buffer.byteLength(command.stderr) });
  }
  const firstInventory = inventory(first), secondInventory = inventory(second);
  check(JSON.stringify(firstInventory) === JSON.stringify(secondInventory), 'two assemblies have identical bytes and modes', { entries: firstInventory.length });
  receipt.artifacts.known_good = firstInventory;
  const checkPlan = (label, app) => {
    const launcherPath = path.join(app, 'Contents', 'MacOS', 'gallery-launcher');
    const command = run(label, launcherPath, ['-check'], 0);
    const body = JSON.parse(command.stdout), expectedServer = path.join(app, 'Contents', 'Resources', 'gallery-server');
    const expectedNative = path.join(app, 'Contents', 'Resources', 'gallery-native');
    check(command.stderr === '' && body.checked_only === true && body.plan?.Executable === expectedServer && JSON.stringify(body.plan?.Args) === JSON.stringify(['-native-host', expectedNative, '-native-host-sha256', fixtureNative.sha256]),
      `${label} exact checked-only plan`, { checked_only: body.checked_only, executable: body.plan?.Executable, args: body.plan?.Args });
  };
  checkPlan('check-known-good', first);
  const relocated = path.join(outputArg, 'relocated', 'Gallery.app'); fs.renameSync(second, relocated);
  check(JSON.stringify(firstInventory) === JSON.stringify(inventory(relocated)), 'relocated bytes and modes unchanged', { destination: relocated });
  checkPlan('check-relocated', relocated); receipt.artifacts.relocated = inventory(relocated);

  const failureCopy = name => { const dst = path.join(outputArg, 'failures', `${name}.app`); copyTree(first, dst); return dst; };
  const failureCheck = (label, app, args = ['-check']) => { const result = run(label, path.join(app, 'Contents', 'MacOS', 'gallery-launcher'), args, args.length === 1 ? 1 : 2); noCheckSuccess(result, label); };
  let failed = failureCopy('altered-server'); fs.appendFileSync(path.join(failed, 'Contents', 'Resources', 'gallery-server'), 'changed'); failureCheck('altered-server', failed);
  failed = failureCopy('missing-native'); fs.unlinkSync(path.join(failed, 'Contents', 'Resources', 'gallery-native')); failureCheck('missing-native', failed);
  failed = failureCopy('noncanonical-metadata'); fs.writeFileSync(path.join(failed, 'Contents', 'Resources', 'bundle.json'), ` ${fs.readFileSync(path.join(failed, 'Contents', 'Resources', 'bundle.json'), 'utf8')}`); failureCheck('noncanonical-metadata', failed);
  failed = failureCopy('invalid-metadata'); { const manifest = JSON.parse(fs.readFileSync(path.join(failed, 'Contents', 'Resources', 'bundle.json'))); manifest.min_macos = '12'; fs.writeFileSync(path.join(failed, 'Contents', 'Resources', 'bundle.json'), `${JSON.stringify(manifest)}\n`); } failureCheck('invalid-metadata', failed);
  const extra = run('extra-launch-flag', path.join(first, 'Contents', 'MacOS', 'gallery-launcher'), ['-check', '-unexpected'], 2); noCheckSuccess(extra, 'extra-launch-flag');
  const occupied = path.join(outputArg, 'failures', 'occupied.app'); mkdir(occupied);
  const collision = run('occupied-destination', retainedPreparer, prepareArgs(occupied), 1); noCheckSuccess(collision, 'occupied-destination');
  check(JSON.stringify(inventory(first)) === JSON.stringify(firstInventory), 'known-good inventory unchanged after refusals', { entries: firstInventory.length });
  check(fs.readdirSync(occupied).length === 0, 'occupied destination refusal left existing directory empty', { entries: fs.readdirSync(occupied) });
  const retained = [[retainedPreparer, retainedHelpers.preparer], [retainedLauncher, retainedHelpers.launcher], [retainedHarness, config.retained_harness],
    [configPath, receipt.config], [server, fixtureServer], [native, fixtureNative]];
  check(retained.every(([current, expected]) => { const got = identity(current); return got.sha256 === expected.sha256 && got.bytes === expected.bytes && got.mode === expected.mode; }),
    'retained helper, harness, config, and fixture bytes and modes unchanged', { files: retained.map(([current]) => current) });
  receipt.artifacts.failures = Object.fromEntries(fs.readdirSync(path.join(outputArg, 'failures')).sort().map(name => [name, inventory(path.join(outputArg, 'failures', name))]));
  receipt.completed = true; save();
  console.log(JSON.stringify({ completed: true, receipt: receiptPath }));
} catch (error) {
  receipt.failure = error instanceof Error ? error.message : String(error); save();
  console.error(JSON.stringify({ completed: false, error: receipt.failure, receipt: receiptPath || null }));
  process.exitCode = 1;
}
