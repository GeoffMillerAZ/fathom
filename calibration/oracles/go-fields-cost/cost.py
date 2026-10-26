#!/usr/bin/env python3
"""Measure the Go field change with alternating, fresh-store direct ingests."""

import argparse
import hashlib
import json
import os
import pathlib
import resource
import sqlite3
import statistics
import subprocess
import time
import sys


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def manifest(root):
    digest = hashlib.sha256()
    for item in sorted(root.rglob("*")):
        if item.is_symlink():
            body = os.readlink(item).encode()
        elif item.is_file():
            body = item.read_bytes()
        else:
            continue
        digest.update(str(item.relative_to(root)).encode() + b"\0" + hashlib.sha256(body).digest())
    return digest.hexdigest()


def commit(value):
    if len(value) != 40 or any(c not in "0123456789abcdef" for c in value):
        raise argparse.ArgumentTypeError("commit must be a full 40-character lowercase SHA")
    return value


def read_store(store):
    wal = pathlib.Path(str(store) + "-wal")
    if wal.exists() and wal.stat().st_size:
        raise RuntimeError(f"nonempty WAL: {wal}; immutable read would be unsafe")
    con = sqlite3.connect("file:" + str(store.resolve()) + "?mode=ro&immutable=1", uri=True)
    try:
        names = {r[0] for r in con.execute("select name from sqlite_master where type='table'")}
        rows = {}
        for table in ("observations", "typed_edges", "symbols", "symbol_nodes", "centrality"):
            if table in names:
                rows[table] = con.execute("select count(*) from " + table).fetchone()[0]
        kinds = {}
        if "typed_edges" in names:
            cols = {r[1] for r in con.execute("pragma table_info(typed_edges)")}
            if "kind" in cols:
                kinds = dict(con.execute("select kind,count(*) from typed_edges group by kind"))
        return rows, kinds
    finally:
        con.close()


def workload(report):
    keys = ("outcome", "exit_code", "discovered", "extracted", "skipped", "read_failed",
            "unclassified", "oversize", "classified_zero", "skipped_generated",
            "classified_by_sniff", "file_resolved", "excluded")
    return {k: report[k] for k in keys if k in report}


def typed(report):
    return report.get("typed_edges", {})


def save(path, receipt):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    for name in ("baseline", "candidate", "subject", "stores", "out", "log_dir"):
        ap.add_argument("--" + name.replace("_", "-"), required=True, type=pathlib.Path)
    ap.add_argument("--source-commit", required=True, type=commit)
    ap.add_argument("--candidate-commit", required=True, type=commit)
    args = ap.parse_args()
    binary = {"baseline": args.baseline.resolve(), "candidate": args.candidate.resolve()}
    subject = args.subject.resolve()
    if not subject.is_dir() or any(not p.is_file() for p in binary.values()):
        ap.error("subject directory and both binaries must exist")
    args.stores.mkdir(parents=True, exist_ok=True)
    args.log_dir.mkdir(parents=True, exist_ok=True)
    priority = subprocess.run(["nice", "-n", "10", sys.executable, "-c", "import os; print(os.getpriority(os.PRIO_PROCESS, 0))"], capture_output=True, text=True, check=True)
    if int(priority.stdout.strip()) < 10 or priority.stderr.strip():
        raise RuntimeError("nice 10 unavailable; do not measure at the wrong priority")
    source_sha = manifest(subject)
    receipt = {
        "source_commit": args.source_commit,
        "candidate_commit": args.candidate_commit,
        "source_manifest_sha256": source_sha,
        "binary_sha256": {arm: sha256(p) for arm, p in binary.items()},
        "method": "three alternating baseline/candidate direct CLI ingests, fresh stores, nice 10, 20-minute timeout; no daemon or SCIP",
        "wall_gate": 0.15, "store_gate": 0.20,
        "readout": "SQLite immutable read only after process exit and no nonempty WAL",
        "runs": [],
    }
    save(args.out, receipt)
    env = os.environ.copy()
    env.update({"GOMAXPROCS": "2", "GOFLAGS": "-p=2", "GOWORK": "off",
                "GOTOOLCHAIN": "local", "FATHOM_DECIDE": "off"})
    for run in range(1, 4):
        for arm in ("baseline", "candidate"):
            while os.getloadavg()[0] > 9:
                print(f"waiting for load <=9 before {arm} run {run}: {os.getloadavg()}", flush=True)
                time.sleep(20)
            if manifest(subject) != source_sha:
                raise RuntimeError("source manifest changed before run")
            store = args.stores / f"{arm}-{run}.sqlite"
            if store.exists():
                raise RuntimeError(f"store already exists: {store}")
            stdout = args.log_dir / f"{arm}-{run}.json"
            stderr = args.log_dir / f"{arm}-{run}.log"
            if stdout.exists() or stderr.exists():
                raise RuntimeError(f"run log already exists for {arm} {run}")
            load_start = os.getloadavg()[0]
            while load_start > 9:
                print(f"post-manifest load wait: {load_start}", flush=True)
                time.sleep(20)
                load_start = os.getloadavg()[0]
            max_load = load_start
            before = resource.getrusage(resource.RUSAGE_CHILDREN)
            start = time.monotonic()
            with stdout.open("w") as out_file, stderr.open("w") as err_file:
                process = subprocess.Popen(
                    ["nice", "-n", "10", str(binary[arm]), "ingest", "--store", str(store),
                     "--service", "self-cost", "--path", str(subject), "--json"],
                    stdout=out_file, stderr=err_file, env=env)
                while process.poll() is None:
                    time.sleep(1)
                    max_load = max(max_load, os.getloadavg()[0])
                    if time.monotonic() - start > 1200:
                        process.terminate()
                        process.wait()
                        raise TimeoutError(f"{arm} run {run} exceeded 20 minutes")
            elapsed = time.monotonic() - start
            after = resource.getrusage(resource.RUSAGE_CHILDREN)
            unchanged = manifest(subject) == source_sha
            if process.returncode != 0 or not unchanged:
                raise RuntimeError(f"invalid {arm} run {run}: exit={process.returncode}, source_unchanged={unchanged}; see {stderr}")
            report = json.loads(stdout.read_text())
            if (report.get("outcome") != "ok" or report.get("read_failed", 0) != 0
                    or typed(report).get("packages_processed", 0) <= 0
                    or typed(report).get("packages_failed", 0) != 0
                    or typed(report).get("failed", False)):
                raise RuntimeError("required Go work failed or did not execute")
            rows, kinds = read_store(store)
            sidecars = sum(pathlib.Path(str(store) + suffix).stat().st_size for suffix in ("-wal", "-shm")
                           if pathlib.Path(str(store) + suffix).exists())
            row = {"run": run, "arm": arm, "exit": process.returncode,
                   "wall_seconds": round(elapsed, 3),
                   "cpu_seconds": round((after.ru_utime + after.ru_stime) - (before.ru_utime + before.ru_stime), 3),
                   "store_bytes": store.stat().st_size, "store_sidecar_bytes": sidecars,
                   "load_start": round(load_start, 2), "load_end": round(os.getloadavg()[0], 2),
                   "max_sampled_load": round(max_load, 2), "source_unchanged": unchanged,
                   "workload": workload(report), "typed": typed(report),
                   "table_rows": rows, "typed_edge_kinds": kinds, "symbols": report.get("symbols", 0)}
            receipt["runs"].append(row)
            save(args.out, receipt)
            print(f"completed {arm} run {run}: {elapsed:.3f}s, {row['store_bytes']} bytes", flush=True)

    base = [r for r in receipt["runs"] if r["arm"] == "baseline"]
    cand = [r for r in receipt["runs"] if r["arm"] == "candidate"]
    control_keys = ("workload", "typed", "typed_edge_kinds")
    equivalent = all(r[key] == base[0][key] for r in base + cand for key in control_keys)
    changed_work = all(r["symbols"] > base[0]["symbols"] for r in cand)
    equivalent = equivalent and changed_work
    receipt["changed_field_work_executed"] = changed_work
    receipt["work_equivalent"] = equivalent
    receipt["baseline"] = {"median_wall_seconds": statistics.median(r["wall_seconds"] for r in base),
                           "median_store_bytes": statistics.median(r["store_bytes"] for r in base)}
    receipt["candidate"] = {"median_wall_seconds": statistics.median(r["wall_seconds"] for r in cand),
                            "median_store_bytes": statistics.median(r["store_bytes"] for r in cand)}
    receipt["wall_change"] = receipt["candidate"]["median_wall_seconds"] / receipt["baseline"]["median_wall_seconds"] - 1
    receipt["store_change"] = receipt["candidate"]["median_store_bytes"] / receipt["baseline"]["median_store_bytes"] - 1
    receipt["gate"] = "PASS" if equivalent and receipt["wall_change"] <= .15 and receipt["store_change"] <= .20 else "FAIL"
    save(args.out, receipt)
    print(f"COST GATE {receipt['gate']}: wall {receipt['wall_change']:+.3%}, store {receipt['store_change']:+.3%}, work_equivalent={equivalent}", flush=True)
    if receipt["gate"] != "PASS":
        raise SystemExit(1)


if __name__ == "__main__":
    main()
