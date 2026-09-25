import assert from 'node:assert/strict';
import test from 'node:test';
import {createHash} from 'node:crypto';
import {gzipSync} from 'node:zlib';
import {auditSampleAssets, auditStaticResponse} from './cached-static-checks.mjs';

const hash = value => createHash('sha256').update(value).digest('hex');
const css = Buffer.from('body{color:CanvasText}');
const font = Buffer.from('synthetic-font');
const assets = [
  {url: '/source.css', mime: 'text/css', sha256: hash(css), bytes: css.length},
  {url: '/inter.woff2', mime: 'font/woff2', sha256: hash(font), bytes: font.length},
];
const headers = (body, encoding = 'identity') => ({
  'cache-control': 'private, max-age=60', vary: 'Accept-Encoding', 'content-type': 'text/css; charset=utf-8',
  'x-content-type-options': 'nosniff', 'content-length': String(body.length), etag: `"${hash(body)}"`,
  ...(encoding === 'gzip' ? {'content-encoding': 'gzip'} : {}),
});
const resource = (asset, hit = false) => ({url: `http://127.0.0.1:4444${asset.url}`, mime_type: asset.mime, cache_control: 'private, max-age=60', cache_provenance: hit ? 'disk-cache' : 'unknown', body: {availability: 'observed', sha256: asset.sha256, decoded_body_bytes_observed: asset.bytes}});

test('audits pinned identity and gzip bytes without returning bodies', () => {
  const identity = auditStaticResponse({asset: assets[0], method: 'GET', encoding: 'identity', status: 200, headers: headers(css), body: css});
  const gzip = gzipSync(css);
  const compressed = auditStaticResponse({asset: assets[0], method: 'GET', encoding: 'gzip', status: 200, headers: headers(gzip, 'gzip'), body: gzip});
  assert.equal(identity.decoded_asset_sha256, assets[0].sha256);
  assert.equal(compressed.encoding, 'gzip');
  assert.equal('body' in compressed, false);
});

test('refuses encoded digest/length/ETag drift and bad protocol headers', () => {
  assert.throws(() => auditStaticResponse({asset: assets[0], method: 'GET', encoding: 'identity', status: 200, headers: {...headers(css), etag: '"bad"'}, body: css}), /ETag/);
  assert.throws(() => auditStaticResponse({asset: assets[0], method: 'GET', encoding: 'identity', status: 200, headers: {...headers(css), 'content-length': '1'}, body: css}), /content-length/);
  assert.throws(() => auditStaticResponse({asset: assets[0], method: 'GET', encoding: 'identity', status: 200, headers: {...headers(css), vary: 'Origin'}, body: css}), /Vary/);
  assert.throws(() => auditStaticResponse({asset: assets[0], method: 'GET', encoding: 'gzip', status: 200, headers: headers(css, 'gzip'), body: css}), /gzip/);
  const missingMIME = headers(css); delete missingMIME['content-type'];
  assert.throws(() => auditStaticResponse({asset: assets[0], method: 'GET', encoding: 'identity', status: 200, headers: missingMIME, body: css}), /content-type required/);
  const oversized = gzipSync(Buffer.alloc(assets[0].bytes + 2));
  assert.throws(() => auditStaticResponse({asset: assets[0], method: 'GET', encoding: 'gzip', status: 200, headers: headers(oversized, 'gzip'), body: oversized}), /pinned bound/);
});

test('permits omitted 304 content metadata but requires empty conditional bodies', () => {
  const result = auditStaticResponse({asset: assets[0], method: 'HEAD', encoding: 'identity', status: 304, conditional: true, headers: {'cache-control': 'private, max-age=60', vary: 'Accept-Encoding', 'x-content-type-options': 'nosniff', etag: `"${hash(css)}"`}, body: Buffer.alloc(0)});
  assert.equal(result.content_type_available, false);
  assert.equal(result.content_length_available, false);
  assert.throws(() => auditStaticResponse({asset: assets[0], method: 'HEAD', encoding: 'identity', status: 304, conditional: true, headers: {'cache-control': 'private, max-age=60', vary: 'Accept-Encoding', 'x-content-type-options': 'nosniff', etag: `"${hash(css)}"`}, body: Buffer.from('x')}), /empty body/);
});

test('refuses unselected cacheability, unknown warm hits, and failed samples', () => {
  const cold = {ok: true, kind: 'cold', resources: [resource(assets[0]), resource(assets[1]), {url: 'http://127.0.0.1:4444/light/settings', cache_control: 'no-store'}]};
  assert.equal(auditSampleAssets(cold, assets).selected_count, 2);
  assert.throws(() => auditSampleAssets({...cold, resources: [...cold.resources, {url: 'http://127.0.0.1:4444/controller.mjs', cache_control: 'private, max-age=60'}]}, assets), /unselected/);
  assert.throws(() => auditSampleAssets({...cold, kind: 'warm', cache: {observed_hits: [{url: 'http://127.0.0.1:4444/not-selected'}]}}, assets), /warm cache hit/);
  assert.throws(() => auditSampleAssets({...cold, kind: 'warm', cache: {observed_hits: [{url: 'http://127.0.0.1:4444/source.css'}]}}, assets), /selected-asset hit/);
  const warm = {...cold, kind: 'warm', resources: [resource(assets[0], true), resource(assets[1])], cache: {observed_hits: [{url: 'http://127.0.0.1:4444/source.css'}]}};
  assert.equal(auditSampleAssets(warm, assets).warm_cache_hit_count, 1);
  assert.throws(() => auditSampleAssets({...cold, ok: false}, assets), /admitted successful/);
});

test('query-bearing font requests do not silently inherit a query-free asset cache pin', () => {
  const fontRequest = {...resource(assets[1]), url: 'http://127.0.0.1:4444/inter.woff2?v=4.1', cache_control: 'no-store'};
  const sample = {ok: true, kind: 'cold', resources: [resource(assets[0]), fontRequest]};
  assert.throws(() => auditSampleAssets(sample, assets), /selected CSS and font/);
  assert.throws(() => auditSampleAssets({...sample, resources: [resource(assets[0]), {...fontRequest, cache_control: 'private, max-age=60'}]}, assets), /unselected response/);
  const aliasPin = {...assets[1], url: assets[1].url + '?v=4.1'};
  assert.equal(auditSampleAssets({...sample, resources: [resource(assets[0]), {...fontRequest, cache_control: 'private, max-age=60'}]}, [...assets, aliasPin]).selected_count, 2);
  assert.throws(() => auditSampleAssets({...sample, resources: [resource(assets[0]), {...fontRequest, url: fontRequest.url + '&extra=1', cache_control: 'private, max-age=60'}]}, [...assets, aliasPin]), /unselected response/);
});
