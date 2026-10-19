#!/usr/bin/env python3
"""tags_oracle.py — a tier-C L1/L2 oracle for fathom's language-breadth
harness (docs/design/language-breadth.md §1.3 tier C, §10; ADR-048
amendment D5): an upstream tags.scm run by an INDEPENDENT tree-sitter
runtime (py-tree-sitter, the C library, never fathom's wazero engine).
Bench-only instrument, run as a separate process.

Writes the neutral tool-oracle JSON `indexbench lang-oracle -tool-oracle`
reads, tier "C":
  calls:        each @reference.call's @name.reference.call text paired
                with the @name.definition.function of the innermost
                @definition.function node containing it (a call with no
                enclosing function is at top level and skipped, or with
                --module-caller paired with that name; variable
                assignments are not callables and never enclose)
  definitions:  (--defs only) every @definition.function, ranged by its node

Usage:
  tags_oracle.py --lang bash --query bash-tags.scm [--defs] \
      --ext .sh --ext .bash --shebang bash --shebang sh <root> > out.json
  tags_oracle.py --lang lua --query lua-tags.scm --ext .lua <root> > out.json
  tags_oracle.py --lang ruby --query ruby-tags.scm --locals ruby-locals.scm \
      --ext .rb --ext .rake --ext .ru --basename Rakefile <root> > out.json

--locals FILE applies the grammar's upstream locals query the way tree-sitter-tags does
for a tags pattern carrying `(#is-not? local)` (Ruby's bare-identifier and constant
references): a name that a `@local.definition` in an enclosing `@local.scope` (or the
file's root scope) binds before the reference is a local variable, not a call. Each name
node yields one tag, the first pattern to claim it (a class's own name is its definition,
never also a reference).
"""
import argparse
import json
import os
import re
import sys

import tree_sitter

LANGS = {"bash": ("tree_sitter_bash", "language"), "javascript": ("tree_sitter_javascript", "language"),
         "lua": ("tree_sitter_lua", "language"), "luau": ("tree_sitter_luau", "language"), "ruby": ("tree_sitter_ruby", "language"),
         "scala": ("tree_sitter_scala", "language"), "r": ("tree_sitter_r", "language")}


def load_language(name):
    mod_name, fn = LANGS[name]
    mod = __import__(mod_name)
    return tree_sitter.Language(getattr(mod, fn)())


def candidate(path, rel, exts, shebangs, basenames=()):
    if os.path.splitext(rel)[1].lower() in exts or os.path.basename(rel) in basenames:
        return True
    if os.path.splitext(rel)[1] or not shebangs:
        return False
    try:
        with open(path, "rb") as f:
            first = f.readline(256).decode("utf-8", "replace")
    except OSError:
        return False
    m = re.match(r"#!\s*(?:/usr/bin/env\s+)?\S*?([A-Za-z0-9_.-]+)(?:\s|$)", first)
    return bool(m) and m.group(1) in shebangs


def read_locals(lq, cursor, root, size, src):
    """The file's local scopes (start, end, inherits) — the root scope first — and its
    local definitions (name, start byte, innermost scope index), as tree-sitter-tags reads them."""
    scopes, defs = [(0, size, False)], []
    matches = list(cursor.matches(root))
    for pat, caps in matches:
        if "local.scope" in caps:
            inherits = lq.pattern_settings(pat).get("local.scope-inherits") != "false"
            n = caps["local.scope"][0]
            scopes.append((n.start_byte, n.end_byte, inherits))
    for pat, caps in matches:
        for n in caps.get("local.definition", []):
            best = 0
            for i, (s0, e0, _) in enumerate(scopes):
                if s0 <= n.start_byte and n.end_byte <= e0 and (scopes[best][1] - scopes[best][0]) >= (e0 - s0):
                    best = i
            defs.append((src[n.start_byte:n.end_byte], n.start_byte, best))
    return scopes, defs


def is_local(node, src, scopes, defs):
    name = src[node.start_byte:node.end_byte]
    containing = [i for i, (s0, e0, _) in enumerate(scopes) if s0 <= node.start_byte and node.end_byte <= e0]
    containing.sort(key=lambda i: scopes[i][1] - scopes[i][0])
    for i in containing:
        for dn, dstart, di in defs:
            # a definition binds its own name node and every later reference (a `baz` read
            # before `baz = 5` is a call; tree-sitter-tags reads the definitions in order)
            if di == i and dn == name and dstart <= node.start_byte:
                return True
        if not scopes[i][2]:
            break
    return False


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--lang", required=True, choices=sorted(LANGS))
    ap.add_argument("--query", required=True)
    ap.add_argument("--ext", action="append", default=[])
    ap.add_argument("--shebang", action="append", default=[])
    ap.add_argument("--basename", action="append", default=[], help="a file name (no extension) that is a candidate, e.g. Rakefile")
    ap.add_argument("--locals", help="the grammar's locals query (see the module doc)")
    ap.add_argument("--defs", action="store_true", help="emit definitions instead of calls")
    ap.add_argument("--module-caller", default="",
                    help="pair a call no definition encloses with this caller (K18: a script's top level is its "
                         "module symbol's code, core.ModuleSymbolName `<module>`) instead of skipping it")
    ap.add_argument("root")
    a = ap.parse_args()

    lang = load_language(a.lang)
    parser = tree_sitter.Parser(lang)
    with open(a.query, encoding="utf-8") as f:
        query = tree_sitter.Query(lang, f.read())
    cursor = tree_sitter.QueryCursor(query)
    # Patterns that carry `(#is-not? local)`: their @name must not be a local variable.
    nonlocal_patterns = {i for i in range(query.pattern_count)
                         if "local" in query.pattern_assertions(i) and not query.pattern_assertions(i)["local"][1]}
    locals_query = locals_cursor = None
    if a.locals:
        with open(a.locals, encoding="utf-8") as f:
            locals_query = tree_sitter.Query(lang, f.read())
        locals_cursor = tree_sitter.QueryCursor(locals_query)
    elif nonlocal_patterns:
        sys.exit("the query has (#is-not? local) patterns: pass --locals")
    root = os.path.abspath(a.root)
    files = []
    for dp, dns, fns in os.walk(root):
        dns[:] = sorted(d for d in dns if d != ".git")
        for fn in sorted(fns):
            p = os.path.join(dp, fn)
            rel = os.path.relpath(p, root).replace(os.sep, "/")
            if os.path.isfile(p) and candidate(p, rel, [e.lower() for e in a.ext], a.shebang, a.basename):
                files.append((p, rel))
    files.sort(key=lambda x: x[1])

    defs_out, calls_out, errors = [], [], 0
    for p, rel in files:
        with open(p, "rb") as f:
            src = f.read()
        tree = parser.parse(src)
        if tree.root_node.has_error:
            errors += 1
        funcs, calls = [], []
        local_scopes, local_defs = [(0, len(src), False)], []
        if locals_cursor is not None:
            local_scopes, local_defs = read_locals(locals_query, locals_cursor, tree.root_node, len(src), src)
        # With --locals the tags library's own rules apply: an `@ignore` capture (Ruby's
        # `(setter (identifier) @ignore)`) drops that node's tag and a name node yields one tag.
        # Without it (bash, lua) every match counts, as before.
        matches = list(cursor.matches(tree.root_node))
        ignored = {(n.start_byte, n.end_byte) for _, caps in matches for n in caps.get("ignore", [])}
        claimed = set(ignored)
        for pat, caps in matches:
            if locals_cursor is not None and "name" in caps:
                key = (caps["name"][0].start_byte, caps["name"][0].end_byte)
                if key in claimed:
                    continue
                if pat in nonlocal_patterns and is_local(caps["name"][0], src, local_scopes, local_defs):
                    continue
                claimed.add(key)
            # aider's tags.scm names its captures @name.definition.function; a
            # grammar's own tags.scm (tree-sitter-lua) writes a plain @name and
            # tags a colon method as @definition.method, which encloses calls too.
            for kind in ("definition.function", "definition.method"):
                if kind in caps:
                    node = caps[kind][0]
                    nm = caps.get("name." + kind) or caps["name"]
                    funcs.append((node, nm[0].text.decode("utf-8", "replace")))
                    break
            else:
                if "reference.call" in caps:
                    calls.append((caps.get("name.reference.call") or caps["name"])[0])
        for node, name in funcs:
            defs_out.append({"path": rel, "name": name, "line": node.start_point[0] + 1,
                             "end": node.end_point[0] + 1, "kind": "function"})
        pairs = set()
        for c in calls:
            enc = None
            for node, name in funcs:
                if node.start_byte <= c.start_byte and c.end_byte <= node.end_byte:
                    if enc is None or (enc[0].start_byte <= node.start_byte and node.end_byte <= enc[0].end_byte):
                        enc = (node, name)
            if enc is not None:
                pairs.add((enc[1], c.text.decode("utf-8", "replace")))
            elif a.module_caller:
                pairs.add((a.module_caller, c.text.decode("utf-8", "replace")))
        for frm, to in sorted(pairs):
            calls_out.append({"path": rel, "from": frm, "to": to})

    from importlib.metadata import version
    out = {"tool": "tree-sitter-tags:" + os.path.basename(a.query),
           "version": "py-tree-sitter %s, tree-sitter-%s %s" % (version("tree-sitter"), a.lang, version("tree-sitter-" + a.lang)),
           "tier": "C"}
    if a.defs:
        out["definitions"] = defs_out
    else:
        out["calls"] = calls_out
    json.dump(out, sys.stdout, indent=None, separators=(",", ":"))
    sys.stdout.write("\n")
    print("files=%d with_parse_errors=%d definitions=%d calls=%d" % (len(files), errors, len(defs_out), len(calls_out)), file=sys.stderr)


if __name__ == "__main__":
    main()
