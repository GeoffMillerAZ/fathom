import hashlib
import copy
import importlib.util
import json
import math
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
AUDIT_PATH = HERE / "cpp-audit.py"
HISTORICAL_DRIVER_SHA256 = "baf94aaa397277fff0bd78cfefa7a384142aefba0665c08ea187ebafda348461"
TMP = HERE.parents[2] / "workspace" / "tmp"


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


report = load("cpp_report_staged", HERE / "cpp-report.py")
audit = load("cpp_audit_staged", AUDIT_PATH)


def sha(b):
    return hashlib.sha256(b).hexdigest()


def put(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(json.dumps(value, sort_keys=True).encode())


def field(number, value):
    assert len(value) < 128
    return bytes([number * 8 + 2, len(value)]) + value


def index(paths, version):
    tool = field(1, b"scip-clang") + field(2, version[1:].encode())
    meta = field(2, tool) + field(3, b"file:///private/subject")
    return field(1, meta) + b"".join(field(2, field(1, p.encode())) for p in paths)


def subject(slug, *, files=1, bytes_=7):
    counts = {
        "matched": 8, "oracle_defs": 10, "fathom_defs": 10, "adjudicated": 1,
        "kind_equal": 7, "range_ok": 6, "range_eligible": 8,
        "callables_signed": 4, "callables": 5,
        "call_truth_found": 3, "call_truth": 5, "call_fathom_in_truth": 3,
        "call_fathom": 10, "calls_oracle_blind": 2,
        "imports_resolved_agree": 3, "imports_in_repo": 4,
        "file_edges_agree": 6, "file_edges_fathom": 10,
        "file_edges_oracle_blind": 2, "file_edges_truth": 8,
        "scip_defs_mapped": 7, "scip_defs": 10,
        "scip_edges_typed": 9, "scip_edges": 12,
        "scip_file_edges_every_definer": 1, "scip_edges_macro_generated": 2,
    }
    return {"name": audit.PINS[slug][1], "sha": audit.PINS[slug][0],
            "files": files, "bytes": bytes_, "counts": counts,
            "families": {"l1": "B", "calls": "A", "defmap": "A",
                         "file_edges": "A", "imports": "A", "typed": "A"},
            "tenancy": {"faults": 0, "faulting_files": 0,
                        "files_with_error": 1, "files_with_error_or_missing": 1}}


def reading(version):
    subjects = [subject(slug) for slug in audit.PINS]
    metric_tiers = {"sym_recall": "B", "sym_precision": "B",
                    "kind_agreement": "B", "range_agreement": "B",
                    "signature_coverage": "B", "call_recall": "A",
                    "call_precision": "A", "import_resolution": "A",
                    "file_edge_precision": "A", "file_edge_recall": "A",
                    "def_mapping": "A", "typed_recall": "A"}
    oracles = [{"tool": name, "tier": tier, "version": release}
               for name, tier, release in (("universal-ctags", "B", "6.2.1"),
                   ("scip-clang", "A", version[1:]), ("clang-ast", "A", "synthetic"),
                   ("clang-preprocessor", "A", "synthetic"))]
    metrics = {}
    for name in metric_tiers:
        rows = [report.metric_rows(s)[name] for s in subjects]
        n = sum(row["numerator"] for row in rows)
        d = sum(row["denominator"] for row in rows)
        metrics[name] = math.floor(n / d * 10000 + 0.5) / 10000 if d else None
    pooled_counts = {key: sum(s["counts"].get(key, 0) for s in subjects)
                     for key in subjects[0]["counts"]}
    return {"dialect": "cpp", "fathom_commit": report.BASELINE,
            "subjects": subjects, "oracles": oracles,
            "counts": pooled_counts,
            "metrics": metrics,
            "metric_tiers": metric_tiers,
            "notes": ["PRIVATE PERSON /private/subject/file.cpp"]}


def audit_fixture(test):
    TMP.mkdir(parents=True, exist_ok=True)
    base = Path(tempfile.mkdtemp(dir=TMP))
    test.addCleanup(lambda: __import__("shutil").rmtree(base))
    clean = mock.patch.object(report, "_require_clean_repo")
    clean.start()
    test.addCleanup(clean.stop)
    binary = base / "cpp-screen-bin"
    binary.write_bytes(b"synthetic screen binary")
    binary_pin = mock.patch.object(report, "SCREEN_BINARY_SHA256", sha(binary.read_bytes()))
    binary_pin.start()
    test.addCleanup(binary_pin.stop)
    root = base / "audit-r3"
    (root / "readings").mkdir(parents=True)
    (root / "indexes").mkdir()
    (root / "work" / "native").mkdir(parents=True)
    (root / "work" / "grade").mkdir()
    index_rows = []
    index_evidence = {}
    for slug in audit.PINS:
        index_evidence[slug] = {}
        for version in audit.INDEX_BINARY:
            raw = index(["a.cpp", "a.cpp", "outside.cpp"], version)
            idx = root / "indexes" / f"{slug}-{version}.scip"
            idx.write_bytes(raw)
            index_rows.append({"subject": audit.PINS[slug][1], "subject_sha": audit.PINS[slug][0],
                               "version": version, "scip_sha256": sha(raw), "documents": 3,
                               "binary_sha256": audit.INDEX_BINARY[version]})
            index_evidence[slug][version] = {"scip_sha256": sha(raw), "documents": 3}
    index_receipt = {"status": "complete", "producer": "calibration/oracles/scip-clang/cpp-audit.py",
                     "driver_sha256": HISTORICAL_DRIVER_SHA256, "rows": index_rows}
    put(root / "indexes" / "candidate-indexes.json", index_receipt)
    native_outputs = {}
    for slug in audit.PINS:
        native_outputs[slug] = {}
        for name in report.NATIVE_FILES:
            path = root / "work" / "native" / slug / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b"synthetic native")
            native_outputs[slug][name] = sha(path.read_bytes())
    native = {"status": "complete", "inputs": {"synthetic": 1,
              "clang_sha256": "a" * 64, "driver_sha256": HISTORICAL_DRIVER_SHA256},
              "outputs": native_outputs}
    put(root / "work" / "native" / "receipt.json", native)
    outputs = {}
    for scope in ("legacy", "full"):
        for version in audit.INDEX_BINARY:
            stem = f"cpp-{scope}-{version}"
            raw = reading(version)
            p = root / "readings" / f"{stem}.json"
            put(p, raw)
            outputs[p.name] = {"sha256": sha(p.read_bytes()), "subjects": 3,
                               "median_run": 2, "throughput_mb_cpu_s": 1.0,
                               "run_rates_mb_cpu_s": [1.0, 1.0, 1.0]}
            for slug in audit.PINS:
                put(root / "work" / "grade" / stem / slug / "unmatched.json",
                    {"tenancy_files": ["a.cpp"], "dialect_only": ["PRIVATE"]})
    grade = {"status": "complete", "commit": report.BASELINE,
             "inputs": {"driver_sha256": HISTORICAL_DRIVER_SHA256,
                        "indexbench_sha256": "b" * 64, "ctags_sha256": "c" * 64,
                        "tool_identity": {"indexbench": {"source": "cmd/indexbench",
                          "revision": report.BASELINE, "sha256": "b" * 64, "modified": False},
                          "ctags": {"version": "6.2.1", "sha256": "c" * 64}},
                        "native_inputs": native["inputs"], "native_outputs": native["outputs"],
                        "index_evidence": {"receipt_sha256": sha((root / "indexes" / "candidate-indexes.json").read_bytes()),
                                           "indexes": index_evidence}},
             "readings": outputs}
    put(root / "readings" / "cpp-audit-receipt.json", grade)
    (root / "screens").mkdir()
    counts = {key: 0 for key in report.SYNTAX_FIELDS}
    for scope in ("legacy", "full"):
        for slug in audit.PINS:
            unmatched = root / "work" / "grade" / f"cpp-{scope}-v0.4.1" / slug / "unmatched.json"
            screen = {"status": "complete", "counts_complete": True, "files": 1,
                      "written": counts, "selected": counts,
                      "reading_sha256": outputs[f"cpp-{scope}-v0.4.1.json"]["sha256"],
                      "unmatched_sha256": sha(unmatched.read_bytes()),
                      "population_sha256": sha(b"a.cpp\n"),
                      "product_revision": report.BASELINE,
                      "grammar_blob_sha256": report.GRAMMAR,
                      "grammar_commit": report.GRAMMAR_COMMIT,
                      "grammar_entry_point": report.GRAMMAR_ENTRY,
                      "product_binary_sha256": sha(binary.read_bytes())}
            if (scope, slug) == ("full", "json"):
                screen.update(status="failed", failure_code="parse_fault",
                              counts_complete=False, files=0)
            put(root / "screens" / f"{scope}-{slug}.json", screen)
    return root, base / "summary.json", base


def timing_fixture(root, base):
    timing = base / "timing"
    timing.mkdir()
    for scope in report.SCOPES:
        for version in audit.INDEX_BINARY:
            name = f"cpp-{scope}-{version}.json"
            original = root / "readings" / name
            fresh = copy.deepcopy(json.loads(original.read_text()))
            fresh["tenancy"] = {"throughput_mb_s": 1.2, "max_file_ms": 8}
            for sub in fresh["subjects"]:
                sub["tenancy"].update(extract_ms=8, extract_cpu_ms=7, max_file_ms=8)
            fresh["notes"].append("throughput (design 1.9 T1, T2): retimed")
            put(timing / name, fresh)
            log = {"schema": "fathom-lang-retime-load-v1",
                   "reading_before_sha256": sha(original.read_bytes()),
                   "reading_after_sha256": sha((timing / name).read_bytes()),
                   "os": "darwin", "arch": "arm64", "product_revision": "d" * 40,
                   "product_binary_sha256": "e" * 64, "product_modified": False,
                   "runs": [{"number": n, "rate_mb_cpu_s": rate,
                             "load_before": 2.0 + n, "load_after": 2.5 + n}
                            for n, rate in enumerate((1.1, 1.2, 1.3), 1)]}
            put(timing / (name + ".loads.json"), log)
    return timing


class ReportTests(unittest.TestCase):
    def test_aggregate_counts_match_subjects_before_ratio_rounding(self):
        value = reading("v0.4.0")
        report._validate_pooled(value)
        value["counts"]["matched"] += 1
        with self.assertRaisesRegex(report.Refusal, "aggregate count"):
            report._validate_pooled(value)

    def test_fresh_timing_positive_and_identity_refusals(self):
        root, out, base = audit_fixture(self)
        timing = timing_fixture(root, base)
        kwargs = {"workspace_roots": [base], "audit_module": audit,
                  "screen_binary": base / "cpp-screen-bin", "timing_dir": timing}
        with mock.patch.object(report, "_current_revision", return_value="d" * 40):
            result = report.produce(root, out, **kwargs)
        row = result["readings"]["cpp-full-v0.4.0.json"]
        self.assertEqual(row["throughput_mb_cpu_s"], 1.2)
        self.assertEqual(row["run_rates_mb_cpu_s"], [1.1, 1.2, 1.3])
        self.assertEqual(row["run_loads_1m"], [{"before": 3.0, "after": 3.5},
                                                 {"before": 4.0, "after": 4.5},
                                                 {"before": 5.0, "after": 5.5}])
        self.assertEqual(row["historical_timing"]["load_condition"], "unrecorded")
        self.assertEqual(result["timing_product_revision"], "d" * 40)
        self.assertEqual(result["timing_product_binary_sha256"], "e" * 64)
        self.assertEqual(len(result["audit_inputs_sha256"]), report.MAX_INPUTS)
        self.assertEqual(len(result["timing_inputs"]), 4)
        out.unlink()
        for fault in ("counts", "before_hash", "after_hash", "binary", "revision", "loads", "rate"):
            with self.subTest(fault=fault):
                path = timing / "cpp-full-v0.4.0.json"
                log_path = timing / (path.name + ".loads.json")
                original_reading, original_log = path.read_bytes(), log_path.read_bytes()
                reading, log = json.loads(original_reading), json.loads(original_log)
                if fault == "counts":
                    reading["subjects"][0]["counts"]["matched"] += 1
                    put(path, reading)
                    log["reading_after_sha256"] = sha(path.read_bytes())
                elif fault == "before_hash":
                    log["reading_before_sha256"] = "0" * 64
                elif fault == "after_hash":
                    log["reading_after_sha256"] = "0" * 64
                elif fault == "binary":
                    log["product_binary_sha256"] = "0" * 64
                elif fault == "revision":
                    log["product_revision"] = "0" * 40
                elif fault == "loads":
                    log["runs"][0]["load_before"] = -1
                else:
                    log["runs"][1]["rate_mb_cpu_s"] = 9.0
                put(log_path, log)
                with mock.patch.object(report, "_current_revision", return_value="d" * 40):
                    with self.assertRaises(report.Refusal):
                        report.produce(root, out, **kwargs)
                self.assertFalse(out.exists())
                path.write_bytes(original_reading)
                log_path.write_bytes(original_log)

    def test_timing_shared_budget_and_growing_file(self):
        root, out, base = audit_fixture(self)
        timing = timing_fixture(root, base)
        kwargs = {"workspace_roots": [base], "audit_module": audit,
                  "screen_binary": base / "cpp-screen-bin", "timing_dir": timing}
        total = sum(path.stat().st_size for path in root.rglob("*.json")) + \
                sum(path.stat().st_size for path in timing.glob("*.json"))
        with mock.patch.object(report, "_current_revision", return_value="d" * 40):
            report.produce(root, out, json_cap=total, **kwargs)
            out.unlink()
            with self.assertRaises(report.Refusal):
                report.produce(root, out, json_cap=total - 1, **kwargs)
        self.assertFalse(out.exists())
        path = timing / "cpp-full-v0.4.0.json"
        size = path.stat().st_size
        original_open = Path.open
        changed = [False]
        def growing_open(item, *args, **kwargs):
            if item == path and not changed[0]:
                changed[0] = True
                path.write_bytes(path.read_bytes() + b"x")
            return original_open(item, *args, **kwargs)
        with mock.patch.object(Path, "open", growing_open):
            with self.assertRaises(report.Refusal):
                report._safe_file(path, timing, size, [size], {})
        self.assertTrue(changed[0])

    def test_clean_repo_status_gate(self):
        with mock.patch.object(report, "_git", return_value=b"") as git:
            report._require_clean_repo()
            self.assertEqual(git.call_args.args[1:], ("status", "--porcelain", "--untracked-files=all"))
        with mock.patch.object(report, "_git", return_value=b" M tracked.py\n"):
            with self.assertRaisesRegex(report.Refusal, "nonignored fathom tree changed"):
                report._require_clean_repo()

    def test_metric_numerators_denominators_and_zero(self):
        row = report.metric_rows(subject("fmt"))
        self.assertEqual((row["sym_precision"]["numerator"], row["sym_precision"]["denominator"]), (8, 9))
        self.assertEqual((row["call_precision"]["numerator"], row["call_precision"]["denominator"]), (3, 12))
        self.assertEqual((row["file_edge_precision"]["numerator"], row["file_edge_precision"]["denominator"]), (6, 12))
        self.assertEqual((row["def_mapping"]["numerator"], row["def_mapping"]["denominator"]), (7, 10))
        self.assertEqual(len(row), 12)
        empty = subject("fmt"); empty["counts"]["call_truth"] = 0
        self.assertIsNone(report.metric_rows(empty)["call_recall"]["ratio"])

    def test_oracle_blind_cap_is_per_subject(self):
        row = subject("fmt")
        counts = row["counts"]
        counts["calls_oracle_blind"] = 1
        counts["call_fathom"] = 9
        self.assertEqual(report.metric_rows(row)["call_precision"]["denominator"], 9)
        counts["calls_oracle_blind"] = 2
        self.assertEqual(report.metric_rows(row)["call_precision"]["denominator"], 11)
        counts["file_edges_oracle_blind"] = 1
        counts["file_edges_fathom"] = 9
        self.assertEqual(report.metric_rows(row)["file_edge_precision"]["denominator"], 9)
        counts["file_edges_oracle_blind"] = 2
        self.assertEqual(report.metric_rows(row)["file_edge_precision"]["denominator"], 11)

    def test_diagnostic_drift_allowed_graded_drift_refused(self):
        first, second = reading("v0.4.0"), reading("v0.4.1")
        second["subjects"][0]["counts"]["scip_edges_macro_generated"] += 1
        report._same_grading(first, second)
        second["subjects"][0]["counts"]["scip_edges_typed"] += 1
        with self.assertRaises(report.Refusal):
            report._same_grading(first, second)

    def test_scip_records_vs_unique_paths_and_tenancy(self):
        row = report.scip_coverage(index(["a.cpp", "a.cpp", "outside.cpp"], "v0.4.0"),
                                   ["a.cpp", "missing.cpp"], "v0.4.0")
        self.assertEqual(row, {"document_records": 3, "unique_document_paths": 2,
                               "tenancy_files": 2, "tenancy_intersection": 1,
                               "outside_tenancy": 1, "unattested_tenancy": 1})
        with self.assertRaises(report.Refusal):
            report.scip_coverage(index(["../escape.cpp"], "v0.4.0"), ["a.cpp"], "v0.4.0")

    def test_complete_receipt_privacy_and_no_clobber(self):
        root, out, base = audit_fixture(self)
        result = report.produce(root, out, workspace_roots=[base], audit_module=audit,
                                screen_binary=base / "cpp-screen-bin")
        published = out.read_text()
        self.assertEqual(result["status"], "complete")
        self.assertNotIn("PRIVATE", published)
        self.assertNotIn("/private/", published)
        self.assertNotIn("a.cpp", published)
        self.assertEqual(set(result["subjects"]), set(audit.PINS))
        self.assertEqual(result["screen_binary_sha256"], sha((base / "cpp-screen-bin").read_bytes()))
        self.assertEqual(result["producer_sha256"], sha((HERE / "cpp-report.py").read_bytes()))
        self.assertEqual(result["commit"], report.BASELINE)
        self.assertEqual(result["driver_sha256"], HISTORICAL_DRIVER_SHA256)
        self.assertEqual(result["current_audit_helper_sha256"], sha(AUDIT_PATH.read_bytes()))
        self.assertEqual(result["indexbench_sha256"], "b" * 64)
        self.assertEqual(result["ctags_sha256"], "c" * 64)
        self.assertEqual(result["clang_sha256"], "a" * 64)
        self.assertEqual(result["scip_clang_sha256"], audit.INDEX_BINARY)
        self.assertEqual(result["readings"]["cpp-full-v0.4.0.json"]["run_rates_mb_cpu_s"],
                         [1.0, 1.0, 1.0])
        self.assertIsNone(result["readings"]["cpp-full-v0.4.0.json"]["run_loads_1m"])
        self.assertEqual(result["readings"]["cpp-full-v0.4.0.json"]["load_condition"],
                         "unrecorded")
        with self.assertRaises(report.Refusal):
            report.produce(root, out, workspace_roots=[base], audit_module=audit,
                                screen_binary=base / "cpp-screen-bin")
        self.assertEqual(out.read_text(), published)

    def test_clean_repo_checked_before_and_after_processing(self):
        root, out, base = audit_fixture(self)
        with mock.patch.object(report, "_require_clean_repo", side_effect=report.Refusal("dirty")):
            with self.assertRaisesRegex(report.Refusal, "dirty"):
                report.produce(root, out, workspace_roots=[base], audit_module=audit,
                               screen_binary=base / "cpp-screen-bin")
        self.assertFalse(out.exists())
        with mock.patch.object(report, "_require_clean_repo", side_effect=[None, report.Refusal("dirty")]) as clean:
            with self.assertRaisesRegex(report.Refusal, "dirty"):
                report.produce(root, out, workspace_roots=[base], audit_module=audit,
                               screen_binary=base / "cpp-screen-bin")
            self.assertEqual(clean.call_count, 2)
        self.assertFalse(out.exists())

    def test_receipt_identity_refuses_drift(self):
        for fault in ("commit", "driver", "indexbench", "ctags", "clang", "scip"):
            with self.subTest(fault=fault):
                root, out, base = audit_fixture(self)
                grade_path = root / "readings/cpp-audit-receipt.json"
                grade = json.loads(grade_path.read_text())
                if fault == "commit":
                    grade["commit"] = "0" * 40
                elif fault == "driver":
                    grade["inputs"]["driver_sha256"] = sha(AUDIT_PATH.read_bytes())
                elif fault in ("indexbench", "ctags"):
                    grade["inputs"][fault + "_sha256"] = "0" * 64
                elif fault == "clang":
                    grade["inputs"]["native_inputs"]["clang_sha256"] = "0" * 64
                else:
                    grade["inputs"]["index_evidence"]["indexes"]["fmt"]["v0.4.0"]["scip_sha256"] = "0" * 64
                put(grade_path, grade)
                with self.assertRaises(report.Refusal):
                    report.produce(root, out, workspace_roots=[base], audit_module=audit,
                                   screen_binary=base / "cpp-screen-bin")
                self.assertFalse(out.exists())

    def test_pooled_reading_metrics_and_tiers_refuse_drift(self):
        for field in ("metrics", "metric_tiers", "counts"):
            with self.subTest(field=field):
                root, out, base = audit_fixture(self)
                grade_path = root / "readings/cpp-audit-receipt.json"
                grade = json.loads(grade_path.read_text())
                for version in audit.INDEX_BINARY:
                    path = root / "readings" / f"cpp-full-{version}.json"
                    row = json.loads(path.read_text())
                    if field == "metrics":
                        row[field]["kind_agreement"] = 0.1
                    elif field == "metric_tiers":
                        row[field]["kind_agreement"] = "A"
                    else:
                        row[field]["matched"] += 1
                    put(path, row)
                    grade["readings"][path.name]["sha256"] = sha(path.read_bytes())
                put(grade_path, grade)
                with self.assertRaises(report.Refusal):
                    report.produce(root, out, workspace_roots=[base], audit_module=audit,
                                   screen_binary=base / "cpp-screen-bin")
                self.assertFalse(out.exists())

    def test_input_count_tenancy_and_byte_caps_at_boundary(self):
        root, out, base = audit_fixture(self)
        path = root / "indexes/fmt-v0.4.0.scip"
        raw = path.read_bytes()
        with mock.patch.object(report, "MAX_TENANCY", 1):
            self.assertEqual(report.scip_coverage(raw, ["a.cpp"], "v0.4.0")["tenancy_files"], 1)
            with self.assertRaises(report.Refusal):
                report.scip_coverage(raw, ["a.cpp", "b.cpp"], "v0.4.0")
        snapshots = {root / f"prior-{i}": ("0" * 64, 1) for i in range(report.MAX_INPUTS - 1)}
        self.assertEqual(report._safe_file(path, root, len(raw), [len(raw)], snapshots), raw)
        with self.assertRaises(report.Refusal):
            report._safe_file(root / "indexes/json-v0.4.0.scip", root, len(raw), [len(raw)], snapshots)
        with self.assertRaises(report.Refusal):
            report._safe_file(path, root, len(raw) - 1, [len(raw)], {})
        with self.assertRaises(report.Refusal):
            report._safe_file(path, root, len(raw), [len(raw) - 1], {})
        with mock.patch.object(Path, "read_bytes", side_effect=AssertionError("unbounded read")):
            self.assertEqual(report._safe_file(path, root, len(raw), [len(raw)], {}), raw)

    def test_scip_cap_and_shared_budget_at_boundary(self):
        root, out, base = audit_fixture(self)
        scip_sizes = [p.stat().st_size for p in (root / "indexes").glob("*.scip")]
        cap = max(scip_sizes)
        result = report.produce(root, out, workspace_roots=[base], audit_module=audit,
                                screen_binary=base / "cpp-screen-bin", scip_cap=cap)
        self.assertEqual(result["status"], "complete")
        out.unlink()
        with self.assertRaises(report.Refusal):
            report.produce(root, out, workspace_roots=[base], audit_module=audit,
                           screen_binary=base / "cpp-screen-bin", scip_cap=cap - 1)
        self.assertFalse(out.exists())
        json_size = sum(p.stat().st_size for p in root.rglob("*.json"))
        report.produce(root, out, workspace_roots=[base], audit_module=audit,
                       screen_binary=base / "cpp-screen-bin", json_cap=json_size)
        out.unlink()
        with self.assertRaises(report.Refusal):
            report.produce(root, out, workspace_roots=[base], audit_module=audit,
                           screen_binary=base / "cpp-screen-bin", json_cap=json_size - 1)

    def test_bad_metadata_missing_input_and_cap(self):
        for fault in ("status", "missing", "cap", "path"):
            with self.subTest(fault=fault):
                root, out, base = audit_fixture(self)
                kwargs = {"workspace_roots": [base], "audit_module": audit,
                          "screen_binary": base / "cpp-screen-bin"}
                if fault == "status":
                    p = root / "readings" / "cpp-audit-receipt.json"
                    value = json.loads(p.read_text()); value["status"] = "failed"; put(p, value)
                elif fault == "missing":
                    (root / "indexes" / "fmt-v0.4.0.scip").unlink()
                elif fault == "cap":
                    kwargs["json_cap"] = 1
                else:
                    p = root / "work" / "grade" / "cpp-full-v0.4.0" / "fmt" / "unmatched.json"
                    put(p, {"tenancy_files": ["../escape.cpp"]})
                with self.assertRaises(report.Refusal):
                    report.produce(root, out, **kwargs)
                self.assertFalse(out.exists())

    def test_diagnostic_changes_cover_both_scopes(self):
        root, out, base = audit_fixture(self)
        for scope in ("legacy", "full"):
            path = root / "readings" / f"cpp-{scope}-v0.4.1.json"
            value = json.loads(path.read_text())
            value["subjects"][0]["counts"]["scip_file_edges_every_definer"] += 2
            value["counts"]["scip_file_edges_every_definer"] += 2
            put(path, value)
            receipt = root / "readings" / "cpp-audit-receipt.json"
            grade = json.loads(receipt.read_text())
            grade["readings"][path.name]["sha256"] = sha(path.read_bytes())
            put(receipt, grade)
            for slug in audit.PINS:
                screen = root / "screens" / f"{scope}-{slug}.json"
                row = json.loads(screen.read_text())
                row["reading_sha256"] = sha(path.read_bytes())
                put(screen, row)
        result = report.produce(root, out, workspace_roots=[base], audit_module=audit,
                                screen_binary=base / "cpp-screen-bin")
        for scope in ("legacy", "full"):
            self.assertEqual(result["diagnostic_changes"]["fmt"][scope]
                             ["scip_file_edges_every_definer"], [1, 3])

    def test_same_count_changed_path_refuses(self):
        root, out, base = audit_fixture(self)
        path = root / "work/grade/cpp-full-v0.4.0/fmt/unmatched.json"
        value = json.loads(path.read_text())
        value["tenancy_files"] = ["other.cpp"]
        put(path, value)
        with self.assertRaises(report.Refusal):
            report.produce(root, out, workspace_roots=[base], audit_module=audit,
                                screen_binary=base / "cpp-screen-bin")
        self.assertFalse(out.exists())

    def test_screen_and_native_attestation_refuse(self):
        for fault in ("native_missing", "native_changed", "screen_hash", "screen_population"):
            with self.subTest(fault=fault):
                root, out, base = audit_fixture(self)
                native_path = root / "work/native/fmt/calls.json"
                screen_path = root / "screens/full-fmt.json"
                if fault == "native_missing":
                    native_path.unlink()
                elif fault == "native_changed":
                    native_path.write_bytes(b"changed native")
                else:
                    row = json.loads(screen_path.read_text())
                    row["unmatched_sha256" if fault == "screen_hash" else
                        "population_sha256"] = "0" * 64
                    put(screen_path, row)
                with self.assertRaises(report.Refusal):
                    report.produce(root, out, workspace_roots=[base], audit_module=audit,
                                screen_binary=base / "cpp-screen-bin")
                self.assertFalse(out.exists())

    def test_screen_binary_and_grammar_identity_refuse(self):
        for fault in ("binary", "binary_receipt", "grammar_commit", "grammar_entry_point",
                      "missing_binary_receipt", "missing_grammar_commit",
                      "missing_grammar_entry_point", "symlink_binary"):
            with self.subTest(fault=fault):
                root, out, base = audit_fixture(self)
                if fault == "binary":
                    (base / "cpp-screen-bin").write_bytes(b"wrong binary")
                elif fault == "symlink_binary":
                    (base / "cpp-screen-bin").unlink()
                    (base / "cpp-screen-bin").symlink_to(base / "other-bin")
                else:
                    path = root / "screens/legacy-fmt.json"
                    row = json.loads(path.read_text())
                    field = fault.removeprefix("missing_")
                    if field == "binary_receipt":
                        field = "product_binary_sha256"
                    if fault.startswith("missing_"):
                        row.pop(field)
                    else:
                        row[field] = "wrong"
                    put(path, row)
                with self.assertRaises(report.Refusal):
                    report.produce(root, out, workspace_roots=[base], audit_module=audit,
                                   screen_binary=base / "cpp-screen-bin")
                self.assertFalse(out.exists())

    def test_binary_change_before_publish_refuses(self):
        root, out, base = audit_fixture(self)
        def mutate():
            (base / "cpp-screen-bin").write_bytes(b"changed after read")
        with self.assertRaises(report.Refusal):
            report.produce(root, out, workspace_roots=[base], audit_module=audit,
                           screen_binary=base / "cpp-screen-bin", before_publish=mutate)
        self.assertFalse(out.exists())

    def test_native_change_before_publish_refuses(self):
        root, out, base = audit_fixture(self)
        path = root / "work/native/fmt/preproc.json"
        def mutate():
            path.write_bytes(b"changed after read")
        with self.assertRaises(report.Refusal):
            report.produce(root, out, workspace_roots=[base], audit_module=audit, screen_binary=base / "cpp-screen-bin",
                           before_publish=mutate)
        self.assertFalse(out.exists())

    def test_changed_input_refuses_whole_output(self):
        root, out, base = audit_fixture(self)
        reading_path = root / "readings" / "cpp-full-v0.4.0.json"
        def mutate():
            reading_path.write_bytes(reading_path.read_bytes() + b" ")
        with self.assertRaises(report.Refusal):
            report.produce(root, out, workspace_roots=[base], audit_module=audit, screen_binary=base / "cpp-screen-bin",
                           before_publish=mutate)
        self.assertFalse(out.exists())


if __name__ == "__main__":
    unittest.main()
