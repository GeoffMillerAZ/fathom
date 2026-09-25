// Paired static Gallery navigation/CSS-alias probe. Screenshots are supplemental;
// this does not assert pixel equivalence, performance, accessibility, or containment.
// Args: Playwright entry, Chrome executable, before root, after root, new output.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { createReadStream } from 'node:fs';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [playwrightEntry, chromePath, beforeRoot, afterRoot, outputRoot] = process.argv.slice(2);
for (const value of [playwrightEntry, chromePath, beforeRoot, afterRoot, outputRoot]) assert.ok(path.isAbsolute(value));
const routes = ['/analytics/signed', '/catalog', '/quality'];
const cases = routes.flatMap(route => ['light', 'dark'].flatMap(theme => [1280, 390].map(width =>
  ({ route, theme, width, id: `${route.slice(1).replaceAll('/', '-')}-${theme}-${width}` }))));
const properties = ['display', 'visibility', 'position', 'box-sizing', 'color', 'background-color', 'font-family', 'font-size',
  'font-weight', 'line-height', 'letter-spacing', 'text-align', 'text-decoration-line', 'border-top-width', 'border-right-width',
  'border-bottom-width', 'border-left-width', 'border-top-color', 'border-right-color', 'border-bottom-color', 'border-left-color',
  'border-radius', 'padding-top', 'padding-right', 'padding-bottom', 'padding-left', 'margin-top', 'margin-right', 'margin-bottom',
  'margin-left', 'gap', 'grid-template-columns', 'flex-direction', 'align-items', 'justify-content', 'min-width', 'min-height',
  'overflow-x', 'overflow-y', 'outline-style', 'outline-width', 'outline-color', 'outline-offset', 'box-shadow', 'opacity',
  'transform', 'white-space', 'cursor'];
const report = { scope: '3 Gallery routes x 2 themes x 2 widths; paired retained HTML/CSS', identities: {}, cases: [],
  blocked: [], failures: [], passed: false, limits: ['screenshots are supplemental', 'not performance', 'not full accessibility',
    'not HTTP or OS network containment', 'no JavaScript states'] };
let browser, watchdog, timedOut = false; const contexts = new Set();

const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
function hashFile(file) { return new Promise((resolve, reject) => { const hash = crypto.createHash('sha256');
  createReadStream(file).on('error', reject).on('data', chunk => hash.update(chunk)).on('end', () => resolve(hash.digest('hex'))); }); }
async function packageIdentity(entry) { let dir = path.dirname(await fs.realpath(entry));
  while (true) { try { const pkg = JSON.parse(await fs.readFile(path.join(dir, 'package.json'), 'utf8'));
      if (pkg.version && String(pkg.name).includes('playwright')) return { package: pkg.name, version: pkg.version }; }
    catch (error) { if (error.code !== 'ENOENT') throw error; }
    const parent = path.dirname(dir); if (parent === dir) return { package: null, version: null }; dir = parent; } }
function check(cell, name, pass, detail) { cell.checks.push({ name, pass, ...(detail === undefined ? {} : { detail }) });
  if (!pass) report.failures.push({ case: cell.id, name, detail }); }
async function closeBounded(owner, label) { if (!owner) return; let timer;
  try { await Promise.race([owner.close(), new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(`${label} close timeout`)), 3000); })]); }
  catch (error) { report.failures.push({ case: 'owner', name: `${label} close`, detail: String(error) }); }
  finally { clearTimeout(timer); } }
async function readRoot(root, label) {
  const realRoot = await fs.realpath(root), manifestBytes = await fs.readFile(path.join(realRoot, 'inputs.json'));
  const entries = JSON.parse(manifestBytes); assert.ok(Array.isArray(entries), `${label} inputs.json must be an array`);
  const css = await fs.readFile(path.join(realRoot, 'app.css')), documents = new Map(), identities = [];
  for (const entry of entries) { assert.ok(routes.includes(entry.path) && ['light', 'dark'].includes(entry.theme));
    const file = path.resolve(realRoot, entry.file); assert.ok(file.startsWith(realRoot + path.sep), `${label} input escapes root`);
    const body = await fs.readFile(file), digest = sha(body); assert.equal(digest, entry.sha256, `${label} ${entry.file} hash`);
    const key = `${entry.path}\0${entry.theme}`; assert.ok(!documents.has(key), `${label} duplicate ${key}`); documents.set(key, body);
    identities.push({ path: entry.path, theme: entry.theme, file: entry.file, bytes: body.length, sha256: digest });
  }
  assert.equal(documents.size, 6, `${label} document count`);
  for (const route of routes) for (const theme of ['light', 'dark']) assert.ok(documents.has(`${route}\0${theme}`));
  return { root: realRoot, css, documents, identity: { root: realRoot, manifest: { bytes: manifestBytes.length, sha256: sha(manifestBytes) },
    css: { bytes: css.length, sha256: sha(css) }, documents: identities } };
}
async function firstVisible(locator) { for (const candidate of await locator.all()) if (await candidate.isVisible()) return candidate;
  throw new Error(`no visible match for ${locator}`); }
async function tabTo(page, locator) { const handle = await locator.elementHandle(); assert.ok(handle);
  for (let presses = 1; presses <= 100; presses++) { await page.keyboard.press('Tab');
    if (await handle.evaluate(node => node === document.activeElement).catch(() => false)) return presses; }
  return 0;
}
async function visitTarget(page, cell, locator, name) {
  const presses = await tabTo(page, locator), state = await locator.evaluate((node, presses) => { const c = getComputedStyle(node), r = node.getBoundingClientRect();
    return { tag: node.tagName, text: node.textContent.replace(/\s+/g, ' ').trim(), href: node.getAttribute('href'), presses,
      rect: [r.x, r.y, r.width, r.height].map(value => +value.toFixed(3)), outline: [c.outlineStyle, parseFloat(c.outlineWidth) || 0, c.outlineColor, c.outlineOffset] }; }, presses);
  check(cell, `${name} actual-Tab reachability`, presses > 0, state); check(cell, `${name} 2px focus`, state.outline[0] !== 'none' && state.outline[1] >= 2, state);
  check(cell, `${name} 44px target`, state.rect[2] >= 44 && state.rect[3] >= 44, state); await locator.hover();
  state.hover = await locator.evaluate(node => { const c = getComputedStyle(node); return { color: c.color, background: c.backgroundColor,
    decoration: c.textDecorationLine, border: c.borderColor, shadow: c.boxShadow }; }); return state;
}
async function exerciseNavigation(page, cell, width) {
  await page.locator('body').click({ position: { x: 1, y: 1 } }); const states = [];
  states.push(await visitTarget(page, cell, await firstVisible(page.locator('header a').filter({ hasText: /^app-kit Gallery$/ })), 'home'));
  states.push(await visitTarget(page, cell, await firstVisible(page.locator('header [role="group"][aria-label="Theme"] a[aria-current="true"]')), 'selected theme'));
  if (width === 1280) {
    states.push(await visitTarget(page, cell, await firstVisible(page.locator('header nav[aria-label="Gallery sections"] a:not([aria-current])')), 'desktop noncurrent nav'));
  } else {
    const summary = page.locator('#gallery-navigation > summary'); states.push(await visitTarget(page, cell, summary, 'mobile summary'));
    await page.keyboard.press('Enter'); check(cell, 'mobile native disclosure opens', await page.locator('#gallery-navigation').evaluate(node => node.open));
    states.push(await visitTarget(page, cell, await firstVisible(page.locator('#gallery-navigation nav a:not([aria-current])')), 'mobile noncurrent nav'));
  }
  return states;
}
async function snapshot(page) {
  return page.evaluate(properties => { const round = value => +value.toFixed(3), norm = text => text.replace(/\s+/g, ' ').trim();
    const nodes = [document.body, ...document.body.querySelectorAll('*')];
    const semantics = nodes.map((node, index) => ({ index, tag: node.tagName,
      attrs: [...node.attributes].filter(attr => attr.name !== 'class').map(attr => [attr.name, attr.value]).sort((a, b) => a[0].localeCompare(b[0])),
      text: norm([...node.childNodes].filter(child => child.nodeType === Node.TEXT_NODE).map(child => child.textContent).join(' ')) }));
    const computed = nodes.map((node, index) => { const c = getComputedStyle(node), r = node.getBoundingClientRect(), styles = {};
      for (const property of properties) styles[property] = c.getPropertyValue(property); return { index, tag: node.tagName,
        rect: [r.x, r.y, r.width, r.height].map(round), styles }; });
    const visibleTargets = [...document.querySelectorAll('header a, header summary')].filter(node => node.getClientRects().length && getComputedStyle(node).visibility !== 'hidden');
    return { semantics, computed, fit: document.documentElement.scrollWidth <= innerWidth && document.body.scrollWidth <= innerWidth && scrollX === 0,
      targets44: visibleTargets.every(node => { const r = node.getBoundingClientRect(); return r.width >= 44 && r.height >= 44; }),
      mainText: norm(document.querySelector('main')?.innerText || ''),
      sources: [...document.querySelectorAll('main [data-viz-source], main [id$="-src"], main figcaption')].map(node => ({ tag: node.tagName,
        attrs: [...node.attributes].filter(attr => attr.name !== 'class').map(attr => [attr.name, attr.value]).sort(), text: norm(node.innerText) })) };
  }, properties);
}
function compare(cell, name, before, after) {
  if (JSON.stringify(before) === JSON.stringify(after)) { check(cell, name, true); return; }
  const differences = [], a = Array.isArray(before) ? before : [before], b = Array.isArray(after) ? after : [after]; let count = 0;
  for (let i = 0; i < Math.max(a.length, b.length); i++) if (JSON.stringify(a[i]) !== JSON.stringify(b[i])) {
    count++; if (differences.length < 40) differences.push({ index: i, before: a[i], after: b[i] }); }
  cell.differences.push({ name, count, truncated: count > differences.length, differences });
  check(cell, name, false, { count, retained: differences.length, bounded: 40 });
}
async function openSide(root, entry, spec, side, cell) {
  const context = await browser.newContext({ viewport: { width: spec.width, height: 900 }, colorScheme: spec.theme,
    javaScriptEnabled: false, serviceWorkers: 'block', reducedMotion: 'reduce' }); contexts.add(context); context.setDefaultTimeout(10000);
  const pageURL = `http://127.0.0.1:65533${spec.route}?theme=${spec.theme}`;
  await context.route('**/*', async route => { const req = route.request(), url = req.url();
    if (req.method() === 'GET' && url === pageURL) return route.fulfill({ status: 200, contentType: 'text/html', body: entry });
    if (req.method() === 'GET' && url === 'http://127.0.0.1:65533/static/app.css') return route.fulfill({ status: 200, contentType: 'text/css', body: root.css });
    report.blocked.push({ case: cell.id, side, method: req.method(), url }); await route.abort(); });
  await context.routeWebSocket('**/*', socket => { report.blocked.push({ case: cell.id, side, websocket: socket.url() }); socket.close(); });
  const page = await context.newPage(); await page.goto(pageURL, { waitUntil: 'load', timeout: 10000 }); return { context, page };
}
async function run() {
  await fs.mkdir(outputRoot, { mode: 0o700 }); const driver = await fs.realpath(process.argv[1]);
  const pw = await fs.realpath(playwrightEntry), chrome = await fs.realpath(chromePath), before = await readRoot(beforeRoot, 'before'), after = await readRoot(afterRoot, 'after');
  report.identities = { driver: { path: driver, sha256: await hashFile(driver) }, before: before.identity, after: after.identity,
    playwright: { entry: pw, sha256: await hashFile(pw), ...await packageIdentity(pw) }, chrome: { executable: chrome, sha256: await hashFile(chrome), version: null } };
  if (!report.identities.playwright.version) report.failures.push({ case: 'identity', name: 'Playwright version unavailable' });
  const { chromium } = await import(pathToFileURL(pw).href); browser = await chromium.launch({ executablePath: chrome, headless: true, timeout: 30000 });
  report.identities.chrome.version = browser.version();
  for (const spec of cases) { if (timedOut) break; const cell = { ...spec, checks: [], differences: [], screenshots: {} }; report.cases.push(cell); let left, right;
    try { left = await openSide(before, before.documents.get(`${spec.route}\0${spec.theme}`), spec, 'before', cell);
      right = await openSide(after, after.documents.get(`${spec.route}\0${spec.theme}`), spec, 'after', cell);
      const initialBefore = await snapshot(left.page), initialAfter = await snapshot(right.page);
      compare(cell, 'initial DOM attributes/direct text excluding class', initialBefore.semantics, initialAfter.semantics);
      compare(cell, 'initial computed styles and geometry', initialBefore.computed, initialAfter.computed);
      compare(cell, 'main content semantics', initialBefore.mainText, initialAfter.mainText); compare(cell, 'source semantics', initialBefore.sources, initialAfter.sources);
      check(cell, 'initial body fit', initialBefore.fit && initialAfter.fit, { before: initialBefore.fit, after: initialAfter.fit });
      check(cell, 'visible header targets are 44px', initialBefore.targets44 && initialAfter.targets44, { before: initialBefore.targets44, after: initialAfter.targets44 });
      const beforeState = await exerciseNavigation(left.page, cell, spec.width), afterState = await exerciseNavigation(right.page, cell, spec.width);
      compare(cell, 'focus hover and native-navigation states', beforeState, afterState);
      const postBefore = await snapshot(left.page), postAfter = await snapshot(right.page);
      compare(cell, 'post-task DOM attributes/direct text excluding class', postBefore.semantics, postAfter.semantics);
      compare(cell, 'post-task computed styles and geometry', postBefore.computed, postAfter.computed);
      check(cell, 'post-task body fit', postBefore.fit && postAfter.fit, { before: postBefore.fit, after: postAfter.fit });
      for (const [side, page] of [['before', left.page], ['after', right.page]]) { const file = `${cell.id}-${side}.png`; cell.screenshots[side] = file;
        await page.screenshot({ path: path.join(outputRoot, file), fullPage: false, animations: 'disabled', caret: 'hide' }); await fs.chmod(path.join(outputRoot, file), 0o600); }
    } catch (error) { check(cell, 'paired cell completed', false, String(error.stack || error)); }
    finally { for (const owner of [left?.context, right?.context]) { await closeBounded(owner, `${cell.id} context`); contexts.delete(owner); } }
  }
}

const work = run(); const deadline = new Promise((_, reject) => { watchdog = setTimeout(() => { timedOut = true;
  void closeBounded(browser, 'watchdog browser'); reject(new Error('120s owner timeout')); }, 120000); });
try { await Promise.race([work, deadline]); }
catch (error) { report.failures.push({ case: 'owner', name: timedOut ? 'owner timeout' : 'execution', detail: String(error.stack || error) }); }
finally { clearTimeout(watchdog); for (const owner of [...contexts]) await closeBounded(owner, 'final context'); contexts.clear();
  await closeBounded(browser, 'browser'); browser = undefined; let drainTimer;
  const drained = await Promise.race([work.then(() => true, () => true), new Promise(resolve => { drainTimer = setTimeout(() => resolve(false), 3000); })]); clearTimeout(drainTimer);
  if (!drained) report.failures.push({ case: 'owner', name: 'worker did not settle after cleanup' });
  report.passed = !timedOut && report.cases.length === 12 && report.failures.length === 0 && report.blocked.length === 0;
  try { await fs.writeFile(path.join(outputRoot, 'report.json'), JSON.stringify(report, null, 2), { flag: 'wx', mode: 0o600 }); }
  catch (error) { process.stderr.write(`report write failed: ${error}\n`); process.exitCode = 1; } if (!report.passed) process.exitCode = 1; }
