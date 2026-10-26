"""Source-scope controls for the read-only CSS grammar census."""

import json
from pathlib import Path
import subprocess
import tempfile
import unittest

from gate import canonical_json, check_manifest, checked_scope, sha256


def git(root, *args):
    subprocess.run(["git", "-C", str(root), *args], check=True, stdout=subprocess.DEVNULL)


def canonical_temp_root(path):
    return Path(path).resolve()


def sidecar_for(root, destination, names):
    commit = subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()
    files = [{"path": name, "sha256": sha256((root / name).read_bytes())} for name in sorted(names)]
    manifest = {"commit": commit, "files": files}
    manifest["digest"] = sha256(canonical_json(manifest))
    destination.write_text(json.dumps({"source_manifest": manifest}))
    return destination


class ManifestScopeTest(unittest.TestCase):
    def test_symlinked_temp_root_is_resolved_before_scope_and_manifest_checks(self):
        with tempfile.TemporaryDirectory() as scratch:
            base = canonical_temp_root(scratch)
            actual = base / "actual"
            actual.mkdir()
            alias = base / "alias"
            alias.symlink_to(actual, target_is_directory=True)
            root = canonical_temp_root(alias)
            self.assertNotEqual(alias, root)
            (root / "css").mkdir()
            (root / "css" / "a.css").write_text(".a {}\n")
            git(root, "init", "-q")
            git(root, "add", ".")
            git(root, "-c", "user.name=Fixture", "-c", "user.email=fixture@test.invalid", "commit", "-qm", "fixture")
            sidecar = sidecar_for(root, base / "source.json", ["css/a.css"])
            self.assertEqual(checked_scope(root, "."), "")
            self.assertEqual(check_manifest(root, sidecar, ".")["scope"], ".")

    def test_declared_root_rejects_truncated_manifest_even_when_listed_files_share_subdir(self):
        with tempfile.TemporaryDirectory() as scratch:
            base = canonical_temp_root(scratch)
            root = base / "repo"
            root.mkdir()
            (root / "css").mkdir()
            (root / "root.css").write_text(".root {}\n")
            (root / "css" / "a.css").write_text(".a {}\n")
            git(root, "init", "-q")
            git(root, "add", ".")
            git(root, "-c", "user.name=Fixture", "-c", "user.email=fixture@test.invalid", "commit", "-qm", "fixture")
            full = sidecar_for(root, base / "full.source.json", ["css/a.css", "root.css"])
            self.assertEqual(check_manifest(root, full, ".")["scope"], ".")

            truncated = sidecar_for(root, base / "truncated.source.json", ["css/a.css"])
            with self.assertRaisesRegex(ValueError, "complete CSS list"):
                check_manifest(root, truncated, ".")
            self.assertEqual(check_manifest(root, truncated, "css")["scope"], "css")

            altered = json.loads(full.read_text())
            altered["source_manifest"]["files"][0]["sha256"] = "0" * 64
            identity = altered["source_manifest"]
            identity["digest"] = sha256(canonical_json({"commit": identity["commit"], "files": identity["files"]}))
            bad_hash = base / "bad-hash.source.json"
            bad_hash.write_text(json.dumps(altered))
            with self.assertRaisesRegex(ValueError, "source bytes differ"):
                check_manifest(root, bad_hash, ".")

            (root / "css" / "a.css").write_text(".a { color: blue; }\n")
            with self.assertRaisesRegex(ValueError, "dirty|bytes differ"):
                check_manifest(root, full, ".")

    def test_scope_must_be_canonical_directory(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = canonical_temp_root(scratch)
            (root / "css").mkdir()
            self.assertEqual(checked_scope(root, "."), "")
            self.assertEqual(checked_scope(root, "css"), "css")
            for bad in ("", "/css", "../css", "css/../css", "css/.", "css//x", "css\\x", "missing"):
                with self.subTest(bad=bad), self.assertRaises(ValueError):
                    checked_scope(root, bad)


if __name__ == "__main__":
    unittest.main()
