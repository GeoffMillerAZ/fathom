#!/usr/bin/env node
// pulumi_yaml_oracle.js - the language-breadth harness's oracle for the
// pulumi-yaml doc-engine dialect (design section 1.3, 13.4 step 19;
// LB-Q-pulumi-yaml).
//
//   node pulumi_yaml_oracle.js -subject <checkout> -yaml <node_modules/yaml> \
//     -out <file> [-files-out <file>] [-summary-out <file>]
//
// Tier C, a reference parse: the tool's own evaluation is out of reach offline
// (`pulumi convert --from yaml --language pcl` type-checks every resource
// against its provider's schema, so a program converts to nothing without the
// provider plugins, hundreds of megabytes each; `pulumi preview` needs a
// backend and credentials). The `yaml` package (ISC, an independent YAML 1.2
// reader with source ranges) reads every Pulumi.yaml, Pulumi.yml and
// Pulumi.<stack>.yaml of the subject, and the adapter applies Pulumi YAML's
// documented program model:
//
//   - definitions: a project file's project (a `package`, named by `name`, when
//     it has `runtime`), each `config` key (`variable_decl`; a stack file's too),
//     each `variables` entry (`variable`), each `resources` entry that has a
//     `type` (`resource`) and each `outputs` entry (`output`), at its key's line;
//   - references: an interpolation `${name}`, `${name.prop}`, `${name[0]}` in
//     any string (or key) of a resource, a variable or an output (`$${` is a
//     literal), from that entity to the resource, variable or config key `name`
//     names (the builtin `pulumi` and an undeclared name are not references).
//
// Directories the connector never reads (testdata, vendor, node_modules,
// generated, .git and the editor directories) are not walked. Bench-only.

'use strict';

const fs = require('fs');
const path = require('path');

const args = {};
for (let i = 2; i < process.argv.length; i += 2) args[process.argv[i].replace(/^-+/, '')] = process.argv[i + 1];
const YAML = require(args.yaml);

const SKIP_DIRS = new Set(['vendor', '.git', 'testdata', 'generated', 'node_modules', '.idea', '.vscode', '.claude']);
const PROJECT = /^Pulumi\.ya?ml$/;
const STACK = /^Pulumi\.[^/]+\.ya?ml$/;


// eperm: a host whose volume access is revoked mid-run refuses reads with
// EPERM; -on-eperm names its repair, run before a retry.
function retryEperm(fn) {
  for (let attempt = 0; ; attempt++) {
    try {
      return fn();
    } catch (err) {
      if (!(err && (err.code === 'EPERM' || err.code === 'EACCES')) || attempt >= 10) throw err;
      if (args['on-eperm']) require('child_process').spawnSync('/bin/sh', ['-c', args['on-eperm']]);
      require('child_process').spawnSync('/bin/sleep', [String(1 + attempt)]);
    }
  }
}

function discover(root) {
  const out = [];
  const walk = (dir) => {
    for (const e of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
      if (e.isDirectory()) {
        if (!SKIP_DIRS.has(e.name)) walk(path.join(dir, e.name));
      } else if (PROJECT.test(e.name) || STACK.test(e.name)) {
        out.push(path.relative(root, path.join(dir, e.name)).split(path.sep).join('/'));
      }
    }
  };
  walk(root);
  return out;
}

// interpolation roots in a string, Pulumi YAML's grammar: `${` a property
// access `}`; `$${` is a literal `${`
function roots(s, out) {
  for (let i = 0; i < s.length; i++) {
    if (s[i] !== '$') continue;
    if (s[i + 1] === '$') {
      i++;
      continue;
    }
    if (s[i + 1] !== '{') continue;
    const m = /^\s*([A-Za-z_][A-Za-z0-9_:-]*)/.exec(s.slice(i + 2));
    if (m) out.push(m[1]);
  }
}

function strings(node, out) {
  if (!node) return;
  if (YAML.isScalar(node)) {
    if (typeof node.value === 'string') out.push(node.value);
  } else if (YAML.isMap(node)) {
    for (const p of node.items) {
      if (p.key && YAML.isScalar(p.key) && typeof p.key.value === 'string') out.push(p.key.value);
      strings(p.value, out);
    }
  } else if (YAML.isSeq(node)) {
    for (const it of node.items) strings(it, out);
  }
}

function main() {
  const subject = path.resolve(args.subject);
  const files = retryEperm(() => discover(subject));
  const defs = [];
  const refs = new Set();
  const accepted = [];
  const counts = {};
  const count = (k) => (counts[k] = (counts[k] || 0) + 1);
  for (const rel of files) {
    const text = retryEperm(() => fs.readFileSync(path.join(subject, rel), 'utf8'));
    const lc = new YAML.LineCounter();
    const doc = YAML.parseDocument(text, { lineCounter: lc, strict: false, uniqueKeys: false });
    if (doc.errors.length) {
      count('yaml_refused');
      continue;
    }
    const root = doc.contents;
    if (!YAML.isMap(root)) {
      count('not_a_mapping');
      continue;
    }
    const line = (off) => lc.linePos(off).line;
    const get = (m, k) => {
      const p = m.items.find((x) => x.key && x.key.value === k);
      return p ? p : null;
    };
    const base = path.posix.basename(rel);
    const project = PROJECT.test(base);
    const ents = [];
    const def = (name, pair, kind) => {
      const start = line(pair.key.range[0]);
      const endOff = pair.value && pair.value.range ? pair.value.range[1] : pair.key.range[1];
      ents.push({ name, kind, node: pair.value, line: start, end: Math.max(start, line(Math.max(pair.key.range[0], endOff - 1))) });
    };
    if (project) {
      const nm = get(root, 'name');
      if (nm && get(root, 'runtime') && YAML.isScalar(nm.value)) {
        const first = root.items[0].key.range[0];
        defs.push({ path: rel, name: String(nm.value.value), line: line(first), end: line(Math.max(first, root.range[1] - 1)), kind: 'package' });
      }
    }
    const section = (key, kind, needType) => {
      const p = get(root, key);
      if (!p || !YAML.isMap(p.value)) return;
      for (const e of p.value.items) {
        if (!e.key || !YAML.isScalar(e.key)) continue;
        if (needType && !(YAML.isMap(e.value) && get(e.value, 'type'))) continue;
        def(String(e.key.value), e, kind);
      }
    };
    section('config', 'variable_decl', false);
    if (project) {
      section('variables', 'variable', false);
      section('resources', 'resource', true);
      section('outputs', 'output', false);
    }
    accepted.push(rel);
    const declared = new Set(ents.filter((e) => ['resource', 'variable', 'variable_decl'].includes(e.kind)).map((e) => e.name));
    for (const e of ents) {
      defs.push({ path: rel, name: e.name, line: e.line, end: e.end, kind: e.kind });
      if (!['resource', 'variable', 'output'].includes(e.kind)) continue;
      const ss = [];
      strings(e.node, ss);
      const rs = [];
      for (const s of ss) roots(s, rs);
      for (const r of rs) {
        if (!declared.has(r)) continue;
        if (r === e.name && e.kind !== 'output') continue; // a declaration naming itself
        refs.add([rel, e.name, r].join('\0'));
      }
    }
  }
  const references = [...refs].sort().map((s) => {
    const [p, f, t] = s.split('\0');
    return { path: p, from: f, to: t };
  });
  defs.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line || (a.name < b.name ? -1 : 1)));
  const version = require(path.join(path.dirname(require.resolve(args.yaml)), '..', 'package.json')).version;
  fs.writeFileSync(args.out, JSON.stringify({ tool: 'pulumi-yaml reference parse (yaml ' + version + ')', version: '1', tier: 'C', definitions: defs, references }, null, 1));
  if (args['files-out']) fs.writeFileSync(args['files-out'], accepted.sort().join('\n') + (accepted.length ? '\n' : ''));
  const summary = { files: files.length, accepted: accepted.length, counts, definitions: defs.length, references: references.length };
  if (args['summary-out']) fs.writeFileSync(args['summary-out'], JSON.stringify(summary, null, 1));
  process.stderr.write(JSON.stringify(summary) + '\n');
}

main();
