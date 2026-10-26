#!/usr/bin/env python3
"""Bounded, provenance-recorded regrade of frozen Perl native evidence."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import receipt
import runner

HERE = Path(__file__).resolve().parent
NAMES = ("mojo", "Perl-Critic")
TOTAL_SECONDS = 600
PHASE_SECONDS = 240
NICE = "/usr/bin/nice"


def require(ok, message):
    if not ok:
        raise ValueError(message)


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def fresh_output(value, repo):
    out = Path(value).resolve()
    allowed = [repo / "workspace"]
    if os.environ.get("TMPDIR"):
        allowed.append(Path(os.environ["TMPDIR"]).resolve(strict=True))
    require(not out.exists() and any(out.is_relative_to(root) for root in allowed),
            "out-dir must be fresh under repo workspace or configured TMPDIR")
    return out


def assigned_build_dirs(repo):
    for key in ("TMPDIR", "GOTMPDIR", "GOCACHE"):
        raw = os.environ.get(key)
        require(raw and Path(raw).is_absolute(), key + " must be an assigned absolute directory")
        path = Path(raw).resolve(strict=True)
        require(path.is_dir() and not path.is_relative_to(repo),
                key + " must exist outside the checkout")


def subject_roots(values):
    result = {}
    for value in values:
        name, sep, path = value.partition("=")
        require(sep and name in NAMES and name not in result and path,
                "--subject must uniquely name mojo or Perl-Critic")
        result[name] = Path(path).resolve(strict=True)
    require(set(result) == set(NAMES) and len(set(result.values())) == 2,
            "exactly two distinct pinned subjects required")
    return result


def immutable_inputs(input_dir, roots, ctags, native_commit):
    phases = json.loads((input_dir / "phases.json").read_text())
    require(len(phases) == 5 and all(row["exit"] == 0 and
            row["instrument_commit"] == native_commit for row in phases),
            "native phase provenance mismatch")
    identity = runner.ctags_identity(ctags)
    files = {}
    manifests = {}
    for name in NAMES:
        meta = json.loads((input_dir / (name + ".ranges.meta.json")).read_text())
        current = runner.snapshot(roots[name])
        require(meta["subject"] == name and current == meta["source_manifest"],
                "subject source differs from frozen native manifest")
        require(meta["ctags"] == identity, "Ctags differs from frozen native instrument")
        ranges = json.loads((input_dir / (name + ".ranges.json")).read_text())
        require(ranges["units"] == len(current["files"]) and
                ranges["errored"] + meta["compiled_files"] == ranges["units"],
                "range companion population mismatch")
        primary = json.loads((input_dir / (name + ".json")).read_text())
        require(primary["tool"] == "ppi-calls" and primary["version"] == "PPI 1.270" and
                primary["tier"] == "C", "primary call oracle mismatch")
        manifests[name] = {"commit": current["commit"], "digest": current["digest"],
                           "units": len(current["files"])}
        for suffix in (".json", ".ranges.json", ".ranges.meta.json"):
            path = input_dir / (name + suffix)
            files[name + suffix] = sha(path)
    files["phases.json"] = sha(input_dir / "phases.json")
    return identity, manifests, files


def wait_for_load(deadline):
    while os.getloadavg()[0] > 9:
        require(time.monotonic() < deadline, "total deadline exceeded in load wait")
        time.sleep(min(5, max(0.1, deadline - time.monotonic())))
    return os.getloadavg()[0]


def phase(label, command, cwd, out, env, deadline, harness_commit, binary):
    launch_load = wait_for_load(deadline)
    started = time.monotonic()
    timeout = min(PHASE_SECONDS, deadline - started)
    require(timeout > 0, "total deadline exceeded")
    log = out / (label + ".log")
    with log.open("wb") as stream:
        process = subprocess.Popen([NICE, "-n", "10", *map(str, command)],
                                   cwd=cwd, env=env, stdout=stream,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        timed_out = False
        try:
            process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            timed_out = True
        finally:
            runner.stop_group(process)
    row = {"phase": label, "exit": process.returncode, "timeout": timed_out,
           "launch_load": launch_load, "seconds": round(time.monotonic() - started, 3),
           "instrument_commit": harness_commit,
           "binary_sha256": sha(binary) if binary.is_file() else None}
    return row


def run(args):
    repo = Path(runner.git(HERE, "rev-parse", "--show-toplevel")).resolve(strict=True)
    harness_commit = runner.git(repo, "rev-parse", "HEAD")
    require(receipt.HEX.fullmatch(harness_commit) and
            not runner.git(repo, "status", "--porcelain", "--untracked-files=all"),
            "harness checkout must be a clean full commit")
    require(receipt.HEX.fullmatch(args.native_instrument_commit),
            "native instrument commit must be full hex")
    roots = subject_roots(args.subject)
    input_dir = Path(args.input_dir).resolve(strict=True)
    ctags = Path(args.ctags).resolve(strict=True)
    assigned_build_dirs(repo)
    out = fresh_output(args.out_dir, repo)
    identity, manifests, input_hashes = immutable_inputs(
        input_dir, roots, ctags, args.native_instrument_commit)
    out.mkdir(parents=True)
    for name in NAMES:
        for suffix in (".json", ".ranges.json"):
            shutil.copyfile(input_dir / (name + suffix), out / (name + suffix))
    env = os.environ.copy()
    for key in ("PERL5LIB", "PERLLIB", "PERL5OPT"):
        env.pop(key, None)
    env.update({"GOPROXY": "off", "GOSUMDB": "off",
                "GOTOOLCHAIN": "local", "GOWORK": "off"})
    binary = out / "indexbench"
    deadline = time.monotonic() + TOTAL_SECONDS
    phases = []

    def execute(label, command):
        row = phase(label, command, repo, out, env, deadline, harness_commit, binary)
        phases.append(row)
        (out / "grade-phases.json").write_text(json.dumps(phases, sort_keys=True, indent=2) + "\n")
        require(not row["timeout"] and row["exit"] == 0, "phase failed: " + label)

    execute("build-indexbench", ["go", "build", "-trimpath", "-buildvcs=false",
                                 "-o", binary, "./cmd/indexbench"])
    for name in NAMES:
        execute("grade-" + name,
                [binary, "lang-oracle", "-dialect", "perl", "-subject", roots[name],
                 "-subject-name", name, "-ctags", ctags,
                 "-tool-oracle", out / (name + ".json"),
                 "-out", out / "reading.json", "-commit", harness_commit])
    execute("retime", [binary, "lang-retime", "-reading", out / "reading.json",
                       "-subject", "mojo=" + str(roots["mojo"]),
                       "-subject", "Perl-Critic=" + str(roots["Perl-Critic"]),
                       "-runs", "3", "-write"])
    require(sha(binary) == phases[-1]["binary_sha256"], "binary changed during grade")
    require(input_hashes == immutable_inputs(input_dir, roots, ctags,
            args.native_instrument_commit)[2], "native evidence changed during grade")
    baseline = json.loads((repo / "docs/bench/readings/lang/perl.json").read_text())
    reading = json.loads((out / "reading.json").read_text())
    require(reading["fathom_commit"] == harness_commit, "reading harness commit mismatch")
    receipt.validate_reading(reading, baseline)
    return {"native_instrument_commit": args.native_instrument_commit,
            "harness_commit": harness_commit, "binary_sha256": sha(binary),
            "ctags_binary_sha256": identity["binary_sha256"],
            "source_manifests": manifests, "input_sha256": input_hashes,
            "grade_phases_sha256": sha(out / "grade-phases.json"),
            "reading_sha256": sha(out / "reading.json")}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input-dir", required=True)
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--subject", action="append", required=True)
    parser.add_argument("--ctags", required=True)
    parser.add_argument("--native-instrument-commit", required=True)
    args = parser.parse_args()
    result = run(args)
    output = Path(args.out_dir).resolve() / "grade.meta.json"
    output.write_text(json.dumps(result, sort_keys=True, indent=2) + "\n")
    print(json.dumps({"grade_meta_sha256": sha(output),
                      "harness_commit": result["harness_commit"]}, sort_keys=True))


if __name__ == "__main__":
    main()
