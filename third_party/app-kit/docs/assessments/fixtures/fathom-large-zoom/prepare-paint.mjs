// Prepare a separate one-cell contract; never repin or edit a historical run.
// node prepare-paint.mjs ABS_OLD_CONTRACT NEW_PRIVATE_ROOT
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
const previousBytes = await fs.readFile(oldFile);
const previous = JSON.parse(previousBytes);
assert.equal(previous.inputs.node.path, process.execPath);
for (const [key, input] of Object.entries(previous.inputs))
  assert.equal(await digest(input.path), input.sha256, 'historical input drift: ' + key);
assert.equal(previous.inputs.binary.sha256, '3fe1408f016a85535d3289dfd545f90772cb664a03af32b819e6b1e66629e7d8');
const archiveSHA = await digest(previous.source_archive);
assert.equal(archiveSHA, '5431ea83d12d31c27574460afdf45203b923fd2f1eca6264e7e68a092ec115eb');
const inputs = structuredClone(previous.inputs);
const harness = path.join(root, 'harness/fathom-large-zoom');
await fs.mkdir(harness, { recursive: true, mode: 0o700 });
await fs.mkdir(path.join(root, 'harness/fathom-guide-only-2026-09-22/browser'), { recursive: true, mode: 0o700 });
const sources = {
  owner: [previous.inputs.owner.path, path.join(harness, 'owner.mjs')],
  driver: [fileURLToPath(new URL('./paint-browser.mjs', import.meta.url)), path.join(harness, 'browser.mjs')],
  zoom_helper: [previous.inputs.zoom_helper.path, path.join(harness, 'zoom-contract.mjs')],
  helper: [previous.inputs.helper.path, path.join(root, 'harness/fathom-guide-only-2026-09-22/browser/owned_process.mjs')],
};
for (const [key, [source, destination]] of Object.entries(sources)) {
  await fs.copyFile(source, destination, 1);
  inputs[key] = { path: destination, sha256: await digest(destination) };
}
await fs.copyFile(fileURLToPath(import.meta.url), path.join(root, 'prepare-paint.mjs'), 1);
await fs.writeFile(path.join(root, 'previous-contract.json'), previousBytes, { flag: 'wx' });
const active = [];
for (const record of previous.selected_active_source_equivalence) {
  try {
    const actual = await digest(path.join('/Volumes/m2ext/code/geoffmilleraz/sextant', record.file));
    active.push({ file: record.file, expected: record.sha256, actual, matched: actual === record.sha256 });
  } catch (error) { active.push({ file: record.file, matched: false, error: String(error) }); }
}
const contract = { schema_version: 1,
  scope: 'R4/R5 one historical synthetic light/200 call chart; capture diagnosis only, no active consumer writes',
  prepared_at: new Date().toISOString(), inputs,
  prior_contract_sha256: crypto.createHash('sha256').update(previousBytes).digest('hex'),
  source_archive: { path: previous.source_archive, sha256: archiveSHA },
  current_selected_consumer_source: active,
  workload: { mode: 'one-light-200-paint-v1', chart_cells: 1, schemes: ['light'], zoom_percent: [200],
    captures: ['cdp-before', 'playwright-control', 'cdp-after'], timeout_ms: 180000,
    appearance_acceptance: 'manual retained PNG review, not machine report.passed' } };
await fs.writeFile(path.join(root, 'contract.json'), JSON.stringify(contract, null, 2) + '\n', { flag: 'wx' });
console.log(JSON.stringify({ root, input_count: Object.keys(inputs).length,
  current_consumer_matches: active.filter(a => a.matched).length, current_consumer_selected: active.length }));
