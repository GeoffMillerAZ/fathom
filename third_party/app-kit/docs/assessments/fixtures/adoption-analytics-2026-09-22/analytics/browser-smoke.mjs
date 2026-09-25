// Synthetic-only real Gallery recipe. Usage: node browser-smoke.mjs <playwright-module> <chrome> <origin> <fresh-output-dir>
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [playwrightPath, chromePath, origin, output] = process.argv.slice(2);
const parsed = new URL(origin);
assert.equal(parsed.origin, origin);
assert.equal(parsed.protocol, 'http:');
assert.equal(parsed.hostname, '127.0.0.1');
assert.ok(path.isAbsolute(output));
const report = { origin, checks: [], screenshots: [], blocked: [], pageErrors: [] };
const check = (name, condition) => {
  report.checks.push({ name, passed: Boolean(condition) });
  assert.ok(condition, name);
};

// Parse the fixed WriteCSV contract, including its quoted context strings.
function csvRecords(input) {
  const records = [];
  let cells = [], cell = '', quoted = false;
  for (let i = 0; i < input.length; i++) {
    const char = input[i];
    if (quoted) {
      if (char === '"' && input[i + 1] === '"') { cell += '"'; i++; }
      else if (char === '"') quoted = false;
      else cell += char;
    } else if (char === '"') quoted = true;
    else if (char === ',') { cells.push(cell); cell = ''; }
    else if (char === '\n') { cells.push(cell.replace(/\r$/, '')); records.push(cells); cells = []; cell = ''; }
    else cell += char;
  }
  if (quoted) throw new Error('unterminated CSV quote');
  if (cell || cells.length) { cells.push(cell); records.push(cells); }
  return records;
}

let browser;
let ownsOutput = false;
try {
  await fs.mkdir(output);
  ownsOutput = true;
  const { chromium } = await import(pathToFileURL(playwrightPath).href);
  browser = await chromium.launch({ executablePath: chromePath, headless: true });
  report.browser = browser.version();
  for (const [lane, width, theme] of [['desktop-light', 1440, 'light'], ['mobile-dark', 390, 'dark']]) {
    const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme: theme, reducedMotion: 'reduce', acceptDownloads: true });
    await context.route('**/*', route => {
      if (new URL(route.request().url()).origin === origin) return route.continue();
      report.blocked.push(route.request().url());
      return route.abort();
    });
    const page = await context.newPage();
    page.setDefaultTimeout(12000);
    page.on('pageerror', error => report.pageErrors.push({ lane, message: error.message }));
    page.on('dialog', async dialog => { report.pageErrors.push({ lane, message: 'unexpected dialog: ' + dialog.message() }); await dialog.dismiss(); });
    for (const state of ['partial', 'complete']) {
      const selection = `period=2026-09&compare=none&category=all&state=${state}&theme=${theme}`;
      const response = await page.goto(`${origin}/analytics/recipe?${selection}`, { waitUntil: 'load' });
      check(`${lane}/${state}: HTTP`, response.status() === 200);
      check(`${lane}/${state}: explicit theme`, await page.locator('html').getAttribute('data-theme') === theme);
      check(`${lane}/${state}: CSS loaded`, await page.evaluate(() => Array.from(document.styleSheets).some(s => s.href?.endsWith('/static/app.css') && s.cssRules.length > 0)));
      const legend = await page.locator('ul[aria-label="Chart legend"] li').allTextContents();
      check(`${lane}/${state}: plotted legend`, JSON.stringify(legend.map(s => s.trim())) === JSON.stringify(state === 'partial' ? ['Scheduled'] : ['Recorded', 'Scheduled']));
      const table = page.locator('table');
      const rows = await table.locator('tbody tr').allTextContents();
      check(`${lane}/${state}: three selected rows`, rows.length === 3 && rows[0].includes('Measured zero') && rows[1].includes('Other service') && rows[2].includes('Recorded service'));
      check(`${lane}/${state}: zero and missing distinct`, rows[0].includes('0') && rows[0].includes('Present') && (state === 'partial' ? rows[2].includes('Missing') && rows[2].includes('Source unavailable') : rows[2].includes('3') && rows[2].includes('Present')));
      const chartTexts = await page.locator('svg[aria-label^="Failed service operations"] text').allTextContents();
      check(`${lane}/${state}: measured plot excludes missing`, chartTexts.includes('Measured zero') && chartTexts.includes('Other service') && (state === 'partial' ? !chartTexts.includes('Recorded service') : chartTexts.includes('Recorded service')));
      await page.keyboard.press('Tab');
      check(`${lane}/${state}: visible keyboard control`, await page.evaluate(() => {
        const el = document.activeElement, box = el.getBoundingClientRect(), style = getComputedStyle(el);
        return el.matches('a,button,input,select,summary,[tabindex]') && box.width > 0 && box.height > 0 && style.visibility === 'visible' && style.display !== 'none';
      }));
      const geometry = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth }));
      check(`${lane}/${state}: no page overflow`, geometry.document <= geometry.viewport + 1);
      const screenshot = `${lane}-${state}.png`;
      await page.screenshot({ path: path.join(output, screenshot), fullPage: true });
      report.screenshots.push({ file: screenshot, ...geometry });
      const [download] = await Promise.all([
        page.waitForEvent('download'),
        page.getByRole('link', { name: 'Export renderer CSV' }).click(),
      ]);
      check(`${lane}/${state}: native CSV download`, download.suggestedFilename() === 'service-operations-recipe-2026-09.csv');
      const records = csvRecords(await fs.readFile(await download.path(), 'utf8'));
      const contextRows = Object.fromEntries(records.slice(0, 17).map(row => [row[0], row[1]]));
      check(`${lane}/${state}: CSV context`, records.length === 22 && contextRows.completeness === state && contextRows.source?.includes('Synthetic Gallery operating ledger') && contextRows.coverage_note?.includes(state === 'partial' ? 'lower bounds' : '') && records[18].join(',') === 'id,label,value,unit,state,missing_reason,lower,upper,role,role_label');
      const data = records.slice(19);
      check(`${lane}/${state}: CSV values and roles`, data.length === 3 && data[0][0] === 'zero' && data[0][2] === '0' && data[0][4] === 'present' && data[0][8] === 'accent' && data[1][2] === '7' && data[2][8] === 'danger' && data[2][9] === 'Recorded' && (state === 'partial' ? data[2][2] === '' && data[2][4] === 'missing' && data[2][5] === 'Source unavailable' : data[2][2] === '3' && data[2][4] === 'present'));
    }
    await context.close();
  }
  check('no external requests', report.blocked.length === 0);
  check('no page script errors', report.pageErrors.length === 0);
  report.status = 'pass';
} catch (error) {
  report.status = 'fail';
  report.failure = String(error.stack || error);
  process.exitCode = 1;
} finally {
  if (browser) {
    try { await browser.close(); }
    catch (error) { report.browserCleanupError = String(error); report.status = 'fail'; process.exitCode = 1; }
  }
  try {
    const response = await fetch(origin + '/__fixture/stop', {
      method: 'POST', headers: { 'X-Fixture-Token': 'synthetic-session' }, signal: AbortSignal.timeout(3000), redirect: 'error',
    });
    assert.equal(response.status, 204);
    report.fixtureStopped = true;
  } catch (error) { report.fixtureStopError = String(error); report.status = 'fail'; process.exitCode = 1; }
  if (ownsOutput) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify(report, null, 2));
}
