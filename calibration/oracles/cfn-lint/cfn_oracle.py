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
    entities. `Fn::Sub` references are not in the truth (the dialect does not
    read a Sub string's ${} names) except the CloudWatch ones below.

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

The CloudWatch Sub references (LB-F368) are in the truth: a Fn::Sub's ${X} or
${X.attr} naming a resource of the template (not a name its variable map binds,
nor the resource itself), written at an alarm's Dimensions[].Value (and each
Metrics[].MetricStat.Metric's), its AlarmActions, OKActions or
InsufficientDataActions entries, or a LogGroupName: cfn-lint's graph draws
every one of them (it reads every Sub), so they are tier A and the cross-check
still explains every graph edge.

Tier C (-xref-out) also holds, since wave c3 (LB-F369, LB-F370), every
reference carrying "to" (the referenced entity, so the comparison is at entity
granularity) and "edge":

  - "nested_stack" / "nested_application": as above, to the child template's
    own entity (its file name);
  - "metric" and "log_group", the identity joins: a resource whose Properties
    (or a Metrics[].MetricStat.Metric) name a literal Namespace and MetricName
    joins the metric a metric filter's MetricTransformations declare
    (`<namespace>/<name>`, literal, the first declaration in its file), and a
    literal LogGroupName /aws/lambda/<name> joins the function whose literal
    FunctionName is <name>; the nearest declarations only (the referring
    file's own, else those under the deepest directory enclosing it that holds
    one), never the referring entity itself, none past 64;
  - "handler": a Lambda function's handler symbol, by the runtimes' own rules
    (python `pkg.module.func` is def func in <code>/pkg/module.py or
    pkg/module/__init__.py; nodejs `dir/file.export` the export in
    <code>/dir/file.js, .mjs or .cjs, a SAM esbuild EntryPoints entry, or a
    .ts, .mts, .cts there; java `pkg.Class::method` (bare: handleRequest) in
    <code>/src/main/java/pkg/Class.java, else the one pkg/Class.java under
    <code>; ruby `file.method` in <code>/file.rb), <code> being SAM's CodeUri
    (Globals.Function default, else the template's directory), an
    AWS::Lambda::Function's local Code path, and Runtime and Handler
    defaulting from Globals.Function, and a Runtime that is no literal read
    as the first of python, nodejs and ruby whose rule names a file of the
    subject; an image, inline code, an S3 or
    artifact location and a value computed at deploy time bind nothing. The
    symbol is confirmed in the file independently of fathom's extractors:
    Python's own `ast` (a top-level def or assignment), and, tier C, a line
    pattern for an exported JavaScript or TypeScript function, a Java method
    declaration and a Ruby def. It is named as fathom names it (Class.method
    for Java).

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
                pairs |= cw_sub_pairs(name, body, resources)
                dep = body.get("DependsOn")
                for d in dep if isinstance(dep, list) else [dep]:
                    if isinstance(d, str) and d in resources:
                        pairs.add((name, str(d)))
            cond = body.get("Condition")
            if isinstance(cond, str) and cond in conditions:
                pairs.add((name, str(cond)))
    return pairs


# The Properties paths whose Fn::Sub names are references (LB-F368).
CW_SUB_PATHS = [("Dimensions", "*", "Value"), ("Metrics", "*", "MetricStat", "Metric", "Dimensions", "*", "Value"),
                ("AlarmActions", "*"), ("OKActions", "*"), ("InsufficientDataActions", "*"), ("LogGroupName",)]


def at_path(node, steps):
    """Every value a Properties path reaches ("*" is every item of a list; a
    single value reads as its own one item)."""
    cur = [node]
    for st in steps:
        nxt = []
        for n in cur:
            if st == "*":
                nxt.extend(n if isinstance(n, list) else ([n] if n is not None else []))
            elif isinstance(n, dict) and st in n:
                nxt.append(n[st])
        cur = nxt
    return cur


def cw_sub_pairs(name, body, resources):
    """A resource's CloudWatch Sub references: the ${X} or ${X.attr} of a
    Fn::Sub at CW_SUB_PATHS naming a declared resource other than itself."""
    props = body.get("Properties") if isinstance(body.get("Properties"), dict) else {}
    pairs = set()
    for steps in CW_SUB_PATHS:
        for v in at_path(props, steps):
            if not isinstance(v, dict) or "Fn::Sub" not in v:
                continue
            sub, names = v["Fn::Sub"], {}
            if isinstance(sub, list) and sub:
                if len(sub) == 2 and isinstance(sub[1], dict):
                    names = sub[1]
                sub = sub[0]
            if not isinstance(sub, str):
                continue
            for tok in SUB_RE.findall(sub):
                if tok in names:
                    continue
                t = tok.split(".")[0]
                if t in resources and t != name:
                    pairs.add((name, str(t)))
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


TEMPLATED = ("${", "{{", "$(")
LAMBDA_LOG_RE = re.compile(r"^/aws/lambda/([^/${}\s]+)$")
FANOUT_CAP = 64


def scalar(v):
    """A value written as a scalar, as a string ("" for a map, a list or none)."""
    if isinstance(v, (dict, list)) or v is None:
        return ""
    if isinstance(v, bool):
        return "true" if v else "false"
    return str(v)


def first_scalar(items, key):
    for it in items if isinstance(items, list) else [items]:
        if isinstance(it, dict):
            v = scalar(it.get(key))
            if v:
                return v
    return ""


def literal(v):
    v = scalar(v)
    return "" if any(m in v for m in TEMPLATED) else v


def resources_of(tpl):
    return [(k, v) for k, v in entries(tpl, "Resources")]


def identity_xrefs(templates):
    """The identity joins (LB-F369): see the module docstring."""
    signed = {}  # signature -> [(path, entity)]
    for rel, tpl in sorted(templates.items()):
        seen = set()
        for name, body in resources_of(tpl):
            typ = body.get("Type")
            props = body.get("Properties") if isinstance(body.get("Properties"), dict) else {}
            if typ == "AWS::Logs::MetricFilter":
                mts = props.get("MetricTransformations")
                ns, mn = first_scalar(mts, "MetricNamespace"), first_scalar(mts, "MetricName")
                ident = ns + "/" + mn
                if ns and mn and not any(m in ident for m in TEMPLATED) and ident not in seen:
                    seen.add(ident)
                    signed.setdefault("aws.cloudwatch.metric=" + ident, []).append((rel, ident))
            if typ in ("AWS::Lambda::Function", "AWS::Serverless::Function"):
                fn = literal(props.get("FunctionName"))
                if fn:
                    signed.setdefault("aws.lambda.function_name=" + fn, []).append((rel, fn))
    out, seen = [], set()
    for rel, tpl in sorted(templates.items()):
        for name, body in resources_of(tpl):
            props = body.get("Properties") if isinstance(body.get("Properties"), dict) else {}
            refs = []
            metrics = [props]
            ms = props.get("Metrics")
            for m in ms if isinstance(ms, list) else ([ms] if ms is not None else []):
                if isinstance(m, dict) and isinstance(m.get("MetricStat"), dict) and isinstance(m["MetricStat"].get("Metric"), dict):
                    metrics.append(m["MetricStat"]["Metric"])
            for m in metrics:
                ns, mn = scalar(m.get("Namespace")), scalar(m.get("MetricName"))
                if ns and mn:
                    refs.append(("metric", "aws.cloudwatch.metric=" + ns + "/" + mn))
            lg = LAMBDA_LOG_RE.match(scalar(props.get("LogGroupName")))
            if lg:
                refs.append(("log_group", "aws.lambda.function_name=" + lg.group(1)))
            for edge, sig in refs:
                cands = [t for t in signed.get(sig, []) if t != (rel, name)]
                for path, to in nearest(rel, cands) if len(nearest(rel, cands)) <= FANOUT_CAP else []:
                    k = (rel, name, path, to, edge)
                    if k not in seen:
                        seen.add(k)
                        out.append({"path": rel, "from": str(name), "to_path": path, "to": to, "edge": edge})
    return out


def nearest(rel, cands):
    """The candidates in rel itself, else under the deepest directory
    enclosing rel that holds any (the subject's root last)."""
    same = [c for c in cands if c[0] == rel]
    if same or not cands:
        return same
    d = posixpath.dirname(rel)
    while True:
        under = [c for c in cands if d == "" or c[0].startswith(d + "/")]
        if under or d == "":
            return under
        d = posixpath.dirname(d)


# ── Lambda handlers (LB-F370) ───────────────────────────────────────────────


def norm(p):
    p = posixpath.normpath(p)
    return None if p == ".." or p.startswith("../") else p


def code_dir(tdir, v, default):
    """A CodeUri's or Code's local directory, or (None, reason)."""
    if v is None:
        v = default
    if isinstance(v, dict):
        return None, "remote" if ("Bucket" in v or "S3Bucket" in v) else "templated"
    if isinstance(v, list):
        return None, "templated"
    v = scalar(v)
    low = v.lower()
    if not v:
        return None, "no-file"
    if "://" in v:
        return None, "remote"
    if any(m in v for m in TEMPLATED):
        return None, "templated"
    if low.endswith(".zip") or low.endswith(".jar"):
        return None, "artifact"
    p = norm(posixpath.join(tdir, v.replace("\\", "/")))
    return (p, "") if p else (None, "no-file")


def lambda_functions(rel, tpl):
    """(name, runtime, handler, code dir, entry points, unbound reason) per function."""
    glob = tpl.get("Globals") if isinstance(tpl.get("Globals"), dict) else {}
    g = glob.get("Function") if isinstance(glob.get("Function"), dict) else {}
    tdir = posixpath.dirname(rel)
    out = []
    for name, body in resources_of(tpl):
        typ = body.get("Type")
        props = body.get("Properties") if isinstance(body.get("Properties"), dict) else {}
        if typ == "AWS::Serverless::Function":
            def prop(k):
                return props[k] if k in props else g.get(k)
            runtime, handler = literal(prop("Runtime")), literal(prop("Handler"))
            eps, why, cdir = [], "", None
            if scalar(prop("PackageType")) == "Image" or "ImageUri" in props:
                why = "image"
            elif "InlineCode" in props:
                why = "inline"
            else:
                cdir, why = code_dir(tdir, prop("CodeUri"), ".")
            meta = body.get("Metadata") if isinstance(body.get("Metadata"), dict) else {}
            if not why and scalar(meta.get("BuildMethod")) == "esbuild":
                bp = meta.get("BuildProperties") if isinstance(meta.get("BuildProperties"), dict) else {}
                for e in bp.get("EntryPoints") or []:
                    if scalar(e) and norm(posixpath.join(cdir, scalar(e))):
                        eps.append(norm(posixpath.join(cdir, scalar(e))))
            hset = "Handler" in props or "Handler" in g
        elif typ == "AWS::Lambda::Function":
            runtime, handler = literal(props.get("Runtime")), literal(props.get("Handler"))
            code, eps, cdir, why = props.get("Code"), [], None, ""
            if scalar(props.get("PackageType")) == "Image" or (isinstance(code, dict) and "ImageUri" in code):
                why = "image"
            elif isinstance(code, dict) and "ZipFile" in code:
                why = "inline"
            elif code is None:
                why = "no-file"
            else:
                cdir, why = code_dir(tdir, code, "")
            hset = "Handler" in props
        else:
            continue
        if not why and not handler:
            why = "templated" if hset else "no-handler"
        out.append((str(name), runtime, handler, cdir, eps, why))
    return out


JS_EXPORT = r"(?:export\s+(?:default\s+)?(?:async\s+)?function\s*\*?\s*{n}\b|export\s+(?:const|let|var)\s+{n}\b|(?:module\.)?exports\.{n}\s*=|(?:^|[^.\w]){n}\s*[:=]\s*(?:async\s+)?(?:function\b|\([^)]*\)\s*(?::[^=]*)?=>|[A-Za-z_$][\w$]*\s*=>)|(?:^|[^.\w])(?:async\s+)?function\s+{n}\s*\()"


def has_symbol(path, family, name, cls=None):
    try:
        with open(path, encoding="utf-8", errors="replace") as f:
            src = f.read()
    except OSError:
        return False
    if family == "python":
        import ast
        try:
            mod = ast.parse(src)
        except SyntaxError:
            return False
        for n in mod.body:
            if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef)) and n.name == name:
                return True
            if isinstance(n, (ast.Assign, ast.AnnAssign)):
                targets = n.targets if isinstance(n, ast.Assign) else [n.target]
                if any(isinstance(t, ast.Name) and t.id == name for t in targets):
                    return True
        return False
    if family == "node":
        return re.search(JS_EXPORT.replace("{n}", re.escape(name)), src, re.M) is not None
    if family == "java":
        if not re.search(r"\b(?:class|record|enum)\s+" + re.escape(cls) + r"\b", src):
            return False
        return re.search(r"^[ \t]*(?:@\w+(?:\([^)]*\))?\s+)*(?:(?:public|protected|private|static|final|synchronized|abstract|default)\s+)*(?:<[^>]+>\s+)?[\w.<>\[\], ?]+\s+" + re.escape(name) + r"\s*\(", src, re.M) is not None
    if family == "ruby":
        return re.search(r"^\s*def\s+(?:self\.)?" + re.escape(name) + r"\b", src, re.M) is not None
    return False


def handler_target(root, files, fn):
    """The (file, symbol name) a function's handler names, or (None, reason)."""
    name, runtime, h, cdir, eps, why = fn
    if why:
        return None, why
    if not runtime:  # a Runtime that is no literal: read the family from the code
        runtime = infer_runtime(files, cdir, h)

    def first(cands, family, sym, cls=None):
        for c in cands:
            if c and c in files:
                return ((c, sym), "") if has_symbol(os.path.join(root, c), family, sym.split(".")[-1], cls) else (None, "no-symbol")
        return None, "no-file"
    if runtime.startswith("python"):
        if "." not in h.strip(".") or h.endswith("."):
            return None, "no-handler"
        mod, fn_name = h.rsplit(".", 1)
        mod = mod.replace(".", "/")
        return first([norm(posixpath.join(cdir, mod + ".py")), norm(posixpath.join(cdir, mod, "__init__.py"))], "python", fn_name)
    if runtime.startswith("nodejs"):
        base = posixpath.basename(h)
        rootp = h[:len(h) - len(base)]
        if "." not in base or base.startswith(".") or base.endswith("."):
            return None, "no-handler"
        module, export = base.split(".", 1)
        export = export.rsplit(".", 1)[-1]
        module = rootp + module
        cands = [e for e in eps if posixpath.splitext(e)[0] == norm(posixpath.join(cdir, module)) or posixpath.splitext(e)[0].endswith("/" + module)]
        cands += [norm(posixpath.join(cdir, module + ext)) for ext in (".js", ".mjs", ".cjs", ".ts", ".mts", ".cts")]
        return first(cands, "node", export)
    if runtime.startswith("java"):
        cls, _, method = h.partition("::")
        method = method or "handleRequest"
        if not cls:
            return None, "no-handler"
        simple = cls.rsplit(".", 1)[-1].rsplit("$", 1)[-1]
        relj = cls.split("$")[0].replace(".", "/") + ".java"
        cands = [norm(posixpath.join(cdir, "src/main/java", relj))]
        if cands[0] not in files:
            pre = "" if cdir in ("", ".") else cdir + "/"
            cands += sorted(f for f in files if f.startswith(pre) and f.endswith("/" + relj))
        return first(cands, "java", simple + "." + method, simple)
    if runtime.startswith("ruby"):
        base = posixpath.basename(h)
        rootp = h[:len(h) - len(base)]
        if "." not in base or base.startswith(".") or base.endswith("."):
            return None, "no-handler"
        module, meth = base.split(".", 1)
        meth = meth.rsplit("::", 1)[-1]
        return first([norm(posixpath.join(cdir, rootp + module + ".rb"))], "ruby", meth)
    return None, "runtime"


def infer_runtime(files, cdir, h):
    """The first of python, nodejs and ruby whose rule names a file of the
    subject, for a function whose Runtime is no literal."""
    if "." not in h:
        return ""
    py = h.rsplit(".", 1)[0].replace(".", "/")
    if norm(posixpath.join(cdir, py + ".py")) in files or norm(posixpath.join(cdir, py, "__init__.py")) in files:
        return "python"
    base = posixpath.basename(h)
    if "." not in base or base.startswith("."):
        return ""
    mod = h[:len(h) - len(base)] + base.split(".", 1)[0]
    if any(norm(posixpath.join(cdir, mod + e)) in files for e in (".js", ".mjs", ".cjs", ".ts", ".mts", ".cts")):
        return "nodejs"
    return "ruby" if norm(posixpath.join(cdir, mod + ".rb")) in files else ""


def subject_files(root):
    out = set()
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        for f in filenames:
            out.add(os.path.relpath(os.path.join(dirpath, f), root).replace(os.sep, "/"))
    return out


def handler_xrefs(root, templates):
    files = subject_files(root)
    out, stats = [], {"functions": 0, "bound": 0, "unbound": {}}
    for rel, tpl in sorted(templates.items()):
        for fn in lambda_functions(rel, tpl):
            stats["functions"] += 1
            target, why = handler_target(root, files, fn)
            if target is None:
                stats["unbound"][why] = stats["unbound"].get(why, 0) + 1
                continue
            stats["bound"] += 1
            out.append({"path": rel, "from": fn[0], "to_path": target[0], "to": target[1], "edge": "handler"})
    return out, stats


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
            xrefs.append({"path": rel, "from": str(name), "to_path": target, "to": posixpath.basename(target),
                          "edge": "nested_stack" if typ == "AWS::CloudFormation::Stack" else "nested_application"})
    xrefs.extend(identity_xrefs(templates))
    hx, hstats = handler_xrefs(root, templates)
    xrefs.extend(hx)
    stats["handler_functions"] = hstats["functions"]
    stats["handler_bound"] = hstats["bound"]
    for k, v in sorted(hstats["unbound"].items()):
        stats["handler_unbound_" + k] = v
    version = cfnlint.version.__version__
    with open(a.out, "w") as f:
        json.dump({"tool": "cfn-lint", "version": version, "tier": "A",
                   "definitions": sorted(defs, key=lambda d: (d["path"], d["line"], d["name"])),
                   "references": refs}, f, indent=1)
        f.write("\n")
    if a.xref_out and xrefs:
        with open(a.xref_out, "w") as f:
            json.dump({"tool": "cfn-lint (nested templates, identity joins and Lambda handlers resolved by the adapter)", "version": version,
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
