#!/usr/bin/env node
// sls_oracle.js - the language-breadth harness's oracle for the serverless
// doc-engine dialect (design section 1.3, 13.4 step 19; LB-Q-serverless).
//
//   node sls_oracle.js -subject <checkout> -sls <node_modules/serverless> \
//     -yaml <node_modules/yaml> -scratch <dir> -out <file> [-files-out <file>] \
//     [-summary-out <file>]
//
// Tier A: the Serverless Framework v3 itself (MIT), installed with npm under the
// scratch directory, offline: no login, no deployment, no credentials, its
// telemetry and notifications off. For each serverless.yml and serverless.yaml of
// the subject:
//
//   - the framework's own reader (lib/configuration/read: its YAML loader, with
//     CloudFormation's short-form tags) reads the file as written;
//   - `serverless print --format json` evaluates it in a scratch copy of the
//     service's directory (its variables resolved: ${self:}, ${opt:} with stage
//     dev, ${file()}, ${sls:}); the copy leaves out the `frameworkVersion` pin
//     (most examples pin v1 or v2) and the `plugins` list (the plugins are not
//     installed; print does not need them). A file the tool refuses (a
//     non-AWS provider whose plugin is absent, a ${ssm:}, ${cf:} or ${aws:}
//     source that needs credentials, a syntax error) is counted and left out;
//   - the framework's variable parser (lib/configuration/variables/resolve-meta)
//     lists, for every property of the file as written, each variable and its
//     sources.
//
// An independent YAML reader (the `yaml` package, ISC) gives each entity's lines.
//
//   - definitions: the service (a `package`, named by `service` or
//     `service.name`), `provider` (a `section`) and each of its keys (`key`),
//     each function (`function`), each event (a `route` for http, httpApi and
//     websocket, named "METHOD path" or as written, a `rule` for any other, named
//     by its type), each resources.Resources entry (`resource`) and
//     resources.Outputs entry (`output`), each custom key (`variable_decl`) and
//     each plugin (`import`), every one the evaluated configuration holds at the
//     same place (a plugin: the reader's list);
//   - references: a `self:custom.X` or `self:provider.X` source the parser finds
//     in a property, from the innermost entity holding the property to custom
//     key X or provider key X; and a CloudFormation Ref, Fn::GetAtt (and a
//     resource's DependsOn) naming a resources entry, from every entity whose
//     value holds it.
//
// Bench-only; never a product dependency.

'use strict';

const fs = require('fs');
const path = require('path');
const childProcess = require('child_process');

const args = {};
for (let i = 2; i < process.argv.length; i += 2) args[process.argv[i].replace(/^-+/, '')] = process.argv[i + 1];
const SLS = args.sls;
const YAML = require(args.yaml);
const readConfiguration = require(path.join(SLS, 'lib/configuration/read'));
const resolveMeta = require(path.join(SLS, 'lib/configuration/variables/resolve-meta'));

const SKIP_DIRS = new Set(['vendor', '.git', 'testdata', 'generated', 'node_modules', '.idea', '.vscode', '.claude']);
const ROUTES = new Set(['http', 'httpApi', 'websocket']);

function discover(root) {
  const out = [];
  const walk = (dir) => {
    for (const e of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
      if (e.isDirectory()) {
        if (!SKIP_DIRS.has(e.name)) walk(path.join(dir, e.name));
      } else if (e.name === 'serverless.yml' || e.name === 'serverless.yaml') {
        out.push(path.relative(root, path.join(dir, e.name)).split(path.sep).join('/'));
      }
    }
  };
  walk(root);
  return out;
}

const isMap = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);

// lines: the yaml package's node for a path (keys and indices), its key's line
// and its value's last line.
function locator(text) {
  const lc = new YAML.LineCounter();
  const doc = YAML.parseDocument(text, { lineCounter: lc, strict: false, uniqueKeys: false, prettyErrors: false });
  const line = (off) => lc.linePos(off).line;
  const find = (pathKeys) => {
    let node = doc.contents;
    let keyNode = null;
    for (const k of pathKeys) {
      if (!node) return null;
      if (YAML.isMap(node)) {
        const pair = node.items.find((p) => p.key && String(p.key.value !== undefined ? p.key.value : p.key) === String(k));
        if (!pair) return null;
        keyNode = pair.key;
        node = pair.value;
      } else if (YAML.isSeq(node)) {
        node = node.items[Number(k)];
        keyNode = null;
      } else return null;
    }
    const start = keyNode && keyNode.range ? keyNode.range[0] : node && node.range ? node.range[0] : null;
    if (start === null) return null;
    const endOff = node && node.range ? node.range[1] : start;
    return { line: line(start), end: Math.max(line(start), line(Math.max(start, endOff - 1))) };
  };
  return find;
}

// a scalar is named as written (`http: true`, an Azure plugin's marker, is
// "true"), as the dialect reads every scalar
const scalar = (v) => (v === null || v === undefined || typeof v === 'object' ? null : String(v));

function eventName(type, v) {
  if (ROUTES.has(type)) {
    if (type === 'websocket') return isMap(v) ? v.route : scalar(v);
    if (isMap(v)) {
      if (typeof v.method !== 'string' || typeof v.path !== 'string') return null;
      return `${v.method.toUpperCase()} ${v.path}`;
    }
    return scalar(v);
  }
  return type;
}

// entities of the raw configuration: [{kind, name, path(keys), ...}]
function entities(raw) {
  const out = [];
  if (!isMap(raw)) return out;
  const svc = typeof raw.service === 'string' ? raw.service : isMap(raw.service) && typeof raw.service.name === 'string' ? raw.service.name : null;
  if (svc) out.push({ kind: 'package', name: svc, path: [] });
  if (isMap(raw.provider)) {
    out.push({ kind: 'section', name: 'provider', path: ['provider'] });
    for (const k of Object.keys(raw.provider)) out.push({ kind: 'key', name: k, path: ['provider', k] });
  }
  const fnGroups = Array.isArray(raw.functions) ? raw.functions.map((g, i) => [g, [i]]) : [[raw.functions, []]];
  for (const [group, prefix] of fnGroups) {
    if (!isMap(group)) continue;
    for (const [fname, fn] of Object.entries(group)) {
      if (!isMap(fn)) continue;
      const fpath = ['functions', ...prefix, fname];
      out.push({ kind: 'function', name: fname, path: fpath, fn: fname });
      if (!Array.isArray(fn.events)) continue;
      fn.events.forEach((ev, i) => {
        if (!isMap(ev)) return;
        for (const [type, v] of Object.entries(ev)) {
          const name = eventName(type, v);
          if (name === null || name === undefined || name === '') continue;
          out.push({ kind: ROUTES.has(type) ? 'route' : 'rule', name, path: [...fpath, 'events', i, type], fn: fname, index: i, type });
        }
      });
    }
  }
  const resGroups = Array.isArray(raw.resources) ? raw.resources.map((g, i) => [g, [i]]) : [[raw.resources, []]];
  for (const [group, prefix] of resGroups) {
    if (!isMap(group)) continue;
    for (const sect of ['Resources', 'Outputs']) {
      if (!isMap(group[sect])) continue;
      for (const [k, v] of Object.entries(group[sect])) {
        if (isMap(v)) out.push({ kind: sect === 'Resources' ? 'resource' : 'output', name: k, path: ['resources', ...prefix, sect, k] });
      }
    }
  }
  if (isMap(raw.custom)) for (const k of Object.keys(raw.custom)) out.push({ kind: 'variable_decl', name: k, path: ['custom', k] });
  const plugins = Array.isArray(raw.plugins) ? raw.plugins.map((p, i) => [p, ['plugins', i]]) : isMap(raw.plugins) && Array.isArray(raw.plugins.modules) ? raw.plugins.modules.map((p, i) => [p, ['plugins', 'modules', i]]) : [];
  for (const [p, pp] of plugins) if (typeof p === 'string') out.push({ kind: 'import', name: p, path: pp, plugin: true });
  return out;
}

// attested: the entity is in the evaluated configuration
function attested(e, printed) {
  if (!isMap(printed)) return false;
  const get = (obj, keys) => keys.reduce((o, k) => (o === null || o === undefined ? o : o[k]), obj);
  switch (e.kind) {
    case 'package':
      return printed.service !== undefined;
    case 'import':
      return true; // the reader's list: print ran without the plugins
    case 'function':
    case 'route':
    case 'rule': {
      const fns = Array.isArray(printed.functions) ? Object.assign({}, ...printed.functions) : printed.functions;
      if (!isMap(fns) || !isMap(fns[e.fn])) return false;
      if (e.kind === 'function') return true;
      const evs = fns[e.fn].events;
      return Array.isArray(evs) && isMap(evs[e.index]) && e.type in evs[e.index];
    }
    case 'resource':
    case 'output': {
      const sect = e.kind === 'resource' ? 'Resources' : 'Outputs';
      const groups = Array.isArray(printed.resources) ? printed.resources : [printed.resources];
      return groups.some((g) => isMap(g) && isMap(g[sect]) && e.name in g[sect]);
    }
    default:
      return get(printed, e.path) !== undefined;
  }
}

function cfnTargets(v, out) {
  if (Array.isArray(v)) {
    v.forEach((x) => cfnTargets(x, out));
  } else if (isMap(v)) {
    for (const [k, x] of Object.entries(v)) {
      if (k === 'Ref' && typeof x === 'string') out.push(x);
      else if (k === 'Fn::GetAtt') {
        if (Array.isArray(x) && typeof x[0] === 'string') out.push(x[0]);
        else if (typeof x === 'string') out.push(x.split('.')[0]);
      }
      cfnTargets(x, out);
    }
  }
}

// eperm: a host whose volume access is revoked mid-run refuses reads with
// EPERM; -on-eperm names its repair, run before a retry.
function retryEperm(fn) {
  for (let attempt = 0; ; attempt++) {
    try {
      return fn();
    } catch (err) {
      if (!(err && (err.code === 'EPERM' || err.code === 'EACCES')) || attempt >= 10) throw err;
      if (args['on-eperm']) childProcess.spawnSync('/bin/sh', ['-c', args['on-eperm']]);
      childProcess.spawnSync('/bin/sleep', [String(1 + attempt)]);
    }
  }
}

async function main() {
  const subject = path.resolve(args.subject);
  const scratch = path.resolve(args.scratch);
  fs.mkdirSync(scratch, { recursive: true });
  const slsBin = path.join(SLS, 'bin/serverless.js');
  const version = require(path.join(SLS, 'package.json')).version;
  const files = retryEperm(() => discover(subject));
  const counts = {};
  const count = (k) => (counts[k] = (counts[k] || 0) + 1);
  const failures = [];
  const defs = [];
  const refs = new Set();
  const accepted = [];
  for (const rel of files) {
    const abs = path.join(subject, rel);
    let raw;
    let readErr = null;
    for (let attempt = 0; attempt < 10; attempt++) {
      try {
        raw = await readConfiguration(abs);
        readErr = null;
        break;
      } catch (err) {
        readErr = err;
        if (!/EPERM|not permitted/.test(String(err.message))) break;
        if (args['on-eperm']) childProcess.spawnSync('/bin/sh', ['-c', args['on-eperm']]);
      }
    }
    if (readErr) {
      count('reader_refused');
      failures.push([rel, 'reader: ' + String(readErr.message).slice(0, 160)]);
      continue;
    }
    // the evaluation, in a scratch copy of the service directory
    const work = path.join(scratch, 'work', String(accepted.length + failures.length));
    retryEperm(() => fs.rmSync(work, { recursive: true, force: true }));
    retryEperm(() => fs.cpSync(path.dirname(abs), work, { recursive: true, filter: (src) => !/[\\/](node_modules|\.git)([\\/]|$)/.test(src) }));
    const cfg = path.join(work, path.basename(rel));
    const lines = retryEperm(() => fs.readFileSync(cfg, 'utf8')).split('\n');
    const kept = [];
    let skipping = false;
    for (const ln of lines) {
      if (skipping) {
        if (/^\s/.test(ln) || ln.trim() === '' || /^-/.test(ln)) continue;
        skipping = false;
      }
      if (/^frameworkVersion\s*:/.test(ln)) continue;
      if (/^plugins\s*:/.test(ln)) {
        skipping = true;
        continue;
      }
      kept.push(ln);
    }
    retryEperm(() => fs.writeFileSync(cfg, kept.join('\n')));
    const env = Object.assign({}, process.env, {
      SLS_TELEMETRY_DISABLED: '1', SLS_NOTIFICATIONS_MODE: 'off', SLS_DEPRECATION_DISABLE: '*', SLS_INTERACTIVE_SETUP_DISABLE: '1',
      HOME: work, AWS_ACCESS_KEY_ID: '', AWS_SECRET_ACCESS_KEY: '', AWS_PROFILE: '', AWS_SDK_LOAD_CONFIG: '',
    });
    const cliArgs = [slsBin, 'print', '--format', 'json', '--stage', 'dev', '--region', 'us-east-1'];
    if (path.basename(rel) !== 'serverless.yml') cliArgs.push('--config', path.basename(rel));
    let r;
    for (let attempt = 0; attempt < 8; attempt++) {
      r = childProcess.spawnSync(process.execPath, cliArgs, { cwd: work, env, encoding: 'utf8', timeout: 180000 });
      if (!/EPERM|operation not permitted/i.test(String(r.stderr) + String(r.stdout))) break;
      if (args['on-eperm']) childProcess.spawnSync('/bin/sh', ['-c', args['on-eperm']]);
    }
    let printed = null;
    if (r.status === 0) {
      try {
        const s = r.stdout;
        printed = JSON.parse(s.slice(s.indexOf('{')));
      } catch (err) {
        printed = null;
      }
    }
    retryEperm(() => fs.rmSync(work, { recursive: true, force: true }));
    if (!printed) {
      count('print_refused');
      const out = String(r.stderr || '') + '\n' + String(r.stdout || '');
      const msg = out.split('\n').map((l) => l.trim()).filter((l) => l && !/^(Environment|Credentials|Docs|Support|Slack|Bugs|Issues|Your Environment)/.test(l)).slice(0, 3).join(' | ') || `exit ${r.status}`;
      failures.push([rel, String(msg).trim().slice(0, 200)]);
      continue;
    }
    accepted.push(rel);
    const find = locator(retryEperm(() => fs.readFileSync(abs, 'utf8')));
    const ents = entities(raw).filter((e) => {
      if (attested(e, printed)) return true;
      count('entity_unattested');
      return false;
    });
    const declared = { variable_decl: new Set(), key: new Set(), resource: new Set() };
    for (const e of ents) {
      const loc = find(e.path);
      if (!loc) {
        count('entity_unlocated');
        continue;
      }
      e.loc = loc;
      defs.push({ path: rel, name: e.name, line: loc.line, end: loc.end, kind: e.kind });
      if (declared[e.kind]) declared[e.kind].add(e.name);
    }
    const located = ents.filter((e) => e.loc);
    // innermost entity holding a property path (service and provider own nothing of their own beyond keys)
    const holder = (keys) => {
      let best = null;
      for (const e of located) {
        if (e.kind === 'import' || e.kind === 'section') continue;
        if (e.path.length <= keys.length && e.path.every((k, i) => String(k) === String(keys[i]))) {
          if (!best || e.path.length > best.path.length) best = e;
        }
      }
      return best;
    };
    // variables (the framework's parser, over the file as written)
    let meta;
    try {
      meta = resolveMeta(raw);
    } catch (err) {
      meta = new Map();
      count('meta_failed');
    }
    for (const [key, m] of meta) {
      if (!m.variables) continue;
      const h = holder(key.split('\0'));
      if (!h) continue;
      const visit = (vars) => {
        for (const v of vars || []) {
          for (const src of v.sources || []) {
            if (src.type === 'self' && src.address && typeof src.address.value === 'string') {
              const [head, target] = src.address.value.split('.');
              if (head === 'custom' && declared.variable_decl.has(target) && !(h.kind === 'variable_decl' && h.name === target)) refs.add([rel, h.name, target].join('\0'));
              if (head === 'provider' && declared.key.has(target) && !(h.kind === 'key' && h.name === target)) refs.add([rel, h.name, target].join('\0'));
            }
            if (src.params) visit(src.params.map((p) => ({ sources: [p] })).concat(src.params.filter((p) => p.variables).map((p) => p.variables).flat()));
            if (src.address && src.address.variables) visit(src.address.variables);
          }
        }
      };
      visit(m.variables);
    }
    // CloudFormation references (subtree: an entity owns those its value holds)
    for (const e of located) {
      if (['package', 'section', 'import'].includes(e.kind)) continue;
      const v = e.path.reduce((o, k) => (o === null || o === undefined ? o : o[k]), raw);
      const ts = [];
      cfnTargets(v, ts);
      if (e.kind === 'resource' && isMap(v) && v.DependsOn !== undefined) {
        for (const d of [].concat(v.DependsOn)) if (typeof d === 'string') ts.push(d);
      }
      for (const t of ts) if (declared.resource.has(t)) refs.add([rel, e.name, t].join('\0'));
    }
  }
  const references = [...refs].sort().map((s) => {
    const [p, f, t] = s.split('\0');
    return { path: p, from: f, to: t };
  });
  defs.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line || (a.name < b.name ? -1 : 1)));
  fs.writeFileSync(args.out, JSON.stringify({ tool: 'serverless print', version, tier: 'A', definitions: defs, references }, null, 1));
  if (args['files-out']) fs.writeFileSync(args['files-out'], accepted.sort().join('\n') + (accepted.length ? '\n' : ''));
  const summary = { version, files: files.length, accepted: accepted.length, counts, definitions: defs.length, references: references.length, failures: failures.length, failure_examples: failures.slice(0, 60) };
  if (args['summary-out']) fs.writeFileSync(args['summary-out'], JSON.stringify(summary, null, 1));
  const { failure_examples, ...brief } = summary;
  process.stderr.write(JSON.stringify(brief, null, 1) + '\n');
}

main().catch((err) => {
  process.stderr.write(String(err.stack || err) + '\n');
  process.exit(1);
});
