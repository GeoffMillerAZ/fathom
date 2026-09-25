// Local enhanced-form tasks only. The controller remains optional: every native
// POST assertion below exercises the server, and a withheld-module lane retains it.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath, pathToFileURL } from 'node:url';

const [base, parent] = process.argv.slice(2);
const assert = (ok, message) => { if (!ok) throw Error(message); };
const origin = new URL(base);
assert(origin.origin === base && origin.protocol === 'http:' && origin.hostname === '127.0.0.1' && origin.port, 'Explicit loopback origin required');
assert(path.isAbsolute(parent) && fs.lstatSync(parent).isDirectory() && !fs.lstatSync(parent).isSymbolicLink(), 'Explicit existing private output parent required');
const modulePath = process.env.APPKIT_PLAYWRIGHT_MODULE;
assert(modulePath && path.isAbsolute(modulePath), 'Select approved isolated Playwright');
const playwrightPackage = JSON.parse(fs.readFileSync(path.join(path.dirname(modulePath), 'package.json')));
assert(playwrightPackage.name === 'playwright' && playwrightPackage.version === '1.63.0', 'Pinned Playwright 1.63.0 required');

const hash = body => crypto.createHash('sha256').update(body).digest('hex');
const out = fs.mkdtempSync(path.join(parent, 'p10-enhanced-browser-'));
fs.chmodSync(out, 0o700);
const put = (name, body) => {
  const bytes = Buffer.from(body);
  fs.writeFileSync(path.join(out, name), bytes, { flag: 'wx', mode: 0o600 });
  return { name, sha256: hash(bytes), bytes: bytes.length };
};
const plan = {
  schema_version: 1,
  fixture: 'complete-p10-enhanced-http',
  runner_sha256: hash(fs.readFileSync(fileURLToPath(import.meta.url))),
  chrome: '152.0.7977.84',
  playwright: '1.63.0',
  matrix: [390, 1440].flatMap(width => ['light', 'dark'].map(theme => ({ width, theme }))),
  withheld_module_lane: { width: 1440, theme: 'light', javascript: true, controller: 'withheld' },
  javascript: true,
  action_timeout_ms: 10_000,
  total_timeout_ms: 120_000,
  observation: 'font readiness plus fixed 50ms Node wait before every snapshot; no polling, RAF, or latency claim',
  checks: [
    'external module and strict CSP without inline/eval',
    'complete auto-mounted ten-field form with canonical filtered regions',
    'local 240/241 emoji count retains exact draft without requests',
    'country change retains focus and clears incompatible region locally',
    'native POST validation, correction, and Enter retain drafts',
    'editing clears success without a request',
    'pointer and keyboard enhanced Reset each restore all ten edited fields against the initial browser snapshot, same focused node/document and no request',
    'dispose restores native region groups without replacing the focused node and remount is idempotent',
    'withheld controller leaves the native form usable',
  ],
  exclusions: [
    'actual 200% browser zoom', 'performance or payload budgets', 'complete visual fidelity',
    'malformed controller configuration', 'multiple form instances', 'real consumer adoption',
  ],
  verification: 'not-verified',
};
const planFile = put('plan.json', JSON.stringify(plan));
console.log(JSON.stringify({ out, plan_sha256: planFile.sha256, status: 'enhanced tasks starting' }));

const report = { checks: [], errors: [], forbidden: [], requests: [], responses: [], frames: [], lanes: [] };
const check = (name, ok, actual) => {
  report.checks.push({ name, ok, actual });
  assert(ok, name);
};
const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
const settingsPath = theme => `/${theme}/enhanced/settings`;
let browser;
let timedOut = false;
const totalTimeout = setTimeout(() => {
  timedOut = true;
  report.errors.push('total timeout after 120000ms');
  if (browser) void browser.close().catch(error => report.errors.push(`timeout browser close: ${error.message}`));
}, plan.total_timeout_ms);

function sameValues(actual, expected) {
  const keys = Object.keys(expected);
  return Object.keys(actual).length === keys.length && keys.every(key => actual[key] === expected[key]);
}

async function snapshot(page) {
  await page.evaluate(async () => { await document.fonts.ready; });
  await wait(50);
  return page.locator('form[data-p10-settings]').evaluate(form => ({
    enhanced: form.getAttribute('data-p10-enhanced'),
    values: Object.fromEntries([...form.querySelectorAll('input, textarea, select')]
      .filter(control => control.form === form && control.type !== 'hidden' && control.name)
      .map(control => [control.name, control.type === 'checkbox' ? control.checked : control.value])),
    fields: [...form.querySelectorAll('input:not([type=hidden]),textarea,select')].map(control => ({
      name: control.name, type: control.type, id: control.id, value: control.type === 'checkbox' ? control.checked : control.value,
      invalid: control.getAttribute('aria-invalid'), height: control.getBoundingClientRect().height,
    })),
    headings: [...form.querySelectorAll('h1,h2,h3')].map(element => element.textContent.trim()),
    regionGroups: form.querySelectorAll('select[name=region] optgroup').length,
    regionOptions: [...form.querySelector('select[name=region]').options].map(option => option.value),
    errors: [...form.querySelectorAll('[data-p10-error]:not([hidden])')].map(element => element.getAttribute('data-p10-error')),
    status: form.querySelector('[role=status]')?.textContent || '',
    count: form.querySelector('[id$="-bio-count"]')?.textContent || '',
    active: document.activeElement?.id || '',
    activeName: document.activeElement?.name || '',
    activeValue: document.activeElement?.value || '',
    width: innerWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));
}

async function nativePost(page, button, expectedStatus, label) {
  const [response] = await Promise.all([
    page.waitForNavigation({ waitUntil: 'load' }),
    page.getByRole('button', { name: button, exact: true }).click(),
  ]);
  check(`${label}: ${button} native POST ${expectedStatus}`, response?.status() === expectedStatus, response?.status());
}

async function exerciseEnhanced(lane, chromium) {
  const label = `${lane.width}-${lane.theme}`;
  const context = await browser.newContext({
    viewport: { width: lane.width, height: 1200 }, colorScheme: lane.theme, reducedMotion: 'reduce',
    serviceWorkers: 'block', javaScriptEnabled: true,
  });
  try {
    await context.route('**/*', route => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.origin === base && ['GET', 'POST', 'HEAD'].includes(request.method())) return route.continue();
      report.forbidden.push({ lane: label, url: url.href, method: request.method() });
      return route.abort();
    });
    const page = await context.newPage();
    page.setDefaultTimeout(plan.action_timeout_ms);
    page.setDefaultNavigationTimeout(plan.action_timeout_ms);
    page.on('pageerror', error => report.errors.push(`${label}: ${error.message}`));
    page.on('request', request => report.requests.push({ lane: label, path: new URL(request.url()).pathname, method: request.method() }));
    page.on('response', response => report.responses.push({ lane: label, path: new URL(response.url()).pathname, status: response.status(), method: response.request().method() }));

    const response = await page.goto(base + settingsPath(lane.theme), { waitUntil: 'load' });
    check(`${label}: initial enhanced response`, response?.status() === 200, response?.status());
    const csp = response?.headers()['content-security-policy'] || '';
    const script = await page.locator('script').evaluateAll(nodes => nodes.map(node => ({ type: node.type, src: node.getAttribute('src'), inline: node.textContent.trim() })));
    check(`${label}: external controller and CSP forbid inline/eval`,
      /script-src 'self'/.test(csp) && !/unsafe-inline|unsafe-eval/.test(csp) &&
      script.length === 1 && script[0].type === 'module' && script[0].src === '/controller.mjs' && script[0].inline === '',
      { csp, script });

    let state = await snapshot(page);
    const expectedInitial = { ...state.values };
    check(`${label}: auto-mounted complete form`, state.enhanced === 'true' && state.headings.join('|') === 'Settings|Organization Name|Organization Bio|Organization Email|Address|Currency' && state.fields.length === 10 && new Set(state.fields.map(field => field.name)).size === 10 && state.errors.length === 0 && state.status === '', state);
    check(`${label}: canonical region is filtered without overflow`, state.regionGroups === 1 && state.regionOptions.includes(state.values.region) && state.scrollWidth <= state.width, state);
    const defaultShot = put(`default-${label}.png`, await page.locator('form[data-p10-settings]').screenshot());
    report.frames.push({ ...lane, state: 'default', screenshot: defaultShot });

    const requestsBeforeLocal = report.requests.length;
    await page.locator('select[name="country[code]"]').focus();
    await page.locator('select[name="country[code]"]').selectOption('MX');
    state = await snapshot(page);
    check(`${label}: country change retains Country focus and clears incompatible region`, state.activeName === 'country[code]' && state.values['country[code]'] === 'MX' && state.values.region === '' && state.regionGroups === 1, state);
    await page.locator('[name=bio]').fill('😀'.repeat(240));
    state = await snapshot(page);
    check(`${label}: local 240 emoji count is exact`, state.values.bio === '😀'.repeat(240) && state.count.startsWith('240 / 240') && !state.errors.includes('bio'), state);
    await page.locator('[name=bio]').fill('😀'.repeat(241));
    state = await snapshot(page);
    check(`${label}: local 241 emoji count retains full draft`, state.values.bio === '😀'.repeat(241) && state.count.startsWith('241 / 240') && state.errors.includes('bio'), state);
    check(`${label}: local country and count operations made no request`, report.requests.length === requestsBeforeLocal, report.requests.slice(requestsBeforeLocal));

    await nativePost(page, 'Validate changes', 422, label);
    state = await snapshot(page);
    check(`${label}: native 422 retains 241 draft and focuses bio`, state.values.bio === '😀'.repeat(241) && state.errors.includes('bio') && state.errors.includes('region') && state.active.endsWith('-bio'), state);
    const errorShot = put(`errors-${label}.png`, await page.locator('form[data-p10-settings]').screenshot());
    report.frames.push({ ...lane, state: 'errors', screenshot: errorShot });

    await page.locator('[name=bio]').fill('Corrected local example');
    await nativePost(page, 'Validate changes', 422, label);
    state = await snapshot(page);
    check(`${label}: corrected bio leaves native region correction focused`, state.values.bio === 'Corrected local example' && !state.errors.includes('bio') && state.errors.includes('region') && state.active.endsWith('-region'), state);
    await page.locator('select[name=region]').selectOption('Jalisco');
    await nativePost(page, 'Validate changes', 200, label);
    state = await snapshot(page);
    check(`${label}: corrected native POST succeeds`, state.values.bio === 'Corrected local example' && state.values.region === 'Jalisco' && state.values['country[code]'] === 'MX' && state.errors.length === 0 && /validat/i.test(state.status), state);

    const requestsBeforeEdit = report.requests.length;
    await page.locator('input[name=name]').fill('Keyboard-enter example');
    state = await snapshot(page);
    check(`${label}: edit clears success without a request`, state.values.name === 'Keyboard-enter example' && state.status === '' && report.requests.length === requestsBeforeEdit, state);
    const [enterResponse] = await Promise.all([
      page.waitForNavigation({ waitUntil: 'load' }),
      page.locator('input[name=name]').press('Enter'),
    ]);
    state = await snapshot(page);
    check(`${label}: Enter validates and retains corrected draft`, enterResponse?.status() === 200 && new URL(page.url()).pathname === settingsPath(lane.theme) && state.values.name === 'Keyboard-enter example' && /validat/i.test(state.status), state);

    const dirtyAllFields = async () => {
      for (const [name,value] of Object.entries({name:'Reset draft',bio:'😀'.repeat(241),email:'modified@example.test',address:'2 Synthetic Way',city:'Example city',postal_code:'54321'})) await page.locator(`[name="${name}"]`).fill(value);
      await page.locator('[name=email_is_public]').setChecked(!expectedInitial.email_is_public);
      await page.locator('select[name="country[code]"]').selectOption('MX');
      await page.locator('select[name=region]').selectOption('Jalisco');
      const otherCurrency=await page.locator('select[name=currency]').evaluate((select,initial)=>[...select.options].find(option=>option.value&&option.value!==initial).value,expectedInitial.currency);
      await page.locator('select[name=currency]').selectOption(otherCurrency);
    };
    await dirtyAllFields();
    await page.getByRole('button', { name: 'Reset', exact: true }).focus();
    await page.evaluate(() => { window.__p10ResetNode = document.activeElement; });
    const requestsBeforePointerReset = report.requests.length;
    await page.getByRole('button', { name: 'Reset', exact: true }).click();
    state = await snapshot(page);
    const pointerReset = await page.evaluate(() => ({
      sameNode: document.activeElement === window.__p10ResetNode,
      sameDocument: window.__p10ResetNode?.ownerDocument === document,
      connected: window.__p10ResetNode?.isConnected,
    }));
    check(`${label}: pointer Reset restores initial same focused node without request`, sameValues(state.values, expectedInitial) && state.errors.length === 0 && state.status === '' && pointerReset.sameNode && pointerReset.sameDocument && pointerReset.connected && report.requests.length === requestsBeforePointerReset, { state, pointerReset });

    await dirtyAllFields();
    await page.getByRole('button', { name: 'Reset', exact: true }).focus();
    const requestsBeforeKeyboardReset = report.requests.length;
    await page.keyboard.press('Enter');
    state = await snapshot(page);
    const keyboardReset = await page.evaluate(() => ({
      sameNode: document.activeElement === window.__p10ResetNode,
      sameDocument: window.__p10ResetNode?.ownerDocument === document,
      connected: window.__p10ResetNode?.isConnected,
    }));
    check(`${label}: keyboard Reset preserves initial focus and makes no request`, sameValues(state.values, expectedInitial) && state.errors.length === 0 && state.status === '' && keyboardReset.sameNode && keyboardReset.sameDocument && keyboardReset.connected && report.requests.length === requestsBeforeKeyboardReset, { state, keyboardReset });

    const disposal = await page.evaluate(async () => {
      const form = document.querySelector('form[data-p10-settings]');
      const region = form.querySelector('select[name=region]');
      const originalValue = region.value;
      region.focus();
      const { mountSettings } = await import('/controller.mjs');
      const first = mountSettings(form);
      const second = mountSettings(form);
      first.dispose();
      const afterFirst = {
        enhanced: form.hasAttribute('data-p10-enhanced'), groups: region.querySelectorAll('optgroup').length,
        sameRegion: form.querySelector('select[name=region]') === region, sameValue: region.value === originalValue, value: region.value, focused: document.activeElement === region,
      };
      first.dispose();
      const afterSecondDispose = { enhanced: form.hasAttribute('data-p10-enhanced'), groups: region.querySelectorAll('optgroup').length, focused: document.activeElement === region };
      const remounted = mountSettings(form);
      const afterRemount = { enhanced: form.hasAttribute('data-p10-enhanced'), groups: region.querySelectorAll('optgroup').length, sameHandle: first === second, remounted: Boolean(remounted) };
      remounted.dispose();
      return { afterFirst, afterSecondDispose, afterRemount, finalGroups: region.querySelectorAll('optgroup').length, finalFocus: document.activeElement === region };
    });
    check(`${label}: disposal restores native groups and remount remains idempotent`, disposal.afterFirst.enhanced === false && disposal.afterFirst.groups === 3 && disposal.afterFirst.sameRegion && disposal.afterFirst.sameValue && disposal.afterFirst.focused && disposal.afterSecondDispose.groups === 3 && disposal.afterSecondDispose.focused && disposal.afterRemount.enhanced && disposal.afterRemount.groups === 1 && disposal.afterRemount.sameHandle && disposal.afterRemount.remounted && disposal.finalGroups === 3 && disposal.finalFocus, disposal);
    report.lanes.push({ ...lane, status: 'enhanced-passed' });
  } finally {
    await context.close();
  }
}

async function exerciseWithheldModule() {
  const lane = plan.withheld_module_lane;
  const label = `withheld-${lane.width}-${lane.theme}`;
  const context = await browser.newContext({
    viewport: { width: lane.width, height: 1200 }, colorScheme: lane.theme, reducedMotion: 'reduce',
    serviceWorkers: 'block', javaScriptEnabled: true,
  });
  try {
    await context.route('**/*', route => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.origin !== base || !['GET', 'POST', 'HEAD'].includes(request.method())) {
        report.forbidden.push({ lane: label, url: url.href, method: request.method() });
        return route.abort();
      }
      if (url.pathname === '/controller.mjs') return route.abort();
      return route.continue();
    });
    const page = await context.newPage();
    page.setDefaultTimeout(plan.action_timeout_ms);
    page.setDefaultNavigationTimeout(plan.action_timeout_ms);
    page.on('pageerror', error => report.errors.push(`${label}: ${error.message}`));
    page.on('request', request => report.requests.push({ lane: label, path: new URL(request.url()).pathname, method: request.method() }));
    const response = await page.goto(base + settingsPath(lane.theme), { waitUntil: 'load' });
    check(`${label}: enhanced route loads when controller is withheld`, response?.status() === 200, response?.status());
    const state = await snapshot(page);
    check(`${label}: native form remains unmounted and usable`, state.enhanced === null && state.fields.length === 10 && state.regionGroups === 3, state);
    await page.locator('input[name=name]').fill('Withheld-module native draft');
    const [postResponse] = await Promise.all([
      page.waitForNavigation({ waitUntil: 'load' }),
      page.locator('input[name=name]').press('Enter'),
    ]);
    const retained = await snapshot(page);
    check(`${label}: native Enter still POSTs and retains draft`, postResponse?.status() === 200 && retained.values.name === 'Withheld-module native draft' && /validat/i.test(retained.status), retained);
    report.lanes.push({ ...lane, status: 'withheld-module-native-passed' });
  } finally {
    await context.close();
  }
}

try {
  const { chromium } = await import(pathToFileURL(modulePath).href);
  browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true });
  check('pinned Chrome', browser.version() === plan.chrome, browser.version());
  for (const lane of plan.matrix) {
    if (timedOut) throw Error('total timeout');
    await exerciseEnhanced(lane, chromium);
  }
  if (timedOut) throw Error('total timeout');
  await exerciseWithheldModule();
  check('no page errors or forbidden requests', report.errors.length === 0 && report.forbidden.length === 0, { errors: report.errors, forbidden: report.forbidden });
} catch (error) {
  report.errors.push(error.stack || error.message);
  process.exitCode = 1;
} finally {
  clearTimeout(totalTimeout);
  if (browser) {
    try { await browser.close(); } catch (error) { report.errors.push(error.message); process.exitCode = 1; }
  }
  report.status = report.errors.length || report.forbidden.length ? 'failed' : 'scoped-enhanced-tasks-passed';
  const result = put('report.json', JSON.stringify(report));
  console.log(JSON.stringify({ out, status: report.status, checks: report.checks.length, errors: report.errors, report_sha256: result.sha256 }));
}
