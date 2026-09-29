#!/usr/bin/env bash
# scratch-home_test.sh — PE-28: scratch-home.sh gives its command a fresh
# FATHOM_HOME under workspace/tmp/ and removes it afterwards, keeps a caller's
# own FATHOM_HOME (and does not delete it), and passes the exit status
# through. Run by `task scripts-check`.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"

fail() {
  echo "scratch-home test: $*" >&2
  exit 1
}

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-scratch-home-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

# 1. Unset: a fresh directory under workspace/tmp, gone afterwards.
seen="${scratch}/seen"
env -u FATHOM_HOME bash "${SCRIPT_DIR}/scratch-home.sh" bash -c 'printf "%s" "$FATHOM_HOME" >"$1"; [ -d "$FATHOM_HOME" ]' _ "$seen" ||
  fail "the command did not see an existing FATHOM_HOME directory"
home="$(cat "$seen")"
case "$home" in
  "${ROOT}/workspace/tmp/fathom-home."*) ;;
  *) fail "FATHOM_HOME=$home is not under ${ROOT}/workspace/tmp/" ;;
esac
[ ! -e "$home" ] || fail "FATHOM_HOME $home was not removed after the command"

# 2. Set: the caller's own is kept and left in place.
own="${scratch}/own"
mkdir -p "$own"
FATHOM_HOME="$own" bash "${SCRIPT_DIR}/scratch-home.sh" bash -c '[ "$FATHOM_HOME" = "$1" ]' _ "$own" ||
  fail "a caller's FATHOM_HOME was replaced"
[ -d "$own" ] || fail "a caller's FATHOM_HOME was deleted"

# 3. The exit status passes through, and the directory is still removed.
set +e
env -u FATHOM_HOME bash "${SCRIPT_DIR}/scratch-home.sh" bash -c 'printf "%s" "$FATHOM_HOME" >"$1"; exit 7' _ "$seen"
code=$?
set -e
[ "$code" -eq 7 ] || fail "exit status $code, want 7"
[ ! -e "$(cat "$seen")" ] || fail "FATHOM_HOME was not removed after a failing command"

# 4. No command is a usage error.
set +e
env -u FATHOM_HOME bash "${SCRIPT_DIR}/scratch-home.sh" 2>/dev/null
code=$?
set -e
[ "$code" -eq 2 ] || fail "no command: exit $code, want 2"

echo "scratch-home test: ok"
