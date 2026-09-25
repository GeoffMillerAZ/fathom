// Separate prospective task; never change historical contracts or binary inputs.
// node prepare-dark-guide.mjs ABS_ORIGINAL_LARGE_CONTRACT EMPTY_PRIVATE_ROOT
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import crypto from 'node:crypto';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const [oldFile, root] = process.argv.slice(2);
assert.equal(process.argv.length, 4);
assert.ok(path.isAbsolute(oldFile) && path.isAbsolute(root));
assert.deepEqual(await fs.readdir(root), [], 'private staging root must be empty');
const digest = async file => {
  const h = crypto.createHash('sha256');
  for await (const bytes of createReadStream(file)) h.update(bytes);
  return h.digest('hex');
};
const previousBytes = await fs.readFile(oldFile), previous = JSON.parse(previousBytes);
assert.equal(previous.inputs.node.path, process.execPath);
for (const [key, input] of Object.entries(previous.inputs))
  assert.equal(await digest(input.path), input.sha256, 'historical input drift: ' + key);
assert.equal(previous.inputs.binary.sha256, '3fe1408f016a85535d3289dfd545f90772cb664a03af32b819e6b1e66629e7d8');
const archiveSHA = await digest(previous.source_archive);
assert.equal(archiveSHA, '5431ea83d12d31c27574460afdf45203b923fd2f1eca6264e7e68a092ec115eb');
const active = [];
for (const record of previous.selected_active_source_equivalence) {
  const actual = await digest(path.join('/Volumes/m2ext/code/geoffmilleraz/sextant', record.file));
  active.push({ file: record.file, expected: record.sha256, actual, matched: actual === record.sha256 });
  assert.equal(actual, record.sha256, 'selected consumer file drift: ' + record.file);
}
const inputs = structuredClone(previous.inputs), harness = path.join(root, 'harness/fathom-large-zoom');
await fs.mkdir(harness, { recursive: true, mode: 0o700 });
await fs.mkdir(path.join(root, 'harness/fathom-guide-only-2026-09-22/browser'), { recursive: true, mode: 0o700 });
for (const [key, source, relative] of [
  ['owner', previous.inputs.owner.path, 'fathom-large-zoom/owner.mjs'],
  ['driver', fileURLToPath(new URL('./dark-guide-browser.mjs', import.meta.url)), 'fathom-large-zoom/browser.mjs'],
  ['zoom_helper', previous.inputs.zoom_helper.path, 'fathom-large-zoom/zoom-contract.mjs'],
  ['helper', previous.inputs.helper.path, 'fathom-guide-only-2026-09-22/browser/owned_process.mjs'],
]) {
  const destination = path.join(root, 'harness', relative);
  await fs.copyFile(source, destination, 1);
  inputs[key] = { path: destination, sha256: await digest(destination) };
}
await fs.copyFile(fileURLToPath(import.meta.url), path.join(root, 'prepare-dark-guide.mjs'), 1);
await fs.writeFile(path.join(root, 'previous-contract.json'), previousBytes, { flag: 'wx' });
const contract = { schema_version: 1,
  scope: 'R4/R5 frozen synthetic dark/200 charts and theme-guide native recovery; no consumer writes',
  prepared_at: new Date().toISOString(), inputs,
  prior_contract_sha256: crypto.createHash('sha256').update(previousBytes).digest('hex'),
  source_archive: { path: previous.source_archive, sha256: archiveSHA },
  current_selected_consumer_source: active,
  workload: { mode: 'dark-200-guide-paint-v1', chart_cells: 1, schemes: ['dark'], zoom_percent: [200],
    guide: 'native dark source, invalid radius, repair to 1rem, preview, reset to default light source',
    captures: 7, capture_method: 'no-clip CDP compositor viewport', timeout_ms: 180000,
    appearance_acceptance: 'manual retained PNG review, not machine report.passed' } };
await fs.writeFile(path.join(root, 'contract.json'), JSON.stringify(contract, null, 2) + '\n', { flag: 'wx' });
console.log(JSON.stringify({ root, input_count: Object.keys(inputs).length, selected_consumer_matches: active.length }));
