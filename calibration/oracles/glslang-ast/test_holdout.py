"""Synthetic tests for the independent GLSL holdout driver."""
import importlib.util
import io
import json
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest import mock

spec = importlib.util.spec_from_file_location("holdout", Path(__file__).with_name("holdout.py"))
holdout = importlib.util.module_from_spec(spec)
spec.loader.exec_module(holdout)


class HoldoutTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        Path("workspace/tmp").mkdir(parents=True, exist_ok=True)

    def test_population_includes_every_stage_extension_and_sniffed_fs(self):
        with tempfile.TemporaryDirectory(dir="workspace/tmp") as tmp:
            root = Path(tmp)
            for extension in holdout.STAGE_EXTENSIONS:
                (root / ("a" + extension)).write_text("void main() {}")
            (root / "a.glsl").write_text("void main() {}")
            (root / "shader.fs").write_text("#version 460\nvoid main() {}")
            (root / "commented_shader.fs").write_text("// comment\n#version 460\nvoid main() {}")
            (root / "not_shader.fs").write_text("module App")
            (root / "ignore.txt").write_text("#version 460")
            files, rejected = holdout.population(root)
            self.assertEqual(len(files), 19)
            self.assertEqual(rejected, 1)
            self.assertEqual(len({row["path"] for row in files}), len(files))

    def test_archive_rejects_traversal_and_links(self):
        with tempfile.TemporaryDirectory(dir="workspace/tmp") as tmp:
            root = Path(tmp)
            for name, symlink in (("../escape", False), ("root/link", True)):
                archive = root / "bad.tar.gz"
                with tarfile.open(archive, "w:gz") as tar:
                    info = tarfile.TarInfo(name)
                    if symlink:
                        info.type = tarfile.SYMTYPE
                        info.linkname = "target"
                    else:
                        info.size = 1
                    tar.addfile(info, io.BytesIO(b"x") if not symlink else None)
                with self.assertRaises(ValueError):
                    holdout.extract_safe(archive, root / ("out-link" if symlink else "out-traversal"))

    def test_oracle_scope_and_shadowing(self):
        rows = [
            {"path": "a.vert", "status": "parsed", "definitions": ["main", "custom"],
             "sites": [["main", "custom"], ["main", "sin"]]},
            {"path": "b.frag", "status": "unknown_stage", "definitions": [], "sites": []},
        ]
        with self.assertRaisesRegex(ValueError, "95%"):
            holdout.make_oracle(rows, {"sin"}, {"vec4"})
        rows.pop()
        oracle, summary = holdout.make_oracle(rows, {"sin"}, {"vec4"})
        self.assertEqual(oracle["calls"], [{"path": "a.vert", "from": "main", "to": "custom"}])
        self.assertEqual(summary["user_call_denominator"], 1)
        rows[0]["definitions"].append("sin")
        with self.assertRaisesRegex(ValueError, "shadow"):
            holdout.make_oracle(rows, {"sin"}, {"vec4"})

    def test_baseline_keeps_original_core_constructor_exclusions(self):
        names = holdout.legacy_constructor_names()
        self.assertEqual(len(names), 44)
        self.assertTrue({"bool", "float", "vec4", "dvec2", "mat4x3"} <= names)
        self.assertNotIn("sampler2D", names)
        rows = [{"path": "a.vert", "status": "parsed", "definitions": ["main"],
                 "sites": [["main", "vec4"]]}]
        oracle, _ = holdout.make_oracle(rows, {"sin"}, names)
        self.assertIn({"path": "a.vert", "to": "vec4", "reason": "pinned catalog type identifier"},
                      oracle["call_exclusions"])

    def test_below_coverage_diagnostic_has_no_validation_claim(self):
        rows = [{"path": "a.vert", "status": "parsed", "definitions": ["main", "helper"],
                 "sites": [["main", "helper"]]},
                {"path": "b.frag", "status": "compile_failed", "definitions": [], "sites": []}]
        with self.assertRaisesRegex(ValueError, "95%"):
            holdout.make_oracle(rows, set(), set())
        oracle, summary = holdout.make_oracle(rows, set(), set(), diagnostic=True)
        self.assertEqual(oracle["scope"], ["a.vert"])
        self.assertEqual(summary["population"], 2)
        self.assertFalse(summary["validation_claimable"])
        self.assertTrue(summary["diagnostic_only"])
        self.assertEqual(summary["user_call_denominator"], 1)

    def test_catalog_modes_keep_independent_calls_fixed(self):
        rows = [{"path": "a.vert", "status": "parsed", "definitions": ["main", "custom"],
                 "sites": [["main", "custom"], ["main", "texture"]]}]
        core, a = holdout.make_oracle(rows, {"sin"}, set())
        full, b = holdout.make_oracle(rows, {"sin", "texture"}, set())
        self.assertEqual(core["calls"], full["calls"])
        self.assertEqual(a["user_call_denominator"], b["user_call_denominator"])
        self.assertEqual(len(full["call_exclusions"]), 2)

    def test_source_manifest_detects_changed_bytes(self):
        with tempfile.TemporaryDirectory(dir="workspace/tmp") as tmp:
            root = Path(tmp)
            source = root / "a.vert"
            source.write_text("void main() {}")
            files, _ = holdout.population(root)
            source.write_text("void main() { } ")
            with self.assertRaisesRegex(ValueError, "changed"):
                holdout.verify_manifest(root, files)

    def test_source_commit_rejects_untracked_files(self):
        sha = "a" * 40
        with mock.patch.object(holdout.subprocess, "check_output", side_effect=[sha + "\n", "?? new.py\n"]) as call:
            with self.assertRaisesRegex(ValueError, "dirty"):
                holdout.checked_source_commit(Path("."), sha)
        self.assertIn("--untracked-files=all", call.call_args_list[1].args[0])

    def test_neutral_reading_requires_real_call_counts(self):
        with self.assertRaisesRegex(ValueError, "lacks required"):
            holdout.call_counts({"counts": {"call_truth": 1}}, "baseline-core")

    def test_unverified_include_origin_is_a_gap_without_compilation(self):
        with tempfile.TemporaryDirectory(dir="workspace/tmp") as tmp:
            root = Path(tmp)
            (root / "a.frag").write_text('#include "defs.glsl"\nvoid main() {}')
            files, _ = holdout.population(root)
            with mock.patch.object(holdout.subprocess, "run") as run:
                rows = holdout.compile_rows(root, files, Path("/unused/glslang"), root / "receipt.json")
            run.assert_not_called()
            self.assertEqual(rows[0]["status"], "unverified_source_origin")

    def test_registry_union_must_match_extension_maps(self):
        with tempfile.TemporaryDirectory(dir="workspace/tmp") as tmp:
            catalog = Path(tmp) / "registry.json"
            source = {"repository": "example/shaders", "revision": "a" * 40,
                      "archive_sha256": "b" * 64, "source_count": 1}
            catalog.write_text(json.dumps({"complete": True, "functions": ["extra"], "types": [],
                                           "functions_by_extension": {"GL_ARB_test": []},
                                           "types_by_extension": {}, "sources": [source],
                                           "audit": {"unresolved_candidates": 0, "challenger_gaps": 0}}))
            pin = {"extension_catalog_sha256": holdout.digest(catalog), "accepted_extension_sources": [source]}
            with self.assertRaisesRegex(ValueError, "union differs"):
                holdout.load_registry(catalog, pin)
            with self.assertRaisesRegex(ValueError, "independently accepted pin"):
                holdout.load_registry(catalog, {"extension_catalog_sha256": None,
                                               "accepted_extension_sources": []})

    def test_registry_requires_zero_unresolved_evidence(self):
        with tempfile.TemporaryDirectory(dir="workspace/tmp") as tmp:
            catalog = Path(tmp) / "registry.json"
            source = {"repository": "example/shaders", "revision": "a" * 40,
                      "archive_sha256": "b" * 64, "source_count": 1}
            catalog.write_text(json.dumps({"complete": True, "functions": [], "types": [],
                                           "functions_by_extension": {}, "types_by_extension": {},
                                           "sources": [source],
                                           "audit": {"unresolved_candidates": 1, "challenger_gaps": 0}}))
            pin = {"extension_catalog_sha256": holdout.digest(catalog), "accepted_extension_sources": [source]}
            with self.assertRaisesRegex(ValueError, "unresolved completeness"):
                holdout.load_registry(catalog, pin)

    def test_exact_tenancy_paths_and_scope(self):
        receipt = {"tenancy_files": ["a.frag", "b.vert"]}
        self.assertEqual(holdout.tenancy_paths(receipt, ["a.frag", "b.vert"], ["a.frag"]), 2)
        with self.assertRaisesRegex(ValueError, "differs"):
            holdout.tenancy_paths(receipt, ["a.frag", "c.vert"], ["a.frag"])
        with self.assertRaisesRegex(ValueError, "outside"):
            holdout.tenancy_paths(receipt, ["a.frag", "b.vert"], ["c.vert"])
        with self.assertRaisesRegex(ValueError, "lacks"):
            holdout.tenancy_paths({}, [], [])

    def test_normalized_oracle_truth_is_order_independent_and_exact(self):
        first = {"scope": ["b.frag", "a.vert"],
                 "calls": [{"path": "b.frag", "from": "main", "to": "helper"}]}
        second = {"scope": ["a.vert", "b.frag"],
                  "calls": [{"to": "helper", "from": "main", "path": "b.frag"}]}
        self.assertEqual(holdout.oracle_truth_hash(first), holdout.oracle_truth_hash(second))
        second["calls"][0]["to"] = "other"
        self.assertNotEqual(holdout.oracle_truth_hash(first), holdout.oracle_truth_hash(second))

    def test_paired_claim_requires_exact_truth_subject_and_manifest(self):
        base = {"call_truth": 2, "call_truth_found": 2, "call_fathom_in_truth": 2,
                "oracle_truth_sha256": "a" * 64, "subject_sha": "b" * 40,
                "manifest_sha256": "c" * 64, "exact_tenancy_files": 3}
        scores = {name: dict(base) for name in ("baseline-core", "core-plus-types", "full-registry")}
        self.assertTrue(holdout.require_exact_pairing(scores))
        for key in ("call_truth_found", "call_fathom_in_truth", "oracle_truth_sha256",
                    "subject_sha", "manifest_sha256", "exact_tenancy_files"):
            changed = {name: dict(row) for name, row in scores.items()}
            changed["full-registry"][key] = 1 if isinstance(base[key], str) else base[key] + 1
            with self.assertRaisesRegex(ValueError, "paired holdout"):
                holdout.require_exact_pairing(changed)


if __name__ == "__main__":
    unittest.main()
