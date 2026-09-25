// Opt-in private static-byte delivery proof, not React mounting or browser proof.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { copyFileSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, truncateSync, unlinkSync, writeFileSync } from 'node:fs';
import http from 'node:http';
import net from 'node:net';
import { tmpdir } from 'node:os';
import { dirname, extname, isAbsolute, join } from 'node:path';
import { spawn, spawnSync } from 'node:child_process';
import { fileURLToPath, pathToFileURL } from 'node:url';
import test from 'node:test';
import { startPinnedStaticReference, startReference } from './reference-server.mjs';

const project = process.env.APPKIT_MATDASH_REFERENCE_ROOT;
assert.ok(project && isAbsolute(project), 'Set APPKIT_MATDASH_REFERENCE_ROOT to an absolute prepared project; missing evidence is not a skip.');
const here = dirname(fileURLToPath(import.meta.url));
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
const metadataBytes = readFileSync(join(here, 'build-assets.json'));
const metadataSha256 = '545f1944260dcccc1abdca03cfc4b5d5ae81af9969457675dd7b78d3ed252a6f';
assert.ok(metadataBytes.length <= 128 * 1024);
assert.equal(digest(metadataBytes), metadataSha256, 'Pin metadata before deriving fixture paths.');
const metadata = JSON.parse(metadataBytes);
const expectedTypes = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8', '.svg': 'image/svg+xml', '.png': 'image/png',
  '.jpg': 'image/jpeg', '.gif': 'image/gif', '.webp': 'image/webp' };

function ownedRoot(t, prefix = 'appkit-matdash-canary-') {
  const root = mkdtempSync(join(tmpdir(), prefix));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  return root;
}

function fixture(t, assets = 'none') {
  const root = ownedRoot(t);
  for (const pin of metadata.source_pins) {
    const source = join(project, pin.path), target = join(root, pin.path);
    assert.ok(lstatSync(source).isFile() && !lstatSync(source).isSymbolicLink());
    assert.equal(digest(readFileSync(source)), pin.sha256);
    mkdirSync(dirname(target), { recursive: true });
    copyFileSync(source, target);
  }
  mkdirSync(join(root, 'dist'), { recursive: true });
  if (assets !== 'none') {
    for (const file of metadata.assets) {
      const target = join(root, 'dist', file.path);
      mkdirSync(dirname(target), { recursive: true });
      if (assets === 'full') copyFileSync(join(project, 'dist', file.path), target);
      else writeFileSync(target, 'synthetic');
    }
    if (assets === 'first') {
      const first = metadata.assets[0];
      copyFileSync(join(project, 'dist', first.path), join(root, 'dist', first.path));
    }
  }
  return root;
}

function request(origin, path, { method = 'GET', headers = {}, body = '' } = {}) {
  const url = new URL(origin);
  assert.equal(url.protocol, 'http:');
  assert.equal(url.hostname, '127.0.0.1');
  return new Promise((resolve, reject) => {
    const req = http.request({ hostname: '127.0.0.1', port: url.port, path, method, headers, agent: false }, res => {
      let size = 0;
      const chunks = [];
      res.on('data', chunk => {
        size += chunk.length;
        if (size > 4 * 1024 * 1024) req.destroy(new Error('response safety ceiling exceeded'));
        else chunks.push(chunk);
      });
      res.once('error', reject);
      res.once('end', () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks) }));
    });
    const timer = setTimeout(() => req.destroy(new Error('loopback request deadline exceeded')), 5000);
    req.once('close', () => clearTimeout(timer));
    req.once('error', reject);
    req.end(body);
  });
}

function securityHeaders(response) {
  assert.equal(response.headers['cache-control'], 'no-store');
  assert.equal(response.headers['x-content-type-options'], 'nosniff');
  assert.equal(response.headers['x-frame-options'], 'DENY');
  assert.equal(response.headers['referrer-policy'], 'no-referrer');
  assert.equal(response.headers['content-security-policy'], "frame-ancestors 'none'; base-uri 'self'; form-action 'self'");
  assert.equal(response.headers['x-appkit-reference'], 'matdash-static-read-only');
  for (const header of ['nel', 'report-to', 'reporting-endpoints']) assert.equal(response.headers[header], undefined);
}

async function websocketClosed(origin) {
  const url = new URL(origin);
  assert.equal(url.hostname, '127.0.0.1');
  return new Promise((resolve, reject) => {
    const socket = net.connect({ host: '127.0.0.1', port: Number(url.port) });
    let received = 0;
    const timer = setTimeout(() => socket.destroy(new Error('upgrade did not close')), 3000);
    socket.once('connect', () => socket.write(`GET / HTTP/1.1\r\nHost: ${url.host}\r\nOrigin: ${origin}\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n`));
    socket.on('data', bytes => { received += bytes.length; if (received > 8192) socket.destroy(new Error('unexpected upgrade payload')); });
    socket.once('error', error => { if (error.code !== 'ECONNRESET') reject(error); });
    socket.once('close', () => { clearTimeout(timer); resolve(received); });
  });
}

test('exact static assets and declared client routes remain local and read-only', { timeout: 45000 }, async t => {
  assert.equal(metadata.source_pins.length, 17);
  assert.equal(metadata.assets.length, 366);
  assert.equal(metadata.routes.length, 133);
  assert.equal(metadata.total_bytes, 14627979);
  assert.equal(metadata.source_pins.find(pin => pin.path.endsWith('/Dashboard2.tsx')).sha256, '36c37212b4e5571367618d4e0d8fc9d1dfa35c75266228170f8eac071112b277');
  const reference = await startReference(project);
  t.after(() => reference.close());
  const index = metadata.assets.find(file => file.path === 'index.html');
  assert.ok(index);
  let total = 0;
  for (const file of metadata.assets) {
    const response = await request(reference.origin, `/${file.path}`);
    assert.equal(response.status, 200, file.path);
    assert.equal(response.body.length, file.bytes, file.path);
    assert.equal(digest(response.body), file.sha256, file.path);
    assert.equal(response.headers['content-type'], expectedTypes[extname(file.path)], file.path);
    securityHeaders(response);
    total += response.body.length;
  }
  assert.equal(total, metadata.total_bytes);
  for (const route of metadata.routes) {
    const path = route.replace(/:[A-Za-z_][A-Za-z0-9_]*/g, 'synthetic-123');
    const response = await request(reference.origin, path);
    assert.equal(response.status, 200, route);
    assert.equal(response.headers['content-type'], expectedTypes['.html']);
    assert.equal(digest(response.body), index.sha256, route);
  }
  for (const path of ['/dashboards/dashboard2', '/apps/ecommerce/addproduct', '/apps/ecommerce/editproduct']) {
    assert.ok(metadata.routes.includes(path), 'required pilot host route absent');
    const response = await request(reference.origin, `${path}?reference=synthetic%20query`, {
      headers: { origin: reference.origin, 'sec-fetch-site': 'same-origin' },
    });
    assert.equal(digest(response.body), index.sha256);
  }
  const head = await request(reference.origin, '/dashboards/dashboard2', { method: 'HEAD' });
  assert.equal(head.status, 200);
  assert.equal(head.body.length, 0);
  assert.equal(head.headers['content-length'], String(index.bytes));
  const assetHead = await request(reference.origin, `/${metadata.assets[0].path}?v=synthetic`, { method: 'HEAD' });
  assert.equal(assetHead.status, 200);
  assert.equal(assetHead.body.length, 0);
  assert.equal(assetHead.headers['content-length'], String(metadata.assets[0].bytes));

  const cases = [
    ['POST', '/apps/ecommerce/addproduct', { method: 'POST', headers: { 'content-length': '1' }, body: 'x' }, 405],
    ['foreign Host', '/', { headers: { host: 'example.invalid' } }, 403],
    ['foreign Origin', '/', { headers: { origin: 'https://example.invalid' } }, 403],
    ['duplicate Origin', '/', { headers: { origin: [reference.origin, reference.origin] } }, 403],
    ['cross-site', '/', { headers: { 'sec-fetch-site': 'cross-site' } }, 403],
    ['same-site', '/', { headers: { 'sec-fetch-site': 'same-site' } }, 403],
    ['duplicate metadata', '/', { headers: { 'sec-fetch-site': ['none', 'none'] } }, 403],
    ['GET body', '/', { headers: { 'content-length': '1' }, body: 'x' }, 400],
    ['chunked GET', '/', { headers: { 'transfer-encoding': 'chunked' }, body: 'x' }, 400],
    ['absolute target', `${reference.origin}/`, {}, 400],
    ['authority target', '//example.invalid/', {}, 400],
    ['path alias', '/%64ashboards/dashboard2', {}, 400],
    ['encoded traversal', '/assets/%2e%2e/package.json', {}, 400],
    ['literal traversal', '/assets/../index.html', {}, 400],
    ['double slash', '/assets//missing.js', {}, 400],
    ['backslash', '/assets\\missing.js', {}, 400],
    ['malformed query', '/?value=%ff', {}, 400],
    ['query NUL', '/?value=%00', {}, 400],
    ['fragment', '/#fragment', {}, 400],
    ['long URI', `/?q=${'x'.repeat(2048)}`, {}, 400],
    ['missing asset', '/assets/appkit-missing.js', {}, 404],
    ['missing API', '/api/products', {}, 404],
    ['missing page', '/appkit-missing-page', {}, 404],
    ['source never served', '/src/main.tsx', {}, 404],
    ['manifest never served', '/package.json', {}, 404],
    ['dynamic ID is one segment', '/apps/blog/detail/a/b', {}, 404],
  ];
  for (const [name, path, options, status] of cases) {
    await t.test(name, async () => {
      const response = await request(reference.origin, path, options);
      assert.equal(response.status, status);
      securityHeaders(response);
      assert.notEqual(digest(response.body), index.sha256);
      if (status === 405) assert.equal(response.headers.allow, 'GET, HEAD');
    });
  }
  assert.equal(await websocketClosed(reference.origin), 0);
  await reference.close();
  await reference.close();
  await assert.rejects(request(reference.origin, '/'), error => error.code === 'ECONNREFUSED');
  t.diagnostic('366 asset hashes/MIME and 133 declared client-route responses checked; no React/browser execution or offline asset-closure claim.');
});

test('missing, changed, extra and nonregular inputs refuse before listening', async t => {
  for (const kind of ['missing source', 'changed source', 'oversized source', 'missing asset', 'changed asset', 'extra asset', 'symlink asset']) {
    await t.test(kind, async t => {
      const root = fixture(t, kind.includes('asset') ? 'first' : 'none');
      const relative = kind.includes('asset') ? `dist/${metadata.assets[0].path}` : metadata.source_pins[0].path;
      const target = join(root, relative);
      if (kind.startsWith('missing')) unlinkSync(target);
      else if (kind.startsWith('oversized')) truncateSync(target, 4 * 1024 * 1024 + 1);
      else if (kind.startsWith('extra')) writeFileSync(join(root, 'dist', 'unreviewed.txt'), 'synthetic');
      else if (kind.startsWith('symlink')) {
        unlinkSync(target);
        symlinkSync(join(root, metadata.source_pins[0].path), target);
      } else {
        const body = readFileSync(target);
        body[body.length - 1] ^= 1;
        writeFileSync(target, body);
      }
      const pattern = kind === 'missing source' ? /ENOENT/ : kind.includes('source') ? /source identity|bounds/ :
        kind === 'changed asset' ? /asset identity/ : kind === 'symlink asset' ? /symlink/ : /file set differs/;
      await assert.rejects(startReference(root), pattern);
    });
  }
  await assert.rejects(startReference('relative'), /absolute prepared/);
});

function copiedVerifier(t) {
  const root = ownedRoot(t, 'appkit-matdash-metadata-');
  mkdirSync(join(root, 'matdash-native'));
  mkdirSync(join(root, 'catalyst-native'));
  copyFileSync(join(here, 'reference-server.mjs'), join(root, 'matdash-native', 'reference-server.mjs'));
  copyFileSync(join(here, '../catalyst-native/reference-server.mjs'), join(root, 'catalyst-native', 'reference-server.mjs'));
  return root;
}

test('shared entry refuses malformed metadata options before filesystem access', async t => {
  const absentProject = join(ownedRoot(t), 'absent-project');
  const valid = { metadataFile: join(here, 'build-assets.json'), metadataSha256 };
  const cases = [
    ['missing options', undefined, /metadata options/],
    ['null options', null, /metadata options/],
    ['array options', [], /metadata options/],
    ['missing path', { metadataSha256 }, /absolute reference metadata/],
    ['relative path', { ...valid, metadataFile: 'build-assets.json' }, /absolute reference metadata/],
    ['non-string path', { ...valid, metadataFile: 7 }, /absolute reference metadata/],
    ['control path', { ...valid, metadataFile: `${valid.metadataFile}\n` }, /absolute reference metadata/],
    ['missing pin', { metadataFile: valid.metadataFile }, /SHA-256 metadata pin/],
    ['non-string pin', { ...valid, metadataSha256: 7 }, /SHA-256 metadata pin/],
    ['short pin', { ...valid, metadataSha256: 'a'.repeat(63) }, /SHA-256 metadata pin/],
    ['non-hex pin', { ...valid, metadataSha256: 'z'.repeat(64) }, /SHA-256 metadata pin/],
    ['uppercase pin', { ...valid, metadataSha256: metadataSha256.toUpperCase() }, /SHA-256 metadata pin/],
  ];
  for (const [name, options, pattern] of cases) {
    await t.test(name, async () => {
      await assert.rejects(startPinnedStaticReference(absentProject, options), pattern);
    });
  }
});

test('shared entry verifies its selected pin and delivers the same static bytes', { timeout: 15000 }, async t => {
  const root = ownedRoot(t);
  const metadataFile = join(root, 'selected-build.json');
  copyFileSync(join(here, 'build-assets.json'), metadataFile);
  await assert.rejects(startPinnedStaticReference(project, { metadataFile, metadataSha256: '0'.repeat(64) }), /metadata hash mismatch/);
  const reference = await startPinnedStaticReference(project, { metadataFile, metadataSha256 });
  t.after(() => reference.close());
  // Metadata is captured before listening, just like verified source and asset bytes.
  unlinkSync(metadataFile);
  const index = metadata.assets.find(file => file.path === 'index.html');
  const script = metadata.assets.find(file => file.path.endsWith('.js'));
  for (const [path, expected] of [['/dashboards/dashboard2', index], [`/${script.path}`, script]]) {
    const response = await request(reference.origin, path);
    assert.equal(response.status, 200);
    assert.equal(response.body.length, expected.bytes);
    assert.equal(digest(response.body), expected.sha256);
    assert.equal(response.headers['content-type'], expectedTypes[extname(expected.path)]);
    securityHeaders(response);
  }
  assert.equal((await request(reference.origin, '/package.json')).status, 404);
  assert.equal((await request(reference.origin, '/', { method: 'POST' })).status, 405);
  await reference.close();
  await assert.rejects(request(reference.origin, '/'), error => error.code === 'ECONNREFUSED');
});

test('shared entry serves reviewed plain-text license bytes without interpreting markup', async t => {
  const root = ownedRoot(t);
  mkdirSync(join(root, 'dist'));
  const source = Buffer.from('synthetic source identity\n');
  const index = Buffer.from('<!doctype html><title>Synthetic reference</title>');
  const license = Buffer.from('Synthetic license notice <script>not executed</script>\n');
  writeFileSync(join(root, 'source.txt'), source);
  writeFileSync(join(root, 'dist', 'index.html'), index);
  writeFileSync(join(root, 'dist', 'LICENSE.txt'), license);
  const record = (path, bytes) => ({ path, bytes: bytes.length, sha256: digest(bytes) });
  const bytes = Buffer.from(JSON.stringify({ schema: 1,
    source_pins: [record('source.txt', source)],
    assets: [record('index.html', index), record('LICENSE.txt', license)],
    routes: ['/'], total_bytes: index.length + license.length,
  }));
  const metadataFile = join(root, 'synthetic-build.json');
  writeFileSync(metadataFile, bytes);
  const reference = await startPinnedStaticReference(root, { metadataFile, metadataSha256: digest(bytes) });
  t.after(() => reference.close());
  const response = await request(reference.origin, '/LICENSE.txt');
  assert.equal(response.status, 200);
  assert.equal(response.headers['content-type'], 'text/plain; charset=utf-8');
  assert.deepEqual(response.body, license);
  securityHeaders(response);
  assert.equal((await request(reference.origin, '/source.txt')).status, 404);
});

test('metadata byte pin precedes path use', async t => {
  const root = copiedVerifier(t);
  const altered = structuredClone(metadata);
  altered.source_pins[0].path = '../outside-source';
  writeFileSync(join(root, 'matdash-native', 'build-assets.json'), JSON.stringify(altered));
  const copied = await import(pathToFileURL(join(root, 'matdash-native', 'reference-server.mjs')).href);
  await assert.rejects(copied.startReference(root), /metadata hash mismatch/);
});

test('regular-file to FIFO race refuses without waiting for a writer', t => {
  const root = copiedVerifier(t);
  copyFileSync(join(here, 'build-assets.json'), join(root, 'matdash-native', 'build-assets.json'));
  const childSource = `
    import assert from 'node:assert/strict';
    import fs from 'node:fs';
    import { syncBuiltinESMExports } from 'node:module';
    import { execFileSync } from 'node:child_process';
    import { join } from 'node:path';
    import { pathToFileURL } from 'node:url';
    const root = fs.realpathSync(process.argv[1]), target = join(root, 'matdash-native', 'build-assets.json');
    const realOpen = fs.openSync;
    let swapped = false;
    fs.openSync = (path, ...args) => {
      if (path === target && !swapped) {
        assert.ok(fs.lstatSync(target).isFile());
        fs.unlinkSync(target);
        execFileSync('/usr/bin/mkfifo', [target], { timeout: 1000 });
        swapped = true;
      }
      return realOpen(path, ...args);
    };
    syncBuiltinESMExports();
    const { startReference } = await import(pathToFileURL(join(root, 'matdash-native', 'reference-server.mjs')).href);
    await assert.rejects(startReference(root), /bounds or is nonregular/);
    assert.ok(swapped);
  `;
  const result = spawnSync(process.execPath, ['--input-type=module', '--eval', childSource, root], {
    encoding: 'utf8', timeout: 2500, killSignal: 'SIGKILL', env: {},
  });
  assert.equal(result.error, undefined, 'FIFO child must refuse, not reach the kill deadline');
  assert.equal(result.status, 0, result.stderr);
});

test('served bytes remain the verified snapshot after owned-copy changes', { timeout: 15000 }, async t => {
  // One bounded 14.6 MB build-only copy: never clone dependencies or mutate the prepared project.
  const root = fixture(t, 'full');
  const reference = await startReference(root);
  t.after(() => reference.close());
  const index = metadata.assets.find(file => file.path === 'index.html');
  const script = metadata.assets.find(file => file.path.endsWith('.js'));
  writeFileSync(join(root, 'dist', 'index.html'), 'changed only in owned copy');
  unlinkSync(join(root, 'dist', script.path));
  unlinkSync(join(root, metadata.source_pins[0].path));
  for (const [path, expected] of [['/dashboards/dashboard2', index], [`/${script.path}`, script]]) {
    const response = await request(reference.origin, path);
    assert.equal(response.status, 200);
    assert.equal(digest(response.body), expected.sha256);
  }
  await reference.close();
});

test('CLI usage and absent private input fail loudly without serving', () => {
  const options = { encoding: 'utf8', timeout: 5000, env: {} };
  const usage = spawnSync(process.execPath, [join(here, 'reference-server.mjs')], options);
  assert.equal(usage.status, 2);
  assert.equal(usage.stdout, '');
  assert.match(usage.stderr, /Usage:/);
  const missing = spawnSync(process.execPath, ['--test', fileURLToPath(import.meta.url)], options);
  assert.equal(missing.status, 1);
  assert.match(missing.stdout + missing.stderr, /Set APPKIT_MATDASH_REFERENCE_ROOT/);
  const relative = spawnSync(process.execPath, [join(here, 'reference-server.mjs'), 'relative'], options);
  assert.equal(relative.status, 1);
  assert.equal(relative.stdout, '');
  assert.match(relative.stderr, /reference refused/);
});

test('actual CLI signals close only their owned static listener', { timeout: 20000 }, async t => {
  for (const signal of ['SIGINT', 'SIGTERM']) {
    await t.test(signal, async t => {
      const child = spawn(process.execPath, [join(here, 'reference-server.mjs'), project], { env: {}, stdio: ['ignore', 'pipe', 'pipe'] });
      t.after(() => { if (child.exitCode === null && child.signalCode === null) child.kill('SIGKILL'); });
      let output = '', errors = '';
      const timer = setTimeout(() => child.kill('SIGKILL'), 8000);
      const exited = new Promise((resolve, reject) => {
        child.once('error', reject);
        child.once('exit', (code, stoppedBy) => { clearTimeout(timer); resolve({ code, stoppedBy }); });
      });
      child.stderr.on('data', bytes => { errors += bytes.toString('utf8'); if (errors.length > 4096) child.kill('SIGKILL'); });
      const origin = await new Promise((resolve, reject) => {
        child.once('error', reject);
        child.once('exit', () => reject(new Error('CLI exited before bounded ready output')));
        child.stdout.on('data', bytes => {
          output += bytes.toString('utf8');
          if (output.length > 4096) { child.kill('SIGKILL'); reject(new Error('CLI output exceeded bounds')); return; }
          const match = /^app-kit MatDash static reference: (http:\/\/127\.0\.0\.1:[1-9][0-9]*)\/dashboards\/dashboard2\n$/.exec(output);
          if (match) resolve(match[1]);
        });
      });
      assert.equal((await request(origin, '/dashboards/dashboard2')).status, 200);
      assert.ok(child.kill(signal));
      assert.deepEqual(await exited, { code: 0, stoppedBy: null });
      assert.equal(errors, '');
      await assert.rejects(request(origin, '/'), error => error.code === 'ECONNREFUSED');
    });
  }
});
