"""Focused offline fixtures for the pinned native adapter."""
import pathlib
import tempfile
import unittest
from runpy import run_path

oracle = run_path(str(pathlib.Path(__file__).with_name("modulefinder_oracle.py")))
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

    @staticmethod
    def write(root, name, text):
        dest = root / name
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(text, encoding="utf-8")


if __name__ == "__main__":
    unittest.main()
