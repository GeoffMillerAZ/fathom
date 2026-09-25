// Opt-in loopback proof for semantic analytics roles in HTML and CSV.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {pathToFileURL} from 'node:url';

const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const origin = new URL(process.argv[2] ?? '');
if (origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || !origin.port || origin.pathname !== '/' || origin.search || origin.hash || origin.username || origin.password) throw Error('explicit bare loopback HTTP origin required');
const parent = process.env.APPKIT_ANALYTICS_ROLE_OUTPUT_PARENT ?? '';
const parentStat = fs.lstatSync(parent);
if (!parentStat.isDirectory() || parentStat.isSymbolicLink() || (parentStat.mode & 0o777) !== 0o700 || fs.realpathSync(parent) !== path.resolve(parent)) throw Error('output parent must be an existing real 0700 directory');
const out = fs.mkdtempSync(path.join(parent, 'analytics-roles-'));
fs.chmodSync(out, 0o700);

const entry = fs.realpathSync(process.env.APPKIT_PLAYWRIGHT_ENTRY ?? '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs');
const packageBytes = fs.readFileSync(path.join(path.dirname(entry), 'package.json'));
const pkg = JSON.parse(packageBytes);
if (path.basename(entry) !== 'index.mjs' || pkg.name !== 'playwright' || pkg.version !== '1.63.0') throw Error('reviewed Playwright 1.63.0 required; no install or repin');
const report = {
  schema: 1, origin: origin.origin, harnessSHA256: sha(fs.readFileSync(new URL(import.meta.url))),
  playwright: {version: pkg.version, entrySHA256: sha(fs.readFileSync(entry)), packageSHA256: sha(packageBytes)},
  passes: [], failures: [], errors: [], blocked: [], requests: [], screenshots: [], downloads: [], downloadAborts: [],
  limits: ['Synthetic three-service Gallery fixture only.', 'Headless Chrome at 100% zoom with page scripts disabled; no screen-reader, native-popup, performance, or consumer-adoption claim.', 'Keyboard observation covers the native horizontal scroll region, not every assistive technology.'],
};
const check = (ok, name, detail) => report[ok ? 'passes' : 'failures'].push({name, detail});
const allowed = new Set(['/analytics/recipe', '/analytics/recipe/export.csv', '/__test/analytics-roles/long', '/__test/analytics-roles/long.csv', '/static/app.css']);

function parseCSV(text) {
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
  if (quoted) throw Error('unterminated CSV quote');
  if (field || row.length) { row.push(field); rows.push(row); }
  const headerIndex = rows.findIndex(r => JSON.stringify(r) === JSON.stringify(['id','label','value','unit','state','missing_reason','lower','upper','role','role_label']));
  if (headerIndex < 0) throw Error('ten-column analytics header absent');
  const nonblank = r => !(r.length === 1 && r[0] === '');
  return {context: rows.slice(0, headerIndex).filter(nonblank), header: rows[headerIndex], data: rows.slice(headerIndex + 1).filter(nonblank)};
}

const completeFirstEight = [
  ['gateway-api','Gateway API','4','failed operations','present','','',''],
  ['search-api','Search API','1','failed operations','present','','',''],
  ['report-worker','Report worker','0','failed operations','present','','',''],
];
const meanings = [['accent','API services'], ['accent','API services'], ['info','Worker services']];
const expectedCells = [
  ['Gateway API','4','Not provided','Present','API services'],
  ['Search API','1','Not provided','Present','API services'],
  ['Report worker','0','Not provided','Present','Worker services'],
];

async function tableState(page) {
  return page.evaluate(() => {
    const table = document.querySelector('figure table');
    const region = table?.closest('[role="region"]');
    const rows = [...(table?.tBodies[0]?.rows ?? [])].map(r => [...r.cells].map(c => c.textContent.trim()));
    const firstRow = table?.tBodies[0]?.rows[0];
    const lineRects = cell => {
      const rects = [];
      const walker = document.createTreeWalker(cell, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        if (!node.textContent.trim()) continue;
        const range = document.createRange(); range.selectNodeContents(node);
        for (const rect of range.getClientRects()) if (rect.width > 0 && rect.height > 0) rects.push({top: rect.top, bottom: rect.bottom, left: rect.left, right: rect.right});
      }
      return rects.sort((a, b) => a.top - b.top || a.left - b.left);
    };
    const cellLines = [...(firstRow?.cells ?? [])].map(lineRects);
    const labelStyle = firstRow ? getComputedStyle(firstRow.cells[0]) : null;
    return {headers: [...(table?.tHead?.rows[0]?.cells ?? [])].map(c => c.textContent.trim()), rows,
      svg: document.querySelectorAll('figure svg:not([data-analytics-legend-swatch])').length,
      swatches: [...document.querySelectorAll('figure svg[data-analytics-legend-swatch]')].map(s => ({hidden: s.getAttribute('aria-hidden'), role: s.closest('li')?.textContent.trim()})),
      firstRowLineTops: cellLines.map(rects => rects[0]?.top ?? null),
      firstRowLabel: {lines: cellLines[0]?.length ?? 0, textOverflow: labelStyle?.textOverflow, whiteSpace: labelStyle?.whiteSpace, overflowWrap: labelStyle?.overflowWrap,
        clientWidth: firstRow?.cells[0]?.clientWidth, scrollWidth: firstRow?.cells[0]?.scrollWidth},
      region: region ? {clientWidth: region.clientWidth, scrollWidth: region.scrollWidth, scrollLeft: region.scrollLeft} : null};
  });
}

async function focusAndScrollTable(page, name) {
  for (let i = 0; i < 80; i++) {
    if (await page.evaluate(() => document.activeElement?.matches('[role="region"]') && !!document.activeElement.querySelector('table'))) break;
    await page.keyboard.press('Tab');
  }
  const focused = await page.evaluate(() => document.activeElement?.matches('[role="region"]') && !!document.activeElement.querySelector('table'));
  check(focused, name + ': keyboard reaches exact-value table region');
  if (!focused) return;
  const before = await page.evaluate(() => ({left: document.activeElement.scrollLeft, width: document.activeElement.clientWidth, scroll: document.activeElement.scrollWidth}));
  for (let i = 0; i < 60; i++) { await page.keyboard.press('ArrowRight'); await page.waitForTimeout(10); }
  await page.waitForTimeout(250);
  const after = await page.evaluate(() => {
    const region = document.activeElement, last = region.querySelector('tbody tr td:last-child');
    const rr = region.getBoundingClientRect(), lr = last.getBoundingClientRect();
    return {left: region.scrollLeft, lastVisible: lr.left >= rr.left - 1 && lr.right <= rr.right + 1, region: {left: rr.left, right: rr.right}, last: {left: lr.left, right: lr.right}};
  });
  check(before.scroll <= before.width || (after.left > before.left && after.lastVisible), name + ': native keys expose final Meaning column', {before, after});
}

const {chromium} = await import(pathToFileURL(entry).href);
const browser = await chromium.launch({executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, args: ['--no-first-run', '--no-default-browser-check']});
report.browser = browser.version();
try {
  for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
    const lane = width + '-' + theme;
    const context = await browser.newContext({viewport: {width, height: 1000}, colorScheme: theme, reducedMotion: 'reduce', serviceWorkers: 'block', acceptDownloads: true});
    const pending = [];
    try {
      context.setDefaultTimeout(6000);
      await context.route('**/*', route => {
        const request = route.request(), url = new URL(request.url());
        if (url.origin === origin.origin && allowed.has(url.pathname) && ['GET', 'HEAD'].includes(request.method())) return route.continue();
        report.blocked.push({url: request.url(), method: request.method()}); return route.abort();
      });
      await context.routeWebSocket('**/*', ws => {report.blocked.push({websocket: ws.url()}); ws.close();});
      const page = await context.newPage();
      page.on('pageerror', error => report.errors.push({lane, kind: 'page', text: error.message}));
      page.on('console', message => {if (message.type() === 'error') report.errors.push({lane, kind: 'console', text: message.text()});});
      page.on('requestfailed', request => {
        const url = new URL(request.url());
        // Chrome aborts the navigation request after handing a successful
        // attachment to its download manager; the saved bytes are verified below.
        if (url.origin === origin.origin && url.pathname === '/analytics/recipe/export.csv' && request.failure()?.errorText === 'net::ERR_ABORTED') {
          report.downloadAborts.push({lane, url: request.url()}); return;
        }
        report.errors.push({lane, kind: 'request', url: request.url(), text: request.failure()?.errorText});
      });
      page.on('response', response => {
        if (response.headers()['content-disposition']?.includes('attachment')) return;
        pending.push((async () => {const bytes = await response.body(); report.requests.push({lane, url: response.url(), status: response.status(), bytes: bytes.length, sha256: sha(bytes)}); check(response.status() === 200, lane + ': admitted response succeeds', response.url());})().catch(error => report.errors.push({lane, kind: 'response', text: error.message})));
      });
      await (await context.newCDPSession(page)).send('Emulation.setScriptExecutionDisabled', {value: true});

      await page.goto(origin.origin + '/analytics/recipe?compare=none&theme=' + theme, {waitUntil: 'networkidle'});
      const normal = await tableState(page);
      check(JSON.stringify(normal.headers) === JSON.stringify(['Label','Value (failed operations)','Bounds (failed operations)','State','Meaning']), lane + ': visible Meaning header', normal.headers);
      check(JSON.stringify(normal.rows) === JSON.stringify(expectedCells), lane + ': production values and app-authored meanings agree', normal.rows);
      check(normal.svg === 1, lane + ': production renderer retains chart', normal.svg);
      check(normal.swatches.length === 2 && normal.swatches.every(s => s.hidden === 'true') && JSON.stringify(normal.swatches.map(s => s.role)) === JSON.stringify(['API services','Worker services']), lane + ': decorative legend swatches remain hidden from accessibility semantics', normal.swatches);
      await focusAndScrollTable(page, lane + '-production');

      const exportLink = page.locator('a[href^="/analytics/recipe/export.csv"]');
      const exportHref = await exportLink.getAttribute('href');
      const [download] = await Promise.all([page.waitForEvent('download'), exportLink.press('Enter')]);
      const csvPath = path.join(out, lane + '.csv');
      await download.saveAs(csvPath); fs.chmodSync(csvPath, 0o600);
      const csvBytes = fs.readFileSync(csvPath), parsed = parseCSV(csvBytes.toString('utf8'));
      report.downloads.push({url: new URL(exportHref, origin).href, path: path.basename(csvPath), bytes: csvBytes.length, sha256: sha(csvBytes)});
      check(parsed.context.length === 17 && parsed.header.length === 10 && parsed.data.length === 3, lane + ': CSV retains 17-context/10-column shape', {context: parsed.context.length, rows: parsed.data.length});
      check(JSON.stringify(parsed.data.map(r => r.slice(0, 8))) === JSON.stringify(completeFirstEight), lane + ': CSV first eight fields retain independent values', parsed.data);
      check(JSON.stringify(parsed.data.map(r => r.slice(8))) === JSON.stringify(meanings), lane + ': CSV binds exact role and label', parsed.data);

      await page.goto(origin.origin + '/analytics/recipe?compare=none&state=missing&theme=' + theme, {waitUntil: 'networkidle'});
      const missing = await tableState(page);
      check(missing.svg === 0 && missing.rows.length === 3 && missing.rows.every((r, i) => r[1] === 'Missing' && r[2] === 'Not provided' && r[3].includes('missing is not zero') && r[4] === expectedCells[i][4]), lane + ': missing remains missing with textual meanings', missing.rows);

      await page.goto(origin.origin + '/__test/analytics-roles/long?theme=' + theme, {waitUntil: 'networkidle'});
      const fallback = await tableState(page);
      check(fallback.svg === 0 && fallback.swatches.length === 0 && fallback.rows.length === 3, lane + ': long label triggers explicit table-only fallback without orphan swatches', fallback);
      check(JSON.stringify(fallback.rows.map(r => r[4])) === JSON.stringify(['API services','API services','Worker services']), lane + ': fallback preserves per-row Meaning', fallback.rows);
      const labelTop = fallback.firstRowLineTops[0];
      check(fallback.firstRowLineTops.length === 5 && fallback.firstRowLineTops.every(top => top !== null && Math.abs(top - labelTop) <= 1), lane + ': every first-row value starts beside the wrapped label first line', fallback.firstRowLineTops);
      check(fallback.firstRowLabel.lines > 1 && fallback.firstRowLabel.textOverflow !== 'ellipsis' && fallback.firstRowLabel.whiteSpace === 'normal' && fallback.firstRowLabel.scrollWidth <= fallback.firstRowLabel.clientWidth + 1, lane + ': long label wraps without ellipsis or internal clipping', fallback.firstRowLabel);
      await focusAndScrollTable(page, lane + '-fallback');
      const pngPath = path.join(out, lane + '-fallback.png');
      await page.screenshot({path: pngPath, fullPage: true}); fs.chmodSync(pngPath, 0o600);
      const png = fs.readFileSync(pngPath); report.screenshots.push({path: path.basename(pngPath), bytes: png.length, sha256: sha(png)});
      check(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), lane + ': fallback has no page-level horizontal overflow');
      await Promise.all(pending);
    } catch (error) {
      report.failures.push({name: lane + ': task completes', detail: error.stack});
    } finally {
      await context.close();
    }
  }
  check(report.screenshots.length === 4, 'Four bounded fallback screenshots retained', report.screenshots.length);
  check(report.downloadAborts.every(a => report.downloads.some(d => new URL(d.url, origin).href === a.url)), 'Every expected Chrome download abort has verified saved bytes', report.downloadAborts);
  check(report.blocked.length === 0, 'No external, write, or socket requests', report.blocked);
  check(report.errors.length === 0, 'No browser/resource errors', report.errors);
} finally {
  await browser.close();
  report.finished = new Date().toISOString();
  const receipt = path.join(out, 'receipt.json');
  fs.writeFileSync(receipt, JSON.stringify(report, null, 2), {flag: 'wx', mode: 0o600});
  console.log(JSON.stringify({out, passes: report.passes.length, failures: report.failures.length, receiptSHA256: sha(fs.readFileSync(receipt))}));
}
if (report.failures.length) process.exitCode = 1;
