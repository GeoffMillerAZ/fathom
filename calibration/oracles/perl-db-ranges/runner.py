#!/usr/bin/env python3
"""Sandboxed Perl 5.34.1 compile-only named-sub range census (C-R9)."""

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

from dependencies import library_fingerprint

EXTENSIONS = {".pm", ".pl", ".t", ".psgi", ".plx", ".perl"}
TOTAL_SECONDS = 600
PER_FILE_SECONDS = 2
OBSERVER = Path(__file__).resolve().with_name("observer.pl")
SANDBOX_EXEC = Path("/usr/bin/sandbox-exec")
SANDBOX_PROFILE = "(version 1) (allow default) (deny network*)"
NICE_BINARY = Path("/usr/bin/nice")
PERL_ENV_KEYS = ("PERL5LIB", "PERLLIB", "PERL5OPT")
LOCALE_ENV = ("LC_ALL", "LC_CTYPE", "LANG")


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def perl_environment(perl_lib=None):
    """Drop ambient Perl code/dependency injection; the observer adds locked paths."""
    env = os.environ.copy()
    for key in PERL_ENV_KEYS:
        env.pop(key, None)
    for key in LOCALE_ENV:
        env[key] = "C"
    return env


def sandbox_command(perl, *args, profile=None):
    if not SANDBOX_EXEC.is_file() or not os.access(SANDBOX_EXEC, os.X_OK):
        raise RuntimeError("C-R9 requires executable /usr/bin/sandbox-exec; refusing unsandboxed Perl")
    return [str(SANDBOX_EXEC), "-p", profile or SANDBOX_PROFILE,
            str(NICE_BINARY), "-n", "10", perl, *args]


def verify_network_denied(perl, env):
    """Fail closed unless the fixed sandbox makes a loopback TCP connect fail EPERM."""
    control = (
        "use strict; use warnings; use Socket; "
        "socket(my $s, PF_INET, SOCK_STREAM, getprotobyname('tcp')) or die \"socket:$!\"; "
        "my $ok = connect($s, sockaddr_in(1, inet_aton('127.0.0.1'))); "
        "die 'loopback TCP unexpectedly connected' if $ok; "
        "die \"loopback TCP was not denied with EPERM: $!\" unless 0 + $! == 1;"
    )
    result = subprocess.run(sandbox_command(perl, "-e", control), env=env,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=5)
    if result.returncode:
        raise RuntimeError("C-R9 network-denial control failed: " + result.stderr.strip())
    return {"destination": "127.0.0.1:1/tcp", "denial": "EPERM", "profile": SANDBOX_PROFILE}


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


def observe(perl, root, rel, raw_out, deadline, env, inc_context):
    if not wait_for_load(deadline):
        return None, "total_timeout_waiting_for_load"
    env = env.copy()
    env["PERL5DB"] = 'BEGIN { require "' + str(OBSERVER).replace('\\', '\\\\').replace('"', '\\"') + '" }'
    env["FATHOM_DB_RANGE_OUT"] = str(raw_out)
    env["FATHOM_ALLOWED_INC"] = os.pathsep.join(inc_context["inc"])
    env["FATHOM_ALLOWED_ROOTS"] = os.pathsep.join(inc_context["roots"])
    env["FATHOM_PATH_SEP"] = os.pathsep
    # No app body execution: -c compiles only, but BEGIN/CHECK/use can execute.
    proc = subprocess.Popen(
        sandbox_command(perl, "-d", "-c", "-I" + str(root / "lib"), rel,
                        profile=inc_context["filesystem_read_profile"]),
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


def perl_inc_context(perl, root, perl_lib, env):
    code = (
        "use Config; use JSON::PP; "
        "print JSON::PP->new->encode({privlibexp=>$Config{privlibexp},"
        "archlibexp=>$Config{archlibexp},archname=>$Config{archname},"
        "config_module=>$INC{'Config.pm'}});"
    )
    raw = subprocess.check_output(sandbox_command(perl, "-e", code), env=env,
                                  text=True, timeout=5)
    config = json.loads(raw)
    core = [Path(config[key]).resolve(strict=True) for key in ("privlibexp", "archlibexp")]
    core_module_identity(config["config_module"], core)
    forbidden = [Path("/Library/Perl"), Path("/System/Library/Perl/Extras")]
    if any(path == root_path or path.is_relative_to(root_path) for path in core for root_path in forbidden):
        raise ValueError("Perl Config core roots overlap a forbidden site/Extras Perl tree")
    subject_lib_path = root / "lib"
    if subject_lib_path.is_symlink():
        raise ValueError("subject lib directory must not be a symlink")
    subject_lib = subject_lib_path.resolve(strict=True) if subject_lib_path.exists() else subject_lib_path
    if not subject_lib.is_relative_to(root):
        raise ValueError("subject lib directory escapes the checkout")
    inc = [*map(str, core), str(subject_lib)]
    roots = [*map(str, core), str(root)]
    if perl_lib:
        local = perl_lib.resolve(strict=True)
        if any(local == root_path or local.is_relative_to(root_path) for root_path in forbidden):
            raise ValueError("local Perl library overlaps a forbidden site/Extras Perl tree")
        arch = local / config["archname"]
        if arch.is_symlink():
            raise ValueError("local Perl architecture directory must not be a symlink")
        inc.append(str(local))
        roots.append(str(local))
        if arch.is_dir():
            inc.append(str(arch))
            roots.append(str(arch))
    interpreter = Path(perl).resolve(strict=True)
    observer = OBSERVER.resolve(strict=True)
    exact_files = [interpreter, observer, NICE_BINARY.resolve(strict=True), Path("/"),
                   Path("/dev/null"), Path("/dev/urandom"), Path("/dev/random")]
    allowed_dirs = [*core, root, Path("/usr/lib"), Path("/System/Library/dyld"),
                    Path("/System/Library/Frameworks"),
                    Path("/System/Cryptexes/OS/System/Library/dyld"),
                    Path("/System/Volumes/Preboot/Cryptexes/OS/System/Library/dyld")]
    if perl_lib:
        allowed_dirs.append(perl_lib.resolve(strict=True))
    allow_terms = [f'(literal {json.dumps(str(path))})' for path in exact_files]
    allow_terms.extend(f'(subpath {json.dumps(str(path))})' for path in allowed_dirs)
    filesystem_read_profile = (
        "(version 1) (allow default) (deny network*) "
        "(deny file-read-data (require-not (require-any "
        + " ".join(allow_terms) + ")))"
    )
    return {
        "inc": inc,
        "roots": roots,
        "core": list(map(str, core)),
        "filesystem_read_profile": filesystem_read_profile,
        "filesystem_read_allowlist": [*map(str, exact_files), *map(str, allowed_dirs)],
    }


def core_module_identity(module_path, core_roots):
    """Validate a loaded module against pinned Perl Config core roots."""
    module = Path(module_path).resolve(strict=True)
    for index, value in enumerate(core_roots):
        core = Path(value).resolve(strict=True)
        if module.is_relative_to(core) and module.is_file():
            return {"core_root": f"core[{index}]", "relative_path": module.relative_to(core).as_posix(),
                    "sha256": sha256(module.read_bytes())}
    raise ValueError("loaded Perl module is outside Config core roots")


def declared_dependency(root, value):
    path = Path(value)
    if path.is_absolute() or ".." in path.parts or not path.parts:
        raise ValueError("declared-dependencies must be a subject-relative file")
    rel = path.as_posix()
    target = root
    for part in path.parts:
        target = target / part
        if target.is_symlink():
            raise ValueError("declared-dependencies path must not contain symlinks")
    if not target.is_file() or not target.resolve(strict=True).is_relative_to(root):
        raise ValueError("declared-dependencies must be a regular subject file")
    if rel not in git(root, "ls-files").splitlines():
        raise ValueError("declared-dependencies file must be tracked")
    return {"path": rel, "sha256": sha256(target.read_bytes())}


def verify_dependency_lock(lock_path, root, subject, declared_path, perl_lib):
    if lock_path is None or declared_path is None:
        raise ValueError("local --perl-lib requires --dependency-lock and --declared-dependencies")
    declared = declared_dependency(root, declared_path)
    current = library_fingerprint(perl_lib)
    lock_file = Path(lock_path)
    if lock_file.is_symlink() or not lock_file.is_file():
        raise ValueError("dependency lock must be a regular file")
    binary = lock_file.read_bytes()
    lock = json.loads(binary)
    if not isinstance(lock, dict) or lock.get("format") != "perl-dependency-lock-v1":
        raise ValueError("unsupported dependency lock format")
    if lock.get("subject") != subject:
        raise ValueError("dependency lock subject does not match measurement")
    if lock.get("declared_dependencies") != declared:
        raise ValueError("dependency lock does not match the subject declaration file")
    if lock.get("local_lib_perl5") != current:
        raise ValueError("dependency lock does not match the prepared local library fingerprint")
    return {
        "lock_sha256": sha256(binary),
        "declared_dependencies": declared,
        "fingerprint_before": current,
        "fingerprint_after": None,
        "setup_attestation": "caller-attested declared-dependency recipe; not inferred from the library path",
    }


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
    parser.add_argument("--perl-lib", help="optional sole PERL5LIB directory (for example PREFIX/lib/perl5)")
    parser.add_argument("--dependency-lock", help="prepared JSON matching the declared dependencies and local library")
    parser.add_argument("--declared-dependencies", help="subject-relative tracked declaration file, for example cpanfile")
    args = parser.parse_args()
    name, sep, root_text = args.subject.partition("=")
    if not sep or not name or not root_text:
        parser.error("--subject must be NAME=ROOT")
    root = Path(root_text).resolve()
    perl_binary = Path(args.perl).resolve(strict=True)
    if not perl_binary.is_file() or not os.access(perl_binary, os.X_OK):
        parser.error("--perl must name a regular executable")
    if args.perl_lib:
        perl_lib_arg = Path(args.perl_lib)
        if perl_lib_arg.is_symlink() or perl_lib_arg.parent.is_symlink():
            parser.error("--perl-lib path must not contain a symlink")
        perl_lib = perl_lib_arg.resolve(strict=True)
        if not perl_lib.is_dir() or perl_lib.name != "perl5":
            parser.error("--perl-lib must name an existing directory")
        if not args.dependency_lock or not args.declared_dependencies:
            parser.error("--perl-lib requires --dependency-lock and --declared-dependencies")
    else:
        if args.dependency_lock or args.declared_dependencies:
            parser.error("dependency lock arguments require --perl-lib")
        perl_lib = None
    env = perl_environment(perl_lib)
    out = Path(args.out)
    log_dir = Path(args.log_dir)
    meta = out.with_name(out.stem + ".meta.json")
    if out.exists() or meta.exists() or log_dir.exists():
        parser.error("output, metadata, and log directory must not exist")
    version = subprocess.check_output(sandbox_command(str(perl_binary), "-e", "print $^V"), env=env,
                                      text=True, timeout=5).strip()
    if version != "v5.34.1":
        parser.error("Perl executable must be exactly 5.34.1")
    network_control = verify_network_denied(str(perl_binary), env)
    perl_binary_sha256 = sha256(perl_binary.read_bytes())
    ctags = ctags_identity(args.ctags)
    before = snapshot(root)
    dependency_context = None
    if perl_lib:
        try:
            dependency_context = verify_dependency_lock(
                args.dependency_lock, root, name, args.declared_dependencies, perl_lib
            )
        except (OSError, ValueError, json.JSONDecodeError) as exc:
            parser.error(str(exc))
    inc_context = perl_inc_context(str(perl_binary), root, perl_lib, env)
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
        data, failure = observe(str(perl_binary), root, rel, raw, file_deadline, env, inc_context)
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
    if perl_lib:
        dependency_after = library_fingerprint(perl_lib)
        if dependency_after != dependency_context["fingerprint_before"]:
            raise RuntimeError("prepared local Perl library changed during run")
        dependency_context["fingerprint_after"] = dependency_after
    ranges.sort(key=lambda r: (r["path"], r["line"], r["name"], r["end"]))
    payload = {
        "tool": "perl-DB-sub-compile", "version": version, "tier": "A",
        "units": len(before["files"]), "errored": len(failures),
        "kinds": {"function": "function"},
        "scope": "Tier A only for the actual compiled population. Named subroutines compiled in their own source file have end lines from compiler %DB::sub. Name and declaration line join to independent Universal Ctags Perl subroutine tags in the same file, within two lines, only when reciprocal unique. Anonymous/eval/imported-outside-source and ambiguous joins excluded. Units include every tracked source file; any file that cannot compile remains unresolved against coverage and is counted in errored. No network access; only subject-declared dependencies are permitted. -c omits program body but BEGIN/CHECK/use may execute.",
        "ranges": ranges,
    }
    metadata = {
        "subject": name, "source_manifest": before, "failures": failures,
        "perl_binary_sha256": perl_binary_sha256,
        "ctags": ctags, "joining": joining,
        "network_policy": {"sandbox_exec": str(SANDBOX_EXEC), "profile": SANDBOX_PROFILE,
                           "control": network_control},
        "compiler_sandbox": {
            "filesystem_read_profile": inc_context["filesystem_read_profile"],
            "filesystem_read_allowlist": inc_context["filesystem_read_allowlist"],
        },
        "dependency_context": {
            "inherited_PERL5LIB_PERLLIB_PERL5OPT": "cleared",
            "allowed_search_context": {
                "core": "Perl Config privlibexp and archlibexp",
                "subject": "subject root, subject lib, and source-declared use lib paths",
                "local": "explicit locked PREFIX/lib/perl5 and its Config archname subdirectory" if perl_lib else None,
            },
            "lock_sha256": dependency_context["lock_sha256"] if dependency_context else None,
            "declared_dependencies": dependency_context["declared_dependencies"] if dependency_context else None,
            "local_lib_perl5_fingerprint_before": dependency_context["fingerprint_before"] if dependency_context else None,
            "local_lib_perl5_fingerprint_after": dependency_context["fingerprint_after"] if dependency_context else None,
            "setup_attestation": dependency_context["setup_attestation"] if dependency_context else "no local dependency prefix supplied; external dependencies remain unresolved",
        },
        "compiled_files": sum(row["compiler_status"] == "ok" for row in joining["files"]),
        "unresolved_files": sum(row["compiler_status"] != "ok" for row in joining["files"]),
        "tier_status": "C-R9 tier A for actual compiled population only; unresolved files remain in units; range quality pending measurement",
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
