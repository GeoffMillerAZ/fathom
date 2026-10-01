"""cfn_oracle.py - the language-breadth harness's oracle for the cloudformation
doc-engine dialect (design section 1.3, 13.4 step 19; LB-Q-cloudformation).

Run it with cfn-lint's own interpreter, so the library is the installed tool's:

    <cfn-lint libexec>/bin/python cfn_oracle.py -subject <checkout> -out <file> \
        [-xref-out <file>] [-files-out <file>] [-summary-out <file>]

Tier A (-out): cfn-lint 1.52.1, AWS's CloudFormation linter, as the format's
reference parser. `cfnlint.decode.decode(path)` reads every .yaml, .yml and .json
of the subject the way cfn-lint reads a template (its YAML loader turns each short
form, `!Ref X`, `!GetAtt A.B`, `!Sub s`, into its long form, and marks every node
with its source line); a file it cannot decode is counted with its error and left
out. A decoded file is a template when its root is a mapping with a `Resources`
mapping and it names AWSTemplateFormatVersion, an `AWS::` Transform, or a
resource whose Type is a Namespace::Service name. Emitted:

  - definitions: a `package` per template (named by the file, at its root
    mapping's first line), and a `variable_decl` per Parameters entry, a `table`
    per Mappings entry, a `rule` per Conditions entry, a `resource` per Resources
    entry and an `output` per Outputs entry whose value is a mapping, each at its
    key's line;
  - references, from each resource and output: every `Ref` under it that names a
    parameter or a resource of the template, every `Fn::GetAtt` under it whose
    resource ([A, B] or "A.B") is one, a resource's `DependsOn` (a string or a
    list) naming resources, a `Condition` attribute naming a condition, and the
    condition every `Fn::If` under it names first. A pseudo parameter
    (AWS::Region) and a name the template never declares are not the template's
    entities. `Fn::Sub` references are not in the truth: the dialect does not
    read a Sub string's ${} names.

The truth is cross-checked against the tool's own dependency graph,
`cfnlint.template.Template(...).graph` (Ref, GetAtt and DependsOn edges between
resources and outputs, and the ${} names of a Sub; it omits parameter Refs and
conditions): every graph edge is either in the truth or comes from a Fn::Sub
(-summary-out and stderr count both, and any other edge).

Tier C (-xref-out, written only when there is one): cfn-lint does not resolve a
nested template, so a nested stack's TemplateURL (AWS::CloudFormation::Stack)
and a serverless application's Location (AWS::Serverless::Application) that is a
relative local path are resolved here, from the template's directory (a leading
/ from the subject's root), to a file of the subject that is itself a template:
cross_references at file granularity.

Directories the connector never reads (testdata, vendor, node_modules,
generated, .git and the editor directories) are not walked. Bench-only; never a
product dependency.
"""

import argparse
import json
import os
import posixpath
import re
import sys

import cfnlint.decode
import cfnlint.template
import cfnlint.version

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
EXTS = {".yaml", ".yml", ".json"}
TYPE_RE = re.compile(r"^[A-Za-z0-9]+::[A-Za-z0-9]+")
SUB_RE = re.compile(r"\$\{([a-zA-Z0-9.]*)\}")
URL_RE = re.compile(r"^[A-Za-z][A-Za-z0-9+.-]*://")
SECTIONS = [("Parameters", "variable_decl"), ("Mappings", "table"), ("Conditions", "rule"),
            ("Resources", "resource"), ("Outputs", "output")]


def line_of(node, fallback=0):
    mark = getattr(node, "start_mark", None)
    return mark.line + 1 if mark is not None else fallback


def is_template(tpl):
    if not isinstance(tpl, dict) or not isinstance(tpl.get("Resources"), dict):
        return False
    if "AWSTemplateFormatVersion" in tpl:
        return True
    tr = tpl.get("Transform")
    for t in tr if isinstance(tr, list) else [tr]:
        if isinstance(t, str) and t.startswith("AWS::"):
            return True
        if isinstance(t, dict) and str(t.get("Name", "")).startswith("AWS::"):
            return True
    for v in tpl["Resources"].values():
        if isinstance(v, dict) and isinstance(v.get("Type"), str) and TYPE_RE.match(v["Type"]):
            return True
    return False


def entries(tpl, section):
    sec = tpl.get(section)
    if not isinstance(sec, dict):
        return []
    return [(k, v) for k, v in sec.items() if isinstance(k, str) and isinstance(v, dict)]


def walk(node, visit):
    if isinstance(node, dict):
        for k, v in node.items():
            visit(k, v)
            walk(v, visit)
    elif isinstance(node, list):
        for it in node:
            walk(it, visit)


def references(tpl):
    """The truth pairs (source, target) of one template."""
    params = {k for k, _ in entries(tpl, "Parameters")}
    resources = {k for k, _ in entries(tpl, "Resources")}
    conditions = {k for k, _ in entries(tpl, "Conditions")}
    pairs = set()
    for section in ("Resources", "Outputs"):
        for name, body in entries(tpl, section):
            def visit(k, v, name=name):
                if k == "Ref" and isinstance(v, str) and (v in params or v in resources):
                    pairs.add((name, str(v)))
                elif k == "Fn::GetAtt":
                    t = None
                    if isinstance(v, list) and v and isinstance(v[0], str):
                        t = v[0]
                    elif isinstance(v, str):
                        t = v.split(".", 1)[0]
                    if t in resources:
                        pairs.add((name, str(t)))
                elif k == "Fn::If" and isinstance(v, list) and v and isinstance(v[0], str) and v[0] in conditions:
                    pairs.add((name, str(v[0])))
            walk(body, visit)
            if section == "Resources":
                dep = body.get("DependsOn")
                for d in dep if isinstance(dep, list) else [dep]:
                    if isinstance(d, str) and d in resources:
                        pairs.add((name, str(d)))
            cond = body.get("Condition")
            if isinstance(cond, str) and cond in conditions:
                pairs.add((name, str(cond)))
    return pairs


def sub_pairs(tpl):
    """The resource targets the graph draws from a Fn::Sub (its own rule)."""
    resources = {k for k, _ in entries(tpl, "Resources")}
    pairs = set()
    for section in ("Resources", "Outputs"):
        for name, body in entries(tpl, section):
            def visit(k, v, name=name):
                if k != "Fn::Sub":
                    return
                s, names = v, {}
                if isinstance(v, list) and v:
                    s = v[0]
                    if len(v) == 2 and isinstance(v[1], dict):
                        names = v[1]
                if not isinstance(s, str):
                    return
                for tok in SUB_RE.findall(s):
                    if tok in names:
                        continue
                    t = tok.split(".")[0] if "." in tok else tok
                    if t in resources:
                        pairs.add((name, t))
            walk(body, visit)
    return pairs


def graph_pairs(path, tpl):
    t = cfnlint.template.Template(path, tpl, ["us-east-1"])
    if t.graph is None:
        return None
    out = set()
    for u, v, _ in t.graph.graph.edges(data=True):
        if u.startswith("Parameter-"):
            continue
        if u.startswith("Output-"):
            u = u[len("Output-"):]
        out.add((u, v))
    return out


def find_files(root):
    found = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        for f in sorted(filenames):
            if os.path.splitext(f)[1].lower() in EXTS:
                found.append(os.path.relpath(os.path.join(dirpath, f), root).replace(os.sep, "/"))
    return sorted(found)


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-xref-out", default="")
    ap.add_argument("-files-out", default="")
    ap.add_argument("-summary-out", default="")
    a = ap.parse_args(argv)
    root = os.path.abspath(a.subject)
    defs, refs, xrefs, templates = [], [], [], {}
    rejected = {}
    stats = {"files": 0, "templates": 0, "not_templates": 0, "rejected": 0, "graph_edges": 0,
             "graph_edges_in_truth": 0, "graph_edges_from_sub": 0, "graph_edges_unexplained": 0,
             "graph_unavailable": 0, "truth_pairs": 0, "truth_resource_pairs_not_in_graph": 0}
    unexplained = []
    for rel in find_files(root):
        stats["files"] += 1
        path = os.path.join(root, rel)
        try:
            tpl, matches = cfnlint.decode.decode(path)
        except Exception as e:  # pylint: disable=broad-except
            tpl, matches = None, [str(e)]
        if matches or tpl is None:
            why = str(matches[0].message if hasattr(matches[0], "message") else matches[0]) if matches else "no document"
            why = why.split("\n")[0][:120]
            rejected.setdefault(why, []).append(rel)
            stats["rejected"] += 1
            continue
        if not is_template(tpl):
            stats["not_templates"] += 1
            continue
        stats["templates"] += 1
        templates[rel] = tpl
        defs.append({"path": rel, "name": posixpath.basename(rel), "line": line_of(tpl, 1), "kind": "package"})
        for section, kind in SECTIONS:
            for k, _ in entries(tpl, section):
                defs.append({"path": rel, "name": str(k), "line": line_of(k), "kind": kind})
        truth = references(tpl)
        stats["truth_pairs"] += len(truth)
        for src, dst in sorted(truth):
            refs.append({"path": rel, "from": src, "to": dst})
        try:
            graph = graph_pairs(path, tpl)
        except Exception:  # pylint: disable=broad-except
            graph = None
        if graph is None:
            stats["graph_unavailable"] += 1
        else:
            resources = {k for k, _ in entries(tpl, "Resources")}
            subs = sub_pairs(tpl)
            stats["graph_edges"] += len(graph)
            for e in sorted(graph):
                if e in truth:
                    stats["graph_edges_in_truth"] += 1
                elif e in subs:
                    stats["graph_edges_from_sub"] += 1
                else:
                    stats["graph_edges_unexplained"] += 1
                    unexplained.append(f"{rel}: {e[0]} -> {e[1]}")
            stats["truth_resource_pairs_not_in_graph"] += len({p for p in truth if p[1] in resources} - graph)
    for rel, tpl in sorted(templates.items()):
        for name, body in entries(tpl, "Resources"):
            typ = body.get("Type")
            props = body.get("Properties") if isinstance(body.get("Properties"), dict) else {}
            raw = props.get("TemplateURL") if typ == "AWS::CloudFormation::Stack" else (
                props.get("Location") if typ == "AWS::Serverless::Application" else None)
            if not isinstance(raw, str) or URL_RE.match(raw) or "${" in raw:
                continue
            raw = raw.replace("\\", "/")
            target = posixpath.normpath(raw.lstrip("/") if raw.startswith("/") else posixpath.join(posixpath.dirname(rel), raw))
            if target.startswith("..") or target not in templates or target == rel:
                continue
            xrefs.append({"path": rel, "from": str(name), "to_path": target})
    version = cfnlint.version.__version__
    with open(a.out, "w") as f:
        json.dump({"tool": "cfn-lint", "version": version, "tier": "A",
                   "definitions": sorted(defs, key=lambda d: (d["path"], d["line"], d["name"])),
                   "references": refs}, f, indent=1)
        f.write("\n")
    if a.xref_out and xrefs:
        with open(a.xref_out, "w") as f:
            json.dump({"tool": "cfn-lint (local nested templates resolved by the adapter)", "version": version,
                       "tier": "C", "cross_references": xrefs}, f, indent=1)
            f.write("\n")
    if a.files_out:
        with open(a.files_out, "w") as f:
            f.write("".join(r + "\n" for r in sorted(templates)))
    stats["cross_references"] = len(xrefs)
    if a.summary_out:
        with open(a.summary_out, "w") as f:
            json.dump({"stats": stats, "rejected": rejected, "unexplained_graph_edges": unexplained}, f, indent=1, sort_keys=True)
            f.write("\n")
    print("cfn-oracle: " + ", ".join(f"{k} {v}" for k, v in stats.items()), file=sys.stderr)
    for why in sorted(rejected):
        print(f"  rejected ({why}): {len(rejected[why])} file(s), e.g. {rejected[why][0]}", file=sys.stderr)
    for u in unexplained[:10]:
        print(f"  graph edge neither in the truth nor from a Sub: {u}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
