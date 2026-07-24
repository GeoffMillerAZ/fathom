#!/usr/bin/env bash
#
# complete-dir-rename.sh — the LAST step of the sextant→fathom migration.
#
# Everything else is done: module path, binary, launchd label
# (com.geoffmilleraz.fathom-http), MCP registration ("fathom" → localhost:7575),
# code, and docs. The only surviving "sextant" is the on-disk DIRECTORY NAME and
# the absolute paths that point at it (the launchd plist and two Claude Code
# hooks). This script finishes that rename atomically.
#
# ⚠️ RUN THIS FROM A REAL TERMINAL, NOT VIA AN AGENT/Bash-tool.
#   Two reasons an agent cannot do this safely:
#   1. It reloads the launchd daemon — launchctl daemon ops wedge when driven
#      from the Claude Code Bash tool.
#   2. The daemon binary path holds a PATH-KEYED TCC RemovableVolumes grant.
#      Moving the directory invalidates it; the daemon's next start touches
#      /Volumes/m2ext and HANGS in tccd until you answer a macOS "Allow" dialog
#      — a GUI click only a human at the machine can make.
#
# ⚠️ CLOSE other Claude Code / editor sessions working in this directory first
#   (e.g. the Throughline session that also commits here) — their cwd breaks on
#   the move.
#
# Idempotent: safe to re-run. If the directory is already renamed it just
# repoints config and reloads.

set -euo pipefail

OLD="/Volumes/m2ext/code/geoffmilleraz/sextant"
NEW="/Volumes/m2ext/code/geoffmilleraz/fathom"
LABEL="com.geoffmilleraz.fathom-http"
PLIST="$HOME/Library/LaunchAgents/${LABEL}.plist"
SETTINGS="$HOME/.claude/settings.json"
UID_NUM="$(id -u)"

say() { printf '\n\033[1;36m▸ %s\033[0m\n' "$*"; }
warn() { printf '\033[1;33m! %s\033[0m\n' "$*"; }

# 1. Stop the daemon so nothing holds the old path / store lock during the move.
say "Stopping daemon ${LABEL} (ignore 'No such process' if already down)"
launchctl bootout "gui/${UID_NUM}/${LABEL}" 2>/dev/null || true

# 2. Rename the directory (only if it hasn't been done already).
if [[ -d "$OLD" && ! -e "$NEW" ]]; then
  say "Renaming directory: $OLD  →  $NEW"
  mv "$OLD" "$NEW"
elif [[ -d "$NEW" ]]; then
  warn "Directory already named fathom — skipping mv."
else
  echo "ERROR: neither $OLD nor $NEW exists as expected." >&2
  exit 1
fi

# 3. Repoint the launchd plist (binary, --store, --call-log, WorkingDirectory).
say "Repointing plist paths to /fathom: $PLIST"
/usr/bin/sed -i '' 's#geoffmilleraz/sextant#geoffmilleraz/fathom#g' "$PLIST"

# 4. Repoint the two Claude Code hook command paths (session-hook, edit-hook).
if [[ -f "$SETTINGS" ]]; then
  say "Repointing Claude Code hook paths in $SETTINGS (backup: ${SETTINGS}.bak)"
  /usr/bin/sed -i '.bak' 's#geoffmilleraz/sextant/bin/fathom#geoffmilleraz/fathom/bin/fathom#g' "$SETTINGS"
else
  warn "No $SETTINGS found — skipping hook repoint (adjust manually if you use hooks)."
fi

# 5. Reload the daemon from the corrected plist.
say "Reloading daemon"
launchctl bootstrap "gui/${UID_NUM}" "$PLIST"
launchctl kickstart -k "gui/${UID_NUM}/${LABEL}" 2>/dev/null || true

# 6. Verify it came up. If it does NOT, the likely cause is the TCC re-grant.
say "Verifying daemon is listening on 127.0.0.1:7575 (waiting up to ~15s)"
ok=""
for _ in $(seq 1 15); do
  if /usr/sbin/lsof -nP -iTCP:7575 -sTCP:LISTEN >/dev/null 2>&1; then ok=1; break; fi
  sleep 1
done

if [[ -n "$ok" ]]; then
  printf '\n\033[1;32m✔ fathom daemon is up on 127.0.0.1:7575 from the renamed directory.\033[0m\n'
  echo "  Migration complete. Rebuild the fleet index if you want clean paths:"
  echo "    $NEW/scripts/reingest-fleet.sh"
else
  warn "Daemon did NOT come up within 15s."
  warn "This is almost certainly the path-keyed TCC RemovableVolumes grant:"
  warn "  the new binary path ($NEW/bin/fathom) has no grant yet, so tccd is"
  warn "  blocking its access to /Volumes/m2ext. Look for a macOS permission"
  warn "  dialog and click Allow, then re-run this script. Diagnostics:"
  echo "    tail -5 /tmp/fathom-http.err"
  echo "    log show --last 2m --predicate 'eventMessage CONTAINS \"deny\"' --style compact | grep -i fathom"
  exit 1
fi
