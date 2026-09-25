// Opt-in browser contract for a separately identified private derivative.
// No install, original mutation, real account, save backend or certification.
import { createHash } from 'node:crypto';
import { readFile, writeFile, mkdtemp, stat } from 'node:fs/promises';
import { dirname, isAbsolute, join } from 'node:path';
import { tmpdir } from 'node:os';
import { pathToFileURL } from 'node:url';
import { verifyHardenedReference } from './reference.mjs';

const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const base = process.env.APPKIT_CATALYST_HARDENED_BROWSER_ORIGIN;
const project = process.env.APPKIT_CATALYST_HARDENED_ROOT;
const modulePath = process.env.APPKIT_PLAYWRIGHT_MODULE;
const reviewedModule = '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
if (!base || !project || !isAbsolute(project) || modulePath !== reviewedModule) throw new Error('Select explicit APPKIT_CATALYST_HARDENED_BROWSER_ORIGIN, absolute APPKIT_CATALYST_HARDENED_ROOT and the reviewed APPKIT_PLAYWRIGHT_MODULE; nothing is installed.');
const origin = new URL(base);
if (origin.origin !== base || origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || !origin.port) throw new Error('Select exact HTTP 127.0.0.1 origin with explicit non-default port and no extra URL data.');
const proof = verifyHardenedReference(project);
const playwrightPackage = await readFile(join(dirname(modulePath), 'package.json'));
if (JSON.parse(playwrightPackage).name !== 'playwright' || JSON.parse(playwrightPackage).version !== '1.63.0') throw new Error('Reviewed Playwright 1.63.0 is required.');
const root = await mkdtemp(join(tmpdir(), 'appkit-catalyst-hardened-browser-'));
console.log(`Private hardened Catalyst browser evidence: ${root}`);
const report = { origin: base, project, started: new Date().toISOString(), source_manifest_sha256: proof.source_manifest_sha256,
  verifier_sha256: hash(await readFile(new URL('./reference.mjs', import.meta.url))), probe_sha256: hash(await readFile(new URL(import.meta.url))),
  playwright: { module: modulePath, module_sha256: hash(await readFile(modulePath)), package_sha256: hash(playwrightPackage), version: '1.63.0' },
  checks: [], views: [], states: [], responses: [], errors: [], console: [], blocked: [], limits: [
    'Separate hardened source-native derivative; original source, font verifier and browser oracle remain independent and unchanged.',
    'Synthetic local pre-submit behavior only. Validation success means nothing was saved; no backend, account, consumer adoption or target certification.',
    'Observed static response agreement with selected prepared files; dynamic Next HTML/RSC status only, not process attestation, whole-build reproducibility or all possible egress.',
    'Code points, not graphemes. Combining marks and ZWJ code points are intentionally counted separately.',
    'Tampering canaries change only this disposable page DOM, not source files. No browser zoom, complete accessibility, IME or production security claim.' ] };
const check = (name, pass, detail) => report.checks.push({ name, pass: !!pass, detail });
const dynamicRoutes = new Set(['/', '/settings', '/events', '/orders', '/events/1000', '/events/1001', '/events/1002', '/events/1003']);
const fontRoot = '/__appkit_reference_fonts/46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441/';
const publicPins = new Map(proof.files.filter(f => f.path.startsWith('public/')).map(f => ['/' + f.path.slice('public/'.length), f]));
const fontPins = new Map([...publicPins].filter(([path]) => path.startsWith(fontRoot)));
function selectedFile(path) {
  if (publicPins.has(path)) return join(project, publicPins.get(path).path);
  if (/^\/_next\/static\/chunks\/[A-Za-z0-9_.~-]+\.(js|css)$/.test(path)) return join(project, '.next', path.slice('/_next/'.length));
  return null;
}
const staticPins = new Map();
async function selectedHash(file) {
  if (!staticPins.has(file)) {
    const info = await stat(file);
    if (!info.isFile() || info.size > 4 * 1024 * 1024 || staticPins.size >= 512) throw new Error('Selected static input exceeds the regular-file/4 MiB/512-file bound.');
    staticPins.set(file, hash(await readFile(file)));
  }
  return staticPins.get(file);
}
const expectedViews = ['hardened-390-light', 'hardened-390-dark', 'hardened-1440-light', 'hardened-1440-dark'];
const expectedStates = ['hardened-390-light-errors', 'hardened-390-dark-errors'];
const bioCases = [
  ['ASCII 240', 'x'.repeat(240), 240, true], ['ASCII 241', 'x'.repeat(241), 241, false],
  ['emoji 240', '😀'.repeat(240), 240, true], ['emoji 241', '😀'.repeat(241), 241, false],
  ['combining 240', 'e\u0301'.repeat(120), 240, true], ['combining 242', 'e\u0301'.repeat(121), 242, false],
];
const expectedChecks = [
  'hydrated form starts with reviewed defaults and honest local-only notice',
  ...bioCases.map(([name]) => `${name}: exact draft, count, validity, status and associated error`),
  'country choices retain the complete supplied set',
  'Canada to Mexico clears region and associates its visible error without moving Country focus',
  'invalid biography takes focus before invalid email and empty region',
  'invalid email takes focus before empty region',
  'empty region blocks validation and receives focus',
  'explicit compatible region selection permits honest local validation',
  'compatible region selection clears the Region invalid state',
  'success status clears after a native field edit',
  'success status clears after a Headless checkbox change',
  'tampered nonmember region is rejected and focused',
  'tampered visible-hidden region mismatch is rejected and focused',
  'tampered unknown country is rejected and focused',
  'before Reset every selected field is dirty and both validation errors are present',
  'Reset restores every native and controlled FormData value',
  'Reset retains the same button element and focus',
  'Reset clears validity, associated errors, counters and status',
  'no-JavaScript fieldset disables native controls and explains the limitation',
  'no-JavaScript disabled submit produces no submission',
  ...expectedViews.flatMap(view => [`${view}: no page-width overflow`, `${view}: used Inter weights loaded`, `${view}: no broken laid-out image`]),
  ...expectedStates.map(state => `${state}: fits width with both associated errors, retained draft and Bio focus`),
  ...expectedStates.map(state => `${state}: Postal code surface does not stretch beside the Region error`),
  'all four requested derivative views captured',
  'both narrow error states captured separately from default views',
  'observed static responses agree with selected derivative and pinned fonts',
  'no outside or write request observed', 'no page or response-processing error', 'no browser console warning or error',
];
const pending = new Set();
const drainResponses = async () => { while (pending.size) await Promise.allSettled([...pending]); };
let browser, phase = 'launch';
const { chromium } = await import(pathToFileURL(modulePath).href);
async function auditedPage(options) {
  const context = await browser.newContext({ deviceScaleFactor: 1, reducedMotion: 'reduce', serviceWorkers: 'block', acceptDownloads: false, ...options });
  context.setDefaultTimeout(5000); context.setDefaultNavigationTimeout(15000);
  await context.route('**/*', route => {
    const req = route.request(), u = new URL(req.url());
    if (u.origin !== base || !['GET', 'HEAD'].includes(req.method()) || (!dynamicRoutes.has(u.pathname) && !selectedFile(u.pathname) && u.pathname !== '/favicon.ico')) {
      report.blocked.push({ phase, url: req.url(), method: req.method() }); return route.abort('blockedbyclient');
    }
    return route.continue();
  });
  await context.routeWebSocket('**/*', ws => { report.blocked.push({ phase, url: ws.url(), method: 'WS' }); ws.close(); });
  const page = await context.newPage();
  page.on('pageerror', error => report.errors.push({ phase, error: String(error) }));
  page.on('console', message => { if (['warning', 'error'].includes(message.type())) report.console.push({ phase, type: message.type(), text: message.text() }); });
  page.on('response', response => {
    const at = phase, responseURL = response.url();
    const task = (async () => {
      const u = new URL(responseURL), file = selectedFile(u.pathname);
      if (!file) { report.responses.push({ phase: at, url: responseURL, status: response.status(), comparison: 'dynamic-status-only', matches: null }); return; }
      let timer;
      const bytes = await Promise.race([response.body(), new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(`Static response body timeout: ${responseURL}`)), 10000); })]).finally(() => clearTimeout(timer));
      const expected = publicPins.get(u.pathname)?.sha256 || await selectedHash(file);
      report.responses.push({ phase: at, url: responseURL, status: response.status(), bytes: bytes.length, sha256: hash(bytes), comparison: fontPins.has(u.pathname) ? 'pinned-font' : publicPins.has(u.pathname) ? 'pinned-public' : 'selected-build', matches: expected === hash(bytes) });
    })();
    pending.add(task); task.catch(error => report.errors.push({ phase: at, url: responseURL, error: String(error) })).finally(() => pending.delete(task));
  });
  return { context, page };
}
const formState = page => page.evaluate(() => {
  const form = document.querySelector('form');
  const described = element => (element?.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean).map(id => {
    const node = document.getElementById(id), rect = node?.getBoundingClientRect();
    return { id, text: node?.textContent, visible: !!rect?.width && !!rect?.height && getComputedStyle(node).visibility !== 'hidden' };
  });
  const bio = form.elements.namedItem('bio'), region = form.querySelector('[aria-label="Region"]');
  return { entries: [...new FormData(form).entries()], country: form.querySelector('[aria-label="Country"]')?.textContent.trim(), region: region?.textContent.trim(),
    focused: document.activeElement?.getAttribute('aria-label') || document.activeElement?.textContent.trim(), status: form.querySelector('[role="status"]')?.textContent.trim(),
    disabled: form.querySelector('fieldset').disabled, checkbox: form.querySelector('[role="checkbox"]')?.getAttribute('aria-checked'),
    bio: { value: bio.value, valid: bio.validity.valid, customError: bio.validity.customError, invalid: bio.getAttribute('aria-invalid'), count: document.getElementById('bio-count')?.textContent, descriptions: described(bio) },
    regionError: { invalid: region?.getAttribute('aria-invalid'), descriptions: described(region) },
    invalidControls: [...form.querySelectorAll('input,select,textarea')].filter(e => e.willValidate && !e.validity.valid).map(e => e.name),
    errors: [...form.querySelectorAll('[data-slot="error"]')].map(e => e.textContent) };
});
const localSuccess = 'Validation passed for this local reference. Nothing was saved.';
const associatedError = (state, text) => state.invalid === 'true' && state.descriptions.some(d => d.visible && d.text?.includes(text));
async function openSettings(page, ready = true) {
  await page.goto(base + '/settings', { waitUntil: 'networkidle' });
  if (ready) await page.waitForFunction(() => document.querySelector('form > fieldset')?.disabled === false);
  await page.evaluate(() => document.fonts.ready);
}
async function selectOption(page, control, option) {
  await page.getByRole('button', { name: control, exact: true }).click();
  await page.getByRole('listbox').getByRole('option', { name: option, exact: true }).click();
  await page.getByRole('listbox').waitFor({ state: 'hidden' });
}
async function closeContext(context) { await drainResponses(); await context.close(); await drainResponses(); }

try {
  browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, timeout: 15000, args: ['--disable-background-networking', '--no-proxy-server'] });
  report.browser = browser.version(); phase = 'hardened-form-task';
  const { context, page } = await auditedPage({ viewport: { width: 1440, height: 1000 }, colorScheme: 'light' });
  await openSettings(page); report.initial = await formState(page);
  const initialFields = Object.fromEntries(report.initial.entries);
  const initialExpected = { name: 'Catalyst', bio: '', email: 'info@example.com', email_is_public: 'on', address: '147 Catalyst Ave', city: 'Toronto', postal_code: 'A1A 1A1', currency: 'cad', region: 'Ontario', 'country[code]': 'CA' };
  check('hydrated form starts with reviewed defaults and honest local-only notice', !report.initial.disabled && Object.entries(initialExpected).every(([key, value]) => initialFields[key] === value) && report.initial.country === 'Canada' && report.initial.region === 'Ontario' && report.initial.checkbox === 'true' && await page.getByText('Local validation reference. Nothing is saved.', { exact: true }).isVisible(), report.initial);
  const bio = page.getByRole('textbox', { name: 'Organization Bio', exact: true }), validate = page.getByRole('button', { name: 'Validate changes', exact: true }), reset = page.getByRole('button', { name: 'Reset', exact: true });
  report.biographies = [];
  for (const [name, value, count, accepted] of bioCases) {
    await bio.fill(value); await validate.click(); const state = await formState(page);
    const pass = state.bio.value === value && state.bio.count === `${count} / 240 Unicode code points` && state.bio.valid === accepted && state.bio.customError === !accepted &&
      (accepted ? state.status === localSuccess && state.bio.invalid !== 'true' : state.status === '' && state.focused === 'Organization Bio' && associatedError(state.bio, 'Your draft has not been shortened.'));
    check(`${name}: exact draft, count, validity, status and associated error`, pass, state); report.biographies.push({ name, state });
  }
  await reset.click();
  const country = page.getByRole('button', { name: 'Country', exact: true });
  await country.focus(); await page.keyboard.press('Space'); await page.getByRole('listbox').waitFor({ state: 'visible' });
  const countries = await page.getByRole('listbox').getByRole('option').allTextContents();
  check('country choices retain the complete supplied set', JSON.stringify(countries) === JSON.stringify(['Canada', 'Mexico', 'United States']), countries);
  await page.keyboard.press('ArrowDown'); await page.keyboard.press('Enter'); await page.getByRole('listbox').waitFor({ state: 'hidden' });
  let state = await formState(page); report.afterCountry = state;
  check('Canada to Mexico clears region and associates its visible error without moving Country focus', state.country === 'Mexico' && state.region === 'Choose a region' && !state.entries.some(([key, value]) => key === 'region' && value) && state.focused === 'Country' && associatedError(state.regionError, 'Choose a region in Mexico.'), state);
  const email = page.getByRole('textbox', { name: 'Organization Email', exact: true });
  await email.fill('not-an-email'); await bio.fill('x'.repeat(241)); await validate.click(); state = await formState(page);
  check('invalid biography takes focus before invalid email and empty region', state.focused === 'Organization Bio' && state.status === '' && state.invalidControls.includes('bio') && state.invalidControls.includes('email'), state);
  await bio.fill(''); await validate.click(); state = await formState(page);
  check('invalid email takes focus before empty region', state.focused === 'Organization Email' && state.status === '' && state.invalidControls.includes('email'), state);
  await email.fill('info@example.com'); await validate.click(); state = await formState(page);
  check('empty region blocks validation and receives focus', state.focused === 'Region' && state.status === '' && associatedError(state.regionError, 'Choose a region in Mexico.'), state);
  await selectOption(page, 'Region', 'Aguascalientes'); await validate.click(); state = await formState(page);
  check('explicit compatible region selection permits honest local validation', state.region === 'Aguascalientes' && Object.fromEntries(state.entries).region === 'Aguascalientes' && state.status === localSuccess && state.errors.length === 0, state);
  check('compatible region selection clears the Region invalid state', state.regionError.invalid !== 'true' && state.regionError.descriptions.every(d => !d.text?.startsWith('Choose ')), state.regionError);
  await page.getByRole('textbox', { name: 'Organization Name', exact: true }).fill('Synthetic Observatory');
  check('success status clears after a native field edit', (await formState(page)).status === '');
  await validate.click(); const beforeCheckbox = await formState(page);
  await page.getByRole('checkbox', { name: 'Include email in validated sample data', exact: true }).click(); state = await formState(page);
  check('success status clears after a Headless checkbox change', beforeCheckbox.status === localSuccess && state.status === '' && state.checkbox !== beforeCheckbox.checkbox, state);
  for (const [name, field, value, message] of [
    ['nonmember region', 'region', 'Ontario', 'Choose a region in Mexico.'],
    ['visible-hidden region mismatch', 'region', 'Baja California', 'visible and submitted values agree'],
    ['unknown country', 'country[code]', 'ZZ', 'Choose a country from the available options.'],
  ]) {
    await reset.click(); await selectOption(page, 'Country', 'Mexico'); await selectOption(page, 'Region', 'Aguascalientes');
    await page.locator(`input[name="${field}"]`).evaluate((input, next) => { input.value = next; }, value);
    await validate.click(); state = await formState(page);
    check(`tampered ${name} is rejected and focused`, state.status === '' && state.focused === 'Region' && state.region === 'Aguascalientes' && associatedError(state.regionError, message), state);
  }
  await reset.click();
  for (const [name, value] of [['Organization Name', 'Synthetic Observatory'], ['Organization Email', 'invalid-email'], ['Street Address', '25 Synthetic Way'], ['City', 'Example City'], ['Postal code', 'TEST']]) await page.getByRole('textbox', { name, exact: true }).fill(value);
  await bio.fill('😀'.repeat(241)); await page.getByRole('combobox', { name: 'Currency', exact: true }).selectOption('usd');
  await page.getByRole('checkbox', { name: 'Include email in validated sample data', exact: true }).click();
  await selectOption(page, 'Country', 'Mexico'); await validate.click();
  report.beforeReset = await formState(page);
  const dirtyFields = Object.fromEntries(report.beforeReset.entries);
  const dirtyExpected = { name: 'Synthetic Observatory', bio: '😀'.repeat(241), email: 'invalid-email', address: '25 Synthetic Way', city: 'Example City', postal_code: 'TEST', currency: 'usd', 'country[code]': 'MX' };
  check('before Reset every selected field is dirty and both validation errors are present', Object.entries(dirtyExpected).every(([key, value]) => dirtyFields[key] === value) &&
    !report.beforeReset.entries.some(([key, value]) => key === 'email_is_public' || key === 'region' && value) && report.beforeReset.checkbox === 'false' &&
    report.beforeReset.country === 'Mexico' && report.beforeReset.region === 'Choose a region' && report.beforeReset.bio.value === '😀'.repeat(241) &&
    report.beforeReset.bio.count === '241 / 240 Unicode code points' && report.beforeReset.bio.customError && !report.beforeReset.bio.valid &&
    report.beforeReset.invalidControls.includes('bio') && report.beforeReset.invalidControls.includes('email') && report.beforeReset.status === '' &&
    associatedError(report.beforeReset.bio, 'Your draft has not been shortened.') && associatedError(report.beforeReset.regionError, 'Choose a region in Mexico.'), report.beforeReset);
  const resetElement = await reset.elementHandle(); await reset.click(); state = await formState(page); report.afterReset = state;
  check('Reset restores every native and controlled FormData value', JSON.stringify(state.entries) === JSON.stringify(report.initial.entries) && state.country === report.initial.country && state.region === report.initial.region && state.checkbox === report.initial.checkbox, state);
  check('Reset retains the same button element and focus', await resetElement.evaluate(element => element.isConnected && element === document.querySelector('button[type="reset"]') && document.activeElement === element)); await resetElement.dispose();
  check('Reset clears validity, associated errors, counters and status', state.status === '' && state.errors.length === 0 && state.invalidControls.length === 0 && state.bio.value === '' && state.bio.count === '0 / 240 Unicode code points' && !state.bio.customError && state.bio.invalid !== 'true' && state.regionError.invalid !== 'true', state);
  await closeContext(context);

  phase = 'hardened-no-javascript';
  const noJS = await auditedPage({ viewport: { width: 390, height: 1000 }, colorScheme: 'light', javaScriptEnabled: false });
  await openSettings(noJS.page, false);
  const disabled = await noJS.page.evaluate(() => ({ fieldset: document.querySelector('form > fieldset').disabled, controls: [...document.querySelectorAll('form input,form button,form textarea,form select')].map(e => ({ name: e.name || e.textContent, disabled: e.matches(':disabled') })), notice: document.querySelector('form noscript')?.textContent }));
  check('no-JavaScript fieldset disables native controls and explains the limitation', disabled.fieldset && disabled.controls.length >= 10 && disabled.controls.every(c => c.disabled) && disabled.notice?.includes('Saving is not available.'), disabled);
  await drainResponses();
  const noJSResponses = report.responses.length, noJSBlocked = report.blocked.length;
  await noJS.page.locator('button[type="submit"]').evaluate(button => button.click()); await noJS.page.waitForTimeout(200); await drainResponses();
  check('no-JavaScript disabled submit produces no submission', noJS.page.url() === base + '/settings' && report.responses.length === noJSResponses && report.blocked.length === noJSBlocked);
  await closeContext(noJS.context);

  for (const width of [390, 1440]) for (const scheme of ['light', 'dark']) {
    phase = `hardened-${width}-${scheme}`;
    const capture = await auditedPage({ viewport: { width, height: 1000 }, colorScheme: scheme }); await openSettings(capture.page);
    const view = await capture.page.evaluate(() => ({ width: innerWidth, scrollWidth: document.documentElement.scrollWidth, scheme: matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',
      font: getComputedStyle(document.body).fontFamily, loadedFonts: [...document.fonts].filter(f => f.status === 'loaded').map(f => ({ family: f.family, weight: f.weight })),
      brokenImages: [...document.images].filter(i => i.getBoundingClientRect().width > 0 && (!i.complete || !i.naturalWidth)).map(i => i.getAttribute('src')) }));
    check(`${phase}: no page-width overflow`, view.width === width && view.scrollWidth <= width && view.scheme === scheme, view);
    check(`${phase}: used Inter weights loaded`, (width === 390 ? ['400', '600'] : ['400', '500', '600']).every(weight => view.loadedFonts.some(f => f.family === 'Inter' && f.weight === weight)), view.loadedFonts);
    check(`${phase}: no broken laid-out image`, view.brokenImages.length === 0, view.brokenImages);
    const screenshot = join(root, `${phase}.png`); await capture.page.screenshot({ path: screenshot, fullPage: true });
    report.views.push({ name: phase, ...view, screenshot, screenshot_sha256: hash(await readFile(screenshot)) });
    if (width === 390) {
      phase = `hardened-390-${scheme}-errors`;
      await capture.page.getByRole('textbox', { name: 'Organization Bio', exact: true }).fill('x'.repeat(241));
      await selectOption(capture.page, 'Country', 'Mexico');
      await capture.page.getByRole('button', { name: 'Validate changes', exact: true }).click();
      const state = await formState(capture.page);
      const geometry = await capture.page.evaluate(() => {
        const postal = document.querySelector('input[name="postal_code"]');
        const control = postal.closest('span[data-slot="control"]');
        return { width: innerWidth, scrollWidth: document.documentElement.scrollWidth,
          postalInputHeight: postal.getBoundingClientRect().height, postalSurfaceHeight: control.getBoundingClientRect().height };
      });
      check(`${phase}: fits width with both associated errors, retained draft and Bio focus`, geometry.width === 390 && geometry.scrollWidth <= 390 &&
        state.bio.value === 'x'.repeat(241) && state.bio.count === '241 / 240 Unicode code points' && !state.bio.valid && state.bio.customError &&
        associatedError(state.bio, 'Your draft has not been shortened.') && associatedError(state.regionError, 'Choose a region in Mexico.') &&
        state.country === 'Mexico' && state.region === 'Choose a region' && state.status === '' && state.focused === 'Organization Bio', { ...geometry, state });
      check(`${phase}: Postal code surface does not stretch beside the Region error`, geometry.postalInputHeight > 0 &&
        Math.abs(geometry.postalSurfaceHeight - geometry.postalInputHeight) <= 2, geometry);
      const stateScreenshot = join(root, `${phase}.png`); await capture.page.screenshot({ path: stateScreenshot, fullPage: true });
      report.states.push({ name: phase, scheme, ...geometry, state, screenshot: stateScreenshot, screenshot_sha256: hash(await readFile(stateScreenshot)) });
    }
    await closeContext(capture.context);
  }
  check('all four requested derivative views captured', JSON.stringify(report.views.map(v => v.name)) === JSON.stringify(expectedViews));
  check('both narrow error states captured separately from default views', JSON.stringify(report.states.map(state => state.name)) === JSON.stringify(expectedStates));
} catch (error) { report.failure = String(error); report.failurePhase = phase; }
finally {
  await drainResponses(); try { if (browser) await browser.close(); } catch (error) { report.errors.push({ phase: 'teardown', error: String(error) }); } await drainResponses();
  check('observed static responses agree with selected derivative and pinned fonts', report.responses.some(r => r.comparison === 'pinned-font') && report.responses.some(r => r.comparison === 'selected-build') && report.responses.every(r => r.status === 200 && r.matches !== false), report.responses.length);
  check('no outside or write request observed', report.blocked.length === 0, report.blocked);
  check('no page or response-processing error', report.errors.length === 0, report.errors);
  check('no browser console warning or error', report.console.length === 0, report.console);
  check('exact derivative behavior and view assertion manifest completed', JSON.stringify(report.checks.map(c => c.name).sort()) === JSON.stringify([...expectedChecks].sort()), { expected: expectedChecks.length, observed: report.checks.length, missing: expectedChecks.filter(name => !report.checks.some(c => c.name === name)) });
  report.ok = !report.failure && report.views.length === 4 && report.states.length === 2 && report.checks.every(c => c.pass); if (!report.ok) process.exitCode = 1;
  await writeFile(join(root, 'receipt.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({ root, ok: report.ok, checks: report.checks.length, failed: report.checks.filter(c => !c.pass), failure: report.failure, errors: report.errors, console: report.console }));
}
