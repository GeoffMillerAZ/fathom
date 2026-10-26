#!/usr/bin/env python3
"""Copy the verified Kustomize median reading with public, counts-only notes."""

import argparse
import copy
import hashlib
import json
from pathlib import Path

from run_native import PINS_PATH, digest, pins

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


def verify(median_path, public_path, current_path, cache):
    frozen = pins()
    median, public, current = read(median_path), read(public_path), read(current_path)
    require(digest(PINS_PATH) == public["pins_sha256"], "pin manifest differs from public receipt")
    require(digest(median_path) == public["median_reading_sha256"],
            "unannotated median SHA256 differs from public receipt")
    require(public["base_commit"] == frozen["base_commit"] == median["fathom_commit"],
            "product source commit differs")
    require(median["dialect"] == current["dialect"] == "kustomize", "dialect differs")
    for key in ("kind", "target_level", "required_kinds"):
        require(median[key] == current[key], "existing reading contract changed: " + key)
    for key in ("counts", "metrics", "metric_tiers", "grade", "level_met"):
        require(median[key] == public[key], "measured reading differs from public receipt: " + key)
    measured, published = check_subjects(median, public, frozen)
    require(median["measured_at"] == public["date"], "measurement date differs")
    require(median["oracles"] == [{"tier": "A", "tool": "kustomize", "version": "v5.8.1"}],
            "median tool oracle identity differs")
    preference_log = ROOT / "workspace" / "logs" / "kustomize-r1-directory-preference.log"
    preference = preference_log.read_text()
    require("ok  \tgithub.com/geoffmilleraz/fathom/internal/fusion" in preference
            and "FAIL" not in preference, "directory-preference check is not green")
    counts = median["counts"]
    require(counts["xref_fathom"] == public["D19_xref_denominators"]["precision"] >= 20,
            "D19 precision denominator below 20 or inconsistent")
    require(counts["xref_truth"] == public["D19_xref_denominators"]["recall"] >= 20,
            "D19 recall denominator below 20 or inconsistent")
    require(not public["D19_xref_thin"], "public D19 receipt says thin")
    originals = {pin["repository"] for pin in frozen["subjects"] if pin["membership"] == "original"}
    require(len(originals) == 2, "original subject membership differs")
    for name in originals:
        require(public["original_xref_truth"][name] == measured[name]["counts"]["xref_truth"],
                "original xref count differs: " + name)
    require(public["original_xref_truth"] == {"kubernetes-sigs/kustomize": 12,
                                               "argoproj/argocd-example-apps": 1},
            "original subject xref floors changed")
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
            and median["tenancy"]["throughput_mb_s"] == 12.36
            and median["tenancy"]["max_file_ms"] == 0,
            "median CPU/fault timing differs")
    require(public["niceness"] >= 10 and public["go_runtime_env"] == {
        "GOMAXPROCS": "2", "GOFLAGS": "-p=2", "GOWORK": "off", "GOTOOLCHAIN": "go1.27.3"},
        "recorded execution settings differ")
    launches = public["load_at_launch_by_run"]
    require(len(launches) == 3 and all(len(run) == 5 for run in launches)
            and all(set(row["repository"] for row in run) == set(measured)
                    and all(0 <= row["load_at_launch"] <= 9 for row in run)
                    for run in launches), "per-launch load records missing or above 9")
    subjects = list(published.values())
    require(sum(row["native_files"] for row in subjects) == 58
            and sum(row["graded_files"] for row in subjects) == 46
            and sum(row["component_contexts"] for row in subjects) == 15
            and sum(row["component_context_counts"]["valid"] for row in subjects) == 12
            and sum(row["component_context_counts"]["tried"] for row in subjects) == 12
            and sum(row["component_context_counts"]["effective_entries"] for row in subjects) == 17
            and sum(row["native_excluded"].get("component-no-valid-context", 0)
                    for row in subjects) == 9, "native population summary differs")
    require(sum(row["native_cross_references"] for row in subjects) == 144
            and counts["xref_truth"] == 35, "raw/scoped cross-reference counts differ")
    require(counts["fathom_defs"] - counts["oracle_defs"] == 8
            and counts["matched"] == counts["oracle_defs"] == 194,
            "eight extra definitions or native coverage changed")
    flux = {"fluxcd/flux2-kustomize-helm-example", "fluxcd/flux2-multi-tenancy"}
    for name, row in published.items():
        extra = row["graded_counts"]["fathom_defs"] - row["graded_counts"]["oracle_defs"]
        require(extra == (4 if name in flux else 0), "extra definitions moved: " + name)
    require(sum(row["native_stats"].get("noop:resources", 0) for row in subjects) == 8
            and sum(row["native_stats"].get("noop", 0) for row in subjects) == 8,
            "native resource no-op count differs")
    native_pin = frozen["native"]
    require(native_pin["version"] == "v5.8.1"
            and native_pin["api_version"] == native_pin["kyaml_version"] == "v0.21.1"
            and public["native_bin_sha256"] == "bc77e3505fbd414551cf7b15d1e772a288bc5afd97e31e49655b10524d1888e9",
            "pinned native provenance differs")
    require(median["grade"] == {"L1": "good", "L2": "excellent", "L3": "excellent",
                                "L4": "not-applicable"} and median["level_met"] == "L3",
            "expected measured grade differs")
    return median, public


def notes(public):
    native = pins()["native"]
    return [
        "Native provenance: official Kustomize v5.8.1 archive SHA256 "
        + native["archive_sha256"] + ", binary SHA256 " + public["native_bin_sha256"]
        + "; typed API and kyaml v0.21.1; fixed five-subject selection followed private exploration, so it is not a blind holdout.",
        "Native full population: 58 recognized Kustomizations, 46 built files, 15 Components; "
        "six Components have valid real parents across 12 contexts and 17 effective entries; "
        "nine have no valid context and are excluded. No synthetic Component wrapper, Helm renderer, plugin or cloud execution.",
        "Native truth has 144 raw cross-file references; the unchanged dialect owns 35 scored "
        "cross-file references after file/target scope. D19 precision denominator 35 and recall "
        "denominator 35, each at its own metric; original subject xref floors remain 12 and 1.",
        "Fathom has 202 definitions against 194 native definitions; eight extra definitions "
        "are native resource no-ops and remain precision costs. Measured grade: L1 good, "
        "L2 excellent, L3 excellent; references 148/148 and cross-file references 35/35.",
        "Shared Apple Silicon, GOTOOLCHAIN=go1.27.3, nice 10, GOMAXPROCS=2, GOFLAGS=-p=2: "
        "three full-pool CPU throughput runs 11, 12.74 and 12.36 MB/s with stable counts, "
        "metrics and grades; median 12.36 MB/CPU-s, zero faults, max_file_ms 0. "
        "Per-launch one-minute load was recorded at or below 9; concurrent jobs were not "
        "recorded and wall throughput is not comparable across machines.",
        "Product engine unchanged at clean source commit " + public["base_commit"]
        + "; existing directory-preference test green. Public counts/hash evidence: "
        "docs/assessments/2026-10-26-kustomize-d19-evidence.md and "
        "docs/assessments/data/2026-10-26-kustomize-r1.json.",
    ]


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
    written["notes"] = notes(public)
    require(not any(bad in note for note in written["notes"] for bad in
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
