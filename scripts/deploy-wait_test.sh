#!/usr/bin/env bash
# deploy-wait_test.sh — EW-17 (O129): `task deploy`'s health wait outlasts
# launchd's respawn throttle, and a wait that runs out prints launchd's last
# exit reason instead of a bare timeout. The waiter runs against a fake curl
# (healthy after N calls, so nothing here reads a clock) and a fake
# launchctl (a fixed `print` fixture, and a log proving the script only ever
# reads: it never kickstarts, boots out or kills a job). Run by
# `task scripts-check`, under macOS /bin/bash 3.2 as well as a newer bash.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
WAIT="${SCRIPT_DIR}/deploy-wait.sh"

fail() {
  echo "deploy-wait test: $*" >&2
  exit 1
}

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-deploy-wait-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
fake_bin="${scratch}/bin"
state="${scratch}/state"
mkdir -p "$fake_bin" "$state"

# curl: succeeds ("ok") once it has been called more than $FAKE_HEALTH_AFTER
# times, else fails like a refused connection.
cat >"${fake_bin}/curl" <<'EOF'
#!/usr/bin/env bash
n=$(cat "$FAKE_STATE/curl.count" 2>/dev/null || echo 0)
n=$((n + 1))
echo "$n" >"$FAKE_STATE/curl.count"
if [ "$n" -gt "${FAKE_HEALTH_AFTER:-1000000}" ]; then
  echo ok
  exit 0
fi
exit 7
EOF
# launchctl: `print` answers the fixture (or fails with FAKE_LAUNCHCTL_RC);
# everything is logged so the test can check it only ever read.
cat >"${fake_bin}/launchctl" <<'EOF'
#!/usr/bin/env bash
echo "$*" >>"$FAKE_STATE/launchctl.log"
if [ "$1" = print ]; then
  if [ "${FAKE_LAUNCHCTL_RC:-0}" != 0 ]; then
    echo 'Could not find service "x" in domain for user gui: 501' >&2
    exit "$FAKE_LAUNCHCTL_RC"
  fi
  cat "$FAKE_LAUNCHCTL_PRINT"
  exit 0
fi
exit 99
EOF
chmod +x "${fake_bin}/curl" "${fake_bin}/launchctl"

# What `launchctl print gui/<uid>/<label>` says on this machine's agent
# (ThrottleInterval 60 is "minimum runtime", ExitTimeOut 20 is "exit timeout").
fixture="${scratch}/print.txt"
cat >"$fixture" <<'EOF'
gui/501/com.example.fathom-http = {
	active count = 0
	state = spawn scheduled
	program = /Users/x/.local/libexec/fathom/fathom
	minimum runtime = 60
	exit timeout = 20
	runs = 11
	last exit reason = OS_REASON_CODESIGNING
	job state = spawn scheduled
}
EOF

# wait_run ARGS...: run the waiter with the fakes first on PATH; sets
# rc, out and the fakes' counters. FAKE_* and FATHOM_* come from the caller.
wait_run() {
  rm -f "$state/curl.count" "$state/launchctl.log"
  rc=0
  out="$(
    PATH="${fake_bin}:$PATH" FAKE_STATE="$state" FAKE_LAUNCHCTL_PRINT="$fixture" \
      FATHOM_DEPLOY_POLL_S=0 bash "$WAIT" "$@" 2>&1
  )" || rc=$?
  curls="$(cat "$state/curl.count" 2>/dev/null || echo 0)"
}

only_reads() {
  [ ! -e "$state/launchctl.log" ] || ! grep -v '^print ' "$state/launchctl.log" >/dev/null \
    || fail "the waiter ran a launchctl verb other than print: $(cat "$state/launchctl.log")"
}

# 1. A daemon that comes up after the throttle (61 refused polls) is waited
#    for: the old 30-try wait gave up 31 tries short.
export FAKE_HEALTH_AFTER=61
wait_run com.example.fathom-http
[ "$rc" -eq 0 ] || fail "a daemon healthy on poll 62 was reported failed (rc $rc): $out"
[ "$curls" -eq 62 ] || fail "expected 62 health polls, got $curls"
case "$out" in *"OS_REASON_CODESIGNING"*) ;; *) fail "a slow bring-up did not name launchd's last exit reason: $out" ;; esac
case "$out" in *"healthz ok"*) ;; *) fail "success was not announced: $out" ;; esac
only_reads

# 2. A daemon that never comes up fails after throttle + exit timeout + a
#    startup margin (60 + 20 + 30 = 110 polls), and the failure names launchd's
#    state and last exit reason, not a bare timeout.
export FAKE_HEALTH_AFTER=1000000
wait_run com.example.fathom-http
[ "$rc" -ne 0 ] || fail "a daemon that never came up was reported healthy"
[ "$curls" -eq 110 ] || fail "expected 110 health polls (60 throttle + 20 exit timeout + 30 margin), got $curls"
case "$out" in *"last exit reason = OS_REASON_CODESIGNING"*) ;; *) fail "the failure did not print launchd's last exit reason: $out" ;; esac
case "$out" in *"spawn scheduled"*) ;; *) fail "the failure did not print launchd's state: $out" ;; esac
case "$out" in *"ThrottleInterval 60"*) ;; *) fail "the failure did not name the throttle it waited out: $out" ;; esac
only_reads

# 3. A daemon that is up on the first poll costs one poll and no noise.
export FAKE_HEALTH_AFTER=0
wait_run com.example.fathom-http
[ "$rc" -eq 0 ] && [ "$curls" -eq 1 ] || fail "an already-healthy daemon: rc $rc after $curls polls: $out"
case "$out" in *"OS_REASON_CODESIGNING"*) fail "a first-poll success printed a stale exit reason: $out" ;; esac

# 4. launchd unreadable (no such label): the launchd defaults size the wait
#    (10 throttle + 20 exit timeout + 30 margin = 60) and the failure says
#    launchd could not be read.
export FAKE_HEALTH_AFTER=1000000 FAKE_LAUNCHCTL_RC=113
wait_run com.example.missing
[ "$rc" -ne 0 ] || fail "an unreachable daemon was reported healthy"
[ "$curls" -eq 60 ] || fail "unreadable launchd: expected 60 polls from the defaults, got $curls"
case "$out" in *"could not read launchd"*) ;; *) fail "unreadable launchd was not said: $out" ;; esac
unset FAKE_LAUNCHCTL_RC

# 5. No label (no launchd agent installed): the wait is the plain 30 polls
#    and never asks launchctl.
wait_run
[ "$rc" -ne 0 ] && [ "$curls" -eq 30 ] || fail "no label: rc $rc after $curls polls (want failure after 30): $out"
[ ! -e "$state/launchctl.log" ] || fail "with no label the waiter asked launchctl: $(cat "$state/launchctl.log")"
case "$out" in *"no launchd agent"*) ;; *) fail "no label: the failure did not say there was no launchd agent to ask: $out" ;; esac

# 6. FATHOM_DEPLOY_WAIT_S overrides the computed wait.
FATHOM_DEPLOY_WAIT_S=7 wait_run com.example.fathom-http
[ "$rc" -ne 0 ] && [ "$curls" -eq 7 ] || fail "FATHOM_DEPLOY_WAIT_S=7: rc $rc after $curls polls: $out"

# 7. `task deploy` ends on this waiter, not on the bare curl retry it replaced
#    (30 one-second tries, which a 60 s throttle outlasts), and the waiter is
#    in the gate's script list so this test keeps running.
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"
grep -Fq 'bash scripts/deploy-wait.sh "$label"' "${REPO_ROOT}/Taskfile.yml" \
  || fail "the Taskfile deploy task does not run scripts/deploy-wait.sh"
if grep -Fq 'retry-connrefused' "${REPO_ROOT}/Taskfile.yml"; then
  fail "the Taskfile still carries a fixed curl retry as its health wait"
fi
grep -Fq 'bash scripts/deploy-wait_test.sh' "${REPO_ROOT}/Taskfile.yml" \
  || fail "task scripts-check does not run this test"

echo "deploy-wait test: PASS"
