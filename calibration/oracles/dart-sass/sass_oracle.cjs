// sass_oracle.cjs ROOT FILELIST — the SCSS oracle (LB-F466): Dart Sass, Sass's reference implementation, run in
// the measuring harness only. Two of its parts, each attesting its own family:
//
//   definitions  sass-parser (the sass/dart-sass package that exposes Dart Sass's own parser as a tree): every
//                @mixin, @function, variable declaration ($x: ..., at any depth), @each and @for loop variable,
//                @keyframes rule and custom property declaration, and every class, id and placeholder a style
//                rule's selector list writes, read from Dart Sass's selector parser (pseudo-class arguments
//                included), each with its own line. Names are as written, the sigil dropped; a member name keeps
//                its underscores (Dart Sass normalizes `_` to `-` for lookup, never in the source).
//   imports      every @use, @forward and Sass @import the parser reads, paired with the file Dart Sass loads
//                for it: each file of the list is compiled (`sass.compile`); a compile that succeeds attests the
//                files it loaded (`loadedUrls`), and a statement of a loaded file is paired with the one loaded
//                file its URL can name under Sass's load rule (the name or its `_` partial, with .scss, .sass or
//                .css, or the directory's `_index`/`index` file). A load of a package outside the subject
//                (node_modules) is served an empty stylesheet by a stub importer so the compile goes on; it is
//                no file of the subject. A statement in a file no successful compile loaded, or naming no
//                loaded file (a branch not taken, a package), is counted and written with no `to_path`;
//                `sass:` built-in modules name no file.
//
// A file sass-parser cannot parse (0.4.57 rejects a keyframe selector written as a percentage, `0% { }`, which
// Dart Sass itself compiles) has no definition truth, and a file whose loads no successful compile evaluated has
// no import truth: both are named on stderr and left out of `-files-out`, the files the reading measures
// (`lang-oracle -only`). The tier is A for what each part attests (§1.3: the
// language's own implementation); source.end is exclusive and is converted to an inclusive end line.
//
//   calls        design 1.8 W5 (w5_refs.cjs): the name references of the module graph, walked from the same trees
//                (@include, user function calls, $variable reads, @extend %placeholder), with call_exclusions for the
//                names a file's reads never resolve; graded as L2's call pairs.
//
// Bench-only, MIT, installed into scratch:
//     npm install --prefix <scratch>/npm sass-parser@0.4.57 sass@1.105.1
//     NODE_PATH=<scratch>/npm/node_modules node calibration/oracles/dart-sass/sass_oracle.cjs <subject> <files.txt> \
//         [-files-out <parsed.txt>] > <out>.json
'use strict';
const fs = require('fs');
const path = require('path');
const { pathToFileURL, fileURLToPath } = require('url');
const sp = require('sass-parser');
const sass = require('sass');
const pkgVersion = (name) => JSON.parse(fs.readFileSync(path.join(path.dirname(require.resolve(name)), name === 'sass' ? '' : '../..', 'package.json'), 'utf8')).version;

const args = process.argv.slice(2);
const root = path.resolve(args[0]);
const files = fs.readFileSync(args[1], 'utf8').split('\n').filter(Boolean).sort();
const fo = args.indexOf('-files-out');
const filesOut = fo >= 0 ? args[fo + 1] : '';

const out = { tool: 'dart-sass', version: '', tier: 'A', definitions: [], imports: [] };
const stats = { files: files.length, parse_failures: [], compiled_ok: 0, compile_failed: 0, statements: 0, paired: 0,
  unattested_file: 0, no_loaded_target: 0, ambiguous: 0, builtin: 0, plain_css: 0, definitions: {} };
const statements = []; // {path, line, url, kind}
const parsed = [];
const parsedTrees = new Map(); // rel -> {src, tree}, W5's walk
const w5 = require('./w5_refs.cjs');

// asWritten is a member's name as the source writes it at the node's own offset: Dart Sass's parser hands it
// back normalized (`_` read as `-`, which Sass treats as one identifier), never as written.
const asWritten = (src, node, re, value) => {
  const at = node && node.source && node.source.start ? node.source.start.offset : -1;
  const m = at >= 0 ? re.exec(src.slice(at, at + 400)) : null;
  return m ? m[1] : value;
};
const def = (rel, kind, name, node) => {
  const line = node && node.source && node.source.start ? node.source.start.line : 0;
  const endPosition = node && node.source && node.source.end;
  if (!line || !endPosition || endPosition.offset <= node.source.start.offset) {
    throw new Error(`missing or empty source span for ${rel}: ${kind} ${name}`);
  }
  const end = endPosition.column === 1 ? endPosition.line - 1 : endPosition.line;
  out.definitions.push({ path: rel, name, line, end, kind });
  stats.definitions[kind] = (stats.definitions[kind] || 0) + 1;
};

// The loop node spans its body. Its declared variables are source tokens before the expression.
// Match only the names the AST declares, in order, so comments and later reads cannot become definitions.
function loopVariables(src, node, names, expression) {
  const start = node.source && node.source.start && node.source.start.offset;
  const limit = expression && expression.source && expression.source.start && expression.source.start.offset;
  if (!Number.isInteger(start) || !Number.isInteger(limit) || limit <= start) {
    throw new Error(`missing loop header span at ${node.source && node.source.start && node.source.start.line}`);
  }
  const header = src.slice(start, limit);
  const tokens = [];
  let i = 0;
  let line = node.source.start.line;
  const advance = (to) => {
    for (; i < to; i++) {
      if (header[i] === '\n' || (header[i] === '\r' && header[i + 1] !== '\n')) line++;
    }
  };
  while (i < header.length) {
    if (header.startsWith('/*', i)) {
      const close = header.indexOf('*/', i + 2);
      if (close < 0) throw new Error(`unterminated loop header comment at line ${line}`);
      advance(close + 2);
      continue;
    }
    if (header.startsWith('//', i)) {
      const close = header.slice(i).search(/[\r\n]/);
      advance(close < 0 ? header.length : i + close);
      continue;
    }
    if (header[i] !== '$') { advance(i + 1); continue; }
    const offset = start + i;
    const tokenLine = line;
    let j = i + 1;
    let last = j;
    let written = '';
    let canonical = '';
    while (j < header.length) {
      const ch = header[j];
      if (/[A-Za-z0-9_-]/.test(ch) || ch.charCodeAt(0) >= 0x80) {
        written += ch; canonical += ch; j++; last = j;
      } else if (ch === '\\' && j + 1 < header.length && !/[\r\n\f]/.test(header[j + 1])) {
        const hex = /^[0-9a-fA-F]{1,6}/.exec(header.slice(j + 1));
        if (hex) {
          written += '\\' + hex[0];
          const code = parseInt(hex[0], 16);
          canonical += String.fromCodePoint(code === 0 || code > 0x10ffff || (code >= 0xd800 && code <= 0xdfff) ? 0xfffd : code);
          j += 1 + hex[0].length;
          last = j;
          if (header.slice(j, j + 2) === '\r\n') j += 2;
          else if (/[ \t\r\n\f]/.test(header[j] || '')) j++;
        } else {
          written += header.slice(j, j + 2); canonical += header[j + 1]; j += 2; last = j;
        }
      } else break;
    }
    if (!written) { advance(i + 1); continue; }
    advance(last);
    const endLine = line;
    advance(j);
    tokens.push({ name: written, canonical, source: { start: { line: tokenLine, offset }, end: { line: endLine, column: 2, offset: start + last } } });
  }
  let next = 0;
  return names.map((name) => {
    const wanted = String(name).replace(/_/g, '-');
    while (next < tokens.length && tokens[next].canonical.replace(/_/g, '-') !== wanted) next++;
    if (next === tokens.length) throw new Error(`missing loop variable $${name} at line ${node.source.start.line}`);
    return tokens[next++];
  });
}

function selectorNames(rel, node) {
  let ps;
  try { ps = node.parsedSelector; } catch (e) { return; }
  const visit = (x, depth) => {
    if (!x || typeof x !== 'object' || depth > 40) return;
    if (x.sassType === 'class') def(rel, 'class', String(x.class), x);
    else if (x.sassType === 'id') def(rel, 'id', String(x.id), x);
    else if (x.sassType === 'placeholder') def(rel, 'placeholder', String(x.placeholder !== undefined ? x.placeholder : x.name), x);
    for (const k of ['nodes', 'components', 'compound', 'selectors', 'argument', 'selector']) {
      const v = x[k];
      if (Array.isArray(v)) v.forEach((y) => visit(y, depth + 1));
      else if (v && typeof v === 'object') visit(v, depth + 1);
    }
  };
  visit(ps, 0);
}

for (const rel of files) {
  const src = fs.readFileSync(path.join(root, rel), 'utf8');
  let tree;
  try {
    tree = sp.scss.parse(src, { from: rel });
  } catch (e) {
    stats.parse_failures.push(rel + ': ' + String(e.message).split('\n')[0]);
    continue;
  }
  parsed.push(rel);
  parsedTrees.set(rel, { src, tree });
  tree.walk((n) => {
    switch (n.sassType) {
      case 'mixin-rule': def(rel, 'mixin', asWritten(src, n, /^@mixin\s+([\w-]+)/, n.mixinName), n); break;
      case 'function-rule': def(rel, 'function', asWritten(src, n, /^@function\s+([\w-]+)/, n.functionName), n); break;
      case 'variable-declaration': def(rel, 'variable', asWritten(src, n, /^(?:[\w-]+\.)?\$([\w-]+)/, n.variableName), n); break;
      case 'each-rule': for (const v of loopVariables(src, n, n.variables, n.eachExpression)) def(rel, 'variable', v.name, v); break;
      case 'for-rule': for (const v of loopVariables(src, n, [n.variable], n.fromExpression)) def(rel, 'variable', v.name, v); break;
      case 'rule': selectorNames(rel, n); break;
      case 'decl': {
        const prop = String(n.propInterpolation !== undefined ? n.propInterpolation : n.prop);
        if (prop.startsWith('--')) def(rel, 'custom_property', prop, n);
        break;
      }
      case 'atrule':
        if (/^(-[a-z]+-)?keyframes$/.test(n.name)) def(rel, 'keyframes', String(n.params).trim(), n);
        break;
      case 'use-rule': statements.push({ path: rel, line: n.source.start.line, url: n.useUrl }); break;
      case 'forward-rule': statements.push({ path: rel, line: n.source.start.line, url: n.forwardUrl }); break;
      case 'import-rule':
        for (const im of n.imports.nodes) {
          if (im.sassType === 'dynamic-import') statements.push({ path: rel, line: n.source.start.line, url: im.url });
          else stats.plain_css++;
        }
        break;
    }
  });
}

// Each file compiled alone; a success attests the files it loaded. A load neither the file's directory nor
// Sass's rule finds (a package from node_modules, `@primer/primitives/...`) reaches the stub importer, which
// loads an empty stylesheet, so the in-repo loads around it are still evaluated; the stub is no file of the
// subject and is never a target.
const loadedBy = []; // sets of repo-relative paths, one per successful compile
const silent = { warn() {}, debug() {} };
let stubbed = 0;
const stub = {
  canonicalize(url) { return url.startsWith('sass:') ? null : new URL('stub:' + encodeURIComponent(url)); },
  load() { stubbed++; return { contents: '', syntax: 'scss' }; },
};
for (const rel of files) {
  try {
    const r = sass.compile(path.join(root, rel), { logger: silent, style: 'expanded', importers: [stub] });
    const set = new Set();
    for (const u of r.loadedUrls) {
      if (u.protocol !== 'file:') continue;
      const p = path.relative(root, fileURLToPath(u)).split(path.sep).join('/');
      if (!p.startsWith('../')) set.add(p);
    }
    loadedBy.push(set);
    stats.compiled_ok++;
  } catch (e) {
    stats.compile_failed++;
  }
}

// candidates is every file Sass's load rule lets url name from the importer's directory.
function candidates(importer, url) {
  const base = path.posix.normalize(path.posix.join(path.posix.dirname(importer), url));
  const dir = path.posix.dirname(base), name = path.posix.basename(base);
  const exts = ['.scss', '.sass', '.css'];
  const out = [];
  const known = exts.find((e) => name.endsWith(e));
  if (known) {
    out.push(base, path.posix.join(dir, '_' + name));
  } else {
    for (const e of exts) out.push(base + e, path.posix.join(dir, '_' + name + e));
    for (const e of exts) out.push(path.posix.join(base, '_index' + e), path.posix.join(base, 'index' + e));
  }
  return out;
}

for (const st of statements) {
  stats.statements++;
  const entry = { path: st.path, line: st.line, to_path: '', module: st.url };
  if (st.url.startsWith('sass:')) { stats.builtin++; out.imports.push(entry); continue; }
  const sets = loadedBy.filter((s) => s.has(st.path));
  if (!sets.length) { stats.unattested_file++; out.imports.push(entry); continue; }
  const cands = candidates(st.path, st.url);
  const hits = new Set();
  for (const s of sets) for (const c of cands) if (s.has(c)) hits.add(c);
  if (hits.size === 1) { entry.to_path = [...hits][0]; stats.paired++; }
  else if (hits.size > 1) stats.ambiguous++;
  else stats.no_loaded_target++;
  out.imports.push(entry);
}

// W5 (design 1.8): the name references of the module graph, graded as L2's call pairs.
{
  const exists = new Set(files);
  const r = w5.analyze(parsedTrees, exists, candidates);
  out.calls = r.calls;
  out.call_exclusions = r.call_exclusions;
  stats.w5 = r.stats;
  out.disclose = [
    'W5 (design 1.8): L2 is name references over the @use/@forward/@import closure, walked from sass-parser\'s tree (tier A for the tree, the binding the harness\'s own walk): ' + r.stats.reads + ' references read, ' + r.stats.resolved + ' resolve to a definition in the referring file\'s closure, ' + r.stats.builtin + ' name a sass: built-in module member, ' + r.stats.package + ' name a package outside the subject, ' + r.stats.param + ' read a parameter (no definition), ' + r.stats.unresolved + ' name nothing the closure defines; every name no read of a file resolves leaves both sides, counted',
  ];
}
out.version = 'sass ' + sass.info.split('\n')[0].split('\t')[1] + ', sass-parser ' + pkgVersion('sass-parser');
const byPath = (a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0);
out.definitions.sort((a, b) => byPath(a, b) || a.line - b.line || (a.kind + a.name < b.kind + b.name ? -1 : 1));
out.imports.sort((a, b) => byPath(a, b) || a.line - b.line || (a.module < b.module ? -1 : 1));
// -files-out: the files both parts attest, the reading's -only scope: sass-parser parsed it, and no compile left
// its loads unevaluated (a file whose every compile failed, on a test framework's mixins or a package's
// variables, has statements with no truth).
const unevaluated = new Set(statements.filter((st) => !st.url.startsWith('sass:') && !loadedBy.some((l) => l.has(st.path))).map((st) => st.path));
stats.files_unevaluated = [...unevaluated].sort();
if (filesOut) fs.writeFileSync(filesOut, parsed.filter((f) => !unevaluated.has(f)).join('\n') + '\n');
process.stdout.write(JSON.stringify(out, null, 1) + '\n');
console.error(JSON.stringify({ ...stats, stub_loads: stubbed, parse_failures: stats.parse_failures }));
