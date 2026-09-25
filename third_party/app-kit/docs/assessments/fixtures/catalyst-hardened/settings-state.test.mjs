// First-party policy tests only: no Next import, component execution or UI proof.
import assert from 'node:assert/strict';
import { isAbsolute, join } from 'node:path';
import test from 'node:test';
import { pathToFileURL } from 'node:url';

const project = process.env.APPKIT_CATALYST_HARDENED_ROOT;
assert.ok(project && isAbsolute(project), 'Set APPKIT_CATALYST_HARDENED_ROOT to the absolute prepared hardened project; missing evidence is not a skip.');
// Node 26 strips the helper's type-only Country import; this does not load data.ts.
const { BIO_LIMIT, bioLength, bioIssue, regionAfterCountryChange, regionIssue } =
  await import(pathToFileURL(join(project, 'src/app/(app)/settings/settings-state.ts')).href);

const countries = Object.freeze([
  Object.freeze({ name: 'Synthetic A', code: 'AA', flagUrl: '/synthetic-a.svg', regions: Object.freeze(['North', 'Shared']) }),
  Object.freeze({ name: 'Synthetic B', code: 'BB', flagUrl: '/synthetic-b.svg', regions: Object.freeze(['South', 'Shared']) }),
]);

test('biography limit is exactly 240 Unicode code points', () => {
  assert.equal(BIO_LIMIT, 240);
  assert.equal(bioLength(''), 0);
  assert.equal(bioIssue(''), '');
  for (const character of ['x', '😀']) {
    const accepted = character.repeat(240), rejected = character.repeat(241);
    assert.equal(bioLength(accepted), 240);
    assert.equal(bioIssue(accepted), '');
    assert.equal(bioLength(rejected), 241);
    assert.match(bioIssue(rejected), /^Remove 1 Unicode code point to stay within 240\./);
  }
});

test('combining sequences count as multiple code points, not graphemes', () => {
  assert.equal(bioLength('e\u0301'), 2);
  assert.equal(bioLength('e\u0301'.repeat(120)), 240);
  assert.equal(bioIssue('e\u0301'.repeat(120)), '');
  assert.equal(bioLength('e\u0301'.repeat(121)), 242);
  assert.match(bioIssue('e\u0301'.repeat(121)), /^Remove 2 Unicode code points to stay within 240\./);
});

test('ZWJ and modifier sequences retain their individual code points', () => {
  const family = '👩‍👩‍👧‍👦';
  assert.equal(bioLength(family), 7);
  assert.equal(bioLength('👍🏽'), 2);
  const accepted = family.repeat(34) + 'xx';
  assert.equal(bioLength(accepted), 240);
  assert.equal(bioIssue(accepted), '');
  assert.equal(bioLength(accepted + 'x'), 241);
  assert.match(bioIssue(accepted + 'x'), /^Remove 1 Unicode code point to stay within 240\./);
});

test('over-limit biography reports the full excess without shortening the draft', () => {
  const draft = '  ' + '😀'.repeat(241) + '\ne\u0301';
  const before = draft;
  assert.equal(bioLength(draft), 246);
  const issue = bioIssue(draft);
  assert.equal(draft, before);
  assert.equal(bioLength(draft), 246);
  assert.equal(issue, 'Remove 6 Unicode code points to stay within 240. Your draft has not been shortened.');
});

test('non-string biographies are rejected rather than coerced or counted', () => {
  for (const value of [undefined, null, 0, false, ['text'], { text: 'text' }, new String('text'), Symbol('text')]) {
    assert.equal(bioIssue(value), 'Enter a text biography.');
  }
});

test('country changes preserve only an exact region member', () => {
  assert.equal(regionAfterCountryChange(countries[0], 'North'), 'North');
  assert.equal(regionAfterCountryChange(countries[1], 'Shared'), 'Shared');
  for (const region of ['North', 'Unknown', 'shared', 'SHARED', ' Shared', 'Shared ', '']) {
    assert.equal(regionAfterCountryChange(countries[1], region), '');
  }
  assert.deepEqual(countries[0].regions, ['North', 'Shared']);
  assert.deepEqual(countries[1].regions, ['South', 'Shared']);
});

test('region validation accepts only a selected country and exact member pair', () => {
  assert.equal(regionIssue(countries, 'AA', 'North'), '');
  assert.equal(regionIssue(countries, 'BB', 'South'), '');
  assert.equal(regionIssue(countries, 'AA', 'Shared'), '');
  assert.equal(regionIssue(countries, 'BB', 'Shared'), '');
  for (const region of ['North', '', 'south', 'South ', 'Unknown', undefined, null, 0, false, ['South'], new String('South')]) {
    assert.equal(regionIssue(countries, 'BB', region), 'Choose a region in Synthetic B.');
  }
});

test('unknown, case-variant, empty and non-string country codes are rejected', () => {
  for (const code of ['ZZ', 'aa', 'AA ', '', undefined, null, 0, false, ['AA'], new String('AA')]) {
    assert.equal(regionIssue(countries, code, 'North'), 'Choose a country from the available options.');
  }
  assert.equal(regionIssue([], 'AA', 'North'), 'Choose a country from the available options.');
});
