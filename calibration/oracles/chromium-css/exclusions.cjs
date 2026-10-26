'use strict';
// Retrospective proposal classification. This never produces oracle ends.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { scanBlocks } = require('./blocks.cjs');
const { readBoundPrimary, requireSameSource } = require('./source.cjs');
const key = row => JSON.stringify([row.path, row.kind, row.name, row.line]);
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
function classify(definitions, ranges, sources) {
  const available = new Map();
  for (const row of ranges) available.set(key(row), (available.get(key(row)) || 0) + 1);
  const totals = { primary: 0, attested: 0, excluded: 0, by_kind: {}, selector_proposals: {} };
  const blocks = new Map();
  for (const [rel, source] of sources) blocks.set(rel, scanBlocks(source));
  const inc = (object, label) => { object[label] = (object[label] || 0) + 1; };
  for (const row of definitions) {
    const k = key(row);
    const attested = (available.get(k) || 0) > 0;
    if (attested) available.set(k, available.get(k) - 1);
    if (row.path.endsWith('.min.css')) continue;
    totals.primary++;
    if (attested) { totals.attested++; continue; }
    totals.excluded++;
    inc(totals.by_kind, row.kind);
    if (row.kind !== 'selector') continue;
    const matches = (blocks.get(row.path) || []).filter(block => block.parts.some(part => part.text === row.name && part.line === row.line));
    let reason;
    if (!matches.length) reason = 'unmatched_source_header';
    else if (matches.length > 1) reason = 'multiple_matching_proposals';
    else reason = matches[0].hasNested ? 'nested_parent_proposal' : 'flat_proposal_unattested';
    inc(totals.selector_proposals, reason);
  }
  assert.ok([...available.values()].every(n => n === 0), 'companion is not a multiset subset of primary');
  assert.equal(totals.primary, totals.attested + totals.excluded);
  return totals;
}
function main(args) {
  if (args[0] === '--self-test') {
    const defs = [
      {path:'a.css',kind:'selector',name:'.parent',line:1},
      {path:'a.css',kind:'selector',name:'.flat',line:4},
      {path:'a.css',kind:'selector',name:'.unknown',line:5},
      {path:'a.css',kind:'keyframes',name:'k',line:6},
    ];
    const sources = new Map([['a.css','.parent {\n .child {}\n}\n.flat {}\n']]);
    const result = classify(defs, [], sources);
    assert.deepEqual(result.by_kind, {selector:3,keyframes:1});
    assert.deepEqual(result.selector_proposals, {nested_parent_proposal:1,flat_proposal_unattested:1,unmatched_source_header:1});
    assert.throws(() => classify([], [defs[0]], sources), /subset/);
    console.log('EXCLUSION CONTROLS PASS'); return;
  }
  const [subject, primary, companion, ...dirs] = args;
  if (!subject || !primary || !companion) throw new Error('usage: exclusions.cjs ROOT PRIMARY COMPANION [DIR...]');
  const {oracle, manifest} = readBoundPrimary(primary, subject, dirs);
  const ranges = JSON.parse(fs.readFileSync(companion, 'utf8'));
  const sources = new Map(manifest.files.map(file => [file.path, fs.readFileSync(path.join(subject,file.path),'utf8')]));
  const counts = classify(oracle.definitions, ranges.ranges, sources);
  requireSameSource(manifest,subject,dirs);
  console.log(JSON.stringify({source_digest:manifest.digest,primary_sha256:hash(primary),companion_sha256:hash(companion),...counts,
    interpretation:'Retrospective source-proposal classes, not recorded native rejection causes. Flat proposals were not attested in the frozen output; native errors were not persisted. No ends or source files are changed.'}));
}
if (require.main === module) { try {main(process.argv.slice(2));} catch(e) {console.error(e.message);process.exitCode=1;} }
module.exports = { classify };
