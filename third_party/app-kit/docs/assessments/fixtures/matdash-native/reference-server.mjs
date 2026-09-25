// Private MatDash static-delivery rehearsal. Never loads or executes vendor JS.
import { createHash } from 'node:crypto';
import { closeSync, constants, fstatSync, lstatSync, openSync, opendirSync, readSync, realpathSync } from 'node:fs';
import http from 'node:http';
import { basename, dirname, extname, isAbsolute, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { refuseRequest } from '../catalyst-native/reference-server.mjs';

const metadataHash = '545f1944260dcccc1abdca03cfc4b5d5ae81af9969457675dd7b78d3ed252a6f';
const maxFile = 4 * 1024 * 1024, maxTotal = 64 * 1024 * 1024, maxFiles = 2000;
const mime = new Map(Object.entries({
  '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8', '.json': 'application/json; charset=utf-8',
  '.txt': 'text/plain; charset=utf-8',
  '.svg': 'image/svg+xml', '.png': 'image/png', '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg', '.gif': 'image/gif', '.webp': 'image/webp',
  '.ico': 'image/x-icon', '.woff': 'font/woff', '.woff2': 'font/woff2',
}));
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
function requireValue(ok, message) { if (!ok) throw new Error(message); }

function relativePath(path) {
  requireValue(typeof path === 'string' && path.length > 0 && path.length <= 512 &&
    !isAbsolute(path) && !/[\\\x00-\x20\x7f%?#:]/.test(path) &&
    path.split('/').every(part => part && part !== '.' && part !== '..'), 'Unsafe metadata path.');
  return path;
}

function checkedPath(root, relative, directory = false) {
  const parts = relativePath(relative).split('/');
  let path = root;
  for (let i = 0; i < parts.length; i++) {
    path = join(path, parts[i]);
    const stat = lstatSync(path);
    requireValue(!stat.isSymbolicLink(), 'Reference symlink refused.');
    requireValue(i < parts.length - 1 || directory ? stat.isDirectory() : stat.isFile(), 'Nonregular reference input refused.');
  }
  return path;
}

function readRegular(root, relative, limit) {
  const fd = openSync(checkedPath(root, relative), constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const stat = fstatSync(fd);
    requireValue(stat.isFile() && Number.isSafeInteger(stat.size) && stat.size <= limit, 'Reference input exceeds bounds or is nonregular.');
    const bytes = Buffer.alloc(stat.size + 1);
    let count = 0, read;
    while (count < bytes.length && (read = readSync(fd, bytes, count, bytes.length - count, null)) > 0) count += read;
    requireValue(count === stat.size, 'Reference changed during reading.');
    return bytes.subarray(0, count);
  } finally { closeSync(fd); }
}

function distFiles(root) {
  const files = [];
  let entries = 0;
  const walk = (relative, depth) => {
    requireValue(depth <= 24, 'Reference directory depth exceeds bounds.');
    const directory = opendirSync(checkedPath(root, relative, true));
    try {
      let entry;
      while ((entry = directory.readSync()) !== null) {
        requireValue(++entries <= maxFiles * 2, 'Reference directory entries exceed bounds.');
        const path = `${relative}/${entry.name}`;
        const stat = lstatSync(join(root, relativePath(path)));
        requireValue(!stat.isSymbolicLink(), 'Reference symlink refused.');
        if (stat.isDirectory()) walk(path, depth + 1);
        else {
          requireValue(stat.isFile(), 'Nonregular reference input refused.');
          requireValue(files.length < maxFiles, 'Reference file count exceeds bounds.');
          files.push(path.slice('dist/'.length));
        }
      }
    } finally { directory.closeSync(); }
  };
  walk('dist', 0);
  return files.sort();
}

function routePattern(route) {
  requireValue(typeof route === 'string' && route.length <= 512 && route.startsWith('/'), 'Invalid declared route.');
  if (route === '/') return /^\/$/;
  const parts = route.slice(1).split('/');
  requireValue(parts.every(part => /^:[A-Za-z_][A-Za-z0-9_]*$/.test(part) ||
    (/^[A-Za-z0-9._~-]+$/.test(part) && part !== '.' && part !== '..')), 'Invalid declared route.');
  return new RegExp(`^/${parts.map(part => part.startsWith(':') ? '[A-Za-z0-9._~-]+' : part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('/')}$`);
}

function snapshot(project, metadataFile, metadataSha256) {
  requireValue(typeof project === 'string' && isAbsolute(project) && !/[\x00-\x1f\x7f]/.test(project), 'Select an absolute prepared MatDash project.');
  const stat = lstatSync(project);
  requireValue(stat.isDirectory() && !stat.isSymbolicLink(), 'Selected project must be a regular directory.');
  const root = realpathSync(project);
  // Pin bytes before JSON parsing or using any metadata-derived filesystem path.
  const bytes = readRegular(dirname(metadataFile), basename(metadataFile), 128 * 1024);
  requireValue(hash(bytes) === metadataSha256, 'Pinned build metadata hash mismatch.');
  const metadata = JSON.parse(bytes);
  requireValue(metadata.schema === 1 && Array.isArray(metadata.source_pins) && metadata.source_pins.length > 0 &&
    metadata.source_pins.length <= maxFiles && Array.isArray(metadata.assets) && metadata.assets.length > 0 &&
    metadata.source_pins.length + metadata.assets.length <= maxFiles &&
    Array.isArray(metadata.routes) && metadata.routes.length > 0 && metadata.routes.length <= maxFiles,
  'Invalid reference metadata shape.');
  const routes = metadata.routes.map(routePattern);
  requireValue(new Set(metadata.routes).size === routes.length, 'Duplicate declared route.');
  let total = 0;
  const checkedRecords = records => {
    const names = new Set();
    for (const record of records) {
      relativePath(record.path);
      requireValue(!names.has(record.path) && /^[0-9a-f]{64}$/.test(record.sha256) &&
        Number.isSafeInteger(record.bytes) && record.bytes >= 0 && record.bytes <= maxFile, 'Invalid reference file metadata.');
      names.add(record.path);
      total += record.bytes;
      requireValue(total <= maxTotal, 'Reference aggregate exceeds bounds.');
    }
  };
  checkedRecords(metadata.source_pins);
  checkedRecords(metadata.assets);
  const assetTotal = metadata.assets.reduce((sum, file) => sum + file.bytes, 0);
  requireValue(Number.isSafeInteger(metadata.total_bytes) && assetTotal === metadata.total_bytes, 'Reference byte total mismatch.');
  for (const pin of metadata.source_pins) {
    const body = readRegular(root, pin.path, pin.bytes);
    requireValue(body.length === pin.bytes && hash(body) === pin.sha256, 'Reviewed source identity mismatch.');
  }
  requireValue(JSON.stringify(distFiles(root)) === JSON.stringify(metadata.assets.map(file => file.path).sort()), 'Reference dist file set differs.');
  const assets = new Map();
  for (const file of metadata.assets) {
    const type = mime.get(extname(file.path).toLowerCase());
    requireValue(type, 'Unreviewed asset MIME type.');
    const body = readRegular(root, `dist/${file.path}`, file.bytes);
    requireValue(body.length === file.bytes && hash(body) === file.sha256, 'Reference asset identity mismatch.');
    assets.set(`/${file.path}`, { body, type });
  }
  requireValue(assets.has('/index.html'), 'Reference index is missing.');
  return { assets, routes };
}

function requestPath(target) {
  if (target.length > 2048 || /[\\\x00-\x20\x7f]/.test(target)) return null;
  const split = target.indexOf('?'), path = split < 0 ? target : target.slice(0, split);
  if (path.includes('%') || path.includes('//') || (path !== '/' && path.endsWith('/')) ||
    path.split('/').some(part => part === '.' || part === '..')) return null;
  try {
    if (split >= 0 && /[\x00-\x1f\x7f]/.test(decodeURIComponent(target.slice(split + 1).replaceAll('+', ' ')))) return null;
  } catch { return null; }
  return path;
}

export async function startReference(project) {
  return startPinnedStaticReference(project, {
    metadataFile: fileURLToPath(new URL('./build-assets.json', import.meta.url)),
    metadataSha256: metadataHash,
  });
}

// Trusted first-party fixtures supply a reviewed absolute metadata path and a
// hard-coded pin. Neither option is accepted from HTTP requests or CLI arguments.
export async function startPinnedStaticReference(project, options) {
  requireValue(options !== null && typeof options === 'object' && !Array.isArray(options), 'Select pinned reference metadata options.');
  const { metadataFile, metadataSha256 } = options;
  requireValue(typeof metadataFile === 'string' && isAbsolute(metadataFile) &&
    !/[\x00-\x1f\x7f]/.test(metadataFile), 'Select an absolute reference metadata file.');
  requireValue(typeof metadataSha256 === 'string' && /^[0-9a-f]{64}$/.test(metadataSha256), 'Select a lowercase SHA-256 metadata pin.');
  relativePath(basename(metadataFile));
  const { assets, routes } = snapshot(project, metadataFile, metadataSha256);
  let origin;
  const server = http.createServer({ maxHeaderSize: 8192 }, (request, response) => {
    response.setHeader('Cache-Control', 'no-store');
    response.setHeader('Referrer-Policy', 'no-referrer');
    response.setHeader('X-Content-Type-Options', 'nosniff');
    response.setHeader('X-Frame-Options', 'DENY');
    response.setHeader('Content-Security-Policy', "frame-ancestors 'none'; base-uri 'self'; form-action 'self'");
    response.setHeader('X-AppKit-Reference', 'matdash-static-read-only');
    const send = (status, body, type = 'text/plain; charset=utf-8') => {
      response.writeHead(status, { 'Content-Type': type, 'Content-Length': Buffer.byteLength(body) });
      response.end(request.method === 'HEAD' ? undefined : body);
    };
    const refusal = refuseRequest(request, origin);
    if (refusal) {
      if (refusal[0] === 405) response.setHeader('Allow', 'GET, HEAD');
      send(...refusal);
      return;
    }
    const path = requestPath(request.url);
    if (path === null) { send(400, 'Expected a canonical reference path and valid query.'); return; }
    const asset = assets.get(path) ?? (routes.some(pattern => pattern.test(path)) ? assets.get('/index.html') : undefined);
    if (!asset) { send(404, 'Unknown reference resource.'); return; }
    send(200, asset.body, asset.type);
  });
  server.headersTimeout = 5000;
  server.requestTimeout = 10000;
  server.keepAliveTimeout = 2000;
  server.setTimeout(5000, socket => socket.destroy());
  server.maxConnections = 64;
  server.on('upgrade', (_request, socket) => socket.destroy());
  await new Promise((ready, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      origin = `http://127.0.0.1:${server.address().port}`;
      ready();
    });
  });
  let closing;
  return { origin, close: () => closing ??= new Promise((resolveClosed, reject) => {
    const deadline = setTimeout(() => server.closeAllConnections(), 5000);
    server.close(error => {
      clearTimeout(deadline);
      if (error) reject(error); else resolveClosed();
    });
  }) };
}

if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
  if (process.argv.length !== 3) {
    console.error('Usage: node reference-server.mjs ABSOLUTE_PREPARED_PROJECT');
    process.exitCode = 2;
  } else {
    try {
      const reference = await startReference(process.argv[2]);
      console.log(`app-kit MatDash static reference: ${reference.origin}/dashboards/dashboard2`);
      let stopping = false;
      const stop = async () => {
        if (stopping) return;
        stopping = true;
        const deadline = setTimeout(() => process.exit(1), 6000);
        try { await reference.close(); } catch { process.exitCode = 1; }
        finally { clearTimeout(deadline); }
      };
      process.on('SIGINT', stop);
      process.on('SIGTERM', stop);
    } catch {
      console.error('MatDash reference refused: selected metadata, source, or build failed verification.');
      process.exitCode = 1;
    }
  }
}
