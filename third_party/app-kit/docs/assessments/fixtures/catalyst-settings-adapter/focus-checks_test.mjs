import test from 'node:test';
import assert from 'node:assert/strict';
import {contrast, visibleOutline, outlinePixels} from './focus-checks.mjs';
const fact = () => ({matches: true, visible: true, style: 'solid', width: 2, offset: 2,
  color: [0, 0, 0, 1], background: [1, 1, 1, 1]});
test('measured opaque sRGB contrast retains thresholds without rounding', () => {
  assert.equal(contrast([0, 0, 0, 1], [1, 1, 1, 1]), 21);
  assert.equal(contrast([0.5, 0.5, 0.5, 1], [0.5, 0.5, 0.5, 1]), 1);
  assert.ok(contrast([0.47, 0.47, 0.47, 1], [1, 1, 1, 1]) < 4.5);
  for (const bad of [[0, 0, 0, 0.5], [NaN, 0, 0, 1], [2, 0, 0, 1], [0, 0, 0]]) {
    assert.throws(() => contrast(bad, [1, 1, 1, 1]));
  }
});
test('focus-visible match alone cannot prove a painted outline', () => {
  assert.equal(visibleOutline(fact()), true);
  for (const mutation of [{style: 'none'}, {style: 'hidden'}, {width: 0}, {width: 1}, {width: NaN},
    {offset: 0}, {offset: NaN}, {matches: false}, {visible: false}, {color: [1, 1, 1, 1]}, {color: [0, 0, 0, 0]}]) {
    assert.equal(visibleOutline({...fact(), ...mutation}), false, JSON.stringify(mutation));
  }
});
test('outline pixels require all sides, expected ink and local adjacent contrast', () => {
  const rect = {left: 20, top: 20, right: 80, bottom: 80};
  const image = {width: 100, height: 100, data: new Uint8Array(100 * 100 * 4).fill(255)};
  assert.equal(outlinePixels(image, rect, 2, 2, [0, 0, 0, 1]).ok, false);
  const set = (x, y, value) => { const i = (y * image.width + x) * 4; image.data[i] = image.data[i + 1] = image.data[i + 2] = value; };
  for (let t = 16; t <= 84; t++) for (const edge of [16, 17, 82, 83]) { set(edge, t, 0); set(t, edge, 0); }
  assert.equal(outlinePixels(image, rect, 2, 2, [0, 0, 0, 1]).ok, true);
  set(50, 83, 255); // one occluded sampled segment must fail
  assert.equal(outlinePixels(image, rect, 2, 2, [0, 0, 0, 1]).ok, false);
  set(50, 83, 0); set(50, 86, 0); // local dark backdrop defeats black outline
  assert.equal(outlinePixels(image, rect, 2, 2, [0, 0, 0, 1]).ok, false);
  assert.throws(() => outlinePixels(image, {...rect, top: 1}, 2, 2, [0, 0, 0, 1]), /outside capture/);
});
