#!/usr/bin/env python3
"""Grade the fixed native Kustomize population three complete times."""

import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess

from run_native import PINS_PATH, checkout, digest, paths_file, pins, slug, wait_for_load

BASE_COMMIT = "6db920e72898abb20bf86c22652b401902e34598"
SUPPORTED = {"kustomization.yaml", "kustomization.yml"}
GO_ENV = {"GOMAXPROCS": "2", "GOFLAGS": "-p=2", "GOWORK": "off", "GOTOOLCHAIN": "go1.27.3"}


def source_binary(binary):
    details = subprocess.check_output(["go", "version", "-m", str(binary)], text=True)
    settings = {}
    for line in details.splitlines():
        fields = line.split()
        if len(fields) == 2 and fields[0] == "build" and "=" in fields[1]:
            key, value = fields[1].split("=", 1)
            settings[key] = value
    if settings.get("vcs.revision") != BASE_COMMIT or settings.get("vcs.modified") != "false":
        raise RuntimeError("indexbench is not a clean build of the fixed source commit")
    return digest(binary)


def exact_scope(subject, built):
    supported = [p for p in built if Path(p).name in SUPPORTED]
    unsupported = len(built) - len(supported)
    if not supported:
        raise RuntimeError("no built Kustomization files supported by the dialect")
    if any("," in p for p in supported):
        raise RuntimeError("file path cannot be represented exactly by -only")
    bytes_count = sum((subject / p).stat().st_size for p in supported)
    return supported, unsupported, bytes_count


def check_measurement(reading, missing, expected, size, pin):
    rows = reading.get("subjects", [])
    row = next((item for item in rows if item.get("name") == pin["repository"]), None)
    if row is None or row.get("sha") != pin["sha"]:
        raise RuntimeError("graded subject identity differs: " + pin["repository"])
    if row.get("files") != len(expected) or row.get("bytes") != size:
        raise RuntimeError("graded file/byte counts differ: " + pin["repository"])
    measured = missing.get("tenancy_files")
    if measured is None or set(measured) != set(expected) or len(measured) != len(expected):
        raise RuntimeError("graded measured-file set differs: " + pin["repository"])
    if row.get("tenancy", {}).get("faults") != 0:
        raise RuntimeError("faulting subject: " + pin["repository"])
    return row


def ratios(counts):
    pairs = {"sym_precision": ("matched", "fathom_defs"),
             "sym_recall": ("matched", "oracle_defs"),
             "ref_precision": ("ref_fathom_in_truth", "ref_fathom"),
             "ref_recall": ("ref_truth_found", "ref_truth"),
             "xref_precision": ("xref_fathom_in_truth", "xref_fathom"),
             "xref_recall": ("xref_truth_found", "xref_truth")}
    return {name: (counts.get(num, 0) / counts[den] if counts.get(den, 0) else None)
            for name, (num, den) in pairs.items()}


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--cache", type=Path, required=True)
    ap.add_argument("--indexbench", type=Path, required=True)
    ap.add_argument("--checkout", type=Path, required=True,
                    help="clean source checkout at the fixed base commit")
    args = ap.parse_args()
    cache = args.cache.resolve()
    binary = args.indexbench.resolve(strict=True)
    source = args.checkout.resolve(strict=True)
    if subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip() != BASE_COMMIT:
        raise RuntimeError("source checkout differs from fixed commit")
    if subprocess.check_output(["git", "-C", str(source), "status", "--porcelain", "--untracked-files=all"]):
        raise RuntimeError("source checkout is dirty")
    binary_sha = source_binary(binary)
    priority = os.getpriority(os.PRIO_PROCESS, 0)
    if priority < 10:
        raise RuntimeError("run under nice -n 10")
    if any(os.environ.get(key) != value for key, value in GO_ENV.items()):
        raise RuntimeError("fixed Go runtime settings differ: GOMAXPROCS=2 GOFLAGS=-p=2 GOWORK=off GOTOOLCHAIN=go1.27.3")
    native_dir = cache / "kustomize-r1" / "native"
    if not (native_dir / "before-summary.json").exists():
        raise RuntimeError("native before phase is required")
    out = cache / "kustomize-r1" / "grade"
    out.mkdir(parents=True, exist_ok=True)
    subjects = []
    for pin in pins()["subjects"]:
        subject = checkout(cache, pin)
        base = native_dir / (slug(pin) + "-before")
        built = paths_file(base.with_suffix(".files"))
        selected, unsupported, size = exact_scope(subject, built)
        subjects.append((pin, subject, base.with_suffix(".json"), selected, unsupported, size))
    runs = []
    for number in range(1, 4):
        reading_path = out / ("run-" + str(number) + ".json")
        if reading_path.exists():
            reading_path.unlink()
        per_subject = []
        conditions = []
        for pin, subject, oracle, selected, unsupported, size in subjects:
            missing_path = out / (slug(pin) + "-unmatched-" + str(number) + ".json")
            log_path = out / (slug(pin) + "-" + str(number) + ".log")
            cmd = [str(binary), "lang-oracle", "-dialect", "kustomize", "-subject", str(subject),
                   "-subject-name", pin["repository"], "-subject-sha", pin["sha"],
                   "-only", ",".join(selected), "-tool-oracle", str(oracle),
                   "-date", "2026-10-26", "-commit", BASE_COMMIT,
                   "-unmatched", str(missing_path), "-out", str(reading_path),
                   "-note", "Native built-file scope; unsupported native basename counted separately; three serialized full-pool runs"]
            load_at_launch = wait_for_load()
            conditions.append({"repository": pin["repository"], "load_at_launch": load_at_launch})
            with log_path.open("wb") as log:
                subprocess.run(cmd, stdout=log, stderr=subprocess.STDOUT, check=True, cwd=source)
            reading = json.loads(reading_path.read_text())
            row = check_measurement(reading, json.loads(missing_path.read_text()), selected, size, pin)
            per_subject.append({"repository": pin["repository"], "sha": pin["sha"],
                                "files": len(selected), "bytes": size,
                                "unsupported_native_basename": unsupported,
                                "counts": row.get("counts", {}),
                                "ratios": ratios(row.get("counts", {})),
                                "families": row.get("families", {})})
        reading = json.loads(reading_path.read_text())
        if len(reading.get("subjects", [])) != len(subjects) or reading.get("tenancy", {}).get("faults") != 0:
            raise RuntimeError("incomplete or faulting pooled reading")
        runs.append({"reading": reading, "subjects": per_subject, "launch_conditions": conditions})
    for run in runs[1:]:
        for key in ("counts", "metrics", "metric_tiers", "grade", "level_met", "required_kinds"):
            if run["reading"].get(key) != runs[0]["reading"].get(key):
                raise RuntimeError("non-deterministic pooled " + key)
        if run["subjects"] != runs[0]["subjects"]:
            raise RuntimeError("non-deterministic per-subject counts")
    rates = [r["reading"]["tenancy"]["throughput_mb_s"] for r in runs]
    median = sorted(range(3), key=lambda i: (rates[i], i))[1]
    shutil.copyfile(out / ("run-" + str(median + 1) + ".json"), out / "median.json")
    original = {p["repository"]: p for p in pins()["subjects"] if p["membership"] == "original"}
    original_counts = {name: next(s for s in runs[median]["subjects"] if s["repository"] == name)["counts"].get("xref_truth", 0)
                       for name in original}
    summary = {"pins_sha256": digest(PINS_PATH), "base_commit": BASE_COMMIT,
               "indexbench_sha256": binary_sha, "runs_mb_per_cpu_second": rates,
               "niceness": priority,
               "go_runtime_env": GO_ENV,
               "load_at_launch_by_run": [run["launch_conditions"] for run in runs],
               "median_run": median + 1, "reading_sha256": digest(out / "median.json"),
               "counts": runs[median]["reading"]["counts"], "metrics": runs[median]["reading"]["metrics"],
               "metric_tiers": runs[median]["reading"].get("metric_tiers", {}),
               "grade": runs[median]["reading"]["grade"], "level_met": runs[median]["reading"]["level_met"],
               "subjects": runs[median]["subjects"], "original_xref_truth": original_counts,
               "original_xref_floor_met": all(original_counts[n] >= (12 if n == "kubernetes-sigs/kustomize" else 1)
                                             for n in original_counts),
               "pool_xref_floor_met": runs[median]["reading"]["counts"].get("xref_truth", 0) >= 13,
               "D19_xref_denominators": {"precision": runs[median]["reading"]["counts"].get("xref_fathom", 0),
                                          "recall": runs[median]["reading"]["counts"].get("xref_truth", 0)}}
    (out / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"median_run": median + 1, "level_met": summary["level_met"],
                      "original_xref_floor_met": summary["original_xref_floor_met"],
                      "pool_xref_floor_met": summary["pool_xref_floor_met"]}, sort_keys=True))


if __name__ == "__main__":
    main()
