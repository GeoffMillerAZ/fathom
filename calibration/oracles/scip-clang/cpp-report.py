#!/usr/bin/env python3
"""Publish bounded, identity-checked C++ audit numbers without source content."""

import argparse
import copy
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile


class Refusal(Exception):
    pass


METRICS = {
    "sym_recall": ("matched", "oracle_defs", "B"),
    "sym_precision": ("matched", "fathom_defs", "B"),
    "kind_agreement": ("kind_equal", "matched", "B"),
    "range_agreement": ("range_ok", "range_eligible", "B"),
    "signature_coverage": ("callables_signed", "callables", "B"),
    "call_recall": ("call_truth_found", "call_truth", "A"),
    "call_precision": ("call_fathom_in_truth", "call_fathom", "A"),
    "import_resolution": ("imports_resolved_agree", "imports_in_repo", "A"),
    "file_edge_precision": ("file_edges_agree", "file_edges_fathom", "A"),
    "file_edge_recall": ("file_edges_agree", "file_edges_truth", "A"),
    "def_mapping": ("scip_defs_mapped", "scip_defs", "A"),
    "typed_recall": ("scip_edges_typed", "scip_edges", "A"),
}
DIAGNOSTICS = frozenset(("scip_file_edges_every_definer", "scip_edges_macro_generated"))
SCOPES = ("legacy", "full")
MAX_INPUTS = 48
BASELINE = "0deba117abaadee0343a58a1645f9232a92b6167"
GRAMMAR = "b5ca4b0cd346bc1cd1060fdc2151125a11ad7695da595157667a5e8b30b03e34"
GRAMMAR_COMMIT = "f41e1a044c8a84ea9fa8577fdd2eab92ec96de02"
GRAMMAR_ENTRY = "tree_sitter_cpp"
SCREEN_BINARY_SHA256 = "f7c4c351006b4b59cd76eedef27f03ba093e0ec4eae088f15e8b972fb496cdd4"
SCREEN_BINARY_CAP = 64 << 20
NATIVE_FILES = ("calls.json", "preproc.json", "preproc.ranges.json")
SYNTAX_FIELDS = ("error_containing", "missing_containing", "both", "error_only",
                 "missing_only", "union", "faults")
MAX_TENANCY = 100000
DRIVER_PATH = "calibration/oracles/scip-clang/cpp-audit.py"


def _git(repo, *args):
    try:
        result = subprocess.run(["git", "-C", str(repo), *args], capture_output=True,
                                timeout=30, check=False)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise Refusal("git identity check unavailable") from exc
    if result.returncode:
        raise Refusal("git identity check failed")
    return result.stdout


def _require_clean_repo():
    repo = Path(__file__).resolve().parents[3]
    if _git(repo, "status", "--porcelain", "--untracked-files=all"):
        raise Refusal("nonignored fathom tree changed")


def _historical_driver_sha256():
    repo = Path(__file__).resolve().parents[3]
    return hashlib.sha256(_git(repo, "show", f"{BASELINE}:{DRIVER_PATH}")).hexdigest()


def _current_revision():
    repo = Path(__file__).resolve().parents[3]
    return _git(repo, "rev-parse", "HEAD").decode("ascii").strip()


def _bounded_bytes(path, cap):
    if cap < 0:
        raise Refusal("invalid input cap")
    with path.open("rb") as stream:
        data = stream.read(cap + 1)
    if len(data) > cap:
        raise Refusal("input absent or over cap")
    return data

def _integer(value):
    if type(value) is not int or value < 0:
        raise Refusal("invalid count")
    return value


def metric_rows(subject):
    counts = subject.get("counts")
    if not isinstance(counts, dict):
        raise Refusal("subject counts absent")
    rows = {}
    for name, (num_key, den_key, tier) in METRICS.items():
        n, d = _integer(counts.get(num_key)), _integer(counts.get(den_key))
        if name == "sym_precision":
            d -= _integer(counts.get("adjudicated", 0))
        elif name in ("call_precision", "file_edge_precision"):
            blind = "calls_oracle_blind" if name == "call_precision" else "file_edges_oracle_blind"
            b = _integer(counts.get(blind, 0))
            # merge.go: an exclusion over 10% of this subject's own total
            # returns to the precision denominator.
            if b * 10 > (d + b):
                d += b
        if d < 0 or (d and n > d):
            raise Refusal("incoherent metric counts")
        rows[name] = {"numerator": n, "denominator": d,
                      "ratio": n / d if d else None, "tier": tier}
    return rows


def _relative(value):
    if not isinstance(value, str) or not value or value.startswith("/") or \
            "\\" in value or "\0" in value or any(
                part in ("", ".", "..") for part in value.split("/")):
        raise Refusal("unsafe relative path")
    return value


def scip_coverage(raw, tenancy, version, *, wire_fields=None):
    if wire_fields is None:
        wire_fields = _audit().wire_fields
    if not isinstance(tenancy, list) or len(tenancy) > MAX_TENANCY:
        raise Refusal("invalid tenancy population")
    selected = {_relative(path) for path in tenancy}
    if len(selected) != len(tenancy):
        raise Refusal("duplicate tenancy file")
    records, paths, meta = 0, set(), []
    try:
        for field, value in wire_fields(raw):
            if field == 1:
                meta.append(value)
            elif field == 2:
                records += 1
                path = [v.decode("utf-8") for f, v in wire_fields(value) if f == 1]
                if len(path) != 1:
                    raise Refusal("SCIP document path absent")
                paths.add(_relative(path[0]))
        if len(meta) != 1 or not records:
            raise Refusal("SCIP metadata or documents absent")
        tools = [v for f, v in wire_fields(meta[0]) if f == 2]
        if len(tools) != 1:
            raise Refusal("SCIP tool absent")
        fields = list(wire_fields(tools[0]))
        names = [v.decode("utf-8") for f, v in fields if f == 1]
        versions = [v.decode("utf-8") for f, v in fields if f == 2]
        if names != ["scip-clang"] or versions != [version.removeprefix("v")]:
            raise Refusal("SCIP tool identity mismatch")
    except (UnicodeError, ValueError) as exc:
        raise Refusal("invalid SCIP wire") from exc
    overlap = len(paths & selected)
    return {"document_records": records, "unique_document_paths": len(paths),
            "tenancy_files": len(selected), "tenancy_intersection": overlap,
            "outside_tenancy": len(paths) - overlap,
            "unattested_tenancy": len(selected) - overlap}


def _audit():
    path = Path(__file__).with_name("cpp-audit.py")
    if not path.is_file() or path.is_symlink():
        raise Refusal("sibling audit module absent")
    sys.dont_write_bytecode = True
    spec = importlib.util.spec_from_file_location("cpp_audit", path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def _safe_file(path, root, cap, budget, snapshots):
    if path in snapshots or len(snapshots) >= MAX_INPUTS:
        raise Refusal("duplicate or excessive input")
    if root not in path.parents:
        raise Refusal("input outside audit root")
    cursor = root
    if cursor.is_symlink() or not cursor.is_dir():
        raise Refusal("unsafe audit root")
    for part in path.relative_to(root).parts[:-1]:
        cursor /= part
        if cursor.is_symlink() or not cursor.is_dir():
            raise Refusal("unsafe input directory")
    try:
        info = path.lstat()
        if not stat.S_ISREG(info.st_mode) or info.st_size > cap or info.st_size > budget[0]:
            raise Refusal("input absent or over cap")
        data = _bounded_bytes(path, min(cap, budget[0]))
        if len(data) != info.st_size:
            raise Refusal("input changed during read")
        digest = hashlib.sha256(data).hexdigest()
        if hashlib.sha256(_bounded_bytes(path, min(cap, budget[0]))).hexdigest() != digest:
            raise Refusal("input changed during read")
    except OSError as exc:
        raise Refusal("input unreadable") from exc
    budget[0] -= len(data)
    snapshots[path] = (digest, cap)
    return data


def _read_json(path, root, cap, budget, snapshots):
    try:
        return json.loads(_safe_file(path, root, cap, budget, snapshots))
    except (ValueError, UnicodeError) as exc:
        raise Refusal("malformed JSON input") from exc


def _same_grading(a, b):
    if a.get("metrics") != b.get("metrics") or a.get("metric_tiers") != b.get("metric_tiers"):
        raise Refusal("version graded metrics changed")
    for key in ("dialect", "fathom_commit", "target_level", "level_met",
                "grade", "extractor", "required_kinds", "kind_intersection",
                "excluded_by_kind"):
        if a.get(key) != b.get(key):
            raise Refusal("version graded identity changed")
    oa = [(o.get("tool"), o.get("tier"), None if o.get("tool") == "scip-clang" else o.get("version"))
          for o in a.get("oracles", [])]
    ob = [(o.get("tool"), o.get("tier"), None if o.get("tool") == "scip-clang" else o.get("version"))
          for o in b.get("oracles", [])]
    if oa != ob:
        raise Refusal("version oracle inputs changed")
    ca, cb = a.get("counts", {}), b.get("counts", {})
    if set(ca) != set(cb) or any(ca[k] != cb[k] for k in ca if k not in DIAGNOSTICS):
        raise Refusal("version aggregate counts changed")
    left, right = a.get("subjects"), b.get("subjects")
    if not isinstance(left, list) or not isinstance(right, list) or len(left) != len(right):
        raise Refusal("version subject set changed")
    for x, y in zip(left, right):
        for key in ("name", "sha", "files", "bytes", "families", "kind_intersection", "excluded_by_kind"):
            if x.get(key) != y.get(key):
                raise Refusal("version population changed")
        cx, cy = x.get("counts"), y.get("counts")
        if not isinstance(cx, dict) or not isinstance(cy, dict) or set(cx) != set(cy):
            raise Refusal("version count keys changed")
        if any(cx[key] != cy[key] for key in cx if key not in DIAGNOSTICS):
            raise Refusal("version graded count changed")
        tx, ty = x.get("tenancy", {}), y.get("tenancy", {})
        for field in ("faults", "faulting_files", "files_with_error"):
            if tx.get(field) != ty.get(field):
                raise Refusal("version graded tenancy changed")


def _validate_pooled(reading):
    subjects = reading.get("subjects")
    metrics = reading.get("metrics")
    tiers = reading.get("metric_tiers")
    aggregate = reading.get("counts")
    if not isinstance(subjects, list) or not isinstance(metrics, dict) or \
            not isinstance(tiers, dict) or not isinstance(aggregate, dict):
        raise Refusal("reading pooled metrics absent")
    subject_counts = [subject.get("counts") for subject in subjects]
    if any(not isinstance(counts, dict) for counts in subject_counts):
        raise Refusal("subject counts absent")
    keys = set().union(*(counts.keys() for counts in subject_counts))
    if set(aggregate) != keys or any(
            _integer(aggregate[key]) != sum(_integer(counts.get(key, 0))
                                            for counts in subject_counts)
            for key in keys):
        raise Refusal("reading aggregate count mismatch")
    rows = [metric_rows(subject) for subject in subjects]
    for name in METRICS:
        n = sum(row[name]["numerator"] for row in rows)
        d = sum(row[name]["denominator"] for row in rows)
        value = math.floor(n / d * 10000 + 0.5) / 10000 if d else None
        if metrics.get(name) != value or tiers.get(name) != METRICS[name][2]:
            raise Refusal("reading pooled metric or tier mismatch")


def _timing_measurement(reading):
    value = copy.deepcopy(reading)
    value.pop("grade", None)
    value.pop("level_met", None)
    value["notes"] = sorted(note for note in value.get("notes", [])
                            if not note.startswith("throughput (design 1.9 T1, T2)"))
    timing_keys = ("throughput_mb_s", "wall_throughput_mb_s", "max_file_ms")
    tenancy = value.get("tenancy", {})
    for key in timing_keys:
        tenancy.pop(key, None)
    if not tenancy:
        value.pop("tenancy", None)
    for subject in value.get("subjects", []):
        tenancy = subject.get("tenancy", {})
        for key in ("extract_ms", "extract_cpu_ms", "max_file_ms"):
            tenancy.pop(key, None)
        if not tenancy:
            subject.pop("tenancy", None)
    return value


def _timing_row(original, fresh, log, old_hash, new_hash, current_revision, binary_hash):
    if _timing_measurement(original) != _timing_measurement(fresh):
        raise Refusal("retime changed measured reading")
    old_tenancy = original.get("tenancy", {})
    new_tenancy = fresh.get("tenancy", {})
    old_floor = (old_tenancy.get("throughput_mb_s", 0) >= 1,
                 old_tenancy.get("max_file_ms", 0) <= 2000)
    new_floor = (new_tenancy.get("throughput_mb_s", 0) >= 1,
                 new_tenancy.get("max_file_ms", 0) <= 2000)
    if old_floor == new_floor and (original.get("grade") != fresh.get("grade") or
                                   original.get("level_met") != fresh.get("level_met")):
        raise Refusal("retime changed grade without timing floor change")
    if log.get("schema") != "fathom-lang-retime-load-v1" or \
            log.get("reading_before_sha256") != old_hash or \
            log.get("reading_after_sha256") != new_hash or \
            log.get("os") != "darwin" or log.get("arch") != "arm64" or \
            log.get("product_revision") != current_revision or \
            log.get("product_binary_sha256") != binary_hash or \
            log.get("product_modified") is not False:
        raise Refusal("retime log provenance mismatch")
    runs = log.get("runs")
    if not isinstance(runs, list) or len(runs) != 3:
        raise Refusal("retime runs incomplete")
    rates, loads = [], []
    for number, run in enumerate(runs, 1):
        if not isinstance(run, dict) or type(run.get("number")) is not int or run["number"] != number:
            raise Refusal("retime run sequence mismatch")
        for key in ("rate_mb_cpu_s", "load_before", "load_after"):
            value = run.get(key)
            if isinstance(value, bool) or not isinstance(value, (int, float)) or \
                    not math.isfinite(value) or value < 0 or (key == "rate_mb_cpu_s" and value == 0):
                raise Refusal("retime run value invalid")
        rates.append(run["rate_mb_cpu_s"])
        loads.append({"before": run["load_before"], "after": run["load_after"]})
    median_index = sorted(range(3), key=lambda i: (rates[i], i))[1]
    median = rates[median_index]
    if new_tenancy.get("throughput_mb_s") != median:
        raise Refusal("retime median disagrees with reading")
    return {"sha256": new_hash, "median_run": median_index + 1,
            "throughput_mb_cpu_s": median, "run_rates_mb_cpu_s": rates,
            "run_loads_1m": loads, "load_condition": "observed"}


def _screen(screen, reading_hash, unmatched_hash, tenancy, measured, scope, slug, binary_hash):
    population = hashlib.sha256("".join(path + "\n" for path in tenancy).encode()).hexdigest()
    complete = screen.get("status") == "complete" and screen.get("counts_complete") is True
    partial = (scope, slug) == ("full", "json") and screen.get("status") == "failed" and \
        screen.get("failure_code") == "parse_fault" and screen.get("counts_complete") is False
    if not (complete or partial) or screen.get("reading_sha256") != reading_hash or \
            screen.get("unmatched_sha256") != unmatched_hash or \
            screen.get("population_sha256") != population or \
            screen.get("product_revision") != BASELINE or \
            screen.get("grammar_blob_sha256") != GRAMMAR or \
            screen.get("grammar_commit") != GRAMMAR_COMMIT or \
            screen.get("grammar_entry_point") != GRAMMAR_ENTRY or \
            screen.get("product_binary_sha256") != binary_hash or \
            not isinstance(screen.get("files"), int) or screen["files"] < 0 or \
            (complete and screen["files"] != measured) or screen["files"] > measured:
        raise Refusal("screen receipt does not attest population and inputs")
    row = {"status": screen["status"], "counts_complete": complete,
           "files_screened": screen["files"], "files_total": measured}
    if complete:
        for phase in ("written", "selected"):
            counts = screen.get(phase)
            if not isinstance(counts, dict):
                raise Refusal("complete screen counts absent")
            row[phase] = {key: _integer(counts.get(key)) for key in SYNTAX_FIELDS}
    return row


def produce(audit_root, out, *, workspace_roots=None, audit_module=None,
            json_cap=256 << 20, scip_cap=256 << 20, before_publish=None,
            screen_binary=None, timing_dir=None):
    _require_clean_repo()
    audit = audit_module or _audit()
    if Path(audit_root).is_symlink() or Path(out).parent.is_symlink():
        raise Refusal("symlinked audit root or output directory")
    try:
        root = Path(audit_root).resolve(strict=True)
        target = Path(out).parent.resolve(strict=True) / Path(out).name
    except OSError as exc:
        raise Refusal("audit root or output directory absent") from exc
    if root.name != "audit-r3" or target.exists() or target.is_symlink():
        raise Refusal("audit root or output invalid")
    if workspace_roots is None:
        workspace_roots = audit.workspace_roots()
    roots = [Path(p).resolve(strict=True) for p in workspace_roots if Path(p).is_dir()]
    if not roots:
        raise Refusal("no existing ignored workspace root")
    if not any(base == target.parent or base in target.parents for base in roots) or \
            not any(base == root or base in root.parents for base in roots):
        raise Refusal("output or audit root outside workspace")
    if target == root or root in target.parents or target in root.parents:
        raise Refusal("output overlaps audit inputs")
    timing_root = None
    if timing_dir is not None:
        directory = Path(timing_dir)
        if not directory.is_absolute() or directory != Path(os.path.abspath(directory)) or \
                directory.is_symlink():
            raise Refusal("timing directory must be canonical and regular")
        try:
            timing_root = directory.resolve(strict=True)
        except OSError as exc:
            raise Refusal("timing directory absent") from exc
        if directory != timing_root or not timing_root.is_dir() or not any(
                base == timing_root or base in timing_root.parents
                                               for base in roots) or \
                timing_root == root or timing_root in root.parents or root in timing_root.parents or \
                timing_root == target or timing_root in target.parents or target in timing_root.parents:
            raise Refusal("timing directory outside workspace or overlaps inputs")
    cursor = target.parent
    if not cursor.is_dir() or cursor.is_symlink():
        raise Refusal("output directory absent or symlinked")
    for base in roots:
        if base == target.parent or base in target.parents:
            cursor = base
            for part in target.parent.relative_to(base).parts:
                cursor /= part
                if cursor.is_symlink():
                    raise Refusal("output directory symlinked")
    if screen_binary is None:
        raise Refusal("baseline screen binary absent")
    binary = Path(screen_binary)
    if not binary.is_absolute() or binary != Path(os.path.abspath(binary)):
        raise Refusal("screen binary path must be absolute and canonical")
    binary_roots = [base for base in roots if base in binary.parents]
    if not binary_roots:
        raise Refusal("baseline screen binary outside workspace")
    binary_snapshots = {}
    try:
        _safe_file(binary, binary_roots[0], SCREEN_BINARY_CAP,
                   [SCREEN_BINARY_CAP], binary_snapshots)
        binary_hash = binary_snapshots[binary][0]
        if binary_hash != SCREEN_BINARY_SHA256:
            raise Refusal("baseline screen binary hash mismatch")
        producer = Path(__file__)
        if producer.is_symlink() or not producer.is_file():
            raise Refusal("report producer absent or symlinked")
        if producer.stat().st_size > 1 << 20:
            raise Refusal("report producer over cap")
        producer_hash = hashlib.sha256(_bounded_bytes(producer, 1 << 20)).hexdigest()
    except OSError as exc:
        raise Refusal("binary or producer unavailable") from exc
    json_budget, scip_budget, snapshots = [json_cap], [scip_cap * 6], {}
    index_receipt = _read_json(root / "indexes/candidate-indexes.json", root, json_cap, json_budget, snapshots)
    grade = _read_json(root / "readings/cpp-audit-receipt.json", root, json_cap, json_budget, snapshots)
    native = _read_json(root / "work/native/receipt.json", root, json_cap, json_budget, snapshots)
    historical_driver_hash = _historical_driver_sha256()
    helper = Path(audit.__file__)
    if helper.is_symlink() or not helper.is_file() or helper.stat().st_size > 1 << 20:
        raise Refusal("current audit helper absent or over cap")
    helper_hash = hashlib.sha256(_bounded_bytes(helper, 1 << 20)).hexdigest()
    inputs = grade.get("inputs")
    if not isinstance(inputs, dict):
        raise Refusal("grade inputs absent")
    native_inputs = native.get("inputs")
    identity = inputs.get("tool_identity")
    if not isinstance(native_inputs, dict) or not isinstance(identity, dict):
        raise Refusal("tool identity absent")
    indexbench = identity.get("indexbench")
    ctags = identity.get("ctags")
    if not isinstance(indexbench, dict) or not isinstance(ctags, dict) or \
            indexbench.get("source") != "cmd/indexbench" or \
            indexbench.get("revision") != BASELINE or indexbench.get("modified") is not False or \
            indexbench.get("sha256") != inputs.get("indexbench_sha256") or \
            ctags.get("version") != "6.2.1" or ctags.get("sha256") != inputs.get("ctags_sha256"):
        raise Refusal("tool identity mismatch")
    for value in (inputs.get("indexbench_sha256"), inputs.get("ctags_sha256"),
                  native_inputs.get("clang_sha256")):
        if not isinstance(value, str) or len(value) != 64 or any(c not in "0123456789abcdef" for c in value):
            raise Refusal("tool hash absent")
    if index_receipt.get("status") != "complete" or grade.get("status") != "complete" or \
            native.get("status") != "complete" or \
            index_receipt.get("producer") != "calibration/oracles/scip-clang/cpp-audit.py" or \
            grade.get("commit") != BASELINE or \
            index_receipt.get("driver_sha256") != historical_driver_hash or \
            inputs.get("driver_sha256") != historical_driver_hash or \
            native_inputs.get("driver_sha256") != historical_driver_hash or \
            inputs.get("native_inputs") != native_inputs or \
            inputs.get("native_outputs") != native.get("outputs"):
        raise Refusal("incomplete or unbound audit receipts")
    native_outputs = native.get("outputs")
    if not isinstance(native_outputs, dict) or set(native_outputs) != set(audit.PINS):
        raise Refusal("native output subject set mismatch")
    for slug in audit.PINS:
        files = native_outputs[slug]
        if not isinstance(files, dict) or set(files) != set(NATIVE_FILES):
            raise Refusal("native output family set mismatch")
        for name in NATIVE_FILES:
            path = root / "work/native" / slug / name
            _safe_file(path, root, json_cap, json_budget, snapshots)
            if snapshots[path][0] != files[name]:
                raise Refusal("native output digest mismatch")
    rows = index_receipt.get("rows")
    if not isinstance(rows, list) or len(rows) != 6:
        raise Refusal("incomplete index rows")
    index_rows = {(row.get("subject"), row.get("version")): row for row in rows}
    if len(index_rows) != 6:
        raise Refusal("duplicate index rows")
    if grade["inputs"].get("index_evidence", {}).get("receipt_sha256") != snapshots[root / "indexes/candidate-indexes.json"][0]:
        raise Refusal("grade/index receipt mismatch")
    readings = {}
    receipt_rows = {}
    for scope in SCOPES:
        for version in audit.INDEX_BINARY:
            name = f"cpp-{scope}-{version}.json"
            value = _read_json(root / "readings" / name, root, json_cap, json_budget, snapshots)
            attested = grade.get("readings", {}).get(name, {})
            if attested.get("sha256") != snapshots[root / "readings" / name][0] or attested.get("subjects") != 3 or \
                    value.get("fathom_commit") != grade.get("commit"):
                raise Refusal("reading receipt mismatch")
            rates = attested.get("run_rates_mb_cpu_s")
            median_run = attested.get("median_run")
            throughput = attested.get("throughput_mb_cpu_s")
            if not isinstance(rates, list) or len(rates) != 3 or any(
                    isinstance(rate, bool) or not isinstance(rate, (float, int)) or
                    not math.isfinite(rate) or rate <= 0 for rate in rates) or \
                    type(median_run) is not int or median_run not in (1, 2, 3) or \
                    isinstance(throughput, bool) or not isinstance(throughput, (float, int)) or \
                    throughput != rates[median_run - 1]:
                raise Refusal("reading timing receipt mismatch")
            try:
                audit.validate_cpp_reading(value, version)
            except audit.Refusal as exc:
                raise Refusal("reading validation failed") from exc
            _validate_pooled(value)
            readings[(scope, version)] = value
            receipt_rows[name] = {"sha256": attested["sha256"],
                                  "median_run": median_run,
                                  "throughput_mb_cpu_s": throughput,
                                  "run_rates_mb_cpu_s": rates,
                                  "run_loads_1m": None,
                                  "load_condition": "unrecorded"}
        _same_grading(readings[(scope, "v0.4.0")], readings[(scope, "v0.4.1")])
    timing_snapshots = {}
    timing_product_revision = None
    timing_product_binary_sha256 = None
    if timing_root is not None:
        timing_product_revision = _current_revision()
        if len(timing_product_revision) != 40 or any(
                char not in "0123456789abcdef" for char in timing_product_revision):
            raise Refusal("current timing revision invalid")
        for scope in SCOPES:
            for version in audit.INDEX_BINARY:
                name = f"cpp-{scope}-{version}.json"
                reading_path = timing_root / name
                log_path = timing_root / (name + ".loads.json")
                fresh = _read_json(reading_path, timing_root, json_cap, json_budget, snapshots)
                log = _read_json(log_path, timing_root, json_cap, json_budget, snapshots)
                try:
                    audit.validate_cpp_reading(fresh, version)
                except audit.Refusal as exc:
                    raise Refusal("retimed reading validation failed") from exc
                _validate_pooled(fresh)
                timing_log_binary = log.get("product_binary_sha256")
                if not isinstance(timing_log_binary, str) or len(timing_log_binary) != 64 or any(
                        char not in "0123456789abcdef" for char in timing_log_binary):
                    raise Refusal("timing binary hash invalid")
                if timing_product_binary_sha256 is None:
                    timing_product_binary_sha256 = timing_log_binary
                row = _timing_row(readings[(scope, version)], fresh, log,
                                  receipt_rows[name]["sha256"], snapshots[reading_path][0],
                                  timing_product_revision, timing_product_binary_sha256)
                row["historical_timing"] = receipt_rows[name]
                receipt_rows[name] = row
                timing_snapshots[name] = {"reading_sha256": snapshots[reading_path][0],
                                          "load_log_sha256": snapshots[log_path][0]}
    result = {"status": "complete", "dialect": "cpp", "subjects": {},
              "readings": receipt_rows, "indexes": {}, "audit_inputs_sha256": {},
              "diagnostic_changes": {}, "syntax": {},
              "commit": BASELINE, "driver_sha256": historical_driver_hash,
              "current_audit_helper_sha256": helper_hash,
              "indexbench_sha256": inputs["indexbench_sha256"],
              "ctags_sha256": inputs["ctags_sha256"],
              "clang_sha256": native_inputs["clang_sha256"],
              "scip_clang_sha256": dict(audit.INDEX_BINARY),
              "screen_binary_sha256": binary_hash, "producer_sha256": producer_hash}
    if timing_root is not None:
        result["timing_product_revision"] = timing_product_revision
        result["timing_product_binary_sha256"] = timing_product_binary_sha256
        result["timing_inputs"] = timing_snapshots
    for slug, pin in audit.PINS.items():
        result["subjects"][slug] = {}
        result["indexes"][slug] = {}
        result["syntax"][slug] = {}
        populations_by_scope = {}
        for scope in SCOPES:
            value = readings[(scope, "v0.4.0")]
            sub = next((s for s in value["subjects"] if s["name"] == pin[1]), None)
            if sub is None or sub.get("sha") != pin[0]:
                raise Refusal("subject pin mismatch")
            tenancy = sub.get("tenancy", {})
            faults = tenancy.get("faulting_files", [])
            fault_count = len(faults) if isinstance(faults, list) else _integer(faults)
            if not isinstance(faults, (list, int)) or fault_count > sub["files"]:
                raise Refusal("fault count invalid")
            result["subjects"][slug][scope] = {
                "files": _integer(sub["files"]), "bytes": _integer(sub["bytes"]),
                "faults": _integer(tenancy.get("faults", 0)), "faulting_files": fault_count,
                "files_with_error": _integer(tenancy.get("files_with_error", 0)),
                "metrics": metric_rows(sub)}
        for version in audit.INDEX_BINARY:
            row = index_rows.get((pin[1], version))
            idx_path = root / "indexes" / f"{slug}-{version}.scip"
            raw = _safe_file(idx_path, root, scip_cap, scip_budget, snapshots)
            evidence = grade["inputs"]["index_evidence"]["indexes"][slug][version]
            if row is None or row.get("subject_sha") != pin[0] or \
                    row.get("binary_sha256") != audit.INDEX_BINARY[version] or \
                    row.get("scip_sha256") != snapshots[idx_path][0] or \
                    evidence.get("scip_sha256") != row["scip_sha256"] or \
                    evidence.get("documents") != row.get("documents"):
                raise Refusal("index evidence mismatch")
            populations = {}
            for scope in SCOPES:
                path = root / "work/grade" / f"cpp-{scope}-{version}" / slug / "unmatched.json"
                unmatched = _read_json(path, root, json_cap, json_budget, snapshots)
                coverage = scip_coverage(raw, unmatched.get("tenancy_files"), version,
                                         wire_fields=audit.wire_fields)
                if coverage["document_records"] != row.get("documents"):
                    raise Refusal("index document count mismatch")
                measured = next(s["files"] for s in readings[(scope, version)]["subjects"]
                                if s["name"] == pin[1])
                if coverage["tenancy_files"] != measured:
                    raise Refusal("grade tenancy count mismatch")
                populations[scope] = coverage
                populations_by_scope[(scope, version)] = (unmatched["tenancy_files"], snapshots[path][0])
            result["indexes"][slug][version] = populations
        result["diagnostic_changes"][slug] = {}
        for scope in SCOPES:
            old_paths, _ = populations_by_scope[(scope, "v0.4.0")]
            paths, unmatched_hash = populations_by_scope[(scope, "v0.4.1")]
            if old_paths != paths:
                raise Refusal("SCIP versions grade different tenancy paths")
            screen = _read_json(root / "screens" / f"{scope}-{slug}.json", root,
                                json_cap, json_budget, snapshots)
            measured = next(s["files"] for s in readings[(scope, "v0.4.1")]["subjects"]
                            if s["name"] == pin[1])
            screen_reading = receipt_rows[f"cpp-{scope}-v0.4.1.json"]
            result["syntax"][slug][scope] = _screen(
                screen, screen_reading.get("historical_timing", screen_reading)["sha256"],
                unmatched_hash, paths, measured, scope, slug, binary_hash)
            first = readings[(scope, "v0.4.0")]["subjects"]
            second = readings[(scope, "v0.4.1")]["subjects"]
            x = next(s for s in first if s["name"] == pin[1])["counts"]
            y = next(s for s in second if s["name"] == pin[1])["counts"]
            result["diagnostic_changes"][slug][scope] = {
                key: [x[key], y[key]] for key in DIAGNOSTICS if x.get(key) != y.get(key)}
    if set(grade.get("readings", {})) != set(receipt_rows):
        raise Refusal("unexpected reading receipt")
    result["audit_inputs_sha256"] = {
        ("timing/" + path.relative_to(timing_root).as_posix()
         if timing_root is not None and timing_root in path.parents else path.relative_to(root).as_posix()): digest
        for path, (digest, _) in sorted(snapshots.items())}
    if before_publish:
        before_publish()
    _require_clean_repo()
    if binary.is_symlink() or not binary.is_file() or binary.stat().st_size > SCREEN_BINARY_CAP or \
            hashlib.sha256(_bounded_bytes(binary, SCREEN_BINARY_CAP)).hexdigest() != binary_hash or \
            producer.is_symlink() or hashlib.sha256(_bounded_bytes(producer, 1 << 20)).hexdigest() != producer_hash or \
            helper.is_symlink() or hashlib.sha256(_bounded_bytes(helper, 1 << 20)).hexdigest() != helper_hash:
        raise Refusal("binary or producer changed before publish")
    for path, (digest, cap) in snapshots.items():
        if path.is_symlink() or not path.is_file() or path.stat().st_size > cap or \
                hashlib.sha256(_bounded_bytes(path, cap)).hexdigest() != digest:
            raise Refusal("audit input changed before publish")
    data = (json.dumps(result, indent=2, sort_keys=True, allow_nan=False) + "\n").encode()
    temp = None
    try:
        with tempfile.NamedTemporaryFile(dir=target.parent, prefix=".cpp-report-", delete=False) as stream:
            temp = Path(stream.name)
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.link(temp, target)
    except FileExistsError as exc:
        raise Refusal("output exists") from exc
    finally:
        if temp is not None:
            temp.unlink(missing_ok=True)
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--audit-root", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--screen-binary", type=Path, required=True)
    parser.add_argument("--timing-dir", type=Path)
    args = parser.parse_args()
    try:
        produce(args.audit_root, args.out, screen_binary=args.screen_binary,
                timing_dir=args.timing_dir)
    except Refusal as exc:
        parser.error(str(exc))
