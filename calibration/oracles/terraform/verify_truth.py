#!/usr/bin/env python3
"""Reject incomplete raw native truth before grading Terraform."""

import argparse
import hashlib
import json
import pathlib


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--subject", required=True, help="repository name in pins.json")
    parser.add_argument("--oracle", type=pathlib.Path, required=True)
    args = parser.parse_args()
    pins = json.loads(pathlib.Path(__file__).with_name("pins.json").read_text())
    floor = pins["xref_truth_floors"][args.subject]
    raw = args.oracle.read_bytes()
    truth = json.loads(raw)
    if truth.get("tier") != "A" or truth.get("version") != pins["opentofu"]["version"]:
        raise SystemExit("native tool tier/version differs from pinned calibration")
    count = len(truth.get("cross_references", []))
    if count < floor:
        raise SystemExit("xref truth below pinned floor: {} < {}".format(count, floor))
    if not truth.get("definitions") or not truth.get("references"):
        raise SystemExit("native truth missing definitions or references")
    print(json.dumps({"subject": args.subject, "xref_truth": count, "floor": floor,
                      "raw_sha256": hashlib.sha256(raw).hexdigest()}, sort_keys=True))


if __name__ == "__main__":
    main()
