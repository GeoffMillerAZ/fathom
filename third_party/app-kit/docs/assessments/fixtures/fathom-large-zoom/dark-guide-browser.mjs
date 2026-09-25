// Separate dark/200 appearance task. Historical probes and failed rasters stay intact.
// Copy as browser.mjs beside the unchanged frozen owner/zoom/lifecycle helpers.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import { inspectZoom } from './zoom-contract.mjs';

const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bounded = async work => {
  let timer;
  try {
    return await Promise.race([work(), new Promise((_, reject) => {
      timer = setTimeout(() => reject(Error('dark guide operation exceeded 15s')), 15000);
    })]);
  } finally { clearTimeout(timer); }
};

export async function runProbe({ context, origin, output, report }) {
  assert.equal(new URL(origin).hostname, '127.0.0.1');
  assert.equal(new URL(origin).protocol, 'http:');
  assert.equal(report.contract.workload.mode, 'dark-200-guide-paint-v1');
  const page = await context.newPage();
  const session = await context.newCDPSession(page);
  const pending = new Set(), blocked = [], errors = [], consoleErrors = [], bodies = [], captures = [];
  report.dark_guide = { blocked, errors, consoleErrors, bodies, captures,
    visual_acceptance: 'not evaluated by this driver; inspect exact retained PNGs',
    limits: ['Selected dark charts and dark specimen journey, not all guide specimens.',
      'Reset intentionally returns to the source default light specimen.',
      'Historical stress fixture is not an aggregate/chart agreement or performance oracle.'] };
  const check = (name, pass, detail) => {
    report.checks.push({ name, pass: Boolean(pass), detail });
    assert.ok(pass, name);
  };
  const drain = () => bounded(async () => { while (pending.size) await Promise.allSettled([...pending]); });
  async function state(locator) {
    return bounded(async () => ({
      ...await page.evaluate(() => ({ dpr: devicePixelRatio, inner: innerWidth, height: innerHeight,
        outer: outerWidth, scale: visualViewport.scale, scrollWidth: document.documentElement.scrollWidth,
        scrollX, scrollY, theme: document.documentElement.dataset.theme,
        focused: { tag: document.activeElement?.tagName, id: document.activeElement?.id,
          role: document.activeElement?.getAttribute('role') } })),
      box: await locator.boundingBox(), text: await locator.textContent(),
    }));
  }
  async function capture(name, locator) {
    check(`${name} unique target`, await locator.count() === 1);
    await locator.scrollIntoViewIfNeeded();
    await drain();
    const before = await state(locator), b = before.box;
    check(`${name} target inside viewport`, b && b.x >= 0 && b.y >= 0 &&
      b.x + b.width <= before.inner && b.y + b.height <= before.height, before);
    const metrics = await bounded(() => session.send('Page.getLayoutMetrics'));
    check(`${name} actual 200% zoom`, inspectZoom('2', before, metrics), metrics);
    const request = { format: 'png', fromSurface: true, captureBeyondViewport: false };
    const bytes = Buffer.from((await bounded(() => session.send('Page.captureScreenshot', request))).data, 'base64');
    const file = path.join(output, name + '.png');
    await fs.writeFile(file, bytes, { flag: 'wx' });
    const after = await state(locator);
    const item = { name, file, request, sha256: hash(bytes), bytes: bytes.length, before, after, metrics };
    captures.push(item); // Retain pixels even if the postconditions below fail.
    check(`${name} PNG signature`, bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])) &&
      bytes.toString('ascii', 12, 16) === 'IHDR');
    item.png = { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
    check(`${name} 1440x1000 physical raster`, item.png.width === 1440 && item.png.height === 1000, item.png);
    check(`${name} capture preserves target, scroll and focus`, JSON.stringify(before) === JSON.stringify(after), { before, after });
  }
  async function assets(frame, label) {
    const s = await bounded(() => frame.evaluate(async () => {
      await document.fonts.ready;
      return { ready: document.readyState, fonts: document.fonts.status, scripts: document.scripts.length,
        styles: [...document.querySelectorAll('link[rel~="stylesheet"]')].map(n => Boolean(n.sheet)),
        theme: document.documentElement.dataset.theme, innerWidth,
        scrollWidth: document.documentElement.scrollWidth,
        radius: getComputedStyle(document.documentElement).getPropertyValue('--ak-radius-md').trim(),
        background: getComputedStyle(document.documentElement).backgroundColor };
    }));
    check(`${label} completed script-free assets`, s.ready === 'complete' && s.fonts === 'loaded' &&
      s.scripts === 0 && s.styles.length > 0 && s.styles.every(Boolean), s);
    check(`${label} no horizontal overflow`, s.scrollWidth <= s.innerWidth + 1, s);
    await drain();
    return s;
  }
  async function tabTo(locator, label) {
    check(`${label} unique control`, await locator.count() === 1);
    for (let i = 0; i < 180; i++) {
      if (await locator.evaluate(n => n === document.activeElement)) return;
      await page.keyboard.press('Tab');
    }
    throw Error(`${label} not reached by native Tab`);
  }
  async function submit(name, expectedStatus) {
    await tabTo(page.getByRole('button', { name, exact: true }), name);
    const [response] = await Promise.all([
      page.waitForResponse(r => r.url() === origin + '/dev/theme' && r.request().method() === 'POST'),
      page.waitForNavigation({ waitUntil: 'load' }), page.keyboard.press('Enter'),
    ]);
    check(`${name} HTTP ${expectedStatus}`, response.status() === expectedStatus, response.status());
    await assets(page, name);
  }
  async function preview(label, scheme) {
    const iframe = page.locator('iframe.sg-preview');
    check(`${label} opaque preview`, await iframe.count() === 1 && await iframe.getAttribute('sandbox') === '');
    const frame = await (await iframe.elementHandle()).contentFrame();
    assert.ok(frame);
    const s = await assets(frame, label);
    check(`${label} selected scheme`, s.theme === scheme && await page.locator('select[name="scheme"]').inputValue() === scheme, s);
    const controls = await page.evaluate(() => [...document.querySelectorAll('.sg button,.sg input:not([type="hidden"]),.sg select,.sg-editor summary')]
      .filter(n => n.checkVisibility()).map(n => ({ name: n.getAttribute('name') || n.textContent.trim(),
        rect: n.getBoundingClientRect().toJSON() })));
    check(`${label} controls fit and reach 44px`, controls.length > 0 && controls.every(({ rect: r }) =>
      r.x >= -1 && r.right <= 721 && r.width >= 44 && r.height >= 44), controls);
    const marker = frame.locator('main > p').first();
    check(`${label} synthetic disclosure`, (await marker.textContent()).includes('Synthetic appearance preview only'));
    await capture(label, marker);
    return s;
  }
  try {
    await page.setViewportSize({ width: 1440, height: 1000 });
    page.setDefaultTimeout(10000); page.setDefaultNavigationTimeout(15000);
    await context.route('**/*', route => {
      const r = route.request(), u = new URL(r.url());
      const allowed = u.origin === origin && (
        r.method() === 'GET' && (u.pathname === '/large' && u.search === '?scheme=dark' ||
          u.pathname === '/app.css' && u.search === '' ||
          ['/dev/theme/preview', '/dev/theme/style.css'].includes(u.pathname)) ||
        ['GET', 'POST'].includes(r.method()) && u.pathname === '/dev/theme' && u.search === '');
      if (allowed) return route.continue();
      blocked.push({ method: r.method(), url: r.url() }); return route.abort('blockedbyclient');
    });
    await context.routeWebSocket('**/*', socket => { blocked.push({ websocket: socket.url() }); socket.close(); });
    page.on('pageerror', e => errors.push(String(e)));
    page.on('requestfailed', r => errors.push(r.url() + ': ' + r.failure()?.errorText));
    page.on('console', m => { if (m.type() === 'error') consoleErrors.push({ text: m.text(), location: m.location() }); });
    page.on('download', d => { errors.push('unexpected download'); d.cancel().catch(e => errors.push(String(e))); });
    page.on('response', response => {
      if (new URL(response.url()).origin !== origin) return;
      const task = bounded(async () => {
        const bytes = await response.body();
        bodies.push({ url: response.url(), method: response.request().method(), status: response.status(),
          bytes: bytes.length, sha256: hash(bytes) });
      }).catch(e => errors.push(String(e))).finally(() => pending.delete(task));
      pending.add(task);
    });
    await page.goto('chrome://settings/appearance', { waitUntil: 'load' });
    await page.locator('#zoomLevel').selectOption('2');
    check('Chrome Appearance 200 selected', await page.locator('#zoomLevel').inputValue() === '2');
    await page.goto('about:blank', { waitUntil: 'load' });
    await bounded(() => session.send('Emulation.setScriptExecutionDisabled', { value: true }));
    await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
    const response = await page.goto(origin + '/large?scheme=dark', { waitUntil: 'load' });
    check('dark chart HTTP 200', response.status() === 200);
    check('dark chart theme', (await assets(page, 'dark chart')).theme === 'dark');
    for (const [name, aria, expected] of [
      ['dark-200-grade', 'grade distribution on 2026-09-21', ['A', '1,000']],
      ['dark-200-calls', 'calls per tool', ['find_symbol', '1,234,567', 'references', '1,000']],
    ]) {
      const svg = page.locator(`svg[aria-label="${aria}"]`);
      check(`${name} exact labels`, JSON.stringify(await svg.locator('text').allTextContents()) === JSON.stringify(expected));
      await capture(name, svg);
    }
    const guide = await page.goto(origin + '/dev/theme', { waitUntil: 'load' });
    check('guide HTTP 200', guide.status() === 200); await assets(page, 'guide');
    const scheme = page.locator('select[name="scheme"]');
    await tabTo(scheme, 'Scheme');
    await page.keyboard.press('Home'); await page.keyboard.press('ArrowDown'); await page.keyboard.press('Tab');
    check('native Scheme selection is dark', await scheme.inputValue() === 'dark');
    await submit('Show source', 200);
    const baseline = await preview('guide-dark-200-source', 'dark');
    const summary = page.locator('.sg-editor summary').filter({ hasText: 'Corner radii' });
    await tabTo(summary, 'Corner radii'); await page.keyboard.press('Enter');
    const radius = page.locator('input[name="radius.md"]');
    await tabTo(radius, 'radius.md'); await page.keyboard.press('Meta+A'); await page.keyboard.type('-1px');
    await submit('Preview candidate', 422);
    check('validation alert receives native focus', await page.locator(':focus').getAttribute('role') === 'alert');
    await capture('guide-dark-200-validation', page.getByRole('alert'));
    const repair = page.getByRole('link', { name: 'Fix radius.md', exact: true });
    await tabTo(repair, 'repair'); await page.keyboard.press('Enter');
    check('repair focuses retained invalid value', await radius.evaluate(n => n === document.activeElement) &&
      await radius.inputValue() === '-1px' && await radius.getAttribute('aria-invalid') === 'true');
    await capture('guide-dark-200-repair', radius);
    await page.keyboard.press('Meta+A'); await page.keyboard.type('1rem');
    await submit('Preview candidate', 200);
    check('candidate value retained', await radius.inputValue() === '1rem');
    const candidate = await preview('guide-dark-200-candidate', 'dark');
    check('preview uses changed radius token', candidate.radius === '1rem' && candidate.radius !== baseline.radius, { baseline, candidate });
    check('candidate has reviewed identity and token diff', /^[a-f0-9]{64}$/.test(await page.locator('input[name="review_digest"]').inputValue()) &&
      (await page.getByRole('table', { name: 'Resolved candidate compared with immutable source' }).textContent()).includes('radius.md'));
    await tabTo(page.getByRole('link', { name: 'Reset to source', exact: true }), 'Reset to source');
    await Promise.all([page.waitForNavigation({ waitUntil: 'load' }), page.keyboard.press('Enter')]);
    await assets(page, 'reset');
    check('reset clears edits and acknowledgement', await radius.inputValue() === '' &&
      await page.locator('input[name="review_digest"]').inputValue() === '');
    const reset = await preview('guide-200-reset-default', 'light');
    check('reset restores source radius', reset.radius === baseline.radius, { reset: reset.radius, source: baseline.radius });
    await drain();
    const css = bodies.filter(b => b.url === origin + '/app.css');
    check('all consumer CSS bodies exact', css.length >= 5 && css.every(b => b.status === 200 &&
      b.sha256 === report.contract.inputs.app_css.sha256), css);
    check('responses only 200 and intended validation 422', bodies.length > 0 &&
      bodies.every(b => b.status === 200 || b.url === origin + '/dev/theme' && b.method === 'POST' && b.status === 422) &&
      bodies.filter(b => b.status === 422).length === 1, bodies);
    check('no blocked requests or browser errors', blocked.length === 0 && errors.length === 0, { blocked, errors });
    check('console only expected 422', consoleErrors.length <= 1 && consoleErrors.every(e =>
      e.location.url === origin + '/dev/theme' && /^Failed to load resource: the server responded with a status of 422\b/.test(e.text)), consoleErrors);
    check('seven planned rasters retained', captures.length === 7);
    report.cells.push({ name: 'dark-200-chart-and-guide', captures: captures.length });
  } finally {
    await drain().catch(e => report.errors.push(String(e)));
    await bounded(() => session.detach()).catch(e => report.errors.push(String(e)));
    await bounded(() => page.close()).catch(e => report.errors.push(String(e)));
  }
}
