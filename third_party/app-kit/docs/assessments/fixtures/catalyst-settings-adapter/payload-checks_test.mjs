import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyBody, applyFinished, applyResponse, assertAllowedURL, byteObservation,
  cacheProvenance, descriptiveStatistics, exactLoopbackOrigin, newResource, networkFailureFacts, percentile, requestDiagnostics,
  sampleDisposition, summarizeResources, warmCacheObservation,
} from './payload-checks.mjs';

test('only exact explicit loopback origins and their requests are admitted', () => {
  assert.equal(exactLoopbackOrigin('http://127.0.0.1:4444'), 'http://127.0.0.1:4444');
  for (const value of ['http://localhost:4444', 'https://127.0.0.1:4444', 'http://127.0.0.1:4444/', 'http://127.0.0.1', 'http://127.0.0.1:4444/settings']) assert.throws(() => exactLoopbackOrigin(value));
  assert.equal(assertAllowedURL('http://127.0.0.1:4444/settings', ['http://127.0.0.1:4444']).pathname, '/settings');
  assert.throws(() => assertAllowedURL('https://example.test/x', ['http://127.0.0.1:4444']), /Outside-origin/);
});

test('resource groups preserve document, assets, runtime and controller accounting', () => {
  const origins = ['http://127.0.0.1:4444'];
  const make = (id, url, type, headers) => {
    const resource = newResource({requestId: id, request: {url, method: 'GET'}, type}, origins, 'source');
    return applyResponse(resource, {status: 200, headers, mimeType: headers['content-type']});
  };
  assert.deepEqual([
    make('1', 'http://127.0.0.1:4444/settings', 'Document', {'content-type': 'text/html'}).group,
    make('2', 'http://127.0.0.1:4444/app.css', 'Stylesheet', {'content-type': 'text/css'}).group,
    make('3', 'http://127.0.0.1:4444/inter.woff2', 'Font', {'content-type': 'font/woff2'}).group,
    make('4', 'http://127.0.0.1:4444/flag.svg', 'Image', {'content-type': 'image/svg+xml'}).group,
    make('5', 'http://127.0.0.1:4444/_next/a.js', 'Script', {'content-type': 'application/javascript'}).group,
    make('6', 'http://127.0.0.1:4444/controller.mjs', 'Script', {'content-type': 'application/javascript'}).group,
  ], ['document', 'css', 'font', 'image', 'runtime', 'controller']);
});

test('unknown cache state is not silently warm', () => {
  const unknown = {url: 'http://127.0.0.1:4444/a.css', group: 'css', cache_provenance: 'unknown'};
  assert.equal(cacheProvenance({}, false), 'unknown');
  assert.equal(warmCacheObservation([unknown]).status, 'not-observed');
  assert.equal(warmCacheObservation([{...unknown, cache_provenance: 'disk-cache'}]).status, 'observed-cache-hit');
});

test('encoded transfer and observed decoded body are different units', () => {
  assert.deepEqual(byteObservation({encodedDataLength: 31, decodedBodyBytes: 73}), {encoded_transfer_bytes: 31, decoded_body_bytes_observed: 73});
  assert.deepEqual(byteObservation({encodedDataLength: -1, decodedBodyBytes: NaN}), {encoded_transfer_bytes: null, decoded_body_bytes_observed: null});
  const resource = newResource({requestId: 'r', request: {url: 'http://127.0.0.1:4444/x.css'}, type: 'Stylesheet'}, ['http://127.0.0.1:4444'], 'native');
  applyResponse(resource, {status: 200, headers: {'content-type': 'text/css'}}); applyFinished(resource, {encodedDataLength: 7}); applyBody(resource, new Uint8Array([1, 2, 3, 4]));
  assert.equal(resource.encoded_transfer_bytes, 7); assert.equal(resource.decoded_body_bytes_observed, 4);
  assert.deepEqual(summarizeResources([resource]).css, {
    count: 1, encoded_transfer_bytes: 7, encoded_transfer_bytes_unknown: 0,
    decoded_body_bytes_observed: 4, decoded_body_bytes_unknown: 0, body_unavailable: 0,
  });
});

test('a missing body stays explicit rather than receiving a synthetic digest', () => {
  const resource = newResource({requestId: 'r', request: {url: 'http://127.0.0.1:4444/x'}, type: 'Document'}, ['http://127.0.0.1:4444'], 'source');
  applyResponse(resource, {status: 200, headers: {'content-type': 'text/html'}});
  assert.deepEqual(resource.body, {availability: 'unavailable', sha256: null, decoded_body_bytes_observed: null});
  assert.equal(summarizeResources([resource]).document.body_unavailable, 1);
  assert.equal(summarizeResources([resource]).document.encoded_transfer_bytes_unknown, 1);
  assert.equal(summarizeResources([resource]).document.decoded_body_bytes_unknown, 1);
});

test('profile failure canaries are fail closed', () => {
  const native = sampleDisposition({profile: 'native', readiness: {complete: true, javascript_disabled: true, application_script_elements: 0}, resources: [{group: 'runtime'}]});
  assert.equal(native.ok, false); assert.ok(native.failures.includes('native-application-javascript-observed'));
  const enhanced = sampleDisposition({profile: 'enhanced', readiness: {complete: true, enhanced_ready: false}, resources: []});
  assert.equal(enhanced.ok, false); assert.ok(enhanced.failures.includes('enhanced-controller-not-ready')); assert.ok(enhanced.failures.includes('enhanced-controller-module-unobserved'));
  assert.deepEqual(descriptiveStatistics([9, 1, 7, 3]), {count: 4, min: 1, p50: 3, p95: 9, max: 9});
  assert.equal(percentile([9, 1, 7, 3], 0), 1);
});

test('unsettled and unusable responses cannot pass a descriptive observation', () => {
  const base = {profile: 'source', readiness: {complete: true}, resources: []};
  const unsettled = sampleDisposition({...base, resources: [{settled: false, status: 200, group: 'document'}]});
  assert.equal(unsettled.ok, false); assert.ok(unsettled.failures.includes('network-request-unsettled'));
  const rejected = sampleDisposition({...base, resources: [{settled: true, status: 404, group: 'document'}]});
  assert.equal(rejected.ok, false); assert.ok(rejected.failures.includes('http-response-unusable'));
});

test('resource failures and missing bodies refuse independently of event-error bookkeeping', () => {
  const resource = newResource({requestId: 'r', request: {url: 'http://127.0.0.1:4444/settings'}, type: 'Document'}, ['http://127.0.0.1:4444'], 'source');
  applyResponse(resource, {status: 200, headers: {'content-type': 'text/html'}});
  applyFinished(resource, {encodedDataLength: 14});
  applyBody(resource, Buffer.from('synthetic'));
  const inspect = observed => sampleDisposition({profile: 'source', readiness: {complete: true}, resources: [observed], blocked: []});
  assert.equal(inspect(resource).ok, true);
  for (const field of ['failure', 'body_failure']) {
    const result = inspect({...resource, [field]: 'synthetic failure'});
    assert.equal(result.ok, false);
    assert.ok(result.failures.includes('resource-failure'));
  }
  const unavailable = inspect({...resource, body: {availability: 'unavailable', sha256: null, decoded_body_bytes_observed: null}});
  assert.equal(unavailable.ok, false);
  assert.ok(unavailable.failures.includes('resource-body-unavailable'));
});

test('policy refusal and transport failure stay distinct and both fail closed', () => {
  const base = {profile: 'source', readiness: {complete: true}, resources: []};
  assert.deepEqual(sampleDisposition({...base, blocked: ['outside-origin']}), {ok: false, failures: ['blocked-request']});
  assert.deepEqual(sampleDisposition({...base, collectorErrors: ['net::ERR_ABORTED']}), {ok: false, failures: ['collector-event-error']});
});

test('failure facts preserve absent versus false and do not infer a cancellation actor', () => {
  assert.equal(networkFailureFacts({}).canceled, null);
  assert.equal(networkFailureFacts({canceled: false}).canceled, false);
  const facts = networkFailureFacts({canceled: true, blockedReason: 'inspector', type: 'Fetch', corsErrorStatus: {corsError: 'InvalidAllowOriginValue', failedParameter: 'synthetic'}});
  assert.equal(facts.canceled, true);
  assert.deepEqual(facts.blocked_reason, {value: 'inspector', truncated: false});
  assert.equal(facts.cors_error.value, 'InvalidAllowOriginValue');
  assert.equal(facts.cors_failed_parameter.value, 'synthetic');
  assert.equal(facts.actor, undefined);
});

test('request diagnostics allowlist purpose headers and bound initiator metadata explicitly', () => {
  const facts = requestDiagnostics({request: {headers: {RSC: '1', 'Next-Router-Prefetch': '1', 'Next-Router-State-Tree': 'private-state', Authorization: 'secret', Cookie: 'secret', purpose: 'x'.repeat(300)}}, initiator: {type: 'script', stack: {callFrames: Array.from({length: 9}, () => ({functionName: 'f', url: 'http://127.0.0.1:4444/a.js', lineNumber: 0, columnNumber: 1}))}}});
  assert.deepEqual(Object.keys(facts.purpose_headers).sort(), ['next-router-prefetch', 'purpose', 'rsc']);
  assert.equal(facts.purpose_headers.purpose.truncated, true);
  assert.equal(facts.purpose_headers.purpose.value.length, 256);
  assert.equal(facts.initiator_frames.length, 8);
  assert.equal(facts.initiator_frames[0].line, 0);
  assert.equal(facts.initiator_stack_incomplete, true);
  assert.equal(requestDiagnostics({}).initiator_frames, null);
  assert.equal(JSON.stringify(facts).includes('secret'), false);
  assert.equal(JSON.stringify(facts).includes('private-state'), false);
});

test('cache identity receipt allowlists exact bounded response headers without body inference', () => {
  const observed = headers => {
    const resource = newResource({requestId: 'cache', request: {url: 'http://127.0.0.1:4444/a.css'}, type: 'Stylesheet'}, ['http://127.0.0.1:4444'], 'source');
    return applyResponse(resource, {status: 200, headers});
  };
  const resource = observed({
    ETag: ' W/"not-normalized" ', VARY: 'Accept-Encoding', 'CONTENT-LENGTH': '00017',
    'X-AppKit-Delivery-Experiment': 'p10-cached-static-v2', 'x-appkit-font-aliases-sha256': 'a'.repeat(64),
    Cookie: 'must-not-retain', Authorization: 'must-not-retain', 'X-Unrelated': 'must-not-retain',
  });
  assert.deepEqual(resource.cache_identity, {
    etag: {availability: 'observed', value: ' W/"not-normalized" ', truncated: false, ambiguous: false},
    vary: {availability: 'observed', value: 'Accept-Encoding', truncated: false, ambiguous: false},
    content_length: {availability: 'observed', value: '00017', truncated: false, ambiguous: false},
    delivery_experiment: {availability: 'observed', value: 'p10-cached-static-v2', truncated: false, ambiguous: false},
    font_aliases_sha256: {availability: 'observed', value: 'a'.repeat(64), truncated: false, ambiguous: false},
  });
  assert.deepEqual(resource.body, {availability: 'unavailable', sha256: null, decoded_body_bytes_observed: null});
  assert.equal(JSON.stringify(resource.cache_identity).includes('must-not-retain'), false);

  const missing = observed({});
  for (const value of Object.values(missing.cache_identity)) assert.deepEqual(value, {availability: 'absent', value: null, truncated: false, ambiguous: false});

  const oversized = observed({etag: 'x'.repeat(513), vary: 42});
  assert.deepEqual(oversized.cache_identity.etag, {availability: 'truncated', value: null, truncated: true, ambiguous: false});
  assert.deepEqual(oversized.cache_identity.vary, {availability: 'malformed', value: null, truncated: false, ambiguous: false});

  const ambiguous = observed({etag: '"one"', ETag: '"two"', 'content-length': '1', 'Content-Length': '1'});
  assert.deepEqual(ambiguous.cache_identity.etag, {availability: 'ambiguous', value: null, truncated: false, ambiguous: true});
  assert.deepEqual(ambiguous.cache_identity.content_length, {availability: 'ambiguous', value: null, truncated: false, ambiguous: true});
  assert.deepEqual(ambiguous.cache_identity.delivery_experiment, {availability: 'absent', value: null, truncated: false, ambiguous: false});
});
