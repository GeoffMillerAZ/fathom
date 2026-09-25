import assert from 'node:assert/strict';
import test from 'node:test';
import {makeFontCSS, subsetRanges} from './font-delivery.mjs';

const sourceFace = '@font-face{font-family:InterVariable;font-style:normal;font-weight:100 900;font-display:block;src:url(/assets/inter.woff2)format("woff2")}';
const parseFaceRanges = css => [...css.matchAll(/@font-face\{([^}]*)\}/g)].map(match => {
  const descriptor = match[1];
  const value = descriptor.match(/unicode-range:([^;}]+)/)?.[1];
  return value ? value.split(',').map(part => part.slice(2).split('-').map(hex => Number.parseInt(hex, 16))) : [];
});

test('fixed ranges are inclusive and emitted unchanged with descriptor-preserving URLs', () => {
  assert.deepEqual(subsetRanges, [[0, 0x024f], [0x0300, 0x036f], [0x2000, 0x206f], [0x20a0, 0x20cf], [0x2190, 0x21ff]]);
  const before = '/* keep */.x{color:rebeccapurple}' + sourceFace + '.y{display:block}';
  const after = makeFontCSS(before);
  assert.equal(after, '/* keep */.x{color:rebeccapurple}' + sourceFace +
    '@font-face{font-family:InterVariable;font-style:normal;font-weight:100 900;font-display:block;src:url(/assets/inter-subset.woff2)format("woff2");unicode-range:U+0000-024F,U+0300-036F,U+2000-206F,U+20A0-20CF,U+2190-21FF}' +
    '.y{display:block}');
});

test('unrestricted original stays first; subset overlaps exactly its selected scalars', () => {
  const faces = parseFaceRanges(makeFontCSS(sourceFace));
  assert.equal(faces.length, 2);
  assert.deepEqual(faces[0], [], 'original unicode coverage must never be restricted');
  assert.deepEqual(faces[1], subsetRanges, 'only the last subset face restricts coverage');
  for (let codepoint = 0; codepoint <= 0x10ffff; codepoint++) {
    if (codepoint >= 0xd800 && codepoint <= 0xdfff) continue;
    const matches = faces.filter(ranges => ranges.length === 0 || ranges.some(([start, end]) => codepoint >= start && codepoint <= end));
    const selected = subsetRanges.some(([start, end]) => codepoint >= start && codepoint <= end);
    assert.equal(matches.length, selected ? 2 : 1, `scalar U+${codepoint.toString(16)}: original always eligible, selected-only overlap`);
  }
  for (let codepoint = 0xd800; codepoint <= 0xdfff; codepoint++) {
    assert.equal(faces[1].some(([start, end]) => codepoint >= start && codepoint <= end), false);
  }
});

test('missing, duplicate, malformed, or descriptor-drifted Inter faces refuse', () => {
  assert.throws(() => makeFontCSS('.x{}'), /exactly one/);
  assert.throws(() => makeFontCSS(sourceFace + sourceFace), /exactly one/);
  assert.throws(() => makeFontCSS('@font-face{font-family:InterVariable;font-style:normal}'), /exactly one|descriptors/);
  assert.throws(() => makeFontCSS(sourceFace.replace('font-weight:100 900', 'font-weight:400')), /descriptors/);
  assert.throws(() => makeFontCSS(sourceFace.replace('/assets/inter.woff2', '/assets/other.woff2')), /descriptors/);
  assert.throws(() => makeFontCSS(Buffer.from(sourceFace)), /string/);
});
