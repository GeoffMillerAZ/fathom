// Explicit private source probe. Fetch only with --fetch-assets; no installs,
// signed-in browser, application data or source-store writes.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import * as fs from 'node:fs/promises';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [kitArg, probeArg, assetMode, assetPath, assetDigest] = process.argv.slice(2);
assert((process.argv.length === 5 && assetMode === '--fetch-assets') || (process.argv.length === 7 && assetMode === '--assets-from' && path.isAbsolute(assetPath) && /^[a-f0-9]{64}$/.test(assetDigest)), 'usage: node browser.mjs KIT PRIVATE_PROBE --fetch-assets | --assets-from PRIVATE_ASSETS_JSON SHA256');
assert(path.isAbsolute(kitArg) && path.isAbsolute(probeArg));
const kit = await fs.realpath(kitArg), probe = await fs.realpath(probeArg);
assert(probe.startsWith(await fs.realpath(os.tmpdir()) + path.sep), 'probe must be a system-temp child');
const sha = b => createHash('sha256').update(b).digest('hex');
const read = p => fs.readFile(p);
const write = (p, b) => fs.writeFile(p, b, { mode: 0o600, flag: 'wx' });
const out = await fs.mkdtemp(path.join(probe, 'browser-'));
await fs.chmod(out, 0o700);
const assetsDir = path.join(out, 'assets');
await fs.mkdir(assetsDir, { mode: 0o700 });
const inputs = {}, checks = [], captures = [], responses = [], errors = [], inherited = [];
const check = (name, ok, detail = {}) => { checks.push({ name, ok: Boolean(ok), ...detail }); };
let browser, browserServer, server;
let watchdogFired = false;
const watchdog = setTimeout(() => {
  watchdogFired = true;
  process.exitCode = 1;
  errors.push('four-minute-watchdog');
  browserServer?.process()?.kill('SIGKILL');
  server?.closeAllConnections();
}, 240000);
const boundedClose = async promise => {
  let timer;
  try { await Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('cleanup timeout')), 10000); })]); }
  finally { clearTimeout(timer); }
};
try {
  inputs.harness_sha256 = sha(await read(new URL(import.meta.url)));
  const source = await read(path.join(probe, 'original.html'));
  assert.equal(sha(source), '356c19f1095d82529cb54014e3b94a67b7531489bb26b96b5fba8686d94beba9');
  const artifactBytes = await read(path.join(probe, 'delivery/artifact.json'));
  const a = JSON.parse(artifactBytes);
  assert.equal(a.verification, 'not-verified');
  assert.equal(a.source.sha256, sha(source));
  assert.equal(a.adapter, 'p01-hero-v1');
  inputs.artifact = sha(artifactBytes); inputs.source = sha(source);
  for (const [file, digest] of [['hero.templ', a.templ_sha256], ['hero.go', a.model_sha256], ['hero.css', a.css_sha256], ['defaults.json', a.defaults_sha256]]) {
    assert.equal(sha(await read(path.join(probe, 'delivery', file))), digest, 'delivery changed');
  }
  const assetSpecs = [
    ['logo-light.svg', a.source_assets.logo_light_url, 'image/svg+xml'],
    ['logo-dark.svg', a.source_assets.logo_dark_url, 'image/svg+xml'],
    ['hero.jpg', a.source_assets.image_url, 'image/jpeg'],
  ];
  let retainedAssets;
  if (assetMode === '--assets-from') {
    const real = await fs.realpath(assetPath);
    assert(real.startsWith(await fs.realpath(os.tmpdir()) + path.sep));
    const data = await read(real); assert(data.length <= 16384);
    assert.equal(sha(data), assetDigest, 'retained asset manifest changed');
    retainedAssets = { root: path.dirname(real), manifest: JSON.parse(data) };
    assert.equal(retainedAssets.manifest.assets.length, 3);
    inputs.retained_asset_manifest_sha256 = sha(data);
  }
  const assets = [];
  for (const [name, url, mime] of assetSpecs) {
    const expectedHost = name === 'hero.jpg' ? 'images.unsplash.com' : 'tailwindcss.com';
    assert.equal(new URL(url).origin, `https://${expectedHost}`, 'source asset host changed');
    let data;
    if (retainedAssets) {
      const matches = retainedAssets.manifest.assets.filter(a => a.name === name);
      assert.equal(matches.length, 1); const pin = matches[0];
      assert.equal(pin.url, url); assert.equal(pin.mime, mime);
      const p = path.join(retainedAssets.root, 'assets', name);
      const stat = await fs.lstat(p); assert(stat.isFile() && stat.size > 0 && stat.size <= 3 * 2 ** 20);
      data = await read(p); assert.equal(data.length, pin.bytes); assert.equal(sha(data), pin.sha256);
    } else {
      const response = await fetch(url, { redirect: 'error', signal: AbortSignal.timeout(20000), headers: { Accept: mime } });
      assert(response.ok, `asset HTTP ${response.status}`);
      assert.equal(response.headers.get('content-type')?.split(';')[0], mime, 'unexpected asset type');
      const chunks = []; let size = 0;
      for await (const chunk of response.body) { size += chunk.length; assert(size <= 3 * 2 ** 20, 'asset too large'); chunks.push(chunk); }
      data = Buffer.concat(chunks);
    }
    assert(data.length > 0);
    if (mime === 'image/jpeg') assert.equal(data.subarray(0, 3).toString('hex'), 'ffd8ff');
    await write(path.join(assetsDir, name), data);
    assets.push({ name, url, mime, bytes: data.length, sha256: sha(data) });
  }
  await write(path.join(out, 'assets.json'), JSON.stringify({ captured_at: new Date().toISOString(), assets }, null, 2) + '\n');
  // This retained font is a controlled test condition, not portal font provenance.
  const fontDocument = await read(path.join(kit, 'data/design-assets/previews/p02-pricing/62a91c913f3860527b9d77b0b2227595d21b35ae1d06fb37a03a55091a30c8b8/original-light.html'));
  assert.equal(sha(fontDocument), '5d3a5473878298809b4e3c6d0a142e8379c56ffc974ee1a7dcadce0614ca04ad');
  const fonts = [...fontDocument.toString().matchAll(/data:font\/woff2;base64,([A-Za-z0-9+/=]+)/g)];
  assert.equal(fonts.length, 1);
  const font = Buffer.from(fonts[0][1], 'base64');
  await write(path.join(assetsDir, 'inter.woff2'), font);
  inputs.font = { sha256: sha(font), bytes: font.length, provenance: 'retained local InterVariable test font, not portal baseline' };
  const tw = '/Users/geoff/Library/Caches/app-kit/tailwindcss/v4.3.0/tailwindcss-macos-arm64';
  assert.equal(sha(await read(tw)), '56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e');
  const cssInput = '@import "tailwindcss" source(none);\n@source "../original.html";\n@source "../delivery/hero.templ";\n@theme { --font-sans: InterVariable, ui-sans-serif, sans-serif; }\n@font-face{font-family:InterVariable;font-style:normal;font-weight:100 900;font-display:block;src:url("/assets/inter.woff2") format("woff2")}\n';
  await write(path.join(out, 'input.css'), cssInput);
  const built = spawnSync(tw, ['-i', path.join(out, 'input.css'), '-o', path.join(out, 'source.css'), '--minify'], { timeout: 30000, maxBuffer: 65536 });
  assert.equal(built.status, 0, 'pinned CSS compiler failed');
  await fs.chmod(path.join(out, 'source.css'), 0o600);
  const css = await read(path.join(out, 'source.css'));
  inputs.tailwind = { version: '4.3.0', executable_sha256: sha(await read(tw)), css_sha256: sha(css), bytes: css.length };
  let reference = source.toString(), links = 0;
  reference = reference.replace(/href="#"/g, () => `href="${['/announcement', '/start', '/learn'][links++]}"`);
  assert.equal(links, 3);
  for (const [name, url] of assetSpecs) {
    const needle = `src="${url}"`; assert.equal(reference.split(needle).length, 2);
    reference = reference.replace(needle, `src="/assets/${name}"`);
  }
  const fragments = new Map([['reference', reference]]);
  for (const name of ['typed', 'enhanced', 'typed-long', 'enhanced-long']) {
    const b = await read(path.join(probe, name + '.html')); inputs[name] = sha(b); fragments.set(name, b.toString());
  }
  const resources = new Map([['/source.css', { type: 'text/css', body: css }], ['/hero.css', { type: 'text/css', body: await read(path.join(probe, 'delivery/hero.css')) }]]);
  for (const entry of [...assets, { name: 'inter.woff2', mime: 'font/woff2' }]) resources.set('/assets/' + entry.name, { type: entry.mime, body: await read(path.join(assetsDir, entry.name)) });
  const doc = (name, fragment) => `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Private hero ${name}</title><link rel="stylesheet" href="/source.css">${name.startsWith('enhanced') ? '<link rel="stylesheet" href="/hero.css">' : ''}</head><body>${fragment}</body></html>`;
  for (const [name, fragment] of fragments) resources.set('/' + name, { type: 'text/html', body: Buffer.from(doc(name, fragment)) });
  for (const target of ['/announcement', '/start', '/learn']) resources.set(target, { type: 'text/html', body: Buffer.from(`<!doctype html><html lang="en"><title>Local fixture destination</title><h1>Local ${target.slice(1)} fixture</h1><p>No account, purchase or external action occurred.</p></html>`) });
  const csp = "default-src 'none'; style-src 'self'; font-src 'self'; img-src 'self'; script-src 'none'; connect-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'none'";
  let base;
  server = http.createServer((req, res) => {
    const u = new URL(req.url, base);
    if (req.method !== 'GET' || req.headers.host !== new URL(base).host || u.origin !== base) { res.writeHead(403).end(); return; }
    if (u.pathname === '/favicon.ico') { res.writeHead(204).end(); return; }
    const r = resources.get(u.pathname);
    if (!r) { errors.push('unexpected-route:' + u.pathname); res.writeHead(404).end(); return; }
    responses.push({ path: u.pathname, bytes: r.body.length });
    res.writeHead(200, { 'Content-Type': r.type, 'Content-Security-Policy': csp, 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' }); res.end(r.body);
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  base = `http://127.0.0.1:${server.address().port}`;
  const pw = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
  assert.equal(sha(await read(pw)), 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
  const { chromium } = await import(pathToFileURL(pw));
  browserServer = await chromium.launchServer({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true });
  browser = await chromium.connect(browserServer.wsEndpoint());
  inputs.browser = browser.version(); inputs.playwright_entry_sha256 = sha(await read(pw));
  inputs.browser_pid = browserServer.process().pid;
  for (const scheme of ['light', 'dark']) for (const width of [390, 768, 1440]) {
    const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme: scheme, reducedMotion: 'reduce', deviceScaleFactor: 1, javaScriptEnabled: true });
    await context.route('**/*', route => {
      if (new URL(route.request().url()).origin === base) return route.continue();
      errors.push('external-browser-request'); return route.abort();
    });
    const page = await context.newPage();
    page.on('pageerror', () => errors.push('pageerror'));
    const pair = {};
    for (const kind of fragments.keys()) {
      const label = `${scheme}-${width}-${kind}`;
      await page.goto(base + '/' + kind, { waitUntil: 'networkidle' });
      await page.evaluate(() => document.fonts.ready);
      await page.evaluate(() => Promise.all([...document.images].map(i => i.decode())));
      const state = await page.evaluate(() => ({
        scroll: document.documentElement.scrollWidth, width: innerWidth,
        font: document.fonts.check('16px InterVariable'), scripts: document.scripts.length,
        images: [...document.images].map(i => ({ loaded: i.complete && i.naturalWidth > 0, visible: i.getBoundingClientRect().width > 0, alt: i.alt })),
        anchors: [...document.querySelectorAll('a')].filter(a => a.getBoundingClientRect().width > 0).map(a => ({ href: a.getAttribute('href'), width: a.getBoundingClientRect().width, height: a.getBoundingClientRect().height, contentWidth: a.scrollWidth, boxWidth: a.clientWidth })),
      }));
      check(label + ':font-and-no-scripts', state.font && state.scripts === 0);
      const wantedLinks = width < 640 ? ['/start', '/learn'] : ['/announcement', '/start', '/learn'];
      check(label + ':visible-link-inventory', JSON.stringify(state.anchors.map(a => a.href)) === JSON.stringify(wantedLinks));
      check(label + ':images', state.images.length === 3 && state.images.every(i => i.loaded) && state.images.filter(i => i.visible).length === 2, { images: state.images });
      if (kind.startsWith('enhanced')) {
        check(label + ':no-horizontal-overflow', state.scroll <= width, { scroll: state.scroll });
        check(label + ':44px-targets', state.anchors.every(a => a.height >= 44 && a.width >= 44), { anchors: state.anchors });
        check(label + ':content-within-link-boxes', state.anchors.every(a => a.contentWidth <= a.boxWidth + 1), { anchors: state.anchors });
      } else {
        inherited.push({ label, horizontal_overflow: state.scroll > width, undersized_targets: state.anchors.filter(a => a.height < 44 || a.width < 44) });
      }
      const shot = await page.screenshot({ animations: 'disabled', fullPage: false });
      await write(path.join(out, label + '.png'), shot); captures.push({ name: label + '.png', sha256: sha(shot), bytes: shot.length });
      if (kind === 'reference' || kind === 'typed') pair[kind] = shot;
      if (kind === 'typed' || kind.startsWith('enhanced')) {
        for (const a of state.anchors) {
          await Promise.all([page.waitForURL(base + a.href, { waitUntil: 'networkidle' }), page.locator(`a[href="${a.href}"]`).click()]);
          check(label + ':pointer:' + a.href, new URL(page.url()).pathname === a.href);
          await page.goBack({ waitUntil: 'networkidle' });
        }
      }
    }
    check(`${scheme}-${width}:exact-source-typed-pixels`, pair.reference.equals(pair.typed));
    // Fresh document starts keyboard navigation from the top, no focus injection.
    await page.goto(base + '/enhanced', { waitUntil: 'networkidle' });
    for (const target of width < 640 ? ['/start', '/learn'] : ['/announcement', '/start', '/learn']) {
      await page.keyboard.press('Tab');
      const focused = await page.evaluate(() => {
        const e = document.activeElement, s = getComputedStyle(e), r = e.getBoundingClientRect();
        const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
        const c = canvas.getContext('2d');
        const rgba = color => { c.clearRect(0, 0, 1, 1); c.fillStyle = color; c.fillRect(0, 0, 1, 1); return [...c.getImageData(0, 0, 1, 1).data]; };
        const ring = rgba(s.outlineColor), offset = Math.max(0, parseFloat(s.outlineOffset)) + parseFloat(s.outlineWidth);
        let bg = null, clipped = r.left - offset < 0 || r.right + offset > innerWidth || r.top - offset < 0 || r.bottom + offset > innerHeight;
        for (let n = e.parentElement; n; n = n.parentElement) {
          const style = getComputedStyle(n), box = n.getBoundingClientRect(), color = rgba(style.backgroundColor);
          if (!bg && color[3] === 255) bg = color;
          if (/hidden|clip|scroll|auto/.test(style.overflowX) && (r.left - offset < box.left || r.right + offset > box.right)) clipped = true;
          if (/hidden|clip|scroll|auto/.test(style.overflowY) && (r.top - offset < box.top || r.bottom + offset > box.bottom)) clipped = true;
        }
        const luminance = a => a.slice(0, 3).map(v => v / 255).map(v => v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4).reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i], 0);
        const x = luminance(ring), y = bg ? luminance(bg) : 0;
        return { href: e.getAttribute('href'), width: parseFloat(s.outlineWidth), style: s.outlineStyle, ring, background: bg, clipped, contrast: bg ? (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05) : 0 };
      });
      check(`${scheme}-${width}:keyboard-focus:${target}`, focused.href === target && focused.width >= 2 && focused.style === 'solid' && focused.ring[3] === 255 && focused.contrast >= 3 && !focused.clipped, focused);
    }
    const focusedShot = await page.screenshot({ animations: 'disabled' });
    const focusedName = `${scheme}-${width}-keyboard-focus.png`;
    await write(path.join(out, focusedName), focusedShot); captures.push({ name: focusedName, sha256: sha(focusedShot), bytes: focusedShot.length });
    await Promise.all([page.waitForURL(base + '/learn', { waitUntil: 'networkidle' }), page.keyboard.press('Enter')]);
    check(`${scheme}-${width}:keyboard-enter`, new URL(page.url()).pathname === '/learn');
    await page.goBack({ waitUntil: 'networkidle' });
    check(`${scheme}-${width}:keyboard-back`, new URL(page.url()).pathname === '/enhanced');
    await context.close();
  }
  check('no-unexpected-network-or-runtime-errors', errors.length === 0, { errors });
  check('harness-unchanged', sha(await read(new URL(import.meta.url))) === inputs.harness_sha256);
  const receipt = { schema: 1, kind: 'private-p01-browser-screening', verification: 'not-certified', inputs, assets, checks, inherited, captures, responses,
    limits: ['controlled Inter/Tailwind conditions, not full portal fidelity', 'headless Chromium only', 'viewport screenshots, not full-page visual review', 'no actual 200% zoom, screen reader, touch hardware or adverse-network proof', 'no performance benchmark, standalone consumer adoption or target certification', 'script-src none; browser instrumentation uses evaluation, not app JavaScript'] };
  const bytes = JSON.stringify(receipt, null, 2) + '\n'; await write(path.join(out, 'receipt.json'), bytes);
  console.log(JSON.stringify({ out, checks: checks.length, passed: checks.filter(c => c.ok).length, failed: checks.filter(c => !c.ok).map(c => c.name), receipt_sha256: sha(bytes) }));
  if (checks.some(c => !c.ok)) process.exitCode = 1;
} catch (error) {
  await write(path.join(out, 'failure.json'), JSON.stringify({ error: String(error), inputs, checks, errors }, null, 2) + '\n');
  console.error(`Hero browser proof failed; inspect ${path.join(out, 'failure.json')}`); process.exitCode = 1;
} finally {
  const cleanup = [];
  try { if (browser) await boundedClose(browser.close()); } catch { cleanup.push('browser-client-close'); }
  try { if (browserServer) await boundedClose(browserServer.close()); } catch { cleanup.push('browser-host-close'); browserServer?.process()?.kill('SIGKILL'); }
  try { if (server) { server.closeAllConnections(); await boundedClose(new Promise(resolve => server.close(resolve))); } } catch { cleanup.push('loopback-server-close'); }
  clearTimeout(watchdog);
  if (watchdogFired) cleanup.push('four-minute-watchdog');
  await write(path.join(out, 'cleanup.json'), JSON.stringify({ ok: cleanup.length === 0, errors: cleanup, scope: 'normal browser host and server close; forced-kill descendants are not certified' }) + '\n');
  if (cleanup.length) { process.exitCode = 1; console.error('Hero probe cleanup failed; inspect cleanup.json'); }
}
