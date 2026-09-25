// Read-only retained PNG/HTTP identity check against an independently named bundle.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';

const [base, root, outputParent] = process.argv.slice(2);
const origin = new URL(base);
assert.equal(origin.hostname, '127.0.0.1');
assert.equal(origin.protocol, 'http:');
const expected = '994e15283cbaa57570986429816bd42fadf91455531985c69c4ca48b7c7bbe1e';
const id = `evidence@${expected}`;
const file = path.join(root, 'evidence', 'entries', `${expected}.json`);
const bytes = fs.readFileSync(file);
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
assert.equal(sha(bytes), expected);
const bundle = JSON.parse(bytes);
const slots = [...new Map(Object.values(bundle.receipt.results).flatMap(r => r.evidence).filter(e => e.role === 'image').map(e => [e.id, e])).values()];
assert.equal(slots.length, 24);
const out = fs.mkdtempSync(path.join(outputParent, 'http-'));
const report = { verification: 'not-verified', record: id, bundle_sha256: expected, checker_sha256: sha(fs.readFileSync(fileURLToPath(import.meta.url))), images: [], errors: [] };
try {
  for (const slot of slots) {
    const original = Buffer.from(bundle.content[slot.id], 'base64');
    assert.equal(sha(original), slot.sha256);
    const response = await fetch(new URL(`/target-evidence/${id}/images/${slot.id}`, origin));
    assert.equal(response.status, 200);
    assert.equal(response.headers.get('content-type'), 'image/png');
    assert.equal(response.headers.get('cache-control'), 'no-store');
    assert.equal(response.headers.get('x-appkit-verification'), 'not-verified');
    assert.equal(response.headers.get('x-appkit-evidence-sha256'), slot.sha256);
    const served = Buffer.from(await response.arrayBuffer());
    assert.deepEqual(served, original);
    assert.equal(served.length, slot.bytes);
    report.images.push({ id: slot.id, sha256: sha(served), bytes: served.length });
  }
  assert.equal(sha(fs.readFileSync(file)), expected);
  fs.writeFileSync(path.join(out, 'receipt.json'), JSON.stringify(report, null, 2)+'\n', { mode: 0o600 });
  console.log(JSON.stringify({ out, images: report.images.length, errors: report.errors.length }));
} catch (err) {
  report.errors.push(err.stack);
  fs.writeFileSync(path.join(out, 'failure.json'), JSON.stringify(report, null, 2)+'\n', { mode: 0o600 });
  throw err;
}
