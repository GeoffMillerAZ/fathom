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
                enclosing function is at top level and skipped; variable
                assignments are not callables and never enclose)
  definitions:  (--defs only) every @definition.function, ranged by its node

Usage:
  tags_oracle.py --lang bash --query bash-tags.scm [--defs] \
      --ext .sh --ext .bash --shebang bash --shebang sh <root> > out.json
"""
import argparse
import json
import os
import re
import sys

import tree_sitter

LANGS = {"bash": ("tree_sitter_bash", "language")}


def load_language(name):
    mod_name, fn = LANGS[name]
    mod = __import__(mod_name)
    return tree_sitter.Language(getattr(mod, fn)())


def candidate(path, rel, exts, shebangs):
    if os.path.splitext(rel)[1].lower() in exts:
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


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--lang", required=True, choices=sorted(LANGS))
    ap.add_argument("--query", required=True)
    ap.add_argument("--ext", action="append", default=[])
    ap.add_argument("--shebang", action="append", default=[])
    ap.add_argument("--defs", action="store_true", help="emit definitions instead of calls")
    ap.add_argument("root")
    a = ap.parse_args()

    lang = load_language(a.lang)
    parser = tree_sitter.Parser(lang)
    with open(a.query, encoding="utf-8") as f:
        query = tree_sitter.Query(lang, f.read())
    cursor = tree_sitter.QueryCursor(query)
    root = os.path.abspath(a.root)
    files = []
    for dp, dns, fns in os.walk(root):
        dns[:] = sorted(d for d in dns if d != ".git")
        for fn in sorted(fns):
            p = os.path.join(dp, fn)
            rel = os.path.relpath(p, root).replace(os.sep, "/")
            if os.path.isfile(p) and candidate(p, rel, [e.lower() for e in a.ext], a.shebang):
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
        for _, caps in cursor.matches(tree.root_node):
            if "definition.function" in caps:
                node = caps["definition.function"][0]
                name = caps["name.definition.function"][0].text.decode("utf-8", "replace")
                funcs.append((node, name))
            elif "reference.call" in caps:
                calls.append(caps["name.reference.call"][0])
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
