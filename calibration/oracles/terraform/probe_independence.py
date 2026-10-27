#!/usr/bin/env python3
"""Check native input boundary and source sensitivity on a synthetic module.

Changing an unrelated JSON file is only a boundary check, not an actual
extractor-output perturbation. The latter requires a separate indexbench run.
This invokes tofu init/show/graph only; no plan, apply, or cloud API.
"""

import argparse
import hashlib
import json
import pathlib
import subprocess


def run(oracle, tofu, mirror, source, scratch):
    result = subprocess.check_output((str(oracle), "-subject", str(source),
                                      "-scratch", str(scratch), "-mirror", str(mirror),
                                      "-tofu", str(tofu)))
    return hashlib.sha256(result).hexdigest(), len(json.loads(result)["references"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=pathlib.Path, required=True)
    parser.add_argument("--oracle-bin", type=pathlib.Path, required=True)
    parser.add_argument("--tofu", type=pathlib.Path, required=True)
    parser.add_argument("--mirror", type=pathlib.Path, required=True)
    args = parser.parse_args()
    base = args.cache.resolve() / "native" / "independence"
    source = base / "source"
    source.mkdir(parents=True, exist_ok=True)
    main_tf = source / "main.tf"
    unrelated = source / "unrelated.json"
    main_tf.write_text('variable "a" {\n  type = string\n}\noutput "value" {\n  value = var.a\n}\n')
    unrelated.write_text('{"probe":1}\n')
    same_source, refs = run(args.oracle_bin, args.tofu, args.mirror, source, base / "scratch")
    unrelated.write_text('{"probe":2}\n')
    changed_unrelated, _ = run(args.oracle_bin, args.tofu, args.mirror, source, base / "scratch")
    main_tf.write_text('variable "b" {\n  type = string\n}\noutput "value" {\n  value = var.b\n}\n')
    changed_source, _ = run(args.oracle_bin, args.tofu, args.mirror, source, base / "scratch")
    if same_source != changed_unrelated or same_source == changed_source or refs == 0:
        raise SystemExit("native truth independence probe failed")
    receipt = {"unrelated_json_perturbation": "raw truth unchanged (boundary only)",
               "source_fact_perturbation": "raw truth changed",
               "original_raw_sha256": same_source,
               "changed_source_raw_sha256": changed_source,
               "original_references": refs}
    (base / "receipt.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
    print(json.dumps(receipt, sort_keys=True))


if __name__ == "__main__":
    main()
