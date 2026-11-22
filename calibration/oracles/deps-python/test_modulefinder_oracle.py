"""Focused offline fixtures for the pinned native adapter."""
import importlib._bootstrap as bootstrap
from importlib.machinery import PathFinder
import io
import modulefinder
import pathlib
import py_compile
import subprocess
import sys
import tempfile
from types import ModuleType
import unittest
from unittest.mock import patch
from runpy import run_path

SCRIPT = pathlib.Path(__file__).with_name("modulefinder_oracle.py")
oracle = run_path(str(SCRIPT))
read_entry = oracle["read_entry"]


class OracleTests(unittest.TestCase):
    def test_direct_bytecode_and_filename(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            self.write(root, "app/__init__.py", "import hidden\n")
            self.write(root, "app/main.py", "import app.helper\nfrom .leaf import value\nif False: import hidden\n")
            self.write(root, "app/helper.py", "import hidden\n")
            self.write(root, "app/leaf.py", "value = 3\n")
            self.write(root, "hidden.py", "unused = True\n")
            before = read_entry(root, [root], "app/main.py")
            self.assertEqual(before["gap"], "")
            self.assertEqual(before["optimized_away"], 1)
            self.assertEqual(
                [(e["Line"], e["Raw"], e["To"]) for e in before["edges"]],
                [(1, "app.helper", "app/__init__.py"), (1, "app.helper", "app/helper.py"), (2, ".leaf", "app/leaf.py")],
            )

            self.write(root, "app/helper.py", "import other\n")
            self.assertEqual(read_entry(root, [root], "app/main.py"), before)
            self.write(root, "app/main.py", "from .leaf import value\n")
            self.assertNotEqual(read_entry(root, [root], "app/main.py"), before)

    def test_coverage_gaps(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            self.write(root, "ns/child.py", "")
            self.write(root, "app/__init__.py", "")
            self.write(root, "app/child.py", "")
            cases = [
                ("import ns.child\n", "namespace"),
                ("from app import child\n", "package-fromlist"),
                ("from absent import child\n", "unresolved-direct"),
                ("__import__(computed_name)\n", "runtime"),
                ("from app import (\n", "entry-compile"),
            ]
            for src, gap in cases:
                with self.subTest(gap=gap):
                    self.write(root, "main.py", src)
                    self.assertEqual(read_entry(root, [root], "main.py")["gap"], gap)

    def test_builtin_binding_can_refute_a_local_edge(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            self.write(root, "main.py", "import sys\n")
            self.write(root, "sys.py", "pass\n")
            self.assertEqual(read_entry(root, [root], "main.py")["edges"], [
                {"Kind": "import", "Raw": "sys", "Line": 1, "Resolution": "external", "To": "python:sys"},
            ])

    def test_declarative_fromlist_uses_only_direct_native_bindings(self):
        oracle["pin"]()
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp).resolve()
            self.write(root, "app/__init__.py", "from .leaf import value as value\nother = 3\n")
            self.write(root, "app/leaf.py", "raise AssertionError('source must not execute')\n")
            self.write(root, "app/main.py", "from . import value as alias, other\nimport sys\n")
            self.write(root, "sys.py", "pass\n")
            tracked = {"app/__init__.py", "app/leaf.py", "app/main.py", "sys.py"}

            def read():
                return read_entry(root, [root], "app/main.py", tracked=tracked)

            self.assertEqual(read_entry(root, [root], "app/main.py")["gap"], "package-fromlist")
            expected = [
                {"Kind": "import", "Raw": ".", "Line": 1, "Resolution": "file", "To": "app/__init__.py"},
                {"Kind": "import", "Raw": "sys", "Line": 2, "Resolution": "external", "To": "python:sys"},
            ]
            self.assertEqual(read(), {"path": "app/main.py", "edges": expected, "gap": "", "optimized_away": 0})

            native = modulefinder.ModuleFinder.scan_opcodes

            def without_native_store(finder, code):
                for opcode, args in native(finder, code):
                    if not (code.co_filename.endswith("/__init__.py") and opcode == "store"):
                        yield opcode, args

            with patch.object(modulefinder.ModuleFinder, "scan_opcodes", without_native_store):
                self.assertEqual(read()["gap"], "package-fromlist")
            self.write(root, "app/__init__.py", "other = 3\n")
            self.assertEqual(read()["gap"], "package-fromlist")
            self.write(root, "app/__init__.py", "value = 1\nother = 3\n")
            self.assertEqual(read(), {"path": "app/main.py", "edges": expected, "gap": "", "optimized_away": 0})

    def test_declarative_fromlist_refuses_child_after_deletion_or_cycle(self):
        package = ModuleType("pkg")
        requested = []
        bootstrap._handle_fromlist(package, ("token",), requested.append)
        self.assertEqual(requested, ["pkg.token"])
        package.token = 1
        requested.clear()
        bootstrap._handle_fromlist(package, ("token",), requested.append)
        self.assertEqual(requested, [])

        fixtures = {
            "deletion": ("entry.py", {
                "pkg/__init__.py": "token = 1\nfrom . import side_effect\n",
                "pkg/side_effect.py": "import pkg\ndel pkg.token\n",
                "entry.py": "from pkg import token\n",
            }),
            "cycle": ("pkg/consumer.py", {
                "pkg/__init__.py": "from . import consumer\ntoken = 1\n",
                "pkg/consumer.py": "from . import token\n",
            }),
        }
        with tempfile.TemporaryDirectory() as temp:
            for case, (entry, sources) in fixtures.items():
                with self.subTest(case=case):
                    root = pathlib.Path(temp).resolve() / case
                    for name, source in sources.items():
                        self.write(root, name, source)
                    without_child = read_entry(root, [root], entry, tracked=set(sources))
                    self.assertEqual(without_child["gap"], "")
                    self.assertEqual([e["To"] for e in without_child["edges"]], ["pkg/__init__.py"])
                    child = "pkg/token.py"
                    self.write(root, child, "pass\n")
                    row = read_entry(root, [root], entry, tracked=set(sources) | {child})
                    self.assertEqual(row["gap"], "package-fromlist")
        self.assertTrue({"pkg", "pkg.side_effect", "pkg.consumer", "pkg.token"}.isdisjoint(sys.modules))

    def test_declarative_fromlist_refuses_ambiguous_child(self):
        for child in ("pkg/Token.py", "pkg/token/__init__.py", "pkg/Token/leaf.py",
                      "pkg/token.so", "pkg/token.pyc", "pkg/token.py", "pkg/token"):
            with self.subTest(child=child), tempfile.TemporaryDirectory() as temp:
                root = pathlib.Path(temp).resolve()
                self.write(root, "pkg/__init__.py", "token = 1\n")
                self.write(root, "entry.py", "from pkg import token\n")
                tracked = {"pkg/__init__.py", "entry.py"}
                if child == "pkg/token":
                    (root / child).symlink_to(root / "absent")
                else:
                    self.write(root, child, "raise AssertionError('source must not execute')\n")
                    if child != "pkg/token.py":  # exercise a physical candidate outside the tracked view
                        tracked.add(child)
                self.assertTrue(read_entry(root, [root], "entry.py", tracked=tracked)["gap"])

    def test_declarative_fromlist_requires_one_initializer_edge(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp).resolve()
            sources = {
                "pkg/__init__.py": "pass\n",
                "pkg/nested/__init__.py": "token = 1\n",
                "entry.py": "from pkg.nested import token\n",
            }
            for name, source in sources.items():
                self.write(root, name, source)
            row = read_entry(root, [root], "entry.py", tracked=set(sources))
            self.assertEqual(row["gap"], "package-fromlist")
            self.assertEqual([edge["To"] for edge in row["edges"]], ["pkg/__init__.py", "pkg/nested/__init__.py"])

    def test_declarative_fromlist_confines_source_before_native_open(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            outside = base / "outside"
            self.write(outside, "__init__.py", "token = 1\n")
            root = base / "checkout"
            self.write(root, "entry.py", "from pkg import token\n")
            (root / "pkg").symlink_to(outside, target_is_directory=True)
            (root / "linked_entry.py").symlink_to(outside / "__init__.py")
            self.write(root, "untracked.py", "raise AssertionError('source must not execute')\n")
            self.write(root, "untracked_user.py", "import untracked\n")
            real_open, real_read, real_compile = io.open_code, pathlib.Path.read_bytes, compile
            opened, read, compiled = set(), set(), set()

            def record_open(path):
                opened.add(pathlib.Path(path).resolve())
                return real_open(path)

            def record_read(path):
                read.add(path.resolve())
                return real_read(path)

            def record_compile(data, path, *args, **kwargs):
                if pathlib.Path(path).is_absolute():
                    compiled.add(pathlib.Path(path).resolve())
                return real_compile(data, path, *args, **kwargs)

            for entry, roots, tracked, forbidden in [
                ("entry.py", [root], None, outside / "__init__.py"),
                ("entry.py", [root], {"entry.py", "pkg/__init__.py"}, outside / "__init__.py"),
                ("untracked_user.py", [root], {"untracked_user.py"}, root / "untracked.py"),
                ("linked_entry.py", [root], {"linked_entry.py"}, outside / "__init__.py"),
                ("entry.py", [root / "pkg"], {"entry.py"}, outside / "__init__.py"),
            ]:
                with self.subTest(entry=entry, tracked=tracked):
                    opened.clear()
                    read.clear()
                    compiled.clear()
                    with (
                        patch("io.open_code", record_open),
                        patch.object(pathlib.Path, "read_bytes", record_read),
                        patch("builtins.compile", record_compile),
                    ):
                        row = read_entry(root, roots, entry, tracked=tracked)
                    if tracked is None:
                        self.assertEqual(row["gap"], "native-target")
                        self.assertIn(forbidden, opened)
                        self.assertIn(forbidden, compiled)
                    else:
                        self.assertNotIn(forbidden, opened)
                        self.assertNotIn(forbidden, read)
                        self.assertNotIn(forbidden, compiled)
                        self.assertTrue(row["gap"])

    def test_declarative_fromlist_refuses_uncertainty(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp).resolve()
            tracked = {"app/__init__.py", "app/main.py", "main.py"}
            for init, entry in [
                ("if True: value = 1", "from app import value"),
                ("value = make()", "from app import value"),
                ("value = set()", "from app import value"),
                ("value = 1\nvalue = 2", "from app import value"),
                ("value = 1\n__path__ = []", "from app import value"),
                ("value = 1\n__getattr__ = hook", "from app import value"),
                ("value = 1\nfrom elsewhere import *", "from app import value"),
                ("value = 1", "from app import *"),
                ("value = 1", "from app import value, missing"),
                ("value = 1", "from app import value\nimport absent"),
                ("return\nvalue = 1", "from app import value"),
            ]:
                with self.subTest(init=init, entry=entry):
                    self.write(root, "app/__init__.py", init)
                    self.write(root, "main.py", entry)
                    before = read_entry(root, [root], "main.py")
                    after = read_entry(root, [root], "main.py", tracked=tracked)
                    self.assertTrue(before["gap"])
                    self.assertTrue(after["gap"])
            self.write(root, "app/__init__.py", "value = 1")
            self.write(root, "main.py", "from app import value")
            self.assertEqual(read_entry(root, [root], "main.py", tracked={"main.py"})["gap"], "native-target")
            self.write(root, "app/__init__.py", "from . import value\nvalue = 1")
            self.assertTrue(read_entry(root, [root], "app/__init__.py", tracked=tracked)["gap"])

    def test_declarative_fromlist_preserves_root_boundaries(self):
        with tempfile.TemporaryDirectory() as temp:
            base = pathlib.Path(temp).resolve()
            root = base / "checkout"
            tracked = {"main.py", "real/__init__.py", "alias/__init__.py", "ns/child.py", "app/main.py",
                       "app/__init__.py", "src/app/__init__.py"}
            self.write(root, "real/__init__.py", "value = 1")
            (root / "alias").symlink_to(root / "real", target_is_directory=True)
            self.write(root, "main.py", "from alias import value")
            self.assertTrue(read_entry(root, [root], "main.py", tracked=tracked)["gap"])
            self.write(base, "outside/__init__.py", "value = 1")
            self.write(root, "main.py", "from outside import value")
            self.assertTrue(read_entry(root, [base, root], "main.py", tracked=tracked)["gap"])
            self.write(root, "ns/child.py", "value = 1")
            self.write(root, "main.py", "from ns import child")
            self.assertEqual(read_entry(root, [root], "main.py", tracked=tracked)["gap"], "namespace")
            self.write(root, "src/app/__init__.py", "value = 1")
            self.write(root, "app/__init__.py", "value = 1")
            self.write(root, "app/main.py", "from . import value")
            self.assertTrue(read_entry(root, [root / "src", root], "app/main.py", tracked=tracked)["gap"])

    def test_declarative_fromlist_requires_native_entry_package_order(self):
        oracle["pin"](declarative_fromlist=True)
        for prefix in ("", "second/"):
            with self.subTest(entry_prefix=prefix), tempfile.TemporaryDirectory() as temp:
                root = pathlib.Path(temp).resolve()
                sentinel = "raise AssertionError('fixture executed')\n"
                sources = {
                    "src/pkg/__init__.py": "",
                    "src/pkg/module.py": sentinel,
                    prefix + "pkg/__init__.py": "",
                    prefix + "pkg/module.py": sentinel,
                    prefix + "pkg/consumer.py": "import pkg.module\nfrom helper import token\n",
                    "helper/__init__.py": "token = 1\n",
                }
                if prefix:
                    sources["second/__init__.py"] = ""
                for name, source in sources.items():
                    self.write(root, name, source)
                git = ["git", "-C", str(root), "-c", "core.hooksPath=/dev/null"]
                subprocess.run([*git, "init", "-q", "--template="], check=True)
                subprocess.run([*git, "add", "--all"], check=True)
                subprocess.run([*git, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                                "commit", "-qm", "Track search order fixture"], check=True)
                revision = subprocess.check_output([*git, "rev-parse", "HEAD"], text=True).strip()
                roots = [root / "src", root / "second", root] if prefix else [root / "src", root]
                search = list(map(str, roots))
                entry = prefix + "pkg/consumer.py"
                if prefix:
                    outer = PathFinder.find_spec("second", search)
                    package = PathFinder.find_spec("second.pkg", outer.submodule_search_locations)
                    qualified = PathFinder.find_spec("second.pkg.consumer", package.submodule_search_locations)
                    self.assertEqual(qualified.origin, str(root / entry))
                parent = PathFinder.find_spec("pkg", search)
                child = PathFinder.find_spec("pkg.module", parent.submodule_search_locations)
                self.assertEqual(parent.origin, str(root / "src/pkg/__init__.py"))
                self.assertEqual(child.origin, str(root / "src/pkg/module.py"))

                subject = {"name": "synthetic", "commit": revision, "files": len(sources)}
                tracked, manifest = oracle["contents"](root, subject, confined=True)
                self.assertEqual(read_entry(root, roots, entry)["gap"], "package-fromlist")
                unshadowed = read_entry(root, roots[1:], entry, tracked=tracked)
                self.assertEqual(unshadowed["gap"], "", unshadowed)
                self.assertEqual([e["To"] for e in unshadowed["edges"]],
                                 [prefix + "pkg/__init__.py", prefix + "pkg/module.py", "helper/__init__.py"])
                row = read_entry(root, roots, entry, tracked=tracked)
                self.assertEqual((tracked, manifest), oracle["contents"](root, subject, confined=True))
                self.assertEqual(row["gap"], "native-target", row)
                self.assertEqual(row["edges"], [])
        self.assertTrue({"pkg", "pkg.module", "second", "second.pkg", "helper"}.isdisjoint(sys.modules))

    def test_native_entry_rejects_package_shadowing_selected_module(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp).resolve()
            entry = "second/pkg/consumer.py"
            shadow = "second/pkg/consumer/__init__.py"
            sources = {
                "second/__init__.py": "",
                "second/pkg/__init__.py": "",
                entry: "import pkg.consumer\nfrom helper import token\n",
                shadow: "raise AssertionError('fixture executed')\n",
                "helper/__init__.py": "token = 1\n",
            }
            for name, source in sources.items():
                self.write(root, name, source)
            roots = [root / "second", root]
            parent = PathFinder.find_spec("pkg", list(map(str, roots)))
            self.assertEqual(parent.origin, str(root / "second/pkg/__init__.py"))
            native = PathFinder.find_spec("pkg.consumer", parent.submodule_search_locations)
            self.assertEqual(native.origin, str(root / shadow))
            self.assertEqual(read_entry(root, roots, entry)["gap"], "package-fromlist")
            finder = oracle["EntryFinder"](root, roots, set(sources))
            with self.assertRaises(oracle["UnconfinedSource"]):
                finder.set_entry(roots, root / entry)
            self.assertEqual(finder.modules, {})
            with patch("io.open_code", side_effect=AssertionError("unexpected native open")):
                row = read_entry(root, roots, entry, tracked=set(sources))
            self.assertEqual((row["gap"], row["edges"]), ("native-target", []), row)
            (root / shadow).unlink()
            row = read_entry(root, roots, entry, tracked=set(sources) - {shadow})
            self.assertEqual(row["gap"], "", row)

    def test_native_entry_refuses_namespace_and_root_module_shadow(self):
        for entry, other in (
            ("second/ns/consumer.py", "ns/__init__.py"),
            ("second/ns/consumer.py", "ns/other.py"),
            ("second/consumer.py", "second/consumer/__init__.py"),
            ("second/consumer.extra.py", "second/consumer.py"),
        ):
            with self.subTest(entry=entry), tempfile.TemporaryDirectory() as temp:
                root = pathlib.Path(temp).resolve()
                self.write(root, entry, "from helper import token\n")
                self.write(root, other, "")
                self.write(root, "helper/__init__.py", "token = 1\n")
                row = read_entry(root, [root / "second", root], entry,
                                 tracked={entry, other, "helper/__init__.py"})
                self.assertEqual((row["gap"], row["edges"]), ("native-target", []), row)

    def test_native_entry_allows_unique_namespace_without_cache_reuse(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp).resolve()
            entry = "ns/nested/consumer.py"
            helper = "helper/__init__.py"
            self.write(root, entry, "from helper import token\n")
            self.write(root, helper, "token = 1\n")
            finder = oracle["EntryFinder"](root, [root], {entry, helper})
            finder.set_entry([root], root / entry)
            self.assertFalse(finder.modules)
            row = read_entry(root, [root], entry, tracked={entry, helper})
            self.assertEqual(row["gap"], "", row)
            self.write(root, entry, "from . import unproven\nfrom helper import token\n")
            self.assertTrue(read_entry(root, [root], entry, tracked={entry, helper})["gap"])

    def test_declarative_fromlist_refuses_sourceless_loader(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp).resolve()
            self.write(root, "app/__init__.py", "value = 1")
            self.write(root, "main.py", "from app import value")
            init = root / "app/__init__.py"
            py_compile.compile(str(init), cfile=str(init.with_suffix(".pyc")), doraise=True)
            init.unlink()
            self.assertTrue(read_entry(root, [root], "main.py", tracked={"main.py"})["gap"])

    def test_declarative_fromlist_requires_exact_unoptimized_pin(self):
        oracle["pin"](declarative_fromlist=True)
        for flags, error in [
            (["-I", "-B", "-S", "-O"], "requires stock, unoptimized modulefinder"),
            (["-B", "-S"], "invoke the pinned Python with -I -B -S"),
        ]:
            with self.subTest(flags=flags):
                result = subprocess.run(
                    [sys.executable, *flags, str(SCRIPT), "unused", "--declarative-fromlist"],
                    capture_output=True, text=True,
                )
                self.assertEqual(result.returncode, 1)
                self.assertIn(error, result.stderr)

    @staticmethod
    def write(root, name, text):
        dest = root / name
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(text, encoding="utf-8")


if __name__ == "__main__":
    unittest.main()
