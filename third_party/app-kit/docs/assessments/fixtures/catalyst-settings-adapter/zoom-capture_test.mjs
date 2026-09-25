import test from 'node:test';
import assert from 'node:assert/strict';
import {crc32, deflateSync} from 'node:zlib';
import {captureGeometry, verifyCapturePNG} from './zoom-capture.mjs';

const metrics = () => ({contentSize: {x: 0, y: 0, width: 1440, height: 2484},
  cssContentSize: {x: 0, y: 0, width: 720, height: 1242}});
const bounds = () => ({form: {left: 32, top: 32, right: 688, bottom: 1210},
  lastButton: {left: 546, top: 1166, right: 688, bottom: 1210}, lastButtonName: 'Validate changes'});
const chunk = (type, data) => {
  const bytes = Buffer.alloc(data.length + 12);
  bytes.writeUInt32BE(data.length, 0); bytes.write(type, 4); data.copy(bytes, 8);
  bytes.writeUInt32BE(crc32(bytes.subarray(4, bytes.length - 4)), bytes.length - 4);
  return bytes;
};
const png = (width, height, pixels = Buffer.alloc((width * 3 + 1) * height)) => {
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0); header.writeUInt32BE(height, 4); header[8] = 8; header[9] = 2;
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', header), chunk('IDAT', deflateSync(pixels)), chunk('IEND', Buffer.alloc(0))]);
};

test('actual zoom capture maps complete form and final action into full DIP document', () => {
  const geometry = captureGeometry(metrics(), bounds());
  assert.deepEqual(geometry.clip, {x: 0, y: 0, width: 1440, height: 2484, scale: 1});
  assert.equal(geometry.mapped.lastButton.bottom, 2420);
  assert.deepEqual(verifyCapturePNG(png(1440, 2484), geometry), {width: 1440, height: 2484});
});
test('historical locator crop and viewport-only capture both fail full-frame oracle', () => {
  const geometry = captureGeometry(metrics(), bounds());
  for (const dimensions of [[656, 1178], [1440, 1200], [720, 1242], [1440, 2483]]) {
    assert.throws(() => verifyCapturePNG(png(...dimensions), geometry), /full DIP document/);
  }
});
test('missing, mismatched, non-zoom and unbounded metrics fail closed', () => {
  for (const mutate of [m => { delete m.contentSize; }, m => { m.contentSize.width = 720; },
    m => { m.contentSize.height = 1242; }, m => { m.cssContentSize.width = 0; },
    m => { m.contentSize.x = 4; }, m => { m.contentSize.height = Infinity; },
    m => { m.contentSize.height = 40_000; m.cssContentSize.height = 20_000; }]) {
    const value = metrics(); mutate(value);
    assert.throws(() => captureGeometry(value, bounds()));
  }
});
test('clipped, missing or misidentified final actions cannot pass', () => {
  for (const mutate of [b => { b.form.right = 721; }, b => { b.form.top = -1; },
    b => { b.lastButton.bottom = 1243; }, b => { b.lastButton.top = 0; },
    b => { b.lastButton.right = b.lastButton.left; }, b => { b.lastButton = null; },
    b => { b.lastButtonName = 'Reset'; }, b => { b.form.bottom = NaN; }]) {
    const value = bounds(); mutate(value);
    assert.throws(() => captureGeometry(metrics(), value));
  }
});
test('invalid or truncated PNG header is refused', () => {
  const geometry = captureGeometry(metrics(), bounds());
  assert.throws(() => verifyCapturePNG(Buffer.alloc(24), geometry), /PNG header/);
  const bytes = png(1440, 2484); bytes[0] = 0;
  assert.throws(() => verifyCapturePNG(bytes, geometry), /PNG header/);
});
test('matching dimensions never admit truncated or corrupt image data', () => {
  const geometry = captureGeometry(metrics(), bounds());
  const bytes = png(1440, 2484);
  for (const value of [bytes.subarray(0, 33), bytes.subarray(0, -12), bytes.subarray(0, -1),
    Buffer.concat([bytes, Buffer.from([0])])]) assert.throws(() => verifyCapturePNG(value, geometry));
  const corrupt = Buffer.from(bytes); corrupt[corrupt.length - 5] ^= 1;
  assert.throws(() => verifyCapturePNG(corrupt, geometry), /CRC mismatch/);
  assert.throws(() => verifyCapturePNG(png(1440, 2484, Buffer.alloc(1)), geometry), /pixel rows/);
  const filters = Buffer.alloc((1440 * 3 + 1) * 2484); filters[0] = 5;
  assert.throws(() => verifyCapturePNG(png(1440, 2484, filters), geometry), /row filter/);
});
