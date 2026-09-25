// Explicit P08 focus-repair candidate at actual Chrome Appearance 200%.
// This does not alter or certify the unchanged drawer adapter or its references.
import assert from 'node:assert/strict';
import {createHash, randomBytes} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import {createRequire} from 'node:module';
import fs from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';

const [kitArg, probeArg, normalArg, normalSHA, baselineArg, baselineSHA, modeArg] = process.argv.slice(2);
assert((process.argv.length === 8 || (process.argv.length === 9 && modeArg === '--normal')) && [kitArg, probeArg, normalArg, baselineArg].every(path.isAbsolute)
  && [normalSHA, baselineSHA].every(value => /^[a-f0-9]{64}$/.test(value ?? '')),
  'usage: node repair-zoom.mjs KIT PRIVATE_PROBE NORMAL_RECEIPT NORMAL_SHA BASELINE_FAILURE BASELINE_SHA [--normal]');
const mode = modeArg === '--normal' ? 'normal' : 'actual-200-percent';
const [kit, probe, normalPath, baselinePath, tempRoot] = await Promise.all(
  [kitArg, probeArg, normalArg, baselineArg, os.tmpdir()].map(value => fs.realpath(value)));
assert(probe.startsWith(tempRoot + path.sep));
assert(normalPath.startsWith(probe + path.sep) && baselinePath.startsWith(probe + path.sep));

const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const artifactSHA = '34644520c31c87de479b4c61472b6ea3fb0e759f7c757db2fbb291fd9b2883ca';
const compileSHA = 'ca83dcba88f43e5145d672b83fe182ad49040417800f8abd1688426a0e42bc59';
const repairCSSSHA = '9b1fcd1b50999d5d82878e0444c55ad097a4755fd57df2057f2ab25bb453ffe8';
const normalHarnessSHA = 'f939dfd904dae4920958a9f4bedfb114e5926c1fe6f24121e18186717d6b93d6';
const baselineHarnessSHA = 'ef39958d6ab627f592eebdd9db1d7d6c0d6862d71337359949805bb9ed8bedec';
const out = await fs.mkdtemp(path.join(probe, mode === 'normal' ? 'repair-normal-' : 'repair-zoom-'));
await fs.chmod(out, 0o700);
const inputs = new Map(), outputs = new Map();
async function input(file, expected) {
  const normalized = file instanceof URL ? fileURLToPath(file) : file;
  const stat = await fs.lstat(normalized);
  assert(stat.isFile() && stat.nlink === 1 && stat.size > 0 && stat.size <= 8 * 2 ** 20, 'invalid bounded input');
  const bytes = await fs.readFile(normalized), digest = sha(bytes);
  if (expected) assert.equal(digest, expected, 'input identity differs: ' + path.basename(normalized));
  inputs.set(normalized, digest);
  return bytes;
}
async function write(name, bytes) {
  await fs.writeFile(path.join(out, name), bytes, {mode: 0o600, flag: 'wx'});
  outputs.set(name, {sha256: sha(bytes), bytes: bytes.length});
}

// Pure canaries for the geometry and paint rules used below.
const outerExtent = (width, offset) => Math.max(0, width + offset);
const sourceOver = (front, back) => {
  const fa = front[3] / 255, ba = back[3] / 255, alpha = fa + ba * (1 - fa);
  if (alpha === 0) return [0, 0, 0, 0];
  return [...front.slice(0, 3).map((value, index) => Math.round((value * fa + back[index] * ba * (1 - fa)) / alpha)), Math.round(alpha * 255)];
};
const compositeUpward = layers => layers.reduce((paint, ancestor) => sourceOver(paint, ancestor), [0, 0, 0, 0]);
const backdropFor = ({width, offset}, elementLayers, parentLayers) => compositeUpward(offset <= -width ? elementLayers : parentLayers);
function syntheticClip(rect, extent, chain) {
  const reasons = [];
  for (const item of chain) {
    const box = item.bounds;
    if (/hidden|clip|scroll|auto/.test(item.overflowX) && (rect.left - extent < box.left || rect.right + extent > box.right)) reasons.push(item.label + ':x');
    if (/hidden|clip|scroll|auto/.test(item.overflowY) && (rect.top - extent < box.top || rect.bottom + extent > box.bottom)) reasons.push(item.label + ':y');
    if (item.popover) break;
  }
  return reasons;
}
assert.equal(outerExtent(2, -2), 0);
assert.equal(outerExtent(2, 0), 2);
assert.equal(outerExtent(2, 4), 6);
assert.deepEqual(sourceOver([255, 0, 0, 128], [0, 0, 255, 255]), [128, 0, 127, 255]);
assert.deepEqual(backdropFor({width: 2, offset: -2}, [[255, 0, 0, 128], [0, 0, 255, 255]], [[0, 255, 0, 255]]), [128, 0, 127, 255]);
assert.deepEqual(backdropFor({width: 2, offset: 0}, [[255, 0, 0, 255]], [[0, 255, 0, 255]]), [0, 255, 0, 255]);
const box = {left: 0, right: 100, top: 0, bottom: 100}, inside = {left: 2, right: 98, top: 2, bottom: 98};
assert.deepEqual(syntheticClip(inside, 0, [{label: 'popover', overflowX: 'hidden', overflowY: 'hidden', bounds: box, popover: true}, {label: 'ancestor', overflowX: 'hidden', overflowY: 'hidden', bounds: {left: 20, right: 80, top: 20, bottom: 80}}]), []);
assert.deepEqual(syntheticClip({left: -1, right: 98, top: 2, bottom: 98}, 0, [{label: 'popover', overflowX: 'hidden', overflowY: 'hidden', bounds: box, popover: true}]), ['popover:x']);
assert.deepEqual(syntheticClip(inside, 0, [{label: 'child', overflowX: 'visible', overflowY: 'visible', bounds: box}, {label: 'ancestor', overflowX: 'hidden', overflowY: 'hidden', bounds: {left: 20, right: 80, top: 20, bottom: 80}}]), ['ancestor:x', 'ancestor:y']);

const report = {
  schema: 1, kind: mode === 'normal' ? 'private-p08-drawer-focus-repair-normal' : 'private-p08-drawer-focus-repair-actual-zoom', mode, verification: 'not-certified',
  variant: 'explicit-wrapper-focus-css-candidate', artifact_sha256: artifactSHA,
  compile_receipt_sha256: compileSHA, normal_receipt_sha256: normalSHA,
  baseline_failure_sha256: baselineSHA, repair_css_sha256: repairCSSSHA,
  canaries: {inset_extent: true, outset_extent: true, translucent_compositing: true, popover_own_clip_only: true},
  checks: [], captures: [], requests: [], errors: [], hosts: [], cleanup: 'pending',
  focus_screening: {minimum_outline_css_px: 2, minimum_contrast: 3, requires_unclipped: true,
    touched_target_minimum_css_px: 44, note: 'Initial noninteractive dialog-entry focus is recorded separately.'},
  limits: ['Explicit opt-in wrapper and candidate CSS only; unchanged v1/reference output is not certified.',
    'Actual installed headless Chrome Appearance 200%; keyboard and wheel are browser-protocol input, not physical input.',
    'Four synthetic GET destinations are not real application backends; no source image is fetched or substituted.']
};
const check = (name, ok, detail = {}) => report.checks.push({name, ok: Boolean(ok), ...detail});
const bounded = async promise => { let timer; try { return await Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(Error('operation deadline')), 12000); })]); } finally { clearTimeout(timer); } };
let context, server, base, profile, pid, timedOut = false;
function alive() {
  if (!pid || !profile) return false;
  const result = spawnSync('/bin/ps', ['-p', String(pid), '-o', 'command='], {timeout: 2000, maxBuffer: 16384});
  if (result.status === 1) return false;
  assert.equal(result.status, 0, 'could not inspect owned Chrome');
  assert(result.stdout.toString().includes('--user-data-dir=' + profile), 'browser identity differs');
  return true;
}
const watchdog = setTimeout(() => {
  timedOut = true; report.errors.push('five-minute deadline'); process.exitCode = 1;
  try { if (alive()) process.kill(pid, 'SIGKILL'); } catch { report.errors.push('owned process stop failed'); }
  server?.closeAllConnections();
}, 300000);

try {
  report.harness_sha256 = sha(await input(new URL(import.meta.url)));
  const normal = JSON.parse(await input(normalPath, normalSHA));
  assert.equal(normal.kind, 'private-p08-drawer-browser-screening');
  assert.equal(normal.harness_sha256, normalHarnessSHA); assert.equal(normal.passed, 229); assert.equal(normal.failed, 0);
  assert.deepEqual(normal.errors, []); assert.equal(normal.cleanup, 'ok'); assert(normal.browser_pid_absent);
  const baseline = JSON.parse(await input(baselinePath, baselineSHA));
  assert.equal(baseline.kind, 'private-p08-actual-zoom'); assert.equal(baseline.harness_sha256, baselineHarnessSHA);
  assert.equal(baseline.passed, 144); assert.equal(baseline.failed, 30); assert.deepEqual(baseline.errors, []); assert.equal(baseline.cleanup, 'ok');
  const compile = JSON.parse(await input(path.join(probe, 'compile-receipt.json'), compileSHA));
  const artifact = JSON.parse(await input(path.join(probe, 'delivery/artifact.json'), artifactSHA));
  assert.equal(compile.artifact_sha256, artifactSHA); assert.equal(artifact.adapter, 'p08-profile-drawer-v1');
  const repairCSS = await input(path.join(kit, 'internal/designadapt/drawer_focus.css'), repairCSSSHA);
  const normalRoot = path.dirname(normalPath), sourcePath = normal.requests.find(request => request.path.endsWith('/source.css'))?.path;
  assert(sourcePath); const assetPrefix = path.posix.dirname(sourcePath), docPrefix = '/repair-' + randomBytes(18).toString('hex');
  const sourceCSS = await input(path.join(normalRoot, 'source.css'), normal.outputs['source.css'].sha256);
  const assetsRoot = path.join(kit, 'data/design-assets/reference-assets/elements-1.0.22-pilot');
  const runtime = await input(path.join(assetsRoot, 'elements.js'), normal.assets['elements.js'].sha256);
  const font = await input(path.join(assetsRoot, 'inter.woff2'), normal.assets['inter.woff2'].sha256);
  const fixtureCSS = Buffer.from('html{font-family:InterVariable,system-ui,sans-serif}body{margin:0;padding:24px;background:#f9fafb;color:#111827}html.dark body{background:#111827;color:#f9fafb}h1{font-size:1rem;margin:0 0 24px}');
  const photo = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96"><rect width="96" height="96" fill="#64748b"/><circle cx="48" cy="37" r="17" fill="#e2e8f0"/><path fill="#e2e8f0" d="M17 94c2-24 14-35 31-35s29 11 31 35"/></svg>');
  const favicon = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><path fill="#64748b" d="M0 0h1v1H0z"/></svg>');
  for (const [name, body] of [['fixture.css', fixtureCSS], ['photo.svg', photo], ['favicon.svg', favicon]]) assert.equal(sha(body), normal.assets[name].sha256);
  const repairPath = assetPrefix + '/drawer-focus.css';
  const resources = new Map([[assetPrefix + '/elements.js', {type: 'text/javascript', body: runtime}], [assetPrefix + '/inter.woff2', {type: 'font/woff2', body: font}], [assetPrefix + '/source.css', {type: 'text/css', body: sourceCSS}], [assetPrefix + '/fixture.css', {type: 'text/css', body: fixtureCSS}], [assetPrefix + '/favicon.svg', {type: 'image/svg+xml', body: favicon}], [repairPath, {type: 'text/css', body: repairCSS}], ['/photo.svg', {type: 'image/svg+xml', body: photo}]]);
  for (const kind of ['typed', 'long']) for (const scheme of ['light', 'dark']) {
    const fragment = await input(path.join(probe, kind + '.html'), compile.rendered_sha256[kind + '.html']);
    const body = Buffer.from('<!doctype html><html lang="en" class="' + (scheme === 'dark' ? 'dark' : '') + '"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Drawer repair zoom specimen</title><link rel="icon" href="' + assetPrefix + '/favicon.svg"><link rel="stylesheet" href="' + assetPrefix + '/source.css"><link rel="stylesheet" href="' + assetPrefix + '/fixture.css"><link rel="stylesheet" href="' + repairPath + '"><script type="module" src="' + assetPrefix + '/elements.js"></script></head><body><h1>Synthetic drawer focus repair screening</h1><main><div data-appkit-drawer-focus>' + fragment + '</div></main></body></html>');
    resources.set(docPrefix + '/' + kind + '-' + scheme, {type: 'text/html; charset=utf-8', body});
  }
  const destinations = new Map([['/compose', 'Synthetic compose handoff'], ['/call-setup', 'Synthetic call setup'], ['/profile', 'Synthetic profile destination'], ['/history', 'Synthetic history destination']]);
  for (const [route, label] of destinations) resources.set(route, {type: 'text/html; charset=utf-8', body: Buffer.from('<!doctype html><title>Fixture result</title><h1>' + label + '</h1>')});
  report.resource_sha256 = Object.fromEntries([...resources].map(([name, value]) => [name, {sha256: sha(value.body), bytes: value.body.length}]));
  server = http.createServer((request, response) => {
    const url = new URL(request.url, base || 'http://invalid.test');
    if (!base || request.method !== 'GET' || request.headers.host !== new URL(base).host || url.origin !== base || url.search) { response.writeHead(403).end(); return; }
    const resource = resources.get(url.pathname); if (!resource) { report.errors.push('unknown route: ' + url.pathname); response.writeHead(404).end(); return; }
    report.requests.push({method: request.method, path: url.pathname, sha256: sha(resource.body)});
    response.writeHead(200, {'Content-Type': resource.type, 'Content-Security-Policy': "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self'; connect-src 'none'; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'", 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff'}); response.end(resource.body);
  });
  await bounded(new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); }));
  base = 'http://127.0.0.1:' + server.address().port;
  const playwrightPath = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
  await input(playwrightPath, 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
  const {chromium} = await import(pathToFileURL(playwrightPath));
  const {PNG} = createRequire(import.meta.url)('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright-core/lib/utilsBundle.js');
  async function ready(page) {
    await page.waitForFunction(() => ['el-dialog', 'el-dropdown', 'el-menu'].every(tag => customElements.get(tag)) && document.fonts.status === 'loaded');
    let stable = 0;
    for (let index = 0; index < 100; index++) {
      const settled = await page.evaluate(() => document.getAnimations().every(animation => animation.playState !== 'running' && !animation.pending)
        && [...document.querySelectorAll('el-menu:popover-open')].every(menu => getComputedStyle(menu).opacity === '1'));
      stable = settled ? stable + 1 : 0;
      if (stable >= 3) return;
      await page.waitForTimeout(50);
    }
    throw Error('Elements animation did not settle');
  }
  async function tabTo(page, selector, max = 24, container = '') {
    for (let index = 0; index < max; index++) { await page.keyboard.press('Tab'); if (container) check('modal Tab remains contained ' + index, await page.locator(container).evaluate(element => element.contains(document.activeElement))); if (await page.locator(selector).evaluate(element => element === document.activeElement)) return true; }
    return false;
  }
  async function focusGeometry(page) {
    const facts = await page.evaluate(() => {
      const element = document.activeElement, rect = element.getBoundingClientRect(), style = getComputedStyle(element);
      const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1; const context = canvas.getContext('2d');
      const rgba = color => { context.clearRect(0, 0, 1, 1); context.fillStyle = color; context.fillRect(0, 0, 1, 1); return [...context.getImageData(0, 0, 1, 1).data]; };
      const clipping = [];
      for (let node = element.parentElement; node; node = node.parentElement) {
        if (node === document.body || node === document.documentElement) continue;
        const nodeStyle = getComputedStyle(node), box = node.getBoundingClientRect();
        clipping.push({label: node.tagName.toLowerCase() + (node.id ? '#' + node.id : ''), overflowX: nodeStyle.overflowX, overflowY: nodeStyle.overflowY, bounds: {left: box.left, right: box.right, top: box.top, bottom: box.bottom}, popover: node.matches(':popover-open')});
      }
      const layers = start => { const values = []; for (let node = start; node; node = node.parentElement) { const nodeStyle = getComputedStyle(node); values.push({rgba: rgba(nodeStyle.backgroundColor), image: nodeStyle.backgroundImage}); } return values; };
      return {tag: element.tagName, href: element.getAttribute('href'), command: element.getAttribute('command'), rect: {left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom, width: rect.width, height: rect.height}, viewport: {width: innerWidth, height: innerHeight}, outline: style.outlineStyle, outlineWidth: parseFloat(style.outlineWidth) || 0, outlineOffset: parseFloat(style.outlineOffset) || 0, ring: rgba(style.outlineColor), elementLayers: layers(element), parentLayers: layers(element.parentElement), clipping};
    });
    const extent = outerExtent(facts.outlineWidth, facts.outlineOffset), clipReasons = [];
    if (facts.rect.left - extent < 0 || facts.rect.right + extent > facts.viewport.width) clipReasons.push('viewport:x');
    if (facts.rect.top - extent < 0 || facts.rect.bottom + extent > facts.viewport.height) clipReasons.push('viewport:y');
    clipReasons.push(...syntheticClip(facts.rect, extent, facts.clipping));
    const selectedLayers = facts.outlineOffset <= -facts.outlineWidth ? facts.elementLayers : facts.parentLayers;
    let backdrop = compositeUpward(selectedLayers.map(layer => layer.rgba));
    if (backdrop[3] < 255) backdrop = sourceOver(backdrop, [255, 255, 255, 255]);
    const luminance = color => color.slice(0, 3).map(value => value / 255).map(value => value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4).reduce((sum, value, index) => sum + value * [.2126, .7152, .0722][index], 0), ringLuminance = luminance(facts.ring), backdropLuminance = luminance(backdrop);
    return {...facts, width: facts.rect.width, height: facts.rect.height, top: facts.rect.top, bottom: facts.rect.bottom, outerExtent: extent, backdrop, contrast: (Math.max(ringLuminance, backdropLuminance) + .05) / (Math.min(ringLuminance, backdropLuminance) + .05), clipped: clipReasons.length > 0, clipReasons, backgroundImages: [...new Set(selectedLayers.map(layer => layer.image))]};
  }
  const focusOK = geometry => geometry.outline === 'solid' && geometry.outlineWidth >= 2 && geometry.ring[3] === 255 && geometry.contrast >= 3 && !geometry.clipped && geometry.backgroundImages.every(value => value === 'none');
  const target44 = geometry => geometry.width >= 44 && geometry.height >= 44;
  async function selectedGeometry(page) {
    return page.locator('dialog').evaluate(dialog => { const final = dialog.querySelector('dl>div:last-child dd'); let scroll = null; for (let node = final; node && node !== dialog.parentElement; node = node.parentElement) { const style = getComputedStyle(node); if (/auto|scroll/.test(style.overflowY) && node.scrollHeight > node.clientHeight + 1) { scroll = node; break; } } const clipNode = scroll ?? dialog, clip = clipNode.getBoundingClientRect(), measure = selector => { const node = dialog.querySelector(selector); if (!node) return null; const range = document.createRange(); range.selectNodeContents(node); const text = [...range.getClientRects()].filter(rect => rect.width > 0 && rect.height > 0).map(rect => ({left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom})); return {text, horizontal: text.length > 0 && text.every(rect => rect.left >= clip.left - 1 && rect.right <= clip.right + 1), vertical: text.length > 0 && text.every(rect => rect.top >= clip.top - 1 && rect.bottom <= clip.bottom + 1)}; }; return {pageWidth: document.documentElement.scrollWidth, viewportWidth: innerWidth, scrollable: Boolean(scroll), clip: {left: clip.left, right: clip.right, top: clip.top, bottom: clip.bottom, scrollTop: clipNode.scrollTop, scrollHeight: clipNode.scrollHeight, clientHeight: clipNode.clientHeight}, selected: {heading: measure('h2'), name: measure('h3'), action: measure('a[href="/compose"]'), detail: measure('dl>div:last-child dd')}}; });
  }
  let expectedCaptureWidth = 1440;
  async function capture(page, name) {
    const session = await context.newCDPSession(page); try { const {data} = await session.send('Page.captureScreenshot', {format: 'png', fromSurface: true, captureBeyondViewport: false}), bytes = Buffer.from(data, 'base64'), png = PNG.sync.read(bytes); let opaque = 0, dark = 0, light = 0; const colors = new Set(); for (let index = 0; index < png.data.length; index += 4) { const [r, g, b, a] = png.data.subarray(index, index + 4); if (a === 255) opaque++; if (Math.max(r, g, b) < 100) dark++; if (Math.min(r, g,b) > 220) light++; if (colors.size < 100) colors.add((r << 16) | (g << 8) | b); } const pixels = png.width * png.height; check(name + ': painted capture nonblank', png.width === expectedCaptureWidth && png.height === 1000 && opaque === pixels && colors.size >= 32 && dark / pixels > .001 && light / pixels > .001, {width: png.width, height: png.height, colors: colors.size, dark, light}); await write(name + '.png', bytes); report.captures.push({file: name + '.png', sha256: sha(bytes), bytes: bytes.length, mode: 'painted viewport DIP'}); } finally { await session.detach(); }
  }
  for (const scheme of ['light', 'dark']) {
    profile = await fs.mkdtemp(path.join(out, 'profile-')); await fs.chmod(profile, 0o700);
    context = await chromium.launchPersistentContext(profile, {executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, timeout: 20000, viewport: {width: 1440, height: 1000}, colorScheme: scheme, reducedMotion: 'reduce', serviceWorkers: 'block', args: ['--enable-automation', '--disable-background-networking', '--no-proxy-server', '--no-first-run', '--no-default-browser-check']});
    const host = await context.browser().newBrowserCDPSession(); try { const command = await host.send('Browser.getBrowserCommandLine'); assert(command.arguments.includes('--user-data-dir=' + profile)); const hosts = (await host.send('SystemInfo.getProcessInfo')).processInfo.filter(process => process.type === 'browser'); assert.equal(hosts.length, 1); pid = hosts[0].id; assert(alive()); report.hosts.push({pid, profile}); } finally { await host.detach(); }
    const page = context.pages()[0]; page.setDefaultTimeout(6000); page.setDefaultNavigationTimeout(12000);
    await page.goto('chrome://settings/appearance'); await page.locator('#zoomLevel').selectOption(mode === 'normal' ? '1' : '2'); check(scheme + ': requested Appearance zoom setting', await page.locator('#zoomLevel').inputValue() === (mode === 'normal' ? '1' : '2')); await page.goto('about:blank');
    context.on('serviceworker', () => report.errors.push('unexpected service worker')); await context.routeWebSocket('**/*', socket => { report.errors.push('unexpected websocket'); socket.close(); });
    await context.route('**/*', route => { const request = route.request(), url = new URL(request.url()); if (url.origin !== base || request.method() !== 'GET' || url.search || !resources.has(url.pathname)) { report.errors.push('unexpected request'); return route.abort(); } return route.continue(); });
    page.on('pageerror', error => report.errors.push('pageerror: ' + error.message)); page.on('requestfailed', request => report.errors.push('request failed: ' + new URL(request.url()).pathname));
    for (const viewportWidth of mode === 'normal' ? [390, 1440] : [1440]) {
    if (mode === 'normal') await page.setViewportSize({width: viewportWidth, height: 1000});
    expectedCaptureWidth = viewportWidth;
    const lane = mode === 'normal' ? `${scheme}-${viewportWidth}` : scheme;
    const navigate = async kind => { await page.goto(base + docPrefix + '/' + kind + '-' + scheme, {waitUntil: 'networkidle'}); await ready(page); const geometry = await page.evaluate(() => ({inner: innerWidth, outer: outerWidth, dpr: devicePixelRatio, scale: visualViewport.scale, wrapper: document.querySelectorAll('[data-appkit-drawer-focus]').length})); const expected = mode === 'normal' ? geometry.inner === viewportWidth && geometry.dpr === 1 && geometry.scale === 1 : geometry.inner === 720 && geometry.outer === 1440 && geometry.dpr === 2 && geometry.scale === 1; check(`${lane}-${kind}: requested geometry and explicit wrapper`, expected && geometry.wrapper === 1, geometry); };
    for (const kind of ['typed', 'long']) {
      const key = `${lane}-${kind}`; await navigate(kind);
      check(key + ': Tab reaches opener', await tabTo(page, 'button[command="show-modal"]')); let focus = await focusGeometry(page); check(key + ': opener 44px target', target44(focus), focus); check(key + ': opener focus screening', focusOK(focus), focus);
      await page.keyboard.press('Enter'); await ready(page); focus = await focusGeometry(page); check(key + ': valid noninteractive modal-entry focus recorded', focus.tag === 'DIV' && focus.href === null && focus.command === null, focus);
      await page.keyboard.press('Tab'); check(key + ': first modal Tab reaches close', await page.locator('dialog button[command="close"]').evaluate(element => element === document.activeElement)); focus = await focusGeometry(page); check(key + ': unchanged close focus screening', focusOK(focus), focus);
      const geometry = await selectedGeometry(page); check(key + ': selected text fits scroll clip', geometry.pageWidth <= geometry.viewportWidth && geometry.clip && Object.values(geometry.selected).every(value => value?.horizontal), geometry);
      if (geometry.scrollable) { await page.mouse.move((geometry.clip.left + geometry.clip.right) / 2, Math.max(geometry.clip.top + 1, Math.min(geometry.clip.bottom - 1, (geometry.clip.top + geometry.clip.bottom) / 2))); for (let index = 0; index < 4; index++) await page.mouse.wheel(0, 2000); await page.waitForTimeout(100); const reached = await page.locator('dialog').evaluate(dialog => { const final = dialog.querySelector('dl>div:last-child dd'); let scroll = null; for (let node = final; node && node !== dialog.parentElement; node = node.parentElement) { const style = getComputedStyle(node); if (/auto|scroll/.test(style.overflowY) && node.scrollHeight > node.clientHeight + 1) { scroll = node; break; } } if (!scroll) return null; const clip = scroll.getBoundingClientRect(), range = document.createRange(); range.selectNodeContents(final); const text = [...range.getClientRects()].filter(rect => rect.width > 0 && rect.height > 0); return {scrollTop: scroll.scrollTop, atEnd: scroll.scrollTop + scroll.clientHeight >= scroll.scrollHeight - 1, fullyVisible: text.length > 0 && text.every(rect => rect.left >= clip.left - 1 && rect.right <= clip.right + 1 && rect.top >= clip.top - 1 && rect.bottom <= clip.bottom + 1)}; }); check(key + ': wheel reveals final detail', reached?.scrollTop > 0 && reached.atEnd && reached.fullyVisible, reached ?? {}); }
      else check(key + ': final detail visible without scrolling', kind === 'typed' && geometry.selected.detail.vertical, geometry.selected.detail);
      await navigate(kind); check(key + ': Tab re-enters opener', await tabTo(page, 'button[command="show-modal"]')); await page.keyboard.press('Enter'); await ready(page); check(key + ': Tab reaches nested trigger', await tabTo(page, 'dialog el-dropdown>button', 24, 'dialog')); focus = await focusGeometry(page); check(key + ': nested trigger 44px target', target44(focus), focus); check(key + ': nested trigger focus screening', focusOK(focus), focus);
      await page.keyboard.press('Enter'); await ready(page); const items = page.locator('dialog el-menu a'); check(key + ': first item focused', await items.first().evaluate(element => element === document.activeElement)); focus = await focusGeometry(page); check(key + ': first item 44px target', target44(focus), focus); check(key + ': first item focus screening', focusOK(focus), focus);
      await page.keyboard.press('ArrowDown'); focus = await focusGeometry(page); check(key + ': ArrowDown selects second item', await items.nth(1).evaluate(element => element === document.activeElement)); check(key + ': second item 44px target', target44(focus), focus); check(key + ': second item focus screening', focusOK(focus), focus); await capture(page, key + '-focus');
      await page.keyboard.press('Escape'); await page.waitForFunction(() => !document.querySelector('dialog el-menu').matches(':popover-open')); await ready(page); focus = await focusGeometry(page); check(key + ': menu Escape restores screened trigger', await page.locator('dialog el-dropdown>button').evaluate(element => element === document.activeElement) && focusOK(focus), focus); await page.keyboard.press('Escape'); await page.waitForFunction(() => !document.querySelector('dialog').open); await ready(page); focus = await focusGeometry(page); check(key + ': dialog Escape restores screened opener', await page.locator('button[command="show-modal"]').evaluate(element => element === document.activeElement) && focusOK(focus), focus);
    }
    for (const target of destinations.keys()) {
      await navigate('typed'); await tabTo(page, 'button[command="show-modal"]'); await page.keyboard.press('Enter'); await ready(page);
      if (target === '/compose' || target === '/call-setup') check(`${lane}-${target}: Tab reaches destination`, await tabTo(page, `dialog a[href="${target}"]`, 24, 'dialog')); else { check(`${lane}-${target}: Tab reaches menu trigger`, await tabTo(page, 'dialog el-dropdown>button', 24, 'dialog')); await page.keyboard.press('Enter'); await ready(page); if (target === '/history') await page.keyboard.press('ArrowDown'); check(`${lane}-${target}: nested destination focused`, await page.locator(`dialog a[href="${target}"]`).evaluate(element => element === document.activeElement)); }
      const destinationFocus = await focusGeometry(page); check(`${lane}-${target}: destination 44px target`, target44(destinationFocus), destinationFocus); check(`${lane}-${target}: destination focus screening`, focusOK(destinationFocus), destinationFocus); await Promise.all([page.waitForURL(base + target, {waitUntil: 'networkidle'}), page.keyboard.press('Enter')]); check(`${lane}-${target}: native GET destination`, await page.locator('h1').textContent() === destinations.get(target));
    }
    }
    await bounded(context.close()); context = undefined; check(scheme + ': owned Chrome exited', !alive()); report.hosts.at(-1).absent = true; pid = undefined; profile = undefined;
  }
  for (const [file, digest] of inputs) assert.equal(sha(await fs.readFile(file)), digest, 'input changed during proof'); report.inputs_unchanged = true;
} catch (error) { report.errors.push(String(error)); process.exitCode = 1; }
finally {
  try { if (context) await bounded(context.close()); if (pid) { assert(!alive(), 'owned Chrome remains'); report.hosts.at(-1).absent = true; } if (server?.listening) { server.closeAllConnections(); await bounded(new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()))); } if (base) await bounded(new Promise((resolve, reject) => { const socket = net.createConnection({host: '127.0.0.1', port: Number(new URL(base).port)}); socket.once('connect', () => { socket.destroy(); reject(Error('listener remains')); }); socket.once('error', error => { socket.destroy(); error.code === 'ECONNREFUSED' ? resolve() : reject(error); }); })); report.cleanup = 'ok'; report.listener_refused = Boolean(base); } catch (error) { report.errors.push('cleanup: ' + String(error)); try { if (alive()) process.kill(pid, 'SIGKILL'); } catch {} server?.closeAllConnections(); process.exitCode = 1; }
  clearTimeout(watchdog); if (timedOut) process.exitCode = 1; report.inputs = Object.fromEntries(inputs); report.outputs = Object.fromEntries(outputs); report.passed = report.checks.filter(check => check.ok).length; report.failed = report.checks.filter(check => !check.ok).length; if (report.failed || report.errors.length) process.exitCode = 1;
  const body = Buffer.from(JSON.stringify(report, null, 2) + '\n'), name = report.failed || report.errors.length ? 'failure.json' : 'receipt.json'; await fs.writeFile(path.join(out, name), body, {mode: 0o600, flag: 'wx'}); process.stdout.write(JSON.stringify({out, passed: report.passed, failed: report.failed, errors: report.errors.map(error => error.split('\n')[0]), file: name, sha256: sha(body)}) + '\n');
}
