// Generate a test-only Go overlay; never add a file to the consumer checkout.
// node run.mjs <fathom-checkout> <existing-approved-playwright/index.mjs> [--reader-candidate <prepared-source.go>]
import {readFile, writeFile, realpath, lstat, mkdtemp, mkdir} from 'node:fs/promises';
import {join, resolve, dirname, isAbsolute} from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath} from 'node:url';
import {createHash} from 'node:crypto';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
const exec = promisify(execFile);
const args = process.argv.slice(2);
const [input, moduleInput, mode, candidateInput] = args;
if (args.length !== 2 && !(args.length === 4 && ['--reader-candidate', '--display-candidate', '--display-baseline'].includes(mode) && isAbsolute(candidateInput))) throw Error('Expected consumer, Playwright, and optionally a known candidate mode with an absolute prepared source path');
if (!input || !moduleInput || !isAbsolute(input) || !isAbsolute(moduleInput)) throw Error('Explicit absolute consumer and approved Playwright paths required');
const consumer = await realpath(input), modulePath = await realpath(moduleInput);
const fixture = dirname(fileURLToPath(import.meta.url));
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
if (!(await readFile(join(consumer, 'go.mod'), 'utf8')).startsWith('module github.com/geoffmilleraz/fathom\n')) throw Error('Not the expected Fathom module');
const virtual = join(consumer, 'cmd/fathom/appkit_actual_handler_probe_test.go');
const reader = join(consumer, 'cmd/fathom/instrument_adoption.go');
const virtualPaths = [virtual];
const replacements = {[virtual]: join(fixture, 'actual_handler_probe_test.go.txt')};
let candidate;
const display = mode === '--display-candidate' || mode === '--display-baseline';
const displayFiles = {
  'cmd/fathom/observatory_render.go': ['309c86dd3df035a0a22a3271359498daa9e28837d90cbe4cbbdd26b25f50b34c', 'c17dfae367276856579902d1184a56f2aa53a0a533b65d52d5d272870b99290b'],
  'cmd/fathom/observatory_series.go': ['6530bce155e003a0384a79e03ccd07da232c04f2f8aee938caeced4c5c8ee886', '7bf0e15b0e1503f2539bb781e303c7028967d2361580eb63ac7a31ed3c78d41e'],
  // Rebuilt with the candidate: the pinned kit already emits these utilities.
  'cmd/fathom/ui/app.css': ['0e0b5e5408c5e2484f0f2adcdba6a7c9491214d3528051ef7b43300dc52acaa1', '0e0b5e5408c5e2484f0f2adcdba6a7c9491214d3528051ef7b43300dc52acaa1'],
};
const displayBytes = {};
if (display) {
  const info = await lstat(candidateInput);
  if (!info.isDirectory() || await realpath(candidateInput) !== candidateInput) throw Error('Display candidate must be a real absolute directory');
  for (const [file, [baseSHA, candidateSHA]] of Object.entries(displayFiles)) {
    const path = join(candidateInput, file), info = await lstat(path);
    if (!info.isFile() || info.size > 1024 * 1024 || await realpath(path) !== path) throw Error('Display candidate must contain bounded regular files without symlinks');
    const bytes = await readFile(path);
    if (sha(bytes) !== candidateSHA) throw Error('Display candidate identity mismatch: ' + file);
    if (sha(await readFile(join(consumer, file))) !== baseSHA) throw Error('Display base changed; review the patch again: ' + file);
    displayBytes[file] = bytes;
  }
  const unitVirtual = join(consumer, 'cmd/fathom/appkit_display_quality_test.go');
  virtualPaths.push(unitVirtual);
  replacements[unitVirtual] = join(fixture, 'display_quality_test.go.txt');
}
if (mode === '--reader-candidate') {
  // This is one reviewed, unapplied repair, not an arbitrary source-overlay facility.
  const info = await lstat(candidateInput);
  if (!info.isFile() || info.size > 1024 * 1024) throw Error('Candidate must be a bounded regular source file');
  candidate = await readFile(candidateInput);
  if (sha(candidate) !== '20052f0bf5cf364aaaf36f3d05a19f09295206cff8dbb4f467fd82ad93ec0292') throw Error('Prepared reader candidate identity mismatch');
  if (sha(await readFile(reader)) !== 'e56cf78ddc16160a0536e245b26f1d5c063a2b2ec0749764d4fb9986228ef197') throw Error('Reader base changed; review the patch again');
  const unitVirtual = join(consumer, 'cmd/fathom/appkit_call_log_read_errors_test.go');
  virtualPaths.push(unitVirtual);
  replacements[unitVirtual] = join(fixture, 'call_log_read_errors_test.go.txt');
}
for (const path of virtualPaths) {
  try { await lstat(path); throw Error('Virtual probe path already exists; refusing to shadow consumer work'); }
  catch (e) { if (e.code !== 'ENOENT') throw e; }
}
let reusedCache;
if (process.env.APPKIT_FATHOM_HANDLER_CACHE) {
  const path = process.env.APPKIT_FATHOM_HANDLER_CACHE, info = await lstat(path);
  if (!isAbsolute(path) || !info.isDirectory() || await realpath(path) !== path) throw Error('Explicit existing real private compiler cache required');
  reusedCache = path;
}
const root = await mkdtemp(join(tmpdir(), 'appkit-fathom-handler-'));
const reports = join(root, 'reports'); await mkdir(reports, {mode: 0o700});
console.log('Actual handler evidence: ' + root);
const overlay = join(root, 'overlay.json');
if (candidate) {
  const snapshot = join(root, 'reader-candidate.go');
  await writeFile(snapshot, candidate, {flag: 'wx', mode: 0o600});
  replacements[reader] = snapshot;
}
if (display && mode === '--display-candidate') {
  for (const [file, bytes] of Object.entries(displayBytes)) {
    const snapshot = join(root, 'candidate', file);
    await mkdir(dirname(snapshot), {recursive: true, mode: 0o700});
    await writeFile(snapshot, bytes, {flag: 'wx', mode: 0o600});
    replacements[join(consumer, file)] = snapshot;
  }
}
// Retain the complete local harness/configuration closure before executing it.
// Browser-relative imports keep their original layout inside the private snapshot.
const snapshotRoot = join(root, 'source-snapshot');
const localFiles = ['docs/assessments/fixtures/fathom-handler/run.mjs',
  'docs/assessments/fixtures/fathom-handler/browser.mjs',
  'docs/assessments/fixtures/fathom-handler/actual_handler_probe_test.go.txt',
  'apps/gallery/browser/config.mjs',
  ...(display ? ['docs/assessments/fixtures/fathom-handler/display_quality_test.go.txt'] : []),
  ...(candidate ? ['docs/assessments/fixtures/fathom-handler/call_log_read_errors_test.go.txt', 'docs/assessments/fixtures/fathom-handler/call-log-read-errors.patch'] : [])];
const kitRoot = resolve(fixture, '../../../..'), snapshots = {};
for (const file of localFiles) {
  const bytes = await readFile(join(kitRoot, file)), to = join(snapshotRoot, file);
  await mkdir(dirname(to), {recursive: true, mode: 0o700});
  await writeFile(to, bytes, {flag: 'wx', mode: 0o600}); snapshots[file] = sha(bytes);
}
for (const path of virtualPaths) replacements[path] = join(snapshotRoot, 'docs/assessments/fixtures/fathom-handler', replacements[path].split('/').at(-1));
await writeFile(overlay, JSON.stringify({Replace: replacements}, null, 2) + '\n', {flag: 'wx', mode: 0o600});
const env = {...process.env, GOWORK: 'off', GOPROXY: 'off', GOSUMDB: 'off', GOENV: 'off', GOTOOLCHAIN: 'local', GOFLAGS: '',
  GOCACHE: reusedCache || join(root, 'go-build-cache'),
  APPKIT_FATHOM_HANDLER_BROWSER: join(snapshotRoot, 'docs/assessments/fixtures/fathom-handler/browser.mjs'), APPKIT_FATHOM_HANDLER_REPORT_ROOT: reports,
  APPKIT_PLAYWRIGHT_MODULE: modulePath};
for (const key of Object.keys(env)) if (key.startsWith('GIT_')) delete env[key];
const files = ['go.mod', 'go.sum', 'cmd/fathom/observatory_render.go', 'cmd/fathom/observatory_series.go', 'cmd/fathom/observatory.go',
  'cmd/fathom/instrument_adoption.go', 'cmd/fathom/defaultpaths.go',
  'cmd/fathom/observatory_test.go', 'cmd/fathom/sessionhook.go', 'cmd/fathom/origin.go', 'cmd/fathom/configpanel.go',
  'cmd/fathom/fleetreport.go', 'cmd/fathom/ui/app.css', 'third_party/app-kit/.vendor-commit',
  'third_party/app-kit/appkit/ui/dashboard/nav.go', 'third_party/app-kit/appkit/ui/dashboard/shell.go'];
const identities = async () => Object.fromEntries(await Promise.all(files.map(async file => [file, sha(await readFile(join(consumer, file)))])));
const git = async args => (await exec('git', ['-C', consumer, ...args], {env, maxBuffer: 2 ** 20})).stdout;
const report = {consumer, mode: display ? mode.slice(2) : candidate ? 'unapplied-reader-candidate' : 'current-consumer', replacements, snapshots,
  compilerCache: {path: env.GOCACHE, reused: !!reusedCache},
  ...(candidate ? {candidateSHA256: sha(candidate)} : {}),
  startedAt: new Date().toISOString(), limitations: ['Selected source/asset identities and dirty inventory, not a full build-closure or clean-checkout proof.',
  'Candidate modes change compilation only; a pass is not consumer integration or permission to apply the patch.',
  'Uses current consumer bytes and existing module cache offline; an explicitly supplied private compiler cache may be reused. Not a cold-build measurement. No dependencies or consumer files installed.'], commands: []};
let before, status;
const fixtureFiles = ['run.mjs', 'browser.mjs', 'actual_handler_probe_test.go.txt', ...(display ? ['display_quality_test.go.txt'] : []), ...(candidate ? ['call_log_read_errors_test.go.txt', 'call-log-read-errors.patch'] : [])];
const fixtureIdentities = async () => Object.fromEntries(await Promise.all(fixtureFiles.map(async f => [f, sha(await readFile(join(fixture, f)))])));
try {
  const gitRoot = await realpath((await git(['rev-parse', '--show-toplevel'])).trim());
  if (gitRoot !== consumer) throw Error('The explicit consumer directory must be its canonical Git root');
  before = await identities(); status = await git(['status', '--porcelain=v1', '--untracked-files=all']);
  if (candidate && before['cmd/fathom/instrument_adoption.go'] !== 'e56cf78ddc16160a0536e245b26f1d5c063a2b2ec0749764d4fb9986228ef197') throw Error('Reader changed before candidate execution');
  if (display && Object.entries(displayFiles).some(([file, [base]]) => before[file] !== base)) throw Error('Display base changed before execution');
  report.before = before; report.head = (await git(['rev-parse', 'HEAD'])).trim(); report.status = status;
  report.fixture = await fixtureIdentities();
  report.go = (await exec('go', ['version'], {env})).stdout.trim();
  const options = {cwd: consumer, env, maxBuffer: 8 * 2 ** 20, timeout: 240000};
  const listArgs = ['list', '-mod=readonly', '-overlay', overlay, '-f', '{{range .TestGoFiles}}{{println .}}{{end}}', './cmd/fathom'];
  const listed = await exec('go', listArgs, options);
  report.commands.push({args: listArgs, stderr: listed.stderr});
  if (!virtualPaths.every(path => listed.stdout.split('\n').includes(path.slice(path.lastIndexOf('/') + 1)))) throw Error('Go did not discover every overlay test; no run permitted');
  const names = [...(mode === '--display-baseline' ? [] : ['TestAppkitActualObservatoryHandlerProbe']),
    ...(candidate ? ['TestAppkitCallLogReadErrors'] : []),
    ...(display ? ['TestAppkitFleetFreshnessDisplayQuality', 'TestAppkitKVLongDiagnosticWraps', 'TestObservatoryFleetPage', 'TestObservatoryServicePage', 'TestObservatoryImpactPage'] : [])];
  const args = ['test', '-race', '-mod=readonly', '-overlay', overlay, './cmd/fathom', '-run', '^(' + names.join('|') + ')$', '-count=1', '-v', '-timeout=180s'];
  console.log('Virtual test discovered; running actual handlers and isolated browser.');
  try {
    const result = await exec('go', args, options);
    report.commands.push({args, ...result});
    // Exit zero with no matched tests is not proof of execution.
    if (!names.every(name => result.stdout.includes('--- PASS: ' + name + ' '))) throw Error('Expected named test pass is absent');
    report.passed = true;
  } catch (error) {
    report.commands.push({args, code: error.code, stdout: error.stdout, stderr: error.stderr}); throw error;
  }
} catch (error) {
  report.passed = false; report.error = String(error); process.exitCode = 1;
} finally {
  if (before) {
    report.after = await identities();
    report.selectedSourcesUnchanged = JSON.stringify(before) === JSON.stringify(report.after);
    report.dirtyInventoryUnchanged = status === await git(['status', '--porcelain=v1', '--untracked-files=all']);
    report.fixtureAfter = await fixtureIdentities();
    report.fixturesUnchanged = JSON.stringify(report.fixture) === JSON.stringify(report.fixtureAfter);
    report.virtualFileAbsent = true;
    for (const path of virtualPaths) {
      try {await lstat(path); report.virtualFileAbsent = false;}
      catch (e) {if (e.code !== 'ENOENT') throw e;}
    }
    report.candidateSnapshotUnchanged = !candidate || sha(await readFile(replacements[reader])) === sha(candidate);
    report.harnessSnapshotsUnchanged = true;
    for (const [file, digest] of Object.entries(snapshots)) {
      if (sha(await readFile(join(snapshotRoot, file))) !== digest || sha(await readFile(join(kitRoot, file))) !== digest) report.harnessSnapshotsUnchanged = false;
    }
    if (display && mode === '--display-candidate') {
      for (const [file, [, digest]] of Object.entries(displayFiles)) {
        if (sha(await readFile(replacements[join(consumer, file)])) !== digest) report.candidateSnapshotUnchanged = false;
      }
    }
    if (!report.selectedSourcesUnchanged || !report.dirtyInventoryUnchanged || !report.fixturesUnchanged || !report.harnessSnapshotsUnchanged || !report.virtualFileAbsent || !report.candidateSnapshotUnchanged) {report.passed = false; process.exitCode = 1;}
  }
  report.finishedAt = new Date().toISOString();
  await writeFile(join(root, 'run-report.json'), JSON.stringify(report, null, 2) + '\n', {flag: 'wx', mode: 0o600});
  console.log(JSON.stringify({passed: report.passed, error: report.error, sourcesUnchanged: report.selectedSourcesUnchanged, inventoryUnchanged: report.dirtyInventoryUnchanged, report: join(root, 'run-report.json')}));
}
