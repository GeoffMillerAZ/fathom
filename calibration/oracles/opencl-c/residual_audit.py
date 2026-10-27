#!/usr/bin/env python3
"""Reproduce the bounded failing-file receipt from a frozen native screen.

Reads the existing private screen evidence and parses only its failing subset.
The public output omits source text, absolute paths and clean-file inventories.
"""

import argparse
from collections import Counter
import hashlib
import importlib.util
import json
from pathlib import Path, PurePosixPath
import subprocess
import sys


sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
PUBLIC_SCREEN = ROOT / "docs/assessments/data/opencl-c-grammar-gate-2026-10-26/screen.json"
PUBLIC_BUILDS = ROOT / "docs/assessments/data/opencl-c-grammar-gate-2026-10-26/builds.json"
BASELINE_LABELS = ROOT / "docs/assessments/data/opencl-c-grammar-gate-2026-10-26/residual-labels.json"
POOL_RULE = HERE / "pool-rule.json"
GRAMMAR_ID = "opencl-patched"
BUILD_ID = "opencl-c-patched"
GRAMMAR_SYMBOL = "tree_sitter_opencl"
REPRODUCE = (
    "workspace/tools/python/bin/python -I -B "
    "calibration/oracles/opencl-c/residual_audit.py "
    "--screen-dir workspace/opencl-c/screen "
    "--subjects-root workspace/opencl-c/subjects "
    "--grammar workspace/opencl-c/build/opencl-c-patched/native.dylib "
    "--build-receipt workspace/opencl-c/build/opencl-c-patched/build-receipt.json "
    "--out workspace/opencl-c/residual-reproduced.json"
)
IDENTITY_FIELDS = frozenset({"library_sha256", "native_sha256", "receipt_sha256",
                             "configuration_sha256"})
SEMICOLONLESS = frozenset({
    ("darktable", "data/kernels/bilinear.cl"),
    ("darktable", "data/kernels/toneequal.cl"),
    ("opencv", "modules/core/src/opencl/copymakeborder.cl"),
    ("opencv", "modules/core/src/opencl/gemm.cl"),
    ("opencv", "modules/dnn/src/opencl/gemm_image.cl"),
    ("opencv", "modules/dnn/src/opencl/slice.cl"),
    ("opencv", "modules/imgproc/src/opencl/corner.cl"),
    ("opencv", "modules/imgproc/src/opencl/covardata.cl"),
    ("opencv", "modules/imgproc/src/opencl/pyr_down.cl"),
    ("opencv", "modules/imgproc/src/opencl/remap.cl"),
})
MACRO_PARAMETERS = frozenset({
    ("opencv", "modules/core/src/opencl/arithm.cl"),
    ("opencv", "modules/core/src/opencl/mixchannels.cl"),
    ("opencv", "modules/core/src/opencl/reduce.cl"),
    ("opencv", "modules/core/src/opencl/split_merge.cl"),
    ("opencv", "modules/dnn/src/opencl/conv_layer_spatial.cl"),
    ("opencv", "modules/imgproc/src/opencl/morph.cl"),
    ("opencv", "modules/imgproc/src/opencl/morph3x3.cl"),
})
CONDITIONAL_BRANCHES = frozenset({
    ("opencv", "modules/core/src/opencl/inrange.cl"),
    ("opencv", "modules/core/src/opencl/minmaxloc.cl"),
})
RAW_MISSING_SEMICOLON = ("opencv", "modules/dnn/src/opencl/mvn.cl")
MACRO_GENERATED = frozenset({
    ("pyopencl", "pyopencl/cl/pyopencl-random123/philox.cl"),
    ("pyopencl", "pyopencl/cl/pyopencl-random123/threefry.cl"),
})
RODINIA = ("rodinia", "opencl/lavaMD/kernel/kernel_gpu_opencl.cl")


def sha256(raw):
    return hashlib.sha256(raw).hexdigest()


def classify(subject, path):
    key = (subject, path)
    if subject == "clblas":
        return "host_string_template"
    if key in SEMICOLONLESS:
        return "semicolonless_macro_invocation"
    if key == RODINIA:
        return "conditional_cpp_linkage"
    if key in MACRO_PARAMETERS:
        return "macro_parameter_fragment"
    if key in CONDITIONAL_BRANCHES:
        return "conditional_branch_fragment"
    if key == RAW_MISSING_SEMICOLON:
        return "raw_missing_semicolon"
    if key in MACRO_GENERATED:
        return "macro_generated_syntax"
    raise ValueError(f"unknown residual path for {subject}: {path}")


def first_diagnostic(root):
    stack = [root]
    found = []
    while stack:
        node = stack.pop()
        if node.type == "ERROR":
            found.append((node.start_byte, 0, "ERROR", node.start_point[0] + 1))
        if node.is_missing:
            found.append((node.start_byte, 1, f"MISSING {node.type}",
                          node.start_point[0] + 1))
        stack.extend(reversed(node.children))
    if not found:
        raise ValueError("no ERROR or MISSING diagnostic in recorded failure")
    _, _, kind, line = min(found)
    return {"kind": kind, "line": line}


def _load_json(path):
    raw = path.read_bytes()
    return json.loads(raw), sha256(raw)


def _without_native_identity(value):
    """Remove only output identities affected by a native build's binary bytes."""
    if isinstance(value, dict):
        return {key: _without_native_identity(item) for key, item in value.items()
                if key not in IDENTITY_FIELDS}
    if isinstance(value, list):
        return [_without_native_identity(item) for item in value]
    return value


def verify_screen_summary(fresh, baseline):
    if _without_native_identity(fresh) != _without_native_identity(baseline):
        raise ValueError("fresh screen differs from committed population, policy or counts")


def verify_build_receipt(receipt, committed_builds, native_sha):
    matches = [row for row in committed_builds["candidates"]
               if row["candidate"]["id"] == BUILD_ID]
    if len(matches) != 1:
        raise ValueError("committed build is missing pinned candidate")
    baseline = matches[0]
    if receipt.get("schema") != baseline["schema"] or receipt.get("candidate") != baseline["candidate"]:
        raise ValueError("build receipt candidate or commit differs from committed build")
    if receipt.get("patches") != baseline["patches"]:
        raise ValueError("build receipt patches differ from committed build")
    hashes = receipt.get("hashes")
    if not isinstance(hashes, dict) or hashes.get("native_sha256") != native_sha:
        raise ValueError("build receipt native SHA differs from supplied library")
    for name in ("archive_sha256", "candidate_config_sha256", "license_sha256",
                 "original_parser_c_sha256", "parser_c_sha256", "patches_sha256"):
        if hashes.get(name) != baseline["hashes"][name]:
            raise ValueError(f"build receipt {name} differs from committed build")
    return baseline["hashes"]["parser_c_sha256"]


def verify_residual_subset(rows, baseline_labels):
    fields = ("class", "first", "git_blob", "source_sha256", "subject_pin")
    expected = {(row["subject"], row["path"]): tuple(row[field] for field in fields)
                for row in baseline_labels["rows"]}
    actual = {(row["subject"], row["path"]): tuple(row[field] for field in fields)
              for row in rows}
    if len(expected) != 63 or len(actual) != len(rows) or expected != actual:
        raise ValueError("fresh residual paths, labels, diagnostics, pins or blobs differ from baseline")


def verify_subject_pins(configuration, policy):
    expected = {row["id"]: row["pin"] for row in policy["primary_subjects"]}
    actual = {row["id"]: row["pin"] for row in configuration["subjects"]}
    if (len(expected) != len(policy["primary_subjects"])
            or len(actual) != len(configuration["subjects"])
            or actual != expected):
        raise ValueError("fresh subject pins differ from frozen pool rule")


def _load_grammar(path):
    spec = importlib.util.spec_from_file_location("opencl_residual_triage", HERE / "triage.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.load_grammar(GRAMMAR_ID, GRAMMAR_SYMBOL, path).parser


def _subject_source(subject_root, pin, path, tracked):
    relative = PurePosixPath(path)
    if relative.is_absolute() or ".." in relative.parts or relative.suffix != ".cl":
        raise ValueError("unsafe subject-relative .cl path")
    root = subject_root.resolve()
    head = subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"],
                                   text=True).strip()
    if head != pin:
        raise ValueError("subject checkout does not match frozen pin")
    target = root.joinpath(*relative.parts)
    source = target.read_bytes()
    if sha256(source) != tracked["content_sha256"] or len(source) != tracked["bytes"]:
        raise ValueError("subject source differs from captured screen bytes")
    object_id = tracked["object_id"]
    git_hash = hashlib.sha1(b"blob " + str(len(source)).encode() + b"\0" + source).hexdigest()
    if git_hash != object_id:
        raise ValueError("source Git blob differs from captured screen blob")
    return source


def build_manifest(screen_dir, subject_root, grammar_path, build_receipt_path):
    baseline_public, baseline_public_sha = _load_json(PUBLIC_SCREEN)
    public, public_sha = _load_json(screen_dir / "summary.json")
    verify_screen_summary(public, baseline_public)
    committed_builds, _ = _load_json(PUBLIC_BUILDS)
    build_receipt, build_receipt_sha = _load_json(build_receipt_path)
    private_path = screen_dir / "private/evidence.json"
    private, private_sha = _load_json(private_path)
    configuration = private["configuration"]
    instrument = public["instrument"]
    policy, policy_sha = _load_json(POOL_RULE)
    if policy_sha != instrument["pool_rule_sha256"]:
        raise ValueError("fresh screen pool rule differs from frozen policy")
    verify_subject_pins(configuration, policy)
    if (configuration["policy_sha256"] != instrument["pool_rule_sha256"]
            or configuration["source_records_sha256"] != instrument["source_records_sha256"]
            or sha256(json.dumps(configuration, sort_keys=True, separators=(",", ":"))
                      .encode("utf-8")) != instrument["configuration_sha256"]):
        raise ValueError("private screen policy differs from committed public screen")
    public_grammar = next(g for g in public["primary_totals"]["grammars"]
                          if g["id"] == GRAMMAR_ID)
    grammar_sha = sha256(grammar_path.read_bytes())
    if grammar_sha != public_grammar["library_sha256"]:
        raise ValueError("native grammar differs from fresh screen")
    private_grammar = next(g for g in configuration["grammars"] if g["id"] == GRAMMAR_ID)
    if private_grammar != {"id": GRAMMAR_ID, "symbol": GRAMMAR_SYMBOL, "sha256": grammar_sha}:
        raise ValueError("private screen grammar differs from supplied library")
    for subject in public["primary"]:
        row = next(g for g in subject["grammars"] if g["id"] == GRAMMAR_ID)
        if row["library_sha256"] != grammar_sha:
            raise ValueError("subject screen grammar differs from supplied library")
    parser_c_sha = verify_build_receipt(build_receipt, committed_builds, grammar_sha)
    parser = _load_grammar(grammar_path)
    public_subjects = {s["id"]: s for s in public["primary"]}
    frozen_pins = {s["id"]: s["pin"] for s in configuration["subjects"]}
    rows = []
    for subject in private["subjects"]:
        subject_id = subject["id"]
        if subject_id == "hashcat":
            continue
        if subject_id not in public_subjects or subject["pin"] != frozen_pins[subject_id]:
            raise ValueError("private subject differs from committed public screen")
        grammar = next(g for g in subject["grammars"] if g["id"] == GRAMMAR_ID)
        tracked = {r["path"]: r for r in subject["tracked_rows_before"]}
        failures = [r for r in grammar["parse_rows"] if r["error"] or r["missing"]]
        public_count = next(g["error_or_missing_count"] for g in public_subjects[subject_id]["grammars"]
                            if g["id"] == GRAMMAR_ID)
        if len(failures) != public_count:
            raise ValueError("private residual count differs from committed public screen")
        for failure in failures:
            path = failure["path"]
            source = _subject_source(subject_root / subject_id, subject["pin"], path,
                                     tracked[path])
            tree = parser.parse(source)
            if tree is None or tree.root_node is None:
                raise ValueError("parser returned no tree")
            first = first_diagnostic(tree.root_node)
            rows.append({"subject": subject_id, "path": path,
                         "class": classify(subject_id, path), "first": first,
                         "git_blob": tracked[path]["object_id"],
                         "source_sha256": tracked[path]["content_sha256"],
                         "subject_pin": subject["pin"]})
            reset = getattr(parser, "reset", None)
            if reset is not None:
                reset()
    rows.sort(key=lambda r: (r["subject"], r["path"]))
    if len(rows) != public_grammar["error_or_missing_count"] or len(rows) != 63:
        raise ValueError("residual manifest count differs from committed public screen")
    baseline_labels, _ = _load_json(BASELINE_LABELS)
    verify_residual_subset(rows, baseline_labels)
    counts = dict(sorted(Counter(row["class"] for row in rows).items()))
    return {"schema": "opencl-c-residual-subset-v1", "candidate": GRAMMAR_ID,
            "reproduce": REPRODUCE,
            "label_basis": "First observed syntax site; labels do not claim that macro expansion is required.",
            "pool_rule_sha256": instrument["pool_rule_sha256"],
            "baseline_public_screen_sha256": baseline_public_sha,
            "public_screen_sha256": public_sha, "private_screen_sha256": private_sha,
            "native_grammar_sha256": grammar_sha,
            "build_receipt_sha256": build_receipt_sha,
            "parser_c_sha256": parser_c_sha, "count": len(rows),
            "class_counts": counts, "rows": rows}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--screen-dir", required=True, type=Path)
    parser.add_argument("--subjects-root", required=True, type=Path)
    parser.add_argument("--grammar", required=True, type=Path)
    parser.add_argument("--build-receipt", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args(argv)
    manifest = build_manifest(args.screen_dir, args.subjects_root, args.grammar,
                              args.build_receipt)
    args.out.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(f"wrote {manifest['count']} residual rows; classes {manifest['class_counts']}")


if __name__ == "__main__":
    main()
