"""Tests for pyproject_oracle (python -m unittest, run with the oracle's venv:
<venv>/bin/python -m unittest discover -s calibration/oracles/python-packaging).

The backend tests need flit_core in that venv and are skipped without it."""

import importlib.util
import os
import sys
import tempfile
import textwrap
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import pyproject_oracle as po  # noqa: E402

HAS_FLIT = importlib.util.find_spec("flit_core") is not None
HAS_GROUPS = importlib.util.find_spec("dependency_groups") is not None


class LineScannerTest(unittest.TestCase):
    def test_headers_keys_dotted_inline_and_multiline(self):
        src = textwrap.dedent(
            """\
            # comment [project]
            [project]
            name = "demo"
            dependencies = [
              "a = 1",
              "b",
            ]
            description = '''
            not = a key
            '''
            scripts.inline-dotted = "m:f"

            [project.scripts]
            demo = "demo:main"
            "quoted-name" = "demo:q"

            [project.entry-points."pytest11"]
            plug = "demo.plug"

            [project.gui-scripts]
            g = { a = 1 }

            [dependency-groups]
            test = ["pytest", {include-group = "lint"}]
            lint = [
              "ruff",
            ]
            """
        )
        lines = po.scan_lines(src)
        self.assertEqual(lines[("project",)], 2)
        self.assertEqual(lines[("project", "scripts", "demo")], 14)
        self.assertEqual(lines[("project", "scripts", "quoted-name")], 15)
        self.assertEqual(lines[("project", "scripts", "inline-dotted")], 11)
        self.assertEqual(lines[("project", "entry-points", "pytest11", "plug")], 18)
        self.assertEqual(lines[("project", "gui-scripts", "g")], 21)
        self.assertEqual(lines[("dependency-groups", "test")], 24)
        self.assertEqual(lines[("dependency-groups", "lint")], 25)
        self.assertNotIn(("project", "not"), lines)
        self.assertNotIn(("project", "a = 1"), lines)

    def test_an_implicit_table_is_at_its_first_subtable(self):
        lines = po.scan_lines("[tool.ruff.lint]\nselect = []\n\n[[tool.mypy.overrides]]\nmodule = 'x'\n")
        self.assertEqual(lines[("tool", "ruff")], 1)
        self.assertEqual(lines[("tool", "mypy")], 4)


def write(root, rel, body):
    p = os.path.join(root, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w") as f:
        f.write(textwrap.dedent(body))


@unittest.skipUnless(HAS_GROUPS, "dependency_groups is not installed")
class GroupsTest(unittest.TestCase):
    def test_groups_and_includes_come_from_the_reference_implementation(self):
        with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as root:
            write(root, "pyproject.toml", """\
                [dependency-groups]
                test = ["pytest", {include-group = "lint"}]
                lint = ["ruff"]
                bad = [{include-group = "nope"}]
                """)
            out, refs, stats, accepted, rejected = po.run(root, os.path.join(root, ".work"))
            defs = {(d["name"], d["kind"], d["line"]) for d in out["definitions"]}
            self.assertIn(("test", "target", 2), defs)
            self.assertIn(("lint", "target", 3), defs)
            self.assertNotIn("bad", {d["name"] for d in out["definitions"]})
            # the references are their own file, tier A; the definitions file carries none
            self.assertEqual(refs["tier"], "A")
            self.assertEqual(refs["references"], [{"path": "pyproject.toml", "from": "test", "to": "lint"}])
            self.assertEqual(out["references"], [])
            self.assertEqual(stats["groups_rejected"], 1)


@unittest.skipUnless(HAS_FLIT, "flit_core is not installed")
class BackendTest(unittest.TestCase):
    def test_name_and_scripts_come_from_the_backend_metadata(self):
        with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as root:
            write(root, "pyproject.toml", """\
                [build-system]
                requires = ["flit_core"]
                build-backend = "flit_core.buildapi"

                [project]
                name = "demo"
                version = "1.0"
                description = "d"

                [project.scripts]
                demo = "demo:main"

                [tool.ruff]
                line-length = 88
                """)
            write(root, "demo/__init__.py", '"""d"""\n')
            out, refs, stats, accepted, rejected = po.run(root, os.path.join(root, ".work"))
            defs = {(d["name"], d["kind"], d["line"]) for d in out["definitions"]}
            # [tool.*] tables have no evaluator: tomllib names them, so the file is tier C
            self.assertEqual(defs, {("demo", "package", 5), ("demo", "route", 11), ("ruff", "section", 13)})
            self.assertEqual(out["tier"], "C")
            self.assertEqual(accepted, ["pyproject.toml"])

    def test_a_backend_that_fails_rejects_the_file(self):
        with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as root:
            write(root, "pyproject.toml", """\
                [build-system]
                requires = ["nothing"]
                build-backend = "no_such_backend"

                [project]
                name = "demo"
                version = "1.0"
                """)
            out, refs, stats, accepted, rejected = po.run(root, os.path.join(root, ".work"))
            self.assertEqual(out["definitions"], [])
            self.assertEqual(rejected, ["pyproject.toml"])
            self.assertEqual(accepted, [])


if __name__ == "__main__":
    unittest.main()
