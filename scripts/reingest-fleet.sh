#!/usr/bin/env bash
#
# Rebuild the fathom fleet index from scratch with clean service IDs and path
# metadata.
#
# Why: the live fleet store was ingested without ServiceMeta.path/commit (and
# with malformed "../app-kit"-style duplicate services). Because auto-refresh,
# staleness_report, and the `ingest` MCP tool all rely on ServiceMeta.path, the
# daemon's index was frozen — it could serve the original symbols but never pick
# up new commits. This re-ingests each fleet repo with `--service <name>
# --path <repo>`, which records the path + git HEAD, so the daemon can finally
# self-refresh. Dropping and recreating the store also removes the malformed
# duplicates (the ingest guard now rejects path-like IDs).
#
# Run this in YOUR terminal — it stops/starts the launchd daemon via launchctl,
# which must run in your login session (it wedges when issued from a sandboxed
# tool). The old store file is backed up, not deleted.
#
# ⚠️ NOT non-destructive, despite the backup. This rebuilds the index from
#   `ingest` alone, which regenerates ONLY observations/fingerprints/commits.
#   Everything produced by the compile and enrichment passes is LOST from the
#   live store — today that is 628 semantic_claims, 1293 claim_links and 14
#   compile_runs for the fathom service alone. `ingest` cannot regenerate them;
#   only `scripts/recompile-fleet.sh` can, and that costs LLM calls.
#   Reach for this script ONLY to rebuild from scratch (malformed/duplicate
#   service IDs, missing ServiceMeta.path). To fix paths after a directory
#   move, use `fathom remap` instead — it is surgical and free.
#   After running this, run scripts/recompile-fleet.sh or obligations
#   injection stays dead.
#
#   bash scripts/reingest-fleet.sh
#
set -euo pipefail

FATHOM_REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$FATHOM_REPO/bin/fathom"
STORE="$FATHOM_REPO/workspace/db/fleet.sqlite"
FLEET_BASE="$(dirname "$FATHOM_REPO")"
LABEL="com.geoffmilleraz.fathom-http"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
DOMAIN="gui/$(id -u)"

# Fleet services — ingested as "<name>" from "$FLEET_BASE/<name>".
SERVICES=(fathom agentic-kit app-kit throughline geoffmiller-cloud resume_as_code growing-gardens-bi car-assistant crossplane-platform)
# Services whose corpus is NOT $FLEET_BASE/<svc> (nested project roots).
declare -A SERVICE_PATHS=(
  [crossplane-platform]="$FLEET_BASE/planning/projects/pdk/substrates/timoni/crossplane-platform"
)

echo "==> fathom fleet re-ingest  (store: $STORE)"
[ -x "$BIN" ] || { echo "ERROR: build the binary first:  (cd $FATHOM_REPO && go build -o bin/fathom ./cmd/fathom)"; exit 1; }

echo "==> stopping the daemon (releases the SQLite write lock)"
launchctl bootout "$DOMAIN/$LABEL" 2>/dev/null || true
# NOTE the pattern: the daemon runs the INSTALLED copy
# (~/.local/libexec/fathom/fathom serve-http), so the substring "bin/fathom"
# never appears in its command line — this guard matched nothing for as long as
# the install-to-a-stable-path convention has existed, silently clearing the way
# to `mv` a live store. Match on "fathom serve-http", exactly as
# recompile-fleet.sh:62 and migrate-fleet-store.sh:104 already do.
for _ in $(seq 1 15); do pgrep -f "fathom serve-http" >/dev/null || break; sleep 1; done
if pgrep -f "fathom serve-http" >/dev/null; then
  echo "ERROR: a serve-http process is still holding the lock; reap it and retry."; exit 1
fi

echo "==> backing up + removing the old store (drops malformed duplicate services)"
if [ -f "$STORE" ]; then
  ts="$(date +%Y%m%d-%H%M%S)"
  mv "$STORE" "$STORE.bak-$ts"
  # SQLite names the sidecars with a HYPHEN (fleet.sqlite-wal / -shm), not a
  # dot — the old "$STORE.wal" test never matched, so a non-empty WAL was left
  # beside the fresh database.
  [ -f "$STORE-wal" ] && mv "$STORE-wal" "$STORE-wal.bak-$ts" || true
  [ -f "$STORE-shm" ] && mv "$STORE-shm" "$STORE-shm.bak-$ts" || true
  echo "    backup: $STORE.bak-$ts  (delete once you've confirmed the new index)"
fi

echo "==> ingesting each fleet repo with a clean service ID + path"
for svc in "${SERVICES[@]}"; do
  path="${SERVICE_PATHS[$svc]:-$FLEET_BASE/$svc}"
  # The fathom repo itself may still live in a directory named after the old
  # brand — resolve its service to this script's own repo root.
  if [ "$svc" = "fathom" ] && [ ! -d "$path/.git" ]; then path="$FATHOM_REPO"; fi
  if ! git -C "$path" rev-parse --show-toplevel >/dev/null 2>&1; then echo "    - skip $svc (not inside a git repo: $path)"; continue; fi
  if out="$("$BIN" ingest --service "$svc" --path "$path" --store "$STORE" 2>&1)"; then
    echo "    - $svc  $(echo "$out" | awk '/^symbols:/{print "→ "$2" symbols"}')  ($path)"
  else
    echo "    - $svc  FAILED:"; echo "$out" | tail -2 | sed 's/^/        /'
  fi
done

echo "==> restarting the daemon"
launchctl bootstrap "$DOMAIN" "$PLIST" 2>/dev/null || launchctl kickstart -k "$DOMAIN/$LABEL" 2>/dev/null || true

echo "==> verifying"
ok=0
for i in $(seq 1 30); do
  if curl -s -m 2 http://localhost:7575/healthz 2>/dev/null | grep -q ok; then echo "    healthz OK (${i}s)"; ok=1; break; fi
  sleep 1
done
if [ "$ok" != 1 ]; then echo "    daemon not up yet — check /tmp/fathom-http.err"; exit 1; fi

echo "    services + symbol counts (no '../' duplicates expected):"
curl -s -m 8 -X POST http://localhost:7575/rpc -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{}}}' 2>/dev/null \
  | python3 -c 'import sys,json
r=json.load(sys.stdin).get("result",{}); c=r.get("content")
p=json.loads(c[0]["text"]) if isinstance(c,list) and c else r
for s in p.get("services",[]): print("      %-20s %s symbols" % (s.get("service"), s.get("symbol_count")))'

echo "    freshness (indexed_commit should now be populated):"
curl -s -m 8 -X POST http://localhost:7575/rpc -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"staleness_report","arguments":{"service":"fathom"}}}' 2>/dev/null \
  | python3 -c 'import sys,json
r=json.load(sys.stdin).get("result",{}); c=r.get("content")
p=json.loads(c[0]["text"]) if isinstance(c,list) and c else r
print("      fathom indexed_commit:", p.get("indexed_commit") or "(still empty — check ingest)")'

echo "==> done. Clean fleet index with path metadata; the daemon self-refreshes on HEAD drift."
