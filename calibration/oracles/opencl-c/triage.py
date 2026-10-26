#!/usr/bin/env python3
"""Isolated native tree-sitter triage for tracked OpenCL C files.

This records parse diagnostics only. It does not claim fathom product support or a
language level. Subject identities, paths, manifests, and per-file rows stay in
the private output directory.
"""

import argparse
import base64
import ctypes
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys


TREE_SITTER_VERSION = "0.25.2"
FULL_SHA_RE = re.compile(r"[0-9a-f]{40}\Z")
GRAMMAR_ID_RE = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.-]*\Z")
SYMBOL_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]*\Z")


class CheckoutEvidence:
    def __init__(self, root, expected_commit, stage_raw, rows, eligible, eligible_sources,
                 untracked_raw, ignored_raw, untracked_rows, public):
        self.root = root
        self.expected_commit = expected_commit
        self.stage_raw = stage_raw
        self.rows = rows
        self.eligible = eligible
        self.eligible_sources = eligible_sources
        self.untracked_raw = untracked_raw
        self.ignored_raw = ignored_raw
        self.untracked_rows = untracked_rows
        self.public = public


class LoadedGrammar:
    def __init__(self, grammar_id, symbol, path, library, language, parser, sha256):
        self.grammar_id = grammar_id
        self.symbol = symbol
        self.path = path
        self.library = library
        self.language = language
        self.parser = parser
        self.sha256 = sha256


def _sha256(data):
    return hashlib.sha256(data).hexdigest()


def _git_blob_oid(content, staged_oid):
    if not re.fullmatch(r"[0-9a-f]{40}", staged_oid):
        if not re.fullmatch(r"[0-9a-f]{64}", staged_oid):
            raise ValueError("tracked manifest has an invalid object ID")
        algorithm = hashlib.sha256
    else:
        algorithm = hashlib.sha1
    header = b"blob " + str(len(content)).encode("ascii") + b"\0"
    return algorithm(header + content).hexdigest()


def _git(root, *args):
    return subprocess.run(
        ["git", "-C", str(root), *args],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )


def _git_output(root, *args):
    result = _git(root, *args)
    if result.returncode:
        raise ValueError("subject is not a readable Git checkout")
    return result.stdout


def _assert_tracked_clean(root):
    # Gitlink contents are outside this checkout's tracked-file population. The
    # index object ID is frozen below; ordinary tracked files are checked here.
    for args in (("diff", "--quiet", "--ignore-submodules=all", "--"),
                 ("diff", "--cached", "--quiet", "--ignore-submodules=none", "--")):
        result = _git(root, *args)
        if result.returncode == 1:
            raise ValueError("tracked checkout is dirty")
        if result.returncode:
            raise ValueError("could not inspect tracked checkout")


def _read_regular_no_follow(path):
    flags = os.O_RDONLY
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    descriptor = os.open(path, flags)
    try:
        metadata = os.fstat(descriptor)
        if not stat.S_ISREG(metadata.st_mode):
            raise ValueError("tracked regular-file entry is not a regular file")
        chunks = []
        while True:
            chunk = os.read(descriptor, 1024 * 1024)
            if not chunk:
                return b"".join(chunks)
            chunks.append(chunk)
    finally:
        os.close(descriptor)


def _parse_stage(stage_raw):
    entries = []
    for record in stage_raw.split(b"\0"):
        if not record:
            continue
        header, separator, path_raw = record.partition(b"\t")
        fields = header.split(b" ")
        if not separator or len(fields) != 3:
            raise ValueError("malformed git tracked manifest")
        mode_raw, object_raw, stage = fields
        if stage != b"0":
            raise ValueError("unmerged tracked entry")
        entries.append((mode_raw.decode("ascii"), object_raw.decode("ascii"), path_raw))
    return entries


def _path_text(path_raw):
    return os.fsdecode(path_raw)


def _path_b64(path_raw):
    return base64.b64encode(path_raw).decode("ascii")


def _untracked_snapshot(root):
    untracked_raw = _git_output(root, "ls-files", "-z", "--others", "--exclude-standard")
    ignored_raw = _git_output(
        root, "ls-files", "-z", "--others", "--ignored", "--exclude-standard")
    classified = []
    for category, raw in (("untracked", untracked_raw), ("ignored", ignored_raw)):
        for path_raw in raw.split(b"\0"):
            if path_raw:
                classified.append((category, path_raw))
    classified.sort(key=lambda item: (item[1], item[0]))
    digest_input = b"".join(
        category.encode("ascii") + b"\0" + path_raw + b"\0"
        for category, path_raw in classified
    )
    rows = [
        {"category": category, "path": _path_text(path_raw), "path_base64": _path_b64(path_raw)}
        for category, path_raw in classified
    ]
    public = {
        "files": len(classified),
        "unignored_files": sum(category == "untracked" for category, _ in classified),
        "ignored_files": sum(category == "ignored" for category, _ in classified),
        "path_bytes": sum(len(path_raw) for _, path_raw in classified),
        "paths_sha256": _sha256(digest_input),
    }
    return untracked_raw, ignored_raw, rows, public


def capture_checkout(root, expected_commit):
    """Capture a complete tracked manifest and full untracked path identity set."""
    root = Path(root).resolve()
    if not FULL_SHA_RE.fullmatch(expected_commit):
        raise ValueError("expected commit must be a full lowercase 40-character SHA")
    inside = _git_output(root, "rev-parse", "--is-inside-work-tree").strip()
    if inside != b"true":
        raise ValueError("subject is not a Git worktree")
    actual_commit = _git_output(root, "rev-parse", "HEAD").decode("ascii").strip()
    if actual_commit != expected_commit:
        raise ValueError("subject HEAD does not match expected commit")
    _assert_tracked_clean(root)

    stage_raw = _git_output(root, "ls-files", "--stage", "-z")
    entries = _parse_stage(stage_raw)
    rows = []
    eligible = []
    eligible_sources = {}
    regular_files = regular_bytes = eligible_bytes = symlinks = symlink_bytes = gitlinks = 0
    manifest = hashlib.sha256()
    manifest.update(b"git-ls-files-stage\0")
    manifest.update(stage_raw)
    for mode, object_id, path_raw in entries:
        relative = Path(_path_text(path_raw))
        path = root / relative
        if mode in ("100644", "100755"):
            try:
                content = _read_regular_no_follow(path)
            except (FileNotFoundError, OSError) as exc:
                raise ValueError("tracked regular file is missing or changed") from exc
            kind = "regular"
            content_hash = _sha256(content)
            size = len(content)
            if _git_blob_oid(content, object_id) != object_id:
                raise ValueError("tracked content does not match index")
            regular_files += 1
            regular_bytes += size
            if path_raw.endswith(b".cl"):
                eligible.append(path)
                eligible_sources[path] = content
                eligible_bytes += size
        elif mode == "120000":
            try:
                metadata = os.lstat(path)
                if not stat.S_ISLNK(metadata.st_mode):
                    raise ValueError("tracked symlink entry is not a symlink")
                target = os.fsencode(os.readlink(path))
            except (FileNotFoundError, OSError) as exc:
                raise ValueError("tracked symlink is missing or changed") from exc
            kind = "symlink"
            content_hash = _sha256(target)
            size = len(target)
            if _git_blob_oid(target, object_id) != object_id:
                raise ValueError("tracked content does not match index")
            symlinks += 1
            symlink_bytes += size
        elif mode == "160000":
            kind = "gitlink"
            content_hash = None
            size = 0
            gitlinks += 1
        else:
            raise ValueError("unsupported tracked entry mode")
        manifest.update(mode.encode("ascii") + b"\0")
        manifest.update(object_id.encode("ascii") + b"\0")
        manifest.update(path_raw + b"\0")
        manifest.update((content_hash or object_id).encode("ascii") + b"\0")
        rows.append({
            "path": _path_text(path_raw),
            "path_base64": _path_b64(path_raw),
            "mode": mode,
            "object_id": object_id,
            "kind": kind,
            "content_sha256": content_hash,
            "bytes": size,
        })

    untracked_raw, ignored_raw, untracked_rows, untracked_public = _untracked_snapshot(root)
    tracked_bytes = regular_bytes + symlink_bytes
    public = {
        "tracked": {
            "manifest_sha256": manifest.hexdigest(),
            "stage_sha256": _sha256(stage_raw),
            "entries": len(entries),
            "bytes": tracked_bytes,
            "stage_bytes": len(stage_raw),
            "regular_files": regular_files,
            "regular_bytes": regular_bytes,
            "symlinks": symlinks,
            "symlink_target_bytes": symlink_bytes,
            "gitlinks": gitlinks,
        },
        "eligible": {
            "files": len(eligible),
            "bytes": eligible_bytes,
        },
        "untracked": untracked_public,
    }
    return CheckoutEvidence(
        root, expected_commit, stage_raw, rows, eligible, eligible_sources,
        untracked_raw, ignored_raw, untracked_rows, public)


def assert_checkout_unchanged(before, after):
    if before.root != after.root or before.expected_commit != after.expected_commit:
        raise ValueError("checkout comparison does not describe the same subject")
    if (before.stage_raw != after.stage_raw
            or before.public["tracked"] != after.public["tracked"]
            or before.public["untracked"] != after.public["untracked"]):
        raise ValueError("checkout changed during triage")


def _walk_nodes(root_node):
    stack = [root_node]
    while stack:
        node = stack.pop()
        yield node
        stack.extend(reversed(node.children))


def parse_population(paths, parser, *, root=None, captured_sources=None):
    """Parse every path and return aggregate counts plus private per-file rows."""
    paths = list(paths)
    root = Path(root) if root is not None else None
    rows = []
    parsed_count = error_only = missing_only = both = faults = clean = 0
    for path in paths:
        path = Path(path)
        row = {"path": path.relative_to(root).as_posix() if root else str(path)}
        try:
            source = (captured_sources[path] if captured_sources is not None
                      else _read_regular_no_follow(path))
            tree = parser.parse(source)
            if tree is None or tree.root_node is None:
                raise RuntimeError("parser returned no tree")
            has_error = False
            has_missing = False
            node_count = 0
            for node in _walk_nodes(tree.root_node):
                node_count += 1
                if node.type == "ERROR":
                    has_error = True
                if node.is_missing:
                    has_missing = True
            parsed_count += 1
            if has_error and has_missing:
                both += 1
            elif has_error:
                error_only += 1
            elif has_missing:
                missing_only += 1
            else:
                clean += 1
            row.update({"parsed": True, "error": has_error, "missing": has_missing,
                        "nodes_visited": node_count, "fault": None})
        except Exception as exc:  # A triage fault is evidence and must remain counted.
            faults += 1
            row.update({"parsed": False, "error": False, "missing": False,
                        "nodes_visited": 0, "fault": type(exc).__name__,
                        "fault_message": str(exc)})
        finally:
            reset = getattr(parser, "reset", None)
            if reset is not None:
                try:
                    reset()
                except Exception as exc:
                    if row.get("fault") is None:
                        parsed_count -= 1
                        if row["error"] and row["missing"]:
                            both -= 1
                        elif row["error"]:
                            error_only -= 1
                        elif row["missing"]:
                            missing_only -= 1
                        else:
                            clean -= 1
                        faults += 1
                        row.update({"parsed": False, "fault": type(exc).__name__,
                                    "fault_message": str(exc)})
        rows.append(row)
    union = error_only + missing_only + both
    total = len(paths)
    summary = {
        "population_count": total,
        "parsed_count": parsed_count,
        "error_or_missing_count": union,
        "error_only_count": error_only,
        "missing_only_count": missing_only,
        "both_count": both,
        "fault_count": faults,
        "clean_count": clean,
        "clean_rate": clean / total if total else 0.0,
    }
    return summary, rows


def parse_grammar_spec(value):
    fields = value.split("=", 2)
    if len(fields) != 3 or not GRAMMAR_ID_RE.fullmatch(fields[0]) \
            or not SYMBOL_RE.fullmatch(fields[1]) or not fields[2]:
        raise argparse.ArgumentTypeError("expected id=symbol=localdylib")
    return fields[0], fields[1], Path(fields[2])


def load_grammar(grammar_id, symbol, path):
    from tree_sitter import Language, Parser

    path = path.resolve()
    library_bytes = _read_regular_no_follow(path)
    library = ctypes.CDLL(str(path))
    try:
        factory = getattr(library, symbol)
    except AttributeError as exc:
        raise ValueError("grammar library lacks requested symbol") from exc
    factory.restype = ctypes.c_void_p
    pointer = factory()
    if not pointer:
        raise ValueError("grammar factory returned a null language pointer")
    capsule_factory = ctypes.pythonapi.PyCapsule_New
    capsule_factory.restype = ctypes.py_object
    capsule_factory.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
    capsule = capsule_factory(pointer, b"tree_sitter.Language", None)
    language = Language(capsule)
    parser = Parser(language)
    return LoadedGrammar(
        grammar_id, symbol, path, library, language, parser, _sha256(library_bytes))


def write_receipts(out, public, private):
    out = Path(out)
    private_dir = out / "private"
    private_dir.mkdir(parents=True, exist_ok=True)
    (private_dir / "evidence.json").write_text(
        json.dumps(private, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    # Publish the summary last so its presence means the private evidence was
    # written successfully.
    (out / "summary.json").write_text(
        json.dumps(public, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def _validated_out(value, parser):
    out = value.resolve()
    workspace = (Path.cwd() / "workspace").resolve()
    if out == workspace or workspace not in out.parents:
        parser.error("--out must be below this checkout's workspace/")
    if out.exists():
        parser.error("--out must not already exist")
    return out


def main(argv=None):
    argument_parser = argparse.ArgumentParser(description=__doc__)
    argument_parser.add_argument("--subject", required=True, type=Path)
    argument_parser.add_argument("--commit", required=True)
    argument_parser.add_argument("--grammar", action="append", required=True,
                                 type=parse_grammar_spec,
                                 help="repeatable id=symbol=localdylib")
    argument_parser.add_argument("--out", required=True, type=Path)
    args = argument_parser.parse_args(argv)
    if not FULL_SHA_RE.fullmatch(args.commit):
        argument_parser.error("--commit must be a full lowercase 40-character SHA")
    grammar_ids = [item[0] for item in args.grammar]
    if len(grammar_ids) != len(set(grammar_ids)):
        argument_parser.error("grammar ids must be unique")
    out = _validated_out(args.out, argument_parser)

    version = importlib.metadata.version("tree-sitter")
    if version != TREE_SITTER_VERSION:
        raise ValueError(f"tree-sitter {version}; expected {TREE_SITTER_VERSION}")
    before = capture_checkout(args.subject, args.commit)
    loaded = [load_grammar(*spec) for spec in args.grammar]
    out.mkdir(parents=True)
    private_dir = out / "private"
    private_dir.mkdir()
    (private_dir / "tracked-before.stage-z").write_bytes(before.stage_raw)
    (private_dir / "untracked-before.z").write_bytes(before.untracked_raw)
    (private_dir / "ignored-before.z").write_bytes(before.ignored_raw)

    grammar_summaries = []
    parse_rows = {}
    try:
        for grammar in loaded:
            summary, rows = parse_population(
                before.eligible, grammar.parser, root=before.root,
                captured_sources=before.eligible_sources)
            grammar_summaries.append({
                "id": grammar.grammar_id,
                "library_sha256": grammar.sha256,
                "abi_version": grammar.language.abi_version,
                **summary,
            })
            parse_rows[grammar.grammar_id] = rows
        after = capture_checkout(args.subject, args.commit)
        (private_dir / "tracked-after.stage-z").write_bytes(after.stage_raw)
        (private_dir / "untracked-after.z").write_bytes(after.untracked_raw)
        (private_dir / "ignored-after.z").write_bytes(after.ignored_raw)
        assert_checkout_unchanged(before, after)
        public = {
            "schema": "opencl-c-native-triage-v1",
            "scope": "native grammar parse triage only",
            "tree_sitter": version,
            "expected_commit": args.commit,
            "tracked": {
                "before": before.public["tracked"],
                "after": after.public["tracked"],
                "unchanged": True,
            },
            "eligible": before.public["eligible"],
            "untracked": {
                "before": before.public["untracked"],
                "after": after.public["untracked"],
                "unchanged": True,
            },
            "checkout_unchanged": True,
            "grammars": grammar_summaries,
        }
        private = {
            "subject": str(before.root),
            "tracked_rows_before": before.rows,
            "tracked_rows_after": after.rows,
            "untracked_rows_before": before.untracked_rows,
            "untracked_rows_after": after.untracked_rows,
            "grammars": [
                {"id": grammar.grammar_id, "symbol": grammar.symbol,
                 "library": str(grammar.path), "parse_rows": parse_rows[grammar.grammar_id]}
                for grammar in loaded
            ],
        }
        write_receipts(out, public, private)
        print(json.dumps(public, sort_keys=True))
        return 1 if any(item["fault_count"] for item in grammar_summaries) else 0
    except Exception as exc:
        (private_dir / "failure.json").write_text(json.dumps({
            "type": type(exc).__name__, "message": str(exc)}, sort_keys=True) + "\n")
        raise


if __name__ == "__main__":
    sys.exit(main())
