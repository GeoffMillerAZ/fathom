"""Focused offline fixtures for the pinned native adapter."""
import modulefinder
import pathlib
import py_compile
import subprocess
import sys
import tempfile
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
            self.write(root, "app/value.py", "pass\n")
            self.write(root, "app/main.py", "from . import value as alias, other\nimport sys\n")
            self.write(root, "sys.py", "pass\n")
            tracked = {"app/__init__.py", "app/leaf.py", "app/value.py", "app/main.py", "sys.py"}

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
            self.assertEqual(read()["gap"], "package-fromlist")  # value.py alone supplies no proof
            self.write(root, "app/__init__.py", "value = 1\nother = 3\n")
            self.assertEqual(read()["edges"], expected)

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
            self.assertEqual(read_entry(root, [root], "main.py", tracked={"main.py"})["gap"], "package-fromlist")
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
