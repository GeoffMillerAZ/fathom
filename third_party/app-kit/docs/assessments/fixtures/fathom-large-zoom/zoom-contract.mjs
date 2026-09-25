// This workload's native Chrome zoom oracle, including non-overlay scrollbars.
// Appearance selection is observed separately through the Chrome settings control.
export function inspectZoom(appearance, exposed, metrics) {
  if (!['1', '2'].includes(appearance) || !exposed || !metrics) return false;
  const factor = Number(appearance);
  const physical = metrics.contentSize?.width, css = metrics.cssContentSize?.width;
  return [physical, css, exposed.scrollWidth].every(value => Number.isSafeInteger(value) && value > 0) &&
    exposed.dpr === factor && exposed.inner === 1440 / factor && exposed.outer === 1440 &&
    exposed.scale === 1 && css === exposed.scrollWidth && css <= exposed.inner &&
    physical <= exposed.outer && physical >= css * factor && physical < (css + 1) * factor;
}
