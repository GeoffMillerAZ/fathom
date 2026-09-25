const channel = value => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
const luminance = rgba => rgba.slice(0, 3).map(channel).reduce((sum, value, i) => sum + value * [0.2126, 0.7152, 0.0722][i], 0);
const opaque = rgba => Array.isArray(rgba) && rgba.length === 4 && rgba.every(v => Number.isFinite(v) && v >= 0 && v <= 1) && rgba[3] === 1;
export function contrast(foreground, background) {
  if (!opaque(foreground) || !opaque(background)) throw Error('Opaque finite sRGB samples required');
  const a = luminance(foreground), b = luminance(background);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}
export function visibleOutline(fact) {
  if (!fact.matches || !fact.visible || !Number.isFinite(fact.width) || fact.width < 2 ||
    !Number.isFinite(fact.offset) || fact.offset < 2 || !['solid', 'double'].includes(fact.style)) return false;
  try { return contrast(fact.color, fact.background) >= 3; } catch { return false; }
}

// Inspect straight outline segments on every side, and the immediately adjacent
// outside pixels. This is a bounded screenshot oracle, not all-pixel conformance.
export function outlinePixels(image, rect, width, offset, expectedColor) {
  if (!image || !Number.isInteger(image.width) || !Number.isInteger(image.height) ||
    image.data?.length !== image.width * image.height * 4 || !opaque(expectedColor) ||
    ![rect.left, rect.top, rect.right, rect.bottom, width, offset].every(Number.isFinite) ||
    rect.right <= rect.left || rect.bottom <= rect.top || width < 2 || offset < 2) throw Error('Invalid outline pixel input');
  const rgba = (x, y) => {
    x = Math.floor(x); y = Math.floor(y);
    if (x < 0 || y < 0 || x >= image.width || y >= image.height) throw Error('Outline sample outside capture');
    return Array.from(image.data.subarray((y * image.width + x) * 4, (y * image.width + x) * 4 + 4), v => v / 255);
  };
  const samples = [], middle = offset + width / 2, outside = offset + width + 2;
  for (const t of [0.35, 0.5, 0.65]) {
    const x = rect.left + (rect.right - rect.left) * t, y = rect.top + (rect.bottom - rect.top) * t;
    for (const [side, ringPoint, backgroundPoint] of [
      ['left', [rect.left - middle, y], [rect.left - outside, y]],
      ['right', [rect.right + middle, y], [rect.right + outside, y]],
      ['top', [x, rect.top - middle], [x, rect.top - outside]],
      ['bottom', [x, rect.bottom + middle], [x, rect.bottom + outside]],
    ]) {
      const ring = rgba(...ringPoint), background = rgba(...backgroundPoint);
      const ratio = contrast(ring, background);
      samples.push({side, ringPoint, backgroundPoint, ring, background, ratio,
        ok: ratio >= 3 && ring.every((v, i) => Math.abs(v - expectedColor[i]) <= 0.02)});
    }
  }
  return {ok: samples.every(s => s.ok), samples};
}
