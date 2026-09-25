import crypto from 'node:crypto';

export const sourceManifestSHA256 = '2fb9ae848b8040f8aee05f74a741253592a53a331201baca299dd813dc121f48';
export const sourceSHA256 = '9114b0b981055638eb28fede17eff7bc49c91f40fc32ce758514bfdb7d19f937';
export const runtimeESMSHA256 = '120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013';
export const runtimeTGZSHA256 = 'fc684f17d960fb60dadfdf9a0b49ba8fd295af4b8fd1715a26cd20b832ea245b';

const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const flat = name => typeof name === 'string' && /^[A-Za-z0-9._-]+$/.test(name);
const identity = bytes => ({sha256: sha(bytes), size: bytes.length});
const sameIdentity = (want, body) => !!want && want.sha256 === sha(body) && want.size === body.length;
const preparedNames = new Set(['USAGE.md', 'artifact.json', 'defaults.json', 'elements.js', 'faq.css',
  'faq.templ', 'faq_model.go', 'inter.woff2', 'native-dark.html', 'native-formatted-dark.html',
  'native-formatted-light.html', 'native-formatted.html', 'native-light.html', 'native.html',
  'reference-dark.html', 'reference-formatted-dark.html', 'reference-formatted-light.html',
  'reference-formatted.html', 'reference-light.html', 'reference.css', 'reference.html',
  'source-dark.html', 'source-light.html', 'source-manifest.json', 'source.html']);

export function nativeDocumentBytes(theme, fragment) {
  if (!['light', 'dark'].includes(theme) || !Buffer.isBuffer(fragment) || !fragment.length)
    throw Error('invalid native document input');
  return Buffer.from(`<!doctype html><html lang="en" class="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Private FAQ native review</title><link rel="stylesheet" href="reference.css"><link rel="stylesheet" href="faq.css"></head><body><main id="specimen">${fragment}</main></body></html>`);
}

export function parsePreparedManifest(bytes, expectedSHA256) {
  if (!/^[a-f0-9]{64}$/.test(expectedSHA256 || '') || sha(bytes) !== expectedSHA256)
    throw Error('unreviewed FAQ manifest; supply its exact SHA-256');
  const manifest = JSON.parse(bytes);
  if (manifest.source_manifest_sha256 !== sourceManifestSHA256)
    throw Error('FAQ preparation is not bound to the frozen source manifest');
  return manifest;
}

export function validateSourceFileBindings(manifest, files, sourceManifest) {
  const fail = () => { throw Error('invalid FAQ lane binding'); };
  if (!sourceManifest || sourceManifest.schema_version !== 1 || sourceManifest.kind !== 'elements-source-reference' ||
      sourceManifest.verification !== 'not-verified' || !sourceManifest.files ||
      Object.keys(sourceManifest.files).length !== 18) fail();
  const sourceRuntime = sourceManifest.runtime;
  if (sourceRuntime?.package !== '@tailwindplus/elements' || sourceRuntime.version !== '1.0.22' ||
      sourceRuntime.archive_sha256 !== runtimeTGZSHA256 || sourceRuntime.elements_sha256 !== runtimeESMSHA256) fail();
  const source = sourceManifest?.cases?.filter(item => item.id === 'P03-faq');
  if (!source || source.length !== 1 || source[0].source.sha256 !== sourceSHA256 ||
      !flat(source[0].source_file) || !source[0].documents ||
      Object.keys(source[0].documents).sort().join(',') !== 'dark,light' ||
      source[0].remote_assets?.length !== 0) fail();
  for (const key of ['archive_id', 'path', 'sha256', 'size'])
    if (manifest.source?.[key] !== source[0].source[key]) fail();
  const copied = new Map([['source.html', source[0].source_file], ['elements.js', 'elements.js'],
    ['reference.css', 'reference.css'], ['inter.woff2', 'inter.woff2']]);
  for (const [prepared, original] of copied)
    if (!flat(original) || !files.has(prepared) || !sameIdentity(sourceManifest.files[original], files.get(prepared))) fail();
  for (const theme of ['light', 'dark']) {
    const original = source[0].documents[theme], prepared = manifest.documents?.source?.[theme];
    if (!flat(original) || !flat(prepared) || !files.has(prepared) ||
        !sameIdentity(sourceManifest.files[original], files.get(prepared))) fail();
  }
}

export function validatePreparedDocumentBindings(manifest, files) {
  const fail = () => { throw Error('invalid FAQ lane binding'); };
  if (manifest.schema_version !== 1 || manifest.kind !== 'p03-faq-typed-preparation' ||
      manifest.verification !== 'not-verified' || manifest.source?.sha256 !== sourceSHA256 ||
      !files || !(files instanceof Map)) fail();
  const runtime = manifest.runtime?.source_reference;
  if (runtime?.package !== '@tailwindplus/elements' || runtime.version !== '1.0.22' ||
      runtime.esm_sha256 !== runtimeESMSHA256 || runtime.tgz_sha256 !== runtimeTGZSHA256 ||
      runtime.delivery !== 'host-provided-module' || manifest.runtime.native_details !== null) fail();
  if (manifest.artifact?.adapter !== 'p03-faq-v1' || manifest.artifact.verification !== 'not-verified' ||
      manifest.artifact.source_sha256 !== sourceSHA256) fail();
  const required = new Set(['source.html', 'reference.html', 'reference-formatted.html',
    'native.html', 'native-formatted.html', 'elements.js', 'reference.css', 'faq.css', 'inter.woff2']);
  for (const name of required) if (!files.has(name)) fail();
  const lanes = manifest.documents;
  for (const lane of ['source', 'reference', 'reference_formatted', 'native', 'native_formatted']) {
    if (!lanes?.[lane]) fail();
    for (const theme of ['light', 'dark']) if (!flat(lanes[lane][theme]) || !files.has(lanes[lane][theme])) fail();
  }
  const names = Object.values(lanes).flatMap(pair => Object.values(pair));
  if (new Set(names).size !== names.length) fail();
  const original = files.get('source.html');
  for (const theme of ['light', 'dark']) {
    const sourceDoc = files.get(lanes.source[theme]);
    if (sourceDoc.indexOf(original) < 0 || sourceDoc.indexOf(original, sourceDoc.indexOf(original) + 1) >= 0 ||
        !sourceDoc.includes(Buffer.from('reference.css')) || !sourceDoc.includes(Buffer.from('elements.js')) ||
        sourceDoc.includes(Buffer.from('faq.css'))) fail();
    for (const lane of ['reference', 'reference_formatted']) {
      const fragment = files.get(lane === 'reference' ? 'reference.html' : 'reference-formatted.html');
      const expected = Buffer.concat([sourceDoc.subarray(0, sourceDoc.indexOf(original)), fragment,
        sourceDoc.subarray(sourceDoc.indexOf(original) + original.length)]);
      if (!files.get(lanes[lane][theme]).equals(expected)) fail();
    }
    for (const lane of ['native', 'native_formatted']) {
      const fragment = files.get(lane === 'native' ? 'native.html' : 'native-formatted.html');
      const document = files.get(lanes[lane][theme]);
      if (!document.equals(nativeDocumentBytes(theme, fragment)) || document.includes(Buffer.from('<script')) ||
          document.indexOf(fragment) < 0 || document.indexOf(fragment, document.indexOf(fragment) + 1) >= 0) fail();
    }
  }
}

export function validatePreparedBindings(manifest, files) {
  const sourceManifestBytes = files?.get('source-manifest.json');
  if (!Buffer.isBuffer(sourceManifestBytes) || sha(sourceManifestBytes) !== sourceManifestSHA256)
    throw Error('FAQ preparation is not bound to the frozen source manifest bytes');
  let sourceManifest;
  try { sourceManifest = JSON.parse(sourceManifestBytes); }
  catch { throw Error('FAQ preparation has an invalid frozen source manifest'); }
  if (!manifest.files || Object.keys(manifest.files).length !== preparedNames.size || files.size !== preparedNames.size)
    throw Error('invalid FAQ prepared file closure');
  for (const name of preparedNames) {
    const body = files.get(name), want = manifest.files[name];
    if (!flat(name) || !Buffer.isBuffer(body) || !want ||
        want.sha256 !== identity(body).sha256 || want.size !== body.length)
      throw Error('invalid FAQ prepared file closure');
  }
  validatePreparedDocumentBindings(manifest, files);
  validateSourceFileBindings(manifest, files, sourceManifest);
}

export function requestsAreLocalGET(requests, base) {
  return requests.every(request => request.method === 'GET' && new URL(request.url).origin === base && request.allowed);
}
