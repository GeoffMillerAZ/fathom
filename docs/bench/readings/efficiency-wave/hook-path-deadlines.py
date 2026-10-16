#!/usr/bin/env python3
"""EW-75: what the hook-path calls cost against their callers' deadlines, from
the daemon's call log and the hook-latency log (read-only copies; statistics
only, no prompt, path or argument content is read or printed).

usage: hook-path-deadlines.py <mcp-calls.jsonl> <hook-latency.jsonl.1> <hook-latency.jsonl>

The call log's rows carry the tool, latency_ms, the X-Fathom-Origin header
(origin), and since 2026-09-27T14:12Z an outcome; an abandoned row's error
names its cause ("the client went away" vs "the call's deadline passed",
internal/mcp/admission.go settleCall). An abandoned call's latency_ms is the
time the handler took to notice the dead context, so it sits at the caller's
deadline and is a lower bound for the work, never the work itself.

The load gauge is EW-6's (hook-latency.md): the p90 wall time of the
edit-hook rows of hook-latency.jsonl in the 120 s either side of a call
(at least 5 rows), so only calls inside that log's window are gauged; idle
is the lowest third of the gauged hook-path calls, loaded the highest third.
"""
import collections, json, sys
from bisect import bisect_left, bisect_right
from datetime import datetime, timezone

HOOK_ORIGINS = ("test-run-hook", "edit-hook", "prompt-hook", "session-hook", "stop-hook", "prime")

# The caller's own deadline per (origin, tool), as the code reads at 99aa205a.
DEADLINES = {
    ("test-run-hook", "localize"): "2 s client (callTestRunTool) inside a 4 s hook deadline (TimeoutSec 5)",
    ("test-run-hook", "test_impact"): "2 s client inside the same 4 s",
    ("edit-hook", "obligations"): "2 s client (post: renderPostEdit; pre: what is left of a shared 2 s, at least 200 ms) in a 4 s hook",
    ("edit-hook", "list_services"): "2 s client (candidatesWithDaemon)",
    ("edit-hook", "blast_radius"): "500 ms client (blastRadiusTimeout, --pre on a guarded file)",
    ("edit-hook", "verify_plan"): "5 s client (claim-moment verify) in a 4 s hook",
    ("session-hook", "query_symbols"): "2 s (liveExampleLine's own ceiling) in an 8 s hook (TimeoutSec 10)",
    ("session-hook", "list_services"): "2 s client",
    ("prime", "query_symbols"): "2 s (liveExampleLine, the same call from `fathom prime`)",
    ("prime", "list_services"): "2 s client",
    ("prompt-hook", "obligations"): "what is left of a shared 8 s deadline (promptHookDeadlineBudget)",
    ("prompt-hook", "find_symbol"): "what is left of the same 8 s",
    ("prompt-hook", "list_services"): "what is left of the same 8 s",
    ("stop-hook", "verify_plan"): "5 s client (stopHookVerifyTimeout) in an 8 s hook",
    ("stop-hook", "list_services"): "2 s client",
}


def pct(xs, p):
    xs = sorted(xs)
    if not xs:
        return None
    k = (len(xs) - 1) * p
    f = int(k)
    c = min(f + 1, len(xs) - 1)
    return round(xs[f] + (xs[c] - xs[f]) * (k - f))


def ts(s):
    return datetime.fromisoformat(s.replace("Z", "+00:00")).timestamp()


def read(path):
    with open(path) as f:
        for line in f:
            try:
                yield json.loads(line)
            except ValueError:
                pass


def fmt(v):
    return "-" if v is None else "{:,}".format(v)


def main():
    calls = [d for d in read(sys.argv[1]) if "outcome" in d]
    hooklat = []
    for p in sys.argv[2:]:
        hooklat.extend(d for d in read(p) if "at" in d)
    calls.sort(key=lambda d: d["ts"])
    print("call log: %d rows carry an outcome, %s to %s" % (len(calls), calls[0]["ts"][:16], calls[-1]["ts"][:16]))
    by_origin = collections.Counter(d.get("origin", "none") for d in calls)
    ab_origin = collections.Counter(d.get("origin", "none") for d in calls if d["outcome"] == "abandoned")
    total_ab = sum(ab_origin.values())
    print()
    print("## Abandonment by origin")
    print()
    print("| origin | calls | abandoned | share of its calls | share of all abandonments |")
    print("|---|---|---|---|---|")
    for o, n in sorted(by_origin.items(), key=lambda kv: -ab_origin[kv[0]]):
        print("| %s | %s | %s | %.1f%% | %.1f%% |" % (o, fmt(n), fmt(ab_origin[o]), 100.0 * ab_origin[o] / n, 100.0 * ab_origin[o] / total_ab if total_ab else 0))
    hook_ab = sum(ab_origin[o] for o in HOOK_ORIGINS)
    print()
    print("hook-path abandonments: %d of %d (%.1f%%); no origin (agent MCP clients): %d" % (hook_ab, total_ab, 100.0 * hook_ab / total_ab, ab_origin["none"]))

    groups = collections.defaultdict(list)
    for d in calls:
        groups[(d.get("origin", "none"), d["tool"])].append(d)
    print()
    print("## Per hook-path tool and origin")
    print()
    print("| origin | tool | calls | abandoned (share) | cause: client gone / deadline | completed p50 / p95 ms | all rows p50 / p95 ms | abandoned at p50 / p95 ms | caller's deadline |")
    print("|---|---|---|---|---|---|---|---|---|")
    for k in sorted(groups):
        if k[0] not in HOOK_ORIGINS:
            continue
        v = groups[k]
        ab = [d for d in v if d["outcome"] == "abandoned"]
        done = [d["latency_ms"] for d in v if d["outcome"] != "abandoned"]
        client = sum(1 for d in ab if "client went away" in d.get("error", ""))
        print("| %s | %s | %s | %s (%.1f%%) | %d / %d | %s / %s | %s / %s | %s / %s | %s |" % (
            k[0], k[1], fmt(len(v)), fmt(len(ab)), 100.0 * len(ab) / len(v), client, len(ab) - client,
            fmt(pct(done, .5)), fmt(pct(done, .95)), fmt(pct([d["latency_ms"] for d in v], .5)), fmt(pct([d["latency_ms"] for d in v], .95)),
            fmt(pct([d["latency_ms"] for d in ab], .5)), fmt(pct([d["latency_ms"] for d in ab], .95)), DEADLINES.get(k, "not read")))
    print()
    print("no-origin rows of the same tools (agent MCP clients, not a hook): ", end="")
    parts = []
    for t in ("localize", "symbol_context", "query_symbols", "obligations"):
        v = groups.get(("none", t), [])
        if v:
            ab = sum(1 for d in v if d["outcome"] == "abandoned")
            parts.append("%s %d of %d (%.1f%%)" % (t, ab, len(v), 100.0 * ab / len(v)))
    print("; ".join(parts))

    # G11's window, read per origin (the 2026-10-11 correction).
    print()
    print("## G11's window (2026-10-09 to 2026-10-11), per origin")
    print()
    print("| tool | origin | calls | abandoned |")
    print("|---|---|---|---|")
    for t in ("localize", "symbol_context", "query_symbols", "obligations"):
        w = collections.defaultdict(lambda: [0, 0])
        for d in calls:
            if d["tool"] == t and "2026-10-09" <= d["ts"][:10] <= "2026-10-11":
                o = d.get("origin", "none")
                w[o][0] += 1
                w[o][1] += d["outcome"] == "abandoned"
        for o in sorted(w, key=lambda o: -w[o][1]):
            print("| %s | %s | %d | %d |" % (t, o, w[o][0], w[o][1]))

    # The load gauge.
    edits = sorted((ts(d["at"]), d["ms"]) for d in hooklat if d.get("channel") == "edit-hook" and "ms" in d)
    et = [e[0] for e in edits]
    gauged = []
    for d in calls:
        if d.get("origin") not in HOOK_ORIGINS:
            continue
        t = ts(d["ts"])
        lo, hi = bisect_left(et, t - 120), bisect_right(et, t + 120)
        if hi - lo >= 5:
            gauged.append((pct([edits[i][1] for i in range(lo, hi)], .9), d))
    print()
    print("## Load: hook-path calls inside the hook-latency log's window (%s to %s)" % (
        datetime.fromtimestamp(et[0], timezone.utc).strftime("%Y-%m-%dT%H:%MZ") if et else "-", datetime.fromtimestamp(et[-1], timezone.utc).strftime("%Y-%m-%dT%H:%MZ") if et else "-"))
    if gauged:
        g = sorted(x[0] for x in gauged)
        lo_cut, hi_cut = g[len(g) // 3], g[(2 * len(g)) // 3]
        print()
        print("%d gauged calls; idle third: edit-hook p90 at most %d ms, loaded third: over %d ms" % (len(gauged), lo_cut, hi_cut))
        print()
        print("| origin | tool | idle: calls, abandoned, all-rows p50 ms | loaded: calls, abandoned, all-rows p50 ms |")
        print("|---|---|---|---|")
        sub = collections.defaultdict(lambda: {"idle": [], "loaded": []})
        for gv, d in gauged:
            if gv <= lo_cut:
                sub[(d["origin"], d["tool"])]["idle"].append(d)
            elif gv > hi_cut:
                sub[(d["origin"], d["tool"])]["loaded"].append(d)
        for k in sorted(sub):
            cells = []
            for c in ("idle", "loaded"):
                v = sub[k][c]
                ab = sum(1 for d in v if d["outcome"] == "abandoned")
                cells.append("%d, %d (%s), %s" % (len(v), ab, "%.0f%%" % (100.0 * ab / len(v)) if v else "-", fmt(pct([d["latency_ms"] for d in v], .5))))
            print("| %s | %s | %s | %s |" % (k[0], k[1], cells[0], cells[1]))

    # The hooks themselves.
    print()
    print("## Hook wall time against the hook's budget (hook-latency.jsonl)")
    print()
    print("| channel | budget_ms | invocations | p50 / p95 ms | past budget |")
    print("|---|---|---|---|---|")
    ch = collections.defaultdict(list)
    for d in hooklat:
        ch[(d.get("channel"), d.get("budget_ms"))].append(d["ms"])
    for k in sorted(ch, key=lambda k: (str(k[0]), k[1] or 0)):
        v = ch[k]
        over = sum(1 for x in v if k[1] and x > k[1])
        print("| %s | %s | %s | %s / %s | %d |" % (k[0], k[1] if k[1] else "none", fmt(len(v)), fmt(pct(v, .5)), fmt(pct(v, .95)), over))


if __name__ == "__main__":
    main()
