#!/usr/bin/env python3
"""Verify GLSL oracle bundles and annotate a median-throughput reading."""
import argparse
from collections import Counter
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys

HERE = Path(__file__).resolve().parent
SUBJECTS = ("vulkan", "spirv-cross", "vulkan-samples")
DISPLAY_NAMES = {"vulkan": "SaschaWillems/Vulkan", "spirv-cross": "KhronosGroup/SPIRV-Cross",
                 "vulkan-samples": "KhronosGroup/Vulkan-Samples"}
def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def oracle_truth_sha256(oracle):
    """Hash only normalized call and scope truth, excluding incidental JSON shape."""
    calls = sorted(oracle["calls"], key=lambda row: (row["path"], row["from"], row["to"]))
    truth = {"calls": calls, "scope": sorted(oracle["scope"])}
    raw = json.dumps(truth, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(raw).hexdigest()


def adapter():
    sys.dont_write_bytecode = True
    spec = importlib.util.spec_from_file_location("glslang_ast", HERE / "glslang_ast.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def source_identity(override=None, expected=None):
    """Require a clean source checkout and bind any override to its actual HEAD."""
    env = os.environ.copy()
    env["GIT_OPTIONAL_LOCKS"] = "0"
    head = subprocess.run(["git", "rev-parse", "HEAD"], capture_output=True, text=True,
                          env=env, check=True).stdout.strip()
    if not re.fullmatch(r"[0-9a-f]{40}", head):
        raise ValueError("source HEAD is not a full SHA-1 commit")
    status = subprocess.run(["git", "status", "--porcelain", "--untracked-files=all"],
                            capture_output=True, text=True, env=env, check=True).stdout
    if status:
        raise ValueError("source worktree is dirty: " + status[:500])
    if override and override != head:
        raise ValueError(f"FATHOM_COMMIT must equal actual source HEAD {head}")
    if expected and expected != head:
        raise ValueError(f"source HEAD changed during reading: {head} != {expected}")
    return head


def check_bundle(clones, runs, kind, catalog_mode="core-plus-types",
                 extension_index=None, extension_review=None):
    tool = adapter()
    manifest_path = tool.RECIPES_PATH
    _, manifest = tool.load_recipe_manifest(manifest_path)
    root = (clones / kind).resolve()
    receipt_path = runs / kind / "receipt.json"
    oracle_path = runs / kind / "oracle.json"
    receipt = json.loads(receipt_path.read_text())
    oracle = json.loads(oracle_path.read_text())
    identity = tool.subject_identity(root, manifest["pins"][kind])
    for key in ("subject_sha", "shader_tree_sha256", "shader_tree_file_count"):
        if receipt[key] != identity[key]:
            raise ValueError(f"{kind}: receipt {key} mismatch")
    compiler = Path(receipt["compiler"])
    pin = tool.compiler_identity(compiler)
    for key, value in pin.items():
        if receipt.get(key) != value:
            raise ValueError(f"{kind}: receipt {key} mismatch")
    if receipt["recipe_manifest_sha256"] != sha(manifest_path):
        raise ValueError(f"{kind}: recipe manifest digest mismatch")
    _, _, catalog = tool.catalog_for_mode(
        catalog_mode, HERE / "glsl460-functions.json", extension_index, extension_review)
    for key in ("catalog_mode", "catalog_fingerprint_sha256", "core_catalog_sha256",
                "extension_catalog_sha256", "catalog_function_count",
                "catalog_constructor_count"):
        if receipt.get(key) != catalog[key]:
            raise ValueError(f"{kind}: receipt {key} mismatch")
    if receipt.get("builtin_index_sha256") != catalog["catalog_fingerprint_sha256"]:
        raise ValueError(f"{kind}: builtin catalog fingerprint mismatch")
    rows = receipt["files"]
    expected = manifest["subjects"][kind]
    if receipt["population"] != expected["population"] or len(rows) != expected["population"]:
        raise ValueError(f"{kind}: population mismatch")
    if {row["path"] for row in rows} != {item["path"] for item in expected["files"]}:
        raise ValueError(f"{kind}: file set mismatch")
    parsed = {row["path"] for row in rows if row["status"] == "parsed"}
    if receipt["counts"] != dict(Counter(row["status"] for row in rows)):
        raise ValueError(f"{kind}: status counts mismatch")
    if set(oracle["scope"]) != parsed or len(oracle["scope"]) != len(parsed):
        raise ValueError(f"{kind}: oracle scope mismatch")
    if oracle.get("definitions") != [] or any(call["path"] not in parsed for call in oracle["calls"]):
        raise ValueError(f"{kind}: oracle call scope/shape mismatch")
    if len(parsed) / len(rows) < 0.95:
        raise ValueError(f"{kind}: below 95% oracle coverage")
    return {"subject": kind, "files": len(rows), "parsed": len(parsed), "calls": len(oracle["calls"]),
            "rejects": {k: v for k, v in receipt["counts"].items() if k != "parsed"},
            "subject_sha": receipt["subject_sha"],
            "shader_tree_sha256": receipt["shader_tree_sha256"],
            "oracle_path": str(oracle_path.resolve()), "oracle_sha256": sha(oracle_path),
            "oracle_truth_sha256": oracle_truth_sha256(oracle),
            "receipt_sha256": sha(receipt_path), "catalog_mode": catalog["catalog_mode"],
            "catalog_fingerprint_sha256": catalog["catalog_fingerprint_sha256"],
            "extension_catalog_sha256": catalog["extension_catalog_sha256"],
            "catalog_function_count": catalog["catalog_function_count"],
            "catalog_constructor_count": catalog["catalog_constructor_count"]}


def median_run(readings):
    rates = [float(data["tenancy"]["throughput_mb_s"]) for data in readings]
    return sorted(range(len(rates)), key=lambda i: (rates[i], i))[len(rates) // 2]


def annotate(readings, bundles, source_commit):
    if any(data.get("fathom_commit") != source_commit for data in readings):
        raise ValueError("reading source commits differ from requested commit")
    expected_names = set(DISPLAY_NAMES.values())
    expected_files = {DISPLAY_NAMES[b["subject"]]: b["files"] for b in bundles}
    expected_shas = {DISPLAY_NAMES[b["subject"]]: b["subject_sha"] for b in bundles}
    modes = {bundle["catalog_mode"] for bundle in bundles}
    fingerprints = {bundle["catalog_fingerprint_sha256"] for bundle in bundles}
    catalog_counts = {(bundle["catalog_function_count"], bundle["catalog_constructor_count"])
                      for bundle in bundles}
    if len(modes) != 1:
        raise ValueError("oracle bundles differ in catalog mode")
    if len(fingerprints) != 1:
        raise ValueError("oracle bundles differ in catalog fingerprint")
    if len(catalog_counts) != 1:
        raise ValueError("oracle bundles differ in catalog counts")
    for data in readings:
        entries = data["subjects"]
        if len(entries) != len(expected_names) or {item["name"] for item in entries} != expected_names:
            raise ValueError("reading subjects differ from canonical GLSL subjects")
        if {item["name"]: item["files"] for item in entries} != expected_files:
            raise ValueError("reading file counts differ from unchanged oracle populations")
        if {item["name"]: item["sha"] for item in entries} != expected_shas:
            raise ValueError("reading subject SHAs differ from pinned oracle subjects")
    chosen = median_run(readings)
    reading = readings[chosen]
    catalog_mode = next(iter(modes))
    catalog_fingerprint = next(iter(fingerprints))
    catalog_function_count, catalog_constructor_count = next(iter(catalog_counts))
    subjects = reading["subjects"]
    errors = sum(item["tenancy"]["files_with_error"] for item in subjects)
    files = sum(item["files"] for item in subjects)
    rates = [data["tenancy"]["throughput_mb_s"] for data in readings]
    notes = [
        "GLSL oracle coverage: " + "; ".join(
            f"{b['subject']} {b['parsed']}/{b['files']} parsed, {b['calls']} call pairs, rejects {b['rejects']}"
            for b in bundles) + "; every file remains in the reading (no -only)",
        "GLSL tenancy parse errors: " + "; ".join(
            f"{item['name']} {item['tenancy']['files_with_error']}/{item['files']}"
            for item in subjects) + f"; pooled {errors}/{files} ({100 * errors / files:.1f}%)",
        "GLSL CPU throughput: three full-corpus runs " + ", ".join(str(x) for x in rates)
        + f" MB/s; median run {chosen + 1} ({rates[chosen]} MB/s)",
        "GLSL oracle receipts: " + "; ".join(
            f"{b['subject']} subject {b['subject_sha']}, oracle SHA-256 {b['oracle_sha256']}, receipt SHA-256 {b['receipt_sha256']}"
            for b in bundles),
        "GLSL source commit: " + source_commit,
        "GLSL exclusion catalog: " + catalog_mode + " SHA-256 " + catalog_fingerprint
        + f"; {catalog_function_count} functions, {catalog_constructor_count} type identifiers",
    ]
    reading["notes"] = sorted(set(reading.get("notes", [])) | set(notes))
    return reading, chosen


def evidence_payload(reading_path, chosen, bundles, scratch, source_commit):
    """Bind one emitted reading to its selected-run inputs in a private receipt."""
    modes = {bundle["catalog_mode"] for bundle in bundles}
    fingerprints = {bundle["catalog_fingerprint_sha256"] for bundle in bundles}
    extension_hashes = {bundle.get("extension_catalog_sha256") for bundle in bundles}
    if len(modes) != 1 or len(fingerprints) != 1 or len(extension_hashes) != 1:
        raise ValueError("oracle bundles differ in evidence catalog identity")
    rows = []
    by_subject = {bundle["subject"]: bundle for bundle in bundles}
    if set(by_subject) != set(SUBJECTS):
        raise ValueError("evidence bundles differ from canonical GLSL subjects")
    for subject in SUBJECTS:
        bundle = by_subject[subject]
        oracle_path = Path(bundle["oracle_path"])
        oracle = json.loads(oracle_path.read_text())
        if sha(oracle_path) != bundle["oracle_sha256"]:
            raise ValueError(f"{subject}: oracle changed after verification")
        if oracle_truth_sha256(oracle) != bundle["oracle_truth_sha256"]:
            raise ValueError(f"{subject}: oracle truth changed after verification")
        unmatched_path = (scratch / f"{subject}-run{chosen + 1}-unmatched.json").resolve()
        if not unmatched_path.is_file():
            raise ValueError(f"{subject}: selected-run unmatched file missing")
        rows.append({
            "name": DISPLAY_NAMES[subject], "sha": bundle["subject_sha"],
            "files": bundle["files"], "shader_tree_sha256": bundle["shader_tree_sha256"],
            "unmatched_path": str(unmatched_path), "unmatched_sha256": sha(unmatched_path),
            "oracle_path": str(oracle_path.resolve()), "oracle_sha256": bundle["oracle_sha256"],
            "oracle_truth_sha256": bundle["oracle_truth_sha256"],
        })
    return {
        "product_commit": source_commit, "catalog_mode": next(iter(modes)),
        "catalog_fingerprint_sha256": next(iter(fingerprints)),
        "extension_catalog_sha256": next(iter(extension_hashes)),
        "reading_sha256": sha(reading_path), "subjects": rows,
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("action", choices=("source", "verify", "annotate"))
    ap.add_argument("--clones", type=Path)
    ap.add_argument("--runs", type=Path)
    ap.add_argument("--scratch", type=Path)
    ap.add_argument("--out", type=Path)
    ap.add_argument("--source-commit")
    ap.add_argument("--override")
    ap.add_argument("--expected")
    ap.add_argument("--catalog-mode", choices=("baseline-core", "core-plus-types", "full-registry"),
                    default="core-plus-types")
    ap.add_argument("--extension-index", type=Path)
    ap.add_argument("--extension-review", type=Path)
    args = ap.parse_args()
    if args.action == "source":
        print(source_identity(args.override, args.expected))
        return
    if not args.clones or not args.runs:
        ap.error("verify and annotate require --clones and --runs")
    bundles = [check_bundle(args.clones, args.runs, kind, args.catalog_mode,
                            args.extension_index, args.extension_review) for kind in SUBJECTS]
    if args.action == "verify":
        print(json.dumps(bundles, indent=2))
        return
    if not args.scratch or not args.out or not args.source_commit:
        ap.error("annotate requires --scratch, --out and --source-commit")
    readings = [json.loads((args.scratch / f"run{i}.json").read_text()) for i in (1, 2, 3)]
    reading, chosen = annotate(readings, bundles, args.source_commit)
    args.out.write_text(json.dumps(reading, indent=2) + "\n")
    evidence_path = args.scratch / "evidence.json"
    evidence = evidence_payload(args.out, chosen, bundles, args.scratch, args.source_commit)
    evidence_path.write_text(json.dumps(evidence, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"median_run": chosen + 1, "level_met": reading.get("level_met"),
                      "grade": reading.get("grade"),
                      "throughput_mb_s": reading["tenancy"]["throughput_mb_s"],
                      "evidence": str(evidence_path)}))


if __name__ == "__main__":
    main()
