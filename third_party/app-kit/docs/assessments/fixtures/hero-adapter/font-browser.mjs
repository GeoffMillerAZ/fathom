// Opt-in P01 font candidate browser evidence. Run only from ROOT/runner.
import fs from 'node:fs/promises';
import {createReadStream} from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import {paintedFonts} from './font-painted.mjs';

const exec = promisify(execFile), here = path.dirname(fileURLToPath(import.meta.url));
const [rootArg] = process.argv.slice(2);
assert.ok(rootArg && path.isAbsolute(rootArg), 'usage: node ROOT/runner/font-browser.mjs ABS_PREPARED_ROOT');
const root = await fs.realpath(rootArg);
assert.equal(await fs.realpath(here), await fs.realpath(path.join(root, 'runner')), 'must execute ROOT/runner/font-browser.mjs');
const sha = b => createHash('sha256').update(b).digest('hex');
const hashFile = async file => { const hash = createHash('sha256'); for await (const chunk of createReadStream(file)) hash.update(chunk); return hash.digest('hex'); };
const read = async file => { const st = await fs.lstat(file); assert.ok(st.isFile() && !st.isSymbolicLink() && st.size <= 64 * 1024 * 1024, 'bounded regular input: ' + file); return fs.readFile(file); };
const write = (file, body) => fs.writeFile(file, body, {flag: 'wx', mode: 0o600});
const prepared = JSON.parse(await read(path.join(root, 'prepared.json')));
assert.equal(prepared.schema, 1, 'prepared schema');
assert.ok(Array.isArray(prepared.resources) && prepared.font && typeof prepared.expectedBrowserVersion === 'string', 'prepared fields');
const inside = async rel => { assert.ok(typeof rel === 'string' && !path.isAbsolute(rel), 'relative resource path'); const lexical = path.resolve(root, rel); const lexicalTail = path.relative(root, lexical); assert.ok(lexicalTail && !lexicalTail.startsWith('..' + path.sep) && lexicalTail !== '..' && !path.isAbsolute(lexicalTail), 'resource escapes root'); const file = await fs.realpath(lexical); const tail = path.relative(root, file); assert.ok(tail && !tail.startsWith('..' + path.sep) && tail !== '..' && !path.isAbsolute(tail), 'real resource escapes root'); return file; };
const resources = new Map();
for (const item of prepared.resources) {
  assert.ok(item && typeof item.path === 'string' && typeof item.type === 'string' && typeof item.file === 'string' && /^[a-f0-9]{64}$/.test(item.sha256) && Number.isSafeInteger(item.bytes), 'resource shape');
  assert.ok(item.path.startsWith('/') && !item.path.includes('?') && !resources.has(item.path), 'unique absolute request path');
  const body = await read(await inside(item.file));
  assert.equal(body.length, item.bytes, item.path + ' bytes'); assert.equal(sha(body), item.sha256, item.path + ' sha');
  resources.set(item.path, {...item, body});
}
for (const wanted of ['/enhanced', '/source.css', '/candidate.css', '/hero.css', '/assets/inter.woff2', '/assets/inter-subset.woff2', '/assets/logo-light.svg', '/assets/logo-dark.svg', '/assets/hero.jpg']) assert.ok(resources.has(wanted), 'missing prepared ' + wanted);
assert.equal(resources.get('/assets/inter.woff2').sha256, prepared.font.originalSHA, 'original font pin');
assert.equal(resources.get('/assets/inter-subset.woff2').sha256, prepared.font.subsetSHA, 'subset font pin');
const modulePath = process.env.APPKIT_PLAYWRIGHT_MODULE;
assert.ok(modulePath && path.isAbsolute(modulePath), 'existing approved Playwright module required');
assert.equal(sha(await read(await fs.realpath(modulePath))), prepared.playwrightEntrySHA, 'Playwright entry pin');
const out = path.join(root, 'font-browser-evidence'); await fs.mkdir(out, {mode: 0o700});
const report = {schema: 1, kind: 'p01-font-delivery-browser-v1', experiment: 'separate font-delivery counterfactual; baseline source font and its receipt remain unchanged', startedAt: new Date().toISOString(), root, inputs: {}, captures: [], lanes: [], serverRequests: [], failures: [], limits: ['Paired P01 fidelity, typography and font-selection evidence only.', 'No performance matrix is executed here; its request-bound warm body gate remains separate.', 'Inherited performance coordinates/budgets do not alter the unchanged-assets target claim.', 'Headless Chrome at named viewports, not target certification or consumer acceptance.']};
report.inputs.preparedSHA = sha(await read(path.join(root, 'prepared.json')));
report.inputs.runnerSHA = sha(await read(fileURLToPath(import.meta.url)));
for (const [name, item] of resources) report.inputs[name] = {sha256: item.sha256, bytes: item.bytes};

let activeVariant = 'baseline', activeControl = 'none', server, base, allowedHost = '', browserServer, browserProcess, browser, closing, aborted = false;
const bounded = (promise, ms, label) => new Promise((resolve, reject) => { const timer = setTimeout(() => reject(Error(label + ' exceeded ' + ms + 'ms')), ms); Promise.resolve(promise).then(value => { clearTimeout(timer); resolve(value); }, error => { clearTimeout(timer); reject(error); }); });
const waitForExit = proc => !proc || proc.exitCode !== null || proc.signalCode !== null ? Promise.resolve() : bounded(new Promise(resolve => proc.once('close', resolve)), 5000, 'owned browser exit');
const closeBrowser = async () => {
  if (closing) return closing;
  closing = (async () => {
    if (browser) await bounded(browser.close(), 10000, 'browser disconnect').catch(e => report.failures.push({phase: 'cleanup', error: String(e)}));
    if (browserServer) await bounded(browserServer.close(), 10000, 'browser close').catch(async e => { report.failures.push({phase: 'cleanup', error: String(e)}); if (browserProcess?.exitCode === null && browserProcess.signalCode === null) browserProcess.kill('SIGKILL'); await waitForExit(browserProcess).catch(exitError => report.failures.push({phase: 'cleanup', error: String(exitError)})); });
    await waitForExit(browserProcess).catch(e => report.failures.push({phase: 'cleanup', error: String(e)}));
    report.browserProcess = browserProcess ? {pid: browserProcess.pid, exitCode: browserProcess.exitCode, signalCode: browserProcess.signalCode} : null;
  })();
  return closing;
};
const stopOwnedWork = reason => { if (aborted) return; aborted = true; report.failures.push({phase: 'watchdog', error: reason}); server?.closeAllConnections(); closeBrowser().catch(error => report.failures.push({phase: 'cleanup', error: String(error)})); };
const watchdog = setTimeout(() => stopOwnedWork('120s bound'), 120000);
process.once('SIGTERM', () => stopOwnedWork('SIGTERM received'));
const fail = (lane, error) => { lane.ok = false; lane.failures.push(String(error)); report.failures.push({lane: lane.id, error: String(error)}); };
const csp = "default-src 'self'; script-src 'none'; connect-src 'none'; img-src 'self' data:; font-src 'self'; style-src 'self' 'unsafe-inline'; base-uri 'none'; form-action 'none'";
const typographyCases = [
  'AVATAR To WA', 'office affine fi fl ffi ffl', 'café | é | naïve | ñ | Å | Å', 'İ | i̇ | ı', 'ß | ẞ | Straße',
  'ɏ | ɐ | ɐ́', 'Ά | ά', 'Ж́', '“quote”—€↔', 'A⃝ | aͯ',
];
const typographySelectors = typographyCases.flatMap((_, i) => [100, 400, 500, 600, 900].flatMap(weight => [14, 32].map(opsz => `[data-font-id="${i}-${weight}-${opsz}"]`)));
const typographyHTML = mode => {
  const rows = typographyCases.flatMap((text, i) => [100, 400, 500, 600, 900].flatMap(weight => [14, 32].map(opsz => `<p data-case="c${i}" data-font-id="${i}-${weight}-${opsz}" data-weight="${weight}" data-opsz="${opsz}" style="font-family:InterVariable;font-size:32px;font-weight:${weight};font-optical-sizing:none;font-variation-settings:'opsz' ${opsz},'wght' ${weight}">${text}</p>`))).join('');
  const selection = mode === 'subset' ? '<p data-selection="subset" style="font-family:InterVariable;font-size:32px">office café €↔</p>' : mode === 'original' ? '<p data-selection="original" style="font-family:InterVariable;font-size:32px">Α</p>' : '';
  return `<!doctype html><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="${csp}"><link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'/%3E"><link rel="stylesheet" href="/source.css"><main id="font-fixture">${selection || rows}</main>`;
};
server = http.createServer((req, res) => {
  const url = new URL(req.url, 'http://local'); const record = {variant: activeVariant, control: activeControl, method: req.method, host: req.headers.host ?? '', path: url.pathname + url.search}; report.serverRequests.push(record);
  if (req.headers.host !== allowedHost || req.method !== 'GET' || url.search) { record.status = 400; res.writeHead(400, {'Content-Type': 'text/plain', 'Cache-Control': 'no-store'}); return res.end('refused'); }
  let item;
  if (url.pathname === '/typography') item = {body: Buffer.from(typographyHTML('all')), type: 'text/html; charset=utf-8'};
  else if (url.pathname === '/selection-subset') item = {body: Buffer.from(typographyHTML('subset')), type: 'text/html; charset=utf-8'};
  else if (url.pathname === '/selection-original') item = {body: Buffer.from(typographyHTML('original')), type: 'text/html; charset=utf-8'};
  else if (url.pathname === '/source.css') item = resources.get(activeVariant === 'candidate' ? '/candidate.css' : '/source.css');
  else item = resources.get(url.pathname);
  if (!item || url.search) { record.status = 404; res.writeHead(404, {'Content-Type': 'text/plain', 'Cache-Control': 'no-store'}); return res.end('not found'); }
  const missingSubset = activeControl === 'missing-subset' && url.pathname === '/assets/inter-subset.woff2'; const body = missingSubset ? Buffer.alloc(0) : item.body, status = missingSubset ? 404 : 200;
  record.status = status; record.bodyBytes = body.length;
  res.writeHead(status, {'Content-Type': item.type, 'Content-Length': body.length, 'Cache-Control': missingSubset || url.pathname === '/enhanced' || url.pathname.startsWith('/typography') || url.pathname.startsWith('/selection-') ? 'no-store' : 'private,max-age=3600,immutable', 'Content-Security-Policy': csp, 'X-Content-Type-Options': 'nosniff'}); res.end(body);
});
await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); }); base = `http://127.0.0.1:${server.address().port}`; allowedHost = new URL(base).host; report.origin = base;

const expectedBody = (pathname, variant, dynamic) => dynamic?.get(pathname)?.body ?? (pathname === '/source.css' ? resources.get(variant === 'candidate' ? '/candidate.css' : '/source.css').body : resources.get(pathname)?.body);
const geometry = page => page.evaluate(() => ({text: document.body.innerText, links: [...document.querySelectorAll('a')].filter(a => { const b = a.getBoundingClientRect(); return b.width > 0 && b.height > 0; }).map(a => a.getAttribute('href')), boxes: [...document.querySelectorAll('body *')].map(el => { const b = el.getBoundingClientRect(); return [el.tagName, el.getAttribute('id'), el.getAttribute('class'), el.children.length ? null : el.textContent, b.x, b.y, b.width, b.height]; })}));
async function graphAndBodies(cdp, observed, variant, dynamic, lane) {
  const graph = [];
  for (const item of observed.values()) {
    const expected = expectedBody(item.path, variant, dynamic); const row = {...item, expectedSHA256: expected ? sha(expected) : null, expectedBytes: expected?.length ?? null};
    try { const requestURL = new URL(item.url); assert.equal(requestURL.origin, base, 'request origin'); assert.equal(requestURL.search, '', 'request query'); assert.equal(item.method, 'GET', 'request method'); assert.equal(item.status, 200, 'response status'); assert.equal(item.finished, true, 'response completed'); assert.equal(item.error, undefined, 'response failure'); assert.equal(item.cache, false, 'fresh-context cache'); const body = await cdp.send('Network.getResponseBody', {requestId: item.id}); const bytes = Buffer.from(body.body, body.base64Encoded ? 'base64' : 'utf8'); row.sha256 = sha(bytes); row.bodyBytes = bytes.length; if (!expected || row.sha256 !== row.expectedSHA256 || row.bodyBytes !== row.expectedBytes) throw Error('response identity mismatch ' + item.path); }
    catch (error) { row.bodyError = String(error); lane.assetFailures ??= []; lane.assetFailures.push({path: item.path, status: item.status ?? null, error: String(error)}); if (!lane.control) fail(lane, error); }
    graph.push(row);
  }
  lane.graph = graph;
}
async function customFont(cdp, selector, lane) {
  await cdp.send('DOM.enable'); await cdp.send('CSS.enable'); const doc = await cdp.send('DOM.getDocument'); const node = await cdp.send('DOM.querySelector', {nodeId: doc.root.nodeId, selector}); const result = await cdp.send('CSS.getPlatformFontsForNode', {nodeId: node.nodeId});
  const fonts = result.fonts ?? []; lane.platformFonts ??= {}; lane.platformFonts[selector] = fonts;
  assert.ok(fonts.length && fonts.every(font => font.isCustomFont && /Inter/i.test(font.familyName)), 'custom Inter only for ' + selector);
}
async function runLane({id, variant, pathname, width, scheme, dynamic, expectedPaths, screenshot = true, platformSelectors = [], control = null}) {
  const lane = {id, variant, pathname, width, scheme, control, ok: true, failures: [], assetFailures: []}; report.lanes.push(lane); activeVariant = variant; activeControl = control ?? 'none';
  let context;
  try {
    assert.equal(aborted, false, 'watchdog already fired');
    context = await browser.newContext({viewport: {width, height: 1000}, colorScheme: scheme, reducedMotion: 'reduce', deviceScaleFactor: 1, serviceWorkers: 'block'});
    const external = []; await context.route('**/*', route => { const u = new URL(route.request().url()); if (u.origin !== base) { external.push(route.request().url()); return route.abort(); } return route.continue(); });
    const page = await context.newPage(); page.setDefaultTimeout(7000); const cdp = await context.newCDPSession(page); const observed = new Map();
    await cdp.send('Network.enable'); await cdp.send('Network.clearBrowserCache'); await cdp.send('Network.setCacheDisabled', {cacheDisabled: false});
    cdp.on('Network.requestWillBeSent', e => { const u = new URL(e.request.url); observed.set(e.requestId, {id: e.requestId, path: u.pathname, url: e.request.url, method: e.request.method, type: e.type, cache: false}); });
    cdp.on('Network.requestServedFromCache', e => { const item = observed.get(e.requestId); if (item) item.cache = true; });
    cdp.on('Network.responseReceived', e => { const item = observed.get(e.requestId); if (item) Object.assign(item, {status: e.response.status, mime: e.response.mimeType, cache: Boolean(item.cache || e.response.fromDiskCache || e.response.fromPrefetchCache || e.response.fromServiceWorker)}); });
    cdp.on('Network.loadingFinished', e => { const item = observed.get(e.requestId); if (item) item.finished = true; });
    cdp.on('Network.loadingFailed', e => { const item = observed.get(e.requestId); if (item) item.error = e.errorText || 'loading failed'; });
    const response = await page.goto(base + pathname, {waitUntil: 'load'}); assert.equal(response.status(), 200); const fontSettled = await page.evaluate(async () => { await document.fonts.ready; await Promise.all([...document.images].map(image => image.decode().catch(() => {}))); await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))); return {settled: true, status: document.fonts.status}; }); assert.equal(aborted, false, 'watchdog fired during lane');
    const state = await geometry(page); lane.state = state; const fontCheck = await page.evaluate(text => document.fonts.check('32px InterVariable', text), state.text); lane.fontCheck = {...fontSettled, text: state.text, check: fontCheck}; if (!control && (!fontSettled.settled || fontSettled.status !== 'loaded')) fail(lane, Error('FontFaceSet did not settle loaded'));
    try { const painted = await paintedFonts(page, cdp); lane.paintedNodes = painted.nodes; lane.paintedFailures = painted.failures; if (!control) for (const issue of painted.failures) fail(lane, Error(issue)); } catch (error) { lane.paintedFailures = [String(error)]; if (!control) fail(lane, error); }
    for (const selector of platformSelectors) { try { await customFont(cdp, selector, lane); } catch (error) { fail(lane, error); } }
    const tab = []; for (let i = 0; i < state.links.length; i++) { await page.keyboard.press('Tab'); tab.push(await page.evaluate(() => document.activeElement?.getAttribute('href'))); } assert.deepEqual(tab, state.links, 'native Tab order'); lane.tab = tab;
    if (screenshot) { const png = await page.screenshot({fullPage: pathname !== '/enhanced', animations: 'disabled'}); const file = `${id}.png`; await write(path.join(out, file), png); lane.capture = {file, sha256: sha(png), bytes: png.length}; report.captures.push(lane.capture); }
    await graphAndBodies(cdp, observed, variant, dynamic, lane);
    if (control && lane.assetFailures.length) lane.ok = false;
    const paths = [...new Set([...observed.values()].map(item => item.path))];
    if (control === 'missing-subset') { assert.ok(expectedPaths.every(item => paths.includes(item)), 'control retains ordinary request paths'); assert.ok(paths.every(item => expectedPaths.includes(item) || item === '/assets/inter.woff2'), 'control only permits original fallback extra request'); assert.equal(paths.length, observed.size, 'control requests have no duplicates'); }
    else { assert.equal(observed.size, expectedPaths.length, 'exact request count'); assert.deepEqual(paths.sort(), [...expectedPaths].sort(), 'exact request paths'); }
    assert.equal(external.length, 0, 'external requests');
  } catch (error) { fail(lane, error); } finally { activeControl = 'none'; if (context) await context.close(); }
  return lane;
}

try {
  const chrome = process.env.APPKIT_BROWSER_EXECUTABLE || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'; assert.ok(path.isAbsolute(chrome)); const version = (await exec(chrome, ['--version'], {timeout: 10000})).stdout.trim(); assert.ok(version.endsWith(' ' + prepared.expectedBrowserVersion), 'Chrome version'); report.browser = {path: chrome, version, sha256: await hashFile(chrome)};
  const {chromium} = await import(pathToFileURL(modulePath)); browserServer = await chromium.launchServer({headless: true, executablePath: chrome, args: ['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1']}); browserProcess = browserServer.process(); report.browserProcess = {pid: browserProcess.pid, exitCode: browserProcess.exitCode, signalCode: browserProcess.signalCode};
  const ownedBrowserFile = path.join(root, 'browser-owned.json'); await write(ownedBrowserFile, JSON.stringify({schema: 1, pid: browserProcess.pid, executable: chrome, startedAt: new Date().toISOString()}, null, 2) + '\n'); report.ownedBrowser = {file: ownedBrowserFile, pid: browserProcess.pid};
  browser = await chromium.connect(browserServer.wsEndpoint()); assert.equal(browser.version(), prepared.expectedBrowserVersion);
  const hero = new Map([['/enhanced', resources.get('/enhanced')], ['/source.css', resources.get('/source.css')], ['/hero.css', resources.get('/hero.css')], ['/assets/logo-light.svg', resources.get('/assets/logo-light.svg')], ['/assets/logo-dark.svg', resources.get('/assets/logo-dark.svg')], ['/assets/hero.jpg', resources.get('/assets/hero.jpg')]]);
  const pairs = [];
  for (const width of [390, 1440]) for (const scheme of ['light', 'dark']) {
    const key = `${width}-${scheme}`; const baseline = await runLane({id: `hero-baseline-${key}`, variant: 'baseline', pathname: '/enhanced', width, scheme, expectedPaths: [...hero.keys(), '/assets/inter.woff2']});
    const candidate = await runLane({id: `hero-candidate-${key}`, variant: 'candidate', pathname: '/enhanced', width, scheme, expectedPaths: [...hero.keys(), '/assets/inter-subset.woff2']});
    const pair = {key, ok: baseline.ok && candidate.ok, baseline: baseline.id, candidate: candidate.id}; try { assert.ok(baseline.capture && candidate.capture, 'paired viewport capture missing'); assert.equal(candidate.capture.sha256, baseline.capture.sha256, 'viewport PNG'); assert.deepEqual(candidate.state, baseline.state, 'visible text and geometry'); } catch (error) { pair.ok = false; fail(candidate, error); } pairs.push(pair);
  }
  report.fidelity = {pairs, passed: pairs.every(pair => pair.ok)};
  const typographyDynamic = new Map([['/typography', {body: Buffer.from(typographyHTML('all'))}]]);
  const baselineTypography = await runLane({id: 'typography-baseline', variant: 'baseline', pathname: '/typography', width: 1440, scheme: 'light', dynamic: typographyDynamic, expectedPaths: ['/typography', '/source.css', '/assets/inter.woff2'], platformSelectors: typographySelectors});
  const candidateTypography = await runLane({id: 'typography-candidate', variant: 'candidate', pathname: '/typography', width: 1440, scheme: 'light', dynamic: typographyDynamic, expectedPaths: ['/typography', '/source.css', '/assets/inter.woff2', '/assets/inter-subset.woff2'], platformSelectors: typographySelectors});
  report.typography = {passed: baselineTypography.ok && candidateTypography.ok}; try { assert.ok(baselineTypography.capture && candidateTypography.capture, 'paired typography capture missing'); assert.equal(candidateTypography.capture.sha256, baselineTypography.capture.sha256, 'typography PNG'); assert.deepEqual(candidateTypography.state, baselineTypography.state, 'typography geometry'); } catch (error) { report.typography.passed = false; fail(candidateTypography, error); }
  const subsetDynamic = new Map([['/selection-subset', {body: Buffer.from(typographyHTML('subset'))}]]), originalDynamic = new Map([['/selection-original', {body: Buffer.from(typographyHTML('original'))}]]);
  const subset = await runLane({id: 'selection-subset', variant: 'candidate', pathname: '/selection-subset', width: 1440, scheme: 'light', dynamic: subsetDynamic, expectedPaths: ['/selection-subset', '/source.css', '/assets/inter-subset.woff2'], platformSelectors: ['[data-selection="subset"]']});
  const original = await runLane({id: 'selection-original', variant: 'candidate', pathname: '/selection-original', width: 1440, scheme: 'light', dynamic: originalDynamic, expectedPaths: ['/selection-original', '/source.css', '/assets/inter.woff2'], platformSelectors: ['[data-selection="original"]']});
  report.selection = {passed: subset.ok && original.ok};
  const candidateHeroPaths = [...hero.keys(), '/assets/inter-subset.woff2'];
  const missingSubset = await runLane({id: 'negative-missing-subset-390-light', variant: 'candidate', pathname: '/enhanced', width: 390, scheme: 'light', expectedPaths: candidateHeroPaths, control: 'missing-subset'});
  const subsetRows = missingSubset.graph.filter(row => row.path === '/assets/inter-subset.woff2');
  const fallbackRows = missingSubset.graph.filter(row => row.path === '/assets/inter.woff2');
  const ordinaryRows = missingSubset.graph.filter(row => row.path !== '/assets/inter-subset.woff2');
  const rowOK = row => row.status === 200 && row.finished === true && !row.error && row.cache === false && row.sha256 === row.expectedSHA256 && row.bodyBytes === row.expectedBytes;
  const controlServer = report.serverRequests.filter(row => row.control === 'missing-subset');
  const exactSubset404 = subsetRows.length === 1 && subsetRows[0].status === 404 && missingSubset.assetFailures.some(row => row.path === '/assets/inter-subset.woff2') && controlServer.filter(row => row.path === '/assets/inter-subset.woff2' && row.status === 404 && row.bodyBytes === 0).length === 1;
  const ordinaryServerExact = controlServer.filter(row => row.path !== '/assets/inter-subset.woff2').every(row => row.status === 200);
  const ordinaryExact = candidateHeroPaths.filter(pathname => pathname !== '/assets/inter-subset.woff2').every(pathname => ordinaryRows.filter(row => row.path === pathname).length === 1) && ordinaryRows.every(row => rowOK(row)) && ordinaryServerExact;
  const noOtherAssetFailure = missingSubset.assetFailures.every(row => row.path === '/assets/inter-subset.woff2');
  const paintedFacts = Array.isArray(missingSubset.paintedNodes) && missingSubset.paintedNodes.length > 0;
  const paintedFailures = Array.isArray(missingSubset.paintedFailures) ? missingSubset.paintedFailures : null;
  const fullFontFallback = fallbackRows.length === 1 && rowOK(fallbackRows[0]) && paintedFacts && paintedFailures?.length === 0;
  const systemFallback = paintedFacts && missingSubset.paintedNodes.some(node => Array.isArray(node.fonts) && node.fonts.some(font => font?.isCustomFont === false));
  const faultLaneFailed = missingSubset.ok === false;
  const noGeneralLaneFailure = missingSubset.failures.length === 0;
  report.negativeControl = {lane: missingSubset.id, exactSubset404, ordinaryExact, ordinaryServerExact, noOtherAssetFailure, fullFontFallback, systemFallback, faultLaneFailed, noGeneralLaneFailure, passed: exactSubset404 && ordinaryExact && noOtherAssetFailure && faultLaneFailed && noGeneralLaneFailure && (fullFontFallback || systemFallback)};
  if (!report.negativeControl.passed) report.failures.push({phase: 'negative-missing-subset', error: JSON.stringify(report.negativeControl)});
} catch (error) { report.failures.push({phase: 'execution', error: String(error.stack || error)}); }
finally {
  clearTimeout(watchdog);
  await closeBrowser(); if (server) { server.closeAllConnections(); try { await bounded(new Promise(resolve => server.close(resolve)), 3000, 'HTTP close'); report.serverClosed = !server.listening; } catch (error) { report.failures.push({phase: 'cleanup', error: String(error)}); } }
  report.fidelity ??= {passed: false}; report.typography ??= {passed: false}; report.selection ??= {passed: false}; report.negativeControl ??= {passed: false}; report.performance = {executed: false, status: 'separate runner required; no warm-body result is inferred here'};
  report.passed = report.failures.length === 0 && report.serverClosed === true && report.browserProcess?.exitCode === 0 && report.browserProcess?.signalCode === null && report.fidelity.passed && report.typography.passed && report.selection.passed && report.negativeControl.passed;
  report.finishedAt = new Date().toISOString(); await write(path.join(root, 'font-browser-receipt.json'), JSON.stringify(report, null, 2) + '\n'); console.log(JSON.stringify({out, receipt: path.join(root, 'font-browser-receipt.json'), passed: report.passed, fidelity: report.fidelity.passed, typography: report.typography.passed, selection: report.selection.passed, performance: report.performance.status}));
}
process.exitCode = report.passed ? 0 : 1;
