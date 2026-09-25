import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs/promises';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {trackOwnedChild, terminateOwned} from '../bar-resolution-2026-09-22/browser-owned_process.mjs';

const [manifestPath, output] = process.argv.slice(2);
assert(manifestPath?.startsWith('/') && output?.startsWith('/'));
const manifest = JSON.parse(await fs.readFile(manifestPath, 'utf8')), I = manifest.inputs;
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const canonical = value => { const u = new URL(value, 'http://127.0.0.1'); u.searchParams.sort(); return u.pathname + (u.search ? `?${u.searchParams}` : ''); };
const report = {startedAt: new Date().toISOString(), checks: [], errors: [], unexpectedRequests: [], cells: [], cleanup: {}};
const check = (ok, name, detail = {}) => { report.checks.push({ok, name, detail}); if (!ok) report.errors.push({name, detail}); };
const bound = async (promise, ms, name) => { let timer; try { return await Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(Error(`${name} timeout`)), ms); })]); } finally { clearTimeout(timer); } };
await fs.mkdir(output, {mode: 0o700});
let server, browser, owned;
const watchdog = setTimeout(() => { report.timedOut = true; try { owned?.child.kill('SIGTERM'); } catch {} }, 90000);
try {
  for (const input of Object.values(I)) if (input.path) { assert(input.path.startsWith('/')); assert.equal(sha(await fs.readFile(input.path)), input.sha256); }
  assert.equal(fileURLToPath(import.meta.url), I.driver.path);
  assert.equal(fileURLToPath(new URL('../bar-resolution-2026-09-22/browser-owned_process.mjs', import.meta.url)), I.helper.path);
  assert.equal(process.execPath, I.node.path); assert.equal(process.version, manifest.node_version);
  const pkg = JSON.parse(await fs.readFile(new URL('package.json', pathToFileURL(I.pwPath.path)), 'utf8')); assert.equal(pkg.version, manifest.pw_version);
  const packet = JSON.parse(await fs.readFile(I.responses.path, 'utf8')), responses = packet.Responses;
  const {chromium} = await import(pathToFileURL(I.pwPath.path).href);
  server = await chromium.launchServer({executablePath: I.chromePath.path, headless: true, args: ['--no-proxy-server', '--disable-background-networking']});
  owned = trackOwnedChild(server.process(), {name: 'chrome'}); browser = await chromium.connect(server.wsEndpoint()); assert.equal(browser.version(), manifest.chrome_version);
  for (const lane of [{name: 'wide-light', width: 1440, theme: 'light'}, {name: 'narrow-dark', width: 390, theme: 'dark'}]) {
    const routes = packet.Routes[lane.theme]; let context;
    try {
      context = await browser.newContext({viewport: {width: lane.width, height: 1000}, colorScheme: lane.theme, javaScriptEnabled: false, serviceWorkers: 'block'}); context.setDefaultTimeout(5000);
      await context.route('**/*', route => { const request = route.request(), u = new URL(request.url()), body = responses[canonical(u.pathname + u.search)]; const allowed = request.method() === 'GET' && u.origin === 'http://127.0.0.1' && body; if (!allowed) report.unexpectedRequests.push({lane: lane.name, url: request.url()}); return allowed ? route.fulfill({status: body.Status, headers: Object.fromEntries(Object.entries(body.Headers).map(([k, v]) => [k.toLowerCase(), v.join(', ')])), body: body.Body}) : route.abort('blockedbyclient'); });
      const page = await context.newPage();
      async function observe(action, path, status) {
        const key = canonical(path);
        const docReady = page.waitForResponse(r => new URL(r.url()).origin === 'http://127.0.0.1' && canonical(r.url()) === key && r.request().isNavigationRequest());
        const cssReady = page.waitForResponse(r => new URL(r.url()).pathname === '/static/app.css');
        const completed = Promise.all([docReady, cssReady]); completed.catch(() => {});
        await action(); const [doc, css] = await completed; await page.waitForLoadState('load');
        await Promise.all([doc.finished(), css.finished(), bound(page.evaluate(() => document.fonts.ready), 3000, 'fonts')]);
        const body = await doc.body(), cssBody = await css.body();
        check(canonical(page.url()) === key && doc.status() === status && sha(body) === sha(responses[key].Body), `${lane.name} exact document ${path}`, {status: doc.status(), actualURL: page.url()});
        check(sha(cssBody) === sha(responses['/static/app.css'].Body), `${lane.name} exact completed CSS ${path}`);
        check(await page.locator('script').count() === 0 && await page.evaluate(t => document.documentElement.dataset.theme === t, lane.theme), `${lane.name} script-free/theme ${path}`);
        check(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `${lane.name} no overflow ${path}`);
      }
      async function focusAndEnter(href) { const link = page.locator('main a[href=' + JSON.stringify(href) + ']'); assert.equal(await link.count(), 1); for (let i = 0; i < 120; i++) { await page.keyboard.press('Tab'); if (await link.evaluate(e => e === document.activeElement)) break; } const visible = await link.evaluate(e => { const r = e.getBoundingClientRect(), s = getComputedStyle(e); return e === document.activeElement && r.height >= 44 && r.left >= 0 && r.right <= innerWidth && s.outlineStyle !== 'none'; }); check(visible, `${lane.name} keyboard focus ${href}`); await page.keyboard.press('Enter'); }
      async function submitScope() {
        const form = page.locator('form[aria-label="Quality work filters"]'), scope = form.locator('select[name="scope"]'), button = form.locator('button');
        for (let i = 0; i < 120; i++) { await page.keyboard.press('Tab'); if (await scope.evaluate(e => e === document.activeElement)) break; }
        assert(await scope.evaluate(e => e === document.activeElement), 'native Tab did not reach scope');
        await page.keyboard.press('a'); check(await scope.inputValue() === 'all', `${lane.name} native scope selection`, {value: await scope.inputValue()});
        for (let i = 0; i < 8; i++) { await page.keyboard.press('Tab'); if (await button.evaluate(e => e === document.activeElement)) break; }
        check(await button.evaluate(e => { const r = e.getBoundingClientRect(), s = getComputedStyle(e); return e === document.activeElement && r.height >= 44 && r.top >= 0 && r.bottom <= innerHeight && s.outlineStyle !== 'none'; }), `${lane.name} keyboard filter submit`);
        await page.keyboard.press('Enter');
      }
      await observe(() => page.goto('http://127.0.0.1' + routes.CatalogDetail), routes.CatalogDetail, 200); assert.equal(await page.getByRole('heading', {name: 'Meaning-bearing analytics', exact: true}).count(), 1);
      await observe(() => focusAndEnter(routes.QualityList), routes.QualityList, 200); check((await page.locator('main').textContent()).includes('core.analytics'), `${lane.name} quality list contains selected work`);
      const guardedDetail = new URL(await page.locator('main a[href^="/quality/core.analytics"]').first().getAttribute('href'), 'http://x'); check(guardedDetail.searchParams.get('expected_catalog_revision') === packet.CatalogRevision && guardedDetail.searchParams.get('expected_revision') === packet.QualityRevision, `${lane.name} detail preserves catalog and quality identity`);
      await observe(() => focusAndEnter(routes.QualityDetail), routes.QualityDetail, 200); const detailText = await page.locator('main').textContent(); check(detailText.includes('Seven enhancement standards') && detailText.includes('Nine recorded gates'), `${lane.name} quality detail standards/gates`); check(detailText.includes('Authored planning, not certification.') && !/certified ready|certification achieved|promotion (?:ready|approved|granted)/i.test(detailText), `${lane.name} no certification claim`);
      await observe(() => focusAndEnter(routes.QualityBack), routes.QualityBack, 200); const form = page.locator('form[aria-label="Quality work filters"]'); check(await form.locator('input[name="expected_catalog_revision"]').count() === 1 && await form.locator('select[name="scope"]').inputValue() === 'core', `${lane.name} guarded core filter`);
      await observe(() => submitScope(), routes.QualityAll, 200); check(await page.locator('select[name="scope"]').inputValue() === 'all', `${lane.name} native all-scope filter`); await observe(() => focusAndEnter(routes.QualityAllDetail), routes.QualityAllDetail, 200);
      await observe(() => page.goto('http://127.0.0.1' + routes.QualityStale), routes.QualityStale, 409); const staleText = await page.locator('main').textContent(); check(await page.getByRole('heading', {name: 'This quality-work request needs a reset', exact: true}).count() === 1 && !staleText.includes('Seven enhancement standards') && staleText.includes('Your application and adoption pins have not changed.'), `${lane.name} stale quality refuses item`); check(await page.getByRole('link', {name: 'Review current capability catalog', exact: true}).count() === 1, `${lane.name} stale quality offers catalog recovery`); await page.screenshot({path: `${output}/${lane.name}-stale.png`, fullPage: true});
      await observe(() => focusAndEnter(routes.CatalogRecovery), routes.CatalogRecovery, 200); await observe(() => focusAndEnter(routes.CatalogRecoveredDetail), routes.CatalogRecoveredDetail, 200);
      await observe(() => focusAndEnter(routes.QualityList), routes.QualityList, 200);
      check((await page.locator('main').textContent()).includes('core.analytics'), `${lane.name} recovered capability hands off to quality work`);
      await page.screenshot({path: `${output}/${lane.name}-recovered.png`, fullPage: true}); report.cells.push({lane: lane.name, routes});
    } finally { if (context) await bound(context.close(), 5000, 'context close'); }
  }
} catch (error) { report.errors.push({name: 'run error', detail: String(error.stack || error)}); }
finally { try { if (browser) await bound(browser.close(), 5000, 'browser close'); } catch (e) { report.cleanup.browserCloseError = String(e); report.errors.push({name: 'browser close', detail: String(e)}); } try { if (server) await bound(server.close(), 5000, 'server close'); } catch (e) { report.cleanup.serverCloseError = String(e); report.errors.push({name: 'server close', detail: String(e)}); } if (owned) { try { report.cleanup.chrome = await terminateOwned(owned, {graceMs: 3000, killMs: 2000, drainMs: 2000}); if (!report.cleanup.chrome.ok) report.errors.push({name: 'chrome cleanup', detail: report.cleanup.chrome}); } catch (e) { report.cleanup.chromeError = String(e); report.errors.push({name: 'chrome cleanup', detail: String(e)}); } } clearTimeout(watchdog); report.endedAt = new Date().toISOString(); report.pass = !report.timedOut && report.errors.length === 0 && report.unexpectedRequests.length === 0 && report.cells.length === 2 && report.cleanup.chrome?.ok === true; await fs.writeFile(`${output}/report.json`, JSON.stringify(report, null, 2) + '\n', {flag: 'wx'}); console.log(JSON.stringify({pass: report.pass, failures: report.errors.length})); process.exitCode = report.pass ? 0 : 1; }
