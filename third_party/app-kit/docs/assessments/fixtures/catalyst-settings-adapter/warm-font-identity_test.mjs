// Pure validator contract tests. These fixtures model receipt metadata only; no
// browser, CDP session, network request, cache, or response body is created.
import assert from 'node:assert/strict';
import test from 'node:test';
import {WARM_IDENTITY_ID, PLAYWRIGHT_WRAPPER_SHA256, MISSING_BODY_MESSAGE, auditWarmFontIdentity, exactMissingBodyError} from './warm-font-identity.mjs';

const origin = 'http://127.0.0.1:40123';
const aliasSHA = 'a'.repeat(64), fontSHA = 'b'.repeat(64), etag = `"${'c'.repeat(64)}"`;
const fontURL = '/__fonts/pinned/font-files/Inter-Regular.woff2?v=4.1';
const fullURL = origin + fontURL;
const observed = value => ({availability: 'observed', truncated: false, ambiguous: false, value});

function identity() {
  return {
    etag: observed(etag), content_length: observed('24'), vary: observed('Accept-Encoding'),
    delivery_experiment: observed('p10-cached-static-v2'), font_aliases_sha256: observed(aliasSHA),
  };
}
function line(phase, loader, request, response, finished, cache) {
  return {collection_id: '123e4567-e89b-42d3-a456-426614174000', phase, frame_id: 'frame-main', loader_id: loader,
    request_sequence: request, response_sequence: response, finished_sequence: finished, ...(cache ? {cache_event_sequence: cache} : {})};
}
function documentResource(requestID, phase, loader, request, response, finished) {
  return {request_id: requestID, url: origin + '/settings', group: 'document', settled: true, status: 200, method: 'GET', failure: null, failure_details: null,
    body: {availability: 'observed', sha256: 'document-observed', decoded_body_bytes_observed: 1}, lineage: line(phase, loader, request, response, finished)};
}
function primeFont() {
  return {request_id: 'prime-font', url: fullURL, group: 'font', settled: true, status: 200, method: 'GET', failure: null, failure_details: null,
    mime_type: 'font/woff2', cache_control: 'private, max-age=60', content_encoding: 'gzip', cache_identity: identity(),
    body: {availability: 'observed', sha256: fontSHA, decoded_body_bytes_observed: 4}, decoded_body_bytes_observed: 4,
    cache_flags: {from_service_worker: false, from_prefetch_cache: false}, lineage: line('warm-prime', 'loader-prime', 5, 6, 7)};
}
function warmFont() {
  const bodyFailure = `Error: ${MISSING_BODY_MESSAGE}`;
  return {request_id: 'warm-font', url: fullURL, group: 'font', settled: true, status: 200, method: 'GET', failure: null, failure_details: null,
    mime_type: 'font/woff2', cache_control: 'private, max-age=60', content_encoding: 'gzip', cache_identity: identity(),
    body: {availability: 'unavailable', sha256: null, decoded_body_bytes_observed: null}, decoded_body_bytes_observed: null, body_failure: bodyFailure,
    body_error: {source: 'playwright-cdp', command: 'Network.getResponseBody', name: {value: 'Error', truncated: false}, message: {value: MISSING_BODY_MESSAGE, truncated: false}, code: {availability: 'absent', value: null}},
    cache_flags: {from_service_worker: false, from_prefetch_cache: false}, served_from_cache_event: true, cache_provenance: 'disk-cache', lineage: line('warm-reload', 'loader-warm', 13, 14, 16, 15)};
}

function fixture() {
  const warm = warmFont();
  const sample = {
    profile: 'source', kind: 'warm', ok: false, route: origin + '/settings', collection_id: '123e4567-e89b-42d3-a456-426614174000', readiness: {complete: true},
    resources: [documentResource('warm-document', 'warm-reload', 'loader-warm', 10, 11, 12), warm],
    warm_prime_observation: {collection_id: '123e4567-e89b-42d3-a456-426614174000', resources: [documentResource('prime-document', 'warm-prime', 'loader-prime', 2, 3, 4), primeFont()]},
    blocked: [], page_errors: [], collector_errors: [`Response body unavailable for ${fullURL}: ${warm.body_failure}`],
    failures: ['collector-event-error', 'resource-failure', 'resource-body-unavailable', 'cache-contract: cached static check: sample must be an admitted successful observation'],
    timeline: [
      {sequence: 1, node_monotonic_ms: 1, kind: 'warm-prime-start', marker: true},
      {sequence: 2, node_monotonic_ms: 2, kind: 'Network.requestWillBeSent', request_id: 'prime-document', phase: 'warm-prime'},
      {sequence: 3, node_monotonic_ms: 3, kind: 'Network.responseReceived', request_id: 'prime-document', phase: 'warm-prime'},
      {sequence: 4, node_monotonic_ms: 4, kind: 'Network.loadingFinished', request_id: 'prime-document', phase: 'warm-prime'},
      {sequence: 5, node_monotonic_ms: 5, kind: 'Network.requestWillBeSent', request_id: 'prime-font', phase: 'warm-prime'},
      {sequence: 6, node_monotonic_ms: 6, kind: 'Network.responseReceived', request_id: 'prime-font', phase: 'warm-prime'},
      {sequence: 7, node_monotonic_ms: 7, kind: 'Network.loadingFinished', request_id: 'prime-font', phase: 'warm-prime'},
      {sequence: 8, node_monotonic_ms: 8, kind: 'warm-prime-resources-cleared', marker: true},
      {sequence: 9, node_monotonic_ms: 9, kind: 'warm-reload-start', marker: true},
      {sequence: 10, node_monotonic_ms: 10, kind: 'Network.requestWillBeSent', request_id: 'warm-document', phase: 'warm-reload'},
      {sequence: 11, node_monotonic_ms: 11, kind: 'Network.responseReceived', request_id: 'warm-document', phase: 'warm-reload'},
      {sequence: 12, node_monotonic_ms: 12, kind: 'Network.loadingFinished', request_id: 'warm-document', phase: 'warm-reload'},
      {sequence: 13, node_monotonic_ms: 13, kind: 'Network.requestWillBeSent', request_id: 'warm-font', phase: 'warm-reload'},
      {sequence: 14, node_monotonic_ms: 14, kind: 'Network.responseReceived', request_id: 'warm-font', phase: 'warm-reload'},
      {sequence: 15, node_monotonic_ms: 15, kind: 'Network.requestServedFromCache', request_id: 'warm-font', phase: 'warm-reload'},
      {sequence: 16, node_monotonic_ms: 16, kind: 'Network.loadingFinished', request_id: 'warm-font', phase: 'warm-reload'},
      {sequence: 17, node_monotonic_ms: 17, kind: 'context-close-start', marker: true},
    ],
  };
  return {sample, assets: [{url: fontURL, mime: 'font/woff2', sha256: fontSHA, bytes: 4}],
    http: [{host: 'source', url: fontURL, method: 'GET', conditional: false, encoding: 'gzip', observed: {status: 200, method: 'GET', conditional: false, encoding: 'gzip', url: fontURL, decoded_asset_sha256: fontSHA, etag, content_length: 24, content_type: 'font/woff2', cache_control: 'private, max-age=60', vary: 'Accept-Encoding'}, delivery: {experiment: 'p10-cached-static-v2', alias_manifest_sha256: aliasSHA}}],
    origin, host: 'source', transport: {policy: 'p10-cached-static-v2', aliasSHA}, toolchain: {playwright_version: '1.63.0', wrapper_sha256: PLAYWRIGHT_WRAPPER_SHA256}};
}
function audit(input) { return auditWarmFontIdentity(input); }
function warm(input) { return input.sample.resources[1]; }
function prime(input) { return input.sample.warm_prime_observation.resources[1]; }

test('accepted validator link is bounded and does not mutate strict receipt input', () => {
  const input = fixture(), before = structuredClone(input);
  const result = audit(input);
  assert.deepEqual(input, before);
  assert.equal(result.id, WARM_IDENTITY_ID);
  assert.equal(result.status, 'validator-linked-identity-only');
  assert.equal(result.strict_sample_ok, false);
  assert.equal(result.direct_warm_body_observed, false);
  assert.deepEqual(result.linked_fonts.map(value => value.url), [fullURL]);
  assert.equal(result.linked_fonts[0].decoded_asset_sha256, fontSHA);
});

test('validator rejects compact receipt-boundary mutations', () => {
  const changes = [
    ['warm URL', input => { warm(input).url = origin + '/other.woff2?v=4.1'; }],
    ['collection', input => { warm(input).lineage.collection_id = '123e4567-e89b-42d3-a456-426614174001'; }],
    ['frame', input => { warm(input).lineage.frame_id = 'other-frame'; }],
    ['loader', input => { warm(input).lineage.loader_id = 'loader-prime'; }],
    ['sequence', input => { warm(input).lineage.request_sequence = 14; }],
    ['cache event', input => { warm(input).served_from_cache_event = false; }],
    ['worker', input => { warm(input).cache_flags.from_service_worker = true; }],
    ['prefetch', input => { warm(input).cache_flags.from_prefetch_cache = true; }],
    ['missing prefetch flag', input => { delete warm(input).cache_flags.from_prefetch_cache; }],
    ['304', input => { warm(input).status = 304; }],
    ['warm 204', input => { warm(input).status = 204; }],
    ['prime 204', input => { prime(input).status = 204; }],
    ['ETag', input => { warm(input).cache_identity.etag.value = `"${'d'.repeat(64)}"`; }],
    ['encoding', input => { warm(input).content_encoding = null; }],
    ['MIME', input => { warm(input).mime_type = 'font/woff'; }],
    ['length', input => { warm(input).cache_identity.content_length.value = '23'; }],
    ['vary', input => { warm(input).cache_identity.vary.value = 'accept-encoding'; }],
    ['v2 header', input => { warm(input).cache_identity.delivery_experiment.value = 'p10-cached-static-v1'; }],
    ['alias header', input => { warm(input).cache_identity.font_aliases_sha256.value = 'd'.repeat(64); }],
    ['asset pin', input => { input.assets[0].sha256 = 'd'.repeat(64); }],
    ['probe ETag', input => { input.http[0].observed.etag = `"${'d'.repeat(64)}"`; }],
    ['probe delivery', input => { input.http[0].delivery.alias_manifest_sha256 = 'd'.repeat(64); }],
    ['tool version', input => { input.toolchain.playwright_version = '1.62.0'; }],
    ['wrapper pin', input => { input.toolchain.wrapper_sha256 = 'd'.repeat(64); }],
    ['body substitution', input => { warm(input).body.sha256 = fontSHA; }],
    ['missing prime', input => { input.sample.warm_prime_observation.resources = input.sample.warm_prime_observation.resources.filter(resource => resource.group !== 'font'); }],
    ['unrelated error', input => { input.sample.collector_errors.push('unexpected'); }],
    ['extra candidate', input => { const duplicate = structuredClone(warm(input)); duplicate.request_id = 'warm-font-2'; input.sample.resources.push(duplicate); }],
    ['loading failure', input => { input.sample.timeline.push({sequence: 18, node_monotonic_ms: 18, kind: 'Network.loadingFailed'}); }],
  ];
  for (const [name, change] of changes) {
    const input = fixture(); change(input);
    assert.throws(() => audit(input), /warm identity/, name);
  }
});

test('missing-data classification is exact and structured', () => {
  const numeric = fixture();
  warm(numeric).body_error.code = {availability: 'observed', value: -32000};
  assert.doesNotThrow(() => exactMissingBodyError(warm(numeric), numeric.toolchain), 'actual numeric code may be retained, never invented');
  const changes = [
    ['body error name', input => { warm(input).body_error.name.value = 'TypeError'; }],
    ['body error name extra field', input => { warm(input).body_error.name.availability = 'observed'; }],
    ['body error message character', input => { warm(input).body_error.message.value = MISSING_BODY_MESSAGE.replace('given', 'gixen'); }],
    ['body error newline', input => { warm(input).body_error.message.value = MISSING_BODY_MESSAGE + '\n'; }],
    ['body error period', input => { warm(input).body_error.message.value = MISSING_BODY_MESSAGE + '.'; }],
    ['body error message extra field', input => { warm(input).body_error.message.ambiguous = true; }],
    ['different command', input => { warm(input).body_error.command = 'Fetch.getResponseBody'; }],
    ['different source', input => { warm(input).body_error.source = 'network'; }],
    ['raw failure mismatch', input => { warm(input).body_failure += '.'; input.sample.collector_errors[0] = `Response body unavailable for ${fullURL}: ${warm(input).body_failure}`; }],
    ['malformed numeric code', input => { warm(input).body_error.code = {availability: 'observed', value: 'x'}; }],
    ['ambiguous numeric code', input => { warm(input).body_error.code = {availability: 'observed', value: -32000, ambiguous: true}; }],
  ];
  for (const [name, change] of changes) {
    const input = fixture(); change(input);
    assert.throws(() => exactMissingBodyError(warm(input), input.toolchain), /warm identity/, name);
  }
});
