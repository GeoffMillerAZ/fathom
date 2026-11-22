"""COV-M1/M2 controls using synthetic rows and authorization records."""
from concurrent.futures import ThreadPoolExecutor
from contextlib import redirect_stdout
import copy
import fcntl
from io import StringIO
import json
import os
from pathlib import Path
from runpy import run_path
import signal
import stat
import subprocess
import sys
import tempfile
import time
from types import SimpleNamespace
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve()
final = run_path(str(SCRIPT.with_name("cov_final.py")))
scope = final["run"].__globals__


def authorization(root, reader, harness):
    return dict(authority=final["AUTHORITY"], operation="initial", status="authorized",
                root=str(root), identity={}, reader_revision=reader, harness_revision=harness,
                linux_ci=dict(revision=reader, status="success", receipt="synthetic-ci"),
                processed_acks=dict(revision=reader, receipt="synthetic-acks"),
                seal_review=dict(revision=harness, reviewer="synthetic", receipt="synthetic-review"),
                baselines={})


def write_authorization(root, reader, harness):
    journal = root / "journal"
    journal.mkdir(mode=0o700)
    final["claim"](journal / "initial.authorization.json", authorization(root, reader, harness))
    return journal


def initial_args(root):
    return SimpleNamespace(root=root, operation="initial", compiler="compiler", pyright="pyright")


LANDED_READER = "59ef2b56bf979f5f4ed91ea7182338f30a63c9b9"
FAILED_HARNESS = "598ec9ded011ded5b93fa04805c07c168226c21f"
CANDIDATE = "b" * 40


def replacement_authorization(root):
    journal = root / "journal"
    journal.mkdir(mode=0o700)
    old = root.parent / "original"
    (old / "journal").mkdir(mode=0o700, parents=True)
    failed = old / "journal/initial.json"
    final["claim"](failed, dict(operation="initial", status="failed", root=str(old),
                               reader_revision=LANDED_READER, harness_revision=FAILED_HARNESS))
    preflight = journal / "metadata-preflight.txt"
    preflight.write_bytes(b"pin-check PASS\npopulation-check PASS\n")
    preflight.chmod(0o600)
    return dict(authorization(root, LANDED_READER, CANDIDATE), operation="replacement",
                authority="COV-D2/ESCALATE-23",
                failed_initial=dict(path=str(failed), sha256=final["sha"](failed.read_bytes())),
                metadata_preflight=dict(path=str(preflight), sha256=final["sha"](preflight.read_bytes())))


def synthetic_attempt(root):
    repo = root / "repo"
    overrides = dict(REPO=repo, identity=lambda *_: {}, ensure_reader=lambda *_: None,
                     git=lambda *_: b"b" * 40, baselines=lambda *_: None)
    with patch.dict(scope, overrides), patch.dict(os.environ, {"PATH": str(root)}):
        try:
            final["run"](initial_args(root))
        except FileExistsError:
            print("replay denied")
        except (Exception, KeyboardInterrupt):
            print("operation failed")
        else:
            raise AssertionError("synthetic failure unexpectedly published")


class FinalControls(unittest.TestCase):
    def test_rehearsal_claims_assesses_and_publishes_twice(self):
        def row(files, original_truth=None):
            value = {"score": dict(Complete=1, Files=files, B1=True, B2=True, B3=True,
                                   CompleteWithoutOracle=0), "closure": {"Selected": files, "Complete": 0}}
            if original_truth is not None:
                value["native_classes"] = {"original/exact": original_truth}
            return value

        development = {"python": row(1314, 156), "javascript": row(1508), "typescript": row(1224)}
        results = dict(development=development, holdout=development, confirmation={"typescript": row(204)})
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp).resolve()
            parent = repo / "workspace/tmp/cov-final-rehearsal"
            parent.mkdir(parents=True)
            published = []
            for run in ("run-one", "run-two"):
                root = parent / run
                root.mkdir(mode=0o700)
                journal = root / "journal"
                journal.mkdir(mode=0o700)

                def worker(config, _check):
                    self.assertEqual(config["mode"], "rehearse")
                    final["active"](config, "holdout")
                    return results

                with redirect_stdout(StringIO()), patch.dict(scope, REPO=repo, git=lambda *_: b"b" * 40,
                                identity=lambda *_: {}, ensure_reader=lambda *_: None, measure=worker,
                                baselines=lambda *_: self.fail("rehearsal opened production baselines")):
                    final["rehearse"](initial_args(root))
                    with self.assertRaises(FileExistsError):
                        final["rehearse"](initial_args(root))
                aggregate = (journal / "rehearse.aggregate.json").read_bytes()
                record = json.loads((journal / "rehearse.json").read_bytes())
                self.assertEqual(record["status"], "completed")
                self.assertEqual(record["aggregate_sha256"], final["sha"](aggregate))
                self.assertEqual(json.loads(aggregate)["schema"], "cov-final-rehearsal/v1")
                self.assertIn({"stage": "assessment", "pool": "holdout"}, record["progress"])
                published.append(aggregate)
            self.assertEqual(*published)

    def test_rehearsal_selectors_and_frozen_truth_are_confined(self):
        def forbidden():
            self.fail("rehearsal consulted the production cohorts")

        with patch.dict(scope, cohorts=forbidden):
            development = final["rehearsal_subjects"]("development")
            self.assertEqual(len(development), 10)
            self.assertEqual(final["rehearsal_subjects"]("holdout"), development)
            confirmation, = final["rehearsal_subjects"]("confirmation")
            self.assertEqual((confirmation["name"], confirmation["files"]), ("fp-ts", 204))
            for pool, name in (("holdout", "ad-hoc"), ("other", "fp-ts"), ("confirmation", "rxjs")):
                with self.subTest(pool=pool, name=name), self.assertRaises(ValueError):
                    final["subject"](pool, name, "rehearse")
        for mode in ("rehearse", "initial"):
            with self.subTest(mode=mode), self.assertRaises(ValueError):
                final["validate_python_overlap"]({"native_classes": {"original/exact": 155}}, "holdout", mode)
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp).resolve()
            rehearsal = repo / "workspace/tmp/cov-final-rehearsal"
            rehearsal.mkdir(parents=True)
            unrelated = repo / "unrelated"
            unrelated.mkdir()
            alias = rehearsal / "run-alias"
            alias.symlink_to(unrelated, target_is_directory=True)
            with patch.dict(scope, REPO=repo, identity=lambda *_: self.fail("tools accessed before root validation")):
                for root in (unrelated, alias, rehearsal / "../unrelated"):
                    with self.subTest(root=root), self.assertRaises(ValueError):
                        final["rehearse"](initial_args(root))
        with self.assertRaises(ValueError):
            final["run"](SimpleNamespace(operation="rehearse"))

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

    def test_correction_is_disabled(self):
        self.assertEqual(final["operation_pools"]("initial"), ["development", "holdout", "confirmation"])
        for mode in ("correction", "preflight"):
            with self.subTest(mode=mode), self.assertRaises(ValueError):
                final["operation_pools"](mode)

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
            claim = journal / "initial.json"
            config = dict(mode="initial", root=str(root), journal=str(claim), binding="synthetic",
                          nonce="synthetic", python="python", compiler="compiler", pyright="pyright")
            tools = {key: config[key] for key in ("python", "compiler", "pyright")}
            record = dict(operation="initial", status="started", root=str(root), binding="synthetic",
                          nonce_sha256=final["sha"](b"synthetic"), tool_paths=tools,
                          pools=["development", "holdout", "confirmation"])
            final["claim"](claim, record)
            final["active"](config, "holdout")
            for change in ({"nonce": "wrong"}, {"binding": "wrong"}, {"pyright": "ad-hoc"}):
                with self.assertRaises(ValueError):
                    final["active"](dict(config, **change), "holdout")
            with self.assertRaises(ValueError):
                final["active"](config, "ad-hoc")
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

    def test_replacement_requires_bound_private_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve() / "replacement"
            root.mkdir()
            auth = replacement_authorization(root)
            failed = Path(auth["failed_initial"]["path"])
            original = failed.read_bytes()
            preflight = Path(auth["metadata_preflight"]["path"])
            passed = preflight.read_bytes()
            self.assertEqual(scope.get("FAILED_INITIAL_SHA256"),
                             "91b36b4a463cfe1fde13b519abc3f8f9cbca5cc3ef02ea77c1c423dce59f8bf7")

            def authorize(record=auth, mode="replacement"):
                return final["authorized"](record, root, mode, {}, CANDIDATE)

            with patch.dict(scope, FAILED_INITIAL_SHA256=auth["failed_initial"]["sha256"]):
                authorize()
                for change in ({"authority": final["AUTHORITY"]}, {"operation": "initial"},
                               {"harness_revision": LANDED_READER},
                               {"linux_ci": dict(auth["linux_ci"], status="pending")},
                               {"processed_acks": dict(auth["processed_acks"], receipt="")},
                               {"seal_review": dict(auth["seal_review"], revision=LANDED_READER)}):
                    with self.subTest(change=change), self.assertRaises(ValueError):
                        authorize(dict(auth, **change))
                with self.assertRaises(ValueError):
                    authorize(auth, "initial")
                with self.assertRaises(ValueError):
                    authorize(dict(auth, reader_revision=CANDIDATE,
                                   linux_ci=dict(auth["linux_ci"], revision=CANDIDATE),
                                   processed_acks=dict(auth["processed_acks"], revision=CANDIDATE)))
                for field in ("failed_initial", "metadata_preflight"):
                    for change in ({}, {"path": auth[field]["path"], "sha256": "0" * 64},
                                   dict(auth[field], extra="unexpected")):
                        with self.subTest(field=field, change=change), self.assertRaises(ValueError):
                            authorize(dict(auth, **{field: change}))
                    with self.assertRaises(ValueError):
                        authorize({key: value for key, value in auth.items() if key != field})
                    path = Path(auth[field]["path"])
                    path.chmod(0o640)
                    with self.assertRaises(ValueError):
                        authorize()
                    path.chmod(0o600)
                aggregate = failed.with_name("initial.aggregate.json")
                aggregate.symlink_to(failed.with_name("missing"))
                with self.assertRaises(ValueError):
                    authorize()
                aggregate.unlink()
                failed.write_bytes(original + b" ")
                with self.assertRaises(ValueError):
                    authorize()
                failed.write_bytes(original)
                preflight.write_bytes(passed + b" ")
                with self.assertRaises(ValueError):
                    authorize()
                preflight.write_bytes(passed)
                other = preflight.with_name("other.txt")
                preflight.rename(other)
                with self.assertRaises(ValueError):
                    authorize(dict(auth, metadata_preflight=dict(auth["metadata_preflight"], path=str(other))))
                preflight.symlink_to(other)
                with self.assertRaises(ValueError):
                    authorize()
                preflight.unlink()
                other.rename(preflight)
                same_campaign = preflight.with_name("initial.json")
                same_campaign.write_bytes(original)
                same_campaign.chmod(0o600)
                with self.assertRaisesRegex(ValueError, "different campaign root"):
                    authorize(dict(auth, failed_initial=dict(auth["failed_initial"], path=str(same_campaign))))

            for change in ({"operation": "replacement"}, {"status": "started"}, {"status": "completed"},
                           {"harness_revision": CANDIDATE}, {"reader_revision": CANDIDATE},
                           {"aggregate_sha256": "published"}, {"root": str(root)}):
                with self.subTest(change=change):
                    data = final["canonical"](dict(json.loads(original), **change))
                    failed.write_bytes(data)
                    digest = final["sha"](data)
                    with patch.dict(scope, FAILED_INITIAL_SHA256=digest), self.assertRaises(ValueError):
                        authorize(dict(auth, failed_initial=dict(auth["failed_initial"], sha256=digest)))
            failed.write_bytes(original)
            with patch.dict(scope, FAILED_INITIAL_SHA256=auth["failed_initial"]["sha256"]):
                for evidence in (b"", b"\n", b"pin FAIL\n", b"pin PASS\nsecond FAIL\n",
                                 b"pin PASS raw evidence\n", b"pin PASS\n\n"):
                    with self.subTest(evidence=evidence):
                        preflight.write_bytes(evidence)
                        ref = dict(auth["metadata_preflight"], sha256=final["sha"](evidence))
                        with self.assertRaises(ValueError):
                            authorize(dict(auth, metadata_preflight=ref))

    def test_replacement_uses_production_cohorts_and_journal(self):
        self.assertEqual(final["operation_pools"]("replacement"), final["operation_pools"]("initial"))
        for pool, pins in final["cohorts"]().items():
            for pin in pins:
                self.assertEqual(final["subject"](pool, pin["name"], "replacement"), pin)
        with self.assertRaises(ValueError):
            final["subject"]("confirmation", "fp-ts", "replacement")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve() / "replacement"
            root.mkdir()
            auth = replacement_authorization(root)
            journal = root / "journal"
            path = journal / "replacement.json"
            argv = [str(SCRIPT.with_name("cov_final.py")), "run", "--compiler", "compiler",
                    "--pyright", "pyright", str(root), "replacement"]

            def worker(config, _check):
                self.assertEqual(config["mode"], "replacement")
                record = final["active"](config, "holdout")
                self.assertEqual(record["failed_initial"], auth["failed_initial"])
                self.assertEqual(record["metadata_preflight"], auth["metadata_preflight"])
                with self.assertRaises(ValueError):
                    final["active"](dict(config, mode="initial"), "holdout")
                return {}

            with redirect_stdout(StringIO()), patch.object(sys, "argv", argv), patch.dict(
                    scope, FAILED_INITIAL_SHA256=auth["failed_initial"]["sha256"],
                    identity=lambda *_: {}, ensure_reader=lambda *_: None,
                    git=lambda *_: CANDIDATE.encode(), baselines=lambda *_: None, measure=worker,
                    assess=lambda _, __, *, mode, report: {"mode": mode}):
                auth_path = journal / "replacement.authorization.json"
                final["claim"](auth_path, dict(auth, metadata_preflight={}))
                with self.assertRaises(ValueError):
                    final["main"]()
                self.assertFalse(path.exists())
                auth_path.write_bytes(final["canonical"](auth))
                final["main"]()
                with self.assertRaises(FileExistsError):
                    final["main"]()

            record = json.loads(path.read_bytes())
            self.assertEqual(record["status"], "completed")
            self.assertEqual(record["binding"], final["sha"](auth_path.read_bytes()))
            for field in ("failed_initial", "metadata_preflight"):
                self.assertEqual(record[field], auth[field])
            self.assertEqual(json.loads((journal / "replacement.aggregate.json").read_bytes()),
                             {"mode": "replacement"})
            self.assertFalse((journal / "initial.json").exists())

    def test_worker_stderr_is_private_on_success_and_failure(self):
        for mode in ("rehearse", "replacement"):
            for outcome in ("success", "failure", "signal", "occupied"):
                with self.subTest(mode=mode, outcome=outcome), tempfile.TemporaryDirectory() as tmp:
                    root = Path(tmp).resolve()
                    repo = root / "repo"
                    (repo / "pkg/lang/deps").mkdir(parents=True)
                    journal = root / "journal"
                    journal.mkdir(mode=0o700)
                    receipt = journal / (mode + ".stderr")
                    if outcome == "occupied":
                        receipt.write_bytes(b"existing receipt")
                    go = root / "go"
                    go.write_text(f"#!{sys.executable}\n" + rf"""
import fcntl, os, signal, sys, time
from pathlib import Path

if '-o' in sys.argv:
    os.write(2, b'compiler stderr')
    Path(sys.argv[sys.argv.index('-o') + 1]).symlink_to(Path(__file__).resolve())
    sys.exit(0)
sys.stdin.buffer.read()
os.write(2, b'worker stderr: \xff\n')
if {outcome!r} in ('failure', 'signal'):
    lock = (Path(__file__).resolve().parent / 'worker.lock').open('wb')
    fcntl.flock(lock, fcntl.LOCK_EX)
    if os.fork() == 0:
        time.sleep(5)
        os._exit(0)
if {outcome!r} == 'signal':
    os.kill(os.getppid(), signal.SIGTERM)
    signal.pause()
if {outcome!r} == 'failure':
    os.write(1, b'private stdout')
    sys.exit(1)
os.write(1, b'{{"aggregate":true}}\nPASS\n')
""")
                    go.chmod(0o700)
                    config = dict(mode=mode, journal=str(journal / (mode + ".json")))
                    with patch.dict(scope, REPO=repo, progress=lambda *_: None), patch.dict(
                            os.environ, PATH=str(root)), final["termination"]() as check:
                        if outcome == "success":
                            self.assertEqual(final["measure"](config, check), {"aggregate": True})
                        else:
                            expected = {"failure": subprocess.CalledProcessError, "signal": InterruptedError,
                                        "occupied": FileExistsError}[outcome]
                            with self.assertRaises(expected):
                                final["measure"](config, check)
                    if outcome == "occupied":
                        self.assertEqual(receipt.read_bytes(), b"existing receipt")
                    else:
                        self.assertEqual(stat.S_IMODE(receipt.stat().st_mode), 0o600)
                        self.assertEqual(receipt.read_bytes(), b"worker stderr: \xff\n")
                    self.assertEqual(list(journal.iterdir()), [receipt])
                    if outcome in ("failure", "signal"):
                        with (root / "worker.lock").open("rb") as lock:
                            deadline = time.monotonic() + 2
                            while True:
                                try:
                                    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                                    break
                                except BlockingIOError:
                                    if time.monotonic() >= deadline:
                                        self.fail("subprocess group still holds the worker lock")
                                    time.sleep(0.02)

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
        before, after = {"a.py": True, "b.py": False}, {"a.py": False, "b.py": True}
        self.assertEqual(sum(before.values()), sum(after.values()))
        self.assertTrue(any(after[path] and not before[path] for path in before))
        # Identical edge evidence and counts cannot prove a per-file transition.
        with self.assertRaises((TypeError, ValueError)):
            final["assess"](correction, ({}, corrected), initial)

    def test_hash_valid_closure_must_be_numeric_and_reconcile(self):
        def closure(files):
            return dict(Selected=files, Complete=1, OnlyUnmeasured=2, OnlyMeasured=3,
                        MeasuredAndUnmeasured=files - 6, RejectedBlocked=4)
        original = {"development": {}, "holdout": {}, "holdout_rejected_inputs": {}}
        held = {}
        for dialect, (complete, files) in final["ORIGINAL_HOLDOUT"].items():
            dev_complete, dev_files = final["ORIGINAL_DEVELOPMENT"][dialect]
            original["development"][dialect] = {
                "Analysis": {"Score": dict(Complete=dev_complete, Files=dev_files, RejectedInputs=0)},
                "Closure": closure(dev_files),
            }
            original["holdout"][dialect] = dict(complete=complete, files=files)
            original["holdout_rejected_inputs"][dialect] = 0
            held[dialect] = closure(files)
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()

            def reference(name, data):
                path = root / name
                blob = final["canonical"](data)
                path.write_bytes(blob)
                return dict(path=str(path), sha256=final["sha"](blob))

            refs = {"original": reference("original", original), "closure": reference("closure", held)}
            for dialect, complete in final["CORRECTED_HOLDOUT"].items():
                refs[dialect + "_r5"] = reference(dialect, dict(
                    complete=complete, files=final["ORIGINAL_HOLDOUT"][dialect][1], b1=True, b2=True, b3=True))
            with patch.dict(scope, ORIGINAL_AGGREGATE=refs["original"]["sha256"]):
                before, _ = final["baselines"](refs)
                self.assertEqual(before["holdout"]["typescript"]["closure"], held["typescript"])
                for change in ({"diagnostic": ["synthetic.py"]}, {"Complete": "1"}, {"Complete": True},
                               {"Complete": 1.0}, {"OnlyMeasured": -1}, {"OnlyMeasured": 4},
                               {"RejectedBlocked": held["typescript"]["Selected"]}, {"Complete": 724}):
                    with self.subTest(change=change), self.assertRaises(ValueError):
                        bad = dict(held, typescript=dict(held["typescript"], **change))
                        final["baselines"](dict(refs, closure=reference("closure", bad)))
                with self.assertRaises(ValueError):
                    bad = dict(held, typescript={k: v for k, v in held["typescript"].items()
                                                 if k != "RejectedBlocked"})
                    final["baselines"](dict(refs, closure=reference("closure", bad)))
                with self.assertRaises(ValueError):
                    final["baselines"](dict(refs, closure=reference("closure", dict(held, diagnostic="synthetic"))))

    def test_failed_journal_revalidation_preserves_last_transition(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            journal = write_authorization(root, "a" * 40, "b" * 40)
            path = journal / "initial.json"
            durable = None

            def worker(config, _check):
                nonlocal durable
                final["progress"](config, "native", "development")
                durable = path.read_bytes()
                path.chmod(0o640)
                raise InterruptedError

            with patch.dict(scope, identity=lambda *_: {}, ensure_reader=lambda *_: None,
                            git=lambda *_: b"b" * 40, baselines=lambda *_: None, measure=worker):
                with self.assertRaisesRegex(ValueError, "journal storage must be private"):
                    final["run"](initial_args(root))
            self.assertIsNotNone(durable)
            self.assertEqual(path.read_bytes(), durable)
            self.assertEqual(json.loads(durable)["progress"][-1],
                             {"stage": "native", "pool": "development"})
            self.assertFalse((journal / "initial.aggregate.json").exists())

    def test_failure_receipt_cleanup_and_replay(self):
        for condition in ("failure", "term", "kill"):
            with self.subTest(condition=condition), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp).resolve()
                journal = write_authorization(root, "a" * 40, "b" * 40)
                (root / "repo/pkg/lang/deps").mkdir(parents=True)
                go = root / "go"
                go.write_text(f"#!{sys.executable}\n" + f"""
import fcntl, json, os, sys, time
from pathlib import Path

if '-o' in sys.argv:
    binary = Path(sys.argv[sys.argv.index('-o') + 1])
    binary.symlink_to(Path(__file__).resolve())
    sys.exit(0)
root = Path({str(root)!r})
config = json.load(sys.stdin)
assert Path(config['scratch']).is_dir()
assert json.loads(Path(config['journal']).read_bytes())['status'] == 'started'
import subprocess
callback = [sys.executable, '-I', '-B', '-S', {str(SCRIPT.with_name("cov_final.py"))!r}, '_progress']
for pool, stage in [('holdout', 'native'), ('private-pool-name', 'native'), ('holdout', 'private-stage-name')]:
    result = subprocess.run(callback + [pool, stage], input=json.dumps(config), text=True, capture_output=True)
    assert result.returncode == int(pool != 'holdout' or stage != 'native')
    assert 'private-' not in result.stdout + result.stderr
with (root / 'worker.lock').open('wb') as lock:
    fcntl.flock(lock, fcntl.LOCK_EX)
    child = os.fork()
    if child == 0:
        time.sleep(30)
        sys.exit(0)
    pending = root / 'worker.pending'
    pending.write_text(str(os.getpid()) + ' ' + str(child))
    pending.replace(root / 'worker.json')
    if {condition!r} == 'failure':
        print('private-stdout-snippet')
        print('private-stderr-snippet', file=sys.stderr)
        sys.exit(1)
    time.sleep(30)
""")
                go.chmod(0o700)
                cmd = [sys.executable, "-I", "-B", "-S", SCRIPT, str(root)]
                pids = []
                try:
                    with subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                          start_new_session=True) as proc:
                        ready = root / "worker.json"
                        deadline = time.monotonic() + 10
                        while not ready.exists() and proc.poll() is None and time.monotonic() < deadline:
                            time.sleep(0.02)
                        self.assertTrue(ready.exists(), "synthetic measured worker never started")
                        pids = [int(pid) for pid in ready.read_text().split()]
                        self.assertEqual(len(pids), 2)
                        if condition != "failure":
                            proc.send_signal(signal.SIGTERM if condition == "term" else signal.SIGKILL)
                        try:
                            stdout, stderr = proc.communicate(timeout=10)
                        finally:
                            if proc.poll() is None:
                                proc.kill()
                    receipt = json.loads((journal / "initial.json").read_bytes())
                    self.assertEqual(receipt["status"], "started" if condition == "kill" else "failed")
                    self.assertEqual("finished_at" in receipt, condition != "kill")
                    event = {"stage": "native", "pool": "holdout"}
                    self.assertEqual(receipt["progress"], [{"stage": stage} for stage in
                                     ("claim", "checkout", "compile", "worker")] + [event])
                    if condition != "kill":
                        kind = "CalledProcessError" if condition == "failure" else "InterruptedError"
                        self.assertEqual(receipt["failure"], dict(event, exception=kind))
                    self.assertNotIn(b"private-", (journal / "initial.json").read_bytes() + stdout + stderr)
                    self.assertFalse((journal / "initial.aggregate.json").exists())
                    self.assertFalse((journal / "initial.progress").exists())
                    if condition != "kill":
                        self.assertEqual(list((root / "repo/workspace/tmp").iterdir()), [])
                        with (root / "worker.lock").open("rb") as lock:
                            deadline = time.monotonic() + 5
                            while True:
                                try:
                                    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                                    break
                                except BlockingIOError:
                                    if time.monotonic() >= deadline:
                                        self.fail("subprocess group still holds the worker lock")
                                    time.sleep(0.02)
                        pids.clear()
                    replay = subprocess.check_output(cmd, timeout=10, text=True)
                    self.assertEqual(replay, "replay denied\n")
                    self.assertEqual(receipt, json.loads((journal / "initial.json").read_bytes()))
                finally:
                    for pid in pids:
                        try:
                            os.kill(pid, signal.SIGKILL)
                        except ProcessLookupError:
                            pass

    def test_git_checks_bind_the_original_objects_and_worktree(self):
        with patch.object(subprocess, "check_output", return_value=b"tree") as command:
            self.assertEqual(final["git"]("ls-tree", "HEAD"), b"tree")
        arguments = command.call_args.args[0]
        self.assertIn("--no-replace-objects", arguments)
        self.assertIn(f"--work-tree={scope['REPO']}", arguments)

    def test_clean_revision_drift_blocks_publication(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            repo = root / "repo"
            repo.mkdir()

            def git(*args):
                return subprocess.check_output(
                    ["git", "-C", str(repo), "-c", "core.hooksPath=/dev/null",
                     "-c", "commit.gpgsign=false", "-c", "user.name=Synthetic",
                     "-c", "user.email=synthetic@example.test", *args], stderr=subprocess.PIPE).decode().strip()

            def commit(path, contents):
                file = repo / path
                file.parent.mkdir(parents=True, exist_ok=True)
                file.write_text(contents)
                git("add", path)
                git("commit", "-qm", contents)
                return git("rev-parse", "HEAD")

            git("init", "-q")
            reader = commit("reader.txt", "approved reader")
            harness = commit(final["OWNED"][0], "reviewed harness")
            other = commit("reader.txt", "different reader")
            git("checkout", "--detach", harness)
            journal = write_authorization(root, reader, harness)
            auth = authorization(root, reader, harness)

            with patch.dict(scope, REPO=repo):
                final["ensure_reader"](auth)
                (repo / "reader.txt").write_text("unreviewed bytes")
                with self.assertRaisesRegex(ValueError, "dirty"):
                    final["ensure_reader"](auth)
                real_git = scope["git"]

                def report_clean(*args):
                    return b"" if args[0] == "status" else real_git(*args)

                with patch.dict(scope, git=report_clean):
                    with self.assertRaisesRegex(ValueError, "raw checkout bytes"):
                        final["ensure_reader"](auth)
                git("restore", "reader.txt")
                (repo / ".git/info/exclude").write_text("unreviewed.go\n")
                ignored = repo / "unreviewed.go"
                ignored.write_text("synthetic")
                with self.assertRaisesRegex(ValueError, "dirty"):
                    final["ensure_reader"](auth)
                ignored.unlink()
                git("checkout", "-b", "synthetic")
                with self.assertRaisesRegex(ValueError, "detached"):
                    final["ensure_reader"](auth)
                git("checkout", "--detach", harness)

            def drift(*_):
                git("checkout", "--detach", other)
                self.assertEqual(git("status", "--porcelain"), "")
                return {}

            with patch.dict(scope, REPO=repo, identity=lambda *_: {}, baselines=lambda *_: None,
                            measure=drift, assess=lambda *_, **__: {}):
                with self.assertRaisesRegex(ValueError, "HEAD"):
                    final["run"](initial_args(root))
            self.assertEqual(json.loads((journal / "initial.json").read_bytes())["status"], "failed")
            self.assertFalse((journal / "initial.aggregate.json").exists())


if __name__ == "__main__":
    if len(sys.argv) == 2 and Path(sys.argv[1]).is_absolute():
        synthetic_attempt(Path(sys.argv[1]))
    else:
        unittest.main()
