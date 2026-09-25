// Opt-in actual consumer handler probe. Invoked by the isolated Go test overlay.
// node browser.mjs <loopback-origin> <scenario> <empty-report-dir> <playwright/index.mjs>
import {readFile, writeFile, readdir, realpath} from 'node:fs/promises';
import {join, relative, isAbsolute} from 'node:path';
import {tmpdir} from 'node:os';
import {createHash} from 'node:crypto';
import {configuration} from '../../../../apps/gallery/browser/config.mjs';

const [origin, scenario, output, modulePath] = process.argv.slice(2);
if (!['registry-unavailable', 'pathless-service'].includes(scenario)) throw Error('Unknown synthetic handler scenario');
const config = configuration({APPKIT_SHELL_BROWSER_ORIGIN: origin, APPKIT_PLAYWRIGHT_MODULE: modulePath});
if (!output || !isAbsolute(output)) throw Error('An explicit empty temporary report directory is required');
const root = await realpath(output), temporary = await realpath(tmpdir());
const child = relative(temporary, root);
if (!child || child.startsWith('..') || isAbsolute(child) || (await readdir(root)).length) throw Error('Report must be a fresh child of the system temporary directory');
const {chromium} = await import(config.moduleURL);
const report = {scenario, origin, browser: null, checks: [], failures: [], blocked: [], pageErrors: [], consoleErrors: [], requests: [], screenshots: [], detailScreenshots: [], cases: [], quality: [],
  limits: ['Real mounted landing/Observatory/service/CSS handlers with a synthetic registry and hermetic local inputs.',
    'Not full serve-http/MCP middleware, operational fleet data, mutation, complete accessibility or performance proof.',
    'Automated Tab/Enter on installed Chrome; no physical keyboard, assistive technology or native-host claim.']};
const check = (name, pass, detail) => {
  report.checks.push({name, pass: !!pass, ...(detail === undefined ? {} : {detail})});
  if (!pass) { report.failures.push(name); throw Error(name); }
};
// Quality observations are deliberately non-short-circuiting: every lane still
// completes the existing task journey and capture before an observed defect
// makes the process fail.
const observe = (name, pass, detail) => {
  report.checks.push({name, pass: !!pass, ...(detail === undefined ? {} : {detail})});
  if (!pass) report.failures.push(name);
};
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
report.source = {file: 'browser.mjs', sha256: sha(await readFile(new URL(import.meta.url)))};
const routes = new Set(['/', '/observatory', '/assets/app.css', '/observatory/service?id=alpha']);
const browser = await chromium.launch({executablePath: config.executablePath, headless: true,
  args: ['--no-proxy-server', '--disable-background-networking']});
report.browser = browser.version();
const backgrounds = new Map();
let context, activePage;

async function tabTo(page, target, tag) {
  check(`${tag}: unique target`, await target.count() === 1);
  for (let step = 0; step < 48; step++) {
    if (await target.evaluate(el => el === document.activeElement)) return;
    await page.keyboard.press('Tab');
  }
  check(`${tag}: reachable by Tab`, false);
}

async function focusVisible(target, tag) {
  const state = await target.evaluate(el => {
    const b = el.getBoundingClientRect(), s = getComputedStyle(el), x = b.x + b.width / 2, y = b.y + b.height / 2;
    const hit = document.elementFromPoint(x, y);
    const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
    const painter = canvas.getContext('2d'); painter.fillStyle = s.outlineColor; painter.fillRect(0, 0, 1, 1);
    return {active: document.activeElement === el, width: b.width, height: b.height,
      visible: b.width > 0 && b.height > 0 && x >= 0 && y >= 0 && x < innerWidth && y < innerHeight && !!hit && (hit === el || el.contains(hit)),
      outline: parseFloat(s.outlineWidth), style: s.outlineStyle, color: s.outlineColor, alpha: painter.getImageData(0, 0, 1, 1).data[3]};
  });
  check(`${tag}: focused and hit-test visible`, state.active && state.visible, state);
  check(`${tag}: opaque visible outline style`, state.outline >= 2 && state.alpha === 255 && !['none', 'hidden'].includes(state.style), state);
  check(`${tag}: 44px target`, state.height >= 44 && state.width >= 44, state);
}

async function follow(page, target, path, tag) {
  await tabTo(page, target, tag);
  await focusVisible(target, tag);
  await Promise.all([page.waitForURL(origin + path), page.keyboard.press('Enter')]);
  check(`${tag}: exact destination`, page.url() === origin + path, page.url());
}

async function nav(page, width, tag) {
  if (width < 1024) {
    const menu = page.locator('summary').filter({hasText: /^Menu$/});
    await tabTo(page, menu, tag + ': menu');
    await focusVisible(menu, tag + ': menu');
    check(`${tag}: native menu starts closed`, !await menu.evaluate(el => el.parentElement.open));
    await page.keyboard.press('Enter');
    check(`${tag}: native menu opened`, await menu.evaluate(el => el.parentElement.open));
    return page.getByRole('navigation', {name: 'Primary (mobile)', exact: true});
  }
  return page.getByRole('navigation', {name: 'Primary', exact: true});
}

async function documentCheck(page, activePath, tag) {
  await page.evaluate(async () => { await document.fonts.ready; await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))); });
  const state = await page.evaluate(() => ({
    main: document.querySelectorAll('main#observatory-main[tabindex="-1"]').length,
    headings: document.querySelectorAll('h1').length, scripts: document.scripts.length,
    current: [...document.querySelectorAll('nav a[aria-current="page"]')].map(el => el.getAttribute('href')),
    overflow: document.documentElement.scrollWidth > innerWidth,
    background: getComputedStyle(document.body).backgroundColor,
  }));
  check(`${tag}: shared shell landmarks`, state.main === 1 && state.headings === 1, state);
  check(`${tag}: exact current destinations`, state.current.length === 2 && state.current.every(p => p === activePath), state.current);
  check(`${tag}: script-free native page`, state.scripts === 0, state.scripts);
  check(`${tag}: no page overflow`, !state.overflow, state);
  return state;
}

async function missingness(page, tag) {
  const text = await page.locator('body').innerText();
  if (scenario === 'registry-unavailable') {
    check(`${tag}: registry failure named`, text.includes('service registry unreachable') && text.includes('fleet tiles are not measured, never empty-as-healthy'));
    check(`${tag}: no invented service details`, await page.locator('a[href^="/observatory/service?"]').count() === 0);
  } else {
    check(`${tag}: unreadable log named`, text.includes('call log unreadable'));
  }
}

async function qualityObservations(page, tag) {
  const quality = await page.evaluate(scenario => {
    const rect = value => value ? {left: value.left, right: value.right, top: value.top, bottom: value.bottom, width: value.width, height: value.height} : null;
    const contained = (inner, outer, tolerance = 1) => !!inner && !!outer &&
      inner.left >= outer.left - tolerance && inner.right <= outer.right + tolerance &&
      inner.top >= outer.top - tolerance && inner.bottom <= outer.bottom + tolerance;
    const band = document.querySelector('dl[aria-label="fleet evidence status"]');
    const freshnessLabel = band && [...band.querySelectorAll('dt')].find(node => node.textContent.trim() === 'index freshness');
    const freshnessCell = freshnessLabel?.nextElementSibling;
    const freshnessParts = freshnessCell ? [...freshnessCell.querySelectorAll('span')].map(node => node.innerText.trim()).filter(Boolean) : [];
    const result = {
      scenario,
      selectors: {freshness: 'dl[aria-label="fleet evidence status"] dt[index freshness] + dd'},
      freshness: {
        bandCount: document.querySelectorAll('dl[aria-label="fleet evidence status"]').length,
        labelCount: freshnessLabel ? 1 : 0,
        text: freshnessCell?.innerText.trim() || '',
        headline: freshnessParts[0] || '',
        explanation: freshnessParts.slice(1).join(' '),
      },
    };
    if (scenario !== 'pathless-service') return result;

    const grade = document.querySelector('svg[role="img"][aria-label^="grade distribution on "]');
    const gradeRegion = grade?.closest('section');
    const gradeRect = grade?.getBoundingClientRect();
    const gradeRegionRect = gradeRegion?.getBoundingClientRect();
    result.selectors.grade = 'svg[role="img"][aria-label^="grade distribution on "] within closest section';
    result.grade = {
      count: document.querySelectorAll('svg[role="img"][aria-label^="grade distribution on "]').length,
      rect: rect(gradeRect), region: rect(gradeRegionRect),
      positiveSize: !!gradeRect && gradeRect.width > 0 && gradeRect.height > 0,
      contained: contained(gradeRect, gradeRegionRect),
      texts: grade ? [...grade.querySelectorAll('text')].map(node => {
        const matrix = node.getScreenCTM();
        return {text: node.textContent, contained: contained(node.getBoundingClientRect(), gradeRect),
          renderedFontSize: matrix ? parseFloat(getComputedStyle(node).fontSize) * Math.hypot(matrix.c, matrix.d) : 0};
      }) : [],
    };

    const serviceRegion = [...document.querySelectorAll('section')].find(section =>
      [...section.querySelectorAll('h2')].some(heading => heading.textContent.trim() === 'alpha'));
    const walker = serviceRegion && document.createTreeWalker(serviceRegion, NodeFilter.SHOW_TEXT);
    const errorNodes = [];
    while (walker && walker.nextNode()) {
      if (walker.currentNode.textContent.includes('call log unreadable')) errorNodes.push(walker.currentNode);
    }
    const errorNode = errorNodes.length === 1 ? errorNodes[0] : null;
    const errorRange = errorNode && document.createRange();
    if (errorRange) errorRange.selectNodeContents(errorNode);
    const errorRects = errorRange ? [...errorRange.getClientRects()].filter(r => r.width > 0 && r.height > 0) : [];
    const serviceRect = serviceRegion?.getBoundingClientRect();
    let paintVisible = !!errorNode;
    const clippingRegions = [];
    for (let node = errorNode?.parentElement; node && serviceRegion?.contains(node); node = node.parentElement) {
      const style = getComputedStyle(node);
      if (style.display === 'none' || style.visibility !== 'visible' || Number.parseFloat(style.opacity) === 0) paintVisible = false;
      if (node === serviceRegion || ['hidden', 'clip', 'auto', 'scroll'].includes(style.overflowX) ||
          ['hidden', 'clip', 'auto', 'scroll'].includes(style.overflowY)) clippingRegions.push(node.getBoundingClientRect());
      if (node === serviceRegion) break;
    }
    const unclipped = errorRects.length > 0 && clippingRegions.every(clip => errorRects.every(value => contained(value, clip)));
    result.selectors.unreadableLog = 'section headed alpha text node containing call log unreadable';
    result.unreadableLog = {
      regionCount: serviceRegion ? 1 : 0, textNodeCount: errorNodes.length, rectCount: errorRects.length,
      region: rect(serviceRect), rects: errorRects.map(rect),
      paintVisible, unclipped,
      contained: errorRects.length > 0 && errorRects.every(value => contained(value, serviceRect)),
      textAlign: errorNode ? getComputedStyle(errorNode.parentElement).textAlign : '',
    };
    return result;
  }, scenario);
  report.quality.push({tag, ...quality});
  const absence = /(?:unavailable|unknown|not measured)/i;
  const truthfulFreshness = quality.freshness.bandCount === 1 && quality.freshness.labelCount === 1 &&
    !/all fresh/i.test(quality.freshness.text) &&
    (absence.test(quality.freshness.headline) ||
      (quality.freshness.headline === '—' && !!quality.freshness.explanation && absence.test(quality.freshness.explanation)));
  observe(`${tag}: unavailable freshness is not reported all fresh`, truthfulFreshness, quality.freshness);
  if (scenario === 'pathless-service') {
    observe(`${tag}: named grade SVG stays inside its widget`, quality.grade.count === 1 && quality.grade.positiveSize && quality.grade.contained, quality.grade);
    observe(`${tag}: grade labels remain unscaled and inside the SVG`, quality.grade.texts.length > 0 &&
      quality.grade.texts.every(text => text.contained && text.renderedFontSize >= 11), quality.grade.texts);
    observe(`${tag}: unreadable-log text is visible inside the alpha card`, quality.unreadableLog.regionCount === 1 &&
      quality.unreadableLog.textNodeCount === 1 && quality.unreadableLog.paintVisible && quality.unreadableLog.unclipped &&
      quality.unreadableLog.contained, quality.unreadableLog);
    observe(`${tag}: unavailable-source prose retains a left reading edge`, quality.unreadableLog.textAlign === 'left', quality.unreadableLog.textAlign);
  }
}

try {
  for (const theme of ['light', 'dark']) for (const width of [390, 1440]) {
    const tag = `${scenario}/${theme}/${width}`;
    const entry = {theme, width, finished: false}; report.cases.push(entry);
    context = await browser.newContext({viewport: {width, height: 1000}, colorScheme: theme, reducedMotion: 'reduce',
      deviceScaleFactor: 1, serviceWorkers: 'block', acceptDownloads: false});
    await context.route('**/*', async route => {
      const req = route.request(), url = new URL(req.url());
      report.requests.push({method: req.method(), path: url.pathname + url.search});
      if (url.origin === origin && routes.has(url.pathname + url.search) && req.method() === 'GET') await route.continue();
      else { report.blocked.push({url: req.url(), method: req.method()}); await route.abort(); }
    });
    await context.routeWebSocket('**/*', ws => {report.blocked.push({url: ws.url(), method: 'WebSocket'}); ws.close();});
    const page = await context.newPage(); activePage = page; page.setDefaultTimeout(5000);
    page.on('pageerror', e => report.pageErrors.push(e.message));
    page.on('console', m => {if (m.type() === 'error') report.consoleErrors.push(m.text());});
    page.on('dialog', async d => {report.pageErrors.push('unexpected ' + d.type() + ' dialog'); await d.dismiss();});
    const response = await page.goto(origin + '/');
    check(`${tag}: landing HTTP200`, response.status() === 200);
    const homeState = await documentCheck(page, '/', tag + ': home');
    backgrounds.set(`${theme}/${width}`, homeState.background);
    await page.keyboard.press('Tab');
    check(`${tag}: first native Tab is skip`, await page.locator('a[href="#observatory-main"]').evaluate(el => el === document.activeElement));
    await page.keyboard.press('Enter');
    check(`${tag}: native skip transfers focus`, await page.locator('#observatory-main').evaluate(el => el === document.activeElement));
    // A fresh document removes skip-fragment restoration from the navigation trial.
    await page.goto(origin + '/');
    const homeNav = await nav(page, width, tag + ': home');
    await follow(page, homeNav.locator('a[href="/observatory"]'), '/observatory', tag + ': fleet');
    await documentCheck(page, '/observatory', tag + ': fleet');
    await missingness(page, tag + ': fleet');
    if (scenario === 'pathless-service') {
      await follow(page, page.locator('main a[href="/observatory/service?id=alpha"]'), '/observatory/service?id=alpha', tag + ': service');
      await documentCheck(page, '/observatory', tag + ': service');
      const text = await page.locator('main').innerText();
      check(`${tag}: configuration absence named`, text.includes('no recorded corpus path') && text.includes('configuration not inspectable'));
      const serviceNav = await nav(page, width, tag + ': service');
      await follow(page, serviceNav.locator('a[href="/observatory"]'), '/observatory', tag + ': return fleet');
      await missingness(page, tag + ': returned fleet');
    }
    await qualityObservations(page, tag + ': returned fleet quality');
    const screenshot = `${scenario}-${theme}-${width}.png`;
    await page.screenshot({path: join(root, screenshot), fullPage: true});
    report.screenshots.push({file: screenshot, sha256: sha(await readFile(join(root, screenshot)))});
    if (width === 390) {
      await page.setViewportSize({width: 320, height: 1000});
      await documentCheck(page, '/observatory', tag + ': resized 320');
      await qualityObservations(page, tag + ': resized 320 quality');
      const file = `${scenario}-${theme}-320.png`;
      await page.screenshot({path: join(root, file), fullPage: true});
      report.detailScreenshots.push({file, sha256: sha(await readFile(join(root, file)))});
      await page.setViewportSize({width, height: 1000});
      await documentCheck(page, '/observatory', tag + ': restored 390');
    }
    const fleetNav = await nav(page, width, tag + ': return');
    await follow(page, fleetNav.locator('a[href="/"]'), '/', tag + ': return home');
    await documentCheck(page, '/', tag + ': returned home');
    entry.finished = true;
    await context.close(); context = null; activePage = null;
  }
  for (const width of [390, 1440]) check(`actual CSS theme changes at ${width}`, backgrounds.get(`light/${width}`) !== backgrounds.get(`dark/${width}`), Object.fromEntries(backgrounds));
  check('all four scenario journeys completed', report.cases.length === 4 && report.cases.every(c => c.finished));
  check('no unexpected traffic or browser errors', !report.blocked.length && !report.pageErrors.length && !report.consoleErrors.length,
    {blocked: report.blocked, pageErrors: report.pageErrors, consoleErrors: report.consoleErrors});
  if (report.failures.length) process.exitCode = 1;
} catch (error) {
  report.failures.push(String(error)); process.exitCode = 1;
  if (activePage) {
    try {
      report.failureText = (await activePage.locator('body').innerText()).slice(0, 20000);
      const file = `failure-${scenario}.png`;
      await activePage.screenshot({path: join(root, file), fullPage: true});
      report.failureScreenshot = {file, sha256: sha(await readFile(join(root, file)))};
    } catch (diagnosticError) {report.failureCaptureError = String(diagnosticError);}
  }
} finally {
  if (context) await context.close();
  await browser.close();
  await writeFile(join(root, 'browser-report.json'), JSON.stringify(report, null, 2) + '\n', {flag: 'wx', mode: 0o600});
  console.log(JSON.stringify({scenario, checks: report.checks.length, failures: report.failures, completed: report.cases.filter(c => c.finished).length}));
}
