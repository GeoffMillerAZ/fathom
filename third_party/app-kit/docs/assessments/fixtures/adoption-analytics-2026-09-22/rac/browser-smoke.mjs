// Native CSV through actual RAC Pipeline/Handler, synthetic temporary stores only.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [pwPath, chrome, origin, output] = process.argv.slice(2);
assert.equal(new URL(origin).origin, origin);
assert.equal(new URL(origin).hostname, '127.0.0.1');
assert.equal(new URL(origin).protocol, 'http:');
assert.ok(path.isAbsolute(output));
const report = { scope: 'synthetic native Pipeline channel CSV; not auth, full-app, human or release acceptance', checks: [], blocked: [] };
function check(name, value) { report.checks.push({ name, passed: !!value }); assert.ok(value, name); }
function csv(text) {
  const rows = []; let row = [], field = '', quoted = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (c === '"') {
      if (quoted && text[i + 1] === '"') { field += '"'; i++; }
      else quoted = !quoted;
    } else if (c === ',' && !quoted) { row.push(field); field = ''; }
    else if (c === '\n' && !quoted) { row.push(field.replace(/\r$/, '')); rows.push(row); row = []; field = ''; }
    else field += c;
  }
  assert.equal(quoted, false);
  assert.equal(field, '');
  assert.equal(row.length, 0);
  return rows;
}
let browser, page, ownsOutput = false;
try {
  await fs.mkdir(output);
  ownsOutput = true;
  const { chromium } = await import(pathToFileURL(pwPath).href);
  browser = await chromium.launch({ executablePath: chrome, headless: true });
  report.browser = browser.version();
  for (const [lane, width, colorScheme] of [['desktop-light', 1440, 'light'], ['mobile-dark', 390, 'dark']]) {
    const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme, javaScriptEnabled: false, reducedMotion: 'reduce', acceptDownloads: true });
    await context.route('**/*', route => {
      if (new URL(route.request().url()).origin === origin) return route.continue();
      report.blocked.push(route.request().url()); return route.abort();
    });
    page = await context.newPage(); page.setDefaultTimeout(10000);
    for (const [state, status, applied, rate, ui] of [
      ['coherent', 'ready', '5', '0%', 'Response: 0% (0/5).'],
      ['incomplete', 'incomplete-sent-evidence', '5', 'rate unavailable: outcome missing applied evidence', 'Rates unavailable: incomplete sent evidence.'],
      ['zero', 'ready', '5', '0%', 'Response: 0% (0/5).'],
      ['empty', 'no-applications', '0', 'not enough outcomes yet (n=0, need 5)', 'No application history yet.'],
      ['no-sent', 'no-sent-evidence', '0', 'not enough outcomes yet (n=0, need 5)', 'missing sent evidence'],
    ]) {
      const label = `${lane}/${state}`;
      const response = await page.goto(`${origin}/pipeline?fixture_case=${state}`, { waitUntil: 'load' });
      check(`${label}: actual page`, response.status() === 200);
      check(`${label}: embedded CSS`, await page.evaluate(() => Array.from(document.styleSheets).some(s => s.href?.endsWith('/assets/app.css') && s.cssRules.length > 0)));
      const section = page.locator('section[aria-labelledby="pipeline-analytics-heading"]');
      if (state !== 'empty') {
        const summary = page.getByText('Rates, sample sizes and sources', { exact: true });
        await summary.click();
        check(`${label}: native rate disclosure opens`, await summary.evaluate(e => e.parentElement.open));
      }
      check(`${label}: truth state visible`, (await section.innerText()).includes(ui));
      const link = page.getByRole('link', { name: 'Download channel-yield CSV', exact: true });
      await link.scrollIntoViewIfNeeded();
      check(`${label}: native export target`, (await link.getAttribute('href')) === '/api/export?format=csv&view=channel-yield');
      // Reach the native link with Tab, not programmatic focus/click or request emulation.
      for (let i = 0; i < 100 && !(await link.evaluate(e => e === document.activeElement)); i++) await page.keyboard.press('Tab');
      check(`${label}: keyboard reaches export`, await link.evaluate(e => e === document.activeElement));
      const pending = page.waitForEvent('download');
      await page.keyboard.press('Enter');
      const download = await pending;
      check(`${label}: native download`, download.suggestedFilename() === 'channel-yield.csv' && (await download.failure()) === null);
      const file = path.join(output, `${label.replace('/', '-')}.csv`);
      await download.saveAs(file);
      const rows = csv(await fs.readFile(file, 'utf8'));
      check(`${label}: aggregate shape`, rows.length === 5 && rows.every(r => r.length === 16) && rows[0][0] === 'channel');
      check(`${label}: independent count/rate/state`, rows[1][0] === 'cold-apply' && rows[1][2] === applied && rows[1][7] === rate && rows[1][9] === status);
      check(`${label}: all rows disclose scope and freshness`, rows.slice(1).every(r => r[10] === '5' && r[11] === 'stored application registry' && r[12] === 'all recorded applications; no date filter; closed included' && r[13] === 'source freshness not independently verified'));
      if (state === 'coherent') {
        check(`${label}: all channel results`, JSON.stringify(rows.slice(1).map(r => [r[0], r[2], r[3], r[4], r[7], r[8]])) === JSON.stringify([
          ['cold-apply', '5', '0', '0', '0%', '0%'], ['outreach', '5', '1', '0', '20%', '0%'], ['referral', '5', '2', '1', '40%', '20%'], ['firm', '5', '3', '2', '60%', '40%'],
        ]));
      }
      const bounds = await section.boundingBox();
      check(`${label}: analytical region fits viewport`, bounds && bounds.x >= -1 && bounds.x + bounds.width <= width + 1);
      if (state === 'coherent' || state === 'incomplete') await section.screenshot({ path: path.join(output, `${lane}-${state}.png`) });
    }
    await context.close();
  }
  check('no external requests', report.blocked.length === 0);
  report.taskPassed = true;
  report.passed = true;
} catch (error) {
  report.passed = false; report.error = String(error.stack || error); process.exitCode = 1;
  if (page && ownsOutput) await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true }).catch(() => {});
} finally {
  await browser?.close();
  try {
    const stopped = await fetch(origin + '/__fixture/stop', { method: 'POST', headers: { 'X-Fixture-Token': 'synthetic-session' }, redirect: 'error', signal: AbortSignal.timeout(3000) });
    report.fixtureStopped = stopped.status === 204;
    if (!report.fixtureStopped) { report.passed = false; process.exitCode = 1; }
  } catch (error) { report.cleanupError = String(error); report.passed = false; process.exitCode = 1; }
  if (ownsOutput) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify(report, null, 2));
}
