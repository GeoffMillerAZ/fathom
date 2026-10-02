#!/usr/bin/env node
// cdk_oracle.js - the language-breadth harness's oracle for the aws-cdk
// doc-engine dialect (design section 1.3, 13.4 step 19; LB-Q-aws-cdk).
//
//   node cdk_oracle.js -subject <checkout> -cdk <node_modules/aws-cdk> \
//     -yaml <node_modules/yaml> -scratch <dir> -out <defs file> \
//     -refs-out <refs file> [-files-out <file>] [-summary-out <file>]
//
// Definitions, tier A (-out): the AWS CDK CLI (aws-cdk, Apache-2.0) itself,
// installed with npm under the scratch directory and run offline in a scratch
// copy of each cdk.json's directory (HOME there too, so no user setting or cache
// is read): `cdk context --json -vv` loads the file the way every cdk command
// does and logs the configuration it read ("cdk.json: {...}"), and prints the
// context. It runs no app and synthesizes nothing. A file the CLI refuses is
// counted and left out. Each entry of what the CLI read is a definition, at its
// key's line by the `yaml` package (ISC, which reads JSON with source ranges):
// `app` (a `task`), each `context` key (a `key`; a feature flag is one named
// `@aws-cdk...`), each `watch` include and exclude glob (a `selector`) and
// every other top-level setting (a `key`).
//
// References, tier C (-refs-out): the program the app command runs, read from
// the command (the first argument naming a .ts, .js, .mjs, .cjs, .py, .go,
// .csproj or .fsproj file) when that file is in the subject's tree (`git
// ls-tree`, so a sparse checkout still answers), from `app` to the path as
// written. The CLI never names it: only running the app would.
//
// Directories the connector never reads are not walked. Bench-only.

'use strict';

const fs = require('fs');
const path = require('path');
const childProcess = require('child_process');

const args = {};
for (let i = 2; i < process.argv.length; i += 2) args[process.argv[i].replace(/^-+/, '')] = process.argv[i + 1];
const YAML = require(args.yaml);
const SKIP_DIRS = new Set(['vendor', '.git', 'testdata', 'generated', 'node_modules', '.idea', '.vscode', '.claude', 'cdk.out']);
const ENTRY = /(?:^|[\s&;|])((?:\.{0,2}\/)?[A-Za-z0-9_][A-Za-z0-9_./-]*\.(?:ts|js|mjs|cjs|py|go|csproj|fsproj))(?=\s|$|[;&|])/;


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
      } else if (e.name === 'cdk.json') out.push(path.relative(root, path.join(dir, e.name)).split(path.sep).join('/'));
    }
  };
  walk(root);
  return out;
}

// the configuration the CLI logs it read: the JSON object after "cdk.json: "
function loggedConfig(stderr) {
  const i = stderr.indexOf('cdk.json: {');
  if (i < 0) return null;
  let depth = 0;
  let inStr = false;
  const start = stderr.indexOf('{', i);
  for (let j = start; j < stderr.length; j++) {
    const c = stderr[j];
    if (inStr) {
      if (c === '\\') j++;
      else if (c === '"') inStr = false;
      continue;
    }
    if (c === '"') inStr = true;
    else if (c === '{') depth++;
    else if (c === '}' && --depth === 0) {
      try {
        return JSON.parse(stderr.slice(start, j + 1));
      } catch (err) {
        return null;
      }
    }
  }
  return null;
}

function main() {
  const subject = path.resolve(args.subject);
  const scratch = path.resolve(args.scratch);
  fs.mkdirSync(scratch, { recursive: true });
  const cdkBin = path.join(args.cdk, 'bin', 'cdk');
  const version = require(path.join(args.cdk, 'package.json')).version;
  const tree = new Set(childProcess.execFileSync('git', ['-C', subject, 'ls-tree', '-r', '--name-only', 'HEAD'], { encoding: 'utf8', maxBuffer: 1 << 28 }).split('\n'));
  const files = retryEperm(() => discover(subject));
  const defs = [];
  const refs = [];
  const accepted = [];
  const counts = {};
  const count = (k) => (counts[k] = (counts[k] || 0) + 1);
  const failures = [];
  files.forEach((rel, n) => {
    const text = retryEperm(() => fs.readFileSync(path.join(subject, rel), 'utf8'));
    const work = path.join(scratch, 'work', String(n));
    retryEperm(() => fs.rmSync(work, { recursive: true, force: true }));
    retryEperm(() => fs.mkdirSync(work, { recursive: true }));
    retryEperm(() => fs.writeFileSync(path.join(work, 'cdk.json'), text));
    const env = Object.assign({}, process.env, { HOME: work, CDK_DISABLE_VERSION_CHECK: '1', CDK_DISABLE_CLI_TELEMETRY: '1', AWS_ACCESS_KEY_ID: '', AWS_SECRET_ACCESS_KEY: '', AWS_PROFILE: '' });
    let r;
    for (let attempt = 0; attempt < 8; attempt++) {
      r = childProcess.spawnSync(process.execPath, [cdkBin, 'context', '--json', '-vv', '--no-notices'], { cwd: work, env, encoding: 'utf8', timeout: 120000 });
      if (!/EPERM|operation not permitted/i.test(String(r.stderr))) break;
      if (args['on-eperm']) childProcess.spawnSync('/bin/sh', ['-c', args['on-eperm']]);
    }
    retryEperm(() => fs.rmSync(work, { recursive: true, force: true }));
    const read = r.status === 0 ? loggedConfig(String(r.stderr)) : null;
    if (!read) {
      count('cli_refused');
      failures.push([rel, String(r.stderr || '').split('\n').filter((l) => /rror/.test(l))[0] || `exit ${r.status}`]);
      return;
    }
    accepted.push(rel);
    const lc = new YAML.LineCounter();
    const doc = YAML.parseDocument(text, { lineCounter: lc, strict: false, uniqueKeys: false });
    const line = (off) => lc.linePos(off).line;
    const root = doc.contents;
    if (!YAML.isMap(root)) return;
    const add = (name, keyNode, valNode, kind) => {
      const st = line(keyNode.range[0]);
      const endOff = valNode && valNode.range ? valNode.range[1] : keyNode.range[1];
      defs.push({ path: rel, name, line: st, end: Math.max(st, line(Math.max(keyNode.range[0], endOff - 1))), kind });
    };
    for (const p of root.items) {
      const k = p.key && p.key.value;
      if (typeof k !== 'string' || !(k in read)) {
        count('entry_unattested');
        continue;
      }
      if (k === 'app') {
        if (YAML.isScalar(p.value) && typeof p.value.value === 'string') {
          add('app', p.key, p.value, 'task');
          const m = ENTRY.exec(p.value.value);
          if (m) {
            const target = path.posix.normalize(path.posix.join(path.posix.dirname(rel), m[1]));
            if (tree.has(target)) refs.push({ path: rel, from: 'app', to: m[1] });
            else count('entry_not_in_tree');
          } else count('entry_unnamed');
        }
      } else if (k === 'context' && YAML.isMap(p.value)) {
        for (const c of p.value.items) {
          if (c.key && typeof c.key.value === 'string' && read.context && c.key.value in read.context) add(c.key.value, c.key, c.value, 'key');
        }
      } else if (k === 'watch' && YAML.isMap(p.value)) {
        for (const w of p.value.items) {
          const wk = w.key && w.key.value;
          if ((wk !== 'include' && wk !== 'exclude') || !YAML.isSeq(w.value)) continue;
          w.value.items.forEach((g, i) => {
            if (YAML.isScalar(g) && typeof g.value === 'string' && read.watch && Array.isArray(read.watch[wk]) && read.watch[wk][i] === g.value) add(g.value, g, g, 'selector');
          });
        }
      } else add(k, p.key, p.value, 'key');
    }
  });
  defs.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line || (a.name < b.name ? -1 : 1)));
  fs.writeFileSync(args.out, JSON.stringify({ tool: 'cdk context -vv', version, tier: 'A', definitions: defs }, null, 1));
  fs.writeFileSync(args['refs-out'], JSON.stringify({ tool: 'cdk app command reference parse', version, tier: 'C', definitions: [], references: refs }, null, 1));
  if (args['files-out']) fs.writeFileSync(args['files-out'], accepted.sort().join('\n') + (accepted.length ? '\n' : ''));
  const summary = { version, files: files.length, accepted: accepted.length, counts, definitions: defs.length, references: refs.length, failures: failures.length, failure_examples: failures.slice(0, 30) };
  if (args['summary-out']) fs.writeFileSync(args['summary-out'], JSON.stringify(summary, null, 1));
  const { failure_examples, ...brief } = summary;
  process.stderr.write(JSON.stringify(brief) + '\n');
}

main();
