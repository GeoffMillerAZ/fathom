import http from 'node:http';

const MAX_POST = 8 * 1024;
const MAX_RESPONSE = 512 * 1024;
const UPSTREAM_TIMEOUT = 5_000;
const hopHeaders = new Set(['connection', 'keep-alive', 'proxy-connection', 'te', 'trailer', 'transfer-encoding', 'upgrade']);
const getTargets = new Set([
  '/shell/profile', '/shell/profile?theme=light', '/shell/profile?theme=dark',
  '/shell/settings', '/shell/settings?theme=light', '/shell/settings?theme=dark',
  '/static/app.css', '/static/shell-demo.js',
]);

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  promise.catch(() => {});
  return {promise, resolve, reject};
}

class TransportFailure extends Error {
  constructor(code, expected = false) { super(code); this.code = code; this.expected = expected; }
}

function targetURL(raw) {
  if (typeof raw !== 'string') throw new Error('target must be an exact HTTP IPv4 loopback origin');
  const url = new URL(raw);
  if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !url.port || raw !== url.origin) {
    throw new Error('target must be an exact HTTP 127.0.0.1 origin with an explicit port');
  }
  return url;
}

function headersWithoutHops(headers) {
  return Object.fromEntries(Object.entries(headers).filter(([name]) => !hopHeaders.has(name.toLowerCase())));
}

function reply(res, status, text) {
  if (res.destroyed || res.headersSent) return;
  const body = Buffer.from(text);
  res.writeHead(status, {'content-type': 'text/plain; charset=utf-8', 'cache-control': 'no-store', 'content-length': body.length});
  res.end(body);
}

function readPost(req) {
  return new Promise((resolve, reject) => {
    const declared = Number(req.headers['content-length'] || 0);
    if (Number.isFinite(declared) && declared > MAX_POST) {
      req.resume();
      reject(new TransportFailure('post-too-large', true));
      return;
    }
    const chunks = [];
    let size = 0;
    req.on('data', chunk => {
      size += chunk.length;
      if (size > MAX_POST) reject(new TransportFailure('post-too-large', true));
      else chunks.push(chunk);
    });
    req.on('end', () => size <= MAX_POST && resolve(Buffer.concat(chunks)));
    req.on('error', () => reject(new TransportFailure('downstream-read')));
  });
}

export async function recoveryTransport({target, stage, result}) {
  const upstream = targetURL(target);
  if (!['before-dispatch', 'after-acceptance'].includes(stage)) throw new Error('stage must be before-dispatch or after-acceptance');
  if (!result || typeof result !== 'object' || Array.isArray(result)) throw new Error('result object required');
  result.posts = Number.isSafeInteger(result.posts) ? result.posts : 0;
  result.forwardedHeldPosts = 0;
  result.heldStatus = null;
  result.transportErrors = Array.isArray(result.transportErrors) ? result.transportErrors : [];

  const reached = deferred(), interrupted = deferred(), released = deferred(), settled = deferred();
  const downstreamSockets = new Set(), upstreamSockets = new Set(), holdTasks = new Set();
  let origin, armed = false, holdConsumed = false, ownerReleased = false, closed = false, closePromise;

  const record = (phase, error) => {
    if (!error?.expected) result.transportErrors.push({phase, code: error?.code || 'transport-error'});
  };
  const exchange = (req, body) => new Promise((resolve, reject) => {
    const headers = headersWithoutHops(req.headers);
    delete headers['transfer-encoding'];
    headers['content-length'] = String(body.length);
    const outgoing = http.request({hostname: upstream.hostname, port: upstream.port, method: req.method,
      path: req.url, headers, agent: false}, response => {
      const chunks = [];
      let size = 0;
      response.on('data', chunk => {
        size += chunk.length;
        if (size > MAX_RESPONSE) response.destroy(new TransportFailure('response-too-large', true));
        else chunks.push(chunk);
      });
      response.on('end', () => resolve({status: response.statusCode || 502,
        headers: headersWithoutHops(response.headers), body: Buffer.concat(chunks)}));
      response.on('error', reject);
    });
    const timer = setTimeout(() => outgoing.destroy(new TransportFailure('upstream-timeout')), UPSTREAM_TIMEOUT);
    outgoing.on('socket', socket => {
      upstreamSockets.add(socket);
      socket.once('close', () => upstreamSockets.delete(socket));
    });
    outgoing.on('error', reject);
    outgoing.on('close', () => clearTimeout(timer));
    outgoing.end(body);
  });
  const forward = async (req, res, body) => {
    try {
      const response = await exchange(req, body);
      response.headers['content-length'] = String(response.body.length);
      if (!res.destroyed) { res.writeHead(response.status, response.headers); res.end(response.body); }
    } catch (error) { record('forward', error); reply(res, 502, 'upstream response unavailable'); }
  };
  const hold = async (req, res, body) => {
    // ServerResponse.close is the downstream browser/socket boundary. Request.close
    // also fires after a normally consumed request body and cannot prove interruption.
    res.once('close', () => {
      if (!ownerReleased && !closed) interrupted.resolve();
    });
    try {
      if (stage === 'after-acceptance') {
        result.forwardedHeldPosts = 1;
        const response = await exchange(req, body);
        result.heldStatus = response.status;
      }
      reached.resolve();
      await released.promise;
      if (!res.destroyed) res.destroy();
    } catch (error) {
      record('hold', error);
      reached.reject(new Error('recovery transport hold failed'));
      reply(res, 502, 'upstream response unavailable');
    } finally { settled.resolve(); }
  };

  const server = http.createServer(async (req, res) => {
    if (closed) return reply(res, 503, 'transport closed');
    const exactHost = req.headers.host === new URL(origin).host;
    const localPeer = req.socket.remoteAddress === '127.0.0.1';
    const exactOrigin = req.headers.origin === origin;
    if (!exactHost || !localPeer || (req.headers.origin && !exactOrigin) || (req.method === 'POST' && !exactOrigin)) {
      req.resume(); return reply(res, 403, 'request authority refused');
    }
    const isGet = req.method === 'GET' && getTargets.has(req.url);
    const isPost = req.method === 'POST' && req.url === '/shell/action';
    if (!isGet && !isPost) { req.resume(); return reply(res, ['GET', 'POST'].includes(req.method) ? 404 : 405, 'route refused'); }
    if (isGet) return forward(req, res, Buffer.alloc(0));
    result.posts++;
    const holdThis = armed;
    if (holdThis) { armed = false; holdConsumed = true; }
    let body;
    try { body = await readPost(req); }
    catch (error) { record('request', error); return reply(res, error?.code === 'post-too-large' ? 413 : 400, 'request body refused'); }
    if (!holdThis) return forward(req, res, body);
    const task = hold(req, res, body);
    holdTasks.add(task);
    task.finally(() => holdTasks.delete(task));
  });
  server.on('connection', socket => {
    downstreamSockets.add(socket);
    socket.once('close', () => downstreamSockets.delete(socket));
  });
  server.on('clientError', (_, socket) => socket.end('HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\n'));
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => { server.off('error', reject); resolve(); });
  });
  origin = `http://127.0.0.1:${server.address().port}`;

  return {
    origin,
    arm() {
      if (closed || armed || holdConsumed) throw new Error('recovery transport can arm exactly once');
      armed = true;
    },
    reached: reached.promise,
    interrupted: interrupted.promise,
    release() { ownerReleased = true; released.resolve(); },
    settled: settled.promise,
    close() {
      if (closePromise) return closePromise;
      closePromise = (async () => {
        closed = true; ownerReleased = true; released.resolve();
        for (const socket of [...downstreamSockets, ...upstreamSockets]) socket.destroy();
        await Promise.allSettled([...holdTasks]);
        await new Promise(resolve => server.close(resolve));
      })();
      return closePromise;
    },
  };
}
