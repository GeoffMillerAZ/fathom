#!/usr/bin/env python3
"""Grade old and repaired Terraform engines against the same pinned native truth."""

import argparse
import hashlib
import json
import pathlib
import shutil
import subprocess


PINS = json.loads(pathlib.Path(__file__).with_name("pins.json").read_text())
DATE = "2026-10-26"
EXPECTED = {
    "terraform-aws-modules/terraform-aws-eks": (87, 523029, 866),
    "terraform-aws-modules/terraform-aws-vpc": (77, 453543, 2662),
}


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def subject_path(cache, pin):
    return cache / "git-subjects" / (pin["repository"].replace("/", "--") + "--" + pin["sha"][:12])


def check_inputs(cache):
    raw_hashes = {}
    for pin in PINS["subjects"]:
        repo = pin["repository"]
        slug = repo.rsplit("-", 1)[-1]
        receipt = json.loads((cache / "native" / (slug + "-before-receipt.json")).read_text())
        oracle = cache / "native" / (slug + "-before.json")
        if (receipt["subject_sha"] != pin["sha"] or receipt["oracle_raw_sha256"] != sha256(oracle)
                or receipt["xref_truth"] != EXPECTED[repo][2]):
            raise RuntimeError("native input identity or truth denominator differs: " + repo)
        head = subprocess.check_output(("git", "-C", str(subject_path(cache, pin)), "rev-parse", "HEAD"), text=True).strip()
        if head != pin["sha"]:
            raise RuntimeError("subject checkout moved: " + repo)
        raw_hashes[repo] = sha256(oracle)
    return raw_hashes


def verify_binary(binary, cwd, expected_prefix):
    head = subprocess.check_output(("git", "-C", str(cwd), "rev-parse", "HEAD"), text=True).strip()
    status = subprocess.check_output(("git", "-C", str(cwd), "status", "--porcelain"))
    if not head.startswith(expected_prefix) or status:
        raise RuntimeError("grading checkout is not the clean expected commit: " + expected_prefix)
    build = subprocess.check_output(("go", "version", "-m", str(binary)), text=True)
    settings = {}
    for line in build.splitlines():
        if line.startswith("\tbuild\t") and "=" in line:
            key, value = line.split("\tbuild\t", 1)[1].split("=", 1)
            settings[key] = value
    if settings.get("vcs.revision") != head or settings.get("vcs.modified") != "false":
        raise RuntimeError("indexbench binary was not built from its clean grading checkout: " + expected_prefix)
    return head


def check_reading(path, commit):
    reading = json.loads(path.read_text())
    if reading["fathom_commit"] != commit or reading["measured_at"] != DATE:
        raise RuntimeError("reading version/date differs: " + str(path))
    if len(reading["subjects"]) != 2:
        raise RuntimeError("reading omitted or duplicated a subject: " + str(path))
    for subject in reading["subjects"]:
        repo = subject["name"]
        files, size, floor = EXPECTED[repo]
        if (subject["files"], subject["bytes"], subject["counts"]["xref_truth"]) != (files, size, floor):
            raise RuntimeError("measured file set or native denominator changed: " + repo)
        if subject["sha"] != next(pin["sha"] for pin in PINS["subjects"] if pin["repository"] == repo):
            raise RuntimeError("subject SHA differs: " + repo)
    if reading["counts"]["xref_truth"] != 3528:
        raise RuntimeError("combined native denominator differs")
    if reading["tenancy"]["faults"] != 0:
        raise RuntimeError("extractor fault in reading")
    return reading


def stable(reading):
    return {"grade": reading["grade"], "metrics": reading["metrics"],
            "counts": reading["counts"],
            "subjects": {subject["name"]: subject["counts"] for subject in reading["subjects"]}}


def measure(cache, binary, cwd, commit, label, runs):
    directory = cache / "native" / "grades" / label
    directory.mkdir(parents=True, exist_ok=True)
    readings = []
    for attempt in range(1, runs + 1):
        path = directory / f"run-{attempt}.json"
        path.unlink(missing_ok=True)
        for pin in PINS["subjects"]:
            repo = pin["repository"]
            slug = repo.rsplit("-", 1)[-1]
            args = (str(binary), "lang-oracle", "-dialect", "terraform",
                    "-subject", str(subject_path(cache, pin)), "-subject-name", repo,
                    "-tool-oracle", str(cache / "native" / (slug + "-before.json")),
                    "-date", DATE, "-commit", commit, "-out", str(path))
            subprocess.run(args, cwd=cwd, check=True)
        readings.append(check_reading(path, commit))
    if any(stable(reading) != stable(readings[0]) for reading in readings[1:]):
        raise RuntimeError("grade or count varied across runs: " + label)
    middle = sorted(range(runs), key=lambda i: readings[i]["tenancy"]["throughput_mb_s"])[runs // 2]
    selected = directory / "selected.json"
    shutil.copyfile(directory / f"run-{middle + 1}.json", selected)
    return {"commit": commit, "binary_sha256": sha256(binary),
            "selected_run": middle + 1, "selected_reading_sha256": sha256(selected),
            "run_sha256": [sha256(directory / f"run-{i}.json") for i in range(1, runs + 1)],
            "metrics": readings[middle]["metrics"], "grade": readings[middle]["grade"],
            "counts": readings[middle]["counts"],
            "subjects": {subject["name"]: {"files": subject["files"], "bytes": subject["bytes"],
                                          "counts": subject["counts"]}
                         for subject in readings[middle]["subjects"]}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=pathlib.Path, required=True)
    parser.add_argument("--baseline-bin", type=pathlib.Path, required=True)
    parser.add_argument("--baseline-cwd", type=pathlib.Path, required=True)
    parser.add_argument("--repaired-bin", type=pathlib.Path, required=True)
    parser.add_argument("--repaired-cwd", type=pathlib.Path, required=True)
    parser.add_argument("--runs", type=int, default=3)
    args = parser.parse_args()
    if args.runs < 3 or args.runs % 2 == 0:
        parser.error("--runs must be odd and at least 3")
    cache = args.cache.resolve()
    raw_hashes = check_inputs(cache)
    baseline_head = verify_binary(args.baseline_bin.resolve(), args.baseline_cwd.resolve(), "6db920e72")
    repaired_head = verify_binary(args.repaired_bin.resolve(), args.repaired_cwd.resolve(), "2ff8b2ac3")
    baseline = measure(cache, args.baseline_bin.resolve(), args.baseline_cwd.resolve(),
                       "6db920e72", "baseline", args.runs)
    repaired = measure(cache, args.repaired_bin.resolve(), args.repaired_cwd.resolve(),
                       "2ff8b2ac3", "repaired", args.runs)
    report = {"date": DATE, "native_raw_sha256": raw_hashes,
              "baseline_full_commit": baseline_head, "repaired_full_commit": repaired_head,
              "baseline": baseline, "repaired": repaired}
    path = cache / "native" / "grades" / "comparison.json"
    path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"date": DATE, "baseline": {"commit": baseline["commit"],
                      "xref_recall": baseline["metrics"]["xref_recall"],
                      "xref_truth_found": baseline["counts"]["xref_truth_found"]},
                      "repaired": {"commit": repaired["commit"],
                      "xref_recall": repaired["metrics"]["xref_recall"],
                      "xref_truth_found": repaired["counts"]["xref_truth_found"]}}, sort_keys=True))


if __name__ == "__main__":
    main()
