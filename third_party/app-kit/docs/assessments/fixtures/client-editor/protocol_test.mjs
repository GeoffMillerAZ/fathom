import assert from 'node:assert/strict';
import test from 'node:test';
import { parseInitial, parseContext } from './protocol.ts';

const initial = () => ({ version: 1, instanceId: 'primary', label: 'Description', hint: '', initialText: '' });
const context = () => ({ version: 1, instanceId: 'primary', label: 'Updated', hint: 'Local only' });

test('valid initial/context records are detached, frozen and preserve meaningful text', () => {
  const input = initial(); input.initialText = '\nHello\r\n世界 🌵\n';
  const parsed = parseInitial(input); input.label = 'Changed';
  assert.equal(parsed.label, 'Description'); assert.equal(parsed.initialText, '\nHello\r\n世界 🌵\n');
  assert.ok(Object.isFrozen(parsed)); assert.throws(() => { parsed.hint = 'no'; }, TypeError);
  const source = context(), update = parseContext(source, 'primary'); source.label = 'Changed';
  assert.equal(update.label, 'Updated'); assert.ok(Object.isFrozen(update));
  assert.deepEqual(parseInitial(Object.assign(Object.create(null), initial())), initial());
});

test('reject malformed shapes, accessors, symbols, versions and primitive types without coercion', () => {
  for (const value of [null, undefined, [], 'record', 1, new Date(), Object.create(initial())]) {
    assert.throws(() => parseInitial(value));
  }
  for (const field of Object.keys(initial())) {
    const missing = initial(); delete missing[field]; assert.throws(() => parseInitial(missing));
    for (const value of [null, undefined, {}, [], true]) assert.throws(() => parseInitial({ ...initial(), [field]: value }));
  }
  for (const version of [0, 2, '1', 1n, NaN]) assert.throws(() => parseInitial({ ...initial(), version }));
  assert.throws(() => parseInitial({ ...initial(), extra: true }));
  assert.throws(() => parseInitial({ ...initial(), [Symbol('extra')]: true }));
  let called = false;
  const accessor = { ...initial(), get label() { called = true; return 'x'; } };
  assert.throws(() => parseInitial(accessor)); assert.equal(called, false);
});

test('ASCII instance identifiers and nonempty labels have exact bounds', () => {
  assert.equal(parseInitial({ ...initial(), instanceId: 'A' + '-9'.repeat(23) + 'Z' }).instanceId.length, 48);
  for (const instanceId of ['', '1a', '-a', 'a_b', 'a b', 'é', 'a\n', 'a'.repeat(49)]) {
    assert.throws(() => parseInitial({ ...initial(), instanceId }));
  }
  assert.throws(() => parseInitial({ ...initial(), label: '' }));
});

test('byte budgets admit exact ASCII, CJK and emoji boundaries but never truncate', () => {
  for (const field of ['label', 'hint']) {
    for (const value of ['a'.repeat(512), '界'.repeat(170) + 'ab', '🌵'.repeat(128)]) {
      assert.equal(parseInitial({ ...initial(), [field]: value })[field], value);
      assert.throws(() => parseInitial({ ...initial(), [field]: value + 'a' }), RangeError);
    }
  }
  for (const value of ['a'.repeat(65536), '界'.repeat(21845) + 'a', '🌵'.repeat(16384)]) {
    assert.equal(parseInitial({ ...initial(), initialText: value }).initialText, value);
    assert.throws(() => parseInitial({ ...initial(), initialText: value + 'a' }), RangeError);
  }
});

test('reject lone surrogates instead of silently encoding replacement characters', () => {
  for (const field of ['label', 'hint', 'initialText']) {
    for (const value of ['\ud800', '\udfff', 'a\ud800b', '\ud800\ud800', '\udc00\ud800']) {
      assert.throws(() => parseInitial({ ...initial(), [field]: value }), /Unicode/);
    }
    assert.equal(parseInitial({ ...initial(), [field]: '\ufffd🌵' })[field], '\ufffd🌵');
  }
});

test('grossly oversized strings refuse before UTF-8 allocation', t => {
  const encode = TextEncoder.prototype.encode;
  let longest = 0;
  t.mock.method(TextEncoder.prototype, 'encode', function (value) {
    longest = Math.max(longest, value.length);
    return encode.call(this, value);
  });
  assert.throws(() => parseInitial({ ...initial(), initialText: 'a'.repeat(1000000) }), RangeError);
  assert.ok(longest < 512, `Unexpected allocation for ${longest} code units`);
});

test('context updates cannot replace documents, target another instance or bypass text validation', () => {
  assert.throws(() => parseContext(initial(), 'primary'));
  for (const field of ['initialText', 'document', 'html']) assert.throws(() => parseContext({ ...context(), [field]: 'replacement' }, 'primary'));
  assert.throws(() => parseContext(context(), 'secondary'), /mismatch/);
  for (const field of Object.keys(context())) {
    const missing = context(); delete missing[field]; assert.throws(() => parseContext(missing, 'primary'));
  }
  for (const label of ['', '\ud800', '🌵'.repeat(129)]) assert.throws(() => parseContext({ ...context(), label }, 'primary'));
  assert.throws(() => parseContext({ ...context(), hint: 'a'.repeat(513) }, 'primary'));
});
