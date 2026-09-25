import assert from 'node:assert/strict';
import test from 'node:test';
import {
  countCodePoints,
  textareaCodePoints,
  parseSettingsConfig,
  retainedRegionAfterCountryChange,
} from './catalyst_enhance.mjs';

const initial = {
  name: 'Example', bio: '', email: 'example@example.test', email_is_public: true,
  address: '1 Example Way', city: 'Exampletown', postal_code: '12345',
  country: 'US', region: 'Arizona', currency: 'USD',
};

test('bio uses Unicode code points rather than UTF-16 code units', () => {
  assert.equal(countCodePoints('a😀e\u0301'), 4);
  assert.equal(countCodePoints('😀'.repeat(241)), 241);
  assert.equal(textareaCodePoints('\r'), 1);
  assert.equal(textareaCodePoints('\n'), 1);
  assert.equal(textareaCodePoints('\r\n'), 1);
  assert.equal(textareaCodePoints('\r\r\n'), 2);
});

test('config is exact, bounded, and preserves escaped template-hostile text', () => {
  const source = JSON.stringify({ schema_version: 1, bio_limit: 240, initial: { ...initial, bio: '</template> " & \u2028\u2029' } });
  assert.deepEqual(parseSettingsConfig(source)?.initial.bio, '</template> " & \u2028\u2029');
  assert.equal(parseSettingsConfig(JSON.stringify({ schema_version: 1, bio_limit: 241, initial })), null);
  assert.equal(parseSettingsConfig(JSON.stringify({ schema_version: 1, bio_limit: 240, initial: { ...initial, extra: 'no' } })), null);
  assert.equal(parseSettingsConfig('{not json'), null);
});

test('config refuses duplicate members before their last value can win', () => {
  const wire = JSON.stringify({ schema_version: 1, bio_limit: 240, initial });
  assert.equal(parseSettingsConfig(wire.replace('"schema_version":1', '"schema_version":0,"schema_version":1')), null);
  assert.equal(parseSettingsConfig(wire.replace('"country":"US"', '"country":"MX","country":"US"')), null);
  assert.equal(parseSettingsConfig(wire.replace('"name":"Example"', '"n\\u0061me":"first","name":"Example"')), null);
});

test('config bounds UTF-8 bytes and rejects invalid Unicode and control data', () => {
  const config = (changes) => JSON.stringify({ schema_version: 1, bio_limit: 240, initial: { ...initial, ...changes } });
  assert.equal(parseSettingsConfig(' '.repeat((64 << 10) + 1) + config({})), null);
  assert.equal(parseSettingsConfig(config({ name: 'a'.repeat(513) })), null);
  assert.equal(parseSettingsConfig(config({ name: 'é'.repeat(257) })), null);
  assert.equal(parseSettingsConfig(config({ name: '\ud800' })), null);
  assert.equal(parseSettingsConfig(config({ email: 'a\u0000@example.test' })), null);
  assert.equal(parseSettingsConfig(config({ bio: 'line one\nline two\t' }))?.initial.bio, 'line one\nline two\t');
  assert.equal(parseSettingsConfig(config({ bio: '\u0000' })), null);
  assert.equal(parseSettingsConfig(config({ bio: '😀'.repeat(241) })), null);
  const exact = 'x'.repeat(119) + '\r\n' + 'x'.repeat(120);
  assert.equal(parseSettingsConfig(config({ bio: exact }))?.initial.bio, exact);
  assert.equal(parseSettingsConfig(config({ bio: 'x'.repeat(120) + '\r\n' + 'x'.repeat(120) })), null);
});

test('country changes retain only canonical region membership', () => {
  const groups = new Map([
    ['US', { regions: new Set(['Arizona', 'Colorado']) }],
    ['MX', { regions: new Set(['Jalisco']) }],
  ]);
  assert.equal(retainedRegionAfterCountryChange(groups, 'US', 'Arizona'), 'Arizona');
  assert.equal(retainedRegionAfterCountryChange(groups, 'MX', 'Arizona'), '');
  assert.equal(retainedRegionAfterCountryChange(groups, 'ZZ', 'Arizona'), '');
});
