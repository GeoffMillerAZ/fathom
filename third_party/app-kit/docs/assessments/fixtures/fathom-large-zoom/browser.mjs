// Synthetic large-count and actual Chrome Appearance zoom probe. The owner
// controls the browser/server lifecycle and freezes this module before use.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import path from 'node:path';
import { inspectZoom } from './zoom-contract.mjs';

const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const wait = (label, work, ms = 15000) => {
  let timer;
  return Promise.race([work(), new Promise((_, reject) => {
    timer = setTimeout(() => reject(Error(`${label} exceeded ${ms}ms`)), ms);
  })]).finally(() => clearTimeout(timer));
};

export async function runProbe({ chromium, browser, context, origin, output, report }) {
  assert.ok(chromium && browser && context);
  const base = new URL(origin);
  assert.equal(base.protocol, 'http:');
  assert.equal(base.hostname, '127.0.0.1');
  assert.equal(base.origin, origin);
  assert.ok(path.isAbsolute(output));

  const failures = [];
  const pendingBodies = new Set();
  const requests = [];
  const responses = [];
  const requestFailures = [];
  const pageErrors = [];
  const consoleErrors = [];
  const blocked = [];
  const screenshots = [];
  const cells = [];
  let cellTag = 'setup';
  let session;
  const page = await context.newPage();
  await page.setViewportSize({ width: 1440, height: 1000 });
  page.setDefaultTimeout(10000);
  page.setDefaultNavigationTimeout(15000);

  report.large_zoom = {
    scope: 'synthetic production renderer at native Chrome Appearance 100% and 200%',
    expected: { gradeA: 1000, find_symbol: 1234567, references: 1000 },
    guide: { planned: 'light/200 only', dark: 'not run; bounded matrix intentionally omits duplicate guide journey' },
    requests, responses, request_failures: requestFailures, page_errors: pageErrors,
    console_errors: consoleErrors, blocked, screenshots, cells, failures,
    limits: ['Chart SVG geometry and exact rendered labels, not chart/table agreement.',
      'Native automated keyboard input, not physical keyboard or assistive-technology certification.',
      'Fresh private Chrome profile owned by the external runner; no installed profile or live data.'],
  };
  report.cells = cells;

  const check = (name, condition, detail) => {
    const pass = Boolean(condition);
    const item = { name, pass, ...(detail === undefined ? {} : { detail }) };
    report.checks.push(item);
    if (!pass) failures.push(name);
    return pass;
  };
  const allowed = request => {
    const url = new URL(request.url());
    if (url.origin !== origin || request.isNavigationRequest() && request.resourceType() !== 'document') return false;
    const method = request.method();
    if (method === 'GET' && url.pathname === '/large')
      return [...url.searchParams.keys()].length === 1 && url.searchParams.has('scheme') &&
        ['light', 'dark'].includes(url.searchParams.get('scheme'));
    if (method === 'GET' && url.pathname === '/app.css') return url.search === '';
    if (url.pathname === '/dev/theme') return (method === 'GET' || method === 'POST') && url.search === '';
    return method === 'GET' && ['/dev/theme/preview', '/dev/theme/style.css'].includes(url.pathname);
  };
  await context.route('**/*', async route => {
    const request = route.request();
    if (allowed(request)) return route.continue();
    blocked.push({ cell: cellTag, method: request.method(), url: request.url() });
    return route.abort('blockedbyclient');
  });
  await context.routeWebSocket('**/*', socket => {
    blocked.push({ cell: cellTag, method: 'WebSocket', url: socket.url() }); socket.close();
  });
  page.on('request', request => {
    if (request.url().startsWith(origin)) requests.push({ cell: cellTag, method: request.method(), url: request.url() });
  });
  page.on('requestfailed', request => requestFailures.push({ cell: cellTag, method: request.method(),
    url: request.url(), error: request.failure()?.errorText || '' }));
  page.on('pageerror', error => pageErrors.push({ cell: cellTag, error: String(error) }));
  page.on('console', message => {
    if (message.type() === 'error') consoleErrors.push({ cell: cellTag, text: message.text(), location: message.location() });
  });
  page.on('download', download => {
    failures.push('unexpected download');
    download.cancel().catch(error => failures.push(`download cancellation: ${error}`));
  });
  page.on('response', response => {
    if (!response.url().startsWith(origin)) return;
    const observedCell = cellTag;
    const task = wait('response body', async () => {
      const body = await response.body();
      responses.push({ cell: observedCell, method: response.request().method(), url: response.url(),
        status: response.status(), bytes: body.length, sha256: hash(body), body_complete: true });
    }).catch(error => {
      responses.push({ cell: observedCell, method: response.request().method(), url: response.url(),
        status: response.status(), body_complete: false, error: String(error) });
      failures.push(`${observedCell} response body incomplete`);
    }).finally(() => pendingBodies.delete(task));
    pendingBodies.add(task);
  });

  async function drainBodies() {
    while (pendingBodies.size) await Promise.allSettled([...pendingBodies]);
  }
  async function setZoom(value) {
    cellTag = `settings-${value}`;
    await wait('about:blank before enabling settings scripts', () => page.goto('about:blank', { waitUntil: 'load' }));
    if (!session) session = await context.newCDPSession(page);
    await session.send('Emulation.setScriptExecutionDisabled', { value: false });
    await wait('Chrome Appearance navigation', () => page.goto('chrome://settings/appearance', { waitUntil: 'load' }));
    const selector = page.locator('#zoomLevel');
    await wait('Chrome Appearance zoom selector', () => selector.selectOption(value));
    check(`Chrome Appearance selected ${value === '1' ? '100%' : '200%'}`,
      await selector.inputValue() === value, await selector.inputValue());
    await wait('about:blank before disabling application scripts', () => page.goto('about:blank', { waitUntil: 'load' }));
    await session.send('Emulation.setScriptExecutionDisabled', { value: true });
  }
  async function assetsComplete(label) {
    const state = await wait(`${label} assets`, () => page.evaluate(async () => {
      if (document.fonts) await document.fonts.ready;
      const links = [...document.querySelectorAll('link[rel~="stylesheet"]')];
      return { ready: document.readyState, fonts: document.fonts?.status || 'unsupported',
        links: links.map(link => ({ href: link.href, loaded: link.sheet !== null })),
        styleSheets: document.styleSheets.length, scripts: document.scripts.length };
    }));
    check(`${label} fonts and styles complete`, state.ready === 'complete' && state.fonts === 'loaded' &&
      state.styleSheets > 0 && state.links.every(link => link.loaded), state);
    check(`${label} has no application scripts`, state.scripts === 0, state.scripts);
    return state;
  }
  async function capture(locator, name) {
    const file = path.join(output, name + '.png');
    let targetBox;
    if (typeof locator.boundingBox === 'function') {
      await locator.scrollIntoViewIfNeeded();
      targetBox = await locator.boundingBox();
      const viewport = await page.evaluate(() => ({ width: innerWidth, height: innerHeight }));
      check(`${name} target is inside captured viewport`, targetBox && targetBox.x >= 0 && targetBox.y >= 0 &&
        targetBox.x + targetBox.width <= viewport.width && targetBox.y + targetBox.height <= viewport.height,
      { targetBox, viewport });
    }
    // Playwright's element clip was blank at native zoom. Capture the real viewport
    // with the target visibly in it; do not rescale or crop pixels after acquisition.
    await wait(`${name} screenshot`, () => page.screenshot({ path: file, fullPage: false, timeout: 12000 }));
    const bytes = await fs.readFile(file);
    const item = { name, file, bytes: bytes.length, sha256: hash(bytes), targetBox };
    screenshots.push(item);
    return item;
  }
  async function inspectChart(aria, width, expectedText, screenshotName) {
    const svg = page.locator(`svg[role="img"][aria-label="${aria}"]`);
    check(`${cellTag} unique ${aria} chart`, await svg.count() === 1, await svg.count());
    const geometry = await svg.evaluate((node, expectedWidth) => {
      const overlap = (a, b) => a.left < b.right - 0.1 && a.right > b.left + 0.1 &&
        a.top < b.bottom - 0.1 && a.bottom > b.top + 0.1;
      const view = node.viewBox.baseVal;
      const children = [...node.querySelectorAll('text,rect')].map((element, index) => {
        const box = element.getBBox(), screen = element.getBoundingClientRect();
        return { index, tag: element.tagName.toLowerCase(), text: element.textContent,
          box: { x: box.x, y: box.y, width: box.width, height: box.height },
          screen: { left: screen.left, right: screen.right, top: screen.top, bottom: screen.bottom },
          contained: box.x >= -0.1 && box.y >= -0.1 && box.x + box.width <= view.width + 0.1 &&
            box.y + box.height <= view.height + 0.1 };
      });
      const texts = children.filter(item => item.tag === 'text');
      const bars = children.filter(item => item.tag === 'rect');
      const textPairs = [];
      for (let i = 0; i < texts.length; i++) for (let j = i + 1; j < texts.length; j++)
        if (overlap(texts[i].screen, texts[j].screen)) textPairs.push([texts[i].text, texts[j].text]);
      const textBars = [];
      for (const text of texts) for (const bar of bars)
        if (overlap(text.screen, bar.screen)) textBars.push([text.text, bar.index]);
      return { width: node.getAttribute('width'), viewBox: [view.x, view.y, view.width, view.height],
        rect: node.getBoundingClientRect().toJSON(), expectedWidth, children, textPairs, textBars,
        texts: texts.map(item => item.text) };
    }, width);
    check(`${cellTag} ${aria} intrinsic width ${width}`, geometry.width === String(width) &&
      geometry.viewBox[2] === width, geometry);
    check(`${cellTag} ${aria} exact visible SVG text`,
      JSON.stringify(geometry.texts) === JSON.stringify(expectedText), geometry.texts);
    check(`${cellTag} ${aria} glyph geometry contained`,
      geometry.children.length > 0 && geometry.children.every(item => item.contained), geometry.children);
    check(`${cellTag} ${aria} text does not overlap text or bars`,
      geometry.textPairs.length === 0 && geometry.textBars.length === 0,
      { textPairs: geometry.textPairs, textBars: geometry.textBars });
    await capture(svg, screenshotName);
    return geometry;
  }
  async function nativeTabTo(locator, label, limit = 180) {
    check(`${label} unique target`, await locator.count() === 1, await locator.count());
    for (let step = 0; step < limit; step++) {
      if (await locator.evaluate(element => element === document.activeElement)) return;
      await page.keyboard.press('Tab');
    }
    failures.push(`${label} not reached by native Tab`);
    throw Error(`${label} not reached by native Tab`);
  }
  async function submitGuide(expectedStatus) {
    const button = page.getByRole('button', { name: 'Preview candidate', exact: true });
    await nativeTabTo(button, `${cellTag} Preview candidate`);
    const [response] = await Promise.all([
      page.waitForResponse(item => item.request().method() === 'POST' && new URL(item.url()).pathname === '/dev/theme'),
      page.waitForNavigation({ waitUntil: 'load' }), page.keyboard.press('Enter'),
    ]);
    check(`${cellTag} preview HTTP ${expectedStatus}`, response.status() === expectedStatus, response.status());
  }
  async function inspectGuideLayout(label) {
    const main = await page.evaluate(() => {
      const elements = [...document.querySelectorAll('.sg button,.sg input:not([type="hidden"]),.sg select,.sg-editor summary')]
        .filter(element => element.checkVisibility()).map(element => {
          const rect = element.getBoundingClientRect();
          return { name: element.getAttribute('name') || element.textContent.trim().slice(0, 80),
            x: rect.x, right: rect.right, width: rect.width, height: rect.height };
        });
      return { innerWidth, scrollWidth: document.documentElement.scrollWidth, elements };
    });
    check(`${label} editor has no horizontal overflow`, main.scrollWidth <= main.innerWidth + 1, main);
    check(`${label} visible primary controls fit and reach 44px`, main.elements.length > 0 && main.elements.every(item =>
      item.x >= -1 && item.right <= main.innerWidth + 1 && item.width >= 44 && item.height >= 44), main.elements);
    const handle = await page.locator('iframe.sg-preview').elementHandle();
    const frame = await handle?.contentFrame();
    check(`${label} opaque preview exists`, !!frame && await page.locator('iframe.sg-preview').getAttribute('sandbox') === '');
    if (!frame) return;
    const preview = await frame.evaluate(async () => {
      if (document.fonts) await document.fonts.ready;
      return { innerWidth, scrollWidth: document.documentElement.scrollWidth,
        scripts: document.scripts.length, ready: document.readyState,
        fonts: document.fonts?.status || 'unsupported', styleSheets: document.styleSheets.length,
        linkedStyles: [...document.querySelectorAll('link[rel~="stylesheet"]')].map(link =>
          ({ href: link.href, loaded: link.sheet !== null })) };
    });
    check(`${label} preview has no horizontal overflow or scripts`, preview.scrollWidth <= preview.innerWidth + 1 &&
      preview.scripts === 0, preview);
    check(`${label} preview fonts and styles complete`, preview.ready === 'complete' && preview.fonts === 'loaded' &&
      preview.styleSheets > 0 && preview.linkedStyles.every(link => link.loaded), preview);
  }

  try {
    for (const zoom of ['1', '2']) {
      await setZoom(zoom);
      for (const scheme of ['light', 'dark']) {
        cellTag = `${scheme}-${zoom === '1' ? '100' : '200'}`;
        const cell = { name: cellTag, scheme, zoom_percent: zoom === '1' ? 100 : 200 };
        cells.push(cell);
        await page.emulateMedia({ colorScheme: scheme, reducedMotion: 'reduce' });
        const response = await wait(`${cellTag} navigation`, () => page.goto(`${origin}/large?scheme=${scheme}`, { waitUntil: 'load' }));
        check(`${cellTag} large HTTP 200`, response.status() === 200, response.status());
        await assetsComplete(cellTag);
        const cdp = await session.send('Page.getLayoutMetrics');
        const exposed = await page.evaluate(() => ({ dpr: devicePixelRatio, inner: innerWidth,
          outer: outerWidth, scale: visualViewport.scale, scrollWidth: document.documentElement.scrollWidth,
          theme: document.documentElement.dataset.theme, background: getComputedStyle(document.body).backgroundColor,
          foreground: getComputedStyle(document.body).color }));
        check(`${cellTag} requested theme is applied`, exposed.theme === scheme, exposed);
        cell.zoom = { exposed, content_width: cdp.contentSize.width, css_width: cdp.cssContentSize.width };
        check(`${cellTag} actual Chrome zoom geometry`, inspectZoom(zoom, exposed, cdp), cell.zoom);
        check(`${cellTag} page has no horizontal overflow`, exposed.scrollWidth <= exposed.inner + 1, exposed);
        cell.grade = await inspectChart('grade distribution on 2026-09-21', 256,
          ['A', '1,000'], `${cellTag}-grade`);
        cell.calls = await inspectChart('calls per tool', 560,
          ['find_symbol', '1,234,567', 'references', '1,000'], `${cellTag}-calls`);
        await drainBodies();
        const cellResponses = responses.filter(item => item.cell === cellTag);
        const appCSS = cellResponses.filter(item => new URL(item.url).pathname === '/app.css');
        check(`${cellTag} response bodies complete`, cellResponses.length === 2 &&
          cellResponses.every(item => item.body_complete && item.status === 200), cellResponses);
        check(`${cellTag} uses exactly the frozen consumer app CSS`, appCSS.length === 1 &&
          appCSS[0].sha256 === report.contract.inputs.app_css.sha256,
        { observed: appCSS, expected_sha256: report.contract.inputs.app_css.sha256 });

        if (zoom === '2' && scheme === 'light') {
          cellTag = 'guide-light-200';
          const guide = { name: cellTag };
          report.large_zoom.guide.light_200 = guide;
          const guideResponse = await wait('guide navigation', () => page.goto(origin + '/dev/theme', { waitUntil: 'load' }));
          check('guide-light-200 HTTP 200', guideResponse.status() === 200, guideResponse.status());
          await assetsComplete(cellTag);
          await inspectGuideLayout('guide-light-200 source');
          const summary = page.locator('.sg-editor summary').filter({ hasText: 'Corner radii' });
          await nativeTabTo(summary, 'guide-light-200 Corner radii'); await page.keyboard.press('Enter');
          const radius = page.locator('input[name="radius.md"]');
          await nativeTabTo(radius, 'guide-light-200 radius.md');
          await page.keyboard.press('Meta+A'); await page.keyboard.type('-1px');
          await submitGuide(422);
          check('guide-light-200 validation error receives native focus',
            await page.locator(':focus').getAttribute('role') === 'alert', await page.locator(':focus').getAttribute('role'));
          const repair = page.getByRole('link', { name: 'Fix radius.md', exact: true });
          await nativeTabTo(repair, 'guide-light-200 repair link'); await page.keyboard.press('Enter');
          check('guide-light-200 repair link focuses retained invalid radius',
            await radius.evaluate(element => element === document.activeElement) && await radius.inputValue() === '-1px');
          await page.keyboard.press('Meta+A'); await page.keyboard.type('1rem');
          await submitGuide(200);
          check('guide-light-200 repaired value retained', await radius.inputValue() === '1rem', await radius.inputValue());
          await inspectGuideLayout('guide-light-200 candidate');
          await capture(page, 'guide-light-200-candidate');
          const reset = page.getByRole('link', { name: 'Reset to source', exact: true });
          await nativeTabTo(reset, 'guide-light-200 reset');
          await Promise.all([page.waitForNavigation({ waitUntil: 'load' }), page.keyboard.press('Enter')]);
          check('guide-light-200 reset clears candidate', await radius.inputValue() === '' &&
            await page.locator('input[name="review_digest"]').inputValue() === '');
          await inspectGuideLayout('guide-light-200 reset');
          await drainBodies();
          guide.completed = true;
        }
      }
    }
    check('no out-of-scope network requests', blocked.length === 0, blocked);
    check('no request failures', requestFailures.length === 0, requestFailures);
    check('no browser page errors', pageErrors.length === 0, pageErrors);
    check('console errors are only the expected native 422 diagnostic', consoleErrors.length <= 1 &&
      consoleErrors.every(item => item.cell === 'guide-light-200' && item.location.url === origin + '/dev/theme' &&
        /^Failed to load resource: the server responded with a status of 422\b/.test(item.text)), consoleErrors);
    const allAppCSS = responses.filter(item => new URL(item.url).pathname === '/app.css');
    check('every observed app CSS body matches the frozen consumer CSS', allAppCSS.length >= 4 &&
      allAppCSS.every(item => item.body_complete && item.sha256 === report.contract.inputs.app_css.sha256),
    { observed: allAppCSS, expected_sha256: report.contract.inputs.app_css.sha256 });
    check('response bodies all completed', responses.length > 0 && responses.every(item => item.body_complete), responses);
    if (failures.length) throw Error(`large/zoom probe failed ${failures.length} check(s): ${failures.join('; ')}`);
    report.large_zoom.passed = true;
  } finally {
    await drainBodies();
    if (session) await session.detach().catch(error => report.errors.push(`driver CDP detach: ${error}`));
    await wait('owned probe page close', () => page.close(), 5000).catch(error => report.errors.push(`driver page close: ${error}`));
  }
}
