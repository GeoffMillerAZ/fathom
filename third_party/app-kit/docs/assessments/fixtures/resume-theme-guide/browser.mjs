import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import {
  chmodSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  realpathSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';

const playwrightPath = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
const playwrightSHA256 = 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f';
const chromePath = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const hash = bytes => createHash('sha256').update(bytes).digest('hex');

function argumentsOf(argv) {
  const result = {};
  for (let i = 0; i < argv.length; i += 2) {
    if (!['--url', '--out'].includes(argv[i]) || !argv[i + 1]) throw new Error('Usage: browser.mjs --url URL --out EMPTY_DIRECTORY');
    result[argv[i].slice(2)] = argv[i + 1];
  }
  if (!result.url || !result.out || Object.keys(result).length !== 2) throw new Error('Usage: browser.mjs --url URL --out EMPTY_DIRECTORY');
  return result;
}

const selected = argumentsOf(process.argv.slice(2));
const guide = new URL(selected.url);
if (guide.protocol !== 'http:' || guide.hostname !== '127.0.0.1' || !guide.port || guide.pathname !== '/theme-guide' || guide.search || guide.hash || guide.username || guide.password || guide.href !== selected.url) {
  throw new Error('Select the canonical loopback theme-guide URL.');
}
if (!existsSync(dirname(selected.out))) throw new Error('Output parent must already exist.');
if (!existsSync(selected.out)) mkdirSync(selected.out, { mode: 0o700 });
const output = realpathSync(selected.out);
if (!statSync(output).isDirectory() || readdirSync(output).length !== 0) throw new Error('Output directory must be empty.');
chmodSync(output, 0o700);
if (hash(readFileSync(playwrightPath)) !== playwrightSHA256) throw new Error('Approved Playwright identity differs.');
if (!statSync(chromePath).isFile()) throw new Error('Approved Chrome executable is unavailable.');

const { chromium } = await import('file://' + playwrightPath);
const profile = mkdtempSync(join(tmpdir(), 'appkit-rac-theme-guide-profile-'));
chmodSync(profile, 0o700);
const report = {
  schema_version: 1,
  origin: guide.origin,
  guide: guide.href,
  probe_sha256: hash(readFileSync(new URL(import.meta.url))),
  playwright: { path: playwrightPath, sha256: playwrightSHA256 },
  chrome: { path: chromePath },
  checks: [], requests: [], responses: [], blocked: [], request_failures: [], download_aborts: [], errors: [], console: [], screenshots: [], downloads: [],
  limits: [
    'Synthetic RAC theme-guide tasks only; no config, registries, credentials, pollers or real records.',
    'Fresh headless Chrome with JavaScript disabled; native forms and keyboard actions, not physical input or assistive-technology certification.',
    'Two viewport widths and nine specimen states; not every browser, zoom level, theme token or whole-app route.',
    'Page evaluation reads focus, attributes, computed styles and geometry only; it never mutates the DOM or bypasses CSP.',
  ],
};
const check = (name, pass, detail) => report.checks.push({ name, pass: Boolean(pass), detail });
const expected = [];
const requireCheck = (name, pass, detail) => { expected.push(name); check(name, pass, detail); };
const pending = new Set();
let context;
let page;
let submission = 0;

async function boundedBody(response) {
  let timer;
  try {
    return await Promise.race([
      response.body(),
      new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('Response body deadline exceeded')), 10_000); }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}

async function drain() {
  while (pending.size) await Promise.allSettled([...pending]);
}

async function screenshot(name) {
  const path = join(output, name + '.png');
  await page.screenshot({ path, fullPage: false });
  report.screenshots.push({ name, path, sha256: hash(readFileSync(path)) });
}

async function openEditorSection(name) {
  const summary = page.locator('summary').filter({ hasText: name }).first();
  const details = summary.locator('..');
  if (!await details.evaluate(element => element.open)) await summary.click();
  if (!await details.evaluate(element => element.open)) throw new Error(`${name} editor section did not open.`);
}

async function submit(buttonName, status) {
  const [response] = await Promise.all([
    page.waitForResponse(r => r.request().method() === 'POST' && new URL(r.url()).pathname === '/theme-guide'),
    page.waitForNavigation({ waitUntil: 'networkidle' }),
    page.getByRole('button', { name: buttonName, exact: true }).click(),
  ]);
  submission++;
  requireCheck(`${buttonName} submission ${submission} returns ${status}`, response.status() === status, response.status());
  return response;
}

async function previewFrame() {
  const frame = page.frames().find(candidate => candidate !== page.mainFrame() && new URL(candidate.url()).pathname === '/theme-guide/preview');
  if (!frame) throw new Error('Preview frame did not load.');
  await frame.locator('body').waitFor();
  return frame;
}

async function actualThemeUsage(frame) {
  const font = frame.locator('.font-sans:visible').first();
  const radius = frame.locator('.rounded-lg:visible').first();
  const accent = frame.locator('.bg-accent:visible').first();
  const counts = await Promise.all([font.count(), radius.count(), accent.count()]);
  if (counts.some(count => count === 0)) throw new Error('Preview lacks a visible font-sans, rounded-lg or bg-accent theme consumer.');
  return {
    fontFamily: await font.evaluate(element => getComputedStyle(element).fontFamily),
    borderRadius: await radius.evaluate(element => getComputedStyle(element).borderRadius),
    backgroundColor: await accent.evaluate(element => getComputedStyle(element).backgroundColor),
    foregroundColor: await accent.evaluate(element => getComputedStyle(element).color),
  };
}

async function computedCandidate(scheme) {
  await page.locator('[name="scheme"]').selectOption(scheme);
  await submit('Preview candidate', 200);
  const frame = await previewFrame();
  const values = await frame.locator('html').evaluate(element => {
    const style = getComputedStyle(element);
    return {
      font: style.getPropertyValue('--ak-font-sans').trim(),
      radius: style.getPropertyValue('--ak-radius-lg').trim(),
      accent: style.getPropertyValue('--ak-color-accent').trim().toLowerCase(),
      accentInk: style.getPropertyValue('--ak-color-accent-ink').trim().toLowerCase(),
      inkFaint: style.getPropertyValue('--ak-color-ink-faint').trim().toLowerCase(),
    };
  });
  requireCheck(`${scheme} candidate computes approved token edits`, values.font === 'ui-monospace, monospace' && values.radius === '1rem' && values.accent === '#b69aff' && values.accentInk === '#180f2e' && values.inkFaint === '#c9bfe8', values);
  const usage = await actualThemeUsage(frame);
  requireCheck(`${scheme} candidate applies the mono font to visible content`, usage.fontFamily === 'ui-monospace, monospace', usage);
  requireCheck(`${scheme} candidate applies the 16px radius to a visible rounded-lg element`, usage.borderRadius === '16px', usage);
  requireCheck(`${scheme} candidate applies readable accent and accent-ink to a visible action`, usage.backgroundColor === 'rgb(182, 154, 255)' && usage.foregroundColor === 'rgb(24, 15, 46)', usage);
  return frame;
}

async function run() {
  context = await chromium.launchPersistentContext(profile, {
    executablePath: chromePath,
    headless: true,
    javaScriptEnabled: false,
    acceptDownloads: true,
    viewport: { width: 1440, height: 1000 },
    reducedMotion: 'reduce',
    args: ['--disable-background-networking', '--no-proxy-server', '--no-first-run', '--no-default-browser-check'],
  });
  report.browser = context.browser()?.version();
  context.setDefaultTimeout(6_000);
  context.setDefaultNavigationTimeout(15_000);
  context.on('serviceworker', worker => report.errors.push('Service worker: ' + worker.url()));
  await context.routeWebSocket('**/*', socket => { report.blocked.push({ method: 'WS', url: socket.url() }); socket.close(); });
  await context.route('**/*', route => {
    const request = route.request();
    const url = new URL(request.url());
    const allowedPath = ['/theme-guide', '/theme-guide/style.css', '/theme-guide/preview', '/theme-guide/app.css', '/theme-guide/brand.css'].includes(url.pathname);
    const allowedMethod = request.method() === 'GET' || (request.method() === 'POST' && url.pathname === '/theme-guide');
    if (url.origin !== guide.origin || !allowedPath || !allowedMethod) {
      report.blocked.push({ method: request.method(), url: request.url() });
      return route.abort('blockedbyclient');
    }
    report.requests.push({ method: request.method(), path: url.pathname, origin: request.headers().origin });
    return route.continue();
  });
  page = context.pages()[0] ?? await context.newPage();
  page.on('pageerror', error => report.errors.push(String(error)));
  page.on('requestfailed', request => {
    const failure = { method: request.method(), url: request.url(), error: request.failure()?.errorText };
    if (request.method() === 'POST' && new URL(request.url()).pathname === '/theme-guide' && request.postData()?.includes('action=export') && failure.error === 'net::ERR_ABORTED') report.download_aborts.push(failure);
    else report.request_failures.push(failure);
  });
  page.on('console', message => {
    if (message.type() === 'error' || message.type() === 'warning') report.console.push({ type: message.type(), text: message.text() });
  });
  page.on('response', response => {
    const task = (async () => {
      const headers = response.headers();
      if (headers['content-disposition']?.includes('attachment')) {
        report.responses.push({ url: response.url(), status: response.status(), attachment: true, headers });
        return;
      }
      const body = await boundedBody(response);
      report.responses.push({ url: response.url(), status: response.status(), bytes: body.length, sha256: hash(body), headers });
    })();
    pending.add(task);
    task.catch(error => report.errors.push(String(error))).finally(() => pending.delete(task));
  });

  const initial = await page.goto(guide.href, { waitUntil: 'networkidle' });
  requireCheck('guide loads over selected loopback origin', initial?.status() === 200, initial?.status());
  requireCheck('guide runs without page JavaScript', await page.locator('script').count() === 0, await page.locator('script').count());
  requireCheck('preview uses an opaque sandbox', await page.locator('iframe').getAttribute('sandbox') === '', await page.locator('iframe').getAttribute('sandbox'));
  const contrastRows = () => page.locator('h2#sg-checks').locator('xpath=following-sibling::div//tbody/tr');
  const baselineFailures = await contrastRows().evaluateAll(rows => rows.filter(row => row.textContent.includes('Needs review')).map(row => row.querySelector('th').textContent).sort());
  requireCheck('source exposes all six selected contrast failures', JSON.stringify(baselineFailures) === JSON.stringify(['action-dark', 'action-light', 'badge-accent-dark', 'badge-accent-light', 'table-secondary-dark', 'table-secondary-light']), baselineFailures);

  await openEditorSection('Corner radii');
  const radius = page.getByLabel('radius.lg', { exact: true });
  await radius.fill('-1px');
  await submit('Preview candidate', 422);
  requireCheck('invalid radius focuses the native error summary', await page.getByRole('alert').evaluate(element => element === document.activeElement), await page.locator(':focus').getAttribute('id'));
  requireCheck('invalid radius remains available for repair', await radius.inputValue() === '-1px' && await radius.getAttribute('aria-invalid') === 'true', await radius.inputValue());
  await screenshot('invalid-radius-1440');
  const repair = page.getByRole('link', { name: 'Fix radius.lg', exact: true });
  await repair.focus();
  await page.keyboard.press('Enter');
  requireCheck('native repair link focuses radius.lg', await radius.evaluate(element => element === document.activeElement), await page.locator(':focus').getAttribute('name'));
  await radius.fill('1rem');

  await openEditorSection('Typography');
  await page.locator('[name="fonts.sans"]').selectOption('system-mono');
  await openEditorSection('Color roles');
  for (const name of ['colors.ink-faint.light', 'colors.ink-faint.dark']) await page.getByLabel(name, { exact: true }).fill('#c9bfe8');
  for (const name of ['colors.accent.light', 'colors.accent.dark']) await page.getByLabel(name, { exact: true }).fill('#b69aff');
  for (const name of ['colors.accent-ink.light', 'colors.accent-ink.dark']) await page.getByLabel(name, { exact: true }).fill('#180f2e');

  await computedCandidate('light');
  const allChecks = await contrastRows().allTextContents();
  requireCheck('corrected candidate passes every configured contrast check', allChecks.length === 12 && allChecks.every(text => text.includes('Pass') && !text.includes('Needs review')), allChecks);
  await computedCandidate('dark');
  await screenshot('candidate-dark-1440');

  const specimenIDs = ['today-populated', 'today-empty', 'today-incomplete', 'pipeline-populated', 'pipeline-empty', 'pipeline-incomplete', 'people-populated', 'people-empty', 'people-incomplete'];
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.locator('select[name="viewport"]').selectOption(width === 390 ? 'narrow' : 'wide');
    for (const specimen of specimenIDs) {
      await page.locator('[name="specimen"]').selectOption(specimen);
      await submit('Preview candidate', 200);
      const frame = await previewFrame();
      const facts = await frame.locator('body').evaluate(body => ({
        text: body.innerText.slice(0, 240),
        width: document.documentElement.clientWidth,
        scroll: document.documentElement.scrollWidth,
        scripts: document.scripts.length,
        anchors: [...document.querySelectorAll('a')].map(anchor => ({ inert: anchor.hasAttribute('inert'), href: anchor.getAttribute('href') })),
      }));
      requireCheck(`${width} ${specimen} renders synthetic bounded specimen`, facts.text.includes('Synthetic theme specimen') && facts.scripts === 0 && facts.scroll <= facts.width + 1, facts);
      requireCheck(`${width} ${specimen} marks every specimen link inert`, facts.anchors.length > 0 && facts.anchors.every(anchor => anchor.inert), facts.anchors);
      if (specimen === 'pipeline-populated') {
        const disclosure = frame.locator('summary').filter({ hasText: 'All current-stage counts' });
        const disclosureCount = await disclosure.count();
        requireCheck(`${width} names exactly one pipeline counts disclosure`, disclosureCount === 1, disclosureCount);
        if (disclosureCount === 1) {
          await disclosure.focus();
          const disclosureFocus = await frame.locator('body').evaluate(() => ({ tag: document.activeElement?.tagName, text: document.activeElement?.textContent?.trim().slice(0, 80) }));
          requireCheck(`${width} pipeline disclosure accepts native keyboard focus`, await disclosure.evaluate(element => element === document.activeElement), disclosureFocus);
          await page.keyboard.press('Enter');
          const details = disclosure.locator('..');
          const disclosureFacts = {
            open: await details.evaluate(element => element.open),
            captionVisible: await details.getByText('Every tracked application, including closed stages.', { exact: true }).isVisible(),
            tableRows: await details.locator('tbody tr').count(),
          };
          requireCheck(`${width} pipeline counts disclosure opens and reveals analytics natively`, disclosureFacts.open && disclosureFacts.captionVisible && disclosureFacts.tableRows > 0, disclosureFacts);
        }
      }
      if (specimen === 'today-empty') {
        const anchor = frame.locator('a[href="/pipeline"]:visible').filter({ hasText: 'View pipeline' });
        const anchorCount = await anchor.count();
        requireCheck(`${width} selects the actual visible inert View pipeline link`, anchorCount === 1 && await anchor.isVisible() && await anchor.getAttribute('inert') === '', anchorCount);
        if (anchorCount !== 1) throw new Error('Expected exactly one visible inert View pipeline link.');
        const addSummary = frame.locator('summary').filter({ hasText: '+ Add' }).first();
        await addSummary.focus();
        const initialFocus = await frame.locator('body').evaluate(() => ({ tag: document.activeElement?.tagName, text: document.activeElement?.textContent?.trim().slice(0, 80) }));
        requireCheck(`${width} establishes native focus before inert-link Tab check`, await addSummary.evaluate(element => element === document.activeElement), initialFocus);
        await page.keyboard.press('Tab');
        const tabFocus = await frame.locator('body').evaluate(() => {
          const active = document.activeElement;
          return { tag: active?.tagName, href: active?.getAttribute?.('href'), text: active?.textContent?.trim().slice(0, 80) };
        });
        requireCheck(`${width} visible inert View pipeline link is skipped by native Tab`, !await anchor.evaluate(element => element === document.activeElement) && tabFocus.href !== '/pipeline', tabFocus);
        const before = frame.url();
        const box = await anchor.boundingBox();
        if (!box) throw new Error('Visible inert pipeline link has no clickable bounds.');
        await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
        await page.waitForTimeout(150);
        requireCheck(`${width} visible inert View pipeline link cannot navigate`, frame.url() === before, { before, after: frame.url() });
      }
    }
    await screenshot(`candidate-${width}`);
  }

  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.getByLabel('radius.lg', { exact: true }).fill('0.75rem');
  await submit('Export acknowledged candidate ZIP', 409);
  requireCheck('stale export is refused and acknowledgement cleared', (await page.locator('[name="review_digest"]').inputValue()) === '' && await page.getByRole('button', { name: 'Export acknowledged candidate ZIP', exact: true }).isDisabled());
  await page.getByLabel('radius.lg', { exact: true }).fill('1rem');
  await submit('Preview candidate', 200);
  const digest = await page.locator('[name="review_digest"]').inputValue();
  requireCheck('current candidate has a fresh acknowledgement', /^[0-9a-f]{64}$/.test(digest), digest);

  const downloadPromise = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Export acknowledged candidate ZIP', exact: true }).click();
  const download = await downloadPromise;
  const zipPath = join(output, 'app-theme-candidate.zip');
  await download.saveAs(zipPath);
  if (statSync(zipPath).size > 1 << 20) throw new Error('Export exceeds the bounded ZIP inspection limit');
  const zip = readFileSync(zipPath);
  const downloadFailure = await download.failure();
  requireCheck('current acknowledged candidate exports a bounded ZIP', download.suggestedFilename() === 'app-theme-candidate.zip' && zip.length > 0 && zip.length <= 1 << 20 && downloadFailure === null, { filename: download.suggestedFilename(), bytes: zip.length, failure: downloadFailure });
  const unzipLimits = { timeout: 5000, maxBuffer: 64 * 1024 };
  execFileSync('/usr/bin/unzip', ['-tq', zipPath], unzipLimits);
  const zipEntries = execFileSync('/usr/bin/unzip', ['-Z1', zipPath], { ...unzipLimits, encoding: 'utf8' }).trim().split('\n').sort();
  requireCheck('export ZIP has the complete bounded review bundle', JSON.stringify(zipEntries) === JSON.stringify(['candidate.css', 'candidate.cue', 'diff.json', 'original.cue', 'receipt.json']), zipEntries);
  report.downloads.push({ path: zipPath, bytes: zip.length, sha256: hash(zip) });
  requireCheck('download navigation has one accounted browser abort', report.download_aborts.length === 1, report.download_aborts);

  await page.getByRole('button', { name: 'Show source', exact: true }).click();
  await page.waitForLoadState('networkidle');
  requireCheck('source view preserves the acknowledged candidate edits', await page.getByRole('heading', { name: 'Source baseline', exact: true }).count() === 1 && await page.getByLabel('radius.lg', { exact: true }).inputValue() === '1rem' && await page.locator('[name="review_digest"]').inputValue() === digest);
  const sourceUsage = await actualThemeUsage(await previewFrame());
  requireCheck('source view restores visible source theme usage', sourceUsage.fontFamily !== 'ui-monospace, monospace' && sourceUsage.borderRadius === '8px' && sourceUsage.backgroundColor === 'rgb(124, 77, 255)', sourceUsage);
  await Promise.all([page.waitForNavigation({ waitUntil: 'networkidle' }), page.getByRole('link', { name: 'Reset to source', exact: true }).click()]);
  requireCheck('reset restores source and clears candidate state', await page.locator('[name="radius.lg"]').inputValue() === '' && await page.locator('[name="fonts.sans"]').inputValue() === '' && await page.locator('[name="review_digest"]').inputValue() === '');
  const resetUsage = await actualThemeUsage(await previewFrame());
  requireCheck('reset preserves visible source theme usage', JSON.stringify(resetUsage) === JSON.stringify(sourceUsage), { source: sourceUsage, reset: resetUsage });
  await screenshot('source-reset-1440');
}

try {
  let timer;
  try {
    await Promise.race([
      run(),
      new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('Probe exceeded 120-second deadline')), 120_000); }),
    ]);
  } finally {
    clearTimeout(timer);
  }
} catch (error) {
  report.failure = String(error?.stack ?? error);
  if (page) await screenshot('failure').catch(() => {});
} finally {
  await drain();
  if (context) await context.close().catch(error => report.errors.push('Browser cleanup: ' + String(error)));
  await drain();
  try { rmSync(profile, { recursive: true, force: false }); }
  catch (error) { report.errors.push('Profile cleanup: ' + String(error)); }
  requireCheck('no out-of-scope request occurred', report.blocked.length === 0, report.blocked);
  requireCheck('all requested resources completed', report.request_failures.length === 0, report.request_failures);
  requireCheck('no browser execution error occurred', report.errors.length === 0, report.errors);
  const expectedHTTP = report.console.filter(entry => entry.type === 'error' && (entry.text.includes('422 (Unprocessable Entity)') || entry.text.includes('409 (Conflict)')));
  const sandboxDiagnostics = report.console.filter(entry => entry.type === 'error' && entry.text.includes("document's frame is sandboxed") && entry.text.includes("'allow-scripts' permission is not set"));
  requireCheck('only expected refusal and sandbox-inspection console diagnostics occurred', report.console.length === expectedHTTP.length + sandboxDiagnostics.length && expectedHTTP.length === 2, report.console);
  requireCheck('response CSP blocks scripts and active specimen forms', report.responses.filter(entry => entry.headers['content-type']?.startsWith('text/html')).every(entry => entry.headers['content-security-policy']?.includes("script-src 'none'") && entry.headers['content-security-policy']?.includes("default-src 'none'")), report.responses.filter(entry => entry.headers['content-type']?.startsWith('text/html')).map(entry => ({ url: entry.url, csp: entry.headers['content-security-policy'] })));
  const assets = report.responses.filter(entry => ['/theme-guide/app.css', '/theme-guide/brand.css'].includes(new URL(entry.url).pathname));
  const assetPaths = [...new Set(assets.map(entry => new URL(entry.url).pathname))].sort();
  requireCheck('both opaque preview CSS assets return 200 as CSS', JSON.stringify(assetPaths) === JSON.stringify(['/theme-guide/app.css', '/theme-guide/brand.css']) && assets.every(entry => entry.status === 200 && entry.headers['content-type']?.startsWith('text/css')), assets.map(entry => ({ url: entry.url, status: entry.status, contentType: entry.headers['content-type'] })));
  requireCheck('exact assertion set completed once', report.checks.length === expected.length && new Set(expected).size === expected.length, { checks: report.checks.length, expected: expected.length, unique: new Set(expected).size });
  report.finished = new Date().toISOString();
  report.ok = !report.failure && report.checks.every(entry => entry.pass);
  writeFileSync(join(output, 'receipt.json'), JSON.stringify(report, null, 2) + '\n', { flag: 'wx' });
  console.log(JSON.stringify({ output, ok: report.ok, checks: report.checks.length, failed: report.checks.filter(entry => !entry.pass), failure: report.failure }));
  if (!report.ok) process.exitCode = 1;
}
