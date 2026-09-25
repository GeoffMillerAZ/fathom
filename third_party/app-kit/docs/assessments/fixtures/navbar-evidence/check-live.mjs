// Read-only P09 historical-image inspection. Never executes retained vendor code.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';

const [url, root, pin, parent] = process.argv.slice(2);
assert.equal(process.argv.length, 6, 'usage: node check-live.mjs LOOPBACK_URL PRIVATE_ROOT REVIEWED_BUNDLE_SHA OUTPUT_PARENT');
const origin = new URL(url);
assert.equal(origin.protocol, 'http:');
assert.equal(origin.hostname, '127.0.0.1');
assert.equal(origin.username + origin.password + origin.search + origin.hash, '');
assert.equal(origin.pathname, '/');
assert.match(pin, /^[0-9a-f]{64}$/);
for (const dir of [root, parent]) {
  assert(path.isAbsolute(dir));
  const info = fs.lstatSync(dir);
  assert(info.isDirectory() && !info.isSymbolicLink());
}
assert.equal(fs.statSync(parent).mode & 0o777, 0o700);
assert.equal(fs.statSync(parent).uid, process.getuid());
const out = fs.mkdtempSync(path.join(parent, 'inspection-'));
console.log(`Private navbar inspection: ${out}`);
const sha = b => createHash('sha256').update(b).digest('hex');
const read = (filename, limit) => {
  const info = fs.lstatSync(filename);
  assert(info.isFile() && !info.isSymbolicLink() && info.size <= limit, 'bounded regular input required');
  return fs.readFileSync(filename);
};
const write = (name, body) => fs.writeFileSync(path.join(out, name), body, { flag: 'wx', mode: 0o600 });
const deadline = async (promise, ms = 15_000) => {
  let timer;
  try { return await Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(Error('operation deadline')), ms); })]); }
  finally { clearTimeout(timer); }
};
const report = {
  kind: 'private-navbar-evidence-inspection', verification: 'not-verified',
  server: { url: origin.origin, identity: 'caller-selected URL; process/binary attestation is separate' },
  inputs: {}, checks: [], images: [], captures: [], errors: [], cleanup: 'pending',
  limits: ['Historical viewer tasks, not component behavior or visual equivalence.',
    'No certification, consumer adoption, performance budget, assistive-input or cross-browser claim.',
    'Cleanup checks the owned Chrome parent only; descendant processes are not inventoried.'],
};
const check = (name, ok) => { assert(ok, name); report.checks.push(name); };
let server, browser, ownedProcess;
try {
  const harness = read(fileURLToPath(import.meta.url), 1 << 20);
  write('harness.mjs', harness);
  report.inputs.harness = sha(harness);
  const playwright = process.env.APPKIT_PLAYWRIGHT;
  assert(path.isAbsolute(playwright));
  report.inputs.playwright = sha(read(playwright, 1 << 20));
  const configuration = Buffer.from(JSON.stringify({ origin: origin.origin, root, pin, parent, playwright,
    widths: [390, 1440], height: 1000, themes: ['light', 'dark'], javaScriptEnabled: false,
    imagePhaseDeadlineMS: 180_000, imageRequestDeadlineMS: 30_000 }, null, 2) + '\n');
  write('configuration.json', configuration); report.inputs.configuration = sha(configuration);
  const bundlePath = path.join(root, 'evidence', 'entries', `${pin}.json`);
  const bundleBytes = read(bundlePath, 92 << 20);
  assert.equal(sha(bundleBytes), pin);
  report.inputs.bundle = pin;
  const bundle = JSON.parse(bundleBytes);
  assert.equal(bundle.contract.subject.id, 'reference.navbar-adapter');
  assert.equal(bundle.contract.subject.source_revision, '79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7');
  assert.equal(bundle.contract.subject.artifact_sha256, '67f015c1efa6c41b2acb38d56ac00ecc24b4c7d406729655d8330d425a5a6137');
  assert.equal(bundle.receipt.results.interaction.status, 'failed');
  assert.equal(Object.keys(bundle.receipt.results).length, 9);
  assert(Object.values(bundle.receipt.results).every(r => ['failed', 'not-run'].includes(r.status)));
  const slots = Object.values(bundle.receipt.results).flatMap(r => r.evidence).filter(e => e.role === 'image');
  assert.equal(slots.length, 114);
  assert.equal(new Set(slots.map(s => s.id)).size, slots.length);
  const route = `/target-evidence/evidence@${pin}`;
  const imagePhaseDeadline = AbortSignal.timeout(180_000);
  for (const slot of slots) {
    assert.match(slot.id, /^[a-z0-9][a-z0-9._-]*$/);
    const expected = Buffer.from(bundle.content[slot.id], 'base64');
    assert.equal(sha(expected), slot.sha256);
    const started = performance.now();
    const response = await fetch(new URL(`${route}/images/${slot.id}`, origin), { signal: AbortSignal.any([imagePhaseDeadline, AbortSignal.timeout(30_000)]), redirect: 'error' });
    assert.equal(response.status, 200);
    for (const [key, value] of Object.entries({ 'content-type': 'image/png', 'cache-control': 'no-store', 'x-appkit-verification': 'not-verified', 'x-appkit-evidence-sha256': slot.sha256 })) assert.equal(response.headers.get(key), value);
    const chunks = []; let length = 0;
    for await (const chunk of response.body) { length += chunk.length; assert(length <= 8 << 20); chunks.push(chunk); }
    const served = Buffer.concat(chunks);
    assert.equal(served.length, slot.bytes);
    assert.deepEqual(served, expected);
    report.images.push({ slot: slot.id, sha256: sha(served), bytes: served.length, elapsed_ms: performance.now() - started });
  }
  check('all 114 retained PNG HTTP bodies match exact private bytes', true);
  const { chromium } = await import(playwright);
  server = await chromium.launchServer({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, timeout: 30_000 });
  ownedProcess = server.process(); report.browser_pid = ownedProcess.pid;
  browser = await chromium.connect(server.wsEndpoint(), { timeout: 15_000 });
  report.browser = browser.version();
  for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
    const key = `${width}-${theme}`;
    const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme: theme, reducedMotion: 'reduce', javaScriptEnabled: false });
    try {
      await context.route('**/*', async request => {
        if (new URL(request.request().url()).origin !== origin.origin) { report.errors.push('external request refused'); await request.abort(); }
        else await request.continue();
      });
      const page = await context.newPage();
      page.setDefaultTimeout(15_000); page.setDefaultNavigationTimeout(30_000);
      page.on('pageerror', e => report.errors.push(e.message));
      page.on('console', e => { if (e.type() === 'error') report.errors.push(e.text()); });
      page.on('requestfailed', r => report.errors.push(`${r.url()}: ${r.failure()?.errorText}`));
      await page.goto(new URL(`/catalog/reference.navbar-adapter?theme=${theme}`, origin).href);
      await page.getByRole('link', { name: 'Inspect retained target evidence and captures' }).click();
      check(`${key}: exact subject discovery`, new URL(page.url()).searchParams.get('subject_id') === 'reference.navbar-adapter');
      await page.locator(`a[href^="${route}?"]`).click();
      const options = await page.locator('#evidence-image option').evaluateAll(items => items.map(e => e.value));
      assert.deepEqual([...options].sort(), slots.map(s => s.id).sort());
      check(`${key}: exact complete capture inventory without duplicates`, options.length === 114);
      const zoomWidth = width === 390 ? 780 : 1440;
      const pairs = [
        [`navbar-base-${theme}-${width}-original${width === 390 ? '-open' : ''}`, `navbar-base-${theme}-${width}-reference${width === 390 ? '-open' : ''}`],
        [`navbar-baseline-zoom-${theme}-${zoomWidth}-typed-first-account`, `navbar-runtime-zoom-${theme}-${zoomWidth}-typed-first-account`],
        [`navbar-delivery-${theme}-${width}-typed-account`, `navbar-delivery-${theme}-${width}-minimal`],
      ];
      for (const [i, [first, second]] of pairs.entries()) {
        assert(slots.some(s => s.id === first) && slots.some(s => s.id === second));
        await page.locator('#evidence-image').focus();
        await page.keyboard.type(first);
        await page.keyboard.press('Tab');
        check(`${key}/${i}: comparison keyboard focus`, await page.locator('#evidence-compare').evaluate(e => e === document.activeElement));
        await page.keyboard.type(second);
        await page.keyboard.press('Tab');
        check(`${key}/${i}: submit keyboard focus`, await page.getByRole('button', { name: 'Show captures' }).evaluate(e => e === document.activeElement));
        await Promise.all([page.waitForURL(u => u.searchParams.get('image') === first && u.searchParams.get('compare') === second), page.keyboard.press('Enter')]);
        check(`${key}/${i}: theme retained`, new URL(page.url()).searchParams.get('theme') === theme);
        for (const image of await page.locator('figure img').all()) { await image.scrollIntoViewIfNeeded(); await deadline(image.evaluate(e => e.decode())); }
        const geometry = await page.locator('figure img').evaluateAll(images => images.map(e => { const r = e.getBoundingClientRect(); return { src: e.getAttribute('src'), x: r.x, width: r.width, height: r.height, top: r.top, bottom: r.bottom, naturalWidth: e.naturalWidth, naturalHeight: e.naturalHeight }; }));
        check(`${key}/${i}: decoded selected order`, geometry.length === 2 && geometry[0].src.endsWith(first) && geometry[1].src.endsWith(second) && geometry.every(g => g.naturalWidth > 0));
        check(`${key}/${i}: proportional unclipped stack`, geometry[1].top >= geometry[0].bottom && geometry.every(g => g.x >= 0 && g.x + g.width <= width + 1 && Math.abs(g.width / g.height - g.naturalWidth / g.naturalHeight) < 0.01));
        check(`${key}/${i}: no document horizontal overflow`, await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
        if (i === 2) {
          await page.locator('#evidence-captures').scrollIntoViewIfNeeded();
          const image = await page.screenshot();
          assert.equal(image.readUInt32BE(16), width); assert.equal(image.readUInt32BE(20), 1000);
          const filename = `${key}.png`; write(filename, image);
          report.captures.push({ file: filename, purpose: 'comparison controls', sha256: sha(image), bytes: image.length, geometry });
          for (const [index, selected] of [first, second].entries()) {
            const displayed = page.locator('figure img').nth(index);
            await displayed.scrollIntoViewIfNeeded();
            const bounds = await displayed.boundingBox();
            check(`${key}/${index}: entire selected image inside viewport`, bounds && bounds.x >= 0 && bounds.y >= 0 && bounds.x + bounds.width <= width + 1 && bounds.y + bounds.height <= 1001);
            const screenshot = await page.screenshot();
            assert.equal(screenshot.readUInt32BE(16), width); assert.equal(screenshot.readUInt32BE(20), 1000);
            const name = `${key}-image-${index + 1}.png`; write(name, screenshot);
            report.captures.push({ file: name, purpose: 'complete displayed image', selected, bounds, sha256: sha(screenshot), bytes: screenshot.length });
          }
          const fullSize = page.locator(`figure a[href="${route}/images/${first}"]`);
          check(`${key}: exact full-size link`, await fullSize.count() === 1);
          const [response] = await Promise.all([page.waitForNavigation(), fullSize.click()]);
          assert.equal(response.status(), 200);
          assert.equal(response.headers()['content-type'], 'image/png');
          const bytes = await response.body();
          assert.equal(sha(bytes), slots.find(s => s.id === first).sha256);
          const nativeImage = await page.locator('img').evaluate(async e => { await e.decode(); return { width: e.naturalWidth, height: e.naturalHeight }; });
          assert.deepEqual(nativeImage, { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) });
          check(`${key}: full-size navigation`, new URL(page.url()).pathname === `${route}/images/${first}`);
          await page.goBack();
          check(`${key}: Back restores selection`, new URL(page.url()).searchParams.get('image') === first && new URL(page.url()).searchParams.get('compare') === second);
        }
      }
    } finally { await deadline(context.close()); }
  }
  check('no browser errors or external requests', report.errors.length === 0);
  check('bundle unchanged', sha(read(bundlePath, 92 << 20)) === pin);
  check('executed harness unchanged', sha(read(fileURLToPath(import.meta.url), 1 << 20)) === report.inputs.harness);
} catch (error) { report.errors.push(error.stack); process.exitCode = 1; }
finally {
  try {
    if (browser) await deadline(browser.close());
    if (server) await deadline(server.close());
    assert(!ownedProcess || ownedProcess.exitCode !== null || ownedProcess.signalCode !== null, 'owned browser still live');
    report.cleanup = 'owned browser parent stopped';
  } catch (error) {
    report.errors.push(`cleanup: ${error.stack}`); process.exitCode = 1;
    if (server) await deadline(server.kill()).catch(e => report.errors.push(e.message));
    report.cleanup = 'failed';
  }
  write(report.errors.length ? 'failure.json' : 'receipt.json', JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify({ out, checks: report.checks.length, images: report.images.length, captures: report.captures.length, errors: report.errors.length, cleanup: report.cleanup }));
}
