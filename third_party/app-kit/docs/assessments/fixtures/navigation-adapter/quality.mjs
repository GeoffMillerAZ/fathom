// Prospective, private P06 Links input/contrast observations. No installs or app JS.
// The separate Go oracle evaluates facts; this runner never calls a report "passed".
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import {spawn} from 'node:child_process';
import {pathToFileURL, fileURLToPath} from 'node:url';

const root = fs.realpathSync(process.argv[2] || '');
const dir = fs.realpathSync(process.argv[3] || '');
const read = (file, max = 2 << 20) => {
  const s = fs.lstatSync(file);
  if (!s.isFile() || s.isSymbolicLink() || s.size > max) throw Error(`Not a bounded regular fixture: ${file}`);
  return fs.readFileSync(file);
};
const sha = body => crypto.createHash('sha256').update(body).digest('hex');
const canonical = value => JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, c => `\\u${c.charCodeAt(0).toString(16).padStart(4, '0')}`);
const preparedBody = read(path.join(dir, 'prepare.json'));
const prepared = JSON.parse(preparedBody);
const binarySHA = sha(read(path.join(dir, 'preview.test'), 128 << 20));
const cssSHA = sha(read(path.join(dir, 'navigation.css')));
const artifactSHA = sha(read(path.join(dir, 'delivery/artifact.json')));
const sourceSHA = sha(read(path.join(dir, 'source.html')));
if (process.platform !== 'darwin' || process.arch !== 'arm64' || prepared.status !== 'passed' ||
    binarySHA !== prepared.binary_sha256 || cssSHA !== prepared.css_sha256 || artifactSHA !== prepared.artifact_sha256 ||
    sourceSHA !== '01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31') throw Error('Prepared identity/platform mismatch');
const oracleFiles = ['check.go', 'schema.cue'].map(file => read(path.join(root, 'internal/navigationproof', file)));
const oracleSHA = sha(Buffer.concat([oracleFiles[0], Buffer.from([0]), oracleFiles[1]]));
const runnerSHA = sha(read(fileURLToPath(import.meta.url)));
const entry = process.env.APPKIT_PLAYWRIGHT_MODULE;
if (!entry || !path.isAbsolute(entry)) throw Error('Set APPKIT_PLAYWRIGHT_MODULE to the approved isolated Playwright index.mjs');
const version = JSON.parse(read(path.join(path.dirname(entry), 'package.json'))).version;
if (version !== '1.63.0') throw Error('Unexpected Playwright pin');
const {chromium} = await import(pathToFileURL(entry).href);
const chrome = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const out = fs.mkdtempSync(path.join(dir, 'quality-')); fs.chmodSync(out, 0o700);
const save = (name, body) => fs.writeFileSync(path.join(out, name), body, {flag: 'wx', mode: 0o600});
const cases = [];
// width is the requested Playwright viewport before browser zoom, not the outer
// window. Record actual outerWidth unchanged; the oracle checks zoom pairs.
for (const width of [390, 1440]) for (const theme of ['light', 'dark']) for (const input of ['keyboard', 'pointer', 'touch']) for (const zoom of [1, 2]) {
  cases.push({id: `${width}-${theme}-${input}-${zoom}`, width, theme, input, zoom});
}
const report = {schema_version: 1, contract_sha256: '', chrome_version: '', playwright_version: version, cases: [], errors: [], forbidden_requests: []};
const screenshots = [];
const profiles = {};
let logs = '', closed = false, expectedShutdown = false, logOverflow = false, currentContext, currentProfile;
const server = spawn(path.join(dir, 'preview.test'), ['-test.run=^TestServeNavigationPreview$', '-test.v'], {
  cwd: path.join(dir, 'build'), env: {...process.env, APPKIT_P06_SERVE: '1', APPKIT_P06_CSS: path.join(dir, 'navigation.css')}, stdio: ['ignore', 'pipe', 'pipe'],
});
const exited = new Promise(resolve => server.once('exit', (code, signal) => {
  closed = true;
  if (!expectedShutdown) report.errors.push(`Unexpected fixture exit: ${code}/${signal}`);
  resolve();
}));
const ready = new Promise((resolve, reject) => {
  const timeout = setTimeout(() => reject(Error('Fixture startup timeout')), 15000);
  const receive = (b, stdout) => {
    logs += b;
    if (logs.length > 65536) {
      logs = logs.slice(0, 65536);
      if (!logOverflow) {logOverflow = true; report.errors.push('Fixture log budget exceeded');}
      clearTimeout(timeout); reject(Error('Fixture log budget exceeded'));
    }
    if (stdout) {const m = logs.match(/APPKIT_P06_URL=(http:\/\/127\.0\.0\.1:\d+)/); if (m) {clearTimeout(timeout); resolve(m[1]);}}
  };
  server.stdout.on('data', b => receive(b, true)); server.stderr.on('data', b => receive(b, false));
  server.once('error', e => {clearTimeout(timeout); reject(e);});
  server.once('exit', () => {clearTimeout(timeout); reject(Error('Fixture exited before readiness'));});
});
const loaded = async page => {await page.waitForLoadState('load'); await page.evaluate(() => document.fonts.ready);};

// Read rendered colors, including alpha compositing. Contrast math lives only in
// the independent Go oracle. No DOM/CSS patch makes the specimen easier to pass.
async function controlObservation(anchor) {
  return anchor.evaluate(e => {
    const s = getComputedStyle(e), r = e.getBoundingClientRect();
    const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
    const context = canvas.getContext('2d', {willReadFrequently: true});
    const rgba = value => {context.clearRect(0, 0, 1, 1); context.fillStyle = value; context.fillRect(0, 0, 1, 1); return [...context.getImageData(0, 0, 1, 1).data];};
    const composite = (a, b) => a.slice(0, 3).map((v, i) => v * a[3] / 255 + b[i] * (1 - a[3] / 255));
    let bg = [255, 255, 255]; const ancestors = [];
    for (let n = e; n; n = n.parentElement) ancestors.unshift(n);
    for (const n of ancestors.slice(0, -1)) bg = composite(rgba(getComputedStyle(n).backgroundColor), bg);
    const outlineBG = [...bg];
    bg = composite(rgba(s.backgroundColor), bg);
    return {href: e.getAttribute('href'), label: e.textContent, visible: e.checkVisibility(), width: r.width, height: r.height,
      client_width: e.clientWidth, scroll_width: e.scrollWidth, fg: composite(rgba(s.color), bg), bg,
      outline_bg: outlineBG, outline_rgb: composite(rgba(s.outlineColor), outlineBG), outline_width: parseFloat(s.outlineWidth), outline_style: s.outlineStyle,
      focus_visible: e.matches(':focus-visible'), focused_href: document.activeElement.getAttribute('href') || ''};
  });
}
async function tabTo(page, href) {
  for (let tabs = 1; tabs <= 8; tabs++) {
    await page.keyboard.press('Tab');
    if (await page.evaluate(() => document.activeElement.getAttribute('href')) === href) return tabs;
  }
  throw Error(`Natural Tab did not reach ${href}`);
}
async function state(page, from, target, input, status, tabs) {
  const current = page.locator('#specimen a[aria-current="page"]:visible');
  const count = await current.count();
  return {from_path: from, target_key: target, input, path: new URL(page.url()).pathname, status,
    specimen_current: await page.locator('#specimen').getAttribute('data-current'),
    current_href: count === 1 ? await current.getAttribute('href') : '', current_count: count, tabs_to_target: tabs};
}

function removeProfile(profile) {
  if (path.dirname(profile) !== out || !path.basename(profile).startsWith('chrome-profile-')) throw Error('Unexpected cleanup target');
  fs.rmSync(profile, {recursive: true}); profiles[path.basename(profile)] = 'removed after browser close';
}

try {
  const base = await ready;
  const contract = {schema_version: 1, task: 'p06-links-input-contrast-v1', source_sha256: sourceSHA,
    artifact_sha256: artifactSHA, prepare_sha256: sha(preparedBody), binary_sha256: binarySHA, css_sha256: cssSHA,
    runner_sha256: runnerSHA, oracle_sha256: oracleSHA, chrome_version: '152.0.7977.84', playwright_version: version,
    platform: 'darwin', arch: 'arm64', origin: base, cases};
  const contractBody = canonical(contract);
  // Freeze before launching Chrome or observing/activating any navigation task.
  save('contract.json', contractBody); report.contract_sha256 = sha(contractBody);
  console.log(`Prospective P06 contract: ${out} ${report.contract_sha256}`);
  for (const input of ['keyboard', 'pointer', 'touch']) for (const zoom of [1, 2]) {
    const profile = fs.mkdtempSync(path.join(out, 'chrome-profile-')); fs.chmodSync(profile, 0o700);
    profiles[path.basename(profile)] = 'retained until successful browser cleanup';
    let context;
    try {
      context = await chromium.launchPersistentContext(profile, {executablePath: chrome, headless: true,
        viewport: {width: 1440, height: 1000}, hasTouch: input === 'touch', serviceWorkers: 'block', reducedMotion: 'reduce'});
      currentContext = context; currentProfile = profile;
      const page = context.pages()[0];
      // Observe browser configuration and teardown too; never hide those errors.
      page.on('pageerror', e => report.errors.push(`${page.url()}: ${e.stack || e.message}`.slice(0, 2048)));
      const actualVersion = context.browser().version();
      if (report.chrome_version && report.chrome_version !== actualVersion) throw Error('Chrome changed between cases');
      report.chrome_version = actualVersion;
      if (actualVersion !== contract.chrome_version) throw Error('Chrome version differs from frozen contract');
      await page.goto('chrome://settings/appearance');
      await page.locator('#zoomLevel').selectOption(String(zoom));
      if (await page.locator('#zoomLevel').inputValue() !== String(zoom)) throw Error('Chrome did not select requested zoom');
      // Finish browser-settings teardown before disabling application scripts.
      // Disabling WebUI's own callbacks can cause unrelated lifecycle exceptions.
      await page.goto('about:blank'); await page.waitForLoadState('load');
      if (page.url() !== 'about:blank') throw Error('Browser settings did not leave for the blank handoff');
      const cdp = await context.newCDPSession(page);
      await cdp.send('Emulation.setScriptExecutionDisabled', {value: true});
      context.setDefaultTimeout(5000); context.setDefaultNavigationTimeout(5000);
      page.on('dialog', async dialog => {report.errors.push(`Unexpected dialog: ${dialog.type()}`); await dialog.dismiss();});
      context.on('page', async unexpected => {if (unexpected !== page) {report.errors.push('Unexpected window'); await unexpected.close();}});
      await context.route('**/*', route => {
        const request = route.request(), u = new URL(request.url());
        if (u.origin === base && request.method() === 'GET' && !u.search && !u.hash &&
          (u.pathname === '/navigation.css' || /^\/(light|dark)\/links\/page-[1-4]$/.test(u.pathname))) return route.continue();
        report.forbidden_requests.push(`${request.method()} ${u.href}`.slice(0, 2048)); return route.abort();
      });
      await context.routeWebSocket('**/*', socket => {report.forbidden_requests.push(`WebSocket ${socket.url()}`.slice(0, 2048)); socket.close();});
      for (const spec of cases.filter(c => c.input === input && c.zoom === zoom)) {
        await page.setViewportSize({width: spec.width, height: 1000});
        if (page.viewportSize()?.width !== spec.width || page.viewportSize()?.height !== 1000) throw Error('Configured viewport differs from requested case');
        await page.emulateMedia({colorScheme: spec.theme, reducedMotion: 'reduce'});
        await page.goto(`${base}/${spec.theme}/links/page-3`); await loaded(page);
        const environment = await page.evaluate(() => ({inner_width: innerWidth, outer_width: outerWidth, dpr: devicePixelRatio,
          viewport_scale: visualViewport.scale, fonts_loaded: [...document.fonts].some(f => f.family.replaceAll('"', '') === 'InterVariable' && f.status === 'loaded'),
          scroll_width: document.documentElement.scrollWidth}));
        const controls = [];
        for (let i = 1; i <= 4; i++) {
          const href = `/${spec.theme}/links/page-${i}`, anchor = page.locator(`#specimen a[href="${href}"]:visible`);
          await page.mouse.move(1, 1); const normal = await controlObservation(anchor);
          await anchor.hover(); const hover = await controlObservation(anchor);
          await page.mouse.move(1, 1);
          await tabTo(page, href); const focus = await controlObservation(anchor);
          controls.push({href: normal.href, label: normal.label, visible: normal.visible, width: normal.width, height: normal.height,
            client_width: normal.client_width, scroll_width: normal.scroll_width, normal_fg: normal.fg, normal_bg: normal.bg,
            hover_fg: hover.fg, hover_bg: hover.bg, focus_fg: focus.fg, focus_bg: focus.bg,
            outline_bg: focus.outline_bg, outline_rgb: focus.outline_rgb, outline_width: focus.outline_width, outline_style: focus.outline_style,
            focus_visible: focus.focus_visible, focused_href: focus.focused_href});
        }
        const steps = [];
        for (let i = 1; i <= 4; i++) {
          const target = `/${spec.theme}/links/page-${i}`, from = new URL(page.url()).pathname;
          const anchor = page.locator(`#specimen a[href="${target}"]:visible`);
          let tabs = 0;
          if (input === 'keyboard') tabs = await tabTo(page, target);
          const response = page.waitForResponse(r => r.url() === base + target && r.request().isNavigationRequest());
          const navigation = page.waitForURL(base + target);
          const action = input === 'keyboard' ? page.keyboard.press('Enter') : input === 'touch' ? anchor.tap() : anchor.click();
          const [received] = await Promise.all([response, navigation, action]); await loaded(page);
          steps.push(await state(page, from, `page-${i}`, input, received.status(), tabs));
        }
        const from = new URL(page.url()).pathname;
        const backResponse = await page.goBack(); await loaded(page);
        const back = await state(page, from, 'page-3', 'history', backResponse?.status() || 0, 0);
        report.cases.push({spec, ...environment, controls, steps, back});
        // Viewport-only capture while the current page and final control are visible.
        await tabTo(page, `/${spec.theme}/links/page-4`);
        const capture = await cdp.send('Page.captureScreenshot', {format: 'png', fromSurface: true, captureBeyondViewport: false});
        const image = Buffer.from(capture.data, 'base64'), name = `${spec.id}.png`;
        save(name, image); screenshots.push({name, sha256: sha(image)});
        console.log(`Observed ${spec.id}`);
      }
    } finally {
      // A failed launch without a controllable context is retained and reported,
      // never deleted while process ownership/termination is uncertain.
      if (context) {await context.close(); currentContext = undefined; removeProfile(profile); currentProfile = undefined;}
    }
  }
  for (const [name, expected, max] of [
    ['prepare.json', sha(preparedBody), 2 << 20], ['preview.test', binarySHA, 128 << 20],
    ['navigation.css', cssSHA, 2 << 20], ['delivery/artifact.json', artifactSHA, 2 << 20], ['source.html', sourceSHA, 2 << 20],
  ]) if (sha(read(path.join(dir, name), max)) !== expected) throw Error(`Frozen input changed: ${name}`);
  if (sha(read(path.join(out, 'contract.json'))) !== report.contract_sha256 ||
      sha(read(fileURLToPath(import.meta.url))) !== runnerSHA ||
      sha(Buffer.concat(['check.go', 'schema.cue'].map(file => read(path.join(root, 'internal/navigationproof', file))).flatMap((b, i) => i ? [Buffer.from([0]), b] : [b]))) !== oracleSHA) {
    throw Error('Contract, runner or oracle changed during the run');
  }
} catch (e) {
  report.errors.push(String(e.message).slice(0, 2048)); process.exitCode = 1;
} finally {
  if (currentContext) {
    try {await currentContext.close(); currentContext = undefined; if (currentProfile) removeProfile(currentProfile);}
    catch (e) {report.errors.push(`Browser cleanup: ${String(e.message).slice(0, 1000)}`);}
  }
  expectedShutdown = true;
  if (!closed) {
    server.kill('SIGTERM');
    let timeout;
    await Promise.race([exited, new Promise(resolve => {timeout = setTimeout(resolve, 4000);})]); clearTimeout(timeout);
    if (!closed) {report.errors.push('Fixture needed forced termination'); server.kill('SIGKILL'); await exited;}
  }
  save('report.json', canonical(report)); save('screenshots.json', canonical(screenshots)); save('profiles.json', canonical(profiles)); save('server.txt', logs);
  console.log(JSON.stringify({directory: out, state: 'recorded-not-certified', cases: report.cases.length, errors: report.errors,
    contract_sha256: report.contract_sha256, report_sha256: sha(read(path.join(out, 'report.json')))}));
  if (report.errors.length || report.forbidden_requests.length || report.cases.length !== 24) process.exitCode = 1;
}
