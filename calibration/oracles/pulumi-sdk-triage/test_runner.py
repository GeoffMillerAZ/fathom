"""Refusal/privacy tests. No compiler, indexer, or Pulumi program is launched."""

import importlib.util
import hashlib
import json
import os
import sys
from pathlib import Path
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("pulumi_triage_runner", Path(__file__).with_name("runner.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)
FIXTURE = Path(__file__).with_name("fixture")


class RefusalTests(unittest.TestCase):
    def test_wrong_named_cache_refused_before_any_native_launch(self):
        with tempfile.TemporaryDirectory() as temp, mock.patch.object(runner.subprocess, "Popen") as launch:
            with self.assertRaisesRegex(ValueError, "named pulumi-probe"):
                runner.run(Path(temp), Path(temp) / "scratch", Path(temp) / "receipt.json")
            launch.assert_not_called()

    def test_stale_source_pin_refused_before_any_native_launch(self):
        with tempfile.TemporaryDirectory() as temp, mock.patch.object(runner.subprocess, "Popen") as launch:
            root = Path(temp) / "pulumi-probe"
            root.mkdir()
            for name in runner.FILES:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(b"changed")
            with self.assertRaisesRegex(ValueError, "frozen cache input differs"):
                runner.run(root, Path(temp) / "scratch", Path(temp) / "receipt.json")
            launch.assert_not_called()

    def test_existing_scratch_refused_before_any_native_launch(self):
        with tempfile.TemporaryDirectory() as temp, mock.patch.object(runner, "validate_cache") as cache, \
                mock.patch.object(runner, "installed_tree") as sdk_tree, \
                mock.patch.object(runner.subprocess, "Popen") as launch:
            cache.return_value = Path(temp) / "pulumi-probe"
            sdk_tree.return_value = Path(temp) / "installed"
            scratch = Path(temp) / "scratch"
            scratch.mkdir()
            with self.assertRaisesRegex(ValueError, "scratch must be fresh"):
                runner.run(cache.return_value, scratch, Path(temp) / "receipt.json")
            launch.assert_not_called()

    def test_cache_output_refused_before_any_native_launch(self):
        with tempfile.TemporaryDirectory() as temp, mock.patch.object(runner, "validate_cache") as cache, \
                mock.patch.object(runner, "installed_tree") as sdk_tree, \
                mock.patch.object(runner.subprocess, "Popen") as launch:
            root = Path(temp).resolve() / "pulumi-probe"
            root.mkdir()
            cache.return_value = root
            sdk_tree.return_value = Path(temp).resolve() / "installed"
            with self.assertRaisesRegex(ValueError, "output must be separate"):
                runner.run(root, Path(temp) / "scratch", root / "receipt.json")
            launch.assert_not_called()

    def test_load_guard_refuses_high_load(self):
        with mock.patch.object(runner.os, "getloadavg", return_value=(10.0, 0.0, 0.0)), \
                mock.patch.object(runner.time, "monotonic", side_effect=[0.0, 0.0, 1.0]):
            with self.assertRaisesRegex(RuntimeError, "load remained above 9"):
                runner.wait_load(seconds=0.1)

    def test_tree_digest_detects_byte_change_without_following_symlinks(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "node_modules"
            root.mkdir()
            (root / "package.js").write_bytes(b"one")
            (root / "link").symlink_to("../outside")
            before = runner.tree_digest(root)
            self.assertEqual(before["counts"], {"directories": 0, "files": 1, "symlinks": 1})
            (Path(temp) / "outside").write_bytes(b"outside changed")
            self.assertEqual(runner.tree_digest(root), before)
            (root / "package.js").write_bytes(b"two")
            self.assertNotEqual(runner.tree_digest(root)["sha256"], before["sha256"])

    def test_absolute_sdk_symlink_refused_before_path_can_enter_digest(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "node_modules"
            root.mkdir()
            (root / "link").symlink_to(Path(temp) / "private-target")
            with self.assertRaisesRegex(ValueError, "absolute SDK symlink"):
                runner.tree_digest(root)

    def test_committed_lock_and_pin_metadata_match_frozen_inputs(self):
        for name in ("package-lock.json", "pins-before-install.json"):
            raw = (FIXTURE / name).read_bytes()
            self.assertEqual(hashlib.sha256(raw).hexdigest(), runner.FILES[name])
        lock = json.loads((FIXTURE / "package-lock.json").read_text())
        pins = json.loads((FIXTURE / "pins-before-install.json").read_text())
        for pin in pins:
            name = pin["name"]
            self.assertEqual(lock["packages"]["node_modules/" + name]["version"], runner.VERSIONS[name])
            self.assertEqual(lock["packages"]["node_modules/" + name]["integrity"], pin["dist"]["integrity"])

    def test_cwd_echo_receipts_are_identical_across_different_scratch_roots(self):
        # Exercise the real child boundary: the command echoes both cwd and a
        # path-shaped diagnostic. Scheduling values are fixed independently.
        with tempfile.TemporaryDirectory() as temp:
            records = []
            for name in ("short", "a-different-and-longer-scratch-root"):
                cwd = Path(temp) / name
                cwd.mkdir()
                code = "import os,sys; print(os.getcwd()); print(os.getcwd(), file=sys.stderr)"
                with mock.patch.object(runner, "wait_load", return_value=2.0), \
                        mock.patch.object(runner.time, "monotonic", return_value=0.0):
                    record = runner.bounded([sys.executable, "-c", code], cwd,
                                            {"PATH": "/usr/bin:/bin", "LANG": "C"})
                self.assertEqual(record["exit"], 0)
                public = json.dumps(record)
                self.assertNotIn(str(cwd), public)
                self.assertNotIn(hashlib.sha256((str(cwd) + "\n").encode()).hexdigest(), public)
                for stream in ("stdout", "stderr"):
                    self.assertNotIn(stream + "_bytes", record)
                    self.assertNotIn(stream + "_sha256", record)
                records.append(record)
            self.assertEqual(records[0], records[1])

    def test_public_launch_record_contains_policy_but_no_private_profile_hash(self):
        with tempfile.TemporaryDirectory() as temp:
            staged = Path(temp) / "staged"
            sdk = Path(temp) / "sdk"
            profile = runner.sandbox_profile(staged, sdk)
            self.assertIn(str(staged), profile)
            self.assertIn(str(sdk), profile)
            record = runner.public_launch_record(0, 1.0, 2.0, 10, b"", b"")
            serialized = json.dumps(record)
            self.assertNotIn(temp, serialized)
            self.assertNotIn(hashlib.sha256(profile.encode()).hexdigest(), serialized)
            self.assertIn("deny writes to staged cache", serialized)
            self.assertNotIn("profile_sha256", record)


if __name__ == "__main__":
    unittest.main()
