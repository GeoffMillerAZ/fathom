#!/usr/bin/env python3
"""Build a pinned tier-B OpenCL C compilation database from a Git snapshot.

The database reconstructs one uniform compiler context for every tracked,
regular, lowercase ``.cl`` file.  It does not infer the subject's runtime
BuildProgram grouping or claim that the reconstructed context is upstream's.
"""

import argparse
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import sys


_TRIAGE_PATH = Path(__file__).with_name("triage.py")
_TRIAGE_SPEC = importlib.util.spec_from_file_location("opencl_c_triage_for_compdb", _TRIAGE_PATH)
triage = importlib.util.module_from_spec(_TRIAGE_SPEC)
_TRIAGE_SPEC.loader.exec_module(triage)

SHA256_RE = re.compile(r"[0-9a-f]{64}\Z")
PINNED_RESOURCE_HEADERS = ("opencl-c-base.h", "opencl-c.h")


def sha256_file(path):
    """Hash a regular file without following a final-component symlink."""
    return hashlib.sha256(triage._read_regular_no_follow(Path(path))).hexdigest()


def verify_sha256(path, expected, label):
    """Return the digest, rejecting a mismatch even under ``python -O``."""
    if not SHA256_RE.fullmatch(expected):
        raise ValueError(f"{label} SHA-256 pin must be 64 lowercase hexadecimal characters")
    actual = sha256_file(path)
    if actual != expected:
        raise ValueError(f"{label} SHA-256 does not match its pin")
    return actual


def _validated_resource_headers(resource_dir, expected_sha256):
    """Read exactly the pinned OpenCL headers and return their verified bytes."""
    resource_dir = Path(resource_dir).resolve()
    if not isinstance(expected_sha256, dict) or set(expected_sha256) != set(PINNED_RESOURCE_HEADERS):
        names = " and ".join(PINNED_RESOURCE_HEADERS)
        raise ValueError(f"resource header pins must name exactly {names}")
    rows = []
    contents = {}
    for name in PINNED_RESOURCE_HEADERS:
        expected = expected_sha256[name]
        if not SHA256_RE.fullmatch(expected):
            raise ValueError("resource header SHA-256 pin must be 64 lowercase hexadecimal characters")
        content = triage._read_regular_no_follow(resource_dir / "include" / name)
        digest = hashlib.sha256(content).hexdigest()
        if digest != expected:
            raise ValueError("resource header SHA-256 does not match its pin")
        rows.append({"path": name, "sha256": digest})
        contents[name] = content
    manifest = hashlib.sha256()
    for row in rows:
        manifest.update(row["path"].encode("utf-8", "surrogateescape"))
        manifest.update(b"\0")
        manifest.update(row["sha256"].encode("ascii"))
        manifest.update(b"\0")
    return rows, contents, manifest.hexdigest()


def hash_resource_headers(resource_dir, expected_sha256):
    """Hash and verify exactly the two pinned OpenCL Clang resource headers."""
    rows, _, manifest = _validated_resource_headers(resource_dir, expected_sha256)
    return rows, manifest


def _write_resource_snapshot(resource_dir, contents, expected_sha256):
    """Write and revalidate the minimal read-only Clang resource tree."""
    include = Path(resource_dir).resolve() / "include"
    include.mkdir(parents=True, exist_ok=False)
    for name in PINNED_RESOURCE_HEADERS:
        destination = include / name
        with destination.open("xb") as output:
            output.write(contents[name])
        destination.chmod(0o444)
        verify_sha256(destination, expected_sha256[name], "resource snapshot header")


def _row_path(row):
    raw = base64.b64decode(row["path_base64"], validate=True)
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ValueError("tracked paths must be valid UTF-8 for compilation database consumers") from exc
    relative = Path(text)
    if relative.is_absolute() or ".." in relative.parts:
        raise ValueError("tracked manifest contains an unsafe path")
    return relative, raw


def validate_utf8_tracked_paths(evidence):
    """Reject Git path bytes the JSON/Go consumer boundary cannot preserve."""
    for row in evidence.rows:
        _row_path(row)


def snapshot_regular_files(evidence, snapshot_root):
    """Copy every verified tracked regular file and return aggregate evidence."""
    snapshot_root = Path(snapshot_root).resolve()
    snapshot_root.mkdir(parents=True, exist_ok=False)
    manifest = hashlib.sha256()
    files = total_bytes = 0
    compile_relatives = []
    for row in sorted(evidence.rows, key=lambda item: item["path_base64"]):
        if row["kind"] != "regular":
            continue
        relative, raw_path = _row_path(row)
        source = evidence.root / relative
        content = triage._read_regular_no_follow(source)
        if triage._git_blob_oid(content, row["object_id"]) != row["object_id"]:
            raise ValueError("tracked content changed before snapshot copy")
        destination = snapshot_root / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        with destination.open("xb") as output:
            output.write(content)
        destination.chmod(0o555 if row["mode"] == "100755" else 0o444)
        digest = hashlib.sha256(content).hexdigest()
        manifest.update(row["mode"].encode("ascii") + b"\0")
        manifest.update(raw_path + b"\0")
        manifest.update(digest.encode("ascii") + b"\0")
        files += 1
        total_bytes += len(content)
        if raw_path.endswith(b".cl"):
            compile_relatives.append(relative)
    compile_relatives.sort(key=lambda path: os.fsencode(str(path)))
    return {
        "files": files,
        "bytes": total_bytes,
        "manifest_sha256": manifest.hexdigest(),
    }, compile_relatives


def build_entries(snapshot_root, relatives, clang, resource_dir):
    """Return sorted JSON Compilation Database entries for all compile roots."""
    snapshot_root = Path(snapshot_root).resolve()
    clang = Path(clang).resolve()
    resource_dir = Path(resource_dir).resolve()
    entries = []
    for relative in relatives:
        source = (snapshot_root / relative).resolve()
        arguments = [
            str(clang),
            "--no-default-config",
            "-x", "cl",
            "--target=spir64-unknown-unknown",
            "-cl-std=CL3.0",
            "-resource-dir", str(resource_dir),
            "-Xclang", "-finclude-default-header",
            "-c", str(source),
            "-o", os.devnull,
        ]
        entries.append({
            "directory": str(snapshot_root),
            "file": str(source),
            "arguments": arguments,
        })
    return entries


def _compile_manifest(entries, snapshot_root):
    digest = hashlib.sha256()
    for entry in entries:
        relative = Path(entry["file"]).relative_to(snapshot_root).as_posix()
        digest.update(relative.encode("utf-8", "surrogateescape") + b"\0")
        for argument in entry["arguments"]:
            digest.update(argument.encode("utf-8", "surrogateescape") + b"\0")
    return digest.hexdigest()


def generate(*, subject, commit, clang, clang_sha256, resource_dir, out,
             expected_header_sha256=None):
    """Capture, snapshot, and synthesize the database; return its public receipt."""
    subject = Path(subject).resolve()
    clang = Path(clang).resolve()
    resource_dir = Path(resource_dir).resolve()
    out = Path(out).resolve()
    if out.exists():
        raise ValueError("output must not already exist")
    if out == subject or subject in out.parents:
        raise ValueError("output must be outside the subject checkout")

    compiler_before = verify_sha256(clang, clang_sha256, "compiler")
    header_rows, header_contents, header_manifest = _validated_resource_headers(
        resource_dir, expected_header_sha256)
    before = triage.capture_checkout(subject, commit)
    validate_utf8_tracked_paths(before)
    out.mkdir(parents=True)
    resource_snapshot = out / "resource"
    _write_resource_snapshot(resource_snapshot, header_contents, expected_header_sha256)
    snapshot_root = out / "subject"
    snapshot_summary, compile_relatives = snapshot_regular_files(before, snapshot_root)
    entries = build_entries(snapshot_root, compile_relatives, clang, resource_snapshot)
    (out / "compile_commands.json").write_text(
        json.dumps(entries, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    after = triage.capture_checkout(subject, commit)
    triage.assert_checkout_unchanged(before, after)
    compiler_after = verify_sha256(clang, clang_sha256, "compiler")
    if compiler_before != compiler_after:
        raise ValueError("compiler changed during compilation-database capture")

    public = {
        "schema": "opencl-c-synthetic-compdb-v1",
        "oracle_tier": "B",
        "context": "uniform reconstructed OpenCL C context; runtime source grouping is not inferred",
        "expected_commit": commit,
        "source": {
            "before": before.public["tracked"],
            "after": after.public["tracked"],
            "unchanged": True,
        },
        "snapshot": snapshot_summary,
        "omitted": {
            "symlinks": before.public["tracked"]["symlinks"],
            "gitlinks": before.public["tracked"]["gitlinks"],
        },
        "compile_units": {
            "files": len(entries),
            "manifest_sha256": _compile_manifest(entries, snapshot_root),
        },
        "compiler": {
            "before_sha256": compiler_before,
            "after_sha256": compiler_after,
            "unchanged": True,
        },
        "resource_headers": {
            "files": len(header_rows),
            "manifest_sha256": header_manifest,
            "content_sha256": sorted(row["sha256"] for row in header_rows),
        },
    }
    private_dir = out / "private"
    private_dir.mkdir()
    (private_dir / "tracked-before.stage-z").write_bytes(before.stage_raw)
    (private_dir / "tracked-after.stage-z").write_bytes(after.stage_raw)
    (private_dir / "untracked-before.z").write_bytes(before.untracked_raw)
    (private_dir / "untracked-after.z").write_bytes(after.untracked_raw)
    (private_dir / "ignored-before.z").write_bytes(before.ignored_raw)
    (private_dir / "ignored-after.z").write_bytes(after.ignored_raw)
    (private_dir / "evidence.json").write_text(json.dumps({
        "subject": str(subject),
        "snapshot": str(snapshot_root),
        "clang": str(clang),
        "resource_dir": str(resource_dir),
        "resource_snapshot": str(resource_snapshot),
        "tracked_rows_before": before.rows,
        "tracked_rows_after": after.rows,
        "untracked_rows_before": before.untracked_rows,
        "untracked_rows_after": after.untracked_rows,
        "resource_headers": header_rows,
        "compile_roots": [path.as_posix() for path in compile_relatives],
    }, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    # Summary last: its presence means the snapshot, database, and private
    # evidence all completed successfully.
    (out / "summary.json").write_text(
        json.dumps(public, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return public


def _load_compiler_pins():
    pins = json.loads(Path(__file__).with_name("pins.json").read_text(encoding="utf-8"))
    compiler = pins.get("compiler", {})
    binary = compiler.get("binary_sha256")
    headers = compiler.get("resource_header_sha256")
    if not SHA256_RE.fullmatch(binary or "") or not isinstance(headers, dict) or not headers:
        raise ValueError("pins.json lacks complete compiler pins")
    return binary, headers


def _validated_out(value, parser, subject):
    out = value.resolve()
    workspace = (Path(__file__).resolve().parents[3] / "workspace").resolve()
    if out == workspace or workspace not in out.parents:
        parser.error("--out must be a fresh descendant of this checkout's workspace/")
    subject = subject.resolve()
    if out == subject or subject in out.parents:
        parser.error("--out must be outside --subject")
    if out.exists():
        parser.error("--out must not already exist")
    return out


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--subject", required=True, type=Path)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--clang", required=True, type=Path)
    parser.add_argument("--clang-sha256", required=True)
    parser.add_argument("--resource-dir", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args(argv)
    if not triage.FULL_SHA_RE.fullmatch(args.commit):
        parser.error("--commit must be a full lowercase 40-character SHA")
    if not SHA256_RE.fullmatch(args.clang_sha256):
        parser.error("--clang-sha256 must be 64 lowercase hexadecimal characters")
    out = _validated_out(args.out, parser, args.subject)
    pinned_compiler, pinned_headers = _load_compiler_pins()
    if args.clang_sha256 != pinned_compiler:
        raise ValueError("--clang-sha256 does not match pins.json compiler pin")
    public = generate(
        subject=args.subject,
        commit=args.commit,
        clang=args.clang,
        clang_sha256=args.clang_sha256,
        resource_dir=args.resource_dir,
        out=out,
        expected_header_sha256=pinned_headers,
    )
    print(json.dumps(public, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
