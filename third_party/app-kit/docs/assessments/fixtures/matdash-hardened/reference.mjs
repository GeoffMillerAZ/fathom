// Separately pinned private derivative; original MatDash fixtures stay unchanged.
import { readFileSync, realpathSync } from 'node:fs';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { sourceRoots, sourceSnapshot, sha256 } from './source-closure.mjs';
import { startPinnedStaticReference } from '../matdash-native/reference-server.mjs';

export const manifestHash = '2c3d53eea0ffb3247aa1b7247c8869fd3fa9200ea6f0e48c8eede627eb6f4007';
export const buildHash = 'efa4bba0c8fb5da36eded564a717a034c47ac2ca22796a5797ef14be0f20cf54';
export function verifySource(project, original = false) {
  const bytes = readFileSync(new URL('./source-manifest.json', import.meta.url));
  if (sha256(bytes) !== manifestHash) throw new Error('Pinned source metadata mismatch.');
  const manifest = JSON.parse(bytes);
  if (JSON.stringify(sourceRoots) !== JSON.stringify(manifest.roots)) throw new Error('Selected source roots differ.');
  const actual = sourceSnapshot(project), expected = original ? manifest.original : manifest.candidate;
  if (actual.tree_sha256 !== expected.tree_sha256 || actual.count !== expected.count || actual.total_bytes !== expected.total_bytes) throw new Error('Selected source closure differs.');
  return { id: original ? 'matdash-asset-bound-base' : manifest.id, source_manifest_sha256: manifestHash,
    files: actual.count, total_bytes: actual.total_bytes, tree_sha256: actual.tree_sha256, scope: manifest.scope };
}
export async function startHardenedReference(project) {
  verifySource(project);
  return startPinnedStaticReference(project, {
    metadataFile: fileURLToPath(new URL('./build-assets.json', import.meta.url)), metadataSha256: buildHash,
  });
}
if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
  const [, , mode, project] = process.argv;
  if (process.argv.length !== 4 || !['--verify', '--verify-base', '--serve'].includes(mode)) {
    console.error('Usage: node reference.mjs --verify|--verify-base|--serve ABSOLUTE_PRIVATE_PROJECT'); process.exitCode = 2;
  } else {
    try {
      if (mode !== '--serve') console.log(JSON.stringify(verifySource(project, mode === '--verify-base')));
      else {
        const server = await startHardenedReference(project);
        console.log(JSON.stringify({ origin: server.origin, source_manifest_sha256: manifestHash, build_metadata_sha256: buildHash }));
        let closing = false;
        const stop = async () => { if (closing) return; closing = true; try { await server.close(); } catch { process.exitCode = 1; } };
        process.on('SIGINT', stop); process.on('SIGTERM', stop);
      }
    } catch { console.error('Hardened MatDash reference refused: selected source or build verification failed.'); process.exitCode = 1; }
  }
}
