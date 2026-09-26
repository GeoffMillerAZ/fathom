// Retained harness: the exact function executed through the Playwright MCP
// browser_run_code tool against a Gallery binary on 127.0.0.1:8775 (see the
// assessment). It is evidence input, not a maintained test; `task check` does
// not run it. SHOTS is replaced with a private output directory before running.
async (page0) => {
  const ctx = page0.context();
  const page = await ctx.newPage();
  const checks = [];
  const check = (name, ok, detail) => checks.push({ name, ok: !!ok, detail });
  const shots = 'SHOTS/';
  const narrow = {}; // per-width layout sequences, reused by the script-free check
  const docRects = (p) => p.evaluate(() => [...document.querySelectorAll('[data-live-source], [data-live-source] > *, #live-fixture-title, [data-live-page]')].map(e => { const r = e.getBoundingClientRect(); return [Math.round(r.x + scrollX), Math.round(r.y + scrollY), Math.round(r.width), Math.round(r.height)].join(','); }));
  const bodies = (p) => p.evaluate(() => [...document.querySelectorAll('[data-live-source]')].map(r => [...r.children].filter(c => !c.hasAttribute('data-live-status')).map(c => c.outerHTML).join('')));
  const state = () => page.evaluate(() => window.AppKitLive.state());
  const post = (path, body) => page.evaluate(([p, b]) => fetch(p, { method: 'POST', body: new URLSearchParams(b), headers: { 'X-Live-Fixture': 'fetch' } }).then(r => r.status), [path, body]);
  // Canvas conversion handles oklch computed colors; transparent layers are skipped.
  const contrast = (p, selectors) => p.evaluate((sels) => {
    const rgba = (c) => { const cv = document.createElement('canvas'); cv.width = cv.height = 1; const x = cv.getContext('2d'); x.clearRect(0, 0, 1, 1); x.fillStyle = c; x.fillRect(0, 0, 1, 1); return [...x.getImageData(0, 0, 1, 1).data]; };
    const lum = ([r, g, b]) => { const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }; return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b); };
    const bg = (el) => { for (; el; el = el.parentElement) { const c = rgba(getComputedStyle(el).backgroundColor); if (c[3] === 255) return c; } return rgba(getComputedStyle(document.body).backgroundColor); };
    const out = {};
    for (const [name, sel] of Object.entries(sels)) { const el = document.querySelector(sel); const a = lum(rgba(getComputedStyle(el).color)), b = lum(bg(el)); out[name] = +(((Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)).toFixed(2)); }
    return out;
  }, selectors);
  const sels = { asof: '#live-2 [data-live-asof]', updated: '#live-1 [data-live-updated]', link: '#live-1 [data-live-refresh]', connection: '[data-live-connection]', toggle: '[data-live-toggle]', eyebrow: '[data-page-heading] p', meta: '[data-page-heading] li' };
  const lastIds = [];
  const scriptErrors = [];
  const resourceErrors = [];
  page.on('pageerror', (e) => scriptErrors.push(String(e)));
  page.on('console', (m) => { if (m.type() === 'error') (m.text().startsWith('Failed to load resource') ? resourceErrors : scriptErrors).push(m.text()); });
  page.on('request', (req) => { if (req.url().endsWith('/live/events')) lastIds.push(req.headers()['last-event-id'] ?? '(none)'); });
  try {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto('http://127.0.0.1:8775/live?theme=light');
  await page.waitForFunction(() => window.AppKitLive && window.AppKitLive.state().status === 'live');
  const cursor = await page.getAttribute('[data-live-page]', 'data-live-last-event-id');
  check('A1 auto-starts, reports live and sends the render-time cursor', (await page.locator('[data-live-connection]').innerText()) === 'Live updates on.' && lastIds[0] === cursor, { cursor, lastIds: lastIds.slice() });
  check('A2 updated layers start invisible and unfocusable', await page.evaluate(() => [...document.querySelectorAll('[data-live-updated]')].every(e => getComputedStyle(e).visibility === 'hidden') && (() => { const a = document.querySelector('[data-live-refresh]'); a.focus(); return document.activeElement !== a; })()));
  const r0 = await docRects(page); const b0 = await bodies(page);
  await page.locator('[data-live-toggle]').focus();
  await page.keyboard.press('Tab');
  check('A3 keyboard reaches the first fixture control after the stop control', (await page.evaluate(() => document.activeElement.textContent.trim())) === 'Publish Plans change');
  await page.keyboard.press('Enter');
  await page.waitForFunction(() => document.querySelector('#live-1').getAttribute('data-live-state') === 'updated', null, { timeout: 5000 });
  const r1 = await docRects(page); const b1 = await bodies(page);
  check('A4 marking moves no region, region child, page status or later section (document coordinates)', JSON.stringify(r0) === JSON.stringify(r1), { compared: r0.length, moved: r0.map((v, i) => v === r1[i] ? null : [i, v, r1[i]]).filter(Boolean) });
  check('A5 region content unchanged', JSON.stringify(b0) === JSON.stringify(b1));
  check('A6 only the matching region shows the affordance; its as-of text stays in place', await page.evaluate(() => { const r = document.querySelector('#live-1'); const a = r.querySelector('[data-live-asof]').getBoundingClientRect(), u = r.querySelector('[data-live-updated]').getBoundingClientRect(); return getComputedStyle(r.querySelector('[data-live-updated]')).visibility === 'visible' && getComputedStyle(r.querySelector('[data-live-asof]')).visibility === 'hidden' && getComputedStyle(document.querySelector('#live-2 [data-live-updated]')).visibility === 'hidden' && Math.round(a.top) === Math.round(u.top) && Math.round(a.left) === Math.round(u.left); }));
  await page.waitForTimeout(1300);
  check('A7 one polite announcement names the panel', (await page.locator('[data-live-announcer]').innerText()) === 'Plan status has newer data. Use its refresh link to see it.');
  check('A8 marking leaves the connection line unchanged', (await page.locator('[data-live-connection]').innerText()) === 'Live updates on.');
  check('A9 refresh link accessible name includes its panel', (await page.getByRole('link', { name: 'refresh this panel (Plan status)' }).count()) === 1);
  await page.locator('#live-1 [data-live-refresh]').focus();
  const focus = await page.evaluate(() => { const s = getComputedStyle(document.activeElement); const r = document.activeElement.getBoundingClientRect(); return { outline: s.outlineStyle + ' ' + s.outlineWidth, h: Math.round(r.height) }; });
  check('A10 refresh link has visible focus and a 44px target', focus.outline === 'solid 2px' && focus.h >= 44, focus);
  const lightContrast = await contrast(page, sels);
  check('A12 light text contrast >= 4.5', Object.values(lightContrast).every(v => v >= 4.5), lightContrast);
  await page.screenshot({ path: shots + 'A-wide-light-updated.png', fullPage: true });
  const scrollBefore = await page.evaluate(() => { document.querySelector('#live-1').scrollIntoView({ block: 'center' }); return Math.round(scrollY); });
  await page.locator('#live-1 [data-live-refresh]').focus();
  await Promise.all([page.waitForNavigation(), page.keyboard.press('Enter')]);
  await page.waitForFunction(() => window.AppKitLive && window.AppKitLive.state().status === 'live');
  const after = await page.evaluate(() => ({ hash: location.hash, active: document.activeElement.id, asof: document.querySelector('#live-1 [data-live-asof]').innerText, y: Math.round(scrollY), cursor: document.querySelector('[data-live-page]').getAttribute('data-live-last-event-id') }));
  check('A11 Enter on refresh reloads at the region with the new revision, returns focus without scrolling', after.hash === '#live-1' && after.active === 'live-1' && /revision 1$/.test(after.asof) && after.y === scrollBefore, { scrollBefore, ...after });
  const ring = await page.evaluate(() => { const a = document.activeElement; const s = getComputedStyle(a); return { id: a.id, visible: a.matches(':focus-visible'), outline: s.outlineStyle + ' ' + s.outlineWidth }; });
  check('A13 the refocused region paints its focus-visible ring', ring.id === 'live-1' && ring.visible && ring.outline === 'solid 2px', ring);

  lastIds.length = 0;
  await post('/live/drop', {});
  await post('/live/publish', { source: 'index:fixture' });
  await page.waitForFunction(() => document.querySelector('#live-2').getAttribute('data-live-state') === 'updated', null, { timeout: 8000 });
  check('B1 dropped stream reconnects with the page cursor and replays the missed event', lastIds.includes(after.cursor) && (await state()).status === 'live', lastIds.slice());

  const seen = [];
  await page.exposeFunction('__liveSeen', (s) => seen.push(s));
  await page.evaluate(() => window.addEventListener('appkit:live-status', (e) => window.__liveSeen([e.detail.status, e.detail.attempt, e.detail.error || '', Math.round(performance.now())].join(':'))));
  await post('/live/outage', { count: '3' });
  await post('/live/drop', {});
  const t0 = Date.now();
  await page.waitForFunction(() => window.AppKitLive.state().error === 'http-503', null, { timeout: 5000 });
  await page.waitForFunction(() => window.AppKitLive.state().status === 'live', null, { timeout: 30000 });
  const refused = seen.filter(s => s.includes(':http-503:') && s.startsWith('reconnecting'));
  // The drop closes the stream (attempt n, closed); the first refused retry
  // reports the next attempt, so their gap is the first reconnect delay.
  const ci = seen.findIndex(s => s.startsWith('reconnecting:') && s.includes(':closed:'));
  const ri = seen.findIndex((s, i) => i > ci && s.includes(':http-503:'));
  const firstDelay = ci >= 0 && ri > ci ? +seen[ri].split(':')[3] - +seen[ci].split(':')[3] : null;
  check('B2 three refused reconnects back off, then recover; the first retry waits 1-1.5 s', refused.length === 3 && (await state()).status === 'live' && Date.now() - t0 >= 3000 && firstDelay >= 950 && firstDelay <= 1600, { seen: seen.slice(), ms: Date.now() - t0, firstDelay });

  await page.evaluate(() => { window.AppKitLive.stop(); window.AppKitLive.start({ url: '/live/events', events: ['change'], baseDelay: 100, maxDelay: 200 }); });
  await page.waitForFunction(() => window.AppKitLive.state().status === 'live');
  seen.length = 0;
  await post('/live/outage', { count: '8' });
  await post('/live/drop', {});
  await page.waitForFunction(() => window.AppKitLive.state().status === 'unavailable', null, { timeout: 15000 });
  const un = await state();
  check('B3 eight refused attempts stop at unavailable with honest copy and a resume control', un.attempt === 8 && un.error === 'http-503' && seen.filter(s => s.startsWith('reconnecting') && s.includes(':http-503:')).length === 7 && (await page.locator('[data-live-connection]').innerText()) === 'Live updates unavailable. Reload to retry.' && (await page.locator('[data-live-toggle]').innerText()) === 'Resume live updates', { seen: seen.slice() });
  await page.waitForTimeout(300);
  check('B4 unavailable is announced politely', (await page.locator('[data-live-announcer]').innerText()) === 'Live updates unavailable. Reload to retry.');
  await page.locator('[data-live-toggle]').focus();
  await page.keyboard.press('Enter');
  await page.waitForFunction(() => window.AppKitLive.state().status === 'live', null, { timeout: 5000 });
  check('B5 keyboard resume reconnects and keeps focus on the one control', await page.evaluate(() => document.activeElement.hasAttribute('data-live-toggle') && document.activeElement.innerText === 'Stop live updates'));
  check('B5a the control is named only by its visible label (the reserved wider label is hidden)', (await page.getByRole('button', { name: 'Stop live updates', exact: true }).count()) === 1 && (await page.getByRole('button', { name: /Resume/ }).count()) === 0);
  await page.keyboard.press('Enter');
  check('B6 keyboard stop keeps focus and states stopped', (await state()).status === 'stopped' && (await page.evaluate(() => document.activeElement.innerText)) === 'Resume live updates' && (await page.locator('[data-live-connection]').innerText()) === 'Live updates stopped. Reload for newer data.');
  await post('/live/publish', { source: 'sessions:fixture' });
  await page.waitForTimeout(1500);
  check('B7 a stopped page is not marked', (await page.evaluate(() => document.querySelector('#live-3').getAttribute('data-live-state'))) === null);
  await page.keyboard.press('Enter');
  check('B8 resume replays the event missed while stopped', await page.waitForFunction(() => document.querySelector('#live-3').getAttribute('data-live-state') === 'updated', null, { timeout: 5000 }).then(() => true, () => false));

  await page.evaluate(() => { Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' }); document.dispatchEvent(new Event('visibilitychange')); });
  await page.waitForTimeout(4000);
  const early = (await state()).status;
  await page.waitForTimeout(1500);
  check('B9 stream stays through a short hide, pauses after 5 s hidden (synthetic visibility)', early === 'live' && (await state()).status === 'paused' && (await page.locator('[data-live-connection]').innerText()) === 'Paused while this page is hidden.', { early });
  const n = lastIds.length;
  await post('/live/publish', { source: 'plans:fixture' });
  await page.evaluate(() => { Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' }); document.dispatchEvent(new Event('visibilitychange')); });
  await page.waitForFunction(() => window.AppKitLive.state().status === 'live', null, { timeout: 5000 });
  const replayed = await page.waitForFunction(() => document.querySelector('#live-1').getAttribute('data-live-state') === 'updated', null, { timeout: 5000 }).then(() => true, () => false);
  check('B10 visible again reconnects with Last-Event-ID and replays the hidden-time event', lastIds.length === n + 1 && lastIds[n] !== '(none)' && replayed, lastIds.slice(-2));
  await page.screenshot({ path: shots + 'B-wide-light-all-updated.png', fullPage: true });

  // A non-Latin-1 cursor must reach the server as UTF-8 bytes. The fixture only
  // accepts decimal ids and a query-free URL, so it answers 400: that proves the
  // request was sent (fetch did not refuse the header) and was not retried.
  const utf8 = [];
  const onUtf8 = (req) => { if (req.url().includes('/live/events?cursor')) utf8.push(req.headers()['last-event-id']); };
  page.on('request', onUtf8);
  await page.evaluate(() => { window.AppKitLive.stop(); window.AppKitLive.start({ url: '/live/events?cursor', events: ['change'], lastEventId: 'plan-ω1' }); });
  await page.waitForFunction(() => window.AppKitLive.state().status === 'unavailable', null, { timeout: 5000 });
  const refusedCursor = await state();
  page.off('request', onUtf8);
  const sent = await page.evaluate((hs) => hs.map((h) => new TextDecoder().decode(Uint8Array.from([...(h || '')].map((c) => c.charCodeAt(0))))), utf8);
  check('F1 a non-Latin-1 cursor is sent as its UTF-8 bytes (server refusal, not a fetch TypeError)', refusedCursor.error === 'refused-400' && utf8.length === 1 && (sent[0] === 'plan-ω1' || utf8[0] === 'plan-ω1'), { error: refusedCursor.error, raw: utf8, decoded: sent });
  await page.evaluate(() => { window.AppKitLive.stop(); window.AppKitLive.start({ url: '/live/events', events: ['change'] }); });
  await page.waitForFunction(() => window.AppKitLive.state().status === 'live', null, { timeout: 5000 });

  // Marking and every state change keep the page status and all panels in place
  // at narrow widths (320 px is the WCAG reflow width), light and dark.
  const layoutBoxes = (p) => p.evaluate(() => [...document.querySelectorAll('[data-live-page], [data-live-source]')].map(e => { const r = e.getBoundingClientRect(); return [Math.round(r.y + scrollY), Math.round(r.height)].join(','); }).join(' '));
  for (const [w, theme] of [[320, 'light'], [340, 'dark'], [360, 'light']]) {
    await page.setViewportSize({ width: w, height: 844 });
    await page.goto(`http://127.0.0.1:8775/live?theme=${theme}`);
    await page.waitForFunction(() => window.AppKitLive && window.AppKitLive.state().status === 'live');
    const seq = [['live', await layoutBoxes(page)]];
    await post('/live/publish', { source: 'sessions:fixture' });
    await page.waitForFunction(() => window.AppKitLive.state().updated.length === 1, null, { timeout: 5000 });
    seq.push(['marked', await layoutBoxes(page)]);
    await page.evaluate(() => window.AppKitLive.stop());
    seq.push(['stopped', await layoutBoxes(page)]);
    await page.evaluate(() => window.AppKitLive.start());
    await page.waitForFunction(() => window.AppKitLive.state().status === 'live', null, { timeout: 5000 });
    seq.push(['resumed', await layoutBoxes(page)]);
    await post('/live/outage', { count: '1' });
    await post('/live/drop', {});
    await page.waitForFunction(() => window.AppKitLive.state().error === 'http-503', null, { timeout: 5000 });
    seq.push(['reconnecting', await layoutBoxes(page)]);
    await page.waitForFunction(() => window.AppKitLive.state().status === 'live', null, { timeout: 10000 });
    narrow[w] = { theme, seq };
    check(`F2 ${w}px ${theme}: marking, stop, resume and reconnect move no panel or status box`, seq.every(([, b]) => b === seq[0][1]) && (await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)) <= 0, seq);
    if (w === 320) await page.screenshot({ path: shots + 'F-320-light-marked.png', fullPage: true });
  }

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('http://127.0.0.1:8775/live?theme=dark');
  await page.waitForFunction(() => window.AppKitLive && window.AppKitLive.state().status === 'live');
  const layout = await page.evaluate(() => { const h1 = document.querySelector('h1').getBoundingClientRect(); const acts = document.querySelector('[data-page-heading] .flex-wrap.items-center.gap-2').getBoundingClientRect(); return { overflow: document.documentElement.scrollWidth - innerWidth, actionsBelow: acts.top >= h1.bottom, theme: document.documentElement.dataset.theme }; });
  check('C1 narrow dark: no horizontal overflow, heading actions wrap below the title', layout.overflow <= 0 && layout.actionsBelow && layout.theme === 'dark', layout);
  const n0 = await docRects(page); const nb0 = await bodies(page);
  await page.getByRole('button', { name: 'Publish a gap (all panels)' }).click();
  await page.waitForFunction(() => [...document.querySelectorAll('[data-live-source]')].every(r => r.getAttribute('data-live-state') === 'updated'), null, { timeout: 5000 });
  const n1 = await docRects(page); const nb1 = await bodies(page);
  check('C2 narrow: a gap wildcard marks every panel and still moves nothing', JSON.stringify(n0) === JSON.stringify(n1) && JSON.stringify(nb0) === JSON.stringify(nb1), n0.map((v, i) => v === n1[i] ? null : [i, v, n1[i]]).filter(Boolean));
  await page.waitForTimeout(1300);
  check('C3 one coalesced announcement for all panels', (await page.locator('[data-live-announcer]').innerText()) === '3 panels have newer data: Plan status, Index freshness, Recent sessions.');
  const darkContrast = await contrast(page, { ...sels, asof: '#live-1 [data-live-updated]' });
  check('C4 dark text contrast >= 4.5', Object.values(darkContrast).every(v => v >= 4.5), darkContrast);
  await page.screenshot({ path: shots + 'C-narrow-dark-updated.png', fullPage: true });

  for (const [w, h, theme] of [[1280, 900, 'light'], [390, 844, 'dark']]) {
    await page.setViewportSize({ width: w, height: h });
    for (const v of ['full', 'minimal', 'long']) {
      await page.goto(`http://127.0.0.1:8775/page-heading?variant=${v}&theme=${theme}`);
      const m = await page.evaluate(() => { const hd = document.querySelector('[data-page-heading]'); const h1 = hd.querySelector('h1').getBoundingClientRect(); const actions = hd.querySelector('.gap-2.flex-wrap'); const a = actions && actions.getBoundingClientRect(); const targets = actions ? [...actions.children].map(c => Math.round(c.getBoundingClientRect().height)) : []; return { h1s: document.querySelectorAll('h1').length, overflow: document.documentElement.scrollWidth - innerWidth, below: a ? a.top >= h1.bottom - 1 : null, beside: a ? a.left >= h1.right - 1 : null, targets, h1Height: Math.round(h1.height) }; });
      const wide = w > 600;
      const layoutOk = m.below === null || (wide ? m.beside : m.below);
      check(`D ${theme} ${w}px ${v}: one h1, no overflow, actions ${wide ? 'beside' : 'below'} title, targets >= 44px`, m.h1s === 1 && m.overflow <= 0 && layoutOk && m.targets.every(t => t >= 44), m);
      if (v !== 'minimal') await page.screenshot({ path: shots + `D-${theme}-${w}-${v}.png`, fullPage: true });
    }
  }
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto('http://127.0.0.1:8775/page-heading?variant=long&theme=light');
  await page.locator('[data-page-heading] nav a').first().focus();
  const order = [];
  for (let i = 0; i < 8; i++) { order.push(await page.evaluate(() => { const a = document.activeElement; const s = getComputedStyle(a); return (a.getAttribute('aria-disabled') ? '[inert]' : '') + (a.textContent || '').trim().slice(0, 28) + (s.outlineStyle !== 'none' || s.boxShadow !== 'none' ? ' (focus shown)' : ' (NO FOCUS STYLE)'); })); await page.keyboard.press('Tab'); }
  check('D2 keyboard order: breadcrumbs, actions (inert unsafe action skipped), then variant links', order[0].startsWith('Overview') && order.some(o => o.startsWith('Re-rank')) && !order.some(o => o.startsWith('[inert]')) && order.findIndex(o => o.startsWith('Full')) > order.findIndex(o => o.startsWith('Open evidence')), order);
  check('D3 focus indication on every heading stop', order.every(o => o.endsWith('(focus shown)')), order);
  const inert = await page.evaluate(() => { const s = document.querySelector('[aria-disabled="true"]'); return { text: s.textContent, role: s.getAttribute('role'), tab: s.tabIndex, href: s.hasAttribute('href') }; });
  check('D4 unsafe action is an inert, unfocusable label', inert.role === 'link' && inert.tab === -1 && !inert.href, inert);
  } catch (e) { check('run error', false, String(e)); }
  check('Z no script errors (resource 503s during deliberate outages are expected)', scriptErrors.length === 0, { scriptErrors, resourceErrors: resourceErrors.length });
  await page.close();

  const nojs = await ctx.browser().newContext({ javaScriptEnabled: false, viewport: { width: 1280, height: 900 } });
  const p = await nojs.newPage();
  try {
    await p.goto('http://127.0.0.1:8775/live?theme=light');
    const t = await p.locator('[data-live-connection]').innerText();
    const toggle = await p.locator('[data-live-toggle]').isVisible();
    const updatedVisible = await p.locator('[data-live-updated]').first().isVisible();
    check('E1 without JavaScript: honest off state, no control, no updated claim', t === 'Live updates are off. Reload for newer data.' && !toggle && !updatedVisible, { t, toggle, updatedVisible });
    await p.setViewportSize({ width: 320, height: 844 });
    await p.reload();
    const offBox = Math.round((await p.locator('[data-live-page]').boundingBox()).height);
    const liveBox = +narrow[320].seq[0][1].split(' ')[0].split(',')[1];
    check('E3 at 320 px the script-free status box already has the running height, so starting moves nothing', offBox === liveBox, { offBox, liveBox });
    await p.setViewportSize({ width: 1280, height: 900 });
    await p.getByRole('button', { name: 'Publish Plans change' }).click();
    await p.waitForURL('**/live');
    const asof = await p.locator('#live-1 [data-live-asof]').innerText();
    check('E2 without JavaScript: fixture POST returns to the page with the newer revision', /revision \d+$/.test(asof) && !asof.endsWith('revision 0'), asof);
  } catch (e) { check('E run error', false, String(e)); }
  await nojs.close();
  return { passed: checks.filter(c => c.ok).length, total: checks.length, failed: checks.filter(c => !c.ok), details: checks.map(c => [c.ok ? 'PASS' : 'FAIL', c.name, c.detail]) };
}
