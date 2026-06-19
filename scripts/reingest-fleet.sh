#!/usr/bin/env bash
#
# Rebuild the sextant fleet index from scratch with clean service IDs and path
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
# tool). It is safe + idempotent: the old store is backed up, not deleted.
#
#   bash scripts/reingest-fleet.sh
#
set -euo pipefail

SEXTANT_REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$SEXTANT_REPO/bin/sextant"
STORE="$SEXTANT_REPO/workspace/db/fleet.duckdb"
FLEET_BASE="$(dirname "$SEXTANT_REPO")"
LABEL="com.geoffmilleraz.sextant-http"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
DOMAIN="gui/$(id -u)"

# Fleet services — ingested as "<name>" from "$FLEET_BASE/<name>".
SERVICES=(sextant agentic-kit app-kit throughline geoffmiller-cloud resume_as_code)

echo "==> sextant fleet re-ingest  (store: $STORE)"
[ -x "$BIN" ] || { echo "ERROR: build the binary first:  (cd $SEXTANT_REPO && go build -o bin/sextant ./cmd/sextant)"; exit 1; }

echo "==> stopping the daemon (releases the single-writer DuckDB lock)"
launchctl bootout "$DOMAIN/$LABEL" 2>/dev/null || true
for _ in $(seq 1 15); do pgrep -f "bin/sextant serve-http" >/dev/null || break; sleep 1; done
if pgrep -f "bin/sextant serve-http" >/dev/null; then
  echo "ERROR: a serve-http process is still holding the lock; reap it and retry."; exit 1
fi

echo "==> backing up + removing the old store (drops malformed duplicate services)"
if [ -f "$STORE" ]; then
  ts="$(date +%Y%m%d-%H%M%S)"
  mv "$STORE" "$STORE.bak-$ts"
  [ -f "$STORE.wal" ] && mv "$STORE.wal" "$STORE.wal.bak-$ts" || true
  echo "    backup: $STORE.bak-$ts  (delete once you've confirmed the new index)"
fi

echo "==> ingesting each fleet repo with a clean service ID + path"
for svc in "${SERVICES[@]}"; do
  path="$FLEET_BASE/$svc"
  if [ ! -d "$path/.git" ]; then echo "    - skip $svc (no git repo at $path)"; continue; fi
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
if [ "$ok" != 1 ]; then echo "    daemon not up yet — check /tmp/sextant-http.err"; exit 1; fi

echo "    services + symbol counts (no '../' duplicates expected):"
curl -s -m 8 -X POST http://localhost:7575/rpc -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{}}}' 2>/dev/null \
  | python3 -c 'import sys,json
r=json.load(sys.stdin).get("result",{}); c=r.get("content")
p=json.loads(c[0]["text"]) if isinstance(c,list) and c else r
for s in p.get("services",[]): print("      %-20s %s symbols" % (s.get("service"), s.get("symbol_count")))'

echo "    freshness (indexed_commit should now be populated):"
curl -s -m 8 -X POST http://localhost:7575/rpc -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"staleness_report","arguments":{"service":"sextant"}}}' 2>/dev/null \
  | python3 -c 'import sys,json
r=json.load(sys.stdin).get("result",{}); c=r.get("content")
p=json.loads(c[0]["text"]) if isinstance(c,list) and c else r
print("      sextant indexed_commit:", p.get("indexed_commit") or "(still empty — check ingest)")'

echo "==> done. Clean fleet index with path metadata; the daemon self-refreshes on HEAD drift."
