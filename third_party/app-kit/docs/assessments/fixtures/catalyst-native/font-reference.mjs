// Read-only checks for one explicitly prepared private Catalyst font reference.
// This does not load Next, execute fonts, authenticate an archive, or certify UI.
import { createHash } from 'node:crypto';
import { closeSync, constants, fstatSync, lstatSync, openSync, readdirSync, readSync, realpathSync } from 'node:fs';
import { dirname, isAbsolute, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const metadataHash = 'dc7298020ee8a2e2502e7e038e8941c98123a03db4ffc8f52b6ff7fdc41f4655';
const cssHash = '46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441';
const publicPath = `/__appkit_reference_fonts/${cssHash}/`;
const sourcePins = [
  ['package.json', '04f5b915bdd1b1ef041291a7a30b4a2af1e0edb43156ab876b2f27ae72c56c53'],
  ['package-lock.json', '3a9469d59cb2c862c35735c5153f43b7ddf27c9f30c867cf64838df1f9b66b3c'],
  ['src/app/(app)/settings/page.tsx', '1ca7f4e14738e917390815dc38d48047588eb324090d337f5c718b9d2ec6fa21'],
  ['src/app/layout.tsx', 'd69e6ed489e1d2b48bf6c1a9b0c1761abd8eef2e299eec4feb482730512db0b1'],
  ['src/styles/tailwind.css', '64b1a37f5d699d33ab9ed4f88f4a41c486accf25c9760b8ed4750134541cd2c4'],
];
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
function requireValue(ok, message) { if (!ok) throw new Error(message); }

function safeRelative(path) {
  requireValue(typeof path === 'string' && path.length > 0 && path.length <= 512 &&
    !isAbsolute(path) && !/[\\\x00-\x20\x7f%?#:]/.test(path) &&
    path.split('/').every(part => part && part !== '.' && part !== '..'), 'Unsafe reference metadata path.');
  return path;
}

function selectedPath(root, relative, directory = false) {
  const parts = safeRelative(relative).split('/');
  let path = root;
  for (let i = 0; i < parts.length; i++) {
    path = join(path, parts[i]);
    const stat = lstatSync(path);
    requireValue(!stat.isSymbolicLink(), `Reference symlink refused: ${relative}`);
    requireValue(i < parts.length - 1 || directory ? stat.isDirectory() : stat.isFile(), `Reference is not a regular ${directory ? 'directory' : 'file'}: ${relative}`);
  }
  return path;
}

function readRegular(root, relative, maxBytes) {
  const path = selectedPath(root, relative);
  // A checked regular file may become a FIFO before open; never wait for a
  // writer. The descriptor check below still rejects every non-regular file.
  const fd = openSync(path, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const stat = fstatSync(fd);
    requireValue(stat.isFile() && stat.size <= maxBytes, `Reference file exceeds bounds: ${relative}`);
    // A changing file cannot grow an unbounded allocation during this check.
    const bytes = Buffer.alloc(stat.size + 1);
    let count = 0, n;
    while (count < bytes.length && (n = readSync(fd, bytes, count, bytes.length - count, null)) > 0) count += n;
    requireValue(count === stat.size, `Reference file changed while reading: ${relative}`);
    return bytes.subarray(0, count);
  } finally { closeSync(fd); }
}

function exactNames(directory, expected) {
  requireValue(JSON.stringify(readdirSync(directory).sort()) === JSON.stringify([...expected].sort()), 'Reference bundle closure differs from the pinned file set.');
}

export function verifyFontReference(project) {
  requireValue(typeof project === 'string' && isAbsolute(project) && !/[\x00-\x1f\x7f]/.test(project), 'Select an absolute prepared Catalyst project directory.');
  const projectStat = lstatSync(project);
  requireValue(projectStat.isDirectory() && !projectStat.isSymbolicLink(), 'Selected project must be a regular directory, not a symlink.');
  const root = resolve(project);
  // Check the fixed metadata byte pin BEFORE parsing any supplied paths.
  const metadataBytes = readRegular(dirname(fileURLToPath(import.meta.url)), 'font-assets.json', 64 * 1024);
  requireValue(hash(metadataBytes) === metadataHash, 'Pinned font metadata hash mismatch.');
  const metadata = JSON.parse(metadataBytes);
  requireValue(metadata.schema_version === 1 && metadata.font_count === 38 && metadata.fonts.length === 38, 'Unexpected pinned font inventory.');
  requireValue(metadata.source_css.path === 'inter.css' && metadata.source_css.sha256 === cssHash, 'Unexpected source stylesheet identity.');

  for (const [path, expected] of sourcePins) {
    const bytes = readRegular(root, path, 2 * 1024 * 1024);
    requireValue(hash(bytes) === expected, `Reviewed source hash mismatch: ${path}`);
  }
  const bundleRelative = `public${publicPath.slice(0, -1)}`;
  const bundle = selectedPath(root, bundleRelative, true);
  exactNames(bundle, ['inter.css', 'LICENSE.txt', 'font-files']);
  const fontNames = metadata.fonts.map(font => {
    requireValue(/^font-files\/[A-Za-z0-9-]+\.woff2$/.test(font.path), 'Unsafe reference metadata font path.');
    requireValue(font.url === `https://rsms.me/inter/${font.path}?v=4.1`, 'Unexpected upstream font identity.');
    return font.path.slice('font-files/'.length);
  });
  requireValue(new Set(fontNames).size === 38, 'Duplicate font inventory entry.');
  exactNames(selectedPath(bundle, 'font-files', true), fontNames);
  let css, totalBytes = 0, fontBytes = 0;
  const files = [metadata.source_css, metadata.license, ...metadata.fonts].map(record => {
    safeRelative(record.path);
    requireValue(Number.isSafeInteger(record.size) && record.size > 0 && record.size <= 2 * 1024 * 1024 && /^[0-9a-f]{64}$/.test(record.sha256), 'Invalid reference file metadata.');
    const bytes = readRegular(bundle, record.path, record.size);
    requireValue(bytes.length === record.size, `Reference size mismatch: ${record.path}`);
    if (record.path.endsWith('.woff2')) {
      requireValue(bytes.length >= 48 && bytes.toString('ascii', 0, 4) === 'wOF2' && bytes.readUInt32BE(8) === bytes.length, `Invalid WOFF2 header/length: ${record.path}`);
      fontBytes += bytes.length;
    }
    requireValue(hash(bytes) === record.sha256, `Reference hash mismatch: ${record.path}`);
    if (record.path === 'inter.css') css = bytes.toString('utf8');
    totalBytes += bytes.length;
    return { path: record.path, url: `${publicPath}${record.path}${record.path.endsWith('.woff2') ? '?v=4.1' : ''}`, sha256: record.sha256, size: bytes.length };
  });
  requireValue(totalBytes === metadata.bundle_bytes && fontBytes === metadata.font_bytes, 'Font bundle aggregate differs from metadata.');
  // This is a parser for the exact pinned stylesheet, not arbitrary CSS.
  const cssURLs = [...css.matchAll(/url\(\s*(['"])([^'"]+)\1\s*\)/g)].map(match => match[2]);
  const expectedURLs = metadata.fonts.map(font => `${font.path}?v=4.1`).sort();
  requireValue(cssURLs.length === 40 && JSON.stringify([...new Set(cssURLs)].sort()) === JSON.stringify(expectedURLs), 'Stylesheet font dependency closure mismatch.');
  requireValue(!/@import\b/i.test(css), 'Unexpected stylesheet import.');
  return { metadata_sha256: metadataHash, stylesheet: `${publicPath}inter.css`, font_count: 38, font_bytes: fontBytes, bundle_bytes: totalBytes, files,
    scope: 'Pinned selected-source and local font-byte checks only; no browser, font-decoding, visual or whole-application offline proof.' };
}

if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
  if (process.argv.length !== 3) {
    console.error('Usage: node font-reference.mjs ABSOLUTE_PREPARED_PROJECT');
    process.exitCode = 2;
  } else {
    try { console.log(JSON.stringify(verifyFontReference(process.argv[2]))); }
    catch (error) { console.error(`Font reference refused: ${error.message}`); process.exitCode = 1; }
  }
}
