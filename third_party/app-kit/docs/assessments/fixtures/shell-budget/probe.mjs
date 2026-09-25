// Local shell-budget v3 proof. It measures a supplied Gallery binary; it never builds it.
import assert from 'node:assert/strict';
import {spawn, execFile as execFileCB} from 'node:child_process';
import {createHash} from 'node:crypto';
import {promisify} from 'node:util';
import fs from 'node:fs/promises';
import net from 'node:net';
import path from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';

const execFile = promisify(execFileCB);
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const finite = value => Number.isFinite(value) && value > 0;

export function assessNavigation(contract, navigation) {
  const failures = [];
  const fail = (code, detail = {}) => failures.push({code, detail});
  const budget = contract?.budget;
  if (!budget || !Number.isSafeInteger(budget.decodedResponseBodyBytes)) return [{code: 'contract.budget'}];
  const timing = navigation?.timing;
  for (const [name, limit] of Object.entries({
    responseEndMs: budget.responseEndMs,
    domContentLoadedMs: budget.domContentLoadedMs,
    loadEventMs: budget.loadEventMs,
  })) {
    const value = timing?.[name];
    if (!finite(value)) fail('timing.missing', {name, value});
    else if (value > limit) fail('timing.overbudget', {name, value, limit});
  }
  if (!(timing?.responseEndMs <= timing?.domContentLoadedMs && timing?.domContentLoadedMs <= timing?.loadEventMs)) fail('timing.order', {timing});
  if (!Array.isArray(navigation?.resources) || navigation.resources.length === 0) fail('resources.missing');
  let decoded = 0;
  for (const resource of navigation?.resources || []) {
    if (!resource || resource.terminal !== true) { fail('resource.unsettled', {resource}); continue; }
    if (resource.fromCache === true) fail('resource.cache-served', {resource});
    if (resource.bodyAvailable !== true || !Number.isSafeInteger(resource.decodedBodyBytes) || resource.decodedBodyBytes < 0) {
      fail('resource.body-unavailable', {resource}); continue;
    }
    if (!Number.isSafeInteger(resource.status) || resource.status < 100 || resource.status > 599) fail('resource.status', {resource});
    if (resource.expected !== true) fail('request.forbidden', {resource});
    decoded += resource.decodedBodyBytes;
  }
  if (decoded > budget.decodedResponseBodyBytes) fail('resources.overbudget', {decoded, limit: budget.decodedResponseBodyBytes});
  if (navigation?.taskPassed !== true) fail('task.failed', {taskPassed: navigation?.taskPassed});
  if (navigation?.forbiddenAttempts?.length) fail('request.forbidden', {attempts: navigation.forbiddenAttempts});
  return failures;
}

export function assessResourceChain(resources, documents, assets) {
  const failures = [];
  const docs = resources.filter(resource => !new URL(resource.url).pathname.startsWith('/static/'));
  const actualAssets = resources.filter(resource => new URL(resource.url).pathname.startsWith('/static/'));
  if (docs.length !== documents.length) failures.push({code:'chain.document-count', detail:{actual:docs.length,expected:documents.length}});
  for (const [index, want] of documents.entries()) { const resource = docs[index]; if (!resource || resource.method !== want.method || new URL(resource.url).pathname !== want.path || resource.status !== want.status) failures.push({code:'chain.document',detail:{index,resource,expected:want}}); }
  if (actualAssets.length !== assets.length) failures.push({code:'chain.asset-count',detail:{actual:actualAssets.length,expected:assets.length}});
  for (const want of assets) { const resource = actualAssets.find(item => new URL(item.url).pathname === want.path); if (!resource || resource.status !== 200 || resource.decodedBodyBytes !== want.bytes || resource.sha256 !== want.sha256) failures.push({code:'chain.asset',detail:{resource,expected:want}}); }
  return failures;
}

const main = async () => {
  const [parentArg, binaryArg, cssArg] = process.argv.slice(2);
  for (const [name, value] of [['evidence parent', parentArg], ['Gallery binary', binaryArg], ['CSS', cssArg]]) assert.ok(value && path.isAbsolute(value), `${name} must be an absolute path`);
  const source = fileURLToPath(import.meta.url);
  const rootSource = path.dirname(source);
  const [parent, binary, css] = await Promise.all([fs.realpath(parentArg), fs.realpath(binaryArg), fs.realpath(cssArg)]);
  const {stdout: contractText} = await execFile('go', ['tool', 'cue', 'export', '-e', 'contract', 'contract.cue'], {cwd: rootSource, maxBuffer: 1024 * 1024});
  const contractBytes = Buffer.from(contractText);
  const contract = JSON.parse(contractText);
  const scriptSource = path.resolve(rootSource, '../../../../apps/gallery/shell_demo.js');
  const [sourceBytes, contractSourceBytes, binaryBytes, cssBytes, scriptBytes] = await Promise.all([fs.readFile(source), fs.readFile(path.join(rootSource, 'contract.cue')), fs.readFile(binary), fs.readFile(css), fs.readFile(scriptSource)]);
  assert.equal(sha(binaryBytes), contract.inputs.gallerySHA256, 'unapproved Gallery binary');
  assert.equal(sha(cssBytes), contract.inputs.cssSHA256, 'unapproved CSS');
  assert.equal(sha(scriptBytes), contract.inputs.shellDemoJS.sha256, 'unapproved shell demo JS'); assert.equal(scriptBytes.length, contract.inputs.shellDemoJS.bytes, 'shell demo JS size');
  const playwrightPath = process.env.APPKIT_PLAYWRIGHT_MODULE;
  const chromePath = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
  assert.ok(playwrightPath && path.isAbsolute(playwrightPath), 'APPKIT_PLAYWRIGHT_MODULE must be absolute');
  await fs.access(chromePath);
  const playwrightBytes = await fs.readFile(playwrightPath);
  assert.equal(sha(playwrightBytes), 'a0f5715ea22354f922791a9c53dc012d5d5c067ff9cc4cd35ffb7cd272071a9f', 'unapproved Playwright entry');
  const root = await fs.mkdtemp(path.join(parent, 'shell-budget-'));
  await fs.chmod(root, 0o700);
  const write = (name, bytes) => fs.writeFile(path.join(root, name), bytes, {flag: 'wx', mode: 0o600});
  await Promise.all([write('probe.mjs', sourceBytes), write('contract.cue', contractSourceBytes), write('contract.json', contractBytes), write('gallery', binaryBytes), write('expected-app.css', cssBytes), write('expected-shell-demo.js', scriptBytes)]);
  await fs.chmod(path.join(root, 'gallery'), 0o700);
  const report = {kind: contract.version, root, inputs: {harnessSHA256: sha(sourceBytes), contractSHA256: sha(contractBytes), contractSourceSHA256: sha(contractSourceBytes), binarySHA256: sha(binaryBytes), cssSHA256: sha(cssBytes), playwrightSHA256: sha(playwrightBytes), chromePath}, contract, observations: [], failures: [], requests: [], limits: ['Local unthrottled engineering limits on this host only; not a production SLO, human-task metric, warm-cache result, native proof, or consumer acceptance.', 'Decoded response bodies are counted; transfer and encoded size are not used.', 'Missing, unavailable, non-finite, forbidden, error, or unknown resource facts fail closed.']};
  let gallery, browserServer, browser, galleryExit, stdout = '', stderr = '', galleryPort;
  const bounded = (promise, ms, label) => Promise.race([promise, new Promise((_, reject) => setTimeout(() => reject(Error(`${label} timeout`)), ms))]);
  try {
    gallery = spawn(path.join(root, 'gallery'), ['-addr', '127.0.0.1:0', '-design-assets', ''], {cwd: root, stdio: ['ignore', 'pipe', 'pipe']});
    galleryExit = new Promise(resolve => gallery.once('close', resolve));
    gallery.stdout.on('data', b => { stdout += b; }); gallery.stderr.on('data', b => { stderr += b; });
    const origin = await bounded(new Promise((resolve, reject) => { gallery.once('error', reject); gallery.once('exit', () => reject(Error('Gallery exited before readiness'))); gallery.stderr.on('data', () => { const hit = stderr.match(/app-kit Gallery → (http:\/\/127\.0\.0\.1:\d+)/); if (hit) resolve(hit[1]); }); }), 15000, 'Gallery readiness');
    report.origin = origin; galleryPort = Number(new URL(origin).port);
    const {chromium} = await import(pathToFileURL(playwrightPath));
    browserServer = await chromium.launchServer({headless: true, executablePath: chromePath, args: ['--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1']});
    browser = await chromium.connect(browserServer.wsEndpoint()); report.chromeVersion = browser.version();
    for (const lane of contract.lanes) {
      const context = await browser.newContext({viewport: {width: lane.width, height: lane.height}, colorScheme: lane.theme, javaScriptEnabled: lane.javaScriptEnabled, serviceWorkers: 'block'});
      const cdp = await context.newCDPSession(await context.newPage()); await cdp.send('Network.enable'); await cdp.send('Network.setCacheDisabled', {cacheDisabled: true});
      cdp.on('Network.requestServedFromCache', event => report.failures.push({code: 'resource.cache-served', detail: {lane, requestId: event.requestId}}));
      const page = context.pages()[0]; page.setDefaultTimeout(7000);
      let active = null; const ledger = new Map(); let sequence = 0;
      const expected = (method, url) => { const u = new URL(url); return u.origin === origin && ((method === 'GET' && [contract.paths.profile, contract.paths.settings, contract.paths.css, ...(lane.javaScriptEnabled ? ['/static/shell-demo.js'] : [])].includes(u.pathname)) || (method === 'POST' && u.pathname === contract.paths.action)); };
      await context.route('**/*', route => { const request = route.request(), ok = expected(request.method(), request.url()); const item = {lane, method: request.method(), url: request.url(), expected: ok}; report.requests.push(item); if (!ok) { report.failures.push({code: 'request.forbidden-attempt', detail: item}); if (active) active.forbiddenAttempts.push(item); return route.abort(); } return route.continue(); });
      page.on('request', request => { if (!active) return; const item = {sequence: ++sequence, url: request.url(), method: request.method(), expected: expected(request.method(), request.url()), terminal: false, bodyAvailable: false}; active.resources.push(item); ledger.set(request, item); });
      page.on('response', response => { const item = ledger.get(response.request()); if (!item) return; item.status = response.status(); item.headers = response.headers(); item.fromCache = response.fromServiceWorker(); item.bodyPromise = response.body().then(body => { item.bodyAvailable = true; item.decodedBodyBytes = body.length; item.sha256 = sha(body); item.terminal = true; }).catch(error => { const h = item.headers || {}; const protocolEmpty = h['content-length'] === '0' && !h['transfer-encoding'] && !h['content-encoding']; if (protocolEmpty) { item.bodyAvailable = true; item.decodedBodyBytes = 0; item.protocolDeclaredEmpty = true; } else item.bodyError = String(error); item.terminal = true; }); });
      page.on('requestfailed', request => { const item = ledger.get(request); if (item) { item.failed = request.failure(); item.terminal = true; } });
      page.on('pageerror', error => { if (active) active.pageError = String(error); });
      const assets = () => [{path:contract.paths.css,bytes:cssBytes.length,sha256:sha(cssBytes)}, ...(lane.javaScriptEnabled ? [{path:'/static/shell-demo.js',bytes:contract.inputs.shellDemoJS.bytes,sha256:contract.inputs.shellDemoJS.sha256}] : [])];
      async function finish(stage, taskPassed, documents) { await page.waitForLoadState('networkidle'); for (let i = 0; i < 100 && active.resources.some(item => item.terminal !== true); i++) await page.waitForTimeout(10); await Promise.all(active.resources.map(async item => { try { await bounded(item.bodyPromise || new Promise(() => {}), 1000, 'response body'); } catch (error) { item.bodyError = String(error); item.terminal = false; } })); active.timing = await page.evaluate(() => { const n = performance.getEntriesByType('navigation').at(-1); const body=getComputedStyle(document.body), root=getComputedStyle(document.documentElement); return {responseEndMs:n?.responseEnd-n?.startTime, domContentLoadedMs:n?.domContentLoadedEventEnd-n?.startTime, loadEventMs:n?.loadEventEnd-n?.startTime, visual:{polarity:root.getPropertyValue('--ak-polarity').trim(),backgroundColor:body.backgroundColor,color:body.color}}; }); active.visual = active.timing?.visual; active.taskPassed = taskPassed && active.visual?.polarity === lane.theme && !['','transparent','rgba(0, 0, 0, 0)'].includes(active.visual?.backgroundColor) && !['','transparent','rgba(0, 0, 0, 0)'].includes(active.visual?.color) && !active.pageError; const failures = [...assessNavigation(contract, active), ...assessResourceChain(active.resources, documents, assets())]; report.observations.push({...active, decodedResponseBodyBytes: active.resources.reduce((sum, r) => sum + (r.decodedBodyBytes || 0), 0), failures}); report.failures.push(...failures.map(failure => ({lane, stage, ...failure}))); active = null; ledger.clear(); }
      async function visit(pathname, stage) { active = {lane, stage, resources: [], forbiddenAttempts: [], taskPassed: false}; const response = await page.goto(origin + pathname, {waitUntil: 'load'}); const field = pathname === contract.paths.profile ? '#shell-display_name' : '#shell-timezone'; const save = page.locator('[data-shell-save]'); const heading = pathname === contract.paths.profile ? 'Your profile' : 'Your preferences'; active.taskFacts = {heading: (await page.locator('main h2').allTextContents()).includes(heading), fieldVisible: await page.locator(field).isVisible(), fieldEnabled: await page.locator(field).isEnabled(), saveVisible: await save.isVisible(), saveEnabled: await save.isEnabled()}; const task = Boolean(response) && active.taskFacts.heading && active.taskFacts.fieldVisible && active.taskFacts.fieldEnabled && active.taskFacts.saveVisible && active.taskFacts.saveEnabled === !lane.javaScriptEnabled && await page.locator('link[href="/static/app.css"]').count() === 1; await finish(stage, task, [{method:'GET', path:pathname, status:200}]); }
      async function recovery(rep) { await visit(contract.paths.profile, `profile-before-recovery-${rep}`); const fixture = page.locator('details').filter({has: page.locator('#shell-availability')}); await fixture.locator('summary').click(); const open = await fixture.evaluate(node => node.open); await page.locator('#shell-availability').selectOption('error'); async function submit(button) { const before = await page.evaluate(() => performance.timeOrigin); const post = page.waitForRequest(request => request.method() === 'POST' && new URL(request.url()).pathname === contract.paths.action); const navigation = page.waitForFunction(old => performance.timeOrigin > old, before); await Promise.all([post, navigation, button.click()]); } active = {lane, stage: `load-failure-${rep}`, resources: [], forbiddenAttempts: [], taskPassed: false}; await submit(page.locator('form input[name="action"][value="fixture"]').locator('..').locator('button')); const failed = await page.locator('#shell-unavailable-title').textContent(); const retry = page.getByRole('button', {name: 'Retry loading account'}); active.taskFacts = {disclosureOpen:open,errorHeading:failed?.includes('could not be loaded'),retryVisible:await retry.isVisible(),retryEnabled:await retry.isEnabled()}; await finish(active.stage, Object.values(active.taskFacts).every(Boolean) && await page.locator('link[href="/static/app.css"]').count() === 1, [{method:'POST',path:contract.paths.action,status:303},{method:'GET',path:contract.paths.profile,status:503}]); active = {lane, stage: `load-retry-${rep}`, resources: [], forbiddenAttempts: [], taskPassed: false}; await submit(retry); const save = page.locator('[data-shell-save]'); active.taskFacts = {heading:(await page.locator('main h2').allTextContents()).includes('Your profile'),fieldVisible:await page.locator('#shell-display_name').isVisible(),fieldEnabled:await page.locator('#shell-display_name').isEnabled(),saveVisible:await save.isVisible(),saveEnabled:await save.isEnabled()}; await finish(active.stage, active.taskFacts.heading && active.taskFacts.fieldVisible && active.taskFacts.fieldEnabled && active.taskFacts.saveVisible && active.taskFacts.saveEnabled === !lane.javaScriptEnabled && await page.locator('link[href="/static/app.css"]').count() === 1, [{method:'POST',path:contract.paths.action,status:303},{method:'GET',path:contract.paths.profile,status:200}]); }
      for (let rep = 1; rep <= contract.repetitions; rep++) { await visit(contract.paths.profile, `profile-ready-${rep}`); await visit(contract.paths.settings, `settings-ready-${rep}`); await recovery(rep); }
      await cdp.detach(); await context.close();
    }
  } catch (error) { report.failures.push({code: 'execution', detail: String(error.stack || error)}); }
  finally {
    if (browser) try { await browser.close(); } catch (error) { report.failures.push({code: 'cleanup.browser-client', detail: String(error)}); }
    if (browserServer) { const process = browserServer.process(); try { await browserServer.close(); } catch (error) { report.failures.push({code: 'cleanup.browser', detail: String(error)}); } report.browserProcess = {pid:process?.pid,exitCode:process?.exitCode,signalCode:process?.signalCode}; if (process && process.exitCode === null && process.signalCode === null) report.failures.push({code: 'cleanup.browser-exit', detail: report.browserProcess}); }
    if (gallery) { gallery.kill('SIGTERM'); await bounded(galleryExit, 8000, 'Gallery close').catch(error => { report.failures.push({code: 'cleanup.gallery', detail: String(error)}); gallery.kill('SIGKILL'); }); report.galleryProcess = {pid:gallery.pid,exitCode:gallery.exitCode,signalCode:gallery.signalCode}; if (gallery.exitCode !== 0 || gallery.signalCode !== null) report.failures.push({code: 'cleanup.gallery-exit', detail: report.galleryProcess}); }
    if (galleryPort) await new Promise(resolve => { const socket = net.connect({host: '127.0.0.1', port: galleryPort}); socket.once('connect', () => { report.cleanupPortRefused = false; report.failures.push({code: 'cleanup.gallery-port-open', detail: galleryPort}); socket.destroy(); resolve(); }); socket.once('error', () => { report.cleanupPortRefused = true; resolve(); }); });
    await write('gallery.stdout', stdout); await write('gallery.stderr', stderr); report.passed = report.failures.length === 0; await write('receipt.json', JSON.stringify(report, null, 2) + '\n'); console.log(JSON.stringify({root, passed: report.passed, observations: report.observations.length, failures: report.failures.length}, null, 2)); process.exitCode = report.passed ? 0 : 1;
  }
};

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await main();
