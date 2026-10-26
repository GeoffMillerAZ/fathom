#!/usr/bin/env python3
"""One-shot, pinned Perl range holdout; select and freeze a subject before running."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import statistics
import subprocess
import sys
import tempfile
import time

CPANM_ARCHIVE_SHA256 = "59b60907ab9fa4f72ca2004fbe6054911439ae9a906890b4d842a87b25f20f3c"
MAX_SECONDS = 600
HERE = Path(__file__).resolve().parent
HEX = re.compile(r"^[0-9a-f]{40}$")
ALREADY_MEASURED_COMMITS = {
    "90b6e651c1f1a7c576072fdd039ef15afecb67a5",  # Dancer2 spent
    "7b99f4adce82a45fd0965ef4dd56550b4f5ee89b",  # mojo canonical
    "382d701fd1129822c98fdcdb08403a733a24dc08",  # Perl-Critic canonical
}


def need(ok, message):
    if not ok:
        raise ValueError(message)


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], text=True,
                                   timeout=10).strip()


def wait_load(deadline, load_reader=os.getloadavg):
    while True:
        load = load_reader()[0]
        if load <= 9:
            return load
        need(time.monotonic() < deadline, "load wait exceeded phase deadline")
        time.sleep(min(5, max(.01, deadline - time.monotonic())))


def stop_group(proc):
    def signal_group(sig):
        try:
            os.killpg(proc.pid, sig)
        except (ProcessLookupError, PermissionError):
            # Darwin can report EPERM when only an unreaped zombie remains.
            return False
        return True

    signal_group(signal.SIGTERM)
    end = time.monotonic() + .3
    while time.monotonic() < end:
        proc.poll()
        try:
            os.killpg(proc.pid, 0)
        except (ProcessLookupError, PermissionError):
            break
        time.sleep(.01)
    # The leader may exit while a child that ignored TERM is still alive.
    signal_group(signal.SIGKILL)
    try:
        proc.wait(timeout=1)
    except subprocess.TimeoutExpired:
        signal_group(signal.SIGKILL)
        proc.wait(timeout=1)


def phase(name, argv, cwd, out, env, seconds=MAX_SECONDS, stdout=None,
          load_reader=os.getloadavg):
    nice = shutil.which("nice")
    need(nice is not None, "nice executable required")
    started = time.monotonic()
    deadline = started + min(seconds, MAX_SECONDS)
    load = wait_load(deadline, load_reader)
    need(time.monotonic() < deadline, "phase deadline elapsed before launch")
    started_utc = datetime.now(timezone.utc).isoformat()
    log = out / (name + ".log")
    with log.open("wb") as stream:
        if stdout is None:
            target = stream
        else:
            target = stdout.open("xb")
        try:
            proc = subprocess.Popen([nice, "-n", "10", *map(str, argv)],
                                    cwd=cwd, env=env, stdout=target, stderr=stream,
                                    start_new_session=True)
            timed_out = False
            try:
                proc.wait(timeout=max(.01, deadline - time.monotonic()))
            except subprocess.TimeoutExpired:
                timed_out = True
            finally:
                stop_group(proc)
        finally:
            if stdout is not None:
                target.close()
    row = {"phase": name, "exit": proc.returncode, "timed_out": timed_out,
           "launch_load": load, "seconds": round(time.monotonic() - started, 3),
           "started_utc": started_utc, "ended_utc": datetime.now(timezone.utc).isoformat(),
           "instrument_commit": env["FATHOM_INSTRUMENT_COMMIT"],
           "source_commit": env["FATHOM_SOURCE_COMMIT"]}
    if name in ("build-indexbench", "grade-holdout", "retime-three"):
        binary = out / "indexbench"
        if binary.is_file():
            row["binary_sha256"] = sha(binary)
    phases = out / "phases.json"
    existing = json.loads(phases.read_text()) if phases.exists() else []
    existing.append(row)
    phases.write_text(json.dumps(existing, sort_keys=True, indent=2) + "\n")
    need(not timed_out and proc.returncode == 0, "phase failed: " + name)
    return row


def assigned_caches(repo):
    for key in ("TMPDIR", "GOTMPDIR", "GOCACHE"):
        raw = os.environ.get(key)
        need(raw and Path(raw).is_absolute(), key + " must be assigned outside checkout")
        path = Path(raw).resolve(strict=True)
        need(path.is_dir() and not path.is_relative_to(repo), key + " must be an external directory")


def declared(root):
    tracked = set(git(root, "ls-files").splitlines())
    for name in ("cpanfile", "Makefile.PL", "Build.PL", "META.json", "META.yml"):
        path = root / name
        if name in tracked and path.is_file() and not path.is_symlink():
            return name
    return None


def install_dependencies(args, root, out, env, range_dir):
    declaration = declared(root)
    if declaration is None:
        return None
    need(args.cpanm and args.cpanm_archive, "declared dependencies require pinned cpanminus")
    cpanm = Path(args.cpanm).resolve(strict=True)
    archive = Path(args.cpanm_archive).resolve(strict=True)
    need(sha(archive) == CPANM_ARCHIVE_SHA256 and
         "App-cpanminus-1.7048" in cpanm.parts and cpanm.name == "cpanm",
         "cpanminus 1.7048 identity mismatch")
    stage = out / "declared-only"
    stage.mkdir()
    if declaration == "cpanfile":
        shutil.copyfile(root / declaration, stage / declaration)
    else:
        # Makefile.PL may read unchanged project version/metadata files.
        tracked = subprocess.check_output(["git", "-C", str(root), "ls-files", "-z"])
        for raw in tracked.split(b"\0"):
            if not raw:
                continue
            relative = Path(os.fsdecode(raw))
            source, target = root / relative, stage / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            if source.is_symlink():
                need(source.resolve(strict=True).is_relative_to(root),
                     "tracked installation symlink escapes subject")
                target.symlink_to(os.readlink(source))
            elif source.is_file():
                shutil.copy2(source, target)
    prefix, archives = out / "local-perl", out / "saved-dists"
    prefix.mkdir()
    archives.mkdir()
    phase("install-declared-dependencies", [args.perl, cpanm, "--notest", "--installdeps",
          "--no-lwp", "--no-wget", "--curl", "--no-man-pages", "--mirror",
          "https://cpan.metacpan.org", "--mirror-only", "--save-dists", archives,
          "--local-lib-contained", prefix, stage], out, out, env)
    sys.path.insert(0, str(range_dir))
    from dependencies import fingerprint
    state = fingerprint(prefix, archives)
    lock = {"format": "perl-dependency-lock-v1", "subject": args.subject,
            "declared_dependencies": {"path": declaration, "sha256": sha(root / declaration)},
            "local_lib_perl5": state["local_lib_perl5"]}
    lock_path = out / "dependency-lock.json"
    lock_path.write_text(json.dumps(lock, sort_keys=True, indent=2) + "\n")
    (out / "dependency-archives.private.json").write_text(
        json.dumps(state, sort_keys=True, indent=2) + "\n")
    return {"declaration": declaration, "prefix": prefix,
            "lock": lock_path, "fingerprint": state["local_lib_perl5"]}


def spend(scratch, source_commit, instrument_commit, registry=None):
    marker = scratch / "holdout-spent.json"
    need(not marker.exists(), "holdout already spent")
    record = {"source_commit": source_commit, "instrument_commit": instrument_commit,
              "spent_before": "clone-once", "spent_at_utc": datetime.now(timezone.utc).isoformat(),
              "status": "holdout exposed; no source-tuned rerun"}
    contents = json.dumps(record, sort_keys=True, indent=2) + "\n"
    if registry is not None:
        registry.mkdir(parents=True, exist_ok=True)
        with (registry / (source_commit + ".json")).open("x") as stream:
            stream.write(contents)
    with marker.open("x") as stream:
        stream.write(contents)


def run(args):
    repo = Path(args.instrument_repo).resolve(strict=True)
    need(re.fullmatch(r"[A-Za-z][A-Za-z0-9_-]*", args.subject) and
         HEX.fullmatch(args.source_commit) and
         args.subject not in {"Dancer2", "mojo", "Perl-Critic"} and
         args.source_commit not in ALREADY_MEASURED_COMMITS,
         "fresh subject identifier and full unmeasured source commit required")
    need(git(repo, "rev-parse", "--show-toplevel") == str(repo), "instrument repo root required")
    need(git(repo, "rev-parse", "HEAD") == args.instrument_commit and
         HEX.fullmatch(args.instrument_commit) and
         not git(repo, "status", "--porcelain", "--untracked-files=all"),
         "instrument must be frozen clean commit")
    assigned_caches(repo)
    scratch = Path(args.scratch).resolve()
    need(not scratch.exists() and (scratch.is_relative_to(repo / "workspace") or
         scratch.is_relative_to(Path(os.environ["TMPDIR"]).resolve(strict=True))),
         "fresh workspace/cache scratch required")
    range_dir = repo / "calibration/oracles/perl-db-ranges"
    ppi = repo / "calibration/oracles/ppi-calls/ppi_calls.pl"
    need(range_dir.is_dir() and ppi.is_file(), "pinned oracles missing")
    # Check tool identities before the first source read spends the holdout.
    sys.path.insert(0, str(range_dir))
    import runner
    need(all(Path(value).is_absolute() for value in
             (args.perl, args.ctags, args.cpanm, args.cpanm_archive)),
         "tool inputs must be absolute paths")
    preflight_env = os.environ.copy()
    for key in ("PERL5LIB", "PERLLIB", "PERL5OPT"):
        preflight_env.pop(key, None)
    perl_version = subprocess.check_output([args.perl, "-e", "print $^V"],
                                           env=preflight_env, text=True, timeout=5)
    need(perl_version == "v5.34.1", "Perl must be 5.34.1")
    ppi_version = subprocess.check_output([args.perl, "-MPPI", "-e", "print $PPI::VERSION"],
                                          env=preflight_env, text=True, timeout=5)
    need(ppi_version == "1.270", "PPI must be 1.270")
    ppi_module = subprocess.check_output([args.perl, "-MPPI", "-e", "print $INC{'PPI.pm'}"],
                                         env=preflight_env, text=True, timeout=5)
    ctags = runner.ctags_identity(args.ctags)
    cpanm_path = Path(args.cpanm).resolve(strict=True)
    need(cpanm_path.name == "cpanm" and "App-cpanminus-1.7048" in cpanm_path.parts and
         sha(Path(args.cpanm_archive).resolve(strict=True)) == CPANM_ARCHIVE_SHA256,
         "pinned cpanminus inputs required")
    scratch.mkdir(parents=True)
    root = scratch / (args.subject + "-source")
    env = os.environ.copy()
    env["FATHOM_RANGE_DIR"] = str(range_dir)
    env["FATHOM_INSTRUMENT_COMMIT"] = args.instrument_commit
    env["FATHOM_SOURCE_COMMIT"] = args.source_commit
    (scratch / "cpanm-home").mkdir()
    env["PERL_CPANM_HOME"] = str(scratch / "cpanm-home")
    env.pop("PERL_CPANM_OPT", None)
    env.update({"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local", "GOWORK": "off"})
    for key in ("PERL5LIB", "PERLLIB", "PERL5OPT"):
        env.pop(key, None)
    (scratch / "preflight.private.json").write_text(json.dumps({
        "instrument_commit": args.instrument_commit,
        "source_commit": args.source_commit,
        "perl_binary_sha256": sha(args.perl),
        "ctags_binary_sha256": ctags["binary_sha256"],
        "ppi_script_sha256": sha(ppi),
        "ppi_module_sha256": sha(ppi_module),
        "cpanm_binary_sha256": sha(cpanm_path),
        "cpanm_archive_sha256": sha(args.cpanm_archive),
        "range_runner_sha256": sha(range_dir / "runner.py"),
        "holdout_runner_sha256": sha(__file__),
    }, sort_keys=True, indent=2) + "\n")
    spend(scratch, args.source_commit, args.instrument_commit,
          repo / "workspace/tmp/perl-holdout-spent")
    phase("clone-once", ["git", "clone", "--no-checkout", args.remote, root], scratch,
          scratch, env)
    phase("checkout-pin", ["git", "-C", root, "checkout", "--detach", args.source_commit], scratch,
          scratch, env)
    before = runner.snapshot(root)
    need(before["commit"] == args.source_commit, "holdout source pin mismatch")
    deps = install_dependencies(args, root, scratch, env, range_dir)
    primary = scratch / (args.subject + ".json")
    phase("ppi-primary", [args.perl, ppi, root], scratch, scratch, env, stdout=primary)
    primary_data = json.loads(primary.read_text())
    need(primary_data["tool"] == "ppi-calls" and primary_data["version"] == "PPI 1.270" and
         primary_data["tier"] == "C", "primary oracle contract mismatch")
    range_argv = [sys.executable, range_dir / "runner.py", "--perl", args.perl,
                  "--ctags", args.ctags, "--subject", args.subject + "=" + str(root),
                  "--out", scratch / (args.subject + ".ranges.json"),
                  "--log-dir", scratch / "range-raw"]
    if deps:
        range_argv += ["--perl-lib", deps["prefix"] / "lib/perl5",
                       "--dependency-lock", deps["lock"],
                       "--declared-dependencies", deps["declaration"]]
    phase("compiler-ranges", range_argv, scratch, scratch, env)
    bench = scratch / "indexbench"
    phase("build-indexbench", ["go", "build", "-trimpath", "-buildvcs=false", "-o",
          bench, "./cmd/indexbench"], repo, scratch, env)
    phase("grade-holdout", [bench, "lang-oracle", "-dialect", "perl", "-subject", root,
          "-subject-name", args.subject, "-ctags", args.ctags, "-tool-oracle", primary,
          "-out", scratch / "reading.json", "-commit", args.instrument_commit],
          repo, scratch, env)
    phase("retime-three", [bench, "lang-retime", "-reading", scratch / "reading.json",
          "-subject", args.subject + "=" + str(root), "-runs", "3", "-write"],
          repo, scratch, env)
    need(runner.snapshot(root) == before, "holdout source changed")
    ranges = json.loads((scratch / (args.subject + ".ranges.json")).read_text())
    meta = json.loads((scratch / (args.subject + ".ranges.meta.json")).read_text())
    reading = json.loads((scratch / "reading.json").read_text())
    need(ranges["units"] == len(before["files"]) and
         meta["compiled_files"] + meta["unresolved_files"] == ranges["units"] and
         ranges["errored"] == meta["unresolved_files"], "unresolved range units lost")
    need(meta["source_manifest"] == before, "range source manifest mismatch")
    need(reading["fathom_commit"] == args.instrument_commit and
         len(reading["subjects"]) == 1 and reading["subjects"][0]["sha"] == args.source_commit,
         "reading identity mismatch")
    runs = re.findall(r"^run ([0-9]+): ([0-9]+\.[0-9]+) MB/CPU-s$",
                      (scratch / "retime-three.log").read_text(), re.M)
    need([label for label, _ in runs] == ["1", "2", "3"],
         "retime did not produce exactly three numbered runs")
    run_values = [float(value) for _, value in runs]
    measured = {"holdout_source_commit": args.source_commit, "instrument_commit": args.instrument_commit,
                "dependency_declaration": deps["declaration"] if deps else None,
                "dependency_install_attempted": deps is not None,
                "source_manifest_sha256": before["digest"], "source_units": len(before["files"]),
                "range_compiled": meta["compiled_files"],
                "range_unresolved": meta["unresolved_files"],
                "range_tag_total": meta["joining"]["tag_total"],
                "range_paired": meta["joining"]["paired"],
                "range_eligible": reading["counts"]["range_eligible"],
                "range_ok": reading["counts"]["range_ok"],
                "grade": reading["grade"], "metric_tiers": reading["metric_tiers"],
                "metrics": reading["metrics"], "counts": reading["counts"],
                "throughput_mb_s": reading["tenancy"]["throughput_mb_s"],
                "throughput_three_runs_mb_cpu_s": run_values,
                "throughput_median_mb_cpu_s": statistics.median(run_values),
                "range_agreement_at_least_0_90":
                    reading["metrics"]["range_agreement"] is not None and
                    reading["metrics"]["range_agreement"] >= .90,
                "l1_excellent": reading["grade"].get("L1") == "excellent",
                "throughput_median_at_least_1_mb_cpu_s": statistics.median(run_values) >= 1,
                "max_file_at_most_2_seconds": reading["tenancy"]["max_file_ms"] <= 2000,
                "fault_rate_at_most_0_1_percent":
                    reading["tenancy"]["faults"] / max(1, reading["subjects"][0]["files"]) <= .001,
                "ppi_sha256": sha(primary), "ranges_sha256": sha(scratch / (args.subject + ".ranges.json")),
                "reading_sha256": sha(scratch / "reading.json"),
                "binary_sha256": sha(bench), "ctags_binary_sha256": ctags["binary_sha256"]}
    measured["all_preregistered_excellence_gates"] = all(measured[key] for key in
        ("range_agreement_at_least_0_90", "l1_excellent",
         "throughput_median_at_least_1_mb_cpu_s", "max_file_at_most_2_seconds",
         "fault_rate_at_most_0_1_percent"))
    (scratch / "measured.private.json").write_text(json.dumps(measured, sort_keys=True, indent=2) + "\n")
    receipt_path = scratch / "holdout-receipt.json"
    subprocess.run([sys.executable, str(range_dir / "holdout_receipt.py"),
                    "--scratch", str(scratch), "--instrument-repo", str(repo),
                    "--instrument-commit", args.instrument_commit,
                    "--subject", args.subject, "--source-commit", args.source_commit,
                    "--coverage-floor", ".95", "--output", str(receipt_path)],
                   cwd=repo, env=env, check=True, timeout=30)
    return {"measured": measured, "receipt_sha256": sha(receipt_path),
            "receipt": json.loads(receipt_path.read_text())}


def self_test():
    workspace_tmp = Path(git(HERE, "rev-parse", "--show-toplevel")) / "workspace/tmp"
    workspace_tmp.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="perl-holdout-self-test-", dir=workspace_tmp) as dirname:
        base = Path(dirname)
        source = base / "subject"
        source.mkdir()
        subprocess.run(["git", "init", "-q", str(source)], check=True, timeout=5)
        (source / "cpanfile").write_text("requires 'Example::Own';\n")
        (source / "Makefile.PL").write_text("1;\n")
        subprocess.run(["git", "-C", str(source), "add", "cpanfile", "Makefile.PL"],
                       check=True, timeout=5)
        need(declared(source) == "cpanfile", "dependency declaration order changed")
        (source / "cpanfile").unlink()
        need(declared(source) == "Makefile.PL", "missing declaration fallback")
        env = os.environ.copy()
        env["FATHOM_INSTRUMENT_COMMIT"] = "a" * 40
        env["FATHOM_SOURCE_COMMIT"] = "b" * 40
        registry = base / "spent-register"
        spend(base, env["FATHOM_SOURCE_COMMIT"], env["FATHOM_INSTRUMENT_COMMIT"], registry)
        need((registry / ("b" * 40 + ".json")).is_file(), "spend register missing")
        try:
            spend(base / "second-scratch", env["FATHOM_SOURCE_COMMIT"],
                  env["FATHOM_INSTRUMENT_COMMIT"], registry)
        except FileExistsError:
            pass
        else:
            raise AssertionError("spent source was reused in a fresh scratch")
        check_spent = ("from pathlib import Path; import sys; "
                       "assert Path(sys.argv[1]).is_file()")
        phase("clone-once", [sys.executable, "-c", check_spent,
                             base / "holdout-spent.json"], base, base, env,
              seconds=5, load_reader=lambda: (1.0,))
        need(json.loads((base / "phases.json").read_text())[0]["source_commit"] == "b" * 40,
             "phase source identity missing")
        (base / "indexbench").write_bytes(b"synthetic binary")
        phase("build-indexbench", [sys.executable, "-c", "pass"], base, base, env,
              seconds=5, load_reader=lambda: (1.0,))
        rows = json.loads((base / "phases.json").read_text())
        need(rows[1]["binary_sha256"] == sha(base / "indexbench") and
             rows[1]["instrument_commit"] == "a" * 40 and
             rows[1]["started_utc"] and rows[1]["ended_utc"],
             "phase identity or binary hash missing")
        child_pid_file = base / "term-ignoring-child.pid"
        spawn_ignorer = (
            "import os, signal, sys, time\n"
            "child = os.fork()\n"
            "if child == 0:\n"
            "    signal.signal(signal.SIGTERM, signal.SIG_IGN)\n"
            "    with open(sys.argv[1], 'w') as marker: marker.write(str(os.getpid()))\n"
            "    while True: time.sleep(.1)\n"
            "while True: time.sleep(.1)\n"
        )
        try:
            phase("timeout-control", [sys.executable, "-c", spawn_ignorer,
                                      child_pid_file], base, base, env,
                  seconds=1, load_reader=lambda: (1.0,))
        except ValueError:
            pass
        else:
            raise AssertionError("phase timeout was accepted")
        need(json.loads((base / "phases.json").read_text())[-1]["timed_out"],
             "timeout receipt missing")
        need(child_pid_file.is_file(), "TERM-ignoring child never started")
        child_pid = int(child_pid_file.read_text())
        def child_running():
            state = subprocess.run(["ps", "-o", "stat=", "-p", str(child_pid)],
                                   text=True, capture_output=True, timeout=2).stdout.strip()
            return bool(state) and not state.startswith("Z")
        end = time.monotonic() + 2
        while child_running() and time.monotonic() < end:
            time.sleep(.02)
        need(not child_running(), "TERM-ignoring descendant escaped process-group cleanup")
        sys.path.insert(0, str(HERE))
        import holdout_receipt
        failed = holdout_receipt.coverage_criteria(94, 100, 89, 95, 100, 0, .95)
        need(not failed["compiled_coverage_floor_passed"] and
             not failed["harness_population_complete"], "failed coverage accepted")
        holdout_receipt.public_shape({"criteria": failed, "units": 100,
                                      "harness_files": 89, "native_minus_harness_files": 11})
    return {"spent_before_read": "pass", "dependency_selection": "pass",
            "phase_identity": "pass", "timeout": "rejected",
            "failed_coverage_receipt": "publishable"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--instrument-repo")
    parser.add_argument("--instrument-commit")
    parser.add_argument("--subject")
    parser.add_argument("--source-commit")
    parser.add_argument("--remote")
    parser.add_argument("--scratch")
    parser.add_argument("--perl")
    parser.add_argument("--ctags")
    parser.add_argument("--cpanm")
    parser.add_argument("--cpanm-archive")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        need(not any(value for key, value in vars(args).items() if key != "self_test"),
             "self-test takes no run inputs")
        print(json.dumps(self_test(), sort_keys=True))
        return
    need(all(value for key, value in vars(args).items() if key != "self_test"),
         "all pinned run inputs required")
    result = run(args)
    print(json.dumps({"receipt_sha256": result["receipt_sha256"],
                      "source_units": result["receipt"]["units"],
                      "coverage_and_existing_gates_pass":
                          result["receipt"]["criteria"]["coverage_and_existing_gates_pass"]},
                     sort_keys=True))


if __name__ == "__main__":
    main()
