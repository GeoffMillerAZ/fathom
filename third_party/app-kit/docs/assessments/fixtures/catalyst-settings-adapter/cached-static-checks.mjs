// Pure, local checks for the opt-in cached-static rehearsal. These checks are not
// a browser result, performance measurement, target certification, or delivery API.
import {createHash} from 'node:crypto';
import {gunzipSync} from 'node:zlib';

const sha256 = body => createHash('sha256').update(body).digest('hex');
const fail = message => { throw Error(`cached static check: ${message}`); };

function normalizedHeaders(headers) {
  if (!headers || typeof headers !== 'object') fail('headers object required');
  const normalized = {};
  for (const [name, value] of Object.entries(headers)) normalized[String(name).toLowerCase()] = Array.isArray(value) ? value.join(', ') : String(value);
  return normalized;
}

function validAsset(asset) {
  if (!asset || typeof asset.url !== 'string' || typeof asset.mime !== 'string' || !/^[a-f0-9]{64}$/.test(asset.sha256) || !Number.isInteger(asset.bytes) || asset.bytes < 0) fail('asset pin required');
  return asset;
}

function strongETag(value) { return typeof value === 'string' && /^"[a-f0-9]{64}"$/.test(value); }
function baseMIME(value) { return typeof value === 'string' ? value.split(';', 1)[0].trim().toLowerCase() : null; }
function headerLength(value) { return typeof value === 'string' && /^(?:0|[1-9][0-9]*)$/.test(value) ? Number(value) : null; }

// `body` is the selected encoded response bytes. The return value deliberately
// excludes it (and any decoded body), retaining only bounded verification metadata.
export function auditStaticResponse({asset, method, encoding, status, headers, body, conditional = false}) {
  validAsset(asset);
  if (!['GET', 'HEAD'].includes(method)) fail('method must be GET or HEAD');
  if (!(body instanceof Buffer)) fail('body must be a Buffer');
  if (status !== (conditional ? 304 : 200)) fail(`status ${status} does not match conditional=${conditional}`);
  const values = normalizedHeaders(headers);
  if (values['cache-control'] !== 'private, max-age=60') fail('cache-control must be private, max-age=60');
  if (!values.vary?.split(',').map(value => value.trim().toLowerCase()).includes('accept-encoding')) fail('Vary must include Accept-Encoding');
  if (values['x-content-type-options']?.toLowerCase() !== 'nosniff') fail('X-Content-Type-Options must be nosniff');
  if ((method !== 'GET' || status === 304) && body.length !== 0) fail('HEAD/304 must have an empty body');

  const is304 = status === 304;
  const mime = baseMIME(values['content-type']);
  const length = headerLength(values['content-length']);
  if (!is304 && mime === null) fail('content-type required');
  if (mime !== null && mime !== asset.mime) fail('content-type base MIME differs from asset pin');
  if (!is304 && length === null) fail('content-length required');
  if (!is304 && method === 'HEAD' && length <= 0) fail('HEAD content-length must be positive');
  if (is304 && length !== null && length <= 0) fail('304 content-length must be positive when present');
  if (!strongETag(values.etag)) fail('strong ETag required');

  const selectedEncoding = encoding === 'gzip' ? 'gzip' : encoding === 'identity' ? 'identity' : fail('encoding must be identity or gzip');
  if (selectedEncoding === 'gzip' && values['content-encoding']?.toLowerCase() !== 'gzip') fail('gzip response lacks Content-Encoding: gzip');
  if (selectedEncoding === 'identity' && values['content-encoding'] && values['content-encoding'].toLowerCase() !== 'identity') fail('identity response has unexpected Content-Encoding');

  if (method === 'GET' && !is304) {
    if (length !== body.length) fail('content-length differs from selected encoded body');
    if (values.etag !== `"${sha256(body)}"`) fail('ETag is not the selected encoded-body SHA-256');
    let decoded = body;
    if (selectedEncoding === 'gzip') {
      try { decoded = gunzipSync(body, {maxOutputLength: asset.bytes + 1}); } catch { fail('gzip body cannot be decoded within pinned bound'); }
    }
    if (decoded.length !== asset.bytes || sha256(decoded) !== asset.sha256) fail('decoded body differs from asset pin');
  }
  return {
    url: asset.url, method, status, conditional: Boolean(conditional), encoding: selectedEncoding,
    cache_control: values['cache-control'], vary: values.vary, etag: values.etag,
    content_type: mime, content_length: length,
    content_type_available: mime !== null, content_length_available: length !== null,
    decoded_asset_sha256: method === 'GET' && !is304 ? asset.sha256 : null,
  };
}

function assetForResource(resource, assets) {
  if (!resource || typeof resource.url !== 'string') return null;
  return assets.find(asset => resource.url === asset.url || (() => {
    try { const url = new URL(resource.url); return url.href === url.origin + asset.url; } catch { return false; }
  })()) ?? null;
}

function observedHit(resource) { return resource?.cache_provenance && resource.cache_provenance !== 'unknown'; }

// Checks the collector's already-recorded metadata. It neither obtains bodies nor
// turns matching fixture bytes into a compatibility or performance claim.
export function auditSampleAssets(sample, assets) {
  if (!sample || sample.ok !== true) fail('sample must be an admitted successful observation');
  if (!Array.isArray(sample.resources) || !Array.isArray(assets) || !assets.length) fail('resources and pinned assets required');
  const pins = assets.map(validAsset);
  const selected = [];
  for (const resource of sample.resources) {
    const asset = assetForResource(resource, pins);
    if (!asset) {
      if (resource?.cache_control !== 'no-store') fail(`unselected response lacks no-store: ${resource?.url ?? 'unknown'}`);
      continue;
    }
    if (resource.body?.availability !== 'observed' || resource.body.sha256 !== asset.sha256 || resource.body.decoded_body_bytes_observed !== asset.bytes || baseMIME(resource.mime_type) !== asset.mime) fail(`selected asset drift: ${asset.url}`);
    if (resource.cache_control !== 'private, max-age=60') fail(`selected asset cache-control drift: ${asset.url}`);
    selected.push({url: asset.url, mime: asset.mime, sha256: asset.sha256, bytes: asset.bytes, observed_hit: observedHit(resource)});
  }
  if (!selected.some(asset => asset.mime === 'text/css') || !selected.some(asset => asset.mime === 'font/woff' || asset.mime === 'font/woff2')) fail('at least one selected CSS and font asset required');
  if (sample.kind === 'warm') {
    const hits = Array.isArray(sample.cache?.observed_hits) ? sample.cache.observed_hits : [];
    if (!hits.length) fail('warm sample lacks an observed cache hit');
    for (const hit of hits) {
      const asset = assetForResource(hit, pins);
      if (!asset || !selected.some(selectedAsset => selectedAsset.url === asset.url)) fail(`warm cache hit is not a selected asset: ${hit?.url ?? 'unknown'}`);
    }
    if (!selected.some(asset => asset.observed_hit)) fail('warm sample lacks an observed selected-asset hit');
  }
  return {selected_assets: selected, selected_count: selected.length, warm_cache_hit_count: sample.kind === 'warm' ? selected.filter(asset => asset.observed_hit).length : 0};
}
