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
        "target_level": "L3",
        "disposition": "pass",
        "grade": {"L1": "excellent", "L2": "good", "L3": "good"},
        "metrics": {"call_precision": found / 95, "call_recall": found / truth},
        "metric_tiers": {"call_precision": "L2", "call_recall": "L2"},
        "required_kinds": ["call"],
        "kind_intersection": ["call"],
        "excluded_by_kind": {"definition": 4},
        "counts": {"call_fathom": 95, "call_truth": truth, "call_truth_found": found,
                   "oracle_defs": 80},
        "oracles": [
            {"tier": "B", "tool": "clang-includes", "version": "Ubuntu clang version 18.1.3"},
            {"tier": "B", "tool": "dxc-ast", "version": "libdxcompiler.so: 1.9"},
        ],
        "subjects": [
            {"name": "subject-a", "sha": "a" * 40, "files": 6, "bytes": 120,
             "counts": {"call_truth": truth},
             "families": {"calls": {"kind": "call", "tier": "B", "measured": True}},
             "adjudicated": ["private/person/shader.hlsl:12 SecretName (call)"],
             "tenancy": {"faults": 1, "faulting_files": ["private/person/shader.hlsl"],
                         "files_with_error": 1}},
            {"name": "subject-b", "sha": "b" * 40, "files": 4, "bytes": 80,
             "counts": {"call_truth": 0},
             "families": {"calls": {"kind": "call", "tier": "B", "measured": True}},
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
    source_commit = "6db920e72898abb20bf86c22652b401902e34598"

    def build(self, baseline, current, control, receipt=None):
        publish = load_publish()
        return publish.build_diff(
            baseline, current, control, receipt or {},
            source_commit=self.source_commit, dxc_sha256="a" * 64,
            required_glibc="2.38", runner_glibc="2.39", go_version="go1.27.3",
            controller_commit=CONTROLLER_COMMIT,
            control_patch_sha256="4" * 64,
            control_receipt={"input_files": 9, "input_bytes": 2048, "unchanged": True},
            active_workflow_mode="manual-no-active-workflow",
            active_workflow_path="", active_workflow_sha256="",
            **CONTROLLER_HASHES)

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

    def test_sanitize_retains_public_evidence_and_redacts_only_privacy_matches(self):
        publish = load_publish()
        raw = reading()
        raw["notes"].append(
            "Author: Example Maintainer <maintainer@example.invalid> inspected /Users/local/repo/a.hlsl")
        raw["subjects"][0]["adjudicated"].append(
            "Copyright 2026 Example Maintainer; /home/local/a.hlsl:2 accepted")
        sanitized, receipt = publish.sanitize_reading(raw, self.source_commit)

        rendered = json.dumps(sanitized, sort_keys=True)
        self.assertIn("private/person/shader.hlsl", rendered)
        self.assertIn("PersonName", rendered)
        self.assertIn("SecretName", rendered)
        self.assertNotIn("maintainer@example.invalid", rendered)
        self.assertNotIn("Example Maintainer", rendered)
        self.assertNotIn("/Users/local/repo/a.hlsl", rendered)
        self.assertNotIn("/home/local/a.hlsl", rendered)
        self.assertEqual(sanitized["counts"], raw["counts"])
        self.assertEqual(sanitized["notes"][:4], raw["notes"][:4])
        self.assertEqual(sanitized["subjects"][0]["adjudicated"][0],
                         raw["subjects"][0]["adjudicated"][0])
        tenancy_blocks = [sanitized["tenancy"]]
        tenancy_blocks.extend(subject["tenancy"] for subject in sanitized["subjects"])
        for tenancy in tenancy_blocks:
            self.assertEqual(tenancy["faults"], len(tenancy["faulting_files"]))
        self.assertGreaterEqual(receipt["person_matches_redacted"], 2)
        self.assertEqual(receipt["absolute_paths_redacted"], 2)
        self.assertNotIn("sha256", json.dumps(receipt))

    def test_diff_reports_every_nested_field_and_separates_control_effect(self):
        publish = load_publish()
        baseline = reading(commit="6c5d2d56", level="L2", calls=(80, 100),
                           throughput=1.92, runs=(0.75, 0.69, 0.65))
        current = reading(level="L3", calls=(93, 101), throughput=2.1,
                          runs=(2.0, 2.1, 2.4))
        control = copy.deepcopy(current)
        control["fathom_commit"] = self.source_commit[:12] + "-dirty-control"
        control["counts"]["call_fathom"] = 91
        control["tenancy"].update({"peak_module_pages": 9, "compiled_bytes": 44})
        current["counts"]["call_fathom"] = 99
        current["tenancy"].update({"peak_module_pages": 10, "compiled_bytes": 44})
        current["subjects"][0]["families"]["calls"]["tier"] = "A"
        current["subjects"][0]["tenancy"]["files_with_error"] = None
        current["subjects"][1]["families"]["calls"]["measured"] = False
        current["kind_intersection"].append("definition")
        del control["excluded_by_kind"]

        _, receipt = publish.sanitize_reading(current, self.source_commit)
        diff = self.build(baseline, current, control, receipt)
        paths = {row["path"]: row for row in diff["comparisons"]["control_to_main"]["fields"]}
        self.assertEqual(paths["$.counts.call_fathom"]["delta"], 8)
        self.assertEqual(paths["$.tenancy.peak_module_pages"]["delta"], 1)
        self.assertEqual(paths["$.subjects[0].families.calls.tier"]["change"], "changed")
        self.assertEqual(paths["$.subjects[0].tenancy.files_with_error"]["change"],
                         "type-changed")
        self.assertEqual(paths["$.subjects[1].families.calls.measured"]["after"], False)
        self.assertNotIn("delta", paths["$.subjects[1].families.calls.measured"])
        self.assertEqual(paths["$.kind_intersection[1]"]["change"], "added")
        self.assertEqual(paths["$.excluded_by_kind"]["change"], "added")
        self.assertEqual(set(diff["comparisons"]),
                         {"committed_to_main", "committed_to_control", "control_to_main"})
        self.assertEqual(diff["control_reading"]["counts"]["call_fathom"], 91)
        self.assertIn("causal", diff["comparisons"]["control_to_main"]["interpretation"])
        self.assertIn("not causal", diff["performance"]["comparison"])
        self.assertEqual(diff["source"]["control_patch_sha256"], "4" * 64)
        self.assertEqual(diff["source"]["control_oracle_inputs"],
                         {"input_files": 9, "input_bytes": 2048, "unchanged": True})

    def test_call_summaries_preserve_null_metrics_without_inventing_deltas(self):
        baseline = reading(commit="6c5d2d56")
        current = reading()
        control = reading(commit=self.source_commit[:12] + "-dirty-control")
        baseline["metrics"]["call_precision"] = None
        current["metrics"]["call_recall"] = None
        control["metrics"]["call_precision"] = None

        calls = self.build(baseline, current, control)["calls"]
        self.assertIsNone(calls["delta"]["precision"])
        self.assertIsNone(calls["delta"]["recall"])
        self.assertEqual(calls["change"], {
            "precision": "became-measured", "recall": "became-unmeasured"})
        self.assertIsNone(calls["control_to_main_delta"]["precision"])
        self.assertIsNone(calls["control_to_main_delta"]["recall"])
        self.assertEqual(calls["control_to_main_change"], {
            "precision": "became-measured", "recall": "became-unmeasured"})

    def test_recursive_diff_covers_missing_null_false_and_rejects_bad_json_values(self):
        publish = load_publish()
        rows = {row["path"]: row for row in publish.recursive_diff(
            {"value": None, "flag": True, "gone": 1},
            {"value": False, "flag": False, "new": None})["fields"]}
        self.assertEqual(rows["$.value"]["change"], "type-changed")
        self.assertEqual(rows["$.flag"]["change"], "changed")
        self.assertNotIn("delta", rows["$.flag"])
        self.assertEqual(rows["$.gone"]["change"], "removed")
        self.assertEqual(rows["$.new"]["change"], "added")
        for invalid in (float("nan"), float("inf"), object(), {1: "bad"}):
            with self.subTest(invalid=repr(invalid)), self.assertRaises(ValueError):
                publish.recursive_diff({}, {"bad": invalid})

    def test_control_receipt_requires_same_nine_oracle_inputs(self):
        publish = load_publish()
        for receipt in (
                {"input_files": 8, "input_bytes": 2048, "unchanged": True},
                {"input_files": 9, "input_bytes": 2048, "unchanged": False},
                {"input_files": 9, "input_bytes": 2048, "unchanged": True,
                 "opaque_hash": "uncheckable"}):
            with self.subTest(receipt=receipt), self.assertRaisesRegex(
                    ValueError, "control oracle-input receipt"):
                publish._validate_control_receipt(receipt)

    def test_diff_refuses_duplicate_subjects_changed_pins_and_wrong_revisions(self):
        publish = load_publish()
        baseline = reading(commit="6c5d2d56")
        for mutate, message in (
                (lambda main, control: main["subjects"].append(copy.deepcopy(main["subjects"][0])),
                 "unique subject names"),
                (lambda main, control: control["subjects"][0].update(sha="c" * 40),
                 "subject pins"),
                (lambda main, control: main.update(fathom_commit="f" * 7),
                 "main reading.*source commit"),
                (lambda main, control: control.update(fathom_commit="f" * 7),
                 "control reading.*source commit")):
            main, control = reading(), reading(commit=self.source_commit[:12] + "-dirty-control")
            mutate(main, control)
            with self.subTest(message=message), self.assertRaisesRegex(ValueError, message):
                self.build(baseline, main, control)

    def test_cli_requires_control_and_records_control_patch_identity(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            baseline = reading(commit="6c5d2d56")
            current = reading()
            baseline_path = root / "baseline.json"
            current_path = root / "current.json"
            control_path = root / "control.json"
            reading_out = root / "reading.json"
            diff_out = root / "diff.json"
            baseline_path.write_text(json.dumps(baseline))
            current_path.write_text(json.dumps(current))
            control = reading(commit=self.source_commit[:12] + "-dirty-control")
            control_path.write_text(json.dumps(control))
            command = [
                "python3", "-I", "-B", str(PUBLISH),
                "--baseline", str(baseline_path),
                "--regenerated", str(current_path),
                "--control", str(control_path),
                "--control-receipt", '{"input_files":9,"input_bytes":2048,"unchanged":true}',
                "--reading-out", str(reading_out),
                "--diff-out", str(diff_out),
                "--source-commit", "6db920e72898abb20bf86c22652b401902e34598",
                "--dxc-sha256", "a" * 64,
                "--required-glibc", "2.38",
                "--runner-glibc", "2.39",
                "--go-version", "go1.27.3",
                "--controller-commit", CONTROLLER_COMMIT,
                "--control-patch-sha256", "4" * 64,
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
            self.assertEqual(json.loads(diff_out.read_text())["source"]["control_patch_sha256"],
                             "4" * 64)
            missing_control = [arg for arg in command if arg not in ("--control", str(control_path))]
            result = subprocess.run(missing_control, text=True, stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("--control", result.stderr)


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

    def test_control_failure_keeps_details_in_private_log(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            scripts = root / "scripts"
            scripts.mkdir()
            (scripts / "control.py").write_text(
                "import sys\nprint('private-control-detail', file=sys.stderr)\nsys.exit(17)\n")
            log = root / "private.log"
            result = self.run_bash(
                f"REPLAY_LIBRARY_ONLY=1 source {REPLAY}; "
                f"run_control {scripts} {root} {root / 'patch'} {root / 'snapshot'} "
                f"{root / 'reading'} {root / 'receipt'} {log}")
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("HLSL control stage failed", result.stderr)
            self.assertNotIn("private-control-detail", result.stderr + result.stdout)
            self.assertIn("private-control-detail", log.read_text())

    def test_controller_identity_requires_expected_commit_and_committed_files(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            owned = root / "calibration/oracles/dxc-ast/scope-replay"
            owned.mkdir(parents=True)
            for filename in ("replay.sh", "publish.py", "control.py", "scope-control.patch",
                             "hlsl-scope-replay.yml"):
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
            for filename in ("replay.sh", "publish.py", "control.py", "scope-control.patch"):
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
    def test_push_read_only_bounded_x64_workflow_contract(self):
        spec = importlib.util.spec_from_file_location(
            "hlsl_workflow_contract", HERE / "test_workflow.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        module.validate_workflow(WORKFLOW.read_text())


if __name__ == "__main__":
    unittest.main()
