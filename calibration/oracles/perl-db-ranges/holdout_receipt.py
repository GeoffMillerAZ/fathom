#!/usr/bin/env python3
"""Count/hash-only receipt for a pinned Perl holdout; Dancer2 legacy is diagnostic."""

import argparse
from collections import Counter
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import statistics
import subprocess
import sys

PIN = "90b6e651c1f1a7c576072fdd039ef15afecb67a5"
HEX = re.compile(r"^[0-9a-f]{40}$")
SHA = re.compile(r"^[0-9a-f]{64}$")
PHASES = ("clone-once", "checkout-pin", "ppi-primary", "compiler-ranges",
          "build-indexbench", "grade-holdout", "retime-three")
FROZEN_FILES = ("runner.py", "dependencies.py", "FathomPerlCompileGuard.pm")


def need(ok, message):
    if not ok:
        raise ValueError(message)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def frozen_file_hashes(repo, commit):
    need(HEX.fullmatch(commit), "instrument commit must be full hex")
    hashes = {}
    for name in FROZEN_FILES:
        path = "calibration/oracles/perl-db-ranges/" + name
        result = subprocess.run(["git", "-C", str(repo), "show", commit + ":" + path],
                                stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                                check=True, timeout=10)
        hashes[name] = hashlib.sha256(result.stdout).hexdigest()
    return hashes


def load(path):
    return json.loads(Path(path).read_text())


def ratio(a, b):
    return round(a / b, 6) if b else None


def coverage_passes(compiled, units, harness_units, paired, tags, ctags_unknown_files, floor):
    return bool(units and tags and harness_units == units and ctags_unknown_files == 0 and
                compiled / units >= floor and paired / tags >= floor)


def coverage_criteria(compiled, units, harness_units, paired, tags, ctags_unknown_files, floor):
    return {
        "coverage_floors_preregistered": floor is not None,
        "compiled_coverage_floor_passed": floor is not None and
            bool(units and compiled / units >= floor),
        "ctags_population_complete": ctags_unknown_files == 0,
        "harness_population_complete": harness_units == units,
        "named_pairing_floor_passed": floor is not None and
            bool(tags and ctags_unknown_files == 0 and paired / tags >= floor),
    }


def phase_labels(phases, installed, instrument=None, source_commit=None,
                 binary_sha256=None, legacy=False):
    expected = list(PHASES)
    if installed:
        expected.insert(2, "install-declared-dependencies")
    need([row["phase"] for row in phases] == expected and
         all(row["exit"] == 0 and not row["timed_out"] and row["launch_load"] <= 9
             for row in phases), "phase sequence/status/load mismatch")
    if not legacy:
        need(HEX.fullmatch(instrument or "") and HEX.fullmatch(source_commit or "") and
             SHA.fullmatch(binary_sha256 or ""), "phase provenance inputs missing")
        need(all(row.get("instrument_commit") == instrument and
                 row.get("source_commit") == source_commit for row in phases),
             "phase instrument/source identity drift")
        need(all(row.get("binary_sha256") == binary_sha256 for row in phases
                 if row["phase"] in ("build-indexbench", "grade-holdout", "retime-three")),
             "phase binary identity drift")


def public_shape(value):
    allowed = {"Perl holdout count/hash receipt", "clone-once",
               "cpanfile", "Makefile.PL", "Build.PL", "META.json", "META.yml",
               "EPERM", "excellent", "good", "limited", "poor", "not-attempted",
               "A", "B", "C"}
    if isinstance(value, dict):
        need(all(isinstance(key, str) and re.fullmatch(r"[A-Za-z0-9_.-]+", key)
                 for key in value), "unsafe public key")
        for child in value.values():
            public_shape(child)
    elif isinstance(value, list):
        for child in value:
            public_shape(child)
    elif isinstance(value, str):
        need(value in allowed or HEX.fullmatch(value) or SHA.fullmatch(value) or
             re.fullmatch(r"[A-Za-z0-9._+-]+\.(?:tar\.gz|tgz|zip)", value) or
             re.fullmatch(r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.+\-]+", value),
             "unsafe public text")
    else:
        need(value is None or isinstance(value, (bool, int, float)), "unsafe public value")


def validate(private, snapshot, instrument, subject="Dancer2", source_commit=PIN,
             coverage_floor=None):
    spent, meta, ranges, reading, phases, measured, primary = (
        private[key] for key in ("spent", "meta", "ranges", "reading", "phases",
                                 "measured", "primary"))
    need(HEX.fullmatch(instrument) and spent["instrument_commit"] == instrument and
         measured["instrument_commit"] == instrument and
         reading["fathom_commit"] == instrument, "instrument identity mismatch")
    need(spent["source_commit"] == measured["holdout_source_commit"] == source_commit and
         spent["spent_before"] == "clone-once" and spent["spent_at_utc"],
         "holdout spend/pin mismatch")
    manifest = meta["source_manifest"]
    entries = manifest["files"]
    encoded = json.dumps(entries, sort_keys=True, separators=(",", ":")).encode()
    need(manifest["commit"] == source_commit and manifest["digest"] == hashlib.sha256(encoded).hexdigest() and
         snapshot == manifest and measured["source_manifest_sha256"] == manifest["digest"],
         "source snapshot mismatch")
    need(meta["subject"] == subject and ranges["tool"] == "perl-DB-sub-compile" and
         ranges["version"] == "v5.34.1" and ranges["tier"] == "A",
         "compiler companion identity mismatch")
    units = len(entries)
    compiled, unresolved = meta["compiled_files"], meta["unresolved_files"]
    need(units == ranges["units"] == measured["source_units"] and
         compiled + unresolved == units and ranges["errored"] == unresolved and
         len(meta["failures"]) == unresolved and
         sum(row["compiler_status"] == "ok" for row in meta["joining"]["files"]) == compiled and
         len(meta["joining"]["files"]) == units,
         "compiled/unresolved population mismatch")
    network = meta["network_policy"]["control"]
    need(network["denial"] == "EPERM" and network["destination"] == "127.0.0.1:1/tcp" and
         network["profile"] == "(version 1) (allow default) (deny network*)",
         "network denial not proven")
    dep = meta["dependency_context"]
    need(dep["inherited_PERL5LIB_PERLLIB_PERL5OPT"] == "cleared", "ambient Perl injection")
    declaration = measured["dependency_declaration"]
    install = measured["dependency_install_attempted"]
    need((declaration is None) == (not install), "dependency phase mismatch")
    need(declaration in (None, "cpanfile", "Makefile.PL", "Build.PL", "META.json", "META.yml"),
         "unexpected dependency declaration")
    if install:
        lock = private["lock"]
        need(lock["format"] == "perl-dependency-lock-v1" and lock["subject"] == subject and
             lock["declared_dependencies"]["path"] == declaration and
             dep["declared_dependencies"] == lock["declared_dependencies"] and
             dep["lock_sha256"] == private["lock_sha256"] and
             dep["local_lib_perl5_fingerprint_before"] ==
             dep["local_lib_perl5_fingerprint_after"] == lock["local_lib_perl5"] ==
             private["current_library_fingerprint"],
             "own dependency lock/fingerprint mismatch")
    else:
        need("lock" not in private and dep["declared_dependencies"] is None and
             dep["lock_sha256"] is None and
             dep["local_lib_perl5_fingerprint_before"] is None and
             dep["local_lib_perl5_fingerprint_after"] is None,
             "undeclared dependency context present")
    joined = meta["joining"]
    paired, tags = joined["paired"], joined["tag_total"]
    need(joined["ctags_unknown_files"] ==
         sum(row["ctags_status"] == "unknown" for row in joined["files"]) and
         joined["compiler_unknown_files"] == unresolved,
         "unknown Ctags/compiler population mismatch")
    reasons = Counter(reason for row in meta["failures"] for reason in row["reasons"])
    allowed_reasons = {"ctags_error_or_timeout", "compile_error", "file_timeout",
                       "total_timeout_waiting_for_load", "total_timeout_unattempted",
                       "missing_observer_output", "invalid_observer_output", "output_limit"}
    need(set(reasons) <= allowed_reasons, "unknown compiler failure reason")
    counts = reading["counts"]
    need(paired == len(ranges["ranges"]) == measured["range_paired"] and
         counts["range_eligible"] == measured["range_eligible"] and
         counts["range_ok"] == measured["range_ok"] and
         0 <= counts["range_ok"] <= counts["range_eligible"] <= paired and
         reading["metrics"]["range_agreement"] ==
             (round(counts["range_ok"] / counts["range_eligible"], 4)
              if counts["range_eligible"] else None) and
         paired + joined["tag_unpaired"] == tags == measured["range_tag_total"],
         "range companion/reading counts mismatch")
    need(len(reading["subjects"]) == 1 and reading["subjects"][0]["name"] == subject and
         reading["subjects"][0]["sha"] == source_commit and
         isinstance(reading["subjects"][0]["files"], int) and reading["subjects"][0]["files"] >= 0 and
         reading["subjects"][0]["counts"]["range_eligible"] == counts["range_eligible"] and
         reading["subjects"][0]["counts"]["range_ok"] == counts["range_ok"],
         "reading subject mismatch")
    harness_units = reading["subjects"][0]["files"]
    need(primary["tool"] == "ppi-calls" and primary["version"] == "PPI 1.270" and
         primary["tier"] == "C", "primary call oracle changed")
    legacy = subject == "Dancer2" and source_commit == PIN and coverage_floor is None
    phase_labels(phases, install, instrument, source_commit, private["binary_sha256"], legacy)
    runs = private["retime_runs"]
    need([label for label, _ in runs] == ["1", "2", "3"], "retime labels must be 1,2,3 once")
    timings = [float(value) for _, value in runs]
    need(measured["throughput_three_runs_mb_cpu_s"] == timings and
         measured["throughput_median_mb_cpu_s"] == statistics.median(timings),
         "retime values mismatch")
    need(measured["grade"] == reading["grade"] and
         measured["metric_tiers"] == reading["metric_tiers"] and
         measured["metrics"] == reading["metrics"] and
         measured["counts"] == counts,
         "measured reading mismatch")
    gates = {
        "range_agreement_at_least_0_90": reading["metrics"]["range_agreement"] is not None and
            reading["metrics"]["range_agreement"] >= .90,
        "l1_excellent": reading["grade"].get("L1") == "excellent",
        "throughput_median_at_least_1_mb_cpu_s": statistics.median(timings) >= 1,
        "max_file_at_most_2_seconds": reading["tenancy"]["max_file_ms"] <= 2000,
        "fault_rate_at_most_0_1_percent":
            reading["tenancy"]["faults"] / max(1, reading["subjects"][0]["files"]) <= .001,
    }
    need(all(measured[key] == value for key, value in gates.items()) and
         measured["all_preregistered_excellence_gates"] == all(gates.values()),
         "excellence gate mismatch")
    coverage_ok = (coverage_floor is not None and
                   coverage_passes(compiled, units, harness_units, paired, tags,
                                   joined["ctags_unknown_files"], coverage_floor))
    need(private["binary_sha256"] == measured["binary_sha256"] and
         measured["ppi_sha256"] == private["primary_sha256"] and
         measured["ranges_sha256"] == private["ranges_sha256"] and
         measured["reading_sha256"] == private["reading_sha256"],
         "binary/oracle/reading hash mismatch")
    need(SHA.fullmatch(private["binary_sha256"]) and
         SHA.fullmatch(meta["perl_binary_sha256"]) and
         SHA.fullmatch(meta["ctags"]["binary_sha256"]) and
         meta["ctags"]["binary_sha256"] == measured["ctags_binary_sha256"],
         "tool hash mismatch")
    result = {
        "tool": "Perl holdout count/hash receipt",
        "source_commit": source_commit, "instrument_commit": instrument,
        "spent_before": "clone-once", "spent_at_utc": spent["spent_at_utc"],
        "source_manifest_sha256": manifest["digest"], "units": units,
        "harness_files": harness_units, "native_minus_harness_files": units - harness_units,
        "compiled": compiled, "unresolved": unresolved,
        "failure_reason_counts": dict(sorted(reasons.items())),
        "ctags_unknown_files": joined["ctags_unknown_files"],
        "compiled_coverage": ratio(compiled, units),
        "named_tags": tags, "paired": paired, "paired_tag_coverage": ratio(paired, tags),
        "paired_outside_reading": paired - counts["range_eligible"],
        "coverage_floor": coverage_floor,
        "phase_identity_bound": not legacy,
        "legacy_phase_provenance_incomplete": legacy,
        "range_eligible": counts["range_eligible"], "range_ok": counts["range_ok"],
        "range_disagree": counts["range_eligible"] - counts["range_ok"],
        "range_agreement": reading["metrics"]["range_agreement"],
        "grade": {k: reading["grade"][k] for k in ("L1", "L2", "L3", "L4")},
        "metric_tiers": {k: reading["metric_tiers"][k] for k in sorted(reading["metric_tiers"])},
        "call_truth": counts["call_truth"], "call_truth_found": counts["call_truth_found"],
        "dependency_declaration": declaration, "dependency_installed": install,
        "local_library_sha256": (dep["local_lib_perl5_fingerprint_before"] or {}).get("sha256"),
        "dependency_lock_sha256": dep["lock_sha256"],
        "network_denial": "EPERM", "perl_binary_sha256": meta["perl_binary_sha256"],
        "ctags_binary_sha256": meta["ctags"]["binary_sha256"],
        "indexbench_binary_sha256": private["binary_sha256"],
        "throughput_three_runs_mb_cpu_s": timings,
        "throughput_median_mb_cpu_s": statistics.median(timings),
        "phase_launch_load": [row["launch_load"] for row in phases],
        "phase_seconds": [row["seconds"] for row in phases],
        "criteria": {**gates, "all_preregistered_excellence_gates": all(gates.values()),
                     **coverage_criteria(compiled, units, harness_units, paired, tags,
                                         joined["ctags_unknown_files"], coverage_floor),
                     "coverage_and_existing_gates_pass": coverage_ok and all(gates.values())},
    }
    public_shape(result)
    return result


def collect(scratch, repo, instrument, subject="Dancer2", source_commit=PIN,
            coverage_floor=None):
    spec = importlib.util.spec_from_file_location(
        "perl_holdout_runner", repo / "calibration/oracles/perl-db-ranges/runner.py")
    need(spec and spec.loader, "range runner unavailable")
    sys.path.insert(0, str(repo / "calibration/oracles/perl-db-ranges"))
    runner = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(runner)
    snapshot = runner.snapshot(scratch / (subject + "-source"))
    paths = {"spent": scratch / "holdout-spent.json",
             "meta": scratch / (subject + ".ranges.meta.json"),
             "ranges": scratch / (subject + ".ranges.json"),
             "reading": scratch / "reading.json", "phases": scratch / "phases.json",
             "measured": scratch / "measured.private.json", "primary": scratch / (subject + ".json"),
             "retime": scratch / "retime-three.log", "binary": scratch / "indexbench"}
    private = {key: load(path) for key, path in paths.items() if key not in ("retime", "binary")}
    private["retime_runs"] = re.findall(
        r"^run ([0-9]+): ([0-9]+\.[0-9]+) MB/CPU-s$", paths["retime"].read_text(), re.M)
    private.update({"binary_sha256": digest(paths["binary"]),
                    "primary_sha256": digest(paths["primary"]),
                    "ranges_sha256": digest(paths["ranges"]),
                    "reading_sha256": digest(paths["reading"])})
    lock_path = scratch / "dependency-lock.json"
    if lock_path.is_file():
        private["lock"] = load(lock_path)
        private["lock_sha256"] = digest(lock_path)
        from importlib.machinery import SourceFileLoader
        dep_module = SourceFileLoader(
            "perl_holdout_dependencies",
            str(repo / "calibration/oracles/perl-db-ranges/dependencies.py")).load_module()
        private["current_library_fingerprint"] = dep_module.library_fingerprint(
            scratch / "local-perl/lib/perl5")
        archive_path = scratch / "dependency-archives.private.json"
        archive_state = load(archive_path)
        need(archive_state["local_lib_perl5"] == private["current_library_fingerprint"],
             "saved archive metadata differs from local library")
        private["archive_state"] = archive_state
    else:
        need(not private["measured"]["dependency_install_attempted"],
             "installed dependencies lack lock")
    result = validate(private, snapshot, instrument, subject, source_commit, coverage_floor)
    result["collector_sha256"] = digest(__file__)
    result["frozen_instrument_file_sha256"] = frozen_file_hashes(repo, instrument)
    result["input_sha256"] = {key + ".sha256": digest(path) for key, path in paths.items()
                               if key != "binary"}
    result["input_sha256"]["indexbench.sha256"] = private["binary_sha256"]
    if lock_path.is_file():
        result["input_sha256"]["dependency-lock.sha256"] = private["lock_sha256"]
        archive_rows = private["archive_state"]["saved_distributions"]
        need(all(re.fullmatch(r"[A-Za-z0-9._+-]+\.(?:tar\.gz|tgz|zip)", row["basename"]) and
                 SHA.fullmatch(row["sha256"]) and row["bytes"] >= 0 for row in archive_rows),
             "unsafe dependency archive metadata")
        need(len(archive_rows) == len({row["basename"] for row in archive_rows}),
             "duplicate dependency archive basename")
        result["dependency_archives"] = [
            {"name": row["basename"], "sha256": row["sha256"], "bytes": row["bytes"]}
            for row in sorted(archive_rows, key=lambda row: row["basename"])]
        result["input_sha256"]["dependency-archives.sha256"] = digest(archive_path)
    public_shape(result)
    return result


def self_test():
    # Exercise the privacy boundary and a real consistency refusal without a source checkout.
    private = {"name": "Author Person", "path": "private/source.pm",
               "native": [{"name": "Private::Method", "path": "private/source.pm"}]}
    projected = {"native_count": len(private["native"]), "private_sha256":
                 hashlib.sha256(json.dumps(private, sort_keys=True).encode()).hexdigest()}
    need("Author Person" not in json.dumps(projected) and
         "private/source.pm" not in json.dumps(projected), "private field leaked")
    phases = [{"phase": label, "exit": 0, "timed_out": False, "launch_load": 1}
              for label in PHASES]
    phase_labels(phases, False, legacy=True)
    try:
        phase_labels(phases[:1] + phases[:1] + phases[2:], False, legacy=True)
    except ValueError:
        pass
    else:
        raise AssertionError("duplicate phase accepted")
    try:
        public_shape({"native": private["native"]})
    except ValueError:
        pass
    else:
        raise AssertionError("private native array accepted")
    need(coverage_passes(95, 100, 100, 95, 100, 0, .95) and
         not coverage_passes(94, 100, 100, 95, 100, 0, .95) and
         not coverage_passes(95, 100, 100, 94, 100, 0, .95) and
         not coverage_passes(95, 100, 100, 95, 100, 1, .95) and
         not coverage_passes(95, 100, 89, 95, 100, 0, .95),
         "coverage floor control failed")
    failed_coverage = coverage_criteria(94, 100, 100, 95, 100, 0, .95)
    need(not failed_coverage["compiled_coverage_floor_passed"] and
         failed_coverage["named_pairing_floor_passed"], "low coverage incorrectly passed")
    public_shape({"criteria": failed_coverage})
    gap_coverage = coverage_criteria(95, 100, 89, 95, 100, 0, .95)
    need(not gap_coverage["harness_population_complete"],
         "harness discovery gap incorrectly passed")
    public_shape({"criteria": gap_coverage, "native_minus_harness_files": 11})
    future = [{**row, "instrument_commit": "a" * 40,
               "source_commit": "b" * 40,
               **({"binary_sha256": "c" * 64} if row["phase"] in
                  ("build-indexbench", "grade-holdout", "retime-three") else {})}
              for row in phases]
    phase_labels(future, False, "a" * 40, "b" * 40, "c" * 64)
    for key, index, bad in (("instrument_commit", 0, "d" * 40),
                            ("source_commit", 1, "d" * 40),
                            ("binary_sha256", 4, "d" * 64)):
        drift = [dict(row) for row in future]
        drift[index][key] = bad
        try:
            phase_labels(drift, False, "a" * 40, "b" * 40, "c" * 64)
        except ValueError:
            pass
        else:
            raise AssertionError("phase identity drift accepted: " + key)
    return {"privacy": "pass", "duplicate_phase": "rejected",
            "native_array": "rejected", "low_coverage": "publishable_failure",
            "ctags_unknown": "coverage_failure", "population_gap": "publishable_failure",
            "phase_identity_drift": "rejected"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scratch", type=Path)
    parser.add_argument("--instrument-repo", type=Path)
    parser.add_argument("--instrument-commit")
    parser.add_argument("--subject", default="Dancer2")
    parser.add_argument("--source-commit", default=PIN)
    parser.add_argument("--coverage-floor", type=float)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        need(not any((args.scratch, args.instrument_repo, args.instrument_commit,
                      args.output)), "self-test takes no inputs")
        print(json.dumps(self_test(), sort_keys=True))
        return
    need(all((args.scratch, args.instrument_repo, args.instrument_commit, args.output)),
         "scratch, instrument repo/commit, output required")
    need(re.fullmatch(r"[A-Za-z][A-Za-z0-9_-]*", args.subject) and
         HEX.fullmatch(args.source_commit), "unsafe subject or source commit")
    need((args.subject == "Dancer2" and args.source_commit == PIN and args.coverage_floor is None) or
         (args.subject != "Dancer2" and args.coverage_floor == .95),
         "new holdout requires preregistered 95% coverage floors; Dancer2 is spent")
    need(not args.output.exists(), "output must be a new file")
    result = collect(args.scratch.resolve(strict=True),
                     args.instrument_repo.resolve(strict=True), args.instrument_commit,
                     args.subject, args.source_commit, args.coverage_floor)
    args.output.write_text(json.dumps(result, sort_keys=True, indent=2) + "\n")
    print(json.dumps({"receipt_sha256": digest(args.output), "units": result["units"]},
                     sort_keys=True))


if __name__ == "__main__":
    main()
