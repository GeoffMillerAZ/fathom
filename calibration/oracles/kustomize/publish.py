#!/usr/bin/env python3
"""Publish counts and hashes only after before/after and grading checks pass."""

import argparse
import json
from pathlib import Path

from run_native import PINS_PATH, digest, pins, slug


def load(path):
    return json.loads(path.read_text())


def numeric_map(value):
    if not isinstance(value, dict) or any(not isinstance(v, int) or isinstance(v, bool) for v in value.values()):
        raise RuntimeError("expected numeric count map")
    return value


def d19_xref_disposition(counts, claimed_denominators):
    """Report each metric's observed denominator without selecting on outcome."""
    actual = {"precision": counts.get("xref_fathom", 0),
              "recall": counts.get("xref_truth", 0)}
    if actual != claimed_denominators:
        raise RuntimeError("D19 cross-reference denominators differ from measured counts")
    if any(not isinstance(value, int) or isinstance(value, bool) or value < 0
           for value in actual.values()):
        raise RuntimeError("invalid D19 cross-reference denominator")
    return {"denominators": actual, "thin": any(value < 20 for value in actual.values())}


def population_disposition(scored, d19_thin):
    """Keep conservation floors as promotion criteria, never publication gates."""
    original_floors = {"kubernetes-sigs/kustomize": 12,
                       "argoproj/argocd-example-apps": 1}
    originals = scored.get("original_xref_truth", {})
    if set(originals) != set(original_floors):
        raise RuntimeError("original subject conservation counts missing")
    expected = {"original_xref_floor_met": all(originals[name] >= floor
                for name, floor in original_floors.items()),
                "pool_xref_floor_met": scored["counts"]["xref_truth"] >= 13}
    for key in ("original_xref_floor_met", "pool_xref_floor_met"):
        if type(scored.get(key)) is not bool:
            raise RuntimeError("missing or invalid conservation result: " + key)
        if scored[key] != expected[key]:
            raise RuntimeError("conservation result differs from measured counts: " + key)
    failed = not (scored["original_xref_floor_met"] and scored["pool_xref_floor_met"])
    return {"original_xref_floor_met": scored["original_xref_floor_met"],
            "pool_xref_floor_met": scored["pool_xref_floor_met"],
            "original_population_conservation_failed": failed,
            "lane_disposition": "held" if failed or d19_thin else "count_guard_met_pending_D19"}


def checked_leave_one_out(scored, subject_pins):
    names = [pin["repository"] for pin in subject_pins]
    rows = scored["leave_one_subject_out"]
    if len(rows) != len(names) or [row.get("omitted_repository") for row in rows] != names:
        raise RuntimeError("leave-one-out rows differ from pinned subject order")
    subjects = {row["repository"]: row["counts"] for row in scored["subjects"]}
    if set(subjects) != set(names):
        raise RuntimeError("leave-one-out subject counts differ from pins")
    public = []
    for row in rows:
        omitted = row["omitted_repository"]
        expected = {key: value - subjects[omitted].get(key, 0)
                    for key, value in scored["counts"].items()}
        if row["counts"] != expected:
            raise RuntimeError("leave-one-out counts differ from measured subjects: " + omitted)
        if not isinstance(row["metrics"], dict) or not isinstance(row["grade"], dict) or not isinstance(row["level_met"], str):
            raise RuntimeError("leave-one-out harness result incomplete: " + omitted)
        public.append({key: row[key] for key in
                       ("omitted_repository", "counts", "metrics", "grade", "level_met")})
    return public


def self_test():
    """Small synthetic fixtures for publication disclosure; no subject data."""
    for p, r in ((19, 20), (20, 19), (0, 0)):
        assert d19_xref_disposition({"xref_fathom": p, "xref_truth": r},
                                    {"precision": p, "recall": r})["thin"]
    assert not d19_xref_disposition({"xref_fathom": 20, "xref_truth": 21},
                                    {"precision": 20, "recall": 21})["thin"]
    try:
        d19_xref_disposition({"xref_fathom": 20, "xref_truth": 21},
                             {"precision": 21, "recall": 21})
    except RuntimeError as error:
        assert "differ" in str(error)
    else:
        raise AssertionError("falsified denominator fixture passed")


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--cache", type=Path)
    ap.add_argument("--out", type=Path,
                    help="public JSON destination; this file contains counts and hashes only")
    ap.add_argument("--self-test", action="store_true", help="run synthetic D19 gate fixtures")
    args = ap.parse_args()
    if args.self_test:
        self_test()
        print("D19 publication disclosure fixtures passed")
        return
    if args.cache is None or args.out is None:
        ap.error("--cache and --out are required for publication")
    cache = args.cache.resolve()
    native = cache / "kustomize-r1" / "native"
    manifests = cache / "kustomize-r1" / "manifests"
    grade = cache / "kustomize-r1" / "grade"
    before = load(native / "before-summary.json")
    after = load(native / "after-summary.json")
    scored = load(grade / "summary.json")
    fixed = pins()
    median_path = grade / "median.json"
    median = load(median_path)
    if scored["reading_sha256"] != digest(median_path):
        raise RuntimeError("median reading hash changed after grading")
    for key in ("counts", "metrics", "metric_tiers", "grade", "level_met"):
        if scored[key] != median[key]:
            raise RuntimeError("graded summary differs from median reading: " + key)
    if scored["base_commit"] != fixed["base_commit"]:
        raise RuntimeError("fixed product source commit changed")
    if any(r.get("pins_sha256") != digest(PINS_PATH) for r in (before, after, scored)):
        raise RuntimeError("pin manifest changed during measurement")
    for key in ("oracle_bin_sha256", "native_bin_sha256"):
        if before.get(key) != after.get(key):
            raise RuntimeError("native tool identity changed")
    if len(before["subjects"]) != len(fixed["subjects"]) or len(after["subjects"]) != len(fixed["subjects"]):
        raise RuntimeError("subject population differs")
    public_subjects = []
    for pin in fixed["subjects"]:
        name = pin["repository"]
        b = next((s for s in before["subjects"] if s["repository"] == name), None)
        a = next((s for s in after["subjects"] if s["repository"] == name), None)
        g = next((s for s in scored["subjects"] if s["repository"] == name), None)
        if not b or not a or not g or any(s["subject_sha"] != pin["sha"] for s in (b, a)):
            raise RuntimeError("subject identity missing or changed: " + name)
        for suffix in (".json", ".files", ".receipt"):
            pre = native / (slug(pin) + "-before" + suffix)
            post = native / (slug(pin) + "-after" + suffix)
            if pre.read_bytes() != post.read_bytes():
                raise RuntimeError("native truth changed: " + name)
        raw_truth = native / (slug(pin) + "-before.json")
        raw_files = native / (slug(pin) + "-before.files")
        raw_receipt = native / (slug(pin) + "-before.receipt")
        truth = load(raw_truth)
        if (b["oracle_sha256"] != digest(raw_truth) or b["files_sha256"] != digest(raw_files)
                or b["receipt_sha256"] != digest(raw_receipt)):
            raise RuntimeError("native summary hashes differ from raw evidence: " + name)
        if b["native_receipt"] != load(raw_receipt):
            raise RuntimeError("native receipt summary differs from raw evidence: " + name)
        if any(len(truth[key]) != b[label] for key, label in (("definitions", "definitions"),
                                                               ("references", "references"),
                                                               ("cross_references", "cross_references"))):
            raise RuntimeError("native summary counts differ from raw evidence: " + name)
        mpre = load(manifests / (slug(pin) + "-before.json"))
        mpost = load(manifests / (slug(pin) + "-after.json"))
        if mpre != mpost:
            raise RuntimeError("tracked subject content changed: " + name)
        rec = b["native_receipt"]
        if rec["binary_sha256"] != before["native_bin_sha256"]:
            raise RuntimeError("native receipt binary differs from measured tool: " + name)
        contexts = rec["component_contexts"]
        context_counts = {key: sum(item[key] for item in contexts.values())
                          for key in ("valid", "tried", "excluded", "effective_entries", "noop_entries")}
        if g["files"] + g["unsupported_native_basename"] != b["built_files"]:
            raise RuntimeError("built-file conservation failed: " + name)
        if g["sha"] != pin["sha"]:
            raise RuntimeError("graded subject pin differs: " + name)
        public_subjects.append({"repository": name, "subject_sha": pin["sha"],
                                "membership": pin["membership"],
                                "tracked_manifest_sha256": mpre["sha256"],
                                "tracked_entries": mpre["entries"], "tracked_bytes": mpre["bytes"],
                                "oracle_sha256": b["oracle_sha256"],
                                "built_file_list_sha256": b["files_sha256"],
                                "native_receipt_sha256": b["receipt_sha256"],
                                "native_files": rec["files"], "native_built_roots": rec["built_roots"],
                                "native_typed_entries": rec["typed_entries"],
                                "native_excluded": numeric_map(rec["excluded"]),
                                "native_inventory": numeric_map(rec["inventory"]),
                                "native_stats": numeric_map(rec["stats"]),
                                "component_contexts": len(contexts),
                                "component_context_counts": context_counts,
                                "native_definitions": b["definitions"],
                                "native_references": b["references"],
                                "native_cross_references": b["cross_references"],
                                "native_yaml_target_cross_references": sum(
                                    Path(ref["to_path"]).name not in
                                    ("kustomization.yaml", "kustomization.yml", "Kustomization")
                                    and Path(ref["to_path"]).suffix in (".yaml", ".yml")
                                    for ref in truth["cross_references"]),
                                "graded_files": g["files"], "graded_bytes": g["bytes"],
                                "unsupported_native_basename": g["unsupported_native_basename"],
                                "graded_counts": numeric_map(g["counts"]),
                                "graded_ratios": g["ratios"]})
    d19 = d19_xref_disposition(scored["counts"], scored["D19_xref_denominators"])
    disposition = population_disposition(scored, d19["thin"])
    leave_one_subject_out = checked_leave_one_out(scored, fixed["subjects"])
    public = {"date": fixed["date"], "base_commit": fixed["base_commit"],
              "pins_sha256": digest(PINS_PATH),
              "oracle_bin_sha256": before["oracle_bin_sha256"],
              "native_bin_sha256": before["native_bin_sha256"],
              "indexbench_local_artifact_sha256": scored["indexbench_sha256"],
              "indexbench_source_identity": {"vcs_revision": fixed["base_commit"],
                                             "vcs_modified": False},
              "median_reading_sha256": scored["reading_sha256"],
              "median_run": scored["median_run"],
              "throughput_mb_per_cpu_second": scored["runs_mb_per_cpu_second"],
              "counts": numeric_map(scored["counts"]), "metrics": scored["metrics"],
              "metric_tiers": scored["metric_tiers"], "grade": scored["grade"],
              "level_met": scored["level_met"],
              "original_xref_truth": scored["original_xref_truth"],
              "D19_xref_denominators": d19["denominators"],
              "D19_xref_thin": d19["thin"],
              "niceness": scored["niceness"],
              "go_runtime_env": scored["go_runtime_env"],
              "load_at_launch_by_run": scored["load_at_launch_by_run"],
              "subjects": public_subjects,
              "leave_one_subject_out": leave_one_subject_out,
              **disposition}
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(public, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"subjects": len(public_subjects), "level_met": public["level_met"],
                      "D19_xref_thin": public["D19_xref_thin"]}, sort_keys=True))


if __name__ == "__main__":
    main()
