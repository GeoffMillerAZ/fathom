// Explicit private source/compiled runtime pair. No consumer is changed or certified.
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import crypto from 'node:crypto';
import zlib from 'node:zlib';
import {pathToFileURL} from 'node:url';
import {parsePairManifest, sourceManifestSHA256, validatePairBindings, declaredImageRefusalsOnly} from './contract.mjs';

if (process.argv.length !== 4) throw Error('usage: node browser.mjs /private/prepared-pair EXPECTED_PAIR_MANIFEST_SHA256');
const root = fs.realpathSync(process.argv[2] || '');
const expectedPairSHA256 = process.argv[3];
const sha = b => crypto.createHash('sha256').update(b).digest('hex');
const manifestBytes = fs.readFileSync(path.join(root, 'manifest.json'));
const manifest = parsePairManifest(manifestBytes, expectedPairSHA256);
const sourcePins = {
  'P03-faq': '9114b0b981055638eb28fede17eff7bc49c91f40fc32ce758514bfdb7d19f937',
  'P07-menu': '10ed82d0c337ad246f1ab8687d73cc0a7dc5d834bb7b3ec46a01bc7c5d5f144a',
  'P08-nested-dialog-menu': '1fbde54a0be874618cc295bf3b837fee4d89f38a1543dd9a6e2374bbd1d13415',
  'P09-mobile-nav': '79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7',
};
const files = new Map();
for (const [name, id] of Object.entries(manifest.files)) {
  if (!/^[a-zA-Z0-9._-]+$/.test(name)) throw Error('non-flat artifact name');
  const file = path.join(root, name), stat = fs.lstatSync(file);
  if (!stat.isFile() || stat.size > 8 * 1024 * 1024) throw Error('invalid artifact');
  const body = fs.readFileSync(file);
  if (body.length !== id.size || sha(body) !== id.sha256) throw Error(`artifact changed: ${name}`);
  files.set(name, body);
}
if (manifest.kind !== 'elements-original-compiled-pair' || manifest.verification !== 'not-verified' ||
    manifest.source_manifest_sha256 !== sourceManifestSHA256 || sha(files.get('source-manifest.json')) !== sourceManifestSHA256 ||
    !Array.isArray(manifest.cases) || manifest.cases.length !== 4 ||
    new Set(manifest.cases.map(c => c.id)).size !== 4) throw Error('invalid reference manifest');
validatePairBindings(manifest, files, JSON.parse(files.get('source-manifest.json')));
for (const item of manifest.cases) {
  if (!sourcePins[item.id] || sha(files.get(item.fragments.source)) !== sourcePins[item.id] || !files.has(item.fragments.compiled)) throw Error('source binding mismatch');
  for (const theme of ['light', 'dark']) if (!files.has(item.source_documents[theme]) || !files.has(item.compiled_documents[theme])) throw Error('missing paired document');
  for (const name of [item.receipt, item.emitted_templ, item.formatted_templ, item.compiled_html, item.formatted_html])
    if (!files.has(name)) throw Error('missing pair artifact');
  const receipt = JSON.parse(files.get(item.receipt).toString('utf8'));
  if (receipt.kind !== 'elements-syntax' || receipt.status !== 'translated' || receipt.verification !== 'not-verified' ||
      receipt.source.sha256 !== sourcePins[item.id] || receipt.runtime.package !== '@tailwindplus/elements' ||
      receipt.runtime.version !== '1.0.22' || receipt.runtime.esm_sha256 !== '120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013' ||
      receipt.runtime.tgz_sha256 !== 'fc684f17d960fb60dadfdf9a0b49ba8fd295af4b8fd1715a26cd20b832ea245b' ||
      receipt.artifact.sha256 !== sha(files.get(item.emitted_templ))) throw Error('receipt identity mismatch');
}
if (sha(files.get('elements.js')) !== '120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013') throw Error('runtime pin mismatch');
const entry = process.env.APPKIT_PLAYWRIGHT_MODULE || '/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs';
const version = JSON.parse(fs.readFileSync(path.join(path.dirname(entry), 'package.json'))).version;
if (version !== '1.63.0') throw Error('unexpected Playwright pin');
const {chromium} = await import(pathToFileURL(entry).href);
const out = fs.mkdtempSync(path.join(root, 'browser-')); fs.chmodSync(out, 0o700);
const save = (name, body) => fs.writeFileSync(path.join(out, name), body, {flag: 'wx', mode: 0o600});
const report = {
  schema: 1, kind: 'elements-original-compiled-browser', verification: 'not-verified',
  pair_manifest_sha256: sha(manifestBytes), source_manifest_sha256: sourceManifestSHA256,
  harness_sha256: sha(fs.readFileSync(new URL(import.meta.url))),
  contract_sha256: sha(fs.readFileSync(new URL('./contract.mjs', import.meta.url))),
  versions: {playwright: version}, checks: [], errors: [], requests: [], source_findings: [], screenshots: [],
  runtime_bytes: files.get('elements.js').length, runtime_gzip_bytes: zlib.gzipSync(files.get('elements.js')).length,
  runtime_brotli_bytes: zlib.brotliCompressSync(files.get('elements.js')).length,
  timing_samples: [], focus_observations: [], geometry_observations: [], visual_pairs: [],
  limits: ['Paired Chrome runtime evidence, not accessibility, compatibility, or consumer certification.',
    'Remote P08/P09 images remain blocked and unpinned; their complete visual baseline is missing.',
    'Placeholder actions have no app backend. All form submission is refused.',
    'No touch, screen reader, true zoom, Datastar morph, heap-leak or cross-browser proof.',
    'Timings measure trusted input to second animation frame, not certified presentation latency.'],
};
const check = (name, ok, detail = null) => report.checks.push({name, ok: !!ok, detail});
const visualPairs = new Map();
let base, browser, omitRuntime = false;
const prefix = `/reference-${crypto.randomBytes(18).toString('hex')}/`;
const csp = "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'none'; form-action 'none'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'";
const server = http.createServer((req, res) => {
  res.setHeader('Content-Security-Policy', csp);
  res.setHeader('Cache-Control', 'no-store'); res.setHeader('X-Content-Type-Options', 'nosniff');
  res.setHeader('Referrer-Policy', 'same-origin'); res.setHeader('Cross-Origin-Resource-Policy', 'same-origin');
  if (!base || req.headers.host !== new URL(base).host || req.headers['sec-fetch-site'] === 'cross-site' ||
      (req.headers.origin && req.headers.origin !== base) || req.method !== 'GET') {res.writeHead(403); res.end(); return;}
  const url = new URL(req.url, base);
  const name = url.pathname.startsWith(prefix) ? url.pathname.slice(prefix.length) : '';
  if (url.search || !files.has(name)) {res.writeHead(404); res.end(); return;}
  const type = name.endsWith('.html') ? 'text/html; charset=utf-8' : name.endsWith('.js') ? 'text/javascript; charset=utf-8' : name.endsWith('.css') ? 'text/css; charset=utf-8' : name.endsWith('.woff2') ? 'font/woff2' : 'text/plain; charset=utf-8';
  res.writeHead(200, {'Content-Type': type});
  res.end(omitRuntime && name === 'elements.js' ? '// Runtime deliberately withheld by the negative control.\n' : files.get(name));
});
async function settle(page) {
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  // Source transitions remain intact, even if the source lacks reduced-motion CSS.
  // Wait for their bounded completion rather than misreporting a transient state.
  await page.waitForFunction(() => document.getAnimations().every(a => a.playState !== 'running' && !a.pending), null, {timeout: 2000});
}
const openMenu = menu => menu.evaluate(e => e.matches(':popover-open'));
const shown = locator => locator.evaluate(e => {
  if (e.hidden || getComputedStyle(e).display === 'none') return false;
  // P03 deliberately uses display:contents. Its host has no layout box; its
  // descendants must still render. Keep hidden state and visible content distinct.
  return [e, ...e.querySelectorAll('*')].some(n => n.getBoundingClientRect().height > 0 &&
    getComputedStyle(n).visibility !== 'hidden' && getComputedStyle(n).display !== 'none');
});
async function shot(page, name, pairKey, variant) {
  const b = await page.screenshot(), hash = sha(b);
  save(name, b); report.screenshots.push({name, sha256: hash, pair: pairKey, variant});
  const prior = visualPairs.get(pairKey);
  if (!prior) visualPairs.set(pairKey, {variant, sha256: hash, name});
  else {
    const pair = {pair: pairKey, source: variant === 'source' ? hash : prior.sha256,
      compiled: variant === 'compiled' ? hash : prior.sha256, equal: prior.sha256 === hash};
    report.visual_pairs.push(pair);
    check(`${pairKey}: source/compiled screenshot bytes match`, pair.equal, pair);
  }
}
async function observeState(page, key, state, keyboard, pairKey, variant) {
  const geometry = await page.locator('#specimen').evaluate(root => {
    const visible = e => e.checkVisibility() && e.getBoundingClientRect().height > 0 && getComputedStyle(e).visibility !== 'hidden';
    const rect = e => {const r = e.getBoundingClientRect(); return {tag: e.tagName, width: r.width, height: r.height, left: r.left, right: r.right, top: r.top, bottom: r.bottom};};
    return {width: innerWidth, height: innerHeight, scroll: document.documentElement.scrollWidth,
      controls: [...root.querySelectorAll('button,a')].filter(visible).map(rect),
      popovers: [...root.querySelectorAll('el-menu:popover-open,dialog[open]')].filter(visible).map(rect)};
  });
  report.geometry_observations.push({key, state, ...geometry});
  check(`${key}: ${state} no page overflow`, geometry.scroll <= geometry.width);
  check(`${key}: ${state} open overlays stay in viewport`, geometry.popovers.every(r => r.left >= -0.5 && r.right <= geometry.width + 0.5 && r.top >= -0.5 && r.bottom <= geometry.height + 0.5), geometry.popovers);
  const small = geometry.controls.filter(c => c.height < 44 || c.width < 44);
  if (small.length) report.source_findings.push({key, state, finding: 'source controls below 44px target goal', controls: small});
  if (keyboard) {
    const focus = await page.evaluate(() => {
      const e = document.activeElement, s = getComputedStyle(e), r = e.getBoundingClientRect();
      return {tag: e.tagName, focusVisible: e.matches(':focus-visible'), outlineStyle: s.outlineStyle,
        outlineWidth: s.outlineWidth, outlineColor: s.outlineColor, boxShadow: s.boxShadow,
        left: r.left, top: r.top, width: r.width, height: r.height};
    });
    report.focus_observations.push({key, state, ...focus});
    // A computed outline is only a screening observation. Shadows, contrast and
    // obscured controls still need review; activeElement alone proves no indicator.
    if (!focus.focusVisible || focus.outlineStyle === 'none' || parseFloat(focus.outlineWidth) === 0)
      report.source_findings.push({key, state, finding: 'visible focus not established by sampled outline; review keyboard screenshot', focus});
    await shot(page, `${key}-${state}-keyboard.png`, `${pairKey}-${state}-keyboard`, variant);
  }
}
async function tabTo(page, selector, max = 24) {
  for (let i = 0; i < max; i++) {
    await page.keyboard.press('Tab');
    if (await page.locator(selector).evaluate(e => e === document.activeElement)) return true;
  }
  return false;
}
async function menuTask(page, dropdown, key, pairKey, variant) {
  const trigger = dropdown.locator('button').first(), menu = dropdown.locator('el-menu');
  await trigger.focus(); await page.keyboard.press('Enter'); await settle(page);
  check(`${key}: keyboard opens menu`, await openMenu(menu));
  check(`${key}: menu receives focus`, await menu.evaluate(e => e.contains(document.activeElement)));
  const first = await menu.evaluate(e => [...e.querySelectorAll('[role="menuitem"]')].indexOf(document.activeElement));
  await page.keyboard.press('ArrowDown'); await settle(page);
  const next = await menu.evaluate(e => [...e.querySelectorAll('[role="menuitem"]')].indexOf(document.activeElement));
  check(`${key}: ArrowDown advances`, first >= 0 && next >= 0 && next !== first, {first, next});
  await observeState(page, key, 'menu-open', true, pairKey, variant);
  await page.keyboard.press('Escape'); await settle(page);
  check(`${key}: Escape closes and restores trigger`, !(await openMenu(menu)) && await trigger.evaluate(e => e === document.activeElement));
  await trigger.click(); await settle(page);
  check(`${key}: pointer opens menu`, await openMenu(menu));
  return {trigger, menu};
}
async function faqToggle(page, input = 'Enter') {
  const trigger = page.locator('button[commandfor="faq-1"]');
  const panel = page.locator('#faq-1'), before = await shown(panel);
  if (input === 'pointer') await trigger.click();
  else {await trigger.focus(); await page.keyboard.press(input);}
  await settle(page);
  return (await shown(panel)) !== before;
}
async function runCase(page, item, key, pairKey, variant, width) {
  const root = page.locator('#specimen');
  if (item.id === 'P03-faq') {
    check(`${key}: six panels`, await root.locator('el-disclosure').count() === 6);
    const initial = await root.locator('el-disclosure').evaluateAll(es => es.map(e => e.hidden));
    check(`${key}: original initial state`, JSON.stringify(initial) === JSON.stringify([false, true, true, true, true, true]), initial);
    check(`${key}: initial open panel has visible content`, await shown(page.locator('#faq-0')));
    check(`${key}: native Tab reaches first trigger`, await tabTo(page, 'button[commandfor="faq-0"]'));
    await observeState(page, key, 'faq-entry', true, pairKey, variant);
    for (const input of ['Enter', 'Space', 'pointer']) {
      check(`${key}: ${input} toggles panel`, await faqToggle(page, input));
      if (input !== 'pointer') check(`${key}: ${input} preserves trigger focus`, await page.locator('button[commandfor="faq-1"]').evaluate(e => e === document.activeElement));
      const other = await root.locator('el-disclosure').evaluateAll(es => es.filter(e => e.id !== 'faq-1').map(e => e.hidden));
      check(`${key}: ${input} preserves other panels`, JSON.stringify(other) === JSON.stringify([false, true, true, true, true]));
    }
    await shot(page, `${key}-open.png`, `${pairKey}-open`, variant);
  } else if (item.id === 'P07-menu') {
    const {menu} = await menuTask(page, root.locator('el-dropdown'), key, pairKey, variant);
    await shot(page, `${key}-open.png`, `${pairKey}-open`, variant);
    await page.mouse.click(width / 2, 950); await settle(page);
    check(`${key}: outside click dismisses`, !(await openMenu(menu)));
  } else if (item.id === 'P08-nested-dialog-menu') {
    const trigger = page.locator('button[command="show-modal"]'), dialog = page.locator('dialog#drawer');
    check(`${key}: Tab reaches dialog opener`, await tabTo(page, 'button[command="show-modal"]'));
    await page.keyboard.press('Enter'); await settle(page);
    check(`${key}: dialog opens and contains focus`, await dialog.evaluate(e => e.open && e.contains(document.activeElement)));
    const {menu} = await menuTask(page, dialog.locator('el-dropdown'), key, pairKey, variant);
    await shot(page, `${key}-nested.png`, `${pairKey}-nested`, variant);
    await page.keyboard.press('Escape'); await settle(page);
    check(`${key}: first Escape closes only menu`, !(await openMenu(menu)) && await dialog.evaluate(e => e.open));
    await page.keyboard.press('Escape'); await settle(page);
    check(`${key}: second Escape closes dialog and returns focus`, !(await dialog.evaluate(e => e.open)) && await trigger.evaluate(e => e === document.activeElement));
  } else {
    const trigger = page.locator('button[commandfor="mobile-menu"]'), panel = page.locator('#mobile-menu');
    check(`${key}: mobile trigger visibility`, await trigger.isVisible() === (width === 390));
    if (width === 390) {
      check(`${key}: Tab reaches mobile trigger`, await tabTo(page, 'button[commandfor="mobile-menu"]'));
      await page.keyboard.press('Enter'); await settle(page);
      check(`${key}: narrow menu opens`, await shown(panel));
      await observeState(page, key, 'mobile-open', true, pairKey, variant);
      await page.keyboard.press('Space'); await settle(page);
      check(`${key}: Space closes narrow menu`, !(await shown(panel)));
      await trigger.click(); await settle(page);
      check(`${key}: pointer opens narrow menu`, await shown(panel));
      await page.setViewportSize({width: 1440, height: 1000}); await settle(page);
      check(`${key}: desktop hides mobile region`, !(await panel.isVisible()) && !(await trigger.isVisible()));
      await page.setViewportSize({width, height: 1000}); await settle(page);
    }
    const {menu} = await menuTask(page, root.locator('el-dropdown'), key, pairKey, variant);
    await shot(page, `${key}-account.png`, `${pairKey}-account`, variant);
    await page.keyboard.press('Escape'); await settle(page);
    check(`${key}: account menu dismisses`, !(await openMenu(menu)));
  }
  // Reinsert the exact lane fragment rather than cloning runtime-mutated state.
  // The compiled lane must never fall back to the original source fragment.
  const original = files.get(item.fragments[variant]).toString('utf8');
  for (let round = 1; round <= 3; round++) {
    await root.evaluate((e, html) => {e.replaceChildren(); e.innerHTML = html;}, original);
    await settle(page);
    if (item.id === 'P03-faq') check(`${key}: remount ${round} single toggle`, await faqToggle(page, 'pointer'));
    else if (item.id === 'P08-nested-dialog-menu') {
      await page.locator('button[command="show-modal"]').click(); await settle(page);
      check(`${key}: remount ${round} opens dialog`, await page.locator('dialog#drawer').evaluate(e => e.open));
      await page.keyboard.press('Escape'); await settle(page);
      check(`${key}: remount ${round} closes dialog`, !(await page.locator('dialog#drawer').evaluate(e => e.open)));
    } else {
      const dropdown = root.locator('el-dropdown'), trigger = dropdown.locator('button').first(), menu = dropdown.locator('el-menu');
      await trigger.click(); await settle(page);
      check(`${key}: remount ${round} opens menu`, await openMenu(menu));
      await page.keyboard.press('Escape'); await settle(page);
      check(`${key}: remount ${round} restores focus`, !(await openMenu(menu)) && await trigger.evaluate(e => e === document.activeElement));
    }
  }
}
async function contextFor(width, theme) {
  const context = await browser.newContext({viewport: {width, height: 1000}, colorScheme: theme, reducedMotion: 'reduce', serviceWorkers: 'block'});
  await context.route('**/*', route => {
    const req = route.request(), url = new URL(req.url());
    const allowed = url.origin === base && url.pathname.startsWith(prefix) && files.has(url.pathname.slice(prefix.length)) && req.method() === 'GET' && !url.search;
    report.requests.push({url: req.url(), method: req.method(), allowed});
    return allowed ? route.continue() : route.abort('blockedbyclient');
  });
  await context.addInitScript(() => {
    window.__referenceTimings = [];
    window.__referenceViolations = [];
    document.addEventListener('securitypolicyviolation', event => {
      window.__referenceViolations.push({directive: event.effectiveDirective, blocked: event.blockedURI});
    });
    for (const kind of ['click', 'keydown']) document.addEventListener(kind, event => {
      if (!event.isTrusted) return;
      const start = performance.now();
      requestAnimationFrame(() => requestAnimationFrame(() => window.__referenceTimings.push({kind, ms: performance.now() - start})));
    }, true);
  });
  return context;
}
try {
  await new Promise((resolve, reject) => {server.once('error', reject); server.listen(0, '127.0.0.1', resolve);});
  base = `http://127.0.0.1:${server.address().port}`;
  browser = await chromium.launch({executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true});
  report.versions.chrome = await browser.version();
  check('runtime gzip meets declared 50KiB pilot screening budget', report.runtime_gzip_bytes <= 50 * 1024);
  for (const item of manifest.cases) for (const width of [390, 1440]) for (const theme of ['light', 'dark']) for (const variant of ['source', 'compiled']) {
    const pairKey = `${item.id}-${width}-${theme}`, key = `${item.id}-${variant}-${width}-${theme}`;
    const context = await contextFor(width, theme), page = await context.newPage();
    page.setDefaultTimeout(3000);
    const errors = [], consoleErrors = [], requestStart = report.requests.length;
    page.on('pageerror', error => errors.push(String(error)));
    page.on('console', message => {if (message.type() === 'error') consoleErrors.push(message.text());});
    try {
      await page.goto(base + prefix + item[`${variant}_documents`][theme]);
      await page.evaluate(() => document.fonts.ready);
      await page.waitForFunction(() => ['el-disclosure', 'el-dropdown', 'el-dialog'].every(t => customElements.get(t)));
      await settle(page);
      check(`${key}: pinned font loaded`, await page.evaluate(() => [...document.fonts].some(f => f.family.replaceAll('"', '') === 'InterVariable' && f.status === 'loaded')));
      await runCase(page, item, key, pairKey, variant, width);
      await observeState(page, key, 'after-remount', false, pairKey, variant);
      report.timing_samples.push({key, samples: await page.evaluate(() => window.__referenceTimings)});
      const remote = report.requests.slice(requestStart).filter(r => new URL(r.url).origin !== base);
      const expected = new Set(item.remote_assets);
      check(`${key}: every request refusal is a declared external image`, declaredImageRefusalsOnly(report.requests.slice(requestStart), base, expected));
      check(`${key}: off-origin requests stay blocked and declared`, remote.every(r => !r.allowed && expected.has(r.url)), remote);
      const violations = await page.evaluate(() => window.__referenceViolations);
      check(`${key}: CSP blocks only declared missing images`, violations.every(v => v.directive === 'img-src' && expected.has(v.blocked)), violations);
      if (expected.size) check(`${key}: missing imagery refusal actually observed`, violations.some(v => v.directive === 'img-src'));
      // Responsive images can be hidden by CSS yet still requested; keep the
      // observed subset, not a false claim that every source image was loaded.
      if (expected.size) report.source_findings.push({key, finding: 'complete visual baseline unavailable: remote imagery not admitted', declared: [...expected], observedBlocked: remote.map(r => r.url), cspBlocked: violations.map(v => v.blocked)});
    } catch (error) {check(`${key}: task completes`, false, String(error));}
    finally {
      check(`${key}: no runtime exceptions`, errors.length === 0, errors);
      const unexpected = consoleErrors.filter(message => !/Refused to load the image|violates.*img-src|Failed to load resource: net::ERR_BLOCKED_BY_CLIENT/i.test(message));
      check(`${key}: no unexpected error-console diagnostics`, unexpected.length === 0, unexpected);
      if (consoleErrors.length) report.source_findings.push({key, finding: 'browser console diagnostics', messages: consoleErrors});
      await context.close();
    }
    console.log(JSON.stringify({case: key, failures: report.checks.filter(c => !c.ok).length}));
  }
  for (const variant of ['source', 'compiled']) {
    omitRuntime = true;
    const context = await contextFor(390, 'light'), page = await context.newPage();
    try {
      const item = manifest.cases.find(c => c.id === 'P03-faq');
      await page.goto(base + prefix + item[`${variant}_documents`].light); await page.evaluate(() => document.fonts.ready);
      const changed = await faqToggle(page);
      check(`${variant}: runtime-withheld canary rejects same disclosure-toggle assertion`, !changed);
    } finally {await context.close(); omitRuntime = false;}
  }
  check('every named screenshot state has one byte-identical source/compiled pair',
    report.visual_pairs.length === report.screenshots.length / 2 && report.visual_pairs.every(pair => pair.equal),
    {pairs: report.visual_pairs.length, screenshots: report.screenshots.length});
} catch (error) {report.errors.push(String(error));}
finally {
  await browser?.close();
  server.closeAllConnections(); await new Promise(resolve => server.close(resolve));
  report.status = report.errors.length === 0 && report.checks.every(c => c.ok) && report.checks.length > 200 ? 'passed' : 'failed';
  save('report.json', Buffer.from(JSON.stringify(report, null, 2) + '\n'));
  console.log(JSON.stringify({status: report.status, output: out, checks: report.checks.length, failed: report.checks.filter(c => !c.ok).map(c => c.name), errors: report.errors}));
  if (report.status !== 'passed') process.exitCode = 1;
}
