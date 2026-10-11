#!/usr/bin/env python3
"""Count abandoned MCP calls by tool and origin in a fathom call log window.

Usage: call-log-abandonment.py <mcp-calls.jsonl> <from-ts> <to-ts>
Timestamps are RFC 3339 UTC strings compared as text (the call log writes them
that way). Reads the file only; no network, no model. Prints aggregate counts:
no service path, session id or caller pid leaves the script.
"""
import collections
import json
import statistics
import sys

TOOLS = ("localize", "symbol_context", "query_symbols", "obligations", "verify_plan")


def main(path, lo, hi):
    total = 0
    calls = collections.Counter()
    abandoned = collections.Counter()
    lat = collections.defaultdict(list)
    lat_ab = collections.defaultdict(list)
    with open(path) as f:
        for line in f:
            try:
                row = json.loads(line)
            except ValueError:
                continue
            ts = row.get("ts", "")
            if not (lo <= ts < hi):
                continue
            total += 1
            tool = row.get("tool", "")
            if tool not in TOOLS:
                continue
            origin = row.get("origin", "none")
            calls[(tool, origin)] += 1
            lat[tool].append(row.get("latency_ms", 0))
            if row.get("outcome") == "abandoned":
                cause = "client" if "client went away" in row.get("error", "") else "other"
                abandoned[(tool, origin, cause)] += 1
                lat_ab[tool].append(row.get("latency_ms", 0))
    print("window", lo, hi, "calls", total)
    for key in sorted(calls):
        print("calls", *key, calls[key])
    for key in sorted(abandoned):
        print("abandoned", *key, abandoned[key])
    for tool in sorted(lat):
        p50 = statistics.median(lat[tool])
        p50_ab = statistics.median(lat_ab[tool]) if lat_ab[tool] else "-"
        print("p50_ms", tool, "all", p50, "abandoned", p50_ab)


if __name__ == "__main__":
    if len(sys.argv) != 4:
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2], sys.argv[3])
