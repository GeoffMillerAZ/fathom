// Prospective P10 browser-zoom evidence. This uses Chrome's persisted 200% setting,
// never CSS zoom, a device scale factor, or CDP page scaling. It remains a bounded
// local fixture: it is not a physical-input, contrast, performance, or fidelity claim.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {captureZoomDocument} from './zoom-capture.mjs';

const [base, parent] = process.argv.slice(2);
const assert = (ok, message) => { if (!ok) throw Error(message); };
const origin = new URL(base);
assert(origin.origin === base && origin.protocol === 'http:' && origin.hostname === '127.0.0.1' && origin.port,
  'Explicit loopback origin required');
assert(path.isAbsolute(parent) && fs.lstatSync(parent).isDirectory() && !fs.lstatSync(parent).isSymbolicLink(),
  'Explicit existing private output parent required');
const modulePath = process.env.APPKIT_PLAYWRIGHT_MODULE;
assert(modulePath && path.isAbsolute(modulePath), 'Select approved isolated Playwright');
const playwrightPackage = JSON.parse(fs.readFileSync(path.join(path.dirname(modulePath), 'package.json')));
assert(playwrightPackage.name === 'playwright' && playwrightPackage.version === '1.63.0', 'Pinned Playwright 1.63.0 required');

const chrome = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const hash = body => crypto.createHash('sha256').update(body).digest('hex');
const canonical = value => JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, char =>
  `\\u${char.charCodeAt(0).toString(16).padStart(4, '0')}`);
const out = fs.mkdtempSync(path.join(parent, 'p10-zoom-browser-'));
fs.chmodSync(out, 0o700);
const put = (name, body) => {
  const bytes = Buffer.from(body);
  fs.writeFileSync(path.join(out, name), bytes, {flag: 'wx', mode: 0o600});
  return {name, sha256: hash(bytes), bytes: bytes.length};
};
const lanes = ['light', 'dark'].flatMap(theme => [
  {id: `${theme}-native`, theme, mode: 'native', javascript: 'withheld', path: `/${theme}/settings`},
  {id: `${theme}-enhanced`, theme, mode: 'enhanced', javascript: 'available', path: `/${theme}/enhanced/settings`},
]);
const plan = {
  schema_version: 1,
  fixture: 'complete-p10-actual-browser-zoom',
  runner_sha256: hash(fs.readFileSync(fileURLToPath(import.meta.url))),
  capture_helper_sha256: hash(fs.readFileSync(new URL('./zoom-capture.mjs', import.meta.url))),
  chrome: '152.0.7977.84',
  playwright: '1.63.0',
  node: process.version,
  zoom: {method: 'chrome://settings/appearance #zoomLevel.selectOption(2)', requested: '2', viewport: {width: 1440, height: 1200}},
  lanes,
  action_timeout_ms: 10_000,
  total_timeout_ms: 120_000,
  gates: [
    'fresh persistent profile and Chrome WebUI reports 200 percent before each lane',
    'actual 200 percent environment: devicePixelRatio 2, inner/client width 720, outer width 1440, visual viewport scale 1',
    'complete ten-field/five-section form, no overflow or blank controls, 44 CSS-pixel controls and checkbox label',
    'keyboard focus plus server autofocus observations; default and error screenshots',
    'full-document CDP DIP capture: measured 2x CSS ratio, 1440px PNG width, exact full height, complete form and final action containment, complete CRC-checked PNG pixel rows',
    'native HTTP 422 retains exact 241-emoji draft and focuses bio; correction/Enter are HTTP 200',
    'native Reset POST restores snapshot and Reset focus; enhanced Reset restores all values on the same focused node without POST',
    'after Chrome WebUI handoff, only the exact loopback origin may issue GET, POST, or HEAD requests',
  ],
  observation: 'fonts.ready followed by a fixed 50ms Node wait; no requestAnimationFrame, polling, latency, or visual-fidelity claim',
  exclusions: [
    'physical keyboard or pointer input certification', 'full contrast review', 'performance or payload budget',
    'complete source fidelity', 'cross-browser/native-host proof', 'real consumer adoption',
  ],
  verification: 'not-verified',
};
const planFile = put('plan.json', canonical(plan));
console.log(JSON.stringify({out, plan_sha256: planFile.sha256, status: 'actual-zoom tasks starting'}));

const report = {schema_version: 1, plan_sha256: planFile.sha256, checks: [], errors: [], forbidden_requests: [], requests: [], responses: [], screenshots: [], profiles: {}, lanes: []};
const check = (name, ok, actual = null) => {
  report.checks.push({name, ok: !!ok, actual});
  assert(ok, name);
};
const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
const sameValues = (actual, expected) => {
  const keys = Object.keys(expected);
  return Object.keys(actual).length === keys.length && keys.every(key => actual[key] === expected[key]);
};
let currentContext;
let currentProfile;
let timedOut = false;
const totalTimeout = setTimeout(() => {
  timedOut = true;
  report.errors.push('total timeout after 120000ms; closing owned persistent context');
  if (currentContext) void currentContext.close().catch(error => report.errors.push(`timeout context close: ${error.message}`));
}, plan.total_timeout_ms);

function cleanupProfile(profile) {
  const stat = fs.lstatSync(profile);
  assert(path.dirname(profile) === out && path.basename(profile).startsWith('chrome-profile-') &&
    stat.isDirectory() && !stat.isSymbolicLink(), 'Unexpected profile cleanup target');
  fs.rmSync(profile, {recursive: true});
  report.profiles[path.basename(profile)] = 'removed after successful context close';
}

async function settle(page) {
  // Page timers and rAF do not run when the native lane has script execution
  // disabled, so the only wait is externally driven and deliberately fixed.
  await page.evaluate(async () => { await document.fonts.ready; });
  await wait(50);
}

async function snapshot(page) {
  await settle(page);
  return page.locator('form[data-p10-settings]').evaluate(form => {
    const controls = [...form.querySelectorAll('input:not([type=hidden]), textarea, select')]
      .filter(control => control.form === form && control.name);
    const buttonBoxes = [...form.querySelectorAll('button')].map(button => {
      const rect = button.getBoundingClientRect();
      return {name: button.textContent.trim(), width: rect.width, height: rect.height};
    });
    return {
      enhanced: form.getAttribute('data-p10-enhanced'),
      headings: [...form.querySelectorAll('h1,h2,h3')].map(element => element.textContent.trim()),
      values: Object.fromEntries(controls.map(control => [control.name, control.type === 'checkbox' ? control.checked : control.value])),
      controls: controls.map(control => {
        const rect = control.getBoundingClientRect();
        return {name: control.name, type: control.type, id: control.id, width: rect.width, height: rect.height,
          clientWidth: control.clientWidth, scrollWidth: control.scrollWidth, invalid: control.getAttribute('aria-invalid')};
      }),
      buttons: buttonBoxes,
      checkboxLabelHeight: form.querySelector('[data-p10-checkbox-label]')?.getBoundingClientRect().height || 0,
      regionGroups: form.querySelectorAll('select[name=region] optgroup').length,
      errors: [...form.querySelectorAll('[data-p10-error]:not([hidden])')].map(element => element.getAttribute('data-p10-error')),
      status: form.querySelector('[role=status]')?.textContent || '',
      active: document.activeElement?.id || '',
      activeName: document.activeElement?.name || '',
      activeValue: document.activeElement?.value || '',
      focusVisible: document.activeElement?.matches(':focus-visible') || false,
      environment: {dpr: devicePixelRatio, inner: innerWidth, outer: outerWidth, client: document.documentElement.clientWidth,
        visualScale: visualViewport.scale, scroll: document.documentElement.scrollWidth},
    };
  });
}

async function post(page, button, status, label) {
  const [response] = await Promise.all([
    page.waitForNavigation({waitUntil: 'load'}),
    page.getByRole('button', {name: button, exact: true}).click(),
  ]);
  check(`${label}: ${button} HTTP ${status}`, response?.status() === status, response?.status());
}

async function configureLane(context, page, lane) {
  check(`${lane.id}: pinned Chrome`, context.browser().version() === plan.chrome, context.browser().version());
  await page.goto('chrome://settings/appearance', {waitUntil: 'load'});
  await page.locator('#zoomLevel').selectOption('2');
  check(`${lane.id}: Chrome WebUI selected 200 percent`, await page.locator('#zoomLevel').inputValue() === '2');
  await page.goto('about:blank', {waitUntil: 'load'});
  check(`${lane.id}: left Chrome WebUI before application restrictions`, page.url() === 'about:blank', page.url());
  if (lane.mode === 'native') {
    const cdp = await context.newCDPSession(page);
    await cdp.send('Emulation.setScriptExecutionDisabled', {value: true});
  }
  context.setDefaultTimeout(plan.action_timeout_ms);
  context.setDefaultNavigationTimeout(plan.action_timeout_ms);
  page.on('pageerror', error => report.errors.push(`${lane.id}: ${error.message}`));
  page.on('requestfailed', request => report.errors.push(`${lane.id}: request failed ${request.method()} ${request.url()} ${request.failure()?.errorText || ''}`));
  page.on('request', request => report.requests.push({lane: lane.id, method: request.method(), url: request.url()}));
  page.on('response', response => report.responses.push({lane: lane.id, method: response.request().method(), url: response.url(), status: response.status()}));
  await context.route('**/*', route => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.origin === base && ['GET', 'POST', 'HEAD'].includes(request.method())) return route.continue();
    report.forbidden_requests.push({lane: lane.id, method: request.method(), url: request.url()});
    return route.abort();
  });
  await context.routeWebSocket('**/*', socket => {
    report.forbidden_requests.push({lane: lane.id, method: 'WEBSOCKET', url: socket.url()});
    socket.close();
  });
}

async function completeLane(lane, chromium) {
  const profile = fs.mkdtempSync(path.join(out, 'chrome-profile-'));
  fs.chmodSync(profile, 0o700);
  report.profiles[path.basename(profile)] = 'retained until successful persistent-context close';
  let context;
  let closed = false;
  try {
    context = await chromium.launchPersistentContext(profile, {executablePath: chrome, headless: true,
      viewport: {width: 1440, height: 1200}, colorScheme: lane.theme, reducedMotion: 'reduce', serviceWorkers: 'block'});
    currentContext = context;
    currentProfile = profile;
    const page = context.pages()[0];
    check(`${lane.id}: one owned initial page`, context.pages().length === 1, context.pages().length);
    context.on('page', unexpected => {
      if (unexpected !== page) {
        report.errors.push(`${lane.id}: unexpected window`);
        void unexpected.close();
      }
    });
    await configureLane(context, page, lane);
    if (timedOut) throw Error('total timeout');
    const response = await page.goto(base + lane.path, {waitUntil: 'load'});
    check(`${lane.id}: initial response is 200`, response?.status() === 200, response?.status());
    let state = await snapshot(page);
    const expectedInitial = {...state.values};
    check(`${lane.id}: true browser-zoom environment`, state.environment.dpr === 2 && state.environment.inner === 720 &&
      state.environment.client === 720 && state.environment.outer === 1440 && state.environment.visualScale === 1 &&
      state.environment.scroll <= state.environment.client, state.environment);
    check(`${lane.id}: complete P10 composition`, state.headings.join('|') === 'Settings|Organization Name|Organization Bio|Organization Email|Address|Currency' &&
      state.controls.length === 10 && new Set(state.controls.map(control => control.name)).size === 10, state);
    check(`${lane.id}: controls are visible and 44 CSS pixels tall`, state.controls.every(control =>
      control.width > 0 && control.clientWidth > 0 && control.scrollWidth <= control.clientWidth &&
      (control.type === 'checkbox' || control.height >= 43.99)) && state.buttons.every(button => button.width > 0 && button.height >= 43.99) &&
      state.checkboxLabelHeight >= 43.99, state);
    check(`${lane.id}: expected script lane`, lane.mode === 'enhanced' ? state.enhanced === 'true' && state.regionGroups === 1 :
      state.enhanced === null && state.regionGroups === 3, {enhanced: state.enhanced, regionGroups: state.regionGroups});
    const cdp = await context.newCDPSession(page);
    const defaultCapture = await captureZoomDocument(page, cdp);
    check(`${lane.id}: complete default frame at actual zoom`, true, defaultCapture.evidence);
    const defaultShot = put(`default-${lane.id}.png`, defaultCapture.bytes);
    report.screenshots.push({lane: lane.id, state: 'default', ...defaultShot, geometry: defaultCapture.evidence});
    await page.keyboard.press('Tab');
    state = await snapshot(page);
    check(`${lane.id}: keyboard produces visible form focus`, state.activeName !== '' && state.focusVisible, state);

    const draft = '😀'.repeat(241);
    await page.locator('[name=bio]').fill(draft);
    await page.locator('select[name="country[code]"]').selectOption('MX');
    await post(page, 'Validate changes', 422, lane.id);
    state = await snapshot(page);
    check(`${lane.id}: 422 retains exact 241-emoji draft and observes bio autofocus`, state.values.bio === draft &&
      state.values['country[code]'] === 'MX' && state.errors.includes('bio') && state.errors.includes('region') &&
      state.active.endsWith('-bio'), state);
    const errorCapture = await captureZoomDocument(page, cdp);
    check(`${lane.id}: complete error frame at actual zoom`, true, errorCapture.evidence);
    const errorShot = put(`errors-${lane.id}.png`, errorCapture.bytes);
    report.screenshots.push({lane: lane.id, state: 'error', ...errorShot, geometry: errorCapture.evidence});

    await page.locator('[name=bio]').fill('Corrected local example');
    await page.locator('select[name=region]').selectOption('Jalisco');
    await post(page, 'Validate changes', 200, lane.id);
    state = await snapshot(page);
    check(`${lane.id}: corrected native POST is valid`, state.values.bio === 'Corrected local example' &&
      state.values.region === 'Jalisco' && state.values['country[code]'] === 'MX' && state.errors.length === 0 && /validat/i.test(state.status), state);

    await page.locator('input[name=name]').fill('Zoom keyboard-enter example');
    const [enterResponse] = await Promise.all([
      page.waitForNavigation({waitUntil: 'load'}),
      page.locator('input[name=name]').press('Enter'),
    ]);
    state = await snapshot(page);
    check(`${lane.id}: Enter validates and retains name`, enterResponse?.status() === 200 &&
      new URL(page.url()).pathname === lane.path && state.values.name === 'Zoom keyboard-enter example' && /validat/i.test(state.status), state);

    if (lane.mode === 'native') {
      await post(page, 'Reset', 200, lane.id);
      state = await snapshot(page);
      check(`${lane.id}: native Reset POST restores snapshot and server Reset focus`, sameValues(state.values, expectedInitial) &&
        state.errors.length === 0 && state.status === '' && state.activeName === '_action' && state.activeValue === 'reset', state);
    } else {
      const dirty = {name: 'Reset draft', bio: '😀'.repeat(241), email: 'modified@example.test', address: '2 Synthetic Way', city: 'Example city', postal_code: '54321'};
      for (const [name, value] of Object.entries(dirty)) await page.locator(`[name="${name}"]`).fill(value);
      await page.locator('[name=email_is_public]').setChecked(!expectedInitial.email_is_public);
      await page.locator('select[name="country[code]"]').selectOption('MX');
      await page.locator('select[name=region]').selectOption('Jalisco');
      const otherCurrency = await page.locator('select[name=currency]').evaluate((select, initial) =>
        [...select.options].find(option => option.value && option.value !== initial)?.value || '', expectedInitial.currency);
      assert(otherCurrency, 'A distinct fixture currency is required for reset proof');
      await page.locator('select[name=currency]').selectOption(otherCurrency);
      const reset = page.getByRole('button', {name: 'Reset', exact: true});
      await reset.focus();
      await page.evaluate(() => { window.__p10ZoomResetNode = document.activeElement; });
      const requestsBeforeReset = report.requests.length;
      await reset.click();
      state = await snapshot(page);
      const resetFocus = await page.evaluate(() => ({sameNode: document.activeElement === window.__p10ZoomResetNode,
        connected: window.__p10ZoomResetNode?.isConnected, sameDocument: window.__p10ZoomResetNode?.ownerDocument === document}));
      check(`${lane.id}: enhanced Reset restores all values on same focused node without POST`, sameValues(state.values, expectedInitial) &&
        state.errors.length === 0 && state.status === '' && resetFocus.sameNode && resetFocus.connected && resetFocus.sameDocument &&
        report.requests.length === requestsBeforeReset, {state, resetFocus, requestsBeforeReset, requestsAfterReset: report.requests.length});
    }
    report.lanes.push({lane: lane.id, mode: lane.mode, status: 'passed'});
  } finally {
    if (context) {
      try {
        await context.close();
        closed = true;
      } catch (error) {
        report.errors.push(`${lane.id}: persistent context close: ${error.message}`);
      }
    }
    if (currentContext === context) currentContext = undefined;
    if (currentProfile === profile) currentProfile = undefined;
    // Retain a profile whenever launch/close ownership is uncertain; deletion is
    // only permitted for this exact fresh mkdtemp child after a successful close.
    if (closed) cleanupProfile(profile);
  }
}

try {
  const {chromium} = await import(pathToFileURL(modulePath).href);
  for (const lane of lanes) {
    if (timedOut) throw Error('total timeout');
    await completeLane(lane, chromium);
  }
  check('all lanes completed', report.lanes.length === lanes.length, report.lanes);
  check('no application errors or disallowed traffic', report.errors.length === 0 && report.forbidden_requests.length === 0,
    {errors: report.errors, forbidden_requests: report.forbidden_requests});
} catch (error) {
  report.errors.push(error.stack || error.message);
  process.exitCode = 1;
} finally {
  clearTimeout(totalTimeout);
  if (currentContext) {
    try { await currentContext.close(); } catch (error) { report.errors.push(`final context close: ${error.message}`); }
    currentContext = undefined;
  }
  report.status = report.errors.length || report.forbidden_requests.length || report.lanes.length !== lanes.length ? 'failed-not-verified' : 'recorded-not-verified';
  const receipt = put('report.json', canonical(report));
  console.log(JSON.stringify({out, status: report.status, lanes: report.lanes.length, checks: report.checks.length,
    errors: report.errors, report_sha256: receipt.sha256, profile_disposition: report.profiles}));
  if (report.status !== 'recorded-not-verified') process.exitCode = 1;
}
