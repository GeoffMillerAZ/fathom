// Bounded local proof for Gallery quality-work HTML. Synthetic data; no app scripts.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';

const [parentArg, binaryArg, cssArg] = process.argv.slice(2);
for (const [name, value] of [['evidence parent', parentArg], ['Gallery binary', binaryArg], ['expected CSS', cssArg]]) {
  assert.ok(value && path.isAbsolute(value), `${name} must be an absolute path`);
}
const parent = await fs.realpath(parentArg);
const binary = await fs.realpath(binaryArg);
const expectedCSS = await fs.realpath(cssArg);
const chromePath = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const playwrightPath = process.env.APPKIT_PLAYWRIGHT_MODULE;
assert.ok(playwrightPath && path.isAbsolute(playwrightPath), 'APPKIT_PLAYWRIGHT_MODULE must be absolute');

const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const writeNew = (name, bytes) => fs.writeFile(path.join(root, name), bytes, {flag: 'wx', mode: 0o600});
const sourcePath = fileURLToPath(import.meta.url);
const [sourceBytes, binaryBytes, expectedCSSBytes, playwrightBytes] = await Promise.all([
  fs.readFile(sourcePath), fs.readFile(binary), fs.readFile(expectedCSS), fs.readFile(playwrightPath),
]);
const playwrightSHA = sha(playwrightBytes);
assert.equal(playwrightSHA, 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f',
  'unapproved Playwright entry');
await fs.access(chromePath);

const root = await fs.mkdtemp(path.join(parent, 'gallery-quality-'));
await fs.chmod(root, 0o700);
await writeNew('probe.mjs', sourceBytes);
await writeNew('gallery', binaryBytes);
await fs.chmod(path.join(root, 'gallery'), 0o700);
await writeNew('expected-app.css', expectedCSSBytes);
const config = {
  kind: 'gallery-quality-probe-config-v1',
  binaryInput: binary,
  expectedCSSInput: expectedCSS,
  chromePath,
  playwrightPath,
  server: {args: ['-addr', '127.0.0.1:0', '-design-assets', ''], exactLoopbackGETOnly: true},
  lanes: [
    {width: 320, theme: 'light'}, {width: 320, theme: 'dark'},
    {width: 1440, theme: 'light'}, {width: 1440, theme: 'dark'},
  ],
  context: {height: 1000, javaScriptEnabled: false, reducedMotion: 'reduce', serviceWorkers: 'block'},
};
const configBytes = Buffer.from(JSON.stringify(config, null, 2) + '\n');
await writeNew('run-config.json', configBytes);

const {chromium} = await import(pathToFileURL(playwrightPath));
const report = {
  kind: 'gallery-quality-browser-proof-v1',
  verification: 'bounded-synthetic-not-certification',
  root,
  startedAt: new Date().toISOString(),
  inputs: {
    harness: {inputPath: sourcePath, retainedFile: 'probe.mjs', sha256: sha(sourceBytes)},
    config: {retainedFile: 'run-config.json', sha256: sha(configBytes)},
    binary: {inputPath: binary, retainedFile: 'gallery', sha256: sha(binaryBytes)},
    expectedCSS: {inputPath: expectedCSS, retainedFile: 'expected-app.css', sha256: sha(expectedCSSBytes)},
    playwrightEntry: {path: playwrightPath, sha256: playwrightSHA},
    chrome: {path: chromePath},
  },
  checks: [], failures: [], pages: [], captures: [], requests: [],
  limits: [
    'Local synthetic Chrome proof of Gallery quality-work HTML only; not assistive-technology, physical touch, browser parity, consumer acceptance, or certification.',
    'The embedded queue is authored planning. This probe does not execute gates, compute readiness, certify promotion, or authenticate referenced evidence.',
    'Binary, expected and served CSS, harness, run configuration, HTML responses, Playwright entry, Chrome version, requests, logs, and outcomes are retained.',
  ],
};
function check(name, passed, detail = {}) {
  report.checks.push({name, passed, detail});
  if (!passed) report.failures.push({name, detail});
}
async function bounded(promise, ms, label) {
  let timer;
  try {
    return await Promise.race([promise, new Promise((_, reject) => {
      timer = setTimeout(() => reject(Error(`${label} timed out after ${ms}ms`)), ms);
    })]);
  } finally { clearTimeout(timer); }
}
async function tabTo(page, selector) {
  for (let i = 0; i < 120; i++) {
    if (await page.evaluate(s => document.activeElement?.matches(s) === true, selector)) return;
    await page.keyboard.press('Tab');
  }
  throw Error(`native Tab did not reach ${selector}`);
}
async function pressEnterAndWait(page, selector, urlPredicate) {
  await tabTo(page, selector);
  check('keyboard focus visible', await page.evaluate(() => {
    const style = getComputedStyle(document.activeElement);
    return style.outlineStyle !== 'none' && parseFloat(style.outlineWidth) >= 2;
  }), {url: page.url(), selector});
  const navigation = page.waitForURL(urlPredicate, {waitUntil: 'load'});
  await page.keyboard.press('Enter');
  await navigation;
}

let gallery;
let galleryClosed;
let server;
let chrome;
let chromeClosed;
let browser;
let origin;
let stdout = '';
let stderr = '';
const serverArgs = config.server.args;
const watchdog = setTimeout(() => {
  report.failures.push({name: 'watchdog', detail: '180 seconds'});
  try { server?.kill(); } catch {}
  try { gallery?.kill('SIGKILL'); } catch {}
}, 180000);

try {
  gallery = spawn(path.join(root, 'gallery'), serverArgs, {cwd: root, stdio: ['ignore', 'pipe', 'pipe']});
  galleryClosed = new Promise(resolve => gallery.once('close', resolve));
  gallery.stdout.on('data', bytes => { stdout += bytes.toString(); });
  origin = await bounded(new Promise((resolve, reject) => {
    gallery.once('error', reject);
    gallery.once('exit', () => reject(Error('Gallery exited before readiness')));
    gallery.stderr.on('data', bytes => {
      stderr += bytes.toString();
      const match = stderr.match(/app-kit Gallery → (http:\/\/127\.0\.0\.1:\d+)/);
      if (match) resolve(match[1]);
    });
  }), 15000, 'Gallery readiness');
  report.origin = origin;

  server = await chromium.launchServer({
    headless: true,
    executablePath: chromePath,
    args: ['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1'],
  });
  chrome = server.process();
  chromeClosed = new Promise(resolve => chrome.once('exit', resolve));
  browser = await chromium.connect(server.wsEndpoint());
  report.inputs.chrome.version = browser.version();

  for (const lane of config.lanes) {
    const context = await browser.newContext({
      viewport: {width: lane.width, height: config.context.height},
      colorScheme: lane.theme,
      reducedMotion: config.context.reducedMotion,
      javaScriptEnabled: config.context.javaScriptEnabled,
      serviceWorkers: config.context.serviceWorkers,
    });
    await context.route('**/*', route => {
      const request = route.request();
      const url = new URL(request.url());
      const admitted = url.origin === origin && request.method() === 'GET' &&
        (url.pathname === '/static/app.css' || url.pathname === '/quality' ||
          /^\/quality\/[a-z][a-z0-9._-]{0,95}$/.test(url.pathname));
      report.requests.push({lane, method: request.method(), url: url.href, admitted});
      if (admitted) return route.continue();
      check('request allowlist', false, {lane, method: request.method(), url: url.href});
      return route.abort();
    });
    const page = await context.newPage();
    page.setDefaultTimeout(7000);
    page.on('pageerror', error => check('page error', false, {lane, error: String(error)}));
    page.on('requestfailed', request => check('request completed', false,
      {lane, url: request.url(), error: request.failure()}));
    let cssResponses = [];
    page.on('response', response => {
      if (response.url() === origin + '/static/app.css') cssResponses.push(response);
    });

    async function visit(target, stage, expectedStatus = 200) {
      cssResponses = [];
      const response = await page.goto(origin + target, {waitUntil: 'load'});
      assert.ok(response, 'document response missing');
      const body = await response.body();
      check('document HTTP status', response.status() === expectedStatus,
        {lane, stage, status: response.status(), expectedStatus, url: response.url()});
      check('exact loopback document GET', response.request().method() === 'GET' &&
        new URL(response.url()).origin === origin, {lane, stage, url: response.url()});
      for (const cssResponse of cssResponses) {
        const served = await cssResponse.body();
        check('served CSS exact identity', sha(served) === report.inputs.expectedCSS.sha256,
          {lane, stage, sha256: sha(served), expected: report.inputs.expectedCSS.sha256});
        if (!report.inputs.servedCSS) {
          await writeNew('served-app.css', served);
          report.inputs.servedCSS = {retainedFile: 'served-app.css', sha256: sha(served), bytes: served.length};
        }
      }
      const file = `${lane.width}-${lane.theme}-${stage}.html`;
      await writeNew(file, body);
      report.pages.push({lane, stage, file, sha256: sha(body), bytes: body.length,
        status: response.status(), url: response.url()});
      check('JavaScript disabled and page script-free', await page.locator('script').count() === 0,
        {lane, stage});
      return response;
    }
    async function capture(stage) {
      const file = `${lane.width}-${lane.theme}-${stage}.png`;
      const png = await page.screenshot({fullPage: true, animations: 'disabled'});
      await writeNew(file, png);
      report.captures.push({lane, stage, file, sha256: sha(png), bytes: png.length});
    }
    async function inspect(stage) {
      const metrics = await page.evaluate(() => {
        const root = document.documentElement;
        const targets = [...document.querySelectorAll('main a, main button, main input:not([type="hidden"]), main select, main summary')]
          .filter(element => element.checkVisibility()).map(element => {
            const rect = element.getBoundingClientRect();
            return {tag: element.tagName, text: element.textContent?.trim() || element.getAttribute('aria-label') || '',
              width: rect.width, height: rect.height};
          });
        return {viewport: innerWidth, scrollWidth: root.scrollWidth, clientWidth: root.clientWidth,
          theme: root.dataset.theme, targets};
      });
      check('no viewport horizontal overflow',
        metrics.scrollWidth <= metrics.clientWidth && metrics.clientWidth === metrics.viewport,
        {lane, stage, ...metrics});
      check('theme preserved', metrics.theme === lane.theme, {lane, stage, actual: metrics.theme});
      for (const target of metrics.targets) {
        check('visible standalone target own box at least 44px',
          target.width >= 44 && target.height >= 44, {lane, stage, ...target});
      }
    }
    async function setSelectWithKeyboard(selector, firstLetter) {
      await tabTo(page, selector);
      await page.keyboard.press(firstLetter);
      await page.keyboard.press('Tab');
    }
    async function submitFilter(expected) {
      await pressEnterAndWait(page, 'form[aria-label="Quality work filters"] button',
        url => expected(url));
    }

    await visit(`/quality?scope=core&theme=${lane.theme}`, 'list');
    check('quality list heading', await page.locator('main h1').textContent() === 'Quality work', {lane});
    check('core scope selected', await page.locator('#quality-scope').inputValue() === 'core', {lane});
    const results = 'section[aria-labelledby="quality-results"]';
    const coreKinds = await page.locator(`${results} article`).allTextContents();
    check('core scope contains only core work', coreKinds.length > 0 && coreKinds.every(text => text.includes('Core ·')),
      {lane, count: coreKinds.length});
    await inspect('list');
    await capture('list');

    // Native keyboard changes scope, enters an exact capability, and submits.
    await setSelectWithKeyboard('#quality-scope', 'a');
    await tabTo(page, '#quality-capability');
    await page.keyboard.type('analytics.comparison');
    await page.keyboard.press('Tab');
    await submitFilter(url => url.pathname === '/quality' && url.searchParams.get('scope') === 'all' &&
      url.searchParams.get('capability_id') === 'analytics.comparison' &&
      url.searchParams.get('theme') === lane.theme);
    await visit(new URL(page.url()).pathname + new URL(page.url()).search, 'exact-capability');
    const exactArticles = page.locator(`${results} article`);
    check('exact capability filter', await exactArticles.count() === 1 &&
      (await exactArticles.first().textContent()).includes('core.analytics'), {lane});

    const detailLink = `${results} article h3 a`;
    await pressEnterAndWait(page, detailLink, url => url.pathname === '/quality/core.analytics');
    await visit(new URL(page.url()).pathname + new URL(page.url()).search, 'detail');
    await inspect('detail');
    const standards = page.locator('section[aria-labelledby="quality-standards"] article');
    const gates = page.locator('section[aria-labelledby="quality-gates"] article');
    check('seven enhancement standards', await standards.count() === 7, {lane, count: await standards.count()});
    check('nine recorded gates', await gates.count() === 9, {lane, count: await gates.count()});
    for (const [kind, entries] of [['standard', standards], ['gate', gates]]) {
      const rows = await entries.evaluateAll(elements => elements.map(element => {
        const text = element.textContent || '';
        return {heading: element.querySelector('h3')?.textContent?.trim() || '',
          reason: /Reason:\s*\S/.test(text), nextStep: /Next step:\s*\S/.test(text)};
      }));
      check(`${kind} statuses rendered`, rows.every(row =>
        /:\s*(complete|partial|not-started|not-run|passed|failed|blocked)$/i.test(row.heading)), {lane, rows});
      check(`${kind} reasons represented`, rows.some(row => row.reason), {lane, rows});
      check(`${kind} next steps rendered`, rows.every(row => row.nextStep), {lane, rows});
    }
    const detailText = (await page.locator('main').textContent()).toLowerCase();
    check('no readiness score or certification promotion',
      !/(readiness score|ready score|certified|certification achieved|promotion (?:ready|approved|granted))/.test(detailText),
      {lane});
    const back = page.locator('main > a[href^="/quality"]');
    const backURL = new URL(await back.getAttribute('href'), origin);
    check('detail back preserves filter and theme', backURL.pathname === '/quality' &&
      backURL.searchParams.get('scope') === 'all' &&
      backURL.searchParams.get('capability_id') === 'analytics.comparison' &&
      backURL.searchParams.get('theme') === lane.theme &&
      !backURL.searchParams.has('expected_revision'), {lane, href: backURL.href});

    const evidenceSummary = 'section[aria-labelledby="quality-standards"] details summary, section[aria-labelledby="quality-gates"] details summary';
    check('evidence disclosure exists', await page.locator(evidenceSummary).count() > 0, {lane});
    if (await page.locator(evidenceSummary).count() > 0) {
      await tabTo(page, evidenceSummary);
      await page.keyboard.press('Enter');
      check('native evidence disclosure opens', await page.evaluate(selector =>
        document.activeElement?.matches(selector) === true && document.activeElement.closest('details')?.open === true,
      evidenceSummary), {lane});
    }
    await capture('detail');
    await pressEnterAndWait(page, 'main > a[href^="/quality"]', url => url.pathname === '/quality');
    check('keyboard back preserves filter', new URL(page.url()).searchParams.get('capability_id') ===
      'analytics.comparison', {lane, url: page.url()});

    await visit(`/quality?scope=on-demand&theme=${lane.theme}`, 'on-demand');
    const onDemandKinds = await page.locator(`${results} article`).allTextContents();
    check('on-demand scope excludes core work', onDemandKinds.length > 0 &&
      onDemandKinds.every(text => text.includes('On-demand ·')), {lane, count: onDemandKinds.length});
    await inspect('on-demand');

    if (lane.width === 320) {
      await visit(`/quality?scope=all&theme=${lane.theme}`, 'all-recorded');
      const allIDs = await page.locator(`${results} article h3 a`).allTextContents();
      check('all recorded IDs rendered in narrow lane', [
        'core.shell-settings',
        'core.analytics',
        'core.static-content',
        'core.client-heavy-boundary',
        'core.themes-style-guide',
        'candidate.personal-site-series-navigation',
      ].every(id => allIDs.includes(id)), {lane, allIDs});
      await inspect('all-recorded');
      for (const id of ['core.client-heavy-boundary', 'candidate.personal-site-series-navigation']) {
        await visit(`/quality/${id}?scope=all&theme=${lane.theme}`, `detail-${id}`);
        check('long-ID detail identity', (await page.locator('main h1').textContent()).trim() === id,
          {lane, id});
        await inspect(`detail-${id}`);
      }
    }

    await visit(`/quality?scope=core&capability_id=analytics.timeline&theme=${lane.theme}`, 'known-empty');
    check('known empty state', await page.locator(`${results} article`).count() === 0 &&
      (await page.locator(results).textContent()).includes('No recorded work matches'), {lane});

    for (const problem of [
      {stage: 'unknown', path: `/quality?scope=core&capability_id=not.real&theme=${lane.theme}`, status: 400},
      {stage: 'duplicate', path: `/quality?scope=core&scope=all&theme=${lane.theme}`, status: 400},
      {stage: 'stale', path: `/quality/core.analytics?scope=all&expected_revision=${'0'.repeat(64)}&theme=${lane.theme}`, status: 400},
    ]) {
      await visit(problem.path, problem.stage, problem.status);
      check(`${problem.stage} problem offers reset`,
        await page.locator('main a[href="/quality"]').count() === 1 &&
        (await page.locator('main').textContent()).includes('needs a reset'), {lane});
    }
    await context.close();
  }

  check('approximately eight captures', report.captures.length === 8, {count: report.captures.length});
  check('served CSS observed', Boolean(report.inputs.servedCSS), report.inputs.servedCSS || {});
  assert.equal(sha(await fs.readFile(sourcePath)), report.inputs.harness.sha256, 'source harness drift');
  assert.equal(sha(await fs.readFile(binary)), report.inputs.binary.sha256, 'input binary drift');
  assert.equal(sha(await fs.readFile(expectedCSS)), report.inputs.expectedCSS.sha256, 'input CSS drift');
  assert.equal(sha(await fs.readFile(playwrightPath)), report.inputs.playwrightEntry.sha256, 'Playwright entry drift');
} catch (error) {
  report.failures.push({name: 'execution', detail: String(error.stack || error)});
} finally {
  if (server) {
    try { await bounded(server.close(), 7000, 'Chrome close'); }
    catch (error) {
      report.failures.push({name: 'Chrome close', detail: String(error)});
      try { server.kill(); } catch {}
    }
  }
  if (chrome) {
    await bounded(chromeClosed, 3000, 'Chrome exit').catch(error =>
      report.failures.push({name: 'Chrome exit wait', detail: String(error)}));
    report.chromeProcess = {pid: chrome.pid, exitCode: chrome.exitCode, signalCode: chrome.signalCode};
    check('Chrome normal exit', chrome.exitCode === 0 && chrome.signalCode === null, report.chromeProcess);
  }
  if (gallery) {
    gallery.kill('SIGTERM');
    try { await bounded(galleryClosed, 8000, 'Gallery close'); }
    catch (error) {
      report.failures.push({name: 'Gallery close', detail: String(error)});
      gallery.kill('SIGKILL');
      await bounded(galleryClosed, 3000, 'Gallery kill').catch(() => {});
    }
    report.galleryProcess = {pid: gallery.pid, args: serverArgs, exitCode: gallery.exitCode,
      signalCode: gallery.signalCode};
    check('Gallery normal exit', gallery.exitCode === 0 && gallery.signalCode === null,
      report.galleryProcess);
  }
  clearTimeout(watchdog);
  await writeNew('gallery.stdout', stdout);
  await writeNew('gallery.stderr', stderr);
  report.finishedAt = new Date().toISOString();
  report.passed = report.failures.length === 0;
  await writeNew('receipt.json', Buffer.from(JSON.stringify(report, null, 2) + '\n'));
}

console.log(JSON.stringify({
  root, passed: report.passed, checks: report.checks.length, failures: report.failures.length,
  captures: report.captures.length, chrome: report.chromeProcess, gallery: report.galleryProcess,
}, null, 2));
process.exitCode = report.passed ? 0 : 1;
