#!/usr/bin/env python3
"""Publish count/hash-only Terraform calibration evidence from private raw runs."""

import argparse
import hashlib
import json
import pathlib

from manifests import recipe_body


PINS_PATH = pathlib.Path(__file__).with_name("pins.json")
PINS = json.loads(PINS_PATH.read_text())


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=pathlib.Path, required=True)
    parser.add_argument("--landing-report", type=pathlib.Path,
                        help="clean landing-engine grade report; retains the original baseline")
    parser.add_argument("--out", type=pathlib.Path, required=True)
    args = parser.parse_args()
    cache = args.cache.resolve()
    native = cache / "native"
    comparison = json.loads((native / "grades" / "comparison.json").read_text())
    grades = comparison.copy()
    if args.landing_report:
        landing = json.loads(args.landing_report.read_text())
        if landing["date"] != comparison["date"]:
            raise RuntimeError("landing/baseline grade dates differ")
        if landing["native_raw_sha256"] != comparison["native_raw_sha256"]:
            raise RuntimeError("landing/baseline native truth hashes differ")
        if landing["repaired"]["commit"] != landing["repaired_full_commit"]:
            raise RuntimeError("landing grade commit differs from full engine commit")
        grades["repaired"] = landing["repaired"]
        grades["repaired_full_commit"] = landing["repaired_full_commit"]
    subjects = []
    for pin in PINS["subjects"]:
        repo = pin["repository"]
        slug = repo.rsplit("-", 1)[-1]
        receipts = [json.loads((native / f"{slug}-{phase}-receipt.json").read_text())
                    for phase in ("before", "after")]
        before, after = receipts
        identity = ("subject_sha", "tracked_manifest_sha256", "tracked_entries", "tracked_bytes",
                    "measured_file_count", "measured_file_list_sha256", "pinned_module_instances",
                    "native_local_agreements", "native_registry_alias_agreements",
                    "pinned_module_manifest_sha256",
                    "oracle_raw_sha256", "xref_truth",
                    "xref_truth_floor", "definitions", "references")
        if any(before[key] != after[key] for key in identity):
            raise RuntimeError("native before/after evidence differs: " + repo)
        if before["subject_sha"] != pin["sha"]:
            raise RuntimeError("native evidence differs from pin: " + repo)
        for phase in ("before", "after"):
            if digest(native / f"{slug}-{phase}.json") != before["oracle_raw_sha256"]:
                raise RuntimeError("raw native JSON differs from receipt: " + repo)
        if grades["native_raw_sha256"][repo] != before["oracle_raw_sha256"]:
            raise RuntimeError("grade/native raw truth differs: " + repo)
        subject = {key: before[key] for key in identity}
        subject["repository"] = repo
        subject["raw_truth_unchanged"] = True
        for label in ("baseline", "repaired"):
            grade_subject = grades[label]["subjects"][repo]
            subject[label] = {"files": grade_subject["files"], "bytes": grade_subject["bytes"],
                              "xref_found": grade_subject["counts"]["xref_truth_found"],
                              "xref_truth": grade_subject["counts"]["xref_truth"]}
            if grade_subject["counts"]["xref_truth"] != before["xref_truth"]:
                raise RuntimeError("grade/native denominator differs: " + repo)
        subjects.append(subject)
    result = {"date": "2026-10-26", "tier": "A", "native_tool": "OpenTofu 1.13.0",
              "pin_manifest_sha256": digest(PINS_PATH),
              "tracked_set_recipe_sha256": hashlib.sha256(recipe_body().encode()).hexdigest(),
              "oracle_binary_sha256": digest(cache / "tools" / "terraform-oracle"),
              "subjects": subjects,
              "grades": {label: {key: grades[label][key] for key in
                                 ("commit", "binary_sha256", "selected_run", "selected_reading_sha256",
                                  "run_sha256", "metrics", "grade", "counts")}
                         for label in ("baseline", "repaired")},
              "baseline_full_commit": grades["baseline_full_commit"],
              "repaired_full_commit": grades["repaired_full_commit"]}
    if args.landing_report:
        result["grades"]["repaired"]["run_conditions"] = grades["repaired"]["run_conditions"]
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"published": str(args.out), "subjects": len(subjects),
                      "xref_truth": sum(x["xref_truth"] for x in subjects)}, sort_keys=True))


if __name__ == "__main__":
    main()
