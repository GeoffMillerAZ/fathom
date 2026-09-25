import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp, mkdir, writeFile, symlink, rm, realpath} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath} from 'node:url';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';

const exec = promisify(execFile);
const runner = fileURLToPath(new URL('./run.mjs', import.meta.url));
async function refused(args, message) {
  try {
    await exec(process.execPath, [runner, ...args], {timeout: 5000});
    assert.fail('Runner unexpectedly accepted a refused input');
  } catch (error) {
    assert.equal(error.code, 1);
    assert.match(error.stderr, message);
    assert.equal(error.stdout, '', 'Refusal must happen before allocating evidence or invoking Go');
  }
}

for (const [name, args] of [
  ['missing arguments', []],
  ['unknown mode', ['/consumer', '/playwright', '--anything', '/candidate']],
  ['relative candidate', ['/consumer', '/playwright', '--reader-candidate', 'candidate.go']],
  ['relative display candidate', ['/consumer', '/playwright', '--display-candidate', 'candidate']],
  ['relative display baseline', ['/consumer', '/playwright', '--display-baseline', 'candidate']],
  ['extra arguments', ['/consumer', '/playwright', '--reader-candidate', '/candidate', 'extra']],
]) test(name, () => refused(args, /Expected consumer, Playwright/));

async function fixture(t) {
  const root = await realpath(await mkdtemp(join(tmpdir(), 'appkit-handler-refusal-')));
  t.after(() => rm(root, {recursive: true, force: true}));
  await mkdir(join(root, 'cmd/fathom'), {recursive: true});
  await writeFile(join(root, 'go.mod'), 'module github.com/geoffmilleraz/fathom\n');
  const module = join(root, 'playwright.mjs');
  await writeFile(module, 'throw Error("Never import this refusal fixture");\n');
  return {root, module, candidate: join(root, 'candidate.go')};
}

test('tampered reader bytes are refused before compilation', async t => {
  const f = await fixture(t);
  await writeFile(f.candidate, 'package main\n');
  await refused([f.root, f.module, '--reader-candidate', f.candidate], /candidate identity mismatch/);
});

test('symlink candidate is refused', async t => {
  const f = await fixture(t);
  await symlink(f.module, f.candidate);
  await refused([f.root, f.module, '--reader-candidate', f.candidate], /bounded regular source file/);
});

test('oversized candidate is refused', async t => {
  const f = await fixture(t);
  await writeFile(f.candidate, 'x'.repeat(1024 * 1024 + 1));
  await refused([f.root, f.module, '--reader-candidate', f.candidate], /bounded regular source file/);
});

test('an existing consumer probe is never shadowed', async t => {
  const f = await fixture(t);
  await writeFile(join(f.root, 'cmd/fathom/appkit_actual_handler_probe_test.go'), 'package main\n');
  await refused([f.root, f.module], /refusing to shadow consumer work/);
});

test('display directory symlink is refused before evidence allocation', async t => {
  const f = await fixture(t);
  await symlink(f.root, f.candidate);
  await refused([f.root, f.module, '--display-candidate', f.candidate], /real absolute directory/);
});

test('tampered display source is refused before compilation', async t => {
  const f = await fixture(t);
  await mkdir(join(f.candidate, 'cmd/fathom'), {recursive: true});
  await writeFile(join(f.candidate, 'cmd/fathom/observatory_render.go'), 'package main\n');
  await refused([f.root, f.module, '--display-candidate', f.candidate], /Display candidate identity mismatch/);
});

test('display source symlink is refused before compilation', async t => {
  const f = await fixture(t);
  await mkdir(join(f.candidate, 'cmd/fathom'), {recursive: true});
  await symlink(f.module, join(f.candidate, 'cmd/fathom/observatory_render.go'));
  await refused([f.root, f.module, '--display-candidate', f.candidate], /bounded regular files without symlinks/);
});
