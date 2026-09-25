// Typography evidence for inert synthetic Fathom observation exports.
// node metrics.mjs <export-dir> <empty-report-dir> <playwright/index.mjs>
import { createServer } from 'node:http';
import { chmod, mkdir, mkdtemp, readFile, readdir, realpath, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, relative, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { createHash } from 'node:crypto';
import { once } from 'node:events';
import { inspectBandDividers, inspectMetricTypography } from './metric-geometry.mjs';

const [input, output, playwrightModule] = process.argv.slice(2);
if (!input || !output || !playwrightModule) throw Error('export, empty report, and isolated Playwright module paths required');
const source = await realpath(input);
const destination = await realpath(output);
for (const dir of [source, destination]) {
  if (!dir.includes('/workspace/tmp/')) throw Error('artifacts must stay in a workspace/tmp child');
}
if (!relative(source, destination).startsWith('..')) throw Error('report must not mutate the export');
if ((await readdir(destination)).length !== 0) throw Error('report directory must be empty');
const modulePath = await realpath(resolve(playwrightModule));
const { chromium } = await import(pathToFileURL(modulePath).href);

const scenarios = [
  'all-unknown', 'partial', 'complete-zero', 'complete-positive', 'registry-unavailable',
  'registry-partial', 'call-log-unavailable', 'registry-call-log-unavailable',
  'measured-zero-catches', 'measured-positive-catches',
];
const themes = ['light', 'dark'];
const widths = [320, 768, 1440];
const screenshotScenarios = new Set(['complete-zero', 'complete-positive', 'registry-call-log-unavailable']);
const limits = { htmlBytes: 163840, cssBytes: 102400, domElements: 2500, dclMS: 1500 };
const failures = [];
const counts = { cases: 0, metrics: 0, labels: 0, bands: 0, separators: 0, checks: 0, passed: 0, failed: 0, requests: 0, blocked: 0, screenshots: 0 };
const evidence = [];
const canaryResults = { unitWrap: false, captionOverlap: false, lowContrast: false, separatorRemoval: false, mutationsRestored: false };
const compact = detail => {
  if (!detail || typeof detail !== 'object' || Array.isArray(detail)) return detail;
  if (detail.cell && detail.valueRects && detail.captionRects) {
    const all = [...detail.valueRects, ...detail.captionRects];
    return {
      value: detail.value, caption: detail.caption,
      valueLines: detail.valueRects.length, captionLines: detail.captionRects.length,
      cell: [detail.cell.left, detail.cell.top, detail.cell.right, detail.cell.bottom],
      content: all.length ? [Math.min(...all.map(item => item.left)), Math.min(...all.map(item => item.top)), Math.max(...all.map(item => item.right)), Math.max(...all.map(item => item.bottom))] : [],
    };
  }
  if (detail.value && detail.rects) return { value: detail.value, lines: detail.rects.length, lineTops: detail.rects.map(item => item.top) };
  return detail;
};
const check = (caseID, assertion, pass, detail = undefined) => {
  counts.checks++;
  if (pass) counts.passed++;
  else {
    counts.failed++;
    failures.push({ case: caseID, assertion, ...(detail === undefined ? {} : { detail: compact(detail) }) });
  }
};
const sha = value => createHash('sha256').update(value).digest('hex');

const profile = await mkdtemp(join(tmpdir(), 'appkit-fathom-metrics-'));
await chmod(profile, 0o700);
await mkdir(join(destination, 'screenshots'));
let context;
let server;
let fatal;
let browserVersion;
let currentOrigin = '';
let allowedPaths = new Set();
let canariesRun = false;
let separatorCanaryRun = false;

try {
  context = await chromium.launchPersistentContext(profile, {
    executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    headless: true,
    viewport: { width: 320, height: 1000 },
    deviceScaleFactor: 1,
    serviceWorkers: 'block',
    acceptDownloads: false,
    args: ['--no-proxy-server', '--disable-background-networking', '--disable-component-update'],
  });
  browserVersion = context.browser()?.version();
  await context.route('**/*', async route => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.origin === currentOrigin && allowedPaths.has(url.pathname + url.search) && ['GET', 'HEAD'].includes(request.method())) {
      await route.continue();
      return;
    }
    counts.blocked++;
    failures.push({ case: 'network', assertion: 'fixed loopback document/CSS allowlist', detail: { method: request.method(), url: request.url() } });
    await route.abort();
  });
  await context.routeWebSocket('**/*', socket => {
    counts.blocked++;
    failures.push({ case: 'network', assertion: 'no WebSockets', detail: socket.url() });
    socket.close();
  });
  const pages = context.pages();
  const page = pages[0] || await context.newPage();
  for (const extra of pages.slice(1)) await extra.close();
  page.setDefaultTimeout(5000);
  page.on('pageerror', error => failures.push({ case: 'browser', assertion: 'no page errors', detail: error.message }));
  page.on('console', message => {
    if (message.type() === 'error') failures.push({ case: 'browser', assertion: 'no console errors', detail: message.text() });
  });

  for (const scenario of scenarios) for (const theme of themes) {
    const mount = join(source, scenario, theme);
    const htmlPath = join(mount, 'impact/index.html');
    const cssPath = join(mount, 'assets/app.css');
    const html = await readFile(htmlPath);
    const css = await readFile(cssPath);
    check(`${scenario}/${theme}`, 'HTML payload budget', html.length <= limits.htmlBytes, html.length);
    check(`${scenario}/${theme}`, 'CSS payload budget', css.length <= limits.cssBytes, css.length);
    evidence.push(`${relative(source, htmlPath)}\t${html.length}\t${sha(html)}`);
    evidence.push(`${relative(source, cssPath)}\t${css.length}\t${sha(css)}`);
    const bodies = new Map([['/observatory/impact', html], ['/assets/app.css', css]]);

    server = createServer((request, response) => {
      counts.requests++;
      const expectedHost = currentOrigin.slice('http://'.length);
      if (request.headers.host !== expectedHost) { response.writeHead(403); response.end(); return; }
      if (!['GET', 'HEAD'].includes(request.method)) { response.writeHead(405, { Allow: 'GET, HEAD' }); response.end(); return; }
      const body = bodies.get(request.url);
      if (!body) { response.writeHead(404); response.end(); return; }
      response.writeHead(200, {
        'Content-Type': request.url === '/assets/app.css' ? 'text/css; charset=utf-8' : 'text/html; charset=utf-8',
        'Content-Length': body.length,
        'Cache-Control': 'no-store',
        'X-Content-Type-Options': 'nosniff',
        'Referrer-Policy': 'no-referrer',
        'Content-Security-Policy': "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'; frame-ancestors 'none'",
      });
      response.end(request.method === 'HEAD' ? undefined : body);
    }).listen(0, '127.0.0.1');
    await once(server, 'listening');
    currentOrigin = `http://127.0.0.1:${server.address().port}`;
    allowedPaths = new Set(bodies.keys());

    for (const width of widths) {
      const caseID = `${scenario}/${theme}/${width}`;
      counts.cases++;
      await page.setViewportSize({ width, height: 1000 });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      await page.goto(currentOrigin + '/observatory/impact', { waitUntil: 'domcontentloaded' });
      await page.evaluate(async () => {
        await document.fonts.ready;
        await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      });

      const pageFacts = await page.evaluate(() => ({
        theme: document.documentElement.dataset.theme,
        scripts: document.scripts.length,
        forms: document.forms.length,
        elements: document.querySelectorAll('*').length,
        overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
        dclMS: performance.getEntriesByType('navigation')[0]?.domContentLoadedEventEnd ?? Infinity,
        zoom: { devicePixelRatio, visualViewportScale: visualViewport?.scale ?? null },
      }));
      check(caseID, 'requested theme rendered', pageFacts.theme === theme, pageFacts.theme);
      check(caseID, 'inert document', pageFacts.scripts === 0 && pageFacts.forms === 0, pageFacts);
      check(caseID, 'bounded DOM and load', pageFacts.elements <= limits.domElements && pageFacts.dclMS <= limits.dclMS, pageFacts);
      check(caseID, 'no document horizontal overflow', !pageFacts.overflow, pageFacts.overflow);
      check(caseID, '100% browser scale', pageFacts.zoom.devicePixelRatio === 1 && pageFacts.zoom.visualViewportScale === 1, pageFacts.zoom);

      const { metrics, colors } = await page.locator('main').evaluate(inspectMetricTypography);
      check(caseID, 'exactly nine metric values', metrics.length === 9, metrics.length);
      counts.metrics += metrics.length;
      for (const metric of metrics) {
        const id = `${caseID}/metric-${metric.index}:${metric.label || 'unlabelled'}`;
        check(id, 'value is visible and a single Range line', metric.visible && metric.singleLine, { value: metric.value, rects: metric.valueRects });
        check(id, 'value and caption stay within stat cell', metric.bounded, { value: metric.value, caption: metric.caption, cell: metric.cellRect, valueRects: metric.valueRects, captionRects: metric.captionRects });
        check(id, 'value and caption do not overlap', metric.separated, { value: metric.value, caption: metric.caption, valueRects: metric.valueRects, captionRects: metric.captionRects });
      }

      const luminance = rgba => rgba.slice(0, 3).map(value => value / 255).map(value => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4).reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0);
      const ratio = sample => {
        if (sample.unsupported.length) return null;
        const foreground = luminance(sample.foreground), background = luminance(sample.background);
        return (Math.max(foreground, background) + 0.05) / (Math.min(foreground, background) + 0.05);
      };
      for (const sample of colors) {
        counts.labels += sample.role === 'value' ? 0 : 1;
        const id = `${caseID}/${sample.role}-${sample.index}:${sample.label || 'unlabelled'}`;
        const measured = ratio(sample);
        check(id, 'color/background composition supported', sample.unsupported.length === 0, sample);
        if (sample.role === 'value') {
          const largeBold = sample.fontSize >= 24 && sample.fontWeight >= 600;
          check(id, 'metric value is at least 24px and bold', largeBold, { fontSize: sample.fontSize, fontWeight: sample.fontWeight });
          check(id, 'large metric contrast is at least 3:1', measured !== null && measured >= 3, { ratio: measured, foreground: sample.foreground, background: sample.background });
        } else {
          check(id, 'label/caption contrast is at least 4.5:1', measured !== null && measured >= 4.5, { ratio: measured, foreground: sample.foreground, background: sample.background, text: sample.text });
        }
      }

      const bands = await page.locator('main').evaluate(inspectBandDividers);
      check(caseID, 'exactly three stat bands', bands.length === 3, bands.length);
      counts.bands += bands.length;
      for (const band of bands) {
        const separatorCount = band.vertical.length + band.horizontal.length;
        counts.separators += separatorCount;
        const id = `${caseID}/band-${band.index}`;
        check(id, 'clipped wrapper owns rounded outer edge', band.clips && band.wrapperOwnsEdge && band.offsetAtClipEdge, band);
        check(id, 'every cell owns one-pixel bottom/right borders', band.cellsOwnBorders, band.cells);
        check(id, 'last row/column borders are clipped without stray separators', band.trailingRightClipped && band.trailingBottomClipped, band);
        check(id, 'internal separator bounds are measurable', separatorCount > 0 && band.internalBounds, { vertical: band.vertical, horizontal: band.horizontal });
      }

      if (!canariesRun) {
        const chosen = metrics.find(metric => metric.value.length >= 3 && metric.caption);
        if (chosen) {
          const locator = page.locator('main .text-2xl').nth(chosen.index);
          const mutate = mode => locator.evaluate((element, requested) => {
            const pair = element.parentElement;
            const caption = [...pair.children].find(child => child !== element && child.textContent.trim());
            const saved = [element, pair, caption].map(node => node.getAttribute('style'));
            if (requested === 'nowrap') element.style.cssText += ';display:inline-block;width:max-content;max-width:none;white-space:nowrap';
            if (requested === 'wrap') element.style.cssText += ';width:1px;max-width:1px;white-space:normal;overflow-wrap:anywhere';
            if (requested === 'overlap') {
              pair.style.cssText += ';position:relative;min-height:3rem';
              element.style.cssText += ';position:absolute;left:0;top:0';
              caption.style.cssText += ';position:absolute;left:0;top:0';
            }
            return saved;
          }, mode);
          const restore = saved => locator.evaluate((element, original) => {
            const pair = element.parentElement;
            const caption = [...pair.children].find(child => child !== element && child.textContent.trim());
            [element, pair, caption].forEach((node, index) => original[index] === null ? node.removeAttribute('style') : node.setAttribute('style', original[index]));
          }, saved);
          const original = await mutate('nowrap');
          const nowrap = await page.locator('main').evaluate(inspectMetricTypography);
          await mutate('wrap');
          const wrapped = await page.locator('main').evaluate(inspectMetricTypography);
          await restore(original);
          const overlapOriginal = await mutate('overlap');
          const overlapped = await page.locator('main').evaluate(inspectMetricTypography);
          await restore(overlapOriginal);
          const baselineColor = colors.find(sample => sample.role === 'value' && sample.label === chosen.label && sample.text === chosen.value);
          const contrastOriginal = await locator.evaluate((element, background) => {
            const pair = element.parentElement;
            const caption = [...pair.children].find(child => child !== element && child.textContent.trim());
            const saved = [element, pair, caption].map(node => node.getAttribute('style'));
            element.style.color = `rgb(${background[0]} ${background[1]} ${background[2]})`;
            return saved;
          }, baselineColor.background);
          const lowContrast = await page.locator('main').evaluate(inspectMetricTypography);
          await restore(contrastOriginal);
          const restored = await page.locator('main').evaluate(inspectMetricTypography);
          const metricAt = result => result.metrics[chosen.index];
          const valueColor = result => result.colors.find(sample => sample.role === 'value' && sample.label === chosen.label && sample.text === chosen.value);
          const lowSample = valueColor(lowContrast);
          const canary = {
            value: chosen.value,
            nowrap: metricAt(nowrap), wrapped: metricAt(wrapped), overlapped: metricAt(overlapped), restored: metricAt(restored),
            lowContrast: lowSample ? ratio(lowSample) : null,
          };
          check(caseID, 'synthetic unit-wrap canary trips single-line oracle', canary.nowrap.singleLine && !canary.wrapped.singleLine, canary);
          check(caseID, 'synthetic overlap canary trips separation oracle', canary.nowrap.separated && !canary.overlapped.separated, canary);
          check(caseID, 'synthetic low-contrast canary trips contrast oracle', canary.lowContrast !== null && canary.lowContrast < 3, canary);
          check(caseID, 'synthetic canaries restore original geometry', canary.restored.singleLine === chosen.singleLine && canary.restored.separated === chosen.separated && canary.restored.bounded === chosen.bounded, canary);
          canaryResults.unitWrap = canary.nowrap.singleLine && !canary.wrapped.singleLine;
          canaryResults.captionOverlap = canary.nowrap.separated && !canary.overlapped.separated;
          canaryResults.lowContrast = canary.lowContrast !== null && canary.lowContrast < 3;
          canaryResults.mutationsRestored = canary.restored.singleLine === chosen.singleLine && canary.restored.separated === chosen.separated && canary.restored.bounded === chosen.bounded;
          canariesRun = true;
        }
      }

      if (!separatorCanaryRun && bands.length) {
        const cell = page.locator('main dl.grid').first().locator(':scope > div').nth(1);
        const originalStyle = await cell.evaluate(element => {
          const saved = element.getAttribute('style');
          element.style.borderBottomWidth = '0px';
          return saved;
        });
        const removed = await page.locator('main').evaluate(inspectBandDividers);
        await cell.evaluate((element, saved) => saved === null ? element.removeAttribute('style') : element.setAttribute('style', saved), originalStyle);
        const restoredBands = await page.locator('main').evaluate(inspectBandDividers);
        canaryResults.separatorRemoval = removed[0]?.valid === false && removed[0]?.cellsOwnBorders === false;
        const separatorRestored = restoredBands[0]?.valid === bands[0]?.valid;
        check(caseID, 'synthetic internal-border removal trips shared-border oracle', canaryResults.separatorRemoval, { removed: removed[0], restored: restoredBands[0] });
        check(caseID, 'synthetic separator canary restores original geometry', separatorRestored, { original: bands[0], restored: restoredBands[0] });
        canaryResults.mutationsRestored = canaryResults.mutationsRestored && separatorRestored;
        separatorCanaryRun = true;
      }

      if ((width === 320 && screenshotScenarios.has(scenario)) || (width === 768 && scenario === 'complete-zero')) {
        const file = `screenshots/${scenario}-${theme}-${width}-impact.png`;
        await page.screenshot({ path: join(destination, file), fullPage: true });
        counts.screenshots++;
      }
    }
    server.closeIdleConnections();
    server.closeAllConnections();
    await new Promise(resolveClose => server.close(resolveClose));
    server = undefined;
    currentOrigin = '';
    allowedPaths = new Set();
  }
} catch (error) {
  fatal = error.stack || String(error);
} finally {
  if (server) {
    server.closeIdleConnections();
    server.closeAllConnections();
    await new Promise(resolveClose => server.close(resolveClose));
  }
  if (context) await context.close();
  await rm(profile, { recursive: true, force: true });
  const screenshots = (await readdir(join(destination, 'screenshots'))).sort().map(file => `screenshots/${file}`);
  check('evidence', 'exactly eight distinct screenshots', screenshots.length === 8 && counts.screenshots === 8, { files: screenshots.length, captures: counts.screenshots });
  const report = {
    schema: 1,
    status: !fatal && failures.length === 0 && counts.blocked === 0 ? 'pass' : 'fail',
    boundary: 'Fixed inert synthetic impact HTML in fresh Chrome at 100%; no live handlers, providers, real data, production latency, OS chrome, or accessibility certification.',
    source,
    destination,
    playwrightModule: modulePath,
    browser: browserVersion,
    host: { platform: process.platform, architecture: process.arch },
    matrix: { scenarios, themes, widths, expectedCases: scenarios.length * themes.length * widths.length },
    limits,
    counts,
    inputManifest: { files: evidence.length, sha256: sha(evidence.sort().join('\n')) },
    canaries: { exercised: canariesRun, ...canaryResults },
    screenshots,
    failures,
    ...(fatal ? { fatal } : {}),
  };
  await writeFile(join(destination, 'report.json'), JSON.stringify(report, null, 2) + '\n', { flag: 'wx' });
  console.log(JSON.stringify({ status: report.status, counts, failures: failures.slice(0, 12), fatal, report: join(destination, 'report.json'), screenshots }));
  if (report.status !== 'pass') process.exitCode = 1;
}
