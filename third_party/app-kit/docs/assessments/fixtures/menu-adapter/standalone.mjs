// Private macOS P07 consumer rehearsal. No install, signed-in profile or app data.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { spawn } from 'node:child_process';
import * as fs from 'node:fs/promises';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const [kitArg, probeArg, compileReceiptSHA, goArg, proxyArg] = process.argv.slice(2);
assert.equal(process.argv.length, 7, 'usage: node standalone.mjs KIT P07_PROBE COMPILE_RECEIPT_SHA256 GO_BINARY PUBLIC_MODULE_ZIP_CACHE');
assert.equal(process.platform, 'darwin', 'this proof uses macOS sandbox-exec');
assert.match(compileReceiptSHA ?? '', /^[0-9a-f]{64}$/, 'exact compile receipt SHA-256 required');
for (const p of [kitArg, probeArg, goArg, proxyArg]) assert(path.isAbsolute(p), 'absolute explicit inputs required');
const [kit, probe, go, proxy] = await Promise.all([kitArg, probeArg, goArg, proxyArg].map(p => fs.realpath(p)));
const tempRoot = await fs.realpath(os.tmpdir());
assert(probe.startsWith(tempRoot + path.sep), 'P07 probe must be a command-temp child');
assert((await fs.stat(proxy)).isDirectory() && proxy.endsWith('/cache/download'), 'public module ZIP cache is not a Go file proxy');
const fixture = path.dirname(fileURLToPath(import.meta.url));
const appkit = await fs.realpath(path.join(probe, 'appkit'));
const delivery = await fs.realpath(path.join(probe, 'delivery'));
const oracle = await fs.realpath(path.join(probe, 'typed.html'));
const compileReceiptPath = await fs.realpath(path.join(probe, 'compile-receipt.json'));
for (const binary of [appkit, go]) {
  const stat = await fs.stat(binary);
  assert(stat.isFile() && (stat.mode & 0o111) !== 0, 'input executable is not a regular executable');
}
const oracleStat = await fs.lstat(oracle);
assert(oracleStat.isFile() && oracleStat.nlink === 1 && oracleStat.size > 0 && oracleStat.size <= 1 << 20, 'typed render oracle is invalid');

const workspace = await fs.mkdtemp(path.join(tempRoot, 'menu-standalone-'));
await fs.chmod(workspace, 0o700);
console.log(`Private P07 standalone workspace: ${workspace}`);
const sha = body => createHash('sha256').update(body).digest('hex');
const read = p => fs.readFile(p);
const hash = async p => sha(await read(p));
const write = (p, body) => fs.writeFile(p, body, { flag: 'wx', mode: 0o600 });
const commands = [];

async function command(label, binary, args, { cwd = workspace, env = {}, expected = 0 } = {}) {
  const started = Date.now();
  const result = await new Promise((resolve, reject) => {
    const child = spawn(binary, args, { cwd, env, stdio: ['ignore', 'pipe', 'pipe'] });
    const stdout = [], stderr = [];
    let size = 0, exceeded = false, timedOut = false;
    const collect = bucket => chunk => {
      size += chunk.length;
      if (size > 1 << 20) { exceeded = true; child.kill('SIGKILL'); return; }
      bucket.push(chunk);
    };
    child.stdout.on('data', collect(stdout));
    child.stderr.on('data', collect(stderr));
    const timer = setTimeout(() => { timedOut = true; child.kill('SIGKILL'); }, 240_000);
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
assert.deepEqual(deliveredNames, ['USAGE.md', 'artifact.json', 'defaults.json', 'menu.css', 'menu.go', 'menu.templ']);
assert.equal(await hash(compileReceiptPath), compileReceiptSHA, 'compile receipt does not match the explicit identity');
const compileReceipt = JSON.parse(await read(compileReceiptPath));
const artifactSHA = await hash(path.join(delivery, 'artifact.json'));
const oracleSHA = await hash(oracle);
assert.equal(compileReceipt.artifact_sha256, artifactSHA, 'compile receipt does not bind the delivered artifact');
assert.equal(compileReceipt.rendered_sha256?.['typed.html'], oracleSHA, 'compile receipt does not bind the typed render oracle');
const trackedProbePaths = ['appkit', 'compile-receipt.json', 'typed.html', ...deliveredNames.map(name => `delivery/${name}`)];
const probeBefore = await Promise.all(trackedProbePaths.map(async p => ({ path: p, sha256: await hash(path.join(probe, p)) })));
const archive = 'data/design-assets/archives/b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8.zip';
const kitPaths = ['go.mod', 'go.sum', 'data/design-assets/catalog.json', archive];
const kitBefore = await Promise.all(kitPaths.map(async p => ({ path: p, sha256: await hash(path.join(kit, p)) })));
assert.equal(kitBefore[3].sha256, 'b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8');
const fixtureNames = ['standalone.mjs', '../pricing-adapter/consumer.mod.txt', '../pricing-adapter/consumer.sum.txt'];
const fixtureBodies = await Promise.all(fixtureNames.map(name => read(path.join(fixture, name))));
const fixtureHashes = fixtureNames.map((name, i) => ({ path: name, sha256: sha(fixtureBodies[i]) }));
const sourceMod = fixtureBodies[1].toString();
assert.equal(sourceMod.match(/^module example\.invalid\/pricing-proof$/gm)?.length, 1, 'pricing module fixture identity changed');
const goMod = Buffer.from(sourceMod.replace('module example.invalid/pricing-proof', 'module example.invalid/menu-proof'));
const goSum = fixtureBodies[2];
assert(!/^replace\b/m.test(goMod.toString()) && !goMod.toString().includes('app-kit'), 'consumer cannot depend on app-kit or replacements');

const consumerTest = Buffer.from(`package consumer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"example.invalid/menu-proof/converted"
)

func render(t *testing.T, component interface { Render(context.Context, io.Writer) error }) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := component.Render(context.Background(), &out); err != nil { t.Fatal("render failed") }
	return out.Bytes()
}

func writeExclusive(t *testing.T, name string, body []byte) {
	t.Helper()
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil { t.Fatal("output must be absent") }
	_, writeErr := f.Write(body)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil { t.Fatal("output failed") }
}

// This is a synthetic render task, not application adoption or certification.
func TestDeliveredMenuConsumer(t *testing.T) {
	body, err := os.ReadFile("converted/defaults.json")
	if err != nil { t.Fatal("delivered defaults missing") }
	var data converted.Menu
	if json.Unmarshal(body, &data) != nil { t.Fatal("delivery defaults invalid") }
	if len(data.Links) != 3 || data.Submit == nil { t.Fatal("delivery defaults shape changed") }
	for _, link := range data.Links {
		if link.URL != "" { t.Fatal("app-owned link URL was not deliberately blank") }
	}
	if data.Submit.Action != "" || data.Submit.CSRFName != "" || data.Submit.CSRFToken != "" {
		t.Fatal("app-owned POST and CSRF fields were not deliberately blank")
	}
	if _, err := converted.NewMenu(data); err == nil { t.Fatal("blank app-owned routes were accepted") }
	for i, route := range []string{"/profile", "/help", "/license"} { data.Links[i].URL = route }
	data.Submit.Action, data.Submit.CSRFName, data.Submit.CSRFToken = "/action", "csrf", "synthetic-menu-token"
	faithful, err := converted.NewMenu(data)
	if err != nil { t.Fatal("faithful constructor failed") }
	baseline := render(t, faithful)
	want, err := os.ReadFile("typed-oracle.html")
	if err != nil || !bytes.Equal(baseline, want) { t.Fatal("faithful render differs from exact typed oracle") }
	writeExclusive(t, "baseline.html", baseline)
	enhanced, err := converted.NewMenuEnhanced(data)
	if err != nil { t.Fatal("enhanced constructor failed") }
	enhancedBody := render(t, enhanced)
	if bytes.Count(enhancedBody, baseline) != 1 || !bytes.Contains(enhancedBody, []byte("data-appkit-menu")) {
		t.Fatal("enhanced wrapper does not contain the faithful render exactly once")
	}
	writeExclusive(t, "enhanced.html", enhancedBody)
	linksOnly := data
	linksOnly.Submit = nil
	linksOnlyComponent, err := converted.NewMenu(linksOnly)
	if err != nil { t.Fatal("links-only constructor failed") }
	if body := render(t, linksOnlyComponent); bytes.Contains(body, []byte("<form")) {
		t.Fatal("links-only model rendered a form")
	}
	disabled := data
	submit := *data.Submit
	submit.Disabled = true
	disabled.Submit = &submit
	disabledComponent, err := converted.NewMenu(disabled)
	if err != nil { t.Fatal("disabled constructor failed") }
	if body := render(t, disabledComponent); !bytes.Contains(body, []byte(" disabled")) {
		t.Fatal("disabled submit did not render disabled")
	}
}
`);

let canary;
try {
  const checkArgs = ['data', 'assets-adapt-menu', '-root', path.join(kit, 'data/design-assets'), '-check', delivery];
  const before = await command('check-before', appkit, checkArgs);
  assert.equal(before.stdout.length + before.stderr.length, 0, 'successful delivery check was not silent');

  canary = http.createServer((req, res) => { res.writeHead(200, { 'Content-Type': 'text/plain' }); res.end('owned isolation canary'); });
  await new Promise((resolve, reject) => { canary.once('error', reject); canary.listen(0, '127.0.0.1', resolve); });
  const canaryURL = `http://127.0.0.1:${canary.address().port}/`;
  const reachable = await fetch(canaryURL, { signal: AbortSignal.timeout(2000) });
  assert.equal(reachable.status, 200, 'owned unsandboxed canary is not reachable');

  const profile = `(version 1) (allow default) (deny network*) (deny file-read* (subpath ${JSON.stringify(path.dirname(kit))}) (subpath ${JSON.stringify(delivery)})) (deny file-write* (subpath ${JSON.stringify(proxy)}))`;
  await write(path.join(workspace, 'sandbox.sb'), profile);
  const runs = [];
  let goVersion;
  for (const name of ['first', 'replay']) {
    const run = path.join(workspace, name);
    const consumer = path.join(run, 'consumer');
    await fs.mkdir(path.join(consumer, 'converted'), { recursive: true, mode: 0o700 });
    await fs.chmod(run, 0o700); await fs.chmod(consumer, 0o700);
    for (const file of ['menu.templ', 'menu.go', 'defaults.json']) {
      await write(path.join(consumer, 'converted', file), await read(path.join(delivery, file)));
    }
    await write(path.join(consumer, 'typed-oracle.html'), await read(oracle));
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
    for (const [label, forbidden] of [['kit-source', path.join(kit, 'go.mod')], ['pristine-delivery', path.join(delivery, 'menu.go')]]) {
      const denied = await isolated(label, '/bin/cat', [forbidden], 'failure');
      assert(denied.stderr.toString().includes('Operation not permitted'), `${label}: file denial was not enforced`);
    }
    await isolated('network', '/usr/bin/curl', ['--silent', '--show-error', '--max-time', '2', canaryURL], 'failure');
    const version = await isolated('go-version', go, ['version']);
    const observedGoVersion = version.stdout.toString().trim();
    assert.match(observedGoVersion, /^go version go[0-9.]+ darwin\/arm64$/, 'unexpected Go tool identity');
    if (goVersion === undefined) goVersion = observedGoVersion;
    else assert.equal(observedGoVersion, goVersion, 'Go tool identity changed between cold runs');
    await isolated('templ', go, ['tool', 'templ', 'generate', '-path', 'converted']);
    await isolated('build', go, ['test', '-c', '-trimpath', '-buildvcs=false', '-o', 'consumer.test', '.']);
    const runtime = await isolated('runtime', path.join(consumer, 'consumer.test'), ['-test.run', '^TestDeliveredMenuConsumer$', '-test.v']);
    assert(runtime.stdout.toString().includes('--- PASS: TestDeliveredMenuConsumer'), 'named runtime test did not execute and pass');
    await isolated('verify-modules', go, ['mod', 'verify']);

    const moduleRows = [];
    for (const [label, target] of [['consumer', '.'], ['compiler', 'github.com/a-h/templ/cmd/templ']]) {
      const listed = await isolated(`${label}-modules`, go, ['list', '-deps', '-test', '-f',
        '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}|{{if .Module.Replace}}replace{{end}}{{end}}', target]);
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
    for (const file of ['menu.templ', 'menu.go', 'defaults.json']) {
      assert.equal(await hash(path.join(consumer, 'converted', file)), await hash(path.join(delivery, file)), `${file} changed during standalone build`);
    }
    const outputNames = ['converted/menu_templ.go', 'baseline.html', 'enhanced.html', 'consumer.test'];
    const outputs = await Promise.all(outputNames.map(async p => ({ path: p, bytes: (await fs.stat(path.join(consumer, p))).size, sha256: await hash(path.join(consumer, p)) })));
    runs.push({ name, outputs, resolved_modules: moduleRows });

    await fs.rename(path.join(consumer, 'converted/menu_templ.go'), path.join(run, 'menu_templ.go.withheld'));
    const negative = await isolated('missing-generated-code', go, ['test', '-c', '-trimpath', '-buildvcs=false', '-o', 'must-not-build', '.'], 'failure');
    assert(negative.stderr.toString().includes('undefined:'), 'missing-generated-file canary failed for another reason');
    await assert.rejects(fs.stat(path.join(consumer, 'must-not-build')), { code: 'ENOENT' });
    await fs.rename(path.join(run, 'menu_templ.go.withheld'), path.join(consumer, 'converted/menu_templ.go'));
  }
  assert.deepEqual(runs[0].outputs, runs[1].outputs, 'cold standalone builds did not reproduce generated/rendered/binary outputs');

  const after = await command('check-after', appkit, checkArgs);
  assert.equal(after.stdout.length + after.stderr.length, 0, 'final delivery check was not silent');
  for (const input of probeBefore) assert.equal(await hash(path.join(probe, input.path)), input.sha256, `P07 probe input changed: ${input.path}`);
  for (const input of kitBefore) assert.equal(await hash(path.join(kit, input.path)), input.sha256, `private source input changed: ${input.path}`);
  for (const input of fixtureHashes) assert.equal(await hash(path.join(fixture, input.path)), input.sha256, `fixture changed: ${input.path}`);

  const receipt = {
    schema_version: 1, kind: 'private-p07-standalone-consumer-rehearsal', verification: 'not-certified',
    compile_receipt_sha256: compileReceiptSHA, probe_before_after: probeBefore, source_before_after: kitBefore, fixture_hashes: fixtureHashes,
    tool_versions: { node: process.version, go: goVersion }, appkit_sha256: await hash(appkit),
    generated_consumer_test_sha256: sha(consumerTest), derived_go_mod_sha256: sha(goMod), sandbox_profile_sha256: sha(profile),
    owned_unsandboxed_network_canary: 200, commands, runs,
    limits: ['synthetic render workflow only', 'macOS sandbox-exec only', 'no CSS build, browser task or Elements runtime exercise',
      'GOSUMDB configured but network denied; cached public ZIPs are checked against pinned go.sum',
      'actual-used consumer/compiler module closure, not the complete transitive graph',
      'no release, real consumer adoption, visual/accessibility proof or target certification'],
  };
  await write(path.join(workspace, 'receipt.json'), JSON.stringify(receipt, null, 2) + '\n');
  console.log(JSON.stringify({ result: 'passed', workspace, receipt_sha256: await hash(path.join(workspace, 'receipt.json')), runs: 2, rendered_variants: 2 }));
} catch (error) {
  await write(path.join(workspace, 'failure.json'), JSON.stringify({ error: String(error), commands }, null, 2) + '\n');
  console.error(`P07 standalone proof failed; inspect private ${path.join(workspace, 'failure.json')}`);
  process.exitCode = 1;
} finally {
  if (canary) {
    canary.closeAllConnections();
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('owned canary close timeout')), 5000);
      canary.close(error => {
        clearTimeout(timer);
        if (error) reject(error); else resolve();
      });
    });
  }
}
