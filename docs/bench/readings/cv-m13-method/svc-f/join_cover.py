import json, sys, re
ct = sys.argv[1]
d = json.load(open(ct + "/classes.json"))
cov = {}
for line in open(ct + "/cover-func.txt"):
    m = re.match(r'^github\.com/geoffmilleraz/conjure/(\S+?):(\d+):\s+(\S+)\s+([\d.]+)%', line)
    if m:
        cov[(m.group(1), int(m.group(2)))] = (m.group(3), float(m.group(4)))
rows = []
for r in d["functions"]:
    c = cov.get((r["file"], r["line"]))
    r["cover_func"] = c[0] if c else None
    r["cover_pct"] = c[1] if c else None
    rows.append(r)
json.dump(rows, open(ct + "/classes-with-coverage.json", "w"), indent=2)
zero = [r for r in rows if r["cover_pct"] == 0.0]
nocov = [r for r in rows if r["cover_pct"] is None]
print("changed", len(rows), "coverage 0.0%:", len(zero), "no coverage row:", len(nocov))
with open(ct + "/classes-with-coverage.tsv", "w") as f:
    f.write("class\tchange\tsymbol\tfile\tline\tcover_pct\treach_tests\tresolved_callers\tattesting_test\tvia_dispatch\n")
    for r in rows:
        f.write("\t".join(str(x) for x in [r["class"], r["change"], r["symbol"], r["file"], r["line"], r["cover_pct"], r["reach_tests"], r["resolved_callers"], r["attesting_test"], r["attesting_via_dispatch"]]) + "\n")
for r in rows:
    print("%-8s %-6s %-40s %-42s L%-4d cov=%s" % (r["class"], r["change"], r["symbol"], r["file"], r["line"], r["cover_pct"]))
