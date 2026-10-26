#!/usr/bin/env python3
"""Sanitize one HLSL replay reading and emit an aggregate baseline diff."""

import argparse
import copy
import json
import math
from pathlib import Path
import re


DXC_RELEASE = "v1.9.2602"
DXC_ASSET = "linux_dxc_2026_02_20.x86_64.tar.gz"
SOURCE_COMMIT = "6db920e72898abb20bf86c22652b401902e34598"

_RUNS_RE = re.compile(
    r"three runs over every measured file read "
    r"([0-9.]+), ([0-9.]+), ([0-9.]+) MB/s CPU")
_SCOPE_RE = re.compile(
    r"^([^:]+): dxc-ast .*?: ([0-9]+) of ([0-9]+) measured files in scope")


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


def _pins(data, label):
    subjects = data.get("subjects")
    if not isinstance(subjects, list):
        raise ValueError(f"{label} reading subjects are not a list")
    pins = []
    names = set()
    for row in subjects:
        if not isinstance(row, dict) or not isinstance(row.get("name"), str):
            raise ValueError(f"{label} reading has an invalid subject")
        if row["name"] in names:
            raise ValueError(f"{label} reading must have unique subject names")
        if not isinstance(row.get("sha"), str) or not re.fullmatch(r"[0-9a-f]{40}", row["sha"]):
            raise ValueError(f"{label} reading has an invalid subject pin")
        names.add(row["name"])
        pins.append((row["name"], row.get("sha")))
    return sorted(pins)


def _fmt(values):
    return ", ".join(f"{value:g}" for value in values)


_NAME_EMAIL_RE = re.compile(
    r"\b[A-Za-z][A-Za-z .'-]{0,100}\s*<[^<>\s@]+@[^<>\s@]+>"
)
_EMAIL_RE = re.compile(r"(?<![\w.+-])[^\s<>@]+@[^\s<>@]+")
_METADATA_RE = re.compile(
    r"(?im)\b(Author|Committer)\s*:(?!\s*\[redacted-person\])\s*[^\n;,]+|"
    r"\bCopyright(?:\s+\(c\)|\s+©)?(?:\s+\d{4}(?:-\d{4})?)?\s+[^\n;]+"
)
_ABSOLUTE_PATH_RE = re.compile(
    r"(?<![\w:/.])/(?:[A-Za-z0-9._-]+/)+[A-Za-z0-9._-]+(?::[0-9]+)?"
)


def _redact_string(value, counts):
    def replace_person(match):
        counts["person_matches_redacted"] += 1
        text = match.group(0)
        if ":" in text and text.split(":", 1)[0].lower() in ("author", "committer"):
            return text.split(":", 1)[0] + ": [redacted-person]"
        if text.lower().startswith("copyright"):
            prefix = re.match(
                r"(?i)Copyright(?:\s+\(c\)|\s+©)?(?:\s+\d{4}(?:-\d{4})?)?", text)
            return prefix.group(0) + " [redacted-person]"
        return "[redacted-person]"

    value = _NAME_EMAIL_RE.sub(replace_person, value)
    value = _EMAIL_RE.sub(replace_person, value)
    value = _METADATA_RE.sub(replace_person, value)

    def replace_path(_match):
        counts["absolute_paths_redacted"] += 1
        return "[redacted-absolute-path]"

    return _ABSOLUTE_PATH_RE.sub(replace_path, value)


def _sanitize_value(value, counts):
    if isinstance(value, dict):
        if any(not isinstance(key, str) for key in value):
            raise ValueError("JSON object has a non-string key")
        return {key: _sanitize_value(item, counts) for key, item in value.items()}
    if isinstance(value, list):
        return [_sanitize_value(item, counts) for item in value]
    if isinstance(value, str):
        return _redact_string(value, counts)
    if value is None or isinstance(value, (bool, int, float)):
        if isinstance(value, float) and not math.isfinite(value):
            raise ValueError("JSON number is not finite")
        return value
    raise ValueError(f"unrecognized JSON value type: {type(value).__name__}")


def sanitize_reading(data, source_commit):
    """Redact person-shaped text and absolute host paths while retaining evidence."""
    if not re.fullmatch(r"[0-9a-f]{40}", source_commit):
        raise ValueError("source commit must be a full lowercase SHA")
    counts = {"person_matches_redacted": 0, "absolute_paths_redacted": 0}
    result = _sanitize_value(copy.deepcopy(data), counts)
    tenancy_blocks = [result.get("tenancy", {})]
    tenancy_blocks.extend(row.get("tenancy", {}) for row in result.get("subjects", []))
    for tenancy in tenancy_blocks:
        paths = tenancy.get("faulting_files")
        if paths is None:
            if tenancy.get("faults", 0) != 0:
                raise ValueError("tenancy faults do not match faulting-file entries")
        elif not isinstance(paths, list) or tenancy.get("faults") != len(paths):
            raise ValueError("tenancy faults do not match faulting-file entries")
    return result, counts


_MISSING = object()


def _json_type(value):
    if value is _MISSING:
        return "missing"
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "boolean"
    if isinstance(value, (int, float)):
        return "number"
    if isinstance(value, str):
        return "string"
    if isinstance(value, list):
        return "array"
    if isinstance(value, dict):
        return "object"
    raise ValueError(f"unrecognized JSON value type: {type(value).__name__}")


def _path_key(path, key):
    if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", key):
        return f"{path}.{key}"
    return f"{path}[{json.dumps(key, ensure_ascii=False)}]"


def recursive_diff(before, after):
    """Return a deterministic field-by-field comparison of two JSON values."""
    _sanitize_value(before, {"person_matches_redacted": 0, "absolute_paths_redacted": 0})
    _sanitize_value(after, {"person_matches_redacted": 0, "absolute_paths_redacted": 0})
    fields = []

    def walk(path, old, new):
        old_type, new_type = _json_type(old), _json_type(new)
        if old is _MISSING:
            change = "added"
        elif new is _MISSING:
            change = "removed"
        elif old_type != new_type:
            change = "type-changed"
        else:
            change = "unchanged" if old == new else "changed"
        row = {"path": path, "before_type": old_type, "after_type": new_type,
               "change": change}
        if old_type not in ("object", "array", "missing"):
            row["before"] = old
        if new_type not in ("object", "array", "missing"):
            row["after"] = new
        if (old_type == new_type == "number" and
                not isinstance(old, bool) and not isinstance(new, bool)):
            delta = new - old
            if isinstance(delta, float) and not math.isfinite(delta):
                raise ValueError(f"numeric delta is not finite at {path}")
            row["delta"] = delta
        fields.append(row)

        if old_type == "object" or new_type == "object":
            old_map = old if old_type == "object" else {}
            new_map = new if new_type == "object" else {}
            for key in sorted(set(old_map) | set(new_map)):
                walk(_path_key(path, key), old_map.get(key, _MISSING),
                     new_map.get(key, _MISSING))
        elif old_type == "array" or new_type == "array":
            old_list = old if old_type == "array" else []
            new_list = new if new_type == "array" else []
            for index in range(max(len(old_list), len(new_list))):
                walk(f"{path}[{index}]",
                     old_list[index] if index < len(old_list) else _MISSING,
                     new_list[index] if index < len(new_list) else _MISSING)

    walk("$", before, after)
    return {"fields": fields,
            "changed_fields": sum(row["change"] != "unchanged" for row in fields)}


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


def _call_metrics(data):
    return {"precision": data["metrics"]["call_precision"],
            "recall": data["metrics"]["call_recall"]}


def _call_summary(baseline, current, control):
    committed_to_main = _metric_comparison(
        _call_metrics(baseline), _call_metrics(current))
    control_to_main = _metric_comparison(
        _call_metrics(control), _call_metrics(current))
    return {
        "baseline": committed_to_main["baseline"],
        "current": committed_to_main["current"],
        "control": control_to_main["baseline"],
        "delta": committed_to_main["delta"],
        "change": committed_to_main["change"],
        "control_to_main_delta": control_to_main["delta"],
        "control_to_main_change": control_to_main["change"],
    }


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


def _validate_revision(data, label, source_commit, allow_dirty=False):
    revision = data.get("fathom_commit")
    pattern = r"([0-9a-f]{7,40})(?:[-+.]dirty(?:[-+.][A-Za-z0-9._]+)*)?" if allow_dirty else r"([0-9a-f]{7,40})"
    match = re.fullmatch(pattern, revision or "") if isinstance(revision, str) else None
    if not match or not source_commit.startswith(match.group(1)):
        raise ValueError(f"{label} reading does not identify the replay source commit")


def _validate_control_receipt(receipt):
    if (not isinstance(receipt, dict) or set(receipt) !=
            {"input_files", "input_bytes", "unchanged"} or
            receipt.get("input_files") != 9 or
            isinstance(receipt.get("input_bytes"), bool) or
            not isinstance(receipt.get("input_bytes"), int) or
            receipt["input_bytes"] <= 0 or receipt.get("unchanged") is not True):
        raise ValueError("invalid control oracle-input receipt")
    return receipt


def build_diff(baseline, current, control, redactions, *, source_commit, dxc_sha256,
               required_glibc, runner_glibc, go_version, controller_commit,
               replay_sha256, publish_sha256, workflow_sha256, control_patch_sha256,
               control_receipt,
               active_workflow_mode, active_workflow_path,
               active_workflow_sha256):
    if source_commit != SOURCE_COMMIT:
        raise ValueError("source commit is not the pinned replay source")
    baseline_pins = _pins(baseline, "baseline")
    current_pins = _pins(current, "main")
    control_pins = _pins(control, "control")
    if baseline_pins != current_pins or baseline_pins != control_pins:
        raise ValueError("baseline, main, and control subject pins differ")
    _validate_revision(current, "main", source_commit)
    _validate_revision(control, "control", source_commit, allow_dirty=True)
    if not re.fullmatch(r"[0-9a-f]{64}", control_patch_sha256 or ""):
        raise ValueError("invalid control patch sha256")
    control_receipt = _validate_control_receipt(control_receipt)

    public_baseline, _ = sanitize_reading(baseline, source_commit)
    public_current, _ = sanitize_reading(current, source_commit)
    public_control, control_redactions = sanitize_reading(control, source_commit)
    comparisons = {
        "committed_to_main": recursive_diff(public_baseline, public_current),
        "committed_to_control": recursive_diff(public_baseline, public_control),
        "control_to_main": recursive_diff(public_control, public_current),
    }
    comparisons["committed_to_main"]["interpretation"] = (
        "historical committed reading versus the fresh main reading; changes may include baseline drift")
    comparisons["committed_to_control"]["interpretation"] = (
        "historical committed reading versus the fresh control reading; changes may include baseline drift")
    comparisons["control_to_main"]["interpretation"] = (
        "semantic counter and metric changes isolate the causal effect of #45; timing differences are not causal proof")

    baseline_counts = _numeric_map(baseline["counts"], "baseline counts")
    current_counts = _numeric_map(current["counts"], "current counts")
    count_delta = _counter_delta(baseline_counts, current_counts)
    metrics = _metric_comparison(baseline["metrics"], current["metrics"])
    baseline_scope = _scope(baseline)
    current_scope = _scope(current)
    control_scope = _scope(control)
    scope_subjects = _scope_comparison(baseline_scope, current_scope)
    baseline_scope = {key: value for key, value in baseline_scope.items() if key != "subjects"}
    current_scope = {key: value for key, value in current_scope.items() if key != "subjects"}
    control_scope = {key: value for key, value in control_scope.items() if key != "subjects"}

    return {
        "schema": "fathom-hlsl-scope-replay-diff/v2",
        "source": {
            "commit": source_commit,
            "baseline_reading": "docs/bench/readings/lang/hlsl.json",
            "driver": "calibration/oracles/dxc-ast/reading.sh",
            "driver_unmodified": True,
            "go_version": go_version,
            "control_patch_sha256": control_patch_sha256,
            "control_oracle_inputs": control_receipt,
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
            "pins": [{"name": name, "sha": sha} for name, sha in current_pins],
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
            "control": {
                "dxc": _oracle_version(control, "dxc-ast"),
                "clang": _oracle_version(control, "clang-includes"),
            },
        },
        "scope": {
            "baseline": baseline_scope,
            "current": current_scope,
            "control": control_scope,
            "dxc_files_delta": current_scope["dxc_files"] - baseline_scope["dxc_files"],
            "measured_files_delta": (current_scope["measured_files"] -
                                     baseline_scope["measured_files"]),
            "subjects": scope_subjects,
            "control_to_main": {
                "dxc_files_delta": current_scope["dxc_files"] - control_scope["dxc_files"],
                "measured_files_delta": (current_scope["measured_files"] -
                                         control_scope["measured_files"]),
            },
        },
        "level": {
            "baseline": baseline["level_met"],
            "current": current["level_met"],
            "control": control["level_met"],
            "changed": baseline["level_met"] != current["level_met"],
            "control_to_main_changed": control["level_met"] != current["level_met"],
        },
        "grade": {"baseline": baseline["grade"], "current": current["grade"],
                  "control": control["grade"]},
        "calls": _call_summary(baseline, current, control),
        "metrics": metrics,
        "control_to_main_metrics": _metric_comparison(control["metrics"], current["metrics"]),
        "counts": {
            "baseline": baseline_counts,
            "current": current_counts,
            "delta": count_delta,
            "control": _numeric_map(control["counts"], "control counts"),
            "control_to_main_delta": _counter_delta(
                _numeric_map(control["counts"], "control counts"), current_counts),
        },
        "subject_counts": _subject_counts(baseline, current),
        "performance": {
            "comparison": ("fresh tenancy participates in grading and may change the current level or grade; "
                           "timing is reported separately and is not causal proof of #45's effect"),
            "baseline_committed_throughput_mb_s": baseline["tenancy"]["throughput_mb_s"],
            "baseline_historical_driver_runs_mb_s": _runs(baseline),
            "fresh_driver_runs_mb_s": _runs(current),
            "fresh_control_driver_runs_mb_s": _runs(control),
            "fresh_selected_median_throughput_mb_s": current["tenancy"]["throughput_mb_s"],
            "fresh_control_selected_median_throughput_mb_s":
                control["tenancy"]["throughput_mb_s"],
            "basis": current["tenancy"].get("throughput_basis", "cpu"),
        },
        "redactions": redactions,
        "control_redactions": control_redactions,
        "control_reading": public_control,
        "comparisons": comparisons,
    }


def _load(path):
    return json.loads(Path(path).read_text())


def _load_json_argument(value):
    if value.lstrip().startswith("{"):
        return json.loads(value)
    return _load(value)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--baseline", required=True)
    parser.add_argument("--regenerated", required=True)
    parser.add_argument("--control", required=True)
    parser.add_argument("--control-receipt", required=True)
    parser.add_argument("--reading-out", required=True)
    parser.add_argument("--diff-out", required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--dxc-sha256", required=True)
    parser.add_argument("--required-glibc", required=True)
    parser.add_argument("--runner-glibc", required=True)
    parser.add_argument("--go-version", required=True)
    parser.add_argument("--controller-commit", required=True)
    parser.add_argument("--control-patch-sha256", required=True)
    parser.add_argument("--replay-sh-sha256", required=True)
    parser.add_argument("--publish-py-sha256", required=True)
    parser.add_argument("--workflow-sha256", required=True)
    parser.add_argument("--active-workflow-mode", required=True)
    parser.add_argument("--active-workflow-path", required=True)
    parser.add_argument("--active-workflow-sha256", required=True)
    args = parser.parse_args()

    baseline = _load(args.baseline)
    regenerated = _load(args.regenerated)
    control = _load(args.control)
    control_receipt = _load_json_argument(args.control_receipt)
    sanitized, redactions = sanitize_reading(regenerated, args.source_commit)
    diff = build_diff(
        baseline, regenerated, control, redactions, source_commit=args.source_commit,
        dxc_sha256=args.dxc_sha256, required_glibc=args.required_glibc,
        runner_glibc=args.runner_glibc, go_version=args.go_version,
        controller_commit=args.controller_commit,
        control_patch_sha256=args.control_patch_sha256,
        control_receipt=control_receipt,
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
