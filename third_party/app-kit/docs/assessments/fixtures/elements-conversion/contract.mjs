import crypto from 'node:crypto';

export const sourceManifestSHA256 = '2fb9ae848b8040f8aee05f74a741253592a53a331201baca299dd813dc121f48';

const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');

export function parsePairManifest(bytes, expectedPairSHA256) {
  if (!/^[a-f0-9]{64}$/.test(expectedPairSHA256 || '') || sha(bytes) !== expectedPairSHA256)
    throw Error('unreviewed pair manifest; supply the exact reviewed SHA-256');
  const manifest = JSON.parse(bytes);
  if (manifest.source_manifest_sha256 !== sourceManifestSHA256)
    throw Error('pair is not bound to the frozen source manifest');
  return manifest;
}

// The caller has already checked every file and the frozen source-manifest hash.
// A reviewed pair hash does not itself prove that two lanes are actually distinct.
export function validatePairBindings(manifest, files, sourceManifest) {
  const fail = () => { throw Error('invalid original/compiled lane binding'); };
  if (!Array.isArray(manifest.cases) || !Array.isArray(sourceManifest.cases) ||
      manifest.cases.length !== sourceManifest.cases.length) fail();
  const seen = new Set();
  for (const item of manifest.cases) {
    const source = sourceManifest.cases.find(c => c.id === item.id);
    if (!source || seen.has(item.id)) fail();
    seen.add(item.id);
    for (const key of ['archive_id', 'path', 'sha256', 'size'])
      if (item.source?.[key] !== source.source?.[key]) fail();
    if (item.fragments?.source !== source.source_file ||
        item.fragments?.compiled !== item.formatted_html ||
        item.fragments.compiled === item.fragments.source) fail();
    const original = files.get(source.source_file), compiled = files.get(item.formatted_html);
    if (!Buffer.isBuffer(original) || !original.length || !Buffer.isBuffer(compiled) || !compiled.length) fail();
    for (const theme of ['light', 'dark']) {
      const sourceName = source.documents?.[theme], compiledName = item.compiled_documents?.[theme];
      if (!sourceName || item.source_documents?.[theme] !== sourceName || !compiledName ||
          Object.values(source.documents).includes(compiledName)) fail();
      const wrapper = files.get(sourceName), output = files.get(compiledName);
      if (!Buffer.isBuffer(wrapper) || !Buffer.isBuffer(output)) fail();
      const offset = wrapper.indexOf(original);
      if (offset < 0 || wrapper.indexOf(original, offset + 1) >= 0) fail();
      const expected = Buffer.concat([wrapper.subarray(0, offset), compiled, wrapper.subarray(offset + original.length)]);
      if (!output.equals(expected)) fail();
    }
  }
}

export function declaredImageRefusalsOnly(requests, base, expected) {
  return requests.every(request => request.allowed ||
    (request.method === 'GET' && new URL(request.url).origin !== base && expected.has(request.url)));
}
