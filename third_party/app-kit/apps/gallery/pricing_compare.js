// Local view controls only. Prepared documents remain opaque, sandboxed frames.
(function () {
  'use strict';

  const route = '/comparisons/p02-pricing';
  const choices = {
    layout: ['flip', 'side', 'stack'], active: ['original', 'converted'],
    scale: ['fit', 'actual'], viewport: ['390', '1440'],
    preview_theme: ['light', 'dark'], theme: ['', 'light', 'dark'],
  };
  const valid = (state) => Object.entries(choices).every(([key, values]) => values.includes(state[key]));

  function mount(doc, win) {
    const root = doc.getElementById('comparison');
    const form = doc.getElementById('comparison-controls');
    if (!root || !form) return null;
    const status = doc.getElementById('comparison-status');
    const scaleLabel = doc.getElementById('comparison-scale-label');
    const fullscreen = doc.getElementById('comparison-fullscreen');
    const fields = Object.fromEntries(Object.keys(choices).map((key) => [key, form.elements.namedItem(key)]));
    if (!status || !scaleLabel || Object.keys(choices).some((key) => key !== 'theme' && !fields[key])) return null;
    const panes = Array.from(root.querySelectorAll('.comparison-pane')).map((pane) => ({
      pane, side: pane.dataset.side, window: pane.querySelector('.comparison-window'),
      box: pane.querySelector('.comparison-framebox'), frame: pane.querySelector('iframe'),
    }));
    if (panes.length !== 2 || choices.active.some((side) => panes.filter((p) => p.side === side).length !== 1) ||
        panes.some((p) => !p.window || !p.box || !p.frame)) return null;
    const links = Array.from(doc.querySelectorAll('[data-select-side]'));
    const initialURL = new URL(win.location.href);
    const theme = fields.theme ? fields.theme.value : (initialURL.searchParams.get('theme') || '');
    const read = () => Object.fromEntries(Object.keys(choices).map((key) => [key, key === 'theme' ? theme : fields[key].value]));
    let state = read();
    if (!valid(state)) return null;

    function restoreFields() {
      for (const key of Object.keys(choices)) if (fields[key]) fields[key].value = state[key];
    }

    function href(active) {
      const query = new URLSearchParams();
      for (const key of Object.keys(choices)) {
        const value = key === 'active' ? active : state[key];
        if (key !== 'theme' || value) query.set(key, value);
      }
      return route + '?' + query.toString() + initialURL.hash;
    }

    function reflow() {
      const available = Math.min(...panes.filter((p) => !p.pane.hidden).map((p) => p.window.clientWidth));
      if (!Number.isFinite(available) || available < 0) return;
      const scale = state.scale === 'actual' ? 1 : Math.max(0, Math.min(1, available / Number(state.viewport)));
      for (const p of panes) {
        p.frame.style.transformOrigin = 'top left';
        p.frame.style.transform = 'scale(' + scale + ')';
        p.box.style.width = Number(state.viewport) * scale + 'px';
        p.box.style.height = 1000 * scale + 'px';
      }
      scaleLabel.textContent = Math.round(scale * 100) + '% · ' + state.viewport + ' × 1000 CSS px';
    }

    function apply(next, initial = false) {
      if (!valid(next)) {
        restoreFields();
        status.textContent = 'Unsupported comparison selection. The previous view is unchanged.';
        return;
      }
      const previous = state;
      state = next;
      restoreFields();
      root.dataset.layout = state.layout;
      root.dataset.active = state.active;
      root.dataset.scale = state.scale;
      for (const p of panes) {
        p.pane.hidden = state.layout === 'flip' && p.side !== state.active;
        if (initial || state.viewport !== previous.viewport) {
          p.frame.setAttribute('width', state.viewport);
          p.frame.style.width = state.viewport + 'px';
          p.frame.style.height = '1000px';
        }
        if (initial || state.viewport !== previous.viewport || state.preview_theme !== previous.preview_theme) {
          const name = p.side === 'original' ? 'Original HTML' : 'Converted templ';
          p.frame.setAttribute('title', name + ': ' + state.viewport + ' × 1000, ' + state.preview_theme);
        }
        // Do not reassign unchanged sources: layout/flip/zoom preserve frame state.
        if (!initial && state.preview_theme !== previous.preview_theme) {
          p.frame.setAttribute('src', route + '/document/' + p.side + '-' + state.preview_theme + '.html');
        }
      }
      for (const link of links) {
        const side = link.dataset.selectSide;
        if (!choices.active.includes(side)) continue;
        link.setAttribute('href', href(side));
        if (side === state.active) link.setAttribute('aria-current', 'true');
        else link.removeAttribute('aria-current');
      }
      reflow();
      if (!initial) {
        try { win.history.replaceState(null, '', href(state.active)); } catch (_) {
          // Native form and links remain usable if history updates are unavailable.
        }
        status.textContent = state.layout === 'flip'
          ? (state.active === 'original' ? 'Original HTML' : 'Converted templ') + ' shown. Pane scroll positions are independent.'
          : 'Both previews shown. Pane scroll positions are independent.';
      }
    }

    form.addEventListener('change', () => apply(read()));
    form.addEventListener('submit', (event) => { event.preventDefault(); apply(read()); });
    for (const link of links) link.addEventListener('click', (event) => {
      if (event.defaultPrevented || event.button > 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      event.preventDefault();
      apply({ ...state, active: link.dataset.selectSide });
    });
    win.addEventListener('resize', reflow);
    if (typeof win.ResizeObserver === 'function') {
      const observer = new win.ResizeObserver(reflow);
      for (const p of panes) observer.observe(p.window);
    }

    if (fullscreen && doc.fullscreenEnabled && typeof doc.documentElement.requestFullscreen === 'function') {
      fullscreen.hidden = false;
      const fullscreenState = () => {
        fullscreen.textContent = doc.fullscreenElement ? 'Exit fullscreen' : 'Enter fullscreen';
        fullscreen.setAttribute('aria-pressed', String(!!doc.fullscreenElement));
        reflow();
      };
      fullscreen.addEventListener('click', async () => {
        try {
          if (doc.fullscreenElement) await doc.exitFullscreen();
          else await doc.documentElement.requestFullscreen();
        } catch (_) {
          status.textContent = 'Fullscreen is unavailable. Continue using the comparison on this page.';
        }
      });
      doc.addEventListener('fullscreenchange', fullscreenState);
      fullscreenState();
    }
    apply(state, true);
    return { reflow };
  }

  if (typeof module === 'object' && module.exports) module.exports = { mount };
  if (typeof document !== 'undefined' && typeof window !== 'undefined') {
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => mount(document, window), { once: true });
    else mount(document, window);
  }
}());
