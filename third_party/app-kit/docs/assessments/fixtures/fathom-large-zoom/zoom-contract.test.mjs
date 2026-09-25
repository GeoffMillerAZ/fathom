import assert from 'node:assert/strict';
import test from 'node:test';
import { inspectZoom } from './zoom-contract.mjs';

const exposed = { dpr: 2, inner: 720, outer: 1440, scale: 1, scrollWidth: 712 };
const metrics = { contentSize: { width: 1425 }, cssContentSize: { width: 712 } };
test('native zoom admits the observed scrollbar and integer CSS rounding', () => {
  assert.equal(inspectZoom('2', exposed, metrics), true);
  assert.equal(inspectZoom('1', { dpr: 1, inner: 1440, outer: 1440, scale: 1, scrollWidth: 1425 },
    { contentSize: { width: 1425 }, cssContentSize: { width: 1425 } }), true);
  assert.equal(inspectZoom('2', { ...exposed, scrollWidth: 720 },
    { contentSize: { width: 1440 }, cssContentSize: { width: 720 } }), true);
});
for (const [name, field, value] of [
  ['narrow viewport without zoom', 'dpr', 1], ['wrong inner width', 'inner', 1440],
  ['wrong outer width', 'outer', 720], ['pinch zoom', 'scale', 2],
  ['overflow', 'scrollWidth', 721], ['missing document extent', 'scrollWidth', 0],
]) test('refuses ' + name, () => assert.equal(inspectZoom('2', { ...exposed, [field]: value }, metrics), false));
for (const [name, bad] of [
  ['wrong physical ratio', { contentSize: { width: 712 }, cssContentSize: { width: 712 } }],
  ['physical extent exceeds viewport', { contentSize: { width: 1441 }, cssContentSize: { width: 712 } }],
  ['CSS extent differs from document', { contentSize: { width: 1425 }, cssContentSize: { width: 700 } }],
  ['missing metrics', {}], ['nonfinite metrics', { contentSize: { width: NaN }, cssContentSize: { width: 712 } }],
  ['fractional physical extent', { contentSize: { width: 1424.5 }, cssContentSize: { width: 712 } }],
]) test('refuses ' + name, () => assert.equal(inspectZoom('2', exposed, bad), false));
test('setting and metric scale must agree', () => {
  assert.equal(inspectZoom('1', exposed, metrics), false);
  assert.equal(inspectZoom('3', exposed, metrics), false);
});
