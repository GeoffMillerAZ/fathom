// tailwind_oracle.cjs ROOT [DIR...] — the Tailwind CSS v4 oracle of LB-Q-tailwind: the tailwindcss v4 compiler's
// own design system (`__unstable__loadDesignSystem`, tier A for what it attests) says which theme variables,
// custom utilities and custom variants a stylesheet defines and which custom utility an `@apply` candidate
// resolves to, and this script writes the neutral oracle file `indexbench lang-oracle -tool-oracle` reads:
//
//   definitions  a theme variable (kind theme: each `--name` of an `@theme` block the compiler registers), a custom
//                utility (kind utility: `@utility name`, or `@utility name-*` named `name-`, as the dialect names a
//                functional one) and a custom variant (kind variant: `@custom-variant name`, and the v4.0
//                `@variant name (selector);` spelling), each one the compiler confirms in the design system built from
//                the file and its `@import` closure;
//   calls        (owner, name) pairs: an `@apply` candidate the compiler parses to a custom utility of the file's
//                closure (a variant prefix and `!important` allowed; the pair names the utility's definition), and a
//                `var(--name)` whose name the compiler registered as a theme variable;
//   imports      every `@import` statement, resolved by this script's resolver to a file of the subject (the same
//                resolver the compiler's loadStylesheet is given).
//
// What is the compiler's and what is the source's. EXISTENCE and RESOLUTION are the compiler's: a definition or a
// pair is emitted only when the design system built from the file agrees. NAMES and LINES are the source's, as the
// lightningcss oracle's are: the tailwindcss package exports no parser, so this script reads the file with its own
// small scanner (comments, strings, brackets and braces) to place each name, and counts every candidate the compiler
// rejects. No end line is written (the compiler's AST is not exported either), so range_agreement is not measured and
// L1 is capped at good.
//
// Owners are the dialect's own rule (internal/extractor/dialects/css/tags.scm): a declaration of a style rule to the
// rule's first selector, a custom property's value to the property, an @utility body to the utility, an @keyframes
// rule's blocks to the rule, anything else to the file's module symbol `<module>`.
//
// Offline: unresolved imports and plugins are stubbed (an empty stylesheet, a plugin that does nothing) and counted.
// Utility classes used in templates (the oxide scanner's candidates) are out of scope for this oracle.
//
//     npm install --prefix <toolcache>/tailwind tailwindcss@4.3.2 @tailwindcss/node@4.3.2 @tailwindcss/oxide@4.3.2
//     NODE_PATH=<toolcache>/tailwind/node_modules node tailwind_oracle.cjs <clone> [dir...] > out.json
'use strict';
const fs = require('fs');
const path = require('path');
const tw = require('tailwindcss');
const TWNM = path.resolve(path.dirname(require.resolve('tailwindcss')), '..', '..');
const version = JSON.parse(fs.readFileSync(path.join(TWNM, 'tailwindcss', 'package.json'), 'utf8')).version;

const stats = {};
const bump = (k, n = 1) => { stats[k] = (stats[k] || 0) + n; };

function listCSS(root, dirs) {
  const out = [];
  const walk = (rel) => {
    const abs = path.join(root, rel);
    if (fs.statSync(abs).isFile()) { if (rel.endsWith('.css')) out.push(rel); return; }
    for (const e of fs.readdirSync(abs, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
      if (e.name === '.git' || e.name === 'node_modules') continue;
      const r = rel ? rel + '/' + e.name : e.name;
      if (e.isDirectory()) walk(r);
      else if (e.isFile() && e.name.endsWith('.css')) out.push(r);
    }
  };
  for (const d of dirs.length ? dirs : ['']) walk(d.replace(/\/+$/, ''));
  return out.sort();
}

// ---- the scanner -----------------------------------------------------------------------------------------------

function lineStarts(src) {
  const s = [0];
  for (let i = 0; i < src.length; i++) if (src.charCodeAt(i) === 10) s.push(i + 1);
  return s;
}
function lineOf(starts, off) {
  let lo = 0, hi = starts.length - 1;
  while (lo < hi) { const m = (lo + hi + 1) >> 1; if (starts[m] <= off) lo = m; else hi = m - 1; }
  return lo + 1;
}

// parse reads src into nested nodes: {kind: 'rule'|'at'|'decl', prelude, line, body: nodes|null}. A prelude runs to the
// first top-level `;`, `{` or `}`; `{` opens a block, the others end a statement.
function parse(src) {
  const n = src.length;
  const starts = lineStarts(src);
  let i = 0;
  const skipTrivia = () => {
    for (;;) {
      while (i < n && /\s/.test(src[i])) i++;
      if (src[i] === '/' && src[i + 1] === '*') { const e = src.indexOf('*/', i + 2); i = e < 0 ? n : e + 2; } else return;
    }
  };
  const list = (top) => {
    const nodes = [];
    for (;;) {
      skipTrivia();
      if (i >= n) return nodes;
      if (src[i] === '}') { if (top) { i++; continue; } i++; return nodes; }
      if (src[i] === ';') { i++; continue; }
      const start = i;
      let depth = 0, term = '';
      let text = '';
      while (i < n) {
        const c = src[i];
        if (c === '/' && src[i + 1] === '*') { const e = src.indexOf('*/', i + 2); text += ' '; i = e < 0 ? n : e + 2; continue; }
        if (c === '"' || c === "'") {
          let j = i + 1;
          while (j < n && src[j] !== c && src[j] !== '\n') { if (src[j] === '\\') j++; j++; }
          text += src.slice(i, j + 1); i = j + 1; continue;
        }
        if (c === '(' || c === '[') depth++;
        else if (c === ')' || c === ']') depth = Math.max(0, depth - 1);
        else if (depth === 0 && (c === ';' || c === '{' || c === '}')) { term = c; break; }
        text += c; i++;
      }
      const prelude = text.trim();
      const line = lineOf(starts, start);
      if (term === '{') {
        i++;
        const body = list(false);
        nodes.push({ kind: prelude.startsWith('@') ? 'at' : 'rule', prelude, line, body });
      } else {
        if (term === ';') i++; // a `}` stays for the caller
        if (prelude === '') continue;
        if (prelude.startsWith('@')) nodes.push({ kind: 'at', prelude, line, body: null });
        else nodes.push({ kind: 'decl', prelude, line, body: null });
      }
    }
  };
  return list(true);
}

function atName(prelude) {
  const m = /^@([A-Za-z-]+)/.exec(prelude);
  return m ? m[1] : '';
}
function atParams(prelude) { return prelude.replace(/^@[A-Za-z-]+/, '').trim(); }

// splitTop splits s at top-level occurrences of sep (outside strings, brackets and parentheses).
function splitTop(s, sep) {
  const out = [];
  let depth = 0, cur = '';
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (c === '"' || c === "'") {
      let j = i + 1;
      while (j < s.length && s[j] !== c) { if (s[j] === '\\') j++; j++; }
      cur += s.slice(i, j + 1); i = j; continue;
    }
    if (c === '(' || c === '[') depth++;
    else if (c === ')' || c === ']') depth--;
    if (depth === 0 && (sep === ' ' ? /\s/.test(c) : c === sep)) { out.push(cur); cur = ''; continue; }
    cur += c;
  }
  out.push(cur);
  return out.map((x) => x.trim()).filter((x) => x !== '');
}

// ---- resolution ------------------------------------------------------------------------------------------------

const EMPTY = { path: '', base: '', content: '' };

// resolve names the file an @import or @plugin id loads, or null: a relative id against the importing directory, a
// package id against the tailwindcss install (the one package the compiler's CSS resolver has offline).
function resolve(id, baseDir) {
  if (/^([a-z]+:)?\/\//i.test(id) || id.startsWith('data:')) return null;
  const cands = [];
  if (id.startsWith('.') || path.isAbsolute(id)) {
    const abs = path.resolve(baseDir, id);
    cands.push(abs, abs + '.css');
  } else {
    const abs = path.join(TWNM, id);
    cands.push(abs, abs + '.css', path.join(abs, 'index.css'));
    if (id === 'tailwindcss') cands.unshift(path.join(TWNM, 'tailwindcss', 'index.css'));
  }
  for (const c of cands) {
    try { if (fs.statSync(c).isFile()) return c; } catch (e) { /* next */ }
  }
  return null;
}

// ---- one file --------------------------------------------------------------------------------------------------

const scanCache = new Map();
function scanFile(abs) {
  if (!scanCache.has(abs)) {
    let text = '';
    try { text = fs.readFileSync(abs, 'utf8'); } catch (e) { /* unreadable */ }
    scanCache.set(abs, { text, tree: parse(text) });
  }
  return scanCache.get(abs);
}

// utilityDefs are the @utility names of a tree, as {name (the dialect's), root, functional}.
function utilityDefs(nodes, out = []) {
  for (const nd of nodes) {
    if (nd.kind === 'at' && atName(nd.prelude) === 'utility' && nd.body) {
      const p = atParams(nd.prelude);
      if (/^[A-Za-z0-9_-]+(-\*)?$/.test(p)) {
        const functional = p.endsWith('-*');
        const root = functional ? p.slice(0, -2) : p;
        out.push({ name: functional ? root + '-' : root, root, functional, line: nd.line });
      }
    }
    if (nd.body) utilityDefs(nd.body, out);
  }
  return out;
}

function variantDefs(nodes, out = []) {
  for (const nd of nodes) {
    if (nd.kind === 'at') {
      const k = atName(nd.prelude);
      const p = atParams(nd.prelude);
      const m = /^([A-Za-z0-9_-]+)/.exec(p);
      if (m && (k === 'custom-variant' || (k === 'variant' && !nd.body && /^[A-Za-z0-9_-]+\s*\(/.test(p)))) {
        out.push({ name: m[1], line: nd.line });
      }
    }
    if (nd.body) variantDefs(nd.body, out);
  }
  return out;
}

function themeDefs(nodes, inTheme = false, out = []) {
  for (const nd of nodes) {
    if (nd.kind === 'at' && atName(nd.prelude) === 'theme' && nd.body) {
      for (const d of nd.body) {
        if (d.kind !== 'decl') continue;
        const m = /^(--[^\s:]+)\s*:/.exec(d.prelude);
        if (!m) continue;
        if (m[1].includes('*')) { bump('theme_namespace_resets'); continue; }
        out.push({ name: m[1], line: d.line });
      }
    } else if (nd.body && nd.kind !== 'decl') themeDefs(nd.body, false, out);
  }
  return out;
}

// refs walks a tree with the current owner and reports each @apply candidate and var() name as (owner, token).
function refs(nodes, owner, inKeyframes, emit) {
  for (const nd of nodes) {
    if (nd.kind === 'decl') {
      const m = /^([^:]+):([\s\S]*)$/.exec(nd.prelude);
      if (!m) continue;
      const prop = m[1].trim();
      const o = prop.startsWith('--') ? prop : owner;
      const re = /var\(\s*(--[^\s,)]+)/g;
      let v;
      while ((v = re.exec(m[2])) !== null) emit('var', o, v[1]);
    } else if (nd.kind === 'at') {
      const k = atName(nd.prelude);
      const p = atParams(nd.prelude);
      if (k === 'apply' && !nd.body) {
        for (const tok of splitTop(p, ' ')) if (tok !== '!important') emit('apply', owner, tok);
      } else if (nd.body) {
        let o = owner, kf = inKeyframes;
        if (k === 'utility') {
          const mm = /^([A-Za-z0-9_-]+?)(-\*)?$/.exec(p);
          if (mm) o = mm[2] ? mm[1] + '-' : mm[1];
        } else if (/^(-[a-z]+-)?keyframes$/.test(k)) { o = p; kf = true; }
        refs(nd.body, o, kf, emit);
      }
    } else if (nd.body) {
      if (inKeyframes) refs(nd.body, owner, inKeyframes, emit);
      else refs(nd.body, splitTop(nd.prelude, ',')[0] || owner, false, emit);
    }
  }
}

function importsOf(nodes, out = []) {
  for (const nd of nodes) {
    if (nd.kind === 'at' && atName(nd.prelude) === 'import' && !nd.body) {
      const m = /^@import\s+(?:url\(\s*)?(["'])(.*?)\1/.exec(nd.prelude) || /^@import\s+url\(\s*([^"')\s]+)/.exec(nd.prelude);
      if (m) out.push({ id: m[2] !== undefined ? m[2] : m[1], line: nd.line });
    }
  }
  return out;
}

// withoutApply replaces every `@apply ...;` statement with an inert declaration (its lines kept): the design system is asked about each
// candidate one by one, and a candidate the file's own partial context cannot resolve (a core utility in a sheet an
// entry sheet imports into Tailwind) would otherwise refuse the whole file.
const withoutApply = (t) => t.replace(/@apply\b[^;}]*;?/g, (m) => '--tw-oracle:0;' + m.replace(/[^\n]/g, ''));

async function designSystem(abs, text, closure) {
  const loadStylesheet = async (id, base) => {
    const r = resolve(id, base);
    if (!r) { bump('imports_stubbed'); return { path: '', base, content: '' }; }
    closure.add(r);
    return { path: r, base: path.dirname(r), content: withoutApply(fs.readFileSync(r, 'utf8')) };
  };
  const loadModule = async (id, base) => {
    bump('modules_stubbed');
    return { path: '', base, module: () => {} };
  };
  // A sheet an entry sheet imports into Tailwind is compiled in that entry's context: when the file does not import
  // tailwindcss itself, the compiler is given the import first (same line count, the import on a line of its own).
  let css = withoutApply(text);
  if (!/@import\s+(?:url\()?["']tailwindcss(?:["'/])/.test(css)) { css = '@import "tailwindcss";' + css; bump('core_import_prepended'); }
  return tw.__unstable__loadDesignSystem(css, { base: path.dirname(abs), from: abs, loadStylesheet, loadModule });
}

async function main() {
  const [root0, ...dirs] = process.argv.slice(2);
  if (!root0) { console.error('usage: tailwind_oracle.cjs ROOT [DIR...]'); process.exit(2); }
  const root = path.resolve(root0);
  const rels = listCSS(root, dirs);
  const definitions = [], calls = [], imports = [], disclose = [];
  const seen = new Set();
  const addDef = (d) => { const k = d.path + '\0' + d.kind + '\0' + d.name + '\0' + d.line; if (!seen.has(k)) { seen.add(k); definitions.push(d); } };
  const pairs = new Set();
  const inSubject = (abs) => abs.startsWith(root + path.sep);
  const relOf = (abs) => path.relative(root, abs).split(path.sep).join('/');

  for (const rel of rels) {
    const abs = path.join(root, rel);
    const { text, tree } = scanFile(abs);
    bump('files');
    for (const im of importsOf(tree)) {
      const r = resolve(im.id, path.dirname(abs));
      imports.push({ path: rel, line: im.line, to_path: r && inSubject(r) ? relOf(r) : '' });
    }
    const themes = themeDefs(tree), utils = utilityDefs(tree), variants = variantDefs(tree);
    const rs = [];
    refs(tree, '<module>', false, (kind, owner, tok) => rs.push({ kind, owner, tok }));
    const applies = rs.filter((r) => r.kind === 'apply'), vars = rs.filter((r) => r.kind === 'var');
    if (!themes.length && !utils.length && !variants.length && !applies.length && !vars.length) continue;
    const closure = new Set([abs]);
    let ds;
    try { ds = await designSystem(abs, text, closure); } catch (e) {
      bump('files_without_design_system');
      disclose.push(`${rel}: the compiler refused the file (${String(e.message).split('\n')[0].slice(0, 120)}); nothing of it is attested`);
      continue;
    }
    bump('files_compiled');
    const themeKeys = new Set([...ds.theme.entries()].map((e) => e[0]));
    for (const t of themes) {
      if (themeKeys.has(t.name)) addDef({ path: rel, name: t.name, line: t.line, end: 0, kind: 'theme' });
      else bump('theme_not_registered');
    }
    for (const u of utils) {
      if (ds.utilities.has(u.root, u.functional ? 'functional' : 'static')) addDef({ path: rel, name: u.name, line: u.line, end: 0, kind: 'utility' });
      else bump('utility_not_registered');
    }
    for (const v of variants) {
      if (ds.variants.has(v.name)) addDef({ path: rel, name: v.name, line: v.line, end: 0, kind: 'variant' });
      else bump('variant_not_registered');
    }
    // The custom utilities of the file's closure, as the compiler registered them.
    const custom = new Map(); // `${functional}:${root}` -> the dialect's name
    for (const c of closure) {
      if (!inSubject(c)) continue;
      for (const u of utilityDefs(scanFile(c).tree)) {
        if (ds.utilities.has(u.root, u.functional ? 'functional' : 'static')) custom.set((u.functional ? 'f:' : 's:') + u.root, u.name);
      }
    }
    for (const a of applies) {
      let css;
      try { css = ds.candidatesToCss([a.tok])[0]; } catch (e) { css = null; }
      if (css === null || css === undefined) { bump('apply_candidates_rejected'); if (process.env.TW_ORACLE_DEBUG) console.error('rejected', rel, a.tok); continue; }
      let hit = '';
      for (const c of ds.parseCandidate(a.tok)) {
        if (c.kind === 'static' && custom.has('s:' + c.root)) hit = custom.get('s:' + c.root);
        else if (c.kind === 'functional' && custom.has('f:' + c.root)) { hit = custom.get('f:' + c.root); bump('apply_pairs_functional_utility'); }
        if (hit) break;
      }
      if (hit) pairs.add(JSON.stringify({ path: rel, from: a.owner, to: hit }));
      else bump('apply_candidates_core');
    }
    for (const v of vars) {
      if (themeKeys.has(v.tok)) pairs.add(JSON.stringify({ path: rel, from: v.owner, to: v.tok }));
      else bump('var_not_theme');
    }
  }
  for (const p of [...pairs].sort()) calls.push(JSON.parse(p));
  definitions.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line || (a.name < b.name ? -1 : 1)));
  imports.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line));
  const note = Object.keys(stats).sort().map((k) => `${k} ${stats[k]}`).join(', ');
  disclose.unshift(`the compiler (tailwindcss ${version}) could not load ${stats.imports_stubbed || 0} @import and ${stats.modules_stubbed || 0} @plugin or @config offline and read each as empty; oracle counts: ${note}`);
  process.stdout.write(JSON.stringify({ tool: 'tailwindcss', version, tier: 'A', licence: 'MIT', definitions, calls, imports, disclose }, null, 1) + '\n');
  console.error(note);
}

main().catch((e) => { console.error(e); process.exit(1); });
