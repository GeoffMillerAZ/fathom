import test from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import {declaredImageRefusalsOnly, parsePairManifest, sourceManifestSHA256, validatePairBindings} from './contract.mjs';

const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');

test('pair manifest requires explicit hash and the frozen source manifest', () => {
  const bytes = Buffer.from(JSON.stringify({source_manifest_sha256: sourceManifestSHA256}));
  const expected = sha(bytes);
  assert.equal(parsePairManifest(bytes, expected).source_manifest_sha256, sourceManifestSHA256);
  assert.throws(() => parsePairManifest(bytes, '0'.repeat(64)), /unreviewed pair manifest/);
  assert.throws(() => parsePairManifest(bytes, ''), /exact reviewed/);
  const changed = Buffer.from(JSON.stringify({source_manifest_sha256: '0'.repeat(64)}));
  assert.throws(() => parsePairManifest(changed, sha(changed)), /frozen source manifest/);
});

test('request oracle admits only local GETs or declared off-origin images', () => {
  const base = 'http://127.0.0.1:43123';
  const image = 'https://images.example/photo.jpg';
  const expected = new Set([image]);
  assert.equal(declaredImageRefusalsOnly([
    {url: `${base}/random/document.html`, method: 'GET', allowed: true},
    {url: image, method: 'GET', allowed: false},
  ], base, expected), true);
  for (const bad of [
    {url: `${base}/random/missing.css`, method: 'GET', allowed: false},
    {url: 'https://other.example/script.js', method: 'GET', allowed: false},
    {url: image, method: 'POST', allowed: false},
  ]) assert.equal(declaredImageRefusalsOnly([bad], base, expected), false);
});

test('rehashed pair manifests cannot alias or misbind the compiled lane', () => {
  const source = {archive_id: 'synthetic', path: 'menu.html', sha256: 'a'.repeat(64), size: 25};
  const reference = {cases: [{id: 'menu', source, source_file: 'original.html',
    documents: {light: 'source-light.html', dark: 'source-dark.html'}}]};
  const original = '<el-menu>Menu</el-menu>', compiled = '<el-menu>Menu</el-menu>\n';
  const files = new Map([['original.html', original], ['formatted.html', compiled],
    ['source-light.html', `<main>${original}</main>`], ['source-dark.html', `<main class="dark">${original}</main>`],
    ['compiled-light.html', `<main>${compiled}</main>`], ['compiled-dark.html', `<main class="dark">${compiled}</main>`],
  ].map(([name, body]) => [name, Buffer.from(body)]));
  const pair = {source_manifest_sha256: sourceManifestSHA256, cases: [{id: 'menu', source,
    fragments: {source: 'original.html', compiled: 'formatted.html'}, formatted_html: 'formatted.html',
    source_documents: reference.cases[0].documents,
    compiled_documents: {light: 'compiled-light.html', dark: 'compiled-dark.html'}}]};
  const reviewed = value => { const bytes = Buffer.from(JSON.stringify(value)); return parsePairManifest(bytes, sha(bytes)); };
  assert.doesNotThrow(() => validatePairBindings(reviewed(pair), files, reference));
  const changes = [
    p => { p.cases[0].fragments.compiled = p.cases[0].fragments.source; },
    p => { p.cases[0].formatted_html = p.cases[0].fragments.compiled = 'original.html'; },
    p => { p.cases[0].compiled_documents = p.cases[0].source_documents; },
    p => { p.cases[0].compiled_documents.light = 'source-dark.html'; },
    p => { p.cases[0].source_documents.light = 'compiled-light.html'; },
    p => { p.cases[0].fragments.source = 'formatted.html'; },
    p => { p.cases[0].source.path = 'different.html'; },
    p => { p.cases[0].compiled_documents.dark = 'compiled-light.html'; },
  ];
  for (const change of changes) {
    const changed = structuredClone(pair); change(changed);
    assert.throws(() => validatePairBindings(reviewed(changed), files, reference), /lane binding/);
  }
  for (const [name, body] of [
    ['source-light.html', `<main>${original}${original}</main>`],
    ['compiled-light.html', `<main>${original}</main>`],
    ['compiled-dark.html', `<main>${compiled}</main>`],
  ]) {
    const changedFiles = new Map(files); changedFiles.set(name, Buffer.from(body));
    assert.throws(() => validatePairBindings(reviewed(pair), changedFiles, reference), /lane binding/);
  }
});
