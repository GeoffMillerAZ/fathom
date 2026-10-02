#!/usr/bin/env python3
"""hcl-tools-oracle: the language-breadth harness's oracle for the hcl-tools
native dialect (design §1.3, LB-Q-hcl-tools): Packer, Nomad and docker-bake HCL
files. Bench-only; never a product dependency. It writes `indexbench lang-oracle`'s
tool-oracle JSON, tier C: definitions and references from an independent HCL2
parse (python-hcl2's Lark tree, hcl2tree.py), which shares no code with
hashicorp/hcl's hclsyntax, the parser the dialect reads with.

  - packer  *.pkr.hcl, *.pkrvars.hcl: packer inspect (tier A) needs the packer
            binary, which is not installed here (a HashiCorp release zip is not
            fetched without the orchestrator's approval), so the whole of Packer is
            tier C.
  - nomad   *.nomad, *.nomad.hcl, and a .hcl with a top-level job block:
            nomad job validate (tier A) needs the nomad binary, not installed:
            tier C.
  - bake    docker-bake*.hcl: `docker buildx bake --progress=quiet
            --list=type=targets,format=json -f <file>` (buildx 0.31.x, no daemon,
            no build) lists the file's targets and groups as buildx resolves them.
            Each file's list is compared with the parse's names and the agreement
            is counted in the summary (never scored): buildx hides a private
            (_-prefixed) target and lists each expansion of a matrix target as
            <target>-<value> (the matrix target itself as a group), which the comparison
            allows for. The entities and lines
            are the parse's, so the file stays tier C.

The naming contract is the dialect's (internal/extractor/hcltoolsdialect.go):

  packer  source.<t>.<n>, build / build.<name>, var.<n>, local.<k>, data.<t>.<n>,
          <build>.provisioner.<t>, <build>.post-processor.<t> (a repeat gets "#2"),
          a .pkrvars.hcl file's keys; references: var./local./data./source.
          traversals and a build's sources.
  nomad   <job>, <job>.<group>, <job>.<group>.<task>, <parent>.service.<name>,
          var.<n>, local.<k>; references: var. and local. traversals (an entity
          reads its own block, its nested groups, tasks and services excluded).
  bake    <target>, group.<g>, var.<v>, func.<f>; references: a variable named bare
          (var.<v>), a call of a declared function (func.<f>), target.<t>.<attr>
          and a target's inherits and "target:<t>" contexts (<t>), a group's
          targets (<t> or group.<g>).

    hcltools_oracle.py -subject <checkout> -out <json> [-files-out <file>] [-summary-out <file>]
"""
import argparse
import json
import os
import posixpath
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import hcl2tree as h  # noqa: E402

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}


def tool_of(rel, tree):
    b = posixpath.basename(rel)
    if b.endswith(".pkr.hcl") or b.endswith(".pkrvars.hcl"):
        return "packer"
    if b.startswith("docker-bake") and b.endswith(".hcl"):
        return "bake"
    if b.endswith(".nomad") or b.endswith(".nomad.hcl"):
        return "nomad"
    if b.endswith(".hcl"):
        for it in h.top(tree):
            if it[0] == "block" and it[1] == "job" and len(it[2]) == 1:
                return "nomad"
    return None


def discover(root):
    out = []
    for d, dirs, files in os.walk(root):
        dirs[:] = sorted(x for x in dirs if x not in SKIP_DIRS)
        for f in sorted(files):
            if f.endswith(".hcl") or f.endswith(".nomad"):
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


def keyf(roots, depth):
    def f(refs):
        out = set()
        for root, attrs, line in refs:
            if root not in roots:
                continue
            n = depth[root]
            if len(attrs) >= n - 1 and all(a for a in attrs[: n - 1]):
                out.add(".".join([root] + attrs[: n - 1]))
        return sorted(out)

    return f


PACKER_ROOTS = {"var": 2, "local": 2, "data": 3, "source": 3}
NOMAD_ROOTS = {"var": 2, "local": 2}


def refs_of(node, roots, skip_blocks=()):
    """Reference keys under a body or expression node, nested blocks of the types
    in skip_blocks left to their own entities."""
    if node is None:
        return []
    f = keyf(set(roots), roots)
    if isinstance(node, h.Tree) and node.data == "body":
        found = []
        for it in h.items(node):
            if it[0] == "attr":
                found += h.refs(it[2], set(roots))
            elif it[1] not in skip_blocks and it[3] is not None:
                found += h.refs(it[3], set(roots))
        return f(found)
    return f(h.refs(node, set(roots)))


def packer(rel, tree, add):
    if rel.endswith(".pkrvars.hcl"):
        for it in h.top(tree):
            if it[0] == "attr":
                add(it[1], "key", it[3], it[4], [])
        return
    for it in h.top(tree):
        if it[0] != "block":
            continue
        _, typ, labels, body, line, end = it
        if body is None:
            continue
        if typ == "source" and len(labels) >= 2:
            add("source.%s.%s" % (labels[0], labels[1]), "resource", line, end, refs_of(body, PACKER_ROOTS))
        elif typ == "variable" and labels:
            add("var." + labels[0], "variable_decl", line, end, refs_of(body, PACKER_ROOTS))
        elif typ == "data" and len(labels) >= 2:
            add("data.%s.%s" % (labels[0], labels[1]), "data_source", line, end, refs_of(body, PACKER_ROOTS))
        elif typ == "locals":
            for sub in h.items(body):
                if sub[0] == "attr":
                    add("local." + sub[1], "variable", sub[3], sub[4], refs_of(sub[2], PACKER_ROOTS))
        elif typ == "build":
            name = "build"
            for sub in h.items(body):
                if sub[0] == "attr" and sub[1] == "name":
                    v = h.literal_string(sub[2])
                    if v:
                        name = "build." + v
            skip = ("provisioner", "post-processor", "post-processors")
            refs = set(refs_of_skip_attr(body, PACKER_ROOTS, skip, ("sources",)))
            for sub in h.items(body):
                if sub[0] == "attr" and sub[1] == "sources":
                    for e in h.tuple_items(sub[2]):
                        v = h.literal_string(e)
                        if v is not None:
                            refs.add(v if v.startswith("source.") else "source." + v)
                        else:
                            refs.update(refs_of(e, PACKER_ROOTS))
                elif sub[0] == "block" and sub[1] == "source" and sub[2]:
                    v = sub[2][0]
                    refs.add(v if v.startswith("source.") else "source." + v)
            name = add(name, "target", line, end, sorted(refs))
            packer_steps(body, name, add)


def refs_of_skip_attr(body, roots, skip_blocks, skip_attrs):
    found = []
    for it in h.items(body):
        if it[0] == "attr":
            if it[1] not in skip_attrs:
                found += h.refs(it[2], set(roots))
        elif it[1] not in skip_blocks and it[3] is not None:
            found += h.refs(it[3], set(roots))
    return keyf(set(roots), roots)(found)


def packer_steps(body, build, add):
    for it in h.items(body):
        if it[0] != "block" or it[3] is None:
            continue
        _, typ, labels, inner, line, end = it
        if typ == "provisioner" and labels:
            add("%s.provisioner.%s" % (build, labels[0]), "step", line, end, refs_of(inner, PACKER_ROOTS))
        elif typ == "post-processor" and labels:
            add("%s.post-processor.%s" % (build, labels[0]), "step", line, end, refs_of(inner, PACKER_ROOTS))
        elif typ == "post-processors":
            packer_steps(inner, build, add)


def nomad(rel, tree, add):
    for it in h.top(tree):
        if it[0] != "block":
            continue
        _, typ, labels, body, line, end = it
        if body is None:
            continue
        if typ == "variable" and labels:
            add("var." + labels[0], "variable_decl", line, end, refs_of(body, NOMAD_ROOTS))
        elif typ == "locals":
            for sub in h.items(body):
                if sub[0] == "attr":
                    add("local." + sub[1], "variable", sub[3], sub[4], refs_of(sub[2], NOMAD_ROOTS))
        elif typ == "job" and labels:
            job = add(labels[0], "job", line, end, refs_of(body, NOMAD_ROOTS, ("group", "task", "service")))
            nomad_children(job, body, add)


def nomad_children(parent, body, add):
    for it in h.items(body):
        if it[0] != "block" or it[3] is None:
            continue
        _, typ, labels, inner, line, end = it
        if typ == "group" and labels:
            g = add("%s.%s" % (parent, labels[0]), "section", line, end, refs_of(inner, NOMAD_ROOTS, ("group", "task", "service")))
            nomad_children(g, inner, add)
        elif typ == "task" and labels:
            t = add("%s.%s" % (parent, labels[0]), "task", line, end, refs_of(inner, NOMAD_ROOTS, ("group", "task", "service")))
            nomad_children(t, inner, add)
        elif typ == "service":
            n = "service"
            for sub in h.items(inner):
                if sub[0] == "attr" and sub[1] == "name":
                    v = h.literal_string(sub[2])
                    if v:
                        n = v
            add("%s.service.%s" % (parent, n), "service", line, end, refs_of(inner, NOMAD_ROOTS))


def bake(rel, tree, add):
    vars_, funcs, targets, groups = set(), set(), set(), set()
    for it in h.top(tree):
        if it[0] == "block" and it[2]:
            {"variable": vars_, "function": funcs, "target": targets, "group": groups}.get(it[1], set()).add(it[2][0])

    def refs(node, skip_attrs=()):
        found = []
        items = h.items(node) if node is not None else []
        for sub in items:
            if sub[0] == "attr" and sub[1] not in skip_attrs:
                found += h.refs(sub[2], vars_ | {"target"}, bare=True, funcs=funcs)
            elif sub[0] == "block" and sub[3] is not None:
                found += h.refs(sub[3], vars_ | {"target"}, bare=True, funcs=funcs)
        out = set()
        for root, attrs, line in found:
            if root == "()":
                out.add("func." + attrs[0])
            elif root == "target":
                if attrs and attrs[0] in targets:
                    out.add(attrs[0])
            elif root in vars_:
                out.add("var." + root)
        return out

    def names(expr):
        out = set()
        for e in h.tuple_items(expr):
            v = h.literal_string(e)
            if v in targets:
                out.add(v)
            elif v in groups:
                out.add("group." + v)
        return out

    for it in h.top(tree):
        if it[0] != "block" or not it[2] or it[3] is None:
            continue
        _, typ, labels, body, line, end = it
        n = labels[0]
        if typ == "variable":
            add("var." + n, "variable_decl", line, end, sorted(refs(body)))
        elif typ == "function":
            add("func." + n, "function", line, end, sorted(refs(body)))
        elif typ == "group":
            r = refs(body, ("targets",))
            for sub in h.items(body):
                if sub[0] == "attr" and sub[1] == "targets":
                    r |= names(sub[2])
            add("group." + n, "stage", line, end, sorted(r))
        elif typ == "target":
            r = refs(body, ("inherits",))
            for sub in h.items(body):
                if sub[0] == "attr" and sub[1] == "inherits":
                    r |= names(sub[2])
                if sub[0] == "attr" and sub[1] == "contexts":
                    for kn, val, l, e in h.object_items(sub[2]) or []:
                        v = h.literal_string(val)
                        if v and v.startswith("target:") and v[7:] in targets:
                            r.add(v[7:])
            add(n, "target", line, end, sorted(r))
    return targets | {"group." + g for g in groups}


def bake_list(path):
    """Target and group names buildx resolves in one bake file, or None."""
    r = subprocess.run(["docker", "buildx", "bake", "--progress=quiet", "--list=type=targets,format=json", "-f", os.path.basename(path)],
                       cwd=os.path.dirname(path), capture_output=True, text=True, timeout=120)
    if r.returncode != 0:
        return None
    try:
        return {(("group." if x.get("group") else "") + x["name"]) for x in json.loads(r.stdout)}
    except Exception:
        return None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-files-out", dest="files_out")
    ap.add_argument("-summary-out", dest="summary_out")
    a = ap.parse_args()
    subject = os.path.abspath(a.subject)
    defs, refs, measured = [], [], []
    summary = {"files": 0, "by_tool": {}, "parse_failed": [], "bake_crosscheck": {"files": 0, "agree": 0, "disagree": [], "tool_failed": []}}
    for rel in discover(subject):
        src = open(os.path.join(subject, rel), encoding="utf-8", errors="replace").read()
        try:
            tree = h.parse(src)
        except Exception:
            if rel.endswith((".pkr.hcl", ".pkrvars.hcl", ".nomad", ".nomad.hcl")) or posixpath.basename(rel).startswith("docker-bake"):
                summary["parse_failed"].append(rel)
            continue
        tool = tool_of(rel, tree)
        if tool is None:
            continue
        summary["files"] += 1
        summary["by_tool"][tool] = summary["by_tool"].get(tool, 0) + 1
        ns = Names()
        got = []

        def add(name, kind, line, end, rk):
            name = ns.get(name)
            got.append((name, kind, line, end, rk))
            return name

        names = None
        if tool == "packer":
            packer(rel, tree, add)
        elif tool == "nomad":
            nomad(rel, tree, add)
        else:
            names = bake(rel, tree, add)
        if tool == "bake":
            cc = summary["bake_crosscheck"]
            listed = bake_list(os.path.join(subject, rel))
            if listed is None:
                cc["tool_failed"].append(rel)
            else:
                cc["files"] += 1
                # buildx hides a private (_-prefixed) target and lists each
                # expansion of a matrix target as <target>-<value>
                pub = {n for n in names if not n.startswith("_") and not n.startswith("group._")}
                miss_t = {p for p in pub if p not in listed and not any(l.startswith(p + "-") for l in listed)}
                miss_p = {l for l in listed if l not in pub and not any(l.startswith(p + "-") for p in pub) and not (l.startswith("group.") and l[6:] in pub)}
                if not miss_t and not miss_p:
                    cc["agree"] += 1
                else:
                    cc["disagree"].append("%s: buildx lists %s, the parse %s" % (rel, sorted(miss_p), sorted(miss_t)))
        measured.append(rel)
        for (name, kind, line, end, rk) in got:
            defs.append({"path": rel, "name": name, "line": line, "end": end, "kind": kind})
            for k in rk:
                refs.append({"path": rel, "from": name, "to": k})
    json.dump({"tool": "python-hcl2 (HCL2 parse tree)", "version": "8.1.4", "tier": "C", "definitions": defs, "references": refs}, open(a.out, "w"), indent=1)
    if a.files_out:
        open(a.files_out, "w").write("\n".join(measured) + "\n")
    summary["definitions"], summary["references"], summary["measured"] = len(defs), len(refs), len(measured)
    if a.summary_out:
        json.dump(summary, open(a.summary_out, "w"), indent=1)
    print("files", summary["files"], summary["by_tool"], "defs", len(defs), "refs", len(refs), "parse_failed", len(summary["parse_failed"]),
          "bake agree", summary["bake_crosscheck"]["agree"], "/", summary["bake_crosscheck"]["files"], file=sys.stderr)


if __name__ == "__main__":
    main()
