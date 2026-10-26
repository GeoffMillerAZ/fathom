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
    with tempfile.TemporaryDirectory() as scratch:
        base = Path(scratch)
        root = base / "subject"
        root.mkdir()
        (root / "lib").mkdir()
        (root / ".gitignore").write_text("ignored.pl\n")
        (root / "ignored.pl").write_text("sub Ignored {}\n")
        marker = base / "begin-marker"
        (root / "example.pl").write_text(
            "BEGIN { open my $f, '>', $ENV{DB_RANGE_BEGIN_MARKER} or die $!; print {$f} 'ran'; close $f; }\n"
            "sub Forward ($);\n"
            "sub Multiline\n{\n  return 1;\n}\n"
            "my $anon = sub { return 2; };\n"
            "die 'program body executed';\n"
        )
        subprocess.run(["git", "init", "-q", str(root)], check=True)
        subprocess.run(["git", "-C", str(root), "add", "."], check=True)
        subprocess.run(["git", "-C", str(root), "-c", "user.name=Range test", "-c", "user.email=range@test.invalid", "commit", "-qm", "fixture"], check=True)
        before = runner.snapshot(root)
        assert [e["path"] for e in before["files"]] == ["example.pl"], before
        (root / "untracked.pl").write_text("sub Untracked {}\n")
        rejects_dirty(root)
        (root / "untracked.pl").unlink()
        env = os.environ.copy()
        env["DB_RANGE_BEGIN_MARKER"] = str(marker)
        out = base / "oracle.json"
        logs = base / "logs"
        subprocess.run([sys.executable, str(Path(runner.__file__).resolve()), "--perl", args.perl, "--ctags", args.ctags,
                        "--subject", "synthetic=" + str(root), "--out", str(out), "--log-dir", str(logs)],
                       env=env, check=True)
        result = json.loads(out.read_text())
        meta = json.loads(out.with_name(out.stem + ".meta.json").read_text())
        assert set(result) == {"tool", "version", "tier", "units", "errored", "kinds", "scope", "ranges"}, result
        assert meta["source_manifest"] == before and meta["failures"] == [], meta
        assert meta["perl_binary_sha256"] == runner.sha256(Path(args.perl).resolve().read_bytes()), meta
        assert meta["joining"]["tag_total"] >= meta["joining"]["paired"], meta
        assert result["units"] == 1 and result["errored"] == 0, result
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
        assert [e["path"] for e in linked_snapshot["files"]] == ["Policy/included.pm", "example.pl"], linked_snapshot
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
