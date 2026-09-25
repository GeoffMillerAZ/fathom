// Opt-in synthetic Gallery selection-disclosure browser check. Args: copied
// Gallery binary, compiled CSS, Playwright entry, Chrome, fresh output directory.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { trackOwnedChild, terminateOwned } from '../../../../apps/styleguide/browser/owned_process.mjs';
import { waitForPhase } from '../../../../apps/styleguide/browser/phase_wait.mjs';

const [binary, cssFile, pw, chrome, output] = process.argv.slice(2);
for (const item of [binary, cssFile, pw, chrome, output]) assert.ok(path.isAbsolute(item), `absolute path required: ${item}`);
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
async function hashFile(file) { const sha = crypto.createHash('sha256');
  for await (const chunk of createReadStream(file)) sha.update(chunk); return sha.digest('hex'); }
async function bounded(label, work, ms) { let timer; try { return await Promise.race([work(),
  new Promise((_, reject) => { timer = setTimeout(() => reject(Error(`${label} timed out`)), ms); })]); }
  finally { clearTimeout(timer); } }
const self = fileURLToPath(import.meta.url);
const helper = path.resolve(path.dirname(self), '../../../../apps/styleguide/browser/owned_process.mjs');
const barrier = path.resolve(path.dirname(self), '../../../../apps/styleguide/browser/phase_wait.mjs');
const cssText = await fs.readFile(cssFile, 'utf8');
const cssSHA = hash(Buffer.from(cssText));
const surface = {};
for (const match of cssText.matchAll(/\[data-theme=(light|dark)\]\{([^}]*)\}/g)) {
  const token = match[2].match(/--ak-color-surface:(oklch\([^)]+\))/);
  if (token) surface[match[1]] = token[1];
}
assert.ok(surface.light && surface.dark && surface.light !== surface.dark,
  'source CSS light/dark surfaces');
async function playwrightPackage(entry) { let dir = path.dirname(entry);
  for (let n = 0; n < 8; n++) { const file = path.join(dir, 'package.json');
    try { const pkg = JSON.parse(await fs.readFile(file, 'utf8'));
      if (pkg.name === 'playwright' || pkg.name === 'playwright-core')
        return { file, name: pkg.name, version: pkg.version }; }
    catch (error) { if (error.code !== 'ENOENT') throw error; }
    const parent = path.dirname(dir); if (parent === dir) break; dir = parent;
  } throw Error('Playwright package manifest unavailable'); }
const report = { scope: 'synthetic Gallery candidate, not consumer acceptance', cells: [], documents: [], errors: [],
  blocked: [], task_complete: false, passed: false, chrome_exit_verified: false, gallery_exit_verified: false };
let gallery, browserServer, browser, chromeOwned, origin, madeOutput = false;
function capture(owned) { const data = { stdout: '', stderr: '', truncated: false };
  for (const name of ['stdout', 'stderr']) owned.child[name]?.on('data', chunk => {
    const next = data[name] + chunk.toString(); if (Buffer.byteLength(next) > 1024 * 1024) data.truncated = true;
    else data[name] = next;
  }); return data; }
async function ready(owned, captured) { const deadline = Date.now() + 20000;
  while (Date.now() < deadline) { if (owned.state.exit || owned.state.error) throw Error('Gallery exited before ready');
    const match = captured.stderr.match(/app-kit Gallery → (http:\/\/127\.0\.0\.1:([1-9]\d*))/);
    if (match) { if (Number(match[2]) > 65535) throw Error('invalid printed port');
      try { const response = await fetch(match[1] + '/analytics', { signal: AbortSignal.timeout(1500) });
        if (response.status === 200) { await response.arrayBuffer(); return match[1]; } } catch { /* starting */ }
    } await new Promise(resolve => setTimeout(resolve, 100));
  } throw Error('owned Gallery ready line and HTTP 200 absent'); }
async function tabTo(page, selector, backwards = false) { for (let n = 0; n < 60; n++) {
  if (await page.evaluate(s => document.activeElement?.matches(s), selector)) return;
  await page.keyboard.press(backwards ? 'Shift+Tab' : 'Tab'); }
  throw Error(`native Tab did not reach ${selector}`); }
async function choose(page, field, keys, value) { await tabTo(page, `select[name=${field}]`);
  for (const key of keys) await page.keyboard.press(key);
  assert.equal(await page.locator(`select[name=${field}]`).inputValue(), value, `native ${field}`); }
function target(state, scheme, period = '2026-09', compare = 'previous') {
  return `/analytics?period=${period}&compare=${compare}&category=api&state=${state}&theme=${scheme}`;
}
function assertSelection(page, pathname, scheme, state = 'partial', period = '2026-08', compare = 'none') {
  const selected = new URL(page.url()); assert.equal(selected.origin, origin);
  assert.equal(selected.pathname, pathname);
  for (const [key, value] of Object.entries({ period, compare, category: 'api', state, theme: scheme }))
    assert.equal(selected.searchParams.get(key), value, `${pathname} ${key}`);
}
const documentCSP = "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'";
async function observeDocument(page, name, pathname, navigate) {
  const phase = { name, requests: [], responses: [], pending: 0 };
  const onRequest = request => phase.requests.push({ method: request.method(),
    path: new URL(request.url()).pathname, url: request.url() });
  const onResponse = response => { phase.pending++;
    void bounded('response body', () => response.body(), 7000).then(bytes => {
      phase.responses.push({ method: response.request().method(), path: new URL(response.url()).pathname,
        url: response.url(), status: response.status(), headers: response.headers(),
        response_body_bytes: bytes.length, sha256: hash(bytes), finished_ms: Date.now() });
    }, error => phase.responses.push({ method: response.request().method(),
      path: new URL(response.url()).pathname, read_error: String(error) }))
      .finally(() => { phase.pending--; });
  };
  page.on('request', onRequest); page.on('response', onResponse);
  try {
    await bounded(`${name} navigation`, navigate, 8000);
    await waitForPhase({ inspect: () => phase, expected: [`GET ${pathname}`, 'GET /static/app.css'], timeoutMs: 8000 });
    const html = phase.responses.find(row => row.path === pathname);
    const css = phase.responses.find(row => row.path === '/static/app.css');
    assert.equal(html.status, 200); assert.equal(css.status, 200);
    assert.equal(html.url, origin + pathname + new URL(page.url()).search);
    assert.equal(css.url, origin + '/static/app.css');
    assert.equal(html.headers['content-security-policy'], documentCSP);
    assert.match(html.headers['content-type'] || '', /^text\/html(?:;|$)/i);
    assert.match(css.headers['content-type'] || '', /^text\/css(?:;|$)/i);
    assert.ok(html.response_body_bytes <= 32768 &&
      html.response_body_bytes + css.response_body_bytes <= 163840, `${name} bounded complete bodies`);
    assert.equal(css.sha256, cssSHA, `${name} source CSS hash`);
    report.documents.push(phase);
  } finally { page.off('request', onRequest); page.off('response', onResponse); }
}
async function staticFacts(page, state, scheme, { firstScreen = false, compare = 'previous' } = {}) {
  const facts = await page.evaluate(() => {
    const outer = document.querySelector('#analytics-selection');
    const summary = outer?.querySelector(':scope > summary');
    const reading = document.querySelector('#analytics-reading');
    const stateLabel = document.querySelector('section[aria-labelledby="analytics-reading"] h3');
    const rect = element => { const r = element?.getBoundingClientRect(); return r &&
      { top: r.top, bottom: r.bottom, height: r.height, width: r.width }; };
    const style = summary && getComputedStyle(summary);
    const rootStyle = getComputedStyle(document.documentElement);
    const stylesheet = [...document.querySelectorAll('link[rel="stylesheet"]')].filter(link =>
      link.href === location.origin + '/static/app.css' && !link.disabled && link.sheet && link.sheet.cssRules.length > 0);
    return { outerClosed: outer?.tagName === 'DETAILS' && !outer.open,
      summary: summary?.textContent.trim(), summaryRect: rect(summary),
      readingRect: rect(reading), stateRect: rect(stateLabel), stateLabel: stateLabel?.textContent.trim(),
      summaryFocusStyle: style && { outline: style.outlineStyle, width: style.outlineWidth },
      dataTheme: document.documentElement.getAttribute('data-theme'),
      computedPolarity: rootStyle.getPropertyValue('--ak-polarity').trim(),
      computedSurface: rootStyle.getPropertyValue('--ak-color-surface').trim(),
      stylesheetCount: stylesheet.length,
      scriptCount: document.scripts.length, outerWidth: document.documentElement.scrollWidth,
      viewport: document.documentElement.clientWidth };
  });
  assert.ok(facts.outerClosed && facts.summary.includes('Change comparison') &&
    facts.summary.includes('API services') &&
    facts.summary.includes(compare === 'none' ? 'No comparison' : 'Previous calendar month'));
  assert.equal(facts.scriptCount, 0); assert.ok(facts.outerWidth <= facts.viewport);
  assert.equal(facts.dataTheme, scheme); assert.equal(facts.computedPolarity, scheme);
  assert.equal(facts.computedSurface, surface[scheme]); assert.equal(facts.stylesheetCount, 1);
  assert.ok(facts.summaryRect?.height >= 44 && facts.readingRect?.height > 0 && facts.stateRect?.height > 0);
  assert.equal(facts.stateLabel, { complete: 'Complete reporting window',
    missing: 'Source reports missing', stale: 'Source observation is stale',
    partial: 'Partial month: first 20 days only' }[state]);
  if (firstScreen) assert.ok(facts.readingRect.top >= 0 && facts.readingRect.bottom <= 1000 &&
    facts.stateRect.top >= 0 && facts.stateRect.bottom <= 1000, '390px first-screen result');
  return facts;
}
function parseCSV(text) { const rows = []; let row = [], field = '', quoted = false;
  for (let i = 0; i < text.length; i++) { const c = text[i];
    if (quoted) { if (c === '"' && text[i + 1] === '"') { field += c; i++; }
      else if (c === '"') quoted = false; else field += c; }
    else if (c === '"') quoted = true; else if (c === ',') { row.push(field); field = ''; }
    else if (c === '\n') { row.push(field); rows.push(row); row = []; field = ''; }
    else if (c !== '\r') field += c;
  } if (quoted) throw Error('unclosed CSV quote'); if (field || row.length) { row.push(field); rows.push(row); }
  return rows; }
const header = ['series','selection_period','selection_compare','selection_category','selection_state','selection_service',
  ...['chart_id','title','unit','grain','aggregation','window_kind','window','source_kind','source','source_href',
    'source_draft','evaluated_at','freshness','completeness','coverage_note','uncertainty','decimal_places']
    .map(value => 'context_' + value),
  'id','label','value','unit','state','missing_reason','lower','upper','role','role_label'];
function assertCSV(rows) { assert.equal(rows.length, 3); assert.deepEqual(rows[0], header);
  assert.equal(new Set(header).size, 33);
  const records = rows.slice(1).map(row => { assert.equal(row.length, 33);
    return Object.fromEntries(header.map((field, index) => [field, row[index]])); });
  assert.deepEqual(records.map(r => [r.id, r.label, r.value, r.state]),
    [['gateway-api','Gateway API','4','present'],['search-api','Search API','2','present']]);
  for (const row of records) {
    assert.equal(row.series, 'current'); assert.equal(row.selection_period, '2026-08');
    assert.equal(row.selection_compare, 'none'); assert.equal(row.selection_category, 'api');
    assert.equal(row.selection_state, 'partial'); assert.equal(row.selection_service, '');
    assert.equal(row.context_unit, 'failed operations'); assert.equal(row.context_completeness, 'partial');
    assert.equal(row.context_window, '2026-08-01T00:00:00Z to 2026-09-01T00:00:00Z (end exclusive; UTC)');
    assert.ok(row.context_source.includes('Synthetic Gallery operating ledger v1; period=2026-08; category=api; state=partial; comparison=none.'));
    assert.ok(row.context_coverage_note.includes('first 20 days') && row.context_coverage_note.includes('lower bound'));
    assert.equal(row.unit, 'failed operations'); assert.equal(row.role, 'accent');
    assert.equal(row.role_label, 'API services'); assert.equal(row.missing_reason, '');
  } return records.map(row => ({ id: row.id, value: row.value })); }

try {
  await fs.mkdir(output); madeOutput = true;
  const pkg = await playwrightPackage(pw);
  report.playwright_package = { name: pkg.name, version: pkg.version };
  report.playwright_manifest_path = pkg.file;
  report.node_version = process.version;
  const inputs = { binary, cssFile, pw, chrome, self, helper, barrier,
    node: process.execPath, playwright_manifest: pkg.file };
  report.input_sha256_before =
    Object.fromEntries(await Promise.all(Object.entries(inputs).map(async ([key, file]) => [key, await hashFile(file)])));
  gallery = trackOwnedChild(spawn(binary, ['-addr','127.0.0.1:0','-design-assets',''],
    { cwd: path.dirname(binary), stdio: ['ignore','pipe','pipe'] }));
  report.gallery_capture = capture(gallery);
  origin = await ready(gallery, report.gallery_capture); report.origin = origin;
  const { chromium } = await import(pathToFileURL(pw).href);
  browserServer = await bounded('Chrome launch', () => chromium.launchServer({ executablePath: chrome,
    headless: true, host: '127.0.0.1', timeout: 7000 }), 8000);
  chromeOwned = trackOwnedChild(browserServer.process()); report.chrome_capture = capture(chromeOwned);
  browser = await bounded('Chrome connect', () => chromium.connect(browserServer.wsEndpoint(), { timeout: 7000 }), 8000);
  report.chrome_version = browser.version();
  await bounded('two-lane synthetic journey', async () => {
    for (const lane of [{ name: '390-dark', width: 390, scheme: 'dark' },
      { name: '1440-light', width: 1440, scheme: 'light' }]) {
      const item = { lane: lane.name, states: [], journey_complete: false, errors: [] }; report.cells.push(item);
      const context = await browser.newContext({ viewport: { width: lane.width, height: 1000 },
        colorScheme: lane.scheme, reducedMotion: 'reduce', javaScriptEnabled: false,
        serviceWorkers: 'block', acceptDownloads: true });
      const page = await context.newPage(); page.setDefaultTimeout(7000);
      const cdp = await context.newCDPSession(page);
      await cdp.send('Network.enable'); await cdp.send('Network.setCacheDisabled', { cacheDisabled: true });
      const pageErrors = [];
      page.on('pageerror', error => pageErrors.push(String(error)));
      page.on('console', message => { if (message.type() === 'error') pageErrors.push(message.text()); });
      await context.route('**/*', route => { const request = route.request(), url = new URL(request.url());
        if (url.origin === origin && request.method() === 'GET' &&
          ['/analytics','/analytics/gateway-api','/analytics/export.csv','/static/app.css'].includes(url.pathname)) return route.continue();
        report.blocked.push({ url: request.url(), method: request.method() }); return route.abort(); });
      try {
        for (const state of ['complete','partial','missing','stale']) {
          const narrowPartial = lane.width === 390 && state === 'partial';
          const route = target(state, lane.scheme, narrowPartial ? '2026-08' : '2026-09', narrowPartial ? 'none' : 'previous');
          await observeDocument(page, `${lane.name}/${state}`, '/analytics', () =>
            page.goto(origin + route, { waitUntil: 'load' }));
          item.states.push({ state, path: route, facts: await staticFacts(page, state, lane.scheme,
            { firstScreen: narrowPartial, compare: narrowPartial ? 'none' : 'previous' }) });
          if (state === 'complete') {
            assert.deepEqual(await page.locator('#analytics-value-table tbody tr').evaluateAll(rows => rows.map(row =>
              [row.getAttribute('data-value-id'), ...[...row.querySelectorAll('td')].map(cell => cell.textContent.trim())])),
            [['gateway-api','4','6','2 fewer','Present'],['search-api','1','3','2 fewer','Present']]);
          }
          if (state === 'partial') {
            await page.evaluate(() => scrollTo(0, 0));
            const shot = path.join(output, `${lane.name}-initial-partial.png`);
            await bounded('initial partial screenshot', () => page.screenshot({ path: shot, fullPage: false, timeout: 3000 }), 4000);
            item.initial_partial_screenshot_sha256 = await hashFile(shot);
          }
        }
        await observeDocument(page, `${lane.name}/task-start`, '/analytics', () =>
          page.goto(origin + target('complete', lane.scheme), { waitUntil: 'load' }));
        await tabTo(page, '#analytics-selection > summary');
        const focused = await page.locator('#analytics-selection > summary').evaluate(node => {
          const s = getComputedStyle(node), r = node.getBoundingClientRect();
          return { visible: node.matches(':focus-visible'), outline: s.outlineStyle, width: parseFloat(s.outlineWidth), height: r.height };
        });
        assert.ok(focused.visible && focused.outline !== 'none' && focused.width > 0 && focused.height >= 44);
        await page.keyboard.press('Enter'); assert.ok(await page.locator('#analytics-selection').evaluate(node => node.open));
        await choose(page, 'period', ['a'], '2026-08'); await choose(page, 'compare', ['n'], 'none');
        await choose(page, 'category', ['a','p','i'], 'api');
        await tabTo(page, '#analytics-filter details summary'); await page.keyboard.press('Enter');
        await choose(page, 'state', ['p'], 'partial');
        await tabTo(page, '#analytics-filter button[type=submit]', true);
        await observeDocument(page, `${lane.name}/apply`, '/analytics', async () => {
          const applied = page.waitForURL(url => url.pathname === '/analytics' &&
            url.searchParams.get('period') === '2026-08' && url.searchParams.get('compare') === 'none' &&
            url.searchParams.get('category') === 'api' && url.searchParams.get('state') === 'partial');
          await page.keyboard.press('Enter'); await applied;
        });
        assertSelection(page, '/analytics', lane.scheme);
        item.applied = await staticFacts(page, 'partial', lane.scheme, { compare: 'none' });
        assert.deepEqual(await page.locator('#analytics-value-table tbody tr').evaluateAll(rows => rows.map(row =>
          [row.getAttribute('data-value-id'), row.querySelector('td')?.textContent.trim()])),
        [['gateway-api','At least 4'],['search-api','At least 2']]);
        await tabTo(page, '#analytics-chart-gateway-api a');
        await observeDocument(page, `${lane.name}/detail`, '/analytics/gateway-api', async () => {
          const detail = page.waitForURL(url => url.pathname === '/analytics/gateway-api');
          await page.keyboard.press('Enter'); await detail;
        });
        assertSelection(page, '/analytics/gateway-api', lane.scheme);
        assert.equal(await page.locator('h1').innerText(), 'Gateway API');
        assert.equal(await page.locator('#analytics-value-table tbody tr').count(), 1);
        assert.equal(await page.locator('#analytics-value-table tbody tr td').first().innerText(), 'At least 4');
        assert.ok(!(await page.locator('#analytics-selection').count()));
        await tabTo(page, '#gallery-main header > a');
        await observeDocument(page, `${lane.name}/back`, '/analytics', async () => {
          const back = page.waitForURL(url => url.pathname === '/analytics');
          await page.keyboard.press('Enter'); await back;
        });
        assertSelection(page, '/analytics', lane.scheme);
        await tabTo(page, 'a[href^="/analytics/export.csv?"]');
        const download = page.waitForEvent('download', { timeout: 7000 });
        await page.keyboard.press('Enter'); const artifact = await download;
        const file = path.join(output, `${lane.name}.csv`); await artifact.saveAs(file);
        assert.equal(await artifact.failure(), null);
        item.csv = { sha256: hash(await fs.readFile(file)), rows: assertCSV(parseCSV(await fs.readFile(file, 'utf8'))) };
        assert.deepEqual(pageErrors, []); assert.equal(report.blocked.length, 0);
        const shot = path.join(output, `${lane.name}.png`);
        await page.evaluate(() => scrollTo(0, 0));
        await bounded('post-task screenshot', () => page.screenshot({ path: shot, fullPage: false, timeout: 3000 }), 4000);
        item.screenshot_sha256 = await hashFile(shot); item.journey_complete = true;
      } catch (error) { item.errors.push(String(error.stack || error)); throw error; }
      finally { await bounded('context close', () => context.close(), 2000); }
    }
  }, 75000);
  report.task_complete = report.cells.length === 2 && report.cells.every(cell => cell.journey_complete);
} catch (error) { report.errors.push(String(error.stack || error)); }
finally {
  try { if (browser) await bounded('browser close', () => browser.close(), 2000); }
  catch (error) { report.errors.push(`browser close: ${error}`); }
  try { if (browserServer) await bounded('browser server close', () => browserServer.close(), 2000); }
  catch (error) { report.errors.push(`browser server close: ${error}`); }
  if (chromeOwned) try { report.chrome_process = chromeOwned.state;
    report.chrome_termination = await terminateOwned(chromeOwned,
    { graceMs: 2000, killMs: 2000, drainMs: 2000 });
    report.chrome_exit_verified = report.chrome_termination.ok && chromeOwned.state.exit?.code === 0 &&
      !report.chrome_capture.truncated; }
  catch (error) { report.errors.push(`Chrome cleanup: ${error}`); }
  if (gallery) try { report.gallery_process = gallery.state;
    report.gallery_termination = await terminateOwned(gallery,
    { graceMs: 6000, killMs: 2000, drainMs: 2000 });
    report.gallery_exit_verified = report.gallery_termination.ok && gallery.state.exit?.code === 0 &&
      gallery.state.signals.some(signal => signal.signal === 'SIGTERM' && signal.returned) &&
      !report.gallery_capture.truncated; }
  catch (error) { report.errors.push(`Gallery cleanup: ${error}`); }
  if (origin) try { const response = await fetch(origin + '/analytics', { signal: AbortSignal.timeout(1000) });
    report.errors.push(`owned listener still answered ${response.status}`); }
  catch (error) { report.port_refused = error.cause?.code === 'ECONNREFUSED';
    if (!report.port_refused) report.errors.push(`port refusal unverified: ${error}`); }
  try { report.input_sha256_after = Object.fromEntries(await Promise.all(
    Object.entries({ binary, cssFile, pw, chrome, self, helper, barrier,
      node: process.execPath, playwright_manifest: report.playwright_manifest_path })
      .map(async ([key, file]) => [key, await hashFile(file)])));
    if (Object.keys(report.input_sha256_before || {}).some(key =>
      report.input_sha256_before[key] !== report.input_sha256_after[key])) report.errors.push('input identity drift'); }
  catch (error) { report.errors.push(`input reread: ${error}`); }
  report.passed = report.task_complete && report.chrome_exit_verified && report.gallery_exit_verified &&
    report.port_refused && !report.blocked.length && !report.errors.length;
  if (madeOutput) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({ passed: report.passed, task_complete: report.task_complete,
    chrome_exit_verified: report.chrome_exit_verified, gallery_exit_verified: report.gallery_exit_verified,
    errors: report.errors, output }, null, 2));
  if (!report.passed) process.exitCode = 1;
}
