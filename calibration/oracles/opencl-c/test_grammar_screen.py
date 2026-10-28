#!/usr/bin/env python3
"""Focused tests for the frozen OpenCL C native grammar screen."""

import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
from types import SimpleNamespace
import subprocess
import sys
import tempfile
import unittest
from unittest import mock


HERE = Path(__file__).parent
ROOT = HERE.parents[2]
SPEC = importlib.util.spec_from_file_location("opencl_c_grammar_screen", HERE / "grammar_screen.py")
screen = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(screen)


class Node:
    def __init__(self, node_type="translation_unit", *, missing=False, children=()):
        self.type = node_type
        self.is_missing = missing
        self.children = list(children)


class Parser:
    def __init__(self, node):
        self.node = node

    def parse(self, _source):
        return SimpleNamespace(root_node=Node(children=[self.node]))


def evidence(root, pin, source=b"source"):
    path = root / "private-name.cl"
    public = {
        "tracked": {"manifest_sha256": hashlib.sha256(source).hexdigest()},
        "eligible": {"files": 1, "bytes": len(source)},
        "untracked": {"files": 0, "paths_sha256": hashlib.sha256(b"").hexdigest()},
    }
    return screen.triage.CheckoutEvidence(
        root, pin, b"stage", [], [path], {path: source}, b"", b"", [], public)


class GrammarScreenTests(unittest.TestCase):
    def setUp(self):
        (ROOT / "workspace" / "tmp").mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=ROOT / "workspace" / "tmp")
        self.base = Path(self.temp.name)
        self.ids = ["sdk", "darktable", "opencv", "arrayfire", "clblas", "rodinia", "shoc", "pyopencl"]
        self.policy = {
            "primary": {subject_id: f"{index + 1:040x}" for index, subject_id in enumerate(self.ids)},
            "diagnostics": {"hashcat", "pocl"},
            "sha256": screen.POOL_RULE_SHA256,
        }
        self.hashcat_pin = "f" * 40
        self.inputs = {
            "policy": self.policy,
            "hashcat_pin": self.hashcat_pin,
            "source_records": [{"path": "pins.json", "sha256": "a" * 64}],
            "source_records_sha256": "b" * 64,
        }
        self.subjects = [(subject_id, self.base / subject_id) for subject_id in self.ids]
        self.grammar = self.base / "candidate.dylib"
        self.opencl = self.base / "opencl.dylib"
        self.c = self.base / "c.dylib"
        for path, data in ((self.grammar, b"candidate"), (self.opencl, b"opencl"), (self.c, b"c")):
            path.write_bytes(data)
        self.inputs["pristine"] = {
            "opencl": {"symbol": "tree_sitter_opencl",
                       "sha256": hashlib.sha256(b"opencl").hexdigest(),
                       "candidate": "opencl-stock", "commit": "1" * 40,
                       "archive_sha256": "2" * 64, "license_sha256": "3" * 64,
                       "parser_c_sha256": "4" * 64},
            "c": {"symbol": "tree_sitter_c", "sha256": hashlib.sha256(b"c").hexdigest(),
                  "candidate": "c-stock", "commit": "5" * 40,
                  "archive_sha256": "6" * 64, "license_sha256": "7" * 64,
                  "parser_c_sha256": "8" * 64},
        }

    def tearDown(self):
        self.temp.cleanup()

    def argv(self, out, subjects=None, hashcat_grammars=None, build_receipts=(),
             grammars=None, baseline=None):
        values = []
        for subject_id, path in subjects if subjects is not None else self.subjects:
            values.extend(["--subject", f"{subject_id}={path}"])
        for spec in grammars or [f"candidate=tree_sitter_opencl={self.grammar}"]:
            values.extend(["--grammar", spec])
        if baseline is not None:
            values.extend(["--baseline", baseline])
        values.extend(["--hashcat", str(self.base / "hashcat")])
        for spec in hashcat_grammars or (
                f"opencl=tree_sitter_opencl={self.opencl}",
                f"c=tree_sitter_c={self.c}"):
            values.extend(["--hashcat-grammar", spec])
        for spec in build_receipts:
            values.extend(["--hashcat-build-receipt", spec])
        values.extend(["--out", str(out)])
        return values

    def audit_result(self):
        primary = [{"id": subject_id, "role": "primary", "files": 1,
                    "distinct_blobs": 1, "bytes": 6} for subject_id in self.ids]
        diagnostic = [{"id": "hashcat", "role": "diagnostic", "files": 1,
                       "distinct_blobs": 1, "bytes": 6}]
        return ({
            "primary": primary,
            "primary_totals": {"subjects": 8, "files": 8, "distinct_blobs": 8, "bytes": 48},
            "diagnostics": diagnostic,
            "diagnostic_totals": {"subjects": 1, "files": 1, "distinct_blobs": 1, "bytes": 6},
        }, {"subjects": []})

    def loaded(self, grammar_id, symbol, path):
        parser = Parser(Node("ERROR" if grammar_id in ("candidate", "opencl") else "node"))
        return SimpleNamespace(
            grammar_id=grammar_id, symbol=symbol, path=path.resolve(), parser=parser,
            sha256=hashlib.sha256(path.read_bytes()).hexdigest(),
            language=SimpleNamespace(abi_version=15),
        )

    def test_isolated_cli_loads_sibling_modules(self):
        result = subprocess.run(
            [sys.executable, "-I", "-B", str(HERE / "grammar_screen.py"), "--help"],
            capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("--hashcat-grammar", result.stdout)
        self.assertIn("--hashcat-build-receipt", result.stdout)

    def test_aggregates_every_primary_and_required_hashcat_diagnostic_privately(self):
        captures = []
        for subject_id, root in self.subjects:
            item = evidence(root.resolve(), self.policy["primary"][subject_id])
            captures.extend([item, item])
        hashcat = evidence((self.base / "hashcat").resolve(), self.hashcat_pin)
        captures.extend([hashcat, hashcat])
        out = self.base / "out"

        with mock.patch.object(screen, "_load_fixed_inputs", return_value=self.inputs), \
                mock.patch.object(screen.pool_audit, "audit", return_value=self.audit_result()), \
                mock.patch.object(screen.triage, "capture_checkout", side_effect=captures), \
                mock.patch.object(screen.triage, "load_grammar", side_effect=self.loaded), \
                mock.patch.object(screen.importlib.metadata, "version", return_value=screen.triage.TREE_SITTER_VERSION), \
                contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(screen.main(self.argv(out)), 0)

        public = json.loads((out / "summary.json").read_text())
        private = (out / "private" / "evidence.json").read_text()
        self.assertEqual([row["id"] for row in public["primary"]], sorted(self.ids))
        pooled = public["primary_totals"]["grammars"][0]
        self.assertEqual(pooled["population_count"], 8)
        self.assertEqual(pooled["error_or_missing_count"], 8)
        self.assertEqual(public["diagnostic"]["id"], "hashcat")
        self.assertEqual([row["id"] for row in public["diagnostic"]["grammars"]], ["c", "opencl"])
        self.assertEqual(public["triage_facts"]["fault_count"], 0)
        self.assertIn("wrapper_sha256", public["instrument"])
        self.assertIn("configuration_sha256", public["instrument"])
        public_text = json.dumps(public, sort_keys=True)
        self.assertNotIn(str(self.base), public_text)
        self.assertNotIn(self.hashcat_pin, public_text)
        self.assertNotIn("private-name.cl", public_text)
        self.assertIn(str(self.base.resolve()), private)

    def test_requires_exact_primary_ids_and_rejects_diagnostics_and_duplicates(self):
        cases = (
            self.subjects[:-1],
            self.subjects + [("hashcat", self.base / "other")],
            self.subjects + [("sdk", self.base / "duplicate")],
        )
        for index, subjects in enumerate(cases):
            out = self.base / f"bad-{index}"
            with self.subTest(index=index), \
                    mock.patch.object(screen, "_load_fixed_inputs", return_value=self.inputs), \
                    mock.patch.object(screen.pool_audit, "audit") as audit:
                with self.assertRaises(SystemExit):
                    screen.main(self.argv(out, subjects=subjects))
                audit.assert_not_called()
                self.assertFalse(out.exists())

    def test_requires_exact_pristine_hashcat_grammar_ids_and_symbols(self):
        invalid = (
            [f"opencl=tree_sitter_opencl={self.opencl}"],
            [f"opencl=tree_sitter_opencl={self.opencl}", f"opencl=tree_sitter_opencl={self.opencl}"],
            [f"opencl=wrong={self.opencl}", f"c=tree_sitter_c={self.c}"],
        )
        for index, specs in enumerate(invalid):
            with self.subTest(index=index), mock.patch.object(
                    screen, "_load_fixed_inputs", return_value=self.inputs):
                with self.assertRaises(SystemExit):
                    screen.main(self.argv(self.base / f"grammar-{index}", hashcat_grammars=specs))

    def test_rejects_duplicate_candidate_grammar_ids_before_audit(self):
        argv = self.argv(self.base / "duplicate-candidate")
        argv[argv.index("--hashcat"):argv.index("--hashcat")] = [
            "--grammar", f"candidate=tree_sitter_opencl={self.grammar}"]
        with mock.patch.object(screen, "_load_fixed_inputs", return_value=self.inputs), \
                mock.patch.object(screen.pool_audit, "audit") as audit:
            with self.assertRaises(SystemExit):
                screen.main(argv)
            audit.assert_not_called()

    def test_output_must_be_fresh_before_any_checkout_is_consumed(self):
        out = self.base / "occupied"
        out.mkdir()
        (out / "keep").write_text("existing")
        with mock.patch.object(screen, "_load_fixed_inputs", return_value=self.inputs), \
                mock.patch.object(screen.pool_audit, "audit") as audit:
            with self.assertRaises(SystemExit):
                screen.main(self.argv(out))
            audit.assert_not_called()
        self.assertEqual((out / "keep").read_text(), "existing")

    def test_rejects_nonpristine_hashcat_library_before_screening(self):
        out = self.base / "wrong-library"
        fixed = {**self.inputs, "pristine": {
            **self.inputs["pristine"],
            "opencl": {"symbol": "tree_sitter_opencl", "sha256": "0" * 64},
        }}
        with mock.patch.object(screen, "_load_fixed_inputs", return_value=fixed), \
                mock.patch.object(screen.pool_audit, "audit", return_value=self.audit_result()), \
                mock.patch.object(screen.triage, "load_grammar") as load, \
                mock.patch.object(screen.triage, "capture_checkout") as capture, \
                mock.patch.object(screen.importlib.metadata, "version", return_value=screen.triage.TREE_SITTER_VERSION):
            with self.assertRaisesRegex(ValueError, "not the pristine pinned library"):
                screen.main(self.argv(out))
            load.assert_not_called()
            capture.assert_not_called()
        self.assertFalse(out.exists())

    def build_receipt(self, grammar_id, native_bytes):
        pin = self.inputs["pristine"][grammar_id]
        return {
            "schema": "opencl-native-grammar-build-v1",
            "scope": "calibration-only native grammar build; no product WASM",
            "candidate": {"id": pin["candidate"], "repository": "private",
                          "version": "fixed", "commit": pin["commit"],
                          "entry_point": pin["symbol"], "generate": False,
                          "scanner": False},
            "hashes": {"candidate_config_sha256": "9" * 64,
                       "archive_sha256": pin["archive_sha256"],
                       "license_sha256": pin["license_sha256"],
                       "original_parser_c_sha256": pin["parser_c_sha256"],
                       "patches_sha256": hashlib.sha256(b"[]").hexdigest(),
                       "parser_c_sha256": pin["parser_c_sha256"],
                       "native_sha256": hashlib.sha256(native_bytes).hexdigest(),
                       "tool_versions_sha256": "a" * 64},
            "patches": [], "tool_versions": {}, "test": None,
        }

    def test_accepts_stock_builder_receipt_and_rejects_generation_or_patches(self):
        receipt_path = self.base / "opencl-receipt.json"
        receipt = self.build_receipt("opencl", b"fresh-opencl")
        receipt_path.write_text(json.dumps(receipt))
        evidence = screen._validate_build_receipt(
            "opencl", receipt_path, b"fresh-opencl", self.inputs["pristine"]["opencl"])
        self.assertEqual(evidence["native_sha256"], hashlib.sha256(b"fresh-opencl").hexdigest())
        self.assertEqual(evidence["receipt_sha256"], hashlib.sha256(receipt_path.read_bytes()).hexdigest())

        receipt["candidate"]["generate"] = True
        receipt_path.write_text(json.dumps(receipt))
        with self.assertRaisesRegex(ValueError, "stock build"):
            screen._validate_build_receipt(
                "opencl", receipt_path, b"fresh-opencl", self.inputs["pristine"]["opencl"])
        receipt["candidate"]["generate"] = False
        receipt["patches"] = [{"path": "patch", "sha256": "b" * 64}]
        receipt_path.write_text(json.dumps(receipt))
        with self.assertRaisesRegex(ValueError, "stock build"):
            screen._validate_build_receipt(
                "opencl", receipt_path, b"fresh-opencl", self.inputs["pristine"]["opencl"])

    def test_builder_receipt_must_match_frozen_archive_and_native_bytes(self):
        path = self.base / "receipt.json"
        receipt = self.build_receipt("c", b"fresh-c")
        receipt["hashes"]["archive_sha256"] = "0" * 64
        path.write_text(json.dumps(receipt))
        with self.assertRaisesRegex(ValueError, "frozen pins"):
            screen._validate_build_receipt("c", path, b"fresh-c", self.inputs["pristine"]["c"])
        receipt = self.build_receipt("c", b"other")
        path.write_text(json.dumps(receipt))
        with self.assertRaisesRegex(ValueError, "native bytes"):
            screen._validate_build_receipt("c", path, b"fresh-c", self.inputs["pristine"]["c"])

    def test_stages_captured_library_bytes_before_loading(self):
        stage = self.base / "stage"
        original = self.grammar.read_bytes()

        def load(grammar_id, symbol, path):
            self.grammar.write_bytes(b"mutated-after-capture")
            self.assertEqual(path.read_bytes(), original)
            return self.loaded(grammar_id, symbol, path)

        with mock.patch.object(screen.triage, "load_grammar", side_effect=load):
            loaded = screen._stage_and_load(
                stage, [("candidate", "tree_sitter_opencl", self.grammar, original)], "primary")
        self.assertEqual(loaded[0].sha256, hashlib.sha256(original).hexdigest())
        self.assertNotEqual(loaded[0].path, self.grammar.resolve())

    def test_requires_explicit_present_baseline_for_multiple_grammars(self):
        grammars = [
            ("before", "tree_sitter_opencl", self.grammar),
            ("after", "tree_sitter_opencl", self.grammar),
        ]
        with self.assertRaises(SystemExit):
            screen._validate_specs(
                screen.argparse.ArgumentParser(), self.policy, self.subjects,
                grammars, [("opencl", "tree_sitter_opencl", self.opencl),
                           ("c", "tree_sitter_c", self.c)], [], None)
        with self.assertRaises(SystemExit):
            screen._validate_specs(
                screen.argparse.ArgumentParser(), self.policy, self.subjects,
                grammars, [("opencl", "tree_sitter_opencl", self.opencl),
                           ("c", "tree_sitter_c", self.c)], [], "missing")

    def test_transition_counts_detect_inverse_regression_with_equal_totals(self):
        baseline = [
            {"path": "first.cl", "parsed": True, "error": False, "missing": False, "fault": None},
            {"path": "second.cl", "parsed": True, "error": True, "missing": False, "fault": None},
        ]
        candidate = [
            {"path": "first.cl", "parsed": True, "error": True, "missing": False, "fault": None},
            {"path": "second.cl", "parsed": True, "error": False, "missing": False, "fault": None},
        ]
        public, private = screen._transition_summary("before", "after", baseline, candidate)
        self.assertEqual(public["clean_to_error_count"], 1)
        self.assertEqual(public["error_to_clean_count"], 1)
        self.assertEqual(public["unchanged_count"], 0)
        self.assertNotIn("first.cl", json.dumps(public))
        self.assertIn("first.cl", json.dumps(private))
        pooled = screen._pooled_transitions([
            {"transitions": [public]}, {"transitions": [public]},
        ])[0]
        self.assertEqual(pooled["population_count"], 4)
        self.assertEqual(pooled["clean_to_error_count"], 2)
        self.assertEqual(pooled["error_to_clean_count"], 2)

    def test_policy_source_record_hashes_are_verified(self):
        root = self.base / "repo"
        source = root / "record.json"
        source.parent.mkdir()
        source.write_bytes(b"fixed")
        policy = {"source_records": [{
            "path": "record.json", "sha256": hashlib.sha256(b"fixed").hexdigest()}]}
        rows, digest = screen._verify_source_records(policy, root)
        self.assertEqual(rows, policy["source_records"])
        self.assertEqual(len(digest), 64)
        source.write_bytes(b"changed")
        with self.assertRaisesRegex(ValueError, "source-record hash"):
            screen._verify_source_records(policy, root)

    def test_rejects_wrong_frozen_policy_digest(self):
        with mock.patch.object(screen, "_sha256", return_value="0" * 64):
            with self.assertRaisesRegex(ValueError, "pool-rule bytes"):
                screen._load_fixed_inputs()

    def test_pin_mismatch_from_pool_audit_aborts_without_output(self):
        out = self.base / "mismatch"
        with mock.patch.object(screen, "_load_fixed_inputs", return_value=self.inputs), \
                mock.patch.object(screen.pool_audit, "audit", side_effect=ValueError("HEAD does not match")), \
                mock.patch.object(screen.importlib.metadata, "version", return_value=screen.triage.TREE_SITTER_VERSION):
            with self.assertRaisesRegex(ValueError, "HEAD does not match"):
                screen.main(self.argv(out))
        self.assertFalse(out.exists())


if __name__ == "__main__":
    unittest.main()
