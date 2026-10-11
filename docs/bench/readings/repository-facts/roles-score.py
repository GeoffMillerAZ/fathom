#!/usr/bin/env python3
"""roles-score.py: RF-M1's scorer (docs/bench/readings/repository-facts/roles.md).

Per subject:   roles-score.py subject <name> <eco> <harness-dir> <oracle.txt>
  joins the harness's files.tsv (TestRFM1RoleReading) with the oracle's list of test files
  (repo-relative paths, one per line) and prints one JSON line: the convention test role's
  precision, its Wilson 95% lower bound, recall, the unknown share of indexed files, and the
  disagreeing files.
Flag drift:    roles-score.py flags <harness-dir>...
  reads every flags.tsv and prints the share of hits whose search flags changed with RF-2, served
  (first 50 per query) and all, with every change grouped by the rule that made it.
"""
import collections
import json
import math
import os
import sys

EXTS = {
    "go": {".go"},
    "py": {".py"},
    "ts": {".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"},
    "php": {".php"},
    "java": {".java"},
}


def wilson_lower(tp, n, z=1.96):
    if n == 0:
        return 0.0
    p = tp / n
    denom = 1 + z * z / n
    centre = p + z * z / (2 * n)
    margin = z * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n))
    return (centre - margin) / denom


def read_tsv(path):
    with open(path, encoding="utf-8") as f:
        header = f.readline().rstrip("\n").split("\t")
        for line in f:
            yield dict(zip(header, line.rstrip("\n").split("\t")))


def subject(name, eco, hdir, oracle_path):
    files = list(read_tsv(os.path.join(hdir, "files.tsv")))
    indexed = {r["path"] for r in files}
    unknown = sum(1 for r in files if r["role"] == "unknown")
    lang = {r["path"] for r in files if os.path.splitext(r["path"])[1].lower() in EXTS[eco]}
    conv = {r["path"] for r in files if r["convention_test"] == "1" and r["path"] in lang}
    with open(oracle_path, encoding="utf-8") as f:
        oracle_all = {l.strip().lstrip("./") for l in f if l.strip()}
    oracle = oracle_all & lang
    not_indexed = sorted(oracle_all - indexed)
    other_lang = sorted((oracle_all & indexed) - lang)
    tp = len(conv & oracle)
    fp = sorted(conv - oracle)
    fn = sorted(oracle - conv)
    n = tp + len(fp)
    roles = collections.Counter(r["role"] for r in files)
    bases = collections.Counter(r["basis"] or "-" for r in files)
    out = {
        "subject": name, "eco": eco, "indexed": len(indexed), "graded": len(lang),
        "convention_test": len(conv), "oracle": len(oracle), "tp": tp, "fp": len(fp), "fn": len(fn),
        "precision": round(tp / n, 4) if n else None,
        "wilson_lower": round(wilson_lower(tp, n), 4) if n else None,
        "recall": round(tp / len(oracle), 4) if oracle else None,
        "unknown": unknown, "unknown_share": round(unknown / len(indexed), 4) if indexed else None,
        "oracle_not_indexed": not_indexed, "oracle_other_language": other_lang,
        "fp_files": fp, "fn_files": fn, "roles": dict(sorted(roles.items())), "bases": dict(sorted(bases.items())),
    }
    print(json.dumps(out))


def change_class(path, before, after):
    b, a = set(filter(None, before.split(","))), set(filter(None, after.split(",")))
    parts = []
    for flag in sorted(b | a):
        if flag in a and flag not in b:
            parts.append("+" + flag)
        elif flag in b and flag not in a:
            parts.append("-" + flag)
    base = path.rsplit("/", 1)[-1]
    ext = os.path.splitext(base)[1].lower() or base
    return " ".join(parts) + " [" + ext + "]"


def flags(hdirs):
    tot = collections.Counter()
    changed = collections.Counter()
    classes = collections.defaultdict(collections.Counter)
    examples = collections.defaultdict(list)
    for hdir in hdirs:
        name = os.path.basename(os.path.normpath(hdir))
        for r in read_tsv(os.path.join(hdir, "flags.tsv")):
            for scope in ("all", "served") if r["served"] == "1" else ("all",):
                tot[scope] += 1
                if r["before"] != r["after"]:
                    changed[scope] += 1
                    c = change_class(r["path"], r["before"], r["after"])
                    classes[scope][c] += 1
                    if scope == "all" and len(examples[c]) < 4 and r["path"] not in examples[c]:
                        examples[c].append(name + ":" + r["path"])
    out = {}
    for scope in ("served", "all"):
        out[scope] = {
            "hits": tot[scope], "changed": changed[scope],
            "share": round(changed[scope] / tot[scope], 4) if tot[scope] else None,
            "classes": dict(classes[scope].most_common()),
        }
    out["examples"] = examples
    print(json.dumps(out, indent=1))


if __name__ == "__main__":
    if len(sys.argv) >= 6 and sys.argv[1] == "subject":
        subject(*sys.argv[2:6])
    elif len(sys.argv) >= 3 and sys.argv[1] == "flags":
        flags(sys.argv[2:])
    else:
        sys.exit(__doc__)
