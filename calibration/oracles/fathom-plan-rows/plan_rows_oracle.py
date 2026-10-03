"""plan_rows_oracle: the language-breadth harness's oracle for the
fathom-plan-rows native dialect (design section 1.3 and 18.3;
LB-Q-fathom-plan-rows). It never reads fathom's own plan code
(internal/verify.ParseMarkdownPlan, gen-plan-cue): the row grammar is read
again here from docs/plans/README.md ("Line format", "Annotation keys",
"Annotation placement") by a line parser of its own.

    python3 plan_rows_oracle.py -subject <checkout> -out <json> \
        [-cue <cue> -schema schema/plan/schema.cue -work <scratch>] \
        [-ctags <universal-ctags> -symbols-out <json>] [-summary-out <json>]
    python3 plan_rows_oracle.py -compare <edges.tsv> -symbols <json>

Which metric family each oracle grades (stated in the reading's notes):

  - Rows and needs, tier C (-out): this parser, the plan grammar's reading
    of a Markdown file. definitions are one `task` per row that carries an
    author-owned id (line = the checkbox line, end = the last line of the
    row's wrapped text), the L1 family. references are a row to each row of
    the SAME plan its `needs` names (bare id, or <this plan>.md#ID), the L2
    family. cross_references are a row to the row of ANOTHER plan under the
    same directory that `needs: <plan>.md#ID` names, the L3 family (the plan
    to plan joins). Tier C: the oracle's own reading of a grammar no tool
    implements for it; it caps every metric at good (section 1.3).
  - Structure, tier A (-cue): `cue vet -d '#Plan' schema/plan/schema.cue` over
    the JSON this parser produces for each plan. The count that fails is
    printed on stderr and kept in -summary-out; a file that fails is still
    listed, because the dialect reads a row whether or not it is schema-valid.
    It grades that the parser's rows have the plan schema's shape, not the
    entities.
  - Symbol anchors to files, tier B (-ctags, -symbols-out): universal-ctags
    over the subject's Go, Python, JavaScript and TypeScript source gives,
    for each `symbol:` (with its `package:` directory when set) the files
    defining that name. The harness scores only references between files it
    measures (plan to plan), so the plan to code join is compared by
    `-compare` against the edges a scratch ingest stored, and reported, not
    part of the reading's levels.

Plans are `docs/plans/*.md` (not the archive/ subdirectory, not README.md).
Bench-only; never a product dependency.
"""

import argparse
import json
import os
import re
import subprocess
import sys

TOOL = "plan_rows_oracle.py: a line parser of docs/plans/README.md's row grammar"
SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}

ROW = re.compile(r"^\s*[-*]\s+\[([ xX~-])\]\s+(.*\S)\s*$")
LIST_ITEM = re.compile(r"^\s*([-*+]|\d+[.)])(\s|$)")
BASELINE = re.compile(r"^\s*baseline\s*:\s*(\S+)", re.I)
PROOF = re.compile(r"^\s*proof\s*:", re.I)
ID_TOKEN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")
CROSS = re.compile(r"^([A-Za-z0-9][A-Za-z0-9._/-]*\.md)#([A-Za-z0-9][A-Za-z0-9._-]*)$")


def trailing_block(s):
    """The {...} block whose closing brace is last; braces inside a
    double-quoted span are value content. Returns (open, close) or None."""
    pairs, stack, in_q = [], [], False
    for i, c in enumerate(s):
        if c == '"':
            in_q = not in_q
        elif c == "{" and not in_q:
            stack.append(i)
        elif c == "}" and not in_q and stack:
            pairs.append((stack.pop(), i))
    if not pairs:
        return None
    return max(pairs, key=lambda p: p[1])


def split_pairs(block):
    out, cur, in_q = [], [], False
    for c in block:
        if c == '"':
            in_q = not in_q
        if c == "," and not in_q:
            out.append("".join(cur))
            cur = []
        else:
            cur.append(c)
    out.append("".join(cur))
    return out


def annotation(s):
    """(key -> value or None when the row has no annotation, title)."""
    b = trailing_block(s)
    if not b:
        return None, s.strip()
    ann = {}
    for pair in split_pairs(s[b[0] + 1:b[1]]):
        if ":" not in pair:
            continue
        k, v = pair.split(":", 1)
        k, v = k.strip(), v.strip()
        if len(v) >= 2 and v[0] == '"' and v[-1] == '"':
            v = v[1:-1]
        if k:
            ann[k] = v
    title = (s[:b[0]].strip() + " " + s[b[1] + 1:].strip()).strip()
    return (ann or None), title


def parse_plan(text):
    """The rows of one plan: dicts with id, line, end, state, title, ann."""
    lines = text.split("\n")
    rows = []
    i = 0
    while i < len(lines):
        m = ROW.match(lines[i])
        if not m:
            i += 1
            continue
        raw = m.group(2)
        ann, title = annotation(raw)
        start, end = i, i
        if ann is None:
            cont = []
            j = i + 1
            while j < len(lines):
                l = lines[j].rstrip("\r")
                t = l.strip()
                if t == "" or l[0] not in " \t":
                    break
                if LIST_ITEM.match(l) or t.startswith("<!--") or t.startswith("```") or t.startswith("~~~") or BASELINE.match(l) or PROOF.match(l):
                    break
                cont.append(t)
                j += 1
            if cont:
                ann, title = annotation(" ".join([raw] + cont))
                end = i + len(cont)
        i = end + 1
        rid = (ann or {}).get("id", "")
        rows.append({"id": rid, "line": start + 1, "end": end + 1, "state": m.group(1), "title": title, "ann": ann or {}})
    return rows


def needs_of(row):
    v = row["ann"].get("needs", "")
    return [t.strip() for t in v.split(",") if t.strip()]


def plan_files(subject):
    d = os.path.join(subject, "docs", "plans")
    if not os.path.isdir(d):
        return []
    return sorted(f for f in os.listdir(d) if f.endswith(".md") and f != "README.md" and os.path.isfile(os.path.join(d, f)))


def plan_json(name, rows):
    """The parser's rows as a plan document, for `cue vet -d '#Plan'`."""
    tasks = []
    for r in rows:
        a = r["ann"]
        t = {"id": r["id"] or "md-%03d" % (len(tasks) + 1), "title": r["title"]}
        st = {"x": "claimed", "X": "claimed", "~": "in-progress", "-": "skipped"}.get(r["state"])
        if st:
            t["state"] = st
        for k in ("disposition", "reason", "trigger"):
            if a.get(k):
                t[k] = a[k]
        e = {}
        for k in ("symbol", "package", "reachable_from", "oracle", "path", "pattern", "min_tier"):
            if a.get(k):
                e[k] = a[k]
        if a.get("skip"):
            e["skip_reason"] = a["skip"]
        for k in ("skip_test", "allow_contradiction", "library"):
            if a.get(k, "").lower() in ("true", "yes", "1"):
                e[k] = True
        for k in ("max", "min", "eq"):
            if re.fullmatch(r"-?\d+", a.get(k, "")):
                e[k] = int(a[k])
        if e:
            t["expect"] = e
        tasks.append(t)
    return {"service": "subject", "tasks": tasks}


def definitions_and_refs(subject, only_summary):
    defs, refs, xrefs, anchors = [], [], [], []
    plans = {}
    for f in plan_files(subject):
        plans[f] = parse_plan(open(os.path.join(subject, "docs", "plans", f), encoding="utf-8").read())
    declared = {f: {r["id"] for r in rows if r["id"]} for f, rows in plans.items()}
    for f, rows in plans.items():
        path = "docs/plans/" + f
        for r in rows:
            if not r["id"]:
                continue
            defs.append({"path": path, "name": r["id"], "line": r["line"], "end": r["end"], "kind": "task"})
            seen = set()
            for n in needs_of(r):
                m = CROSS.match(n)
                if m:
                    tid = m.group(2)
                    if m.group(1) == f:
                        if tid in declared[f] and tid != r["id"] and tid not in seen:
                            seen.add(tid)
                            refs.append({"path": path, "from": r["id"], "to": tid})
                    elif m.group(1) in plans and tid in declared[m.group(1)]:
                        xrefs.append({"path": path, "from": r["id"], "to_path": "docs/plans/" + m.group(1), "to": tid})
                elif ID_TOKEN.match(n):
                    if n in declared[f] and n != r["id"] and n not in seen:
                        seen.add(n)
                        refs.append({"path": path, "from": r["id"], "to": n})
            a = r["ann"]
            if a.get("symbol") or a.get("reachable_from") or a.get("package"):
                anchors.append({"path": path, "row": r["id"], "symbol": a.get("symbol", ""), "reachable_from": a.get("reachable_from", ""), "package": a.get("package", "").strip().strip("/")})
    return plans, defs, refs, xrefs, anchors


def ctags_defs(subject, ctags):
    """name -> {(file, kind)} over the subject's source (tier B)."""
    cmd = [ctags, "-R", "--output-format=json", "--fields=+nK", "--languages=Go,Python,JavaScript,TypeScript",
           "--exclude=.git", "--exclude=vendor", "--exclude=node_modules", "--exclude=testdata", "--exclude=ref", "--exclude=third_party", "--exclude=.claude", "-f", "-", "."]
    p = subprocess.run(cmd, cwd=subject, capture_output=True, text=True)
    out = {}
    for line in p.stdout.splitlines():
        try:
            t = json.loads(line)
        except ValueError:
            continue
        if t.get("_type") != "tag" or t.get("kind") not in ("func", "function", "method", "class", "type", "struct", "interface", "const", "var", "constant", "variable"):
            continue
        out.setdefault(t["name"], set()).add(os.path.normpath(t["path"]))
    return out


def symbol_truth(anchors, tags):
    """Each anchor's file by ctags: the one file defining the name under its
    package directory (unique), else none (absent or ambiguous)."""
    res = []
    for a in anchors:
        for field in ("symbol", "reachable_from"):
            sym = a[field]
            if not sym:
                continue
            bare = sym.split(".")[-1]
            files = sorted(f for f in tags.get(bare, ()) if not a["package"] or os.path.dirname(f) == a["package"])
            res.append({"path": a["path"], "row": a["row"], "field": field, "symbol": sym, "package": a["package"], "files": files})
    return res


def compare(edges_tsv, symbols_json):
    """Product plan-symbol edges (from_file, row, to_file) against ctags truth."""
    truth = json.load(open(symbols_json))
    want = {}
    for t in truth:
        if len(t["files"]) >= 1:
            want.setdefault((t["path"], t["row"]), set()).update(t["files"])
    got = {}
    for line in open(edges_tsv, encoding="utf-8"):
        f = line.rstrip("\n").split("\t")
        if len(f) == 3:
            got.setdefault((f[0], f[1]), set()).add(f[2])
    pairs_got = {(k, v) for k, vs in got.items() for v in vs}
    pairs_want = {(k, v) for k, vs in want.items() for v in vs}
    # a product edge is correct when its target file defines the anchor's name
    # (ctags names the file); recall is over rows ctags resolves to ONE file.
    unique = {(t["path"], t["row"], t["field"]): t["files"][0] for t in truth if len(t["files"]) == 1}
    unique_pairs = {((p, r), f) for (p, r, _fld), f in unique.items()}
    correct = {pg for pg in pairs_got if pg in pairs_want}
    found = {pw for pw in unique_pairs if pw in pairs_got}
    res = {
        "product_edges": len(pairs_got),
        "product_edges_in_ctags_truth": len(correct),
        "precision": round(len(correct) / len(pairs_got), 4) if pairs_got else None,
        "ctags_unique_anchors": len(unique_pairs),
        "ctags_unique_found": len(found),
        "recall": round(len(found) / len(unique_pairs), 4) if unique_pairs else None,
    }
    print(json.dumps(res, indent=1))
    return res


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject")
    ap.add_argument("-out")
    ap.add_argument("-cue", default="")
    ap.add_argument("-schema", default="schema/plan/schema.cue")
    ap.add_argument("-work", default="")
    ap.add_argument("-ctags", default="")
    ap.add_argument("-symbols-out", default="")
    ap.add_argument("-summary-out", default="")
    ap.add_argument("-compare", default="")
    ap.add_argument("-symbols", default="")
    a = ap.parse_args()
    if a.compare:
        compare(a.compare, a.symbols)
        return
    subject = os.path.abspath(a.subject)
    plans, defs, refs, xrefs, anchors = definitions_and_refs(subject, False)
    summary = {"plans": len(plans), "rows": sum(len(r) for r in plans.values()), "rows_with_id": len(defs),
               "references": len(refs), "cross_references": len(xrefs), "rows_with_anchor": len(anchors)}
    if a.cue:
        ok = bad = 0
        for f, rows in plans.items():
            tmp = os.path.join(a.work or os.path.dirname(os.path.abspath(a.out)), "plan-vet.json")
            json.dump(plan_json(f, rows), open(tmp, "w"))
            p = subprocess.run([a.cue, "vet", "-d", "#Plan", a.schema, tmp], capture_output=True, text=True)
            if p.returncode == 0:
                ok += 1
            else:
                bad += 1
                print("cue vet #Plan failed: %s: %s" % (f, (p.stderr.strip().splitlines() or [""])[0]), file=sys.stderr)
            os.remove(tmp)
        summary["cue_vet_plan_valid"], summary["cue_vet_plan_failed"] = ok, bad
    if a.ctags:
        tags = ctags_defs(subject, a.ctags)
        truth = symbol_truth(anchors, tags)
        summary["anchors"] = len(truth)
        summary["anchors_one_file"] = sum(1 for t in truth if len(t["files"]) == 1)
        summary["anchors_ambiguous"] = sum(1 for t in truth if len(t["files"]) > 1)
        summary["anchors_absent"] = sum(1 for t in truth if not t["files"])
        if a.symbols_out:
            json.dump(truth, open(a.symbols_out, "w"), indent=1)
    out = {"tool": TOOL, "version": "python " + sys.version.split()[0], "tier": "C",
           "definitions": defs, "references": refs, "cross_references": xrefs}
    json.dump(out, open(a.out, "w"), indent=1)
    if a.summary_out:
        json.dump(summary, open(a.summary_out, "w"), indent=1)
    print("plan-rows oracle: " + json.dumps(summary), file=sys.stderr)


if __name__ == "__main__":
    main()
