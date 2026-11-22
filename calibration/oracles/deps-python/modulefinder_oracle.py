"""Pinned CPython modulefinder adapter for direct, source-only DEP-M2 truth.

Run under python3.13 -I -B -S. Subjects are clean checkouts in CORPUS/name.
The adapter never imports subject modules or installs subject dependencies.
"""
import argparse
import ast
import dis
import hashlib
from importlib.machinery import (
    AppleFrameworkLoader, ExtensionFileLoader, FrozenImporter, PathFinder,
    SourceFileLoader, SourcelessFileLoader, all_suffixes,
)
import io
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
CHILD_SUFFIXES = (*all_suffixes(), ".fwork", ".pyd")


class UnconfinedSource(ValueError):
    pass


class NamespaceTarget(ValueError):
    pass


def confined_file(root, filename):
    return (filename.is_relative_to(root) and filename.resolve(strict=True) == filename
            and stat.S_ISREG(filename.lstat().st_mode))


def checkout_root(path, *, confined):
    resolved = path.resolve(strict=True)
    if confined and resolved != path.absolute():
        raise UnconfinedSource("checkout path is not physical")
    return resolved


def pin(*, declarative_fromlist=False):
    pins = json.loads(PINS.read_text(encoding="utf-8"))
    version = ".".join(map(str, sys.version_info[:3]))
    digest = hashlib.sha256(Path(modulefinder.__file__).read_bytes()).hexdigest()
    if (sys.implementation.name, version, digest, sys.version_info.releaselevel) != (
        pins["implementation"], pins["version"], pins["modulefinder_sha256"], "final",
    ):
        raise ValueError("CPython or modulefinder identity differs from pins.json")
    if not (sys.flags.isolated and sys.flags.dont_write_bytecode and sys.flags.no_site):
        raise ValueError("invoke the pinned Python with -I -B -S")
    if declarative_fromlist and (sys.flags.optimize or modulefinder.packagePathMap or modulefinder.replacePackageMap):
        raise ValueError("declarative from-list requires stock, unoptimized modulefinder")
    return pins


def git(root, *argv):
    return subprocess.check_output(["git", "-C", str(root), *argv])


def contents(root, subject, *, confined=False):
    if Path(os.fsdecode(git(root, "rev-parse", "--show-toplevel")).strip()).resolve() != root:
        raise ValueError(f"{subject['name']}: expected a checkout root")
    if git(root, "rev-parse", "HEAD").decode().strip() != subject["commit"]:
        raise ValueError(f"{subject['name']}: HEAD differs from the pin")
    def require_clean():
        if git(root, "status", "--porcelain=v1", "--untracked-files=all", "--ignored=matching"):
            raise ValueError(f"{subject['name']}: checkout contains changed, generated or untracked files")

    if not confined:
        require_clean()
    manifest = hashlib.sha256()
    names = []
    counts = dict(files=0, bytes=0, symlinks=0, gitlinks=0, python_bytes=0)
    rows = git(root, "ls-files", "-z", "--stage").split(b"\0")
    if confined:
        for row in filter(None, rows):
            identity, filename = row.split(b"\t", 1)
            if identity.split()[0] in (b"100644", b"100755") and not confined_file(root, root / os.fsdecode(filename)):
                raise UnconfinedSource("tracked file is outside the physical checkout")
        require_clean()
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


def declarative_bindings(filename, name):
    """Require syntactic declarations and CPython module-scope store evidence."""
    data = filename.read_bytes()
    tree = ast.parse(data, name)
    code = compile(data, name, "exec", dont_inherit=True, optimize=0)
    if runtime_construct(tree):
        return None
    reserved = RUNTIME_NAMES | RUNTIME_MEMBERS | {
        "__cached__", "__class__", "__dict__", "__doc__", "__file__", "__getattr__", "__name__",
    }
    bindings = set()
    for stmt in tree.body:
        match stmt:
            case ast.Pass() | ast.Expr(value=ast.Constant()):
                continue
            case ast.Import(names=imports):
                added = [alias.asname or alias.name.split(".")[0] for alias in imports]
            case ast.ImportFrom(names=imports):
                added = [alias.asname or alias.name for alias in imports]
            case ast.Assign(targets=targets, value=value) if all(isinstance(t, ast.Name) for t in targets):
                # literal_eval also accepts set(), which still calls a name.
                if any(isinstance(node, ast.Call) for node in ast.walk(value)):
                    return None
                ast.literal_eval(value)
                added = [target.id for target in targets]
            case _:
                return None
        names = set(added)
        if len(names) != len(added) or names & (bindings | reserved | {"*"}):
            return None
        bindings.update(names)
    # Do not recurse into code objects or ask modulefinder to load initializer
    # imports. Its stock opcode scanner supplies native STORE_NAME evidence.
    native = {args[0] for op, args in modulefinder.ModuleFinder().scan_opcodes(code) if op == "store"}
    return bindings & native


class EntryFinder(modulefinder.ModuleFinder):
    """Keep modulefinder's binding rules and trace imports attributed to one file."""

    def __init__(self, root, roots, tracked=None):
        # Isolated startup supplies only the pinned interpreter's stdlib.
        super().__init__(path=[*map(str, roots), *sys.path])
        self.root = root
        self.tracked = tracked
        self.declarations = {}
        self.package_children = {}
        self.declared_site = None
        self.active = None
        self.edges = set()
        self.gaps = set()
        self.bytecode_imports = 0

    def set_entry(self, roots, filename):
        if self.tracked is not None:
            self.set_native_entry(roots, filename)
            return
        for root in roots:
            if not filename.is_relative_to(root):
                continue
            source = filename.relative_to(root)
            parents = source.parts[:-1]
            initializers = [root.joinpath(*parents[:idx], "__init__.py") for idx in range(1, len(parents) + 1)]
            if not all(path.is_file() for path in initializers):
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

    def set_native_entry(self, roots, filename):
        root = next((r for r in roots if filename.is_relative_to(r)), None)
        if root is None:
            raise UnconfinedSource("entry is outside the declared search roots")
        source = filename.relative_to(root)
        package = source.name == "__init__.py" and len(source.parts) > 1
        parts = source.parts[:-1] if package else (*source.parts[:-1], source.stem)
        if not all(part.isidentifier() for part in parts):
            raise UnconfinedSource("entry has no unambiguous module name")
        chain = self.native_source_chain(parts, package=package, namespace_root=root)
        if chain is None or chain[-1] != filename:
            raise UnconfinedSource("entry differs from native search order")

        if None in chain or (len(parts) == 1 and not package):
            # A unique namespace can locate the file, but modulefinder cannot
            # bind relative imports through it. Do not seed a package or __main__.
            self.entry = modulefinder.Module("__main__")
            self.entry.__file__ = str(filename)
            return
        for index, origin in enumerate(chain):
            module = self.add_module(".".join(parts[:index + 1]))
            module.__file__ = str(origin)
            if index < len(parts) - 1 or package:
                module.__path__ = [str(origin.parent)]
        self.entry = module

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

    def tracked_source(self, source):
        return (source.is_relative_to(self.root)
                and source.relative_to(self.root).as_posix() in self.tracked
                and confined_file(self.root, source))

    def find_module(self, name, path, parent=None):
        if self.tracked is None or (path is None and name in sys.builtin_module_names):
            return super().find_module(name, path, parent)

        # The stock finder opens a non-package origin inside _find_module.
        # Classify the same native spec and open only its validated origin.
        search = self.path if path is None else path
        PathFinder.invalidate_caches()
        spec = PathFinder.find_spec(name, search)
        if spec is None:
            raise ImportError(name)
        if spec.loader is None:
            raise NamespaceTarget(name)
        if spec.origin is None:
            raise UnconfinedSource("finder origin is unavailable")
        source = Path(spec.origin)
        if source.is_relative_to(self.root):
            if type(spec.loader) is not SourceFileLoader or not self.tracked_source(source):
                raise UnconfinedSource("finder origin is outside the tracked source view")
            if spec.submodule_search_locations is not None and source.name != "__init__.py":
                raise UnconfinedSource("unsupported package initializer")
        elif source.resolve().is_relative_to(self.root):
            raise UnconfinedSource("external finder origin points into the checkout")

        if spec.loader.is_package(name):
            return None, str(source.parent), ("", "", modulefinder._PKG_DIRECTORY)
        match spec.loader:
            case SourceFileLoader():
                kind = modulefinder._PY_SOURCE
            case ExtensionFileLoader() | AppleFrameworkLoader():
                kind = modulefinder._C_EXTENSION
            case SourcelessFileLoader():
                kind = modulefinder._PY_COMPILED
            case _:
                raise UnconfinedSource("unsupported finder loader")
        return io.open_code(str(source)), str(source), (source.suffix, "rb", kind)

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
        if self.active and self.active[3] and mod.__path__ is not None and not self.proves_fromlist(mod):
            # importlib checks live package attributes, __getattr__ and
            # __all__. Modulefinder's Module object has none of that state.
            self.gaps.add("package-fromlist")
        return mod

    def proves_fromlist(self, mod):
        if self.tracked is None:
            return False
        site = (self.active, mod)
        if self.declared_site == site:
            return True
        raw, line, level, names = self.active
        if "*" in names or mod is self.entry or not mod.__file__:
            return False
        try:
            if level:
                package = self.entry
                if not package.__path__:
                    package = self.modules.get(package.__name__.rpartition(".")[0])
                if package is None or self.package_bindings(package) is None:
                    return False
            bindings = self.package_bindings(mod)
            if bindings is None or not bindings.issuperset(names):
                return False
            filename = Path(mod.__file__).relative_to(self.root)
            targets = {(kind, to) for at, name, kind, to in self.edges if at == line and name == raw}
            if targets != {("file", filename.as_posix())}:
                return False
            if self.has_possible_child(filename.parent, names):
                return False
        except (OSError, SyntaxError, TypeError, ValueError, RuntimeError):
            return False
        # The declaration qualifies the site. In the tracked package view, an
        # absent live binding cannot add a local direct edge via native fallback.
        self.declared_site = site
        return True

    def has_possible_child(self, directory, names):
        if directory not in self.package_children:
            # The dispatcher verifies a clean tracked checkout. Also inspect
            # the directory itself to include non-Python loaders, namespace
            # directories and symlinks, without opening any of their sources.
            prefix = directory.as_posix().casefold() + "/"
            children = {path.name.casefold() for path in (self.root / directory).iterdir()}
            children.update(path.casefold()[len(prefix):].split("/", 1)[0]
                            for path in self.tracked if path.casefold().startswith(prefix))
            self.package_children[directory] = children
        children = self.package_children[directory]
        members = {name.casefold() for name in names}
        return any(
            child == member or (child.startswith(member + ".") and child.endswith(CHILD_SUFFIXES))
            for member in members for child in children
        )

    def native_source_chain(self, parts, *, package, namespace_root=None):
        """Resolve identifiers in native order and validate each origin before reuse."""
        search = self.path
        sources = []
        PathFinder.invalidate_caches()
        for index, part in enumerate(parts):
            fullname = ".".join(parts[:index + 1])
            if (index == 0 and part in sys.builtin_module_names) or FrozenImporter.find_spec(fullname):
                return None
            # Modulefinder passes the one component with an explicit search
            # path, so namespace lookup does not consult a live parent module.
            spec = PathFinder.find_spec(part, search)
            if spec is None:
                return None
            locations = spec.submodule_search_locations
            is_package = index < len(parts) - 1 or package
            if spec.loader is None and namespace_root is not None and index < len(parts) - 1:
                directory = namespace_root.joinpath(*parts[:index + 1])
                if list(locations or ()) != [str(directory)] or directory.resolve(strict=True) != directory:
                    return None
                sources.append(None)
            elif type(spec.loader) is SourceFileLoader and (locations is not None) == is_package:
                source = Path(spec.origin)
                if not self.tracked_source(source):
                    return None
                if is_package and (source.name != "__init__.py" or list(locations) != [str(source.parent)]):
                    return None
                if namespace_root is not None:
                    expected = namespace_root.joinpath(*parts[:index + 1])
                    expected = expected / "__init__.py" if is_package else expected.with_suffix(".py")
                    if source != expected:
                        return None
                sources.append(source)
            else:
                return None
            search = locations
        return sources

    def package_bindings(self, mod):
        filename = Path(mod.__file__)
        if mod.__path__ != [str(filename.parent)]:
            return None
        key = (mod.__name__, filename)
        if key in self.declarations:
            return self.declarations[key]

        sources = self.native_source_chain(mod.__name__.split("."), package=True)
        if sources is None or sources[-1] != filename:
            return None
        for source in sources:
            bindings = declarative_bindings(source, source.relative_to(self.root).as_posix())
            if bindings is None:
                return None
        self.declarations[key] = bindings
        return bindings

    def ensure_fromlist(self, mod, fromlist, recursive=0):
        # Native Module attributes describe modules already discovered by the
        # finder, not assignments in a package. Do not manufacture attributes.
        if self.declared_site != (self.active, mod):
            super().ensure_fromlist(mod, fromlist, recursive)

    def _add_badmodule(self, name, caller):
        super()._add_badmodule(name, caller)
        if self.active is not None:
            self.gaps.add("unresolved-direct")


def read_entry(root, roots, name, *, tracked=None):
    row = {"path": name, "edges": [], "gap": "", "optimized_away": 0}
    filename = root / name
    if tracked is not None:
        try:
            if (name not in tracked or not confined_file(root, filename)
                    or any(not r.is_relative_to(root) or r.resolve(strict=True) != r or not r.is_dir() for r in roots)):
                raise UnconfinedSource("entry or search root is outside the tracked source view")
        except (OSError, RuntimeError, UnconfinedSource):
            row["gap"] = "native-target"
            return row
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

    finder = EntryFinder(root, roots, tracked)
    try:
        finder.set_entry(roots, filename)
        finder.scan_code(code, finder.entry)
    except UnconfinedSource:
        row["gap"] = "native-target"
    except NamespaceTarget:
        row["gap"] = "namespace"
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


def read_rows(root, subject, names, *, declarative_fromlist=False):
    declared_roots = [root / member for member in subject["roots"]]
    roots = declared_roots if declarative_fromlist else [member.resolve(strict=True) for member in declared_roots]
    tracked = frozenset(names) if declarative_fromlist else None
    return [read_entry(root, roots, name, tracked=tracked) for name in names]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("corpus", type=Path)
    parser.add_argument("--declarative-fromlist", action="store_true")
    args = parser.parse_args()
    pins = pin(declarative_fromlist=args.declarative_fromlist)
    subjects = {}
    for subject in pins["subjects"]:
        name = subject["name"]
        path = args.corpus / name
        root = checkout_root(path, confined=args.declarative_fromlist)
        names, before = contents(root, subject, confined=args.declarative_fromlist)
        rows = read_rows(root, subject, names, declarative_fromlist=args.declarative_fromlist)
        if (names, before) != contents(root, subject, confined=args.declarative_fromlist):
            raise ValueError(f"{name}: inputs changed during oracle collection")
        subjects[name] = {"manifest": before, "rows": rows}
    json.dump(subjects, sys.stdout, sort_keys=True, separators=(",", ":"))
    print()


if __name__ == "__main__":
    main()
