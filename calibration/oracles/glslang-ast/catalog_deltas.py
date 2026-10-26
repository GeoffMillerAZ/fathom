"""Report paired G2 precision changes and overlapping registered families.

Usage: python3 -I -B calibration/oracles/glslang-ast/catalog_deltas.py comparison.json out.json
The comparison names before/after reading files, a reviewed catalog/pin, and
pairs of subject name + before/after diagnostic receipt files. Detailed paths
stay in workspace; the output contains counts, identities and hashes only.
"""
import argparse
from collections import Counter
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import sys


def _subjects(reading):
    rows = reading["subjects"]
    result = {r["name"]: (r["sha"], r["files"]) for r in rows}
    if len(result) != len(rows):
        raise ValueError("duplicate reading subject")
    return result


def _pairs(rows, paths):
    values = {(r["path"], r["caller"], r["callee"]) for r in rows}
    if len(values) != len(rows) or any(p not in paths for p, _, _ in values):
        raise ValueError("invalid or duplicate diagnostic call pair")
    return values


def _counts(reading):
    result = {key: reading["counts"][key] for key in
              ("call_truth", "call_truth_found", "call_fathom_in_truth", "call_fathom")}
    if any(type(value) is not int or value < 0 for value in result.values()):
        raise ValueError("invalid reading call counts")
    return result


def _score(counts):
    return {**counts, "precision": counts["call_fathom_in_truth"] / counts["call_fathom"]
            if counts["call_fathom"] else None,
            "recall": counts["call_truth_found"] / counts["call_truth"] if counts["call_truth"] else None}


def compare(before, after, receipts, core_types, baseline_types, function_map, type_map):
    source = before["fathom_commit"]
    if source != after["fathom_commit"] or not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("paired readings must use the same pinned product commit")
    subjects = _subjects(before)
    if subjects != _subjects(after) or {r[0] for r in receipts} != set(subjects) or len(receipts) != len(subjects):
        raise ValueError("paired subject identity differs")
    a, b = _counts(before), _counts(after)
    for key in ("call_truth", "call_truth_found", "call_fathom_in_truth"):
        if a[key] != b[key]:
            raise ValueError("paired truth or matched-call counts changed: " + key)
    functions = set().union(*(set(v) for v in function_map.values())) if function_map else set()
    types = set().union(*(set(v) for v in type_map.values())) if type_map else set()
    removed_by_kind, families = Counter(), Counter()
    before_extra = after_extra = removed_count = 0
    for subject, old, new in receipts:
        paths = old["tenancy_files"]
        if (paths != sorted(set(paths)) or new["tenancy_files"] != paths or
                len(paths) != subjects[subject][1]):
            raise ValueError("exact paired tenancy differs: " + subject)
        path_set = set(paths)
        old_extra = _pairs(old["calls"]["fathom_only"], path_set)
        new_extra = _pairs(new["calls"]["fathom_only"], path_set)
        if new_extra - old_extra:
            raise ValueError("broader exclusions introduced extra calls")
        if _pairs(old["calls"]["oracle_only"], path_set) != _pairs(new["calls"]["oracle_only"], path_set):
            raise ValueError("paired truth misses changed")
        before_extra += len(old_extra)
        after_extra += len(new_extra)
        removed = old_extra - new_extra
        removed_count += len(removed)
        for _, _, name in removed:
            if name in core_types - baseline_types:
                removed_by_kind["core_type"] += 1
            elif name in functions and name in types:
                removed_by_kind["extension_function_or_type"] += 1
                families.update({extension for extension, names in function_map.items() if name in names} |
                                {extension for extension, names in type_map.items() if name in names})
            elif name in functions:
                removed_by_kind["extension_function"] += 1
                families.update(extension for extension, names in function_map.items() if name in names)
            elif name in types:
                removed_by_kind["extension_type"] += 1
                families.update(extension for extension, names in type_map.items() if name in names)
            else:
                raise ValueError("unexplained removed pair outside catalog additions")
    if (before_extra + a["call_fathom_in_truth"] != a["call_fathom"] or
            after_extra + b["call_fathom_in_truth"] != b["call_fathom"] or
            removed_count != a["call_fathom"] - b["call_fathom"]):
        raise ValueError("diagnostic pairs do not reconcile with reading counts")
    return {"product_commit": source, "before": _score(a), "after": _score(b),
            "removed_pairs": removed_count, "removed_by_kind": dict(sorted(removed_by_kind.items())),
            "extension_families": dict(sorted(families.items())), "family_counts_overlap": True,
            "disclosure": "A name declared by multiple extensions counts in every declaring specification family; family counts are not additive. Shared function/type spellings retain kind ambiguity."}


def _read(path):
    return json.loads(Path(path).read_text())


def _hash(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def _bound_file(path, expected, label):
    if _hash(path) != expected:
        raise ValueError(label + " hash differs from measurement evidence")


def validate_pair_evidence(config, pin, catalog_hash, fingerprints):
    """Bind comparisons to the measurement drivers' frozen input receipts."""
    evidence = config.get("evidence", {})
    before_mode = config.get("before_mode", "baseline-core")
    after_mode = config.get("after_mode", "full-registry")
    rank = {"baseline-core": 0, "core-plus-types": 1, "full-registry": 2}
    if (before_mode not in fingerprints or after_mode not in fingerprints or
            rank.get(before_mode, 3) >= rank.get(after_mode, -1)):
        raise ValueError("invalid paired catalog modes")
    if evidence.get("kind") == "holdout":
        summary = _read(evidence["summary"])
        if (summary.get("paired_comparison_claimable") is not True or
                summary.get("holdout_commit") != pin["commit_sha"] or
                summary.get("archive_sha256") != pin["archive_sha256"] or
                summary.get("extension_catalog_sha256") != catalog_hash):
            raise ValueError("holdout summary differs from accepted pins")
        for field in ("baseline_catalog_sha256", "core_catalog_sha256", "baseline_constructors_sha256"):
            if summary.get(field) != pin[field]:
                raise ValueError("holdout source catalog hash differs from accepted pin: " + field)
        rows = _read(evidence["oracle_receipt"])["files"]
        files = [{key: row[key] for key in ("path", "extension", "sha256", "bytes")} for row in rows]
        manifest = hashlib.sha256(json.dumps(files, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
        if manifest != summary["manifest_sha256"] or len({row["path"] for row in files}) != len(files):
            raise ValueError("holdout source manifest differs from measurement evidence")
        if len(config["pairs"]) != 1:
            raise ValueError("holdout requires one paired subject")
        pair = config["pairs"][0]
        truth_hashes = set()
        for side, mode in (("before", before_mode), ("after", after_mode)):
            measured = summary["modes"][mode]
            score = measured["score"]
            if (measured["catalog_mode"] != mode or
                    measured["catalog_fingerprint_sha256"] != fingerprints[mode] or
                    score["subject_sha"] != pin["commit_sha"] or score["manifest_sha256"] != manifest):
                raise ValueError("holdout catalog mode or source identity differs")
            _bound_file(config[side + "_reading"], score["reading_sha256"], "reading")
            _bound_file(pair[side + "_unmatched"], score["unmatched_sha256"], "unmatched receipt")
            data = _read(config[side + "_reading"])
            if (data["fathom_commit"] != summary["product_commit"] or
                    _subjects(data) != {pair["subject"]: (pin["commit_sha"], len(files))} or
                    _read(pair[side + "_unmatched"])["tenancy_files"] != sorted(row["path"] for row in files)):
                raise ValueError("holdout reading differs from frozen subject population")
            truth_hashes.add(score["oracle_truth_sha256"])
        if len(truth_hashes) != 1:
            raise ValueError("paired oracle truth differs")
        return {"kind": "holdout", "manifest_sha256": manifest,
                "validation_claimable": summary.get("validation_claimable", False),
                "diagnostic_only": summary.get("diagnostic_only", True),
                "summary_sha256": _hash(evidence["summary"])}
    if evidence.get("kind") == "canonical":
        identities, truths = [], []
        for side, mode in (("before", before_mode), ("after", after_mode)):
            measured = _read(evidence[side])
            if (measured["catalog_mode"] != mode or
                    measured["catalog_fingerprint_sha256"] != fingerprints[mode] or
                    measured["extension_catalog_sha256"] != (catalog_hash if mode == "full-registry" else None)):
                raise ValueError("canonical catalog mode differs from accepted catalog")
            _bound_file(config[side + "_reading"], measured["reading_sha256"], "reading")
            data = _read(config[side + "_reading"])
            if data["fathom_commit"] != measured["product_commit"]:
                raise ValueError("canonical product identity differs")
            subjects = {row["name"]: row for row in measured["subjects"]}
            if set(subjects) != {row["subject"] for row in config["pairs"]}:
                raise ValueError("canonical evidence subject set differs")
            identity, truth = {}, {}
            for pair in config["pairs"]:
                name = pair["subject"]
                row = subjects[name]
                _bound_file(pair[side + "_unmatched"], row["unmatched_sha256"], "unmatched receipt")
                _bound_file(row["oracle_path"], row["oracle_sha256"], "oracle")
                oracle = _read(row["oracle_path"])
                normalized = {"calls": sorted(oracle["calls"], key=lambda call: (call["path"], call["from"], call["to"])),
                              "scope": sorted(oracle["scope"])}
                oracle_hash = hashlib.sha256(json.dumps(normalized, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
                if oracle_hash != row["oracle_truth_sha256"]:
                    raise ValueError("canonical oracle truth hash differs")
                if _subjects(data).get(name) != (row["sha"], row["files"]):
                    raise ValueError("canonical subject identity differs")
                identity[name] = (row["sha"], row["files"], row["shader_tree_sha256"])
                truth[name] = oracle_hash
            identities.append(identity)
            truths.append(truth)
        if identities[0] != identities[1] or truths[0] != truths[1]:
            raise ValueError("canonical paired source population or exact oracle truth differs")
        return {"kind": "canonical", "before_evidence_sha256": _hash(evidence["before"]),
                "after_evidence_sha256": _hash(evidence["after"])}
    raise ValueError("paired comparison requires driver evidence")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("comparison", type=Path)
    parser.add_argument("out", type=Path)
    args = parser.parse_args()
    config = _read(args.comparison)
    sys.dont_write_bytecode = True
    spec = importlib.util.spec_from_file_location("glsl_holdout", Path(__file__).with_name("holdout.py"))
    holdout = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(holdout)
    pin = _read(config.get("pin", Path(__file__).with_name("holdout-pin.json")))
    catalog_path = Path(config["catalog"])
    _, _, catalog_hash = holdout.load_registry(catalog_path, pin)
    catalog = _read(catalog_path)
    function_map = catalog["functions_by_extension"]
    type_map = catalog["types_by_extension"]
    core_path = Path(__file__).with_name("glsl460-functions.json")
    core = _read(core_path)
    if hashlib.sha256(core_path.read_bytes()).hexdigest() != pin["core_catalog_sha256"]:
        raise ValueError("core catalog pin mismatch")
    if holdout.legacy_constructor_hash() != pin["baseline_constructors_sha256"]:
        raise ValueError("baseline constructor pin mismatch")
    fingerprints = {}
    for mode in ("baseline-core", "core-plus-types", "full-registry"):
        functions = set(core["functions"]) | (set(catalog["functions"]) if mode == "full-registry" else set())
        types = holdout.legacy_constructor_names() if mode == "baseline-core" else set(core["types"])
        if mode == "full-registry":
            types |= set(catalog["types"])
        fingerprints[mode] = holdout.ast._catalog_fingerprint(mode, functions, types)
    evidence = validate_pair_evidence(config, pin, catalog_hash, fingerprints)
    receipts = [(row["subject"], _read(row["before_unmatched"]), _read(row["after_unmatched"]))
                for row in config["pairs"]]
    result = compare(_read(config["before_reading"]), _read(config["after_reading"]), receipts,
                     set(core["types"]), holdout.legacy_constructor_names(), function_map, type_map)
    result["registry_catalog_sha256"] = catalog_hash
    result["measurement_evidence"] = evidence
    args.out.write_text(json.dumps(result, indent=2) + "\n")


if __name__ == "__main__":
    main()
