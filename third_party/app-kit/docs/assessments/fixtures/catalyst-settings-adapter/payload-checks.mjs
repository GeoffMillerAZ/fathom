// Pure checks for the P10 descriptive payload/readiness collector.  These helpers
// deliberately contain no browser, filesystem, or network side effects.
import {createHash} from 'node:crypto';

export const LOOPBACK_HOST = '127.0.0.1';
export const RESOURCE_GROUPS = Object.freeze(['document', 'css', 'font', 'image', 'runtime', 'controller', 'other']);

export function exactLoopbackOrigin(value, label = 'origin') {
  let parsed;
  try { parsed = new URL(value); } catch { throw Error(`${label} must be an absolute URL`); }
  if (parsed.origin !== value || parsed.protocol !== 'http:' || parsed.hostname !== LOOPBACK_HOST || !parsed.port || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    throw Error(`${label} must be an exact http://127.0.0.1:PORT origin`);
  }
  return parsed.origin;
}

export function assertAllowedURL(value, origins) {
  const url = new URL(value);
  if (!origins.includes(url.origin)) throw Error(`Outside-origin request refused: ${url.href}`);
  return url;
}

const contentType = headers => String(headers?.['content-type'] ?? headers?.['Content-Type'] ?? '').split(';', 1)[0].trim().toLowerCase();
const extension = pathname => pathname.split('/').pop()?.split('?')[0]?.toLowerCase() ?? '';

export function classifyResource({url, resourceType, headers, profile}) {
  const parsed = new URL(url);
  const mime = contentType(headers), ext = extension(parsed.pathname), type = String(resourceType ?? '').toLowerCase();
  if (type === 'document' || mime === 'text/html') return 'document';
  if (type === 'stylesheet' || mime === 'text/css' || ext.endsWith('.css')) return 'css';
  if (type === 'font' || mime.startsWith('font/') || /\.(woff2?|ttf|otf)$/.test(ext)) return 'font';
  if (type === 'image' || mime.startsWith('image/') || /\.(png|jpe?g|gif|webp|avif|svg)$/.test(ext)) return 'image';
  if (parsed.pathname === '/controller.mjs') return 'controller';
  if (type === 'script' || /(?:javascript|ecmascript)/.test(mime) || /\.(m?js)$/.test(ext)) return 'runtime';
  return 'other';
}

export function cacheProvenance(response = {}, servedFromCache = false) {
  if (response.fromServiceWorker) return 'service-worker';
  if (response.fromPrefetchCache) return 'prefetch-cache';
  if (response.fromDiskCache) return 'disk-cache';
  if (servedFromCache) return 'cache-observed-unspecified';
  return 'unknown';
}

export function byteObservation({encodedDataLength, decodedBodyBytes} = {}) {
  return {
    // CDP's encodedDataLength is retained as an instrument value. It must not be
    // relabelled as a payload body or used to manufacture a gzip calculation.
    encoded_transfer_bytes: Number.isFinite(encodedDataLength) && encodedDataLength >= 0 ? encodedDataLength : null,
    decoded_body_bytes_observed: Number.isFinite(decodedBodyBytes) && decodedBodyBytes >= 0 ? decodedBodyBytes : null,
  };
}

export function bodyObservation(body) {
  if (!(body instanceof Uint8Array)) return {availability: 'unavailable', sha256: null, decoded_body_bytes_observed: null};
  return {availability: 'observed', sha256: createHash('sha256').update(body).digest('hex'), decoded_body_bytes_observed: body.byteLength};
}

const boundedFact = (value, limit = 512) => typeof value === 'string'
  ? {value: value.slice(0, limit), truncated: value.length > limit} : {value: null, truncated: false};

const CACHE_IDENTITY_HEADERS = Object.freeze([
  ['etag', 'etag'],
  ['vary', 'vary'],
  ['content-length', 'content_length'],
  ['x-appkit-delivery-experiment', 'delivery_experiment'],
  ['x-appkit-font-aliases-sha256', 'font_aliases_sha256'],
]);
const CACHE_IDENTITY_VALUE_LIMIT = 512;

// This is a deliberately tiny receipt allowlist, not a raw-response-header
// capture. Values below the bound remain byte-for-byte strings for a later
// finite comparison; oversized values are not partially retained.
function cacheIdentityMetadata(headers) {
  const values = Object.entries(headers ?? {});
  const metadata = {};
  for (const [header, field] of CACHE_IDENTITY_HEADERS) {
    const matches = values.filter(([name]) => String(name).toLowerCase() === header);
    if (matches.length === 0) {
      metadata[field] = {availability: 'absent', value: null, truncated: false, ambiguous: false};
      continue;
    }
    if (matches.length !== 1) {
      metadata[field] = {availability: 'ambiguous', value: null, truncated: false, ambiguous: true};
      continue;
    }
    const value = matches[0][1];
    if (typeof value !== 'string') {
      metadata[field] = {availability: 'malformed', value: null, truncated: false, ambiguous: false};
      continue;
    }
    if (value.length > CACHE_IDENTITY_VALUE_LIMIT) {
      metadata[field] = {availability: 'truncated', value: null, truncated: true, ambiguous: false};
      continue;
    }
    metadata[field] = {availability: 'observed', value, truncated: false, ambiguous: false};
  }
  return metadata;
}

export function requestDiagnostics(request) {
  // Retain only purpose metadata, never Cookie, Authorization, router state or bodies.
  const purposeHeaders = {};
  for (const [name, value] of Object.entries(request.request?.headers ?? {})) {
    const key = name.toLowerCase();
    if (['rsc', 'next-router-prefetch', 'next-router-segment-prefetch', 'purpose', 'sec-purpose'].includes(key)) purposeHeaders[key] = boundedFact(value, 256);
  }
  const initiator = request.initiator ?? {};
  const frames = initiator.stack?.callFrames;
  return {
    purpose_headers: purposeHeaders,
    initiator_type: boundedFact(initiator.type, 64),
    initiator_url: boundedFact(initiator.url),
    initiator_frames: Array.isArray(frames) ? frames.slice(0, 8).map(frame => ({
      function_name: boundedFact(frame.functionName, 128), url: boundedFact(frame.url),
      line: Number.isInteger(frame.lineNumber) ? frame.lineNumber : null,
      column: Number.isInteger(frame.columnNumber) ? frame.columnNumber : null,
    })) : null,
    initiator_stack_incomplete: !Array.isArray(frames) || frames.length > 8 || !!initiator.stack?.parent || !!initiator.stack?.parentId,
  };
}

export function networkFailureFacts(event) {
  return {
    canceled: typeof event.canceled === 'boolean' ? event.canceled : null,
    blocked_reason: boundedFact(event.blockedReason, 128),
    cors_error: boundedFact(event.corsErrorStatus?.corsError, 128),
    cors_failed_parameter: boundedFact(event.corsErrorStatus?.failedParameter, 256),
    resource_type: boundedFact(event.type, 64),
  };
}

export function newResource(request, origins, profile) {
  const wireRequest = request.request ?? request;
  const url = assertAllowedURL(wireRequest.url, origins);
  return {
    request_id: request.requestId,
    url: url.href,
    method: wireRequest.method ?? 'GET',
    resource_type: request.type ?? request.resourceType ?? 'Other',
    request_diagnostics: requestDiagnostics(request),
    profile,
    status: null,
    mime_type: null,
    content_encoding: null,
    cache_identity: cacheIdentityMetadata(),
    cache_provenance: 'unknown',
    group: 'other',
    settled: false,
    ...byteObservation(),
    body: bodyObservation(),
    failure: null,
    failure_details: null,
  };
}

export function applyResponse(resource, response, servedFromCache = false) {
  resource.status = response.status ?? null;
  resource.mime_type = response.mimeType ?? contentType(response.headers);
  resource.content_encoding = response.headers?.['content-encoding'] ?? response.headers?.['Content-Encoding'] ?? null;
  resource.cache_control = response.headers?.['cache-control'] ?? response.headers?.['Cache-Control'] ?? null;
  resource.cache_identity = cacheIdentityMetadata(response.headers);
  resource.cache_provenance = cacheProvenance(response, servedFromCache);
  resource.group = classifyResource({url: resource.url, resourceType: resource.resource_type, headers: response.headers, profile: resource.profile});
  return resource;
}

export function applyFinished(resource, finished) {
  Object.assign(resource, byteObservation({encodedDataLength: finished.encodedDataLength, decodedBodyBytes: resource.body.decoded_body_bytes_observed}));
  resource.settled = true;
  return resource;
}

export function applyBody(resource, body) {
  resource.body = bodyObservation(body);
  Object.assign(resource, byteObservation({encodedDataLength: resource.encoded_transfer_bytes, decodedBodyBytes: resource.body.decoded_body_bytes_observed}));
  return resource;
}

export function summarizeResources(resources) {
  const groups = Object.fromEntries(RESOURCE_GROUPS.map(group => [group, {
    count: 0, encoded_transfer_bytes: 0, encoded_transfer_bytes_unknown: 0,
    decoded_body_bytes_observed: 0, decoded_body_bytes_unknown: 0, body_unavailable: 0,
  }]));
  for (const resource of resources) {
    const target = groups[RESOURCE_GROUPS.includes(resource.group) ? resource.group : 'other'];
    target.count++;
    if (resource.encoded_transfer_bytes !== null) target.encoded_transfer_bytes += resource.encoded_transfer_bytes;
    else target.encoded_transfer_bytes_unknown++;
    if (resource.decoded_body_bytes_observed !== null) target.decoded_body_bytes_observed += resource.decoded_body_bytes_observed;
    else target.decoded_body_bytes_unknown++;
    if (resource.body?.availability !== 'observed') target.body_unavailable++;
  }
  return groups;
}

export function warmCacheObservation(resources) {
  const hits = resources.filter(resource => resource.cache_provenance !== 'unknown');
  return {
    status: hits.length ? 'observed-cache-hit' : 'not-observed',
    observed_hits: hits.map(resource => ({url: resource.url, group: resource.group, provenance: resource.cache_provenance})),
  };
}

export function percentile(values, p) {
  if (!Array.isArray(values) || !Number.isFinite(p) || p < 0 || p > 1) throw Error('Finite percentile in [0,1] required');
  const sorted = values.filter(Number.isFinite).sort((a, b) => a - b);
  if (!sorted.length) return null;
  return sorted[Math.max(0, Math.min(sorted.length - 1, Math.ceil(p * sorted.length) - 1))];
}

export function descriptiveStatistics(values) {
  const finite = values.filter(Number.isFinite);
  return {count: finite.length, min: finite.length ? Math.min(...finite) : null, p50: percentile(finite, 0.5), p95: percentile(finite, 0.95), max: finite.length ? Math.max(...finite) : null};
}

export function sampleDisposition({blocked = [], collectorErrors = [], pageErrors = [], readiness = null, resources = [], profile}) {
  const failures = [];
  if (blocked.length) failures.push('blocked-request');
  if (collectorErrors.length) failures.push('collector-event-error');
  if (pageErrors.length) failures.push('page-error');
  if (!readiness?.complete) failures.push('form-readiness-incomplete');
  if (resources.some(resource => !resource.settled)) failures.push('network-request-unsettled');
  if (resources.some(resource => resource.status === null || resource.status >= 400)) failures.push('http-response-unusable');
  // A receipt must reject its own failed resource facts, even if a caller lost
  // the separate event-error array while finalizing asynchronous observations.
  if (resources.some(resource => resource.failure || resource.body_failure)) failures.push('resource-failure');
  if (resources.some(resource => resource.body?.availability !== 'observed')) failures.push('resource-body-unavailable');
  if (profile === 'native' && readiness && (!readiness.javascript_disabled || readiness.application_script_elements !== 0 || resources.some(resource => ['runtime', 'controller'].includes(resource.group)))) failures.push('native-application-javascript-observed');
  if (profile === 'enhanced' && readiness && !readiness.enhanced_ready) failures.push('enhanced-controller-not-ready');
  if (profile === 'enhanced' && !resources.some(resource => resource.group === 'controller' && resource.status >= 200 && resource.status < 300)) failures.push('enhanced-controller-module-unobserved');
  return {ok: failures.length === 0, failures};
}
