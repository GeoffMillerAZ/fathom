#!/usr/bin/env python3
"""Prove the Crossplane qualification row rejects damaged reading evidence.

The store bootstraps symbol access only. The verifier reads the claimed row and
the live reading from a separate scratch Git tree for every verdict.
"""

import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

from regrade_d19 import admit_launch


ROOT = Path(__file__).resolve().parents[3]
ROW_ID = "LB-IAC-crossplane-pt1"
PLAN = Path("docs/plans/language-breadth.md")
READING = Path("docs/bench/readings/lang/crossplane.json")
MAIN_COMMIT = "3118be853"
TIMEOUT_SECONDS = 120


def sha256(raw):
    return hashlib.sha256(raw).hexdigest()


def row_from(raw):
    rows = [line for line in raw.decode("utf-8").splitlines(keepends=True)
            if re.search(r"\bid:\s*" + re.escape(ROW_ID) + r"(?=[,}])", line)]
    if len(rows) != 1 or not re.match(r"^- \[ \] ", rows[0]):
        raise RuntimeError("qualification row must occur once and remain open")
    row = rows[0]
    if "oracle: lexical" not in row or "path: " + str(READING) not in row:
        raise RuntimeError("qualification row must bind the canonical reading")
    return row


def launch(command, cwd, label, env=None):
    admit_launch()
    try:
        result = subprocess.run(command, cwd=cwd, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, timeout=TIMEOUT_SECONDS,
                                check=False, env=env)
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError(label + " timed out") from exc
    return result


def verify(binary, store, scratch, label, expected_exit, expected_status):
    ledger = scratch / "workspace/artifacts/qualification-verify-ledger.jsonl"
    ledger.parent.mkdir(parents=True, exist_ok=True)
    env = {**os.environ, "FATHOM_VERIFY_LEDGER": str(ledger)}
    result = launch([str(binary), "verify", "--strict", "--service", "fathom",
                     "--store", str(store), "--plan", str(PLAN), "--row", ROW_ID,
                     "--json"], scratch, label, env=env)
    logs = ROOT / "workspace/logs"
    logs.mkdir(parents=True, exist_ok=True)
    (logs / ("crossplane-qualification-" + label + ".json")).write_bytes(result.stdout)
    (logs / ("crossplane-qualification-" + label + ".log")).write_bytes(result.stderr)
    try:
        document = json.loads(result.stdout)
        reports = document["reports"]
        if len(reports) != 1 or len(reports[0]["tasks"]) != 1:
            raise ValueError("unexpected report cardinality")
        task = reports[0]["tasks"][0]
        status = task["status"]
        gate_pass = document["gate"]["pass"]
        if task["id"] != ROW_ID or task.get("claimed") is not True:
            raise ValueError("scratch row was not the claimed target")
    except (ValueError, KeyError, IndexError, TypeError) as exc:
        raise RuntimeError(label + " did not return a claimed-row JSON verdict") from exc
    if (result.returncode != expected_exit or status != expected_status
            or gate_pass is not (expected_exit == 0)):
        raise RuntimeError(label + " failed the expected CLI gate contract: exit="
                           + str(result.returncode) + ", row=" + str(status)
                           + ", gate=" + str(gate_pass))
    return {"label": label, "exit_code": result.returncode,
            "row_status": status, "gate_pass": gate_pass}


def write_reading(path, document):
    path.write_text(json.dumps(document, indent=2, ensure_ascii=False) + "\n",
                    encoding="utf-8")


def mutate(original, field, before, after):
    document = copy.deepcopy(original)
    section, key = field.split(".")
    if document[section][key] != before:
        raise RuntimeError("canonical value changed for " + field)
    document[section][key] = after
    return document


def run(args):
    fathom = Path(args.fathom).resolve(strict=True)
    indexbench = Path(args.indexbench).resolve(strict=True)
    store = Path(args.store).resolve(strict=True)
    output = Path(args.out)
    if output.resolve() in {(ROOT / PLAN).resolve(), (ROOT / READING).resolve()}:
        raise RuntimeError("receipt must not overwrite production plan or reading")
    revision = launch(["git", "rev-parse", args.main_commit], ROOT, "main revision")
    if revision.returncode != 0:
        raise RuntimeError("main revision is unavailable")
    full_main = revision.stdout.decode().strip()
    for role, binary in (("fathom", fathom), ("indexbench", indexbench)):
        metadata = launch(["go", "version", "-m", str(binary)], ROOT, role + " identity")
        if (metadata.returncode != 0
                or ("vcs.revision=" + full_main).encode() not in metadata.stdout
                or b"vcs.modified=false" not in metadata.stdout):
            raise RuntimeError(role + " must be a clean build of the named main revision")
    if not store.is_file() or store.stat().st_size == 0:
        raise RuntimeError("store must be an existing nonempty SQLite file")
    with store.open("rb") as database:
        signature = database.read(16)
    if signature != b"SQLite format 3\x00":
        raise RuntimeError("store is not SQLite")
    plan_raw = (ROOT / PLAN).read_bytes()
    reading_raw = (ROOT / READING).read_bytes()
    row = row_from(plan_raw)
    original = json.loads(reading_raw)
    if original.get("dialect") != "crossplane":
        raise RuntimeError("canonical reading is not Crossplane")

    scratch_parent = ROOT / "workspace/tmp"
    scratch_parent.mkdir(parents=True, exist_ok=True)
    cases = []
    with tempfile.TemporaryDirectory(prefix="crossplane-qualification-",
                                     dir=scratch_parent) as temporary:
        scratch = Path(temporary)
        scratch_plan = scratch / PLAN
        scratch_reading = scratch / READING
        scratch_plan.parent.mkdir(parents=True)
        scratch_reading.parent.mkdir(parents=True)
        scratch_plan.write_text(row.replace("- [ ] ", "- [x] ", 1),
                                encoding="utf-8")
        scratch_reading.write_bytes(reading_raw)
        initialized = launch(["git", "init", "-q"], scratch, "scratch Git init")
        if initialized.returncode != 0:
            raise RuntimeError("scratch Git init failed")

        baseline = verify(fathom, store, scratch, "baseline", 0, "pass")
        cases.append(baseline)
        mutations = [
            ("grade_L1", "grade.L1", "good", "unmet"),
            ("grade_L2", "grade.L2", "good", "unmet"),
            ("grade_L3", "grade.L3", "good", "unmet"),
            ("grade_L4", "grade.L4", "not-applicable", "unmet"),
            ("metric_sym_recall", "metrics.sym_recall", 1, 0.97),
            ("tier_sym_recall", "metric_tiers.sym_recall", "C", "D"),
            ("counts_matched", "counts.matched", 428, 427),
        ]
        for label, field, before, after in mutations:
            write_reading(scratch_reading, mutate(original, field, before, after))
            verdict = verify(fathom, store, scratch, label, 1, "fail")
            verdict["mutation"] = field
            verdict["before"] = before
            verdict["after"] = after
            cases.append(verdict)

        population_mutants = []
        document = copy.deepcopy(original)
        before = document["subjects"][0]["files"]
        document["subjects"][0]["files"] = before - 1
        population_mutants.append(("population_files", "subjects[0].files",
                                   before, before - 1, document))
        document = copy.deepcopy(original)
        before = len(document["subjects"])
        document["subjects"].pop()
        population_mutants.append(("population_subjects", "subjects",
                                   before, before - 1, document))
        for key in ("required_kinds", "kind_intersection"):
            document = copy.deepcopy(original)
            before = len(document[key])
            document[key].remove("element")
            population_mutants.append(("population_" + key, key,
                                       before, before - 1, document))
        for label, field, before, after, document in population_mutants:
            write_reading(scratch_reading, document)
            verdict = verify(fathom, store, scratch, label, 1, "fail")
            verdict.update({"mutation": field, "before": before, "after": after})
            cases.append(verdict)

        regrade_input = copy.deepcopy(original)
        found_before = []
        for subject in regrade_input["subjects"]:
            counts = subject["counts"]
            found_before.append(counts["xref_truth_found"])
            counts["xref_truth_found"] = 0
        write_reading(scratch_reading, regrade_input)
        regraded = launch([str(indexbench), "lang-regrade", "-commit",
                           args.main_commit, str(scratch_reading)], ROOT,
                          "regrade_xref_truth_found")
        if regraded.returncode != 0:
            raise RuntimeError("real lang-regrade failed")
        regraded_reading = json.loads(scratch_reading.read_bytes())
        if (regraded_reading["grade"]["L3"] != "unmet"
                or regraded_reading["level_met"] not in ("L1", "L2")):
            raise RuntimeError("real regrade did not lower L3 and level_met")
        verdict = verify(fathom, store, scratch, "regraded_xref_truth_found", 1, "fail")
        verdict["mutation"] = "subjects[*].counts.xref_truth_found"
        verdict["before"] = sum(found_before)
        verdict["after"] = 0
        verdict["regraded_L3"] = regraded_reading["grade"]["L3"]
        verdict["regraded_level_met"] = regraded_reading["level_met"]
        cases.append(verdict)

        scratch_reading.write_bytes(reading_raw)
        cases.append(verify(fathom, store, scratch, "restored_baseline", 0, "pass"))

    production_plan_unchanged = (ROOT / PLAN).read_bytes() == plan_raw
    production_reading_unchanged = (ROOT / READING).read_bytes() == reading_raw
    if not production_plan_unchanged or not production_reading_unchanged:
        raise RuntimeError("production plan or reading changed during probe")
    receipt = {
        "row_id": ROW_ID,
        "main_commit": args.main_commit,
        "tool_revision": full_main,
        "tools_clean": True,
        "method": "real strict CLI verification of one claimed row and live reading in an isolated scratch Git tree; the supplied store bootstraps symbol access only",
        "canonical_reading_sha256": sha256(reading_raw),
        "row_contract_sha256": sha256(row.encode("utf-8")),
        "scratch_claimed": True,
        "production_claimed": False,
        "production_plan_unchanged": production_plan_unchanged,
        "production_reading_unchanged": production_reading_unchanged,
        "cases": cases,
    }
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n",
                      encoding="utf-8")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fathom", required=True)
    parser.add_argument("--indexbench", required=True)
    parser.add_argument("--store", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--main-commit", default=MAIN_COMMIT)
    run(parser.parse_args())
