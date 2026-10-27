"""Small synthetic controls for the measurement driver; no C++ subjects run."""

import importlib.util
import json
import os
from pathlib import Path
import tempfile
import time
import unittest
from unittest import mock


SCRIPT = Path(__file__).with_name("cpp-audit.py")
TEST_TMP = SCRIPT.parents[3] / "workspace" / "tmp" / "cpp-audit-tests"
TEST_TMP.mkdir(parents=True, exist_ok=True)
spec = importlib.util.spec_from_file_location("cpp_audit", SCRIPT)
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)


class DriverTest(unittest.TestCase):
    def test_clean_repo_refuses_dirty_checkout(self):
        with mock.patch.object(audit, "git", side_effect=lambda root, *argv: (
                " M calibration/oracles/scip-clang/cpp-audit.py"
                if argv[0] == "status" else "a" * 40)):
            with self.assertRaisesRegex(audit.Refusal, "fathom tree changed"):
                audit.clean_repo("a" * 40)

    def test_clean_repo_refuses_clean_checkout_at_different_head(self):
        expected = "a" * 40
        with mock.patch.object(audit, "git", side_effect=lambda root, *argv: (
                "" if argv[0] == "status" else "b" * 40)):
            with self.assertRaisesRegex(audit.Refusal, "fathom revision changed"):
                audit.clean_repo(expected)

    def test_clean_repo_refuses_head_change_during_status(self):
        heads = iter(("a" * 40, "b" * 40))
        with mock.patch.object(audit, "git", side_effect=lambda root, *argv: (
                "" if argv[0] == "status" else next(heads))):
            with self.assertRaisesRegex(audit.Refusal, "fathom revision changed"):
                audit.clean_repo("a" * 40)

    def test_bounded_inputs_refuse_growth_after_initial_check(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            source = Path(tmp) / "reading.json"
            source.write_bytes(b"{}")
            original_digest = audit.digest
            def grow(path, cap=None):
                result = original_digest(path, cap)
                if path == source:
                    source.write_bytes(b" " * 11)
                return result
            with mock.patch.object(audit, "digest", side_effect=grow), \
                    self.assertRaisesRegex(audit.Refusal, "over cap"):
                audit.load_json(source, 10)
            with mock.patch.object(audit, "READING_CAP", 10), \
                    mock.patch.object(audit, "digest", side_effect=grow), \
                    self.assertRaisesRegex(audit.Refusal, "over cap"):
                audit.scip_identity(source, source.parent, "v0.4.0")

    def test_bounded_json_accepts_exact_cap_and_refuses_next_byte(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            source = Path(tmp) / "reading.json"
            source.write_bytes(b'{"a":1}')
            self.assertEqual(audit.load_json(source, 7), {"a": 1})
            with self.assertRaisesRegex(audit.Refusal, "over cap"):
                audit.load_json(source, 6)

    def test_digest_refuses_growth_after_stale_stat(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            source = Path(tmp) / "growing.bin"
            source.write_bytes(b"x" * 11)
            stat = Path.stat
            def stale_size(path, *args, **kwargs):
                result = stat(path, *args, **kwargs)
                if path == source:
                    return os.stat_result((result.st_mode, result.st_ino, result.st_dev,
                        result.st_nlink, result.st_uid, result.st_gid, 10,
                        result.st_atime, result.st_mtime, result.st_ctime))
                return result
            with mock.patch.object(Path, "stat", stale_size), \
                    self.assertRaisesRegex(audit.Refusal, "over cap"):
                audit.digest(source, 10)

    def test_scip_identity_accepts_exact_cap_and_refuses_next_byte(self):
        def varint(number):
            out = bytearray()
            while number > 127:
                out.append((number & 127) | 128)
                number >>= 7
            return bytes(out + bytes((number,)))
        def field(number, value):
            return varint(number * 8 + 2) + varint(len(value)) + value
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            root = Path(tmp)
            tool = field(1, b"scip-clang") + field(2, b"0.4.0")
            metadata = field(2, tool) + field(3, root.as_uri().encode())
            data = field(1, metadata) + field(2, field(1, b"unit.cpp"))
            source = root / "index.scip"
            source.write_bytes(data)
            with mock.patch.object(audit, "READING_CAP", len(data)):
                self.assertEqual(audit.scip_identity(source, root, "v0.4.0"), 1)
            with mock.patch.object(audit, "READING_CAP", len(data) - 1), \
                    self.assertRaisesRegex(audit.Refusal, "over cap"):
                audit.scip_identity(source, root, "v0.4.0")

    def test_each_phase_refuses_dirty_repo_before_outputs(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            root = Path(tmp)
            args = mock.Mock(index_receipt=root / "index.json", readings_dir=root / "readings",
                             work_dir=root / "work")
            with mock.patch.object(audit, "clean_repo", side_effect=audit.Refusal("fathom tree changed")):
                for phase in (audit.index_phase, audit.native, audit.grade):
                    with self.subTest(phase=phase.__name__), \
                            self.assertRaisesRegex(audit.Refusal, "fathom tree changed"):
                        phase(args, {}, "a" * 40)
            self.assertFalse(args.index_receipt.exists())
            self.assertFalse(args.readings_dir.exists())
            self.assertFalse(args.work_dir.exists())

    def test_build_fanout_refuses_at_cap_plus_one_without_draining(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            root = Path(tmp)
            subject, db = root / "subject", root / "compile_commands.json"
            subject.mkdir()
            db.write_text("[]")
            for number in range(5):
                (root / str(number)).write_text("input")
            with audit.os.scandir(root) as scan:
                entries = list(scan)
            reads = []
            class Scan:
                def __iter__(self):
                    for entry in entries:
                        reads.append(entry.name)
                        yield entry
                def __enter__(self):
                    return self
                def __exit__(self, *args):
                    pass
            with mock.patch.object(audit, "BUILD_FANOUT_CAP", 2), \
                    mock.patch.object(audit.os, "scandir", return_value=Scan()):
                with self.assertRaisesRegex(audit.Refusal, "fanout cap"):
                    audit.build_inputs(subject, db)
            self.assertEqual(len(reads), 3, "refuse before draining an unbounded directory")

    def test_native_inputs_survive_json_receipt_roundtrip(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            root = Path(tmp)
            subject, db, binary = root / "subject", root / "compile_commands.json", root / "clang"
            subject.mkdir()
            db.write_text("[]")
            binary.write_text("clang")
            args = mock.Mock(clang=binary)
            found = {"fmt": (subject, db, 1, ())}
            with mock.patch.object(audit, "tracked_manifest", return_value="a" * 64), \
                    mock.patch.object(audit, "language_inventory", return_value=(1, "b" * 64)), \
                    mock.patch.object(audit, "compiler_digests", return_value=("c" * 64,)), \
                    mock.patch.object(audit, "build_inputs", return_value={"files": 1, "sha256": "d" * 64}), \
                    mock.patch.object(audit, "index_evidence", return_value={"receipt_sha256": "e" * 64}):
                inputs = audit.native_inputs(args, found, {"preproc": binary})
            self.assertEqual(inputs, json.loads(json.dumps(inputs)))

    def test_build_manifest_refuses_cap_fanout_and_escaping_inputs(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            root = Path(tmp)
            subject, build = root / "subject", root / "build"
            subject.mkdir()
            build.mkdir()
            source = subject / "unit.cpp"
            source.write_text("int value;\n")
            db = build / "compile_commands.json"
            def command(file=source, flags=None):
                db.write_text(json.dumps([{"directory": str(build), "file": str(file),
                    "arguments": ["c++", *(flags or []), "-c", str(file)]}]))
            command()
            first = audit.build_inputs(subject, db)
            self.assertEqual(first["files"], 1)
            (build / "generated.h").write_text("#define VALUE 1\n")
            self.assertNotEqual(first["sha256"], audit.build_inputs(subject, db)["sha256"])
            with mock.patch.object(audit, "BUILD_FILE_CAP", 1), \
                    self.assertRaisesRegex(audit.Refusal, "size cap"):
                audit.build_inputs(subject, db)
            with mock.patch.object(audit, "BUILD_BYTE_CAP", audit.build_inputs(subject, db)["bytes"] - 1), \
                    self.assertRaisesRegex(audit.Refusal, "size cap"):
                audit.build_inputs(subject, db)
            with mock.patch.object(audit, "BUILD_FANOUT_CAP", 1), \
                    self.assertRaisesRegex(audit.Refusal, "fanout cap"):
                audit.build_inputs(subject, db)
            for ordinal in range(3):
                (build / f"empty-{ordinal}").mkdir()
            with mock.patch.object(audit, "BUILD_ENTRY_CAP", 4), \
                    self.assertRaisesRegex(audit.Refusal, "entry cap"):
                audit.build_inputs(subject, db)
            outside = root / "outside.h"
            outside.write_text("external")
            command(flags=["-include", str(outside)])
            with self.assertRaisesRegex(audit.Refusal, "explicit include escapes"):
                audit.build_inputs(subject, db)
            command(file=outside)
            with self.assertRaisesRegex(audit.Refusal, "compilation source escapes"):
                audit.build_inputs(subject, db)
            command()
            (build / "escape.h").symlink_to(outside)
            with self.assertRaisesRegex(audit.Refusal, "symlink escapes"):
                audit.build_inputs(subject, db)
            (build / "escape.h").unlink()
            metadata = build / ".git"
            metadata.mkdir()
            (metadata / "hidden.h").write_text("unhashed")
            (build / "hidden.h").symlink_to(metadata / "hidden.h")
            with self.assertRaisesRegex(audit.Refusal, "symlink.*metadata"):
                audit.build_inputs(subject, db)
            (build / "hidden.h").unlink()
            command(file=metadata / "hidden.h")
            with self.assertRaisesRegex(audit.Refusal, "compilation source escapes"):
                audit.build_inputs(subject, db)
            command(flags=["-I", str(metadata)])
            with self.assertRaisesRegex(audit.Refusal, "explicit include escapes"):
                audit.build_inputs(subject, db)

    def test_fetched_source_requires_pinned_clean_head(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            root = Path(tmp)
            subject, build = root / "subject", root / "build"
            subject.mkdir()
            source = subject / "unit.cpp"
            source.write_text("int value;\n")
            fetched = build / "_deps" / "fixture-src"
            fetched.mkdir(parents=True)
            (fetched / "header.h").write_text("#define VALUE 1\n")
            db = build / "compile_commands.json"
            db.write_text(json.dumps([{"directory": str(build), "file": str(source),
                "arguments": ["c++", "-I", str(fetched), "-c", str(source)]}]))
            head = "a" * 40
            def clean_git(repo, *args):
                return str(fetched.resolve()) if args[-1] == "--show-toplevel" else (
                    head if args[-1] == "HEAD" else "")
            with mock.patch.object(audit, "FETCHED_PINS", {"subject": {"fixture-src": head}}), \
                    mock.patch.object(audit, "git", side_effect=clean_git):
                identity = audit.build_inputs(subject, db)
                self.assertEqual(identity["dependencies"], [{"label": "dependency_1", "head": head}])
                self.assertNotIn("fixture-src", json.dumps(identity))
                with mock.patch.object(audit, "git", side_effect=lambda repo, *args: (
                        " M header.h" if args[0] == "status" else clean_git(repo, *args))), \
                        self.assertRaisesRegex(audit.Refusal, "cleanliness mismatch"):
                    audit.build_inputs(subject, db)
            with mock.patch.object(audit, "FETCHED_PINS", {"subject": {"fixture-src": "b" * 40}}), \
                    mock.patch.object(audit, "git", side_effect=clean_git), \
                    self.assertRaisesRegex(audit.Refusal, "revision or cleanliness mismatch"):
                audit.build_inputs(subject, db)

    def test_external_build_source_and_header_change_index_stamp(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            root = Path(tmp)
            subject, build = root / "subject", root / "build"
            subject.mkdir()
            build.mkdir()
            source = build / "external.cpp"
            header = build / "generated.h"
            source.write_text("int value = 1;\n")
            header.write_text("#define VALUE 1\n")
            compdb = build / "compile_commands.json"
            compdb.write_text(json.dumps([{"directory": str(build), "file": str(source),
                "arguments": ["c++", "-I", str(build), "-c", str(source)]}]))
            binary, license_path = root / "binary", root / "LICENSE"
            binary.write_text("binary")
            license_path.write_text("license")
            native_dir = root / "work" / "native" / "subject"
            native_dir.mkdir(parents=True)
            for filename in ("preproc.json", "calls.json", "preproc.ranges.json"):
                (native_dir / filename).write_text("{}")
            logs = root / "logs"
            logs.mkdir()
            for filename in ("subject-binary-index.log", "subject-binary-stdout.log"):
                (logs / filename).write_text("log")
            args = mock.Mock(builds_root=root, clang=binary, index_receipt=license_path,
                             indexbench=binary, ctags=binary, work_dir=root / "work",
                             index_logs_dir=logs)
            with mock.patch.object(audit, "git", return_value="a" * 40), \
                    mock.patch.object(audit, "tracked_manifest", return_value="b" * 64), \
                    mock.patch.object(audit, "compiler_digests", return_value=("c" * 64,)):
                def stamps():
                    return (audit.index_stamp(args, subject, compdb, binary, license_path),
                            audit.native_command_stamp(args, "subject", subject, compdb, binary,
                                                       "calibration/oracles/scip-clang/preproc"),
                            audit.grade_command_stamp(args, "subject", subject, compdb, binary))
                before = stamps()
                source.write_text("int value = 2;\n")
                after_source = stamps()
                self.assertTrue(all(left != right for left, right in zip(before, after_source)))
                header.write_text("#define VALUE 2\n")
                after_header = stamps()
                self.assertTrue(all(left != right for left, right in zip(after_source, after_header)))

    def test_output_confinement_refuses_subject_and_tracked_repo(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
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
                base.work_dir = workspace / "work"
                base.readings_dir = audit.REPO / "docs" / "bench" / "readings" / "lang" / "cpp"
                with self.assertRaises(ValueError):
                    audit.confined_output_paths(base, parser)
                base.work_dir = audit.REPO / "docs" / "bench" / "readings" / "lang" / "tracked"
                with self.assertRaises(ValueError):
                    audit.confined_output_paths(base, parser)
            self.assertFalse(workspace.exists())

    def test_grade_rejects_uncommitted_binary_and_ctags_oracle(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
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
        # Current harness output adds a native range family only for subjects
        # with eligible companion pairs; the range metric retains its B cap.
        for index in (0, 2):
            reading["subjects"][index]["families"]["ranges"] = "A"
        audit.validate_cpp_reading(reading, "v0.4.0")
        reading["subjects"][0]["families"]["ranges"] = "B"
        with self.assertRaises(audit.Refusal):
            audit.validate_cpp_reading(reading, "v0.4.0")
        reading["subjects"][0]["families"]["ranges"] = "A"
        reading["subjects"][1]["families"]["unexpected"] = "A"
        with self.assertRaises(audit.Refusal):
            audit.validate_cpp_reading(reading, "v0.4.0")
        reading["subjects"][1]["families"].pop("unexpected")
        reading["metric_tiers"]["range_agreement"] = "A"
        with self.assertRaises(audit.Refusal):
            audit.validate_cpp_reading(reading, "v0.4.0")
        reading["metric_tiers"]["range_agreement"] = "B"
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
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
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
                    mock.patch.object(audit, "clean_repo") as clean, \
                    mock.patch.object(audit, "SCIP_LICENSE", audit.digest(binaries / "v0.4.0" / "LICENSE")), \
                    mock.patch.object(audit, "POPULATION", {slug: (1, 1, 0) for slug in audit.PINS}), \
                    mock.patch.object(audit, "tracked_manifest", return_value="a" * 64), \
                    mock.patch.object(audit, "language_inventory", return_value=(1, "b" * 64)), \
                    mock.patch.object(audit, "compiler_digests", return_value=("c" * 64,)), \
                    mock.patch.object(audit, "build_inputs", return_value={"files": 1, "sha256": "d" * 64}), \
                    mock.patch.object(audit, "index_stamp", return_value=("frozen",)), \
                    mock.patch.object(audit, "run", side_effect=fake_run):
                audit.index_phase(args, found, "a" * 40)
                evidence = audit.index_evidence(args, found)
                self.assertGreaterEqual(clean.call_count, 10)
                self.assertTrue(all(call.args == ("a" * 40,) for call in clean.call_args_list))
            receipt = args.index_receipt.read_text()
            self.assertNotIn(tmp, receipt)
            self.assertEqual(json.loads(receipt)["status"], "complete")
            self.assertEqual(len(json.loads(receipt)["rows"]), 6)
            self.assertEqual(len(evidence["indexes"]), 3)
            legacy = json.loads(receipt)
            del legacy["rows"][0]["build_inputs"]
            args.index_receipt.write_text(json.dumps(legacy))
            with mock.patch.object(audit, "tracked_manifest", return_value="a" * 64), \
                    mock.patch.object(audit, "language_inventory", return_value=(1, "b" * 64)), \
                    mock.patch.object(audit, "compiler_digests", return_value=("c" * 64,)), \
                    mock.patch.object(audit, "build_inputs", return_value={"files": 1, "sha256": "d" * 64}), \
                    self.assertRaisesRegex(audit.Refusal, "does not attest frozen input"):
                audit.index_evidence(args, found)

    def test_runner_refuses_failed_timeout_and_cap(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
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
            for stream in ("stdout", "stderr"):
                with self.subTest(stream=stream), mock.patch.object(audit.os, "nice"), \
                        mock.patch.object(audit, "LOG_CAP", 100):
                    out, err = root / f"exact-{stream}.out", root / f"exact-{stream}.err"
                    audit.run(["python3", "-c", f"import sys; sys.{stream}.write('x'*100)"],
                              root, out, err, 5, 100, load_limit=1000)
                    self.assertEqual((out if stream == "stdout" else err).stat().st_size, 100)
            source = root / "capped.json"
            source.write_bytes(b"x" * 100)
            audit.digest(source, 100)
            source.write_bytes(b"x" * 101)
            with self.assertRaisesRegex(audit.Refusal, "input absent or over cap"):
                audit.digest(source, 100)

    def test_runner_kills_child_after_leader_exits(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            root = Path(tmp)
            code = ("import subprocess,sys; "
                    "subprocess.Popen([sys.executable,'-c','import time; time.sleep(20)'])")
            start = time.monotonic()
            with mock.patch.object(audit.os, "nice"), self.assertRaises(audit.Refusal):
                audit.run(["python3", "-c", code], root, root / "out", root / "err",
                          0.2, 100, load_limit=1000)
            self.assertLess(time.monotonic() - start, 7)

    def test_grade_scope_and_tool_flags(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
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
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            root = Path(tmp)
            for slug in audit.PINS:
                (root / slug).mkdir()
            args = mock.Mock(subjects_root=root, builds_root=root)
            with mock.patch.object(audit, "git", return_value="wrong"), self.assertRaises(audit.Refusal):
                audit.subjects(args)

    def test_failed_grade_receipt_has_null_readings_and_no_paths(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
            root = Path(tmp)
            work = root / "private-work"
            readings = root / "published"
            args = mock.Mock(readings_dir=readings, work_dir=work)
            with mock.patch.object(audit, "git", return_value="a" * 40), \
                    mock.patch.object(audit, "clean_repo"), \
                    mock.patch.object(audit, "grade_inputs", side_effect=audit.Refusal("private path")), \
                    self.assertRaises(audit.Refusal):
                audit.grade(args, {}, "a" * 40)
            receipt = (readings / "cpp-audit-receipt.json").read_text()
            self.assertNotIn(tmp, receipt)
            data = json.loads(receipt)
            self.assertEqual(data["status"], "failed")
            self.assertTrue(all(value is None for value in data["readings"].values()))

    def test_grade_merges_three_subjects_then_retimes(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
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
                    mock.patch.object(audit, "clean_repo") as clean, \
                    mock.patch.object(audit, "grade_inputs", return_value={"safe_sha256": "b" * 64}), \
                    mock.patch.object(audit, "grade_command_stamp", return_value=("frozen",)), \
                    mock.patch.object(audit, "run", side_effect=fake_run):
                audit.grade(args, found, "a" * 40)
                self.assertGreaterEqual(clean.call_count, 21)
                self.assertTrue(all(call.args == ("a" * 40,) for call in clean.call_args_list))
            self.assertEqual(len(commands), 16)
            receipt = (args.readings_dir / "cpp-audit-receipt.json").read_text()
            self.assertNotIn(tmp, receipt)
            data = json.loads(receipt)
            self.assertEqual(data["status"], "complete")
            self.assertTrue(all(value["median_run"] == 3 for value in data["readings"].values()))
            self.assertTrue(all(len(json.loads((args.readings_dir / name).read_text())["subjects"]) == 3
                                for name in data["readings"]))
            for phase, outcomes in (
                    ("reading_validation", [audit.Refusal("synthetic validation failure")]),
                    ("retime_validation", [None, audit.Refusal("synthetic validation failure")])):
                with self.subTest(phase=phase):
                    args.readings_dir = root / (phase + "-published")
                    args.work_dir = root / (phase + "-work")
                    with mock.patch.object(audit, "git", return_value="a" * 40), \
                            mock.patch.object(audit, "clean_repo"), \
                            mock.patch.object(audit, "grade_inputs", return_value={"safe_sha256": "b" * 64}), \
                            mock.patch.object(audit, "grade_command_stamp", return_value=("frozen",)), \
                            mock.patch.object(audit, "run", side_effect=fake_run), \
                            mock.patch.object(audit, "validate_cpp_reading", side_effect=outcomes), \
                            self.assertRaises(audit.Refusal):
                        audit.grade(args, found, "a" * 40)
                    failed = json.loads((args.readings_dir / "cpp-audit-receipt.json").read_text())
                    self.assertEqual(failed["reason"], phase)
                    self.assertEqual(failed["status"], "failed")
                    self.assertTrue(all(value is None for value in failed["readings"].values()))

    def test_native_errors_write_failed_receipt_with_counts(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
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
                    mock.patch.object(audit, "clean_repo"), \
                    mock.patch.object(audit, "native_inputs", return_value={"frozen": True}), \
                    mock.patch.object(audit, "native_command_stamp", return_value=("frozen",)), \
                    mock.patch.object(audit, "run", side_effect=fake_run), \
                    self.assertRaises(audit.Refusal):
                audit.native(args, found, "a" * 40)
            receipt = json.loads((root / "native" / "receipt.json").read_text())
            self.assertEqual(receipt["status"], "failed")
            self.assertIsNone(receipt["outputs"])
            self.assertEqual(receipt["counts"]["fmt"]["ranges"]["errored"], 1)

    def test_preprocessor_stopped_is_recorded_and_refused(self):
        with tempfile.TemporaryDirectory(dir=TEST_TMP) as tmp:
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
                    mock.patch.object(audit, "clean_repo"), \
                    mock.patch.object(audit, "native_inputs", return_value={"frozen": True}), \
                    mock.patch.object(audit, "native_command_stamp", return_value=("frozen",)), \
                    mock.patch.object(audit, "run", side_effect=fake_run), \
                    self.assertRaises(audit.Refusal):
                audit.native(args, found, "a" * 40)
            receipt = json.loads((root / "native" / "receipt.json").read_text())
            self.assertEqual(receipt["status"], "failed")
            self.assertEqual(receipt["counts"]["fmt"]["preproc"], {"units": 1, "stopped": 1})


if __name__ == "__main__":
    unittest.main()
