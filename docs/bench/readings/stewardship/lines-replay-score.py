#!/usr/bin/env python3
"""Score SV-M1 from lines-replay.sh's output against the bars fixed in the row.

    lines-replay-score.py OUT_DIR BACKUP_DIR

Bars (docs/plans/stewardship.md SV-M1, vision 41 §4.1 and §7):
  1. both valuable held items and the unfinished worker read claims rows, touches
     rows or at risk;
  2. at least 9 in 10 of the lines the consolidation removed as merged read landed;
  3. at most 25 lines flagged for a look (claims rows, touches rows or at risk).
A CUT is bar 1 or bar 3 failing (vision 41 §7).
"""
import json
import os
import sys

out, backup = sys.argv[1], sys.argv[2]
FLAG = {"claims-rows", "touches-rows", "at-risk"}


def load(name):
    with open(os.path.join(out, name + ".lines.json")) as f:
        return json.load(f)


def counts(rep):
    c = {}
    for l in rep["lines"]:
        c[l["class"]] = c.get(l["class"], 0) + 1
    return c


repos = ["sextant", "agentic-kit"]
held = {"held-wa_kit": ["wa-kit", "wa-kit-lefthook"], "held-agentsmd_kit": ["agentsmd"], "held-lq-wip-css": ["wip-css/lang/css"]}
reps = {r: load(r) for r in repos + list(held)}

print("## per repository")
for r in repos + list(held):
    rep = reps[r]
    print(f"{r}: {len(rep['lines'])} lines, open rows {rep['open_rows']}, classes {dict(sorted(counts(rep).items()))}")

print("\n## bar 1: held items")
bar1 = True
for r, names in held.items():
    for n in names:
        l = next((x for x in reps[r]["lines"] if x["name"] == n), None)
        cls = l["class"] if l else "missing"
        ok = cls in FLAG
        bar1 &= ok
        print(f"{r} {n}: {cls} {l.get('at_risk') if l else ''} {'PASS' if ok else 'FAIL'}")

print("\n## bar 2: lines the consolidation removed as merged (sextant)")
# The 9-29 record: every non-main local branch (53) and every linked worktree
# (46) of sextant was removed. worktrees.tsv records how each worktree was
# handled: "clean (HEAD in bundle)" or its dirty diff saved as a patch first.
wt_state = {}
with open(os.path.join(backup, "manifest.parts", "worktrees.tsv")) as f:
    for row in f:
        cols = row.rstrip("\n").split("\t")
        if cols[0] != "sextant":
            continue
        wt_state[os.path.basename(cols[1])] = cols[4].split()[0] != "0"
sx = reps["sextant"]
branches = [l for l in sx["lines"] if l["kind"] == "branch"]
detached = [l for l in sx["lines"] if l["kind"] == "worktree"]
landed_b = [l for l in branches if l["class"] == "landed"]
dirty_b = [l for l in branches if any(a.startswith("dirty worktree") for a in (l.get("at_risk") or []))]
print(f"branch lines: {len(branches)}, landed {len(landed_b)} ({len(landed_b)/len(branches):.3f})")
print(f"  not landed: {[ (l['name'], l['class'], l.get('at_risk')) for l in branches if l['class'] != 'landed']}")
clean_b = [l for l in branches if l not in dirty_b]
print(f"branch lines whose worktree the record removed clean (or that had none): {len(clean_b)}, landed {sum(l['class']=='landed' for l in clean_b)}")
print(f"detached worktrees (removed clean, HEAD in bundle): {len(detached)}, landed {sum(l['class']=='landed' for l in detached)}: {[(l['class'], l.get('claims'), l.get('at_risk')) for l in detached]}")
merged_set = clean_b + detached
m_landed = sum(l["class"] == "landed" for l in merged_set)
print(f"removed without a saved diff (clean branches + detached worktrees): {m_landed}/{len(merged_set)} = {m_landed/len(merged_set):.3f}")
strict = branches + detached
s_landed = sum(l["class"] == "landed" for l in strict)
print(f"every removed branch and detached worktree: {s_landed}/{len(strict)} = {s_landed/len(strict):.3f}")
print(f"stashes (dropped; their messages say landed): {[(l['name'], l['class'], l.get('touches')) for l in sx['lines'] if l['kind']=='stash']}")

print("\n## bar 3: lines flagged for a look")
total = 0
for r in repos + list(held):
    n = sum(1 for l in reps[r]["lines"] if l["class"] in FLAG)
    total += n
    print(f"{r}: {n}")
two = sum(1 for r in repos for l in reps[r]["lines"] if l["class"] in FLAG)
print(f"the two repositories: {two}; with the held clones: {total}; bar <= 25: {'PASS' if two <= 25 else 'FAIL'}")
print(f"\nbar 1 {'PASS' if bar1 else 'FAIL'}; verdict {'KEEP' if bar1 and two <= 25 else 'CUT'}")
