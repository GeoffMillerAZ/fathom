#!/usr/bin/env python3
"""Fast tests for the isolated OpenCL C compilation-database synthesizer."""

import base64
import importlib.util
import io
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stderr
from unittest import mock


MODULE_PATH = Path(__file__).with_name("compdb.py")
SPEC = importlib.util.spec_from_file_location("opencl_c_compdb", MODULE_PATH)
compdb = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(compdb)


def git(root, *args):
    return subprocess.run(
        ["git", "-C", str(root), *args],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=True,
    ).stdout


class CompDBTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.base = Path(self.temp.name)
        self.subject = self.base / "subject"
        self.subject.mkdir()
        git(self.subject, "init", "-q")
        git(self.subject, "config", "user.email", "test@example.invalid")
        git(self.subject, "config", "user.name", "Test")
        (self.subject / "z.cl").write_bytes(b"kernel void z() {}\n")
        (self.subject / "nested").mkdir()
        (self.subject / "nested" / "a.cl").write_bytes(b"kernel void a() {}\n")
        (self.subject / "nested" / "header.h").write_bytes(b"#define X 1\n")
        os.symlink("z.cl", self.subject / "linked.cl")
        git(self.subject, "add", ".")
        git(self.subject, "commit", "-qm", "fixture")
        git(self.subject, "update-index", "--add", "--cacheinfo",
            "160000," + "1" * 40 + ",vendor/sub")
        git(self.subject, "commit", "-qm", "gitlink")
        self.commit = git(self.subject, "rev-parse", "HEAD").decode().strip()

        self.clang = self.base / "fake-clang"
        self.clang.write_bytes(b"fake clang bytes")
        self.clang_hash = compdb.sha256_file(self.clang)
        self.resource = self.base / "resource"
        (self.resource / "include").mkdir(parents=True)
        (self.resource / "include" / "opencl-c.h").write_bytes(b"opencl")
        (self.resource / "include" / "opencl-c-base.h").write_bytes(b"base")
        self.header_pins = {
            name: compdb.sha256_file(self.resource / "include" / name)
            for name in ("opencl-c.h", "opencl-c-base.h")
        }

    def tearDown(self):
        self.temp.cleanup()

    def test_snapshots_all_regular_files_and_builds_sorted_entries(self):
        out = self.base / "out"
        result = compdb.generate(
            subject=self.subject,
            commit=self.commit,
            clang=self.clang,
            clang_sha256=self.clang_hash,
            resource_dir=self.resource,
            out=out,
            expected_header_sha256=self.header_pins,
        )

        entries = json.loads((out / "compile_commands.json").read_text())
        files = [Path(entry["file"]).relative_to(out / "subject").as_posix()
                 for entry in entries]
        self.assertEqual(files, ["nested/a.cl", "z.cl"])
        self.assertEqual((out / "subject" / "nested" / "header.h").read_bytes(),
                         b"#define X 1\n")
        expected_prefix = [
            str(self.clang.resolve()), "--no-default-config", "-x", "cl",
            "--target=spir64-unknown-unknown", "-cl-std=CL3.0",
            "-resource-dir", str((out / "resource").resolve()),
            "-Xclang", "-finclude-default-header",
            "-c",
        ]
        for entry in entries:
            self.assertEqual(entry["directory"], str((out / "subject").resolve()))
            self.assertEqual(entry["arguments"][:len(expected_prefix)], expected_prefix)
            self.assertEqual(entry["arguments"][-2:], ["-o", os.devnull])
        self.assertEqual(result["source"]["before"]["symlinks"], 1)
        self.assertEqual(result["source"]["before"]["gitlinks"], 1)
        self.assertEqual(result["compile_units"]["files"], 2)
        summary_text = (out / "summary.json").read_text()
        self.assertNotIn(str(self.subject), summary_text)
        self.assertNotIn("a.cl", summary_text)
        self.assertNotIn("header.h", summary_text)

    def test_compiler_pin_mismatch_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "compiler SHA-256"):
            compdb.verify_sha256(self.clang, "0" * 64, "compiler")

        program = f"""
import importlib.util
spec = importlib.util.spec_from_file_location('optimized_compdb', {str(MODULE_PATH)!r})
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
try:
    module.verify_sha256({str(self.clang)!r}, '0' * 64, 'compiler')
except ValueError:
    pass
else:
    raise SystemExit('digest mismatch survived no exception under -O')
"""
        subprocess.run([sys.executable, "-I", "-B", "-O", "-c", program], check=True)

    def test_snapshot_bytes_do_not_follow_later_subject_tampering(self):
        evidence = compdb.triage.capture_checkout(self.subject, self.commit)
        snapshot = self.base / "snapshot"
        compdb.snapshot_regular_files(evidence, snapshot)

        (self.subject / "nested" / "a.cl").write_bytes(b"tampered")

        self.assertEqual((snapshot / "nested" / "a.cl").read_bytes(),
                         b"kernel void a() {}\n")

    def test_snapshot_files_are_read_only(self):
        evidence = compdb.triage.capture_checkout(self.subject, self.commit)
        snapshot = self.base / "snapshot"

        compdb.snapshot_regular_files(evidence, snapshot)

        regular_mode = stat.S_IMODE((snapshot / "nested" / "a.cl").stat().st_mode)
        self.assertEqual(regular_mode, 0o444)

    def test_header_pin_mismatch_is_rejected(self):
        bad = dict(self.header_pins)
        bad["opencl-c.h"] = "0" * 64

        with self.assertRaisesRegex(ValueError, "resource header SHA-256"):
            compdb.hash_resource_headers(self.resource, bad)

    def test_resource_header_pin_set_must_be_exact(self):
        (self.resource / "include" / "stddef.h").write_bytes(b"stddef")
        bad = dict(self.header_pins)
        bad["stddef.h"] = compdb.sha256_file(self.resource / "include" / "stddef.h")

        with self.assertRaisesRegex(ValueError, "exactly"):
            compdb.hash_resource_headers(self.resource, bad)

    def test_copies_only_pinned_resource_headers_read_only(self):
        (self.resource / "include" / "unrelated.h").write_bytes(b"unrelated")
        out = self.base / "out"

        compdb.generate(
            subject=self.subject,
            commit=self.commit,
            clang=self.clang,
            clang_sha256=self.clang_hash,
            resource_dir=self.resource,
            out=out,
            expected_header_sha256=self.header_pins,
        )

        copied = out / "resource" / "include"
        self.assertEqual(sorted(path.name for path in copied.iterdir()),
                         ["opencl-c-base.h", "opencl-c.h"])
        for name, expected in self.header_pins.items():
            path = copied / name
            self.assertEqual(compdb.sha256_file(path), expected)
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o444)

    def test_rejects_non_utf8_tracked_path_before_writing_output(self):
        evidence = compdb.triage.capture_checkout(self.subject, self.commit)
        evidence.rows[0]["path_base64"] = base64.b64encode(b"non-utf8-\xff.cl").decode("ascii")
        out = self.base / "out"

        with mock.patch.object(compdb.triage, "capture_checkout", return_value=evidence):
            with self.assertRaisesRegex(ValueError, "UTF-8"):
                compdb.generate(
                    subject=self.subject,
                    commit=self.commit,
                    clang=self.clang,
                    clang_sha256=self.clang_hash,
                    resource_dir=self.resource,
                    out=out,
                    expected_header_sha256=self.header_pins,
                )

        self.assertFalse(out.exists())

    def test_private_evidence_preserves_raw_git_identities_and_rows(self):
        (self.subject / ".git" / "info" / "exclude").write_text("ignored.tmp\n")
        (self.subject / "untracked.tmp").write_bytes(b"untracked")
        (self.subject / "ignored.tmp").write_bytes(b"ignored")
        out = self.base / "out"

        compdb.generate(
            subject=self.subject,
            commit=self.commit,
            clang=self.clang,
            clang_sha256=self.clang_hash,
            resource_dir=self.resource,
            out=out,
            expected_header_sha256=self.header_pins,
        )

        before = compdb.triage.capture_checkout(self.subject, self.commit)
        private = out / "private"
        self.assertEqual((private / "tracked-before.stage-z").read_bytes(), before.stage_raw)
        self.assertEqual((private / "tracked-after.stage-z").read_bytes(), before.stage_raw)
        self.assertEqual((private / "untracked-before.z").read_bytes(), before.untracked_raw)
        self.assertEqual((private / "untracked-after.z").read_bytes(), before.untracked_raw)
        self.assertEqual((private / "ignored-before.z").read_bytes(), before.ignored_raw)
        self.assertEqual((private / "ignored-after.z").read_bytes(), before.ignored_raw)
        evidence = json.loads((private / "evidence.json").read_text())
        self.assertEqual(evidence["untracked_rows_before"], before.untracked_rows)
        self.assertEqual(evidence["untracked_rows_after"], before.untracked_rows)

    def test_cli_rejects_removed_extra_arg_option(self):
        stderr = io.StringIO()
        argv = [
            "--subject", str(self.subject),
            "--commit", self.commit,
            "--clang", str(self.clang),
            "--clang-sha256", self.clang_hash,
            "--resource-dir", str(self.resource),
            "--out", str(self.base / "out"),
            "--extra-arg=-DVALUE=1",
        ]

        with redirect_stderr(stderr), self.assertRaises(SystemExit):
            compdb.main(argv)

        self.assertIn("unrecognized arguments: --extra-arg", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
