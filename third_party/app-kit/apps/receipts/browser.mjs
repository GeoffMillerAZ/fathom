// Bounded browser evidence for the first-party synthetic receipts reference.
// RECEIPTS_BASE_URL=http://127.0.0.1:8766 RECEIPTS_EVIDENCE_DIR=/tmp/empty-dir node browser.mjs
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const base = new URL(process.env.RECEIPTS_BASE_URL ?? '');
if (base.protocol !== 'http:' || base.hostname !== '127.0.0.1' || !base.port || base.username || base.password || base.pathname !== '/' || base.search || base.hash) {
  throw new Error('RECEIPTS_BASE_URL must be an explicit bare http://127.0.0.1:PORT origin');
}
const out = fs.realpathSync(process.env.RECEIPTS_EVIDENCE_DIR ?? '');
if (!path.isAbsolute(out) || out === path.parse(out).root || fs.readdirSync(out).length) {
	throw new Error('RECEIPTS_EVIDENCE_DIR must be an explicit existing empty directory');
}

const playwrightEntry = fs.realpathSync('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs');
const playwrightRoot = fs.realpathSync('/tmp/appkit-browser-review.xBrDCp');
const entryRelative = path.relative(playwrightRoot, playwrightEntry);
if (!entryRelative || entryRelative === '..' || entryRelative.startsWith(`..${path.sep}`) || path.isAbsolute(entryRelative)) throw new Error('Playwright entry escaped its approved isolated directory');
const packagePath = fs.realpathSync(path.join(path.dirname(playwrightEntry), 'package.json'));
const packageBytes = fs.readFileSync(packagePath), pkg = JSON.parse(packageBytes);
if (path.basename(playwrightEntry) !== 'index.mjs' || pkg.name !== 'playwright' || pkg.version !== '1.63.0') throw new Error('Reviewed existing Playwright 1.63.0 is required');
const chrome = fs.realpathSync('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome');
const { chromium } = await import(pathToFileURL(playwrightEntry).href);
const sha256 = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const report = {
  schema: 1,
  scope: 'Runnable receipts browser task on first-party synthetic fixture data',
  origin: base.origin,
  started: new Date().toISOString(),
  inputs: {
    harness: { bytes: fs.statSync(new URL(import.meta.url)).size, sha256: sha256(fs.readFileSync(new URL(import.meta.url))) },
    playwright: { version: pkg.version, entry: playwrightEntry, entry_sha256: sha256(fs.readFileSync(playwrightEntry)), package_sha256: sha256(packageBytes) },
  },
  browser: {},
  budgets: { html_bytes: 32768, chart_target_css_pixels: 44, viewports_css_pixels: [390, 1440], chrome_default_zoom: 2 },
  checks: [], failures: [], requests: [], responses: [], blocked: [], browser_errors: [], expected_http_errors: [], screenshots: [], downloads: [], geometry: [], focus: [],
  limits: [
    'Synthetic three-product fixture only; no production records, providers, authentication, native host, screen reader, touch hardware, task-time, or consumer-adoption claim.',
    'The 256-character product-label case mutates only a fetched document in the browser harness; it proves compiled-CSS wrapping geometry, not a live runtime fixture or server input path.',
    'Chrome 200 percent zoom is selected through chrome://settings in a fresh temporary profile and must produce DPR 2 plus a 720 CSS-pixel layout viewport in the unchanged 1440-pixel window.',
  ],
};
const check = (name, ok, detail = null) => {
  report.checks.push({ name, ok, detail });
  if (!ok) report.failures.push({ name, detail });
};
const allowedPath = pathname => pathname === '/' || pathname === '/assets/app.css' || pathname === '/export.csv' || /^\/product\/(atlas|beacon|cedar)$/.test(pathname);
const canonical = (pathname = '/', values = {}) => {
  const selected = { period: '2024-09', channel: 'all', compare: 'previous', stale: 'false', theme: 'light', ...values };
  const query = new URLSearchParams();
  for (const key of ['period', 'channel', 'compare', 'stale', 'theme']) query.set(key, selected[key]);
  if (pathname === '/export.csv' && selected.product) query.set('product', selected.product);
  return `${base.origin}${pathname}?${query}`;
};

const browser = await chromium.launch({ executablePath: chrome, headless: true, args: ['--no-first-run', '--no-default-browser-check', '--disable-background-networking'] });
report.browser = { product: 'Google Chrome', version: browser.version(), executable: chrome, profile: 'fresh Playwright-managed temporary profile' };

async function makePage(width, theme, downloads = false) {
  const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme: theme, reducedMotion: 'reduce', acceptDownloads: downloads, serviceWorkers: 'block' });
  context.setDefaultTimeout(8000);
  context.setDefaultNavigationTimeout(8000);
  await context.route('**/*', route => {
    const request = route.request(), target = new URL(request.url());
    const allowed = target.origin === base.origin && allowedPath(target.pathname) && ['GET', 'HEAD'].includes(request.method());
    report.requests.push({ url: request.url(), method: request.method(), resource: request.resourceType(), allowed });
    if (allowed) return route.continue();
    report.blocked.push({ url: request.url(), method: request.method() });
    return route.abort('blockedbyclient');
  });
  await context.routeWebSocket('**/*', socket => {
    report.blocked.push({ websocket: socket.url() });
    socket.close();
  });
  const pending = [];
  const page = await context.newPage();
  page.on('pageerror', error => report.browser_errors.push({ kind: 'page', text: error.message }));
  page.on('console', message => {
    if (message.type() === 'error') report.browser_errors.push({ kind: 'console', text: message.text(), url: page.url() });
  });
  page.on('response', response => {
    if (response.headers()['content-disposition']?.startsWith('attachment;')) return;
    pending.push((async () => {
      const body = await response.body(), headers = response.headers();
      report.responses.push({ url: response.url(), status: response.status(), bytes: body.length, sha256: sha256(body), type: headers['content-type'], csp: headers['content-security-policy'] });
      if (headers['content-type']?.startsWith('text/html')) {
        check(`HTML payload within 32 KiB: ${new URL(response.url()).pathname}`, body.length <= report.budgets.html_bytes, body.length);
        check(`Hardened HTML response: ${new URL(response.url()).pathname}`, headers['x-content-type-options'] === 'nosniff' && headers['cache-control'] === 'no-store' && headers['referrer-policy'] === 'no-referrer' && headers['content-security-policy'] === "default-src 'none'; style-src 'self'; img-src data:; form-action 'self'; base-uri 'none'; frame-ancestors 'none'", headers);
      }
    })().catch(error => report.browser_errors.push({ kind: 'response', text: error.message, url: response.url() })));
  });
  return { context, page, pending };
}

async function screenshot(page, name) {
  const filename = `${name}.png`, target = path.join(out, filename);
  const bytes = await page.screenshot({ path: target, fullPage: true });
  report.screenshots.push({ file: filename, bytes: bytes.length, sha256: sha256(bytes) });
}

async function chromeViewportScreenshot(context, page, name) {
  const cdp = await context.newCDPSession(page);
  const captured = await cdp.send('Page.captureScreenshot', { format: 'png', fromSurface: true, captureBeyondViewport: false });
  const bytes = Buffer.from(captured.data, 'base64');
  const width = bytes.readUInt32BE(16), height = bytes.readUInt32BE(20);
  const controls = await page.evaluate(() => [...document.querySelectorAll('.receipts-theme, .receipts-filters label, .receipts-filters button')].filter(element => element.checkVisibility()).map(element => {
    const bounds = element.getBoundingClientRect();
    const control = element.matches('label') ? element.querySelector('select') : element;
    return { text: element.textContent.trim().replace(/\s+/g, ' ').slice(0, 90), value: control?.value ?? null, left: bounds.left, right: bounds.right, top: bounds.top, bottom: bounds.bottom };
  }));
  const filename = `${name}.png`, target = path.join(out, filename);
  fs.writeFileSync(target, bytes, { mode: 0o600 });
  report.screenshots.push({ file: filename, bytes: bytes.length, sha256: sha256(bytes), width, height, capture: 'CDP Page.captureScreenshot fromSurface=true captureBeyondViewport=false', controls });
  const expected = { Period: '2024-09', Channel: 'all', Comparison: 'previous', 'Fixture source age': 'false' };
  check('200 percent viewport capture maps every right-edge and selection control into 1440 physical pixels', width === 1440 && height === 1000 && controls.length === 6 && controls.every(control => control.left >= 0 && control.right <= 720 && control.top >= 0 && control.bottom <= 1000 && control.right * 2 <= width) && Object.entries(expected).every(([label, value]) => controls.some(control => control.text.startsWith(label) && control.value === value)) && controls.some(control => control.text === 'Use dark theme' && control.right > 580), { width, height, controls });
}

async function focusEvidence(page, name) {
  const detail = await page.evaluate(() => {
    const element = document.activeElement, style = getComputedStyle(element), bounds = element.getBoundingClientRect();
    return { tag: element.tagName, id: element.id, href: element.getAttribute('href'), text: element.textContent?.trim().slice(0, 90), focus_visible: element.matches(':focus-visible'), outline_style: style.outlineStyle, outline_width: parseFloat(style.outlineWidth), width: bounds.width, height: bounds.height };
  });
  report.focus.push({ name, ...detail });
  check(`${name} has visible focus`, detail.focus_visible && detail.outline_style !== 'none' && detail.outline_width >= 2, detail);
}

async function tabTo(page, selector, standardFocus = true) {
  for (let count = 0; count < 100; count++) {
    if (await page.evaluate(value => document.activeElement?.matches(value), selector)) return standardFocus ? focusEvidence(page, selector) : undefined;
    await page.keyboard.press('Tab');
  }
  throw new Error(`Keyboard traversal did not reach ${selector}`);
}

async function inspectPage(page, name) {
  const geometry = await page.evaluate(() => {
    const chartTargets = [...document.querySelectorAll('.receipts-chart-product-link, .receipts-chart-svg a')].filter(element => element.checkVisibility()).map(element => {
      const bounds = element.getBoundingClientRect();
      return { product: element.closest('[data-product]')?.getAttribute('data-product'), series: element.getAttribute('data-series') || element.closest('[data-series]')?.getAttribute('data-series'), width: bounds.width, height: bounds.height };
    });
    const table = document.querySelector('.receipts-table-scroll');
    return {
      viewport: document.documentElement.clientWidth,
      scroll_width: document.documentElement.scrollWidth,
      table_client_width: table?.clientWidth,
      table_scroll_width: table?.scrollWidth,
      scripts: document.scripts.length,
      workers: navigator.serviceWorker?.controller ? 1 : 0,
      chartTargets,
    };
  });
  report.geometry.push({ name, ...geometry });
  check(`${name}: no document horizontal overflow`, geometry.scroll_width <= geometry.viewport + 1, geometry);
  check(`${name}: table owns any narrow overflow`, geometry.table_scroll_width >= geometry.table_client_width, geometry);
  check(`${name}: no scripts or service worker`, geometry.scripts === 0 && geometry.workers === 0, geometry);
  for (const target of geometry.chartTargets) check(`${name}: chart target is at least 44px`, target.width >= 44 && target.height >= 44, target);
}

async function assertDefaultOracle(page, name) {
  const evidence = await page.evaluate(() => {
    const rows = [...document.querySelectorAll('#receipts-values tbody tr')].map(row => ({
      product: row.dataset.product,
      series: row.dataset.series,
      values: Object.fromEntries([...row.querySelectorAll('td[data-measure]')].map(cell => [cell.dataset.measure, cell.dataset.state === 'missing' ? null : Number(cell.dataset.cents)])),
    }));
    const geometry = product => {
      const bar = document.querySelector(`.receipts-chart-bar[data-product="${product}"][data-series="current"]`);
      const axis = bar?.closest('svg')?.querySelector('.receipts-chart-axis-line');
      if (!bar || !axis) return null;
      const b = bar.getBoundingClientRect(), a = axis.getBoundingClientRect();
      return { left: b.left, right: b.right, width: b.width, axis: a.left };
    };
    return { rows, atlas: geometry('atlas'), cedar: geometry('cedar'), decision: document.querySelector('#decision-heading')?.textContent.trim() };
  });
  const expected = [
    ['atlas', 'current', 100000, 20000, 80000], ['beacon', 'current', 95000, 5000, 90000], ['cedar', 'current', 10000, 25000, -15000],
    ['atlas', 'previous', 150000, 10000, 140000], ['beacon', 'previous', 90000, 0, 90000], ['cedar', 'previous', 20000, 20000, 0],
  ].map(([product, series, receipts, refunds, net]) => ({ product, series, values: { receipts, refunds, net } }));
  check(`${name}: current and previous literal table oracle`, JSON.stringify(evidence.rows) === JSON.stringify(expected), evidence.rows);
  check(`${name}: decision explains exact September change`, evidence.decision === 'Net receipts fell $750.00', evidence.decision);
  const ratio = evidence.atlas?.width / evidence.cedar?.width;
  check(`${name}: signed bars use proportional actual geometry around one zero`, evidence.atlas?.width > 0 && evidence.atlas.left >= evidence.atlas.axis - 1 && evidence.cedar?.width > 0 && evidence.cedar.right <= evidence.cedar.axis + 1 && evidence.cedar.left < evidence.cedar.axis && Math.abs(evidence.atlas.axis - evidence.cedar.axis) <= 1 && Math.abs(ratio - 80000 / 15000) < 0.02, { ...evidence, width_ratio: ratio });
}

function parseCSV(text) {
  const records = []; let row = [], field = '', quoted = false;
  for (let index = 0; index < text.length; index++) {
    const character = text[index];
    if (quoted) {
      if (character === '"' && text[index + 1] === '"') { field += '"'; index++; }
      else if (character === '"') quoted = false;
      else field += character;
    } else if (character === '"') quoted = true;
    else if (character === ',') { row.push(field); field = ''; }
    else if (character === '\n') { row.push(field); records.push(row); row = []; field = ''; }
    else if (character !== '\r') field += character;
  }
  if (quoted) throw new Error('CSV contains an unclosed quoted field');
  if (field || row.length) { row.push(field); records.push(row); }
  const [header, ...values] = records;
  const expectedHeader = [
    'series', 'measure', 'selection_period', 'selection_compare', 'selection_channel', 'selection_stale', 'selection_product', 'product_channel',
    'context_chart_id', 'context_title', 'context_unit', 'context_grain', 'context_aggregation', 'context_window_kind', 'context_window',
    'context_source_kind', 'context_source', 'context_source_href', 'context_source_draft', 'context_evaluated_at', 'context_freshness',
    'context_completeness', 'context_coverage_note', 'context_uncertainty', 'context_decimal_places',
    'id', 'label', 'value', 'unit', 'state', 'missing_reason', 'lower', 'upper', 'role', 'role_label',
  ];
  if (!header || JSON.stringify(header) !== JSON.stringify(expectedHeader) || new Set(header).size !== 35 || values.some(value => value.length !== 35)) throw new Error('CSV is not the exact rectangular unique 35-column export');
  return values.map(value => Object.fromEntries(header.map((key, index) => [key, value[index]])));
}

async function exerciseJourney() {
  const { context, page, pending } = await makePage(1440, 'light', true);
  try {
    await page.goto(canonical('/', { theme: 'light' }), { waitUntil: 'load' });
    await assertDefaultOracle(page, 'journey');

    await page.keyboard.press('Tab');
    check('Skip link is first keyboard stop', await page.evaluate(() => document.activeElement?.getAttribute('href') === '#receipts-main'), await page.evaluate(() => document.activeElement?.outerHTML));
    await focusEvidence(page, 'skip link');
    await page.keyboard.press('Enter');
    check('Skip link moves focus to main', await page.evaluate(() => document.activeElement?.id === 'receipts-main'), await page.evaluate(() => document.activeElement?.id));

    await page.locator('.receipts-chart-product-link').filter({ hasText: 'Atlas' }).click();
    await page.waitForURL(url => url.pathname === '/product/atlas');
    check('Pointer opens exact Atlas detail with product-scoped total', new URL(page.url()).searchParams.get('compare') === 'previous' && await page.locator('[data-selection-product="atlas"]').count() === 1 && (await page.locator('#decision-heading').innerText()).startsWith('Product net receipts') && await page.locator('[data-total="current"]').innerText() === '$800.00', { url: page.url(), heading: await page.locator('#decision-heading').innerText(), total: await page.locator('[data-total="current"]').innerText() });
    await tabTo(page, '.receipts-back');
    await page.keyboard.press('Enter');
    await page.waitForURL(url => url.pathname === '/');
    check('Keyboard Back restores overview selection', new URL(page.url()).searchParams.get('compare') === 'previous' && await page.locator('[data-selection-product=""]').count() === 1, page.url());

    await tabTo(page, '#receipts-values tr[data-product="cedar"][data-series="current"] a');
    await page.keyboard.press('Enter');
    await page.waitForURL(url => url.pathname === '/product/cedar');
    check('Keyboard opens exact Cedar detail', await page.locator('[data-selection-product="cedar"]').count() === 1, page.url());
    await page.locator('.receipts-back').click();
    await page.waitForURL(url => url.pathname === '/');

    const cedarSVG = '.receipts-chart-svg a[data-product="cedar"][data-series="current"]';
    await tabTo(page, cedarSVG, false);
    const svgFocus = await page.locator(cedarSVG).evaluate(element => {
      const hit = element.querySelector('.receipts-chart-hit'), style = getComputedStyle(hit), bounds = element.getBoundingClientRect();
      return { active: document.activeElement === element, focus_visible: element.matches(':focus-visible'), width: bounds.width, height: bounds.height, hit_stroke: style.stroke, hit_stroke_width: parseFloat(style.strokeWidth), vector_effect: style.vectorEffect };
    });
    report.focus.push({ name: 'negative Cedar SVG hit target', ...svgFocus });
    check('Negative Cedar SVG target has visible non-scaling focus stroke', svgFocus.active && svgFocus.focus_visible && svgFocus.width >= 44 && svgFocus.height >= 44 && svgFocus.hit_stroke !== 'none' && svgFocus.hit_stroke_width === 2 && svgFocus.vector_effect === 'non-scaling-stroke', svgFocus);
    const svgDetail = page.waitForURL(url => url.pathname === '/product/cedar');
    await page.keyboard.press('Enter');
    await svgDetail;
    await tabTo(page, '.receipts-back');
    const svgBack = page.waitForURL(url => url.pathname === '/');
    await page.keyboard.press('Enter');
    await svgBack;
    check('SVG keyboard detail and native Back preserve selection', new URL(page.url()).searchParams.get('compare') === 'previous' && new URL(page.url()).searchParams.get('theme') === 'light', page.url());

    await page.locator('.receipts-theme').click();
    await page.waitForURL(url => url.searchParams.get('theme') === 'dark');
    check('Theme navigation preserves full selection', await page.locator('html[data-theme="dark"]').count() === 1 && ['period', 'channel', 'compare', 'stale'].every(key => new URL(page.url()).searchParams.has(key)), page.url());

    for (const [selector, key, expected] of [['select[name="period"]', 'n', '2024-11'], ['select[name="channel"]', 'm', 'mobile'], ['select[name="compare"]', 'n', 'none'], ['select[name="stale"]', '4', 'true']]) {
      await page.locator(selector).focus();
      await page.keyboard.press(key);
      check(`Native keyboard changes ${selector}`, await page.locator(selector).inputValue() === expected, await page.locator(selector).inputValue());
    }
    await tabTo(page, 'button[type="submit"]');
    const filterNavigation = page.waitForURL(url => url.searchParams.get('period') === '2024-11' && url.searchParams.get('channel') === 'mobile' && url.searchParams.get('compare') === 'none' && url.searchParams.get('stale') === 'true' && url.searchParams.get('theme') === 'dark');
    await page.keyboard.press('Enter');
    await filterNavigation;
    const filtered = await page.evaluate(() => ({ rows: [...document.querySelectorAll('#receipts-values tbody tr')].map(row => `${row.dataset.series}/${row.dataset.product}`), heading: document.querySelector('#decision-heading')?.textContent.trim() }));
    check('Key-only native filters submit exact partial selection', JSON.stringify(filtered.rows) === JSON.stringify(['current/beacon']) && filtered.heading === 'The monthly total is not yet known' && (await page.locator('#receipts-values').innerText()).includes('November 2024 (partial, before 21 Nov 00:00 UTC)'), filtered);

    await page.goto(canonical('/', { period: '2024-10', channel: 'mobile', compare: 'none', stale: 'false', theme: 'dark' }), { waitUntil: 'load' });
    check('All-missing selection is Not known, never zero', await page.locator('[data-total="current"]').innerText() === 'Not known' && (await page.locator('#decision-heading').innerText()).includes('not yet known'), { total: await page.locator('[data-total="current"]').innerText(), heading: await page.locator('#decision-heading').innerText() });

    await page.goto(canonical('/', { period: '2024-10', channel: 'all', compare: 'previous', stale: 'true', theme: 'dark' }), { waitUntil: 'load' });
    check('Stale selection is disclosed for both periods', await page.locator('.receipts-period-context[data-freshness="stale"]').count() === 2, await page.locator('.receipts-period-context').evaluateAll(elements => elements.map(element => element.dataset.freshness)));
    await tabTo(page, '.receipts-export');
    const downloadEvent = page.waitForEvent('download');
    await page.keyboard.press('Enter');
    const download = await downloadEvent, downloadPath = path.join(out, 'october-all-previous-stale.csv');
    await download.saveAs(downloadPath);
    const bytes = fs.readFileSync(downloadPath), rows = parseCSV(bytes.toString());
    const values = Object.fromEntries(rows.map(row => [`${row.series}/${row.id}/${row.measure}`, { value: row.value, state: row.state, reason: row.missing_reason }]));
    const literal = {
      'current/atlas/receipts': ['0', 'present'], 'current/atlas/refunds': ['0', 'present'], 'current/atlas/net': ['0', 'present'],
      'current/beacon/receipts': ['', 'missing'], 'current/beacon/refunds': ['', 'missing'], 'current/beacon/net': ['', 'missing'],
      'current/cedar/receipts': ['50', 'present'], 'current/cedar/refunds': ['', 'missing'], 'current/cedar/net': ['', 'missing'],
      'previous/atlas/receipts': ['1000', 'present'], 'previous/atlas/refunds': ['200', 'present'], 'previous/atlas/net': ['800', 'present'],
      'previous/beacon/receipts': ['950', 'present'], 'previous/beacon/refunds': ['50', 'present'], 'previous/beacon/net': ['900', 'present'],
      'previous/cedar/receipts': ['100', 'present'], 'previous/cedar/refunds': ['250', 'present'], 'previous/cedar/net': ['-150', 'present'],
    };
    // The fixture models leave Row.Role empty and declare no Legend: the shared
    // contract therefore exports the documented default role and no guessed label.
    const expectedRole = { role: 'accent', role_label: '' };
    check('Native CSV download has all literal current/previous values, missingness, and role meaning', rows.length === 18 && Object.entries(literal).every(([key, [value, state]]) => values[key]?.value === value && values[key]?.state === state) && rows.every(row => row.selection_period === '2024-10' && row.selection_compare === 'previous' && row.selection_channel === 'all' && row.selection_stale === 'true' && row.context_freshness.startsWith('Stale:') && row.role === expectedRole.role && row.role_label === expectedRole.role_label), { rows: rows.length, values, expectedRole });
    report.downloads.push({ file: path.basename(downloadPath), suggested_filename: download.suggestedFilename(), bytes: bytes.length, sha256: sha256(bytes) });

    const badResponse = await page.goto(`${base.origin}/?unsafe=%3Cscript%3E`, { waitUntil: 'load' });
    report.expected_http_errors.push({ url: page.url(), status: badResponse?.status() });
    check('Unknown query returns safe recovery without reflection', badResponse?.status() === 400 && !((await page.content()).includes('<script>')) && await page.locator('a[href="/"]').count() === 1, { status: badResponse?.status() });
    await page.locator('a[href="/"]').click();
    await page.waitForURL(url => url.pathname === '/' && !url.search);
    check('Recovery returns immutable defaults without leaked state', await page.locator('select[name="period"]').inputValue() === '2024-09' && await page.locator('select[name="channel"]').inputValue() === 'all' && await page.locator('select[name="compare"]').inputValue() === 'previous' && await page.locator('html[data-theme="light"]').count() === 1 && await page.evaluate(() => !localStorage.length && !sessionStorage.length), page.url());

    await tabTo(page, '#receipts-method summary');
    await page.keyboard.press('Enter');
    const methodText = await page.locator('#receipts-method').innerText();
    check('Keyboard opens analytical explanation', await page.locator('#receipts-method').evaluate(element => element.open) && methodText.includes('Net receipts equal receipts minus refunds') && methodText.includes('not a lower bound'), methodText);
    check('No cookies or service workers retain selection', (await context.cookies()).length === 0 && context.serviceWorkers().length === 0, { cookies: await context.cookies(), workers: context.serviceWorkers().length });
  } finally {
    await Promise.all(pending);
    await context.close();
  }
}

async function exerciseSnapshots() {
  for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
    const { context, page, pending } = await makePage(width, theme);
    const name = `${width}-${theme}`;
    try {
      await page.goto(canonical('/', { theme }), { waitUntil: 'load' });
      await inspectPage(page, name);
      await assertDefaultOracle(page, name);
      if (width === 390) {
        check(`${name}: desktop sidebar hidden and mobile Menu visible`, !(await page.locator('aside').isVisible()) && await page.locator('details > summary').filter({ hasText: 'Menu' }).isVisible(), null);
        await tabTo(page, 'details > summary');
        await page.keyboard.press('Enter');
        check(`${name}: native mobile Menu opens`, await page.locator('details').first().evaluate(element => element.open), null);
      } else {
        check(`${name}: desktop sidebar visible and mobile Menu hidden`, await page.locator('aside').isVisible() && !(await page.locator('details > summary').filter({ hasText: 'Menu' }).isVisible()), null);
      }
      await screenshot(page, name);
    } finally {
      await Promise.all(pending);
      await context.close();
    }
  }
}

async function exerciseZoom() {
  const profile = fs.mkdtempSync(path.join(fs.realpathSync(os.tmpdir()), 'receipts-chrome-profile-'));
  const context = await chromium.launchPersistentContext(profile, { executablePath: chrome, headless: true, viewport: { width: 1440, height: 1000 }, colorScheme: 'light', reducedMotion: 'reduce', serviceWorkers: 'block', args: ['--no-first-run', '--no-default-browser-check', '--disable-background-networking'] });
  const page = context.pages()[0] ?? await context.newPage();
  try {
    await page.goto('chrome://settings/appearance');
    await page.locator('#zoomLevel').selectOption('2');
    check('Fresh Chrome profile records 200 percent default zoom', await page.locator('#zoomLevel').inputValue() === '2', await page.locator('#zoomLevel').inputValue());
    await context.route('**/*', route => {
      const request = route.request(), target = new URL(request.url());
      const allowed = target.origin === base.origin && allowedPath(target.pathname) && ['GET', 'HEAD'].includes(request.method());
      report.requests.push({ url: request.url(), method: request.method(), resource: request.resourceType(), allowed, phase: 'zoom' });
      if (allowed) return route.continue();
      report.blocked.push({ url: request.url(), method: request.method(), phase: 'zoom' });
      return route.abort('blockedbyclient');
    });
    await page.goto(canonical('/', { theme: 'light' }), { waitUntil: 'load' });
    const zoom = await page.evaluate(() => ({ dpr: devicePixelRatio, inner_width: innerWidth, outer_width: outerWidth, layout_width: document.documentElement.clientWidth, visual_width: visualViewport.width, visual_scale: visualViewport.scale, scroll_width: document.documentElement.scrollWidth }));
    check('Chrome browser zoom is exactly 200 percent', zoom.dpr === 2 && zoom.inner_width === 720 && zoom.outer_width === 1440 && zoom.layout_width === 720 && zoom.visual_width === 720 && zoom.visual_scale === 1, zoom);
    check('200 percent browser zoom reflows without document overflow', zoom.scroll_width <= zoom.layout_width + 1, zoom);
    const zoomDetail = page.locator('.receipts-chart-product-link').filter({ hasText: 'Atlas' });
    await zoomDetail.focus();
    await focusEvidence(page, '200 percent zoom Atlas detail');
    const detailNavigation = page.waitForURL(url => url.pathname === '/product/atlas');
    await page.keyboard.press('Enter');
    await detailNavigation;
    const zoomBack = page.locator('.receipts-back');
    await zoomBack.focus();
    await focusEvidence(page, '200 percent zoom Back');
    const backNavigation = page.waitForURL(url => url.pathname === '/');
    await page.keyboard.press('Enter');
    await backNavigation;
    const zoomAfterTask = await page.evaluate(() => ({ dpr: devicePixelRatio, inner_width: innerWidth, outer_width: outerWidth }));
    check('Native detail and Back retain 200 percent browser zoom', zoomAfterTask.dpr === 2 && zoomAfterTask.inner_width === 720 && zoomAfterTask.outer_width === 1440, zoomAfterTask);
    await chromeViewportScreenshot(context, page, '1440-light-zoom-200-viewport');
  } finally {
    await context.close();
    fs.rmSync(profile, { recursive: true, force: true });
  }
}

async function exerciseLongLabel() {
  const { context, page, pending } = await makePage(390, 'dark');
  try {
    await page.goto(canonical('/', { theme: 'dark' }), { waitUntil: 'load' });
    await page.locator('details > summary').filter({ hasText: 'Menu' }).click();
    const stress = await page.evaluate(() => {
      const label = ('Synthetic product ' + 'long-label '.repeat(40)).slice(0, 256);
      const selectors = [
        '.receipts-chart-product-link',
        '#receipts-values tr[data-product="atlas"][data-series="current"] th a',
        'nav[aria-label="Primary (mobile)"] a[href^="/product/atlas"] span:last-child',
      ];
      const elements = selectors.map(selector => document.querySelector(selector));
      for (const element of elements) if (element) element.textContent = label;
      const geometry = elements.map((element, index) => {
        if (!element) return { selector: selectors[index], missing: true };
        const bounds = element.getBoundingClientRect(), style = getComputedStyle(element), lineHeight = parseFloat(style.lineHeight);
        return { selector: selectors[index], text_length: element.textContent.length, width: bounds.width, height: bounds.height, client_width: element.clientWidth, scroll_width: element.scrollWidth, line_height: lineHeight, overflow_wrap: style.overflowWrap, visible: element.checkVisibility() };
      });
      return { label_length: label.length, document_client_width: document.documentElement.clientWidth, document_scroll_width: document.documentElement.scrollWidth, geometry };
    });
    check('Synthetic 256-character labels render and wrap without clipping', stress.label_length === 256 && stress.document_scroll_width <= stress.document_client_width + 1 && stress.geometry.every(item => !item.missing && item.visible && item.text_length === 256 && item.scroll_width <= item.client_width + 1 && item.height >= item.line_height * 2 && item.overflow_wrap === 'anywhere'), stress);
    await screenshot(page, '390-dark-long-label-static');
  } finally {
    await Promise.all(pending);
    await context.close();
  }
}

try {
  await exerciseSnapshots();
  await exerciseJourney();
  await exerciseLongLabel();
  await exerciseZoom();
} catch (error) {
  report.failures.push({ name: 'harness completed', detail: error.stack });
} finally {
  const unrelatedConsole = report.browser_errors.filter(error => !(error.kind === 'console' && error.url.includes('unsafe=%3Cscript%3E') && error.text.includes('400')));
  check('No unexpected browser errors', unrelatedConsole.length === 0, report.browser_errors);
  check('No blocked outside, write, or WebSocket request', report.blocked.length === 0, report.blocked);
  await browser.close();
  report.finished = new Date().toISOString();
  report.totals = { checks: report.checks.length, passed: report.checks.filter(item => item.ok).length, failed: report.failures.length, requests: report.requests.length, responses: report.responses.length, screenshots: report.screenshots.length, downloads: report.downloads.length };
  const reportPath = path.join(out, 'report.json');
  fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`, { mode: 0o600 });
  console.log(JSON.stringify({ evidence: out, report: reportPath, report_sha256: sha256(fs.readFileSync(reportPath)), ...report.totals }));
}
process.exitCode = report.failures.length ? 1 : 0;
