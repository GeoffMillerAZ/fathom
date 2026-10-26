#!/usr/bin/env python3
"""Optional synthetic controls; never substitutes for a full corpus gate."""

import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time

import dependencies
import runner


def rejects_dirty(root):
    try:
        runner.snapshot(root)
    except ValueError:
        return
    raise AssertionError("dirty source snapshot was accepted")


def descendant_cleanup(base):
    marker = base / "descendant-pid"
    child = (
        "import os, signal, time; "
        "signal.signal(signal.SIGTERM, signal.SIG_IGN); "
        f"open({str(marker)!r}, 'w').write(str(os.getpid())); "
        "time.sleep(30)"
    )
    parent = f"import subprocess,time,sys; subprocess.Popen([sys.executable,'-c',{child!r}]); time.sleep(30)"
    proc = subprocess.Popen([sys.executable, "-c", parent], start_new_session=True)
    calls = []
    original = runner.os.killpg
    try:
        until = time.monotonic() + 2
        while not marker.exists() and time.monotonic() < until:
            time.sleep(.01)
        assert marker.exists(), "descendant did not start"

        def recorded(pgid, sig):
            calls.append(sig)
            return original(pgid, sig)

        runner.os.killpg = recorded
        runner.stop_group(proc)
        assert proc.poll() is not None, "group leader survived cleanup"
        assert signal.SIGTERM in calls and signal.SIGKILL in calls, calls
    finally:
        runner.os.killpg = original
        try:
            original(proc.pid, signal.SIGKILL)
        except (ProcessLookupError, PermissionError):
            pass
        proc.wait(timeout=2)


def injected_eperm_cleanup():
    class FakeProcess:
        pid = 123456
        waited = False

        def poll(self):
            return None

        def wait(self, timeout=None):
            self.waited = True
            return 0

    original = runner.os.killpg
    for denied in ({0}, {signal.SIGTERM, 0, signal.SIGKILL}):
        calls = []

        def fake_killpg(pgid, sig):
            calls.append(sig)
            if sig in denied:
                raise PermissionError(1, "synthetic Darwin zombie probe")

        runner.os.killpg = fake_killpg
        try:
            proc = FakeProcess()
            runner.stop_group(proc)
            assert proc.waited and signal.SIGKILL in calls, calls
        finally:
            runner.os.killpg = original


def sandbox_controls(perl, base):
    original = runner.SANDBOX_EXEC
    runner.SANDBOX_EXEC = base / "missing-sandbox-exec"
    try:
        try:
            runner.sandbox_command(perl, "-e", "1")
        except RuntimeError as exc:
            assert "refusing unsandboxed Perl" in str(exc), exc
        else:
            raise AssertionError("missing sandbox-exec did not fail closed")
    finally:
        runner.SANDBOX_EXEC = original

    saved = {key: os.environ.get(key) for key in (*runner.PERL_ENV_KEYS, *runner.LOCALE_ENV)}
    try:
        for key in (*runner.PERL_ENV_KEYS, *runner.LOCALE_ENV):
            os.environ[key] = "ambient-poison"
        clean = runner.perl_environment()
        assert all(key not in clean for key in runner.PERL_ENV_KEYS), clean
        assert all(clean[key] == "C" for key in runner.LOCALE_ENV), clean
        prefix_lib = base / "prefix" / "lib" / "perl5"
        prefix_lib.mkdir(parents=True)
        selected = runner.perl_environment(prefix_lib)
        assert all(key not in selected for key in runner.PERL_ENV_KEYS), selected
        # A local B::Xref shadow is neither injected nor accepted as core.
        shadow = prefix_lib / "B" / "Xref.pm"
        shadow.parent.mkdir()
        shadow.write_text("package B::Xref; 1;\n")
        core = base / "synthetic-core"
        core_module = core / "B" / "Xref.pm"
        core_module.parent.mkdir(parents=True)
        core_module.write_text("package B::Xref; 1;\n")
        identity = runner.core_module_identity(core_module, [core])
        assert identity["relative_path"] == "B/Xref.pm", identity
        try:
            runner.core_module_identity(shadow, [core])
        except ValueError:
            pass
        else:
            raise AssertionError("local B::Xref shadow was accepted as core")
    finally:
        for key, value in saved.items():
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = value

    denial = runner.verify_network_denied(perl, runner.perl_environment())
    assert denial["destination"] == "127.0.0.1:1/tcp" and denial["denial"] == "EPERM", denial


def timeout_cleanup(base):
    proc = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(30)"], start_new_session=True)
    try:
        try:
            proc.communicate(timeout=.02)
        except subprocess.TimeoutExpired:
            runner.stop_group(proc)
        else:
            raise AssertionError("timeout control finished before its deadline")
        assert proc.poll() is not None, "timed-out group leader survived"
    finally:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except (ProcessLookupError, PermissionError):
            pass
        proc.wait(timeout=2)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--perl", required=True)
    ap.add_argument("--ctags", required=True)
    args = ap.parse_args()
    # The compiler may start at a multiline body's opening brace, while the
    # source-written tag names the declaration. The compiler end is immutable.
    native = [{"path": "example.pl", "name": "Pkg::Multiline", "kind": "function", "line": 4, "end": 6},
              {"path": "example.pl", "name": "Pkg::qualified", "kind": "function", "line": 9, "end": 11}]
    paired, detail = runner.bridge(native, [{"name": "Multiline", "line": 3},
                                            {"name": "Pkg::qualified", "line": 9}])
    assert paired == [
        {"path": "example.pl", "name": "Multiline", "kind": "function", "line": 3, "end": 6},
        {"path": "example.pl", "name": "Pkg::qualified", "kind": "function", "line": 9, "end": 11},
    ], paired
    assert detail["paired"] == 2 and detail["native"][0]["native_start"] == 4, detail
    paired, detail = runner.bridge(native[:1], [{"name": "Multiline", "line": 3}, {"name": "Multiline", "line": 4}])
    assert not paired and detail["ambiguous"] == 1, detail
    reverse_collision = [native[0], {**native[0], "name": "Other::Multiline"}]
    paired, detail = runner.bridge(reverse_collision, [{"name": "Multiline", "line": 3}])
    assert not paired and detail["ambiguous"] == 2, detail
    paired, detail = runner.bridge(native[:1], [{"name": "Different", "line": 3}])
    assert not paired and detail["unmatched"] == 1, detail
    hundred_tags = [{"name": f"Sub{index}", "line": index + 1} for index in range(100)]
    ten_native = [{"path": "bulk.pl", "name": f"Pkg::Sub{index}",
                   "kind": "function", "line": index + 1, "end": index + 2}
                  for index in range(10)]
    joining = runner.new_joining()
    bulk = runner.record_file(joining, "bulk.pl", hundred_tags, ten_native)
    assert len(bulk) == 10, bulk
    assert (joining["paired"], joining["tag_total"], joining["tag_unpaired"]) == (10, 100, 90), joining
    assert joining["files"][0]["tag_total"] == 100 and joining["files"][0]["tag_unpaired"] == 90, joining
    # Independent ctags still contributes to the denominator when Perl
    # compilation fails and supplies no native entries for this file.
    failed_tags = [{"name": f"Failed{index}", "line": index + 1} for index in range(3)]
    assert runner.record_file(joining, "failed.pl", failed_tags, None) == []
    assert (joining["tag_total"], joining["tag_unpaired"], joining["compiler_unknown_files"]) == (103, 93, 1), joining
    assert joining["files"][1] == {"path": "failed.pl", "tag_total": 3, "tag_unpaired": 3,
                                  "ctags_status": "ok", "compiler_status": "unknown"}, joining
    runner.record_file(joining, "ctags-failed.pl", None, None)
    assert joining["ctags_unknown_files"] == 1 and joining["files"][2]["tag_total"] is None, joining
    dependencies.self_test()
    scratch_root = Path(runner.__file__).resolve().parents[3] / "workspace" / "tmp"
    scratch_root.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="perl-db-ranges-", dir=scratch_root) as scratch:
        base = Path(scratch)
        sandbox_controls(args.perl, base)
        root = base / "subject"
        root.mkdir()
        (root / "lib").mkdir()
        (root / "t" / "lib" / "Subject").mkdir(parents=True)
        (root / ".gitignore").write_text("ignored.pl\n")
        (root / "ignored.pl").write_text("sub Ignored {}\n")
        (root / "bad.pl").write_text("sub Broken {\n")
        (root / "cpanfile").write_text("requires 'Local::Allowed';\n")
        (root / "t" / "lib" / "Subject" / "Helper.pm").write_text("package Subject::Helper; sub helper { 1 } 1;\n")
        (root / "t-lib.pl").write_text("use lib 't/lib'; use Subject::Helper; sub TLibAccepted { 1 }\n")
        (root / "local-lib.pl").write_text("use Local::Allowed; sub LocalAccepted { 1 }\n")
        (root / "contamination.pl").write_text("use Poison::Helper; sub VendorMustNotLoad { 1 }\n")
        (root / "inside-load.pl").write_text("BEGIN { do $ENV{SUBJECT_SCRIPT} or die 'subject do failed'; } sub InsideDo { 1 }\n")
        (root / "allowed-do.pl").write_text("1;\n")
        outside_script = base / "outside-undeclared.pl"
        outside_do_marker = base / "outside-do-loaded"
        outside_script.write_text(
            "BEGIN { open my $f, '>', $ENV{OUTSIDE_DO_MARKER} or die $!; print {$f} 'loaded'; close $f; } 1;\n"
        )
        outside_module = base / "outside-Undeclared.pm"
        outside_require_marker = base / "outside-require-loaded"
        outside_module.write_text(
            "package Outside::Undeclared; BEGIN { open my $f, '>', $ENV{OUTSIDE_REQUIRE_MARKER} or die $!; "
            "print {$f} 'loaded'; close $f; } 1;\n"
        )
        (root / "outside-do.pl").write_text("BEGIN { do $ENV{OUTSIDE_SCRIPT} or die 'outside do escaped'; }\n")
        (root / "outside-require.pl").write_text(
            "BEGIN { require $ENV{OUTSIDE_MODULE}; delete $INC{$ENV{OUTSIDE_MODULE}}; }\n"
        )
        marker = base / "begin-marker"
        (root / "example.pl").write_text(
            "BEGIN { open my $f, '>', $ENV{DB_RANGE_BEGIN_MARKER} or die $!; print {$f} 'ran'; close $f; }\n"
            "sub Forward ($);\n"
            "sub Multiline\n{\n  return 1;\n}\n"
            "my $anon = sub { return 2; };\n"
            "die 'program body executed';\n"
        )
        poison_lib = base / "poison" / "Poison"
        poison_lib.mkdir(parents=True)
        (poison_lib / "Helper.pm").write_text("package Poison::Helper; 1;\n")
        private_perl_lib = base / "prefix" / "lib" / "perl5"
        (private_perl_lib / "Local").mkdir(parents=True)
        (private_perl_lib / "Local" / "Allowed.pm").write_text("package Local::Allowed; 1;\n")
        subprocess.run(["git", "init", "-q", str(root)], check=True)
        subprocess.run(["git", "-C", str(root), "add", "."], check=True)
        subprocess.run(["git", "-C", str(root), "-c", "user.name=Range test", "-c", "user.email=range@test.invalid", "commit", "-qm", "fixture"], check=True)
        before = runner.snapshot(root)
        assert [e["path"] for e in before["files"]] == [
            "allowed-do.pl", "bad.pl", "contamination.pl", "example.pl", "inside-load.pl",
            "local-lib.pl", "outside-do.pl", "outside-require.pl", "t-lib.pl", "t/lib/Subject/Helper.pm"
        ], before
        (root / "untracked.pl").write_text("sub Untracked {}\n")
        rejects_dirty(root)
        (root / "untracked.pl").unlink()
        env = os.environ.copy()
        env["DB_RANGE_BEGIN_MARKER"] = str(marker)
        env["PERL5LIB"] = str(base / "poison")
        env["PERLLIB"] = str(base / "poison")
        env["PERL5OPT"] = "-MPoison::Helper"
        env["OUTSIDE_SCRIPT"] = str(outside_script)
        env["OUTSIDE_MODULE"] = str(outside_module)
        env["OUTSIDE_DO_MARKER"] = str(outside_do_marker)
        env["OUTSIDE_REQUIRE_MARKER"] = str(outside_require_marker)
        env["SUBJECT_SCRIPT"] = str(root / "allowed-do.pl")
        out = base / "oracle.json"
        logs = base / "logs"
        declared = runner.declared_dependency(root, "cpanfile")
        fingerprint = dependencies.library_fingerprint(private_perl_lib)
        lock_data = {"format": "perl-dependency-lock-v1", "subject": "synthetic", "declared_dependencies": declared,
                     "local_lib_perl5": fingerprint}
        lock = base / "dependency-lock.json"
        lock.write_text(json.dumps(lock_data, sort_keys=True))
        accepted = runner.verify_dependency_lock(lock, root, "synthetic", "cpanfile", private_perl_lib)
        assert accepted["fingerprint_before"] == fingerprint, accepted
        for wrong in (None, {**lock_data, "subject": "wrong"},
                      {**lock_data, "declared_dependencies": {**declared, "sha256": "0" * 64}},
                      {**lock_data, "local_lib_perl5": {**fingerprint, "sha256": "0" * 64}}):
            wrong_path = None if wrong is None else base / "wrong-lock.json"
            if wrong_path:
                wrong_path.write_text(json.dumps(wrong))
            try:
                runner.verify_dependency_lock(wrong_path, root, "synthetic", "cpanfile", private_perl_lib)
            except ValueError:
                pass
            else:
                raise AssertionError("missing or mismatched dependency lock accepted")
        subprocess.run([sys.executable, str(Path(runner.__file__).resolve()), "--perl", args.perl, "--ctags", args.ctags,
                        "--subject", "synthetic=" + str(root), "--out", str(out), "--log-dir", str(logs),
                        "--perl-lib", str(private_perl_lib), "--dependency-lock", str(lock),
                        "--declared-dependencies", "cpanfile"],
                       env=env, check=True)
        result = json.loads(out.read_text())
        meta = json.loads(out.with_name(out.stem + ".meta.json").read_text())
        assert set(result) == {"tool", "version", "tier", "units", "errored", "kinds", "scope", "ranges"}, result
        assert meta["source_manifest"] == before and meta["failures"] == [
            {"path": "bad.pl", "reasons": ["compile_error"]},
            {"path": "contamination.pl", "reasons": ["compile_error"]},
            {"path": "outside-do.pl", "reasons": ["compile_error"]},
            {"path": "outside-require.pl", "reasons": ["compile_error"]},
        ], meta
        assert meta["perl_binary_sha256"] == runner.sha256(Path(args.perl).resolve().read_bytes()), meta
        assert result["tier"] == "A" and result["units"] == 10 and result["errored"] == 4, result
        assert meta["compiled_files"] == 6 and meta["unresolved_files"] == 4, meta
        assert meta["network_policy"]["control"]["denial"] == "EPERM", meta
        read_profile = meta["compiler_sandbox"]["filesystem_read_profile"]
        allowlist = meta["compiler_sandbox"]["filesystem_read_allowlist"]
        assert "(deny network*)" in read_profile and "(deny file-read-data" in read_profile, read_profile
        assert "(require-not (require-any" in read_profile and str(root) in allowlist, meta
        assert '(literal "/")' in read_profile, read_profile
        assert "/System/Cryptexes/OS/System/Library/dyld" in allowlist, allowlist
        assert "/System/Volumes/Preboot/Cryptexes/OS/System/Library/dyld" in allowlist, allowlist
        assert "/System" not in allowlist and "/Library" not in allowlist, allowlist
        assert str(private_perl_lib.resolve()) in allowlist, meta
        assert '(subpath "/Library/Perl")' not in read_profile and '(subpath "/System/Library/Perl/Extras")' not in read_profile, read_profile
        assert meta["dependency_context"]["inherited_PERL5LIB_PERLLIB_PERL5OPT"] == "cleared", meta
        assert meta["dependency_context"]["local_lib_perl5_fingerprint_before"] == fingerprint, meta
        assert meta["dependency_context"]["local_lib_perl5_fingerprint_after"] == fingerprint, meta
        assert meta["dependency_context"]["setup_attestation"].startswith("caller-attested"), meta
        assert meta["joining"]["tag_total"] >= meta["joining"]["paired"], meta
        compile_status = {item["path"]: item["compiler_status"] for item in meta["joining"]["files"]}
        assert compile_status["local-lib.pl"] == "ok" and compile_status["t-lib.pl"] == "ok", compile_status
        assert compile_status["contamination.pl"] == "unknown", compile_status
        assert compile_status["inside-load.pl"] == "ok" and compile_status["allowed-do.pl"] == "ok", compile_status
        assert compile_status["outside-do.pl"] == "unknown" and compile_status["outside-require.pl"] == "unknown", compile_status
        assert not outside_do_marker.exists() and not outside_require_marker.exists(), "outside source executed despite read denial"
        assert marker.read_text() == "ran", "BEGIN must be disclosed as executed"
        names = {r["name"] for r in result["ranges"]}
        assert "Multiline" in names and "Forward" not in names, names
        assert all("__ANON__" not in n for n in names), names
        multiline = next(r for r in result["ranges"] if r["name"] == "Multiline")
        assert (multiline["line"], multiline["end"]) == (3, 6), multiline
        compiler = next(r for r in meta["joining"]["native"] if r["compiler_key"] == "main::Multiline")
        assert (compiler["native_start"], compiler["native_end"], compiler["join"]) == (4, 6, "paired"), compiler
        (root / "example.pl").write_text((root / "example.pl").read_text().replace("return 1", "return 3"))
        rejects_dirty(root)  # same-name, same-line mutation is rejected
        subprocess.run(["git", "-C", str(root), "restore", "example.pl"], check=True)
        (root / "linked.pm").symlink_to("example.pl")
        subprocess.run(["git", "-C", str(root), "add", "linked.pm"], check=True)
        subprocess.run(["git", "-C", str(root), "-c", "user.name=Range test", "-c", "user.email=range@test.invalid", "commit", "-qm", "symlink fixture"], check=True)
        try:
            runner.snapshot(root)
        except ValueError as exc:
            assert "symlink" in str(exc), exc
        else:
            raise AssertionError("tracked symlink was accepted")
        (root / "linked.pm").unlink()
        subprocess.run(["git", "-C", str(root), "rm", "-q", "linked.pm"], check=True)
        (root / "Policy").mkdir()
        (root / "Policy" / "included.pm").write_text("sub Included {}\n")
        (root / "PolicyLink").symlink_to("Policy", target_is_directory=True)
        subprocess.run(["git", "-C", str(root), "add", "Policy", "PolicyLink"], check=True)
        subprocess.run(["git", "-C", str(root), "-c", "user.name=Range test", "-c", "user.email=range@test.invalid", "commit", "-qm", "directory link fixture"], check=True)
        linked_snapshot = runner.snapshot(root)
        assert [e["path"] for e in linked_snapshot["files"]] == [
            "Policy/included.pm", "allowed-do.pl", "bad.pl", "contamination.pl", "example.pl", "inside-load.pl",
            "local-lib.pl", "outside-do.pl", "outside-require.pl", "t-lib.pl", "t/lib/Subject/Helper.pm"
        ], linked_snapshot
        for link, target in (("outside-link", base), ("broken-link", root / "missing-directory"),
                             ("source-link.pm", root / "example.pl")):
            path = root / link
            path.symlink_to(target)
            subprocess.run(["git", "-C", str(root), "add", link], check=True)
            try:
                runner.files_under(root)
            except ValueError as exc:
                assert "symlink" in str(exc), exc
            else:
                raise AssertionError(f"tracked {link} symlink was accepted")
            subprocess.run(["git", "-C", str(root), "reset", "-q", "HEAD", "--", link], check=True)
            path.unlink()
        injected_eperm_cleanup()
        timeout_cleanup(base)
        descendant_cleanup(base)
    print("synthetic compiler-range controls passed")


if __name__ == "__main__":
    main()
