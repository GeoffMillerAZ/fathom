// Diagnostic: observe native input/submit without substituting requestSubmit/fetch.
// Args: Playwright module, installed Chrome, exact loopback origin, new output dir.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import { pathToFileURL } from 'node:url';

const [pw, chrome, origin, output] = process.argv.slice(2);
const target = new URL(origin);
assert.equal(target.protocol, 'http:'); assert.equal(target.hostname, '127.0.0.1');
assert.equal(target.origin, origin); assert.ok(path.isAbsolute(output));
const report = { scope: 'root-kit synthetic native-export diagnostic, not consumer adoption', events: [], requests: [], responses: [], failures: [], blocked: [], attempts: [] };
const now = () => Date.now();
let browser, page, ownsOutput = false;
async function state() {
  return page.locator('button[value="export"]').evaluate(b => {
    const f = b.form, r = b.getBoundingClientRect();
    return { ownerMatches: f === document.querySelector('form'), contained: f?.contains(b), type: b.type, disabled: b.disabled, actionAttribute: f?.getAttribute('action'), method: f?.method, actionPropertyType: typeof f?.action, digest: f?.elements.namedItem('review_digest')?.value, invalid: f ? Array.from(f.elements).filter(e => e.willValidate && !e.validity.valid).map(e => ({ name: e.name, message: e.validationMessage })) : null, bounds: { x: r.x, y: r.y, w: r.width, h: r.height }, centerTarget: document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2)?.outerHTML.slice(0, 500) };
  });
}
try {
  await fs.mkdir(output); ownsOutput = true;
  const { chromium } = await import(pathToFileURL(pw).href);
  browser = await chromium.launch({ executablePath: chrome, headless: true });
  report.browser = browser.version();
  const context = await browser.newContext({ acceptDownloads: true, viewport: { width: 1440, height: 1000 }, colorScheme: 'light', reducedMotion: 'reduce' });
  await context.route('**/*', route => {
    if (new URL(route.request().url()).origin === origin) return route.continue();
    report.blocked.push(route.request().url()); return route.abort();
  });
  page = await context.newPage(); page.setDefaultTimeout(10000);
  page.on('console', m => {
    if (m.text().startsWith('APPKIT_NATIVE_EVENT ')) {
      try { report.events.push({ observedAt: now(), ...JSON.parse(m.text().slice(20)) }); } catch (e) { report.failures.push({ phase: 'event decode', error: String(e), text: m.text() }); }
    } else if (m.type() === 'error') report.failures.push({ phase: 'console', text: m.text() });
  });
  page.on('pageerror', e => report.failures.push({ phase: 'page', error: String(e) }));
  page.on('request', req => {
    if (req.method() === 'POST') report.requests.push({ at: now(), url: req.url(), action: new URLSearchParams(req.postData()).get('action'), origin: req.headers().origin });
  });
  page.on('requestfailed', req => report.failures.push({ phase: 'network', at: now(), url: req.url(), method: req.method(), error: req.failure()?.errorText }));
  page.on('response', response => {
    if (response.request().method() === 'POST') report.responses.push({ at: now(), status: response.status(), action: new URLSearchParams(response.request().postData()).get('action'), headers: response.headers() });
  });
  await page.addInitScript(() => {
    for (const type of ['pointerdown', 'pointerup', 'click', 'submit', 'invalid', 'formdata', 'securitypolicyviolation']) {
      document.addEventListener(type, event => {
        if ((type === 'pointerdown' || type === 'pointerup' || type === 'click') && !event.target.closest?.('button')) return;
        const button = event.submitter || event.target.closest?.('button');
        const record = { type, browserTime: performance.now(), trusted: event.isTrusted, target: event.target.tagName, buttonName: button?.name, buttonValue: button?.value, invalidName: type === 'invalid' ? event.target.name : undefined, action: type === 'formdata' ? event.formData.get('action') : undefined, directive: event.violatedDirective, blockedURI: event.blockedURI };
        queueMicrotask(() => console.info('APPKIT_NATIVE_EVENT ' + JSON.stringify({ ...record, defaultPrevented: event.defaultPrevented })));
      }, true);
    }
  });
  const initial = await page.goto(origin + '/dev/theme', { waitUntil: 'load' });
  assert.equal(initial.status(), 200);
  await page.getByText('Corner radii', { exact: true }).click();
  await page.locator('input[name="radius.md"]').fill('0.75rem');
  await Promise.all([page.waitForNavigation({ waitUntil: 'load' }), page.getByRole('button', { name: 'Preview candidate', exact: true }).click()]);
  const exportButton = page.getByRole('button', { name: 'Export acknowledged candidate ZIP', exact: true });
  await exportButton.scrollIntoViewIfNeeded();
  report.reviewed = await state();
  assert.equal(report.reviewed.ownerMatches, true); assert.equal(report.reviewed.type, 'submit');
  assert.equal(report.reviewed.disabled, false); assert.equal(report.reviewed.invalid.length, 0);
  assert.match(report.reviewed.digest, /^[a-f0-9]{64}$/);
  for (const activation of ['pointer', 'enter']) {
    const attempt = { activation, startedAt: now(), before: await state(), requestStart: report.requests.length, eventStart: report.events.length };
    // Only native pointer/key activation. No direct HTTP export or synthetic submit.
    const pending = page.waitForEvent('download', { timeout: 8000 }).then(value => ({ value }), error => ({ error: String(error) }));
    if (activation === 'pointer') await exportButton.click();
    else { await exportButton.focus(); await page.keyboard.press('Enter'); }
    const result = await pending;
    if (result.value) {
      const file = path.join(output, activation + '-candidate.zip');
      await result.value.saveAs(file);
      const bytes = await fs.readFile(file);
      attempt.download = { filename: result.value.suggestedFilename(), failure: await result.value.failure(), bytes: bytes.length, sha256: crypto.createHash('sha256').update(bytes).digest('hex'), zipMagic: bytes.subarray(0, 4).toString('hex') };
    } else attempt.error = result.error;
    attempt.after = await state(); attempt.endedAt = now();
    report.attempts.push(attempt);
  }
  await page.screenshot({ path: path.join(output, 'after-export.png'), fullPage: true });
  report.observationComplete = true;
} catch (error) {
  report.error = String(error.stack || error); process.exitCode = 1;
  if (page && ownsOutput) await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true }).catch(() => {});
} finally {
  await browser?.close();
  try {
    const stop = await fetch(origin + '/__fixture/stop', { method: 'POST', headers: { 'X-Fixture-Token': 'synthetic-session' }, redirect: 'error', signal: AbortSignal.timeout(3000) });
    report.fixtureStopped = stop.status === 204;
    if (!report.fixtureStopped) process.exitCode = 1;
  } catch (error) { report.cleanupError = String(error); process.exitCode = 1; }
  if (ownsOutput) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({ observationComplete: report.observationComplete, fixtureStopped: report.fixtureStopped, attempts: report.attempts.map(a => ({ activation: a.activation, download: a.download, error: a.error })), requests: report.requests, events: report.events, failures: report.failures, error: report.error, cleanupError: report.cleanupError }, null, 2));
}
