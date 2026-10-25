#!/usr/bin/env python3
"""ctags_c_glsl.py -- tier-C GLSL L1 oracle (definitions; see README.md).

Runs universal-ctags' C parser over every GLSL file under <root> (the C parser is told to take each GLSL extension with
--map-C=+.ext) and writes the neutral oracle file `indexbench lang-oracle -tool-oracle` reads:
  {"tool","version","tier":"C","definitions":[{path,name,line,end,kind}],"disclose":[...]}
Paths are repo-relative with forward slashes, lines 1-based, names bare. Only the kinds the dialect's oracle_kinds names
are written (function, struct, macro); the rest of ctags' C output (members, variables, prototypes) is counted in
`disclose`. ctags 6.2.1 has no GLSL parser, so this is a foreign parser over a C-like language: tier C, never B.

usage: python3 -I ctags_c_glsl.py --ctags <ctags> <root> <out.json>
"""
import argparse
import json
import os
import re
import subprocess
import sys
import tempfile

EXTS = [".glsl", ".vert", ".frag", ".comp", ".geom", ".tesc", ".tese", ".rgen", ".rchit", ".rmiss", ".rahit",
        ".rint", ".rcall", ".mesh", ".task", ".vsh", ".fsh"]
KEEP = {"function", "struct", "macro"}
SKIP_DIRS = {".git"}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ctags", required=True)
    ap.add_argument("root")
    ap.add_argument("out")
    a = ap.parse_args()
    root = os.path.abspath(a.root)
    a.ctags = os.path.abspath(a.ctags)
    files = []
    for d, dirs, names in os.walk(root):
        dirs[:] = sorted(x for x in dirs if x not in SKIP_DIRS)
        for n in sorted(names):
            if os.path.splitext(n)[1] in EXTS:
                files.append(os.path.relpath(os.path.join(d, n), root).replace(os.sep, "/"))
    ver = subprocess.run([a.ctags, "--version"], capture_output=True, text=True).stdout.splitlines()[0]
    m = re.search(r"Universal Ctags ([0-9][^,( ]*)", ver)
    version = m.group(1) if m else ver
    with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as lf:
        lf.write("\n".join(files) + "\n")
    cmd = [a.ctags, "-f", "-", "--languages=C", "--fields=+neKS", "--sort=no", "--excmd=number", "-L", lf.name]
    cmd += ["--map-C=+" + e for e in EXTS]
    p = subprocess.run(cmd, cwd=root, capture_output=True, text=True, errors="replace")
    os.unlink(lf.name)
    if p.returncode != 0:
        sys.exit("ctags failed: " + p.stderr.strip()[:500])
    defs, skipped = [], {}
    for ln in p.stdout.splitlines():
        if ln.startswith("!_"):
            continue
        f = ln.split("\t")
        if len(f) < 4:
            continue
        name, path, kind = f[0], f[1], f[3]
        ext = {}
        for x in f[4:]:
            k, _, v = x.partition(":")
            ext[k] = v
        if kind not in KEEP:
            skipped[kind] = skipped.get(kind, 0) + 1
            continue
        line = int(ext["line"])
        defs.append({"path": path, "name": name, "line": line, "end": int(ext.get("end", line)), "kind": kind})
    defs.sort(key=lambda d: (d["path"], d["line"], d["name"], d["kind"]))
    out = {
        "tool": "universal-ctags (C parser over GLSL)",
        "version": version,
        "tier": "C",
        "definitions": defs,
        "disclose": [
            "L1 oracle is universal-ctags %s's C parser run over %d GLSL files (--map-C for each GLSL extension): ctags has no GLSL "
            "parser, so it is a foreign parser, tier C, and L1 caps at good (owner ruling G1); kinds outside function, struct and "
            "macro (%s) are not graded"
            % (version, len(files), ", ".join("%s %d" % (k, v) for k, v in sorted(skipped.items())) or "none")
        ],
    }
    with open(a.out, "w") as fh:
        json.dump(out, fh, indent=1)
    print("files=%d definitions=%d skipped=%s" % (len(files), len(defs), skipped), file=sys.stderr)


if __name__ == "__main__":
    main()
