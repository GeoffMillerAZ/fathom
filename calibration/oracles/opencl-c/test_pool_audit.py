#!/usr/bin/env python3
"""Focused tests for the OpenCL C frozen-pool audit."""

import contextlib
import io
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


HERE = Path(__file__).parent
sys.path.insert(0, str(HERE))
SPEC = importlib.util.spec_from_file_location("opencl_c_pool_audit", HERE / "pool_audit.py")
pool_audit = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(pool_audit)


def git(root, *args):
    return subprocess.run(
        ["git", "-C", str(root), *args], stdout=subprocess.PIPE,
        stderr=subprocess.PIPE, check=True).stdout


def make_repo(root, files):
    root.mkdir()
    git(root, "init", "-q")
    git(root, "config", "user.email", "test@example.invalid")
    git(root, "config", "user.name", "Test")
    for name, content in files.items():
        path = root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(content)
    git(root, "add", ".")
    git(root, "commit", "-qm", "fixture")
    return git(root, "rev-parse", "HEAD").decode().strip()


def policy_data(subjects, diagnostics=()):
    return {
        "schema": pool_audit.POLICY_SCHEMA,
        "osi_approved_licenses": ["MIT", "Apache-2.0"],
        "primary_subjects": [
            {"id": subject_id, "pin": pin, "licenses": ["MIT"], "role": "primary"}
            for subject_id, pin in subjects
        ],
        "diagnostic_or_ineligible": [
            {"id": subject_id, "reason": "fixed diagnostic role"} for subject_id in diagnostics
        ],
    }


class PoolAuditTests(unittest.TestCase):
    def test_isolated_cli_loads_sibling_module(self):
        result = subprocess.run([sys.executable, "-I", "-B", str(HERE / "pool_audit.py"), "--help"], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("--subject", result.stdout)

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.base = Path(self.temp.name)
        self.repo = self.base / "repo"
        self.pin = make_repo(self.repo, {
            "a.cl": b"same", "copy.cl": b"same", "nested/b.cl": b"other",
            "upper.CL": b"upper", "note.txt": b"note",
        })
        os.symlink("a.cl", self.repo / "linked.cl")
        git(self.repo, "add", "linked.cl")
        git(self.repo, "commit", "-qm", "symlink")
        self.pin = git(self.repo, "rev-parse", "HEAD").decode().strip()

    def tearDown(self):
        self.temp.cleanup()

    def write_policy(self, data):
        path = self.base / "policy.json"
        path.write_text(json.dumps(data), encoding="utf-8")
        return path

    def load_policy(self, data):
        return pool_audit.load_policy(self.write_policy(data))

    def test_counts_duplicate_files_and_excludes_nonregular_and_uppercase(self):
        public, private = pool_audit.audit(
            self.load_policy(policy_data([("one", self.pin)])), [("one", self.repo)])

        self.assertEqual(public["primary"][0], {
            "id": "one", "role": "primary", "files": 3,
            "distinct_blobs": 2, "bytes": 13,
        })
        self.assertEqual(sum(row["files"] for row in private["subjects"][0]["blobs"]), 3)

    def test_refuses_wrong_full_pin(self):
        policy = self.load_policy(policy_data([("one", "0" * 40)]))
        with self.assertRaisesRegex(ValueError, "HEAD does not match"):
            pool_audit.audit(policy, [("one", self.repo)])

    def test_refuses_dirty_index_that_does_not_match_head(self):
        (self.repo / "a.cl").write_bytes(b"dirty")
        git(self.repo, "add", "a.cl")
        policy = self.load_policy(policy_data([("one", self.pin)]))
        with self.assertRaisesRegex(ValueError, "dirty"):
            pool_audit.audit(policy, [("one", self.repo)])

    def test_refuses_non_osi_primary_license(self):
        data = policy_data([("one", self.pin)])
        data["primary_subjects"][0]["licenses"].append("CC-BY-3.0")
        with self.assertRaisesRegex(ValueError, "every primary license"):
            self.load_policy(data)

    def test_outcome_fields_cannot_change_membership(self):
        clean = policy_data([("one", self.pin)])
        failed = json.loads(json.dumps(clean))
        clean["primary_subjects"][0]["parse_outcome"] = "clean"
        failed["primary_subjects"][0]["parse_outcome"] = "failed"

        self.assertEqual(self.load_policy(clean)["primary"], self.load_policy(failed)["primary"])

    def test_requires_every_and_only_policy_primary(self):
        policy = self.load_policy(policy_data([("one", self.pin), ("two", self.pin)]))
        with self.assertRaisesRegex(ValueError, "exactly match"):
            pool_audit.audit(policy, [("one", self.repo)])

    def test_refuses_duplicate_supplied_subject(self):
        policy = self.load_policy(policy_data([("one", self.pin)]))
        with self.assertRaisesRegex(ValueError, "unique"):
            pool_audit.audit(policy, [("one", self.repo), ("one", self.repo)])

    def test_diagnostic_is_reported_separately_and_never_joins_primary(self):
        policy = self.load_policy(policy_data([("one", self.pin)], ["diag"]))
        public, _ = pool_audit.audit(
            policy, [("one", self.repo)], [("diag", self.repo, self.pin)])

        self.assertEqual(public["primary_totals"]["subjects"], 1)
        self.assertEqual(public["diagnostic_totals"]["subjects"], 1)
        self.assertEqual(public["diagnostics"][0]["role"], "diagnostic")

    def test_refuses_undeclared_diagnostic(self):
        policy = self.load_policy(policy_data([("one", self.pin)]))
        with self.assertRaisesRegex(ValueError, "not declared"):
            pool_audit.audit(policy, [("one", self.repo)], [("extra", self.repo, self.pin)])

    def test_cli_keeps_checkout_pin_blob_and_filename_out_of_public_summary(self):
        policy_path = self.write_policy(policy_data([("one", self.pin)]))
        old_cwd = Path.cwd()
        try:
            os.chdir(self.base)
            with contextlib.redirect_stdout(io.StringIO()):
                result = pool_audit.main([
                    "--policy", str(policy_path), "--subject", f"one={self.repo}",
                    "--out", "workspace/summary.json",
                ])
        finally:
            os.chdir(old_cwd)
        self.assertEqual(result, 0)
        text = (self.base / "workspace" / "summary.json").read_text()
        private = (self.base / "workspace" / "private" / "summary-evidence.json").read_text()

        self.assertNotIn(str(self.repo), text)
        self.assertNotIn(self.pin, text)
        self.assertNotIn("a.cl", text)
        self.assertNotIn("object_id", text)
        self.assertIn(str(self.repo.resolve()), private)


if __name__ == "__main__":
    unittest.main()
