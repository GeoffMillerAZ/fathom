"""Focused tests for the read-only AWS CDK inventory. Copy beside the script as test_inventory.py."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("inventory.py")
TEMP_ROOT = Path(__file__).resolve().parents[3] / "workspace" / "tmp"
TEMP_ROOT.mkdir(parents=True, exist_ok=True)
OPAQUE = "b" * 64


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
            subject = cache / "git-subjects" / OPAQUE
            subject.mkdir()
            (subject / "cdk.json").write_text('{"app":"node app.js"}\n')
            payload, output = self.run_inventory(repo, cache)
            self.assertEqual(payload["counts"]["cdk_json_files_in_subject_roots"], 1)
            self.assertEqual(payload["counts"]["subject_roots_with_cdk_json"], 1)
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
            self.assertEqual(payload["counts"]["subject_roots_with_cdk_json"], 0)
            self.assertNotIn(root, output)


if __name__ == "__main__":
    unittest.main()
