// Opt-in audit of a prepared private reference. No install, source mutation,
// save request, signed-in profile access, baseline acceptance or certification.
import { createHash } from 'node:crypto';
import { readFile, writeFile, mkdtemp } from 'node:fs/promises';
import { dirname, isAbsolute, join } from 'node:path';
import { tmpdir } from 'node:os';
import { pathToFileURL } from 'node:url';
import { verifyFontReference } from './font-reference.mjs';

const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const base = process.env.APPKIT_CATALYST_BROWSER_ORIGIN;
const project = process.env.APPKIT_CATALYST_FONT_REFERENCE_ROOT;
const modulePath = process.env.APPKIT_PLAYWRIGHT_MODULE;
if (!base || !modulePath || !isAbsolute(modulePath)) throw new Error('Select explicit APPKIT_CATALYST_BROWSER_ORIGIN and absolute existing APPKIT_PLAYWRIGHT_MODULE; nothing is installed.');
const origin = new URL(base);
if (origin.origin !== base || origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || !origin.port) throw new Error('Select exact HTTP 127.0.0.1 origin with explicit non-default port, no extra URL data.');
const fonts = verifyFontReference(project);
const sourcePins = [
  ['src/app/(app)/settings/address.tsx', '4c0bc2c0b9827c91d6cebe9dc3d4c3bd8c63d62154bafdded843a636ecb28b0f'],
  ['src/components/listbox.tsx', '15789fda48026a66b0ce6cdcd82583ec0ce75bedd8bdc1c5a43001e5813c0eac'],
  ['src/components/textarea.tsx', 'febb2af242693ab7e7b383350cb749243f4261759693e675c2b21cb412b81ac2'],
];
for (const [path, expected] of sourcePins) if (hash(await readFile(join(project, path))) !== expected) throw new Error(`Reviewed source changed: ${path}`);
const fontPins = new Map(fonts.files.map(f => [new URL(f.url, base).pathname, f]));
const dynamicRoutes = new Set(['/', '/settings', '/events', '/orders', '/events/1000', '/events/1001', '/events/1002', '/events/1003']);
function referencePath(path) {
  if (fontPins.has(path)) return join(project, 'public', path);
  if (/^\/_next\/static\/chunks\/[A-Za-z0-9_.~-]+\.(js|css)$/.test(path)) return join(project, '.next', path.slice('/_next/'.length));
  if (/^\/(flags|teams|users|events)\/[A-Za-z0-9_-]+\.(svg|jpg)$/.test(path)) return join(project, 'public', path);
  return null;
}
const root = await mkdtemp(join(tmpdir(), 'appkit-catalyst-browser-'));
console.log(`Private Catalyst browser evidence: ${root}`);
const report = { origin: base, started: new Date().toISOString(), project, sourcePins,
  font_metadata_sha256: fonts.metadata_sha256, probe_sha256: hash(await readFile(new URL(import.meta.url))),
  checks: [], views: [], errors: [], console: [], blocked: [], responses: [], limits: [
    'Selected source/font pins and observed static response agreement with selected prepared build; not whole-build reproducibility or server process attestation.',
    'Dynamic Next HTML/RSC responses have status observations only; streamed prefetch bodies are not awaited or hashed.',
    'Synthetic pre-submit behavior only; no save backend, browser zoom, complete accessibility, consumer adoption or conversion certification.' ] };
const check = (name, pass, detail) => report.checks.push({ name, pass: !!pass, detail });
const playwrightPackage = await readFile(join(dirname(modulePath), 'package.json'));
report.playwright = { module: modulePath, module_sha256: hash(await readFile(modulePath)), package_sha256: hash(playwrightPackage), version: JSON.parse(playwrightPackage).version };
report.font_verifier_sha256 = hash(await readFile(new URL('./font-reference.mjs', import.meta.url)));
const { chromium } = await import(pathToFileURL(modulePath).href);
let browser, phase = 'launch';
const pending = new Set();
async function drainResponses() { while (pending.size) await Promise.allSettled([...pending]); }
const expectedViews = ['p10-390-light', 'p10-390-dark', 'p10-1440-light', 'p10-1440-dark'];
const expectedChecks = [
  'advertised 240-character bio bound is enforced before save',
  'country choices retain the complete supplied set',
  'keyboard country selection closes and restores focus',
  'country change cannot submit a hidden stale region',
  'Reset restores the complete original compound form state',
  ...expectedViews.flatMap(v => [`${v}: no page-width overflow`, `${v}: used Inter weights loaded`, `${v}: no broken laid-out image`]),
  'all four requested views captured',
  'observed static responses agree with selected build and pinned fonts',
  'no outside or write request observed',
  'no page or response-processing error',
  'no browser console warning or error',
];
const formState = page => page.evaluate(() => ({ entries: [...new FormData(document.querySelector('form')).entries()],
  region: document.querySelector('[aria-label="Region"]')?.textContent,
  country: document.querySelector('[aria-label="Country"]')?.textContent,
  focused: document.activeElement?.getAttribute('aria-label') }));
async function auditedPage(options) {
  const context = await browser.newContext({ deviceScaleFactor: 1, reducedMotion: 'reduce', serviceWorkers: 'block', acceptDownloads: false, ...options });
  context.setDefaultTimeout(5000); context.setDefaultNavigationTimeout(15000);
  await context.route('**/*', route => {
    const req = route.request(), u = new URL(req.url());
    if (u.origin !== base || !['GET', 'HEAD'].includes(req.method()) || (!dynamicRoutes.has(u.pathname) && !referencePath(u.pathname) && u.pathname !== '/favicon.ico')) {
      report.blocked.push({ phase, url: req.url(), method: req.method() }); return route.abort('blockedbyclient');
    }
    return route.continue();
  });
  await context.routeWebSocket('**/*', ws => { report.blocked.push({ phase, url: ws.url(), method: 'WS' }); ws.close(); });
  const page = await context.newPage();
  page.on('pageerror', e => report.errors.push({ phase, error: String(e) }));
  page.on('console', m => { if (['warning', 'error'].includes(m.type())) report.console.push({ phase, type: m.type(), text: m.text() }); });
  page.on('response', response => {
    const at = phase, responseURL = response.url();
    const task = (async () => {
      const u = new URL(responseURL), file = referencePath(u.pathname);
      // Next's partial prefetch response may stay open intentionally. It is not a
      // static asset; do not wait for an unbounded stream to certify its bytes.
      if (!file) { report.responses.push({ phase: at, url: responseURL, status: response.status(), comparison: 'dynamic-status-only', matches: null }); return; }
      let timer;
      const bytes = await Promise.race([response.body(), new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(`Static response body timeout: ${u.pathname}`)), 10000); })]).finally(() => clearTimeout(timer));
      const expected = fontPins.get(u.pathname)?.sha256 || hash(await readFile(file));
      report.responses.push({ phase: at, url: responseURL, status: response.status(), bytes: bytes.length, sha256: hash(bytes),
        comparison: fontPins.has(u.pathname) ? 'pinned-font' : 'selected-build', matches: expected === hash(bytes) });
    })();
    pending.add(task); task.catch(e => report.errors.push({ phase: at, url: responseURL, error: String(e) })).finally(() => pending.delete(task));
  });
  return { context, page };
}
try {
  browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, timeout: 15000, args: ['--disable-background-networking', '--no-proxy-server'] });
  report.browser = browser.version();
  phase = 'p10-form-task';
  const { context: formContext, page } = await auditedPage({ viewport: { width: 1440, height: 1000 }, colorScheme: 'light' });
  await page.goto(base + '/settings', { waitUntil: 'networkidle' }); await page.evaluate(() => document.fonts.ready);
  report.initial = await formState(page);
  await page.getByRole('textbox', { name: 'Organization Name', exact: true }).fill('Synthetic Observatory');
  const bio = page.getByRole('textbox', { name: 'Organization Bio', exact: true });
  await bio.fill('x'.repeat(241));
  report.bio = await bio.evaluate(e => ({ length: e.value.length, maxLength: e.maxLength, valid: e.checkValidity() }));
  check('advertised 240-character bio bound is enforced before save', report.bio.length <= 240 || !report.bio.valid, report.bio);
  const country = page.getByRole('button', { name: 'Country', exact: true });
  await country.focus(); await page.keyboard.press('Space'); await page.getByRole('listbox').waitFor({ state: 'visible' });
  report.countryOptions = await page.getByRole('listbox').getByRole('option').allTextContents();
  check('country choices retain the complete supplied set', JSON.stringify(report.countryOptions) === JSON.stringify(['Canada', 'Mexico', 'United States']));
  await page.keyboard.press('ArrowDown'); await page.keyboard.press('Enter'); await page.getByRole('listbox').waitFor({ state: 'hidden' });
  report.afterCountry = await formState(page);
  check('keyboard country selection closes and restores focus', report.afterCountry.country === 'Mexico' && report.afterCountry.focused === 'Country');
  const fields = Object.fromEntries(report.afterCountry.entries), regions = report.afterCountry.entries.filter(([k]) => /^country\[regions\]\[\d+\]$/.test(k)).map(([,v]) => v);
  check('country change cannot submit a hidden stale region', regions.includes(fields.region) && report.afterCountry.region === fields.region, { region: fields.region, displayedRegion: report.afterCountry.region, country: fields['country[name]'] });
  await page.getByRole('button', { name: 'Reset', exact: true }).click(); report.afterReset = await formState(page);
  check('Reset restores the complete original compound form state', JSON.stringify(report.afterReset.entries) === JSON.stringify(report.initial.entries) && report.afterReset.region === report.initial.region && report.afterReset.country === report.initial.country);
  await page.waitForLoadState('networkidle'); await drainResponses(); await formContext.close(); await drainResponses();
  for (const width of [390, 1440]) for (const scheme of ['light', 'dark']) {
    phase = `p10-${width}-${scheme}`;
    const { context, page: capturePage } = await auditedPage({ viewport: { width, height: 1000 }, colorScheme: scheme });
    await capturePage.goto(base + '/settings', { waitUntil: 'networkidle' }); await capturePage.evaluate(() => document.fonts.ready);
    const view = await capturePage.evaluate(() => ({ width: innerWidth, scrollWidth: document.documentElement.scrollWidth,
      scheme: matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light', font: getComputedStyle(document.body).fontFamily,
      loadedFonts: [...document.fonts].filter(f => f.status === 'loaded').map(f => ({ family: f.family, weight: f.weight })),
      brokenImages: [...document.images].filter(i => i.getBoundingClientRect().width > 0 && (!i.complete || !i.naturalWidth)).map(i => i.getAttribute('src')) }));
    check(`${phase}: no page-width overflow`, view.width === width && view.scrollWidth <= width, { width, scrollWidth: view.scrollWidth });
    const usedWeights = width === 390 ? ['400', '600'] : ['400', '500', '600'];
    check(`${phase}: used Inter weights loaded`, usedWeights.every(w => view.loadedFonts.some(f => f.family === 'Inter' && f.weight === w)), view.loadedFonts);
    check(`${phase}: no broken laid-out image`, view.brokenImages.length === 0, view.brokenImages);
    const screenshot = join(root, `${phase}.png`); await capturePage.screenshot({ path: screenshot, fullPage: true });
    report.views.push({ name: phase, ...view, screenshot, screenshot_sha256: hash(await readFile(screenshot)) });
    await capturePage.waitForLoadState('networkidle'); await drainResponses(); await context.close(); await drainResponses();
  }
  check('all four requested views captured', JSON.stringify(report.views.map(v => v.name)) === JSON.stringify(expectedViews));
} catch (error) { report.failure = String(error); report.failurePhase = phase; }
finally {
  await drainResponses();
  try { if (browser) await browser.close(); } catch (error) { report.errors.push({ phase: 'teardown', error: String(error) }); }
  await drainResponses();
  check('observed static responses agree with selected build and pinned fonts', report.responses.some(r => r.comparison === 'pinned-font') && report.responses.some(r => r.comparison === 'selected-build') && report.responses.every(r => r.status === 200 && r.matches !== false), report.responses.length);
  check('no outside or write request observed', report.blocked.length === 0, report.blocked);
  check('no page or response-processing error', report.errors.length === 0, report.errors);
  check('no browser console warning or error', report.console.length === 0, report.console);
  check('exact behavior and view assertion manifest completed', JSON.stringify(report.checks.map(c => c.name).sort()) === JSON.stringify([...expectedChecks].sort()), { expected: expectedChecks.length, observed: report.checks.length });
  report.ok = !report.failure && report.views.length === 4 && report.checks.every(c => c.pass); if (!report.ok) process.exitCode = 1;
  await writeFile(join(root, 'receipt.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({ root, ok: report.ok, checks: report.checks.length, failed: report.checks.filter(c => !c.pass), failure: report.failure, errors: report.errors, console: report.console }));
}
