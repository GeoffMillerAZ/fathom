// Explicit private source-closure test. Every mutation uses a disposable copy;
// the selected base and hardened projects remain read-only evidence inputs.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, unlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, isAbsolute, join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { verifyBaseReference, verifyHardenedReference } from './reference.mjs';

const hardened = process.env.APPKIT_CATALYST_HARDENED_ROOT;
const base = process.env.APPKIT_CATALYST_FONT_REFERENCE_ROOT;
assert.ok(hardened && isAbsolute(hardened), 'Set APPKIT_CATALYST_HARDENED_ROOT to an absolute prepared hardened project; missing evidence is not a skip.');
assert.ok(base && isAbsolute(base), 'Set APPKIT_CATALYST_FONT_REFERENCE_ROOT to an absolute prepared font-bound base; missing evidence is not a skip.');

const manifestPath = fileURLToPath(new URL('./source-manifest.json', import.meta.url));
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
const manifestBytes = readFileSync(manifestPath);
const manifestHash = 'f8de11343773dbb8dbeb53bb713d684fee23fcd36604ddff921b07da57450ee2';
assert.equal(digest(manifestBytes), manifestHash, 'Pin the manifest before deriving any scratch-copy paths.');
const manifest = JSON.parse(manifestBytes);
const expectedChanges = [
  'src/app/(app)/settings/address.tsx',
  'src/app/(app)/settings/page.tsx',
  'src/app/(app)/settings/settings-form.tsx',
  'src/app/(app)/settings/settings-state.ts',
  'src/components/listbox.tsx',
];

function fixture(t, source = hardened) {
  const root = mkdtempSync(join(tmpdir(), 'appkit-catalyst-hardened-canary-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const relative of manifest.roots) {
    const target = join(root, relative);
    mkdirSync(dirname(target), { recursive: true });
    cpSync(join(source, relative), target, { recursive: true, dereference: false, verbatimSymlinks: true });
  }
  return root;
}

test('base and hardened selected source closures match every pinned record', () => {
  const original = verifyBaseReference(base);
  const derivative = verifyHardenedReference(hardened);
  assert.equal(original.id, 'catalyst-font-bound-base');
  assert.equal(derivative.id, 'catalyst-settings-hardened-v3');
  assert.equal(original.source_manifest_sha256, manifestHash);
  assert.equal(derivative.source_manifest_sha256, manifestHash);
  assert.equal(original.files.length, 114);
  assert.equal(derivative.files.length, 116);
  assert.equal(original.files.length, manifest.original_files.length);
  assert.equal(derivative.files.length, manifest.files.length);
  assert.ok(original.total_bytes > 0);
  assert.ok(derivative.total_bytes > original.total_bytes);
});

test('manifest identifies exactly three replacements and two additions', () => {
  const original = new Map(manifest.original_files.map(record => [record.path, record]));
  const derivative = new Map(manifest.files.map(record => [record.path, record]));
  const allPaths = new Set([...original.keys(), ...derivative.keys()]);
  const differences = [...allPaths].filter(path => {
    const before = original.get(path), after = derivative.get(path);
    return !before || !after || before.size !== after.size || before.sha256 !== after.sha256;
  }).sort();
  assert.deepEqual(differences, expectedChanges);
  assert.deepEqual(manifest.changes.map(change => change.path).sort(), expectedChanges);
  assert.equal([...allPaths].filter(path => {
    const before = original.get(path), after = derivative.get(path);
    return before && after && before.size === after.size && before.sha256 === after.sha256;
  }).length, 111);
  for (const change of manifest.changes) {
    assert.equal(change.original_sha256, original.get(change.path)?.sha256 ?? null);
    assert.equal(change.sha256, derivative.get(change.path)?.sha256);
  }
  assert.match(manifest.patch_sha256, /^[0-9a-f]{64}$/);
});

test('changed, missing and extra selected files fail in isolated copies', async t => {
  const cases = [
    ['changed file', root => writeFileSync(join(root, expectedChanges[3]), `${readFileSync(join(root, expectedChanges[3]), 'utf8')}\n// synthetic change\n`), /Reviewed source mismatch/],
    ['missing file', root => unlinkSync(join(root, expectedChanges[2])), /Selected source closure is incomplete/],
    ['extra file', root => writeFileSync(join(root, 'src/app/(app)/settings/unreviewed.tsx'), '// synthetic extra\n'), /Unexpected selected source file/],
  ];
  for (const [name, mutate, error] of cases) {
    await t.test(name, t => {
      const root = fixture(t);
      mutate(root);
      assert.throws(() => verifyHardenedReference(root), error);
    });
  }
});

test('selected file and directory symlinks fail in isolated copies', async t => {
  await t.test('file symlink', t => {
    const root = fixture(t), target = join(root, expectedChanges[3]);
    unlinkSync(target);
    symlinkSync('page.tsx', target);
    assert.throws(() => verifyHardenedReference(root), /Symlink refused/);
  });
  await t.test('directory symlink', t => {
    const root = fixture(t), target = join(root, 'src/components');
    rmSync(target, { recursive: true });
    // The target and link are both confined to this test's disposable tree.
    symlinkSync('app', target, 'dir');
    assert.throws(() => verifyHardenedReference(root), /Symlink refused/);
  });
});

test('private root selection fails loudly rather than skipping', () => {
  const result = spawnSync(process.execPath, ['--test', fileURLToPath(import.meta.url)], {
    encoding: 'utf8',
    timeout: 5000,
    env: { NEXT_TELEMETRY_DISABLED: '1' },
  });
  assert.equal(result.status, 1, result.stderr);
  assert.match(result.stdout + result.stderr, /Set APPKIT_CATALYST_HARDENED_ROOT/);
  assert.match(result.stdout + result.stderr, /skipped 0/);
});

test('mutation canaries leave both selected evidence roots valid', () => {
  assert.equal(verifyBaseReference(base).files.length, 114);
  assert.equal(verifyHardenedReference(hardened).files.length, 116);
});
