// Actual browser zoom, not CSS scaling. Reuses hash-bound private browser inputs;
// no asset fetching, dependency installation, source edits or signed-in profile.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import * as fs from 'node:fs/promises';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';
import { zoomCaptureClip } from '../gallery-analytics/zoom-contract.mjs';

const [probeArg, receiptArg, receiptSHA] = process.argv.slice(2);
assert(process.argv.length === 5 && path.isAbsolute(probeArg) && path.isAbsolute(receiptArg) && /^[a-f0-9]{64}$/.test(receiptSHA),
  'usage: node zoom.mjs PRIVATE_PROBE BROWSER_RECEIPT EXPECTED_SHA256');
const tempRoot = await fs.realpath(os.tmpdir());
const probe = await fs.realpath(probeArg), priorPath = await fs.realpath(receiptArg);
assert(probe.startsWith(tempRoot + path.sep) && priorPath.startsWith(probe + path.sep));
const sha = b => createHash('sha256').update(b).digest('hex');
const write = (p, b) => fs.writeFile(p, b, { mode: 0o600, flag: 'wx' });
const out = await fs.mkdtemp(path.join(probe, 'zoom-'));
await fs.chmod(out, 0o700);
const inputs = new Map(), checks = [], errors = [], captures = [], responses = [];
const report = { schema: 1, kind: 'private-p01-actual-zoom', verification: 'not-certified',
  browser_receipt_sha256: receiptSHA, checks, errors, captures, responses,
  limits: ['headless Chrome and automated native keys, not physical input or screen-reader certification',
    'controlled source CSS/font/assets, not complete portal provenance or real consumer adoption',
    '200% at one desktop window size; no performance, cross-browser, touch or complete target claim'] };
const check = (name, ok, detail = {}) => { checks.push({ name, ok: Boolean(ok), ...detail }); assert(ok, name); };
async function input(p, expected) {
  const s = await fs.lstat(p);
  assert(s.isFile() && s.size > 0 && s.size <= 4 * 2 ** 20, 'bounded regular input required');
  const data = await fs.readFile(p), digest = sha(data);
  if (expected) assert.equal(digest, expected, 'input identity differs: ' + path.basename(p));
  inputs.set(p, digest); return data;
}
let context, server, activeProfile, ownedPID, timedOut = false;
function ownedProcessAlive() {
  if (!ownedPID || !activeProfile) return false;
  const found = spawnSync('/bin/ps', ['-p', String(ownedPID), '-o', 'command='], { timeout: 2000, maxBuffer: 16384 });
  if (found.status === 1) return false;
  assert.equal(found.status, 0, 'cannot check owned Chrome process');
  assert(found.stdout.toString().includes('--user-data-dir=' + activeProfile), 'owned process identity changed');
  return true;
}
function stopOwnedProcess() {
  if (ownedProcessAlive()) process.kill(ownedPID, 'SIGKILL');
}
async function bounded(promise) {
  let timer;
  try { return await Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(Error('operation deadline')), 10000); })]); }
  finally { clearTimeout(timer); }
}
const deadline = setTimeout(() => {
  timedOut = true; process.exitCode = 1; errors.push('four-minute-deadline');
  try { stopOwnedProcess(); } catch { errors.push('deadline-process-stop-failed'); }
  server?.closeAllConnections();
}, 240000);
try {
  await input(new URL(import.meta.url));
  await input(new URL('../gallery-analytics/zoom-contract.mjs', import.meta.url));
  const prior = JSON.parse(await input(priorPath, receiptSHA));
  assert.equal(prior.kind, 'private-p01-browser-screening');
  assert.equal(prior.verification, 'not-certified');
  // The historical 198-check receipt remains usable as a failing regression input;
  // the strengthened normal screen adds twelve link-content checks (210 total).
  assert([198, 210].includes(prior.checks.length) && prior.checks.every(c => c.ok));
  const oldRoot = path.dirname(priorPath);
  const artifact = JSON.parse(await input(path.join(probe, 'delivery/artifact.json'), prior.inputs.artifact));
  assert.equal(artifact.adapter, 'p01-hero-v1'); assert.equal(artifact.verification, 'not-verified');
  await input(path.join(probe, 'original.html'), prior.inputs.source);
  for (const [file, key] of [['hero.templ', 'templ_sha256'], ['hero.go', 'model_sha256'], ['defaults.json', 'defaults_sha256']]) {
    await input(path.join(probe, 'delivery', file), artifact[key]);
  }
  const resources = new Map();
  resources.set('/source.css', { type: 'text/css', body: await input(path.join(oldRoot, 'source.css'), prior.inputs.tailwind.css_sha256) });
  resources.set('/hero.css', { type: 'text/css', body: await input(path.join(probe, 'delivery/hero.css'), artifact.css_sha256) });
  assert.deepEqual(prior.assets.map(a => a.name), ['logo-light.svg', 'logo-dark.svg', 'hero.jpg']);
  for (const a of [...prior.assets, { name: 'inter.woff2', mime: 'font/woff2', ...prior.inputs.font }]) {
    const body = await input(path.join(oldRoot, 'assets', a.name), a.sha256);
    assert.equal(body.length, a.bytes); resources.set('/assets/' + a.name, { type: a.mime, body });
  }
  const kinds = ['enhanced', 'enhanced-long'];
  for (const kind of kinds) {
    const fragment = await input(path.join(probe, kind + '.html'), prior.inputs[kind]);
    const body = Buffer.from(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Private hero ${kind}</title><link rel="stylesheet" href="/source.css"><link rel="stylesheet" href="/hero.css"></head><body>${fragment}</body></html>`);
    resources.set('/' + kind, { type: 'text/html', body });
  }
  const targets = ['/announcement', '/start', '/learn'];
  for (const target of targets) resources.set(target, { type: 'text/html', body: Buffer.from(`<!doctype html><html lang="en"><title>Local fixture destination</title><h1>Local ${target.slice(1)} fixture</h1><p>No account, purchase or external action occurred.</p></html>`) });
  let base;
  const csp = "default-src 'none'; style-src 'self'; font-src 'self'; img-src 'self'; script-src 'none'; connect-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'none'";
  server = http.createServer((req, res) => {
    const url = new URL(req.url, base);
    if (req.method !== 'GET' || req.headers.host !== new URL(base).host || url.origin !== base || url.search) { res.writeHead(403).end(); return; }
    if (url.pathname === '/favicon.ico') { res.writeHead(204).end(); return; }
    const resource = resources.get(url.pathname);
    if (!resource) { errors.push('unknown-route:' + url.pathname); res.writeHead(404).end(); return; }
    responses.push({ path: url.pathname, bytes: resource.body.length, sha256: sha(resource.body) });
    res.writeHead(200, { 'Content-Type': resource.type, 'Content-Security-Policy': csp, 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' });
    res.end(resource.body);
  });
  await bounded(new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); }));
  base = `http://127.0.0.1:${server.address().port}`; report.base = base;
  const pw = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
  await input(pw, 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
  const { chromium } = await import(pathToFileURL(pw));
  const { PNG } = createRequire(import.meta.url)('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright-core/lib/utilsBundle.js');
  report.profiles = [];
  for (const scheme of ['light', 'dark']) {
    activeProfile = await fs.mkdtemp(path.join(out, 'profile-')); await fs.chmod(activeProfile, 0o700);
    report.profiles.push(activeProfile);
    context = await chromium.launchPersistentContext(activeProfile, {
      executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true,
      viewport: { width: 1440, height: 1000 }, colorScheme: scheme, reducedMotion: 'reduce',
      ignoreDefaultArgs: ['--disable-back-forward-cache'],
      args: ['--enable-automation', '--disable-background-networking', '--no-proxy-server', '--no-first-run', '--no-default-browser-check'],
    });
    const browserSession = await bounded(context.browser().newBrowserCDPSession());
    try {
      const launch = await bounded(browserSession.send('Browser.getBrowserCommandLine'));
      assert(launch.arguments.includes('--user-data-dir=' + activeProfile));
      check(scheme + ':native-history-cache-not-disabled', !launch.arguments.includes('--disable-back-forward-cache'));
      const processes = await bounded(browserSession.send('SystemInfo.getProcessInfo'));
      const hosts = processes.processInfo.filter(p => p.type === 'browser'); assert.equal(hosts.length, 1);
      ownedPID = hosts[0].id; assert(ownedProcessAlive());
      (report.hosts ||= []).push({ pid: ownedPID, profile: activeProfile, arguments: launch.arguments });
    } finally { await bounded(browserSession.detach()); }
    let page, scriptSession;
    await bounded((async () => {
      page = context.pages()[0]; page.setDefaultTimeout(6000); page.setDefaultNavigationTimeout(12000);
      await page.goto('chrome://settings/appearance');
      await page.locator('#zoomLevel').selectOption('2');
      check(scheme + ':actual-appearance-setting', await page.locator('#zoomLevel').inputValue() === '2');
      await page.goto('about:blank');
      scriptSession = await context.newCDPSession(page);
      await scriptSession.send('Emulation.setScriptExecutionDisabled', { value: true });
      scriptSession.on('Page.backForwardCacheNotUsed', event => (report.historyDiagnostics ||= []).push(event));
      await scriptSession.send('Page.enable');
    })());
    report.browser = context.browser()?.version();
    context.on('serviceworker', () => errors.push('unexpected-serviceworker'));
    await context.routeWebSocket('**/*', socket => { errors.push('unexpected-websocket'); socket.close(); });
    await context.route('**/*', route => {
      const req = route.request(), u = new URL(req.url());
      if (u.origin === base && req.method() === 'GET' && !u.search && (resources.has(u.pathname) || u.pathname === '/favicon.ico')) return route.continue();
      errors.push('unexpected-request'); return route.abort();
    });
    page.on('pageerror', () => errors.push('pageerror'));
    page.on('requestfailed', r => errors.push('request-failed:' + new URL(r.url()).pathname));
    const capture = async name => {
      const session = await context.newCDPSession(page);
      try {
        const clip = zoomCaptureClip(await session.send('Page.getLayoutMetrics'));
        const { data } = await session.send('Page.captureScreenshot', { format: 'png', clip, captureBeyondViewport: true, fromSurface: true });
        const bytes = Buffer.from(data, 'base64'), filename = name + '.png';
        await write(path.join(out, filename), bytes);
        const png = PNG.sync.read(bytes), colors = new Set(); let dark = 0, light = 0, opaque = 0;
        for (let i = 0; i < png.data.length; i += 4) {
          const [r, g, b] = png.data.subarray(i, i + 3);
          if (Math.max(r, g, b) < 100) dark++; if (Math.min(r, g, b) > 220) light++;
          if (png.data[i + 3] === 255) opaque++;
          if (colors.size < 100) colors.add((r << 16) | (g << 8) | b);
        }
        const pixels = png.width * png.height;
        check(name + ':full-document-nonblank-canary', png.width === clip.width && png.height === clip.height && colors.size >= 32 && dark / pixels > 0.001 && light / pixels > 0.001 && opaque === pixels,
          { width: png.width, height: png.height, colors: colors.size, dark, light, opaque, pixels });
        captures.push({ filename, sha256: sha(bytes), bytes: bytes.length, clip });
      } finally { await session.detach(); }
    };
    for (const kind of kinds) {
      const label = `${scheme}-${kind}`;
      await page.goto(base + '/' + kind, { waitUntil: 'networkidle' });
      await page.evaluate(() => document.fonts.ready);
      await page.evaluate(() => Promise.all([...document.images].map(i => i.decode())));
      const state = await page.evaluate(() => ({
        inner: innerWidth, outer: outerWidth, dpr: devicePixelRatio, scale: visualViewport.scale,
        scroll: document.documentElement.scrollWidth, scripts: document.scripts.length,
        font: document.fonts.check('16px InterVariable'),
        images: [...document.images].map(i => ({ loaded: i.complete && i.naturalWidth > 0, visible: i.checkVisibility() })),
        anchors: [...document.querySelectorAll('a')].filter(a => a.checkVisibility()).map(a => {
          const r = a.getBoundingClientRect(), s = getComputedStyle(a);
          return { href: a.getAttribute('href'), x: r.x, right: r.right, width: r.width, height: r.height,
            clipped: /hidden|clip/.test(s.overflow), contentWidth: a.scrollWidth, boxWidth: a.clientWidth };
        }),
      }));
      check(label + ':actual-200-percent', state.inner === 720 && state.outer === 1440 && state.dpr === 2 && state.scale === 1, state);
      check(label + ':complete-scriptless-font-image-inventory', state.scripts === 0 && state.font && state.images.length === 3 && state.images.every(i => i.loaded) && state.images.filter(i => i.visible).length === 2);
      check(label + ':three-visible-actions', JSON.stringify(state.anchors.map(a => a.href)) === JSON.stringify(targets));
      check(label + ':reflow-and-44px-targets', state.scroll <= 720 && state.anchors.every(a => a.x >= 0 && a.right <= 720 && a.width >= 44 && a.height >= 44 && a.contentWidth <= a.boxWidth + 1 && !a.clipped), { anchors: state.anchors, scroll: state.scroll });
      await capture(label + '-full');
      // Every destination gets a fresh keyboard-only journey. No locator.focus,
      // click, tabindex edits or programmatic scrolling prepares the task.
      for (let index = 0; index < targets.length; index++) {
        await page.goto(base + '/' + kind, { waitUntil: 'networkidle' });
        for (let n = 0; n <= index; n++) await page.keyboard.press('Tab');
        const focusState = async () => page.evaluate(() => {
          const e = document.activeElement, r = e.getBoundingClientRect(), s = getComputedStyle(e);
          const extent = Math.max(0, parseFloat(s.outlineOffset)) + parseFloat(s.outlineWidth);
          let clipped = r.left - extent < 0 || r.right + extent > innerWidth || r.top - extent < 0 || r.bottom + extent > innerHeight;
          for (let n = e.parentElement; n; n = n.parentElement) {
            const t = getComputedStyle(n), b = n.getBoundingClientRect();
            if (/hidden|clip|scroll|auto/.test(t.overflowX) && (r.left - extent < b.left || r.right + extent > b.right)) clipped = true;
            if (/hidden|clip|scroll|auto/.test(t.overflowY) && (r.top - extent < b.top || r.bottom + extent > b.bottom)) clipped = true;
          }
          const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
          const ctx = canvas.getContext('2d');
          const rgba = color => { ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = color; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data]; };
          const ring = rgba(s.outlineColor); let background;
          for (let n = e.parentElement; n; n = n.parentElement) {
            const color = rgba(getComputedStyle(n).backgroundColor);
            if (color[3] === 255) { background = color; break; }
          }
          const luminance = a => a.slice(0, 3).map(v => v / 255).map(v => v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4).reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i], 0);
          const x = luminance(ring), y = background ? luminance(background) : 0;
          return { href: e.getAttribute('href'), outline: parseFloat(s.outlineWidth), style: s.outlineStyle, clipped, ring, background,
            contrast: background ? (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05) : 0,
            top: r.top, bottom: r.bottom, viewportHeight: innerHeight };
        });
        const visibleFocus = f => f.href === targets[index] && f.outline >= 2 && f.style === 'solid' && f.ring[3] === 255 && f.contrast >= 3 && !f.clipped;
        const focused = await focusState();
        check(label + ':native-tab-visible-focus:' + index, visibleFocus(focused), focused);
        if (index === 2) {
          // Playwright's CSS-coordinate screenshot path can capture a blank
          // off-document region after scrolling at real browser zoom. Capture
          // the currently painted viewport directly, without a CSS clip.
          const cdp = await context.newCDPSession(page);
          try {
            const { data } = await cdp.send('Page.captureScreenshot', { format: 'png', fromSurface: true, captureBeyondViewport: false });
            const bytes = Buffer.from(data, 'base64'), filename = label + '-focus.png';
            await write(path.join(out, filename), bytes);
            const png = PNG.sync.read(bytes), colors = new Set(); let opaque = 0, dark = 0, light = 0;
            for (let i = 0; i < png.data.length; i += 4) {
              const [r, g, b, alpha] = png.data.subarray(i, i + 4);
              if (alpha === 255) opaque++;
              if (Math.max(r, g, b) < 100) dark++; if (Math.min(r, g, b) > 220) light++;
              if (colors.size < 100) colors.add((r << 16) | (g << 8) | b);
            }
            const pixels = png.width * png.height;
            check(label + ':focused-viewport-nonblank-canary', png.width === 1440 && png.height === 1000 && opaque === pixels && colors.size >= 32 && dark / pixels > 0.001 && light / pixels > 0.001,
              { width: png.width, height: png.height, opaque, colors: colors.size, dark, light });
            captures.push({ filename, sha256: sha(bytes), bytes: bytes.length, mode: 'painted viewport without CSS clip' });
          } finally { await cdp.detach(); }
        }
        await Promise.all([page.waitForURL(base + targets[index], { waitUntil: 'networkidle' }), page.keyboard.press('Enter')]);
        check(label + ':enter-destination:' + index, new URL(page.url()).pathname === targets[index] && await page.locator('h1').textContent() === `Local ${targets[index].slice(1)} fixture`);
        await page.goBack({ waitUntil: 'networkidle' });
        const restored = await focusState();
        check(label + ':back-restores-visible-focus:' + index, new URL(page.url()).pathname === '/' + kind && visibleFocus(restored), restored);
      }
      for (const target of [...targets].slice(0, -1).reverse()) {
        await page.keyboard.press('Shift+Tab');
        check(label + ':reverse-tab:' + target, await page.locator(':focus').getAttribute('href') === target);
      }
    }
    await bounded(context.close()); context = null;
    check(scheme + ':owned-browser-exited', !ownedProcessAlive()); ownedPID = null;
    // Retain the fresh profile with its evidence; never touch the owner's profile.
    activeProfile = null;
  }
  for (const [p, digest] of inputs) check('input-unchanged:' + path.basename(String(p)), sha(await fs.readFile(p)) === digest);
  check('no-network-or-runtime-errors', errors.length === 0, { errors });
  report.inputs = [...inputs].map(([p, sha256]) => ({ path: String(p), sha256 }));
  const encoded = JSON.stringify(report, null, 2) + '\n'; await write(path.join(out, 'receipt.json'), encoded);
  console.log(JSON.stringify({ out, checks: checks.length, passed: checks.filter(c => c.ok).length, captures: captures.length, receipt_sha256: sha(encoded) }));
} catch (e) {
  process.exitCode = 1;
  report.inputs = [...inputs].map(([p, sha256]) => ({ path: String(p), sha256 }));
  await write(path.join(out, 'failure.json'), JSON.stringify({ error: String(e), ...report }, null, 2) + '\n');
  console.error('Hero zoom proof failed; inspect ' + path.join(out, 'failure.json'));
} finally {
  const cleanup = [];
  if (context) try { await bounded(context.close()); } catch { cleanup.push('browser-close'); }
  try { if (ownedProcessAlive()) { cleanup.push('owned-browser-required-kill'); stopOwnedProcess(); } } catch { cleanup.push('owned-browser-stop-failed'); }
  if (server) try { server.closeAllConnections(); await bounded(new Promise(resolve => server.close(resolve))); } catch { cleanup.push('server-close'); }
  clearTimeout(deadline); if (timedOut) cleanup.push('deadline');
  await write(path.join(out, 'cleanup.json'), JSON.stringify({ ok: cleanup.length === 0, errors: cleanup, profiles: report.profiles || [], scope: 'normal closure only; fresh test profiles retained, no owner profile used' }) + '\n');
  if (cleanup.length) process.exitCode = 1;
}
