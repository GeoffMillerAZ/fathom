// Local macOS rehearsal only. No install, signed-in browser, network or app data.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { spawn } from 'node:child_process';
import * as fs from 'node:fs/promises';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const [kitArg, appkitArg, goArg, proxyArg] = process.argv.slice(2);
assert.equal(process.argv.length, 6, 'usage: node delivery-proof.mjs KIT APPKIT_BINARY GO_BINARY PUBLIC_MODULE_ZIP_CACHE');
assert.equal(process.platform, 'darwin', 'this proof uses macOS sandbox-exec');
for (const p of [kitArg, appkitArg, goArg, proxyArg]) assert(path.isAbsolute(p), 'absolute explicit inputs required');
const [kit, appkit, go, proxy] = await Promise.all([kitArg, appkitArg, goArg, proxyArg].map(p => fs.realpath(p)));
const fixture = path.dirname(fileURLToPath(import.meta.url));
const workspace = await fs.mkdtemp(path.join(await fs.realpath(os.tmpdir()), 'pricing-delivery-'));
await fs.chmod(workspace, 0o700);
console.log(`Private delivery proof workspace: ${workspace}`);
const sha = b => createHash('sha256').update(b).digest('hex');
const hash = async p => sha(await fs.readFile(p));
const write = (p, body) => fs.writeFile(p, body, { flag: 'wx', mode: 0o600 });
const commands = [];

async function command(label, binary, args, { cwd = workspace, env = process.env, expected = 0 } = {}) {
  const started = Date.now();
  const result = await new Promise((resolve, reject) => {
    const child = spawn(binary, args, { cwd, env, stdio: ['ignore', 'pipe', 'pipe'] });
    const stdout = [], stderr = []; let size = 0, exceeded = false;
    const collect = bucket => data => {
      size += data.length;
      if (size > 2 ** 20) { exceeded = true; child.kill('SIGKILL'); return; }
      bucket.push(data);
    };
    child.stdout.on('data', collect(stdout)); child.stderr.on('data', collect(stderr));
    const timer = setTimeout(() => child.kill('SIGKILL'), 240_000);
    child.on('error', error => { clearTimeout(timer); reject(error); });
    child.on('close', (code, signal) => {
      clearTimeout(timer);
      resolve({ code, signal, exceeded, stdout: Buffer.concat(stdout), stderr: Buffer.concat(stderr) });
    });
  });
  await write(path.join(workspace, `${label}.stdout`), result.stdout);
  await write(path.join(workspace, `${label}.stderr`), result.stderr);
  commands.push({ label, binary, args, cwd, code: result.code, signal: result.signal,
    milliseconds: Date.now() - started, stdout_sha256: sha(result.stdout), stderr_sha256: sha(result.stderr) });
  assert(!result.exceeded && result.signal === null, `${label}: terminated; inspect private logs`);
  if (expected === 'failure') assert.notEqual(result.code, 0, `${label}: negative canary falsely passed`);
  else assert.equal(result.code, expected, `${label}: unexpected exit; inspect private logs`);
  return result;
}

const archive = 'data/design-assets/archives/8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675.zip';
const sourcePaths = ['data/design-assets/catalog.json', archive];
const sourceBefore = await Promise.all(sourcePaths.map(async p => ({ path: p, sha256: await hash(path.join(kit, p)) })));
const executableBefore = await Promise.all([appkit, go].map(async p => ({ path: p, sha256: await hash(p) })));
const referenceHashes = [
  { path: 'data/design-assets/pricing-typed.JpO2uS/final/baseline.html', sha256: 'b5bd30cf8544d5fd978727e5f13a22ed2d786105602aeaec423e8d3fe1a0695d' },
  { path: 'data/design-assets/pricing-typed.JpO2uS/final/enhanced.html', sha256: '891be428144fe1b9d30f70cd0d6516b3aa36f329712b4b7d97b017d962c69f93' },
];
for (const input of referenceHashes) assert.equal(await hash(path.join(kit, input.path)), input.sha256, 'retained oracle drifted');
const delivery = path.join(workspace, 'delivery');
let server;
try {
  await command('export', appkit, ['data', 'assets-adapt-pricing', '-root', path.join(kit, 'data/design-assets'), '-out', delivery]);
  await command('check-before', appkit, ['data', 'assets-adapt-pricing', '-root', path.join(kit, 'data/design-assets'), '-check', delivery]);
  const names = (await fs.readdir(delivery)).sort();
  assert.deepEqual(names, ['README.md', 'artifact.json', 'defaults.json', 'pricing.css', 'pricing.go', 'pricing.templ']);
  const delivered = await Promise.all(names.map(async name => ({ path: name,
    bytes: (await fs.stat(path.join(delivery, name))).size, sha256: await hash(path.join(delivery, name)) })));
  const fixtureNames = ['consumer.mod.txt', 'consumer.sum.txt', 'consumer_test.go.txt', 'delivery-proof.mjs'];
  const fixtureBodies = await Promise.all(fixtureNames.map(name => fs.readFile(path.join(fixture, name))));
  const inputHashes = fixtureNames.map((name, i) => ({ path: name, sha256: sha(fixtureBodies[i]) }));
  const [goMod, goSum, consumerTest] = fixtureBodies;
  assert(!/^replace\b/m.test(goMod.toString()) && !goMod.toString().includes('app-kit'), 'consumer cannot depend on app-kit or replaces');
  const profile = `(version 1) (allow default) (deny network*) (deny file-read* (subpath ${JSON.stringify(path.dirname(kit))}) (subpath ${JSON.stringify(delivery)}))`;
  await write(path.join(workspace, 'sandbox.sb'), profile);
  server = http.createServer((req, res) => { res.writeHead(200, { 'Content-Type': 'text/plain' }); res.end('owned isolation canary'); });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const canaryURL = `http://127.0.0.1:${server.address().port}/`;
  assert.equal((await fetch(canaryURL)).status, 200, 'unsandboxed canary not reachable');
  const proofRuns = [];
  for (const name of ['first', 'replay']) {
    const run = path.join(workspace, name);
    await fs.mkdir(run, { mode: 0o700 });
    const consumer = path.join(run, 'consumer');
    await fs.mkdir(consumer, { mode: 0o700 });
    await fs.mkdir(path.join(consumer, 'converted'), { mode: 0o700 });
    for (const file of names) await write(path.join(consumer, 'converted', file), await fs.readFile(path.join(delivery, file)));
    await write(path.join(consumer, 'go.mod'), goMod);
    await write(path.join(consumer, 'go.sum'), goSum);
    await write(path.join(consumer, 'consumer_test.go'), consumerTest);
    const env = { PATH: `${path.dirname(go)}:/usr/bin:/bin`, GOENV: 'off', GOWORK: 'off', GOTOOLCHAIN: 'local',
      CGO_ENABLED: '0', GOVCS: '*:off', GOFLAGS: '-mod=readonly', GOSUMDB: 'sum.golang.org',
      GOPROXY: pathToFileURL(proxy).href, GOPATH: path.join(run, 'gopath'), GOMODCACHE: path.join(run, 'modules'),
      GOCACHE: path.join(run, 'build-cache'), GOTMPDIR: path.join(run, 'tmp') };
    await fs.mkdir(env.GOTMPDIR, { mode: 0o700 });
    // Both caches really start absent. No ambient GOENV, workspace or credentials.
    for (const cache of [env.GOMODCACHE, env.GOCACHE]) await assert.rejects(fs.stat(cache), { code: 'ENOENT' });
    const isolated = (label, binary, args, expected = 0) => command(`${name}-${label}`, '/usr/bin/sandbox-exec', ['-p', profile, binary, ...args], { cwd: consumer, env, expected });
    for (const [label, forbidden] of [['sibling', path.join(kit, 'go.mod')], ['original-delivery', path.join(delivery, 'pricing.go')]]) {
      const result = await isolated(label, '/bin/cat', [forbidden], 'failure');
      assert(result.stderr.toString().includes('Operation not permitted'), 'file denial was not enforced');
    }
    await isolated('network', '/usr/bin/curl', ['--silent', '--show-error', '--max-time', '2', canaryURL], 'failure');
    await isolated('go-version', go, ['version']);
    await isolated('templ', go, ['tool', 'templ', 'generate', '-path', 'converted']);
    await isolated('build', go, ['test', '-c', '-trimpath', '-buildvcs=false', '-o', 'consumer.test', '.']);
    const runtime = await isolated('runtime', path.join(consumer, 'consumer.test'), ['-test.run', '^TestDeliveredPricingConsumer$', '-test.v']);
    assert(runtime.stdout.toString().includes('--- PASS: TestDeliveredPricingConsumer'), 'named runtime test did not execute');
    await isolated('verify-modules', go, ['mod', 'verify']);
    // Inspect dependencies actually used by the component tests AND compiler.
    // `go list -m all` additionally fetches unused transitive graph metadata;
    // that is not required to compile this pinned package/tool offline.
    const moduleRows = [];
    for (const [label, target] of [['consumer', '.'], ['compiler', 'github.com/a-h/templ/cmd/templ']]) {
      const modules = await isolated(`${label}-modules`, go, ['list', '-deps', '-test', '-f',
        '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}|{{if .Module.Replace}}replace{{end}}{{end}}', target]);
      const rows = [...new Set(modules.stdout.toString().trim().split('\n').filter(Boolean))];
      assert(rows.length > 0, 'no module resolution evidence');
      for (const row of rows) {
        const [modulePath, version, directory, replacement] = row.split('|');
        assert.equal(replacement, '', 'dependency replacement is forbidden');
        assert(!modulePath.includes('github.com/geoffmilleraz/'), 'fleet module in standalone consumer');
        assert(directory === consumer || directory.startsWith(env.GOMODCACHE + path.sep), 'module resolved outside isolated inputs');
        moduleRows.push({ used_by: label, path: modulePath, version, directory });
      }
    }
    assert.equal(await hash(path.join(consumer, 'go.mod')), sha(goMod), 'readonly go.mod changed');
    assert.equal(await hash(path.join(consumer, 'go.sum')), sha(goSum), 'readonly go.sum changed');
    const outputs = await Promise.all(['converted/pricing_templ.go', 'baseline.html', 'enhanced.html', 'consumer.test'].map(async p => ({ path: p,
      bytes: (await fs.stat(path.join(consumer, p))).size, sha256: await hash(path.join(consumer, p)) })));
    proofRuns.push({ name, env, outputs, resolved_modules: moduleRows });
    // Hide generated code recoverably. A compile exit alone is not a passing test.
    await fs.rename(path.join(consumer, 'converted/pricing_templ.go'), path.join(run, 'pricing_templ.go.withheld'));
    const negative = await isolated('missing-generated-code', go, ['test', '-c', '-trimpath', '-buildvcs=false', '-o', 'must-not-build', '.'], 'failure');
    assert(negative.stderr.toString().includes('undefined:'), 'missing-code canary failed for another reason');
    await assert.rejects(fs.stat(path.join(consumer, 'must-not-build')), { code: 'ENOENT' });
    await fs.rename(path.join(run, 'pricing_templ.go.withheld'), path.join(consumer, 'converted/pricing_templ.go'));
  }
  assert.deepEqual(proofRuns[0].outputs, proofRuns[1].outputs, 'cold builds did not reproduce outputs');
  // An earlier independently compiled paid-source probe is a byte oracle, not a
  // browser rerun. Do not rewrite these files or relax comparison on a failure.
  for (const input of referenceHashes) {
    assert.equal(await hash(path.join(workspace, 'first/consumer', path.basename(input.path))), input.sha256, 'render differs from retained prior probe');
    assert.equal(await hash(path.join(kit, input.path)), input.sha256, 'oracle changed during proof');
  }
  await command('check-after', appkit, ['data', 'assets-adapt-pricing', '-root', path.join(kit, 'data/design-assets'), '-check', delivery]);
  for (const input of sourceBefore) assert.equal(await hash(path.join(kit, input.path)), input.sha256, 'original source/catalog changed');
  for (const input of executableBefore) assert.equal(await hash(input.path), input.sha256, 'executable changed during proof');
  for (const input of inputHashes) assert.equal(await hash(path.join(fixture, input.path)), input.sha256, 'fixture changed during proof');
  const receipt = { schema_version: 1, kind: 'private-p02-clean-consumer-rehearsal', verification: 'not-certified',
    source_before_after: sourceBefore, delivered, input_hashes: inputHashes, executables_before_after: executableBefore, reference_hashes: referenceHashes,
    sandbox_profile_sha256: sha(profile), unsandboxed_network_canary: 200, commands, runs: proofRuns,
    limits: ['synthetic task only', 'macOS local host only', 'no CSS compilation or browser rerun',
      'GOSUMDB enabled but network denied: cached ZIPs checked against pinned go.sum, not live sumdb consultation',
      'actual-used consumer/compiler dependency closure, not complete transitive graph download',
      'not a release, actual app adoption or target certificate'] };
  await write(path.join(workspace, 'receipt.json'), JSON.stringify(receipt, null, 2) + '\n');
  console.log(JSON.stringify({ result: 'passed', workspace, receipt_sha256: await hash(path.join(workspace, 'receipt.json')), runs: 2, rendered_variants: 2 }));
} catch (error) {
  await write(path.join(workspace, 'failure.json'), JSON.stringify({ error: String(error), commands }, null, 2) + '\n');
  console.error(`Delivery proof failed; inspect private ${path.join(workspace, 'failure.json')}`);
  process.exitCode = 1;
} finally {
  if (server) { server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); }
}
