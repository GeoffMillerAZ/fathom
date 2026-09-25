// Private macOS P09 repair-delivery rehearsal. No install, profile or app data.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { spawn } from 'node:child_process';
import * as fs from 'node:fs/promises';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const [kitArg, deliveryArg, deliveryReceiptSHA, repairProbeArg, compileReceiptSHA, goArg, proxyArg, appkitArg] = process.argv.slice(2);
assert.equal(process.argv.length, 10, 'usage: node repair-standalone.mjs KIT DELIVERY DELIVERY_RECEIPT_SHA REPAIR_PROBE COMPILE_RECEIPT_SHA GO_BINARY PUBLIC_MODULE_ZIP_CACHE APPKIT_BINARY');
assert.equal(process.platform, 'darwin', 'this proof uses macOS sandbox-exec');
for (const value of [deliveryReceiptSHA, compileReceiptSHA]) assert.match(value ?? '', /^[0-9a-f]{64}$/, 'exact receipt SHA-256 required');
for (const value of [kitArg, deliveryArg, repairProbeArg, goArg, proxyArg, appkitArg]) assert(path.isAbsolute(value), 'absolute explicit inputs required');
const [kit, delivery, repairProbe, go, proxy, appkit] = await Promise.all(
  [kitArg, deliveryArg, repairProbeArg, goArg, proxyArg, appkitArg].map(value => fs.realpath(value)),
);
const tempRoot = await fs.realpath(os.tmpdir());
assert((await fs.stat(tempRoot)).isDirectory(), 'command temp root unavailable');
assert((await fs.stat(proxy)).isDirectory() && proxy.endsWith('/cache/download'), 'public module ZIP cache is not a Go file proxy');
const fixture = path.dirname(fileURLToPath(import.meta.url));
const deliveryReceiptPath = await fs.realpath(path.join(path.dirname(delivery), 'delivery-receipt.json'));
const compileReceiptPath = await fs.realpath(path.join(repairProbe, 'compile-receipt.json'));
const typedOracle = await fs.realpath(path.join(repairProbe, 'repair-typed.html'));
const minimalOracle = await fs.realpath(path.join(repairProbe, 'repair-minimal.html'));
const originalRuntime = await fs.realpath(path.join(kit, 'data/design-assets/reference-assets/elements-1.0.22-pilot/elements.js'));
for (const binary of [appkit, go]) {
  const stat = await fs.stat(binary);
  assert(stat.isFile() && (stat.mode & 0o111) !== 0, 'input executable is not a regular executable');
}

const workspace = await fs.mkdtemp(path.join(tempRoot, 'navbar-repair-standalone-'));
await fs.chmod(workspace, 0o700);
console.log(`Private P09 repair standalone workspace: ${workspace}`);
const sha = body => createHash('sha256').update(body).digest('hex');
const read = filename => fs.readFile(filename);
const hash = async filename => sha(await read(filename));
const write = (filename, body) => fs.writeFile(filename, body, { flag: 'wx', mode: 0o600 });
const commands = [];

async function command(label, binary, args, { cwd = workspace, env = {}, expected = 0 } = {}) {
  const started = Date.now();
  const result = await new Promise((resolve, reject) => {
    const child = spawn(binary, args, { cwd, env, stdio: ['ignore', 'pipe', 'pipe'], detached: true });
    const stdout = [], stderr = [];
    let size = 0, exceeded = false, timedOut = false;
    const terminate = () => { try { process.kill(-child.pid, 'SIGKILL'); } catch (error) { if (error.code !== 'ESRCH') throw error; } };
    const collect = bucket => chunk => {
      size += chunk.length;
      if (size > 1 << 20) { exceeded = true; terminate(); return; }
      bucket.push(chunk);
    };
    child.stdout.on('data', collect(stdout));
    child.stderr.on('data', collect(stderr));
    const timer = setTimeout(() => { timedOut = true; terminate(); }, 240_000);
    child.once('error', error => { clearTimeout(timer); reject(error); });
    child.once('close', (code, signal) => {
      clearTimeout(timer);
      resolve({ code, signal, exceeded, timedOut, stdout: Buffer.concat(stdout), stderr: Buffer.concat(stderr) });
    });
  });
  await write(path.join(workspace, `${label}.stdout`), result.stdout);
  await write(path.join(workspace, `${label}.stderr`), result.stderr);
  commands.push({ label, binary, args, cwd, code: result.code, signal: result.signal,
    milliseconds: Date.now() - started, stdout_sha256: sha(result.stdout), stderr_sha256: sha(result.stderr) });
  assert(!result.exceeded && !result.timedOut && result.signal === null, `${label}: terminated; inspect private logs`);
  if (expected === 'failure') assert.notEqual(result.code, 0, `${label}: negative canary falsely passed`);
  else assert.equal(result.code, expected, `${label}: unexpected exit; inspect private logs`);
  return result;
}

const deliveredNames = (await fs.readdir(delivery)).sort();
assert.deepEqual(deliveredNames, ['USAGE.md', 'artifact.json', 'defaults.json', 'elements-navbar.js', 'navbar.go', 'navbar.templ', 'navbar_repair.css', 'navbar_routes.go', 'navbar_routes.templ']);
assert.equal(await hash(deliveryReceiptPath), deliveryReceiptSHA, 'delivery receipt identity differs');
assert.equal(await hash(compileReceiptPath), compileReceiptSHA, 'compile receipt identity differs');
const deliveryReceipt = JSON.parse(await read(deliveryReceiptPath));
const compileReceipt = JSON.parse(await read(compileReceiptPath));
assert.equal(deliveryReceipt.kind, 'private-p09-navbar-repair-cli-delivery');
assert.equal(deliveryReceipt.verification, 'not-verified');
assert.deepEqual(Object.keys(deliveryReceipt.files).sort(), deliveredNames);
for (const name of deliveredNames) assert.equal(await hash(path.join(delivery, name)), deliveryReceipt.files[name], `delivery receipt does not bind ${name}`);
assert.equal(deliveryReceipt.artifact_sha256, await hash(path.join(delivery, 'artifact.json')));
assert.equal(compileReceipt.artifact_sha256, deliveryReceipt.paired_repair_sha256, 'compiler and delivery pair different repair artifacts');
assert.equal(compileReceipt.rendered_sha256?.['repair-typed.html'], await hash(typedOracle), 'typed oracle identity differs');
assert.equal(compileReceipt.rendered_sha256?.['repair-minimal.html'], await hash(minimalOracle), 'minimal oracle identity differs');
assert.equal(deliveryReceipt.runtime_sha256, await hash(path.join(delivery, 'elements-navbar.js')));
assert.equal(deliveryReceipt.runtime_sha256, '7c7184f9ea5ba3e4842f8624f500cb80d81c3c8621570715cfbd69ecdf1d7179');
assert.equal(await hash(originalRuntime), '120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013');

const trackedDelivery = await Promise.all(deliveredNames.map(async name => ({ path: name, sha256: await hash(path.join(delivery, name)) })));
const trackedProbe = await Promise.all(['compile-receipt.json', 'repair-typed.html', 'repair-minimal.html'].map(async name => ({ path: name, sha256: await hash(path.join(repairProbe, name)) })));
const archive = 'data/design-assets/archives/b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8.zip';
const kitPaths = ['go.mod', 'go.sum', 'data/design-assets/catalog.json', archive, 'data/design-assets/reference-assets/elements-1.0.22-pilot/elements.js'];
const trackedKit = await Promise.all(kitPaths.map(async name => ({ path: name, sha256: await hash(path.join(kit, name)) })));
assert.equal(trackedKit[3].sha256, 'b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8');
const trackedTools = await Promise.all([go, appkit].map(async filename => ({ path: filename, sha256: await hash(filename) })));

const fixtureNames = ['repair-standalone.mjs', 'repair-consumer_test.go.txt', '../pricing-adapter/consumer.mod.txt', '../pricing-adapter/consumer.sum.txt'];
const fixtureBodies = await Promise.all(fixtureNames.map(name => read(path.join(fixture, name))));
const fixtureHashes = fixtureNames.map((name, index) => ({ path: name, sha256: sha(fixtureBodies[index]) }));
const sourceMod = fixtureBodies[2].toString();
assert.equal(sourceMod.match(/^module example\.invalid\/pricing-proof$/gm)?.length, 1, 'pricing module fixture identity changed');
const goMod = Buffer.from(sourceMod.replace('module example.invalid/pricing-proof', 'module example.invalid/navbar-repair-proof'));
const goSum = fixtureBodies[3];
const consumerTest = fixtureBodies[1];
assert(!/^replace\b/m.test(goMod.toString()) && !goMod.toString().includes('app-kit'), 'consumer cannot depend on app-kit or replacements');

let canary, receipt;
try {
  const checkArgs = ['data', 'assets-adapt-navbar', '-root', path.join(kit, 'data/design-assets'), '-repair', '-runtime', originalRuntime, '-check', delivery];
  const before = await command('check-before', appkit, checkArgs);
  assert.equal(before.stdout.length + before.stderr.length, 0, 'successful delivery check was not silent');

  canary = http.createServer((request, response) => { response.writeHead(200, { 'Content-Type': 'text/plain' }); response.end('owned isolation canary'); });
  await new Promise((resolve, reject) => { canary.once('error', reject); canary.listen(0, '127.0.0.1', resolve); });
  const canaryURL = `http://127.0.0.1:${canary.address().port}/`;
  const reachable = await fetch(canaryURL, { signal: AbortSignal.timeout(2000) });
  assert.equal(reachable.status, 200, 'owned unsandboxed network canary unavailable');

  const profile = `(version 1) (allow default) (deny network*) (deny file-read* (subpath ${JSON.stringify(path.dirname(kit))}) (subpath ${JSON.stringify(delivery)}) (subpath ${JSON.stringify(repairProbe)})) (deny file-write* (subpath ${JSON.stringify(proxy)}))`;
  await write(path.join(workspace, 'sandbox.sb'), profile);
  const runs = [];
  let goVersion;
  for (const name of ['first', 'replay']) {
    const run = path.join(workspace, name);
    const consumer = path.join(run, 'consumer');
    const converted = path.join(consumer, 'converted');
    await fs.mkdir(converted, { recursive: true, mode: 0o700 });
    await fs.chmod(run, 0o700); await fs.chmod(consumer, 0o700);
    for (const file of deliveredNames) await write(path.join(converted, file), await read(path.join(delivery, file)));
    await write(path.join(consumer, 'repair-typed-oracle.html'), await read(typedOracle));
    await write(path.join(consumer, 'repair-minimal-oracle.html'), await read(minimalOracle));
    await write(path.join(consumer, 'go.mod'), goMod);
    await write(path.join(consumer, 'go.sum'), goSum);
    await write(path.join(consumer, 'consumer_test.go'), consumerTest);
    const env = {
      PATH: `${path.dirname(go)}:/usr/bin:/bin`, GOENV: 'off', GOWORK: 'off', GOTOOLCHAIN: 'local', CGO_ENABLED: '0',
      GOVCS: '*:off', GOFLAGS: '-mod=readonly', GOSUMDB: 'sum.golang.org', GOPROXY: pathToFileURL(proxy).href,
      GOPATH: path.join(run, 'gopath'), GOMODCACHE: path.join(run, 'modules'), GOCACHE: path.join(run, 'build-cache'), GOTMPDIR: path.join(run, 'tmp'),
    };
    await fs.mkdir(env.GOTMPDIR, { mode: 0o700 });
    for (const cache of [env.GOMODCACHE, env.GOCACHE]) await assert.rejects(fs.stat(cache), { code: 'ENOENT' });
    const isolated = (label, binary, args, expected = 0) => command(`${name}-${label}`, '/usr/bin/sandbox-exec', ['-p', profile, binary, ...args], { cwd: consumer, env, expected });
    for (const [label, forbidden] of [['kit-source', path.join(kit, 'go.mod')], ['pristine-delivery', path.join(delivery, 'navbar.go')], ['compile-oracle', typedOracle]]) {
      const denied = await isolated(label, '/bin/cat', [forbidden], 'failure');
      assert(denied.stderr.toString().includes('Operation not permitted'), `${label}: file denial was not enforced`);
    }
    await isolated('network', '/usr/bin/curl', ['--silent', '--show-error', '--max-time', '2', canaryURL], 'failure');
    const version = await isolated('go-version', go, ['version']);
    const observedVersion = version.stdout.toString().trim();
    assert.match(observedVersion, /^go version go[0-9.]+ darwin\/arm64$/, 'unexpected Go tool identity');
    if (goVersion === undefined) goVersion = observedVersion;
    else assert.equal(observedVersion, goVersion, 'Go tool identity changed');
    await isolated('templ', go, ['tool', 'templ', 'generate', '-path', 'converted']);
    await isolated('build', go, ['test', '-c', '-trimpath', '-buildvcs=false', '-o', 'consumer.test', '.']);
    const runtime = await isolated('runtime', path.join(consumer, 'consumer.test'), ['-test.run', '^TestDeliveredNavbarRepairConsumer$', '-test.v']);
    assert(runtime.stdout.toString().includes('--- PASS: TestDeliveredNavbarRepairConsumer'), 'named consumer task did not pass');
    await isolated('verify-modules', go, ['mod', 'verify']);

    const moduleRows = [];
    for (const [label, target] of [['consumer', '.'], ['renderer', './converted'], ['compiler', 'github.com/a-h/templ/cmd/templ']]) {
      const listed = await isolated(`${label}-modules`, go, ['list', '-deps', '-test', '-f', '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}|{{if .Module.Replace}}replace{{end}}{{end}}', target]);
      const rows = [...new Set(listed.stdout.toString().trim().split('\n').filter(Boolean))];
      assert(rows.length > 0, `${label}: no used module evidence`);
      for (const row of rows) {
        const [modulePath, version, directory, replacement] = row.split('|');
        assert.equal(replacement, '', 'module replacement is forbidden');
        assert(!modulePath.includes('github.com/geoffmilleraz/'), 'fleet module in standalone closure');
        assert(directory === consumer || directory.startsWith(env.GOMODCACHE + path.sep), 'module resolved outside isolated inputs');
        moduleRows.push({ used_by: label, path: modulePath, version, directory });
      }
    }
    assert.equal(await hash(path.join(consumer, 'go.mod')), sha(goMod), 'readonly go.mod changed');
    assert.equal(await hash(path.join(consumer, 'go.sum')), sha(goSum), 'readonly go.sum changed');
    for (const file of deliveredNames) assert.equal(await hash(path.join(converted, file)), deliveryReceipt.files[file], `delivered ${file} changed`);
    const outputNames = ['converted/navbar_templ.go', 'converted/navbar_routes_templ.go', 'repair-typed.html', 'repair-minimal.html', 'consumer.test'];
    const outputs = await Promise.all(outputNames.map(async filename => ({ path: filename, bytes: (await fs.stat(path.join(consumer, filename))).size, sha256: await hash(path.join(consumer, filename)) })));
    runs.push({ name, outputs, copied_delivery: await Promise.all(deliveredNames.map(async filename => ({ path: filename, sha256: await hash(path.join(converted, filename)) }))), resolved_modules: moduleRows });

    await fs.rename(path.join(converted, 'navbar_routes_templ.go'), path.join(run, 'navbar_routes_templ.go.withheld'));
    const negative = await isolated('missing-generated-code', go, ['test', '-c', '-trimpath', '-buildvcs=false', '-o', 'must-not-build', '.'], 'failure');
    assert(negative.stderr.toString().includes('undefined:'), 'missing-generated-file canary failed for another reason');
    await assert.rejects(fs.stat(path.join(consumer, 'must-not-build')), { code: 'ENOENT' });
    await fs.rename(path.join(run, 'navbar_routes_templ.go.withheld'), path.join(converted, 'navbar_routes_templ.go'));
  }
  assert.deepEqual(runs[0].outputs, runs[1].outputs, 'cold builds did not reproduce generated, rendered and binary outputs');
  assert.deepEqual(runs[0].copied_delivery, runs[1].copied_delivery, 'cold copies did not retain CSS/runtime/delivery bytes');

  const after = await command('check-after', appkit, checkArgs);
  assert.equal(after.stdout.length + after.stderr.length, 0, 'final delivery check was not silent');
  for (const input of trackedDelivery) assert.equal(await hash(path.join(delivery, input.path)), input.sha256, `delivery changed: ${input.path}`);
  for (const input of trackedProbe) assert.equal(await hash(path.join(repairProbe, input.path)), input.sha256, `compiler proof changed: ${input.path}`);
  for (const input of trackedKit) assert.equal(await hash(path.join(kit, input.path)), input.sha256, `private kit input changed: ${input.path}`);
  for (const input of trackedTools) assert.equal(await hash(input.path), input.sha256, 'tool input changed');
  for (const input of fixtureHashes) assert.equal(await hash(path.join(fixture, input.path)), input.sha256, `fixture changed: ${input.path}`);

  receipt = {
    schema_version: 1, kind: 'private-p09-navbar-repair-standalone-consumer-rehearsal', verification: 'not-certified',
    delivery_receipt_sha256: deliveryReceiptSHA, compile_receipt_sha256: compileReceiptSHA,
    delivery_before_after: trackedDelivery, compiler_proof_before_after: trackedProbe, source_before_after: trackedKit,
    fixture_hashes: fixtureHashes, tool_before_after: trackedTools, tool_versions: { node: process.version, go: goVersion },
    derived_go_mod_sha256: sha(goMod), consumer_test_sha256: sha(consumerTest), sandbox_profile_sha256: sha(profile),
    owned_unsandboxed_network_canary: 200, commands, runs,
    limits: ['synthetic render workflow only', 'macOS sandbox-exec only', 'CSS and runtime bytes retained but not served in a browser',
      'GOSUMDB configured but network denied; cached public ZIPs checked against pinned go.sum',
      'public ZIP cache write denial configured, not exercised against the owner cache',
      'actual-used consumer/compiler module closure, not complete transitive graph',
      'no release, real consumer adoption, visual/accessibility proof or target certification'],
  };
} catch (error) {
  await write(path.join(workspace, 'failure.json'), JSON.stringify({ error: String(error), commands }, null, 2) + '\n');
  console.error(`P09 repair standalone proof failed; inspect private ${path.join(workspace, 'failure.json')}`);
  process.exitCode = 1;
} finally {
  if (canary) {
    canary.closeAllConnections();
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('owned canary close timeout')), 5000);
      canary.close(error => { clearTimeout(timer); if (error) reject(error); else resolve(); });
    });
  }
  if (receipt && !process.exitCode) {
    receipt.cleanup = 'owned canary closed';
    await write(path.join(workspace, 'receipt.json'), JSON.stringify(receipt, null, 2) + '\n');
    console.log(JSON.stringify({ result: 'passed', workspace, receipt_sha256: await hash(path.join(workspace, 'receipt.json')), runs: 2, rendered_variants: 2 }));
  }
}
