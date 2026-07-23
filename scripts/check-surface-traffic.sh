#!/usr/bin/env bash
#
# scripts/check-surface-traffic.sh — doctrine-7 loud-failure check ("is
# anything flowing?") for fathom's own served MCP surfaces.
#
# Curls the running fathom-http daemon's server_stats tool and asserts that
# each named surface shows nonzero traffic since its recorded activation
# stamp (t0) — an EXISTENCE check (did this surface ever fire since it went
# live), never a liveness-WINDOW check (did it fire recently). This is the
# bash-side consumer of agentic-kit's
# foundations/telemetry/heartbeat.CheckExistence semantics (Phase K, K3-1 of
# docs/plans/kit-uplift.md), applied to fathom's HTTP surface without pulling
# a Go dependency into a shell script.
#
# Spec this codes against (server_stats' JSON result, "surface_traffic"
# key): {"<surface>": {"calls": <int>, "last_ts": "<RFC3339>" | ""}}.
#
# DEPENDENCY NOTE (as of 2026-07-22): the surface_traffic block is being
# added by a sibling Phase-K worker (A1/B2) and is NOT present in this
# worktree's server_stats yet. Until it lands, every check below fails with
# the distinct "surface_traffic missing from server_stats" diagnostic —
# never silently read as "zero calls" — so a missing dependency and a
# genuine zero-traffic finding are never conflated (doctrine 3: never
# silent, never miscategorized).
#
# t0 (doctrine law 6, "activation is stamped, never prose-documented"): read
# from workspace/logs/<surface>.stamp (first line, RFC3339 UTC), written by
# whatever operator step turned the surface on. A surface with no stamp file
# degrades to a pure ever-fired check (calls > 0) with a loud warning —
# never a silent skip.
#
# Usage:
#   scripts/check-surface-traffic.sh <surface> [<surface> ...]
#   scripts/check-surface-traffic.sh agent          # checks /rpc/agent traffic
#   scripts/check-surface-traffic.sh agent companion
#
# Env:
#   FATHOM_HTTP_URL   base URL of the running daemon (default http://localhost:7575)
#
# Exit: 0 all named surfaces pass; 1 any surface fails (including the
# missing-dependency case); 2 usage/transport error (daemon unreachable,
# malformed response).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_URL="${FATHOM_HTTP_URL:-http://localhost:7575}"

if [ "$#" -eq 0 ]; then
  echo "usage: $(basename "$0") <surface> [<surface> ...]" >&2
  exit 2
fi

resp="$(curl -sS -m 10 -X POST "$BASE_URL/rpc" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"server_stats","arguments":{}}}')" || {
  echo "check-surface-traffic: cannot reach $BASE_URL/rpc — is the daemon running?" >&2
  exit 2
}

python3 - "$ROOT" "$resp" "$@" <<'PY'
import datetime
import json
import os
import sys

root, resp_raw = sys.argv[1], sys.argv[2]
surfaces = sys.argv[3:]


def parse_ts(raw):
    if not raw:
        return None
    try:
        return datetime.datetime.fromisoformat(raw.replace("Z", "+00:00"))
    except ValueError:
        return None


try:
    envelope = json.loads(resp_raw)
except json.JSONDecodeError as e:
    print(f"check-surface-traffic: server_stats response is not JSON: {e}", file=sys.stderr)
    sys.exit(2)

if envelope.get("error"):
    print(f"check-surface-traffic: server_stats RPC error: {envelope['error']}", file=sys.stderr)
    sys.exit(2)

try:
    text = envelope["result"]["content"][0]["text"]
    stats = json.loads(text)
except (KeyError, IndexError, TypeError, json.JSONDecodeError) as e:
    print(f"check-surface-traffic: unexpected server_stats envelope shape: {e}", file=sys.stderr)
    sys.exit(2)

traffic = stats.get("surface_traffic")
if traffic is None:
    print(
        "check-surface-traffic: FAIL — server_stats has no \"surface_traffic\" block yet "
        "(dependency not merged: the A1/B2 surface-traffic instrumentation). "
        "This is a MISSING-DEPENDENCY failure, not a zero-traffic finding.",
        file=sys.stderr,
    )
    sys.exit(1)

failed = False
for surface in surfaces:
    stamp_path = os.path.join(root, "workspace", "logs", f"{surface}.stamp")
    since = None
    if os.path.isfile(stamp_path):
        with open(stamp_path) as f:
            line = f.readline().strip()
        since = parse_ts(line)
        if since is None and line:
            print(
                f"check-surface-traffic: WARNING — {stamp_path} does not parse as RFC3339 "
                f"({line!r}); falling back to an ever-fired check for {surface!r}",
                file=sys.stderr,
            )
    else:
        print(
            f"check-surface-traffic: WARNING — no stamp for {surface!r} at "
            f"workspace/logs/{surface}.stamp (doctrine 6); checking ever-fired instead of since-t0",
            file=sys.stderr,
        )

    entry = traffic.get(surface)
    if entry is None:
        print(f"check-surface-traffic: FAIL — {surface!r} not present in surface_traffic at all", file=sys.stderr)
        failed = True
        continue

    calls = entry.get("calls", 0)
    last_ts_raw = entry.get("last_ts") or ""
    last_ts = parse_ts(last_ts_raw)

    if calls <= 0:
        print(
            f"check-surface-traffic: FAIL — {surface!r} expected traffic but observed zero calls "
            "(doctrine 7: an instrument with no existence check is theater)",
            file=sys.stderr,
        )
        failed = True
        continue

    if since is not None and (last_ts is None or last_ts < since):
        print(
            f"check-surface-traffic: FAIL — {surface!r} has {calls} call(s) but none since its "
            f"recorded t0 {since.isoformat()} (last_ts={last_ts_raw!r})",
            file=sys.stderr,
        )
        failed = True
        continue

    print(f"check-surface-traffic: OK — {surface!r}: {calls} call(s), last_ts={last_ts_raw!r}")

sys.exit(1 if failed else 0)
PY
