#!/usr/bin/env bash
#
# context-bench_test.sh — shell-level test for scripts/context-bench.sh's
# scratch-daemon-default behavior (VB-36, O47, review round 2). Runs the
# REAL script against THIS repo (context-bench.sh is tied to this repo's
# own Go source tree — cmd/fathom, cmd/contextbenchscore — so, unlike
# vendor-check_test.sh, there is no cheap throwaway fixture repo to
# substitute), but with the `claude` binary on PATH replaced by a fake
# harness that prints a fixed, deterministic stream-json transcript
# instead of calling any model. The fake harness never enforces the real
# --tools/--allowedTools restriction (it prints canned output regardless
# of what it is told); that is a harness/product behavior no fake can
# exercise without a real API call, and it is not what this test proves.
#
# NO MODEL SPEND ANYWHERE IN THIS FILE. compiled-layer-by-default (VB-36
# review round 2) means a plain default invocation of context-bench.sh
# now ALSO tries to compile the obligations layer for real
# (`fathom compile -dry-run=false`, a real API call) whenever a wired arm
# ("fathom"/"full") is requested and no compile is cached yet. Every
# phase below that requests a wired arm therefore passes `--env raw`
# (deterministic tasks only, no compile) EXCEPT phase 3, which instead
# PRE-SEEDS the compile-cache marker file before running with the
# (default) compiled-layer env — proving the cache short-circuits the
# real compile call (never reaching it) rather than proving the compile
# itself works. Phase 4 (the default-env line, and the 13-task count)
# uses ONLY the grep arm, which never enters the scratch-daemon/compile
# code path at all regardless of --env, so no flag is needed there for
# safety — it is the cheapest possible way to observe the genuine default.
#
# This test proves:
#
#   1. scratch-daemon-default is genuinely the default: no --scratch-daemon
#      flag is passed, yet the log shows a scratch fathom binary built
#      FROM AN EXPORT and served on a 127.0.0.1 port — never the fleet
#      daemon's :7575 default.
#   2. The export (SESSION_ROOT) — now under
#      `${TMPDIR:-/tmp}/fathom-context-bench/`, OUTSIDE this checkout
#      (finding 1a) — contains no occurrence of the live checkout's own
#      absolute path anywhere in its files.
#   3. The scratch build+ingest is cached by export sha and REUSED on a
#      second invocation against the same commit (no rebuild, no
#      re-ingest).
#   4. End to end: a fake transcript that (a) serves a tool_result naming
#      the live checkout's absolute path and (b) calls Bash/Read with an
#      absolute path outside the session root (single-segment, finding 3)
#      produces a scored cell with path_leak: true and
#      out_of_root_reads >= 2, naming the exact paths — proving the shell
#      (`-live-checkout`/`-session-root` flags) is wired to the Go scorer,
#      not just unit-tested in isolation (pathcheck_test.go covers the Go
#      functions on their own, including both the nested and non-nested
#      session-root layouts, finding 1b).
#   5. compiled-layer-by-default (finding 2): a run with no --env flag at
#      all resolves env to "compiled-layer" (the run summary's "env in
#      effect" line) and the committed home corpus admits all 13 home
#      tasks (8 deterministic + 5 requires:[compiled-layer]) under it —
#      the pre-scratch-default behavior, restored.
#   6. compiled-layer-by-default's cache: with the compile marker already
#      present (simulating an earlier real compile that this test never
#      performs), a compiled-layer run recognizes it, logs "reusing", and
#      never attempts the real `fathom compile` call — the run summary's
#      compile status reads "cached".
#
# Cleans up every path it creates under
# `${TMPDIR:-/tmp}/fathom-context-bench/` and
# `workspace/artifacts/context-bench/` — a fresh measurement every time,
# and the real repo's workspace/ is left exactly as it was found.
#
# Usage:
#   bash scripts/context-bench_test.sh

set -uo pipefail

# Defense in depth for phase 3 (below): even with the marker-file guard,
# never let a real LLM credential be reachable from this test's own
# environment — an `ANTHROPIC_API_KEY`/OPENAI/Azure key present on the
# machine running this test must not let a bug here reach a real
# provider. This is belt-and-braces on top of the marker check itself,
# not a substitute for it.
unset ANTHROPIC_API_KEY ANTHROPIC_DEFAULT_KEY OPENAI_API_KEY AZURE_OPENAI_KEY AZURE_OPENAI_ENDPOINT

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

pass=0
ok() {
  pass=$((pass + 1))
  echo "ok — $1"
}
check_contains() {
  local desc="$1" file="$2" needle="$3"
  grep -qF "${needle}" "${file}" || fail "${desc}: expected to find '${needle}' in ${file}"
  ok "${desc}"
}
check_not_contains() {
  local desc="$1" file="$2" needle="$3"
  if grep -qF "${needle}" "${file}"; then
    fail "${desc}: expected NOT to find '${needle}' in ${file}"
  fi
  ok "${desc}"
}

if [ ! -x "${ROOT}/scripts/context-bench.sh" ]; then
  fail "scripts/context-bench.sh not found under ${ROOT}"
fi

HEAD_SHA="$(git -C "${ROOT}" rev-parse --short HEAD 2>/dev/null || echo nogit)"
# Finding 1a: the export and the scratch cache now live OUTSIDE the
# checkout (scripts/stranger-check.sh's own ${TMPDIR:-/tmp} precedent),
# never under workspace/tmp/ — see context-bench.sh's prepare_session_root.
SESSION_ROOT_DIR="${TMPDIR:-/tmp}/fathom-context-bench/subject-${HEAD_SHA}"
SCRATCH_CACHE_DIR="${TMPDIR:-/tmp}/fathom-context-bench/scratch-${HEAD_SHA}-${HEAD_SHA}"

TMP="$(mktemp -d)"
RUN1_DIRS_BEFORE="$(mktemp)"
cleanup() {
  # Kill any scratch daemon this test's own runs left behind (belt and
  # braces: context-bench.sh's own EXIT trap already does this per
  # invocation, but a killed test process could skip it).
  pkill -f "fathom-context-bench/scratch-${HEAD_SHA}-${HEAD_SHA}/fathom serve-http" 2>/dev/null
  rm -rf "${SESSION_ROOT_DIR}" "${SCRATCH_CACHE_DIR}"
  # The content-addressed cell store is a SHARED, persistent cache
  # (workspace/artifacts/context-bench/cells/<key>/) keyed by a hash that
  # never collides with a real run's (this test's fake harness version
  # string and prompt-subjects are test-only), but leaving a stray entry
  # behind is still litter. MUST run before the run-* dirs below are
  # deleted: the scored JSON this reads each key from lives inside one.
  local rundir
  find "${ROOT}/workspace/artifacts/context-bench" -maxdepth 1 -name 'run-*' -newer "${RUN1_DIRS_BEFORE}" 2>/dev/null | while IFS= read -r rundir; do
    [ -d "${rundir}/results" ] || continue
    for scored in "${rundir}/results"/*.json; do
      [ -s "${scored}" ] || continue
      key="$(python3 -c "import json,sys; print(json.load(open(sys.argv[1])).get('cell_key',''))" "${scored}" 2>/dev/null)"
      [ -n "${key}" ] && rm -rf "${ROOT}/workspace/artifacts/context-bench/cells/${key}"
    done
  done
  find "${ROOT}/workspace/artifacts/context-bench" -maxdepth 1 -name 'run-*' -newer "${RUN1_DIRS_BEFORE}" -exec rm -rf {} + 2>/dev/null
  find "${ROOT}/workspace/artifacts/context-bench" -maxdepth 1 \( -name 'results-*' -o -name 'summary-*' \) -newer "${RUN1_DIRS_BEFORE}" -exec rm -f {} + 2>/dev/null
  rm -rf "${TMP}" "${RUN1_DIRS_BEFORE}"
}
trap cleanup EXIT

if [ -e "${SESSION_ROOT_DIR}" ] || [ -e "${SCRATCH_CACHE_DIR}" ]; then
  fail "a subject/scratch cache for HEAD ${HEAD_SHA} already exists — refusing to clobber another run's cache; remove ${SESSION_ROOT_DIR} / ${SCRATCH_CACHE_DIR} first"
fi

# ─── fixtures ───────────────────────────────────────────────────────────
TASKS_FIXTURE="${TMP}/tasks.json"
cat > "${TASKS_FIXTURE}" <<'JSON'
{
  "version": 1,
  "description": "context-bench_test.sh fixture — one deterministic task, no LLM ground truth to protect, no machine_criterion (so the ledger step is skipped entirely).",
  "tasks": [
    {
      "id": "T-FAKE",
      "prompt": "What is the fake gold token?",
      "ground_truth": ["FAKE_GOLD_TOKEN"],
      "scoring_rule": "exact-match"
    }
  ]
}
JSON

# The fake `claude` harness: no model, no network. `--version` answers with
# a fixed harness string (the cell key needs one); a `-p` call ignores
# every flag and prints a fixed stream-json transcript naming the LIVE
# checkout's absolute path from BOTH a served tool_result (path_leak) and
# two Bash/Read tool inputs outside the session root (out_of_root_reads,
# one of them SINGLE-SEGMENT — `cd $ROOT && ...` — finding 3) — the exact
# O47 shape, reproduced on purpose so this test proves the runner catches
# it rather than merely asserting the Go functions do.
FAKE_BIN="${TMP}/bin"
mkdir -p "${FAKE_BIN}"
cat > "${FAKE_BIN}/claude" <<EOF
#!/usr/bin/env bash
if [ "\${1:-}" = "--version" ]; then
  echo "2.1.270 (Claude Code) [context-bench_test.sh fake harness]"
  exit 0
fi
cat <<'JSON'
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"cd ${ROOT} && cat CLAUDE.md"}},{"type":"tool_use","id":"2","name":"Read","input":{"file_path":"${ROOT}/ADR.md"}}],"usage":{"input_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":5}}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served payload names ${ROOT}/CLAUDE.md directly, the live checkout, not the export"},{"type":"tool_result","tool_use_id":"2","content":"adr body text"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"The answer is FAKE_GOLD_TOKEN."}],"usage":{"input_tokens":5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":5}}}
{"type":"result","subtype":"success","is_error":false,"num_turns":2,"result":"The answer is FAKE_GOLD_TOKEN.","duration_ms":1234,"total_cost_usd":0.001,"usage":{"input_tokens":15,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10},"modelUsage":{"claude-sonnet-test":{"outputTokens":10}}}
JSON
EOF
chmod +x "${FAKE_BIN}/claude"

export PATH="${FAKE_BIN}:${PATH}"

# ─── phase 1: cold cache, --env raw (no compile, no spend) ──────────────
OUT1="${TMP}/run1.log"
bash "${ROOT}/scripts/context-bench.sh" \
  --tasks "${TASKS_FIXTURE}" \
  --arms fathom \
  --repeat 1 \
  --env raw \
  --service context-bench-test \
  --prompt-subject 'the fathom codebase (context-bench_test.sh fixture)' \
  --no-reuse-cells \
  > "${OUT1}" 2>&1
rc1=$?
[ "${rc1}" -eq 0 ] || { cat "${OUT1}" >&2; fail "run 1 exited ${rc1} (see log above)"; }

# 1. scratch-daemon-default: no --scratch-daemon flag was passed, yet a
# scratch daemon was built (cache miss on the first run) and served on a
# loopback port — never the fleet daemon's :7575.
check_contains "run 1: scratch daemon is the default (built from an export)" "${OUT1}" \
  "building the scratch fathom binary FROM THE EXPORT"
check_contains "run 1: scratch daemon served on a 127.0.0.1 loopback port" "${OUT1}" \
  "scratch daemon serving the export"
check_not_contains "run 1: never talks to the fleet daemon's default port" "${OUT1}" \
  "localhost:7575"
check_contains "run 1: scratch daemon confirms no served path names the live checkout" "${OUT1}" \
  "no served path can name the live checkout"
check_not_contains "run 1: --env raw never attempts the real obligations compile" "${OUT1}" \
  "COSTS LLM TOKENS"
check_contains "run 1: summary reports --env raw as not-applicable to the compile" "${OUT1}" \
  "obligations compile: n/a (--env 'raw' has no compiled-layer)"

RUN1_DIR="$(grep -oE 'workspace/artifacts/context-bench/run-[0-9TZ]+' "${OUT1}" | head -1)"
[ -n "${RUN1_DIR}" ] || fail "could not find run 1's RUN_DIR in its log"
SCORED1="${ROOT}/${RUN1_DIR}/results/T-FAKE-fathom-t1.json"
[ -s "${SCORED1}" ] || fail "run 1: scored cell not found at ${SCORED1}"

# 2. the export contains no occurrence of the live checkout's own absolute
# path anywhere in its files (finding 1a — it no longer nests under $ROOT
# at all, so this is now trivially true by construction, but assert it
# directly rather than trusting the construction).
if grep -rlF "${ROOT}" "${SESSION_ROOT_DIR}" --exclude-dir=.git 2>/dev/null | grep -q .; then
  fail "the export at ${SESSION_ROOT_DIR} contains the live checkout's own absolute path — it must only ever contain the committed tree's own content"
fi
ok "the export contains no occurrence of the live checkout's absolute path"

# 4a. path_leak: the served tool_result named the live checkout's path,
# which is NOT under the (non-nested) session root, so it is correctly a
# leak.
python3 - "${SCORED1}" "${ROOT}/CLAUDE.md" <<'PY' || fail "scored cell: path_leak was not set true, or did not name the leaked path"
import json, sys
d = json.load(open(sys.argv[1]))
assert d.get("path_leak") is True, d.get("path_leak")
assert any(sys.argv[2] in p for p in d.get("path_leak_paths", [])), d.get("path_leak_paths")
PY
ok "scored cell: path_leak true, naming the live checkout path actually served"

# 4b. out_of_root_reads: the Bash (single-segment: "cd $ROOT", finding 3)
# and Read tool inputs both named an absolute path outside SESSION_ROOT.
python3 - "${SCORED1}" "${ROOT}" <<'PY' || fail "scored cell: out_of_root_reads did not count both the Bash and Read escapes"
import json, sys
d = json.load(open(sys.argv[1]))
root = sys.argv[2]
assert d.get("out_of_root_reads", 0) >= 2, d.get("out_of_root_reads")
paths = d.get("out_of_root_paths", [])
assert any(p.startswith("Bash:") and root in p for p in paths), paths
assert any(p.startswith("Read:") and root in p for p in paths), paths
PY
ok "scored cell: out_of_root_reads counts the single-segment Bash escape and the Read escape by tool"

# ─── phase 2: warm cache (same HEAD), --env raw ─────────────────────────
OUT2="${TMP}/run2.log"
bash "${ROOT}/scripts/context-bench.sh" \
  --tasks "${TASKS_FIXTURE}" \
  --arms fathom \
  --repeat 1 \
  --env raw \
  --service context-bench-test \
  --prompt-subject 'the fathom codebase (context-bench_test.sh fixture)' \
  --no-reuse-cells \
  > "${OUT2}" 2>&1
rc2=$?
[ "${rc2}" -eq 0 ] || { cat "${OUT2}" >&2; fail "run 2 exited ${rc2} (see log above)"; }

# 3. the second invocation against the SAME commit reuses the cached
# scratch binary+store instead of rebuilding/re-ingesting.
check_contains "run 2: scratch daemon build+ingest is reused, not repeated" "${OUT2}" \
  "reusing cached scratch daemon"
check_not_contains "run 2: does not rebuild the scratch fathom binary" "${OUT2}" \
  "building the scratch fathom binary FROM THE EXPORT"

# ─── phase 3: compiled-layer-by-default's CACHE, no real compile call ───
# Pre-seed the compile marker (simulating an earlier real compile this
# test never performs) and run WITHOUT --env raw — the new default,
# --env compiled-layer, applies. If the cache did not short-circuit, this
# would attempt a real `fathom compile -dry-run=false` call; asserting the
# COSTS-LLM-TOKENS line never appears is what proves it did not.
#
# The marker check in context-bench.sh is `[ -s "$SCRATCH_COMPILED_MARKER" ]`
# — SIZE greater than zero, not merely EXISTS. An earlier revision of this
# test wrote an EMPTY marker (`: > file`), which made `-s` false and let a
# real `fathom compile -dry-run=false` process actually START before being
# caught and killed by hand (reported in the VB-36 fix-first round rather
# than recorded here — this file's own git history predates a clean
# working tree to append an incident to). The marker MUST be non-empty,
# and this asserts that directly (fail-closed) rather than trusting the
# write, before phase 3 ever runs.
date -u +%Y%m%dT%H%M%SZ > "${SCRATCH_CACHE_DIR}/.compiled-obligations"
[ -s "${SCRATCH_CACHE_DIR}/.compiled-obligations" ] || fail "pre-seeded compile marker is not non-empty — refusing to run phase 3 (would risk a real compile call)"
OUT3="${TMP}/run3.log"
bash "${ROOT}/scripts/context-bench.sh" \
  --tasks "${TASKS_FIXTURE}" \
  --arms fathom \
  --repeat 1 \
  --service context-bench-test \
  --prompt-subject 'the fathom codebase (context-bench_test.sh fixture)' \
  --no-reuse-cells \
  > "${OUT3}" 2>&1
rc3=$?
[ "${rc3}" -eq 0 ] || { cat "${OUT3}" >&2; fail "run 3 exited ${rc3} (see log above)"; }

check_contains "run 3: compiled-layer is the default (no --env flag passed)" "${OUT3}" \
  "obligations layer already compiled"
check_contains "run 3: the cached compile is reused" "${OUT3}" \
  "reusing"
check_not_contains "run 3: the pre-seeded cache means the real compile is NEVER attempted" "${OUT3}" \
  "COSTS LLM TOKENS"
check_contains "run 3: summary reports the compile as cached" "${OUT3}" \
  "obligations compile: cached"

# ─── phase 4: the default env, and the 13-home-task count — grep arm only,
# which never enters the scratch-daemon/compile code path regardless of
# --env, so no flag is needed here for safety. ───────────────────────────
OUT4="${TMP}/run4.log"
bash "${ROOT}/scripts/context-bench.sh" \
  --tasks "${TASKS_FIXTURE}" \
  --arms grep \
  --repeat 1 \
  --service context-bench-test \
  --prompt-subject 'the fathom codebase (context-bench_test.sh fixture)' \
  --no-reuse-cells \
  > "${OUT4}" 2>&1
rc4=$?
[ "${rc4}" -eq 0 ] || { cat "${OUT4}" >&2; fail "run 4 exited ${rc4} (see log above)"; }
check_contains "run 4: a default invocation (no --env flag) resolves env to compiled-layer" "${OUT4}" \
  "compiled-layer (obligations compile: n/a (no scratch daemon this run))"

# 5b. the committed home corpus, under env=compiled-layer, admits all 13
# home tasks (8 deterministic + 5 requires:[compiled-layer]) — the number
# a default `task bench-home` now covers. Reuses run 1's already-built
# scorer binary rather than paying for a second `go build`.
SCORER_BIN="${ROOT}/${RUN1_DIR}/contextbenchscore"
[ -x "${SCORER_BIN}" ] || fail "run 1's contextbenchscore binary not found at ${SCORER_BIN}"
HOME_TASK_COUNT="$(
  "${SCORER_BIN}" tasks -tasks "${ROOT}/testdata/contextbench/tasks-home.json" -env compiled-layer 2>/dev/null \
    | tr -cd '\0' | wc -c
)"
HOME_TASK_COUNT=$((HOME_TASK_COUNT / 3))
if [ "${HOME_TASK_COUNT}" -ne 13 ]; then
  fail "env=compiled-layer should admit all 13 home tasks (8 deterministic + 5 requires:[compiled-layer]), got ${HOME_TASK_COUNT}"
fi
ok "env=compiled-layer admits all 13 home tasks — a default \`task bench-home\` run now covers all of them"

echo "context-bench_test.sh: ${pass} checks passed"
