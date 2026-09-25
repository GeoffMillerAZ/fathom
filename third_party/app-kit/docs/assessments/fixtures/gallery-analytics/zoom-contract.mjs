// Pinned Chrome 200% capture contract, not a visual-difference tolerance.
export function zoomCaptureClip(metrics) {
  const dip = metrics?.contentSize, css = metrics?.cssContentSize;
  for (const rect of [dip, css]) {
    if (!rect || !['x', 'y', 'width', 'height'].every(k => Number.isFinite(rect[k])) ||
        rect.x !== 0 || rect.y !== 0 || rect.height <= 0) throw Error('Invalid full-document metrics');
  }
  // Chrome reports integer CSS and DIP heights independently (observed 2025/4051).
  // Capture the complete DIP rectangle exactly; never shrink it to CSS height * 2.
  if (dip.width !== 1440 || css.width !== 720 || !Number.isInteger(dip.height) ||
      dip.height > 20000 || Math.abs(dip.height - css.height * 2) > 1) {
    throw Error('Unexpected actual-zoom full-document geometry');
  }
  return {...dip, scale: 1};
}

// Convert one bounded document-relative CSS box into Chrome's 200% DIP space.
// This deliberately does not admit a full tall editor capture: callers select
// the reviewed element in CSS coordinates, while this helper pins the actual
// browser zoom relationship before constructing the CDP clip.
export function zoomElementCaptureClip(metrics, cssBox) {
  zoomCaptureClip(metrics);
  const css = metrics.cssContentSize;
  if (!cssBox || !['x', 'y', 'width', 'height'].every(k => Number.isFinite(cssBox[k])) ||
      cssBox.x < 0 || cssBox.y < 0 || cssBox.width <= 0 || cssBox.height <= 0 ||
      cssBox.width > 720 || cssBox.height > 1000 ||
      cssBox.x + cssBox.width > css.width || cssBox.y + cssBox.height > css.height) {
    throw Error('Invalid bounded element capture geometry');
  }
  return {
    x: cssBox.x * 2,
    y: cssBox.y * 2,
    width: cssBox.width * 2,
    height: cssBox.height * 2,
    scale: 1,
  };
}

// The caller owns normal teardown; setup must not strand a newly created browser.
export async function prepareContext(context, setup) {
  try { return await setup(); }
  catch (error) {
    try { await context.close(); }
    catch (cleanupError) { throw new AggregateError([error, cleanupError], 'Browser setup and cleanup failed'); }
    throw error;
  }
}
