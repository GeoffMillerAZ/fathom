#!/usr/bin/env bash
#
# Measure how much Claude Code actually reaches for fathom vs. shell grep in a
# project's sessions — the dogfood adoption signal.
#
# The SessionStart adoption hook (fathom session-hook) was installed to push
# agents toward the index instead of grepping. This script quantifies whether it
# worked: for each Claude Code transcript of a project, it counts fathom MCP
# tool calls vs. grep-style searches (Grep tool + `grep`/`rg` in Bash), and
# splits the sessions at the hook-install date so you can read before/after.
#
# Read-only: it only parses ~/.claude/projects/*/*.jsonl. Nothing is written.
#
# Call-log source: the serve-http daemon writes JSONL tool-call records to
# workspace/logs/mcp-calls.jsonl (O_APPEND, survives restarts). Override via
# FATHOM_CALL_LOG env var. See internal/mcp/calllog.go + cmd/fathom/servehttp.go.
#
#   bash scripts/fathom-adoption.sh app-kit
#   bash scripts/fathom-adoption.sh app-kit resume_as_code fathom
#   HOOK_DATE=2026-06-17 bash scripts/fathom-adoption.sh app-kit
#
# Args: one or more project names (substring-matched against the encoded
# ~/.claude/projects dir names, so the ~/code-symlink variants are all caught).
# Name aliasing: "fathom" also matches dirs containing "sextant" (and vice versa)
# to handle the sextant→fathom rename; results are reported under the requested name.
# Defaults to the whole fleet if none are given.
#
# After each project's per-session table a per-ISO-week rollup is printed so a
# weekly adoption baseline can be read directly.
#
# A daemon call-log summary (total calls, by surface, by via, top 10 tools) is
# appended at the end. It is omitted gracefully if the log file does not exist.
set -euo pipefail

PROJECTS_DIR="$HOME/.claude/projects"
HOOK_DATE="${HOOK_DATE:-2026-06-17}"   # when the SessionStart adoption hook went live
DEFAULT_NAMES=(fathom agentic-kit app-kit throughline geoffmiller-cloud resume_as_code growing-gardens-bi car-assistant crossplane-platform)

names=("$@")
[ ${#names[@]} -eq 0 ] && names=("${DEFAULT_NAMES[@]}")

[ -d "$PROJECTS_DIR" ] || { echo "no transcripts dir at $PROJECTS_DIR"; exit 1; }

python3 - "$PROJECTS_DIR" "$HOOK_DATE" "${names[@]}" <<'PY'
import json, os, sys, glob
from collections import defaultdict
from datetime import date as _date

projects_dir, hook_date = sys.argv[1], sys.argv[2]
names = sys.argv[3:]

# Alias map: when matching transcript dirs for a name, also search for these
# alternate substrings (handles project renames like sextant→fathom).
ALIASES = {
    "fathom":   ["sextant"],
    "sextant":  ["fathom"],
}

def find_dirs(name):
    """Return sorted, deduplicated list of project dirs matching name + its aliases."""
    terms = [name] + ALIASES.get(name, [])
    dirs = set()
    for t in terms:
        dirs.update(glob.glob(os.path.join(projects_dir, f"*{t}*")))
    return sorted(dirs)

def is_fathom(name):
    return isinstance(name, str) and (
        name.startswith("mcp__fathom__") or name.startswith("mcp__sextant__")
    )  # legacy prefix: pre-rename transcripts

def is_grep(name, inp):
    if name == "Grep":
        return True
    if name == "Bash" and isinstance(inp, dict):
        cmd = inp.get("command", "")
        for tok in cmd.replace("|", " ").replace(";", " ").split():
            if tok in ("grep", "egrep", "rg", " grep"):
                return True
    return False

def walk(obj, sx, gp):
    """Recursively find tool_use blocks; tally fathom vs grep."""
    if isinstance(obj, dict):
        if obj.get("type") == "tool_use":
            nm = obj.get("name", "")
            if is_fathom(nm): sx[0] += 1
            elif is_grep(nm, obj.get("input", {})): gp[0] += 1
        for v in obj.values(): walk(v, sx, gp)
    elif isinstance(obj, list):
        for v in obj: walk(v, sx, gp)

def session_stats(path):
    sx, gp = [0], [0]
    first_ts = None
    with open(path, "r", errors="ignore") as f:
        for line in f:
            line = line.strip()
            if not line: continue
            try: ev = json.loads(line)
            except Exception: continue
            if first_ts is None:
                first_ts = ev.get("timestamp")
            walk(ev, sx, gp)
    if first_ts is None:
        first_ts = ""
    return sx[0], gp[0], (first_ts or "")[:10]   # date only

def iso_week(day_str):
    """Return 'YYYY-Www' label from 'YYYY-MM-DD', or 'unknown'."""
    if not day_str or len(day_str) < 10:
        return "unknown"
    try:
        d = _date.fromisoformat(day_str)
        iso = d.isocalendar()
        return f"{iso[0]}-W{iso[1]:02d}"
    except Exception:
        return "unknown"

grand = {}
for name in names:
    dirs = find_dirs(name)
    sessions = []
    for d in dirs:
        for jf in sorted(glob.glob(os.path.join(d, "*.jsonl"))):
            sx, gp, day = session_stats(jf)
            if sx == 0 and gp == 0:
                continue  # skip sessions with no searching at all
            sessions.append((day, sx, gp, os.path.basename(jf)))
    print(f"\n=== {name} ===")
    if not sessions:
        print("  (no sessions with tool activity"
              + ("" if dirs else " — and no matching project dir") + ")")
        continue
    sessions.sort()
    pre = [s for s in sessions if s[0] and s[0] < hook_date]
    post = [s for s in sessions if s[0] and s[0] >= hook_date]

    # ── per-session table ──────────────────────────────────────────────────
    print(f"  {'date':<11} {'fathom':>8} {'grep':>6} {'fathom share':>14}")
    for day, sx, gp, _ in sessions:
        share = sx / (sx + gp) if (sx + gp) else 0
        marker = "  <- post-hook" if (day and day >= hook_date) else ""
        print(f"  {day:<11} {sx:>8} {gp:>6} {share:>13.0%}{marker}")

    def agg(group):
        s = sum(x[1] for x in group); g = sum(x[2] for x in group)
        share = s / (s + g) if (s + g) else 0
        return s, g, share, len(group)
    ps, pg, psh, pn = agg(pre)
    qs, qg, qsh, qn = agg(post)
    print(f"  --")
    print(f"  pre-hook  ({pn} sess): fathom {ps:>5}  grep {pg:>5}  share {psh:.0%}")
    print(f"  post-hook ({qn} sess): fathom {qs:>5}  grep {qg:>5}  share {qsh:.0%}")
    if pn and qn:
        delta = qsh - psh
        print(f"  Δ fathom share: {delta:+.0%}")

    # ── per-ISO-week rollup ────────────────────────────────────────────────
    weekly = defaultdict(lambda: [0, 0])
    for day, sx, gp, _ in sessions:
        wk = iso_week(day)
        weekly[wk][0] += sx
        weekly[wk][1] += gp
    print(f"\n  Weekly rollup (pre + post combined):")
    print(f"  {'week':<12} {'fathom':>8} {'grep':>6} {'share':>8}")
    for wk in sorted(weekly):
        s, g = weekly[wk]
        share = s / (s + g) if (s + g) else 0
        print(f"  {wk:<12} {s:>8} {g:>6} {share:>7.0%}")

    grand[name] = (pre, post)

print(f"\n(hook install date = {hook_date}; 'fathom share' = fathom / (fathom + grep))")
PY

# ── Daemon call-log summary ──────────────────────────────────────────────────
# Never fails the script — call-log is optional telemetry.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
CALL_LOG="${FATHOM_CALL_LOG:-$REPO_ROOT/workspace/logs/mcp-calls.jsonl}"

python3 - "$CALL_LOG" <<'PY2' || true
import json, sys, os
from collections import Counter

call_log = sys.argv[1]

print("\n=== daemon call-log ===")
if not os.path.isfile(call_log):
    print(f"  (call log not found: {call_log})")
    sys.exit(0)

total = 0
by_surface = Counter()
by_via = Counter()
by_tool = Counter()

with open(call_log, "r", errors="ignore") as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        try:
            rec = json.loads(line)
        except Exception:
            continue
        total += 1
        if rec.get("surface"):
            by_surface[rec["surface"]] += 1
        if rec.get("via"):
            by_via[rec["via"]] += 1
        by_tool[rec.get("tool", "(unknown)")] += 1

print(f"  total calls: {total}")

print(f"\n  calls by surface:")
if by_surface:
    for surf, n in sorted(by_surface.items(), key=lambda x: -x[1]):
        print(f"    {surf:<20} {n:>6}")
else:
    print("    (none recorded)")

print(f"\n  calls by via:")
if by_via:
    for via, n in sorted(by_via.items(), key=lambda x: -x[1]):
        print(f"    {via:<20} {n:>6}")
else:
    print("    (none recorded)")

print(f"\n  top 10 tools:")
for tool, n in by_tool.most_common(10):
    print(f"    {tool:<35} {n:>6}")
PY2
