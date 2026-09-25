import assert from 'node:assert/strict';
import test from 'node:test';
import {closeContextWithCollection, installCDPCollection, oneObservation} from './payload-browser.mjs';

class FakeCDP {
  constructor(bodyErrors = new Map()) { this.handlers = new Map(); this.bodyErrors = bodyErrors; }
  async send(method, argument) {
    if (method === 'Network.getResponseBody' && this.bodyErrors.has(argument.requestId)) throw this.bodyErrors.get(argument.requestId);
    if (method === 'Network.getResponseBody') return {body: argument.requestId, base64Encoded: false};
    return {};
  }
  on(name, handler) { this.handlers.set(name, handler); }
  off(name, handler) { if (this.handlers.get(name) === handler) this.handlers.delete(name); }
  emit(name, event) { this.handlers.get(name)?.(event); }
}

const request = (requestId, url, type = 'Document', timestamp = 1, extra = {}) => ({
  requestId, timestamp, type, request: {url, method: 'GET'}, ...extra,
});
const response = (requestId, timestamp = 2, extra = {}) => ({
  requestId, timestamp, response: {status: 200, mimeType: 'text/html', headers: {'content-type': 'text/html'}, ...extra},
});
const tick = () => new Promise(resolve => setImmediate(resolve));

test('collector receipt retains pre-close and teardown failures with received-phase timeline evidence', async () => {
  const cdp = new FakeCDP(), failures = [];
  const collection = await installCDPCollection(cdp, {
    origins: ['http://127.0.0.1:4444'], profile: 'source', phase: 'cold-navigation', failures,
  });
  cdp.emit('Network.requestWillBeSent', request('pre-failed', 'http://127.0.0.1:4444/pre-failed', 'Document', 10, {frameId: 7, loaderId: 'x'.repeat(257)}));
  await tick();
  cdp.emit('Network.loadingFailed', {requestId: 'pre-failed', timestamp: 11, errorText: 'ERR_ABORTED'});
  cdp.emit('Network.requestWillBeSent', request('completed', 'http://127.0.0.1:4444/completed', 'Document', 12));
  await tick();
  cdp.emit('Network.responseReceived', response('completed', 13));
  cdp.emit('Network.requestServedFromCache', {requestId: 'completed', timestamp: 14});
  cdp.emit('Network.loadingFinished', {requestId: 'completed', timestamp: 15, encodedDataLength: 9});
  collection.marker('readiness-complete');
  await collection.finish('test-observation');

  const context = {async close() {
    cdp.emit('Network.requestWillBeSent', request('teardown-failed', 'http://127.0.0.1:4444/teardown-failed', 'Image', 20));
    await tick();
    cdp.emit('Network.loadingFailed', {requestId: 'teardown-failed', timestamp: 21, errorText: 'ERR_ABORTED'});
  }};
  const receipt = await closeContextWithCollection(context, collection, failures);

  assert.deepEqual(receipt.resources.map(resource => resource.request_id), ['completed', 'pre-failed', 'teardown-failed']);
  const preFailed = receipt.resources.find(resource => resource.request_id === 'pre-failed');
  assert.equal(preFailed.failure, 'ERR_ABORTED');
  assert.equal(preFailed.lineage.frame_id, null);
  assert.equal(preFailed.lineage.loader_id, null);
  assert.equal(preFailed.lineage.response_sequence, null);
  assert.equal(preFailed.lineage.cache_event_sequence, null);
  assert.equal(preFailed.lineage.finished_sequence, null);
  assert.equal(receipt.resources.find(resource => resource.request_id === 'teardown-failed').failure, 'ERR_ABORTED');
  assert.equal(receipt.resources.find(resource => resource.request_id === 'teardown-failed').lineage.phase, 'context-close');
  assert.equal(receipt.resources.find(resource => resource.request_id === 'teardown-failed').encoded_transfer_bytes, null);
  assert.equal(receipt.resource_totals.other.count, 2, 'late resource remains in the post-close totals even without a response');
  const events = receipt.timeline.filter(event => event.kind.startsWith('Network.'));
  assert.equal(events.find(event => event.request_id === 'pre-failed' && event.kind === 'Network.loadingFailed').phase, 'cold-navigation');
  assert.equal(events.find(event => event.request_id === 'teardown-failed' && event.kind === 'Network.requestWillBeSent').phase, 'context-close');
  assert.equal(events.find(event => event.request_id === 'teardown-failed' && event.kind === 'Network.loadingFailed').phase, 'context-close');
  assert.equal(events.find(event => event.request_id === 'completed' && event.kind === 'Network.loadingFinished').cdp_timestamp, 15);
  assert.ok(events.every(event => Number.isFinite(event.node_monotonic_ms) && Number.isInteger(event.sequence)));
  for (const marker of ['readiness-complete', 'resource-drain-start', 'resource-quiet-achieved', 'resource-drain-end', 'context-close-start', 'context-close-end', 'post-close-receipt-finalized']) assert.ok(receipt.timeline.some(event => event.kind === marker), marker);
  assert.ok(failures.some(failure => failure.includes('pre-failed')));
  assert.ok(failures.some(failure => failure.includes('teardown-failed')));
});

test('one collection records bounded prime/reload lineage and explicit response cache flags', async () => {
  const cdp = new FakeCDP(), failures = [];
  const collection = await installCDPCollection(cdp, {
    origins: ['http://127.0.0.1:4444'], profile: 'source', phase: 'warm-prime', failures,
  });
  const frame = 'frame-1', loader = 'loader-1';
  cdp.emit('Network.requestWillBeSent', request('prime', 'http://127.0.0.1:4444/font.woff2', 'Font', 1, {frameId: frame, loaderId: loader}));
  await tick();
  cdp.emit('Network.responseReceived', response('prime', 2, {fromServiceWorker: false, fromPrefetchCache: false, fromDiskCache: false}));
  cdp.emit('Network.loadingFinished', {requestId: 'prime', timestamp: 3, encodedDataLength: 5});
  const primeResources = await collection.finish('warm-prime');
  const prime = primeResources[0];

  collection.resetResources();
  collection.setPhase('warm-reload');
  cdp.emit('Network.requestWillBeSent', request('warm', 'http://127.0.0.1:4444/font.woff2', 'Font', 4, {frameId: frame, loaderId: loader}));
  await tick();
  cdp.emit('Network.requestServedFromCache', {requestId: 'warm', timestamp: 5});
  cdp.emit('Network.responseReceived', response('warm', 6, {fromPrefetchCache: false, fromDiskCache: true}));
  cdp.emit('Network.loadingFinished', {requestId: 'warm', timestamp: 7, encodedDataLength: 0});
  await collection.finish('warm-reload');
  const snapshot = collection.snapshot(), warm = snapshot.resources[0];

  assert.match(collection.id, /^[0-9a-f-]{36}$/);
  assert.equal(snapshot.collection_id, collection.id);
  assert.deepEqual(prime.lineage, {
    collection_id: collection.id, phase: 'warm-prime', frame_id: frame, loader_id: loader,
    request_sequence: prime.lineage.request_sequence, response_sequence: prime.lineage.response_sequence,
    cache_event_sequence: null, finished_sequence: prime.lineage.finished_sequence,
  });
  assert.ok(prime.lineage.request_sequence < prime.lineage.response_sequence && prime.lineage.response_sequence < prime.lineage.finished_sequence);
  assert.equal(warm.lineage.collection_id, collection.id);
  assert.equal(warm.lineage.phase, 'warm-reload');
  assert.equal(warm.lineage.frame_id, frame);
  assert.equal(warm.lineage.loader_id, loader);
  assert.ok(warm.lineage.request_sequence < warm.lineage.cache_event_sequence && warm.lineage.cache_event_sequence < warm.lineage.response_sequence && warm.lineage.response_sequence < warm.lineage.finished_sequence);
  assert.deepEqual(prime.cache_flags, {from_service_worker: false, from_prefetch_cache: false, from_disk_cache: false});
  assert.deepEqual(warm.cache_flags, {from_service_worker: null, from_prefetch_cache: false, from_disk_cache: true});
  assert.equal(failures.length, 0);
});

test('missing response bodies retain bounded structured CDP error facts without changing failures', async () => {
  const absentCode = new Error('No data found for resource with given identifier');
  absentCode.name = 'ProtocolError';
  const malformedCode = new Error('m'.repeat(513));
  malformedCode.name = 'n'.repeat(65); malformedCode.code = '-32000';
  const observedCode = new Error('bounded protocol failure');
  observedCode.name = 'ProtocolError'; observedCode.code = -32000;
  const cdp = new FakeCDP(new Map([['absent', absentCode], ['malformed', malformedCode], ['observed', observedCode]])), failures = [];
  const collection = await installCDPCollection(cdp, {
    origins: ['http://127.0.0.1:4444'], profile: 'source', phase: 'warm-reload', failures,
  });
  for (const [index, requestId] of ['absent', 'malformed', 'observed'].entries()) {
    cdp.emit('Network.requestWillBeSent', request(requestId, `http://127.0.0.1:4444/${requestId}.woff2`, 'Font', index * 3 + 1));
    await tick();
    cdp.emit('Network.responseReceived', response(requestId, index * 3 + 2));
    cdp.emit('Network.loadingFinished', {requestId, timestamp: index * 3 + 3, encodedDataLength: 0});
  }
  await collection.finish('missing-body');
  const resources = collection.snapshot().resources;
  const absent = resources.find(resource => resource.request_id === 'absent');
  const malformed = resources.find(resource => resource.request_id === 'malformed');
  const observed = resources.find(resource => resource.request_id === 'observed');
  assert.equal(absent.body.availability, 'unavailable');
  assert.equal(absent.body_failure, String(absentCode));
  assert.deepEqual(absent.body_error, {
    source: 'playwright-cdp', command: 'Network.getResponseBody',
    name: {value: 'ProtocolError', truncated: false}, message: {value: 'No data found for resource with given identifier', truncated: false},
    code: {availability: 'absent', value: null},
  });
  assert.equal(malformed.body_error.name.value.length, 64);
  assert.equal(malformed.body_error.name.truncated, true);
  assert.equal(malformed.body_error.message.value.length, 512);
  assert.equal(malformed.body_error.message.truncated, true);
  assert.deepEqual(malformed.body_error.code, {availability: 'malformed', value: null});
  assert.deepEqual(observed.body_error.code, {availability: 'observed', value: -32000});
  assert.ok(failures.some(failure => failure.includes('absent.woff2')));
  assert.ok(failures.some(failure => failure.includes('malformed.woff2')));
  assert.ok(failures.some(failure => failure.includes('observed.woff2')));
});

class FakePage {
  constructor(cdp) { this.cdp = cdp; }
  setDefaultTimeout() {}
  setDefaultNavigationTimeout() {}
  on() {}
  isClosed() { return false; }
  async goto() {
    this.cdp.emit('Network.requestWillBeSent', request('document', 'http://127.0.0.1:4444/settings', 'Document', 30));
    await tick();
    this.cdp.emit('Network.responseReceived', response('document', 31));
    this.cdp.emit('Network.loadingFinished', {requestId: 'document', timestamp: 32, encodedDataLength: 8});
  }
  async reload() {
    this.cdp.emit('Network.requestWillBeSent', request('document', 'http://127.0.0.1:4444/settings', 'Document', 33));
    await tick();
    this.cdp.emit('Network.requestServedFromCache', {requestId: 'document', timestamp: 34});
    this.cdp.emit('Network.responseReceived', response('document', 35));
    this.cdp.emit('Network.loadingFinished', {requestId: 'document', timestamp: 36, encodedDataLength: 0});
  }
  async waitForLoadState() {}
  async waitForFunction() {}
  async evaluate(expression) {
    if (typeof expression === 'string') return {complete: true, form_present: true, editable_control_observation: 10, javascript_disabled: false, application_script_elements: 1, enhanced_ready: false};
    return {navigation: null, dom_observed_ms: 1, fonts_status: 'loaded', fonts_observed_ms: 2};
  }
}

class FakeObservationContext {
  constructor() { this.cdp = new FakeCDP(); this.page = new FakePage(this.cdp); }
  async routeWebSocket() {}
  async newPage() { return this.page; }
  async newCDPSession() { return this.cdp; }
  async close() {
    this.cdp.emit('Network.requestWillBeSent', request('late-close', 'http://127.0.0.1:4444/late-close', 'Image', 40));
    await tick();
    this.cdp.emit('Network.loadingFailed', {requestId: 'late-close', timestamp: 41, errorText: 'ERR_ABORTED'});
  }
}

test('oneObservation fail-closes on a context-close failure and freezes aligned receipt totals', async () => {
  const context = new FakeObservationContext();
  const browser = {async newContext() { return context; }};
  const sample = await oneObservation(browser, {
    profile: {id: 'source', route: '/settings', javaScriptEnabled: true, accounting: 'source-full-react-page-with-shell'},
    lane: {width: 1440, theme: 'light'}, sourceOrigin: 'http://127.0.0.1:4444', targetOrigin: 'http://127.0.0.1:5555',
  }, {kind: 'cold', ordinal: 1});

  assert.equal(sample.ok, false);
  assert.ok(sample.failures.includes('collector-event-error'));
  assert.ok(!sample.failures.includes('blocked-request'), 'network cancellation is not a policy refusal');
  assert.deepEqual(sample.blocked, []);
  assert.ok(sample.collector_errors.some(error => error.includes('late-close')));
  assert.ok(sample.failures.includes('resource-failure'));
  assert.ok(sample.resources.some(resource => resource.request_id === 'late-close' && resource.failure === 'ERR_ABORTED'));
  assert.equal(Object.values(sample.resource_totals).reduce((count, group) => count + group.count, 0), sample.resources.length);
  assert.ok(sample.timeline.some(event => event.kind === 'Network.loadingFailed' && event.request_id === 'late-close' && event.phase === 'context-close'));
  assert.ok(sample.timeline.some(event => event.kind === 'cdp-listener-detach-end'));
  assert.equal(context.cdp.handlers.size, 0, 'only collector listeners were detached before the frozen receipt');
});

test('oneObservation admits a complete synthetic response and remains frozen after listener shutdown', async () => {
  const context = new FakeObservationContext();
  context.close = async () => {};
  const sample = await oneObservation({async newContext() { return context; }}, {
    profile: {id: 'source', route: '/settings', javaScriptEnabled: true, accounting: 'source-full-react-page-with-shell'},
    lane: {width: 1440, theme: 'light'}, sourceOrigin: 'http://127.0.0.1:4444', targetOrigin: 'http://127.0.0.1:5555',
  }, {kind: 'cold', ordinal: 1});
  assert.equal(sample.ok, true);
  assert.deepEqual(sample.failures, []);
  assert.equal(sample.resources.length, 1);
  assert.equal(sample.resource_totals.document.encoded_transfer_bytes, 8);
  const frozen = JSON.stringify(sample);
  context.cdp.emit('Network.loadingFailed', {requestId: 'document', timestamp: 50, errorText: 'after receipt boundary'});
  await tick();
  assert.equal(JSON.stringify(sample), frozen);
});

test('warm sample and separately retained prime snapshot share exactly one collection id', async () => {
  const context = new FakeObservationContext();
  context.close = async () => {};
  const sample = await oneObservation({async newContext() { return context; }}, {
    profile: {id: 'source', route: '/settings', javaScriptEnabled: true, accounting: 'source-full-react-page-with-shell'},
    lane: {width: 1440, theme: 'light'}, sourceOrigin: 'http://127.0.0.1:4444', targetOrigin: 'http://127.0.0.1:5555',
  }, {kind: 'warm', ordinal: 1});
  assert.equal(sample.ok, true);
  assert.match(sample.collection_id, /^[0-9a-f-]{36}$/);
  assert.equal(sample.warm_prime_observation.collection_id, sample.collection_id);
  assert.ok(sample.resources.every(resource => resource.lineage.collection_id === sample.collection_id));
  assert.ok(sample.warm_prime_observation.resources.every(resource => resource.lineage.collection_id === sample.collection_id));
});
