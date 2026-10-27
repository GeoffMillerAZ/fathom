#!/usr/bin/env python3
"""Historical validate replay refusal and order-stable comparison tests."""

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

MODULE = Path(__file__).with_name("replay_validation.py")
spec = importlib.util.spec_from_file_location("replay_validation", MODULE)
replay = importlib.util.module_from_spec(spec)
spec.loader.exec_module(replay)
WORKSPACE_TMP = Path(__file__).resolve().parents[3] / "workspace" / "tmp"


class ReplayTests(unittest.TestCase):
    def setUp(self):
        WORKSPACE_TMP.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=WORKSPACE_TMP)
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.capture = root / "capture"
        self.compare = root / "compare"
        self.receipt_path = root / "receipt.json"
        self.out = root / "replay.json"
        providers = {}
        for name in replay.probe.NAMES:
            source = b"fixture\n"
            schema = b'{"schema":1}\n'
            show = b'{"show":1}\n'
            provider = {"status": "complete", "source_sha256": replay.sha256(source),
                        "schema": {"raw_sha256": replay.sha256(schema)},
                        "show": {"raw_sha256": replay.sha256(show)}, "mutations": []}
            for capture in (self.capture, self.compare):
                (capture / name).mkdir(parents=True)
                (capture / name / "main.tf").write_bytes(source)
                (capture / (name + ".schema.stdout")).write_bytes(schema)
                (capture / (name + ".show.stdout")).write_bytes(show)
            for label, rc in [("baseline", 0)] + [("mutation" + str(i), 1) for i in range(4)]:
                diagnostics = [] if rc == 0 else [
                    {"severity": "warning", "summary": "Notice"},
                    {"severity": "error", "summary": "Unsupported attribute",
                     "detail": "No attribute " + replay.probe.SENTINEL,
                     "range": {"filename": "main.tf", "start": {"line": 2}}}]
                data = {"valid": rc == 0, "error_count": int(rc != 0),
                        "warning_count": int(rc != 0), "diagnostics": diagnostics}
                raw = json.dumps(data, separators=(",", ":")).encode()
                for capture in (self.capture, self.compare):
                    (capture / (name + "." + label + ".stdout")).write_bytes(raw)
                historical = replay.probe.summarize_validation(data, rc, replay.sha256(raw))
                historical.pop("canonical_sha256")  # Immutable pre-F3 receipt shape.
                if label == "baseline":
                    provider["baseline_validation"] = historical
                else:
                    provider["mutations"].append({"output_index": int(label[-1]), "validation": historical})
            providers[name] = provider
        self.receipt = {"status": "complete", "providers": providers}
        self.write_receipt()

    def write_receipt(self):
        self.receipt_path.write_text(json.dumps(self.receipt))

    def invoke(self, compare=False):
        argv = ["--capture", str(self.capture), "--receipt", str(self.receipt_path),
                "--out", str(self.out)]
        if compare:
            argv += ["--compare-capture", str(self.compare)]
        return replay.main(argv)

    def test_reordered_diagnostics_compare_equal_canonical_but_distinct_raw(self):
        path = self.compare / "aws.mutation1.stdout"
        data = json.loads(path.read_bytes())
        data["diagnostics"].reverse()
        path.write_bytes(json.dumps(data, separators=(",", ":")).encode())
        self.assertEqual(self.invoke(compare=True), 0)
        result = json.loads(self.out.read_bytes())
        self.assertEqual(result["validate_stream_count"], 15)
        self.assertEqual(result["comparison"]["canonical_equal_count"], 15)
        self.assertEqual(result["comparison"]["raw_equal_count"], 14)

    def test_primary_hash_mismatch_refuses_whole_output(self):
        (self.capture / "google.baseline.stdout").write_bytes(b"{}\n")
        with self.assertRaisesRegex(ValueError, "validation_capture_mismatch"):
            self.invoke()
        self.assertFalse(self.out.exists())

    def test_comparison_source_mismatch_refuses_whole_output(self):
        (self.compare / "azurerm" / "main.tf").write_bytes(b"drift\n")
        with self.assertRaisesRegex(ValueError, "source_capture_mismatch"):
            self.invoke(compare=True)
        self.assertFalse(self.out.exists())

    def test_malformed_receipt_or_changed_aggregate_refuses(self):
        self.receipt["providers"]["aws"]["mutations"].pop()
        self.write_receipt()
        with self.assertRaisesRegex(ValueError, "final_receipt_incomplete"):
            self.invoke()
        self.assertFalse(self.out.exists())
        self.receipt["providers"]["aws"]["mutations"].append(
            {"output_index": 3, "validation": self.receipt["providers"]["azurerm"]["mutations"][3]["validation"]})
        self.receipt["providers"]["aws"]["baseline_validation"]["warning_count"] = 9
        self.write_receipt()
        with self.assertRaisesRegex(ValueError, "validation_aggregate_mismatch"):
            self.invoke()
        self.assertFalse(self.out.exists())

    def test_existing_output_remains_untouched(self):
        self.out.write_bytes(b"stale\n")
        with self.assertRaises(FileExistsError):
            self.invoke()
        self.assertEqual(self.out.read_bytes(), b"stale\n")


if __name__ == "__main__":
    unittest.main()
