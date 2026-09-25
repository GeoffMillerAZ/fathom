// Optional local browser evidence, never a production dependency or certification.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import { pathToFileURL } from 'node:url';
import { verifyCapturePNG } from '../catalyst-settings-adapter/zoom-capture.mjs';
import { zoomCaptureClip, prepareContext } from './zoom-contract.mjs';

const origin = new URL(process.argv[2] ?? '');
if (origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || !origin.port || origin.username || origin.password || origin.pathname !== '/' || origin.search || origin.hash) throw Error('Explicit bare HTTP 127.0.0.1 origin required');
if (process.argv.length > 4 || (process.argv[3] && process.argv[3] !== '--zoom=200')) throw Error('Only the optional --zoom=200 mode is supported');
const zoom = process.argv[3] === '--zoom=200';
const entry = process.env.APPKIT_PLAYWRIGHT_ENTRY ?? '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
const sha = b => crypto.createHash('sha256').update(b).digest('hex');
const resolvedEntry = fs.realpathSync(entry);
const packagePath = path.join(path.dirname(resolvedEntry), 'package.json');
const packageBytes = fs.readFileSync(packagePath), pkg = JSON.parse(packageBytes);
if (path.basename(resolvedEntry) !== 'index.mjs' || pkg.name !== 'playwright' || pkg.version !== '1.63.0') throw Error('Reviewed existing Playwright 1.63.0 entry required; never auto-install or silently repin');
const playwright = { entry: resolvedEntry, entrySHA256: sha(fs.readFileSync(resolvedEntry)), packagePath, packageSHA256: sha(packageBytes), version: pkg.version };
const comparisonSelection = Object.freeze({ period: '2026-09', compare: 'previous', category: 'api' });
const expectedCSVHeader = Object.freeze([
  'series', 'selection_period', 'selection_compare', 'selection_category', 'selection_state', 'selection_service',
  ...['chart_id', 'title', 'unit', 'grain', 'aggregation', 'window_kind', 'window', 'source_kind', 'source', 'source_href', 'source_draft', 'evaluated_at', 'freshness', 'completeness', 'coverage_note', 'uncertainty', 'decimal_places'].map(key => `context_${key}`),
  'id', 'label', 'value', 'unit', 'state', 'missing_reason', 'lower', 'upper', 'role', 'role_label',
]);
const comparisonOracle = Object.freeze({
  complete: Object.freeze({
    key: 'c', current: ['4', '1'], currentDisplay: ['4', '1'], previous: ['6', '3'], changes: ['2 fewer', '2 fewer'], rowState: 'Present', completeness: 'complete',
    narrative: 'Synthetic fixture for September 2026, API services (2 services). The selected full-month counts total 5 failed operations. The previous calendar month, August 2026, totals 9 for the same services; the selected count is 4 lower. Counts are not normalized for traffic or month length and do not establish a reliability rate or cause.',
    detail: 'Synthetic fixture. Failed incoming request attempts at the gateway; retried attempts count separately. For September 2026, the full-month failed-operation count is 4. For August 2026, the full-month failed-operation count is 6. These counts do not establish a reliability rate or the cause of a change.',
    stateLabel: 'Complete reporting window', stateDescription: 'The fixture contains counts for the whole selected calendar month. A recorded zero means no failed operations in that reporting window.',
    freshness: 'Fresh: source observed 2026-10-05T11:00:00Z; maximum age 24h0m0s', chart: 'paired',
  }),
  missing: Object.freeze({
    key: 'm', current: ['', ''], currentDisplay: ['Missing', 'Missing'], previous: ['6', '3'], changes: ['Not comparable', 'Not comparable'], rowState: 'Missing, not zero', completeness: 'unknown',
    missingReasons: ['No source report for Gateway API in 2026-09; missing is not zero.', 'No source report for Search API in 2026-09; missing is not zero.'],
    narrative: 'Synthetic fixture for September 2026, API services (2 services). All selected counts are missing, not zero. No current total or change can be calculated. Obtain the missing source reports before making a comparison.',
    detail: 'Synthetic fixture. Failed incoming request attempts at the gateway; retried attempts count separately. For September 2026, the count is missing, not zero. For August 2026, the full-month failed-operation count is 6. These counts do not establish a reliability rate or the cause of a change.',
    stateLabel: 'Source reports missing', stateDescription: 'Current counts are unknown. There is no current total, zero-value substitution or calculated change. Obtain the source reports before comparing.',
    freshness: 'Source freshness unknown: The missing fixture has no current-period source observation.', chart: 'absent',
  }),
  stale: Object.freeze({
    key: 's', current: ['4', '1'], currentDisplay: ['4', '1'], previous: ['6', '3'], changes: ['2 fewer', '2 fewer'], rowState: 'Present, source stale', completeness: 'complete',
    narrative: 'Synthetic fixture for September 2026, API services (2 services). The selected full-month counts total 5 failed operations. The previous calendar month, August 2026, totals 9 for the same services; the selected count is 4 lower. Counts are not normalized for traffic or month length and do not establish a reliability rate or cause. Source data is stale at the fixed fixture evaluation time; refresh the source before acting.',
    detail: 'Synthetic fixture. Failed incoming request attempts at the gateway; retried attempts count separately. For September 2026, the full-month failed-operation count is 4. Its source observation is stale at the fixture evaluation time. For August 2026, the full-month failed-operation count is 6. These counts do not establish a reliability rate or the cause of a change.',
    stateLabel: 'Source observation is stale', stateDescription: 'The recorded counts are still visible, but the source is older than its 24-hour freshness budget at the fixed fixture clock. Refresh the source before acting.',
    freshness: 'Stale: source observed 2026-10-02T12:00:00Z; maximum age 24h0m0s', chart: 'paired',
  }),
  partial: Object.freeze({
    key: 'p', current: ['3', '1'], currentDisplay: ['At least 3', 'At least 1'], previous: ['6', '3'], changes: ['Not comparable', 'Not comparable'], rowState: 'Partial, lower bound', completeness: 'partial',
    narrative: 'Synthetic fixture for September 2026, API services (2 services). The first 20 days contain 4 observed failed operations; this is only a lower bound for the full month. No complete-period change is reported because later days are unavailable.',
    detail: 'Synthetic fixture. Failed incoming request attempts at the gateway; retried attempts count separately. For September 2026, the first-20-day failed-operation count is 3; this is a lower bound for the full month. For August 2026, the full-month failed-operation count is 6. These counts do not establish a reliability rate or the cause of a change.',
    stateLabel: 'Partial month: first 20 days only', stateDescription: 'Present counts are lower bounds. Later days are unobserved. Previous full-month values remain available in the table and export, but no full-month difference is calculated or plotted.',
    freshness: 'Fresh: source observed 2026-10-05T11:00:00Z; maximum age 24h0m0s', chart: 'current-only',
  }),
});
const { chromium } = await import(pathToFileURL(resolvedEntry).href);
const out = fs.mkdtempSync(path.join(os.tmpdir(), 'appkit-analytics-quality-'));
const report = {
  schema: 1, origin: origin.origin, started: new Date().toISOString(), playwright,
  harnessSHA256: sha(fs.readFileSync(new URL(import.meta.url))),
  hardware: { platform: os.platform(), arch: os.arch(), cpu: os.cpus()[0]?.model, logicalCPUs: os.cpus().length, memoryBytes: os.totalmem() },
  budgets: { htmlBytes: 32768, touchTargetCSSPixels: 44, normalTextContrast: 4.5, largeTextContrast: 3, localDOMContentLoadedP95Ms: 250 },
  zoom: zoom ? {percent: 200, method: 'Chrome Appearance setting', outerWidth: 1440, expectedCSSWidth: 720} : null,
  captureHelperSHA256: sha(fs.readFileSync(new URL('../catalyst-settings-adapter/zoom-capture.mjs', import.meta.url))),
  zoomContractSHA256: sha(fs.readFileSync(new URL('./zoom-contract.mjs', import.meta.url))),
  passes: [], failures: [], requests: [], blocked: [], errors: [], keys: [], geometry: [], contrasts: [], timings: [], artifacts: [], profiles: [],
  comparisonOracle: { selection: comparisonSelection, states: comparisonOracle },
  comparisonStates: Object.keys(comparisonOracle).map(state => ({ state, status: 'not-run', phases: { selection: 'not-run', overview: 'not-run', overviewCSV: 'not-run', detail: 'not-run', detailCSV: 'not-run', returned: 'not-run', cleanup: 'not-run' }, failures: [] })),
  limits: ['Fixed three-service synthetic workload on loopback, not production scale or human task-time/error.', 'Keyboard uses native type-ahead, not native popup-arrow interaction.', zoom ? 'Only Chrome 200% at 1440 outer pixels; no screen-reader, touch-device or full accessibility certification.' : 'No browser zoom, screen-reader, touch-device or full accessibility certification.', 'Contrast samples opaque text and ancestor backgrounds; no claim for gradients, overlapping elements, or antialiased pixels.', 'DOM timing and response bodies are local observations, not INP, network-throttled or real-device performance.'],
};
function check(ok, name, detail) { report[ok ? 'passes' : 'failures'].push({ name, detail }); }
const allowed = new Set(['/analytics', '/analytics/gateway-api', '/analytics/search-api', '/analytics/report-worker', '/analytics/export.csv', '/analytics/gateway-api/export.csv', '/static/app.css']);
const launch = { executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, args: ['--no-first-run', '--no-default-browser-check'] };
const browser = zoom ? null : await chromium.launch(launch);
report.browser = browser?.version();
async function makePage(width, theme) {
  const options = { viewport: { width, height: 1000 }, colorScheme: theme, reducedMotion: 'reduce', acceptDownloads: true, serviceWorkers: 'block' };
  let context;
  if (zoom) {
    const profile = fs.mkdtempSync(path.join(out, 'chrome-profile-')); fs.chmodSync(profile, 0o700);
    report.profiles.push({path: path.basename(profile), disposition: 'retained synthetic-only profile'});
    context = await chromium.launchPersistentContext(profile, {...launch, ...options});
  } else context = await browser.newContext(options);
  return prepareContext(context, async () => {
    if (zoom) {
      report.browser = context.browser().version();
      const settings = context.pages()[0];
      settings.setDefaultTimeout(6000);
      await settings.goto('chrome://settings/appearance');
      await settings.locator('#zoomLevel').selectOption('2');
      if (await settings.locator('#zoomLevel').inputValue() !== '2') throw Error('Chrome did not accept 200% zoom');
      check(true, 'Chrome Appearance setting selects actual 200%');
      // Leave WebUI before collecting application errors or disabling scripts.
      await settings.goto('about:blank');
    }
  context.setDefaultTimeout(6000);
  await context.route('**/*', async route => {
    const r = route.request(), u = new URL(r.url());
    if (u.origin === origin.origin && allowed.has(u.pathname) && ['GET', 'HEAD'].includes(r.method())) return route.continue();
    report.blocked.push({ url: r.url(), method: r.method() }); return route.abort();
  });
  await context.routeWebSocket('**/*', ws => { report.blocked.push({ websocket: ws.url() }); ws.close(); });
  const pending = [];
  const page = zoom ? context.pages()[0] : await context.newPage();
  if (zoom) await (await context.newCDPSession(page)).send('Emulation.setScriptExecutionDisabled', {value: true});
  page.on('pageerror', e => report.errors.push({ kind: 'page', text: e.message }));
  page.on('console', m => { if (m.type() === 'error') report.errors.push({ kind: 'console', text: m.text() }); });
  page.on('response', response => {
    if (response.headers()['content-disposition']?.includes('attachment')) return;
    pending.push((async () => {
      const b = await response.body();
      report.requests.push({ url: response.url(), status: response.status(), bytes: b.length, sha256: sha(b), type: response.headers()['content-type'] });
      check(response.status() === 200, 'Allowed response returned 200', { url: response.url(), status: response.status() });
      if (response.headers()['content-type']?.startsWith('text/html')) check(b.length <= report.budgets.htmlBytes, 'HTML payload bound', { url: response.url(), bytes: b.length });
    })().catch(e => report.errors.push({ kind: 'response', text: e.message })));
  });
  return { context, page, pending };
  });
}
async function key(page, value) {
  await page.keyboard.press(value);
  report.keys.push({ key: value, active: await page.evaluate(() => {
    const e = document.activeElement;
    return { tag: e.tagName, name: e.getAttribute('name'), id: e.id, text: e.textContent?.trim().slice(0, 90), value: e.value, href: e.getAttribute('href') };
  }) });
}
async function tabTo(page, selector, backwards = false) {
  for (let i = 0; i < 60; i++) {
    if (await page.evaluate(s => document.activeElement?.matches(s), selector)) {
      const focus = await page.evaluate(() => {
        const e = document.activeElement, s = getComputedStyle(e), r = e.getBoundingClientRect();
        return { outline: s.outlineStyle, width: parseFloat(s.outlineWidth), color: s.outlineColor, focusVisible: e.matches(':focus-visible'), top: r.top, bottom: r.bottom, viewport: innerHeight };
      });
      check(focus.focusVisible && focus.outline !== 'none' && focus.width > 0, 'Visible keyboard focus: ' + selector, focus);
      return;
    }
    await key(page, backwards ? 'Shift+Tab' : 'Tab');
  }
  throw Error('Keyboard traversal did not reach ' + selector);
}
async function skip(page) {
  await tabTo(page, 'a[href="#gallery-main"]');
  await key(page, 'Enter');
  check(await page.evaluate(() => document.activeElement?.id === 'gallery-main'), 'Skip link moves actual focus', page.url());
}
async function screenshot(page, name) {
  const target = path.join(out, name + '.png');
  if (zoom) {
    const cdp = await page.context().newCDPSession(page);
    try {
      const metrics = await cdp.send('Page.getLayoutMetrics');
      const clip = zoomCaptureClip(metrics);
      const shot = await cdp.send('Page.captureScreenshot', {format: 'png', fromSurface: true, captureBeyondViewport: true, clip});
      const bytes = Buffer.from(shot.data, 'base64');
      const png = verifyCapturePNG(bytes, {clip});
      fs.writeFileSync(target, bytes, {flag: 'wx', mode: 0o600});
      check(true, name + ': complete CRC-checked zoom screenshot', {metrics, png});
    } finally { await cdp.detach(); }
  } else await page.screenshot({ path: target, fullPage: true });
  const b = fs.readFileSync(target); report.artifacts.push({ path: path.basename(target), bytes: b.length, sha256: sha(b) });
}
function csvRecords(text, expectedRows = 2) {
  const rows = []; let row = [], field = '', quoted = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quoted) {
      if (c === '"' && text[i + 1] === '"') { field += c; i++; }
      else if (c === '"') quoted = false;
      else field += c;
    } else if (c === '"') quoted = true;
    else if (c === ',') { row.push(field); field = ''; }
    else if (c === '\n') { row.push(field); rows.push(row); row = []; field = ''; }
    else if (c !== '\r') field += c;
  }
  if (quoted) throw Error('Unclosed CSV quote');
  if (field || row.length) { row.push(field); rows.push(row); }
  const [header, ...values] = rows;
  if (!header || JSON.stringify(header) !== JSON.stringify(expectedCSVHeader) || new Set(header).size !== 33 || values.length !== expectedRows || values.some(r => r.length !== 33)) throw Error(`Expected exact 33-column header and exactly ${expectedRows} rectangular CSV records with role meaning`);
  return values.map(r => Object.fromEntries(header.map((h,i) => [h,r[i]])));
}
function expectedComparisonRows(state, oracle, serviceID = '') {
  const ids = serviceID ? ['gateway-api'] : ['gateway-api', 'search-api'];
  const labels = serviceID ? ['Gateway API'] : ['Gateway API', 'Search API'];
  const selectedIndexes = serviceID ? [0] : [0, 1];
  const baseAggregation = 'non-negative integer count of failed API request attempts or worker job attempts in the selected calendar month; retries count separately';
  const partialAggregation = 'non-negative integer count of failed API request attempts or worker job attempts during the observed first 20 days of the selected month; retries count separately';
  const baseUncertainty = 'Exact synthetic fixture counts, not production measurements. No statistical interval is estimated; counts are not normalized for traffic or month length.';
  const partialUncertainty = baseUncertainty + ' Partial counts may increase; the number of failures on unobserved days is unknown.';
  const coverage = state === 'missing' ? 'No current-period source report is available for any selected service; missing counts are not zero.' : state === 'partial' ? 'Only 2026-09-01 to 2026-09-21 (end exclusive; UTC), the first 20 days, is observed. Later days are unavailable; every present count is a lower bound, not a full-month total.' : '';
  const selection = { selection_period: comparisonSelection.period, selection_compare: comparisonSelection.compare, selection_category: comparisonSelection.category, selection_state: state, selection_service: serviceID };
  const datasetContext = (series, period, periodLabel, datasetState, window, freshness, completeness) => ({
    series, ...selection,
    context_chart_id: `service-failures-${period}-api-${datasetState}${serviceID ? '-gateway-api' : ''}`,
    context_title: `Failed service operations — ${periodLabel} · API services${serviceID ? ' · Gateway API' : ''}`,
    context_unit: 'failed operations',
    context_grain: 'one service in the API services selection',
    context_aggregation: series === 'current' && state === 'partial' ? partialAggregation : baseAggregation,
    context_window_kind: 'range', context_window: window,
    context_source_kind: 'derived',
    context_source: `Synthetic Gallery operating ledger v1; period=${period}; category=api; state=${datasetState}; comparison=previous. Not live telemetry.${serviceID ? ' Detail service=gateway-api.' : ''}`,
    context_source_href: '', context_source_draft: 'false', context_evaluated_at: '2026-10-05T12:00:00Z',
    context_freshness: freshness, context_completeness: completeness,
    context_coverage_note: series === 'current' ? coverage : '',
    context_uncertainty: series === 'current' && state === 'partial' ? partialUncertainty : baseUncertainty,
    context_decimal_places: '0',
  });
  const data = (id, label, value, rowState, missingReason = '') => ({ id, label, value, unit: 'failed operations', state: rowState, missing_reason: missingReason, lower: '', upper: '', role: 'accent', role_label: 'API services' });
  const currentContext = datasetContext('current', '2026-09', 'September 2026', state, '2026-09-01T00:00:00Z to 2026-10-01T00:00:00Z (end exclusive; UTC)', oracle.freshness, oracle.completeness);
  const previousContext = datasetContext('previous', '2026-08', 'August 2026', 'complete', '2026-08-01T00:00:00Z to 2026-09-01T00:00:00Z (end exclusive; UTC)', 'Fresh: source observed 2026-10-05T11:00:00Z; maximum age 24h0m0s', 'complete');
  const current = selectedIndexes.map((sourceIndex, index) => ({ ...currentContext, ...data(ids[index], labels[index], oracle.current[sourceIndex], oracle.current[sourceIndex] === '' ? 'missing' : 'present', oracle.missingReasons?.[sourceIndex] ?? '') }));
  const previous = selectedIndexes.map((sourceIndex, index) => ({ ...previousContext, ...data(ids[index], labels[index], oracle.previous[sourceIndex], 'present') }));
  return [...current, ...previous];
}
function validateComparisonCSV(records, state, oracle, serviceID = '') {
  const expected = expectedComparisonRows(state, oracle, serviceID);
  if (records.length !== expected.length) throw Error(`Comparison CSV row count ${records.length}; want ${expected.length}`);
  for (let index = 0; index < expected.length; index++) {
    const expectedFields = Object.keys(expected[index]);
    if (expectedFields.length !== expectedCSVHeader.length || expectedCSVHeader.some(field => !Object.hasOwn(expected[index], field))) throw Error(`Independent comparison oracle row ${index} does not cover all 33 columns`);
    for (const field of expectedCSVHeader) {
      const want = expected[index][field];
      if (records[index][field] !== want) throw Error(`Comparison CSV row ${index} ${field}=${JSON.stringify(records[index][field])}; want ${JSON.stringify(want)}`);
    }
  }
  return expected.map(row => `${row.series}:${row.id}:${row.value}`);
}
async function inspect(page, name) {
  const geometry = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth,
    zoom: {dpr: devicePixelRatio, inner: innerWidth, outer: outerWidth, visualScale: visualViewport.scale},
    controls: [...document.querySelectorAll('#gallery-main select, #gallery-main button, #gallery-main summary, #analytics-value-table a')].filter(e => e.checkVisibility()).map(e => {
      const r = e.getBoundingClientRect(); return { tag: e.tagName, text: e.textContent.trim().slice(0, 60), width: r.width, height: r.height };
    }), scripts: document.scripts.length,
  }));
  report.geometry.push({ name, ...geometry });
  if (zoom) check(geometry.zoom.dpr === 2 && geometry.zoom.inner === 720 && geometry.zoom.outer === 1440 && geometry.viewport === 720 && geometry.zoom.visualScale === 1, name + ': actual 200% browser zoom', geometry.zoom);
  check(geometry.scroll <= geometry.viewport, name + ': no outer overflow', geometry);
  check(geometry.scripts === 0, name + ': production scripts absent', geometry.scripts);
  for (const c of geometry.controls) check(c.width >= 44 && c.height >= 44, name + ': control target ' + c.text, c);
  const contrast = await page.evaluate(() => {
    const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
    const ctx = canvas.getContext('2d', { willReadFrequently: true });
    const rgba = value => { ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = value; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data].map((v,i) => i === 3 ? v / 255 : v); };
    const lum = color => color.slice(0,3).map(v => { v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4; }).reduce((s,v,i) => s + v * [.2126,.7152,.0722][i], 0);
    return [...document.querySelectorAll('#gallery-main :is(p, h1, h2, h3, label, select, button, summary, a, th, td, dt, dd, text)')].filter(e => e.checkVisibility() && [...e.childNodes].some(n => n.nodeType === 3 && n.textContent.trim())).map(e => {
      const s = getComputedStyle(e), fg = rgba(e.tagName === 'text' ? s.fill : s.color);
      let bg, skipped;
      for (let p = e; p; p = p.parentElement) {
        const st = getComputedStyle(p);
        if (parseFloat(st.opacity) !== 1 || st.backgroundImage !== 'none') { skipped = 'opacity or image background'; break; }
        const c = rgba(st.backgroundColor);
        if (c[3] === 1) { bg = c; break; }
        if (c[3] !== 0) { skipped = 'translucent background'; break; }
      }
      const large = parseFloat(s.fontSize) >= 24 || (parseFloat(s.fontSize) >= 18.666 && parseFloat(s.fontWeight) >= 700);
      return { tag: e.tagName, text: e.textContent.trim().slice(0,90), foreground: fg, background: bg, font: s.fontSize, large, skipped: skipped ?? (!bg || fg[3] !== 1 ? 'nonopaque or unresolved colors' : undefined), ratio: bg && fg[3] === 1 ? (Math.max(lum(fg),lum(bg))+.05)/(Math.min(lum(fg),lum(bg))+.05) : null };
    });
  });
  report.contrasts.push({ name, samples: contrast });
  for (const c of contrast) check(!c.skipped && c.ratio >= (c.large ? 3 : 4.5), name + ': text contrast ' + c.text, c);
}
async function comparisonTable(page) {
  const rows = await page.locator('#analytics-value-table tbody tr').evaluateAll(nodes => nodes.map(node => ({
    id: node.getAttribute('data-value-id'),
    cells: [...node.querySelectorAll('td')].map(cell => cell.textContent.trim()),
  })));
  return rows;
}
async function comparisonPageFacts(page) {
  const section = page.locator('section[aria-labelledby="analytics-reading"]');
  const sourceFreshness = await page.getByText('Source freshness', { exact: true }).evaluate(node => node.nextElementSibling?.textContent.trim());
  return {
    narrative: (await section.locator(':scope > p').innerText()).trim(),
    stateLabel: (await section.locator('h3').innerText()).trim(),
    stateDescription: (await section.locator('h3 + p').innerText()).trim(),
    sourceFreshness,
    rows: await comparisonTable(page),
    chartTexts: await page.locator('section[aria-labelledby="analytics-comparison"] svg').allTextContents(),
    chartValues: await page.locator('section[aria-labelledby="analytics-comparison"] svg').evaluateAll(nodes => nodes.map(node => [...node.querySelectorAll('text')].map(text => text.textContent.trim()))),
    noPlotText: await page.locator('section[aria-labelledby="analytics-comparison"] > div > p').innerText(),
  };
}
function assertComparisonFacts(facts, oracle, detail) {
  const indexes = detail ? [0] : [0, 1];
  const ids = detail ? ['gateway-api'] : ['gateway-api', 'search-api'];
  const expectedRows = indexes.map((sourceIndex, index) => ({ id: ids[index], cells: [oracle.currentDisplay[sourceIndex], oracle.previous[sourceIndex], oracle.changes[sourceIndex], oracle.rowState] }));
  const expectedNarrative = detail ? oracle.detail : oracle.narrative;
  const expectedCharts = oracle.chart === 'absent' ? [] : indexes.map(i => oracle.chart === 'current-only' ? ['First 20 days', oracle.current[i]] : ['Sep 2026', oracle.current[i], 'Aug 2026', oracle.previous[i]]);
  if (JSON.stringify(facts.chartValues) !== JSON.stringify(expectedCharts)) throw Error(`Chart values differ from independent fixture: ${JSON.stringify({got: facts.chartValues, expectedCharts})}`);
  if (facts.narrative !== expectedNarrative || facts.stateLabel !== oracle.stateLabel || facts.stateDescription !== oracle.stateDescription || facts.sourceFreshness !== (oracle.freshness.startsWith('Fresh:') ? 'Source observed 5 Oct 2026, 11:00 UTC. Freshness budget: 24 hours.' : oracle.freshness.startsWith('Stale:') ? 'Source observed 2 Oct 2026, 12:00 UTC. Freshness budget: 24 hours.' : 'The missing fixture has no current-period source observation.')) throw Error(`Comparison explanation mismatch: ${JSON.stringify(facts)}`);
  if (JSON.stringify(facts.rows) !== JSON.stringify(expectedRows)) throw Error(`Comparison table mismatch: ${JSON.stringify({ got: facts.rows, expectedRows })}`);
  if (oracle.chart === 'absent') {
    if (facts.chartTexts.length !== 0 || facts.noPlotText.trim() !== 'No observed failed-operation values are available to plot.') throw Error(`Missing state acquired a plot: ${JSON.stringify(facts.chartTexts)}`);
  } else if (oracle.chart === 'current-only') {
    if (facts.chartTexts.length !== indexes.length || facts.chartTexts.some(text => !text.includes('First 20 days') || text.includes('Aug 2026'))) throw Error(`Partial state plotted a complete comparison: ${JSON.stringify(facts.chartTexts)}`);
  } else if (facts.chartTexts.length !== indexes.length || facts.chartTexts.some(text => !text.includes('Sep 2026') || !text.includes('Aug 2026'))) throw Error(`Complete comparison labels changed: ${JSON.stringify(facts.chartTexts)}`);
}
async function comparisonDownload(page, state, oracle, name, serviceID = '') {
  await skip(page);
  const selector = serviceID ? `a[href^="/analytics/${serviceID}/export.csv?"]` : 'a[href^="/analytics/export.csv?"]';
  await tabTo(page, selector);
  const downloading = page.waitForEvent('download');
  await page.keyboard.press('Enter');
  const download = await downloading;
  const suffix = serviceID || 'overview';
  const target = path.join(out, `${name}-${suffix}.csv`);
  await download.saveAs(target);
  const bytes = fs.readFileSync(target);
  const expectedRows = serviceID ? 2 : 4;
  const records = csvRecords(bytes.toString(), expectedRows);
  const values = validateComparisonCSV(records, state, oracle, serviceID);
  report.artifacts.push({ path: path.basename(target), bytes: bytes.length, sha256: sha(bytes), columns: 33, rows: expectedRows, values });
  return { path: path.basename(target), bytes: bytes.length, sha256: sha(bytes), columns: 33, rows: expectedRows, values };
}
async function runComparisonState(item, theme) {
  const oracle = comparisonOracle[item.state];
  const name = `comparison-${item.state}-${zoom ? 'zoom200' : 'ordinary'}`;
  const startingFailures = report.failures.length, startingErrors = report.errors.length, startingBlocked = report.blocked.length;
  item.status = 'executing'; item.executed = true; item.started = new Date().toISOString();
  let activePhase;
  const startPhase = phase => { activePhase = phase; item.phases[phase] = 'executing'; };
  const passPhase = phase => { item.phases[phase] = 'passed'; activePhase = undefined; };
  let context, page, pending = [];
  try {
    startPhase('selection');
    ({ context, page, pending } = await makePage(1440, theme));
    await page.goto(origin.origin + '/analytics?theme=' + theme, { waitUntil: 'load' });
    await skip(page);
    for (const [field, letters, want] of [['period', 's', comparisonSelection.period], ['compare', 'p', comparisonSelection.compare], ['category', 'api', comparisonSelection.category]]) {
      await tabTo(page, `select[name=${field}]`);
      for (const letter of letters) await key(page, letter);
      if (await page.locator(`select[name=${field}]`).inputValue() !== want) throw Error(`Native ${field} selection did not reach ${want}`);
    }
    await tabTo(page, '#analytics-filter details summary'); await key(page, 'Enter');
    await tabTo(page, 'select[name=state]'); await key(page, oracle.key);
    if (await page.locator('select[name=state]').inputValue() !== item.state) throw Error(`Native state selection did not reach ${item.state}`);
    await tabTo(page, '#analytics-filter button[type=submit]', true);
    const selected = page.waitForURL(url => url.pathname === '/analytics' && url.searchParams.get('period') === comparisonSelection.period && url.searchParams.get('compare') === comparisonSelection.compare && url.searchParams.get('category') === comparisonSelection.category && url.searchParams.get('state') === item.state && url.searchParams.get('theme') === theme);
    await page.keyboard.press('Enter'); await selected;
    passPhase('selection');

    startPhase('overview');
    await skip(page); await tabTo(page, 'details:has(> dl) > summary'); await key(page, 'Enter');
    if (!(await page.locator('#analytics-source').isVisible())) throw Error('Native disclosure did not expose provenance');

    const overviewFacts = await comparisonPageFacts(page);
    assertComparisonFacts(overviewFacts, oracle, false);
    await inspect(page, name + '-overview');
    await screenshot(page, name + '-overview');
    item.overview = overviewFacts; passPhase('overview');
    startPhase('overviewCSV');
    item.overviewCSV = await comparisonDownload(page, item.state, oracle, name);
    passPhase('overviewCSV');

    startPhase('detail');
    await skip(page); await tabTo(page, '#analytics-chart-gateway-api a');
    const detailNavigation = page.waitForURL(url => url.pathname === '/analytics/gateway-api');
    await page.keyboard.press('Enter'); await detailNavigation;
    const detailURL = new URL(page.url());
    for (const [key, value] of Object.entries({ ...comparisonSelection, state: item.state, theme })) if (detailURL.searchParams.get(key) !== value) throw Error(`Gateway detail lost ${key}=${value}`);
    const detailFacts = await comparisonPageFacts(page);
    assertComparisonFacts(detailFacts, oracle, true);
    await inspect(page, name + '-detail');
    await screenshot(page, name + '-detail');
    item.detail = detailFacts; passPhase('detail');
    startPhase('detailCSV');
    item.detailCSV = await comparisonDownload(page, item.state, oracle, name, 'gateway-api');
    passPhase('detailCSV');

    startPhase('returned');
    await skip(page); await tabTo(page, 'a[href^="/analytics?"]');
    const returnedNavigation = page.waitForURL(url => url.pathname === '/analytics');
    await page.keyboard.press('Enter'); await returnedNavigation;
    const returned = new URL(page.url());
    for (const [key, value] of Object.entries({ ...comparisonSelection, state: item.state, theme })) if (returned.searchParams.get(key) !== value) throw Error(`Back lost ${key}=${value}`);
    item.returnedURL = page.url();
    if (context.serviceWorkers().length !== 0) throw Error('Service worker unexpectedly active in comparison lane');
    passPhase('returned');
    item.status = 'passed';
  } catch (error) {
    if (activePhase && item.phases[activePhase] === 'executing') item.phases[activePhase] = 'failed';
    activePhase = undefined;
    item.status = 'failed'; item.failures.push(error.stack ?? String(error));
    if (page) {
      try { await screenshot(page, name + '-failure'); }
      catch (captureError) { item.failures.push(`failure capture: ${captureError.stack ?? String(captureError)}`); }
    }
  } finally {
    if (context) item.phases.cleanup = 'executing';
    if (pending.length) {
      try { await Promise.all(pending); }
      catch (error) { item.phases.cleanup = 'failed'; item.failures.push(`response settlement: ${error.stack ?? String(error)}`); item.status = 'failed'; }
    }
    if (report.failures.length !== startingFailures || report.errors.length !== startingErrors || report.blocked.length !== startingBlocked) {
      if (context) item.phases.cleanup = 'failed';
      item.failures.push('Comparison lane added a failed check, browser error, or blocked request; inspect the global receipt arrays.');
      item.status = 'failed';
    }
    if (context) {
      try { await context.close(); if (item.phases.cleanup === 'executing') item.phases.cleanup = 'passed'; }
      catch (error) { item.phases.cleanup = 'failed'; item.failures.push(`context close: ${error.stack ?? String(error)}`); item.status = 'failed'; }
    } else item.phases.cleanup = 'not-run';
    item.finished = new Date().toISOString();
  }
}
try {
  for (const width of (zoom ? [1440] : [390, 768, 1440])) for (const theme of ['light', 'dark']) {
    const { context, page, pending } = await makePage(width, theme);
    const name = width + '-' + theme;
    try {
      await page.goto(origin.origin + '/analytics?theme=' + theme, { waitUntil: 'load' });
      await inspect(page, name + '-overview');
      await screenshot(page, name + '-overview');
      await skip(page);
      for (const [field, letters, want] of [['period','a','2026-08'],['compare','n','none'],['category','api','api']]) {
        await tabTo(page, 'select[name=' + field + ']');
        for (const letter of letters) await key(page, letter);
        check(await page.locator('select[name=' + field + ']').inputValue() === want, name + ': native type-ahead ' + field, want);
      }
      await tabTo(page, '#analytics-filter details summary'); await key(page, 'Enter');
      await tabTo(page, 'select[name=state]'); await key(page, 'p');
      check(await page.locator('select[name=state]').inputValue() === 'partial', name + ': native type-ahead state', 'partial');
      await tabTo(page, '#analytics-filter button[type=submit]', true);
      const changed = page.waitForURL(u => u.searchParams.get('period') === '2026-08' && u.searchParams.get('compare') === 'none' && u.searchParams.get('category') === 'api' && u.searchParams.get('state') === 'partial' && u.searchParams.get('theme') === theme);
      await page.keyboard.press('Enter'); await changed;
      check((await page.locator('[data-value-id="gateway-api"] td').first().innerText()) === 'At least 4', name + ': key-only filter result', page.url());
      await inspect(page, name + '-partial');
      await skip(page); await tabTo(page, '#analytics-chart-gateway-api a');
      const detail = page.waitForURL(u => u.pathname === '/analytics/gateway-api');
      await page.keyboard.press('Enter'); await detail;
      check(new URL(page.url()).searchParams.get('state') === 'partial' && await page.locator('#analytics-source').isVisible(), name + ': detail context and source', page.url());
      if (zoom) await inspect(page, name + '-detail');
      await screenshot(page, name + '-detail');
      await skip(page); await tabTo(page, 'a[href^="/analytics?"]');
      const back = page.waitForURL(u => u.pathname === '/analytics');
      await page.keyboard.press('Enter'); await back;
      check(new URL(page.url()).searchParams.get('state') === 'partial', name + ': Back retains selection', page.url());
      await skip(page); await tabTo(page, 'a[href^="/analytics/export.csv?"]');
      const downloading = page.waitForEvent('download'); await page.keyboard.press('Enter');
      const download = await downloading, target = path.join(out, name + '.csv'); await download.saveAs(target);
      const csv = fs.readFileSync(target); report.artifacts.push({ path: path.basename(target), bytes: csv.length, sha256: sha(csv) });
      const records = csvRecords(csv.toString());
      check(records.map(r => r.id + ':' + r.value).join(',') === 'gateway-api:4,search-api:2' && records.every(r => r.series === 'current' && r.state === 'present' && r.selection_period === '2026-08' && r.selection_compare === 'none' && r.selection_category === 'api' && r.selection_state === 'partial' && r.context_completeness === 'partial' && r.context_window === '2026-08-01T00:00:00Z to 2026-09-01T00:00:00Z (end exclusive; UTC)'), name + ': key-only export values and selection', { bytes: csv.length, records });
      check(context.serviceWorkers().length === 0, name + ': no service workers', 0);
    } catch (e) { report.failures.push({ name, detail: e.stack }); }
    finally { await Promise.all(pending); await context.close(); }
  }
  for (const [index, item] of report.comparisonStates.entries()) await runComparisonState(item, index % 2 === 0 ? 'light' : 'dark');
  // Five fresh contexts; no input delays or browser-driver time included in DOM timing.
  for (let i = 0; i < 5; i++) {
    const { context, page, pending } = await makePage(1440, 'light');
    try {
      await page.goto(origin.origin + '/analytics?theme=light', { waitUntil: 'load' });
      report.timings.push(await page.evaluate(() => { const n = performance.getEntriesByType('navigation')[0]; return { domContentLoadedMs: n.domContentLoadedEventEnd, responseEndMs: n.responseEnd, loadMs: n.loadEventEnd, transferBytes: n.transferSize, encodedBytes: n.encodedBodySize, decodedBytes: n.decodedBodySize }; }));
    } finally { await Promise.all(pending); await context.close(); }
  }
  const sorted = report.timings.map(t => t.domContentLoadedMs).sort((a,b) => a-b);
  report.localDOMContentLoadedP95Ms = sorted[Math.ceil(sorted.length * .95) - 1];
  check(sorted.length === 5 && report.localDOMContentLoadedP95Ms <= 250, 'Predeclared local DOM-ready budget', report.localDOMContentLoadedP95Ms);
} catch (e) { report.failures.push({ name: 'harness', detail: e.stack }); }
finally {
  check(report.comparisonStates.every(item => item.status === 'passed'), 'All four exact comparison states completed', report.comparisonStates.map(item => ({ state: item.state, status: item.status, phases: item.phases, failures: item.failures })));
  check(report.errors.length === 0, 'No unexpected browser errors', report.errors);
  check(report.blocked.length === 0, 'No outside/write/WebSocket requests', report.blocked);
  await browser?.close(); report.finished = new Date().toISOString();
  fs.writeFileSync(path.join(out, 'receipt.json'), JSON.stringify(report, null, 2) + '\n');
}
console.log(JSON.stringify({ out, passes: report.passes.length, failures: report.failures.length, failuresByName: report.failures.map(f=>f.name), timingP95Ms: report.localDOMContentLoadedP95Ms }));
process.exitCode = report.failures.length ? 1 : 0;
