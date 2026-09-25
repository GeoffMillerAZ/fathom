// Opt-in browser receipt for the pinned Datastar Activity observer. It does
// not mock the adapter, signals, declarative attribute, or transport.
import {mkdir, writeFile} from 'node:fs/promises';
import path from 'node:path';
import {bootstrap} from './config.mjs';

const {base, variant, root, chromium, executablePath} = await bootstrap();
const screenshots = path.join(root, 'screenshots');
const reportPath = path.join(root, 'shell-observer-report.json');
await mkdir(screenshots, {recursive: true});

const report = {base, variant, browser: null, passes: [], failures: [], requests: [], streams: [], blocked: [], websockets: [], console: [], expectedConsole: [], pageErrors: [], csp: {}, claims_not_made: ['No keyboard, zoom, native-client, performance, comprehensive accessibility, or production-user-data claim.']};
const pass = (name, detail = '') => report.passes.push({name, detail});
const check = (condition, name, detail = '') => condition ? pass(name, detail) : report.failures.push({name, detail});
const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));
const redactCSP = (value) => value.replace(/'nonce-[^']+'/g, "'nonce-[redacted]'");
const shellGET = new Set(['/shell', '/shell/home', '/shell/activity', '/shell/profile', '/shell/settings', '/shell/help', '/shell/about', '/shell/events']);
const assets = new Set(['/static/app.css', '/static/shell-demo.js', '/static/shell-live-entry.js', '/static/shell-live.js', '/static/datastar-v1.0.3.js']);
function allowed(url, method) {
  return url.origin === base && ((method === 'GET' || method === 'HEAD') && (shellGET.has(url.pathname) || assets.has(url.pathname)) || method === 'POST' && url.pathname === '/shell/action');
}
function safeHeaders(headers) {
  return Object.fromEntries(['accept', 'content-type', 'datastar-request', 'origin', 'sec-fetch-site'].flatMap(key => headers[key] === undefined ? [] : [[key, headers[key]]]));
}

const browser = await chromium.launch({headless: true, executablePath});
report.browser = await browser.version();
const context = await browser.newContext({serviceWorkers: 'block', viewport: {width: 1280, height: 900}});
context.setDefaultTimeout(5000);
await context.route('**/*', async route => {
  const request = route.request();
  const url = new URL(request.url());
  const item = {method: request.method(), path: `${url.pathname}${url.search}`};
  if (url.pathname === '/shell/events' || url.pathname === '/shell/action') item.headers = safeHeaders(request.headers());
  if (url.pathname === '/shell/events') {
    item.postData = request.postData();
    report.streams.push(item);
  }
  if (allowed(url, request.method())) { report.requests.push(item); return route.continue(); }
  report.blocked.push({...item, url: request.url()});
  return route.abort('blockedbyclient');
});
await context.routeWebSocket('**/*', ws => { report.websockets.push(ws.url()); ws.close(); });
function observe(page, name) {
  page.on('console', message => {
    if (message.type() !== 'error') return;
    const location = message.location();
    report.console.push({page: name, text: message.text(), url: location.url});
  });
  page.on('pageerror', error => report.pageErrors.push(`${name}: ${error}`));
}

try {
  const page = await context.newPage();
  observe(page, 'observer');
  const response = await page.goto(`${base}/shell/activity`, {waitUntil: 'networkidle'});
  const initialCSP = response.headers()['content-security-policy'] || '';
  check(response.status() === 200, 'activity loads');
  check(/script-src 'nonce-[^']+'/.test(initialCSP) && !/unsafe-eval|unsafe-inline/.test(initialCSP), 'nonce CSP excludes unsafe eval and inline');
  check(/connect-src 'self'/.test(initialCSP), 'activity CSP limits connect to self');
  report.csp.initial = redactCSP(initialCSP);
  await page.waitForSelector('#shell-live-start:not([disabled])');
  check(await page.locator('#shell-live').getAttribute('data-state') === 'idle', 'observer mounts idle');
  check((await page.locator('#shell-live-summary').textContent()).includes('Page-load version 1 · Alex Morgan'), 'ordinary first snapshot remains visible');
  check(report.streams.length === 0, 'no stream before Connect');

  const draft = await context.newPage();
  observe(draft, 'draft');
  await draft.goto(`${base}/shell/profile`, {waitUntil: 'networkidle'});
  await draft.locator('input[name="display_name"]').fill('Unsubmitted observer draft');
  check(await draft.locator('input[name="display_name"]').inputValue() === 'Unsubmitted observer draft', 'same-session draft established');

  await page.locator('#shell-live-start').click();
  await page.waitForFunction(() => document.querySelector('#shell-live')?.dataset.state === 'watching');
  check((await page.locator('#shell-live-summary').textContent()).includes('Last observed version 1 · Alex Morgan'), 'declarative Connect renders initial SSE snapshot');
  check(report.streams.length === 1, 'one initial stream request');
  await page.screenshot({path: path.join(screenshots, 'initial-live-snapshot.png'), fullPage: true});

  const updater = await context.newPage();
  observe(updater, 'updater');
  await updater.goto(`${base}/shell/profile`, {waitUntil: 'networkidle'});
  await updater.locator('input[name="display_name"]').fill('Observer Update Avery');
  const save = await Promise.all([
    updater.waitForResponse(r => new URL(r.url()).pathname === '/shell/action' && r.request().method() === 'POST'),
    updater.locator('button[data-shell-save]').click(),
  ]).then(([r]) => r);
  check(save.status() === 303, 'synthetic profile save accepted', `status=${save.status()}`);
  await page.waitForFunction(() => document.querySelector('#shell-live-summary')?.textContent.includes('Observer Update Avery'));
  check((await page.locator('#shell-live-summary').textContent()).includes('Observer Update Avery'), 'second-tab profile save reaches observer');
  check(await draft.locator('input[name="display_name"]').inputValue() === 'Unsubmitted observer draft', 'separate unsaved draft remains unchanged');
  await page.screenshot({path: path.join(screenshots, 'watching-second-tab-update.png'), fullPage: true});

  await page.locator('#shell-live-stop').click();
  await page.waitForFunction(() => document.querySelector('#shell-live')?.dataset.state === 'paused');
  const stoppedStreams = report.streams.length;
  await sleep(1400);
  check(report.streams.length === stoppedStreams, 'stop has no auto reconnect in bounded 1.4-second observation', `streams=${stoppedStreams}`);
  await page.locator('#shell-live-start').click();
  await page.waitForFunction(() => document.querySelector('#shell-live')?.dataset.state === 'watching');
  check(report.streams.length === stoppedStreams + 1, 'manual reconnect opens exactly one new stream');
  await page.screenshot({path: path.join(screenshots, 'manual-reconnect.png'), fullPage: true});

  await updater.getByText('My account', {exact: true}).click();
  const signout = await Promise.all([
    updater.waitForResponse(r => new URL(r.url()).pathname === '/shell/action' && r.request().method() === 'POST'),
    updater.getByRole('button', {name: 'Sign out of demo'}).click(),
  ]).then(([r]) => r);
  check(signout.status() === 303, 'second-tab synthetic sign-out accepted', `status=${signout.status()}`);
  await page.waitForFunction(() => document.querySelector('#shell-live')?.dataset.state === 'revoked');
  check(await page.locator('#shell-live-content').isHidden(), 'observer hides observed content after sign-out');
  check((await page.locator('#shell-live-summary').textContent()) === '' && await page.locator('#shell-live-activity li').count() === 0, 'observer clears terminal snapshot and activity');
  await page.screenshot({path: path.join(screenshots, 'observer-terminal-redaction.png'), fullPage: true});
  const signedOut = await updater.goto(`${base}/shell/activity`, {waitUntil: 'networkidle'});
  const signedOutCSP = signedOut.headers()['content-security-policy'] || '';
  report.csp.signed_out = redactCSP(signedOutCSP);
  check(await updater.locator('#shell-live').count() === 0, 'signed-out activity removes observer UI');
  check(!/connect-src/.test(signedOutCSP), 'signed-out CSP removes connect-src');
} catch (error) {
  report.failures.push({name: 'harness execution', detail: String(error?.stack || error)});
} finally {
  for (const stream of report.streams) {
    check(stream.path === '/shell/events' && stream.postData === null && stream.headers.accept === 'text/event-stream', 'stream request shape', JSON.stringify(stream));
  }
  const expected401 = (entry) => {
    if (!/^Failed to load resource: the server responded with a status of 401 \(Unauthorized\)$/.test(entry.text)) return false;
    try {
      const url = new URL(entry.url);
      return url.origin === base && (url.pathname === '/shell/profile' || url.pathname === '/shell/action');
    } catch { return false; }
  };
  report.expectedConsole = report.console.filter(expected401);
  const unexpectedConsole = report.console.filter(entry => !expected401(entry));
  check(report.expectedConsole.length === 1 && report.expectedConsole[0].url === `${base}/shell/profile`, 'exact intentional signout error console multiset', JSON.stringify(report.expectedConsole));
  check(report.blocked.length === 0, 'all observed requests within allowlist', JSON.stringify(report.blocked));
  check(report.websockets.length === 0, 'no WebSocket attempted', JSON.stringify(report.websockets));
  check(unexpectedConsole.length === 0, 'no unexpected browser console errors', JSON.stringify(unexpectedConsole));
  check(report.pageErrors.length === 0, 'no page errors', JSON.stringify(report.pageErrors));
  report.ok = report.failures.length === 0;
  await writeFile(reportPath, `${JSON.stringify(report, null, 2)}\n`);
  await browser.close();
}
console.log(JSON.stringify({ok: report.ok, passes: report.passes.length, failures: report.failures.length, reportPath}, null, 2));
process.exitCode = report.ok ? 0 : 1;
