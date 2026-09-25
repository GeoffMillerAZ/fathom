// Synthetic signed-chart browser probe. No dependency install, signed-in profile,
// external request, production endpoint or performance claim.
// Args: Playwright entry, installed Chrome, exact fixture origin, new output dir.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [playwrightEntry, chrome, origin, output] = process.argv.slice(2);
const base = new URL(origin);
assert.equal(base.protocol, 'http:');
assert.equal(base.hostname, '127.0.0.1');
assert.equal(base.origin, origin);
assert.ok(path.isAbsolute(playwrightEntry));
assert.ok(path.isAbsolute(chrome));
assert.ok(path.isAbsolute(output));

const cases = [
  { id: 'wide-light', width: 1280, theme: 'light' },
  { id: 'wide-dark', width: 1280, theme: 'dark' },
  { id: 'narrow-light', width: 390, theme: 'light' },
  { id: 'narrow-dark', width: 390, theme: 'dark' },
];
const expectedRows = [
  ['n2', 'Negative two', '-2', 'points', 'present', '', '', '', 'accent', 'Synthetic observation; sign is not a quality rating'],
  ['n1', 'Negative one', '-1', 'points', 'present', '', '', '', 'accent', 'Synthetic observation; sign is not a quality rating'],
  ['zero', 'Zero', '0', 'points', 'present', '', '', '', 'accent', 'Synthetic observation; sign is not a quality rating'],
  ['p1', 'Positive one', '1', 'points', 'present', '', '', '', 'accent', 'Synthetic observation; sign is not a quality rating'],
  ['p2', 'Positive two', '2', 'points', 'present', '', '', '', 'accent', 'Synthetic observation; sign is not a quality rating'],
  ['missing', 'Unobserved', '', 'points', 'missing', 'Not reported', '', '', 'accent', 'Synthetic observation; sign is not a quality rating'],
];
const expectedContext = {
  chart_id: 'signed-observations', title: 'Signed observations', unit: 'points',
  grain: 'synthetic observation', aggregation: 'one selected observation per row; no aggregation',
  window_kind: 'snapshot', window: 'Snapshot at 2026-09-22T12:00:00Z [UTC]',
  source_kind: 'derived', source: 'Fixed synthetic signed observations; not production measurements.', source_href: '', source_draft: 'false',
  evaluated_at: '2026-09-22T12:00:00Z', freshness: 'Source freshness unknown: Synthetic fixture has no independent source observation time',
  completeness: 'partial', coverage_note: 'Five measured values and one unobserved row; not a complete total.',
  uncertainty: 'Illustrative values only; uncertainty is not quantified.', decimal_places: '2',
};
const allowedKeys = new Set(['theme', 'period', 'compare', 'category', 'state']);
const report = {
  scope: 'four synthetic signed-chart cells; layout, semantics and native CSV only',
  cases: [], requests: [], blocked: [], failures: [],
  limits: ['not a performance result', 'not accessibility certification', 'not consumer adoption'],
};
let browser;
let ownsOutput = false;

function check(pass, name, detail) {
  if (!pass) report.failures.push({ name, detail });
}
function sha(bytes) {
  return crypto.createHash('sha256').update(bytes).digest('hex');
}
function parseCSV(text) {
  const rows = [];
  let row = [], field = '', quoted = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (quoted) {
      if (ch === '"' && text[i + 1] === '"') { field += '"'; i++; }
      else if (ch === '"') quoted = false;
      else field += ch;
    } else if (ch === '"') quoted = true;
    else if (ch === ',') { row.push(field); field = ''; }
    else if (ch === '\n') { row.push(field.replace(/\r$/, '')); rows.push(row); row = []; field = ''; }
    else field += ch;
  }
  if (quoted) throw new Error('unterminated CSV quote');
  if (field || row.length) { row.push(field.replace(/\r$/, '')); rows.push(row); }
  return rows;
}
function validateCSV(text, id) {
  const rows = parseCSV(text), blank = rows.findIndex(row => row.length === 1 && row[0] === '');
  assert.ok(blank > 0, `${id}: CSV context separator`);
  const context = Object.fromEntries(rows.slice(0, blank).map(row => {
    assert.equal(row.length, 2, `${id}: context row width`); return row;
  }));
  assert.deepEqual(Object.keys(context), [
    'chart_id', 'title', 'unit', 'grain', 'aggregation', 'window_kind', 'window',
    'source_kind', 'source', 'source_href', 'source_draft', 'evaluated_at',
    'freshness', 'completeness', 'coverage_note', 'uncertainty', 'decimal_places',
  ], `${id}: exact context fields/order`);
  assert.deepEqual(context, expectedContext, `${id}: exact context values`);
  assert.deepEqual(rows[blank + 1], ['id', 'label', 'value', 'unit', 'state', 'missing_reason', 'lower', 'upper', 'role', 'role_label']);
  const data = rows.slice(blank + 2);
  assert.equal(data.length, expectedRows.length);
  for (let i = 0; i < data.length; i++) {
    assert.deepEqual(data[i], expectedRows[i], `${id}: exact row ${expectedRows[i][0]}`);
  }
  return context;
}
async function tabTo(page, locator, name) {
  await page.locator('body').click({ position: { x: 1, y: 1 } });
  for (let i = 0; i < 80; i++) {
    await page.keyboard.press('Tab');
    if (await locator.evaluate((node) => node === document.activeElement).catch(() => false)) return;
  }
  throw new Error(`${name} was not keyboard reachable`);
}
async function inspectKeyboardScroller(page, locator, name) {
  await tabTo(page, locator, name);
  const before = await locator.evaluate(node => {
    const style = getComputedStyle(node);
    return { left: node.scrollLeft, client: node.clientWidth, scroll: node.scrollWidth, outlineStyle: style.outlineStyle, outlineWidth: style.outlineWidth };
  });
  const outlined = before.outlineStyle !== 'none' && Number.parseFloat(before.outlineWidth) > 0;
  check(outlined, `${name}: visible focus outline`, before);
  if (before.scroll > before.client) {
    await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(100);
    const after = await locator.evaluate(node => node.scrollLeft);
    check(after > before.left, `${name}: ArrowRight scrolls overflowing region`, { before: before.left, after, client: before.client, scroll: before.scroll });
  }
  return before;
}

try {
  await fs.mkdir(output); await fs.chmod(output, 0o700); ownsOutput = true;
  const { chromium } = await import(pathToFileURL(await fs.realpath(playwrightEntry)).href);
  browser = await chromium.launch({ executablePath: await fs.realpath(chrome), headless: true });
  report.browser = browser.version();
  for (const testCase of cases) {
    const result = { ...testCase, checks: {}, responses: [] };
    const failureStart = report.failures.length;
    const context = await browser.newContext({
      acceptDownloads: true, viewport: { width: testCase.width, height: 1000 },
      colorScheme: testCase.theme, reducedMotion: 'reduce', serviceWorkers: 'block',
    });
    try {
      context.setDefaultTimeout(10000);
      await context.route('**/*', async route => {
        const request = route.request(), url = new URL(request.url());
        const exactPath = ['/analytics/signed', '/analytics/signed/export.csv'].includes(url.pathname);
        const exactCSS = url.pathname === '/static/app.css' && !url.search;
        const keysOK = [...url.searchParams.keys()].every(key => allowedKeys.has(key));
        const themeOK = !url.searchParams.has('theme') || url.searchParams.get('theme') === testCase.theme;
        if (url.origin === origin && ['GET', 'HEAD'].includes(request.method()) && ((exactPath && keysOK && themeOK) || exactCSS)) return route.continue();
        report.blocked.push({ case: testCase.id, method: request.method(), url: request.url() });
        await route.abort();
      });
      await context.routeWebSocket('**/*', socket => { report.blocked.push({ case: testCase.id, websocket: socket.url() }); socket.close(); });
      const page = await context.newPage();
      page.on('pageerror', error => report.failures.push({ name: `${testCase.id}: page error`, detail: error.message }));
      page.on('console', message => { if (message.type() === 'error') report.failures.push({ name: `${testCase.id}: console error`, detail: message.text() }); });
      page.on('requestfailed', request => {
        if (!request.url().includes('/analytics/signed/export.csv')) report.failures.push({ name: `${testCase.id}: request failed`, detail: { url: request.url(), error: request.failure()?.errorText } });
      });
      page.on('response', async response => {
        const url = new URL(response.url());
        if (url.origin !== origin || !['/analytics/signed', '/static/app.css'].includes(url.pathname)) return;
        const bytes = await response.body();
        result.responses.push({ path: url.pathname, status: response.status(), bytes: bytes.length, sha256: sha(bytes) });
      });
      const response = await page.goto(`${origin}/analytics/signed?theme=${testCase.theme}`, { waitUntil: 'load' });
      assert.equal(response.status(), 200);
      const html = await response.body();
      const csp = response.headers()['content-security-policy'] || '';
      check(csp === "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'", `${testCase.id}: exact restrictive CSP`, csp);
      check(await page.locator('script').count() === 0, `${testCase.id}: document has no scripts`);
      check(html.length < 20 * 1024, `${testCase.id}: HTML under 20KiB`, html.length);
      const cssResponse = await context.request.get(`${origin}/static/app.css`), css = await cssResponse.body();
      check(cssResponse.status() === 200 && html.length + css.length < 128 * 1024, `${testCase.id}: HTML+CSS under 128KiB`, { html: html.length, css: css.length });
      const chart = page.locator('[data-analytics-diverging]');
      const axis = chart.locator('[data-analytics-zero-axis]');
      assert.equal(await chart.count(), 1); assert.equal(await axis.count(), 1);
      const geometry = await chart.evaluate(svg => {
        const number = (node, name) => Number(node.getAttribute(name));
        const axis = svg.querySelector('[data-analytics-zero-axis]'), x0 = number(axis, 'x1');
        const rows = [...svg.querySelectorAll('[data-analytics-row]')].map(group => {
          const id = group.getAttribute('data-analytics-row'), bar = group.querySelector('[data-analytics-bar]'), zero = group.querySelector('[data-analytics-zero]');
          return { id, bar: bar && { x: number(bar, 'x'), width: number(bar, 'width') }, zero: zero && number(zero, 'x1') };
        });
        const box = svg.getBoundingClientRect();
        const labelsFit = [...svg.querySelectorAll('text')].every(label => { const r = label.getBoundingClientRect(); return r.left >= box.left - 1 && r.right <= box.right + 1 && r.top >= box.top - 1 && r.bottom <= box.bottom + 1; });
        return { x0, rows, labelsFit };
      });
      assert.deepEqual(geometry.rows.map(row => row.id), ['n2', 'n1', 'zero', 'p1', 'p2']);
      const byID = Object.fromEntries(geometry.rows.map(row => [row.id, row]));
      const near = (a, b) => Math.abs(a - b) <= 0.05;
      assert.ok(byID.n2.bar.width > 0 && near(byID.n2.bar.width, byID.p2.bar.width));
      assert.ok(byID.n1.bar.width > 0 && near(byID.n1.bar.width, byID.p1.bar.width));
      assert.ok(near(byID.n2.bar.width, 2 * byID.n1.bar.width));
      assert.ok(near(byID.n2.bar.x + byID.n2.bar.width, geometry.x0));
      assert.ok(near(byID.n1.bar.x + byID.n1.bar.width, geometry.x0));
      assert.ok(near(byID.p1.bar.x, geometry.x0) && near(byID.p2.bar.x, geometry.x0));
      assert.ok(near(byID.zero.zero, geometry.x0));
      check(geometry.labelsFit, `${testCase.id}: SVG glyph boxes fit`);
      const contrast = await chart.evaluate(svg => {
        const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
        const ctx = canvas.getContext('2d', { willReadFrequently: true });
        const rgba = value => {
          ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = '#000'; ctx.fillStyle = value; ctx.fillRect(0, 0, 1, 1);
          return [...ctx.getImageData(0, 0, 1, 1).data].map((part, i) => i === 3 ? part / 255 : part);
        };
        const backdrop = node => {
          for (let parent = node.parentElement; parent; parent = parent.parentElement) {
            const color = rgba(getComputedStyle(parent).backgroundColor);
            if (color[3] >= 0.999) return color;
          }
          return rgba(getComputedStyle(document.documentElement).backgroundColor);
        };
        const composite = (foreground, background) => foreground.slice(0, 3).map((part, i) => part * foreground[3] + background[i] * (1 - foreground[3]));
        const luminance = color => color.slice(0, 3).map(part => part / 255).map(part => part <= .04045 ? part / 12.92 : ((part + .055) / 1.055) ** 2.4).reduce((sum, part, i) => sum + part * [.2126, .7152, .0722][i], 0);
        const ratio = (node, property) => {
          const bg = backdrop(node), fg = composite(rgba(getComputedStyle(node)[property]), bg), a = luminance(fg), b = luminance(bg);
          return (Math.max(a, b) + .05) / (Math.min(a, b) + .05);
        };
        return {
          text: [...svg.querySelectorAll('text')].map(node => ({ text: node.textContent, ratio: ratio(node, 'fill') })),
          marks: [...svg.querySelectorAll('[data-analytics-bar]')].map(node => ratio(node, 'fill')),
          axis: ratio(svg.querySelector('[data-analytics-zero-axis]'), 'stroke'),
        };
      });
      check(contrast.text.every(item => item.ratio >= 4.5), `${testCase.id}: computed SVG text contrast >=4.5`, contrast.text);
      check(contrast.marks.every(ratio => ratio >= 3), `${testCase.id}: computed marks contrast >=3`, contrast.marks);
      check(contrast.axis >= 3, `${testCase.id}: computed zero-axis contrast >=3`, contrast.axis);
      result.contrast = contrast;
      check(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth && document.body.scrollWidth <= document.body.clientWidth), `${testCase.id}: no body overflow`);
      const chartRegion = page.locator('[role="region"]:has([data-analytics-diverging])').first();
      const tableRegion = page.locator('[role="region"]:has(table)').first();
      const exportLink = page.locator('a[href*="/analytics/signed/export.csv"]').first();
      result.chartScroller = await inspectKeyboardScroller(page, chartRegion, `${testCase.id}: chart region`);
      result.tableScroller = await inspectKeyboardScroller(page, tableRegion, `${testCase.id}: table region`);
      await tabTo(page, exportLink, `${testCase.id}: export link`);
      const pending = page.waitForEvent('download', { timeout: 10000 });
      await page.keyboard.press('Enter');
      const download = await pending, destination = path.join(output, `${testCase.id}.csv`);
      await download.saveAs(destination);
      assert.equal(await download.failure(), null);
      const csv = await fs.readFile(destination);
      result.csv = { bytes: csv.length, sha256: sha(csv), context: validateCSV(csv.toString('utf8'), testCase.id) };
      await page.screenshot({ path: path.join(output, `${testCase.id}.png`), fullPage: true });
      result.passed = report.failures.length === failureStart;
    } catch (error) {
      result.passed = false; result.error = String(error.stack || error); report.failures.push({ name: `${testCase.id}: case`, detail: result.error });
    } finally {
      await context.close().catch(error => report.failures.push({ name: `${testCase.id}: context cleanup`, detail: String(error) }));
      report.cases.push(result);
    }
  }
  check(report.cases.length === 4 && report.cases.every(result => result.passed), 'all four cells complete');
  check(report.blocked.length === 0, 'no request escaped the allowlist', report.blocked);
  report.observationComplete = report.failures.length === 0;
} catch (error) {
  report.error = String(error.stack || error); process.exitCode = 1;
} finally {
  await browser?.close().catch(error => { report.browserCleanupError = String(error); process.exitCode = 1; });
  try {
    const stop = await fetch(`${origin}/__fixture/stop`, { method: 'POST', headers: { 'X-Fixture-Token': 'synthetic-session' }, redirect: 'error', signal: AbortSignal.timeout(3000) });
    report.fixtureStopped = stop.status === 204;
  } catch (error) { report.cleanupError = String(error); report.fixtureStopped = false; }
  if (!report.fixtureStopped || report.failures.length || report.error || report.browserCleanupError) process.exitCode = 1;
  report.finished = new Date().toISOString();
  if (ownsOutput) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  console.log(JSON.stringify({ observationComplete: report.observationComplete, fixtureStopped: report.fixtureStopped, cases: report.cases.map(({ id, passed, error, csv }) => ({ id, passed, error, csv })), failures: report.failures, blocked: report.blocked, error: report.error, cleanupError: report.cleanupError }, null, 2));
}
