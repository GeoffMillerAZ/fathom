"""Offline controls for the COV thin dispatcher and frozen cohort identity."""
import importlib.util
from pathlib import Path
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('cov_runner', HERE / 'cov_run.py')
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class CohortControls(unittest.TestCase):
    def test_original_populations_unchanged(self):
        pools = runner.subjects()
        totals = {}
        for p in pools['development']:
            totals[p['dialect']] = totals.get(p['dialect'], 0) + p['files']
        self.assertEqual(totals, {'python': 1314, 'typescript': 1224, 'javascript': 1508})
        self.assertFalse({p['commit'] for p in pools['development']} & {p['commit'] for p in pools['holdout']})

    def test_holdout_copies_landed_registration(self):
        design = (HERE.parents[2] / 'docs/design/dependency-edges.md').read_text()
        for p in runner.subjects()['holdout']:
            self.assertIn(p['commit'], design)
            self.assertIn(p['selectionSHA256'], design)
            self.assertIn(f"{p['files']:,}", design)


class OverlapControls(unittest.TestCase):
    def test_compares_native_rows_and_gap_transitions(self):
        def row(path, gap="", target="pkg/__init__.py"):
            return {"path": path, "gap": gap, "edges": [{"To": target}], "optimized_away": 0}

        frozen = [row("same.py"), row("different.py"), row("lost.py"),
                  row("new.py", "package-fromlist"), row("unknown.py", "package-fromlist")]
        extension = [row("same.py"), row("different.py", target="pkg/child.py"),
                     row("lost.py", "runtime"), row("new.py"), row("unknown.py", "unresolved-direct")]
        report = runner.compare_python(frozen, extension)
        self.assertEqual(report["counts"], {
            "agree": 1, "disagree": 1, "lost": 1, "gained": 1, "neither": 1,
        })
        self.assertEqual(report["disagreements"], ["different.py"])
        self.assertIn(["package-fromlist", "unresolved-direct", 1], report["gap_transitions"])
        self.assertNotEqual(report["frozen_sha256"], report["extension_sha256"])
        with self.assertRaisesRegex(ValueError, "file populations"):
            runner.compare_python(frozen, extension[:-1])


if __name__ == '__main__':
    unittest.main()
