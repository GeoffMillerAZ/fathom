#!/usr/bin/env python3
"""Run pinned native Kustomize truth over every clean, fixed subject."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

HERE = Path(__file__).resolve().parent
PINS_PATH = HERE / "pins.json"
NATIVE_SHA256 = "bc77e3505fbd414551cf7b15d1e772a288bc5afd97e31e49655b10524d1888e9"


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def pins():
    return json.loads(PINS_PATH.read_text())


def slug(pin):
    return pin["repository"].replace("/", "--")


def checkout(cache, pin):
    root = cache / "git-subjects" / (slug(pin) + "--" + pin["sha"][:12])
    env = dict(os.environ, GIT_OPTIONAL_LOCKS="0")
    head = subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True, env=env).strip()
    dirty = subprocess.check_output(["git", "-C", str(root), "status", "--porcelain", "--untracked-files=all"], env=env)
    if head != pin["sha"] or dirty:
        raise RuntimeError("subject HEAD or clean tree differs: " + pin["repository"])
    subject = (root / pin.get("subpath", ".")).resolve()
    if not subject.is_dir() or not subject.is_relative_to(root.resolve()):
        raise RuntimeError("invalid pinned subpath: " + pin["repository"])
    return subject


def paths_file(path):
    raw = path.read_text()
    rows = raw.splitlines()
    if not rows or len(rows) != len(set(rows)) or rows != sorted(rows):
        raise RuntimeError("native built-file list is empty, duplicated or unsorted")
    if any(not p or Path(p).is_absolute() or ".." in Path(p).parts for p in rows):
        raise RuntimeError("unsafe native built-file path")
    return rows


def wait_for_load():
    """Wait between subjects rather than abandoning a completed partial run."""
    while True:
        load = os.getloadavg()[0]
        if load <= 9:
            return load
        time.sleep(20)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--cache", type=Path, required=True)
    ap.add_argument("--oracle-bin", type=Path, required=True)
    ap.add_argument("--phase", choices=("before", "after"), required=True)
    args = ap.parse_args()
    priority = os.getpriority(os.PRIO_PROCESS, 0)
    if priority < 10:
        raise RuntimeError("run under nice -n 10")
    cache = args.cache.resolve()
    oracle = args.oracle_bin.resolve(strict=True)
    native = cache / "tools" / "kustomize-5.8.1"
    if digest(native) != NATIVE_SHA256:
        raise RuntimeError("native Kustomize binary differs from pinned SHA256")
    if subprocess.check_output([str(native), "version"], text=True).strip() != "v5.8.1":
        raise RuntimeError("native Kustomize version differs")
    out = cache / "kustomize-r1" / "native"
    out.mkdir(parents=True, exist_ok=True)
    summary = []
    for pin in pins()["subjects"]:
        subject = checkout(cache, pin)
        base = out / (slug(pin) + "-" + args.phase)
        truth, files, receipt, log = [base.with_suffix(s) for s in (".json", ".files", ".receipt", ".log")]
        scratch = cache / "kustomize-r1" / "scratch" / (slug(pin) + "-" + args.phase)
        scratch.mkdir(parents=True, exist_ok=True)
        cmd = [str(oracle), "-subject", str(subject), "-kustomize", str(native),
               "-kustomize-sha256", NATIVE_SHA256, "-scratch", str(scratch),
               "-files-out", str(files), "-receipt-out", str(receipt)]
        load_at_launch = wait_for_load()
        with truth.open("wb") as stdout, log.open("wb") as stderr:
            subprocess.run(cmd, stdout=stdout, stderr=stderr, check=True)
        data = json.loads(truth.read_text())
        rec = json.loads(receipt.read_text())
        built = paths_file(files)
        if (data.get("tier") != "A" or rec.get("version") != "v5.8.1"
                or rec.get("binary_sha256") != NATIVE_SHA256 or rec.get("built_roots", 0) < 1):
            raise RuntimeError("native truth/receipt identity or successful build missing")
        for family in ("definitions", "references", "cross_references"):
            if not isinstance(data.get(family), list):
                raise RuntimeError("native oracle family missing: " + family)
        manifest = cache / "kustomize-r1" / "manifests" / (slug(pin) + "-" + args.phase + ".json")
        if not manifest.exists():
            raise RuntimeError("tracked-set manifest must precede native run: " + pin["repository"])
        if args.phase == "after":
            prior = out / (slug(pin) + "-before")
            for suffix in (".json", ".files", ".receipt"):
                if (prior.with_suffix(suffix)).read_bytes() != base.with_suffix(suffix).read_bytes():
                    raise RuntimeError("native before/after bytes changed: " + pin["repository"] + suffix)
        summary.append({"repository": pin["repository"], "subject_sha": pin["sha"],
                        "subpath": pin.get("subpath", "."), "oracle_sha256": digest(truth),
                        "files_sha256": digest(files), "receipt_sha256": digest(receipt),
                        "built_files": len(built), "definitions": len(data["definitions"]),
                        "references": len(data["references"]),
                        "cross_references": len(data["cross_references"]),
                        "native_receipt": rec, "load_at_launch": load_at_launch})
    record = {"phase": args.phase, "pins_sha256": digest(PINS_PATH),
              "oracle_bin_sha256": digest(oracle), "native_bin_sha256": digest(native),
              "niceness": priority,
              "go_runtime_env": {key: os.environ.get(key, "") for key in ("GOMAXPROCS", "GOFLAGS", "GOWORK", "GOTOOLCHAIN")},
              "subjects": summary}
    (out / (args.phase + "-summary.json")).write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"phase": args.phase, "subjects": len(summary)}, sort_keys=True))


if __name__ == "__main__":
    main()
