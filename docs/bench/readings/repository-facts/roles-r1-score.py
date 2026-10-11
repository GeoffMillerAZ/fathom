#!/usr/bin/env python3
"""roles-r1-score.py: RF-M8's oracle helpers and scorer (docs/bench/readings/repository-facts/roles-r1.md).

Committed with the split before any read; the reading's header is the contract it implements.

Oracle lists (each prints the runner's collected test files, repo-relative, one per line):
  roles-r1-score.py oracle go <root>                 go list -test in every go.mod directory, GOOS x GOARCH union
  roles-r1-score.py oracle py <root> <collect.txt>   pytest --collect-only -q output: node ids and error lines
  roles-r1-score.py oracle ts <root> <list.txt>      vitest list --filesOnly / jest --listTests output
  roles-r1-score.py oracle php <root> <list.xml>     phpunit --list-tests-xml, classes mapped by declaration
  roles-r1-score.py oracle java <root>               classes in every target/{surefire,failsafe}-reports/TEST-*.xml
Per subject:
  roles-r1-score.py subject <name> <eco> <harness-dir> <oracle.txt | -> [unmeasured reason]
    joins the harness's files.tsv (TestRFM1RoleReading) with the oracle and prints one JSON line.
Flag drift (RF-M1's, unchanged):
  roles-r1-score.py flags <harness-dir>...
Verdict:
  roles-r1-score.py verdict <subjects.jsonl> <flags.json> [--ungained SERVED,ALL]
    pools each ecosystem's subjects under 35 runner-collected tests (O881), applies the bars and prints
    the tables and the verdict line. --ungained gives the labelled shares when a changed share is over 2%.
"""
import collections
import json
import math
import os
import re
import subprocess
import sys
import xml.etree.ElementTree as ET

EXTS = {
    "go": {".go"},
    "py": {".py"},
    "ts": {".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"},
    "php": {".php"},
    "java": {".java"},
}
ECOS = ["go", "ts", "py", "java", "php"]
POOL_BELOW = 35  # runner-collected tests (O881): n / (n + 3.8416) >= 0.90 needs n >= 34.6
BARS = {"precision": 0.95, "wilson_lower": 0.90, "recall": 0.90, "unknown_share": 0.05, "flags": 0.02}


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


def rel(root, p):
    p = p.strip()
    if os.path.isabs(p):
        p = os.path.relpath(os.path.realpath(p), os.path.realpath(root))
    return p[2:] if p.startswith("./") else p


def tracked(root, suffix=None):
    out = subprocess.run(["git", "-C", root, "ls-files"], capture_output=True, text=True, check=True).stdout
    return [p for p in out.splitlines() if suffix is None or p.endswith(suffix)]


# ---- oracles -------------------------------------------------------------------------------------

def oracle_go(root):
    files = set()
    env = dict(os.environ, GOWORK="off", GOFLAGS="-mod=mod")
    fmt = '{{.Dir}}\t{{join .TestGoFiles " "}} {{join .XTestGoFiles " "}}'
    for mod in sorted(p for p in tracked(root) if os.path.basename(p) == "go.mod"):
        d = os.path.join(root, os.path.dirname(mod))
        for goos in ("linux", "darwin", "windows"):
            for goarch in ("amd64", "arm64"):
                r = subprocess.run(["go", "list", "-e", "-find", "-f", fmt, "./..."], cwd=d, capture_output=True, text=True,
                                   env=dict(env, GOOS=goos, GOARCH=goarch))
                if r.returncode != 0:
                    print(f"# go list {mod} {goos}/{goarch}: exit {r.returncode}: {r.stderr.strip()[:300]}", file=sys.stderr)
                for line in r.stdout.splitlines():
                    if "\t" not in line:
                        continue
                    pdir, names = line.split("\t", 1)
                    for n in names.split():
                        files.add(rel(root, os.path.join(pdir, n)))
    return files


def oracle_py(root, collect):
    files = set()
    for line in open(collect, encoding="utf-8", errors="replace"):
        line = line.strip()
        m = re.match(r"^ERROR (?:collecting )?(\S+\.py)", line)
        if m:
            files.add(rel(root, m.group(1)))
            continue
        if "::" in line:
            p = line.split("::", 1)[0]
            if p.endswith(".py"):
                files.add(rel(root, p))
    return files


def oracle_ts(root, listing):
    files = set()
    for line in open(listing, encoding="utf-8", errors="replace"):
        line = re.sub(r"\x1b\[[0-9;]*m", "", line).strip()
        if not line or not re.search(r"\.[cm]?[jt]sx?$", line):
            continue
        if line.startswith("[") and "] " in line:  # vitest prefixes a project name
            line = line.split("] ", 1)[1]
        files.add(rel(root, line))
    return files


PHP_DECLS = r'''
$out = [];
foreach (array_slice($argv, 1) as $f) {
  $t = @token_get_all(file_get_contents($f)); $ns = ''; $n = count($t);
  for ($i = 0; $i < $n; $i++) {
    if (!is_array($t[$i])) continue;
    if ($t[$i][0] === T_NAMESPACE) {
      $ns = ''; for ($j = $i + 1; $j < $n; $j++) {
        if (is_array($t[$j]) && in_array($t[$j][0], [T_STRING, T_NAME_QUALIFIED, T_NS_SEPARATOR])) $ns .= $t[$j][1];
        elseif ($t[$j] === ';' || $t[$j] === '{') break; }
    } elseif ($t[$i][0] === T_CLASS) {
      $p = $i - 1; while ($p >= 0 && is_array($t[$p]) && $t[$p][0] === T_WHITESPACE) $p--;
      if ($p >= 0 && is_array($t[$p]) && $t[$p][0] === T_DOUBLE_COLON) continue;
      for ($j = $i + 1; $j < $n; $j++) { if (is_array($t[$j]) && $t[$j][0] === T_STRING) { $out[] = [ltrim($ns . '\\' . $t[$j][1], '\\'), $f]; break; }
        if (!is_array($t[$j]) || $t[$j][0] !== T_WHITESPACE) break; }
    }
  }
}
echo json_encode($out);
'''


def oracle_php(root, listing):
    classes = set()
    tree = ET.parse(listing)
    for el in tree.iter():
        if el.tag.rsplit("}", 1)[-1] in ("testCaseClass", "testClass") and el.get("name"):
            classes.add(el.get("name").lstrip("\\"))
    decl = collections.defaultdict(set)
    paths = [p for p in tracked(root, ".php")]
    for i in range(0, len(paths), 400):
        chunk = [os.path.join(root, p) for p in paths[i:i + 400]]
        r = subprocess.run(["php", "-r", PHP_DECLS] + chunk, capture_output=True, text=True, check=True)
        for cls, f in json.loads(r.stdout or "[]"):
            decl[cls].add(rel(root, f))
    files = set()
    for c in sorted(classes):
        found = decl.get(c, set())
        if len(found) != 1:
            print(f"# php class {c}: mapped to {sorted(found) or 'no file'}", file=sys.stderr)
        files |= found
    return files


def oracle_java(root):
    classes = set()
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in (".git", "node_modules")]
        if os.path.basename(dirpath) not in ("surefire-reports", "failsafe-reports"):
            continue
        for fn in filenames:
            if fn.startswith("TEST-") and fn.endswith(".xml"):
                for tc in ET.parse(os.path.join(dirpath, fn)).iter("testcase"):
                    if tc.get("classname"):
                        classes.add(tc.get("classname"))
    srcs = collections.defaultdict(set)
    for p in tracked(root, ".java"):
        srcs[p[:-5].replace("/", ".")].add(p)
    files = set()
    for c in sorted(classes):
        outer = re.split(r"[$]", c)[0]
        cands = set()
        parts = outer.split(".")
        # a nested class reported as Outer.Inner: try the longest file-backed prefix
        for k in range(len(parts), 0, -1):
            fq = ".".join(parts[:k])
            cands = {p for key, ps in srcs.items() if key.endswith("." + fq) or key == fq for p in ps}
            if cands:
                break
        if len(cands) != 1:
            print(f"# java class {c}: mapped to {sorted(cands) or 'no file'}", file=sys.stderr)
        files |= cands
    return files


def oracle(eco, root, arg=None):
    fn = {"go": lambda: oracle_go(root), "py": lambda: oracle_py(root, arg), "ts": lambda: oracle_ts(root, arg),
          "php": lambda: oracle_php(root, arg), "java": lambda: oracle_java(root)}[eco]
    for p in sorted(fn()):
        print(p)


# ---- scoring -------------------------------------------------------------------------------------

def subject(name, eco, hdir, oracle_path, reason=""):
    files = list(read_tsv(os.path.join(hdir, "files.tsv")))
    indexed = {r["path"] for r in files}
    unknown = sum(1 for r in files if r["role"] == "unknown")
    lang = {r["path"] for r in files if os.path.splitext(r["path"])[1].lower() in EXTS[eco]}
    conv = {r["path"] for r in files if r["convention_role"] == "test" and r["path"] in lang}
    support = {r["path"] for r in files if r["convention_role"] == "test-support" and r["path"] in lang}
    out = {"subject": name, "eco": eco, "indexed": len(indexed), "graded": len(lang),
           "convention_test": len(conv), "convention_test_support": len(support),
           "unknown": unknown, "unknown_share": round(unknown / len(indexed), 4) if indexed else None,
           "roles": dict(sorted(collections.Counter(r["role"] for r in files).items()))}
    if oracle_path == "-":
        out["unmeasured"] = reason or "oracle did not run"
        print(json.dumps(out))
        return
    with open(oracle_path, encoding="utf-8") as f:
        oracle_all = {l.strip() for l in f if l.strip() and not l.startswith("#")}
    oracle_set = oracle_all & lang
    tp = len(conv & oracle_set)
    fp = sorted(conv - oracle_set)
    fn = sorted(oracle_set - conv)
    n = tp + len(fp)
    out.update({
        "oracle": len(oracle_set), "tp": tp, "fp": len(fp), "fn": len(fn),
        "precision": round(tp / n, 4) if n else None,
        "wilson_lower": round(wilson_lower(tp, n), 4) if n else None,
        "recall": round(tp / len(oracle_set), 4) if oracle_set else None,
        "oracle_not_indexed": sorted(oracle_all - indexed), "oracle_other_language": sorted((oracle_all & indexed) - lang),
        "fp_files": fp, "fn_files": fn, "fn_read_test_support": sorted(set(fn) & support),
        "support_collected": len(support & oracle_set), "support_not_collected": len(support - oracle_set),
    })
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


def fmt(v):
    return "" if v is None else (f"{v:.3f}" if isinstance(v, float) else str(v))


def verdict(subjects_path, flags_path, ungained=None):
    subs = [json.loads(l) for l in open(subjects_path, encoding="utf-8") if l.strip()]
    fl = json.load(open(flags_path, encoding="utf-8"))
    fails = []
    print("| Subject | Eco | Graded | Conv test | Conv test-support | Oracle | TP | FP | FN | Precision | Wilson lower | Recall | Unknown (of indexed) | Judged | Bars |")
    print("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|")
    strata = collections.defaultdict(list)
    for s in subs:
        unk = f'{s["unknown"]} / {s["indexed"]} ({100 * (s["unknown_share"] or 0):.1f}%)'
        if "unmeasured" in s:
            print(f'| {s["subject"]} | {s["eco"]} | {s["graded"]} | {s["convention_test"]} | {s["convention_test_support"]} | unmeasured | | | | | | | {unk} | | unmeasured: {s["unmeasured"]} |')
            continue
        own = []
        if s["recall"] is not None and s["recall"] < BARS["recall"]:
            own.append("recall")
        if s["unknown_share"] is None or s["unknown_share"] > BARS["unknown_share"]:
            own.append("unknown")
        judged = "alone" if s["oracle"] >= POOL_BELOW else "pooled"
        if judged == "alone":
            if s["precision"] is None or s["precision"] < BARS["precision"]:
                own.append("precision")
            if s["wilson_lower"] is None or s["wilson_lower"] < BARS["wilson_lower"]:
                own.append("Wilson")
        else:
            strata[s["eco"]].append(s)
        if own:
            fails.append(f'{s["subject"]}: {", ".join(own)}')
        print(f'| {s["subject"]} | {s["eco"]} | {s["graded"]} | {s["convention_test"]} | {s["convention_test_support"]} | {s["oracle"]} | {s["tp"]} | {s["fp"]} | {s["fn"]} | {fmt(s["precision"])} | {fmt(s["wilson_lower"])} | {fmt(s["recall"])} | {unk} | {judged} | {"fails " + ", ".join(own) if own else ("pass" if judged == "alone" else "own bars pass")} |')
    print()
    print("| Pooled stratum | Subjects | Oracle | TP | FP | Precision | Wilson lower | Bars |")
    print("|---|---|---|---|---|---|---|---|")
    for eco in ECOS:
        members = strata.get(eco)
        if not members:
            continue
        tp = sum(s["tp"] for s in members)
        n = tp + sum(s["fp"] for s in members)
        prec = tp / n if n else None
        wl = wilson_lower(tp, n) if n else None
        bad = []
        if prec is None or prec < BARS["precision"]:
            bad.append("precision")
        if wl is None or wl < BARS["wilson_lower"]:
            bad.append("Wilson")
        if bad:
            fails.append(f"{eco} pooled stratum: {', '.join(bad)}")
        print(f'| {eco} | {", ".join(s["subject"] for s in members)} | {sum(s["oracle"] for s in members)} | {tp} | {n - tp} | {fmt(prec)} | {fmt(wl)} | {"fails " + ", ".join(bad) if bad else "pass"} |')
    print()
    measured = {s["eco"] for s in subs if "unmeasured" not in s}
    for eco in ECOS:
        if eco not in measured:
            fails.append(f"{eco}: no measured subject")
    shares = {k: fl[k]["share"] for k in ("served", "all")}
    if ungained:
        shares = dict(zip(("served", "all"), ungained))
        print(f"Flag bar, labelled shares without a gain: served {shares['served']:.4f}, all {shares['all']:.4f}")
    else:
        print(f"Flag bar, every changed flag counted as without a gain (an upper bound): served {shares['served']:.4f}, all {shares['all']:.4f}")
    for k, v in shares.items():
        if v is None or v > BARS["flags"]:
            fails.append(f"flags ({k}): {v} over 2%" + ("" if ungained else ", label the change classes and pass --ungained"))
    print()
    for f in fails:
        print(f"- fails: {f}")
    print()
    print("RF-M8 verdict: " + ("CUT" if fails else "KEEP"))


if __name__ == "__main__":
    a = sys.argv[1:]
    if len(a) >= 3 and a[0] == "oracle":
        oracle(a[1], a[2], a[3] if len(a) > 3 else None)
    elif len(a) >= 5 and a[0] == "subject":
        subject(a[1], a[2], a[3], a[4], " ".join(a[5:]))
    elif len(a) >= 2 and a[0] == "flags":
        flags(a[1:])
    elif len(a) >= 3 and a[0] == "verdict":
        ug = None
        if len(a) >= 5 and a[3] == "--ungained":
            ug = [float(x) for x in a[4].split(",")]
        verdict(a[1], a[2], ug)
    else:
        sys.exit(__doc__)
