#!/usr/bin/env python3
"""meson_oracle.py: Meson's own parser and evaluation as a tier-A oracle for indexbench lang-oracle (LB-Q-meson).

Bench-only, never imported by fathom. Python 3 with Meson's `mesonbuild` package importable (Homebrew's
meson installs it beside python@3.14: /opt/homebrew/opt/python@3.14/bin/python3.14). It writes the neutral
oracle file `indexbench lang-oracle -tool-oracle` reads (calibration/oracles/README.md), from two sources:

  Meson's parser (mesonbuild.mparser, the format's reference parser, tier A by design section 1.3), on every
  meson.build, meson.options and meson_options.txt of the subject:
    definitions       project('name') [module], a target call named by its first string argument [target]
                      (executable, the library calls, custom_target, jar, build_target, run_target), the variable
                      a target call is assigned to [variable_decl], subproject('x') [module], option('x')
                      [variable_decl], and `v = dependency(...)` or `v = declare_dependency(...)` named by the
                      variable [import]; each at the line of its name and ending at the call's `)`.
                      A target whose first argument is not a string literal ('t_' + name) has no name to read.
    references        (path, target or dependency variable, identifier) for each identifier spelled in the
                      link_with, link_whole, dependencies or depends argument of such a call.
  Meson's evaluation (`meson setup` then `meson introspect --targets`, a configured build directory):
    cross_references  (path, target, to_path, dependency target) for each target another target links
                      (its linker line names the other's output) or depends on (`depends`), where the two are
                      defined in different files, on the file the linking target is defined in; file granularity.

  meson_oracle.py -subject <checkout> -build <scratch dir> [-D option=value]... [-no-evaluate]
"""
import argparse
import contextlib
import dataclasses
import json
import os
import subprocess
import sys

# Meson reads `testcase ... endtestcase` (its own test suite's expect_error clause) only under this variable.
os.environ.setdefault("MESON_RUNNING_IN_PROJECT_TESTS", "1")

from mesonbuild import mparser  # noqa: E402

TARGETS = ("executable", "static_library", "shared_library", "library", "both_libraries", "shared_module",
           "custom_target", "jar", "build_target", "run_target")
DEPS = ("dependency", "declare_dependency")
LINK_KWARGS = ("link_with", "link_whole", "dependencies", "depends")
FILES = ("meson.build", "meson.options", "meson_options.txt")


def children(value):
    """The AST nodes directly held by a field value (a node, or a list or dict of them)."""
    if isinstance(value, (list, tuple)):
        for v in value:
            yield from children(v)
    elif isinstance(value, dict):
        for k, v in value.items():
            yield from children(k)
            yield from children(v)
    elif isinstance(value, mparser.BaseNode):
        yield value


def walk(node):
    yield node
    if dataclasses.is_dataclass(node):
        for f in dataclasses.fields(node):
            for c in children(getattr(node, f.name)):
                yield from walk(c)


def positional(call):
    return [a for a in call.args.arguments]


def kwargs(call):
    return {(k.value if hasattr(k, "value") else str(k)): v for k, v in call.args.kwargs.items()}


def end_line(call):
    r = getattr(call, "rpar", None)
    return r.lineno if r is not None else call.end_lineno


def idents(value):
    if isinstance(value, mparser.IdNode):
        return [value.value]
    if isinstance(value, mparser.ArrayNode):
        return [a.value for a in value.args.arguments if isinstance(a, mparser.IdNode)]
    return []


def parse_file(subject, rel):
    code = open(os.path.join(subject, rel), encoding="utf-8").read()
    try:
        with contextlib.redirect_stdout(sys.stderr):  # the parser logs its warnings to stdout
            return mparser.Parser(code, rel).parse()
    except Exception as e:  # a file Meson itself refuses is not in the truth
        sys.stderr.write("meson_oracle: %s: %s\n" % (rel, str(e).splitlines()[0] if str(e) else type(e).__name__))
        return None


def source_files(subject):
    out = []
    for d, dirs, names in os.walk(subject):
        dirs[:] = [x for x in dirs if x != ".git"]
        for n in names:
            if n in FILES:
                out.append(os.path.relpath(os.path.join(d, n), subject))
    return sorted(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-build", required=True)
    ap.add_argument("-D", action="append", default=[], dest="defs")
    ap.add_argument("-no-evaluate", action="store_true")
    a = ap.parse_args()
    subject = os.path.realpath(a.subject)
    defs, refs = [], set()
    for rel in source_files(subject):
        ast = parse_file(subject, rel)
        if ast is None:
            continue
        for node in walk(ast):
            call, owner_var = None, None
            if isinstance(node, mparser.AssignmentNode) and type(node).__name__ == "AssignmentNode" and isinstance(node.value, mparser.FunctionNode):
                call, owner_var = node.value, node.var_name.value
            elif isinstance(node, mparser.FunctionNode):
                call = node
            else:
                continue
            fname = call.func_name.value
            pos = positional(call)
            first = pos[0] if pos and isinstance(pos[0], mparser.StringNode) else None
            owner = None
            if isinstance(node, mparser.AssignmentNode):
                if fname in DEPS:
                    defs.append((rel, owner_var, node.var_name.lineno, end_line(call), "import"))
                    owner = owner_var
                    # the call is also visited as a bare FunctionNode: it names nothing of its own
                elif fname in TARGETS:
                    defs.append((rel, owner_var, node.var_name.lineno, end_line(call), "variable_decl"))
            elif first is not None and fname == "project":
                defs.append((rel, first.value, first.lineno, end_line(call), "module"))
            elif first is not None and fname in TARGETS:
                defs.append((rel, first.value, first.lineno, end_line(call), "target"))
                owner = first.value
            elif first is not None and fname == "subproject":
                defs.append((rel, first.value, first.lineno, end_line(call), "module"))
            elif first is not None and fname == "option":
                defs.append((rel, first.value, first.lineno, end_line(call), "variable_decl"))
            if owner is None:
                continue
            for k, v in kwargs(call).items():
                if k in LINK_KWARGS:
                    for name in idents(v):
                        refs.add((rel, owner, name))
    xrefs, notes = set(), {}
    if not a.no_evaluate:
        build = os.path.realpath(a.build)
        if build.startswith(subject + os.sep):
            sys.exit("meson_oracle: -build must not be inside the subject")
        r = subprocess.run(["meson", "setup", build, subject] + ["-D" + d for d in a.defs],
                           stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, timeout=1800)
        if r.returncode != 0:
            sys.exit("meson_oracle: meson setup failed:\n" + r.stdout[-3000:])
        t = subprocess.run(["meson", "introspect", build, "--targets"], stdout=subprocess.PIPE, text=True, check=True)
        targets = json.loads(t.stdout)
        by_id = {x["id"]: x for x in targets}
        out_of = {}
        for x in targets:
            for f in x.get("filename", []):
                out_of[os.path.relpath(f, build)] = x
        def rel_of(x):
            d = x["defined_in"]
            p = os.path.relpath(d, subject) if os.path.isabs(d) else d
            return None if p.startswith("..") else p
        notes["targets"] = len(targets)
        for x in targets:
            src = rel_of(x)
            if src is None:
                continue
            deps = set()
            for s in x.get("target_sources", []):
                for p in s.get("parameters", []) if "linker" in s else []:
                    if p in out_of and out_of[p] is not x:
                        deps.add(out_of[p]["id"])
            deps.update(i for i in x.get("depends", []) if i in by_id)
            for i in sorted(deps):
                y = by_id[i]
                dst = rel_of(y)
                if dst is not None and dst != src:
                    xrefs.add((src, x["name"], dst, y["name"]))
    out = {
        "tool": "meson mparser + introspect",
        "version": subprocess.run(["meson", "--version"], stdout=subprocess.PIPE, text=True).stdout.strip(),
        "tier": "A",
        "definitions": [{"path": p, "name": n, "line": l, "end": e, "kind": k} for p, n, l, e, k in sorted(set(defs))],
        "references": [{"path": p, "from": f, "to": t} for p, f, t in sorted(refs)],
    }
    if not a.no_evaluate:  # without an evaluation there is no cross-file truth: L3 is left unmeasured, never scored against nothing
        out["cross_references"] = [{"path": p, "from": f, "to_path": tp, "to": t} for p, f, tp, t in sorted(xrefs)]
    json.dump(out, sys.stdout, indent=1)
    sys.stdout.write("\n")
    sys.stderr.write("meson_oracle: %d definitions, %d references, %d cross_references %s\n" % (len(set(defs)), len(refs), len(xrefs), notes))


if __name__ == "__main__":
    main()
