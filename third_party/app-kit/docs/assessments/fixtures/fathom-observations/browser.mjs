// Isolated synthetic consumer proof; no operational Fathom routes or live data.
// node browser.mjs <export-dir> <empty-report-dir> <playwright/index.mjs>
import { createServer } from 'node:http';
import { readFile, writeFile, mkdir, realpath, readdir } from 'node:fs/promises';
import { join, relative, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { createHash } from 'node:crypto';
import { once } from 'node:events';

const [input, output, playwrightModule] = process.argv.slice(2);
if (!input || !output || !playwrightModule) throw Error('export, empty report, and isolated Playwright module paths required');
const source = await realpath(input), destination = await realpath(output);
for (const dir of [source, destination]) {
  if (!dir.includes('/workspace/tmp/')) throw Error('artifacts must stay in a workspace/tmp child');
}
if (!(relative(source, destination).startsWith('..'))) throw Error('report must not mutate the export');
if ((await readdir(destination)).length) throw Error('report directory must be empty');
const { chromium } = await import(pathToFileURL(resolve(playwrightModule)).href);
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const report = { checks: [], errors: [], blocked: [], pages: [], journeys: [], evidenceJourneys: [], screenshots: [], sources: [],
  budgets: { htmlBytes: 163840, cssBytes: 102400, domElements: 2500, localDOMContentLoadedMS: 1500, detailTargetPX: 44 },
  boundary: 'Fixed inert synthetic HTML, Chrome only; no live handlers, providers, production latency or accessibility certification.' };
const check = (name, pass, detail) => report.checks.push({ name, pass, detail });
// Chrome resolves authored OKLCH into opaque 8-bit sRGB canvas samples below.
// A non-opaque result is unknown, never a pass; record authored colors as well.
const contrast = ({ foregroundRGBA, backgroundRGBA }) => {
  const luminance = n => n.map(v => v / 255).map(v => v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4).reduce((sum, v, i) => sum + v * [0.2126, 0.7152, 0.0722][i], 0);
  if (foregroundRGBA?.[3] !== 255 || backgroundRGBA?.[3] !== 255) return null;
  const foreground = foregroundRGBA.slice(0, 3), backdrop = backgroundRGBA.slice(0, 3);
  const a = luminance(foreground), b = luminance(backdrop);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
};
const scenarios = {
  'all-unknown': { value: '—', note: 'no observation counts available', services: [['alpha', 'no observations file'], ['beta', 'no observations file'], ['gamma', 'no observations file']] },
  partial: { value: '≥ 4', note: '2 of 3 repos reporting; lower bound', services: [['alpha', '0'], ['beta', '4'], ['gamma', 'no observations file']] },
  'complete-zero': { value: '0', note: '3 of 3 repos reporting', services: [['alpha', '0'], ['beta', '0'], ['gamma', '0']] },
  'complete-positive': { value: '6', note: '3 of 3 repos reporting', services: [['alpha', '0'], ['beta', '4'], ['gamma', '2']] },
  'registry-unavailable': { value: '—', note: 'Synthetic service registry unavailable; observation coverage is unknown, never zero.', registryNote: 'Synthetic service registry unavailable; observation coverage is unknown, never zero.' },
  'registry-partial': { value: '≥ 6', note: '3 of 3 listed repos reporting; lower bound; fleet coverage unavailable: Synthetic registry incomplete; listed services only.', registryNote: 'Synthetic registry incomplete; listed services only.', services: [['alpha', '0'], ['beta', '4'], ['gamma', '2']] },
  'call-log-unavailable': { value: '6', note: '3 of 3 repos reporting', callLogNote: 'Synthetic call log unavailable; traffic counts are unknown.', services: [['alpha', '0'], ['beta', '4'], ['gamma', '2']] },
  'registry-call-log-unavailable': { value: '—', note: 'Synthetic service registry unavailable; observation coverage is unknown, never zero.', registryNote: 'Synthetic service registry unavailable; observation coverage is unknown, never zero.', callLogNote: 'Synthetic call log unavailable; traffic counts are unknown.' },
  'measured-zero-catches': { value: '6', note: '3 of 3 repos reporting', catches: '0', catchesNote: 'Synthetic verify ledger: 2 reporting sessions.', services: [['alpha', '0'], ['beta', '4'], ['gamma', '2']] },
  'measured-positive-catches': { value: '6', note: '3 of 3 repos reporting', catches: '3', catchesNote: 'Synthetic verify ledger: 2 reporting sessions.', services: [['alpha', '0'], ['beta', '4'], ['gamma', '2']] },
};
const flatText = async node => (await node.innerText()).replace(/\s+/g, ' ').trim();
const q2Label = 'Q2 · what did fathom prevent?';
const q3Label = 'Q3 · where does the agent still flounder?';
const homeQuestion = (page, label) => page.locator('section').filter({ has: page.getByText(label, { exact: true }) });
const callContext = expected => expected.callLogNote ? 'per-tool outcomes unavailable: ' + expected.callLogNote
  : expected.registryNote ? 'per-tool outcomes unavailable: ' + expected.registryNote : 'plus per-tool miss rates from 30 logged calls';
const expectedQ2 = expected => `${q2Label} ${expected.catches ?? '—'} verify catches ${expected.catchesNote ? expected.catchesNote + ' ' : ''}see the evidence →`;
const expectedQ3 = expected => `${q3Label} ${expected.value} open observations ${expected.note}; ${callContext(expected)} see the evidence →`;
const keyboardFollow = async (page, link, url, tag) => {
  const target = await link.boundingBox();
  check(`${tag}: evidence target`, target && target.width >= 44 && target.height >= 44, target);
  const beforeShadow = await link.evaluate(el => getComputedStyle(el).boxShadow);
  await link.focus();
  const focus = await link.evaluate(el => ({ active: el === document.activeElement, tabIndex: el.tabIndex, outline: getComputedStyle(el).outlineWidth, style: getComputedStyle(el).outlineStyle, shadow: getComputedStyle(el).boxShadow }));
  check(`${tag}: visible keyboard focus`, focus.active && focus.tabIndex >= 0 && ((parseFloat(focus.outline) > 0 && !['none', 'hidden'].includes(focus.style)) || (focus.shadow !== 'none' && focus.shadow !== beforeShadow)), focus);
  await Promise.all([page.waitForURL(url), page.keyboard.press('Enter')]);
  check(`${tag}: exact evidence route`, page.url() === url, page.url());
};
const detailScreenshots = new Set(['all-unknown/light/320/alpha', 'all-unknown/dark/1440/gamma', 'complete-zero/light/768/beta', 'registry-partial/dark/320/gamma']);
await mkdir(join(destination, 'screenshots'));
const browser = await chromium.launch({
  executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true,
  args: ['--no-proxy-server', '--disable-background-networking'],
});
report.browser = browser.version();
report.host = { platform: process.platform, architecture: process.arch };
let currentServer;
try {
  for (const [scenario, expected] of Object.entries(scenarios)) for (const theme of ['light', 'dark']) {
    const mount = join(source, scenario, theme);
    const routes = new Map([
      ['/observatory', 'overview/index.html'], ['/observatory/impact', 'impact/index.html'],
      ['/', 'home/index.html'], ['/observatory/story', 'story/index.html'], ['/assets/app.css', 'assets/app.css'],
    ]);
    if (expected.services) for (const [id] of expected.services) {
      routes.set('/observatory/service?id=' + id, 'service/' + id + '/index.html');
    }
    const bodies = new Map();
    for (const [route, file] of routes) {
      const data = await readFile(join(mount, file));
      bodies.set(route, data);
      report.sources.push({ file: relative(source, join(mount, file)), bytes: data.length, sha256: sha(data) });
      check(`${scenario}/${theme}${route}: payload budget`, data.length <= (file.endsWith('.css') ? report.budgets.cssBytes : report.budgets.htmlBytes), data.length);
    }
    const requests = [];
    currentServer = createServer((req, res) => {
      requests.push({ method: req.method, url: req.url });
      if (!['GET', 'HEAD'].includes(req.method)) { res.writeHead(405); res.end(); return; }
      const bytes = bodies.get(req.url);
      if (!bytes) { res.writeHead(404); res.end(); return; }
      res.writeHead(200, { 'Content-Type': req.url === '/assets/app.css' ? 'text/css' : 'text/html', 'Cache-Control': 'no-store' });
      res.end(req.method === 'HEAD' ? undefined : bytes);
    }).listen(0, '127.0.0.1');
    await once(currentServer, 'listening');
    const origin = `http://127.0.0.1:${currentServer.address().port}`;
    check(`${scenario}/${theme}: denied mutation`, (await fetch(origin + '/observatory', { method: 'POST' })).status === 405);
    check(`${scenario}/${theme}: denied unknown route`, (await fetch(origin + '/api/observatory')).status === 404);
    check(`${scenario}/${theme}: denied arbitrary service`, (await fetch(origin + '/observatory/service?id=outside')).status === 404);
    const context = await browser.newContext({ serviceWorkers: 'block', acceptDownloads: false, deviceScaleFactor: 1 });
    await context.route('**/*', async route => {
      const req = route.request(), url = new URL(req.url());
      if (url.origin === origin && bodies.has(url.pathname + url.search) && ['GET', 'HEAD'].includes(req.method())) await route.continue();
      else { report.blocked.push(req.url()); await route.abort(); }
    });
    await context.routeWebSocket('**/*', ws => { report.blocked.push(ws.url()); ws.close(); });
    const page = await context.newPage();
    page.setDefaultTimeout(5000);
    page.on('pageerror', e => report.errors.push(e.message));
    page.on('console', m => { if (m.type() === 'error') report.errors.push(m.text()); });
    for (const width of [320, 768, 1440]) {
      const tag = `${scenario}/${theme}/${width}`;
      await page.setViewportSize({ width, height: 1000 });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: 'reduce' });
      for (const route of ['/', '/observatory', '/observatory/impact', '/observatory/story']) {
        await page.goto(origin + route);
        await page.evaluate(async () => { await document.fonts.ready; await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))); });
        const label = route === '/' ? 'Q3 · where does the agent still flounder?' : route.endsWith('/impact') ? 'open observations across the fleet' : route.endsWith('/story') ? 'open observations (fleet snapshot)' : 'open observations';
        const labelNode = route.endsWith('/story')
          ? page.getByText(/^open observations \(fleet(?: snapshot)?\)$/).first()
          : page.getByText(label, { exact: true }).first();
        const row = route === '/' ? page.locator('section').filter({ has: labelNode }) : labelNode.locator('..');
        const text = (await row.innerText()).replace(/\s+/g, ' ');
        const actualLabel = await labelNode.innerText();
        const expectedText = route === '/' ? expectedQ3(expected) : `${label} ${expected.value} ${expected.note}`;
        check(`${tag}${route}: snapshot label`, actualLabel === label, actualLabel);
        check(`${tag}${route}: exact observation and coverage`, text === expectedText, { actual: text, expected: expectedText });
        if (route === '/') {
          check(`${tag}: exact catch value and source`, await flatText(homeQuestion(page, q2Label)) === expectedQ2(expected));
          if (expected.catches === undefined) check(`${tag}: unavailable catches are not zero`, await page.getByText('— verify catches', { exact: true }).count() === 1);
        }
        if (route === '/observatory/impact' && expected.callLogNote) {
          const cost = page.getByText('total response cost (est.)', { exact: true }).locator('..');
          const actual = await flatText(cost);
          check(`${tag}: unreadable response cost is absent with exact reason`, actual === `total response cost (est.) — ${expected.callLogNote}`, actual);
          const ratio = page.getByText('tokens per discovery action', { exact: true }).locator('..');
          check(`${tag}: unreadable cost ratio has exact reason`, await flatText(ratio) === `tokens per discovery action — ${expected.callLogNote}`);
          const outcomes = page.locator('section').filter({ has: page.getByRole('heading', { name: 'per-tool outcomes', exact: true }) });
          const outcomesText = await outcomes.count() === 1 ? await flatText(outcomes) : null;
          check(`${tag}: per-tool outcomes retain source failure`, outcomesText === `per-tool outcomes ${expected.callLogNote}`, outcomesText);
        }
        if (expected.value.startsWith('≥')) {
          const headline = row.locator('dd > div > span, .text-right > span, .text-xl').first();
          const lines = await headline.evaluate((el, length) => {
            const range = document.createRange(); range.setStart(el.firstChild, 0); range.setEnd(el.firstChild, length);
            return [...range.getClientRects()].filter(r => r.width > 0).length;
          }, expected.value.length);
          check(`${tag}${route}: lower-bound sign stays with number`, lines === 1, lines);
        }
        const geometry = await row.evaluate(el => {
          const box = el.getBoundingClientRect(), ranges = [], walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
          while (walker.nextNode()) { const r = document.createRange(); r.selectNodeContents(walker.currentNode); ranges.push(...[...r.getClientRects()].map(x => ({ left: x.left, right: x.right, top: x.top, bottom: x.bottom }))); }
          return { box: { left: box.left, right: box.right, top: box.top, bottom: box.bottom }, ranges };
        });
        check(`${tag}${route}: observation glyph bounds`, geometry.ranges.every(r => r.left >= geometry.box.left - 1 && r.right <= geometry.box.right + 1 && r.top >= geometry.box.top - 1 && r.bottom <= geometry.box.bottom + 1), geometry);
        const colors = await row.evaluate(el => {
          const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
          const ctx = canvas.getContext('2d', { colorSpace: 'srgb', willReadFrequently: true });
          const rgba = color => { ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = color; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data]; };
          return [...el.querySelectorAll('*')].filter(e => [...e.childNodes].some(n => n.nodeType === Node.TEXT_NODE && n.textContent.trim())).map(e => {
            let p = e, background = '';
            while (p) { background = getComputedStyle(p).backgroundColor; if (rgba(background)[3] !== 0) break; p = p.parentElement; }
            const color = getComputedStyle(e).color;
            return { text: e.textContent, color, background, foregroundRGBA: rgba(color), backgroundRGBA: rgba(background) };
          });
        });
        const ratios = colors.map(c => ({ ...c, contrast: contrast(c) }));
        check(`${tag}${route}: sampled metric text contrast`, ratios.length > 0 && ratios.every(c => c.contrast !== null && c.contrast >= 4.5), ratios);
        const measured = await page.evaluate(() => ({
          overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
          elements: document.querySelectorAll('*').length, scripts: document.scripts.length, forms: document.forms.length,
          theme: document.documentElement.dataset.theme,
          dcl: performance.getEntriesByType('navigation')[0].domContentLoadedEventEnd,
        }));
        report.pages.push({ tag, route, ...measured });
        check(`${tag}${route}: bounded page`, !measured.overflow, measured);
        check(`${tag}${route}: inert and themed`, measured.scripts === 0 && measured.forms === 0 && measured.theme === theme, measured);
        check(`${tag}${route}: DOM and local timing budgets`, measured.elements <= report.budgets.domElements && measured.dcl <= report.budgets.localDOMContentLoadedMS, measured);
        if ((scenario === 'partial' && width !== 768) || (scenario === 'all-unknown' && width === 320) || (expected.callLogNote && width === 320 && route === '/observatory/impact')) {
          const file = `screenshots/${scenario}-${theme}-${width}-${route.split('/').pop() || 'home'}.png`;
          await page.screenshot({ path: join(destination, file), fullPage: true }); report.screenshots.push(file);
        }
      }
      for (const label of [q2Label, q3Label]) {
        await page.goto(origin + '/');
        const started = performance.now();
        const failedBefore = report.checks.filter(c => !c.pass).length;
        await keyboardFollow(page, homeQuestion(page, label).getByRole('link', { name: 'see the evidence →', exact: true }), origin + '/observatory/impact', `${tag}/${label}`);
        check(`${tag}/${label}: impact heading`, await page.getByRole('heading', { level: 1, name: 'fathom · impact', exact: true }).count() === 1);
        const catchRow = page.getByText('verify catches', { exact: true }).locator('..');
        const catchExpected = `verify catches ${expected.catches ?? '—'} ${expected.catchesNote || 'not measured'}`;
        check(`${tag}/${label}: impact preserves catch source`, await flatText(catchRow) === catchExpected, await flatText(catchRow));
        check(`${tag}/${label}: impact preserves observation coverage`, await flatText(page.getByText('open observations across the fleet', { exact: true }).locator('..')) === `open observations across the fleet ${expected.value} ${expected.note}`);
        await keyboardFollow(page, page.getByRole('link', { name: 'Inspect session evidence', exact: true }), origin + '/observatory/story', `${tag}/${label}/session`);
        check(`${tag}/${label}: explicit synthetic ledger source`, await page.getByText('Synthetic session summary. No production ledger is read.', { exact: true }).count() === 1);
        check(`${tag}/${label}: weekly catches retain separate period`, await flatText(page.getByText('verify catches', { exact: true }).locator('..')) === 'verify catches 0 verify ran in 2 of 2 sessions');
        check(`${tag}/${label}: story keeps fleet snapshot separate`, await flatText(page.getByText('open observations (fleet snapshot)', { exact: true }).locator('..')) === `open observations (fleet snapshot) ${expected.value} ${expected.note}`);
        if (width < 768) { await page.getByText('Menu', { exact: true }).focus(); await page.keyboard.press('Enter'); }
        const nav = page.locator(width < 768 ? 'nav[aria-label="Primary (mobile)"]' : 'nav[aria-label="Primary"]');
        await keyboardFollow(page, nav.getByRole('link', { name: 'home', exact: true }), origin + '/', `${tag}/${label}/return`);
        check(`${tag}/${label}: exact home catch return`, await flatText(homeQuestion(page, q2Label)) === expectedQ2(expected));
        check(`${tag}/${label}: exact home coverage return`, await flatText(homeQuestion(page, q3Label)) === expectedQ3(expected));
        check(`${tag}/${label}: home theme retained`, await page.locator('html').getAttribute('data-theme') === theme);
        report.evidenceJourneys.push({ tag, question: label, automatedMS: performance.now() - started, failures: report.checks.filter(c => !c.pass).length - failedBefore, boundary: 'Script-paced synthetic task, not human completion time.' });
      }
      if (!expected.services) {
        await page.goto(origin + '/observatory');
        const detailLinks = await page.locator('a[href^="/observatory/service?id="]').count();
        const knownStatus = (await fetch(origin + '/observatory/service?id=alpha')).status;
        check(`${tag}: unavailable registry has no detail links`, detailLinks === 0, detailLinks);
        check(`${tag}: unavailable registry refuses known service route`, knownStatus === 404, knownStatus);
        report.journeys.push({ tag, scenario, theme, width, service: null, detailLinks, knownStatus });
      }
      for (const [id, value] of expected.services || []) {
        await page.goto(origin + '/observatory');
        const detail = page.locator(`a[href="/observatory/service?id=${id}"]`);
        const target = await detail.boundingBox();
        check(`${tag}/${id}: detail target`, target && target.height >= report.budgets.detailTargetPX && target.width >= report.budgets.detailTargetPX, target);
        await detail.focus();
        const focus = await detail.evaluate(el => ({ active: el === document.activeElement, tabIndex: el.tabIndex, outline: getComputedStyle(el).outlineWidth, style: getComputedStyle(el).outlineStyle }));
        check(`${tag}/${id}: keyboard focus indicator`, focus.active && focus.tabIndex >= 0 && parseFloat(focus.outline) >= 2 && focus.style !== 'none', focus);
        await Promise.all([page.waitForURL(origin + '/observatory/service?id=' + id), page.keyboard.press('Enter')]);
        await page.evaluate(async () => { await document.fonts.ready; await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))); });
        const detailURL = page.url();
        const text = (await page.getByText('open observations', { exact: true }).locator('..').innerText()).replace(/\s+/g, ' ');
        check(`${tag}/${id}: service evidence`, text === 'open observations ' + value, text);
        check(`${tag}/${id}: exact destination`, detailURL === origin + '/observatory/service?id=' + id, detailURL);
        check(`${tag}/${id}: exact service heading`, await page.getByRole('heading', { level: 1, name: 'fathom · ' + id, exact: true }).count() === 1);
        const detailMeasured = await page.evaluate(() => ({
          overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
          elements: document.querySelectorAll('*').length, scripts: document.scripts.length, forms: document.forms.length,
          theme: document.documentElement.dataset.theme,
          dcl: performance.getEntriesByType('navigation')[0].domContentLoadedEventEnd,
        }));
        check(`${tag}/${id}: bounded detail page`, !detailMeasured.overflow && detailMeasured.elements <= report.budgets.domElements && detailMeasured.dcl <= report.budgets.localDOMContentLoadedMS, detailMeasured);
        check(`${tag}/${id}: inert themed detail page`, detailMeasured.scripts === 0 && detailMeasured.forms === 0 && detailMeasured.theme === theme, detailMeasured);
        if (detailScreenshots.has(`${scenario}/${theme}/${width}/${id}`)) {
          const file = `screenshots/${scenario}-${theme}-${width}-service-${id}.png`;
          await page.screenshot({ path: join(destination, file), fullPage: true }); report.screenshots.push(file);
        }
        if (width < 768) { await page.getByText('Menu', { exact: true }).focus(); await page.keyboard.press('Enter'); }
        const nav = page.locator(width < 768 ? 'nav[aria-label="Primary (mobile)"]' : 'nav[aria-label="Primary"]');
        await nav.getByRole('link', { name: 'fleet', exact: true }).focus();
        await Promise.all([page.waitForURL(origin + '/observatory'), page.keyboard.press('Enter')]);
        const returned = (await page.getByText('open observations', { exact: true }).first().locator('..').innerText()).replace(/\s+/g, ' ');
        const expectedReturn = `open observations ${expected.value} ${expected.note}`;
        const returnedTheme = await page.locator('html').getAttribute('data-theme');
        check(`${tag}/${id}: keyboard return retains scenario`, returned === expectedReturn, { actual: returned, expected: expectedReturn });
        check(`${tag}/${id}: keyboard return retains theme`, returnedTheme === theme, returnedTheme);
        report.journeys.push({ tag, scenario, theme, width, service: id, value, detailURL, detail: detailMeasured, returned, returnedTheme, returnURL: page.url() });
      }
    }
    await context.close();
    report.sources.push({ scenario, theme, requests });
    await new Promise(resolve => currentServer.close(resolve)); currentServer = undefined;
  }
} catch (error) { report.fatal = error.stack; }
finally {
  await browser.close();
  if (currentServer) await new Promise(resolve => currentServer.close(resolve));
  report.failed = report.checks.filter(c => !c.pass);
  await writeFile(join(destination, 'report.json'), JSON.stringify(report, null, 2) + '\n', { flag: 'wx' });
  console.log(JSON.stringify({ checks: report.checks.length, failed: report.failed.length, examples: report.failed.slice(0, 5), errors: report.errors, blocked: report.blocked, pages: report.pages.length, screenshots: report.screenshots.length, fatal: report.fatal }));
  if (report.fatal || report.failed.length || report.errors.length || report.blocked.length) process.exitCode = 1;
}
