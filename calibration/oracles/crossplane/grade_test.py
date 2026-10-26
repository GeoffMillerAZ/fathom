"""Contract checks for the fixed Crossplane grading driver."""

import unittest
from unittest import mock
from pathlib import Path
import sys
import tempfile

from calibration.oracles.crossplane import grade


class GradeContractTest(unittest.TestCase):
    def test_invalid_engine_commit_removes_stale_output(self):
        with tempfile.TemporaryDirectory(dir=grade.REPO / "workspace/tmp") as directory:
            scratch = Path(directory)
            out = scratch / "stale-reading.json"
            out.write_text('{"stale":true}\n')
            argv = ["grade.py", "--cache", str(scratch), "--manifest", str(scratch / "manifest.json"),
                    "--indexbench", str(scratch / "indexbench"), "--oracle-bin", str(scratch / "oracle"),
                    "--engine-commit", "invalid", "--out", str(out)]
            with mock.patch.object(sys, "argv", argv):
                with self.assertRaisesRegex(RuntimeError, "40-digit Git SHA"):
                    grade.main()
            self.assertFalse(out.exists())

    def test_original_pins_and_manifest(self):
        rows = [{"id": name, "sha": sha, "scope": list(scope), "root": "/missing"}
                for name, sha, scope in grade.CANONICAL]
        with self.assertRaisesRegex(FileNotFoundError, "missing"):
            grade.canonical_subjects({"subjects": rows})
        rows[0]["scope"] = ["accepted-native-only.yaml"]
        with self.assertRaisesRegex(RuntimeError, "exact canonical subject"):
            grade.canonical_subjects({"subjects": rows})

    def test_measured_scope_rejects_hidden_or_extra_file(self):
        name, sha, _ = grade.CANONICAL[0]
        row = {"name": name, "sha": sha, "files": 2, "tenancy": {"faults": 0},
               "families": {"l1": "C", "refs": "C", "xref": "C"}}
        reading = {"subjects": [row]}
        with self.assertRaisesRegex(RuntimeError, "outside canonical scope"):
            grade.verify_reading(reading, {"tenancy_files": ["a.yaml", "outside.yaml"]},
                                 [name], name, sha, {"a.yaml", "b.yaml"})
        with self.assertRaisesRegex(RuntimeError, "file count"):
            grade.verify_reading(reading, {"tenancy_files": ["a.yaml"]},
                                 [name], name, sha, {"a.yaml", "b.yaml"})

    def test_count_metric_and_tier_agreement(self):
        counts = {key: 1 for pair in grade.COUNT_PAIRS.values() for key in pair}
        reading = {"subjects": [{"counts": counts}], "counts": counts,
                   "metrics": {metric: 1 for metric in grade.COUNT_PAIRS},
                   "metric_tiers": {metric: "C" for metric in grade.COUNT_PAIRS},
                   "tenancy": {"faults": 0}, "level_met": "L3",
                   "grade": {"L1": "good", "L2": "good", "L3": "good"}}
        grade.check_counts(reading)
        reading["metric_tiers"]["sym_precision"] = "A"
        with self.assertRaisesRegex(RuntimeError, "non-C pooled metric"):
            grade.check_counts(reading)
        reading["metric_tiers"]["sym_precision"] = "C"
        reading["metrics"]["sym_precision"] = 0.5
        with self.assertRaisesRegex(RuntimeError, "metric/count disagreement"):
            grade.check_counts(reading)

    def test_unmet_grade_is_recordable(self):
        counts = {key: 1 for pair in grade.COUNT_PAIRS.values() for key in pair}
        reading = {"subjects": [{"counts": counts}], "counts": counts,
                   "metrics": {metric: 1 for metric in grade.COUNT_PAIRS},
                   "metric_tiers": {metric: "C" for metric in grade.COUNT_PAIRS},
                   "tenancy": {"faults": 0}, "level_met": "L1",
                   "grade": {"L1": "good", "L2": "thin", "L3": "thin"}}
        grade.check_counts(reading)

    @mock.patch.object(grade.time, "sleep")
    @mock.patch.object(grade.os, "getloadavg", side_effect=[(10.0, 0, 0), (8.5, 0, 0)])
    @mock.patch.object(grade.os, "getpriority", return_value=10)
    def test_each_launch_waits_for_load(self, priority, load, sleep):
        result = grade.launch_conditions()
        self.assertEqual(result["load_1m"], 8.5)
        self.assertEqual(result["nice"], 10)
        sleep.assert_called_once_with(20)
        self.assertEqual(priority.call_count, 2)

    @mock.patch.object(grade.os, "getpriority", return_value=0)
    def test_launch_rejects_high_priority(self, _priority):
        with self.assertRaisesRegex(RuntimeError, "nice -n 10"):
            grade.launch_conditions()


if __name__ == "__main__":
    unittest.main()
