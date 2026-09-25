// Bounded candidate-only task probe. Never uses a signed-in browser profile.
// Usage: node browser-smoke.mjs <playwright-module> <chrome> <origin> <new-output-dir>
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [pwPath, chrome, origin, output] = process.argv.slice(2);
const url = new URL(origin);
assert.equal(url.protocol, 'http:');
assert.equal(url.hostname, '127.0.0.1');
assert.equal(url.origin, origin);
assert.ok(output && path.isAbsolute(output));
await fs.mkdir(output); // Refuse an existing evidence destination.
const { chromium } = await import(pathToFileURL(pwPath).href);
const results = { origin, scope: 'synthetic guide only; not complete G3/G4 acceptance', checks: [], errors: [], blocked: [] };
let browser;
try {
  // Refuse an unready server before paying for a browser acquisition. The
  // caller owns the candidate process; this probe never launches a daemon.
  const ready = await fetch(origin + '/dev/theme', { redirect: 'error', signal: AbortSignal.timeout(10000) });
  results.readinessStatus = ready.status;
  assert.equal(ready.status, 200, 'candidate must be listening before browser launch');
  await ready.arrayBuffer();
  browser = await chromium.launch({ executablePath: chrome, headless: true });
  results.browser = browser.version();
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce', acceptDownloads: true });
  await context.route('**/*', route => {
    const request = new URL(route.request().url());
    if (request.origin === origin) return route.continue();
    results.blocked.push(request.origin);
    return route.abort();
  });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  page.on('pageerror', err => results.errors.push(err.message));
  const check = (name, condition) => {
    results.checks.push({ name, passed: Boolean(condition) });
    assert.ok(condition, name);
  };
  const preview = async () => {
    const element = await page.locator('iframe.sg-preview').elementHandle();
    const frame = await element.contentFrame();
    await frame.waitForSelector('main');
    return frame;
  };
  const submitPreview = async () => {
    await Promise.all([
      page.waitForNavigation({ waitUntil: 'load' }),
      page.getByRole('button', { name: 'Preview candidate', exact: true }).click(),
    ]);
    return preview();
  };
  await page.goto(origin + '/dev/theme', { waitUntil: 'load' });
  check('guide heading', await page.getByRole('heading', { name: 'Fathom Observatory theme review', exact: true }).count() === 1);
  let frame = await preview();
  check('source specimen rendered', (await frame.locator('body').innerText()).includes('Synthetic'));
  check('preview operational links disabled', await frame.locator('a[href]').count() === 0);
  check('baseline light scheme', await frame.locator('html').getAttribute('data-theme') === 'light');
  await page.screenshot({ path: path.join(output, 'baseline-light.png'), fullPage: true });

  await page.getByRole('combobox', { name: 'App specimen', exact: true }).selectOption('partial');
  await page.getByRole('combobox', { name: 'Scheme', exact: true }).selectOption('dark');
  await page.getByRole('combobox', { name: 'Preview width', exact: true }).selectOption('narrow');
  frame = await submitPreview();
  check('partial dark scheme', await frame.locator('html').getAttribute('data-theme') === 'dark');
  check('partial registry qualification visible', (await frame.locator('body').innerText()).includes('fleet coverage unavailable'));
  check('partial ledger qualification visible', (await frame.locator('body').innerText()).includes('ledger'));
  const geometry = await frame.evaluate(() => ({ viewport: innerWidth, scroll: document.documentElement.scrollWidth }));
  results.previewGeometry = geometry;
  check('narrow document does not overflow', geometry.scroll <= geometry.viewport + 1);
  await page.screenshot({ path: path.join(output, 'partial-dark-narrow.png'), fullPage: true });

  await page.getByText('Color roles', { exact: true }).click();
  await page.locator('input[name="colors.ink.light"]').fill('not-a-color');
  await submitPreview();
  check('invalid token refused visibly', await page.getByRole('alert').count() === 1);
  check('invalid candidate cannot export', await page.getByRole('button', { name: 'Export acknowledged candidate ZIP', exact: true }).isDisabled());
  await page.getByRole('link', { name: 'Reset to source', exact: true }).click();
  await page.waitForLoadState('load');
  check('reset clears error', await page.getByRole('alert').count() === 0);
  check('reset clears override', await page.locator('input[name="colors.ink.light"]').inputValue() === '');
  await page.getByText('Corner radii', { exact: true }).click();
  await page.locator('input[name="radius.md"]').fill('0.75rem');
  await submitPreview();
  const exportButton = page.getByRole('button', { name: 'Export acknowledged candidate ZIP', exact: true });
  check('reviewed valid candidate can export', await exportButton.isEnabled());
  const [download] = await Promise.all([page.waitForEvent('download'), exportButton.click()]);
  const zip = path.join(output, 'candidate.zip');
  await download.saveAs(zip);
  check('native ZIP download', (await fs.stat(zip)).size > 0);
  check('no external requests', results.blocked.length === 0);
  check('no page script errors', results.errors.length === 0);
  results.status = 'pass';
} catch (error) {
  results.status = 'fail';
  results.failure = String(error.stack || error);
  process.exitCode = 1;
} finally {
  if (browser) await browser.close();
  await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(results, null, 2) + '\n');
  console.log(JSON.stringify(results, null, 2));
}
