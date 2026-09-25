// Synthetic contract tests only: no Next project, paid input, browser, or cache
// process is started here.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {mkdtempSync, rmSync, symlinkSync, writeFileSync} from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import {gunzipSync, gzipSync} from 'node:zlib';
import {CACHED_STATIC_POLICY_ID, createCachedStaticReferenceHandler, prepareCachedStaticAssets, readCachedStaticPacket, selectEncoding} from './cached-static-reference-server.mjs';

const hash = value => createHash('sha256').update(value).digest('hex');

async function fixtureServer() {
  const body = Buffer.from('body{color:CanvasText}');
  const assets = prepareCachedStaticAssets(new Map([['/static/p10.css', {body, mime: 'text/css', sha256: hash(body)}]]));
  const objectHeaders = {'Content-Type': 'text/plain; charset=utf-8', 'Cache-Control': 'public, max-age=86400', 'X-Delegated': 'object'};
  const rawHeaders = ['Content-Type', 'text/plain; charset=utf-8', 'Cache-Control', 'public, max-age=86400', 'X-Delegated', 'raw-array'];
  const delegate = (request, response) => {
    response.setHeader('Cache-Control', 'public, max-age=86400');
    response.removeHeader('Cache-Control');
    if (request.url === '/raw-array') {
      response.writeHead(404, rawHeaders);
      response.end('delegated');
      return;
    }
    if (request.url === '/object-header') {
      response.writeHead(404, objectHeaders);
      response.end('delegated');
      return;
    }
    response.setHeader('X-Delegated', 'yes');
    response.writeHead(404, {'Content-Type': 'text/plain; charset=utf-8'});
    response.end('delegated');
  };
  const server = http.createServer(createCachedStaticReferenceHandler({
    origin: 'http://127.0.0.1:0', assets, delegate,
  }));
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  const origin = `http://127.0.0.1:${server.address().port}`;
  server.removeAllListeners('request');
  server.on('request', createCachedStaticReferenceHandler({origin, assets, delegate}));
  return {origin, assets, objectHeaders, rawHeaders, close: () => new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()))};
}

function request(origin, path, {method = 'GET', headers = {}} = {}) {
  const url = new URL(origin);
  return new Promise((resolve, reject) => {
    const req = http.request({hostname: url.hostname, port: url.port, path, method, headers, agent: false}, response => {
      const chunks = [];
      response.on('data', chunk => chunks.push(chunk));
      response.once('error', reject);
      response.once('end', () => resolve({status: response.statusCode, headers: response.headers, body: Buffer.concat(chunks)}));
    });
    req.once('error', reject);
    req.end();
  });
}

test('cached static identity, gzip, HEAD and negotiation contract', async t => {
  const fixture = await fixtureServer();
  t.after(fixture.close);
  const raw = fixture.assets.get('/static/p10.css').identity.body;
  const identity = await request(fixture.origin, '/static/p10.css', {headers: {'accept-encoding': 'gzip;q=0'}});
  assert.equal(identity.status, 200);
  assert.equal(identity.headers['cache-control'], 'private, max-age=60');
  assert.equal(identity.headers.vary, 'Accept-Encoding');
  assert.equal(identity.headers['content-type'], 'text/css');
  assert.equal(identity.headers['content-encoding'], undefined);
  assert.equal(identity.headers.etag, `"${hash(raw)}"`);
  assert.equal(identity.headers['content-length'], String(raw.length));
  assert.deepEqual(identity.body, raw);
  assert.equal(identity.headers['x-appkit-cached-static-policy'], CACHED_STATIC_POLICY_ID);

  const gzipped = await request(fixture.origin, '/static/p10.css', {headers: {'accept-encoding': 'gzip, identity;q=0.2'}});
  assert.equal(gzipped.status, 200);
  assert.equal(gzipped.headers['content-encoding'], 'gzip');
  assert.equal(gzipped.headers.etag, `"${hash(gzipped.body)}"`);
  assert.equal(gzipped.headers['content-length'], String(gzipped.body.length));
  assert.deepEqual(gunzipSync(gzipped.body), raw);
  assert.deepEqual(gzipped.body, gzipSync(raw, {level: 6, mtime: 0}));

  const head = await request(fixture.origin, '/static/p10.css', {method: 'HEAD', headers: {'accept-encoding': 'gzip'}});
  assert.equal(head.status, 200);
  assert.equal(head.headers['cache-control'], 'private, max-age=60');
  assert.equal(head.headers['content-encoding'], 'gzip');
  assert.equal(head.headers.etag, gzipped.headers.etag);
  assert.equal(head.headers['content-length'], String(gzipped.body.length));
  assert.equal(head.body.length, 0);

  const weakMatch = await request(fixture.origin, '/static/p10.css', {headers: {'accept-encoding': 'gzip', 'if-none-match': `"different", W/${gzipped.headers.etag}`}});
  assert.equal(weakMatch.status, 304);
  assert.equal(weakMatch.headers.etag, gzipped.headers.etag);
  assert.equal(weakMatch.headers['content-length'], String(gzipped.body.length));
  assert.equal(weakMatch.body.length, 0);
  const starMatch = await request(fixture.origin, '/static/p10.css', {method: 'HEAD', headers: {'if-none-match': '*'}});
  assert.equal(starMatch.status, 304);
  assert.equal(starMatch.headers.etag, identity.headers.etag);

  const rejected = await request(fixture.origin, '/static/p10.css', {headers: {'accept-encoding': 'identity;q=0, gzip;q=0'}});
  assert.equal(rejected.status, 406);
  assert.equal(rejected.headers['cache-control'], 'no-store');
  assert.ok(!String(rejected.headers['cache-control']).includes('max-age'));
});

test('only exact non-route-data GET/HEAD static URLs are cacheable', async t => {
  const fixture = await fixtureServer();
  t.after(fixture.close);
  for (const [name, path, options, status] of [
    ['query', '/static/p10.css?v=1', {}, 404],
    ['RSC', '/static/p10.css', {headers: {rsc: '1'}}, 404],
    ['prefetch', '/static/p10.css', {headers: {'next-router-prefetch': '1'}}, 404],
    ['segment prefetch', '/static/p10.css', {headers: {'next-router-segment-prefetch': '1'}}, 404],
    ['unknown', '/static/unknown.css', {}, 404],
    ['raw delegated headers', '/raw-array', {}, 404],
    ['object delegated headers', '/object-header', {}, 404],
    ['POST', '/static/p10.css', {method: 'POST'}, 405],
    ['foreign host', '/static/p10.css', {headers: {host: 'example.invalid'}}, 403],
  ]) {
    await t.test(name, async () => {
      const response = await request(fixture.origin, path, options);
      assert.equal(response.status, status);
      assert.equal(response.headers['cache-control'], 'no-store');
      if (name === 'raw delegated headers') assert.equal(response.headers['x-delegated'], 'raw-array');
      else if (name === 'object delegated headers') assert.equal(response.headers['x-delegated'], 'object');
      else if (status === 404) assert.equal(response.headers['x-delegated'], 'yes');
      if (name === 'foreign host') assert.equal(response.headers.etag, undefined, 'authority checks run before cache lookup');
    });
  }
  assert.equal(fixture.objectHeaders['Cache-Control'], 'public, max-age=86400', 'delegated object headers remain caller-owned');
  assert.equal(fixture.rawHeaders[3], 'public, max-age=86400', 'delegated raw headers remain caller-owned');
});

test('encoding q values retain identity default and honor exclusion', () => {
  assert.equal(selectEncoding(undefined), 'identity');
  assert.equal(selectEncoding('gzip;q=0'), 'identity');
  assert.equal(selectEncoding('gzip;q=0.9, identity;q=1'), 'identity');
  assert.equal(selectEncoding('gzip;q=1, identity;q=0'), 'gzip');
  assert.equal(selectEncoding('gzip;q=0, identity;q=0'), null);
  assert.equal(selectEncoding('gzip;q=NaN, identity;q=0'), null);
  assert.equal(selectEncoding('gzip;q=1.0000, identity;q=0'), null);
  assert.equal(selectEncoding('gzip;q=1junk, identity;q=0'), null);
  assert.equal(selectEncoding('gzip;q=0.0001, identity;q=0'), null);
  assert.equal(selectEncoding('gzip;q = 0, identity;q=0'), null);
  assert.equal(selectEncoding('gzip;q = 1, identity;q=0'), null);
  assert.equal(selectEncoding('gzip;q=1;trailing, identity;q=0'), null);
  assert.equal(selectEncoding('gzip;q=0;q=1, identity;q=0'), null);
  assert.equal(selectEncoding('gzip;q, identity;q=0'), null);
});

function temporaryPacket() {
  const root = mkdtempSync(join(os.tmpdir(), 'appkit-p10-cached-static-'));
  const manifest = 'a'.repeat(64), assets = [];
  for (let index = 0; index < 43; index++) {
    const body = Buffer.from(`asset-${index}`), name = `asset-${index}.css`;
    writeFileSync(join(root, name), body, {mode: 0o600});
    assets.push({url: `/static/${index}.css`, mime: 'text/css', file: {name, sha256: hash(body), bytes: body.length}});
  }
  const reference = {schema_version: 1, id: 'test-p10', source_manifest_sha256: manifest, assets};
  const bytes = Buffer.from(JSON.stringify(reference));
  writeFileSync(join(root, 'reference.json'), bytes, {mode: 0o600});
  return {root, reference, referenceSHA256: hash(bytes), manifest};
}

test('packet loader verifies the complete bounded asset closure and refuses drift/symlinks', t => {
  const packet = temporaryPacket();
  t.after(() => rmSync(packet.root, {recursive: true, force: true}));
  const expected = {referenceSHA256: packet.referenceSHA256, sourceManifestSHA256: packet.manifest, id: 'test-p10', assetCount: 43};
  const loaded = readCachedStaticPacket(packet.root, expected);
  assert.equal(loaded.assets.length, 43);
  assert.equal(loaded.assets[0].sha256, hash(Buffer.from('asset-0')));
  writeFileSync(join(packet.root, 'asset-0.css'), 'drift', {mode: 0o600});
  assert.throws(() => readCachedStaticPacket(packet.root, expected), /asset drift/);

  const unsafe = temporaryPacket();
  t.after(() => rmSync(unsafe.root, {recursive: true, force: true}));
  const reference = structuredClone(unsafe.reference);
  reference.assets[0].url = '/settings';
  const unsafeBytes = Buffer.from(JSON.stringify(reference));
  writeFileSync(join(unsafe.root, 'reference.json'), unsafeBytes, {mode: 0o600});
  assert.throws(() => readCachedStaticPacket(unsafe.root, {...expected, referenceSHA256: hash(unsafeBytes)}), /Invalid cached-static asset pin/);

  const alias = `${packet.root}-link`;
  symlinkSync(packet.root, alias);
  t.after(() => rmSync(alias, {force: true}));
  assert.throws(() => readCachedStaticPacket(alias, expected), /non-symlink directory/);
});
