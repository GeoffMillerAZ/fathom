import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import {recoveryTransport} from './recovery_transport.mjs';

const security = {
  'content-security-policy': "default-src 'none'",
  'x-content-type-options': 'nosniff',
  'referrer-policy': 'same-origin',
};

function listen(server) {
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      server.off('error', reject);
      resolve(`http://127.0.0.1:${server.address().port}`);
    });
  });
}

function close(server) {
  return new Promise(resolve => server.close(resolve));
}

async function syntheticUpstream(t) {
  const state = {posts: 0, profileGets: 0, settingsGets: 0, requests: [], bodies: [], bigCSS: false};
  const server = http.createServer((req, res) => {
    state.requests.push({method: req.method, url: req.url, host: req.headers.host, origin: req.headers.origin});
    if (req.method === 'POST' && req.url === '/shell/action') {
      const chunks = [];
      req.on('data', chunk => chunks.push(chunk));
      req.on('end', () => {
        state.posts++;
        state.bodies.push(Buffer.concat(chunks).toString());
        res.writeHead(303, {...security, location: '/shell/profile'});
        res.end('accepted');
      });
      return;
    }
    if (req.method === 'GET' && req.url.startsWith('/shell/profile')) {
      state.profileGets++;
      res.writeHead(200, {...security, 'content-type': 'text/html'});
      res.end('profile');
      return;
    }
    if (req.method === 'GET' && ['/shell/settings', '/shell/settings?theme=light', '/shell/settings?theme=dark'].includes(req.url)) {
      state.settingsGets++;
      res.writeHead(200, {...security, 'content-type': 'text/html'});
      res.end(`settings:${req.url}`);
      return;
    }
    if (req.method === 'GET' && req.url === '/static/app.css') {
      res.writeHead(200, {...security, 'content-type': 'text/css'});
      res.end(state.bigCSS ? Buffer.alloc(512 * 1024 + 1, 97) : 'body{}');
      return;
    }
    if (req.method === 'GET' && req.url === '/static/shell-demo.js') {
      res.writeHead(200, {...security, 'content-type': 'text/javascript'});
      res.end('export{}');
      return;
    }
    res.writeHead(404, security); res.end('missing');
  });
  const origin = await listen(server);
  t.after(() => close(server));
  return {origin, state};
}

function request(origin, path, {method = 'GET', headers = {}, body = '', signal} = {}) {
  return new Promise((resolve, reject) => {
    const req = http.request(origin + path, {method, headers, signal}, res => {
      const chunks = [];
      res.on('data', chunk => chunks.push(chunk));
      res.on('aborted', () => reject(new Error('response aborted')));
      res.on('end', () => resolve({status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks)}));
    });
    req.on('error', reject);
    req.end(body);
  });
}

async function within(promise, label) {
  let timer;
  try {
    return await Promise.race([promise, new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error(`timeout: ${label}`)), 2_000);
    })]);
  } finally { clearTimeout(timer); }
}

function heldRequest(transport, body = 'display_name=Held', signal) {
  return request(transport.origin, '/shell/action', {
    method: 'POST',
    headers: {'content-type': 'application/x-www-form-urlencoded', origin: transport.origin},
    body, signal,
  }).then(value => ({value}), error => ({error}));
}

async function remainsPending(promise) {
  return await Promise.race([promise.then(() => false), new Promise(resolve => setTimeout(() => resolve(true), 20))]);
}

test('before-dispatch holds one POST without creating an upstream request', async t => {
  const upstream = await syntheticUpstream(t);
  const result = {};
  const transport = await recoveryTransport({target: upstream.origin, stage: 'before-dispatch', result});
  t.after(() => transport.close());

  const profile = await request(transport.origin, '/shell/profile?theme=dark');
  assert.equal(profile.status, 200);
  assert.equal(profile.headers['content-security-policy'], security['content-security-policy']);
  assert.equal(upstream.state.requests[0].host, new URL(transport.origin).host);

  transport.arm();
  const held = heldRequest(transport);
  await within(transport.reached, 'before-dispatch reached');
  assert.equal(result.posts, 1);
  assert.equal(upstream.state.posts, 0);
  assert.equal((await request(transport.origin, '/static/shell-demo.js')).status, 200);
  transport.release();
  await within(transport.settled, 'before-dispatch settled');
  assert.ok((await held).error, 'held downstream response must be destroyed');
  assert.equal(await remainsPending(transport.interrupted), true, 'owner release is not client interruption');

  const retry = await request(transport.origin, '/shell/action', {
    method: 'POST', headers: {origin: transport.origin}, body: 'display_name=Retry',
  });
  assert.equal(retry.status, 303);
  assert.equal(upstream.state.posts, 1);
  assert.equal(result.posts, 2);
  assert.deepEqual(result.transportErrors, []);
});

test('client abort resolves interruption before owner release', async t => {
  const upstream = await syntheticUpstream(t);
  const result = {};
  const transport = await recoveryTransport({target: upstream.origin, stage: 'after-acceptance', result});
  t.after(() => transport.close());
  const controller = new AbortController();

  transport.arm();
  const held = heldRequest(transport, 'display_name=Interrupted', controller.signal);
  await within(transport.reached, 'accepted hold reached');
  controller.abort();
  await within(transport.interrupted, 'client interruption observed');
  assert.ok((await held).error);
  assert.equal(result.forwardedHeldPosts, 1);
  assert.equal(result.heldStatus, 303);
  transport.release();
  await within(transport.settled, 'client-aborted hold settled');
  assert.deepEqual(result.transportErrors, []);
});

test('after-acceptance forwards once without following or exposing the response', async t => {
  const upstream = await syntheticUpstream(t);
  const result = {posts: 0, transportErrors: []};
  const transport = await recoveryTransport({target: upstream.origin, stage: 'after-acceptance', result});
  t.after(() => transport.close());

  transport.arm();
  const held = heldRequest(transport, 'display_name=Accepted');
  await within(transport.reached, 'after-acceptance reached');
  assert.equal(upstream.state.posts, 1);
  assert.deepEqual(upstream.state.bodies, ['display_name=Accepted']);
  assert.equal(upstream.state.profileGets, 0, '303 must not be followed');
  assert.equal(result.forwardedHeldPosts, 1);
  assert.equal(result.heldStatus, 303);
  const accepted = upstream.state.requests.find(item => item.method === 'POST');
  assert.equal(accepted.host, new URL(transport.origin).host);
  assert.equal(accepted.origin, transport.origin);

  transport.release();
  await within(transport.settled, 'after-acceptance settled');
  assert.ok((await held).error, 'accepted response must remain withheld');
  assert.equal((await request(transport.origin, '/shell/profile')).status, 200);
  assert.deepEqual(result.transportErrors, []);
});

test('authority, route and byte bounds refuse before unsafe forwarding', async t => {
  const upstream = await syntheticUpstream(t);
  const result = {};
  const transport = await recoveryTransport({target: upstream.origin, stage: 'before-dispatch', result});
  t.after(() => transport.close());

  assert.equal((await request(transport.origin, '/shell/profile', {headers: {host: 'remote.example'}})).status, 403);
  assert.equal((await request(transport.origin, '/shell/action', {method: 'POST', headers: {origin: 'http://127.0.0.1:1'}})).status, 403);
  for (const path of ['/shell/settings', '/shell/settings?theme=light', '/shell/settings?theme=dark']) {
    const settings = await request(transport.origin, path);
    assert.equal(settings.status, 200);
    assert.equal(settings.body.toString(), `settings:${path}`);
    assert.equal(settings.headers['content-security-policy'], security['content-security-policy']);
  }
  assert.equal(upstream.state.settingsGets, 3);
  const afterSettings = upstream.state.requests.length;
  assert.equal((await request(transport.origin, '/shell/profile?theme=blue')).status, 404);
  assert.equal((await request(transport.origin, '/shell/settings?theme=system')).status, 404);
  assert.equal((await request(transport.origin, '/shell/settings?theme=light&theme=dark')).status, 404);
  assert.equal((await request(transport.origin, '/shell/settings?x=1')).status, 404);
  assert.equal((await request(transport.origin, '/shell/settings', {
    method: 'POST', headers: {origin: transport.origin}, body: 'theme=dark',
  })).status, 404);
  assert.equal((await request(transport.origin, '/static/app.css?x=1')).status, 404);
  assert.equal((await request(transport.origin, '/shell/action?x=1', {method: 'POST', headers: {origin: transport.origin}})).status, 404);
  assert.equal((await request(transport.origin, '/shell/profile', {method: 'PUT'})).status, 405);
  assert.equal(upstream.state.requests.length, afterSettings);

  const tooLarge = await request(transport.origin, '/shell/action', {
    method: 'POST', headers: {origin: transport.origin}, body: Buffer.alloc(8 * 1024 + 1),
  });
  assert.equal(tooLarge.status, 413);
  assert.equal(upstream.state.requests.length, afterSettings);
  upstream.state.bigCSS = true;
  assert.equal((await request(transport.origin, '/static/app.css')).status, 502);
  assert.deepEqual(result.transportErrors, []);
});

test('close releases a held request and destroys only transport-owned sockets', async t => {
  const upstream = await syntheticUpstream(t);
  const result = {};
  const transport = await recoveryTransport({target: upstream.origin, stage: 'before-dispatch', result});
  transport.arm();
  const held = heldRequest(transport);
  await within(transport.reached, 'cleanup reached');
  await within(transport.close(), 'transport close');
  await within(transport.settled, 'cleanup settled');
  assert.ok((await held).error);
  assert.equal(await remainsPending(transport.interrupted), true, 'owner close is not client interruption');
  assert.equal((await request(upstream.origin, '/shell/profile')).status, 200, 'upstream remains live');
  await assert.rejects(request(transport.origin, '/shell/profile'));
  assert.deepEqual(result.transportErrors, []);
});

test('configuration rejects non-exact targets, stages and result containers', async () => {
  for (const target of ['https://127.0.0.1:1234', 'http://localhost:1234', 'http://127.0.0.1',
    'http://127.0.0.1:1234/', 'http://user@127.0.0.1:1234', 'http://127.1:1234']) {
    await assert.rejects(recoveryTransport({target, stage: 'before-dispatch', result: {}}));
  }
  await assert.rejects(recoveryTransport({target: 'http://127.0.0.1:1234', stage: 'later', result: {}}));
  await assert.rejects(recoveryTransport({target: 'http://127.0.0.1:1234', stage: 'before-dispatch'}));
});
