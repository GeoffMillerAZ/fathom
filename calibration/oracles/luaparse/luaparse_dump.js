#!/usr/bin/env node
// luaparse_dump.js — tier-B Lua L1 oracle (syntax only; see README.md).
//
// Parses every *.lua file under <root> with luaparse (an independent Lua parser, no resolution)
// and writes the neutral oracle file `indexbench lang-oracle -tool-oracle` reads:
//   {"tool","version","tier":"B","definitions":[{path,name,line,end,kind}]}
// Paths are repo-relative with forward slashes, lines 1-based, names bare (the last segment).
//
// A definition is a function stored in a statically named slot:
//   function f() / local function f() / function a.b.c() / function a.b:c()   (name: the last identifier)
//   x = function ... / local x = function ... / a.b = function ...            (name: the variable's last identifier)
//   { k = function ... }                                                       (name: the key)
//   { ["k"] = function ... } / t["k"] = function ...                           (name: the string, unquoted)
// A function stored in a computed slot ({ [expr] = function ... }, t[expr] = function ...) has no
// static name and is not a definition; an anonymous function passed as an argument, returned or
// assigned to nothing is not one either. `local a, b = function..., function...` pairs each variable
// with the value at its position. `line` is the slot's (or `function` keyword's) first line, `end`
// the function's last line.
//
// usage: NODE_PATH=<tools>/npm/node_modules node luaparse_dump.js <root> > out.json
'use strict';
const fs = require('fs');
const path = require('path');
const luaparse = require('luaparse');

const root = path.resolve(process.argv[2] || '.');
const files = [];
(function walk(dir) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    if (ent.name === '.git') continue;
    const p = path.join(dir, ent.name);
    if (ent.isDirectory()) walk(p);
    else if (ent.isFile() && ent.name.endsWith('.lua')) files.push(p);
  }
})(root);

const defs = [];
const forms = {};
let errors = 0;

// bare: the last segment of a name, as the harness compares names (design 1.4): after the last
// of . : # / \ space or ).
function bare(name) {
  const i = Math.max(...['.', ':', '#', '/', '\\', ' ', ')'].map((c) => name.lastIndexOf(c)));
  return name.slice(i + 1);
}
function isFn(n) { return n && n.type === 'FunctionDeclaration'; }
// strKey: a string literal's text as written (escapes not decoded, quotes dropped), or null.
function strKey(n) {
  if (!n || n.type !== 'StringLiteral') return null;
  const raw = String(n.raw || '');
  const q = /^(["'])([\s\S]*)\1$/.exec(raw);
  const s = q ? q[2] : (typeof n.value === 'string' ? n.value : raw);
  return s === '' ? null : s;
}
// slotName: the static name of an assignment target, or null for a computed slot.
function slotName(v) {
  if (!v) return null;
  if (v.type === 'Identifier') return { name: v.name, form: 'variable' };
  if (v.type === 'MemberExpression') return { name: v.identifier.name, form: 'member' };
  if (v.type === 'IndexExpression') { const s = strKey(v.index); return s === null ? null : { name: s, form: 'string-index' }; }
  return null;
}

function walk(node, rel) {
  if (Array.isArray(node)) { for (const c of node) walk(c, rel); return; }
  if (!node || typeof node !== 'object' || !node.type) return;
  const add = (name, line, endNode, form) => {
    defs.push({ path: rel, name: bare(name), line, end: endNode.loc.end.line, kind: 'function' });
    forms[form] = (forms[form] || 0) + 1;
  };
  switch (node.type) {
    case 'FunctionDeclaration':
      if (node.identifier) {
        const id = node.identifier;
        add(id.type === 'MemberExpression' ? id.identifier.name : id.name, node.loc.start.line, node, 'declaration');
      }
      break;
    case 'LocalStatement':
    case 'AssignmentStatement':
      (node.init || []).forEach((val, i) => {
        if (!isFn(val) || val.identifier) return;
        const s = slotName(node.variables[i]);
        if (s) add(s.name, node.variables[i].loc.start.line, val, node.type === 'LocalStatement' ? 'local' : s.form);
        else forms.computed_assignment = (forms.computed_assignment || 0) + 1;
      });
      break;
    case 'TableKeyString':
      if (isFn(node.value)) add(node.key.name, node.loc.start.line, node.value, 'field');
      break;
    case 'TableKey':
      if (isFn(node.value)) {
        const s = strKey(node.key);
        if (s !== null) add(s, node.loc.start.line, node.value, 'string-field');
        else forms.computed_field = (forms.computed_field || 0) + 1;
      }
      break;
    default:
  }
  for (const k of Object.keys(node)) {
    if (k === 'loc' || k === 'range') continue;
    walk(node[k], rel);
  }
}

const failed = [];
for (const p of files) {
  const rel = path.relative(root, p).split(path.sep).join('/');
  let ast;
  try {
    ast = luaparse.parse(fs.readFileSync(p, 'utf8'), { luaVersion: 'LuaJIT', locations: true, comments: false });
  } catch (e) {
    errors++;
    failed.push(rel + ': ' + String(e.message).split('\n')[0]);
    continue;
  }
  walk(ast, rel);
}

defs.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line || (a.name < b.name ? -1 : 1)));
process.stdout.write(JSON.stringify({ tool: 'luaparse', version: require('luaparse/package.json').version + ' (luaVersion LuaJIT)', tier: 'B', definitions: defs }) + '\n');
process.stderr.write(`files=${files.length} parse_errors=${errors} definitions=${defs.length} forms=${JSON.stringify(forms)}\n`);
for (const f of failed.slice(0, 20)) process.stderr.write('  parse error: ' + f + '\n');
