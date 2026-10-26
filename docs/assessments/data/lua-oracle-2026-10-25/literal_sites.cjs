#!/usr/bin/env node
// Literal-site scanner derived from the committed lua-require oracle; no lookup or tier claim.
//
// Statements: luaparse (an independent Lua parser, no resolution) finds every call of the global `require` and
// every `pcall(require, <name>)`. A name that is one string literal with no escape is static; a concatenation,
// a variable, an escape sequence or any other argument is dynamic and counted, never resolved (no static reader
// can resolve it, fathom's included).
// Lookup: Lua's own package.searchpath, run by a real Lua (lua_searchpath.lua), over the operator's package.path
// (--roots, in order, each as ?.lua then ?/init.lua) from the subject root. Only a file inside the subject counts.
//
// usage: NODE_PATH=<tools>/luaparse/node_modules node lua_require_dump.js --lua <lua> [--roots ,lua,lib,src] <root> > out.json
'use strict';
const fs = require('fs');
const path = require('path');
const cp = require('child_process');
const luaparse = require('luaparse');

const args = process.argv.slice(2);
let luaBin = 'lua';
let roots = ['', 'lua', 'lib', 'src'];
let root = '.';
for (let i = 0; i < args.length; i++) {
  if (args[i] === '--lua') luaBin = args[++i];
  else if (args[i] === '--roots') roots = args[++i].split(',');
  else root = args[i];
}
root = path.resolve(root);

const helper = path.join(__dirname, 'lua_searchpath.lua');

const files = [];
(function walk(dir) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    if (ent.name === '.git') continue;
    const p = path.join(dir, ent.name);
    if (ent.isDirectory()) walk(p);
    else if (ent.isFile() && ent.name.endsWith('.lua')) files.push(p);
  }
})(root);

const counts = { files: files.length, parse_errors: 0, statements: 0, static: 0, dynamic: 0, escaped: 0, pcall_form: 0 };
const stmts = [];

function literal(n) {
  if (!n || n.type !== 'StringLiteral') return null;
  const raw = n.raw || '';
  if (raw[0] !== '[' && raw.indexOf('\\') >= 0) return { escaped: true };
  const m = /^(?:"(.*)"|'(.*)'|\[(=*)\[([\s\S]*)\]\3\])$/s.exec(raw);
  if (!m) return { escaped: true };
  return { value: m[1] !== undefined ? m[1] : m[2] !== undefined ? m[2] : m[4], line: n.loc.start.line };
}
function isId(n, name) { return n && n.type === 'Identifier' && n.name === name; }

function visit(n, rel) {
  if (!n || typeof n !== 'object') return;
  if (Array.isArray(n)) { for (const c of n) visit(c, rel); return; }
  if (n.type === 'CallExpression' || n.type === 'StringCallExpression' || n.type === 'TableCallExpression') {
    let arg;
    let pcall = false;
    if (isId(n.base, 'require')) {
      arg = n.type === 'CallExpression' ? n.arguments[0] : n.type === 'StringCallExpression' ? n.argument : n.arguments;
    } else if (n.type === 'CallExpression' && isId(n.base, 'pcall') && isId(n.arguments[0], 'require')) {
      arg = n.arguments[1];
      pcall = true;
    }
    if (isId(n.base, 'require') || pcall) {
      counts.statements++;
      const lit = literal(arg);
      if (lit && lit.value !== undefined) {
        counts.static++;
        if (pcall) counts.pcall_form++;
        stmts.push({ path: rel, line: lit.line, module: lit.value });
      } else {
        counts.dynamic++;
        if (lit && lit.escaped) counts.escaped++;
      }
    }
  }
  for (const k of Object.keys(n)) {
    if (k === 'loc' || k === 'range') continue;
    visit(n[k], rel);
  }
}

for (const f of files) {
  const rel = path.relative(root, f).split(path.sep).join('/');
  let ast;
  try {
    ast = luaparse.parse(fs.readFileSync(f, 'utf8'), { luaVersion: 'LuaJIT', locations: true, comments: false });
  } catch (e) {
    counts.parse_errors++;
    continue;
  }
  visit(ast.body, rel);
}

process.stdout.write(JSON.stringify({counts, stmts}) + '\n');
