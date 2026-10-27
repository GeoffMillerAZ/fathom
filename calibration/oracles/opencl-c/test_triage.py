#!/usr/bin/env python3
"""Focused tests for the isolated OpenCL C native grammar triage."""

import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


MODULE_PATH = Path(__file__).with_name("triage.py")
SPEC = importlib.util.spec_from_file_location("opencl_c_triage", MODULE_PATH)
triage = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(triage)


class Node:
    def __init__(self, node_type="node", *, missing=False, children=()):
        self.type = node_type
        self.is_missing = missing
        self.children = list(children)


class Tree:
    def __init__(self, root):
        self.root_node = root


class FakeParser:
    def __init__(self, results):
        self.results = iter(results)

    def parse(self, _source):
        result = next(self.results)
        if isinstance(result, Exception):
            raise result
        return result


def git(root, *args, input_bytes=None):
    return subprocess.run(
        ["git", "-C", str(root), *args],
        input=input_bytes,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=True,
    ).stdout


class ParsePopulationTests(unittest.TestCase):
    def test_counts_error_missing_both_clean_and_faults(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            paths = []
            for index in range(5):
                path = root / f"{index}.cl"
                path.write_bytes(b"kernel void k() {}")
                paths.append(path)
            parser = FakeParser([
                Tree(Node(children=[Node("ERROR")])),
                Tree(Node(children=[Node(missing=True)])),
                Tree(Node(children=[Node("ERROR", missing=True)])),
                Tree(Node()),
                RuntimeError("parse failed"),
            ])

            summary, rows = triage.parse_population(paths, parser, root=root)

            self.assertEqual(summary["parsed_count"], 4)
            self.assertEqual(summary["error_or_missing_count"], 3)
            self.assertEqual(summary["error_only_count"], 1)
            self.assertEqual(summary["missing_only_count"], 1)
            self.assertEqual(summary["both_count"], 1)
            self.assertEqual(summary["fault_count"], 1)
            self.assertEqual(summary["clean_count"], 1)
            self.assertEqual(summary["clean_rate"], 0.2)
            self.assertEqual(len(rows), 5)
            self.assertEqual(rows[-1]["fault"], "RuntimeError")

    def test_walks_every_node_even_without_parent_error_hint(self):
        with tempfile.TemporaryDirectory() as td:
            path = Path(td) / "nested.cl"
            path.write_bytes(b"x")
            root = Node(children=[Node(children=[Node(missing=True), Node("ERROR")])])

            summary, _ = triage.parse_population([path], FakeParser([Tree(root)]), root=path.parent)

            self.assertEqual(summary["both_count"], 1)


class CheckoutEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        git(self.root, "init", "-q")
        git(self.root, "config", "user.email", "test@example.invalid")
        git(self.root, "config", "user.name", "Test")
        (self.root / ".gitignore").write_text("ignored.cl\n")
        (self.root / "a.cl").write_bytes(b"a")
        (self.root / "nested").mkdir()
        (self.root / "nested" / "b.cl").write_bytes(b"bb")
        (self.root / "upper.CL").write_bytes(b"no")
        (self.root / "note.txt").write_bytes(b"note")
        os.symlink("a.cl", self.root / "linked.cl")
        git(self.root, "add", ".")
        git(self.root, "commit", "-qm", "fixture")
        gitlink_oid = "1" * 40
        git(self.root, "update-index", "--add", "--cacheinfo", "160000," + gitlink_oid + ",vendor/sub")
        git(self.root, "commit", "-qm", "gitlink")
        self.commit = git(self.root, "rev-parse", "HEAD").decode().strip()

    def tearDown(self):
        self.temp.cleanup()

    def test_manifest_hashes_regular_symlink_and_gitlink_and_includes_all_dot_cl(self):
        evidence = triage.capture_checkout(self.root, self.commit)

        self.assertEqual([p.relative_to(self.root.resolve()).as_posix() for p in evidence.eligible],
                         ["a.cl", "nested/b.cl"])
        self.assertEqual(evidence.public["eligible"], {"files": 2, "bytes": 3})
        self.assertEqual(evidence.public["tracked"]["regular_files"], 5)
        self.assertEqual(evidence.public["tracked"]["symlinks"], 1)
        self.assertEqual(evidence.public["tracked"]["gitlinks"], 1)
        gitlink = next(row for row in evidence.rows if row["kind"] == "gitlink")
        self.assertEqual(gitlink["object_id"], "1" * 40)
        symlink = next(row for row in evidence.rows if row["kind"] == "symlink")
        self.assertEqual(symlink["bytes"], len(b"a.cl"))

    def test_refuses_dirty_tracked_checkout(self):
        (self.root / "a.cl").write_bytes(b"changed")

        with self.assertRaisesRegex(ValueError, "tracked checkout is dirty"):
            triage.capture_checkout(self.root, self.commit)

    def test_refuses_dirty_index(self):
        (self.root / "a.cl").write_bytes(b"staged")
        git(self.root, "add", "a.cl")

        with self.assertRaisesRegex(ValueError, "tracked checkout is dirty"):
            triage.capture_checkout(self.root, self.commit)

    def test_refuses_assume_unchanged_tampering(self):
        git(self.root, "update-index", "--assume-unchanged", "a.cl")
        (self.root / "a.cl").write_bytes(b"hidden change")

        with self.assertRaisesRegex(ValueError, "tracked content does not match index"):
            triage.capture_checkout(self.root, self.commit)

    def test_refuses_skip_worktree_tampering(self):
        git(self.root, "update-index", "--skip-worktree", "nested/b.cl")
        (self.root / "nested" / "b.cl").write_bytes(b"hidden change")

        with self.assertRaisesRegex(ValueError, "tracked content does not match index"):
            triage.capture_checkout(self.root, self.commit)

    def test_refuses_assume_unchanged_symlink_tampering(self):
        git(self.root, "update-index", "--assume-unchanged", "linked.cl")
        (self.root / "linked.cl").unlink()
        os.symlink("note.txt", self.root / "linked.cl")

        with self.assertRaisesRegex(ValueError, "tracked content does not match index"):
            triage.capture_checkout(self.root, self.commit)

    def test_refuses_missing_tracked_file(self):
        (self.root / "note.txt").unlink()

        with self.assertRaises(ValueError):
            triage.capture_checkout(self.root, self.commit)

    def test_untracked_and_ignored_identities_are_private_and_detect_changes(self):
        (self.root / "secret-untracked.cl").write_bytes(b"u")
        (self.root / "ignored.cl").write_bytes(b"i")
        before = triage.capture_checkout(self.root, self.commit)
        self.assertEqual(before.public["untracked"]["files"], 2)
        self.assertNotIn("secret-untracked", json.dumps(before.public))
        self.assertTrue(any(row["path"] == "secret-untracked.cl" for row in before.untracked_rows))
        (self.root / "later.cl").write_bytes(b"later")
        after = triage.capture_checkout(self.root, self.commit)

        with self.assertRaisesRegex(ValueError, "checkout changed"):
            triage.assert_checkout_unchanged(before, after)

    def test_parse_uses_verified_bytes_during_transient_mutation(self):
        before = triage.capture_checkout(self.root, self.commit)
        second = self.root.resolve() / "nested" / "b.cl"

        class MutatingParser:
            def __init__(self):
                self.seen = []

            def parse(inner_self, source):
                inner_self.seen.append(source)
                if len(inner_self.seen) == 1:
                    second.write_bytes(b"transient tampering")
                else:
                    second.write_bytes(b"bb")
                return Tree(Node())

        parser = MutatingParser()
        triage.parse_population(
            before.eligible,
            parser,
            root=before.root,
            captured_sources=before.eligible_sources,
        )
        after = triage.capture_checkout(self.root, self.commit)
        triage.assert_checkout_unchanged(before, after)

        self.assertEqual(parser.seen, [b"a", b"bb"])


class ReceiptPrivacyTests(unittest.TestCase):
    def test_public_receipt_excludes_private_paths_rows_and_fault_messages(self):
        public = {
            "expected_commit": "a" * 40,
            "tracked": {"manifest_sha256": "b" * 64, "entries": 1, "bytes": 3},
            "eligible": {"files": 1, "bytes": 3},
            "untracked": {"files": 0, "ignored_files": 0, "paths_sha256": "c" * 64},
            "grammars": [{"id": "opencl", "library_sha256": "d" * 64,
                          "parsed_count": 1, "fault_count": 0}],
        }
        private = {"subject": "/Users/person/private/repo", "rows": [{"path": "person.cl"}]}
        with tempfile.TemporaryDirectory() as td:
            triage.write_receipts(Path(td), public, private)
            summary = (Path(td) / "summary.json").read_text()
            private_text = (Path(td) / "private" / "evidence.json").read_text()

        self.assertNotIn("/Users/person", summary)
        self.assertNotIn("person.cl", summary)
        self.assertIn("/Users/person/private/repo", private_text)


if __name__ == "__main__":
    unittest.main()
