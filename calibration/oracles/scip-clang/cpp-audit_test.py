"""Small synthetic controls for the measurement driver; no C++ subjects run."""

import importlib.util
import json
from pathlib import Path
import tempfile
import time
import unittest
from unittest import mock


SCRIPT = Path(__file__).with_name("cpp-audit.py")
spec = importlib.util.spec_from_file_location("cpp_audit", SCRIPT)
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)


class DriverTest(unittest.TestCase):
    def test_output_confinement_refuses_subject_and_tracked_repo(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            workspace = root / "workspace"
            subject = workspace / "subjects" / "fmt"
            parser = mock.Mock()
            parser.error.side_effect = ValueError("unsafe output")
            base = mock.Mock(subjects_root=subject.parent, builds_root=workspace / "builds",
                             indexes_dir=workspace / "indexes", lock_dir=workspace / "lock",
                             readings_dir=workspace / "readings", indexbench=None, ctags=None,
                             clang=None, scip_binaries_root=None)
            with mock.patch.object(audit, "workspace_roots", return_value=(workspace,)):
                base.work_dir = subject
                with self.assertRaises(ValueError):
                    audit.confined_output_paths(base, parser)
                base.work_dir = audit.REPO / "docs" / "bench" / "readings" / "lang" / "tracked"
                with self.assertRaises(ValueError):
                    audit.confined_output_paths(base, parser)
            self.assertFalse(workspace.exists())

    def test_grade_rejects_uncommitted_binary_and_ctags_oracle(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            indexbench = root / "indexbench"
            ctags = root / "ctags"
            indexbench.write_text("go")
            ctags.write_text("ctags")
            args = mock.Mock(indexbench=indexbench, ctags=ctags)
            head = "a" * 40
            build = (f"path\tgithub.com/geoffmilleraz/fathom/cmd/indexbench\n"
                     f"build\tvcs.revision={head}\nbuild\tvcs.modified=true\n").encode()
            result = mock.Mock(returncode=0, stdout=build)
            with mock.patch.object(audit, "git", return_value=head), \
                    mock.patch.object(audit.subprocess, "run", return_value=result), \
                    self.assertRaises(audit.Refusal):
                audit.grade_binary_identity(args)
            clean_build = result.stdout.replace(b"vcs.modified=true", b"vcs.modified=false")
            with mock.patch.object(audit, "git", return_value=head), \
                    mock.patch.object(audit.subprocess, "run", side_effect=[
                        mock.Mock(returncode=0, stdout=clean_build),
                        mock.Mock(returncode=0, stdout=b"Exuberant Ctags 5.8\n")]), \
                    self.assertRaises(audit.Refusal):
                audit.grade_binary_identity(args)
            reading = {"dialect": "cpp", "oracles": [
                {"tool": "universal-ctags", "tier": "B", "version": "0.0.0"},
                {"tool": "scip-clang", "tier": "A", "version": "0.4.0"},
                {"tool": "clang-ast", "tier": "A", "version": "Apple clang 21"},
                {"tool": "clang-preprocessor", "tier": "A", "version": "Apple clang 21"}],
                "metrics": {name: 0.5 for name in (
                    "def_mapping", "typed_recall", "file_edge_precision", "file_edge_recall",
                    "call_precision", "call_recall", "sym_precision", "sym_recall",
                    "range_agreement")}}
            with self.assertRaises(audit.Refusal):
                audit.validate_cpp_reading(reading, "v0.4.0")

    def test_cpp_reading_requires_subject_families_and_aggregate_import(self):
        families = {"l1": "B", "calls": "A", "defmap": "A", "file_edges": "A",
                    "imports": "A", "typed": "A"}
        tier_a = ("def_mapping", "typed_recall", "file_edge_precision", "file_edge_recall",
                  "call_precision", "call_recall", "range_agreement", "import_resolution")
        reading = {"dialect": "cpp", "subjects": [
            {"name": pin[1], "sha": pin[0], "files": 1, "bytes": 10,
             "families": families.copy()} for pin in audit.PINS.values()],
            "oracles": [{"tool": tool, "tier": tier, "version": release}
                        for tool, tier, release in (
                            ("universal-ctags", "B", "6.2.1"),
                            ("scip-clang", "A", "0.4.0"),
                            ("clang-ast", "A", "Apple clang 21"),
                            ("clang-preprocessor", "A", "Apple clang 21"))],
            "metrics": {name: 0.5 for name in tier_a + ("sym_precision", "sym_recall")},
            "metric_tiers": {**{name: "A" for name in tier_a},
                             "sym_precision": "B", "sym_recall": "B", "range_agreement": "B"}}
        audit.validate_cpp_reading(reading, "v0.4.0")
        reading["subjects"][0]["families"].pop("imports")
        with self.assertRaises(audit.Refusal):
            audit.validate_cpp_reading(reading, "v0.4.0")
        reading["subjects"][0]["families"]["imports"] = "A"
        reading["metrics"].pop("import_resolution")
        with self.assertRaises(audit.Refusal):
            audit.validate_cpp_reading(reading, "v0.4.0")
        reading["metrics"]["import_resolution"] = 0.5
        reading["metric_tiers"].pop("import_resolution")
        with self.assertRaises(audit.Refusal):
            audit.validate_cpp_reading(reading, "v0.4.0")

    def test_index_phase_receipt_attests_six_synthetic_indexes(self):
        def varint(number):
            out = bytearray()
            while number > 127:
                out.append((number & 127) | 128)
                number >>= 7
            out.append(number)
            return bytes(out)
        def field(number, data):
            return varint(number * 8 + 2) + varint(len(data)) + data
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            binaries = root / "tools"
            versions = {}
            for version in ("v0.4.0", "v0.4.1"):
                directory = binaries / version
                directory.mkdir(parents=True)
                binary = directory / "scip-clang"
                binary.write_text(version)
                (directory / "LICENSE").write_text("Apache")
                versions[version] = audit.digest(binary)
            found = {}
            for slug in audit.PINS:
                subject = root / slug
                subject.mkdir()
                db = root / (slug + ".db")
                db.write_text("[]")
                found[slug] = (subject, db, 1, ())
            args = mock.Mock(scip_binaries_root=binaries, indexes_dir=root / "indexes",
                             index_logs_dir=root / "indexes" / "logs",
                             index_receipt=root / "indexes" / "candidate-indexes.json",
                             work_dir=root / "work", timeout=5, load_limit=9)
            def fake_run(argv, cwd, stdout, stderr, *unused, **kwargs):
                stdout.parent.mkdir(parents=True, exist_ok=True)
                stderr.parent.mkdir(parents=True, exist_ok=True)
                if "--version" in argv:
                    stdout.write_text("scip-clang " + Path(argv[0]).parent.name[1:] + "\n")
                    stderr.write_text("")
                    return
                version = Path(argv[0]).parent.name
                tool = field(1, b"scip-clang") + field(2, version[1:].encode())
                meta = field(2, tool) + field(3, Path(cwd).as_uri().encode())
                document = field(1, b"src/a.cpp")
                Path(argv[argv.index("--index-output-path") + 1]).write_bytes(
                    field(1, meta) + field(2, document))
                stdout.write_text("")
                stderr.write_text("Finished indexing 1 translation units in 0.1s "
                                  "(indexing: 0.1s, merging: 0.0s, num errored TUs: 0).\n")
            with mock.patch.object(audit, "INDEX_BINARY", versions), \
                    mock.patch.object(audit, "SCIP_LICENSE", audit.digest(binaries / "v0.4.0" / "LICENSE")), \
                    mock.patch.object(audit, "POPULATION", {slug: (1, 1, 0) for slug in audit.PINS}), \
                    mock.patch.object(audit, "tracked_manifest", return_value="a" * 64), \
                    mock.patch.object(audit, "language_inventory", return_value=(1, "b" * 64)), \
                    mock.patch.object(audit, "compiler_digests", return_value=("c" * 64,)), \
                    mock.patch.object(audit, "index_stamp", return_value=("frozen",)), \
                    mock.patch.object(audit, "run", side_effect=fake_run):
                audit.index_phase(args, found)
                evidence = audit.index_evidence(args, found)
            receipt = args.index_receipt.read_text()
            self.assertNotIn(tmp, receipt)
            self.assertEqual(json.loads(receipt)["status"], "complete")
            self.assertEqual(len(json.loads(receipt)["rows"]), 6)
            self.assertEqual(len(evidence["indexes"]), 3)

    def test_runner_refuses_failed_timeout_and_cap(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            cases = (("import sys; sys.exit(7)", 5, "subprocess failed"),
                     ("import time; time.sleep(5)", 0.1, "subprocess timeout"),
                     ("import sys; sys.stdout.write('x'*101)", 5, "output cap exceeded"),
                     ("import sys; sys.stderr.write('x'*101)", 5, "output cap exceeded"))
            for index, (code, timeout, reason) in enumerate(cases):
                with self.subTest(code=code), mock.patch.object(audit.os, "nice"), \
                        mock.patch.object(audit, "LOG_CAP", 100), \
                        self.assertRaisesRegex(audit.Refusal, reason):
                    audit.run(["python3", "-c", code], root,
                              root / f"out-{index}", root / f"err-{index}",
                              timeout, 100, load_limit=1000)
            source = root / "capped.json"
            source.write_bytes(b"x" * 100)
            audit.digest(source, 100)
            source.write_bytes(b"x" * 101)
            with self.assertRaisesRegex(audit.Refusal, "input absent or over cap"):
                audit.digest(source, 100)

    def test_runner_kills_child_after_leader_exits(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            code = ("import subprocess,sys; "
                    "subprocess.Popen([sys.executable,'-c','import time; time.sleep(20)'])")
            start = time.monotonic()
            with mock.patch.object(audit.os, "nice"), self.assertRaises(audit.Refusal):
                audit.run(["python3", "-c", code], root, root / "out", root / "err",
                          0.2, 100, load_limit=1000)
            self.assertLess(time.monotonic() - start, 7)

    def test_grade_scope_and_tool_flags(self):
        with tempfile.TemporaryDirectory() as tmp:
            args = mock.Mock(indexbench=Path(tmp) / "indexbench", ctags=Path(tmp) / "ctags",
                             work_dir=Path(tmp))
            for slug in audit.PINS:
                legacy = audit.grade_command(args, slug, Path(tmp) / slug,
                                             Path(tmp) / "index.scip", Path(tmp) / "reading.json",
                                             Path(tmp) / "unmatched.json", "legacy", "a" * 40)
                full = audit.grade_command(args, slug, Path(tmp) / slug,
                                           Path(tmp) / "index.scip", Path(tmp) / "reading.json",
                                           Path(tmp) / "unmatched.json", "full", "a" * 40)
                self.assertEqual(legacy[legacy.index("-only") + 1], audit.PINS[slug][2])
                self.assertNotIn("-only", full)
                self.assertNotIn("-scip-defs", legacy)
                self.assertEqual(legacy.count("-tool-oracle"), 2)
                self.assertEqual(legacy[legacy.index("-calls-from") + 1], "scip")

    def test_subject_refuses_pin_mismatch(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for slug in audit.PINS:
                (root / slug).mkdir()
            args = mock.Mock(subjects_root=root, builds_root=root)
            with mock.patch.object(audit, "git", return_value="wrong"), self.assertRaises(audit.Refusal):
                audit.subjects(args)

    def test_failed_grade_receipt_has_null_readings_and_no_paths(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            work = root / "private-work"
            readings = root / "published"
            args = mock.Mock(readings_dir=readings, work_dir=work)
            with mock.patch.object(audit, "git", return_value="a" * 40), \
                    mock.patch.object(audit, "grade_inputs", side_effect=audit.Refusal("private path")), \
                    self.assertRaises(audit.Refusal):
                audit.grade(args, {})
            receipt = (readings / "cpp-audit-receipt.json").read_text()
            self.assertNotIn(tmp, receipt)
            data = json.loads(receipt)
            self.assertEqual(data["status"], "failed")
            self.assertTrue(all(value is None for value in data["readings"].values()))

    def test_grade_merges_three_subjects_then_retimes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            args = mock.Mock(readings_dir=root / "published", work_dir=root / "work",
                             indexes_dir=root / "indexes", indexbench=root / "indexbench",
                             ctags=root / "ctags", timeout=10, load_limit=1000)
            found = {slug: (root / slug, root / (slug + ".db"), 1) for slug in audit.PINS}
            commands = []
            def fake_run(argv, cwd, stdout, stderr, *unused, **kwargs):
                commands.append(argv)
                if argv[1] == "lang-retime":
                    stdout.parent.mkdir(parents=True, exist_ok=True)
                    stdout.write_text("run 1: 10.00 MB/CPU-s\nrun 2: 30.00 MB/CPU-s\n"
                                      "run 3: 20.00 MB/CPU-s\nmedian run 3: 20.00 MB/CPU-s\n")
                    stderr.write_text("")
                    return
                reading = Path(argv[argv.index("-out") + 1])
                reading.parent.mkdir(parents=True, exist_ok=True)
                version = reading.parent.name.split("-")[-1]
                previous = json.loads(reading.read_text()) if reading.exists() else {
                    "dialect": "cpp", "subjects": [],
                    "oracles": [{"tool": tool, "tier": tier, "version": release} for tool, tier, release in (
                        ("universal-ctags", "B", "6.2.1"), ("scip-clang", "A", version[1:]),
                        ("clang-ast", "A", "Apple clang 21"),
                        ("clang-preprocessor", "A", "Apple clang 21"))],
                    "metrics": {name: 0.5 for name in (
                        "def_mapping", "typed_recall", "file_edge_precision", "file_edge_recall",
                        "call_precision", "call_recall", "sym_precision", "sym_recall",
                        "range_agreement", "import_resolution")},
                    "metric_tiers": {**{name: "A" for name in (
                        "def_mapping", "typed_recall", "file_edge_precision", "file_edge_recall",
                        "call_precision", "call_recall", "range_agreement", "import_resolution")},
                        "sym_precision": "B", "sym_recall": "B", "range_agreement": "B"}}
                name = argv[argv.index("-subject-name") + 1]
                slug = next(key for key, pin in audit.PINS.items() if pin[1] == name)
                previous["subjects"].append({
                    "name": name, "sha": audit.PINS[slug][0], "files": 1, "bytes": 10,
                    "families": {"l1": "B", "calls": "A", "defmap": "A",
                                 "file_edges": "A", "imports": "A", "typed": "A"}})
                reading.write_text(json.dumps(previous))
            with mock.patch.object(audit, "git", return_value="a" * 40), \
                    mock.patch.object(audit, "grade_inputs", return_value={"safe_sha256": "b" * 64}), \
                    mock.patch.object(audit, "grade_command_stamp", return_value=("frozen",)), \
                    mock.patch.object(audit, "run", side_effect=fake_run):
                audit.grade(args, found)
            self.assertEqual(len(commands), 16)
            receipt = (args.readings_dir / "cpp-audit-receipt.json").read_text()
            self.assertNotIn(tmp, receipt)
            data = json.loads(receipt)
            self.assertEqual(data["status"], "complete")
            self.assertTrue(all(value["median_run"] == 3 for value in data["readings"].values()))
            self.assertTrue(all(len(json.loads((args.readings_dir / name).read_text())["subjects"]) == 3
                                for name in data["readings"]))

    def test_native_errors_write_failed_receipt_with_counts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            args = mock.Mock(work_dir=root, clang=root / "clang", timeout=2, load_limit=1000)
            args.clang.write_text("clang")
            found = {"fmt": (root, root / "db", 1, (audit.digest(args.clang),))}
            (root / "db").write_text("[]")
            def fake_run(argv, cwd, stdout, stderr, *unused, **kwargs):
                if stdout.name.endswith("partial"):
                    name = "preproc" if stdout.name == "preproc.json.partial" else (
                        "ranges" if stdout.name == "preproc.ranges.json.partial" else "calls")
                    data = {"tool": name, "tier": "A", "units": 1, "errored": 1}
                    stdout.write_text(json.dumps(data))
                    suffix = "files 0 includes_in_subject 0" if name == "preproc" else "sites 0"
                    stderr.write_text(f"translation_units 1 stopped 0 {suffix}\n")
            with mock.patch.object(audit, "build_oracles", return_value={
                    name: args.clang for name, _, _ in audit.ORACLES}), \
                    mock.patch.object(audit, "native_inputs", return_value={"frozen": True}), \
                    mock.patch.object(audit, "native_command_stamp", return_value=("frozen",)), \
                    mock.patch.object(audit, "run", side_effect=fake_run), \
                    self.assertRaises(audit.Refusal):
                audit.native(args, found)
            receipt = json.loads((root / "native" / "receipt.json").read_text())
            self.assertEqual(receipt["status"], "failed")
            self.assertIsNone(receipt["outputs"])
            self.assertEqual(receipt["counts"]["fmt"]["ranges"]["errored"], 1)

    def test_preprocessor_stopped_is_recorded_and_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            clang = root / "clang"
            clang.write_text("clang")
            db = root / "db"
            db.write_text("[]")
            args = mock.Mock(work_dir=root, clang=clang, timeout=2, load_limit=9)
            found = {"fmt": (root, db, 1, (audit.digest(clang),))}
            def fake_run(argv, cwd, stdout, stderr, *unused, **kwargs):
                stdout.parent.mkdir(parents=True, exist_ok=True)
                stdout.write_text('{"tool":"clang-preprocessor","tier":"A"}')
                stderr.write_text("translation_units 1 stopped 1 files 0 includes_in_subject 0\n")
            with mock.patch.object(audit, "build_oracles", return_value={
                    name: clang for name, _, _ in audit.ORACLES}), \
                    mock.patch.object(audit, "native_inputs", return_value={"frozen": True}), \
                    mock.patch.object(audit, "native_command_stamp", return_value=("frozen",)), \
                    mock.patch.object(audit, "run", side_effect=fake_run), \
                    self.assertRaises(audit.Refusal):
                audit.native(args, found)
            receipt = json.loads((root / "native" / "receipt.json").read_text())
            self.assertEqual(receipt["status"], "failed")
            self.assertEqual(receipt["counts"]["fmt"]["preproc"], {"units": 1, "stopped": 1})


if __name__ == "__main__":
    unittest.main()
