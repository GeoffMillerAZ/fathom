"""pyproject_oracle: the language-breadth harness's oracle for the
python-packaging doc-engine dialect (README.md "Tiers"): references tier A,
definitions tier C.

Each pyproject.toml is evaluated by the tools that own its tables, offline,
in the oracle's venv (no build isolation, no index):

  - [project]: the file's own build backend (build-system.build-backend and
    backend-path; setuptools' legacy backend when none is named, PEP 517)
    through PyPA's pyproject_hooks, prepare_metadata_for_build_wheel: the
    METADATA Name is the package, the entry_points.txt sections
    (console_scripts, gui_scripts, every other group) are the routes. A
    backend that fails rejects the file (named on stderr, left out of
    -files-out): nothing attests its entities.
  - [dependency-groups]: PyPA's dependency-groups package, the PEP 735
    reference implementation pip's --group runs: each group the resolver
    looks up and resolves is a target, and each include-group its lookup
    returns is a reference. A group it refuses is counted and left out.
  - [tool.*] tables are read by each tool alone and nothing evaluates them
    for all tools: tomllib (TOML's parser, not pyproject's: tier C) names
    each table whose value is a map, so the definitions file is tier C. The
    include-group references are their own file (-refs-out), tier A.

No tool prints a position, so each line comes from this program's own line
scanner (scan_lines: table headers and keys, never a TOML library), the
compose, envoy, cargo and custodian precedent. tomllib only reads which
tables a file has and the build-system keys to hand the hook caller.

  <venv>/bin/python pyproject_oracle.py -subject <checkout> -out <json> \
      [-refs-out <json>] [-files-out <file>] [-work <scratch dir outside the subject>]
"""

import argparse
import configparser
import email.parser
import json
import os
import re
import shutil
import sys
import tomllib

TOOL = "pyproject build backend (prepare_metadata_for_build_wheel via pyproject_hooks) + dependency-groups (PEP 735 reference implementation) + tomllib for [tool.*] tables"
REFS_TOOL = "dependency-groups (PEP 735 reference implementation: DependencyGroupResolver.lookup)"

# ── an independent line scanner ──────────────────────────────────────────

_header = re.compile(r"^\s*\[\[?\s*(.+?)\s*\]\]?\s*(#.*)?$")
_key = re.compile(r'^\s*((?:"[^"]*"|\'[^\']*\'|[A-Za-z0-9_-]+)(?:\s*\.\s*(?:"[^"]*"|\'[^\']*\'|[A-Za-z0-9_-]+))*)\s*=')
_part = re.compile(r'"([^"]*)"|\'([^\']*)\'|([A-Za-z0-9_-]+)')


def _parts(dotted):
    return tuple(a or b or c for a, b, c in _part.findall(dotted))


def _strip_strings(s):
    """The line with string contents blanked, so brackets inside strings do not count."""
    return re.sub(r'"(?:[^"\\]|\\.)*"|\'[^\']*\'', '""', s)


def scan_lines(src):
    """(table path + key path) -> 1-based line, for table headers and keys.

    Lines inside a multi-line array or string are skipped; an inline table's
    keys are recorded at its line."""
    out = {}
    table = ()
    depth = 0  # open [ / { of a multi-line value
    in_ml = None  # the triple quote of an open multi-line string
    for n, line in enumerate(src.splitlines(), 1):
        if in_ml:
            if in_ml in line:
                in_ml = None
            continue
        if depth > 0:
            depth += _depth(line)
            continue
        m = _header.match(line)
        if m and not _key.match(line):
            table = _parts(m.group(1))
            for i in range(1, len(table) + 1):
                out.setdefault(table[:i], n)  # an implicit table is at its first subtable
            continue
        k = _key.match(line)
        if not k:
            continue
        path = table + _parts(k.group(1))
        out.setdefault(path, n)
        for i in range(len(table) + 1, len(path)):
            out.setdefault(path[:i], n)
        rest = line[k.end():].strip()
        for q in ('"""', "'''"):
            if rest.startswith(q) and rest.count(q) == 1:
                in_ml = q
        if in_ml:
            continue
        if rest.startswith("{"):
            body = _strip_strings(rest)
            for inner in re.findall(r'(?:[{,]\s*)((?:"[^"]*"|[A-Za-z0-9_-]+))\s*=', rest):
                out.setdefault(path + _parts(inner), n)
            depth = _depth(body)
        elif rest.startswith("["):
            depth = _depth(rest)
    return out


def _depth(s):
    s = _strip_strings(s.split("#", 1)[0] if '"' not in s and "'" not in s else s)
    return s.count("[") + s.count("{") - s.count("]") - s.count("}")


# ── the tools ─────────────────────────────────────────────────────────────


def _backend_metadata(path, data, work):
    """The backend's METADATA Name and entry points, or raises."""
    import pyproject_hooks

    bs = data.get("build-system") or {}
    backend = bs.get("build-backend") or "setuptools.build_meta:__legacy__"
    backend_path = bs.get("backend-path")
    src = os.path.dirname(path)
    md = os.path.join(work, "md")
    shutil.rmtree(md, ignore_errors=True)
    os.makedirs(md)
    caller = pyproject_hooks.BuildBackendHookCaller(
        src, backend, backend_path=backend_path, python_executable=sys.executable,
        runner=pyproject_hooks.quiet_subprocess_runner)
    name = caller.prepare_metadata_for_build_wheel(md, {})
    di = os.path.join(md, name)
    with open(os.path.join(di, "METADATA"), encoding="utf-8") as f:
        meta = email.parser.Parser().parse(f, headersonly=True)
    eps = {}
    ep = os.path.join(di, "entry_points.txt")
    if os.path.exists(ep):
        cp = configparser.ConfigParser(delimiters=("=",), interpolation=None)
        cp.optionxform = str
        cp.read(ep, encoding="utf-8")
        for sec in cp.sections():
            eps[sec] = list(cp[sec].keys())
    return meta.get("Name", ""), eps


def run(subject, work):
    import dependency_groups

    # The definitions file is tier C: a [tool.*] table has no evaluator, so
    # tomllib names it, and one file carries one tier. The references (each
    # include-group the reference implementation returns) are their own file,
    # tier A.
    out = {"tool": TOOL, "version": "", "tier": "C", "definitions": [], "references": []}
    refs_out = {"tool": REFS_TOOL, "version": "", "tier": "A", "definitions": [], "references": []}
    stats = {"files": 0, "projects": 0, "projects_rejected": 0, "name_differs": 0, "routes": 0,
             "routes_unlocated": 0, "groups": 0, "groups_rejected": 0, "includes": 0, "sections": 0, "unlocated": 0}
    accepted, rejected = [], []
    files = []
    for dirpath, dirnames, filenames in os.walk(subject):
        dirnames[:] = sorted(d for d in dirnames if d not in (".git", "node_modules"))
        if "pyproject.toml" in filenames:
            files.append(os.path.relpath(os.path.join(dirpath, "pyproject.toml"), subject).replace(os.sep, "/"))
    for rel in sorted(files):
        stats["files"] += 1
        path = os.path.join(subject, rel)
        with open(path, "rb") as f:
            raw = f.read()
        try:
            data = tomllib.loads(raw.decode("utf-8"))
        except (tomllib.TOMLDecodeError, UnicodeDecodeError) as e:
            rejected.append(rel)
            print(f"rejected (not TOML): {rel}: {e}", file=sys.stderr)
            continue
        lines = scan_lines(raw.decode("utf-8"))
        defs, refs = [], []
        ok = True
        if isinstance(data.get("project"), dict) and "name" in data["project"]:
            stats["projects"] += 1
            try:
                name, eps = _backend_metadata(path, data, work)
            except Exception as e:  # the backend refusing the project
                ok = False
                stats["projects_rejected"] += 1
                print(f"rejected by its build backend: {rel}: {type(e).__name__}: {str(e).strip()[-300:]}", file=sys.stderr)
            else:
                if name != data["project"]["name"]:
                    stats["name_differs"] += 1
                    print(f"note: {rel}: METADATA Name {name!r}, project.name {data['project']['name']!r}", file=sys.stderr)
                ln = lines.get(("project",), 0)
                defs.append({"path": rel, "name": name, "line": ln, "kind": "package"})
                for sec, names in sorted(eps.items()):
                    table = {"console_scripts": ("project", "scripts"), "gui_scripts": ("project", "gui-scripts")}.get(
                        sec, ("project", "entry-points", sec))
                    for n in names:
                        stats["routes"] += 1
                        ln = lines.get(table + (n,), 0)
                        if not ln:
                            stats["routes_unlocated"] += 1
                            print(f"note: {rel}: entry point {sec}/{n} has no line the scanner places", file=sys.stderr)
                            continue
                        defs.append({"path": rel, "name": n, "line": ln, "kind": "route"})
        groups = data.get("dependency-groups")
        if isinstance(groups, dict):
            resolver = dependency_groups.DependencyGroupResolver(groups)
            for g, v in groups.items():
                if not isinstance(v, list):
                    continue
                try:
                    items = resolver.lookup(g)
                    resolver.resolve(g)
                except Exception as e:
                    stats["groups_rejected"] += 1
                    print(f"group refused by dependency-groups: {rel} {g}: {type(e).__name__}: {e}", file=sys.stderr)
                    continue
                stats["groups"] += 1
                ln = lines.get(("dependency-groups", g), 0)
                if not ln:
                    stats["unlocated"] += 1
                    continue
                defs.append({"path": rel, "name": g, "line": ln, "kind": "target"})
                for it in items:
                    inc = getattr(it, "include_group", None)
                    if inc is not None:
                        stats["includes"] += 1
                        refs.append({"path": rel, "from": g, "to": inc})
        tool = data.get("tool")
        if isinstance(tool, dict):
            for t, v in tool.items():
                if not isinstance(v, dict):
                    continue
                ln = lines.get(("tool", t), 0)
                if not ln:
                    stats["unlocated"] += 1
                    continue
                stats["sections"] += 1
                defs.append({"path": rel, "name": t, "line": ln, "kind": "section"})
        if not ok:
            rejected.append(rel)
            continue
        accepted.append(rel)
        out["definitions"].extend(defs)
        refs_out["references"].extend(refs)
    out["definitions"].sort(key=lambda d: (d["path"], d["line"], d["kind"], d["name"]))
    refs_out["references"].sort(key=lambda r: (r["path"], r["from"], r["to"]))
    out["version"] = versions()
    refs_out["version"] = versions()
    return out, refs_out, stats, accepted, rejected


def versions():
    from importlib.metadata import version, PackageNotFoundError

    vs = []
    for p in ("pyproject_hooks", "dependency-groups", "flit_core", "pdm-backend", "setuptools"):
        try:
            vs.append(f"{p} {version(p)}")
        except PackageNotFoundError:
            pass
    return "; ".join(vs) + f"; python {sys.version.split()[0]}"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-refs-out", dest="refs_out", help="the tier-A references file")
    ap.add_argument("-files-out", dest="files_out")
    ap.add_argument("-work", help="scratch directory outside the subject (default: <out>.work)")
    a = ap.parse_args()
    subject = os.path.abspath(a.subject)
    work = os.path.abspath(a.work or a.out + ".work")
    if os.path.commonpath([subject, work]) == subject:
        sys.exit("pyproject_oracle: -work must be outside the subject")
    os.makedirs(work, exist_ok=True)
    out, refs_out, stats, accepted, rejected = run(subject, work)
    with open(a.out, "w") as f:
        json.dump(out, f, indent=1)
        f.write("\n")
    if a.refs_out:
        with open(a.refs_out, "w") as f:
            json.dump(refs_out, f, indent=1)
            f.write("\n")
    if a.files_out:
        with open(a.files_out, "w") as f:
            f.write("".join(p + "\n" for p in accepted))
    print("pyproject_oracle: " + json.dumps(stats), file=sys.stderr)
    for r in rejected:
        print(f"  rejected: {r}", file=sys.stderr)


if __name__ == "__main__":
    main()
