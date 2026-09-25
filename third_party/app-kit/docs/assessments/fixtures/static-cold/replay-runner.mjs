import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const evidence = '/Volumes/m2ext/appkit-evidence/appkit-static-replay.ud7EOtjr';
const inputs = '/Volumes/m2ext/appkit-evidence/appkit-static-cold.7qApnn7w';
const candidate = path.join(inputs, 'candidate');
const closure = path.join(inputs, 'closure-v2');
const go = '/nix/store/7ycp8j45iay38g9mjaxmy4jhwdsrb47y-go-1.26.3/share/go/bin/go';
const tailwind = path.join(inputs, 'frozen', 'tools', 'tailwindcss');
const expectedGoSHA = 'c3c8820e5edea04144cd3b6cf504aadd050bd2c7e4e63a69a6da17aed1661163';
const expectedTailwindSHA = '56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e';
const sourceCanary = '/Volumes/m2ext/code/geoffmilleraz/app-kit/go.mod';
const originalCacheCanary = '/Users/geoff/.dev-cache/go/modules/cache/download/golang.org/x/net/@v/v0.54.0.mod';
const hash = body => createHash('sha256').update(body).digest('hex');
const fileHash = name => hash(fs.readFileSync(name));
const runName = process.argv[2];
if (!['cold-1', 'cold-2'].includes(runName) || process.argv.length !== 3) throw Error('usage: node cold-runner.mjs cold-1|cold-2');
const runnerPath = fileURLToPath(import.meta.url);
const frozenManifest = path.join(evidence, 'frozen', 'pre-execution-v2-sha256.txt');
const manifestRecords = fs.readFileSync(frozenManifest, 'utf8').split('\n').filter(Boolean).map(line => {
  const match = line.match(/^([0-9a-f]{64})  (\/.+)$/);
  if (!match) throw Error('invalid pre-execution manifest record');
  return {sha256: match[1], path: match[2]};
});
for (const record of manifestRecords) {
  const info = fs.lstatSync(record.path);
  if (!info.isFile() || info.isSymbolicLink() || fileHash(record.path) !== record.sha256) throw Error(`frozen input drift: ${record.path}`);
}
if (!manifestRecords.some(record => record.path === runnerPath)) throw Error('executed runner is absent from pre-execution manifest');
const {trackOwnedChild, terminateOwned} = await import('file:///Volumes/m2ext/appkit-evidence/appkit-static-cold.7qApnn7w/frozen/harness/owned_process.mjs');
const {assertInventoryEqual} = await import('./inventory.mjs');
const peerName = runName === 'cold-1' ? 'cold-2' : 'cold-1';
const root = path.join(evidence, runName), peer = path.join(evidence, peerName);
if (fs.existsSync(root)) {
  const existing = fs.readdirSync(root).sort();
  if (existing.length !== 1 || existing[0] !== 'foreign-sentinel') throw Error(`${runName} root must be fresh except for its peer canary`);
} else fs.mkdirSync(root, {mode: 0o700});
fs.mkdirSync(peer, {recursive: true, mode: 0o700});
const peerSentinel = path.join(peer, 'foreign-sentinel');
if (!fs.existsSync(peerSentinel)) fs.writeFileSync(peerSentinel, peerName, {flag: 'wx', mode: 0o600});
for (const dir of ['gopath', 'module-cache', 'build-cache', 'tmp', 'logs']) fs.mkdirSync(path.join(root, dir), {mode: 0o700});
for (const name of ['module-cache', 'build-cache']) if (fs.readdirSync(path.join(root, name)).length !== 0) throw Error(`${name} was not initially empty`);

const receipt = JSON.parse(fs.readFileSync(path.join(candidate, 'receipt.json')));
const closureReceipt = JSON.parse(fs.readFileSync(path.join(closure, 'closure-receipt.json')));
const proxyInventory = JSON.parse(fs.readFileSync(path.join(inputs, 'frozen', 'proxy-inventory.json')));
if (!closureReceipt.passed || closureReceipt.candidate.version !== receipt.version) throw Error('unaccepted dependency closure');
if (fileHash(go) !== expectedGoSHA || fileHash(tailwind) !== expectedTailwindSHA) throw Error('tool identity mismatch');
if (fs.realpathSync(closureReceipt.toolchain.go_executable) !== go) throw Error('resolved Go executable changed');
if (closureReceipt.toolchain?.goroot !== '/nix/store/7ycp8j45iay38g9mjaxmy4jhwdsrb47y-go-1.26.3/share/go' ||
    fileHash(closureReceipt.toolchain.compile.path) !== closureReceipt.toolchain.compile.sha256 ||
    fileHash(closureReceipt.toolchain.link.path) !== closureReceipt.toolchain.link.sha256) throw Error('Go toolchain closure drift');
for (const file of closureReceipt.files) {
  const name = path.join(closure, file.path);
  const info = fs.statSync(name);
  if (!info.isFile() || info.size !== file.bytes || fileHash(name) !== file.sha256) throw Error(`dependency closure drift: ${file.path}`);
}
function inventory(rootDir, prefix) {
  const records = [];
  function visit(dir) {
    for (const name of fs.readdirSync(dir).sort()) {
      const full = path.join(dir, name), info = fs.lstatSync(full);
      if (info.isSymbolicLink()) throw Error(`proxy symlink refused: ${full}`);
      if (info.isDirectory()) visit(full);
      else if (info.isFile()) records.push({scope: prefix, path: path.relative(rootDir, full).split(path.sep).join('/'), bytes: info.size, sha256: fileHash(full)});
      else throw Error(`proxy special file refused: ${full}`);
    }
  }
  visit(rootDir);
  return records;
}
const actualProxyInventory = [...inventory(path.join(candidate, 'proxy'), 'candidate'), ...inventory(path.join(closure, 'proxy'), 'dependency')];
assertInventoryEqual(actualProxyInventory, proxyInventory.files);
const candidateBase = `github.com/geoffmilleraz/app-kit/@v/`, candidateVersion = receipt.version;
const expectedCandidatePaths = [`${candidateBase}${candidateVersion}.info`, `${candidateBase}${candidateVersion}.mod`,
  `${candidateBase}${candidateVersion}.zip`, `${candidateBase}list`].sort();
const candidatePaths = actualProxyInventory.filter(file => file.scope === 'candidate').map(file => file.path).sort();
if (JSON.stringify(candidatePaths) !== JSON.stringify(expectedCandidatePaths)) throw Error('candidate proxy inventory is not the exact maintained four-file candidate');
const expectedDependency = closureReceipt.files.map(file => ({scope: 'dependency', path: file.path.replace(/^proxy\//, ''), bytes: file.bytes, sha256: file.sha256}));
const actualDependency = actualProxyInventory.filter(file => file.scope === 'dependency');
assertInventoryEqual(actualDependency, expectedDependency);
for (const name of [sourceCanary, originalCacheCanary, peerSentinel]) {
  const info = fs.lstatSync(name);
  if (!info.isFile() || info.isSymbolicLink()) throw Error(`refusal canary is not a real file: ${name}`);
}

const policy = fs.readFileSync(path.join(evidence, 'frozen', 'policies', `${runName}.sb`), 'utf8');
fs.writeFileSync(path.join(root, 'sandbox.sb'), policy, {flag: 'wx', mode: 0o600});
const env = [
  'PATH=/etc/profiles/per-user/geoff/bin:/usr/bin:/bin:/usr/sbin:/sbin', 'HOME=/var/empty',
  'LANG=en_US.UTF-8', 'TZ=UTC', 'GOENV=off', 'GOWORK=off', 'GOTOOLCHAIN=local',
  'CGO_ENABLED=0', 'GOVCS=*:off', 'GOSUMDB=off', 'GOFLAGS=-mod=readonly',
  `GOPATH=${path.join(root, 'gopath')}`, `GOMODCACHE=${path.join(root, 'module-cache')}`,
  `GOCACHE=${path.join(root, 'build-cache')}`, `TMPDIR=${path.join(root, 'tmp')}`,
  `GOTMPDIR=${path.join(root, 'tmp')}`,
  `GOPROXY=file://${path.join(candidate, 'proxy')},file://${path.join(closure, 'proxy')}`,
];
const prefix = ['/usr/bin/sandbox-exec', '-p', policy, '/usr/bin/env', '-i', ...env];
const commands = [];
async function command(label, executable, args, cwd = root, expectRefusal = false) {
  const child = spawn(prefix[0], [...prefix.slice(1), executable, ...args], {cwd, detached: true, stdio: ['ignore', 'pipe', 'pipe']});
  const owned = trackOwnedChild(child, {label});
  if (!Number.isSafeInteger(child.pid) || child.pid <= 0) throw Error(`${label} has no owned process-group leader`);
  const group = {pid: child.pid, term: null, kill: null, absent: false, cleanup_required: false};
  const alive = () => {
    try { process.kill(-child.pid, 0); return true; }
    catch (error) { if (error?.code === 'ESRCH') return false; throw Error(`${label} process-group state unknown: ${error}`); }
  };
  const signalGroup = name => {
    const record = {signal: name, returned: false, error: null};
    try { process.kill(-child.pid, name); record.returned = true; }
    catch (error) { if (error?.code !== 'ESRCH') record.error = String(error); }
    if (record.error) throw Error(`${label} process-group ${name} failed: ${record.error}`);
    return record;
  };
  const waitAbsent = async milliseconds => {
    const until = Date.now() + milliseconds;
    while (alive() && Date.now() < until) await new Promise(resolve => setTimeout(resolve, 25));
    return !alive();
  };
  const chunks = {stdout: [], stderr: []};
  let sizes = {stdout: 0, stderr: 0}, overflow = false, timedOut = false;
  for (const name of ['stdout', 'stderr']) child[name].on('data', chunk => {
    sizes[name] += chunk.length;
    if (sizes[name] > (128 << 20)) overflow = true;
    else chunks[name].push(chunk);
  });
  let timer;
  await Promise.race([
    new Promise(resolve => child.once('close', resolve)),
    new Promise(resolve => { timer = setTimeout(() => { timedOut = true; resolve(); }, 300000); }),
  ]);
  clearTimeout(timer);
  if (timedOut) {
    group.term = signalGroup('SIGTERM');
    if (!await waitAbsent(3000)) {
      group.kill = signalGroup('SIGKILL');
      if (!await waitAbsent(3000)) throw Error(`${label} process group remains after SIGKILL`);
    }
  }
  const lifecycle = await terminateOwned(owned, {graceMs: 3000, killMs: 3000, drainMs: 3000});
  if (alive()) {
    group.cleanup_required = true;
    group.kill = signalGroup('SIGKILL');
    if (!await waitAbsent(3000)) throw Error(`${label} process group remains after cleanup SIGKILL`);
  }
  group.absent = !alive();
  const stdout = Buffer.concat(chunks.stdout).toString('utf8'), stderr = Buffer.concat(chunks.stderr).toString('utf8');
  fs.writeFileSync(path.join(root, 'logs', `${label}.stdout`), stdout, {flag: 'wx', mode: 0o600});
  fs.writeFileSync(path.join(root, 'logs', `${label}.stderr`), stderr, {flag: 'wx', mode: 0o600});
  commands.push({label, executable, args, cwd, pid: child.pid, group, status: child.exitCode, signal: child.signalCode, timed_out: timedOut,
    output_overflow: overflow, lifecycle, stdout_sha256: hash(stdout), stderr_sha256: hash(stderr)});
  if (timedOut || overflow || !lifecycle.ok || !group.absent || group.cleanup_required) throw Error(`${label} lifecycle failed; see logs`);
  const refused = child.exitCode !== 0 && /operation not permitted/i.test(stderr);
  if (expectRefusal ? !refused : child.exitCode !== 0) throw Error(`${label} ${expectRefusal ? 'was not explicitly refused' : 'failed'}; see logs`);
  return stdout;
}
const result = {schema_version: 'appkit-static-cold-run-v1', run: runName, candidate: closureReceipt.candidate,
  tools: {go_path: go, go_sha256: expectedGoSHA, tailwind_sha256: expectedTailwindSHA}, environment: env,
  initial_caches: {module_cache_entries: 0, build_cache_entries: 0},
  sandbox_sha256: hash(policy), commands, passed: false};
try {
  await command('deny-source', '/usr/bin/head', ['-c', '1', sourceCanary], root, true);
  await command('deny-original-cache', '/usr/bin/head', ['-c', '1', originalCacheCanary], root, true);
  await command('deny-peer', '/usr/bin/head', ['-c', '1', peerSentinel], root, true);
  const network = `import socket\ns=socket.socket()\ntry:s.connect(('127.0.0.1',9))\nexcept PermissionError:raise SystemExit(0)\nexcept OSError as e:print(e);raise SystemExit(1)\nraise SystemExit(1)`;
  await command('deny-network', '/usr/bin/python3', ['-c', network]);
  const download = JSON.parse(await command('candidate-download', go, ['mod', 'download', '-json', `${receipt.module}@${receipt.version}`]));
  if (download.Sum !== receipt.module_sum || download.GoModSum !== receipt.go_mod_sum) throw Error('candidate sums disagree');
  const moduleDir = fs.realpathSync(download.Dir), moduleCache = fs.realpathSync(path.join(root, 'module-cache'));
  if (!moduleDir.startsWith(moduleCache + path.sep)) throw Error('candidate resolved outside owned module cache');
  await command('css', tailwind, ['-i', path.join(moduleDir, 'apps/gallery/cmd/static-content/input.css'), '-o', path.join(root, 'app.css'), '--minify'], moduleDir);
  await command('build', go, ['build', '-mod=readonly', '-trimpath', '-buildvcs=false', '-o', path.join(root, 'static-content'), './apps/gallery/cmd/static-content'], moduleDir);
  await command('export', path.join(root, 'static-content'), ['-out', path.join(root, 'site'), '-css', path.join(root, 'app.css'), '-base-path', '/reading/'], moduleDir);
  const testOut = await command('selected-tests', go, ['test', '-mod=readonly', '-run', '^(TestExportStaticContent|TestStaticExport)', '-count=1', '-json', './apps/gallery', './apps/gallery/cmd/static-content'], moduleDir);
  let skips = 0, failures = 0;
  const passedTests = new Set();
  for (const line of testOut.split('\n').filter(Boolean)) {
    const event = JSON.parse(line);
    if (event.Action === 'skip') skips++;
    if (event.Action === 'fail') failures++;
    if (event.Action === 'pass' && event.Test && !event.Test.includes('/')) passedTests.add(event.Test);
  }
  if (failures || skips) throw Error(`selected tests reported ${failures} failures and ${skips} skips`);
  const expectedTests = ['TestExportStaticContentDeep404UsesSelectedBasePath', 'TestExportStaticContentIsDeterministicAndClosed',
    'TestExportStaticContentRefusesInputsAtomically', 'TestExportStaticContentRefusesInvalidBasePathsBeforeCreation',
    'TestStaticExportCommandHelpAndDelivery', 'TestStaticExportCommandRefusesBeforeCreatingOutput',
    'TestStaticExportValidatorRejectsBrokenOrExecutableDocuments'];
  const observedTests = [...passedTests].sort();
  if (JSON.stringify(observedTests) !== JSON.stringify(expectedTests.sort())) throw Error(`selected test set disagrees: ${observedTests.join(',')}`);
  result.tests = {skips, failures, passed: observedTests};
  result.artifacts = {};
  for (const name of ['static-content', 'app.css']) result.artifacts[name] = {bytes: fs.statSync(path.join(root, name)).size, sha256: fileHash(path.join(root, name))};
  result.site = fs.readdirSync(path.join(root, 'site')).sort().map(name => ({name, bytes: fs.statSync(path.join(root, 'site', name)).size, sha256: fileHash(path.join(root, 'site', name))}));
  result.passed = true;
} catch (error) {
  result.error = error.message;
  process.exitCode = 1;
} finally {
  result.tools_after = {go_sha256: fileHash(go), tailwind_sha256: fileHash(tailwind),
    compile_sha256: fileHash(closureReceipt.toolchain.compile.path), link_sha256: fileHash(closureReceipt.toolchain.link.path)};
  if (result.tools_after.go_sha256 !== expectedGoSHA || result.tools_after.tailwind_sha256 !== expectedTailwindSHA ||
      result.tools_after.compile_sha256 !== closureReceipt.toolchain.compile.sha256 || result.tools_after.link_sha256 !== closureReceipt.toolchain.link.sha256) {
    result.passed = false; result.error = 'tool identity changed'; process.exitCode = 1;
  }
  fs.writeFileSync(path.join(root, 'run-receipt.json'), JSON.stringify(result, null, 2) + '\n', {flag: 'wx', mode: 0o600});
}
