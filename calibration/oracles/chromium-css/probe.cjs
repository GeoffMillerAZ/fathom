'use strict';

const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { startBrowser } = require('./cdp.cjs');
const sha = value => crypto.createHash('sha256').update(value).digest('hex');

async function probe(binary, pin, scratch) {
  assert.equal(sha(fs.readFileSync(binary)), pin.binary_sha256, 'browser checksum differs from pin');
  fs.mkdirSync(scratch, { recursive: true });
  const profile = fs.mkdtempSync(path.join(scratch, 'probe-profile-'));
  const browser = startBrowser(binary, { profileDir: profile });
  try {
    const version = await browser.send('Browser.getVersion');
    assert.ok(version.product.endsWith('/' + pin.version), 'browser version differs from pin');
    const target = await browser.createTargetSession();
    for (const domain of ['Page', 'DOM', 'CSS', 'Network']) await target.send(domain + '.enable');
    await target.send('Network.setBlockedURLs', { urls: ['http://*', 'https://*', 'file://*'] });
    const frame = (await target.send('Page.getFrameTree')).frameTree.frame.id;
    const { styleSheetId } = await target.send('CSS.createStyleSheet', { frameId: frame });
    // These fixtures are controls for the instrument, never language-grade evidence.
    const sources = [
      { name: 'comma', source: '.one,\n.two {\n color: red;\n --tone: green;\n}\n' },
      { name: 'utf16-crlf', source: '/* 🧪 */ .one {\r\n content: "{🧪}"; /* } */\r\n --tone: blue;\r\n}\r\n' },
      { name: 'nested', source: '.parent {\n color: red;\n .child { color: blue; }\n}\n' },
      { name: 'empty', source: '.empty {}\n' },
      { name: 'inactive-media', source: '@media not all {\n .under { color: blue; }\n}\n', inner: true },
    ];
    const results = [];
    for (const fixture of sources) {
      const source = fixture.source;
      await target.send('CSS.setStyleSheetText', { styleSheetId, text: source });
      const starts = [0];
      for (let i = 0; i < source.length; i++) if (source[i] === '\n') starts.push(i + 1);
      const point = offset => {
        let line = starts.length - 1;
        while (starts[line] > offset) line--;
        return { line, column: offset - starts[line] };
      };
      const open = source.indexOf('{', fixture.inner ? source.indexOf('{') + 1 : 0);
      const close = fixture.inner ? source.indexOf('}', open) : source.lastIndexOf('}');
      const a = point(open + 1), b = point(close);
      const range = { startLine: a.line, startColumn: a.column, endLine: b.line, endColumn: b.column };
      const body = source.slice(open + 1, close);
      const badStart = point(Math.min(open + 2, close));
      const badRanges = [
        { ...range, endColumn: range.endColumn + 1 },
        { ...range, startLine: badStart.line, startColumn: badStart.column },
      ];
      let negativeControls = 0;
      for (const bad of badRanges) {
        // Empty bodies have no distinct in-bounds start; the end control remains meaningful.
        if (JSON.stringify(bad) === JSON.stringify(range)) continue;
        let negative;
        try { await target.send('CSS.setStyleTexts', { edits: [{ styleSheetId, range: bad, text: body }] }); negative = 'accepted'; }
        catch (error) { negative = error.message; }
        assert.notEqual(negative, 'accepted', fixture.name + ': malformed body range accepted');
        assert.equal((await target.send('CSS.getStyleSheetText', { styleSheetId })).text, source, 'negative control changed text');
        negativeControls++;
      }
      let positive;
      try {
        const answer = await target.send('CSS.setStyleTexts', { edits: [{ styleSheetId, range, text: body }] });
        assert.equal(answer.styles.length, 1);
        assert.deepEqual(answer.styles[0].range, range, 'native range differs from proposal');
        positive = { accepted: true, range: answer.styles[0].range,
          properties: answer.styles[0].cssProperties.filter(p => p.range).map(p => ({ name: p.name, range: p.range })) };
      } catch (error) { positive = { accepted: false, reason: error.message }; }
      assert.equal((await target.send('CSS.getStyleSheetText', { styleSheetId })).text, source, 'positive control changed text');
      results.push({ name: fixture.name, source_sha256: sha(source), negative_controls_rejected: negativeControls, positive });
    }
    assert.ok(results.find(r => r.name === 'comma').positive.accepted, 'ordinary native range route unavailable');
    return { browser: version, pin, controls_only: true, grade_claim: false, results };
  } finally {
    await browser.close();
    fs.rmSync(profile, { recursive: true, force: true });
  }
}

if (require.main === module) {
  const [binary, pinFile, scratch, output] = process.argv.slice(2);
  if (!binary || !pinFile || !scratch || !output) throw new Error('usage: node probe.cjs BROWSER PIN_JSON SCRATCH OUTPUT_JSON');
  probe(binary, JSON.parse(fs.readFileSync(pinFile, 'utf8')), scratch).then(result => {
    fs.writeFileSync(output, JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ controls: result.results.length, native_accepted: result.results.filter(r => r.positive.accepted).length }));
  }).catch(error => { console.error(error.message); process.exitCode = 1; });
}
module.exports = { probe };
