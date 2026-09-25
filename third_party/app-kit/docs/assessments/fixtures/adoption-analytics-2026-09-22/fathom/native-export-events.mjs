// Frozen-candidate diagnostic only. Args: Playwright module, Chrome, loopback origin,
// copied source CUE, new output directory. Never submits except by native user input.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { trackOwnedChild, terminateOwned } from '../../../../../apps/styleguide/browser/owned_process.mjs';

const [pw, chrome, origin, sourceCue, output] = process.argv.slice(2);
const url = new URL(origin);
assert.equal(url.protocol, 'http:');
assert.equal(url.hostname, '127.0.0.1');
assert.equal(url.origin, origin);
assert.ok(path.isAbsolute(sourceCue) && path.isAbsolute(output));
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const members = ['candidate.css', 'candidate.cue', 'diff.json', 'original.cue', 'receipt.json'];
const report = { scope: 'frozen synthetic Fathom candidate; no active integration',
  observation_complete: false, native_export_checks_passed: false,
  events: [], requests: [], responses: [], failures: [], blocked: [], attempts: [], errors: [] };
let browser, browserServer, browserOwned, page, madeOutput = false;
const stamp = () => Date.now();
async function bounded(label, work, ms) {
  let timer;
  try {
    return await Promise.race([work(), new Promise((_, reject) => {
      timer = setTimeout(() => reject(Error(`${label} exceeded ${ms}ms`)), ms);
    })]);
  } finally { clearTimeout(timer); }
}
function captureBrowserPipes(owned) {
  const capture = { stdout: '', stderr: '', truncated: false };
  for (const name of ['stdout', 'stderr']) owned.child[name]?.on('data', chunk => {
    const next = capture[name] + chunk.toString();
    if (Buffer.byteLength(next) > 1024 * 1024) capture.truncated = true;
    else capture[name] = next;
  });
  return capture;
}

async function formState() {
  return page.locator('button[value="export"]').evaluate(button => {
    const form = button.form, rect = button.getBoundingClientRect();
    return { ownerMatches: form === document.querySelector('form'),
      contained: form?.contains(button), type: button.type, disabled: button.disabled,
      actionAttribute: form?.getAttribute('action'), method: form?.method,
      actionPropertyType: typeof form?.action,
      digest: form?.elements.namedItem('review_digest')?.value,
      invalid: form ? Array.from(form.elements).filter(e => e.willValidate && !e.validity.valid)
        .map(e => ({ name: e.name, message: e.validationMessage })) : null,
      bounds: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
      centerTarget: document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2)?.outerHTML.slice(0, 500),
      centerButtonValue: document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2)?.closest('button')?.value };
  });
}

function inspectZip(file, zipBytes, expectedSource, expectedReview, response) {
  const bytes = execFileSync('/usr/bin/unzip', ['-Z1', file], { maxBuffer: 1024 * 1024, timeout: 3000 })
    .toString().trim().split('\n').sort();
  assert.deepEqual(bytes, [...members].sort(), 'ZIP must contain exactly the five reviewed files');
  const content = Object.fromEntries(members.map(name => [name,
    execFileSync('/usr/bin/unzip', ['-p', file, name], { maxBuffer: 1024 * 1024, timeout: 3000 })]));
  const hashes = Object.fromEntries(members.map(name => [name, digest(content[name])]));
  assert.equal(hashes['original.cue'], expectedSource, 'source CUE hash');
  assert.equal(hashes['receipt.json'], expectedReview, 'receipt file is reviewed digest');
  const receipt = JSON.parse(content['receipt.json'].toString());
  assert.equal(receipt.schema_version, 1);
  assert.equal(receipt.scope, 'experimental-app-owned-styleguide');
  assert.equal(receipt.exportable, true);
  for (const name of members.filter(name => name !== 'receipt.json')) {
    assert.equal(receipt.files[name]?.sha256, hashes[name], `${name} receipt hash`);
    assert.equal(receipt.files[name]?.size, content[name].length, `${name} receipt size`);
  }
  assert.equal(Object.keys(receipt.files).length, 4, 'receipt pins exactly four payloads');
  const changes = JSON.parse(content['diff.json'].toString());
  assert.deepEqual(changes, [{ path: 'radius.md', before: '0.375rem', after: '0.75rem' }],
    'only the reviewed radius override is exported');
  const zipHash = digest(zipBytes);
  const headers = response.headers;
  assert.match(headers['content-type'] || '', /^application\/zip(?:;|$)/i);
  assert.match(headers['content-disposition'] || '', /^attachment;.*filename="?app-theme-candidate\.zip"?/i);
  assert.equal(headers['x-content-sha256'], zipHash, 'body hash header');
  assert.equal(Number(headers['content-length']), response.downloadBytes, 'content length');
  return { members, hashes, receipt_review_digest: hashes['receipt.json'],
    source_sha256: hashes['original.cue'], zip_sha256: zipHash, changes };
}

try {
  await fs.mkdir(output); madeOutput = true;
  report.source_sha256_before = digest(await fs.readFile(sourceCue));
  const { chromium } = await import(pathToFileURL(pw).href);
  browserServer = await chromium.launchServer({ executablePath: chrome, headless: true, host: '127.0.0.1' });
  browserOwned = trackOwnedChild(browserServer.process());
  report.browser_capture = captureBrowserPipes(browserOwned);
  report.browser_launch_mechanism = 'Playwright launchServer/connect; fresh Chrome process explicitly tracked';
  report.browser_pid = browserOwned.child.pid;
  browser = await chromium.connect(browserServer.wsEndpoint());
  report.browser = browser.version();
  const context = await browser.newContext({ acceptDownloads: true,
    viewport: { width: 1440, height: 1000 }, colorScheme: 'light', reducedMotion: 'reduce' });
  await context.route('**/*', route => {
    if (new URL(route.request().url()).origin === origin) return route.continue();
    report.blocked.push(route.request().url()); return route.abort();
  });
  page = await context.newPage(); page.setDefaultTimeout(10000);
  page.on('console', message => {
    if (message.text().startsWith('APPKIT_NATIVE_EVENT ')) {
      try { report.events.push({ observedAt: stamp(), ...JSON.parse(message.text().slice(20)) }); }
      catch (error) { report.errors.push(`event decode: ${error}`); }
    } else if (message.type() === 'error') report.errors.push(`console: ${message.text()}`);
  });
  page.on('pageerror', error => report.errors.push(`page: ${error}`));
  page.on('request', request => {
    if (request.method() === 'POST') report.requests.push({ at: stamp(), url: request.url(),
      action: new URLSearchParams(request.postData() || '').get('action'), origin: request.headers().origin });
  });
  page.on('requestfailed', request => report.failures.push({ at: stamp(), url: request.url(),
    method: request.method(), error: request.failure()?.errorText }));
  page.on('response', response => {
    if (response.request().method() === 'POST') report.responses.push({ at: stamp(),
      url: response.url(), status: response.status(),
      action: new URLSearchParams(response.request().postData() || '').get('action'),
      origin: response.request().headers().origin,
      headers: response.headers() });
  });
  await page.addInitScript(() => {
    for (const type of ['pointerdown', 'pointerup', 'click', 'submit', 'invalid', 'formdata', 'securitypolicyviolation']) {
      document.addEventListener(type, event => {
        if (['pointerdown', 'pointerup', 'click'].includes(type) && !event.target.closest?.('button')) return;
        const button = event.submitter || event.target.closest?.('button');
        const item = { type, browserTime: performance.now(), trusted: event.isTrusted,
          target: event.target.tagName, buttonName: button?.name, buttonValue: button?.value,
          invalidName: type === 'invalid' ? event.target.name : undefined,
          action: type === 'formdata' ? event.formData.get('action') : undefined,
          directive: event.violatedDirective, blockedURI: event.blockedURI };
        queueMicrotask(() => console.info('APPKIT_NATIVE_EVENT ' + JSON.stringify({
          ...item, defaultPrevented: event.defaultPrevented })));
      }, true);
    }
  });
  assert.equal((await page.goto(origin + '/dev/theme', { waitUntil: 'load' })).status(), 200);
  await page.getByText('Corner radii', { exact: true }).click();
  await page.locator('input[name="radius.md"]').fill('0.75rem');
  await Promise.all([page.waitForNavigation({ waitUntil: 'load' }),
    page.getByRole('button', { name: 'Preview candidate', exact: true }).click()]);
  const exportButton = page.getByRole('button', { name: 'Export acknowledged candidate ZIP', exact: true });
  await exportButton.scrollIntoViewIfNeeded();
  report.reviewed = await formState();
  assert.equal(report.reviewed.ownerMatches, true);
  assert.equal(report.reviewed.contained, true);
  assert.equal(report.reviewed.type, 'submit');
  assert.equal(report.reviewed.actionAttribute, '/dev/theme');
  assert.equal(report.reviewed.method.toLowerCase(), 'post');
  assert.equal(report.reviewed.disabled, false);
  assert.equal(report.reviewed.centerButtonValue, 'export');
  assert.equal(report.reviewed.invalid.length, 0);
  assert.match(report.reviewed.digest, /^[a-f0-9]{64}$/);
  for (const activation of ['pointer', 'enter']) {
    const attempt = { activation, startedAt: stamp(), before: await formState(),
      eventStart: report.events.length, requestStart: report.requests.length,
      responseStart: report.responses.length, failureStart: report.failures.length };
    const pending = page.waitForEvent('download', { timeout: 8000 })
      .then(download => ({ download }), error => ({ error: String(error) }));
    try {
      if (activation === 'pointer') await exportButton.click();
      else { await exportButton.focus(); await page.keyboard.press('Enter'); }
      const result = await pending;
      if (result.error) throw Error(result.error);
      const file = path.join(output, `${activation}-candidate.zip`);
      await result.download.saveAs(file);
      const stat = await fs.stat(file);
      assert.ok(stat.size > 0 && stat.size <= 1024 * 1024, 'bounded ZIP bytes');
      const savedZipBytes = await fs.readFile(file);
      attempt.download = { filename: result.download.suggestedFilename(),
        failure: await result.download.failure(), bytes: stat.size, sha256: digest(savedZipBytes),
        zipMagic: savedZipBytes.subarray(0, 4).toString('hex') };
      assert.equal(attempt.download.filename, 'app-theme-candidate.zip');
      assert.equal(attempt.download.failure, null);
      assert.equal(attempt.download.zipMagic, '504b0304');
    } catch (error) { attempt.error = String(error.stack || error); }
    attempt.after = await formState().catch(error => ({ error: String(error) }));
    attempt.endedAt = stamp();
    attempt.events = report.events.slice(attempt.eventStart);
    attempt.requests = report.requests.slice(attempt.requestStart);
    attempt.responses = report.responses.slice(attempt.responseStart);
    attempt.failures = report.failures.slice(attempt.failureStart);
    try {
      assert.ok(!attempt.error, attempt.error);
      assert.equal(attempt.before.digest, report.reviewed.digest);
      assert.equal(attempt.after.digest, report.reviewed.digest);
      assert.equal(attempt.before.centerButtonValue, 'export');
      assert.ok(attempt.events.some(event => event.type === 'click' && event.trusted &&
        event.buttonValue === 'export'), 'trusted export click');
      if (activation === 'pointer') assert.ok(attempt.events.some(event =>
        event.type === 'pointerdown' && event.trusted), 'trusted pointer activation');
      assert.ok(attempt.events.some(event => event.type === 'submit' && event.trusted), 'trusted submit');
      assert.ok(attempt.events.some(event => event.type === 'formdata' && event.trusted &&
        event.action === 'export'), 'trusted formdata export');
      assert.equal(attempt.events.filter(event => event.type === 'invalid').length, 0);
      assert.equal(attempt.events.filter(event => event.type === 'securitypolicyviolation').length, 0);
      assert.equal(attempt.requests.length, 1, 'exactly one POST, no extra same-origin POSTs');
      assert.deepEqual({ url: attempt.requests[0].url, action: attempt.requests[0].action,
        origin: attempt.requests[0].origin },
      { url: origin + '/dev/theme', action: 'export', origin }, 'exact export POST');
      assert.equal(attempt.responses.length, 1, 'exactly one POST response');
      assert.deepEqual({ url: attempt.responses[0].url, action: attempt.responses[0].action,
        origin: attempt.responses[0].origin, status: attempt.responses[0].status },
      { url: origin + '/dev/theme', action: 'export', origin, status: 200 },
      'exact successful export response');
      const response = { ...attempt.responses[0], downloadBytes: attempt.download.bytes };
      const zipBytes = await fs.readFile(path.join(output, `${activation}-candidate.zip`));
      attempt.zip = inspectZip(path.join(output, `${activation}-candidate.zip`), zipBytes,
        report.source_sha256_before, report.reviewed.digest, response);
      const unexpected = attempt.failures.filter(failure => !(failure.method === 'POST' &&
        failure.url === origin + '/dev/theme' && /ERR_ABORTED/.test(failure.error || '') &&
        /^attachment;/i.test(response.headers['content-disposition'] || '')));
      assert.equal(unexpected.length, 0, 'unexpected network failures');
      attempt.passed = true;
    } catch (error) { attempt.passed = false; attempt.validation_error = String(error.stack || error); }
    report.attempts.push(attempt);
  }
  report.observation_complete = true;
  assert.deepEqual(report.requests.map(({ url, action, origin: requestOrigin }) =>
    ({ url, action, origin: requestOrigin })),
  ['preview', 'export', 'export'].map(action => ({ url: origin + '/dev/theme', action, origin })),
  'full workflow has only preview/export/export POSTs');
  assert.deepEqual(report.responses.map(({ url, action, origin: requestOrigin, status }) =>
    ({ url, action, origin: requestOrigin, status })),
  ['preview', 'export', 'export'].map(action => ({ url: origin + '/dev/theme', action, origin, status: 200 })),
  'full workflow has only three successful POST responses');
  if (report.attempts.every(attempt => attempt.passed)) {
    assert.equal(report.attempts[0].download.sha256, report.attempts[1].download.sha256,
      'pointer and Enter ZIP bytes must agree');
  }
} catch (error) {
  report.errors.push(String(error.stack || error));
} finally {
  if (page && madeOutput) {
    try { await bounded('viewport screenshot', () => page.screenshot({
      path: path.join(output, report.observation_complete ? 'after-native.png' : 'failure-native.png'),
      fullPage: false, timeout: 3000 }), 4000); report.viewport_screenshot_saved = true; }
    catch (error) { report.errors.push(`viewport screenshot: ${error}`); }
  }
  try { if (browser) await bounded('browser client close', () => browser.close(), 2000);
    report.browser_close_completed = Boolean(browser); }
  catch (error) { report.errors.push(`browser client close: ${error}`); }
  try { if (browserServer) await bounded('browser server close', () => browserServer.close(), 2000);
    report.browser_server_close_completed = Boolean(browserServer); }
  catch (error) { report.errors.push(`browser server close: ${error}`); }
  if (browserOwned) {
    try {
      report.browser_process = browserOwned.state;
      report.browser_termination = await terminateOwned(browserOwned,
        { graceMs: 2000, killMs: 2000, drainMs: 2000 });
      report.browser_process_exit_verified = report.browser_termination.ok &&
        !report.browser_termination.forced && !report.browser_capture.truncated;
      if (!report.browser_process_exit_verified) report.errors.push('fresh Chrome process exit/drain unresolved');
    } catch (error) { report.errors.push(`browser process shutdown: ${error}`); }
  }
  try { report.source_sha256_after = digest(await fs.readFile(sourceCue)); }
  catch (error) { report.errors.push(`source reread: ${error}`); }
  report.native_export_checks_passed = report.observation_complete && report.attempts.length === 2 &&
    report.attempts.every(attempt => attempt.passed) && !report.errors.length && !report.blocked.length &&
    report.source_sha256_before === report.source_sha256_after && report.browser_process_exit_verified === true;
  if (madeOutput) await fs.writeFile(path.join(output, 'report.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({ observation_complete: report.observation_complete,
    native_export_checks_passed: report.native_export_checks_passed,
    attempts: report.attempts.map(({ activation, passed, error, validation_error }) =>
      ({ activation, passed, error, validation_error })), errors: report.errors }, null, 2));
  if (!report.native_export_checks_passed) process.exitCode = 1;
}
