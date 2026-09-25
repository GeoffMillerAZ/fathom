import assert from 'node:assert/strict';
import test from 'node:test';
import {assertInventoryEqual} from './inventory.mjs';

const sha = character => character.repeat(64);
const glog = {scope: 'dependency', path: 'github.com/golang/glog/@v/v1.2.4.mod', bytes: 101, sha256: sha('a')};
const jwt = {scope: 'dependency', path: 'github.com/golang-jwt/jwt/v5/@v/v5.3.1.mod', bytes: 202, sha256: sha('b')};

test('canonical comparison accepts the observed prefix-order collision', () => {
  assert.equal(assertInventoryEqual([glog, jwt], [jwt, glog]), true);
});

test('old JSON equality reproduces the false mismatch', () => {
  assert.notEqual(JSON.stringify([glog, jwt]), JSON.stringify([jwt, glog]));
});

test('record property insertion order does not affect equality', () => {
  const reordered = {sha256: glog.sha256, bytes: glog.bytes, path: glog.path, scope: glog.scope};
  assert.equal(assertInventoryEqual([reordered], [glog]), true);
});

for (const [name, change] of [
  ['changed SHA', records => [{...records[0], sha256: sha('c')}, records[1]]],
  ['changed size', records => [{...records[0], bytes: records[0].bytes + 1}, records[1]]],
  ['extra record', records => [...records, {scope: 'dependency', path: 'extra/@v/v1.0.0.mod', bytes: 1, sha256: sha('d')}]],
  ['missing record', records => records.slice(0, 1)],
  ['wrong scope', records => [{...records[0], scope: 'candidate'}, records[1]]],
]) test(`${name} is refused`, () => {
  assert.throws(() => assertInventoryEqual(change([glog, jwt]), [glog, jwt]), /inventory records differ/);
});

test('duplicate keys are refused explicitly', () => {
  assert.throws(() => assertInventoryEqual([glog, {...glog}], [glog]), /duplicate inventory key/);
});

test('invalid records are refused', () => {
  for (const invalid of [null, {}, {...glog, extra: true}, {...glog, bytes: -1}, {...glog, sha256: 'A'.repeat(64)}, {...glog, path: 'bad\0path'}]) {
    assert.throws(() => assertInventoryEqual([invalid], [glog]), /must be a record|exactly|invalid field values/);
  }
});

test('comparison does not mutate either input', () => {
  const actual = [structuredClone(glog), structuredClone(jwt)];
  const expected = [structuredClone(jwt), structuredClone(glog)];
  const before = structuredClone({actual, expected});
  assertInventoryEqual(actual, expected);
  assert.deepEqual({actual, expected}, before);
});
