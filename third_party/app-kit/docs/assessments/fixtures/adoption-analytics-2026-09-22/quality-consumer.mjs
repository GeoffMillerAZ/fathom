// Native inspection of the authored RAC adoption record, not a consumer execution.
// Args: installed Playwright module, Chrome, exact loopback origin, new output dir.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [pw, chrome, origin, output] = process.argv.slice(2);
const url = new URL(origin);
assert.equal(url.protocol, 'http:'); assert.equal(url.hostname, '127.0.0.1');
assert.equal(url.origin, origin); assert.ok(path.isAbsolute(output));
const report = { scope: 'root-kit authored consumer metadata; no consumer task or certification', node: process.version, checks: [], blocked: [], failures: [] };
let browser, page, ownsOutput = false;
function check(name, actual, expected = true) {
  assert.deepEqual(actual, expected, name); report.checks.push(name);
}
try {
  await fs.mkdir(output, { mode: 0o700 }); ownsOutput = true;
  const { chromium } = await import(pathToFileURL(pw).href);
  browser = await chromium.launch({ executablePath: chrome, headless: true });
  report.browser = browser.version();
  report.playwright = JSON.parse(await fs.readFile(path.join(path.dirname(pw), 'package.json'), 'utf8')).version;
  for (const [width, height, theme] of [[1440, 1000, 'light'], [390, 844, 'dark']]) {
    const context = await browser.newContext({ viewport: { width, height }, colorScheme: theme, reducedMotion: 'reduce', javaScriptEnabled: false });
    await context.route('**/*', route => {
      if (new URL(route.request().url()).origin === origin) return route.continue();
      report.blocked.push(route.request().url()); return route.abort();
    });
    page = await context.newPage(); page.setDefaultTimeout(10000);
    page.on('pageerror', e => report.failures.push(String(e)));
    page.on('requestfailed', r => report.failures.push(r.url() + ': ' + r.failure()?.errorText));
    const response = await page.goto(origin + '/quality/core.analytics?theme=' + theme, { waitUntil: 'load' });
    const prefix = width + '/' + theme + ': ';
    check(prefix + 'detail response', response.status(), 200);
    const region = page.locator('section[aria-labelledby="quality-consumers"]');
    check(prefix + 'scoped consumer visible', await region.getByText('resume-as-code: verified-scoped', { exact: true }).isVisible());
    check(prefix + 'not-shipped boundary visible', await region.getByText(/not a shipped app/).isVisible());
    check(prefix + 'exact executed artifact retained', (await region.innerText()).includes('c727501e305a091802070e18761d6c8c0ceea8b9efe627be63edb79a86c1e831'));
    const details = region.locator('details');
    const summary = details.locator('summary');
    check(prefix + 'metadata starts collapsed', await details.getAttribute('open'), null);
    let reached = false;
    for (let i = 0; i < 64; i++) {
      await page.keyboard.press('Tab');
      if (await summary.evaluate(el => document.activeElement === el)) { reached = true; break; }
    }
    check(prefix + 'native Tab reaches disclosure', reached);
    const box = await summary.boundingBox();
    check(prefix + 'disclosure owns 44px target', box !== null && box.height >= 44 && box.x >= 0 && box.x + box.width <= width + 1);
    check(prefix + 'keyboard focus visible', await summary.evaluate(el => { const s = getComputedStyle(el); return s.outlineStyle !== 'none' && parseFloat(s.outlineWidth) > 0; }));
    await page.keyboard.press('Enter');
    check(prefix + 'Enter opens metadata', await details.getAttribute('open') !== null);
    check(prefix + 'receipt hash visible', await details.getByText(/1a80513a401bf38181d4ff6ea0f41736171ccbc7b7a7fc31b797ef9217bedbdc/).isVisible());
    check(prefix + 'review identity visible', await details.getByText(/Codex integration and independent scoped evidence review/).isVisible());
    check(prefix + 'metadata is not a source-file link', await details.locator('a').count(), 0);
    check(prefix + 'no horizontal page overflow', await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
    await summary.scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(output, width + '-' + theme + '.png') });
    await page.keyboard.press('Enter');
    check(prefix + 'Enter closes metadata', await details.getAttribute('open'), null);
    check(prefix + 'focus remains on disclosure', await summary.evaluate(el => document.activeElement === el));
    await context.close(); page = null;
  }
  check('no external requests', report.blocked.length, 0);
  check('no page or network failures', report.failures.length, 0);
  report.passed = true;
} catch (error) {
  report.passed = false; report.error = String(error.stack || error); process.exitCode = 1;
  if (page && ownsOutput) await page.screenshot({ path: path.join(output, 'failure.png') }).catch(() => {});
} finally {
  try { await browser?.close(); } catch (error) { report.browserCleanupError = String(error); report.passed = false; process.exitCode = 1; }
  try {
    const stop = await fetch(origin + '/__fixture/stop', { method: 'POST', headers: { 'X-Fixture-Token': 'synthetic-session' }, redirect: 'error', signal: AbortSignal.timeout(3000) });
    report.fixtureStopped = stop.status === 204;
    if (!report.fixtureStopped) { report.passed = false; process.exitCode = 1; }
  } catch (error) { report.cleanupError = String(error); report.passed = false; process.exitCode = 1; }
  if (ownsOutput) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify(report, null, 2));
}
