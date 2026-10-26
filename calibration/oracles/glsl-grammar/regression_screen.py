#!/usr/bin/env python3
"""Pinned, fail-closed native GLSL grammar regression screen."""

from __future__ import annotations

import argparse
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
from typing import Any, Callable, Iterable, Iterator


MACRO_PRECISION = re.compile(
    rb"\bprecision[\t \r\n]+([A-Za-z_][A-Za-z0-9_]*)[\t \r\n]+"
    rb"([A-Za-z_][A-Za-z0-9_]*)[\t \r\n]*;",
    re.MULTILINE,
)
PRECISION_QUALIFIERS = {b"highp", b"mediump", b"lowp"}


class ScreenError(RuntimeError):
    pass


def _glsl_tokens(data: bytes) -> Iterator[tuple[str, bytes, int, int]]:
    """Yield significant tokens while retaining original byte offsets."""
    index = 0
    size = len(data)
    line_start = 0
    while index < size:
        byte = data[index]
        if byte in b" \t\r\n":
            if byte == 10:
                line_start = index + 1
            index += 1
            continue
        if data.startswith(b"//", index):
            newline = data.find(b"\n", index + 2)
            index = size if newline < 0 else newline
            continue
        if data.startswith(b"/*", index):
            start = index
            close = data.find(b"*/", index + 2)
            index = size if close < 0 else close + 2
            newline = data.rfind(b"\n", start, index)
            if newline >= 0:
                line_start = newline + 1
            continue
        if byte == 35 and not data[line_start:index].strip():
            while index < size:
                newline = data.find(b"\n", index)
                if newline < 0:
                    index = size
                    break
                if newline > 0 and data[newline - 1] == 92:
                    index = newline + 1
                    line_start = index
                    continue
                index = newline
                break
            continue
        if byte in (34, 39):
            quote = byte
            start = index
            index += 1
            while index < size:
                if data[index] == 92:
                    index = min(size, index + 2)
                elif data[index] == quote:
                    index += 1
                    break
                else:
                    index += 1
            newline = data.rfind(b"\n", start, index)
            if newline >= 0:
                line_start = newline + 1
            yield "other", data[start:index], start, index
            continue
        if byte == 95 or 65 <= byte <= 90 or 97 <= byte <= 122:
            start = index
            index += 1
            while index < size and (
                data[index] == 95
                or 48 <= data[index] <= 57
                or 65 <= data[index] <= 90
                or 97 <= data[index] <= 122
            ):
                index += 1
            yield "identifier", data[start:index], start, index
            continue
        yield "punctuation", data[index:index + 1], index, index + 1
        index += 1


def precision_statements(data: bytes) -> list[dict[str, Any]]:
    """Find actual precision statements without copying or normalizing source bytes."""
    tokens = list(_glsl_tokens(data))
    statements: list[dict[str, Any]] = []
    for index in range(len(tokens) - 3):
        keyword, qualifier, type_name, terminator = tokens[index:index + 4]
        previous = tokens[index - 1] if index else None
        if (
            keyword[:2] == ("identifier", b"precision")
            and qualifier[0] == "identifier"
            and type_name[0] == "identifier"
            and terminator[:2] == ("punctuation", b";")
            and (previous is None or previous[1] in (b";", b"{", b"}"))
        ):
            statements.append({
                "start_byte": keyword[2],
                "end_byte": terminator[3],
                "qualifier": qualifier[1].decode("ascii"),
                "type": type_name[1].decode("ascii"),
                "qualifier_kind": "literal" if qualifier[1] in PRECISION_QUALIFIERS else "macro",
            })
    return statements


def _node_span(node: Any) -> tuple[int, int]:
    return int(getattr(node, "start_byte", 0)), int(getattr(node, "end_byte", 0))


def inspect_statement(root_node: Any, start: int, end: int) -> dict[str, Any]:
    """Require and inspect the statement's exact named declaration subtree."""
    declarations: list[Any] = []
    overlap_details: list[dict[str, Any]] = []
    stack = [root_node]
    while stack:
        node = stack.pop()
        node_start, node_end = _node_span(node)
        if (
            getattr(node, "type", "") == "declaration"
            and bool(getattr(node, "is_named", True))
            and (node_start, node_end) == (start, end)
        ):
            declarations.append(node)
        is_error = bool(getattr(node, "is_error", False)) or getattr(node, "type", "") == "ERROR"
        is_missing = bool(getattr(node, "is_missing", False))
        overlaps = start <= node_start <= end if node_start == node_end else node_start < end and node_end > start
        if overlaps and (is_error or is_missing):
            overlap_details.append({
                "type": getattr(node, "type", ""),
                "error": is_error,
                "missing": is_missing,
                "start_byte": node_start,
                "end_byte": node_end,
            })
        stack.extend(reversed(list(getattr(node, "children", ()))))

    if len(declarations) != 1:
        return {
            "recognized": False,
            "state": "unrecognized",
            "error_nodes": 0,
            "missing_nodes": 0,
            "nodes": [],
            "overlapping_diagnostics": overlap_details,
            "container": None,
        }

    declaration = declarations[0]
    errors, missing, details = inspect_tree(declaration)
    state = "both" if errors and missing else "error" if errors else "missing" if missing else "passing"
    return {
        "recognized": True,
        "state": state,
        "error_nodes": errors,
        "missing_nodes": missing,
        "nodes": details,
        "overlapping_diagnostics": overlap_details,
        "container": {
            "type": "declaration",
            "start_byte": start,
            "end_byte": end,
        },
    }


def summarize_statement_states(rows: list[dict[str, Any]]) -> dict[str, Any]:
    def counts(selected: list[dict[str, Any]]) -> dict[str, int]:
        return {
            "total": len(selected),
            "passing": sum(row["state"] == "passing" for row in selected),
            "error": sum(row["state"] == "error" for row in selected),
            "missing": sum(row["state"] == "missing" for row in selected),
            "both": sum(row["state"] == "both" for row in selected),
            "unrecognized": sum(row["state"] == "unrecognized" for row in selected),
        }
    return {
        **counts(rows),
        "literal": counts([row for row in rows if row["qualifier_kind"] == "literal"]),
        "macro": counts([row for row in rows if row["qualifier_kind"] == "macro"]),
    }


def _git(root: Path, *args: str) -> bytes:
    try:
        return subprocess.check_output(
            ["git", "-C", str(root), *args], stderr=subprocess.PIPE
        )
    except subprocess.CalledProcessError as exc:
        detail = exc.stderr.decode("utf-8", "replace").strip()
        raise ScreenError(f"git {' '.join(args)} failed: {detail}") from exc


def _write_json(path: Path, value: Any) -> None:
    path.write_text(json.dumps(value, sort_keys=True, ensure_ascii=True) + "\n")


def _write_jsonl(path: Path, rows: Iterable[Any]) -> None:
    with path.open("w", encoding="utf-8", newline="\n") as stream:
        for row in rows:
            stream.write(json.dumps(row, ensure_ascii=True, separators=(",", ":")))
            stream.write("\n")


def _require_identity(root: Path, commit: str) -> str:
    if root.resolve() != Path(os.fsdecode(_git(root, "rev-parse", "--show-toplevel")).rstrip("\n")).resolve():
        raise ScreenError("--subject must be the subject Git checkout root")
    if not re.fullmatch(r"[0-9a-fA-F]{40}", commit):
        raise ScreenError("--commit must be a full 40-character Git commit")
    head = os.fsdecode(_git(root, "rev-parse", "HEAD")).strip().lower()
    if head != commit.lower():
        raise ScreenError(f"subject HEAD {head} does not equal --commit {commit.lower()}")
    if _git(root, "status", "--porcelain=v1", "-z", "--untracked-files=no"):
        raise ScreenError("subject tracked worktree is not clean")
    try:
        repo = os.fsdecode(_git(root, "config", "--get", "remote.origin.url")).strip()
    except ScreenError as exc:
        raise ScreenError("subject has no remote.origin.url for the public summary") from exc
    if not repo:
        raise ScreenError("subject has an empty remote.origin.url")
    return repo


def _tracked_entries(root: Path) -> list[tuple[bytes, bytes, bytes]]:
    raw = _git(root, "ls-files", "-s", "-z", "--cached", "--", ".")
    entries: list[tuple[bytes, bytes, bytes]] = []
    for record in raw.split(b"\0"):
        if not record:
            continue
        meta, name = record.split(b"\t", 1)
        mode, oid, stage = meta.split()
        if stage != b"0":
            raise ScreenError("unmerged index: resolve conflicts first")
        entries.append((name, mode, oid))
    if not entries:
        raise ScreenError("empty tracked set")
    return sorted(entries, key=lambda entry: entry[0])


def capture_manifest(
    root: Path, prefix: Path
) -> tuple[dict[str, Any], list[bytes], dict[str, Any]]:
    lines: list[str] = []
    regular: list[bytes] = []
    size = 0
    counts = {"files": 0, "symlinks": 0, "gitlinks": 0}
    for name, mode, oid in _tracked_entries(root):
        path = os.path.join(os.fsencode(root), name)
        try:
            info = os.lstat(path)
        except FileNotFoundError as exc:
            raise ScreenError(f"tracked path is missing: {os.fsdecode(name)!r}") from exc
        if mode in (b"100644", b"100755"):
            if not stat.S_ISREG(info.st_mode):
                raise ScreenError("tracked regular file has changed type")
            data = Path(os.fsdecode(path)).read_bytes()
            kind, identity = "file", hashlib.sha256(data).hexdigest()
            counts["files"] += 1
            size += len(data)
            regular.append(name)
        elif mode == b"120000":
            if not stat.S_ISLNK(info.st_mode):
                raise ScreenError("tracked symlink has changed type")
            target = os.readlink(path)
            target_bytes = target if isinstance(target, bytes) else os.fsencode(target)
            kind, identity = "symlink", hashlib.sha256(target_bytes).hexdigest()
            counts["symlinks"] += 1
        elif mode == b"160000":
            kind, identity = "gitlink", oid.decode("ascii")
            counts["gitlinks"] += 1
        else:
            raise ScreenError(f"unsupported Git index mode {mode.decode('ascii', 'replace')}")
        lines.append(
            json.dumps(
                [os.fsdecode(name), kind, identity],
                ensure_ascii=True,
                separators=(",", ":"),
            )
            + "\n"
        )
    manifest = "".join(lines).encode("utf-8")
    untracked = _git(root, "ls-files", "-z", "--others", "--", ".")
    prefix.parent.mkdir(parents=True, exist_ok=True)
    Path(str(prefix) + ".manifest").write_bytes(manifest)
    Path(str(prefix) + ".untracked").write_bytes(untracked)
    summary = {
        "sha256": hashlib.sha256(manifest).hexdigest(),
        "entries": sum(counts.values()),
        "bytes": size,
        **counts,
    }
    untracked_summary = {
        "entries": len([item for item in untracked.split(b"\0") if item]),
        "bytes": len(untracked),
        "sha256": hashlib.sha256(untracked).hexdigest(),
    }
    _write_json(Path(str(prefix) + ".json"), summary)
    _write_json(Path(str(prefix) + ".untracked.json"), untracked_summary)
    return summary, regular, untracked_summary


def load_dialect(path: Path) -> tuple[set[str], set[str], re.Pattern[bytes], int]:
    data = json.loads(path.read_text(encoding="utf-8"))
    match = data["match"]
    sniff = match["sniff"]
    flags = re.MULTILINE
    try:
        pattern = re.compile(sniff["pattern"].encode("utf-8"), flags)
    except (KeyError, re.error) as exc:
        raise ScreenError(f"invalid dialect sniff configuration: {exc}") from exc
    return (
        set(match["extensions"]),
        set(sniff["extensions"]),
        pattern,
        int(sniff["scan_bytes"]),
    )


def classify_files(
    root: Path,
    regular: Iterable[bytes],
    dialect: tuple[set[str], set[str], re.Pattern[bytes], int],
) -> tuple[list[tuple[bytes, int, str]], set[bytes]]:
    extensions, sniff_extensions, sniff_pattern, scan_bytes = dialect
    eligible: list[tuple[bytes, int, str]] = []
    macro_subset: set[bytes] = set()
    for name in regular:
        suffix = Path(os.fsdecode(name)).suffix
        if suffix not in extensions and suffix not in sniff_extensions:
            continue
        path = Path(os.fsdecode(os.path.join(os.fsencode(root), name)))
        data = path.read_bytes()
        if suffix in sniff_extensions and suffix not in extensions:
            if sniff_pattern.search(data[:scan_bytes]) is None:
                continue
        digest = hashlib.sha256(data).hexdigest()
        eligible.append((name, len(data), digest))
        if any(match.group(1) not in PRECISION_QUALIFIERS for match in MACRO_PRECISION.finditer(data)):
            macro_subset.add(name)
    return eligible, macro_subset


_NATIVE_LIBRARIES: list[ctypes.CDLL] = []


def native_parser(dylib: Path) -> Any:
    try:
        version = importlib.metadata.version("tree-sitter")
    except importlib.metadata.PackageNotFoundError as exc:
        raise ScreenError("tree_sitter==0.25.2 is required") from exc
    if version != "0.25.2":
        raise ScreenError(f"tree_sitter==0.25.2 is required, found {version}")
    try:
        from tree_sitter import Language, Parser
    except ImportError as exc:
        raise ScreenError("tree_sitter==0.25.2 is required") from exc
    try:
        library = ctypes.CDLL(str(dylib.resolve()), mode=getattr(ctypes, "RTLD_LOCAL", 0))
        symbol = library.tree_sitter_glsl
    except (OSError, AttributeError) as exc:
        raise ScreenError(f"cannot load tree_sitter_glsl from {dylib}: {exc}") from exc
    symbol.argtypes = []
    symbol.restype = ctypes.c_void_p
    pointer = symbol()
    if not pointer:
        raise ScreenError(f"tree_sitter_glsl returned NULL from {dylib}")
    capsule_new = ctypes.pythonapi.PyCapsule_New
    capsule_new.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
    capsule_new.restype = ctypes.py_object
    capsule = capsule_new(pointer, b"tree_sitter.Language", None)
    try:
        language = Language(capsule)
        parser = Parser(language)
    except Exception as exc:
        raise ScreenError(f"tree-sitter rejected native grammar {dylib}: {exc}") from exc
    _NATIVE_LIBRARIES.append(library)
    return parser


def inspect_tree(root_node: Any) -> tuple[int, int, list[dict[str, Any]]]:
    error_count = 0
    missing_count = 0
    details: list[dict[str, Any]] = []
    stack = [root_node]
    while stack:
        node = stack.pop()
        is_error = bool(getattr(node, "is_error", False)) or getattr(node, "type", "") == "ERROR"
        is_missing = bool(getattr(node, "is_missing", False))
        if is_error:
            error_count += 1
        if is_missing:
            missing_count += 1
        if is_error or is_missing:
            details.append(
                {
                    "type": getattr(node, "type", ""),
                    "error": is_error,
                    "missing": is_missing,
                    "start_point": list(getattr(node, "start_point", (0, 0))),
                    "end_point": list(getattr(node, "end_point", (0, 0))),
                }
            )
        stack.extend(reversed(list(getattr(node, "children", ()))))
    return error_count, missing_count, details


def scan_grammar(
    root: Path,
    eligible: list[tuple[bytes, int, str]],
    parser: Any,
    result_path: Path,
    failure_path: Path,
    statement_path: Path,
) -> tuple[dict[bytes, str], list[dict[str, Any]]]:
    states: dict[bytes, str] = {}
    rows: list[dict[str, Any]] = []
    failures: list[dict[str, Any]] = []
    statement_rows: list[dict[str, Any]] = []
    for name, size, digest in eligible:
        data = Path(os.fsdecode(os.path.join(os.fsencode(root), name))).read_bytes()
        if len(data) != size or hashlib.sha256(data).hexdigest() != digest:
            raise ScreenError(f"eligible source changed during scan: {os.fsdecode(name)!r}")
        try:
            tree = parser.parse(data)
            errors, missing, details = inspect_tree(tree.root_node)
        except Exception as exc:
            _write_jsonl(failure_path, failures + [{"path": os.fsdecode(name), "parser_exception": repr(exc)}])
            raise ScreenError(f"parser failed for {os.fsdecode(name)!r}: {exc}") from exc
        state = "both" if errors and missing else "error" if errors else "missing" if missing else "passing"
        states[name] = state
        row = {
            "path": os.fsdecode(name),
            "bytes": size,
            "sha256": digest,
            "error_nodes": errors,
            "missing_nodes": missing,
            "state": state,
        }
        rows.append(row)
        if state != "passing":
            failures.append({**row, "nodes": details})
        for statement in precision_statements(data):
            inspection = inspect_statement(
                tree.root_node, statement["start_byte"], statement["end_byte"]
            )
            statement_rows.append({
                "path": os.fsdecode(name),
                **statement,
                **inspection,
            })
    _write_jsonl(result_path, rows)
    _write_jsonl(failure_path, failures)
    _write_jsonl(statement_path, statement_rows)
    return states, statement_rows


def summarize_states(states: dict[bytes, str]) -> dict[str, int]:
    return {
        "error_only_files": sum(value == "error" for value in states.values()),
        "missing_only_files": sum(value == "missing" for value in states.values()),
        "both_files": sum(value == "both" for value in states.values()),
        "union_files": sum(value != "passing" for value in states.values()),
    }


def _parse_grammar(value: str) -> tuple[str, Path]:
    name, separator, raw_path = value.partition("=")
    if not separator or not re.fullmatch(r"[A-Za-z0-9_.-]+", name) or not raw_path:
        raise argparse.ArgumentTypeError("grammar must be name=DYLIB")
    return name, Path(raw_path)


def run(
    subject: Path,
    commit: str,
    grammars: list[tuple[str, Path]],
    dialect_path: Path,
    out: Path,
    parser_factory: Callable[[Path], Any] = native_parser,
) -> dict[str, Any]:
    subject = subject.resolve()
    out = out.resolve()
    workspace = Path(__file__).resolve().parents[3] / "workspace"
    if out != workspace and workspace not in out.parents:
        raise ScreenError(f"--out must be under {workspace}")
    if out == subject or subject in out.parents:
        raise ScreenError("--out must be outside the subject checkout")
    if len({name for name, _ in grammars}) != len(grammars):
        raise ScreenError("grammar names must be unique")
    if not grammars:
        raise ScreenError("at least one --grammar is required")
    if out.exists() and any(out.iterdir()):
        raise ScreenError("--out must be new or empty")
    out.mkdir(parents=True, exist_ok=True)
    private = out / "private"
    private.mkdir(exist_ok=True)
    repo = _require_identity(subject, commit)
    before, regular, untracked_before = capture_manifest(subject, private / "tracked-before")
    dialect = load_dialect(dialect_path)
    eligible, macro_subset = classify_files(subject, regular, dialect)
    if not eligible:
        raise ScreenError("no eligible tracked regular GLSL files")
    _write_jsonl(
        private / "eligible-files.jsonl",
        ({"path": os.fsdecode(name), "bytes": size, "sha256": digest} for name, size, digest in eligible),
    )
    eligible_receipt = b"".join(
        json.dumps([os.fsdecode(name), size, digest], ensure_ascii=True, separators=(",", ":")).encode("utf-8") + b"\n"
        for name, size, digest in eligible
    )
    all_states: list[dict[bytes, str]] = []
    statement_populations: list[list[dict[str, Any]]] = []
    grammar_summaries: list[dict[str, Any]] = []
    for index, (name, dylib) in enumerate(grammars):
        if not dylib.is_file():
            raise ScreenError(f"grammar library is not a regular file: {dylib}")
        binary_hash = hashlib.sha256(dylib.read_bytes()).hexdigest()
        states, statement_rows = scan_grammar(
            subject,
            eligible,
            parser_factory(dylib),
            private / f"{name}-results.jsonl",
            private / f"{name}-failures.jsonl",
            private / f"{name}-precision-statements.jsonl",
        )
        all_states.append(states)
        statement_populations.append(statement_rows)
        first = all_states[0]
        previous = all_states[index - 1] if index else states
        summary = {
            "name": name,
            "binary_sha256": binary_hash,
            **summarize_states(states),
            "new_passing_to_failing_vs_first": sum(
                first[path] == "passing" and state != "passing" for path, state in states.items()
            ),
            "new_passing_to_failing_vs_previous": sum(
                previous[path] == "passing" and state != "passing" for path, state in states.items()
            ),
            "macro_precision_candidate_files_with_errors": sum(
                path in macro_subset and state != "passing" for path, state in states.items()
            ),
            "precision_statement_subtrees": summarize_statement_states(statement_rows),
        }
        grammar_summaries.append(summary)
    after, _, untracked_after = capture_manifest(subject, private / "tracked-after")
    _require_identity(subject, commit)
    if before != after or untracked_before != untracked_after:
        raise ScreenError("subject tracked or untracked source set changed during screen")
    precision_population = statement_populations[0]
    precision_receipt = b"".join(
        json.dumps(
            [row["path"], row["start_byte"], row["end_byte"], row["qualifier_kind"]],
            ensure_ascii=True,
            separators=(",", ":"),
        ).encode("utf-8") + b"\n"
        for row in precision_population
    )
    summary = {
        "repo": repo,
        "commit": commit.lower(),
        "tracked": before,
        "untracked": untracked_before,
        "eligible": {
            "files": len(eligible),
            "bytes": sum(size for _, size, _ in eligible),
            "sha256": hashlib.sha256(eligible_receipt).hexdigest(),
        },
        "macro_precision_candidate_files": len(macro_subset),
        "precision_statements": {
            "total": len(precision_population),
            "literal": sum(row["qualifier_kind"] == "literal" for row in precision_population),
            "macro": sum(row["qualifier_kind"] == "macro" for row in precision_population),
            "sha256": hashlib.sha256(precision_receipt).hexdigest(),
        },
        "grammars": grammar_summaries,
    }
    _write_json(out / "summary.json", summary)
    return summary


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--subject", required=True, type=Path)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--grammar", required=True, action="append", type=_parse_grammar)
    parser.add_argument("--dialect", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        summary = run(args.subject, args.commit, args.grammar, args.dialect, args.out)
    except (ScreenError, OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    print(json.dumps(summary, sort_keys=True, ensure_ascii=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
