#!/usr/bin/env python3
"""Publish bounded, identity-checked C++ audit numbers without source content."""

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import stat
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
        data = path.read_bytes()
        if len(data) != info.st_size:
            raise Refusal("input changed during read")
        digest = hashlib.sha256(data).hexdigest()
        if hashlib.sha256(path.read_bytes()).hexdigest() != digest:
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
            screen_binary=None):
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
        producer_hash = hashlib.sha256(producer.read_bytes()).hexdigest()
    except OSError as exc:
        raise Refusal("binary or producer unavailable") from exc
    json_budget, scip_budget, snapshots = [json_cap], [scip_cap * 6], {}
    index_receipt = _read_json(root / "indexes/candidate-indexes.json", root, json_cap, json_budget, snapshots)
    grade = _read_json(root / "readings/cpp-audit-receipt.json", root, json_cap, json_budget, snapshots)
    native = _read_json(root / "work/native/receipt.json", root, json_cap, json_budget, snapshots)
    driver_hash = hashlib.sha256(Path(audit.__file__).read_bytes()).hexdigest()
    if index_receipt.get("status") != "complete" or grade.get("status") != "complete" or \
            native.get("status") != "complete" or \
            index_receipt.get("producer") != "calibration/oracles/scip-clang/cpp-audit.py" or \
            index_receipt.get("driver_sha256") != driver_hash or \
            grade.get("inputs", {}).get("driver_sha256") != driver_hash or \
            grade["inputs"].get("native_inputs") != native.get("inputs") or \
            grade["inputs"].get("native_outputs") != native.get("outputs"):
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
            try:
                audit.validate_cpp_reading(value, version)
            except audit.Refusal as exc:
                raise Refusal("reading validation failed") from exc
            readings[(scope, version)] = value
            receipt_rows[name] = {"sha256": attested["sha256"],
                                  "median_run": attested.get("median_run"),
                                  "throughput_mb_cpu_s": attested.get("throughput_mb_cpu_s")}
        _same_grading(readings[(scope, "v0.4.0")], readings[(scope, "v0.4.1")])
    result = {"status": "complete", "dialect": "cpp", "subjects": {},
              "readings": receipt_rows, "indexes": {}, "audit_inputs_sha256": {},
              "diagnostic_changes": {}, "syntax": {},
              "screen_binary_sha256": binary_hash, "producer_sha256": producer_hash}
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
            result["syntax"][slug][scope] = _screen(
                screen, receipt_rows[f"cpp-{scope}-v0.4.1.json"]["sha256"],
                unmatched_hash, paths, measured, scope, slug, binary_hash)
            first = readings[(scope, "v0.4.0")]["subjects"]
            second = readings[(scope, "v0.4.1")]["subjects"]
            x = next(s for s in first if s["name"] == pin[1])["counts"]
            y = next(s for s in second if s["name"] == pin[1])["counts"]
            result["diagnostic_changes"][slug][scope] = {
                key: [x[key], y[key]] for key in DIAGNOSTICS if x.get(key) != y.get(key)}
    if set(grade.get("readings", {})) != set(receipt_rows):
        raise Refusal("unexpected reading receipt")
    result["audit_inputs_sha256"] = {str(path.relative_to(root)): digest for path, (digest, _) in sorted(snapshots.items())}
    if before_publish:
        before_publish()
    if binary.is_symlink() or not binary.is_file() or binary.stat().st_size > SCREEN_BINARY_CAP or \
            hashlib.sha256(binary.read_bytes()).hexdigest() != binary_hash or \
            producer.is_symlink() or hashlib.sha256(producer.read_bytes()).hexdigest() != producer_hash:
        raise Refusal("binary or producer changed before publish")
    for path, (digest, cap) in snapshots.items():
        if path.is_symlink() or not path.is_file() or path.stat().st_size > cap or \
                hashlib.sha256(path.read_bytes()).hexdigest() != digest:
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
    args = parser.parse_args()
    try:
        produce(args.audit_root, args.out, screen_binary=args.screen_binary)
    except Refusal as exc:
        parser.error(str(exc))
