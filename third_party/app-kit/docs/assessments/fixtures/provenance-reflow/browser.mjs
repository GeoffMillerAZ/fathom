// Static provenance reflow/keyboard probe. No real server, scripts, signed-in
// profile, navigation, performance claim, or accessibility certification.
// Args: Playwright entry, Chrome executable, retained input root, new output root.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { createReadStream } from 'node:fs';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [playwrightEntry, chromePath, inputRoot, outputRoot] = process.argv.slice(2);
for (const value of [playwrightEntry, chromePath, inputRoot, outputRoot]) assert.ok(path.isAbsolute(value));
const notice = 'All figures on this page are synthetic examples. Source labels and citations demonstrate the API; they do not substantiate these example values.';
const expectedFigures = [
  ['prov-published', 'published', false, 'figure'], ['prov-derived', 'derived', false, 'figure'],
  ['prov-estimate', 'estimate', false, 'figure'], ['prov-draft', 'estimate', true, 'figure'],
  ['prov-mechanism', null, false, 'mechanism'],
];
const expectedSources = {
  'prov-published': 'Source CNCF Annual Survey 2024, table 6.',
  'prov-derived': 'Derived Sum of the FY25 billing export grouped by owner tag; arithmetic in docs/spend.md.',
  'prov-estimate': "Estimate Author's apportionment from three sampled months; no billing export behind it.",
  'prov-draft': "Estimate Author's apportionment from three sampled months; not yet reconciled.",
  'prov-mechanism': null,
};
const cases = ['light', 'dark'].flatMap(theme => [320, 390, 1280].map(width => ({ theme, width, id: `${theme}-${width}` })));
const report = { scope: 'retained first-party /provenance HTML; 6 responsive keyboard cells', inputs: {}, tools: {},
  cases: [], blocked: [], failures: [], passed: false,
  limits: ['synthetic static task', 'not full accessibility', 'not performance', 'not HTTP or OS network containment'] };
let browser, context, watchdog, timedOut = false;

const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
function hashFile(file) {
  return new Promise((resolve, reject) => { const hash = crypto.createHash('sha256');
    createReadStream(file).on('error', reject).on('data', chunk => hash.update(chunk)).on('end', () => resolve(hash.digest('hex'))); });
}
async function packageIdentity(entry) {
  let dir = path.dirname(await fs.realpath(entry));
  while (true) {
    try { const pkg = JSON.parse(await fs.readFile(path.join(dir, 'package.json'), 'utf8'));
      if (pkg.version && String(pkg.name).includes('playwright')) return { package: pkg.name, version: pkg.version }; }
    catch (error) { if (error.code !== 'ENOENT') throw error; }
    const parent = path.dirname(dir); if (parent === dir) return { package: null, version: null }; dir = parent;
  }
}
function check(cell, name, pass, detail) {
  cell.checks.push({ name, pass, ...(detail === undefined ? {} : { detail }) });
  if (!pass) report.failures.push({ case: cell.id, name, detail });
}
async function closeBounded(owner, label) {
  if (!owner) return;
  let timer;
  try { await Promise.race([owner.close(), new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(`${label} close timeout`)), 3000); })]); }
  catch (error) { report.failures.push({ case: 'owner', name: `${label} close`, detail: String(error) }); }
  finally { clearTimeout(timer); }
}
async function tabTo(page, handle) {
  for (let presses = 1; presses <= 100; presses++) {
    await page.keyboard.press('Tab');
    if (await handle.evaluate(node => node === document.activeElement).catch(() => false)) return presses;
  }
  return 0;
}
async function exerciseRegion(page, cell, handle, name) {
  const presses = await tabTo(page, handle); check(cell, `${name} keyboard reachable`, presses > 0, { presses });
  if (!presses) return;
  const before = await handle.evaluate(node => { const style = getComputedStyle(node); return {
    left: node.scrollLeft, client: node.clientWidth, scroll: node.scrollWidth, overflowX: style.overflowX,
    outlineStyle: style.outlineStyle, outlineWidth: Number.parseFloat(style.outlineWidth) || 0 }; });
  check(cell, `${name} focus outline`, before.outlineStyle !== 'none' && before.outlineWidth >= 2, before);
  check(cell, `${name} scrollbar contract`, ['auto', 'scroll'].includes(before.overflowX), before);
  if (before.scroll > before.client) { await page.keyboard.press('ArrowRight'); await page.waitForTimeout(80);
    const after = await handle.evaluate(node => node.scrollLeft);
    check(cell, `${name} ArrowRight scroll`, after > before.left, { before: before.left, after, client: before.client, scroll: before.scroll }); }
  check(cell, `${name} keeps outer viewport fixed`, await page.evaluate(() => window.scrollX === 0));
}
async function run() {
  await fs.mkdir(outputRoot, { mode: 0o700 });
  const driver = await fs.realpath(process.argv[1]), pw = await fs.realpath(playwrightEntry), chrome = await fs.realpath(chromePath);
  const html = {}, css = await fs.readFile(path.join(inputRoot, 'app.css'));
  for (const theme of ['light', 'dark']) html[theme] = await fs.readFile(path.join(inputRoot, `provenance-${theme}.html`));
  report.inputs = { driver: { path: driver, sha256: await hashFile(driver) }, css: { file: 'app.css', bytes: css.length, sha256: sha(css) },
    html: Object.fromEntries(Object.entries(html).map(([theme, body]) => [theme, { file: `provenance-${theme}.html`, bytes: body.length, sha256: sha(body) }])) };
  report.tools = { playwright: { entry: pw, sha256: await hashFile(pw), ...await packageIdentity(pw) },
    chrome: { executable: chrome, sha256: await hashFile(chrome), version: null } };
  if (!report.tools.playwright.version) report.failures.push({ case: 'identity', name: 'Playwright version unavailable' });
  const { chromium } = await import(pathToFileURL(pw).href);
  browser = await chromium.launch({ executablePath: chrome, headless: true, timeout: 30000 }); report.tools.chrome.version = browser.version();
  for (const spec of cases) {
    if (timedOut) break;
    const cell = { ...spec, checks: [], screenshots: {} }; report.cases.push(cell);
    try {
      context = await browser.newContext({ viewport: { width: spec.width, height: 1000 }, colorScheme: spec.theme,
        javaScriptEnabled: false, serviceWorkers: 'block', reducedMotion: 'reduce' }); context.setDefaultTimeout(10000);
      const pageURL = `http://127.0.0.1:65533/provenance?theme=${spec.theme}`;
      await context.route('**/*', async route => { const req = route.request(), url = req.url();
        if (req.method() === 'GET' && url === pageURL) return route.fulfill({ status: 200, contentType: 'text/html', body: html[spec.theme] });
        if (req.method() === 'GET' && url === 'http://127.0.0.1:65533/static/app.css') return route.fulfill({ status: 200, contentType: 'text/css', body: css });
        report.blocked.push({ case: cell.id, method: req.method(), url }); await route.abort(); });
      await context.routeWebSocket('**/*', socket => { report.blocked.push({ case: cell.id, websocket: socket.url() }); socket.close(); });
      const page = await context.newPage(); await page.goto(pageURL, { waitUntil: 'load', timeout: 10000 });
      cell.screenshots.initial = `${cell.id}-initial.png`; await page.screenshot({ path: path.join(outputRoot, cell.screenshots.initial), fullPage: true });
      const state = await page.evaluate(({ expectedFigures, notice }) => {
        const readable = node => {
          if (!node) return false;
          const range = document.createRange(); range.selectNodeContents(node);
          const rects = [...range.getClientRects()];
          return rects.length > 0 && rects.every(rect => rect.left >= 0 && rect.right <= innerWidth);
        };
        const figures = [...document.querySelectorAll('figure.ak-vizframe')];
        const figureState = figures.map(figure => { const id = figure.id;
          const regions = [...figure.querySelectorAll('[role="region"][tabindex="0"]')].filter(node => node.querySelector('svg'));
          const region = regions[0], caption = figure.querySelector('figcaption'), source = document.getElementById(`${id}-src`), svg = figure.querySelector('svg');
          return { id, sourceKind: figure.getAttribute('data-viz-source'), draft: figure.getAttribute('data-viz-draft') === 'true',
            carries: figure.getAttribute('data-viz-carries'), regionCount: regions.length,
            named: Boolean(region && (region.getAttribute('aria-label') || region.getAttribute('aria-labelledby'))),
            captionOutside: Boolean(caption && region && !region.contains(caption)), sourceOutside: Boolean(!source || region && !region.contains(source)),
            sourceText: source ? source.textContent.replace(/\s+/g, ' ').trim() : null,
            captionFits: readable(caption), sourceFits: !source || readable(source),
            width: svg?.getAttribute('width'), labels: [...(svg?.querySelectorAll('text') || [])].map(node => node.textContent.trim()) }; });
        const pres = [...document.querySelectorAll('pre')];
        return { figures: figureState, preCount: pres.length,
          accessiblePres: pres.filter(node => node.matches('[role="region"][tabindex="0"]') && (node.getAttribute('aria-label') || node.getAttribute('aria-labelledby'))).length,
          fit: document.documentElement.scrollWidth <= innerWidth && document.body.scrollWidth <= innerWidth && scrollX === 0,
          notice: document.body.innerText.includes(notice), expectedIDs: expectedFigures.map(row => row[0]) };
      }, { expectedFigures, notice });
      check(cell, 'exact synthetic-example notice', state.notice);
      check(cell, 'document and body fit viewport', state.fit, state);
      check(cell, 'exact figure IDs', JSON.stringify(state.figures.map(item => item.id)) === JSON.stringify(state.expectedIDs), state.figures);
      for (const [id, kind, draft, carries] of expectedFigures) {
        const actual = state.figures.find(item => item.id === id);
        check(cell, `${id} provenance metadata`, Boolean(actual) && actual.sourceKind === kind && actual.draft === draft && actual.carries === carries, actual);
        if (!actual) continue;
        check(cell, `${id} one named chart region`, actual.regionCount === 1 && actual.named, actual);
        check(cell, `${id} caption/source outside chart region`, actual.captionOutside && actual.sourceOutside, actual);
        check(cell, `${id} source text preserved`, actual.sourceText === expectedSources[id], actual);
        check(cell, `${id} caption/source text fits viewport`, actual.captionFits && actual.sourceFits, actual);
        const labels = id === 'prov-mechanism' ? ['ingest', 'validate', 'index', 'serve'] : ['platform', '48%', 'data', '31%', 'security', '21%'];
        check(cell, `${id} fixed SVG content`, actual.width === (id === 'prov-mechanism' ? '460' : '320') && JSON.stringify(actual.labels) === JSON.stringify(labels), actual);
      }
      check(cell, 'six code examples', state.preCount === 6, state); check(cell, 'six named code scroll regions', state.accessiblePres === 6, state);
      const handles = await page.locator('figure.ak-vizframe [role="region"][tabindex="0"]:has(svg), pre[role="region"][tabindex="0"]').elementHandles();
      check(cell, 'eleven keyboard scroll regions', handles.length === 11, { count: handles.length });
      await page.locator('body').click({ position: { x: 1, y: 1 } });
      for (let i = 0; i < handles.length; i++) await exerciseRegion(page, cell, handles[i], `region ${i + 1}`);
      check(cell, 'outer viewport remains fixed after tasks', await page.evaluate(() => window.scrollX === 0));
      cell.screenshots.postTask = `${cell.id}-post-task.png`; await page.screenshot({ path: path.join(outputRoot, cell.screenshots.postTask), fullPage: true });
    } catch (error) { check(cell, 'cell completed', false, String(error.stack || error)); }
    finally { await closeBounded(context, `${cell.id} context`); context = undefined; }
  }
}

const work = run();
const deadline = new Promise((_, reject) => { watchdog = setTimeout(() => { timedOut = true;
  void closeBounded(browser, 'watchdog browser'); reject(new Error('120s owner timeout')); }, 120000); });
try { await Promise.race([work, deadline]); }
catch (error) { report.failures.push({ case: 'owner', name: timedOut ? 'owner timeout' : 'execution', detail: String(error.stack || error) }); }
finally {
  clearTimeout(watchdog); await closeBounded(context, 'final context'); context = undefined;
  await closeBounded(browser, 'browser'); browser = undefined;
  let drainTimer;
  const drained = await Promise.race([work.then(() => true, () => true), new Promise(resolve => { drainTimer = setTimeout(() => resolve(false), 3000); })]);
  clearTimeout(drainTimer);
  if (!drained) report.failures.push({ case: 'owner', name: 'worker did not settle after cleanup' });
  report.passed = !timedOut && report.cases.length === 6 && report.failures.length === 0 && report.blocked.length === 0;
  try { await fs.writeFile(path.join(outputRoot, 'report.json'), JSON.stringify(report, null, 2), { flag: 'wx', mode: 0o600 }); }
  catch (error) { process.stderr.write(`report write failed: ${error}\n`); process.exitCode = 1; }
  if (!report.passed) process.exitCode = 1;
}
