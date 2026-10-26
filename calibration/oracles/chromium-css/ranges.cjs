'use strict';

// Native end-range companion for a separately produced LightningCSS definition oracle.
// Usage: node ranges.cjs BROWSER PIN_JSON SUBJECT PRIMARY_ORACLE_JSON OUTPUT_RANGES_JSON SCRATCH [DIR...]
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { startBrowser } = require('./cdp.cjs');
const { scanBlocks, lineStarts, lineOf, point } = require('./blocks.cjs');
const { listCSS, requireSameSource, readBoundPrimary, binaryCompare } = require('./source.cjs');

const TIME_LIMIT_MS = 20 * 60 * 1000;
const key = (kind, name, line) => JSON.stringify([kind, name, line]);

function sourceOffset(starts, source, rangePoint) {
  const { line, column } = rangePoint;
  if (!Number.isInteger(line) || !Number.isInteger(column) || line < 0 || column < 0 || line >= starts.length) {
    throw new Error('invalid native CSS range coordinate');
  }
  const offset = starts[line] + column;
  if (offset > source.length || (line + 1 < starts.length && offset >= starts[line + 1])) {
    throw new Error('native CSS range coordinate outside source line');
  }
  return offset;
}

function lastOccupiedLine(starts, source, range) {
  const end = sourceOffset(starts, source, { line: range.endLine, column: range.endColumn });
  const begin = sourceOffset(starts, source, { line: range.startLine, column: range.startColumn });
  if (end <= begin) throw new Error('empty native CSSProperty range');
  return lineOf(starts, end - 1);
}

function exactProposalRange(starts, block) {
  const first = point(starts, block.open + 1);
  const last = point(starts, block.close);
  return { startLine: first.line, startColumn: first.column, endLine: last.line, endColumn: last.column };
}

function sameRange(a, b) {
  return a && b && a.startLine === b.startLine && a.startColumn === b.startColumn &&
    a.endLine === b.endLine && a.endColumn === b.endColumn;
}

function indexDefinitions(definitions, files) {
  const inScope = new Set(files);
  const byPath = new Map();
  const totals = { selector: 0, custom_property: 0, keyframes: 0 };
  for (const definition of definitions) {
    if (!inScope.has(definition.path)) throw new Error('primary oracle definition outside CSS source scope');
    if (!Number.isInteger(definition.line) || definition.line < 1 || typeof definition.name !== 'string') {
      throw new Error('invalid primary oracle definition');
    }
    if (!(definition.kind in totals)) continue;
    totals[definition.kind]++;
    if (!byPath.has(definition.path)) byPath.set(definition.path, []);
    byPath.get(definition.path).push(definition);
  }
  return { byPath, totals };
}

function attestFile(source, rel, primary, acceptedStyles, stats) {
  const starts = lineStarts(source);
  const unused = new Map();
  for (const definition of primary) {
    const k = key(definition.kind, definition.name, definition.line);
    if (!unused.has(k)) unused.set(k, []);
    unused.get(k).push(definition);
  }
  const ranges = [];
  const add = (kind, name, line, end) => {
    const matches = unused.get(key(kind, name, line));
    if (!matches || !matches.length) return;
    matches.shift();
    ranges.push({ path: rel, name, kind, line, end });
    stats.attested[kind]++;
  };
  for (const { block, style } of acceptedStyles) {
    const end = lineOf(starts, block.close);
    // A parent's native CSSStyle body is not a proven selector end when it
    // encloses nested rules; leave that definition explicitly unmeasured.
    if (!block.hasNested) for (const part of block.parts) add('selector', part.text, part.line, end);
    for (const property of style.cssProperties || []) {
      if (!property.name || !property.name.startsWith('--') || !property.range ||
          property.disabled || property.implicit || property.parsedOk === false) continue;
      const line = property.range.startLine + 1;
      const propertyEnd = lastOccupiedLine(starts, source, property.range);
      add('custom_property', property.name, line, propertyEnd);
    }
  }
  return ranges;
}

async function rangeCompanion(binary, pin, subject, primary, scratch, dirs = [], sourceManifest) {
  if (!Array.isArray(primary.definitions) || primary.tool !== 'lightningcss') {
    throw new Error('PRIMARY_ORACLE_JSON must be a LightningCSS definitions oracle');
  }
  requireSameSource(sourceManifest, subject, dirs);
  const files = listCSS(subject, dirs);
  const { byPath, totals } = indexDefinitions(primary.definitions, files);
  assert.equal(require('node:crypto').createHash('sha256').update(fs.readFileSync(binary)).digest('hex'),
    pin.binary_sha256, 'browser checksum differs from pin');
  fs.mkdirSync(scratch, { recursive: true });
  const profile = fs.mkdtempSync(path.join(scratch, 'ranges-profile-'));
  let browser;
  let cancelled;
  const cancel = reason => {
    cancelled = reason;
    if (browser) void browser.close().catch(() => {});
  };
  const onTerm = () => cancel(new Error('SIGTERM interrupted native CSS range run'));
  process.once('SIGTERM', onTerm);
  const deadline = setTimeout(() => cancel(new Error('native CSS range run exceeded 20 minutes')), TIME_LIMIT_MS);
  const check = () => { if (cancelled) throw cancelled; };
  const stats = { files: files.length, candidates: 0, accepted: 0, rejected: 0,
    total: totals, attested: { selector: 0, custom_property: 0, keyframes: 0 } };
  const ranges = [];
  let version;
  let result;
  try {
    browser = startBrowser(binary, { profileDir: profile });
    version = await browser.send('Browser.getVersion');
    check();
    assert.ok(version.product.endsWith('/' + pin.version), 'browser version differs from pin');
    for (let index = 0; index < files.length; index++) {
      check();
      const rel = files[index];
      const source = fs.readFileSync(path.join(subject, rel), 'utf8');
      const starts = lineStarts(source);
      const defs = byPath.get(rel) || [];
      const possible = new Map();
      for (const definition of defs) {
        const k = key(definition.kind, definition.name, definition.line);
        possible.set(k, (possible.get(k) || 0) + 1);
      }
      const candidates = scanBlocks(source).filter(block => {
        if (block.parts.some(part => possible.has(key('selector', part.text, part.line)))) return true;
        const first = lineOf(starts, block.open + 1), last = lineOf(starts, block.close);
        return defs.some(d => d.kind === 'custom_property' && d.line >= first && d.line <= last);
      });
      const target = await browser.createTargetSession();
      try {
        for (const domain of ['Page', 'DOM', 'CSS', 'Network']) await target.send(domain + '.enable');
        await target.send('Network.setBlockedURLs', { urls: ['http://*', 'https://*', 'file://*'] });
        const frameId = (await target.send('Page.getFrameTree')).frameTree.frame.id;
        const { styleSheetId } = await target.send('CSS.createStyleSheet', { frameId });
        await target.send('CSS.setStyleSheetText', { styleSheetId, text: source });
        assert.equal((await target.send('CSS.getStyleSheetText', { styleSheetId })).text, source,
          'browser changed initial CSS source');
        const acceptedStyles = [];
        for (const block of candidates) {
          check();
          assert.equal(source[block.close], '}', 'scanner did not propose a closing brace');
          const range = exactProposalRange(starts, block);
          const text = source.slice(block.open + 1, block.close);
          stats.candidates++;
          let answer;
          try {
            answer = await target.send('CSS.setStyleTexts', { edits: [{ styleSheetId, range, text }] });
          } catch (error) {
            if (/Source range did(?:n't| not) match existing style source range/i.test(error.message)) {
              stats.rejected++;
              continue;
            }
            throw error;
          }
          if (!Array.isArray(answer.styles) || answer.styles.length !== 1 || !sameRange(answer.styles[0].range, range)) {
            throw new Error('native CSSStyle range differs from proposal');
          }
          stats.accepted++;
          acceptedStyles.push({ block, style: answer.styles[0] });
        }
        assert.equal((await target.send('CSS.getStyleSheetText', { styleSheetId })).text, source,
          'browser changed CSS source after no-op edits');
        ranges.push(...attestFile(source, rel, defs, acceptedStyles, stats));
      } finally {
        await browser.send('Target.closeTarget', { targetId: target.targetId });
      }
      if ((index + 1) % 25 === 0 || index + 1 === files.length) {
        process.stderr.write(JSON.stringify({ processed: index + 1, files: files.length,
          candidates: stats.candidates, accepted: stats.accepted, rejected: stats.rejected }) + '\n');
      }
    }
    check();
    ranges.sort((a, b) => binaryCompare(a.path, b.path) || a.line - b.line ||
      binaryCompare(a.name, b.name) || binaryCompare(a.kind, b.kind) || a.end - b.end);
    const unattested = Object.fromEntries(Object.keys(totals).map(kind => [kind, totals[kind] - stats.attested[kind]]));
    result = {
      companion: {
        tool: 'Chromium DevTools Protocol CSS.setStyleTexts', tier: 'A', version: pin.version,
        units: files.length, errored: 0,
        kinds: { selector: 'selector', custom_property: 'variable' },
        scope: 'Native-validated CSSStyle body ranges for eligible selectors and CSSProperty ranges for custom properties; nested-parent selectors and keyframes without a native range are unmeasured',
        ranges,
      },
      diagnostics: { ...stats, unattested },
    };
  } finally {
    clearTimeout(deadline);
    process.removeListener('SIGTERM', onTerm);
    try { if (browser) await browser.close(); }
    finally { fs.rmSync(profile, { recursive: true, force: true }); }
  }
  requireSameSource(sourceManifest, subject, dirs);
  return result;
}

async function main(args) {
  const [binary, pinFile, subject, primaryFile, outputFile, scratch, ...dirs] = args;
  if (!binary || !pinFile || !subject || !primaryFile || !outputFile || !scratch) {
    throw new Error('usage: node ranges.cjs BROWSER PIN_JSON SUBJECT PRIMARY_ORACLE_JSON OUTPUT_RANGES_JSON SCRATCH [DIR...]');
  }
  if (fs.existsSync(outputFile)) throw new Error('output already exists; remove stale companion before run');
  const pin = JSON.parse(fs.readFileSync(pinFile, 'utf8'));
  const { oracle, manifest } = readBoundPrimary(primaryFile, subject, dirs);
  const { companion, diagnostics } = await rangeCompanion(binary, pin, subject, oracle, scratch, dirs, manifest);
  const temp = outputFile + '.tmp-' + process.pid;
  try {
    fs.writeFileSync(temp, JSON.stringify(companion, null, 2) + '\n', { flag: 'wx' });
    fs.renameSync(temp, outputFile);
  } finally {
    fs.rmSync(temp, { force: true });
  }
  process.stderr.write(JSON.stringify(diagnostics) + '\n');
}

if (require.main === module) main(process.argv.slice(2)).catch(error => {
  console.error(error.message);
  process.exitCode = 1;
});

module.exports = { rangeCompanion, exactProposalRange, lastOccupiedLine, attestFile };
