// Local, opt-in browser acceptance of a retained independently built host.
// No build, installation, default-browser selection or native window launch.
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import {createHash} from 'node:crypto';
import {spawn, spawnSync} from 'node:child_process';
import {setTimeout as delay} from 'node:timers/promises';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {validateLaunchURL} from './handoff.mjs';
import {launchManaged, closeContext, bounded} from './lifecycle.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SHA = /^[a-f0-9]{64}$/;
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const FILES = ['run.mjs', 'run_test.mjs', 'handoff.mjs', 'handoff_test.mjs', 'lifecycle.mjs', 'lifecycle_test.mjs'];
const READS = new Set(['/shell', '/shell/home', '/shell/profile', '/shell/settings', '/shell/activity', '/shell/help', '/shell/about', '/shell/events', '/static/app.css', '/static/shell-demo.js', '/static/shell-live-entry.js', '/static/shell-live.js', '/static/datastar-v1.0.3.js']);
const hash = body => createHash('sha256').update(body).digest('hex');
const exclusive = (file, body, mode = 0o600) => fs.writeFileSync(file, body, {flag: 'wx', mode});
const quote = value => `'${value.replaceAll("'", "'\\''")}'`;

export function parseArgs(args) {
  const options = {};
  for (let i = 0; i < args.length; i += 2) {
    const key = args[i];
    if (!['--receipt', '--receipt-sha256', '--playwright', '--out'].includes(key) || !args[i+1] || options[key]) throw new Error('arguments');
    options[key] = args[i+1];
  }
  if (Object.keys(options).length !== 4 || !SHA.test(options['--receipt-sha256'])) throw new Error('arguments');
  for (const key of ['--receipt', '--playwright', '--out']) if (!path.isAbsolute(options[key])) throw new Error('absolute-path-required');
  return options;
}

function regular(file, max) {
  const stat = fs.lstatSync(file);
  if (!stat.isFile() || stat.isSymbolicLink() || stat.size > max) throw new Error('unsafe-file');
  return fs.readFileSync(file);
}

export function validateReceipt(body, expected) {
  if (hash(body) !== expected) throw new Error('receipt-identity');
  const r = JSON.parse(body);
  if (r.kind !== 'app-kit-private-gallery-host-rehearsal-v1' || r.source_unchanged !== true || r.identical_server !== true || r.identical_native !== true || r.error || r.runs?.length !== 2) throw new Error('incomplete-host-receipt');
  if (!/^v0\.0\.0-rehearsal\.h[a-f0-9]{64}$/.test(r.candidate?.version)) throw new Error('candidate-identity');
  for (const [i, run] of r.runs.entries()) {
    if (run.error || run.execution?.name !== `host-run-${i+1}` || !SHA.test(run.execution.binary_sha256) || !SHA.test(run.native_sha256)) throw new Error('host-run-identity');
  }
  if (r.runs[0].execution.binary_sha256 !== r.runs[1].execution.binary_sha256) throw new Error('server-equality');
  const css = r.candidate.files?.find(file => file.path === 'apps/gallery/ui/app.css');
  if (!css || !SHA.test(css.sha256)) throw new Error('candidate-css-identity');
  return r;
}

export function allowedRequest(origin, method, raw, launch) {
  let url;
  try { url = new URL(raw); } catch { return false; }
  if (url.origin !== origin || url.username || url.password || url.hash) return false;
  if (method === 'GET' && raw === launch) return true;
  if (url.search) return false;
  return ((method === 'GET' || method === 'HEAD') && READS.has(url.pathname)) || (method === 'POST' && url.pathname === '/shell/action');
}

export function safeFailure(error) {
  const message = String(error?.message || '');
  if (error?.name === 'TimeoutError') return 'timeout';
  const network = message.match(/\bnet::(ERR_[A-Z_]+)\b/);
  if (network) return network[1].toLowerCase().replaceAll('_', '-');
  if (/Target page, context or browser has been closed/.test(message)) return 'browser-target-closed';
  if (/interrupted by another navigation/.test(message)) return 'navigation-interrupted';
  return /^[a-z0-9-]{1,80}$/.test(message) ? message : 'execution-failed';
}

function prepare(options) {
  const receiptPath = fs.realpathSync(options['--receipt']);
  const body = regular(receiptPath, 1 << 20);
  const receipt = validateReceipt(body, options['--receipt-sha256']);
  const source = path.dirname(receiptPath);
  const binary = regular(path.join(source, 'host-run-1/gallery-server'), 80 << 20);
  const other = regular(path.join(source, 'host-run-2/gallery-server'), 80 << 20);
  if (hash(binary) !== receipt.runs[0].execution.binary_sha256 || hash(other) !== hash(binary)) throw new Error('server-artifact-drift');
  const output = path.join(fs.realpathSync(path.dirname(options['--out'])), path.basename(options['--out']));
  for (const fleet of ['/Volumes/m2ext/code/geoffmilleraz', '/Users/geoff/code/geoffmilleraz']) if (output === fleet || output.startsWith(fleet+'/')) throw new Error('output-must-be-outside-fleet');
  if (fs.existsSync(output)) throw new Error('output-exists');
  const playwright = fs.realpathSync(options['--playwright']);
  regular(playwright, 1 << 20);
  regular(CHROME, 300 << 20);
  fs.mkdirSync(output, {mode: 0o700});
  fs.mkdirSync(path.join(output, 'harness'), {mode: 0o700});
  const files = [];
  for (const name of FILES) {
    const bytes = regular(path.join(HERE, name), 1 << 20);
    exclusive(path.join(output, 'harness', name), bytes);
    files.push({name, bytes: bytes.length, sha256: hash(bytes)});
  }
  exclusive(path.join(output, 'host-receipt.json'), body);
  exclusive(path.join(output, 'gallery-server'), binary, 0o700);
  exclusive(path.join(output, 'harness-manifest.json'), JSON.stringify(files, null, 2));
  return {output, receipt, receiptSHA256: hash(body), binarySHA256: hash(binary), playwright, files};
}

export function profile() {
  return `(version 1)
(allow default)
(deny network*)
(allow network-inbound (local ip "localhost:*"))
(allow network-outbound (remote ip "localhost:*"))
(deny file-read* (subpath "/Volumes/m2ext/code/geoffmilleraz"))
(deny file-read* (subpath "/Users/geoff/code/geoffmilleraz"))`;
}

async function until(test, milliseconds, code) {
  const end = Date.now() + milliseconds;
  while (Date.now() < end) { const value = await test(); if (value) return value; await delay(30); }
  throw new Error(code);
}

export function sandboxCanaries(root, command = spawnSync) {
  const policy = profile();
  const canaryOptions = {cwd: root, env: {PATH: '/usr/bin:/bin'}, encoding: 'utf8', timeout: 5000};
  const refusal = command('/usr/bin/sandbox-exec', ['-p', policy, '/usr/bin/head', '-c', '1', '/Volumes/m2ext/code/geoffmilleraz/app-kit/go.mod'], canaryOptions);
  if (refusal.status === 0 || !refusal.stderr?.toLowerCase().includes('operation not permitted')) throw new Error('source-denial-canary');
  const remote = command('/usr/bin/sandbox-exec', ['-p', policy, '/usr/bin/python3', '-c', "import socket\ns=socket.socket();s.settimeout(.2)\ntry:s.connect(('192.0.2.1',9))\nexcept PermissionError:pass\nelse:raise RuntimeError('network not denied')"], canaryOptions);
  if (remote.status !== 0) throw new Error('network-denial-canary');
}

async function startServer(root, binary) {
  const bin = path.join(root, 'bin');
  fs.mkdirSync(bin, {mode: 0o700});
  const handoff = path.join(root, 'launch.json');
  exclusive(path.join(bin, 'open'), `#!/bin/sh\nexec ${quote(process.execPath)} ${quote(path.join(HERE, 'handoff.mjs'))} "$@"\n`, 0o700);
  const policy = profile();
  sandboxCanaries(root);
  const child = spawn('/usr/bin/sandbox-exec', ['-p', policy, binary, '-browser'], {
    cwd: root, detached: true, stdio: ['ignore', 'pipe', 'pipe'],
    env: {PATH: `${bin}:/usr/bin:/bin`, TMPDIR: root, APPKIT_BROWSER_HANDOFF: handoff},
  });
  let log = '', overflow = false, spawnError = false;
  const collect = bytes => { if (!overflow) { log += bytes; if (log.length > 65536) { overflow = true; log = log.slice(0, 65536); } } };
  child.stdout.on('data', collect);
  child.stderr.on('data', collect);
  child.on('error', () => { spawnError = true; });
  const exited = new Promise(resolve => child.once('close', (code, signal) => resolve({code, signal})));
  const stop = async () => {
    if (child.exitCode === null && child.signalCode === null && child.pid) child.kill('SIGINT');
    const result = await Promise.race([exited, delay(7000).then(() => null)]);
    if (!result) { if (child.pid) process.kill(-child.pid, 'SIGKILL'); await bounded(exited, 5000, 'server-kill-timeout'); throw new Error('server-shutdown-timeout'); }
    if (fs.existsSync(handoff)) fs.unlinkSync(handoff);
    if (overflow || log.includes('token=')) throw new Error('unsafe-server-diagnostics');
    return {...result, diagnosticsSHA256: hash(log)};
  };
  try {
    await until(() => {
      if (spawnError || child.exitCode !== null || overflow) throw new Error('server-startup');
      return fs.existsSync(handoff);
    }, 12000, 'handoff-timeout');
    const info = fs.lstatSync(handoff);
    if (!info.isFile() || info.isSymbolicLink() || (info.mode & 0o777) !== 0o600) throw new Error('handoff-file-mode');
    const packet = JSON.parse(regular(handoff, 512));
    fs.unlinkSync(handoff); // Ephemeral one-use capability, never retained evidence.
    const validated = validateLaunchURL(packet.launchURL);
    if (packet.origin !== validated.origin || !log.includes(`Gallery at ${validated.origin} `)) throw new Error('handoff-origin');
    return {...validated, child, stop, state: () => ({code: child.exitCode, signal: child.signalCode, openerFailed: log.includes('browser launch failed; no launch credential was logged')}), policySHA256: hash(policy)};
  } catch (error) { await stop(); throw error; }
}

async function tabTo(page, selector) {
  await page.bringToFront();
  const target = page.locator(selector);
  for (let i = 0; i < 70; i++) {
    if (await target.evaluate(element => element === document.activeElement)) return;
    await page.keyboard.press('Tab');
  }
  throw new Error('keyboard-target-unreachable');
}

async function secondWindow(context, page) {
  // Separate visible windows preserve the observer's intentional hidden-tab pause.
  const cdp = await context.newCDPSession(page);
  try {
    const {targetInfo} = await cdp.send('Target.getTargetInfo');
    const [other] = await Promise.all([
      context.waitForEvent('page'),
      cdp.send('Target.createTarget', {url: 'about:blank', browserContextId: targetInfo.browserContextId, newWindow: true}),
    ]);
    return other;
  } finally { await cdp.detach(); }
}

async function typeName(page, name) {
  await tabTo(page, '#shell-display_name');
  await page.keyboard.press('Meta+A');
  await page.keyboard.type(name);
}

export async function submit(page, selector, expected, check, id, keyboard = true) {
  if (keyboard) await tabTo(page, selector);
  const [response] = await Promise.all([
    page.waitForResponse(r => r.request().method() === 'POST' && new URL(r.url()).pathname === '/shell/action'),
    // Arm before Enter: an already-loaded old document is not POST/redirect proof.
    page.waitForNavigation({waitUntil: 'load'}),
    keyboard ? page.keyboard.press('Enter') : page.locator(selector).click(),
  ]);
  check(response.status() === expected, id, {status: response.status()});
}

export async function run(config) {
  const {output, receipt, binarySHA256, receiptSHA256} = config;
  const report = {kind: 'app-kit-exact-server-browser-rehearsal-v1', receiptSHA256, binarySHA256,
    candidateVersion: receipt.candidate.version, harness: config.files, variants: [], captures: [], failures: [],
    host: {platform: os.platform(), release: os.release(), arch: os.arch()},
    limitations: ['Test-only PATH opener handoff; no LaunchServices/default-browser verification.', 'Synthetic in-memory Gallery account only; no production data, native host, full accessibility, physical-human input, IME, zoom or performance certification.', 'Playwright key events exercise named browser paths; fixture selection uses Playwright control APIs.', 'Browser request allowlist is application-traffic evidence, not a browser-process network sandbox.', 'Chrome may temporarily persist the one-use launch URL; private profile cleanup is required before success.', 'PNG dimensions and visible task regions are checked; visual quality requires separate image review.'],
  };
  let browser, managed;
  const originalTmp = process.env.TMPDIR;
  const temp = path.join(output, 'browser-temp'); fs.mkdirSync(temp, {mode: 0o700});
  process.env.TMPDIR = temp;
  try {
    const {chromium} = await import(pathToFileURL(config.playwright).href);
    managed = await launchManaged(chromium, {headless: false, executablePath: CHROME});
    browser = managed.browser;
    report.browser = browser.version(); report.node = process.version;
    for (const variant of [
      {name: 'desktop-light', width: 1440, theme: 'light', js: true},
      {name: 'narrow-dark', width: 390, theme: 'dark', js: true},
      {name: 'plain-desktop-light', width: 1440, theme: 'light', js: false},
      {name: 'plain-narrow-dark', width: 390, theme: 'dark', js: false},
    ]) {
      const result = {...variant, checks: [], blockedRequests: 0, pageErrors: 0, unexpectedConsole: 0, dialogs: 0, websockets: 0, cssResponses: 0};
      report.variants.push(result);
      const check = (ok, id, observed = {}) => { result.checks.push({id, ok, ...observed}); if (!ok) throw new Error(id); };
      const root = path.join(output, variant.name); fs.mkdirSync(root, {mode: 0o700});
      let server, context, stage = 'start-server';
      const pending = [];
      try {
        server = await startServer(root, path.join(output, 'gallery-server'));
        const origin = server.origin;
        result.policySHA256 = server.policySHA256;
        context = await browser.newContext({javaScriptEnabled: variant.js, viewport: {width: variant.width, height: 1000}, colorScheme: variant.theme, reducedMotion: 'reduce', serviceWorkers: 'block'});
        context.setDefaultTimeout(6000);
        await context.route('**/*', route => {
          const request = route.request();
          if (!allowedRequest(origin, request.method(), request.url(), server?.launchURL || '')) { result.blockedRequests++; return route.abort(); }
          return route.continue();
        });
        await context.routeWebSocket('**/*', socket => { result.websockets++; socket.close(); });
        context.on('page', page => {
          page.on('pageerror', () => result.pageErrors++);
          page.on('dialog', async dialog => { result.dialogs++; await dialog.dismiss(); });
          page.on('console', message => {
            if (message.type() !== 'error') return;
            const location = message.location().url;
            const known = location === origin+'/shell/action' && /^Failed to load resource: the server responded with a status of (422|409|503) /.test(message.text());
            if (!known) result.unexpectedConsole++;
          });
          page.on('response', response => {
            if (response.url() === origin+'/static/app.css') pending.push((async () => {
              const css = await response.body();
              check(hash(css) === receipt.candidate.files.find(file => file.path === 'apps/gallery/ui/app.css').sha256, 'exact-candidate-css'); result.cssResponses++;
            })().catch(() => { result.cssFailure = true; }));
          });
        });
        const page = await context.newPage();
        stage = 'authenticated-launch';
        const response = await page.goto(server.launchURL, {waitUntil: 'load'});
        check(response.status() === 200 && new URL(page.url()).pathname === '/shell/profile', 'launch-exchanges-and-redirects');
        check(await page.locator('#shell-display_name').inputValue() === 'Alex Morgan', 'fresh-synthetic-profile');
        const replay = await context.request.get(server.launchURL, {maxRedirects: 0});
        check(replay.status() === 403, 'launch-cannot-replay');
        server.launchURL = ''; // Consumed capability no longer needed or allowed.
        stage = 'keyboard-profile-save';
        await page.keyboard.press('Tab');
        check(await page.evaluate(() => document.activeElement?.getAttribute('href')) === '#shell-content', 'first-tab-skip-link');
        await page.keyboard.press('Enter');
        check(await page.evaluate(() => document.activeElement?.id) === 'shell-content', 'skip-link-focuses-main');
        await typeName(page, 'Independent Keyboard Avery');
        await submit(page, '[data-shell-save]', 303, check, 'keyboard-profile-save');
        check(await page.locator('#shell-display_name').inputValue() === 'Independent Keyboard Avery', 'save-survives-redirect');
        stage = 'validation';
        await typeName(page, 'X'.repeat(81));
        await submit(page, '[data-shell-save]', 422, check, 'native-post-validation');
        check(await page.locator('#shell-display_name').inputValue() === 'X'.repeat(81) && await page.locator('#shell-display_name').getAttribute('aria-invalid') === 'true', 'validation-preserves-and-identifies-draft');
        await typeName(page, 'Validated Independent Avery');
        await submit(page, '[data-shell-save]', 303, check, 'validation-recovery');
        const other = await secondWindow(context, page);
        await other.goto(server.origin+'/shell/profile');
        stage = 'two-tab-conflict';
        await typeName(page, 'Retained Independent Draft');
        await typeName(other, 'Newer Independent Save');
        await submit(other, '[data-shell-save]', 303, check, 'second-tab-save');
        await submit(page, '[data-shell-save]', 409, check, 'stale-write-refused');
        check(await page.locator('#shell-display_name').inputValue() === 'Retained Independent Draft' && (await page.locator('[aria-labelledby=shell-conflict-title]').innerText()).includes('Newer Independent Save'), 'conflict-shows-current-and-preserves-draft');
        check(await page.locator('[data-shell-save]').innerText() === 'Apply reviewed draft', 'explicit-review-action');
        await capture(page, `${variant.name}-conflict`, report, output);
        await submit(page, '[data-shell-save]', 303, check, 'reviewed-draft-save');
        stage = 'save-failure';
        await other.goto(server.origin+'/shell/profile');
        await other.getByText('Laboratory controls and evidence limits', {exact: true}).click();
        await other.locator('[name=fail_next_save]').check();
        await submit(other, 'button:has-text("Apply fixture")', 303, check, 'one-shot-failure-fixture', false);
        await page.goto(server.origin+'/shell/profile');
        await typeName(page, 'Independent Retry Avery');
        await submit(page, '[data-shell-save]', 503, check, 'save-failure-visible');
        check(await page.locator('#shell-display_name').inputValue() === 'Independent Retry Avery' && await page.locator('[data-shell-save]').isEnabled(), 'failed-draft-remains-retryable');
        await submit(page, '[data-shell-save]', 303, check, 'retry-succeeds');
        if (variant.js) {
          stage = 'live-observer';
          await other.goto(server.origin+'/shell/activity');
          await tabTo(other, '#shell-live-start'); await other.keyboard.press('Enter');
          await other.waitForFunction(() => document.querySelector('#shell-live')?.dataset.state === 'watching');
          check((await other.locator('#shell-live-summary').innerText()).includes('Independent Retry Avery'), 'live-initial-snapshot');
          await typeName(page, 'Independent Live Avery');
          await submit(page, '[data-shell-save]', 303, check, 'live-update-save');
          check(await other.evaluate(() => !document.hidden), 'observer-window-remains-visible');
          await other.waitForFunction(() => document.querySelector('#shell-live-summary')?.textContent.includes('Independent Live Avery'));
          check(true, 'live-cross-tab-update');
          await tabTo(other, '#shell-live-stop'); await other.keyboard.press('Enter');
          await other.waitForFunction(() => document.querySelector('#shell-live')?.dataset.state === 'paused');
          await tabTo(other, '#shell-live-start'); await other.keyboard.press('Enter');
          await other.waitForFunction(() => document.querySelector('#shell-live')?.dataset.state === 'watching');
          check((await other.locator('#shell-live-summary').innerText()).includes('Independent Live Avery'), 'manual-live-resnapshot');
          await capture(other, `${variant.name}-live`, report, output);
        }
        stage = 'layout-and-settings';
        await page.goto(server.origin+'/shell/settings');
        const geometry = await page.evaluate(() => ({scroll: document.documentElement.scrollWidth, client: document.documentElement.clientWidth, mains: document.querySelectorAll('main').length}));
        check(geometry.scroll <= geometry.client+1 && geometry.mains === 1, 'settings-fits-viewport', geometry);
        await page.locator('#shell-timezone').selectOption('UTC');
        await page.locator('#shell-start_page').selectOption('activity');
        await submit(page, '[data-shell-save]', 303, check, 'settings-native-save');
        check(await page.locator('#shell-timezone').inputValue() === 'UTC' && await page.locator('#shell-start_page').inputValue() === 'activity', 'settings-persist');
        await capture(page, `${variant.name}-settings`, report, output);
        stage = 'shutdown';
        result.shutdown = await server.stop(); server = null;
        check(result.shutdown.code === 0 && !result.shutdown.signal, 'foreground-sigint-exits-cleanly');
        if (variant.js) {
          await other.bringToFront();
          await other.waitForFunction(() => ['closed', 'interrupted'].includes(document.querySelector('#shell-live')?.dataset.state));
          check((await other.locator('#shell-live').innerText()).includes('may be stale') && await other.locator('#shell-live-start').isEnabled(), 'stopped-server-has-truthful-recovery');
          await capture(other, `${variant.name}-stopped`, report, output);
        }
        await Promise.all(pending);
        check(result.cssResponses > 0 && !result.cssFailure, 'candidate-css-was-loaded');
        check(result.blockedRequests === 0 && result.pageErrors === 0 && result.unexpectedConsole === 0 && result.dialogs === 0 && result.websockets === 0, 'browser-safety-observations-clean');
      } catch (error) {
        result.failure = {stage, code: safeFailure(error), browserConnected: browser.isConnected(), server: server?.state() || null};
        report.failures.push({variant: variant.name, ...result.failure});
        if (context) {
          const pages = context.pages();
          if (pages.length) await capture(pages[0], `${variant.name}-failure`, report, output).catch(() => {});
        }
      } finally {
        if (context) await closeContext(context).catch(() => report.failures.push({variant: variant.name, code: 'context-close'}));
        if (server) await server.stop().catch(() => report.failures.push({variant: variant.name, code: 'server-cleanup'}));
        result.handoffClean = fs.readdirSync(root).every(name => name !== 'launch.json' && !name.startsWith('.launch.json.staging-'));
        if (!result.handoffClean) report.failures.push({variant: variant.name, code: 'handoff-cleanup'});
        await bounded(Promise.all(pending), 6000, 'css-settle-timeout').catch(() => report.failures.push({variant: variant.name, code: 'css-settle'}));
        result.safetyAfterContextClose = result.blockedRequests === 0 && result.pageErrors === 0 && result.unexpectedConsole === 0 && result.dialogs === 0 && result.websockets === 0 && !result.cssFailure;
        if (!result.safetyAfterContextClose) report.failures.push({variant: variant.name, code: 'final-browser-observations'});
      }
      if (report.failures.length) break;
    }
  } catch {
    report.failures.push({code: 'browser-start-or-run'});
  } finally {
    report.browserCleanup = 'not-confirmed';
    if (managed) await managed.close().then(() => { report.browserCleanup = 'confirmed'; }).catch(() => report.failures.push({code: 'browser-close'}));
    try { report.browserTempEmpty = fs.readdirSync(temp).length === 0; } catch { report.browserTempEmpty = false; }
    if (!report.browserTempEmpty || report.browserCleanup !== 'confirmed') report.failures.push({code: 'browser-profile-cleanup'});
    if (originalTmp === undefined) delete process.env.TMPDIR; else process.env.TMPDIR = originalTmp;
    try {
      report.binaryUnchanged = hash(regular(path.join(output, 'gallery-server'), 80 << 20)) === binarySHA256;
      report.harnessUnchanged = config.files.every(file => hash(regular(path.join(output, 'harness', file.name), 1 << 20)) === file.sha256);
    } catch { report.failures.push({code: 'final-artifact-read'}); }
    report.ok = report.failures.length === 0 && report.variants.length === 4 && report.binaryUnchanged && report.harnessUnchanged;
    exclusive(path.join(output, 'browser-receipt.json'), JSON.stringify(report, null, 2));
  }
  console.log(JSON.stringify({ok: report.ok, variants: report.variants.length, checks: report.variants.reduce((n, v) => n+v.checks.length, 0), failures: report.failures, report: path.join(output, 'browser-receipt.json')}));
  return report.ok ? 0 : 1;
}

async function capture(page, name, report, output) {
  await page.bringToFront();
  const region = await page.locator('main').boundingBox();
  if (!region || region.width < 100 || region.height < 100 || !await page.locator('main').isVisible()) throw new Error('capture-task-region');
  const png = await page.screenshot({fullPage: true});
  if (png.length > 5 << 20) throw new Error('capture-bound');
  const dimensions = pngDimensions(png);
  if (dimensions.width !== page.viewportSize().width || dimensions.height < page.viewportSize().height) throw new Error('capture-dimensions');
  exclusive(path.join(output, name+'.png'), png);
  report.captures.push({file: name+'.png', sha256: hash(png), viewport: page.viewportSize(), dimensions, main: region, kind: 'full-document', visualReview: 'pending'});
  const selector = name.endsWith('-conflict') ? '#shell-display_name' : name.endsWith('-settings') ? '[data-shell-save]' : name.endsWith('-live') || name.endsWith('-stopped') ? '#shell-live-start' : null;
  if (!selector) return;
  const target = page.locator(selector);
  // Capture setup, not a claimed keyboard action. Keep fixed app chrome intact.
  await target.evaluate(element => element.scrollIntoView({block: 'center', behavior: 'instant'}));
  // DOM geometry is synchronous; screenshot owns paint synchronization. Do not
  // require a page animation callback in the deliberately script-disabled lane.
  const visible = await target.evaluate(element => {
    const r = element.getBoundingClientRect();
    const hit = document.elementFromPoint(r.x+r.width/2, r.y+r.height/2);
    return {unobscured: !!hit && (hit === element || element.contains(hit)) && r.top >= 0 && r.bottom <= innerHeight, top:r.top, bottom:r.bottom, scrollY};
  });
  if (!visible.unobscured) throw new Error('capture-task-obscured');
  const viewportPNG = await page.screenshot({fullPage: false});
  const size = pngDimensions(viewportPNG), viewport = page.viewportSize();
  if (size.width !== viewport.width || size.height !== viewport.height || viewportPNG.length > 5 << 20) throw new Error('capture-viewport-dimensions');
  exclusive(path.join(output, name+'-viewport.png'), viewportPNG);
  report.captures.push({file: name+'-viewport.png', sha256:hash(viewportPNG), viewport, dimensions:size, kind:'task-viewport', selector, ...visible, visualReview:'pending'});
}

export function pngDimensions(png) {
  if (png.length < 33 || !png.subarray(0, 8).equals(Buffer.from([137,80,78,71,13,10,26,10])) || png.toString('ascii', 12, 16) !== 'IHDR' || png.readUInt32BE(8) !== 13) throw new Error('capture-png');
  const width = png.readUInt32BE(16), height = png.readUInt32BE(20);
  if (width < 1 || height < 1 || width > 10000 || height > 20000) throw new Error('capture-png');
  return {width, height};
}

if (process.argv[1] && fs.realpathSync(process.argv[1]) === fs.realpathSync(fileURLToPath(import.meta.url))) {
  try {
    const config = prepare(parseArgs(process.argv.slice(2)));
    // Execute the retained source/relative-import closure, not the mutable checkout.
    const frozen = await import(pathToFileURL(path.join(config.output, 'harness/run.mjs')).href);
    process.exitCode = await frozen.run(config);
  } catch { console.error('browser rehearsal refused; inspect inputs or retained receipt'); process.exitCode = 1; }
}
