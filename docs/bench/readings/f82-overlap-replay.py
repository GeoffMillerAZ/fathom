#!/usr/bin/env python3
"""F82 Promote-when replay: recompute every merge in a window with git merge-tree
and class each conflicted file. Zero cost: git only, no daemon, no model.
Run: python3 -I docs/bench/readings/f82-overlap-replay.py [since] [until] [ref] > out.tsv
Output: TSV rows  merge_sha  date  file  class  detail   (sorted, deterministic)
Plus '#' summary lines on stderr-free stdout tail are produced by the reading, not here."""
import subprocess, sys, re, collections

SINCE = sys.argv[1] if len(sys.argv) > 1 and sys.argv[1] != 'summary' else "2026-09-19"
UNTIL = sys.argv[2] if len(sys.argv) > 2 else "2026-10-20"
REF = sys.argv[3] if len(sys.argv) > 3 else "origin/main"

def git(*a, check=True):
    r = subprocess.run(["git", *a], capture_output=True)
    if check and r.returncode not in (0, 1):
        raise RuntimeError(r.stderr.decode(errors="replace"))
    return r

APPEND_PATH = [
    r"^CHANGELOG\.md$", r"^docs/languages\.md$", r"^docs/plans/.*\.md$",
    r"^\.fathom/observations(\.md|/)", r"^ADR\.md$", r"^docs/dogfood-observations\.md$",
    r"^docs/intent/(futures|decisions|incidents|questions)", r"^docs/changelog/",
    r"_gen\.go$", r"^docs/languages/", r"^docs/mcp-tools\.md$",
]
GEN_HDR = re.compile(rb"^// Code generated|DO NOT EDIT")

def hunks(blob):
    """conflict hunks in a blob: list of (start_line, ours_lines, theirs_lines, enclosing_func)"""
    lines = blob.decode(errors="replace").split("\n")
    out, func, i = [], None, 0
    while i < len(lines):
        l = lines[i]
        m = re.match(r"^func (\([^)]*\) )?([A-Za-z0-9_]+)", l)
        if m: func = m.group(2)
        if l.startswith("<<<<<<< "):
            j = i + 1; ours = []
            while j < len(lines) and not lines[j].startswith("=======") and not lines[j].startswith("||||||| "):
                ours.append(lines[j]); j += 1
            while j < len(lines) and not lines[j].startswith("======="): j += 1
            k = j + 1; theirs = []
            while k < len(lines) and not lines[k].startswith(">>>>>>> "):
                theirs.append(lines[k]); k += 1
            # inside a func body if the next top-level close brace is a func end: approximate by
            # indentation of the hunk lines (tab-indented => body)
            body = any(x.startswith("\t") or x.startswith("    ") for x in ours + theirs) and func is not None
            out.append((i + 1, ours, theirs, func if body else None))
            i = k
        i += 1
    return out

def ranges(base, side, path):
    """changed base-line ranges (a, n) of `side` vs `base`: old lines a..a+n-1; n==0 means insertion after line a"""
    r = git("diff", "-U0", "--no-renames", base, side, "--", path, check=False)
    out = []
    for m in re.finditer(r"^@@ -(\d+)(?:,(\d+))? ", r.stdout.decode(errors="replace"), re.M):
        out.append((int(m.group(1)), int(m.group(2)) if m.group(2) is not None else 1))
    return out

def symbols(basesrc):
    """top-level symbols of a Go file: (kind, name, first_line, last_line), 1-based, by column-0 conventions"""
    L = basesrc.split("\n"); syms = []; i = 0
    while i < len(L):
        l = L[i]
        m = re.match(r"^(func) (?:\([^)]*\) )?([A-Za-z0-9_]+)|^(type) ([A-Za-z0-9_]+)|^(var|const) ([A-Za-z0-9_]+|\()", l)
        if m:
            kind = m.group(1) or m.group(3) or m.group(5)
            name = m.group(2) or m.group(4) or m.group(6)
            j = i
            if l.rstrip().endswith("{") or l.rstrip().endswith("("):
                close = "}" if l.rstrip().endswith("{") else ")"
                j = i + 1
                while j < len(L) and not L[j].startswith(close): j += 1
            syms.append((kind, name, i + 1, j + 1)); i = j
        i += 1
    return syms

def touched(syms, rs):
    t = set()
    for a, n in rs:
        for kind, name, f, l in syms:
            if n > 0 and not (a + n - 1 < f or a > l): t.add((kind, name))
            elif n == 0 and f <= a < l: t.add((kind, name))
    return t

def classify(path, blob, mb, p1, p2):
    for p in APPEND_PATH:
        if re.search(p, path): return "APPEND-POINT", "path:" + p
    if blob is None: return "OTHER", "no blob (delete/modify or binary)"
    if GEN_HDR.search(blob[:400]): return "APPEND-POINT", "generated file"
    if not path.endswith(".go"): return "OTHER", "non-Go text, same region edited (not a symbol)"
    b = git("show", f"{mb}:{path}", check=False)
    if b.returncode != 0: return "ADJACENT", "add/add of one Go file"
    syms = symbols(b.stdout.decode(errors="replace"))
    both = sorted(touched(syms, ranges(mb, p1, path)) & touched(syms, ranges(mb, p2, path)))
    funcs = [n for k, n in both if k in ("func", "type")]
    tables = [n for k, n in both if k in ("var", "const")]
    if funcs: return "SEMANTIC", "both sides changed " + ",".join(funcs)
    if tables: return "APPEND-POINT", "table/const " + ",".join(tables)
    return "ADJACENT", "same region, no shared symbol (adds beside each other)"

def main():
    r = git("log", "--merges", "--format=%H %cs", f"--since={SINCE}", f"--until={UNTIL}", REF)
    merges = [l.split() for l in r.stdout.decode().splitlines() if l]
    rows, stats = [], collections.Counter()
    for sha, date in merges:
        parents = git("rev-list", "--parents", "-n1", sha).stdout.decode().split()[1:]
        stats["merges"] += 1
        if len(parents) != 2:
            stats["octopus"] += 1; continue
        m = git("merge-tree", "--write-tree", "--name-only", parents[0], parents[1])
        if m.returncode == 0:
            stats["clean"] += 1; continue
        if m.returncode != 1:
            stats["error"] += 1; continue
        out = m.stdout.decode(errors="replace").split("\n")
        tree = out[0]
        files = []
        for l in out[1:]:
            if l == "": break
            files.append(l)
        stats["conflicted"] += 1
        mb = git("merge-base", parents[0], parents[1]).stdout.decode().split()[0]
        for f in sorted(set(files)):
            b = git("cat-file", "blob", f"{tree}:{f}", check=False)
            blob = b.stdout if b.returncode == 0 else None
            c, d = classify(f, blob, mb, parents[0], parents[1])
            rows.append((date, sha[:9], f, c, d))
    rows.sort()
    for r in rows: print("\t".join(r))
    print("#stats\t" + "\t".join(f"{k}={v}" for k, v in sorted(stats.items())))

def summary(tsv):
    """markdown counts from the TSV (stdin): python3 -I f82-overlap-replay.py summary < out.tsv"""
    rows = [l.rstrip("\n").split("\t") for l in tsv if not l.startswith("#")]
    merges = {}
    for date, sha, f, c, d in rows:
        m = merges.setdefault((date[:7], sha), {"files": [], "cls": set(), "sem_nontest": False})
        m["files"].append(f); m["cls"].add(c)
        if c == "SEMANTIC" and not f.endswith("_test.go"): m["sem_nontest"] = True
    months = sorted({k[0] for k in merges})
    print("| month | conflicted merges | with SEMANTIC (any Go file) | with SEMANTIC (non-test Go) | with APPEND-POINT | with ADJACENT | with OTHER |")
    print("|---|---|---|---|---|---|---|")
    tot = collections.Counter()
    for mo in months + ["window"]:
        ms = [v for k, v in merges.items() if mo == "window" or k[0] == mo]
        c = [len(ms), sum("SEMANTIC" in v["cls"] for v in ms), sum(v["sem_nontest"] for v in ms),
             sum("APPEND-POINT" in v["cls"] for v in ms), sum("ADJACENT" in v["cls"] for v in ms), sum("OTHER" in v["cls"] for v in ms)]
        print("| %s | %s |" % (mo, " | ".join(map(str, c))))
    print()
    print("| class | conflicted files |"); print("|---|---|")
    cc = collections.Counter(r[3] for r in rows)
    for k in sorted(cc): print("| %s | %d |" % (k, cc[k]))

def table(tsv):
    """markdown table, one row per (merge, class): python3 -I f82-overlap-replay.py table < out.tsv"""
    g = collections.OrderedDict()
    for l in tsv:
        if l.startswith("#"): continue
        d, sha, f, c, x = l.rstrip("\n").split("\t")
        g.setdefault((d, sha, c), []).append(f)
    print("| merge | date | class | files |"); print("|---|---|---|---|")
    for (d, sha, c), fs in g.items():
        print("| %s | %s | %s | %s |" % (sha, d, c, ", ".join("`%s`" % f for f in sorted(fs))))

if len(sys.argv) > 1 and sys.argv[1] == "summary":
    summary(sys.stdin)
elif len(sys.argv) > 1 and sys.argv[1] == "table":
    table(sys.stdin)
else:
    main()
