// Source-native delivery/security checks only. No browser, hydration, fonts,
// screenshots, working saves, or visual/interaction equivalence is asserted.
import assert from 'node:assert/strict';
import http from 'node:http';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { isAbsolute, join } from 'node:path';
import net from 'node:net';
import test from 'node:test';
import { refuseRequest, startReference } from './reference-server.mjs';

process.env.NEXT_TELEMETRY_DISABLED = '1';
const project = process.env.APPKIT_CATALYST_REFERENCE_ROOT;
assert.ok(project && isAbsolute(project), 'Set APPKIT_CATALYST_REFERENCE_ROOT to an absolute prepared P10 project; missing evidence is not a skip.');
for (const [path, expected] of [
  ['package.json', '04f5b915bdd1b1ef041291a7a30b4a2af1e0edb43156ab876b2f27ae72c56c53'],
  ['package-lock.json', '3a9469d59cb2c862c35735c5153f43b7ddf27c9f30c867cf64838df1f9b66b3c'],
  ['src/app/(app)/settings/page.tsx', '1ca7f4e14738e917390815dc38d48047588eb324090d337f5c718b9d2ec6fa21'],
  ['src/app/layout.tsx', '4cb1963cfefaf9cb15864bf884a5343c99a6d7681ebe4d129a49663b57ba3162'],
]) {
  assert.equal(createHash('sha256').update(readFileSync(join(project, path))).digest('hex'), expected, `reviewed P10 input changed: ${path}`);
}
const fixtureOrigin = 'http://127.0.0.1:18473';

function synthetic({ method = 'GET', url = '/settings', peer = '127.0.0.1', headers = {}, rawHeaders } = {}) {
  const allHeaders = { host: new URL(fixtureOrigin).host, ...headers };
  return {
    method, url, headers: allHeaders, socket: { remoteAddress: peer },
    rawHeaders: rawHeaders ?? Object.entries(allHeaders).flatMap(([name, value]) => value === undefined ? [] : [name, value]),
  };
}

test('pure authority, method, body and request-target refusal matrix', async t => {
  const host = new URL(fixtureOrigin).host;
  const cases = [
    ['ordinary GET', {}, null],
    ['ordinary HEAD', { method: 'HEAD' }, null],
    ['same-origin headers', { headers: { origin: fixtureOrigin, 'sec-fetch-site': 'same-origin' } }, null],
    ['direct-navigation metadata', { headers: { 'sec-fetch-site': 'none' } }, null],
    ['IPv6 loopback', { peer: '::1' }, null],
    ['mapped loopback', { peer: '::ffff:127.0.0.1' }, null],
    ['explicit empty body', { headers: { 'content-length': '0' } }, null],
    ['query remains same-origin', { url: '/settings?reference=synthetic' }, null],
    ['missing Host', { headers: { host: undefined } }, 403],
    ['duplicate Host', { rawHeaders: ['Host', host, 'hOsT', host] }, 403],
    ['foreign Host', { headers: { host: 'example.invalid' } }, 403],
    ['wrong port', { headers: { host: '127.0.0.1:18474' } }, 403],
    ['foreign Origin', { headers: { origin: 'https://example.invalid' } }, 403],
    ['null Origin', { headers: { origin: 'null' } }, 403],
    ['duplicate Origin', { headers: { origin: fixtureOrigin }, rawHeaders: ['Host', host, 'Origin', fixtureOrigin, 'origin', fixtureOrigin] }, 403],
    ['cross-site metadata', { headers: { 'sec-fetch-site': 'cross-site' } }, 403],
    ['same-site is not same-origin', { headers: { 'sec-fetch-site': 'same-site' } }, 403],
    ['duplicate metadata', { headers: { 'sec-fetch-site': 'same-origin' }, rawHeaders: ['Host', host, 'Sec-Fetch-Site', 'same-origin', 'sec-fetch-site', 'same-origin'] }, 403],
    ['nonloopback cannot forward loopback authority', { peer: '192.0.2.10', headers: { 'x-forwarded-for': '127.0.0.1', 'x-forwarded-host': host, forwarded: 'for=127.0.0.1' } }, 403],
    ['forwarded Host cannot replace actual Host', { headers: { host: 'example.invalid', 'x-forwarded-host': host } }, 403],
    ['POST', { method: 'POST' }, 405],
    ['PUT', { method: 'PUT' }, 405],
    ['DELETE', { method: 'DELETE' }, 405],
    ['GET body', { headers: { 'content-length': '1' } }, 400],
    ['HEAD body', { method: 'HEAD', headers: { 'content-length': '1' } }, 400],
    ['chunked body', { headers: { 'transfer-encoding': 'chunked' } }, 400],
    ['noncanonical body length', { headers: { 'content-length': '00' } }, 400],
    ['absolute URL', { url: 'http://127.0.0.1:18473/settings' }, 400],
    ['foreign absolute URL', { url: 'https://example.invalid/settings' }, 400],
    ['authority-form URL', { url: '//example.invalid/settings' }, 400],
    ['empty target', { url: '' }, 400],
    ['fragment', { url: '/settings#fragment' }, 400],
    ['query followed by fragment', { url: '/settings?reference=synthetic#fragment' }, 400],
  ];
  for (const [name, options, status] of cases) {
    await t.test(name, () => {
      const got = refuseRequest(synthetic(options), fixtureOrigin);
      assert.equal(got?.[0] ?? null, status);
      if (status !== null) assert.ok(typeof got[1] === 'string' && got[1].length > 0);
    });
  }
});

function localRequest(origin, path, { method = 'GET', headers = {}, body = '' } = {}) {
  const base = new URL(origin);
  assert.equal(base.protocol, 'http:');
  assert.equal(base.hostname, '127.0.0.1');
  assert.ok(path.startsWith('/') && !path.startsWith('//'));
  return new Promise((resolve, reject) => {
    const request = http.request({ hostname: '127.0.0.1', port: base.port, path, method, headers, agent: false }, response => {
      const chunks = [];
      let size = 0;
      response.on('data', chunk => {
        size += chunk.length;
        // Safety ceiling only, not an application payload/performance budget.
        if (size > 2 * 1024 * 1024) request.destroy(new Error('test response safety ceiling exceeded'));
        else chunks.push(chunk);
      });
      response.once('error', reject);
      response.once('end', () => resolve({ status: response.statusCode, headers: response.headers, body: Buffer.concat(chunks) }));
    });
    const timer = setTimeout(() => request.destroy(new Error('loopback request deadline exceeded')), 5000);
    request.once('close', () => clearTimeout(timer));
    request.once('error', reject);
    request.end(body);
  });
}

function closedUpgrade(origin) {
  const base = new URL(origin);
  assert.equal(base.hostname, '127.0.0.1');
  return new Promise((resolve, reject) => {
    const socket = net.connect({ host: '127.0.0.1', port: Number(base.port) });
    const chunks = [];
    const timer = setTimeout(() => socket.destroy(new Error('upgrade was not closed')), 3000);
    socket.once('connect', () => socket.write(
      `GET /_next/webpack-hmr HTTP/1.1\r\nHost: ${base.host}\r\nOrigin: ${origin}\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n`,
    ));
    socket.on('data', chunk => {
      chunks.push(chunk);
      if (chunks.reduce((n, value) => n + value.length, 0) > 8192) socket.destroy(new Error('unexpected upgrade response exceeded safety ceiling'));
    });
    socket.once('error', error => { if (error.code !== 'ECONNRESET') reject(error); });
    socket.once('close', () => { clearTimeout(timer); resolve(Buffer.concat(chunks)); });
  });
}

test('built Catalyst settings delivery stays read-only, local and disposable', { timeout: 45000 }, async t => {
  const reference = await startReference(project);
  let closed = false;
  t.after(async () => { if (!closed) await reference.close(); });
  assert.match(reference.origin, /^http:\/\/127\.0\.0\.1:[1-9][0-9]*$/);

  const page = await localRequest(reference.origin, '/settings');
  assert.equal(page.status, 200);
  assert.match(page.headers['content-type'], /^text\/html/);
  assert.equal(page.headers['x-appkit-reference'], 'source-native-read-only');
  assert.equal(page.headers['x-frame-options'], 'DENY');
  assert.equal(page.headers['x-content-type-options'], 'nosniff');
  assert.equal(page.headers['referrer-policy'], 'no-referrer');
  const html = page.body.toString('utf8');
  assert.match(html, /<title>Settings - Catalyst<\/title>/);
  assert.match(html, /<h1\b[^>]*>Settings<\/h1>/);
  for (const label of ['Organization Name', 'Organization Bio', 'Organization Email', 'Save changes']) assert.ok(html.includes(label), `missing source-native ${label}`);
  const nameInput = [...html.matchAll(/<input\b[^>]*>/g)].map(match => match[0]).find(tag => tag.includes('name="name"'));
  assert.ok(nameInput?.includes('value="Catalyst"'), 'original Catalyst name fixture missing');

  // Follow only same-origin build artifacts explicitly named by this page.
  // External Inter stylesheet/font references are deliberately not fetched.
  const links = [...html.matchAll(/(?:src|href)="([^"<>]+)"/g)].map(match => match[1]);
  const css = links.find(link => /^\/_next\/static\/[^?#]+\.css(?:\?[^#]*)?$/.test(link));
  const script = links.find(link => /^\/_next\/static\/[^?#]+\.js(?:\?[^#]*)?$/.test(link));
  assert.ok(css, 'built page must reference a local CSS artifact');
  assert.ok(script, 'built page must reference a local JS artifact');
  for (const [path, type] of [[css, /text\/css/], [script, /(?:java|ecma)script/]]) {
    const asset = await localRequest(reference.origin, path.replaceAll('&amp;', '&'));
    assert.equal(asset.status, 200, path);
    assert.match(asset.headers['content-type'], type);
    assert.ok(asset.body.length > 0);
  }
  const head = await localRequest(reference.origin, '/settings', { method: 'HEAD' });
  assert.equal(head.status, 200);
  assert.equal(head.body.length, 0);
  const queried = await localRequest(reference.origin, '/settings?reference=synthetic');
  assert.equal(queried.status, 200);

  for (const [name, options, status] of [
    ['POST cannot save', { method: 'POST', headers: { origin: reference.origin, 'content-type': 'application/x-www-form-urlencoded', 'content-length': String(Buffer.byteLength('name=Not%20Saved!')) }, body: 'name=Not%20Saved!' }, 405],
    ['foreign Host', { headers: { host: 'example.invalid' } }, 403],
    ['foreign Origin', { headers: { origin: 'https://example.invalid' } }, 403],
    ['cross-site metadata', { headers: { 'sec-fetch-site': 'cross-site' } }, 403],
    ['same-site metadata', { headers: { 'sec-fetch-site': 'same-site' } }, 403],
    ['duplicate Origin', { headers: { origin: [reference.origin, reference.origin] } }, 403],
    ['GET body', { headers: { 'content-length': '1' }, body: 'x' }, 400],
  ]) {
    await t.test(name, async () => {
      const response = await localRequest(reference.origin, '/settings', options);
      assert.equal(response.status, status);
      assert.equal(response.headers['x-appkit-reference'], 'source-native-read-only');
      assert.ok(!response.body.toString('utf8').includes('Organization Name'));
      if (status === 405) assert.equal(response.headers.allow, 'GET, HEAD');
    });
  }
  const afterPost = await localRequest(reference.origin, '/settings');
  assert.equal(afterPost.status, 200);
  assert.ok(!afterPost.body.toString('utf8').includes('Not Saved!'));
  assert.equal((await localRequest(reference.origin, '/appkit-synthetic-unknown-route')).status, 404);

  // The successful requests above prime any upgrade listener Next installs.
  assert.equal((await closedUpgrade(reference.origin)).length, 0, 'upgrade must close without a handshake or response');

  await reference.close();
  closed = true;
  await assert.rejects(localRequest(reference.origin, '/settings'), error => error.code === 'ECONNREFUSED');
});
