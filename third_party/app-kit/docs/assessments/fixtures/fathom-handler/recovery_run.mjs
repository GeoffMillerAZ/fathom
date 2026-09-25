// Separate recovery acquisition. No production replacement or consumer writes.
// node recovery_run.mjs <absolute-fathom-root> <approved-playwright/index.mjs>
import {readFile, writeFile, realpath, lstat, mkdtemp, mkdir} from 'node:fs/promises';
import {join, dirname, resolve, isAbsolute} from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath} from 'node:url';
import {createHash, randomBytes} from 'node:crypto';
import {spawn} from 'node:child_process';

const fixture = dirname(fileURLToPath(import.meta.url));
const relativeFixture = 'docs/assessments/fixtures/fathom-handler';
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const selected = ['go.mod', 'go.sum', 'cmd/fathom/observatory_render.go',
  'cmd/fathom/observatory_series.go', 'cmd/fathom/observatory.go',
  'cmd/fathom/instrument_adoption.go', 'cmd/fathom/defaultpaths.go',
  'cmd/fathom/observatory_test.go', 'cmd/fathom/sessionhook.go', 'cmd/fathom/origin.go',
  'cmd/fathom/configpanel.go', 'cmd/fathom/fleetreport.go', 'cmd/fathom/ui/app.css',
  'third_party/app-kit/.vendor-commit', 'third_party/app-kit/appkit/ui/dashboard/nav.go',
  'third_party/app-kit/appkit/ui/dashboard/shell.go'];
const localFiles = ['recovery_run.mjs', 'recovery_browser.mjs', 'recovery_run_test.mjs',
  'actual_handler_recovery_probe_test.go.txt', 'actual_handler_probe_test.go.txt',
  'recovery_contract.json', 'recovery.log.jsonl'].map(file => join(relativeFixture, file))
  .concat('apps/gallery/browser/config.mjs');
const originals = ['run.mjs', 'run_test.mjs', 'browser.mjs', 'actual_handler_probe_test.go.txt'];

async function regular(path, max = 2 ** 20) {
  const st = await lstat(path);
  if (!st.isFile() || st.size > max || await realpath(path) !== path)
    throw Error('Expected bounded canonical regular file: ' + path);
  return readFile(path);
}
async function directory(path) {
  if (!isAbsolute(path) || !(await lstat(path)).isDirectory() || await realpath(path) !== path)
    throw Error('Expected canonical absolute directory: ' + path);
  return path;
}
async function exclusive(path, bytes) {
  await mkdir(dirname(path), {recursive: true, mode: 0o700});
  await writeFile(path, bytes, {flag: 'wx', mode: 0o600});
}
async function absent(path) {
  try { await lstat(path); } catch (error) { if (error.code === 'ENOENT') return; throw error; }
  throw Error('Refusing to shadow existing consumer file: ' + path);
}
// Own the process group so timeout cannot silently leave the Go/Node child tree.
async function command(command, args, options = {}) {
  const {timeout = 300000, ...spawnOptions} = options;
  return new Promise((resolveCommand, reject) => {
    const child = spawn(command, args, {...spawnOptions, detached: true, stdio: ['ignore', 'pipe', 'pipe']});
    const chunks = {stdout: [], stderr: []}; let size = 0, reason;
    const kill = () => {try {process.kill(-child.pid, 'SIGKILL');} catch (error) {if (error.code !== 'ESRCH') reason ||= String(error);}};
    const timer = setTimeout(() => {reason = 'command timeout'; kill();}, timeout);
    for (const channel of ['stdout', 'stderr']) child[channel].on('data', bytes => {
      size += bytes.length;
      if (size > 8 * 2 ** 20) {reason = 'command output bound'; kill(); return;}
      chunks[channel].push(bytes);
    });
    child.on('error', error => {clearTimeout(timer); reject(error);});
    child.on('close', (code, signal) => {
      clearTimeout(timer);
      resolveCommand({command, args, pid: child.pid, code, signal, reason,
        stdout: Buffer.concat(chunks.stdout).toString(), stderr: Buffer.concat(chunks.stderr).toString()});
    });
  });
}
const assertSuccess = result => {
  if (result.code !== 0 || result.signal || result.reason) throw Error('Command failed: ' + JSON.stringify({command: result.command, code: result.code, signal: result.signal, reason: result.reason}));
  return result.stdout;
};
async function identities(base, files) {
  return Object.fromEntries(await Promise.all(files.map(async file => [file, sha(await regular(join(base, file)))])));
}

async function prepare(args) {
  if (args.length !== 2 || args.some(arg => !isAbsolute(arg)))
    throw Error('Expected explicit absolute consumer and existing approved Playwright module');
  const consumer = await directory(await realpath(args[0]));
  const modulePath = await realpath(args[1]);
  const playwrightSHA256 = sha(await regular(modulePath));
  if (!(await regular(join(consumer, 'go.mod'))).toString().startsWith('module github.com/geoffmilleraz/fathom\n'))
    throw Error('Unexpected consumer module');
  const evidenceBase = await directory(process.env.APPKIT_FATHOM_RECOVERY_EVIDENCE_ROOT || await realpath(tmpdir()));
  const root = await mkdtemp(join(evidenceBase, 'appkit-fathom-recovery-'));
  console.log('Recovery evidence: ' + root);
  const kit = resolve(fixture, '../../../..'), frozen = join(root, 'source-snapshot');
  const config = {schema: 'appkit-fathom-recovery-run-v1', root, kit, consumer, modulePath,
    playwrightSHA256, token: randomBytes(32).toString('hex'),
    compilerCache: process.env.APPKIT_FATHOM_HANDLER_CACHE
      ? await directory(process.env.APPKIT_FATHOM_HANDLER_CACHE) : join(root, 'go-build-cache'),
    reusedCompilerCache: !!process.env.APPKIT_FATHOM_HANDLER_CACHE,
    preparedAt: new Date().toISOString(), snapshots: {}, selectedSources: {},
    originalFixtures: await identities(fixture, originals)};
  for (const file of localFiles) {
    const bytes = await regular(join(kit, file));
    await exclusive(join(frozen, file), bytes); config.snapshots[file] = sha(bytes);
  }
  for (const file of selected) {
    const bytes = await regular(join(consumer, file));
    await exclusive(join(root, 'consumer-selected-source', file), bytes); config.selectedSources[file] = sha(bytes);
  }
  await exclusive(join(root, 'run-config.json'), JSON.stringify(config, null, 2) + '\n');
  // Execute only retained code; subsequent edits cannot replace a failed harness.
  const child = await command(process.execPath, [join(frozen, relativeFixture, 'recovery_run.mjs'), '--frozen', root],
    {env: process.env, timeout: 600000});
  await exclusive(join(root, 'launcher-result.json'), JSON.stringify(child, null, 2) + '\n');
  process.stdout.write(child.stdout); process.stderr.write(child.stderr);
  assertSuccess(child);
}

async function runFrozen(root) {
  await directory(root);
  const configBytes = await regular(join(root, 'run-config.json'));
  const config = JSON.parse(configBytes);
  if (config.schema !== 'appkit-fathom-recovery-run-v1' || config.root !== root ||
      resolve(fixture, '../../../../..') !== root || !/^[a-f0-9]{64}$/.test(config.token))
    throw Error('Frozen runner/configuration location mismatch');
  const snapshotRoot = join(root, 'source-snapshot');
  const env = {...process.env, GOWORK: 'off', GOPROXY: 'off', GOSUMDB: 'off', GOENV: 'off',
    GOTOOLCHAIN: 'local', GOFLAGS: '', GOCACHE: config.compilerCache,
    TMPDIR: join(root, 'tmp'), GOTMPDIR: join(root, 'tmp'),
    APPKIT_PLAYWRIGHT_MODULE: config.modulePath,
    APPKIT_FATHOM_RECOVERY_CONTRACT: join(fixture, 'recovery_contract.json'),
    APPKIT_FATHOM_RECOVERY_TOKEN: config.token,
    APPKIT_FATHOM_FROZEN_ROOT: root,
    APPKIT_FATHOM_HANDLER_BROWSER: join(fixture, 'recovery_browser.mjs'),
    APPKIT_FATHOM_HANDLER_REPORT_ROOT: join(root, 'reports')};
  for (const key of Object.keys(env)) if (key.startsWith('GIT_')) delete env[key];
  const virtualNames = ['appkit_actual_handler_probe_test.go', 'appkit_actual_handler_recovery_probe_test.go'];
  const virtual = virtualNames.map(name => join(config.consumer, 'cmd/fathom', name));
  const report = {startedAt: new Date().toISOString(), mode: 'unchanged-consumer-recovery-overlay',
    configSHA256: sha(configBytes), selectedSources: config.selectedSources,
    commands: [], passed: false, limitations: ['Selected source identities, not a complete immutable build closure.',
      'Offline existing dependencies and optional reused compiler cache, not a cold build or clean consumer.',
      'Synthetic Chrome read-only recovery, not production, native, or full G2 acceptance.']};
  const run = async (bin, args, timeout) => {
    const result = await command(bin, args, {cwd: config.consumer, env, timeout});
    report.commands.push(result); return assertSuccess(result);
  };
  const git = args => run('git', ['-C', config.consumer, ...args], 20000);
  try {
    if (JSON.stringify(await identities(snapshotRoot, localFiles)) !== JSON.stringify(config.snapshots))
      throw Error('Frozen harness drift');
    if (JSON.stringify(await identities(config.consumer, selected)) !== JSON.stringify(config.selectedSources))
      throw Error('Consumer drift before execution');
    if (sha(await regular(config.modulePath)) !== config.playwrightSHA256) throw Error('Playwright entry drift');
    for (const path of virtual) await absent(path);
    if (await realpath((await git(['rev-parse', '--show-toplevel'])).trim()) !== config.consumer)
      throw Error('Expected canonical consumer Git root');
    report.head = (await git(['rev-parse', 'HEAD'])).trim();
    report.dirtyBefore = await git(['status', '--porcelain=v1', '--untracked-files=all']);
    const contract = JSON.parse(await regular(join(fixture, 'recovery_contract.json')));
    if (contract.version !== 1 || contract.log.file !== 'recovery.log.jsonl' ||
        sha(await regular(join(fixture, contract.log.file))) !== contract.log.sha256)
      throw Error('Synthetic log contract mismatch');
    const overlay = join(root, 'overlay.json');
    await exclusive(overlay, JSON.stringify({Replace: {
      [virtual[0]]: join(fixture, 'actual_handler_probe_test.go.txt'),
      [virtual[1]]: join(fixture, 'actual_handler_recovery_probe_test.go.txt')
    }}, null, 2) + '\n');
    await mkdir(env.APPKIT_FATHOM_HANDLER_REPORT_ROOT, {mode: 0o700});
    await mkdir(env.TMPDIR, {mode: 0o700});
    const refusalEnv = {...env}; delete refusalEnv.APPKIT_FATHOM_FROZEN_ROOT;
    const refusal = await command(process.execPath, ['--test', join(fixture, 'recovery_run_test.mjs')],
      {cwd: config.consumer, env: refusalEnv, timeout: 30000});
    report.commands.push({...refusal, expectedRefusal: true});
    if (refusal.code !== 1 || refusal.signal || refusal.reason ||
        !(refusal.stdout + refusal.stderr).includes('unfrozen execution refused'))
      throw Error('Missing frozen-root control did not fail closed');
    await run(process.execPath, ['--test', join(fixture, 'recovery_run_test.mjs')], 30000);
    report.goVersion = (await run('go', ['version'], 10000)).trim();
    const listed = await run('go', ['list', '-mod=readonly', '-overlay', overlay,
      '-f', '{{range .TestGoFiles}}{{println .}}{{end}}', './cmd/fathom'], 120000);
    if (!virtualNames.every(name => listed.split('\n').includes(name))) throw Error('Go did not discover both retained overlays');
    const name = 'TestAppkitActualObservatoryRecoveryProbe';
    const stdout = await run('go', ['test', '-race', '-mod=readonly', '-overlay', overlay,
      './cmd/fathom', '-run', '^' + name + '$', '-count=1', '-v', '-timeout=260s'], 360000);
    if (!stdout.includes('--- PASS: ' + name + ' ')) throw Error('Named recovery test did not pass');
    report.passed = true;
  } catch (error) { report.error = String(error); }
  finally {
    try {
      report.sourceAfter = await identities(config.consumer, selected);
      report.sourceUnchanged = JSON.stringify(report.sourceAfter) === JSON.stringify(config.selectedSources);
      report.headUnchanged = !report.head || report.head === (await git(['rev-parse', 'HEAD'])).trim();
      report.dirtyUnchanged = report.dirtyBefore === undefined || report.dirtyBefore === await git(['status', '--porcelain=v1', '--untracked-files=all']);
      report.snapshotsUnchanged = JSON.stringify(await identities(snapshotRoot, localFiles)) === JSON.stringify(config.snapshots);
      report.originalFixturesUnchanged = JSON.stringify(await identities(join(config.kit, relativeFixture), originals)) === JSON.stringify(config.originalFixtures);
      report.virtualFilesAbsent = true;
      for (const path of virtual) await absent(path);
      report.configUnchanged = sha(await regular(join(root, 'run-config.json'))) === report.configSHA256;
      if (!report.sourceUnchanged || !report.headUnchanged || !report.dirtyUnchanged || !report.snapshotsUnchanged ||
          !report.originalFixturesUnchanged || !report.configUnchanged) report.passed = false;
    } catch (error) { report.cleanupError = String(error); report.passed = false; }
    report.finishedAt = new Date().toISOString();
    await exclusive(join(root, 'run-report.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify({passed: report.passed, error: report.error, cleanupError: report.cleanupError,
      report: join(root, 'run-report.json')}));
  }
  if (!report.passed) process.exitCode = 1;
}

try {
  const args = process.argv.slice(2);
  if (args[0] === '--frozen' && args.length === 2) await runFrozen(args[1]);
  else await prepare(args);
} catch (error) { console.error(String(error)); process.exitCode = 1; }
