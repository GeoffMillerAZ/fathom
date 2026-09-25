// Private, source-native Catalyst rehearsal. No archive or component edits.
import http from 'node:http';
import { realpathSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve, isAbsolute } from 'node:path';
import { pathToFileURL } from 'node:url';

export function refuseRequest(request, origin) {
  const peer = request.socket.remoteAddress;
  if (!['127.0.0.1', '::1', '::ffff:127.0.0.1'].includes(peer)) return [403, 'Local reference only.'];
  const counts = new Map();
  for (let i = 0; i < request.rawHeaders.length; i += 2) {
    const key = request.rawHeaders[i].toLowerCase();
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  if (counts.get('host') !== 1 || request.headers.host !== new URL(origin).host) return [403, 'Unexpected reference host.'];
  for (const key of ['origin', 'sec-fetch-site']) {
    if ((counts.get(key) ?? 0) > 1) return [403, 'Ambiguous request authority.'];
  }
  if (request.headers.origin !== undefined && request.headers.origin !== origin) return [403, 'Same-origin reference only.'];
  if (request.headers['sec-fetch-site'] !== undefined && !['same-origin', 'none'].includes(request.headers['sec-fetch-site'])) return [403, 'Same-origin reference only.'];
  if (!['GET', 'HEAD'].includes(request.method)) return [405, 'Read-only source reference; saving is not wired.'];
  if (!request.url?.startsWith('/') || request.url.startsWith('//') || request.url.includes('#')) return [400, 'Expected a relative request target.'];
  if (request.headers['transfer-encoding'] !== undefined || (request.headers['content-length'] !== undefined && request.headers['content-length'] !== '0')) return [400, 'Reference reads accept no body.'];
  return null;
}

export async function startReference(projectRoot) {
  if (!isAbsolute(projectRoot)) throw new Error('An explicit absolute reviewed project directory is required.');
  const requireProject = createRequire(resolve(projectRoot, 'package.json'));
  const next = requireProject('next');
  let origin, handle;
  const server = http.createServer(async (request, response) => {
    response.setHeader('Cache-Control', 'no-store');
    response.setHeader('Referrer-Policy', 'no-referrer');
    response.setHeader('X-Content-Type-Options', 'nosniff');
    response.setHeader('X-Frame-Options', 'DENY');
    response.setHeader('Content-Security-Policy', "frame-ancestors 'none'; base-uri 'self'; form-action 'self'");
    response.setHeader('X-AppKit-Reference', 'source-native-read-only');
    const refusal = refuseRequest(request, origin);
    if (refusal) {
      if (refusal[0] === 405) response.setHeader('Allow', 'GET, HEAD');
      response.writeHead(refusal[0], { 'Content-Type': 'text/plain; charset=utf-8' });
      response.end(refusal[1]);
      return;
    }
    if (!handle) {
      response.writeHead(503, { 'Content-Type': 'text/plain; charset=utf-8' });
      response.end('The source reference is preparing.');
      return;
    }
    try {
      await handle(request, response);
    } catch {
      if (!response.headersSent) response.writeHead(500, { 'Content-Type': 'text/plain; charset=utf-8' });
      response.end('The source reference failed to render.');
    }
  });
  server.headersTimeout = 5000;
  server.requestTimeout = 10000;
  server.keepAliveTimeout = 2000;
  server.on('upgrade', (_request, socket) => socket.destroy());
  await new Promise((resolveReady, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolveReady);
  });
  origin = `http://127.0.0.1:${server.address().port}`;
  // Next substitutes 3000 for port:0 internally. Give its router the actual
  // outer listener port; no second, unguarded TCP listener is created.
  const app = next({ dir: projectRoot, dev: false, hostname: '127.0.0.1', port: server.address().port });
  try {
    await app.prepare();
    handle = app.getRequestHandler();
  } catch (error) {
    server.closeAllConnections();
    await new Promise(resolveClosed => server.close(resolveClosed));
    await app.close();
    throw error;
  }
  return {
    origin,
    close: async () => {
      const deadline = setTimeout(() => server.closeAllConnections(), 5000);
      try {
        await new Promise((resolveClosed, reject) => server.close(error => error ? reject(error) : resolveClosed()));
        await app.close();
      } finally {
        clearTimeout(deadline);
      }
    },
  };
}

if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) {
  if (process.argv.length !== 3) throw new Error('Usage: node reference-server.mjs ABSOLUTE_REVIEWED_PROJECT');
  const reference = await startReference(process.argv[2]);
  console.log(`app-kit Catalyst reference: ${reference.origin}/settings`);
  let stopping = false;
  const stop = async () => {
    if (stopping) return;
    stopping = true;
    const deadline = setTimeout(() => process.exit(1), 6000);
    try { await reference.close(); } finally { clearTimeout(deadline); }
  };
  process.on('SIGINT', stop);
  process.on('SIGTERM', stop);
}
