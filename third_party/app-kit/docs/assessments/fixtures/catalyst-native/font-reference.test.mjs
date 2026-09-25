// Explicit private fixture test, never a default kit runtime or browser claim.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { copyFileSync, mkdtempSync, mkdirSync, readFileSync, rmSync, symlinkSync, unlinkSync, writeFileSync } from 'node:fs';
import http from 'node:http';
import { tmpdir } from 'node:os';
import { dirname, isAbsolute, join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath, pathToFileURL } from 'node:url';
import test from 'node:test';
import { verifyFontReference } from './font-reference.mjs';
import { startReference } from './reference-server.mjs';

process.env.NEXT_TELEMETRY_DISABLED = '1';
const project = process.env.APPKIT_CATALYST_FONT_REFERENCE_ROOT;
assert.ok(project && isAbsolute(project), 'Set APPKIT_CATALYST_FONT_REFERENCE_ROOT to an absolute prepared project; missing evidence is not a skip.');
const here = dirname(fileURLToPath(import.meta.url));
const metadataPath = join(here, 'font-assets.json');
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
const metadataBytes = readFileSync(metadataPath);
assert.equal(digest(metadataBytes), 'dc7298020ee8a2e2502e7e038e8941c98123a03db4ffc8f52b6ff7fdc41f4655', 'Pin metadata before deriving any fixture-copy paths.');
const metadata = JSON.parse(metadataBytes);
const bundlePath = 'public/__appkit_reference_fonts/46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441';
const selectedSources = ['package.json', 'package-lock.json', 'src/app/(app)/settings/page.tsx', 'src/app/layout.tsx', 'src/styles/tailwind.css'];
const selectedAssets = ['inter.css', 'LICENSE.txt', ...metadata.fonts.map(font => font.path)];

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'appkit-font-canary-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  // Only five small selected sources plus the 4.93 MB passive bundle. No Next,
  // node_modules, database, browser profile or application cache is copied.
  for (const path of [...selectedSources, ...selectedAssets.map(path => `${bundlePath}/${path}`)]) {
    mkdirSync(dirname(join(root, path)), { recursive: true });
    copyFileSync(join(project, path), join(root, path));
  }
  return root;
}

test('static pinned source and complete font closure', () => {
  const proof = verifyFontReference(project);
  assert.equal(proof.metadata_sha256, 'dc7298020ee8a2e2502e7e038e8941c98123a03db4ffc8f52b6ff7fdc41f4655');
  assert.equal(proof.font_count, 38);
  assert.equal(proof.files.length, 40);
  assert.equal(proof.font_bytes, 4914456);
  assert.equal(proof.bundle_bytes, 4930333);
  const theme = readFileSync(join(project, 'src/styles/tailwind.css'), 'utf8');
  assert.match(theme, /--font-sans:\s*Inter, sans-serif;/);
  assert.match(theme, /--font-sans--font-feature-settings:\s*'cv11';/);
});

test('static missing, changed, extra and symlink inputs fail in disposable copies', async t => {
  const font = `${bundlePath}/${metadata.fonts[0].path}`;
  const cases = [
    ['missing CSS', `${bundlePath}/inter.css`, 'missing', /closure differs/],
    ['corrupt CSS', `${bundlePath}/inter.css`, 'corrupt', /hash mismatch/],
    ['missing font', font, 'missing', /closure differs/],
    ['corrupt font', font, 'corrupt', /hash mismatch/],
    ['WOFF2 magic', font, 'magic', /WOFF2 header/],
    ['WOFF2 length', font, 'length', /WOFF2 header/],
    ['changed layout', 'src/app/layout.tsx', 'corrupt', /source hash mismatch/],
    ['changed package', 'package.json', 'corrupt', /source hash mismatch/],
    ['changed lock', 'package-lock.json', 'corrupt', /source hash mismatch/],
    ['changed P10', 'src/app/(app)/settings/page.tsx', 'corrupt', /source hash mismatch/],
    ['changed theme', 'src/styles/tailwind.css', 'corrupt', /source hash mismatch/],
    ['symlink font', font, 'symlink', /symlink refused/],
    ['extra bundle file', `${bundlePath}/unreviewed.txt`, 'extra', /closure differs/],
  ];
  for (const [name, path, kind, error] of cases) {
    await t.test(name, t => {
      const root = fixture(t), target = join(root, path);
      if (kind === 'missing') unlinkSync(target);
      else if (kind === 'extra') writeFileSync(target, 'synthetic extra file');
      else if (kind === 'symlink') {
        unlinkSync(target);
        // Target remains inside this test's disposable directory.
        symlinkSync(join(root, bundlePath, 'LICENSE.txt'), target);
      } else {
        const bytes = readFileSync(target);
        if (kind === 'magic') bytes[0] ^= 1;
        else if (kind === 'length') bytes.writeUInt32BE(bytes.length + 1, 8);
        else bytes[bytes.length - 1] ^= 1;
        writeFileSync(target, bytes);
      }
      assert.throws(() => verifyFontReference(root), error);
    });
  }
  assert.throws(() => verifyFontReference('relative-project'), /absolute prepared/);
  // Recheck the original prepared inputs: the canaries never mutate them.
  assert.equal(verifyFontReference(project).bundle_bytes, 4930333);
});

test('static altered metadata is refused before resolving any supplied path', async t => {
  for (const path of ['../outside.woff2', '/outside.woff2', 'font-files/../../outside.woff2', 'font-files\\outside.woff2', 'font-files/%2e%2e/outside.woff2']) {
    await t.test(path, async t => {
      const root = mkdtempSync(join(tmpdir(), 'appkit-font-metadata-'));
      t.after(() => rmSync(root, { recursive: true, force: true }));
      copyFileSync(join(here, 'font-reference.mjs'), join(root, 'font-reference.mjs'));
      const changed = structuredClone(metadata);
      changed.fonts[0].path = path;
      writeFileSync(join(root, 'font-assets.json'), JSON.stringify(changed));
      const copied = await import(pathToFileURL(join(root, 'font-reference.mjs')).href);
      assert.throws(() => copied.verifyFontReference(root), /metadata hash mismatch/);
    });
  }
});

test('static regular-file to FIFO race refuses promptly before reading', t => {
  const root = mkdtempSync(join(tmpdir(), 'appkit-font-fifo-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  copyFileSync(join(here, 'font-reference.mjs'), join(root, 'font-reference.mjs'));
  copyFileSync(metadataPath, join(root, 'font-assets.json'));
  // Interposition exists only in a bounded child. Swap this child's own copy
  // after lstat has accepted it, immediately before the actual OS open call.
  const childSource = `
    import assert from 'node:assert/strict';
    import fs from 'node:fs';
    import { execFileSync } from 'node:child_process';
    import { syncBuiltinESMExports } from 'node:module';
    import { join } from 'node:path';
    import { pathToFileURL } from 'node:url';
    const root = fs.realpathSync(process.argv[1]);
    const target = join(root, 'font-assets.json');
    const realOpen = fs.openSync;
    let replaced = false;
    fs.openSync = (path, ...args) => {
      if (path === target && !replaced) {
        assert.ok(fs.lstatSync(target).isFile());
        fs.unlinkSync(target);
        execFileSync('/usr/bin/mkfifo', [target], { timeout: 1000 });
        assert.ok(fs.lstatSync(target).isFIFO());
        replaced = true;
      }
      return realOpen(path, ...args);
    };
    syncBuiltinESMExports();
    const { verifyFontReference } = await import(pathToFileURL(join(root, 'font-reference.mjs')).href);
    assert.throws(() => verifyFontReference(root), /Reference file exceeds bounds: font-assets.json/);
    assert.ok(replaced, 'the race was not exercised');
    console.log(JSON.stringify({ replaced, refused: true }));
  `;
  const result = spawnSync(process.execPath, ['--input-type=module', '--eval', childSource, root], {
    encoding: 'utf8', timeout: 2500, killSignal: 'SIGKILL', env: { NEXT_TELEMETRY_DISABLED: '1' },
  });
  assert.equal(result.error, undefined, `FIFO open hung or child failed: ${result.error}`);
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(JSON.parse(result.stdout), { replaced: true, refused: true });
});

test('static CLI is read-only JSON on success and loud on missing input', () => {
  const cli = join(here, 'font-reference.mjs');
  const run = args => spawnSync(process.execPath, [cli, ...args], { encoding: 'utf8', timeout: 5000,
    env: { NEXT_TELEMETRY_DISABLED: '1' } });
  const good = run([project]);
  assert.equal(good.status, 0, good.stderr);
  assert.equal(good.stderr, '');
  assert.equal(JSON.parse(good.stdout).files.length, 40);
  const missing = run([]);
  assert.equal(missing.status, 2);
  assert.equal(missing.stdout, '');
  assert.match(missing.stderr, /Usage:/);
  const bad = run(['relative']);
  assert.equal(bad.status, 1);
  assert.equal(bad.stdout, '');
  assert.match(bad.stderr, /absolute prepared/);
  const missingRoot = spawnSync(process.execPath, ['--test', fileURLToPath(import.meta.url)], {
    encoding: 'utf8', timeout: 5000, env: { NEXT_TELEMETRY_DISABLED: '1' },
  });
  assert.equal(missingRoot.status, 1);
  assert.match(missingRoot.stdout + missingRoot.stderr, /Set APPKIT_CATALYST_FONT_REFERENCE_ROOT/);
});

function localURL(value, origin) {
  assert.ok(value && !/[\x00-\x20\x7f\\]/.test(value), 'invalid passive URL');
  const url = new URL(value.replaceAll('&amp;', '&'), `${origin}/settings`);
  assert.equal(url.origin, origin, 'passive resource would leave the owned loopback server');
  assert.equal(url.username + url.password + url.hash, '');
  return url.pathname + url.search;
}

function passiveURLs(html, origin) {
  // The fixed trusted SSR specimen emits quoted attributes. Remove comments and
  // inline script bodies so strings inside Flight data cannot look like elements.
  const tags = html.replace(/<!--[\s\S]*?-->/g, '').replace(/(<script\b[^>]*>)[\s\S]*?<\/script\s*>/gi, '$1</script>');
  const urls = new Map();
  for (const match of tags.matchAll(/<(script|link|img)\b([^>]*)>/gi)) {
    const tag = match[1].toLowerCase(), attrs = new Map();
    for (const attribute of match[2].matchAll(/([^\s=/>]+)\s*=\s*(?:"([^"]*)"|'([^']*)')/g)) {
      const name = attribute[1].toLowerCase();
      assert.ok(!attrs.has(name), 'ambiguous passive-resource attribute');
      attrs.set(name, attribute[2] ?? attribute[3]);
    }
    // Do not silently omit URLs if this pinned specimen changes its grammar.
    for (const attribute of match[2].matchAll(/(?:^|\s)(src|href|srcset|imagesrcset)\s*=/gi)) {
      assert.ok(attrs.has(attribute[1].toLowerCase()), 'unquoted passive-resource URL requires review');
    }
    if (tag === 'link') {
      const relations = (attrs.get('rel') ?? '').toLowerCase().split(/\s+/);
      assert.ok(!relations.some(rel => ['preconnect', 'dns-prefetch'].includes(rel)), 'unexpected external connection hint');
      assert.ok(relations.some(rel => ['stylesheet', 'preload', 'modulepreload', 'icon', 'apple-touch-icon', 'mask-icon'].includes(rel)), 'unreviewed link relation');
    }
    const value = attrs.get(tag === 'link' ? 'href' : 'src');
    if (value !== undefined) urls.set(localURL(value, origin), tag === 'link' && attrs.get('rel') === 'stylesheet' ? 'css' : tag);
    const srcset = attrs.get(tag === 'link' ? 'imagesrcset' : 'srcset');
    if (srcset) {
      for (const candidate of srcset.split(',')) {
        const [path, descriptor, extra] = candidate.trim().split(/\s+/);
        assert.ok(!extra && (!descriptor || /^\d+(?:\.\d+)?[wx]$/.test(descriptor)), 'unreviewed srcset grammar');
        urls.set(localURL(path, origin), 'img');
      }
    }
  }
  return urls;
}

test('static passive extraction excludes anchors, comments and inline scripts', () => {
  const origin = 'http://127.0.0.1:18473';
  const html = `<a href="https://example.invalid/no-navigation">Not an asset</a>
    <!-- <img src="https://example.invalid/comment"> -->
    <script>const example = '<img src="https://example.invalid/script-string">';</script>
    <script src="/app.js"></script><link rel="stylesheet" href="/app.css">
    <img src="/photo.png" srcset="/small.png 1x, /large.png 2x">`;
  assert.deepEqual([...passiveURLs(html, origin).keys()], ['/app.js', '/app.css', '/photo.png', '/small.png', '/large.png']);
  for (const invalid of [
    '<img src="https://example.invalid/photo.png">',
    '<script src="//example.invalid/app.js"></script>',
    '<link rel="preconnect" href="https://example.invalid">',
    '<img src=/unquoted.png>',
    '<img src="/first.png" src="/second.png">',
  ]) assert.throws(() => passiveURLs(invalid, origin));
});

function request(origin, path) {
  const base = new URL(origin);
  assert.equal(base.protocol, 'http:');
  assert.equal(base.hostname, '127.0.0.1');
  assert.equal(localURL(path, origin), path);
  return new Promise((resolve, reject) => {
    const req = http.get({ hostname: '127.0.0.1', port: base.port, path, agent: false }, res => {
      const chunks = [];
      let size = 0;
      res.on('data', chunk => {
        size += chunk.length;
        if (size > 2 * 1024 * 1024) req.destroy(new Error('response safety ceiling exceeded'));
        else chunks.push(chunk);
      });
      res.once('error', reject);
      res.once('end', () => resolve({ status: res.statusCode, headers: res.headers, bytes: Buffer.concat(chunks) }));
    });
    const timer = setTimeout(() => req.destroy(new Error('local request deadline exceeded')), 5000);
    req.once('close', () => clearTimeout(timer));
    req.once('error', reject);
  });
}

function noReporting(headers) {
  for (const name of ['nel', 'report-to', 'reporting-endpoints']) assert.equal(headers[name], undefined, `upstream reporting header leaked: ${name}`);
}

test('HTTP prepared settings and complete passive font closure', { timeout: 45000 }, async t => {
  // Verification precedes the only vendor runtime load in this test file.
  const proof = verifyFontReference(project);
  const reference = await startReference(project);
  let closed = false;
  t.after(async () => { if (!closed) await reference.close(); });
  const page = await request(reference.origin, '/settings');
  assert.equal(page.status, 200);
  noReporting(page.headers);
  const html = page.bytes.toString('utf8');
  assert.match(html, /<title>Settings - Catalyst<\/title>/);
  assert.match(html, /<h1\b[^>]*>Settings<\/h1>/);
  assert.ok(html.includes(proof.stylesheet), 'pinned local stylesheet is not in the actual HTML');
  assert.ok(!/https?:\/\/rsms\.me\//i.test(html), 'remote font URL remains in the rendered page');
  const passive = passiveURLs(html, reference.origin);
  assert.equal(passive.get(proof.stylesheet), 'css');
  assert.ok([...passive.values()].includes('script'));
  assert.ok([...passive.values()].includes('img'));
  let generatedCSS = '';
  for (const [path, kind] of passive) {
    const resource = await request(reference.origin, path);
    assert.equal(resource.status, 200, `passive resource ${path}`);
    assert.ok(resource.bytes.length > 0, path);
    noReporting(resource.headers);
    if (kind === 'css') {
      assert.match(resource.headers['content-type'], /^text\/css/);
      if (path !== proof.stylesheet) generatedCSS += resource.bytes.toString('utf8');
    } else if (kind === 'script') assert.match(resource.headers['content-type'], /(?:java|ecma)script/);
    else if (kind === 'img') assert.match(resource.headers['content-type'], /^image\//);
  }
  assert.match(generatedCSS, /--font-sans:\s*Inter\s*,\s*sans-serif/);
  assert.match(generatedCSS, /--font-sans--font-feature-settings:\s*["']cv11["']/);
  for (const file of proof.files) {
    const resource = await request(reference.origin, file.url);
    assert.equal(resource.status, 200, file.path);
    assert.equal(resource.bytes.length, file.size, file.path);
    assert.equal(digest(resource.bytes), file.sha256, file.path);
    noReporting(resource.headers);
    if (file.path.endsWith('.woff2')) {
      assert.match(resource.headers['content-type'], /^font\/woff2(?:;|$)/);
      assert.equal(resource.bytes.toString('ascii', 0, 4), 'wOF2');
      assert.equal(resource.bytes.readUInt32BE(8), resource.bytes.length);
      assert.ok(file.url.endsWith('?v=4.1'));
    } else assert.match(resource.headers['content-type'], file.path.endsWith('.css') ? /^text\/css/ : /^text\/plain/);
  }
  t.diagnostic(`HTTP pass: ${passive.size} distinct script/link/img resource URLs emitted by the fixed SSR HTML; 38 font URLs; 40 pinned bundle files, 4930333 bytes. Not all browser egress, visual or whole-app offline proof.`);
  await reference.close();
  closed = true;
  await assert.rejects(request(reference.origin, '/settings'), error => error.code === 'ECONNREFUSED');
});
