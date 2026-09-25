// Narrow diagnostic after the retained native ZIP timeout; not a passing task gate.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import { pathToFileURL } from 'node:url';
const [pw, chrome, origin, output] = process.argv.slice(2);
assert.equal(new URL(origin).hostname, '127.0.0.1');
assert.equal(new URL(origin).protocol, 'http:');
await fs.mkdir(output);
const ready = await fetch(origin + '/dev/theme', { signal: AbortSignal.timeout(10000) });
assert.equal(ready.status, 200);
await ready.arrayBuffer();
const { chromium } = await import(pathToFileURL(pw).href);
const browser = await chromium.launch({ executablePath: chrome, headless: true });
const result = { posts: [], console: [], downloads: [], pageErrors: [] };
try {
  const context = await browser.newContext({ acceptDownloads: true });
  await context.route('**/*', route => new URL(route.request().url()).origin === origin ? route.continue() : route.abort());
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  page.on('console', m => result.console.push({ type: m.type(), text: m.text() }));
  page.on('pageerror', e => result.pageErrors.push(String(e)));
  page.on('response', r => {
    if (r.request().method() === 'POST') result.posts.push({ status: r.status(), headers: r.headers(), action: new URLSearchParams(r.request().postData()).get('action') });
  });
  page.on('download', d => result.downloads.push(d.suggestedFilename()));
  await page.goto(origin + '/dev/theme', { waitUntil: 'load' });
  await page.getByText('Corner radii', { exact: true }).click();
  await page.locator('input[name="radius.md"]').fill('0.75rem');
  await Promise.all([page.waitForNavigation({ waitUntil: 'load' }), page.getByRole('button', { name: 'Preview candidate', exact: true }).click()]);
  result.form = await page.locator('form').evaluate(f => ({
    invalid: Array.from(f.elements).filter(e => e.willValidate && !e.validity.valid).map(e => ({ name: e.name, message: e.validationMessage })),
    reviewed: f.elements.namedItem('review_digest')?.value,
  }));
  const actions = await Promise.allSettled([
    page.waitForEvent('download', { timeout: 8000 }),
    page.getByRole('button', { name: 'Export acknowledged candidate ZIP', exact: true }).click(),
  ]);
  result.exportActions = actions.map(a => ({ status: a.status, error: a.status === 'rejected' ? String(a.reason) : undefined }));
  if (actions[0].status === 'fulfilled') await actions[0].value.saveAs(path.join(output, 'candidate.zip'));
  result.alerts = await page.getByRole('alert').allTextContents();
  result.finalURL = page.url();
  await page.screenshot({ path: path.join(output, 'after-export.png'), fullPage: true });
} catch (e) {
  result.error = String(e.stack || e);
} finally {
  await browser.close();
  await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify(result, null, 2));
}
