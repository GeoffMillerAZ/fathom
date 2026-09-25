// Actual Chrome 200% zoom and native keyboard recovery on the synthetic RAC guide.
// No application JS, injected focus, DOM edits, saved profile or live app routes.
import { chromium } from '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
import { readFileSync, writeFileSync, mkdtempSync, mkdirSync, rmSync, chmodSync, statSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { performance } from 'node:perf_hooks';
import { createRequire } from 'node:module';
import { prepareContext, zoomCaptureClip, zoomElementCaptureClip } from '../gallery-analytics/zoom-contract.mjs';
import { inspectSynthwaveCapture } from './capture-contract.mjs';

const { PNG } = createRequire(import.meta.url)('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright-core/lib/utilsBundle.js');

const [url, directory] = process.argv.slice(2);
if (!url || !directory || process.argv.length !== 4) throw Error('usage: node zoom.mjs EXACT_GUIDE_URL NEW_OUTPUT');
const target = new URL(url);
if (target.protocol !== 'http:' || target.hostname !== '127.0.0.1' || !target.port ||
    target.pathname !== '/theme-guide' || target.search || target.hash || target.username || target.password || target.href !== url)
  throw Error('exact loopback theme-guide URL required');
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const harness = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
if (hash(readFileSync(harness)) !== 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f') throw Error('harness identity changed');
const output = resolve(directory);
mkdirSync(output, { mode: 0o700 }); // no overwrite
const profile = mkdtempSync(join(tmpdir(), 'appkit-rac-zoom-'));
chmodSync(profile, 0o700);
const stageNames = ['source', 'invalid', 'radius-repaired', 'badge-refused', 'candidate', 'stale', 'reacknowledged', 'source-comparison', 'reset'];
const report = {
  schema: 1, url, probeSHA256: hash(readFileSync(new URL(import.meta.url))),
  zoomHelperSHA256: hash(readFileSync(new URL('../gallery-analytics/zoom-contract.mjs', import.meta.url))),
  captureHelperSHA256: hash(readFileSync(new URL('./capture-contract.mjs', import.meta.url))),
  checks: [], stages: [], responses: [], blocked: [], errors: [], requestFailures: [], downloads: [], timings: [], screenshots: [],
  limits: ['Actual browser zoom in a fresh synthetic profile; no existing Chrome settings changed.',
    'Native automated keys, not physical keyboard or screen-reader certification.',
    'Local submission-to-load timings include navigation, not human task time or production latency budgets.',
    'Selected editor/preview geometry and token use, not full visual or whole-app acceptance.'],
};
const must = (name, pass, detail) => {
  report.checks.push({ name, pass: !!pass, detail });
  if (!pass) throw Error(name);
};
const pending = new Set();
let context, page, scriptSession;
async function drain() { while (pending.size) await Promise.allSettled([...pending]); }
async function tabTo(locator, limit = 120, reverse = false) {
  if (await locator.and(page.locator(':focus')).count() === 1) return;
  for (let n = 0; n < limit; n++) {
    await page.keyboard.press(reverse ? 'Shift+Tab' : 'Tab');
    if (await locator.and(page.locator(':focus')).count() === 1) return;
  }
  throw Error('Native Tab did not reach ' + await locator.first().getAttribute('name'));
}
async function typeAt(locator, value) {
  await tabTo(locator);
  await page.keyboard.press('Meta+A');
  await page.keyboard.type(value);
}
async function submit(label, status) {
  await tabTo(page.getByRole('button', { name: label, exact: true }));
  const begin = performance.now();
  const [response] = await Promise.all([
    page.waitForResponse(r => r.request().method() === 'POST' && new URL(r.url()).pathname === '/theme-guide'),
    page.waitForNavigation({ waitUntil: 'load' }), page.keyboard.press('Enter'),
  ]);
  report.timings.push({ label, status: response.status(), submissionToLoadMS: performance.now() - begin });
  must(label + ' HTTP ' + status + ' #' + report.timings.length, response.status() === status);
}
async function screenshot(name, frameOnly = false) {
  const path = join(output, name + '.png');
  let capture;
  if (frameOnly) {
    // Screenshot preparation only: offscreen opaque frames may not be composited.
    // Does not change focus, form data, document styling or the keyboard journey.
    await page.locator('iframe').scrollIntoViewIfNeeded();
    const cssBox = await page.locator('iframe').evaluate(el => {
      const r = el.getBoundingClientRect();
      return { x: r.x + scrollX, y: r.y + scrollY, width: r.width, height: r.height };
    });
    const cdp = await context.newCDPSession(page);
    try {
      const metrics = await cdp.send('Page.getLayoutMetrics');
      const clip = zoomElementCaptureClip(metrics, cssBox);
      const { data } = await cdp.send('Page.captureScreenshot', { format: 'png', clip, captureBeyondViewport: true, fromSurface: true });
      const bytes = Buffer.from(data, 'base64');
      writeFileSync(path, bytes, { flag: 'wx' }); // retain failed images for diagnosis
      capture = { preparation: 'scroll iframe into view without changing focus', cssBox, clip,
        paint: inspectSynthwaveCapture(PNG.sync.read(bytes), clip.width, clip.height) };
      must(name + ' captures the painted Synthwave specimen', true, capture);
    } finally { await cdp.detach(); }
  } else await page.screenshot({ path, fullPage: false });
  report.screenshots.push({ name, sha256: hash(readFileSync(path)), capture });
}
async function inspectStage(name) {
  const facts = await page.evaluate(() => {
    const all = [...document.querySelectorAll('.sg a,.sg button,.sg input:not([type="hidden"]),.sg select,.sg-editor summary')].filter(el => el.checkVisibility());
    const primary = new Set(document.querySelectorAll('.sg button,.sg input:not([type="hidden"]),.sg select,.sg-editor summary'));
    return { width: innerWidth, scroll: document.documentElement.scrollWidth, count: all.length,
      controls: all.slice(0, 160).map(el => { const r = el.getBoundingClientRect(); return {
        name: (el.getAttribute('name') || el.textContent.trim()).slice(0, 80), x: r.x, right: r.right, w: r.width, h: r.height, primary: primary.has(el),
      }; }) };
  });
  must(name + ' editor width and complete inventory', facts.width === 720 && facts.scroll <= 721 && facts.count > 0 && facts.count <= 160 && facts.controls.length === facts.count, facts);
  must(name + ' controls contained with 44px primary targets', facts.controls.every(c => c.x >= -1 && c.right <= 721 && (!c.primary || (c.w >= 44 && c.h >= 44))), facts.controls.filter(c => c.x < -1 || c.right > 721 || (c.primary && (c.w < 44 || c.h < 44))));
  const frame = page.frames().find(f => f !== page.mainFrame() && new URL(f.url()).pathname === '/theme-guide/preview');
  must(name + ' opaque specimen exists', !!frame && await page.locator('iframe').getAttribute('sandbox') === '');
  const preview = await frame.evaluate(() => ({ width: innerWidth, scroll: document.documentElement.scrollWidth,
    synthetic: document.body.textContent.includes('Synthetic theme specimen'), scripts: document.scripts.length,
    font: getComputedStyle(document.querySelector('.font-sans')).fontFamily }));
  must(name + ' actual styled specimen remains contained', preview.synthetic && preview.scripts === 0 && preview.width > 0 && preview.scroll <= preview.width + 1 && preview.font.includes('ui-sans-serif'), preview);
  report.stages.push({ name, facts, preview });
}
async function radiusValue() {
  const frame = page.frames().find(f => f !== page.mainFrame() && new URL(f.url()).pathname === '/theme-guide/preview');
  return frame.locator('.rounded-lg:visible').first().evaluate(el => getComputedStyle(el).borderRadius);
}
async function badgeColors() {
  const frame = page.frames().find(f => f !== page.mainFrame() && new URL(f.url()).pathname === '/theme-guide/preview');
  return frame.getByText('Email proposal', { exact: true }).evaluate(el => {
    const style = getComputedStyle(el);
    return { foreground: style.color, background: style.backgroundColor, fontSize: style.fontSize };
  });
}
async function run() {
  context = await chromium.launchPersistentContext(profile, { executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    headless: true, viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce', acceptDownloads: true,
    args: ['--disable-background-networking', '--no-proxy-server', '--no-first-run', '--no-default-browser-check'] });
  await prepareContext(context, async () => {
    page = context.pages()[0];
    page.setDefaultTimeout(6000); page.setDefaultNavigationTimeout(15000);
    await page.goto('chrome://settings/appearance');
    await page.locator('#zoomLevel').selectOption('2');
    must('Chrome Appearance is 200%', await page.locator('#zoomLevel').inputValue() === '2');
    await page.goto('about:blank');
    scriptSession = await context.newCDPSession(page);
    await scriptSession.send('Emulation.setScriptExecutionDisabled', { value: true });
    // Retain the emulation session until browser teardown; CSP also forbids scripts.
  });
  report.browser = context.browser()?.version();
  context.on('serviceworker', w => report.errors.push('Service worker: ' + w.url()));
  await context.routeWebSocket('**/*', socket => { report.blocked.push(socket.url()); socket.close(); });
  await context.route('**/*', route => {
    const r = route.request(), u = new URL(r.url());
    if (u.origin === target.origin && ((r.method() === 'GET' && ['/theme-guide', '/theme-guide/preview', '/theme-guide/style.css', '/theme-guide/app.css', '/theme-guide/brand.css'].includes(u.pathname)) || (r.method() === 'POST' && u.pathname === '/theme-guide'))) return route.continue();
    report.blocked.push({ method: r.method(), url: r.url() }); return route.abort();
  });
  page.on('pageerror', e => report.errors.push(String(e)));
  page.on('requestfailed', r => {
    const download = r.method() === 'POST' && new URL(r.url()).pathname === '/theme-guide' && r.postData()?.includes('action=export') && r.failure()?.errorText === 'net::ERR_ABORTED';
    if (!download) report.requestFailures.push({ url: r.url(), error: r.failure() });
  });
  page.on('response', r => {
    const task = (async () => {
      const headers = r.headers();
      if (headers['content-disposition']?.includes('attachment')) return;
      let timer;
      try {
        const body = await Promise.race([r.body(), new Promise((_, reject) => { timer = setTimeout(() => reject(Error('body deadline')), 10000); })]);
        report.responses.push({ url: r.url(), status: r.status(), headers, bytes: body.length, sha256: hash(body) });
      } finally { clearTimeout(timer); }
    })();
    pending.add(task); task.catch(e => report.errors.push(String(e))).finally(() => pending.delete(task));
  });
  await page.goto(url, { waitUntil: 'load' });
  const cdp = await context.newCDPSession(page);
  const metrics = await cdp.send('Page.getLayoutMetrics');
  report.zoom = { clip: zoomCaptureClip(metrics), exposed: await page.evaluate(() => ({ dpr: devicePixelRatio, inner: innerWidth, outer: outerWidth, scale: visualViewport.scale })) };
  await cdp.detach();
  must('Browser scale is actual 200%', report.zoom.exposed.dpr === 2 && report.zoom.exposed.inner === 720 && report.zoom.exposed.outer === 1440 && report.zoom.exposed.scale === 1, report.zoom);
  await inspectStage('source');
  const sourceFailures = await page.locator('h2#sg-checks').locator('xpath=following-sibling::div//tbody/tr').evaluateAll(rows => rows.filter(r => r.textContent.includes('Needs review')).map(r => r.querySelector('th').textContent).sort());
  must('Source names all six selected contrast failures', JSON.stringify(sourceFailures) === JSON.stringify(['action-dark', 'action-light', 'badge-accent-dark', 'badge-accent-light', 'table-secondary-dark', 'table-secondary-light']), sourceFailures);
  const radius = page.getByLabel('radius.lg', { exact: true });
  await tabTo(page.locator('.sg-editor summary').filter({ hasText: 'Corner radii' })); await page.keyboard.press('Enter');
  await typeAt(radius, '-1px'); await submit('Preview candidate', 422);
  must('Error summary has native focus', await page.locator(':focus').getAttribute('role') === 'alert');
  await inspectStage('invalid'); await screenshot('zoom-invalid');
  await tabTo(page.getByRole('link', { name: 'Fix radius.lg', exact: true }), 1); await page.keyboard.press('Enter');
  must('Repair link focuses retained invalid field', await page.locator(':focus').getAttribute('name') === 'radius.lg' && await radius.inputValue() === '-1px' && await radius.getAttribute('aria-invalid') === 'true');
  await typeAt(radius, '1rem'); await submit('Preview candidate', 200);
  const exportButton = page.getByRole('button', { name: 'Export acknowledged candidate ZIP', exact: true });
  must('Radius repair does not waive source contrast failures', await exportButton.isDisabled());
  await inspectStage('radius-repaired');
  const colors = page.locator('.sg-editor summary').filter({ hasText: 'Color roles' });
  await tabTo(colors); await page.keyboard.press('Enter');
  for (const scheme of ['dark', 'light']) await typeAt(page.getByLabel('colors.accent.' + scheme, { exact: true }), '#492c99');
  for (const scheme of ['dark', 'light']) await typeAt(page.getByLabel('colors.ink-faint.' + scheme, { exact: true }), '#c9bfe8');
  await submit('Preview candidate', 200);
  const badgeFailures = await page.locator('h2#sg-checks').locator('xpath=following-sibling::div//tbody/tr').evaluateAll(rows => rows.filter(r => r.textContent.includes('Needs review')).map(r => r.querySelector('th').textContent).sort());
  must('Button-only repair cannot export unreadable badges', await exportButton.isDisabled() && JSON.stringify(badgeFailures) === JSON.stringify(['badge-accent-dark', 'badge-accent-light']), badgeFailures);
  const failedBadge = await badgeColors();
  must('Refused candidate reaches the actual badge', failedBadge.foreground === 'rgb(73, 44, 153)' && failedBadge.background === 'rgb(42, 33, 80)', failedBadge);
  await inspectStage('badge-refused');
  for (const scheme of ['dark', 'light']) await typeAt(page.getByLabel('colors.accent.' + scheme, { exact: true }), '#b69aff');
  for (const scheme of ['dark', 'light']) await typeAt(page.getByLabel('colors.accent-ink.' + scheme, { exact: true }), '#180f2e');
  await submit('Preview candidate', 200);
  const rows = await page.locator('h2#sg-checks').locator('xpath=following-sibling::div//tbody/tr').allTextContents();
  must('Candidate passes all twelve app-selected checks', rows.length === 12 && rows.every(t => t.includes('Pass') && !t.includes('Needs review')), rows);
  const repairedBadge = await badgeColors();
  must('Joint repair reaches the actual badge', repairedBadge.foreground === 'rgb(182, 154, 255)' && repairedBadge.background === 'rgb(42, 33, 80)', repairedBadge);
  must('Actual visible radius becomes 16px', await radiusValue() === '16px');
  await inspectStage('candidate'); await screenshot('zoom-candidate-preview', true);
  await tabTo(exportButton);
  const downloading = page.waitForEvent('download'); await page.keyboard.press('Enter');
  const download = await downloading, zip = join(output, 'candidate.zip'); await download.saveAs(zip);
  const zipBytes = statSync(zip).size;
  must('Keyboard export downloads a bounded review bundle', await download.failure() === null && zipBytes > 0 && zipBytes <= 1024 * 1024, zipBytes);
  const unzipLimits = { timeout: 5000, maxBuffer: 64 * 1024 };
  execFileSync('/usr/bin/unzip', ['-tq', zip], unzipLimits);
  const zipEntries = execFileSync('/usr/bin/unzip', ['-Z1', zip], { ...unzipLimits, encoding: 'utf8' }).trim().split('\n').sort();
  must('Keyboard export contains exactly the five review files', JSON.stringify(zipEntries) === JSON.stringify(['candidate.css', 'candidate.cue', 'diff.json', 'original.cue', 'receipt.json']), zipEntries);
  report.downloads.push({ bytes: readFileSync(zip).length, sha256: hash(readFileSync(zip)) });
  await typeAt(radius, '1.25rem'); await submit('Export acknowledged candidate ZIP', 409);
  must('Stale export clears acknowledgement without losing edit', await exportButton.isDisabled() && await page.locator('[name="review_digest"]').inputValue() === '' && await radius.inputValue() === '1.25rem');
  await inspectStage('stale'); await screenshot('zoom-stale');
  await submit('Preview candidate', 200);
  must('Fresh preview acknowledges 20px candidate', /^[0-9a-f]{64}$/.test(await page.locator('[name="review_digest"]').inputValue()) && await radiusValue() === '20px');
  await inspectStage('reacknowledged');
  await submit('Show source', 200);
  must('Source comparison preserves edit and restores 8px', await radius.inputValue() === '1.25rem' && await radiusValue() === '8px');
  await inspectStage('source-comparison');
  await tabTo(page.getByRole('link', { name: 'Reset to source', exact: true }));
  await Promise.all([page.waitForNavigation({ waitUntil: 'load' }), page.keyboard.press('Enter')]);
  must('Reset clears candidate and preserves source', await radius.inputValue() === '' && await page.locator('[name="review_digest"]').inputValue() === '' && await radiusValue() === '8px');
  await inspectStage('reset'); await screenshot('zoom-reset-preview', true);
}
try { await run(); }
catch (error) { report.failure = String(error.stack || error); if (page) await screenshot('failure').catch(() => {}); }
finally {
  await drain();
  if (context) await context.close().catch(e => report.errors.push('cleanup: ' + String(e)));
  await drain();
  try { rmSync(profile, { recursive: true, force: false }); }
  catch (error) { report.errors.push('profile cleanup: ' + String(error)); }
  report.completedAllStages = JSON.stringify(report.stages.map(s => s.name)) === JSON.stringify(stageNames);
  const html = report.responses.filter(r => r.headers['content-type']?.startsWith('text/html'));
  const css = report.responses.filter(r => ['/theme-guide/app.css', '/theme-guide/brand.css'].includes(new URL(r.url).pathname));
  const cssPaths = [...new Set(css.map(r => new URL(r.url).pathname))].sort();
  report.resourceContract = html.length > 0 && html.every(r => r.headers['content-security-policy']?.includes("script-src 'none'")) &&
    JSON.stringify(cssPaths) === JSON.stringify(['/theme-guide/app.css', '/theme-guide/brand.css']) &&
    css.every(r => r.status === 200 && r.headers['content-type']?.startsWith('text/css'));
  report.ok = !report.failure && report.completedAllStages && report.resourceContract && report.checks.every(c => c.pass) && !report.errors.length && !report.requestFailures.length && !report.blocked.length && report.downloads.length === 1;
  writeFileSync(join(output, 'receipt.json'), JSON.stringify(report, null, 2) + '\n', { flag: 'wx' });
  console.log(JSON.stringify({ output, ok: report.ok, checks: report.checks.length, stages: report.stages.map(s => s.name), failure: report.failure, failed: report.checks.filter(c => !c.pass), errors: report.errors, blocked: report.blocked, requestFailures: report.requestFailures }));
  if (!report.ok) process.exitCode = 1;
}
