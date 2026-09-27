#!/usr/bin/env bash
# tenancy-stress.sh — the responsible-tenant stress harness
# (docs/design/responsible-tenant.md §15.2, RT-136; RT-M3, RT-M5, RT-M10).
#
# Committed, NOT in the gate: heavy, run only on a quiet machine with the
# operator's explicit go-ahead.
#
#   bash scripts/tenancy-stress.sh --i-have-the-operators-go-ahead \
#        --store-source <fleet.sqlite> [--budget 1GiB] [--fixture-files 40000] \
#        [--fixture-gib 5] [--cycles 5] [--throttle 60] [--duration 120] \
#        [--binary <path> | build HEAD] [--reading docs/bench/readings/...]
#        [--port N] [--run-root <dir on an external volume>] [--fixture-big 400]
#   bash scripts/tenancy-stress.sh --self-check     # the refusals only (task scripts-check)
#
# What a run does:
#   1. builds the binary under test into workspace/tmp/ (unless --binary);
#   2. copies the store with sqlite3 .backup into a mktemp -d under
#      workspace/tmp/ (the external volume — never the internal disk);
#   3. generates an app-kit-shaped fixture corpus under workspace/tmp/
#      (40k files, 5 GiB including a gitignored cache dir and 400 files
#      over 2 MiB, by default);
#   4. starts an UNMANAGED daemon on a free port — its parent is this
#      harness (--exit-with-parent), `--lifecycle-id tenancy-stress-<run>`
#      so the breaker sees every relaunch in one ledger, a scratch HOME
#      under the run dir, FATHOM_TENANCY_BUDGET forced (1 GiB) with the exit
#      rung enabled;
#   5. drives the load mix (32 concurrent cold list_services, 16
#      obligations on the fixture, 4 references over services ["*"], one
#      ingest) for --duration seconds;
#   6. SIGKILLs and relaunches the daemon --cycles times (a --throttle
#      second launchd-style throttle between relaunches);
#   7. samples /metrics over TCP every second, recording each sample's
#      reachability (the RT-D7 trigger);
#   8. writes a reading with the plain-language verdict first.
#
# It refuses to start when the port, the store or the call log resolve to
# the managed daemon's, or the fixture dir is on the internal disk (the
# device $HOME lives on). --self-check exercises exactly those refusals.
#
# Overrides (the self-check sets them so it never reads operator state):
#   FATHOM_STRESS_MANAGED_PORT      the managed daemon's port (default: its record, else 7575)
#   FATHOM_STRESS_MANAGED_STORE     the managed daemon's store (default: its record)
#   FATHOM_STRESS_MANAGED_CALL_LOG  the managed daemon's call log (default: its record)
#   FATHOM_STRESS_INTERNAL_PROBE    a path on the internal disk (default: $HOME)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"

die() { echo "tenancy-stress: $*" >&2; exit 2; }
refuse() { echo "tenancy-stress: REFUSED: $*" >&2; return 3; }

# realpath that works for a path that does not exist yet (its parent does).
resolve_path() {
  local p="$1"
  if [ -e "$p" ]; then
    (cd "$(dirname "$p")" && echo "$(pwd -P)/$(basename "$p")")
  elif [ -d "$(dirname "$p")" ]; then
    (cd "$(dirname "$p")" && echo "$(pwd -P)/$(basename "$p")")
  else
    echo "$p"
  fi
}

# device id of a path (the nearest existing ancestor).
device_of() {
  local p="$1"
  while [ ! -e "$p" ] && [ "$p" != "/" ]; do p="$(dirname "$p")"; done
  if stat -f %d "$p" >/dev/null 2>&1; then stat -f %d "$p"; else stat -c %d "$p"; fi
}

# managed_facts: the managed daemon's port, store and call log — from the
# overrides, else the managed record under ~/.fathom/daemon/*/record.json
# (read only; never written), else port 7575.
managed_facts() {
  MANAGED_PORT="${FATHOM_STRESS_MANAGED_PORT:-}"
  MANAGED_STORE="${FATHOM_STRESS_MANAGED_STORE:-}"
  MANAGED_CALL_LOG="${FATHOM_STRESS_MANAGED_CALL_LOG:-}"
  if [ -z "$MANAGED_PORT$MANAGED_STORE$MANAGED_CALL_LOG" ]; then
    local rec
    for rec in "${HOME}"/.fathom/daemon/*/record.json; do
      [ -f "$rec" ] || continue
      grep -q '"managed": *true' "$rec" || continue
      MANAGED_PORT="$(sed -n 's/.*"addr": *"[^"]*:\([0-9]*\)".*/\1/p' "$rec" | head -1)"
      MANAGED_STORE="$(sed -n 's/.*"store": *"\([^"]*\)".*/\1/p' "$rec" | head -1)"
      MANAGED_CALL_LOG="$(sed -n 's/.*"call_log": *"\([^"]*\)".*/\1/p' "$rec" | head -1)"
      break
    done
  fi
  MANAGED_PORT="${MANAGED_PORT:-7575}"
}

# preflight PORT STORE CALL_LOG FIXTURE_DIR — the refusals (§15.2).
preflight() {
  local port="$1" store="$2" calllog="$3" fixture="$4"
  managed_facts
  if [ "$port" = "$MANAGED_PORT" ]; then
    refuse "port $port is the managed daemon's port"; return 3
  fi
  if [ -n "$MANAGED_STORE" ] && [ "$(resolve_path "$store")" = "$(resolve_path "$MANAGED_STORE")" ]; then
    refuse "store $store is the managed daemon's store — the harness runs on a .backup copy"; return 3
  fi
  if [ -n "$MANAGED_CALL_LOG" ] && [ "$(resolve_path "$calllog")" = "$(resolve_path "$MANAGED_CALL_LOG")" ]; then
    refuse "call log $calllog is the managed daemon's call log"; return 3
  fi
  local probe="${FATHOM_STRESS_INTERNAL_PROBE:-$HOME}"
  if [ "$(device_of "$fixture")" = "$(device_of "$probe")" ]; then
    refuse "fixture dir $fixture is on the internal disk (the device $probe lives on) — put the run under an external volume"; return 3
  fi
  return 0
}

# ledger_start_of LEDGER PATTERN — the start number (1-based count of
# start rows so far) at the first ledger line matching PATTERN; "" when
# none matches.
ledger_start_of() {
  local ledger="$1" pattern="$2"
  [ -f "$ledger" ] || return 0
  # A breaker row is written by the start it decides, before that start's
  # own row: a match on a non-start row belongs to the next start.
  awk -v pat="$pattern" '/"row":"start"/ {n++} $0 ~ pat {print (/"row":"start"/ ? n : n+1); exit}' "$ledger"
}

# stress_verdict MAX_FP EXIT_MARK DIAG SAFE_START STOP_START — RT-M3's
# seatbelt contract (docs/plans/responsible-tenant.md): max own footprint
# ≤ exit mark + 1 GiB, a diag capture, safe mode by start 3, the breaker's
# stop by start 5. Prints PASS or FAIL and, for FAIL, each criterion missed.
stress_verdict() {
  local max_fp="$1" exit_mark="$2" diag="$3" safe_start="$4" stop_start="$5" why=()
  if [ -z "$exit_mark" ]; then
    why+=("the exit mark is unknown")
  elif [ "$max_fp" -gt $(( exit_mark + 1073741824 )) ]; then
    why+=("max footprint ${max_fp} B is over the exit mark + 1 GiB")
  fi
  [ -n "$diag" ] || why+=("no diag capture")
  if [ -z "$safe_start" ]; then
    why+=("no safe mode")
  elif [ "$safe_start" -gt 3 ]; then
    why+=("safe mode first at start ${safe_start}, not by start 3")
  fi
  if [ -z "$stop_start" ]; then
    why+=("the breaker never stopped it")
  elif [ "$stop_start" -gt 5 ]; then
    why+=("stopped at start ${stop_start}, not by start 5")
  fi
  if [ "${#why[@]}" -eq 0 ]; then
    echo "PASS"
  else
    local IFS=';'
    echo "FAIL (${why[*]})"
  fi
}

verdict_self_check() {
  local dir="$1" ok=0 got
  local l="${dir}/lifecycle.jsonl"
  check() {
    local label="$1" want="$2"; shift 2
    got="$(stress_verdict "$@")"
    if [ "${got%% *}" = "$want" ]; then
      echo "self-check: verdict: ${label} reads ${want}"
    else
      echo "self-check: FAILED — verdict: ${label} read ${got}" >&2; ok=1
    fi
  }
  printf '%s\n' '{"row":"start","mode":"normal"}' '{"row":"exit"}' '{"row":"start","mode":"normal"}' '{"row":"exit"}' \
    '{"row":"start","mode":"safe"}' '{"row":"exit"}' '{"row":"start","mode":"safe"}' '{"row":"breaker","to":"stop"}' >"$l"
  local safe stop
  safe="$(ledger_start_of "$l" '"mode":"safe"|"to":"safe"')"
  stop="$(ledger_start_of "$l" '"to":"stop"|"row":"stop"')"
  check "the seatbelt held (safe mode by start 3, stop by start 5)" PASS 100 4294967296 /diag "$safe" "$stop"
  check "no safe mode" FAIL 100 4294967296 /diag "" "$stop"
  printf '%s\n' '{"row":"start"}' '{"row":"start"}' '{"row":"start"}' '{"row":"start","mode":"safe"}' '{"row":"stop"}' >"$l"
  check "safe mode first at start $(ledger_start_of "$l" '"mode":"safe"')" FAIL 100 4294967296 /diag "$(ledger_start_of "$l" '"mode":"safe"')" "$(ledger_start_of "$l" '"row":"stop"')"
  # The ledger as a daemon writes it: the breaker row precedes the start it
  # decides (5 SIGKILL cycles: safe at start 4, stop at start 6).
  printf '%s\n' '{"row":"start"}' '{"row":"exit"}' '{"row":"start"}' '{"row":"exit"}' '{"row":"start"}' '{"row":"exit"}' \
    '{"row":"breaker","to":"safe"}' '{"row":"start","mode":"safe"}' '{"row":"exit"}' '{"row":"breaker","to":"safe"}' \
    '{"row":"start","mode":"safe"}' '{"row":"exit"}' '{"row":"breaker","to":"stop"}' >"$l"
  safe="$(ledger_start_of "$l" '"mode":"safe"|"to":"safe"')"
  stop="$(ledger_start_of "$l" '"to":"stop"|"row":"stop"')"
  if [ "$safe" = 4 ] && [ "$stop" = 6 ]; then
    echo "self-check: ledger: a breaker row is attributed to the start it decides (safe 4, stop 6)"
  else
    echo "self-check: FAILED — ledger attribution read safe ${safe}, stop ${stop} (want 4, 6)" >&2; ok=1
  fi
  check "no stop" FAIL 100 4294967296 /diag 3 ""
  check "no diag capture" FAIL 100 4294967296 "" 3 5
  return "$ok"
}

self_check() {
  local scratch
  scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-stress-selfcheck.XXXXXX")"
  trap 'rm -rf "$scratch"' RETURN
  export FATHOM_STRESS_MANAGED_PORT=7575
  export FATHOM_STRESS_MANAGED_STORE="${scratch}/managed/fleet.sqlite"
  export FATHOM_STRESS_MANAGED_CALL_LOG="${scratch}/managed/mcp-calls.jsonl"
  mkdir -p "${scratch}/managed" "${scratch}/run"
  : >"$FATHOM_STRESS_MANAGED_STORE"
  : >"$FATHOM_STRESS_MANAGED_CALL_LOG"
  local ok=0 rc
  expect_refused() {
    local label="$1"; shift
    set +e; preflight "$@" 2>"${scratch}/err"; rc=$?; set -e
    if [ "$rc" -eq 3 ] && grep -q "REFUSED" "${scratch}/err"; then
      echo "self-check: refused ${label}: $(sed 's/^tenancy-stress: REFUSED: //' "${scratch}/err")"
    else
      echo "self-check: FAILED — ${label} was not refused (rc=$rc)" >&2; ok=1
    fi
  }
  # The internal disk is the device the fixture's own tree is on …
  export FATHOM_STRESS_INTERNAL_PROBE="$scratch"
  expect_refused "the managed port" 7575 "${scratch}/run/store.sqlite" "${scratch}/run/calls.jsonl" "/dev/null"
  # The store through another spelling of the same path.
  expect_refused "the managed store" 18501 "${scratch}/managed/../managed/fleet.sqlite" "${scratch}/run/calls.jsonl" "/dev/null"
  expect_refused "the managed call log" 18501 "${scratch}/run/store.sqlite" "${scratch}/managed/mcp-calls.jsonl" "/dev/null"
  expect_refused "a fixture on the internal disk" 18501 "${scratch}/run/store.sqlite" "${scratch}/run/calls.jsonl" "${scratch}/run/fixture"
  # … and a run that names none of them starts: /dev is its own device.
  export FATHOM_STRESS_INTERNAL_PROBE=/dev
  set +e; preflight 18501 "${scratch}/run/store.sqlite" "${scratch}/run/calls.jsonl" "${scratch}/run/fixture" 2>"${scratch}/err"; rc=$?; set -e
  if [ "$rc" -ne 0 ]; then
    echo "self-check: FAILED — a scratch run was refused: $(cat "${scratch}/err")" >&2; ok=1
  else
    echo "self-check: a scratch port, store copy, call log and external fixture pass"
  fi
  # A real run without the operator's go-ahead is refused before anything starts.
  set +e; FATHOM_STRESS_SELF_CHECK_CHILD=1 bash "${BASH_SOURCE[0]}" --store-source "${scratch}/run/store.sqlite" >/dev/null 2>"${scratch}/err"; rc=$?; set -e
  if [ "$rc" -eq 2 ] && grep -q "operator's go-ahead" "${scratch}/err"; then
    echo "self-check: a run without --i-have-the-operators-go-ahead is refused"
  else
    echo "self-check: FAILED — a run without the go-ahead was not refused (rc=$rc)" >&2; ok=1
  fi
  verdict_self_check "$scratch" || ok=1
  return "$ok"
}

free_port() {
  local p
  for p in $(seq 18500 18599); do
    if ! (exec 3<>"/dev/tcp/127.0.0.1/$p") 2>/dev/null; then
      echo "$p"; return 0
    fi
  done
  return 1
}

# gen_fixture DIR FILES GIB — an app-kit-shaped corpus: source files in
# nested packages, a gitignored cache dir holding most of the bytes, and
# 400 files over 2 MiB.
gen_fixture() {
  local dir="$1" files="$2" gib="$3" big="${4:-400}"
  mkdir -p "$dir/src" "$dir/.cache"
  (cd "$dir" && git init -q . && printf '.cache/\n' >.gitignore)
  local i pkg
  for i in $(seq 1 "$files"); do
    pkg=$(( i % 400 ))
    mkdir -p "$dir/src/pkg$pkg"
    printf 'package pkg%d\n\n// F%d is generated.\nfunc F%d() int { return %d }\n' "$pkg" "$i" "$i" "$i" >"$dir/src/pkg$pkg/f$i.go"
  done
  for i in $(seq 1 "$big"); do
    dd if=/dev/zero of="$dir/src/pkg$(( i % 400 ))/big$i.bin" bs=1048576 count=3 2>/dev/null
  done
  local cache_mib=$(( gib * 1024 - big * 3 ))
  [ "$cache_mib" -gt 0 ] || cache_mib=1
  local chunk
  for chunk in $(seq 1 $(( (cache_mib + 255) / 256 ))); do
    dd if=/dev/zero of="$dir/.cache/blob$chunk" bs=1048576 count=256 2>/dev/null
  done
  (cd "$dir" && git add -A src .gitignore >/dev/null 2>&1 && git -c user.email=stress@local -c user.name=stress commit -qm fixture) || true
}

rpc() { # rpc PORT NAME ARGS_JSON
  curl -s -m 30 -o /dev/null -w '%{http_code}\n' -H 'Content-Type: application/json' -H 'X-Fathom-Origin: agent' \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"$2\",\"arguments\":$3}}" \
    "http://127.0.0.1:$1/rpc" || echo 000
}

main() {
  local go_ahead=0 store_source="" budget="1GiB" files=40000 gib=5 big=400 cycles=5 throttle=60 duration=120 binary="" reading="" port="" run_root=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --self-check) self_check; exit $? ;;
      --i-have-the-operators-go-ahead) go_ahead=1 ;;
      --store-source) store_source="$2"; shift ;;
      --budget) budget="$2"; shift ;;
      --fixture-files) files="$2"; shift ;;
      --fixture-gib) gib="$2"; shift ;;
      --fixture-big) big="$2"; shift ;;
      --cycles) cycles="$2"; shift ;;
      --throttle) throttle="$2"; shift ;;
      --duration) duration="$2"; shift ;;
      --binary) binary="$2"; shift ;;
      --reading) reading="$2"; shift ;;
      --port) port="$2"; shift ;;
      --run-root) run_root="$2"; shift ;;
      -h|--help) sed -n '2,45p' "${BASH_SOURCE[0]}"; exit 0 ;;
      *) die "unknown argument $1" ;;
    esac
    shift
  done
  [ "$go_ahead" -eq 1 ] || die "every run needs the operator's go-ahead on a quiet machine: pass --i-have-the-operators-go-ahead (or --self-check for the refusals only)"
  [ -n "$store_source" ] || die "--store-source <fleet.sqlite> is required (it is copied with sqlite3 .backup, never opened in place)"
  command -v sqlite3 >/dev/null || die "sqlite3 is required"

  local run_id; run_id="$(date -u +%Y%m%dT%H%M%SZ)-$$"
  [ -n "$run_root" ] || run_root="${REPO_ROOT}/workspace/tmp"
  mkdir -p "$run_root"
  local run_dir; run_dir="$(mktemp -d "${run_root}/tenancy-stress.XXXXXX")"
  local store="${run_dir}/fleet.sqlite" calllog="${run_dir}/mcp-calls.jsonl" fixture="${run_dir}/fixture"
  if [ -z "$port" ]; then
    port="$(free_port)" || die "no free port in 18500-18599"
  fi
  preflight "$port" "$store" "$calllog" "$fixture" || { rmdir "$run_dir" 2>/dev/null; exit 3; }
  if [ "$(resolve_path "$store_source")" = "$(resolve_path "${MANAGED_STORE:-/nonexistent}")" ]; then
    echo "tenancy-stress: copying the managed store with sqlite3 .backup (read-only for it)" >&2
  fi
  local lifecycle_id="tenancy-stress-${run_id}"
  local home="${run_dir}/home"; mkdir -p "$home"
  echo "tenancy-stress: run ${run_id} in ${run_dir} on port ${port}, budget ${budget}, lifecycle id ${lifecycle_id}"

  if [ -z "$binary" ]; then
    binary="${REPO_ROOT}/workspace/tmp/fathom-bin-stress-$$"
    (cd "$REPO_ROOT" && GOWORK=off go build -o "$binary" ./cmd/fathom) || die "build failed"
  fi
  sqlite3 "$store_source" ".backup '${store}'" || die "sqlite3 .backup of ${store_source} failed"
  echo "tenancy-stress: generating the fixture (${files} files, ${gib} GiB)"
  gen_fixture "$fixture" "$files" "$gib" "$big"

  local daemon_pid=""
  start_daemon() {
    # The exit rung is on unless FATHOM_TENANCY_EXIT=off; never set it here.
    HOME="$home" FATHOM_TENANCY_BUDGET="$budget" \
      "$binary" serve-http --addr "127.0.0.1:${port}" --store "$store" --call-log "$calllog" \
      --lifecycle-id "$lifecycle_id" --exit-with-parent --idle-exit 0 --ttl 2h >>"${run_dir}/daemon.log" 2>&1 &
    daemon_pid=$!
    local i
    for i in $(seq 1 100); do
      # -f: /healthz answers 503 while the daemon waits for its store
      # (storewait.go); load sent then is refused daemon-wide, not measured.
      curl -sf -m 1 "http://127.0.0.1:${port}/healthz" >/dev/null 2>&1 && return 0
      kill -0 "$daemon_pid" 2>/dev/null || return 1
      sleep 0.2
    done
    return 1
  }
  stop_all() {
    [ -n "${sampler_pid:-}" ] && kill "$sampler_pid" 2>/dev/null || true
    [ -n "$daemon_pid" ] && kill -9 "$daemon_pid" 2>/dev/null || true
  }
  trap 'stop_all; exit 130' INT TERM
  trap stop_all EXIT

  # /metrics every second: reachability and footprint.
  local samples="${run_dir}/metrics.tsv"
  local harness_pid=$$
  ( while kill -0 "$harness_pid" 2>/dev/null; do
      local ts body code fp
      ts="$(date -u +%s)"
      body="$(curl -s -m 1 -w '\n%{http_code}' "http://127.0.0.1:${port}/metrics" 2>/dev/null || true)"
      code="$(printf '%s' "$body" | tail -1)"
      fp="$(printf '%s' "$body" | sed -n 's/^fathom_footprint_bytes \([0-9]*\)$/\1/p')"
      printf '%s\t%s\t%s\n' "$ts" "${code:-000}" "${fp:-}" >>"$samples"
      sleep 1
    done ) &
  local sampler_pid=$!

  start_daemon || echo "tenancy-stress: the first start did not answer /healthz" >&2
  local fixture_file; fixture_file="$(cd "$fixture" && ls src/pkg1/*.go | head -1)"
  local end=$(( $(date +%s) + duration )) round=0
  # Wait on the load's own jobs only — a bare `wait` would wait on the
  # sampler, which never ends.
  local pids=() ingest_pid
  rpc "$port" ingest "{\"service\":\"stress-fixture\",\"path\":\"${fixture}\"}" >>"${run_dir}/codes.txt" &
  ingest_pid=$!
  while [ "$(date +%s)" -lt "$end" ]; do
    round=$((round + 1))
    local j
    pids=()
    for j in $(seq 1 32); do rpc "$port" list_services '{}' >>"${run_dir}/codes.txt" & pids+=($!); done
    for j in $(seq 1 16); do rpc "$port" obligations "{\"service\":\"stress-fixture\",\"file\":\"${fixture_file}\"}" >>"${run_dir}/codes.txt" & pids+=($!); done
    for j in $(seq 1 4); do rpc "$port" references '{"query":"F1","services":["*"]}' >>"${run_dir}/codes.txt" & pids+=($!); done
    wait "${pids[@]}" 2>/dev/null || true
  done
  wait "$ingest_pid" 2>/dev/null || true

  # SIGKILL / relaunch cycles, a launchd-style throttle between them.
  local c starts=1
  for c in $(seq 1 "$cycles"); do
    kill -9 "$daemon_pid" 2>/dev/null || true
    wait "$daemon_pid" 2>/dev/null || true
    sleep "$throttle"
    start_daemon || true
    starts=$((starts + 1))
    pids=()
    for j in $(seq 1 8); do rpc "$port" list_services '{}' >>"${run_dir}/codes.txt" & pids+=($!); done
    wait "${pids[@]}" 2>/dev/null || true
  done
  stop_all
  trap - EXIT INT TERM

  # The reading.
  local ledger="${home}/.fathom/daemon/${lifecycle_id}/lifecycle.jsonl"
  local max_fp reach total diag safe_at stop_at
  max_fp="$(awk -F'\t' '$3!="" && $3>m {m=$3} END {print m+0}' "$samples")"
  total="$(wc -l <"$samples" | tr -d ' ')"
  reach="$(awk -F'\t' '$2=="200"' "$samples" | wc -l | tr -d ' ')"
  diag="$(ls -d "${home}/.fathom/daemon/${lifecycle_id}/diag/"* 2>/dev/null | head -1 || true)"
  # The start number (not the ledger line) each first appeared at.
  safe_at="$(ledger_start_of "$ledger" '"mode":"safe"|"to":"safe"')"
  stop_at="$(ledger_start_of "$ledger" '"to":"stop"|"row":"stop"')"
  local refused ok503
  refused="$(grep -c '^503$' "${run_dir}/codes.txt" 2>/dev/null || true)"
  ok503="$(grep -c '^200$' "${run_dir}/codes.txt" 2>/dev/null || true)"
  local exit_mark_bytes verdict_line verdict
  # --url: without it resources falls back to the managed daemon's address
  # and would report the operator's daemon, not this run's. `|| true`:
  # resources exits 1 whenever its verdict is not "within budget" (a
  # crash-looping ledger is exactly what the cycles produce), and under
  # pipefail that would end the harness before the reading is written.
  local res_json
  res_json="$(HOME="$home" FATHOM_TENANCY_BUDGET="$budget" "$binary" resources --id "$lifecycle_id" --url "http://127.0.0.1:${port}" --json 2>/dev/null || true)"
  exit_mark_bytes="$(printf '%s\n' "$res_json" | sed -n 's/.*"exit_mark": *\([0-9]*\).*/\1/p' | head -1 || true)"
  local budget_bytes over_budget_exits tenant_line
  budget_bytes="$(printf '%s\n' "$res_json" | sed -n 's/.*"budget": *\([0-9]*\).*/\1/p' | head -1 || true)"
  over_budget_exits="$(grep -c '"reason":"over_budget"' "$ledger" 2>/dev/null || true)"
  # RT-M3 on a post-wave-D binary: never exits over budget, stays ≤ budget.
  if [ -n "$budget_bytes" ] && [ "${over_budget_exits:-0}" -eq 0 ] && [ "$max_fp" -le "$budget_bytes" ]; then
    tenant_line="PASS (no over-budget exit; max own footprint ${max_fp} B ≤ budget ${budget_bytes} B)"
  else
    tenant_line="FAIL (over-budget exits ${over_budget_exits:-?}; max own footprint ${max_fp} B; budget ${budget_bytes:-unknown} B)"
  fi
  verdict_line="$(stress_verdict "$max_fp" "$exit_mark_bytes" "$diag" "$safe_at" "$stop_at")"
  verdict="${verdict_line%% *}"
  local diag_word="does not exist" safe_word="did not appear" stop_word="did not stop it"
  [ -z "$diag" ] || diag_word="exists"
  [ -z "$safe_at" ] || safe_word="appeared at start ${safe_at}"
  [ -z "$stop_at" ] || stop_word="stopped it at start ${stop_at}"
  [ -n "$reading" ] || reading="${REPO_ROOT}/docs/bench/readings/tenancy-stress-$(date -u +%Y-%m-%d).md"
  {
    echo "# Tenancy stress reading — $(date -u +%Y-%m-%d)"
    echo
    echo "**Verdict: ${verdict_line}.** The daemon under a ${budget} budget peaked at $(( max_fp / 1048576 )) MiB of its own footprint (exit mark $(( ${exit_mark_bytes:-0} / 1048576 )) MiB), /metrics answered ${reach} of ${total} one-second samples, ${refused} calls were refused with 503 and ${ok503} answered 200; a diagnostic capture ${diag_word}; safe mode ${safe_word}; the breaker ${stop_word} over ${starts} starts (RT-M3: safe mode by start 3, stop by start 5)."
    echo
    echo "**Tenant (post-wave-D RT-M3: never exits over budget, stays ≤ budget): ${tenant_line}.** The seatbelt verdict above expects over-budget exits; SIGKILL cycles alone reach safe mode on the start after the third counted exit."
    echo
    echo "| Measure | Value | Instrument |"
    echo "|---|---|---|"
    echo "| run | ${run_id} | harness |"
    echo "| binary | ${binary} | harness |"
    echo "| budget / exit mark | ${budget} / ${exit_mark_bytes:-unknown} B | fathom resources --json |"
    echo "| max own footprint | ${max_fp} B | /metrics fathom_footprint_bytes, 1 s |"
    echo "| /metrics reachability | ${reach}/${total} | curl, 1 s (RT-D7 trigger when < total) |"
    echo "| calls 200 / 503 | ${ok503} / ${refused} | curl status codes |"
    echo "| starts | ${starts} (${cycles} SIGKILL cycles, ${throttle} s throttle) | harness |"
    echo "| start of the first safe-mode row | ${safe_at:-none} | ${ledger} |"
    echo "| start of the first breaker/stop row | ${stop_at:-none} | ${ledger} |"
    echo "| diag capture | ${diag:-none} | ~/.fathom/daemon/<id>/diag (scratch HOME) |"
    echo "| fixture | ${files} files, ${gib} GiB, ${big} over 2 MiB | gen_fixture |"
  } >"$reading"
  echo "tenancy-stress: reading → ${reading} (verdict ${verdict}); run dir ${run_dir} (delete it when read)"
  [ "$verdict" = "PASS" ]
}

main "$@"
