#!/usr/bin/env python3
"""Run the aggregate-only COV-M1/M2 campaign.

Use Python 3.13 -I -B -S. Run preflight and run under the machine gate lease.
Each pool uses primary/<subject> and relocated/<subject> physical checkouts.
Preflight takes only a development directory and emits only its aggregates.
Run both commands from a dedicated clean detached Git checkout at the exact
harness revision. Use a physical campaign directory outside the checkout.

For run, ROOT is a dedicated physical campaign directory containing the
three cohorts and a private journal/ directory. The foreman supplies
journal/initial.authorization.json (0600). No CLI grants access.
The authorization contains:
  authority="COV-D2/ESCALATE-20-R7", operation, status="authorized", root,
  reader_revision (landed full Git SHA), harness_revision (reviewed HEAD),
  identity (exact output of 'identity', decoded as JSON),
  linux_ci={revision, status:"success", receipt},
  processed_acks={revision, receipt}, seal_review={revision, reviewer, receipt},
  baselines={original, closure, javascript_r5, typescript_r5}, each
    {path: absolute aggregate path, sha256: digest}.
This harness runs the initial operation only. The operation journal is
exclusive, including after failure. The independent post-run attestation
binds the completed journal and aggregate digest.
"""

import argparse
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import re
from runpy import run_path
import secrets
import signal
import stat
import subprocess
import sys
import tempfile
from datetime import datetime, timezone

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[2]
RUNNER = run_path(str(HERE / "cov_run.py"))
PYRIGHT = run_path(str(HERE / "cov_python_pyright.py"))
DIALECTS = ("python", "javascript", "typescript")
ARMS = ("primary", "relocated")
AUTHORITY = "COV-D2/ESCALATE-20-R7"
ORIGINAL_PYTHON_TRUTH = {"development": 156, "holdout": 896}
ORIGINAL_AGGREGATE = "d8d68af4735a00a6711b92247fb8bac7f3fb8b6e244567c95d0f61960523b58a"
ORIGINAL_HOLDOUT = {"python": (795, 2820), "javascript": (112, 317), "typescript": (723, 2077)}
CORRECTED_HOLDOUT = {"javascript": 112, "typescript": 501}
ORIGINAL_DEVELOPMENT = {"python": (121, 1314), "javascript": (153, 1508), "typescript": (250, 1224)}
CLOSURE_PARTITION = ("Complete", "OnlyUnmeasured", "OnlyMeasured", "MeasuredAndUnmeasured")
CLOSURE_FIELDS = ("Selected", *CLOSURE_PARTITION, "RejectedBlocked")
OWNED = (
    "pkg/lang/deps/cov_final_test.go",
    "pkg/lang/deps/cov_final_subject_test.go",
    "calibration/oracles/dep-coverage/cov_final.py",
    "calibration/oracles/dep-coverage/cov_final_test.py",
    "calibration/oracles/dep-coverage/cov_confirmation.json",
)
PINS = (
    "calibration/oracles/dep-coverage/cov_holdout.json",
    "calibration/oracles/dep-coverage/cov_confirmation.json",
    "calibration/oracles/deps-python/pins.json",
    "calibration/oracles/deps-tsjs/pins.json",
)
EXCLUSIONS = {
    "selection": "pinned tracked regular files only; no symlinks, gitlinks or untracked inputs",
    "javascript_confirmation": "no eligible unused subject",
    "value": "confirmation excluded from the original holdout denominators",
}


def canonical(value):
    return (RUNNER["canonical"](value) + "\n").encode()


def sha(data):
    return hashlib.sha256(data).hexdigest()


def git(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    return subprocess.check_output(
        ["git", "-c", "core.fsmonitor=false", "-c", "core.ignorestat=false",
         "-c", "core.trustctime=true", "-C", str(REPO), *args],
        env=env, stderr=subprocess.DEVNULL,
    )


def cohorts():
    return dict(RUNNER["subjects"](),
                confirmation=json.loads((HERE / "cov_confirmation.json").read_bytes()))


def subject(pool, name):
    for pin in cohorts().get(pool, ()):
        if pin["name"] == name:
            return pin
    raise ValueError("subject is outside the pinned cohort")


def operation_pools(operation):
    if operation == "initial":
        return ["development", "holdout", "confirmation"]
    raise ValueError("unknown final operation")


def physical(path):
    path = Path(path)
    if not path.is_absolute() or path.resolve(strict=True) != path:
        raise ValueError("expected absolute physical path")
    return path


def private(path, *, directory=False):
    path = physical(path)
    info = path.stat()
    check = stat.S_ISDIR if directory else stat.S_ISREG
    if not check(info.st_mode) or info.st_uid != os.getuid() or info.st_mode & 0o077:
        raise ValueError("journal storage must be private")
    return path


def claim(path, record):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "wb") as out:
        out.write(canonical(record))
        out.flush()
        os.fsync(out.fileno())
    sync_directory(path.parent)


def sync_directory(directory):
    fd = os.open(directory, os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def finish(path, record):
    pending = path.with_suffix(".finish")
    claim(pending, record)
    os.replace(pending, path)
    sync_directory(path.parent)


def now():
    return datetime.now(timezone.utc).isoformat()


def compare_python(names, original, extended, graph, source, roots, tracked):
    if len(names) != len(set(names)) or any([row["path"] for row in rows] != names
                                             for rows in (original, extended)):
        raise ValueError("CPython and Pyright selections differ")
    result = {}
    for old, new in zip(original, extended, strict=True):
        path = old["path"]
        if not old["gap"]:
            if old != new:
                raise ValueError("original CPython truth changed")
            row, cohort = old, "pre-existing"
        else:
            row, cohort = new, "new"
        if row["gap"]:
            result[path] = {"class": "native-gap", "truth": None, "cohort": cohort}
            continue
        if path not in graph:
            result[path] = {"class": "missing-graph", "truth": None, "cohort": cohort}
            continue
        kind, truth = PYRIGHT["compare_row"](row, graph[path], source(path), path, roots, tracked)
        result[path] = {"class": kind, "truth": sorted(truth) if truth is not None else None,
                        "cohort": cohort}
    return result


def active(config, pool):
    mode = config["mode"]
    if mode == "preflight":
        if pool != "development" or "journal" in config or "nonce" in config:
            raise ValueError("preflight is development-only")
        return
    if pool not in operation_pools(mode):
        raise ValueError("pool is outside the operation")
    root = physical(config["root"])
    journal = private(root / "journal", directory=True)
    expected = journal / (mode + ".json")
    if Path(config["journal"]) != expected:
        raise ValueError("journal outside campaign")
    record = json.loads(private(expected).read_bytes())
    tools = {key: config[key] for key in ("python", "compiler", "pyright")}
    if (record["status"] != "started" or record["nonce_sha256"] != sha(config["nonce"].encode()) or
            record["tool_paths"] != tools or
            record["binding"] != config["binding"] or record["root"] != str(root) or
            record["operation"] != mode or record["pools"] != operation_pools(mode)):
        raise ValueError("operation journal is not active")


def python_capture(config, pool, name, arm):
    active(config, pool)
    pin = subject(pool, name)
    if pin["dialect"] != "python" or arm not in ARMS:
        raise ValueError("wrong Python subject or relocation arm")
    base = Path(config["root"]) if config["mode"] == "preflight" else Path(config["root"]) / pool
    root = physical(base / arm / name)
    oracle = RUNNER["python_oracle"]()
    oracle.pin(declarative_fromlist=False)
    names, manifest = oracle.contents(root, pin, confined=True)
    selection = sha(b"".join(os.fsencode(name) + b"\0" for name in names))
    if pin.get("selectionSHA256", selection) != selection:
        raise ValueError("Python selection differs from pin")
    if oracle.git(root, "remote", "get-url", "origin").decode().strip() != pin["url"]:
        raise ValueError("Python origin differs from pin")
    original = RUNNER["python_rows"](oracle, root, pin, names, declarative_fromlist=False)
    if (names, manifest) != oracle.contents(root, pin, confined=True):
        raise ValueError("Python input changed during original collection")
    oracle.pin(declarative_fromlist=True)
    extended = RUNNER["python_rows"](oracle, root, pin, names, declarative_fromlist=True)
    # The reviewed static host requires a physical, regular-only checkout.
    # A subject outside that host's domain retains only its original truth.
    scratch = private(config["scratch"], directory=True)
    if scratch.parent != (REPO / "workspace/tmp").resolve():
        raise ValueError("native scratch is outside the campaign workspace")
    graph, tracked = {}, set()
    with tempfile.TemporaryDirectory(prefix="python-graph-", dir=scratch) as tmp:
        work = Path(tmp).resolve(strict=True)
        view = work / "00-view"
        if not manifest["symlinks"] and not manifest["gitlinks"]:
            staged_names, staged_manifest, tracked = PYRIGHT["stage"](vars(oracle), root, pin, view)
            if (staged_names, staged_manifest) != (names, manifest):
                raise ValueError("CPython and Pyright manifests differ")
            tool = PYRIGHT["verify_tool"](Path(config["pyright"]))
            try:
                graph = PYRIGHT["run_graph"](tool, view, pin["roots"], tracked, set(names), work, original=root)
            except (ValueError, subprocess.TimeoutExpired):
                # A graph unavailable from the pinned tool adds no truth.
                graph = {}
        compared = compare_python(names, original, extended, graph,
                                  lambda name: (view / name).read_bytes(), pin["roots"], tracked)
    if (names, manifest) != oracle.contents(root, pin, confined=True):
        raise ValueError("Python input changed during native collection")
    graph_digest = sha(canonical({path: sorted(edges) for path, edges in graph.items()}))
    return {"original": {"manifest": manifest, "rows": original},
            "extended": {"manifest": manifest, "rows": extended},
            "graph_sha256": graph_digest, "compared": compared}


def identity(compiler, pyright):
    compiler = physical(compiler)
    pyright = physical(pyright)
    cp = RUNNER["python_oracle"]().pin(declarative_fromlist=True)
    node, _ = PYRIGHT["verify_tool"](pyright)
    ts = json.loads((REPO / PINS[-1]).read_bytes())
    if sha(compiler.read_bytes()) != ts["compilerSHA256"] or ts["node"] != PYRIGHT["NODE_VERSION"]:
        raise ValueError("compiler identity differs")
    files = (*OWNED, *PINS,
             "calibration/oracles/dep-coverage/cov_run.py",
             "calibration/oracles/dep-coverage/cov_python_pyright.py",
             "calibration/oracles/deps-python/modulefinder_oracle.py",
             "calibration/oracles/deps-tsjs/trace.cjs")
    return {
        "pins": cohorts(),
        "exclusions": EXCLUSIONS,
        "source_sha256": {name: sha((REPO / name).read_bytes()) for name in files},
        "tools": {
            "cpython": {key: cp[key] for key in ("implementation", "version", "modulefinder_sha256")},
            "python_executable_sha256": sha(Path(sys.executable).read_bytes()),
            "node": ts["node"], "node_executable_sha256": sha(Path(node).read_bytes()),
            "typescript": ts["typescript"], "compiler_sha256": ts["compilerSHA256"],
            "pyright": {key.lower(): PYRIGHT[key] for key in
                        ("VERSION", "ARCHIVE_DIGEST", "ARCHIVE_SRI", "MANIFEST_DIGEST", "LAUNCH_DIGEST", "TOOL_FILES")},
        },
    }


def revision(value):
    return isinstance(value, str) and re.fullmatch("[0-9a-f]{40}", value)


def authorized(record, root, mode, pinned, head):
    reader = record["reader_revision"]
    if (record["authority"] != AUTHORITY or record["operation"] != mode
            or record["status"] != "authorized" or record["root"] != str(root)
            or record["identity"] != pinned or record["harness_revision"] != head
            or not revision(head) or not revision(reader)):
        raise ValueError("authorization does not bind this campaign")
    for name, rev in (("linux_ci", reader), ("processed_acks", reader), ("seal_review", head)):
        entry = record[name]
        if entry["revision"] != rev or not entry["receipt"]:
            raise ValueError("missing gate evidence")
    if record["linux_ci"]["status"] != "success" or not record["seal_review"]["reviewer"]:
        raise ValueError("gates are not green")


def ensure_reader(record):
    reader, head = record["reader_revision"], record["harness_revision"]
    if git("rev-parse", "HEAD").decode().strip() != head:
        raise ValueError("checkout HEAD differs from reviewed harness")
    if git("rev-parse", "--abbrev-ref", "HEAD").strip() != b"HEAD":
        raise ValueError("measurement requires a detached checkout")
    git("merge-base", "--is-ancestor", reader, head)
    if git("diff", "--name-only", reader, head, "--", ".", *[":(exclude)" + p for p in OWNED]):
        raise ValueError("reviewed checkout does not contain the exact landed reader")
    # Git normally ignores some untracked files, including files a compiler
    # could consume. Allow scratch only in the designated workspace.
    if (any(not entry.startswith(b"H ") for entry in git("ls-files", "-v", "-z").split(b"\0") if entry)
            or git("status", "--porcelain", "--untracked-files=no", "--ignore-submodules=none")
            or git("ls-files", "--others", "--exclude=/workspace/tmp/", "-z")):
        raise ValueError("reviewed checkout is dirty")


def read_aggregate(ref):
    data = physical(ref["path"]).read_bytes()
    if sha(data) != ref["sha256"]:
        raise ValueError("baseline digest differs")
    return json.loads(data)


def validate_population(row, files):
    score, closure = row["score"], row["closure"]
    if (score["Files"] != files or closure["Selected"] != files or not 0 <= score["Complete"] <= files
            or not 0 <= closure["Complete"] <= score["Complete"]):
        raise ValueError("campaign population differs from the preregistration")


def sound(row):
    score = row["score"]
    return all(score[key] for key in ("B1", "B2", "B3")) and score["CompleteWithoutOracle"] == 0


def original_metrics(score, closure, rejected, denominator, complete, *, sealed=False):
    c_key, f_key = ("complete", "files") if sealed else ("Complete", "Files")
    if (score[c_key], score[f_key]) != (complete, denominator):
        raise ValueError("original baseline differs from the preregistration")
    if not isinstance(closure, dict) or closure.keys() != set(CLOSURE_FIELDS):
        raise ValueError("baseline closure fields differ")
    counts = {key: closure[key] for key in CLOSURE_FIELDS}
    if any(type(count) is not int or count < 0 for count in counts.values()):
        raise ValueError("baseline closure requires nonnegative integer counts")
    if (counts["Selected"] != denominator or sum(counts[key] for key in CLOSURE_PARTITION) != denominator
            or counts["Complete"] > complete or counts["RejectedBlocked"] > denominator - counts["Complete"]):
        raise ValueError("baseline closure counts do not reconcile")
    return {"score": score, "closure": counts, "rejected_inputs": rejected}


def baselines(refs):
    if refs["original"]["sha256"] != ORIGINAL_AGGREGATE:
        raise ValueError("original aggregate differs from the reproduced baseline")
    original = read_aggregate(refs["original"])
    closure = read_aggregate(refs["closure"])
    if not isinstance(closure, dict) or closure.keys() != set(DIALECTS):
        raise ValueError("baseline closure dialects differ")
    before = {"development": {}, "holdout": {}}
    for dialect in DIALECTS:
        complete, files = ORIGINAL_HOLDOUT[dialect]
        dev_complete, dev_files = ORIGINAL_DEVELOPMENT[dialect]
        dev = original["development"][dialect]
        score = dev["Analysis"]["Score"]
        before["development"][dialect] = original_metrics(
            score, dev["Closure"], score["RejectedInputs"], dev_files, dev_complete)
        before["holdout"][dialect] = original_metrics(
            original["holdout"][dialect], closure[dialect], original["holdout_rejected_inputs"][dialect],
            files, complete, sealed=True)
    corrected = {dialect: read_aggregate(refs[dialect + "_r5"]) for dialect in CORRECTED_HOLDOUT}
    for dialect, table in corrected.items():
        if ((table["complete"], table["files"]) != (CORRECTED_HOLDOUT[dialect], ORIGINAL_HOLDOUT[dialect][1])
                or not all(table[key] for key in ("b1", "b2", "b3"))):
            raise ValueError("R5 baseline differs from the closed correction")
    return before, corrected


def validate_python_overlap(row, pool):
    originally_graded = sum(n for kind, n in row["native_classes"].items() if kind.startswith("original/"))
    if originally_graded != ORIGINAL_PYTHON_TRUTH[pool]:
        raise ValueError("original CPython coverage drifted from the frozen baseline")


def assess(result, baseline):
    before, corrected = baseline
    output = {"schema": "cov-final/v1", "current": result, "original": before,
              "secondary_r5": {}, "verdicts": {}}
    confirmation = result["confirmation"]["typescript"]
    validate_population(confirmation, 979)
    output["confirmation_soundness"] = sound(confirmation)
    doubled_and_sound = 0
    for pool in ("development", "holdout"):
        validate_python_overlap(result[pool]["python"], pool)
    for dialect in DIALECTS:
        complete, files = ORIGINAL_HOLDOUT[dialect]
        dev, held = result["development"][dialect], result["holdout"][dialect]
        validate_population(dev, ORIGINAL_DEVELOPMENT[dialect][1])
        validate_population(held, files)
        count = held["score"]["Complete"]
        if dialect in corrected:
            r5 = corrected[dialect]
            output["secondary_r5"][dialect] = {"complete": r5["complete"], "files": files,
                                                 "ratio": count / r5["complete"] if r5["complete"] else None}
        passes = sound(dev) and sound(held)
        value = count >= 2 * complete
        doubled_and_sound += passes and value
        if passes:
            label = "sound" if count * 5 >= files * 4 else "sound, low coverage, improved"
        else:
            label = "soundness failed; correction or revert required"
        output["verdicts"][dialect] = {
            "soundness": passes, "doubled": value, "label": label, "complete": count, "files": files,
            "original_ratio": count / complete,
        }
    output["program"] = {"dialects_doubled_with_soundness": doubled_and_sound, "met": doubled_and_sound >= 2}
    return output


@contextmanager
def termination():
    pending = None

    def request(signum, _frame):
        nonlocal pending
        pending = signum

    def check():
        if pending is not None:
            raise InterruptedError(f"campaign interrupted by signal {pending}")

    # Defer delivery across process creation and journal writes so neither
    # process ownership nor the exclusive receipt can be lost on SIGTERM.
    signals = (signal.SIGTERM, signal.SIGINT)
    handlers = {sig: signal.signal(sig, request) for sig in signals}
    try:
        yield check
    finally:
        for sig, handler in handlers.items():
            signal.signal(sig, handler)


def run_group(command, *, cwd, env, check, data=None):
    check()
    with subprocess.Popen(command, cwd=cwd, env=env, start_new_session=True,
                          stdin=subprocess.PIPE if data is not None else subprocess.DEVNULL,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE) as process:
        try:
            while True:
                check()
                try:
                    stdout, stderr = process.communicate(data, timeout=0.2)
                    break
                except subprocess.TimeoutExpired as error:
                    # A failed leader can leave descendants holding its pipes.
                    if process.poll():
                        raise subprocess.CalledProcessError(process.returncode, command,
                                                            error.output, error.stderr) from None
                    data = None  # communicate retains pending input across retries.
            check()
            if process.returncode:
                raise subprocess.CalledProcessError(process.returncode, command, stdout, stderr)
            return stdout
        except BaseException:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait()
            raise


def measure(config, check):
    workspace = REPO / "workspace/tmp"
    workspace.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="cov-final-", dir=workspace) as tmp:
        scratch = Path(tmp).resolve(strict=True)
        config = dict(config, scratch=str(scratch))
        environment = {k: v for k, v in os.environ.items()
                       if k in ("GOCACHE", "GOMODCACHE") or
                       not k.startswith(("FATHOM_COV_", "NODE_", "PYTHON", "GIT_", "GO", "CGO_"))}
        environment.update(TMPDIR=str(scratch), GOTMPDIR=str(scratch),
                           GOENV="off", GOWORK="off", GOTOOLCHAIN="local",
                           GOPROXY="off", GOSUMDB="off", CGO_ENABLED="1",
                           GIT_CONFIG_COUNT="2", GIT_CONFIG_KEY_0="core.hooksPath", GIT_CONFIG_VALUE_0="/dev/null",
                           GIT_CONFIG_KEY_1="core.fsmonitor", GIT_CONFIG_VALUE_1="false",
                           GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0")
        binary = scratch / "cov-final.test"
        run_group(["go", "test", "-race", "-c", "-o", str(binary), "./pkg/lang/deps"],
                  cwd=REPO, env=environment, check=check)
        environment["FATHOM_COV_FINAL_WORKER"] = "1"
        output = run_group([binary, "-test.run=^TestCOVFinalMeasure$", "-test.timeout=45m"],
                           cwd=REPO / "pkg/lang/deps", env=environment, check=check, data=canonical(config))
        aggregate, marker = output.rsplit(b"\n", 2)[:2]
        if marker != b"PASS":
            raise ValueError("final worker did not complete")
        return json.loads(aggregate)


def run(args):
    pools = operation_pools(args.operation)
    root = physical(args.root)
    pinned = identity(args.compiler, args.pyright)
    tool_paths = {"python": str(Path(sys.executable).resolve()), "compiler": str(args.compiler),
                  "pyright": str(args.pyright)}
    config = dict(tool_paths, root=str(root), mode=args.operation)
    journal = private(root / "journal", directory=True)
    auth_path = private(journal / (args.operation + ".authorization.json"))
    auth_data = auth_path.read_bytes()
    auth = json.loads(auth_data)
    head = git("rev-parse", "HEAD").decode().strip()
    authorized(auth, root, args.operation, pinned, head)
    ensure_reader(auth)
    baseline = baselines(auth["baselines"])
    path = journal / (args.operation + ".json")
    binding = sha(auth_data)
    nonce = secrets.token_hex(32)
    record = dict(auth, status="started", started_at=now(), pools=pools,
                  binding=binding, nonce_sha256=sha(nonce.encode()), tool_paths=tool_paths)
    config.update(journal=str(path), binding=binding, nonce=nonce)
    with termination() as check:
        check()
        claim(path, record)
        status = "failed"
        try:
            ensure_reader(auth)
            measured = measure(config, check)
            aggregate = assess(measured, baseline)
            ensure_reader(auth)
            check()
            claim(journal / "initial.aggregate.json", aggregate)
            record["aggregate_sha256"] = sha(canonical(aggregate))
            status = "completed"
        finally:
            record.update(status=status, finished_at=now())
            finish(path, record)
    print(record["aggregate_sha256"])


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    commands = parser.add_subparsers(dest="command", required=True)
    for command in ("preflight", "identity", "run"):
        sub = commands.add_parser(command)
        sub.add_argument("--compiler", required=True, type=Path)
        sub.add_argument("--pyright", required=True, type=Path)
        if command != "identity":
            sub.add_argument("root", type=Path)
        if command == "run":
            sub.add_argument("operation", choices=("initial",))
    native = commands.add_parser("_python")
    native.add_argument("pool")
    native.add_argument("subject")
    native.add_argument("arm")
    check = commands.add_parser("_active")
    check.add_argument("pool")
    args = parser.parse_args()
    if args.command == "_active":
        active(json.load(sys.stdin), args.pool)
    elif args.command == "_python":
        if not all(stat.S_ISFIFO(os.fstat(pipe.fileno()).st_mode) for pipe in (sys.stdin, sys.stdout)):
            raise ValueError("native capture requires private instrument pipes")
        sys.stdout.buffer.write(canonical(python_capture(json.load(sys.stdin), args.pool, args.subject, args.arm)))
    elif args.command == "identity":
        sys.stdout.buffer.write(canonical(identity(args.compiler, args.pyright)))
    elif args.command == "preflight":
        root = physical(args.root)
        identity(args.compiler, args.pyright)
        config = {"root": str(root), "mode": "preflight", "python": str(Path(sys.executable).resolve()),
                  "compiler": str(args.compiler), "pyright": str(args.pyright)}
        head = git("rev-parse", "HEAD").decode().strip()
        expected = dict(reader_revision=head, harness_revision=head)
        with termination() as check:
            ensure_reader(expected)
            rows = measure(config, check)["development"]
            ensure_reader(expected)
            check()
        validate_python_overlap(rows["python"], "development")
        for dialect, (_, files) in ORIGINAL_DEVELOPMENT.items():
            validate_population(rows[dialect], files)
            if not sound(rows[dialect]):
                raise ValueError("development soundness failed")
        sys.stdout.buffer.write(canonical({"schema": "cov-final-preflight/v1", "development": rows}))
    else:
        run(args)


if __name__ == "__main__":
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        # Development diagnostics are useful during preflight and never
        # receive a held cohort. All final failures are aggregate-only.
        if sys.argv[1:2] == ["preflight"]:
            output = getattr(error, "stdout", None) or getattr(error, "stderr", None)
            if output:
                sys.stderr.buffer.write(output)
            print(f"COV development preflight: {error}", file=sys.stderr)
        else:
            print("COV final operation failed; no private diagnostics emitted", file=sys.stderr)
        raise SystemExit(1)
