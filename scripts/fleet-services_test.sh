#!/usr/bin/env bash
# fleet-services_test.sh — scripts/lib/fleet-services.sh reads the fleet list
# from a gitignored workspace/fleet-services (comments and blank lines
# skipped), and without the file falls back to fathom alone and says so on
# stderr. Run by `task scripts-check`.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# shellcheck source=scripts/lib/fleet-services.sh
source "${SCRIPT_DIR}/lib/fleet-services.sh"

fail() {
  echo "fleet-services test: $*" >&2
  exit 1
}

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-fleet-services-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

# 1. No file: fathom alone, and stderr names the missing list.
fathom_fleet_services "$scratch" 2>"$scratch/err"
[ "${SERVICES[*]}" = "fathom" ] || fail "no file gave (${SERVICES[*]}), want (fathom)"
grep -q 'fathom alone' "$scratch/err" || fail "no file was not reported on stderr"

# 2. A file: one name per line, comments, blank lines and stray spaces dropped,
# order kept, and a last line without a newline still read.
mkdir -p "$scratch/workspace"
printf '# the fleet\nfathom\n\n  svc-a  # first\nsvc-b' >"$scratch/workspace/fleet-services"
fathom_fleet_services "$scratch" 2>"$scratch/err"
[ "${SERVICES[*]}" = "fathom svc-a svc-b" ] || fail "file gave (${SERVICES[*]}), want (fathom svc-a svc-b)"
[ ! -s "$scratch/err" ] || fail "a readable list still wrote to stderr: $(cat "$scratch/err")"

# 3. A file with only comments counts as no list.
printf '# nothing yet\n\n' >"$scratch/workspace/fleet-services"
fathom_fleet_services "$scratch" 2>"$scratch/err"
[ "${SERVICES[*]}" = "fathom" ] || fail "comment-only file gave (${SERVICES[*]}), want (fathom)"

echo "fleet-services test: ok"
