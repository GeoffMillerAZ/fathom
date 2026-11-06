#!/usr/bin/env python3
"""grade_reads.py - join an oracle report with a reader dump and grade it (LL-M3, LL-M4, LL-M5).

usage: grade_reads.py --tool {terraform,cloudformation,pulumi-yaml} --subjects subjects/<tool>.json --clones <dir>
                      --oracle <oracle.json> [--schema <schema.json>] --reader <reader.json> --json <out.json> [--md <out.md>]

The mapping and the metrics are the ones README.md fixes before any reading. Nothing here is tuned to the readers.
"""
import argparse, json, os, re, sys
from collections import defaultdict

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from select_subjects import clone_dir  # noqa: E402  (path helper only)

BAR = {"precision": 0.99, "recall": 0.95, "agreement": 0.98}


# ---------- HCL: where the dynamic blocks are (text only, no reader code) ----------

def skip_string(t, i):
    """t[i] is a quote. Return the index after the closing quote, stepping over ${ } template expressions."""
    n = len(t)
    i += 1
    while i < n:
        c = t[i]
        if c == "\\":
            i += 2
        elif c == '"':
            return i + 1
        elif c in "$%" and t.startswith("{", i + 1):
            i = skip_braces(t, i + 1)
        else:
            i += 1
    return n


def skip_braces(t, i):
    """t[i] is '{'. Return the index after its matching '}'."""
    n, depth = len(t), 0
    while i < n:
        c = t[i]
        if c == '"':
            i = skip_string(t, i)
            continue
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return i + 1
        i += 1
    return n


HEREDOC = re.compile(r"<<-?([A-Za-z_][A-Za-z0-9_]*)[ \t]*\r?\n")


def dynamic_ranges(t):
    out, stack = [], []
    n, i, line = len(t), 0, 1
    header_start = 0
    while i < n:
        c = t[i]
        if c == "\n":
            line += 1
            header_start = i + 1
            i += 1
        elif c == "#" or t.startswith("//", i):
            while i < n and t[i] != "\n":
                i += 1
        elif t.startswith("/*", i):
            j = t.find("*/", i + 2)
            j = n if j < 0 else j + 2
            line += t.count("\n", i, j)
            i = j
        elif c == '"':
            j = skip_string(t, i)
            line += t.count("\n", i, j)
            i = j
        elif c == "<" and t.startswith("<<", i) and HEREDOC.match(t, i):
            m = HEREDOC.match(t, i)
            tag = m.group(1)
            j = m.end()
            line += 1
            while j < n:
                e = t.find("\n", j)
                e = n if e < 0 else e
                ln = t[j:e].strip()
                j = e + 1
                line += 1
                if ln == tag:
                    break
            i = j
            header_start = i
        elif c == "{":
            hdr = t[header_start:i]
            stack.append((re.match(r'^\s*dynamic\s+"[^"]*"\s*$', hdr) is not None, line))
            i += 1
        elif c == "}":
            if stack:
                dyn, start = stack.pop()
                if dyn:
                    out.append((start, line))
            i += 1
        else:
            i += 1
    return out


# ---------- Terraform schema walk ----------

def in_schema(path, blk):
    segs = path.split(".")
    for k, seg in enumerate(segs):
        if seg.endswith("[*]"):
            name = seg[:-3]
            if name not in blk["blocks"]:
                return False
            blk = blk["blocks"][name]
        else:
            return k == len(segs) - 1 and seg in blk["attrs"]
    return False


def rhs_text(lines, start):
    """The written expression of the attribute that starts at 1-based line `start` (to the end of its brackets)."""
    text, depth = "", 0
    for ln in lines[start - 1 : start + 40]:
        text += ln + "\n"
        s = re.sub(r'"(\\.|[^"\\])*"', '""', ln)
        depth += sum(s.count(x) for x in "([{") - sum(s.count(x) for x in ")]}")
        if depth <= 0:
            break
    return text.split("=", 1)[1] if "=" in text else text


FOLD_OPS = re.compile(r"(\s(\+|-|\*|/|%|&&|\|\||==|!=|<=|>=|<|>)\s)")


def folded_form(rhs):
    s = re.sub(r'"(\\.|[^"\\])*"', '""', rhs).strip()
    if s.startswith("!") or s.startswith("-"):
        return True
    if re.match(r"^[\[{]\s*for\b", s):
        return True
    if "?" in s and ":" in s:
        return True
    return FOLD_OPS.search(s) is not None


# ---------- class mapping ----------

def tf_var_refs(refs, variables):
    """(kind, ok): kind is 'locals' when a local is referenced, else 'vars'; ok when every name is a variable with a default."""
    names = set()
    for r in refs:
        if r.startswith("local."):
            return "locals", False
        if r.startswith("var."):
            names.add(r.split(".")[1].split("[")[0])
        else:
            return "vars", False
    return "vars", bool(names) and all(variables.get(n, {}).get("has_default") for n in names)


def agree(tool, rc, oc, ctx):
    """Does a reader class agree with an oracle leaf? Returns True, False or None (ungraded)."""
    c = oc["class"]
    if rc == "literal":
        return c == "constant"
    if rc == "reference":
        return c == "references"
    if rc == "defaulted":
        if c != "references":
            return False
        if tool == "terraform":
            kind, ok = tf_var_refs(oc["refs"], ctx["variables"])
            return None if kind == "locals" else ok
        return bool(oc.get("defaultable"))
    if rc == "expression":
        if c in ("references", "empty"):
            return True
        if c == "constant" and tool == "terraform":
            if folded_form(ctx.get("rhs", "")):
                ctx["folded"] = True
                return True
            return False
        return False
    if rc == "unknown":
        return c == "unresolved"
    return False

TOOL_NAME = {"terraform": "Terraform", "cloudformation": "CloudFormation", "pulumi-yaml": "Pulumi YAML"}
UNIT = {"terraform": "root", "cloudformation": "template", "pulumi-yaml": "program"}


def render(res, sf, row):
    t = res["tool"]
    c = res["counts"]
    ex = res["excluded"]
    name, unit = TOOL_NAME[t], UNIT[t]
    verdict = "PASS" if res["pass"] else "MISS"
    out = []
    out.append("%s verdict: %s" % (row, verdict))
    out.append("")
    out.append("%s resource reads against %s, on %d preregistered public %ss (%d resources, %d reader paths). Path precision %.4f (bar %.2f), path recall %.4f (bar %.2f), class agreement %.4f (bar %.2f)." % (
        name, res["oracle"], res["roots_graded"], unit, c.get("resources_reader", 0), c.get("reader_paths", 0),
        res["precision"], BAR["precision"], res["recall"], BAR["recall"], res["agreement"], BAR["agreement"]))
    out.append("")
    out.append("## Reading")
    out.append("")
    out.append("| Item | Value |")
    out.append("|---|---|")
    out.append("| Oracle | %s |" % res["oracle"])
    out.append("| Reader | `%s` (`internal/iacfacts`) |" % {"terraform": "ReadTerraformModule", "cloudformation": "ReadCloudFormationFile", "pulumi-yaml": "ReadPulumiYAMLFile"}[t])
    out.append("| Subjects | `calibration/oracles/lang-library/subjects/%s.json`, committed before any oracle or reader ran on them (selection rule inside) |" % t)
    out.append("| %ss selected, graded, rejected by the oracle | %d, %d, %d |" % (unit.capitalize(), len(sf["selected"]), res["roots_graded"], res["roots_rejected"]))
    out.append("| Resources in oracle, in reader | %d, %d |" % (c.get("resources_oracle", 0), c.get("resources_reader", 0)))
    out.append("| Graded paths: reader, oracle, in both | %d, %d, %d |" % (c.get("reader_paths", 0), c.get("oracle_paths", 0), c.get("matched_paths", 0)))
    out.append("| Graded class leaves, agreeing | %d, %d |" % (c.get("class_graded", 0), c.get("class_agree", 0)))
    out.append("| Reader syntax errors reported | %d |" % c.get("syntax_errors", 0))
    out.append("")
    out.append("## Against the bar")
    out.append("")
    out.append("| Metric | Bar | Read | Count |")
    out.append("|---|---|---|---|")
    out.append("| Path precision | at least %.2f | %.4f | %d of %d reader paths are in the oracle |" % (BAR["precision"], res["precision"], c.get("matched_paths", 0), c.get("reader_paths", 0)))
    out.append("| Path recall | at least %.2f | %.4f | %d of %d oracle paths are in the reader |" % (BAR["recall"], res["recall"], c.get("matched_paths", 0), c.get("oracle_paths", 0)))
    out.append("| Class agreement | at least %.2f | %.4f | %d of %d leaves in both agree |" % (BAR["agreement"], res["agreement"], c.get("class_agree", 0), c.get("class_graded", 0)))
    out.append("")
    out.append("Agreement by reader class:")
    out.append("")
    out.append("| Reader class | Graded | Agree |")
    out.append("|---|---|---|")
    for k, v in res["by_reader_class"].items():
        out.append("| %s | %d | %d |" % (k, v.get("graded", 0), v.get("agree", 0)))
    out.append("")
    out.append("## Subjects")
    out.append("")
    out.append("| Repository | Pin | %ss | Paths (reader, oracle) | Leaves graded, agree |" % unit.capitalize())
    out.append("|---|---|---|---|---|")
    pins = {r["repo"]: r["sha"][:12] for r in sf["repos"]}
    for repo, v in res["per_repo"].items():
        out.append("| %s | `%s` | %d | %d, %d | %d, %d |" % (repo, pins.get(repo, ""), v.get("roots", 0), v.get("reader_paths", 0), v.get("oracle_paths", 0), v.get("class_graded", 0), v.get("class_agree", 0)))
    out.append("")
    out.append("## Not graded, counted")
    out.append("")
    if t == "terraform":
        out.append("| Item | Count |")
        out.append("|---|---|")
        out.append("| Reader attributes inside a `dynamic` block (tofu drops dynamic blocks) | %d |" % ex.get("dynamic", 0))
        out.append("| Reader attributes under `lifecycle` | %d |" % ex.get("lifecycle", 0))
        out.append("| Reader attributes outside the provider schema (`depends_on`) | %d |" % ex.get("out_of_schema", 0))
        out.append("| Module calls (reader, oracle) | %d, %d |" % (ex.get("module_calls_reader", 0), ex.get("module_calls_oracle", 0)))
        out.append("| Defaulted leaves through a local (ungraded) | %d |" % ex.get("locals_ungraded", 0))
        out.append("| Graded leaves whose oracle references include a local (in the agreement figure) | %d, of which %d disagree |" % (c.get("leaves_through_locals", 0), c.get("leaves_through_locals_disagree", 0)))
        out.append("| Expression leaves that agree only because tofu folded them to a constant | %d |" % c.get("expression_agreed_through_folding", 0))
    else:
        out.append("Nothing leaves the denominators on %s. Every reader and oracle path is graded." % name)
    out.append("")
    out.append("## Disagreement classes")
    out.append("")
    if res["disagreements"]:
        out.append("| Class | Count | One example (subject, resource, path) |")
        out.append("|---|---|---|")
        for k, v in res["disagreements"].items():
            out.append("| %s | %d | %s |" % (k, v["n"], "`%s`" % v["example"]))
    else:
        out.append("None.")
    out.append("")
    out.append("## Rejected by the oracle")
    out.append("")
    if res["rejected"]:
        out.append("| %s | Reason |" % unit.capitalize())
        out.append("|---|---|")
        for r in res["rejected"]:
            out.append("| `%s` | %s |" % (r["root"], r["reason"].replace("|", "/")))
    else:
        out.append("None.")
    if res["reader_failed"]:
        out.append("")
        out.append("Refused by the reader: %s." % ", ".join("`%s`" % r["root"] for r in res["reader_failed"]))
    out.append("")
    return "\n".join(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tool", required=True)
    ap.add_argument("--subjects", required=True)
    ap.add_argument("--clones", required=True)
    ap.add_argument("--oracle", required=True)
    ap.add_argument("--schema")
    ap.add_argument("--reader", required=True)
    ap.add_argument("--json", required=True)
    ap.add_argument("--md")
    ap.add_argument("--row", help="plan row id for the verdict line, such as LL-M3")
    a = ap.parse_args()
    tool = a.tool
    sf = json.load(open(a.subjects))
    orc = json.load(open(a.oracle))
    rdr = json.load(open(a.reader))
    schema = json.load(open(a.schema)) if a.schema else {}
    oroots = orc["roots"] if tool == "terraform" else orc.get("templates") or orc.get("programs")

    tot = defaultdict(int)
    excl = defaultdict(int)
    rejected, reader_failed = [], []
    dis = defaultdict(lambda: {"n": 0, "example": None})
    per_repo = defaultdict(lambda: defaultdict(int))
    by_class = defaultdict(lambda: defaultdict(int))
    src_cache = {}

    def source_lines(repo, f):
        k = (repo, f)
        if k not in src_cache:
            p = os.path.join(clone_dir(a.clones, tool, repo), f)
            try:
                src_cache[k] = open(p, encoding="utf-8", errors="replace").read()
            except OSError:
                src_cache[k] = ""
        return src_cache[k]

    def note(kind, example, n=1):
        d = dis[kind]
        d["n"] += n
        if d["example"] is None:
            d["example"] = example

    for s in sf["selected"]:
        key = s["repo"] + ":" + s["path"]
        o = oroots.get(key)
        if o is None or not o.get("ok"):
            rejected.append({"root": key, "reason": (o or {}).get("error", "no oracle record"), "stage": (o or {}).get("stage", "")})
            continue
        r = rdr["roots"][key]
        tot["roots"] += 1
        pr = per_repo[s["repo"]]
        pr["roots"] += 1
        ores = o["resources"]
        if tool == "terraform":
            variables = o["variables"]
            excl["module_calls_oracle"] += o["module_calls"]
        else:
            variables = {}
        if not r.get("ok"):
            reader_failed.append({"root": key, "error": r.get("error", "")})
            for rk, res in ores.items():
                leaves = res if tool == "terraform" else res["leaves"]
                tot["oracle_paths"] += len(leaves)
                pr["oracle_paths"] += len(leaves)
                note("recall miss: reader refused the input", "%s %s" % (key, rk), len(leaves))
            continue
        rres = {}
        for x in r.get("resources") or []:
            if tool == "terraform" and x["mode"] not in ("managed", "data"):
                if x["mode"] == "module":
                    excl["module_calls_reader"] += 1
                else:
                    excl["other_mode_" + x["mode"]] += 1
                continue
            rres[x["key"]] = x
        tot["syntax_errors"] += r.get("syntaxErrors", 0)
        # dynamic ranges, once per file
        dyn = {}
        if tool == "terraform":
            files = {at["file"] for x in rres.values() for at in (x["attrs"] or [])}
            for f in files:
                dyn[f] = dynamic_ranges(source_lines(s["repo"], f))
        for rk in sorted(set(ores) | set(rres)):
            res_o = ores.get(rk)
            res_r = rres.get(rk)
            o_leaves = {}
            if res_o is not None:
                o_leaves = res_o if tool == "terraform" else res_o["leaves"]
            tot["resources_oracle"] += 1 if res_o is not None else 0
            tot["resources_reader"] += 1 if res_r is not None else 0
            r_paths = defaultdict(list)  # path -> [attr]
            if res_r is not None:
                tf_blk = None
                if tool == "terraform":
                    mode, typ, _ = rk.split(".", 2)
                    tf_blk = schema.get(mode + "." + typ)
                for at in (res_r["attrs"] or []):
                    p = at["path"]
                    if tool == "terraform":
                        if any(lo <= at["line"] <= hi for lo, hi in dyn.get(at["file"], [])):
                            excl["dynamic"] += 1
                            continue
                        if p == "lifecycle" or p.startswith("lifecycle."):
                            excl["lifecycle"] += 1
                            continue
                        if tf_blk is None or not in_schema(p, tf_blk):
                            excl["out_of_schema"] += 1
                            continue
                    r_paths[p].append(at)
            rset, oset = set(r_paths), set(o_leaves)
            matched = rset & oset
            tot["reader_paths"] += len(rset)
            tot["oracle_paths"] += len(oset)
            tot["matched_paths"] += len(matched)
            pr["reader_paths"] += len(rset)
            pr["oracle_paths"] += len(oset)
            pr["matched_paths"] += len(matched)
            if res_r is None and res_o is not None:
                dropped = rk in (r.get("dropped") or [])
                note("recall miss: resource not read%s" % (" (named in Dropped)" if dropped else ""), "%s %s" % (key, rk), len(oset))
            elif res_o is None and res_r is not None:
                note("precision miss: resource the oracle does not have", "%s %s" % (key, rk), len(rset))
            else:
                for p in sorted(rset - oset):
                    cl = r_paths[p][0]["class"]
                    note("precision miss: reader path the oracle lacks (reader class %s)" % cl, "%s %s %s" % (key, rk, p))
                for p in sorted(oset - rset):
                    oc = sorted({x["class"] for x in o_leaves[p]})
                    note("recall miss: oracle path the reader lacks (oracle class %s)" % "/".join(oc), "%s %s %s" % (key, rk, p))
            for p in sorted(matched):
                for at in {x["class"]: x for x in r_paths[p]}.values():
                    ctx = {"variables": variables}
                    if tool == "terraform" and at["class"] == "expression":
                        lines = source_lines(s["repo"], at["file"]).splitlines()
                        ctx["rhs"] = rhs_text(lines, at["line"])
                    verdicts = [agree(tool, at["class"], oc, ctx) for oc in o_leaves[p]]
                    if all(v is None for v in verdicts):
                        excl["locals_ungraded"] += 1
                        continue
                    ok = any(v for v in verdicts)
                    if ctx.get("folded") and ok:
                        tot["expression_agreed_through_folding"] += 1
                    tot["class_graded"] += 1
                    pr["class_graded"] += 1
                    bc = by_class[at["class"]]
                    bc["graded"] += 1
                    has_local = tool == "terraform" and any(x.startswith("local.") for oc in o_leaves[p] for x in oc["refs"])
                    if has_local:
                        tot["leaves_through_locals"] += 1
                    if ok:
                        tot["class_agree"] += 1
                        pr["class_agree"] += 1
                        bc["agree"] += 1
                    else:
                        if has_local:
                            tot["leaves_through_locals_disagree"] += 1
                        oc = "/".join(sorted({x["class"] for x in o_leaves[p]}))
                        note("class disagreement: reader %s, oracle %s" % (at["class"], oc), "%s %s %s" % (key, rk, p or "(whole properties)"))

    prec = tot["matched_paths"] / tot["reader_paths"] if tot["reader_paths"] else 0.0
    rec = tot["matched_paths"] / tot["oracle_paths"] if tot["oracle_paths"] else 0.0
    agr = tot["class_agree"] / tot["class_graded"] if tot["class_graded"] else 0.0
    res = {
        "tool": tool, "oracle": orc.get("oracle"), "reader": rdr.get("reader"),
        "roots_graded": tot["roots"], "roots_rejected": len(rejected), "rejected": rejected, "reader_failed": reader_failed,
        "precision": prec, "recall": rec, "agreement": agr,
        "counts": dict(tot), "excluded": dict(excl),
        "disagreements": {k: v for k, v in sorted(dis.items(), key=lambda kv: -kv[1]["n"])},
        "by_reader_class": {k: dict(v) for k, v in sorted(by_class.items())},
        "per_repo": {k: dict(v) for k, v in sorted(per_repo.items())},
        "pass": prec >= BAR["precision"] and rec >= BAR["recall"] and agr >= BAR["agreement"],
    }
    json.dump(res, open(a.json, "w"), indent=1, sort_keys=False)
    if a.md:
        open(a.md, "w").write(render(res, sf, a.row or "LL-M?"))
    print("%s: roots %d graded, %d rejected; precision %.4f recall %.4f agreement %.4f (%s)" % (
        tool, tot["roots"], len(rejected), prec, rec, agr, "PASS" if res["pass"] else "MISS"))


if __name__ == "__main__":
    main()
