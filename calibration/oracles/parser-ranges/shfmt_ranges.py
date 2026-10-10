#!/usr/bin/env python3
"""shfmt_ranges.py: Bash definition ranges from mvdan.cc/sh's parser (tier B).

universal-ctags 6.2.1 writes no end line for a Sh function, so the bash
reading's L1 caps range_agreement at not_measured. shfmt (mvdan.cc/sh/v3, an
independent Go parser of POSIX shell and Bash) prints its syntax tree with
`--to-json`; every FuncDecl node carries the position of its first byte and of
the byte after its last. This script runs shfmt once per subject file and
writes each function declaration's first and last line as the range companion
`indexbench lang-oracle` reads beside a tool oracle (<name>.ranges.json).

    python3 -I shfmt_ranges.py --shfmt <bin> --ext .sh --ext .bash \
        --shebang bash --shebang sh <clone> > out/<tool-oracle>.ranges.json

Files: every file with a listed extension, and every file with no extension
whose `#!` line names a listed interpreter (the bash manifest's match rule;
env's first non-flag argument, a version suffix allowed). Each file is parsed
as Bash (`-ln bash`). A file shfmt rejects is counted in `errored` and named in
`unparsed`, never guessed. Nested function declarations are reported too.
"""

import argparse
import json
import os
import subprocess
import sys


def interpreter(head):
    """The interpreter a `#!` first line names, as connector.shebangInterpreter reads it."""
    if not head.startswith(b"#!"):
        return ""
    line = head[2:].split(b"\n", 1)[0].decode("utf-8", "replace").strip()
    fields = line.split()
    if not fields:
        return ""
    prog = os.path.basename(fields[0])
    if prog == "env":
        for f in fields[1:]:
            if not f.startswith("-") and "=" not in f:
                return os.path.basename(f)
        return ""
    return prog


def interpreter_is(interp, name):
    return interp.startswith(name) and interp[len(name):].strip("0123456789.") == ""


def wanted(root, rel, exts, shebangs):
    base = os.path.basename(rel)
    i = base.rfind(".")
    ext = base[i:].lower() if i >= 0 else ""  # Go's filepath.Ext: ".bashrc" has one
    if ext:
        return ext in exts
    try:
        with open(os.path.join(root, rel), "rb") as fh:
            head = fh.read(256)
    except OSError:
        return False
    interp = interpreter(head)
    return any(interpreter_is(interp, s) for s in shebangs)


def func_decls(node, out):
    if isinstance(node, dict):
        if node.get("Type") == "FuncDecl":
            name = node.get("Name", {}).get("Value", "")
            start, end = node["Pos"], node["End"]
            last = end["Line"]
            # End is the position after the last byte: at column 1 it is the
            # next line's start, so the declaration ends on the line before.
            if end.get("Col", 0) <= 1 and last > start["Line"]:
                last -= 1
            out.append((name, start["Line"], last))
        for v in node.values():
            func_decls(v, out)
    elif isinstance(node, list):
        for v in node:
            func_decls(v, out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--shfmt", required=True)
    ap.add_argument("--ext", action="append", default=[])
    ap.add_argument("--shebang", action="append", default=[])
    ap.add_argument("root")
    a = ap.parse_args()
    exts = {e.lower() for e in a.ext}
    version = subprocess.run([a.shfmt, "--version"], capture_output=True, text=True, check=True).stdout.strip()
    files = []
    for dirpath, dirnames, filenames in os.walk(a.root):
        dirnames[:] = sorted(d for d in dirnames if d != ".git")
        for f in sorted(filenames):
            rel = os.path.relpath(os.path.join(dirpath, f), a.root).replace(os.sep, "/")
            if os.path.isfile(os.path.join(a.root, rel)) and wanted(a.root, rel, exts, a.shebang):
                files.append(rel)
    ranges, unparsed = [], []
    for rel in sorted(files):
        with open(os.path.join(a.root, rel), "rb") as fh:
            src = fh.read()
        p = subprocess.run([a.shfmt, "-ln", "bash", "--to-json"], input=src, capture_output=True)
        if p.returncode != 0:
            unparsed.append(rel)
            print(f"shfmt: {rel}: {p.stderr.decode('utf-8', 'replace').strip()}", file=sys.stderr)
            continue
        found = []
        func_decls(json.loads(p.stdout), found)
        for name, line, end in found:
            ranges.append({"path": rel, "name": name, "kind": "function", "line": line, "end": end})
    ranges.sort(key=lambda r: (r["path"], r["line"], r["name"]))
    json.dump({
        "tool": "shfmt --to-json (mvdan.cc/sh/v3 syntax, -ln bash)",
        "tier": "B",
        "version": version,
        "units": len(files) - len(unparsed),
        "errored": len(unparsed),
        "unparsed": unparsed,
        "kinds": {"function": "function"},
        "scope": "the first and last line of each function declaration (a syntax.FuncDecl, nested ones included)",
        "ranges": ranges,
    }, sys.stdout, indent=1, sort_keys=True)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
