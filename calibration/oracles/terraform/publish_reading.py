#!/usr/bin/env python3
"""Install the measured repaired Terraform reading with current provenance notes."""

import argparse
import hashlib
import json
import pathlib


NOTES = [
    "Native oracle: OpenTofu 1.13.0 show -json -config and graph -type=plan on every .tf directory (19 roots per subject), after init -backend=false -get=false. Preparation checks scratch .tf paths and bytes, strips all cached local records, and preseeds exact registry pins. Plain native init installs local entries; bounded retries retain native locals and restore only missing pinned registry descendants. Every native local key and source agrees with the separate scanner proposal; subject-local destinations agree lexically. Physical checkout paths for local children under registry modules pass only when their resolved target equals the pinned cache alias, with a separate alias count in the receipt. Native Dir values are never rewritten. All roots succeeded. Positions come from an independent line reader: 0 definitions unlocated, range_agreement 1.0 across 3,111 definitions. Registry module files are outside the measured subject. Providers came only from a private filesystem mirror; no plan, apply or cloud API was run. AWS 6.67.0 was identified previously; seven ancillary provider versions and ten registry module versions are newly pinned compatible inputs. See docs/assessments/data/terraform-two-hop-2026-10-26/summary.json.",
    "Independence: a provider-free fixture's native raw truth changes when a source fact changes. A separate document-fixture replay changes real Terraform extractor output while the raw native truth remains identical before and after; other exercised document extraction and mixed-dialect fixture results stay stable. The replay is fixture coverage, not full-subject regrading of other document dialects. See docs/assessments/2026-10-26-iac-docxref-impact.md.",
    "Timing: CPU-throughput median of three on a shared Apple Silicon helper, Go 1.27.3, nice 10, GOMAXPROCS=2, GOFLAGS=-p=2, load guard <=9 at launch. Per-run load and concurrent jobs were not recorded. Wall extract_ms is not comparable, and this reading makes no extraction-cost regression claim.",
]

HISTORICAL_NOTES = json.loads(pathlib.Path(__file__).with_name("historical_notes.json").read_text())
if len(HISTORICAL_NOTES) != 7:
    raise RuntimeError("historical Terraform adjudication notes changed")


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def grade_notes(comparison):
    baseline, repaired = comparison["baseline"], comparison["repaired"]
    names = ("terraform-aws-modules/terraform-aws-eks", "terraform-aws-modules/terraform-aws-vpc")
    details = []
    for label, grade in (("baseline", baseline), ("repaired", repaired)):
        pairs = [f'{grade["subjects"][name]["counts"]["xref_truth_found"]}/{grade["subjects"][name]["counts"]["xref_truth"]}'
                 for name in names]
        details.append(f'{label} {grade["commit"]}: EKS {pairs[0]}, VPC {pairs[1]}, '
                       f'total {grade["counts"]["xref_truth_found"]}/{grade["counts"]["xref_truth"]}, '
                       f'recall {grade["metrics"]["xref_recall"]:.4f}, '
                       f'precision {grade["metrics"]["xref_precision"]:.4f}')
    gain = repaired["counts"]["xref_truth_found"] - baseline["counts"]["xref_truth_found"]
    extras = repaired["counts"]["xref_fathom"] - repaired["counts"]["xref_fathom_in_truth"]
    notes = [HISTORICAL_NOTES[0],
             "L3: " + "; ".join(details) + ". Both clean binaries were graded three times against the same raw native truth and the full 87-file/523029-byte and 77-file/453543-byte measured sets; counts, metrics and grades agreed. The reading uses the median CPU-throughput run. L1 and L2 grades are " +
             f'{repaired["grade"]["L1"]} and {repaired["grade"]["L2"]}, respectively.',
             f"L3 limits: the repaired engine emits {extras} cross-file pairs beyond the native set. Historical adjudication: the 13 beyond the tool are the L2 blind spots, a local or a data source's dynamic block referring to variables.tf. This rerun did not individually re-adjudicate them. The native oracle scores file-granularity destinations, not typed attribute binding. The recovered {gain} pairs show the repair's measured effect; this aggregate result does not itself classify every historical miss by cause. Registry, remote, escaping, caller-directory and nonliteral module sources remain outside the repaired two-hop strategy.",
             HISTORICAL_NOTES[2], NOTES[0], HISTORICAL_NOTES[4],
             HISTORICAL_NOTES[5], HISTORICAL_NOTES[6], NOTES[1], NOTES[2]]
    return notes


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cache", type=pathlib.Path, required=True)
    parser.add_argument("--out", type=pathlib.Path, required=True)
    args = parser.parse_args()
    cache = args.cache.resolve()
    selected = cache / "native" / "grades" / "repaired" / "selected.json"
    comparison = json.loads((cache / "native" / "grades" / "comparison.json").read_text())
    if sha256(selected) != comparison["repaired"]["selected_reading_sha256"]:
        raise RuntimeError("selected measured reading changed")
    reading = json.loads(selected.read_text())
    if (reading["fathom_commit"] != "2ff8b2ac3" or reading["measured_at"] != "2026-10-26"
            or reading["counts"]["xref_truth"] != 3528
            or reading["counts"]["xref_truth_found"] != 3528
            or reading["grade"]["L3"] != "excellent"):
        raise RuntimeError("selected reading is not the measured repaired grade")
    old = json.loads(args.out.read_text())
    if reading["required_kinds"] != old["required_kinds"]:
        raise RuntimeError("required kind set changed")
    receipt = json.loads((pathlib.Path(__file__).resolve().parents[3] / "docs" / "assessments" /
                          "data" / "terraform-two-hop-2026-10-26" / "summary.json").read_text())
    if not all(subject["raw_truth_unchanged"] for subject in receipt["subjects"]):
        raise RuntimeError("native after identity not proven")
    if not all(subject.get("native_local_agreements", 0) > 0 for subject in receipt["subjects"]):
        raise RuntimeError("native installer local destinations not proven")
    if (receipt["baseline_full_commit"] != comparison["baseline_full_commit"]
            or receipt["repaired_full_commit"] != comparison["repaired_full_commit"]):
        raise RuntimeError("public receipt still names a different engine comparison")
    reading["notes"] = grade_notes(comparison)
    args.out.write_text(json.dumps(reading, indent=2, ensure_ascii=False) + "\n")
    print(json.dumps({"reading_sha256": sha256(args.out), "xref_truth": 3528,
                      "xref_found": 3528, "grade": reading["grade"]["L3"]}, sort_keys=True))


if __name__ == "__main__":
    main()
