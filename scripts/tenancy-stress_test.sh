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
# Here-strings, not `printf | grep -q`: under pipefail grep exiting on its first
# match SIGPIPEs the writer of a multi-chunk value (scripts/pipefail-grep_test.sh).
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
  "verdict: no diag capture reads FAIL" \
  "ledger: a breaker row is attributed to the start it decides (safe 4, stop 6)"; do
  grep -qF "$want" <<<"$out" || fail "self-check output lacks \"$want\":
$out"
done
if grep -q "FAILED" <<<"$out"; then
  fail "self-check reported a failure:
$out"
fi

# The harness names its scratch-daemon contract in the script itself.
grep -q -- '--lifecycle-id' "${SCRIPT_DIR}/tenancy-stress.sh" || fail "the harness no longer passes --lifecycle-id"
# rt-waves-c-f reading (2026-09-27): /healthz answers 503 while the store
# opens, so readiness needs curl -f; resources needs the run's --url or it
# reports the managed daemon.
grep -q 'curl -sf -m 1 "http://127.0.0.1:${port}/healthz"' "${SCRIPT_DIR}/tenancy-stress.sh" || fail "the readiness probe does not fail on a 503 /healthz"
grep -q 'resources --id "$lifecycle_id" --url "http://127.0.0.1:${port}"' "${SCRIPT_DIR}/tenancy-stress.sh" || fail "resources is not pointed at the run's daemon"
grep -q -- '--exit-with-parent' "${SCRIPT_DIR}/tenancy-stress.sh" || fail "the harness no longer ties the daemon to its parent"

echo "tenancy-stress test: ok (managed port, store, call log and internal-disk fixture refused; go-ahead required)"
