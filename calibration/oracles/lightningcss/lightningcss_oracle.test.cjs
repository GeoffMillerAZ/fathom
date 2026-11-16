'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const test = require('node:test');
const lightningcss = require('lightningcss');

function run(files, extraEnv = {}, refusal = false) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'lightningcss-nesting-'));
  try {
    for (const [name, source] of Object.entries(files)) fs.writeFileSync(path.join(root, name), source);
    const child = spawnSync(process.execPath, [path.join(__dirname, 'lightningcss_oracle.cjs'), root], {
      encoding: 'utf8', timeout: 10000, maxBuffer: 8 * 1024 * 1024,
      env: { ...process.env, ...extraEnv },
    });
    assert.ifError(child.error);
    if (refusal) {
      assert.equal(child.status, 1, child.stderr);
      assert.equal(child.stdout, '');
      assert.match(child.stderr, /withdrawn nesting depth exceeds 64/);
      return;
    }
    assert.equal(child.status, 0, child.stderr);
    return { truth: JSON.parse(child.stdout), stats: JSON.parse(child.stderr.trim()), raw: child.stdout };
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
}
const def = (name, line, kind = 'selector', file = 'case.css') => ({ path: file, name, line, end: 0, kind });
const call = (from, to) => ({ path: 'case.css', from, to });

test('withdrawn nesting enumerates each selector and custom declaration with the narrowest owner', () => {
  const source = [
    '@import "./base.css";',
    '.outer {',
    '  --parent: var(--base);',
    '  color: var(--outer);',
    '  @nest .child &,',
    '    .sibling & {',
    '    --local: var(--base, var(--fallback)) !important;',
    '    color: var(--tone);',
    '    animation: pulse 1s;',
    '    @media (width > 1px) {',
    '      @nest .grand & { color: var(--deep); }',
    '    }',
    '  }',
    '  & .modern { color: var(--modern); }',
    '}',
    '@keyframes pulse { to { opacity: 1 } }',
  ].join('\n');
  // Native parser owns the population; these assertions do not read an extractor or corpus.
  let sheet;
  lightningcss.transform({ code: Buffer.from(source), errorRecovery: true,
    visitor: { StyleSheet(value) { sheet = value; } } });
  const native = sheet.rules.find(r => r.type === 'style').value.rules[0];
  assert.equal(native.type, 'nesting');
  assert.equal(native.value.style.selectors.length, 2);
  const { truth, stats } = run({ 'case.css': source, 'base.css': ':root { --base: 1; }' });
  assert.equal(truth.tool, 'lightningcss');
  assert.equal(truth.version, '1.33.0');
  assert.equal(truth.tier, 'A'); // Historical W2 label retained; no tier ruling here.
  assert.deepEqual(truth.definitions, [
    def('--base', 1, 'custom_property', 'base.css'), def(':root', 1, 'selector', 'base.css'),
    def('.outer', 2), def('--parent', 3, 'custom_property'),
    def('.child &', 5), def('.sibling &', 6), def('--local', 7, 'custom_property'),
    def('.grand &', 11), def('& .modern', 14), def('pulse', 16, 'keyframes'),
  ]);
  assert.deepEqual(truth.calls, [
    call('& .modern', '--modern'), call('--local', '--base'), call('--local', '--fallback'),
    call('--parent', '--base'), call('.child &', '--tone'), call('.child &', 'pulse'),
    call('.grand &', '--deep'), call('.outer', '--outer'),
  ]);
  assert.deepEqual(truth.imports, [{ path: 'case.css', line: 1, to_path: 'base.css', module: './base.css' }]);
  assert.equal(stats.files, 2);
  assert.equal(stats.custom_located, 3);
  assert.equal(stats.custom_unlocated, 0);
  assert.equal(stats.selector_cut_mismatch, 0);
  assert.deepEqual(stats.parse_failures, []);
});

test('nesting prelude excludes the native at-keyword including case, escapes, comments and CRLF', () => {
  for (const keyword of ['@nest', '@NeSt', '@n\\65 st', '@nest/* { , } */']) {
    const { truth, stats } = run({ 'case.css': [
      '.outer {', `  ${keyword}`, '  .é[data-x="{,}" ] &,', '  .other & {',
      '    --local: var(--base);', '    color: var(--tone);', '  }', '}',
    ].join('\r\n') });
    assert.deepEqual(truth.definitions, [def('.outer', 1), def('.é[data-x="{,}" ] &', 3),
      def('.other &', 4), def('--local', 5, 'custom_property')], keyword);
    assert.deepEqual(truth.calls, [call('--local', '--base'), call('.é[data-x="{,}" ] &', '--tone')], keyword);
    assert.equal(stats.selector_cut_mismatch, 0, keyword);
    assert.equal(stats.custom_unlocated, 0, keyword);
  }
});

test('malformed nesting never manufactures rejected definitions or parent references', () => {
  const { truth, stats } = run({ 'case.css': '.outer { @nest { color: var(--rejected); } }' });
  assert.deepEqual(truth.definitions, [def('.outer', 1)]);
  assert.deepEqual(truth.calls, []);
  assert.equal(stats.warnings, 2);
});

test('raw truth ignores perturbed and removed extractor artifacts; input facts change it', () => {
  const files = { 'case.css': '.outer { @nest .child & { color: var(--tone); } }' };
  const baseline = run(files);
  assert.equal(run({ ...files, 'extractor.json': JSON.stringify([{ name: 'wrong', line: 999 }]) }).raw, baseline.raw);
  assert.equal(run({ ...files, 'extractor.json': '[]' }).raw, baseline.raw);
  const changed = run({ 'case.css': files['case.css'].replace('--tone', '--changed') });
  assert.notEqual(changed.raw, baseline.raw);
  assert.deepEqual(changed.truth.calls, [call('.child &', '--changed')]);
});


test('withdrawn nesting accepts depth 64 and refuses the whole output at 65', () => {
  const nested = n => '.outer {' + '@nest .child & {'.repeat(n) + 'color: var(--tone);' + '}'.repeat(n + 1);
  const { truth } = run({ 'case.css': nested(64) });
  assert.equal(truth.definitions.length, 65);
  assert.deepEqual(truth.calls, [call('.child &', '--tone')]);
  run({ 'case.css': nested(65), 'a.css': '.before { --x: 1; }' }, {}, true);
});
