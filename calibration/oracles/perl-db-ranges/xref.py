#!/usr/bin/env python3
"""Diagnostic census of direct calls exposed by stock Perl B::Xref 1.07."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import time

import runner


def parse_xref(raw, source_prefix, caller=None, include_lines=False):
    """Count only source-file subused rows; method targets remain unknown."""
    direct = 0
    methods = 0
    emitted_method_targets = 0
    non_direct_unknown = 0
    malformed = 0
    source_lines = {"direct": [], "non_direct_unknown": []}
    for line in raw.splitlines():
        if not line.startswith(source_prefix):
            continue
        fields = line[len(source_prefix):].split()
        if not fields:
            continue
        if caller is not None and fields[0] != caller:
            continue
        if fields[-1] == "meth":
            if len(fields) >= 4 and "->" in fields[:-1]:
                methods += 1
                emitted_method_targets += 1
            else:
                malformed += 1
            continue
        if fields[-1] != "subused":
            continue
        if len(fields) >= 4 and fields[-3] == "&":
            direct += 1
            category = "direct"
        elif len(fields) >= 3 and fields[-2] and fields[-1] == "subused":
            non_direct_unknown += 1
            category = "non_direct_unknown"
        else:
            malformed += 1
            continue
        if len(fields) >= 2 and fields[1].isdigit():
            source_lines[category].append(int(fields[1]))
    result = {"direct_call_records": direct,
            "method_call_records": methods,
            "method_target_emitted_records": emitted_method_targets,
            "non_direct_target_unknown_records": non_direct_unknown,
            "malformed_subused_records": malformed}
    if include_lines:
        result["source_lines"] = source_lines
    return result


def synthetic_test():
    fixture = ("/synthetic/subject.pl Caller 4 Target & first subused\n"
               "/synthetic/subject.pl Caller 5 Target & second subused\n"
               "/synthetic/subject.pl Caller 6 (lexical) $ receiverAny subused\n"
               "/synthetic/subject.pl Caller 7 ? ? subused\n"
               "/synthetic/subject.pl Caller 8 Sample -> method meth\n"
               "/synthetic/subject.pl Caller 9 Sample unrelated event\n")
    got = parse_xref(fixture, "/synthetic/subject.pl ")
    expected = {"direct_call_records": 2,
                "non_direct_target_unknown_records": 2,
                "method_call_records": 1,
                "method_target_emitted_records": 1,
                "malformed_subused_records": 0}
    if got != expected:
        raise AssertionError(f"synthetic Xref parser control failed: {got}")
    return got


def launch_xref(perl, source, root, env, inc, prefix, deadline):
    """Run one compile through the shared C-R9 Xref invocation path."""
    observer_json = prefix.with_suffix(".guard.json")
    stdout_path = prefix.with_suffix(".stdout.raw")
    stderr_path = prefix.with_suffix(".stderr.raw")
    run_env = env.copy()
    run_env["FATHOM_DB_RANGE_OUT"] = str(observer_json)
    run_env["FATHOM_ALLOWED_INC"] = os.pathsep.join(inc["inc"])
    run_env["FATHOM_ALLOWED_ROOTS"] = os.pathsep.join(inc["roots"])
    run_env["FATHOM_PATH_SEP"] = os.pathsep
    core_args = ["-I" + core for core in inc["core"]]
    guard_args = ["-I" + str(runner.OBSERVER.parent), "-MFathomPerlCompileGuard"]
    command = runner.sandbox_command(
        perl, *core_args, *guard_args, "-MO=Xref,-r", "-c", str(source),
        profile=inc["filesystem_read_profile"])
    with stdout_path.open("wb") as out, stderr_path.open("wb") as err:
        proc = subprocess.Popen(command, cwd=root, env=run_env, stdout=out, stderr=err,
                                start_new_session=True)
        timed_out = False
        try:
            try:
                proc.communicate(timeout=max(.01, deadline-time.monotonic()))
            except subprocess.TimeoutExpired:
                timed_out = True
        finally:
            runner.stop_group(proc)
    size = stdout_path.stat().st_size + stderr_path.stat().st_size
    if timed_out:
        return None, "file_timeout"
    if size > 64 * 1024 * 1024:
        return None, "output_limit"
    if proc.returncode or not observer_json.is_file():
        return None, "compile_error" if proc.returncode else "missing_observer_output"
    return stdout_path.read_text(errors="replace"), None


def native_self_test(perl):
    """Actual stock compiler control; invoke only when host serialization permits."""
    perl = str(Path(perl).resolve(strict=True))
    env = runner.perl_environment()
    version = subprocess.check_output(runner.sandbox_command(perl, "-e", "print $^V"),
                                      env=env, text=True, timeout=5).strip()
    if version != "v5.34.1":
        raise RuntimeError("Perl executable must be exactly 5.34.1")
    if runner.verify_network_denied(perl, env)["denial"] != "EPERM":
        raise RuntimeError("C-R9 network control did not prove EPERM")
    scratch_parent = Path.cwd().resolve() / "workspace" / "tmp"
    if not scratch_parent.is_dir():
        raise RuntimeError("native control requires gitignored workspace/tmp")
    import tempfile
    scratch = Path(tempfile.mkdtemp(prefix="perl-xref-native-control-", dir=scratch_parent))
    (scratch / "lib").mkdir()
    source = scratch / "control.pl"
    source.write_text("""package Sample;
sub bare_callee { return 1 }
sub fullyqualified_callee { return 2 }
sub method { return 3 }
sub caller {
    bare_callee();
    Sample::fullyqualified_callee();
    my $object = bless {}, 'Sample';
    $object->method();
    Sample->method();
}
die 'ordinary body must not execute';
1;
""")
    inc = runner.perl_inc_context(perl, scratch, None, env)
    core_args = ["-I" + core for core in inc["core"]]
    probe = ("use B::Xref; use JSON::PP; print JSON::PP->new->encode({"
             "version=>$B::Xref::VERSION,module=>$INC{'B/Xref.pm'}});")
    info = json.loads(subprocess.check_output(
        runner.sandbox_command(perl, *core_args, "-e", probe), env=env, text=True, timeout=5))
    if info["version"] != "1.07":
        raise RuntimeError("native control requires B::Xref 1.07")
    runner.core_module_identity(info["module"], inc["core"])
    deadline = time.monotonic() + runner.TOTAL_SECONDS
    if not runner.wait_for_load(deadline):
        raise RuntimeError("native control skipped because system load exceeds 9")
    compile_deadline = runner.compiler_deadline(deadline)
    out, failure = launch_xref(perl, source, scratch, env, inc, scratch / "control",
                               compile_deadline)
    if failure:
        raise RuntimeError("native B::Xref control unresolved: " + failure)
    counts = parse_xref(out, str(source) + " ", caller="Sample::caller", include_lines=True)
    if counts["direct_call_records"] != 2 or counts["non_direct_target_unknown_records"] != 2:
        raise RuntimeError("native B::Xref control counts differ: " + json.dumps(counts, sort_keys=True))
    if {key: sorted(lines) for key, lines in counts["source_lines"].items()} != {
            "direct": [6, 7], "non_direct_unknown": [9, 10]}:
        raise RuntimeError("native B::Xref source attribution differs: " + json.dumps(counts, sort_keys=True))
    return {"perl_version": version, "counts": counts,
            "body_not_executed": True, "network_denial": "EPERM"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--self-test", action="store_true")
    parser.add_argument("--native-self-test", action="store_true")
    parser.add_argument("--perl")
    parser.add_argument("--subject", help="name=/clean/git/root")
    parser.add_argument("--log-dir", help="private workspace directory for raw compiler outputs")
    parser.add_argument("--perl-lib", help="optional sole PREFIX/lib/perl5")
    parser.add_argument("--dependency-lock")
    parser.add_argument("--declared-dependencies")
    args = parser.parse_args()
    if args.self_test and args.native_self_test:
        parser.error("choose one self-test mode")
    if args.self_test:
        print(json.dumps({"synthetic_parser_control": synthetic_test()}, sort_keys=True))
        return
    if args.native_self_test:
        if not args.perl:
            parser.error("--native-self-test requires --perl")
        print(json.dumps({"native_compiler_control": native_self_test(args.perl)}, sort_keys=True))
        return
    if not all((args.perl, args.subject, args.log_dir)):
        parser.error("--perl, --subject, and --log-dir are required")

    name, sep, root_text = args.subject.partition("=")
    if not sep or not name or not root_text:
        parser.error("--subject must be NAME=ROOT")
    root = Path(root_text).resolve(strict=True)
    perl = str(Path(args.perl).resolve(strict=True))
    perl_path = Path(perl)
    if not perl_path.is_file() or not os.access(perl_path, os.X_OK):
        parser.error("--perl must name a regular executable")
    perl_lib = None
    if args.perl_lib:
        lib_arg = Path(args.perl_lib)
        if lib_arg.is_symlink() or lib_arg.parent.is_symlink():
            parser.error("--perl-lib path must not contain a symlink")
        perl_lib = lib_arg.resolve(strict=True)
        if not perl_lib.is_dir() or perl_lib.name != "perl5":
            parser.error("--perl-lib must name an existing directory")
    elif args.dependency_lock or args.declared_dependencies:
        parser.error("dependency lock arguments require --perl-lib")

    logs = Path(args.log_dir)
    if logs.exists():
        parser.error("--log-dir must not exist")
    env = runner.perl_environment(perl_lib)
    version = subprocess.check_output(
        runner.sandbox_command(perl, "-e", "print $^V"),
        env=env, text=True, timeout=5).strip()
    if version != "v5.34.1":
        parser.error("Perl executable must be exactly 5.34.1")
    inc = runner.perl_inc_context(perl, root, perl_lib, env)
    core_args = [arg for core in inc["core"] for arg in ("-I" + core,)]
    xref_probe = ("use B::Xref; use JSON::PP; "
                  "print JSON::PP->new->encode({version=>$B::Xref::VERSION,"
                  "module=>$INC{'B/Xref.pm'}});")
    xref_info = json.loads(subprocess.check_output(
        runner.sandbox_command(perl, *core_args, "-e", xref_probe),
        env=env, text=True, timeout=5))
    xref_version = xref_info["version"]
    if xref_version != "1.07":
        parser.error("stock B::Xref must be exactly version 1.07")
    try:
        xref_identity = runner.core_module_identity(xref_info["module"], inc["core"])
    except (OSError, ValueError, KeyError) as exc:
        parser.error(str(exc))
    network_control = runner.verify_network_denied(perl, env)
    interpreter_hash = runner.sha256(perl_path.read_bytes())
    before = runner.snapshot(root)
    dependency = None
    if perl_lib:
        dependency = runner.verify_dependency_lock(
            args.dependency_lock, root, name, args.declared_dependencies, perl_lib)
    logs.mkdir(parents=True)

    totals = {"direct_call_records": 0, "method_call_records": 0,
              "method_target_emitted_records": 0,
              "non_direct_target_unknown_records": 0,
              "malformed_subused_records": 0}
    failures = 0
    compiled = 0
    deadline = time.monotonic() + runner.TOTAL_SECONDS
    for index, rel in enumerate(entry["path"] for entry in before["files"]):
        if time.monotonic() >= deadline:
            failures += 1
            continue
        if not runner.wait_for_load(deadline):
            failures += 1
            continue
        file_deadline = runner.compiler_deadline(deadline)
        source = (root / rel).resolve(strict=True)
        raw, failure = launch_xref(perl, source, root, env, inc,
                                   logs / f"{index:06d}", file_deadline)
        if failure:
            failures += 1
            continue
        compiled += 1
        counts = parse_xref(raw, str(source) + " ")
        for key in totals:
            totals[key] += counts[key]

    if runner.snapshot(root) != before:
        raise RuntimeError("subject source manifest changed during run")
    if perl_lib:
        after = runner.library_fingerprint(perl_lib)
        if after != dependency["fingerprint_before"]:
            raise RuntimeError("prepared local Perl library changed during run")
        dependency["fingerprint_after"] = after
    result = {
        "tool": "stock Perl B::Xref census", "perl_version": version,
        "version": xref_version,
        "backend_module": xref_identity,
        "status": "diagnostic census only; no grade", "subject": name,
        "units": len(before["files"]), "compiled": compiled,
        "compile_failures_or_timeouts": failures, **totals,
        "source_commit": before["commit"],
        "source_manifest_sha256": before["digest"],
        "perl_interpreter_sha256": interpreter_hash,
        "network_control": network_control,
        "dependency_lock_sha256": dependency["lock_sha256"] if dependency else None,
        "local_library_fingerprint_before": dependency["fingerprint_before"] if dependency else None,
        "local_library_fingerprint_after": dependency["fingerprint_after"] if dependency else None,
        "controls": {"synthetic_parser": synthetic_test(), "network_denial": "EPERM"},
    }
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()
