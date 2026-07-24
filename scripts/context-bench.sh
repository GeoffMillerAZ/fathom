#!/usr/bin/env bash
#
# scripts/context-bench.sh — B1-2 (docs/plans/falsifiability.md): the
# two-arm context-bench runner for the ×10 tokenomics falsifiability gate
# (coding-agent-excellence.md §0 sentence 5 / §7 gate 6).
#
# Runs every task in testdata/contextbench/tasks.json (B1-1's committed
# corpus) TWICE via headless `claude -p` subagents:
#   - arm "fathom": may use ONLY the fathom MCP companion tools (find_symbol,
#     find_callers, references, symbol_context, outline, repo_map,
#     pack_context, entrypoints, query_symbols, capabilities, list_services,
#     staleness_report, verify_plan, obligations, read_source — the T5-4
#     bounded raw-text read added to the 16-tool /rpc/agent surface since the
#     prior H3-1 run) — never Bash/Read/Grep; mcp__fathom__ingest stays
#     explicitly denied (read-only against the live daemon, per CLAUDE.md).
#   - arm "grep":   may use ONLY grep/find/cat via Bash — never Read, never
#     any MCP tool.
# Both arms score real, non-fabricated runs when the fathom daemon is
# reachable; see the honest-degradation note below for what happens when
# it is not.
#
# Methodology notes (read before trusting the numbers):
#   - Neither arm uses --bare or --safe-mode: --bare forces API-key auth
#     (this repo's claude -p lane is the OAuth/subscription lane, which
#     --bare cannot use — verified live, "Not logged in"), and --safe-mode
#     disables MCP servers even with an explicit --mcp-config (verified
#     live). Both arms therefore run in the default startup mode, so
#     CLAUDE.md + user memory load identically for both — a fixed,
#     symmetric baseline of cache_creation/cache_read tokens per call
#     (tens of thousands of tokens; see the summary's per-arm totals). This
#     means TotalTokens as reported is dominated by that shared baseline,
#     NOT by discovery-call cost alone, for any single trivial task — the
#     ×10 discovery-token claim is only visible in aggregate / on
#     multi-hop tasks (cross-repo, multi-caller lookups), which is why
#     B1-1's corpus favors those categories. Report this caveat alongside
#     any number pulled from this script's output.
#   - --strict-mcp-config with a single-server config file scopes each
#     call's MCP surface precisely: arm "fathom" sees only the fathom HTTP
#     server (at $FATHOM_HTTP_URL/rpc/agent); arm "grep" sees no MCP
#     servers at all. Neither arm can accidentally use the user's other
#     registered servers (playwright, tailwindplus, appkit, apify).
#   - mcp__fathom__ingest is explicitly disallowed for arm "fathom" — this
#     script performs read-only queries against the live daemon only
#     (CLAUDE.md: never mutate the shared fleet store from a worker
#     script). If the daemon is unreachable at all, arm "fathom" is
#     skipped entirely (not faked) and every task's fathom-arm row is
#     marked accordingly in the results artifact; arm "grep" still runs
#     for real. This is the honest-degradation path required by
#     docs/plans/falsifiability.md B1-3.
#
# Output: workspace/artifacts/context-bench/run-<ts>/ (raw claude -p JSON
# per task/arm + scored ArmResult JSON per task/arm) and, at the top level,
# workspace/artifacts/context-bench/results-<ts>.json (machine-readable,
# internal/contextbench.RunResults shape) + summary-<ts>.md (rendered table).
#
# Usage:
#   scripts/context-bench.sh                  # run every task, both arms
#   scripts/context-bench.sh --arms grep       # grep arm only (skip fathom
#                                               # even if the daemon is up)
#   scripts/context-bench.sh --tasks <file>    # override the corpus path
#
# Env:
#   FATHOM_HTTP_URL          base URL of the fathom daemon (default
#                             http://localhost:7575)
#   CONTEXT_BENCH_MODEL      claude -p --model value (default sonnet)
#   CONTEXT_BENCH_TIMEOUT    per-call watchdog timeout in seconds (default
#                             180) — macOS has no `timeout` in the base
#                             install, so this script implements its own.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TASKS="$ROOT/testdata/contextbench/tasks.json"
FATHOM_HTTP_URL="${FATHOM_HTTP_URL:-http://localhost:7575}"
MODEL="${CONTEXT_BENCH_MODEL:-sonnet}"
TIMEOUT="${CONTEXT_BENCH_TIMEOUT:-180}"
ARMS="fathom grep"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --arms) ARMS="$2"; shift 2 ;;
    --tasks) TASKS="$2"; shift 2 ;;
    *) echo "usage: $(basename "$0") [--arms 'fathom grep'] [--tasks <path>]" >&2; exit 2 ;;
  esac
done

if [ ! -f "$TASKS" ]; then
  echo "context-bench: tasks file not found: $TASKS" >&2
  exit 2
fi

TS="$(date -u +%Y%m%dT%H%M%SZ)"
ARTIFACTS_DIR="$ROOT/workspace/artifacts/context-bench"
RUN_DIR="$ARTIFACTS_DIR/run-$TS"
RAW_DIR="$RUN_DIR/raw"
RESULTS_DIR="$RUN_DIR/results"
mkdir -p "$RAW_DIR" "$RESULTS_DIR"

echo "context-bench: building the scorer (GOWORK=off — nested worktrees can't resolve ../agentic-kit)"
SCORER="$RUN_DIR/contextbenchscore"
if ! GOWORK=off CGO_ENABLED=1 go build -o "$SCORER" "$ROOT/cmd/contextbenchscore"; then
  echo "context-bench: failed to build cmd/contextbenchscore" >&2
  exit 2
fi

# ─── fathom-arm reachability preflight (honest degradation, B1-3) ─────────
FATHOM_REACHABLE=0
if echo "$ARMS" | grep -qw fathom; then
  if curl -sS -m 5 -o /dev/null -w '' "$FATHOM_HTTP_URL/rpc/agent" \
      -X POST -H 'Content-Type: application/json' \
      -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"capabilities","arguments":{}}}'; then
    FATHOM_REACHABLE=1
    echo "context-bench: fathom daemon reachable at $FATHOM_HTTP_URL/rpc/agent — arm 'fathom' will run for real"
  else
    echo "context-bench: WARNING — fathom daemon NOT reachable at $FATHOM_HTTP_URL/rpc/agent" >&2
    echo "context-bench: degrading honestly — arm 'fathom' is SKIPPED (not faked); arm 'grep' still runs for real" >&2
  fi
fi

FATHOM_MCP_CONFIG="$RUN_DIR/fathom-mcp.json"
cat > "$FATHOM_MCP_CONFIG" <<EOF
{"mcpServers":{"fathom":{"type":"http","url":"$FATHOM_HTTP_URL/rpc/agent"}}}
EOF
EMPTY_MCP_CONFIG="$RUN_DIR/empty-mcp.json"
echo '{"mcpServers":{}}' > "$EMPTY_MCP_CONFIG"

# Tiny helper (avoids fragile nested-quote python -c one-liners): prints one
# scored ArmResult's headline to stdout.
cat > "$RUN_DIR/print_result.py" <<'PY'
import json, sys
d = json.load(sys.stdin)
status = "PASS" if d["success"] else "FAIL"
print("context-bench: [{}/{}] {} tokens={} wall_ms={}".format(
    d["task_id"], d["arm"], status, d["total_tokens"], d["wall_time_ms"]))
PY

# T5-8 (docs/plans/trust-and-tokenomics.md): read_source joined the 16-tool
# companion allowlist (T5-4) since the previous H3-1 run — added here so the
# fathom arm's allowed surface matches the live /rpc/agent registry exactly
# (companionWant in internal/mcp/surface_test.go, minus ingest, which stays
# explicitly denied below: this script never lets an arm mutate the shared
# fleet store).
FATHOM_ALLOWED="mcp__fathom__find_symbol mcp__fathom__find_callers mcp__fathom__references mcp__fathom__symbol_context mcp__fathom__outline mcp__fathom__repo_map mcp__fathom__pack_context mcp__fathom__entrypoints mcp__fathom__query_symbols mcp__fathom__capabilities mcp__fathom__list_services mcp__fathom__staleness_report mcp__fathom__verify_plan mcp__fathom__obligations mcp__fathom__read_source"
FATHOM_DISALLOWED="Bash Read Grep Glob Write Edit Task WebSearch WebFetch NotebookEdit mcp__fathom__ingest"
GREP_ALLOWED="Bash(grep:*) Bash(find:*) Bash(cat:*)"
GREP_DISALLOWED="Read Grep Glob Task WebSearch WebFetch Write Edit NotebookEdit"

# run_with_timeout OUTFILE CMD...  — macOS's base install has no `timeout(1)`;
# this is the standard background-watchdog substitute. Redirects CMD's
# stdout to OUTFILE (stderr to OUTFILE.stderr), returns CMD's exit code, or
# 124 (matching GNU timeout's convention) if the watchdog had to kill it.
run_with_timeout() {
  local outfile="$1"; shift
  "$@" < /dev/null > "$outfile" 2>"$outfile.stderr" &
  local pid=$!
  ( sleep "$TIMEOUT" && kill -TERM "$pid" 2>/dev/null ) &
  local watchdog=$!
  local rc=0
  wait "$pid" || rc=$?
  kill "$watchdog" 2>/dev/null
  wait "$watchdog" 2>/dev/null
  return "$rc"
}

run_arm() {
  local task_id="$1" prompt="$2" arm="$3" system_prompt="$4" mcp_config="$5" allowed="$6" disallowed="$7"
  local raw="$RAW_DIR/$task_id-$arm.json"
  local scored="$RESULTS_DIR/$task_id-$arm.json"

  echo "context-bench: [$task_id/$arm] running..."
  if ! run_with_timeout "$raw" claude -p "$prompt" \
      --append-system-prompt "$system_prompt" \
      --output-format json \
      --mcp-config "$mcp_config" \
      --strict-mcp-config \
      --allowedTools "$allowed" \
      --disallowedTools "$disallowed" \
      --model "$MODEL"; then
    echo "context-bench: [$task_id/$arm] claude -p exited non-zero (see $raw.stderr)" >&2
  fi

  if [ ! -s "$raw" ]; then
    echo "context-bench: [$task_id/$arm] no output captured — skipping scoring" >&2
    return 1
  fi

  if ! "$SCORER" score -tasks "$TASKS" -task-id "$task_id" -arm "$arm" -claude-json "$raw" -out "$scored"; then
    echo "context-bench: [$task_id/$arm] scoring failed" >&2
    return 1
  fi
  python3 "$RUN_DIR/print_result.py" < "$scored"
}

# ─── main loop ──────────────────────────────────────────────────────────
FATHOM_SYSTEM_PROMPT="You are answering one factual question about the fathom codebase using ONLY the fathom MCP tools available to you. Do not use Bash, Read, Grep, Glob, or any other tool — solve the task purely through fathom tool calls. Answer concisely, citing exact file paths / symbol names / IDs, since your answer is scored by substring match against a ground truth."
GREP_SYSTEM_PROMPT="You are answering one factual question about the fathom codebase. You may ONLY use grep, find, and cat, invoked via the Bash tool — no Read tool, no Grep tool, no Glob tool, no MCP tools of any kind. Answer concisely, citing exact file paths / symbol names / IDs, since your answer is scored by substring match against a ground truth."

TASK_COUNT=0
while IFS= read -r -d '' task_id && IFS= read -r -d '' prompt; do
  TASK_COUNT=$((TASK_COUNT + 1))
  if echo "$ARMS" | grep -qw grep; then
    run_arm "$task_id" "$prompt" grep "$GREP_SYSTEM_PROMPT" "$EMPTY_MCP_CONFIG" "$GREP_ALLOWED" "$GREP_DISALLOWED" || true
  fi
  if echo "$ARMS" | grep -qw fathom; then
    if [ "$FATHOM_REACHABLE" -eq 1 ]; then
      run_arm "$task_id" "$prompt" fathom "$FATHOM_SYSTEM_PROMPT" "$FATHOM_MCP_CONFIG" "$FATHOM_ALLOWED" "$FATHOM_DISALLOWED" || true
    fi
  fi
done < <(python3 -c '
import json, sys
d = json.load(open(sys.argv[1]))
out = sys.stdout.buffer
for t in d["tasks"]:
    out.write(t["id"].encode() + b"\x00")
    out.write(t["prompt"].encode() + b"\x00")
' "$TASKS")

echo "context-bench: ran $TASK_COUNT tasks"

if [ "$FATHOM_REACHABLE" -eq 0 ] && echo "$ARMS" | grep -qw fathom; then
  echo "context-bench: arm 'fathom' rows are ABSENT from this run's results (daemon unreachable) — do not report them as measured" >&2
fi

# ─── aggregate + render (only if we have anything to aggregate) ──────────
if [ -z "$(ls -A "$RESULTS_DIR" 2>/dev/null)" ]; then
  echo "context-bench: no scored results produced — nothing to aggregate" >&2
  exit 1
fi

RESULTS_JSON="$ARTIFACTS_DIR/results-$TS.json"
SUMMARY_MD="$ARTIFACTS_DIR/summary-$TS.md"
"$SCORER" report -tasks "$TASKS" -results "$RESULTS_DIR" -out-json "$RESULTS_JSON" -out-md "$SUMMARY_MD"

echo "context-bench: done"
echo "  raw transcripts + per-task scores: $RUN_DIR"
echo "  results:                           $RESULTS_JSON"
echo "  summary:                           $SUMMARY_MD"
