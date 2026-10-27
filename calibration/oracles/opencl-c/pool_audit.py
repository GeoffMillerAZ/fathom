#!/usr/bin/env python3
"""Audit the frozen OpenCL C pool using verified Git checkouts.

The public receipt contains subject IDs, counts and the frozen policy digest.
Checkout paths and raw object inventories remain in the private workspace receipt.
"""

import argparse
from collections import Counter
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import sys

sys.dont_write_bytecode = True
_SPEC = importlib.util.spec_from_file_location("opencl_pool_triage", Path(__file__).with_name("triage.py"))
triage = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(triage)


FULL_SHA_RE = re.compile(r"[0-9a-f]{40}\Z")
ID_RE = re.compile(r"[a-z0-9][a-z0-9_-]*\Z")
POLICY_SCHEMA = "opencl-c-pool-rule-v1"
SUMMARY_SCHEMA = "opencl-c-pool-audit-v1"


def _sha256(data):
    return hashlib.sha256(data).hexdigest()


def load_policy(path):
    """Return outcome-blind primary pins and declared diagnostic IDs."""
    try:
        raw = path.read_bytes()
        policy = json.loads(raw)
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError("policy is not readable JSON") from exc
    if policy.get("schema") != POLICY_SCHEMA:
        raise ValueError("unsupported policy schema")

    approved_list = policy.get("osi_approved_licenses")
    if (not isinstance(approved_list, list) or not approved_list
            or any(not isinstance(item, str) or not item for item in approved_list)
            or len(approved_list) != len(set(approved_list))):
        raise ValueError("policy approved-license list is invalid")
    approved = set(approved_list)

    primary = {}
    rows = policy.get("primary_subjects")
    if not isinstance(rows, list) or not rows:
        raise ValueError("policy has no primary subjects")
    for row in rows:
        if not isinstance(row, dict):
            raise ValueError("policy primary subject is invalid")
        subject_id = row.get("id")
        pin = row.get("pin")
        licenses = row.get("licenses")
        if not isinstance(subject_id, str) or not ID_RE.fullmatch(subject_id):
            raise ValueError("policy primary subject ID is invalid")
        if subject_id in primary:
            raise ValueError("policy primary subject IDs must be unique")
        if row.get("role") != "primary":
            raise ValueError("policy primary subject has a non-primary role")
        if not isinstance(pin, str) or not FULL_SHA_RE.fullmatch(pin):
            raise ValueError("policy primary subject pin is invalid")
        if (not isinstance(licenses, list) or not licenses
                or any(not isinstance(item, str) or item not in approved for item in licenses)):
            raise ValueError("every primary license must be OSI-approved by the policy")
        primary[subject_id] = pin

    diagnostics = set()
    diagnostic_rows = policy.get("diagnostic_or_ineligible")
    if not isinstance(diagnostic_rows, list):
        raise ValueError("policy diagnostic subjects are invalid")
    for row in diagnostic_rows:
        if not isinstance(row, dict) or not isinstance(row.get("id"), str):
            raise ValueError("policy diagnostic subject is invalid")
        subject_id = row["id"]
        if not ID_RE.fullmatch(subject_id) or subject_id in diagnostics or subject_id in primary:
            raise ValueError("policy diagnostic subject ID is invalid or duplicated")
        diagnostics.add(subject_id)
    return {"primary": primary, "diagnostics": diagnostics, "sha256": _sha256(raw)}


def parse_subject(value):
    subject_id, separator, path = value.partition("=")
    if not separator or not ID_RE.fullmatch(subject_id) or not path:
        raise argparse.ArgumentTypeError("subject must be id=checkout")
    return subject_id, Path(path)


def parse_diagnostic(value):
    first, separator, pin = value.rpartition("=")
    subject_id, separator2, path = first.partition("=")
    if (not separator or not separator2 or not ID_RE.fullmatch(subject_id)
            or not path or not FULL_SHA_RE.fullmatch(pin)):
        raise argparse.ArgumentTypeError("diagnostic must be id=checkout=full-pin")
    return subject_id, Path(path), pin


def _git_cat_file_sizes(root, object_ids):
    unique = sorted(set(object_ids))
    if not unique:
        return {}
    result = subprocess.run(
        ["git", "-C", str(root), "cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)"],
        input=("\n".join(unique) + "\n").encode("ascii"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if result.returncode:
        raise ValueError("could not inspect tracked blob metadata")
    sizes = {}
    for line in result.stdout.decode("ascii").splitlines():
        fields = line.split(" ")
        if len(fields) != 3 or fields[1] != "blob" or fields[0] not in unique:
            raise ValueError("tracked object is not a readable blob")
        try:
            sizes[fields[0]] = int(fields[2])
        except ValueError as exc:
            raise ValueError("tracked blob has invalid size metadata") from exc
    if set(sizes) != set(unique):
        raise ValueError("tracked blob metadata is incomplete")
    return sizes


def audit_checkout(root, pin):
    """Verify a checkout and count every regular exact-lowercase .cl entry."""
    evidence = triage.capture_checkout(root, pin)
    entries = triage._parse_stage(evidence.stage_raw)
    object_ids = [object_id for mode, object_id, path_raw in entries
                  if mode in ("100644", "100755") and path_raw.endswith(b".cl")]
    sizes = _git_cat_file_sizes(evidence.root, object_ids)
    occurrences = Counter(object_ids)
    public = {
        "files": len(object_ids),
        "distinct_blobs": len(occurrences),
        "bytes": sum(sizes[object_id] for object_id in object_ids),
    }
    private = {
        "checkout": str(evidence.root),
        "pin": pin,
        "stage_sha256": _sha256(evidence.stage_raw),
        "blobs": [
            {"object_id": object_id, "files": occurrences[object_id], "bytes": sizes[object_id]}
            for object_id in sorted(occurrences)
        ],
    }
    return public, private


def audit(policy, subjects, diagnostic_subjects=()):
    supplied = {}
    for subject_id, root in subjects:
        if subject_id in supplied:
            raise ValueError("subject IDs must be unique")
        supplied[subject_id] = root
    expected = set(policy["primary"])
    if set(supplied) != expected:
        raise ValueError("supplied primary subjects must exactly match the policy")

    diagnostics = {}
    for subject_id, root, pin in diagnostic_subjects:
        if subject_id in diagnostics or subject_id in supplied:
            raise ValueError("all supplied subject IDs must be unique")
        if subject_id not in policy["diagnostics"]:
            raise ValueError("diagnostic subject is not declared by the policy")
        diagnostics[subject_id] = (root, pin)

    public_rows = []
    private_rows = []
    for subject_id in sorted(supplied):
        counts, evidence = audit_checkout(supplied[subject_id], policy["primary"][subject_id])
        if counts["files"] == 0:
            raise ValueError("declared primary subject has an empty lowercase .cl population")
        public_rows.append({"id": subject_id, "role": "primary", **counts})
        private_rows.append({"id": subject_id, "role": "primary", **evidence})
    for subject_id in sorted(diagnostics):
        root, pin = diagnostics[subject_id]
        counts, evidence = audit_checkout(root, pin)
        public_rows.append({"id": subject_id, "role": "diagnostic", **counts})
        private_rows.append({"id": subject_id, "role": "diagnostic", **evidence})

    primary_rows = [row for row in public_rows if row["role"] == "primary"]
    diagnostic_rows = [row for row in public_rows if row["role"] == "diagnostic"]
    def totals(rows):
        row_ids = {row["id"] for row in rows}
        blob_ids = {
            blob["object_id"]
            for subject in private_rows if subject["id"] in row_ids
            for blob in subject["blobs"]
        }
        return {
            "subjects": len(rows),
            "files": sum(row["files"] for row in rows),
            "distinct_blobs": len(blob_ids),
            "bytes": sum(row["bytes"] for row in rows),
        }
    public = {
        "schema": SUMMARY_SCHEMA,
        "policy_sha256": policy["sha256"],
        "primary": primary_rows,
        "primary_totals": totals(primary_rows),
        "diagnostics": diagnostic_rows,
        "diagnostic_totals": totals(diagnostic_rows),
    }
    private = {"policy_sha256": policy["sha256"], "subjects": private_rows}
    return public, private


def _validated_out(value, parser):
    out = value.resolve()
    workspace = (Path.cwd() / "workspace").resolve()
    if workspace not in out.parents or out.suffix != ".json":
        parser.error("--out must be a JSON file below this checkout's workspace/")
    if out.exists():
        parser.error("--out must not already exist")
    private = out.parent / "private" / (out.stem + "-evidence.json")
    if private.exists():
        parser.error("private evidence output must not already exist")
    return out, private


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--policy", type=Path, default=Path(__file__).with_name("pool-rule.json"))
    parser.add_argument("--subject", action="append", required=True, type=parse_subject,
                        help="repeat for every primary: id=checkout")
    parser.add_argument("--diagnostic", action="append", default=[], type=parse_diagnostic,
                        help="optional declared diagnostic: id=checkout=full-pin")
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args(argv)
    out, private_out = _validated_out(args.out, parser)
    policy = load_policy(args.policy)
    public, private = audit(policy, args.subject, args.diagnostic)

    private_out.parent.mkdir(parents=True, exist_ok=True)
    private_out.write_text(json.dumps(private, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(public, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(public, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
