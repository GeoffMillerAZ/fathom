#!/usr/bin/env python3
"""Sanitize one HLSL replay reading and emit an aggregate baseline diff."""

import argparse
import copy
import hashlib
import json
from pathlib import Path
import re


DXC_RELEASE = "v1.9.2602"
DXC_ASSET = "linux_dxc_2026_02_20.x86_64.tar.gz"

_RUNS_RE = re.compile(
    r"three runs over every measured file read "
    r"([0-9.]+), ([0-9.]+), ([0-9.]+) MB/s CPU")
_SCOPE_RE = re.compile(
    r"^([^:]+): dxc-ast .*?: ([0-9]+) of ([0-9]+) measured files in scope")


def _sha(value):
    encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":"),
                         sort_keys=True).encode("utf-8")
    return hashlib.sha256(encoded).hexdigest()


def _runs(data):
    for note in data.get("notes", []):
        match = _RUNS_RE.search(note)
        if match:
            return [float(value) for value in match.groups()]
    raise ValueError("reading has no three-run throughput receipt")


def _scope(data):
    rows = {}
    for note in data.get("notes", []):
        match = _SCOPE_RE.search(note)
        if match:
            name = match.group(1)
            if name in rows:
                raise ValueError(f"reading has duplicate scope note for {name}")
            rows[name] = (int(match.group(2)), int(match.group(3)))
    names = [row["name"] for row in data.get("subjects", [])]
    if set(rows) != set(names):
        raise ValueError("reading scope notes do not cover every subject")
    subjects = [{"name": name, "dxc_files": rows[name][0],
                 "measured_files": rows[name][1]} for name in names]
    return {
        "dxc_files": sum(row["dxc_files"] for row in subjects),
        "measured_files": sum(row["measured_files"] for row in subjects),
        "subjects": subjects,
    }


def _number(value, label):
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise ValueError(f"{label} is not numeric")
    return value


def _pins(data):
    return sorted((row["name"], row["sha"]) for row in data["subjects"])


def _fmt(values):
    return ", ".join(f"{value:g}" for value in values)


def sanitize_reading(data, source_commit):
    """Remove disclosure-bearing notes and path lists without changing metrics."""
    result = copy.deepcopy(data)
    raw_notes = list(result.get("notes", []))
    runs = _runs(data)
    scope = _scope(data)

    faulting_paths = []
    adjudicated_entries = []
    tenancy_blocks = [result.get("tenancy", {})]
    tenancy_blocks.extend(row.get("tenancy", {}) for row in result.get("subjects", []))
    fault_ordinal = 0
    for tenancy in tenancy_blocks:
        paths = tenancy.get("faulting_files")
        if paths is None:
            if tenancy.get("faults", 0) != 0:
                raise ValueError("tenancy faults do not match faulting-file entries")
            continue
        if not isinstance(paths, list) or tenancy.get("faults") != len(paths):
            raise ValueError("tenancy faults do not match faulting-file entries")
        faulting_paths.extend(paths)
        replacements = []
        for _ in paths:
            fault_ordinal += 1
            replacements.append(f"redacted-fault-{fault_ordinal:06d}")
        tenancy["faulting_files"] = replacements
    for subject in result.get("subjects", []):
        entries = subject.get("adjudicated")
        if entries is not None:
            if not isinstance(entries, list):
                raise ValueError("subject adjudicated evidence is not a list")
            adjudicated_entries.extend(entries)
            subject["adjudicated"] = []

    receipt = {
        "diagnostic_notes_removed": len(raw_notes),
        "diagnostic_notes_sha256": _sha(raw_notes),
        "faulting_file_entries_removed": len(faulting_paths),
        "faulting_file_entries_sha256": _sha(faulting_paths),
        "adjudicated_entries_removed": len(adjudicated_entries),
        "adjudicated_entries_sha256": _sha(adjudicated_entries),
    }
    result["notes"] = [
        ("public replay sanitization: removed "
         f"{receipt['diagnostic_notes_removed']} diagnostic notes "
         f"(sha256 {receipt['diagnostic_notes_sha256']}); retained no raw note text"),
        ("public replay sanitization: removed "
         f"{receipt['faulting_file_entries_removed']} faulting-file path entries "
         f"(sha256 {receipt['faulting_file_entries_sha256']}); replaced them with "
         "neutral ordinal placeholders and retained fault counts"),
        ("public replay sanitization: removed "
         f"{receipt['adjudicated_entries_removed']} adjudicated path/name entries "
         f"(sha256 {receipt['adjudicated_entries_sha256']}); retained no entry text"),
        ("public replay scope: DXC read "
         f"{scope['dxc_files']} of {scope['measured_files']} measured files"),
        ("public replay throughput: fresh driver runs were "
         f"{_fmt(runs)} MB/s CPU; the reading retains the selected median"),
        f"public replay source: fathom commit {source_commit}",
    ]
    return result, receipt


def _numeric_map(data, label):
    return {key: _number(value, f"{label}.{key}")
            for key, value in sorted(data.items())}


def _counter_delta(before, after):
    keys = sorted(set(before) | set(after))
    return {key: after.get(key, 0) - before.get(key, 0) for key in keys}


def _metric_comparison(before, after):
    keys = sorted(set(before) | set(after))
    old, new, delta, change = {}, {}, {}, {}
    missing = object()
    for key in keys:
        b = before.get(key, missing)
        a = after.get(key, missing)
        for value, side in ((b, "baseline"), (a, "current")):
            if value is not missing and value is not None:
                _number(value, f"{side} metric.{key}")
        old[key] = None if b is missing else b
        new[key] = None if a is missing else a
        if b is missing:
            delta[key], change[key] = None, "added"
        elif a is missing:
            delta[key], change[key] = None, "removed"
        elif b is None and a is not None:
            delta[key], change[key] = None, "became-measured"
        elif b is not None and a is None:
            delta[key], change[key] = None, "became-unmeasured"
        elif b is None:
            delta[key], change[key] = None, "unchanged-unmeasured"
        else:
            delta[key] = a - b
            change[key] = "changed" if a != b else "unchanged"
    return {"baseline": old, "current": new, "delta": delta, "change": change}


def _subject_counts(baseline, current):
    old = {row["name"]: row for row in baseline["subjects"]}
    result = []
    for row in current["subjects"]:
        name = row["name"]
        before = _numeric_map(old[name].get("counts", {}), f"baseline subject {name}")
        after = _numeric_map(row.get("counts", {}), f"current subject {name}")
        result.append({"name": name, "sha": row["sha"], "baseline": before,
                       "current": after, "delta": _counter_delta(before, after)})
    return result


def _scope_comparison(baseline, current):
    old = {row["name"]: row for row in baseline["subjects"]}
    subjects = []
    for row in current["subjects"]:
        before = old[row["name"]]
        subjects.append({
            "name": row["name"],
            "baseline": {"dxc_files": before["dxc_files"],
                         "measured_files": before["measured_files"]},
            "current": {"dxc_files": row["dxc_files"],
                        "measured_files": row["measured_files"]},
            "dxc_files_delta": row["dxc_files"] - before["dxc_files"],
            "measured_files_delta": row["measured_files"] - before["measured_files"],
        })
    return subjects


def _oracle_version(data, tool):
    matches = [row["version"] for row in data.get("oracles", [])
               if row.get("tool") == tool]
    if len(matches) != 1:
        raise ValueError(f"reading must name exactly one {tool} oracle")
    return matches[0]


def _active_workflow(mode, path, digest):
    if mode == "manual-no-active-workflow" and not path and not digest:
        return {"mode": mode, "path": None, "sha256": None}
    if (mode == "github-actions-verified-active-copy" and
            path == ".github/workflows/hlsl-scope-replay.yml" and
            re.fullmatch(r"[0-9a-f]{64}", digest or "")):
        return {"mode": mode, "path": path, "sha256": digest}
    raise ValueError("invalid active workflow identity")


def build_diff(baseline, current, redactions, *, source_commit, dxc_sha256,
               required_glibc, runner_glibc, go_version, controller_commit,
               replay_sha256, publish_sha256, workflow_sha256,
               active_workflow_mode, active_workflow_path,
               active_workflow_sha256):
    if _pins(baseline) != _pins(current):
        raise ValueError("baseline and current subject pins differ")
    current_revision = current.get("fathom_commit")
    if (not isinstance(current_revision, str) or
            not re.fullmatch(r"[0-9a-f]{7,40}", current_revision) or
            not source_commit.startswith(current_revision)):
        raise ValueError("current reading does not identify the replay source commit")

    baseline_counts = _numeric_map(baseline["counts"], "baseline counts")
    current_counts = _numeric_map(current["counts"], "current counts")
    count_delta = _counter_delta(baseline_counts, current_counts)
    metrics = _metric_comparison(baseline["metrics"], current["metrics"])
    baseline_scope = _scope(baseline)
    current_scope = _scope(current)
    scope_subjects = _scope_comparison(baseline_scope, current_scope)
    baseline_scope = {key: value for key, value in baseline_scope.items() if key != "subjects"}
    current_scope = {key: value for key, value in current_scope.items() if key != "subjects"}

    return {
        "schema": "fathom-hlsl-scope-replay-diff/v1",
        "source": {
            "commit": source_commit,
            "baseline_reading": "docs/bench/readings/lang/hlsl.json",
            "driver": "calibration/oracles/dxc-ast/reading.sh",
            "driver_unmodified": True,
            "go_version": go_version,
            "controller": {
                "commit": controller_commit,
                "files": {
                    "calibration/oracles/dxc-ast/scope-replay/replay.sh": replay_sha256,
                    "calibration/oracles/dxc-ast/scope-replay/publish.py": publish_sha256,
                    "calibration/oracles/dxc-ast/scope-replay/hlsl-scope-replay.yml": workflow_sha256,
                },
                "active_workflow": _active_workflow(
                    active_workflow_mode, active_workflow_path,
                    active_workflow_sha256),
            },
        },
        "corpus": {
            "manifest": "calibration/corpus.d/hlsl.yaml",
            "subject_count": len(current["subjects"]),
            "pins": [{"name": name, "sha": sha} for name, sha in _pins(current)],
        },
        "runtime": {
            "dxc_release": DXC_RELEASE,
            "dxc_asset": DXC_ASSET,
            "dxc_archive_sha256": dxc_sha256,
            "required_glibc": required_glibc,
            "runner_glibc": runner_glibc,
        },
        "tools": {
            "baseline": {
                "dxc": _oracle_version(baseline, "dxc-ast"),
                "clang": _oracle_version(baseline, "clang-includes"),
            },
            "current": {
                "dxc": _oracle_version(current, "dxc-ast"),
                "clang": _oracle_version(current, "clang-includes"),
            },
        },
        "scope": {
            "baseline": baseline_scope,
            "current": current_scope,
            "dxc_files_delta": current_scope["dxc_files"] - baseline_scope["dxc_files"],
            "measured_files_delta": (current_scope["measured_files"] -
                                     baseline_scope["measured_files"]),
            "subjects": scope_subjects,
        },
        "level": {
            "baseline": baseline["level_met"],
            "current": current["level_met"],
            "changed": baseline["level_met"] != current["level_met"],
        },
        "grade": {"baseline": baseline["grade"], "current": current["grade"]},
        "calls": {
            "baseline": {
                "precision": baseline["metrics"]["call_precision"],
                "recall": baseline["metrics"]["call_recall"],
            },
            "current": {
                "precision": current["metrics"]["call_precision"],
                "recall": current["metrics"]["call_recall"],
            },
            "delta": {
                "precision": (current["metrics"]["call_precision"] -
                              baseline["metrics"]["call_precision"]),
                "recall": (current["metrics"]["call_recall"] -
                           baseline["metrics"]["call_recall"]),
            },
        },
        "metrics": metrics,
        "counts": {
            "baseline": baseline_counts,
            "current": current_counts,
            "delta": count_delta,
        },
        "subject_counts": _subject_counts(baseline, current),
        "performance": {
            "comparison": ("fresh tenancy participates in grading and may change the current level or grade; "
                           "timing is reported separately from scope and counter changes"),
            "baseline_committed_throughput_mb_s": baseline["tenancy"]["throughput_mb_s"],
            "baseline_historical_driver_runs_mb_s": _runs(baseline),
            "fresh_driver_runs_mb_s": _runs(current),
            "fresh_selected_median_throughput_mb_s": current["tenancy"]["throughput_mb_s"],
            "basis": current["tenancy"].get("throughput_basis", "cpu"),
        },
        "redactions": redactions,
    }


def _load(path):
    return json.loads(Path(path).read_text())


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--baseline", required=True)
    parser.add_argument("--regenerated", required=True)
    parser.add_argument("--reading-out", required=True)
    parser.add_argument("--diff-out", required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--dxc-sha256", required=True)
    parser.add_argument("--required-glibc", required=True)
    parser.add_argument("--runner-glibc", required=True)
    parser.add_argument("--go-version", required=True)
    parser.add_argument("--controller-commit", required=True)
    parser.add_argument("--replay-sh-sha256", required=True)
    parser.add_argument("--publish-py-sha256", required=True)
    parser.add_argument("--workflow-sha256", required=True)
    parser.add_argument("--active-workflow-mode", required=True)
    parser.add_argument("--active-workflow-path", required=True)
    parser.add_argument("--active-workflow-sha256", required=True)
    args = parser.parse_args()

    baseline = _load(args.baseline)
    regenerated = _load(args.regenerated)
    sanitized, redactions = sanitize_reading(regenerated, args.source_commit)
    diff = build_diff(
        baseline, regenerated, redactions, source_commit=args.source_commit,
        dxc_sha256=args.dxc_sha256, required_glibc=args.required_glibc,
        runner_glibc=args.runner_glibc, go_version=args.go_version,
        controller_commit=args.controller_commit,
        replay_sha256=args.replay_sh_sha256,
        publish_sha256=args.publish_py_sha256,
        workflow_sha256=args.workflow_sha256,
        active_workflow_mode=args.active_workflow_mode,
        active_workflow_path=args.active_workflow_path,
        active_workflow_sha256=args.active_workflow_sha256)
    Path(args.reading_out).write_text(json.dumps(sanitized, indent=2) + "\n")
    Path(args.diff_out).write_text(json.dumps(diff, indent=2) + "\n")


if __name__ == "__main__":
    main()
