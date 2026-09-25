// One-cell capture diagnosis. Copy as browser.mjs beside the unchanged owner and
// zoom contract in a private harness. Never substitutes for a visual review.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import { inspectZoom } from './zoom-contract.mjs';

const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const expected = ['find_symbol', '1,234,567', 'references', '1,000'];
const bounded = async (work, ms = 12000) => {
  let timer;
  try {
    return await Promise.race([work(), new Promise((_, reject) => {
      timer = setTimeout(() => reject(Error('paint operation deadline')), ms);
    })]);
  } finally { clearTimeout(timer); }
};

export async function runProbe({ context, origin, output, report }) {
  assert.equal(new URL(origin).hostname, '127.0.0.1');
  assert.equal(new URL(origin).protocol, 'http:');
  assert.equal(report.contract.workload.mode, 'one-light-200-paint-v1');
  const page = await context.newPage();
  const session = await context.newCDPSession(page);
  const pending = new Set(), blocked = [], errors = [], bodies = [], captures = [];
  report.paint = { expected, blocked, errors, bodies, captures,
    visual_acceptance: 'not evaluated by this driver; inspect exact retained PNGs',
    limits: ['One light/200 cell, not dark or guide acceptance.',
      'Same historical stress fixture; no aggregate/table agreement or performance claim.'] };
  const check = (name, pass, detail) => {
    report.checks.push({ name, pass: Boolean(pass), detail });
    assert.ok(pass, name);
  };
  const state = () => page.evaluate(() => {
    const node = document.querySelector('svg[aria-label="calls per tool"]');
    return { targetCount: document.querySelectorAll('svg[aria-label="calls per tool"]').length,
      text: [...(node?.querySelectorAll('text') || [])].map(el => el.textContent),
      box: node?.getBoundingClientRect().toJSON(), scrollY, scrollX,
      dpr: devicePixelRatio, inner: innerWidth, height: innerHeight, outer: outerWidth,
      scale: visualViewport.scale, scrollWidth: document.documentElement.scrollWidth,
      theme: document.documentElement.dataset.theme };
  });
  const inside = s => s.box && s.box.x >= 0 && s.box.y >= 0 &&
    s.box.right <= s.inner && s.box.bottom <= s.height;
  const capture = async (name, method) => {
    const before = await state();
    check(`${name} exact target visible geometrically`, before.targetCount === 1 &&
      JSON.stringify(before.text) === JSON.stringify(expected) && inside(before), before);
    const metrics = await session.send('Page.getLayoutMetrics');
    check(`${name} native 200% geometry`, inspectZoom('2', before, metrics), metrics);
    // With no clip, CDP captures the current compositor viewport directly. It
    // receives no mixture of CSS scroll offsets and device-pixel viewport sizes.
    const request = { format: 'png', fromSurface: true, captureBeyondViewport: false };
    const bytes = method === 'cdp'
      ? Buffer.from((await bounded(() => session.send('Page.captureScreenshot', request))).data, 'base64')
      : await bounded(() => page.screenshot({ fullPage: false, timeout: 10000 }));
    const after = await state();
    assert.ok(bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])), 'PNG signature');
    assert.equal(bytes.toString('ascii', 12, 16), 'IHDR');
    const png = { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
    check(`${name} physical viewport raster`, png.width === 1440 && png.height === 1000, png);
    check(`${name} capture preserves target and scroll`, JSON.stringify(before) === JSON.stringify(after), { before, after });
    const file = path.join(output, name + '.png');
    await fs.writeFile(file, bytes, { flag: 'wx' });
    captures.push({ name, method, request: method === 'cdp' ? request : { fullPage: false },
      file, sha256: hash(bytes), bytes: bytes.length, png, before, after, metrics });
  };
  try {
    await page.setViewportSize({ width: 1440, height: 1000 });
    page.setDefaultTimeout(10000);
    page.setDefaultNavigationTimeout(15000);
    await context.route('**/*', route => {
      const r = route.request(), u = new URL(r.url());
      if (r.method() === 'GET' && u.origin === origin &&
          (u.pathname === '/large' && u.search === '?scheme=light' || u.pathname === '/app.css' && u.search === ''))
        return route.continue();
      blocked.push({ method: r.method(), url: r.url() });
      return route.abort('blockedbyclient');
    });
    await context.routeWebSocket('**/*', socket => { blocked.push({ websocket: socket.url() }); socket.close(); });
    page.on('pageerror', error => errors.push(String(error)));
    page.on('requestfailed', r => errors.push(r.url() + ': ' + r.failure()?.errorText));
    page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
    page.on('response', response => {
      if (!response.url().startsWith(origin)) return;
      const task = bounded(async () => {
        const bytes = await response.body();
        bodies.push({ url: response.url(), status: response.status(), bytes: bytes.length, sha256: hash(bytes) });
      }).catch(error => errors.push(String(error))).finally(() => pending.delete(task));
      pending.add(task);
    });
    await page.goto('chrome://settings/appearance', { waitUntil: 'load' });
    await page.locator('#zoomLevel').selectOption('2');
    check('Chrome Appearance 200 selected', await page.locator('#zoomLevel').inputValue() === '2');
    await page.goto('about:blank', { waitUntil: 'load' });
    await session.send('Emulation.setScriptExecutionDisabled', { value: true });
    await page.emulateMedia({ colorScheme: 'light', reducedMotion: 'reduce' });
    const response = await page.goto(origin + '/large?scheme=light', { waitUntil: 'load' });
    check('synthetic page HTTP 200', response.status() === 200);
    const assets = await bounded(() => page.evaluate(async () => {
      await document.fonts.ready;
      return { fonts: document.fonts.status, scripts: document.scripts.length, ready: document.readyState,
        styles: [...document.querySelectorAll('link[rel="stylesheet"]')].map(link => Boolean(link.sheet)) };
    }));
    check('script-free page and completed assets', assets.fonts === 'loaded' && assets.scripts === 0 &&
      assets.ready === 'complete' && assets.styles.length === 1 && assets.styles.every(Boolean), assets);
    await Promise.all([...pending]);
    check('exact page and CSS bodies complete', bodies.length === 2 && bodies.every(b => b.status === 200) &&
      bodies.filter(b => b.url === origin + '/app.css').length === 1 &&
      bodies.find(b => b.url === origin + '/app.css').sha256 === report.contract.inputs.app_css.sha256, bodies);
    await page.locator('svg[aria-label="calls per tool"]').scrollIntoViewIfNeeded();
    check('light theme selected', (await state()).theme === 'light');
    // Paired instrument comparison, bracketed by the candidate on the SAME page.
    // No DOM mutation, style override, arbitrary delay or image transformation.
    await capture('light-200-cdp-before', 'cdp');
    await capture('light-200-playwright-control', 'playwright');
    await capture('light-200-cdp-after', 'cdp');
    check('bracketed compositor rasters match', captures[0].sha256 === captures[2].sha256,
      captures.map(c => ({ name: c.name, sha256: c.sha256 })));
    check('no blocked requests or browser errors', blocked.length === 0 && errors.length === 0, { blocked, errors });
    report.cells.push({ name: 'light-200-paint', captures: captures.length });
  } finally {
    await Promise.allSettled([...pending]);
    await session.detach().catch(error => report.errors.push(String(error)));
    await page.close().catch(error => report.errors.push(String(error)));
  }
}
