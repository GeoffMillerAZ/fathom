// Isolated Fathom navigation proof over synthetic exported HTML only.
// node browser.mjs <preview-directory> <empty-report-directory>
import http from 'node:http';
import {createHash} from 'node:crypto';
import {mkdir, readFile, readdir, realpath, writeFile} from 'node:fs/promises';
import {dirname, relative, resolve, sep} from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';

const routes = [
  ['home', '/', 'home'], ['fleet', '/observatory', 'fleet'],
  ['wiring', '/observatory/wiring/', 'wiring'], ['impact', '/observatory/impact', 'impact'],
  ['story', '/observatory/story', 'story'], ['brief', '/observatory/brief', 'brief'],
  ['statusz', '/statusz', 'statusz'],
].map(([key, href, label]) => ({key, href, label}));
const themes = ['light', 'dark'], widths = [320, 768, 1440];
const csp = "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'; frame-ancestors 'none'";
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const args = process.argv.slice(2);
if (args.length !== 2) throw Error('Expected a preview directory and an empty report directory.');
const workspaceTmp = await realpath('/Volumes/m2ext/code/geoffmilleraz/sextant/workspace/tmp');
const [root, out] = await Promise.all(args.map(path => realpath(resolve(path))));
for (const path of [root, out]) {
  const rel = relative(workspaceTmp, path);
  if (!rel || rel === '..' || rel.startsWith('..' + sep) || rel.startsWith(sep)) throw Error('Directories must be distinct existing children of the Fathom workspace/tmp directory.');
}
if (root === out || (await readdir(out)).length) throw Error('Distinct input and empty output required.');

const harnessPath = fileURLToPath(import.meta.url);
const playwrightEntry = await realpath('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs');
const packagePath = await realpath(resolve(dirname(playwrightEntry), 'package.json'));
const packageBytes = await readFile(packagePath), pkg = JSON.parse(packageBytes);
if (pkg.name !== 'playwright' || pkg.version !== '1.63.0') throw Error(`Expected isolated playwright 1.63.0, got ${pkg.name} ${pkg.version}`);
const {chromium} = await import(pathToFileURL(playwrightEntry).href);

const report = {
  started: new Date().toISOString(), root, out,
  harnessSHA256: sha(await readFile(harnessPath)),
  playwright: {entry: playwrightEntry, version: pkg.version, entrySHA256: sha(await readFile(playwrightEntry)), packageSHA256: sha(packageBytes)},
  input: [], checks: [], samples: [], responses: [], screenshots: [], errors: [], blocked: [], downloads: [], workers: [],
  budgets: {textContrast: 4.5, focusContrast: 3, focusWidth: 2, targetSize: 44, navigationMS: 5000, actionMS: 3000},
  boundary: 'Synthetic inert HTML only; no Fathom daemon, store, providers, production routes, or accessibility certification.',
};
const check = (name, pass, detail) => report.checks.push({name, pass: Boolean(pass), detail});
const inputBytes = new Map();
for (const theme of themes) {
  for (const route of routes) {
    const file = resolve(root, theme, route.key, 'index.html'), bytes = await readFile(file);
    inputBytes.set(`${theme}:${route.href}`, bytes);
    report.input.push({file: relative(root, file), bytes: bytes.length, sha256: sha(bytes)});
  }
  const file = resolve(root, theme, 'assets/app.css'), bytes = await readFile(file);
  inputBytes.set(`${theme}:/assets/app.css`, bytes);
  report.input.push({file: relative(root, file), bytes: bytes.length, sha256: sha(bytes)});
}
await mkdir(resolve(out, 'screenshots'));

const focusSample = el => {
  const style = getComputedStyle(el), rect = el.getBoundingClientRect();
  const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
  const ctx = canvas.getContext('2d', {colorSpace: 'srgb', willReadFrequently: true});
  const rgba = css => {ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = css; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data];};
  const composite = (front, back) => {const a = front[3] / 255, b = back[3] / 255, out = a + b * (1 - a); return out ? [0, 1, 2].map(i => Math.round((front[i] * a + back[i] * b * (1 - a)) / out)).concat(Math.round(out * 255)) : [0, 0, 0, 0];};
  const backdrop = start => {let color = [0, 0, 0, 0], unknown = false; for (let node = start; node; node = node.parentElement) {const s = getComputedStyle(node); unknown ||= Number(s.opacity) !== 1 || s.backgroundImage !== 'none'; if (color[3] < 255) color = composite(color, rgba(s.backgroundColor));} return {color, unknown: unknown || color[3] !== 255};};
  const textBG = backdrop(el), focusBG = backdrop(el.parentElement), unknown = textBG.unknown || focusBG.unknown;
  const lum = c => c.slice(0, 3).map(v => {v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4;}).reduce((sum, v, i) => sum + v * [.2126, .7152, .0722][i], 0);
  const ratio = (a, b) => (Math.max(lum(a), lum(b)) + .05) / (Math.min(lum(a), lum(b)) + .05);
  return {tag: el.tagName, href: el.getAttribute('href'), text: el.textContent.trim(), active: el === document.activeElement,
    outlineStyle: style.outlineStyle, outlineWidth: parseFloat(style.outlineWidth), unknown,
    textContrast: unknown ? null : ratio(composite(rgba(style.color), textBG.color), textBG.color),
    focusContrast: unknown ? null : ratio(composite(rgba(style.outlineColor), focusBG.color), focusBG.color),
    rect: {left: rect.left, right: rect.right, width: rect.width, height: rect.height}};
};

let browser, server, origin = '', activeTheme = 'light';
try {
  server = http.createServer((req, res) => {
    const requestURL = new URL(req.url, origin || 'http://127.0.0.1');
    report.responses.push({method: req.method, url: requestURL.pathname + requestURL.search});
    res.setHeader('Content-Security-Policy', csp); res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Referrer-Policy', 'no-referrer'); res.setHeader('Cache-Control', 'no-store');
    if (!origin || req.headers.host !== new URL(origin).host) {res.writeHead(403).end(); return;}
    if (!['GET', 'HEAD'].includes(req.method)) {res.writeHead(405, {Allow: 'GET, HEAD'}).end(); return;}
    if (requestURL.search) {res.writeHead(404).end(); return;}
    const route = routes.find(item => item.href === requestURL.pathname), key = `${activeTheme}:${requestURL.pathname}`;
    if (!route && requestURL.pathname !== '/assets/app.css') {res.writeHead(404).end(); return;}
    const bytes = inputBytes.get(key); if (!bytes) {res.writeHead(404).end(); return;}
    res.writeHead(200, {'Content-Type': requestURL.pathname.endsWith('.css') ? 'text/css; charset=utf-8' : 'text/html; charset=utf-8', 'Content-Length': bytes.length});
    res.end(req.method === 'HEAD' ? undefined : bytes);
  });
  await new Promise((ok, fail) => {server.once('error', fail); server.listen(0, '127.0.0.1', ok);});
  origin = `http://127.0.0.1:${server.address().port}`;
  const probes = await Promise.all([
    fetch(origin + '/api/provider'), fetch(origin + '/observatory', {method: 'POST'}),
    fetch(origin + '/observatory?live=true'), fetch(origin + '/statusz', {method: 'HEAD'}),
  ]);
  check('server boundary', JSON.stringify(probes.map(r => r.status)) === JSON.stringify([404, 405, 404, 200]), probes.map(r => r.status));
  browser = await chromium.launch({executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, args: ['--no-proxy-server', '--disable-background-networking']});
  report.browser = browser.version();

  for (const javaScriptEnabled of [false, true]) for (const theme of themes) {
    activeTheme = theme;
    const context = await browser.newContext({acceptDownloads: false, javaScriptEnabled, serviceWorkers: 'block'});
    context.setDefaultTimeout(report.budgets.actionMS); context.setDefaultNavigationTimeout(report.budgets.navigationMS);
    context.on('serviceworker', worker => report.workers.push(worker.url()));
    await context.route('**/*', async route => {
      const request = route.request(), url = new URL(request.url());
      if (url.origin === origin && ['GET', 'HEAD'].includes(request.method()) && !url.search && (routes.some(item => item.href === url.pathname) || url.pathname === '/assets/app.css')) await route.continue();
      else {report.blocked.push({method: request.method(), url: request.url()}); await route.abort();}
    });
    await context.routeWebSocket('**/*', socket => {report.blocked.push({method: 'WS', url: socket.url()}); socket.close();});
    const page = await context.newPage();
    page.on('pageerror', error => report.errors.push(error.message));
    page.on('console', message => {if (['error', 'warning'].includes(message.type())) report.errors.push(message.text());});
    page.on('download', download => {report.downloads.push(download.suggestedFilename()); void download.cancel();});

    for (const width of widths) for (let routeIndex = 0; routeIndex < routes.length; routeIndex++) {
      const route = routes[routeIndex], tag = `${theme}/${width}/${route.key}/js-${javaScriptEnabled ? 'on' : 'off'}`;
      await page.setViewportSize({width, height: 900}); await page.emulateMedia({colorScheme: theme, reducedMotion: 'reduce'});
      const response = await page.goto(origin + route.href, {waitUntil: 'load'}), headers = await response.headers();
      const state = await page.evaluate(({routes, route, width, theme}) => {
        const visible = el => el.checkVisibility({checkOpacity: true, checkVisibilityCSS: true});
        const navs = [...document.querySelectorAll('nav[aria-label="Primary"], nav[aria-label="Primary (mobile)"]')];
        const inspect = nav => ({label: nav.getAttribute('aria-label'), visible: visible(nav), links: [...nav.querySelectorAll('a')].map(a => ({href: a.getAttribute('href'), label: a.textContent.trim(), current: a.getAttribute('aria-current')}))});
        return {scripts: document.scripts.length, forms: document.forms.length, main: document.querySelectorAll('main#observatory-main[tabindex="-1"]').length,
          skip: document.querySelectorAll('body > a[href="#observatory-main"]').length, current: document.querySelectorAll('a[aria-current="page"]').length,
          navs: navs.map(inspect), width: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth,
          meaning: document.querySelector('#navigation-fixture-title')?.textContent.trim(), bodyLink: document.querySelector('main a')?.getAttribute('href'), target: document.querySelectorAll('#fixture-next').length,
          theme: document.documentElement.dataset.theme, expectedVisible: width >= 768 ? 'Primary' : null,
          expectedMeaning: `Synthetic navigation state: ${route.key}`, expectedRoutes: routes.map(({href, label}) => ({href, label}))};
      }, {routes, route, width, theme});
      report.samples.push({tag, state});
      check(`${tag}: inert exact response`, response.status() === 200 && headers['content-security-policy'] === csp && state.scripts === 0 && state.forms === 0 && state.theme === theme, state);
      check(`${tag}: shell and fixture meaning`, state.main === 1 && state.skip === 1 && state.meaning === state.expectedMeaning && state.bodyLink === '#fixture-next' && state.target === 1, state);
      check(`${tag}: exact routes and current`, state.current === 2 && state.navs.length === 2 && state.navs.every(nav => JSON.stringify(nav.links.map(({href, label}) => ({href, label}))) === JSON.stringify(state.expectedRoutes) && nav.links.filter(link => link.current === 'page').length === 1 && nav.links.find(link => link.current === 'page')?.href === route.href), state.navs);
      check(`${tag}: responsive visibility`, width >= 768 ? state.navs.filter(nav => nav.visible).length === 1 && state.navs.find(nav => nav.visible)?.label === 'Primary' : state.navs.every(nav => !nav.visible), state.navs);
      check(`${tag}: no outer overflow`, state.scrollWidth <= state.width + 1, {width: state.width, scrollWidth: state.scrollWidth});

      await page.keyboard.press('Tab');
      const skip = await page.locator('body > a[href="#observatory-main"]').evaluate(focusSample);
      check(`${tag}: native first tab skip`, skip.active && skip.outlineStyle !== 'none' && skip.outlineWidth >= 2, skip);
      await page.keyboard.press('Enter');
      check(`${tag}: native skip focuses main`, await page.locator('main#observatory-main').evaluate(el => document.activeElement === el));
      await page.goto('about:blank'); await page.goto(origin + route.href, {waitUntil: 'load'}); await page.keyboard.press('Tab'); await page.keyboard.press('Tab');
      const mobile = width < 768, navLabel = mobile ? 'Primary (mobile)' : 'Primary';
      if (mobile) {
        const menu = page.locator('details.md\\:hidden > summary'), sample = await menu.evaluate(focusSample);
        check(`${tag}: native menu focus and target`, sample.active && sample.outlineStyle !== 'none' && sample.outlineWidth >= 2 && sample.rect.width >= 44 && sample.rect.height >= 44, sample);
        await page.keyboard.press('Enter'); check(`${tag}: native menu opens`, await menu.evaluate(el => el.parentElement.open));
        if (!javaScriptEnabled && route.key === 'home') {const file = `screenshots/${theme}-${width}-home-mobile-open.png`; await page.screenshot({path: resolve(out, file), fullPage: true}); report.screenshots.push({file, sha256: sha(await readFile(resolve(out, file)))});}
        await page.keyboard.press('Enter'); check(`${tag}: native menu closes`, !(await menu.evaluate(el => el.parentElement.open)));
        await page.keyboard.press('Enter'); check(`${tag}: native menu reopens`, await menu.evaluate(el => el.parentElement.open));
        await page.keyboard.press('Tab');
      }
      const focused = [];
      for (let i = 0; i < routes.length; i++) {focused.push(await page.locator(':focus').evaluate(focusSample)); if (i + 1 < routes.length) await page.keyboard.press('Tab');}
      check(`${tag}: native seven-route order`, focused.every((item, i) => item.href === routes[i].href), focused);
      check(`${tag}: nav targets and contrast`, focused.every(item => item.active && item.rect.width >= 44 && item.rect.height >= 44 && !item.unknown && item.outlineStyle !== 'none' && item.outlineWidth >= 2 && item.textContrast >= 4.5 && item.focusContrast >= 3), focused);

      const next = routes[(routeIndex + 1) % routes.length];
      await page.locator(`nav[aria-label="${navLabel}"] a[href="${next.href}"]`).click(); await page.waitForURL(origin + next.href);
      check(`${tag}: click next selects exact route`, await page.locator(`nav[aria-label="Primary"] a[href="${next.href}"][aria-current="page"]`).count() === 1);
      await page.goBack({waitUntil: 'load'}); check(`${tag}: browser back restores route`, page.url() === origin + route.href);
      await page.locator('main a[href="#fixture-next"]').click(); check(`${tag}: content target remains local`, page.url() === origin + route.href + '#fixture-next');
      await page.goBack({waitUntil: 'load'}); check(`${tag}: content-target back restores route`, page.url() === origin + route.href);

      if (!javaScriptEnabled && route.key === 'home') {const file = `screenshots/${theme}-${width}-home.png`; await page.screenshot({path: resolve(out, file), fullPage: true}); report.screenshots.push({file, sha256: sha(await readFile(resolve(out, file)))});}
    }
    await context.close();
  }
  check('no unexpected runtime activity', !report.blocked.length && !report.errors.length && !report.downloads.length && !report.workers.length, {blocked: report.blocked, errors: report.errors, downloads: report.downloads, workers: report.workers});
  check('input identities unchanged', (await Promise.all(report.input.map(async item => sha(await readFile(resolve(root, item.file))) === item.sha256))).every(Boolean));
} catch (error) {report.fatal = error.stack;}
finally {
  if (browser) await browser.close(); if (server) await new Promise(ok => server.close(ok));
  report.finished = new Date().toISOString(); report.failed = report.checks.filter(item => !item.pass);
  await writeFile(resolve(out, 'report.json'), JSON.stringify(report, null, 2) + '\n', {mode: 0o600, flag: 'wx'});
  console.log(JSON.stringify({checks: report.checks.length, failed: report.failed.map(item => item.name), errors: report.errors, blocked: report.blocked, screenshots: report.screenshots.length, fatal: report.fatal}, null, 2));
  if (report.fatal || report.failed.length || report.errors.length || report.blocked.length) process.exitCode = 1;
}
