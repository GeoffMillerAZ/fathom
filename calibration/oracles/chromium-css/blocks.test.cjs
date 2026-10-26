'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const { scanBlocks } = require('./blocks.cjs');

test('scanner proposes balanced nested blocks with lexical headers and selector pieces', () => {
  const src = '/* { } */ .a:is(.b, .c),\n .d[title="x,y"] {\n color:red;\n @media not all {\n  .child, .other { content: "}"; }\n }\n}\n.empty {}';
  const blocks = scanBlocks(src);
  assert.deepEqual(blocks.map(b => b.parts.map(p => [p.text, p.line])), [
    [['.a:is(.b, .c)', 1], ['.d[title="x,y"]', 2]],
    [['@media not all', 4]],
    [['.child', 5], ['.other', 5]],
    [['.empty', 8]],
  ]);
  for (const block of blocks) {
    assert.equal(src[block.open], '{');
    assert.equal(src[block.close], '}');
  }
  assert.deepEqual(blocks.map(b => b.hasNested), [true, true, false, false]);
});

test('scanner ignores escaped braces, comments and strings and resets header after declarations', () => {
  const src = '.x { content: "\\\"}"; /* { */ color: red; .y\\{z { --a: "x; }"; } }';
  const blocks = scanBlocks(src);
  assert.equal(blocks.length, 2);
  assert.deepEqual(blocks.map(b => b.parts[0].text), ['.x', '.y\\{z']);
});
