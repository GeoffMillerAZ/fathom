// Explicit read-only P07/P08 retention probe. Private outputs; no source execution,
// signed-in browser, remote traffic, publication or component certification.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';

const origin = new URL(process.env.APPKIT_OVERLAY_URL);
assert.equal(origin.hostname, '127.0.0.1');
assert.equal(origin.protocol, 'http:');
const root = process.env.APPKIT_OVERLAY_EVIDENCE_ROOT;
const parent = process.env.APPKIT_OVERLAY_OUTPUT_PARENT;
assert.ok(path.isAbsolute(root) && path.isAbsolute(parent));
const parentInfo = fs.lstatSync(parent);
assert.ok(parentInfo.isDirectory() && !parentInfo.isSymbolicLink(), 'output parent must be a real directory');
assert.equal(parentInfo.mode & 0o777, 0o700, 'output parent must have mode 0700');
assert.equal(parentInfo.uid, process.getuid(), 'output parent must be owned by the current user');
const out = fs.mkdtempSync(path.join(parent, 'inspection-'));
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const specs = [
  { name: 'menu', pin: '66aff1381147060cae5488e8c26a3c4491fb37109ce33a78391fba1a5d5c926f', images: 52 },
  { name: 'drawer', pin: '1ce15a0ce6e58db27a03c87654f3c8a97580c10111483c8cc5489159364a53dc', images: 44 },
];
const binaryPath = process.env.APPKIT_OVERLAY_BINARY;
assert.ok(path.isAbsolute(binaryPath));
const binarySHA256 = sha(fs.readFileSync(binaryPath));
const inputs = Object.fromEntries([fileURLToPath(import.meta.url), process.env.APPKIT_PLAYWRIGHT, ...specs.map(s => path.join(root, 'evidence', 'entries', `${s.pin}.json`))].map(p => [p, sha(fs.readFileSync(p))]));
const report = {
  verification: 'not-verified',
  provenance_context: {
    server: { url: origin.origin, attestation: 'caller-selected loopback URL; no server-process or binary identity attestation' },
    binary: { path: binaryPath, sha256: binarySHA256, executed: false, bound_to_server: false },
  },
  inputs, checks: [], images: [], captures: [], errors: [], browser: null,
};
const check = (name, value) => { assert.ok(value, name); report.checks.push(name); };
let browser;
try {
  for (const spec of specs) {
    const id = `evidence@${spec.pin}`;
    const file = path.join(root, 'evidence', 'entries', `${spec.pin}.json`);
    assert.equal(sha(fs.readFileSync(file)), spec.pin);
    const bundle = JSON.parse(fs.readFileSync(file));
    assert.equal(bundle.receipt.results.interaction.status, 'failed');
    const slots = [...new Map(Object.values(bundle.receipt.results).flatMap(r => r.evidence).filter(e => e.role === 'image').map(e => [e.id, e])).values()];
    assert.equal(slots.length, spec.images);
    for (const slot of slots) {
      const expected = Buffer.from(bundle.content[slot.id], 'base64');
      assert.equal(sha(expected), slot.sha256);
      const response = await fetch(new URL(`/target-evidence/${id}/images/${slot.id}`, origin), { signal: AbortSignal.timeout(30_000) });
      assert.equal(response.status, 200);
      assert.equal(response.headers.get('content-type'), 'image/png');
      assert.equal(response.headers.get('cache-control'), 'no-store');
      assert.equal(response.headers.get('x-appkit-verification'), 'not-verified');
      assert.equal(response.headers.get('x-appkit-evidence-sha256'), slot.sha256);
      const served = Buffer.from(await response.arrayBuffer());
      assert.deepEqual(served, expected);
      assert.equal(served.length, slot.bytes);
      report.images.push({ record: id, slot: slot.id, sha256: sha(served), bytes: served.length });
    }
    check(`${spec.name}: all retained images match HTTP bytes`, true);
  }
  const { chromium } = await import(process.env.APPKIT_PLAYWRIGHT);
  browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true });
  report.browser = browser.version();
  for (const spec of specs) for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
    const key = `${spec.name}-${width}-${theme}`;
    const route = `/target-evidence/evidence@${spec.pin}`;
    const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme: theme, reducedMotion: 'reduce', javaScriptEnabled: false });
    await context.route('**/*', async route => {
      if (new URL(route.request().url()).origin !== origin.origin) { report.errors.push(`external: ${route.request().url()}`); await route.abort(); }
      else await route.continue();
    });
    const page = await context.newPage();
    page.on('pageerror', e => report.errors.push(e.message));
    page.on('console', e => { if (e.type() === 'error') report.errors.push(e.text()); });
    page.on('requestfailed', r => report.errors.push(`failed: ${r.url()}: ${r.failure()?.errorText}`));
    await page.goto(new URL(`/catalog/reference.${spec.name}-adapter?theme=${theme}`, origin).href);
    await page.getByRole('link', { name: 'Inspect retained target evidence and captures' }).click();
    check(`${key}: exact subject discovery`, new URL(page.url()).searchParams.get('subject_id') === `reference.${spec.name}-adapter`);
    await page.locator(`a[href^="${route}?"]`).click();
    check(`${key}: exact record and capture choices`, new URL(page.url()).pathname === route && await page.locator('#evidence-image option').count() === spec.images);
    const pairs = [
      [`${spec.name}-base-${theme}-${width}-original-open`, `${spec.name}-base-${theme}-${width}-typed-open`],
      spec.name === 'menu'
        ? ['menu-baseline-zoom-light-enhanced-last-focus', 'menu-repair-zoom-light-enhanced-last-focus']
        : [`drawer-baseline-zoom-${theme}-typed-focus`, `drawer-repair-zoom-${theme}-typed-focus`],
    ];
    for (const [i, [first, second]] of pairs.entries()) {
      await page.locator('#evidence-image').focus();
      await page.keyboard.type(first);
      await page.keyboard.press('Tab');
      check(`${key}/${i}: comparison keyboard focus`, await page.locator('#evidence-compare').evaluate(e => e === document.activeElement));
      await page.keyboard.type(second);
      await page.keyboard.press('Tab');
      check(`${key}/${i}: submit keyboard focus`, await page.getByRole('button', { name: 'Show captures' }).evaluate(e => e === document.activeElement));
      await Promise.all([page.waitForURL(u => u.searchParams.get('image') === first && u.searchParams.get('compare') === second), page.keyboard.press('Enter')]);
      check(`${key}/${i}: theme retained`, new URL(page.url()).searchParams.get('theme') === theme);
      for (const image of await page.locator('figure img').all()) { await image.scrollIntoViewIfNeeded(); await image.evaluate(e => e.decode()); }
      const geometry = await page.locator('figure img').evaluateAll(images => images.map(e => { const r = e.getBoundingClientRect(); return { src: e.getAttribute('src'), x: r.x, width: r.width, height: r.height, top: r.top, bottom: r.bottom, naturalWidth: e.naturalWidth, naturalHeight: e.naturalHeight }; }));
      check(`${key}/${i}: two decoded images in selected order`, geometry.length === 2 && geometry[0].src.endsWith(first) && geometry[1].src.endsWith(second) && geometry.every(g => g.naturalWidth > 0));
      check(`${key}/${i}: unclipped proportional stack`, geometry[1].top >= geometry[0].bottom && geometry.every(g => g.x >= 0 && g.x + g.width <= width + 1 && Math.abs(g.width / g.height - g.naturalWidth / g.naturalHeight) < 0.01));
      check(`${key}/${i}: no horizontal document overflow`, await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
      if (i === 1) {
        const capture = path.join(out, `${key}.png`);
        await page.locator('#evidence-captures').scrollIntoViewIfNeeded();
        await page.screenshot({ path: capture });
        report.captures.push({ case: key, path: capture, sha256: sha(fs.readFileSync(capture)), geometry });
      }
    }
    await context.close();
  }
  check('no browser errors or external requests', report.errors.length === 0);
  for (const [p, pin] of Object.entries(inputs)) check(`input unchanged: ${p}`, sha(fs.readFileSync(p)) === pin);
  check('unexecuted binary provenance context unchanged', sha(fs.readFileSync(binaryPath)) === binarySHA256);
  fs.writeFileSync(path.join(out, 'receipt.json'), JSON.stringify(report, null, 2)+'\n', { mode: 0o600 });
  console.log(JSON.stringify({ out, checks: report.checks.length, images: report.images.length, captures: report.captures.length, errors: report.errors.length }));
} catch (err) {
  report.errors.push(err.stack);
  fs.writeFileSync(path.join(out, 'failure.json'), JSON.stringify(report, null, 2)+'\n', { mode: 0o600 });
  console.error(JSON.stringify({ out, checks: report.checks.length, images: report.images.length, error: err.message }));
  process.exitCode = 1;
} finally { if (browser) await browser.close(); }
