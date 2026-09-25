import crypto from 'node:crypto';

// Frozen after independent source preparation and exact offline replay. A new
// source/toolchain/wrapper revision needs a reviewed reference, not a rehashed
// editable manifest that silently changes the browser oracle.
export const manifestSHA256 = '2fb9ae848b8040f8aee05f74a741253592a53a331201baca299dd813dc121f48';

export function parseReferenceManifest(bytes) {
  if (crypto.createHash('sha256').update(bytes).digest('hex') !== manifestSHA256)
    throw Error('unreviewed reference manifest; prepare/review a separate baseline revision');
  return JSON.parse(bytes);
}

export function declaredImageRefusalsOnly(requests, base, expected) {
  return requests.every(request => request.allowed ||
    (request.method === 'GET' && new URL(request.url).origin !== base && expected.has(request.url)));
}
