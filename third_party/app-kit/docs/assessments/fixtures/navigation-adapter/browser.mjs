// Approved isolated Chrome; local synthetic routes; application JavaScript disabled.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {spawn} from 'node:child_process';
import {pathToFileURL} from 'node:url';

const dir = fs.realpathSync(process.argv[2] || '');
const read = name => fs.readFileSync(path.join(dir, name));
const sha = b => crypto.createHash('sha256').update(b).digest('hex');
const prepared = JSON.parse(read('prepare.json'));
if (prepared.status !== 'passed' || sha(read('preview.test')) !== prepared.binary_sha256 || sha(read('navigation.css')) !== prepared.css_sha256) throw Error('prepared identity mismatch');
const entry = process.env.APPKIT_PLAYWRIGHT_MODULE || '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
const version = JSON.parse(fs.readFileSync(path.join(path.dirname(entry), 'package.json'))).version;
if (version !== '1.63.0') throw Error('unexpected Playwright pin');
const {chromium} = await import(pathToFileURL(entry).href);
const chrome = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const out = fs.mkdtempSync(path.join(dir, 'browser-')); fs.chmodSync(out, 0o700);
const report = {schema: 1, scope: 'P06 source and operational navigation', verification: 'not-verified', artifact_sha256: prepared.artifact_sha256, harness_sha256: sha(fs.readFileSync(new URL(import.meta.url))), javascript: false, versions: {playwright: version}, checks: [], source_failures: [], screenshots: [], requests: [], responses: [], errors: [], limits: ['Synthetic routes only; no consumer, screen reader, touch or OS-picker claim.', 'Long-label mutation tests compiled CSS geometry; generated-runtime tests separately check bound input.', 'Native selects can truncate long options; choose Links when full option text must stay visible.']};
const check = (name, ok, detail = null) => report.checks.push({name, ok: !!ok, detail});
const save = (name, bytes) => fs.writeFileSync(path.join(out, name), bytes, {flag: 'wx', mode: 0o600});
let browser, logs = '', stderr = '';
const server = spawn(path.join(dir, 'preview.test'), ['-test.run=^TestServeNavigationPreview$', '-test.v'], {cwd: path.join(dir, 'build'), env: {...process.env, APPKIT_P06_SERVE: '1', APPKIT_P06_CSS: path.join(dir, 'navigation.css')}, stdio: ['ignore', 'pipe', 'pipe']});
server.stderr.on('data', b => stderr += b);
const readyURL = new Promise((resolve, reject) => {
  const timer = setTimeout(() => reject(Error('fixture startup timeout')), 15000);
  server.stdout.on('data', b => {logs += b; const m = logs.match(/APPKIT_P06_URL=(http:\/\/127\.0\.0\.1:\d+)/); if (m) {clearTimeout(timer); resolve(m[1]);}});
  server.once('error', e => {clearTimeout(timer); reject(e);});
  server.once('exit', code => {clearTimeout(timer); reject(Error(`fixture exited ${code}`));});
});

async function loaded(page) {await page.waitForLoadState('load'); await page.evaluate(() => document.fonts.ready);}
async function capture(page, name) {const b = await page.screenshot(); save(name, b); report.screenshots.push({name, sha256: sha(b)}); return b;}
async function metrics(page) {
  return page.evaluate(() => {
    const visible = e => e.checkVisibility();
    const box = e => {const r = e.getBoundingClientRect(), s = getComputedStyle(e); return {tag: e.tagName, width: r.width, height: r.height, scroll: e.scrollWidth, client: e.clientWidth, outline: s.outlineWidth, outlineStyle: s.outlineStyle, text: e.textContent};};
    return {width: innerWidth, scroll: document.documentElement.scrollWidth, controls: [...document.querySelectorAll('#specimen a, #specimen select, #specimen button')].filter(visible).map(box), fonts: [...document.fonts].some(f => f.family.replaceAll('"', '') === 'InterVariable' && f.status === 'loaded')};
  });
}
async function contrast(page) {
  return page.evaluate(() => {
    const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
    const ctx = canvas.getContext('2d', {willReadFrequently: true});
    const rgba = css => {ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = css; ctx.fillRect(0, 0, 1, 1); return [...ctx.getImageData(0, 0, 1, 1).data];};
    const over = (a, b) => a.slice(0, 3).map((v, i) => v * a[3] / 255 + b[i] * (1 - a[3] / 255));
    const luminance = rgb => rgb.map(v => v / 255).map(v => v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4).reduce((n, v, i) => n + v * [.2126, .7152, .0722][i], 0);
    return [...document.querySelectorAll('#specimen a, #specimen select, #specimen button')].filter(e => e.checkVisibility()).map(e => {
      const ancestors = []; for (let n = e; n; n = n.parentElement) ancestors.unshift(n);
      let bg = [255, 255, 255]; for (const n of ancestors) bg = over(rgba(getComputedStyle(n).backgroundColor), bg);
      const color = getComputedStyle(e).color, fg = over(rgba(color), bg), a = luminance(fg), b = luminance(bg);
      return {tag: e.tagName, ratio: (Math.max(a, b) + .05) / (Math.min(a, b) + .05), foreground: color, background: bg};
    });
  });
}
async function dom(page) {
  return page.locator('#specimen').evaluate(root => {
    const visit = n => ({type: n.nodeType, ns: n.namespaceURI, name: n.nodeName, data: n.nodeValue, attrs: n.attributes ? [...n.attributes].map(a => [a.namespaceURI, a.name, a.value]).sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b))) : [], children: [...n.childNodes].map(visit)});
    return JSON.stringify([...root.childNodes].map(visit));
  });
}

try {
  const base = await readyURL;
  const guard = async context => {
    await context.route('**/*', route => {const r = route.request(), u = new URL(r.url()); const allowed = u.origin === base && r.method() === 'GET'; report.requests.push({url: r.url(), allowed}); if (allowed) return route.continue(); check('no external/non-GET request', false, r.url()); return route.abort();});
    context.setDefaultTimeout(5000); context.setDefaultNavigationTimeout(5000);
  };
  const observe = page => {
    page.on('pageerror', e => report.errors.push(e.message));
    page.on('response', r => report.responses.push({url: r.url(), status: r.status(), location: r.headers().location ?? null}));
  };
  browser = await chromium.launch({executablePath: chrome, headless: true}); report.versions.chrome = browser.version();
  for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
    const context = await browser.newContext({viewport: {width, height: 1000}, javaScriptEnabled: false, colorScheme: theme, reducedMotion: 'reduce', serviceWorkers: 'block'}); await guard(context);
    const page = await context.newPage(); observe(page); const prefix = `${width}-${theme}`;
    try {
      await page.goto(`${base}/${theme}/original`); await loaded(page);
      const sourceDOM = await dom(page), sourceGeometry = await metrics(page), original = await capture(page, `${prefix}-original.png`);
      await page.goto(`${base}/${theme}/reference`); await loaded(page);
      const referenceDOM = await dom(page), referenceGeometry = await metrics(page), reference = await capture(page, `${prefix}-reference.png`);
      check(`${prefix}: source/reference exact DOM`, sourceDOM === referenceDOM);
      check(`${prefix}: source/reference exact pixels`, original.equals(reference));
      check(`${prefix}: source/reference geometry/font`, JSON.stringify(sourceGeometry) === JSON.stringify(referenceGeometry) && sourceGeometry.fonts, sourceGeometry);
      for (const c of sourceGeometry.controls) if (c.width < 44 || c.height < 44) report.source_failures.push({name: `${prefix}: original target below 44px`, control: c});

      for (const mode of ['get', 'links']) {
        const key = `${prefix}-${mode}`;
        await page.goto(`${base}/${theme}/${mode}/page-3`); await loaded(page);
        const g = await metrics(page), narrowGET = width === 390 && mode === 'get';
        check(`${key}: visible controls/targets/font`, g.fonts && g.scroll <= g.width && g.controls.length === (narrowGET ? 2 : 4) && g.controls.every(c => c.height >= 44 && c.width >= 44), g);
        check(`${key}: responsive exclusivity`, await page.locator('#specimen select:visible').count() === (narrowGET ? 1 : 0) && await page.locator('#specimen a:visible').count() === (narrowGET ? 0 : 4));
        const sampledContrast = await contrast(page);
        check(`${key}: sampled normal-text contrast`, sampledContrast.every(c => c.ratio >= 4.5), sampledContrast);
        for (let i = 0; i < (narrowGET ? 2 : 4); i++) {
          await page.keyboard.press('Tab');
          const active = await page.evaluate(() => ({tag: document.activeElement.tagName, href: document.activeElement.getAttribute('href'), visible: document.activeElement.checkVisibility(), outline: parseFloat(getComputedStyle(document.activeElement).outlineWidth), outlineStyle: getComputedStyle(document.activeElement).outlineStyle}));
          check(`${key}: native Tab ${i + 1}`, active.visible && active.outline >= 2 && active.outlineStyle === 'solid' && (narrowGET ? active.tag === (i === 0 ? 'SELECT' : 'BUTTON') : active.href === `/${theme}/${mode}/page-${i + 1}`), active);
        }
        await capture(page, `${key}.png`);
        if (narrowGET) {
          // The prior Tab walk ends on Submit. Native select type-ahead works in
          // this headless macOS Chrome; OS-popup arrow navigation is not proven.
          await page.keyboard.press('Shift+Tab');
          check(`${key}: native select focused`, await page.locator('select').evaluate(e => e === document.activeElement));
          const initial = (await page.locator('option').first().textContent()).trim()[0].toLowerCase();
          await page.keyboard.press(initial);
          const pending = await page.locator('select').inputValue();
          check(`${key}: selection is pending until submit`, pending === 'page-1' && new URL(page.url()).pathname.endsWith('/page-3'), {pending, url: page.url()});
          await page.keyboard.press('Tab');
          await Promise.all([page.waitForURL(`${base}/${theme}/get/page-1`), page.keyboard.press('Enter')]); await loaded(page);
          check(`${key}: native GET reaches selected page`, await page.locator('#specimen').getAttribute('data-current') === 'page-1' && await page.locator('select').inputValue() === 'page-1');
          check(`${key}: exact GET redirect`, report.responses.some(r => r.url === `${base}/${theme}/get/choose?page_key=page-1` && r.status === 303 && r.location === `/${theme}/get/page-1`));
          await page.goBack(); await loaded(page);
          check(`${key}: Back restores previous page`, new URL(page.url()).pathname.endsWith('/page-3') && await page.locator('#specimen').getAttribute('data-current') === 'page-3');
        } else {
          for (let i = 1; i <= 4; i++) {
            const target = `/${theme}/${mode}/page-${i}`, anchor = page.locator(`#specimen a[href="${target}"]:visible`);
            if (i % 2) {await anchor.focus(); await Promise.all([page.waitForURL(base + target), page.keyboard.press('Enter')]);} else {await Promise.all([page.waitForURL(base + target), anchor.click()]);}
            await loaded(page);
            check(`${key}: selected page ${i}`, await page.locator('#specimen a[aria-current="page"]:visible').count() === 1 && await page.locator('#specimen a[aria-current="page"]:visible').getAttribute('href') === target && await page.locator('#specimen').getAttribute('data-current') === `page-${i}`);
          }
          await page.goBack(); await loaded(page); check(`${key}: browser Back`, new URL(page.url()).pathname.endsWith('/page-3'));
          await page.locator('#specimen a').evaluateAll(anchors => {for (const a of anchors) a.textContent = 'Synthetic long label '.repeat(20).slice(0, 256);});
          const long = await metrics(page);
          check(`${key}: 256-character wrapping`, long.scroll <= long.width && long.controls.every(c => c.text.length === 256 && c.scroll <= c.client + 1 && c.height >= 44), long);
          await capture(page, `${key}-long.png`);
        }
      }
    } finally {await context.close();}
  }
  // Real browser zoom, not viewport narrowing or CDP visual-scale emulation.
  const profile = fs.mkdtempSync(path.join(out, 'chrome-profile-')); fs.chmodSync(profile, 0o700);
  const zoomContext = await chromium.launchPersistentContext(profile, {executablePath: chrome, headless: true, viewport: {width: 1440, height: 1000}, colorScheme: 'dark', serviceWorkers: 'block'});
  try {
    const page = zoomContext.pages()[0]; await page.goto('chrome://settings/appearance'); await page.locator('#zoomLevel').selectOption('2');
    check('Chrome setting is 200 percent', await page.locator('#zoomLevel').inputValue() === '2');
    const control = await zoomContext.newCDPSession(page);
    await control.send('Emulation.setScriptExecutionDisabled', {value: true});
    await guard(zoomContext); observe(page); await page.goto(`${base}/dark/links/page-3`); await loaded(page);
    const zoom = await page.evaluate(() => ({dpr: devicePixelRatio, inner: innerWidth, outer: outerWidth, client: document.documentElement.clientWidth, scale: visualViewport.scale, scroll: document.documentElement.scrollWidth}));
    check('Actual 200 percent browser zoom', zoom.dpr === 2 && zoom.inner === 720 && zoom.outer === 1440 && zoom.client === 720 && zoom.scale === 1 && zoom.scroll <= zoom.client, zoom);
    await page.keyboard.press('Tab'); await Promise.all([page.waitForURL(`${base}/dark/links/page-1`), page.keyboard.press('Enter')]); await loaded(page);
    check('Keyboard navigation at actual zoom', await page.locator('#specimen').getAttribute('data-current') === 'page-1');
    const cdp = await zoomContext.newCDPSession(page), shot = await cdp.send('Page.captureScreenshot', {format: 'png', fromSurface: true, captureBeyondViewport: false});
    const bytes = Buffer.from(shot.data, 'base64'); save('200-percent-zoom.png', bytes); report.screenshots.push({name: '200-percent-zoom.png', sha256: sha(bytes), physical_width: bytes.readUInt32BE(16)});
    const rightEdge = await page.locator('#specimen a:visible').last().evaluate(e => ({right: e.getBoundingClientRect().right, width: innerWidth}));
    check('Zoom capture dimensions and last-control containment', bytes.readUInt32BE(16) === 1440 && bytes.readUInt32BE(20) === 1000 && rightEdge.right <= rightEdge.width, rightEdge);
  } finally {await zoomContext.close(); fs.rmSync(profile, {recursive: true}); report.zoom_profile_disposition = 'fresh isolated profile removed after browser close';}
  check('No page script errors', report.errors.length === 0, report.errors);
} catch (e) {check('Harness completed', false, e.message);} finally {
  if (browser) await browser.close();
  server.kill('SIGTERM'); await new Promise(resolve => {if (server.exitCode !== null) return resolve(); server.once('exit', resolve); setTimeout(resolve, 4000);});
  save('server.txt', logs + stderr);
  report.status = report.checks.every(c => c.ok) ? 'passed-with-source-limitations' : 'failed';
  save('report.json', JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({directory: out, status: report.status, checks: report.checks.length, failed: report.checks.filter(c => !c.ok), source_failures: report.source_failures.length, report_sha256: sha(fs.readFileSync(path.join(out, 'report.json')))}));
  if (report.status === 'failed') process.exitCode = 1;
}
