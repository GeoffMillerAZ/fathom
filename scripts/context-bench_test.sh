#!/usr/bin/env bash
#
# context-bench_test.sh — shell-level test for scripts/context-bench.sh's
# scratch-daemon-default behavior (VB-36, O47). Runs the REAL script
# against THIS repo (context-bench.sh is tied to this repo's own Go source
# tree — cmd/fathom, cmd/contextbenchscore — so, unlike vendor-check_test.sh,
# there is no cheap throwaway fixture repo to substitute), but with the
# `claude` binary on PATH replaced by a fake harness that prints a fixed,
# deterministic stream-json transcript instead of calling any model — NO
# MODEL, NO NETWORK, NO SPEND. The fake harness never enforces the real
# --tools/--allowedTools restriction (it prints canned output regardless of
# what it is told); that is a harness/product behavior no fake can exercise
# without a real API call, and it is not what this test is proving. This
# test proves the runner's OWN plumbing:
#
#   1. scratch-daemon-default is genuinely the default: no --scratch-daemon
#      flag is passed, yet the log shows a scratch fathom binary built
#      FROM AN EXPORT and served on a 127.0.0.1 port — never the fleet
#      daemon's :7575 default.
#   2. The export (SESSION_ROOT) contains no occurrence of the live
#      checkout's own absolute path anywhere in its files.
#   3. The scratch build+ingest is cached by export sha and REUSED on a
#      second invocation against the same commit (no rebuild, no
#      re-ingest) — the mechanism `--repeat`/multi-cell reuse and the
#      (separately gated, --env compiled-layer, never exercised here)
#      obligations-layer compile both rely on.
#   4. End to end: a fake transcript that (a) serves a tool_result naming
#      the live checkout's absolute path and (b) calls Bash/Read with an
#      absolute path outside the session root produces a scored cell with
#      path_leak: true and out_of_root_reads > 0, naming the exact paths —
#      proving the shell (`-live-checkout`/`-session-root` flags) is wired
#      to the Go scorer (DetectPathLeak/DetectOutOfRootReads), not just
#      unit-tested in isolation (internal/contextbench/pathcheck_test.go
#      covers the Go functions on their own).
#
# --env compiled-layer is NEVER passed here: that is the one path in
# context-bench.sh that can spend real LLM tokens (`fathom compile
# -dry-run=false`), and this test must never trigger it.
#
# Cleans up every workspace/ path it creates (bench-subject-<sha>,
# bench-scratch-<sha>-<sha>, run-<ts>, results/summary artifacts) — a
# fresh measurement every time, and the real repo's workspace/ is left
# exactly as it was found.
#
# Usage:
#   bash scripts/context-bench_test.sh

set -uo pipefail

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
SESSION_ROOT_DIR="${ROOT}/workspace/tmp/bench-subject-${HEAD_SHA}"
SCRATCH_CACHE_DIR="${ROOT}/workspace/tmp/bench-scratch-${HEAD_SHA}-${HEAD_SHA}"

TMP="$(mktemp -d)"
RUN1_DIRS_BEFORE="$(mktemp)"
cleanup() {
  # Kill any scratch daemon this test's own runs left behind (belt and
  # braces: context-bench.sh's own EXIT trap already does this per
  # invocation, but a killed test process could skip it).
  pkill -f "workspace/tmp/bench-scratch-${HEAD_SHA}-${HEAD_SHA}/fathom serve-http" 2>/dev/null
  rm -rf "${SESSION_ROOT_DIR}" "${SCRATCH_CACHE_DIR}"
  # The content-addressed cell store is a SHARED, persistent cache
  # (workspace/artifacts/context-bench/cells/<key>/) keyed by a hash that
  # never collides with a real run's (this fixture's task id and
  # prompt-subject are test-only strings), but leaving a stray entry
  # behind is still litter — remove the specific key(s) this run wrote.
  # MUST run before the run-* dirs below are deleted: the scored JSON this
  # reads the key from lives inside one of them.
  for scored in "${SCORED1:-}" "${SCORED2:-}"; do
    [ -s "${scored}" ] || continue
    key="$(python3 -c "import json,sys; print(json.load(open(sys.argv[1])).get('cell_key',''))" "${scored}" 2>/dev/null)"
    [ -n "${key}" ] && rm -rf "${ROOT}/workspace/artifacts/context-bench/cells/${key}"
  done
  find "${ROOT}/workspace/artifacts/context-bench" -maxdepth 1 -name 'run-*' -newer "${RUN1_DIRS_BEFORE}" -exec rm -rf {} + 2>/dev/null
  find "${ROOT}/workspace/artifacts/context-bench" -maxdepth 1 \( -name 'results-*' -o -name 'summary-*' \) -newer "${RUN1_DIRS_BEFORE}" -exec rm -f {} + 2>/dev/null
  rm -rf "${TMP}" "${RUN1_DIRS_BEFORE}"
}
trap cleanup EXIT

if [ -e "${SESSION_ROOT_DIR}" ] || [ -e "${SCRATCH_CACHE_DIR}" ]; then
  fail "a bench-subject/bench-scratch cache for HEAD ${HEAD_SHA} already exists — refusing to clobber another run's cache; remove ${SESSION_ROOT_DIR} / ${SCRATCH_CACHE_DIR} first"
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
# two Bash/Read tool inputs outside the session root (out_of_root_reads) —
# the exact O47 shape, reproduced on purpose so this test proves the
# runner catches it rather than merely asserting the Go functions do.
FAKE_BIN="${TMP}/bin"
mkdir -p "${FAKE_BIN}"
cat > "${FAKE_BIN}/claude" <<EOF
#!/usr/bin/env bash
if [ "\${1:-}" = "--version" ]; then
  echo "2.1.270 (Claude Code) [context-bench_test.sh fake harness]"
  exit 0
fi
cat <<'JSON'
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Bash","input":{"command":"cat ${ROOT}/CLAUDE.md"}},{"type":"tool_use","id":"2","name":"Read","input":{"file_path":"${ROOT}/ADR.md"}}],"usage":{"input_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":5}}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"1","content":"served payload names ${ROOT}/CLAUDE.md directly, the live checkout, not the export"},{"type":"tool_result","tool_use_id":"2","content":"adr body text"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"The answer is FAKE_GOLD_TOKEN."}],"usage":{"input_tokens":5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":5}}}
{"type":"result","subtype":"success","is_error":false,"num_turns":2,"result":"The answer is FAKE_GOLD_TOKEN.","duration_ms":1234,"total_cost_usd":0.001,"usage":{"input_tokens":15,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10},"modelUsage":{"claude-sonnet-test":{"outputTokens":10}}}
JSON
EOF
chmod +x "${FAKE_BIN}/claude"

export PATH="${FAKE_BIN}:${PATH}"

# ─── run 1: cold cache ──────────────────────────────────────────────────
OUT1="${TMP}/run1.log"
bash "${ROOT}/scripts/context-bench.sh" \
  --tasks "${TASKS_FIXTURE}" \
  --arms fathom \
  --repeat 1 \
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
check_contains "run 1: obligations-layer compile never triggered (no --env compiled-layer)" "${OUT1}" \
  "no served path can name the live checkout"

RUN1_DIR="$(grep -oE 'workspace/artifacts/context-bench/run-[0-9TZ]+' "${OUT1}" | head -1)"
[ -n "${RUN1_DIR}" ] || fail "could not find run 1's RUN_DIR in its log"
SCORED1="${ROOT}/${RUN1_DIR}/results/T-FAKE-fathom-t1.json"
[ -s "${SCORED1}" ] || fail "run 1: scored cell not found at ${SCORED1}"

# 2. the export contains no occurrence of the live checkout's own absolute
# path anywhere in its files.
if grep -rlF "${ROOT}/CLAUDE.md" "${SESSION_ROOT_DIR}" >/dev/null 2>&1 \
  || grep -rlF "${ROOT}" "${SESSION_ROOT_DIR}" --exclude-dir=.git 2>/dev/null | grep -q .; then
  fail "the export at ${SESSION_ROOT_DIR} contains the live checkout's own absolute path — it must only ever contain the committed tree's own content"
fi
ok "the export contains no occurrence of the live checkout's absolute path"

# 4a. path_leak: the served tool_result named the live checkout's path.
python3 - "${SCORED1}" "${ROOT}/CLAUDE.md" <<'PY' || fail "scored cell: path_leak was not set true, or did not name the leaked path"
import json, sys
d = json.load(open(sys.argv[1]))
assert d.get("path_leak") is True, d.get("path_leak")
assert any(sys.argv[2] in p for p in d.get("path_leak_paths", [])), d.get("path_leak_paths")
PY
ok "scored cell: path_leak true, naming the live checkout path actually served"

# 4b. out_of_root_reads: the Bash and Read tool inputs both named an
# absolute path outside SESSION_ROOT.
python3 - "${SCORED1}" "${ROOT}" <<'PY' || fail "scored cell: out_of_root_reads did not count both the Bash and Read escapes"
import json, sys
d = json.load(open(sys.argv[1]))
root = sys.argv[2]
assert d.get("out_of_root_reads", 0) >= 2, d.get("out_of_root_reads")
paths = d.get("out_of_root_paths", [])
assert any(p.startswith("Bash:") and root in p for p in paths), paths
assert any(p.startswith("Read:") and root in p for p in paths), paths
PY
ok "scored cell: out_of_root_reads counts the Bash escape and the Read escape by tool"

# ─── run 2: warm cache (same HEAD) ──────────────────────────────────────
OUT2="${TMP}/run2.log"
bash "${ROOT}/scripts/context-bench.sh" \
  --tasks "${TASKS_FIXTURE}" \
  --arms fathom \
  --repeat 1 \
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

echo "context-bench_test.sh: ${pass} checks passed"
