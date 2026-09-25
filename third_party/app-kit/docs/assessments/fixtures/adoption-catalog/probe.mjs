// Actual-process metadata discovery and explicit synthetic adoption walkthrough.
// No real consumer, installed binary, model or network is used.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

const [baseline, candidate, output] = process.argv.slice(2);
assert([baseline, candidate, output].every(p => p && path.isAbsolute(p)));
fs.mkdirSync(output, { mode: 0o700 }); // Refuse reuse of an earlier run.
const hash = body => crypto.createHash('sha256').update(body).digest('hex');
const write = (name, body) => fs.writeFileSync(path.join(output, name), body, { flag: 'wx', mode: 0o600 });
write('probe.mjs', fs.readFileSync(new URL(import.meta.url)));
const binaries = [baseline, candidate].map(file => ({ file, sha256: hash(fs.readFileSync(file)) }));
write('inputs.json', JSON.stringify({ binaries, node: process.version }, null, 2));
const root = path.join(output, 'synthetic-consumer');
fs.mkdirSync(root, { mode: 0o700 });
const files = {
  'theme.css': '/* Synthetic app-owned theme bytes. */\n',
  'app-kit.md': '# Synthetic task\nReview metadata, selected bytes and contribution guidance.\n',
  'appkit.adoption.json': 'invalid manifest; discovery and draft must not require or overwrite it\n',
};
for (const [name, body] of Object.entries(files)) fs.writeFileSync(path.join(root, name), body);
const calls = [];
function unchangedFixture() {
  assert.deepEqual(fs.readdirSync(root).sort(), Object.keys(files).sort());
  for (const [name, body] of Object.entries(files)) assert.equal(fs.readFileSync(path.join(root, name), 'utf8'), body);
}
function run(binary, args, expected = 0) {
  const r = spawnSync(binary, ['adoption', ...args], {
    cwd: root, env: { PATH: '/usr/bin:/bin', TMPDIR: output, NO_COLOR: '1' },
    encoding: 'utf8', timeout: 10000, maxBuffer: 256 * 1024,
  });
  const record = { binary, args, expected, status: r.status, signal: r.signal, error: r.error ? String(r.error) : null, stdout: r.stdout, stderr: r.stderr };
  calls.push(record); write(`call-${calls.length}.json`, JSON.stringify(record, null, 2));
  assert(!r.error, String(r.error)); assert.equal(r.signal, null); assert.equal(r.status, expected, r.stderr);
  assert(Buffer.byteLength(r.stdout) <= 128 * 1024);
  return r;
}
function json(args) {
  const r = run(candidate, args);
  assert.equal(r.stderr, '');
  return JSON.parse(r.stdout);
}
const result = {};
try {
  for (const args of [['search'], ['capability', '--id', 'foundation.tokens']]) assert.equal(run(baseline, args, 2).stdout, '');
  const searchArgs = ['search', '--query', 'tokens', '--profile', 'portable', '--layer', 'L0', '--limit', '1'];
  const first = run(candidate, searchArgs);
  assert.equal(run(candidate, searchArgs).stdout, first.stdout);
  const page = JSON.parse(first.stdout);
  assert.equal(page.items[0].id, 'foundation.tokens');
  const id = page.items[0].id;
  const detail = json(['capability', '--id', id, '--expected-catalog-revision', page.revision]);
  assert.equal(detail.revision, page.revision); assert.equal(detail.entry.id, id);
  assert.equal(detail.capability_revision.id, id); assert.match(detail.capability_revision.sha256, /^[0-9a-f]{64}$/);
  assert(detail.entry.usage && Array.isArray(detail.entry.gaps) && Array.isArray(detail.entry.app_owns));
  const empty = json(['search', '--profile', 'cli', '--layer', 'L3a']);
  assert.deepEqual(empty.items, []); assert.equal(empty.total, 0); assert.equal(empty.next_cursor, '');
  const paged = json(['search', '--limit', '1']);
  assert(paged.next_cursor);
  const next = json(['search', '--limit', '1', '--cursor', paged.next_cursor]);
  assert.notEqual(next.items[0].id, paged.items[0].id); assert.equal(next.revision, paged.revision);
  for (const args of [
    ['search', '--limit', '2', '--cursor', paged.next_cursor],
    ['search', '--profile', 'unavailable'], ['search', '--json'],
    ['capability', '--id', 'missing'],
    ['capability', '--id', id, '--expected-catalog-revision', '0'.repeat(64)],
    ['capability', '--id', id, '--expected-catalog-revision', 'A'.repeat(64)],
  ]) assert.equal(run(candidate, args, 2).stdout, '');
  const help = run(candidate, ['capability', '--help']);
  assert.equal(help.stdout, ''); assert(help.stderr.includes('not adoption proof'));
  unchangedFixture();
  const draft = run(candidate, ['draft', '--root', root, '--consumer', 'synthetic-catalog', '--profile', 'portable',
    '--contract', 'app-kit.md', '--layer', 'L0', '--capability', id, '--artifact', 'theme.css', '--expected-catalog-revision', page.revision]).stdout;
  unchangedFixture();
  const selection = JSON.parse(draft);
  assert.deepEqual(selection.capability_revisions, [detail.capability_revision]);
  assert.equal(selection.artifacts[0].sha256, hash(files['theme.css']));
  // Deliberate fixture-only saving, never an appkit command side effect.
  fs.writeFileSync(path.join(root, 'appkit.adoption.json'), draft);
  for (const mode of ['inspect', 'pins', 'check']) {
    const args = [mode, '--root', root, '--json'];
    assert.equal(run(candidate, args).stdout, run(baseline, args).stdout, `${mode} existing output drifted`);
  }
  assert.equal(json(['check', '--root', root, '--json']).outcome, 'passed');
  fs.appendFileSync(path.join(root, 'theme.css'), '/* intentional synthetic drift */\n');
  assert.equal(JSON.parse(run(candidate, ['check', '--root', root, '--json'], 1).stdout).outcome, 'failed');
  fs.writeFileSync(path.join(root, 'theme.css'), files['theme.css']);
  assert.equal(json(['check', '--root', root, '--json']).outcome, 'passed');
  for (const binary of binaries) assert.equal(hash(fs.readFileSync(binary.file)), binary.sha256);
  result.pass = true;
} catch (error) { result.pass = false; result.error = String(error); }
result.calls = calls.length;
write('result.json', JSON.stringify(result, null, 2));
console.log(JSON.stringify(result));
process.exitCode = result.pass ? 0 : 1;
