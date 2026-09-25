// Explicit private static asset evidence. Never executes vendor React/font/icon code.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { closeSync, constants, copyFileSync, fstatSync, lstatSync, mkdirSync, mkdtempSync, openSync, readFileSync, readSync, realpathSync, rmSync, unlinkSync, writeFileSync } from 'node:fs';
import http from 'node:http';
import { tmpdir } from 'node:os';
import { basename, dirname, extname, isAbsolute, join } from 'node:path';
import { spawn, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { startAssetReference } from './asset-reference.mjs';

const selected = process.env.APPKIT_MATDASH_ASSET_REFERENCE_ROOT;
assert.ok(selected && isAbsolute(selected), 'Set APPKIT_MATDASH_ASSET_REFERENCE_ROOT to an absolute prepared asset project; missing evidence is not a skip.');
assert.ok(lstatSync(selected).isDirectory() && !lstatSync(selected).isSymbolicLink());
const project = realpathSync(selected);
const here = dirname(fileURLToPath(import.meta.url));
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
const metadataFile = join(here, 'asset-build.json');
assert.ok(lstatSync(metadataFile).isFile() && lstatSync(metadataFile).size <= 128 * 1024);
const metadataBytes = readFileSync(metadataFile);
assert.equal(digest(metadataBytes), 'ab9a1a037634eff411d93de64a04ade52751638e282a9a2c7016e13b66f5896a', 'Pin metadata before deriving fixture paths.');
const metadata = JSON.parse(metadataBytes);
const referencePrefix = '__appkit_reference_assets/';
const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8', '.json': 'application/json; charset=utf-8', '.txt': 'text/plain; charset=utf-8',
  '.svg': 'image/svg+xml', '.png': 'image/png', '.jpg': 'image/jpeg', '.gif': 'image/gif', '.webp': 'image/webp', '.woff2': 'font/woff2' };

function ownedRoot(t) {
  const root = mkdtempSync(join(tmpdir(), 'appkit-matdash-asset-canary-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  return root;
}

function pinnedSource(relative) {
  const pin = metadata.source_pins.find(record => record.path === relative);
  assert.ok(pin, 'required source identity is absent');
  let target = project;
  for (const part of relative.split('/')) {
    target = join(target, part);
    assert.ok(!lstatSync(target).isSymbolicLink(), 'source symlink refused');
  }
  const fd = openSync(target, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const stat = fstatSync(fd);
    assert.ok(stat.isFile() && stat.size === pin.bytes && stat.size <= 4 * 1024 * 1024, 'source bounds or identity changed');
    const bytes = Buffer.alloc(pin.bytes + 1);
    let length = 0, size;
    while (length < bytes.length && (size = readSync(fd, bytes, length, bytes.length - length, null)) > 0) length += size;
    assert.equal(length, pin.bytes, 'source size changed while reading');
    const body = bytes.subarray(0, length);
    assert.equal(digest(body), pin.sha256, 'source identity changed');
    return body;
  } finally { closeSync(fd); }
}

function request(origin, path, method = 'GET') {
  const url = new URL(origin);
  assert.equal(url.protocol, 'http:');
  assert.equal(url.hostname, '127.0.0.1');
  return new Promise((resolve, reject) => {
    const req = http.request({ hostname: '127.0.0.1', port: url.port, path, method, agent: false }, response => {
      const chunks = [];
      let size = 0;
      response.on('data', chunk => {
        size += chunk.length;
        if (size > 4 * 1024 * 1024) req.destroy(new Error('response safety ceiling exceeded'));
        else chunks.push(chunk);
      });
      response.once('error', reject);
      response.once('end', () => resolve({ status: response.statusCode, headers: response.headers, body: Buffer.concat(chunks) }));
    });
    const timer = setTimeout(() => req.destroy(new Error('loopback request deadline exceeded')), 5000);
    req.once('close', () => clearTimeout(timer));
    req.once('error', reject);
    req.end();
  });
}

function securityHeaders(response) {
  assert.equal(response.headers['cache-control'], 'no-store');
  assert.equal(response.headers['x-content-type-options'], 'nosniff');
  assert.equal(response.headers['x-frame-options'], 'DENY');
  assert.equal(response.headers['referrer-policy'], 'no-referrer');
  assert.equal(response.headers['content-security-policy'], "frame-ancestors 'none'; base-uri 'self'; form-action 'self'");
  assert.equal(response.headers['x-appkit-reference'], 'matdash-static-read-only');
}

test('fixed asset variant delivers every pinned byte and declared client route', { timeout: 45000 }, async t => {
  assert.equal(metadata.source_pins.length, 24);
  assert.equal(metadata.assets.length, 383);
  assert.equal(metadata.routes.length, 133);
  assert.equal(metadata.total_bytes, 15247074);
  const reference = await startAssetReference(project);
  t.after(() => reference.close());
  const served = new Map();
  let total = 0;
  for (const asset of metadata.assets) {
    const response = await request(reference.origin, `/${asset.path}`);
    assert.equal(response.status, 200, asset.path);
    assert.equal(response.body.length, asset.bytes, asset.path);
    assert.equal(digest(response.body), asset.sha256, asset.path);
    assert.ok(types[extname(asset.path)], 'asset MIME oracle missing');
    assert.equal(response.headers['content-type'], types[extname(asset.path)], asset.path);
    securityHeaders(response);
    total += response.body.length;
    served.set(asset.path, response.body);
  }
  assert.equal(total, metadata.total_bytes);
  const index = served.get('index.html').toString('utf8');
  for (const route of metadata.routes) {
    const response = await request(reference.origin, route.replace(/:[A-Za-z_][A-Za-z0-9_]*/g, 'synthetic-123'));
    assert.equal(response.status, 200, route);
    assert.equal(response.headers['content-type'], types['.html']);
    assert.equal(digest(response.body), digest(served.get('index.html')), route);
  }
  for (const path of ['/dashboards/dashboard2', '/apps/ecommerce/addproduct', '/apps/ecommerce/editproduct']) {
    assert.ok(metadata.routes.includes(path));
    const response = await request(reference.origin, `${path}?reference=synthetic%20query`, 'HEAD');
    assert.equal(response.status, 200);
    assert.equal(response.body.length, 0);
    assert.equal(response.headers['content-length'], String(Buffer.byteLength(index)));
  }
  await t.test('both original font families retain all 22 rules and 10 local WOFF2 files', () => {
    const fontLinks = [...index.matchAll(/<link\b[^>]*\bhref="([^"<>]+)"[^>]*>/g)].map(match => match[1]).filter(href => href.endsWith('/manrope.css') || href.endsWith('/jakarta.css'));
    assert.deepEqual(fontLinks, [`/${referencePrefix}jakarta.css`, `/${referencePrefix}manrope.css`]);
    assert.ok(!index.includes('fonts.googleapis.com') && !index.includes('fonts.gstatic.com'));
    const files = new Set();
    let faces = 0;
    for (const [family, count, unique] of [['manrope', 6, 6], ['jakarta', 16, 4]]) {
      const original = pinnedSource(`reference-inputs/${family}-original.css`).toString('utf8');
      const local = served.get(`${referencePrefix}${family}.css`).toString('utf8');
      let replacements = 0;
      const rewritten = original.replace(/url\(https:\/\/fonts\.gstatic\.com\/[^)\s]+\/([A-Za-z0-9_-]+\.woff2)\)/g, (_all, name) => {
        replacements++;
        return `url(fonts/${name})`;
      });
      assert.equal(replacements, count, 'every original face has one pinned font URL');
      assert.ok(local === rewritten, 'local font CSS differs beyond exact font URL substitutions');
      const rules = [...local.matchAll(/@font-face\s*\{/g)].length;
      assert.equal(rules, count);
      faces += rules;
      assert.ok(!/@import\b|https?:\/\/|local\s*\(/i.test(local));
      const urls = [...local.matchAll(/url\(([^)]+)\)/g)].map(match => match[1]);
      assert.equal(urls.length, count);
      assert.equal(new Set(urls).size, unique);
      for (const url of urls) {
        assert.match(url, /^fonts\/[A-Za-z0-9_-]+\.woff2$/);
        const asset = `${referencePrefix}${url}`;
        const bytes = served.get(asset);
        assert.ok(bytes && bytes.length >= 48, 'font missing from verified HTTP closure');
        assert.equal(bytes.toString('ascii', 0, 4), 'wOF2');
        assert.equal(bytes.readUInt32BE(8), bytes.length, 'WOFF2 declared length mismatch');
        assert.equal(bytes.readUInt16BE(14), 0, 'WOFF2 reserved field must be zero');
        assert.ok(bytes.readUInt16BE(12) > 0, 'WOFF2 table count missing');
        files.add(asset);
      }
    }
    assert.equal(faces, 22);
    assert.equal(files.size, 10);
    assert.deepEqual([...files].sort(), metadata.assets.filter(asset => asset.path.endsWith('.woff2')).map(asset => asset.path).sort());
  });
  await t.test('local map and license files retain exact identities, without a shape claim', () => {
    const localFiles = metadata.assets.filter(asset => asset.path.startsWith(referencePrefix));
    assert.equal(localFiles.length, 17);
    const licenses = localFiles.filter(asset => asset.path.endsWith('.txt')).map(asset => basename(asset.path)).sort();
    assert.deepEqual(licenses, ['ICON-NOTICES.txt', 'Jakarta-OFL.txt', 'Manrope-OFL.txt', 'us-atlas-LICENSE.txt']);
    for (const name of licenses) assert.ok(served.get(referencePrefix + name).length > 0);
    const map = JSON.parse(served.get(`${referencePrefix}states-10m.json`));
    assert.equal(map.type, 'Topology');
    assert.deepEqual(Object.keys(map.objects).sort(), ['nation', 'states']);
    assert.equal(map.objects.states.type, 'GeometryCollection');
    assert.equal(map.objects.states.geometries.length, 56);
    assert.equal(map.objects.nation.type, 'GeometryCollection');
    assert.equal(map.objects.nation.geometries.length, 1);
    assert.equal(map.arcs.length, 419);
  });
  assert.equal((await request(reference.origin, '/src/reference-assets/iconsets.json')).status, 404);
  assert.equal((await request(reference.origin, '/reference-inputs/manrope-original.css')).status, 404);
  assert.equal((await request(reference.origin, '/', 'POST')).status, 405);
  await reference.close();
  await assert.rejects(request(reference.origin, '/'), error => error.code === 'ECONNREFUSED');
  t.diagnostic('383 asset identities/MIME, 133 routes, 22 preserved font-face rules, 10 WOFF2 headers and map counts checked. No React/font execution, visual fidelity or offline-app claim.');
});

test('missing or changed font, map and icon-preload inputs refuse before serving', { timeout: 30000 }, async t => {
  const root = ownedRoot(t);
  const records = [...metadata.source_pins.map(pin => ({ ...pin, relative: pin.path })), ...metadata.assets.map(asset => ({ ...asset, relative: `dist/${asset.path}` }))];
  for (const record of records) {
    const source = join(project, record.relative), target = join(root, record.relative);
    assert.ok(lstatSync(source).isFile() && !lstatSync(source).isSymbolicLink());
    mkdirSync(dirname(target), { recursive: true });
    copyFileSync(source, target, constants.COPYFILE_FICLONE);
  }
  const pristine = await startAssetReference(root);
  await pristine.close();
  const font = metadata.assets.find(asset => asset.path.endsWith('.woff2'));
  assert.ok(font);
  for (const [kind, relative] of [['font', `dist/${font.path}`], ['map', `dist/${referencePrefix}states-10m.json`], ['icon-preload', 'src/reference-assets/preload-icons.ts']]) {
    for (const operation of ['missing', 'changed']) {
      await t.test(`${operation} ${kind}`, async () => {
        const target = join(root, relative);
        if (operation === 'missing') unlinkSync(target);
        else writeFileSync(target, 'synthetic invalid identity\n');
        try { await assert.rejects(startAssetReference(root)); }
        finally { copyFileSync(join(project, relative), target); }
      });
    }
  }
  const restored = await startAssetReference(root);
  await restored.close();
});

test('CLI and explicit evidence input fail loudly when absent or invalid', t => {
  const script = join(here, 'asset-reference.mjs');
  const options = { encoding: 'utf8', timeout: 5000, env: {} };
  const usage = spawnSync(process.execPath, [script], options);
  assert.equal(usage.status, 2);
  assert.equal(usage.stdout, '');
  assert.match(usage.stderr, /Usage:/);
  const relative = spawnSync(process.execPath, [script, 'relative'], options);
  assert.equal(relative.status, 1);
  assert.equal(relative.stdout, '');
  assert.match(relative.stderr, /reference refused/);
  const absent = spawnSync(process.execPath, [script, join(ownedRoot(t), 'absent-project')], options);
  assert.equal(absent.status, 1);
  assert.equal(absent.stdout, '');
  assert.match(absent.stderr, /reference refused/);
  const extra = spawnSync(process.execPath, [script, project, '--metadata=unreviewed'], options);
  assert.equal(extra.status, 2);
  assert.equal(extra.stdout, '');
  const missing = spawnSync(process.execPath, ['--test', fileURLToPath(import.meta.url)], options);
  assert.equal(missing.status, 1);
  assert.match(missing.stdout + missing.stderr, /Set APPKIT_MATDASH_ASSET_REFERENCE_ROOT/);
});

test('asset CLI SIGINT and SIGTERM close only their owned loopback listener', { timeout: 20000 }, async t => {
  for (const signal of ['SIGINT', 'SIGTERM']) {
    await t.test(signal, async t => {
      const child = spawn(process.execPath, [join(here, 'asset-reference.mjs'), project], { env: {}, stdio: ['ignore', 'pipe', 'pipe'] });
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
        child.once('exit', () => reject(new Error('asset CLI exited before bounded ready output')));
        child.stdout.on('data', bytes => {
          output += bytes.toString('utf8');
          if (output.length > 4096) { child.kill('SIGKILL'); reject(new Error('asset CLI output exceeded bounds')); return; }
          const match = /^app-kit MatDash asset reference: (http:\/\/127\.0\.0\.1:[1-9][0-9]*)\/dashboards\/dashboard2\n$/.exec(output);
          if (match) resolve(match[1]);
        });
      });
      assert.equal((await request(origin, '/dashboards/dashboard2')).status, 200);
      child.kill(signal);
      assert.deepEqual(await exited, { code: 0, stoppedBy: null });
      assert.equal(errors, '');
      await assert.rejects(request(origin, '/'), error => error.code === 'ECONNREFUSED');
    });
  }
});
