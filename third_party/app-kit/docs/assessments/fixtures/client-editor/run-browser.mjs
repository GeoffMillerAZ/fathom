// Own the exact P12 Go host and browser-harness lifecycle for one retained trial.
// This runner installs nothing and writes only beneath the selected private build.
import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { spawn } from 'node:child_process';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const [buildArg, zoomArg] = process.argv.slice(2);
if (!buildArg || ![3,4].includes(process.argv.length) || (zoomArg !== undefined && zoomArg !== '--zoom=200') || !path.isAbsolute(buildArg) || path.normalize(buildArg) !== buildArg) {
  throw new Error('Usage: node run-browser.mjs ABSOLUTE_PRIVATE_BUILD_DIR [--zoom=200]');
}
const buildInput = await fs.lstat(buildArg);
if (!buildInput.isDirectory() || buildInput.isSymbolicLink() || (buildInput.mode & 0o077) !== 0) {
  throw new Error('Build must be a private non-symlink directory');
}
const build = await fs.realpath(buildArg);
if (build !== buildArg) throw new Error('Build path must already be canonical');

const here = path.dirname(fileURLToPath(import.meta.url));
const trial = await fs.mkdtemp(path.join(build, 'trial-'));
await fs.chmod(trial, 0o700);
const sha = body => createHash('sha256').update(body).digest('hex');
const write = (name, body) => fs.writeFile(path.join(trial, name), body, { flag: 'wx', mode: 0o600 });
const receipt = {
  schema: 1,
  kind: 'p12-client-editor-owned-host-browser-trial',
  verification: 'scoped-synthetic-evidence',
  zoom: zoomArg ? 200 : 100,
  build,
  trial,
  startedAt: new Date().toISOString(),
  toolchain: { node: process.version, platform: os.platform(), arch: os.arch(), release: os.release() },
  snapshots: [],
  commands: [],
  probes: [],
  errors: [],
};
let hostChild;
let hostExit;
let hostStdout = Buffer.alloc(0);
let hostStderr = Buffer.alloc(0);
const hostStdoutChunks = [];
const hostStderrChunks = [];
let browserChild;
let browserExit;
let startup;

function errorText(error) {
  return error instanceof Error ? `${error.message}${error.stack ? `\n${error.stack}` : ''}` : String(error);
}

async function regularSource(name) {
  const info = await fs.lstat(name);
  if (!info.isFile() || info.isSymbolicLink() || info.size < 1 || info.size > 2 * 1024 * 1024) {
    throw new Error(`Required bounded regular source missing: ${name}`);
  }
  return fs.readFile(name);
}

async function snapshot(from, relative) {
  const body = await regularSource(from);
  const target = path.join(trial, relative);
  await fs.mkdir(path.dirname(target), { recursive: true, mode: 0o700 });
  // Write the bytes that were hashed, rather than reopening a mutable source.
  await fs.writeFile(target, body, { flag: 'wx', mode: 0o600 });
  const record = { path: relative, sha256: sha(body), bytes: body.length };
  receipt.snapshots.push(record);
  return { target, body, record };
}

function collectBounded(chunks, limit, child, label) {
  let size = 0;
  return chunk => {
    size += chunk.length;
    if (size > limit) {
      child.kill('SIGTERM');
      receipt.errors.push(`${label} output exceeded ${limit} bytes`);
      return;
    }
    chunks.push(Buffer.from(chunk));
  };
}

async function command(label, executable, args, options = {}) {
  const stdout = [], stderr = [];
  const started = Date.now();
  const timeoutMS = options.timeoutMS ?? 60_000;
  const limit = options.outputLimit ?? 1024 * 1024;
  const record = {
    label,
    executable,
    args,
    cwd: options.cwd ?? trial,
    environment: options.recordEnvironment ?? {},
    timeoutMS,
  };
  receipt.commands.push(record);
  const child = spawn(executable, args, {
    cwd: record.cwd,
    env: options.env ?? process.env,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let spawnError = '';
  const close = new Promise(resolve => {
    child.once('error', error => { spawnError = errorText(error); });
    child.once('close', (code, signal) => resolve({ code, signal, spawnError }));
  });
  if (label === 'browser') {
    browserChild = child;
    browserExit = close;
  }
  child.stdout.on('data', collectBounded(stdout, limit, child, `${label} stdout`));
  child.stderr.on('data', collectBounded(stderr, limit, child, `${label} stderr`));
  let timedOut = false;
  let forced = false;
  let forceTimer;
  const timer = setTimeout(() => {
    timedOut = true;
    child.kill('SIGTERM');
    forceTimer = setTimeout(() => {
      if (child.exitCode === null && child.signalCode === null) {
        forced = true;
        child.kill('SIGKILL');
      }
    }, 6_000);
  }, timeoutMS);
  const result = await close;
  clearTimeout(timer);
  clearTimeout(forceTimer);
  if (label === 'browser') {
    browserChild = undefined;
    browserExit = undefined;
  }
  const out = Buffer.concat(stdout), err = Buffer.concat(stderr);
  await write(`${label}.stdout`, out);
  await write(`${label}.stderr`, err);
  Object.assign(record, {
    ...result,
    timedOut,
    forced,
    milliseconds: Date.now() - started,
    stdoutSHA256: sha(out),
    stderrSHA256: sha(err),
    stdoutBytes: out.length,
    stderrBytes: err.length,
  });
  return { ...record, stdout: out, stderr: err };
}

async function request(origin, name, method, target, headers = {}) {
  const url = new URL(target, origin);
  const result = await new Promise((resolve, reject) => {
    const req = http.request({
      protocol: url.protocol,
      hostname: url.hostname,
      port: url.port,
      path: `${url.pathname}${url.search}`,
      method,
      headers,
      agent: false,
    }, response => {
      const chunks = [];
      let size = 0;
      response.on('data', chunk => {
        size += chunk.length;
        if (size > 2 * 1024 * 1024) {
          response.destroy(new Error(`${name} response exceeded 2 MiB`));
          return;
        }
        chunks.push(Buffer.from(chunk));
      });
      response.once('error', reject);
      response.once('end', () => resolve({
        status: response.statusCode,
        headers: response.headers,
        rawHeaders: response.rawHeaders,
        body: Buffer.concat(chunks),
      }));
    });
    req.once('error', reject);
    req.setTimeout(5_000, () => req.destroy(new Error(`${name} timed out`)));
    req.end();
  });
  const record = {
    name,
    method,
    target,
    requestHeaders: headers,
    status: result.status,
    responseHeaders: result.headers,
    rawResponseHeaders: result.rawHeaders,
    bodyBytes: result.body.length,
    bodySHA256: sha(result.body),
  };
  receipt.probes.push(record);
  return { ...result, record };
}

function requireProbe(condition, message) {
  if (!condition) throw new Error(`Live host probe failed: ${message}`);
}

async function stopOwnedHost() {
  if (!hostChild || !hostExit) return;
  const alreadyExited = hostChild.exitCode !== null || hostChild.signalCode !== null;
  const cleanup = { pid: hostChild.pid, sentSIGTERM: false, forcedSIGKILL: false, alreadyExited };
  receipt.hostCleanup = cleanup;
  if (alreadyExited) {
    cleanup.exit = await hostExit;
    receipt.errors.push('Owned host exited before requested shutdown');
    return;
  }
  cleanup.sentSIGTERM = hostChild.kill('SIGTERM');
  let timer;
  const outcome = await Promise.race([
    hostExit.then(exit => ({ exit })),
    new Promise(resolve => { timer = setTimeout(() => resolve({ timeout: true }), 6_000); }),
  ]);
  clearTimeout(timer);
  if (outcome.timeout) {
    cleanup.forcedSIGKILL = hostChild.kill('SIGKILL');
    receipt.errors.push('Owned host did not stop within six seconds after SIGTERM');
    cleanup.exit = await hostExit;
  } else {
    cleanup.exit = outcome.exit;
  }
  if (cleanup.exit.code !== 0 || cleanup.exit.signal !== null || cleanup.exit.spawnError) {
    receipt.errors.push(`Owned host shutdown was not clean: ${JSON.stringify(cleanup.exit)}`);
  }
}

async function stopOwnedBrowser() {
  if (!browserChild || !browserExit) return;
  const alreadyExited = browserChild.exitCode !== null || browserChild.signalCode !== null;
  const cleanup = { pid: browserChild.pid, sentSIGTERM: false, forcedSIGKILL: false, alreadyExited };
  receipt.browserCleanup = cleanup;
  receipt.errors.push('Browser child remained active during runner cleanup');
  const waitForExit = async () => {
    let timer;
    const outcome = await Promise.race([
      browserExit.then(exit => ({ exit })),
      new Promise(resolve => { timer = setTimeout(() => resolve({ timeout: true }), 6_000); }),
    ]);
    clearTimeout(timer);
    return outcome;
  };
  if (alreadyExited) {
    const outcome = await waitForExit();
    if (outcome.timeout) {
      cleanup.exitMissing = true;
      receipt.errors.push('Owned browser had exit status but did not report close within grace period');
    } else {
      cleanup.exit = outcome.exit;
    }
    return;
  }
  cleanup.sentSIGTERM = browserChild.kill('SIGTERM');
  let outcome = await waitForExit();
  if (outcome.timeout) {
    cleanup.forcedSIGKILL = browserChild.kill('SIGKILL');
    outcome = await waitForExit();
  }
  if (outcome.timeout) {
    cleanup.exitMissing = true;
    receipt.errors.push('Owned browser did not report exit after SIGKILL grace period');
  } else {
    cleanup.exit = outcome.exit;
  }
}

try {
  const sources = [
    [fileURLToPath(import.meta.url), 'run-browser.mjs'],
    [path.join(here, 'browser.mjs'), 'browser.mjs'],
    [path.join(here, 'scale-tasks.mjs'), 'scale-tasks.mjs'],
    [path.join(here, 'morph-tasks.mjs'), 'morph-tasks.mjs'],
    [path.resolve(here, '../matdash-hardened/editor-tasks.mjs'), 'editor-tasks.mjs'],
    [path.resolve(here, '../gallery-analytics/zoom-contract.mjs'), 'zoom-contract.mjs'],
    [path.resolve(here, '../catalyst-settings-adapter/zoom-capture.mjs'), 'zoom-capture.mjs'],
    [path.join(build, 'host/main.go'), 'host/main.go'],
    [path.join(build, 'host/main_test.go'), 'host/main_test.go'],
    [path.join(build, 'dist-a/manifest.json'), 'manifest.json'],
  ];
  const retained = new Map();
  for (const [from, relative] of sources) retained.set(relative, await snapshot(from, relative));

  const manifestBody = retained.get('manifest.json').body;
  const manifest = JSON.parse(manifestBody);
  if (manifest.schema !== 1 || !Array.isArray(manifest.files) || typeof manifest.entry !== 'string' || !Array.isArray(manifest.styles)) {
    throw new Error('Retained build manifest has an invalid shape');
  }
  const manifestSHA256 = sha(manifestBody);
  receipt.manifest = { sha256: manifestSHA256, bytes: manifestBody.length, path: 'manifest.json' };

  const buildEnvironment = {
    GOCACHE: '/Volumes/m2ext/appkit-menu-proof.Vyw3imOW/go-cache',
    GOMODCACHE: '/Users/geoff/.dev-cache/go/modules',
    GOWORK: 'off',
    GOPROXY: 'off',
    GOSUMDB: 'off',
    GOENV: 'off',
    GOTOOLCHAIN: 'local',
  };
  const goEnvironment = { ...process.env, ...buildEnvironment };
  const goVersion = await command('go-version', 'go', ['version'], {
    env: goEnvironment,
    recordEnvironment: buildEnvironment,
    timeoutMS: 10_000,
  });
  if (goVersion.code !== 0 || goVersion.signal !== null || goVersion.timedOut || goVersion.forced || goVersion.spawnError) {
    throw new Error('Pinned local Go version query failed');
  }
  receipt.toolchain.go = goVersion.stdout.toString('utf8').trim();

  const binary = path.join(trial, 'client-editor-host');
  const buildArgs = ['build', '-trimpath', '-o', binary, retained.get('host/main.go').target];
  await write('build-command.json', `${JSON.stringify({ executable: 'go', args: buildArgs, cwd: trial, environment: buildEnvironment }, null, 2)}\n`);
  const compiled = await command('go-build', 'go', buildArgs, {
    env: goEnvironment,
    recordEnvironment: buildEnvironment,
    timeoutMS: 60_000,
  });
  if (compiled.code !== 0 || compiled.signal !== null || compiled.timedOut || compiled.forced || compiled.spawnError) {
    throw new Error('Owned host compilation failed');
  }
  const binaryInfo = await fs.lstat(binary);
  if (!binaryInfo.isFile() || binaryInfo.isSymbolicLink() || (binaryInfo.mode & 0o111) === 0) {
    throw new Error('Compiled host is not a regular executable');
  }
  const binaryBody = await fs.readFile(binary);
  const binarySHA256 = sha(binaryBody);
  const sourceSHA256 = retained.get('host/main.go').record.sha256;
  receipt.hostBuild = {
    sourceSHA256,
    sourceTestSHA256: retained.get('host/main_test.go').record.sha256,
    binarySHA256,
    binaryBytes: binaryBody.length,
  };

  const assets = path.join(build, 'dist-a');
  const hostArgs = ['-assets', assets, '-manifest-sha', manifestSHA256];
  const hostCommand = [binary, ...hostArgs];
  hostChild = spawn(binary, hostArgs, { cwd: trial, stdio: ['ignore', 'pipe', 'pipe'] });
  let stdoutBytes = 0, stderrBytes = 0;
  let startupBuffer = '';
  hostChild.stdout.setEncoding('utf8');
  hostChild.stderr.setEncoding('utf8');
  hostExit = new Promise(resolve => {
    let spawnError = '';
    hostChild.once('error', error => { spawnError = errorText(error); });
    hostChild.once('close', (code, signal) => resolve({ code, signal, spawnError }));
  });
  const startupLine = new Promise((resolve, reject) => {
    let settled = false;
    const fail = error => { if (!settled) { settled = true; reject(error); } };
    hostChild.once('error', fail);
    hostChild.once('close', () => fail(new Error('Owned host exited before startup')));
    hostChild.stdout.on('data', chunk => {
      const body = Buffer.from(chunk);
      stdoutBytes += body.length;
      if (stdoutBytes > 64 * 1024) {
        fail(new Error('Owned host stdout exceeded 64 KiB'));
        hostChild.kill('SIGTERM');
        return;
      }
      hostStdoutChunks.push(body);
      startupBuffer += chunk;
      const newline = startupBuffer.indexOf('\n');
      if (!settled && newline >= 0) {
        settled = true;
        resolve(startupBuffer.slice(0, newline));
      }
    });
    hostChild.stderr.on('data', chunk => {
      const body = Buffer.from(chunk);
      stderrBytes += body.length;
      if (stderrBytes > 64 * 1024) {
        fail(new Error('Owned host stderr exceeded 64 KiB'));
        hostChild.kill('SIGTERM');
        return;
      }
      hostStderrChunks.push(body);
    });
  });
  let startupTimer;
  const rawStartup = await Promise.race([
    startupLine,
    new Promise((_, reject) => { startupTimer = setTimeout(() => reject(new Error('Owned host startup exceeded ten seconds')), 10_000); }),
  ]).finally(() => clearTimeout(startupTimer));
  let parsedStartup;
  try { parsedStartup = JSON.parse(rawStartup); } catch { throw new Error('Owned host startup line is not JSON'); }
  const startupKeys = parsedStartup && typeof parsedStartup === 'object' ? Object.keys(parsedStartup).sort() : [];
  const origin = new URL(parsedStartup?.origin);
  const port = Number(origin.port);
  if (startupKeys.join(',') !== 'manifestSHA,origin' || parsedStartup.manifestSHA !== manifestSHA256 ||
      origin.origin !== parsedStartup.origin || origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' ||
      !origin.port || !Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error('Owned host startup identity is malformed or mismatched');
  }
  startup = {
    origin: origin.origin,
    pid: hostChild.pid,
    command: hostCommand,
    sourceSHA256,
    binarySHA256,
    manifestSHA256,
    rawStartup,
  };
  await write('startup.json', `${JSON.stringify(startup, null, 2)}\n`);
  receipt.host = startup;

  const csp = "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'self'; frame-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; object-src 'none'";
  const page = await request(origin.origin, 'root-get', 'GET', '/');
  requireProbe(page.status === 200, 'GET / status');
  requireProbe(page.headers['content-security-policy'] === csp, 'GET / exact CSP');
  await write('root.html', page.body);
  page.record.retainedBody = 'root.html';

  const context = await request(origin.origin, 'context-get', 'GET', '/context');
  requireProbe(context.status === 200, 'GET /context status');
  let contextValue;
  try { contextValue = JSON.parse(context.body); } catch { throw new Error('GET /context returned invalid JSON'); }
  const contextKeys = contextValue && typeof contextValue === 'object' ? Object.keys(contextValue).sort() : [];
  requireProbe(contextKeys.join(',') === 'hint,instanceId,label,version' && contextValue.version === 1 &&
    contextValue.instanceId === 'primary' && contextValue.label === 'Updated product description' &&
    contextValue.hint === 'Context updated. Your local draft is unchanged.', 'GET /context schema');
  context.record.json = contextValue;

  const headRoot = await request(origin.origin, 'root-head', 'HEAD', '/');
  requireProbe(headRoot.status === 200 && headRoot.body.length === 0 && headRoot.headers['content-length'] === String(page.body.length), 'HEAD / body and length');
  const headContext = await request(origin.origin, 'context-head', 'HEAD', '/context');
  requireProbe(headContext.status === 200 && headContext.body.length === 0 && headContext.headers['content-length'] === String(context.body.length), 'HEAD /context body and length');
  requireProbe((await request(origin.origin, 'wrong-host', 'GET', '/', { Host: 'evil.example' })).status === 403, 'wrong Host refusal');
  requireProbe((await request(origin.origin, 'cross-origin', 'GET', '/', { Origin: 'https://evil.example' })).status === 403, 'cross-origin refusal');
  requireProbe((await request(origin.origin, 'cross-site', 'GET', '/', { 'Sec-Fetch-Site': 'cross-site' })).status === 403, 'cross-site refusal');
  const post = await request(origin.origin, 'post', 'POST', '/');
  requireProbe(post.status === 405 && post.headers.allow === 'GET, HEAD', 'POST refusal');
  requireProbe((await request(origin.origin, 'unknown', 'GET', '/unknown')).status === 404, 'unknown path refusal');
  requireProbe((await request(origin.origin, 'query', 'GET', '/context?extra=1')).status === 404, 'query-bearing path refusal');
  receipt.hostProbesPassed = true;

  const browserEnvironment = {
    ...process.env,
    APPKIT_P12_TRIAL: trial,
    APPKIT_P12_EDITOR_TASKS: path.join(trial, 'editor-tasks.mjs'),
    APPKIT_P12_SCALE_TASKS: path.join(trial, 'scale-tasks.mjs'),
    APPKIT_P12_MORPH_TASKS: path.join(trial, 'morph-tasks.mjs'),
    APPKIT_P12_ZOOM_CONTRACT: path.join(trial, 'zoom-contract.mjs'),
    APPKIT_P12_ZOOM_CAPTURE: path.join(trial, 'zoom-capture.mjs'),
  };
  const startupBody = Buffer.from(`${JSON.stringify(startup, null, 2)}\n`);
  await write('harness-inputs.json', `${JSON.stringify({ schema: 1, files: receipt.snapshots, startupSHA256: sha(startupBody) }, null, 2)}\n`);
  const browser = await command('browser', process.execPath, [path.join(trial, 'browser.mjs'), origin.origin, build, ...(zoomArg ? [zoomArg] : [])], {
    cwd: trial,
    env: browserEnvironment,
    recordEnvironment: {
      APPKIT_P12_TRIAL: trial,
      APPKIT_P12_EDITOR_TASKS: path.join(trial, 'editor-tasks.mjs'),
      APPKIT_P12_SCALE_TASKS: path.join(trial, 'scale-tasks.mjs'),
      APPKIT_P12_MORPH_TASKS: path.join(trial, 'morph-tasks.mjs'),
      APPKIT_P12_ZOOM_CONTRACT: path.join(trial, 'zoom-contract.mjs'),
      APPKIT_P12_ZOOM_CAPTURE: path.join(trial, 'zoom-capture.mjs'),
    },
    timeoutMS: 240_000,
    outputLimit: 4 * 1024 * 1024,
  });
  receipt.browser = {
    code: browser.code,
    signal: browser.signal,
    timedOut: browser.timedOut,
    forced: browser.forced,
    milliseconds: browser.milliseconds,
    stdoutSHA256: browser.stdoutSHA256,
    stderrSHA256: browser.stderrSHA256,
  };
  if (browser.code !== 0 || browser.signal !== null || browser.timedOut || browser.forced || browser.spawnError) {
    throw new Error('Retained browser harness failed');
  }
} catch (error) {
  receipt.errors.push(errorText(error));
  process.exitCode = 1;
} finally {
  try { await stopOwnedBrowser(); } catch (error) {
    receipt.errors.push(`Browser cleanup failed: ${errorText(error)}`);
    process.exitCode = 1;
  }
  try { await stopOwnedHost(); } catch (error) {
    receipt.errors.push(`Host cleanup failed: ${errorText(error)}`);
    process.exitCode = 1;
  }
  hostStdout = Buffer.concat(hostStdoutChunks);
  hostStderr = Buffer.concat(hostStderrChunks);
  if (startup) {
    const stdoutText = hostStdout.toString('utf8');
    if (stdoutText.trim() !== startup.rawStartup || stdoutText !== `${startup.rawStartup}\n`) {
      receipt.errors.push('Owned host stdout was not exactly the retained startup JSON line');
      process.exitCode = 1;
    }
  }
  try {
    await write('host.stdout', hostStdout);
    await write('host.stderr', hostStderr);
  } catch (error) {
    receipt.errors.push(`Host log retention failed: ${errorText(error)}`);
    process.exitCode = 1;
  }
  receipt.finishedAt = new Date().toISOString();
  receipt.passed = receipt.errors.length === 0 && receipt.hostProbesPassed === true && receipt.browser?.code === 0;
  if (!receipt.passed) process.exitCode = 1;
  try {
    const body = Buffer.from(`${JSON.stringify(receipt, null, 2)}\n`);
    await write('run-receipt.json', body);
    process.stdout.write(`${JSON.stringify({ trial, receipt: path.join(trial, 'run-receipt.json'), passed: receipt.passed, receiptSHA256: sha(body), errors: receipt.errors.map(value => value.split('\n')[0]) })}\n`);
  } catch (error) {
    process.stderr.write(`P12 runner could not retain receipt: ${errorText(error)}\n`);
    process.exitCode = 1;
  }
}
