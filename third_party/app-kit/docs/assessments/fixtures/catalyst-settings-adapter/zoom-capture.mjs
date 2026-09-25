// Chrome-152-specific actual-browser-zoom capture. CDP's legacy contentSize is
// measured in DIP; cssContentSize and DOM rectangles are CSS pixels. Do not feed
// a CSS rectangle directly into Page.captureScreenshot at browser zoom 200%.
import {crc32, inflateSync} from 'node:zlib';
const requireFact = (ok, message) => { if (!ok) throw Error(message); };
const near = (a, b) => Number.isFinite(a) && Number.isFinite(b) && Math.abs(a - b) < 0.01;

export function captureGeometry(metrics, bounds) {
  const dip = metrics.contentSize;
  const css = metrics.cssContentSize;
  for (const rect of [dip, css]) {
    requireFact(rect && ['x', 'y', 'width', 'height'].every(key => Number.isFinite(rect[key])) &&
      rect.x === 0 && rect.y === 0 && rect.width > 0 && rect.height > 0, 'Invalid document metrics');
  }
  const ratio = {x: dip.width / css.width, y: dip.height / css.height};
  requireFact(near(ratio.x, 2) && near(ratio.y, 2) && near(ratio.x, ratio.y), 'Expected actual 200% CSS-to-DIP ratio');
  requireFact(dip.width === 1440 && css.width === 720 && dip.height <= 20_000, 'Unexpected or unbounded capture dimensions');
  const mapped = {};
  for (const name of ['form', 'lastButton']) {
    const rect = bounds[name];
    requireFact(rect && ['left', 'top', 'right', 'bottom'].every(key => Number.isFinite(rect[key])) &&
      rect.right > rect.left && rect.bottom > rect.top, 'Missing or blank ' + name);
    mapped[name] = {left: rect.left * ratio.x, top: rect.top * ratio.y,
      right: rect.right * ratio.x, bottom: rect.bottom * ratio.y};
    requireFact(mapped[name].left >= 0 && mapped[name].top >= 0 && mapped[name].right <= dip.width &&
      mapped[name].bottom <= dip.height, 'Capture clips ' + name);
  }
  requireFact(bounds.lastButtonName === 'Validate changes', 'Expected the final validation action');
  const form = mapped.form, button = mapped.lastButton;
  requireFact(button.left >= form.left && button.right <= form.right && button.top >= form.top &&
    button.bottom <= form.bottom, 'Final action lies outside form');
  return {metrics, bounds, ratio, mapped, clip: {...dip, scale: 1}};
}

export function verifyCapturePNG(bytes, geometry) {
  requireFact(Buffer.isBuffer(bytes) && bytes.length >= 33 &&
    bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])) &&
    bytes.readUInt32BE(8) === 13 && bytes.toString('ascii', 12, 16) === 'IHDR', 'Invalid PNG header');
  const png = {width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20)};
  requireFact(png.width === geometry.clip.width && png.height === geometry.clip.height,
    'PNG dimensions do not cover full DIP document');
  // Intentionally bounded to the measured Chrome RGB/RGBA non-interlaced output,
  // not a general PNG decoder. A header alone cannot prove complete image data.
  requireFact(png.width === 1440 && png.height > 0 && png.height <= 20_000 &&
    bytes[24] === 8 && [2, 6].includes(bytes[25]) && bytes[26] === 0 &&
    bytes[27] === 0 && bytes[28] === 0, 'Unsupported PNG encoding or dimensions');
  const compressed = [];
  let offset = 8, ended = false;
  while (offset < bytes.length) {
    requireFact(offset + 12 <= bytes.length, 'Truncated PNG chunk');
    const length = bytes.readUInt32BE(offset);
    const end = offset + length + 12;
    requireFact(end <= bytes.length, 'Truncated PNG chunk data');
    const type = bytes.toString('ascii', offset + 4, offset + 8);
    requireFact(crc32(bytes.subarray(offset + 4, end - 4)) === bytes.readUInt32BE(end - 4), 'PNG CRC mismatch');
    if (offset === 8) requireFact(type === 'IHDR', 'Missing first PNG IHDR');
    else if (type === 'IDAT') compressed.push(bytes.subarray(offset + 8, end - 4));
    else if (type === 'IEND') {
      requireFact(length === 0 && end === bytes.length && compressed.length > 0, 'Invalid PNG IEND');
      ended = true;
    } else requireFact(false, 'Unexpected Chrome PNG chunk');
    offset = end;
  }
  requireFact(ended, 'Incomplete PNG image');
  const rowBytes = png.width * (bytes[25] === 2 ? 3 : 4) + 1;
  const expectedBytes = rowBytes * png.height;
  const pixels = inflateSync(Buffer.concat(compressed), {maxOutputLength: expectedBytes});
  requireFact(pixels.length === expectedBytes, 'Incomplete PNG pixel rows');
  for (let row = 0; row < png.height; row++) requireFact(pixels[row * rowBytes] <= 4, 'Invalid PNG row filter');
  return png;
}

export async function captureZoomDocument(page, cdp) {
  const bounds = await page.locator('form[data-p10-settings]').evaluate(form => {
    const rect = element => {
      const box = element.getBoundingClientRect();
      return {left: box.left + scrollX, top: box.top + scrollY,
        right: box.right + scrollX, bottom: box.bottom + scrollY};
    };
    const button = [...form.querySelectorAll('button')].at(-1);
    return {form: rect(form), lastButton: button ? rect(button) : null,
      lastButtonName: button?.textContent.trim()};
  });
  const geometry = captureGeometry(await cdp.send('Page.getLayoutMetrics'), bounds);
  const shot = await cdp.send('Page.captureScreenshot', {format: 'png', fromSurface: true,
    captureBeyondViewport: true, clip: geometry.clip});
  const bytes = Buffer.from(shot.data, 'base64');
  return {bytes, evidence: {...geometry, png: verifyCapturePNG(bytes, geometry)}};
}
