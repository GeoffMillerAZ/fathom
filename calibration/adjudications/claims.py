#!/usr/bin/env python3
"""Draft D3 adjudication claims (docs/design/language-breadth.md §1.5).

usage: claims.py <classes.json> <unmatched.json> <subject-root> > claims.json

<unmatched.json> is `indexbench lang-oracle -unmatched`'s output for one
subject. Every dialect-only definition whose canonical kind a class names
and whose source text (its first line through at most two more, never past
its end) matches the class pattern ({name} = the escaped name) becomes a
claim. This only drafts: `lang-oracle -adjudicate` re-checks every claim
deterministically (langbench.Adjudicate) and admits or refuses it; the
reading lists what it admitted. Classes carry `why` (the verified oracle
deficiency, with a file:line that shows it) and `kinds` (the canonical
kinds they may claim; the harness ignores `kinds`).
"""
import json, re, sys

classes = json.load(open(sys.argv[1]))
unmatched = json.load(open(sys.argv[2]))
root = sys.argv[3]
cache = {}
claims = []
for d in unmatched.get("dialect_only") or []:
    path, line, name, kind, end = d["Path"], d["Line"], d["Name"], d["Kind"], d["End"]
    if path not in cache:
        with open(f"{root}/{path}", encoding="utf-8", errors="replace") as f:
            cache[path] = f.read().split("\n")
    lines = cache[path]
    last = min(line + 2, len(lines))
    if line <= end < last:
        last = end
    text = "\n".join(lines[line - 1:last])
    for cname, c in sorted(classes.items()):
        if kind not in c.get("kinds", []):
            continue
        if re.search(c["pattern"].replace("{name}", re.escape(name)), text):
            claims.append({"path": path, "line": line, "name": name, "class": cname})
            break
out = {"classes": {k: {"why": v["why"], "pattern": v["pattern"]} for k, v in classes.items()},
       "definitions": claims}
json.dump(out, sys.stdout, indent=1)
print()
