#!/usr/bin/env python3
"""Native Blade grammar screen. Receipts contain aggregates, never corpus paths or text."""

import argparse
import collections
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys


PINNED = {
    "breeze": ("47f6b30a3bdf376a3f75cf9ad8e68ea3941048e2", 57),
    "filament": ("85d51ef9e5c4e3db281c568315f535652639e09f", 197),
}
CLI_VERSION = "tree-sitter 0.26.11"
CONTROL_DIR = Path(__file__).resolve().parent / "testdata"


def command(argv, *, cwd=None, timeout=30):
    return subprocess.run(argv, cwd=cwd, capture_output=True, text=True,
                          timeout=timeout, check=False)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def clean_checkout(root, expected):
    if not (root / ".git").exists():
        raise ValueError(f"{root.name}: missing .git")
    head = command(["git", "rev-parse", "HEAD"], cwd=root)
    if head.returncode or head.stdout.strip() != expected:
        raise ValueError(f"{root.name}: wrong HEAD")
    status = command(["git", "status", "--porcelain=v1", "--untracked-files=all"], cwd=root)
    ignored = command(["git", "ls-files", "--others", "--ignored", "--exclude-standard"], cwd=root)
    if status.returncode or ignored.returncode or status.stdout or ignored.stdout:
        raise ValueError(f"{root.name}: checkout is not clean (including ignored files)")


def corpus(root, expected_count):
    paths = sorted(root.rglob("*.blade.php"))
    if len(paths) != expected_count:
        raise ValueError(f"{root.name}: expected {expected_count} Blade files, got {len(paths)}")
    lines = []
    total_bytes = 0
    for path in paths:
        raw = path.read_bytes()
        total_bytes += len(raw)
        lines.append(f"{path.relative_to(root).as_posix()} {digest(raw)}\n")
    return paths, {"files": len(paths), "bytes": total_bytes,
                   "path_content_set_sha256": digest("".join(lines).encode())}


def grammar_sources(root):
    parser = root / "src/parser.c"
    if not parser.is_file():
        raise ValueError("grammar lacks src/parser.c")
    scanners = [p for p in (root / "src/scanner.c", root / "src/scanner.cc") if p.is_file()]
    if len(scanners) > 1:
        raise ValueError("grammar has both scanner.c and scanner.cc")
    sources = [parser] + scanners
    return sources, {p.name: digest(p.read_bytes()) for p in sources}


def build(root, sources, out):
    objects = []
    for source in sources:
        obj = out / (source.name + ".o")
        compiler = "c++" if source.suffix == ".cc" else "cc"
        result = command([compiler, "-O0", "-fPIC", "-I", str(root / "src"),
                          "-c", str(source), "-o", str(obj)], timeout=300)
        if result.returncode:
            raise RuntimeError(f"native compile failed for {source.name}: {result.stderr[-1000:]}")
        objects.append(obj)
    library = out / ("libblade.dylib" if sys.platform == "darwin" else "libblade.so")
    linker = "c++" if any(p.suffix == ".cc" for p in sources) else "cc"
    shared_flag = "-dynamiclib" if sys.platform == "darwin" else "-shared"
    result = command([linker, shared_flag, *map(str, objects), "-o", str(library)], timeout=300)
    if result.returncode:
        raise RuntimeError(f"native link failed: {result.stderr[-1000:]}")
    return library


def parse(cli, library, path, timeout, quiet=True):
    args = [cli, "parse"]
    if quiet:
        args.append("-q")
    args += ["--lib-path", str(library), "--lang-name", "blade", str(path)]
    return command(args, timeout=timeout)


def screen_files(cli, library, paths, timeout):
    result = collections.Counter()
    for path in paths:
        try:
            run = parse(cli, library, path, timeout)
            if run.returncode == 0:
                result["pass"] += 1
            elif run.returncode == 1:
                # -q suppresses the tree. Re-run only failures to distinguish
                # ERROR/MISSING from CLI/load failures.
                detail = parse(cli, library, path, timeout, quiet=False)
                if "ERROR" in detail.stdout or "MISSING" in detail.stdout:
                    result["error_or_missing"] += 1
                else:
                    result["infrastructure_failure"] += 1
            else:
                result["infrastructure_failure"] += 1
        except subprocess.TimeoutExpired:
            result["timeout"] += 1
    return {key: result[key] for key in
            ("pass", "error_or_missing", "infrastructure_failure", "timeout")}


def controls(cli, library, out, timeout):
    summary = {}
    for path in sorted(CONTROL_DIR.glob("*.blade.php")):
        try:
            run = parse(cli, library, path, timeout, quiet=False)
        except subprocess.TimeoutExpired as exc:
            raise RuntimeError(f"control {path.name} timed out") from exc
        if run.returncode not in (0, 1) or not run.stdout.strip():
            raise RuntimeError(f"control {path.name} did not produce an AST")
        ast = run.stdout
        (out / (path.name + ".ast.txt")).write_text(ast)
        summary[path.name] = {
            "source_sha256": digest(path.read_bytes()),
            "ast_sha256": digest(ast.encode()),
            "exit_code": run.returncode,
            "node_types": dict(sorted(collections.Counter(
                re.findall(r"\(([A-Za-z_][A-Za-z_0-9.-]*)", ast)).items())),
        }
    return summary


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--grammar", type=Path, required=True)
    ap.add_argument("--grammar-commit", required=True)
    ap.add_argument("--breeze", type=Path, default=Path("workspace/calibration/breeze"))
    ap.add_argument("--filament", type=Path, default=Path("workspace/calibration/filament"))
    ap.add_argument("--out", type=Path, required=True, help="directory under workspace/")
    ap.add_argument("--tree-sitter", default="tree-sitter")
    ap.add_argument("--timeout", type=int, default=10, help="per-file seconds")
    args = ap.parse_args()
    out = args.out.resolve()
    workspace = Path("workspace").resolve()
    if out == workspace or workspace not in out.parents or out.exists():
        ap.error("--out must be a new directory below this worktree's workspace/")
    if args.timeout < 1:
        ap.error("--timeout must be positive")
    version = command([args.tree_sitter, "--version"])
    if version.returncode or version.stdout.strip() != CLI_VERSION:
        ap.error(f"requires {CLI_VERSION}")
    subjects = {}
    paths = []
    for name in PINNED:
        root = getattr(args, name).resolve()
        commit, count = PINNED[name]
        clean_checkout(root, commit)
        files, receipt = corpus(root, count)
        subjects[name] = {"commit": commit, **receipt}
        paths.extend(files)
    grammar = args.grammar.resolve()
    clean_checkout(grammar, args.grammar_commit)
    sources, source_hashes = grammar_sources(grammar)
    out.mkdir(parents=True)
    try:
        library = build(grammar, sources, out)
        counts = screen_files(args.tree_sitter, library, paths, args.timeout)
        control_receipt = controls(args.tree_sitter, library, out, args.timeout)
        receipt = {
            "tool": CLI_VERSION, "timeout_seconds_per_file": args.timeout,
            "grammar": {"commit": args.grammar_commit, "sources_sha256": source_hashes,
                        "native_library_sha256": digest(library.read_bytes())},
            "subjects": subjects, "gate": {"total": len(paths), **counts,
                "passes_error_ceiling": counts["error_or_missing"] <= 12
                and counts["infrastructure_failure"] == 0 and counts["timeout"] == 0},
            "controls": control_receipt,
            "structural_review": "manual inspection required; low ERROR rate alone is insufficient",
        }
        (out / "receipt.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
        print(json.dumps(receipt, sort_keys=True))
        return 0 if receipt["gate"]["passes_error_ceiling"] else 1
    except Exception as exc:
        (out / "failure.txt").write_text(str(exc) + "\n")
        raise


if __name__ == "__main__":
    sys.exit(main())
