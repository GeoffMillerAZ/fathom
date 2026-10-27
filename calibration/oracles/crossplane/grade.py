#!/usr/bin/env python3
"""Grade the original Crossplane corpus against the complete tier-C walk.

Inputs and intermediate evidence stay in the operator's ignored cache. The
final reading is copied byte-for-byte from one of three full harness runs.
"""

import argparse
import fnmatch
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time


REPO = Path(__file__).resolve().parents[3]
PINS = REPO / "calibration/oracles/crossplane/pins-r1.json"
CANONICAL = (
    ("crossplane/crossplane", "bf8ffd4048a0bb1b408b8b745e31960c717009cb",
     ("cluster/composition/**", "test/e2e/manifests/**/*.yaml")),
    ("upbound/platform-ref-aws", "be065e6afe89e656080557bacf1732168502e325",
     ("*.yaml", "*.yml")),
    ("upbound/configuration-aws-network", "4631cb7ca7b4cecd4a54b8d64b061cc4e7f19c7d",
     ("*.yaml", "*.yml")),
)
COUNT_PAIRS = {
    "sym_precision": ("matched", "fathom_defs"),
    "sym_recall": ("matched", "oracle_defs"),
    "ref_precision": ("ref_fathom_in_truth", "ref_fathom"),
    "ref_recall": ("ref_truth_found", "ref_truth"),
    "xref_precision": ("xref_fathom_in_truth", "xref_fathom"),
    "xref_recall": ("xref_truth_found", "xref_truth"),
}


def digest(path):
    sha = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            sha.update(chunk)
    return sha.hexdigest()


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], text=True).strip()


def clean_head(root, sha):
    if git(root, "rev-parse", "--show-toplevel") != str(root):
        raise RuntimeError(f"subject is not a checkout root: {root}")
    if git(root, "rev-parse", "HEAD") != sha:
        raise RuntimeError(f"subject HEAD differs from pin: {root}")
    if git(root, "status", "--porcelain", "--untracked-files=all"):
        raise RuntimeError(f"subject checkout is dirty: {root}")


def binary_identity(binary, commit):
    details = subprocess.check_output(["go", "version", "-m", str(binary)], text=True)
    settings = {}
    for line in details.splitlines():
        fields = line.split()
        if len(fields) == 2 and fields[0] == "build" and "=" in fields[1]:
            key, value = fields[1].split("=", 1)
            settings[key] = value
    if settings.get("vcs.revision") != commit or settings.get("vcs.modified") != "false":
        raise RuntimeError("indexbench is not a clean build of --engine-commit")
    return digest(binary), settings


def canonical_subjects(manifest):
    pins = json.loads(PINS.read_text())["subjects"]
    if [(p["repo"], p["sha"], tuple(p.get("scope", ("*.yaml", "*.yml"))))
            for p in pins[:3]] != list(CANONICAL) or any(not p.get("prior") for p in pins[:3]):
        raise RuntimeError("original three pins differ from the canonical corpus")
    rows = manifest.get("subjects")
    if not isinstance(rows, list):
        raise RuntimeError("manifest has no subjects")
    by_id = {row.get("id"): row for row in rows}
    if len(by_id) != len(rows):
        raise RuntimeError("duplicate manifest subject")
    selected = []
    for name, sha, scope in CANONICAL:
        row = by_id.get(name)
        if row is None or row.get("sha") != sha or tuple(row.get("scope", ())) != scope:
            raise RuntimeError(f"manifest lacks exact canonical subject: {name}")
        root = Path(row["root"]).resolve(strict=True)
        clean_head(root, sha)
        selected.append((name, sha, scope, root))
    return selected


def scope_files(root, scope):
    """All canonical YAML candidates, before Crossplane document dispatch."""
    files = []
    for path in root.rglob("*"):
        if ".git" in path.parts or path.is_symlink() or not path.is_file():
            continue
        rel = path.relative_to(root).as_posix()
        if path.suffix.lower() in (".yaml", ".yml") and any(
                fnmatch.fnmatchcase(rel, pattern) for pattern in scope):
            files.append(rel)
    return sorted(files)


def verify_reading(reading, unmatched, names, name, sha, expected):
    rows = reading.get("subjects", [])
    if len(rows) != len(names) or {row.get("name") for row in rows} != set(names):
        raise RuntimeError("reading subject population differs")
    row = next(row for row in rows if row.get("name") == name)
    if row.get("sha") != sha:
        raise RuntimeError("reading subject identity differs")
    measured = unmatched.get("tenancy_files")
    if not isinstance(measured, list) or len(measured) != len(set(measured)):
        raise RuntimeError("missing or duplicate measured-file receipt")
    if not set(measured).issubset(expected):
        raise RuntimeError("harness measured files outside canonical scope")
    if row.get("files") != len(measured):
        raise RuntimeError("reading file count differs from measured-file receipt")
    if row.get("tenancy", {}).get("faults") != 0:
        raise RuntimeError("faulting Crossplane subject")
    if any(row.get("families", {}).get(f) != "C" for f in ("l1", "refs", "xref")):
        raise RuntimeError("non-C oracle family")
    return row, measured


def check_counts(reading):
    subjects = reading["subjects"]
    for metric, (numerator, denominator) in COUNT_PAIRS.items():
        counts = reading["counts"]
        if sum(row["counts"].get(numerator, 0) for row in subjects) != counts.get(numerator, 0):
            raise RuntimeError(f"pooled numerator differs: {metric}")
        if sum(row["counts"].get(denominator, 0) for row in subjects) != counts.get(denominator, 0):
            raise RuntimeError(f"pooled denominator differs: {metric}")
        expected = (counts[numerator] / counts[denominator]) if counts[denominator] else None
        actual = reading["metrics"].get(metric)
        if expected is None:
            if actual is not None:
                raise RuntimeError(f"unexpected metric: {metric}")
        elif not isinstance(actual, (int, float)) or abs(actual - expected) > 0.000051:
            raise RuntimeError(f"metric/count disagreement: {metric}")
        if reading["metric_tiers"].get(metric) != "C":
            raise RuntimeError(f"non-C pooled metric: {metric}")
    if reading.get("tenancy", {}).get("faults") != 0:
        raise RuntimeError("faulting pooled reading")


def launch_conditions():
    """Admit each child separately; an outer nice/load check can go stale."""
    while True:
        priority = os.getpriority(os.PRIO_PROCESS, 0)
        if priority < 10:
            raise RuntimeError("run the driver under nice -n 10 or lower priority")
        load = os.getloadavg()[0]
        if load <= 9:
            return {"load_1m": load, "nice": priority,
                    "go_env": {key: os.environ.get(key) for key in
                               ("GOMAXPROCS", "GOFLAGS", "GOWORK", "GOTOOLCHAIN")}}
        time.sleep(20)


def run_logged(command, cwd, log):
    conditions = launch_conditions()
    with log.open("wb") as stream:
        subprocess.run(command, cwd=cwd, stdout=stream, stderr=subprocess.STDOUT, check=True)
    return conditions


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--cache", type=Path, required=True, help="ignored private evidence directory")
    ap.add_argument("--manifest", type=Path, required=True, help="pinned probe manifest")
    ap.add_argument("--indexbench", type=Path, required=True)
    ap.add_argument("--oracle-bin", type=Path, required=True)
    ap.add_argument("--engine-commit", required=True, help="full commit of clean indexbench build")
    ap.add_argument("--out", type=Path, required=True, help="private final reading path")
    args = ap.parse_args()
    args.out.unlink(missing_ok=True)
    if len(args.engine_commit) != 40 or any(c not in "0123456789abcdef" for c in args.engine_commit):
        raise RuntimeError("--engine-commit must be a full 40-digit Git SHA")
    cache = args.cache.resolve(strict=True)
    manifest = args.manifest.resolve(strict=True)
    out = args.out.resolve()
    if not out.is_relative_to(cache) or not manifest.is_relative_to(cache):
        raise RuntimeError("manifest and output must stay under --cache")
    binary = args.indexbench.resolve(strict=True)
    oracle = args.oracle_bin.resolve(strict=True)
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise RuntimeError("run the driver under nice -n 10 or lower priority")
    bench_sha, settings = binary_identity(binary, args.engine_commit)
    subjects = canonical_subjects(json.loads(manifest.read_text()))
    work = cache / "crossplane-canonical-grade"
    work.mkdir(parents=True, exist_ok=True)
    expected = {(name, sha): set(scope_files(root, scope)) for name, sha, scope, root in subjects}
    if sum(map(len, expected.values())) < 244:
        raise RuntimeError("canonical subjects have fewer than 244 YAML candidates")
    run_records = []
    for number in range(1, 4):
        reading_path = work / f"run-{number}.json"
        reading_path.unlink(missing_ok=True)
        subjects_record = []
        for name, sha, scope, root in subjects:
            label = name.replace("/", "--")
            oracle_path = work / f"oracle-{number}-{label}.json"
            oracle_log = work / f"oracle-{number}-{label}.log"
            oracle_command = [str(oracle), "-subject", str(root)]
            oracle_conditions = launch_conditions()
            with oracle_path.open("wb") as truth, oracle_log.open("wb") as errors:
                subprocess.run(oracle_command, cwd=REPO, stdout=truth, stderr=errors, check=True)
            raw = json.loads(oracle_path.read_text())
            if raw.get("tier") != "C" or raw.get("tool") != "crossplane-oracle (independent gopkg.in/yaml.v3 walk)":
                raise RuntimeError("oracle is not the independent full tier-C walk")
            outside_scope = {family: sum(item.get("path") not in expected[(name, sha)]
                                         for item in raw[family])
                             for family in ("definitions", "references", "cross_references")}
            unmatched = work / f"unmatched-{number}-{label}.json"
            harness_log = work / f"harness-{number}-{label}.log"
            disclosure = ("Independent yaml.v3 tier-C full walk, including composed elements; "
                          "original three pinned subjects and canonical corpus scope; "
                          "no native validation filter or post-harness reading edits")
            cmd = [str(binary), "lang-oracle", "-dialect", "crossplane", "-subject", str(root),
                   "-subject-name", name, "-subject-sha", sha, "-tool-oracle", str(oracle_path),
                   "-date", "2026-10-26", "-commit", args.engine_commit, "-unmatched", str(unmatched),
                   "-out", str(reading_path), "-note", disclosure]
            if scope != ("*.yaml", "*.yml"):
                cmd += ["-only", ",".join(scope)]
            harness_conditions = run_logged(cmd, REPO, harness_log)
            reading = json.loads(reading_path.read_text())
            row, measured = verify_reading(reading, json.loads(unmatched.read_text()),
                                           [s[0] for s in subjects[:len(subjects_record) + 1]],
                                           name, sha, expected[(name, sha)])
            subjects_record.append({"name": name, "sha": sha, "canonical_scope": list(scope),
                                    "yaml_candidates": len(expected[(name, sha)]),
                                    "measured_files": len(measured), "bytes": row["bytes"],
                                    "counts": row["counts"], "oracle_sha256": digest(oracle_path),
                                    "oracle_entries_outside_canonical_scope": outside_scope,
                                    "oracle_command": oracle_command, "oracle_log_sha256": digest(oracle_log),
                                    "oracle_launch": oracle_conditions,
                                    "harness_command": cmd, "harness_log_sha256": digest(harness_log),
                                    "harness_launch": harness_conditions,
                                    "unmatched_sha256": digest(unmatched)})
        if [s["measured_files"] for s in subjects_record] != [238, 3, 3]:
            raise RuntimeError("full canonical Crossplane population must be 238 + 3 + 3 = 244 files")
        check_counts(reading)
        run_records.append({"reading_path": str(reading_path), "reading_sha256": digest(reading_path),
                            "throughput_mb_per_cpu_second": reading["tenancy"]["throughput_mb_s"],
                            "subjects": subjects_record, "counts": reading["counts"],
                            "metrics": reading["metrics"], "grade": reading["grade"],
                            "level_met": reading["level_met"]})
    first = run_records[0]
    for record in run_records[1:]:
        for field in ("counts", "metrics", "grade", "level_met"):
            if record[field] != first[field]:
                raise RuntimeError(f"three full-pool runs disagree on {field}")
        if [(s["counts"], s["measured_files"]) for s in record["subjects"]] != [
                (s["counts"], s["measured_files"]) for s in first["subjects"]]:
            raise RuntimeError("three full-pool runs disagree by subject")
    median = sorted(range(3), key=lambda i: (run_records[i]["throughput_mb_per_cpu_second"], i))[1]
    out.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(run_records[median]["reading_path"], out)
    summary = {"manifest_sha256": digest(manifest), "pins_sha256": digest(PINS),
               "indexbench_sha256": bench_sha, "indexbench_build": settings,
               "oracle_sha256": digest(oracle), "engine_commit": args.engine_commit,
               "median_run": median + 1, "output_sha256": digest(out), "runs": run_records}
    (work / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"median_run": median + 1, "output_sha256": digest(out),
                      "files": [s["measured_files"] for s in run_records[median]["subjects"]],
                      "grade": first["grade"]}, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (OSError, KeyError, ValueError, RuntimeError, subprocess.CalledProcessError) as exc:
        sys.exit(f"crossplane grade: {exc}")
