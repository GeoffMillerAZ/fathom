// Private P10 v2 font-alias delivery experiment.  It composes the unchanged v1
// static policy with a finite CSS-derived query alias set; it is not a URL
// rewriting proxy and does not alter the v1 listener.
import {createHash} from 'node:crypto';
import {constants, closeSync, fstatSync, lstatSync, openSync, readSync} from 'node:fs';
import {createRequire} from 'node:module';
import http from 'node:http';
import {isAbsolute, join, resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {deriveFontAliases} from '../catalyst-settings-adapter/font-query-aliases.mjs';
import {CACHED_STATIC_POLICY_ID, createCachedStaticReferenceHandler, prepareCachedStaticAssets, readCachedStaticPacket, tryServeCachedStatic} from './cached-static-reference-server.mjs';
import {refuseRequest} from './reference-server.mjs';
import {verifyHardenedReference} from '../catalyst-hardened/reference.mjs';

export const FONT_ALIAS_DELIVERY_EXPERIMENT = 'p10-cached-static-v2';
const MAX_PACKET_FILE_BYTES = 16 * 1024 * 1024;
const ROUTE_DATA_HEADERS = Object.freeze(['rsc', 'next-router-prefetch', 'next-router-segment-prefetch', 'next-router-state-tree', 'next-url', 'next-action']);

const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const requireValue = (value, message) => { if (!value) throw new Error(message); };
const safeAssetURL = value => typeof value === 'string' && /^\/[^?#\\]+$/.test(value) && !/[\\\x00-\x20\x7f%:]/.test(value) && !value.startsWith('//') && value !== '/' && !value.slice(1).split('/').some(part => part === '.' || part === '..' || part === '');
const safeFontAliasURL = value => typeof value === 'string' && /^\/[A-Za-z0-9_./-]+\/font-files\/[A-Za-z0-9_.-]+\.woff2\?v=4\.1$/.test(value) && !value.split(/[/?]/).some(part => part === '.' || part === '..');

function readSourceAsset(sourceRoot, url, expected) {
  const relative = url.startsWith('/_next/') ? join('.next', url.slice('/_next/'.length)) : join('public', url.slice(1));
  const selected = resolve(sourceRoot, relative);
  requireValue(selected.startsWith(`${resolve(sourceRoot)}/`), `Source asset escapes root: ${url}`);
  let current = sourceRoot;
  for (const part of relative.split('/')) {
    current = join(current, part);
    requireValue(!lstatSync(current).isSymbolicLink(), `Source asset symlink refused: ${url}`);
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

function hasRouteDataHeaders(request) {
  return ROUTE_DATA_HEADERS.some(name => request.headers[name] !== undefined);
}

function applyV1SecurityHeaders(response) {
  response.setHeader('Cache-Control', 'no-store');
  response.setHeader('Referrer-Policy', 'no-referrer');
  response.setHeader('X-Content-Type-Options', 'nosniff');
  response.setHeader('X-Frame-Options', 'DENY');
  response.setHeader('Content-Security-Policy', "frame-ancestors 'none'; base-uri 'self'; form-action 'self'");
  response.setHeader('X-AppKit-Reference', 'source-native-cached-static-p10-v1');
  response.setHeader('X-AppKit-Cached-Static-Policy', CACHED_STATIC_POLICY_ID);
}

function checkedAliasMap(manifest, assets) {
  requireValue(manifest?.schema_version === 1 && manifest.id === 'p10-font-query-aliases-v1' && safeAssetURL(manifest.css?.url) && /^[a-f0-9]{64}$/.test(manifest.css?.sha256) && Number.isInteger(manifest.css?.bytes) && manifest.css.bytes > 0 && Array.isArray(manifest.aliases), 'Invalid font alias manifest.');
  const stylesheet = assets.get(manifest.css.url);
  requireValue(stylesheet?.mime === 'text/css' && stylesheet.identity.body.length === manifest.css.bytes && stylesheet.identity.etag === `"${manifest.css.sha256}"`, 'Unbound font alias stylesheet.');
  const result = new Map();
  for (const alias of manifest.aliases) {
    requireValue(safeFontAliasURL(alias?.url) && safeAssetURL(alias.canonical_url) && typeof alias.mime === 'string' && /^[a-f0-9]{64}$/.test(alias.sha256) && Number.isInteger(alias.bytes) && alias.bytes > 0, 'Invalid font alias entry.');
    const selected = assets.get(alias.canonical_url);
    requireValue(selected && !result.has(alias.url), 'Unselected or duplicate font alias.');
    requireValue(alias.url === alias.canonical_url + '?v=4.1' && selected.mime === 'font/woff2' && alias.mime === selected.mime && selected.identity.body.length === alias.bytes && selected.identity.etag === `"${alias.sha256}"`, 'Font alias representation identity mismatch.');
    result.set(alias.url, Object.freeze({...alias}));
  }
  requireValue(result.size === manifest.aliases.length && result.size > 0, 'Empty font alias manifest.');
  return result;
}

// The request object is deliberately never rewritten.  Only a fresh minimal
// object supplies the already-authorized canonical lookup to the v1 response
// helper, so a source delegate cannot observe alias-to-canonical substitution.
export function createFontAliasReferenceHandler({origin, assets, aliasManifest, delegate}) {
  requireValue(origin && assets instanceof Map && typeof delegate === 'function', 'Font alias listener requires origin, assets and source delegate.');
  const aliases = checkedAliasMap(aliasManifest, assets);
  const aliasManifestSHA256 = sha256(Buffer.from(JSON.stringify(aliasManifest)));
  const v1Handler = createCachedStaticReferenceHandler({origin, assets, delegate});
  return async (request, response) => {
    const refusal = refuseRequest(request, origin);
    if (refusal) {
      applyV1SecurityHeaders(response);
      if (refusal[0] === 405) response.setHeader('Allow', 'GET, HEAD');
      response.writeHead(refusal[0], {'Content-Type': 'text/plain; charset=utf-8'});
      response.end(refusal[1]);
      return;
    }
    const alias = !hasRouteDataHeaders(request) && aliases.get(request.url);
    const canonical = !hasRouteDataHeaders(request) && (request.method === 'GET' || request.method === 'HEAD') && assets.has(request.url);
    if (alias || canonical) {
      // These identify an admitted selected-static response (including its
      // explicit 406 representation refusal), never an authority/method/route
      // veto or ordinary no-store delegate response.
      response.setHeader('X-AppKit-Delivery-Experiment', FONT_ALIAS_DELIVERY_EXPERIMENT);
      response.setHeader('X-AppKit-Font-Aliases-SHA256', aliasManifestSHA256);
      applyV1SecurityHeaders(response);
      const served = tryServeCachedStatic(alias ? {method: request.method, url: alias.canonical_url, headers: request.headers} : request, response, assets);
      if (served) return;
      response.removeHeader('X-AppKit-Delivery-Experiment');
      response.removeHeader('X-AppKit-Font-Aliases-SHA256');
      response.setHeader('Cache-Control', 'no-store');
      response.writeHead(500, {'Content-Type': 'text/plain; charset=utf-8'});
      response.end('The selected font alias could not be delivered.');
      return;
    }
    return v1Handler(request, response);
  };
}

export async function startFontAliasReference(projectRoot, packetRoot) {
  requireValue(typeof projectRoot === 'string' && isAbsolute(projectRoot), 'An explicit absolute reviewed project directory is required.');
  const packet = readCachedStaticPacket(packetRoot);
  const source = verifyHardenedReference(projectRoot);
  requireValue(source.source_manifest_sha256 === packet.sourceManifestSHA256, 'Selected source root does not match packet lineage.');
  const sourceAssets = new Map(packet.assets.map(asset => {
    const body = readSourceAsset(projectRoot, asset.url, asset);
    return [asset.url, Object.freeze({body, mime: asset.mime, sha256: asset.sha256})];
  }));
  const cssAsset = packet.assets.find(asset => asset.mime === 'text/css' && asset.url.endsWith('/inter.css'));
  requireValue(cssAsset, 'Pinned font stylesheet is missing.');
  const aliasManifest = deriveFontAliases({css: sourceAssets.get(cssAsset.url).body, cssURL: cssAsset.url, assets: packet.assets});
  const assets = prepareCachedStaticAssets(sourceAssets);
  checkedAliasMap(aliasManifest, assets);
  const aliasManifestSHA256 = sha256(Buffer.from(JSON.stringify(aliasManifest)));
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
  requestHandler = createFontAliasReferenceHandler({origin, assets, aliasManifest, delegate});
  const app = next({dir: projectRoot, dev: false, hostname: '127.0.0.1', port: server.address().port});
  try { await app.prepare(); handle = app.getRequestHandler(); }
  catch (error) {
    server.closeAllConnections();
    await new Promise(resolveClosed => server.close(resolveClosed));
    await app.close();
    throw error;
  }
  return {origin, policy_id: FONT_ALIAS_DELIVERY_EXPERIMENT, alias_manifest_sha256: aliasManifestSHA256, close: async () => {
    const deadline = setTimeout(() => server.closeAllConnections(), 5000);
    try {
      await new Promise((resolveClosed, reject) => server.close(error => error ? reject(error) : resolveClosed()));
      await app.close();
    } finally { clearTimeout(deadline); }
  }};
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  if (process.argv.length !== 4) throw new Error('Usage: node cached-static-font-alias-reference-server.mjs ABSOLUTE_REVIEWED_SOURCE_ROOT ABSOLUTE_SOURCE_PACKET');
  const reference = await startFontAliasReference(process.argv[2], process.argv[3]);
  console.log(JSON.stringify({origin: reference.origin, policy_id: reference.policy_id, alias_manifest_sha256: reference.alias_manifest_sha256}));
  let stopping = false;
  const stop = async () => {
    if (stopping) return;
    stopping = true;
    const deadline = setTimeout(() => process.exit(1), 6000);
    try { await reference.close(); } finally { clearTimeout(deadline); }
  };
  process.on('SIGINT', stop); process.on('SIGTERM', stop);
}
