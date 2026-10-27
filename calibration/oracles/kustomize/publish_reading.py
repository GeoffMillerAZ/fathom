#!/usr/bin/env python3
"""Copy the verified Kustomize median reading with public, counts-only notes."""

import argparse
import copy
import hashlib
import json
from pathlib import Path

from run_native import PINS_PATH, digest, pins
from publish import checked_leave_one_out, population_disposition

ROOT = Path(__file__).resolve().parents[3]


def require(ok, message):
    if not ok:
        raise RuntimeError(message)


def read(path):
    return json.loads(path.read_text())


def check_subjects(median, public, frozen):
    expected = {pin["repository"]: pin["sha"] for pin in frozen["subjects"]}
    measured = {row["name"]: row for row in median["subjects"]}
    published = {row["repository"]: row for row in public["subjects"]}
    require(len(expected) == len(measured) == len(published)
            == len(median["subjects"]) == len(public["subjects"]) == 5,
            "subject population must be five distinct pins")
    require(set(measured) == set(published) == set(expected), "subject membership differs")
    for name, sha in expected.items():
        m, p = measured[name], published[name]
        require(m["sha"] == p["subject_sha"] == sha, "subject SHA differs: " + name)
        require(m["files"] == p["graded_files"] and m["bytes"] == p["graded_bytes"],
                "graded file/byte counts differ: " + name)
        require(m["counts"] == p["graded_counts"], "graded counts differ: " + name)
        counts = m["counts"]
        ratios = {"sym_precision": ("matched", "fathom_defs"),
                  "sym_recall": ("matched", "oracle_defs"),
                  "ref_precision": ("ref_fathom_in_truth", "ref_fathom"),
                  "ref_recall": ("ref_truth_found", "ref_truth"),
                  "xref_precision": ("xref_fathom_in_truth", "xref_fathom"),
                  "xref_recall": ("xref_truth_found", "xref_truth")}
        for metric, (num, den) in ratios.items():
            value = counts[num] / counts[den] if counts[den] else None
            require(value == p["graded_ratios"][metric],
                    "graded subject ratio differs: " + name + "/" + metric)
        require(m["tenancy"]["faults"] == 0, "faulting subject: " + name)
    return measured, published


def verify_operational_conditions(public, subject_names):
    niceness = public["niceness"]
    require(type(niceness) is int and niceness >= 10 and public["go_runtime_env"],
            "recorded execution settings missing or niceness below 10")
    launches = public["load_at_launch_by_run"]
    require(len(launches) == 3 and all(len(run) == len(subject_names) for run in launches)
            and all(set(row["repository"] for row in run) == set(subject_names)
                    and all(type(row["load_at_launch"]) in (int, float)
                            and 0 <= row["load_at_launch"] <= 9 for row in run)
                    for run in launches), "per-launch load records missing or above 9")


def verify(median_path, public_path, current_path, cache):
    frozen = pins()
    median, public, current = read(median_path), read(public_path), read(current_path)
    require(digest(PINS_PATH) == public["pins_sha256"], "pin manifest differs from public receipt")
    require(digest(median_path) == public["median_reading_sha256"],
            "unannotated median SHA256 differs from public receipt")
    require(public["base_commit"] == frozen["base_commit"] == median["fathom_commit"],
            "product source commit differs")
    require(public["indexbench_source_identity"] ==
            {"vcs_revision": frozen["base_commit"], "vcs_modified": False},
            "indexbench clean source identity differs")
    require(public["indexbench_local_artifact_sha256"] ==
            read(cache / "kustomize-r1" / "grade" / "summary.json")["indexbench_sha256"],
            "indexbench local artifact digest differs from graded summary")
    require(median["dialect"] == current["dialect"] == "kustomize", "dialect differs")
    for key in ("kind", "target_level", "required_kinds"):
        require(median[key] == current[key], "existing reading contract changed: " + key)
    for key in ("counts", "metrics", "metric_tiers", "grade", "level_met"):
        require(median[key] == public[key], "measured reading differs from public receipt: " + key)
    measured, published = check_subjects(median, public, frozen)
    require(checked_leave_one_out({"counts": public["counts"],
                                   "subjects": [{"repository": row["repository"],
                                                 "counts": row["graded_counts"]}
                                                for row in public["subjects"]],
                                   "leave_one_subject_out": public["leave_one_subject_out"]},
                                  frozen["subjects"]) == public["leave_one_subject_out"],
            "leave-one-out receipt differs from measured subjects")
    require(median["measured_at"] == public["date"], "measurement date differs")
    require(median["oracles"] == [{"tier": "A", "tool": "kustomize", "version": "v5.8.1"}],
            "median tool oracle identity differs")
    preference_log = ROOT / "workspace" / "logs" / "kustomize-r1-directory-preference.log"
    preference = preference_log.read_text()
    require("ok  \tgithub.com/geoffmilleraz/fathom/internal/fusion" in preference
            and "FAIL" not in preference, "directory-preference check is not green")
    counts = median["counts"]
    require(counts["xref_fathom"] == public["D19_xref_denominators"]["precision"]
            and counts["xref_truth"] == public["D19_xref_denominators"]["recall"],
            "D19 denominators differ from measured counts")
    require(public["D19_xref_thin"] == (min(counts["xref_fathom"], counts["xref_truth"]) < 20),
            "D19 thin disposition differs from measured counts")
    require({key: public[key] for key in ("original_xref_floor_met", "pool_xref_floor_met",
                                        "original_population_conservation_failed", "lane_disposition")}
            == population_disposition(public, public["D19_xref_thin"]),
            "population disposition differs from public conservation results")
    originals = {pin["repository"] for pin in frozen["subjects"] if pin["membership"] == "original"}
    require(len(originals) == 2, "original subject membership differs")
    for name in originals:
        require(public["original_xref_truth"][name] == measured[name]["counts"]["xref_truth"],
                "original xref count differs: " + name)
    runs = [read(cache / "kustomize-r1" / "grade" / f"run-{n}.json") for n in (1, 2, 3)]
    for run in runs:
        for key in ("counts", "metrics", "metric_tiers", "grade", "level_met", "required_kinds"):
            require(run[key] == median[key], "three-run stability failed: " + key)
        require(run["tenancy"]["faults"] == 0, "faulting grade run")
    rates = [run["tenancy"]["throughput_mb_s"] for run in runs]
    require(rates == public["throughput_mb_per_cpu_second"], "three-run CPU rates differ")
    chosen = sorted(range(3), key=lambda index: (rates[index], index))[1]
    require(chosen + 1 == public["median_run"] and runs[chosen] == median,
            "median selection or exact reading differs")
    require(median["tenancy"]["throughput_basis"] == "cpu"
            and median["tenancy"]["throughput_mb_s"] == rates[chosen],
            "median CPU rate differs")
    verify_operational_conditions(public, measured)
    native_pin = frozen["native"]
    require(native_pin["version"] == median["oracles"][0]["version"],
            "pinned native version differs")
    return median, public


def notes(public):
    native = pins()["native"]
    subjects = public["subjects"]
    total = lambda key: sum(row[key] for row in subjects)
    context = lambda key: sum(row["component_context_counts"][key] for row in subjects)
    excluded = lambda key: sum(row["native_excluded"].get(key, 0) for row in subjects)
    counts = public["counts"]
    yaml_targets = total("native_yaml_target_cross_references")
    require(0 <= yaml_targets <= total("native_cross_references"),
            "YAML-target count exceeds native population")
    d19 = public["D19_xref_denominators"]
    rates = public["throughput_mb_per_cpu_second"]
    rate = rates[public["median_run"] - 1]
    if public["original_population_conservation_failed"]:
        disposition = "original-subject or pooled cross-reference conservation failed; lane held"
    elif public["D19_xref_thin"]:
        disposition = "thin; lane held"
    else:
        disposition = "clears the frozen count guard; apply main lang-regrade before current publication"
    omitted = []
    for index, row in enumerate(public["leave_one_subject_out"], 1):
        measured = row["counts"]
        omitted.append(f"pin {index}: xref {measured['xref_fathom']}/{measured['xref_truth']}, "
                       f"symbol precision {row['metrics']['sym_precision']}, "
                       f"level_met {row['level_met']}")
    return [
        "Native provenance: official Kustomize " + native["version"] + " binary SHA256 "
        + public["native_bin_sha256"] + "; platform archive checksums are pinned in pins.json"
        + "; typed API and kyaml " + native["api_version"]
        + f"; fixed {len(subjects)}-subject selection followed private exploration, so it is not a blind holdout.",
        f"Measured files: native built-file scope via -only, {total('graded_files')} of {total('native_files')} recognized; "
        f"{total('native_files') - total('graded_files')} excluded with reasons in the assessment. "
        f"Real Component contexts: {context('valid')} valid / {context('tried')} tried, "
        f"{context('effective_entries')} effective entries; {excluded('component-no-valid-context')} without valid context.",
        f"Native truth has {total('native_cross_references')} raw cross-file references; "
        f"the unchanged dialect scores {counts['xref_truth']} after file/target scope. "
        f"Native cross-file references include {yaml_targets} YAML-target references outside "
        "dialect file scope (manifests and patches). Manifest-target edges resolve to nothing "
        "at ingest until the kubernetes dialect is dispatched. "
        f"Frozen metric counts for D19: precision denominator {d19['precision']}, recall denominator {d19['recall']}; "
        f"{disposition}.",
        "Leave-one-subject-out under current harness rules, in frozen pin order: "
        + "; ".join(omitted) + ". The assessment identifies the subjects and limits.",
        f"Fathom/native definitions {counts['fathom_defs']}/{counts['oracle_defs']}; "
        f"matched {counts['matched']}. Measured grade: "
        + ", ".join(f"{level} {value}" for level, value in public["grade"].items())
        + f"; references {counts['ref_truth_found']}/{counts['ref_truth']} and "
        f"cross-file references {counts['xref_truth_found']}/{counts['xref_truth']}.",
        "CPU throughput runs " + ", ".join(str(value) for value in rates)
        + f" MB/CPU-s; median {rate}. Settings: nice {public['niceness']}, "
        + ", ".join(f"{key}={value}" for key, value in sorted(public["go_runtime_env"].items()))
        + "; launch loads are in the public receipt. Wall throughput is not comparable across machines.",
        "Product engine unchanged at clean source commit " + public["base_commit"]
        + "; existing directory-preference test green. Public counts/hash evidence: "
        "docs/assessments/2026-10-26-kustomize-d19-evidence.md and "
        "docs/assessments/data/2026-10-26-kustomize-r1.json.",
    ]


def merge_notes(have, add):
    return list(dict.fromkeys([*have, *add]))


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--cache", type=Path, required=True)
    ap.add_argument("--public", type=Path, required=True)
    ap.add_argument("--reading", type=Path, required=True)
    ap.add_argument("--check", action="store_true", help="validate and report without writing")
    args = ap.parse_args()
    cache = args.cache.resolve()
    median_path = cache / "kustomize-r1" / "grade" / "median.json"
    median, public = verify(median_path, args.public, args.reading, cache)
    written = copy.deepcopy(median)
    lane_notes = notes(public)
    written["notes"] = merge_notes(median.get("notes", []), lane_notes)
    require(not any(bad in note for note in lane_notes for bad in
                    (".yaml", ".yml", "/Users/", "CODEOWNERS", "copyright", "@")),
            "private source identity appeared in public notes")
    require({key: value for key, value in written.items() if key != "notes"}
            == {key: value for key, value in median.items() if key != "notes"},
            "writer changed a measured field")
    if not args.check:
        args.reading.write_text(json.dumps(written, indent=2, ensure_ascii=False) + "\n")
    print(json.dumps({"checked": True, "written": not args.check,
                      "median_sha256": digest(median_path),
                      "notes_sha256": hashlib.sha256(json.dumps(written["notes"]).encode()).hexdigest(),
                      "level_met": written["level_met"]}, sort_keys=True))


if __name__ == "__main__":
    main()
