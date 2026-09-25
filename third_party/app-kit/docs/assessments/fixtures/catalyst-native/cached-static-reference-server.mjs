// Private P10 cached-static delivery experiment. This is a separately named
// listener: reference-server.mjs remains the independently runnable no-store
// baseline. It never caches documents, React runtime/chunks, RSC/route data,
// mutations, errors, or a request carrying route-data/prefetch headers.
import {createHash} from 'node:crypto';
import {constants, closeSync, fstatSync, lstatSync, openSync, readSync, realpathSync} from 'node:fs';
import {createRequire} from 'node:module';
import {gzipSync} from 'node:zlib';
import http from 'node:http';
import {isAbsolute, join, resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {refuseRequest} from './reference-server.mjs';
import {verifyHardenedReference} from '../catalyst-hardened/reference.mjs';

export const CACHED_STATIC_POLICY_ID = 'p10-cached-static-v1';
const REFERENCE_SHA256 = '94d3ab4c145e33bf29af750918429833d833fff3f9ac8820e1314c7f2c67f53d';
const SOURCE_MANIFEST_SHA256 = 'f8de11343773dbb8dbeb53bb713d684fee23fcd36604ddff921b07da57450ee2';
const PACKET_ID = 'p10-hardened-rendered-reference-v1';
const ASSET_COUNT = 43;
const MAX_PACKET_FILE_BYTES = 16 * 1024 * 1024;
const ALLOWED_MIME = new Set(['text/css', 'font/woff', 'font/woff2', 'image/svg+xml']);

const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const requireValue = (value, message) => { if (!value) throw new Error(message); };
const safeFlatName = value => typeof value === 'string' && /^[A-Za-z0-9_.-]+$/.test(value);
const safeAssetURL = value => typeof value === 'string' && /^\/[^?#\\]+$/.test(value) && !/[\\\x00-\x20\x7f%:]/.test(value) && !value.startsWith('//') && value !== '/' && !value.slice(1).split('/').some(part => part === '.' || part === '..' || part === '');
const cacheableAssetURL = value => safeAssetURL(value) &&
  !['/settings', '/favicon.ico', '/fixture.css', '/native.css', '/controller.mjs'].includes(value) &&
  !value.startsWith('/settings/') && !value.startsWith('/api/') && !value.startsWith('/_next/data/') &&
  (!value.startsWith('/_next/') || value.startsWith('/_next/static/'));

function readRegular(root, relative, maximum = MAX_PACKET_FILE_BYTES) {
  requireValue(safeFlatName(relative), 'Unsafe packet filename.');
  const selected = join(root, relative);
  requireValue(resolve(selected).startsWith(`${resolve(root)}/`), 'Packet path escapes its root.');
  const before = lstatSync(selected);
  requireValue(before.isFile() && !before.isSymbolicLink() && before.size >= 0 && before.size <= maximum, `Refused packet file: ${relative}`);
  const fd = openSync(selected, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const stat = fstatSync(fd);
    requireValue(stat.isFile() && stat.size === before.size && stat.size <= maximum, `Packet file changed: ${relative}`);
    const bytes = Buffer.alloc(stat.size + 1);
    let count = 0, read = 0;
    while (count < bytes.length && (read = readSync(fd, bytes, count, bytes.length - count, null)) > 0) count += read;
    requireValue(count === stat.size, `Packet file changed while reading: ${relative}`);
    return bytes.subarray(0, count);
  } finally { closeSync(fd); }
}

function readSourceAsset(sourceRoot, url, expected) {
  const relative = url.startsWith('/_next/') ? join('.next', url.slice('/_next/'.length)) : join('public', url.slice(1));
  const selected = resolve(sourceRoot, relative);
  requireValue(selected.startsWith(`${resolve(sourceRoot)}/`), `Source asset escapes root: ${url}`);
  const parts = relative.split('/');
  let current = sourceRoot;
  for (const part of parts) {
    current = join(current, part);
    const stat = lstatSync(current);
    requireValue(!stat.isSymbolicLink(), `Source asset symlink refused: ${url}`);
  }
  const before = lstatSync(selected);
  requireValue(before.isFile() && before.size === expected.bytes && before.size <= MAX_PACKET_FILE_BYTES, `Source asset changed: ${url}`);
  const fd = openSync(selected, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const stat = fstatSync(fd);
    requireValue(stat.isFile() && stat.size === before.size, `Source asset changed: ${url}`);
    const bytes = Buffer.alloc(stat.size + 1);
    let count = 0, read = 0;
    while (count < bytes.length && (read = readSync(fd, bytes, count, bytes.length - count, null)) > 0) count += read;
    requireValue(count === stat.size && sha256(bytes.subarray(0, count)) === expected.sha256, `Source asset digest drift: ${url}`);
    return bytes.subarray(0, count);
  } finally { closeSync(fd); }
}

// The optional expected argument exists solely so the synthetic test can use a
// temporary packet. startCachedStaticReference always supplies this fixed P10 contract.
export function readCachedStaticPacket(packetRoot, expected = {}) {
  const contract = {
    referenceSHA256: expected.referenceSHA256 ?? REFERENCE_SHA256,
    sourceManifestSHA256: expected.sourceManifestSHA256 ?? SOURCE_MANIFEST_SHA256,
    id: expected.id ?? PACKET_ID,
    assetCount: expected.assetCount ?? ASSET_COUNT,
  };
  requireValue(typeof packetRoot === 'string' && isAbsolute(packetRoot), 'An explicit absolute source packet directory is required.');
  const packet = lstatSync(packetRoot);
  requireValue(packet.isDirectory() && !packet.isSymbolicLink(), 'Source packet must be a non-symlink directory.');
  const referenceBytes = readRegular(packetRoot, 'reference.json', 1024 * 1024);
  requireValue(sha256(referenceBytes) === contract.referenceSHA256, 'Pinned source packet reference drift.');
  let reference;
  try { reference = JSON.parse(referenceBytes); } catch { throw new Error('Malformed source packet reference.'); }
  requireValue(reference?.id === contract.id && reference.schema_version === 1 && reference.source_manifest_sha256 === contract.sourceManifestSHA256, 'Source packet lineage mismatch.');
  requireValue(Array.isArray(reference.assets) && reference.assets.length === contract.assetCount, 'Source packet must contain the fixed static asset closure.');
  const urls = new Set(), names = new Set(), assets = [];
  for (const asset of reference.assets) {
    const pin = asset?.file;
    requireValue(cacheableAssetURL(asset?.url) && ALLOWED_MIME.has(asset?.mime) && safeFlatName(pin?.name) && /^[a-f0-9]{64}$/.test(pin?.sha256) && Number.isInteger(pin?.bytes) && pin.bytes > 0 && pin.bytes <= MAX_PACKET_FILE_BYTES, 'Invalid cached-static asset pin.');
    requireValue(!urls.has(asset.url) && !names.has(pin.name), 'Duplicate cached-static asset URL or file.');
    urls.add(asset.url); names.add(pin.name);
    const body = readRegular(packetRoot, pin.name);
    requireValue(body.length === pin.bytes && sha256(body) === pin.sha256, `Source packet asset drift: ${asset.url}`);
    assets.push(Object.freeze({url: asset.url, mime: asset.mime, sha256: pin.sha256, bytes: pin.bytes}));
  }
  return Object.freeze({referenceSHA256: contract.referenceSHA256, sourceManifestSHA256: contract.sourceManifestSHA256, assets: Object.freeze(assets)});
}

function requestHasRouteData(request) {
  if (request.url?.includes('?')) return true;
  return ['rsc', 'next-router-prefetch', 'next-router-segment-prefetch', 'next-router-state-tree', 'next-url', 'next-action'].some(name => request.headers[name] !== undefined);
}

function acceptedCodings(header) {
  const values = new Map();
  if (header === undefined) return values;
  for (const part of (Array.isArray(header) ? header.join(',') : String(header)).split(',')) {
    const trimmed = part.trim().toLowerCase();
    if (!trimmed) continue;
    const match = /^([!#$%&'*+.^_`|~0-9a-z-]+)(?:;\s*q=([01](?:\.\d{0,3})?))?$/.exec(trimmed);
    if (!match || (match[2]?.startsWith('1.') && !/^1\.0{0,3}$/.test(match[2]))) {
      const token = /^([!#$%&'*+.^_`|~0-9a-z-]+)/.exec(trimmed)?.[1];
      if (token) values.set(token, 0);
      continue;
    }
    values.set(match[1], Number(match[2] ?? 1));
  }
  return values;
}

function quality(codings, coding) {
  const values = codings;
  if (values.has(coding)) return values.get(coding);
  if (coding === 'identity') return values.has('*') && values.get('*') === 0 ? 0 : 1;
  return values.get('*') ?? 0;
}

export function selectEncoding(acceptEncoding) {
  const codings = acceptedCodings(acceptEncoding);
  const identity = quality(codings, 'identity'), gzip = quality(codings, 'gzip');
  if (identity <= 0 && gzip <= 0) return null;
  return gzip >= identity && gzip > 0 ? 'gzip' : 'identity';
}

export function prepareCachedStaticAssets(assets) {
  requireValue(assets instanceof Map, 'Cached static preparation requires an asset map.');
  const prepared = new Map();
  for (const [url, asset] of assets) {
    requireValue(cacheableAssetURL(url) && asset && Buffer.isBuffer(asset.body) && ALLOWED_MIME.has(asset.mime) && /^[a-f0-9]{64}$/.test(asset.sha256) && sha256(asset.body) === asset.sha256, 'Invalid prepared cached-static asset.');
    const identity = Object.freeze({body: asset.body, etag: `"${asset.sha256}"`});
    const gzip = gzipSync(asset.body, {level: 6, mtime: 0});
    prepared.set(url, Object.freeze({
      mime: asset.mime,
      identity,
      gzip: Object.freeze({body: gzip, etag: `"${sha256(gzip)}"`}),
    }));
  }
  return prepared;
}

function matchesIfNoneMatch(header, etag) {
  if (header === undefined) return false;
  for (const candidate of (Array.isArray(header) ? header.join(',') : String(header)).split(',')) {
    const value = candidate.trim();
    if (value === '*') return true;
    const weak = value.startsWith('W/') ? value.slice(2).trim() : value;
    if (/^"[^"\\]*"$/.test(weak) && weak === etag) return true;
  }
  return false;
}

export function tryServeCachedStatic(request, response, assets) {
  if (request.method !== 'GET' && request.method !== 'HEAD') return false;
  if (requestHasRouteData(request)) return false;
  const asset = assets.get(request.url);
  if (!asset) return false;
  const encoding = selectEncoding(request.headers['accept-encoding']);
  if (!encoding) {
    response.setHeader('Cache-Control', 'no-store');
    response.setHeader('Vary', 'Accept-Encoding');
    response.writeHead(406, {'Content-Type': 'text/plain; charset=utf-8'});
    response.end('No acceptable static representation.');
    return true;
  }
  const representation = asset[encoding];
  response.setHeader('Cache-Control', 'private, max-age=60');
  response.setHeader('Vary', 'Accept-Encoding');
  response.setHeader('Content-Type', asset.mime);
  response.setHeader('ETag', representation.etag);
  response.setHeader('Content-Length', String(representation.body.length));
  response.setHeader('X-Content-Type-Options', 'nosniff');
  if (encoding === 'gzip') response.setHeader('Content-Encoding', 'gzip');
  if (matchesIfNoneMatch(request.headers['if-none-match'], representation.etag)) {
    response.writeHead(304);
    response.end();
  } else {
    response.writeHead(200);
    response.end(request.method === 'HEAD' ? undefined : representation.body);
  }
  return true;
}

// Next owns its routing, but this listener owns the delivery policy. Lock the
// delegated response to no-store even if a route tries to install its own cache
// header (writeHead headers otherwise take precedence over setHeader values).
function lockDelegatedNoStore(response) {
  const setHeader = response.setHeader.bind(response), writeHead = response.writeHead.bind(response), removeHeader = response.removeHeader.bind(response);
  setHeader('Cache-Control', 'no-store');
  response.setHeader = (name, value) => setHeader(name, String(name).toLowerCase() === 'cache-control' ? 'no-store' : value);
  response.removeHeader = name => String(name).toLowerCase() === 'cache-control' ? undefined : removeHeader(name);
  response.writeHead = (status, ...args) => {
    const index = args.findIndex(value => Array.isArray(value) || (value && typeof value === 'object'));
    if (index >= 0) {
      const headers = Array.isArray(args[index]) ? (() => {
        const copied = [];
        for (let i = 0; i < args[index].length; i += 2) if (String(args[index][i]).toLowerCase() !== 'cache-control') copied.push(args[index][i], args[index][i + 1]);
        return copied;
      })() : (() => {
        const copied = {...args[index]};
        for (const name of Object.keys(copied)) if (name.toLowerCase() === 'cache-control') delete copied[name];
        return copied;
      })();
      args[index] = headers;
    } else setHeader('Cache-Control', 'no-store');
    return writeHead(status, ...args);
  };
}

export function createCachedStaticReferenceHandler({origin, assets, delegate}) {
  requireValue(origin && assets instanceof Map && typeof delegate === 'function', 'Cached listener requires origin, assets and source delegate.');
  return async (request, response) => {
    response.setHeader('Cache-Control', 'no-store');
    response.setHeader('Referrer-Policy', 'no-referrer');
    response.setHeader('X-Content-Type-Options', 'nosniff');
    response.setHeader('X-Frame-Options', 'DENY');
    response.setHeader('Content-Security-Policy', "frame-ancestors 'none'; base-uri 'self'; form-action 'self'");
    response.setHeader('X-AppKit-Reference', 'source-native-cached-static-p10-v1');
    response.setHeader('X-AppKit-Cached-Static-Policy', CACHED_STATIC_POLICY_ID);
    const refusal = refuseRequest(request, origin);
    if (refusal) {
      if (refusal[0] === 405) response.setHeader('Allow', 'GET, HEAD');
      response.writeHead(refusal[0], {'Content-Type': 'text/plain; charset=utf-8'});
      response.end(refusal[1]);
      return;
    }
    if (tryServeCachedStatic(request, response, assets)) return;
    lockDelegatedNoStore(response);
    try { await delegate(request, response); }
    catch {
      if (!response.headersSent) response.writeHead(500, {'Content-Type': 'text/plain; charset=utf-8'});
      response.end('The source reference failed to render.');
    }
  };
}

export async function startCachedStaticReference(projectRoot, packetRoot) {
  requireValue(typeof projectRoot === 'string' && isAbsolute(projectRoot), 'An explicit absolute reviewed project directory is required.');
  const packet = readCachedStaticPacket(packetRoot);
  const source = verifyHardenedReference(projectRoot);
  requireValue(source.source_manifest_sha256 === packet.sourceManifestSHA256, 'Selected source root does not match packet lineage.');
  const sourceAssets = new Map(packet.assets.map(asset => {
    const body = readSourceAsset(projectRoot, asset.url, asset);
    return [asset.url, Object.freeze({body, mime: asset.mime, sha256: asset.sha256})];
  }));
  const assets = prepareCachedStaticAssets(sourceAssets);
  const assetManifestSHA256 = sha256(Buffer.from(JSON.stringify(packet.assets.map(asset => ({url: asset.url, sha256: asset.sha256, mime: asset.mime, bytes: asset.bytes})))));
  const requireProject = createRequire(resolve(projectRoot, 'package.json'));
  const next = requireProject('next');
  let origin, handle, requestHandler;
  const server = http.createServer((request, response) => {
    if (!requestHandler) {
      response.setHeader('Cache-Control', 'no-store');
      response.writeHead(503, {'Content-Type': 'text/plain; charset=utf-8'});
      response.end('The cached source reference is preparing.');
      return;
    }
    return requestHandler(request, response);
  });
  const delegate = async (innerRequest, innerResponse) => {
      if (!handle) {
        innerResponse.writeHead(503, {'Content-Type': 'text/plain; charset=utf-8'});
        innerResponse.end('The cached source reference is preparing.');
        return;
      }
      await handle(innerRequest, innerResponse);
    };
  server.headersTimeout = 5000;
  server.requestTimeout = 10000;
  server.keepAliveTimeout = 2000;
  server.on('upgrade', (_request, socket) => socket.destroy());
  await new Promise((resolveReady, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolveReady); });
  origin = `http://127.0.0.1:${server.address().port}`;
  requestHandler = createCachedStaticReferenceHandler({origin, assets, delegate});
  const app = next({dir: projectRoot, dev: false, hostname: '127.0.0.1', port: server.address().port});
  try { await app.prepare(); handle = app.getRequestHandler(); }
  catch (error) {
    server.closeAllConnections();
    await new Promise(resolveClosed => server.close(resolveClosed));
    await app.close();
    throw error;
  }
  return {origin, policy_id: CACHED_STATIC_POLICY_ID, asset_manifest_sha256: assetManifestSHA256, close: async () => {
    const deadline = setTimeout(() => server.closeAllConnections(), 5000);
    try {
      await new Promise((resolveClosed, reject) => server.close(error => error ? reject(error) : resolveClosed()));
      await app.close();
    } finally { clearTimeout(deadline); }
  }};
}

if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
  if (process.argv.length !== 4) throw new Error('Usage: node cached-static-reference-server.mjs ABSOLUTE_REVIEWED_SOURCE_ROOT ABSOLUTE_SOURCE_PACKET');
  const reference = await startCachedStaticReference(process.argv[2], process.argv[3]);
  console.log(JSON.stringify({origin: reference.origin, policy_id: reference.policy_id, asset_manifest_sha256: reference.asset_manifest_sha256}));
  let stopping = false;
  const stop = async () => {
    if (stopping) return;
    stopping = true;
    const deadline = setTimeout(() => process.exit(1), 6000);
    try { await reference.close(); } finally { clearTimeout(deadline); }
  };
  process.on('SIGINT', stop); process.on('SIGTERM', stop);
}
