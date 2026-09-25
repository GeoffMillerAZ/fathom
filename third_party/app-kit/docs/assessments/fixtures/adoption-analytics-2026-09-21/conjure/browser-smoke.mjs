// Real Conjure handler/embedded assets, synthetic Deps, fresh browser only.
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
const token = 'synthetic-session';
const report = { origin, scope: 'synthetic status-read workflow; not full dashboard or human acceptance', checks: [], errors: [], blocked: [], geometry: [] };
const check = (name, condition) => {
  report.checks.push({ name, passed: Boolean(condition) });
  assert.ok(condition, name);
};
async function control(endpoint, method = 'GET') {
  const response = await fetch(origin + '/__fixture/' + endpoint, {
    method, headers: { 'X-Fixture-Token': token }, redirect: 'error', signal: AbortSignal.timeout(3000),
  });
  assert.ok(response.ok, `fixture ${endpoint}: HTTP ${response.status}`);
  return response.status === 204 ? undefined : response.json();
}
async function streams(page, count) {
  await page.waitForFunction(async ({ token, count }) => {
    const response = await fetch('/__fixture/status', { headers: { 'X-Fixture-Token': token } });
    return response.ok && (await response.json()).active_streams === count;
  }, { token, count });
}
const fragments = ['dash-stats', 'dash-feed', 'dash-machine', 'dash-store', 'dash-doctor', 'dash-runs'];
const snapshot = page => page.evaluate(ids => Object.fromEntries(ids.map(id => [id, document.getElementById(id).outerHTML])), fragments);
async function waitText(page, id, text) {
  await page.waitForFunction(({ id, text }) => document.getElementById(id)?.textContent.includes(text), { id, text });
}
async function geometry(page, lane, state) {
  const result = await page.evaluate(() => ({ width: innerWidth, scroll: document.documentElement.scrollWidth }));
  report.geometry.push({ lane, state, ...result });
  check(`${lane}: ${state} fits viewport`, result.scroll <= result.width + 1);
}
let browser;
let page;
let ownsOutput = false;
try {
  await fs.mkdir(output); // Never overwrite a prior attempt.
  ownsOutput = true;
  const { chromium } = await import(pathToFileURL(pwPath).href);
  assert.equal((await control('status')).active_streams, 0, 'exclusive fixture required');
  // Two fixed lanes; actual five-second production polling, no timer/DOM mocks.
  browser = await chromium.launch({ executablePath: chrome, headless: true });
  report.browser = browser.version();
  for (const [lane, width, theme] of [['desktop-light', 1440, 'light'], ['mobile-dark', 390, 'dark']]) {
    await control('state?value=failed', 'POST');
    const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme: theme, reducedMotion: 'reduce' });
    await context.addCookies([{ name: 'conjure_ui_theme', value: theme, url: origin }]);
    await context.route('**/*', route => {
      if (new URL(route.request().url()).origin === origin) return route.continue();
      report.blocked.push(route.request().url());
      return route.abort();
    });
    page = await context.newPage();
    page.setDefaultTimeout(12000);
    page.on('pageerror', error => report.errors.push({ lane, message: error.message }));
    page.on('dialog', async dialog => { report.errors.push({ lane, message: 'unexpected dialog: ' + dialog.message() }); await dialog.dismiss(); });
    await page.goto(origin + '/?token=' + token, { waitUntil: 'load' });
    await streams(page, 1);
    await waitText(page, 'dash-stats', 'Status unavailable; no data yet.');
    check(`${lane}: explicit theme`, await page.locator('html').getAttribute('data-theme') === theme);
    check(`${lane}: stylesheet loaded`, await page.evaluate(() => Array.from(document.styleSheets).some(sheet => sheet.href?.endsWith('/static/app.css') && sheet.cssRules.length > 0)));
    check(`${lane}: failed read is not empty or converged`, !/No units declared|Estate is fully converged|No divergence detected/.test(await page.locator('#page').innerText()));
    check(`${lane}: error is text, not executable markup`, await page.locator('#dash-banner script').count() === 0 && (await page.locator('#dash-banner').innerText()).includes("<script>alert('fixture')</script>"));
    check(`${lane}: polite status announcement`, await page.locator('#dash-banner').getAttribute('role') === 'status' && await page.locator('#dash-banner').getAttribute('aria-live') === 'polite');
    await geometry(page, lane, 'first-failure');
    await page.screenshot({ path: path.join(output, lane + '-first-failure.png'), fullPage: true });
    await page.keyboard.press('Tab');
    check(`${lane}: keyboard reaches a visible control`, await page.evaluate(() => {
      const element = document.activeElement;
      const bounds = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      return element.matches('a,button,input,select,summary,[tabindex]') && bounds.width > 0 && bounds.height > 0 && style.visibility === 'visible' && style.display !== 'none';
    }));

    await control('state?value=good', 'POST');
    await waitText(page, 'dash-stats', 'converged — 3 units');
    await page.waitForFunction(() => document.getElementById('dash-banner').textContent.trim() === '');
    check(`${lane}: live success clears error`, (await page.locator('#dash-banner').innerText()).trim() === '');
    const before = await snapshot(page);
    await control('state?value=failed', 'POST');
    await waitText(page, 'dash-banner', 'showing last good data from 13:04:05');
    check(`${lane}: failed refresh retains every data fragment`, JSON.stringify(before) === JSON.stringify(await snapshot(page)));
    check(`${lane}: patch roots remain unique`, await page.evaluate(ids => ids.every(id => document.querySelectorAll('[id="' + id + '"]').length === 1), ['dash-banner', ...fragments]));
    await geometry(page, lane, 'stale');
    await page.screenshot({ path: path.join(output, lane + '-stale.png'), fullPage: true });

    const fresh = await context.newPage();
    fresh.setDefaultTimeout(12000);
    fresh.on('pageerror', error => report.errors.push({ lane, message: error.message }));
    await fresh.addInitScript(() => {
      window.fixturePatchSelectors = [];
      document.addEventListener('datastar-fetch', event => {
        if (event.detail?.type === 'datastar-patch-elements') window.fixturePatchSelectors.push(event.detail.argsRaw.selector);
      });
    });
    await fresh.goto(origin + '/', { waitUntil: 'load' });
    await streams(fresh, 2);
    await fresh.waitForFunction(() => ['#dash-banner', '#dash-stats', '#dash-runs'].every(selector => window.fixturePatchSelectors.includes(selector)));
    check(`${lane}: new stream does not inherit old data`, (await fresh.locator('#dash-stats').innerText()).includes('Status unavailable; no data yet.') && !(await fresh.locator('#dash-banner').innerText()).includes('showing last good data'));
    await fresh.close();
    await streams(page, 1);
    await control('state?value=recovered', 'POST');
    await waitText(page, 'dash-stats', 'converged — 5 units');
    await page.waitForFunction(() => document.getElementById('dash-banner').textContent.trim() === '');
    check(`${lane}: recovery updates count and clears error`, (await page.locator('#dash-banner').innerText()).trim() === '');
    await geometry(page, lane, 'recovered');
    await page.screenshot({ path: path.join(output, lane + '-recovered.png'), fullPage: true });
    await control('state?value=empty', 'POST');
    await waitText(page, 'dash-stats', 'No units declared');
    check(`${lane}: successful empty is distinct`, !(await page.locator('#dash-stats').innerText()).includes('Status unavailable'));
    await control('state?value=orphan', 'POST');
    await waitText(page, 'dash-feed', 'file:old-zshrc');
    check(`${lane}: orphan cannot claim full convergence`, !(await page.locator('#dash-stats').innerText()).includes('Estate is fully converged'));
    await context.close();
    page = undefined;
    // Observe cleanup through a fresh API-only context, without opening a stream.
    const probe = await browser.newContext();
    const observer = await probe.newPage();
    await observer.goto(origin + '/__fixture/status?token=' + token);
    await streams(observer, 0);
    check(`${lane}: closing clients releases all streams`, (await control('status')).active_streams === 0);
    await probe.close();
  }
  check('no external requests', report.blocked.length === 0);
  check('no page script errors or injected dialogs', report.errors.length === 0);
  report.status = 'pass';
} catch (error) {
  report.status = 'fail';
  report.failure = String(error.stack || error);
  if (page && !page.isClosed()) await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true }).catch(() => {});
  process.exitCode = 1;
} finally {
  if (browser) {
    try { await browser.close(); }
    catch (error) { report.browserCleanupError = String(error); process.exitCode = 1; report.status = 'fail'; }
  }
  try {
    report.finalFixture = await control('status');
    await control('stop', 'POST');
  } catch (error) { report.cleanupError = String(error); process.exitCode = 1; report.status = 'fail'; }
  if (ownsOutput) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify(report, null, 2));
}
