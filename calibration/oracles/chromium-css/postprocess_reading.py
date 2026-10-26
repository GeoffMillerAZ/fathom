#!/usr/bin/env python3
"""Restore historical identity/timing after the C2-F1 classified-oracle replay.

The result is an intermediate reading. Finish it with indexbench lang-regrade,
which recomputes grades from subjects and writes langbench.MarshalReading bytes.
"""

import argparse
import copy
import json
from pathlib import Path


CORRECTION = (
    "C2-F1 correction: proposal-selected ranges are tier C; a coverage floor alone cannot grant tier A. "
    "Historical source/oracle data and timing are preserved; the receipt records the classification harness commit and original hashes."
)


def corrected(historical, replay):
    if historical["metric_tiers"].get("range_agreement") != "A":
        raise ValueError("historical range tier must be A")
    if replay["metric_tiers"].get("range_agreement") != "C":
        raise ValueError("classified replay range tier must be C")
    for key in ("dialect", "kind", "target_level", "counts", "metrics", "required_kinds"):
        if historical.get(key) != replay.get(key):
            raise ValueError(f"replay changed {key}")
    prior_tiers = {key: tier for key, tier in historical["metric_tiers"].items() if key != "range_agreement"}
    replay_tiers = {key: tier for key, tier in replay["metric_tiers"].items() if key != "range_agreement"}
    if prior_tiers != replay_tiers:
        raise ValueError("replay changed non-range metric tiers")
    old_subjects = {s["name"]: s for s in historical["subjects"]}
    new_subjects = {s["name"]: s for s in replay["subjects"]}
    if old_subjects.keys() != new_subjects.keys():
        raise ValueError("replay changed subject names")
    for name, old in old_subjects.items():
        new = new_subjects[name]
        for key in ("sha", "files", "bytes", "counts", "kind_intersection", "excluded_by_kind"):
            if old.get(key) != new.get(key):
                raise ValueError(f"replay changed {name} {key}")
        families = dict(new["families"])
        if families.pop("ranges", None) != "C" or families != old["families"]:
            raise ValueError(f"replay changed {name} families beyond range tier")

    result = dict(historical)
    result["level_met"] = replay["level_met"]
    result["grade"] = replay["grade"]
    result["metric_tiers"] = replay["metric_tiers"]
    result["subjects"] = []
    for old in historical["subjects"]:
        subject = dict(old)
        subject["families"] = new_subjects[old["name"]]["families"]
        result["subjects"].append(subject)
    notes = [note.replace("tier A", "tier C", 1) if "range_agreement measured against " in note else note
             for note in historical.get("notes", [])]
    notes += [note for note in replay.get("notes", []) if ": restricted to " in note]
    result["notes"] = sorted(set(notes + [CORRECTION]))
    return result


def self_test():
    subject = {"name": "sample", "sha": "abc", "files": 1, "bytes": 10,
               "counts": {"range_eligible": 1}, "families": {"l1": "A"}}
    historical = {"dialect": "css", "kind": "language", "target_level": "L1",
                  "level_met": "L1", "grade": {"L1": "excellent"},
                  "counts": {"range_eligible": 1}, "metrics": {"range_agreement": 1.0},
                  "metric_tiers": {"range_agreement": "A", "sym_recall": "A"},
                  "required_kinds": ["rule"], "subjects": [subject],
                  "fathom_commit": "original", "measured_at": "frozen",
                  "tenancy": {"extract_ms": 123}, "notes": ["Timing correction: frozen trial"]}
    replay = copy.deepcopy(historical)
    replay["metric_tiers"]["range_agreement"] = "C"
    replay["grade"]["L1"] = "good"
    replay["subjects"][0]["families"]["ranges"] = "C"
    replay["fathom_commit"] = "replay"
    replay["tenancy"] = {"extract_ms": 999}
    result = corrected(historical, replay)
    if (result["fathom_commit"], result["measured_at"], result["tenancy"]) != \
            ("original", "frozen", {"extract_ms": 123}):
        raise AssertionError("historical identity or timing was lost")
    if "Timing correction: frozen trial" not in result["notes"]:
        raise AssertionError("historical timing note was lost")
    for label, change in (
        ("counts", lambda r: r["counts"].update(range_eligible=2)),
        ("metrics", lambda r: r["metrics"].update(range_agreement=0.5)),
        ("subjects", lambda r: r["subjects"][0].update(sha="different")),
    ):
        altered = copy.deepcopy(replay)
        change(altered)
        try:
            corrected(historical, altered)
        except ValueError:
            pass
        else:
            raise AssertionError(f"changed {label} was accepted")
    print("CSS reading postprocessing controls passed")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--self-test", action="store_true")
    parser.add_argument("--historical", type=Path)
    parser.add_argument("--replay", type=Path)
    parser.add_argument("--out", type=Path)
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return
    if args.historical is None or args.replay is None or args.out is None:
        parser.error("--historical, --replay and --out are required")
    if args.out.exists():
        parser.error("refusing to overwrite an existing reading")
    historical = json.loads(args.historical.read_text())
    replay = json.loads(args.replay.read_text())
    result = corrected(historical, replay)
    args.out.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")


if __name__ == "__main__":
    main()
