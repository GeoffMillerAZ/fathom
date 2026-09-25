// Synthetic v2 composition tests only. No Next project, private packet, browser,
// process launch, or cache directory is used here.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import http from 'node:http';
import test from 'node:test';
import {gunzipSync} from 'node:zlib';
import {deriveFontAliases} from '../catalyst-settings-adapter/font-query-aliases.mjs';
import {FONT_ALIAS_DELIVERY_EXPERIMENT, createFontAliasReferenceHandler} from './cached-static-font-alias-reference-server.mjs';
import {prepareCachedStaticAssets} from './cached-static-reference-server.mjs';

const hash = value => createHash('sha256').update(value).digest('hex');

function testClosure() {
  const body = Buffer.from('synthetic-font-byte-identity');
  const cssURL = '/__test_fonts/a/inter.css';
  const css = Buffer.from('@font-face { src: url("font-files/Test.woff2?v=4.1") format("woff2"); }');
  const assets = [
    {url: cssURL, mime: 'text/css', sha256: hash(css), bytes: css.length},
    {url: '/__test_fonts/a/font-files/Test.woff2', mime: 'font/woff2', sha256: hash(body), bytes: body.length},
  ];
  const aliases = deriveFontAliases({css, cssURL, assets, expectedCSSSHA256: hash(css), expectedOccurrences: 1, expectedAliases: 1});
  return {body, aliases, assetMap: prepareCachedStaticAssets(new Map(assets.map((asset, index) => [asset.url, {body: index ? body : css, mime: asset.mime, sha256: asset.sha256}])))};
}

async function fixtureServer() {
  const closure = testClosure(), seen = [];
  const delegate = (request, response) => {
    seen.push(request.url);
    response.setHeader('Cache-Control', 'public, max-age=86400');
    response.writeHead(404, {'Content-Type': 'text/plain; charset=utf-8'});
    response.end('delegated');
  };
  const server = http.createServer();
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  const origin = `http://127.0.0.1:${server.address().port}`;
  server.on('request', createFontAliasReferenceHandler({origin, assets: closure.assetMap, aliasManifest: closure.aliases, delegate}));
  return {origin, ...closure, seen, close: () => new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()))};
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

test('exact CSS alias and canonical URL share prepared representations', async t => {
  const fixture = await fixtureServer();
  t.after(fixture.close);
  const alias = fixture.aliases.aliases[0], canonical = alias.canonical_url;
  const canonicalIdentity = await request(fixture.origin, canonical, {headers: {'accept-encoding': 'identity'}});
  const aliasIdentity = await request(fixture.origin, alias.url, {headers: {'accept-encoding': 'identity'}});
  const canonicalResponse = await request(fixture.origin, canonical, {headers: {'accept-encoding': 'gzip'}});
  const aliasResponse = await request(fixture.origin, alias.url, {headers: {'accept-encoding': 'gzip'}});
  assert.equal(aliasResponse.status, 200);
  assert.equal(aliasResponse.headers['x-appkit-delivery-experiment'], FONT_ALIAS_DELIVERY_EXPERIMENT);
  assert.equal(aliasResponse.headers['x-appkit-font-aliases-sha256'], hash(Buffer.from(JSON.stringify(fixture.aliases))));
  assert.equal(aliasResponse.headers['cache-control'], 'private, max-age=60');
  assert.equal(aliasResponse.headers['content-encoding'], 'gzip');
  assert.equal(aliasResponse.headers.etag, canonicalResponse.headers.etag);
  assert.equal(aliasResponse.headers['content-length'], canonicalResponse.headers['content-length']);
  assert.equal(canonicalResponse.headers['x-appkit-delivery-experiment'], FONT_ALIAS_DELIVERY_EXPERIMENT);
  assert.equal(canonicalResponse.headers['x-appkit-font-aliases-sha256'], hash(Buffer.from(JSON.stringify(fixture.aliases))));
  assert.deepEqual(aliasResponse.body, canonicalResponse.body);
  assert.deepEqual(gunzipSync(aliasResponse.body), fixture.body);
  assert.equal(aliasIdentity.headers.etag, canonicalIdentity.headers.etag);
  assert.equal(aliasIdentity.headers['content-length'], canonicalIdentity.headers['content-length']);
  assert.deepEqual(aliasIdentity.body, canonicalIdentity.body);
  const head = await request(fixture.origin, alias.url, {method: 'HEAD', headers: {'accept-encoding': 'gzip'}});
  assert.equal(head.status, 200);
  assert.equal(head.headers.etag, aliasResponse.headers.etag);
  assert.equal(head.headers['content-length'], aliasResponse.headers['content-length']);
  assert.equal(head.body.length, 0);
  assert.deepEqual(fixture.seen, []);
});

test('only the raw exact alias is admitted and delegation observes its original URL', async t => {
  const fixture = await fixtureServer();
  t.after(fixture.close);
  const alias = fixture.aliases.aliases[0].url;
  for (const path of [`${alias}&extra=1`, alias.replace('?v=', '?V='), alias.replace('Test.woff2', 'Test%2ewoff2'), `${alias.slice(0, -3)}10`]) {
    const response = await request(fixture.origin, path);
    assert.equal(response.status, 404);
    assert.equal(response.headers['cache-control'], 'no-store');
    assert.equal(response.headers['x-appkit-delivery-experiment'], undefined);
    assert.equal(response.headers['x-appkit-font-aliases-sha256'], undefined);
  }
  assert.deepEqual(fixture.seen, [`${alias}&extra=1`, alias.replace('?v=', '?V='), alias.replace('Test.woff2', 'Test%2ewoff2'), `${alias.slice(0, -3)}10`]);
});

test('authority, method, and all route-data headers veto alias serving', async t => {
  const fixture = await fixtureServer();
  t.after(fixture.close);
  const alias = fixture.aliases.aliases[0].url;
  const foreign = await request(fixture.origin, alias, {headers: {host: 'example.invalid'}});
  assert.equal(foreign.status, 403);
  assert.equal(foreign.headers['cache-control'], 'no-store');
  assert.equal(foreign.headers.etag, undefined);
  assert.equal(foreign.headers['x-appkit-delivery-experiment'], undefined);
  const post = await request(fixture.origin, alias, {method: 'POST'});
  assert.equal(post.status, 405);
  assert.equal(post.headers.allow, 'GET, HEAD');
  assert.equal(post.headers['cache-control'], 'no-store');
  assert.equal(post.headers['x-appkit-delivery-experiment'], undefined);
  for (const name of ['rsc', 'next-router-prefetch', 'next-router-segment-prefetch', 'next-router-state-tree', 'next-url', 'next-action']) {
    const response = await request(fixture.origin, alias, {headers: {[name]: '1'}});
    assert.equal(response.status, 404, name);
    assert.equal(response.headers['cache-control'], 'no-store', name);
    assert.equal(response.headers.etag, undefined, name);
    assert.equal(response.headers['x-appkit-delivery-experiment'], undefined, name);
    assert.equal(response.headers['x-appkit-font-aliases-sha256'], undefined, name);
  }
  assert.equal(fixture.seen.length, 6);
  assert.ok(fixture.seen.every(value => value === alias));
});

test('handler refuses alias metadata that disagrees with prepared representations', () => {
  for (const field of ['sha256', 'bytes', 'mime', 'canonical_url']) {
    const fixture = testClosure(), manifest = structuredClone(fixture.aliases);
    manifest.aliases[0][field] = field === 'bytes' ? 1 : field === 'mime' ? 'text/css' : field === 'sha256' ? '0'.repeat(64) : manifest.css.url;
    assert.throws(() => createFontAliasReferenceHandler({origin: 'http://127.0.0.1:4444', assets: fixture.assetMap, aliasManifest: manifest, delegate: () => {}}), /identity mismatch/);
  }
  const fixture = testClosure(); fixture.aliases.css.sha256 = '0'.repeat(64);
  assert.throws(() => createFontAliasReferenceHandler({origin: 'http://127.0.0.1:4444', assets: fixture.assetMap, aliasManifest: fixture.aliases, delegate: () => {}}), /Unbound/);
});
