#!/usr/bin/env python3
"""Bounded compile-only control for the named-method B::Xref 1.07 derivative."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import resource
import subprocess
import sys
import tempfile
import time

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import runner  # noqa: E402

MAX_OUTPUT_BYTES = 64 * 1024 * 1024
EXPECTED_DIRECT = {(19, "first"), (20, "second")}
EXPECTED_NAMED = {(22, "method"), (23, "method")}
EXPECTED_DYNAMIC_LINES = {25}


def checked_output_dir(value):
    outdir = Path(value).resolve()
    repo_root = Path(runner.git(HERE, "rev-parse", "--show-toplevel")).resolve(strict=True)
    permitted = [repo_root / "workspace"]
    if os.environ.get("TMPDIR"):
        permitted.append(Path(os.environ["TMPDIR"]).resolve(strict=True))
    if (outdir.exists() or outdir.is_relative_to(HERE) or
            not any(outdir.is_relative_to(root) for root in permitted)):
        raise ValueError("--output-dir must be a fresh repo workspace or TMPDIR path outside the instrument")
    return outdir


def file_limit(bytes_per_file):
    def apply():
        resource.setrlimit(resource.RLIMIT_FSIZE, (bytes_per_file, bytes_per_file))
    return apply


def self_test():
    repo_root = Path(runner.git(HERE, "rev-parse", "--show-toplevel")).resolve(strict=True)
    scratch = repo_root / "workspace/tmp"
    scratch.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="perl-method-selftest-", dir=scratch) as temporary:
        directory = Path(temporary)
        if checked_output_dir(directory / "fresh") != directory / "fresh":
            raise AssertionError("workspace path control failed")
        try:
            checked_output_dir(HERE / "forbidden")
        except ValueError:
            pass
        else:
            raise AssertionError("instrument path accepted")
        example = f"{HERE / 'named_methods.pl'} MethodProbe::caller    22 (method) -> method meth\n"
        if raw_records(example, HERE / "named_methods.pl")[0]["name"] != "method":
            raise AssertionError("raw record parser failed")
        target = directory / "limited.raw"
        with target.open("wb") as output:
            child = subprocess.run(
                [sys.executable, "-c",
                 "import os; assert os.write(1, b'x' * 1024) == 1024; os.write(1, b'y')"],
                stdout=output, stderr=subprocess.DEVNULL,
                preexec_fn=file_limit(1024), timeout=5)
        if child.returncode == 0 or target.stat().st_size > 1024:
            raise AssertionError("live file size cap failed")
    return {"path_controls": "pass", "raw_parser": "pass", "live_file_cap": "pass"}


def raw_records(raw, source):
    prefix = str(source) + " "
    rows = []
    for line in raw.splitlines():
        if not line.startswith(prefix):
            continue
        fields = line[len(prefix):].split()
        if len(fields) < 6 or not fields[1].isdigit():
            continue
        rows.append({"caller": fields[0], "line": int(fields[1]),
                     "package": fields[2], "type": fields[3],
                     "name": fields[4], "event": fields[5]})
    return rows


def run(args):
    perl = str(Path(args.perl).resolve(strict=True))
    source = (HERE / "named_methods.pl").resolve(strict=True)
    if source.parent != HERE or not source.is_file():
        raise ValueError("fixed probe fixture missing or outside instrument directory")
    env = runner.perl_environment()
    version = subprocess.check_output(runner.sandbox_command(perl, "-e", "print $^V"),
                                      env=env, text=True, timeout=5).strip()
    if version != "v5.34.1":
        raise ValueError("native probe requires Perl 5.34.1")
    network = runner.verify_network_denied(perl, env)
    inc = runner.perl_inc_context(perl, HERE, None, env)
    core_args = ["-I" + path for path in inc["core"]]
    identity_code = ("use B::Xref; use JSON::PP; "
                     "print JSON::PP->new->encode({version=>$B::Xref::VERSION,"
                     "module=>$INC{'B/Xref.pm'}})")
    identity = json.loads(subprocess.check_output(
        runner.sandbox_command(perl, *core_args, "-e", identity_code),
        env=env, text=True, timeout=5))
    if identity["version"] != "1.07":
        raise ValueError("requires stock B::Xref 1.07")
    stock = runner.core_module_identity(identity["module"], inc["core"])
    adapter = HERE / "FathomPerlNamedMethodXref.pm"
    adapter_digest = hashlib.sha256(adapter.read_bytes()).hexdigest()
    source_digest = hashlib.sha256(source.read_bytes()).hexdigest()
    binary_digest = hashlib.sha256(Path(perl).read_bytes()).hexdigest()
    # The guard resets @INC; explicitly retain the instrument's adapter root.
    inc["inc"].append(str(HERE))
    inc["roots"].append(str(HERE))
    outdir = checked_output_dir(args.output_dir)
    outdir.mkdir(parents=True)
    profile = runner.compiler_profile(inc, outdir)
    run_env = env.copy()
    run_env.update({"FATHOM_DB_RANGE_OUT": str(outdir / "guard.json"),
                    "FATHOM_ALLOWED_INC": os.pathsep.join(inc["inc"]),
                    "FATHOM_ALLOWED_ROOTS": os.pathsep.join(inc["roots"]),
                    "FATHOM_PATH_SEP": os.pathsep})
    command = runner.sandbox_command(
        perl, *core_args, "-I" + str(runner.OBSERVER.parent),
        "-MFathomPerlCompileGuard", "-I" + str(HERE),
        "-MFathomPerlNamedMethodXref", "-MO=Xref,-r", "-c", str(source),
        profile=profile)
    deadline = time.monotonic() + runner.TOTAL_SECONDS
    if not runner.wait_for_load(deadline):
        raise RuntimeError("system load exceeded 9 until total deadline")

    with (outdir / "stdout.raw").open("wb") as stdout, (outdir / "stderr.raw").open("wb") as stderr:
        proc = subprocess.Popen(command, cwd=HERE, env=run_env,
                                stdout=stdout, stderr=stderr, start_new_session=True,
                                preexec_fn=file_limit(MAX_OUTPUT_BYTES // 2))
        timed_out = False
        try:
            try:
                proc.communicate(timeout=max(.01, runner.compiler_deadline(deadline) - time.monotonic()))
            except subprocess.TimeoutExpired:
                timed_out = True
        finally:
            runner.stop_group(proc)
    size = sum((outdir / name).stat().st_size for name in ("stdout.raw", "stderr.raw"))
    if (timed_out or size >= MAX_OUTPUT_BYTES or proc.returncode or
            not (outdir / "guard.json").is_file()):
        raise RuntimeError(json.dumps({"timeout": timed_out, "output_bytes": size,
                                       "returncode": proc.returncode,
                                       "guard_written": (outdir / "guard.json").is_file()}))
    rows = raw_records((outdir / "stdout.raw").read_text(errors="replace"), source)
    caller = [row for row in rows if row["caller"] == "MethodProbe::caller"]
    direct = [row for row in caller if row["event"] == "subused" and row["type"] == "&"]
    named = [row for row in caller if row["event"] == "meth" and row["name"] == "method"]
    unknown = [row for row in caller if row["event"] == "subused" and row["type"] != "&"]
    observed_direct = {(row["line"], row["name"]) for row in direct}
    observed_named = {(row["line"], row["name"]) for row in named}
    observed_dynamic = {row["line"] for row in unknown}
    if (observed_direct != EXPECTED_DIRECT or observed_named != EXPECTED_NAMED or
            observed_dynamic != EXPECTED_DYNAMIC_LINES):
        raise RuntimeError(json.dumps({"expected_direct": sorted(EXPECTED_DIRECT),
                                       "observed_direct": sorted(observed_direct),
                                       "expected_named": sorted(EXPECTED_NAMED),
                                       "observed_named": sorted(observed_named),
                                       "expected_dynamic": sorted(EXPECTED_DYNAMIC_LINES),
                                       "observed_dynamic": sorted(observed_dynamic)}))
    closure_observed = sum(row["event"] == "meth" and row["name"] == "closure_only" for row in rows)
    constant_observed = sum(row["caller"] == "MethodProbe::constant_caller" and
                            row["event"] == "subused" for row in rows)
    result = {"tool": "Fathom named-method derivative of stock B::Xref 1.07",
              "stock_backend": stock, "adapter_sha256": adapter_digest,
              "perl_version": version, "perl_binary_sha256": binary_digest,
              "network_control": network, "source": source.name,
              "source_sha256": source_digest,
              "expected_direct_lines": [19, 20],
              "direct_lines": sorted(row["line"] for row in direct),
              "expected_named_method_lines": [22, 23],
              "named_method_lines": sorted(row["line"] for row in named),
              "expected_unknown_lines": [25],
              "unknown_lines": sorted(row["line"] for row in unknown),
              "closure_source_calls": 1, "closure_method_records": closure_observed,
              "closure_unresolved": max(0, 1 - closure_observed),
              "constant_source_calls": 1, "constant_source_call_records": constant_observed,
              "constant_unresolved": max(0, 1 - constant_observed),
              "scope": "Synthetic control only; unresolved closure and optimized constant calls are population gaps, not oracle-known edges"}
    (outdir / "result.json").write_text(json.dumps(result, sort_keys=True, indent=2) + "\n")
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--perl")
    parser.add_argument("--output-dir")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        if args.perl or args.output_dir:
            parser.error("--self-test takes no probe arguments")
        print(json.dumps(self_test(), sort_keys=True))
        return
    if not args.perl or not args.output_dir:
        parser.error("--perl and --output-dir are required for the probe")
    print(json.dumps(run(args), sort_keys=True))


if __name__ == "__main__":
    main()
