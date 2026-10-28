#!/usr/bin/env python3
"""Receipt binding and output refusal for the offline export selector."""

import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

MODULE = Path(__file__).with_name("summarize_exports.py")
spec = importlib.util.spec_from_file_location("summarize_exports", MODULE)
summarize = importlib.util.module_from_spec(spec)
spec.loader.exec_module(summarize)
WORKSPACE_TMP = Path(__file__).resolve().parents[3] / "workspace" / "tmp"


class SummarizeExportsTests(unittest.TestCase):
    def setUp(self):
        WORKSPACE_TMP.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=WORKSPACE_TMP)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.capture = self.root / "capture"
        self.capture.mkdir()
        self.out = self.root / "selected.json"
        self.receipt_path = self.root / "native.json"
        providers = {}
        launches = []
        for name in summarize.probe.NAMES:
            provider = {"status": "complete"}
            for kind in ("schema", "show"):
                raw = b"{}\n"
                (self.capture / (name + "." + kind + ".stdout")).write_bytes(raw)
                digest = hashlib.sha256(raw).hexdigest()
                provider[kind] = {"raw_sha256": digest}
                launches.append({"command": name + "." + kind, "status": "exited",
                                 "exit_code": 0, "stdout_bytes": len(raw), "stdout_sha256": digest})
            providers[name] = provider
        self.receipt = {"status": "complete", "providers": providers, "launches": launches}
        self.write_receipt()

    def write_receipt(self):
        self.receipt_path.write_text(json.dumps(self.receipt))

    def invoke(self):
        return summarize.main(["--capture", str(self.capture), "--receipt", str(self.receipt_path),
                               "--out", str(self.out)])

    def test_existing_output_is_untouched(self):
        self.out.write_bytes(b"keep exactly this\n")
        with self.assertRaises(FileExistsError):
            self.invoke()
        self.assertEqual(self.out.read_bytes(), b"keep exactly this\n")

    def test_one_capture_hash_mismatch_refuses_whole_output(self):
        (self.capture / "google.show.stdout").write_bytes(b'{"changed":true}\n')
        with self.assertRaisesRegex(ValueError, "capture_receipt_mismatch"):
            self.invoke()
        self.assertFalse(self.out.exists())

    def test_failed_provider_refuses_whole_output(self):
        self.receipt["providers"]["azurerm"]["status"] = "failed"
        self.write_receipt()
        with self.assertRaisesRegex(ValueError, "receipt_incomplete"):
            self.invoke()
        self.assertFalse(self.out.exists())

    def test_launch_byte_count_mismatch_refuses_whole_output(self):
        launch = next(item for item in self.receipt["launches"] if item["command"] == "aws.schema")
        launch["stdout_bytes"] += 1
        self.write_receipt()
        with self.assertRaisesRegex(ValueError, "capture_receipt_mismatch"):
            self.invoke()
        self.assertFalse(self.out.exists())


if __name__ == "__main__":
    unittest.main()
