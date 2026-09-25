// Exact, pinned P10 stylesheet closure. Not a general CSS parser, URL normalizer,
// query cache policy, or replacement for the immutable source packet.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';

export const FONT_CSS_SHA256 = '46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441';
const hash = bytes => createHash('sha256').update(bytes).digest('hex');

export function deriveFontAliases({css, cssURL, assets, expectedCSSSHA256 = FONT_CSS_SHA256, expectedOccurrences = 40, expectedAliases = 38}) {
  assert.ok(Buffer.isBuffer(css) && css.length > 0 && css.length <= 64 * 1024, 'bounded pinned font stylesheet required');
  assert.equal(hash(css), expectedCSSSHA256, 'font stylesheet digest drift');
  assert.ok(typeof cssURL === 'string' && /^\/[A-Za-z0-9_./-]+\/inter\.css$/.test(cssURL) && !cssURL.split('/').some(part => part === '.' || part === '..'), 'clean font stylesheet URL required');
  assert.ok(Array.isArray(assets), 'selected asset pins required');
  const pins = new Map();
  for (const asset of assets) {
    assert.ok(asset && typeof asset.url === 'string' && !/[?#%\\]/.test(asset.url) && !pins.has(asset.url), 'unique canonical asset URL required');
    assert.ok(/^[a-f0-9]{64}$/.test(asset.sha256) && Number.isSafeInteger(asset.bytes) && asset.bytes > 0 && asset.bytes <= 16 * 1024 * 1024, 'bounded canonical asset identity required');
    pins.set(asset.url, asset);
  }
  const stylesheet = pins.get(cssURL);
  assert.ok(stylesheet && stylesheet.mime === 'text/css' && stylesheet.sha256 === expectedCSSSHA256 && stylesheet.bytes === css.length, 'stylesheet must match a selected pin');
  const text = css.toString('utf8');
  assert.equal(Buffer.from(text).compare(css), 0, 'stylesheet must be valid UTF-8');
  const tokens = [...text.matchAll(/url\(\s*(['"]?)([^'"()\s]+)\1\s*\)/g)];
  assert.equal(tokens.length, expectedOccurrences, 'fixed stylesheet URL occurrence count');
  assert.equal([...text.matchAll(/url\(/g)].length, tokens.length, 'unparsed stylesheet URL refused');
  const aliases = new Map();
  const prefix = cssURL.slice(0, -'inter.css'.length);
  for (const match of tokens) {
    const literal = match[2];
    assert.match(literal, /^font-files\/[A-Za-z0-9_.-]+\.woff2\?v=4\.1$/, 'only exact declared font query is admitted');
    // This split applies to verified CSS literals, never to an incoming request.
    const canonicalURL = prefix + literal.slice(0, -'?v=4.1'.length);
    const pin = pins.get(canonicalURL);
    assert.ok(pin && pin.mime === 'font/woff2', 'CSS font URL lacks a selected WOFF2 pin');
    const url = prefix + literal;
    aliases.set(url, {url, canonical_url: canonicalURL, mime: pin.mime, sha256: pin.sha256, bytes: pin.bytes});
  }
  assert.equal(aliases.size, expectedAliases, 'fixed font alias count');
  const fonts = assets.filter(asset => asset.mime === 'font/woff2');
  assert.equal(fonts.length, expectedAliases, 'selected font closure count');
  assert.deepEqual([...aliases.values()].map(alias => alias.canonical_url).sort(), fonts.map(font => font.url).sort(), 'CSS aliases must equal the complete selected font closure');
  return {
    schema_version: 1, id: 'p10-font-query-aliases-v1',
    css: {url: cssURL, sha256: expectedCSSSHA256, bytes: css.length},
    aliases: [...aliases.values()].sort((a, b) => a.url < b.url ? -1 : a.url > b.url ? 1 : 0),
  };
}
