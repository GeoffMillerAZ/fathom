import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';

const evidence = '/Volumes/m2ext/appkit-evidence/appkit-static-cold.7qApnn7w';
const candidate = path.join(evidence, 'candidate');
const publicProxy = '/Users/geoff/.dev-cache/go/modules/cache/download';
const go = '/etc/profiles/per-user/geoff/bin/go';
const expectedHead = '76d3ca6b908de01d7946f7ef0845cadb6a3d8a48';
const expectedGoSHA = 'c3c8820e5edea04144cd3b6cf504aadd050bd2c7e4e63a69a6da17aed1661163';
const expectedGOROOT = '/nix/store/7ycp8j45iay38g9mjaxmy4jhwdsrb47y-go-1.26.3/share/go';
const maxClosure = 512 * 1024 * 1024;
const out = path.join(evidence, 'closure-v2');
const warm = path.join(evidence, 'preparation', 'warm-v2');
const logs = path.join(evidence, 'preparation', 'closure-logs-v2');
const hash = body => createHash('sha256').update(body).digest('hex');
const read = name => fs.readFileSync(name);

if (fs.existsSync(out) || fs.existsSync(warm) || fs.existsSync(logs)) throw Error('closure outputs must be fresh');
for (const dir of [out, warm, logs, path.join(warm, 'module-cache'), path.join(warm, 'build-cache'), path.join(warm, 'gopath'), path.join(warm, 'tmp')]) fs.mkdirSync(dir, {recursive: true, mode: 0o700});

const receipt = JSON.parse(read(path.join(candidate, 'receipt.json')));
if (receipt.source_head !== expectedHead || receipt.output !== candidate || receipt.module !== 'github.com/geoffmilleraz/app-kit') throw Error('candidate identity mismatch');
if (hash(read(go)) !== expectedGoSHA) throw Error('Go executable hash mismatch');

const env = {
  PATH: '/etc/profiles/per-user/geoff/bin:/usr/bin:/bin:/usr/sbin:/sbin',
  HOME: '/var/empty', LANG: 'en_US.UTF-8', TZ: 'UTC', GOENV: 'off', GOWORK: 'off',
  GOTOOLCHAIN: 'local', CGO_ENABLED: '0', GOVCS: '*:off', GOSUMDB: 'off',
  GOPATH: path.join(warm, 'gopath'), GOMODCACHE: path.join(warm, 'module-cache'),
  GOCACHE: path.join(warm, 'build-cache'), TMPDIR: path.join(warm, 'tmp'),
  GOPROXY: `file://${path.join(candidate, 'proxy')},file://${publicProxy}`,
};
const commands = [];
function command(label, args, cwd = warm) {
  const result = spawnSync(go, args, {cwd, env, encoding: 'utf8', timeout: 300000, maxBuffer: 128 << 20});
  fs.writeFileSync(path.join(logs, `${label}.stdout`), result.stdout || '', {flag: 'wx', mode: 0o600});
  fs.writeFileSync(path.join(logs, `${label}.stderr`), result.stderr || '', {flag: 'wx', mode: 0o600});
  commands.push({label, executable: go, args, cwd, status: result.status, signal: result.signal,
    stdout_sha256: hash(result.stdout || ''), stderr_sha256: hash(result.stderr || '')});
  if (result.error || result.status !== 0) throw Error(`${label} failed; retained logs`);
  return result.stdout;
}
function jsonStream(body) {
  const values = [];
  let depth = 0, start = -1, quoted = false, escaped = false;
  for (let i = 0; i < body.length; i++) {
    const c = body[i];
    if (quoted) {
      if (escaped) escaped = false;
      else if (c === '\\') escaped = true;
      else if (c === '"') quoted = false;
      continue;
    }
    if (c === '"') quoted = true;
    else if (c === '{') { if (depth++ === 0) start = i; }
    else if (c === '}' && --depth === 0) { values.push(JSON.parse(body.slice(start, i + 1))); start = -1; }
  }
  if (depth !== 0 || quoted) throw Error('incomplete JSON stream');
  return values;
}
function put(relative, body, mode = 0o600) {
  const dest = path.join(out, relative);
  fs.mkdirSync(path.dirname(dest), {recursive: true, mode: 0o700});
  fs.writeFileSync(dest, body, {flag: 'wx', mode});
}
function proxyEscape(value) {
  return [...value].map(c => c >= 'A' && c <= 'Z' ? '!' + c.toLowerCase() : c).join('');
}

let report = {schema_version: 'appkit-static-cold-closure-v1', candidate: {
  source_head: receipt.source_head, version: receipt.version, tree_sha256: receipt.tree_sha256,
  zip_sha256: receipt.zip_sha256, module_sum: receipt.module_sum, go_mod_sum: receipt.go_mod_sum,
}, environment: env, commands, graph_modules: [], build_test_modules: [], files: [], passed: false};
try {
  const version = command('go-version', ['version']).trim();
  const [goroot, goos, goarch] = command('go-environment', ['env', 'GOROOT', 'GOOS', 'GOARCH']).trim().split('\n');
  if (version !== 'go version go1.26.3 darwin/arm64' || goroot !== expectedGOROOT || goos !== 'darwin' || goarch !== 'arm64') throw Error('Go toolchain environment mismatch');
  const compile = command('go-compile-path', ['tool', '-n', 'compile']).trim();
  const link = command('go-link-path', ['tool', '-n', 'link']).trim();
  for (const tool of [compile, link]) if (!path.isAbsolute(tool) || !fs.statSync(tool).isFile()) throw Error(`invalid Go tool path: ${tool}`);
  report.toolchain = {version, goroot, goos, goarch, go_executable: go, go_sha256: hash(read(go)),
    compile: {path: compile, sha256: hash(read(compile))}, link: {path: link, sha256: hash(read(link))}};
  const downloaded = JSON.parse(command('candidate-download', ['mod', 'download', '-json', `${receipt.module}@${receipt.version}`]));
  if (downloaded.Sum !== receipt.module_sum || downloaded.GoModSum !== receipt.go_mod_sum || downloaded.Path !== receipt.module || downloaded.Version !== receipt.version) throw Error('downloaded candidate sums disagree with receipt');
  const moduleDir = fs.realpathSync(downloaded.Dir);
  const cache = fs.realpathSync(path.join(warm, 'module-cache'));
  if (!moduleDir.startsWith(cache + path.sep)) throw Error('candidate resolved outside warm module cache');
  const graph = jsonStream(command('module-graph', ['list', '-mod=readonly', '-m', '-json', 'all'], moduleDir));
  const packages = jsonStream(command('build-test-deps', ['list', '-mod=readonly', '-deps', '-test', '-json', './apps/gallery', './apps/gallery/cmd/static-content'], moduleDir));
  const graphByID = new Map();
  for (const mod of graph) {
    if (mod.Replace) throw Error(`replacement refused: ${mod.Path}`);
    if (mod.Main) continue;
    if (!mod.Path || !mod.Version) throw Error('incomplete module graph record');
    const relativeMod = path.join(proxyEscape(mod.Path), '@v', proxyEscape(mod.Version) + '.mod');
    const sourceMod = mod.GoMod || path.join(publicProxy, relativeMod);
    const info = fs.lstatSync(sourceMod);
    if (!info.isFile() || info.isSymbolicLink()) throw Error(`unsafe graph metadata: ${mod.Path}@${mod.Version}`);
    graphByID.set(`${mod.Path}@${mod.Version}`, {...mod, sourceMod, relativeMod});
  }
  const actual = new Set();
  for (const pkg of packages) {
    const mod = pkg.Module;
    if (!mod || mod.Main) continue;
    if (mod.Replace) throw Error(`replacement refused in package graph: ${mod.Path}`);
    actual.add(`${mod.Path}@${mod.Version}`);
  }
  for (const id of actual) if (!graphByID.has(id)) throw Error(`build/test module absent from full graph: ${id}`);

  const warmDownload = path.join(cache, 'cache', 'download') + path.sep;
  const selected = new Map();
  for (const [id, mod] of [...graphByID].sort()) {
    selected.set(mod.relativeMod, mod.sourceMod);
    if (actual.has(id)) {
      for (const suffix of ['.info', '.zip']) {
        const relative = mod.relativeMod.slice(0, -4) + suffix;
        const warmSource = path.join(warmDownload, relative);
        const source = fs.existsSync(warmSource) ? warmSource : path.join(publicProxy, relative);
        if (!fs.statSync(source).isFile()) throw Error(`missing ${suffix} for ${id}`);
        selected.set(relative, source);
      }
    }
  }
  let total = 0;
  for (const [relative, source] of [...selected].sort()) {
    const body = read(source);
    total += body.length;
    if (total > maxClosure) throw Error('dependency closure exceeds 512 MiB');
    put(path.join('proxy', relative), body);
    report.files.push({path: path.posix.join('proxy', relative.split(path.sep).join('/')), bytes: body.length, sha256: hash(body)});
  }
  report.graph_modules = [...graphByID.keys()].sort();
  report.build_test_modules = [...actual].sort();
  report.total_bytes = total;
  report.module_dir = moduleDir;
  report.passed = true;
} catch (error) {
  report.error = error.message;
  process.exitCode = 1;
} finally {
  report.finished = new Date().toISOString();
  fs.writeFileSync(path.join(out, 'closure-receipt.json'), JSON.stringify(report, null, 2) + '\n', {flag: 'wx', mode: 0o600});
}
