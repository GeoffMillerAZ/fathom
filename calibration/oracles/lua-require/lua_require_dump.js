#!/usr/bin/env node
// lua_require_dump.js — tier-B Lua import oracle (LB-Q-lua L3, wave lang1; see README.md).
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
const luaVersion = (cp.spawnSync(luaBin, ['-v'], { encoding: 'utf8' }).stdout || 'Lua').trim().split(' ').slice(0, 2).join(' ');
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

const mods = [...new Set(stmts.map((s) => s.module))];
const pathTemplate = roots.map((r) => (r === '' ? '' : r.replace(/\/+$/, '') + '/')).flatMap((r) => [`./${r}?.lua`, `./${r}?/init.lua`]).join(';');
const res = cp.spawnSync(luaBin, [helper, pathTemplate], { cwd: root, input: mods.join('\n') + '\n', encoding: 'utf8', maxBuffer: 1 << 28 });
if (res.status !== 0) { console.error(res.stderr); process.exit(2); }
const found = new Map();
for (const line of res.stdout.split('\n')) {
  const i = line.indexOf('\t');
  if (i > 0) found.set(line.slice(0, i), line.slice(i + 1));
}
const imports = [];
let unresolved = 0;
for (const s of stmts) {
  const to = found.get(s.module);
  if (to) imports.push({ path: s.path, line: s.line, to_path: to, module: s.module });
  else unresolved++;
}
imports.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line || (a.to_path < b.to_path ? -1 : 1)));
counts.in_subject = imports.length;
counts.unresolved = unresolved;
const disclose = [
  `lua-require (tier B): ${counts.statements} require statements in ${counts.files} files (${counts.parse_errors} luaparse rejected): ` +
  `${counts.static} name a string literal (${counts.pcall_form} as pcall(require, name)), of which ${counts.in_subject} resolve to a subject file ` +
  `under package.path "${pathTemplate}" and ${counts.unresolved} name no subject file (an installed rock, a C module, a runtime-provided one); ` +
  `${counts.dynamic} are computed (${counts.escaped} literals with an escape sequence), counted and never resolved; the package.path is the operator's choice`,
];
console.error(Object.entries(counts).map(([k, v]) => `${k} ${v}`).join(' '));
process.stdout.write(JSON.stringify({ tool: 'lua-require', tier: 'B', version: `luaparse 0.3.1 (luaVersion LuaJIT) statements; ${luaVersion} package.searchpath lookup`, disclose, imports }, null, 1) + '\n');
