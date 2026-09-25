// Private CLI replay only; not a standalone CSS build or browser certification.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const [kitArg, binaryArg, parentArg, baselineArg] = process.argv.slice(2);
assert(process.argv.length === 6 && [kitArg, binaryArg, parentArg, baselineArg].every(path.isAbsolute),
  'usage: node focus-delivery.mjs KIT PRIVATE_BINARY PRIVATE_PARENT BASE_ARTIFACT');
const [kit, binary, parent, baseline] = [kitArg, binaryArg, parentArg, baselineArg].map(p => fs.realpathSync(p));
assert.equal(fs.statSync(parent).mode & 0o777, 0o700);
assert(binary.startsWith(parent + path.sep) && fs.lstatSync(binary).isFile() && fs.lstatSync(binary).nlink === 1);
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const expectedBase = '34644520c31c87de479b4c61472b6ea3fb0e759f7c757db2fbb291fd9b2883ca';
const expectedCSS = '9b1fcd1b50999d5d82878e0444c55ad097a4755fd57df2057f2ab25bb453ffe8';
const archive = 'b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8';
const assets = path.join(kit, 'data/design-assets');
const inputs = new Map();
function input(file, expected) {
  const bytes = fs.readFileSync(file), digest = hash(bytes);
  if (expected) assert.equal(digest, expected, 'input digest differs: ' + path.basename(file));
  inputs.set(file, digest); return bytes;
}
input(fileURLToPath(import.meta.url)); input(binary);
const baselineBytes = input(baseline, expectedBase);
const css = input(path.join(kit, 'internal/designadapt/drawer_focus.css'), expectedCSS);
input(path.join(assets, 'archives', archive + '.zip'), archive);
input(path.join(assets, 'catalog.json'));
const out = fs.mkdtempSync(path.join(parent, 'focus-cli-'));
fs.chmodSync(out, 0o700);
const report = {kind: 'private-p08-focus-cli-delivery', verification: 'not-certified',
  inputs: Object.fromEntries(inputs), commands: [], inventories: {}, checks: [], errors: []};
const write = (name, body) => fs.writeFileSync(path.join(out, name), body, {mode: 0o600, flag: 'wx'});
function invoke(expected, flags, json = false) {
  const args = ['data', 'assets-adapt-drawer', '-root', assets, ...flags];
  const result = spawnSync(binary, args, {cwd: out, timeout: 30000, maxBuffer: 4 * 2 ** 20});
  const number = report.commands.length;
  const stdout = result.stdout ?? Buffer.alloc(0), stderr = result.stderr ?? Buffer.alloc(0);
  write(number + '.stdout', stdout); write(number + '.stderr', stderr);
  report.commands.push({args, status: result.status, expected, signal: result.signal,
    stdout_sha256: hash(stdout), stderr_sha256: hash(stderr)});
  assert(!result.error && !result.signal, 'command did not complete normally');
  assert.equal(result.status, expected, 'unexpected CLI exit');
  if (!json) assert.equal(stdout.length, 0, 'non-JSON operation wrote stdout');
  if (expected === 0) assert.equal(stderr.length, 0, 'successful command wrote stderr');
  return stdout;
}
function inventory(dir, expectedNames) {
  assert.equal(fs.statSync(dir).mode & 0o777, 0o700);
  assert.deepEqual(fs.readdirSync(dir).sort(), [...expectedNames].sort());
  return Object.fromEntries(expectedNames.map(name => {
    const file = path.join(dir, name), stat = fs.lstatSync(file);
    assert(stat.isFile() && stat.nlink === 1 && (stat.mode & 0o777) === 0o600);
    const bytes = fs.readFileSync(file);
    return [name, {sha256: hash(bytes), bytes: bytes.length}];
  }));
}
try {
  const base = invoke(0, [], true), disabled = invoke(0, ['-repair=false'], true);
  assert.deepEqual(base, baselineBytes); assert.deepEqual(base, disabled);
  report.checks.push('default JSON and explicit false match prior base bytes');
  const first = invoke(0, ['-repair'], true), second = invoke(0, ['-repair'], true);
  assert.deepEqual(first, second);
  const artifact = JSON.parse(first);
  assert.equal(artifact.adapter, 'p08-drawer-focus-v1');
  assert.equal(artifact.verification, 'not-verified');
  assert.deepEqual(artifact.base, JSON.parse(base));
  assert.equal(artifact.css_sha256, expectedCSS); assert.deepEqual(Buffer.from(artifact.css), css);
  assert.equal(artifact.wrapper_attribute, 'data-appkit-drawer-focus');
  report.base_artifact_sha256 = hash(base); report.repair_artifact_sha256 = hash(first);
  report.repair_css_sha256 = expectedCSS;
  const baseDir = path.join(out, 'base'), a = path.join(out, 'repair-a'), b = path.join(out, 'repair-b');
  invoke(0, ['-out', baseDir]); invoke(0, ['-repair', '-out', a]); invoke(0, ['-repair', '-out', b]);
  const names = ['drawer.templ', 'drawer.go', 'defaults.json', 'drawer_focus.css', 'USAGE.md', 'artifact.json'];
  report.inventories.base = inventory(baseDir, ['drawer.templ', 'drawer.go', 'defaults.json', 'USAGE.md', 'artifact.json']);
  report.inventories.repairA = inventory(a, names); report.inventories.repairB = inventory(b, names);
  assert.deepEqual(report.inventories.repairA, report.inventories.repairB);
  assert.equal(report.inventories.base['artifact.json'].sha256, expectedBase);
  assert.equal(report.inventories.repairA['artifact.json'].sha256, hash(first));
  for (const name of ['drawer.templ', 'drawer.go', 'defaults.json']) assert.deepEqual(report.inventories.base[name], report.inventories.repairA[name]);
  report.checks.push('two exact private six-file exports preserve base source/defaults');
  invoke(0, ['-check', baseDir]); invoke(0, ['-repair', '-check', a]); invoke(0, ['-repair', '-check', b]);
  invoke(1, ['-check', a]); invoke(1, ['-repair', '-check', baseDir]);
  invoke(1, ['-repair', '-check', path.join(out, 'missing')]);
  invoke(2, ['-repair', '-out', a]); invoke(2, ['-repair', '-out', path.join(out, 'invalid'), '-check', a]);
  assert(!fs.existsSync(path.join(out, 'invalid')));
  const changedCSS = css.toString() + '\n/* synthetic drift */';
  fs.writeFileSync(path.join(b, 'drawer_focus.css'), changedCSS);
  invoke(1, ['-repair', '-check', b]);
  const changedArtifact = {...artifact, css: changedCSS, css_sha256: hash(Buffer.from(changedCSS))};
  fs.writeFileSync(path.join(b, 'artifact.json'), JSON.stringify(changedArtifact) + '\n');
  invoke(1, ['-repair', '-check', b]);
  report.checks.push('changed and rehashed CSS refused against independent replay');
  fs.writeFileSync(path.join(b, 'drawer_focus.css'), css); fs.writeFileSync(path.join(b, 'artifact.json'), first);
  fs.chmodSync(path.join(b, 'drawer_focus.css'), 0o640); invoke(1, ['-repair', '-check', b]);
  fs.chmodSync(path.join(b, 'drawer_focus.css'), 0o600);
  invoke(0, ['-repair', '-check', b]);
  fs.writeFileSync(path.join(b, 'extra.txt'), 'synthetic extra member\n', {mode: 0o600, flag: 'wx'});
  invoke(1, ['-repair', '-check', b]);
  report.negative_control = 'repair-b retains intentional extra.txt; it is not a matching delivery';
  invoke(0, ['-repair', '-check', a]); invoke(0, ['-check', baseDir]);
  assert.deepEqual(report.inventories.repairA, inventory(a, names));
  for (const [file, digest] of inputs) assert.equal(hash(fs.readFileSync(file)), digest, 'input changed');
  report.inputs_unchanged = true;
  report.checks.push('source/tool inputs unchanged; valid base and repair-a still match');
} catch (error) { report.errors.push(String(error)); process.exitCode = 1; }
report.limit = 'CLI/source delivery only; no standalone CSS/runtime build, browser run, dependency installation or real consumer adoption.';
const body = Buffer.from(JSON.stringify(report, null, 2) + '\n');
const name = report.errors.length ? 'failure.json' : 'receipt.json';
write(name, body);
console.log(JSON.stringify({out, file: name, sha256: hash(body), commands: report.commands.length, checks: report.checks, errors: report.errors}));
