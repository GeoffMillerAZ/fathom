"""Offline tests for the independent COV-D9 graph and comparison."""
import hashlib
import os
from pathlib import Path
from runpy import run_path
import tempfile
import unittest

driver = run_path(str(Path(__file__).with_name("cov_python_pyright.py")))


def native(*edges):
    return {"gap": "", "edges": [
        {"Raw": operand, "Line": line, "To": target, "Resolution": "file"}
        for operand, line, target in edges
    ]}


class GraphTests(unittest.TestCase):
    def test_graph_counts_and_direction(self):
        root = Path("/private/development/source view")
        entry, child = "entry.py", "pkg/child ø.py"
        graph = (
            "diagnostic\npyright 1.1.408\nCompleted in 0.01sec\n\n"
            "source view/entry.py\n Imports     2 files\n"
            f"    {(root / child).as_uri()}\n"
            "    file:///stdlib/builtins.pyi\n"
            " Imported by 1 file\n"
            f"    {(root / 'other.py').as_uri()}\n"
            "\n1 file not explicitly imported\n"
        )
        self.assertEqual(driver["parse_graph"](root, {entry, child}, graph), {entry: {child}})
        with self.assertRaises(ValueError):
            driver["parse_graph"](root, {entry, child}, graph.replace("2 files", "3 files"))
        with self.assertRaises(ValueError):
            driver["parse_graph"](root, {entry}, graph)

    def test_superset_requires_a_unique_literal_site(self):
        row = native(("pkg", 1, "pkg/__init__.py"))
        observed = {"pkg/__init__.py", "pkg/child.py"}

        def compare(source, graph=observed):
            return driver["compare_row"](row, graph, source, "entry.py", ["."], observed)

        one = "from pkg import child\n"
        self.assertEqual(compare(one), ("initializer-plus-submodule", observed))
        self.assertEqual(compare(one, {"pkg/__init__.py"}), ("exact", {"pkg/__init__.py"}))
        self.assertEqual(compare(one + "import pkg.child\n"), ("ambiguous-site", None))
        self.assertEqual(compare(one + "import pkg\n", {"pkg/__init__.py"}), ("ambiguous-site", None))
        self.assertEqual(compare("import pkg as a, pkg as b\n", {"pkg/__init__.py"}),
                         ("ambiguous-site", None))
        self.assertEqual(compare(one, {"pkg/child.py"}), ("incomparable", None))
        self.assertEqual(compare(one, set()), ("pyright-subset", None))
        self.assertEqual(compare(one, None), ("missing-graph", None))

    def test_initializer_subset_stays_a_gap(self):
        row = native(("a.b.c", 1, "a/__init__.py"), ("a.b.c", 1, "a/b/__init__.py"),
                     ("a.b.c", 1, "a/b/c.py"))
        local = {edge["To"] for edge in row["edges"]}
        self.assertEqual(driver["compare_row"](
            row, local - {"a/b/__init__.py"}, "from a.b.c import token\n", "entry.py", ["."], local,
        ), ("pyright-omits-absolute-from-initializer", None))
        self.assertEqual(driver["compare_row"](
            row, local - {"a/b/__init__.py"}, "from a.b.c import token\nimport a.b\n",
            "entry.py", ["."], local,
        ), ("pyright-subset", None))

    def test_untracked_source_is_refused_before_starting_pyright(self):
        with tempfile.TemporaryDirectory() as temp:
            work = Path(temp).resolve()
            view = work / "00-view"
            view.mkdir()
            (view / "entry.py").write_text("import untracked\n", encoding="utf-8")
            (view / "untracked.py").write_text("pass\n", encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "differs from the tracked inventory"):
                driver["run_graph"](None, view, ["."], {"entry.py"}, {"entry.py"}, work)

    def test_native_search_order_applies_to_entries_outside_the_first_root(self):
        cache = os.environ.get("COV_PYRIGHT_CACHE")
        if not cache:
            self.skipTest("COV_PYRIGHT_CACHE names the offline pin")
        tool = driver["verify_tool"](Path(cache))
        with tempfile.TemporaryDirectory() as temp:
            work = Path(temp).resolve()
            view = work / "00-view"
            (view / "src").mkdir(parents=True)
            files = {"entry.py": "import shared\n", "src/shared.py": "first = 1\n", "shared.py": "second = 2\n"}
            for name, source in files.items():
                (view / name).write_text(source, encoding="utf-8")
            graph = driver["run_graph"](tool, view, ["src", "."], set(files), {"entry.py"}, work)
            self.assertEqual(graph["entry.py"], {"src/shared.py"})

    def test_pinned_graph_reacts_to_adding_and_removing_child(self):
        cache = os.environ.get("COV_PYRIGHT_CACHE")
        if not cache:
            self.skipTest("COV_PYRIGHT_CACHE names the offline pin")
        tool = driver["verify_tool"](Path(cache))
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            entry, initializer, child = "entry.py", "pkg/__init__.py", "pkg/child.py"
            files = {entry: "from pkg import child\n", initializer: "pass\n"}

            def run(label):
                work = root / label
                view = work / "00-view"
                (view / "pkg").mkdir(parents=True)
                for name, source in files.items():
                    (view / name).write_text(source, encoding="utf-8")
                return driver["run_graph"](tool, view, ["."], set(files), {entry}, work)[entry]

            before = run("before")
            files[child] = "raise AssertionError('source executed')\n"
            after = run("after")
            del files[child]
            self.assertEqual(run("restored"), before)
            self.assertEqual(before, {"pkg/__init__.py"})
            self.assertEqual(after, before | {child})


if __name__ == "__main__":
    unittest.main()
