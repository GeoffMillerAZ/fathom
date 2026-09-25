#!/usr/bin/env python3
"""wholejoin.py — supplement: every non-test Go function at the r2 head,
reading-set class (same attested rule) vs execution (cover -func > 0.0%)."""
import csv, json, re
from collections import Counter, defaultdict

M = __import__("os").environ["CVM13_DIR"]
MOD = "github.com/geoffmilleraz/fathom/"
cover = {}
for ln in open(f"{M}/fathom/r2-cover-func.txt"):
    m = re.match(r"^(\S+?):(\d+):\s+(\S+)\s+([\d.]+)%$", ln.strip())
    if m:
        f, line, name, pct = m.groups()
        cover[(f[len(MOD):] if f.startswith(MOD) else f, int(line))] = (name, float(pct))

items = json.load(open(f"{M}/fathom/r2-wholehead-classes.json"))
rows = []
for it in items:
    cv = cover.get((it["file"], it["line"]))
    path = it.get("nearest_attester_path") or []
    kinds = [h["edge_kind"] for h in path]
    callers = [h["caller"].split("::")[-1] for h in path]
    feats = []
    if "iface-dispatch" in kinds:
        feats.append("iface-dispatch-hop")
    if "func-value" in kinds:
        feats.append("func-value-hop")
    if any(re.search(r"\.register[A-Z]\w*$", h["caller"]) for h in path):
        feats.append("register*-closure-hop")
    if any(c.endswith(".runWithStdin") or c.endswith(".run") for c in callers):
        feats.append("cli-dispatcher-hop")
    if any(c.endswith(".newServer") or c.endswith(".NewServer") for c in callers):
        feats.append("mcp-server-construction-hop")
    rows.append({
        "name": it["name"], "file": it["file"], "line": it["line"], "class": it["class"],
        "ambiguous_key": it["ambiguous_key"], "reaching_tests": it["reaching_tests"],
        "attesting_tests": it["attesting_tests"], "nearest_attester": it.get("nearest_attester", ""),
        "nearest_hops": len(path),
        "nearest_path": " <- ".join([it["name"]] + [c + "[" + k + "]" for c, k in zip(callers, kinds)]),
        "path_features": ";".join(feats),
        "cover_pct": "" if cv is None else cv[1],
        "executed": "" if cv is None else cv[1] > 0.0,
    })
with open(f"{M}/fathom/r2-wholehead-per-function.csv", "w", newline="") as fh:
    w = csv.DictWriter(fh, fieldnames=list(rows[0].keys()))
    w.writeheader()
    w.writerows(rows)

joined = [r for r in rows if r["executed"] != ""]
print("functions", len(rows), "joined to cover -func", len(joined), "unjoined", len(rows) - len(joined))
cm = Counter((r["class"], r["executed"]) for r in joined)
for k in sorted(cm):
    print("  ", k, cm[k])
ane = [r for r in joined if r["class"] == "attested" and not r["executed"]]
print("attested-never-executed", len(ane), "by path feature:")
print("  ", Counter(r["path_features"] or "(plain direct path)" for r in ane).most_common())
print("attested-executed by path feature:")
print("  ", Counter(r["path_features"] or "(plain direct path)" for r in joined if r["class"] == "attested" and r["executed"]).most_common())
print("hops (never-executed):", sorted(Counter(r["nearest_hops"] for r in ane).items()))
print("hops (executed):", sorted(Counter(r["nearest_hops"] for r in joined if r["class"] == "attested" and r["executed"]).items()))
json.dump(rows, open(f"{M}/fathom/r2-wholehead-per-function.json", "w"), indent=1)
