#!/usr/bin/env bash
# One isolated cold arm. See README.md for evidence semantics and prerequisites.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
EVIDENCE="$SCRIPT_DIR/cold_arm.py"
die() { echo "cold-arm: $*" >&2; exit 2; }
subject='' service='' plans='' ids='' budget='' out='' binary=fathom
while [ "$#" -gt 0 ]; do
  case "$1" in
    --subject|--service|--plans|--ids|--budget|--out|--fathom)
      [ "$#" -ge 2 ] || die "missing value for $1"
      case "$1" in
        --subject) subject="$2" ;; --service) service="$2" ;;
        --plans) plans="$2" ;; --ids) ids="$2" ;; --budget) budget="$2" ;;
        --out) out="$2" ;; --fathom) binary="$2" ;;
      esac
      shift 2 ;;
    *) die "usage: $0 --subject REPO --service NAME --plans FILE --ids FILE --budget BYTES --out NEW_DIR [--fathom BINARY]" ;;
  esac
done
[ -n "$subject" ] && [ -n "$service" ] && [ -n "$plans" ] && [ -n "$ids" ] && [ -n "$out" ] || die 'all required options must be supplied'
case "$budget" in ''|*[!0-9]*|0) die '--budget must be positive integer bytes' ;; esac
for tool in python3 curl git go; do command -v "$tool" >/dev/null || die "missing $tool"; done
binary="$(command -v "$binary")" || die 'fathom binary not found'
binary="$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$binary")"
subject="$(cd "$subject" && pwd -P)"
# Clear inherited operator paths, endpoint and tenancy overrides. Only the
# new home/config and explicit endpoint below govern this arm.
while IFS= read -r variable; do unset "$variable"; done < <(compgen -e | LC_ALL=C sort | sed -n '/^FATHOM_/p')
python3 "$EVIDENCE" prepare "$subject" "$service" "$plans" "$ids" "$budget" "$out" "$binary"
out="$(cd "$out" && pwd -P)"
export FATHOM_HOME="$out/home"
store="$out/store.sqlite"
daemon_pid='' sampler_pid='' url='' stopped=false
stop() {
  local code=0
  [ "$stopped" = false ] || return 0
  stopped=true
  if [ -n "$daemon_pid" ]; then
    kill -TERM "$daemon_pid" 2>/dev/null || true
    # serve-http has its own bounded drain. The watcher is only a fallback
    # for a wedged build, recorded as a forced stop rather than a clean exit.
    python3 "$EVIDENCE" watchdog "$out" "$daemon_pid" &
    local watcher=$!
    wait "$daemon_pid" || code=$?
    kill "$watcher" 2>/dev/null || true
    wait "$watcher" 2>/dev/null || true
  fi
  if [ -n "$sampler_pid" ]; then
    touch "$out/stop-sampling"
    wait "$sampler_pid" || true
    if [ -n "$url" ]; then python3 "$EVIDENCE" sample-once "$out" "$binary" "$url" || true; fi
  fi
  printf '%s\n' "$code" > "$out/daemon-exit.txt"
}
finish() {
  local code=$?
  trap - EXIT
  stop
  local receipt_code=0
  python3 "$EVIDENCE" finish "$out" "$code" || receipt_code=$?
  if [ "$code" -eq 0 ]; then code="$receipt_code"; fi
  exit "$code"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
# Port 0 is supported by serve-http's net.Listener. Its private record gives
# the actual address, rather than the startup log's requested :0 address.
serve=("$binary" serve-http --addr 127.0.0.1:0 --store "$store" --call-log "$out/calls.jsonl" --auto-refresh=false)
printf '%s\n' "${serve[@]}" > "$out/daemon-argv.txt"
(cd "$out" && exec "${serve[@]}") > "$out/daemon.stdout" 2> "$out/daemon.stderr" &
daemon_pid=$!
printf '%s\n' "$daemon_pid" > "$out/daemon-pid.txt"
python3 "$EVIDENCE" sample "$out" "$binary" &
sampler_pid=$!
printf '%s\n' "$sampler_pid" > "$out/sampler-pid.txt"
url="$(python3 "$EVIDENCE" ready "$out" "$daemon_pid")"
export FATHOM_HTTP="$url"
run() { python3 "$EVIDENCE" run "$out" "$@"; }
rpc() {
  local name="$1" surface="$2"
  run "$name" rpc "$out/payloads/$name.json" curl --noproxy '*' --silent --show-error \
    --fail-with-body --connect-timeout 2 --max-time 900 --header 'Content-Type: application/json' \
    --data-binary "@$out/payloads/$name.json" "$url/rpc/$surface"
}
# Cold ingest is a setup call on the extended surface, inside the daemon's
# governor. Poll only the returned run id if the daemon answers asynchronously.
rpc ingest extended
for ((poll=0; poll<900; poll++)); do
  state="$(python3 "$EVIDENCE" ingest-state "$out")"
  case "$state" in
    done|failed|unknown) break ;;
    *) sleep 1; python3 "$EVIDENCE" poll "$out" "$state"; rpc ingest-poll extended ;;
  esac
done
printf '%s\n' "$state" > "$out/ingest-state.txt"
(cd "$subject"
  while IFS= read -r plan; do
    run "verify:$plan" json - "$binary" verify --strict --service "$service" --url "$url" --plan "$plan" --json
  done < "$out/plans.txt"
  symbol="$(python3 "$EVIDENCE" select "$out" symbol)"
  context_plan="$(python3 "$EVIDENCE" select "$out" context_plan)"
  context_id="$(python3 "$EVIDENCE" select "$out" context_id)"
  run cli:symbol json - "$binary" export receipt --store "$store" --service "$service" --symbol "$symbol"
  run cli:context text - "$binary" plans context "$context_plan" --row "$context_id" --service "$service" --url "$url"
  plan_args=()
  while IFS= read -r plan; do plan_args+=(--plan "$plan"); done < "$out/plans.txt"
  run cli:plans json - "$binary" plans status --service "$service" --url "$url" --json "${plan_args[@]}"
  run cli:obligations json - "$binary" obligations --service "$service" --url "$url" --json
)
rpc initialize agent
rpc tools-list agent
for name in symbol_context review_diff obligations verify_plan; do rpc "$name" agent; done
# The final, sequential resources reading in stop also retains the ring peak.
stop
