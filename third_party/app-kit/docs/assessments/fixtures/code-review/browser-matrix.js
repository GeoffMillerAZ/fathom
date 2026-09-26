// Browser matrix for the Gallery /agentic/code specimens (content.code-excerpt,
// work.unified-diff). Run as a Playwright MCP browser_run_code function against a
// Gallery built from the tested revision; it opens its own browser context so a
// shared MCP page cannot be navigated underneath it. It returns observations,
// not a verdict: compare them with the expectations in the assessment.
async (sharedPage) => {
  const BASE = 'http://127.0.0.1:8774/agentic/code';
  const browser = sharedPage.context().browser();
  const ctx = browser ? await browser.newContext() : sharedPage.context();
  const page = await ctx.newPage();
  const errors = [];
  // Chrome's resource-error text omits the URL, so the favicon filter reads the location.
  page.on('console', m => { const where = m.text() + ' @ ' + (m.location().url || ''); if (m.type() === 'error' && !where.includes('favicon')) errors.push(where); });
  page.on('pageerror', e => errors.push(String(e)));
  const R = { isolatedContext: !!browser, userAgent: await page.evaluate(() => navigator.userAgent) };
  const active = () => page.evaluate(() => { const a = document.activeElement; const cs = getComputedStyle(a); return { tag: a.tagName, name: a.getAttribute('aria-label') || a.innerText.slice(0, 60), focusVisible: a.matches(':focus-visible'), outline: cs.outlineStyle + ' ' + cs.outlineWidth + ' ' + cs.outlineColor }; });
  const targetInfo = () => page.evaluate(() => { const li = document.querySelector(':target'); if (!li) return null; const cs = getComputedStyle(li, '::after'); return { id: li.id, frame: cs.position + ' ' + cs.borderTopWidth + ' ' + cs.borderTopStyle + ' ' + cs.borderTopColor + ' z' + cs.zIndex, top: Math.round(li.getBoundingClientRect().top) }; });
  try {
    // T1 1440/light keyboard: Tab order, visible focus, permalink by Enter.
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(BASE + '?theme=light');
    await page.locator('a[href="#review-L49"]').first().focus();
    const order = [];
    for (let i = 0; i < 3; i++) { await page.keyboard.press('Tab'); order.push(await active()); }
    await page.keyboard.press('Enter');
    R.T1 = { order, hash: await page.evaluate(() => location.hash), target: await targetInfo() };
    // T2 pointer: in-page jump link targets a not-executed line.
    await page.locator('a[href="#review-L49"]').first().click();
    R.T2 = await targetInfo();
    // T3 keyboard horizontal scroll with a pinned gutter; no wrapping.
    const wide = page.getByRole('region', { name: 'limits.go, lines 1 to 7, long lines scroll horizontally' });
    await wide.focus();
    const probe = () => wide.evaluate(el => { const n = el.querySelector('a[href="#wide-L4"]'); return { scrollLeft: el.scrollLeft, overflow: el.scrollWidth - el.clientWidth, gutterX: Math.round(n.getBoundingClientRect().left - el.getBoundingClientRect().left) }; });
    const before = await probe();
    for (let i = 0; i < 10; i++) await page.keyboard.press('ArrowRight');
    await page.waitForTimeout(300);
    const after = await probe();
    for (let i = 0; i < 10; i++) await page.keyboard.press('ArrowLeft');
    await page.waitForTimeout(300);
    R.T3 = { focus: await active(), before, after, back: await probe(), rowHeights: await page.evaluate(() => ({ excerpt: [...new Set([...document.querySelectorAll('[data-code-excerpt] li[data-line]')].map(li => Math.round(li.getBoundingClientRect().height)))], diff: [...new Set([...document.querySelectorAll('[data-unified-diff] li[data-kind]')].filter(li => li.offsetParent).map(li => Math.round(li.getBoundingClientRect().height)))] })) };
    // T4 1440/dark keyboard: collapsed context opens and closes natively.
    await page.goto(BASE + '?theme=dark');
    await page.getByRole('region', { name: 'deploy/ledger.toml line changes, current to proposed' }).focus();
    await page.keyboard.press('Tab');
    const summary = await active();
    const height = await page.evaluate(() => Math.round(document.activeElement.getBoundingClientRect().height));
    await page.keyboard.press('Enter');
    const opened = await page.evaluate(() => ({ open: document.activeElement.parentElement.open, text: document.activeElement.innerText, shown: [...document.activeElement.parentElement.querySelectorAll('li')].filter(li => li.getBoundingClientRect().height > 0).length }));
    await page.keyboard.press(' ');
    const closed = await page.evaluate(() => ({ open: document.activeElement.parentElement.open, text: document.activeElement.innerText }));
    R.T4 = { summary, height, opened, closed, theme: await page.evaluate(() => document.documentElement.dataset.theme), rows: await page.evaluate(() => { const bg = s => getComputedStyle(document.querySelector(s)).backgroundColor; return { added: bg('[data-kind="added"]'), removed: bg('[data-kind="removed"]'), notExecuted: bg('[data-state="not-executed"]'), changed: bg('[data-state="changed"]') }; }) };
    // T5 390 wide in both themes: no page overflow, inner scrolling, 44px caption
    // link and summary, and the size and pitch of every line-number permalink.
    R.T5 = {};
    for (const theme of ['light', 'dark']) {
      await page.setViewportSize({ width: 390, height: 844 });
      await page.goto(BASE + '?theme=' + theme);
      R.T5[theme] = await page.evaluate(() => {
        // Every line-number permalink: its box and the distance to the next one
        // in the same excerpt (WCAG 2.2 2.5.8 needs 24x24 or 24px spacing).
        const excerpts = [...document.querySelectorAll('[data-code-excerpt] ol')];
        const boxes = excerpts.map(ol => [...ol.querySelectorAll('li[data-line] a')].map(a => a.getBoundingClientRect()));
        const sizes = boxes.flat().map(r => Math.round(r.width) + 'x' + Math.round(r.height));
        const pitches = boxes.flatMap(list => list.slice(1).map((r, i) => Math.round(r.top - list[i].top)));
        return { pageOverflow: Math.max(0, document.documentElement.scrollWidth - document.documentElement.clientWidth), regionsScroll: [...document.querySelectorAll('[data-code-excerpt] [role=region], [data-unified-diff] [role=region]')].map(r => r.scrollWidth > r.clientWidth), captionLink: Math.round(document.querySelector('[data-code-excerpt] figcaption a').getBoundingClientRect().height), summary: Math.round(document.querySelector('[data-diff-collapsed] summary').getBoundingClientRect().height), permalinkSizes: [...new Set(sizes)], permalinkCount: sizes.length, minPitch: Math.min(...pitches), agenticCurrent: !!document.querySelector('a[href^="/agentic"][aria-current="page"]') };
      });
    }
    // T6 accessibility tree: state labels, spoken diff lines, hunk summary, legend.
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(BASE + '?theme=light');
    R.T6 = { excerptLine: await page.locator('#review-L48').ariaSnapshot(), diffLine: await page.locator('[data-unified-diff] [data-kind="added"]').first().ariaSnapshot(), hunk: await page.locator('[data-hunk-header]').first().ariaSnapshot(), legend: await page.locator('[aria-label="Line states"]').first().ariaSnapshot() };
    // T7 revealed characters: the line separator and tag characters draw a
    // visible code point; the precomputed diff states skipped ranges per side.
    R.T7 = await page.evaluate(() => ({
      revealed: [...document.querySelectorAll('[data-code-excerpt] [data-revealed]')].map(s => { const r = s.getBoundingClientRect(); return s.textContent + ':' + Math.round(r.width) + 'x' + Math.round(r.height); }),
      omitted: [...document.querySelectorAll('[data-diff-omitted]')].map(p => p.innerText),
    }));
    R.consoleErrors = errors;
  } finally {
    await page.close();
    if (browser) await ctx.close();
  }
  return R;
}
