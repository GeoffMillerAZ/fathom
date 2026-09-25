// Local Gallery stat specimen checks. Uses the consumer typography oracle.
// node stat_reflow.mjs <loopback-gallery-origin> <empty-report-dir> <playwright-module>
import { inspectMetricTypography, inspectBandDividers } from '../../../docs/assessments/fixtures/fathom-observations/metric-geometry.mjs';
import { mkdtemp, readFile, readdir, realpath, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { createHash } from 'node:crypto';

const [address, output, modulePath] = process.argv.slice(2);
const base = new URL(address);
if (base.protocol !== 'http:' || base.hostname !== '127.0.0.1' || base.username || base.password || base.pathname !== '/' || base.search || base.hash) throw Error('explicit HTTP loopback origin required');
const directory = await realpath(output);
if ((await readdir(directory)).length) throw Error('report directory must be empty');
const { chromium } = await import(pathToFileURL(await realpath(modulePath)).href);
const profile = await mkdtemp(join(tmpdir(), 'appkit-stat-gallery-'));
const sha = body => createHash('sha256').update(body).digest('hex');
const report = { scope: 'Gallery stat specimen and 3/4/5-item band dividers, 100% zoom, DPR1, reduced motion; not whole-page accessibility', checks: [], cases: [], inputs: {}, errors: [] };
const check = (id, name, pass, detail) => report.checks.push({ id, name, pass, detail });
const luma = color => color.slice(0, 3).map(x => x / 255).map(x => x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4).reduce((sum, x, i) => sum + x * [0.2126, 0.7152, 0.0722][i], 0);
let context;
try {
  context = await chromium.launchPersistentContext(profile, {
    executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    headless: true, deviceScaleFactor: 1, serviceWorkers: 'block', acceptDownloads: false,
    reducedMotion: 'reduce', args: ['--no-proxy-server', '--disable-background-networking', '--disable-component-update'],
  });
  await context.route('**/*', async route => {
    const request = route.request(), url = new URL(request.url());
    const allowed = url.origin === base.origin && request.method() === 'GET' && (
      (url.pathname === '/layout' && ['?theme=light', '?theme=dark'].includes(url.search)) ||
      (url.pathname === '/static/app.css' && !url.search));
    if (allowed) return route.continue();
    report.errors.push(`blocked ${request.method()} ${url}`);
    await route.abort();
  });
  const page = await context.newPage();
  page.on('pageerror', error => report.errors.push(error.message));
  report.browser = await page.evaluate(() => navigator.userAgent);
  for (const theme of ['light', 'dark']) for (const width of [320, 768, 1024, 1440]) {
    const id = `${theme}-${width}`;
    await page.setViewportSize({ width, height: 1000 });
    const response = await page.goto(`${base.origin}/layout?theme=${theme}`, { waitUntil: 'networkidle' });
    check(id, 'HTTP 200', response.status() === 200);
    report.inputs[`layout-${theme}`] = sha(await response.body());
    await page.evaluate(() => document.fonts.ready);
    check(id, 'selected theme', await page.locator('html').getAttribute('data-theme') === theme);
    const sample = await page.locator('#stat-reflow').evaluate(inspectMetricTypography);
    check(id, 'seven band/tile/spark metrics', sample.metrics.length === 7);
    const expected = ['0 B', '≥ 7', '—', '0 B', '≥ 7', '—', '123 ms'];
    const labels = ['Schema size', 'Reported observations', 'Unreported source', 'Schema size', 'Reported observations', 'Unreported source', 'Synthetic response time'];
    const captions = [
      '0 tools carried by every connected session',
      'Lower bound: two of three synthetic sources reported',
      'Source unavailable: synthetic/observation/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    ];
    for (const [i, metric] of sample.metrics.entries()) {
      check(id, `metric ${i} exact value`, metric.value === expected[i], metric.value);
      check(id, `metric ${i} exact label`, metric.label === labels[i], metric.label);
      const caption = i < 6 ? captions[i % 3] : 'p99 across 1,000 synthetic requests | -12%';
      check(id, `metric ${i} exact caption`, metric.caption === caption, metric.caption);
      check(id, `metric ${i} visible single line`, metric.visible && metric.singleLine, metric.valueRects);
      check(id, `metric ${i} stays inside cell`, metric.bounded, metric);
      check(id, `metric ${i} does not overlap caption`, metric.separated, metric);
    }
    for (const [i, color] of sample.colors.entries()) {
      const supported = color.unsupported.length === 0;
      check(id, `color ${i} supported`, supported, color.unsupported);
      const foreground = supported ? luma(color.foreground) : 0, background = supported ? luma(color.background) : 0;
      const ratio = (Math.max(foreground, background) + 0.05) / (Math.min(foreground, background) + 0.05);
      const threshold = color.fontSize >= 24 || (color.fontSize >= 18.667 && color.fontWeight >= 700) ? 3 : 4.5;
      check(id, `color ${i} contrast`, supported && ratio >= threshold, { text: color.text, ratio, threshold });
    }
    const spark = await page.locator('#stat-reflow svg').evaluate(svg => {
      const a = svg.getBoundingClientRect(), b = svg.parentElement.parentElement.parentElement.getBoundingClientRect();
      return { width: a.width, height: a.height, inside: a.left >= b.left && a.right <= b.right && a.top >= b.top && a.bottom <= b.bottom };
    });
    check(id, 'sparkline retains size and containment', spark.inside && spark.width === 88 && spark.height === 28, spark);
    const bands = await page.locator('main').evaluate(inspectBandDividers);
    check(id, 'three independent stat bands', bands.length === 3, bands.map(band => band.cells.length));
    const counts = [4, 3, 5];
    for (const [i, band] of bands.entries()) {
      const count = counts[i];
      const columns = width < 768 ? 1 : count === 4 ? (width < 1024 ? 2 : 4) : count === 5 ? (width < 1024 ? 2 : width < 1280 ? 3 : 5) : 3;
      check(id, `band ${i} exact cell count`, band.cells.length === count, band.cells.length);
      check(id, `band ${i} responsive rows`, band.rows === Math.ceil(count / columns), { rows: band.rows, columns });
      check(id, `band ${i} clipped outer and visible internal dividers`, band.valid, band);
      check(id, `band ${i} horizontal divider segments`, band.horizontal.length === Math.floor((count - 1) / columns) * columns, band.horizontal.length);
      check(id, `band ${i} vertical divider segments`, band.vertical.length === count - Math.floor(count / columns), band.vertical.length);
    }
    await page.locator('#stat-reflow').screenshot({ path: join(directory, `${id}.png`) });
    await page.locator('#stat-dense-reflow').screenshot({ path: join(directory, `${id}-dense.png`) });
    report.cases.push({ id, sample, bands });
  }
  report.inputs.css = sha(await (await context.request.get(`${base.origin}/static/app.css`)).body());
  report.inputs.oracle = sha(await readFile(new URL('../../../docs/assessments/fixtures/fathom-observations/metric-geometry.mjs', import.meta.url)));
} catch (error) {
  report.errors.push(error.stack || String(error));
} finally {
  await context?.close();
  await rm(profile, { recursive: true, force: true });
  report.total = report.checks.length;
  report.failed = report.checks.filter(row => !row.pass).length;
  await writeFile(join(directory, 'report.json'), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({ total: report.total, failed: report.failed, errors: report.errors, report: join(directory, 'report.json') }));
}
process.exitCode = report.failed || report.errors.length ? 1 : 0;
