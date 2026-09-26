async (page) => {
  // Executed through the Playwright MCP browser_run_code tool against a Gallery
  // binary serving 127.0.0.1:8772. Not part of task check; see
  // docs/assessments/2026-09-25-layered-graph.md for identities and results.
  const base = 'http://127.0.0.1:8772';
  const out = {};
  for (const [w, h, theme] of [[1280, 900, 'light'], [1280, 900, 'dark'], [390, 844, 'light'], [390, 844, 'dark']]) {
    await page.setViewportSize({ width: w, height: h });
    const resp = await page.goto(`${base}/graph?theme=${theme}`);
    const r = { status: resp.status() };
    r.pageOverflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    // Sixteen graph-page tab stops: no SVG descendant may take focus, and every
    // stop must show at least a 2px outline.
    await page.focus('#gallery-main');
    let svgFocus = 0;
    const noOutline = [];
    for (let i = 0; i < 16; i++) {
      await page.keyboard.press('Tab');
      const d = await page.evaluate(() => {
        const a = document.activeElement;
        const cs = getComputedStyle(a);
        return { inSvg: !!a.closest('svg'), o: cs.outlineStyle !== 'none' && parseFloat(cs.outlineWidth) >= 2, n: (a.getAttribute('aria-label') || a.textContent || '').trim().slice(0, 40) };
      });
      if (d.inSvg) svgFocus++;
      if (!d.o) noOutline.push(d.n);
    }
    r.svgFocus = svgFocus;
    r.noOutline = noOutline;
    // Target size: open every table alternative, then measure each table link,
    // each SVG node link and each disclosure summary. The kit rule is a 44px
    // high hit area; the table's cell padding does not count.
    r.targets = await page.evaluate(() => {
      document.querySelectorAll('section[id^="graph-"] details').forEach((d) => { d.open = true; });
      const measure = (sel) => [...document.querySelectorAll(sel)].map((el) => {
        const b = el.getBoundingClientRect();
        return { n: (el.textContent || '').trim().slice(0, 32), w: Math.round(b.width), h: Math.round(b.height) };
      });
      const tableLinks = measure('section[id^="graph-"] details table a[href]');
      const svgLinks = measure('section[id^="graph-"] svg a[href]');
      const summaries = measure('section[id^="graph-"] details > summary');
      const small = (list) => list.filter((t) => t.h < 44);
      document.querySelectorAll('section[id^="graph-"] details').forEach((d) => { d.open = false; });
      return {
        tableLinks: tableLinks.length, svgLinks: svgLinks.length, summaries: summaries.length,
        minTableLinkH: Math.min(...tableLinks.map((t) => t.h)), minSvgLinkH: Math.min(...svgLinks.map((t) => t.h)),
        minSummaryH: Math.min(...summaries.map((t) => t.h)),
        below44: [...small(tableLinks), ...small(svgLinks), ...small(summaries)],
        sample: tableLinks.slice(0, 2).concat(svgLinks.slice(0, 2)),
      };
    });
    // Open the first table alternative by keyboard and reach its first link.
    await page.focus('#graph-neighbourhood-section summary');
    await page.keyboard.press('Space');
    r.detailsOpen = await page.evaluate(() => document.querySelector('#graph-neighbourhood-section details').open);
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    r.linkFocus = await page.evaluate(() => {
      const a = document.activeElement;
      const cs = getComputedStyle(a);
      return { href: a.getAttribute('href'), outline: cs.outlineStyle + ' ' + cs.outlineWidth };
    });
    // Accessibility tree: each image exposes a name and description, no children.
    const cdp = await page.context().newCDPSession(page);
    const { nodes } = await cdp.send('Accessibility.getFullAXTree');
    const byId = new Map(nodes.map((n) => [n.nodeId, n]));
    const unignored = (id) => {
      const n = byId.get(id);
      if (!n) return [];
      if (!n.ignored) return [n];
      return (n.childIds || []).flatMap(unignored);
    };
    const imgs = nodes.filter((n) => !n.ignored && n.role && n.role.value === 'image');
    r.images = imgs.map((i) => ({ name: i.name && i.name.value, hasDesc: !!(i.description && i.description.value), exposedChildren: (i.childIds || []).flatMap(unignored).length }));
    r.axLinksInSvg = nodes.filter((n) => !n.ignored && n.role && n.role.value === 'link' && n.name && / internal\//.test(n.name.value)).length;
    await cdp.detach();
    if (w === 390) {
      const region = page.locator('#graph-neighbourhood [role=region]');
      await region.focus();
      await page.keyboard.press('ArrowRight');
      await page.keyboard.press('ArrowRight');
      await page.waitForTimeout(300);
      r.regionScrollLeft = await region.evaluate((el) => el.scrollLeft);
    }
    out[`${theme}-${w}`] = r;
  }
  // Destinations: the SVG node link by pointer, the table link by keyboard.
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`${base}/graph?theme=light`);
  await Promise.all([page.waitForURL(/symbol=store\.ResolveEdges/), page.locator('#graph-neighbourhood svg a[href*="store.ResolveEdges"] rect').click()]);
  out.pointerDestination = page.url().replace(base, '');
  await page.goto(`${base}/graph?theme=dark`);
  await page.focus('#graph-neighbourhood-section summary');
  await page.keyboard.press('Enter');
  await page.keyboard.press('Tab');
  await page.keyboard.press('Tab');
  await Promise.all([page.waitForURL(/symbol=api\.Handle/), page.keyboard.press('Enter')]);
  out.keyboardDestination = page.url().replace(base, '');
  return out;
}
