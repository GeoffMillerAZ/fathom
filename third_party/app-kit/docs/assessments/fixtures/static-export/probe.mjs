#!/usr/bin/env node
import { createHash } from 'node:crypto';
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs/promises';
import http from 'node:http';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const sha256 = value => createHash('sha256').update(value).digest('hex');
const exact = (actual, expected) => JSON.stringify(actual) === JSON.stringify(expected);
const unique = values => new Set(values).size === values.length;
const finitePositive = value => Number.isFinite(value) && value > 0;
const expectedFiles = [
  'index.html', 'index-light.html', 'index-dark.html',
  'profile.html', 'profile-light.html', 'profile-dark.html',
  '404.html', '404-light.html', '404-dark.html',
  'app.css', 'manifest.json',
];
const expectedLanes = [
  [390, 1000, 'system', 'dark'], [390, 1000, 'light', 'light'], [390, 1000, 'dark', 'dark'],
  [1440, 1000, 'system', 'dark'], [1440, 1000, 'light', 'light'], [1440, 1000, 'dark', 'dark'],
];

export function validateContract(contract) {
  if (contract?.schema_version !== 'static-export-local-v1' || contract.manifest_schema_version !== 1 || contract.base_path !== '/reading/' || contract.javascript !== false || contract.cache !== 'disabled' || contract.network !== 'unthrottled-owned-loopback') throw Error('unsupported contract');
  if (!exact(contract.budgets, { decoded_bytes: 131072, response_end_ms: 100, dom_content_loaded_ms: 250, load_end_ms: 500 })) throw Error('contract budgets changed');
  if (!exact(contract.files, expectedFiles) || !unique(contract.files)) throw Error('contract files changed');
  const lanes = contract.lanes?.map(lane => [lane.width, lane.height, lane.theme, lane.color_scheme]);
  if (!exact(lanes, expectedLanes) || !unique(lanes?.map(String) ?? [])) throw Error('contract lanes changed');
  if (!exact(contract.paths, { index: 'index.html', profile: 'profile.html', not_found: '404.html', deep_missing: '/reading/missing/deep/page.html' })) throw Error('contract paths changed');
  const expectedOracles = {
    index_heading: 'Choose a profile before you integrate a capability', profile_heading: 'Choosing a profile', problem_heading: 'Content destination unavailable',
    faq_question: 'Which profile should I choose?', faq_destination_label: 'Read the related guide', return_label: 'Return to the integration guide',
    resource_labels: ['Match the runtime to the task', 'Review delivery evidence'], resource_fragments: ['content-runtime-fit', 'content-delivery-evidence'],
    toc_fragments: ['content-guide', 'start-with-task', 'keep-policy-local', 'content-faq'],
  };
  if (!exact(contract.oracles, expectedOracles)) throw Error('contract oracles changed');
  return contract;
}

export function selectedFile(kind, theme) {
  const base = { index: 'index', profile: 'profile', not_found: '404' }[kind];
  if (!base || !['system', 'light', 'dark'].includes(theme)) throw Error('unknown static selection');
  return `${base}${theme === 'system' ? '' : `-${theme}`}.html`;
}

export function resolveStaticRequest(contract, rawURL, method = 'GET') {
  const failures = [];
  let url;
  try { url = new URL(rawURL, 'http://127.0.0.1'); } catch { return { failures: ['request.url'] }; }
  if (method !== 'GET') failures.push('request.method');
  const base = contract.base_path;
  if (url.pathname === `${base}app.css`) {
    if (url.search) failures.push('request.css-query');
    return { failures, file: 'app.css', status: 200, mime: 'text/css; charset=utf-8' };
  }
  if (url.pathname === base && !url.search) return { failures, file: 'index.html', status: 200, mime: 'text/html; charset=utf-8' };
  const direct = url.pathname.startsWith(base) ? url.pathname.slice(base.length) : '';
  if (expectedFiles.filter(name => name.endsWith('.html')).includes(direct) && !url.search) return { failures, file: direct, status: 200, mime: 'text/html; charset=utf-8' };
  if (url.pathname === contract.paths.deep_missing) {
    if (url.search) failures.push('request.query');
    return { failures, file: selectedFile('not_found', 'system'), status: 404, mime: 'text/html; charset=utf-8' };
  }
  failures.push('request.path');
  return { failures };
}

export function assessAllowedRequest(origin, rawURL, method, contract) {
  let url;
  try { url = new URL(rawURL); } catch { return ['request.url']; }
  const failures = url.origin === origin ? [] : ['request.origin'];
  return [...failures, ...resolveStaticRequest(contract, url.href, method).failures];
}

export function validateManifest(contract, manifest, site) {
  const failures = [];
  const listed = manifest?.files;
  if (manifest?.schema_version !== contract.manifest_schema_version) failures.push('manifest.schema');
  if (manifest?.base_path !== contract.base_path) failures.push('manifest.base');
  if (!exact(manifest?.manifest_excludes, ['manifest.json'])) failures.push('manifest.excludes');
  if (!Array.isArray(listed) || !unique((listed ?? []).map(entry => entry?.path))) failures.push('manifest.files');
  const expected = expectedFiles.filter(name => name !== 'manifest.json');
  if (!exact((listed ?? []).map(entry => entry.path).sort(), [...expected].sort())) failures.push('manifest.closure');
  if (!exact(Object.keys(site).sort(), [...expectedFiles].sort())) failures.push('site.closure');
  for (const name of expectedFiles) {
    const actual = site[name];
    if (!actual || !Number.isInteger(actual.bytes) || actual.bytes <= 0 || !/^[a-f0-9]{64}$/.test(actual.sha256)) failures.push(`site.identity:${name}`);
    if (actual?.kind !== 'file') failures.push(`site.kind:${name}`);
    if (name !== 'manifest.json') {
      const entry = (listed ?? []).find(candidate => candidate.path === name);
      if (!entry || entry.bytes !== actual?.bytes || entry.sha256 !== actual?.sha256) failures.push(`manifest.identity:${name}`);
    }
  }
  return [...new Set(failures)];
}

export function assessLinks(contract, file, links) {
  const failures = [];
  const notFound = file.startsWith('404');
  const theme = file.includes('-light.') ? 'light' : file.includes('-dark.') ? 'dark' : 'system';
  const allowedFiles = new Set(expectedFiles.filter(name => name.endsWith('.html')));
  for (const href of links ?? []) {
    if (typeof href !== 'string' || !href || href.endsWith('#') || /[\s\\%?:]/.test(href) || href.includes('..') || href.startsWith('//') || href.split('#').length > 2) { failures.push('link.unsafe'); continue; }
    const [rawPath, fragment = ''] = href.split('#');
    let targetFile = file;
    if (rawPath && notFound) {
      targetFile = rawPath.startsWith(contract.base_path) ? rawPath.slice(contract.base_path.length) : '';
      if (!allowedFiles.has(targetFile)) failures.push('link.not-found-path');
    } else if (rawPath) {
      targetFile = rawPath;
      if (rawPath.startsWith('/') || rawPath.includes('/') || !allowedFiles.has(rawPath)) failures.push('link.normal-path');
    }
    if (fragment) {
      const allowedFragments = targetFile.startsWith('index')
        ? new Set(['static-reference-main', ...contract.oracles.toc_fragments])
        : targetFile.startsWith('profile')
          ? new Set(['static-reference-main', 'content-profile-title', ...contract.oracles.resource_fragments])
          : new Set(['static-reference-main']);
      if (!allowedFragments.has(fragment)) failures.push('link.fragment');
    }
  }
  const indexFile = selectedFile('index', theme), profileFile = selectedFile('profile', theme);
  if (notFound && !links?.includes(`${contract.base_path}${indexFile}`)) failures.push('link.recovery');
  if (!notFound && file.startsWith('index') && !links?.includes(`${profileFile}#content-runtime-fit`)) failures.push('link.resource');
  if (!notFound && file.startsWith('profile') && !links?.includes(`${indexFile}#content-guide`)) failures.push('link.return');
  return [...new Set(failures)];
}

export function assessObservation(contract, observation, site) {
  const failures = [];
  const resolved = resolveStaticRequest(contract, observation.url, 'GET');
  if (resolved.failures.length || resolved.file !== observation.expectedFile || resolved.status !== observation.expectedStatus) failures.push('document.route');
  if (observation.status !== observation.expectedStatus || observation.taskPassed !== true) failures.push(observation.status !== observation.expectedStatus ? 'document.status' : 'task');
  if (observation.scriptNodes !== 0 || observation.scriptAttempts !== 0) failures.push('script-free');
  if (!Array.isArray(observation.deniedRequests) || observation.deniedRequests.length) failures.push('network');
  const resources = observation.resources ?? [];
  const docs = resources.filter(resource => resource.kind === 'document');
  const css = resources.filter(resource => resource.kind === 'stylesheet');
  if (resources.length !== 2 || docs.length !== 1 || css.length !== 1) failures.push('resources.exact');
  for (const resource of resources) {
    let request;
    try { request = new URL(resource.url); } catch { failures.push('resource.url'); continue; }
    if (request.origin !== observation.origin || resource.method !== 'GET') failures.push('resource.request');
    if (resource.fromCache !== false) failures.push('resource.cache');
    if (resource.terminal !== true || resource.bodyAvailable !== true || !Number.isInteger(resource.decodedBytes) || resource.decodedBytes <= 0) failures.push('resource.body');
    const route = resolveStaticRequest(contract, resource.url, resource.method);
    const expected = site[route.file];
    if (route.failures.length || !expected || resource.status !== route.status || resource.mime !== route.mime || resource.sha256 !== expected.sha256 || resource.decodedBytes !== expected.bytes) failures.push('resource.identity');
  }
  const document = docs[0];
  if (document && (document.url !== observation.url || document.sha256 !== observation.html?.sha256 || document.decodedBytes !== observation.html?.bytes || observation.html.file !== observation.expectedFile)) failures.push('document.artifact');
  const total = resources.reduce((sum, resource) => sum + (Number.isInteger(resource.decodedBytes) ? resource.decodedBytes : 0), 0);
  if (total > contract.budgets.decoded_bytes) failures.push('budget.bytes');
  const timing = observation.timings ?? {};
  for (const [field, budget] of [['responseEnd', contract.budgets.response_end_ms], ['domContentLoaded', contract.budgets.dom_content_loaded_ms], ['loadEnd', contract.budgets.load_end_ms]]) {
    if (!finitePositive(timing[field])) failures.push(`timing.${field}`);
    else if (timing[field] > budget) failures.push(`budget.${field}`);
  }
  if (finitePositive(timing.responseEnd) && finitePositive(timing.domContentLoaded) && finitePositive(timing.loadEnd) && !(timing.responseEnd <= timing.domContentLoaded && timing.domContentLoaded <= timing.loadEnd)) failures.push('timing.order');
  if (observation.taskFacts?.noOverflow !== true || observation.taskFacts?.controls44 !== true || observation.taskFacts?.themeCorrect !== true) failures.push('task.facts');
  return [...new Set(failures)];
}

export function assessCleanup(cleanup) {
  const failures = [];
  if (cleanup?.browser?.exitCode !== 0 || cleanup?.browser?.signalCode !== null) failures.push('cleanup.browser');
  if (cleanup?.serverClosed !== true || cleanup?.portRefused !== true || cleanup?.refusalCode !== 'ECONNREFUSED') failures.push('cleanup.server');
  return failures;
}
export const isConnectionRefused = error => error?.code === 'ECONNREFUSED';

export function assessPresentationFacts(facts, { requireToc = false } = {}) {
  const failures = [];
  if (facts?.appearanceSelected !== true) failures.push('presentation.appearance');
  if (requireToc && (facts?.tocUnderlined !== true || facts?.tocLinkCount !== 4)) failures.push('presentation.toc');
  return failures;
}

async function readSite(siteRoot) {
  const names = await fs.readdir(siteRoot);
  const site = {};
  for (const name of names) {
    const full = path.join(siteRoot, name), stat = await fs.lstat(full);
    const bytes = stat.isFile() && !stat.isSymbolicLink() ? await fs.readFile(full) : Buffer.alloc(0);
    site[name] = { kind: stat.isFile() && !stat.isSymbolicLink() ? 'file' : 'other', bytes: bytes.length, sha256: sha256(bytes) };
  }
  return site;
}

function bounded(promise, label, millis = 8000) {
  return Promise.race([promise, new Promise((_, reject) => setTimeout(() => reject(Error(`${label} timeout`)), millis))]);
}

async function portRefusal(port) {
  return new Promise(resolve => {
    const socket = net.connect({ host: '127.0.0.1', port });
    const timer = setTimeout(() => { socket.destroy(); resolve({ refused: false, code: 'TIMEOUT' }); }, 2000);
    socket.once('error', error => { clearTimeout(timer); resolve({ refused: isConnectionRefused(error), code: error.code }); });
    socket.once('connect', () => { clearTimeout(timer); socket.destroy(); resolve({ refused: false, code: 'CONNECTED' }); });
  });
}

async function orchestrate() {
  const [, , evidenceParent, siteRoot, runtimePath, contractPath] = process.argv;
  if (![evidenceParent, siteRoot, runtimePath, contractPath].every(value => value && path.isAbsolute(value))) throw Error('usage: probe.mjs ABS_EVIDENCE_PARENT ABS_SITE_ROOT ABS_RUNTIME_JSON ABS_CONTRACT_JSON');
  const parent = await fs.realpath(evidenceParent), siteCanonical = await fs.realpath(siteRoot);
  const source = fileURLToPath(import.meta.url), testSource = path.join(path.dirname(source), 'probe_test.mjs');
  const [sourceBytes, testBytes, runtimeBytes, contractBytes] = await Promise.all([source, testSource, runtimePath, contractPath].map(file => fs.readFile(file)));
  const contract = validateContract(JSON.parse(contractBytes));
  const site = await readSite(siteCanonical), manifest = JSON.parse(await fs.readFile(path.join(siteCanonical, 'manifest.json')));
  const manifestFailures = validateManifest(contract, manifest, site);
  if (manifestFailures.length) throw Error(`invalid frozen site: ${manifestFailures.join(', ')}`);
  const root = await fs.mkdtemp(path.join(parent, 'static-export-')); await fs.chmod(root, 0o700);
  const snapshots = path.join(root, 'inputs'), siteSnapshot = path.join(snapshots, 'site');
  await fs.mkdir(siteSnapshot, { recursive: true, mode: 0o700 });
  for (const [name, bytes, mode] of [['probe.mjs', sourceBytes, 0o700], ['probe_test.mjs', testBytes, 0o600], ['runtime.json', runtimeBytes, 0o600], ['contract.json', contractBytes, 0o600]]) await fs.writeFile(path.join(snapshots, name), bytes, { flag: 'wx', mode });
  for (const name of expectedFiles) await fs.copyFile(path.join(siteCanonical, name), path.join(siteSnapshot, name), fs.constants.COPYFILE_EXCL);
  const child = spawn(process.execPath, [path.join(snapshots, 'probe.mjs'), '--execute', root], { cwd: root, stdio: 'inherit', env: { ...process.env, TMPDIR: root } });
  process.exitCode = await new Promise((resolve, reject) => { child.once('error', reject); child.once('exit', code => resolve(code ?? 1)); });
}

async function execute(root) {
  const inputRoot = path.join(root, 'inputs'), siteRoot = path.join(inputRoot, 'site');
  const contractBytes = await fs.readFile(path.join(inputRoot, 'contract.json')), runtimeBytes = await fs.readFile(path.join(inputRoot, 'runtime.json'));
  const contract = validateContract(JSON.parse(contractBytes)), runtime = JSON.parse(runtimeBytes), site = await readSite(siteRoot);
  const manifest = JSON.parse(await fs.readFile(path.join(siteRoot, 'manifest.json')));
  const report = { schema_version: 'static-export-browser-v1', started: new Date().toISOString(), inputs: {}, observations: [], requests: [], captures: [], failures: [] };
  const fail = (name, detail = {}) => report.failures.push({ name, detail });
  for (const [name, relative] of [['harness', 'probe.mjs'], ['tests', 'probe_test.mjs'], ['contract', 'contract.json'], ['runtime', 'runtime.json']]) report.inputs[name] = { file: `inputs/${relative}`, sha256: sha256(await fs.readFile(path.join(inputRoot, relative))) };
  report.inputs.site = site;
  let server, browserServer, browser, port, origin;
  let globalDenied = [];
  try {
    const manifestFailures = validateManifest(contract, manifest, site); if (manifestFailures.length) throw Error(manifestFailures.join(', '));
    if (runtime?.schema_version !== 'static-export-runtime-v1' || runtime.playwright?.version !== '1.62.1') throw Error('unsupported runtime configuration');
    for (const tool of [runtime.playwright, runtime.chrome]) {
      if (!path.isAbsolute(tool.path) || sha256(await fs.readFile(tool.path)) !== tool.sha256) throw Error('runtime tool identity mismatch');
    }
    const playwrightPackage = JSON.parse(await fs.readFile(runtime.playwright.package_json));
    if (playwrightPackage.version !== runtime.playwright.version || sha256(await fs.readFile(runtime.playwright.package_json)) !== runtime.playwright.package_sha256) throw Error('Playwright package identity mismatch');
    const { chromium } = await import(pathToFileURL(runtime.playwright.path));
    server = http.createServer(async (request, response) => {
      const route = resolveStaticRequest(contract, request.url, request.method);
      report.requests.push({ method: request.method, url: request.url, admitted: route.failures.length === 0, file: route.file, status: route.status });
      if (route.failures.length) { response.writeHead(400, { 'content-type': 'text/plain; charset=utf-8', 'cache-control': 'no-store' }); response.end('refused'); return; }
      const body = await fs.readFile(path.join(siteRoot, route.file));
      response.writeHead(route.status, { 'content-type': route.mime, 'content-length': body.length, 'cache-control': 'no-store' }); response.end(body);
    });
    await bounded(new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); }), 'server listen');
    port = server.address().port; origin = `http://127.0.0.1:${port}`;
    browserServer = await chromium.launchServer({ headless: true, executablePath: runtime.chrome.path, args: ['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1', '--disable-features=BackForwardCache'], timeout: 15000 });
    browser = await chromium.connect(browserServer.wsEndpoint());
    const browserPID = browserServer.process()?.pid;
    const ps = spawnSync('/bin/ps', ['-ww', '-p', String(browserPID), '-o', 'command='], { encoding: 'utf8', timeout: 2000, maxBuffer: 65536 });
    if (!Number.isInteger(browserPID) || ps.error || ps.status !== 0 || ps.stderr !== '' || !ps.stdout.trim()) throw Error('browser process identity unavailable');
    const profileArguments = [...ps.stdout.matchAll(/(?:^|\s)--user-data-dir=([^\s]+)/g)].map(match => match[1]);
    if (profileArguments.length !== 1 || !path.resolve(profileArguments[0]).startsWith(`${path.resolve(root)}${path.sep}`)) throw Error('browser profile is not fresh and owned');
    report.browser = { version: browser.version(), pid: browserPID, commandLine: ps.stdout.trim(), userDataDir: profileArguments[0] };
    if (report.browser.version !== runtime.chrome.version) throw Error('Chrome version mismatch');
    for (const lane of contract.lanes) {
      const context = await browser.newContext({ viewport: { width: lane.width, height: lane.height }, colorScheme: lane.color_scheme, reducedMotion: 'reduce', javaScriptEnabled: false, serviceWorkers: 'block' });
      const page = await context.newPage(); page.setDefaultTimeout(7000);
      const cdp = await context.newCDPSession(page); await cdp.send('Network.enable'); await cdp.send('Network.setCacheDisabled', { cacheDisabled: true });
      let active = null; const cached = new Set(), urls = new Map();
      cdp.on('Network.requestWillBeSent', event => urls.set(event.requestId, event.request.url));
      cdp.on('Network.requestServedFromCache', event => { if (urls.get(event.requestId)) cached.add(urls.get(event.requestId)); });
      cdp.on('Network.responseReceived', event => { if (event.response.fromDiskCache || event.response.fromPrefetchCache || event.response.fromServiceWorker) cached.add(event.response.url); });
      await context.route('**/*', route => { const request = route.request(), denied = assessAllowedRequest(origin, request.url(), request.method(), contract), resolved = resolveStaticRequest(contract, request.url(), request.method()); if (active && ![active.expectedFile, 'app.css'].includes(resolved.file)) denied.push('request.unexpected-file'); if (denied.length) { const fact = { url: request.url(), method: request.method(), denied }; globalDenied.push(fact); active?.deniedRequests.push(fact); fail('forbidden-request', fact); return route.abort(); } return route.continue(); });
      page.on('request', request => { if (active) active.pending.set(request, { url: request.url(), method: request.method(), kind: request.resourceType() === 'document' ? 'document' : request.resourceType() === 'stylesheet' ? 'stylesheet' : request.resourceType(), terminal: false, bodyAvailable: false, fromCache: false }); });
      page.on('response', response => { const state = active, resource = state?.pending.get(response.request()); if (!resource) return; state.settlements.push((async () => { resource.status = response.status(); resource.mime = response.headers()['content-type']; try { const body = await response.body(); resource.decodedBytes = body.length; resource.sha256 = sha256(body); resource.bodyAvailable = true; } catch (error) { resource.bodyError = String(error); } resource.fromCache = response.fromServiceWorker() || cached.has(response.url()); resource.terminal = true; })()); });
      page.on('requestfailed', request => { const resource = active?.pending.get(request); if (resource) { resource.failure = request.failure(); resource.terminal = true; } });
      const capture = async stage => { const bytes = await page.screenshot({ fullPage: true, animations: 'disabled' }), file = `${lane.width}-${lane.theme}-${stage}.png`; await fs.writeFile(path.join(root, file), bytes, { flag: 'wx' }); const entry = { lane, stage, file, sha256: sha256(bytes) }; report.captures.push(entry); return entry; };
      const focusFact = () => page.evaluate(() => { const element = document.activeElement, style = getComputedStyle(element), rect = element.getBoundingClientRect(), color = style.outlineColor; return { tag: element?.tagName, id: element?.id, href: element?.getAttribute?.('href'), outlineStyle: style.outlineStyle, outlineWidth: Number.parseFloat(style.outlineWidth), outlineColor: color, rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height }, painted: style.outlineStyle !== 'none' && Number.parseFloat(style.outlineWidth) >= 2 && color !== 'transparent' && color !== 'rgba(0, 0, 0, 0)' && rect.width > 0 && rect.height > 0 && rect.bottom > 0 && rect.top < innerHeight }; });
      const tabTo = async selector => { for (let index = 0; index < 100; index++) { if (await page.evaluate(value => document.activeElement?.matches(value), selector)) return; await page.keyboard.press('Tab'); } throw Error(`Tab did not reach ${selector}`); };
      const tabToLinkName = async name => { for (let index = 0; index < 100; index++) { if (await page.evaluate(value => document.activeElement?.matches('a') && document.activeElement.textContent.trim() === value, name)) return; await page.keyboard.press('Tab'); } throw Error(`Tab did not reach link ${name}`); };
      const baseFacts = async (theme = lane.theme) => page.evaluate(({ expectedTheme, expectedScheme }) => { const controls = [...document.querySelectorAll('a,button,summary')].filter(element => element.checkVisibility() && !(element.classList.contains('sr-only') && document.activeElement !== element)); const style = getComputedStyle(document.body); const pixel = value => { const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1; const context = canvas.getContext('2d', { willReadFrequently: true }); context.clearRect(0, 0, 1, 1); context.fillStyle = value; context.fillRect(0, 0, 1, 1); return [...context.getImageData(0, 0, 1, 1).data]; }; const luminance = ([r, g, b]) => (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255; const foregroundPixel = pixel(style.color), backgroundPixel = pixel(style.backgroundColor), dark = luminance(backgroundPixel) < luminance(foregroundPixel), dataTheme = document.documentElement.dataset.theme ?? '', selectedAppearance = document.querySelector('nav[aria-label="Appearance"] [aria-current="page"]'), appearancePeers = [...document.querySelectorAll('nav[aria-label="Appearance"] a:not([aria-current="page"])')], selectedStyle = selectedAppearance ? getComputedStyle(selectedAppearance) : null, borderWidths = selectedStyle ? [selectedStyle.borderTopWidth, selectedStyle.borderRightWidth, selectedStyle.borderBottomWidth, selectedStyle.borderLeftWidth].map(Number.parseFloat) : [], selectedBorderPixel = selectedStyle ? pixel(selectedStyle.borderTopColor) : [0, 0, 0, 0], selectedBackgroundPixel = selectedStyle ? pixel(selectedStyle.backgroundColor) : [0, 0, 0, 0], peerBordersTransparent = appearancePeers.length === 2 && appearancePeers.every(element => pixel(getComputedStyle(element).borderTopColor)[3] === 0), appearanceSelected = Boolean(selectedAppearance) && borderWidths.some(width => width > 0) && selectedBorderPixel[3] > 0 && selectedBackgroundPixel[3] > 0 && Number.parseFloat(selectedStyle.fontWeight) >= 600 && peerBordersTransparent, tocLinks = [...document.querySelectorAll('main details > ol a')], tocUnderlined = tocLinks.length > 0 && tocLinks.every(element => getComputedStyle(element).textDecorationLine.split(' ').includes('underline')); const expectedDark = expectedTheme === 'dark' || (expectedTheme === 'system' && expectedScheme === 'dark'); return { dataTheme, foreground: style.color, background: style.backgroundColor, foregroundPixel, backgroundPixel, selectedBorderPixel, selectedBackgroundPixel, selectedFontWeight: selectedStyle?.fontWeight, peerBordersTransparent, appearanceSelected, tocLinkCount: tocLinks.length, tocUnderlined, noOverflow: document.documentElement.scrollWidth <= innerWidth, controls44: controls.every(element => { const rect = element.getBoundingClientRect(); return rect.width >= 44 && rect.height >= 44; }), themeCorrect: (expectedTheme === 'system' ? dataTheme === '' : dataTheme === expectedTheme) && dark === expectedDark && appearanceSelected }; }, { expectedTheme: theme, expectedScheme: lane.color_scheme });
      const navigate = async (target, stage, expectedFile, expectedStatus, task, action) => {
        active = { expectedFile, pending: new Map(), settlements: [], deniedRequests: [] }; let response;
        if (action) { const navigation = page.waitForNavigation({ waitUntil: 'load' }).then(value => ({ value }), error => ({ error })); await action(); const result = await navigation; if (result.error) throw result.error; response = result.value; }
        else response = await page.goto(origin + target, { waitUntil: 'load' });
        await page.waitForLoadState('networkidle'); await Promise.all(active.settlements); await page.waitForTimeout(0);
        for (const resource of active.pending.values()) if (cached.has(resource.url)) resource.fromCache = true;
        const html = await response.body(), htmlFile = `${lane.width}-${lane.theme}-${stage}.html`; await fs.writeFile(path.join(root, htmlFile), html, { flag: 'wx' });
        const timings = await page.evaluate(() => { const entry = performance.getEntriesByType('navigation')[0]; return { responseEnd: entry?.responseEnd, domContentLoaded: entry?.domContentLoadedEventEnd, loadEnd: entry?.loadEventEnd }; });
        const taskFacts = await task(), observation = { lane, stage, origin, url: response.url(), status: response.status(), expectedFile, expectedStatus, html: { file: expectedFile, retained: htmlFile, bytes: html.length, sha256: sha256(html) }, scriptNodes: await page.locator('script').count(), scriptAttempts: [...active.pending.values()].filter(resource => resource.kind === 'script').length, deniedRequests: active.deniedRequests, resources: [...active.pending.values()], timings, taskFacts, taskPassed: taskFacts.passed === true };
        observation.failures = assessObservation(contract, observation, site); if (observation.failures.length) fail('observation', { lane, stage, failures: observation.failures }); report.observations.push(observation); await capture(stage); active = null; return observation;
      };
      const themeFile = (kind, theme = lane.theme) => selectedFile(kind, theme), themeHref = (kind, theme = lane.theme, fragment = '') => `${themeFile(kind, theme)}${fragment}`;
      await navigate(`/reading/${themeFile('index')}`, 'index', themeFile('index'), 200, async () => { const base = await baseFacts(), links = await page.locator('a').evaluateAll(elements => elements.map(element => element.getAttribute('href'))), presentationFailures = assessPresentationFacts(base, { requireToc: true }); const linkFailures = assessLinks(contract, themeFile('index'), links); return { ...base, heading: await page.locator('#content-title').textContent(), linkFailures, presentationFailures, passed: base.noOverflow && base.controls44 && base.themeCorrect && !linkFailures.length && !presentationFailures.length && await page.locator('#content-title').textContent() === contract.oracles.index_heading }; });
      await tabTo('a[href="#static-reference-main"]'); const skipFocus = await focusFact(); await page.keyboard.press('Enter'); const skipTarget = await focusFact(); const skipPassed = skipFocus.painted && skipFocus.rect.width >= 44 && skipFocus.rect.height >= 44 && skipTarget.id === 'static-reference-main' && skipTarget.painted && await page.evaluate(() => location.hash) === '#static-reference-main'; if (!skipPassed) fail('skip-focus', { lane, skipFocus, skipTarget }); await capture('skip-focus');
      for (const fragment of contract.oracles.toc_fragments) {
        await tabTo(`a[href="#${fragment}"]`); const tocLink = await focusFact(); await page.keyboard.press('Enter'); const tocTarget = await focusFact(); const tocPassed = tocLink.painted && tocTarget.id === fragment && tocTarget.painted && await page.evaluate(value => location.hash === `#${value}`, fragment); if (!tocPassed) fail('toc-focus', { lane, fragment, tocLink, tocTarget }); await capture(`toc-${fragment}`);
      }
      await tabTo('#profile summary'); const faqFocus = await focusFact(), faqQuestion = (await page.locator('#profile summary').textContent()).trim(); await page.keyboard.press('Enter'); await page.locator('#source summary').click(); const faqPassed = faqQuestion === contract.oracles.faq_question && faqFocus.painted && await page.locator('#profile').evaluate(element => element.open) && await page.locator('#source').evaluate(element => element.open); if (!faqPassed) fail('faq', { lane, faqQuestion, faqFocus }); await capture('faq');
      await navigate(null, 'profile', themeFile('profile'), 200, async () => { const base = await baseFacts(), links = await page.locator('a').evaluateAll(elements => elements.map(element => element.getAttribute('href'))), destination = await page.evaluate(() => ({ hash: location.hash, activeId: document.activeElement?.id })); const linkFailures = assessLinks(contract, themeFile('profile'), links); return { ...base, heading: await page.locator('#content-profile-title').textContent(), destination, linkFailures, passed: base.noOverflow && base.controls44 && base.themeCorrect && !linkFailures.length && destination.hash === '#content-profile-title' && destination.activeId === 'content-profile-title' && await page.locator('#content-profile-title').textContent() === contract.oracles.profile_heading }; }, async () => {
        // Both open FAQs have the same visible link label. Select the declared
        // profile question's link, never whichever repeated label Tab reaches first.
        await tabTo(`#profile a[href="${themeHref('profile', lane.theme, '#content-profile-title')}"]`);
        if (await page.evaluate(() => document.activeElement.textContent.trim()) !== contract.oracles.faq_destination_label) throw Error('unexpected FAQ destination label');
        await page.keyboard.press('Enter');
      });
      await navigate(null, 'history-back', themeFile('index'), 200, async () => { const base = await baseFacts(); return { ...base, route: page.url(), passed: base.noOverflow && base.controls44 && base.themeCorrect && await page.locator('#content-title').textContent() === contract.oracles.index_heading }; }, async () => { await page.goBack({ waitUntil: 'load' }); });
      await navigate(null, 'history-forward', themeFile('profile'), 200, async () => { const base = await baseFacts(); return { ...base, route: page.url(), passed: base.noOverflow && base.controls44 && base.themeCorrect && await page.locator('#content-profile-title').textContent() === contract.oracles.profile_heading }; }, async () => { await page.goForward({ waitUntil: 'load' }); });
      const opposite = lane.theme === 'light' ? 'dark' : 'light';
      await navigate(null, 'profile-theme-opposite', themeFile('profile', opposite), 200, async () => { const base = await baseFacts(opposite); return { ...base, passed: base.noOverflow && base.controls44 && base.themeCorrect && await page.locator('#content-profile-title').isVisible() }; }, async () => { await tabTo(`a[href="${themeFile('profile', opposite)}"]`); await page.keyboard.press('Enter'); });
      await navigate(null, 'profile-theme-return', themeFile('profile'), 200, async () => { const base = await baseFacts(); return { ...base, passed: base.noOverflow && base.controls44 && base.themeCorrect && await page.locator('#content-profile-title').isVisible() }; }, async () => { await tabTo(`a[href="${themeFile('profile')}"]`); await page.keyboard.press('Enter'); });
      for (let index = 0; index < contract.oracles.resource_labels.length; index++) {
        await navigate(null, `resource-return-${index}`, themeFile('index'), 200, async () => { const base = await baseFacts(); return { ...base, passed: base.noOverflow && base.controls44 && base.themeCorrect && await page.locator('#content-title').isVisible() }; }, async () => { await tabToLinkName(contract.oracles.return_label); await page.keyboard.press('Enter'); });
        await navigate(null, `resource-${index}`, themeFile('profile'), 200, async () => { const base = await baseFacts(); return { ...base, passed: base.noOverflow && base.controls44 && base.themeCorrect && await page.evaluate(fragment => location.hash === `#${fragment}` && document.activeElement?.id === fragment, contract.oracles.resource_fragments[index]) }; }, async () => { await tabToLinkName(contract.oracles.resource_labels[index]); await page.keyboard.press('Enter'); });
      }
      await navigate(null, 'return', themeFile('index'), 200, async () => { const base = await baseFacts(); return { ...base, passed: base.noOverflow && base.controls44 && base.themeCorrect && await page.locator('#content-title').isVisible() }; }, async () => { await tabTo(`a[href="${themeHref('index', lane.theme, '#content-guide')}"]`); await page.keyboard.press('Enter'); });
      await navigate(null, 'theme-opposite', themeFile('index', opposite), 200, async () => { const base = await baseFacts(opposite); return { ...base, passed: base.noOverflow && base.controls44 && base.themeCorrect && await page.locator('#content-title').isVisible() }; }, async () => { await tabTo(`a[href="${themeFile('index', opposite)}"]`); await page.keyboard.press('Enter'); });
      await navigate(`/reading/${themeFile('not_found')}`, 'named-not-found', themeFile('not_found'), 200, async () => { const base = await baseFacts(), links = await page.locator('a').evaluateAll(elements => elements.map(element => element.getAttribute('href'))); const linkFailures = assessLinks(contract, themeFile('not_found'), links); return { ...base, linkFailures, passed: base.noOverflow && base.controls44 && base.themeCorrect && !linkFailures.length && await page.locator('#content-problem-title').textContent() === contract.oracles.problem_heading }; });
      await navigate(contract.paths.deep_missing, 'deep-not-found', themeFile('not_found', 'system'), 404, async () => { const base = await baseFacts('system'), links = await page.locator('a').evaluateAll(elements => elements.map(element => element.getAttribute('href'))); const linkFailures = assessLinks(contract, themeFile('not_found', 'system'), links); return { ...base, linkFailures, passed: base.noOverflow && base.controls44 && base.themeCorrect && !linkFailures.length && await page.locator('#content-problem-title').textContent() === contract.oracles.problem_heading }; });
      await navigate(null, 'recovered', themeFile('index', 'system'), 200, async () => { const base = await baseFacts('system'); return { ...base, passed: base.noOverflow && base.controls44 && base.themeCorrect && await page.locator('#content-title').isVisible() }; }, async () => { await tabTo(`a[href="/reading/${themeFile('index', 'system')}"]`); await page.keyboard.press('Enter'); });
      await context.close();
    }
    if (globalDenied.length) fail('global-forbidden-requests', { globalDenied });
  } catch (error) { fail('runtime', { error: String(error), stack: error.stack }); }
  finally {
    try { if (browser) await bounded(browser.close(), 'browser close'); if (browserServer) await bounded(browserServer.close(), 'browser server close'); } catch (error) { fail('cleanup.browser', { error: String(error) }); try { browserServer?.process()?.kill('SIGKILL'); } catch {} }
    const process = browserServer?.process(); if (process && process.exitCode === null && process.signalCode === null) await bounded(new Promise(resolve => process.once('close', resolve)), 'browser exit').catch(error => fail('cleanup.browser-exit', { error: String(error) }));
    try { if (server) await bounded(new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve())), 'server close'); } catch (error) { fail('cleanup.server-close', { error: String(error) }); }
    const refusal = Number.isInteger(port) ? await portRefusal(port) : { refused: false, code: 'NO_PORT' };
    report.cleanup = { browser: { exitCode: process?.exitCode, signalCode: process?.signalCode ?? null }, serverClosed: server ? !server.listening : false, portRefused: refusal.refused, refusalCode: refusal.code, port };
    report.failures.push(...assessCleanup(report.cleanup).map(name => ({ name })));
    report.finished = new Date().toISOString(); await fs.writeFile(path.join(root, 'receipt.json'), JSON.stringify(report, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  }
  if (report.failures.length) process.exitCode = 1;
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) {
  (process.argv[2] === '--execute' ? execute(process.argv[3]) : orchestrate()).catch(error => { console.error(error.stack || error.message); process.exitCode = 1; });
}
