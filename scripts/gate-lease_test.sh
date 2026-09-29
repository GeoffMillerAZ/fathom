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

# A killed holder releases the lease (the kernel drops the flock). The killed
# holder's orphaned stub sleeps for an hour (it records its pid and execs
# sleep, so it is one process the test can kill), so a lease that were still
# held would leave the next holder waiting for good: the next holder finishing
# at all is "released at once", and the poll below is only a hang guard (600
# polls of 0.1 s), never a latency the lease is held to (EW-7).
hold="${scratch}/hold.sh"
hold_pid="${scratch}/hold.pid"
printf '#!/usr/bin/env bash\necho $$ >"$1"\nexec sleep 3600\n' >"$hold"
chmod +x "$hold"
trap 'kill "$(cat "$hold_pid" 2>/dev/null)" 2>/dev/null || true; rm -rf "$scratch"' EXIT
bash "${SCRIPT_DIR}/gate-lease.sh" "$hold" "$hold_pid" &
pk=$!
for _ in $(seq 1 100); do
  [ -s "$hold_pid" ] && break
  sleep 0.05
done
[ -s "$hold_pid" ] || fail "the holder never started"
kill -9 "$pk" 2>/dev/null || true
wait "$pk" 2>/dev/null || true
bash "${SCRIPT_DIR}/gate-lease.sh" true &
pn=$!
for _ in $(seq 1 600); do
  kill -0 "$pn" 2>/dev/null || break
  sleep 0.1
done
if kill -0 "$pn" 2>/dev/null; then
  kill "$pn" 2>/dev/null || true
  fail "a killed holder's lease was not released"
fi
wait "$pn" || fail "the lease after a killed holder"
kill "$(cat "$hold_pid")" 2>/dev/null || true

# The default priority is nice 10, never macOS background QoS (which ran
# the gate ~3x slower and timed cmd/fathom out).
got_nice="$(env -u FATHOM_GATE_NICE bash "${SCRIPT_DIR}/gate-lease.sh" sh -c 'ps -o nice= -p $$' | tr -d ' ')"
# nice is relative: under a caller already niced (the gate runs this test inside its
# own lease, at nice 10) the lease adds 10 more, capped at 20.
base_nice="$(ps -o nice= -p $$ | tr -d ' ')"
want_nice=$(( base_nice + 10 )); [ "$want_nice" -gt 20 ] && want_nice=20
[ "$got_nice" = "$want_nice" ] || fail "default priority: nice $got_nice, want $want_nice (caller at $base_nice)"

# The pre-push gate's heavy steps (the scratch ingest and the verify runs)
# take the lease too, not only `task validate` (§12.3).
hooks="${SCRIPT_DIR}/../lefthook.yml"
grep -q 'lease="bash scripts/gate-lease.sh"' "$hooks" || fail "the pre-push verify step defines no gate lease"
if grep -E 'bin/fathom (ingest|verify)' "$hooks" | grep -v '^ *#' | grep -vq '\$lease bin/fathom'; then
  fail "lefthook.yml runs an ingest or verify outside the gate lease: $(grep -E 'bin/fathom (ingest|verify)' "$hooks" | grep -v '^ *#' | grep -v '\$lease bin/fathom')"
fi

echo "gate-lease test: ok (two holders serialized, holder named, status passed, killed holder released)"
