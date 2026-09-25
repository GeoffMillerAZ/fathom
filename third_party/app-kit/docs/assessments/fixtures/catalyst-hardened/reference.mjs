// One private, reviewed React derivative. Metadata is first-party provenance;
// paid source/patches stay private. These checks do not certify a Go port, a
// dependency installation, generated build reproducibility, or an app backend.
import { createHash } from 'node:crypto';
import { closeSync, constants, fstatSync, lstatSync, openSync, readdirSync, readSync, realpathSync } from 'node:fs';
import { dirname, isAbsolute, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { startReference } from '../catalyst-native/reference-server.mjs';

const manifestHash = 'f8de11343773dbb8dbeb53bb713d684fee23fcd36604ddff921b07da57450ee2';
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const requireValue = (condition, message) => { if (!condition) throw new Error(message); };
function safePath(path) {
  requireValue(typeof path === 'string' && path.length <= 512 && !isAbsolute(path) &&
    !/[\\\x00-\x20\x7f%?#:]/.test(path) && path.split('/').every(p => p && p !== '.' && p !== '..'), 'Unsafe manifest path.');
  return path;
}
function selectedPath(root, relative) {
  let selected = root;
  const parts = safePath(relative).split('/');
  for (const [i, part] of parts.entries()) {
    selected = join(selected, part);
    const stat = lstatSync(selected);
    requireValue(!stat.isSymbolicLink(), `Symlink refused: ${relative}`);
    if (i < parts.length - 1) requireValue(stat.isDirectory(), `Non-directory parent: ${relative}`);
  }
  return selected;
}
function readRegular(root, relative, maximum) {
  const fd = openSync(selectedPath(root, relative), constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const stat = fstatSync(fd);
    requireValue(stat.isFile() && stat.size <= maximum, `Not a bounded regular file: ${relative}`);
    const bytes = Buffer.alloc(stat.size + 1);
    let n, count = 0;
    while (count < bytes.length && (n = readSync(fd, bytes, count, bytes.length - count, null)) > 0) count += n;
    requireValue(count === stat.size, `File changed during read: ${relative}`);
    return bytes.subarray(0, count);
  } finally { closeSync(fd); }
}
function verify(project, original) {
  requireValue(typeof project === 'string' && isAbsolute(project) && !/[\x00-\x1f\x7f]/.test(project), 'Select an absolute prepared private project directory.');
  const stat = lstatSync(project);
  requireValue(stat.isDirectory() && !stat.isSymbolicLink(), 'Project must be a regular directory, not a symlink.');
  const root = resolve(project);
  const metadata = readRegular(dirname(fileURLToPath(import.meta.url)), 'source-manifest.json', 128 * 1024);
  requireValue(hash(metadata) === manifestHash, 'Pinned source manifest hash mismatch.');
  const manifest = JSON.parse(metadata);
  const records = original ? manifest.original_files : manifest.files;
  const expected = new Map(records.map(f => [f.path, f]));
  let count = 0, totalBytes = 0;
  const observed = [];
  function visit(relative) {
    requireValue(++count <= 1024, 'Source closure entry bound exceeded.');
    const selected = selectedPath(root, relative), stat = lstatSync(selected);
    if (stat.isDirectory()) {
      const children = readdirSync(selected);
      requireValue(children.length <= 256, 'Source directory entry bound exceeded.');
      for (const child of children.sort()) visit(`${relative}/${child}`);
      return;
    }
    requireValue(expected.has(relative), `Unexpected selected source file: ${relative}`);
    const record = expected.get(relative);
    const bytes = readRegular(root, relative, 16 * 1024 * 1024);
    requireValue(bytes.length === record.size && hash(bytes) === record.sha256, `Reviewed source mismatch: ${relative}`);
    totalBytes += bytes.length;
    requireValue(totalBytes <= 128 * 1024 * 1024, 'Source byte bound exceeded.');
    observed.push(relative);
  }
  for (const relative of manifest.roots) visit(relative);
  requireValue(JSON.stringify(observed.sort()) === JSON.stringify([...expected.keys()].sort()), 'Selected source closure is incomplete.');
  return { id: original ? 'catalyst-font-bound-base' : manifest.id, source_manifest_sha256: manifestHash,
    patch_sha256: manifest.patch_sha256, files: records, total_bytes: totalBytes, scope: manifest.scope };
}
export const verifyHardenedReference = project => verify(project, false);
export const verifyBaseReference = project => verify(project, true);

if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
  const [, , mode, project] = process.argv;
  if (process.argv.length !== 4 || !['--verify', '--verify-base', '--serve'].includes(mode)) {
    console.error('Usage: node reference.mjs --verify|--verify-base|--serve ABSOLUTE_PRIVATE_PROJECT'); process.exitCode = 2;
  } else {
    try {
      const proof = mode === '--verify-base' ? verifyBaseReference(project) : verifyHardenedReference(project);
      if (mode !== '--serve') console.log(JSON.stringify(proof));
      else {
        const server = await startReference(project);
        console.log(JSON.stringify({ origin: server.origin, id: proof.id, source_manifest_sha256: proof.source_manifest_sha256,
          scope: 'Private hardened React reference; no save backend or target certification. Generated build and dependencies are separately selected.' }));
        let closing = false;
        async function close() { if (closing) return; closing = true; await server.close(); }
        process.on('SIGINT', close); process.on('SIGTERM', close);
      }
    } catch (error) { console.error(`Hardened reference refused: ${error.message}`); process.exitCode = 1; }
  }
}
