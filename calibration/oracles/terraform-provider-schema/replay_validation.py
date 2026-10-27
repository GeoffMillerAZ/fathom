#!/usr/bin/env python3
"""Replay final native validate captures without changing historical receipts.

No provider runs. The optional second capture is a byte comparison, not a new
native observation; its exit status is borrowed from the bound final receipt.
"""

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path

SPEC = importlib.util.spec_from_file_location("provider_schema_probe", Path(__file__).with_name("probe.py"))
probe = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(probe)

AGGREGATES = ("exit_zero", "valid", "error_count", "warning_count", "diagnostic_count",
              "attributed_error_count", "unattributed_error_count", "diagnostic_severities")


def sha256(raw):
    return hashlib.sha256(raw).hexdigest()


def require_receipt(receipt):
    if (not isinstance(receipt, dict) or receipt.get("status") != "complete" or
            not isinstance(receipt.get("providers"), dict) or
            set(receipt["providers"]) != set(probe.NAMES)):
        raise ValueError("final_receipt_incomplete")
    for name in probe.NAMES:
        provider = receipt["providers"][name]
        if (not isinstance(provider, dict) or provider.get("status") != "complete" or
                not isinstance(provider.get("baseline_validation"), dict) or
                not isinstance(provider.get("mutations"), list) or len(provider["mutations"]) != 4 or
                [row.get("output_index") for row in provider["mutations"] if isinstance(row, dict)] != list(range(4))):
            raise ValueError("final_receipt_incomplete")


def validate_capture_identity(capture, receipt):
    """Require the same authored sources and schema/show bytes for all providers."""
    for name in probe.NAMES:
        provider = receipt["providers"][name]
        source = (capture / name / "main.tf").read_bytes()
        if sha256(source) != provider.get("source_sha256"):
            raise ValueError("source_capture_mismatch")
        for kind in ("schema", "show"):
            raw = (capture / (name + "." + kind + ".stdout")).read_bytes()
            section = provider.get(kind)
            if not isinstance(section, dict) or sha256(raw) != section.get("raw_sha256"):
                raise ValueError("schema_show_capture_mismatch")


def replay_one(capture, name, label, historical, enforce_original):
    if not isinstance(historical, dict) or not isinstance(historical.get("exit_zero"), bool):
        raise ValueError("validation_receipt_shape")
    raw = (capture / (name + "." + label + ".stdout")).read_bytes()
    raw_hash = sha256(raw)
    if enforce_original and raw_hash != historical.get("raw_sha256"):
        raise ValueError("validation_capture_mismatch")
    data = json.loads(raw)
    rc = 0 if historical["exit_zero"] else 1
    observed = probe.summarize_validation(data, rc, raw_hash)
    if enforce_original and any(observed[key] != historical.get(key) for key in AGGREGATES):
        raise ValueError("validation_aggregate_mismatch")
    return {"raw_sha256": observed["raw_sha256"],
            "canonical_sha256": observed["canonical_sha256"],
            "exit_zero_from_receipt": historical["exit_zero"],
            "valid": observed["valid"], "error_count": observed["error_count"],
            "warning_count": observed["warning_count"],
            "diagnostic_count": observed["diagnostic_count"],
            "attributed_error_count": observed["attributed_error_count"],
            "unattributed_error_count": observed["unattributed_error_count"],
            "diagnostic_severities": observed["diagnostic_severities"]}


def replay(capture, receipt, enforce_original):
    validate_capture_identity(capture, receipt)
    rows = {}
    for name in probe.NAMES:
        provider = receipt["providers"][name]
        rows[name] = {"baseline": replay_one(capture, name, "baseline",
                                               provider["baseline_validation"], enforce_original)}
        for index, mutation in enumerate(provider["mutations"]):
            rows[name]["mutation" + str(index)] = replay_one(
                capture, name, "mutation" + str(index), mutation["validation"], enforce_original)
    return rows


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--capture", type=Path, required=True)
    parser.add_argument("--receipt", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--compare-capture", type=Path)
    args = parser.parse_args(argv)
    if os.path.lexists(args.out):
        raise FileExistsError("stale_output")
    receipt_bytes = args.receipt.read_bytes()
    receipt = json.loads(receipt_bytes)
    require_receipt(receipt)
    primary = replay(args.capture, receipt, True)
    result = {"scope": "historical validate capture replay; no native command executed",
              "receipt_sha256": sha256(receipt_bytes),
              "probe_sha256": probe.digest(Path(probe.__file__)),
              "validate_stream_count": 15, "providers": primary}
    if args.compare_capture is not None:
        comparison = replay(args.compare_capture, receipt, False)
        result["comparison"] = {
            "scope": "capture-only comparison; exit status borrowed from final receipt",
            "providers": comparison,
            "canonical_equal_count": sum(
                primary[name][label]["canonical_sha256"] == comparison[name][label]["canonical_sha256"]
                for name in probe.NAMES for label in primary[name]),
            "raw_equal_count": sum(
                primary[name][label]["raw_sha256"] == comparison[name][label]["raw_sha256"]
                for name in probe.NAMES for label in primary[name]),
        }
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("xb") as stream:
        stream.write(probe.json_bytes(result) + b"\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
