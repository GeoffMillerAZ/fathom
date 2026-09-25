// Private synthetic assessment. No active consumer, user data or installed profile.
// node owner.mjs ABS_CONTRACT_JSON NEW_OUTPUT_DIRECTORY
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import { spawn } from 'node:child_process';
import path from 'node:path';
import { pathToFileURL, fileURLToPath } from 'node:url';
import { trackOwnedChild, terminateOwned } from '../fathom-guide-only-2026-09-22/browser/owned_process.mjs';

const [contractFile, output] = process.argv.slice(2);
assert.equal(process.argv.length, 4);
assert.ok(path.isAbsolute(contractFile) && path.isAbsolute(output));
const contractBytes = await fs.readFile(contractFile);
const contract = JSON.parse(contractBytes.toString('utf8'));
assert.equal(contract.schema_version, 1);
assert.deepEqual(Object.keys(contract.inputs).sort(),
  ['app_css', 'binary', 'chrome', 'cue', 'driver', 'fixture', 'fixture_policy', 'helper', 'node', 'owner', 'playwright', 'playwright_manifest', 'theme_css', 'zoom_helper']);
const inputs = contract.inputs;
for (const [key, value] of Object.entries(inputs)) {
  assert.ok(path.isAbsolute(value.path), key);
  assert.match(value.sha256, /^[a-f0-9]{64}$/, key);
}
assert.equal(inputs.node.path, process.execPath);
assert.equal(inputs.owner.path, fileURLToPath(import.meta.url));
assert.equal(inputs.driver.path, fileURLToPath(new URL('./browser.mjs', import.meta.url)));
assert.equal(inputs.zoom_helper.path, fileURLToPath(new URL('./zoom-contract.mjs', import.meta.url)));
assert.equal(inputs.helper.path, fileURLToPath(new URL('../fathom-guide-only-2026-09-22/browser/owned_process.mjs', import.meta.url)));
const report = { schema_version: 1, scope: contract.scope, contract, started_at: new Date().toISOString(),
  contract_sha256: crypto.createHash('sha256').update(contractBytes).digest('hex'),
  passed: false, checks: [], cells: [], errors: [], processes: {}, timeout_ms: 180000,
  limits: ['Synthetic private fixture, not active consumer delivery.',
    'Native automated browser input, not physical or screen-reader acceptance.',
    'Browser route restrictions do not prove OS-wide network containment.',
    'No full accessibility, usability, memory or performance certification.'] };
let server, chrome, browser, madeOutput = false, origin, timer, timedOut = false;
const captures = new Map();
async function digest(file) {
  const hash = crypto.createHash('sha256');
  for await (const chunk of createReadStream(file)) hash.update(chunk);
  return hash.digest('hex');
}
async function hashInputs() {
  return Object.fromEntries(await Promise.all(Object.entries(inputs).map(async ([key, value]) => [key, await digest(value.path)])));
}
function owned(command, args, label, options = {}) {
  const process = trackOwnedChild(spawn(command, args, { stdio: ['ignore', 'pipe', 'pipe'], ...options }));
  const capture = { stdout: '', stderr: '', truncated: false };
  for (const name of ['stdout', 'stderr']) process.child[name].on('data', bytes => {
    if (Buffer.byteLength(capture[name]) + bytes.length > 1024 * 1024) capture.truncated = true;
    else capture[name] += bytes.toString();
  });
  captures.set(process, capture);
  report.processes[label] = { state: process.state, capture };
  return process;
}
async function waitForLine(process, expression, ms = 15000) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline && !timedOut) {
    const capture = captures.get(process);
    assert.ok(!capture.truncated, 'process output truncated');
    const match = (capture.stdout + '\n' + capture.stderr).match(expression);
    if (match) return match[1];
    assert.ok(!process.state.exit && !process.state.error, 'process exited before readiness');
    await new Promise(resolve => setTimeout(resolve, 50));
  }
  throw Error('owned process readiness deadline');
}
async function bounded(work, ms) {
  let alarm;
  try { return await Promise.race([work(), new Promise((_, reject) => { alarm = setTimeout(() => reject(Error('bounded operation timeout')), ms); })]); }
  finally { clearTimeout(alarm); }
}
async function run() {
  await fs.mkdir(output, { mode: 0o700 }); madeOutput = true;
  report.input_sha256_before = await hashInputs();
  for (const [key, value] of Object.entries(inputs)) assert.equal(report.input_sha256_before[key], value.sha256, `${key} identity`);
  // Copy the complete executed harness/contract into this exclusive run before use.
  await fs.mkdir(path.join(output, 'executed'), { mode: 0o700 });
  for (const key of ['fixture', 'fixture_policy', 'driver', 'owner', 'helper', 'zoom_helper'])
    await fs.copyFile(inputs[key].path, path.join(output, 'executed', key + path.extname(inputs[key].path)), 1);
  await fs.writeFile(path.join(output, 'executed', 'contract.json'), contractBytes, { flag: 'wx' });
  report.node_version = process.version;
  report.playwright_version = JSON.parse(await fs.readFile(inputs.playwright_manifest.path, 'utf8')).version;
  server = owned('/usr/bin/sandbox-exec', ['-f', inputs.fixture_policy.path, inputs.binary.path,
    '-test.run=^TestAppKitLargeChartFixture$', '-test.v', '-test.timeout=180s'], 'server',
    { cwd: path.dirname(inputs.binary.path), env: { ...process.env, APPKIT_LARGE_CHART_FIXTURE: '1' } });
  origin = await waitForLine(server, /APPKIT_LARGE_CHART_ORIGIN=(http:\/\/127\.0\.0\.1:\d+)/);
  report.origin = origin;
  const ready = await fetch(origin + '/dev/theme', { signal: AbortSignal.timeout(10000), redirect: 'error' });
  assert.equal(ready.status, 200); await ready.arrayBuffer();
  const profile = path.join(output, 'chrome-profile');
  await fs.mkdir(profile, { mode: 0o700 });
  chrome = owned(inputs.chrome.path, ['--headless=new', '--remote-debugging-address=127.0.0.1', '--remote-debugging-port=0',
    '--user-data-dir=' + profile, '--disk-cache-dir=' + path.join(profile, 'cache'), '--window-size=1440,1000',
    '--disable-background-networking', '--no-proxy-server', '--no-first-run', '--no-default-browser-check', 'about:blank'], 'chrome');
  const endpoint = await waitForLine(chrome, /DevTools listening on (ws:\/\/127\.0\.0\.1:\d+\/devtools\/browser\/[^\s]+)/);
  const { chromium } = await import(pathToFileURL(inputs.playwright.path).href);
  browser = await chromium.connectOverCDP(endpoint, { timeout: 10000 });
  report.browser_version = browser.version();
  assert.equal(browser.contexts().length, 1);
  const { runProbe } = await import(pathToFileURL(inputs.driver.path).href);
  await runProbe({ chromium, browser, context: browser.contexts()[0], origin, output, report });
  report.observation_complete = true;
}
try {
  await Promise.race([run(), new Promise((_, reject) => { timer = setTimeout(() => {
    timedOut = true; reject(Error('180-second acquisition deadline'));
  }, 180000); })]);
} catch (error) { report.errors.push(String(error.stack || error)); }
finally {
  clearTimeout(timer);
  if (browser) {
    try { await bounded(() => browser.close(), 5000); }
    catch (error) { report.errors.push(`browser close: ${error}`); }
  }
  for (const [label, process] of [['chrome', chrome], ['server', server]]) if (process) {
    try {
      const termination = await terminateOwned(process, { graceMs: 2000, killMs: 2000, drainMs: 2000 });
      report.processes[label].termination = termination;
      if (!termination.ok || termination.forced || captures.get(process).truncated) report.errors.push(label + ' cleanup/capture incomplete');
      if (label === 'server' && (process.state.exit?.code !== 0 ||
          !captures.get(process).stdout.includes('--- PASS: TestAppKitLargeChartFixture')))
        report.errors.push('fixture did not report a passing graceful shutdown');
    } catch (error) { report.errors.push(`${label} cleanup: ${error}`); }
  }
  if (origin) {
    try { await fetch(origin + '/dev/theme', { signal: AbortSignal.timeout(1500), redirect: 'error' }); report.errors.push('fixture port still answers'); }
    catch (error) { report.port_refused = error.cause?.code === 'ECONNREFUSED'; if (!report.port_refused) report.errors.push(`port closure unknown: ${error}`); }
  }
  try {
    assert.equal(await digest(contractFile), report.contract_sha256, 'prospective contract changed');
    report.input_sha256_after = await hashInputs();
    for (const [key, value] of Object.entries(inputs)) assert.equal(report.input_sha256_after[key], value.sha256, key + ' changed');
  } catch (error) { report.errors.push(String(error)); }
  report.passed = report.observation_complete === true && report.port_refused === true && report.errors.length === 0 && !timedOut;
  report.finished_at = new Date().toISOString();
  if (madeOutput) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n', { flag: 'wx' });
  console.log(JSON.stringify({ passed: report.passed, checks: report.checks.length, cells: report.cells.length, errors: report.errors, output }));
  if (!report.passed) process.exitCode = 1;
}
