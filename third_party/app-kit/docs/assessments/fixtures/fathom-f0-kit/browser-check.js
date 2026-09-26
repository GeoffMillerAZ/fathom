async (page) => {
  // Executed through the Playwright MCP browser_run_code tool against a local
  // Gallery binary. Not part of task check; see
  // docs/assessments/2026-09-25-fathom-f0-kit-returns.md for identities and
  // results. The same bytes run against the baseline binary with only `base`
  // changed, to reproduce each failure before the fix.
  const base = 'http://127.0.0.1:18933';
  const out = {};
  for (const [w, h, theme] of [[390, 844, 'light'], [390, 844, 'dark'], [1280, 900, 'light'], [1280, 900, 'dark']]) {
    await page.setViewportSize({ width: w, height: h });
    const cell = {};

    // FATHOM-F0-KIT-01/02: /layout's specimen table (three columns, an Open
    // link column with an sr-only "Actions" header).
    let resp = await page.goto(`${base}/layout?theme=${theme}`);
    cell.layoutStatus = resp.status();
    cell.table = await page.evaluate(() => {
      const table = document.querySelector('table');
      const wrapper = table.parentElement;
      const hidden = [...table.querySelectorAll('th .sr-only')].map((s) => ({
        text: s.textContent,
        containedByWrapper: s.offsetParent === wrapper,
      }));
      return {
        pageOverflow: document.documentElement.scrollWidth - window.innerWidth,
        wrapperScrolls: wrapper.scrollWidth > wrapper.clientWidth,
        wrapperPosition: getComputedStyle(wrapper).position,
        headers: table.querySelectorAll('th').length,
        headersWithColScope: table.querySelectorAll('th[scope="col"]').length,
        hidden,
      };
    });

    // FATHOM-F0-KIT-03: /agentic's breadcrumb trail. Measure every crumb link,
    // then reach the first one with Tab (a real keyboard path, so
    // :focus-visible applies) and read its outline.
    resp = await page.goto(`${base}/agentic?theme=${theme}`);
    cell.agenticStatus = resp.status();
    cell.crumbs = await page.evaluate(() => [...document.querySelectorAll('nav[aria-label="Breadcrumb"] a')].map((a) => {
      const b = a.getBoundingClientRect();
      return { text: a.textContent, w: Math.round(b.width * 10) / 10, h: Math.round(b.height * 10) / 10 };
    }));
    let focus = null;
    for (let i = 0; i < 400 && !focus; i++) {
      await page.keyboard.press('Tab');
      focus = await page.evaluate(() => {
        const a = document.activeElement;
        if (!a || !a.closest('nav[aria-label="Breadcrumb"]')) return null;
        const cs = getComputedStyle(a);
        return { text: a.textContent, focusVisible: a.matches(':focus-visible'), outlineStyle: cs.outlineStyle, outlineWidth: cs.outlineWidth, outlineOffset: cs.outlineOffset, outlineColor: cs.outlineColor };
      });
      if (focus) focus.tabs = i + 1;
    }
    cell.crumbFocus = focus;

    // FATHOM-F0-KIT-04: /components' fluid sparkline fills its card's content
    // box at every width, keeps the 320x48 viewBox ratio and stays inside.
    resp = await page.goto(`${base}/components?theme=${theme}`);
    cell.componentsStatus = resp.status();
    cell.fluid = await page.evaluate(() => {
      const fig = [...document.querySelectorAll('figure')].find((f) => f.querySelector('figcaption')?.textContent === 'Sparkline — fluid (fills its container)');
      if (!fig) return { present: false };
      const svg = fig.querySelector('svg');
      const box = svg.parentElement;
      const cs = getComputedStyle(box);
      const content = box.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
      const s = svg.getBoundingClientRect();
      const f = fig.getBoundingClientRect();
      return {
        present: true,
        svgW: Math.round(s.width * 10) / 10,
        svgH: Math.round(s.height * 10) / 10,
        contentW: Math.round(content * 10) / 10,
        ratio: Math.round((s.width / s.height) * 1000) / 1000,
        inside: s.left >= f.left - 0.5 && s.right <= f.right + 0.5,
        pageOverflow: document.documentElement.scrollWidth - window.innerWidth,
      };
    });
    out[`${w}-${theme}`] = cell;
  }
  return out;
}
