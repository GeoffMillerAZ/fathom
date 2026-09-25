// Assessment only: exact native Gallery report task. Args: Playwright entry,
// Chrome executable, printed loopback origin, compiled CSS, new output dir.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import { performance } from 'node:perf_hooks';
import { pathToFileURL } from 'node:url';
import { trackOwnedChild, terminateOwned } from '../../../../apps/styleguide/browser/owned_process.mjs';
import { waitForPhase } from '../../../../apps/styleguide/browser/phase_wait.mjs';

const [pw, chrome, origin, cssFile, output] = process.argv.slice(2);
const base = new URL(origin);
assert.equal(base.protocol, 'http:'); assert.equal(base.hostname, '127.0.0.1');
assert.ok(base.port && base.origin === origin);
for (const value of [pw, chrome, cssFile, output]) assert.ok(path.isAbsolute(value));
const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const cssText = await fs.readFile(cssFile, 'utf8');
const cssSHA = sha(Buffer.from(cssText));
const surfaces = [...new Set([...cssText.matchAll(/--ak-color-surface:(oklch\([^)]+\))/g)].map(match => match[1]))];
assert.equal(surfaces.length, 2, 'compiled CSS must declare source light/dark surfaces');
assert.ok(cssText.includes('--text-lg:1.125rem') && cssText.includes('--font-weight-semibold:600'),
  'compiled CSS must retain heading canary tokens');
const sourceStyle = { light: surfaces[0], dark: surfaces[1] };
const cells = [{ name: 'ordinary-1440-light', width: 1440, scheme: 'light', degraded: false, limit: 500 },
  { name: 'degraded-390-dark', width: 390, scheme: 'dark', degraded: true, limit: 2500 }];
const phaseNames = ['initial', 'apply', 'detail', 'back'];
const report = { schema: 'appkit-gallery-report-readiness-v1', css_sha256: cssSHA,
  observation_complete: false, task_passed: false, budgets_passed: false, passed: false,
  cells: [], errors: [], blocked: [], browser_process_exit_verified: false };
let browserServer, browser, owned, page, created = false;

async function bounded(label, run, ms, onTimeout = () => {}) {
  let timer;
  try { return await Promise.race([run(), new Promise((_, reject) => {
    timer = setTimeout(() => { onTimeout(); reject(Error(`${label} timeout ${ms}ms`)); }, ms);
  })]); } finally { clearTimeout(timer); }
}
function drainProcess(child) {
  const capture = { stdout: '', stderr: '', truncated: false };
  for (const name of ['stdout', 'stderr']) child[name]?.on('data', chunk => {
    const next = capture[name] + chunk.toString();
    if (Buffer.byteLength(next) > 1024 * 1024) capture.truncated = true;
    else capture[name] = next;
  });
  return capture;
}
async function tabTo(page, selector, backwards = false) {
  for (let n = 0; n < 60; n++) {
    if (await page.evaluate(s => document.activeElement?.matches(s), selector)) return;
    await page.keyboard.press(backwards ? 'Shift+Tab' : 'Tab');
  }
  throw Error(`native Tab never reached ${selector}`);
}
async function choose(page, field, keys, value) {
  await tabTo(page, `select[name=${field}]`);
  for (const key of keys) await page.keyboard.press(key);
  assert.equal(await page.locator(`select[name=${field}]`).inputValue(), value, `native ${field}`);
}
async function prepareApply(page) {
  await choose(page, 'period', ['a'], '2026-08');
  await choose(page, 'compare', ['n'], 'none');
  await choose(page, 'category', ['a', 'p', 'i'], 'api');
  await tabTo(page, '#analytics-filter details summary');
  await page.keyboard.press('Enter');
  await choose(page, 'state', ['p'], 'partial');
  await tabTo(page, '#analytics-filter button[type=submit]', true);
}
async function taskFacts(page, phase, scheme) {
  const want = phase === 'initial' ? { period: '2026-09', compare: 'previous', category: 'api', state: 'complete' } :
    { period: '2026-08', compare: 'none', category: 'api', state: 'partial' };
  const location = new URL(page.url());
  assert.equal(location.origin, origin);
  assert.equal(location.pathname, phase === 'detail' ? '/analytics/gateway-api' : '/analytics');
  for (const [key, value] of Object.entries(want)) assert.equal(location.searchParams.get(key), value, `${phase} ${key}`);
  assert.equal(location.searchParams.get('theme'), scheme);
  const facts = await page.evaluate(() => {
    const visible = element => {
      if (!element?.checkVisibility()) return false;
      const style = getComputedStyle(element), rect = element.getBoundingClientRect();
      return style.visibility === 'visible' && Number(style.opacity) > 0 && rect.width > 0 && rect.height > 0 &&
        rect.right > 0 && rect.left < innerWidth;
    };
    const rows = [...document.querySelectorAll('#analytics-value-table tbody tr')].map(row => ({
      id: row.getAttribute('data-value-id'), cells: [...row.querySelectorAll('td')].map(cell => cell.textContent.trim()),
      cellsComputedVisible: [...row.querySelectorAll('td')].every(cell => cell.checkVisibility() &&
        getComputedStyle(cell).visibility === 'visible' && cell.getBoundingClientRect().height > 0),
      visible: visible(row), height: row.getBoundingClientRect().height }));
    const main = document.querySelector('#gallery-main');
    const chart = document.querySelector('#analytics-chart-gateway-api');
    const fitsViewport = element => { const rect = element?.getBoundingClientRect();
      return rect && rect.width > 0 && rect.left >= -1 && rect.right <= innerWidth + 1; };
    const chartRows = [...document.querySelectorAll('section[aria-labelledby="analytics-comparison"] li')];
    const chartsFit = chartRows.every(row => {
      const region = row.querySelector('[role="region"]'), svg = region?.querySelector('svg');
      if (!fitsViewport(row) || !fitsViewport(region) || !svg ||
          !['auto', 'scroll'].includes(getComputedStyle(region).overflowX) ||
          region.scrollWidth < region.clientWidth) return false;
      const box = svg.viewBox.baseVal;
      return [...svg.querySelectorAll('text')].every(label => {
        const glyph = label.getBBox();
        return glyph.x >= box.x - 1 && glyph.y >= box.y - 1 &&
          glyph.x + glyph.width <= box.x + box.width + 1 &&
          glyph.y + glyph.height <= box.y + box.height + 1;
      });
    });
    const tableRegion = document.querySelector('#analytics-value-table')?.parentElement;
    const labels = [...document.querySelectorAll('details dt')];
    const source = Object.fromEntries(labels.map(label => [label.textContent.trim(), label.nextElementSibling?.textContent.trim()]));
    const linkedCSS = [...document.querySelectorAll('link[rel="stylesheet"]')].filter(link =>
      link.href === location.origin + '/static/app.css' && !link.disabled &&
      link.sheet && link.sheet.cssRules.length > 0);
    const rootStyle = getComputedStyle(document.documentElement);
    const headingStyle = getComputedStyle(document.querySelector('section[aria-labelledby="analytics-reading"] h2'));
    return { rows, mainVisible: visible(main), chartVisible: visible(chart), chartsFit,
      bodyWidth: document.documentElement.scrollWidth, viewportWidth: document.documentElement.clientWidth,
      linkedCSSCount: linkedCSS.length, dataTheme: document.documentElement.getAttribute('data-theme'),
      computedSurface: rootStyle.getPropertyValue('--ak-color-surface').trim(),
      computedPolarity: rootStyle.getPropertyValue('--ak-polarity').trim(),
      headingFontSize: headingStyle.fontSize, headingFontWeight: headingStyle.fontWeight,
      headingFontFamily: headingStyle.fontFamily,
      tableRegion: { visible: visible(tableRegion), tabindex: tableRegion?.getAttribute('tabindex'),
        overflowX: tableRegion ? getComputedStyle(tableRegion).overflowX : '',
        clientWidth: tableRegion?.clientWidth, scrollWidth: tableRegion?.scrollWidth },
      tableRegionFitsViewport: fitsViewport(tableRegion),
      chartValues: [...document.querySelectorAll('section[aria-labelledby="analytics-comparison"] svg')]
        .map(svg => [...svg.querySelectorAll('text')].map(text => text.textContent.trim())),
      chartText: chart?.textContent || '', source,
      narrative: document.querySelector('section[aria-labelledby="analytics-reading"] > p')?.textContent.trim(),
      stateLabel: document.querySelector('section[aria-labelledby="analytics-reading"] h3')?.textContent.trim(),
      fonts: document.fonts.status, scripts: document.scripts.length,
      background: getComputedStyle(document.body).backgroundColor };
  });
  assert.ok(facts.mainVisible && facts.chartVisible && facts.chartsFit,
    `${phase} report/chart fit intended viewport or scroll region`);
  assert.equal(facts.linkedCSSCount, 1, `${phase} exact linked stylesheet enabled and applicable`);
  assert.equal(facts.dataTheme, scheme, `${phase} selected scheme`);
  assert.equal(facts.computedSurface, sourceStyle[scheme], `${phase} source-derived computed surface`);
  assert.equal(facts.computedPolarity, scheme, `${phase} computed polarity`);
  assert.equal(facts.headingFontSize, '18px', `${phase} source-derived heading size`);
  assert.equal(facts.headingFontWeight, '600', `${phase} source-derived heading weight`);
  assert.ok(facts.headingFontFamily.includes('system-ui'), `${phase} system font actually applied`);
  assert.ok(facts.rows.every(row => row.visible && row.cellsComputedVisible && row.height > 0), `${phase} rows visible`);
  assert.ok(facts.bodyWidth <= facts.viewportWidth, `${phase} no outer clipping`);
  assert.ok(facts.tableRegionFitsViewport && facts.tableRegion.visible && facts.tableRegion.tabindex === '0' &&
    ['auto', 'scroll'].includes(facts.tableRegion.overflowX) &&
    facts.tableRegion.scrollWidth >= facts.tableRegion.clientWidth,
  `${phase} keyboard-scrollable exact table`);
  assert.equal(facts.scripts, 0, `${phase} no app scripts`);
  const expected = phase === 'initial' ?
    [['gateway-api', ['4', '6', '2 fewer', 'Present']], ['search-api', ['1', '3', '2 fewer', 'Present']]] :
    phase === 'detail' ? [['gateway-api', ['At least 4', 'Partial, lower bound']]] :
      [['gateway-api', ['At least 4', 'Partial, lower bound']],
        ['search-api', ['At least 2', 'Partial, lower bound']]];
  assert.deepEqual(facts.rows.map(row => [row.id, row.cells]), expected, `${phase} independent table cells`);
  assert.deepEqual(facts.chartValues, phase === 'initial' ?
    [['Sep 2026', '4', 'Aug 2026', '6'], ['Sep 2026', '1', 'Aug 2026', '3']] :
    phase === 'detail' ? [['First 20 days', '4']] :
      [['First 20 days', '4'], ['First 20 days', '2']], `${phase} chart values`);
  if (phase !== 'initial') {
    assert.ok(facts.chartText.includes('First 20 days'), `${phase} lower-bound chart`);
    assert.ok(!facts.chartText.includes('Aug 2026'), `${phase} no previous comparison plot`);
    assert.equal(facts.stateLabel, 'Partial month: first 20 days only');
    assert.ok(facts.narrative.includes('lower bound'), `${phase} partial explanation`);
    assert.ok(facts.source.Source.includes('Synthetic Gallery operating ledger v1; period=2026-08; category=api; state=partial; comparison=none.'), `${phase} source`);
    assert.ok(facts.source['Reporting window'].includes('2026-08-01 inclusive to 2026-09-01 exclusive, UTC.'), `${phase} window`);
    assert.ok(facts.source.Coverage.includes('first 20 days') &&
      facts.source.Coverage.includes('lower bound'), `${phase} coverage`);
  } else {
    assert.equal(facts.stateLabel, 'Complete reporting window');
    assert.ok(facts.narrative.includes('total 5 failed operations') &&
      facts.narrative.includes('totals 9'), 'initial complete comparison totals');
    assert.ok(facts.source.Source.includes('Synthetic Gallery operating ledger v1; period=2026-09; category=api; state=complete; comparison=previous.'), 'initial source');
    assert.ok(facts.source['Reporting window'].includes('2026-09-01 inclusive to 2026-10-01 exclusive, UTC.'), 'initial window');
  }
  assert.ok(facts.source['Grain and unit'].includes('failed operations'), `${phase} unit`);
  if (phase === 'detail') {
    assert.equal(await page.locator('h1').innerText(), 'Gateway API', 'Gateway detail title');
    assert.ok(facts.narrative.includes('first-20-day failed-operation count is 4'), 'Gateway detail lower bound');
  }
  return facts;
}
async function settled(page) {
  await page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });
}

try {
  await fs.mkdir(output); created = true;
  const { chromium } = await import(pathToFileURL(pw).href);
  browserServer = await bounded('fresh Chrome launchServer', () => chromium.launchServer({
    executablePath: chrome, headless: true, host: '127.0.0.1', timeout: 7000 }), 8000);
  owned = trackOwnedChild(browserServer.process());
  report.browser_capture = drainProcess(owned.child);
  report.browser_pid = owned.child.pid;
  browser = await bounded('fresh Chrome connect', () => chromium.connect(browserServer.wsEndpoint(),
    { timeout: 7000 }), 8000);
  report.browser_version = browser.version();
  let workloadExpired = false;
  await bounded('six-journey workload', async () => {
  for (const cell of cells) for (let trial = 1; trial <= 3; trial++) {
    if (workloadExpired) throw Error('workload watchdog expired');
    const run = { cell: cell.name, trial, phases: [], task_complete: false, budgets_passed: false, errors: [] };
    report.cells.push(run);
    let context, cdp;
    try {
      context = await browser.newContext({ viewport: { width: cell.width, height: 1000 },
        // DevTools measurement callbacks need the engine. The exact document CSP
        // below still forbids application scripts; no app script nodes are allowed.
        colorScheme: cell.scheme, reducedMotion: 'reduce', javaScriptEnabled: true,
        serviceWorkers: 'block', acceptDownloads: false });
      page = await context.newPage(); page.setDefaultTimeout(7000);
      cdp = await context.newCDPSession(page);
      await cdp.send('Network.enable');
      await cdp.send('Network.setCacheDisabled', { cacheDisabled: true });
      if (cell.degraded) await cdp.send('Network.emulateNetworkConditions', {
        offline: false, latency: 100, downloadThroughput: 200000, uploadThroughput: 100000 });
      const failures = [];
      page.on('pageerror', error => failures.push({ type: 'page', message: String(error) }));
      page.on('console', item => { if (item.type() === 'error') failures.push({ type: 'console', message: item.text() }); });
      page.on('requestfailed', request => failures.push({ type: 'request', url: request.url(), error: request.failure()?.errorText }));
      await context.route('**/*', route => {
        const request = route.request(), target = new URL(request.url());
        if (target.origin === origin && request.method() === 'GET' &&
            ['/analytics', '/analytics/gateway-api', '/static/app.css'].includes(target.pathname)) return route.continue();
        report.blocked.push({ cell: cell.name, trial, method: request.method(), url: request.url() });
        return route.abort();
      });
      for (const phase of phaseNames) {
        if (workloadExpired) throw Error('workload watchdog expired');
        const item = { name: phase, started_at: new Date().toISOString(), observation_complete: false,
          task_passed: false, budgets_passed: false, requests: [], responses: [], pending: 0 };
        run.phases.push(item);
        const start = performance.now();
        const expectedPath = phase === 'detail' ? '/analytics/gateway-api' : '/analytics';
        const onRequest = request => item.requests.push({ method: request.method(), path: new URL(request.url()).pathname,
          url: request.url(), at_ms: performance.now() });
        const onResponse = response => {
          item.pending++;
          void (async () => {
            const bytes = await response.body();
            item.responses.push({ method: response.request().method(), path: new URL(response.url()).pathname,
              url: response.url(), status: response.status(), headers: response.headers(),
              response_body_bytes: bytes.length, sha256: sha(bytes), finished_ms: performance.now(),
              is_css: new URL(response.url()).pathname === '/static/app.css' });
          })().catch(error => item.responses.push({ method: response.request().method(),
            path: new URL(response.url()).pathname, read_error: String(error) }))
            .finally(() => { item.pending--; });
        };
        page.on('request', onRequest); page.on('response', onResponse);
        try {
          if (phase === 'initial') await page.goto(origin + '/analytics?period=2026-09&compare=previous&category=api&state=complete&theme=' + cell.scheme, { waitUntil: 'load' });
          else {
            await page.keyboard.press('Enter');
            await page.waitForURL(url => url.pathname === expectedPath &&
              url.searchParams.get('period') === '2026-08' && url.searchParams.get('state') === 'partial');
          }
          await waitForPhase({ inspect: () => item, expected: [`GET ${expectedPath}`, 'GET /static/app.css'], timeoutMs: 10000 });
          if (workloadExpired) throw Error('workload watchdog expired');
          assert.equal(item.requests.length, 2);
          assert.equal(item.responses.length, 2);
          for (const response of item.responses) assert.equal(response.status, 200);
          const html = item.responses.find(response => response.path === expectedPath);
          const css = item.responses.find(response => response.is_css);
          assert.ok(html && css && css.sha256 === cssSHA, `${phase} complete exact CSS`);
          assert.match(html.headers['content-type'] || '', /^text\/html(?:;|$)/i);
          assert.equal(html.headers['content-security-policy'],
            "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'",
            `${phase} unchanged application-script denial`);
          assert.match(css.headers['content-type'] || '', /^text\/css(?:;|$)/i);
          assert.ok(html.response_body_bytes <= 32768, `${phase} HTML <=32KiB`);
          assert.ok(html.response_body_bytes + css.response_body_bytes <= 163840, `${phase} decoded document+CSS <=160KiB`);
          assert.equal(report.blocked.length, 0, `${phase} no unexpected requests`);
          item.facts = await taskFacts(page, phase, cell.scheme);
          await bounded('font/frame settlement', () => settled(page), 3000);
          if (workloadExpired) throw Error('workload watchdog expired');
          item.elapsed_ms = performance.now() - start;
          item.observation_complete = true; item.task_passed = true;
          item.budgets_passed = item.elapsed_ms <= cell.limit;
          assert.ok(item.budgets_passed, `${phase} ${item.elapsed_ms.toFixed(1)}ms > ${cell.limit}ms`);
          assert.equal(failures.length, 0, `${phase} browser errors`);
          if (phase === 'initial') await prepareApply(page);
          else if (phase === 'apply') await tabTo(page, '#analytics-chart-gateway-api a');
          else if (phase === 'detail') await tabTo(page, 'a[href^="/analytics?"]');
        } catch (error) {
          item.elapsed_ms ??= performance.now() - start;
          item.error = String(error.stack || error); throw error;
        } finally { page.off('request', onRequest); page.off('response', onResponse); }
      }
      run.task_complete = true;
      run.budgets_passed = run.phases.every(item => item.budgets_passed);
      const shot = path.join(output, `${cell.name}-${trial}.png`);
      await bounded('post-task screenshot', () => page.screenshot({ path: shot, fullPage: false, timeout: 3000 }), 4000);
      run.screenshot = { file: path.basename(shot), sha256: sha(await fs.readFile(shot)) };
      if (failures.length) throw Error(`browser errors: ${JSON.stringify(failures)}`);
    } catch (error) {
      run.errors.push(String(error.stack || error));
      if (page) {
        const shot = path.join(output, `${cell.name}-${trial}-failure.png`);
        try { await bounded('failure screenshot', () => page.screenshot({ path: shot, fullPage: false, timeout: 3000 }), 4000);
          run.failure_screenshot = { file: path.basename(shot), sha256: sha(await fs.readFile(shot)) }; }
        catch (captureError) { run.errors.push(`failure screenshot: ${captureError}`); }
      }
      throw error;
    } finally {
      try { if (cdp) await bounded('CDP detach', () => cdp.detach(), 2000); }
      catch (error) { run.errors.push(`CDP detach: ${error}`); }
      try { if (context) await bounded('context close', () => context.close(), 2000); }
      catch (error) { run.errors.push(`context close: ${error}`); }
      page = null;
    }
  }
  }, 75000, () => { workloadExpired = true; });
  report.observation_complete = report.cells.length === 6 && report.cells.every(run => run.phases.length === 4);
  report.task_passed = report.cells.every(run => run.task_complete && !run.errors.length);
  report.budgets_passed = report.cells.every(run => run.budgets_passed);
} catch (error) { report.errors.push(String(error.stack || error)); }
finally {
  try { if (browser) await bounded('browser client close', () => browser.close(), 2000); }
  catch (error) { report.errors.push(`browser client close: ${error}`); }
  try { if (browserServer) await bounded('browser server close', () => browserServer.close(), 2000); }
  catch (error) { report.errors.push(`browser server close: ${error}`); }
  if (owned) {
    report.browser_process = owned.state;
    try { report.browser_termination = await terminateOwned(owned, { graceMs: 2000, killMs: 2000, drainMs: 2000 });
      report.browser_process_exit_verified = report.browser_termination.ok && !report.browser_capture.truncated; }
    catch (error) { report.errors.push(`browser termination: ${error}`); }
  }
  report.passed = report.observation_complete && report.task_passed && report.budgets_passed &&
    report.browser_process_exit_verified && report.errors.length === 0 && report.blocked.length === 0;
  if (created) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({ passed: report.passed, observation_complete: report.observation_complete,
    task_passed: report.task_passed, budgets_passed: report.budgets_passed,
    cells: report.cells.length, errors: report.errors, output }, null, 2));
  if (!report.passed) process.exitCode = 1;
}
