// A bounded nonblank canary for the selected dark synthwave specimen capture.
// This checks decoded pixels, not visual fidelity or accessibility.
export function inspectSynthwaveCapture(image, expectedWidth, expectedHeight) {
  if (!Number.isInteger(expectedWidth) || !Number.isInteger(expectedHeight) ||
      expectedWidth <= 0 || expectedHeight <= 0 || expectedWidth > 1440 || expectedHeight > 2000 ||
      !image || !Number.isInteger(image.width) || !Number.isInteger(image.height) ||
      image.width !== expectedWidth || image.height !== expectedHeight) {
    throw Error('Invalid capture dimensions');
  }
  const data = image.data;
  const pixels = image.width * image.height;
  if (!ArrayBuffer.isView(data) || data.BYTES_PER_ELEMENT !== 1 ||
      data.length !== pixels * 4 || data.byteLength !== pixels * 4) {
    throw Error('Invalid decoded RGBA bytes');
  }

  let dark = 0, light = 0;
  for (let offset = 0; offset < data.length; offset += 4) {
    const red = data[offset], green = data[offset + 1], blue = data[offset + 2];
    if (data[offset + 3] !== 255) throw Error('Capture is not fully opaque');
    if (red + green + blue < 210) dark++;
    if (red >= 180 && green >= 180 && blue >= 180) light++;
  }
  const darkFraction = dark / pixels;
  const lightFraction = light / pixels;
  if (darkFraction <= 0.5 || lightFraction < 0.005) {
    throw Error('Capture lacks the expected dark and light pixel populations');
  }
  return { width: image.width, height: image.height, darkFraction, lightFraction };
}
