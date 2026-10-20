// w5_refs.cjs — design 1.8 W5 (LB-Q-scss-rest, O851): the name references of Sass's module system, walked over
// sass-parser's tree under W5's scope rules. Required by sass_oracle.cjs.
//
// A pair runs from the referring definition (the innermost @mixin or @function; the file's module symbol
// `<module>` at top level) to the name a reference writes, kept only when the name resolves to a definition
// inside the referring file's @use / @forward / @import closure:
//   @include m, @include ns.m   -> @mixin m
//   f(), ns.f()                 -> a user-defined @function f
//   $v, ns.$v                   -> the $v definition Sass's scoping selects: the innermost enclosing block, then
//                                  the module (a parameter binds first but is no definition: counted, not scored)
//   @extend %p                  -> the %p placeholder defined in the file or any module it loads
// A namespaced member resolves through the `@use ... as ns` that names it, through @forward chains that honour
// `as prefix-*`, `show` and `hide`; private members (`-x`, `_x`) are not visible through a namespace. Built-ins
// (`sass:` modules and plain CSS functions) and names the closure does not define are excluded and counted, never
// scored. The pair's name is the name as the reference writes it (a forwarded prefix kept), the key the dialect
// records; `_` and `-` are one identifier to Sass and are compared normalized, never rewritten.
//
// What the tree does not carry, the walk reads from the node's own text: a name as written at the node's offset,
// the placeholders of an @extend, and the `$name` reads of an at-rule whose prelude the parser leaves as text
// (@media, @supports, @container).
'use strict';
const path = require('path');

const norm = (s) => String(s).replace(/_/g, '-');
const isPrivate = (s) => s.startsWith('-') || s.startsWith('_');
const KINDS = ['mixin', 'function', 'variable'];
const STATEMENT_NODE = /(^|-)rule$|^rule$|^atrule$|^decl$/;
// at-rules whose preludes the parser reads into expressions; every other at-rule's text prelude is scanned.
const PARSED_AT = new Set(['include', 'if', 'else', 'each', 'for', 'while', 'return', 'extend', 'mixin', 'function',
  'use', 'forward', 'import', 'debug', 'warn', 'error', 'content', 'at-root']);

function setOf(x) {
  try { return new Set(x ? [...x] : []); } catch (e) { return new Set(); }
}

// analyze returns W5's pairs, exclusions and counters for the parsed files.
//   files   Map rel -> {src, tree}
//   exists  Set of every file of the subject (the load rule's targets)
//   candidates(importer, url) -> list of paths the load rule lets url name
function analyze(files, exists, candidates) {
  const info = new Map();
  const target = (rel, url) => {
    if (url.startsWith('sass:')) return 'builtin';
    for (const c of candidates(rel, url)) if (exists.has(c)) return c;
    return '';
  };

  // Pass 1: each file's module scope and its loads.
  for (const [rel, { tree }] of files) {
    const fi = { tops: { mixin: new Set(), function: new Set(), variable: new Set(), placeholder: new Set() },
      uses: [], forwards: [], imports: [] };
    const top = (node) => {
      for (const n of node.nodes || []) {
        switch (n.sassType) {
          case 'mixin-rule': fi.tops.mixin.add(norm(n.mixinName)); break;
          case 'function-rule': fi.tops.function.add(norm(n.functionName)); break;
          case 'variable-declaration': fi.tops.variable.add(norm(n.variableName)); break;
          case 'use-rule': fi.uses.push({ url: n.useUrl, ns: n.namespace === undefined || n.namespace === null ? null : n.namespace, to: target(rel, n.useUrl) }); break;
          case 'forward-rule': {
            const show = n.show || {}, hide = n.hide || {};
            fi.forwards.push({ url: n.forwardUrl, to: target(rel, n.forwardUrl), prefix: n.prefix || '',
              show: n.show ? { fn: setOf(show.mixinsAndFunctions), v: setOf(show.variables) } : null,
              hide: n.hide ? { fn: setOf(hide.mixinsAndFunctions), v: setOf(hide.variables) } : null });
            break;
          }
          case 'import-rule':
            for (const im of n.imports.nodes) if (im.sassType === 'dynamic-import') fi.imports.push(target(rel, im.url));
            break;
          default: break;
        }
      }
    };
    top(tree);
    // !global assignments at any depth are module members; placeholders are the selectors' anywhere.
    const deep = (node) => {
      for (const n of node.nodes || []) {
        if (n.sassType === 'variable-declaration' && n.global) fi.tops.variable.add(norm(n.variableName));
        if (n.sassType === 'rule') collectPlaceholders(n, fi.tops.placeholder);
        if (Array.isArray(n.nodes)) deep(n);
      }
    };
    deep(tree);
    info.set(rel, fi);
  }

  function collectPlaceholders(rule, into) {
    let ps;
    try { ps = rule.parsedSelector; } catch (e) { return; }
    const visit = (x, depth) => {
      if (!x || typeof x !== 'object' || depth > 40) return;
      if (x.sassType === 'placeholder') into.add(norm(x.placeholder !== undefined ? x.placeholder : x.name));
      for (const k of ['nodes', 'components', 'compound', 'selectors', 'argument', 'selector']) {
        const v = x[k];
        if (Array.isArray(v)) v.forEach((y) => visit(y, depth + 1));
        else if (v && typeof v === 'object') visit(v, depth + 1);
      }
    };
    visit(ps, 0);
  }

  // Module members. inApi: does module `rel` expose `name` of `kind` to a dependent.
  const apiMemo = new Map();
  function inApi(rel, kind, name, guard = new Set()) {
    const key = rel + '\0' + kind + '\0' + name;
    if (apiMemo.has(key)) return apiMemo.get(key);
    const fi = info.get(rel);
    let ok = false;
    if (fi && !guard.has(rel + '\0' + kind + '\0' + name)) {
      guard.add(rel + '\0' + kind + '\0' + name);
      if (!isPrivate(name) && inClosure(rel, kind, name, guard)) ok = true;
      if (!ok) {
        for (const f of fi.forwards) {
          if (!f.to || f.to === 'builtin') continue;
          let n = name;
          if (f.prefix) { if (!n.startsWith(f.prefix)) continue; n = n.slice(f.prefix.length); }
          const set = kind === 'variable' ? 'v' : 'fn';
          if (f.show && !f.show[set].has(n)) continue;
          if (f.hide && f.hide[set].has(n)) continue;
          if (inApi(f.to, kind, n, guard)) { ok = true; break; }
        }
      }
    }
    apiMemo.set(key, ok);
    return ok;
  }
  // inClosure: the module's own members plus what its @import chain brings into its scope.
  function inClosure(rel, kind, name, guard, seen = new Set()) {
    if (seen.has(rel)) return false;
    seen.add(rel);
    const fi = info.get(rel);
    if (!fi) return false;
    if (fi.tops[kind].has(name)) return true;
    for (const g of fi.imports) if (g && g !== 'builtin' && inClosure(g, kind, name, guard, seen)) return true;
    return false;
  }
  // The names a file's unqualified references can reach: own, `@use as *`, the @import chain (and each imported
  // file's @forward re-exports).
  function inGlobal(rel, kind, name) {
    const fi = info.get(rel);
    if (fi.tops[kind].has(name)) return true;
    for (const u of fi.uses) if (u.ns === null && u.to && u.to !== 'builtin' && inApi(u.to, kind, name)) return true;
    const seen = new Set([rel]);
    const walk = (g) => {
      if (!g || g === 'builtin' || seen.has(g) || !info.has(g)) return false;
      seen.add(g);
      const gi = info.get(g);
      if (gi.tops[kind].has(name)) return true;
      for (const f of gi.forwards) if (f.to && f.to !== 'builtin' && inApi(g, kind, name)) return true;
      return gi.imports.some(walk);
    };
    return fi.imports.some(walk);
  }
  // namespaced: 'builtin' | 'package' | 'missing' | 'resolved'.
  function inNamespace(rel, ns, kind, name) {
    const fi = info.get(rel);
    const u = fi.uses.find((x) => x.ns === ns);
    if (!u) return 'missing';
    if (u.to === 'builtin') return 'builtin';
    if (!u.to) return 'package';
    if (!info.has(u.to)) return 'package';
    return inApi(u.to, kind, name) ? 'resolved' : 'missing';
  }
  function placeholderReachable(rel, name) {
    const seen = new Set();
    const walk = (f) => {
      if (!f || f === 'builtin' || seen.has(f) || !info.has(f)) return false;
      seen.add(f);
      const fi = info.get(f);
      if (fi.tops.placeholder.has(name)) return true;
      return [...fi.uses.map((u) => u.to), ...fi.forwards.map((x) => x.to), ...fi.imports].some(walk);
    };
    return walk(rel);
  }

  // Pass 2: every reference.
  const pairs = [];
  const stats = { reads: 0, resolved: 0, builtin: 0, package: 0, unresolved: 0, param: 0 };
  const perFile = new Map(); // rel -> Map(name -> {resolved, other})
  const note = (rel, name, resolved) => {
    if (!perFile.has(rel)) perFile.set(rel, new Map());
    const m = perFile.get(rel);
    const e = m.get(name) || { resolved: 0, other: 0 };
    if (resolved) e.resolved++; else e.other++;
    m.set(name, e);
  };

  for (const [rel, { src, tree }] of files) {
    const fi = info.get(rel);
    const seen = new WeakSet();
    const record = (owner, name, outcome) => {
      stats.reads++;
      stats[outcome]++;
      note(rel, name, outcome === 'resolved');
      if (outcome === 'resolved') pairs.push({ path: rel, from: owner, to: name });
    };
    const at = (node) => { try { return node.source.start.offset; } catch (e) { return -1; } };
    const written = (node, re, fallback) => {
      const o = at(node);
      const m = o >= 0 ? re.exec(src.slice(o, o + 300)) : null;
      return m ? m : [null, fallback];
    };
    const lookupLocal = (scopes, name) => {
      for (let i = scopes.length - 1; i >= 1; i--) if (scopes[i].has(name)) return scopes[i].get(name);
      return null;
    };
    function readVariable(owner, scopes, ns, name) {
      if (ns) {
        const r = inNamespace(rel, ns, 'variable', norm(name));
        return record(owner, name, r === 'resolved' ? 'resolved' : r === 'builtin' ? 'builtin' : r === 'package' ? 'package' : 'unresolved');
      }
      const local = lookupLocal(scopes, norm(name));
      if (local === 'param') return record(owner, name, 'param');
      if (local) return record(owner, name, 'resolved');
      return record(owner, name, inGlobal(rel, 'variable', norm(name)) ? 'resolved' : 'unresolved');
    }
    function readCallable(owner, kind, ns, name) {
      if (ns) {
        const r = inNamespace(rel, ns, kind, norm(name));
        return record(owner, name, r === 'resolved' ? 'resolved' : r === 'builtin' ? 'builtin' : r === 'package' ? 'package' : 'unresolved');
      }
      return record(owner, name, inGlobal(rel, kind, norm(name)) ? 'resolved' : 'unresolved');
    }
    const kids = (n) => {
      const out = [];
      const nsc = n.nonStatementChildren;
      if (Array.isArray(nsc)) for (const c of nsc) out.push(c);
      return out;
    };

    function expr(n, owner, scopes) {
      if (!n || typeof n !== 'object') return;
      const id = n.proxyOf || n;
      if (seen.has(id)) return;
      seen.add(id);
      switch (n.sassType) {
        case 'variable': {
          const m = written(n, /^(?:([\w-]+)\.)?\$([\w-]+)/, n.variableName);
          readVariable(owner, scopes, n.namespace || null, m[0] === null ? n.variableName : m[2]);
          return;
        }
        case 'function-call': {
          const m = written(n, /^(?:([\w-]+)\.)?([\w-]+)\s*\(/, n.name);
          readCallable(owner, 'function', n.namespace || null, m[0] === null ? n.name : m[2]);
          break;
        }
        default: break;
      }
      for (const c of kids(n)) expr(c, owner, scopes);
    }

    function statements(parent, owner, scopes) {
      for (const n of parent.nodes || []) statement(n, owner, scopes);
    }

    function block(n, owner, scopes, params) {
      const s = new Map();
      for (const p of params || []) s.set(norm(p.name), p.kind);
      const inner = scopes.concat([s]);
      statements(n, owner, inner);
    }

    function statement(n, owner, scopes) {
      const id = n.proxyOf || n;
      if (seen.has(id)) return;
      seen.add(id);
      switch (n.sassType) {
        case 'mixin-rule':
        case 'function-rule': {
          const nm = n.sassType === 'mixin-rule' ? written(n, /^@mixin\s+([\w-]+)/, n.mixinName)[1] : written(n, /^@function\s+([\w-]+)/, n.functionName)[1];
          const ps = (n.parameters && n.parameters.nodes || []).map((p) => ({ name: p.name, kind: 'param' }));
          const s = new Map();
          for (const p of ps) s.set(norm(p.name), 'param');
          const inner = scopes.concat([s]);
          for (const p of n.parameters && n.parameters.nodes || []) if (p.defaultValue) expr(p.defaultValue, nm, inner);
          statements(n, nm, inner);
          return;
        }
        case 'variable-declaration':
          for (const c of kids(n)) expr(c, owner, scopes);
          if (scopes.length > 1 && !n.global) scopes[scopes.length - 1].set(norm(n.variableName), 'var');
          return;
        case 'include-rule': {
          const m = written(n, /^@include\s+(?:([\w-]+)\.)?([\w-]+)/, n.includeName);
          readCallable(owner, 'mixin', n.namespace || null, m[0] === null ? n.includeName : m[2]);
          for (const c of kids(n)) if (c.sassType !== 'parameter-list') expr(c, owner, scopes);
          const ps = (n.using && n.using.nodes || []).map((p) => ({ name: p.name, kind: 'param' }));
          block(n, owner, scopes, ps);
          return;
        }
        case 'each-rule':
        case 'for-rule': {
          for (const c of kids(n)) expr(c, owner, scopes);
          const vars = n.sassType === 'each-rule' ? n.variables : [n.variable];
          block(n, owner, scopes, vars.map((v) => ({ name: v, kind: 'loop' })));
          return;
        }
        case 'atrule':
        case 'rule':
        case 'decl':
        default: {
          if (n.sassType === 'atrule') {
            if (n.name === 'extend') {
              const text = String(n.params || '');
              for (const m of text.matchAll(/%([\w-]+)/g)) {
                stats.reads++;
                const nm = m[1];
                const ok = placeholderReachable(rel, norm(nm));
                stats[ok ? 'resolved' : 'unresolved']++;
                note(rel, nm, ok);
                if (ok) pairs.push({ path: rel, from: owner, to: nm });
              }
            } else if (!PARSED_AT.has(n.name) && n.params) {
              const text = String(n.params);
              for (const m of text.matchAll(/(?:([\w-]+)\.)?\$([\w-]+)/g)) readVariable(owner, scopes, m[1] || null, m[2]);
            }
          }
          for (const c of kids(n)) expr(c, owner, scopes);
          if (Array.isArray(n.nodes) && STATEMENT_NODE.test(n.sassType)) block(n, owner, scopes, null);
          return;
        }
      }
    }

    statements(tree, '<module>', [new Map()]);
  }

  // A name read in a file that no read of it resolves is excluded from both sides.
  const exclusions = [];
  for (const [rel, m] of perFile) {
    for (const [name, e] of m) if (e.resolved === 0 && e.other > 0) exclusions.push({ path: rel, to: name, reason: 'w5-unresolved' });
  }
  const byKey = (a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.to < b.to ? -1 : a.to > b.to ? 1 : 0);
  pairs.sort((a, b) => byKey(a, b) || (a.from < b.from ? -1 : a.from > b.from ? 1 : 0));
  exclusions.sort(byKey);
  const seenPair = new Set();
  const uniq = pairs.filter((p) => { const k = p.path + '\0' + p.from + '\0' + p.to; if (seenPair.has(k)) return false; seenPair.add(k); return true; });
  return { calls: uniq, call_exclusions: exclusions, stats };
}

module.exports = { analyze, isPrivate, path };
