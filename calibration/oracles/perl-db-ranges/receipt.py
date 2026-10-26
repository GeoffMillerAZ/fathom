#!/usr/bin/env python3
"""Project private C-R9 measurements into a count/hash-only public receipt."""

import argparse
import copy
import hashlib
import json
from pathlib import Path
import re
import statistics

HERE = Path(__file__).resolve().parent
SUBJECTS = ("Perl-Critic", "mojo")
HEX = re.compile(r"^[0-9a-f]{40}$")
SHA = re.compile(r"^[0-9a-f]{64}$")
CTAGS = "Universal Ctags 6.2.1"
XREF_HASH = "adbb5535cb333dc3d78025f95618d218e92015d880bee559ceb6277cd0042539"
SAFE_TEXT = {"C-R9 Perl native count/hash receipt", "Perl-Critic", "mojo", CTAGS,
             "EPERM", "L1", "L2", "L3", "L4", "A", "B", "C",
             "excellent", "good", "limited", "poor", "not-attempted"}


def require(ok, message):
    if not ok:
        raise ValueError(message)


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def load(path):
    return json.loads(Path(path).read_text())


def ratio(top, bottom):
    return round(top / bottom, 6) if bottom else None


def safe_projection(value):
    if isinstance(value, dict):
        require(all(isinstance(key, str) and re.fullmatch(r"[A-Za-z0-9_.-]+", key)
                    for key in value), "unsafe public key")
        for child in value.values():
            safe_projection(child)
    elif isinstance(value, list):
        for child in value:
            safe_projection(child)
    else:
        require(value is None or isinstance(value, (str, int, float, bool)),
                "non-scalar public value")
        if isinstance(value, str):
            require(value in SAFE_TEXT or HEX.fullmatch(value) or SHA.fullmatch(value),
                    "unsafe public text")


def network_ok(control):
    return (control.get("denial") == "EPERM" and
            control.get("destination") == "127.0.0.1:1/tcp" and
            control.get("profile") == "(version 1) (allow default) (deny network*)")


def subject_receipt(name, meta, ranges, xref, reading_subject):
    manifest = meta["source_manifest"]
    joining = meta["joining"]
    dependency = meta["dependency_context"]
    units = len(manifest["files"])
    commit, manifest_hash = manifest["commit"], manifest["digest"]
    require(name in SUBJECTS and meta["subject"] == name and xref["subject"] == name,
            "subject mismatch")
    require(HEX.fullmatch(commit) and SHA.fullmatch(manifest_hash), "invalid source identity")
    encoded_manifest = json.dumps(manifest["files"], sort_keys=True,
                                  separators=(",", ":")).encode()
    require(hashlib.sha256(encoded_manifest).hexdigest() == manifest_hash,
            "source manifest digest mismatch")
    require(reading_subject["name"] == name and reading_subject["sha"] == commit,
            "reading source mismatch")
    require(reading_subject["files"] == units == ranges["units"] == xref["units"] == len(joining["files"]),
            "inconsistent units")
    require(xref["source_commit"] == commit and xref["source_manifest_sha256"] == manifest_hash,
            "xref source mismatch")
    require(meta["compiled_files"] + meta["unresolved_files"] == units and
            ranges["errored"] == meta["unresolved_files"] and
            xref["compiled"] + xref["compile_failures_or_timeouts"] == units,
            "inconsistent compile/unresolved population")
    require(sum(row["compiler_status"] == "ok" for row in joining["files"]) == meta["compiled_files"] and
            len(meta["failures"]) == meta["unresolved_files"],
            "compiler status mismatch")
    require(network_ok(meta["network_policy"]["control"]) and network_ok(xref["network_control"]),
            "network denial unproved")
    require(dependency["local_lib_perl5_fingerprint_before"] ==
            dependency["local_lib_perl5_fingerprint_after"] ==
            xref["local_library_fingerprint_before"] == xref["local_library_fingerprint_after"],
            "local dependency fingerprint changed")
    require(dependency["lock_sha256"] == xref["dependency_lock_sha256"],
            "dependency lock mismatch")
    require(meta["perl_binary_sha256"] == xref["perl_interpreter_sha256"] and
            ranges["version"] == xref["perl_version"] == "v5.34.1", "Perl identity mismatch")
    module = xref["backend_module"]
    require(xref["tool"] == "stock Perl B::Xref census" and xref["version"] == "1.07" and
            module["sha256"] == XREF_HASH and module["relative_path"].endswith("/B/Xref.pm"),
            "B::Xref module identity mismatch")
    require(meta["ctags"]["version"].startswith(CTAGS + ",") and
            SHA.fullmatch(meta["ctags"]["binary_sha256"]), "Ctags identity mismatch")
    paired, tags = joining["paired"], joining["tag_total"]
    require(paired == len(ranges["ranges"]) == reading_subject["counts"]["range_eligible"] ==
            reading_subject["counts"]["range_ok"] and paired <= tags,
            "paired/read range count mismatch")
    require(joining["tag_unpaired"] + paired == tags, "tag accounting mismatch")
    require(xref["method_target_emitted_records"] == 0, "stock Xref method target unexpected")
    return {
        "subject": name, "source_commit": commit, "source_manifest_sha256": manifest_hash,
        "units": units, "range_compiled": meta["compiled_files"],
        "range_unresolved": meta["unresolved_files"],
        "range_compiled_coverage": ratio(meta["compiled_files"], units),
        "xref_compiled": xref["compiled"],
        "xref_unresolved": xref["compile_failures_or_timeouts"],
        "native_ranges": len(joining["native"]), "tags": tags, "paired": paired,
        "paired_tag_coverage": ratio(paired, tags),
        "reading_range_eligible": reading_subject["counts"]["range_eligible"],
        "reading_range_ok": reading_subject["counts"]["range_ok"],
        "reading_range_agreement": ratio(reading_subject["counts"]["range_ok"],
                                         reading_subject["counts"]["range_eligible"]),
        "xref_direct_call_records": xref["direct_call_records"],
        "xref_unknown_target_records": xref["non_direct_target_unknown_records"],
        "xref_method_target_records": xref["method_target_emitted_records"],
        "perl_binary_sha256": meta["perl_binary_sha256"],
        "xref_module_sha256": module["sha256"],
        "ctags_binary_sha256": meta["ctags"]["binary_sha256"],
        "ctags_version": CTAGS,
        "ctags_raw_version_sha256": hashlib.sha256(meta["ctags"]["version"].encode()).hexdigest(),
        "local_library_sha256": (dependency["local_lib_perl5_fingerprint_before"] or {}).get("sha256"),
        "dependency_lock_sha256": dependency["lock_sha256"],
        "network_denial": "EPERM",
    }


def validate_reading(reading, baseline):
    require(reading["dialect"] == baseline["dialect"] == "perl", "reading dialect mismatch")
    for key in baseline["counts"]:
        require(reading["counts"].get(key) == baseline["counts"][key],
                "nonrange count regression: " + key)
    require(set(reading["counts"]) - set(baseline["counts"]) == {"range_eligible", "range_ok"},
            "unexpected reading counts")
    for key in baseline["metrics"]:
        if key != "range_agreement":
            require(reading["metrics"].get(key) == baseline["metrics"][key],
                    "nonrange metric regression: " + key)
    old = {row["name"]: row for row in baseline["subjects"]}
    new = {row["name"]: row for row in reading["subjects"]}
    require(set(old) == set(new) == set(SUBJECTS), "reading subjects mismatch")
    for name in SUBJECTS:
        require(new[name]["sha"] == old[name]["sha"] and new[name]["files"] == old[name]["files"],
                "subject source regression")
        for key, value in old[name]["counts"].items():
            require(new[name]["counts"].get(key) == value, "subject nonrange regression: " + key)
        require(set(new[name]["counts"]) - set(old[name]["counts"]) ==
                {"range_eligible", "range_ok"}, "unexpected subject counts")
    require(reading["counts"]["range_eligible"] == sum(new[n]["counts"]["range_eligible"] for n in SUBJECTS) and
            reading["counts"]["range_ok"] == sum(new[n]["counts"]["range_ok"] for n in SUBJECTS),
            "aggregate range count mismatch")
    return new


def retime_values(text):
    rows = re.findall(r"^run ([0-9]+): ([0-9]+\.[0-9]+) MB/CPU-s$", text, re.M)
    require([label for label, _ in rows] == ["1", "2", "3"], "timing labels must be runs 1, 2, 3 exactly once")
    return [float(value) for _, value in rows]


def validate_phases(phases, grade_phases, native_commit, harness_commit):
    require(all(row["exit"] == 0 and row["instrument_commit"] == native_commit for row in phases),
            "native phase failed or instrument changed")
    require(all(row["exit"] == 0 and row["instrument_commit"] == harness_commit for row in grade_phases),
            "grade phase failed or harness changed")
    require(all(row["launch_load"] <= 9 for row in phases + grade_phases),
            "phase launched above load budget")
    require(len(phases) == 5 and len(grade_phases) == 4, "phase population mismatch")
    require([row["phase"] for row in grade_phases] ==
            ["build-indexbench", "grade-mojo", "grade-Perl-Critic", "retime"],
            "grade phase labels mismatch")
    binary_hashes = [row["binary_sha256"] for row in grade_phases]
    require(all(SHA.fullmatch(value) for value in binary_hashes) and
            len(set(binary_hashes)) == 1, "grade binary identity mismatch")


def timing(grade_dir, phases, grade_phases, native_commit, harness_commit):
    validate_phases(phases, grade_phases, native_commit, harness_commit)
    values = retime_values((grade_dir / "retime.log").read_text())
    return {"three_runs_mb_per_cpu_second": values,
            "median_mb_per_cpu_second": statistics.median(values),
            "native_phase_seconds": [row["seconds"] for row in phases],
            "native_phase_launch_load": [row["launch_load"] for row in phases],
            "grade_phase_seconds": [row["seconds"] for row in grade_phases],
            "grade_phase_launch_load": [row["launch_load"] for row in grade_phases],
            "harness_binary_sha256": grade_phases[0]["binary_sha256"]}


def collect(input_dir, grade_dir, reading_path, baseline_path, instrument_commit, harness_commit):
    require(HEX.fullmatch(instrument_commit), "instrument commit must be full hex")
    require(HEX.fullmatch(harness_commit), "harness commit must be full hex")
    reading, baseline = load(reading_path), load(baseline_path)
    require(reading["fathom_commit"] == harness_commit, "harness commit mismatch")
    by_name = validate_reading(reading, baseline)
    subjects, hashes = [], {}
    for name in SUBJECTS:
        files = {suffix: input_dir / (name + suffix) for suffix in
                 (".ranges.meta.json", ".ranges.json", ".xref.json")}
        subjects.append(subject_receipt(name, load(files[".ranges.meta.json"]),
                                        load(files[".ranges.json"]), load(files[".xref.json"]),
                                        by_name[name]))
        hashes.update({name + suffix: digest(path) for suffix, path in files.items()})
    phase_files = {"phases.json": input_dir / "phases.json",
                   "grade-phases.json": grade_dir / "grade-phases.json",
                   "retime.log": grade_dir / "retime.log",
                   "reading.json": reading_path, "baseline.json": baseline_path}
    grade_meta_path = grade_dir / "grade.meta.json"
    require(grade_dir == input_dir or grade_meta_path.is_file(),
            "separate grade requires grade.meta.json")
    if grade_meta_path.is_file():
        grade_meta = load(grade_meta_path)
        require(grade_meta["native_instrument_commit"] == instrument_commit and
                grade_meta["harness_commit"] == harness_commit and
                grade_meta["reading_sha256"] == digest(reading_path) and
                grade_meta["grade_phases_sha256"] == digest(phase_files["grade-phases.json"]) and
                grade_meta["binary_sha256"] == load(phase_files["grade-phases.json"])[0]["binary_sha256"],
                "grade metadata provenance mismatch")
        expected_native_keys = {"phases.json"} | {
            name + suffix for name in SUBJECTS for suffix in
            (".json", ".ranges.json", ".ranges.meta.json")}
        require(set(grade_meta["input_sha256"]) == expected_native_keys and
                all(digest(input_dir / key) == value
                    for key, value in grade_meta["input_sha256"].items()),
                "native input changed after grade")
        require(all(grade_meta["source_manifests"][row["subject"]]["commit"] == row["source_commit"] and
                    grade_meta["source_manifests"][row["subject"]]["digest"] ==
                    row["source_manifest_sha256"] for row in subjects),
                "grade source manifest mismatch")
        phase_files["grade.meta.json"] = grade_meta_path
    hashes.update({key: digest(value) for key, value in phase_files.items()})
    receipt = {"tool": "C-R9 Perl native count/hash receipt", "instrument_commit": instrument_commit,
            "harness_commit": harness_commit,
            "collector_sha256": digest(__file__), "inputs_sha256": dict(sorted(hashes.items())),
            "subjects": subjects, "grade": {key: reading["grade"][key] for key in ("L1", "L2", "L3", "L4")},
            "level_met": reading["level_met"],
            "metric_tiers": {key: reading["metric_tiers"][key] for key in sorted(reading["metric_tiers"])},
            "range_eligible": reading["counts"]["range_eligible"],
            "range_ok": reading["counts"]["range_ok"],
            "range_agreement": reading["metrics"]["range_agreement"],
            "timing": timing(grade_dir, load(phase_files["phases.json"]),
                             load(phase_files["grade-phases.json"]), instrument_commit, harness_commit)}
    safe_projection(receipt)
    return receipt


def self_test():
    # The private fields must never be copied to the public projection.
    private_manifest = [{"path": "Author Person/private.pm", "sha256": "c" * 64}]
    manifest_hash = hashlib.sha256(json.dumps(private_manifest, sort_keys=True,
                                              separators=(",", ":")).encode()).hexdigest()
    example = {"subject": "mojo", "source_manifest": {"commit": "a" * 40,
               "digest": manifest_hash, "files": private_manifest},
               "compiled_files": 1, "unresolved_files": 0, "failures": [],
               "joining": {"files": [{"compiler_status": "ok", "path": "Author Person/private.pm"}],
                           "native": [{"path": "Author Person/private.pm", "name": "Author Person"}],
                           "paired": 1, "tag_total": 1, "tag_unpaired": 0},
               "network_policy": {"control": {"denial": "EPERM", "destination": "127.0.0.1:1/tcp",
                                              "profile": "(version 1) (allow default) (deny network*)"}},
               "dependency_context": {"local_lib_perl5_fingerprint_before": None,
                                      "local_lib_perl5_fingerprint_after": None, "lock_sha256": None},
               "perl_binary_sha256": "d" * 64,
               "ctags": {"version": CTAGS + ", Copyright Author Person", "binary_sha256": "e" * 64}}
    ranges = {"units": 1, "errored": 0, "version": "v5.34.1", "ranges": [{"path": "Author Person/private.pm"}]}
    xref = {"subject": "mojo", "units": 1, "compiled": 1, "compile_failures_or_timeouts": 0,
            "source_commit": "a" * 40, "source_manifest_sha256": manifest_hash,
            "network_control": example["network_policy"]["control"],
            "local_library_fingerprint_before": None, "local_library_fingerprint_after": None,
            "dependency_lock_sha256": None, "perl_interpreter_sha256": "d" * 64,
            "perl_version": "v5.34.1", "tool": "stock Perl B::Xref census", "version": "1.07",
            "backend_module": {"sha256": XREF_HASH, "relative_path": "arch/B/Xref.pm"},
            "method_target_emitted_records": 0, "direct_call_records": 1,
            "non_direct_target_unknown_records": 1}
    subject = {"name": "mojo", "sha": "a" * 40, "files": 1,
               "counts": {"range_eligible": 1, "range_ok": 1}}
    projected = subject_receipt("mojo", example, ranges, xref, subject)
    safe_projection(projected)
    require("Author Person" not in json.dumps(projected), "private field leaked")
    mutations = ((example, "compiled_files", 0), (xref, "source_manifest_sha256", "f" * 64),
                 (xref, "local_library_fingerprint_after", {"sha256": "f" * 64}),
                 (xref["network_control"], "denial", "EACCES"))
    for original, key, value in mutations:
        old = original[key]
        original[key] = value
        try:
            subject_receipt("mojo", example, ranges, xref, subject)
        except ValueError:
            pass
        else:
            raise AssertionError("failed to reject " + key)
        finally:
            original[key] = old
    base = {"dialect": "perl", "fathom_commit": "a" * 40,
            "counts": {"call_truth": 2}, "metrics": {"call_recall": 1, "range_agreement": None},
            "subjects": [{"name": "mojo", "sha": "a" * 40, "files": 1,
                          "counts": {"call_truth": 2}}]}
    # Extend to the required second subject, then mutate one non-range count.
    base["subjects"].append({"name": "Perl-Critic", "sha": "b" * 40, "files": 1,
                             "counts": {"call_truth": 0}})
    actual = copy.deepcopy(base)
    actual["counts"].update(range_eligible=1, range_ok=1)
    for row in actual["subjects"]:
        row["counts"].update(range_eligible=int(row["name"] == "mojo"),
                             range_ok=int(row["name"] == "mojo"))
    actual["counts"]["call_truth"] = 3
    try:
        validate_reading(actual, base)
    except ValueError:
        pass
    else:
        raise AssertionError("failed to reject nonrange regression")
    for text in ("run 1: 1.00 MB/CPU-s\nrun 1: 1.00 MB/CPU-s\nrun 1: 1.00 MB/CPU-s\n",
                 "run 1: 1.00 MB/CPU-s\nrun 2: 1.00 MB/CPU-s\n"):
        try:
            retime_values(text)
        except ValueError:
            pass
        else:
            raise AssertionError("failed to reject malformed timing labels")
    require(retime_values("run 1: 1.00 MB/CPU-s\nrun 2: 2.00 MB/CPU-s\nrun 3: 3.00 MB/CPU-s\n") ==
            [1.0, 2.0, 3.0], "valid timing labels rejected")
    native = [{"exit": 0, "instrument_commit": "a" * 40, "launch_load": 1}] * 5
    grade = [{"exit": 0, "instrument_commit": "b" * 40, "launch_load": 1,
              "phase": label, "binary_sha256": "d" * 64} for label in
             ("build-indexbench", "grade-mojo", "grade-Perl-Critic", "retime")]
    try:
        validate_phases(native, grade, "a" * 40, "c" * 40)
    except ValueError:
        pass
    else:
        raise AssertionError("failed to reject grade harness mismatch")
    return {"private_projection": "pass", "inconsistent_units": "rejected",
            "source_hash": "rejected", "fingerprint": "rejected",
            "network": "rejected", "nonrange_regression": "rejected",
            "duplicate_timing_labels": "rejected", "grade_commit_mismatch": "rejected"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input-dir", type=Path)
    parser.add_argument("--grade-dir", type=Path)
    parser.add_argument("--reading", type=Path)
    parser.add_argument("--baseline", type=Path)
    parser.add_argument("--instrument-commit")
    parser.add_argument("--harness-commit")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        require(not any((args.input_dir, args.grade_dir, args.reading, args.baseline,
                         args.instrument_commit, args.harness_commit, args.output)),
                "self-test takes no inputs")
        print(json.dumps(self_test(), sort_keys=True))
        return
    if not all((args.input_dir, args.reading, args.baseline, args.instrument_commit, args.output)):
        parser.error("all input and output arguments are required")
    require(not args.output.exists(), "output must be new")
    receipt = collect(args.input_dir, args.grade_dir or args.input_dir, args.reading,
                      args.baseline, args.instrument_commit,
                      args.harness_commit or args.instrument_commit)
    args.output.write_text(json.dumps(receipt, sort_keys=True, indent=2) + "\n")
    print(json.dumps({"receipt_sha256": digest(args.output), "subjects": len(receipt["subjects"])},
                     sort_keys=True))


if __name__ == "__main__":
    main()
