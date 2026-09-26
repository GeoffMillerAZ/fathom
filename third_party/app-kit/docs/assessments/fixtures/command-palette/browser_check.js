// Command-palette browser check (interaction.command-palette, Gallery /palette).
// Executed as one Playwright MCP browser_run_code function against a Gallery
// binary on http://127.0.0.1:8771. Each scenario uses its own browser context so a
// shared browser session cannot interleave navigation. Returns one JSON object;
// the retained result is results.json beside this file. Scenario F and the
// hint/no-JavaScript fields in A, D and E reproduce the second-pass review
// findings (re-mount, full-page answers, late triggers, init(root), keyboard
// layouts, zero-result partial answers).
async (page) => {
  const base = 'http://127.0.0.1:8771';
  const browser = page.context().browser();
  const out = { browser: browser.version() };
  // The MCP runner has no URL global outside the page; strip the origin by hand.
  const rel = (u) => u.replace(/^https?:\/\/[^/]+/, '');
  const frag = (p) => p.evaluate(() => {
    const f = document.querySelector('[data-palette-results] [data-palette-fragment]');
    const i = document.getElementById('gallery-palette-input');
    return {
      state: f.getAttribute('data-palette-state'), query: f.getAttribute('data-palette-query'),
      expanded: i.getAttribute('aria-expanded'), active: i.getAttribute('aria-activedescendant'),
      busy: !document.querySelector('[data-palette-busy]').hidden,
    };
  });
  const waitState = (p, state, timeout = 4000) => p.waitForFunction(
    (s) => document.querySelector('[data-palette-results] [data-palette-fragment]').getAttribute('data-palette-state') === s,
    state, { timeout });
  const instrument = (p) => p.evaluate(() => {
    window.__log = [];
    const orig = window.fetch;
    window.fetch = function (url, init) {
      const rec = { q: new URL(String(url)).searchParams.get('q'), header: init && init.headers && init.headers['X-AppKit-Palette'], aborted: false, status: null };
      window.__log.push(rec);
      if (init && init.signal) init.signal.addEventListener('abort', () => { rec.aborted = true; });
      return orig.apply(this, arguments).then((r) => { rec.status = r.status; return r; });
    };
  });
  const log = (p) => p.evaluate(() => window.__log.map((r) => `${r.q}:${r.status === null ? 'none' : r.status}${r.aborted ? ':aborted' : ''}${r.header === '1' ? '' : ':noheader'}`));

  // A. Apple platform (this host), light, 1440 wide: keyboard contract.
  let ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, colorScheme: 'light' });
  let p = await ctx.newPage();
  try {
    await p.goto(base + '/palette?theme=light');
    const trig = p.locator('[data-palette-open]');
    const a = out.A = {};
    a.platform = await p.evaluate(() => navigator.platform);
    a.trigger = await trig.evaluate((el) => ({ h: Math.round(el.getBoundingClientRect().height), haspopup: el.getAttribute('aria-haspopup'), keys: el.getAttribute('aria-keyshortcuts'), hint: el.querySelector('[data-palette-hint]').textContent, hintHidden: el.querySelector('[data-palette-hint]').hidden }));
    await instrument(p);
    await p.keyboard.press('Control+k');
    a.ctrlKOpens = await p.evaluate(() => document.getElementById('gallery-palette').open);
    const invoker = p.getByRole('link', { name: 'the full search page' });
    await invoker.focus();
    await p.keyboard.press('Meta+k');
    a.metaKOpens = await p.evaluate(() => document.getElementById('gallery-palette').open);
    a.focusAfterOpen = await p.evaluate(() => document.activeElement.id);
    a.dialogName = await p.evaluate(() => document.getElementById('gallery-palette').getAttribute('aria-label'));
    await p.keyboard.type('overlay', { delay: 20 });
    await p.waitForSelector('#gallery-palette-listbox');
    await p.waitForTimeout(150);
    a.requests = await log(p);
    a.results = await p.evaluate(() => {
      const i = document.getElementById('gallery-palette-input');
      return {
        controls: i.getAttribute('aria-controls'), active: i.getAttribute('aria-activedescendant'),
        groups: [...document.querySelectorAll('#gallery-palette-listbox [role=group]')].map((g) => document.getElementById(g.getAttribute('aria-labelledby')).textContent),
        options: [...document.querySelectorAll('#gallery-palette-listbox [role=option]')].map((o) => `${o.id}|${o.getAttribute('aria-selected')}|${Math.round(o.getBoundingClientRect().height)}px|${o.getAttribute('tabindex')}`),
        status: document.querySelector('[data-palette-status]').textContent,
        pageLink: document.querySelector('[data-palette-page]').getAttribute('href'),
      };
    });
    await p.keyboard.press('ArrowDown');
    a.afterDown = await p.evaluate(() => document.getElementById('gallery-palette-input').getAttribute('aria-activedescendant'));
    await p.keyboard.press('ArrowDown');
    a.afterDownWrap = await p.evaluate(() => document.getElementById('gallery-palette-input').getAttribute('aria-activedescendant'));
    await p.keyboard.press('ArrowUp');
    a.afterUpWrap = await p.evaluate(() => document.getElementById('gallery-palette-input').getAttribute('aria-activedescendant'));
    a.selectedStyle = await p.evaluate(() => { const cs = getComputedStyle(document.querySelector('[aria-selected=true]')); return `${cs.outlineStyle} ${cs.outlineWidth}; bg ${cs.backgroundColor}`; });
    await p.keyboard.press('Escape');
    a.escape = await p.evaluate(() => ({ open: document.getElementById('gallery-palette').open, focus: document.activeElement.textContent.trim() }));
    await trig.click();
    a.reopen = await p.evaluate(() => { const i = document.activeElement; return { focus: i.id, value: i.value, selected: i.selectionEnd - i.selectionStart }; });
    const href = await p.evaluate(() => document.querySelector('[aria-selected=true]').getAttribute('href'));
    await Promise.all([p.waitForURL('**' + href.split('?')[0] + '**'), p.keyboard.press('Enter')]);
    a.enterFollows = { selected: href, landed: rel(p.url()) };
    await p.goto(base + '/palette?theme=light');
    await p.keyboard.press('Meta+k');
    await p.keyboard.type('overlay');
    await p.waitForSelector('#gallery-palette-listbox');
    await p.waitForTimeout(150);
    await p.keyboard.type('s');
    await Promise.all([p.waitForURL('**/palette/search**'), p.keyboard.press('Enter')]);
    a.staleEnter = rel(p.url());
    await p.goto(base + '/palette?theme=light');
    await p.keyboard.press('Meta+k');
    await p.locator('#gallery-palette [data-overlay-dismiss]').click();
    a.closeButton = await p.evaluate(() => document.getElementById('gallery-palette').open);
    await p.keyboard.press('Meta+k');
    await p.mouse.click(8, 8);
    a.backdrop = await p.evaluate(() => document.getElementById('gallery-palette').open);
    const [tab] = await Promise.all([ctx.waitForEvent('page'), trig.click({ modifiers: ['Meta'] })]);
    await tab.waitForLoadState();
    a.modifiedClick = { newTab: rel(tab.url()), dialogOpen: await p.evaluate(() => document.getElementById('gallery-palette').open) };
    await tab.close();
  } finally { await ctx.close(); }

  // B. States and request lifecycle (Apple platform, dark via data-theme).
  ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  p = await ctx.newPage();
  try {
    await p.goto(base + '/palette?theme=dark');
    await instrument(p);
    await p.keyboard.press('Meta+k');
    const input = p.locator('#gallery-palette-input');
    const b = out.B = {};
    const resultsTop = () => p.evaluate(() => Math.round(document.querySelector('[data-palette-results]').getBoundingClientRect().top));
    b.resultsTopIdle = await resultsTop();
    await input.fill('zzz'); await waitState(p, 'empty'); b.empty = await frag(p);
    await input.fill('partial'); await waitState(p, 'partial'); b.partial = await frag(p);
    b.partialNotice = await p.evaluate(() => document.querySelector('[data-palette-notice]').textContent);
    await input.fill('fail'); await waitState(p, 'error'); b.fail = await frag(p);
    await input.fill('forms'); await waitState(p, 'results'); b.recovered = await frag(p);
    await input.fill('slow');
    await p.waitForTimeout(400);
    b.slowPending = await frag(p);
    b.resultsTopBusy = await resultsTop();
    await input.fill('slowly');
    await p.waitForTimeout(2000);
    b.afterSlowSuperseded = await frag(p);
    const t0 = Date.now();
    await input.fill('stall');
    await waitState(p, 'error', 9000);
    b.stallErrorMs = Date.now() - t0;
    b.stall = await frag(p);
    const n = (await log(p)).length;
    await input.fill('');
    await p.waitForTimeout(400);
    b.cleared = await frag(p);
    b.requestsOnClear = (await log(p)).length - n;
    b.requests = await log(p);
  } finally { await ctx.close(); }

  // C. Narrow 390px, OS dark: layout, wrapping, targets.
  ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, colorScheme: 'dark' });
  p = await ctx.newPage();
  try {
    await p.goto(base + '/palette');
    await p.locator('[data-palette-open]').click();
    await p.locator('#gallery-palette-input').fill('long');
    await p.waitForSelector('#gallery-palette-listbox');
    await p.waitForTimeout(100);
    out.C = await p.evaluate(() => {
      const box = (el) => { const r = el.getBoundingClientRect(); return `${Math.round(r.width)}x${Math.round(r.height)}`; };
      const res = document.querySelector('[data-palette-results]');
      return {
        bodyBg: getComputedStyle(document.body).backgroundColor,
        dialog: box(document.getElementById('gallery-palette')),
        pageOverflowX: document.documentElement.scrollWidth > innerWidth,
        resultsOverflowX: res.scrollWidth > res.clientWidth,
        options: [...document.querySelectorAll('[role=option]')].map(box),
        input: box(document.getElementById('gallery-palette-input')),
        pageLink: box(document.querySelector('[data-palette-page]')),
        overlayClose: box(document.querySelector('#gallery-palette [data-overlay-dismiss]')),
        keyHints: getComputedStyle(document.querySelector('#gallery-palette footer p')).display,
      };
    });
    await p.screenshot({ path: '/private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946-4037-4a56-8b48-3336a3490e6f/scratchpad/kit-palette/shots/final-narrow-dark-long.png' });
  } finally { await ctx.close(); }

  // D. Non-Apple platform emulation: Control is the Mod key.
  ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  await ctx.addInitScript(() => {
    Object.defineProperty(Navigator.prototype, 'platform', { get: () => 'Win32' });
    Object.defineProperty(Navigator.prototype, 'userAgentData', { get: () => ({ platform: 'Windows' }) });
  });
  p = await ctx.newPage();
  try {
    await p.goto(base + '/palette?theme=dark');
    const t = p.locator('[data-palette-open]');
    out.D = { hint: await t.locator('[data-palette-hint]').textContent(), hintVisible: await t.locator('[data-palette-hint]').isVisible(), keys: await t.getAttribute('aria-keyshortcuts') };
    await p.keyboard.press('Meta+k');
    out.D.metaOpens = await p.evaluate(() => document.getElementById('gallery-palette').open);
    await p.keyboard.press('Control+k');
    out.D.ctrlOpens = await p.evaluate(() => document.getElementById('gallery-palette').open);
    out.D.focus = await p.evaluate(() => document.activeElement.id);
  } finally { await ctx.close(); }

  // E. JavaScript disabled: the trigger and GET search page carry the task.
  ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, javaScriptEnabled: false, colorScheme: 'dark' });
  p = await ctx.newPage();
  try {
    await p.goto(base + '/palette?theme=dark');
    const e = out.E = {};
    // Without the script nothing may promise the shortcut it implements.
    e.trigger = await p.locator('[data-palette-open]').evaluate((el) => {
      const h = el.querySelector('[data-palette-hint]');
      return { keys: el.getAttribute('aria-keyshortcuts'), haspopup: el.getAttribute('aria-haspopup'), hintHidden: h.hidden, hintDisplay: getComputedStyle(h).display, hintText: h.textContent };
    });
    await p.keyboard.press('Control+k');
    await p.keyboard.press('Meta+k');
    e.shortcutNoop = { url: rel(p.url()), open: await p.evaluate(() => document.getElementById('gallery-palette').open) };
    await Promise.all([p.waitForURL('**/palette/search**'), p.locator('[data-palette-open]').click()]);
    e.afterTrigger = rel(p.url());
    await p.getByRole('searchbox', { name: 'Search the Gallery' }).fill('overlay');
    await Promise.all([p.waitForURL('**q=overlay**'), p.getByRole('button', { name: 'Search the Gallery' }).click()]);
    e.afterSubmit = rel(p.url());
    e.page = await p.evaluate(() => ({
      theme: document.documentElement.getAttribute('data-theme'),
      headings: [...document.querySelectorAll('main h1, main h2')].map((h) => `${h.tagName}:${h.textContent}`),
      links: [...document.querySelectorAll('[data-palette-option]')].map((l) => `${l.getAttribute('href')} role=${l.getAttribute('role')}`),
      targets: [...document.querySelectorAll('[data-palette-option], main form button, main form input[type=search]')].map((el) => Math.round(el.getBoundingClientRect().height)),
      overflowX: document.documentElement.scrollWidth > innerWidth,
    }));
    await p.getByRole('link', { name: /Overlays/ }).first().focus();
    e.linkFocus = await p.evaluate(() => { const cs = getComputedStyle(document.activeElement); return `${cs.outlineStyle} ${cs.outlineWidth}`; });
    const res = await p.goto(base + '/palette/search?q=fail&theme=dark');
    e.failPage = { status: res.status(), state: await p.evaluate(() => document.querySelector('[data-palette-fragment]').getAttribute('data-palette-state')) };
  } finally { await ctx.close(); }

  // F. Second-pass review corrections (Apple platform, light).
  ctx = await browser.newContext({ viewport: { width: 1280, height: 800 }, colorScheme: 'light' });
  p = await ctx.newPage();
  const pageErrors = [];
  p.on('pageerror', (err) => pageErrors.push(err.message));
  const opened = () => p.evaluate(() => ({ open: document.getElementById('gallery-palette').open, prevented: window.__prevented }));
  const watchPrevented = () => p.evaluate(() => window.addEventListener('keydown', (ev) => { window.__prevented = ev.defaultPrevented; }));
  try {
    const f = out.F = {};
    // F1. Re-mount: replace the palette markup (as an htmx/Datastar swap would),
    // press the shortcut before and after the documented init call.
    await p.goto(base + '/palette?theme=light');
    await watchPrevented();
    await p.keyboard.press('Meta+k');
    f.remountBaseline = await opened();
    await p.keyboard.press('Escape');
    await p.evaluate(() => {
      const old = document.getElementById('gallery-palette');
      const holder = document.createElement('div');
      holder.innerHTML = old.outerHTML;
      old.replaceWith(holder.firstElementChild);
    });
    await p.keyboard.press('Meta+k');
    f.remountBeforeInit = await opened();
    f.remountWired = await p.evaluate(() => { AppKitPalette.init(document); return !!document.querySelector('[data-palette]').__appkitPalette; });
    await p.getByRole('link', { name: 'the full search page' }).focus();
    await p.keyboard.press('Meta+k');
    f.remountAfterInit = { ...(await opened()), focus: await p.evaluate(() => document.activeElement.id) };
    await p.keyboard.type('overlay', { delay: 20 });
    await p.waitForSelector('#gallery-palette-listbox');
    await p.keyboard.press('Escape');
    f.remountEscape = await p.evaluate(() => ({ open: document.getElementById('gallery-palette').open, focus: document.activeElement.textContent.trim() }));

    // F2. A palette request redirected to a full page: one that renders this
    // palette (and so carries its idle fragment), and a control page without it.
    for (const [name, location] of [['redirectToPalettePage', '/palette?theme=light'], ['redirectToOverlays', '/overlays?theme=light']]) {
      await p.route('**/palette/search**', (route) => (route.request().headers()['x-appkit-palette'] === '1'
        ? route.fulfill({ status: 302, headers: { Location: location } })
        : route.continue()));
      await p.goto(base + '/palette?theme=light');
      await p.keyboard.press('Meta+k');
      await p.keyboard.type('overlay');
      await waitState(p, 'error');
      await p.waitForTimeout(150);
      f[name] = { ...(await frag(p)), status: await p.evaluate(() => document.querySelector('[data-palette-status]').textContent) };
      await p.unroute('**/palette/search**');
    }

    // F4. Zero results while a source was not searched is partial, not absence.
    await p.goto(base + '/palette?theme=light');
    await p.keyboard.press('Meta+k');
    await p.locator('#gallery-palette-input').fill('unsearched');
    await waitState(p, 'partial');
    await p.waitForTimeout(150);
    f.unsearched = await p.evaluate(() => {
      const fr = document.querySelector('[data-palette-results] [data-palette-fragment]');
      return {
        state: fr.getAttribute('data-palette-state'), count: fr.getAttribute('data-palette-count'),
        status: document.querySelector('[data-palette-status]').textContent,
        text: fr.textContent,
      };
    });
    await p.keyboard.press('Escape');

    // F5. A trigger rendered after the palette was wired (a swapped header).
    f.lateTrigger = await p.evaluate(async () => {
      const html = await (await fetch('/palette?theme=light')).text();
      const t = new DOMParser().parseFromString(html, 'text/html').querySelector('[data-palette-open]');
      t.id = 'late-trigger';
      document.body.prepend(document.importNode(t, true));
      const read = () => {
        const el = document.getElementById('late-trigger');
        const h = el.querySelector('[data-palette-hint]');
        return { hint: h.textContent, hintHidden: h.hidden, keys: el.getAttribute('aria-keyshortcuts'), haspopup: el.getAttribute('aria-haspopup') };
      };
      const before = read();
      AppKitPalette.init(document);
      return { before, after: read() };
    });
    await p.locator('#late-trigger').click();
    f.lateTriggerClickOpens = (await opened()).open;
    await p.keyboard.press('Escape');

    // F7. init(root) with the [data-palette] element itself.
    f.initRoot = await p.evaluate(() => {
      const holder = document.createElement('div');
      holder.innerHTML = document.getElementById('gallery-palette').outerHTML.replaceAll('gallery-palette', 'second-palette');
      const dlg = holder.firstElementChild;
      document.body.appendChild(dlg);
      const before = AppKitPalette.open('second-palette');
      AppKitPalette.init(dlg.querySelector('[data-palette]'));
      const after = AppKitPalette.open('second-palette');
      const open = dlg.open;
      AppKitPalette.close('second-palette');
      dlg.remove();
      return { before, after, open };
    });

    // F8. Layout-aware matching: synthetic keydowns as other layouts report them.
    f.layouts = await p.evaluate(() => {
      const dlg = document.getElementById('gallery-palette');
      const fire = (key, code) => {
        const ev = new KeyboardEvent('keydown', { key, code, metaKey: true, bubbles: true, cancelable: true });
        document.body.dispatchEvent(ev);
        const r = { open: dlg.open, prevented: ev.defaultPrevented };
        if (dlg.open) dlg.close();
        return r;
      };
      return { dvorakT_KeyK: fire('t', 'KeyK'), dvorakK_KeyV: fire('k', 'KeyV'), cyrillic_KeyK: fire('л', 'KeyK') };
    });
    f.pageErrors = pageErrors;
  } finally { await ctx.close(); }
  return out;
}
