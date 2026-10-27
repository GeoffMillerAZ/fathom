import importlib.util
import hashlib
import json
import subprocess
import sys
import tempfile
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("catalog_deltas", Path(__file__).with_name("catalog_deltas.py"))
delta = importlib.util.module_from_spec(spec)
spec.loader.exec_module(delta)


def reading(predicted):
    return {"fathom_commit": "a" * 40, "subjects": [{"name": "sample", "sha": "b" * 40, "files": 2}],
            "counts": {"call_truth": 1, "call_truth_found": 1, "call_fathom_in_truth": 1,
                       "call_fathom": predicted}}


def receipt(names, paths=None):
    return {"tenancy_files": paths or ["a.frag", "empty.frag"],
            "calls": {"fathom_only": [{"path": "a.frag", "caller": "main", "callee": n} for n in names],
                      "oracle_only": []}}


class CatalogDeltaTests(unittest.TestCase):
    def test_holdout_evidence_binds_readings_receipts_and_manifest(self):
        with tempfile.TemporaryDirectory(dir="workspace/tmp") as tmp:
            root = Path(tmp)
            old, new = reading(2), reading(1)
            rows = [{"path": "a.frag", "extension": ".frag", "sha256": "c" * 64, "bytes": 10},
                    {"path": "empty.frag", "extension": ".frag", "sha256": "d" * 64, "bytes": 12}]
            manifest = hashlib.sha256(json.dumps(rows, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
            files = {"old.json": old, "new.json": new, "old-u.json": receipt(["extFn"]),
                     "new-u.json": receipt([]), "receipt.json": {"files": rows}}
            for name, data in files.items():
                (root / name).write_text(json.dumps(data))
            filehash = lambda name: hashlib.sha256((root / name).read_bytes()).hexdigest()
            modes = {}
            for mode, prefix in (("baseline-core", "old"), ("full-registry", "new")):
                modes[mode] = {"catalog_mode": mode, "catalog_fingerprint_sha256": mode,
                    "score": {"reading_sha256": filehash(prefix + ".json"),
                              "unmatched_sha256": filehash(prefix + "-u.json"),
                              "subject_sha": "b" * 40, "manifest_sha256": manifest,
                              "oracle_truth_sha256": "e" * 64}}
            summary = {"paired_comparison_claimable": True, "product_commit": "a" * 40,
                       "holdout_commit": "b" * 40, "archive_sha256": "f" * 64,
                       "manifest_sha256": manifest, "extension_catalog_sha256": "1" * 64,
                       "modes": modes}
            hashes = {key: str(index) * 64 for index, key in enumerate(
                ("baseline_catalog_sha256", "core_catalog_sha256", "baseline_constructors_sha256"), 2)}
            summary.update(hashes)
            (root / "summary.json").write_text(json.dumps(summary))
            config = {"before_reading": str(root / "old.json"), "after_reading": str(root / "new.json"),
                      "pairs": [{"subject": "sample", "before_unmatched": str(root / "old-u.json"),
                                 "after_unmatched": str(root / "new-u.json")}],
                      "evidence": {"kind": "holdout", "summary": str(root / "summary.json"),
                                   "oracle_receipt": str(root / "receipt.json")}}
            pin = {"commit_sha": "b" * 40, "archive_sha256": "f" * 64, **hashes}
            fingerprints = {mode: mode for mode in modes}
            delta.validate_pair_evidence(config, pin, "1" * 64, fingerprints)
            with self.assertRaisesRegex(ValueError, "invalid paired catalog modes"):
                delta.validate_pair_evidence({**config, "before_mode": "full-registry",
                                              "after_mode": "baseline-core"}, pin, "1" * 64, fingerprints)
            with self.assertRaisesRegex(ValueError, "source catalog hash"):
                delta.validate_pair_evidence(config, {**pin, "core_catalog_sha256": "0" * 64}, "1" * 64, fingerprints)
            (root / "new.json").write_text(json.dumps(reading(9)))
            with self.assertRaisesRegex(ValueError, "reading hash"):
                delta.validate_pair_evidence(config, pin, "1" * 64, fingerprints)
            (root / "new.json").write_text(json.dumps(new))
            rows[1]["sha256"] = "0" * 64
            (root / "receipt.json").write_text(json.dumps({"files": rows}))
            with self.assertRaisesRegex(ValueError, "manifest"):
                delta.validate_pair_evidence(config, pin, "1" * 64, fingerprints)

    def test_precision_and_nonadditive_family_attribution(self):
        pairs = [("sample", receipt(["extFn", "newType", "macro"]), receipt(["macro"]))]
        out = delta.compare(reading(4), reading(2), pairs, {"newType"}, set(),
                            {"GL_EXT_first": ["extFn"], "GL_ARB_second": ["extFn"]}, {})
        self.assertEqual(out["before"]["precision"], 0.25)
        self.assertEqual(out["after"]["precision"], 0.5)
        self.assertEqual(out["removed_by_kind"], {"core_type": 1, "extension_function": 1})
        self.assertEqual(out["extension_families"], {"GL_ARB_second": 1, "GL_EXT_first": 1})
        self.assertEqual(out["removed_pairs"], 2)

    def test_cli_reads_per_extension_maps_after_validating_catalog(self):
        with tempfile.TemporaryDirectory(dir="workspace/tmp") as tmp:
            root = Path(tmp)
            here = Path(__file__).parent
            source = {"repository": "KhronosGroup/GLSL", "revision": "a" * 40,
                      "archive_sha256": "b" * 64, "source_count": 1}
            catalog = {"complete": True, "sources": [source],
                       "audit": {"unresolved_candidates": 0, "challenger_gaps": 0},
                       "functions": ["extFn"], "types": [],
                       "functions_by_extension": {"GL_EXT_first": ["extFn"]}, "types_by_extension": {}}
            files = {"before.json": reading(3), "after.json": reading(2),
                     "old.json": receipt(["extFn", "macro"]), "new.json": receipt(["macro"]),
                     "catalog.json": catalog}
            for name, data in files.items():
                (root / name).write_text(json.dumps(data))
            pin = json.loads((here / "holdout-pin.json").read_text())
            pin["extension_catalog_sha256"] = hashlib.sha256((root / "catalog.json").read_bytes()).hexdigest()
            pin["accepted_extension_sources"] = [source]
            (root / "pin.json").write_text(json.dumps(pin))
            config = {"before_reading": str(root / "before.json"), "after_reading": str(root / "after.json"),
                      "catalog": str(root / "catalog.json"), "pin": str(root / "pin.json"),
                      "pairs": [{"subject": "sample", "before_unmatched": str(root / "old.json"),
                                 "after_unmatched": str(root / "new.json")}]}
            adapter_spec = importlib.util.spec_from_file_location("glsl_ast_test", here / "glslang_ast.py")
            adapter = importlib.util.module_from_spec(adapter_spec)
            adapter_spec.loader.exec_module(adapter)
            core = json.loads((here / "glsl460-functions.json").read_text())
            oracle = {"calls": [], "scope": ["a.frag", "empty.frag"]}
            (root / "oracle.json").write_text(json.dumps(oracle))
            truth_hash = hashlib.sha256(json.dumps(oracle, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
            config["evidence"] = {"kind": "canonical"}
            for side, mode, reading_name, unmatched_name in (
                    ("before", "baseline-core", "before.json", "old.json"),
                    ("after", "full-registry", "after.json", "new.json")):
                functions = set(core["functions"]) | ({"extFn"} if side == "after" else set())
                types = set(core["types"]) if side == "after" else adapter.legacy_constructor_names()
                filehash = lambda name: hashlib.sha256((root / name).read_bytes()).hexdigest()
                evidence = {"product_commit": "a" * 40, "catalog_mode": mode,
                            "catalog_fingerprint_sha256": adapter._catalog_fingerprint(mode, functions, types),
                            "extension_catalog_sha256": pin["extension_catalog_sha256"] if side == "after" else None,
                            "reading_sha256": filehash(reading_name),
                            "subjects": [{"name": "sample", "sha": "b" * 40, "files": 2,
                                "shader_tree_sha256": "c" * 64, "unmatched_sha256": filehash(unmatched_name),
                                "oracle_path": str(root / "oracle.json"), "oracle_sha256": filehash("oracle.json"),
                                "oracle_truth_sha256": truth_hash}]}
                evidence_path = root / (side + "-evidence.json")
                evidence_path.write_text(json.dumps(evidence))
                config["evidence"][side] = str(evidence_path)
            (root / "config.json").write_text(json.dumps(config))
            run = subprocess.run([sys.executable, "-I", "-B", str(here / "catalog_deltas.py"),
                                  str(root / "config.json"), str(root / "out.json")], capture_output=True, text=True)
            self.assertEqual(run.returncode, 0, run.stderr)
            report = json.loads((root / "out.json").read_text())
            self.assertEqual(report["extension_families"], {"GL_EXT_first": 1})

    def test_shared_function_and_type_name_keeps_kind_ambiguity(self):
        pairs = [("sample", receipt(["sharedName"]), receipt([]))]
        out = delta.compare(reading(2), reading(1), pairs, set(), set(),
                            {"GL_EXT_both": ["sharedName"]}, {"GL_EXT_both": ["sharedName"]})
        self.assertEqual(out["removed_by_kind"], {"extension_function_or_type": 1})
        self.assertEqual(out["extension_families"], {"GL_EXT_both": 1})

    def test_rejects_same_count_different_tenant_paths(self):
        pairs = [("sample", receipt(["extra"]), receipt(["extra"], ["b.frag", "empty.frag"]))]
        with self.assertRaisesRegex(ValueError, "tenancy"):
            delta.compare(reading(2), reading(2), pairs, set(), set(), {}, {})

    def test_rejects_truth_loss_or_unexplained_removed_pair(self):
        pairs = [("sample", receipt(["ordinary"]), receipt([]))]
        with self.assertRaisesRegex(ValueError, "unexplained"):
            delta.compare(reading(2), reading(1), pairs, set(), set(), {}, {})
        after = reading(1)
        after["counts"]["call_truth"] = 0
        with self.assertRaisesRegex(ValueError, "truth"):
            delta.compare(reading(2), after, pairs, set(), set(), {}, {})


if __name__ == "__main__":
    unittest.main()
