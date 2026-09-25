// Owned loopback Gallery + fresh Chrome profile. Synthetic data, no app scripts.
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {spawn} from 'node:child_process';

const [parent, binary] = process.argv.slice(2);
assert.ok(path.isAbsolute(parent) && path.isAbsolute(binary));
const sha = b => createHash('sha256').update(b).digest('hex');
const write = (p, b) => fs.writeFile(p, b, {flag: 'wx', mode: 0o600});
const root = await fs.mkdtemp(path.join(await fs.realpath(parent), 'header-journey-'));
await fs.chmod(root, 0o700);
const source = fileURLToPath(import.meta.url), sourceBytes = await fs.readFile(source);
await write(path.join(root, 'header-journey.mjs'), sourceBytes);
const pw = process.env.APPKIT_PLAYWRIGHT_MODULE;
assert.ok(pw && path.isAbsolute(pw));
const pwSHA = sha(await fs.readFile(pw));
assert.equal(pwSHA, 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
const {chromium} = await import(pathToFileURL(pw));
const report = {kind: 'gallery-header-journey-v1', verification: 'not-certified', root,
  startedAt: new Date().toISOString(), sourceSHA: sha(sourceBytes),
  binary: {path: binary, sha256: sha(await fs.readFile(binary))}, playwrightEntrySHA: pwSHA,
  checks: [], failures: [], pages: [], captures: [], limits: [
    'Synthetic Gallery composition in local Chrome; not physical touch, assistive-technology, other browsers, or consumer acceptance.',
    'Binary, executed harness, served HTML and CSS are bound here. Build/source closure remains separate operator evidence.',
    'CSS widths are not an actual browser zoom test. No app JavaScript or network assets are allowed.'
  ]};
function check(name, passed, detail) {
  report.checks.push({name, passed, detail});
  if (!passed) report.failures.push({name, detail});
}
async function bounded(promise, ms, label) {
  let timer;
  try { return await Promise.race([promise, new Promise((_, reject) => {
    timer = setTimeout(() => reject(Error(label + ' timed out')), ms);
  })]); } finally { clearTimeout(timer); }
}
async function tabTo(page, selector) {
  for (let i = 0; i < 110; i++) {
    if (await page.evaluate(s => document.activeElement?.matches(s) === true, selector)) return;
    await page.keyboard.press('Tab');
  }
  throw Error('Native Tab could not reach ' + selector);
}
const navPaths = ['/catalog', '/references', '/analytics', '/shell', '/overview', '/tokens',
  '/themes', '/components', '/layout', '/forms', '/overlays', '/agentic', '/messaging',
  '/provenance', '/dashboard', '/motion'];
let gallery, galleryClosed, server, chrome, chromeExit, browser, origin, stdout = '', stderr = '';
const args = ['-addr', '127.0.0.1:0', '-design-assets', ''];
const watchdog = setTimeout(() => {
  report.failures.push({name: 'watchdog', detail: '180 seconds'});
  server?.kill().catch(() => {}); gallery?.kill('SIGKILL');
}, 180000);
try {
  gallery = spawn(binary, args, {cwd: root, stdio: ['ignore', 'pipe', 'pipe']});
  galleryClosed = new Promise(resolve => gallery.once('close', resolve));
  gallery.stdout.on('data', b => { stdout += b.toString(); });
  origin = await bounded(new Promise((resolve, reject) => {
    gallery.once('error', reject);
    gallery.once('exit', () => reject(Error('Gallery exited before readiness')));
    gallery.stderr.on('data', b => {
      stderr += b.toString();
      const m = stderr.match(/app-kit Gallery → (http:\/\/127\.0\.0\.1:\d+)/);
      if (m) resolve(m[1]);
    });
  }), 15000, 'Gallery readiness');
  report.origin = origin;
  server = await chromium.launchServer({headless: true,
    executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    args: ['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1']});
  chrome = server.process(); chromeExit = new Promise(resolve => chrome.once('exit', resolve));
  browser = await chromium.connect(server.wsEndpoint()); report.browserVersion = browser.version();
  assert.equal(report.browserVersion, '152.0.7977.84');
  for (const width of [320, 390, 768, 1440]) for (const theme of ['light', 'dark']) {
    const lane = {width, theme};
    const context = await browser.newContext({viewport: {width, height: 1000}, colorScheme: theme,
      reducedMotion: 'reduce', javaScriptEnabled: false, serviceWorkers: 'block'});
    await context.route('**/*', route => {
      const u = new URL(route.request().url());
      if (u.origin === origin && route.request().method() === 'GET' &&
          (u.pathname === '/static/app.css' || u.pathname === '/catalog' ||
           u.pathname.startsWith('/analytics'))) return route.continue();
      check('unexpected request', false, {lane, url: u.href}); return route.abort();
    });
    const page = await context.newPage(); page.setDefaultTimeout(7000);
    page.on('pageerror', e => check('page error', false, {lane, error: String(e)}));
    page.on('requestfailed', r => check('request failed', false, {lane, url: r.url(), error: r.failure()}));
    let responses = [];
    page.on('response', r => responses.push(r));
    async function retain(stage) {
      const own = responses; responses = [];
      const doc = own.filter(r => r.request().resourceType() === 'document');
      const css = own.filter(r => r.url() === origin + '/static/app.css');
      assert.equal(doc.length, 1, 'one actual document response per navigation');
      assert.equal(css.length, 1, 'one actual stylesheet response per navigation');
      for (const r of own) check('HTTP success', r.status() === 200, {lane, stage, url: r.url(), status: r.status()});
      const body = await doc[0].body(), stylesheet = await css[0].body();
      if (!report.css) {
        report.css = {sha256: sha(stylesheet), bytes: stylesheet.length};
        await write(path.join(root, 'app.css'), stylesheet);
      }
      check('served CSS identity', sha(stylesheet) === report.css.sha256, {lane, stage});
      check('script-free document', await page.locator('script').count() === 0, {lane, stage});
      check('32KiB document budget', body.length <= 32768, {lane, stage, bytes: body.length});
      const filename = `${width}-${theme}-${stage}.html`;
      await write(path.join(root, filename), body);
      report.pages.push({lane, stage, file: filename, sha256: sha(body), bytes: body.length, url: page.url()});
    }
    async function capture(stage) {
      const filename = `${width}-${theme}-${stage}.png`;
      const png = await page.screenshot({fullPage: true, animations: 'disabled'});
      await write(path.join(root, filename), png);
      report.captures.push({lane, stage, file: filename, sha256: sha(png), bytes: png.length});
    }
    async function inspectHeader(stage, current, expanded = false) {
      const m = await page.evaluate(() => {
        const header = document.querySelector('body > header');
        const canvas = document.createElement('canvas'), ctx = canvas.getContext('2d');
        function rgba(value) { ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = value; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data]; }
        function luminance(rgb) { return rgb.slice(0, 3).map(v => { const s = v / 255; return s <= .04045 ? s / 12.92 : ((s + .055) / 1.055) ** 2.4; }).reduce((s, v, i) => s + v * [.2126, .7152, .0722][i], 0); }
        const targets = [...header.querySelectorAll('a, summary')].filter(e => e.checkVisibility()).map(e => {
          const r = e.getBoundingClientRect(), s = getComputedStyle(e);
          let parent = e, background;
          while (parent) { background = rgba(getComputedStyle(parent).backgroundColor); if (background[3] === 255) break; parent = parent.parentElement; }
          const fg = luminance(rgba(s.color)), bg = luminance(background);
          return {text: e.textContent.trim(), href: e.getAttribute('href'), width: r.width, height: r.height,
            contrast: (Math.max(fg, bg) + .05) / (Math.min(fg, bg) + .05)};
        });
        const menus = [...header.querySelectorAll('nav')];
        return {targets, viewport: innerWidth, scrollWidth: document.documentElement.scrollWidth,
          menuVisible: header.querySelector('details')?.checkVisibility() === true,
          open: header.querySelector('details')?.open === true,
          visibleLinks: menus.flatMap(n => [...n.querySelectorAll('a')]).filter(e => e.checkVisibility()).map(e => ({href: e.getAttribute('href'), current: e.getAttribute('aria-current')})),
          theme: document.documentElement.dataset.theme,
          selectedThemes: [...header.querySelectorAll('[aria-label="Theme"] [aria-current="true"]')].map(e => ({label: e.textContent.trim(), weight: Number(getComputedStyle(e).fontWeight)}))};
      });
      check('no outer overflow', m.scrollWidth <= m.viewport, {lane, stage, viewport: m.viewport, scrollWidth: m.scrollWidth});
      check('theme preserved', m.theme === theme, {lane, stage, theme: m.theme});
      check('one visibly selected theme', m.selectedThemes.length === 1 && m.selectedThemes[0].label === theme && m.selectedThemes[0].weight >= 600, {lane, stage, selected: m.selectedThemes});
      check('responsive disclosure visibility', m.menuVisible === (width < 1024), {lane, stage, visible: m.menuVisible});
      if (width < 1024) check('native disclosure state', m.open === expanded, {lane, stage, open: m.open});
      for (const target of m.targets) {
        check('header target 44px', target.width >= 44 && target.height >= 44, {lane, stage, ...target});
        check('header text contrast', target.contrast >= 4.5, {lane, stage, ...target});
      }
      const expected = width >= 1024 || expanded ? navPaths : [];
      check('visible navigation inventory', JSON.stringify(m.visibleLinks.map(l => new URL(l.href, origin).pathname)) === JSON.stringify(expected), {lane, stage, links: m.visibleLinks});
      const selected = m.visibleLinks.filter(l => l.current === 'page');
      check('one visible current page when navigation exposed', expected.length ? selected.length === 1 && new URL(selected[0].href, origin).pathname === current : selected.length === 0, {lane, stage, selected});
      for (const link of m.visibleLinks) check('navigation preserves theme', new URL(link.href, origin).searchParams.get('theme') === theme, {lane, stage, link});
    }
    async function enterLink(selector, predicate) {
      await tabTo(page, selector);
      check('native link focus outline', await page.evaluate(() => { const s = getComputedStyle(document.activeElement); return s.outlineStyle !== 'none' && parseFloat(s.outlineWidth) >= 2; }), {lane, selector});
      const navigation = page.waitForURL(predicate, {waitUntil: 'load'});
      await page.keyboard.press('Enter'); await navigation;
    }
    await page.goto(origin + '/catalog?theme=' + theme, {waitUntil: 'load'}); await retain('catalog');
    await inspectHeader('closed', '/catalog'); await capture('closed');
    // Native skip, then wrap through the real tab order to the header controls.
    await tabTo(page, 'a[href="#gallery-main"]'); await page.keyboard.press('Enter');
    check('native skip focuses main', await page.evaluate(() => document.activeElement?.id === 'gallery-main'), lane);
    await tabTo(page, 'body > header a[href^="/"]');
    for (const label of ['light', 'dark', 'OS']) {
      await page.keyboard.press('Tab');
      check('header tab order follows visual order', await page.evaluate(want => {
        const e = document.activeElement;
        return e?.closest('[aria-label="Theme"]') !== null && e?.textContent.trim() === want;
      }, label), {lane, expected: label});
    }
    if (width < 1024 && await page.locator('#gallery-navigation').count()) {
      await tabTo(page, '#gallery-navigation summary');
      check('summary focus outline', await page.evaluate(() => parseFloat(getComputedStyle(document.activeElement).outlineWidth) >= 2), lane);
      await page.keyboard.press('Enter'); await inspectHeader('opened', '/catalog', true); await capture('opened');
      // Open disclosure must survive a resize without duplicate visible current links.
      await page.setViewportSize({width: 1440, height: 1000});
      check('resized desktop has one current navigation link', await page.locator('body > header nav [aria-current="page"]').evaluateAll(es => es.filter(e => e.checkVisibility()).length) === 1, lane);
      await page.setViewportSize({width, height: 1000});
      check('native disclosure retains open state on return', await page.locator('#gallery-navigation').evaluate(e => e.open), lane);
    }
    await enterLink('body > header nav a[href^="/analytics"]', u => u.pathname === '/analytics');
    await retain('analytics'); await inspectHeader('analytics', '/analytics');
    await tabTo(page, '#analytics-filter details summary'); await page.keyboard.press('Enter');
    await tabTo(page, '#analytics-filter select[name="state"]');
    // Native select type-ahead avoids platform-dependent popup-open keys.
    await page.keyboard.press('m'); await page.keyboard.press('Tab');
    check('native select chooses missing', await page.locator('select[name="state"]').inputValue() === 'missing', lane);
    await enterLink('#analytics-filter button[type="submit"]', u => u.searchParams.get('state') === 'missing');
    await retain('missing');
    async function missing(stage) {
      const section = page.locator('section[aria-labelledby="analytics-comparison"]');
      check('missing has no plot', await section.locator('svg').count() === 0, {lane, stage});
      check('missing has no numeric scale', !(await section.textContent()).includes('shared scale 0 to'), {lane, stage});
      check('missing is explicitly not zero', (await section.textContent()).includes('Missing is not zero'), {lane, stage});
      const rows = await page.locator('#analytics-value-table tbody tr').evaluateAll(es => es.map(e => [...e.querySelectorAll('th,td')].map(c => c.textContent.trim())));
      check('missing table retains unknown current values', rows.length > 0 && rows.every(r => r[1] === 'Missing'), {lane, stage, rows});
      await capture(stage);
    }
    await missing('missing');
    await enterLink('#analytics-chart-gateway-api h3 a', u => u.pathname === '/analytics/gateway-api');
    await retain('detail'); await missing('detail');
    const selectedQuery = new URL(page.url()).search;
    await enterLink('main header > a', u => u.pathname === '/analytics'); await retain('returned');
    check('detail return preserves selection', new URL(page.url()).search === selectedQuery, lane);
    await page.goto(origin + `/analytics?period=2026-09&compare=none&category=worker&state=complete&theme=${theme}`, {waitUntil: 'load'});
    await retain('zero'); await inspectHeader('zero', '/analytics');
    const zero = page.locator('section[aria-labelledby="analytics-comparison"]');
    check('measured zero retains one SVG', await zero.locator('svg').count() === 1, lane);
    check('measured zero retains numeric scale', (await zero.textContent()).includes('shared scale 0 to 1'), lane);
    const zeroRow = await page.locator('#analytics-value-table tbody tr').first().textContent();
    check('worker zero remains present in table', /Report worker\s+0\s+Present/.test(zeroRow), {lane, zeroRow});
    await capture('zero');
    await context.close();
  }
  assert.equal(sha(await fs.readFile(source)), report.sourceSHA, 'harness drift');
  assert.equal(sha(await fs.readFile(binary)), report.binary.sha256, 'binary drift');
} catch (e) { report.failures.push({name: 'execution', detail: String(e.stack || e)}); }
finally {
  if (server) {
    try { await bounded(server.close(), 7000, 'Chrome close'); }
    catch (e) { report.failures.push({name: 'Chrome close', detail: String(e)}); await bounded(server.kill(), 3000, 'Chrome kill').catch(() => {}); }
  }
  if (chrome) {
    await bounded(chromeExit, 3000, 'Chrome exit').catch(e => report.failures.push({name: 'Chrome exit wait', detail: String(e)}));
    report.chrome = {pid: chrome.pid, exitCode: chrome.exitCode, signalCode: chrome.signalCode};
    check('Chrome normal exit', chrome.exitCode === 0 && chrome.signalCode === null, report.chrome);
  }
  if (gallery) {
    gallery.kill('SIGTERM');
    try { await bounded(galleryClosed, 8000, 'Gallery close'); }
    catch (e) { report.failures.push({name: 'Gallery close', detail: String(e)}); gallery.kill('SIGKILL'); await bounded(galleryClosed, 3000, 'Gallery kill').catch(() => {}); }
    report.gallery = {pid: gallery.pid, args, exitCode: gallery.exitCode, signalCode: gallery.signalCode};
    check('Gallery normal exit', gallery.exitCode === 0 && gallery.signalCode === null, report.gallery);
  }
  clearTimeout(watchdog);
  await write(path.join(root, 'gallery.stdout'), stdout); await write(path.join(root, 'gallery.stderr'), stderr);
  report.finishedAt = new Date().toISOString(); report.passed = report.failures.length === 0;
  await write(path.join(root, 'receipt.json'), JSON.stringify(report, null, 2) + '\n');
}
console.log(JSON.stringify({root, passed: report.passed, checks: report.checks.length, failures: report.failures.length,
  execution: report.failures.filter(f => f.name === 'execution'), chrome: report.chrome, gallery: report.gallery}, null, 2));
process.exitCode = report.passed ? 0 : 1;
