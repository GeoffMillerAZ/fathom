#!/usr/bin/env python3
"""join.py <tag> <base> <head> — join the reading-set class dump with
execution truth, per changed function.

Function level: go tool cover -func (file:line key — the `func` keyword line
in both conserve.funcSitesIn and cmd/cover), executed = pct > 0.0.

Changed-line level (extra, stricter): coverprofile blocks (max count across
the merged per-package profiles) whose line span overlaps a line the range
added/modified at head (git diff -U0 base head) inside the function's extent
([its start line, the next function's start line) per cover -func)."""
import csv, json, re, subprocess, sys
from collections import Counter, defaultdict

M = __import__("os").environ["CVM13_DIR"]
MOD = "github.com/geoffmilleraz/fathom/"
tag, base, head = sys.argv[1:4]
clone = f"{M}/{tag}"


def rel(f):
    return f[len(MOD):] if f.startswith(MOD) else f


cover = {}
starts = defaultdict(list)
for ln in open(f"{M}/fathom/{tag}-cover-func.txt"):
    m = re.match(r"^(\S+?):(\d+):\s+(\S+)\s+([\d.]+)%$", ln.strip())
    if not m:
        continue
    f, line, name, pct = m.groups()
    f = rel(f)
    cover[(f, int(line))] = (name, float(pct))
    starts[f].append(int(line))
for f in starts:
    starts[f].sort()

blocks = defaultdict(dict)  # file -> (sl,sc,el,ec) -> [stmts, maxcount]
with open(f"{M}/cover/{tag}.coverprofile") as fh:
    next(fh)
    for ln in fh:
        m = re.match(r"^(\S+?):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)$", ln.strip())
        if not m:
            continue
        f = rel(m.group(1))
        k = tuple(int(x) for x in m.group(2, 3, 4, 5))
        st, cnt = int(m.group(6)), int(m.group(7))
        cur = blocks[f].get(k)
        if cur is None or cnt > cur[1]:
            blocks[f][k] = [st, cnt]


def changed_lines(f):
    out = subprocess.run(["git", "-C", clone, "diff", "-U0", base, head, "--", f], capture_output=True, text=True, check=True).stdout
    lines, deletion_points = set(), set()
    for m in re.finditer(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@", out, re.M):
        c, d = int(m.group(1)), int(m.group(2) or 1)
        if d == 0:
            deletion_points.add(c)
        lines.update(range(c, c + d))
    return lines, deletion_points


d = json.load(open(f"{M}/fathom/{tag}-classes.json"))
rows = []
cl_cache = {}
for it in d["items"]:
    f, start = it["file"], it["line"]
    cv = cover.get((f, start))
    name, pct = (cv if cv else ("", None))
    executed = None if pct is None else pct > 0.0
    nxt = [s for s in starts.get(f, []) if s > start]
    end = (nxt[0] - 1) if nxt else 10**9
    if f not in cl_cache:
        cl_cache[f] = changed_lines(f)
    cl, dels = cl_cache[f]
    fn_changed = {l for l in cl if start <= l <= end}
    fn_dels = {l for l in dels if start <= l <= end}
    cb = [(k, v) for k, v in blocks.get(f, {}).items() if start <= k[0] <= end and any(k[0] <= l <= k[2] for l in fn_changed | fn_dels)]
    cb_exec = [kv for kv in cb if kv[1][1] > 0]
    stmts = sum(v[0] for _, v in cb)
    stmts_exec = sum(v[0] for _, v in cb_exec)
    cls = it["class"]
    if pct is None:
        cell = "no-coverage-row"
    elif cls == "attested":
        cell = "attested-executed" if executed else "attested-NEVER-executed"
    else:
        cell = f"{cls}-executed-anyway" if executed else f"{cls}-never-executed"
    if not cb:
        line_cell = "no-changed-statement-block"
    elif stmts_exec == stmts:
        line_cell = "all-changed-blocks-executed"
    elif stmts_exec == 0:
        line_cell = "NO-changed-block-executed"
    else:
        line_cell = "some-changed-blocks-unexecuted"
    unexec = ["%d.%d-%d.%d(%d stmt)" % (k[0], k[1], k[2], k[3], v[0]) for k, v in sorted(cb) if v[1] == 0]

    def render(path):
        return " <- ".join([it["symbol"]] + [h["caller"].split("::")[-1] + "[" + (h["edge_kind"] or "?") + "]" for h in path]) if path else ""

    rows.append({
        "range": tag, "base": base, "head": head,
        "symbol": it["symbol"], "file": f, "line": start,
        "change": it["change"], "class": cls, "key": it["key"],
        "ambiguous_key": it["ambiguous_key"], "resolved_callers": it["callers"],
        "reaching_tests": it["reaching_tests"],
        "attesting_tests": len(it["attesting_tests"]),
        "first_attester": it["first_attester"],
        "first_attester_hops": len(it["first_attester_path"]),
        "first_attester_path": render(it["first_attester_path"]),
        "nearest_test": it["nearest_test"], "nearest_test_hops": len(it["nearest_test_path"]),
        "nearest_test_path": render(it["nearest_test_path"]),
        "direct_callers": "; ".join(it["direct_callers"]),
        "cover_func_name": name, "cover_pct": "" if pct is None else pct,
        "executed": "" if executed is None else executed,
        "calibration": cell,
        "fn_changed_lines": len(fn_changed), "fn_deletion_points": len(fn_dels),
        "changed_blocks": len(cb), "changed_blocks_executed": len(cb_exec),
        "changed_stmts": stmts, "changed_stmts_executed": stmts_exec,
        "changed_line_calibration": line_cell,
        "unexecuted_changed_blocks": "; ".join(unexec),
    })

fields = list(rows[0].keys())
with open(f"{M}/fathom/{tag}-per-function.csv", "w", newline="") as fh:
    w = csv.DictWriter(fh, fieldnames=fields)
    w.writeheader()
    w.writerows(rows)
json.dump(rows, open(f"{M}/fathom/{tag}-per-function.json", "w"), indent=1)
print(tag, "changed", len(rows), dict(Counter(r["class"] for r in rows)), dict(Counter(r["calibration"] for r in rows)), dict(Counter(r["changed_line_calibration"] for r in rows)))
for r in rows:
    print(f'  {r["calibration"]:20} {r["cover_pct"]!s:>6}% fn | changed stmts {r["changed_stmts_executed"]}/{r["changed_stmts"]} {r["changed_line_calibration"]:30} {r["symbol"]}  {r["file"]}:{r["line"]} ({r["change"]}) {r["unexecuted_changed_blocks"]}')
