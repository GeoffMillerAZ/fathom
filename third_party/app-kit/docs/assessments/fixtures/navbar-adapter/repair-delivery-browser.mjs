// Private P09 delivery browser rehearsal. It consumes cold standalone outputs only.
import assert from 'node:assert/strict';
import { createHash, randomBytes } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import * as fs from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const [kitArg, standaloneArg, standaloneSHA, deliverySHA, repairProbeArg, compileSHA] = process.argv.slice(2);
assert.equal(process.argv.length, 8, 'usage: node repair-delivery-browser.mjs KIT STANDALONE_ROOT STANDALONE_RECEIPT_SHA DELIVERY_RECEIPT_SHA REPAIR_PROBE COMPILE_RECEIPT_SHA');
for (const value of [standaloneSHA, deliverySHA, compileSHA]) assert.match(value ?? '', /^[0-9a-f]{64}$/, 'exact receipt hash required');
for (const value of [kitArg, standaloneArg, repairProbeArg]) assert(path.isAbsolute(value), 'absolute input required');
const [kit, standalone, repairProbe] = await Promise.all([kitArg, standaloneArg, repairProbeArg].map(value => fs.realpath(value)));
const tempRoot = await fs.realpath(os.tmpdir());
const out = await fs.mkdtemp(path.join(tempRoot, 'navbar-repair-delivery-browser-'));
await fs.chmod(out, 0o700);
console.log(`Private P09 repair delivery browser output: ${out}`);

const sha = body => createHash('sha256').update(body).digest('hex');
const inputs = new Map(), outputs = new Map();
async function input(filename, expected, limit = 96 << 20) {
  const resolved = filename instanceof URL ? fileURLToPath(filename) : await fs.realpath(filename);
  const stat = await fs.lstat(resolved);
  assert(stat.isFile() && stat.nlink === 1 && stat.size > 0 && stat.size <= limit, `invalid bounded input: ${path.basename(resolved)}`);
  const body = await fs.readFile(resolved), digest = sha(body);
  if (expected) assert.equal(digest, expected, `input identity differs: ${path.basename(resolved)}`);
  inputs.set(resolved, digest);
  return body;
}
async function write(name, body) {
  const filename = path.join(out, name);
  await fs.writeFile(filename, body, { flag: 'wx', mode: 0o600 });
  outputs.set(name, { sha256: sha(body), bytes: body.length });
  return filename;
}
function bounded(promise, milliseconds = 12_000) {
  let timer;
  return Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(Error('operation deadline')), milliseconds); })]).finally(() => clearTimeout(timer));
}

const report = {
  schema_version: 1, kind: 'private-p09-navbar-repair-delivery-browser', verification: 'not-certified',
  standalone_receipt_sha256: standaloneSHA, delivery_receipt_sha256: deliverySHA, compile_receipt_sha256: compileSHA,
  envelope_sha256: '67f015c1efa6c41b2acb38d56ac00ecc24b4c7d406729655d8330d425a5a6137',
  checks: [], captures: [], requests: [], expected_failures: [], errors: [], hosts: [], cleanup: 'pending',
  matrix: { widths: [390, 1440], schemes: ['light', 'dark'], fragments: ['repair-typed', 'repair-minimal'],
    tasks: ['enhanced keyboard and pointer GET destinations', 'persistent native directory', 'focus paint and containment', 'loaded Inter'] },
  limits: ['Cold-built synthetic typed/minimal fixtures only.', 'Installed headless Chrome at normal 100% CSS scale; no actual200%, touch, screen reader, cross-browser or performance proof.',
    'Delivered CSS/runtime and actual request bytes are exercised, but this is not real consumer adoption or target/accessibility certification.',
    'Native-directory healthy, JavaScript-disabled, HTTP503 and late-arrival tasks are bounded representatives, not every partial runtime failure.'],
};
const check = (name, ok, details = {}) => report.checks.push({ name, ok: Boolean(ok), ...details });
let server, base, context, browserPID, profile;
const pendingResponses = new Set();
async function drainResponses() {
  while (pendingResponses.size) await Promise.all([...pendingResponses]);
}
function captureGeometry(image, viewport, regions, targets) {
  const dimensions = image.width === viewport.width && image.height === viewport.height;
  const contained = targets.length > 0 && targets.every(rect => rect.left >= 0 && rect.top >= 0 && rect.right <= image.width && rect.bottom <= image.height && rect.right > rect.left && rect.bottom > rect.top);
  if (!dimensions || !contained || regions.length === 0) return { dimensions, contained, region: null, passes: false };
  const region = {
    left: Math.floor(Math.min(...regions.map(rect => rect.left))), top: Math.floor(Math.min(...regions.map(rect => rect.top))),
    right: Math.ceil(Math.max(...regions.map(rect => rect.right))), bottom: Math.ceil(Math.max(...regions.map(rect => rect.bottom))),
  };
  region.width = region.right - region.left; region.height = region.bottom - region.top;
  const regionContained = region.left >= 0 && region.top >= 0 && region.right <= image.width && region.bottom <= image.height && region.width > 0 && region.height > 0;
  return { dimensions, contained, region, passes: regionContained };
}
const watchdog = setTimeout(() => { report.errors.push('six-minute deadline'); process.exitCode = 1; try { if (browserPID) process.kill(browserPID, 'SIGKILL'); } catch {} server?.closeAllConnections(); }, 360_000);

function alive() {
  if (!browserPID || !profile) return false;
  const result = spawnSync('/bin/ps', ['-p', String(browserPID), '-o', 'command='], { timeout: 2000, maxBuffer: 16 << 10 });
  if (result.status === 1) return false;
  assert.equal(result.status, 0);
  assert(result.stdout.toString().includes(`--user-data-dir=${profile}`), 'owned Chrome identity differs');
  return true;
}

try {
  const harnessBody = await input(new URL(import.meta.url)); report.harness_sha256 = sha(harnessBody); await write('harness.mjs', harnessBody);
  const geometryCanary = { width: 390, height: 900 }, viewportCanary = { width: 390, height: 900 }, specimenCanary = { left: 0, top: 0, right: 390, bottom: 116 };
  assert(!captureGeometry({ width: 390, height: 899 }, viewportCanary, [specimenCanary], [specimenCanary]).passes, 'truncated image dimensions accepted');
  assert(!captureGeometry(geometryCanary, viewportCanary, [specimenCanary], [specimenCanary, { left: 200, top: 850, right: 390, bottom: 901 }]).passes, 'menu outside image accepted');
  report.expected_failures.push({ kind: 'truncated-capture-dimension', detected: true }, { kind: 'menu-outside-image', detected: true });
  const drainCanary = Promise.resolve(); pendingResponses.add(drainCanary); drainCanary.finally(() => pendingResponses.delete(drainCanary)); await bounded(drainResponses(), 1000);
  check('cleanup response-drain scope canary', pendingResponses.size === 0);
  const standaloneReceipt = JSON.parse(await input(path.join(standalone, 'receipt.json'), standaloneSHA));
  assert.equal(standaloneReceipt.kind, 'private-p09-navbar-repair-standalone-consumer-rehearsal');
  assert.equal(standaloneReceipt.verification, 'not-certified');
  assert.equal(standaloneReceipt.delivery_receipt_sha256, deliverySHA);
  assert.equal(standaloneReceipt.compile_receipt_sha256, compileSHA);
  assert.equal(standaloneReceipt.runs.length, 2);
  const deliveryReceiptPath = path.join(path.dirname(standaloneReceipt.tool_before_after.find(item => item.path.endsWith('/appkit')).path), 'delivery-receipt.json');
  const deliveryReceipt = JSON.parse(await input(deliveryReceiptPath, deliverySHA));
  assert.equal(deliveryReceipt.artifact_sha256, report.envelope_sha256);
  assert.equal(deliveryReceipt.runtime_sha256, '7c7184f9ea5ba3e4842f8624f500cb80d81c3c8621570715cfbd69ecdf1d7179');
  const compileReceipt = JSON.parse(await input(path.join(repairProbe, 'compile-receipt.json'), compileSHA));
  assert.equal(compileReceipt.artifact_sha256, deliveryReceipt.paired_repair_sha256);

  const runRoots = ['first', 'replay'].map(name => path.join(standalone, name, 'consumer'));
  const fragments = {};
  for (const name of ['repair-typed.html', 'repair-minimal.html']) {
    const bodies = await Promise.all(runRoots.map(root => input(path.join(root, name))));
    for (let index = 0; index < bodies.length; index++) {
      const recorded = standaloneReceipt.runs[index].outputs.find(item => item.path === name);
      assert(recorded && recorded.sha256 === sha(bodies[index]) && recorded.bytes === bodies[index].length, `standalone receipt does not bind ${index}:${name}`);
    }
    assert.equal(sha(bodies[0]), sha(bodies[1]), `cold render differs: ${name}`);
    assert.equal(sha(bodies[0]), compileReceipt.rendered_sha256[name], `compiler oracle differs: ${name}`);
    fragments[name] = bodies;
  }
  const deliveredNames = ['USAGE.md', 'artifact.json', 'defaults.json', 'elements-navbar.js', 'navbar.go', 'navbar.templ', 'navbar_repair.css', 'navbar_routes.go', 'navbar_routes.templ'];
  for (const name of deliveredNames) {
    const bodies = await Promise.all(runRoots.map(root => input(path.join(root, 'converted', name), deliveryReceipt.files[name])));
    assert.equal(sha(bodies[0]), sha(bodies[1]), `cold delivered copy differs: ${name}`);
  }
  const deliveredRoot = path.join(runRoots[0], 'converted');
  const envelope = JSON.parse(await input(path.join(deliveredRoot, 'artifact.json'), report.envelope_sha256));
  assert.equal(envelope.adapter, 'p09-navbar-repair-delivery-v1');
  assert.equal(envelope.verification, 'not-verified');
  const runtime = await input(path.join(deliveredRoot, 'elements-navbar.js'), deliveryReceipt.runtime_sha256);
  const repairCSS = await input(path.join(deliveredRoot, 'navbar_repair.css'), deliveryReceipt.files['navbar_repair.css']);
  const font = await input(path.join(kit, 'data/design-assets/reference-assets/elements-1.0.22-pilot/inter.woff2'), '693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3');
  const tailwind = '/Users/geoff/Library/Caches/app-kit/tailwindcss/v4.3.0/tailwindcss-macos-arm64';
  const tailwindBody = await input(tailwind, '56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e');
  report.tailwind = { version: '4.3.0', sha256: sha(tailwindBody) };

  const cssBuilds = [];
  const sourceDirectives = ['@source "./navbar.templ";', '@source "./navbar_routes.templ";'];
  function cssInput(scans = sourceDirectives) {
    assert.deepEqual(scans, sourceDirectives, 'both delivered templates must be explicit scan inputs');
    return Buffer.from(`@import "tailwindcss" source(none);\n${scans.join('\n')}\n@theme{--font-sans:InterVariable,ui-sans-serif,sans-serif}\n@font-face{font-family:InterVariable;font-weight:100 900;font-display:block;src:url("/assets/inter.woff2") format("woff2")}\n`);
  }
  assert.throws(() => cssInput(sourceDirectives.slice(0, 1)), /both delivered templates/, 'omitted route-template scan was not detected');
  report.expected_failures.push({ kind: 'omitted-route-template-scan', detected: true, note: 'contract canary; route template currently has no utility classes' });
  for (const name of ['css-a', 'css-b']) {
    const root = path.join(out, name);
    await fs.mkdir(root, { mode: 0o700 });
    for (const file of ['navbar.templ', 'navbar_routes.templ']) await write(`${name}/${file}`, await fs.readFile(path.join(deliveredRoot, file)));
    const inputPath = path.join(root, 'input.css'), outputPath = path.join(root, 'source.css');
    const buildInput = cssInput(); await write(`${name}/input.css`, buildInput);
    const built = spawnSync(tailwind, ['-i', inputPath, '-o', outputPath, '--minify'], { timeout: 30_000, maxBuffer: 64 << 10 });
    assert.equal(built.status, 0, `pinned Tailwind build failed: ${name}`);
    await fs.chmod(outputPath, 0o600);
    const body = await fs.readFile(outputPath);
    cssBuilds.push(body); outputs.set(`${name}/source.css`, { sha256: sha(body), bytes: body.length });
  }
  assert.equal(sha(cssBuilds[0]), sha(cssBuilds[1]), 'independent CSS builds differ');
  report.css_sha256 = sha(cssBuilds[0]);

  const frozen = (await input(new URL('./repair.mjs', import.meta.url), '6db51ff8d5f291cfa78f15731a4cc9ce6c47be0eaa2f7d25ace994ed43b900f2')).toString();
  function extract(start, end) {
    const left = frozen.indexOf(start), right = frozen.indexOf(end, left);
    assert(left >= 0 && right > left, `frozen helper unavailable: ${start}`);
    return frozen.slice(left, right).trim();
  }
  const focusFacts = (0, eval)(`(${extract('async function focusFacts(page)', 'async function geometry(page)').replace('async function focusFacts', 'async function')})`);
  const geometry = (0, eval)(`(${extract('async function geometry(page)', '\ntry {').replace('async function geometry', 'async function')})`);

  const prefix = `/delivery-${randomBytes(12).toString('hex')}`;
  const logo = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><rect x="2" y="2" width="28" height="28" rx="5" fill="oklch(0.58 0.20 280)"/><path d="M9 22V10h14v4H13v8z" fill="oklch(0.98 0.01 280)"/></svg>');
  const fixtureCSS = Buffer.from('html{font-family:InterVariable,system-ui,sans-serif;color-scheme:light}html.dark{color-scheme:dark}body{margin:0;background:Canvas;color:CanvasText}#outside{margin:24px}');
  const resources = new Map([
    [`${prefix}/source.css`, { type: 'text/css', body: cssBuilds[0] }],
    [`${prefix}/navbar_repair.css`, { type: 'text/css', body: repairCSS }],
    [`${prefix}/elements-navbar.js`, { type: 'text/javascript', body: runtime }],
    [`${prefix}/elements-unavailable.js`, { type: 'text/javascript', body: Buffer.from('// explicit runtime outage\n'), status: 503 }],
    [`${prefix}/fixture.css`, { type: 'text/css', body: fixtureCSS }],
    ['/assets/inter.woff2', { type: 'font/woff2', body: font }],
    ['/assets/brand-light.svg', { type: 'image/svg+xml', body: logo }],
    ['/assets/brand-dark.svg', { type: 'image/svg+xml', body: logo }],
    ['/assets/avatar.jpg', { type: 'image/svg+xml', body: logo }],
  ]);
  const destinations = new Map([['/dashboard', 'Synthetic dashboard'], ['/notifications', 'Synthetic notifications'], ['/profile', 'Synthetic profile'], ['/settings', 'Synthetic settings']]);
  for (const [url, title] of destinations) resources.set(url, { type: 'text/html; charset=utf-8', body: Buffer.from(`<!doctype html><html lang="en"><title>Destination</title><h1>${title}</h1><p>No account mutation occurred.</p></html>`) });
  const documents = new Map();
  function documentBody(fragment, scheme, mode = 'ready') {
    const script = ['disabled', 'ready', 'missing-css'].includes(mode) ? `${prefix}/elements-navbar.js` : `${prefix}/elements-unavailable.js`;
    const repairLink = mode === 'missing-css' ? '' : `<link rel="stylesheet" href="${prefix}/navbar_repair.css">`;
    return Buffer.from(`<!doctype html><html lang="en" class="${scheme === 'dark' ? 'dark' : ''}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Delivered navbar</title><link rel="stylesheet" href="${prefix}/source.css"><link rel="stylesheet" href="${prefix}/fixture.css">${repairLink}<script type="module" src="${script}"></script></head><body><main id="specimen">${fragment}</main><button id="outside" type="button">Outside navigation</button></body></html>`);
  }
  for (const scheme of ['light', 'dark']) for (const width of [390, 1440]) for (const kind of ['repair-typed', 'repair-minimal']) {
    const run = width === 390 ? 0 : 1;
    documents.set(`/${kind}-${scheme}-${width}`, documentBody(fragments[`${kind}.html`][run], scheme));
  }
  for (const mode of ['disabled', 'unavailable', 'late', 'missing-css']) documents.set(`/repair-typed-light-390-${mode}`, documentBody(fragments['repair-typed.html'][0], 'light', mode));

  server = http.createServer((request, response) => {
    const url = new URL(request.url, base || 'http://invalid.test');
    if (!base || request.method !== 'GET' || request.headers.host !== new URL(base).host || url.origin !== base || url.search) { response.writeHead(403).end(); return; }
    const resource = documents.has(url.pathname) ? { type: 'text/html; charset=utf-8', body: documents.get(url.pathname) } : resources.get(url.pathname);
    if (!resource) { response.writeHead(404).end(); return; }
    const status = resource.status ?? 200;
    report.requests.push({ path: url.pathname, status, sha256: sha(resource.body), bytes: resource.body.length });
    response.writeHead(status, { 'Content-Type': resource.type, 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff',
      'Content-Security-Policy': "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self'; connect-src 'none'; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'" });
    response.end(resource.body);
  });
  await bounded(new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); }));
  base = `http://127.0.0.1:${server.address().port}`;
  const playwright = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
  await input(playwright, 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f');
  const { chromium } = await import(pathToFileURL(playwright));
  const { PNG } = createRequire(import.meta.url)('/tmp/appkit-browser-review.xBrDCp/node_modules/playwright-core/lib/utilsBundle.js');
  report.browser_responses = [];

  async function launch(width, scheme, javaScriptEnabled = true) {
    profile = await fs.mkdtemp(path.join(out, 'profile-')); await fs.chmod(profile, 0o700);
    context = await chromium.launchPersistentContext(profile, { executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true,
      viewport: { width, height: 900 }, colorScheme: scheme, reducedMotion: 'reduce', javaScriptEnabled, serviceWorkers: 'block',
      args: ['--enable-automation', '--disable-background-networking', '--no-proxy-server', '--no-first-run', '--no-default-browser-check', `--window-size=${width},900`] });
    report.chrome_version ??= context.browser().version();
    const session = await context.browser().newBrowserCDPSession();
    try {
      const browsers = (await session.send('SystemInfo.getProcessInfo')).processInfo.filter(process => process.type === 'browser');
      assert.equal(browsers.length, 1); browserPID = browsers[0].id; assert(alive()); report.hosts.push({ pid: browserPID, profile, absent: false });
    } finally { await session.detach(); }
    await context.route('**/*', route => {
      const request = route.request(), url = new URL(request.url());
      if (url.origin !== base || request.method() !== 'GET' || url.search || (!documents.has(url.pathname) && !resources.has(url.pathname))) return route.abort();
      return route.continue();
    });
    const page = context.pages()[0]; page.setDefaultTimeout(6000); page.setDefaultNavigationTimeout(12_000);
    page.on('pageerror', error => report.errors.push(`pageerror: ${error.message}`));
    page.on('requestfailed', request => { if (new URL(request.url()).pathname !== `${prefix}/elements-unavailable.js`) report.errors.push(`requestfailed: ${request.failure()?.errorText}`); });
    page.on('response', response => {
      const pathname = new URL(response.url()).pathname;
      if (response.status() >= 400 && !(pathname === `${prefix}/elements-unavailable.js` && response.status() === 503)) report.errors.push(`HTTP ${response.status()}: ${pathname}`);
      if (![`${prefix}/source.css`, `${prefix}/fixture.css`, `${prefix}/navbar_repair.css`, `${prefix}/elements-navbar.js`].includes(pathname)) return;
      const capture = response.body().then(body => report.browser_responses.push({ path: pathname, status: response.status(), sha256: sha(body), bytes: body.length })).catch(error => report.errors.push(`response body: ${pathname}: ${error}`));
      pendingResponses.add(capture); capture.finally(() => pendingResponses.delete(capture));
    });
    return page;
  }
  async function closeContext() {
    await bounded(drainResponses());
    await bounded(context.close()); context = undefined;
    check('owned Chrome exits', !alive()); report.hosts.at(-1).absent = true; browserPID = undefined; profile = undefined;
  }
  async function ready(page, runtimeReady = true) {
    await page.waitForFunction(() => document.fonts.status === 'loaded');
    if (!runtimeReady) return;
    await page.waitForFunction(() => ['el-disclosure', 'el-dropdown', 'el-menu'].every(tag => customElements.get(tag)));
    let stable = 0;
    for (let index = 0; index < 100; index++) {
      const settled = await page.evaluate(() => document.getAnimations().every(animation => animation.playState !== 'running' && !animation.pending) && [...document.querySelectorAll('el-menu:popover-open')].every(menu => getComputedStyle(menu).opacity === '1'));
      stable = settled ? stable + 1 : 0; if (stable === 3) return; await page.waitForTimeout(50);
    }
    throw Error('runtime did not settle');
  }
  async function baseline(page) {
    await page.evaluate(() => { window.__p09Unfocused = new Map([...document.querySelectorAll('nav a,nav button,details[data-appkit-navbar-routes] > summary')].map(element => { const style = getComputedStyle(element); return [element, { outline: style.outline, boxShadow: style.boxShadow, background: style.backgroundColor, focusVisible: element.matches(':focus-visible') }]; })); });
  }
  async function tabTo(page, selector, limit = 30) {
    for (let index = 0; index < limit; index++) { await page.keyboard.press('Tab'); if (await page.locator(selector).evaluate(element => element === document.activeElement)) return true; }
    return false;
  }
  async function capture(page, name) {
    const facts = await page.evaluate(() => {
      const rect = element => { const value = element.getBoundingClientRect(); return { tag: element.tagName, href: element.getAttribute('href'), left: value.left, right: value.right, top: value.top, bottom: value.bottom }; };
      const specimen = document.querySelector('#specimen'), popovers = [...document.querySelectorAll(':popover-open')].filter(element => element.checkVisibility());
      const selected = document.querySelector('[data-appkit-navbar-repair]');
      const targets = [specimen, selected?.querySelector(':scope > nav'), ...selected?.querySelectorAll('a,button,summary,el-disclosure') ?? [], ...popovers, ...popovers.flatMap(popover => [...popover.querySelectorAll('a,button')])]
        .filter(element => element?.checkVisibility() && element.getBoundingClientRect().width > 0 && element.getBoundingClientRect().height > 0).map(rect);
      return { viewport: { width: innerWidth, height: innerHeight }, regions: [rect(specimen), ...popovers.map(rect)], targets,
        menuOpen: popovers.some(element => element.tagName === 'EL-MENU'), settingsVisible: popovers.some(element => [...element.querySelectorAll('a')].some(link => link.getAttribute('href') === '/settings' && link.checkVisibility())) };
    });
    const body = await page.screenshot({ fullPage: false });
    const png = PNG.sync.read(body), pixels = png.width * png.height;
    let dark = 0, light = 0, opaque = 0;
    for (let offset = 0; offset < png.data.length; offset += 4) { const [r, g, b, a] = png.data.subarray(offset, offset + 4); if (a === 255) opaque++; if (Math.max(r, g, b) < 100) dark++; if (Math.min(r, g, b) > 220) light++; }
    const contract = captureGeometry({ width: png.width, height: png.height }, facts.viewport, facts.regions, facts.targets);
    check(`${name}: exact viewport capture and target containment`, contract.passes && (!facts.menuOpen || facts.settingsVisible), { image: { width: png.width, height: png.height }, ...facts, contract });
    let regionDark = 0, regionLight = 0, regionOpaque = 0;
    if (contract.region) for (let y = contract.region.top; y < contract.region.bottom; y++) for (let x = contract.region.left; x < contract.region.right; x++) {
      const offset = (y * png.width + x) * 4, r = png.data[offset], g = png.data[offset + 1], b = png.data[offset + 2], a = png.data[offset + 3];
      if (a === 255) regionOpaque++; if (Math.max(r, g, b) < 100) regionDark++; if (Math.min(r, g, b) > 220) regionLight++;
    }
    const regionPixels = contract.region ? contract.region.width * contract.region.height : 0;
    check(`${name}: task-region painted capture`, contract.passes && regionOpaque === regionPixels && regionDark / regionPixels > 0.001 && regionLight / regionPixels > 0.001,
      { image: { width: png.width, height: png.height, pixels, opaque, dark, light }, region: { ...contract.region, pixels: regionPixels, opaque: regionOpaque, dark: regionDark, light: regionLight } });
    await write(`${name}.png`, body); report.captures.push({ file: `${name}.png`, sha256: sha(body), bytes: body.length });
  }
  async function visit(page, pathname, runtimeReady = true) {
    await page.goto(base + pathname, { waitUntil: 'networkidle' }); await ready(page, runtimeReady); await baseline(page);
    const metrics = await page.evaluate(() => ({ innerWidth, dpr: devicePixelRatio, scale: visualViewport.scale, zoom: getComputedStyle(document.documentElement).zoom, font: getComputedStyle(document.querySelector('nav')).fontFamily }));
    check(`${pathname}: normal viewport and loaded Inter`, metrics.innerWidth === Number(pathname.match(/-(390|1440)(?:-|$)/)?.[1] ?? metrics.innerWidth) && metrics.dpr === 1 && metrics.scale === 1 && (metrics.zoom === '1' || metrics.zoom === 'normal') && metrics.font.includes('InterVariable') && await page.evaluate(() => document.fonts.check('16px InterVariable')), metrics);
  }
  async function destination(page, title, action) {
    await Promise.all([page.waitForURL(`${base}/${title.toLowerCase().replace('synthetic ', '')}`, { waitUntil: 'networkidle' }), action()]);
    check(`${title}: exact GET destination`, await page.locator('h1').textContent() === title);
  }

  for (const scheme of ['light', 'dark']) for (const width of [390, 1440]) {
    const lane = `${scheme}-${width}`, page = await launch(width, scheme);
    await visit(page, `/repair-typed-${scheme}-${width}`);
    const mobile = width === 390;
    check(`${lane}: responsive visibility`, await page.locator('button[command="--toggle"]').isVisible() === mobile && await page.locator('nav div.hidden a').first().isVisible() !== mobile);
    if (mobile) { await page.locator('button[command="--toggle"]').click(); await ready(page); }
    const stateStyles = await page.evaluate(mobileLane => {
      const root = mobileLane ? document.querySelector('el-disclosure') : document.querySelector('[data-appkit-navbar-repair] > nav');
      const current = [...root.querySelectorAll('a')].find(link => link.checkVisibility() && link.getAttribute('aria-current') === 'page');
      const ordinary = [...root.querySelectorAll('a')].find(link => link.checkVisibility() && !link.hasAttribute('aria-current'));
      const pick = element => { const style = getComputedStyle(element); return { color: style.color, background: style.backgroundColor, weight: style.fontWeight }; };
      return { current: pick(current), ordinary: pick(ordinary) };
    }, mobile);
    check(`${lane}: current and default route states remain distinct`, JSON.stringify(stateStyles.current) !== JSON.stringify(stateStyles.ordinary), stateStyles);
    if (mobile) await visit(page, `/repair-typed-${scheme}-${width}`);
    if (mobile) {
      check(`${lane}: keyboard reaches mobile toggle`, await tabTo(page, 'button[command="--toggle"]'));
      let focus = await focusFacts(page); check(`${lane}: mobile toggle focus paint`, focus.passes, focus);
      await page.keyboard.press('Enter'); await ready(page); check(`${lane}: mobile directory opens`, await page.locator('el-disclosure a').evaluateAll(all => all.length === 2 && all.every(item => item.checkVisibility())));
      check(`${lane}: keyboard reaches mobile dashboard`, await tabTo(page, 'el-disclosure a[href="/dashboard"]'));
      focus = await focusFacts(page); check(`${lane}: mobile link focus paint`, focus.passes, focus);
      await destination(page, 'Synthetic dashboard', () => page.keyboard.press('Enter'));
      await visit(page, `/repair-typed-${scheme}-${width}`); await page.locator('button[command="--toggle"]').click(); await ready(page);
      await destination(page, 'Synthetic dashboard', () => page.locator('el-disclosure a[href="/dashboard"]').click());
      await visit(page, `/repair-typed-${scheme}-${width}`);
    }
    check(`${lane}: keyboard reaches account trigger`, await tabTo(page, 'el-dropdown > button'));
    let focus = await focusFacts(page); check(`${lane}: account trigger focus paint`, focus.passes, focus);
    await page.keyboard.press('Enter'); await ready(page); await page.waitForFunction(() => document.querySelector('el-menu')?.matches(':popover-open'));
    check(`${lane}: first account route owns focus`, await page.locator('el-menu a[href="/profile"]').evaluate(element => element === document.activeElement));
    focus = await focusFacts(page); check(`${lane}: account route focus paint`, focus.passes, focus);
    const qualityPage = { locator: selector => page.locator(selector === 'nav' ? '[data-appkit-navbar-repair] > nav' : selector) };
    const measured = await geometry(qualityPage); check(`${lane}: visible route containment`, measured.passes, measured);
    await capture(page, `${lane}-typed-account`);
    await destination(page, 'Synthetic profile', () => page.keyboard.press('Enter'));
    await visit(page, `/repair-typed-${scheme}-${width}`); await page.locator('el-dropdown > button').click(); await ready(page); await page.waitForFunction(() => document.querySelector('el-menu')?.matches(':popover-open'));
    await destination(page, 'Synthetic settings', () => page.locator('el-menu a[href="/settings"]').click());
    await visit(page, `/repair-typed-${scheme}-${width}`);
    check(`${lane}: native summary reachable`, await tabTo(page, 'details[data-appkit-navbar-routes] > summary')); focus = await focusFacts(page); check(`${lane}: native summary focus paint`, focus.passes, focus);
    await page.keyboard.press('Enter'); check(`${lane}: healthy native directory opens`, await page.locator('details[data-appkit-navbar-routes]').evaluate(element => element.open));

    await visit(page, `/repair-minimal-${scheme}-${width}`);
    check(`${lane}: optional controls absent`, await page.locator('el-dropdown,el-menu,a[href="/notifications"],a[href="/profile"],img[src="/assets/avatar.jpg"]').count() === 0);
    check(`${lane}: minimal native directory retains two primary routes`, await page.locator('details[data-appkit-navbar-routes] a').count() === 2);
    const minimalGeometry = await geometry({ locator: selector => page.locator(selector === 'nav' ? '[data-appkit-navbar-repair] > nav' : selector) });
    check(`${lane}: minimal visible route containment`, minimalGeometry.passes, minimalGeometry); await capture(page, `${lane}-minimal`);
    await closeContext();
  }

  {
    const page = await launch(390, 'light', false);
    await visit(page, '/repair-typed-light-390-disabled', false);
    check('JavaScript-disabled runtime remains undefined', await page.evaluate(() => !customElements.get('el-menu') && !customElements.get('el-disclosure')));
    check('JavaScript-disabled native summary reachable', await tabTo(page, 'details[data-appkit-navbar-routes] > summary')); await page.keyboard.press('Enter');
    check('JavaScript-disabled native directory opens', await page.locator('details[data-appkit-navbar-routes]').evaluate(element => element.open));
    await destination(page, 'Synthetic profile', () => page.locator('details[data-appkit-navbar-routes] a[href="/profile"]').click());
    await closeContext();
  }
  {
    const page = await launch(390, 'light');
    await visit(page, '/repair-typed-light-390-unavailable', false);
    check('HTTP503 runtime remains undefined', await page.evaluate(() => !customElements.get('el-menu')));
    check('HTTP503 native summary reachable', await tabTo(page, 'details[data-appkit-navbar-routes] > summary')); await page.keyboard.press('Enter');
    check('HTTP503 native directory opens', await page.locator('details[data-appkit-navbar-routes]').evaluate(element => element.open));
    check('HTTP503 native profile reachable', await tabTo(page, 'details[data-appkit-navbar-routes] a[href="/profile"]'));
    await destination(page, 'Synthetic profile', () => page.keyboard.press('Enter'));
    report.expected_failures.push({ kind: 'missing-runtime', detected: true, status: 503 });
    await visit(page, '/repair-typed-light-390-late', false); check('late-load native summary reachable', await tabTo(page, 'details[data-appkit-navbar-routes] > summary')); await page.keyboard.press('Enter');
    check('late-load native profile reachable', await tabTo(page, 'details[data-appkit-navbar-routes] a[href="/profile"]'));
    await page.evaluate(src => { const script = document.createElement('script'); script.type = 'module'; script.src = src; document.head.append(script); }, `${base}${prefix}/elements-navbar.js`); await ready(page);
    check('late runtime preserves native focus/open state', await page.locator('details[data-appkit-navbar-routes]').evaluate(element => element.open && element.contains(document.activeElement) && document.activeElement.getAttribute('href') === '/profile'));
    await destination(page, 'Synthetic profile', () => page.keyboard.press('Enter'));
    await visit(page, '/repair-typed-light-390-missing-css'); check('missing CSS canary reaches toggle', await tabTo(page, 'button[command="--toggle"]')); await page.keyboard.press('Enter'); await ready(page);
    check('missing CSS canary reaches primary link', await tabTo(page, 'el-disclosure a[href="/dashboard"]'));
    const missingCSSFocus = await focusFacts(page); check('missing delivered CSS is detected on formerly failing link', !missingCSSFocus.passes, missingCSSFocus);
    await page.evaluate(href => new Promise((resolve, reject) => { const link = document.createElement('link'); link.rel = 'stylesheet'; link.href = href; link.onload = resolve; link.onerror = reject; document.head.append(link); }), `${prefix}/navbar_repair.css`);
    await page.locator('#outside').focus(); await baseline(page); check('restored CSS canary returns to primary link', await tabTo(page, 'el-disclosure a[href="/dashboard"]'));
    const restoredCSSFocus = await focusFacts(page); check('restored delivered CSS passes unchanged focus oracle', restoredCSSFocus.passes, restoredCSSFocus);
    report.expected_failures.push({ kind: 'missing-repair-css', detected: !missingCSSFocus.passes, restored: restoredCSSFocus.passes });
    await closeContext();
  }

  await bounded(drainResponses());
  for (const [name, expected] of [[`${prefix}/source.css`, report.css_sha256], [`${prefix}/navbar_repair.css`, deliveryReceipt.files['navbar_repair.css']], [`${prefix}/elements-navbar.js`, deliveryReceipt.runtime_sha256]]) {
    const received = report.browser_responses.filter(row => row.path === name);
    check(`every browser response matches ${path.basename(name)}`, received.length > 0 && received.every(row => row.status === 200 && row.sha256 === expected), { count: received.length, expected, observed: [...new Set(received.map(row => `${row.status}:${row.sha256}`))] });
  }
  for (const [filename, digest] of inputs) assert.equal(sha(await fs.readFile(filename)), digest, `input changed: ${filename}`);
  report.inputs_unchanged = true;
} catch (error) {
  report.errors.push(String(error)); process.exitCode = 1;
} finally {
  try {
    if (context) { await bounded(drainResponses()); await bounded(context.close()); }
    if (browserPID) { assert(!alive(), 'owned Chrome remains'); report.hosts.at(-1).absent = true; }
    if (server?.listening) { server.closeAllConnections(); await bounded(new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()))); }
    if (base) await bounded(new Promise((resolve, reject) => { const socket = net.createConnection({ host: '127.0.0.1', port: Number(new URL(base).port) }); socket.once('connect', () => { socket.destroy(); reject(Error('listener remains')); }); socket.once('error', error => { socket.destroy(); error.code === 'ECONNREFUSED' ? resolve() : reject(error); }); }));
    report.cleanup = 'ok'; report.listener_refused = Boolean(base);
  } catch (error) { report.errors.push(`cleanup: ${error}`); try { if (alive()) process.kill(browserPID, 'SIGKILL'); } catch {} server?.closeAllConnections(); process.exitCode = 1; }
  clearTimeout(watchdog);
  report.inputs = Object.fromEntries(inputs); report.outputs = Object.fromEntries(outputs);
  report.passed = report.checks.filter(item => item.ok).length; report.failed = report.checks.filter(item => !item.ok).length;
  if (report.failed || report.errors.length) process.exitCode = 1;
  const body = Buffer.from(`${JSON.stringify(report, null, 2)}\n`), name = process.exitCode ? 'failure.json' : 'receipt.json';
  await fs.writeFile(path.join(out, name), body, { flag: 'wx', mode: 0o600 });
  process.stdout.write(`${JSON.stringify({ out, file: name, sha256: sha(body), passed: report.passed, failed: report.failed, errors: report.errors.map(error => error.split('\n')[0]) })}\n`);
}
