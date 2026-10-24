"""checkov_oracle.py - the language-breadth harness's oracle for the checkov-policies
dialect (design section 1.3, 13.4 step 19; LB-Q-checkov-policies).

Run it with the venv that holds checkov (pip install checkov==3.2.500 measured):

    python checkov_oracle.py -subject <checkout> -out <file> [-files-out <file>] [-summary-out <file>]

Oracle, tier A for what checkov decides, tier C for source positions:

  - Python checks (tier A): checkov's own external-checks loader,
    `BaseCheckRegistry.load_external_checks(dir)` (what `checkov --external-checks-dir`
    runs), over every directory of the subject that holds checks
    (`checks/{resource,data,provider,module}`); each file it imports registers its checks
    through `BaseCheck.__init__`. One `check` definition per registered check, named by
    its id, in the file that defines its class; one reference per resource type of the
    check's `supported_entities` (what the loader registered the check under) that the
    check's class writes as a string literal.
  - YAML graph checks (tier A): checkov's graph registry loader,
    `Registry.load_external_checks(dir)` with `GraphCheckParser`, over every
    `checks/graph_checks` directory; one `check` per parsed check, named by its
    `metadata.id`, one reference per resource type the parsed check's solver tree names.
  - lines (tier C): the loaders keep no source positions, so a Python check's line is
    the line of its id literal and its end the end of its class (Python's own `ast`),
    and a YAML check's line is the first line of its mapping (PyYAML `compose`).

A file the loader refuses (an import error, an invalid definition) lists nothing; the
summary counts them. Directories the connector never reads are not walked. Bench-only;
never a product dependency.
"""

import argparse
import ast
import inspect
import json
import logging
import os
import sys

import yaml

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
PY_DIRS = ("resource", "data", "provider", "module")


def rel_of(path, root):
    return os.path.relpath(os.path.realpath(path), root).replace(os.sep, "/")


def find_dirs(root):
    """(python check dirs, graph check dirs) of the subject, sorted."""
    py, graph = [], []
    for dirpath, dirnames, _ in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        parts = dirpath.replace(os.sep, "/").split("/")
        if os.path.basename(dirpath) == "graph_checks" and len(parts) > 1 and parts[-2] == "checks":
            graph.append(dirpath)
            dirnames[:] = []
        elif len(parts) > 1 and parts[-2] == "checks" and parts[-1] in PY_DIRS:
            py.append(dirpath)
            dirnames[:] = []
    return sorted(py), sorted(graph)


def id_line(tree, cls_name, check_id):
    """(line of the id literal, class end line, string literals the class writes) from the file's own ast."""
    for node in ast.walk(tree):
        if isinstance(node, ast.ClassDef) and node.name == cls_name:
            literals = {s.value for s in ast.walk(node) if isinstance(s, ast.Constant) and isinstance(s.value, str)}
            for sub in ast.walk(node):
                if isinstance(sub, ast.Constant) and sub.value == check_id:
                    return sub.lineno, node.end_lineno, literals
            return node.lineno, node.end_lineno, literals
    return None


def yaml_line(text):
    try:
        node = yaml.compose(text, Loader=yaml.SafeLoader)
    except yaml.YAMLError:
        return None
    if not isinstance(node, yaml.MappingNode):
        return None
    return node.value[0][0].start_mark.line + 1, node.end_mark.line


def scalars(node, out):
    if isinstance(node, dict):
        for k, v in node.items():
            scalars(k, out)
            scalars(v, out)
    elif isinstance(node, list):
        for v in node:
            scalars(v, out)
    elif isinstance(node, str):
        out.add(node)
    return out


def graph_resource_types(check, written):
    """The resource types the parsed check names that the file writes as a scalar: a
    `resource_types: all` check expands to every type of a provider, which no text names."""
    return sorted({rt for rt in getattr(check, "resource_types", None) or [] if isinstance(rt, str) and rt in written})


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-files-out", default="")
    ap.add_argument("-summary-out", default="")
    a = ap.parse_args(argv)
    logging.disable(logging.CRITICAL)

    root = os.path.realpath(a.subject)
    from importlib.metadata import version
    from checkov.common.checks.base_check_registry import BaseCheckRegistry
    from checkov.common.checks_infra.checks_parser import GraphCheckParser
    from checkov.common.checks_infra.registry import Registry

    stats = {"python_dirs": 0, "graph_dirs": 0, "python_files": 0, "yaml_files": 0, "py_checks": 0,
             "py_listed": 0, "duplicate": 0, "py_refs": 0, "py_refs_unwritten": 0, "graph_checks": 0, "graph_listed": 0, "no_position": 0}
    defs, refs, files = [], [], set()
    py_dirs, graph_dirs = find_dirs(root)
    stats["python_dirs"], stats["graph_dirs"] = len(py_dirs), len(graph_dirs)

    reg = BaseCheckRegistry("checkov-policies-oracle")
    for d in py_dirs:
        reg.load_external_checks(d)
    trees, seen = {}, set()
    for check in BaseCheckRegistry.get_all_registered_checks():
        # the file of the class's own __init__ code: the loader keys sys.modules by file stem,
        # so two files of one stem (credentials.py, IAMWriteAccess.py) would swap a module's file
        init = type(check).__dict__.get("__init__")
        path = init.__code__.co_filename if init else inspect.getsourcefile(type(check))
        if not path or not os.path.realpath(path).startswith(root + os.sep):
            continue
        stats["py_checks"] += 1
        rel = rel_of(path, root)
        if rel not in trees:
            with open(path, encoding="utf-8") as f:
                trees[rel] = ast.parse(f.read())
        pos = id_line(trees[rel], type(check).__name__, check.id)
        if not pos:
            stats["no_position"] += 1
            continue
        if (rel, check.id) in seen:
            stats["duplicate"] += 1
            continue
        seen.add((rel, check.id))
        stats["py_listed"] += 1
        files.add(rel)
        defs.append({"path": rel, "name": check.id, "line": pos[0], "end": pos[1], "kind": "check"})
        # a resource type is the file's reference only when the class writes it: the entity
        # list a framework's base class supplies (every Kubernetes workload kind, "containers")
        # is registered by the loader but is no text of the file
        written = sorted(set(e for e in check.supported_entities if isinstance(e, str) and e in pos[2]))
        stats["py_refs"] += len(written)
        stats["py_refs_unwritten"] += len(set(check.supported_entities)) - len(written)
        for ent in written:
            refs.append({"path": rel, "from": check.id, "to": ent})
    stats["python_files"] = len(files)

    for d in graph_dirs:
        greg = Registry(parser=GraphCheckParser(), checks_dir=d)
        greg.load_external_checks(d)
        for check in greg.checks:
            stats["graph_checks"] += 1
            path = getattr(check, "check_path", None) or ""
            rel = rel_of(path, root) if path else None
            if not rel:
                stats["no_position"] += 1
                continue
            with open(os.path.join(root, rel), encoding="utf-8") as f:
                text = f.read()
            pos = yaml_line(text)
            if not pos:
                stats["no_position"] += 1
                continue
            stats["graph_listed"] += 1
            stats["yaml_files"] += 1
            files.add(rel)
            defs.append({"path": rel, "name": check.id, "line": pos[0], "end": pos[1], "kind": "check"})
            for rt in graph_resource_types(check, scalars(yaml.safe_load(text), set())):
                refs.append({"path": rel, "from": check.id, "to": rt})

    with open(a.out, "w") as f:
        json.dump({"tool": "checkov (--external-checks-dir loaders)", "version": version("checkov"), "tier": "A",
                   "definitions": sorted(defs, key=lambda x: (x["path"], x["line"], x["name"])),
                   "references": sorted(refs, key=lambda r: (r["path"], r["from"], r["to"]))}, f, indent=1)
        f.write("\n")
    if a.files_out:
        with open(a.files_out, "w") as f:
            f.write("".join(r + "\n" for r in sorted(files)))
    if a.summary_out:
        with open(a.summary_out, "w") as f:
            json.dump({"stats": stats}, f, indent=1, sort_keys=True)
            f.write("\n")
    print("checkov-oracle: " + ", ".join(f"{k} {v}" for k, v in stats.items()), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
