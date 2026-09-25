// Run a caller-built appkit binary against a verified private source checkout.
// Only new, task-owned temporary evidence paths are written. No installation.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import * as fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

const [kitArg, probeArg] = process.argv.slice(2);
assert.equal(process.argv.length, 4);
assert(path.isAbsolute(kitArg) && path.isAbsolute(probeArg));
const kit = await fs.realpath(kitArg), probe = await fs.realpath(probeArg);
assert(probe.startsWith(await fs.realpath(os.tmpdir()) + path.sep));
const binary = path.join(probe, 'appkit'), root = path.join(kit, 'data/design-assets');
const sha = b => createHash('sha256').update(b).digest('hex');
const hash = async p => sha(await fs.readFile(p));
const before = await hash(path.join(root, 'catalog.json'));
const commands = [];
function command(name, args, expected, silent = true) {
  const r = spawnSync(binary, ['data', 'assets-adapt-hero', '-root', root, ...args], { timeout: 30000, maxBuffer: 2 ** 20 });
  commands.push({ name, expected, exit: r.status, signal: r.signal, stdout_sha256: sha(r.stdout ?? ''), stderr_sha256: sha(r.stderr ?? '') });
  assert.equal(r.status, expected, name); assert.equal(r.signal, null, name);
  if (silent) assert.equal(r.stdout.length, 0, 'unexpected stdout');
  return r;
}
const generated = command('default-private-json', [], 0, false);
const a = JSON.parse(generated.stdout);
assert.equal(a.adapter, 'p01-hero-v1'); assert.equal(a.verification, 'not-verified');
command('compiler-delivery-current', ['-check', path.join(probe, 'delivery')], 0);
const delivery = path.join(probe, 'cli-delivery');
command('export', ['-out', delivery], 0);
command('check', ['-check', delivery], 0);
const names = ['hero.templ', 'hero.go', 'hero.css', 'defaults.json', 'USAGE.md', 'artifact.json'];
assert.deepEqual((await fs.readdir(delivery)).sort(), [...names].sort());
for (const name of names) assert.equal(await hash(path.join(delivery, name)), await hash(path.join(probe, 'delivery', name)), 'CLI and compiler delivery differ');
command('no-clobber', ['-out', delivery], 2);
command('missing-delivery', ['-check', path.join(probe, 'missing-delivery')], 1);
command('invalid-mode', ['-out', path.join(probe, 'must-not-exist'), '-check', delivery], 2);
await assert.rejects(fs.stat(path.join(probe, 'must-not-exist')), { code: 'ENOENT' });
await fs.appendFile(path.join(delivery, 'hero.css'), '\n/* intentional private drift canary */\n');
command('observed-drift', ['-check', delivery], 1);
assert.equal(await hash(path.join(root, 'catalog.json')), before);
const receipt = { kind: 'private-p01-real-cli', verification: 'not-certified', binary_sha256: await hash(binary), catalog_before_after_sha256: before, artifact_sha256: sha(generated.stdout), commands,
  limits: ['cli-delivery deliberately retains a drift canary; use the separate pristine delivery', 'no real consumer, browser, release or target certification'] };
const bytes = JSON.stringify(receipt, null, 2) + '\n';
await fs.writeFile(path.join(probe, 'cli-receipt.json'), bytes, { mode: 0o600, flag: 'wx' });
console.log(JSON.stringify({ commands: commands.length, receipt_sha256: sha(bytes) }));
