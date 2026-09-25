// Isolated synthetic RAC navigation review; no app server, records, or provider.
// node browser.mjs <preview-directory> <empty-report-directory>
import http from 'node:http';
import {createHash} from 'node:crypto';
import {readFile, readdir, realpath, writeFile} from 'node:fs/promises';
import {dirname, relative, resolve, sep} from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath, pathToFileURL} from 'node:url';

const expected = [
  ['today', 'Today'], ['pipeline', 'Pipeline'], ['discovery', 'Discovery'],
  ['people', 'People'], ['evidence', 'Evidence'], ['editor', 'Editor'],
  ['agent', 'Agent'], ['settings', 'Settings'],
].map(([slug, label]) => ({slug, label, href: `/${slug}`, file: `routes/${slug}/index.html`}));
const stressPath = '/__stress/navigation';
const csp = "default-src 'none'; style-src 'self' 'unsafe-inline'; img-src data:; form-action 'none'; base-uri 'none'; frame-ancestors 'none'";
const sha = data => createHash('sha256').update(data).digest('hex');
const args = process.argv.slice(2);
if (args.length !== 2) throw new Error('Expected a preview directory and an empty report directory.');
const [root, out, temp] = await Promise.all([realpath(resolve(args[0])), realpath(resolve(args[1])), realpath(tmpdir())]);
for (const path of [root, out]) {
  const rel = relative(temp, path);
  if (!rel || rel === '..' || rel.startsWith('..' + sep) || rel.startsWith(sep)) throw new Error('Directories must be existing children of system temp.');
}
if (root === out || (await readdir(out)).length) throw new Error('Distinct input and empty output required.');

const harnessPath = fileURLToPath(import.meta.url);
const manifestBytes = await readFile(resolve(root, 'manifest.json'));
const manifest = JSON.parse(manifestBytes);
if (manifest.schema_version !== 1 || JSON.stringify(manifest.routes.map(({slug, label, href, file}) => ({slug, label, href, file}))) !== JSON.stringify(expected)) {
  throw new Error('Preview route manifest does not exactly match the eight app-owned destinations.');
}
if (manifest.stress?.href !== stressPath || manifest.stress?.file !== 'stress/index.html' || !manifest.stress.label?.startsWith('Synthetic navigation label') || manifest.stress.label.length < 300) throw new Error('Synthetic long-label stress entry is incomplete.');
if (manifest.assets?.length !== 1 || manifest.assets[0].url !== '/assets/app.css' || manifest.assets[0].file !== 'assets/app.css') throw new Error('Expected exactly the embedded CSS asset.');

const assets = new Map(), inputs = [];
for (const item of [...manifest.routes, manifest.stress, ...manifest.assets]) {
  const allowedFile = expected.some(route => route.file === item.file) || ['stress/index.html', 'assets/app.css'].includes(item.file);
  if (!allowedFile) throw new Error(`Unapproved manifest file ${item.file}`);
  const data = await readFile(resolve(root, item.file));
  if (data.length !== item.bytes || sha(data) !== item.sha256) throw new Error(`Input identity mismatch for ${item.file}`);
  const url = item.url || item.href;
  if (assets.has(url)) throw new Error(`Duplicate route ${url}`);
  assets.set(url, {data, type: item.file.endsWith('.css') ? 'text/css' : 'text/html'});
  inputs.push({url, file: item.file, bytes: data.length, sha256: sha(data)});
}

const requestedEntry = process.env.APPKIT_PLAYWRIGHT_ENTRY || '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
const entry = await realpath(resolve(requestedEntry));
const entryRel = relative(await realpath('/tmp/appkit-browser-review.xBrDCp'), entry);
if (!entryRel || entryRel === '..' || entryRel.startsWith('..' + sep) || entryRel.startsWith(sep)) throw new Error('Playwright entry must be inside the approved isolated harness.');
const packagePath = await realpath(resolve(dirname(entry), 'package.json'));
const packageBytes = await readFile(packagePath), pkg = JSON.parse(packageBytes);
if (pkg.name !== 'playwright' || pkg.version !== '1.63.0') throw new Error(`Expected isolated playwright 1.63.0, got ${pkg.name} ${pkg.version}`);
const {chromium} = await import(pathToFileURL(entry).href);

const report = {
  root, out, started: new Date().toISOString(), harnessSHA256: sha(await readFile(harnessPath)), manifestSHA256: sha(manifestBytes),
  playwright: {entry, entrySHA256: sha(await readFile(entry)), packagePath, packageSHA256: sha(packageBytes), version: pkg.version}, inputs,
  budgets: {textContrast: 4.5, focusContrast: 3, focusWidth: 2, targetSize: 44, navigationMs: 5000, actionMs: 3000},
  checks: [], samples: [], responses: [], blocked: [], errors: [], workers: [], downloads: [], screenshots: [],
};
const check = (name, pass, detail) => report.checks.push({name, pass: Boolean(pass), detail});
let origin = '', server, browser;
try {
  server = http.createServer((req, res) => {
    res.setHeader('Content-Security-Policy', csp); res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Referrer-Policy', 'no-referrer'); res.setHeader('Cache-Control', 'no-store');
    if (req.headers.host !== new URL(origin).host) {res.writeHead(403).end(); return;}
    if (!['GET', 'HEAD'].includes(req.method)) {res.writeHead(405, {Allow: 'GET, HEAD'}).end(); return;}
    if (new URL(req.url, origin).search || !assets.has(req.url)) {res.writeHead(404).end(); return;}
    const asset = assets.get(req.url);
    res.writeHead(200, {'Content-Type': `${asset.type}; charset=utf-8`, 'Content-Length': asset.data.length});
    res.end(req.method === 'HEAD' ? undefined : asset.data);
  });
  await new Promise((ok, fail) => {server.once('error', fail); server.listen(0, '127.0.0.1', ok);});
  origin = `http://127.0.0.1:${server.address().port}`;
  browser = await chromium.launch({executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, timeout: 10000, args: ['--no-proxy-server', '--disable-background-networking']});
  report.browser = browser.version();

  for (const javaScriptEnabled of [false, true]) {
    const context = await browser.newContext({acceptDownloads: false, deviceScaleFactor: 1, javaScriptEnabled});
    context.setDefaultTimeout(report.budgets.actionMs); context.setDefaultNavigationTimeout(report.budgets.navigationMs);
    context.on('serviceworker', worker => report.workers.push(worker.url()));
    await context.route('**/*', async route => {
      const request = route.request(), url = new URL(request.url());
      if (url.origin !== origin || !['GET', 'HEAD'].includes(request.method()) || !assets.has(url.pathname) || url.search) {report.blocked.push({url: request.url(), method: request.method()}); await route.abort();}
      else await route.continue();
    });
    await context.routeWebSocket('**/*', socket => {report.blocked.push({url: socket.url(), method: 'WS'}); socket.close();});
    let primaryPage;
    context.on('page', page => {
      if (primaryPage && page !== primaryPage) report.errors.push(`Unexpected new page: ${page.url()}`);
      page.on('pageerror', error => report.errors.push(error.message));
      page.on('console', msg => {if (['error', 'warning'].includes(msg.type())) report.errors.push(msg.text());});
      page.on('download', download => {report.downloads.push(download.suggestedFilename()); void download.cancel();});
      page.on('response', response => report.responses.push({url: response.url(), status: response.status()}));
    });
    const page = await context.newPage(); primaryPage = page;

    for (const width of [320, 768, 1440]) for (const scheme of ['light', 'dark']) {
      await page.setViewportSize({width, height: 900}); await page.emulateMedia({colorScheme: scheme, reducedMotion: 'reduce'});
      for (let routeIndex = 0; routeIndex < expected.length; routeIndex++) {
        const route = expected[routeIndex], name = `${route.slug}-${width}-${scheme}-js${javaScriptEnabled ? 'on' : 'off'}`;
        const response = await page.goto(origin + route.href, {waitUntil: 'load'});
        const responseHeaders = await response.headers();
        const state = await page.evaluate(({expected, width}) => {
          const visible = el => {
            const closed = el.closest('details:not([open])');
            if (closed && !closed.querySelector(':scope > summary')?.contains(el)) return false;
            return el.checkVisibility({checkOpacity: true, checkVisibilityCSS: true});
          };
          const navs = [...document.querySelectorAll('nav[aria-label="Primary"], nav[aria-label="Primary (mobile)"]')];
          const inspect = nav => ({label: nav.getAttribute('aria-label'), visible: visible(nav), ul: nav.querySelectorAll(':scope > ul').length,
            links: [...nav.querySelectorAll('a')].map(a => ({href: a.getAttribute('href'), label: a.querySelector('span:last-child')?.textContent, current: a.getAttribute('aria-current')}))});
          const geometry = [...document.querySelectorAll('a[href]')].filter(visible).map(a => {const r = a.getBoundingClientRect(); return {href: a.getAttribute('href'), left: r.left, right: r.right, width: r.width, height: r.height};});
          return {scripts: document.scripts.length, forms: document.forms.length, main: document.querySelectorAll('main#content[tabindex="-1"]').length,
            skip: document.querySelectorAll('body > a[href="#content"]').length, currentAll: document.querySelectorAll('a[aria-current="page"]').length,
            navs: navs.map(inspect), pageWidth: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth, geometry,
            bodyColor: getComputedStyle(document.body).color, bodyBackground: getComputedStyle(document.body).backgroundColor, expectedCount: expected.length, desktopExpected: width >= 768};
        }, {expected, width});
        report.samples.push({name, ...state});
        check(`${name}: exact inert response`, response?.status() === 200 && responseHeaders['content-security-policy'] === csp && state.scripts === 0 && state.forms === 0);
        check(`${name}: shell landmarks`, state.main === 1 && state.skip === 1 && state.navs.length === 2 && state.navs.every(n => n.ul === 1));
        check(`${name}: exact routes and current state`, state.currentAll === 2 && state.navs.every(n => JSON.stringify(n.links.map(({href, label}) => ({href, label}))) === JSON.stringify(expected.map(({href, label}) => ({href, label}))) && n.links.filter(l => l.current === 'page').length === 1 && n.links.find(l => l.current === 'page')?.href === route.href));
        check(`${name}: responsive navigation visibility`, width >= 768
          ? state.navs.filter(n => n.visible).length === 1 && state.navs.find(n => n.visible)?.label === 'Primary'
          : state.navs.every(n => !n.visible));
        check(`${name}: outer geometry`, state.scrollWidth <= state.pageWidth + 1 && state.geometry.every(r => r.left >= -1 && r.right <= width + 1 && r.width > 0 && r.height > 0), state.geometry);
        if (width < 768) await page.locator('details.md\\:hidden').evaluate(el => {el.open = true;});
        for (const item of expected) check(`${name}: accessible ${item.slug}`, await page.getByRole('link', {name: item.label, exact: true}).count() === 1);
        if (width < 768) await page.locator('details.md\\:hidden').evaluate(el => {el.open = false;});
        if (route.slug === 'today') {
          const screenshot = `${name}.png`; await page.screenshot({path: resolve(out, screenshot), fullPage: true});
          report.screenshots.push({file: screenshot, sha256: sha(await readFile(resolve(out, screenshot)))});
        }

        await page.keyboard.press('Tab');
        const skip = await page.locator('body > a[href="#content"]').evaluate(el => {const s = getComputedStyle(el), r = el.getBoundingClientRect(); return {active: document.activeElement === el, visible: Boolean(el.getClientRects().length), outline: s.outlineStyle, width: r.width, height: r.height};});
        check(`${name}: native first-tab skip`, skip.active && skip.visible && skip.outline !== 'none' && skip.width > 0 && skip.height > 0, skip);
        await page.keyboard.press('Enter');
        check(`${name}: native skip focuses main`, await page.locator('main#content').evaluate(el => document.activeElement === el));

        // The preceding native skip adds #content. Start a fresh document so
        // fragment focus restored by reload cannot contaminate the nav sequence.
        await page.goto('about:blank'); await page.goto(origin + route.href, {waitUntil: 'load'});
        await page.keyboard.press('Tab'); await page.keyboard.press('Tab');
        if (width < 768) {
          const summary = page.locator('details.md\\:hidden > summary');
          // RAC's app-owned header actions precede the mobile disclosure.
          // Traverse them with real Tab presses; never activate the Add control.
          const beforeMenu = [];
          for (let hops = 0; hops < 8 && !(await summary.evaluate(el => document.activeElement === el)); hops++) {
            beforeMenu.push(await page.evaluate(() => ({tag: document.activeElement.tagName, id: document.activeElement.id, text: document.activeElement.textContent.trim()})));
            await page.keyboard.press('Tab');
          }
          check(`${name}: native mobile summary reached`, await summary.evaluate(el => document.activeElement === el));
          check(`${name}: header traversal stays native`, beforeMenu.length === 2 && beforeMenu[0].tag === 'SUMMARY' && beforeMenu[0].text === '+ Add' && beforeMenu[1].id === 'btnPalette', beforeMenu);
          const menu = await summary.evaluate(el => {const r = el.getBoundingClientRect(), s = getComputedStyle(el); return {width: r.width, height: r.height, outlineStyle: s.outlineStyle, outlineWidth: parseFloat(s.outlineWidth)};});
          check(`${name}: mobile summary target and focus`, menu.width >= 44 && menu.height >= 44 && menu.outlineStyle !== 'none' && menu.outlineWidth >= 2, menu);
          await page.keyboard.press('Enter'); check(`${name}: native mobile opens`, await summary.evaluate(el => el.parentElement.open));
          await page.keyboard.press('Enter'); check(`${name}: native mobile closes`, !(await summary.evaluate(el => el.parentElement.open)));
          await page.keyboard.press('Enter'); await page.keyboard.press('Tab');
        }
        const focused = [];
        for (let i = 0; i < expected.length; i++) {
          focused.push(await page.evaluate(() => {
            const el = document.activeElement, style = getComputedStyle(el), r = el.getBoundingClientRect();
            const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1; const ctx = canvas.getContext('2d', {willReadFrequently: true});
            const rgba = css => {ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = css; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data];};
            const composite = (front, back) => {const a = front[3] / 255, b = back[3] / 255, outA = a + b * (1 - a); if (!outA) return [0, 0, 0, 0]; return [0, 1, 2].map(j => Math.round((front[j] * a + back[j] * b * (1 - a)) / outA)).concat(Math.round(outA * 255));};
            const backdrop = start => {
              let color = [0, 0, 0, 0], unknown = false;
              for (let node = start; node; node = node.parentElement) {
                const s = getComputedStyle(node);
                if (Number(s.opacity) !== 1) unknown = true;
                if (color[3] < 255) {
                  if (s.backgroundImage !== 'none') unknown = true;
                  color = composite(color, rgba(s.backgroundColor));
                }
              }
              return {color, unknown: unknown || color[3] !== 255};
            };
            const textBG = backdrop(el), focusBG = backdrop(el.parentElement);
            const unknown = textBG.unknown || focusBG.unknown;
            const lum = c => c.slice(0, 3).map(v => {v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4;}).reduce((sum, v, j) => sum + v * [.2126, .7152, .0722][j], 0);
            const ratio = (a, b) => (Math.max(lum(a), lum(b)) + .05) / (Math.min(lum(a), lum(b)) + .05);
            return {href: el?.getAttribute?.('href'), outlineStyle: style.outlineStyle, outlineWidth: parseFloat(style.outlineWidth), unknown,
              textContrast: unknown ? null : ratio(composite(rgba(style.color), textBG.color), textBG.color), focusContrast: unknown ? null : ratio(composite(rgba(style.outlineColor), focusBG.color), focusBG.color), rect: {left: r.left, right: r.right, top: r.top, bottom: r.bottom, width: r.width, height: r.height}};
          }));
          if (i + 1 < expected.length) await page.keyboard.press('Tab');
        }
        check(`${name}: keyboard nav order`, focused.every((f, i) => f.href === expected[i].href), focused);
        check(`${name}: nav focus and contrast`, focused.every(f => !f.unknown && f.outlineStyle !== 'none' && f.outlineWidth >= 2 && f.textContrast >= 4.5 && f.focusContrast >= 3), focused);
        check(`${name}: nav target size`, focused.every(f => f.rect.width >= 44 && f.rect.height >= 44), focused.map(f => ({href: f.href, width: f.rect.width, height: f.rect.height})));

        const destination = expected[(routeIndex + 1) % expected.length];
        await page.locator(`nav[aria-label="${width >= 768 ? 'Primary' : 'Primary (mobile)'}"] a[href="${destination.href}"]`).click();
        await page.waitForURL(origin + destination.href);
        check(`${name}: native navigation selects destination`, await page.locator(`nav[aria-label="Primary"] a[href="${destination.href}"][aria-current="page"]`).count() === 1);
        await page.goBack({waitUntil: 'load'}); check(`${name}: browser back restores route`, page.url() === origin + route.href);
      }

      const stressName = `stress-${width}-${scheme}-js${javaScriptEnabled ? 'on' : 'off'}`;
      await page.goto(origin + stressPath, {waitUntil: 'load'});
      if (width < 768) {await page.locator('details.md\\:hidden > summary').focus(); await page.keyboard.press('Enter');}
      const stress = await page.locator(`nav[aria-label="${width >= 768 ? 'Primary' : 'Primary (mobile)'}"] a`).evaluate(el => {const r = el.getBoundingClientRect(), span = el.querySelector('span:last-child'), sr = span.getBoundingClientRect(); return {height: r.height, width: r.width, spanHeight: sr.height, scrollWidth: span.scrollWidth, clientWidth: span.clientWidth, pageWidth: document.documentElement.clientWidth, documentWidth: document.documentElement.scrollWidth};});
      report.samples.push({name: stressName, ...stress});
      check(`${stressName}: long label wraps without overflow`, stress.spanHeight > 40 && stress.scrollWidth <= stress.clientWidth + 1 && stress.documentWidth <= stress.pageWidth + 1, stress);
      const screenshot = `${stressName}.png`; await page.screenshot({path: resolve(out, screenshot), fullPage: true}); report.screenshots.push({file: screenshot, sha256: sha(await readFile(resolve(out, screenshot)))});
    }
    await context.close();
  }
  check('no unexpected traffic or runtime activity', !report.blocked.length && !report.errors.length && !report.workers.length && !report.downloads.length && report.responses.every(r => r.status === 200), {blocked: report.blocked, errors: report.errors, workers: report.workers, downloads: report.downloads});
  check('fixture identities unchanged', sha(await readFile(resolve(root, 'manifest.json'))) === report.manifestSHA256
    && (await Promise.all(inputs.map(async input => sha(await readFile(resolve(root, input.file))) === input.sha256))).every(Boolean));
} catch (error) {report.fatal = error.stack;}
finally {
  if (browser) await browser.close();
  if (server) await new Promise(ok => server.close(ok));
  report.finished = new Date().toISOString();
  await writeFile(resolve(out, 'report.json'), JSON.stringify(report, null, 2) + '\n', {mode: 0o600, flag: 'wx'});
  console.log(JSON.stringify({out, checks: report.checks.length, failed: report.checks.filter(c => !c.pass).map(c => c.name), fatal: report.fatal, screenshots: report.screenshots.length}, null, 2));
  if (report.fatal || report.checks.some(c => !c.pass)) process.exitCode = 1;
}
