#!/usr/bin/env python3
"""Run the aggregate-only COV-M1/M2 campaign.

Use Python 3.13 -I -B -S. Take the machine gate lease for measurements.
Each pool uses primary/<subject> and relocated/<subject> physical checkouts.
Preflight takes only a development directory and emits only its aggregates.
Use a dedicated clean detached Git checkout at the exact harness revision.

The test-only rehearse command uses development pins in all three positions.
Its --inject-native-failure flag substitutes a synthetic nonzero exit in
the simulated holdout JavaScript relocation arm.
It takes a fresh workspace/tmp/cov-final-rehearsal/run-<id> directory with
its own private journal/ and publishes a rehearsal-only aggregate.

For run, ROOT is a dedicated physical campaign directory outside the checkout
containing the three cohorts and a private journal/ directory. The foreman
supplies journal/<operation>.authorization.json (0600). No CLI grants access.
The authorization contains:
  authority, operation, status="authorized", root,
  reader_revision (landed full Git SHA), harness_revision (reviewed HEAD),
  identity (exact output of 'identity', decoded as JSON),
  linux_ci={revision, status:"success", receipt},
  processed_acks={revision, receipt}, seal_review={revision, reviewer, receipt},
  baselines={original, closure, javascript_r5, typescript_r5}, each
    {path: absolute aggregate path, sha256: digest}.
Use authority="COV-D2/ESCALATE-20-R7" for initial. For replacement, use
authority="COV-D2/ESCALATE-23" and supply failed_initial and metadata_preflight
as {path, sha256} references. The first binds the preserved failed initial
journal in a different campaign. The second binds journal/metadata-preflight.txt
with one "<check-name> PASS" per line. Rehearsal and replacement retain the
Go worker's stderr in journal/<operation>.stderr (0600). For final, use
authority="COV-D2/ESCALATE-24" and also supply failed_replacement as a
{path, sha256} reference to the preserved replacement journal. Final uses
its own journal and private stderr receipt.
The operation journal is exclusive, including after failure. The independent
post-run attestation binds the completed journal and aggregate digest.
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
REPLACEMENT_AUTHORITY = "COV-D2/ESCALATE-23"
FINAL_AUTHORITY = "COV-D2/ESCALATE-24"
PRODUCTION_OPERATIONS = ("initial", "replacement", "final")
ISOLATION_MODES = ("rehearse", "final")
NATIVE_NONZERO_STATUS = 73
NATIVE_EXCEPTIONS = {"python": "NativeNonzero", "javascript": "ExitError", "typescript": "ExitError"}
FAILED_INITIAL_HARNESS = "598ec9ded011ded5b93fa04805c07c168226c21f"
FAILED_INITIAL_SHA256 = "91b36b4a463cfe1fde13b519abc3f8f9cbca5cc3ef02ea77c1c423dce59f8bf7"
FAILED_REPLACEMENT_HARNESS = "b67d4fe3e4cc9d324c9cca3c19b699b3c7500132"
FAILED_REPLACEMENT_SHA256 = "7e191d908b378c0db3e53ae1622c868c02f9e4493679ddb7b71577166039c5be"
FINAL_READER = "59ef2b56bf979f5f4ed91ea7182338f30a63c9b9"
REHEARSAL_CONFIRMATION = "fp-ts"
REHEARSAL_CONFIRMATION_FILES = 204
POOL_CLASSES = ("development", "holdout", "confirmation")
WORKER_STAGES = ("pool", "freeze", "native", "read", "refreeze", "score")
EXCEPTION_CLASSES = (ValueError, KeyError, TypeError, OSError, FileNotFoundError,
                     PermissionError, FileExistsError, InterruptedError, KeyboardInterrupt,
                     subprocess.CalledProcessError, subprocess.TimeoutExpired, json.JSONDecodeError)
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
SHARED_HARNESS = (
    "pkg/lang/deps/cov_measure_test.go",
    "calibration/oracles/dep-coverage/cov_python_pyright.py",
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
        ["git", "--no-replace-objects", f"--work-tree={REPO}",
         "-c", "core.fsmonitor=false", "-c", "core.ignorestat=false",
         "-c", "core.trustctime=true", "-C", str(REPO), *args],
        env=env, stderr=subprocess.DEVNULL,
    )


def cohorts():
    return dict(RUNNER["subjects"](),
                confirmation=json.loads((HERE / "cov_confirmation.json").read_bytes()))


def rehearsal_subjects(pool):
    if pool not in POOL_CLASSES:
        raise ValueError("pool is outside the rehearsal")
    pins = RUNNER["python_subjects"]() + json.loads((REPO / PINS[-1]).read_bytes())["subjects"]
    if pool == "confirmation":
        pins = [pin for pin in pins if pin["name"] == REHEARSAL_CONFIRMATION
                and pin["dialect"] == "typescript" and pin["files"] == REHEARSAL_CONFIRMATION_FILES]
        if len(pins) != 1:
            raise ValueError("rehearsal confirmation pin unavailable")
    return pins


def subject(pool, name, mode="initial"):
    pins = rehearsal_subjects(pool) if mode == "rehearse" else cohorts().get(pool, ())
    for pin in pins:
        if pin["name"] == name:
            return pin
    raise ValueError("subject is outside the pinned cohort")


def operation_pools(operation):
    if operation in PRODUCTION_OPERATIONS:
        return list(POOL_CLASSES)
    raise ValueError("unknown final operation")


def campaign_pools(mode):
    return list(POOL_CLASSES) if mode == "rehearse" else operation_pools(mode)


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


def finish(path, record, suffix=".finish"):
    pending = path.with_suffix(suffix)
    claim(pending, record)
    os.replace(pending, path)
    sync_directory(path.parent)


def now():
    return datetime.now(timezone.utc).isoformat()


def rehearsal_root(path):
    path = Path(path)
    parent = REPO / "workspace/tmp/cov-final-rehearsal"
    if path.parent != parent or not re.fullmatch(r"run-[a-z0-9]+", path.name):
        raise ValueError("root is outside the rehearsal workspace")
    for directory in (parent.parent.parent, parent.parent, parent, path, path / "journal"):
        if not stat.S_ISDIR(directory.lstat().st_mode):
            raise ValueError("rehearsal requires physical directories")
    return private(path, directory=True)


def validate_python_rows(names, original, extended):
    if len(names) != len(set(names)) or any([row["path"] for row in rows] != names
                                             for rows in (original, extended)):
        raise ValueError("CPython and Pyright selections differ")
    if any(not old["gap"] and old != new for old, new in zip(original, extended, strict=True)):
        raise ValueError("original CPython truth changed")


def compare_python(names, original, extended, graph, source, roots, tracked):
    validate_python_rows(names, original, extended)
    result = {}
    for old, new in zip(original, extended, strict=True):
        path = old["path"]
        if not old["gap"]:
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
    inject = config.get("inject_native_failure", False)
    if type(inject) is not bool or (inject and mode != "rehearse"):
        raise ValueError("native failure injection requires rehearsal")
    if mode == "preflight":
        if pool != "development" or "journal" in config or "nonce" in config:
            raise ValueError("preflight is development-only")
        return
    if pool not in campaign_pools(mode):
        raise ValueError("pool is outside the operation")
    root = rehearsal_root(config["root"]) if mode == "rehearse" else physical(config["root"])
    journal = private(root / "journal", directory=True)
    expected = journal / (mode + ".json")
    if Path(config["journal"]) != expected:
        raise ValueError("journal outside campaign")
    if mode == "rehearse" and not stat.S_ISREG(expected.lstat().st_mode):
        raise ValueError("rehearsal requires a regular journal")
    record = json.loads(private(expected).read_bytes())
    tools = {key: config[key] for key in ("python", "compiler", "pyright")}
    if (record["status"] != "started" or record["nonce_sha256"] != sha(config["nonce"].encode()) or
            record["tool_paths"] != tools or
            record["binding"] != config["binding"] or record["root"] != str(root) or
            record["operation"] != mode or record["pools"] != campaign_pools(mode) or
            record.get("inject_native_failure", False) is not inject):
        raise ValueError("operation journal is not active")
    return record


def progress(config, stage, pool=None):
    if config["mode"] == "preflight":
        return
    if pool is None:
        if stage not in ("checkout", "compile", "worker", "publication"):
            raise ValueError("unknown campaign stage")
        event = {"stage": stage}
    else:
        if pool not in POOL_CLASSES or stage not in (*WORKER_STAGES, "assessment"):
            raise ValueError("unknown pool transition")
        event = {"stage": stage, "pool": pool}
    record = active(config, pool or "development")
    record["progress"].append(event)
    # The parent waits for the worker, and the worker waits for each
    # callback. Each transition is durable before its stage starts.
    finish(Path(config["journal"]), record, ".progress")


def python_capture(config, pool, name, arm):
    active(config, pool)
    pin = subject(pool, name, config["mode"])
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
    isolate = config["mode"] in ISOLATION_MODES
    if isolate:
        validate_python_rows(names, original, extended)
    # The reviewed static host requires a physical, regular-only checkout.
    # A subject outside that host's domain retains only its original truth.
    scratch = private(config["scratch"], directory=True)
    if scratch.parent != (REPO / "workspace/tmp").resolve():
        raise ValueError("native scratch is outside the campaign workspace")
    graph, tracked = {}, set()
    native_exit = False
    with tempfile.TemporaryDirectory(prefix="python-graph-", dir=scratch) as tmp:
        work = Path(tmp).resolve(strict=True)
        view = work / "00-view"
        if not manifest["symlinks"] and not manifest["gitlinks"]:
            staged_names, staged_manifest, tracked = PYRIGHT["stage"](vars(oracle), root, pin, view)
            if (staged_names, staged_manifest) != (names, manifest):
                raise ValueError("CPython and Pyright manifests differ")
            tool = PYRIGHT["verify_tool"](Path(config["pyright"]))
            try:
                graph = PYRIGHT["run_graph"](tool, view, pin["roots"], tracked, set(names), work,
                                             original=root, classify_exit=isolate)
            except PYRIGHT["NativeNonzero"]:
                native_exit = True
            except (ValueError, subprocess.TimeoutExpired):
                if isolate:
                    raise
                # A graph unavailable from the pinned tool adds no truth.
                graph = {}
        if not native_exit:
            compared = compare_python(names, original, extended, graph,
                                      lambda name: (view / name).read_bytes(), pin["roots"], tracked)
    if (names, manifest) != oracle.contents(root, pin, confined=True):
        raise ValueError("Python input changed during native collection")
    if native_exit:
        raise PYRIGHT["NativeNonzero"]("native truth tool exited nonzero")
    graph_digest = sha(canonical({path: sorted(edges) for path, edges in graph.items()}))
    return {"original": {"manifest": manifest, "rows": original},
            "extended": {"manifest": manifest, "rows": extended},
            "graph_sha256": graph_digest, "compared": compared}


def identity(compiler, pyright, mode="initial"):
    compiler = physical(compiler)
    pyright = physical(pyright)
    cp = RUNNER["python_oracle"]().pin(declarative_fromlist=True)
    node, _ = PYRIGHT["verify_tool"](pyright)
    ts = json.loads((REPO / PINS[-1]).read_bytes())
    if sha(compiler.read_bytes()) != ts["compilerSHA256"] or ts["node"] != PYRIGHT["NODE_VERSION"]:
        raise ValueError("compiler identity differs")
    files = (*OWNED, *SHARED_HARNESS, *PINS,
             "calibration/oracles/dep-coverage/cov_run.py",
             "calibration/oracles/deps-python/modulefinder_oracle.py",
             "calibration/oracles/deps-tsjs/trace.cjs")
    pins = cohorts() if mode != "rehearse" else {"development": rehearsal_subjects("development")}
    if mode == "rehearse":
        files = tuple(name for name in files if name not in PINS[:2])
    return {
        "pins": pins,
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
    authority = {"initial": AUTHORITY, "replacement": REPLACEMENT_AUTHORITY, "final": FINAL_AUTHORITY}[mode]
    if (record["authority"] != authority or record["operation"] != mode
            or record["status"] != "authorized" or record["root"] != str(root)
            or record["identity"] != pinned or record["harness_revision"] != head
            or "inject_native_failure" in record or not revision(head) or not revision(reader)):
        raise ValueError("authorization does not bind this campaign")
    for name, rev in (("linux_ci", reader), ("processed_acks", reader), ("seal_review", head)):
        entry = record[name]
        if entry["revision"] != rev or not entry["receipt"]:
            raise ValueError("missing gate evidence")
    if record["linux_ci"]["status"] != "success" or not record["seal_review"]["reviewer"]:
        raise ValueError("gates are not green")

    if mode in ("replacement", "final"):
        if reader != FINAL_READER:
            raise ValueError("replacement requires the exact landed reader")
        replacement_evidence(record, root)
    if mode == "final":
        final_evidence(record, root)


def evidence_ref(record, field):
    ref = record.get(field)
    if (not isinstance(ref, dict) or ref.keys() != {"path", "sha256"}
            or not isinstance(ref["path"], str) or not isinstance(ref["sha256"], str)
            or not re.fullmatch("[0-9a-f]{64}", ref["sha256"])):
        raise ValueError("invalid replacement evidence reference")
    return Path(ref["path"]), ref["sha256"]


def replacement_evidence(record, root):
    failed, digest = evidence_ref(record, "failed_initial")
    if digest != FAILED_INITIAL_SHA256 or failed.name != "initial.json" or failed.parent.name != "journal":
        raise ValueError("replacement requires the preserved initial journal")
    private(failed.parent, directory=True)
    original_root = failed.parent.parent
    if root.samefile(original_root):
        raise ValueError("replacement requires a different campaign root")
    data = private(failed).read_bytes()
    if sha(data) != digest:
        raise ValueError("failed initial journal digest differs")
    original = json.loads(data)
    if (original["operation"] != "initial" or original["status"] != "failed"
            or original["reader_revision"] != FINAL_READER
            or original["harness_revision"] != FAILED_INITIAL_HARNESS
            or original["root"] != str(original_root) or "aggregate_sha256" in original):
        raise ValueError("failed initial journal does not bind the original campaign")
    try:
        failed.with_name("initial.aggregate.json").lstat()
    except FileNotFoundError:
        pass
    else:
        raise ValueError("initial aggregate already exists")

    preflight, digest = evidence_ref(record, "metadata_preflight")
    if preflight != root / "journal/metadata-preflight.txt":
        raise ValueError("metadata preflight receipt is outside its journal location")
    data = private(preflight).read_bytes()
    if sha(data) != digest:
        raise ValueError("metadata preflight receipt digest differs")
    checks = data.splitlines()
    if not checks or any(not re.fullmatch(rb"[a-zA-Z0-9][a-zA-Z0-9_.-]* PASS", line) for line in checks):
        raise ValueError("metadata preflight checks must all pass")


def final_evidence(record, root):
    failed, digest = evidence_ref(record, "failed_replacement")
    if (digest != FAILED_REPLACEMENT_SHA256 or failed.name != "replacement.json"
            or failed.parent.name != "journal"):
        raise ValueError("final requires the preserved replacement journal")
    private(failed.parent, directory=True)
    previous_root = failed.parent.parent
    if root.samefile(previous_root):
        raise ValueError("final requires a different campaign root")
    data = private(failed).read_bytes()
    if sha(data) != digest:
        raise ValueError("failed replacement journal digest differs")
    previous = json.loads(data)
    if (previous["operation"] != "replacement" or previous["authority"] != REPLACEMENT_AUTHORITY
            or previous["status"] != "failed" or previous["reader_revision"] != FINAL_READER
            or previous["harness_revision"] != FAILED_REPLACEMENT_HARNESS
            or previous["root"] != str(previous_root) or "aggregate_sha256" in previous
            or previous["failed_initial"] != record["failed_initial"]
            or previous["failure"] != {"stage": "native", "pool": "holdout", "exception": "CalledProcessError"}):
        raise ValueError("failed replacement journal does not bind the previous campaign")
    try:
        require_absent(failed.with_name("replacement.aggregate.json"))
    except FileExistsError:
        raise ValueError("replacement aggregate already exists") from None


def require_absent(path):
    try:
        path.lstat()
    except FileNotFoundError:
        return
    raise FileExistsError("campaign artifact already exists")


def ensure_checkout_bytes(head):
    # The compiler reads the worktree bytes. Git status can compare bytes
    # after a configured clean filter, so check the raw blob identity too.
    checked_directories = set()
    for entry in filter(None, git("ls-tree", "-r", "-z", head).split(b"\0")):
        metadata, name = entry.split(b"\t", 1)
        mode, kind, expected = metadata.split()
        file = REPO / os.fsdecode(name)
        if file.parent not in checked_directories:
            if file.parent.resolve(strict=True) != file.parent:
                raise ValueError("reviewed raw checkout bytes cannot be confined")
            checked_directories.add(file.parent)
        actual_mode = file.lstat().st_mode
        if kind == b"blob" and mode == b"120000" and stat.S_ISLNK(actual_mode):
            content = os.readlink(os.fsencode(file))
        elif kind == b"blob" and mode in (b"100644", b"100755") and stat.S_ISREG(actual_mode):
            if bool(actual_mode & stat.S_IXUSR) != (mode == b"100755"):
                raise ValueError("reviewed raw checkout bytes differ")
            content = file.read_bytes()
        else:
            raise ValueError("reviewed raw checkout bytes differ")
        blob = hashlib.sha1(f"blob {len(content)}\0".encode(), usedforsecurity=False)
        blob.update(content)
        if blob.hexdigest().encode() != expected:
            raise ValueError("reviewed raw checkout bytes differ")


def ensure_reader(record):
    reader, head = record["reader_revision"], record["harness_revision"]
    if git("rev-parse", "HEAD").decode().strip() != head:
        raise ValueError("checkout HEAD differs from reviewed harness")
    if git("rev-parse", "--abbrev-ref", "HEAD").strip() != b"HEAD":
        raise ValueError("measurement requires a detached checkout")
    git("merge-base", "--is-ancestor", reader, head)
    allowed = (*OWNED, *SHARED_HARNESS)
    if git("diff", "--name-only", reader, head, "--", ".", *[":(exclude)" + p for p in allowed]):
        raise ValueError("reviewed checkout does not contain the exact landed reader")
    # Git normally ignores some untracked files, including files a compiler
    # could consume. Allow scratch only in the designated workspace.
    if (any(not entry.startswith(b"H ") for entry in git("ls-files", "-v", "-z").split(b"\0") if entry)
            or git("status", "--porcelain", "--untracked-files=no", "--ignore-submodules=none")
            or git("ls-files", "--others", "--exclude=/workspace/tmp/", "-z")):
        raise ValueError("reviewed checkout is dirty")
    ensure_checkout_bytes(head)


def read_aggregate(ref):
    data = physical(ref["path"]).read_bytes()
    if sha(data) != ref["sha256"]:
        raise ValueError("baseline digest differs")
    return json.loads(data)


def validate_population(row, files, *, native_exception=None):
    if "native_exception" in row:
        if (native_exception is None or row.keys() != {"native_exception", "files", "reader_complete_ungraded"}
                or row["native_exception"] != native_exception
                or type(row["files"]) is not int or row["files"] != files
                or type(row["reader_complete_ungraded"]) is not int
                or not 0 <= row["reader_complete_ungraded"] <= files):
            raise ValueError("invalid ungraded campaign population")
        return
    score, closure = row["score"], row["closure"]
    if (score["Files"] != files or closure["Selected"] != files or not 0 <= score["Complete"] <= files
            or not 0 <= closure["Complete"] <= score["Complete"]):
        raise ValueError("campaign population differs from the preregistration")


def sound(row):
    if "native_exception" in row:
        return False
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


def validate_python_overlap(row, pool, mode="initial"):
    originally_graded = sum(n for kind, n in row["native_classes"].items() if kind.startswith("original/"))
    expected = ORIGINAL_PYTHON_TRUTH["development" if mode == "rehearse" else pool]
    if originally_graded != expected:
        raise ValueError("original CPython coverage drifted from the frozen baseline")


def rehearsal_baselines():
    # Use registered development counts and a synthetic closure partition.
    # The rehearsal neither loads nor reconstructs a production artifact.
    before = {"development": {}, "holdout": {}}
    for dialect, (complete, files) in ORIGINAL_DEVELOPMENT.items():
        score = dict(Complete=complete, Files=files, RejectedInputs=0)
        closure = dict.fromkeys(CLOSURE_FIELDS, 0)
        closure.update(Selected=files, OnlyUnmeasured=files)
        before["development"][dialect] = original_metrics(score, closure, 0, files, complete)
        before["holdout"][dialect] = original_metrics(
            dict(complete=complete, files=files), closure, 0, files, complete, sealed=True)
    corrected = {dialect: dict(complete=ORIGINAL_DEVELOPMENT[dialect][0])
                 for dialect in ("javascript", "typescript")}
    return before, corrected


def assess(result, baseline, *, mode="initial", report=lambda _stage, _pool: None):
    before, corrected = baseline
    rehearsal = mode == "rehearse"
    isolate = mode in ISOLATION_MODES
    exceptions = NATIVE_EXCEPTIONS if isolate else {}
    original_holdout = ORIGINAL_DEVELOPMENT if rehearsal else ORIGINAL_HOLDOUT
    output = {"schema": "cov-final-rehearsal/v1" if rehearsal else "cov-final/v1",
              "current": result, "original": before,
              "secondary_r5": {}, "verdicts": {}}
    report("assessment", "confirmation")
    confirmation = result["confirmation"]["typescript"]
    validate_population(confirmation, REHEARSAL_CONFIRMATION_FILES if rehearsal else 979,
                        native_exception=exceptions.get("typescript"))
    output["confirmation_soundness"] = sound(confirmation)
    doubled_and_sound = 0
    for pool in ("development", "holdout"):
        report("assessment", pool)
        python = result[pool]["python"]
        if not isolate or "native_exception" not in python:
            validate_python_overlap(python, pool, mode)
    for dialect in DIALECTS:
        complete, files = original_holdout[dialect]
        report("assessment", "development")
        dev = result["development"][dialect]
        validate_population(dev, ORIGINAL_DEVELOPMENT[dialect][1], native_exception=exceptions.get(dialect))
        report("assessment", "holdout")
        held = result["holdout"][dialect]
        validate_population(held, files, native_exception=exceptions.get(dialect))
        if "native_exception" in held:
            output["verdicts"][dialect] = dict(
                held, soundness=False, doubled=False, label="unverified, native truth unavailable")
            continue
        count = held["score"]["Complete"]
        if dialect in corrected:
            r5 = corrected[dialect]
            output["secondary_r5"][dialect] = {"complete": r5["complete"], "files": files,
                                                 "ratio": count / r5["complete"] if r5["complete"] else None}
        passes = sound(dev) and sound(held)
        value = "native_exception" not in dev and count >= 2 * complete
        doubled_and_sound += passes and value
        if "native_exception" in dev:
            label = "unverified, native truth unavailable"
        elif passes:
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


def run_group(command, *, cwd, env, check, data=None, stderr_target=subprocess.PIPE):
    check()
    with subprocess.Popen(command, cwd=cwd, env=env, start_new_session=True,
                          stdin=subprocess.PIPE if data is not None else subprocess.DEVNULL,
                          stdout=subprocess.PIPE, stderr=stderr_target) as process:
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


@contextmanager
def worker_stderr(config):
    if config["mode"] not in ("rehearse", "replacement", "final"):
        yield subprocess.PIPE
        return
    path = Path(config["journal"]).with_suffix(".stderr")
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "wb") as receipt:
        os.fchmod(receipt.fileno(), 0o600)
        sync_directory(path.parent)
        try:
            yield receipt
        finally:
            os.fsync(receipt.fileno())


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
        progress(config, "compile")
        run_group(["go", "test", "-race", "-c", "-o", str(binary), "./pkg/lang/deps"],
                  cwd=REPO, env=environment, check=check)
        environment["FATHOM_COV_FINAL_WORKER"] = "1"
        progress(config, "worker")
        with worker_stderr(config) as stderr:
            output = run_group([binary, "-test.run=^TestCOVFinalMeasure$", "-test.timeout=45m"],
                               cwd=REPO / "pkg/lang/deps", env=environment, check=check,
                               data=canonical(config), stderr_target=stderr)
        aggregate, marker = output.rsplit(b"\n", 2)[:2]
        if marker != b"PASS":
            raise ValueError("final worker did not complete")
        return json.loads(aggregate)


def run(args):
    operation_pools(args.operation)
    root = physical(args.root)
    pinned = identity(args.compiler, args.pyright)
    journal = private(root / "journal", directory=True)
    auth_path = private(journal / (args.operation + ".authorization.json"))
    auth_data = auth_path.read_bytes()
    auth = json.loads(auth_data)
    head = git("rev-parse", "HEAD").decode().strip()
    authorized(auth, root, args.operation, pinned, head)
    ensure_reader(auth)
    baseline = baselines(auth["baselines"])
    execute(args, root, journal, auth, sha(auth_data), baseline)


def rehearse(args):
    root = rehearsal_root(args.root)
    journal = private(root / "journal", directory=True)
    pinned = identity(args.compiler, args.pyright, "rehearse")
    head = git("rev-parse", "HEAD").decode().strip()
    record = dict(operation="rehearse", reader_revision=FINAL_READER, harness_revision=head)
    if getattr(args, "inject_native_failure", False):
        record["inject_native_failure"] = True
    ensure_reader(record)
    execute(args, root, journal, record, sha(canonical(dict(record, identity=pinned))), rehearsal_baselines())


def execute(args, root, journal, record, binding, baseline):
    mode = record["operation"]
    path = journal / (mode + ".json")
    tool_paths = {"python": str(Path(sys.executable).resolve()), "compiler": str(args.compiler),
                  "pyright": str(args.pyright)}
    nonce = secrets.token_hex(32)
    record = dict(record, status="started", root=str(root), started_at=now(), pools=campaign_pools(mode),
                  binding=binding, nonce_sha256=sha(nonce.encode()), tool_paths=tool_paths,
                  progress=[{"stage": "claim"}])
    config = dict(tool_paths, root=str(root), mode=mode, journal=str(path), binding=binding, nonce=nonce)
    if mode == "rehearse" and record.get("inject_native_failure"):
        config["inject_native_failure"] = True

    def report(stage, pool=None):
        progress(config, stage, pool)

    with termination() as check:
        check()
        claim(path, record)
        try:
            if mode == "final":
                require_absent(journal / "final.aggregate.json")
                require_absent(journal / "final.stderr")
            report("checkout")
            ensure_reader(record)
            measured = measure(config, check)
            aggregate = assess(measured, baseline, mode=mode, report=report)
            report("checkout")
            ensure_reader(record)
            report("publication")
            record = active(config, "development")
            aggregate_sha256 = sha(canonical(aggregate))
            check()
            claim(journal / (mode + ".aggregate.json"), aggregate)
        except BaseException as error:
            # Only settle the receipt if its last durable transition is still trusted.
            record = None
            record = active(config, "development")
            kind = type(error)
            record.update(status="failed", failure=dict(record["progress"][-1],
                          exception=kind.__name__ if kind in EXCEPTION_CLASSES else "Exception"))
            raise
        else:
            record.update(status="completed", aggregate_sha256=aggregate_sha256)
        finally:
            # A terminated worker may have been writing its next transition.
            path.with_suffix(".progress").unlink(missing_ok=True)
            if record is not None:
                record["finished_at"] = now()
                finish(path, record)
    print(record["aggregate_sha256"])


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    commands = parser.add_subparsers(dest="command", required=True)
    for command in ("preflight", "identity", "run", "rehearse"):
        sub = commands.add_parser(command)
        sub.add_argument("--compiler", required=True, type=Path)
        sub.add_argument("--pyright", required=True, type=Path)
        if command != "identity":
            sub.add_argument("root", type=Path)
        if command == "run":
            sub.add_argument("operation", choices=PRODUCTION_OPERATIONS)
        elif command == "rehearse":
            sub.add_argument("--inject-native-failure", action="store_true")
    native = commands.add_parser("_python")
    native.add_argument("pool")
    native.add_argument("subject")
    native.add_argument("arm")
    compiler_identity = commands.add_parser("_compiler_identity")
    compiler_identity.add_argument("pool")
    check = commands.add_parser("_active")
    check.add_argument("pool")
    transition = commands.add_parser("_progress")
    transition.add_argument("pool")
    transition.add_argument("stage")
    args = parser.parse_args()
    if args.command == "_active":
        config = json.load(sys.stdin)
        if config["mode"] == "preflight":
            active(config, args.pool)
        else:
            progress(config, "pool", args.pool)
        if config["mode"] == "rehearse":
            sys.stdout.buffer.write(canonical(rehearsal_subjects(args.pool)))
    elif args.command == "_progress":
        if (args.stage not in WORKER_STAGES or
                not all(stat.S_ISFIFO(os.fstat(pipe.fileno()).st_mode) for pipe in (sys.stdin, sys.stdout))):
            raise ValueError("progress requires private instrument pipes")
        config = json.load(sys.stdin)
        if config["mode"] == "preflight":
            raise ValueError("progress requires an active campaign journal")
        progress(config, args.stage, args.pool)
    elif args.command == "_python":
        if not all(stat.S_ISFIFO(os.fstat(pipe.fileno()).st_mode) for pipe in (sys.stdin, sys.stdout)):
            raise ValueError("native capture requires private instrument pipes")
        try:
            data = python_capture(json.load(sys.stdin), args.pool, args.subject, args.arm)
        except PYRIGHT["NativeNonzero"]:
            raise SystemExit(NATIVE_NONZERO_STATUS) from None
        sys.stdout.buffer.write(canonical(data))
    elif args.command == "_compiler_identity":
        if not all(stat.S_ISFIFO(os.fstat(pipe.fileno()).st_mode) for pipe in (sys.stdin, sys.stdout)):
            raise ValueError("compiler identity requires private instrument pipes")
        config = json.load(sys.stdin)
        record = active(config, args.pool)
        if config["mode"] not in ISOLATION_MODES:
            raise ValueError("compiler exit identity check requires an isolation operation")
        pinned = identity(config["compiler"], config["pyright"], config["mode"])
        if config["mode"] == "final" and pinned != record["identity"]:
            raise ValueError("compiler exit identity differs from authorization")
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
    elif args.command == "rehearse":
        rehearse(args)
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
