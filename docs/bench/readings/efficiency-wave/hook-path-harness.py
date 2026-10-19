#!/usr/bin/env python3
"""EW-75 / EW-76: drive every hook that calls the daemon, with fixed payloads,
against a scratch daemon over a scratch store, unloaded and loaded, and read
each call's latency and outcome from that daemon's own call log.

usage: hook-path-harness.py --bin <fathom> --subject <checkout> --home <scratch dir> \
           --store <scratch store> [--n 20] [--cold 8] [--label base] [--ingest-store <second store>] [--load-bin <fathom>]
       hook-path-harness.py --bin <fathom> --subject <checkout> --home <scratch dir> \
           --url <running daemon> --call-log <its call log> [--n 20] [--ingest-store <home>/<store>] [--load-bin <fathom>]

`--store` is a store already ingested from `--subject` (one scratch ingest,
reused by every reading so the index is the same for the base and the fix).
Everything runs under a private FATHOM_HOME (`--home`); the daemon is a
scratch `serve-http` on a free loopback port with its own call log. The
shared daemon and the real ~/.fathom are never touched: FATHOM_HTTP and
--url name the scratch daemon on every invocation.

`--url` drives an already-running daemon instead, read only: no daemon is
started, stopped or restarted (the cold half is skipped), FATHOM_HTTP and
each hook's --url name it, and the summary reads only this run's rows from
its `--call-log` (opened read only, from its size at the start): rows whose
X-Fathom-Caller names this run's session (`hook-path-harness-<label>-<t>`)
or whose parent process is this script. The loaded half's ingest runs
without FATHOM_HTTP into an `--ingest-store` inside `--home`. A file the
daemon reads index_stale stops the run before it starts, and the guarded
file is re-read before every pre-edit invocation, since the pre-edit hook
would ask the daemon to refresh it (a store write). Pass a --probe-timeout
longer than a graph build, so a direct probe never abandons one. The daemon's graphstore cache
events and tenancy level are read from /metrics at the start, after each
condition's hook loop and after its probes.

Hooks driven (one fresh process each, as the harness runs them):
  test-run    session-hook --event failed-test-run   (a failing `go test`)  -> localize
  edit-post   edit-hook (PostToolUse Edit)                                   -> list_services, obligations
  edit-pre    edit-hook --pre (PreToolUse Edit on a guarded file)            -> list_services, obligations, blast_radius
  prompt      prompt-hook (a prompt naming a file and a symbol)              -> find_symbol, obligations
  session     session-hook (SessionStart)                                    -> list_services, query_symbols
  stop        stop-hook (a working-tree plan claim)                          -> verify_plan

After each condition's hook loop, a direct probe calls each hook-path tool
with the hook's own arguments and a 60 s client, declaring origin
`probe-hook` (admitted as a hook), so a call the hook abandons still has its
full latency read; the probes never run under a measured hook.

Cold: `--cold K` restarts the daemon K times and runs the test-run hook as
the first request after each start, then the direct localize probe.

Conditions: `unloaded` is the machine as it is (other work on it is not this
script's to control; its load average is recorded), `loaded` runs the same
loop beside a scratch ingest of `--subject` into `--ingest-store`, restarted
whenever it finishes until the loop is done. Hold the wave's ingest lock
around the whole run (both halves).

Prints one JSON line per invocation and probe, then the summary tables.
"""
import argparse, json, os, shutil, signal, socket, statistics, subprocess, sys, time, urllib.request

HOOK_ORIGINS = ("test-run-hook", "edit-hook", "prompt-hook", "session-hook", "stop-hook", "prime")

FAILING_TEST = "TestDaemonTestRunIndexQualifiesService"
FAILING_OUTPUT = ("--- FAIL: %s (0.00s)\n"
                  "    testrunhook_test.go:370: test_impact: tests [] err <nil> args []\n"
                  "FAIL\n"
                  "FAIL\tgithub.com/geoffmilleraz/fathom/cmd/fathom\t0.412s\n"
                  "FAIL\n") % FAILING_TEST
POST_FILE = "internal/mcp/admission.go"
PRE_FILE = "cmd/fathom/statusz.go"  # .fathom/guard.cue names it
PROMPT = "Why does settleCall in internal/mcp/admission.go record a call as abandoned?"
CLAIM_PLAN = "docs/plans/efficiency-wave.md"
CLAIM_ROW = "- [ ] (op) EW-76:"  # an open row: EW-75 is closed
GRAPH_TOOLS = ("blast_radius", "localize", "test_impact", "verify_plan")


def pct(xs, p):
    xs = sorted(xs)
    if not xs:
        return None
    k = (len(xs) - 1) * p
    f = int(k)
    c = min(f + 1, len(xs) - 1)
    return round(xs[f] + (xs[c] - xs[f]) * (k - f))


def free_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    p = s.getsockname()[1]
    s.close()
    return p


def loadavg():
    return round(os.getloadavg()[0], 1)


def read_rows(path):
    rows = []
    if os.path.exists(path):
        with open(path) as f:
            for line in f:
                line = line.strip()
                if line:
                    try:
                        rows.append(json.loads(line))
                    except ValueError:
                        pass
    return rows


class Daemon:
    def __init__(self, a, env, call_log):
        self.a, self.env, self.call_log, self.p = a, env, call_log, None
        self.port = free_port()
        self.url = "http://127.0.0.1:%d" % self.port

    def start(self):
        self.p = subprocess.Popen([self.a.bin, "serve-http", "--store", self.a.store, "--addr", "127.0.0.1:%d" % self.port,
                                   "--call-log", self.call_log, "--auto-refresh=false"],
                                  env=self.env, cwd=self.a.subject, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        with open(os.path.join(self.a.home, "daemon.pid"), "w") as f:
            f.write(str(self.p.pid))
        for _ in range(600):
            try:
                with urllib.request.urlopen(self.url + "/healthz", timeout=1) as r:
                    if r.status == 200:
                        return
            except Exception:
                time.sleep(0.1)
        sys.exit("scratch daemon did not answer /healthz")

    def stop(self):
        if self.p and self.p.poll() is None:
            self.p.send_signal(signal.SIGTERM)
            try:
                self.p.wait(15)
            except subprocess.TimeoutExpired:
                self.p.kill()
                self.p.wait()
        self.p = None


def run_hook(argv, env, payload, cwd):
    t0 = time.monotonic()
    p = subprocess.Popen(argv, env=env, cwd=cwd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    out, _ = p.communicate(payload.encode())
    return {"exit": p.returncode, "wall_ms": round((time.monotonic() - t0) * 1000), "out_bytes": len(out.strip()), "pid": p.pid}


def probe(url, tool, args, timeout=60, caller=""):
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": tool, "arguments": args}}).encode()
    headers = {"Content-Type": "application/json", "X-Fathom-Origin": "probe-hook"}
    if caller:
        headers["X-Fathom-Caller"] = caller
    req = urllib.request.Request(url + "/rpc", data=body, headers=headers)
    t0 = time.monotonic()
    ok, raw = True, b""
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            raw = r.read()
            ok = b'"error"' not in raw[:200]
    except Exception:
        ok = False
    answer = "declined" if b"would not finish inside" in raw else "building" if (b"still building" in raw or b"did not finish building" in raw) else ""
    return {"wall_ms": round((time.monotonic() - t0) * 1000), "ok": ok, "answer": answer, "stale": b"index_stale" in raw}


def metrics(url):
    """The daemon's graphstore cache events and tenancy level, from /metrics (a read)."""
    out = {}
    try:
        with urllib.request.urlopen(url + "/metrics", timeout=10) as r:
            for line in r.read().decode().splitlines():
                if line.startswith('fathom_cache_events_total{group="graphstore",'):
                    out[line.split('event="', 1)[1].split('"', 1)[0]] = float(line.rsplit(" ", 1)[1])
                elif line.startswith("fathom_tenancy_level "):
                    out["tenancy_level"] = float(line.rsplit(" ", 1)[1])
    except Exception as e:
        out["error"] = str(e)
    return out


def read_rows_from(path, offset):
    """Call-log rows written after byte offset; the whole file if it rotated since."""
    rows, rotated = [], False
    with open(path, "rb") as f:
        if os.fstat(f.fileno()).st_size < offset:
            offset, rotated = 0, True
        f.seek(offset)
        for line in f:
            try:
                rows.append(json.loads(line))
            except ValueError:
                pass
    return rows, rotated


def answer_class(row):
    """How a hook graph call ended, from its call-log row."""
    o, err = row.get("outcome", ""), row.get("error", "")
    if o.startswith("abandoned"):
        return "abandoned"
    if o == "refused":
        return "refused"
    if "would not finish inside" in err or "(declined)" in err:
        return "declined"
    if o == "error" or err:
        return "error"
    if row.get("tool") == "verify_plan" and not row.get("verdict_outcome"):
        return "building"  # verify_plan's stage partial: still building, or declined (the row does not say which)
    return "answered"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--bin", required=True)
    ap.add_argument("--subject", required=True)
    ap.add_argument("--home", required=True)
    ap.add_argument("--store", default="", help="a scratch store ingested from --subject (required without --url)")
    ap.add_argument("--url", default="", help="drive this already-running daemon, read only: nothing is started, stopped or restarted")
    ap.add_argument("--call-log", default="", help="with --url: that daemon's call log, read only")
    ap.add_argument("--claim-row", default=CLAIM_ROW, help="the open plan row the stop hook's claim checks")
    ap.add_argument("--n", type=int, default=20)
    ap.add_argument("--cold", type=int, default=8)
    ap.add_argument("--probes", type=int, default=8, help="rounds of direct probes per condition, after its hook loop")
    ap.add_argument("--probe-timeout", type=int, default=60,
                    help="the direct probes' client timeout (s); with --url, longer than a graph build, so a probe never abandons one")
    ap.add_argument("--label", default="run")
    ap.add_argument("--ingest-store", default="")
    ap.add_argument("--load-bin", default="", help="the binary the loaded half's ingest runs (default --bin); one fixed load for every reading")
    ap.add_argument("--conditions", default="unloaded,loaded")
    a = ap.parse_args()
    a.subject, a.home = os.path.abspath(a.subject), os.path.abspath(a.home)
    if a.url:
        a.url = a.url.rstrip("/")
        if not a.call_log or not os.path.isfile(a.call_log):
            sys.exit("--url needs --call-log, the daemon's own call log (read only)")
        if a.ingest_store and os.path.commonpath([a.home, os.path.abspath(a.ingest_store)]) != a.home:
            sys.exit("--url: --ingest-store must sit inside --home, a scratch store this run owns")
        if a.cold:
            print("--url: the cold half restarts the daemon, so it is skipped", file=sys.stderr)
            a.cold = 0
    elif not a.store:
        sys.exit("--store is required without --url")
    a.store = os.path.abspath(a.store) if a.store else ""
    load_start = os.getloadavg()
    real = os.path.realpath(os.path.expanduser("~/.fathom"))
    if os.path.realpath(a.home) == real or os.path.commonpath([a.home, a.subject]) == a.home:
        sys.exit("refusing %s: it is the real home or holds the subject" % a.home)
    shutil.rmtree(a.home, ignore_errors=True)
    os.makedirs(a.home)
    fhome = os.path.join(a.home, "fathom-home")
    os.makedirs(fhome)
    transcript = os.path.join(a.home, "transcript.jsonl")
    open(transcript, "w").close()

    env = {k: v for k, v in os.environ.items() if not k.startswith("FATHOM_") and k != "CLAUDE_PROJECT_DIR"}
    # TMPDIR holds the hook breaker's shared state and the identity cache
    # (internal/hookcache): private to this run, so it starts closed.
    tmp = os.path.join(a.home, "tmp")
    os.makedirs(tmp)
    env.update(FATHOM_HOME=fhome, CLAUDE_PROJECT_DIR=a.subject, TMPDIR=tmp)
    if a.url:
        call_log, d, url = os.path.abspath(a.call_log), None, a.url
        log_offset = os.path.getsize(call_log)
    else:
        call_log = os.path.join(a.home, "mcp-calls.jsonl")
        d = Daemon(a, env, call_log)
        url = d.url
    env["FATHOM_HTTP"] = url
    sub = a.subject
    sid = "hook-path-harness"
    caller = ""
    if a.url:
        sid = "hook-path-harness-%s-%d" % (a.label, int(time.time()))
        caller = "pid=%d;session=%s" % (os.getpid(), sid)

    # The stop hook speaks only for a plan box claimed since HEAD: claim one
    # row in the subject's working tree for the run, restored at the end.
    plan = os.path.join(sub, CLAIM_PLAN)
    plan_orig = open(plan).read()
    if a.claim_row not in plan_orig:
        sys.exit("claim row %r not found in %s" % (a.claim_row, CLAIM_PLAN))

    hooks = {
        "test-run": ([a.bin, "session-hook", "--event", "failed-test-run", "--url", url, "--dir", sub],
                     json.dumps({"session_id": sid, "cwd": sub, "hook_event_name": "PostToolUseFailure", "tool_name": "Bash",
                                 "tool_input": {"command": "go test ./cmd/fathom/ -run " + FAILING_TEST},
                                 "tool_use_id": "t1", "error": "Exit code 1\n" + FAILING_OUTPUT, "is_interrupt": False, "duration_ms": 900})),
        "edit-post": ([a.bin, "edit-hook", "--url", url],
                      json.dumps({"session_id": sid, "cwd": sub, "hook_event_name": "PostToolUse", "tool_name": "Edit",
                                  "tool_input": {"file_path": os.path.join(sub, POST_FILE), "old_string": "a", "new_string": "a"},
                                  "tool_response": {"filePath": os.path.join(sub, POST_FILE), "success": True}})),
        "edit-pre": ([a.bin, "edit-hook", "--pre", "--url", url],
                     json.dumps({"session_id": sid, "cwd": sub, "hook_event_name": "PreToolUse", "tool_name": "Edit",
                                 "tool_input": {"file_path": os.path.join(sub, PRE_FILE), "old_string": "a", "new_string": "a"}})),
        "prompt": ([a.bin, "prompt-hook", "--url", url],
                   json.dumps({"session_id": sid, "cwd": sub, "hook_event_name": "UserPromptSubmit", "prompt": PROMPT})),
        "session": ([a.bin, "session-hook", "--url", url, "--dir", sub],
                    json.dumps({"session_id": sid, "transcript_path": transcript, "cwd": sub, "hook_event_name": "SessionStart", "source": "startup"})),
        "stop": ([a.bin, "stop-hook", "--url", url],
                 json.dumps({"session_id": sid, "transcript_path": transcript, "cwd": sub, "hook_event_name": "Stop", "stop_hook_active": False})),
    }
    probes = {
        "localize": {"service": "fathom", "failing_test": FAILING_TEST, "budget": 4},
        "obligations": {"service": "fathom", "file": POST_FILE, "include_snippets": False, "layer": "all"},
        "query_symbols": {"service": "fathom", "concern": "main", "top": 1},
        "blast_radius": {"service": "fathom", "changed_files": [PRE_FILE]},
        "find_symbol": {"service": "fathom", "name": "settleCall"},
    }

    snaps, stopped = [], []

    def stale_args(f):
        return {"service": "fathom", "file": f, "include_snippets": False, "layer": "all"}

    if a.url:
        try:
            with urllib.request.urlopen(url + "/healthz", timeout=5) as r:
                if r.status != 200:
                    sys.exit("%s/healthz answered %d; not run" % (url, r.status))
        except Exception as e:
            sys.exit("%s/healthz did not answer (%s); not run" % (url, e))
        # The pre-edit hook asks the daemon to refresh a file it reads stale:
        # a store write. Refuse to start when either hook file reads stale.
        for f in (PRE_FILE, POST_FILE):
            if probe(url, "obligations", stale_args(f), 30, caller)["stale"]:
                sys.exit("%s reads index_stale on %s: the pre-edit hook would ask it to refresh the file; not run" % (url, f))
        snaps.append(("start", time.time(), metrics(url)))

    results = []

    def emit(r):
        results.append(r)
        print(json.dumps(r), flush=True)

    ingest = {"p": None, "runs": 0}

    def ingest_tick():
        if not a.ingest_store:
            return
        if ingest["p"] is None or ingest["p"].poll() is not None:
            if ingest["p"] is not None:
                ingest["runs"] += 1
            for suffix in ("", "-wal", "-shm"):
                try:
                    os.remove(a.ingest_store + suffix)
                except FileNotFoundError:
                    pass
            ienv = {k: v for k, v in env.items() if not (a.url and k == "FATHOM_HTTP")}
            ingest["p"] = subprocess.Popen([a.load_bin or a.bin, "ingest", "--store", a.ingest_store, "--service", "fathom", "--path", sub],
                                           env=ienv, cwd=sub, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    try:
        with open(plan, "w") as f:
            f.write(plan_orig.replace(a.claim_row, a.claim_row.replace("[ ]", "[x]"), 1))
        # Cold: the first requests after a daemon start.
        for i in range(a.cold):
            d.start()
            la = loadavg()
            r = run_hook(hooks["test-run"][0], env, hooks["test-run"][1], sub)
            r.update(kind="hook", hook="test-run", condition="cold", i=i, load=la, label=a.label, t=time.time())
            emit(r)
            d.stop()
            d.start()
            r = probe(url, "localize", probes["localize"])
            r.update(kind="probe", tool="localize", condition="cold", i=i, load=loadavg(), label=a.label, t=time.time())
            emit(r)
            d.stop()
        if d:
            d.start()
        # Warm-up, not counted: one of each hook.
        for name, (argv, payload) in hooks.items():
            run_hook(argv, env, payload, sub)
        marks = {}
        for cond in a.conditions.split(","):
            if cond == "loaded":
                if not a.ingest_store:
                    continue
                ingest_tick()
                time.sleep(5)  # past the ingest's start-up
            marks[cond] = [time.time(), None]
            for i in range(a.n):
                for name, (argv, payload) in hooks.items():
                    if cond == "loaded":
                        ingest_tick()
                    # The guarded file can go stale mid-run (the daemon's
                    # checkout moved): stop before the pre-edit hook would
                    # ask the daemon to refresh it.
                    if a.url and name == "edit-pre" and not stopped and probe(url, "obligations", stale_args(PRE_FILE), 30, caller)["stale"]:
                        stopped.append("%s read index_stale on %s before %s edit-pre %d; the run stopped there" %
                                       (url, PRE_FILE, cond, i))
                    if stopped:
                        break
                    la = loadavg()
                    r = run_hook(argv, env, payload, sub)
                    r.update(kind="hook", hook=name, condition=cond, i=i, load=la, label=a.label, t=time.time())
                    emit(r)
                if stopped:
                    break
            marks[cond][1] = time.time()
            if a.url:
                snaps.append((cond + " hooks", time.time(), metrics(url)))
            if stopped:
                break
            # The probes run after the hook loop, so their heavy calls never
            # load the daemon under a measured hook.
            for i in range(a.probes):
                for tool, args in probes.items():
                    if cond == "loaded":
                        ingest_tick()
                    r = probe(url, tool, args, a.probe_timeout, caller)
                    r.update(kind="probe", tool=tool, condition=cond, i=i, load=loadavg(), label=a.label, t=time.time())
                    emit(r)
            if a.url:
                snaps.append((cond + " probes", time.time(), metrics(url)))
            if cond == "loaded" and ingest["p"] is not None:
                if ingest["p"].poll() is None:
                    ingest["p"].send_signal(signal.SIGTERM)
                    try:
                        ingest["p"].wait(60)
                    except subprocess.TimeoutExpired:
                        ingest["p"].kill()
                        ingest["p"].wait()
                else:
                    ingest["runs"] += 1
    finally:
        with open(plan, "w") as f:
            f.write(plan_orig)
        if d:
            d.stop()
        if ingest["p"] is not None and ingest["p"].poll() is None:
            ingest["p"].kill()
            ingest["p"].wait()

    # The call log, by condition (rows inside each condition's window).
    if a.url:
        me = os.getpid()
        rows, rotated = read_rows_from(call_log, log_offset)
        mine = lambda c: c.get("session") == sid or c.get("ppid") == me or c.get("pid") == me
        rows = [r for r in rows if mine(r.get("caller") or {})]
        load_end = os.getloadavg()
        print()
        print("daemon %s, session %s: %d call-log rows are this run's%s; daemon versions in them: %s" %
              (url, sid, len(rows), " (the log rotated during the run: rows before it are not read)" if rotated else "",
               ", ".join(sorted({r.get("fathom_version", "?") for r in rows})) or "none"))
        if stopped:
            print("STOPPED: " + stopped[0])
        gone = [name for name, _, m in snaps if "error" in m]
        if gone:
            print("VOID: the daemon did not answer /metrics at: " + ", ".join(gone))
        print("load average (1/5/15 min): start %.1f / %.1f / %.1f, end %.1f / %.1f / %.1f" % (load_start + load_end))
        print("graphstore cache events (/metrics, the whole daemon), each snapshot's change from the one before:")
        keys = sorted({k for _, _, m in snaps for k in m if k not in ("tenancy_level", "error")})
        for i, (name, t, m) in enumerate(snaps):
            prev = snaps[i - 1][2] if i else {}
            delta = " ".join("%s %+d" % (k, m.get(k, 0) - prev.get(k, 0)) for k in keys if i and m.get(k, 0) != prev.get(k, 0))
            print("  %-16s %s  tenancy_level %s  %s%s" % (name, time.strftime("%H:%M:%SZ", time.gmtime(t)), m.get("tenancy_level"),
                                                     delta if i else " ".join("%s %d" % (k, m.get(k, 0)) for k in keys), m.get("error", "")))
        if len(snaps) > 1:
            first, last = snaps[0][2], snaps[-1][2]
            print("  whole run: " + " ".join("%s %+d" % (k, last.get(k, 0) - first.get(k, 0)) for k in keys))
    else:
        rows = read_rows(call_log)
    print()
    print("ingest runs started beside the loaded loop: %d" % (ingest["runs"]))
    print()
    print("call log, each row attributed to the hook invocation it started inside: condition / hook / tool:")
    print("  n, abandoned (share), p50 / p95 latency_ms")
    from datetime import datetime
    def ts(r):
        return datetime.fromisoformat(r["ts"].replace("Z", "+00:00")).timestamp()
    spans = sorted((r["t"] - r["wall_ms"] / 1000.0, r["t"], r) for r in results if r["kind"] == "hook")
    # A row whose X-Fathom-Caller pid is a measured hook's process is that
    # invocation's; the time spans place the rest (back-to-back hooks put a
    # call at the tail of one span within 20 ms of the next one's start).
    by_pid = {r["pid"]: r for r in results if r["kind"] == "hook" and r.get("pid")}
    groups = {}
    for row in rows:
        if row.get("origin", "") not in HOOK_ORIGINS:
            continue
        start = ts(row) - row.get("latency_ms", 0) / 1000.0
        inv = by_pid.get((row.get("caller") or {}).get("pid"))
        if inv is not None:
            groups.setdefault((inv["condition"], inv["hook"], row["tool"]), []).append(row)
            continue
        # the invocation that started last at or before the call began (hooks run one at a time)
        owner = None
        for s0, s1, inv in spans:
            if s0 <= start + 0.02:
                owner = (s0, s1, inv)
            else:
                break
        if owner is not None and start <= owner[1] + 0.02:
            inv = owner[2]
            groups.setdefault((inv["condition"], inv["hook"], row["tool"]), []).append(row)
        else:
            groups.setdefault(("unattributed", row.get("origin"), row["tool"]), []).append(row)
    order = {"cold": 0, "unloaded": 1, "loaded": 2}
    for k in sorted(groups, key=lambda k: (order.get(k[0], 9), str(k[1]), k[2])):
        v = groups[k]
        ab = sum(1 for r in v if r.get("outcome") == "abandoned")
        lat = [r["latency_ms"] for r in v]
        print("  %-12s %-14s %-15s n %3d  abandoned %3d (%5.1f%%)  p50 %6s  p95 %6s" %
              (k[0], k[1], k[2], len(v), ab, 100.0 * ab / len(v), pct(lat, .5), pct(lat, .95)))
    print()
    print("per hook-path tool (all hooks), by condition: n, abandoned (share)")
    for cond in ["cold"] + list(marks):
        tools = {}
        for k, v in groups.items():
            if k[0] == cond:
                tools.setdefault(k[2], []).extend(v)
        for t in sorted(tools):
            v = tools[t]
            ab = sum(1 for r in v if r.get("outcome") == "abandoned")
            print("  %-9s %-15s n %3d  abandoned %3d (%5.1f%%)" % (cond, t, len(v), ab, 100.0 * ab / len(v)))
    print()
    print("hook graph calls (%s), by condition: n, answered (share), building, declined, abandoned, refused, error" % ", ".join(GRAPH_TOOLS))
    for cond in ["cold"] + list(marks):
        allv = []
        for t in GRAPH_TOOLS:
            v = [r for k, rs in groups.items() if k[0] == cond and k[2] == t for r in rs]
            allv += v
            if v:
                c = [answer_class(r) for r in v]
                print("  %-9s %-15s n %3d  answered %3d (%5.1f%%)  building %3d  declined %3d  abandoned %3d  refused %3d  error %3d" %
                      ((cond, t, len(v), c.count("answered"), 100.0 * c.count("answered") / len(v)) + tuple(c.count(x) for x in ("building", "declined", "abandoned", "refused", "error"))))
        if allv:
            n = sum(1 for r in allv if answer_class(r) == "answered")
            print("  %-9s %-15s n %3d  answered %3d (%5.1f%%)" % (cond, "all", len(allv), n, 100.0 * n / len(allv)))
    print()
    print("hooks: condition / hook: n, wall p50 / p95 (ms), share that injected, load average range")
    for cond in ["cold"] + list(marks):
        for name in hooks:
            v = [r for r in results if r["kind"] == "hook" and r["hook"] == name and r["condition"] == cond]
            if not v:
                continue
            w = [r["wall_ms"] for r in v]
            inj = sum(1 for r in v if r["out_bytes"] > 0)
            print("  %-9s %-10s n %3d  wall p50 %6s  p95 %6s  injected %3d/%-3d  load %.1f-%.1f" %
                  (cond, name, len(v), pct(w, .5), pct(w, .95), inj, len(v), min(r["load"] for r in v), max(r["load"] for r in v)))
    print()
    print("beats (the private hooks.jsonl): condition / channel / kind [reason]: count")
    beats = read_rows(os.path.join(fhome, "hooks.jsonl"))
    windows = [("cold", 0, min(m[0] for m in marks.values()))] + [(c, t0, t1) for c, (t0, t1) in marks.items()]
    for cond, t0, t1 in windows:
        counts = {}
        for b in beats:
            if not (t0 <= ts({"ts": b.get("at", "1970-01-01T00:00:00Z")}) <= t1):
                continue
            meta = b.get("meta") or {}
            if b.get("kind") in ("seen",):
                continue
            key = (b.get("channel"), b.get("kind"), meta.get("reason", "") if meta.get("emitted") == "false" else "",
                   ("deadline " + meta["deadline"]) if meta.get("deadline") else "", ("localize " + meta["localize"]) if meta.get("localize") else "")
            counts[key] = counts.get(key, 0) + 1
        for k in sorted(counts, key=lambda k: tuple(str(x) for x in k)):
            print("  %-9s %-16s %-14s %s: %d" % (cond, k[0], k[1], " ".join(x for x in k[2:] if x), counts[k]))
    print()
    print("direct probes (60 s client, origin probe-hook): condition / tool: n, p50 / p95 / max (ms), failed, building, declined")
    for cond in ["cold"] + list(marks):
        for tool in probes:
            v = [r for r in results if r["kind"] == "probe" and r["tool"] == tool and r["condition"] == cond]
            if not v:
                continue
            w = [r["wall_ms"] for r in v]
            print("  %-9s %-14s n %3d  p50 %6s  p95 %6s  max %6s  failed %d  building %d  declined %d" %
                  (cond, tool, len(v), pct(w, .5), pct(w, .95), max(w), sum(1 for r in v if not r["ok"]),
                   sum(1 for r in v if r.get("answer") == "building"), sum(1 for r in v if r.get("answer") == "declined")))


if __name__ == "__main__":
    main()
