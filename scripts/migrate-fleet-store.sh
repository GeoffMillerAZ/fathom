#!/usr/bin/env bash
#
# ⚠️ HISTORICAL — completed 2026-07-23; retained for reference only.
#
# This ran once, successfully (see workspace/logs/store-migration-stamp.txt
# on the fleet checkout). SS-8 (docs/plans/store-sqlite.md; ADR-025) then
# deleted the `migrate-store` CLI command and its DuckDB read path outright,
# since the trigger for removal ("operator runs migrate-store on the fleet
# DB and the daemon runs a full day green on sqlite") had fired. Step 4/8
# below (`$BIN migrate-store ...`) can no longer succeed against any binary
# built from this repo post-SS-8 — a re-run needs a pre-SS-8 build. Kept as
# the executable record of exactly what the migration did.
#
# One-shot ADR-020 fleet-store migration: DuckDB -> SQLite, plus the daemon
# binary move to ~/.local/libexec (decouples the running daemon from
# post-commit rebuilds of bin/fathom, so a crash can never silently respawn
# an unvetted binary with a fresh TCC code-hash).
#
# Run this from YOUR terminal, not an agent tool — it stops/starts the
# launchd daemon (wedges when issued from a sandboxed tool), and the restart
# may raise ONE macOS permission dialog you must be present to answer:
#
#   ┌─────────────────────────────────────────────────────────────────┐
#   │  The rebuilt binary has a new code hash, so the old TCC grant   │
#   │  for removable-volume access no longer matches. If the daemon   │
#   │  start hangs, LOOK FOR the macOS dialog asking to allow access  │
#   │  to files on a removable volume and click ALLOW.                │
#   └─────────────────────────────────────────────────────────────────┘
#
#   bash scripts/migrate-fleet-store.sh
#
# What it does, in order (each step gated on the previous):
#   1. preflight   — binary at repo HEAD, source store present
#   2. stop        — bootout the daemon (releases the DuckDB write lock)
#   3. quarantine  — move aside any existing fleet.sqlite (the pre-migration
#                    file at this name is DuckDB bytes written by the stale
#                    binary's post-commit hook — not a real SQLite store)
#   4. migrate     — port the eight non-derivable tables, hash-verified
#   5. re-ingest   — rebuild derived tables for every fleet service (CLI,
#                    single-writer; epoch "5" makes this a FULL re-extract,
#                    expect ~1min total, one time only)
#   6. install     — copy bin/fathom -> ~/.local/libexec/fathom/fathom
#   7. repoint     — rewrite the launchd plist (libexec binary, .sqlite
#                    store, PATH incl. go for typed-edge enrichment)
#   8. restart     — bootstrap + health check + index verification
#
# The old fleet.duckdb is NOT deleted — it is the rollback path. Delete it
# (and its .wal/.bak siblings) after a day green; until then every
# store-directory scan warns about the same-stem collision by design.
#
# Rollback: restore the previous plist from the .bak this script writes,
# `launchctl bootout` + `bootstrap` again, and delete fleet.sqlite*.
set -euo pipefail

# Associative arrays need bash >= 4 (macOS /bin/bash is 3.2 — invoke as
# `bash scripts/migrate-fleet-store.sh` so PATH resolves a modern bash).
[ "${BASH_VERSINFO[0]}" -ge 4 ] || { echo "ERROR: bash >= 4 required; run: bash $0"; exit 1; }

FATHOM_REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$FATHOM_REPO/bin/fathom"
DB="$FATHOM_REPO/workspace/db"
SRC="$DB/fleet.duckdb"
DST="$DB/fleet.sqlite"
FLEET_BASE="$(dirname "$FATHOM_REPO")"
LABEL="com.geoffmilleraz.fathom-http"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
DOMAIN="gui/$(id -u)"
INSTALL_DIR="$HOME/.local/libexec/fathom"
GO_BIN_DIR="$(dirname "$(command -v go || echo /usr/local/bin/go)")"
TS="$(date +%Y%m%d-%H%M%S)"

SERVICES=(fathom agentic-kit app-kit throughline geoffmiller-cloud resume_as_code growing-gardens-bi car-assistant crossplane-platform)
declare -A SERVICE_PATHS=(
  [crossplane-platform]="$FLEET_BASE/planning/projects/pdk/substrates/timoni/crossplane-platform"
)

echo "==> [1/8] preflight"
STAMP_FILE="$FATHOM_REPO/workspace/logs/store-migration-stamp.txt"
if [ -f "$STAMP_FILE" ]; then
  echo "ERROR: already migrated ($(cat "$STAMP_FILE")) — refusing to re-run."
  echo "  A re-run would re-migrate from the STALE $SRC and silently discard"
  echo "  everything written to $DST since. If you really mean it: rm $STAMP_FILE"
  exit 1
fi
[ -x "$BIN" ] || { echo "ERROR: no binary — build it first: (cd $FATHOM_REPO && GOTMPDIR=/Volumes/m2ext/tmp/gobuild go build -o bin/fathom ./cmd/fathom)"; exit 1; }
[ -f "$SRC" ] || { echo "ERROR: source store $SRC not found — nothing to migrate."; exit 1; }
VERSION_LINE="$("$BIN" version)"
case "$VERSION_LINE" in *"(dirty)"*)
  echo "ERROR: bin/fathom is a DIRTY build ($VERSION_LINE) — it may not match any commit."
  echo "  Rebuild from a clean tree so 'binary at HEAD' means what it says."
  exit 1
esac
BIN_COMMIT="$(printf '%s' "$VERSION_LINE" | sed -n 's/.*@ \([0-9a-f]*\).*/\1/p')"
HEAD_COMMIT="$(git -C "$FATHOM_REPO" rev-parse --short=12 HEAD)"
if [ "$BIN_COMMIT" != "$HEAD_COMMIT" ]; then
  echo "ERROR: bin/fathom is at $BIN_COMMIT but the repo is at $HEAD_COMMIT — rebuild first:"
  echo "  (cd $FATHOM_REPO && GOTMPDIR=/Volumes/m2ext/tmp/gobuild go build -o bin/fathom ./cmd/fathom)"
  exit 1
fi
echo "    binary $BIN_COMMIT == HEAD (clean build), source store present, no prior migration stamp"

echo "==> [2/8] stopping the daemon"
launchctl bootout "$DOMAIN/$LABEL" 2>/dev/null || true
for _ in $(seq 1 20); do pgrep -f "fathom serve-http" >/dev/null || break; sleep 1; done
if pgrep -f "fathom serve-http" >/dev/null; then
  echo "ERROR: a serve-http process is still holding the store lock; reap it and retry."; exit 1
fi

echo "==> [3/8] quarantining the mismatched pre-migration fleet.sqlite (DuckDB bytes under a .sqlite name)"
quarantine_target() {
  local suffix="$1" f
  for f in "$DST" "$DST-wal" "$DST-shm" "$DST.wal"; do
    if [ -f "$f" ]; then
      mv "$f" "$f.$suffix"
      echo "    moved $(basename "$f") -> $(basename "$f").$suffix"
    fi
  done
}
quarantine_target "pre-migration-$TS"

echo "==> [4/8] migrating non-derivable tables (source opened read-only, hash-verified)"
if ! "$BIN" migrate-store --from "$SRC" --to "$DST"; then
  # A sibling repo's post-commit hook may have recreated $DST in the window
  # since quarantine (both repos' hooks write fleet.sqlite now). Quarantine
  # again and retry ONCE; a second failure is real.
  echo "    migrate-store failed — re-quarantining (sibling hook race?) and retrying once"
  quarantine_target "pre-migration-retry-$TS"
  "$BIN" migrate-store --from "$SRC" --to "$DST"
fi

echo "==> [5/8] re-ingesting the fleet (epoch-5 FULL re-extract — one time, ~1min)"
for svc in "${SERVICES[@]}"; do
  path="${SERVICE_PATHS[$svc]:-$FLEET_BASE/$svc}"
  if [ "$svc" = "fathom" ] && [ ! -d "$path/.git" ]; then path="$FATHOM_REPO"; fi
  if ! git -C "$path" rev-parse --show-toplevel >/dev/null 2>&1; then echo "    - skip $svc (not a git repo: $path)"; continue; fi
  t0=$SECONDS
  if out="$("$BIN" ingest --service "$svc" --path "$path" --store "$DST" 2>&1)"; then
    echo "    - $svc  $(echo "$out" | awk '/^symbols:/{print "→ "$2" symbols"}')  ($((SECONDS-t0))s)"
  else
    echo "    - $svc  FAILED:"; echo "$out" | tail -3 | sed 's/^/        /'
  fi
done

echo "==> [6/8] installing the daemon binary to $INSTALL_DIR (decoupled from repo rebuilds)"
mkdir -p "$INSTALL_DIR"
cp -f "$BIN" "$INSTALL_DIR/fathom"

echo "==> [7/8] repointing the launchd plist (libexec binary, sqlite store, go on PATH)"
[ -f "$PLIST" ] && cp "$PLIST" "$PLIST.bak-$TS" && echo "    plist backup: $PLIST.bak-$TS"
cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>$LABEL</string>
  <!-- The daemon runs an INSTALLED copy (internal disk), never the repo's
       bin/fathom directly: post-commit rebuilds must not swap the running
       daemon's binary out from under launchd's KeepAlive. Deploy a new
       build with: cp bin/fathom $INSTALL_DIR/fathom && launchctl kickstart
       -k $DOMAIN/$LABEL (from a real terminal). TCC: a new binary means a
       new code hash — if the first start after a deploy hangs, answer the
       macOS removable-volume dialog with Allow. -->
  <key>ProgramArguments</key><array>
    <string>$INSTALL_DIR/fathom</string>
    <string>serve-http</string>
    <string>--store</string><string>$DST</string>
    <string>--addr</string><string>127.0.0.1:7575</string>
    <string>--call-log</string><string>$FATHOM_REPO/workspace/logs/mcp-calls.jsonl</string>
  </array>
  <key>EnvironmentVariables</key><dict>
    <!-- go on PATH so daemon-side auto-refresh runs typed-edge enrichment
         (go/packages shells out to 'go list'); without it every refresh
         silently falls back to syntax-only edges for Go services. -->
    <key>PATH</key><string>$GO_BIN_DIR:/usr/bin:/bin:/usr/sbin:/sbin</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>WorkingDirectory</key><string>$FATHOM_REPO</string>
  <key>StandardOutPath</key><string>/tmp/fathom-http.log</string>
  <key>StandardErrorPath</key><string>/tmp/fathom-http.err</string>
</dict></plist>
EOF

echo "==> [8/8] restarting the daemon"
echo "    (if this hangs >20s: find the macOS permission dialog and click ALLOW)"
# NO kickstart fallback here: kickstart restarts the PREVIOUSLY LOADED job
# definition — old binary, old DuckDB store — and every later check would
# then pass against the wrong store. Bootstrap the new plist or fail loudly.
booted=0
for _ in 1 2 3 4 5; do
  if launchctl bootstrap "$DOMAIN" "$PLIST" 2>/dev/null; then booted=1; break; fi
  sleep 2
done
if [ "$booted" != 1 ]; then
  echo "ERROR: launchctl bootstrap kept failing — the label may still be registered."
  echo "  Try: launchctl bootout $DOMAIN/$LABEL ; launchctl bootstrap $DOMAIN $PLIST"
  exit 1
fi
ok=0
for i in $(seq 1 60); do
  if curl -s -m 2 http://localhost:7575/healthz 2>/dev/null | grep -q ok; then echo "    healthz OK (${i}s)"; ok=1; break; fi
  sleep 1
done
if [ "$ok" != 1 ]; then
  echo "    daemon not up — check /tmp/fathom-http.err and the TCC dialog note above."
  echo "    Rollback: cp $PLIST.bak-$TS $PLIST && launchctl bootout $DOMAIN/$LABEL; launchctl bootstrap $DOMAIN $PLIST"
  exit 1
fi
# Identity assertion: healthz alone can't distinguish the new daemon from a
# stale one — require the running command line to show BOTH the installed
# binary and the sqlite store.
if ! pgrep -fl "fathom serve-http" | grep -q -- "$INSTALL_DIR/fathom .*$DST"; then
  echo "ERROR: a serve-http process answered healthz but is NOT the installed binary on the sqlite store:"
  pgrep -fl "fathom serve-http" | sed 's/^/    /'
  exit 1
fi
echo "    identity OK: installed binary, sqlite store"

# Informational only from here — a slow first RPC must not fail a migration
# that already passed healthz + identity (hence the || true).
echo "    services + symbol counts:"
curl -s -m 8 -X POST http://localhost:7575/rpc -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{}}}' 2>/dev/null \
  | python3 -c 'import sys,json
r=json.load(sys.stdin).get("result",{}); c=r.get("content")
p=json.loads(c[0]["text"]) if isinstance(c,list) and c else r
for s in p.get("services",[]): print("      %-20s %s symbols" % (s.get("service"), s.get("symbol_count")))' || true

echo "    freshness:"
curl -s -m 8 -X POST http://localhost:7575/rpc -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"staleness_report","arguments":{"service":"fathom"}}}' 2>/dev/null \
  | python3 -c 'import sys,json
r=json.load(sys.stdin).get("result",{}); c=r.get("content")
p=json.loads(c[0]["text"]) if isinstance(c,list) and c else r
print("      fathom indexed_commit:", p.get("indexed_commit") or "(empty — check ingest)")' || true

mkdir -p "$FATHOM_REPO/workspace/logs"
STAMP="$(date -u +%FT%TZ) binary=$BIN_COMMIT store=$DST"
echo "$STAMP" > "$STAMP_FILE"
[ -f "$FATHOM_REPO/workspace/logs/xt6b-bake-start.txt" ] || echo "$STAMP" > "$FATHOM_REPO/workspace/logs/xt6b-bake-start.txt"

echo
echo "==> done. SQLite fleet store live; daemon runs from $INSTALL_DIR/fathom."
echo "    Old DuckDB store kept as rollback: $SRC (delete after a day green —"
echo "    until then, same-stem collision warnings from store scans are expected)."
