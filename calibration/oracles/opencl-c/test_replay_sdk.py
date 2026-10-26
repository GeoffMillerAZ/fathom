#!/usr/bin/env python3
"""Fast tests for the pinned OpenCL SDK replay guards."""

import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock


MODULE_PATH = Path(__file__).with_name("replay-sdk.py")
SPEC = importlib.util.spec_from_file_location("opencl_c_replay_sdk", MODULE_PATH)
replay = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(replay)


class TreeIdentityTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name) / "tree"
        self.root.mkdir()
        (self.root / "empty").mkdir()
        self.source = self.root / "source.cl"
        self.source.write_bytes(b"kernel void k() {}\n")
        self.source.chmod(0o444)

    def tearDown(self):
        self.temp.cleanup()

    def test_identity_covers_bytes_modes_and_empty_directories(self):
        original = replay.tree_identity(self.root)

        self.source.chmod(0o644)
        mode_changed = replay.tree_identity(self.root)
        self.assertNotEqual(mode_changed, original)

        self.source.chmod(0o444)
        (self.root / "another-empty").mkdir()
        directory_changed = replay.tree_identity(self.root)
        self.assertNotEqual(directory_changed, original)

        (self.root / "another-empty").rmdir()
        self.source.chmod(0o644)
        self.source.write_bytes(b"changed\n")
        self.source.chmod(0o444)
        self.assertNotEqual(replay.tree_identity(self.root), original)

    def test_identity_rejects_symlinks(self):
        os.symlink("source.cl", self.root / "linked.cl")

        with self.assertRaisesRegex(ValueError, "symlink or special"):
            replay.tree_identity(self.root)

    def test_identity_rejects_special_files(self):
        os.mkfifo(self.root / "pipe")

        with self.assertRaisesRegex(ValueError, "symlink or special"):
            replay.tree_identity(self.root)


class PreprocessorReceiptTests(unittest.TestCase):
    def setUp(self):
        self.roots = {"a.cl", "nested/b.cl"}
        self.output = {"active_lines": {
            "a.cl": [[1, 2]],
            "nested/b.cl": [[3, 3]],
        }}
        self.stderr = (
            "translation_units 2 stopped 0 files 2 includes_in_subject 0\n"
        )

    def test_accepts_complete_successful_root_set(self):
        replay.validate_preprocessor_receipt(
            self.output, self.stderr, self.roots)

    def test_rejects_stopped_or_incomplete_or_malformed_receipts(self):
        cases = [
            (self.output,
             "translation_units 2 stopped 1 files 2 includes_in_subject 0\n",
             "stopped"),
            (self.output,
             "translation_units 1 stopped 0 files 2 includes_in_subject 0\n",
             "translation-unit"),
            ({"active_lines": {"a.cl": [[1, 2]], "wrong.cl": [[3, 3]]}},
             self.stderr,
             "compile roots"),
            (self.output, "unrelated diagnostic\n", "summary"),
        ]
        for output, stderr, message in cases:
            with self.subTest(message=message):
                with self.assertRaisesRegex(ValueError, message):
                    replay.validate_preprocessor_receipt(output, stderr, self.roots)

    def test_compile_roots_are_subject_relative_and_unique(self):
        with tempfile.TemporaryDirectory() as temp:
            subject = Path(temp) / "subject"
            subject.mkdir()
            nested = subject / "nested"
            nested.mkdir()
            database = Path(temp) / "compile_commands.json"
            database.write_text(json.dumps([
                {"directory": str(subject), "file": "a.cl", "arguments": ["clang"]},
                {"directory": str(subject), "file": str(nested / "b.cl"),
                 "arguments": ["clang"]},
            ]), encoding="utf-8")

            self.assertEqual(
                replay.compile_roots(database, subject),
                {"a.cl", "nested/b.cl"},
            )

            database.write_text(json.dumps([
                {"directory": str(subject), "file": "a.cl", "arguments": ["clang"]},
                {"directory": str(subject), "file": "a.cl", "arguments": ["clang"]},
            ]), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "duplicate"):
                replay.compile_roots(database, subject)


class PrivateLayoutTests(unittest.TestCase):
    def test_entrypoint_creates_fresh_nested_out_before_generation(self):
        class GenerationReached(RuntimeError):
            pass

        workspace = replay.ROOT / "workspace"
        with tempfile.TemporaryDirectory(dir=workspace) as temp:
            root = Path(temp)
            sdk = root / "sdk"
            sdk.mkdir()
            work = root / "fresh" / "nested" / "replay"
            compiler_pin = "0" * 64

            def stop_after_layout(**kwargs):
                self.assertEqual(kwargs["out"], work / "private" / "database")
                self.assertTrue(work.is_dir())
                raise GenerationReached("generate reached")

            argv = [
                "replay-sdk.py",
                "--sdk", str(sdk),
                "--clang", str(root / "clang"),
                "--resource-dir", str(root / "resource"),
                "--out", str(work),
            ]
            with (mock.patch.object(replay.sys, "argv", argv),
                  mock.patch.object(replay.os, "getpriority", return_value=10),
                  mock.patch.object(replay.compdb, "_load_compiler_pins",
                                    return_value=(compiler_pin, {})),
                  mock.patch.object(replay.compdb, "generate",
                                    side_effect=stop_after_layout) as generate,
                  mock.patch.object(replay.subprocess, "run") as subprocess_run):
                with self.assertRaisesRegex(GenerationReached, "generate reached"):
                    replay.main()

            generate.assert_called_once()
            subprocess_run.assert_not_called()


if __name__ == "__main__":
    unittest.main()
