#!/usr/bin/env bash
# tenancy-stress_test.sh — RT-168 (design §15.2): the stress harness
# refuses the managed daemon's port, store and call log, a fixture dir on
# the internal disk, and a run without the operator's go-ahead — proven in
# its --self-check mode, which sets every managed fact itself and never
# reads or starts anything of the operator's. Run by `task scripts-check`.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"

fail() {
  echo "tenancy-stress test: $*" >&2
  exit 1
}

out="$(bash "${SCRIPT_DIR}/tenancy-stress.sh" --self-check 2>&1)" || fail "--self-check failed:
$out"
for want in \
  "refused the managed port" \
  "refused the managed store" \
  "refused the managed call log" \
  "refused a fixture on the internal disk" \
  "a scratch port, store copy, call log and external fixture pass" \
  "a run without --i-have-the-operators-go-ahead is refused" \
  "verdict: the seatbelt held (safe mode by start 3, stop by start 5) reads PASS" \
  "verdict: no safe mode reads FAIL" \
  "verdict: safe mode first at start 4 reads FAIL" \
  "verdict: no stop reads FAIL" \
  "verdict: no diag capture reads FAIL"; do
  printf '%s\n' "$out" | grep -qF "$want" || fail "self-check output lacks \"$want\":
$out"
done
if printf '%s\n' "$out" | grep -q "FAILED"; then
  fail "self-check reported a failure:
$out"
fi

# The harness names its scratch-daemon contract in the script itself.
grep -q -- '--lifecycle-id' "${SCRIPT_DIR}/tenancy-stress.sh" || fail "the harness no longer passes --lifecycle-id"
grep -q -- '--exit-with-parent' "${SCRIPT_DIR}/tenancy-stress.sh" || fail "the harness no longer ties the daemon to its parent"

echo "tenancy-stress test: ok (managed port, store, call log and internal-disk fixture refused; go-ahead required)"
