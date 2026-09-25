// Supplemental, pure receipt check. Never repairs a strict failed sample, copies
// prime bytes into a warm resource, or establishes direct warm-body observation.
import assert from 'node:assert/strict';
import {exactLoopbackOrigin} from './payload-checks.mjs';

export const WARM_IDENTITY_ID = 'p10-warm-font-validator-link-v1';
export const PLAYWRIGHT_WRAPPER_SHA256 = '549070af3acabb3efcc4f55bfe6210f9f7c2fcf633cf7eaa59bfe60719969171';
export const MISSING_BODY_MESSAGE = 'cdpSession.send: Protocol error (Network.getResponseBody): No data found for resource with given identifier';
const SHA = /^[a-f0-9]{64}$/;
const STRONG_ETAG = /^"[a-f0-9]{64}"$/;
const UUID = /^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/;
const ensure = (value, message) => assert.ok(value, `warm identity: ${message}`);
const exactID = value => typeof value === 'string' && value.length > 0 && value.length <= 256 && !/[\x00-\x20\x7f]/.test(value);
const single = (values, message) => { ensure(values.length === 1, message); return values[0]; };
const equal = (actual, expected, message) => ensure(actual === expected, message);
const exactFact = (fact, expected) => fact && Object.keys(fact).sort().join(',') === 'truncated,value' && fact.truncated === false && fact.value === expected;

function observed(field, label) {
  ensure(field && Object.keys(field).sort().join(',') === 'ambiguous,availability,truncated,value' && field.availability === 'observed' && field.truncated === false && field.ambiguous === false && typeof field.value === 'string' && field.value.length > 0 && field.value.length <= 512, `${label} is not an exact observed header`);
  return field.value;
}

function representation(resource, transport) {
  const fields = resource.cache_identity;
  const etag = observed(fields?.etag, 'ETag');
  ensure(STRONG_ETAG.test(etag), 'strong SHA ETag required');
  const length = observed(fields?.content_length, 'Content-Length');
  ensure(/^[1-9][0-9]*$/.test(length) && Number(length) <= 16 * 1024 * 1024, 'bounded canonical encoded length');
  equal(observed(fields?.vary, 'Vary'), 'Accept-Encoding', 'selected Vary differs');
  equal(observed(fields?.delivery_experiment, 'experiment'), transport.policy, 'selected experiment differs');
  equal(observed(fields?.font_aliases_sha256, 'alias manifest'), transport.aliasSHA, 'selected alias manifest differs');
  ensure(resource.content_encoding === 'gzip' || resource.content_encoding === null, 'only explicit gzip or unencoded response');
  equal(resource.mime_type, 'font/woff2', 'selected font MIME differs');
  equal(resource.cache_control, 'private, max-age=60', 'selected cache policy differs');
  return {etag, encoding: resource.content_encoding ?? 'identity', encoded_bytes: Number(length)};
}

export function exactMissingBodyError(resource, toolchain) {
  equal(toolchain?.playwright_version, '1.63.0', 'required Playwright version unavailable');
  equal(toolchain?.wrapper_sha256, PLAYWRIGHT_WRAPPER_SHA256, 'required Playwright wrapper pin unavailable');
  const error = resource.body_error;
  ensure(error && Object.keys(error).sort().join(',') === 'code,command,message,name,source', 'body error schema differs');
  equal(error?.source, 'playwright-cdp', 'body error source unavailable');
  equal(error?.command, 'Network.getResponseBody', 'body error command differs');
  ensure(exactFact(error.name, 'Error'), 'exact body error name unavailable');
  ensure(exactFact(error.message, MISSING_BODY_MESSAGE), 'exact missing-data message unavailable');
  const code = error.code;
  ensure(code && Object.keys(code).sort().join(',') === 'availability,value' && ((code.availability === 'absent' && code.value === null) || (code.availability === 'observed' && Number.isSafeInteger(code.value))), 'protocol code metadata malformed');
  equal(resource.body_failure, `Error: ${MISSING_BODY_MESSAGE}`, 'raw body error differs');
}

// All inputs are prospective observations from ONE pinned smoke acquisition.
// This does not authenticate a hand-authored receipt; the runner supplies source,
// tool, process and raw-HTTP evidence. No filesystem/network fallback exists here.
export function auditWarmFontIdentity({sample, assets, http, origin, host, transport, toolchain}) {
  exactLoopbackOrigin(origin);
  ensure(['source', 'target'].includes(host), 'host selection required');
  ensure(host === 'source' ? sample?.profile === 'source' : ['native', 'enhanced'].includes(sample?.profile), 'sample profile and HTTP host differ');
  equal(sample?.kind, 'warm', 'warm observation required');
  equal(sample.ok, false, 'strict missing-body failure must remain false');
  ensure(sample.readiness?.complete === true, 'form readiness unavailable');
  ensure(UUID.test(sample.collection_id), 'isolated collection identity unavailable');
  const prime = sample.warm_prime_observation;
  equal(prime?.collection_id, sample.collection_id, 'prime context differs');
  ensure(transport?.policy === 'p10-cached-static-v2' && SHA.test(transport.aliasSHA), 'explicit v2 transport identity required');
  ensure(Array.isArray(assets) && assets.length > 0 && assets.length <= 81, 'bounded selected assets required');
  ensure(Array.isArray(http) && http.length > 0 && http.length <= 1296, 'bounded raw HTTP evidence required');
  const pins = new Map();
  for (const asset of assets) {
    ensure(typeof asset.url === 'string' && asset.url.startsWith('/') && !asset.url.startsWith('//') && SHA.test(asset.sha256) && Number.isSafeInteger(asset.bytes) && asset.bytes > 0 && asset.bytes <= 16 * 1024 * 1024, 'asset identity invalid');
    ensure(!pins.has(origin + asset.url), 'duplicate selected asset URL');
    pins.set(origin + asset.url, asset);
  }
  for (const resources of [sample.resources, prime?.resources]) {
    ensure(Array.isArray(resources) && resources.length > 0 && resources.length <= 512, 'bounded complete resource slice required');
    const ids = new Set();
    for (const resource of resources) {
      ensure(exactID(resource.request_id) && !ids.has(resource.request_id), 'ambiguous resource request identity'); ids.add(resource.request_id);
      ensure(typeof resource.url === 'string' && new URL(resource.url).origin === origin, 'resource escaped selected origin');
      ensure(resource.settled === true && [200, 204].includes(resource.status) && resource.method === 'GET' && !resource.failure && !resource.failure_details, 'resource transport failure or unsupported response');
    }
  }
  for (const key of ['blocked', 'page_errors']) ensure(Array.isArray(sample[key]) && sample[key].length === 0, `${key} must remain empty`);
  const candidates = sample.resources.filter(resource => resource.body?.availability !== 'observed');
  ensure(candidates.length > 0 && candidates.length <= 38, 'bounded unavailable-font candidates required');
  const candidateSet = new Set(candidates);
  for (const resource of [...prime.resources, ...sample.resources]) {
    if (candidateSet.has(resource)) continue;
    ensure(!resource.body_failure && resource.body?.availability === 'observed', 'unrelated or prime body failure');
    const pin = pins.get(resource.url);
    if (pin) ensure(resource.body.sha256 === pin.sha256 && resource.body.decoded_body_bytes_observed === pin.bytes && resource.mime_type === pin.mime, 'observed selected asset drift');
  }
  ensure(Array.isArray(sample.collector_errors), 'collector error list unavailable');
  const expectedErrors = candidates.map(resource => `Response body unavailable for ${resource.url}: ${resource.body_failure}`);
  assert.deepEqual([...sample.collector_errors].sort(), expectedErrors.sort(), 'warm identity: unrelated collector error');
  const requiredFailures = ['collector-event-error', 'resource-failure', 'resource-body-unavailable'];
  const permittedFailures = new Set([...requiredFailures, 'cache-contract: cached static check: sample must be an admitted successful observation']);
  ensure(Array.isArray(sample.failures) && requiredFailures.every(value => sample.failures.includes(value)) && sample.failures.every(value => permittedFailures.has(value)), 'unrelated or inconsistent strict failure');

  const timeline = sample.timeline;
  ensure(Array.isArray(timeline) && timeline.length > 0 && timeline.length <= 8192, 'bounded event timeline required');
  const events = new Map(); let lastMS = -Infinity;
  timeline.forEach((event, index) => {
    ensure(event.sequence === index + 1 && Number.isFinite(event.node_monotonic_ms) && event.node_monotonic_ms >= lastMS, 'event chronology drift');
    events.set(event.sequence, event); lastMS = event.node_monotonic_ms;
  });
  const marker = name => single(timeline.filter(event => event.kind === name && event.marker === true), `missing/duplicate ${name}`).sequence;
  const primeStart = marker('warm-prime-start'), cleared = marker('warm-prime-resources-cleared'), reload = marker('warm-reload-start'), close = marker('context-close-start');
  ensure(primeStart < cleared && cleared < reload && reload < close, 'prime/reload phase boundaries differ');
  ensure(!timeline.some(event => event.kind === 'Network.loadingFailed'), 'network failure remains in event timeline');

  function eventFor(resource, field, kind) {
    const sequence = resource.lineage?.[field], event = events.get(sequence);
    ensure(Number.isSafeInteger(sequence) && event?.kind === kind && event.request_id === resource.request_id && event.phase === resource.lineage.phase, `${kind} correlation unavailable`);
    return sequence;
  }
  function lineage(resource, phase, lower, upper) {
    const value = resource.lineage;
    ensure(value?.collection_id === sample.collection_id && value.phase === phase && exactID(value.frame_id) && exactID(value.loader_id), 'context/frame/epoch lineage unavailable');
    const request = eventFor(resource, 'request_sequence', 'Network.requestWillBeSent');
    const response = eventFor(resource, 'response_sequence', 'Network.responseReceived');
    const finished = eventFor(resource, 'finished_sequence', 'Network.loadingFinished');
    ensure(lower < request && request < response && response < finished && finished < upper, 'request crossed navigation boundary');
    return {request, response, finished};
  }
  const primeDocument = single(prime.resources.filter(resource => resource.group === 'document'), 'prime navigation ambiguous');
  const warmDocument = single(sample.resources.filter(resource => resource.group === 'document'), 'warm navigation ambiguous');
  equal(primeDocument.url, sample.route, 'prime navigation route differs');
  equal(warmDocument.url, sample.route, 'warm navigation route differs');
  const primeNav = lineage(primeDocument, 'warm-prime', primeStart, cleared);
  const warmNav = lineage(warmDocument, 'warm-reload', reload, close);
  ensure(primeDocument.lineage.frame_id === warmDocument.lineage.frame_id && primeDocument.lineage.loader_id !== warmDocument.lineage.loader_id, 'reload must preserve frame and change navigation epoch');
  const linked = [];
  const linkedURLs = new Set();
  for (const warm of candidates) {
    const pin = pins.get(warm.url);
    ensure(pin?.mime === 'font/woff2' && warm.group === 'font' && !linkedURLs.has(warm.url), 'unavailable resource is not one unique selected WOFF2'); linkedURLs.add(warm.url);
    ensure(warm.body.availability === 'unavailable' && warm.body.sha256 === null && warm.body.decoded_body_bytes_observed === null && warm.decoded_body_bytes_observed === null, 'warm bytes were substituted');
    exactMissingBodyError(warm, toolchain);
    const before = single(prime.resources.filter(resource => resource.url === warm.url), 'prime font candidate ambiguous');
    ensure(before.status === 200 && warm.status === 200, 'linked fonts require successful GET200 responses');
    ensure(before.body.sha256 === pin.sha256 && before.body.decoded_body_bytes_observed === pin.bytes && before.mime_type === pin.mime, 'prime font differs from decoded asset pin');
    const primeEvents = lineage(before, 'warm-prime', primeStart, cleared), warmEvents = lineage(warm, 'warm-reload', reload, close);
    ensure(before.request_id !== warm.request_id && primeEvents.finished < warmEvents.request, 'prime and warm request identity/order differ');
    for (const [font, document, fontEvents, nav] of [[before, primeDocument, primeEvents, primeNav], [warm, warmDocument, warmEvents, warmNav]]) {
      ensure(font.lineage.frame_id === document.lineage.frame_id && font.lineage.loader_id === document.lineage.loader_id && nav.request < fontEvents.request, 'font does not belong to its navigation');
      ensure(font.cache_flags?.from_service_worker === false && font.cache_flags.from_prefetch_cache === false, 'worker/prefetch response cannot qualify');
    }
    equal(warm.served_from_cache_event, true, 'actual cache event unavailable');
    ensure(['cache-observed-unspecified', 'disk-cache'].includes(warm.cache_provenance), 'selected cache provenance differs');
    const cacheSequence = eventFor(warm, 'cache_event_sequence', 'Network.requestServedFromCache');
    ensure(warmEvents.request < cacheSequence && cacheSequence < warmEvents.finished, 'cache event outside request lifetime');
    const current = representation(warm, transport), previous = representation(before, transport);
    assert.deepEqual(current, previous, 'warm identity: prime/warm representation differs');
    const probe = single(http.filter(row => row.host === host && row.url === pin.url && row.method === 'GET' && row.conditional === false && row.encoding === current.encoding), 'same-encoding HTTP probe ambiguous');
    const result = probe.observed;
    ensure(result?.status === 200 && result.method === 'GET' && result.conditional === false && result.encoding === current.encoding && result.url === pin.url && result.decoded_asset_sha256 === pin.sha256, 'HTTP probe does not bind decoded asset');
    ensure(result.etag === current.etag && result.content_length === current.encoded_bytes && result.content_type === pin.mime && result.cache_control === warm.cache_control && result.vary === 'Accept-Encoding', 'HTTP representation metadata differs');
    ensure(probe.delivery?.experiment === transport.policy && probe.delivery.alias_manifest_sha256 === transport.aliasSHA, 'HTTP transport metadata differs');
    linked.push({url: warm.url, decoded_asset_sha256: pin.sha256, prime_request_id: before.request_id, warm_request_id: warm.request_id, prime_decoded_bytes_observed: pin.bytes, ...current,
      direct_warm_body_observed: false, basis: 'observed prime body, same-navigation-frame cache event and matching strong representation validators'});
  }
  return {schema_version: 1, id: WARM_IDENTITY_ID, status: 'validator-linked-identity-only', strict_sample_ok: sample.ok, direct_warm_body_observed: false, linked_fonts: linked,
    not_claimed: ['direct warm decoded bytes', 'complete cache or delivery proof', 'performance gate', 'target certification']};
}
