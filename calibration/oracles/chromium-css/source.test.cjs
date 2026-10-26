'use strict';

const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const { snapshot, requireSameSource, readBoundPrimary, sidecarFile, writeBoundPrimary } = require('./source.cjs');
const { attestFile } = require('./ranges.cjs');
const { lineStarts, point, scanBlocks } = require('./blocks.cjs');

function git(root, ...args) {
  execFileSync('git', ['-C', root, ...args], { stdio: 'pipe' });
}

test('same-name same-line CSS edit, changed file list and commit all invalidate primary snapshot', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'css-source-test-'));
  try {
    git(root, 'init', '-q');
    git(root, 'config', 'user.name', 'Test');
    git(root, 'config', 'user.email', 'test@example.invalid');
    fs.mkdirSync(path.join(root, 'css'));
    fs.writeFileSync(path.join(root, 'css', 'a.css'), '.same { color: red; }\n');
    git(root, 'add', '.');
    git(root, 'commit', '-qm', 'first');
    const before = snapshot(root, ['css']);
    requireSameSource(before, root, ['css']);
    fs.writeFileSync(path.join(root, 'css', 'a.css'), '.same { color: blue; }\n');
    assert.throws(() => requireSameSource(before, root, ['css']), /clean|source manifest/i);
    git(root, 'add', '.');
    git(root, 'commit', '-qm', 'same line and name');
    assert.throws(() => requireSameSource(before, root, ['css']), /source manifest/i);
    const changed = snapshot(root, ['css']);
    fs.writeFileSync(path.join(root, 'css', 'b.css'), '.new {}\n');
    git(root, 'add', '.');
    git(root, 'commit', '-qm', 'file list');
    assert.throws(() => requireSameSource(changed, root, ['css']), /source manifest/i);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test('native properties exclude disabled, implicit and parse-failed copies without consuming duplicate definitions', () => {
  const source = '.x { --v: a; --v: b; }';
  const block = scanBlocks(source)[0];
  const starts = lineStarts(source);
  const propertyRange = index => {
    const start = source.indexOf('--v', index);
    const end = source.indexOf(';', start) + 1;
    const a = point(starts, start), b = point(starts, end);
    return { startLine: a.line, startColumn: a.column, endLine: b.line, endColumn: b.column };
  };
  const first = propertyRange(0), second = propertyRange(source.indexOf(';') + 1);
  const property = (range, extra = {}) => ({ name: '--v', range, ...extra });
  const primary = [
    { kind: 'selector', name: '.x', line: 1 },
    { kind: 'custom_property', name: '--v', line: 1 },
    { kind: 'custom_property', name: '--v', line: 1 },
  ];
  const stats = { attested: { selector: 0, custom_property: 0 } };
  const ranges = attestFile(source, 'a.css', primary, [{ block, style: { cssProperties: [
    property(first, { disabled: true }), property(first, { implicit: true }),
    property(first, { parsedOk: false }), property(first), property(second), property(second),
  ] } }], stats);
  assert.deepEqual(ranges.map(r => [r.kind, r.name, r.line, r.end]), [
    ['selector', '.x', 1, 1], ['custom_property', '--v', 1, 1], ['custom_property', '--v', 1, 1],
  ]);
});

test('native property end uses last occupied UTF-16 source position across lines', () => {
  const source = '/* 🧪 */ .x {\r\n --v: 🧪\r\n  blue;\r\n}\r\n';
  const block = scanBlocks(source)[0];
  const starts = lineStarts(source);
  const a = point(starts, source.indexOf('--v'));
  const b = point(starts, source.indexOf(';') + 1);
  const stats = { attested: { selector: 0, custom_property: 0 } };
  const ranges = attestFile(source, 'a.css', [{ kind: 'custom_property', name: '--v', line: 2 }], [
    { block, style: { cssProperties: [{ name: '--v', range: {
      startLine: a.line, startColumn: a.column, endLine: b.line, endColumn: b.column,
    } }] } },
  ], stats);
  assert.deepEqual(ranges.map(r => [r.line, r.end]), [[2, 3]]);
});

test('primary remains neutral and sidecar rejects mismatched primary bytes before JSON parsing', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'css-primary-source-'));
  const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'css-primary-output-'));
  try {
    git(root, 'init', '-q');
    git(root, 'config', 'user.name', 'Test');
    git(root, 'config', 'user.email', 'test@example.invalid');
    fs.writeFileSync(path.join(root, 'a.css'), '.same {}\n');
    git(root, 'add', '.');
    git(root, 'commit', '-qm', 'source');
    const manifest = snapshot(root);
    const file = path.join(outputDir, 'primary.json');
    const neutral = { tool: 'lightningcss', tier: 'A', version: 'test', definitions: [], calls: [], imports: [] };
    writeBoundPrimary(file, neutral, manifest);
    assert.deepEqual(JSON.parse(fs.readFileSync(file, 'utf8')), neutral);
    assert.equal(Object.hasOwn(JSON.parse(fs.readFileSync(file, 'utf8')), 'source_manifest'), false);
    assert.deepEqual(readBoundPrimary(file, root).oracle, neutral);
    assert.ok(fs.existsSync(sidecarFile(file)));
    assert.throws(() => writeBoundPrimary(file, neutral, manifest), /already exists/);
    fs.writeFileSync(file, '{ invalid json');
    assert.throws(() => readBoundPrimary(file, root), /bytes differ/);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
    fs.rmSync(outputDir, { recursive: true, force: true });
  }
});
