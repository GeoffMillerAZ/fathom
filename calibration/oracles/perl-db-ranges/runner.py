#!/usr/bin/env python3
"""Perl 5.34.1 compile-only, compiler-reported named-sub range census.

Produces a neutral compiler tier-B range companion. This is independent of
Fathom output; range quality still requires full-corpus measurement.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import tempfile
import time

EXTENSIONS = {".pm", ".pl", ".t", ".psgi", ".plx", ".perl"}
TOTAL_SECONDS = 600
PER_FILE_SECONDS = 2
OBSERVER = Path(__file__).resolve().with_name("observer.pl")


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], text=True).strip()


def files_under(root):
    files = []
    tracked = subprocess.check_output(["git", "-C", str(root), "ls-files", "-z"]).split(b"\0")
    for raw in sorted(name for name in tracked if name):
        rel = os.fsdecode(raw)
        path = root / rel
        try:
            resolved = path.resolve(strict=True)
        except (OSError, RuntimeError) as exc:
            raise ValueError("broken tracked symlink or missing source is not supported") from exc
        if not resolved.is_relative_to(root):
            raise ValueError("tracked symlink or outside source is not supported")
        if path.is_symlink():
            # Git tracks this directory entry, not files below it. A link to
            # an in-tree directory is inert for the tracked-file manifest.
            if resolved.is_dir():
                continue
            raise ValueError("tracked source-file symlink is not supported")
        if not path.is_file():
            continue
        if path.suffix.lower() in EXTENSIONS:
            files.append(rel)
        elif not path.suffix:
            with path.open("rb") as source:
                first = source.readline()
            if first.startswith(b"#!") and re.search(rb"\bperl\b", first):
                files.append(rel)
    return sorted(files)


def snapshot(root):
    if git(root, "rev-parse", "--show-toplevel") != str(root):
        raise ValueError("subject must be the Git repository root")
    commit = git(root, "rev-parse", "HEAD")
    if len(commit) != 40 or git(root, "status", "--porcelain", "--untracked-files=all"):
        raise ValueError("subject must have a clean, full-commit Git checkout")
    entries = [{"path": rel, "sha256": sha256((root / rel).read_bytes())} for rel in files_under(root)]
    encoded = json.dumps(entries, sort_keys=True, separators=(",", ":")).encode()
    return {"commit": commit, "files": entries, "digest": sha256(encoded)}


def wait_for_load(deadline):
    while os.getloadavg()[0] > 9:
        if time.monotonic() >= deadline:
            return False
        time.sleep(min(5, max(0.1, deadline - time.monotonic())))
    return True


def stop_group(proc):
    def signal_group(sig):
        try:
            os.killpg(proc.pid, sig)
        except (ProcessLookupError, PermissionError):
            # Darwin may report EPERM for a group containing only an unreaped
            # zombie. Always reap the leader, and try SIGKILL independently.
            return False
        return True

    signal_group(signal.SIGTERM)
    end = time.monotonic() + .3
    while time.monotonic() < end:
        proc.poll()  # reaps a finished leader before Darwin's group probe
        try:
            os.killpg(proc.pid, 0)
        except (ProcessLookupError, PermissionError):
            break
        time.sleep(.01)
    # A parent can exit while a SIGTERM-ignoring descendant remains alive.
    # SIGKILL is independent of the leader's poll/wait status.
    signal_group(signal.SIGKILL)
    try:
        proc.wait(timeout=1)
    except subprocess.TimeoutExpired:
        signal_group(signal.SIGKILL)
        proc.wait(timeout=1)


def observe(perl, root, rel, raw_out, deadline):
    if not wait_for_load(deadline):
        return None, "total_timeout_waiting_for_load"
    env = os.environ.copy()
    env["PERL5DB"] = 'BEGIN { require "' + str(OBSERVER).replace('\\', '\\\\').replace('"', '\\"') + '" }'
    env["FATHOM_DB_RANGE_OUT"] = str(raw_out)
    # No app body execution: -c compiles only, but BEGIN/CHECK/use can execute.
    proc = subprocess.Popen(
        ["nice", "-n", "10", perl, "-d", "-c", "-I" + str(root / "lib"), rel],
        cwd=root, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE,
        start_new_session=True,
    )
    try:
        try:
            proc.communicate(timeout=min(PER_FILE_SECONDS, max(0.01, deadline - time.monotonic())))
        except subprocess.TimeoutExpired:
            return None, "file_timeout"
    finally:
        stop_group(proc)
    if proc.returncode or not raw_out.exists():
        # Store no raw diagnostics or host paths in the neutral artifact.
        return None, "compile_error" if proc.returncode else "missing_observer_output"
    try:
        data = json.loads(raw_out.read_text())
        if not isinstance(data, dict):
            raise ValueError("observer output not an object")
        return data, None
    except (OSError, json.JSONDecodeError, ValueError):
        return None, "invalid_observer_output"


def collect_ranges(root, rel, records):
    result = []
    target = (root / rel).resolve()
    for name, value in records.items():
        if not isinstance(name, str) or not isinstance(value, str) or "__ANON__" in name:
            continue
        match = re.fullmatch(r"(.+):(\d+)-(\d+)", value)
        if not match:
            continue
        raw_path, start, end = match.groups()
        if raw_path.startswith("(") or not Path(raw_path).is_absolute() and (root / raw_path).resolve() != target:
            continue
        if Path(raw_path).is_absolute() and Path(raw_path).resolve() != target:
            continue
        first, last = int(start), int(end)
        if first < 1 or last < first or not re.fullmatch(r"[A-Za-z_]\w*(?:::[A-Za-z_]\w*)*", name):
            continue
        result.append({"path": rel, "name": name, "kind": "function", "line": first, "end": last})
    return result


def ctags_identity(binary):
    path = Path(binary).resolve()
    if not path.is_file():
        raise ValueError("ctags executable is missing")
    version = subprocess.check_output([str(path), "--version"], text=True, timeout=2).splitlines()[0]
    if not re.match(r"^Universal Ctags 6\.2\.1(?:\b|\()", version):
        raise ValueError("ctags must be Universal Ctags 6.2.1")
    return {"version": version, "binary_sha256": sha256(path.read_bytes())}


def ctags_subs(binary, root, rel, deadline):
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        raise subprocess.TimeoutExpired([binary], 0)
    command = [binary, "--output-format=json", "--fields=+nKzSe",
               "--extras=-{anonymous}", "-f", "-", "--languages=Perl",
               "--map-Perl=+.t", "--map-Perl=+.psgi", "--language-force=Perl", rel]
    result = subprocess.run(command, cwd=root, capture_output=True, text=True,
                            timeout=remaining, check=True)
    target = (root / rel).resolve()
    tags = []
    for line in result.stdout.splitlines():
        item = json.loads(line)
        if item.get("_type") != "tag" or item.get("kind") != "subroutine":
            continue
        name, number, raw_path = item.get("name"), item.get("line"), item.get("path")
        if not isinstance(name, str) or not isinstance(number, int) or number < 1 or not isinstance(raw_path, str):
            raise ValueError("invalid ctags Perl subroutine tag")
        if (root / raw_path).resolve() != target:
            raise ValueError("ctags subroutine outside the source file")
        tags.append({"name": name, "line": number})
    return tags


def bridge(native, tags):
    """Return uniquely joined ranges and diagnostic counts; never invent an end."""
    choices = []
    reverse = [[] for _ in tags]
    for ni, row in enumerate(native):
        terminal = row["name"].rsplit("::", 1)[-1]
        matches = [ti for ti, tag in enumerate(tags)
                   if abs(row["line"] - tag["line"]) <= 2
                   and tag["name"] in (row["name"], terminal)]
        choices.append(matches)
        for ti in matches:
            reverse[ti].append(ni)
    ranges, diagnostics = [], {"paired": 0, "unmatched": 0, "ambiguous": 0,
                               "tag_total": len(tags), "tag_unpaired": 0, "native": []}
    for ni, row in enumerate(native):
        candidates = choices[ni]
        if not candidates:
            status = "unmatched"
        elif len(candidates) != 1 or len(reverse[candidates[0]]) != 1:
            status = "ambiguous"
        else:
            status = "paired"
            tag = tags[candidates[0]]
            ranges.append({"path": row["path"], "name": tag["name"],
                           "kind": "function", "line": tag["line"], "end": row["end"]})
        diagnostics[status] += 1
        diagnostics["native"].append({"path": row["path"], "compiler_key": row["name"],
                                      "native_start": row["line"], "native_end": row["end"],
                                      "join": status})
    diagnostics["tag_unpaired"] = len(tags) - diagnostics["paired"]
    return ranges, diagnostics


def new_joining():
    return {"paired": 0, "unmatched": 0, "ambiguous": 0,
            "tag_total": 0, "tag_unpaired": 0, "ctags_unknown_files": 0,
            "compiler_unknown_files": 0, "files": [], "native": []}


def record_file(joining, rel, tags, native):
    """Account for the independent ctags denominator even if compilation fails."""
    if tags is None:
        joining["ctags_unknown_files"] += 1
    else:
        joining["tag_total"] += len(tags)
    if native is None:
        joining["compiler_unknown_files"] += 1
    if tags is not None and native is not None:
        paired, details = bridge(native, tags)
        for key in ("paired", "unmatched", "ambiguous"):
            joining[key] += details[key]
        joining["native"].extend(details["native"])
    else:
        paired = []
        if native is not None:
            joining["native"].extend({"path": row["path"], "compiler_key": row["name"],
                                      "native_start": row["line"], "native_end": row["end"],
                                      "join": "ctags_unavailable"} for row in native)
    unpaired = None if tags is None else len(tags) - len(paired)
    if unpaired is not None:
        joining["tag_unpaired"] += unpaired
    joining["files"].append({"path": rel, "tag_total": None if tags is None else len(tags),
                             "tag_unpaired": unpaired,
                             "ctags_status": "unknown" if tags is None else "ok",
                             "compiler_status": "unknown" if native is None else "ok"})
    return paired


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--perl", required=True, help="pinned Perl 5.34.1 executable")
    parser.add_argument("--ctags", required=True, help="pinned Universal Ctags 6.2.1 executable")
    parser.add_argument("--subject", required=True, help="name=/clean/git/root")
    parser.add_argument("--out", required=True)
    parser.add_argument("--log-dir", required=True, help="workspace-only raw per-file observer outputs")
    args = parser.parse_args()
    name, sep, root_text = args.subject.partition("=")
    if not sep or not name or not root_text:
        parser.error("--subject must be NAME=ROOT")
    root = Path(root_text).resolve()
    out = Path(args.out)
    log_dir = Path(args.log_dir)
    meta = out.with_name(out.stem + ".meta.json")
    if out.exists() or meta.exists() or log_dir.exists():
        parser.error("output, metadata, and log directory must not exist")
    version = subprocess.check_output([args.perl, "-e", "print $^V"], text=True).strip()
    if version != "v5.34.1":
        parser.error("Perl executable must be exactly 5.34.1")
    perl_binary_sha256 = sha256(Path(args.perl).resolve().read_bytes())
    ctags = ctags_identity(args.ctags)
    before = snapshot(root)
    log_dir.mkdir(parents=True)
    deadline = time.monotonic() + TOTAL_SECONDS
    ranges, failures = [], []
    joining = new_joining()
    for number, entry in enumerate(before["files"]):
        rel = entry["path"]
        raw = log_dir / f"{number:06d}.json"
        file_deadline = min(deadline, time.monotonic() + PER_FILE_SECONDS)
        reasons = []
        try:
            tags = ctags_subs(str(Path(args.ctags).resolve()), root, rel, file_deadline)
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired, OSError, ValueError, json.JSONDecodeError):
            tags = None
            reasons.append("ctags_error_or_timeout")
        data, failure = observe(args.perl, root, rel, raw, file_deadline)
        if failure:
            reasons.append(failure)
        native = None if failure else collect_ranges(root, rel, data)
        ranges.extend(record_file(joining, rel, tags, native))
        if reasons:
            failures.append({"path": rel, "reasons": reasons})
        if time.monotonic() >= deadline:
            for later in before["files"][number + 1:]:
                record_file(joining, later["path"], None, None)
                failures.append({"path": later["path"], "reasons": ["total_timeout_unattempted"]})
            break
    if snapshot(root) != before:
        raise RuntimeError("subject source manifest changed during run")
    ranges.sort(key=lambda r: (r["path"], r["line"], r["name"], r["end"]))
    payload = {
        "tool": "perl-DB-sub-compile", "version": version, "tier": "B",
        "units": len(before["files"]), "errored": len(failures),
        "kinds": {"function": "function"},
        "scope": "Named subroutines compiled in their own source file; end line from compiler %DB::sub. Name and declaration line join to independent Universal Ctags Perl subroutine tags in the same file, within two lines, only when reciprocal unique. Anonymous/eval/imported-outside-source and ambiguous joins excluded. Compilation failures remain in units and errored. -c omits program body but BEGIN/CHECK/use may execute.",
        "ranges": ranges,
    }
    metadata = {
        "subject": name, "source_manifest": before, "failures": failures,
        "perl_binary_sha256": perl_binary_sha256,
        "ctags": ctags, "joining": joining,
        "tier_status": "compiler-dump B by existing tier model; measured range quality pending",
    }
    pending = []
    try:
        for target, data in ((out, payload), (meta, metadata)):
            with tempfile.NamedTemporaryFile("w", dir=target.parent, prefix=".perl-range-", delete=False) as tmp:
                json.dump(data, tmp, indent=2, sort_keys=True)
                tmp.write("\n")
                pending.append((Path(tmp.name), target))
        for temporary, target in pending:
            temporary.replace(target)
    except BaseException:
        for temporary, target in pending:
            temporary.unlink(missing_ok=True)
            target.unlink(missing_ok=True)
        raise


if __name__ == "__main__":
    main()
