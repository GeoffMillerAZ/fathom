// Exact local working-tree preparation, not a clean-checkout/cold-build claim.
import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const root = path.resolve(fileURLToPath(new URL('../../../..', import.meta.url)));
const [parent, cache, modules] = process.argv.slice(2);
if (![parent, cache, modules].every(p => p && path.isAbsolute(p) && fs.statSync(p).isDirectory())) {
  throw new Error('usage: node prepare.mjs EXISTING_PRIVATE_PARENT GO_CACHE MODULE_CACHE');
}
const out = fs.mkdtempSync(path.join(parent, 'prepared-'));
console.log(out);
const go = '/etc/profiles/per-user/geoff/bin/go';
const env = {PATH: '/etc/profiles/per-user/geoff/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin',
  HOME: process.env.HOME, LANG: 'en_US.UTF-8', TZ: 'UTC', GOCACHE: cache, GOMODCACHE: modules, GOENV: 'off',
  GOWORK: 'off', GOTOOLCHAIN: 'local', GOPROXY: 'file:///Users/geoff/.dev-cache/go/modules/cache/download', GOSUMDB: 'off', GOVCS: '*:off',
  GOFLAGS: '-mod=readonly', GOMAXPROCS: '2', TMPDIR: path.join(out, 'tmp'), GOTMPDIR: path.join(out, 'tmp')};
const receipt = {schema_version: 'static-export-prepare-v1', root, out,
  started: new Date().toISOString(), claim: 'local exact inputs, tests, repeat build and export; no browser or deployed-host claim',
  environment: env, snapshot_scope: 'cooperative local tree, stable descriptor reads and before/after drift checks; not adversarial race confinement or a hermetic build',
  commands: [], sources: []};
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
function put(name, bytes) {
  const dest = path.join(out, name);
  fs.mkdirSync(path.dirname(dest), {recursive: true, mode: 0o700});
  fs.writeFileSync(dest, bytes, {flag: 'wx', mode: 0o600});
}
function snapshot(source) {
  if (receipt.sources.some(s => s.path === source)) return;
  const relative = path.relative(root, source);
  if (relative.startsWith('..') || fs.realpathSync(source) !== source) throw new Error(`unexpected source: ${source}`);
  const bytes = stableRead(source);
  const retained = `inputs/${relative}`;
  put(retained, bytes);
  receipt.sources.push({path: source, retained, bytes: bytes.length, sha256: hash(bytes)});
}
function stableRead(source) {
  const fd = fs.openSync(source, fs.constants.O_RDONLY | fs.constants.O_NOFOLLOW);
  try {
    const before = fs.fstatSync(fd);
    if (!before.isFile() || before.size > 512 * 1024 * 1024) throw new Error(`not a bounded regular file: ${source}`);
    const bytes = fs.readFileSync(fd), after = fs.fstatSync(fd);
    if (bytes.length !== before.size || after.size !== before.size || after.mtimeMs !== before.mtimeMs || after.ctimeMs !== before.ctimeMs) throw new Error(`changed during read: ${source}`);
    return bytes;
  } finally { fs.closeSync(fd); }
}
function command(label, executable, args) {
  const invocation = {label, executable, args, cwd: root};
  put(`${label}.command.json`, JSON.stringify(invocation, null, 2));
  const result = spawnSync(executable, args, {cwd: root, env, encoding: 'utf8', timeout: 240000, maxBuffer: 64 * 1024 * 1024});
  put(`${label}.stdout`, result.stdout || '');
  put(`${label}.stderr`, result.stderr || '');
  receipt.commands.push({...invocation, status: result.status, signal: result.signal, error: result.error?.message});
  if (result.error || result.status !== 0) throw new Error(`${label} failed; logs retained in ${out}`);
  return result.stdout;
}
function unchanged() {
  for (const source of receipt.sources) {
    if (hash(stableRead(source.path)) !== source.sha256) throw new Error(`source changed: ${source.path}`);
  }
}
try {
  fs.chmodSync(out, 0o700);
  fs.mkdirSync(path.join(out, 'tmp'), {mode: 0o700});
  for (const rel of ['go.mod', 'go.sum', 'apps/gallery/cmd/static-content/input.css',
    'internal/tailwind/manifest.cue',
    'docs/assessments/fixtures/static-export/prepare.mjs',
    'docs/assessments/fixtures/static-export/probe.mjs',
    'docs/assessments/fixtures/static-export/probe_test.mjs',
    'docs/assessments/fixtures/static-export/contract.cue',
    'docs/assessments/fixtures/static-export/README.md']) snapshot(path.join(root, rel));
  receipt.toolchain = command('version', go, ['version']).trim();
  // Go's synthetic test variants may report absolute GoFiles. Do not prefix twice.
  const fileList = field => `{{range .${field}}}{{if eq (slice . 0 1) "/"}}{{.}}{{else}}{{$.Dir}}/{{.}}{{end}}{{printf "%c" 10}}{{end}}`;
  const format = ['GoFiles', 'CgoFiles', 'EmbedFiles'].map(fileList).join('');
  const files = command('source-list', go, ['list', '-deps', '-test', '-f', format, './apps/gallery', './apps/gallery/cmd/static-content', './appkit/ui/docs']);
  for (const source of [...new Set(files.split('\n').filter(p => p.startsWith(root + path.sep)))].sort()) snapshot(source);
  for (const dir of ['apps/gallery', 'apps/gallery/cmd/static-content', 'appkit/ui/docs']) {
    for (const name of fs.readdirSync(path.join(root, dir)).filter(n => n.endsWith('_test.go')).sort()) snapshot(path.join(root, dir, name));
  }
  // Retain every explicit CSS import and source scan, including test Go/portable JS.
  const input = path.join(root, 'apps/gallery/cmd/static-content/input.css');
  const cssInput = fs.readFileSync(input, 'utf8');
  for (const match of cssInput.matchAll(/@import\s+"(\.[^"]+)"/g)) snapshot(path.resolve(path.dirname(input), match[1]));
  for (const match of cssInput.matchAll(/@source\s+"([^"]+)"/g)) {
    const scan = path.resolve(path.dirname(input), match[1]);
    if (!scan.includes('*')) { snapshot(scan); continue; }
    const dir = path.dirname(scan), suffix = path.basename(scan).replace(/^\*/, '');
    if (path.basename(scan) !== '*' + suffix || suffix.includes('*')) throw new Error(`unsupported scan: ${scan}`);
    for (const name of fs.readdirSync(dir).filter(n => n.endsWith(suffix)).sort()) snapshot(path.join(dir, name));
  }
  put('frozen-sources.json', JSON.stringify(receipt.sources, null, 2));
  const contract = command('contract', go, ['tool', 'cue', 'export', 'docs/assessments/fixtures/static-export/contract.cue', '--out', 'json']);
  JSON.parse(contract);
  put('contract.json', contract);
  command('oracle-tests', process.execPath, ['--test', 'docs/assessments/fixtures/static-export/probe_test.mjs']);
  // Never call a download-capable resolver: execute only a copied, checksum-bound
  // pre-provisioned compiler. The same manifest owns the normal appkit css command.
  const pins = JSON.parse(command('css-pin', go, ['tool', 'cue', 'export', 'internal/tailwind/manifest.cue', '-e', 'manifest', '--out', 'json']));
  if (process.platform !== 'darwin' || process.arch !== 'arm64' || pins.version !== 'v4.3.0') throw new Error('unreviewed compiler platform/version');
  const pin = pins.binaries['darwin-arm64'];
  if (pin.asset !== 'tailwindcss-macos-arm64') throw new Error('unreviewed compiler asset');
  const cachedCompiler = '/Users/geoff/Library/Caches/app-kit/tailwindcss/' + pins.version + '/' + pin.asset;
  const compiler = stableRead(cachedCompiler);
  if (hash(compiler) !== pin.sha256) throw new Error('cached compiler checksum mismatch');
  put('tools/tailwindcss', compiler);
  fs.chmodSync(path.join(out, 'tools/tailwindcss'), 0o700);
  receipt.css_compiler = {source: cachedCompiler, version: pins.version, bytes: compiler.length, sha256: pin.sha256};
  command('css', path.join(out, 'tools/tailwindcss'), ['-i', 'apps/gallery/cmd/static-content/input.css', '-o', path.join(out, 'app.css'), '--minify']);
  // Existing Gallery embeds/tests remain untouched; only the exporter consumes the
  // smaller selected CSS explicitly at execution. No generated working-tree writes.
  command('tests', go, ['test', '-p=2', '-race', '-count=1', '-json', './apps/gallery', './apps/gallery/cmd/static-content', './appkit/ui/docs']);
  for (const name of ['static-content', 'static-content-repeat']) {
    unchanged();
    command(name, go, ['build', '-trimpath', '-buildvcs=false', '-o', path.join(out, name), './apps/gallery/cmd/static-content']);
  }
  if (hash(fs.readFileSync(path.join(out, 'static-content'))) !== hash(fs.readFileSync(path.join(out, 'static-content-repeat')))) throw new Error('repeat build differs');
  for (const [name, binary] of [['site', 'static-content'], ['site-repeat', 'static-content-repeat']]) {
    command(name, path.join(out, binary), ['-out', path.join(out, name), '-css', path.join(out, 'app.css'), '-base-path', '/reading/']);
  }
  const names = fs.readdirSync(path.join(out, 'site')).sort();
  if (JSON.stringify(names) !== JSON.stringify(fs.readdirSync(path.join(out, 'site-repeat')).sort())) throw new Error('repeat inventory differs');
  for (const name of names) if (!fs.readFileSync(path.join(out, 'site', name)).equals(fs.readFileSync(path.join(out, 'site-repeat', name)))) throw new Error(`repeat file differs: ${name}`);
  unchanged();
  receipt.artifacts = Object.fromEntries(['static-content', 'static-content-repeat', 'app.css', 'contract.json', ...names.map(n => `site/${n}`)].map(name => {
    const bytes = fs.readFileSync(path.join(out, name));
    return [name, {bytes: bytes.length, sha256: hash(bytes)}];
  }));
  receipt.passed = true;
} catch (error) {
  receipt.passed = false;
  receipt.error = error.message;
  process.exitCode = 1;
} finally {
  receipt.finished = new Date().toISOString();
  put('prepare-receipt.json', JSON.stringify(receipt, null, 2));
  console.log(JSON.stringify({out, passed: receipt.passed, error: receipt.error}));
}
