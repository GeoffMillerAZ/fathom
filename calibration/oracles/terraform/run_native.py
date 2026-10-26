#!/usr/bin/env python3
"""Run the pinned Terraform native oracle and require complete comparable truth."""

import argparse
import hashlib
import json
import pathlib
import subprocess


PINS = json.loads(pathlib.Path(__file__).with_name("pins.json").read_text())


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def git(subject, *args):
    return subprocess.check_output(("git", "-C", str(subject), *args), text=True).strip()


def run_subject(cache, oracle, scanner, phase, subject_pin):
    repo = subject_pin["repository"]
    slug = repo.rsplit("-", 1)[-1]
    subject = cache / "git-subjects" / (repo.replace("/", "--") + "--" + subject_pin["sha"][:12])
    if git(subject, "rev-parse", "HEAD") != subject_pin["sha"] or git(subject, "status", "--porcelain"):
        raise RuntimeError("subject checkout changed: " + repo)
    manifest = cache / "native" / "manifests" / (slug + "-" + phase + ".json")
    tracked = json.loads(manifest.read_text())
    if tracked["entries"] <= 0 or tracked["files"] <= 0:
        raise RuntimeError("empty tracked-set manifest: " + repo)
    output = cache / "native" / (slug + "-" + phase + ".json")
    partial = output.with_suffix(".json.partial")
    log = cache / "native" / (slug + "-" + phase + ".log")
    files = cache / "native" / (slug + "-" + phase + "-files.txt")
    output.parent.mkdir(parents=True, exist_ok=True)
    preseed = pathlib.Path(__file__).with_name("preseed_modules.py")
    pinned_modules = json.loads(subprocess.check_output(("python3", str(preseed),
        "--cache", str(cache), "--scanner-bin", str(scanner), "--subject", str(subject),
        "--scratch", str(cache / "native" / slug))))
    args = (str(oracle), "-subject", str(subject), "-scratch", str(cache / "native" / slug),
            "-mirror", str(cache / "mirror"), "-tofu", str(cache / "tools" / "tofu-1.13.0"),
            "-files-out", str(files))
    with partial.open("wb") as raw, log.open("wb") as stderr:
        subprocess.run(args, stdout=raw, stderr=stderr, check=True)
    data = partial.read_bytes()
    truth = json.loads(data)
    floor = PINS["xref_truth_floors"][repo]
    count = len(truth.get("cross_references", []))
    if truth.get("tier") != "A" or truth.get("version") != "1.13.0" or count < floor:
        raise RuntimeError("incomplete or below-floor native truth: " + repo + " (" + str(count) + ")")
    receipt = {"repository": repo, "subject_sha": subject_pin["sha"], "phase": phase,
               "tracked_manifest_sha256": tracked["sha256"], "tracked_entries": tracked["entries"],
               "tracked_bytes": tracked["bytes"], "oracle_raw_sha256": sha256(data),
               "xref_truth": count, "xref_truth_floor": floor,
               "definitions": len(truth["definitions"]), "references": len(truth["references"]),
               "measured_file_count": len([x for x in files.read_text().splitlines() if x]),
               "measured_file_list_sha256": sha256(files.read_bytes()),
               "pinned_module_instances": pinned_modules["module_instances"],
               "pinned_module_manifest_sha256": sha256(json.dumps(
                   pinned_modules["manifest_receipts"], sort_keys=True).encode())}
    if phase == "after":
        before = json.loads((cache / "native" / (slug + "-before-receipt.json")).read_text())
        if (receipt["tracked_manifest_sha256"] != before["tracked_manifest_sha256"]
                or receipt["tracked_entries"] != before["tracked_entries"]
                or receipt["oracle_raw_sha256"] != before["oracle_raw_sha256"]
                or receipt["measured_file_list_sha256"] != before["measured_file_list_sha256"]
                or receipt["pinned_module_manifest_sha256"] != before["pinned_module_manifest_sha256"]):
            raise RuntimeError("subject set or raw native truth changed across engine fix: " + repo)
    output.write_bytes(data)
    partial.unlink()
    (cache / "native" / (slug + "-" + phase + "-receipt.json")).write_text(
        json.dumps(receipt, indent=2, sort_keys=True) + "\n")
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=pathlib.Path, required=True)
    parser.add_argument("--oracle-bin", type=pathlib.Path, required=True)
    parser.add_argument("--scanner-bin", type=pathlib.Path, required=True)
    parser.add_argument("--phase", choices=("before", "after"), required=True)
    args = parser.parse_args()
    results = [run_subject(args.cache.resolve(), args.oracle_bin.resolve(), args.scanner_bin.resolve(), args.phase, pin)
               for pin in PINS["subjects"]]
    print(json.dumps({"phase": args.phase, "subjects": results,
                      "total_xref_truth": sum(x["xref_truth"] for x in results)}, sort_keys=True))


if __name__ == "__main__":
    main()
