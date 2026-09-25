// Working-tree source proof, not a clean Git clone or a released consumer pin.
// node standalone-css.mjs <fathom-checkout> <empty-workspace/tmp-report-dir>
import {readFile, writeFile, mkdir, realpath, readdir, lstat, readlink, symlink} from 'node:fs/promises';
import {execFileSync, spawnSync} from 'node:child_process';
import {relative, resolve, dirname, sep, isAbsolute} from 'node:path';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';

if (process.argv.length !== 4) throw Error('Expected Fathom checkout and empty workspace/tmp report directory.');
const [source, out] = await Promise.all(process.argv.slice(2).map(p => realpath(resolve(p))));
const within = (root, path) => {const r = relative(root, path); return r !== '' && r !== '..' && !r.startsWith('..' + sep) && !r.startsWith(sep);};
if (!within(await realpath(`${source}/workspace/tmp`), out) || (await readdir(out)).length) throw Error('Report must be an empty child of consumer workspace/tmp.');
if (!(await readFile(`${source}/go.mod`, 'utf8')).startsWith('module github.com/geoffmilleraz/fathom\n')) throw Error('Wrong consumer.');
const roots = ['cmd', 'internal', 'pkg', 'schema', 'third_party', 'scripts', 'testdata', 'dist', 'main.go', 'go.mod', 'go.sum', 'Taskfile.yml'];
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('GIT_')));
Object.assign(env, {GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_SYSTEM: '/dev/null', GIT_CONFIG_NOSYSTEM: '1',
  GOWORK: 'off', GOPROXY: 'off', GOSUMDB: 'off', GOENV: 'off', GOFLAGS: '-mod=readonly -buildvcs=false',
  GOTOOLCHAIN: 'local', CGO_ENABLED: '1', MACOSX_DEPLOYMENT_TARGET: '14.0'});
const git = args => execFileSync('git', args, {cwd: source, env, maxBuffer: 8 * 1024 * 1024}).toString();
const selected = () => [...new Set(git(['ls-files', '-z', '--cached', '--others', '--exclude-standard', '--', ...roots]).split('\0').filter(Boolean))].sort();
// Git visibility is not Tailwind visibility: an ignored renderer must not silently
// disappear from the evidence copy. The production validator owns CSS grammar.
const cssSources = async () => {
  const input = `${source}/cmd/fathom/ui/input.css`, files = [];
  for (const match of (await readFile(input, 'utf8')).matchAll(/^@source "([^"]+)";$/gm)) {
    const path = resolve(dirname(input), match[1]);
    if (!within(source, path)) throw Error('CSS source escapes checkout.');
    if (path.endsWith('/*.go')) {
      for (const name of await readdir(dirname(path))) if (name.endsWith('.go')) files.push(relative(source, resolve(dirname(path), name)));
    } else if (path.includes('*')) throw Error('Unsupported source glob in snapshot proof.');
    else files.push(relative(source, path));
  }
  if (!files.length) throw Error('Empty CSS source selection.');
  return [...new Set(files)].sort();
};
const report = {schema_version: 1, scope: 'code-only Git-visible working-tree snapshots with complete declared CSS sources; cached dependencies; exact mirror archive identity verified separately by vendor-check; no clean Git clone, cold-cache, live-data, release or deployment proof',
  source, out, started: new Date().toISOString(), head: git(['rev-parse', 'HEAD']).trim(),
  harnessSHA256: hash(await readFile(fileURLToPath(import.meta.url))), files: [], commands: [], checks: [], artifacts: []};
const check = (name, pass) => {report.checks.push({name, pass}); if (!pass) throw Error(name);};
const run = (cwd, name, executable, args) => {
  const result = spawnSync(executable, args, {cwd, env, encoding: 'utf8', timeout: 180000, maxBuffer: 8 * 1024 * 1024});
  report.commands.push({name, cwd, executable, args, exit: result.status, signal: result.signal,
    stdout: result.stdout || '', stderr: result.stderr || '', ...(result.error ? {error: result.error.message} : {})});
  if (result.error || result.status !== 0) throw Error(`${name} failed; see command receipt.`);
  return result.stdout.trim();
};
try {
  report.toolchain = run(source, 'toolchain identity', 'go', ['version']);
  run(source, 'production CSS source validation', 'bash', ['scripts/ui-css.sh', '--check-sources']);
  const paths = selected();
  report.cssSources = await cssSources();
  check('every live declared CSS source is included, even if Git would ignore it', report.cssSources.every(p => paths.includes(p)));
  check('source selection includes both mirror stamps and CSS build inputs', [
    'third_party/app-kit/.vendor-commit', 'third_party/agentic-kit/.vendor-commit',
    'third_party/app-kit/appkit/ui/dashboard/nav.go', 'scripts/ui-css.sh',
    'cmd/fathom/ui/input.css', 'cmd/fathom/ui/app.css',
  ].every(p => paths.includes(p)));
  const snapshots = ['a', 'b'].map(name => `${out}/${name}/source`);
  for (const snapshot of snapshots) await mkdir(snapshot, {recursive: true, mode: 0o700});
  for (const path of paths) {
    if (path.includes('\n') || path.split('/').includes('..') || !roots.includes(path.split('/')[0])) throw Error(`Unexpected selected path: ${path}`);
    const input = `${source}/${path}`, stat = await lstat(input);
    if (!within(source, await realpath(input))) throw Error(`Source escapes checkout: ${path}`);
    const record = {path, mode: stat.mode & 0o777};
    if (stat.isSymbolicLink()) {
      record.kind = 'symlink'; record.target = await readlink(input);
      if (isAbsolute(record.target) || !within(source, resolve(dirname(input), record.target))) throw Error(`Symlink escapes checkout: ${path}`);
    } else if (stat.isFile()) {
      record.kind = 'file'; const bytes = await readFile(input); record.bytes = bytes.length; record.sha256 = hash(bytes);
    } else throw Error(`Nonregular source: ${path}`);
    report.files.push(record);
    for (const snapshot of snapshots) {
      const dest = `${snapshot}/${path}`;
      await mkdir(dirname(dest), {recursive: true, mode: 0o700});
      if (record.kind === 'symlink') await symlink(record.target, dest);
      else {
        const bytes = await readFile(input);
        if (hash(bytes) !== record.sha256) throw Error(`Source changed while copying: ${path}`);
        await writeFile(dest, bytes, {flag: 'wx', mode: record.mode});
      }
    }
  }
  for (const snapshot of snapshots) for (const file of report.files.filter(f => f.kind === 'symlink')) {
    check(`copied symlink stays inside snapshot: ${file.path}`, within(snapshot, await realpath(`${snapshot}/${file.path}`)));
  }
  const artifactPaths = ['cmd/fathom/ui/tokens/theme.css', 'cmd/fathom/ui/app.css'];
  for (const [index, snapshot] of snapshots.entries()) {
    await mkdir(`${snapshot}/workspace/tmp`, {recursive: true, mode: 0o700});
    check(`snapshot ${index}: no Go workspace or sibling kit`, !(await readdir(dirname(snapshot))).includes('app-kit') && !(await readdir(snapshot)).some(p => p.startsWith('go.work')));
    for (const kit of ['app-kit', 'agentic-kit']) {
      const dir = run(snapshot, `${index}: resolve ${kit}`, 'go', ['list', '-m', '-f', '{{.Dir}}', `github.com/geoffmilleraz/${kit}`]);
      check(`snapshot ${index}: ${kit} resolves inside mirror`, dir === `${snapshot}/third_party/${kit}`);
    }
    run(snapshot, `${index}: real CSS regeneration`, 'bash', ['scripts/ui-css.sh']);
    run(snapshot, `${index}: real freshness replay`, 'bash', ['scripts/ui-css.sh', '--check']);
    for (const path of artifactPaths) {
      const bytes = await readFile(`${snapshot}/${path}`), actual = hash(bytes);
      report.artifacts.push({snapshot: index, path, bytes: bytes.length, sha256: actual});
      check(`snapshot ${index}: regenerated ${path} equals selected artifact`, actual === report.files.find(f => f.path === path).sha256);
    }
    run(snapshot, `${index}: standalone full build`, 'go', ['build', './...']);
    run(snapshot, `${index}: standalone CLI artifact`, 'go', ['build', '-trimpath', '-o', `${out}/${index ? 'b' : 'a'}/fathom`, './cmd/fathom']);
  }
  const [a, b] = await Promise.all(['a', 'b'].map(name => readFile(`${out}/${name}/fathom`)));
  report.binary = {bytes: a.length, sha256: hash(a)};
  check('two sibling-free CLI builds byte identical', a.equals(b));
  run(snapshots[0], 'synthetic UI race suite', 'go', ['test', '-race', './cmd/fathom', '-run', '^(TestObservatory|TestAppkit|TestExportAppkit|TestServicePage|TestDimensionSeries)', '-count=1']);
  for (const file of report.files) for (const root of [source, ...snapshots]) {
    const path = `${root}/${file.path}`;
    check(`${root === source ? 'source' : relative(out, root)} unchanged: ${file.path}`,
      file.kind === 'symlink' ? await readlink(path) === file.target : hash(await readFile(path)) === file.sha256);
  }
  check('source path selection unchanged', JSON.stringify(paths) === JSON.stringify(selected()));
  check('declared CSS source set unchanged', JSON.stringify(report.cssSources) === JSON.stringify(await cssSources()));
  check('consumer HEAD unchanged', report.head === git(['rev-parse', 'HEAD']).trim());
} catch (error) { report.fatal = error.message; }
report.finished = new Date().toISOString();
await writeFile(`${out}/report.json`, JSON.stringify(report, null, 2) + '\n', {flag: 'wx', mode: 0o600});
console.log(JSON.stringify({out, files: report.files.length, commands: report.commands.map(c => ({name: c.name, exit: c.exit})),
  checks: report.checks.length, failed: report.checks.filter(c => !c.pass), artifacts: report.artifacts, binary: report.binary, fatal: report.fatal}, null, 2));
if (report.fatal || report.checks.some(c => !c.pass)) process.exitCode = 1;
