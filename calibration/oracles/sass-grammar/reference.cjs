#!/usr/bin/env node
'use strict';

// Cross-check grammar errors against sass-parser's own acceptance of the
// frozen corpus. Arguments: gate.json out.json name=path name=path.
const fs = require('fs');
const path = require('path');
const cp = require('child_process');
const sp = require('sass-parser');

const FROZEN = new Map([
  ['jgthms/bulma', { sha: '3e00a8e6d0d0e566d507328f0185ef84854effba', scope: '.', files: 91 }],
  ['vuetifyjs/vuetify', { sha: 'b1cd40459b4a0801c021c0985ccd46fa65b9f579', scope: 'packages/vuetify/src/', files: 128 }],
]);
const [gatePath, outPath, ...subjectArgs] = process.argv.slice(2);
if (!gatePath || !outPath || subjectArgs.length !== FROZEN.size) {
  throw new Error('usage: node reference.cjs gate.json out.json jgthms/bulma=PATH vuetifyjs/vuetify=PATH');
}
const roots = new Map();
for (const arg of subjectArgs) {
  const at = arg.indexOf('=');
  const name = arg.slice(0, at);
  if (at < 1 || !FROZEN.has(name) || roots.has(name) || !arg.slice(at + 1)) {
    throw new Error(`unknown or duplicate subject: ${arg}`);
  }
  roots.set(name, path.resolve(arg.slice(at + 1)));
}
const gate = JSON.parse(fs.readFileSync(gatePath, 'utf8'));
if (gate.files !== 219 || gate.subjects.length !== FROZEN.size) {
  throw new Error('gate JSON does not cover the frozen 219-file corpus');
}
// The pinned package exports only its parser entry, not ./package.json.
const parserRoot = path.resolve(path.dirname(require.resolve('sass-parser')), '..', '..');
const metadata = JSON.parse(fs.readFileSync(path.join(parserRoot, 'package.json'), 'utf8'));
if (metadata.name !== 'sass-parser') throw new Error('unexpected parser package root');
const version = metadata.version;
if (version !== '0.4.57') throw new Error(`sass-parser ${version}; expected 0.4.57`);
const results = [];
for (const subject of gate.subjects) {
  const pinned = FROZEN.get(subject.name);
  if (!pinned || subject.sha !== pinned.sha || subject.scope !== pinned.scope || subject.files !== pinned.files) {
    throw new Error(`gate subject identity mismatch: ${subject.name}`);
  }
  const root = roots.get(subject.name);
  const sha = cp.execFileSync('git', ['-C', root, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
  if (sha !== pinned.sha) throw new Error(`${subject.name}: commit ${sha}; expected ${pinned.sha}`);
  if (cp.execFileSync('git', ['-C', root, 'status', '--porcelain'], { encoding: 'utf8' }).trim()) {
    throw new Error(`${subject.name}: dirty subject checkout`);
  }
  const files = cp.execFileSync('git', ['-C', root, 'ls-files', '-z', '--', '*.sass'], { encoding: 'utf8' })
    .split('\0').filter(p => p && (pinned.scope === '.' || p.startsWith(pinned.scope))).sort();
  if (files.length !== pinned.files) throw new Error(`${subject.name}: ${files.length} files; expected ${pinned.files}`);
  const bad = new Set(subject.errors.map(x => x.path));
  const rejected = [];
  let accepted = 0;
  let grammarErrors = 0;
  for (const rel of files) {
    try {
      sp.sass.parse(fs.readFileSync(path.join(root, rel), 'utf8'), { from: rel });
      accepted++;
      if (bad.has(rel)) grammarErrors++;
    } catch (error) {
      rejected.push({ path: rel, message: error.message.split('\n')[0] });
    }
  }
  results.push({ name: subject.name, files: files.length, reference_accepted: accepted,
    reference_rejected: rejected, grammar_errors_on_reference_accepted: grammarErrors });
}
fs.mkdirSync(path.dirname(path.resolve(outPath)), { recursive: true });
fs.writeFileSync(outPath, JSON.stringify({ tool: 'sass-parser', version, results }, null, 2) + '\n');
console.log(JSON.stringify(results));
