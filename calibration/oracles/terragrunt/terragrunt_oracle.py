#!/usr/bin/env python3
"""terragrunt-oracle: the language-breadth harness's oracle for the terragrunt
native dialect (design §1.3, LB-Q-terragrunt). Bench-only; never a product
dependency. It writes `indexbench lang-oracle`'s tool-oracle JSON in two files,
one per tier, which lang-oracle takes as repeated -tool-oracle flags:

  -out        tier C: definitions and references, from an independent HCL2
              parse (python-hcl2's Lark tree, hcl2tree.py: no code shared with
              hashicorp/hcl's hclsyntax, which the dialect reads with);
  -xref-out   tier A: cross_references, from Terragrunt itself.

What the tool decides (tier A): every terragrunt.hcl, the unit, is run in place
in a scratch copy of the subject, with no tofu, no backend and no network. (A
file an include brings in has no cross_reference: Terragrunt reads its relative
paths from the including unit's directory, which one file cannot say, and the
dialect leaves them unresolved; the files are still measured for definitions
and references.)

    TG_TF_PATH=/usr/bin/false terragrunt render --json --with-metadata \\
        --experiment optional-dependency-outputs --no-dependency-outputs
    terragrunt find --json --include --filter .

  - include: `find --include` names each include block's resolved file
    (find_in_parent_folders, templates, locals: the tool's own evaluation);
  - dependency and dependencies: `render`'s dependency config_path (and
    dependencies.paths), each from the file `render --with-metadata` says
    declares it (found_in_file), resolved to <dir>/terragrunt.hcl;
  - terraform.source: `render`'s evaluated source of the file that declares it;
    a local one (".", "./…", "../…") is the module directory, and the oracle's
    edge is to each .tf file there that declares a definition (an independent
    scan of the .tf files with the same HCL2 reader: tier C for the file list,
    never the tool's), "//subdir" below it naming the module inside;
  - inputs: a key `render` says the file declares, to each .tf file of the
    module directory that declares variable "<key>" (same scan), when the file
    itself declares a local terraform.source.

What the independent parse decides (tier C): the definitions and references.
The naming contract is the dialect's (internal/extractor/dialects/terragrunt):
the file (package, its base name), include.<label>, dependency.<n>,
dependencies, terraform, remote_state, generate.<n>, feature.<n>, local.<k>,
input.<k> (the keys of an inputs object, directly or as an argument of
merge()); inputs of any other shape is the entity "inputs"; a repeated name
gets "#2". `references` are (entity, entity it reads): local.x, dependency.n,
include.n and feature.n traversals. Where the tool says something about the
same entities (render's inputs, generate, remote_state, dependency, found_in_file),
the agreement is counted in the summary, never scored.

A unit the tool cannot evaluate (a syntax error, an experiment it needs, a
plugin, an unset variable) contributes no cross_reference and is left out of
-files-out, which lists the measured files for `lang-oracle -only`; the summary
names each.

    terragrunt_oracle.py -subject <checkout> -scratch <dir outside the subject> \\
        -terragrunt <bin> -out <defs.json> -xref-out <xref.json> [-files-out <file>] [-summary-out <file>]
"""
import argparse
import json
import os
import posixpath
import re
import shutil
import subprocess
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "hcl-tools"))
import hcl2tree as h  # noqa: E402

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
ROOTS = {"local", "dependency", "include", "feature"}
TOKENS = re.compile(r"\b(include|dependency|dependencies|terraform|inputs)\b")
RENDER_ARGS = ["--experiment", "optional-dependency-outputs", "--no-dependency-outputs", "--log-level", "error", "--non-interactive"]
FIND_ARGS = ["--log-level", "error", "--non-interactive"]


def discover(root, suffix=".hcl"):
    out = []
    for d, dirs, files in os.walk(root):
        dirs[:] = sorted(x for x in dirs if x not in SKIP_DIRS)
        for f in sorted(files):
            if f.endswith(suffix) and not f.endswith(".lock.hcl"):
                out.append(os.path.relpath(os.path.join(d, f), root).replace(os.sep, "/"))
    return sorted(out)


class Names:
    def __init__(self):
        self.seen = {}

    def get(self, name):
        self.seen[name] = self.seen.get(name, 0) + 1
        n = self.seen[name]
        if n > 1:
            name = "%s#%d" % (name, n)
            self.seen[name] = 1
        return name


def ref_keys(node):
    keys = []
    for root, attrs, line in h.refs(node, ROOTS):
        if attrs and attrs[0]:
            keys.append(root + "." + attrs[0])
    return sorted(set(keys))


def entities(rel, tree, nlines):
    """[(name, kind, line, end, [ref keys])] for one parsed file."""
    ns = Names()
    out = []

    def add(name, kind, line, end, refs):
        out.append((ns.get(name), kind, line, end, refs))

    add(posixpath.basename(rel), "package", 1, nlines, [])
    for it in h.top(tree):
        if it[0] == "block":
            _, typ, labels, body, line, end = it
            refs = ref_keys(body) if body is not None else []
            lab = labels[0] if labels else ""
            if typ == "include":
                add("include" + ("." + lab if lab else ""), "import", line, end, refs)
            elif typ == "dependency":
                add("dependency." + lab, "target", line, end, refs)
            elif typ == "dependencies":
                add("dependencies", "target", line, end, refs)
            elif typ == "terraform":
                add("terraform", "module", line, end, refs)
            elif typ == "remote_state":
                add("remote_state", "resource", line, end, refs)
            elif typ == "generate":
                add("generate." + lab, "resource", line, end, refs)
            elif typ == "feature":
                add("feature." + lab, "variable_decl", line, end, refs)
            elif typ == "locals" and body is not None:
                for sub in h.items(body):
                    if sub[0] == "attr":
                        add("local." + sub[1], "variable", sub[3], sub[4], ref_keys(sub[2]))
        else:
            _, name, expr, line, end = it
            if name == "inputs":
                inputs(add, expr, line, end)
    return out


def inputs(add, expr, line, end):
    objs, rest = [], []

    def gather(e):
        e = h.unwrap(e)
        if h.object_items(e) is not None:
            objs.append(e)
            return
        c = h.call(e)
        if c and c[0] == "merge":
            for a in c[1]:
                gather(a)
            return
        rest.append(e)

    gather(expr)
    for o in objs:
        for name, val, l, e in h.object_items(o):
            if name is None:
                rest.append(val)
                continue
            add("input." + name, "key", l, e, ref_keys(val))
    if rest:
        keys = sorted(set(k for r in rest for k in ref_keys(r)))
        add("inputs", "key", line, end, keys)


def tf_facts(root, d):
    """(files with a definition, {variable name: [files]}) of a module directory
    by an independent scan of its .tf files."""
    files, variables = [], {}
    full = os.path.join(root, d)
    if not os.path.isdir(full):
        return files, variables
    for f in sorted(os.listdir(full)):
        if not f.endswith(".tf"):
            continue
        rel = posixpath.join(d, f) if d != "." else f
        try:
            tree = h.parse(open(os.path.join(full, f), encoding="utf-8").read())
        except Exception:
            continue
        has = False
        for it in h.top(tree):
            if it[0] == "block" and it[1] in ("resource", "data", "module", "variable", "output", "locals"):
                has = True
                if it[1] == "variable" and it[2]:
                    variables.setdefault(it[2][0], []).append(rel)
        if has:
            files.append(rel)
    return files, variables


def local_module_dir(d, source):
    """The repository directory a terraform.source value names when it is local."""
    if not (source == "." or source.startswith("./") or source.startswith("../")):
        return None
    source = source.split("?", 1)[0].replace("//", "/", 1)
    c = posixpath.normpath(posixpath.join(d, source))
    if c == ".." or c.startswith("../"):
        return None
    return c


class Tool:
    def __init__(self, binary, scratch):
        self.bin = binary
        self.env = dict(os.environ, TG_TF_PATH="/usr/bin/false", TG_NON_INTERACTIVE="true", XDG_CACHE_HOME=os.path.join(scratch, "cache"), TG_DOWNLOAD_DIR=os.path.join(scratch, "dl"))
        os.makedirs(os.path.join(scratch, "cache"), exist_ok=True)
        r = subprocess.run([binary, "--version"], capture_output=True, text=True, env=self.env)
        self.version = (r.stdout.strip().split()[-1]) if r.returncode == 0 else "unknown"

    def run(self, cwd, args):
        r = subprocess.run([self.bin] + args, cwd=cwd, capture_output=True, text=True, env=self.env, timeout=120)
        if r.returncode != 0:
            msg = (r.stderr or r.stdout).strip().splitlines()
            raise RuntimeError(re.sub(r"\x1b\[[0-9;]*m", "", msg[-1])[:200] if msg else "exit %d" % r.returncode)
        return json.loads(r.stdout)

    def render(self, cwd):
        return self.run(cwd, ["render", "--json", "--with-metadata"] + RENDER_ARGS)

    def find(self, cwd):
        out = self.run(cwd, ["find", "--json", "--include", "--filter", "."] + FIND_ARGS)
        return out[0] if out else {}


def meta_items(section):
    """{name: (value, found_in_file)} of a render --with-metadata section."""
    out = {}
    if not isinstance(section, dict):
        return out
    for k, v in section.items():
        if isinstance(v, dict) and "value" in v and "metadata" in v:
            m = v["metadata"] or {}
            out[k] = (v["value"], m.get("found_in_file"))
    return out


def under(scratch_root, p, base_dir):
    """A path the tool printed, as a repository path (None outside the subject)."""
    if not p:
        return None
    if not os.path.isabs(p):
        p = os.path.join(base_dir, p)
    p = os.path.normpath(p)
    rel = os.path.relpath(p, scratch_root)
    if rel == ".." or rel.startswith("../"):
        return None
    return rel.replace(os.sep, "/")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-scratch", required=True)
    ap.add_argument("-terragrunt", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-xref-out", dest="xref_out", required=True)
    ap.add_argument("-files-out", dest="files_out")
    ap.add_argument("-summary-out", dest="summary_out")
    a = ap.parse_args()
    subject = os.path.abspath(a.subject)
    scratch = os.path.abspath(a.scratch)
    if os.path.commonpath([subject, scratch]) == subject:
        sys.exit("-scratch must lie outside the subject")
    tree_dir = os.path.join(scratch, "tree")
    if os.path.exists(tree_dir):
        shutil.rmtree(tree_dir)
    os.makedirs(scratch, exist_ok=True)
    if subprocess.run(["cp", "-cR", subject, tree_dir], capture_output=True).returncode != 0:
        shutil.copytree(subject, tree_dir, symlinks=True)
    tool = Tool(a.terragrunt, scratch)

    defs, refs, xrefs, measured = [], [], [], []
    summary = {"files": 0, "parse_failed": [], "tool_failed": [], "tool_skipped_no_tokens": 0, "crosscheck": {"checked": 0, "agree": 0, "disagree": []}}
    for rel in discover(subject):
        summary["files"] += 1
        src = open(os.path.join(subject, rel), encoding="utf-8", errors="replace").read()
        try:
            tree = h.parse(src)
        except Exception as e:  # not valid HCL2 as this reader sees it
            summary["parse_failed"].append(rel)
            continue
        ents = entities(rel, tree, max(1, src.count("\n") + (0 if src.endswith("\n") or not src else 1)))
        names = {e[0] for e in ents}
        d = posixpath.dirname(rel) or "."
        edges = []
        if TOKENS.search(src) and posixpath.basename(rel) == "terragrunt.hcl":
            try:
                udir = os.path.join(tree_dir, d)
                rj = tool.render(udir)
                fj = tool.find(udir)
                edges = tool_edges(rel, d, tree_dir, udir, rj, fj, names, tree_dir, summary)
            except Exception as e:
                summary["tool_failed"].append("%s: %s" % (rel, e))
                continue
        else:
            summary["tool_skipped_no_tokens"] += 1
        measured.append(rel)
        for (name, kind, line, end, rk) in ents:
            defs.append({"path": rel, "name": name, "line": line, "end": end, "kind": kind})
            for k in rk:
                refs.append({"path": rel, "from": name, "to": k})
        for (frm, to) in edges:
            xrefs.append({"path": rel, "from": frm, "to_path": to})

    xrefs = finish_xrefs(xrefs)
    ver = "python-hcl2 %s; terragrunt %s" % (hcl2_version(), tool.version)
    json.dump({"tool": "python-hcl2 (HCL2 parse tree)", "version": hcl2_version(), "tier": "C", "definitions": defs, "references": refs}, open(a.out, "w"), indent=1)
    json.dump({"tool": "terragrunt render/find", "version": tool.version, "tier": "A", "cross_references": xrefs}, open(a.xref_out, "w"), indent=1)
    if a.files_out:
        open(a.files_out, "w").write("\n".join(measured) + "\n")
    summary["measured"] = len(measured)
    summary["definitions"] = len(defs)
    summary["references"] = len(refs)
    summary["cross_references"] = len(xrefs)
    if a.summary_out:
        json.dump(summary, open(a.summary_out, "w"), indent=1)
    print(ver, "files", summary["files"], "measured", len(measured), "defs", len(defs), "refs", len(refs), "xrefs", len(xrefs),
          "parse_failed", len(summary["parse_failed"]), "tool_failed", len(summary["tool_failed"]), file=sys.stderr)


def hcl2_version():
    """The installed python-hcl2 distribution's version (hcl2.version is a module, not a string)."""
    from importlib.metadata import PackageNotFoundError, version
    try:
        return version("python-hcl2")
    except PackageNotFoundError:
        return "unknown"


def tool_edges(rel, d, root, udir, rj, fj, names, tree_dir, summary):
    """(from entity, to file) pairs the tool resolves for the file at rel."""
    edges = []
    unit = os.path.join(udir, "terragrunt.hcl")
    # include blocks
    for label, target in sorted((fj.get("include") or {}).items()):
        to = under(tree_dir, target, udir)
        if to is not None:
            edges.append(("include" + ("." + label if label else ""), to))
    # dependency blocks the file declares
    raw_dep = rj.get("dependency") if isinstance(rj.get("dependency"), dict) else {}
    for name, v in sorted(raw_dep.items()):
        found = None
        if isinstance(v, dict) and "metadata" in v:
            found = (v.get("metadata") or {}).get("found_in_file")
            v = v.get("value")
        if found is not None and os.path.normpath(found) != os.path.normpath(unit):
            continue
        cp = (v or {}).get("config_path") if isinstance(v, dict) else None
        to = under(tree_dir, cp, udir)
        if to is not None:
            edges.append(("dependency." + name, to if to.endswith(".hcl") else posixpath.join(to, "terragrunt.hcl")))
    # dependencies { paths }: render lists each path with the file that declares it
    for item in rj.get("dependencies") or []:
        if not isinstance(item, dict) or not isinstance(item.get("value"), str):
            continue
        found = (item.get("metadata") or {}).get("found_in_file")
        if found is not None and os.path.normpath(found) != os.path.normpath(unit):
            continue
        to = under(tree_dir, item["value"], udir)
        if to is not None:
            edges.append(("dependencies", to if to.endswith(".hcl") else posixpath.join(to, "terragrunt.hcl")))
    # terraform.source and inputs, from the file's own local source
    tf = rj.get("terraform")
    if isinstance(tf, dict) and "value" in tf:
        sfound = (tf.get("metadata") or {}).get("found_in_file")
        sval = (tf.get("value") or {}).get("source")
        if isinstance(sval, str) and sfound and os.path.normpath(sfound) == os.path.normpath(unit):
            mod = local_module_dir(d, sval)
            if mod is not None:
                files, variables = tf_facts(tree_dir, mod)
                for f in files:
                    edges.append(("terraform", f))
                for key, (val, found) in sorted(meta_items(rj.get("inputs")).items()):
                    if found and os.path.normpath(found) == os.path.normpath(unit):
                        for f in variables.get(key, []):
                            edges.append(("input." + key, f))
    crosscheck(rel, rj, unit, names, summary)
    return edges


def crosscheck(rel, rj, unit, names, summary):
    """Count what the tool says the file declares against the independent parse."""
    declared = set()
    for sec, prefix in (("inputs", "input."), ("generate", "generate."), ("dependency", "dependency.")):
        for k, v in (rj.get(sec) or {}).items() if isinstance(rj.get(sec), dict) else []:
            found = (v.get("metadata") or {}).get("found_in_file") if isinstance(v, dict) and "metadata" in v else None
            if found and os.path.normpath(found) == os.path.normpath(unit):
                declared.add(prefix + k)
    rs = rj.get("remote_state")
    if isinstance(rs, dict) and (rs.get("metadata") or {}).get("found_in_file") and os.path.normpath(rs["metadata"]["found_in_file"]) == os.path.normpath(unit):
        declared.add("remote_state")
    cc = summary["crosscheck"]
    for n in sorted(declared):
        cc["checked"] += 1
        if n in names or n + "#2" in names:
            cc["agree"] += 1
        else:
            cc["disagree"].append("%s: tool declares %s, the parse does not" % (rel, n))


def finish_xrefs(xs):
    seen, out = set(), []
    for x in xs:
        k = (x["path"], x["from"], x["to_path"])
        if k not in seen and x["to_path"] != x["path"]:
            seen.add(k)
            out.append(x)
    return sorted(out, key=lambda x: (x["path"], x["from"], x["to_path"]))


if __name__ == "__main__":
    main()
