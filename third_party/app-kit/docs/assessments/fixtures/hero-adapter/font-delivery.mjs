// Fixed prospective P01 Inter delivery split. This is deliberately not a
// general font-subsetting API: its one admitted source face is below.

const originalFace = '@font-face{font-family:InterVariable;font-style:normal;font-weight:100 900;font-display:block;src:url(/assets/inter.woff2)format("woff2")}';
const scalarMax = 0x10ffff;
const surrogateStart = 0xd800;
const surrogateEnd = 0xdfff;

export const subsetRanges = Object.freeze([
  Object.freeze([0x0000, 0x024f]),
  Object.freeze([0x0300, 0x036f]),
  Object.freeze([0x2000, 0x206f]),
  Object.freeze([0x20a0, 0x20cf]),
  Object.freeze([0x2190, 0x21ff]),
]);

function assertValidRanges(ranges) {
  let previousEnd = -1;
  for (const range of ranges) {
    if (!Array.isArray(range) || range.length !== 2 || !Number.isInteger(range[0]) || !Number.isInteger(range[1])) {
      throw Error('fixed subset ranges must be inclusive integer pairs');
    }
    const [start, end] = range;
    if (start < 0 || end > scalarMax || start > end || start <= previousEnd || (start <= surrogateEnd && end >= surrogateStart)) {
      throw Error('fixed subset ranges must be ordered non-surrogate scalar intervals');
    }
    previousEnd = end;
  }
}

function rangeCSS(ranges) {
  return ranges.map(([start, end]) => `U+${start.toString(16).toUpperCase().padStart(4, '0')}-${end.toString(16).toUpperCase().padStart(4, '0')}`).join(',');
}

function interFaceBlocks(css) {
  return [...css.matchAll(/@font-face\{[^{}]*\}/g)].filter(match => match[0].includes('font-family:InterVariable'));
}

assertValidRanges(subsetRanges);
const subsetFace = '@font-face{font-family:InterVariable;font-style:normal;font-weight:100 900;font-display:block;src:url(/assets/inter-subset.woff2)format("woff2");unicode-range:' + rangeCSS(subsetRanges) + '}';

// Keep the original face fully eligible for whole-cluster fallback, then give
// the selected subset priority. A disjoint complement caused platform-font
// fallback for an original-supported IPA base + combining acute. This overlap
// must still earn exact browser pixels and subset-only Latin delivery.
export function makeFontCSS(originalCSS) {
  if (typeof originalCSS !== 'string') throw TypeError('original CSS must be a string');
  const faces = interFaceBlocks(originalCSS);
  const interMarkers = originalCSS.match(/font-family:InterVariable/g) ?? [];
  if (faces.length !== 1 || interMarkers.length !== 1) throw Error('expected exactly one InterVariable font face');
  const face = faces[0];
  if (face[0] !== originalFace) throw Error('InterVariable face descriptors differ from the frozen source face');
  return originalCSS.slice(0, face.index) + originalFace + subsetFace + originalCSS.slice(face.index + originalFace.length);
}
