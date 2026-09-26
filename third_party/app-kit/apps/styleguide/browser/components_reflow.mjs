// Opt-in synthetic component-preview geometry, not whole-gallery accessibility.
import { readFileSync, writeFileSync, mkdtempSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { configuration } from '../../gallery/browser/config.mjs';

const config = configuration({...process.env, APPKIT_SHELL_BROWSER_ORIGIN: process.env.APPKIT_THEME_BROWSER_ORIGIN});
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
if (hash(readFileSync(new URL(config.moduleURL))) !== 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f' || hash(readFileSync(new URL('./package.json', config.moduleURL))) !== '8bc2ddebe8d9b051b9ecb87d62c6c63877e59d08617f32c3215681ca8bb70998') throw Error('Approved harness identity differs');
const { chromium } = await import(config.moduleURL);
const root = mkdtempSync(join(tmpdir(), 'appkit-theme-components-'));
const report = {origin: config.base, variant: config.variant, probe_sha256: hash(readFileSync(new URL(import.meta.url))), checks: [], cases: [], blocked: [], errors: [], screenshots: [], limits: ['Static Gallery component specimens only; not real consumer, actual zoom, keyboard scrolling, full accessibility or visual-fidelity certification.', 'Geometry and computed paint are read-only diagnostics, not pixel visibility proof. Screenshots require separate review.']};
console.log('Component preview evidence: ' + root);
const check = (name, pass, detail) => report.checks.push({name, pass: !!pass, detail});
const captions = ['Sparkline', 'StackBar', 'Donut', 'Funnel', 'MeterRow', 'Gauge — graded (case strength)', 'Gauge — graded (likelihood)', 'Sparkline — fluid (fills its container)', 'Heatmap — truth states (value, zero, missing, unknown, error)', 'RangeBars — recovery low/high + most-likely', 'Waterfall — combined recovery → net', 'CostReward — reward vs cost', 'Tornado — what moves the outcome most (sensitivity)', 'Timeline — proportional', 'Timeline — condensed (gaps elided + labeled)', 'Timeline — sequence (even spacing)', 'Timeline — swimlanes (grouped by source)', 'Timeline — density (activity heatmap)', 'Timeline — filtered (EventFilter: evidence lane, H1)'];
let browser;
try {
  browser = await chromium.launch({executablePath: config.executablePath, headless: true, args: ['--disable-background-networking', '--no-proxy-server']});
  report.browser = browser.version();
  for (const width of [320, 390, 1440]) for (const scheme of ['light', 'dark']) {
    const context = await browser.newContext({viewport: {width, height: 1000}, reducedMotion: 'reduce', javaScriptEnabled: false, acceptDownloads: false});
    try {
      context.setDefaultTimeout(5000);
      context.setDefaultNavigationTimeout(15000);
      context.on('serviceworker', () => report.errors.push('Unexpected service worker'));
      await context.route('**/*', route => {
        const request = route.request(), url = new URL(request.url());
        const allowed = url.origin === config.base && ((request.method() === 'GET' && ['/theme-lab', '/theme-lab/style.css', '/theme-lab/preview', '/static/app.css'].includes(url.pathname)) || (request.method() === 'POST' && url.pathname === '/theme-lab'));
        if (!allowed) { report.blocked.push({method: request.method(), url: request.url()}); return route.abort(); }
        return route.continue();
      });
      await context.routeWebSocket('**/*', socket => { report.blocked.push({method: 'WS', url: socket.url()}); socket.close(); });
      const page = await context.newPage();
      page.on('pageerror', error => report.errors.push(String(error)));
      page.on('console', message => { if (['warning', 'error'].includes(message.type())) report.errors.push(message.text()); });
      await page.goto(config.base + '/theme-lab', {waitUntil: 'networkidle'});
      await page.locator('select[name="specimen"]').selectOption('components');
      await page.locator('select[name="scheme"]').selectOption(scheme);
      await page.locator('select[name="viewport"]').selectOption('narrow');
      const [response] = await Promise.all([page.waitForNavigation({waitUntil: 'networkidle'}), page.getByRole('button', {name: 'Preview candidate', exact: true}).click()]);
      const name = width + '-' + scheme;
      check(name + ' preview accepted', response.status() === 200, response.status());
      const frame = await (await page.locator('iframe').elementHandle()).contentFrame();
      const facts = await frame.evaluate(() => {
        const rect = el => { const r = el.getBoundingClientRect(); return {left: r.left, right: r.right, width: r.width, height: r.height}; };
        return {width: innerWidth, scroll: document.documentElement.scrollWidth, figures: [...document.querySelectorAll('figure')].map(el => ({label: el.querySelector('figcaption')?.textContent, rect: rect(el), svg: el.querySelector('svg') ? rect(el.querySelector('svg')) : null})), spark: [...document.querySelectorAll('figure')].find(el => el.querySelector('figcaption')?.textContent === 'Sparkline')?.querySelector('polyline') ? (() => { const el = [...document.querySelectorAll('figure')].find(el => el.querySelector('figcaption')?.textContent === 'Sparkline').querySelector('polyline'); const style = getComputedStyle(el); return {rect: rect(el), stroke: style.stroke, opacity: style.opacity, display: style.display, visibility: style.visibility}; })() : null};
      });
      report.cases.push({name, facts});
      check(name + ' exact component specimen', new URL(frame.url()).searchParams.get('specimen') === 'components' && JSON.stringify(facts.figures.map(f => f.label)) === JSON.stringify(captions));
      check(name + ' every figure fits frame', facts.figures.length === captions.length && facts.scroll <= facts.width + 1 && facts.figures.every(f => f.rect.width > 0 && f.rect.left >= 0 && f.rect.right <= facts.width + 1), facts);
      check(name + ' every SVG has contained geometry', facts.figures.length === captions.length && facts.figures.every(f => f.svg?.width > 0 && f.svg.height > 0 && f.svg.left >= f.rect.left && f.svg.right <= f.rect.right + 1 && f.svg.left >= 0 && f.svg.right <= facts.width + 1), facts.figures);
      check(name + ' sparkline geometry and paint retained', facts.spark?.rect.width > 0 && facts.spark.rect.height > 0 && facts.spark.rect.left >= 0 && facts.spark.rect.right <= facts.width + 1 && facts.spark.stroke !== 'none' && facts.spark.opacity !== '0' && facts.spark.display !== 'none' && facts.spark.visibility === 'visible', facts.spark);
      check(name + ' opaque static preview retained', await page.locator('iframe').getAttribute('sandbox') === '' && (await frame.locator('html').getAttribute('data-theme')) === scheme);
      await page.locator('iframe').scrollIntoViewIfNeeded();
      const path = join(root, name + '.png');
      await page.locator('iframe').screenshot({path});
      report.screenshots.push({name, path, sha256: hash(readFileSync(path))});
      check(name + ' no service worker', context.serviceWorkers().length === 0);
    } finally { await context.close(); }
  }
} catch (error) { report.failure = String(error); }
finally {
  if (browser) { try { await browser.close(); } catch (error) { report.errors.push('Browser close: ' + String(error)); } }
  check('no outside requests', report.blocked.length === 0, report.blocked);
  check('no browser errors', report.errors.length === 0, report.errors);
  const expected = [320, 390, 1440].flatMap(w => ['light', 'dark'].flatMap(s => ['preview accepted', 'exact component specimen', 'every figure fits frame', 'every SVG has contained geometry', 'sparkline geometry and paint retained', 'opaque static preview retained', 'no service worker'].map(n => w + '-' + s + ' ' + n))).concat(['no outside requests', 'no browser errors']);
  check('exact assertion set completed', JSON.stringify(report.checks.map(c => c.name).sort()) === JSON.stringify(expected.sort()));
  report.ok = !report.failure && report.checks.every(c => c.pass);
  writeFileSync(join(root, 'receipt.json'), JSON.stringify(report, null, 2) + '\n', {flag: 'wx'});
  console.log(JSON.stringify({root, ok: report.ok, checks: report.checks.length, failed: report.checks.filter(c => !c.pass).map(c => c.name), failure: report.failure}));
  if (!report.ok) process.exitCode = 1;
}
