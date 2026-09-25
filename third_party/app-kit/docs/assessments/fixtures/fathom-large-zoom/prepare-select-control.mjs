// Retained control only. The conditional guide lane was not admitted by its result.
// node prepare-select-control.mjs ABS_ORIGINAL_CONTRACT EMPTY_ROOT
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import crypto from 'node:crypto';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const [oldFile, root] = process.argv.slice(2);
assert.equal(process.argv.length, 4);
assert.ok(path.isAbsolute(oldFile) && path.isAbsolute(root));
assert.deepEqual(await fs.readdir(root), [], 'staging root must be empty');
const digest = async p => {
  const h = crypto.createHash('sha256'); for await (const bytes of createReadStream(p)) h.update(bytes);
  return h.digest('hex');
};
const bytes = await fs.readFile(oldFile), old = JSON.parse(bytes);
assert.equal(old.inputs.node.path, process.execPath);
for (const [key, v] of Object.entries(old.inputs)) assert.equal(await digest(v.path), v.sha256, key + ' drift');
assert.equal(old.inputs.binary.sha256, '3fe1408f016a85535d3289dfd545f90772cb664a03af32b819e6b1e66629e7d8');
const sourceSHA = await digest(old.source_archive);
assert.equal(sourceSHA, '5431ea83d12d31c27574460afdf45203b923fd2f1eca6264e7e68a092ec115eb');
const selected = [];
for (const v of old.selected_active_source_equivalence) {
  const actual = await digest(path.join('/Volumes/m2ext/code/geoffmilleraz/sextant', v.file));
  assert.equal(actual, v.sha256, 'selected consumer drift: ' + v.file);
  selected.push({ file: v.file, sha256: actual });
}
const inputs = structuredClone(old.inputs);
await fs.mkdir(path.join(root, 'harness/fathom-large-zoom'), { recursive: true, mode: 0o700 });
await fs.mkdir(path.join(root, 'harness/fathom-guide-only-2026-09-22/browser'), { recursive: true, mode: 0o700 });
for (const [key, source, target] of [
  ['owner', old.inputs.owner.path, 'fathom-large-zoom/owner.mjs'],
  ['driver', fileURLToPath(new URL('./select-control-browser.mjs', import.meta.url)), 'fathom-large-zoom/browser.mjs'],
  ['zoom_helper', old.inputs.zoom_helper.path, 'fathom-large-zoom/zoom-contract.mjs'],
  ['helper', old.inputs.helper.path, 'fathom-guide-only-2026-09-22/browser/owned_process.mjs'],
]) {
  const dest = path.join(root, 'harness', target); await fs.copyFile(source, dest, 1);
  inputs[key] = { path: dest, sha256: await digest(dest) };
}
await fs.copyFile(fileURLToPath(import.meta.url), path.join(root, 'prepare-select-control.mjs'), 1);
await fs.writeFile(path.join(root, 'previous-contract.json'), bytes, { flag: 'wx' });
const contract = { schema_version: 1, prepared_at: new Date().toISOString(),
  scope: 'R4/R5 native select control; synthetic fixture, no consumer writes or guide acceptance',
  inputs, selected_consumer_files: selected, source_archive: { path: old.source_archive, sha256: sourceSHA },
  workload: { mode: 'native-select-control-v1', trials: ['Home/ArrowDown/Tab', 'Space/ArrowDown/Enter/Tab'],
    zoom_percent: 200, timeout_ms: 180000 } };
await fs.writeFile(path.join(root, 'contract.json'), JSON.stringify(contract, null, 2) + '\n', { flag: 'wx' });
console.log(JSON.stringify({ root, input_count: Object.keys(inputs).length, selected_consumer_matches: selected.length }));
