"""COV-M1/M2 controls using synthetic rows and authorization records."""
from concurrent.futures import ThreadPoolExecutor
import copy
import json
from pathlib import Path
from runpy import run_path
import tempfile
import unittest

final = run_path(str(Path(__file__).with_name("cov_final.py")))


class FinalControls(unittest.TestCase):
    def test_exclusive_journal(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "initial.json"
            def acquire(_):
                try:
                    final["claim"](path, {"status": "started"})
                    return True
                except FileExistsError:
                    return False
            with ThreadPoolExecutor(4) as workers:
                self.assertEqual(sum(workers.map(acquire, range(8))), 1)
            self.assertEqual(json.loads(path.read_bytes()), {"status": "started"})

    def test_cohort_separation_and_pins(self):
        cohorts = final["cohorts"]()
        self.assertEqual({k: len(v) for k, v in cohorts.items()},
                         {"development": 10, "holdout": 5, "confirmation": 1})
        for pool, name in [("development", "django"), ("confirmation", "express"),
                           ("holdout", "blueprint"), ("holdout", "ad-hoc")]:
            with self.assertRaises(ValueError):
                final["subject"](pool, name)
        self.assertEqual(final["subject"]("confirmation", "blueprint")["files"], 979)

    def test_comparison_and_overlap(self):
        old = {"path": "a.py", "edges": [], "gap": ""}
        gap = {"path": "b.py", "edges": [], "gap": "original-gap"}
        initializer = {"Kind": "import", "Raw": "pkg", "Line": 1,
                       "Resolution": "file", "To": "pkg/__init__.py"}
        extension = dict(gap, gap="", edges=[initializer])
        names = ["a.py", "b.py"]
        tracked = set(names + ["pkg/__init__.py", "pkg/child.py"])
        graph = {"b.py": {"pkg/__init__.py", "pkg/child.py"}}
        sources = {"a.py": b"", "b.py": b"from pkg import child\n"}
        args = (names, [old, gap], [old, extension], graph, sources.__getitem__, ["."], tracked)
        report = final["compare_python"](*args)
        self.assertEqual(report["b.py"]["class"], "initializer-plus-submodule")
        self.assertEqual(report["b.py"]["truth"], ["pkg/__init__.py", "pkg/child.py"])
        sources["b.py"] *= 2
        report = final["compare_python"](*args)
        self.assertEqual(report["b.py"]["class"], "ambiguous-site")
        self.assertIsNone(report["b.py"]["truth"])
        changed = copy.deepcopy(old)
        changed["edges"].append(initializer)
        with self.assertRaises(ValueError):
            final["compare_python"](names, [old, gap], [changed, extension], *args[3:])

    def test_correction_never_acquires_confirmation(self):
        self.assertEqual(final["operation_pools"]("initial"), ["development", "holdout", "confirmation"])
        self.assertEqual(final["operation_pools"]("correction"), ["development", "holdout"])
        with self.assertRaises(ValueError):
            final["operation_pools"]("preflight")

    def test_active_operation_is_bound_to_pool_and_nonce(self):
        final["active"]({"mode": "preflight"}, "development")
        with self.assertRaises(ValueError):
            final["active"]({"mode": "preflight"}, "holdout")
        with self.assertRaises(ValueError):
            final["active"]({"mode": "preflight", "journal": "synthetic"}, "development")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            journal = root / "journal"
            journal.mkdir(mode=0o700)
            claim = journal / "correction.json"
            config = dict(mode="correction", root=str(root), journal=str(claim), binding="synthetic",
                          nonce="synthetic", python="python", compiler="compiler", pyright="pyright")
            tools = {key: config[key] for key in ("python", "compiler", "pyright")}
            record = dict(operation="correction", status="started", root=str(root), binding="synthetic",
                          nonce_sha256=final["sha"](b"synthetic"), tool_paths=tools,
                          pools=["development", "holdout"])
            final["claim"](claim, record)
            final["active"](config, "holdout")
            for change in ({"nonce": "wrong"}, {"binding": "wrong"}, {"pyright": "ad-hoc"}):
                with self.assertRaises(ValueError):
                    final["active"](dict(config, **change), "holdout")
            with self.assertRaises(ValueError):
                final["active"](config, "confirmation")
            final["finish"](claim, dict(record, status="failed"))
            with self.assertRaises(ValueError):
                final["active"](config, "holdout")
            with self.assertRaises(FileExistsError):
                final["claim"](claim, record)

    def test_authorization_requires_exact_green_evidence(self):
        reader, head = "a" * 40, "b" * 40
        root, pinned = Path("/synthetic-campaign"), {"pins": "synthetic"}
        auth = dict(authority=final["AUTHORITY"], operation="initial", status="authorized",
                    root=str(root), identity=pinned, reader_revision=reader, harness_revision=head,
                    linux_ci=dict(revision=reader, status="success", receipt="synthetic-ci"),
                    processed_acks=dict(revision=reader, receipt="synthetic-acks"),
                    seal_review=dict(revision=head, reviewer="synthetic", receipt="synthetic-review"))
        final["authorized"](auth, root, "initial", pinned, head)
        for change in ({"harness_revision": reader}, {"identity": {}},
                       {"linux_ci": dict(auth["linux_ci"], status="pending")},
                       {"processed_acks": dict(auth["processed_acks"], receipt="")},
                       {"seal_review": dict(auth["seal_review"], revision=reader)}):
            with self.assertRaises(ValueError):
                final["authorized"](dict(auth, **change), root, "initial", pinned, head)

    def test_program_bar_uses_the_original_denominators(self):
        def row(complete, files, passed=True):
            return {"score": dict(Complete=complete, Files=files, B1=passed, B2=passed, B3=True,
                                  CompleteWithoutOracle=0), "closure": {"Selected": files, "Complete": 0},
                    "dependency_evidence_sha256": "synthetic"}
        development = {dialect: row(200, files)
                       for dialect, (_, files) in final["ORIGINAL_DEVELOPMENT"].items()}
        result = {"development": development,
                  "holdout": {"python": row(1590, 2820), "javascript": row(254, 317),
                              "typescript": row(1002, 2077)},
                  "confirmation": {"typescript": row(979, 979)}}
        for pool, files in final["ORIGINAL_PYTHON_TRUTH"].items():
            result[pool]["python"]["native_classes"] = {"original/exact": files}
        corrected = {"javascript": {"complete": 112}, "typescript": {"complete": 501}}
        report = final["assess"](result, ({}, corrected))
        self.assertTrue(report["program"]["met"])
        self.assertEqual(report["verdicts"]["javascript"]["label"], "sound")
        self.assertEqual(report["verdicts"]["python"]["label"], "sound, low coverage, improved")
        self.assertFalse(report["verdicts"]["typescript"]["doubled"])
        self.assertEqual(report["secondary_r5"]["typescript"]["ratio"], 2)
        result["holdout"]["python"]["score"]["B2"] = False
        self.assertFalse(final["assess"](result, ({}, corrected))["program"]["met"])
        initial = final["assess"](result, ({}, corrected))
        correction = copy.deepcopy(result)
        del correction["confirmation"]
        corrected_result = final["assess"](correction, ({}, corrected), initial)
        self.assertNotIn("confirmation", corrected_result["current"])
        self.assertEqual(corrected_result["verdicts"]["python"]["label"], "reverted")
        correction["holdout"]["python"]["dependency_evidence_sha256"] = "changed"
        with self.assertRaises(ValueError):
            final["assess"](correction, ({}, corrected), initial)


if __name__ == "__main__":
    unittest.main()
