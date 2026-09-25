import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import test from 'node:test';
import {deriveFontAliases} from './font-query-aliases.mjs';

const hash = value => createHash('sha256').update(value).digest('hex');
function fixture(source = `a{src:url('font-files/Inter-Regular.woff2?v=4.1')}b{src:url("font-files/Inter-Bold.woff2?v=4.1")}c{src:url(font-files/Inter-Regular.woff2?v=4.1)}`) {
  const css = Buffer.from(source), cssURL = '/pinned/inter.css';
  const assets = [{url: cssURL, mime: 'text/css', sha256: hash(css), bytes: css.length}, ...['Inter-Regular', 'Inter-Bold'].map(name => ({url: `/pinned/font-files/${name}.woff2`, mime: 'font/woff2', sha256: hash(name), bytes: name.length}))];
  return {css, cssURL, assets, expectedCSSSHA256: hash(css), expectedOccurrences: 3, expectedAliases: 2};
}

test('exact CSS closure is deterministic, complete and query-preserving', () => {
  const input = fixture(), manifest = deriveFontAliases(input);
  assert.deepEqual(deriveFontAliases({...input, assets: [...input.assets].reverse()}), manifest);
  assert.equal(manifest.aliases.length, 2);
  assert.equal(manifest.aliases[0].url, '/pinned/font-files/Inter-Bold.woff2?v=4.1');
  assert.equal(manifest.aliases[0].canonical_url, '/pinned/font-files/Inter-Bold.woff2');
  assert.equal(manifest.css.sha256, input.expectedCSSSHA256);
  assert.equal('body' in manifest, false);
});

test('stylesheet identity, URL grammar, counts and missing font pins fail closed', () => {
  const input = fixture();
  assert.throws(() => deriveFontAliases({...input, css: Buffer.from('drift')}), /digest drift/);
  assert.throws(() => deriveFontAliases({...input, expectedOccurrences: 4}), /occurrence count/);
  assert.throws(() => deriveFontAliases({...input, expectedAliases: 3}), /alias count/);
  assert.throws(() => deriveFontAliases({...input, assets: input.assets.slice(0, 2)}), /lacks a selected/);
  assert.throws(() => deriveFontAliases({...input, assets: [...input.assets, input.assets[1]]}), /unique canonical/);
  for (const value of ['v=4.2', 'v=4.1&extra=1', 'v=4.1&v=4.1', 'V=4.1']) {
    const altered = fixture(input.css.toString().replace('v=4.1', value));
    assert.throws(() => deriveFontAliases(altered), /exact declared/);
  }
  assert.throws(() => deriveFontAliases(fixture(input.css.toString().replace('font-files/', '../font-files/'))), /exact declared/);
  assert.throws(() => deriveFontAliases(fixture(input.css.toString().replace('font-files/', 'font%2dfiles/'))), /exact declared/);
  assert.throws(() => deriveFontAliases({...input, assets: input.assets.map((asset, i) => i ? asset : {...asset, bytes: 1})}), /stylesheet must match/);
});

test('extra pinned fonts cannot be silently omitted from the alias manifest', () => {
  const input = fixture();
  input.assets.push({url: '/pinned/font-files/Extra.woff2', mime: 'font/woff2', sha256: hash('extra'), bytes: 5});
  assert.throws(() => deriveFontAliases(input), /font closure count/);
});
