#!/usr/bin/env python3
import copy
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


HERE = Path(__file__).resolve().parent
PUBLISH = HERE / "publish.py"
REPLAY = HERE / "replay.sh"
WORKFLOW = HERE / "hlsl-scope-replay.yml"
CONTROLLER_COMMIT = "c" * 40
CONTROLLER_HASHES = {
    "replay_sha256": "1" * 64,
    "publish_sha256": "2" * 64,
    "workflow_sha256": "3" * 64,
}
ACTIVE_WORKFLOW = ".github/workflows/hlsl-scope-replay.yml"


def load_publish():
    spec = importlib.util.spec_from_file_location("scope_replay_publish", PUBLISH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def reading(commit="6db920e72898", level="L3", calls=(90, 100), throughput=2.0,
            runs=(1.8, 2.0, 2.2)):
    found, truth = calls
    return {
        "dialect": "hlsl",
        "fathom_commit": commit,
        "level_met": level,
        "grade": {"L1": "excellent", "L2": "good", "L3": "good"},
        "metrics": {"call_precision": found / 95, "call_recall": found / truth},
        "counts": {"call_fathom": 95, "call_truth": truth, "call_truth_found": found,
                   "oracle_defs": 80},
        "oracles": [
            {"tier": "B", "tool": "clang-includes", "version": "Ubuntu clang version 18.1.3"},
            {"tier": "B", "tool": "dxc-ast", "version": "libdxcompiler.so: 1.9"},
        ],
        "subjects": [
            {"name": "subject-a", "sha": "a" * 40, "files": 6, "bytes": 120,
             "counts": {"call_truth": truth},
             "adjudicated": ["private/person/shader.hlsl:12 SecretName (call)"],
             "tenancy": {"faults": 1, "faulting_files": ["private/person/shader.hlsl"],
                         "files_with_error": 1}},
            {"name": "subject-b", "sha": "b" * 40, "files": 4, "bytes": 80,
             "counts": {"call_truth": 0},
             "tenancy": {"faults": 0, "faulting_files": [], "files_with_error": 0}},
        ],
        "tenancy": {"faults": 1, "faulting_files": ["private/person/shader.hlsl"],
                    "files_with_error": 1, "throughput_mb_s": throughput,
                    "throughput_basis": "cpu", "wall_throughput_mb_s": throughput},
        "notes": [
            "subject-a: dxc-ast (flags): 5 of 6 measured files in scope (their own unit)",
            "subject-b: dxc-ast (flags): 3 of 4 measured files in scope (their own unit)",
            ("subject-a: excluded example private/person/shader.hlsl:12: "
             "use of undeclared identifier 'PersonName'"),
            ("throughput (design 1.9 T1, T2; reading.sh): three runs over every measured "
             f"file read {runs[0]}, {runs[1]}, {runs[2]} MB/s CPU; the reading is the median run, "
             f"{throughput} MB/s"),
        ],
    }


class PublishTests(unittest.TestCase):
    def test_committed_baseline_receipts_are_machine_readable(self):
        publish = load_publish()
        baseline = json.loads((HERE.parents[3] / "docs/bench/readings/lang/hlsl.json").read_text())
        self.assertEqual(publish._scope(baseline),
                         {"dxc_files": 543, "measured_files": 644,
                          "subjects": [
                              {"name": "directx-graphics-samples", "dxc_files": 362,
                               "measured_files": 446},
                              {"name": "directxtk12", "dxc_files": 21,
                               "measured_files": 21},
                              {"name": "kajiya", "dxc_files": 160,
                               "measured_files": 177},
                          ]})
        self.assertEqual(publish._runs(baseline), [0.75, 0.69, 0.65])
        self.assertEqual(len(baseline["counts"]), 24)

    def test_sanitize_removes_disclosures_but_keeps_schema_and_numeric_evidence(self):
        publish = load_publish()
        raw = reading()
        sanitized, receipt = publish.sanitize_reading(raw, "6db920e72898abb20bf86c22652b401902e34598")

        rendered = json.dumps(sanitized, sort_keys=True)
        self.assertNotIn("private/person", rendered)
        self.assertNotIn("PersonName", rendered)
        self.assertNotIn("SecretName", rendered)
        self.assertEqual(sanitized["counts"], raw["counts"])
        self.assertEqual(sanitized["metrics"], raw["metrics"])
        self.assertEqual(sanitized["oracles"], raw["oracles"])
        self.assertEqual(sanitized["subjects"][0]["sha"], "a" * 40)
        self.assertEqual(sanitized["tenancy"]["faulting_files"],
                         ["redacted-fault-000001"])
        self.assertEqual(sanitized["subjects"][0]["tenancy"]["faulting_files"],
                         ["redacted-fault-000002"])
        tenancy_blocks = [sanitized["tenancy"]]
        tenancy_blocks.extend(subject["tenancy"] for subject in sanitized["subjects"])
        for tenancy in tenancy_blocks:
            self.assertEqual(tenancy["faults"], len(tenancy["faulting_files"]))
        self.assertEqual(sanitized["subjects"][0]["adjudicated"], [])
        self.assertEqual(receipt["diagnostic_notes_removed"], 4)
        self.assertEqual(receipt["faulting_file_entries_removed"], 2)
        self.assertEqual(receipt["adjudicated_entries_removed"], 1)
        self.assertRegex(receipt["diagnostic_notes_sha256"], r"^[0-9a-f]{64}$")
        self.assertRegex(receipt["faulting_file_entries_sha256"], r"^[0-9a-f]{64}$")
        self.assertRegex(receipt["adjudicated_entries_sha256"], r"^[0-9a-f]{64}$")
        self.assertTrue(any("removed 4 diagnostic notes" in note for note in sanitized["notes"]))
        self.assertTrue(any("neutral ordinal placeholders" in note
                            for note in sanitized["notes"]))
        self.assertTrue(any("8 of 10 measured files" in note for note in sanitized["notes"]))
        self.assertTrue(any("1.8, 2, 2.2" in note for note in sanitized["notes"]))

    def test_diff_reports_true_current_level_all_counter_deltas_and_separate_timing(self):
        publish = load_publish()
        baseline = reading(commit="6c5d2d56", level="L2", calls=(80, 100),
                           throughput=1.92, runs=(0.75, 0.69, 0.65))
        current = reading(level="L3", calls=(93, 101), throughput=2.1,
                          runs=(2.0, 2.1, 2.4))
        current["counts"].update({"call_fathom": 99, "new_counter": 7})
        baseline["metrics"]["typed_recall"] = None
        current["metrics"].update({"typed_recall": 0.8, "new_quality": 0.7})
        baseline["subjects"][0]["counts"]["call_truth"] = 90
        baseline["subjects"][1]["counts"]["call_truth"] = 10
        current["subjects"][0]["counts"]["call_truth"] = 85
        current["subjects"][1]["counts"]["call_truth"] = 15
        current["notes"][0] = "subject-a: dxc-ast (flags): 4 of 6 measured files in scope (their own unit)"
        current["notes"][1] = "subject-b: dxc-ast (flags): 4 of 4 measured files in scope (their own unit)"
        sanitized, receipt = publish.sanitize_reading(
            current, "6db920e72898abb20bf86c22652b401902e34598")
        diff = publish.build_diff(
            baseline, current, receipt,
            source_commit="6db920e72898abb20bf86c22652b401902e34598",
            dxc_sha256="a1d3e3b5e1c5685b3eb27d5e8890e41d87df45def05112a2d6f1a63a931f7d60",
            required_glibc="2.38", runner_glibc="2.39", go_version="go1.27.3",
            controller_commit=CONTROLLER_COMMIT,
            active_workflow_mode="github-actions-verified-active-copy",
            active_workflow_path=ACTIVE_WORKFLOW,
            active_workflow_sha256=CONTROLLER_HASHES["workflow_sha256"],
            **CONTROLLER_HASHES)

        self.assertEqual(diff["level"]["current"], "L3")
        self.assertEqual(diff["grade"]["current"], sanitized["grade"])
        self.assertEqual(diff["counts"]["delta"]["call_fathom"], 4)
        self.assertEqual(diff["counts"]["delta"]["new_counter"], 7)
        self.assertEqual(set(diff["counts"]["delta"]), set(current["counts"]))
        self.assertAlmostEqual(diff["calls"]["current"]["recall"], 93 / 101)
        self.assertEqual(diff["scope"]["current"]["dxc_files"], 8)
        self.assertEqual(diff["scope"]["subjects"][0]["dxc_files_delta"], -1)
        self.assertEqual(diff["scope"]["subjects"][1]["dxc_files_delta"], 1)
        self.assertEqual(diff["subject_counts"][0]["delta"]["call_truth"], -5)
        self.assertEqual(diff["subject_counts"][1]["delta"]["call_truth"], 5)
        self.assertIsNone(diff["metrics"]["delta"]["typed_recall"])
        self.assertEqual(diff["metrics"]["change"]["typed_recall"], "became-measured")
        self.assertEqual(diff["metrics"]["change"]["new_quality"], "added")
        timing = diff["performance"]
        self.assertEqual(timing["baseline_committed_throughput_mb_s"], 1.92)
        self.assertEqual(timing["baseline_historical_driver_runs_mb_s"], [0.75, 0.69, 0.65])
        self.assertEqual(timing["fresh_driver_runs_mb_s"], [2.0, 2.1, 2.4])
        self.assertNotIn("throughput_delta", timing)
        self.assertIn("may change the current level", timing["comparison"])
        self.assertEqual(diff["runtime"]["required_glibc"], "2.38")
        self.assertEqual(diff["source"]["controller"]["commit"], CONTROLLER_COMMIT)
        self.assertEqual(
            diff["source"]["controller"]["files"]
                ["calibration/oracles/dxc-ast/scope-replay/replay.sh"],
            CONTROLLER_HASHES["replay_sha256"])
        self.assertEqual(diff["source"]["controller"]["active_workflow"], {
            "mode": "github-actions-verified-active-copy",
            "path": ACTIVE_WORKFLOW,
            "sha256": CONTROLLER_HASHES["workflow_sha256"],
        })

    def test_diff_refuses_changed_subject_pins(self):
        publish = load_publish()
        baseline = reading(commit="6c5d2d56")
        current = reading()
        current["subjects"][0]["sha"] = "c" * 40
        _, receipt = publish.sanitize_reading(current, "6db920e72898abb20bf86c22652b401902e34598")
        with self.assertRaisesRegex(ValueError, "subject pins"):
            publish.build_diff(baseline, current, receipt,
                               source_commit="6db920e72898abb20bf86c22652b401902e34598",
                               dxc_sha256="a" * 64, required_glibc="2.38",
                               runner_glibc="2.39", go_version="go1.27.3",
                               controller_commit=CONTROLLER_COMMIT,
                               active_workflow_mode="manual-no-active-workflow",
                               active_workflow_path="", active_workflow_sha256="",
                               **CONTROLLER_HASHES)

    def test_diff_refuses_missing_empty_malformed_or_wrong_source_revision(self):
        publish = load_publish()
        baseline = reading(commit="6c5d2d56")
        cases = (None, "", "xyz1234", "f" * 7)
        for revision in cases:
            current = reading()
            if revision is None:
                del current["fathom_commit"]
            else:
                current["fathom_commit"] = revision
            _, receipt = publish.sanitize_reading(
                current, "6db920e72898abb20bf86c22652b401902e34598")
            with self.subTest(revision=revision), self.assertRaisesRegex(
                    ValueError, "source commit"):
                publish.build_diff(
                    baseline, current, receipt,
                    source_commit="6db920e72898abb20bf86c22652b401902e34598",
                    dxc_sha256="a" * 64, required_glibc="2.38",
                    runner_glibc="2.39", go_version="go1.27.3",
                    controller_commit=CONTROLLER_COMMIT,
                    active_workflow_mode="manual-no-active-workflow",
                    active_workflow_path="", active_workflow_sha256="",
                    **CONTROLLER_HASHES)

    def test_cli_records_controller_identity(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            baseline = reading(commit="6c5d2d56")
            current = reading()
            baseline_path = root / "baseline.json"
            current_path = root / "current.json"
            reading_out = root / "reading.json"
            diff_out = root / "diff.json"
            baseline_path.write_text(json.dumps(baseline))
            current_path.write_text(json.dumps(current))
            command = [
                "python3", "-I", "-B", str(PUBLISH),
                "--baseline", str(baseline_path),
                "--regenerated", str(current_path),
                "--reading-out", str(reading_out),
                "--diff-out", str(diff_out),
                "--source-commit", "6db920e72898abb20bf86c22652b401902e34598",
                "--dxc-sha256", "a" * 64,
                "--required-glibc", "2.38",
                "--runner-glibc", "2.39",
                "--go-version", "go1.27.3",
                "--controller-commit", CONTROLLER_COMMIT,
                "--replay-sh-sha256", CONTROLLER_HASHES["replay_sha256"],
                "--publish-py-sha256", CONTROLLER_HASHES["publish_sha256"],
                "--workflow-sha256", CONTROLLER_HASHES["workflow_sha256"],
                "--active-workflow-mode", "manual-no-active-workflow",
                "--active-workflow-path", "",
                "--active-workflow-sha256", "",
            ]
            result = subprocess.run(command, text=True, stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE)
            self.assertEqual(result.returncode, 0, result.stderr)
            controller = json.loads(diff_out.read_text())["source"]["controller"]
            self.assertEqual(controller["commit"], CONTROLLER_COMMIT)
            self.assertEqual(set(controller["files"].values()),
                             set(CONTROLLER_HASHES.values()))
            self.assertEqual(controller["active_workflow"], {
                "mode": "manual-no-active-workflow", "path": None, "sha256": None})


class ReplayShellTests(unittest.TestCase):
    def run_bash(self, script, env=None):
        return subprocess.run(["bash", "-c", script], cwd=HERE, env=env,
                              text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def test_run_reading_uses_main_checkout_cwd_and_keeps_output_private(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            driver = root / "calibration/oracles/dxc-ast/reading.sh"
            driver.parent.mkdir(parents=True)
            driver.write_text("""#!/usr/bin/env bash
printf '%s\\n' "$PWD" > "$TRACE"
printf '%s\\n' "$HLSL_CLONES" >> "$TRACE"
printf '%s\\n' "$PATH" >> "$TRACE"
printf 'secret-marker stdout\\n'
printf 'secret-marker stderr\\n' >&2
printf '{\"dialect\":\"hlsl\"}\\n' > "$1"
""")
            driver.chmod(0o755)
            trace = root / "trace"
            raw = root / "workspace/tmp/replay/raw.json"
            raw.parent.mkdir(parents=True)
            clones = root / "workspace/calibration/hlsl"
            clang_bin = root / "workspace/tools/replay-bin"
            private_log = root / "workspace/tmp/replay/reading-private.log"
            env = os.environ.copy()
            env.update({"TRACE": str(trace), "REPLAY_LIBRARY_ONLY": "1"})
            result = self.run_bash(
                f"source {REPLAY}; run_reading {root} {clones} {clang_bin} {raw} {private_log}", env)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertNotIn("secret-marker", result.stdout + result.stderr)
            self.assertIn("secret-marker stdout", private_log.read_text())
            self.assertIn("secret-marker stderr", private_log.read_text())
            lines = trace.read_text().splitlines()
            self.assertEqual(lines[0], str(root))
            self.assertEqual(lines[1], str(clones))
            self.assertEqual(lines[2].split(":")[0], str(clang_bin))
            self.assertEqual(json.loads(raw.read_text())["dialect"], "hlsl")

    def test_run_reading_failure_exposes_only_generic_error(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            driver = root / "calibration/oracles/dxc-ast/reading.sh"
            driver.parent.mkdir(parents=True)
            driver.write_text("""#!/usr/bin/env bash
printf 'secret failure detail\\n' >&2
exit 1
""")
            driver.chmod(0o755)
            private_log = root / "workspace/tmp/replay/reading-private.log"
            raw = root / "workspace/tmp/replay/raw.json"
            result = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; "
                f"run_reading {root} {root}/clones {root}/bin {raw} {private_log}")
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn("secret failure detail", result.stdout + result.stderr)
            self.assertIn("HLSL reading stage failed", result.stderr)
            self.assertIn("secret failure detail", private_log.read_text())

    def test_runner_tool_contract_accepts_mikefarah_yq4_and_jq16(self):
        with tempfile.TemporaryDirectory() as td:
            bindir = Path(td)
            tools = ("awk", "cmp", "curl", "getconf", "git", "objdump", "sed",
                     "sha256sum", "tar")
            for tool in tools:
                path = bindir / tool
                path.write_text("#!/bin/sh\nexit 0\n")
                path.chmod(0o755)
            (bindir / "yq").write_text(
                "#!/bin/sh\necho 'yq (https://github.com/mikefarah/yq/) version v4.44.1'\n")
            (bindir / "jq").write_text("#!/bin/sh\necho 'jq-1.6'\n")
            (bindir / "sort").write_text("#!/bin/sh\nshift\ncat\n")
            for tool in ("yq", "jq", "sort"):
                (bindir / tool).chmod(0o755)
            env = os.environ.copy()
            env["PATH"] = f"{bindir}:/usr/bin:/bin"
            result = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; check_runner_tools", env)
            self.assertEqual(result.returncode, 0, result.stderr)

    def _source_repo(self, root):
        subprocess.run(["git", "init", "-q", str(root)], check=True)
        tracked = root / "cmd/indexbench/main.go"
        tracked.parent.mkdir(parents=True)
        tracked.write_text("package main\n")
        subprocess.run(["git", "-C", str(root), "add", "."], check=True)
        subprocess.run(["git", "-C", str(root), "-c", "user.name=test",
                        "-c", "user.email=test@example.invalid", "commit", "-qm", "source"],
                       check=True)
        return tracked

    def test_tracked_source_change_outside_driver_is_refused(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            tracked = self._source_repo(root)
            tracked.write_text("package main\n// dirty\n")
            result = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; verify_checkout_clean {root} source")
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("source checkout is dirty", result.stderr)

    def test_untracked_go_source_is_refused(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            self._source_repo(root)
            (root / "cmd/indexbench/injected.go").write_text("package main\n")
            result = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; verify_checkout_clean {root} source")
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("untracked or ignored content outside workspace", result.stderr)

    def test_ignored_source_outside_workspace_is_refused(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            self._source_repo(root)
            (root / ".gitignore").write_text("generated/\nworkspace/\n")
            subprocess.run(["git", "-C", str(root), "add", ".gitignore"], check=True)
            subprocess.run(["git", "-C", str(root), "-c", "user.name=test",
                            "-c", "user.email=test@example.invalid", "commit", "-qm", "ignore"],
                           check=True)
            generated = root / "generated/injected.go"
            generated.parent.mkdir()
            generated.write_text("package injected\n")
            result = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; verify_checkout_clean {root} source")
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("untracked or ignored content outside workspace", result.stderr)

    def test_ignored_workspace_content_is_allowed(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            self._source_repo(root)
            (root / ".gitignore").write_text("workspace/\n")
            subprocess.run(["git", "-C", str(root), "add", ".gitignore"], check=True)
            subprocess.run(["git", "-C", str(root), "-c", "user.name=test",
                            "-c", "user.email=test@example.invalid", "commit", "-qm", "ignore"],
                           check=True)
            scratch = root / "workspace/tmp/generated.go"
            scratch.parent.mkdir(parents=True)
            scratch.write_text("private generated content\n")
            result = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; verify_checkout_clean {root} source")
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_controller_identity_requires_expected_commit_and_committed_files(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            owned = root / "calibration/oracles/dxc-ast/scope-replay"
            owned.mkdir(parents=True)
            for filename in ("replay.sh", "publish.py", "hlsl-scope-replay.yml"):
                (owned / filename).write_text(f"{filename}\n")
            subprocess.run(["git", "-C", str(root), "add", "."], check=True)
            subprocess.run(["git", "-C", str(root), "-c", "user.name=test",
                            "-c", "user.email=test@example.invalid", "commit", "-qm", "controller"],
                           check=True)
            commit = subprocess.check_output(
                ["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()
            good = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; verify_controller {root} {commit}")
            self.assertEqual(good.returncode, 0, good.stderr)
            wrong = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; verify_controller {root} {'f' * 40}")
            self.assertNotEqual(wrong.returncode, 0)
            self.assertIn("controller commit does not match", wrong.stderr)
            untracked = root / "injected-controller.sh"
            untracked.write_text("echo injected\n")
            extra = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; verify_controller {root} {commit}")
            self.assertNotEqual(extra.returncode, 0)
            self.assertIn("untracked or ignored content outside workspace", extra.stderr)
            untracked.unlink()
            (owned / "publish.py").write_text("dirty\n")
            dirty = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; verify_controller {root} {commit}")
            self.assertNotEqual(dirty.returncode, 0)

    def test_actions_controller_requires_exact_active_workflow_copy(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            owned = root / "calibration/oracles/dxc-ast/scope-replay"
            active = root / ACTIVE_WORKFLOW
            owned.mkdir(parents=True)
            active.parent.mkdir(parents=True)
            for filename in ("replay.sh", "publish.py"):
                (owned / filename).write_text(f"{filename}\n")
            dormant = owned / "hlsl-scope-replay.yml"
            dormant.write_text("name: exact-copy\n")
            active.write_text(dormant.read_text())
            subprocess.run(["git", "-C", str(root), "add", "."], check=True)
            subprocess.run(["git", "-C", str(root), "-c", "user.name=test",
                            "-c", "user.email=test@example.invalid", "commit", "-qm", "exact"],
                           check=True)
            commit = subprocess.check_output(
                ["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()
            env = os.environ.copy()
            env.update({
                "GITHUB_ACTIONS": "true",
                "GITHUB_WORKFLOW_REF": (
                    "owner/repo/.github/workflows/hlsl-scope-replay.yml@refs/heads/run"),
            })
            good = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; "
                f"verify_controller {root} {commit}; "
                "printf '%s|%s|%s' \"$ACTIVE_WORKFLOW_MODE\" "
                "\"$ACTIVE_WORKFLOW_PATH\" \"$ACTIVE_WORKFLOW_SHA256\"", env)
            self.assertEqual(good.returncode, 0, good.stderr)
            mode, path, digest = good.stdout.split("|")
            self.assertEqual(mode, "github-actions-verified-active-copy")
            self.assertEqual(path, ACTIVE_WORKFLOW)
            self.assertRegex(digest, r"^[0-9a-f]{64}$")

            active.write_text("name: widened-upload\n")
            subprocess.run(["git", "-C", str(root), "add", str(active)], check=True)
            subprocess.run(["git", "-C", str(root), "-c", "user.name=test",
                            "-c", "user.email=test@example.invalid", "commit", "-qm", "diverge"],
                           check=True)
            divergent = subprocess.check_output(
                ["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()
            mismatch = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; verify_controller {root} {divergent}",
                env)
            self.assertNotEqual(mismatch.returncode, 0)
            self.assertIn("active workflow does not match", mismatch.stderr)

    def test_platform_guard_refuses_arm_without_running_dxc(self):
        result = self.run_bash(
            f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; check_platform Linux aarch64")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("x86_64", result.stderr)

    def test_glibc_guard_refuses_runner_older_than_archive(self):
        result = self.run_bash(
            f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; require_glibc 2.38 2.35")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("requires glibc 2.38", result.stderr)

    def test_pin_checkout_detaches_at_exact_sha_when_branch_moved(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            origin = root / "origin"
            target = root / "target"
            subprocess.run(["git", "init", "-q", str(origin)], check=True)
            (origin / "subject.hlsl").write_text("first\n")
            subprocess.run(["git", "-C", str(origin), "add", "."], check=True)
            subprocess.run(["git", "-C", str(origin), "-c", "user.name=test",
                            "-c", "user.email=test@example.invalid", "commit", "-qm", "first"],
                           check=True)
            first = subprocess.check_output(
                ["git", "-C", str(origin), "rev-parse", "HEAD"], text=True).strip()
            (origin / "subject.hlsl").write_text("second\n")
            subprocess.run(["git", "-C", str(origin), "add", "."], check=True)
            subprocess.run(["git", "-C", str(origin), "-c", "user.name=test",
                            "-c", "user.email=test@example.invalid", "commit", "-qm", "second"],
                           check=True)
            subprocess.run(["git", "clone", "-q", str(origin), str(target)], check=True)
            result = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; pin_checkout {target} {first}")
            self.assertEqual(result.returncode, 0, result.stderr)
            got = subprocess.check_output(
                ["git", "-C", str(target), "rev-parse", "HEAD"], text=True).strip()
            self.assertEqual(got, first)
            self.assertEqual((target / "subject.hlsl").read_text(), "first\n")


class WorkflowTests(unittest.TestCase):
    def test_manual_read_only_bounded_x64_workflow_contract(self):
        text = WORKFLOW.read_text()
        self.assertIn("workflow_dispatch:", text)
        self.assertRegex(text, r"permissions:\s*\n\s*contents: read")
        self.assertIn("runs-on: ubuntu-24.04", text)
        self.assertIn("timeout-minutes:", text)
        self.assertIn("go-version: 1.27.3", text)
        self.assertIn("ref: ${{ github.sha }}", text)
        self.assertIn("Verify runner tools", text)
        self.assertIn("check_runner_tools", text)
        self.assertIn("yq", text)
        self.assertIn("jq", text)
        self.assertNotIn("container:", text)
        uses = [line.split("uses:", 1)[1].strip() for line in text.splitlines()
                if "uses:" in line]
        self.assertEqual(len(uses), 4)
        for use in uses:
            self.assertRegex(use, r"^[^@]+@[0-9a-f]{40}(?: # v[0-9]+)?$")
        self.assertIn("calibration/oracles/dxc-ast/scope-replay/replay.sh", text)
        self.assertIn("regenerated-reading.json", text)
        self.assertIn("reading-diff.json", text)
        self.assertNotIn("workspace/**", text)
        self.assertNotIn("workspace/", text.split("path: |", 1)[-1].replace(
            "workspace/artifacts/hlsl-scope-replay/regenerated-reading.json", "").replace(
            "workspace/artifacts/hlsl-scope-replay/reading-diff.json", ""))


if __name__ == "__main__":
    unittest.main()
