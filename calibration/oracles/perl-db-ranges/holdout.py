#!/usr/bin/env python3
"""One-shot, pinned Perl range holdout; select and freeze a subject before running."""

import argparse
from datetime import datetime, timezone
import hashlib
import importlib.util
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
LOAD_WAIT_SECONDS = 600
HERE = Path(__file__).resolve().parent
HEX = re.compile(r"^[0-9a-f]{40}$")
PIN_BLOCK = re.compile(r"(?ms)^## Next holdout pin\n```json\n(\{.*?\})\n```(?:\n|\Z)")
ALREADY_MEASURED_COMMITS = {
    "90b6e651c1f1a7c576072fdd039ef15afecb67a5",  # Dancer2 spent
    "7b99f4adce82a45fd0965ef4dd56550b4f5ee89b",  # mojo canonical
    "382d701fd1129822c98fdcdb08403a733a24dc08",  # Perl-Critic canonical
}
ACTIVE_PROCESS = None
INTERRUPT_SIGNALS = {signal.SIGINT, signal.SIGTERM}


class LoadWaitExceeded(ValueError):
    pass


class TerminationRequested(BaseException):
    pass


def need(ok, message):
    if not ok:
        raise ValueError(message)


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], text=True,
                                   timeout=10).strip()


def frozen_clean(repo, commit):
    need(git(repo, "rev-parse", "HEAD") == commit and
         not git(repo, "status", "--porcelain", "--untracked-files=all"),
         "instrument HEAD or tracked/untracked cleanliness changed")


def pinned_subject(repo, instrument_commit, subject, source_commit, remote):
    protocol = subprocess.check_output(
        ["git", "-C", str(repo), "show", instrument_commit + ":calibration/oracles/perl-db-ranges/PROTOCOL.md"],
        text=True, timeout=10)
    blocks = PIN_BLOCK.findall(protocol)
    need(len(blocks) == 1, "exactly one committed next-holdout pin required")
    pin = json.loads(blocks[0])
    need(isinstance(pin, dict) and set(pin) ==
         {"subject", "source_commit", "remote", "license", "dependency_declaration"} and
         all(isinstance(value, str) and value.strip() == value and value for value in pin.values()) and
         re.fullmatch(r"[A-Za-z][A-Za-z0-9_-]*", pin["subject"]) and
         HEX.fullmatch(pin["source_commit"]) and
         pin["dependency_declaration"] in
         {"none", "cpanfile", "Makefile.PL", "Build.PL", "META.json", "META.yml"} and
         pin["subject"] == subject and pin["source_commit"] == source_commit and
         pin["remote"] == remote, "run inputs differ from committed holdout pin")
    return pin


def common_spend_register(repo):
    common = Path(git(repo, "rev-parse", "--git-common-dir"))
    if not common.is_absolute():
        common = repo / common
    return common.resolve(strict=True) / "perl-holdout-spent"


def wait_load(deadline, load_reader=os.getloadavg):
    while True:
        load = load_reader()[0]
        if load <= 9:
            return load
        if time.monotonic() >= deadline:
            raise LoadWaitExceeded("load wait exceeded its independent budget")
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


def terminate_active_group(_signum, _frame):
    if ACTIVE_PROCESS is not None:
        stop_group(ACTIVE_PROCESS)
    raise TerminationRequested("runner received SIGTERM")


def launch_owned(argv, **kwargs):
    """Do not deliver an interrupt between child creation and ownership."""
    global ACTIVE_PROCESS
    previous_mask = signal.pthread_sigmask(signal.SIG_BLOCK, INTERRUPT_SIGNALS)
    try:
        proc = subprocess.Popen(
            argv, start_new_session=True,
            preexec_fn=lambda: signal.pthread_sigmask(signal.SIG_SETMASK, previous_mask),
            **kwargs)
        ACTIVE_PROCESS = proc
        return proc
    finally:
        signal.pthread_sigmask(signal.SIG_SETMASK, previous_mask)


def atomic_write_bytes(path, contents):
    private = None
    try:
        with tempfile.NamedTemporaryFile(mode="wb", dir=path.parent,
                                         prefix=path.name + ".private-", delete=False) as stream:
            private = Path(stream.name)
            stream.write(contents)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(private, path)
    finally:
        if private is not None:
            private.unlink(missing_ok=True)


def update_phases(out, update):
    phases = out / "phases.json"
    previous_mask = signal.pthread_sigmask(signal.SIG_BLOCK, INTERRUPT_SIGNALS)
    try:
        existing = json.loads(phases.read_text()) if phases.exists() else []
        update(existing)
        atomic_write_bytes(phases, (json.dumps(existing, sort_keys=True, indent=2) + "\n").encode())
    finally:
        signal.pthread_sigmask(signal.SIG_SETMASK, previous_mask)


def append_phase(out, row):
    update_phases(out, lambda existing: existing.append(row))


def mark_contract_failure(out, name, env, failure=None):
    label = ("terminated" if isinstance(failure, TerminationRequested) else
             "interrupted" if isinstance(failure, (KeyboardInterrupt, SystemExit)) else
             "contract_failed")
    def update(existing):
        if existing and existing[-1]["phase"] == name:
            if existing[-1].get("failure") is None and existing[-1]["exit"] == 0:
                existing[-1]["failure"] = label
                existing[-1]["exit"] = None
        elif not existing or existing[-1]["phase"] != name:
            now = datetime.now(timezone.utc).isoformat()
            existing.append({"phase": name, "exit": None, "timed_out": False,
                             "launch_load": None, "seconds": 0, "load_wait_seconds": 0,
                             "started_utc": now, "ended_utc": now,
                             "instrument_commit": env["FATHOM_INSTRUMENT_COMMIT"],
                             "source_commit": env["FATHOM_SOURCE_COMMIT"],
                             "failure": label})
    update_phases(out, update)


def phase(name, argv, cwd, out, env, seconds=MAX_SECONDS, stdout=None,
          load_reader=os.getloadavg, retime_input_sha256=None,
          load_wait_seconds=LOAD_WAIT_SECONDS, prelaunch_check=None):
    global ACTIVE_PROCESS
    started_utc = datetime.now(timezone.utc).isoformat()
    row = {"phase": name, "exit": None, "timed_out": False,
           "launch_load": None, "seconds": 0, "load_wait_seconds": 0,
           "started_utc": started_utc, "ended_utc": datetime.now(timezone.utc).isoformat(),
           "instrument_commit": env["FATHOM_INSTRUMENT_COMMIT"],
           "source_commit": env["FATHOM_SOURCE_COMMIT"]}
    proc = None
    load_started = time.monotonic()
    launch_started = None
    try:
        if retime_input_sha256 is not None:
            need(name == "retime-three" and re.fullmatch(r"[0-9a-f]{64}", retime_input_sha256),
                 "retime input digest placement malformed")
        nice = shutil.which("nice")
        need(nice is not None, "nice executable required")
        load = wait_load(load_started + load_wait_seconds, load_reader)
        row["launch_load"] = load
        row["load_wait_seconds"] = round(time.monotonic() - load_started, 3)
        if prelaunch_check is not None:
            try:
                prelaunch_check()
            except BaseException:
                row["failure"] = "contract_failed"
                raise
        launch_started = time.monotonic()
        deadline = launch_started + min(seconds, MAX_SECONDS)
        if time.monotonic() >= deadline:
            row["failure"] = "prelaunch_deadline"
            raise ValueError("phase deadline elapsed before launch")
        log = out / (name + ".log")
        with log.open("wb") as stream:
            target = stream if stdout is None else stdout.open("xb")
            try:
                proc = launch_owned([nice, "-n", "10", *map(str, argv)],
                                    cwd=cwd, env=env, stdout=target, stderr=stream)
                try:
                    proc.wait(timeout=max(.01, deadline - time.monotonic()))
                except subprocess.TimeoutExpired:
                    row["timed_out"] = True
                finally:
                    stop_group(proc)
            finally:
                if stdout is not None:
                    target.close()
        row["exit"] = proc.returncode
        row["seconds"] = round(time.monotonic() - launch_started, 3)
    except BaseException as failure:
        owned = proc or ACTIVE_PROCESS
        if owned is not None:
            stop_group(owned)
            row["exit"] = owned.returncode
        if isinstance(failure, LoadWaitExceeded):
            row["failure"] = "load_wait_exceeded"
        elif isinstance(failure, TerminationRequested):
            row["failure"] = "terminated"
        elif isinstance(failure, (KeyboardInterrupt, SystemExit)):
            row["failure"] = "interrupted"
        else:
            row.setdefault("failure", "launch_failed" if proc is None else "contract_failed")
        raise
    finally:
        if launch_started is None:
            row["load_wait_seconds"] = round(time.monotonic() - load_started, 3)
        else:
            row["seconds"] = round(time.monotonic() - launch_started, 3)
        row["ended_utc"] = datetime.now(timezone.utc).isoformat()
        if name in ("build-indexbench", "grade-holdout", "retime-three"):
            binary = out / "indexbench"
            if binary.is_file():
                row["binary_sha256"] = sha(binary)
        if retime_input_sha256 is not None and re.fullmatch(r"[0-9a-f]{64}", retime_input_sha256):
            row["retime_input_sha256"] = retime_input_sha256
        try:
            append_phase(out, row)
        finally:
            ACTIVE_PROCESS = None
    need(not row["timed_out"] and row["exit"] == 0, "phase failed: " + name)
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


def spend(scratch, source_commit, instrument_commit, registry=None,
          after_registry=None):
    marker = scratch / "holdout-spent.json"
    need(not marker.exists(), "holdout already spent")
    record = {"source_commit": source_commit, "instrument_commit": instrument_commit,
              "spent_before": "clone-once", "spent_at_utc": datetime.now(timezone.utc).isoformat(),
              "status": "holdout exposed; no source-tuned rerun"}
    contents = json.dumps(record, sort_keys=True, indent=2) + "\n"
    reservation = None
    reservation_created = False
    marker_created = False
    try:
        if registry is not None:
            registry.mkdir(parents=True, exist_ok=True)
            reservation = registry / (source_commit + ".json")
            with reservation.open("x") as stream:
                reservation_created = True
                stream.write(contents)
                stream.flush()
                os.fsync(stream.fileno())
        if after_registry is not None:
            after_registry()
        with marker.open("x") as stream:
            marker_created = True
            stream.write(contents)
            stream.flush()
            os.fsync(stream.fileno())
    except BaseException:
        # No clone has launched: release only the reservation created here.
        if marker_created:
            marker.unlink(missing_ok=True)
        if reservation_created:
            reservation.unlink(missing_ok=True)
        raise


def protected_spend(scratch, source_commit, instrument_commit, registry, ownership,
                    after_registry=None):
    previous_mask = signal.pthread_sigmask(signal.SIG_BLOCK, INTERRUPT_SIGNALS)
    try:
        spend(scratch, source_commit, instrument_commit, registry, after_registry)
        ownership["spent"] = True
    finally:
        signal.pthread_sigmask(signal.SIG_SETMASK, previous_mask)


def frozen_collector(scratch, repo, instrument_commit, prepare=False):
    frozen = scratch / "frozen-collector"
    oracle = frozen / "calibration/oracles/perl-db-ranges"
    if prepare:
        oracle.mkdir(parents=True, exist_ok=True)
    else:
        need(oracle.is_dir(), "frozen collector was not prepared before spend")
    git_dir = Path(git(repo, "rev-parse", "--absolute-git-dir")).resolve(strict=True)
    link = frozen / ".git"
    if prepare and not link.exists() and not link.is_symlink():
        link.symlink_to(git_dir, target_is_directory=True)
    need(link.is_symlink() and link.resolve(strict=True) == git_dir,
         "frozen collector Git identity changed")
    for name in ("holdout_receipt.py", "runner.py", "dependencies.py",
                 "FathomPerlCompileGuard.pm"):
        relative = "calibration/oracles/perl-db-ranges/" + name
        frozen_bytes = subprocess.check_output(
            ["git", "-C", str(repo), "show", instrument_commit + ":" + relative],
            timeout=10)
        target = oracle / name
        if target.exists():
            need(target.read_bytes() == frozen_bytes, "frozen collector bytes changed")
        else:
            need(prepare, "frozen collector file missing after spend")
            atomic_write_bytes(target, frozen_bytes)
        need(hashlib.sha256(target.read_bytes()).digest() ==
             hashlib.sha256(frozen_bytes).digest(), "frozen collector hash mismatch")
    return frozen


def frozen_receipt_module(frozen):
    source = frozen / "calibration/oracles/perl-db-ranges/holdout_receipt.py"
    spec = importlib.util.spec_from_file_location("perl_frozen_holdout_receipt", source)
    need(spec is not None and spec.loader is not None, "frozen receipt module unavailable")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    need(Path(module.__file__).resolve(strict=True) == source.resolve(strict=True),
         "retime helper did not load from frozen collector")
    return module


def private_receipt_path(scratch, kind):
    for index in range(1000):
        path = scratch / ("holdout-receipt.private." + kind + "-" + str(index) + ".json")
        if not path.exists():
            return path
    raise ValueError("private receipt slots exhausted")


def archive_existing_receipt(scratch):
    receipt_path = scratch / "holdout-receipt.json"
    if receipt_path.exists():
        receipt_path.replace(private_receipt_path(scratch, "superseded"))


def collect_receipt(scratch, repo, instrument_commit, subject, source_commit, env,
                    failed_run=False):
    global ACTIVE_PROCESS
    receipt_path = scratch / "holdout-receipt.json"
    need(not receipt_path.exists(), "holdout receipt already exists")
    candidate = private_receipt_path(scratch, "candidate")
    collector_repo = frozen_collector(scratch, repo, instrument_commit)
    argv = [sys.executable, str(collector_repo / "calibration/oracles/perl-db-ranges/holdout_receipt.py"),
                    "--scratch", str(scratch), "--instrument-repo", str(collector_repo),
                    "--instrument-commit", instrument_commit,
                    "--subject", subject, "--source-commit", source_commit,
                    "--coverage-floor", ".95", "--output", str(candidate)]
    if failed_run:
        argv.append("--failed-run")
    proc = None
    try:
        proc = launch_owned(argv, cwd=repo, env=env)
        try:
            code = proc.wait(timeout=30)
        finally:
            stop_group(proc)
        if code:
            raise subprocess.CalledProcessError(code, argv)
        need(candidate.is_file(), "collector succeeded without candidate receipt")
        candidate.replace(receipt_path)
    finally:
        owned = proc or ACTIVE_PROCESS
        if owned is not None:
            stop_group(owned)
        ACTIVE_PROCESS = None
    return receipt_path


def publish_failed_receipt(scratch, repo, instrument_commit, subject, source_commit, env):
    archive_existing_receipt(scratch)
    receipt_path = collect_receipt(scratch, repo, instrument_commit,
                                   subject, source_commit, env, failed_run=True)
    receipt = json.loads(receipt_path.read_text())
    need(receipt["criteria"]["coverage_and_existing_gates_pass"] is False,
         "failed phase produced a passing receipt")
    return receipt_path


def run(args):
    repo = Path(args.instrument_repo).resolve(strict=True)
    need(re.fullmatch(r"[A-Za-z][A-Za-z0-9_-]*", args.subject) and
         HEX.fullmatch(args.source_commit) and
         args.subject not in {"Dancer2", "mojo", "Perl-Critic"} and
         args.source_commit not in ALREADY_MEASURED_COMMITS,
         "fresh subject identifier and full unmeasured source commit required")
    need(git(repo, "rev-parse", "--show-toplevel") == str(repo), "instrument repo root required")
    need(HEX.fullmatch(args.instrument_commit), "full instrument commit required")
    frozen_clean(repo, args.instrument_commit)
    pin = pinned_subject(repo, args.instrument_commit, args.subject,
                         args.source_commit, args.remote)
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
    frozen = frozen_collector(scratch, repo, args.instrument_commit, prepare=True)
    frozen_receipt = frozen_receipt_module(frozen)
    frozen_clean(repo, args.instrument_commit)
    previous_term_handler = signal.signal(signal.SIGTERM, terminate_active_group)
    current_phase = "clone-once"
    ownership = {"spent": False}
    try:
        protected_spend(scratch, args.source_commit, args.instrument_commit,
                        common_spend_register(repo), ownership)
        phase("clone-once", ["git", "clone", "--no-checkout", args.remote, root], scratch,
              scratch, env)
        current_phase = "checkout-pin"
        phase("checkout-pin", ["git", "-C", root, "checkout", "--detach", args.source_commit], scratch,
              scratch, env)
        before = runner.snapshot(root)
        need(before["commit"] == args.source_commit, "holdout source pin mismatch")
        need(declared(root) == (None if pin["dependency_declaration"] == "none" else
             pin["dependency_declaration"]), "subject dependency declaration differs from pin")
        current_phase = "install-declared-dependencies" if declared(root) else "ppi-primary"
        deps = install_dependencies(args, root, scratch, env, range_dir)
        primary = scratch / (args.subject + ".json")
        current_phase = "ppi-primary"
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
        current_phase = "compiler-ranges"
        phase("compiler-ranges", range_argv, scratch, scratch, env)
        bench = scratch / "indexbench"
        current_phase = "build-indexbench"
        phase("build-indexbench", ["go", "build", "-trimpath", "-buildvcs=false", "-o",
              bench, "./cmd/indexbench"], repo, scratch, env,
              prelaunch_check=lambda: frozen_clean(repo, args.instrument_commit))
        current_phase = "grade-holdout"
        phase("grade-holdout", [bench, "lang-oracle", "-dialect", "perl", "-subject", root,
              "-subject-name", args.subject, "-ctags", args.ctags, "-tool-oracle", primary,
              "-out", scratch / "reading.json", "-commit", args.instrument_commit],
              repo, scratch, env)
        immutable_reading = json.loads((scratch / "reading.json").read_text())
        retime_digest = frozen_receipt.retime_input_digest(
            before["digest"], sha(primary),
            sha(scratch / (args.subject + ".ranges.json")), sha(bench), immutable_reading)
        current_phase = "retime-three"
        phase("retime-three", [bench, "lang-retime", "-reading", scratch / "reading.json",
              "-subject", args.subject + "=" + str(root), "-runs", "3", "-write"],
              repo, scratch, env, retime_input_sha256=retime_digest)
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
        frozen_clean(repo, args.instrument_commit)
        receipt_path = collect_receipt(scratch, repo, args.instrument_commit,
                                       args.subject, args.source_commit, env)
        return {"measured": measured, "receipt_sha256": sha(receipt_path),
                "receipt": json.loads(receipt_path.read_text())}
    except BaseException as failure:
        if not ownership["spent"]:
            raise
        mark_contract_failure(scratch, current_phase, env, failure)
        try:
            receipt_path = publish_failed_receipt(scratch, repo, args.instrument_commit,
                                                  args.subject, args.source_commit, env)
        except BaseException as publication_failure:
            raise RuntimeError("spent holdout failed and partial receipt publication failed") from publication_failure
        print(json.dumps({"spent_failure": True,
                          "partial_receipt_sha256": sha(receipt_path)}, sort_keys=True),
              file=sys.stderr)
        raise
    finally:
        signal.signal(signal.SIGTERM, previous_term_handler)



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
        for label, kwargs, expected in (
            ("load-wait-control", {"load_reader": lambda: (20.0,),
                                   "load_wait_seconds": .01}, "load_wait_exceeded"),
            ("launch-control", {"cwd": base / "missing-cwd"}, "launch_failed"),
            ("deadline-control", {"seconds": 0}, "prelaunch_deadline"),
            ("interrupt-control", {"load_reader": lambda: (_ for _ in ()).throw(
                KeyboardInterrupt())}, "interrupted"),
            ("system-exit-control", {"load_reader": lambda: (_ for _ in ()).throw(
                SystemExit(3))}, "interrupted"),
            ("term-control", {"load_reader": lambda: terminate_active_group(
                signal.SIGTERM, None)}, "terminated"),
        ):
            control_cwd = kwargs.get("cwd", base)
            try:
                phase(label, [sys.executable, "-c", "pass"], control_cwd, base, env,
                      seconds=kwargs.get("seconds", 5),
                      load_reader=kwargs.get("load_reader", lambda: (1.0,)),
                      load_wait_seconds=kwargs.get("load_wait_seconds", 5))
            except BaseException:
                row = json.loads((base / "phases.json").read_text())[-1]
                need(row["phase"] == label and row["failure"] == expected and
                     row["exit"] is None, "failed attempt lacks terminal phase row")
            else:
                raise AssertionError("failed attempt was accepted: " + label)
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
            try:
                state = subprocess.run(["ps", "-o", "stat=", "-p", str(child_pid)],
                                       text=True, capture_output=True, timeout=2).stdout.strip()
            except PermissionError:
                return None  # Some agent sandboxes prohibit process inspection.
            return bool(state) and not state.startswith("Z")
        end = time.monotonic() + 2
        while child_running() and time.monotonic() < end:
            time.sleep(.02)
        need(child_running() is not True, "TERM-ignoring descendant escaped process-group cleanup")
        prior_term_handler = signal.signal(signal.SIGTERM, terminate_active_group)
        try:
            try:
                phase("sigterm-child-control", [sys.executable, "-c",
                      "import os,signal,sys,time; os.kill(int(sys.argv[1]),signal.SIGTERM); time.sleep(5)",
                      str(os.getpid())], base, base, env,
                      seconds=5, load_reader=lambda: (1.0,))
            except TerminationRequested:
                term_row = json.loads((base / "phases.json").read_text())[-1]
                need(term_row["phase"] == "sigterm-child-control" and
                     term_row["failure"] == "terminated", "SIGTERM child phase not recorded")
            else:
                raise AssertionError("SIGTERM child did not interrupt phase")
        finally:
            signal.signal(signal.SIGTERM, prior_term_handler)
        real_popen = subprocess.Popen
        launched = {}
        def signal_between_popen_and_owner(*argv, **kwargs):
            child = real_popen(*argv, **kwargs)
            launched["child"] = child
            os.kill(os.getpid(), signal.SIGTERM)
            return child
        prior_term_handler = signal.signal(signal.SIGTERM, terminate_active_group)
        subprocess.Popen = signal_between_popen_and_owner
        try:
            try:
                phase("popen-window-control", [sys.executable, "-c",
                      "import time; time.sleep(10)"], base, base, env,
                      seconds=5, load_reader=lambda: (1.0,))
            except TerminationRequested:
                window_row = json.loads((base / "phases.json").read_text())[-1]
                need(window_row["phase"] == "popen-window-control" and
                     window_row["failure"] == "terminated" and
                     launched["child"].poll() is not None,
                     "signal during Popen ownership escaped child cleanup")
            else:
                raise AssertionError("Popen ownership signal was not delivered")
        finally:
            subprocess.Popen = real_popen
            signal.signal(signal.SIGTERM, prior_term_handler)
        atomic_phases = base / "atomic-phase-control"
        atomic_phases.mkdir()
        atomic_row = {"phase": "clone-once", "exit": 0, "timed_out": False,
                      "launch_load": 1, "seconds": 0,
                      "instrument_commit": env["FATHOM_INSTRUMENT_COMMIT"],
                      "source_commit": env["FATHOM_SOURCE_COMMIT"]}
        real_replace = os.replace
        def signal_at_replace(source_path, target_path):
            os.kill(os.getpid(), signal.SIGTERM)
            return real_replace(source_path, target_path)
        prior_term_handler = signal.signal(signal.SIGTERM, terminate_active_group)
        os.replace = signal_at_replace
        try:
            try:
                append_phase(atomic_phases, atomic_row)
            except TerminationRequested as interruption:
                rows = json.loads((atomic_phases / "phases.json").read_text())
                need(rows == [atomic_row], "interrupted phase write was truncated")
            else:
                raise AssertionError("phase-write interrupt was not delivered")
        finally:
            os.replace = real_replace
            signal.signal(signal.SIGTERM, prior_term_handler)
        mark_contract_failure(atomic_phases, "clone-once", env, TerminationRequested())
        need(json.loads((atomic_phases / "phases.json").read_text())[-1]["failure"] ==
             "terminated", "interrupted phase row was not repaired")
        instrument_source = base / "synthetic-instrument"
        instrument_source.mkdir()
        subprocess.run(["git", "init", "-q", str(instrument_source)], check=True, timeout=5)
        protocol_path = instrument_source / "calibration/oracles/perl-db-ranges/PROTOCOL.md"
        protocol_path.parent.mkdir(parents=True)
        protocol_path.write_text("No next holdout pin.\n")
        subprocess.run(["git", "-C", str(instrument_source), "add", "."], check=True, timeout=5)
        subprocess.run(["git", "-C", str(instrument_source), "-c", "user.name=Self Test",
                        "-c", "user.email=self-test@example.invalid", "commit", "-qm", "unpinned"],
                       check=True, timeout=5)
        unpinned_commit = git(instrument_source, "rev-parse", "HEAD")
        synthetic_pin = {"subject": "Synthetic", "source_commit": "b" * 40,
                         "remote": str(source), "license": "Artistic-2.0",
                         "dependency_declaration": "none"}
        protocol_path.write_text("## Next holdout pin\n```json\n" +
                                 json.dumps(synthetic_pin) + "\n```\n")
        subprocess.run(["git", "-C", str(instrument_source), "add", "."], check=True, timeout=5)
        subprocess.run(["git", "-C", str(instrument_source), "-c", "user.name=Self Test",
                        "-c", "user.email=self-test@example.invalid", "commit", "-qm", "pin"],
                       check=True, timeout=5)
        pinned_commit = git(instrument_source, "rev-parse", "HEAD")
        try:
            pinned_subject(instrument_source, unpinned_commit, "Synthetic", "b" * 40,
                           str(source))
        except ValueError:
            pass
        else:
            raise AssertionError("uncommitted holdout pin accepted")
        need(pinned_subject(instrument_source, pinned_commit, "Synthetic", "b" * 40,
                            str(source)) == synthetic_pin, "committed pin rejected")
        try:
            pinned_subject(instrument_source, pinned_commit, "Synthetic", "b" * 40,
                           str(source) + "-different")
        except ValueError:
            pass
        else:
            raise AssertionError("different remote accepted")
        protocol_path.write_text("## Next holdout pin\n```json\n" +
                                 json.dumps({**synthetic_pin, "remote": "changed"}) + "\n```\n")
        need(pinned_subject(instrument_source, pinned_commit, "Synthetic", "b" * 40,
                            str(source)) == synthetic_pin, "live protocol overrode frozen pin")
        try:
            frozen_clean(instrument_source, pinned_commit)
        except ValueError:
            pass
        else:
            raise AssertionError("dirty instrument was accepted")
        linked = base / "linked-worktree"
        subprocess.run(["git", "-C", str(instrument_source), "worktree", "add", "-q",
                        "--detach", str(linked), pinned_commit], check=True, timeout=5)
        need(common_spend_register(instrument_source) == common_spend_register(linked),
             "spend register differs between worktrees")
        protocol_path.write_text("## Next holdout pin\n```json\n" +
                                 json.dumps(synthetic_pin) + "\n```\n")
        for name in ("holdout_receipt.py", "runner.py", "dependencies.py",
                     "FathomPerlCompileGuard.pm"):
            shutil.copyfile(HERE / name, protocol_path.parent / name)
        subprocess.run(["git", "-C", str(instrument_source), "add", "."], check=True, timeout=5)
        subprocess.run(["git", "-C", str(instrument_source), "-c", "user.name=Self Test",
                        "-c", "user.email=self-test@example.invalid", "commit", "-qm", "frozen collector"],
                       check=True, timeout=5)
        (protocol_path.parent / "holdout_receipt.py").write_text(
            "raise RuntimeError('live collector must never execute')\n")
        sys.path.insert(0, str(HERE))
        import holdout_receipt
        failed = holdout_receipt.coverage_criteria(94, 100, 89, 95, 100, 0, .95)
        need(not failed["compiled_coverage_floor_passed"] and
             not failed["harness_population_complete"], "failed coverage accepted")
        holdout_receipt.public_shape({"criteria": failed, "units": 100,
                                      "harness_files": 89, "native_minus_harness_files": 11})
        failure_scratch = base / "synthetic-grade-failure"
        failure_scratch.mkdir()
        instrument = git(instrument_source, "rev-parse", "HEAD")
        interrupted_copy = base / "synthetic-interrupted-copy"
        interrupted_copy.mkdir()
        real_replace = os.replace
        def interrupt_copy(source_path, target_path):
            os.kill(os.getpid(), signal.SIGTERM)
            return real_replace(source_path, target_path)
        prior_term_handler = signal.signal(signal.SIGTERM, terminate_active_group)
        os.replace = interrupt_copy
        try:
            try:
                frozen_collector(interrupted_copy, instrument_source, instrument,
                                 prepare=True)
            except TerminationRequested:
                need(not (interrupted_copy / "frozen-collector/calibration/oracles/"
                          "perl-db-ranges/holdout_receipt.py").exists(),
                     "interrupted copy exposed a partial collector")
            else:
                raise AssertionError("frozen-copy interrupt was not delivered")
        finally:
            os.replace = real_replace
            signal.signal(signal.SIGTERM, prior_term_handler)
        frozen_collector(interrupted_copy, instrument_source, instrument, prepare=True)
        frozen_copy = frozen_collector(base, instrument_source, instrument, prepare=True)
        need(git(frozen_copy, "rev-parse", "HEAD") == instrument,
             "frozen collector lost instrument Git identity")
        frozen_receipt_module(frozen_copy)
        subject_root = failure_scratch / "Synthetic-source"
        subject_root.mkdir()
        subprocess.run(["git", "init", "-q", str(subject_root)], check=True, timeout=5)
        (subject_root / "Tiny.pm").write_text("package Tiny; sub one { 1 } 1;\n")
        subprocess.run(["git", "-C", str(subject_root), "add", "Tiny.pm"], check=True, timeout=5)
        subprocess.run(["git", "-C", str(subject_root), "-c", "user.name=Self Test",
                        "-c", "user.email=self-test@example.invalid", "commit", "-qm", "synthetic"],
                       check=True, timeout=5)
        source_commit = git(subject_root, "rev-parse", "HEAD")
        import runner
        manifest = runner.snapshot(subject_root)
        failure_env = dict(env, FATHOM_INSTRUMENT_COMMIT=instrument,
                           FATHOM_SOURCE_COMMIT=source_commit)
        frozen_collector(failure_scratch, instrument_source, instrument, prepare=True)
        spend(failure_scratch, source_commit, instrument)
        (failure_scratch / "indexbench").write_bytes(b"synthetic binary")
        (failure_scratch / "Synthetic.json").write_text(json.dumps(
            {"tool": "ppi-calls", "version": "PPI 1.270", "tier": "C"}))
        (failure_scratch / "Synthetic.ranges.meta.json").write_text(json.dumps({
            "subject": "Synthetic", "source_manifest": manifest,
            "compiled_files": 1, "unresolved_files": 0, "failures": [],
            "joining": {"files": [{"ctags_status": "ok", "compiler_status": "ok"}],
                        "tag_total": 1, "paired": 1, "ctags_unknown_files": 0,
                        "compiler_unknown_files": 0, "tag_unpaired": 0},
            "network_policy": {"control": {"denial": "EPERM", "destination": "127.0.0.1:1/tcp",
                           "profile": "(version 1) (allow default) (deny network*)"}},
            "dependency_context": {"local_lib_perl5_fingerprint_before": None,
                                   "local_lib_perl5_fingerprint_after": None,
                                   "lock_sha256": None},
            "perl_binary_sha256": "a" * 64,
            "ctags": {"version": "Universal Ctags 6.2.1, synthetic", "binary_sha256": "b" * 64}}))
        (failure_scratch / "Synthetic.ranges.json").write_text(json.dumps({
            "tool": "perl-DB-sub-compile", "version": "v5.34.1", "tier": "A",
            "units": 1, "errored": 0, "ranges": [{}]}))
        prefix = []
        for label, exit_code in (("clone-once", 0), ("checkout-pin", 0),
                                 ("ppi-primary", 0), ("compiler-ranges", 0),
                                 ("build-indexbench", 0)):
            row = {"phase": label, "exit": exit_code, "timed_out": False,
                   "launch_load": 1, "seconds": 0, "instrument_commit": instrument,
                   "source_commit": source_commit}
            if label == "build-indexbench":
                row["binary_sha256"] = sha(failure_scratch / "indexbench")
            prefix.append(row)
        interrupted_spend = base / "synthetic-interrupted-spend"
        interrupted_spend.mkdir()
        frozen_collector(interrupted_spend, instrument_source, instrument, prepare=True)
        interrupted_registry = base / "interrupted-register"
        spend_ownership = {"spent": False}
        prior_term_handler = signal.signal(signal.SIGTERM, terminate_active_group)
        try:
            try:
                protected_spend(interrupted_spend, source_commit, instrument,
                                interrupted_registry, spend_ownership,
                                after_registry=lambda: os.kill(os.getpid(), signal.SIGTERM))
            except TerminationRequested as interruption:
                need(spend_ownership["spent"] and
                     (interrupted_spend / "holdout-spent.json").is_file() and
                     (interrupted_registry / (source_commit + ".json")).is_file(),
                     "signal between register and marker lost spend ownership")
                mark_contract_failure(interrupted_spend, "clone-once", failure_env,
                                      interruption)
                spent_receipt = publish_failed_receipt(
                    interrupted_spend, instrument_source, instrument,
                    "Synthetic", source_commit, failure_env)
                need(json.loads(spent_receipt.read_text())["phase_outcomes"][-1]["failure"] ==
                     "terminated", "interrupted reservation lacked false receipt")
            else:
                raise AssertionError("reservation signal was not delivered")
        finally:
            signal.signal(signal.SIGTERM, prior_term_handler)
        (failure_scratch / "phases.json").write_text(json.dumps(prefix))
        try:
            phase("grade-holdout", [sys.executable, "-c", "import sys; sys.exit(2)"],
                  failure_scratch, failure_scratch, failure_env,
                  seconds=5, load_reader=lambda: (1.0,))
        except ValueError:
            receipt_path = publish_failed_receipt(
                failure_scratch, instrument_source,
                instrument, "Synthetic", source_commit, failure_env)
            receipt = json.loads(receipt_path.read_text())
            need(receipt["phase_outcomes"][-1]["exit"] == 2 and
                 receipt["source_manifest_sha256"] == manifest["digest"] and
                 receipt["compiled"] == 1 and receipt["named_tags"] == 1 and
                 receipt["criteria"]["coverage_and_existing_gates_pass"] is False,
                 "nonzero grade did not publish false-gate receipt")
        else:
            raise AssertionError("synthetic grade failure was accepted")
        old_receipt = failure_scratch / "holdout-receipt.json"
        old_receipt.write_text(json.dumps(
            {"criteria": {"coverage_and_existing_gates_pass": True}}))
        replacement = publish_failed_receipt(
            failure_scratch, instrument_source, instrument,
            "Synthetic", source_commit, failure_env)
        need(json.loads(replacement.read_text())["criteria"][
             "coverage_and_existing_gates_pass"] is False and
             list(failure_scratch.glob("holdout-receipt.private.superseded-*.json")),
             "existing receipt was not archived and replaced after failure")
        interrupted_collect = base / "synthetic-interrupted-collector"
        interrupted_collect.mkdir()
        frozen_collector(interrupted_collect, instrument_source, instrument, prepare=True)
        shutil.copytree(subject_root, interrupted_collect / "Synthetic-source")
        spend(interrupted_collect, source_commit, instrument)
        shutil.copyfile(failure_scratch / "Synthetic.json",
                        interrupted_collect / "Synthetic.json")
        (interrupted_collect / "phases.json").write_text(json.dumps(prefix[:3]))
        real_popen = subprocess.Popen
        collector_child = {}
        def signal_during_collector_wait(argv, **kwargs):
            if len(argv) > 1 and str(argv[1]).endswith("holdout_receipt.py"):
                candidate = argv[argv.index("--output") + 1]
                script = ("import os,pathlib,signal,sys,time; "
                          "pathlib.Path(sys.argv[1]).write_text('{}'); "
                          "os.kill(int(sys.argv[2]),signal.SIGTERM); time.sleep(10)")
                child = real_popen([sys.executable, "-c", script,
                                    candidate, str(os.getpid())], **kwargs)
                collector_child["child"] = child
                return child
            return real_popen(argv, **kwargs)
        prior_term_handler = signal.signal(signal.SIGTERM, terminate_active_group)
        subprocess.Popen = signal_during_collector_wait
        try:
            try:
                collect_receipt(interrupted_collect, instrument_source, instrument,
                                "Synthetic", source_commit, failure_env)
            except TerminationRequested as interruption:
                need(not (interrupted_collect / "holdout-receipt.json").exists() and
                     list(interrupted_collect.glob("holdout-receipt.private.candidate-*.json")) and
                     collector_child["child"].poll() is not None,
                     "interrupted collector promoted receipt or escaped cleanup")
                mark_contract_failure(interrupted_collect, "compiler-ranges",
                                      failure_env, interruption)
            else:
                raise AssertionError("collector wait signal was not delivered")
        finally:
            subprocess.Popen = real_popen
            signal.signal(signal.SIGTERM, prior_term_handler)
        recovered = publish_failed_receipt(
            interrupted_collect, instrument_source, instrument,
            "Synthetic", source_commit, failure_env)
        need(json.loads(recovered.read_text())["phase_outcomes"][-1]["failure"] ==
             "terminated", "collector interrupt did not publish terminal failure")
        for label, kwargs, expected in (
            ("load-wait", {"load_reader": lambda: (20.0,),
                           "load_wait_seconds": .01}, "load_wait_exceeded"),
            ("launch", {"cwd": base / "missing-cwd"}, "launch_failed"),
            ("interrupt", {"load_reader": lambda: (_ for _ in ()).throw(
                KeyboardInterrupt())}, "interrupted"),
            ("system-exit", {"load_reader": lambda: (_ for _ in ()).throw(
                SystemExit(3))}, "interrupted"),
            ("term", {"load_reader": lambda: terminate_active_group(
                signal.SIGTERM, None)}, "terminated"),
        ):
            failing = base / ("synthetic-" + label + "-receipt")
            failing.mkdir()
            frozen_collector(failing, instrument_source, instrument, prepare=True)
            shutil.copytree(subject_root, failing / "Synthetic-source")
            spend(failing, source_commit, instrument)
            shutil.copyfile(failure_scratch / "Synthetic.json", failing / "Synthetic.json")
            (failing / "phases.json").write_text(json.dumps(prefix[:3]))
            try:
                phase("compiler-ranges", [sys.executable, "-c", "pass"],
                      kwargs.get("cwd", failing), failing, failure_env,
                      seconds=5, load_reader=kwargs.get("load_reader", lambda: (1.0,)),
                      load_wait_seconds=kwargs.get("load_wait_seconds", 5))
            except BaseException:
                published = publish_failed_receipt(
                    failing, instrument_source,
                    instrument, "Synthetic", source_commit, failure_env)
                outcome = json.loads(published.read_text())["phase_outcomes"][-1]
                need(outcome["phase"] == "compiler-ranges" and
                     outcome["failure"] == expected and outcome["exit"] is None,
                     "prelaunch failure receipt omitted failing phase")
            else:
                raise AssertionError("prelaunch failure was accepted: " + label)
        contract_scratch = base / "synthetic-contract-receipt"
        contract_scratch.mkdir()
        frozen_collector(contract_scratch, instrument_source, instrument, prepare=True)
        shutil.copytree(subject_root, contract_scratch / "Synthetic-source")
        spend(contract_scratch, source_commit, instrument)
        (contract_scratch / "Synthetic.json").write_text(json.dumps(
            {"tool": "wrong-oracle", "version": "PPI 1.270", "tier": "C"}))
        (contract_scratch / "phases.json").write_text(json.dumps(prefix[:3]))
        mark_contract_failure(contract_scratch, "ppi-primary", failure_env)
        contract_receipt = publish_failed_receipt(
            contract_scratch, instrument_source,
            instrument, "Synthetic", source_commit, failure_env)
        contract_data = json.loads(contract_receipt.read_text())
        need(contract_data["phase_outcomes"][-1]["failure"] == "contract_failed" and
             contract_data["phase_outcomes"][-1]["exit"] is None and
             contract_data["criteria"]["coverage_and_existing_gates_pass"] is False,
             "post-phase contract failure did not publish false receipt")
    return {"spent_before_read": "pass", "dependency_selection": "pass",
            "phase_identity": "pass", "timeout": "rejected",
            "failed_coverage_receipt": "publishable",
            "nonzero_grade_receipt": "published_false_gate",
            "prelaunch_failure_receipts": "published",
            "contract_failure_receipt": "published",
            "popen_signal_window": "child_stopped",
            "spend_signal_window": "published_false_gate",
            "collector_signal_window": "private_candidate_then_false_receipt",
            "superseded_receipt": "archived_then_replaced",
            "phase_write_interrupt": "valid_atomic_row",
            "frozen_copy_interrupt": "retry_from_committed_bytes",
            "retime_helper": "loaded_from_frozen_bytes"}


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
