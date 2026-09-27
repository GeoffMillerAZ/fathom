#!/usr/bin/env bash
# gate-lease_test.sh — RT-129 (design §12.3): two concurrent holders of the
# gate lease around a stub command serialize; the waiter names the holder;
# the command's exit status passes through. Run by `task scripts-check`.
# Uses its own lock file, never ~/.fathom.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"

fail() {
  echo "gate-lease test: $*" >&2
  exit 1
}

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-gate-lease-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
export FATHOM_GATE_LOCK="${scratch}/locks/gate.lock"
export FATHOM_GATE_NICE=off

# The stub records its start and end; each holds the lease ~1 s.
stub="${scratch}/stub.sh"
cat >"$stub" <<'STUB'
#!/usr/bin/env bash
echo "start $1" >>"$2"
sleep 1
echo "end $1" >>"$2"
STUB
chmod +x "$stub"
log="${scratch}/order.log"

bash "${SCRIPT_DIR}/gate-lease.sh" "$stub" A "$log" 2>"${scratch}/a.err" &
pa=$!
# Let A take the lease before B asks for it.
for _ in $(seq 1 50); do
  grep -q "start A" "$log" 2>/dev/null && break
  sleep 0.05
done
grep -q "start A" "$log" || fail "the first holder never started"
bash "${SCRIPT_DIR}/gate-lease.sh" "$stub" B "$log" 2>"${scratch}/b.err" &
pb=$!
wait "$pa" || fail "holder A failed"
wait "$pb" || fail "holder B failed"

got="$(tr '\n' ' ' <"$log")"
[ "$got" = "start A end A start B end B " ] || fail "the two holders overlapped: $got"
grep -q "waiting for gate lease held by pid [0-9]* (repo " "${scratch}/b.err" || fail "the waiter did not name the holder: $(cat "${scratch}/b.err")"

# The command's status passes through.
set +e
bash "${SCRIPT_DIR}/gate-lease.sh" bash -c 'exit 7'
rc=$?
set -e
[ "$rc" -eq 7 ] || fail "exit status: got $rc, want 7"

# A killed holder releases the lease (the kernel drops the flock).
hold="${scratch}/hold.sh"
printf '#!/usr/bin/env bash\nsleep 30\n' >"$hold"
chmod +x "$hold"
bash "${SCRIPT_DIR}/gate-lease.sh" "$hold" &
pk=$!
sleep 0.3
kill -9 "$pk" 2>/dev/null || true
wait "$pk" 2>/dev/null || true
start=$(date +%s)
bash "${SCRIPT_DIR}/gate-lease.sh" true || fail "the lease after a killed holder"
# The orphaned stub keeps no lock: the next holder got it at once.
[ $(( $(date +%s) - start )) -lt 5 ] || fail "a killed holder's lease was not released"
pkill -f "$hold" 2>/dev/null || true

echo "gate-lease test: ok (two holders serialized, holder named, status passed, killed holder released)"
