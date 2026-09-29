#!/usr/bin/env python3
"""EW-29: run the session hook and the stop hook against a scratch daemon and
read each invocation's CPU beside its wall time (the O140 harness in the manner
of docs/.fathom/observations.md O140).

usage: hook-cpu-harness.py --bin <fathom> --repo <clone> --home <scratch dir> \
           [--n 12] [--spinners 0,16] [--real-home ~/.fathom] [--transcript <jsonl>]

Everything runs under a private FATHOM_HOME (`--home`): a copy of the real beat
log and session ledger (the stop hook's retro reads the whole beat log, so its
size is part of the cost), a scratch ingest of `--repo`, and a scratch
serve-http on a free loopback port. The shared daemon and the real ~/.fathom are
never written. Each invocation is a fresh process; its row comes from the
hook-latency log the binary writes (ms, cpu_ms) and is cross-checked against the
operating system's own accounting of the process tree (wait4's rusage: user +
sys, children reaped by the hook included) which also counts the runtime's
start-up and exit outside the guard.

Load is added by `--spinners K` busy processes (0 = the machine as it is). The
machine's own load average before and after each block is recorded, since other
work on the machine is not something this script controls.

Prints one JSON line per invocation and, last, a summary table.
"""
import argparse, json, os, resource, shutil, signal, socket, statistics, subprocess, sys, time


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
        for line in open(path):
            line = line.strip()
            if line:
                try:
                    rows.append(json.loads(line))
                except ValueError:
                    pass
    return rows


def run_hook(argv, env, payload, cwd, latency_path):
    before = len(read_rows(latency_path))
    t0 = time.monotonic()
    p = subprocess.Popen(argv, env=env, cwd=cwd, stdin=subprocess.PIPE, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    p.stdin.write(payload.encode())
    p.stdin.close()
    _, status, ru = os.wait4(p.pid, 0)
    wall = (time.monotonic() - t0) * 1000
    rows = read_rows(latency_path)[before:]
    return {
        "exit": os.waitstatus_to_exitcode(status),
        "wall_ext_ms": round(wall),
        "cpu_ext_ms": round((ru.ru_utime + ru.ru_stime) * 1000),
        "rows": len(rows),
        "ms": rows[-1]["ms"] if rows else None,
        "cpu_ms": rows[-1].get("cpu_ms") if rows else None,
        "channel": rows[-1]["channel"] if rows else None,
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--bin", required=True)
    ap.add_argument("--repo", required=True)
    ap.add_argument("--home", required=True)
    ap.add_argument("--n", type=int, default=12)
    ap.add_argument("--spinners", default="0,16", help="comma list of busy-process counts, one block each")
    ap.add_argument("--real-home", default=os.path.expanduser("~/.fathom"))
    ap.add_argument("--transcript", default="")
    ap.add_argument("--store-from", default="", help="a store already ingested from --repo, copied in place of a fresh ingest")
    a = ap.parse_args()

    home = os.path.abspath(a.home)
    repo = os.path.abspath(a.repo)
    if os.path.realpath(home) == os.path.realpath(a.real_home) or os.path.commonpath([home, repo]) == home:
        sys.exit("refusing to clear %s: it is the real home or holds the repository" % home)
    shutil.rmtree(home, ignore_errors=True)
    os.makedirs(home)
    for name in ("hooks.jsonl", "session-ledger.jsonl"):
        src = os.path.join(a.real_home, name)
        if os.path.exists(src):
            shutil.copyfile(src, os.path.join(home, name))
    transcript = os.path.join(home, "transcript.jsonl")
    if a.transcript:
        shutil.copyfile(a.transcript, transcript)
    else:
        open(transcript, "w").close()
    tail_lines = open(transcript).read().splitlines()[-20:]

    env = dict(os.environ, FATHOM_HOME=home, CLAUDE_PROJECT_DIR=repo)
    for k in list(env):
        if k.startswith("FATHOM_") and k not in ("FATHOM_HOME",):
            env.pop(k)
    service = os.path.basename(repo).lower()
    store = os.path.join(home, "scratch.sqlite")
    if a.store_from:
        shutil.copyfile(a.store_from, store)
    else:
        # exit 1 is "degraded" (for example a typed-edge package that failed to load): the store still answers
        r = subprocess.run([a.bin, "ingest", "--store", store, "--service", service, "--path", repo], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
        if r.returncode not in (0, 1) or not os.path.exists(store):
            sys.exit("ingest failed (%d): %s" % (r.returncode, r.stderr.decode()[-300:]))
    port = free_port()
    url = "http://127.0.0.1:%d" % port
    daemon = subprocess.Popen([a.bin, "serve-http", "--store", store, "--addr", "127.0.0.1:%d" % port, "--call-log", os.path.join(home, "mcp-calls.jsonl")],
                              env=env, cwd=repo, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    results = []
    try:
        for _ in range(100):
            try:
                socket.create_connection(("127.0.0.1", port), 0.2).close()
                break
            except OSError:
                time.sleep(0.1)
        latency = os.path.join(home, "hook-latency.jsonl")
        sid = "hook-cpu-harness-session"
        session_payload = json.dumps({"session_id": sid, "transcript_path": transcript, "cwd": repo, "hook_event_name": "SessionStart", "source": "startup"})
        stop_payload = json.dumps({"session_id": sid, "transcript_path": transcript, "cwd": repo, "hook_event_name": "Stop", "stop_hook_active": False})
        session_argv = [a.bin, "session-hook", "--url", url, "--dir", repo]
        stop_argv = [a.bin, "stop-hook", "--url", url]
        # Warm-up: the first run of each pays one-time cache fills; it is not counted.
        run_hook(session_argv, env, session_payload, repo, latency)
        run_hook(stop_argv, env, stop_payload, repo, latency)
        for block in [int(x) for x in a.spinners.split(",")]:
            spin = [subprocess.Popen([sys.executable, "-c", "while True: pass"]) for _ in range(block)]
            time.sleep(1.0)
            try:
                for verb, argv, payload in (("session-hook", session_argv, session_payload), ("stop-hook", stop_argv, stop_payload)):
                    for i in range(a.n):
                        if verb == "stop-hook" and tail_lines:
                            # a turn's worth of transcript growth, real lines, so each Stop folds something
                            with open(transcript, "a") as f:
                                f.write("\n".join(tail_lines) + "\n")
                        la = loadavg()
                        r = run_hook(argv, env, payload, repo, latency)
                        r.update(verb=verb, spinners=block, load_before=la, load_after=loadavg(), i=i)
                        results.append(r)
                        print(json.dumps(r), flush=True)
            finally:
                for s in spin:
                    s.kill()
                    s.wait()
    finally:
        daemon.send_signal(signal.SIGTERM)
        try:
            daemon.wait(10)
        except subprocess.TimeoutExpired:
            daemon.kill()

    print()
    print("verb          spinners  n   wall p50 (row)  cpu p50 (row)  cpu/wall  cpu p50 (os)  load avg range   rows w/o cpu_ms")
    for verb in ("session-hook", "stop-hook"):
        for block in sorted({r["spinners"] for r in results}):
            rs = [r for r in results if r["verb"] == verb and r["spinners"] == block]
            if not rs:
                continue
            timed = [r for r in rs if r["cpu_ms"] is not None and r["ms"] is not None]
            w = statistics.median(r["ms"] for r in timed) if timed else float("nan")
            c = statistics.median(r["cpu_ms"] for r in timed) if timed else float("nan")
            osc = statistics.median(r["cpu_ext_ms"] for r in rs)
            loads = [r["load_before"] for r in rs] + [r["load_after"] for r in rs]
            print("%-13s %8d %3d %10.0f ms %10.0f ms %8.2f %10.0f ms  %5.1f-%5.1f       %d" %
                  (verb, block, len(rs), w, c, (c / w if timed and w else float("nan")), osc, min(loads), max(loads), len(rs) - len(timed)))


if __name__ == "__main__":
    main()
