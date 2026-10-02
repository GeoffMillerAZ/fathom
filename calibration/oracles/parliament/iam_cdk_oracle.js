#!/usr/bin/env node
// iam_cdk_oracle.js - the language-breadth harness's tier-A oracle for the
// aws-iam-policy doc-engine dialect (design section 1.3; LB-Q-aws-iam-policy-rest).
//
//   node iam_cdk_oracle.js -subject <checkout> -modules <node_modules> -out <file> \
//     [-files-out <file>] [-summary-out <file>]
//
// <node_modules> holds aws-cdk-lib (2.272.0 measured, Apache-2.0) and
// jsonc-parser (3.3.1, MIT), installed in a scratch directory, never vendored.
//
// What decides (tier A): AWS's own policy model, aws-cdk-lib's aws_iam, the code
// AWS maintains for building and reading IAM policy documents, run offline:
//
//   - a document is a policy when it is a JSON object with a Statement and a
//     Version of 2012-10-17 or 2008-10-17 (the dialect's claim), and it is an
//     entity (`policy`, named by the file, at its root object's line) when
//     PolicyDocument.fromJson accepts it; IAM also accepts a Statement written
//     as one object, which fromJson refuses ("Statement must be an array"), so
//     such a document is accepted when its one statement is;
//   - each statement is an entity (`rule`, named by the Sid the parsed
//     PolicyStatement carries, else its position from 0, at the statement
//     object's line) when PolicyStatement.fromJson accepts it (its field
//     types, its action grammar, at least one Action or NotAction);
//   - references: every value of the parsed statement's actions, notActions,
//     resources, notResources, principals and notPrincipals (each principal's
//     JSON, one list per principal type), as written.
//
// Lines come from jsonc-parser (an independent JSON reader with offsets); the
// values come from JSON.parse, the ECMAScript parser aws-cdk-lib is handed.
// A refused document or statement yields nothing and is counted on stderr.
// Directories the connector never reads are not walked. Bench-only.

'use strict';

const fs = require('fs');
const path = require('path');

const args = {};
for (let i = 2; i < process.argv.length; i += 2) args[process.argv[i].replace(/^-+/, '')] = process.argv[i + 1];

const SKIP_DIRS = new Set(['vendor', '.git', 'testdata', 'generated', 'node_modules', '.idea', '.vscode', '.claude']);
const VERSIONS = new Set(['2012-10-17', '2008-10-17']);

function retryEperm(fn) {
  for (let attempt = 0; ; attempt++) {
    try {
      return fn();
    } catch (err) {
      if (!(err && (err.code === 'EPERM' || err.code === 'EACCES')) || attempt >= 10) throw err;
      require('child_process').spawnSync('/bin/sleep', [String(1 + attempt)]);
    }
  }
}

function discover(root) {
  const out = [];
  const walk = (dir) => {
    const entries = retryEperm(() => fs.readdirSync(dir, { withFileTypes: true }));
    for (const e of entries.sort((a, b) => (a.name < b.name ? -1 : 1))) {
      if (e.isDirectory()) {
        if (!SKIP_DIRS.has(e.name)) walk(path.join(dir, e.name));
      } else if (e.name.toLowerCase().endsWith('.json')) {
        out.push(path.relative(root, path.join(dir, e.name)).split(path.sep).join('/'));
      }
    }
  };
  walk(root);
  return out;
}

function lineIndex(text) {
  const starts = [0];
  for (let i = 0; i < text.length; i++) if (text[i] === '\n') starts.push(i + 1);
  return (off) => {
    let lo = 0;
    let hi = starts.length - 1;
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1;
      if (starts[mid] <= off) lo = mid;
      else hi = mid - 1;
    }
    return lo + 1;
  };
}

function principalValues(json, out) {
  if (typeof json === 'string') {
    out.push(json);
  } else if (Array.isArray(json)) {
    for (const v of json) principalValues(v, out);
  } else if (json && typeof json === 'object') {
    for (const k of Object.keys(json)) principalValues(json[k], out);
  }
}

function analyze(subject, iam, jsonc) {
  const defs = [];
  const refs = [];
  const accepted = [];
  const stats = { files: 0, not_json: 0, not_policy: 0, policies: 0, documents_refused: 0, single_object_statement: 0,
    statements: 0, non_object_statements: 0, statements_refused: 0 };
  const refusals = [];
  for (const rel of discover(subject)) {
    stats.files++;
    let text = retryEperm(() => fs.readFileSync(path.join(subject, rel), 'utf8'));
    if (text.charCodeAt(0) === 0xfeff) text = text.slice(1);
    let root;
    try {
      root = JSON.parse(text);
    } catch (e) {
      stats.not_json++;
      continue;
    }
    if (!root || typeof root !== 'object' || Array.isArray(root) || !('Statement' in root) || !VERSIONS.has(root.Version)) {
      stats.not_policy++;
      continue;
    }
    stats.policies++;
    accepted.push(rel);
    const line = lineIndex(text);
    const tree = jsonc.parseTree(text);
    const stmtProp = jsonc.findNodeAtLocation(tree, ['Statement']);
    const single = !Array.isArray(root.Statement);
    const stmts = single ? [root.Statement] : root.Statement;
    const stmtNodes = single ? [stmtProp] : (stmtProp && stmtProp.children) || [];
    if (single) stats.single_object_statement++;
    let docOK = true;
    if (!single) {
      try {
        iam.PolicyDocument.fromJson(root);
      } catch (e) {
        docOK = false;
        stats.documents_refused++;
        refusals.push(rel + ': ' + String(e.message).split('\n')[0]);
      }
    }
    const rules = [];
    let anyOK = false;
    stmts.forEach((s, i) => {
      if (!s || typeof s !== 'object' || Array.isArray(s)) {
        stats.non_object_statements++;
        return;
      }
      stats.statements++;
      let st;
      try {
        st = iam.PolicyStatement.fromJson(s);
      } catch (e) {
        stats.statements_refused++;
        refusals.push(rel + ' statement ' + i + ': ' + String(e.message).split('\n')[0]);
        return;
      }
      anyOK = true;
      const name = st.sid ? String(st.sid) : String(i);
      const node = stmtNodes[i];
      rules.push({ path: rel, name, line: node ? line(node.offset) : 0, kind: 'rule' });
      const vals = new Set([...st.actions, ...st.notActions, ...st.resources, ...st.notResources].filter((v) => typeof v === 'string'));
      for (const p of [...st.principals, ...st.notPrincipals]) {
        const out = [];
        principalValues(p.policyFragment && p.policyFragment.principalJson, out);
        for (const v of out) vals.add(v);
      }
      for (const v of [...vals].sort()) refs.push({ path: rel, from: name, to: v });
    });
    if (single ? anyOK : docOK) {
      defs.push({ path: rel, name: path.posix.basename(rel), line: line(tree.offset), kind: 'policy' });
    }
    defs.push(...rules);
  }
  defs.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0)));
  return { defs, refs, accepted, stats, refusals };
}

function main() {
  const mods = path.resolve(args.modules);
  const iam = require(path.join(mods, 'aws-cdk-lib', 'aws-iam'));
  const jsonc = require(path.join(mods, 'jsonc-parser'));
  const cdkVersion = require(path.join(mods, 'aws-cdk-lib', 'package.json')).version;
  const jsoncVersion = require(path.join(mods, 'jsonc-parser', 'package.json')).version;
  const r = analyze(path.resolve(args.subject), iam, jsonc);
  fs.writeFileSync(args.out, JSON.stringify({
    tool: 'aws-cdk-lib aws_iam PolicyDocument.fromJson and PolicyStatement.fromJson (AWS) + jsonc-parser for lines',
    version: 'aws-cdk-lib ' + cdkVersion + ', jsonc-parser ' + jsoncVersion,
    tier: 'A', definitions: r.defs, references: r.refs,
  }, null, 1) + '\n');
  if (args['files-out']) fs.writeFileSync(args['files-out'], r.accepted.join('\n') + (r.accepted.length ? '\n' : ''));
  if (args['summary-out']) fs.writeFileSync(args['summary-out'], JSON.stringify({ stats: r.stats, refusals: r.refusals }, null, 1) + '\n');
  process.stderr.write('iam-cdk-oracle: ' + Object.entries(r.stats).map(([k, v]) => k + ' ' + v).join(', ') + '\n');
  for (const x of r.refusals.slice(0, 20)) process.stderr.write('  refused ' + x + '\n');
  if (r.refusals.length > 20) process.stderr.write('  and ' + (r.refusals.length - 20) + ' more\n');
}

module.exports = { analyze };
if (require.main === module) main();
