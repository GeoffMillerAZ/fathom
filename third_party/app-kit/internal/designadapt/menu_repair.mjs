// Experimental P07 companion, NOT part of immutable v1 exports. ADR-0050.
// Elements still owns selection, keyboard input, dismissal and action semantics.
const mounted = new WeakMap();
const property = '--appkit-menu-available-block';
const marker = 'data-appkit-menu-repair';

export function availableMenuBlockSize(anchor, viewport, gap = 8, edge = 16) {
  const values = [anchor.top, anchor.bottom, viewport.top, viewport.height, gap, edge];
  if (values.some(v => !Number.isFinite(v)) || anchor.bottom < anchor.top ||
      viewport.height < 0 || gap < 0 || edge < 0) throw new RangeError('invalid menu geometry');
  const above = anchor.top - viewport.top - gap - edge;
  const below = viewport.top + viewport.height - anchor.bottom - gap - edge;
  // Let Elements choose above/below. Its size observer can reposition after this cap.
  return Math.floor(Math.max(0, Math.min(viewport.height - edge * 2, Math.max(above, below))));
}

export function mountMenuRepair(root, {gap = 8, edge = 16} = {}) {
  if (!root?.matches?.('[data-appkit-menu]') || !root.isConnected || mounted.has(root)) {
    throw new TypeError('one connected, unmounted menu wrapper required');
  }
  availableMenuBlockSize({top: 0, bottom: 0}, {top: 0, height: 0}, gap, edge);
  const doc = root.ownerDocument, win = doc?.defaultView;
  const dropdowns = root.querySelectorAll('el-dropdown');
  if (dropdowns.length !== 1) throw new TypeError('one dropdown required');
  const dropdown = dropdowns[0];
  const trigger = dropdown.querySelector(':scope > button');
  const menu = dropdown.querySelector(':scope > el-menu');
  if (!trigger || !menu || !win?.ResizeObserver || !win?.MutationObserver || !win?.requestAnimationFrame || !win?.queueMicrotask) {
    throw new TypeError('P07 structure and browser observers required');
  }
  const originalValue = menu.style.getPropertyValue(property);
  const originalPriority = menu.style.getPropertyPriority(property);
  const originalMarker = root.getAttribute(marker);
  const listeners = [];
  let disposed = false, frame = null, focusPending = false;
  const restore = () => {
    if (originalValue) menu.style.setProperty(property, originalValue, originalPriority);
    else menu.style.removeProperty(property);
    if (originalMarker === null) root.removeAttribute(marker);
    else root.setAttribute(marker, originalMarker);
  };
  const update = () => {
    frame = null;
    if (disposed || !root.isConnected) return;
    const viewport = win.visualViewport;
    const top = viewport?.offsetTop ?? 0, height = viewport?.height ?? win.innerHeight;
    const size = availableMenuBlockSize(trigger.getBoundingClientRect(), {top, height}, gap, edge);
    const value = size + 'px';
    if (menu.style.getPropertyValue(property) !== value) menu.style.setProperty(property, value);
  };
  const schedule = () => {
    if (!disposed && frame === null) frame = win.requestAnimationFrame(update);
  };
  const reconcileFocus = event => {
    if (event.target !== menu || focusPending || disposed) return;
    focusPending = true;
    win.queueMicrotask(() => {
      focusPending = false;
      if (disposed || !root.isConnected || doc.activeElement !== menu || !menu.matches(':popover-open')) return;
      // Read the runtime's selected item; never choose or advance a selection.
      const selected = [...menu.querySelectorAll('a,button')].filter(item =>
        item.tabIndex === 0 && !item.disabled && item.getAttribute('aria-disabled') !== 'true');
      if (selected.length === 1) selected[0].focus({preventScroll: true});
    });
  };
  const listen = (target, type, fn, options) => {
    target.addEventListener(type, fn, options);
    listeners.push(() => target.removeEventListener(type, fn, options));
  };
  const observer = new win.ResizeObserver(schedule);
  // Follow the runtime's inline position updates, including position-only layout
  // shifts. Observing only this menu avoids a global subtree/layout observer.
  const positionObserver = new win.MutationObserver(schedule);
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    if (frame !== null) win.cancelAnimationFrame(frame);
    observer.disconnect();
    positionObserver.disconnect();
    for (const remove of listeners) remove();
    restore();
    mounted.delete(root);
  };
  try {
    root.setAttribute(marker, 'active');
    update();
    observer.observe(trigger); observer.observe(menu);
    positionObserver.observe(menu, {attributes: true, attributeFilter: ['style']});
    listen(root, 'focusin', reconcileFocus);
    listen(menu, 'beforetoggle', schedule);
    listen(menu, 'toggle', schedule);
    listen(win, 'resize', schedule, {passive: true});
    listen(doc, 'scroll', schedule, {capture: true, passive: true});
    if (win.visualViewport) {
      listen(win.visualViewport, 'resize', schedule, {passive: true});
      listen(win.visualViewport, 'scroll', schedule, {passive: true});
    }
    mounted.set(root, dispose);
    return dispose;
  } catch (error) {dispose(); throw error;}
}
