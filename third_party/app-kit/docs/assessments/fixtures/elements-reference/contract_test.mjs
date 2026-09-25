import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {parseReferenceManifest, declaredImageRefusalsOnly} from './contract.mjs';

test('unreviewed manifests fail before rendering or runtime loading', () => {
  for (const value of ['{}', '', '{"verification":"not-verified"}'])
    assert.throws(() => parseReferenceManifest(Buffer.from(value)), /unreviewed reference manifest/);
});

test('all denied requests must be declared off-origin image GETs', () => {
  const base = 'http://127.0.0.1:3456', image = 'https://images.example/photo.jpg';
  const expected = new Set([image]);
  const permitted = {url: `${base}/reference/document.html`, method: 'GET', allowed: true};
  assert.equal(declaredImageRefusalsOnly([permitted], base, expected), true);
  assert.equal(declaredImageRefusalsOnly([{url: image, method: 'GET', allowed: false}], base, expected), true);
  for (const request of [
    {url: `${base}/reference/missing.png`, method: 'GET', allowed: false},
    {url: `${base}/reference/missing.css`, method: 'GET', allowed: false},
    {url: 'https://unexpected.example/file.js', method: 'GET', allowed: false},
    {url: image, method: 'POST', allowed: false},
  ]) assert.equal(declaredImageRefusalsOnly([permitted, request], base, expected), false);
});

test('explicit private manifest rejects document/CSS/font/license/tarball rehashes', {
  skip: !process.env.APPKIT_ELEMENTS_REFERENCE,
}, () => {
  const bytes = fs.readFileSync(path.join(process.env.APPKIT_ELEMENTS_REFERENCE, 'manifest.json'));
  const manifest = parseReferenceManifest(bytes);
  for (const name of ['p03-faq-light.html', 'reference.css', 'inter.woff2', 'elements-LICENSE.md', 'elements-1.0.22.tgz']) {
    const changed = structuredClone(manifest);
    const tampered = Buffer.concat([fs.readFileSync(path.join(process.env.APPKIT_ELEMENTS_REFERENCE, name)), Buffer.from('\n')]);
    changed.files[name] = {sha256: crypto.createHash('sha256').update(tampered).digest('hex'), size: tampered.length};
    assert.throws(() => parseReferenceManifest(Buffer.from(JSON.stringify(changed))), /unreviewed reference manifest/);
  }
  assert.throws(() => parseReferenceManifest(Buffer.concat([bytes, Buffer.from('\n')])), /unreviewed reference manifest/);
});
