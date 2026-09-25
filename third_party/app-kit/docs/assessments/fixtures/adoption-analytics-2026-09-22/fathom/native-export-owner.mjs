// Runs only a copied, hash-pinned frozen candidate. Args: copied binary, copied
// baseline CUE, Playwright module, Chrome executable, new output directory.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import net from 'node:net';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { trackOwnedChild, terminateOwned } from '../../../../../apps/styleguide/browser/owned_process.mjs';

const [binary, cue, pw, chrome, output] = process.argv.slice(2);
const here = path.dirname(fileURLToPath(import.meta.url));
const probe = path.join(here, 'native-export-events.mjs');
const helper = path.resolve(here, '../../../../../apps/styleguide/browser/owned_process.mjs');
for (const item of [binary, cue, pw, chrome, output]) assert.ok(path.isAbsolute(item), `absolute path required: ${item}`);
const inputs = { binary, cue, pw, chrome, probe, owner: fileURLToPath(import.meta.url), helper,
  node: process.execPath,
  contract: path.resolve(here, '../../../2026-09-22-fathom-native-export.md') };
const expected = {
  binary: '3bd8187a5c4181eb86d33e8151b17cb7e669faed2a7defaa16c364aa1a00499a',
  cue: '443461ce94d565b18ea6917631f45fed689a8248839533ec15b2dd328c5267bf',
};
const report = { scope: 'copied frozen Fathom candidate; no active consumer integration',
  observation_complete: false, native_export_checks_passed: false,
  input_paths: inputs, expected_sha256: expected, errors: [], process: {},
  timeout_ms: 120000, browser_process_exit_verified: false };
let server, worker, created = false, origin;
let deadline;
const maxCapture = 1024 * 1024;

async function hashFile(file) {
  const hash = crypto.createHash('sha256');
  for await (const chunk of createReadStream(file)) hash.update(chunk);
  return hash.digest('hex');
}
async function hashes() {
  return Object.fromEntries(await Promise.all(Object.entries(inputs).map(async ([key, file]) => [key, await hashFile(file)])));
}
async function playwrightManifest(entry) {
  let dir = path.dirname(entry);
  for (let n = 0; n < 8; n++) {
    const file = path.join(dir, 'package.json');
    try {
      const value = JSON.parse(await fs.readFile(file, 'utf8'));
      if (value.name === 'playwright' || value.name === 'playwright-core')
        return { file, name: value.name, version: value.version };
    } catch (error) { if (error.code !== 'ENOENT') throw error; }
    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  throw Error('Playwright package manifest not found from exact module entry');
}
function capture(owned, label) {
  const state = { stdout: '', stderr: '', truncated: false };
  for (const name of ['stdout', 'stderr']) owned.child[name]?.on('data', chunk => {
    const next = state[name] + chunk.toString();
    if (Buffer.byteLength(next) > maxCapture) state.truncated = true;
    else state[name] = next;
  });
  report.process[label] = { owned: owned.state, capture: state };
  return state;
}
function awaitClose(owned, timeoutMs) {
  if (owned.state.close) return Promise.resolve(true);
  return new Promise(resolve => {
    let timer;
    const done = value => { clearTimeout(timer); owned.child.off('close', close); resolve(value); };
    const close = () => done(true);
    owned.child.once('close', close);
    timer = setTimeout(() => done(false), timeoutMs);
  });
}
async function reservePort() {
  const listener = net.createServer();
  await new Promise((resolve, reject) => listener.once('error', reject).listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise((resolve, reject) => listener.close(error => error ? reject(error) : resolve()));
  assert.ok(Number.isSafeInteger(port) && port > 0);
  return port;
}
async function ready(captureState, owned, wanted, deadline) {
  while (Date.now() < deadline) {
    if (owned.state.exit || owned.state.error) throw Error('candidate exited before readiness');
    if (captureState.stdout.includes(`Opt-in synthetic Observatory theme guide: ${wanted}/dev/theme`)) {
      try {
        const response = await fetch(wanted + '/dev/theme', { signal: AbortSignal.timeout(1500), redirect: 'error' });
        if (response.status === 200) { await response.arrayBuffer(); return; }
      } catch { /* exact owned listener is still starting */ }
    }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw Error('ready line plus HTTP 200 not observed before deadline');
}

try {
  await fs.mkdir(output); created = true;
  report.node_version = process.version;
  const packageIdentity = await playwrightManifest(pw);
  inputs.playwright_manifest = packageIdentity.file;
  report.playwright_package = { name: packageIdentity.name, version: packageIdentity.version };
  report.input_sha256_before = await hashes();
  assert.equal(report.input_sha256_before.binary, expected.binary, 'frozen binary hash');
  assert.equal(report.input_sha256_before.cue, expected.cue, 'frozen source CUE hash');
  const port = await reservePort();
  origin = `http://127.0.0.1:${port}`; report.origin = origin;
  deadline = Date.now() + 120000;
  server = trackOwnedChild(spawn(binary, ['theme-guide', '-addr', `127.0.0.1:${port}`],
    { cwd: path.dirname(binary), stdio: ['ignore', 'pipe', 'pipe'] }));
  const serverCapture = capture(server, 'server');
  await ready(serverCapture, server, origin, Math.min(Date.now() + 20000, deadline - 15000));
  report.ready_line_seen = true; report.http_200_seen = true;
  const childOutput = path.join(output, 'native-probe');
  worker = trackOwnedChild(spawn(process.execPath, [probe, pw, chrome, origin, cue, childOutput],
    { cwd: here, stdio: ['ignore', 'pipe', 'pipe'] }));
  const workerCapture = capture(worker, 'probe');
  report.probe_closed_before_deadline = await awaitClose(worker, Math.max(1, deadline - Date.now() - 15000));
  report.process.probe.termination = await terminateOwned(worker,
    { graceMs: 2000, killMs: 2000, drainMs: 2000 });
  try {
    report.probe_report = JSON.parse(await fs.readFile(path.join(childOutput, 'report.json'), 'utf8'));
    report.observation_complete = report.probe_report.observation_complete === true;
    report.native_export_checks_passed = report.probe_report.native_export_checks_passed === true;
    report.browser_process_exit_verified = report.probe_report.browser_process_exit_verified === true;
  } catch (error) { report.errors.push(`probe report unavailable: ${error}`); }
  assert.ok(report.probe_closed_before_deadline, 'probe exceeded bounded workload');
  assert.ok(report.process.probe.termination.ok && !workerCapture.truncated, 'probe lifecycle/capture');
  assert.equal(worker.state.exit?.code, 0, 'native probe nonzero');
  assert.ok(report.observation_complete, 'native observation incomplete');
  assert.ok(report.native_export_checks_passed, 'native pointer and Enter export did not both pass');
  assert.ok(report.browser_process_exit_verified, 'fresh Chrome process exit not verified');
} catch (error) { report.errors.push(String(error.stack || error)); }
finally {
  if (worker && !worker.state.termination) {
    try { report.process.probe.termination = await terminateOwned(worker,
      { graceMs: 2000, killMs: 2000, drainMs: 2000 }); }
    catch (error) { report.errors.push(`probe shutdown: ${error}`); }
  }
  if (server) {
    try {
      report.process.server.termination = await terminateOwned(server,
        { graceMs: 2000, killMs: 2000, drainMs: 2000 });
      const result = report.process.server.termination;
      if (!result.ok || result.forced || report.process.server.capture.truncated ||
          !server.state.signals.some(signal => signal.signal === 'SIGTERM' && signal.returned))
        report.errors.push('server SIGTERM/drain lifecycle incomplete');
    } catch (error) { report.errors.push(`server shutdown: ${error}`); }
  }
  if (origin) {
    try {
      const response = await fetch(origin + '/dev/theme', { signal: AbortSignal.timeout(1000), redirect: 'error' });
      report.errors.push(`final port still answered HTTP ${response.status}`);
    } catch (error) {
      report.final_port_refused = error.cause?.code === 'ECONNREFUSED';
      if (!report.final_port_refused) report.errors.push(`final port state inconclusive: ${error}`);
    }
  }
  try {
    report.input_sha256_after = await hashes();
    if (Object.keys(inputs).some(key => report.input_sha256_before?.[key] !== report.input_sha256_after[key]))
      report.errors.push('input identity changed during probe');
  } catch (error) { report.errors.push(`input reread: ${error}`); }
  report.passed = report.observation_complete && report.native_export_checks_passed &&
    report.browser_process_exit_verified && report.final_port_refused === true && report.errors.length === 0;
  if (created) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({ passed: report.passed, observation_complete: report.observation_complete,
    native_export_checks_passed: report.native_export_checks_passed, origin,
    errors: report.errors, output }, null, 2));
  if (!report.passed) process.exitCode = 1;
}
