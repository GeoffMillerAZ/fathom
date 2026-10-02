#!/usr/bin/env python3
"""terragrunt_attest.py: tier-A attestation of the terragrunt oracle's
definitions and references (LB-Q-terragrunt-rest). Bench-only; never a product
dependency.

The entities and references are terragrunt_oracle.py's (python-hcl2's tree, the
dialect's naming contract, the lines); this program keeps only those Terragrunt
1.1.6 itself evidences, run offline on a scratch copy of the subject
(TG_TF_PATH=/usr/bin/false, no dependency outputs, no backend, no network):

  - what render --json --with-metadata names: a unit (a terragrunt.hcl whose
    render succeeds) and every file a render's found_in_file names (a package);
    the include blocks `find --json --include` lists per unit (import); the
    dependency, dependencies, terraform, remote_state, generate and inputs keys
    and a unit's own locals that a render attributes to the file that declares
    them (found_in_file);
  - what Terragrunt evaluates away, by ablation (the kustomize oracle's rule:
    change one entry, re-run the tool on every unit that can read the file, and
    keep the entry when the tool fails or its output changes): an included or
    read file's local (its definition renamed; the render lists only the
    unit's own locals), a feature block (its label renamed), a file no render
    names (the file moved away), and every reference (the traversal's
    attribute renamed inside the entity that writes it).

Units that can read a file: the file itself when it is a unit, the units whose
render or include list names it, and the units at or below its directory (a
find_in_parent_folders and a relative read_terragrunt_config look there), at
most -max-readers of them. A failure only counts when the unablated render
succeeded and the message is not a transient read error. An entry nothing
evidences is left out and counted on stderr, never hidden.

    terragrunt_attest.py -subject <checkout> -scratch <dir outside it> -terragrunt <bin> \\
        -files <measured files, terragrunt_oracle.py -files-out> -out <json> [-summary-out <json>]
"""
import argparse
import json
import os
import posixpath
import re
import shutil
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "hcl-tools"))
import hcl2tree as h  # noqa: E402
import terragrunt_oracle as tgo  # noqa: E402

ABL = "__fathom_ablated__"


class Runner:
    def __init__(self, binary, scratch):
        self.bin = binary
        self.env = dict(os.environ, TG_TF_PATH="/usr/bin/false", TG_NON_INTERACTIVE="true",
                        XDG_CACHE_HOME=os.path.join(scratch, "cache"), TG_DOWNLOAD_DIR=os.path.join(scratch, "dl"))
        os.makedirs(os.path.join(scratch, "cache"), exist_ok=True)
        self.runs = 0

    def run(self, cwd, args):
        """(ok, stdout or error text); a transient read error is retried."""
        for attempt in range(8):
            self.runs += 1
            try:
                r = subprocess.run([self.bin] + args, cwd=cwd, capture_output=True, text=True, env=self.env, timeout=120)
            except subprocess.TimeoutExpired:
                return False, "timeout"
            text = (r.stdout or "") + (r.stderr or "")
            if "not permitted" in text or (r.returncode == 0 and not r.stdout.strip()):
                subprocess.run(["tccutil", "reset", "SystemPolicyRemovableVolumes", "com.mitchellh.ghostty"], capture_output=True)
                time.sleep(1 + attempt)
                continue
            if r.returncode != 0:
                return False, re.sub(r"\x1b\[[0-9;]*m", "", text)
            return True, r.stdout
        return None, "transient"

    def render(self, cwd):
        return self.run(cwd, ["render", "--json", "--with-metadata"] + tgo.RENDER_ARGS)

    def find(self, cwd):
        return self.run(cwd, ["find", "--json", "--include", "--filter", "."] + tgo.FIND_ARGS)


def loads(text):
    """The render's JSON document, or None (log lines before it are skipped)."""
    try:
        return json.loads(text)
    except ValueError:
        i = text.find("{")
        try:
            return json.loads(text[i:]) if i >= 0 else None
        except ValueError:
            return None


def norm(text, tree):
    return text.replace(tree, "<tree>")


def found_files(node, tree, out):
    if isinstance(node, dict):
        m = node.get("metadata")
        if isinstance(m, dict) and isinstance(m.get("found_in_file"), str):
            f = m["found_in_file"]
            if f.startswith(tree + "/"):
                out.add(f[len(tree) + 1:])
        for v in node.values():
            found_files(v, tree, out)
    elif isinstance(node, list):
        for v in node:
            found_files(v, tree, out)


def declared(rj, tree):
    """{(file, entity name)} the render attributes to the file that declares it."""
    out = set()

    def f(meta):
        if isinstance(meta, dict) and isinstance(meta.get("found_in_file"), str) and meta["found_in_file"].startswith(tree + "/"):
            return meta["found_in_file"][len(tree) + 1:]
        return None

    for sec, prefix in (("dependency", "dependency."), ("generate", "generate."), ("inputs", "input."), ("locals", "local.")):
        v = rj.get(sec)
        if isinstance(v, dict):
            for k, item in v.items():
                if isinstance(item, dict) and "metadata" in item:
                    p = f(item["metadata"])
                    if p:
                        out.add((p, prefix + k))
                        if sec == "inputs":
                            out.add((p, "inputs"))
    for sec in ("terraform", "remote_state"):
        v = rj.get(sec)
        if isinstance(v, dict) and "metadata" in v:
            p = f(v["metadata"])
            if p:
                out.add((p, sec))
    for item in rj.get("dependencies") or []:
        if isinstance(item, dict):
            p = f(item.get("metadata"))
            if p:
                out.add((p, "dependencies"))
    return out


def base(name):
    return re.sub(r"#\d+$", "", name)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-scratch", required=True)
    ap.add_argument("-terragrunt", required=True)
    ap.add_argument("-files", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-summary-out", dest="summary_out")
    ap.add_argument("-max-readers", dest="max_readers", type=int, default=40)
    a = ap.parse_args()
    subject = os.path.abspath(a.subject)
    scratch = os.path.abspath(a.scratch)
    tree = os.path.join(scratch, "tree")
    if os.path.exists(tree):
        shutil.rmtree(tree)
    os.makedirs(scratch, exist_ok=True)
    if subprocess.run(["cp", "-cR", subject, tree], capture_output=True).returncode != 0:
        shutil.copytree(subject, tree, symlinks=True)
    tree = os.path.realpath(tree)
    tool = Runner(a.terragrunt, scratch)
    measured = [l.strip() for l in open(a.files) if l.strip()]

    ents = {}
    for rel in measured:
        src = open(os.path.join(tree, rel), encoding="utf-8", errors="replace").read()
        t = h.parse(src)
        ents[rel] = tgo.entities(rel, t, max(1, src.count("\n") + (0 if src.endswith("\n") or not src else 1)))

    units = [r for r in measured if posixpath.basename(r) == "terragrunt.hcl"]
    baseline, names_found, includes = {}, {}, {}
    attested_decl = set()
    for u in units:
        udir = os.path.join(tree, posixpath.dirname(u))
        ok, out = tool.render(udir)
        if not ok:
            continue
        rj = loads(out)
        if rj is None:  # a render whose stdout is not one JSON document
            continue
        ok2, out2 = tool.render(udir)
        baseline[u] = (norm(out, tree), ok2 and norm(out2, tree) == norm(out, tree))
        fs = set()
        found_files(rj, tree, fs)
        names_found[u] = fs
        attested_decl |= declared(rj, tree)
        okf, fout = tool.find(udir)
        incs = {}
        if okf:
            try:
                fj = json.loads(fout)
                fj = fj[0] if fj else {}
                for label, target in (fj.get("include") or {}).items():
                    p = os.path.normpath(target if os.path.isabs(target) else os.path.join(udir, target))
                    if p.startswith(tree + "/"):
                        incs[label] = p[len(tree) + 1:]
            except (ValueError, IndexError, AttributeError):
                pass
        includes[u] = incs

    def readers(rel):
        rs = [u for u in baseline if u == rel or rel in names_found[u] or rel in includes[u].values()]
        d = posixpath.dirname(rel)
        sub = [u for u in baseline if u not in rs and (d == "" or u.startswith(d + "/"))]
        return rs + sub[: max(0, a.max_readers - len(rs))]

    def ablate(rel, edit):
        """Apply edit to rel's text (None moves the file away), render its
        readers, restore; True when the tool fails or its output changes."""
        path = os.path.join(tree, rel)
        orig = open(path, encoding="utf-8", errors="replace").read()
        try:
            if edit is None:
                os.rename(path, path + ".fathom-ablated")
            else:
                new = edit(orig)
                if new == orig:
                    return False
                open(path, "w", encoding="utf-8").write(new)
            for u in readers(rel):
                ok, out = tool.render(os.path.join(tree, posixpath.dirname(u)))
                if ok is None:
                    continue
                if not ok:
                    return True
                text, stable = baseline[u]
                if stable and norm(out, tree) != text:
                    return True
            return False
        finally:
            if edit is None:
                os.rename(path + ".fathom-ablated", path)
            else:
                open(path, "w", encoding="utf-8").write(orig)

    def lines_edit(line, end, fn):
        def edit(text):
            ls = text.split("\n")
            for i in range(line - 1, min(end, len(ls))):
                ls[i] = fn(ls[i])
            return "\n".join(ls)
        return edit

    defs, refs, unev = [], [], {}
    package_seen = set()
    for u in baseline:
        package_seen.add(u)
        package_seen |= names_found[u]
        package_seen |= set(includes[u].values())
    for rel in measured:
        file_ok = rel in package_seen or ablate(rel, None)
        for (name, kind, line, end, rk) in ents[rel]:
            b = base(name)
            ok = False
            if kind == "package":
                ok = file_ok
            elif kind == "import":
                label = b[len("include."):] if b.startswith("include.") else ""
                ok = rel in includes and label in includes[rel]
            elif (rel, b) in attested_decl:
                ok = True
            elif kind == "variable" and b.startswith("local."):
                k = b[len("local."):]
                ok = ablate(rel, lines_edit(line, line, lambda s, k=k: re.sub(r"^(\s*)" + re.escape(k) + r"(\s*=)", r"\g<1>" + ABL + k + r"\2", s, count=1)))
            elif kind == "variable_decl" and b.startswith("feature."):
                k = b[len("feature."):]
                ok = ablate(rel, lines_edit(line, line, lambda s, k=k: re.sub(r'(feature\s+"?)' + re.escape(k) + r'("?)', r"\g<1>" + ABL + k + r"\2", s, count=1)))
            if ok:
                defs.append({"path": rel, "name": name, "line": line, "end": end, "kind": kind})
            else:
                unev.setdefault(kind, []).append("%s %s" % (rel, name))
            for key in rk:
                root, attr = key.split(".", 1)
                pat = re.compile(r"\b" + re.escape(root) + r"\." + re.escape(attr) + r"\b")
                rok = ablate(rel, lines_edit(line, end, lambda s, pat=pat, root=root: pat.sub(root + "." + ABL, s)))
                if rok:
                    refs.append({"path": rel, "from": name, "to": key})
                else:
                    unev.setdefault("reference", []).append("%s %s -> %s" % (rel, name, key))
    json.dump({"tool": "terragrunt render/find + ablation (python-hcl2 for names and lines)", "version": "terragrunt 1.1.6; python-hcl2 " + tgo.hcl2_version(),
               "tier": "A", "definitions": defs, "references": refs}, open(a.out, "w"), indent=1)
    summary = {"units": len(units), "rendered": len(baseline), "unstable": sum(1 for v in baseline.values() if not v[1]),
               "tool_runs": tool.runs, "definitions": len(defs), "references": len(refs),
               "unevidenced": {k: len(v) for k, v in unev.items()}, "unevidenced_entries": unev}
    if a.summary_out:
        json.dump(summary, open(a.summary_out, "w"), indent=1)
    print("terragrunt-attest: %d of %d units render (%d unstable), %d tool runs; %d definitions, %d references attested; unevidenced %s"
          % (len(baseline), len(units), summary["unstable"], tool.runs, len(defs), len(refs), summary["unevidenced"]), file=sys.stderr)


if __name__ == "__main__":
    main()
