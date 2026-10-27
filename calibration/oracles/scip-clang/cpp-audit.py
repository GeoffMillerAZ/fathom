#!/usr/bin/env python3
"""Reproducible, measurement-only C++ index, oracle, and grade driver.

All raw output, diagnostics, and unmatched pairs stay below --work-dir. The
published receipt contains only fixed labels, counts, hashes, and relative
command identities. This driver does not change the shared harness or engine.
"""

import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import re
import shlex
import selectors
import shutil
import signal
import stat
import subprocess
import sys
import time


REPO = Path(__file__).resolve().parents[3]
PINS = {
    "fmt": ("1be298e1bd68957e4cd352e1f676f00e07dcfb57", "fmtlib/fmt",
            "include/**,src/**,test/*.cc,test/*.h,test/fuzzing/**"),
    "json": ("55f93686c01528224f448c19128836e7df245f72", "nlohmann/json",
             "include/**"),
    "spdlog": ("79524ddd08a4ec981b7fea76afd08ee05f83755d", "gabime/spdlog",
               "include/spdlog/*.h,include/spdlog/cfg/**,include/spdlog/details/**,include/spdlog/sinks/**,src/**,tests/*.cpp,tests/*.h,example/**"),
}
ORACLES = (("preproc", "calibration/oracles/scip-clang/preproc", "preproc.json"),
           ("calls", "calibration/oracles/clang-calls", "calls.json"),
           ("ranges", "calibration/oracles/clang-ast-defs", "preproc.ranges.json"))
JSON_CAP = 256 * 1024 * 1024
LOG_CAP = 64 * 1024 * 1024
READING_CAP = 256 * 1024 * 1024
DATE = "2026-10-26"
POPULATION = {"fmt": (51, 31, 0), "json": (93, 78, 0), "spdlog": (141, 34, 107)}
INDEX_BINARY = {"v0.4.0": "ff042fbc8a029f09f4b69fc7692e290e21c52923593207ee52d4e7439473ec64",
                "v0.4.1": "34a2eb7a5ef9bbe9849cded59f4eb22a2331bd5b64a5cec2b17ee09b8aa515de"}
SCIP_LICENSE = "96b8b909d18f94921214e3995e4c12483826b12ca64dd93fc710b87e1bc5a08a"
BUILD_FILE_CAP = 10000
BUILD_ENTRY_CAP = 20000
BUILD_BYTE_CAP = 1024 * 1024 * 1024
BUILD_DEPTH_CAP = 64
BUILD_FANOUT_CAP = 4096
FETCHED_PINS = {"spdlog": {"catch2-src": "53d0d913a422d356b23dd927547febdf69ee9081"}}


class Refusal(Exception):
    pass


def digest(path, cap=None):
    if not path.is_file() or (cap is not None and path.stat().st_size > cap):
        raise Refusal("input absent or over cap")
    h = hashlib.sha256()
    size = 0
    with path.open("rb") as stream:
        while True:
            chunk = stream.read(1024 * 1024 if cap is None else
                                min(1024 * 1024, cap - size + 1))
            if not chunk:
                break
            size += len(chunk)
            if cap is not None and size > cap:
                raise Refusal("input absent or over cap")
            h.update(chunk)
    return h.hexdigest()


def read_bounded(path, cap):
    with path.open("rb") as stream:
        data = stream.read(cap + 1)
    if len(data) > cap:
        raise Refusal("input absent or over cap")
    return data


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    with tmp.open("x") as stream:
        stream.write(json.dumps(value, indent=2, sort_keys=True) + "\n")
    tmp.replace(path)


def load_json(path, cap=JSON_CAP):
    digest(path, cap)
    return json.loads(read_bounded(path, cap))


def git(root, *args):
    result = subprocess.run(["git", "-C", str(root), *args], capture_output=True,
                            timeout=30, check=False)
    if result.returncode:
        raise Refusal("git identity or cleanliness check failed")
    return result.stdout.decode().strip()


def clean_repo():
    if git(REPO, "status", "--porcelain", "--untracked-files=all"):
        raise Refusal("fathom tree changed")


def tracked_manifest(root):
    if git(root, "status", "--porcelain", "--untracked-files=all"):
        raise Refusal("nonignored subject tree changed")
    result = subprocess.run(["git", "-C", str(root), "ls-files", "-z"],
                            capture_output=True, timeout=30, check=False)
    if result.returncode:
        raise Refusal("tracked manifest unavailable")
    h = hashlib.sha256()
    for raw in result.stdout.split(b"\0"):
        if not raw:
            continue
        path = root / os.fsdecode(raw)
        if not path.is_file() and not path.is_symlink():
            raise Refusal("tracked file absent")
        h.update(raw + b"\0")
        if path.is_symlink():
            h.update(os.fsencode(os.readlink(path)))
        else:
            h.update(bytes.fromhex(digest(path)))
    return h.hexdigest()


def language_inventory(root):
    suffixes = {".c", ".cc", ".cpp", ".cxx", ".h", ".hh", ".hpp", ".hxx"}
    h = hashlib.sha256()
    count = 0
    def walk_error(error):
        raise Refusal("language inventory unreadable") from error
    for directory, dirs, files in os.walk(root, onerror=walk_error):
        dirs[:] = sorted(name for name in dirs if name != ".git")
        for name in sorted(files):
            path = Path(directory) / name
            if path.suffix.lower() not in suffixes or not path.is_file():
                continue
            relative = path.relative_to(root).as_posix()
            h.update(relative.encode() + b"\0" + bytes.fromhex(digest(path)))
            count += 1
    return count, h.hexdigest()


def compiler_digests(compdb):
    db = load_json(compdb)
    paths = set()
    for entry in db:
        argv = entry.get("arguments") or shlex.split(entry.get("command", ""))
        if not argv:
            raise Refusal("compilation entry lacks compiler")
        compiler = Path(argv[0])
        if not compiler.is_absolute():
            if "/" in argv[0]:
                compiler = Path(entry["directory"]) / compiler
            else:
                located = shutil.which(argv[0])
                if located is None:
                    raise Refusal("compiler executable unavailable")
                compiler = Path(located)
        paths.add(compiler.resolve(strict=True))
    return tuple(sorted(digest(path) for path in paths))


def build_inputs(subject, compdb):
    """Bounded identity of the complete build tree, including fetched inputs."""
    build = compdb.parent.resolve(strict=True)
    subject = subject.resolve(strict=True)
    def covered(path):
        return any(under(path, root) and ".git" not in path.relative_to(root).parts
                   for root in (subject, build))
    db = load_json(compdb)
    if not isinstance(db, list):
        raise Refusal("compilation database malformed")
    for entry in db:
        if not isinstance(entry, dict) or not entry.get("file") or not entry.get("directory"):
            raise Refusal("compilation entry malformed")
        directory = Path(entry["directory"]).resolve(strict=True)
        source = Path(entry["file"])
        source = (source if source.is_absolute() else directory / source).resolve(strict=True)
        if not covered(source):
            raise Refusal("compilation source escapes subject and build")
        argv = entry.get("arguments") or shlex.split(entry.get("command", ""))
        if not argv:
            raise Refusal("compilation entry lacks command")
        flags = ("-include-pch", "-iframework", "-idirafter", "-isystem", "-imacros",
                 "-iquote", "-include", "-I", "-F")
        i = 1
        while i < len(argv):
            arg = argv[i]
            if arg.startswith("@"):
                raise Refusal("unexpanded compilation response file")
            flag = next((name for name in flags if arg == name or
                         (arg.startswith(name) and len(arg) > len(name))), None)
            if flag is not None:
                if arg == flag:
                    i += 1
                    if i >= len(argv):
                        raise Refusal("include argument missing path")
                    value = argv[i]
                else:
                    value = arg[len(flag):]
                if value.startswith("="):
                    value = value[1:]
                path = Path(value)
                path = (path if path.is_absolute() else directory / path).resolve(strict=True)
                if not covered(path):
                    raise Refusal("explicit include escapes subject and build")
            i += 1

    h = hashlib.sha256()
    files = links = total = entries_seen = 0
    def walk(directory, depth):
        nonlocal files, links, total, entries_seen
        if depth > BUILD_DEPTH_CAP:
            raise Refusal("build tree depth cap exceeded")
        try:
            entries = []
            with os.scandir(directory) as scan:
                for entry in scan:
                    entries.append(entry)
                    if len(entries) > BUILD_FANOUT_CAP:
                        raise Refusal("build tree fanout cap exceeded")
            entries.sort(key=lambda item: item.name)
        except OSError as error:
            raise Refusal("build tree unreadable") from error
        for entry in entries:
            if entry.name == ".git":
                continue
            entries_seen += 1
            if entries_seen > BUILD_ENTRY_CAP:
                raise Refusal("build tree entry cap exceeded")
            path = Path(entry.path)
            relative = path.relative_to(build).as_posix().encode()
            try:
                mode = entry.stat(follow_symlinks=False).st_mode
                if stat.S_ISLNK(mode):
                    target = path.resolve(strict=True)
                    if not under(target, build):
                        raise Refusal("build symlink escapes build tree")
                    if ".git" in target.relative_to(build).parts:
                        raise Refusal("build symlink enters excluded metadata")
                    links += 1
                    h.update(b"L\0" + relative + b"\0" + os.fsencode(os.readlink(path)) + b"\0")
                elif stat.S_ISDIR(mode):
                    walk(path, depth + 1)
                elif stat.S_ISREG(mode):
                    size = entry.stat(follow_symlinks=False).st_size
                    files += 1
                    total += size
                    if files > BUILD_FILE_CAP or total > BUILD_BYTE_CAP:
                        raise Refusal("build tree size cap exceeded")
                    h.update(b"F\0" + relative + b"\0" + bytes.fromhex(digest(path, BUILD_BYTE_CAP)))
                else:
                    raise Refusal("unsupported build tree entry")
            except OSError as error:
                raise Refusal("build tree unreadable") from error
    walk(build, 0)
    dependencies = []
    deps = build / "_deps"
    if deps.exists():
        sources = sorted(p for p in deps.iterdir() if p.name.endswith("-src"))
        for ordinal, source in enumerate(sources, 1):
            if not source.is_dir() or source.is_symlink() or \
                    git(source, "rev-parse", "--show-toplevel") != str(source.resolve()) or \
                    git(source, "rev-parse", "HEAD") != FETCHED_PINS.get(subject.name, {}).get(source.name) or \
                    git(source, "status", "--porcelain", "--untracked-files=all"):
                raise Refusal("fetched source revision or cleanliness mismatch")
            dependencies.append({"label": f"dependency_{ordinal}",
                                 "head": git(source, "rev-parse", "HEAD")})
        if set(p.name for p in sources) != set(FETCHED_PINS.get(subject.name, {})):
            raise Refusal("fetched source set mismatch")
    elif FETCHED_PINS.get(subject.name):
        raise Refusal("fetched source absent")
    return {"files": files, "symlinks": links, "bytes": total,
            "sha256": h.hexdigest(), "dependencies": dependencies}


def subjects(args):
    found = {}
    for slug, (pin, _, _) in PINS.items():
        subject = args.subjects_root / slug
        if not subject.is_dir() or git(subject, "rev-parse", "--show-toplevel") != str(subject.resolve()):
            raise Refusal("subject is not its checkout root")
        if git(subject, "rev-parse", "HEAD") != pin or git(subject, "status", "--porcelain", "--untracked-files=all"):
            raise Refusal("subject pin or nonignored cleanliness mismatch")
        compdb = args.builds_root / slug / "compile_commands.json"
        link = subject / "compile_commands.json"
        if not link.is_symlink() or link.resolve(strict=True) != compdb.resolve(strict=True):
            raise Refusal("root compilation database link mismatch")
        db = load_json(compdb)
        if not isinstance(db, list) or len(db) != POPULATION[slug][0]:
            raise Refusal("compilation database population mismatch")
        own, outside = set(), set()
        for entry in db:
            if not isinstance(entry, dict) or not entry.get("file") or not entry.get("directory"):
                raise Refusal("malformed compilation entry")
            file = Path(entry["file"])
            if not file.is_absolute():
                file = Path(entry["directory"]) / file
            file = file.resolve(strict=True)
            if not under(file, subject) and not under(file, compdb.parent.resolve(strict=True)):
                raise Refusal("compilation source escapes subject and build")
            (own if under(file, subject) else outside).add(file)
        if (len(own), len(outside)) != POPULATION[slug][1:]:
            raise Refusal("compilation database own/outside population mismatch")
        found[slug] = (subject, compdb, len(db), compiler_digests(compdb))
    return found


def under(path, base):
    return path == base or base in path.parents


def workspace_roots():
    common = Path(git(REPO, "rev-parse", "--git-common-dir"))
    if not common.is_absolute():
        common = REPO / common
    checkout = common.resolve(strict=True).parent
    for root in (checkout, REPO):
        ignored = subprocess.run(["git", "-C", str(root), "check-ignore", "-q", "workspace/"],
                                 capture_output=True, timeout=30, check=False)
        if ignored.returncode:
            raise Refusal("checkout workspace is not ignored")
    return (checkout / "workspace", REPO / "workspace")


def confined_output_paths(args, parser):
    """Refuse unsafe outputs before mkdir, lock acquisition, or receipt writes."""
    try:
        roots = tuple(path.resolve() for path in workspace_roots())
    except (Refusal, OSError, subprocess.TimeoutExpired):
        parser.error("ignored checkout workspace unavailable")
    outputs = [args.work_dir, args.indexes_dir, args.lock_dir]
    if args.readings_dir is not None:
        outputs.append(args.readings_dir)
    docs_readings = REPO / "docs" / "bench" / "readings" / "lang"
    for path in outputs:
        allowed = any(under(path, root) for root in roots)
        if path == args.readings_dir and under(path, docs_readings):
            allowed = True
        if not allowed:
            parser.error("output must be within an ignored checkout workspace")
    for first, path in enumerate(outputs):
        for other in outputs[first + 1:]:
            if under(path, other) or under(other, path):
                parser.error("output directories must be disjoint")
    inputs = [*(args.subjects_root / slug for slug in PINS), args.builds_root,
              *[REPO / module for _, module, _ in ORACLES]]
    inputs.extend(path for path in (args.indexbench, args.ctags, args.clang,
                                    args.scip_binaries_root) if path is not None)
    for path in outputs:
        for source in inputs:
            if under(path, source) or under(source, path):
                parser.error("output overlaps a subject, build, tool, or source")


def wire_fields(data):
    """Read only SCIP's length-delimited protobuf fields, with strict bounds."""
    pos = 0
    while pos < len(data):
        def varint():
            nonlocal pos
            value = 0
            for shift in range(0, 70, 7):
                if pos >= len(data):
                    raise Refusal("truncated SCIP wire")
                byte = data[pos]
                pos += 1
                value |= (byte & 127) << shift
                if byte < 128:
                    return value
            raise Refusal("invalid SCIP varint")
        tag = varint()
        number, kind = tag >> 3, tag & 7
        if not number:
            raise Refusal("invalid SCIP field")
        if kind == 2:
            size = varint()
            if size > len(data) - pos:
                raise Refusal("truncated SCIP field")
            value = data[pos:pos + size]
            pos += size
            yield number, value
        elif kind == 0:
            varint()
        elif kind == 1:
            pos += 8
        elif kind == 5:
            pos += 4
        else:
            raise Refusal("unsupported SCIP wire type")
        if pos > len(data):
            raise Refusal("truncated SCIP wire")


def scip_identity(path, subject, version):
    digest(path, READING_CAP)
    raw = read_bounded(path, READING_CAP)
    if not raw:
        raise Refusal("empty SCIP index")
    metadata, documents = [], 0
    for field, value in wire_fields(raw):
        if field == 1:
            metadata.append(value)
        elif field == 2:
            documents += 1
            relative = [v.decode("utf-8") for f, v in wire_fields(value) if f == 1]
            if len(relative) != 1 or not relative[0] or relative[0].startswith("/") or \
                    "\\" in relative[0] or "\0" in relative[0] or \
                    any(part in ("", ".", "..") for part in relative[0].split("/")):
                raise Refusal("SCIP document path is not canonical and relative")
    if len(metadata) != 1 or documents == 0:
        raise Refusal("SCIP metadata or documents absent")
    fields = list(wire_fields(metadata[0]))
    root = [v.decode("utf-8") for f, v in fields if f == 3]
    tools = [v for f, v in fields if f == 2]
    if root != [subject.as_uri()] or len(tools) != 1:
        raise Refusal("SCIP project root or tool absent")
    info = list(wire_fields(tools[0]))
    name = [v.decode("utf-8") for f, v in info if f == 1]
    declared = [v.decode("utf-8") for f, v in info if f == 2]
    if name != ["scip-clang"] or declared != [version[1:]]:
        raise Refusal("SCIP producer name or version mismatch")
    return documents


def index_evidence(args, found):
    receipt = load_json(args.index_receipt)
    if (not isinstance(receipt, dict) or receipt.get("status") != "complete" or
            receipt.get("producer") != "calibration/oracles/scip-clang/cpp-audit.py" or
            receipt.get("driver_sha256") != digest(Path(__file__)) or
            not isinstance(receipt.get("rows"), list) or len(receipt["rows"]) != 6):
        raise Refusal("index receipt is not a complete driver run")
    rows = {(r.get("subject"), r.get("version")): r for r in receipt["rows"] if isinstance(r, dict)}
    if len(rows) != 6:
        raise Refusal("duplicate or malformed index receipt")
    evidence = {}
    for slug, (subject, db, *_) in found.items():
        evidence[slug] = {}
        for version in INDEX_BINARY:
            row = rows.get((PINS[slug][1], version))
            index = args.indexes_dir / f"{slug}-{version}.scip"
            log = args.index_logs_dir / f"{slug}-{version}-index.log"
            stdout_log = args.index_logs_dir / f"{slug}-{version}-stdout.log"
            if (row is None or row.get("subject_sha") != PINS[slug][0] or
                    row.get("version") != version or row.get("binary_sha256") != INDEX_BINARY[version] or
                    row.get("database_sha256") != digest(db) or
                    row.get("tracked_sha256") != tracked_manifest(subject) or
                    row.get("language_inventory") != list(language_inventory(subject)) or
                    row.get("compiler_sha256") != list(compiler_digests(db)) or
                    row.get("build_inputs") != build_inputs(subject, db) or
                    row.get("scip_sha256") != digest(index) or row.get("scip_bytes") != index.stat().st_size):
                raise Refusal("index receipt does not attest frozen input")
            digest(log, LOG_CAP)
            digest(stdout_log, LOG_CAP)
            summary = (read_bounded(log, LOG_CAP).decode(errors="replace") + "\n" +
                       read_bounded(stdout_log, LOG_CAP).decode(errors="replace"))
            lines = re.findall(r"^Finished indexing (\d+) translation units .*num errored TUs: (\d+)\)\.$",
                               summary, re.MULTILINE)
            if len(lines) != 1 or (int(lines[0][0]), int(lines[0][1])) != (POPULATION[slug][0], 0):
                raise Refusal("index log population or errors mismatch")
            if row.get("index_log_sha256") != digest(log) or \
                    row.get("stdout_log_sha256") != digest(stdout_log) or \
                    row.get("indexed_units") != POPULATION[slug][0] or \
                    row.get("errored_units") != 0 or row.get("license_sha256") != SCIP_LICENSE:
                raise Refusal("index log or license receipt mismatch")
            documents = scip_identity(index, subject, version)
            if row.get("documents") != documents:
                raise Refusal("index document count receipt mismatch")
            evidence[slug][version] = {"scip_sha256": row["scip_sha256"],
                "scip_bytes": row["scip_bytes"], "binary_sha256": row["binary_sha256"],
                "index_log_sha256": digest(log), "stdout_log_sha256": digest(stdout_log),
                "indexed_units": int(lines[0][0]),
                "errored_units": 0, "documents": documents}
    return {"receipt_sha256": digest(args.index_receipt), "indexes": evidence}


def index_stamp(args, subject, compdb, binary, license_path):
    return (git(subject, "rev-parse", "HEAD"), tracked_manifest(subject),
            language_inventory(subject), digest(compdb), compiler_digests(compdb),
            build_inputs(subject, compdb),
            digest(binary), digest(license_path), digest(Path(__file__)))


def index_phase(args, found):
    clean_repo()
    if args.scip_binaries_root is None:
        raise Refusal("--scip-binaries-root is required for index")
    expected = [(slug, version) for slug in PINS for version in INDEX_BINARY]
    receipt = args.index_receipt
    if receipt.exists() or any((args.indexes_dir / f"{slug}-{version}.scip").exists()
                               for slug, version in expected):
        raise Refusal("index receipt or output already exists")
    args.indexes_dir.mkdir(parents=True, exist_ok=True)
    args.index_logs_dir.mkdir(parents=True, exist_ok=True)
    rows = []
    stage = "binary_validation"
    state = {"status": "failed", "producer": "calibration/oracles/scip-clang/cpp-audit.py",
             "driver_sha256": digest(Path(__file__)), "rows": None, "reason": stage}
    write_json(receipt, state)
    try:
        binaries = {}
        for version, sha in INDEX_BINARY.items():
            binary = args.scip_binaries_root / version / "scip-clang"
            license_path = args.scip_binaries_root / version / "LICENSE"
            if digest(binary) != sha or digest(license_path) != SCIP_LICENSE:
                raise Refusal("SCIP binary or LICENSE pin mismatch")
            stdout = args.work_dir / "index" / version / "version.stdout"
            run([str(binary), "--version"], REPO, stdout,
                args.work_dir / "index" / version / "version.stderr", 30,
                LOG_CAP, load_limit=args.load_limit)
            clean_repo()
            if read_bounded(stdout, LOG_CAP).decode(errors="replace").splitlines()[:1] != ["scip-clang " + version[1:]]:
                raise Refusal("SCIP binary version mismatch")
            binaries[version] = (binary, license_path)
        phase_stamps = {(slug, version): index_stamp(args, found[slug][0], found[slug][1],
                                                     *binaries[version])
                        for slug, version in expected}
        for slug, version in expected:
            stage = "index_execution"
            subject, compdb, entries, *_ = found[slug]
            binary, license_path = binaries[version]
            index = args.indexes_dir / f"{slug}-{version}.scip"
            partial = index.with_suffix(".scip.partial")
            if partial.exists():
                raise Refusal("partial index already exists")
            log = args.index_logs_dir / f"{slug}-{version}-index.log"
            stdout_log = args.index_logs_dir / f"{slug}-{version}-stdout.log"
            stamp = index_stamp(args, subject, compdb, binary, license_path)
            command = [str(binary), "--compdb-path", str(compdb),
                       "--index-output-path", str(partial), "--jobs", "2",
                       "--no-progress-report", "--log-level", "warning"]
            run(command, subject, stdout_log,
                log, args.timeout, LOG_CAP, load_limit=args.load_limit)
            clean_repo()
            if stamp != index_stamp(args, subject, compdb, binary, license_path):
                raise Refusal("index inputs changed during command")
            digest(log, LOG_CAP)
            digest(stdout_log, LOG_CAP)
            summary = (read_bounded(log, LOG_CAP).decode(errors="replace") + "\n" +
                       read_bounded(stdout_log, LOG_CAP).decode(errors="replace"))
            lines = re.findall(r"^Finished indexing (\d+) translation units .*num errored TUs: (\d+)\)\.$",
                               summary, re.MULTILINE)
            if len(lines) != 1 or (int(lines[0][0]), int(lines[0][1])) != (entries, 0):
                raise Refusal("index population or errors mismatch")
            documents = scip_identity(partial, subject, version)
            partial.replace(index)
            rows.append({"subject": PINS[slug][1], "subject_sha": PINS[slug][0],
                         "version": version, "binary_sha256": INDEX_BINARY[version],
                         "license_sha256": SCIP_LICENSE, "database_sha256": digest(compdb),
                         "tracked_sha256": tracked_manifest(subject),
                         "language_inventory": list(language_inventory(subject)),
                         "compiler_sha256": list(compiler_digests(compdb)),
                         "build_inputs": build_inputs(subject, compdb),
                         "scip_sha256": digest(index), "scip_bytes": index.stat().st_size,
                         "index_log_sha256": digest(log), "stdout_log_sha256": digest(stdout_log),
                         "indexed_units": entries,
                         "errored_units": 0, "documents": documents})
        if any(stamp != index_stamp(args, found[slug][0], found[slug][1], *binaries[version])
               for (slug, version), stamp in phase_stamps.items()):
            raise Refusal("index inputs changed during phase")
        clean_repo()
        state["status"] = "complete"
        state["rows"] = rows
        state.pop("reason", None)
    except Exception:
        state["reason"] = stage
        state["observed"] = [{"subject": row["subject"], "version": row["version"],
                              "indexed_units": row["indexed_units"], "errored_units": row["errored_units"]}
                             for row in rows]
        raise
    finally:
        write_json(receipt, state)


def wait_load(limit, deadline):
    while os.getloadavg()[0] > limit:
        if time.monotonic() >= deadline:
            raise Refusal("load wait timed out")
        time.sleep(1)


def terminate_group(proc):
    # The leader can exit while a child still owns a pipe. Always signal the
    # group, then wait for the group itself to disappear, not just the leader.
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    except PermissionError as error:
        proc.wait(timeout=5)
        raise Refusal("process group termination denied") from error
    deadline = time.monotonic() + 5
    while time.monotonic() < deadline:
        proc.poll()
        try:
            os.killpg(proc.pid, 0)
        except ProcessLookupError:
            break
        except PermissionError:
            # Some sandboxes deny probe signal 0 even after TERM succeeded.
            # The final KILL still runs after the grace period.
            pass
        time.sleep(0.05)
    else:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        except PermissionError as error:
            proc.wait(timeout=5)
            raise Refusal("process group kill denied") from error
    proc.wait()


def run(argv, cwd, stdout_path, stderr_path, timeout, stdout_cap, env=None,
        load_limit=9, load_wait=3600):
    """Drain both pipes with cap+1 detection; kill the entire process group."""
    wait_load(load_limit, time.monotonic() + load_wait)
    stdout_path.parent.mkdir(parents=True, exist_ok=True)
    stderr_path.parent.mkdir(parents=True, exist_ok=True)
    if stdout_path.exists() or stderr_path.exists():
        raise Refusal("subprocess output already exists")
    try:
        proc = subprocess.Popen(argv, cwd=cwd, env=env, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, start_new_session=True,
                                preexec_fn=lambda: os.nice(10))
    except subprocess.SubprocessError as error:
        raise Refusal("subprocess could not start at nice 10") from error
    selector = selectors.DefaultSelector()
    selector.register(proc.stdout, selectors.EVENT_READ, 0)
    selector.register(proc.stderr, selectors.EVENT_READ, 1)
    sizes = [0, 0]
    deadline = time.monotonic() + timeout
    try:
        with stdout_path.open("xb") as out, stderr_path.open("xb") as err:
            streams = (out, err)
            while selector.get_map():
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise Refusal("subprocess timeout")
                for key, _ in selector.select(min(remaining, 1)):
                    chunk = os.read(key.fileobj.fileno(), 65536)
                    if not chunk:
                        selector.unregister(key.fileobj)
                        continue
                    index = key.data
                    sizes[index] += len(chunk)
                    if sizes[index] > (stdout_cap, LOG_CAP)[index]:
                        raise Refusal("subprocess output cap exceeded")
                    streams[index].write(chunk)
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise Refusal("subprocess timeout")
        try:
            code = proc.wait(timeout=remaining)
        except subprocess.TimeoutExpired as error:
            raise Refusal("subprocess timeout") from error
        if code:
            raise Refusal("subprocess failed")
    except (Exception, KeyboardInterrupt):
        terminate_group(proc)
        raise
    finally:
        selector.close()
        proc.stdout.close()
        proc.stderr.close()


def native_inputs(args, found, binaries):
    modules = {name: {p.relative_to(REPO).as_posix(): digest(p)
                      for p in sorted((REPO / module).glob("*.go")) + [REPO / module / "go.mod"]}
               for name, module, _ in ORACLES}
    return {"subjects": {slug: {"sha": PINS[slug][0], "entries": item[2],
                                "tracked_sha256": tracked_manifest(item[0]),
                                "language_inventory": list(language_inventory(item[0])),
                                "compdb_sha256": digest(item[1]),
                                "compiler_sha256": list(compiler_digests(item[1])),
                                "build_inputs": build_inputs(item[0], item[1])}
                         for slug, item in found.items()},
            "index_evidence": index_evidence(args, found),
            "binaries": {name: digest(path) for name, path in binaries.items()},
            "module_sources": modules,
            "clang_sha256": digest(args.clang),
            "driver_sha256": digest(Path(__file__))}


def native_outputs(work):
    return {slug: {filename: digest(work / "native" / slug / filename, JSON_CAP)
                   for _, _, filename in ORACLES} for slug in PINS}


def native_command_stamp(args, slug, subject, compdb, binary, module):
    return (git(subject, "rev-parse", "HEAD"), tracked_manifest(subject), language_inventory(subject),
            digest(compdb), compiler_digests(compdb), build_inputs(subject, compdb),
            digest(args.index_receipt), digest(binary), digest(args.clang),
            tuple((p.name, digest(p)) for p in sorted((REPO / module).glob("*.go"))),
            digest(REPO / module / "go.mod"))


def grade_command_stamp(args, slug, subject, compdb, index):
    return (git(subject, "rev-parse", "HEAD"), tracked_manifest(subject), language_inventory(subject),
            digest(compdb), compiler_digests(compdb), build_inputs(subject, compdb),
            digest(index), digest(args.indexbench), digest(args.ctags),
            digest(args.work_dir / "native" / slug / "preproc.json", JSON_CAP),
            digest(args.work_dir / "native" / slug / "calls.json", JSON_CAP),
            digest(args.work_dir / "native" / slug / "preproc.ranges.json", JSON_CAP),
            digest(args.index_receipt),
            digest(args.index_logs_dir / f"{slug}-{index.stem.split('-')[-1]}-index.log"),
            digest(args.index_logs_dir / f"{slug}-{index.stem.split('-')[-1]}-stdout.log"),
            git(REPO, "rev-parse", "HEAD"))


def build_oracles(args):
    binaries = {}
    env = os.environ.copy()  # parent assigns cache/TMPDIR; never override it
    for name, module, _ in ORACLES:
        binary = args.work_dir / "bin" / name
        run(["bash", str(REPO / "scripts/fathom-go.sh"), "build", "-o", str(binary), "."],
            REPO / module, args.work_dir / "build" / (name + ".stdout"),
            args.work_dir / "build" / (name + ".stderr"), args.timeout, LOG_CAP,
            env=env, load_limit=args.load_limit)
        binaries[name] = binary
    return binaries


def native(args, found):
    clean_repo()
    receipt = args.work_dir / "native" / "receipt.json"
    prior = load_json(receipt) if receipt.exists() else None
    if prior is not None and prior.get("status") != "complete":
        raise Refusal("failed native receipt exists; use a fresh work directory")
    if prior is None:
        write_json(receipt, {"status": "failed", "inputs": None, "outputs": None,
                             "counts": {}, "commands": [module for _, module, _ in ORACLES]})
    counts = {}
    inputs = None
    complete = False
    stage = "build"
    try:
        binaries = ({name: args.work_dir / "bin" / name for name, _, _ in ORACLES}
                    if prior is not None else build_oracles(args))
        inputs = native_inputs(args, found, binaries)
        stage = "input_validation"
        if prior is not None:
            if prior.get("inputs") != inputs or prior.get("outputs") != native_outputs(args.work_dir):
                raise Refusal("native receipt or inputs changed; use a fresh work directory")
            clean_repo()
            complete = True
            return
        if any(digest(args.clang) not in item[3] for item in found.values()):
            raise Refusal("AST clang differs from compilation database compiler")
        for slug, (subject, compdb, entries, *_) in found.items():
            target = args.work_dir / "native" / slug
            target.mkdir(parents=True, exist_ok=True)
            counts[slug] = {}
            for name, module, filename in ORACLES:
                stage = "oracle_" + name
                output = target / filename
                if output.exists() or output.with_suffix(output.suffix + ".partial").exists():
                    raise Refusal("unreceipted native output or partial exists")
                argv = [str(binaries[name]), "-subject", str(subject), "-compdb", str(compdb),
                        "-tier", "A"]
                if name != "preproc":
                    argv.extend(["-jobs", "2"])
                if name == "ranges":
                    argv.extend(["-clang", str(args.clang)])
                partial = output.with_suffix(output.suffix + ".partial")
                stamp = native_command_stamp(args, slug, subject, compdb, binaries[name], module)
                try:
                    run(argv, REPO, partial, target / (name + ".stderr"), args.timeout,
                        JSON_CAP, load_limit=args.load_limit)
                    clean_repo()
                    if stamp != native_command_stamp(args, slug, subject, compdb, binaries[name], module):
                        raise Refusal("native inputs changed during command")
                    data = load_json(partial)
                    if not isinstance(data, dict) or data.get("tier") != "A" or not data.get("tool"):
                        raise Refusal("native oracle shape or tier mismatch")
                    if name == "ranges":
                        counts[slug][name] = {field: data.get(field) for field in ("units", "errored", "skipped")}
                        if (data.get("errored") != 0 or not isinstance(data.get("units"), int) or
                                not isinstance(data.get("skipped"), int) or
                                data["units"] <= 0 or data["units"] + data["skipped"] != entries):
                            raise Refusal("AST range oracle population or errors mismatch")
                    else:
                        summary = read_bounded(target / (name + ".stderr"), LOG_CAP).decode(errors="replace")
                        pattern = (r"^translation_units (\d+) stopped (\d+) files (\d+) includes_in_subject (\d+)$"
                                   if name == "preproc" else
                                   r"^translation_units (\d+) stopped (\d+) sites (\d+)$")
                        matches = [re.fullmatch(pattern, line) for line in summary.splitlines()]
                        matches = [match for match in matches if match]
                        if len(matches) != 1:
                            raise Refusal("native oracle summary absent")
                        counts[slug][name] = {"units": int(matches[0][1]), "stopped": int(matches[0][2])}
                        if counts[slug][name]["stopped"] or counts[slug][name]["units"] <= 0:
                            raise Refusal("native oracle stopped or zero units")
                    partial.replace(output)
                except Exception:
                    partial.unlink(missing_ok=True)
                    raise
        if inputs != native_inputs(args, found, binaries):
            raise Refusal("native inputs changed during phase")
        stage = "finalize"
        clean_repo()
        write_json(receipt, {"status": "complete", "inputs": inputs,
                             "commands": [module for _, module, _ in ORACLES],
                             "outputs": native_outputs(args.work_dir), "counts": counts})
        complete = True
    finally:
        if not complete and prior is None:
            write_json(receipt, {"status": "failed", "inputs": inputs, "outputs": None,
                                 "counts": counts, "reason": stage,
                                 "commands": [module for _, module, _ in ORACLES]})


def grade_inputs(args, found):
    native_receipt = load_json(args.work_dir / "native" / "receipt.json")
    binaries = {name: args.work_dir / "bin" / name for name, _, _ in ORACLES}
    if (native_receipt.get("status") != "complete" or
            native_receipt.get("inputs") != native_inputs(args, found, binaries) or
            native_receipt.get("outputs") != native_outputs(args.work_dir)):
        raise Refusal("native outputs lack a matching complete receipt")
    indexed = index_evidence(args, found)
    tools = grade_binary_identity(args)
    return {"native_inputs": native_receipt["inputs"],
            "native_outputs": native_receipt["outputs"],
            "compdb_sha256": {slug: digest(item[1]) for slug, item in found.items()},
            "index_evidence": indexed, "indexbench_sha256": digest(args.indexbench),
            "ctags_sha256": digest(args.ctags),
            "tool_identity": tools,
            "driver_sha256": digest(Path(__file__))}


def grade_binary_identity(args):
    result = subprocess.run(["bash", str(REPO / "scripts/fathom-go.sh"), "version", "-m",
                             str(args.indexbench)], cwd=REPO, capture_output=True,
                            timeout=30, check=False)
    if result.returncode or len(result.stdout) > 65536:
        raise Refusal("indexbench Go build identity unavailable")
    fields = [line.strip().split(None, 1) for line in result.stdout.decode(errors="replace").splitlines()]
    module = [parts[1] for parts in fields if len(parts) == 2 and parts[0] == "path"]
    revision = [parts[1].split("=", 1)[1] for parts in fields
                if len(parts) == 2 and parts[0] == "build" and parts[1].startswith("vcs.revision=")]
    modified = [parts[1].split("=", 1)[1] for parts in fields
                if len(parts) == 2 and parts[0] == "build" and parts[1].startswith("vcs.modified=")]
    head = git(REPO, "rev-parse", "HEAD")
    if (module != ["github.com/geoffmilleraz/fathom/cmd/indexbench"] or
            revision != [head] or modified != ["false"]):
        raise Refusal("indexbench is not the clean committed HEAD build")
    ctags = subprocess.run([str(args.ctags), "--version"], capture_output=True,
                           timeout=30, check=False)
    if ctags.returncode or len(ctags.stdout) > 65536 or \
            not ctags.stdout.decode(errors="replace").splitlines()[0:1] or \
            not ctags.stdout.decode(errors="replace").splitlines()[0].startswith("Universal Ctags 6.2.1,"):
        raise Refusal("universal-ctags 6.2.1 identity unavailable")
    return {"indexbench": {"source": "cmd/indexbench", "revision": head,
                           "sha256": digest(args.indexbench), "modified": False},
            "ctags": {"version": "6.2.1", "sha256": digest(args.ctags)}}


def validate_cpp_reading(reading, version):
    if not isinstance(reading, dict) or reading.get("dialect") != "cpp":
        raise Refusal("reading dialect mismatch")
    subjects = reading.get("subjects")
    expected_families = {"l1": "B", "calls": "A", "defmap": "A",
                         "file_edges": "A", "imports": "A", "typed": "A"}
    if not isinstance(subjects, list) or len(subjects) != len(PINS):
        raise Refusal("reading subject identity or coverage mismatch")
    seen = set()
    for subject in subjects:
        if not isinstance(subject, dict):
            raise Refusal("reading subject identity or coverage mismatch")
        # A native companion adds its family only when this subject pairs an
        # eligible range. L1 still caps the aggregate range metric at B below.
        families = subject.get("families")
        allowed_families = dict(expected_families)
        if isinstance(families, dict) and "ranges" in families:
            allowed_families["ranges"] = "A"
        name = subject.get("name")
        slug = next((key for key, pin in PINS.items() if pin[1] == name), None)
        if slug is None or slug in seen or subject.get("sha") != PINS[slug][0] or \
                families != allowed_families or any(
                    isinstance(subject.get(field), bool) or
                    not isinstance(subject.get(field), int) or subject[field] <= 0
                    for field in ("files", "bytes")):
            raise Refusal("reading subject identity or coverage mismatch")
        seen.add(slug)
    oracles = reading.get("oracles")
    if not isinstance(oracles, list):
        raise Refusal("reading oracle provenance absent")
    by_tool = {}
    for oracle in oracles:
        if not isinstance(oracle, dict) or not isinstance(oracle.get("tool"), str):
            raise Refusal("reading oracle provenance malformed")
        by_tool.setdefault(oracle["tool"], []).append(oracle)
    expected = {"universal-ctags": ("B", "6.2.1"),
                "scip-clang": ("A", version[1:]),
                "clang-ast": ("A", None), "clang-preprocessor": ("A", None)}
    for name, (tier, release) in expected.items():
        found = by_tool.get(name, [])
        if len(found) != 1 or found[0].get("tier") != tier or \
                not isinstance(found[0].get("version"), str) or not found[0]["version"] or \
                (release is not None and found[0]["version"] != release):
            raise Refusal("reading oracle identity or tier mismatch")
    metrics = reading.get("metrics")
    if not isinstance(metrics, dict):
        raise Refusal("reading metrics absent")
    tier_a = ("def_mapping", "typed_recall", "file_edge_precision", "file_edge_recall",
              "call_precision", "call_recall", "import_resolution")
    tier_b = ("sym_precision", "sym_recall", "range_agreement")
    for name in tier_a + tier_b:
        value = metrics.get(name)
        if isinstance(value, bool) or not isinstance(value, (float, int)) or not math.isfinite(value):
            raise Refusal("required C++ metric unavailable")
    tiers = reading.get("metric_tiers")
    if not isinstance(tiers, dict) or any(tiers.get(name) != "B" for name in tier_b) or \
            any(tiers.get(name) != "A" for name in tier_a):
        raise Refusal("C++ metric tier mismatch")


def grade_command(args, slug, subject, index, output, unmatched, scope, commit):
    cmd = [str(args.indexbench), "lang-oracle", "-dialect", "cpp", "-subject", str(subject),
           "-subject-name", PINS[slug][1], "-ctags", str(args.ctags),
           "-scip", str(index), "-scip-tier", "A", "-calls-from", "scip",
           "-tool-oracle", str(args.work_dir / "native" / slug / "preproc.json"),
           "-tool-oracle", str(args.work_dir / "native" / slug / "calls.json"),
           "-unmatched", str(unmatched), "-out", str(output), "-date", DATE,
           "-commit", commit]
    if scope == "legacy":
        cmd.extend(["-only", PINS[slug][2]])
    return cmd


def grade(args, found):
    clean_repo()
    if args.readings_dir is None:
        raise Refusal("--readings-dir is required for grade")
    commit = git(REPO, "rev-parse", "HEAD")
    outputs = {f"cpp-{scope}-{version}.json": None for scope in ("legacy", "full")
               for version in ("v0.4.0", "v0.4.1")}
    receipt = args.readings_dir / "cpp-audit-receipt.json"
    if receipt.exists() or any((args.readings_dir / name).exists() for name in outputs):
        raise Refusal("published receipt or reading already exists")
    args.readings_dir.mkdir(parents=True, exist_ok=True)
    published = {"status": "failed", "date": DATE, "commit": commit,
                 "inputs": None, "commands": ["cmd/indexbench lang-oracle -dialect cpp",
                                              *[module for _, module, _ in ORACLES]],
                 "readings": outputs, "reason": "input_validation"}
    write_json(receipt, published)
    try:
        published["inputs"] = grade_inputs(args, found)
        for scope in ("legacy", "full"):
            for version in ("v0.4.0", "v0.4.1"):
                stem = f"cpp-{scope}-{version}"
                target = args.work_dir / "grade" / stem
                reading = target / "reading.json"
                if reading.exists() or (target / "retime.stdout").exists():
                    raise Refusal("stale grade or retime output exists")
                for slug, (subject, compdb, *_) in found.items():
                    published["reason"] = "grade_command"
                    index = args.indexes_dir / f"{slug}-{version}.scip"
                    cmd = grade_command(args, slug, subject, index, reading,
                                        target / slug / "unmatched.json", scope, commit)
                    stamp = grade_command_stamp(args, slug, subject, compdb, index)
                    run(cmd, REPO, target / slug / "stdout.log", target / slug / "stderr.log",
                        args.timeout, LOG_CAP, load_limit=args.load_limit)
                    clean_repo()
                    if stamp != grade_command_stamp(args, slug, subject, compdb, index):
                        raise Refusal("grade inputs changed during command")
                published["reason"] = "reading_validation"
                result = load_json(reading, READING_CAP)
                if {s.get("name") for s in result.get("subjects", [])} != {pin[1] for pin in PINS.values()} or \
                        len(result["subjects"]) != 3:
                    raise Refusal("reading has wrong subject identity")
                validate_cpp_reading(result, version)
                before = grade_inputs(args, found)
                published["reason"] = "retime_command"
                retime = [str(args.indexbench), "lang-retime", "-reading", str(reading),
                          "-runs", "3", "-write"]
                for slug, (subject, *_rest) in found.items():
                    retime.extend(["-subject", PINS[slug][1] + "=" + str(subject)])
                run(retime, REPO, target / "retime.stdout", target / "retime.stderr",
                    args.timeout, LOG_CAP, load_limit=args.load_limit)
                clean_repo()
                if before != grade_inputs(args, found):
                    raise Refusal("retime inputs changed during command")
                published["reason"] = "retime_validation"
                retimed = load_json(reading, READING_CAP)
                if {s.get("name") for s in retimed.get("subjects", [])} != {pin[1] for pin in PINS.values()} or \
                        len(retimed["subjects"]) != 3:
                    raise Refusal("retime changed subject identity")
                validate_cpp_reading(retimed, version)
                summary = read_bounded(target / "retime.stdout", LOG_CAP).decode(errors="replace")
                runs = re.findall(r"^run ([123]): ([0-9]+(?:\.[0-9]+)?) MB/CPU-s$", summary, re.MULTILINE)
                median = re.findall(r"^median run ([123]): ([0-9]+(?:\.[0-9]+)?) MB/CPU-s$",
                                    summary, re.MULTILINE)
                if len(runs) != 3 or len(median) != 1 or [int(x[0]) for x in runs] != [1, 2, 3]:
                    raise Refusal("retime summary incomplete")
                final = args.readings_dir / f"{stem}.json"
                clean_repo()
                with reading.open("rb") as source, final.open("xb") as destination:
                    shutil.copyfileobj(source, destination)
                published["readings"][final.name] = {"sha256": digest(final, READING_CAP),
                    "subjects": 3, "median_run": int(median[0][0]),
                    "throughput_mb_cpu_s": float(median[0][1]),
                    "run_rates_mb_cpu_s": [float(x[1]) for x in runs]}
        if published["inputs"] != grade_inputs(args, found):
            raise Refusal("grade inputs changed during phase")
        clean_repo()
        published["status"] = "complete"
        published.pop("reason", None)
    except Exception:
        for name in outputs:
            (args.readings_dir / name).unlink(missing_ok=True)
        published["readings"] = outputs
        raise
    finally:
        write_json(receipt, published)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=("index", "native", "grade"))
    for name in ("subjects-root", "builds-root", "work-dir", "lock-dir", "indexes-dir"):
        parser.add_argument("--" + name, required=True, type=Path)
    for name in ("indexbench", "ctags", "clang"):
        parser.add_argument("--" + name, type=Path)
    parser.add_argument("--index-receipt", type=Path)
    parser.add_argument("--index-logs-dir", type=Path)
    parser.add_argument("--scip-binaries-root", type=Path)
    parser.add_argument("--readings-dir", type=Path)
    parser.add_argument("--timeout", type=int, default=7200)
    parser.add_argument("--load-limit", type=float, default=9.0)
    args = parser.parse_args(argv)
    if args.timeout <= 0 or not 0 < args.load_limit <= 9:
        parser.error("timeout must be positive and load limit at most 9")
    try:
        for name in ("subjects_root", "builds_root", "work_dir", "lock_dir", "indexes_dir"):
            setattr(args, name, getattr(args, name).resolve())
        args.index_logs_dir = (args.index_logs_dir or args.indexes_dir / "logs").resolve()
        args.index_receipt = (args.index_receipt or args.indexes_dir / "candidate-indexes.json").resolve()
        if args.scip_binaries_root is not None:
            args.scip_binaries_root = args.scip_binaries_root.resolve()
    except (OSError, RuntimeError):
        parser.error("input or output path cannot be resolved")
    if not under(args.index_logs_dir, args.indexes_dir) or not under(args.index_receipt, args.indexes_dir):
        parser.error("index logs and receipt must be under indexes directory")
    needed = {"index": (), "native": ("clang",), "grade": ("indexbench", "ctags", "clang")}[args.phase]
    for name in needed:
        if getattr(args, name) is None:
            parser.error("--" + name + " is required for " + args.phase)
    for name in ("indexbench", "ctags", "clang"):
        if getattr(args, name) is None:
            continue
        try:
            path = getattr(args, name).resolve(strict=True)
        except (OSError, RuntimeError):
            parser.error("executable input missing")
        if not path.is_file() or not os.access(path, os.X_OK):
            parser.error("executable input missing")
        setattr(args, name, path)
    if args.readings_dir is not None:
        try:
            args.readings_dir = args.readings_dir.resolve()
        except (OSError, RuntimeError):
            parser.error("reading output path cannot be resolved")
    confined_output_paths(args, parser)
    args.lock_dir.parent.mkdir(parents=True, exist_ok=True)
    try:
        args.lock_dir.mkdir()  # common protocol: mkdir is the exclusive acquisition
    except FileExistsError:
        print("cpp-audit: shared lock already held", file=sys.stderr)
        return 1
    try:
        args.work_dir.mkdir(parents=True, exist_ok=True)
        try:
            clean_repo()
            found = subjects(args)
            if args.phase == "index":
                index_phase(args, found)
            elif args.phase == "native":
                native(args, found)
            else:
                grade(args, found)
        except (Refusal, OSError, ValueError, subprocess.TimeoutExpired,
                subprocess.SubprocessError, json.JSONDecodeError):
            if args.phase == "native":
                failure = args.work_dir / "native" / "receipt.json"
                if not failure.exists():
                    write_json(failure, {"status": "failed", "inputs": None,
                        "outputs": None, "counts": {}, "reason": "preflight"})
            if args.phase == "index" and not args.index_receipt.exists():
                write_json(args.index_receipt, {"status": "failed",
                    "producer": "calibration/oracles/scip-clang/cpp-audit.py",
                    "driver_sha256": digest(Path(__file__)), "rows": None,
                    "reason": "preflight"})
            if args.phase == "grade" and args.readings_dir is not None:
                failure = args.readings_dir / "cpp-audit-receipt.json"
                if not failure.exists():
                    nulls = {f"cpp-{scope}-{version}.json": None
                             for scope in ("legacy", "full") for version in ("v0.4.0", "v0.4.1")}
                    write_json(failure, {"status": "failed", "date": DATE, "commit": None,
                                         "inputs": None, "readings": nulls, "reason": "preflight"})
            # Details and paths remain in ignored logs. The terminal and
            # published receipt receive only this finite reason category.
            print("cpp-audit: refused or failed; inspect private work logs", file=sys.stderr)
            return 1
    finally:
        args.lock_dir.rmdir()
    return 0


if __name__ == "__main__":
    sys.exit(main())
