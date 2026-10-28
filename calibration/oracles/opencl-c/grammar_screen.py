#!/usr/bin/env python3
"""Measure native grammars on the frozen OpenCL C pool and hashcat diagnostic."""

import argparse
import hashlib
import importlib.metadata
import importlib.util
import json
from pathlib import Path
import sys


sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
REPO_ROOT = HERE.parents[2]
POOL_RULE_SHA256 = "2e6ebd4c74fb0d579c72b235d0c20ba12d290fd4ae32330235cd694fed83e108"
SUMMARY_SCHEMA = "opencl-c-native-grammar-screen-v1"
HASHCAT_GRAMMARS = {"opencl": "tree_sitter_opencl", "c": "tree_sitter_c"}


def _load_sibling(name, filename):
    spec = importlib.util.spec_from_file_location(name, HERE / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


triage = _load_sibling("opencl_pool_screen_triage", "triage.py")
pool_audit = _load_sibling("opencl_pool_screen_audit", "pool_audit.py")


def _sha256(data):
    return hashlib.sha256(data).hexdigest()


def _canonical_digest(value):
    return _sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode("utf-8"))


def _verify_source_records(policy_document, root):
    rows = policy_document.get("source_records")
    if not isinstance(rows, list) or not rows:
        raise ValueError("pool policy source records are missing")
    verified = []
    for row in rows:
        if not isinstance(row, dict) or set(row) != {"path", "sha256"}:
            raise ValueError("pool policy source record is invalid")
        relative = Path(row["path"])
        expected = row["sha256"]
        if (relative.is_absolute() or ".." in relative.parts
                or not isinstance(expected, str) or len(expected) != 64):
            raise ValueError("pool policy source record is invalid")
        path = (root / relative).resolve()
        if root.resolve() not in path.parents:
            raise ValueError("pool policy source record escapes the repository")
        try:
            actual = _sha256(path.read_bytes())
        except OSError as exc:
            raise ValueError("pool policy source record is unreadable") from exc
        if actual != expected:
            raise ValueError("pool policy source-record hash does not match")
        verified.append({"path": row["path"], "sha256": expected})
    return verified, _canonical_digest(verified)


def _load_fixed_inputs():
    policy_path = HERE / "pool-rule.json"
    raw = policy_path.read_bytes()
    if _sha256(raw) != POOL_RULE_SHA256:
        raise ValueError("pool-rule bytes do not match the frozen digest")
    try:
        document = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ValueError("pool-rule is not readable JSON") from exc
    source_records, source_records_sha256 = _verify_source_records(document, REPO_ROOT)
    policy = pool_audit.load_policy(policy_path)
    if policy["sha256"] != POOL_RULE_SHA256:
        raise ValueError("pool audit loaded a different policy")

    pins_relative = "calibration/oracles/opencl-c/pins.json"
    pins_path = REPO_ROOT / pins_relative
    pins_raw = pins_path.read_bytes()
    pins_expected = next(row["sha256"] for row in source_records
                         if row["path"] == pins_relative)
    if _sha256(pins_raw) != pins_expected:
        raise ValueError("pins source-record hash changed while loading")
    pins = json.loads(pins_raw)
    hashcat_rows = [row for row in pins.get("subjects", []) if row.get("id") == "hashcat"]
    grammar_rows = {row.get("id"): row for row in pins.get("grammars", [])
                    if row.get("id") in HASHCAT_GRAMMARS}
    if len(hashcat_rows) != 1 or set(grammar_rows) != set(HASHCAT_GRAMMARS):
        raise ValueError("verified pins do not define the required hashcat diagnostic")
    pristine = {}
    for grammar_id, symbol in HASHCAT_GRAMMARS.items():
        row = grammar_rows[grammar_id]
        digest = row.get("native_library_sha256")
        if not isinstance(digest, str) or len(digest) != 64:
            raise ValueError("verified pins do not define pristine native grammar bytes")
        pristine[grammar_id] = {
            "symbol": symbol,
            "sha256": digest,
            "candidate": f"{grammar_id}-stock",
            "commit": row["commit"],
            "archive_sha256": row["archive_sha256"],
            "license_sha256": row["license_sha256"],
            "parser_c_sha256": row["parser_c_sha256"],
        }
    return {
        "policy": policy,
        "hashcat_pin": hashcat_rows[0]["commit"],
        "pristine": pristine,
        "source_records": source_records,
        "source_records_sha256": source_records_sha256,
    }


def _validated_out(value, parser, subject_paths):
    out = value.resolve()
    workspace = (REPO_ROOT / "workspace").resolve()
    if out == workspace or workspace not in out.parents:
        parser.error("--out must be a fresh directory below this checkout's workspace/")
    if out.exists() or out.is_symlink():
        parser.error("--out must not already exist")
    for path in subject_paths:
        root = path.resolve()
        if out == root or root in out.parents:
            parser.error("--out must be outside every subject checkout")
    return out


def _validate_specs(parser, policy, subjects, grammars, hashcat_grammars,
                    build_receipts, baseline):
    subject_ids = [item[0] for item in subjects]
    if len(subject_ids) != len(set(subject_ids)):
        parser.error("subject IDs must be unique")
    if set(subject_ids) != set(policy["primary"]):
        parser.error("--subject must contain every and only the frozen primary IDs")

    grammar_ids = [item[0] for item in grammars]
    if len(grammar_ids) != len(set(grammar_ids)):
        parser.error("grammar IDs must be unique")
    if baseline is not None and baseline not in grammar_ids:
        parser.error("--baseline must name a supplied primary grammar")
    if len(grammar_ids) > 1 and baseline is None:
        parser.error("--baseline is required when more than one primary grammar is supplied")
    diagnostic_ids = [item[0] for item in hashcat_grammars]
    if len(diagnostic_ids) != len(set(diagnostic_ids)) or set(diagnostic_ids) != set(HASHCAT_GRAMMARS):
        parser.error("--hashcat-grammar must contain exactly one opencl and one c grammar")
    for grammar_id, symbol, _ in hashcat_grammars:
        if symbol != HASHCAT_GRAMMARS[grammar_id]:
            parser.error(f"hashcat {grammar_id} grammar must use {HASHCAT_GRAMMARS[grammar_id]}")
    receipt_ids = [item[0] for item in build_receipts]
    if len(receipt_ids) != len(set(receipt_ids)) or not set(receipt_ids).issubset(HASHCAT_GRAMMARS):
        parser.error("hashcat build-receipt IDs must be unique opencl or c IDs")


def _parse_id_path(value):
    item_id, separator, raw_path = value.partition("=")
    if not separator or not triage.GRAMMAR_ID_RE.fullmatch(item_id) or not raw_path:
        raise argparse.ArgumentTypeError("expected id=path")
    return item_id, Path(raw_path)


def _capture_library_specs(specs):
    captured = []
    for grammar_id, symbol, path in specs:
        resolved = path.resolve()
        captured.append((grammar_id, symbol, resolved,
                         triage._read_regular_no_follow(resolved)))
    return captured


def _validate_build_receipt(grammar_id, path, native_bytes, pin):
    try:
        raw = triage._read_regular_no_follow(path.resolve())
        receipt = json.loads(raw)
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError("hashcat build receipt is not readable JSON") from exc
    if not isinstance(receipt, dict):
        raise ValueError("hashcat build receipt schema is invalid")
    candidate = receipt.get("candidate")
    hashes = receipt.get("hashes")
    if receipt.get("schema") != "opencl-native-grammar-build-v1" \
            or not isinstance(candidate, dict) or not isinstance(hashes, dict):
        raise ValueError("hashcat build receipt schema is invalid")
    if (candidate.get("id") != pin["candidate"]
            or candidate.get("entry_point") != pin["symbol"]
            or candidate.get("generate") is not False
            or candidate.get("scanner") is not False
            or receipt.get("patches") != []
            or hashes.get("patches_sha256") != _canonical_digest([])):
        raise ValueError("hashcat receipt must describe the ungenerated, unpatched stock build")
    expected_hashes = {
        "archive_sha256": pin["archive_sha256"],
        "license_sha256": pin["license_sha256"],
        "original_parser_c_sha256": pin["parser_c_sha256"],
    }
    if (candidate.get("commit") != pin["commit"]
            or hashes.get("parser_c_sha256") != pin["parser_c_sha256"]
            or any(hashes.get(name) != value for name, value in expected_hashes.items())):
        raise ValueError("hashcat build receipt does not match the frozen pins")
    native_sha256 = _sha256(native_bytes)
    if hashes.get("native_sha256") != native_sha256:
        raise ValueError("hashcat build receipt does not match the supplied native bytes")
    return {
        "id": grammar_id,
        "mode": "builder-receipt",
        "candidate": pin["candidate"],
        "native_sha256": native_sha256,
        "receipt_sha256": _sha256(raw),
        "receipt_path": str(path.resolve()),
        "receipt_bytes": raw,
    }


def _validate_hashcat_inputs(captured, receipt_specs, pristine):
    receipt_paths = dict(receipt_specs)
    evidence = []
    for grammar_id, symbol, _path, native_bytes in captured:
        pin = pristine[grammar_id]
        if symbol != pin["symbol"]:
            raise ValueError(f"hashcat {grammar_id} grammar symbol does not match its pin")
        if grammar_id in receipt_paths:
            row = _validate_build_receipt(
                grammar_id, receipt_paths[grammar_id], native_bytes, pin)
        else:
            native_sha256 = _sha256(native_bytes)
            if native_sha256 != pin["sha256"]:
                raise ValueError(
                    f"hashcat {grammar_id} grammar is not the pristine pinned library")
            row = {"id": grammar_id, "mode": "legacy-native-pin",
                   "native_sha256": native_sha256}
        evidence.append(row)
    return sorted(evidence, key=lambda row: row["id"])


def _stage_and_load(stage, captured, group):
    stage.mkdir(parents=True, exist_ok=True)
    loaded = []
    for grammar_id, symbol, source, library_bytes in captured:
        suffix = source.suffix if source.suffix else ".native"
        path = stage / f"{group}-{grammar_id}{suffix}"
        path.write_bytes(library_bytes)
        grammar = triage.load_grammar(grammar_id, symbol, path)
        if grammar.sha256 != _sha256(library_bytes):
            raise ValueError("staged grammar bytes changed while loading")
        loaded.append(grammar)
    return loaded


def _row_state(row):
    if not row.get("parsed") or row.get("fault") is not None:
        return "fault"
    if row.get("error") or row.get("missing"):
        return "error"
    return "clean"


def _transition_summary(baseline_id, candidate_id, baseline_rows, candidate_rows):
    before = {row["path"]: row for row in baseline_rows}
    after = {row["path"]: row for row in candidate_rows}
    if set(before) != set(after) or len(before) != len(baseline_rows) \
            or len(after) != len(candidate_rows):
        raise ValueError("grammar transition populations do not match")
    counts = {
        "clean_to_error_count": 0,
        "error_to_clean_count": 0,
        "clean_to_fault_count": 0,
        "error_to_fault_count": 0,
        "fault_to_clean_count": 0,
        "fault_to_error_count": 0,
        "unchanged_count": 0,
    }
    private_rows = []
    for path in sorted(before):
        old = _row_state(before[path])
        new = _row_state(after[path])
        if old == new:
            counts["unchanged_count"] += 1
        else:
            counts[f"{old}_to_{new}_count"] += 1
        private_rows.append({"path": path, "baseline": old, "candidate": new})
    baseline_faults = sum(_row_state(row) == "fault" for row in baseline_rows)
    candidate_faults = sum(_row_state(row) == "fault" for row in candidate_rows)
    public = {
        "id": candidate_id,
        "baseline": baseline_id,
        "population_count": len(baseline_rows),
        "fault_count_delta": candidate_faults - baseline_faults,
        **counts,
    }
    return public, {"id": candidate_id, "baseline": baseline_id, "rows": private_rows}


def _measure_subject(subject_id, root, pin, loaded, counts, baseline=None):
    before = triage.capture_checkout(root, pin)
    if (before.public["eligible"]["files"] != counts["files"]
            or before.public["eligible"]["bytes"] != counts["bytes"]):
        raise ValueError("triage population does not match the audited pool")
    summaries = []
    parse_rows = {}
    for grammar in loaded:
        summary, rows = triage.parse_population(
            before.eligible, grammar.parser, root=before.root,
            captured_sources=before.eligible_sources)
        summaries.append({
            "id": grammar.grammar_id,
            "library_sha256": grammar.sha256,
            "abi_version": grammar.language.abi_version,
            **summary,
        })
        parse_rows[grammar.grammar_id] = rows
    after = triage.capture_checkout(root, pin)
    triage.assert_checkout_unchanged(before, after)
    transitions = []
    private_transitions = []
    if baseline is not None:
        for grammar in loaded:
            if grammar.grammar_id == baseline:
                continue
            public_transition, private_transition = _transition_summary(
                baseline, grammar.grammar_id, parse_rows[baseline],
                parse_rows[grammar.grammar_id])
            transitions.append(public_transition)
            private_transitions.append(private_transition)
    public = {
        "id": subject_id,
        "role": counts["role"],
        "files": counts["files"],
        "distinct_blobs": counts["distinct_blobs"],
        "bytes": counts["bytes"],
        "checkout_unchanged": True,
        "grammars": sorted(summaries, key=lambda row: row["id"]),
        "transitions": sorted(transitions, key=lambda row: row["id"]),
    }
    private = {
        "id": subject_id,
        "checkout": str(before.root),
        "pin": pin,
        "tracked_rows_before": before.rows,
        "tracked_rows_after": after.rows,
        "untracked_rows_before": before.untracked_rows,
        "untracked_rows_after": after.untracked_rows,
        "stage_before": before.stage_raw,
        "stage_after": after.stage_raw,
        "untracked_before": before.untracked_raw,
        "untracked_after": after.untracked_raw,
        "ignored_before": before.ignored_raw,
        "ignored_after": after.ignored_raw,
        "grammars": [
            {"id": grammar.grammar_id, "symbol": grammar.symbol,
             "library": str(grammar.path), "parse_rows": parse_rows[grammar.grammar_id]}
            for grammar in loaded
        ],
        "transitions": private_transitions,
    }
    return public, private


def _pooled_grammars(subjects):
    grammar_ids = sorted({grammar["id"] for subject in subjects for grammar in subject["grammars"]})
    additive = (
        "population_count", "parsed_count", "error_or_missing_count", "error_only_count",
        "missing_only_count", "both_count", "fault_count", "clean_count",
    )
    pooled = []
    for grammar_id in grammar_ids:
        rows = [next(item for item in subject["grammars"] if item["id"] == grammar_id)
                for subject in subjects]
        if len({row["library_sha256"] for row in rows}) != 1 or len({row["abi_version"] for row in rows}) != 1:
            raise ValueError("a grammar identity changed between subjects")
        summary = {name: sum(row[name] for row in rows) for name in additive}
        summary["clean_rate"] = (summary["clean_count"] / summary["population_count"]
                                 if summary["population_count"] else 0.0)
        pooled.append({
            "id": grammar_id,
            "library_sha256": rows[0]["library_sha256"],
            "abi_version": rows[0]["abi_version"],
            **summary,
        })
    return pooled


def _pooled_transitions(subjects):
    candidate_ids = sorted({row["id"] for subject in subjects
                            for row in subject["transitions"]})
    fields = (
        "population_count", "fault_count_delta", "clean_to_error_count",
        "error_to_clean_count", "clean_to_fault_count", "error_to_fault_count",
        "fault_to_clean_count", "fault_to_error_count", "unchanged_count",
    )
    pooled = []
    for candidate_id in candidate_ids:
        rows = [next(row for row in subject["transitions"] if row["id"] == candidate_id)
                for subject in subjects]
        if len({row["baseline"] for row in rows}) != 1:
            raise ValueError("transition baseline changed between subjects")
        pooled.append({"id": candidate_id, "baseline": rows[0]["baseline"],
                       **{field: sum(row[field] for row in rows) for field in fields}})
    return pooled


def _write_receipts(out, public, private, private_subjects):
    private_dir = out / "private"
    private_dir.mkdir(parents=True, exist_ok=True)
    serializable = {**private, "subjects": []}
    for subject in private_subjects:
        subject_dir = private_dir / "subjects" / subject["id"]
        subject_dir.mkdir(parents=True)
        for name in ("stage_before", "stage_after", "untracked_before", "untracked_after",
                     "ignored_before", "ignored_after"):
            (subject_dir / f"{name}.bin").write_bytes(subject.pop(name))
        serializable["subjects"].append(subject)
    (private_dir / "evidence.json").write_text(
        json.dumps(serializable, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    (out / "summary.json").write_text(
        json.dumps(public, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--subject", action="append", required=True, type=pool_audit.parse_subject,
                        help="repeat for every frozen primary: id=checkout")
    parser.add_argument("--grammar", action="append", required=True, type=triage.parse_grammar_spec,
                        help="repeatable candidate id=symbol=localdylib")
    parser.add_argument("--baseline", help="required baseline grammar ID when comparing grammars")
    parser.add_argument("--hashcat", required=True, type=Path)
    parser.add_argument("--hashcat-grammar", action="append", required=True,
                        type=triage.parse_grammar_spec,
                        help="exact pristine opencl and c id=symbol=localdylib")
    parser.add_argument("--hashcat-build-receipt", action="append", default=[],
                        type=_parse_id_path,
                        help="optional stock builder receipt: opencl=path or c=path")
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args(argv)

    out = _validated_out(
        args.out, parser, [path for _, path in args.subject] + [args.hashcat])
    fixed = _load_fixed_inputs()
    _validate_specs(
        parser, fixed["policy"], args.subject, args.grammar, args.hashcat_grammar,
        args.hashcat_build_receipt, args.baseline)
    subjects = sorted(args.subject, key=lambda item: item[0])
    grammars = sorted(args.grammar, key=lambda item: item[0])
    hashcat_grammars = sorted(args.hashcat_grammar, key=lambda item: item[0])
    baseline = args.baseline or grammars[0][0]

    version = importlib.metadata.version("tree-sitter")
    if version != triage.TREE_SITTER_VERSION:
        raise ValueError(f"tree-sitter {version}; expected {triage.TREE_SITTER_VERSION}")
    captured = _capture_library_specs(grammars)
    captured_hashcat = _capture_library_specs(hashcat_grammars)
    hashcat_builds = _validate_hashcat_inputs(
        captured_hashcat, args.hashcat_build_receipt, fixed["pristine"])
    audit_public, audit_private = pool_audit.audit(
        fixed["policy"], subjects,
        [("hashcat", args.hashcat, fixed["hashcat_pin"])])
    out.mkdir()
    private_dir = out / "private"
    stage = private_dir / "staged-grammars"
    for row in hashcat_builds:
        if row["mode"] == "builder-receipt":
            receipt_dir = private_dir / "build-receipts"
            receipt_dir.mkdir(parents=True, exist_ok=True)
            (receipt_dir / f"{row['id']}.json").write_bytes(row["receipt_bytes"])
    loaded = _stage_and_load(stage, captured, "primary")
    loaded_hashcat = _stage_and_load(stage, captured_hashcat, "hashcat")

    audit_primary = {row["id"]: row for row in audit_public["primary"]}
    primary = []
    private_subjects = []
    for subject_id, root in subjects:
        public_row, private_row = _measure_subject(
            subject_id, root, fixed["policy"]["primary"][subject_id], loaded,
            audit_primary[subject_id], baseline)
        primary.append(public_row)
        private_subjects.append(private_row)
    hashcat_counts = next(row for row in audit_public["diagnostics"] if row["id"] == "hashcat")
    diagnostic, diagnostic_private = _measure_subject(
        "hashcat", args.hashcat, fixed["hashcat_pin"], loaded_hashcat, hashcat_counts)
    public_hashcat_builds = [
        {key: value for key, value in row.items()
         if key not in ("receipt_path", "receipt_bytes")}
        for row in hashcat_builds
    ]
    diagnostic["builds"] = public_hashcat_builds
    private_subjects.append(diagnostic_private)

    configuration = {
        "policy_sha256": fixed["policy"]["sha256"],
        "source_records_sha256": fixed["source_records_sha256"],
        "subjects": [{"id": subject_id, "pin": fixed["policy"]["primary"][subject_id]}
                     for subject_id, _ in subjects],
        "grammars": [{"id": grammar.grammar_id, "symbol": grammar.symbol,
                      "sha256": grammar.sha256} for grammar in loaded],
        "baseline": baseline,
        "diagnostic": {"id": "hashcat", "pin": fixed["hashcat_pin"],
                       "grammars": [{"id": grammar.grammar_id, "symbol": grammar.symbol,
                                     "sha256": grammar.sha256} for grammar in loaded_hashcat],
                       "builds": public_hashcat_builds},
    }
    primary_totals = {
        **audit_public["primary_totals"],
        "grammars": _pooled_grammars(primary),
        "transitions": _pooled_transitions(primary),
    }
    all_grammar_rows = [grammar for subject in primary + [diagnostic]
                        for grammar in subject["grammars"]]
    fault_count = sum(row["fault_count"] for row in all_grammar_rows)
    public = {
        "schema": SUMMARY_SCHEMA,
        "scope": "native grammar measurement only",
        "instrument": {
            "wrapper_sha256": _sha256(Path(__file__).read_bytes()),
            "triage_sha256": _sha256((HERE / "triage.py").read_bytes()),
            "pool_audit_sha256": _sha256((HERE / "pool_audit.py").read_bytes()),
            "pool_rule_sha256": fixed["policy"]["sha256"],
            "source_records_sha256": fixed["source_records_sha256"],
            "configuration_sha256": _canonical_digest(configuration),
        },
        "tree_sitter": version,
        "baseline": baseline,
        "primary": primary,
        "primary_totals": primary_totals,
        "diagnostic": diagnostic,
        "triage_facts": {
            "checkout_unchanged": True,
            "primary_subjects": len(primary),
            "primary_measurements": len(primary) * len(loaded),
            "diagnostic_measurements": len(loaded_hashcat),
            "fault_count": fault_count,
        },
    }
    private = {
        "configuration": configuration,
        "source_records": fixed["source_records"],
        "pool_audit": audit_private,
        "hashcat_builds": [
            {key: value for key, value in row.items() if key != "receipt_bytes"}
            for row in hashcat_builds],
    }
    _write_receipts(out, public, private, private_subjects)
    print(json.dumps(public, sort_keys=True))
    return 1 if fault_count else 0


if __name__ == "__main__":
    sys.exit(main())
