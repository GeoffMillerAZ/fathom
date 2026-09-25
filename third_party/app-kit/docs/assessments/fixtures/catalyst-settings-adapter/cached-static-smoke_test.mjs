import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {gzipSync, gunzipSync} from 'node:zlib';
import {auditDeliveryMetadata, observePlaywrightWrapper, requestStatic, supplementalWarmResult, verifiedTargetManifest} from './cached-static-smoke.mjs';

const hash = value => createHash('sha256').update(value).digest('hex');

test('an unavailable supplemental wrapper is recorded rather than throwing out of base smoke', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'appkit-wrapper-observation-test-'));
  try {
    const result = observePlaywrightWrapper(path.join(directory, 'playwright/index.mjs'));
    assert.equal(result.availability, 'unavailable');
    assert.equal(result.sha256, null);
    assert.ok(result.reason.length > 0 && result.reason.length <= 1024);
  } finally { fs.rmdirSync(directory); }
});

test('supplemental refusal does not modify strict failed samples or invent body evidence', () => {
  const sample = {ok: false, kind: 'warm', failures: ['resource-body-unavailable'], resources: [{body: {availability: 'unavailable'}}]};
  const original = JSON.stringify(sample);
  const result = supplementalWarmResult({sample, origin: 'http://127.0.0.1:4444', host: 'source'});
  assert.equal(result.status, 'unavailable');
  assert.equal(result.strict_sample_ok, false);
  assert.equal(result.direct_warm_body_observed, false);
  assert.equal(JSON.stringify(sample), original);
});

test('delivery identity requires both exact selected headers and neither fallback header', () => {
  const transport = {policy: 'p10-cached-static-v2', aliasSHA: 'a'.repeat(64)};
  const headers = {'x-appkit-delivery-experiment': transport.policy, 'x-appkit-font-aliases-sha256': transport.aliasSHA};
  assert.deepEqual(auditDeliveryMetadata(headers, transport, true), {experiment: transport.policy, alias_manifest_sha256: transport.aliasSHA});
  assert.deepEqual(auditDeliveryMetadata({}, transport, false), {experiment: null, alias_manifest_sha256: null});
  for (const name of Object.keys(headers)) {
    const missing = {...headers}; delete missing[name];
    assert.throws(() => auditDeliveryMetadata(missing, transport, true), /scope\/identity/);
    assert.throws(() => auditDeliveryMetadata({...headers, [name]: 'drift'}, transport, true), /scope\/identity/);
    assert.throws(() => auditDeliveryMetadata({[name]: headers[name]}, transport, false), /scope\/identity/);
  }
});
const artifactNames = [
  'settings.templ', 'settings_templ.go', 'policy.go', 'view.go', 'catalyst_native.go', 'catalyst_native.css',
  'settings_test.go', 'go.mod', 'go.sum', 'rendered-default.html', 'rendered-errors.html', 'cmd/server/server.go',
  'cmd/server/server_test.go', 'cmd/server/appearance.go', 'cmd/server/controller.mjs', 'fixture',
];

function temporaryManifestFixture() {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'appkit-cached-static-smoke-test-'));
  const entries = artifactNames.map((name, index) => ({name, body: Buffer.from(`synthetic artifact ${index}\n`)}));
  for (const entry of entries) {
    fs.mkdirSync(path.join(directory, path.dirname(entry.name)), {recursive: true, mode: 0o700});
    fs.writeFileSync(path.join(directory, entry.name), entry.body, {mode: 0o600});
  }
  const writeManifest = records => fs.writeFileSync(path.join(directory, 'SHA256SUMS'), records.map(entry => `${hash(entry.body)}  ${entry.name}`).join('\n') + '\n', {mode: 0o600});
  writeManifest(entries);
  return {directory, binary: path.join(directory, 'fixture'), entries, writeManifest, remove: () => fs.rmSync(directory, {recursive: true, force: true})};
}

test('verifiedTargetManifest admits all sixteen pinned private-output artifacts and refuses drift', () => {
  const fixture = temporaryManifestFixture();
  try {
    const verified = verifiedTargetManifest(fixture.binary);
    assert.equal(verified.verified_files.length, 16);
    assert.equal(verified.verified_files.find(file => file.name === 'fixture').sha256, hash(fs.readFileSync(fixture.binary)));
    fs.appendFileSync(path.join(fixture.directory, 'policy.go'), 'changed');
    assert.throws(() => verifiedTargetManifest(fixture.binary), /target artifact drift: policy\.go/);
  } finally { fixture.remove(); }
});

test('verifiedTargetManifest refuses duplicate, escaping, and binary-missing manifests', () => {
  for (const mode of ['duplicate', 'escape', 'missing-binary']) {
    const fixture = temporaryManifestFixture();
    try {
      const records = fixture.entries.map(entry => ({...entry}));
      if (mode === 'duplicate') records[1] = {...records[0]};
      if (mode === 'escape') records[1] = {name: '../escape', body: records[1].body};
      if (mode === 'missing-binary') {
        const index = records.findIndex(entry => entry.name === 'fixture');
        records[index] = {name: 'other-artifact', body: records[index].body};
        fs.writeFileSync(path.join(fixture.directory, 'other-artifact'), records[index].body, {mode: 0o600});
      }
      fixture.writeManifest(records);
      assert.throws(() => verifiedTargetManifest(fixture.binary), mode === 'duplicate' ? /unique clean/ : mode === 'escape' ? /unique clean/ : /binary and transport source/);
    } finally { fixture.remove(); }
  }
});

test('requestStatic retains raw gzip, does not follow redirects, and refuses unsafe targets/methods', async () => {
  const raw = Buffer.from('synthetic compressed response bytes');
  const gzip = gzipSync(raw, {mtime: 0});
  const server = http.createServer((request, response) => {
    if (request.url === '/redirect') { response.writeHead(302, {location: '/gzip'}); response.end(); return; }
    if (request.url === '/gzip') { response.writeHead(200, {'content-encoding': 'gzip', 'content-length': String(gzip.length)}); response.end(gzip); return; }
    response.writeHead(404); response.end();
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${server.address().port}`;
  try {
    const compressed = await requestStatic(origin, '/gzip');
    assert.equal(compressed.status, 200);
    assert.deepEqual(compressed.body, gzip, 'raw response remains encoded');
    assert.deepEqual(gunzipSync(compressed.body), raw);
    const redirect = await requestStatic(origin, '/redirect');
    assert.equal(redirect.status, 302);
    assert.equal(redirect.headers.location, '/gzip');
    assert.throws(() => requestStatic('http://example.com:80', '/gzip'), /exact http:\/\/127\.0\.0\.1:PORT origin/);
    assert.throws(() => requestStatic(origin, '/gzip', {method: 'PUT'}), /GET\/HEAD or empty static-URL POST/);
  } finally {
    await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
});
