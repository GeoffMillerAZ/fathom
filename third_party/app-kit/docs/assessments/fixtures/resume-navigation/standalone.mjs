// Code-only RAC working-tree delivery proof; never reads resume/ or workspace/.
// node standalone.mjs <rac-checkout> <empty-system-temp-output>
import {readFile, writeFile, mkdir, realpath, readdir, lstat} from 'node:fs/promises';
import {execFileSync, spawnSync} from 'node:child_process';
import {relative, resolve, sep, dirname} from 'node:path';
import {tmpdir} from 'node:os';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';

if (process.argv.length !== 4) throw Error('Expected RAC checkout and empty temporary output.');
const [source, out] = await Promise.all(process.argv.slice(2).map(p => realpath(resolve(p))));
const rel = relative(await realpath(tmpdir()), out);
if (!rel || rel === '..' || rel.startsWith('..' + sep) || rel.startsWith(sep) || (await readdir(out)).length) throw Error('Output must be an empty child of system temp.');
const goMod = await readFile(`${source}/go.mod`, 'utf8');
if (!goMod.startsWith('module github.com/geoffmilleraz/resume-as-code\n')) throw Error('Wrong consumer.');
const requiredGoVersion = /^go ([0-9]+\.[0-9]+(?:\.[0-9]+)?)$/m.exec(goMod)?.[1];
if (!requiredGoVersion) throw Error('Consumer Go version is missing or malformed.');
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const gitEnv = Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('GIT_')));
Object.assign(gitEnv, {GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_SYSTEM: '/dev/null', GIT_CONFIG_NOSYSTEM: '1'});
const git = args => execFileSync('git', args, {cwd: source, env: gitEnv, maxBuffer: 8 * 1024 * 1024});
const snapshot = `${out}/source`, artifacts = `${out}/artifacts`;
await mkdir(snapshot, {mode: 0o700}); await mkdir(artifacts, {mode: 0o700});
const report = {schema_version: 1, scope: 'code-only dirty-working-tree snapshot; not a clean Git checkout or release pin',
  source, snapshot, head: git(['rev-parse', 'HEAD']).toString().trim(),
  harnessSHA256: hash(await readFile(fileURLToPath(import.meta.url))),
  started: new Date().toISOString(), files: [], commands: [], checks: []};
const check = (name, pass) => report.checks.push({name, pass});
const runDevbox = (name, args) => {
  const entry = {name, executable: 'devbox', args};
  report.commands.push(entry);
  const result = spawnSync('devbox', args, {
    cwd: source,
    encoding: 'utf8',
    timeout: 120000,
    maxBuffer: 2 * 1024 * 1024,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  entry.exit = result.status;
  entry.signal = result.signal;
  entry.output = result.stdout || '';
  entry.stderr = result.stderr || '';
  if (result.error || result.status !== 0) {
    if (result.error) entry.error = result.error.code || result.error.name;
    throw Error(`${name} failed; see recorded command output.`);
  }
  return entry.output;
};
const stripKnownDevboxBanner = output => output.replace(/^Welcome to the Resume as Code devbox!\r?\n/, '');
let goRoot;
let goExecutable;
const run = (name, command, extra = []) => {
  // Enter the existing Devbox environment; Go's -C selects the snapshot without
  // a nested shell. No sibling kit, workspace, or module/toolchain download.
  if (command[0] !== 'go') throw Error('Only Go commands are admitted.');
  if (!goRoot || !goExecutable) throw Error('Go toolchain was not resolved.');
  const args = ['run', '--', 'env', 'GOWORK=off', 'GOPROXY=off', 'GOENV=off', 'GOFLAGS=-mod=vendor',
    `GOROOT=${goRoot}`, 'GOTOOLCHAIN=local', goExecutable, '-C', snapshot, ...command.slice(1), ...extra];
  runDevbox(name, args);
};
try {
  const toolchainOutput = runDevbox('resolve cached Go toolchain', [
    'run', '--', 'env', 'GOWORK=off', 'GOPROXY=off', 'GOENV=off',
    'GOFLAGS=-mod=vendor', 'GOTOOLCHAIN=auto', 'go', 'env', '-json', 'GOROOT', 'GOVERSION',
  ]);
  let toolchain;
  try {
    toolchain = JSON.parse(stripKnownDevboxBanner(toolchainOutput));
  } catch {
    throw Error('Cached Go toolchain lookup returned an unexpected envelope.');
  }
  if (toolchain.GOVERSION !== `go${requiredGoVersion}` || typeof toolchain.GOROOT !== 'string') {
    throw Error('Cached Go toolchain does not match the consumer Go version.');
  }
  goRoot = await realpath(toolchain.GOROOT);
  goExecutable = await realpath(`${goRoot}/bin/go`);
  const goStat = await lstat(goExecutable);
  if (!goStat.isFile()) throw Error('Resolved Go executable is not a regular file.');
  const goBytes = await readFile(goExecutable);
  report.toolchain = {
    goversion: toolchain.GOVERSION,
    goroot: goRoot,
    executable: goExecutable,
    bytes: goBytes.length,
    sha256: hash(goBytes),
  };
  check('offline cached toolchain matches consumer Go version', true);
  const paths = [...new Set(git(['ls-files', '-z', '--cached', '--others', '--exclude-standard', '--', 'cmd', 'vendor', 'go.mod', 'go.sum']).toString().split('\0').filter(Boolean))].sort();
  if (!paths.length || !paths.includes('vendor/modules.txt')) throw Error('Vendor closure missing.');
  for (const path of paths) {
    if (path.split('/').includes('..') || !(path.startsWith('cmd/') || path.startsWith('vendor/') || ['go.mod', 'go.sum'].includes(path))) throw Error('Unexpected source path.');
    if (!(await lstat(`${source}/${path}`)).isFile()) throw Error(`Nonregular source: ${path}`);
    const bytes = await readFile(`${source}/${path}`);
    const destination = `${snapshot}/${path}`;
    await mkdir(dirname(destination), {recursive: true, mode: 0o700});
    await writeFile(destination, bytes, {flag: 'wx', mode: 0o600});
    report.files.push({path, bytes: bytes.length, sha256: hash(bytes)});
  }
  check('snapshot has no sibling kit or Go workspace', !(await readdir(out)).includes('app-kit') && !(await readdir(snapshot)).some(p => p.startsWith('go.work')));
  run('vendor selection', ['go', 'list', '-f', '{{.Dir}}', 'github.com/geoffmilleraz/app-kit/appkit/ui/dashboard']);
  // RAC's existing Devbox init prints this exact banner to stdout. Keep the
  // complete log and remove only that known envelope for the path equality.
  const listed = stripKnownDevboxBanner(report.commands.at(-1).output).trim();
  check('Go resolves the snapshot vendor', listed === `${snapshot}/vendor/github.com/geoffmilleraz/app-kit/appkit/ui/dashboard`);
  run('standalone build', ['go', 'build', '-trimpath', '-o', `${artifacts}/racli-a`, './cmd']);
  run('repeat standalone build', ['go', 'build', '-trimpath', '-o', `${artifacts}/racli-b`, './cmd']);
  const [a, b] = await Promise.all(['racli-a', 'racli-b'].map(p => readFile(`${artifacts}/${p}`)));
  report.binary = {sha256: hash(a), bytes: a.length};
  check('repeated binary builds are byte identical', a.equals(b));
  run('synthetic shell race tests', ['go', 'test', '-race', './cmd/serve', '-run', '^(TestNavHTMLAdaptsAppOwnedPagesToTypedNavigation|TestRenderPageAlwaysHasNav|TestRootRedirectsToToday)$', '-count=1']);
  for (const entry of report.files) {
    if (hash(await readFile(`${source}/${entry.path}`)) !== entry.sha256 || hash(await readFile(`${snapshot}/${entry.path}`)) !== entry.sha256) throw Error(`Source/snapshot changed during proof: ${entry.path}`);
  }
  const after = [...new Set(git(['ls-files', '-z', '--cached', '--others', '--exclude-standard', '--', 'cmd', 'vendor', 'go.mod', 'go.sum']).toString().split('\0').filter(Boolean))].sort();
  if (JSON.stringify(after) !== JSON.stringify(paths)) throw Error('Source file set changed during proof.');
  check('all source and snapshot bytes unchanged during proof', true);
  check('consumer HEAD unchanged during proof', git(['rev-parse', 'HEAD']).toString().trim() === report.head);
} catch (error) {report.fatal = error.message;}
report.finished = new Date().toISOString();
await writeFile(`${out}/report.json`, JSON.stringify(report, null, 2) + '\n', {flag: 'wx', mode: 0o600});
console.log(JSON.stringify({out, files: report.files.length, commands: report.commands.map(c => ({name: c.name, exit: c.exit})), checks: report.checks, fatal: report.fatal}, null, 2));
if (report.fatal || report.checks.some(c => !c.pass)) process.exitCode = 1;
