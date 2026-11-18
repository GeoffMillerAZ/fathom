"""Pinned CPython modulefinder adapter for direct, source-only DEP-M2 truth.

Run under python3.13 -I -B -S. Subjects are clean checkouts in CORPUS/name.
The adapter never imports subject modules or installs subject dependencies.
"""
import argparse
import ast
import dis
import hashlib
import json
import modulefinder
import os
from pathlib import Path
import stat
import subprocess
import sys

PINS = Path(__file__).with_name("pins.json")
RUNTIME_NAMES = {
    "__import__", "__builtins__", "__loader__", "__package__", "__path__", "__spec__",
    "declare_namespace", "eval", "exec", "extend_path", "getattr", "globals", "import_module", "locals", "setattr",
}
RUNTIME_MODULES = {"builtins", "imp", "importlib", "pkg_resources", "pkgutil", "runpy", "zipimport"}
RUNTIME_MEMBERS = {
    "exec_module", "load_module", "meta_path", "path_hooks", "path_importer_cache", "spec_from_file_location",
}


def pin():
    pins = json.loads(PINS.read_text(encoding="utf-8"))
    version = ".".join(map(str, sys.version_info[:3]))
    digest = hashlib.sha256(Path(modulefinder.__file__).read_bytes()).hexdigest()
    if (sys.implementation.name, version, digest, sys.version_info.releaselevel) != (
        pins["implementation"], pins["version"], pins["modulefinder_sha256"], "final",
    ):
        raise ValueError("CPython or modulefinder identity differs from pins.json")
    if not (sys.flags.isolated and sys.flags.dont_write_bytecode and sys.flags.no_site):
        raise ValueError("invoke the pinned Python with -I -B -S")
    return pins


def git(root, *argv):
    return subprocess.check_output(["git", "-C", str(root), *argv])


def contents(root, subject):
    if Path(os.fsdecode(git(root, "rev-parse", "--show-toplevel")).strip()).resolve() != root:
        raise ValueError(f"{subject['name']}: expected a checkout root")
    if git(root, "rev-parse", "HEAD").decode().strip() != subject["commit"]:
        raise ValueError(f"{subject['name']}: HEAD differs from the pin")
    if git(root, "status", "--porcelain=v1", "--untracked-files=all", "--ignored=matching"):
        raise ValueError(f"{subject['name']}: checkout contains changed, generated or untracked files")
    manifest = hashlib.sha256()
    names = []
    counts = dict(files=0, bytes=0, symlinks=0, gitlinks=0, python_bytes=0)
    rows = git(root, "ls-files", "-z", "--stage").split(b"\0")
    for row in sorted(filter(None, rows), key=lambda r: r.split(b"\t", 1)[1]):
        identity, filename = row.split(b"\t", 1)
        mode, oid, stage = identity.split()
        if stage != b"0":
            raise ValueError("unmerged corpus index")
        name = os.fsdecode(filename)
        p = root / name
        if mode == b"160000":
            kind, fingerprint = "gitlink", oid.decode()
            counts["gitlinks"] += 1
        elif mode == b"120000" and p.is_symlink():
            kind = "symlink"
            fingerprint = hashlib.sha256(os.fsencode(os.readlink(p))).hexdigest()
            counts["symlinks"] += 1
        elif mode in (b"100644", b"100755") and stat.S_ISREG(p.lstat().st_mode):
            kind = "file"
            data = p.read_bytes()
            fingerprint = hashlib.sha256(data).hexdigest()
            counts["files"] += 1
            counts["bytes"] += len(data)
            if name.endswith(".py"):
                names.append(name)
                counts["python_bytes"] += len(data)
        else:
            raise ValueError(f"unexpected tracked entry type: {name}")
        manifest.update(json.dumps([name, kind, fingerprint], ensure_ascii=True, separators=(",", ":")).encode() + b"\n")
    if len(names) != subject["files"]:
        raise ValueError(f"{subject['name']}: tracked Python file count differs from the pin")
    return names, dict(counts, sha256=manifest.hexdigest())


def runtime_construct(entry):
    """Flag source-level forms outside the modulefinder bytecode model."""
    aliases = {"sys"}
    for node in ast.walk(entry):
        if isinstance(node, ast.Import):
            for alias in node.names:
                head = alias.name.split(".")[0]
                if head in RUNTIME_MODULES:
                    return True
                if head == "sys":
                    aliases.add(alias.asname or "sys")
        if isinstance(node, ast.ImportFrom) and node.level == 0:
            if node.module in RUNTIME_MODULES or (
                node.module == "sys"
                and any(a.name in {"modules", "path", *RUNTIME_MEMBERS} for a in node.names)
            ):
                return True
    return any(
        isinstance(node, ast.Name) and node.id in RUNTIME_NAMES
        or isinstance(node, ast.Attribute) and (
            node.attr in RUNTIME_NAMES | RUNTIME_MEMBERS
            or isinstance(node.value, ast.Name) and node.value.id in aliases and node.attr in {"modules", "path"}
        )
        for node in ast.walk(entry)
    )


class EntryFinder(modulefinder.ModuleFinder):
    """Keep modulefinder's binding rules and trace imports attributed to one file."""

    def __init__(self, root, roots):
        # Isolated startup supplies only the pinned interpreter's stdlib.
        super().__init__(path=[*map(str, roots), *sys.path])
        self.root = root
        self.active = None
        self.edges = set()
        self.gaps = set()
        self.bytecode_imports = 0

    def set_entry(self, roots, filename):
        for root in roots:
            if not filename.is_relative_to(root):
                continue
            source = filename.relative_to(root)
            parents = source.parts[:-1]
            if not all((root.joinpath(*parents[:idx]) / "__init__.py").is_file()
                       for idx in range(1, len(parents) + 1)):
                continue
            # run_script registers __main__ and cannot bind relative imports.
            # Establish the regular package identity without executing any code.
            for idx in range(1, len(parents) + 1):
                pkg = self.add_module(".".join(parents[:idx]))
                pkg.__path__ = [str(root.joinpath(*parents[:idx]))]
                pkg.__file__ = str(Path(pkg.__path__[0], "__init__.py"))
            if parents and source.name == "__init__.py":
                self.entry = pkg
            else:
                self.entry = self.add_module(".".join((*parents, source.stem)) if parents else "__main__")
                self.entry.__file__ = str(filename)
            return
        self.entry = self.add_module("__main__")
        self.entry.__file__ = str(filename)

    def scan_code(self, co, module):
        # Load direct targets to let modulefinder establish filenames. Their
        # code cannot contribute imports or make direct bindings depend on
        # which dependencies happened to be traversed first.
        if module is self.entry:
            super().scan_code(co, module)

    def scan_opcodes(self, co):
        imports = iter(i for i in dis.get_instructions(co) if i.opname == "IMPORT_NAME")
        for opcode, args in super().scan_opcodes(co):
            if opcode != "store":
                site = next(imports, None)
                level, fromlist, name = (0, *args) if opcode == "absolute_import" else args
                if site is None or site.argval != name or site.positions.lineno is None:
                    raise ValueError("modulefinder opcode has no matching native source position")
                self.active = ("." * level + name, site.positions.lineno, level, fromlist)
                self.bytecode_imports += 1
            yield opcode, args
        self.active = None
        if next(imports, None) is not None:
            raise ValueError("native IMPORT_NAME was omitted by modulefinder")

    def import_module(self, partname, fqname, parent):
        loaded = super().import_module(partname, fqname, parent)
        if loaded is None or self.active is None or loaded is self.entry:
            return loaded
        raw, line, _, _ = self.active
        if loaded.__file__:
            filename = Path(loaded.__file__).absolute()
            if filename.is_relative_to(self.root):
                if filename.suffix != ".py" or filename.resolve() != filename:
                    self.gaps.add("native-target")
                else:
                    self.edges.add((line, raw, "file", filename.relative_to(self.root).as_posix()))
                return loaded
        self.edges.add((line, raw, "external", "python:" + loaded.__name__))
        return loaded

    def find_head_package(self, parent, name):
        head, tail = super().find_head_package(parent, name)
        if parent is not None and not head.__name__.startswith(parent.__name__ + "."):
            self.gaps.add("relative-fallback")
        return head, tail

    def load_tail(self, head, tail):
        mod = super().load_tail(head, tail)
        if self.active and self.active[3] and mod.__path__ is not None:
            # importlib checks live package attributes, __getattr__ and
            # __all__. Modulefinder's Module object has none of that state.
            self.gaps.add("package-fromlist")
        return mod

    def _add_badmodule(self, name, caller):
        super()._add_badmodule(name, caller)
        if self.active is not None:
            self.gaps.add("unresolved-direct")


def read_entry(root, roots, name):
    row = {"path": name, "edges": [], "gap": "", "optimized_away": 0}
    filename = root / name
    data = filename.read_bytes()
    try:
        entry = ast.parse(data, name)
        code = compile(data, name, "exec", dont_inherit=True, optimize=0)
    except (SyntaxError, ValueError):
        row["gap"] = "entry-compile"
        return row
    if runtime_construct(entry):
        row["gap"] = "runtime"
        return row

    finder = EntryFinder(root, roots)
    try:
        finder.set_entry(roots, filename)
        finder.scan_code(code, finder.entry)
    except AttributeError as exc:
        # PathFinder gives a namespace spec without a concrete loader.
        row["gap"] = "namespace" if "'NoneType' object has no attribute 'is_package'" in str(exc) else "finder-error"
    except (ImportError, KeyError):
        row["gap"] = "relative-context" if finder.active and finder.active[2] else "finder-error"
    except Exception as exc:
        row["gap"] = "finder-error"
        print(f"{name}: {type(exc).__name__}: {exc}", file=sys.stderr)

    if not row["gap"] and finder.gaps:
        # Give the structural cause before a consequent failed submodule lookup.
        row["gap"] = min(finder.gaps)
    row["edges"] = [
        {"Kind": "import", "Raw": raw, "Line": line, "Resolution": resolution, "To": to}
        for line, raw, resolution, to in sorted(finder.edges)
    ]
    if not row["gap"]:
        statements = sum(len(n.names) if isinstance(n, ast.Import) else isinstance(n, ast.ImportFrom) for n in ast.walk(entry))
        row["optimized_away"] = max(0, statements - finder.bytecode_imports)
    return row


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("corpus", type=Path)
    args = parser.parse_args()
    pins = pin()
    subjects = {}
    for subject in pins["subjects"]:
        name = subject["name"]
        root = (args.corpus / name).resolve(strict=True)
        roots = [(root / member).resolve(strict=True) for member in subject["roots"]]
        names, before = contents(root, subject)
        rows = [read_entry(root, roots, entry) for entry in names]
        if (names, before) != contents(root, subject):
            raise ValueError(f"{name}: inputs changed during oracle collection")
        subjects[name] = {"manifest": before, "rows": rows}
    json.dump(subjects, sys.stdout, sort_keys=True, separators=(",", ":"))
    print()


if __name__ == "__main__":
    main()
