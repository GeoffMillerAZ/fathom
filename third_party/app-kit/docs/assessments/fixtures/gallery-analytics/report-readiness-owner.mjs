// Runs a copied Gallery and the bounded synthetic report-readiness browser probe.
// Args: copied binary, compiled CSS, frozen source archive, Playwright entry,
// Chrome executable, new output directory. No existing listener or app is used.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { trackOwnedChild, terminateOwned } from '../../../../apps/styleguide/browser/owned_process.mjs';

const [binary, css, archive, pw, chrome, output] = process.argv.slice(2);
for (const value of [binary, css, archive, pw, chrome, output]) assert.ok(path.isAbsolute(value), `absolute path: ${value}`);
const here = path.dirname(fileURLToPath(import.meta.url));
const driver = path.join(here, 'report-readiness.mjs');
const inputs = { binary, css, archive, pw, chrome, driver,
  owner: fileURLToPath(import.meta.url), node: process.execPath,
  owned_process: path.resolve(here, '../../../../apps/styleguide/browser/owned_process.mjs'),
  phase_wait: path.resolve(here, '../../../../apps/styleguide/browser/phase_wait.mjs'),
  contract: path.resolve(here, '../../2026-09-22-analytics-report-readiness.md') };
const expected = { binary: '807127451ca088a3334d3b2dc04d66cb52758675cd6bea69b67d6344bcbe6d20',
  css: '94589496d9aa1e0e72c629b316df2c2629104af3dec2dda27e5a73726179a01b',
  archive: 'ad15e73d51ebb264368861c0524936f61be959e60e649b5d5cb3b045fbb6d8b5' };
const report = { schema: 'appkit-gallery-report-readiness-owner-v1', expected_sha256: expected,
  inputs, observation_complete: false, task_passed: false, budgets_passed: false, passed: false,
  errors: [], process: {}, final_port_refused: false };
let gallery, probe, origin, madeOutput = false;

async function hash(file) {
  const value = crypto.createHash('sha256');
  for await (const chunk of createReadStream(file)) value.update(chunk);
  return value.digest('hex');
}
async function allHashes() {
  return Object.fromEntries(await Promise.all(Object.entries(inputs).map(async ([key, file]) => [key, await hash(file)])));
}
async function packageManifest(entry) {
  let dir = path.dirname(entry);
  for (let n = 0; n < 8; n++) {
    const file = path.join(dir, 'package.json');
    try { const value = JSON.parse(await fs.readFile(file, 'utf8'));
      if (['playwright', 'playwright-core'].includes(value.name)) return { file, name: value.name, version: value.version }; }
    catch (error) { if (error.code !== 'ENOENT') throw error; }
    const parent = path.dirname(dir); if (parent === dir) break; dir = parent;
  }
  throw Error('Playwright package manifest unavailable for exact entry');
}
function capture(owned, name) {
  const data = { stdout: '', stderr: '', truncated: false };
  for (const stream of ['stdout', 'stderr']) owned.child[stream]?.on('data', chunk => {
    const next = data[stream] + chunk.toString();
    if (Buffer.byteLength(next) > 1024 * 1024) data.truncated = true;
    else data[stream] = next;
  });
  report.process[name] = { lifecycle: owned.state, capture: data };
  return data;
}
function waitClose(owned, ms) {
  if (owned.state.close) return Promise.resolve(true);
  return new Promise(resolve => {
    let timer;
    const close = () => done(true);
    const done = value => { clearTimeout(timer); owned.child.off('close', close); resolve(value); };
    owned.child.once('close', close);
    timer = setTimeout(() => done(false), ms);
  });
}
async function waitReady(owned, captureState, deadline) {
  while (Date.now() < deadline) {
    if (owned.state.exit || owned.state.error) throw Error('Gallery exited before printed readiness');
    const match = captureState.stderr.match(/app-kit Gallery → (http:\/\/127\.0\.0\.1:([1-9]\d*))/);
    if (match) {
      const selected = match[1];
      if (Number(match[2]) > 65535) throw Error('printed port invalid');
      try { const response = await fetch(selected + '/analytics', { signal: AbortSignal.timeout(1500), redirect: 'error' });
        if (response.status === 200) { await response.arrayBuffer(); return selected; } }
      catch { /* exact Gallery listener may not yet accept requests */ }
    }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw Error('printed loopback origin and HTTP 200 not observed');
}

try {
  await fs.mkdir(output); madeOutput = true;
  report.node_version = process.version;
  const manifest = await packageManifest(pw);
  inputs.playwright_manifest = manifest.file;
  report.playwright_package = { name: manifest.name, version: manifest.version };
  report.sha256_before = await allHashes();
  for (const [key, value] of Object.entries(expected)) assert.equal(report.sha256_before[key], value, `${key} frozen identity`);
  gallery = trackOwnedChild(spawn(binary, ['-addr', '127.0.0.1:0', '-design-assets', ''],
    { cwd: path.dirname(binary), stdio: ['ignore', 'pipe', 'pipe'] }));
  const galleryCapture = capture(gallery, 'gallery');
  origin = await waitReady(gallery, galleryCapture, Date.now() + 20000);
  report.origin = origin; report.ready_line_and_http_200 = true;
  probe = trackOwnedChild(spawn(process.execPath, [driver, pw, chrome, origin, css,
    path.join(output, 'browser')], { cwd: here, stdio: ['ignore', 'pipe', 'pipe'] }));
  const probeCapture = capture(probe, 'probe');
  report.probe_closed_within_120s = await waitClose(probe, 120000);
  report.process.probe.termination = await terminateOwned(probe, { graceMs: 2000, killMs: 2000, drainMs: 2000 });
  try {
    report.browser_report = JSON.parse(await fs.readFile(path.join(output, 'browser', 'report.json'), 'utf8'));
    report.observation_complete = report.browser_report.observation_complete === true;
    report.task_passed = report.browser_report.task_passed === true;
    report.budgets_passed = report.browser_report.budgets_passed === true;
  } catch (error) { report.errors.push(`browser report unavailable: ${error}`); }
  assert.ok(report.probe_closed_within_120s, 'browser probe workload cap');
  assert.ok(report.process.probe.termination.ok && !probeCapture.truncated, 'probe lifecycle/capture');
  assert.equal(probe.state.exit?.code, 0, 'browser probe nonzero');
  assert.equal(report.browser_report?.passed, true, 'nested browser report did not pass');
  assert.equal(report.browser_report?.browser_process_exit_verified, true, 'fresh Chrome exit unverified');
  assert.ok(report.observation_complete && report.task_passed && report.budgets_passed,
    'readiness workload did not fully pass');
} catch (error) { report.errors.push(String(error.stack || error)); }
finally {
  if (probe && !probe.state.termination) {
    try { report.process.probe.termination = await terminateOwned(probe, { graceMs: 2000, killMs: 2000, drainMs: 2000 }); }
    catch (error) { report.errors.push(`probe cleanup: ${error}`); }
  }
  if (gallery) {
    try {
      report.process.gallery.termination = await terminateOwned(gallery, { graceMs: 6000, killMs: 2000, drainMs: 2000 });
      const result = report.process.gallery.termination;
      if (!result.ok || gallery.state.exit?.code !== 0 || report.process.gallery.capture.truncated ||
          !gallery.state.signals.some(signal => signal.signal === 'SIGTERM' && signal.returned))
        report.errors.push('Gallery graceful owned shutdown/drain failed');
    } catch (error) { report.errors.push(`Gallery cleanup: ${error}`); }
  }
  if (origin) {
    try { const response = await fetch(origin + '/analytics', { signal: AbortSignal.timeout(1000), redirect: 'error' });
      report.errors.push(`owned listener still responds ${response.status}`); }
    catch (error) { report.final_port_refused = error.cause?.code === 'ECONNREFUSED';
      if (!report.final_port_refused) report.errors.push(`final port not demonstrably refused: ${error}`); }
  }
  try { report.sha256_after = await allHashes();
    if (Object.keys(inputs).some(key => report.sha256_after[key] !== report.sha256_before?.[key]))
      report.errors.push('frozen input identity changed'); }
  catch (error) { report.errors.push(`input reread: ${error}`); }
  report.passed = report.observation_complete && report.task_passed && report.budgets_passed &&
    report.browser_report?.passed === true && report.browser_report?.browser_process_exit_verified === true &&
    gallery?.state.exit?.code === 0 && report.final_port_refused && report.errors.length === 0;
  if (madeOutput) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({ passed: report.passed, observation_complete: report.observation_complete,
    task_passed: report.task_passed, budgets_passed: report.budgets_passed, origin,
    errors: report.errors, output }, null, 2));
  if (!report.passed) process.exitCode = 1;
}
