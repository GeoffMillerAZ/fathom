import json
from collections import Counter
M = __import__("os").environ["CVM13_DIR"] + "/control"
for tag in ["r1","r2","r3"]:
    d=json.load(open(f"{M}/{tag}/classes.json"))
    rows=json.load(open(f"{M}/{tag}/classes-with-coverage.json"))
    cov={r["file"]+":"+r["symbol"]:r["cover_pct"] for r in rows}
    base=Counter(r["class"] for r in d["functions"])
    tab=Counter()
    for r in d["functions"]:
        p=r["cover_pct"] if "cover_pct" in r else cov.get(r["file"]+":"+r["symbol"])
        if r["key"]=="": continue
        tab[(r["class"]=="attested", None if p is None else p>0)]+=1
    print("==",tag, open(f"{M}/{tag}/range.txt").read().strip())
    print("   conserve residue:", d["residue_counts"])
    print("   %-34s %s  att&exec=%d att&never=%d flag&exec=%d flag&never=%d" % ("baseline", dict(base), tab[(True,True)], tab[(True,False)], tab[(False,True)], tab[(False,False)]))
    for name,m in sorted(d["ablations"].items()):
        c=Counter(m.values()); t=Counter()
        for k,v in m.items():
            p=cov.get(k); t[(v=="attested", None if p is None else p>0)]+=1
        print("   %-34s %s  att&exec=%d att&never=%d flag&exec=%d flag&never=%d" % (name, dict(c), t[(True,True)], t[(True,False)], t[(False,True)], t[(False,False)]))
