#!/usr/bin/env python3
"""Prune a SCRATCH copy of a mirror to its used closure (SV-M3).

    prune-to-closure.py TREE DIR USED_JSON

Reads `fathom report used DIR --json` (its used_files) and deletes every
other file under TREE/DIR, then empty directories. Keeps the provenance and
licence files a vendored copy must carry whatever the closure says
(.vendor-commit, LICENSE*, NOTICE*, COPYING*), and counts them apart. A
throwaway script for a reading: fathom itself never copies, prunes or
vendors (SV-X4). Prints units, files and bytes before and after.
"""
import json
import os
import sys

tree, rel, used_json = sys.argv[1], sys.argv[2], sys.argv[3]
res = json.load(open(used_json))
keep = set(res["used_files"])
KEEP_ALWAYS = (".vendor-commit",)
KEEP_PREFIX = ("LICENSE", "NOTICE", "COPYING")

root = os.path.join(tree, rel)
before_files = before_bytes = after_files = after_bytes = 0
kept_extra = []
for d, dirs, files in os.walk(root):
    for f in files:
        p = os.path.join(d, f)
        r = os.path.relpath(p, tree)
        n = os.path.getsize(p) if os.path.isfile(p) else 0
        before_files += 1
        before_bytes += n
        if r in keep:
            after_files += 1
            after_bytes += n
            continue
        if os.path.dirname(r) == rel and (f in KEEP_ALWAYS or f.upper().startswith(KEEP_PREFIX)):
            kept_extra.append(r)
            after_files += 1
            after_bytes += n
            continue
        os.remove(p)
for d, dirs, files in os.walk(root, topdown=False):
    if d != root and not os.listdir(d):
        os.rmdir(d)

units_before = res["totals"]["go_packages"] + sum(1 for u in res["unreached"] if u["kind"] != "go-package") + sum(1 for u in res["used"] if u["kind"] != "go-package")
units_after = len(res["used"])
print(f"{rel}: units {units_before} -> {units_after} (go packages {res['totals']['go_packages']} -> {res['totals']['used_go_packages']}); "
      f"files {before_files} -> {after_files}; bytes {before_bytes} -> {after_bytes} "
      f"({100 * after_bytes / max(before_bytes, 1):.1f}%); kept beside the closure: {kept_extra}")
