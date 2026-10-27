#!/usr/bin/env python3
"""Run the committed §13.3.1 tracked-set recipe on both exact Git subjects."""

import argparse
import json
import os
import pathlib
import subprocess
import sys


ROOT = pathlib.Path(__file__).resolve().parents[3]
PINS = json.loads(pathlib.Path(__file__).with_name("pins.json").read_text())
RECIPE = ROOT / "docs" / "design" / "language-breadth.md"
START = "SUBJECT=workspace/calibration/<subject> SUBPATH=. RECEIPT=workspace/artifacts/lang-<id>/<subject>-before python3 - <<'PY'\n"


def recipe_body():
    document = RECIPE.read_text()
    if document.count(START) != 1:
        raise RuntimeError("tracked-set recipe marker is absent or ambiguous")
    remainder = document.split(START, 1)[1]
    body, end = remainder.split("\nPY\n```", 1)
    if not end.lstrip("\n").startswith("### 13.4"):
        raise RuntimeError("tracked-set recipe boundary changed")
    return body + "\n"


def manifest(cache, phase, pin, body):
    repo = pin["repository"]
    slug = repo.rsplit("-", 1)[-1]
    subject = cache / "git-subjects" / (repo.replace("/", "--") + "--" + pin["sha"][:12])
    head = subprocess.check_output(("git", "-C", str(subject), "rev-parse", "HEAD"), text=True).strip()
    if head != pin["sha"] or subprocess.check_output(("git", "-C", str(subject), "status", "--porcelain")):
        raise RuntimeError("subject checkout differs from exact clean pin: " + repo)
    receipt = cache / "native" / "manifests" / (slug + "-" + phase)
    env = os.environ.copy()
    env.update(SUBJECT=str(subject), SUBPATH=".", RECEIPT=str(receipt))
    result = subprocess.run((sys.executable, "-c", body), env=env, check=True,
                            capture_output=True, text=True)
    summary = json.loads(result.stdout)
    if (receipt.with_suffix(".untracked")).read_bytes():
        raise RuntimeError("subject checkout has untracked files: " + repo)
    if phase == "after":
        before = json.loads((cache / "native" / "manifests" / (slug + "-before.json")).read_text())
        if summary != before:
            raise RuntimeError("before/after tracked-set manifest changed: " + repo)
    return {"repository": repo, "subject_sha": pin["sha"], **summary}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=pathlib.Path, required=True)
    parser.add_argument("--phase", choices=("before", "after"), required=True)
    args = parser.parse_args()
    body = recipe_body()
    summaries = [manifest(args.cache.resolve(), args.phase, pin, body) for pin in PINS["subjects"]]
    print(json.dumps({"phase": args.phase, "subjects": summaries}, sort_keys=True))


if __name__ == "__main__":
    main()
