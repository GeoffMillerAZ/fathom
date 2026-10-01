#!/usr/bin/env bash
#
# install-intent_test.sh — proves install-intent.sh links every intent-* skill from the
# kit's one real copy (.claude/skills) into a disposable HOME for both Codex
# (~/.agents/skills) and Claude Code (~/.claude/skills), that a re-run is idempotent,
# and that --uninstall removes exactly what it made. Never touches the real HOME.
# Wired into the kit's `task scripts-check`.
#
# Usage:
#   bash scripts/install-intent_test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL="${SCRIPT_DIR}/install-intent.sh"
KIT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"

fail() {
  echo "install-intent test: $*" >&2
  exit 1
}

scratch="$(mktemp -d "${TMPDIR:-/tmp}/install-intent-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
home="$scratch/home"
mkdir -p "$home"

# The kit layout this script depends on: real skills in .claude/skills, and
# .agents/skills the relative link to them.
[ -L "$KIT/.agents/skills" ] || fail ".agents/skills is not a link"
[ "$(readlink "$KIT/.agents/skills")" = "../.claude/skills" ] || fail ".agents/skills does not link to ../.claude/skills"

want=()
for d in "$KIT"/.claude/skills/intent-*/; do
  if [ -L "${d%/}" ]; then fail "${d%/} is a link; .claude/skills must hold the real copy"; fi
  if [ -f "$d/SKILL.md" ]; then want+=("$(basename "$d")"); fi
done
[ "${#want[@]}" -ge 8 ] || fail "found ${#want[@]} intent-* skills in .claude/skills, want at least 8"

check_installed() {
  local s
  for s in "${want[@]}"; do
    [ -f "$home/.agents/skills/$s/SKILL.md" ] || fail "codex: $s does not resolve"
    [ -f "$home/.claude/skills/$s/SKILL.md" ] || fail "claude: $s does not resolve"
    [ "$(readlink "$home/.agents/skills/$s")" = "$KIT/.claude/skills/$s" ] ||
      fail "codex: $s links to $(readlink "$home/.agents/skills/$s"), want $KIT/.claude/skills/$s"
    cmp -s "$home/.claude/skills/$s/SKILL.md" "$KIT/.claude/skills/$s/SKILL.md" ||
      fail "claude: $s is not the kit's copy"
  done
}

# Run from an unrelated working directory: the script finds the kit from its own path.
(cd "$scratch" && HOME="$home" bash "$INSTALL") >/dev/null || fail "install exited non-zero"
check_installed
(cd "$scratch" && HOME="$home" bash "$INSTALL") >/dev/null || fail "re-run exited non-zero"
check_installed

# A pre-existing personal skill that is not intent-* is left alone.
mkdir -p "$home/.claude/skills/mine" && echo mine >"$home/.claude/skills/mine/SKILL.md"
(cd "$scratch" && HOME="$home" bash "$INSTALL" --uninstall) >/dev/null || fail "uninstall exited non-zero"
for s in "${want[@]}"; do
  if [ -e "$home/.agents/skills/$s" ] || [ -L "$home/.agents/skills/$s" ]; then fail "codex: $s survived uninstall"; fi
  if [ -e "$home/.claude/skills/$s" ] || [ -L "$home/.claude/skills/$s" ]; then fail "claude: $s survived uninstall"; fi
done
[ -f "$home/.claude/skills/mine/SKILL.md" ] || fail "uninstall removed a skill it did not install"

echo "install-intent test: ok (${#want[@]} skills)"
