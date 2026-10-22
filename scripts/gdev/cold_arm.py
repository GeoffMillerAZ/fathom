#!/usr/bin/env python3
"""Evidence serialization for cold-arm.sh; Python standard library only."""
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import time


def dump(path, value):
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


def capture(argv, cwd=None):
    start = time.monotonic()
    p = subprocess.run(argv, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                       check=False, timeout=900)
    return {"argv": argv, "exit_code": p.returncode, "response_bytes": len(p.stdout),
            "wall_seconds": time.monotonic() - start,
            "stdout": p.stdout.decode(errors="replace"), "stderr": p.stderr.decode(errors="replace")}


def git(root, *args):
    r = capture(["git", "-C", str(root), *args])
    if r["exit_code"]:
        raise ValueError(r["stderr"])
    return r["stdout"].strip()


def rows(text):
    return [line for line in text.splitlines() if re.match(r"^\s*- \[[ xX~>!-]\] ", line)]


def row_id(line):
    m = re.search(r"\bid:\s*([^,}\s]+)", line)
    if not m:
        raise ValueError("plan row has no authored id: " + line)
    return m[1]


def lines(path):
    return [x.strip() for x in Path(path).read_text().splitlines()
            if x.strip() and not x.lstrip().startswith("#")]


def payload(name, arguments=None, method="tools/call"):
    params = {"name": name, "arguments": arguments} if arguments is not None else {}
    if method == "initialize":
        params = {"protocolVersion": "2024-11-05", "capabilities": {},
                  "clientInfo": {"name": "cold-arm", "version": "1"}}
    return {"jsonrpc": "2.0", "id": 1, "method": method, "params": params}


def prepare(args):
    subject, service, plans_file, ids_file, budget, output, binary = args
    root, out = Path(subject).resolve(), Path(output).resolve()
    if out == root or root in out.parents:
        raise ValueError("--out must be outside the subject to preserve its corpus")
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", service):
        raise ValueError("--service must be a logical service name")
    budget = int(budget)
    if budget <= 0 or budget > 2**63 - 1:
        raise ValueError("--budget must fit a positive int64")
    plans, ids = lines(plans_file), lines(ids_file)
    if not plans or not ids or len(set(plans)) != len(plans) or len(set(ids)) != len(ids):
        raise ValueError("lists must be nonempty and have no duplicates")
    texts, authored = [], []
    symbol = None
    for plan in plans:
        path = (root / plan).resolve()
        if Path(plan).is_absolute() or root not in path.parents:
            raise ValueError("plan paths must be relative and inside the subject")
        text = path.read_text()
        texts.append(text)
        for line in rows(text):
            authored.append(row_id(line))
            match = re.search(r"\bsymbol:\s*([^,}\s]+)", line)
            if symbol is None and match:
                symbol = match[1]
    if sorted(authored) != sorted(ids):
        raise ValueError("--ids must list every row id in the supplied plans exactly once")
    commit = git(root, "rev-parse", "HEAD")
    parent = capture(["git", "-C", str(root), "rev-parse", "--verify", "HEAD^"])
    since = parent["stdout"].strip() if parent["exit_code"] == 0 else commit
    changed_files = sorted(filter(None, git(root, "diff", "--name-only", since, commit).splitlines()))
    review_scope = "parent diff"
    if not changed_files:
        changed_files = sorted(git(root, "ls-tree", "-r", "--name-only", "HEAD").splitlines())
        review_scope = "tracked HEAD files; initial or empty-diff commit"
    subject_id = {"path": str(root), "commit": commit, "tree": git(root, "rev-parse", "HEAD^{tree}"),
                  "dirty": bool(git(root, "status", "--porcelain=v1", "--untracked-files=all")),
                  "status": git(root, "status", "--porcelain=v1", "--untracked-files=all"),
                  "review_since": since, "review_scope": review_scope, "review_files": changed_files}
    # mkdir without exist_ok is the arm's exclusive claim. Never reuse state.
    out.mkdir(parents=True)
    (out / "home").mkdir()
    (out / "payloads").mkdir()
    (out / "responses").mkdir()
    (out / "resources").mkdir()
    (out / "startup").mkdir()
    (out / "home/config.toml").write_text('[tenancy.unmanaged]\nbudget = "%dB"\n' % budget)
    os.environ["FATHOM_HOME"] = str(out / "home")
    (out / "plans.txt").write_text("\n".join(plans) + "\n")
    (out / "ids.txt").write_text("\n".join(ids) + "\n")
    selection = {"symbol": symbol or "__cold_arm_no_symbol_anchor__",
                 "context_plan": str(out / "context.md"), "context_id": "COLD-ARM-CONTEXT"}
    (out / "context.md").write_text("# Context probe\n- [ ] Resolve context {id: COLD-ARM-CONTEXT, ctx: %s}\n" % selection["symbol"])
    (out / "started.wall").write_text(str(time.time()))
    dump(out / "selection.json", selection)
    inputs = {"service": service, "budget_bytes": budget,
              "budget_configuration": "FATHOM_HOME/config.toml [tenancy.unmanaged].budget",
              "plans": plans, "ids": ids,
              "plan_sha256": {p: hashlib.sha256(t.encode()).hexdigest() for p, t in zip(plans, texts)},
              "selection": selection}
    identity = {"fathom": capture([binary, "version"]), "binary_sha256": hashlib.sha256(Path(binary).read_bytes()).hexdigest(),
                "subject": subject_id, "host": {"os": platform.system(), "arch": platform.machine()},
                "go": capture(["go", "version"]),
                "runtime_environment": {k: os.environ[k] for k in sorted(os.environ)
                                        if k in ("GOMEMLIMIT", "GOGC", "GODEBUG", "GOMAXPROCS", "GOFLAGS")}}
    dump(out / "identity.json", identity)
    dump(out / "inputs.json", inputs)
    calls = {"ingest": payload("ingest", {"service": service, "path": str(root)}),
             "initialize": payload("", method="initialize"),
             "tools-list": payload("", method="tools/list"),
             "symbol_context": payload("symbol_context", {"service": service, "symbol": selection["symbol"]}),
             "review_diff": payload("review_diff", {"service": service, "since": since,
                             "changed_files": changed_files}),
             "obligations": payload("obligations", {"service": service}),
             "verify_plan": payload("verify_plan", {"service": service, "plan_markdown": "\n\n".join(texts)})}
    for name, request in calls.items():
        dump(out / "payloads" / (name + ".json"), request)


def decode(raw, mode):
    if mode == "text":
        return raw.decode(errors="replace")
    return json.loads(raw)


def facts(value):
    """Inspect active envelope fields, including JSON carried inside MCP text."""
    refused, truncated, admission, errors = [], [], [], []
    def walk(v, path=""):
        if isinstance(v, dict):
            if v.get("outcome") == "refused" and ("level" in v or "class" in v):
                admission.append({"path": path, "value": v})
            for k, child in v.items():
                key = k.lower()
                where = path + "." + k
                if key in ("admission", "admission_state", "admissionstate"):
                    admission.append({"path": where, "value": child})
                if child and key in ("refused", "refusal", "refusal_reason"):
                    refused.append({"path": where, "value": child})
                if key in ("state", "status", "outcome") and isinstance(child, str) and child in ("refused", "busy", "cancelled", "canceled", "deadline_exceeded", "abandoned", "abandoned-by-governor"):
                    refused.append({"path": where, "value": child})
                if ((key in ("truncated", "elided", "elided_count", "omitted_count", "suppressed_count", "suppressed") and child)
                        or (key.endswith(("_truncated", "_elided", "_omitted")) and child)
                        or (key == "partial" and child)
                        or (key == "complete" and child is False)):
                    truncated.append({"path": where, "value": child})
                if (key == "error" and child) or (key == "iserror" and child is True):
                    errors.append({"path": where, "value": child})
                if key == "text" and isinstance(child, str):
                    try:
                        walk(json.loads(child), where)
                    except (ValueError, TypeError):
                        # Error text is retained through isError/error, not guessed
                        # from arbitrary source snippets containing "refuse".
                        pass
                elif key not in ("inputschema", "properties", "description", "snippet", "source", "plan_markdown"):
                    walk(child, where)
        elif isinstance(v, list):
            for i, child in enumerate(v):
                walk(child, path + "[%d]" % i)
    walk(value)
    # JSON-RPC errors often put named admission refusals in error.message.
    for e in errors:
        text = json.dumps(e["value"]).lower()
        if any(x in text for x in ("refus", "admission", "busy", "cancel", "abandon", "deadline", "warmup", "warming")):
            refused.append(e)
    return refused, truncated, admission, errors


def record(out, name, mode, request, argv, directory="responses", ledger="requests.jsonl", clock=time.monotonic):
    records = out / ledger
    index = len(records.read_text().splitlines()) if records.exists() else 0
    base = out / directory / ("%05d" % index)
    start = clock()
    with base.with_suffix(".stdout").open("wb") as stdout, base.with_suffix(".stderr").open("wb") as stderr:
        try:
            code = subprocess.run(argv, stdout=stdout, stderr=stderr, check=False, timeout=900).returncode
        except subprocess.TimeoutExpired:
            code = 124
    elapsed = clock() - start
    raw = base.with_suffix(".stdout").read_bytes()
    refused, truncated, admission, errors = [], [], [], []
    valid = True
    try:
        value = decode(raw, mode)
        refused, truncated, admission, errors = facts(value)
        if mode == "rpc" and (not isinstance(value, dict) or value.get("jsonrpc") != "2.0" or not ("result" in value or "error" in value) or value.get("id") != json.loads(Path(request).read_text())["id"]):
            valid = False
        if mode == "text":
            for line in value.splitlines():
                if re.search(r"\b(elided|truncated|unresolved)\b", line, re.I):
                    truncated.append({"line": line})
    except (ValueError, TypeError):
        valid = False
    if directory == "responses":
        sampled = entries(out, "samples.jsonl")
        if sampled:
            try:
                latest = json.loads((out / sampled[-1]["response_file"]).read_text())
                admission.append({"source": sampled[-1]["response_file"], "value": latest.get("tenancy"),
                                  "note": "latest sampled tenancy state, not a per-call admission decision"})
            except (ValueError, OSError):
                pass
        if not admission:
            admission.append({"source": "response", "value": "unknown; no admission state exposed"})
    status = ("refused" if refused else "error" if code or errors else
              "unknown" if not valid or not raw else "truncated" if truncated else "complete")
    # verify exit 1 is a failed gate, and may still be a complete JSON report.
    if name.startswith("verify:") and code == 1 and valid and not (errors or refused or truncated):
        status = "complete"
    entry = {"name": "tools/list" if name == "tools-list" else name, "argv": argv,
             "request": json.loads(Path(request).read_text()) if request != "-" else None,
             "exit_code": code, "response_bytes": len(raw),
             "response_sha256": hashlib.sha256(raw).hexdigest(),
             "stderr_bytes": base.with_suffix(".stderr").stat().st_size,
             "completeness": status, "refusal": refused, "admission": admission,
             "truncation": truncated, "errors": errors,
             "wall_seconds": elapsed, "response_file": str(base.with_suffix(".stdout").relative_to(out)),
             "stderr_file": str(base.with_suffix(".stderr").relative_to(out))}
    with records.open("a") as f:
        f.write(json.dumps(entry) + "\n")
    return entry


def ready(out, pid):
    # A startup bound, never a performance assertion. Only private records are read.
    for _ in range(600):
        for p in sorted((out / "home/daemon").glob("*/record.json")):
            try:
                rec = json.loads(p.read_text())
                if rec["pid"] != pid or not re.fullmatch(r"127\.0\.0\.1:[1-9][0-9]*", rec["addr"]):
                    continue
                url = "http://" + rec["addr"]
                (out / "url.txt").write_text(url + "\n")
                response = record(out, "healthz", "json", "-",
                                  ["curl", "--noproxy", "*", "--silent", "--show-error", "--fail-with-body",
                                   "--max-time", "1", url + "/healthz"], ledger="startup.jsonl", directory="startup")
                if response["exit_code"] == 0:
                    return url
            except (OSError, ValueError, KeyError):
                pass
        os.kill(pid, 0)
        time.sleep(0.1)
    raise ValueError("daemon did not become ready; inspect daemon.stderr")


def sample_once(out, binary, url):
    return record(out, "resources", "json", "-", [binary, "resources", "--json", "--url", url],
                  "resources", "samples.jsonl")


def entries(out, file):
    path = out / file
    return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []


def result_value(out, entry):
    value = json.loads((out / entry["response_file"]).read_text())
    result = value.get("result", {})
    if "structuredContent" in result:
        return result["structuredContent"]
    for block in result.get("content", []):
        if block.get("type") == "text":
            return json.loads(block["text"])
    return result


def ingest_state(out):
    entry = entries(out, "requests.jsonl")[-1]
    try:
        value = result_value(out, entry)
        run = value.get("run")
        if isinstance(run, dict):
            if run.get("done"):
                return "done" if run.get("ok", False) else "failed"
            return run["id"]
        if value.get("completed") is True or value.get("done") is True or value.get("ok") is True:
            return "done" if value.get("ok") is True else "failed"
        if value.get("state") == "unknown":
            return "failed"
        if run and entry["completeness"] not in ("refused", "error"):
            return run
    except (ValueError, KeyError, TypeError, AttributeError):
        pass
    return "failed"


def finish(out, harness_exit, clock=time.time):
    requests = entries(out, "requests.jsonl")
    samples = entries(out, "samples.jsonl")
    now, total, estimates, budgets = [], [], [], []
    def estimated(v, path=""):
        found = {}
        if isinstance(v, dict):
            for key, child in v.items():
                p = path + "." + key
                if re.search(r"estimat|memo|heap", key, re.I):
                    found[p] = child
                else:
                    found.update(estimated(child, p))
        return found
    for sample in samples:
        try:
            value = json.loads((out / sample["response_file"]).read_text())
            foot = value.get("footprint", {})
            budget = value.get("budget", {})
            if budget.get("budget", 0) > 0 and ("live daemon" in budget.get("source", "") or "ring header" in budget.get("source", "")):
                budgets.append(budget["budget"])
            if foot.get("now", -1) >= 0:
                now.append(foot["now"])
            if foot.get("samples", 0) > 0 and foot.get("max", -1) >= 0:
                total.append(foot["max"])
            fields = estimated(value)
            if fields:
                estimates.append({"response_file": sample["response_file"], "fields": fields})
        except (ValueError, TypeError):
            pass
    def read(file, default=""):
        p = out / file
        return p.read_text().strip() if p.exists() else default
    daemon = {"argv": read("daemon-argv.txt").splitlines(), "pid": int(read("daemon-pid.txt", "0")),
              "url": read("url.txt"), "exit_code": int(read("daemon-exit.txt", "-1")),
              "forced_stop": (out / "forced-stop").exists(), "harness_exit_code": harness_exit,
              "stdout_file": "daemon.stdout", "stderr_file": "daemon.stderr",
              "readiness_requests": entries(out, "startup.jsonl"),
              "arm_wall_seconds": clock() - float(read("started.wall")),
              "arm_wall_clock": "POSIX wall clock across processes; request durations use process-local monotonic clocks",
              "stop_signal": "SIGTERM",
              "ingest_state": read("ingest-state.txt", "unknown")}
    complete = (not harness_exit and daemon["exit_code"] in (0, 143) and not daemon["forced_stop"] and now
                and daemon["ingest_state"] == "done" and requests and all(r["completeness"] == "complete" and r["exit_code"] == 0 for r in requests))
    arm = {"schema_version": 1, "identity": json.loads((out / "identity.json").read_text()),
           "inputs": json.loads((out / "inputs.json").read_text()), "daemon": daemon, "requests": requests,
           "physical_footprint": {"method": "fathom resources --json --url PRIVATE_URL; poll every 0.25s plus final sample; daemon OS sampler/ring cadence applies",
                                  "peak_daemon_bytes": max(now) if now else None,
                                  "peak_with_children_bytes": max(total) if total else None,
                                  "budget_observed_bytes": sorted(set(budgets)),
                                  "budget_matches_requested": bool(budgets) and set(budgets) == {json.loads((out / "inputs.json").read_text())["budget_bytes"]},
                                  "resource_requests": len(samples), "known_daemon_samples": len(now),
                                  "samples_file": "samples.jsonl",
                                  "limitation": "sampled peak, not an allocation peak; sub-sample spikes and pre-record startup are not observed; unknown is null"},
           "estimated_memory": {"method": "separate estimated/memo/heap fields exposed by resources JSON",
                                "samples": estimates, "note": "no such fields exposed" if not estimates else "estimates, not physical footprint"},
           "outcome": "complete" if complete else "incomplete"}
    dump(out / "arm.json", arm)
    print(str(out / "arm.json"))


def main():
    command, *args = sys.argv[1:]
    if command == "prepare":
        prepare(args)
        return
    out = Path(args[0])
    if command == "ready": print(ready(out, int(args[1])))
    elif command == "run": record(out, args[1], args[2], args[3], args[4:])
    elif command == "select": print(json.loads((out / "selection.json").read_text())[args[1]])
    elif command == "ingest-state": print(ingest_state(out))
    elif command == "poll":
        service = json.loads((out / "inputs.json").read_text())["service"]
        dump(out / "payloads/ingest-poll.json", payload("ingest", {"service": service, "run": args[1]}))
    elif command == "sample-once": sample_once(out, args[1], args[2])
    elif command == "sample":
        while not (out / "stop-sampling").exists():
            if (out / "url.txt").exists():
                sample_once(out, args[1], (out / "url.txt").read_text().strip())
            time.sleep(0.25)
    elif command == "watchdog":
        time.sleep(30)
        try:
            os.kill(int(args[1]), 0)
            (out / "forced-stop").touch()
            os.kill(int(args[1]), 9)
        except ProcessLookupError:
            pass
    elif command == "finish": finish(out, int(args[1]))
    else: raise ValueError("unknown command " + command)


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, subprocess.SubprocessError) as exc:
        sys.exit("cold-arm: " + str(exc))
