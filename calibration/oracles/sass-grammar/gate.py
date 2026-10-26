#!/usr/bin/env python3
"""Reproduce the frozen native tree-sitter Sass parse gate."""

import argparse
import ctypes
import json
import hashlib
import importlib.metadata
import pathlib
import subprocess
import time

from tree_sitter import Language, Parser


CANDIDATE = "bajrangCoder/tree-sitter-sass"
CANDIDATE_COMMIT = "fb280c41b070657e4ff4d4e5e6eea6cb19efd9b8"
SUBJECTS = {
    "jgthms/bulma": ("3e00a8e6d0d0e566d507328f0185ef84854effba", "", 91),
    "vuetifyjs/vuetify": ("b1cd40459b4a0801c021c0985ccd46fa65b9f579", "packages/vuetify/src/", 128),
}


def subject_arg(value):
    name, sep, root = value.partition("=")
    if not sep or name not in SUBJECTS or not root:
        raise argparse.ArgumentTypeError("expected one of the frozen subjects as name=path")
    return name, pathlib.Path(root).resolve()


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args])


def file_list(root, scope):
    paths = git(root, "ls-files", "-z", "--", "*.sass").decode().split("\0")
    return sorted(p for p in paths if p and p.startswith(scope))


def diagnostics(tree):
    if tree is None:
        return []
    out = []
    stack = [tree.root_node]
    fallback = None
    while stack and len(out) < 5:
        node = stack.pop()
        if node.type == "ERROR" or node.is_missing:
            out.append({"type": node.type, "missing": node.is_missing,
                        "line": node.start_point.row + 1,
                        "end_line": node.end_point.row + 1})
        elif node.has_error:
            if fallback is None or (node.start_byte == node.end_byte and fallback.start_byte != fallback.end_byte):
                fallback = node
        stack.extend(reversed(node.children))
    # Some grammar failures are only a zero-width has_error node, without a
    # separate ERROR/missing descendant. Keep a location in that case.
    if not out and fallback is not None:
        out.append({"type": "HAS_ERROR", "missing": False,
                    "line": fallback.start_point.row + 1,
                    "end_line": fallback.end_point.row + 1,
                    "zero_width": fallback.start_byte == fallback.end_byte})
    return out


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--library", required=True, type=pathlib.Path)
    ap.add_argument("--subject", action="append", required=True, type=subject_arg)
    ap.add_argument("--out", required=True, type=pathlib.Path)
    args = ap.parse_args()
    roots = dict(args.subject)
    if len(args.subject) != len(SUBJECTS) or set(roots) != set(SUBJECTS):
        ap.error("supply each of the two frozen subjects exactly once")

    version = importlib.metadata.version("tree-sitter")
    if version != "0.25.2":
        raise ValueError(f"tree-sitter {version}; expected 0.25.2")
    lib = ctypes.CDLL(str(args.library.resolve()))
    lib.tree_sitter_sass.restype = ctypes.c_void_p
    capsule = ctypes.pythonapi.PyCapsule_New
    capsule.restype = ctypes.py_object
    capsule.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
    language = Language(capsule(lib.tree_sitter_sass(), b"tree_sitter.Language", None))
    parser = Parser(language)
    parser.timeout_micros = 2_000_000
    out = {"candidate": CANDIDATE, "commit": CANDIDATE_COMMIT,
           "licence": "MIT", "tree_sitter": version, "abi": language.abi_version,
           "library_sha256": hashlib.sha256(args.library.read_bytes()).hexdigest(),
           "timeout_ms": 2000, "subjects": []}

    for name in SUBJECTS:
        sha, scope, expected_files = SUBJECTS[name]
        root = roots[name]
        actual = git(root, "rev-parse", "HEAD").decode().strip()
        if actual != sha:
            raise ValueError(f"{name}: commit {actual}; expected {sha}")
        if git(root, "status", "--porcelain").strip():
            raise ValueError(f"{name}: dirty subject checkout")
        files = file_list(root, scope)
        if len(files) != expected_files:
            raise ValueError(f"{name}: {len(files)} Sass files; expected {expected_files}")
        errors = []
        byte_count = 0
        max_ms = 0.0
        for rel in files:
            src = (root / rel).read_bytes()
            byte_count += len(src)
            start = time.monotonic()
            failure = None
            try:
                tree = parser.parse(src)
                failed = tree is None or tree.root_node.has_error
            except (ValueError, RuntimeError) as exc:
                tree = None
                failed = True
                failure = str(exc)
            elapsed = (time.monotonic() - start) * 1000
            max_ms = max(max_ms, elapsed)
            if failed:
                entry = {"path": rel, "diagnostics": diagnostics(tree),
                         "timeout": tree is None}
                if failure is not None:
                    entry["failure"] = failure
                errors.append(entry)
            parser.reset()
        out["subjects"].append({"name": name, "sha": sha, "scope": scope or ".",
                                "files": len(files), "bytes": byte_count,
                                "files_in_error": len(errors),
                                "error_rate": len(errors) / len(files),
                                "max_parse_ms": round(max_ms, 3), "errors": errors})

    out["files"] = sum(s["files"] for s in out["subjects"])
    out["files_in_error"] = sum(s["files_in_error"] for s in out["subjects"])
    out["error_rate"] = out["files_in_error"] / out["files"]
    out["gate"] = "PASS" if out["error_rate"] <= 0.05 else "REFUSED"
    out["scope_note"] = "Native candidate parse gate only, no production WASM or support level claimed."
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(out, indent=2) + "\n")
    print(json.dumps({k: v for k, v in out.items() if k != "subjects"}))
    print([(s["name"], s["files"], s["files_in_error"]) for s in out["subjects"]])


if __name__ == "__main__":
    main()
