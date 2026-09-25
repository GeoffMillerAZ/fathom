import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const harness = fileURLToPath(new URL('./check-live.mjs', import.meta.url));

test('refuses non-private and symlinked output parents before creating output', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'overlay-parent-test-'));
  try {
    const real = path.join(root, 'real');
    const permissive = path.join(root, 'permissive');
    const linked = path.join(root, 'linked');
    fs.mkdirSync(real, { mode: 0o700 });
    fs.mkdirSync(permissive, { mode: 0o755 });
    fs.chmodSync(real, 0o700);
    fs.chmodSync(permissive, 0o755);
    fs.symlinkSync(real, linked);
    for (const parent of [permissive, linked]) {
      const result = spawnSync(process.execPath, [harness], {
        encoding: 'utf8',
        env: { ...process.env, APPKIT_OVERLAY_URL: 'http://127.0.0.1:9', APPKIT_OVERLAY_EVIDENCE_ROOT: root, APPKIT_OVERLAY_OUTPUT_PARENT: parent },
      });
      assert.notEqual(result.status, 0, `accepted unsafe parent ${parent}`);
      assert.deepEqual(fs.readdirSync(real), [], 'created output through a symlink');
      assert.deepEqual(fs.readdirSync(permissive), [], 'created output in a permissive directory');
    }
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
