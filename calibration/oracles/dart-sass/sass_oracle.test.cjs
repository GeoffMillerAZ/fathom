'use strict';
const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { test } = require('node:test');

test('definitions use inclusive parser spans and loop variable tokens', () => {
  const scratch = path.join(__dirname, '../../../workspace/tmp');
  fs.mkdirSync(scratch, { recursive: true });
  const root = fs.mkdtempSync(path.join(scratch, 'sass-oracle-'));
  try {
    const source = [
      '@mixin box {',
      '  $long_name:',
      '    red;',
      '  .first,',
      '  .second {',
      '    --tone:',
      '      blue',
      '}',
      '}',
      '@function shade() {',
      '  @return red;',
      '}',
      '@each $left_name, /* $fake */',
      '  $right_name in (red, blue) {',
      '  .item { color: $left_name; }',
      '}',
      '@for $index',
      '  from $index through 2 {',
      '  .n { color: red; }',
      '}',
      '@keyframes pulse {',
      '  from { opacity: 0; }',
      '  to { opacity: 1; }',
      '}',
      '.tail { color: red; }',
      '@each $café in (red, blue) {}',
      '@each $caf\\e9  in (red, blue) {}',
      '@for $niño from 1 through 2 {}',
      '',
    ].join('\r\n');
    fs.writeFileSync(path.join(root, 'case.scss'), source);
    fs.writeFileSync(path.join(root, 'files.txt'), 'case.scss\n');
    const result = JSON.parse(execFileSync(process.execPath, [path.join(__dirname, 'sass_oracle.cjs'), root, path.join(root, 'files.txt')], {
      env: process.env, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'],
    }));
    const defs = result.definitions;
    const span = (kind, name, line, end) => {
      const matches = defs.filter((d) => d.kind === kind && d.name === name);
      assert.equal(matches.length, 1, `${kind} ${name}: ${JSON.stringify(matches)}`);
      assert.deepEqual([matches[0].line, matches[0].end], [line, end], `${kind} ${name}`);
    };
    span('mixin', 'box', 1, 9);
    span('variable', 'long_name', 2, 3);
    span('class', 'first', 4, 4);
    span('class', 'second', 5, 5);
    span('custom_property', '--tone', 6, 7);
    span('function', 'shade', 10, 12);
    span('variable', 'left_name', 13, 13);
    span('variable', 'right_name', 14, 14);
    span('variable', 'index', 17, 17);
    span('keyframes', 'pulse', 21, 24);
    span('class', 'tail', 25, 25);
    span('variable', 'café', 26, 26);
    span('variable', 'caf\\e9', 27, 27);
    span('variable', 'niño', 28, 28);
    assert.equal(defs.some((d) => d.name === 'fake'), false);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
