#!/usr/bin/env bash
#
# A2/A3 (docs/plans/activation.md, docs/vision/11-dial-to-eleven.md §3 Phase
# A): the flag-day recompile. Fires every eval-passed semantic-layer pack
# with an applicable corpus, per fleet repo — obligations fleet-wide (OP-6)
# plus the first-ever production compiles of adr-rules and spec-bindings on
# fathom. 11 service·packs total: obligations x 9 fleet services +
# {adr-rules, spec-bindings} x fathom.
#
# Why direct --store mode, not the HTTP daemon: `fathom compile` writes
# compile_runs + semantic_claims, which needs the same single-writer lock
# respect as `fathom ingest` — this script stops/starts the daemon around
# the compile loop, exactly like scripts/reingest-fleet.sh (the fleet
# service list precedent this script reuses verbatim). Direct mode also puts
# every compile under the A2-1 idempotency guard and the D3-1 eval-gate
# (cmd/fathom compile.go) automatically: a same-watermark/same-prompt rerun
# of this script refuses instead of double-writing superseded claims (the
# exact failure mode A2-1 exists to prevent), and an un-evaled layer refuses
# instead of silently shipping ungated claims.
#
# Cost: ~5c per service*pack on a metered API lane (<=$0.60 total), $0
# marginal on the claude-cli subscription lane (the default here — the same
# lane the eval gates themselves ran on, so metrics transfer). Override with
# FATHOM_LLM_PROVIDER=anthropic (needs ANTHROPIC_API_KEY) for the metered
# lane instead.
#
# Per-pack failure isolation: one service*pack's compile failing (bad
# corpus, API hiccup, budget refusal) does not abort the run — every other
# service*pack still gets attempted, and failures are summarized at the end.
#
# Run this in YOUR terminal — it stops/starts the launchd daemon via
# launchctl, which must run in your login session (it wedges when issued
# from a sandboxed tool, same caveat as reingest-fleet.sh).
#
#   bash scripts/recompile-fleet.sh
#
set -euo pipefail

FATHOM_REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$FATHOM_REPO/bin/fathom"
STORE="$FATHOM_REPO/workspace/db/fleet.sqlite"
STAMP="$FATHOM_REPO/workspace/logs/recompile-stamp.txt"
LABEL="com.geoffmilleraz.fathom-http"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
DOMAIN="gui/$(id -u)"

# claude-cli lane by default (subscription, $0 marginal, the lane the eval
# gates already ran on) — override by exporting FATHOM_LLM_PROVIDER before
# invoking this script.
export FATHOM_LLM_PROVIDER="${FATHOM_LLM_PROVIDER:-claude-cli}"

# Fleet services — same list as scripts/reingest-fleet.sh (obligations
# compiles fleet-wide, OP-6).
SERVICES=(fathom agentic-kit app-kit throughline geoffmiller-cloud resume_as_code growing-gardens-bi car-assistant crossplane-platform)

echo "==> fathom flag-day recompile  (store: $STORE, lane: $FATHOM_LLM_PROVIDER)"
[ -x "$BIN" ] || { echo "ERROR: build the binary first:  (cd $FATHOM_REPO && go build -o bin/fathom ./cmd/fathom)"; exit 1; }
[ -f "$STORE" ] || { echo "ERROR: no fleet store at $STORE — run scripts/reingest-fleet.sh first."; exit 1; }

echo "==> stopping the daemon (releases the single-writer store lock)"
launchctl bootout "$DOMAIN/$LABEL" 2>/dev/null || true
for _ in $(seq 1 15); do pgrep -f "bin/fathom serve-http" >/dev/null || break; sleep 1; done
if pgrep -f "bin/fathom serve-http" >/dev/null; then
  echo "ERROR: a serve-http process is still holding the lock; reap it and retry."; exit 1
fi

mkdir -p "$(dirname "$STAMP")"

declare -a OK_PACKS=()
declare -a FAILED_PACKS=()

compile_one() {
  local svc="$1" layer="$2" label="$svc/$layer"
  echo "==> compiling $label"
  if out="$("$BIN" compile --service "$svc" --layer "$layer" --store "$STORE" --dry-run=false 2>&1)"; then
    echo "    - $label OK"
    echo "$out" | sed 's/^/        /'
    OK_PACKS+=("$label")
  else
    echo "    - $label FAILED (isolated — continuing with the rest):"
    echo "$out" | tail -5 | sed 's/^/        /'
    FAILED_PACKS+=("$label")
  fi
}

echo "==> obligations x ${#SERVICES[@]} fleet services"
for svc in "${SERVICES[@]}"; do
  compile_one "$svc" obligations
done

echo "==> adr-rules + spec-bindings x fathom (first-ever production compiles)"
compile_one fathom adr-rules
compile_one fathom spec-bindings

echo "==> restarting the daemon"
launchctl bootstrap "$DOMAIN" "$PLIST" 2>/dev/null || launchctl kickstart -k "$DOMAIN/$LABEL" 2>/dev/null || true

{
  echo "recompile-fleet.sh run at $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "lane: $FATHOM_LLM_PROVIDER"
  echo "store: $STORE"
  echo "ok (${#OK_PACKS[@]}): ${OK_PACKS[*]:-none}"
  echo "failed (${#FAILED_PACKS[@]}): ${FAILED_PACKS[*]:-none}"
} > "$STAMP"
echo "==> stamped: $STAMP"

if [ "${#FAILED_PACKS[@]}" -gt 0 ]; then
  echo "==> done with failures: ${FAILED_PACKS[*]} — see the stamp and output above."
  exit 1
fi
echo "==> done. ${#OK_PACKS[@]}/${#OK_PACKS[@]} service*packs compiled clean."
