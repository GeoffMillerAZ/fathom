"""Publication invariants, independent of the measured outcome."""
import pathlib
import json
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import publish
import publish_reading
import prepare
import grade


class PublicationTests(unittest.TestCase):
    def test_generated_receipt_has_exact_public_contract_and_scored_artifact(self):
        """Synthetic measurements exercise the real publisher and its exact public schema."""
        root = pathlib.Path(__file__).resolve().parents[3]
        committed = json.loads((root / "docs/assessments/data/2026-10-26-kustomize-r1.json").read_text())
        expected_keys = {
            "D19_xref_denominators", "D19_xref_thin", "base_commit", "counts", "date",
            "go_runtime_env", "grade", "indexbench_local_artifact_sha256",
            "indexbench_source_identity", "lane_disposition", "leave_one_subject_out",
            "level_met", "load_at_launch_by_run", "median_reading_sha256", "median_run",
            "metric_tiers", "metrics", "native_bin_sha256", "niceness",
            "oracle_bin_sha256", "original_population_conservation_failed",
            "original_xref_floor_met", "original_xref_truth", "pins_sha256",
            "pool_xref_floor_met", "subjects", "throughput_mb_per_cpu_second",
        }
        fixed = prepare.PINS
        pin_digest = publish.digest(publish.PINS_PATH)
        artifact_digest = "a" * 64  # Synthetic local build artifact, not a subject hash.
        counts_by_subject = [12, 1, 0, 0, 0]
        pooled = {"xref_truth": 13, "xref_fathom": 13, "matched": 5}
        scored_subjects = []
        before_subjects = []
        root.joinpath("workspace/tmp").mkdir(parents=True, exist_ok=True)

        with tempfile.TemporaryDirectory(dir=root / "workspace/tmp") as path:
            cache = pathlib.Path(path)
            lane = cache / "kustomize-r1"
            native, manifests, graded = (lane / name for name in ("native", "manifests", "grade"))
            for folder in (native, manifests, graded):
                folder.mkdir(parents=True)
            for pin, xrefs in zip(fixed["subjects"], counts_by_subject):
                name = pin["repository"]
                stem = publish.slug(pin)
                truth_path = native / (stem + "-before.json")
                files_path = native / (stem + "-before.files")
                receipt_path = native / (stem + "-before.receipt")
                truth = {"definitions": [{"name": "synthetic"}],
                         "references": [{} for _ in range(xrefs)],
                         "cross_references": [{"to_path": "synthetic.yaml"}
                                              for _ in range(xrefs)]}
                receipt = {"binary_sha256": "native-tool", "files": 1,
                           "built_roots": 1, "typed_entries": 0, "excluded": {},
                           "inventory": {}, "stats": {}, "component_contexts": {}}
                truth_path.write_text(json.dumps(truth))
                files_path.write_text("synthetic fixture\n")
                receipt_path.write_text(json.dumps(receipt))
                for suffix in (".json", ".files", ".receipt"):
                    (native / (stem + "-after" + suffix)).write_bytes(
                        (native / (stem + "-before" + suffix)).read_bytes())
                manifest = {"sha256": "synthetic-manifest", "entries": 1, "bytes": 1}
                for when in ("before", "after"):
                    (manifests / (stem + "-" + when + ".json")).write_text(json.dumps(manifest))
                before_subjects.append({"repository": name, "subject_sha": pin["sha"],
                                        "oracle_sha256": publish.digest(truth_path),
                                        "files_sha256": publish.digest(files_path),
                                        "receipt_sha256": publish.digest(receipt_path),
                                        "native_receipt": receipt, "definitions": 1,
                                        "references": xrefs, "cross_references": xrefs,
                                        "built_files": 1})
                scored_subjects.append({"repository": name, "sha": pin["sha"],
                                        "files": 1, "bytes": 1,
                                        "unsupported_native_basename": 0,
                                        "counts": {"xref_truth": xrefs,
                                                   "xref_fathom": xrefs, "matched": 1},
                                        "ratios": {}})

            native_summary = {"pins_sha256": pin_digest, "oracle_bin_sha256": "oracle-tool",
                              "native_bin_sha256": "native-tool", "subjects": before_subjects}
            for when in ("before", "after"):
                (native / (when + "-summary.json")).write_text(json.dumps(native_summary))
            median = {"counts": pooled, "metrics": {}, "metric_tiers": {},
                      "grade": {"L1": "good"}, "level_met": "L1"}
            median_path = graded / "median.json"
            median_path.write_text(json.dumps(median))
            loo = []
            for subject in scored_subjects:
                omitted = subject["counts"]
                loo.append({"omitted_repository": subject["repository"],
                            "counts": {key: value - omitted.get(key, 0)
                                       for key, value in pooled.items()},
                            "metrics": {}, "grade": {}, "level_met": "none"})
            scored = {**median, "reading_sha256": publish.digest(median_path),
                      "base_commit": fixed["base_commit"], "pins_sha256": pin_digest,
                      "indexbench_sha256": artifact_digest, "median_run": 2,
                      "runs_mb_per_cpu_second": [1.0, 2.0, 3.0],
                      "subjects": scored_subjects, "leave_one_subject_out": loo,
                      "D19_xref_denominators": {"precision": 13, "recall": 13},
                      "original_xref_truth": {
                          fixed["subjects"][0]["repository"]: 12,
                          fixed["subjects"][1]["repository"]: 1},
                      "original_xref_floor_met": True, "pool_xref_floor_met": True,
                      "niceness": 10, "go_runtime_env": {"GOMAXPROCS": "2"},
                      "load_at_launch_by_run": []}
            (graded / "summary.json").write_text(json.dumps(scored))
            output = cache / "public.json"
            with mock.patch.object(sys, "argv", ["publish", "--cache", str(cache),
                                                  "--out", str(output)]):
                publish.main()
            public = json.loads(output.read_text())
            self.assertEqual(set(public), expected_keys)
            self.assertEqual(set(committed), expected_keys)
            self.assertEqual(public["indexbench_local_artifact_sha256"],
                             scored["indexbench_sha256"])
            self.assertNotIn("local_artifact_privacy", public)

    def test_reading_rejects_artifact_digest_different_from_scored_summary(self):
        root = pathlib.Path(__file__).resolve().parents[3]
        root.joinpath("workspace/tmp").mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=root / "workspace/tmp") as path:
            cache = pathlib.Path(path)
            grade_dir = cache / "kustomize-r1" / "grade"
            grade_dir.mkdir(parents=True)
            median_path = grade_dir / "median.json"
            public_path, current_path = cache / "public.json", cache / "current.json"
            median_path.write_text(json.dumps({"fathom_commit": "synthetic-base"}))
            (grade_dir / "summary.json").write_text(json.dumps({"indexbench_sha256": "a" * 64}))
            public_path.write_text(json.dumps({"pins_sha256": "synthetic-pins",
                                               "median_reading_sha256": publish.digest(median_path),
                                               "base_commit": "synthetic-base",
                                               "indexbench_source_identity": {
                                                   "vcs_revision": "synthetic-base",
                                                   "vcs_modified": False},
                                               "indexbench_local_artifact_sha256": "b" * 64}))
            current_path.write_text("{}")
            actual_digest = publish_reading.digest

            def fake_digest(path):
                return "synthetic-pins" if path == publish_reading.PINS_PATH else actual_digest(path)

            with mock.patch.object(publish_reading, "pins",
                                   return_value={"base_commit": "synthetic-base"}), \
                    mock.patch.object(publish_reading, "digest", side_effect=fake_digest):
                with self.assertRaisesRegex(RuntimeError, "indexbench.*artifact|artifact.*indexbench"):
                    publish_reading.verify(median_path, public_path, current_path, cache)

    def test_leave_one_out_uses_harness_regrade(self):
        median = {"subjects": [{"name": "one", "counts": {"matched": 3}},
                               {"name": "two", "counts": {"matched": 4}}]}
        pins = [{"repository": "one"}, {"repository": "two"}]
        called = []

        def fake_regrade(cmd, check, cwd, stdout, stderr):
            self.assertEqual(cmd[1:4], ["lang-regrade", "-commit", grade.BASE_COMMIT])
            reading = json.loads(pathlib.Path(cmd[4]).read_text())
            remaining = reading["subjects"][0]
            called.append(remaining["name"])
            reading.update({"counts": remaining["counts"], "metrics": {"sym_precision": 0.1},
                            "grade": {"L1": "unmet"}, "level_met": "none"})
            pathlib.Path(cmd[4]).write_text(json.dumps(reading))

        root = pathlib.Path(__file__).resolve().parents[3]
        (root / "workspace/tmp").mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=root / "workspace/tmp") as path:
            with mock.patch.object(grade.subprocess, "run", side_effect=fake_regrade):
                rows = grade.regrade_leave_one_out(pathlib.Path("indexbench"), pathlib.Path(path),
                                                    pathlib.Path(path), median, pins)
        self.assertEqual(called, ["two", "one"])
        self.assertEqual([row["omitted_repository"] for row in rows], ["one", "two"])
        self.assertEqual([row["counts"]["matched"] for row in rows], [4, 3])
        self.assertEqual([row["level_met"] for row in rows], ["none", "none"])

    def test_publisher_requires_every_pinned_leave_one_out(self):
        subject_pins = prepare.PINS["subjects"]
        rows = [{"omitted_repository": pin["repository"],
                 "counts": {"matched": 4}, "metrics": {"sym_precision": 1.0},
                 "grade": {"L1": "good"}, "level_met": "L3"}
                for pin in subject_pins]
        scored = {"counts": {"matched": 5},
                  "subjects": [{"repository": pin["repository"], "counts": {"matched": 1}}
                               for pin in subject_pins],
                  "leave_one_subject_out": rows}
        self.assertEqual(publish.checked_leave_one_out(scored, subject_pins), rows)
        with self.assertRaisesRegex(RuntimeError, "leave-one-out"):
            publish.checked_leave_one_out({**scored, "leave_one_subject_out": rows[:-1]},
                                          subject_pins)

    def test_reading_refuses_out_of_bounds_machine_conditions(self):
        def metadata(niceness=10, load=9):
            return {"niceness": niceness, "go_runtime_env": {"GOMAXPROCS": "2"},
                    "load_at_launch_by_run": [[{"repository": "subject", "load_at_launch": load}]
                                              for _ in range(3)]}

        publish_reading.verify_operational_conditions(metadata(), {"subject"})
        for niceness, load in ((9, 9), (10, 9.01), (10, -0.01)):
            with self.subTest(niceness=niceness, load=load), self.assertRaises(RuntimeError):
                publish_reading.verify_operational_conditions(metadata(niceness, load),
                                                              {"subject"})

    def test_native_archive_pins_cover_supported_hosts(self):
        pin = prepare.PINS["native"]
        for system, machine, suffix, key in (
            ("Darwin", "arm64", "darwin_arm64", "archive_sha256"),
            ("Linux", "x86_64", "linux_amd64", "linux_amd64_archive_sha256"),
            ("Linux", "aarch64", "linux_arm64", "linux_arm64_archive_sha256"),
        ):
            with self.subTest(system=system, machine=machine):
                asset, checksum, url = prepare.native_asset(pin, system, machine)
                self.assertTrue(asset.endswith(suffix + ".tar.gz"))
                self.assertEqual(checksum, pin[key])
                self.assertTrue(url.endswith("/" + asset))

    def test_committed_receipt_golden_outcome(self):
        root = pathlib.Path(__file__).resolve().parents[3]
        receipt = json.loads((root / "docs/assessments/data/2026-10-26-kustomize-r1.json").read_text())
        self.assertEqual(receipt["throughput_mb_per_cpu_second"], [11.06, 12.92, 12.91])
        self.assertEqual((sum(row["native_files"] for row in receipt["subjects"]),
                          sum(row["graded_files"] for row in receipt["subjects"])), (58, 46))
        self.assertEqual((receipt["counts"]["xref_fathom"],
                          receipt["counts"]["xref_truth"]), (35, 35))
        self.assertEqual((receipt["counts"]["fathom_defs"],
                          receipt["counts"]["oracle_defs"]), (202, 194))
        self.assertEqual(receipt["grade"]["L1"], "good")
        self.assertEqual(receipt["level_met"], "L3")

    def test_thin_denominators_are_publishable_and_disclosed(self):
        for precision, recall in ((19, 20), (20, 19), (0, 0), (20, 20)):
            with self.subTest(precision=precision, recall=recall):
                result = publish.d19_xref_disposition(
                    {"xref_fathom": precision, "xref_truth": recall},
                    {"precision": precision, "recall": recall})
                self.assertEqual(result["thin"], min(precision, recall) < 20)
        with self.assertRaisesRegex(RuntimeError, "differ"):
            publish.d19_xref_disposition({"xref_fathom": 19, "xref_truth": 20},
                                          {"precision": 20, "recall": 20})

    def test_original_population_floor_failure_is_published_held(self):
        for original_truth, pool_xrefs in (
            ({"kubernetes-sigs/kustomize": 11, "argoproj/argocd-example-apps": 1}, 35),
            ({"kubernetes-sigs/kustomize": 12, "argoproj/argocd-example-apps": 0}, 35),
            ({"kubernetes-sigs/kustomize": 11, "argoproj/argocd-example-apps": 1}, 12),
        ):
            with self.subTest(truth=original_truth, pool=pool_xrefs):
                pooled = pool_xrefs >= 13
                scored = {"original_xref_floor_met": False,
                          "pool_xref_floor_met": pooled,
                          "original_xref_truth": original_truth,
                          "counts": {"xref_truth": pool_xrefs}}
                disposition = publish.population_disposition(scored, d19_thin=False)
                self.assertTrue(disposition["original_population_conservation_failed"])
                self.assertEqual(disposition["lane_disposition"], "held")
                self.assertFalse(disposition["original_xref_floor_met"])
                self.assertEqual(disposition["pool_xref_floor_met"], pooled)
                with self.assertRaisesRegex(RuntimeError, "differs from measured counts"):
                    publish.population_disposition({**scored, "original_xref_floor_met": True},
                                                    d19_thin=False)
        self.assertEqual(publish.population_disposition(
            {"original_xref_floor_met": True, "pool_xref_floor_met": True,
             "original_xref_truth": {"kubernetes-sigs/kustomize": 12,
                                     "argoproj/argocd-example-apps": 1},
             "counts": {"xref_truth": 13}},
            d19_thin=False)["lane_disposition"], "count_guard_met_pending_D19")

    def test_notes_preserve_scope_and_describe_thin_result(self):
        median = {"notes": ["native built-file scope via -only", "oracle scope"]}
        public = {"D19_xref_denominators": {"precision": 19, "recall": 20},
                  "D19_xref_thin": True, "base_commit": "abc", "counts": {
                      "fathom_defs": 10, "oracle_defs": 9, "matched": 9,
                      "ref_truth": 5, "ref_truth_found": 5,
                      "xref_truth": 20, "xref_truth_found": 20,
                      "xref_fathom": 19, "xref_fathom_in_truth": 19},
                  "grade": {"L1": "good", "L2": "good", "L3": "excellent"},
                  "level_met": "L3", "native_bin_sha256": "native",
                  "original_population_conservation_failed": False,
                  "leave_one_subject_out": [{"omitted_repository": "subject",
                                             "counts": {"xref_fathom": 19, "xref_truth": 20},
                                             "metrics": {"sym_precision": 1.0},
                                             "grade": {"L1": "unmet"}, "level_met": "none"}],
                  "subjects": [
                      {"native_files": 4, "graded_files": 3, "component_contexts": 0,
                       "component_context_counts": {"valid": 0, "tried": 0,
                                                    "effective_entries": 0},
                       "native_excluded": {"remote": 1},
                       "native_cross_references": 25, "native_yaml_target_cross_references": 4,
                       "native_stats": {"noop": 1},
                       "graded_counts": {"matched": 9, "fathom_defs": 10,
                                         "xref_fathom": 19, "xref_truth": 20}}],
                  "throughput_mb_per_cpu_second": [1.0, 2.0, 3.0],
                  "median_run": 2, "niceness": 10, "go_runtime_env": {
                      "GOTOOLCHAIN": "go1.27.3", "GOMAXPROCS": "2", "GOFLAGS": "-p=2"}}
        notes = publish_reading.merge_notes(median["notes"], publish_reading.notes(public))
        self.assertEqual(notes[:2], median["notes"])
        self.assertIn("thin", " ".join(notes).lower())
        self.assertIn("3 of 4", " ".join(notes))
        self.assertNotIn("35/35", " ".join(notes))
        self.assertIn("symbol precision 1.0, level_met none", " ".join(notes))
        self.assertIn("4 YAML-target references outside dialect file scope (manifests and patches). "
                      "Manifest-target edges resolve to nothing at ingest until the kubernetes dialect is dispatched",
                      " ".join(notes))
        self.assertNotIn("This is the lane's guard pending D19's lang-regrade", " ".join(notes))
        public["original_population_conservation_failed"] = True
        held_notes = " ".join(publish_reading.notes(public))
        self.assertIn("conservation failed; lane held", held_notes)
        self.assertNotIn("clears the lane's count guard", held_notes)


if __name__ == "__main__":
    unittest.main()
