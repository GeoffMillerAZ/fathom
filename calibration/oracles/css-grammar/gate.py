#!/usr/bin/env python3
"""Read-only native tree-sitter CSS parse census over frozen oracle source manifests."""

import argparse
import ctypes
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path, PurePosixPath
import subprocess
import time

from tree_sitter import Language, Parser


TIMEOUT_MICROS = 2_000_000


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def canonical_json(value):
    return json.dumps(value, separators=(",", ":"), ensure_ascii=False).encode("utf-8")


def subject_arg(value):
    pieces = value.split("=", 2)
    if len(pieces) != 3 or not all(pieces):
        raise argparse.ArgumentTypeError("expected NAME=ROOT=SOURCE_SIDECAR")
    name, root, sidecar = pieces
    return name, Path(root).resolve(), Path(sidecar).resolve()


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], text=True).strip()


def css_files(root, rel):
    target = root / rel
    if target.is_file():
        return [rel] if rel.endswith(".css") else []
    found = []
    for entry in sorted(target.iterdir(), key=lambda p: p.name):
        if entry.name in (".git", "node_modules"):
            continue
        child = f"{rel}/{entry.name}" if rel else entry.name
        if entry.is_symlink():
            continue
        if entry.is_dir():
            found.extend(css_files(root, child))
        elif entry.is_file() and entry.name.endswith(".css"):
            found.append(child)
    return sorted(found)


def manifest_scope(paths):
    """The deepest directory common to every listed file, or the file itself."""
    if not paths:
        return ""
    if len(paths) == 1:
        return paths[0]
    common = os.path.commonpath(paths)
    return "" if common == "." else common


def check_manifest(root, sidecar):
    meta = json.loads(sidecar.read_text())
    manifest = meta.get("source_manifest")
    if not isinstance(manifest, dict):
        raise ValueError("source sidecar has no source_manifest")
    commit, files, digest = manifest.get("commit"), manifest.get("files"), manifest.get("digest")
    if not isinstance(commit, str) or len(commit) != 40 or not isinstance(files, list) or not isinstance(digest, str):
        raise ValueError("invalid source manifest identity")
    if digest != sha256(canonical_json({"commit": commit, "files": files})):
        raise ValueError("source manifest digest mismatch")
    if Path(git(root, "rev-parse", "--show-toplevel")).resolve() != root:
        raise ValueError("subject must be Git repository root")
    if git(root, "rev-parse", "HEAD") != commit or git(root, "status", "--porcelain", "--untracked-files=all"):
        raise ValueError("subject Git HEAD differs or tree is dirty")
    paths = []
    for entry in files:
        if not isinstance(entry, dict) or set(entry) != {"path", "sha256"}:
            raise ValueError("invalid source manifest file entry")
        rel, expected = entry["path"], entry["sha256"]
        if not isinstance(rel, str) or not rel.endswith(".css") or "\\" in rel or rel.startswith("/") or \
           str(PurePosixPath(rel)) != rel or any(part in ("", ".", "..") for part in rel.split("/")):
            raise ValueError("noncanonical CSS source path")
        if not isinstance(expected, str) or len(expected) != 64:
            raise ValueError("invalid CSS source hash")
        actual = root / rel
        if actual.is_symlink() or not actual.is_file() or not actual.resolve().is_relative_to(root):
            raise ValueError(f"CSS source is not a regular in-tree file: {rel}")
        if sha256(actual.read_bytes()) != expected:
            raise ValueError(f"CSS source bytes differ: {rel}")
        paths.append(rel)
    if not paths or paths != sorted(set(paths)):
        raise ValueError("source manifest files must be nonempty, unique and sorted")
    scope = manifest_scope(paths)
    if css_files(root, scope) != paths:
        raise ValueError("source manifest does not cover complete CSS list in its scope")
    if git(root, "rev-parse", "HEAD") != commit or git(root, "status", "--porcelain", "--untracked-files=all"):
        raise ValueError("subject Git identity changed during source check")
    return {"commit": commit, "digest": digest, "files": files, "scope": scope or "."}


def diagnostics(root, rel, limit=3):
    out = []
    stack = [root]
    fallback = None
    while stack and len(out) < limit:
        node = stack.pop()
        if node.type == "ERROR" or node.is_missing:
            out.append({"path": rel, "type": node.type, "missing": bool(node.is_missing),
                        "line": node.start_point.row + 1, "byte_column": node.start_point.column})
        elif node.has_error and fallback is None:
            fallback = node
        stack.extend(reversed(node.children))
    if not out and fallback is not None:
        out.append({"path": rel, "type": "HAS_ERROR", "missing": False,
                    "line": fallback.start_point.row + 1, "byte_column": fallback.start_point.column,
                    "zero_width": fallback.start_byte == fallback.end_byte})
    return out


def parse_one(parser, data):
    start = time.monotonic()
    try:
        tree = parser.parse(data)
        failed = tree is None or bool(tree.root_node.has_error)
        return failed, tree, round((time.monotonic() - start) * 1000, 3)
    finally:
        parser.reset()


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--library", type=Path, required=True)
    ap.add_argument("--symbol", default="tree_sitter_css")
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--subject", action="append", type=subject_arg, required=True)
    args = ap.parse_args()
    if not args.symbol.isidentifier():
        ap.error("--symbol must be a C identifier")
    library_bytes = args.library.read_bytes()
    lib = ctypes.CDLL(str(args.library.resolve()))
    symbol = getattr(lib, args.symbol)
    symbol.restype = ctypes.c_void_p
    capsule = ctypes.pythonapi.PyCapsule_New
    capsule.restype = ctypes.py_object
    capsule.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
    language = Language(capsule(symbol(), b"tree_sitter.Language", None))
    parser = Parser(language)
    if not hasattr(parser, "timeout_micros"):
        raise RuntimeError("pinned tree-sitter runtime lacks required 2 s parser timeout")
    parser.timeout_micros = TIMEOUT_MICROS
    controls = []
    for label, data, expected_error in (
        ("well_formed", b".x { color: red; }", False),
        ("unclosed_block", b".x { color: red;", True),
    ):
        failed, tree, ms = parse_one(parser, data)
        if tree is None or failed != expected_error:
            raise RuntimeError(f"grammar control {label} did not produce expected parse status")
        controls.append({"name": label, "has_error": failed, "parse_ms": ms,
                         "diagnostics": diagnostics(tree.root_node, "<control>") if failed and tree else []})
    seen = set()
    subjects = []
    for name, root, sidecar in args.subject:
        if "tachyons" in name.lower() or "tachyons" in root.name.lower():
            raise ValueError("Tachyons holdout is excluded from candidate census")
        if name in seen:
            raise ValueError(f"duplicate subject name: {name}")
        seen.add(name)
        manifest = check_manifest(root, sidecar)
        raw_errors = eligible_errors = raw_bytes = timeouts = 0
        examples = []
        file_results = []
        eligible_total = 0
        max_ms = 0.0
        for entry in manifest["files"]:
            rel = entry["path"]
            data = (root / rel).read_bytes()
            if sha256(data) != entry["sha256"]:
                raise ValueError(f"CSS source changed before parse: {rel}")
            raw_bytes += len(data)
            eligible = not rel.endswith(".min.css")
            eligible_total += eligible
            failed, tree, ms = parse_one(parser, data)
            max_ms = max(max_ms, ms)
            file_results.append({"path": rel, "has_error": failed, "timeout": tree is None,
                                 "production_eligible": eligible, "parse_ms": ms})
            if failed:
                raw_errors += 1
                eligible_errors += eligible
                timeouts += tree is None
                if tree is not None and len(examples) < 3:
                    examples.extend(diagnostics(tree.root_node, rel, 3 - len(examples)))
            if sha256((root / rel).read_bytes()) != entry["sha256"]:
                raise ValueError(f"CSS source changed after parse: {rel}")
        check_manifest(root, sidecar)
        subjects.append({"name": name, "source_commit": manifest["commit"],
                         "source_digest": manifest["digest"], "scope": manifest["scope"],
                         "raw": {"files": len(manifest["files"]), "bytes": raw_bytes, "files_in_error": raw_errors,
                                 "error_rate": raw_errors / len(manifest["files"]), "timeouts": timeouts},
                         "production_eligible": {"files": eligible_total, "files_in_error": eligible_errors,
                                                 "error_rate": eligible_errors / eligible_total if eligible_total else None,
                                                 "exclusion": "*.min.css only"},
                         "max_parse_ms": max_ms, "first_diagnostics": examples,
                         "files": file_results})
    output = {"tool": "native tree-sitter CSS grammar census", "library_sha256": sha256(library_bytes),
              "symbol": args.symbol, "tree_sitter_runtime": importlib.metadata.version("tree-sitter"),
              "abi": language.abi_version, "timeout_ms": TIMEOUT_MICROS // 1000,
              "controls": controls, "subjects": subjects,
              "scope_note": "Read-only grammar parse census; no Fathom output or language-grade claim."}
    if args.out.exists():
        raise ValueError("output already exists")
    temp = args.out.with_name(args.out.name + f".tmp-{os.getpid()}")
    try:
        temp.write_text(json.dumps(output, indent=2) + "\n")
        temp.rename(args.out)
    finally:
        temp.unlink(missing_ok=True)
    print(json.dumps({"library_sha256": output["library_sha256"],
                      "subjects": [{"name": s["name"], "raw": s["raw"],
                                    "production_eligible": s["production_eligible"]} for s in subjects]}))


if __name__ == "__main__":
    main()
