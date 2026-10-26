#!/usr/bin/env python3
"""Replay repository-owned doc fixtures before/after the Terraform engine change.

This records cross-dialect regression evidence, not full-subject calibration.
It injects the adjacent Go instrument into indexbench with a temporary overlay.
The before mode restores the two changed engine files from the named Git base.
All generated inputs and outputs belong in the caller's ignored artifact cache.
"""

import argparse
import hashlib
import json
import os
import pathlib
import subprocess
import tempfile


ROOT = pathlib.Path(__file__).resolve().parents[3]


def conservation(base):
    """Require one intended reading delta and preserve every other reading."""
    prefix = "docs/bench/readings/lang/"
    tracked = subprocess.check_output(
        ("git", "ls-tree", "-r", "--name-only", base, prefix), cwd=ROOT, text=True)
    paths = sorted(name for name in tracked.splitlines() if name.endswith(".json"))
    current = sorted(str(path.relative_to(ROOT)) for path in (ROOT / prefix).rglob("*.json"))
    if paths != current:
        raise ValueError("reading file population changed")
    before_hashes, after_hashes, changed = [], [], []
    eligible = protected = 0
    for name in paths:
        old = subprocess.check_output(("git", "show", base + ":" + name), cwd=ROOT)
        new = (ROOT / name).read_bytes()
        before_hashes.append([name, hashlib.sha256(old).hexdigest()])
        after_hashes.append([name, hashlib.sha256(new).hexdigest()])
        relative = name[len(prefix):]
        if relative.startswith(("baseline/", "parity/")):
            protected += 1
            if old != new:
                raise ValueError("protected reading bytes changed: " + name)
        else:
            eligible += 1
            a, b = json.loads(old), json.loads(new)
            if a.get("required_kinds") != b.get("required_kinds"):
                raise ValueError("required kinds changed: " + name)
            if a != b:
                changed.append(relative)
        if old != new and relative != "terraform.json":
            raise ValueError("unexpected reading byte change: " + name)
    if changed != ["terraform.json"]:
        raise ValueError("expected exactly the Terraform reading delta")
    result = {"base": base, "files": len(paths), "eligible": eligible,
              "protected_byte_identical": protected, "changed_values": changed,
              "unchanged_bytes": len(paths) - 1, "required_kinds_preserved": True,
              "before_inventory_sha256": hashlib.sha256(json.dumps(before_hashes).encode()).hexdigest(),
              "after_inventory_sha256": hashlib.sha256(json.dumps(after_hashes).encode()).hexdigest()}
    print(json.dumps(result, indent=2, sort_keys=True))


def compare(before_path, after_path):
    before = json.loads(before_path.read_text())
    after = json.loads(after_path.read_text())
    for key in ("reading_files", "eligible_reading_files", "doc_reading_files",
                "eligible_doc_reading_files", "unavailable_routes"):
        if before[key] != after[key]:
            raise ValueError("reading-route population changed: " + key)
    old = {row["dialect"]: row for row in before["dialects"]}
    new = {row["dialect"]: row for row in after["dialects"]}
    if old.keys() != new.keys():
        raise ValueError("fixture dialect population changed")
    changed_extraction, changed_resolution = [], []
    for dialect in sorted(old):
        a, b = old[dialect], new[dialect]
        for key in ("files", "bytes", "input_sha256", "errors", "missing_fixtures"):
            if a[key] != b[key]:
                raise ValueError("fixture input population changed: " + dialect + ":" + key)
        if a["extraction_sha256"] != b["extraction_sha256"]:
            changed_extraction.append(dialect)
        if a["resolved_sha256"] != b["resolved_sha256"]:
            changed_resolution.append(dialect)
        if dialect != "terraform" and a != b:
            raise ValueError("non-Terraform fixture behavior changed: " + dialect)
    summary = {
        "dialects": len(old),
        "fixture_files": sum(row["files"] for row in old.values()),
        "changed_extraction": changed_extraction,
        "changed_resolution": changed_resolution,
        "mixed_edges_before": before["mixed_edges"],
        "mixed_edges_after": after["mixed_edges"],
        "mixed_hash_equal": before["mixed_sha256"] == after["mixed_sha256"],
        "limitation": "repository fixtures; not full pinned calibration subjects",
    }
    print(json.dumps(summary, indent=2, sort_keys=True))


def replay(output, base):
    output = output.resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    scratch = pathlib.Path(os.environ.get("TMPDIR", ROOT / "workspace/tmp"))
    scratch.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="terraform-doc-impact-", dir=scratch) as tmp:
        directory = pathlib.Path(tmp)
        helper = directory / "impact_replay_test.go"
        helper.write_bytes(pathlib.Path(__file__).with_name("impact_replay_test.go.txt").read_bytes())
        replacements = {str(ROOT / "cmd/indexbench/terraform_doc_impact_test.go"): str(helper)}
        if base:
            for name in ("internal/extractor/hclextract.go", "internal/fusion/docxref.go"):
                original = directory / (pathlib.Path(name).name + ".original")
                original.write_bytes(subprocess.check_output(("git", "show", base + ":" + name), cwd=ROOT))
                replacements[str(ROOT / name)] = str(original)
            replacements[str(ROOT / "internal/fusion/docxref_terraform.go")] = ""
        overlay = directory / "overlay.json"
        overlay.write_text(json.dumps({"Replace": replacements}) + "\n")
        environment = dict(os.environ, GOWORK="off", IAC_FIXTURE_REPLAY_OUT=str(output))
        subprocess.run(("go", "test", "-race", "-overlay", str(overlay), "./cmd/indexbench",
                        "-run", "^TestTerraformDocFixtureImpactReplay$", "-count=1", "-v"),
                       cwd=ROOT, env=environment, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=pathlib.Path)
    parser.add_argument("--base", help="pre-change Git commit for the two engine files")
    parser.add_argument("--compare", nargs=2, type=pathlib.Path, metavar=("BEFORE", "AFTER"))
    parser.add_argument("--conservation-base", help="check final reading values and protected bytes against Git")
    args = parser.parse_args()
    if args.conservation_base:
        if args.compare or args.out or args.base:
            parser.error("--conservation-base is a separate mode")
        conservation(args.conservation_base)
    elif args.compare:
        if args.out or args.base:
            parser.error("--compare is a separate mode")
        compare(*args.compare)
    elif args.out:
        replay(args.out, args.base)
    else:
        parser.error("--out, --compare or --conservation-base is required")


if __name__ == "__main__":
    main()
