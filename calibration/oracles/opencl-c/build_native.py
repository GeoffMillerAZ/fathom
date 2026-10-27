#!/usr/bin/env python3
"""Build and corpus-test one pinned experimental OpenCL grammar.

This is a calibration-only builder. It accepts a caller-supplied archive, writes
only to a fresh workspace directory, and never produces a product WASM blob.
"""

import argparse
import copy
import gzip
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import stat
import subprocess
import sys
import tarfile


SCHEMA = "opencl-native-grammar-candidates-v1"
RECEIPT_SCHEMA = "opencl-native-grammar-build-v1"
TREE_SITTER_VERSION = "0.26.11"
FULL_SHA_RE = re.compile(r"[0-9a-f]{40}\Z")
SHA256_RE = re.compile(r"[0-9a-f]{64}\Z")
ID_RE = re.compile(r"[a-z0-9][a-z0-9._-]*\Z")
SYMBOL_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]*\Z")
PATCH_RE = re.compile(r"patches/(?:[A-Za-z0-9._-]+/)*[A-Za-z0-9._-]+\.patch\Z")


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def canonical_json(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"),
                      ensure_ascii=True).encode("utf-8")


def read_regular(path):
    path = Path(path)
    if path.is_symlink():
        raise ValueError(f"refusing symlink: {path.name}")
    try:
        metadata = path.stat()
    except OSError as exc:
        raise ValueError(f"required regular file is unreadable: {path.name}") from exc
    if not stat.S_ISREG(metadata.st_mode):
        raise ValueError(f"required path is not a regular file: {path.name}")
    return path.read_bytes()


def _valid_candidate(row):
    required = {
        "id", "repository", "version", "commit", "archive_sha256",
        "license_sha256", "licence", "entry_point", "generate", "scanner", "patches",
    }
    optional = {"archive_url"}
    if (not isinstance(row, dict) or not required.issubset(row)
            or not set(row).issubset(required | optional)):
        raise ValueError("candidate fields do not match the pinned schema")
    if not isinstance(row["id"], str) or not ID_RE.fullmatch(row["id"]):
        raise ValueError("candidate id is invalid")
    for field in ("repository", "version"):
        if not isinstance(row[field], str) or not row[field] or "\n" in row[field]:
            raise ValueError(f"candidate {field} is invalid")
    if "archive_url" in row and (not isinstance(row["archive_url"], str)
                                 or not row["archive_url"].startswith("https://")
                                 or "\n" in row["archive_url"]):
        raise ValueError("candidate archive_url is invalid")
    if not FULL_SHA_RE.fullmatch(row["commit"] if isinstance(row["commit"], str) else ""):
        raise ValueError("candidate commit must be a full lowercase SHA")
    for field in ("archive_sha256", "license_sha256"):
        if not SHA256_RE.fullmatch(row[field] if isinstance(row[field], str) else ""):
            raise ValueError(f"candidate {field} is invalid")
    if row["licence"] != "MIT":
        raise ValueError("experimental OpenCL candidates must be MIT licensed")
    if not isinstance(row["entry_point"], str) or not SYMBOL_RE.fullmatch(row["entry_point"]):
        raise ValueError("candidate entry point is invalid")
    if type(row["generate"]) is not bool or type(row["scanner"]) is not bool:
        raise ValueError("candidate generate and scanner pins must be booleans")
    if not isinstance(row["patches"], list):
        raise ValueError("candidate patches must be an array")
    seen = set()
    for patch in row["patches"]:
        if not isinstance(patch, dict) or set(patch) != {"path", "sha256"}:
            raise ValueError("candidate patch entry is invalid")
        path = patch["path"]
        digest = patch["sha256"]
        if not isinstance(path, str) or not PATCH_RE.fullmatch(path):
            raise ValueError("candidate patch path is invalid")
        if path in seen:
            raise ValueError("candidate patch path is duplicated")
        seen.add(path)
        if not isinstance(digest, str) or not SHA256_RE.fullmatch(digest):
            raise ValueError("candidate patch hash is invalid")


def load_candidate(path, candidate_id):
    raw = read_regular(path)
    try:
        document = json.loads(raw)
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValueError("grammar candidate descriptor is not valid JSON") from exc
    required = {"schema", "tree_sitter_cli", "candidates"}
    optional = {"scope"}
    if (not isinstance(document, dict) or not required.issubset(document)
            or not set(document).issubset(required | optional)):
        raise ValueError("grammar candidate descriptor fields are invalid")
    if "scope" in document and (not isinstance(document["scope"], str)
                                or not document["scope"]):
        raise ValueError("grammar candidate descriptor scope is invalid")
    if document["schema"] != SCHEMA:
        raise ValueError("grammar candidate descriptor schema is invalid")
    if document["tree_sitter_cli"] != TREE_SITTER_VERSION:
        raise ValueError("tree-sitter CLI pin is invalid")
    if not isinstance(document["candidates"], list):
        raise ValueError("grammar candidates must be an array")
    for row in document["candidates"]:
        _valid_candidate(row)
    ids = [row["id"] for row in document["candidates"]]
    if len(ids) != len(set(ids)):
        raise ValueError("grammar candidate ids must be unique")
    selected = [row for row in document["candidates"] if row["id"] == candidate_id]
    if len(selected) != 1:
        raise ValueError("candidate id must select exactly one pinned row")
    return selected[0], sha256(canonical_json(selected[0]))


def validate_tree_sitter_argument(row, tree_sitter):
    if tree_sitter is not None and not str(tree_sitter):
        raise ValueError("tree-sitter path is empty")
    if not row["generate"] and tree_sitter is not None:
        return


def require_tree_sitter(row, tree_sitter):
    validate_tree_sitter_argument(row, tree_sitter)
    if row["generate"] and tree_sitter is None:
        raise ValueError("--tree-sitter is required for a generated candidate")
    return tree_sitter


def resolve_executable(value, caller_cwd):
    if not isinstance(value, str) or not value or "\0" in value:
        raise ValueError("executable name is invalid")
    caller_cwd = Path(caller_cwd).resolve()
    if "/" in value:
        candidate = Path(value)
        if not candidate.is_absolute():
            candidate = caller_cwd / candidate
    else:
        found = shutil.which(value)
        if found is None:
            raise ValueError(f"executable is unavailable: {value}")
        candidate = Path(found)
        if not candidate.is_absolute():
            candidate = caller_cwd / candidate
    try:
        resolved = candidate.resolve(strict=True)
        metadata = resolved.stat()
    except OSError as exc:
        raise ValueError(f"executable is unavailable: {value}") from exc
    if not stat.S_ISREG(metadata.st_mode) or not os.access(resolved, os.X_OK):
        raise ValueError(f"executable is not a regular executable file: {value}")
    return str(resolved)


def preflight_executables(row, tree_sitter, cc, cxx, caller_cwd):
    requested_tree_sitter = require_tree_sitter(row, tree_sitter)
    resolved = {
        "cc": resolve_executable(cc, caller_cwd),
        "cxx": resolve_executable(cxx, caller_cwd),
        "tree_sitter": None,
    }
    if requested_tree_sitter is not None:
        resolved["tree_sitter"] = resolve_executable(requested_tree_sitter, caller_cwd)
    return resolved


def run_command(command, *, cwd=None, input_bytes=None, env=None, check=False):
    return subprocess.run(
        [str(part) for part in command], cwd=None if cwd is None else str(cwd),
        input=input_bytes, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        env=env, check=check,
    )


def _command_error(label, completed):
    raise ValueError(f"{label} failed with exit {completed.returncode}")


def _stripped_members(handle):
    stripped = []
    roots = set()
    for member in handle.getmembers():
        path = PurePosixPath(member.name)
        if path.is_absolute() or not path.parts or any(part in ("", ".", "..") for part in path.parts):
            raise ValueError("archive contains a noncanonical path")
        roots.add(path.parts[0])
        if len(roots) != 1:
            raise ValueError("archive does not have one common root")
        if len(path.parts) == 1:
            if not member.isdir():
                raise ValueError("archive common root is not a directory")
            continue
        clone = copy.copy(member)
        clone.name = PurePosixPath(*path.parts[1:]).as_posix()
        if clone.issym() or clone.islnk():
            raise ValueError("archive links are refused")
        stripped.append(clone)
    if not roots or not stripped:
        raise ValueError("archive is empty")
    return stripped


def _legacy_safe_extract(handle, members, destination):
    """Python 3.9 equivalent of a strict data-only tar extraction filter."""
    destination = destination.resolve()
    for member in members:
        target = (destination / member.name).resolve()
        try:
            target.relative_to(destination)
        except ValueError as exc:
            raise ValueError("archive member escapes extraction root") from exc
        if member.isdir():
            target.mkdir(parents=True, exist_ok=True)
            continue
        if not member.isfile():
            raise ValueError("archive contains a non-data member")
        target.parent.mkdir(parents=True, exist_ok=True)
        source = handle.extractfile(member)
        if source is None:
            raise ValueError("archive regular file has no content")
        with source, target.open("wb") as output:
            shutil.copyfileobj(source, output)
        os.chmod(target, member.mode & 0o755)


def extract_common_root(tar_bytes, destination):
    destination = Path(destination)
    destination.mkdir(parents=True, exist_ok=False)
    with tarfile.open(fileobj=io.BytesIO(tar_bytes), mode="r:") as handle:
        members = _stripped_members(handle)
        if hasattr(tarfile, "data_filter"):
            try:
                handle.extractall(destination, members=members, filter="data")
            except (tarfile.TarError, OSError) as exc:
                raise ValueError("archive failed safe data extraction") from exc
        else:
            _legacy_safe_extract(handle, members, destination)


def verify_and_extract_archive(archive_path, row, destination):
    archive_bytes = read_regular(archive_path)
    archive_digest = sha256(archive_bytes)
    if archive_digest != row["archive_sha256"]:
        raise ValueError("archive sha256 does not match the candidate pin")
    try:
        tar_bytes = gzip.decompress(archive_bytes)
    except (OSError, EOFError) as exc:
        raise ValueError("candidate archive is not valid gzip") from exc
    commit_result = run_command(["git", "get-tar-commit-id"], input_bytes=tar_bytes)
    if commit_result.returncode:
        _command_error("git get-tar-commit-id", commit_result)
    try:
        commit = commit_result.stdout.decode("ascii").strip()
    except UnicodeDecodeError as exc:
        raise ValueError("archive commit identity is not ASCII") from exc
    if commit != row["commit"] or not FULL_SHA_RE.fullmatch(commit):
        raise ValueError("archive commit identity does not match the candidate pin")
    extract_common_root(tar_bytes, destination)
    license_bytes = read_regular(Path(destination) / "LICENSE")
    license_digest = sha256(license_bytes)
    if license_digest != row["license_sha256"]:
        raise ValueError("LICENSE sha256 does not match the candidate pin")
    parser_bytes = read_regular(Path(destination) / "src" / "parser.c")
    return {
        "archive_sha256": archive_digest,
        "license_sha256": license_digest,
        "parser_sha256": sha256(parser_bytes),
    }


def validate_output(repo_root, output):
    repo_root = Path(repo_root).resolve()
    output = Path(output).resolve()
    try:
        output.relative_to(repo_root / "workspace")
    except ValueError as exc:
        raise ValueError("--out must be under this checkout's workspace directory") from exc
    if output.exists() or output.is_symlink():
        raise ValueError("--out must be fresh and must not already exist")
    return output


def _version_line(command, label):
    completed = run_command(command)
    if completed.returncode:
        _command_error(f"{label} version", completed)
    raw = completed.stdout or completed.stderr
    try:
        line = raw.decode("utf-8", "strict").splitlines()[0].strip()
    except (UnicodeDecodeError, IndexError) as exc:
        raise ValueError(f"{label} did not report a readable version") from exc
    if not line or "/" in line:
        raise ValueError(f"{label} version line is unsafe for the public receipt")
    return line


def exact_tree_sitter_version(executable):
    line = _version_line([executable, "--version"], "tree-sitter")
    match = re.search(r"(?:^|\s)(\d+\.\d+\.\d+)(?:\s|$)", line)
    if not match or match.group(1) != TREE_SITTER_VERSION:
        raise ValueError(f"tree-sitter must be exactly {TREE_SITTER_VERSION}")
    return line


def _private_environment(output):
    private = Path(output) / "private"
    environment = dict(os.environ)
    for name, relative in (
        ("XDG_CACHE_HOME", "cache"), ("XDG_CONFIG_HOME", "config"),
        ("XDG_DATA_HOME", "data"),
    ):
        path = private / relative
        path.mkdir(parents=True, exist_ok=True)
        environment[name] = str(path)
    return environment


def run_logged(command, *, cwd, log_dir, name, env=None):
    completed = run_command(command, cwd=cwd, env=env)
    log_dir = Path(log_dir)
    log_dir.mkdir(parents=True, exist_ok=True)
    (log_dir / f"{name}.stdout").write_bytes(completed.stdout)
    (log_dir / f"{name}.stderr").write_bytes(completed.stderr)
    if completed.returncode:
        _command_error(name, completed)
    return completed


def _scanner_path(root, enabled):
    candidates = [Path(root) / "src" / name for name in ("scanner.c", "scanner.cc")]
    present = [path for path in candidates if path.is_file() and not path.is_symlink()]
    if not enabled:
        return None
    if len(present) != 1:
        raise ValueError("scanner pin requires exactly one src/scanner.c or src/scanner.cc")
    return present[0]


def compile_library(root, output, scanner, cc, cxx, log_name, *, log_dir=None):
    root = Path(root)
    parser = root / "src" / "parser.c"
    read_regular(parser)
    scanner_path = _scanner_path(root, scanner)
    compiler = cxx if scanner_path is not None and scanner_path.suffix == ".cc" else cc
    shared_flag = "-dynamiclib" if sys.platform == "darwin" else "-shared"
    command = [compiler, "-O2", "-fPIC", shared_flag, "-I", root / "src",
               "-o", output, parser]
    if scanner_path is not None:
        command.append(scanner_path)
    return run_logged(command, cwd=root, log_dir=log_dir or output.parent / "logs",
                      name=log_name)


def _copy_regular(source, destination):
    data = read_regular(source)
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_bytes(data)


def _copy_data_tree(source, destination):
    if not source.is_dir() or source.is_symlink():
        raise ValueError(f"required directory is unavailable: {source.name}")
    for path in sorted(source.rglob("*")):
        relative = path.relative_to(source)
        if path.is_symlink():
            raise ValueError("proof input contains a symlink")
        if path.is_dir():
            (destination / relative).mkdir(parents=True, exist_ok=True)
        elif path.is_file():
            _copy_regular(path, destination / relative)
        else:
            raise ValueError("proof input contains a non-data path")


def prepare_proof_tree(source, proof, parser_digest):
    source = Path(source)
    proof = Path(proof)
    proof.mkdir(parents=True, exist_ok=False)
    unique = "openclproof" + parser_digest[:12]
    _copy_data_tree(source / "test" / "corpus", proof / "test" / "corpus")
    for name in ("parser.c", "grammar.json", "node-types.json"):
        _copy_regular(source / "src" / name, proof / "src" / name)
    for name in ("scanner.c", "scanner.cc"):
        path = source / "src" / name
        if path.exists():
            _copy_regular(path, proof / "src" / name)
    _copy_data_tree(source / "src" / "tree_sitter", proof / "src" / "tree_sitter")

    parser_path = proof / "src" / "parser.c"
    parser_text = parser_path.read_text(encoding="utf-8")
    pattern = re.compile(r"\btree_sitter_opencl(?=\s*\(\s*void\s*\))")
    parser_text, replacements = pattern.subn("tree_sitter_" + unique, parser_text)
    if replacements != 1:
        raise ValueError("generated parser must expose exactly one tree_sitter_opencl factory")
    parser_path.write_bytes(parser_text.encode("utf-8"))

    grammar_path = proof / "src" / "grammar.json"
    grammar = json.loads(grammar_path.read_text(encoding="utf-8"))
    if not isinstance(grammar, dict) or not isinstance(grammar.get("name"), str):
        raise ValueError("generated grammar.json has no name")
    grammar = {"name": unique, **{key: value for key, value in grammar.items()
                                 if key != "name"}}
    grammar_path.write_bytes(
        json.dumps(grammar, separators=(",", ":"), ensure_ascii=True).encode("utf-8") + b"\n")
    metadata = {
        "grammars": [{
            "name": unique,
            "camelcase": unique,
            "scope": "source." + unique,
            "path": ".",
            "file-types": ["cl"],
        }],
        "metadata": {"version": "0.0.0"},
    }
    (proof / "tree-sitter.json").write_bytes(canonical_json(metadata) + b"\n")
    (proof / "config.json").write_bytes(
        canonical_json({"parser-directories": []}) + b"\n")
    return unique


def corpus_test_command(tree_sitter, library, language, config):
    return [tree_sitter, "test", "--config-path", config, "--lib-path", library,
            "--lang-name", language]


def parse_test_counts(text):
    ansi = re.compile(r"\x1b\[[0-?]*[ -/]*[@-~]")
    clean = ansi.sub("", text)
    summary = re.search(
        r"\bTotal parses:\s*(\d+)\s*;\s*successful parses:\s*(\d+)\s*;\s*"
        r"failed parses:\s*(\d+)\b",
        clean, re.IGNORECASE,
    )
    if summary:
        total, passed, failed = map(int, summary.groups())
        if passed + failed != total:
            return {}
        return {"total": total, "passed": passed, "failed": failed, "skipped": 0}
    counts = {}
    for name in ("passed", "failed", "skipped"):
        match = re.search(rf"\b(\d+)\s+tests?\s+{name}\b", clean, re.IGNORECASE)
        if match:
            counts[name] = int(match.group(1))
    if counts:
        counts["total"] = sum(counts.values())
    return counts


def _patch_evidence(grammar_root, row):
    evidence = []
    for pin in row["patches"]:
        path = Path(grammar_root) / pin["path"]
        digest = sha256(read_regular(path))
        if digest != pin["sha256"]:
            raise ValueError("patch sha256 does not match the candidate pin")
        evidence.append({"path": pin["path"], "sha256": digest})
    return evidence


def build(args):
    repo_root = Path(__file__).resolve().parents[3]
    caller_cwd = Path.cwd()
    descriptor_path = Path(__file__).with_name("grammar-candidates.json")
    row, candidate_digest = load_candidate(descriptor_path, args.candidate)
    executables = preflight_executables(
        row, args.tree_sitter, args.cc, args.cxx, caller_cwd)
    tree_sitter = executables["tree_sitter"]
    cc = executables["cc"]
    cxx = executables["cxx"]
    tree_sitter_version = exact_tree_sitter_version(tree_sitter) if row["generate"] else None
    output = validate_output(repo_root, args.out)
    # Confirm the convention is backed by Git's ignore rules before writing.
    relative_output = output.relative_to(repo_root)
    ignored = run_command(["git", "-C", repo_root, "check-ignore", "-q", "--",
                           relative_output])
    if ignored.returncode != 0:
        raise ValueError("--out is not ignored by this checkout")

    archive_bytes = read_regular(args.archive)
    if sha256(archive_bytes) != row["archive_sha256"]:
        raise ValueError("archive sha256 does not match the candidate pin")
    patch_evidence = _patch_evidence(
        repo_root / "internal" / "extractor" / "tsengine" / "grammars", row)

    output.mkdir(parents=True, exist_ok=False)
    logs = output / "logs"
    source = output / "source"
    evidence = verify_and_extract_archive(args.archive, row, source)
    lock = repo_root / "internal" / "extractor" / "tsengine" / "grammars" / "grammars.lock.json"
    apply_script = lock.with_name("apply-patches.sh")
    run_logged(
        ["bash", apply_script, lock, canonical_json(row).decode("ascii"), source],
        cwd=repo_root, log_dir=logs, name="apply-patches")

    environment = _private_environment(output)
    if row["generate"]:
        run_logged([tree_sitter, "generate"], cwd=source, log_dir=logs,
                   name="generate", env=environment)

    parser_bytes = read_regular(source / "src" / "parser.c")
    parser_digest = sha256(parser_bytes)
    extension = ".dylib" if sys.platform == "darwin" else ".so"
    native = output / ("native" + extension)
    compile_library(source, native, row["scanner"], cc, cxx,
                    "compile-native", log_dir=logs)

    test_result = None
    proof_native = None
    proof_parser_digest = None
    unique = None
    if row["generate"]:
        if row["entry_point"] != "tree_sitter_opencl":
            raise ValueError("generated OpenCL proof requires tree_sitter_opencl")
        proof = output / "proof"
        unique = prepare_proof_tree(source, proof, parser_digest)
        proof_parser_digest = sha256(read_regular(proof / "src" / "parser.c"))
        proof_native = proof / ("native-proof" + extension)
        compile_library(proof, proof_native, row["scanner"], cc, cxx,
                        "compile-proof", log_dir=logs)
        completed = run_logged(
            corpus_test_command(tree_sitter, proof_native, unique, proof / "config.json"),
            cwd=proof, log_dir=logs, name="corpus-test", env=environment)
        combined = (completed.stdout + b"\n" + completed.stderr).decode("utf-8", "replace")
        test_result = {"exit_code": completed.returncode}
        test_result.update(parse_test_counts(combined))
        if sha256(read_regular(source / "src" / "parser.c")) != parser_digest:
            raise ValueError("proof build changed the original generated parser")

    tools = {
        "cc": _version_line([cc, "--version"], "C compiler"),
        "cxx": _version_line([cxx, "--version"], "C++ compiler"),
        "git": _version_line(["git", "--version"], "git"),
    }
    if tree_sitter_version is not None:
        tools["tree_sitter"] = tree_sitter_version
    receipt = {
        "schema": RECEIPT_SCHEMA,
        "scope": "calibration-only native grammar build; no product WASM",
        "candidate": {
            "id": row["id"], "repository": row["repository"],
            "version": row["version"], "commit": row["commit"],
            "entry_point": row["entry_point"], "generate": row["generate"],
            "scanner": row["scanner"],
        },
        "hashes": {
            "candidate_config_sha256": candidate_digest,
            "archive_sha256": evidence["archive_sha256"],
            "license_sha256": evidence["license_sha256"],
            "original_parser_c_sha256": evidence["parser_sha256"],
            "patches_sha256": sha256(canonical_json(patch_evidence)),
            "parser_c_sha256": parser_digest,
            "native_sha256": sha256(read_regular(native)),
            "tool_versions_sha256": sha256(canonical_json(tools)),
        },
        "patches": patch_evidence,
        "tool_versions": tools,
        "test": test_result,
    }
    if proof_native is not None:
        receipt["proof"] = {
            "language": unique,
            "parser_c_sha256": proof_parser_digest,
            "native_sha256": sha256(read_regular(proof_native)),
            "original_parser_unchanged": True,
        }
    (output / "build-receipt.json").write_bytes(
        json.dumps(receipt, indent=2, sort_keys=True).encode("utf-8") + b"\n")
    return receipt


def parse_args(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate", required=True)
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--tree-sitter", dest="tree_sitter")
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--cc", default="clang")
    parser.add_argument("--cxx", default="clang++")
    return parser.parse_args(argv)


def main(argv=None):
    try:
        receipt = build(parse_args(argv))
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        raise SystemExit(f"build_native.py: {exc}") from exc
    print(json.dumps(receipt, sort_keys=True))


if __name__ == "__main__":
    main()
