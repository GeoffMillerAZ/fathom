import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

test('inspection refuses invalid authority before browser or evidence access', t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'navbar-inspection-refusal-'));
  const publicParent = path.join(root, 'public'), symlink = path.join(root, 'link');
  fs.mkdirSync(publicParent, { mode: 0o755 }); fs.chmodSync(publicParent, 0o755);
  fs.symlinkSync(publicParent, symlink);
  t.after(() => fs.rmSync(root, { recursive: true }));
  const probe = fileURLToPath(new URL('check-live.mjs', import.meta.url));
  const base = ['http://127.0.0.1:1', root, 'a'.repeat(64), root];
  for (const args of [
    ['https://example.invalid', ...base.slice(1)],
    [base[0], root, 'unreviewed', root],
    [base[0], root, base[2], publicParent],
    [base[0], root, base[2], symlink],
  ]) {
    const child = spawnSync(process.execPath, [probe, ...args], { encoding: 'utf8', timeout: 5000 });
    assert.equal(child.status, 1);
    assert.match(child.stderr, /AssertionError/);
    assert(!child.stdout.includes('Private navbar inspection:'));
    assert.deepEqual(fs.readdirSync(root).sort(), ['link', 'public']);
  }
});
