// Assessment-only prospective empty-selection browser task. No app scripts,
// network, signed-in profile or live Gallery server are used.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {trackOwnedChild, terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';

const [manifestPath, output] = process.argv.slice(2);
assert(manifestPath?.startsWith('/') && output?.startsWith('/'));
const manifestBytes = await fs.readFile(manifestPath);
const manifest = JSON.parse(manifestBytes), I = manifest.inputs;
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const canonical = value => {
  const u = new URL(value, 'http://127.0.0.1');
  u.searchParams.sort();
  return u.pathname + (u.search ? `?${u.searchParams}` : '');
};
const contextKeys = [
  'chart_id', 'title', 'unit', 'grain', 'aggregation', 'window_kind', 'window',
  'source_kind', 'source', 'source_href', 'source_draft', 'evaluated_at',
  'freshness', 'completeness', 'coverage_note', 'uncertainty', 'decimal_places',
];
const dataKeys = ['id', 'label', 'value', 'unit', 'state', 'missing_reason', 'lower', 'upper', 'role', 'role_label'];
const galleryHeader = [
  'series', 'selection_period', 'selection_compare', 'selection_category',
  'selection_state', 'selection_service', ...contextKeys.map(key => `context_${key}`), ...dataKeys,
];
const report = {
  schema: 'appkit-gallery-analytics-empty-browser-v1', startedAt: new Date().toISOString(),
  checks: [], cells: [], errors: [], unexpectedRequests: [], responses: [], downloads: [], acceptedConsoleErrors: [],
  input_hashes_before: {}, input_hashes_after: {}, cleanup: {}, timedOut: false,
};
const check = (ok, name, detail = {}) => {
  report.checks.push({ok: Boolean(ok), name, detail});
  if (!ok) report.errors.push({name, detail});
};
const bounded = async (promise, ms, name) => {
  let timer;
  try {
    return await Promise.race([promise, new Promise((_, reject) => {
      timer = setTimeout(() => reject(Error(`${name} timeout after ${ms}ms`)), ms);
    })]);
  } finally {
    clearTimeout(timer);
  }
};
const parseCSV = text => {
  const rows = [], row = [];
  let cell = '', quoted = false;
  for (let i = 0; i < text.length; i++) {
    const char = text[i];
    if (char === '"') {
      if (quoted && text[i + 1] === '"') { cell += '"'; i++; }
      else quoted = !quoted;
    } else if (!quoted && (char === ',' || char === '\n')) {
      row.push(cell.replace(/\r$/, '')); cell = '';
      if (char === '\n') { rows.push([...row]); row.length = 0; }
    } else cell += char;
  }
  if (cell || row.length) { row.push(cell); rows.push([...row]); }
  assert.equal(quoted, false, 'CSV must not end inside a quoted field');
  return rows;
};
const readInputHashes = async inputs => {
  const hashes = {};
  for (const [name, input] of Object.entries(inputs)) {
    if (!input?.path) continue;
    assert(input.path.startsWith('/') && /^[0-9a-f]{64}$/.test(input.sha256), `${name} input pin`);
    const bytes = await fs.readFile(input.path);
    hashes[name] = {path: input.path, bytes: bytes.length, sha256: sha(bytes)};
    assert.equal(hashes[name].sha256, input.sha256, `${name} input hash`);
  }
  return hashes;
};
const responseBody = (responses, path) => {
  const response = responses.get(canonical(path));
  assert(response, `missing retained response ${path}`);
  return response;
};
const firstHeader = (headers, name) => {
  const value = Object.entries(headers).find(([key]) => key.toLowerCase() === name.toLowerCase())?.[1];
  return Array.isArray(value) ? value[0] : value;
};
const routeHeaders = header => Object.fromEntries(Object.entries(header).map(([key, value]) => [key.toLowerCase(), value.join(', ')]));
const exactURL = (path, expected) => {
  const actual = new URL(path, 'http://127.0.0.1');
  for (const [key, value] of Object.entries(expected)) assert.equal(actual.searchParams.get(key), value, `${key} selection`);
  return actual;
};
const visibleFocus = async link => link.evaluate(element => {
  const box = element.getBoundingClientRect(), style = getComputedStyle(element);
  return {active: element === document.activeElement, width: box.width, height: box.height,
    visible: box.top >= 0 && box.bottom <= innerHeight && box.left >= 0 && box.right <= innerWidth,
    outline: style.outlineStyle, outlineWidth: parseFloat(style.outlineWidth)};
});
const hasNumericTotal = text => /\btotal\s+\d|\b\d+\s+(?:observed\s+)?failed operations\b/i.test(text);

await fs.mkdir(output, {recursive: false, mode: 0o700});
const packet = JSON.parse(await fs.readFile(I.responses.path, 'utf8'));
assert.equal(packet.schema, 'appkit-gallery-analytics-empty-selection-v1');
assert.equal(packet.lanes.length, 2);
const responses = new Map(packet.responses.map(item => [canonical(item.path), item]));
const inputHashBefore = await readInputHashes(I);
report.input_hashes_before = inputHashBefore;
report.manifest_sha256_before = sha(manifestBytes);
assert.equal(fileURLToPath(import.meta.url), I.driver.path);
assert.equal(fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs', import.meta.url)), I.helper.path);
assert.equal(process.execPath, I.node.path);
assert.equal(process.version, manifest.node_version);
const playwrightPackage = JSON.parse(await fs.readFile(new URL('package.json', pathToFileURL(I.pwPath.path)), 'utf8'));
assert.equal(playwrightPackage.version, manifest.pw_version);

let browserServer, browser, owned;
const watchdog = setTimeout(() => {
  report.timedOut = true;
  try { owned?.child.kill('SIGTERM'); } catch {}
}, 120000);

async function observeNavigation(page, path, status, id, action = () => page.goto(`http://127.0.0.1${path}`, {waitUntil: 'load'})) {
  return bounded((async () => {
    const key = canonical(path), expected = responseBody(responses, key);
    const documentResponse = page.waitForResponse(response =>
      response.request().isNavigationRequest() && canonical(response.url()) === key);
    const cssResponse = page.waitForResponse(response => new URL(response.url()).pathname === '/static/app.css');
    const navigation = action();
    const [document, css] = await Promise.all([documentResponse, cssResponse, navigation]);
    await Promise.all([document.finished(), css.finished()]);
    const fonts = await bounded(page.evaluate(() => ({
      status: document.fonts.status,
      pending: [...document.fonts].filter(font => font.status !== 'loaded').length,
    })), 3000, `${id} fonts`);
    check(fonts.status === 'loaded' && fonts.pending === 0, `${id} fonts loaded`, fonts);
    const body = await document.body(), cssBody = await css.body();
    report.responses.push({id, path: key, status: document.status(), bytes: body.length, sha256: sha(body), css_sha256: sha(cssBody)});
    check(document.status() === status && sha(body) === sha(expected.body), `${id} exact document body`, {status: document.status(), bytes: body.length});
    check(sha(cssBody) === sha(responseBody(responses, '/static/app.css').body), `${id} exact completed CSS`, {bytes: cssBody.length});
    check(body.length <= 32768, `${id} document <=32KiB`, {bytes: body.length});
    check(await page.locator('script').count() === 0 && await page.locator('iframe').count() === 0, `${id} no authored script or iframe`);
    check(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${id} no page horizontal overflow`);
    check(firstHeader(expected.header, 'content-security-policy')?.includes("default-src 'none'"), `${id} restrictive CSP`);
    return document;
  })(), 15000, `${id} navigation`);
}

async function nativeEnter(page, link, destination, id) {
  let reached = false;
  for (let i = 0; i < 160; i++) {
    if (await link.evaluate(element => element === document.activeElement)) { reached = true; break; }
    await page.keyboard.press('Tab');
  }
  const geometry = await visibleFocus(link);
  check(reached && geometry.active && geometry.width >= 44 && geometry.height >= 44 &&
    geometry.visible && geometry.outline !== 'none' && geometry.outlineWidth >= 2,
  `${id} native Tab reaches recovery`, {geometry});
  await observeNavigation(page, destination, 200, id, () => page.keyboard.press('Enter'));
  check(canonical(page.url()) === canonical(destination), `${id} native Enter preserves destination`, {url: page.url()});
}

async function assertEmptyOverview(page, lane, paths, id) {
  const main = page.locator('main'), text = await main.textContent();
  check(await page.getByRole('heading', {name: 'No matching services', exact: true}).count() === 1, `${id} explicit no-results heading`);
  check(await page.locator('#analytics-value-table').count() === 0 &&
    await page.locator('section[aria-labelledby="analytics-comparison"] li').count() === 0 &&
    !/shared scale 0 to/i.test(text) && !hasNumericTotal(text), `${id} no rows or fabricated total`);
  const recovery = page.getByRole('link', {name: 'Show all service groups', exact: true});
  const exportLink = page.getByRole('link', {name: 'Export CSV (headers only)', exact: true});
  check(await recovery.count() === 1 && await recovery.getAttribute('href') === paths.recoveredOverview, `${id} recovery preserves selection except category`);
  check(await exportLink.count() === 1 && await exportLink.getAttribute('href') === paths.emptyExport, `${id} overview export keeps empty selection`);
  const contextLink = page.getByRole('link', {name: 'Export current-period context CSV', exact: true});
  check(await contextLink.count() === 1 && await contextLink.getAttribute('href') === paths.contextCSV,
    `${id} overview context export keeps current-period empty selection`);
  check(text.includes(`category=empty`) || text.includes('Empty group'), `${id} empty category remains explained`);
  return {recovery, exportLink, contextLink};
}

async function assertRecoveredOverview(page, lane, paths, id) {
  const text = await page.locator('main').textContent();
  const rows = await page.locator('#analytics-value-table tbody tr').count();
  check(rows > 0 && await page.locator('section[aria-labelledby="analytics-comparison"] li').count() > 0,
    `${id} recovered overview restores rows`, {rows});
  check(!/No matching services/.test(text), `${id} recovered overview is not empty`, {rows});
  exactURL(page.url(), {period: lane.period, compare: lane.compare, category: 'all', state: lane.state, theme: lane.theme});
  check(canonical(page.url()) === canonical(paths.recoveredOverview), `${id} recovered URL preserves non-category selection`);
}

async function assertEmptyRecipe(page, lane, paths, id) {
  const text = await page.locator('main').textContent();
  check(await page.getByRole('heading', {name: 'No matching services', exact: true}).count() === 1, `${id} recipe no-results heading`);
  check(await page.locator('.ak-vizframe tbody tr').count() === 1 &&
    (await page.locator('.ak-vizframe tbody tr').textContent())?.trim() === 'No rows in this selection.' &&
    await page.locator('.ak-vizframe svg').count() === 0 && !hasNumericTotal(text), `${id} recipe has no rows or total`);
  const recovery = page.getByRole('link', {name: 'Show all service groups', exact: true});
  const contextLink = page.getByRole('link', {name: 'Export renderer CSV', exact: true});
  const tableLink = page.getByRole('link', {name: 'Export spreadsheet CSV', exact: true});
  check(await recovery.count() === 1 && await recovery.getAttribute('href') === paths.recoveredRecipe, `${id} recipe recovery preserves selection except category`);
  check(await contextLink.count() === 1 && await contextLink.getAttribute('href') === paths.contextCSV, `${id} renderer context export keeps empty selection`);
  check(await tableLink.count() === 1 && await tableLink.getAttribute('href') === paths.headerOnlyTableCSV, `${id} table export keeps empty selection`);
  check(text.includes(`category=empty`) || text.includes('Empty group'), `${id} recipe empty category remains explained`);
  return {recovery, contextLink, tableLink};
}

function assertHeaderOnlyGalleryCSV(bytes, id) {
  const rows = parseCSV(bytes.toString('utf8'));
  check(rows.length === 1 && JSON.stringify(rows[0]) === JSON.stringify(galleryHeader), `${id} Gallery export is exactly 33-column header-only`, {rows});
}

function assertContextCSV(bytes, lane, id) {
  const rows = parseCSV(bytes.toString('utf8'));
  const values = new Map(rows.slice(0, contextKeys.length).map(row => [row[0], row[1]]));
  const headerIndex = rows.findIndex(row => JSON.stringify(row) === JSON.stringify(dataKeys));
  check(headerIndex === contextKeys.length + 1 && JSON.stringify(rows[contextKeys.length]) === JSON.stringify(['']) &&
    rows.length === headerIndex + 1, `${id} renderer CSV retains context and no data rows`, {rows: rows.length});
  check(values.get('source')?.includes(`period=${lane.period}`) && values.get('source')?.includes(`category=empty`) &&
    values.get('source')?.includes(`state=${lane.state}`) && values.get('window')?.includes(`${lane.period}-01T00:00:00Z`),
  `${id} renderer CSV retains category/state/window context`, {source: values.get('source'), window: values.get('window')});
}

function assertHeaderOnlyTableCSV(bytes, id) {
  const rows = parseCSV(bytes.toString('utf8'));
  check(rows.length === 1 && rows[0].length === 27 && rows[0].every((value, index) =>
    value === [...contextKeys.map(key => `context_${key}`), ...dataKeys][index]), `${id} table CSV is header-only with 27 columns`, {rows});
}

async function download(page, link, expectedPath, id, validate) {
  let reached = false;
  for (let i = 0; i < 160; i++) {
    if (await link.evaluate(element => element === document.activeElement)) { reached = true; break; }
    await page.keyboard.press('Tab');
  }
  const geometry = await visibleFocus(link);
  check(reached && geometry.active && geometry.width >= 44 && geometry.height >= 44 && geometry.outline !== 'none', `${id} download focus`, {geometry});
  const event = page.waitForEvent('download');
  await page.keyboard.press('Enter');
  const item = await bounded(event, 10000, `${id} download`);
  const path = `${output}/${id}.csv`;
  await item.saveAs(path);
  const bytes = await fs.readFile(path), expected = responseBody(responses, expectedPath);
  check(!await item.failure() && sha(bytes) === sha(expected.body), `${id} exact native download`, {bytes: bytes.length, sha256: sha(bytes)});
  report.downloads.push({id, path, bytes: bytes.length, sha256: sha(bytes)});
  validate(bytes);
}

async function runOverviewCell(lane) {
  const paths = lane.paths, id = `overview-${lane.name}`;
  let context;
  try {
    context = await browser.newContext({viewport: {width: lane.width, height: lane.height}, colorScheme: lane.theme,
      javaScriptEnabled: false, serviceWorkers: 'block', acceptDownloads: true});
    context.setDefaultTimeout(5000);
    const requests = [];
    await context.route('**/*', route => {
      const request = route.request(), url = new URL(request.url()), body = responses.get(canonical(request.url()));
      const allowed = request.method() === 'GET' && url.origin === 'http://127.0.0.1' && Boolean(body);
      requests.push({method: request.method(), url: request.url(), allowed});
      if (!allowed) { report.unexpectedRequests.push({id, url: request.url()}); return route.abort('blockedbyclient'); }
      return route.fulfill({status: body.status, headers: routeHeaders(body.header), body: body.body});
    });
    const page = await context.newPage();
    let expected404Console = false, pending404Console = [];
    page.on('pageerror', error => report.errors.push({name: `${id} pageerror`, detail: String(error)}));
    page.on('console', message => {
      if (message.type() !== 'error') return;
      const location = message.location();
      const expected404 = message.text() === 'Failed to load resource: the server responded with a status of 404 (Not Found)' &&
        canonical(location.url) === canonical(paths.emptyDetail);
      if (expected404Console && expected404) pending404Console.push({text: message.text(), location: location.url});
      else report.errors.push({name: `${id} console`, detail: {text: message.text(), location: location.url}});
    });
    page.on('websocket', socket => report.unexpectedRequests.push({id, websocket: socket.url()}));
    await observeNavigation(page, paths.emptyOverview, 200, id);
    const initial = await assertEmptyOverview(page, lane, paths, id);
    await page.screenshot({path: `${output}/${id}-empty.png`, fullPage: true});
    expected404Console = true;
    try {
      await observeNavigation(page, paths.emptyDetail, 404, `${id}-detail404`);
    } finally {
      expected404Console = false;
    }
    const detailEvidence = report.responses.find(response => response.id === `${id}-detail404` &&
      response.path === canonical(paths.emptyDetail) && response.status === 404);
    check(Boolean(detailEvidence), `${id} accepted 404 console has exact response evidence`, detailEvidence ?? {path: canonical(paths.emptyDetail), status: 404});
    for (const detail of pending404Console) report.acceptedConsoleErrors.push({id, detail, response: detailEvidence});
    check(await page.getByRole('link', {name: 'Return to a valid report', exact: true}).count() === 1 &&
      await page.getByRole('link', {name: 'Return to a valid report', exact: true}).getAttribute('href') === paths.emptyOverview,
    `${id} empty detail remains 404 with recovery`);
    await page.goBack({waitUntil: 'load'});
    check(canonical(page.url()) === canonical(paths.emptyOverview), `${id} detail Back restores empty overview`);
    const recovered = await nativeEnter(page, initial.recovery, paths.recoveredOverview, id);
    await assertRecoveredOverview(page, lane, paths, id);
    await page.screenshot({path: `${output}/${id}-recovered.png`, fullPage: true});
    await page.goBack({waitUntil: 'load'});
    check(canonical(page.url()) === canonical(paths.emptyOverview), `${id} browser Back restores exact empty selection/theme`);
    await download(page, page.getByRole('link', {name: 'Export CSV (headers only)', exact: true}), paths.emptyExport, `${id}-empty-export`, bytes => assertHeaderOnlyGalleryCSV(bytes, `${id} empty export`));
    await download(page, initial.contextLink, paths.contextCSV, `${id}-context-export`, bytes => assertContextCSV(bytes, lane, `${id} context export`));
    check(requests.every(item => item.allowed), `${id} all browser requests are retained same-origin GETs`, {requests});
    report.cells.push({id, lane: lane.name, requests});
  } finally {
    if (context) await bounded(context.close(), 5000, `${id} context close`);
  }
}

async function runRecipeCell(lane) {
  const paths = lane.paths, id = `recipe-${lane.name}`;
  let context;
  try {
    context = await browser.newContext({viewport: {width: lane.width, height: lane.height}, colorScheme: lane.theme,
      javaScriptEnabled: false, serviceWorkers: 'block', acceptDownloads: true});
    context.setDefaultTimeout(5000);
    const requests = [];
    await context.route('**/*', route => {
      const request = route.request(), url = new URL(request.url()), body = responses.get(canonical(request.url()));
      const allowed = request.method() === 'GET' && url.origin === 'http://127.0.0.1' && Boolean(body);
      requests.push({method: request.method(), url: request.url(), allowed});
      if (!allowed) { report.unexpectedRequests.push({id, url: request.url()}); return route.abort('blockedbyclient'); }
      return route.fulfill({status: body.status, headers: routeHeaders(body.header), body: body.body});
    });
    const page = await context.newPage();
    page.on('pageerror', error => report.errors.push({name: `${id} pageerror`, detail: String(error)}));
    page.on('console', message => { if (message.type() === 'error') report.errors.push({name: `${id} console`, detail: message.text()}); });
    page.on('websocket', socket => report.unexpectedRequests.push({id, websocket: socket.url()}));
    await observeNavigation(page, paths.emptyRecipe, 200, id);
    const initial = await assertEmptyRecipe(page, lane, paths, id);
    await page.screenshot({path: `${output}/${id}-empty.png`, fullPage: true});
    await nativeEnter(page, initial.recovery, paths.recoveredRecipe, id);
    check(await page.locator('.ak-vizframe tbody tr').count() > 0, `${id} recovered recipe restores rows`);
    await page.screenshot({path: `${output}/${id}-recovered.png`, fullPage: true});
    await page.goBack({waitUntil: 'load'});
    check(canonical(page.url()) === canonical(paths.emptyRecipe), `${id} browser Back restores exact empty recipe/theme`);
    await download(page, page.getByRole('link', {name: 'Export renderer CSV', exact: true}), paths.contextCSV, `${id}-recipe-context-export`, bytes => assertContextCSV(bytes, lane, `${id} context export`));
    await download(page, page.getByRole('link', {name: 'Export spreadsheet CSV', exact: true}), paths.headerOnlyTableCSV, `${id}-table-export`, bytes => assertHeaderOnlyTableCSV(bytes, `${id} table export`));
    check(requests.every(item => item.allowed), `${id} all browser requests are retained same-origin GETs`, {requests});
    report.cells.push({id, lane: lane.name, requests});
  } finally {
    if (context) await bounded(context.close(), 5000, `${id} context close`);
  }
}

try {
  const {chromium} = await import(pathToFileURL(I.pwPath.path).href);
  browserServer = await bounded(chromium.launchServer({executablePath: I.chromePath.path, headless: true,
    chromiumSandbox: true, timeout: 15000, args: ['--no-proxy-server', '--disable-background-networking']}), 15000, 'Chrome launch');
  owned = trackOwnedChild(browserServer.process(), {name: 'chrome'});
  browser = await bounded(chromium.connect(browserServer.wsEndpoint(), {timeout: 10000}), 10000, 'Chrome connect');
  report.chromeVersion = browser.version();
  assert.equal(report.chromeVersion, manifest.chrome_version);
  for (const lane of packet.lanes) {
    await bounded(runOverviewCell(lane), 25000, `overview-${lane.name} lane`);
    await bounded(runRecipeCell(lane), 25000, `recipe-${lane.name} lane`);
  }
} catch (error) {
  report.errors.push({name: 'run error', detail: String(error.stack || error)});
} finally {
  try { if (browser) await bounded(browser.close(), 5000, 'browser close'); }
  catch (error) { report.cleanup.browserCloseError = String(error); }
  try { if (browserServer) await bounded(browserServer.close(), 5000, 'browser server close'); }
  catch (error) { report.cleanup.browserServerCloseError = String(error); }
  if (owned) {
    try { report.cleanup.chrome = await terminateOwned(owned, {graceMs: 3000, killMs: 2000, drainMs: 3000}); }
    catch (error) { report.cleanup.chromeError = String(error); }
  }
  clearTimeout(watchdog);
  try {
    report.input_hashes_after = await readInputHashes(I);
    report.manifest_sha256_after = sha(await fs.readFile(manifestPath));
    check(JSON.stringify(report.input_hashes_before) === JSON.stringify(report.input_hashes_after), 'input manifest files unchanged during run');
    check(report.manifest_sha256_before === report.manifest_sha256_after, 'manifest unchanged during run');
  } catch (error) { report.errors.push({name: 'input hash after', detail: String(error)}); }
  report.endedAt = new Date().toISOString();
  report.summary = {checks: report.checks.length, failures: report.errors.length, cells: report.cells.length, downloads: report.downloads.length, acceptedConsoleErrors: report.acceptedConsoleErrors.length};
  report.pass = !report.timedOut && report.errors.length === 0 && report.unexpectedRequests.length === 0 &&
    report.cells.length === 4 && report.downloads.length === 8 && report.cleanup.chrome?.ok === true &&
    !report.cleanup.browserCloseError && !report.cleanup.browserServerCloseError;
  await fs.writeFile(`${output}/report.json`, JSON.stringify(report, null, 2) + '\n', {flag: 'wx'});
  console.log(JSON.stringify({pass: report.pass, summary: report.summary, cleanup: report.cleanup.chrome?.termination}));
  process.exitCode = report.pass ? 0 : 1;
}
