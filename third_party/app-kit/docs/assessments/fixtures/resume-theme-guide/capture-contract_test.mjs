import test from 'node:test';
import assert from 'node:assert/strict';
import { inspectSynthwaveCapture } from './capture-contract.mjs';

function solid(width, height, rgba) {
  const data = new Uint8Array(width * height * 4);
  for (let offset = 0; offset < data.length; offset += 4) data.set(rgba, offset);
  return { width, height, data };
}

test('bounded opaque dark specimen with visible light content passes', () => {
  const image = solid(20, 10, [100, 100, 100, 255]);
  for (let pixel = 0; pixel < 101; pixel++) image.data.set([20, 20, 20, 255], pixel * 4);
  image.data.set([220, 220, 220, 255], 101 * 4);
  assert.deepEqual(inspectSynthwaveCapture(image, 20, 10),
    { width: 20, height: 10, darkFraction: 101 / 200, lightFraction: 1 / 200 });
});

test('white and black blank captures fail opposite population canaries', () => {
  assert.throws(() => inspectSynthwaveCapture(solid(10, 10, [255, 255, 255, 255]), 10, 10));
  assert.throws(() => inspectSynthwaveCapture(solid(10, 10, [0, 0, 0, 255]), 10, 10));
});

test('partial alpha is refused even when color populations would pass', () => {
  const image = solid(10, 10, [0, 0, 0, 255]);
  image.data.set([255, 255, 255, 255], 0);
  image.data[7] = 254;
  assert.throws(() => inspectSynthwaveCapture(image, 10, 10));
});

test('malformed, truncated, extra and mismatched RGBA captures fail', () => {
  const valid = solid(2, 2, [0, 0, 0, 255]);
  for (const [image, width, height] of [
    [null, 2, 2],
    [{ width: 2, height: 2, data: [] }, 2, 2],
    [{ width: 2, height: 2, data: new Uint8Array(15) }, 2, 2],
    [{ width: 2, height: 2, data: new Uint8Array(17) }, 2, 2],
    [{ ...valid, width: 2.5 }, 2, 2],
    [valid, 1, 2],
    [valid, NaN, 2],
    [valid, 2, 0],
  ]) assert.throws(() => inspectSynthwaveCapture(image, width, height));
});

test('capture dimensions have exact inclusive 1440 by 2000 bounds', () => {
  assert.throws(() => inspectSynthwaveCapture({ width: 1441, height: 1, data: new Uint8Array() }, 1441, 1));
  assert.throws(() => inspectSynthwaveCapture({ width: 1, height: 2001, data: new Uint8Array() }, 1, 2001));
  const image = solid(1440, 2000, [0, 0, 0, 255]);
  for (let pixel = 0; pixel < 14400; pixel++) image.data.set([255, 255, 255, 255], pixel * 4);
  assert.doesNotThrow(() => inspectSynthwaveCapture(image, 1440, 2000));
});

test('dark threshold is strict while the half-percent light threshold is inclusive', () => {
  const image = solid(20, 10, [100, 100, 100, 255]);
  for (let pixel = 0; pixel < 100; pixel++) image.data.set([0, 0, 0, 255], pixel * 4);
  image.data.set([255, 255, 255, 255], 100 * 4);
  assert.throws(() => inspectSynthwaveCapture(image, 20, 10));
});
