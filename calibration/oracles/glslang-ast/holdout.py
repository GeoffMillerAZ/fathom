#!/usr/bin/env python3
"""Independent, pinned GLSL holdout. Run only with a CPU lease.

The product executable is built once from one clean source commit. Three oracle
files differ only in exclusion catalogs; the AST truth and shader population do
not change. Detailed receipts live under workspace/, never in a reading shard.
"""
import argparse
from collections import Counter
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import sys

sys.dont_write_bytecode = True
_spec = importlib.util.spec_from_file_location("glslang_ast", Path(__file__).with_name("glslang_ast.py"))
ast = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(ast)

STAGE_EXTENSIONS = frozenset(".vert .frag .comp .geom .tesc .tese .rgen .rchit .rmiss .rahit .rint .rcall .mesh .task .vsh .fsh".split())
EXTENSIONS = STAGE_EXTENSIONS | {".glsl"}
STAGES = {".vsh": "vert", ".fsh": "frag", ".fs": "frag"}
NAME = re.compile(r"[A-Za-z_]\w*\Z")
MAX_UNPACKED_BYTES = 1024 * 1024 * 1024


def digest(path):
    sha = hashlib.sha256()
    with Path(path).open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            sha.update(block)
    return sha.hexdigest()


def extract_safe(archive, destination):
    """Reject links, traversal, and unusually large payloads before extraction."""
    destination = Path(destination)
    destination.mkdir(parents=True, exist_ok=False)
    total = 0
    with tarfile.open(archive, "r:gz") as source:
        for member in source:
            path = Path(member.name)
            if (path.is_absolute() or ".." in path.parts or not path.parts or
                    not (member.isfile() or member.isdir())):
                raise ValueError("unsafe archive entry")
            total += member.size
            if total > MAX_UNPACKED_BYTES:
                raise ValueError("archive exceeds unpacked-byte limit")
            target = destination / path
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
            else:
                target.parent.mkdir(parents=True, exist_ok=True)
                with source.extractfile(member) as input_file, target.open("xb") as output_file:
                    shutil.copyfileobj(input_file, output_file)
    roots = list(destination.iterdir())
    if len(roots) != 1 or not roots[0].is_dir():
        raise ValueError("archive must contain one root directory")
    return roots[0]


def population(root, dialect=None):
    """All GLSL extension files plus .fs files satisfying the dialect sniff."""
    if dialect is None:
        dialect = Path("internal/extractor/dialects/glsl/dialect.json")
    match = json.loads(Path(dialect).read_text())["match"]
    if set(match["extensions"]) != EXTENSIONS or match["sniff"]["extensions"] != [".fs"]:
        raise ValueError("GLSL extension map changed; review holdout population")
    # The connector prepends (?m), so a comment before #version still matches.
    sniff = re.compile(match["sniff"]["pattern"].encode(), re.MULTILINE)
    limit = match["sniff"]["scan_bytes"]
    rows, rejected_fs = [], 0
    for path in sorted(Path(root).rglob("*")):
        if not path.is_file() or path.is_symlink():
            continue
        extension = path.suffix
        if extension == ".fs":
            with path.open("rb") as source:
                head = source.read(limit)
            if not sniff.search(head):
                rejected_fs += 1
                continue
        elif extension not in EXTENSIONS:
            continue
        rows.append({"path": path.relative_to(root).as_posix(), "extension": extension,
                     "sha256": digest(path), "bytes": path.stat().st_size})
    return rows, rejected_fs


def manifest_hash(files):
    return hashlib.sha256(json.dumps(files, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def verify_manifest(root, files):
    for item in files:
        path = Path(root) / item["path"]
        if not path.is_file() or path.is_symlink() or digest(path) != item["sha256"]:
            raise ValueError("holdout source bytes changed: " + item["path"])


def legacy_constructor_names():
    """Use the same original G2 exclusions as the canonical paired adapter."""
    return ast.legacy_constructor_names()


def legacy_constructor_hash():
    data = json.dumps(sorted(legacy_constructor_names()), separators=(",", ":")).encode()
    return hashlib.sha256(data).hexdigest()


def load_catalog(path):
    data = json.loads(Path(path).read_text())
    functions, types = data.get("functions"), data.get("types", [])
    if (not isinstance(functions, list) or not isinstance(types, list) or
            any(not isinstance(name, str) or not NAME.fullmatch(name) for name in functions + types) or
            len(functions) != len(set(functions)) or len(types) != len(set(types))):
        raise ValueError("invalid exclusion catalog")
    return set(functions), set(types), digest(path)


def load_registry(path, pin):
    data = json.loads(Path(path).read_text())
    if data.get("complete") is not True:
        raise ValueError("full extension registry is not proven complete")
    accepted_hash = pin.get("extension_catalog_sha256")
    accepted_sources = pin.get("accepted_extension_sources")
    if (not isinstance(accepted_hash, str) or not re.fullmatch(r"[0-9a-f]{64}", accepted_hash) or
            digest(path) != accepted_hash or not isinstance(accepted_sources, list) or not accepted_sources):
        raise ValueError("full extension registry has no independently accepted pin")
    sources = data.get("sources")
    if sources != accepted_sources or any(
            not isinstance(source, dict) or not isinstance(source.get("repository"), str) or
            not isinstance(source.get("revision"), str) or
            not re.fullmatch(r"[0-9a-f]{64}", str(source.get("archive_sha256", ""))) or
            not isinstance(source.get("source_count"), int) or source["source_count"] <= 0
            for source in sources):
        raise ValueError("extension source revisions or archive hashes differ from accepted pin")
    audit = data.get("audit")
    if (not isinstance(audit, dict) or audit.get("unresolved_candidates") != 0 or
            audit.get("challenger_gaps") != 0):
        raise ValueError("extension catalog has unresolved completeness evidence")
    functions, types, sha = load_catalog(path)
    maps = []
    for key, expected in (("functions_by_extension", functions), ("types_by_extension", types)):
        raw = data.get(key)
        if not isinstance(raw, dict):
            raise ValueError("extension registry lacks " + key)
        entries = {}
        for extension, names in raw.items():
            if (not re.fullmatch(r"GL_[A-Za-z0-9_]+", extension) or not isinstance(names, list) or
                    any(not isinstance(name, str) or not NAME.fullmatch(name) for name in names)):
                raise ValueError("invalid extension registry entry")
            entries[extension] = set(names)
        if set().union(*entries.values()) != expected:
            raise ValueError("extension registry union differs from per-extension map")
        maps.append(entries)
    return functions, types, sha


def make_oracle(rows, functions, types, diagnostic=False):
    """AST truth is user-defined calls; exclusions only change per mode."""
    if not rows:
        raise ValueError("empty population")
    scope = [row["path"] for row in rows if row["status"] == "parsed"]
    fraction = len(scope) / len(rows)
    if fraction < 0.95 and not diagnostic:
        raise ValueError(f"oracle coverage below 95%: {len(scope)}/{len(rows)}")
    if not scope:
        raise ValueError("empty holdout oracle scope")
    calls, exclusions = set(), set()
    for row in rows:
        if row["status"] != "parsed":
            continue
        definitions = set(row["definitions"])
        shadow = definitions & (functions | types)
        if shadow:
            raise ValueError("catalog name shadowed by user definition: " + ", ".join(sorted(shadow)))
        for caller, callee in row["sites"]:
            if callee in definitions:
                calls.add((row["path"], caller, callee))
        exclusions.update((row["path"], name, "pinned catalog function") for name in functions)
        exclusions.update((row["path"], name, "pinned catalog type identifier") for name in types - functions)
    oracle = {"tool": "glslangValidator -i", "version": "16.6.0", "tier": "B",
              "definitions": [], "calls": [{"path": p, "from": f, "to": t} for p, f, t in sorted(calls)],
              "call_exclusions": [{"path": p, "to": t, "reason": reason}
                                  for p, t, reason in sorted(exclusions)],
              "scope": sorted(scope)}
    return oracle, {"population": len(rows), "oracle_scope": len(scope),
                    "oracle_scope_fraction": fraction, "user_call_denominator": len(calls),
                    "validation_claimable": fraction >= 0.95, "diagnostic_only": fraction < 0.95,
                    "exclusion_entries": len(exclusions),
                    "gap_statuses": dict(sorted(Counter(row["status"] for row in rows if row["status"] != "parsed").items()))}


def compile_rows(root, files, compiler, receipts):
    """Generic stage inference is reconstructed tier B, never a native recipe."""
    rows = []
    for item in files:
        rel = item["path"]
        suffix = item["extension"]
        stage = STAGES.get(suffix, suffix.removeprefix(".") if suffix in STAGE_EXTENSIONS else None)
        row = {**item, "status": "unknown_stage", "definitions": [], "sites": [],
               "recipe_tier": "B", "recipe_source": "suffix-inferred, not native build"}
        source_text = (root / rel).read_text(errors="replace")
        if re.search(r"^\s*#\s*(?:include|line)\b", source_text, re.MULTILINE):
            row["status"] = "unverified_source_origin"
            rows.append(row)
            continue
        if stage is None:
            rows.append(row)
            continue
        command = [str(compiler), "-i", "-S", stage, str(root / rel)]
        try:
            run = subprocess.run(command, cwd=root, capture_output=True, text=True, errors="replace", timeout=30)
            output = run.stderr if "Function Definition:" in run.stderr else run.stdout
            row["compiler_exit"] = run.returncode
            row["ast_sha256"] = hashlib.sha256(output.encode()).hexdigest()
            if run.returncode:
                row["status"] = "compile_failed"
                row["diagnostic"] = (run.stderr or run.stdout)[:500]
            else:
                definitions, sites = ast.parse_ast(output, rel)
                row.update(status="parsed", definitions=sorted({d["name"] for d in definitions}), sites=sites)
        except (subprocess.TimeoutExpired, ValueError) as exc:
            row["status"] = "parse_failed"
            row["diagnostic"] = str(exc)[:500]
        if digest(root / rel) != item["sha256"]:
            raise ValueError("shader input changed during AST oracle run: " + rel)
        rows.append(row)
    receipts.write_text(json.dumps({"files": rows}, indent=2) + "\n")
    return rows


def checked_source_commit(root, expected):
    actual = subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()
    if actual != expected:
        raise ValueError("product source commit differs from pinned commit")
    dirty = subprocess.check_output(["git", "-C", str(root), "status", "--porcelain", "--untracked-files=all"], text=True)
    if dirty:
        raise ValueError("product source worktree is dirty")


def call_counts(reading, mode):
    counts = reading["counts"]
    required = ("call_truth", "call_truth_found", "call_fathom", "call_fathom_in_truth")
    if any(key not in counts for key in required):
        raise ValueError("neutral reading lacks required call counts: " + mode)
    truth, found, detected, correct = (counts[key] for key in required)
    return {"call_truth": truth, "call_truth_found": found,
            "call_fathom": detected, "call_fathom_in_truth": correct,
            "call_missed": truth - found, "call_extra": detected - correct,
            "call_recall": found / truth if truth else None,
            "call_precision": correct / detected if detected else None}


def tenancy_paths(unmatched, expected, scope):
    paths = unmatched.get("tenancy_files")
    if (not isinstance(paths, list) or any(not isinstance(path, str) for path in paths) or
            paths != sorted(set(paths))):
        raise ValueError("unmatched receipt lacks sorted exact tenancy_files")
    if paths != sorted(expected):
        raise ValueError("measured GLSL tenancy differs from frozen holdout population")
    if not set(scope) <= set(paths):
        raise ValueError("oracle scope includes a file outside measured tenancy")
    return len(paths)


def oracle_truth_hash(oracle):
    """Hash only invariant truth: normalized calls and scope, never exclusions."""
    scope = oracle.get("scope")
    calls = oracle.get("calls")
    if (not isinstance(scope, list) or any(not isinstance(path, str) for path in scope) or
            not isinstance(calls, list)):
        raise ValueError("invalid holdout oracle truth")
    normalized_scope = sorted(set(scope))
    if len(normalized_scope) != len(scope):
        raise ValueError("duplicate holdout oracle scope")
    normalized_calls = []
    for call in calls:
        if not isinstance(call, dict) or any(not isinstance(call.get(key), str)
                                             for key in ("path", "from", "to")):
            raise ValueError("invalid holdout oracle call")
        normalized_calls.append((call["path"], call["from"], call["to"]))
    normalized_calls.sort()
    if len(normalized_calls) != len(set(normalized_calls)) or any(
            path not in set(normalized_scope) for path, _, _ in normalized_calls):
        raise ValueError("duplicate or out-of-scope holdout oracle call")
    payload = {"scope": normalized_scope, "calls": normalized_calls}
    return hashlib.sha256(json.dumps(payload, separators=(",", ":")).encode()).hexdigest()


def require_exact_pairing(scores):
    required_modes = {"baseline-core", "core-plus-types", "full-registry"}
    if set(scores) != required_modes:
        raise ValueError("paired holdout lacks all three catalog modes")
    invariant = ("call_truth", "call_truth_found", "call_fathom_in_truth",
                 "oracle_truth_sha256", "subject_sha", "manifest_sha256",
                 "exact_tenancy_files")
    for key in invariant:
        if len({row.get(key) for row in scores.values()}) != 1:
            raise ValueError("paired holdout invariant changed: " + key)
    return True


def catalog_fingerprint(mode, functions, types):
    return ast._catalog_fingerprint(mode, functions, types)


def score_modes(args, subject, modes, out):
    """One built product binary, calls-only oracle, three catalogs."""
    binary = out / "indexbench"
    environment = dict(os.environ, GOWORK="off")
    subprocess.run(["go", "build", "-o", str(binary.resolve()), "./cmd/indexbench"],
                   cwd=args.product_source, env=environment, check=True)
    product_binary_sha = digest(binary)
    scores = {}
    for mode in modes:
        reading = out / (mode + "-reading.json")
        unmatched = out / (mode + "-unmatched.json")
        oracle = out / (mode + "-oracle.json")
        command = [str(binary.resolve()), "lang-oracle", "-dialect", "glsl",
                   "-subject", str(subject.resolve()), "-subject-sha", args.pin_commit,
                   "-subject-name", "g-truc/ogl-samples holdout", "-commit", args.product_commit,
                   "-ctags", str((out / "ctags-disabled").resolve()),
                   "-tool-oracle", str(oracle.resolve()), "-unmatched", str(unmatched.resolve()),
                   "-out", str(reading.resolve())]
        subprocess.run(command, cwd=args.product_source, env=environment, check=True)
        data = json.loads(reading.read_text())
        if modes[mode]["diagnostic_only"]:
            data.pop("grade", None)
            data.pop("level_met", None)
            data.setdefault("notes", []).append("Holdout oracle coverage below 95%; diagnostic counts only, no grade or validation claim.")
            reading.write_text(json.dumps(data, indent=2) + "\n")
        oracle_data = json.loads(oracle.read_text())
        subjects = data["subjects"]
        if len(subjects) != 1 or subjects[0]["sha"] != args.pin_commit:
            raise ValueError("neutral reading measured a different subject")
        if "l1" in subjects[0]["families"] or subjects[0]["families"].get("calls") != "B":
            raise ValueError("holdout must carry tier-B L2 calls and no L1 claim")
        tenant = subjects[0]["tenancy"]
        exact_files = tenancy_paths(json.loads(unmatched.read_text()), args.manifest_paths,
                                    oracle_data["scope"])
        scores[mode] = {**call_counts(data, mode), "reading_sha256": digest(reading),
                        "unmatched_sha256": digest(unmatched),
                        "oracle_truth_sha256": oracle_truth_hash(oracle_data),
                        "subject_sha": args.pin_commit, "manifest_sha256": args.manifest_sha256,
                        "tenant_files": subjects[0]["files"], "exact_tenancy_files": exact_files,
                        "tenant_faults": tenant["faults"]}
    return product_binary_sha, scores


def under_workspace(path):
    workspace = Path("workspace").resolve()
    if not Path(path).resolve().is_relative_to(workspace):
        raise ValueError("holdout artifacts must stay under workspace/")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pin", type=Path, default=Path(__file__).with_name("holdout-pin.json"))
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--product-source", type=Path, required=True)
    parser.add_argument("--product-commit", required=True)
    parser.add_argument("--compiler", type=Path, required=True)
    parser.add_argument("--baseline-catalog", type=Path, required=True)
    parser.add_argument("--core-catalog", type=Path, required=True)
    parser.add_argument("--extension-catalog", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--diagnostic-below-coverage", action="store_true",
                        help="emit ungraded paired counts when coverage fails; never authorizes validation")
    args = parser.parse_args()
    args.pin = args.pin.resolve()
    args.archive = args.archive.resolve()
    args.out = args.out.resolve()
    args.baseline_catalog = args.baseline_catalog.resolve()
    args.core_catalog = args.core_catalog.resolve()
    args.extension_catalog = args.extension_catalog.resolve()
    args.compiler = args.compiler.resolve()
    args.product_source = args.product_source.resolve()
    for path in (args.archive, args.out):
        under_workspace(path)
    pin = json.loads(args.pin.read_text())
    committed_pin = json.loads(Path(__file__).with_name("holdout-pin.json").read_text())
    for field in ("commit_sha", "archive_sha256", "archive_bytes"):
        if pin.get(field) != committed_pin[field]:
            raise ValueError("private holdout pin differs from committed metadata: " + field)
    if digest(args.archive) != pin["archive_sha256"] or not re.fullmatch(r"[0-9a-f]{40}", pin["commit_sha"]):
        raise ValueError("holdout archive pin mismatch")
    if args.archive.stat().st_size != pin["archive_bytes"]:
        raise ValueError("holdout archive byte count mismatch")
    checked_source_commit(args.product_source, args.product_commit)
    ast.compiler_identity(args.compiler)
    baseline_functions, baseline_types, baseline_hash = load_catalog(args.baseline_catalog)
    core_functions, core_types, core_hash = load_catalog(args.core_catalog)
    if (baseline_hash != committed_pin["baseline_catalog_sha256"] or
            core_hash != committed_pin["core_catalog_sha256"]):
        raise ValueError("core or baseline catalog pin mismatch")
    pinned_functions, _ = ast.builtin_index(args.core_catalog)
    if (core_functions != pinned_functions or core_types != ast.core_constructor_names() or
            baseline_functions != core_functions or baseline_types or
            not legacy_constructor_names() <= core_types or
            legacy_constructor_hash() != committed_pin["baseline_constructors_sha256"]):
        raise ValueError("core GLSL 4.60 catalog differs from pinned oracle")
    extension_functions, extension_types, extension_hash = load_registry(args.extension_catalog, committed_pin)
    args.out.mkdir(parents=True, exist_ok=False)
    subject = extract_safe(args.archive, args.out / "subject")
    files, rejected_fs = population(subject, args.product_source / "internal/extractor/dialects/glsl/dialect.json")
    rows = compile_rows(subject, files, args.compiler, args.out / "oracle-receipt.json")
    verify_manifest(subject, files)
    coverage = {"population": len(rows), "parsed": sum(row["status"] == "parsed" for row in rows),
                "gap_statuses": dict(sorted(Counter(row["status"] for row in rows if row["status"] != "parsed").items())),
                "manifest_sha256": manifest_hash(files)}
    (args.out / "coverage.json").write_text(json.dumps(coverage, indent=2) + "\n")
    results = {}
    for mode, functions, types in (("baseline-core", baseline_functions, legacy_constructor_names()),
                                   ("core-plus-types", core_functions, core_types),
                                   ("full-registry", core_functions | extension_functions,
                                    core_types | extension_types)):
        oracle, summary = make_oracle(rows, functions, types, diagnostic=args.diagnostic_below_coverage)
        (args.out / f"{mode}-oracle.json").write_text(json.dumps(oracle, indent=2) + "\n")
        results[mode] = {**summary, "catalog_mode": mode,
                         "catalog_fingerprint_sha256": catalog_fingerprint(mode, functions, types)}
    args.pin_commit = pin["commit_sha"]
    args.manifest_paths = [item["path"] for item in files]
    args.manifest_sha256 = manifest_hash(files)
    product_binary_sha, scores = score_modes(args, subject, results, args.out)
    verify_manifest(subject, files)
    for mode, score in scores.items():
        results[mode]["score"] = score
        results[mode]["complete_tenancy"] = (score["call_truth"] == results[mode]["user_call_denominator"]
                                            and score["tenant_files"] == len(files) and score["tenant_faults"] == 0)
    if not all(row["complete_tenancy"] for row in results.values()):
        raise ValueError("paired holdout has incomplete tenancy")
    paired = require_exact_pairing(scores)
    deltas = {}
    for mode in ("core-plus-types", "full-registry"):
        base = scores["baseline-core"]
        after = scores[mode]
        deltas[mode + "_vs_baseline"] = {key: after[key] - base[key]
                                         for key in ("call_truth_found", "call_fathom", "call_missed", "call_extra")}
    report = {"holdout_commit": pin["commit_sha"], "archive_sha256": pin["archive_sha256"],
              "product_commit": args.product_commit, "product_binary_sha256": product_binary_sha,
              "compiler_sha256": digest(args.compiler),
              "baseline_catalog_sha256": baseline_hash, "core_catalog_sha256": core_hash,
              "baseline_constructors_sha256": legacy_constructor_hash(),
              "extension_catalog_sha256": extension_hash,
              "manifest_sha256": manifest_hash(files), "population_by_extension": dict(sorted(Counter(f["extension"] for f in files).items())),
              "sniff_rejected_fs": rejected_fs, "paired_comparison_claimable": paired,
              "validation_claimable": all(row["validation_claimable"] for row in results.values()),
              "diagnostic_only": any(row["diagnostic_only"] for row in results.values()),
              "deltas": deltas, "modes": results,
              "disclosure": "All glslang invocations use suffix-inferred stages without native build flags; tier B at best."}
    checked_source_commit(args.product_source, args.product_commit)
    (args.out / "summary.json").write_text(json.dumps(report, indent=2) + "\n")


if __name__ == "__main__":
    main()
