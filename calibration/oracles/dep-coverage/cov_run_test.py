"""Offline controls for the COV thin dispatcher and frozen cohort identity."""
import importlib.util
import json
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


if __name__ == '__main__':
    unittest.main()
