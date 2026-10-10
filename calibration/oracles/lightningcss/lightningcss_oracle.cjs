// lightningcss_oracle.cjs ROOT [DIR...] — the CSS oracle of design 1.8 W2 (LB-F461): lightningcss, built on
// Mozilla's cssparser, parses every .css file under ROOT (or under each DIR, ROOT-relative) the way a browser
// parser does, and this script writes the neutral oracle file `indexbench lang-oracle -tool-oracle` reads, tier A
// for what lightningcss attests:
//
//   definitions  every style rule's selectors (one per selector of the list), every @keyframes rule, and every
//                custom property a rule declares (`--x: ...`), kinds selector, keyframes, custom_property;
//   calls        every var() read of a custom property and every animation name, as (owner, name) pairs;
//   imports      every @import lightningcss keeps, resolved to a file of the subject by lightningcss's own bundler.
//
// It attests no cascade: which rule a var() reads at run time depends on the document, so a pair names the
// referenced name, never a definition, and the harness grades it only when that name is defined in the file's
// @import closure (langoracle_css.go, W2). It writes no end line: lightningcss's AST carries a rule's start
// (line, column) only, so range_agreement is not measured and L1 is capped at good (design 1.2).
//
// Positions. lightningcss gives a rule's start, never each selector's or each declaration's. The script reads the
// source from that start: a selector list is cut at its top-level commas (outside strings, comments, parentheses
// and brackets) up to the block's `{`, each selector named by its own text and line, and used only when the cut
// gives exactly as many selectors as lightningcss parsed (else the rule is counted and its selectors left out);
// a custom property is placed at the k-th top-level declaration of its name in the rule's own block, the name and
// the count lightningcss's. The names and lines are the source's; what exists is lightningcss's.
//
// Owners. A reference belongs to the narrowest definition around it, the dialect's own rule (tags.scm): a
// declaration of a style rule to the rule's first selector, a custom property's value to the property, an
// @keyframes rule's declarations to the rule, and anything else (an @font-face or @page descriptor) to the file's
// module symbol, `<module>`.
//
// Bench-only: MPL-2.0, run in the measuring harness only, never vendored or modified (npm --prefix in scratch):
//     npm install --prefix <scratch>/npm lightningcss@1.33.0
//     NODE_PATH=<scratch>/npm/node_modules node lightningcss_oracle.cjs <clone> [dir...] > out/css-<subject>.json
'use strict';
const fs = require('fs');
const path = require('path');
const lcss = require('lightningcss');
const version = JSON.parse(fs.readFileSync(path.join(path.dirname(require.resolve('lightningcss')), '..', 'package.json'), 'utf8')).version;

const OPTS = { errorRecovery: true, drafts: { customMedia: true } };

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

// lineStarts are the UTF-16 offsets at which each line of src begins (lightningcss counts columns in UTF-16
// code units, which is what a JS string index is).
function lineStarts(src) {
  const s = [0];
  for (let i = 0; i < src.length; i++) if (src.charCodeAt(i) === 10) s.push(i + 1);
  return s;
}
function lineOf(starts, off) {
  let lo = 0, hi = starts.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (starts[mid] <= off) lo = mid; else hi = mid - 1;
  }
  return lo + 1;
}

// skipString and skipComment return the offset just past a string or comment starting at i.
function skipString(src, i) {
  const q = src[i];
  for (i++; i < src.length; i++) {
    if (src[i] === '\\') { i++; continue; }
    if (src[i] === q || src[i] === '\n') return i + 1;
  }
  return i;
}
function skipComment(src, i) {
  const end = src.indexOf('*/', i + 2);
  return end < 0 ? src.length : end + 2;
}
function isSpace(c) { return c === ' ' || c === '\t' || c === '\n' || c === '\r' || c === '\f'; }

// trimmed is [a, b) with whitespace and comments dropped from both ends.
function trimmed(src, a, b) {
  for (;;) {
    while (a < b && isSpace(src[a])) a++;
    if (a + 1 < b && src[a] === '/' && src[a + 1] === '*') { a = Math.min(skipComment(src, a), b); continue; }
    break;
  }
  for (;;) {
    while (b > a && isSpace(src[b - 1])) b--;
    if (b - 2 >= a && src[b - 1] === '/' && src[b - 2] === '*') {
      const open = src.lastIndexOf('/*', b - 2);
      if (open >= a) { b = open; continue; }
    }
    break;
  }
  return [a, b];
}

// prelude cuts the selector list starting at off: the selectors (text and offset) and the offset of the block's `{`.
function prelude(src, off) {
  const parts = [];
  let depth = 0, start = off, i = off;
  while (i < src.length) {
    const c = src[i];
    if (c === '"' || c === "'") { i = skipString(src, i); continue; }
    if (c === '/' && src[i + 1] === '*') { i = skipComment(src, i); continue; }
    if (c === '\\') { i += 2; continue; }
    if (c === '(' || c === '[') depth++;
    else if (c === ')' || c === ']') depth--;
    else if (depth === 0 && (c === ',' || c === '{')) {
      parts.push(trimmed(src, start, i));
      if (c === '{') return { parts, open: i };
      start = i + 1;
    }
    i++;
  }
  return null;
}

// blockEnd is the offset of the `}` closing the block opened at open.
function blockEnd(src, open) {
  let depth = 0;
  for (let i = open; i < src.length;) {
    const c = src[i];
    if (c === '"' || c === "'") { i = skipString(src, i); continue; }
    if (c === '/' && src[i + 1] === '*') { i = skipComment(src, i); continue; }
    if (c === '\\') { i += 2; continue; }
    if (c === '{') depth++;
    else if (c === '}' && --depth === 0) return i;
    i++;
  }
  return src.length;
}

// regionEnd is the offset of the first `}` at depth 0 from off: the end of the block off is inside.
function regionEnd(src, off) {
  let depth = 0;
  for (let i = off; i < src.length;) {
    const c = src[i];
    if (c === '"' || c === "'") { i = skipString(src, i); continue; }
    if (c === '/' && src[i + 1] === '*') { i = skipComment(src, i); continue; }
    if (c === '\\') { i += 2; continue; }
    if (c === '{') depth++;
    else if (c === '}') { if (depth === 0) return i; depth--; }
    i++;
  }
  return src.length;
}

// customDecls is every top-level `--name:` declaration of the block [open, close], by name, in source order.
function customDecls(src, open, close) {
  const out = new Map();
  let depth = 0, atStart = true;
  for (let i = open + 1; i < close;) {
    const c = src[i];
    if (c === '/' && src[i + 1] === '*') { i = skipComment(src, i); continue; }
    if (isSpace(c)) { i++; continue; }
    if (depth === 0 && atStart && c === '-' && src[i + 1] === '-') {
      let j = i + 2;
      while (j < close && /[A-Za-z0-9_\-\u0080-￿\\]/.test(src[j])) j += src[j] === '\\' ? 2 : 1;
      let k = j;
      for (;;) {
        while (k < close && isSpace(src[k])) k++;
        if (src[k] === '/' && src[k + 1] === '*') { k = skipComment(src, k); continue; }
        break;
      }
      if (src[k] === ':') {
        const name = src.slice(i, j);
        if (!out.has(name)) out.set(name, []);
        out.get(name).push(i);
      }
    }
    atStart = false;
    if (c === '"' || c === "'") { i = skipString(src, i); continue; }
    if (c === '{') depth++;
    else if (c === '}') { depth--; if (depth === 0) atStart = true; }
    else if (c === ';' && depth === 0) atStart = true;
    i++;
  }
  return out;
}

// varsIn is every var() name in a lightningcss value (a parsed property, an unparsed token list, a custom
// property's tokens): the Variable nodes, {type: "var", value: {name: {ident}}}, a fallback's included.
function varsIn(v, out) {
  if (Array.isArray(v)) { for (const x of v) varsIn(x, out); return out; }
  if (v && typeof v === 'object') {
    if (v.type === 'var' && v.value && v.value.name && typeof v.value.name.ident === 'string') out.push(v.value.name.ident);
    for (const k of Object.keys(v)) varsIn(v[k], out);
  }
  return out;
}

function animationNames(decl, stats) {
  const names = [];
  const add = (n) => { if (n && n.type === 'ident') names.push(n.value); };
  if (decl.property === 'animation-name' && Array.isArray(decl.value)) decl.value.forEach(add);
  else if (decl.property === 'animation' && Array.isArray(decl.value)) decl.value.forEach((a) => add(a && a.name));
  else if (decl.property === 'unparsed' && decl.value && decl.value.propertyId &&
    /^animation(-name)?$/.test(decl.value.propertyId.property)) stats.animation_unparsed++;
  return names;
}

function oracleFile(root, rel, out, stats) {
  const abs = path.join(root, rel);
  const src = fs.readFileSync(abs, 'utf8');
  const starts = lineStarts(src);
  let sheet = null;
  let res;
  try {
    res = lcss.transform({ ...OPTS, filename: rel, code: Buffer.from(src), visitor: { StyleSheet(s) { sheet = s; } } });
  } catch (e) {
    stats.parse_failures.push(rel + ': ' + String(e.message || e).split('\n')[0]);
    return [];
  }
  stats.warnings += (res.warnings || []).length;
  const offsetOf = (loc) => starts[loc.line] + loc.column - 1;
  const def = (name, line, kind) => out.definitions.push({ path: rel, name, line, end: 0, kind });
  const call = (from, to) => out.calls.push({ path: rel, from, to });
  const imports = [];

  const declRefs = (decls, owner) => {
    for (const d of decls) {
      if (d.property === 'custom' && d.value && typeof d.value.name === 'string' && d.value.name.startsWith('--')) {
        for (const n of varsIn(d.value.value, [])) call(d.value.name, n);
        continue;
      }
      for (const n of varsIn(d, [])) call(owner, n);
      for (const n of animationNames(d, stats)) call(owner, n);
    }
  };
  const declsOf = (block) => block ? [...(block.declarations || []), ...(block.importantDeclarations || [])] : [];

  // customDefs places each custom property decls declares at the k-th top-level declaration of its name in the
  // block [open, close].
  const customDefs = (decls, open, close) => {
    const custom = decls.filter((d) => d.property === 'custom' && d.value && typeof d.value.name === 'string' && d.value.name.startsWith('--'));
    if (!custom.length) return;
    const at = customDecls(src, open, close);
    const used = new Map();
    for (const d of custom) {
      const k = used.get(d.value.name) || 0;
      used.set(d.value.name, k + 1);
      const offs = at.get(d.value.name) || [];
      if (k < offs.length) { def(d.value.name, lineOf(starts, offs[k]), 'custom_property'); stats.custom_located++; }
      else stats.custom_unlocated++;
    }
  };

  const style = (rule) => {
    const off = offsetOf(rule.loc);
    const pre = prelude(src, off);
    let owner = null;
    if (pre && pre.parts.length === rule.selectors.length) {
      for (const [a, b] of pre.parts) def(src.slice(a, b), lineOf(starts, a), 'selector');
      owner = src.slice(pre.parts[0][0], pre.parts[0][1]);
    } else {
      stats.selector_cut_mismatch++;
    }
    const decls = declsOf(rule.declarations);
    if (pre) customDefs(decls, pre.open, blockEnd(src, pre.open));
    if (owner !== null) declRefs(decls, owner);
    walk(rule.rules || [], owner);
  };

  const walk = (rules, owner) => {
    for (const r of rules) {
      const v = r.value;
      switch (r.type) {
        case 'style': style(v); break;
        case 'media': case 'supports': case 'container': case 'layer-block': case 'starting-style':
        case 'scope': case 'moz-document':
          walk(v.rules || [], owner); break;
        case 'nested-declarations': {
          // Declarations straight inside a group rule nested in a style rule (`.a { @media (x) { --y: 1 } }`):
          // the run starts at v.loc and ends at its enclosing block's `}`.
          const off = offsetOf(v.loc);
          customDefs(declsOf(v.declarations), off - 1, regionEnd(src, off));
          if (owner !== null) declRefs(declsOf(v.declarations), owner);
          break;
        }
        case 'keyframes': {
          const name = v.name && (v.name.type === 'ident' || v.name.type === 'custom') ? v.name.value : null;
          if (name === null) break;
          def(name, v.loc.line + 1, 'keyframes');
          for (const kf of v.keyframes || []) declRefs(declsOf(kf.declarations), name);
          break;
        }
        case 'import':
          imports.push({ line: v.loc.line + 1, url: v.url });
          break;
        case 'unknown': case 'custom': case 'ignored':
          stats.unknown_at_rules++; break;
        default:
          // @font-face, @page, @counter-style, @property, @viewport, @font-palette-values, @position-try,
          // @view-transition, @namespace, @layer statements, @custom-media: no definition of the dialect's;
          // a var() inside one is its owner's, the file's module symbol at the top level.
          for (const n of varsIn(v, [])) call(owner === null ? '<module>' : owner, n);
      }
    }
  };
  walk((sheet && sheet.rules) || [], null);
  return imports;
}

// importTargets is the file lightningcss's bundler reads for each @import of abs, by its own resolution: a
// bundle of abs alone, every other file read as empty so only abs's own imports are followed.
async function importTargets(abs) {
  const reads = [];
  try {
    await lcss.bundleAsync({
      ...OPTS, filename: abs,
      resolver: {
        read(file) {
          if (file === abs) return fs.readFileSync(abs, 'utf8');
          reads.push(file);
          return '';
        },
      },
    });
  } catch (e) {
    return { reads, error: String(e.message || e).split('\n')[0] };
  }
  return { reads };
}

async function main() {
  const [root, ...dirs] = process.argv.slice(2);
  if (!root) { console.error('usage: lightningcss_oracle.cjs ROOT [DIR...]'); process.exit(2); }
  const files = listCSS(root, dirs);
  const out = {
    tool: 'lightningcss',
    version,
    tier: 'A',
    definitions: [],
    calls: [],
    imports: [],
  };
  const stats = { files: files.length, warnings: 0, unknown_at_rules: 0, selector_cut_mismatch: 0, custom_located: 0,
    custom_unlocated: 0, animation_unparsed: 0, imports: 0, imports_resolved: 0, imports_unread: 0, bundle_errors: 0, parse_failures: [] };
  const rootAbs = path.resolve(root);
  for (const rel of files) {
    const imports = oracleFile(root, rel, out, stats);
    if (!imports.length) continue;
    const abs = path.join(rootAbs, rel);
    const { reads, error } = await importTargets(abs);
    if (error) stats.bundle_errors++;
    for (const im of imports) {
      stats.imports++;
      // Pair a read with its @import by the file it names: lightningcss resolves a relative URL against the
      // importing file's directory, so each read is checked against the URL it came from, never guessed.
      const hit = reads.find((p) => path.resolve(p) === path.resolve(path.dirname(abs), im.url));
      let to = '';
      if (hit && fs.existsSync(hit) && fs.statSync(hit).isFile()) {
        const r = path.relative(rootAbs, hit).split(path.sep).join('/');
        if (!r.startsWith('../')) { to = r; stats.imports_resolved++; }
      } else if (!hit) stats.imports_unread++;
      out.imports.push({ path: rel, line: im.line, to_path: to, module: im.url });
    }
  }
  const byPath = (a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0);
  out.definitions.sort((a, b) => byPath(a, b) || a.line - b.line || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
  out.calls.sort((a, b) => byPath(a, b) || (a.from + '\0' + a.to < b.from + '\0' + b.to ? -1 : 1));
  out.imports.sort((a, b) => byPath(a, b) || a.line - b.line);
  process.stdout.write(JSON.stringify(out, null, 1) + '\n');
  const kinds = {};
  for (const d of out.definitions) kinds[d.kind] = (kinds[d.kind] || 0) + 1;
  console.error(JSON.stringify({ ...stats, definitions: kinds, calls: out.calls.length, parse_failures: stats.parse_failures.slice(0, 5) }));
}

main().catch((e) => { console.error(e); process.exit(1); });
