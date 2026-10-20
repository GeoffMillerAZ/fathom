#!/usr/bin/env node
// solc_ast_dump.js -- tier-A Solidity oracle (LB-Q-solidity, wave lang6; see README.md).
//
// Each .sol file under --scope is compiled alone with its import closure by solc's --standard-json
// (ast output only, no code generation). The compiler's AST is the truth: definitions are the declarations
// the dialect names; a call is a FunctionCall (internal or external function call, `emit` of an event, `revert`
// of a custom error) or a ModifierInvocation whose callee carries a positive `referencedDeclaration` that is a
// function, modifier, event or error declared in the file's closure. A type conversion, a struct constructor, a
// builtin (negative id) and a call through a variable are no pair. A file solc rejects (an error-severity
// diagnostic, a pragma the binary does not satisfy, an import it cannot read) has no AST and is excluded, counted
// on stderr and listed in --excluded-out.
//
// usage: node solc_ast_dump.js --solc <solc> --root <base path> [--scope <dir under root>] [--rel-to <dir>]
//        [--remap prefix=target]... [--allow <dir>]... [--jobs 4] [--excluded-out file] > out.json
'use strict';
const fs = require('fs');
const path = require('path');
const cp = require('child_process');

const args = process.argv.slice(2);
const opt = { solc: 'solc', root: '.', scope: '', relTo: '', remap: [], allow: [], jobs: 4, excludedOut: '' };
for (let i = 0; i < args.length; i++) {
  const a = args[i];
  if (a === '--solc') opt.solc = args[++i];
  else if (a === '--root') opt.root = args[++i];
  else if (a === '--scope') opt.scope = args[++i];
  else if (a === '--rel-to') opt.relTo = args[++i];
  else if (a === '--remap') opt.remap.push(args[++i]);
  else if (a === '--allow') opt.allow.push(path.resolve(args[++i]));
  else if (a === '--jobs') opt.jobs = Number(args[++i]);
  else if (a === '--excluded-out') opt.excludedOut = args[++i];
  else throw new Error('unknown argument ' + a);
}
const root = path.resolve(opt.root);
const scope = path.resolve(root, opt.scope);
const relTo = path.resolve(opt.relTo || scope);
const skipDirs = new Set(['.git', 'node_modules', 'lib', 'out', 'cache', 'artifacts', 'broadcast']);

const files = [];
(function walk(dir) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    if (skipDirs.has(ent.name)) continue;
    const p = path.join(dir, ent.name);
    if (ent.isDirectory()) walk(p);
    else if (ent.isFile() && ent.name.endsWith('.sol')) files.push(p);
  }
})(scope);

const version = (cp.spawnSync(opt.solc, ['--version'], { encoding: 'utf8' }).stdout || '').match(/Version: (\S+)/);

function compile(abs) {
  return new Promise((resolve) => {
    const rel = path.relative(root, abs).split(path.sep).join('/');
    const input = {
      language: 'Solidity',
      sources: { [rel]: { content: fs.readFileSync(abs, 'utf8') } },
      settings: { remappings: opt.remap, outputSelection: { '*': { '': ['ast'] } } },
    };
    const allow = [root, ...opt.allow].join(',');
    const child = cp.spawn(opt.solc, ['--standard-json', '--base-path', root, '--allow-paths', allow], { stdio: ['pipe', 'pipe', 'pipe'] });
    const out = [];
    child.stdout.on('data', (d) => out.push(d));
    const timer = setTimeout(() => child.kill('SIGKILL'), 180000);
    child.on('close', () => {
      clearTimeout(timer);
      try {
        resolve({ rel, abs, res: JSON.parse(Buffer.concat(out).toString('utf8')) });
      } catch (e) {
        resolve({ rel, abs, res: { errors: [{ severity: 'error', message: 'unreadable compiler output' }] } });
      }
    });
    child.stdin.end(JSON.stringify(input));
  });
}

const sourceCache = new Map();
function lineIndex(file) {
  let e = sourceCache.get(file);
  if (!e) {
    const buf = fs.readFileSync(path.resolve(root, file));
    const starts = [0];
    for (let i = 0; i < buf.length; i++) if (buf[i] === 10) starts.push(i + 1);
    e = starts;
    sourceCache.set(file, e);
  }
  return e;
}
function lineOf(file, off) {
  const starts = lineIndex(file);
  let lo = 0, hi = starts.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (starts[mid] <= off) lo = mid;
    else hi = mid - 1;
  }
  return lo + 1;
}
function range(file, src) {
  const [s, l] = src.split(':').map(Number);
  return [lineOf(file, s), lineOf(file, Math.max(s, s + l - 1))];
}

const CALLEE_TYPES = new Set(['FunctionDefinition', 'ModifierDefinition', 'EventDefinition', 'ErrorDefinition']);

function walk(node, fn, parents = []) {
  if (Array.isArray(node)) {
    for (const n of node) walk(n, fn, parents);
    return;
  }
  if (!node || typeof node !== 'object') return;
  if (typeof node.nodeType === 'string') {
    fn(node, parents);
    parents = parents.concat(node);
  }
  for (const k of Object.keys(node)) {
    const v = node[k];
    if (v && typeof v === 'object') walk(v, fn, parents);
  }
}

function defKind(node, parent) {
  switch (node.nodeType) {
    case 'ContractDefinition':
      return { kind: node.contractKind === 'interface' ? 'interface' : node.contractKind === 'library' ? 'library' : 'contract', name: node.name };
    case 'StructDefinition': return { kind: 'struct', name: node.name };
    case 'EnumDefinition': return { kind: 'enum', name: node.name };
    case 'UserDefinedValueTypeDefinition': return { kind: 'type', name: node.name };
    case 'ErrorDefinition': return { kind: 'error', name: node.name };
    case 'EventDefinition': return { kind: 'event', name: node.name };
    case 'ModifierDefinition': return { kind: 'modifier', name: node.name };
    case 'FunctionDefinition': {
      if (node.kind === 'constructor' || node.kind === 'receive' || node.kind === 'fallback') return { kind: 'constructor', name: node.kind };
      if (!node.name) return null;
      return { kind: parent && parent.nodeType === 'ContractDefinition' ? 'method' : 'function', name: node.name };
    }
    case 'VariableDeclaration':
      if (parent && parent.nodeType === 'ContractDefinition' && node.stateVariable) return { kind: 'variable', name: node.name };
      if (parent && parent.nodeType === 'SourceUnit' && node.constant) return { kind: 'constant', name: node.name };
      return null;
    default: return null;
  }
}

async function main() {
  const queue = files.slice();
  const results = [];
  await Promise.all(Array.from({ length: opt.jobs }, async () => {
    while (queue.length) results.push(await compile(queue.shift()));
  }));
  results.sort((a, b) => (a.rel < b.rel ? -1 : 1));

  const definitions = [];
  const calls = [];
  const excluded = [];
  let compiled = 0;
  for (const { rel, abs, res } of results) {
    const relOut = path.relative(relTo, abs).split(path.sep).join('/');
    const errs = (res.errors || []).filter((e) => e.severity === 'error');
    if (errs.length || !res.sources || !res.sources[rel]) {
      excluded.push({ path: relOut, reason: (errs[0] && (errs[0].formattedMessage || errs[0].message) || 'no ast').split('\n')[0].slice(0, 160) });
      continue;
    }
    compiled++;
    const decl = new Map();
    for (const f of Object.keys(res.sources)) {
      walk(res.sources[f].ast, (n) => {
        if (typeof n.id === 'number' && n.nodeType) decl.set(n.id, n);
      });
    }
    const ast = res.sources[rel].ast;
    const seen = new Set();
    walk(ast, (node, parents) => {
      const parent = parents[parents.length - 1];
      const d = defKind(node, parent);
      if (!d || !d.name) return;
      const [line, end] = range(rel, node.src);
      definitions.push({ path: relOut, name: d.name, line, end, kind: d.kind });
    });
    // Calls: owned by the nearest enclosing function-like definition (a constructor, receive and fallback are named).
    walk(ast, (node, parents) => {
      let calleeId, nodeKindOK = false, expr;
      if (node.nodeType === 'FunctionCall') {
        if (node.kind !== 'functionCall') return;
        expr = node.expression;
        while (expr && expr.nodeType === 'FunctionCallOptions') expr = expr.expression;
        if (!expr || (expr.nodeType !== 'Identifier' && expr.nodeType !== 'MemberAccess')) return;
        calleeId = expr.referencedDeclaration;
        nodeKindOK = true;
      } else if (node.nodeType === 'ModifierInvocation') {
        calleeId = node.modifierName && node.modifierName.referencedDeclaration;
        nodeKindOK = true;
      }
      if (!nodeKindOK || typeof calleeId !== 'number' || calleeId <= 0) return;
      const target = decl.get(calleeId);
      if (!target || !CALLEE_TYPES.has(target.nodeType) || !target.name) return;
      let owner = null;
      for (let i = parents.length - 1; i >= 0; i--) {
        const p = parents[i];
        if (p.nodeType === 'FunctionDefinition' || p.nodeType === 'ModifierDefinition') {
          const pk = defKind(p, parents[i - 1]);
          if (pk) { owner = pk.name; break; }
        }
      }
      if (!owner) return;
      const key = owner + '\u0000' + target.name;
      if (seen.has(key)) return;
      seen.add(key);
      calls.push({ path: relOut, from: owner, to: target.name });
    });
  }
  process.stderr.write(`solidity: ${files.length} files, ${compiled} compiled, ${excluded.length} excluded; ${definitions.length} definitions, ${calls.length} call pairs\n`);
  if (opt.excludedOut) fs.writeFileSync(opt.excludedOut, JSON.stringify(excluded, null, 1) + '\n');
  process.stdout.write(JSON.stringify({ tool: 'solc', version: version ? version[1] : 'unknown', tier: 'A', definitions, calls }, null, 1) + '\n');
}
main();
