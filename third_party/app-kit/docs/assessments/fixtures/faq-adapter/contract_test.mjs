import test from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import {nativeDocumentBytes, parsePreparedManifest, requestsAreLocalGET, runtimeESMSHA256, runtimeTGZSHA256,
  sourceManifestSHA256, sourceSHA256, validatePreparedBindings, validatePreparedDocumentBindings,
  validateSourceFileBindings} from './contract.mjs';

const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const source = {archive_id: 'marketing@' + 'a'.repeat(64), path: 'html/faq.html', sha256: sourceSHA256, size: 8};
const fileIdentity = bytes => ({sha256: sha(bytes), size: bytes.length});

function fixture() {
  const original = Buffer.from('<p>x</p>'), reference = Buffer.from('<p>x</p>\n');
  const files = new Map([
    ['source.html', original], ['reference.html', reference], ['reference-formatted.html', reference],
    ['native.html', Buffer.from('<details></details>')], ['native-formatted.html', Buffer.from('<details></details>')],
    ['elements.js', Buffer.from('runtime')], ['reference.css', Buffer.from('css')],
    ['faq.css', Buffer.from('native css')], ['inter.woff2', Buffer.from('font')],
  ]);
  const documents = {};
  for (const lane of ['source', 'reference', 'reference_formatted', 'native', 'native_formatted']) {
    documents[lane] = {};
    for (const theme of ['light', 'dark']) {
      const name = `${lane}-${theme}.html`, fragment = lane === 'source' ? original :
        lane.startsWith('reference') ? reference : files.get(lane === 'native' ? 'native.html' : 'native-formatted.html');
      files.set(name, lane.startsWith('native') ? nativeDocumentBytes(theme, fragment) :
        Buffer.concat([Buffer.from('<link href="reference.css"><script src="elements.js"></script><main>'), fragment, Buffer.from('</main>')]));
      documents[lane][theme] = name;
    }
  }
  return {files, manifest: {schema_version: 1, kind: 'p03-faq-typed-preparation',
    source_manifest_sha256: sourceManifestSHA256, source,
    artifact: {adapter: 'p03-faq-v1', source_sha256: sourceSHA256, verification: 'not-verified'},
    runtime: {source_reference: {package: '@tailwindplus/elements', version: '1.0.22',
      esm_sha256: runtimeESMSHA256, tgz_sha256: runtimeTGZSHA256, delivery: 'host-provided-module'}, native_details: null},
    documents, verification: 'not-verified'}};
}

test('manifest requires an explicit reviewed hash and frozen source manifest', () => {
  const bytes = Buffer.from(JSON.stringify({source_manifest_sha256: sourceManifestSHA256}));
  assert.equal(parsePreparedManifest(bytes, sha(bytes)).source_manifest_sha256, sourceManifestSHA256);
  assert.throws(() => parsePreparedManifest(bytes, '0'.repeat(64)), /unreviewed/);
  const changed = Buffer.from(JSON.stringify({source_manifest_sha256: '0'.repeat(64)}));
  assert.throws(() => parsePreparedManifest(changed, sha(changed)), /frozen source/);
});

test('topology rejects rehashed lane aliases, source drift, and runtime invention', () => {
  const {manifest, files} = fixture();
  assert.doesNotThrow(() => validatePreparedDocumentBindings(manifest, files));
  for (const mutate of [
    m => { m.documents.reference.light = m.documents.source.light; },
    m => { m.source.sha256 = '0'.repeat(64); },
    m => { m.runtime.native_details = m.runtime.source_reference; },
    m => { m.runtime.source_reference.version = '1.0.23'; },
    m => { m.artifact.verification = 'verified'; },
  ]) {
    const changed = structuredClone(manifest); mutate(changed);
    assert.throws(() => validatePreparedDocumentBindings(changed, files), /lane binding/);
  }
  const changedFiles = new Map(files); changedFiles.set(manifest.documents.reference.dark, files.get(manifest.documents.source.dark));
  assert.throws(() => validatePreparedDocumentBindings(manifest, changedFiles), /lane binding/);
  for (const body of [files.get('reference.html'), Buffer.from('<details>wrong</details>')]) {
    const changedNative = new Map(files);
    changedNative.set(manifest.documents.native.light, nativeDocumentBytes('light', body));
    assert.throws(() => validatePreparedDocumentBindings(manifest, changedNative), /lane binding/);
  }
});

test('copied P03 source wrappers, CSS, font, and runtime bind to source-manifest identities', () => {
  const {manifest, files} = fixture();
  const originals = {source:'p03-original.html', light:'p03-light.html', dark:'p03-dark.html'};
  const sourceFiles = {
    [originals.source]: fileIdentity(files.get('source.html')),
    [originals.light]: fileIdentity(files.get(manifest.documents.source.light)),
    [originals.dark]: fileIdentity(files.get(manifest.documents.source.dark)),
    'elements.js': fileIdentity(files.get('elements.js')),
    'reference.css': fileIdentity(files.get('reference.css')),
    'inter.woff2': fileIdentity(files.get('inter.woff2')),
  };
  for (let i = 0; i < 12; i++) sourceFiles[`other-${i}.bin`] = fileIdentity(Buffer.from(`other-${i}`));
  const frozen = {schema_version:1, kind:'elements-source-reference', verification:'not-verified',
    runtime:{package:'@tailwindplus/elements', version:'1.0.22', archive_sha256:runtimeTGZSHA256,
      elements_sha256:runtimeESMSHA256}, files:sourceFiles,
    cases:[{id:'P03-faq', source, source_file:originals.source,
      documents:{light:originals.light,dark:originals.dark}, remote_assets:[]}]};
  assert.doesNotThrow(() => validateSourceFileBindings(manifest, files, frozen));
  for (const name of ['source.html', manifest.documents.source.light, 'reference.css', 'inter.woff2', 'elements.js']) {
    const changed = new Map(files); changed.set(name, Buffer.concat([files.get(name), Buffer.from('tampered')]));
    assert.throws(() => validateSourceFileBindings(manifest, changed, frozen), /lane binding/);
  }
  const changedRuntime = structuredClone(frozen); changedRuntime.runtime.version = '1.0.23';
  assert.throws(() => validateSourceFileBindings(manifest, files, changedRuntime), /lane binding/);
});

test('main validator parses and requires the frozen source-manifest bytes', () => {
  const {manifest, files} = fixture();
  const changed = new Map(files);
  changed.set('source-manifest.json', Buffer.from(JSON.stringify({cases:[{id:'P03-faq',source}]})));
  assert.throws(() => validatePreparedBindings(manifest, changed), /frozen source manifest bytes/);
});

test('optional actual bundle rejects rehashed source provenance spoofs', {skip: !process.env.APPKIT_FAQ_PREPARED_CHECK}, () => {
  const root = fs.realpathSync(process.env.APPKIT_FAQ_PREPARED_CHECK);
  const manifestBytes = fs.readFileSync(path.join(root, 'manifest.json'));
  const expected = process.env.APPKIT_FAQ_PREPARED_MANIFEST_SHA256 || '';
  const manifest = parsePreparedManifest(manifestBytes, expected);
  const files = new Map(Object.keys(manifest.files).map(name => [name, fs.readFileSync(path.join(root, name))]));
  assert.doesNotThrow(() => validatePreparedBindings(manifest, files));
  for (const name of ['source-manifest.json', 'source-light.html', 'reference.css', 'inter.woff2']) {
    const changedManifest = structuredClone(manifest), changedFiles = new Map(files);
    const changed = Buffer.concat([files.get(name), Buffer.from('\n')]);
    changedFiles.set(name, changed); changedManifest.files[name] = fileIdentity(changed);
    assert.throws(() => validatePreparedBindings(changedManifest, changedFiles), /frozen source manifest bytes|lane binding/);
  }
});

test('request oracle accepts only admitted local GET traffic', () => {
  const base = 'http://127.0.0.1:43123';
  assert.equal(requestsAreLocalGET([{url: `${base}/source-light.html`, method: 'GET', allowed: true}], base), true);
  for (const bad of [
    {url: 'https://example.test/x', method: 'GET', allowed: false},
    {url: `${base}/x`, method: 'POST', allowed: false},
    {url: `${base}/missing`, method: 'GET', allowed: false},
  ]) assert.equal(requestsAreLocalGET([bad], base), false);
});
