#!/usr/bin/env python3
"""ctags_oracle.py: universal-ctags' Make definitions, with two verified defects corrected.

Writes indexbench lang-oracle's neutral oracle JSON to stdout (tier B, `definitions` only):
  {"tool": "universal-ctags", "version": ..., "tier": "B",
   "definitions": [{"path", "name", "line", "end", "kind": "target" | "variable"}]}

It runs the same invocation the harness runs for a tier-B oracle (langbench.CtagsArgs plus
--languages=Make) over every Makefile-like file of the subject (the harness drops the files fathom
does not read), and corrects the two defects the measurement found in ctags 6.2.1's Make parser
(design section 1.5 D2: a stronger instrument where ctags is deficient, disclosed):

  1. Line. ctags reports every name of a statement continued with a backslash at the statement's
     LAST physical line: in
         o/$(MODE)/a.o \\
         o/$(MODE)/b.o: private CFLAGS += -O3
     both targets are reported at line 2. A name that does not occur on its reported line but
     occurs on an earlier line of the same continued statement is moved to the first such line
     not already taken by the same name (a verified mis-report, not a disagreement: the name is
     not on the line ctags gives).
  2. Modifier keywords. `private CFLAGS += x` is tagged as a variable named `private` (likewise
     `override`, `export`, `unexport`): a make keyword is never a variable name. Those tags are dropped.

Every other ctags tag is passed through unchanged, including the ones this measurement finds
wrong (`V ::= 1` tagged as a target, `define D :=` tagged with the operator in its name, a
target-specific assignment tagged as both a target and a variable). The stderr summary counts each
correction so the reading can disclose it.

Usage: ctags_oracle.py -subject <checkout> [-ctags ctags] > oracle.json
"""
import argparse
import json
import os
import re
import subprocess
import sys

NAMES = {"Makefile", "makefile", "GNUmakefile"}
SUFFIXES = (".mk", ".make")
KEYWORDS = {"private", "override", "export", "unexport"}
KIND = {"target": "target", "macro": "variable"}


def find_files(root):
    out = []
    for d, dirs, files in os.walk(root):
        dirs[:] = sorted(x for x in dirs if x != ".git")
        for f in sorted(files):
            p = os.path.join(d, f)
            if (f in NAMES or f.endswith(SUFFIXES)) and not os.path.islink(p):
                out.append(os.path.relpath(p, root).replace(os.sep, "/"))
    return sorted(out)


def continued(line):
    m = re.search(r"(\\+)$", line)
    return bool(m) and len(m.group(1)) % 2 == 1


def statement_start(lines, line):
    """First physical line (1-based) of the statement whose last physical line is `line`."""
    s = line
    while s > 1 and continued(lines[s - 2]):
        s -= 1
    return s


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-ctags", default="ctags")
    a = ap.parse_args()
    root = os.path.realpath(a.subject)
    ver = subprocess.run([a.ctags, "--version"], stdout=subprocess.PIPE, text=True).stdout
    m = re.search(r"Universal Ctags ([0-9][^,( ]*)", ver)
    if not m:
        sys.exit("ctags_oracle: %s is not universal-ctags" % a.ctags)
    files = find_files(root)
    listing = subprocess.run(
        [a.ctags, "--output-format=json", "--fields=+nKzSe", "--extras=-{anonymous}", "-f", "-",
         "--languages=Make", "-L", "-"],
        cwd=root, input="\n".join(files) + "\n", stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
    src = {}
    def lines_of(p):
        if p not in src:
            src[p] = open(os.path.join(root, p), encoding="utf-8", errors="replace").read().split("\n")
        return src[p]
    defs, moved, dropped, taken = [], 0, 0, set()
    for raw in listing.stdout.splitlines():
        raw = raw.strip()
        if not raw:
            continue
        t = json.loads(raw)
        if t.get("_type") != "tag" or t.get("kind") not in KIND:
            continue
        path, name, line, kind = t["path"], t["name"], t["line"], t["kind"]
        if kind == "macro" and name in KEYWORDS:
            dropped += 1
            continue
        if kind == "target":
            ls = lines_of(path)
            start = statement_start(ls, line)
            if start < line:
                pat = re.compile(r"(?:^|[\s:])" + re.escape(name) + r"(?:$|[\s\\:;])")
                for cand in range(start, line + 1):
                    if pat.search(ls[cand - 1]) and (path, name, cand) not in taken:
                        if cand != line:
                            moved += 1
                        line = cand
                        break
            taken.add((path, name, line))
        defs.append({"path": path, "name": name, "line": line, "end": t.get("end", 0), "kind": KIND[kind]})
    defs.sort(key=lambda d: (d["path"], d["line"], d["name"], d["kind"]))
    json.dump({"tool": "universal-ctags", "version": m.group(1), "tier": "B", "definitions": defs}, sys.stdout, indent=1)
    sys.stdout.write("\n")
    sys.stderr.write("ctags_oracle: %d files, %d definitions; corrected: %d targets moved to the physical line "
                     "that spells them, %d keyword-named variables dropped\n" % (len(files), len(defs), moved, dropped))


if __name__ == "__main__":
    main()
