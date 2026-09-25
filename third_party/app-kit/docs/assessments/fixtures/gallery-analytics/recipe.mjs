// Optional isolated Chrome proof of the actual reusable renderer, not the
// Gallery's separate paired-period implementation. No installation or live data.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import {pathToFileURL} from 'node:url';

const origin = new URL(process.argv[2] ?? '');
if (origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || !origin.port || origin.pathname !== '/' || origin.search || origin.hash || origin.username || origin.password) throw Error('Explicit bare loopback HTTP origin required');
const entry = fs.realpathSync(process.env.APPKIT_PLAYWRIGHT_ENTRY ?? '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs');
const packageBytes = fs.readFileSync(path.join(path.dirname(entry), 'package.json'));
const pkg = JSON.parse(packageBytes);
if (path.basename(entry) !== 'index.mjs' || pkg.name !== 'playwright' || pkg.version !== '1.63.0') throw Error('Reviewed existing Playwright 1.63.0 required; no auto-install');
const sha = b => crypto.createHash('sha256').update(b).digest('hex');
const out = fs.mkdtempSync(path.join(os.tmpdir(), 'appkit-analytics-recipe-'));
const report = {origin: origin.origin, runnerSHA256: sha(fs.readFileSync(new URL(import.meta.url))), playwright: {version: pkg.version, entrySHA256: sha(fs.readFileSync(entry)), packageSHA256: sha(packageBytes)}, passes: [], failures: [], geometry: [], blocked: [], errors: [], artifacts: [], requests: [], budgets: {htmlBytes: 32768}, limits: ['Synthetic three-service fixture only; no consumer or production certification.', 'Chrome at 100% zoom; no screen-reader or native input claim.', 'Geometry checks actual glyph rectangles, not screenshot similarity or full accessibility conformance.']};
const check = (ok, name, detail) => report[ok ? 'passes' : 'failures'].push({name, detail});
const allowed = new Set(['/analytics/recipe', '/analytics/recipe/export.csv', '/analytics', '/static/app.css']);
const {chromium} = await import(pathToFileURL(entry).href);
const browser = await chromium.launch({executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, args: ['--no-first-run', '--no-default-browser-check']});
report.browser = browser.version();

async function tabTo(page, selector) {
  for (let i = 0; i < 65; i++) {
    if (await page.evaluate(s => document.activeElement?.matches(s), selector)) {
      const focus = await page.evaluate(() => {const s = getComputedStyle(document.activeElement); return {visible: document.activeElement.matches(':focus-visible'), width: parseFloat(s.outlineWidth), style: s.outlineStyle};});
      check(focus.visible && focus.width > 0 && focus.style !== 'none', 'Visible focus: ' + selector, focus);
      return true;
    }
    await page.keyboard.press('Tab');
  }
  check(false, 'Keyboard reaches ' + selector);
  return false;
}

async function inspect(page, name, expectedSVG) {
  const g = await page.evaluate(() => {
    const rect = e => {const r = e.getBoundingClientRect(); return {x: r.x, y: r.y, right: r.right, bottom: r.bottom, width: r.width, height: r.height};};
    return {width: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth,
      charts: [...document.querySelectorAll('figure svg:not([data-analytics-legend-swatch])')].map(svg => ({box: rect(svg), texts: [...svg.querySelectorAll('text')].map(t => ({text: t.textContent, ...rect(t)})), marks: [...svg.querySelectorAll('rect')].map(rect)})),
      regions: [...document.querySelectorAll('figure [role="region"]')].map(e => ({label: e.getAttribute('aria-label'), tabIndex: e.tabIndex, width: e.clientWidth, scroll: e.scrollWidth})),
      rows: [...document.querySelectorAll('figure tbody tr')].map(r => [...r.cells].map(c => c.textContent.trim())),
      rowLabelWidths: [...document.querySelectorAll('figure tbody th[scope="row"]')].map(e => e.getBoundingClientRect().width),
      scripts: document.scripts.length};
  });
  report.geometry.push({name, ...g});
  check(g.scroll <= g.width + 1, name + ': no outer horizontal overflow', g);
  check(g.scripts === 0, name + ': script-free task');
  check(g.charts.length === expectedSVG, name + ': correct SVG inventory', g.charts.length);
  check(g.regions.length === (expectedSVG ? 2 : 1) && g.regions.every(r => r.label && r.tabIndex === 0), name + ': named keyboard-scrollable chart/table regions', g.regions);
  check(g.rowLabelWidths.length === 3 && g.rowLabelWidths.every(w => w >= 128), name + ': service-name column retains readable width', g.rowLabelWidths);
  for (const chart of g.charts) for (const t of chart.texts) {
    check(t.x >= chart.box.x - 0.5 && t.right <= chart.box.right + 0.5 && t.y >= chart.box.y - 0.5 && t.bottom <= chart.box.bottom + 0.5, name + ': glyph stays inside SVG: ' + t.text, {text: t, svg: chart.box});
    const overlaps = chart.marks.filter(r => t.x < r.right - 0.5 && t.right > r.x + 0.5 && t.y < r.bottom - 0.5 && t.bottom > r.y + 0.5);
    check(overlaps.length === 0, name + ': text does not overlap bar/track: ' + t.text, overlaps);
  }
  return g;
}

try {
  for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
    const context = await browser.newContext({viewport: {width, height: 1000}, colorScheme: theme, reducedMotion: 'reduce', serviceWorkers: 'block', acceptDownloads: true});
    const pending = [];
    try {
      context.setDefaultTimeout(6000);
      await context.route('**/*', route => {
        const r = route.request(), u = new URL(r.url());
        if (u.origin === origin.origin && allowed.has(u.pathname) && ['GET', 'HEAD'].includes(r.method())) return route.continue();
        report.blocked.push({url: r.url(), method: r.method()}); return route.abort();
      });
      await context.routeWebSocket('**/*', ws => {report.blocked.push({websocket: ws.url()}); ws.close();});
      const page = await context.newPage();
      page.on('pageerror', e => report.errors.push(e.message));
      page.on('console', m => {if (m.type() === 'error') report.errors.push(m.text());});
      page.on('response', response => {
        if (response.headers()['content-disposition']?.includes('attachment')) return;
        pending.push((async () => {
          const bytes = await response.body();
          report.requests.push({url: response.url(), status: response.status(), bytes: bytes.length, sha256: sha(bytes)});
          check(response.status() === 200, 'Successful admitted response', response.url());
          if (response.headers()['content-type']?.startsWith('text/html')) check(bytes.length <= report.budgets.htmlBytes, 'HTML budget', bytes.length);
        })().catch(e => report.errors.push(e.message)));
      });
      await (await context.newCDPSession(page)).send('Emulation.setScriptExecutionDisabled', {value: true});
      await page.goto(origin.origin + '/analytics/recipe?compare=none&theme=' + theme, {waitUntil: 'networkidle'});
      const name = width + '-' + theme;
      const g = await inspect(page, name + '-complete', 1);
      // Independent fixture ledger: September full month is 4/1/0; do not
      // derive the expectation from the HTML or its exported projection.
      check(JSON.stringify(g.rows.map(r => [r[0], r[1], r[3]])) === JSON.stringify([['Gateway API', '4', 'Present'], ['Search API', '1', 'Present'], ['Report worker', '0', 'Present']]), name + ': independent complete values', g.rows);
      const file = path.join(out, name + '.png');
      await page.screenshot({path: file, fullPage: true});
      report.artifacts.push({path: path.basename(file), sha256: sha(fs.readFileSync(file))});
      if (await tabTo(page, 'a[href="#gallery-main"]')) await page.keyboard.press('Enter');
      check(await page.evaluate(() => document.activeElement.id === 'gallery-main'), name + ': skip reaches main');
      if (g.regions.length) {
        if (await tabTo(page, 'figure [role="region"]')) {
          const before = await page.evaluate(() => document.activeElement.scrollLeft);
          await page.keyboard.press('ArrowRight');
          await page.waitForTimeout(150);
          const after = await page.evaluate(() => document.activeElement.scrollLeft);
          check(g.regions[0].scroll <= g.regions[0].width || after > before, name + ': keyboard scroll exposes wide chart', {before, after});
        }
      }
      if (await tabTo(page, 'select[name="state"]')) {
        await page.keyboard.press('Home'); await page.keyboard.type('Missing'); await page.keyboard.press('Tab');
        check(await page.locator('select[name="state"]').inputValue() === 'missing', name + ': native type-ahead selects missing');
        if (await tabTo(page, 'button[type="submit"]')) await Promise.all([page.waitForURL(u => u.searchParams.get('state') === 'missing'), page.keyboard.press('Enter')]);
        const missing = await inspect(page, name + '-missing', 0);
        check(missing.rows.length === 3 && missing.rows.every(r => r[1] === 'Missing' && r[3] !== 'Present'), name + ': missing never becomes zero', missing.rows);
      }
      if (await tabTo(page, 'a[href^="/analytics/recipe/export.csv"]')) {
        const [download] = await Promise.all([page.waitForEvent('download'), page.keyboard.press('Enter')]);
        const file = path.join(out, name + '.csv'); await download.saveAs(file);
        const bytes = fs.readFileSync(file), csv = bytes.toString('utf8');
        report.artifacts.push({path: path.basename(file), bytes: bytes.length, sha256: sha(bytes)});
        check(csv.includes('gateway-api,Gateway API,,failed operations,missing,') && csv.includes('search-api,Search API,,failed operations,missing,') && csv.includes('report-worker,Report worker,,failed operations,missing,'), name + ': actual CSV retains independent missing rows');
      }
      await Promise.all(pending);
    } catch (e) {check(false, width + '-' + theme + ': task completes', e.stack);} finally {await context.close();}
  }
  check(report.blocked.length === 0, 'No blocked external/write/socket requests', report.blocked);
  check(report.errors.length === 0, 'No browser errors', report.errors);
} finally {
  await browser.close();
  fs.writeFileSync(path.join(out, 'receipt.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({out, passes: report.passes.length, failures: report.failures.length, receiptSHA256: sha(fs.readFileSync(path.join(out, 'receipt.json')))}));
}
if (report.failures.length) process.exitCode = 1;
