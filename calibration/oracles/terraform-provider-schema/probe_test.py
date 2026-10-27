#!/usr/bin/env python3
"""Offline checks for refusal paths and authored-control accounting."""

import hashlib
import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


MODULE = Path(__file__).with_name("probe.py")
spec = importlib.util.spec_from_file_location("provider_schema_probe", MODULE)
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)
WORKSPACE_TMP = Path(__file__).resolve().parents[3] / "workspace" / "tmp"


class ProbeTests(unittest.TestCase):
    def test_authored_population_matches_independent_protocol_hashes(self):
        pins = json.loads(MODULE.with_name("pins.json").read_text())
        protocol = json.loads(MODULE.with_name("protocol.json").read_text())
        hashes = {name: hashlib.sha256(probe.fixture(name, pins["providers"][name]["version"]).encode()).hexdigest()
                  for name in probe.NAMES}
        self.assertEqual(hashes, protocol["fixture_sha256"])

    def setUp(self):
        WORKSPACE_TMP.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=WORKSPACE_TMP)
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        tool = root / "tofu"
        archive = root / "archive.zip"
        tool.write_bytes(b"tool")
        archive.write_bytes(b"archive")
        tool_sha = hashlib.sha256(b"tool").hexdigest()
        archive_sha = hashlib.sha256(b"archive").hexdigest()
        self.manifest = {
            "platform": probe.PLATFORM,
            "tool": {"path": str(tool), "sha256": tool_sha},
            "providers": [{"name": name, "version": "1.2.3", "archive": str(archive),
                           "archive_sha256": archive_sha, "binary_sha256": "a" * 64}
                          for name in probe.NAMES],
        }
        self.pins = {"tool": {"version": "1.13.0", "sha256": tool_sha},
                     "providers": {name: {"version": "1.2.3", "archive_sha256": archive_sha,
                                          "binary_sha256": "a" * 64} for name in probe.NAMES}}
        self.baseline_pins = {"opentofu": {"version": "1.13.0"},
                              "providers": {"aws": {"version": "1.2.3", "sha256": archive_sha}}}

    def test_manifest_requires_exact_three_unique_providers_and_hashes(self):
        self.assertEqual(set(probe.check_manifest(self.manifest, self.pins, self.baseline_pins)), set(probe.NAMES))
        self.manifest["providers"][2]["name"] = "aws"
        with self.assertRaisesRegex(probe.ProbeError, "provider_set"):
            probe.check_manifest(self.manifest, self.pins, self.baseline_pins)
        self.manifest["providers"][2]["name"] = "google"
        self.manifest["providers"][0]["archive_sha256"] = "0" * 64
        with self.assertRaisesRegex(probe.ProbeError, "artifact_hash"):
            probe.check_manifest(self.manifest, self.pins, self.baseline_pins)

    def test_independent_pins_reject_matching_file_hash_with_wrong_identity(self):
        tool = Path(self.manifest["tool"]["path"])
        tool.write_bytes(b"another tool")
        self.manifest["tool"]["sha256"] = hashlib.sha256(tool.read_bytes()).hexdigest()
        with self.assertRaisesRegex(probe.ProbeError, "tool_pin"):
            probe.check_manifest(self.manifest, self.pins, self.baseline_pins)
        self.manifest["tool"]["sha256"] = self.pins["tool"]["sha256"]
        tool.write_bytes(b"tool")
        azure = next(p for p in self.manifest["providers"] if p["name"] == "azurerm")
        azure["version"] = "9.9.9"
        with self.assertRaisesRegex(probe.ProbeError, "provider_pin"):
            probe.check_manifest(self.manifest, self.pins, self.baseline_pins)
        azure["version"] = "1.2.3"
        azure["binary_sha256"] = "b" * 64
        with self.assertRaisesRegex(probe.ProbeError, "provider_pin"):
            probe.check_manifest(self.manifest, self.pins, self.baseline_pins)

    def test_authored_outputs_and_individual_mutations(self):
        for name in probe.NAMES:
            source = probe.fixture(name, "1.2.3")
            self.assertEqual(source.count('output "o'), 4)
            self.assertEqual(source.count('alias = "secondary"'), 1)
            self.assertEqual(source.count("provider = " + name + ".secondary"), 1)
            for i in range(4):
                changed = probe.mutate(name, i, source)
                self.assertEqual(changed.count(probe.SENTINEL), 1)
                self.assertEqual(changed.count('output "o'), 4)

    def test_schema_counts_and_native_extras(self):
        schema = {"provider_schemas": {"registry.opentofu.org/hashicorp/aws": {
            "provider": {"block": {"attributes": {"region": {"type": "string"}}}},
            "resource_schemas": {"aws_vpc": {"block": {
                "attributes": {"tags": {"type": ["map", "string"]}},
                "block_types": {"nested": {"block": {"attributes": {"id": {"type": "string"}}}}}}}},
            "data_source_schemas": {"aws_region": {"block": {"attributes": {"name": {"type": "string"}}}}}
        }}}
        result = probe.summarize_schema(json.dumps(schema).encode())
        counts = next(iter(result["namespaces"].values()))
        self.assertEqual((counts["section_counts"]["resource_schemas"], counts["section_counts"]["data_source_schemas"], counts["explicit_attribute_declarations_in_examined_blocks"]), (1, 1, 4))
        self.assertEqual(counts["map_declarations_in_examined_blocks"], 1)
        outputs = {"o%d" % i: {"expression": {"references": [expr, "extra"]}}
                   for i, expr in enumerate(probe.FIXTURES["aws"]["expressions"])}
        show = {"root_module": {"outputs": outputs, "resources": []}, "provider_config": {}}
        observed = probe.summarize_show(json.dumps(show).encode(), "aws")
        self.assertEqual(observed["authored_full_path_count"], 4)
        self.assertEqual(observed["native_extra_reference_count"], 4)

    def test_show_unwrapped_native_shape_counts_exact_map_and_prefix_references(self):
        expressions = probe.FIXTURES["aws"]["expressions"]
        outputs = {"o%d" % i: {"expression": {"references": [expr, expr.rsplit(".", 1)[0]]}}
                   for i, expr in enumerate(expressions[:3])}
        outputs["o3"] = {"expression": {"references": [expressions[3],
                         "aws_vpc.probe.tags", "aws_vpc.probe"]}}
        show = {"root_module": {"outputs": outputs, "resources": [
                    {"provider_config_key": "aws.secondary"},
                    {"provider_config_key": "aws"},
                    {"provider_config_key": "aws"}]},
                "provider_config": {"aws": {}, "aws.secondary": {"alias": "secondary"}}}
        observed = probe.summarize_show(json.dumps(show).encode(), "aws")
        self.assertEqual(observed["native_reference_count"], 9)
        self.assertEqual(observed["authored_full_path_count"], 4)
        self.assertEqual(observed["native_extra_reference_count"], 5)
        self.assertEqual(observed["provider_config_count"], 2)
        self.assertEqual(observed["aliased_provider_config_count"], 1)
        self.assertEqual(observed["resource_provider_mapping_count"], 3)
        self.assertEqual(observed["aliased_resource_mapping_count"], 1)

    def test_mutation_requires_named_attribute_error_at_source_range(self):
        diagnostic = {"severity": "error", "summary": "Unsupported attribute",
                      "detail": "This object has no attribute named " + probe.SENTINEL,
                      "range": {"filename": "main.tf", "start": {"line": 20}}}
        data = {"valid": False, "error_count": 1, "warning_count": 0,
                "diagnostics": [diagnostic]}
        result = probe.summarize_validation(data, 1, "a" * 64)
        self.assertTrue(probe.mutation_is_attributed(result))
        diagnostic["detail"] = "provider authentication failed"
        self.assertFalse(probe.mutation_is_attributed(probe.summarize_validation(data, 1, "a" * 64)))
        diagnostic["detail"] = "This object has no attribute named " + probe.SENTINEL
        diagnostic["range"]["filename"] = "other.tf"
        self.assertFalse(probe.mutation_is_attributed(probe.summarize_validation(data, 1, "a" * 64)))

    def test_validate_canonical_hash_ignores_order_but_not_content(self):
        error = {"severity": "error", "summary": "Unsupported attribute", "detail": "first",
                 "range": {"filename": "main.tf", "start": {"line": 12, "column": 4}}}
        warning = {"severity": "warning", "summary": "Deprecated", "detail": "second",
                   "range": {"filename": "main.tf", "start": {"line": 2, "column": 1}}}
        data = {"valid": False, "error_count": 1, "warning_count": 1,
                "diagnostics": [warning, error]}
        reordered = {**data, "diagnostics": [error, warning]}
        left = probe.summarize_validation(data, 1, "a" * 64)
        right = probe.summarize_validation(reordered, 1, "b" * 64)
        self.assertEqual(left["canonical_sha256"], right["canonical_sha256"])
        self.assertNotEqual(left["raw_sha256"], right["raw_sha256"])
        self.assertEqual(left["canonical_sha256"],
                         probe.canonical_validation_sha256(json.dumps(reordered).encode()))
        changed = {**data, "diagnostics": [{**warning, "detail": "changed"}, error]}
        self.assertNotEqual(left["canonical_sha256"],
                            probe.canonical_validation_sha256(changed))

    def test_runner_timeout_and_output_cap_kill_process_group(self):
        root = Path(self.temp.name)
        with mock.patch.object(probe, "wait_for_load", return_value=1.0), \
             mock.patch.object(probe.os, "getpriority", return_value=10):
            with self.assertRaisesRegex(probe.ProbeError, "timeout"):
                probe.launch([sys.executable, "-c", "import time; time.sleep(5)"], root,
                             {"PATH": "/usr/bin:/bin"}, root / "timeout.stdout", root / "timeout.stderr",
                             1024, timeout=0.2, _sandbox_prefix=[])
            with self.assertRaisesRegex(probe.ProbeError, "output_cap"):
                probe.launch([sys.executable, "-c", "print('x' * 100000)"], root,
                             {"PATH": "/usr/bin:/bin"}, root / "cap.stdout", root / "cap.stderr",
                             1024, timeout=3, _sandbox_prefix=[])
        self.assertGreaterEqual(probe.LAUNCHES[-1]["nice_at_launch"], 10)

    def test_runner_refuses_unavailable_nice_before_launch(self):
        root = Path(self.temp.name)
        with mock.patch.object(probe, "wait_for_load", return_value=1.0), \
             mock.patch.object(probe.os, "getpriority", return_value=0), \
             mock.patch.object(probe.os, "setpriority", side_effect=PermissionError):
            with self.assertRaises(PermissionError):
                probe.launch([sys.executable, "-c", "pass"], root, {"PATH": "/usr/bin:/bin"},
                             root / "refuse.stdout", root / "refuse.stderr", 1024)
        self.assertFalse((root / "refuse.stdout").exists())

    def test_stale_output_refuses_before_native_execution(self):
        root = Path(self.temp.name)
        inputs = root / "inputs.json"
        inputs.write_text(json.dumps(self.manifest))
        out = root / "receipt.json"
        out.write_text("stale")
        scratch = root / "scratch"
        old_argv = sys.argv
        try:
            sys.argv = [str(MODULE), "--inputs", str(inputs), "--scratch", str(scratch), "--out", str(out)]
            self.assertEqual(probe.main(), 1)
        finally:
            sys.argv = old_argv
        self.assertEqual(out.read_text(), "stale")
        self.assertFalse(scratch.exists())


if __name__ == "__main__":
    unittest.main()
