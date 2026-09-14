#!/usr/bin/env bash
#
# scripts/context-bench.sh — B1-2 (docs/plans/falsifiability.md): the
# context-bench runner for the tokenomics falsifiability gate
# (coding-agent-excellence.md §0 sentence 5 / §7 gate 6), extended by
# bench-confidence.md (repetition, the ledger) and bench-public.md
# (resume, subject parameterization, the none arm, precision scoring).
#
# Runs every runnable task in the corpus via headless `claude -p`
# subagents, one fresh session per (task, arm, trial):
#   - arm "fathom": may use ONLY the fathom MCP companion tools listed in
#     FATHOM_ALLOWED below — never Bash/Read/Grep; mcp__fathom__ingest
#     stays explicitly denied (read-only against the daemon it is pointed
#     at). mcp__fathom__memory_search is deliberately NOT in the allowlist:
#     it recalls conversational continuity, not code facts (ADR-028's
#     authority boundary — memory may recall, never assert a code fact),
#     so a bench of code discovery must not let the treatment arm answer
#     from a prior session's notes. Documented in docs/bench/README.md.
#   - arm "full":   the realistic arm (SB-6, docs/plans/strengths-bench.md):
#     the SAME fathom MCP surface as arm "fathom" PLUS Bash/Read/Grep/Glob
#     — no artificial tool-starvation, so a session can mix index calls
#     with a plain grep or read exactly as an unconstrained agent would.
#     Talks to the same fathom daemon as arm "fathom" and its cells key on
#     the fathom commit the same way (arm_surface = FATHOM_ARM_SURFACE).
#   - arm "grep":   may use ONLY grep/find/cat via Bash — never Read, never
#     any MCP tool.
#   - arm "none":   NO tools at all (the memorisation floor, PB-9). A task
#     this arm passes in >= 2 of 3 trials is `leaked` under the corpus's
#     committed leak_rule and leaves the verdict; the ledger row lists it.
# All tool arms score real, non-fabricated runs when the fathom daemon is
# reachable (arm "grep" needs no daemon); see the honest-degradation note
# below for what happens when it is not.
#
# Methodology notes (read before trusting the numbers):
#   - Neither arm uses --bare or --safe-mode: --bare forces API-key auth
#     (this repo's claude -p lane is the OAuth/subscription lane, which
#     --bare cannot use — verified live, "Not logged in"), and --safe-mode
#     disables MCP servers even with an explicit --mcp-config (verified
#     live). All arms therefore run in the default startup mode, so
#     CLAUDE.md + user memory load identically for every arm — a fixed,
#     symmetric baseline of cache_creation/cache_read tokens per call
#     (tens of thousands of tokens; see the summary's per-arm totals). This
#     means TotalTokens as reported is dominated by that shared baseline,
#     NOT by discovery-call cost alone, for any single trivial task — the
#     discovery-token claim is only visible in aggregate / on multi-hop
#     tasks, which is why the corpora favor those categories. Report this
#     caveat alongside any number pulled from this script's output.
#   - --strict-mcp-config with a single-server config file scopes each
#     call's MCP surface precisely: arm "fathom" sees only the fathom HTTP
#     server (at $FATHOM_HTTP_URL/rpc/agent); the other arms see no MCP
#     servers at all. No arm can accidentally use the user's other
#     registered servers.
#   - mcp__fathom__ingest is explicitly disallowed for arm "fathom" — this
#     script performs read-only queries against the daemon only (CLAUDE.md:
#     never mutate the shared fleet store from a worker script). If the
#     daemon is unreachable at all, arm "fathom" is skipped entirely (not
#     faked) and every task's fathom-arm cell is ABSENT in the results
#     artifact; the other arms still run for real. This is the
#     honest-degradation path required by docs/plans/falsifiability.md B1-3.
#   - scratch-daemon-default (VB-36, O47): the scratch daemon (PB-5) is now
#     the DEFAULT for the wired arms ("fathom", "full") — never the fleet
#     store, and never the live checkout. The subject is `git archive HEAD`
#     exported to a scratch dir (prepare_session_root's SESSION_ROOT, the
#     SAME export the sessions themselves run in), a scratch fathom binary
#     is built FROM THAT EXPORT with `-buildvcs=false` (an export has no
#     .git for vcs stamping), and a scratch store is ingested from it —
#     never from the live dev tree. This closes O47: a fleet daemon or a
#     scratch daemon built/ingested from the live checkout can serve an
#     answer that cites the live checkout's own absolute path, which is
#     also a path a live editor may be changing mid-run. The binary+store
#     are cached under workspace/tmp/ keyed by the export's own commit sha
#     and REUSED across cells, across `--repeat`, and across separate
#     invocations of this script for the same export — the obligations-
#     layer compile (below) is the expensive part this protects, so a
#     repeated run pays it once. `--no-scratch-daemon` (or `--daemon-url`,
#     an alias of `--fathom-url`) keeps the old behavior for a deliberate
#     operator run against a daemon they already run. A scratch daemon has
#     no LLM-compiled obligations layer UNTIL ONE IS COMPILED: --env
#     defaults to "" there and tasks that declare `requires:
#     [compiled-layer]` are skipped (never scored as misses) unless the
#     operator explicitly passes `--env compiled-layer`, which both admits
#     those tasks AND (COSTS LLM TOKENS) compiles the layer for real,
#     once per export, cached exactly like the binary/store above. Against
#     the fleet daemon (`--no-scratch-daemon`/`--daemon-url`) --env
#     defaults to "compiled-layer", as before.
#   - Every served payload in a transcript is checked against the live
#     checkout's own absolute path (DetectPathLeak): a hit sets
#     `path_leak: true` on the cell. Every Bash/Read tool input is checked
#     against the session's own root (DetectOutOfRootReads, the O47 shape:
#     `--restricted` confines Read/Glob/Grep to the session root, but a
#     Bash command naming an absolute path escapes it completely): a hit
#     is counted on `out_of_root_reads` and named in `out_of_root_paths`.
#     Both are recorded on the cell (never silently), summed/listed on the
#     ledger row, and documented in docs/bench/README.md.
#
# Output: workspace/artifacts/context-bench/run-<ts>/ (raw claude -p JSON
# per cell + scored ArmResult JSON per cell) and, at the top level,
# workspace/artifacts/context-bench/results-<ts>.json (machine-readable,
# internal/contextbench.RunResults shape) + summary-<ts>.md (rendered table).
#
# Repetition (2026-09-01): --repeat N runs every (task, arm) cell N times.
# The corpus's committed machine_criterion carries a replication floor; a
# run below it is scored INDETERMINATE, not a win. Cost scales linearly
# with N — a --repeat 3 home run is roughly three times the tokens of the
# 2026-07-24 run (~13M tokens, ~$13 at 2026-09 prices), so it is
# operator-initiated, never automatic.
#
# Remembered cells (BP-4, ADR-032 §2): every scored cell is filed under a
# content-addressed key over the invariants that bind it — corpus bytes,
# task, arm, model id, harness version, scoring, prompt subject, and the
# fathom binary commit for the fathom arm (empty for grep and none). Before
# running a cell the runner looks for one with that key and borrows it,
# recording `borrowed: true` and the run that paid for it. The grep and
# `none` arms do not depend on fathom's code, so their cells survive every
# fathom change; a fathom change re-runs only the cells its own commit can
# move. --no-reuse-cells turns borrowing off (default: on).
#
# Targeted runs (BP-8): --only 'H1-T01 H1-T05' runs a subset, and
# --scope targeted marks the row as diff-selected. Such a run does NOT append
# to the committed docs/bench/ledger.jsonl (BP-33): its row goes to
# workspace/artifacts/context-bench/ledger-targeted.jsonl unless --ledger (or
# CONTEXT_BENCH_LEDGER) names one explicitly, and cells_lost counts only the
# SELECTED tasks x arms x trials. The evaluator then
# refuses to turn it into a verdict (indeterminate, reason named). Use
# `task bench-targeted RANGE=origin/main...HEAD`, which chains
# `contextbenchscore select` into this script.
#
# Resume (PB-4): --resume <run-dir> re-uses a run directory, skips every
# cell whose scored file already exists, scores only the gaps, and records
# how many intended cells are still missing (cells_lost on the ledger row).
# One watchdog kill in 78 calls no longer forces a full re-run.
#
# Usage:
#   scripts/context-bench.sh                       # every task, arms "fathom grep"
#   scripts/context-bench.sh --arms 'fathom grep none'
#   scripts/context-bench.sh --arms 'full grep'    # the realistic arm (SB-6): fathom MCP + Bash/Read/Grep
#   scripts/context-bench.sh --tasks <file>        # override the corpus path
#   scripts/context-bench.sh --repeat 3            # three trials per cell
#   scripts/context-bench.sh --resume <run-dir>    # fill the gaps of an earlier run
#   scripts/context-bench.sh --scoring precision   # H2-2's fenced-block scoring
#   scripts/context-bench.sh --criterion H2-2      # evaluate a registered criterion
#   scripts/context-bench.sh --stream              # accepted for back-compat; every
#                                                  # cell captures a stream-json transcript
#                                                  # unconditionally now (VB-36 needs one on
#                                                  # every cell for the path-leak/out-of-root
#                                                  # checks below, and it also fills
#                                                  # tokens-to-first-gold-hit/tool calls)
#   scripts/context-bench.sh --subject-root <dir> --service <id> \
#                            --prompt-subject 'the acme codebase'   # scratch-daemon-default: on by default
#   scripts/context-bench.sh --no-scratch-daemon --fathom-url <url>  # talk to a daemon you already run
#   scripts/context-bench.sh --env compiled-layer  # admits compiled-layer tasks; with the
#                                                  # (default) scratch daemon this ALSO
#                                                  # compiles the layer for real (COSTS
#                                                  # LLM TOKENS), once per export, cached
#   scripts/context-bench.sh --only 'H1-T01 H1-T05' --scope targeted
#   scripts/context-bench.sh --no-reuse-cells      # never borrow a stored cell
#
# Clean room (RC-7, vision 28 §2.2): a smoke test proved the earlier scratch
# CLAUDE_CONFIG_DIR approach loses the login ("Not logged in"), so every
# `claude -p` call in this script instead uses Claude Code 2.1.270's own
# isolation: `--restricted` (ignores the operator's user/project/local
# settings files while still confining file tools to the working
# directory), `--disable-slash-commands`, `--settings <path>` naming a
# settings.json this script renders per arm under the run dir, and
# `--tools <list>` naming exactly the arm's own built-in tools (derived
# from that arm's allow-list variable below, mcp__ entries stripped — an
# MCP server's tools are already scoped by --mcp-config/--strict-mcp-config,
# not --tools). Every arm's settings.json is `{}` today (no hooks, no
# plugins, no slash commands) — the home bench runs the subject hook-free
# (see prepare_session_root) and, for now, runs the harness side hook-free
# too; a future arm meant to exercise the subject's own hook stack would
# render that arm's settings.json with it instead. --permission-mode
# default is passed on every call so an arm's
# --allowedTools/--disallowedTools restriction actually DENIES a
# disallowed tool instead of auto-accepting it. Before this, every session
# — control included — inherited the operator's ~90 slash commands,
# plugins, and fathom's own SessionStart primer (F4/O25): the control was
# not a control. Combined with prepare_session_root's hook-free export of
# the SUBJECT tree below, no operator configuration reaches either arm.
# Every call also passes `--add-dir "$SESSION_ROOT"` (VB-36): sessions run
# WITH the export as cwd already, so this is belt-and-braces, not the only
# confinement — `--restricted` is what actually confines the Read/Glob/Grep
# tools, and it is a shell command naming an absolute path that escapes
# both (O47's out_of_root_reads finding, checked separately below).
#
# Env:
#   FATHOM_HTTP_URL          base URL of the fathom daemon (default
#                             http://localhost:7575; --fathom-url overrides)
#   CONTEXT_BENCH_MODEL      claude -p --model value (default sonnet)
#   CONTEXT_BENCH_TIMEOUT    per-call watchdog timeout in seconds (default
#                             180) — macOS has no `timeout` in the base
#                             install, so this script implements its own.
#   CONTEXT_BENCH_REPEAT     default for --repeat (1)
#   CONTEXT_BENCH_LEDGER     default for --ledger (docs/bench/ledger.jsonl)
#   CONTEXT_BENCH_MODEL_ID   model identity the cell key is built on
#                             (default: the --model alias; pin it to the
#                             dated id from the last ledger row to borrow
#                             safely across a model release)
#   CONTEXT_BENCH_FATHOM_COMMIT  arm_surface for the fathom arm (default:
#                             this tree's HEAD, which --scratch-daemon
#                             builds from and `task deploy` asserts)
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TASKS="$ROOT/testdata/contextbench/tasks.json"
FATHOM_HTTP_URL="${FATHOM_HTTP_URL:-http://localhost:7575}"
MODEL="${CONTEXT_BENCH_MODEL:-sonnet}"
TIMEOUT="${CONTEXT_BENCH_TIMEOUT:-180}"
ARMS="fathom grep"
REPEAT="${CONTEXT_BENCH_REPEAT:-1}"
LEDGER="${CONTEXT_BENCH_LEDGER:-$ROOT/docs/bench/ledger.jsonl}"
# Whether the operator NAMED a ledger (--ledger or CONTEXT_BENCH_LEDGER). A
# targeted run writes to the workspace targeted ledger unless they did (BP-33).
LEDGER_EXPLICIT=0
[ -n "${CONTEXT_BENCH_LEDGER:-}" ] && LEDGER_EXPLICIT=1
RESUME_DIR=""
SUBJECT_ROOT="$ROOT"
SERVICE="fathom"
PROMPT_SUBJECT="the fathom codebase"
# scratch-daemon-default (VB-36): the wired arms talk to a scratch daemon
# of the exported subject by default now, never the fleet store and never
# the live checkout — see the header. --no-scratch-daemon, or naming a
# daemon explicitly (--fathom-url/--daemon-url), opts back into the old
# fleet-daemon behavior for a deliberate operator run; SCRATCH_DAEMON_SET
# / FATHOM_URL_SET record which flags were actually passed so that
# decision can be made once, after argument parsing, regardless of the
# order the operator gave them in.
SCRATCH_DAEMON=1
SCRATCH_DAEMON_SET=0
FATHOM_URL_SET=0
SCORING="substring"
CRITERION=""
TREATMENT="fathom"
STREAM=0
ENV_SET=0
ENV=""
ONLY=""
SCOPE="full"
REUSE_CELLS=1

usage() {
  cat >&2 <<USAGE
usage: $(basename "$0") [--arms 'fathom grep none full'] [--tasks <path>] [--repeat N] [--ledger <path>]
          [--resume <run-dir>] [--scoring substring|precision] [--criterion <id>] [--stream]
          [--subject-root <dir>] [--service <id>] [--fathom-url <url>] [--daemon-url <url>]
          [--prompt-subject <text>] [--scratch-daemon] [--no-scratch-daemon]
          [--env <cap,cap>] [--only '<id id>'] [--scope full|targeted]
          [--no-reuse-cells]
USAGE
  exit 2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --arms) ARMS="$2"; shift 2 ;;
    --tasks) TASKS="$2"; shift 2 ;;
    --repeat) REPEAT="$2"; shift 2 ;;
    --ledger) LEDGER="$2"; LEDGER_EXPLICIT=1; shift 2 ;;
    --resume) RESUME_DIR="$2"; shift 2 ;;
    --scoring) SCORING="$2"; shift 2 ;;
    --criterion) CRITERION="$2"; shift 2 ;;
    --treatment) TREATMENT="$2"; shift 2 ;;
    --stream) STREAM=1; shift ;;
    --subject-root) SUBJECT_ROOT="$(cd "$2" && pwd)" || exit 2; shift 2 ;;
    --service) SERVICE="$2"; shift 2 ;;
    --fathom-url) FATHOM_HTTP_URL="$2"; FATHOM_URL_SET=1; shift 2 ;;
    --daemon-url) FATHOM_HTTP_URL="$2"; FATHOM_URL_SET=1; shift 2 ;;
    --prompt-subject) PROMPT_SUBJECT="$2"; shift 2 ;;
    --scratch-daemon) SCRATCH_DAEMON=1; SCRATCH_DAEMON_SET=1; shift ;;
    --no-scratch-daemon) SCRATCH_DAEMON=0; SCRATCH_DAEMON_SET=1; shift ;;
    --env) ENV="$2"; ENV_SET=1; shift 2 ;;
    --only) ONLY="$2"; shift 2 ;;
    --scope) SCOPE="$2"; shift 2 ;;
    --reuse-cells) REUSE_CELLS=1; shift ;;
    --no-reuse-cells) REUSE_CELLS=0; shift ;;
    *) usage ;;
  esac
done

case "$REPEAT" in
  ''|*[!0-9]*) echo "context-bench: --repeat must be a positive integer, got '$REPEAT'" >&2; exit 2 ;;
esac
[ "$REPEAT" -ge 1 ] || { echo "context-bench: --repeat must be >= 1" >&2; exit 2; }
case "$SCORING" in
  substring|precision) ;;
  *) echo "context-bench: --scoring must be substring or precision, got '$SCORING'" >&2; exit 2 ;;
esac
case "$SCOPE" in
  full|targeted) ;;
  *) echo "context-bench: --scope must be full or targeted, got '$SCOPE'" >&2; exit 2 ;;
esac
# A subset run that still claims full scope would let a diff-chosen sample
# produce a corpus verdict. Refuse it here, not only in the evaluator.
if [ -n "$ONLY" ] && [ "$SCOPE" != "targeted" ]; then
  echo "context-bench: --only requires --scope targeted (a subset never carries a verdict)" >&2
  exit 2
fi

if [ ! -f "$TASKS" ]; then
  echo "context-bench: tasks file not found: $TASKS" >&2
  exit 2
fi

# scratch-daemon-default (VB-36): naming a daemon explicitly
# (--fathom-url/--daemon-url) without ALSO naming --scratch-daemon
# explicitly is the deliberate-operator-run signal — "talk to the daemon I
# named", not "and also build a throwaway one nobody asked for". An
# explicit --scratch-daemon (or --no-scratch-daemon) always wins outright,
# regardless of argument order.
if [ "$FATHOM_URL_SET" -eq 1 ] && [ "$SCRATCH_DAEMON_SET" -eq 0 ]; then
  SCRATCH_DAEMON=0
fi

# The environment's capabilities decide which tasks are runnable
# (`requires`, PB-12). A scratch daemon has no compiled layer UNTIL one is
# compiled (see the header and the scratch-daemon block below) — so --env
# still defaults to "" there, admitting compiled-layer tasks only when the
# operator explicitly asks (which then also triggers the real compile).
if [ "$ENV_SET" -eq 0 ]; then
  if [ "$SCRATCH_DAEMON" -eq 1 ]; then ENV=""; else ENV="compiled-layer"; fi
fi

ARTIFACTS_DIR="$ROOT/workspace/artifacts/context-bench"

# BP-33: a targeted run's row is not committed history. docs/bench/ledger.jsonl
# holds corpus-scoped rows a reader can compare; a --scope targeted / --only
# subset carries no verdict by construction, so its row goes to the gitignored
# workspace ledger unless the operator named --ledger (or CONTEXT_BENCH_LEDGER)
# deliberately. The evaluator makes the same decision (TargetedLedgerPath) — it
# is repeated here only so the run's final lines can say where the row went.
LEDGER_REDIRECTED=0
if [ "$LEDGER_EXPLICIT" -eq 0 ] && { [ -n "$ONLY" ] || [ "$SCOPE" = "targeted" ]; }; then
  LEDGER="$ARTIFACTS_DIR/ledger-targeted.jsonl"
  LEDGER_REDIRECTED=1
fi
if [ -n "$RESUME_DIR" ]; then
  RUN_DIR="$(cd "$RESUME_DIR" && pwd)" || { echo "context-bench: --resume dir not found: $RESUME_DIR" >&2; exit 2; }
  TS="${RUN_DIR##*/run-}"
  echo "context-bench: resuming $RUN_DIR (run id $TS) — cells with a scored file are kept, the rest are run"
else
  TS="$(date -u +%Y%m%dT%H%M%SZ)"
  RUN_DIR="$ARTIFACTS_DIR/run-$TS"
fi
RAW_DIR="$RUN_DIR/raw"
RESULTS_DIR="$RUN_DIR/results"
CELLS_DIR="$ARTIFACTS_DIR/cells"
mkdir -p "$RAW_DIR" "$RESULTS_DIR" "$CELLS_DIR"

# The cell-key invariants (BP-3). Captured once so `borrow`, `score` and
# `ledger` cannot disagree about what a key means.
MODEL_ID="${CONTEXT_BENCH_MODEL_ID:-$MODEL}"
HARNESS_VERSION="$(claude --version 2>/dev/null | head -1)"
if [ -z "$HARNESS_VERSION" ]; then
  echo "context-bench: could not read \`claude --version\` — cells cannot be keyed on an unknown harness (O24: the harness IS a variable)" >&2
  exit 2
fi
# arm_surface for the fathom arm: which fathom the arm was talking to. With
# --scratch-daemon this tree is literally what the daemon was built from;
# against the fleet daemon it is the operator's assertion that the daemon
# serves HEAD (`task deploy` checks it). Override when it does not.
FATHOM_ARM_SURFACE="${CONTEXT_BENCH_FATHOM_COMMIT:-$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
echo "context-bench: cell key inputs — model_id=$MODEL_ID harness='$HARNESS_VERSION' scoring=$SCORING subject='$PROMPT_SUBJECT' fathom_arm_surface=$FATHOM_ARM_SURFACE (reuse_cells=$REUSE_CELLS)"

echo "context-bench: building the scorer (GOWORK=off — nested worktrees can't resolve ../agentic-kit)"
SCORER="$RUN_DIR/contextbenchscore"
if ! GOWORK=off CGO_ENABLED=1 go build -o "$SCORER" "$ROOT/cmd/contextbenchscore"; then
  echo "context-bench: failed to build cmd/contextbenchscore" >&2
  exit 2
fi

# Sessions run in SESSION_ROOT, an export of the subject's tracked tree with
# .claude/settings.json removed: a subject that is itself a fathom-wired
# repo carries hooks (the Stop verify gate, obligations) that otherwise fire
# INSIDE the bench session and replace the answer with a reply to the hook
# (the 2026-09-13 `full` arm answered a stop-hook message about a plan file,
# not the question). The tracked tree is also what a stranger's clone has —
# no workspace/ artifacts for grep to stumble on. Cached by the subject's
# own commit sha (SESSION_SHA) and reused across cells/runs, exactly like
# the scratch daemon below — which is why this is called BEFORE it: the
# scratch daemon (scratch-daemon-default, VB-36) ingests THIS SAME export,
# never the live checkout, so a session and the daemon answering it read
# identical bytes.
prepare_session_root() {
  local sha
  sha="$(git -C "$SUBJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo nogit)"
  SESSION_SHA="$sha"
  SESSION_ROOT="$ROOT/workspace/tmp/bench-subject-$sha"
  if [ ! -d "$SESSION_ROOT" ]; then
    mkdir -p "$SESSION_ROOT"
    if git -C "$SUBJECT_ROOT" rev-parse HEAD >/dev/null 2>&1; then
      git -C "$SUBJECT_ROOT" archive HEAD | tar -x -C "$SESSION_ROOT"
    else
      cp -R "$SUBJECT_ROOT"/. "$SESSION_ROOT"/
    fi
    rm -f "$SESSION_ROOT/.claude/settings.json" "$SESSION_ROOT/.claude/settings.local.json"
  fi
  echo "context-bench: sessions run in $SESSION_ROOT (tracked tree at $sha, hooks removed)"
}
prepare_session_root

# ─── scratch daemon (PB-5; scratch-daemon-default, VB-36) ─────────────
# Never the fleet store, never the live checkout — see the header for the
# full rationale. SCRATCH_PID/cleanup/trap are declared unconditionally so
# a --no-scratch-daemon run's EXIT trap is still a harmless no-op.
SCRATCH_PID=""
cleanup() {
  if [ -n "$SCRATCH_PID" ]; then
    kill "$SCRATCH_PID" 2>/dev/null
    wait "$SCRATCH_PID" 2>/dev/null
    echo "context-bench: scratch daemon stopped"
  fi
}
trap cleanup EXIT

# LIVE_CHECKOUT_FOR_LEAK_CHECK is passed to `contextbenchscore score
# -live-checkout` (VB-36): non-empty only when the scratch daemon is
# actually serving the export, since a deliberate --no-scratch-daemon run
# against the fleet daemon is EXPECTED to answer from the live checkout —
# that is not a leak, it is the operator's own choice.
LIVE_CHECKOUT_FOR_LEAK_CHECK=""
# Only the wired arms ("fathom", "full") ever talk to a daemon at all — a
# --arms grep or --arms none run has no use for one, so the default flip
# to SCRATCH_DAEMON=1 must not start building/ingesting/serving one it
# will never be asked a question against.
if [ "$SCRATCH_DAEMON" -eq 1 ] && { echo "$ARMS" | grep -qw fathom || echo "$ARMS" | grep -qw full; }; then
  # The scratch fathom BINARY is built from fathom's own source at HEAD —
  # never a dirty working tree, and (for a cross-repo run, --subject-root
  # pointing elsewhere) never the external subject's export, which has no
  # cmd/fathom to build. When the subject IS this repo (the common,
  # default home-bench case) FATHOM_SRC_EXPORT is simply SESSION_ROOT
  # itself — no second export to pay for.
  if [ "$SUBJECT_ROOT" = "$ROOT" ]; then
    FATHOM_SRC_SHA="$SESSION_SHA"
    FATHOM_SRC_EXPORT="$SESSION_ROOT"
  else
    FATHOM_SRC_SHA="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo nogit)"
    FATHOM_SRC_EXPORT="$ROOT/workspace/tmp/bench-scratch-src-$FATHOM_SRC_SHA"
    if [ ! -d "$FATHOM_SRC_EXPORT" ]; then
      mkdir -p "$FATHOM_SRC_EXPORT"
      if git -C "$ROOT" rev-parse HEAD >/dev/null 2>&1; then
        git -C "$ROOT" archive HEAD | tar -x -C "$FATHOM_SRC_EXPORT"
      else
        cp -R "$ROOT"/. "$FATHOM_SRC_EXPORT"/
      fi
    fi
  fi

  # scratch-daemon-default: binary + store are cached under workspace/tmp/,
  # keyed by BOTH the fathom source's commit (what the binary can do) and
  # the subject export's commit (what the store contains) — reused across
  # cells, across --repeat, and across separate invocations of this script
  # for the same two shas, so a rebuild only happens when either changes.
  SCRATCH_CACHE_DIR="$ROOT/workspace/tmp/bench-scratch-$FATHOM_SRC_SHA-$SESSION_SHA"
  SCRATCH_BIN="$SCRATCH_CACHE_DIR/fathom"
  SCRATCH_STORE="$SCRATCH_CACHE_DIR/store.sqlite"
  SCRATCH_COMPILED_MARKER="$SCRATCH_CACHE_DIR/.compiled-obligations"
  mkdir -p "$SCRATCH_CACHE_DIR"

  if [ -x "$SCRATCH_BIN" ] && [ -s "$SCRATCH_STORE" ]; then
    echo "context-bench: reusing cached scratch daemon for fathom@$FATHOM_SRC_SHA / subject@$SESSION_SHA ($SCRATCH_CACHE_DIR) — build+ingest paid once per export"
  else
    echo "context-bench: building the scratch fathom binary FROM THE EXPORT $FATHOM_SRC_EXPORT (-buildvcs=false; never the live checkout)"
    # `go build` resolves an absolute/relative PATH argument against the
    # module rooted at the CURRENT directory — a target that merely
    # CONTAINS its own go.mod (the export does, since git archive HEAD
    # includes it) is refused as "outside main module" unless the build
    # actually runs FROM INSIDE that directory, so this cd's there rather
    # than passing $FATHOM_SRC_EXPORT/cmd/fathom as a path from $ROOT.
    if ! ( cd "$FATHOM_SRC_EXPORT" && GOWORK=off CGO_ENABLED=1 go build -buildvcs=false -o "$SCRATCH_BIN" ./cmd/fathom ); then
      echo "context-bench: failed to build cmd/fathom from the export" >&2
      exit 2
    fi
    echo "context-bench: ingesting the export $SESSION_ROOT as service '$SERVICE' into $SCRATCH_STORE (never $ROOT)"
    if ! "$SCRATCH_BIN" ingest -service "$SERVICE" -path "$SESSION_ROOT" -store "$SCRATCH_STORE" > "$RUN_DIR/scratch-ingest.log" 2>&1; then
      echo "context-bench: scratch ingest failed (see $RUN_DIR/scratch-ingest.log)" >&2
      exit 2
    fi
  fi

  # The obligations layer (PB-12's compiled-layer capability) is compiled
  # for real ONLY when the operator explicitly asked for compiled-layer
  # tasks (--env compiled-layer) — that flag is the existing "I accept LLM
  # spend for this run" signal (see `task bench-home`'s own COSTS TOKENS
  # warning), never triggered by a default invocation. Cached by the same
  # two shas as the binary/store above (SCRATCH_COMPILED_MARKER), so a
  # second cell, a --repeat, or a re-run of this script for the same
  # export never re-pays for the compile.
  if echo "$ENV" | tr ',' ' ' | grep -qw compiled-layer; then
    if [ -s "$SCRATCH_COMPILED_MARKER" ]; then
      echo "context-bench: obligations layer already compiled for fathom@$FATHOM_SRC_SHA / subject@$SESSION_SHA (compiled once per export) — reusing"
    else
      echo "context-bench: COSTS LLM TOKENS — compiling the obligations layer once for fathom@$FATHOM_SRC_SHA / subject@$SESSION_SHA (cached for every later cell/run against this export)"
      if ! "$SCRATCH_BIN" compile -service "$SERVICE" -layer obligations -store "$SCRATCH_STORE" -dry-run=false > "$RUN_DIR/scratch-compile.log" 2>&1; then
        echo "context-bench: obligations-layer compile failed (see $RUN_DIR/scratch-compile.log) — compiled-layer tasks will score as misses" >&2
      else
        date -u +%Y%m%dT%H%M%SZ > "$SCRATCH_COMPILED_MARKER"
      fi
    fi
  fi

  SCRATCH_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
  FATHOM_HTTP_URL="http://127.0.0.1:$SCRATCH_PORT"
  ( cd "$ROOT" && exec "$SCRATCH_BIN" serve-http -store "$SCRATCH_STORE" -addr "127.0.0.1:$SCRATCH_PORT" -auto-refresh=false -call-log "" ) > "$RUN_DIR/scratch-daemon.log" 2>&1 &
  SCRATCH_PID=$!
  ready=0
  for _ in $(seq 1 60); do
    if curl -sS -m 2 -o /dev/null "$FATHOM_HTTP_URL/rpc/agent" -X POST -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"capabilities","arguments":{}}}' 2>/dev/null; then
      ready=1; break
    fi
    sleep 0.5
  done
  if [ "$ready" -ne 1 ]; then
    echo "context-bench: scratch daemon did not become ready at $FATHOM_HTTP_URL (see $RUN_DIR/scratch-daemon.log)" >&2
    exit 2
  fi
  echo "context-bench: scratch daemon serving the export $SESSION_ROOT ($SCRATCH_STORE) at $FATHOM_HTTP_URL (pid $SCRATCH_PID) — no served path can name the live checkout $ROOT"
  LIVE_CHECKOUT_FOR_LEAK_CHECK="$ROOT"
elif [ "$SCRATCH_DAEMON" -eq 1 ]; then
  echo "context-bench: --arms '$ARMS' names no wired arm (fathom/full) — scratch daemon not started, nothing would talk to it"
else
  echo "context-bench: --no-scratch-daemon (or an explicit --fathom-url/--daemon-url) — talking to $FATHOM_HTTP_URL, the old fleet-daemon behavior; path_leak is not checked for this deliberate operator run"
fi

# ─── fathom-arm reachability preflight (honest degradation, B1-3) ─────────
# arm "full" talks to the same daemon as arm "fathom" (SB-6), so either
# name in --arms triggers this same preflight.
FATHOM_REACHABLE=0
if echo "$ARMS" | grep -qw fathom || echo "$ARMS" | grep -qw full; then
  if curl -sS -m 5 -o /dev/null -w '' "$FATHOM_HTTP_URL/rpc/agent" \
      -X POST -H 'Content-Type: application/json' \
      -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"capabilities","arguments":{}}}'; then
    FATHOM_REACHABLE=1
    echo "context-bench: fathom daemon reachable at $FATHOM_HTTP_URL/rpc/agent — arms 'fathom'/'full' will run for real"
  else
    echo "context-bench: WARNING — fathom daemon NOT reachable at $FATHOM_HTTP_URL/rpc/agent" >&2
    echo "context-bench: degrading honestly — arms 'fathom'/'full' are SKIPPED (not faked); the other arms still run for real" >&2
  fi
fi

FATHOM_MCP_CONFIG="$RUN_DIR/fathom-mcp.json"
cat > "$FATHOM_MCP_CONFIG" <<EOF
{"mcpServers":{"fathom":{"type":"http","url":"$FATHOM_HTTP_URL/rpc/agent","alwaysLoad":true}}}
EOF
EMPTY_MCP_CONFIG="$RUN_DIR/empty-mcp.json"
echo '{"mcpServers":{}}' > "$EMPTY_MCP_CONFIG"

# RC-7 clean room, `--settings`: one settings.json per arm under the run
# dir. Every arm is `{}` today (hook-free) — see the header comment; a
# future arm meant to carry the subject's own hook stack would render its
# file with that instead of this shared empty one.
GREP_SETTINGS="$RUN_DIR/settings-grep.json"
FATHOM_SETTINGS="$RUN_DIR/settings-fathom.json"
NONE_SETTINGS="$RUN_DIR/settings-none.json"
FULL_SETTINGS="$RUN_DIR/settings-full.json"
for f in "$GREP_SETTINGS" "$FATHOM_SETTINGS" "$NONE_SETTINGS" "$FULL_SETTINGS"; do
  echo '{}' > "$f"
done

# Tiny helpers (avoid fragile nested-quote python -c one-liners).
cat > "$RUN_DIR/print_result.py" <<'PY'
import json, sys
d = json.load(sys.stdin)
status = "PASS" if d["success"] else "FAIL"
print("context-bench: [{}/{}/t{}] {} tokens={} wall_ms={}".format(
    d["task_id"], d["arm"], d.get("trial", 1), status, d["total_tokens"], d["wall_time_ms"]))
PY
# stream-json → the final result envelope (the last {"type":"result"} event
# has the same shape as --output-format json's envelope).
cat > "$RUN_DIR/last_result.py" <<'PY'
import json, sys
last = None
for line in open(sys.argv[1], encoding="utf-8", errors="replace"):
    line = line.strip()
    if not line.startswith("{"):
        continue
    try:
        ev = json.loads(line)
    except ValueError:
        continue
    if ev.get("type") == "result":
        last = ev
if last is None:
    sys.exit(1)
json.dump(last, open(sys.argv[2], "w"))
PY

# The fathom arm's allowed surface is the live /rpc/agent registry
# (companionWant in internal/mcp/surface_test.go) minus two deliberate
# exclusions: ingest (never mutate the store from a worker script) and
# memory_search (continuity, not code facts — see the header).
FATHOM_ALLOWED="mcp__fathom__find_symbol mcp__fathom__find_callers mcp__fathom__references mcp__fathom__symbol_context mcp__fathom__outline mcp__fathom__repo_map mcp__fathom__pack_context mcp__fathom__entrypoints mcp__fathom__query_symbols mcp__fathom__capabilities mcp__fathom__list_services mcp__fathom__staleness_report mcp__fathom__verify_plan mcp__fathom__obligations mcp__fathom__read_source"
FATHOM_DISALLOWED="Bash Read Grep Glob Write Edit Task WebSearch WebFetch NotebookEdit mcp__fathom__ingest mcp__fathom__memory_search"
GREP_ALLOWED="Bash(grep:*) Bash(find:*) Bash(cat:*)"
GREP_DISALLOWED="Read Grep Glob Task WebSearch WebFetch Write Edit NotebookEdit"
NONE_ALLOWED=""
NONE_DISALLOWED="Bash Read Grep Glob Write Edit Task WebSearch WebFetch NotebookEdit"
# arm "full" (SB-6): the realistic arm — the same fathom MCP surface as arm
# "fathom" plus Bash/Read/Grep/Glob, no artificial tool-starvation. Still
# refuses ingest/memory_search (same reasons as arm "fathom") and every
# mutating/agentic tool the other arms also refuse.
# Read-only Bash forms only: the fleet daemon's answers name paths under the
# INDEXED root (the real checkout), so an unrestricted shell in this arm
# could write outside the export the session runs in.
FULL_ALLOWED="$FATHOM_ALLOWED Read Grep Glob Bash(grep:*) Bash(find:*) Bash(cat:*) Bash(ls:*) Bash(wc:*) Bash(head:*) Bash(tail:*) Bash(go doc:*)"
FULL_DISALLOWED="Write Edit Task WebSearch WebFetch NotebookEdit mcp__fathom__ingest mcp__fathom__memory_search"

# tools_from_allowed ALLOWED-STR — an arm's `--tools` value (RC-7),
# derived from its own *_ALLOWED variable above rather than hardcoded:
# extract each entry (a bare tool name, or a scoped `Bash(cmd:*)` rule —
# note some, like "Bash(go doc:*)", have a space inside the parens, hence
# the regex tokenizer rather than plain word-splitting), drop every mcp__
# entry (an MCP server's tools are already scoped by
# --mcp-config/--strict-mcp-config, not --tools), collapse a scoped rule
# down to its bare tool name (--tools takes a built-in tool, not an
# --allowedTools-style scope), then dedupe+sort what remains. An arm whose
# ALLOWED is 100% mcp__ entries (arm "fathom") derives to "" — `claude
# --tools ""` disables every built-in tool, which is what "exclusive
# fathom, never Bash/Read/Grep" means; --disallowedTools already denies
# them too, so this is belt-and-braces, not the only guard.
tools_from_allowed() {
  local allowed="$1"
  printf '%s' "$allowed" \
    | grep -oE '[A-Za-z_]+(\([^)]*\))?' \
    | grep -v '^mcp__' \
    | sed -E 's/\(.*$//' \
    | sort -u \
    | paste -sd, -
}
FATHOM_TOOLS="$(tools_from_allowed "$FATHOM_ALLOWED")"
GREP_TOOLS="$(tools_from_allowed "$GREP_ALLOWED")"
NONE_TOOLS="$(tools_from_allowed "$NONE_ALLOWED")"
FULL_TOOLS="$(tools_from_allowed "$FULL_ALLOWED")"

# run_with_timeout OUTFILE CMD...  — macOS's base install has no `timeout(1)`;
# this is the standard background-watchdog substitute. Runs CMD in the
# subject root, redirects its stdout to OUTFILE (stderr to OUTFILE.stderr),
# returns CMD's exit code, or 124 (GNU timeout's convention) if the
# watchdog had to kill it. SESSION_ROOT/prepare_session_root are set up
# above, before the scratch daemon block.
run_with_timeout() {
  local outfile="$1"; shift
  ( cd "$SESSION_ROOT" && exec "$@" ) < /dev/null > "$outfile" 2>"$outfile.stderr" &
  local pid=$!
  ( sleep "$TIMEOUT" && kill -TERM "$pid" 2>/dev/null ) &
  local watchdog=$!
  local rc=0
  wait "$pid" || rc=$?
  kill "$watchdog" 2>/dev/null
  wait "$watchdog" 2>/dev/null
  return "$rc"
}

CELLS_RUN=0
CELLS_KEPT=0
CELLS_BORROWED=0
run_arm() {
  local task_id="$1" prompt="$2" arm="$3" system_prompt="$4" mcp_config="$5" allowed="$6" disallowed="$7" settings_path="$8" tools="$9" trial="${10:-1}"
  local raw="$RAW_DIR/$task_id-$arm-t$trial.json"
  local stream="$RAW_DIR/$task_id-$arm-t$trial.stream.jsonl"
  local scored="$RESULTS_DIR/$task_id-$arm-t$trial.json"

  if [ -s "$scored" ]; then
    echo "context-bench: [$task_id/$arm/t$trial] kept (scored file exists)"
    CELLS_KEPT=$((CELLS_KEPT + 1))
    return 0
  fi

  local surface=""
  # SB-6: arm "full" keys its cells on the fathom commit exactly like arm
  # "fathom" — both arms talk to the same daemon, so a fathom change that
  # moves one moves the other.
  case "$arm" in
    fathom) surface="$FATHOM_ARM_SURFACE" ;;
    full) surface="$FATHOM_ARM_SURFACE" ;;
  esac

  # BP-4: an earlier run may already have bought this exact cell.
  if [ "$REUSE_CELLS" -eq 1 ]; then
    if "$SCORER" borrow -tasks "$TASKS" -task-id "$task_id" -arm "$arm" -trial "$trial" \
        -scoring "$SCORING" -cells "$CELLS_DIR" -out "$scored" \
        -model-id "$MODEL_ID" -harness-version "$HARNESS_VERSION" \
        -prompt-subject "$PROMPT_SUBJECT" -arm-surface "$surface"; then
      CELLS_BORROWED=$((CELLS_BORROWED + 1))
      return 0
    fi
  fi
  CELLS_RUN=$((CELLS_RUN + 1))

  echo "context-bench: [$task_id/$arm/t$trial] running..."
  # VB-36: every cell captures a stream-json transcript unconditionally now
  # — the path-leak/out-of-root-reads safety checks below need to see every
  # served tool_result and every Bash/Read tool_use, and --output-format
  # json's envelope carries neither (--stream is accepted for back-compat
  # but changes nothing; see the header). last_result.py still derives the
  # plain result envelope from the stream's last {"type":"result"} event.
  local -a fmt=(--output-format stream-json --verbose)
  local capture="$stream"
  if ! run_with_timeout "$capture" claude -p "$prompt" \
      --append-system-prompt "$system_prompt" \
      "${fmt[@]}" \
      --mcp-config "$mcp_config" \
      --strict-mcp-config \
      --restricted \
      --disable-slash-commands \
      --settings "$settings_path" \
      --tools "$tools" \
      --allowedTools "$allowed" \
      --disallowedTools "$disallowed" \
      --permission-mode default \
      --add-dir "$SESSION_ROOT" \
      --model "$MODEL"; then
    echo "context-bench: [$task_id/$arm/t$trial] claude -p exited non-zero (see $capture.stderr)" >&2
  fi
  if [ -s "$stream" ]; then
    python3 "$RUN_DIR/last_result.py" "$stream" "$raw" || true
  fi

  if [ ! -s "$raw" ]; then
    echo "context-bench: [$task_id/$arm/t$trial] no output captured — cell LOST (counted on the ledger row)" >&2
    return 1
  fi

  local -a extra=()
  if [ -s "$stream" ]; then
    extra=(-transcript "$stream" -live-checkout "$LIVE_CHECKOUT_FOR_LEAK_CHECK" -session-root "$SESSION_ROOT")
  fi
  if ! "$SCORER" score -tasks "$TASKS" -task-id "$task_id" -arm "$arm" -trial "$trial" -scoring "$SCORING" -claude-json "$raw" -out "$scored" \
      -cells "$CELLS_DIR" -run-id "$TS" -model-id "$MODEL_ID" -harness-version "$HARNESS_VERSION" \
      -prompt-subject "$PROMPT_SUBJECT" -arm-surface "$surface" "${extra[@]}"; then
    echo "context-bench: [$task_id/$arm/t$trial] scoring failed" >&2
    return 1
  fi
  python3 "$RUN_DIR/print_result.py" < "$scored"
  if python3 -c "import json,sys; d=json.load(open(sys.argv[1])); sys.exit(0 if d.get('path_leak') or d.get('out_of_root_reads') else 1)" "$scored" 2>/dev/null; then
    echo "context-bench: [$task_id/$arm/t$trial] FLAGGED — see path_leak/out_of_root_reads in $scored" >&2
  fi
}

# ─── main loop ──────────────────────────────────────────────────────────
ANSWER_FORM="Answer concisely, citing exact file paths / symbol names / IDs, since your answer is scored by substring match against a ground truth."
if [ "$SCORING" = "precision" ]; then
  ANSWER_FORM="Put your final answer in a single fenced code block (three backticks), one item per line — exact file paths / symbol names / IDs only, no commentary inside the block. Only the last fenced block is scored, its lines are counted against a small cap, and an answer without a fenced block scores zero."
fi
FATHOM_SYSTEM_PROMPT="You are answering one factual question about $PROMPT_SUBJECT using ONLY the fathom MCP tools available to you; the subject is indexed as service \"$SERVICE\". Do not use Bash, Read, Grep, Glob, or any other tool — solve the task purely through fathom tool calls. $ANSWER_FORM"
GREP_SYSTEM_PROMPT="You are answering one factual question about $PROMPT_SUBJECT. You may ONLY use grep, find, and cat, invoked via the Bash tool — no Read tool, no Grep tool, no Glob tool, no MCP tools of any kind. $ANSWER_FORM"
NONE_SYSTEM_PROMPT="You are answering one factual question about $PROMPT_SUBJECT from your own knowledge only. You have no tools: do not attempt to read files, run commands, or call any tool. If you do not know, say so. $ANSWER_FORM"
FULL_SYSTEM_PROMPT="You are answering one factual question about $PROMPT_SUBJECT; the subject is indexed as service \"$SERVICE\". You have the fathom MCP tools AND Bash/Read/Grep/Glob available — use whichever gets you to a confident answer fastest. $ANSWER_FORM"

TASK_COUNT=0
while IFS= read -r -d '' task_id && IFS= read -r -d '' prompt && IFS= read -r -d '' task_min_trials; do
  if [ -n "$ONLY" ] && ! echo " $ONLY " | grep -q " $task_id "; then
    continue
  fi
  TASK_COUNT=$((TASK_COUNT + 1))
  # RC-18: a task's own registered min_trials (the floor for tasks flaky at
  # the corpus default n) overrides --repeat when it asks for MORE trials —
  # never fewer, so an operator's explicit --repeat never gets cut down.
  # 0 (the common case: the task sets none) means "use --repeat as given".
  task_trials="$REPEAT"
  case "$task_min_trials" in
    ''|*[!0-9]*) ;;
    *) [ "$task_min_trials" -gt "$task_trials" ] && task_trials="$task_min_trials" ;;
  esac
  trial=1
  while [ "$trial" -le "$task_trials" ]; do
    if echo "$ARMS" | grep -qw grep; then
      run_arm "$task_id" "$prompt" grep "$GREP_SYSTEM_PROMPT" "$EMPTY_MCP_CONFIG" "$GREP_ALLOWED" "$GREP_DISALLOWED" "$GREP_SETTINGS" "$GREP_TOOLS" "$trial" || true
    fi
    if echo "$ARMS" | grep -qw fathom; then
      if [ "$FATHOM_REACHABLE" -eq 1 ]; then
        run_arm "$task_id" "$prompt" fathom "$FATHOM_SYSTEM_PROMPT" "$FATHOM_MCP_CONFIG" "$FATHOM_ALLOWED" "$FATHOM_DISALLOWED" "$FATHOM_SETTINGS" "$FATHOM_TOOLS" "$trial" || true
      fi
    fi
    if echo "$ARMS" | grep -qw none; then
      run_arm "$task_id" "$prompt" none "$NONE_SYSTEM_PROMPT" "$EMPTY_MCP_CONFIG" "$NONE_ALLOWED" "$NONE_DISALLOWED" "$NONE_SETTINGS" "$NONE_TOOLS" "$trial" || true
    fi
    if echo "$ARMS" | grep -qw full; then
      if [ "$FATHOM_REACHABLE" -eq 1 ]; then
        run_arm "$task_id" "$prompt" full "$FULL_SYSTEM_PROMPT" "$FATHOM_MCP_CONFIG" "$FULL_ALLOWED" "$FULL_DISALLOWED" "$FULL_SETTINGS" "$FULL_TOOLS" "$trial" || true
      fi
    fi
    trial=$((trial + 1))
  done
done < <("$SCORER" tasks -tasks "$TASKS" -env "$ENV")

echo "context-bench: $TASK_COUNT runnable task(s) x >=$REPEAT trial(s) per arm (a task's own min_trials, RC-18, may run more; env='$ENV', scope=$SCOPE); cells run=$CELLS_RUN kept=$CELLS_KEPT borrowed=$CELLS_BORROWED"

if [ "$FATHOM_REACHABLE" -eq 0 ] && { echo "$ARMS" | grep -qw fathom || echo "$ARMS" | grep -qw full; }; then
  echo "context-bench: arm 'fathom'/'full' rows are ABSENT from this run's results (daemon unreachable) — do not report them as measured" >&2
fi

# ─── aggregate + render (only if we have anything to aggregate) ──────────
if [ -z "$(ls -A "$RESULTS_DIR" 2>/dev/null)" ]; then
  echo "context-bench: no scored results produced — nothing to aggregate" >&2
  exit 1
fi

RESULTS_JSON="$ARTIFACTS_DIR/results-$TS.json"
SUMMARY_MD="$ARTIFACTS_DIR/summary-$TS.md"
"$SCORER" report -tasks "$TASKS" -results "$RESULTS_DIR" -out-json "$RESULTS_JSON" -out-md "$SUMMARY_MD"

# ─── ledger: pair the arms, evaluate the COMMITTED criterion, record ─────
# The corpus's registered criterion is the only thing permitted to declare
# a win, and it is read from the committed corpus — never passed in here,
# so this script cannot loosen a bar it just measured against. Exit 3 =
# indeterminate (re-run), 1 = a measured refutation (record it and say so).
LEDGER_STATUS="skipped"
if python3 -c "import json,sys; sys.exit(0 if json.load(open(sys.argv[1])).get('machine_criterion') else 1)" "$TASKS" 2>/dev/null; then
  mkdir -p "$(dirname "$LEDGER")"
  COMMIT="$(git -C "$SUBJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
  DIRTY=()
  if [ -n "$(git -C "$SUBJECT_ROOT" status --porcelain 2>/dev/null)" ]; then
    DIRTY=(-dirty-tree)
  fi
  CRIT=()
  if [ -n "$CRITERION" ]; then
    CRIT=(-criterion "$CRITERION")
  fi
  # No `set -e` is in effect (see the header's `set -uo pipefail`), so the
  # non-zero verdict exit codes are captured directly.
  LEDGER_FLAGS=()
  if [ "$LEDGER_EXPLICIT" -eq 1 ]; then
    LEDGER_FLAGS=(-ledger-explicit)
  fi
  "$SCORER" ledger -tasks "$TASKS" -results "$RESULTS_DIR" -ledger "$LEDGER" -treatment "$TREATMENT" \
    -run-id "$TS" -commit "$COMMIT" -model "$MODEL" -arms "$ARMS" -repeat "$REPEAT" -env "$ENV" \
    -scope "$SCOPE" -scoring "$SCORING" -raw "$RAW_DIR" -only "$ONLY" \
    -artifacts "$ARTIFACTS_DIR" "${LEDGER_FLAGS[@]}" \
    -model-id "$MODEL_ID" -harness-version "$HARNESS_VERSION" \
    -prompt-subject "$PROMPT_SUBJECT" -arm-surface "$FATHOM_ARM_SURFACE" \
    "${DIRTY[@]}" "${CRIT[@]}"
  LEDGER_RC=$?
  # The committed README is prose around a marker-fenced table; only the
  # marker renderer may touch it (a whole-file -out-md once replaced the
  # prose with the bare table, BP-38). A workspace ledger renders nothing.
  if [ "$LEDGER" = "$ROOT/docs/bench/ledger.jsonl" ]; then
    bash "$ROOT/scripts/bench-ledger-render.sh" "$LEDGER" || echo "context-bench: README table not re-rendered (run: task bench-ledger)" >&2
  fi
  case "$LEDGER_RC" in
    0) LEDGER_STATUS="PASS" ;;
    1) LEDGER_STATUS="FAIL (measured refutation — record it)" ;;
    3) LEDGER_STATUS="INDETERMINATE (the run did not measure enough — re-run, or --resume $RUN_DIR to fill lost cells)" ;;
    *) LEDGER_STATUS="ledger error rc=$LEDGER_RC" ;;
  esac
else
  echo "context-bench: corpus has no machine_criterion — no ledger row written (a run with no registered bar scores nothing)" >&2
fi

echo "context-bench: done"
echo "  raw transcripts + per-task scores: $RUN_DIR"
echo "  results:                           $RESULTS_JSON"
echo "  summary:                           $SUMMARY_MD"
echo "  criterion verdict:                 $LEDGER_STATUS"
if [ "$LEDGER_REDIRECTED" -eq 1 ]; then
  echo "  ledger (targeted, NOT committed):  $LEDGER"
  echo "  committed ledger:                  untouched ($ROOT/docs/bench/ledger.jsonl) — pass --ledger explicitly to record there"
else
  echo "  ledger:                            $LEDGER"
fi
