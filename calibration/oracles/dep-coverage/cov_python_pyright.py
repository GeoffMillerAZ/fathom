"""Cross-check pinned CPython development truth with Pyright's direct graph."""
import argparse
import ast
import base64
from collections import defaultdict
import hashlib
import json
import os
from pathlib import Path
import re
from runpy import run_path
import shutil
import stat
import subprocess
from urllib.parse import unquote, urlsplit

HERE = Path(__file__).resolve().parent
VERSION = "1.1.408"
NODE_VERSION = "v26.12.0"
ARCHIVE_DIGEST = "16f73d4281d00ddbd549d5dbe574d0f2cf0aec34f276f91bd6ae63fe1001473e"
ARCHIVE_SRI = "sha512-N61pxaLLCsPcUuPPHMNIrGoZgGBgrbjBX5UqkaT5UV8NVZdL7ExsO6N3ectv1DzAUsLOzdlyqoYtX76u8eF4YA=="
MANIFEST_DIGEST = "01c662bc7575cec6217f9f42ad254e8169448ea1141776286ddbdac361823662"
LAUNCH_DIGEST = "1b6744d5925694667abb3309c7e27611446aa7f1ee645cb99ecb0b9082a4c61e"
TOOL_FILES = 5257
SANDBOX = '(version 1)\n(allow default)\n(deny network*)\n(deny process-fork)'
IMPORTS = re.compile(r" Imports     (\d+) files?")
IMPORTED_BY = re.compile(r" Imported by (\d+) files?")


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def verify_tool(cache):
    cache = cache.resolve(strict=True)
    pin = json.loads((cache / "pin.json").read_bytes())
    expected = dict(name="pyright", version=VERSION, node=NODE_VERSION, tool_files=TOOL_FILES,
                    archive_sha256=ARCHIVE_DIGEST, archive_sha512_sri=ARCHIVE_SRI,
                    tool_files_manifest_sha256=MANIFEST_DIGEST, launch_sha256=LAUNCH_DIGEST)
    if any(pin.get(key) != value for key, value in expected.items()):
        raise ValueError("Pyright pin record differs")
    archive = (cache / ("pyright-" + VERSION + ".tgz")).read_bytes()
    sri = "sha512-" + base64.b64encode(hashlib.sha512(archive).digest()).decode()
    manifest = (cache / "package.sha256").read_bytes()
    if sha256(archive) != ARCHIVE_DIGEST or sri != ARCHIVE_SRI or sha256(manifest) != MANIFEST_DIGEST:
        raise ValueError("Pyright archive or manifest differs")

    package = cache / "package"
    if package.resolve(strict=True) != package:
        raise ValueError("Pyright package is not physical")
    files = {}
    for path in package.rglob("*"):
        mode = path.lstat().st_mode
        if stat.S_ISREG(mode):
            files[path.relative_to(package).as_posix()] = sha256(path.read_bytes())
        elif not stat.S_ISDIR(mode):
            raise ValueError("Pyright package contains a nonphysical entry")
    listed = {name: digest for digest, name in (row.split("  ", 1) for row in manifest.decode().splitlines())}
    if len(files) != TOOL_FILES or files != listed or files.get("index.js") != LAUNCH_DIGEST:
        raise ValueError("Pyright extracted file inventory differs")

    node = shutil.which("node")
    if node is None or run_node(node, ["--version"], cache).stdout.strip() != NODE_VERSION:
        raise ValueError("Node version differs")
    return node, package / "index.js"


def run_node(node, args, work, *, original=None, check=True):
    profile = SANDBOX
    options = []
    if original is not None:
        profile += '\n(deny file-read* (subpath (param "ORIGINAL")))'
        options = ["-D", f"ORIGINAL={original}"]
    return subprocess.run(
        ["/usr/bin/sandbox-exec", *options, "-p", profile, node, *args],
        cwd=work, env={"HOME": str(work), "TMPDIR": str(work), "PATH": "/usr/bin:/bin"},
        text=True, capture_output=True, check=check, timeout=180,
    )


def parse_uri(line):
    uri = urlsplit(line.removeprefix("    "))
    if not line.startswith("    file://") or uri.netloc or uri.query or uri.fragment:
        raise ValueError("unexpected Pyright graph URI")
    return Path(unquote(uri.path, errors="strict"))


def parse_graph(root, tracked, output):
    # The pinned Program.printDependencies emits a header relative to the
    # project config when possible, counted direct/outgoing URIs, then reverse URIs.
    lines = output.splitlines()
    if f"pyright {VERSION}" not in lines:
        raise ValueError("unexpected Pyright graph version")
    graph = {}
    for index, line in enumerate(lines):
        match = IMPORTS.fullmatch(line)
        if match is None:
            continue
        label = lines[index - 1]
        source = parse_uri("    " + label) if label.startswith("file://") else root.parent / label
        imports_end = index + int(match[1]) + 1
        if imports_end >= len(lines) or (reverse := IMPORTED_BY.fullmatch(lines[imports_end])) is None:
            raise ValueError("invalid Pyright direct edge count")
        reverse_end = imports_end + int(reverse[1]) + 1
        if reverse_end > len(lines) or (reverse_end < len(lines) and lines[reverse_end]):
            raise ValueError("invalid Pyright reverse edge count")
        direct = set()
        for target in map(parse_uri, lines[index + 1:imports_end]):
            if target.is_relative_to(root):
                name = target.relative_to(root).as_posix()
                if name not in tracked:
                    raise ValueError("Pyright graph refers to untracked source")
                direct.add(name)
        for line in lines[imports_end + 1:reverse_end]:
            parse_uri(line)
        if source.is_relative_to(root):
            name = source.relative_to(root).as_posix()
            if name not in tracked or name in graph:
                raise ValueError("unexpected or duplicate Pyright source row")
            graph[name] = direct
    return graph


class NativeNonzero(Exception):
    """The pinned graph command returned a positive exit status."""


def run_graph(tool, view, roots, tracked, selected, work, *, original=None, classify_exit=False):
    if (view.parent != work or view.name != "00-view" or view.resolve(strict=True) != view
            or set(work.iterdir()) != {view}):
        raise ValueError("Pyright requires a fresh isolated project")
    physical = set()
    for path in view.rglob("*"):
        mode = path.lstat().st_mode
        if stat.S_ISREG(mode):
            physical.add(path.relative_to(view).as_posix())
        elif not stat.S_ISDIR(mode):
            raise ValueError("nonphysical file in Pyright view")
    if physical != tracked or not selected <= tracked:
        raise ValueError("Pyright view differs from the tracked inventory")
    empty = work / "00-venvs/empty/lib/python3.13/site-packages"
    empty.mkdir(parents=True)
    stubs = work / "00-stubs"
    stubs.mkdir()
    search = [view.joinpath(root) for root in roots]
    if any(not path.is_relative_to(view) or not path.is_dir() or path.resolve() != path for path in search):
        raise ValueError("invalid pinned Pyright roots")
    config = {
        "include": [os.path.relpath(view / name, work) for name in sorted(selected)],
        # Pyright picks execution environments by the source file's path.
        # An empty module root covers all entries and preserves the pinned
        # order in extraPaths even for sources outside the first native root.
        "executionEnvironments": [{"root": ".", "extraPaths": list(map(str, search))}],
        "extraPaths": [], "typeCheckingMode": "off",
        "pythonVersion": "3.13", "pythonPlatform": "Darwin",
        "stubPath": stubs.name, "venvPath": "00-venvs", "venv": "empty", "useLibraryCodeForTypes": True,
    }
    project = work / "pyrightconfig.json"
    project.write_text(json.dumps(config, sort_keys=True), encoding="utf-8")
    node, launch = tool
    result = run_node(node, [str(launch), "--project", str(project), "--dependencies", "--verbose"],
                      work, original=original, check=False)
    (work / "graph.log").write_text(result.stdout, encoding="utf-8")
    (work / "stderr.log").write_text(result.stderr, encoding="utf-8")
    if classify_exit and result.returncode > 0:
        raise NativeNonzero("native truth tool exited nonzero")
    if result.returncode or result.stderr:
        raise ValueError("Pyright graph collection failed; inspect the private logs")
    typeshed = launch.parent / "dist/typeshed-fallback"
    expected_paths = [typeshed / "stdlib", work, *search, stubs, typeshed / "stubs/...", empty]
    _, _, listing = result.stdout.partition("  Search paths:\n")
    actual_paths = []
    for line in listing.splitlines():
        if not line.startswith("    "):
            break
        actual_paths.append(line.strip())
    if actual_paths != list(map(str, expected_paths)):
        raise ValueError("Pyright effective search path differs")
    return parse_graph(view, tracked, result.stdout)


def literal_sites(source, entry, roots, tracked):
    """Map possible targets to syntax sites without guessing a graph edge's origin."""
    sites, children = defaultdict(set), defaultdict(set)
    absolute_from = set()
    path = Path(entry)
    package = next(path.relative_to(root).parent.parts for root in map(Path, roots) if path.is_relative_to(root))
    for node in ast.walk(ast.parse(source)):
        if isinstance(node, ast.Import):
            operands = [(alias.name, alias.name.split("."), (), alias.col_offset) for alias in node.names]
        elif isinstance(node, ast.ImportFrom):
            if node.level > len(package):
                continue
            anchor = list(package[:len(package) - node.level + 1]) if node.level else []
            parts = anchor + (node.module.split(".") if node.module else [])
            operands = [("." * node.level + (node.module or ""), parts,
                         [alias.name for alias in node.names], node.col_offset)]
        else:
            continue
        for raw, parts, members, column in operands:
            site = (node.lineno, column, raw)
            if isinstance(node, ast.ImportFrom) and node.level == 0 and len(parts) == 3:
                absolute_from.add(site)
            for root in map(Path, roots):
                for length in range(1, len(parts) + 1):
                    target = root.joinpath(*parts[:length])
                    sites[(target / "__init__.py").as_posix()].add(site)
                    if length == len(parts):
                        sites[target.with_suffix(".py").as_posix()].add(site)
                base = root.joinpath(*parts)
                initializer = (base / "__init__.py").as_posix()
                for member in members:
                    if member == "*":
                        # A star can request any child listed by the package.
                        for filename in tracked:
                            if Path(filename).is_relative_to(base):
                                sites[filename].add(site)
                        continue
                    for target in (base / (member + ".py"), base / member / "__init__.py"):
                        sites[target.as_posix()].add(site)
                        children[target.as_posix()].add((site, initializer))
    return sites, children, absolute_from


def compare_row(native, graph, source, entry, roots, tracked):
    """Return a class and B1 targets only where the native sources support them."""
    local = defaultdict(set)
    for edge in native["edges"]:
        if edge["Resolution"] == "file":
            local[edge["To"]].add((edge["Line"], edge["Raw"]))
    expected = set(local)
    if graph is None:
        return "missing-graph", None
    sites, children, absolute_from = literal_sites(source, entry, roots, tracked)
    if graph < expected:
        if all(target.endswith("/__init__.py") and len(sites[target]) == 1
               and sites[target] <= absolute_from
               and {(line, raw) for line, _, raw in sites[target]} == local[target]
               for target in expected - graph):
            return "pyright-omits-absolute-from-initializer", None
        return "pyright-subset", None
    if not expected <= graph:
        return "incomparable", None
    if any(len(sites[target]) > 1 for target in graph):
        return "ambiguous-site", None
    if any({(line, raw) for line, _, raw in sites[target]} != local[target] for target in expected):
        return "unattributed-site", None
    extras = graph - expected
    if not extras:
        return "exact", expected
    for target in extras:
        parents = children[target]
        if len(parents) != 1:
            return "pyright-other-superset", None
        site, initializer = next(iter(parents))
        if initializer not in expected or sites[initializer] != {site} or sites[target] != {site}:
            return "pyright-other-superset", None
    return "initializer-plus-submodule", graph


def stage(oracle, root, subject, view):
    names, manifest = oracle["contents"](root, subject, confined=True)
    if manifest["symlinks"] or manifest["gitlinks"]:
        raise ValueError("Pyright view requires a regular-file-only subject")
    tracked = set()
    for row in filter(None, oracle["git"](root, "ls-files", "-z", "--stage").split(b"\0")):
        _, raw = row.split(b"\t", 1)
        name = os.fsdecode(raw)
        source = root / name
        if not oracle["confined_file"](root, source):
            raise ValueError("unconfined file in pinned subject")
        destination = view / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(source.read_bytes())
        tracked.add(name)
    return names, manifest, tracked


def collect(args):
    work = args.work.resolve()
    work.mkdir(mode=0o700)
    tool = verify_tool(args.cache)
    oracle = run_path(str(HERE.parent / "deps-python/modulefinder_oracle.py"))
    pins = oracle["pin"](declarative_fromlist=True)
    frozen, extension = (json.loads(path.read_bytes()) for path in (args.frozen, args.extension))
    report = {}
    for subject in pins["subjects"]:
        name = subject["name"]
        root = oracle["checkout_root"](args.cohort / name, confined=True)
        subject_work = work / name
        subject_work.mkdir(mode=0o700)
        # Only tracked regular files are available to the generated project.
        view = subject_work / "00-view"
        names, manifest, tracked = stage(oracle, root, subject, view)
        if frozen[name]["manifest"] != manifest or extension[name]["manifest"] != manifest:
            raise ValueError("CPython and Pyright subject manifests differ")
        indices = [{row["path"]: row for row in report[name]["rows"]} for report in (frozen, extension)]
        if any(set(index) != set(names) for index in indices):
            raise ValueError("CPython and Pyright populations differ")
        graph = run_graph(tool, view, subject["roots"], tracked, set(names), subject_work, original=root)
        if (names, manifest) != oracle["contents"](root, subject, confined=True):
            raise ValueError("subject changed during the Pyright run")
        old, new = indices
        compared = {}
        for path in names:
            row = old[path] if not old[path]["gap"] else new[path]
            if row["gap"]:
                continue
            kind, truth = compare_row(row, graph.get(path), (view / path).read_bytes(),
                                      path, subject["roots"], tracked)
            compared[path] = {"class": kind, "truth": sorted(truth) if truth is not None else None,
                              "cohort": "pre-existing" if not old[path]["gap"] else "new"}
        graph_data = json.dumps({path: sorted(edges) for path, edges in graph.items()},
                                sort_keys=True, separators=(",", ":")).encode()
        report[name] = dict(manifest=manifest, commit=subject["commit"], roots=subject["roots"],
                            graph_rows=len(set(names) & graph.keys()), graph_sha256=sha256(graph_data),
                            compared=compared)
    return {"tool": dict(version=VERSION, node=NODE_VERSION, archive_sha256=ARCHIVE_DIGEST,
                         manifest_sha256=MANIFEST_DIGEST, launch_sha256=LAUNCH_DIGEST, files=TOOL_FILES),
            "subjects": report}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for option in ("cache", "cohort", "frozen", "extension", "work"):
        parser.add_argument("--" + option, type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(collect(args), sort_keys=True, separators=(",", ":")))


if __name__ == "__main__":
    main()
