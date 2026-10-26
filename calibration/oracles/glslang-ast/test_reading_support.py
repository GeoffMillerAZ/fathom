import importlib.util
import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

spec = importlib.util.spec_from_file_location("reading_support", Path(__file__).with_name("reading_support.py"))
support = importlib.util.module_from_spec(spec)
spec.loader.exec_module(support)


class ReadingSupportTests(unittest.TestCase):
    def synthetic_reading(self, context=...):
        root = Path(__file__).resolve().parents[3]
        source = root / "calibration/oracles/glslang-ast/reading.sh"
        with tempfile.TemporaryDirectory(dir=root / "workspace/tmp") as dirname:
            fixture = Path(dirname)
            checkout = fixture / "checkout"
            script = checkout / "calibration/oracles/glslang-ast/reading.sh"
            script.parent.mkdir(parents=True)
            shutil.copyfile(source, script)
            (checkout / "go.mod").write_text("module synthetic\n")
            (checkout / "workspace/tmp").mkdir(parents=True)
            mockbin = fixture / "bin"
            mockbin.mkdir()
            python = mockbin / "python3"
            python.write_text("""#!/bin/sh
case " $* " in
  *" -I -B - "*) exec "$REAL_PYTHON" "$@" ;;
  *" reading_support.py source "*) printf '%040d\\n' 0 ;;
  *) exit 0 ;;
esac
""")
            go = mockbin / "go"
            go.write_text("""#!/bin/sh
cp "$MOCK_INDEXBENCH_SOURCE" "$3"
chmod +x "$3"
""")
            indexbench = fixture / "indexbench"
            indexbench.write_text("""#!/bin/sh
printf '%s\\n' "$*" >> "$MOCK_LOG"
""")
            for executable in (python, go, indexbench):
                executable.chmod(0o755)
            log = fixture / "lang-oracle.log"
            env = os.environ.copy()
            env.update(PATH=str(mockbin) + os.pathsep + env["PATH"],
                       REAL_PYTHON=os.path.realpath(os.sys.executable),
                       MOCK_INDEXBENCH_SOURCE=str(indexbench), MOCK_LOG=str(log))
            env.pop("GLSL_BENCH_CONTEXT", None)
            if context is not ...:
                env["GLSL_BENCH_CONTEXT"] = context
            result = subprocess.run(["bash", str(script), "workspace/reading.json"],
                                    cwd=checkout, env=env, capture_output=True, text=True)
            return result, log.read_text().splitlines() if log.exists() else []

    def test_reading_optional_bench_context_reaches_mocked_lang_oracle(self):
        without, plain_calls = self.synthetic_reading()
        self.assertEqual(without.returncode, 0, without.stderr)
        self.assertEqual(len(plain_calls), 9)
        self.assertTrue(all(" -note " not in " " + call + " " for call in plain_calls))

        with_context, noted_calls = self.synthetic_reading("synthetic-context")
        self.assertEqual(with_context.returncode, 0, with_context.stderr)
        self.assertEqual(len(noted_calls), 9)
        self.assertTrue(all("-note synthetic-context" in call for call in noted_calls))

    def reading_runs_root(self, value=...):
        root = Path(__file__).resolve().parents[3]
        script = root / "calibration/oracles/glslang-ast/reading.sh"
        driver = r'''
script=$1
set -T
trap 'case "$BASH_COMMAND" in source_commit=*) printf "%s\n" "$runs"; exit 0;; esac' DEBUG
source "$script" workspace/test-reading.json
'''
        env = os.environ.copy()
        env.pop("GLSL_RUNS_ROOT", None)
        env.pop("GLSL_CATALOG_MODE", None)
        if value is not ...:
            env["GLSL_RUNS_ROOT"] = value
        return subprocess.run(["bash", "-c", driver, "reading-test", str(script)],
                              cwd=root, env=env, capture_output=True, text=True)

    def test_reading_runs_root_defaults_to_current_catalog_location(self):
        result = self.reading_runs_root()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(),
                         "workspace/oracles/glslang-ast/catalog-modes/full-registry")

    def test_reading_runs_root_accepts_fresh_workspace_location(self):
        result = self.reading_runs_root("workspace/oracles/glslang-ast/remeasure-2026-10-26")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(),
                         "workspace/oracles/glslang-ast/remeasure-2026-10-26")

    def test_reading_runs_root_rejects_empty_absolute_nonworkspace_and_traversal(self):
        for value in ("", "/tmp/glsl", "artifacts/glsl", "workspace", "workspace/",
                      "workspace/../outside"):
            with self.subTest(value=value):
                result = self.reading_runs_root(value)
                self.assertEqual(result.returncode, 2)
                self.assertIn("invalid GLSL_RUNS_ROOT", result.stderr)

    def test_reading_runs_root_rejects_symlinked_parent_outside_workspace_without_write(self):
        root = Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory(dir=root / "workspace/tmp") as dirname:
            escape = Path(dirname) / "escape"
            escape.symlink_to(root / "docs", target_is_directory=True)
            destination = escape / ("fresh-reading-" + Path(dirname).name)
            value = str(destination.relative_to(root))
            result = self.reading_runs_root(value)
            self.assertEqual(result.returncode, 2)
            self.assertIn("invalid GLSL_RUNS_ROOT", result.stderr)
            self.assertFalse(destination.exists())

    def fixtures(self):
        names = support.SUBJECTS
        bundles = [{"subject": name, "files": 10, "parsed": 10, "calls": i + 1,
                    "rejects": {}, "subject_sha": "s" * 40, "oracle_sha256": "a" * 64,
                    "receipt_sha256": "b" * 64, "catalog_mode": "core-plus-types",
                    "catalog_fingerprint_sha256": "f" * 64,
                    "catalog_function_count": 197, "catalog_constructor_count": 126}
                   for i, name in enumerate(names)]
        readings = []
        for rate in (2.2, 1.1, 1.8):
            readings.append({"fathom_commit": "c" * 40, "tenancy": {"throughput_mb_s": rate},
                             "subjects": [{"name": support.DISPLAY_NAMES[name], "sha": "s" * 40, "files": 10,
                                           "tenancy": {"files_with_error": i}} for i, name in enumerate(names)],
                             "notes": []})
        return readings, bundles

    def test_median_and_generated_notes(self):
        readings, bundles = self.fixtures()
        result, chosen = support.annotate(readings, bundles, "c" * 40)
        self.assertEqual(chosen, 2)
        self.assertEqual(result["tenancy"]["throughput_mb_s"], 1.8)
        self.assertTrue(any("pooled 3/30 (10.0%)" in note for note in result["notes"]))
        self.assertTrue(any("median run 3" in note for note in result["notes"]))
        self.assertTrue(any("GLSL source commit: " + "c" * 40 == note for note in result["notes"]))
        self.assertTrue(any("core-plus-types" in note and "f" * 64 in note
                            for note in result["notes"]))
        self.assertTrue(any("197 functions, 126 type identifiers" in note for note in result["notes"]))

    def test_subject_population_mismatch_rejected(self):
        readings, bundles = self.fixtures()
        readings[0]["subjects"][0]["files"] = 9
        readings[1]["subjects"][0]["files"] = 9
        readings[2]["subjects"][0]["files"] = 9
        with self.assertRaisesRegex(ValueError, "file counts"):
            support.annotate(readings, bundles, "c" * 40)

    def test_nonmedian_subject_population_and_sha_rejected(self):
        readings, bundles = self.fixtures()
        readings[0]["subjects"][0]["files"] = 9
        with self.assertRaisesRegex(ValueError, "file counts"):
            support.annotate(readings, bundles, "c" * 40)
        readings, bundles = self.fixtures()
        readings[0]["subjects"][0]["sha"] = "d" * 40
        with self.assertRaisesRegex(ValueError, "subject SHAs"):
            support.annotate(readings, bundles, "c" * 40)

    def test_source_identity_rejects_override_not_equal_head(self):
        with mock.patch("subprocess.run", side_effect=[mock.Mock(stdout="a" * 40 + "\n"),
                                                     mock.Mock(stdout="")]):
            with self.assertRaisesRegex(ValueError, "FATHOM_COMMIT"):
                support.source_identity(override="b" * 40)

    def test_source_identity_rejects_untracked_worktree(self):
        with mock.patch("subprocess.run", side_effect=[mock.Mock(stdout="a" * 40 + "\n"),
                                                     mock.Mock(stdout="?? calibration/oracles/glslang-ast/new.py\n")]):
            with self.assertRaisesRegex(ValueError, "worktree is dirty"):
                support.source_identity()

    def test_dynamic_adapter_import_does_not_write_source_bytecode(self):
        with tempfile.TemporaryDirectory() as dirname:
            root = Path(dirname)
            for name in ("reading_support.py", "glslang_ast.py"):
                shutil.copyfile(Path(__file__).with_name(name), root / name)
            code = (
                "import importlib.util,sys; from pathlib import Path; "
                "sys.dont_write_bytecode=False; "
                "p=Path(sys.argv[1]); s=importlib.util.spec_from_file_location('reading_support',p); "
                "m=importlib.util.module_from_spec(s); s.loader.exec_module(m); m.adapter()"
            )
            subprocess.run(["python3", "-I", "-c", code, str(root / "reading_support.py")], check=True)
            self.assertEqual(list(root.glob("__pycache__/glslang_ast.*.pyc")), [])

    def test_mixed_catalog_modes_or_fingerprints_are_rejected(self):
        readings, bundles = self.fixtures()
        bundles[1]["catalog_mode"] = "baseline-core"
        with self.assertRaisesRegex(ValueError, "catalog mode"):
            support.annotate(readings, bundles, "c" * 40)

    def test_private_evidence_binds_reading_unmatched_oracle_and_subject_identity(self):
        readings, bundles = self.fixtures()
        with tempfile.TemporaryDirectory() as dirname:
            root = Path(dirname)
            reading = root / "glsl.json"
            reading.write_text(json.dumps(readings[2]))
            for bundle in bundles:
                oracle = {"calls": [], "scope": []}
                oracle_path = root / (bundle["subject"] + "-oracle.json")
                oracle_path.write_text(json.dumps(oracle))
                bundle.update(shader_tree_sha256="d" * 64,
                              extension_catalog_sha256="e" * 64,
                              oracle_path=str(oracle_path),
                              oracle_sha256=support.sha(oracle_path),
                              oracle_truth_sha256=support.oracle_truth_sha256(oracle))
                unmatched = root / f'{bundle["subject"]}-run3-unmatched.json'
                unmatched.write_text('{"calls":{}}')
            evidence = support.evidence_payload(
                reading, 2, bundles, root, "c" * 40)
            self.assertEqual(evidence["reading_sha256"], hashlib.sha256(reading.read_bytes()).hexdigest())
            self.assertEqual(evidence["catalog_mode"], "core-plus-types")
            self.assertEqual(evidence["extension_catalog_sha256"], "e" * 64)
            self.assertEqual(len(evidence["subjects"]), 3)
            self.assertTrue(all(row["unmatched_path"].endswith("run3-unmatched.json")
                                for row in evidence["subjects"]))
            self.assertEqual(evidence["subjects"][0]["oracle_truth_sha256"],
                             support.oracle_truth_sha256({"calls": [], "scope": []}))
        readings, bundles = self.fixtures()
        bundles[1]["catalog_fingerprint_sha256"] = "e" * 64
        with self.assertRaisesRegex(ValueError, "catalog fingerprint"):
            support.annotate(readings, bundles, "c" * 40)


if __name__ == "__main__":
    unittest.main()
