#!/usr/bin/env python3
"""Focused tests for the public OpenCL residual subset receipt."""

import importlib.util
import copy
import json
from pathlib import Path
import unittest


SPEC = importlib.util.spec_from_file_location(
    "residual_audit", Path(__file__).with_name("residual_audit.py"))
audit = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(audit)


class Node:
    def __init__(self, kind, start, line, children=(), missing=False):
        self.type = kind
        self.start_byte = start
        self.start_point = (line - 1, 0)
        self.children = children
        self.is_missing = missing


class ResidualAuditTest(unittest.TestCase):
    def test_first_diagnostic_uses_earliest_byte_not_walk_order(self):
        root = Node("translation_unit", 0, 1, (
            Node("outer", 0, 1, (Node("ERROR", 40, 5),)),
            Node("identifier", 12, 2, missing=True),
        ))
        self.assertEqual(audit.first_diagnostic(root),
                         {"kind": "MISSING identifier", "line": 2})

    def test_first_diagnostic_rejects_clean_tree(self):
        with self.assertRaisesRegex(ValueError, "no ERROR or MISSING"):
            audit.first_diagnostic(Node("translation_unit", 0, 1))

    def test_fresh_screen_accepts_binary_identity_only(self):
        baseline = json.loads(audit.PUBLIC_SCREEN.read_text())
        fresh = copy.deepcopy(baseline)
        fresh["instrument"]["configuration_sha256"] = "1" * 64
        fresh["primary_totals"]["grammars"][0]["library_sha256"] = "2" * 64
        fresh["primary"][0]["grammars"][0]["library_sha256"] = "2" * 64
        audit.verify_screen_summary(fresh, baseline)
        fresh["primary"][0]["grammars"][0]["error_or_missing_count"] += 1
        with self.assertRaisesRegex(ValueError, "counts"):
            audit.verify_screen_summary(fresh, baseline)
        fresh = copy.deepcopy(baseline)
        fresh["primary"][0]["id"] = "wrong-pin-subject"
        with self.assertRaisesRegex(ValueError, "population"):
            audit.verify_screen_summary(fresh, baseline)

    def test_fresh_build_requires_same_parser_and_patches(self):
        builds = json.loads(audit.PUBLIC_BUILDS.read_text())
        old = next(row for row in builds["candidates"]
                   if row["candidate"]["id"] == audit.BUILD_ID)
        fresh = copy.deepcopy(old)
        fresh["hashes"]["native_sha256"] = "a" * 64
        self.assertEqual(audit.verify_build_receipt(fresh, builds, "a" * 64),
                         old["hashes"]["parser_c_sha256"])
        with self.assertRaisesRegex(ValueError, "native SHA"):
            audit.verify_build_receipt(fresh, builds, "b" * 64)
        fresh["hashes"]["parser_c_sha256"] = "b" * 64
        with self.assertRaisesRegex(ValueError, "parser_c_sha256"):
            audit.verify_build_receipt(fresh, builds, "a" * 64)
        fresh = copy.deepcopy(old)
        fresh["patches"][0]["sha256"] = "b" * 64
        with self.assertRaisesRegex(ValueError, "patches"):
            audit.verify_build_receipt(fresh, builds, old["hashes"]["native_sha256"])

    def test_fresh_residuals_must_match_frozen_paths_and_sites(self):
        frozen = json.loads(audit.BASELINE_LABELS.read_text())
        fresh = copy.deepcopy(frozen["rows"])
        audit.verify_residual_subset(fresh, frozen)
        fresh[0]["path"] = "other.cl"
        with self.assertRaisesRegex(ValueError, "paths"):
            audit.verify_residual_subset(fresh, frozen)
        fresh = copy.deepcopy(frozen["rows"])
        fresh[0]["first"]["line"] += 1
        with self.assertRaisesRegex(ValueError, "diagnostics"):
            audit.verify_residual_subset(fresh, frozen)
        fresh = copy.deepcopy(frozen["rows"])
        fresh[0]["subject_pin"] = "0" * 40
        with self.assertRaisesRegex(ValueError, "pins"):
            audit.verify_residual_subset(fresh, frozen)

    def test_fresh_subject_pins_match_frozen_policy(self):
        policy = json.loads(audit.POOL_RULE.read_text())
        configuration = {"subjects": [{"id": row["id"], "pin": row["pin"]}
                                      for row in policy["primary_subjects"]]}
        audit.verify_subject_pins(configuration, policy)
        configuration["subjects"][0]["pin"] = "0" * 40
        with self.assertRaisesRegex(ValueError, "pins"):
            audit.verify_subject_pins(configuration, policy)

    def test_class_labels_are_explicit_and_stable(self):
        self.assertEqual(audit.classify("clblas", "x.cl"), "host_string_template")
        self.assertEqual(audit.classify("opencv", "modules/core/src/opencl/copymakeborder.cl"),
                         "semicolonless_macro_invocation")
        self.assertEqual(audit.classify("rodinia", "opencl/lavaMD/kernel/kernel_gpu_opencl.cl"),
                         "conditional_cpp_linkage")
        self.assertEqual(audit.classify("opencv", "modules/core/src/opencl/arithm.cl"),
                         "macro_parameter_fragment")
        self.assertEqual(audit.classify("opencv", "modules/core/src/opencl/inrange.cl"),
                         "conditional_branch_fragment")
        self.assertEqual(audit.classify("opencv", "modules/dnn/src/opencl/mvn.cl"),
                         "raw_missing_semicolon")
        self.assertEqual(audit.classify("pyopencl", "pyopencl/cl/pyopencl-random123/philox.cl"),
                         "macro_generated_syntax")
        with self.assertRaisesRegex(ValueError, "unknown residual"):
            audit.classify("opencv", "new-file.cl")

    def test_committed_receipt_is_exact_bounded_subset(self):
        path = (audit.ROOT / "docs/assessments/data/opencl-c-grammar-gate-2026-10-26"
                / "residual-labels.json")
        manifest = json.loads(path.read_text())
        self.assertEqual(manifest["count"], 63)
        self.assertEqual(manifest["reproduce"], audit.REPRODUCE)
        self.assertEqual(manifest["baseline_public_screen_sha256"],
                         audit.sha256(audit.PUBLIC_SCREEN.read_bytes()))
        self.assertEqual(len(manifest["rows"]), 63)
        self.assertEqual(sum(manifest["class_counts"].values()), 63)
        self.assertEqual(manifest["public_screen_sha256"],
                         audit.sha256(audit.PUBLIC_SCREEN.read_bytes()))
        screen = json.loads(audit.PUBLIC_SCREEN.read_text())
        grammar = next(row for row in screen["primary_totals"]["grammars"]
                       if row["id"] == audit.GRAMMAR_ID)
        self.assertEqual(manifest["native_grammar_sha256"], grammar["library_sha256"])
        builds = json.loads(audit.PUBLIC_BUILDS.read_text())
        pinned = next(row for row in builds["candidates"]
                      if row["candidate"]["id"] == audit.BUILD_ID)
        self.assertEqual(manifest["parser_c_sha256"], pinned["hashes"]["parser_c_sha256"])
        self.assertEqual(manifest["class_counts"]["host_string_template"], 40)
        self.assertEqual(manifest["class_counts"]["semicolonless_macro_invocation"], 10)
        for row in manifest["rows"]:
            self.assertEqual(row["class"], audit.classify(row["subject"], row["path"]))
            self.assertGreater(row["first"]["line"], 0)
            self.assertFalse(row["path"].startswith("/"))


if __name__ == "__main__":
    unittest.main()
