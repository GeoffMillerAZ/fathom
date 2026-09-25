// Fixed private asset-bound variant: static-byte delivery, not React/browser proof.
import { realpathSync } from 'node:fs';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { startPinnedStaticReference } from './reference-server.mjs';

export async function startAssetReference(project) {
  return startPinnedStaticReference(project, {
    metadataFile: fileURLToPath(new URL('./asset-build.json', import.meta.url)),
    metadataSha256: 'ab9a1a037634eff411d93de64a04ade52751638e282a9a2c7016e13b66f5896a',
  });
}

if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
  if (process.argv.length !== 3) {
    console.error('Usage: node asset-reference.mjs ABSOLUTE_PREPARED_PROJECT');
    process.exitCode = 2;
  } else {
    try {
      const reference = await startAssetReference(process.argv[2]);
      console.log(`app-kit MatDash asset reference: ${reference.origin}/dashboards/dashboard2`);
      let stopping = false;
      const stop = async () => {
        if (stopping) return;
        stopping = true;
        const deadline = setTimeout(() => process.exit(1), 6000);
        try { await reference.close(); } catch { process.exitCode = 1; }
        finally { clearTimeout(deadline); }
      };
      process.on('SIGINT', stop);
      process.on('SIGTERM', stop);
    } catch {
      console.error('MatDash asset reference refused: selected metadata, source, or build failed verification.');
      process.exitCode = 1;
    }
  }
}
