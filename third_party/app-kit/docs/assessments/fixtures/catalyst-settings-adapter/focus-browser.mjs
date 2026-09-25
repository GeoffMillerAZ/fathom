// Prospective native-focus and bounded text/AX checks for the complete P10 form.
// No screen-reader, full contrast, physical-input or target certification claim.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {createRequire} from 'node:module';
import {contrast, visibleOutline, outlinePixels} from './focus-checks.mjs';

const [base, parent] = process.argv.slice(2);
const requireFact = (ok, message) => { if (!ok) throw Error(message); };
const origin = new URL(base);
requireFact(origin.origin === base && origin.protocol === 'http:' && origin.hostname === '127.0.0.1' && origin.port, 'Explicit loopback origin required');
requireFact(path.isAbsolute(parent) && fs.lstatSync(parent).isDirectory() && !fs.lstatSync(parent).isSymbolicLink(), 'Private output parent required');
const modulePath = process.env.APPKIT_PLAYWRIGHT_MODULE;
requireFact(modulePath && path.isAbsolute(modulePath), 'Approved isolated Playwright required');
const pkg = JSON.parse(fs.readFileSync(path.join(path.dirname(modulePath), 'package.json')));
requireFact(pkg.name === 'playwright' && pkg.version === '1.63.0', 'Pinned Playwright required');
const utilityPath = path.resolve(path.dirname(modulePath), '../playwright-core/lib/utilsBundle.js');
const {PNG} = createRequire(import.meta.url)(utilityPath); // already-installed, pinned test tool; no application dependency
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const out = fs.mkdtempSync(path.join(parent, 'p10-focus-browser-')); fs.chmodSync(out, 0o700);
const put = (name, body) => { const bytes = Buffer.from(body); fs.writeFileSync(path.join(out, name), bytes, {flag: 'wx', mode: 0o600}); return {name, sha256: hash(bytes)}; };
const expectedOrder = ['name', 'bio', 'email', 'email_is_public', 'address', 'city', 'region', 'postal_code', 'country[code]', 'currency', 'reset', 'validate'];
const expectedAX = {'Organization Name': 'textbox', 'Organization Bio': 'textbox', 'Organization Email': 'textbox',
  'Include email in validated sample data': 'checkbox', 'Street Address': 'textbox', 'City': 'textbox',
  'Region': 'combobox', 'Postal code': 'textbox', 'Country': 'combobox', 'Currency': 'combobox', 'Reset': 'button', 'Validate changes': 'button'};
const plan = {schema_version: 1, fixture: 'complete-p10-painted-native-focus',
  runner_sha256: hash(fs.readFileSync(fileURLToPath(import.meta.url))), checker_sha256: hash(fs.readFileSync(new URL('./focus-checks.mjs', import.meta.url))),
  chrome: '152.0.7977.84', playwright: '1.63.0', node: process.version, png_tool_sha256: hash(fs.readFileSync(utilityPath)),
  lanes: [390, 1440].flatMap(width => ['light', 'dark'].flatMap(theme => [false, true].map(enhanced => ({width, theme, enhanced})))),
  expectedOrder, expectedAX,
  requirements: ['all twelve controls follow native Tab order', 'visible 2px solid/double outline with >=2px offset and >=3:1 against the page for each keyboard-focused control',
    'twelve screenshot outline segments match expected ink and each clears3:1 against locally adjacent pixels; complete outer samples stay captured',
    'form-owned Chromium AX nodes expose all twelve named roles and invalid/error description after HTTP422',
    'visible heading/description/label/count/error/status text >=4.5:1 using flat Chromium background and sRGB samples; unsupported layers fail measurement',
    'minimum44px controls/checkbox label and no page overflow; native and enhanced retain correction task'],
  observation: 'fonts.ready then fixed50ms Node barrier, not a focus polling loop or latency assertion',
  text_scope: 'computed h1/h2/h3/p/label text color contrast only; no glyph visibility/occlusion, field-value, placeholder, native-popup, image/pseudo-element or non-text boundary certification',
  color_precision: 'browser canvas rgba-float16 sRGB; reject samples within0.01 of a threshold as inconclusive',
  exclusions: ['real200% zoom', 'physical keyboard/touch', 'screen-reader speech', 'complete WCAG or source-fidelity certification', 'full non-text/control/background contrast', 'performance', 'consumer adoption'],
  verification: 'not-verified'};
const planFile = put('plan.json', JSON.stringify(plan));
console.log(JSON.stringify({out, plan_sha256: planFile.sha256, status: 'focus tasks starting'}));
const report = {plan_sha256: planFile.sha256, checks: [], errors: [], forbidden: [], frames: [], text: []};
const check = (name, ok, actual) => report.checks.push({name, ok: !!ok, actual});
let browser;
const deadline = setTimeout(() => { report.errors.push('120s deadline'); void browser?.close(); }, 120_000);
const settle = async page => { await page.evaluate(async () => { await document.fonts.ready; }); await new Promise(resolve => setTimeout(resolve, 50)); };
async function srgb(page, colors) {
  return page.evaluate(colors => {
    const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
    const ctx = canvas.getContext('2d', {colorSpace: 'srgb'});
    return colors.map(color => {
      if (!CSS.supports('color', color)) throw Error('Unresolved CSS color');
      ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = color; ctx.fillRect(0, 0, 1, 1);
      return [...ctx.getImageData(0, 0, 1, 1, {colorSpace: 'srgb', pixelFormat: 'rgba-float16'}).data];
    });
  }, colors);
}
async function textChecks(page, cdp, label) {
  const selector = 'form[data-p10-settings] :is(h1,h2,h3,p,label)';
  const {root} = await cdp.send('DOM.getDocument');
  const {nodeIds} = await cdp.send('DOM.querySelectorAll', {nodeId: root.nodeId, selector});
  const elements = page.locator(selector);
  for (let i = 0; i < nodeIds.length; i++) {
    const fact = await elements.nth(i).evaluate(element => {
      const rect = element.getBoundingClientRect(), style = getComputedStyle(element);
      const unsupported = [];
      for (let a = element; a; a = a.parentElement) {
        const s = getComputedStyle(a);
        if (s.opacity !== '1' || s.backgroundImage !== 'none' || s.filter !== 'none' || s.mixBlendMode !== 'normal') unsupported.push(a.tagName);
      }
      return {text: element.textContent.trim(), visible: rect.width > 0 && rect.height > 0 && style.visibility === 'visible', color: style.color, unsupported};
    });
    if (!fact.visible || !fact.text) continue;
    const bg = await cdp.send('CSS.getBackgroundColors', {nodeId: nodeIds[i]});
    let ratio = null;
    if (fact.unsupported.length === 0 && bg.backgroundColors?.length === 1) {
      const [fg, background] = await srgb(page, [fact.color, bg.backgroundColors[0]]);
      ratio = contrast(fg, background);
    }
    const result = {...fact, background: bg, ratio}; report.text.push({lane: label, ...result});
    check(label + ': text color contrast ' + fact.text.slice(0, 40), ratio !== null && ratio >= 4.5 && Math.abs(ratio - 4.5) > 0.01, result);
  }
}
async function axChecks(cdp, label, invalid) {
  const {root} = await cdp.send('DOM.getDocument');
  const {nodeIds} = await cdp.send('DOM.querySelectorAll', {nodeId: root.nodeId,
    selector: 'form[data-p10-settings] :is(input:not([type=hidden]),textarea,select,button)'});
  const owned = new Set((await Promise.all(nodeIds.map(nodeId => cdp.send('DOM.describeNode', {nodeId})))).map(r => r.node.backendNodeId));
  const tree = await cdp.send('Accessibility.getFullAXTree');
  const nodes = tree.nodes.filter(n => owned.has(n.backendDOMNodeId) && !n.ignored && ['textbox', 'combobox', 'checkbox', 'button'].includes(n.role?.value));
  check(label + ': twelve accessible names and roles', nodes.length === 12 && Object.entries(expectedAX).every(([name, role]) =>
    nodes.filter(n => n.name?.value === name && n.role?.value === role).length === 1), nodes.map(n => ({name: n.name?.value, role: n.role?.value})));
  if (invalid) for (const [name, error] of [['Organization Bio', '240 Unicode code points'], ['Organization Email', 'ASCII addr-spec']]) {
    const node = nodes.find(n => n.name?.value === name);
    check(label + ': AX error ' + name, node?.properties?.some(p => p.name === 'invalid' && p.value.value === 'true') && node.description?.value.includes(error), {description: node?.description, properties: node?.properties});
  }
}
try {
  const {chromium} = await import(pathToFileURL(modulePath).href);
  browser = await chromium.launch({executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true});
  requireFact(browser.version() === plan.chrome, 'Pinned Chrome required');
  for (const lane of plan.lanes) {
    const label = `${lane.width}-${lane.theme}-${lane.enhanced ? 'enhanced' : 'native'}`;
    const context = await browser.newContext({viewport: {width: lane.width, height: 1200}, colorScheme: lane.theme,
      reducedMotion: 'reduce', serviceWorkers: 'block', javaScriptEnabled: lane.enhanced});
    try {
      await context.route('**/*', route => { const r = route.request(); if (new URL(r.url()).origin === base && ['GET', 'POST', 'HEAD'].includes(r.method())) return route.continue(); report.forbidden.push(r.url()); return route.abort(); });
      await context.routeWebSocket('**/*', socket => { report.forbidden.push(socket.url()); socket.close(); });
      const page = await context.newPage(); page.setDefaultTimeout(10_000); page.setDefaultNavigationTimeout(10_000);
      page.on('pageerror', error => report.errors.push(error.message));
      const response = await page.goto(`${base}/${lane.theme}/${lane.enhanced ? 'enhanced/' : ''}settings`);
      requireFact(response.status() === 200, 'Initial fixture response'); await settle(page);
      const cdp = await context.newCDPSession(page); await cdp.send('DOM.enable'); await cdp.send('CSS.enable');
      const {root} = await cdp.send('DOM.getDocument');
      const {nodeId} = await cdp.send('DOM.querySelector', {nodeId: root.nodeId, selector: 'form[data-p10-settings] h1'});
      const bg = await cdp.send('CSS.getBackgroundColors', {nodeId});
      requireFact(bg.backgroundColors?.length === 1, 'Flat page background required');
      const [background] = await srgb(page, bg.backgroundColors);
      await axChecks(cdp, label, false); await textChecks(page, cdp, label + '-default');
      for (const expected of expectedOrder) {
        await page.keyboard.press('Tab'); await settle(page);
        const fact = await page.evaluate(() => {
          const e = document.activeElement, s = getComputedStyle(e), r = e.getBoundingClientRect();
          return {name: e.name === '_action' ? e.value : e.name, matches: e.matches(':focus-visible'),
            visible: r.width > 0 && r.height > 0 && r.top >= 0 && r.bottom <= innerHeight && r.left >= 0 && r.right <= innerWidth,
            style: s.outlineStyle, width: parseFloat(s.outlineWidth), offset: parseFloat(s.outlineOffset), colorCSS: s.outlineColor,
            height: r.height, rect: {left:r.left,top:r.top,right:r.right,bottom:r.bottom},
            viewport: {width:innerWidth,height:innerHeight,dpr:devicePixelRatio}, overflow: document.documentElement.scrollWidth > innerWidth};
        });
        [fact.color] = await srgb(page, [fact.colorCSS]); fact.background = background;
        check(label + ': Tab order ' + expected, fact.name === expected, fact.name);
        check(label + ': computed focus ' + expected, visibleOutline(fact) && Math.abs(contrast(fact.color, background) - 3) > 0.01, fact);
        check(label + ': focused geometry ' + expected, !fact.overflow && (expected === 'email_is_public' || fact.height >= 44), fact);
        const screenshot = await page.screenshot(), pixels = PNG.sync.read(screenshot);
        let sampled;
        try {
          requireFact(fact.viewport.dpr === 1 && pixels.width === fact.viewport.width && pixels.height === fact.viewport.height, 'Exact100% CSS/pixel viewport required');
          sampled = outlinePixels(pixels, fact.rect, fact.width, fact.offset, fact.color);
        } catch (error) { sampled = {ok: false, error: error.message}; }
        check(label + ': sampled painted focus ' + expected, sampled.ok, sampled);
        report.frames.push({lane: label, state: expected, ...put(`focus-${expected.replace(/[^a-z]/g, '-')}-${label}.png`, screenshot)});
      }
      await page.locator('[name=bio]').fill('😀'.repeat(241)); await page.locator('[name=email]').fill('invalid');
      const [invalid] = await Promise.all([page.waitForNavigation({waitUntil: 'load'}), page.getByRole('button', {name: 'Validate changes', exact: true}).click()]);
      requireFact(invalid.status() === 422, 'Real invalid POST422 required'); await settle(page);
      await axChecks(cdp, label + '-invalid', true); await textChecks(page, cdp, label + '-invalid');
    } finally { await context.close(); }
  }
} catch (error) { report.errors.push(error.stack || error.message); }
finally {
  clearTimeout(deadline); if (browser) await browser.close().catch(error => report.errors.push(error.message));
  const failed = report.checks.filter(c => !c.ok);
  report.status = failed.length || report.errors.length || report.forbidden.length ? 'failed-not-verified' : 'scoped-focus-text-ax-passed';
  const receipt = put('report.json', JSON.stringify(report));
  console.log(JSON.stringify({out, status: report.status, checks: report.checks.length, failed: failed.map(c => c.name), errors: report.errors, report_sha256: receipt.sha256}));
  if (report.status !== 'scoped-focus-text-ax-passed') process.exitCode = 1;
}
