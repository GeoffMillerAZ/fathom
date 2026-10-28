"""Focused tests for the read-only Terragrunt inventory. Copy beside the script as test_inventory.py."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("inventory.py")
TEMP_ROOT = Path(__file__).resolve().parents[3] / "workspace" / "tmp"
TEMP_ROOT.mkdir(parents=True, exist_ok=True)
OPAQUE = "a" * 64


class InventoryTest(unittest.TestCase):
    def run_inventory(self, repo, cache):
        result = subprocess.run(
            [sys.executable, "-I", str(SCRIPT), "--cache-root", str(cache),
             "--repo-root", str(repo)],
            check=True, capture_output=True, text=True, timeout=10,
        )
        return json.loads(result.stdout), result.stdout

    def test_opaque_subject_name_does_not_hide_content_or_leak_path(self):
        with tempfile.TemporaryDirectory(dir=TEMP_ROOT) as root:
            repo, cache = Path(root) / "repo", Path(root) / "cache"
            (repo / "workspace").mkdir(parents=True)
            (cache / "tools").mkdir(parents=True)
            (cache / "subjects").mkdir()
            (cache / "git-subjects").mkdir()
            subject = cache / "subjects" / OPAQUE
            subject.mkdir()
            (subject / "terragrunt.hcl").write_text("locals {}\n")
            (subject / "terragrunt.stack.hcl").write_text("unit \"x\" {}\n")
            payload, output = self.run_inventory(repo, cache)
            counts = payload["counts"]
            self.assertEqual((counts["terragrunt_hcl_files_in_subject_roots"],
                              counts["terragrunt_stack_hcl_files_in_subject_roots"],
                              counts["subject_roots_with_terragrunt_content"]), (1, 1, 1))
            self.assertEqual(payload["missing_roots"], [])
            self.assertEqual(set(payload["root_roles"]), {"tools", "subjects", "git_subjects", "workspace"})
            self.assertNotIn(root, output)
            self.assertNotIn(OPAQUE, output)
            self.assertNotIn("root_sha256", output)

    def test_missing_roots_are_explicit(self):
        with tempfile.TemporaryDirectory(dir=TEMP_ROOT) as root:
            repo, cache = Path(root) / "repo", Path(root) / "cache"
            repo.mkdir()
            cache.mkdir()
            payload, output = self.run_inventory(repo, cache)
            self.assertEqual(set(payload["missing_roots"]), {"tools", "subjects", "git_subjects", "workspace"})
            self.assertEqual(payload["counts"]["subject_roots_with_terragrunt_content"], 0)
            self.assertNotIn(root, output)


if __name__ == "__main__":
    unittest.main()
