// Private, bounded macOS runtime evidence for two already-built CLI consumers.
// Only help, version, and deliberately invalid syntax are invoked.
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import * as fs from 'node:fs/promises';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const DEADLINE_MS = 5_000;
const STREAM_LIMIT_BYTES = 64 * 1024;
const EXPECTED_VERSION = 'v0.1.0-dev';
const FIXED_CASES = [
  { id: 'help', args: ['--help'], kind: 'success' },
  { id: 'version', args: ['--version'], kind: 'version' },
  { id: 'unknown-flag', args: ['--appkit-probe-unknown-flag'], kind: 'usage' },
  { id: 'unknown-command', args: ['appkit-probe-unknown-command'], kind: 'usage' },
];
const ENV = {
  NO_COLOR: '1',
  TERM: 'dumb',
  PATH: '/usr/bin:/bin:/usr/sbin:/sbin',
};

function fail(message) {
  throw new Error(message);
}

function sha256(value) {
  return createHash('sha256').update(value).digest('hex');
}

async function hashFile(file) {
  const hash = createHash('sha256');
  const handle = await fs.open(file, 'r');
  try {
    const buffer = Buffer.allocUnsafe(64 * 1024);
    for (;;) {
      const { bytesRead } = await handle.read(buffer, 0, buffer.length, null);
      if (bytesRead === 0) break;
      hash.update(buffer.subarray(0, bytesRead));
    }
  } finally {
    await handle.close();
  }
  return hash.digest('hex');
}

async function canonicalExecutable(input, label) {
  if (!path.isAbsolute(input)) fail(`${label} binary path must be absolute`);
  const resolved = await fs.realpath(input);
  const info = await fs.stat(resolved);
  if (!info.isFile()) fail(`${label} binary is not a regular file`);
  await fs.access(resolved, fs.constants.X_OK);
  return { path: resolved, sha256: await hashFile(resolved) };
}

async function expectAbsent(file) {
  try {
    await fs.lstat(file);
    return false;
  } catch (error) {
    if (error?.code === 'ENOENT') return true;
    throw error;
  }
}

function boundedOutput(chunks, observedBytes, limited) {
  const bytes = Buffer.concat(chunks);
  return {
    bytes: bytes.length,
    observed_bytes_before_stop: observedBytes,
    limited,
    sha256: sha256(bytes),
    raw_text: bytes.toString('utf8'),
  };
}

async function capture(binary, args, cwd) {
  return await new Promise(resolve => {
    const stdout = [];
    const stderr = [];
    const state = {
      stdout: { captured: 0, observed: 0, limited: false },
      stderr: { captured: 0, observed: 0, limited: false },
    };
    let child;
    let timedOut = false;
    let spawnError = null;
    let stopRequested = false;

    const stop = () => {
      if (stopRequested) return;
      stopRequested = true;
      child?.kill('SIGKILL');
    };
    const collect = (name, chunks) => data => {
      const bytes = Buffer.from(data);
      const stream = state[name];
      stream.observed += bytes.length;
      const available = Math.max(0, STREAM_LIMIT_BYTES - stream.captured);
      if (available > 0) {
        const kept = bytes.subarray(0, available);
        chunks.push(kept);
        stream.captured += kept.length;
      }
      if (bytes.length > available) {
        stream.limited = true;
        stop();
      }
    };

    child = spawn(binary, args, {
      cwd,
      env: ENV,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    child.stdout.on('data', collect('stdout', stdout));
    child.stderr.on('data', collect('stderr', stderr));
    child.on('error', error => { spawnError = String(error); });
    const timer = setTimeout(() => {
      timedOut = true;
      stop();
    }, DEADLINE_MS);
    child.on('close', (code, signal) => {
      clearTimeout(timer);
      resolve({
        code,
        signal,
        timed_out: timedOut,
        spawn_error: spawnError,
        stdout: boundedOutput(stdout, state.stdout.observed, state.stdout.limited),
        stderr: boundedOutput(stderr, state.stderr.observed, state.stderr.limited),
      });
    });
  });
}

function sandboxProfile(forbiddenRead) {
  return [
    '(version 1)',
    '(allow default)',
    '(deny network*)',
    '(deny file-write*)',
    '(deny file-read*',
    `  (subpath ${JSON.stringify('/Users')})`,
    `  (subpath ${JSON.stringify('/Volumes/m2ext/code')})`,
    `  (subpath ${JSON.stringify('/Library/Keychains')})`,
    `  (literal ${JSON.stringify(forbiddenRead)}))`,
    '',
  ].join('\n');
}

export function completedNormally(result) {
  return result.spawn_error === null && result.signal === null &&
    !result.timed_out && !result.stdout.limited && !result.stderr.limited;
}

function hasANSI(result) {
  return result.stdout.raw_text.includes('\x1b') || result.stderr.raw_text.includes('\x1b');
}

export function consumerExpectation(testCase, result) {
  const common = completedNormally(result) && !hasANSI(result);
  if (testCase.kind === 'usage') {
    const reporting = common && result.code !== 0 && result.stdout.bytes === 0 &&
      result.stderr.raw_text.trim().length > 0;
    const exitPolicy = common && result.code === 2;
    return {
      passed: reporting && exitPolicy,
      expected: 'exit 2, empty stdout, nonempty stderr, no ANSI, no timeout or output truncation',
      dimensions: {
        reporting: {
          passed: reporting,
          expected: 'nonzero exit, empty stdout, useful stderr diagnostic, no ANSI, timeout or output truncation',
        },
        exit_policy: {
          passed: exitPolicy,
          expected: 'usage syntax exits 2 without signal, spawn failure, timeout or output truncation',
        },
      },
    };
  }
  const versionPresent = testCase.kind !== 'version' || result.stdout.raw_text.includes(EXPECTED_VERSION);
  return {
    passed: common && result.code === 0 && result.stdout.raw_text.trim().length > 0 &&
      result.stderr.bytes === 0 && versionPresent,
    expected: testCase.kind === 'version'
      ? `exit 0, nonempty stdout containing ${EXPECTED_VERSION}, empty stderr, no ANSI, no timeout or output truncation`
      : 'exit 0, nonempty stdout, empty stderr, no ANSI, no timeout or output truncation',
  };
}

async function reachableLoopback(url) {
  return await new Promise((resolve, reject) => {
    const request = http.get(url, { agent: false }, response => {
      response.resume();
      response.on('end', () => resolve(response.statusCode));
    });
    request.setTimeout(2_000, () => request.destroy(new Error('loopback self-check timed out')));
    request.on('error', reject);
  });
}

async function closeServer(server) {
  if (typeof server.closeAllConnections === 'function') server.closeAllConnections();
  await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
}

async function main() {
  const [bosunArg, surveyorArg, outputArg, ...extra] = process.argv.slice(2);
  if (!bosunArg || !surveyorArg || !outputArg || extra.length > 0) {
    fail('usage: node probe.mjs /absolute/bosun /absolute/surveyor /absolute/existing-empty-output-dir');
  }
  if (process.platform !== 'darwin') fail('this probe requires macOS sandbox-exec');
  if (!path.isAbsolute(outputArg)) fail('output directory path must be absolute');

  const outputLstat = await fs.lstat(outputArg);
  if (!outputLstat.isDirectory() || outputLstat.isSymbolicLink()) fail('output must be an existing real directory');
  if (typeof process.getuid === 'function' && outputLstat.uid !== process.getuid()) fail('output directory must be owned by this user');
  if ((await fs.readdir(outputArg)).length !== 0) fail('output directory must start empty');
  const output = await fs.realpath(outputArg);
  const binaries = {
    bosun: await canonicalExecutable(bosunArg, 'bosun'),
    surveyor: await canonicalExecutable(surveyorArg, 'surveyor'),
  };

  const checksDir = path.join(output, 'checks');
  const workDir = path.join(output, 'work');
  await fs.mkdir(checksDir, { mode: 0o700 });
  await fs.mkdir(workDir, { mode: 0o700 });
  const cwd = {};
  for (const name of ['canary', 'bosun', 'surveyor']) {
    cwd[name] = path.join(workDir, name);
    await fs.mkdir(cwd[name], { mode: 0o700 });
  }

  const forbiddenRead = path.join(checksDir, 'forbidden-read.txt');
  const forbiddenWrite = path.join(checksDir, 'forbidden-write.txt');
  await fs.writeFile(forbiddenRead, 'synthetic private read canary\n', { flag: 'wx', mode: 0o600 });
  const profile = sandboxProfile(forbiddenRead);
  const profilePath = path.join(checksDir, 'sandbox.sb');
  await fs.writeFile(profilePath, profile, { flag: 'wx', mode: 0o600 });

  const sandbox = (binary, args, processCwd) =>
    capture('/usr/bin/sandbox-exec', ['-p', profile, binary, ...args], processCwd);
  const checks = [];
  const readCanary = await sandbox('/bin/cat', [forbiddenRead], cwd.canary);
  checks.push({
    id: 'sandbox-denies-synthetic-read',
    passed: completedNormally(readCanary) && readCanary.code !== 0 && readCanary.stdout.bytes === 0 &&
      /Operation not permitted|Permission denied/.test(readCanary.stderr.raw_text),
    expected: 'nonzero exit caused by a permission denial; empty stdout',
    result: readCanary,
  });

  const writeCanary = await sandbox('/usr/bin/touch', [forbiddenWrite], cwd.canary);
  checks.push({
    id: 'sandbox-denies-synthetic-write',
    passed: completedNormally(writeCanary) && writeCanary.code !== 0 &&
      /Operation not permitted|Permission denied/.test(writeCanary.stderr.raw_text) && await expectAbsent(forbiddenWrite),
    expected: 'nonzero exit caused by a permission denial; no write artifact',
    artifact_absent: await expectAbsent(forbiddenWrite),
    result: writeCanary,
  });

  let sandboxConnections = 0;
  let connectionPhase = 'self-check';
  const server = http.createServer((_request, response) => {
    response.writeHead(200, { 'Content-Type': 'text/plain' });
    response.end('owned loopback canary\n');
  });
  server.on('connection', () => {
    if (connectionPhase === 'sandbox') sandboxConnections += 1;
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string') fail('owned loopback server did not bind an IP port');
  const loopbackURL = `http://127.0.0.1:${address.port}/`;
  let parentStatus;
  let networkCanary;
  try {
    parentStatus = await reachableLoopback(loopbackURL);
    connectionPhase = 'sandbox';
    networkCanary = await sandbox('/usr/bin/curl', [
      '--fail', '--silent', '--show-error', '--max-time', '2', loopbackURL,
    ], cwd.canary);
    await new Promise(resolve => setImmediate(resolve));
  } finally {
    connectionPhase = 'closed';
    await closeServer(server);
  }
  checks.push({
    id: 'sandbox-denies-owned-loopback-network',
    passed: parentStatus === 200 && completedNormally(networkCanary) && networkCanary.code !== 0 && sandboxConnections === 0,
    expected: 'parent self-check HTTP 200; sandboxed curl nonzero; zero server connections during sandbox attempt',
    parent_status: parentStatus,
    sandbox_connections: sandboxConnections,
    result: networkCanary,
  });

  const canariesPassed = checks.every(check => check.passed);
  const runs = [];
  if (canariesPassed) {
    for (const [consumer, executable] of Object.entries(binaries)) {
      for (const testCase of FIXED_CASES) {
        const result = await sandbox(executable.path, testCase.args, cwd[consumer]);
        const expectation = consumerExpectation(testCase, result);
        const run = {
          id: `${consumer}-${testCase.id}`,
          consumer,
          binary_sha256: executable.sha256,
          args: testCase.args,
          expected: expectation.expected,
          passed: expectation.passed,
          ...(expectation.dimensions ? { dimensions: expectation.dimensions } : {}),
          result,
        };
        runs.push(run);
        checks.push({ id: run.id, passed: run.passed, expected: run.expected });
      }
      const entries = await fs.readdir(cwd[consumer]);
      checks.push({
        id: `${consumer}-cwd-remains-empty`,
        passed: entries.length === 0,
        expected: 'owned process cwd remains empty',
        entries: entries.sort(),
      });
    }
  }

  const status = canariesPassed && checks.every(check => check.passed) ? 'passed' : 'failed';
  const report = {
    schema_version: 1,
    kind: 'app-kit-cli-consumer-runtime-probe',
    status,
    host: {
      hostname: os.hostname(),
      platform: process.platform,
      architecture: process.arch,
      os_release: os.release(),
      node: process.version,
    },
    bounds: {
      deadline_ms: DEADLINE_MS,
      output_limit_bytes_per_stream: STREAM_LIMIT_BYTES,
      environment: ENV,
      allowed_consumer_arguments: FIXED_CASES.map(testCase => testCase.args),
    },
    binaries,
    sandbox: {
      executable: '/usr/bin/sandbox-exec',
      profile_path: profilePath,
      profile_sha256: sha256(profile),
      definition: profile,
      denied: ['all network', 'all file writes', 'reads under /Users',
        'reads under /Volumes/m2ext/code', 'reads under /Library/Keychains', 'synthetic sentinel read'],
    },
    canaries_passed: canariesPassed,
    checks,
    runs,
    limits: [
      'Local macOS evidence for two exact binary hashes, not source, release, fleet, or non-macOS proof.',
      'Only help, version, and invalid syntax were invoked; no real consumer leaf command ran.',
      'The sandbox keeps default syscall allowances except the recorded network, write, and read denials.',
      'Raw bounded process output is retained only in this operator-owned private output directory.',
    ],
  };
  const reportPath = path.join(checksDir, 'report.json');
  await fs.writeFile(reportPath, `${JSON.stringify(report, null, 2)}\n`, { flag: 'wx', mode: 0o600 });
  console.log(JSON.stringify({ status, report: reportPath, checks: checks.length,
    failures: checks.filter(check => !check.passed).map(check => check.id) }));
  if (status !== 'passed') process.exitCode = 1;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch(error => {
    console.error(String(error));
    process.exitCode = 1;
  });
}
