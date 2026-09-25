// Semantic A/B preparation probe for the analytics usability protocol.
// This is browser evidence for a fixed synthetic task, never human task-time data.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';

if (process.argv.length !== 7) {
  throw Error('usage: node verify-pair.mjs A.json B.json playwright/index.mjs chrome-executable fresh-output-parent');
}

const [artifactAPath, artifactBPath, playwrightPath, chromePath, outputParentPath] = process.argv.slice(2);
const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const shaFile = file => sha(fs.readFileSync(file));
const isSHA256 = value => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const canonicalFile = (file, label) => {
  const resolved = fs.realpathSync(file);
  if (!fs.statSync(resolved).isFile()) throw Error(`${label} must be a regular file`);
  return resolved;
};
const readArtifact = (file, expectedLabel) => {
  const resolved = canonicalFile(file, `${expectedLabel} JSON`);
  const bytes = fs.readFileSync(resolved);
  const value = JSON.parse(bytes);
  if (value.schema !== 'analytics-usability-artifact-v1' || value.label !== expectedLabel) {
    throw Error(`${expectedLabel} JSON has the wrong schema or label`);
  }
  if (value.expectedPlaywrightVersion !== '1.62.1') {
    throw Error(`${expectedLabel} expectedPlaywrightVersion must be exactly 1.62.1`);
  }
  const origin = new URL(value.origin);
  if (origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || !origin.port ||
      origin.username || origin.password || origin.pathname !== '/' || origin.search || origin.hash) {
    throw Error(`${expectedLabel} requires a bare HTTP 127.0.0.1 origin`);
  }
  for (const [name, item] of Object.entries({ binary: value.binary, css: value.css })) {
    if (!item || typeof item.path !== 'string' || !isSHA256(item.sha256)) {
      throw Error(`${expectedLabel} ${name} requires path and lowercase SHA-256`);
    }
  }
  return { resolved, bytes, value, origin: origin.origin };
};

const artifacts = [readArtifact(artifactAPath, 'A'), readArtifact(artifactBPath, 'B')];
if (artifacts[0].origin === artifacts[1].origin) throw Error('A and B require distinct loopback origins');
const resolvedPlaywright = canonicalFile(playwrightPath, 'Playwright entry');
const playwrightPackagePath = canonicalFile(path.join(path.dirname(resolvedPlaywright), 'package.json'), 'Playwright package');
const playwrightPackageBytes = fs.readFileSync(playwrightPackagePath);
const playwrightPackage = JSON.parse(playwrightPackageBytes);
if (artifacts[0].value.expectedPlaywrightVersion !== artifacts[1].value.expectedPlaywrightVersion ||
    path.basename(resolvedPlaywright) !== 'index.mjs' || playwrightPackage.name !== 'playwright' ||
    playwrightPackage.version !== artifacts[0].value.expectedPlaywrightVersion) {
  throw Error('Artifact configs and the approved Playwright package must bind the same exact version');
}
const resolvedChrome = canonicalFile(chromePath, 'Chrome executable');
const outputParent = fs.realpathSync(outputParentPath);
if (!fs.statSync(outputParent).isDirectory() || fs.readdirSync(outputParent).length !== 0) {
  throw Error('Output parent must be an existing empty directory');
}
if (/\s/.test(outputParent)) throw Error('Output parent must not contain whitespace; browser ownership parsing is intentionally strict');
const out = fs.mkdtempSync(path.join(outputParent, 'analytics-usability-pair-'));
fs.chmodSync(out, 0o700);

const harnessPath = canonicalFile(new URL(import.meta.url), 'Harness');
const harnessBytes = fs.readFileSync(harnessPath);
const put = (relative, bytes, mode = 0o600) => {
  const target = path.join(out, relative);
  fs.mkdirSync(path.dirname(target), { recursive: true, mode: 0o700 });
  fs.writeFileSync(target, bytes, { flag: 'wx', mode });
  return target;
};
put('frozen/verify-pair.mjs', harnessBytes);
for (const artifact of artifacts) put(`frozen/artifact-${artifact.value.label}.json`, artifact.bytes);

const report = {
  schema: 'analytics-usability-pair-report-v1',
  startedAt: new Date().toISOString(),
  passed: false,
  scope: 'Automated semantic preparation for the same synthetic participant task; not a usability trial.',
  limits: [
    'No human task time, participant error rate, speedup, population, accessibility, or product-wide quality claim.',
    'Navigation timing is diagnostic only and has no pass threshold.',
    'Target heights are recorded without a pass gate; retained baseline target failures are not reclassified.',
    'Synthetic loopback GET traffic only; no operational or personal data.',
  ],
  oracle: {
    selection: { period: '2026-08', compare: 'none', category: 'api', state: 'partial' },
    values: { 'gateway-api': 'At least 4', 'search-api': 'At least 2' },
    interpretation: 'partial lower bounds, not complete-month totals or reliability rates',
    detail: 'gateway-api', returnSelectionCount: 4,
    csv: { columns: 33, rows: 2, values: ['gateway-api:4', 'search-api:2'] },
  },
  harness: { path: 'frozen/verify-pair.mjs', sha256: sha(harnessBytes) },
  playwright: {
    path: resolvedPlaywright, sha256: shaFile(resolvedPlaywright), version: playwrightPackage.version,
    packagePath: playwrightPackagePath, packageSHA256: sha(playwrightPackageBytes),
  },
  chrome: { path: resolvedChrome, sha256: shaFile(resolvedChrome) },
  host: { platform: os.platform(), arch: os.arch(), node: process.version },
  artifacts: [], lanes: [], failures: [], blockedRequests: [], captures: [], cleanup: [],
};

const persist = () => {
  const target = path.join(out, 'receipt.json');
  const temporary = `${target}.tmp`;
  fs.writeFileSync(temporary, JSON.stringify(report, null, 2) + '\n', { mode: 0o600 });
  fs.renameSync(temporary, target);
};
const fail = (scope, detail) => { report.failures.push({ scope, detail }); persist(); };
const check = (condition, scope, detail) => {
  if (!condition) fail(scope, detail);
  return condition;
};

for (const artifact of artifacts) {
  const facts = {
    label: artifact.value.label,
    config: { path: `frozen/artifact-${artifact.value.label}.json`, sha256: sha(artifact.bytes) },
    origin: artifact.origin,
  };
  try {
    for (const name of ['binary', 'css']) {
      const selected = artifact.value[name];
      const resolved = canonicalFile(selected.path, `${artifact.value.label} ${name}`);
      const actual = shaFile(resolved);
      facts[name] = { path: resolved, expectedSHA256: selected.sha256, actualSHA256: actual, matched: actual === selected.sha256 };
      check(actual === selected.sha256, `${artifact.value.label}: ${name} identity`, facts[name]);
    }
  } catch (error) {
    fail(`${artifact.value.label}: artifact preflight`, error.stack ?? String(error));
  }
  report.artifacts.push(facts);
}
persist(); // All source, configuration, artifact and tool identities precede launch.

let chromium;
try {
  ({ chromium } = await import(pathToFileURL(resolvedPlaywright).href));
} catch (error) {
  fail('Playwright import', error.stack ?? String(error));
  report.finishedAt = new Date().toISOString();
  persist();
  console.log(JSON.stringify({ out, passed: false, lanes: 0, failures: report.failures.length, receiptSHA256: shaFile(path.join(out, 'receipt.json')) }));
  process.exit(1);
}
const lanes = [
  { width: 390, height: 1000, theme: 'light' },
  { width: 390, height: 1000, theme: 'dark' },
  { width: 1440, height: 1000, theme: 'light' },
  { width: 1440, height: 1000, theme: 'dark' },
];
const allowedPaths = new Set(['/analytics', '/analytics/gateway-api', '/analytics/export.csv', '/static/app.css']);
const selectionKeys = new Set(['period', 'compare', 'category', 'state', 'theme']);
// Fixed contract, independently declared before observing either artifact.
const expectedCSVHeader = [
  'series', 'selection_period', 'selection_compare', 'selection_category', 'selection_state', 'selection_service',
  ...['chart_id', 'title', 'unit', 'grain', 'aggregation', 'window_kind', 'window', 'source_kind', 'source',
    'source_href', 'source_draft', 'evaluated_at', 'freshness', 'completeness', 'coverage_note', 'uncertainty', 'decimal_places'].map(key => `context_${key}`),
  'id', 'label', 'value', 'unit', 'state', 'missing_reason', 'lower', 'upper', 'role', 'role_label',
];
const expectedWindow = '2026-08-01T00:00:00Z to 2026-09-01T00:00:00Z (end exclusive; UTC)';

function allowedURL(url, origin) {
  let parsed;
  try { parsed = new URL(url); } catch { return false; }
  if (parsed.origin !== origin || !allowedPaths.has(parsed.pathname) || parsed.hash) return false;
  if (parsed.pathname === '/static/app.css') return parsed.search === '';
  for (const key of parsed.searchParams.keys()) if (!selectionKeys.has(key)) return false;
  for (const key of selectionKeys) if (parsed.searchParams.getAll(key).length > 1) return false;
  return true;
}

function parseCSV(text) {
  const rows = []; let row = [], field = '', quoted = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quoted) {
      if (c === '"' && text[i + 1] === '"') { field += '"'; i++; }
      else if (c === '"') quoted = false;
      else field += c;
    } else if (c === '"') quoted = true;
    else if (c === ',') { row.push(field); field = ''; }
    else if (c === '\n') { row.push(field); rows.push(row); row = []; field = ''; }
    else if (c !== '\r') field += c;
  }
  if (quoted) throw Error('CSV contains an unclosed quote');
  if (field || row.length) { row.push(field); rows.push(row); }
  const [header, ...values] = rows;
  if (!header) throw Error('CSV has no header');
  return { header, values, records: values.map(cells => Object.fromEntries(header.map((key, index) => [key, cells[index]]))) };
}

function validateOverviewCSV(csv) {
  if (JSON.stringify(csv.header) !== JSON.stringify(expectedCSVHeader) || csv.values.length !== 2 ||
      csv.values.some(row => row.length !== 33)) throw Error('Overview CSV differs from the ordered 33-column/two-row contract');
  const values = csv.records.map(record => `${record.id}:${record.value}`);
  if (JSON.stringify(values) !== JSON.stringify(report.oracle.csv.values) || csv.records.some(record =>
    record.series !== 'current' || record.state !== 'present' || record.selection_period !== '2026-08' ||
    record.selection_compare !== 'none' || record.selection_category !== 'api' || record.selection_state !== 'partial' ||
    record.selection_service !== '' || record.context_completeness !== 'partial' || record.context_window !== expectedWindow)) {
    throw Error(`CSV task mismatch: ${JSON.stringify(csv.records)}`);
  }
  return values;
}

// Exercise the actual oracle with synthetic mutations before opening Chrome.
const oracleRows = ['gateway-api', 'search-api'].map((id, i) => ({
  id, value: String([4, 2][i]), series: 'current', state: 'present', selection_period: '2026-08',
  selection_compare: 'none', selection_category: 'api', selection_state: 'partial', selection_service: '',
  context_completeness: 'partial', context_window: expectedWindow,
}));
const csvFixture = () => ({header: [...expectedCSVHeader], records: structuredClone(oracleRows), values: oracleRows.map(row => expectedCSVHeader.map(key => row[key] ?? ''))});
report.oracleControls = [];
try {
  validateOverviewCSV(csvFixture());
  for (const [name, mutate] of [
    ['header rename', csv => { csv.header[7] = 'renamed_title'; }],
    ['header reorder', csv => { [csv.header[7], csv.header[8]] = [csv.header[8], csv.header[7]]; }],
    ['wrong window', csv => { csv.records[0].context_window = 'wrong month'; }],
    ['wrong value', csv => { csv.records[0].value = '40'; }],
    ['missing row', csv => { csv.records.pop(); csv.values.pop(); }],
  ]) {
    const csv = csvFixture(); mutate(csv); let refused = false;
    try { validateOverviewCSV(csv); } catch { refused = true; }
    report.oracleControls.push({name, refused});
    check(refused, 'oracle control: ' + name, 'Expected mutated fixture refusal');
  }
} catch (error) { fail('oracle positive control', String(error)); }
persist();

async function press(page, lane, key, navigation = null) {
  await page.keyboard.press(key);
  // A native Enter may replace the document. Settle its pre-armed URL waiter
  // before inspecting focus; do not mistake a destroyed old context for a UI failure.
  if (navigation) await navigation;
  lane.keys.push({ key, active: await page.evaluate(() => {
    const element = document.activeElement;
    return { tag: element?.tagName, id: element?.id, name: element?.getAttribute('name'), value: element?.value, href: element?.getAttribute('href') };
  }) });
}

// Attach a rejection observer before the keyboard action so an action failure cannot
// leave a navigation/download waiter rejected without a handler.
function observed(promise) {
  promise.catch(() => {});
  return promise;
}

async function tabTo(page, lane, selector, backwards = false) {
  for (let index = 0; index < 60; index++) {
    if (await page.evaluate(value => document.activeElement?.matches(value), selector)) {
      const focus = await page.evaluate(() => {
        const element = document.activeElement, style = getComputedStyle(element), box = element.getBoundingClientRect();
        return { selector: element.matches(':focus-visible'), outlineStyle: style.outlineStyle, outlineWidth: style.outlineWidth, top: box.top, bottom: box.bottom };
      });
      if (!focus.selector || focus.outlineStyle === 'none' || parseFloat(focus.outlineWidth) <= 0) throw Error(`No painted keyboard focus on ${selector}`);
      lane.focus.push({ requested: selector, ...focus });
      return;
    }
    await press(page, lane, backwards ? 'Shift+Tab' : 'Tab');
  }
  throw Error(`Keyboard traversal did not reach ${selector}`);
}

async function skipToMain(page, lane) {
  await tabTo(page, lane, 'a[href="#gallery-main"]');
  await press(page, lane, 'Enter');
  if (!await page.evaluate(() => document.activeElement?.id === 'gallery-main')) throw Error('Skip link did not move focus to gallery-main');
}

async function capture(page, artifact, lane, stage) {
  const name = `${artifact.value.label}-${lane.width}-${lane.theme}-${stage}.png`;
  const target = path.join(out, 'captures', name);
  fs.mkdirSync(path.dirname(target), { recursive: true, mode: 0o700 });
  await page.screenshot({ path: target, fullPage: true });
  const bytes = fs.readFileSync(target);
  const item = { artifact: artifact.value.label, lane: `${lane.width}-${lane.theme}`, stage, file: `captures/${name}`, bytes: bytes.length, sha256: sha(bytes) };
  report.captures.push(item);
  return item;
}

async function processIdentity(context, profile) {
  const browser = context.browser();
  if (!browser) throw Error('Persistent context has no owning browser');
  const cdp = await browser.newBrowserCDPSession();
  let info;
  try { info = await cdp.send('SystemInfo.getProcessInfo'); } finally { await cdp.detach(); }
  const browsers = info.processInfo.filter(item => item.type === 'browser' && Number.isInteger(item.id) && item.id > 1);
  if (browsers.length !== 1) throw Error(`Expected one browser PID, found ${browsers.length}`);
  const pid = browsers[0].id;
  const ps = spawnSync('/bin/ps', ['-ww', '-p', String(pid), '-o', 'pid=,pgid=,lstart=,command='], { encoding: 'utf8', timeout: 2000, maxBuffer: 1024 * 1024 });
  if (ps.error || ps.status !== 0 || !ps.stdout.trim()) throw Error(`Could not bind browser PID ${pid}`);
  const command = ps.stdout.trim();
  const matches = [...command.matchAll(/(?:^|\s)--user-data-dir=([^\s]+)/g)].map(match => match[1]);
  if (matches.length !== 1 || fs.realpathSync(matches[0]) !== fs.realpathSync(profile)) throw Error('Browser command does not bind the owned profile exactly once');
  return { pid, profile, psIdentity: command };
}

const pidState = pid => {
  const result = spawnSync('/bin/ps', ['-p', String(pid), '-o', 'pid='], { encoding: 'utf8', timeout: 2000, maxBuffer: 1024 });
  const output = result.stdout?.trim() ?? '';
  if (result.error) return { state: 'unknown', error: String(result.error), status: result.status, output };
  if (result.status === 1 && output === '' && !result.stderr?.trim()) return { state: 'absent', status: result.status, output };
  if (result.status === 0 && output === String(pid)) return { state: 'present', status: result.status, output };
  return { state: 'unknown', status: result.status, signal: result.signal, stderr: result.stderr?.trim(), output };
};
const waitForExit = async (pid, timeoutMs) => {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const observed = pidState(pid);
    if (observed.state === 'absent') return { exited: true, observed };
    if (observed.state === 'unknown') return { exited: false, observed };
    await new Promise(resolve => setTimeout(resolve, 50));
  }
  const observed = pidState(pid);
  return { exited: observed.state === 'absent', observed };
};

async function runLane(artifact, dimensions) {
  const lane = {
    artifact: artifact.value.label, ...dimensions, passed: false, keys: [], focus: [], requests: [],
    responses: [], navigationTimings: [], targetHeights: [], semanticFacts: {}, failures: [], cleanup: {},
    targetGate: artifact.value.label === 'A' ? 'not-run; known baseline quality failures retained' : 'not-run; semantic pair probe only',
  };
  report.lanes.push(lane); persist();
  const profile = fs.mkdtempSync(path.join(out, `profile-${artifact.value.label}-${dimensions.width}-${dimensions.theme}-`));
  fs.chmodSync(profile, 0o700);
  let context, identity;
  const pending = [];
  try {
    context = await chromium.launchPersistentContext(profile, {
      executablePath: resolvedChrome, headless: true,
      viewport: { width: dimensions.width, height: dimensions.height }, colorScheme: dimensions.theme,
      reducedMotion: 'reduce', acceptDownloads: true, serviceWorkers: 'block',
      args: ['--no-first-run', '--no-default-browser-check'],
    });
    context.setDefaultTimeout(7000);
    identity = await processIdentity(context, profile);
    lane.browser = { ...identity, version: context.browser().version() };
    put(`process/${artifact.value.label}-${dimensions.width}-${dimensions.theme}.json`, JSON.stringify(lane.browser, null, 2) + '\n');
    await context.route('**/*', async route => {
      const request = route.request();
      const fact = { url: request.url(), method: request.method(), resourceType: request.resourceType() };
      lane.requests.push(fact);
      if (request.method() === 'GET' && allowedURL(request.url(), artifact.origin)) return route.continue();
      report.blockedRequests.push({ artifact: artifact.value.label, lane: `${dimensions.width}-${dimensions.theme}`, ...fact });
      await route.abort('blockedbyclient');
    });
    await context.routeWebSocket('**/*', socket => {
      report.blockedRequests.push({ artifact: artifact.value.label, lane: `${dimensions.width}-${dimensions.theme}`, websocket: socket.url() });
      socket.close();
    });
    const page = context.pages()[0] ?? await context.newPage();
    const cdp = await context.newCDPSession(page);
    await cdp.send('Network.enable');
    await cdp.send('Network.setCacheDisabled', { cacheDisabled: true });
    await cdp.send('Emulation.setScriptExecutionDisabled', { value: true });
    await cdp.detach();
    page.on('pageerror', error => lane.failures.push({ scope: 'pageerror', detail: error.message }));
    page.on('console', message => { if (message.type() === 'error') lane.failures.push({ scope: 'console', detail: message.text() }); });
    page.on('response', response => pending.push((async () => {
      if (response.headers()['content-disposition']?.includes('attachment')) {
        lane.responses.push({ url: response.url(), status: response.status(), attachment: true });
        if (response.status() !== 200) throw Error(`Unexpected download response status: ${response.status()}`);
        return;
      }
      const body = await response.body();
      const item = { url: response.url(), status: response.status(), bytes: body.length, sha256: sha(body), contentType: response.headers()['content-type'] };
      lane.responses.push(item);
      if (new URL(response.url()).pathname === '/static/app.css') {
        if (item.status !== 200 || item.sha256 !== artifact.value.css.sha256) throw Error(`Served CSS mismatch: ${JSON.stringify(item)}`);
      } else if (item.status !== 200) throw Error(`Unexpected response status: ${JSON.stringify(item)}`);
    })().catch(error => lane.failures.push({ scope: 'response', detail: error.stack ?? String(error) }))));
    const recordTiming = async stage => lane.navigationTimings.push({ stage, ...(await page.evaluate(() => {
      const entry = performance.getEntriesByType('navigation')[0];
      return entry ? { responseEndMs: entry.responseEnd, domContentLoadedMs: entry.domContentLoadedEventEnd, loadEndMs: entry.loadEventEnd } : { missing: true };
    })) });

    await page.goto(`${artifact.origin}/analytics?theme=${dimensions.theme}`, { waitUntil: 'load' });
    await recordTiming('initial');
    await skipToMain(page, lane);
    for (const [field, letters, expected] of [['period', 'a', '2026-08'], ['compare', 'n', 'none'], ['category', 'api', 'api']]) {
      await tabTo(page, lane, `select[name=${field}]`);
      for (const letter of letters) await press(page, lane, letter);
      if (await page.locator(`select[name=${field}]`).inputValue() !== expected) throw Error(`Native ${field} selection did not reach ${expected}`);
    }
    await tabTo(page, lane, '#analytics-filter details summary');
    await press(page, lane, 'Enter');
    await tabTo(page, lane, 'select[name=state]');
    await press(page, lane, 'p');
    if (await page.locator('select[name=state]').inputValue() !== 'partial') throw Error('Native state selection did not reach partial');
    await tabTo(page, lane, '#analytics-filter button[type=submit]', true);
    const selectedNavigation = observed(page.waitForURL(url => url.pathname === '/analytics' &&
      url.searchParams.get('period') === '2026-08' && url.searchParams.get('compare') === 'none' &&
      url.searchParams.get('category') === 'api' && url.searchParams.get('state') === 'partial' &&
      url.searchParams.get('theme') === dimensions.theme));
    await press(page, lane, 'Enter', selectedNavigation);
    await recordTiming('selected-overview');

    const expectedIDs = Object.keys(report.oracle.values).sort();
    const overviewIDs = await page.locator('[data-value-id]').evaluateAll(rows => rows.map(row => row.getAttribute('data-value-id')).sort());
    if (JSON.stringify(overviewIDs) !== JSON.stringify(expectedIDs)) throw Error(`Overview contains the wrong service set: ${JSON.stringify(overviewIDs)}`);
    const valueFacts = {};
    for (const [id, expected] of Object.entries(report.oracle.values)) {
      const row = page.locator(`[data-value-id="${id}"]`);
      valueFacts[id] = { value: (await row.locator('td').first().innerText()).trim(), state: (await row.locator('td').last().innerText()).trim() };
      if (valueFacts[id].value !== expected || valueFacts[id].state !== 'Partial, lower bound') throw Error(`Wrong independent lower-bound fact for ${id}`);
    }
    const explanationSelector = 'section[aria-labelledby="analytics-reading"] > p';
    const explanationText = (await page.locator(explanationSelector).innerText()).trim();
    lane.semanticFacts.selectedOverview = { url: page.url(), values: valueFacts, explanationSelector, explanationText, lowerBoundExplanation: false };
    const explanation = explanationText.toLowerCase();
    if (!explanation.includes('lower bound') || !explanation.includes('full month')) throw Error(`Partial interpretation is missing its lower-bound/full-month limit: ${JSON.stringify(lane.semanticFacts.selectedOverview)}`);
    lane.semanticFacts.selectedOverview.lowerBoundExplanation = true;
    lane.targetHeights.push(...await page.evaluate(() => [...document.querySelectorAll('#gallery-main select, #gallery-main button, #gallery-main summary, #analytics-value-table a')]
      .filter(element => element.checkVisibility()).map(element => {
        const box = element.getBoundingClientRect();
        return { tag: element.tagName, text: element.textContent.trim().slice(0, 100), width: box.width, height: box.height, below44: box.width < 44 || box.height < 44 };
      })));
    const overviewGeometry = await page.evaluate(() => ({ viewport: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth, scripts: document.scripts.length }));
    if (overviewGeometry.scrollWidth > overviewGeometry.viewport || overviewGeometry.scripts !== 0) throw Error(`Overview geometry/script refusal: ${JSON.stringify(overviewGeometry)}`);
    await capture(page, artifact, lane, 'selected-overview');

    await skipToMain(page, lane);
    await tabTo(page, lane, '#analytics-chart-gateway-api a');
    const detailNavigation = observed(page.waitForURL(url => url.pathname === '/analytics/gateway-api'));
    await press(page, lane, 'Enter', detailNavigation);
    await recordTiming('gateway-detail');
    const detailURL = new URL(page.url());
    const detailSelection = Object.fromEntries([...selectionKeys].map(key => [key, detailURL.searchParams.get(key)]));
    if (JSON.stringify(detailSelection) !== JSON.stringify({ period: '2026-08', compare: 'none', category: 'api', state: 'partial', theme: dimensions.theme })) throw Error(`Detail lost selection: ${JSON.stringify(detailSelection)}`);
    if (!await page.getByRole('heading', { level: 1, name: 'Gateway API', exact: true }).isVisible() || !await page.locator('#analytics-source').isVisible()) throw Error('Gateway detail/source is not visible');
    const detailIDs = await page.locator('[data-value-id]').evaluateAll(rows => rows.map(row => row.getAttribute('data-value-id')));
    const detailRow = page.locator('[data-value-id="gateway-api"]');
    const detailValue = (await detailRow.locator('td').first().innerText()).trim();
    const detailState = (await detailRow.locator('td').last().innerText()).trim();
    const detailExplanation = (await page.locator(explanationSelector).innerText()).trim();
    if (JSON.stringify(detailIDs) !== JSON.stringify(['gateway-api']) || detailValue !== report.oracle.values['gateway-api'] || detailState !== 'Partial, lower bound' ||
        !detailExplanation.toLowerCase().includes('lower bound') || !detailExplanation.toLowerCase().includes('full month')) throw Error(`Gateway detail lost its value/coverage: ${JSON.stringify({detailIDs, detailValue, detailState, detailExplanation})}`);
    lane.semanticFacts.detail = { url: page.url(), selection: detailSelection, sourceVisible: true, ids: detailIDs, value: detailValue, state: detailState, explanation: detailExplanation };
    await capture(page, artifact, lane, 'gateway-detail');

    await skipToMain(page, lane);
    await tabTo(page, lane, 'a[href^="/analytics?"]');
    const backNavigation = observed(page.waitForURL(url => url.pathname === '/analytics'));
    await press(page, lane, 'Enter', backNavigation);
    await recordTiming('returned-overview');
    const returned = new URL(page.url());
    const returnedSelection = ['period', 'compare', 'category', 'state'].map(key => [key, returned.searchParams.get(key)]);
    if (JSON.stringify(returnedSelection) !== JSON.stringify([['period', '2026-08'], ['compare', 'none'], ['category', 'api'], ['state', 'partial']])) throw Error(`Back lost four selections: ${JSON.stringify(returnedSelection)}`);
    lane.semanticFacts.returned = { url: page.url(), selections: returnedSelection };

    await skipToMain(page, lane);
    await tabTo(page, lane, 'a[href^="/analytics/export.csv?"]');
    const downloadEvent = observed(page.waitForEvent('download'));
    await press(page, lane, 'Enter');
    const download = await downloadEvent;
    const csvPath = path.join(out, 'downloads', `${artifact.value.label}-${dimensions.width}-${dimensions.theme}.csv`);
    fs.mkdirSync(path.dirname(csvPath), { recursive: true, mode: 0o700 });
    await download.saveAs(csvPath);
    const csvBytes = fs.readFileSync(csvPath);
    const csv = parseCSV(csvBytes.toString('utf8'));
    const csvValues = validateOverviewCSV(csv);
    lane.semanticFacts.export = { file: path.relative(out, csvPath), bytes: csvBytes.length, sha256: sha(csvBytes), columns: 33, rows: 2, values: csvValues };
    await Promise.all(pending);
    if (!lane.responses.some(response => new URL(response.url).pathname === '/static/app.css' && response.sha256 === artifact.value.css.sha256)) throw Error('No exact served CSS response was observed');
    if (context.serviceWorkers().length !== 0) throw Error('Service worker unexpectedly active');
    if (lane.failures.length) throw Error(`Browser response/console failures: ${JSON.stringify(lane.failures)}`);
    lane.passed = true;
  } catch (error) {
    lane.failures.push({ scope: 'lane', detail: error.stack ?? String(error) });
    if (context) {
      try {
        const page = context.pages().at(-1);
        if (page) lane.failureCapture = await capture(page, artifact, lane, 'failure');
      } catch (captureError) { lane.failures.push({ scope: 'failure-capture', detail: captureError.stack ?? String(captureError) }); }
    }
  } finally {
    if (pending.length) {
      try { await Promise.race([Promise.all(pending), new Promise((_, reject) => setTimeout(() => reject(Error('response settlement timeout')), 3000))]); }
      catch (error) { lane.failures.push({ scope: 'response-settlement', detail: error.stack ?? String(error) }); }
    }
    if (context) {
      try { await Promise.race([context.close(), new Promise((_, reject) => setTimeout(() => reject(Error('browser close timeout')), 5000))]); lane.cleanup.closeReturned = true; }
      catch (error) {
        lane.cleanup.closeError = error.stack ?? String(error);
        lane.failures.push({ scope: 'cleanup-close', detail: lane.cleanup.closeError });
      }
    }
    if (identity) {
      lane.cleanup.pid = identity.pid;
      lane.cleanup.afterClose = await waitForExit(identity.pid, 3000);
      if (!lane.cleanup.afterClose.exited) {
        const current = spawnSync('/bin/ps', ['-ww', '-p', String(identity.pid), '-o', 'pid=,pgid=,lstart=,command='], { encoding: 'utf8', timeout: 2000, maxBuffer: 1024 * 1024 });
        if (!current.error && current.status === 0 && current.stdout.trim() === identity.psIdentity) {
          try {
            process.kill(identity.pid, 'SIGTERM');
            lane.cleanup.ownedSIGTERM = true;
            lane.cleanup.afterSIGTERM = await waitForExit(identity.pid, 3000);
          } catch (error) { lane.cleanup.signalError = error.stack ?? String(error); }
        } else lane.cleanup.identityMismatchRefusedSignal = true;
      }
      const finalPID = pidState(identity.pid);
      lane.cleanup.finalPID = finalPID;
      if (finalPID.state === 'present') lane.failures.push({ scope: 'cleanup', detail: `Owned browser PID ${identity.pid} remains` });
      if (finalPID.state === 'unknown') lane.failures.push({ scope: 'cleanup-pid-unknown', detail: finalPID });
    }
    const removableProfile = identity && lane.cleanup.finalPID?.state === 'absent';
    if (removableProfile) {
      try { fs.rmSync(profile, { recursive: true, force: false }); lane.cleanup.profileRemoved = true; }
      catch (error) { lane.cleanup.profileRemoveError = error.stack ?? String(error); lane.failures.push({ scope: 'cleanup-profile', detail: lane.cleanup.profileRemoveError }); }
    } else {
      lane.cleanup.profileRetainedForUnknownOrLivePID = true;
      if (!identity) lane.failures.push({scope: 'cleanup-pid-unknown', detail: 'Profile retained because browser PID ownership was not established.'});
    }
    if (lane.failures.length) lane.passed = false;
    report.cleanup.push({ artifact: artifact.value.label, lane: `${dimensions.width}-${dimensions.theme}`, ...lane.cleanup });
    persist();
  }
}

for (const artifact of artifacts) {
  const identity = report.artifacts.find(item => item.label === artifact.value.label);
  if (!identity?.binary?.matched || !identity?.css?.matched) continue;
  for (const lane of lanes) await runLane(artifact, lane);
}

for (const artifact of artifacts) {
  const itsLanes = report.lanes.filter(lane => lane.artifact === artifact.value.label);
  if (itsLanes.length !== lanes.length || itsLanes.some(lane => !lane.passed)) fail(`${artifact.value.label}: complete lane set`, { observed: itsLanes.length, passed: itsLanes.filter(lane => lane.passed).length });
}
if (report.blockedRequests.length) fail('Global forbidden request attempts', report.blockedRequests);
report.passed = report.failures.length === 0 && report.lanes.length === artifacts.length * lanes.length && report.lanes.every(lane => lane.passed);
report.finishedAt = new Date().toISOString();
persist();
console.log(JSON.stringify({ out, passed: report.passed, lanes: report.lanes.length, failures: report.failures.length, receiptSHA256: shaFile(path.join(out, 'receipt.json')) }));
process.exitCode = report.passed ? 0 : 1;
