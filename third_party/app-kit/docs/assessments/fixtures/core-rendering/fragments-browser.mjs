// Bounded native-fragment probe. Existing local Playwright and installed Chrome
// only; no installation, signed-in profile, external content or application JS.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {copyFileSync, mkdirSync, readFileSync, writeFileSync} from 'node:fs';
import http from 'node:http';
import net from 'node:net';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';

const [fixturePath, output, modulePath, executablePath] = process.argv.slice(2);
for (const selected of [fixturePath, output, modulePath, executablePath]) {
  assert(selected && path.isAbsolute(selected), 'four explicit absolute paths required');
}
mkdirSync(output, {mode: 0o700}); // Refuse an existing evidence directory.
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const identity = filename => ({path: filename, sha256: hash(readFileSync(filename))});
const input = readFileSync(fixturePath);
const fixtures = JSON.parse(input);
assert.equal(fixtures.length, 24, 'fixed four-renderer, six-ID workload');
const self = fileURLToPath(import.meta.url);
copyFileSync(self, path.join(output, 'probe.mjs'));
copyFileSync(fixturePath, path.join(output, 'fixture.json'));
const receipt = {
  schema: 'appkit-literal-fragments-v1',
  inputs: {fixture: identity(fixturePath), probe: identity(self), node: identity(process.execPath),
    playwright: identity(modulePath), playwrightPackage: identity(path.join(path.dirname(modulePath), 'package.json')),
    browser: identity(executablePath)},
  nodeVersion: process.version,
  observations: [], deniedRequests: [], consoleErrors: [], cleanup: {}, passed: false,
};
const server = http.createServer((request, response) => {
  const match = /^\/case\/(\d+)$/.exec(request.url);
  const selected = match && fixtures[Number(match[1])];
  if (request.method !== 'GET' || !selected) {
    response.writeHead(404).end();
    return;
  }
  response.writeHead(200, {'Content-Type': 'text/html; charset=utf-8',
    'Cache-Control': 'no-store',
    'Content-Security-Policy': "default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"});
  response.end(selected.html);
});
let browserServer, browser, context;
const within = async (promise, label) => {
  let timer;
  try {
    return await Promise.race([promise, new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error(label + ' exceeded 10 seconds')), 10_000);
    })]);
  } finally { clearTimeout(timer); }
};
try {
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const base = 'http://127.0.0.1:' + server.address().port;
  receipt.origin = base;
  const {chromium} = await import(pathToFileURL(modulePath).href);
  browserServer = await chromium.launchServer({executablePath, headless: true, timeout: 15_000});
  receipt.browserPID = browserServer.process().pid;
  browser = await chromium.connect(browserServer.wsEndpoint());
  receipt.browserVersion = browser.version();
  context = await browser.newContext({javaScriptEnabled: false, viewport: {width: 390, height: 844}});
  context.setDefaultTimeout(5000);
  await context.route('**/*', async route => {
    const u = new URL(route.request().url());
    if (u.origin === base && /^\/case\/\d+$/.test(u.pathname) && route.request().method() === 'GET') {
      await route.continue();
    } else {
      receipt.deniedRequests.push({url: u.href, method: route.request().method()});
      await route.abort();
    }
  });
  for (const [index, fixture] of fixtures.entries()) {
    const page = await context.newPage();
    page.on('pageerror', error => receipt.consoleErrors.push(String(error)));
    const observation = {name: fixture.name, expectedID: fixture.id, passed: false};
    try {
      const response = await page.goto(base + '/case/' + index, {waitUntil: 'load'});
      assert.equal(response.status(), 200);
      assert.equal(hash(await response.body()), hash(Buffer.from(fixture.html)), 'served exact selected renderer output');
      let linkFocused = false;
      for (let tabs = 0; tabs < 4; tabs++) {
        await page.keyboard.press('Tab');
        linkFocused = await page.evaluate(() => document.activeElement?.tagName === 'A');
        if (linkFocused) break;
      }
      assert(linkFocused, 'native Tab must reach the renderer link');
      await page.keyboard.press('Enter');
      await page.waitForURL(url => url.hash !== '', {timeout: 5000});
      // Read-only observation; no script, DOM mutation or scripted focus repair.
      const actual = await page.evaluate(() => ({
        target: document.querySelector(':target')?.id ?? null,
        focused: document.activeElement?.id ?? null,
        hash: location.hash, scripts: document.scripts.length,
      }));
      Object.assign(observation, actual);
      assert.equal(actual.scripts, 0);
      assert.equal(actual.target, fixture.id, 'literal target, not percent-decoded sibling');
      assert.equal(actual.focused, fixture.id, 'native fragment navigation transfers focus');
      observation.passed = true;
    } catch (error) { observation.error = String(error); }
    receipt.observations.push(observation);
    if (!observation.passed || index === 0) {
      await page.screenshot({path: path.join(output, 'case-' + index + '.png'), fullPage: true});
    }
    await page.close();
  }
} catch (error) { receipt.error = String(error); }
finally {
  for (const [name, close] of [
    ['context', () => context?.close()], ['browserConnection', () => browser?.close()],
    ['browserProcess', () => browserServer?.close()],
  ]) {
    try { await within(Promise.resolve().then(close), name); receipt.cleanup[name] = true; }
    catch (error) { receipt.cleanup[name] = false; receipt.cleanup[name + 'Error'] = String(error); }
  }
  if (browserServer) {
    receipt.cleanup.browserExited = browserServer.process().exitCode !== null || browserServer.process().signalCode !== null;
    if (!receipt.cleanup.browserExited) {
      try { await within(browserServer.kill(), 'browser kill'); }
      catch (error) { receipt.cleanup.killError = String(error); }
    }
  }
  const port = server.address()?.port;
  server.closeAllConnections();
  try { await within(new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve())), 'server'); receipt.cleanup.serverClosed = true; }
  catch (error) { receipt.cleanup.serverClosed = false; receipt.cleanup.serverError = String(error); }
  if (port) {
    receipt.cleanup.portRefused = await new Promise(resolve => {
      const socket = net.connect({host: '127.0.0.1', port});
      const finish = result => { socket.destroy(); resolve(result); };
      socket.once('connect', () => finish(false));
      socket.once('error', error => finish(error.code === 'ECONNREFUSED'));
      socket.setTimeout(1000, () => finish(false));
    });
  }
  receipt.passed = !receipt.error && receipt.observations.length === 24 &&
    receipt.observations.every(row => row.passed) && receipt.deniedRequests.length === 0 &&
    receipt.consoleErrors.length === 0 && ['context', 'browserConnection', 'browserProcess',
      'browserExited', 'serverClosed', 'portRefused'].every(key => receipt.cleanup[key] === true);
  writeFileSync(path.join(output, 'receipt.json'), JSON.stringify(receipt, null, 2) + '\n', {flag: 'wx', mode: 0o600});
  console.log(JSON.stringify({output, passed: receipt.passed,
    cases: receipt.observations.length, failures: receipt.observations.filter(row => !row.passed).map(row => ({name: row.name, error: row.error})),
    cleanup: receipt.cleanup}));
  if (!receipt.passed) process.exitCode = 1;
}
