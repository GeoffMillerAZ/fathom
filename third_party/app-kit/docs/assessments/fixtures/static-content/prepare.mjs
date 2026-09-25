// Prepare private Gallery artifacts without changing working-tree generated CSS.
// This is a local exact-input build receipt, not a clean-checkout/cold-build proof.
import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const root = path.resolve(fileURLToPath(new URL('../../../..', import.meta.url)));
const [parent, cache] = process.argv.slice(2);
if (!parent || !cache || !path.isAbsolute(parent) || !path.isAbsolute(cache)) {
  throw new Error('usage: node prepare.mjs EXISTING_PRIVATE_PARENT EXISTING_GO_CACHE');
}
for (const dir of [parent, cache]) if (!fs.statSync(dir).isDirectory()) throw new Error(`not directory: ${dir}`);
const out = fs.mkdtempSync(path.join(parent, 'appkit-static-content-'));
fs.chmodSync(out, 0o700);
console.log(out);
const go = '/etc/profiles/per-user/geoff/bin/go';
const env = {...process.env, GOCACHE: cache, GOMODCACHE: '/Users/geoff/.dev-cache/go/modules',
  GOENV: 'off', GOWORK: 'off', GOTOOLCHAIN: 'local', GOPROXY: 'off', GOSUMDB: 'off'};
const receipt = {schema_version: 'static-content-prepare-v1', root, out, started: new Date().toISOString(),
  claim: 'local working-tree exact inputs and repeat build; not standalone/cold or runtime proof', commands: [], sources: []};
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const put = (name, bytes) => {
  const dest = path.join(out, name);
  fs.mkdirSync(path.dirname(dest), {recursive: true, mode: 0o700});
  fs.writeFileSync(dest, bytes, {flag: 'wx', mode: 0o600});
};
function snapshot(source, name) {
  const bytes = fs.readFileSync(source);
  put(name, bytes);
  receipt.sources.push({path: source, retained: name, bytes: bytes.length, sha256: hash(bytes)});
}
function command(label, executable, args) {
  const result = spawnSync(executable, args, {cwd: root, env, encoding: 'utf8', timeout: 240000, maxBuffer: 32 * 1024 * 1024});
  put(`${label}.stdout`, result.stdout || '');
  put(`${label}.stderr`, result.stderr || '');
  receipt.commands.push({label, executable, args, status: result.status, signal: result.signal, error: result.error?.message});
  if (result.error || result.status !== 0) throw new Error(`${label} failed; retained logs in ${out}`);
  return result.stdout;
}
try {
  snapshot(fileURLToPath(import.meta.url), 'inputs/prepare.mjs');
  for (const rel of ['go.mod', 'go.sum', 'apps/gallery/ui/input.css',
    'docs/assessments/fixtures/static-content/contract.cue', 'docs/assessments/fixtures/static-content/README.md']) {
    snapshot(path.join(root, rel), `inputs/${rel}`);
  }
  receipt.toolchain = command('version', go, ['version']).trim();
  const format = '{{range .GoFiles}}{{$.Dir}}/{{.}}{{printf "%c" 10}}{{end}}{{range .CgoFiles}}{{$.Dir}}/{{.}}{{printf "%c" 10}}{{end}}{{range .EmbedFiles}}{{$.Dir}}/{{.}}{{printf "%c" 10}}{{end}}';
  const files = command('source-list', go, ['list', '-mod=readonly', '-deps', '-f', format, './apps/gallery/cmd/gallery']);
  for (const source of [...new Set(files.split('\n').filter(f => f.startsWith(root + path.sep)))].sort()) {
    const relative = path.relative(root, source);
    if (relative.startsWith('..') || fs.realpathSync(source) !== source) throw new Error(`unexpected source: ${source}`);
    if (!receipt.sources.some(s => s.path === source)) snapshot(source, `inputs/${relative}`);
  }
  // Tests are retained explicitly; they are not part of the executable Go closure.
  snapshot(path.join(root, 'apps/gallery/static_content_test.go'), 'inputs/apps/gallery/static_content_test.go');
  const contract = command('contract', go, ['tool', 'cue', 'export', 'docs/assessments/fixtures/static-content/contract.cue', '--out', 'json']);
  JSON.parse(contract);
  put('contract.json', contract);
  put('overlay.json', JSON.stringify({Replace: {[path.join(root, 'apps/gallery/ui/app.css')]: path.join(out, 'app.css')}}, null, 2));
  const commands = [
    ['css', ['run', '-mod=readonly', './cmd/appkit', 'css', '-i', 'apps/gallery/ui/input.css', '-o', path.join(out, 'app.css'), '--minify']],
    ['tests', ['test', '-mod=readonly', '-overlay', path.join(out, 'overlay.json'), './apps/gallery', './appkit/ui/docs', '-count=1']],
    ...['gallery', 'gallery-repeat'].map(name => [name, ['build', '-mod=readonly', '-trimpath', '-buildvcs=false', '-overlay', path.join(out, 'overlay.json'), '-o', path.join(out, name), './apps/gallery/cmd/gallery']]),
  ];
  put('execution-plan.json', JSON.stringify({go, cwd: root, environment: Object.fromEntries(['GOCACHE','GOMODCACHE','GOENV','GOWORK','GOTOOLCHAIN','GOPROXY','GOSUMDB'].map(k => [k, env[k]])), commands}, null, 2));
  for (const [label, args] of commands) command(label, go, args);
  for (const source of receipt.sources) {
    if (hash(fs.readFileSync(source.path)) !== source.sha256) throw new Error(`source changed during preparation: ${source.path}`);
  }
  receipt.artifacts = Object.fromEntries(['gallery', 'gallery-repeat', 'app.css', 'contract.json'].map(name => {
    const bytes = fs.readFileSync(path.join(out, name));
    return [name, {bytes: bytes.length, sha256: hash(bytes)}];
  }));
  if (receipt.artifacts.gallery.sha256 !== receipt.artifacts['gallery-repeat'].sha256) throw new Error('repeat build differs');
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
