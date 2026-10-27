#!/usr/bin/env python3
"""Tests for the HLSL replay's no-scope control arm."""

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest


HERE = Path(__file__).resolve().parent
CONTROL = HERE / "control.py"
PATCH = HERE / "scope-control.patch"
SUBJECTS = ("directx-graphics-samples", "kajiya", "directxtk12")


def load_control():
    spec = importlib.util.spec_from_file_location("hlsl_scope_control", CONTROL)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class FakeRunner:
    def __init__(self):
        self.calls = []

    def __call__(self, command, **kwargs):
        self.calls.append((list(command), kwargs))
        return None


class ControlTests(unittest.TestCase):
    def setUp(self):
        self.control = load_control()

    def make_oracles(self, root):
        oracle = root / "workspace/tmp/oracle"
        clones = root / "workspace/calibration/hlsl"
        oracle.mkdir(parents=True)
        for subject in SUBJECTS:
            (clones / subject).mkdir(parents=True)
            (oracle / f"hlsl-{subject}-dxc.json").write_text(
                json.dumps({"tool": "dxc-ast", "subject": subject}))
            (oracle / f"hlsl-{subject}-imports.json").write_text(
                json.dumps({"tool": "clang-includes", "subject": subject}))
            (oracle / f"hlsl-{subject}.scope").write_text(f"{subject}.hlsl\n")
        return oracle, clones

    def test_plan_has_twelve_commands_and_reuses_original_absolute_inputs(self):
        with tempfile.TemporaryDirectory() as td:
            source = Path(td).resolve()
            oracle, clones = self.make_oracles(source)
            binary = source / "workspace/tmp/hlsl-scope-replay/control-source/workspace/tools/indexbench"
            out = source / "workspace/tmp/hlsl-scope-replay/control"
            commands = self.control.grade_commands(source, binary, out)

            self.assertEqual(len(commands), 12)
            self.assertEqual([Path(c[c.index("-subject") + 1]).name for c in commands],
                             list(SUBJECTS) * 4)
            for command in commands:
                self.assertEqual(command[0], str(binary))
                self.assertNotIn("dxc", Path(command[0]).name.lower())
                subject = Path(command[command.index("-subject") + 1]).name
                tool_paths = [Path(command[i + 1]) for i, arg in enumerate(command)
                              if arg == "-tool-oracle"]
                self.assertEqual(tool_paths, [
                    (oracle / f"hlsl-{subject}-dxc.json").resolve(),
                    (oracle / f"hlsl-{subject}-imports.json").resolve(),
                ])
                self.assertEqual(Path(command[command.index("-subject") + 1]),
                                 (clones / subject).resolve())
            for command in commands[:9]:
                self.assertNotIn("-only", command)
            for command in commands[9:]:
                scope = (oracle / f"hlsl-{Path(command[command.index('-subject') + 1]).name}.scope")
                self.assertEqual(command[command.index("-only") + 1], scope.read_text().strip())

    def test_snapshot_rejects_any_producer_input_mutation(self):
        with tempfile.TemporaryDirectory() as td:
            source = Path(td).resolve()
            oracle, _ = self.make_oracles(source)
            snapshot = self.control.snapshot_inputs(source)
            receipt = self.control.verify_inputs(snapshot)
            self.assertEqual(set(receipt), {"input_files", "input_bytes", "unchanged"})
            self.assertEqual(receipt["input_files"], 9)
            self.assertEqual(receipt["input_bytes"], sum(p.stat().st_size for p in oracle.iterdir()))
            self.assertTrue(receipt["unchanged"])

            target = oracle / "hlsl-kajiya-dxc.json"
            target.write_text(target.read_text() + "\n")
            with self.assertRaisesRegex(RuntimeError, "oracle input changed"):
                self.control.verify_inputs(snapshot)

    def test_patch_is_exactly_the_two_control_changes(self):
        text = PATCH.read_text()
        self.assertIn("-\tif len(tf.Calls) > 0 || len(tf.Scope) > 0 {", text)
        self.assertIn("+\tif len(tf.Calls) > 0 {", text)
        self.assertIn("-\t\tif len(tf.Scope) > 0 {", text)
        self.assertIn("-\t\t\tfor f := range inScope {", text)
        self.assertEqual(text.count("diff --git "), 1)
        self.assertIn("a/cmd/indexbench/langoracle.go", text)
        self.assertNotIn("definitions given", text)

    def test_expected_source_transform_preserves_every_other_byte(self):
        before = b"prefix\n\tif len(tf.Calls) > 0 || len(tf.Scope) > 0 {\n"
        before += b"\t\ttruth := map[langbench.CallPair]bool{}\n"
        before += b"\t\tfiles := map[string]bool{}\n"
        before += b"\t\tif len(tf.Scope) > 0 {\n"
        before += b"\t\t\t// Explicit scope covers files with zero oracle calls. Legacy\n"
        before += b"\t\t\t// nonempty calls without scope cover only their named files.\n"
        before += b"\t\t\tfor f := range inScope {\n\t\t\t\tfiles[f] = true\n\t\t\t}\n\t\t}\n"
        before += b"\t\tfor _, c := range tf.Calls {\nsuffix\n"
        expected = before.replace(
            b"\tif len(tf.Calls) > 0 || len(tf.Scope) > 0 {",
            b"\tif len(tf.Calls) > 0 {").replace(
                b"\t\tif len(tf.Scope) > 0 {\n"
                b"\t\t\t// Explicit scope covers files with zero oracle calls. Legacy\n"
                b"\t\t\t// nonempty calls without scope cover only their named files.\n"
                b"\t\t\tfor f := range inScope {\n"
                b"\t\t\t\tfiles[f] = true\n\t\t\t}\n\t\t}\n", b"")
        after = self.control.expected_control_source(before)
        self.assertEqual(after, expected)
        self.assertEqual(hashlib.sha256(after).hexdigest(),
                         hashlib.sha256(expected).hexdigest())

    def test_run_grades_without_overwriting_main_outputs(self):
        with tempfile.TemporaryDirectory() as td:
            source = Path(td).resolve()
            self.make_oracles(source)
            control_root = source / "workspace/tmp/hlsl-scope-replay/control-source"
            (control_root / "calibration/oracles/dxc-ast").mkdir(parents=True)
            reading = control_root / "calibration/oracles/dxc-ast/reading.sh"
            reading.write_text("#!/usr/bin/env bash\nexit 0\n")
            reading.chmod(0o755)
            binary = control_root / "workspace/tools/indexbench"
            binary.parent.mkdir(parents=True)
            binary.write_text("binary")
            raw = source / "workspace/tmp/hlsl-scope-replay/control-reading-private.json"
            runner = FakeRunner()
            self.control.run_grades(source, control_root, raw, runner=runner)
            grades = [row for row in runner.calls if row[0] and row[0][0] == str(binary)]
            notes = [row for row in runner.calls if row[0][:2] == ["bash", str(reading)]]
            self.assertEqual(len(grades), 12)
            self.assertEqual(len(notes), 1)
            self.assertEqual(notes[0][1]["env"]["NOTES_ONLY"], "1")
            self.assertEqual(notes[0][1]["env"]["HLSL_CLONES"],
                             str((source / "workspace/calibration/hlsl").resolve()))
            for command, _ in grades:
                out = Path(command[command.index("-out") + 1])
                unmatched = Path(command[command.index("-unmatched") + 1])
                self.assertIn("/control-source/workspace/tmp/oracle/", str(out))
                self.assertIn("/control-source/workspace/tmp/oracle/", str(unmatched))
                self.assertNotEqual(out.parent, source / "workspace/tmp/oracle")


if __name__ == "__main__":
    unittest.main()
