#!/usr/bin/env python3
"""Count/hash-only receipt for a pinned Perl holdout; Dancer2 legacy is diagnostic."""

import argparse
from collections import Counter
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import statistics
import subprocess
import sys
import tempfile

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


def artifact_shape(key, value):
    required = {
        "spent": ("instrument_commit", "source_commit", "spent_before", "spent_at_utc"),
        "meta": ("subject", "source_manifest", "compiled_files", "unresolved_files",
                 "failures", "joining", "network_policy", "dependency_context"),
        "ranges": ("tool", "version", "tier", "units", "errored", "ranges"),
        "reading": ("fathom_commit", "subjects", "counts", "metrics"),
        "measured": ("instrument_commit", "holdout_source_commit", "source_manifest_sha256"),
        "primary": ("tool", "version", "tier"),
    }
    if key == "phases":
        return isinstance(value, list) and all(isinstance(row, dict) and
            isinstance(row.get("phase"), str) and
            type(row.get("exit")) is int and isinstance(row.get("timed_out"), bool) and
            type(row.get("launch_load")) in (int, float) and
            type(row.get("seconds")) in (int, float)
            for row in value)
    if not isinstance(value, dict) or not all(field in value for field in required[key]):
        return False
    if key == "meta":
        return (type(value["compiled_files"]) is int and
                type(value["unresolved_files"]) is int and
                isinstance(value["source_manifest"], dict) and
                all(k in value["source_manifest"] for k in ("commit", "digest", "files")) and
                isinstance(value["source_manifest"]["files"], list) and
                isinstance(value["joining"], dict) and
                all(k in value["joining"] for k in ("files", "tag_total", "paired",
                                                    "ctags_unknown_files", "compiler_unknown_files",
                                                    "tag_unpaired")) and
                isinstance(value["joining"]["files"], list) and
                all(type(value["joining"][k]) is int for k in
                    ("tag_total", "paired", "ctags_unknown_files",
                     "compiler_unknown_files", "tag_unpaired")) and
                all(isinstance(row, dict) and "ctags_status" in row
                    for row in value["joining"]["files"]) and
                isinstance(value["failures"], list) and
                all(isinstance(row, dict) and isinstance(row.get("reasons"), list)
                    for row in value["failures"]) and
                isinstance(value["network_policy"], dict) and
                isinstance(value["network_policy"].get("control"), dict) and
                all(k in value["network_policy"]["control"]
                    for k in ("denial", "destination", "profile")) and
                isinstance(value["dependency_context"], dict) and
                all(k in value["dependency_context"]
                    for k in ("local_lib_perl5_fingerprint_before",
                              "local_lib_perl5_fingerprint_after", "lock_sha256")) and
                isinstance(value.get("ctags"), dict) and
                all(k in value["ctags"] for k in ("version", "binary_sha256")) and
                "perl_binary_sha256" in value)
    if key == "reading":
        return (isinstance(value["subjects"], list) and
                len(value["subjects"]) == 1 and
                isinstance(value["subjects"][0], dict) and
                all(k in value["subjects"][0] for k in ("name", "sha", "files")) and
                isinstance(value["counts"], dict) and
                isinstance(value["metrics"], dict) and
                all(type(value["counts"].get(k)) is int
                    for k in ("range_eligible", "range_ok")))
    return True


def available_identity(key, value, instrument, source_commit, subject):
    """Reject parseable identity drift even when the rest of an artifact is truncated."""
    if not isinstance(value, dict):
        return
    expected = {
        "spent": {"instrument_commit": instrument, "source_commit": source_commit},
        "reading": {"fathom_commit": instrument},
        "measured": {"instrument_commit": instrument,
                     "holdout_source_commit": source_commit},
        "meta": {"subject": subject},
    }.get(key, {})
    for field, wanted in expected.items():
        if field in value:
            need(value[field] == wanted, key + " parseable identity drift")
    if key == "meta" and isinstance(value.get("source_manifest"), dict):
        if "commit" in value["source_manifest"]:
            need(value["source_manifest"]["commit"] == source_commit,
                 "meta parseable source identity drift")
    if key == "meta":
        if "perl_binary_sha256" in value:
            need(SHA.fullmatch(value["perl_binary_sha256"] or ""),
                 "meta parseable Perl identity malformed")
        if isinstance(value.get("ctags"), dict):
            ctags = value["ctags"]
            if "version" in ctags:
                need(isinstance(ctags["version"], str) and
                     ctags["version"].startswith("Universal Ctags 6.2.1,"),
                     "meta parseable Ctags version drift")
            if "binary_sha256" in ctags:
                need(SHA.fullmatch(ctags["binary_sha256"] or ""),
                     "meta parseable Ctags hash malformed")
    if key == "ranges":
        for field, wanted in (("tool", "perl-DB-sub-compile"),
                              ("version", "v5.34.1"), ("tier", "A")):
            if field in value:
                need(value[field] == wanted, "ranges parseable tool identity drift")
    if key == "primary":
        for field, wanted in (("tool", "ppi-calls"),
                              ("version", "PPI 1.270"), ("tier", "C")):
            if field in value:
                need(value[field] == wanted, "primary parseable tool identity drift")
    if key == "reading" and isinstance(value.get("subjects"), list):
        for row in value["subjects"]:
            if isinstance(row, dict):
                if "sha" in row:
                    need(row["sha"] == source_commit, "reading parseable source identity drift")
                if "name" in row:
                    need(row["name"] == subject, "reading parseable subject identity drift")


def ratio(a, b):
    return round(a / b, 6) if b else None


def retime_input_digest(source_manifest_digest, primary_sha256, ranges_sha256,
                        binary_sha256, reading):
    """Hash immutable retime inputs; ignore only fields lang-retime rewrites."""
    need(all(SHA.fullmatch(value or "") for value in
             (source_manifest_digest, primary_sha256, ranges_sha256, binary_sha256)),
         "retime input identity malformed")
    projection = copy.deepcopy(reading)
    need(isinstance(projection, dict) and isinstance(projection.get("subjects"), list),
         "retime reading shape malformed")
    projection.pop("grade", None)
    projection.pop("level_met", None)
    projection["notes"] = sorted(note for note in projection.get("notes", [])
                                 if not note.startswith("throughput (design 1.9 T1, T2)"))
    if "kind_intersection" in projection:
        projection["kind_intersection"] = sorted(projection["kind_intersection"])
    for key in ("throughput_mb_s", "throughput_basis", "wall_throughput_mb_s", "max_file_ms"):
        projection.get("tenancy", {}).pop(key, None)
    for subject in projection["subjects"]:
        for key in ("extract_ms", "extract_cpu_ms", "max_file_ms"):
            subject.get("tenancy", {}).pop(key, None)
    payload = {"source_manifest_sha256": source_manifest_digest,
               "primary_sha256": primary_sha256, "ranges_sha256": ranges_sha256,
               "binary_sha256": binary_sha256, "reading": projection}
    encoded = json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(encoded).hexdigest()


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
                 binary_sha256=None, legacy=False, complete=True,
                 retime_input_sha256=None):
    expected = list(PHASES)
    if installed:
        expected.insert(2, "install-declared-dependencies")
    labels = [row["phase"] for row in phases]
    retime_recovery = labels == expected + ["retime-three"]
    need((retime_recovery or labels == (expected if complete else expected[:len(labels)])) and
         all(isinstance(row["exit"], int) and isinstance(row["timed_out"], bool) and
             isinstance(row["launch_load"], (int, float)) and row["launch_load"] >= 0
             for row in phases), "phase sequence/shape mismatch")
    if retime_recovery:
        first, second = phases[-2:]
        need(first["exit"] == 0 and not first["timed_out"] and first["launch_load"] > 9 and
             SHA.fullmatch(first.get("retime_input_sha256", "")) and
             second.get("retime_input_sha256") == first["retime_input_sha256"],
             "retime recovery changed artifacts or was not load-only")
    if not legacy:
        need(HEX.fullmatch(instrument or "") and HEX.fullmatch(source_commit or ""),
             "phase provenance inputs missing")
        need(all(row.get("instrument_commit") == instrument and
                 row.get("source_commit") == source_commit for row in phases),
             "phase instrument/source identity drift")
        need(all((row.get("binary_sha256") == binary_sha256 if binary_sha256 else
                  not row.get("binary_sha256")) for row in phases
                 if row["phase"] in ("build-indexbench", "grade-holdout", "retime-three")),
             "phase binary identity drift")
    if retime_recovery:
        if retime_input_sha256 is None:
            return False  # Incomplete inputs cannot attest a recovery.
        need(phases[-2]["retime_input_sha256"] == retime_input_sha256,
             "retime recovery digest differs from actual frozen inputs")
    effective = phases[:-2] + phases[-1:] if retime_recovery else phases
    return all(row["exit"] == 0 and not row["timed_out"] and row["launch_load"] <= 9
               for row in effective)


def public_phases(phases):
    return [{"phase": row["phase"], "exit": row["exit"],
             "timed_out": row["timed_out"], "launch_load": row["launch_load"]}
            for row in phases]


def public_shape(value):
    allowed = {"Perl holdout count/hash receipt", *PHASES,
               "install-declared-dependencies",
               "spent", "meta", "ranges", "reading", "phases", "measured",
               "primary", "retime", "lock", "archive_state",
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
    retime_digest = retime_input_digest(manifest["digest"], private["primary_sha256"],
                                        private["ranges_sha256"], private["binary_sha256"],
                                        reading)
    phase_ok = phase_labels(phases, install, instrument, source_commit,
                            private["binary_sha256"], legacy,
                            retime_input_sha256=retime_digest)
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
        "phase_outcomes": public_phases(phases),
        "criteria": {**gates, "all_preregistered_excellence_gates": all(gates.values()),
                     "phase_execution_and_load_pass": phase_ok,
                     **coverage_criteria(compiled, units, harness_units, paired, tags,
                                         joined["ctags_unknown_files"], coverage_floor),
                     "coverage_and_existing_gates_pass": coverage_ok and all(gates.values()) and phase_ok},
    }
    public_shape(result)
    return result


def partial_receipt(private, paths, scratch, runner, instrument, subject,
                    source_commit, coverage_floor):
    spent = private["spent"]
    need(HEX.fullmatch(instrument) and spent["instrument_commit"] == instrument and
         spent["source_commit"] == source_commit and spent["spent_before"] == "clone-once" and
         spent["spent_at_utc"], "partial spend/identity mismatch")
    phases = private.get("phases")
    phase_identity_bound = bool(phases)
    phases = phases or []
    installed = any(row["phase"] == "install-declared-dependencies" for row in phases)
    binary_hash = digest(paths["binary"]) if paths["binary"].is_file() else None
    labels = [row["phase"] for row in phases]
    snapshot = None
    source_dir = scratch / (subject + "-source")
    checkout = next((row for row in phases if row["phase"] == "checkout-pin"), None)
    if checkout is not None and checkout["exit"] == 0:
        need(source_dir.is_dir(), "successful checkout lacks source snapshot")
        snapshot = runner.snapshot(source_dir)
        need(snapshot["commit"] == source_commit, "partial source pin mismatch")
    elif checkout is None and source_dir.is_dir() and (source_dir / ".git").exists():
        try:
            snapshot = runner.snapshot(source_dir)
        except (ValueError, subprocess.CalledProcessError, OSError):
            snapshot = None  # Clone may exist before checkout; identity is unknown.
        if snapshot is not None:
            need(snapshot["commit"] == source_commit, "partial source pin mismatch")
    lock = private.get("lock")
    if lock is not None:
        need(lock["format"] == "perl-dependency-lock-v1" and lock["subject"] == subject and
             lock["local_lib_perl5"] == private["current_library_fingerprint"],
             "partial own-dependency lock mismatch")
        if snapshot is not None:
            declaration = lock["declared_dependencies"]
            declaration_path = scratch / (subject + "-source") / declaration["path"]
            need(declaration["path"] in ("cpanfile", "Makefile.PL", "Build.PL", "META.json", "META.yml") and
                 declaration_path.is_file() and digest(declaration_path) == declaration["sha256"],
                 "partial dependency declaration drift")
    primary = private.get("primary")
    if primary is not None:
        need(primary["tool"] == "ppi-calls" and primary["version"] == "PPI 1.270" and
             primary["tier"] == "C", "partial primary oracle identity mismatch")
    meta, ranges = private.get("meta"), private.get("ranges")
    units = compiled = unresolved = tags = paired = ctags_unknown = None
    reasons = {}
    if meta is not None:
        manifest = meta["source_manifest"]
        entries = manifest["files"]
        encoded = json.dumps(entries, sort_keys=True, separators=(",", ":")).encode()
        need(snapshot == manifest and manifest["commit"] == source_commit and
             manifest["digest"] == hashlib.sha256(encoded).hexdigest() and
             meta["subject"] == subject, "partial native source identity mismatch")
        units = len(entries)
        compiled, unresolved = meta["compiled_files"], meta["unresolved_files"]
        need(compiled + unresolved == units and len(meta["failures"]) == unresolved and
             len(meta["joining"]["files"]) == units, "partial native unit mismatch")
        network = meta["network_policy"]["control"]
        need(network["denial"] == "EPERM" and
             network["destination"] == "127.0.0.1:1/tcp" and
             network["profile"] == "(version 1) (allow default) (deny network*)",
             "partial compiler network denial mismatch")
        need(SHA.fullmatch(meta.get("perl_binary_sha256", "")) and
             meta["ctags"].get("version", "").startswith("Universal Ctags 6.2.1,") and
             SHA.fullmatch(meta["ctags"].get("binary_sha256", "")),
             "partial compiler tool identity mismatch")
        dep = meta["dependency_context"]
        need(dep["local_lib_perl5_fingerprint_before"] ==
             dep["local_lib_perl5_fingerprint_after"], "partial dependency fingerprint changed")
        need(dep["lock_sha256"] == private.get("lock_sha256") and
             (lock is None or dep["local_lib_perl5_fingerprint_before"] ==
              private["current_library_fingerprint"]),
             "partial dependency lock/fingerprint mismatch")
        joined = meta["joining"]
        tags, paired, ctags_unknown = (joined["tag_total"], joined["paired"],
                                        joined["ctags_unknown_files"])
        need(ctags_unknown == sum(row["ctags_status"] == "unknown"
                                   for row in joined["files"]) and
             joined["compiler_unknown_files"] == unresolved and
             paired + joined["tag_unpaired"] == tags,
             "partial tag/compiler population mismatch")
        reasons = dict(sorted(Counter(reason for row in meta["failures"]
                                      for reason in row["reasons"]).items()))
        need(set(reasons) <= {"ctags_error_or_timeout", "compile_error", "file_timeout",
                              "total_timeout_waiting_for_load", "total_timeout_unattempted",
                              "missing_observer_output", "invalid_observer_output", "output_limit"},
             "partial compiler failure reason mismatch")
    elif snapshot is not None:
        units = len(snapshot["files"])
    if ranges is not None:
        need(ranges["tool"] == "perl-DB-sub-compile" and
             ranges["version"] == "v5.34.1" and ranges["tier"] == "A" and
             (units is None or ranges["units"] == units) and
             (unresolved is None or ranges["errored"] == unresolved) and
             (paired is None or len(ranges["ranges"]) == paired),
             "partial range companion mismatch")
    reading = private.get("reading")
    harness_units = eligible = ok = None
    if reading is not None:
        need(reading["fathom_commit"] == instrument and len(reading["subjects"]) == 1 and
             reading["subjects"][0]["name"] == subject and
             reading["subjects"][0]["sha"] == source_commit,
             "partial reading identity mismatch")
        harness_units = reading["subjects"][0]["files"]
        eligible, ok = reading["counts"]["range_eligible"], reading["counts"]["range_ok"]
        need(0 <= ok <= eligible and (paired is None or eligible <= paired),
             "partial reading range counts mismatch")
    measured = private.get("measured")
    if measured is not None:
        need(measured["holdout_source_commit"] == source_commit and
             measured["instrument_commit"] == instrument and
             (snapshot is None or measured["source_manifest_sha256"] == snapshot["digest"]),
             "partial measured identity mismatch")
    actual_retime_digest = None
    if snapshot is not None and primary is not None and ranges is not None and \
            reading is not None and binary_hash is not None:
        actual_retime_digest = retime_input_digest(snapshot["digest"],
            digest(paths["primary"]), digest(paths["ranges"]), binary_hash, reading)
    phase_ok = (phase_labels(phases, installed, instrument, source_commit,
                            binary_hash, legacy=False, complete=False,
                            retime_input_sha256=actual_retime_digest)
                if phase_identity_bound else False)
    return {
        "tool": "Perl holdout count/hash receipt", "partial": True,
        "source_commit": source_commit, "instrument_commit": instrument,
        "spent_before": "clone-once", "spent_at_utc": spent["spent_at_utc"],
        "source_manifest_sha256": snapshot["digest"] if snapshot else None,
        "units": units, "harness_files": harness_units,
        "native_minus_harness_files": (units - harness_units if units is not None and
                                       harness_units is not None else None),
        "compiled": compiled, "unresolved": unresolved,
        "named_tags": tags, "paired": paired, "ctags_unknown_files": ctags_unknown,
        "failure_reason_counts": reasons,
        "range_eligible": eligible, "range_ok": ok,
        "indexbench_binary_sha256": binary_hash,
        "dependency_lock_sha256": private.get("lock_sha256"),
        "local_library_sha256": (private.get("current_library_fingerprint") or {}).get("sha256"),
        "phase_identity_bound": phase_identity_bound,
        "phase_outcomes": public_phases(phases) if phase_identity_bound else None,
        "artifact_failure_count": len(private.get("invalid_artifacts", [])),
        "invalid_artifacts": sorted(private.get("invalid_artifacts", [])),
        "criteria": {"phase_execution_and_load_pass": phase_ok and len(labels) == len(PHASES) + installed,
                     "coverage_floors_preregistered": coverage_floor is not None,
                     "coverage_and_existing_gates_pass": False},
    }


def collect(scratch, repo, instrument, subject="Dancer2", source_commit=PIN,
            coverage_floor=None):
    spec = importlib.util.spec_from_file_location(
        "perl_holdout_runner", repo / "calibration/oracles/perl-db-ranges/runner.py")
    need(spec and spec.loader, "range runner unavailable")
    sys.path.insert(0, str(repo / "calibration/oracles/perl-db-ranges"))
    runner = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(runner)
    paths = {"spent": scratch / "holdout-spent.json",
             "meta": scratch / (subject + ".ranges.meta.json"),
             "ranges": scratch / (subject + ".ranges.json"),
             "reading": scratch / "reading.json", "phases": scratch / "phases.json",
             "measured": scratch / "measured.private.json", "primary": scratch / (subject + ".json"),
             "retime": scratch / "retime-three.log", "binary": scratch / "indexbench"}
    need(paths["spent"].is_file(), "spent marker missing")
    private, invalid_artifacts = {}, []
    for key, path in paths.items():
        if key in ("retime", "binary") or not path.is_file():
            continue
        try:
            value = load(path)
        except (json.JSONDecodeError, UnicodeDecodeError):
            value = None
        available_identity(key, value, instrument, source_commit, subject)
        if value is None or not artifact_shape(key, value):
            invalid_artifacts.append(key)
        else:
            private[key] = value
    need("spent" in private, "spent marker missing or malformed")
    private["invalid_artifacts"] = invalid_artifacts
    if paths["retime"].is_file():
        try:
            retime_runs = re.findall(r"^run ([0-9]+): ([0-9]+\.[0-9]+) MB/CPU-s$",
                                     paths["retime"].read_text(), re.M)
        except UnicodeDecodeError:
            retime_runs = []
        if [label for label, _ in retime_runs] != ["1", "2", "3"]:
            invalid_artifacts.append("retime")
    complete = all(path.is_file() for path in paths.values()) and not invalid_artifacts
    if complete:
        private["retime_runs"] = retime_runs
        private.update({"binary_sha256": digest(paths["binary"]),
                        "primary_sha256": digest(paths["primary"]),
                        "ranges_sha256": digest(paths["ranges"]),
                        "reading_sha256": digest(paths["reading"])})
    lock_path = scratch / "dependency-lock.json"
    if lock_path.is_file():
        try:
            lock_value = load(lock_path)
        except (json.JSONDecodeError, UnicodeDecodeError):
            invalid_artifacts.append("lock")
        else:
            if isinstance(lock_value, dict) and "subject" in lock_value:
                need(lock_value["subject"] == subject, "dependency lock subject drift")
            if isinstance(lock_value, dict) and all(key in lock_value for key in
                ("format", "subject", "local_lib_perl5", "declared_dependencies")) and \
                    isinstance(lock_value["declared_dependencies"], dict) and \
                    all(key in lock_value["declared_dependencies"]
                        for key in ("path", "sha256")):
                private["lock"] = lock_value
            else:
                invalid_artifacts.append("lock")
        private["lock_sha256"] = digest(lock_path)
        if "lock" in private:
            from importlib.machinery import SourceFileLoader
            dep_module = SourceFileLoader(
                "perl_holdout_dependencies",
                str(repo / "calibration/oracles/perl-db-ranges/dependencies.py")).load_module()
            private["current_library_fingerprint"] = dep_module.library_fingerprint(
                scratch / "local-perl/lib/perl5")
        archive_path = scratch / "dependency-archives.private.json"
        if archive_path.is_file():
            try:
                archive_state = load(archive_path)
            except (json.JSONDecodeError, UnicodeDecodeError):
                invalid_artifacts.append("archive_state")
            else:
                if not isinstance(archive_state, dict) or not all(
                    key in archive_state for key in ("local_lib_perl5", "saved_distributions")):
                    invalid_artifacts.append("archive_state")
                elif "current_library_fingerprint" in private:
                    need(archive_state["local_lib_perl5"] == private["current_library_fingerprint"],
                         "saved archive metadata differs from local library")
                    private["archive_state"] = archive_state
                else:
                    private["archive_state"] = archive_state
        else:
            need(not complete, "complete dependency run lacks saved archives")
    elif complete:
        need(not private["measured"]["dependency_install_attempted"],
             "installed dependencies lack lock")
    if invalid_artifacts:
        complete = False
    if complete:
        snapshot = runner.snapshot(scratch / (subject + "-source"))
        result = validate(private, snapshot, instrument, subject, source_commit, coverage_floor)
    else:
        result = partial_receipt(private, paths, scratch, runner, instrument,
                                 subject, source_commit, coverage_floor)
    result["collector_sha256"] = digest(__file__)
    result["frozen_instrument_file_sha256"] = frozen_file_hashes(repo, instrument)
    result["input_sha256"] = {("indexbench" if key == "binary" else key) + ".sha256":
                               digest(path) for key, path in paths.items() if path.is_file()}
    if "artifact_failure_count" not in result:
        result["artifact_failure_count"] = len(invalid_artifacts)
        result["invalid_artifacts"] = sorted(invalid_artifacts)
    if lock_path.is_file():
        result["input_sha256"]["dependency-lock.sha256"] = private["lock_sha256"]
    if (scratch / "dependency-archives.private.json").is_file():
        result["input_sha256"]["dependency-archives.sha256"] = digest(
            scratch / "dependency-archives.private.json")
    if "archive_state" in private:
        archive_rows = private["archive_state"]["saved_distributions"]
        need(all(re.fullmatch(r"[A-Za-z0-9._+-]+\.(?:tar\.gz|tgz|zip)", row["basename"]) and
                 SHA.fullmatch(row["sha256"]) and row["bytes"] >= 0 for row in archive_rows),
             "unsafe dependency archive metadata")
        need(len(archive_rows) == len({row["basename"] for row in archive_rows}),
             "duplicate dependency archive basename")
        result["dependency_archives"] = [
            {"name": row["basename"], "sha256": row["sha256"], "bytes": row["bytes"]}
            for row in sorted(archive_rows, key=lambda row: row["basename"])]
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
    phases = [{"phase": label, "exit": 0, "timed_out": False, "launch_load": 1,
               "seconds": 1}
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
    for index, changes in ((5, {"exit": 2}),
                           (3, {"exit": -15, "timed_out": True}),
                           (6, {"launch_load": 10})):
        failed = [dict(row) for row in future]
        failed[index].update(changes)
        need(not phase_labels(failed, False, "a" * 40, "b" * 40, "c" * 64),
             "failed phase accepted as passing")
        public_shape({"phase_outcomes": public_phases(failed),
                      "criteria": {"phase_execution_and_load_pass": False,
                                   "coverage_and_existing_gates_pass": False}})
    retry = [dict(row) for row in future]
    retry[-1].update(launch_load=10, retime_input_sha256="e" * 64)
    retry.append({**future[-1], "retime_input_sha256": "e" * 64})
    need(phase_labels(retry, False, "a" * 40, "b" * 40, "c" * 64,
                      retime_input_sha256="e" * 64),
         "unchanged load-only retime recovery rejected")
    retry[-1]["retime_input_sha256"] = "f" * 64
    try:
        phase_labels(retry, False, "a" * 40, "b" * 40, "c" * 64,
                     retime_input_sha256="e" * 64)
    except ValueError:
        pass
    else:
        raise AssertionError("changed retime artifacts accepted")
    reading = {"subjects": [{"files": 2, "counts": {"range_ok": 1},
                              "tenancy": {"extract_ms": 1}}],
               "counts": {"range_ok": 1}, "metrics": {"range_agreement": 1},
               "notes": [], "grade": {"L1": "good"}}
    actual = retime_input_digest("a" * 64, "b" * 64, "c" * 64, "d" * 64, reading)
    for changed in (
            ("a" * 64, "f" * 64, "c" * 64, "d" * 64, reading),
            ("a" * 64, "b" * 64, "f" * 64, "d" * 64, reading),
            ("a" * 64, "b" * 64, "c" * 64, "f" * 64, reading),
            ("a" * 64, "b" * 64, "c" * 64, "d" * 64,
             {**reading, "counts": {"range_ok": 0}})):
        changed_digest = retime_input_digest(*changed)
        need(changed_digest != actual, "retime input mutation did not change digest")
        old_claims = [dict(row) for row in retry]
        old_claims[-2]["retime_input_sha256"] = actual
        old_claims[-1]["retime_input_sha256"] = actual
        try:
            phase_labels(old_claims, False, "a" * 40, "b" * 40, "c" * 64,
                         retime_input_sha256=changed_digest)
        except ValueError:
            pass
        else:
            raise AssertionError("stale retime claim accepted")
    workspace_tmp = Path(__file__).resolve().parents[3] / "workspace" / "tmp"
    workspace_tmp.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=workspace_tmp) as directory:
        scratch = Path(directory)
        (scratch / "holdout-spent.json").write_text(json.dumps({
            "instrument_commit": "a" * 40, "source_commit": "b" * 40,
            "spent_before": "clone-once", "spent_at_utc": "2026-10-26T00:00:00+00:00"}))
        failed_grade = [dict(row) for row in future[:6]]
        failed_grade[1]["exit"] = 1  # no checkout snapshot available
        failed_grade[-1]["exit"] = 2
        failed_grade[-2].pop("binary_sha256", None)
        failed_grade[-1].pop("binary_sha256", None)
        (scratch / "phases.json").write_text(json.dumps(failed_grade))
        (scratch / "reading.json").write_text('{"truncated":')
        (scratch / "Synthetic-source").mkdir()  # Clone exists; checkout failed.
        saved = globals()["frozen_file_hashes"]
        globals()["frozen_file_hashes"] = lambda _repo, _commit: {}
        try:
            receipt = collect(scratch, Path(__file__).resolve().parents[3],
                              "a" * 40, "Synthetic", "b" * 40, .95)
            need(receipt["artifact_failure_count"] == 1 and
                 receipt["input_sha256"]["reading.sha256"] == digest(scratch / "reading.json") and
                 receipt["range_ok"] is None and
                 receipt["criteria"]["coverage_and_existing_gates_pass"] is False,
                 "failed grade with truncated reading was not published")
            compiler_timeout = [dict(row) for row in future[:4]]
            compiler_timeout[1]["exit"] = 1
            compiler_timeout[-1].update(exit=-15, timed_out=True)
            (scratch / "phases.json").write_text(json.dumps(compiler_timeout))
            receipt = collect(scratch, Path(__file__).resolve().parents[3],
                              "a" * 40, "Synthetic", "b" * 40, .95)
            need(receipt["phase_outcomes"][-1]["timed_out"] is True and
                 receipt["compiled"] is None and
                 receipt["criteria"]["coverage_and_existing_gates_pass"] is False,
                 "compiler timeout was not published")
            (scratch / "phases.json").write_text(json.dumps(failed_grade))
            (scratch / "reading.json").write_text(json.dumps({"fathom_commit": "f" * 40}))
            try:
                collect(scratch, Path(__file__).resolve().parents[3],
                        "a" * 40, "Synthetic", "b" * 40, .95)
            except ValueError:
                pass
            else:
                raise AssertionError("readable partial reading identity drift accepted")
            (scratch / "reading.json").write_text(json.dumps({
                "subjects": [{"name": "Synthetic", "sha": "f" * 40}]}))
            try:
                collect(scratch, Path(__file__).resolve().parents[3],
                        "a" * 40, "Synthetic", "b" * 40, .95)
            except ValueError:
                pass
            else:
                raise AssertionError("readable nested source drift accepted")
            (scratch / "reading.json").write_text('{"truncated":')
            (scratch / "Synthetic.ranges.meta.json").write_text('{"truncated":')
            (scratch / "Synthetic.ranges.json").write_text(json.dumps({
                "tool": "perl-DB-sub-compile", "version": "v5.34.1", "tier": "A",
                "units": 1, "errored": 1, "ranges": []}))
            receipt = collect(scratch, Path(__file__).resolve().parents[3],
                              "a" * 40, "Synthetic", "b" * 40, .95)
            need(receipt["artifact_failure_count"] == 2 and receipt["compiled"] is None,
                 "readable ranges with truncated meta failed publication")
            failed_grade[0]["source_commit"] = "f" * 40
            (scratch / "phases.json").write_text(json.dumps(failed_grade))
            try:
                collect(scratch, Path(__file__).resolve().parents[3],
                        "a" * 40, "Synthetic", "b" * 40, .95)
            except ValueError:
                pass
            else:
                raise AssertionError("parseable wrong source identity accepted")
            (scratch / "phases.json").write_text('{"truncated":')
            receipt = collect(scratch, Path(__file__).resolve().parents[3],
                              "a" * 40, "Synthetic", "b" * 40, .95)
            need(receipt["phase_identity_bound"] is False and
                 receipt["source_manifest_sha256"] is None and
                 receipt["artifact_failure_count"] == 3,
                 "malformed phases falsely claimed provenance")
        finally:
            globals()["frozen_file_hashes"] = saved
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
            "phase_identity_drift": "rejected", "phase_failures": "published",
            "missing_artifacts": "unknown_counts"}


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
