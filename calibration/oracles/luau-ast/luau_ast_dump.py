#!/usr/bin/env python3
"""luau_ast_dump.py -- tier-B Luau L1 oracle (syntax only; see README.md).

Runs Luau's own parser CLI (luau-ast, luau-lang/luau, MIT) over every *.luau file under <root> and writes the neutral
oracle file `indexbench lang-oracle -tool-oracle` reads:
  {"tool","version","tier":"B","definitions":[{path,name,line,end,kind}]}
Paths are repo-relative with forward slashes, lines 1-based, names bare (the last segment).

A definition is a function stored in a statically named slot, or a type alias:
  function f() / local function f() / function a.b.c() / function a.b:c()    (name: the last identifier)
  x = function ... / local x = function ... / a.b = function ...             (name: the variable's last identifier)
  { k = function ... } / { ["k"] = function ... } / t["k"] = function ...    (name: the key, the string unquoted)
  type T = ... / export type T<U> = ...                                       (kind "type", name: T)
A function in a computed slot ({ [expr] = function ... }, t[expr] = function ...) has no static name and is not a
definition; an anonymous function passed as an argument, returned or invoked in place is not one either.
`local a, b = function..., function...` pairs each variable with the value at its position. `line` is the statement's
(or table item's) first line, `end` the function's (the alias's) last line. A file luau-ast rejects is counted and
skipped.

usage: python3 -I luau_ast_dump.py --luau-ast <luau-ast> <root> > out.json
"""
import argparse
import json
import os
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor


def lines(loc):
    a, b = loc.split(" - ")
    return int(a.split(",")[0]) + 1, int(b.split(",")[0]) + 1


def last_name(e):
    """The static name of an assignment target, or None."""
    t = e.get("type")
    if t == "AstExprGlobal":
        return e["global"]
    if t == "AstExprLocal":
        return e["local"]["name"]
    if t == "AstExprIndexName":
        return e["index"]
    if t == "AstExprIndexExpr" and e["index"].get("type") == "AstExprConstantString":
        return e["index"]["value"]
    return None


def walk(n, out):
    if isinstance(n, list):
        for x in n:
            walk(x, out)
        return
    if not isinstance(n, dict):
        return
    t = n.get("type")
    if t == "AstStatTypeAlias":
        s, e = lines(n["location"])
        out.append((n["name"], s, e, "type"))
    elif t == "AstStatLocalFunction":
        s, e = lines(n["location"])
        out.append((n["name"]["name"], s, e, "function"))
    elif t == "AstStatFunction":
        name = last_name(n["name"])
        if name is not None:
            s, e = lines(n["location"])
            out.append((name, s, e, "function"))
    elif t == "AstStatLocal":
        s = lines(n["location"])[0]
        for var, val in zip(n["vars"], n["values"]):
            if val.get("type") == "AstExprFunction":
                out.append((var["name"], s, lines(val["location"])[1], "function"))
    elif t == "AstStatAssign":
        s = lines(n["location"])[0]
        for var, val in zip(n["vars"], n["values"]):
            if val.get("type") == "AstExprFunction":
                name = last_name(var)
                if name is not None:
                    out.append((name, s, lines(val["location"])[1], "function"))
    elif t == "AstExprTable":
        for item in n["items"]:
            val = item.get("value") or {}
            key = item.get("key")
            if val.get("type") == "AstExprFunction" and key and key.get("type") == "AstExprConstantString":
                out.append((key["value"], lines(key["location"])[0], lines(val["location"])[1], "function"))
    for k, v in n.items():
        if k not in ("location", "luauType"):
            walk(v, out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--luau-ast", required=True)
    ap.add_argument("--version", default="luau-ast (luau-lang/luau 0.743)", help="the version string the oracle file records")
    ap.add_argument("root")
    a = ap.parse_args()
    root = os.path.abspath(a.root)
    files = []
    for d, ds, fs in os.walk(root):
        ds[:] = sorted(x for x in ds if x != ".git")
        files += [os.path.join(d, f) for f in sorted(fs) if f.endswith(".luau")]

    def one(f):
        p = subprocess.run([a.luau_ast, f], capture_output=True)
        try:
            doc = json.loads(p.stdout.decode("utf-8", "replace"))
        except ValueError:
            return f, None
        out = []
        walk(doc["root"], out)
        return f, out

    with ThreadPoolExecutor(max_workers=4) as ex:
        results = list(ex.map(one, files))
    defs, bad = [], 0
    forms = {}
    for f, out in results:
        if out is None:
            bad += 1
            continue
        rel = os.path.relpath(f, root).replace(os.sep, "/")
        for name, s, e, kind in out:
            defs.append({"path": rel, "name": name, "line": s, "end": e, "kind": kind})
            forms[kind] = forms.get(kind, 0) + 1
    defs.sort(key=lambda d: (d["path"], d["line"], d["name"]))
    json.dump({"tool": "luau-ast", "tier": "B", "version": a.version, "definitions": defs}, sys.stdout, indent=None, separators=(",", ":"))
    sys.stdout.write("\n")
    print("files=%d luau_ast_rejected=%d definitions=%d forms=%s" % (len(files), bad, len(defs), json.dumps(forms, sort_keys=True)), file=sys.stderr)


if __name__ == "__main__":
    main()
