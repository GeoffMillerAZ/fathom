#!/usr/bin/env python3
"""EW-6: session-hook and stop-hook cost, from the hook-latency log only.

usage: hook-latency.py ~/.fathom/hook-latency.jsonl.1 ~/.fathom/hook-latency.jsonl

Reads the log (wall time `ms`, deadline `budget_ms`, invocation `inv`; it holds
no CPU time and no prompt content) and splits each session-hook and stop-hook
invocation by a load gauge computed from the same log: the p90 wall time of the
edit hook in the 120 seconds either side. The edit hook's work is a stat and a
store read (p50 3 ms), so when it slows the machine is slow. Idle is the lowest
third of the gauge, loaded the highest. Deterministic (fixed bootstrap seed).
"""
import bisect, datetime, json, math, random, sys


def ts(s):
    return datetime.datetime.fromisoformat(s.replace("Z", "+00:00")).timestamp()


def q(v, p):
    v = sorted(v)
    return v[min(len(v) - 1, int(round(p * (len(v) - 1))))] if v else None


def mann_whitney_p(a, b):
    n1, n2 = len(a), len(b)
    allv = sorted([(x, 0) for x in a] + [(x, 1) for x in b])
    ranks, i, tie = {}, 0, 0
    while i < len(allv):
        j = i
        while j + 1 < len(allv) and allv[j + 1][0] == allv[i][0]:
            j += 1
        for k in range(i, j + 1):
            ranks[k] = (i + j) / 2 + 1
        t = j - i + 1
        tie += t**3 - t
        i = j + 1
    r1 = sum(ranks[k] for k in range(len(allv)) if allv[k][1] == 0)
    u1 = r1 - n1 * (n1 + 1) / 2
    n = n1 + n2
    sd = math.sqrt(n1 * n2 / 12 * ((n + 1) - tie / (n * (n - 1))))
    z = (u1 - n1 * n2 / 2) / sd
    return 2 * (1 - 0.5 * (1 + math.erf(abs(z) / math.sqrt(2))))


def main(paths):
    recs = []
    for path in paths:
        for line in open(path):
            line = line.strip()
            if line:
                try:
                    recs.append(json.loads(line))
                except ValueError:
                    pass
    for r in recs:
        r["end"] = ts(r["at"])
    recs.sort(key=lambda r: r["end"])
    edit = [r for r in recs if r["channel"] == "edit-hook"]
    ee = [r["end"] for r in edit]

    def gauge(t, w=120):
        a, b = bisect.bisect_left(ee, t - w), bisect.bisect_right(ee, t + w)
        seg = [edit[i]["ms"] for i in range(a, b)]
        return q(seg, 0.9) if len(seg) >= 10 else None

    print("rows %d, %s to %s, %.1f h" % (len(recs), recs[0]["at"], recs[-1]["at"], (recs[-1]["end"] - recs[0]["end"]) / 3600))
    random.seed(20260929)
    for ch in ("session-hook", "stop-hook"):
        allms = [r["ms"] for r in recs if r["channel"] == ch]
        rs = [r for r in recs if r["channel"] == ch]
        for r in rs:
            r["g"] = gauge(r["end"])
        rs = [r for r in rs if r["g"] is not None]
        cuts = sorted(r["g"] for r in rs)
        lo, hi = q(cuts, 1 / 3), q(cuts, 2 / 3)
        idle = [r["ms"] for r in rs if r["g"] <= lo]
        loaded = [r["ms"] for r in rs if r["g"] > hi]
        boot = []
        for _ in range(4000):
            a = [random.choice(idle) for _ in idle]
            b = [random.choice(loaded) for _ in loaded]
            boot.append(q(b, 0.5) - q(a, 0.5))
        boot.sort()
        print("%s n=%d p50=%s p90=%s p99=%s | idle(gauge<=%s) n=%d p50=%s p90=%s | loaded(gauge>%s) n=%d p50=%s p90=%s | loaded-idle p50 %+d ms, 95%% CI [%d, %d], Mann-Whitney p=%.3f"
              % (ch, len(allms), q(allms, .5), q(allms, .9), q(allms, .99), lo, len(idle), q(idle, .5), q(idle, .9),
                 hi, len(loaded), q(loaded, .5), q(loaded, .9), q(loaded, .5) - q(idle, .5), boot[100], boot[3899], mann_whitney_p(idle, loaded)))


if __name__ == "__main__":
    main(sys.argv[1:])
