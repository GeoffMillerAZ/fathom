#!/usr/bin/env bash
# deploy-wait.sh [LABEL] — the health wait `task deploy` ends on (EW-17, O129).
#
# After the kickstart the new daemon can be minutes from serving: launchd
# holds a respawn back by the job's ThrottleInterval (60 s for the installed
# agent) after a spawn that failed, and the old process may take its exit
# timeout to drain first. The wait it replaced gave up after 30 one-second
# tries, so two deploys of 2026-09-29 exited 201 while the daemon came up 61 s
# later. This waits throttle + exit timeout + a 30 s startup margin, read
# from launchd (`launchctl print` says "minimum runtime" for ThrottleInterval
# and "exit timeout"), one health poll per nominal second; and when the wait
# runs out it prints launchd's last exit reason and job state instead of a bare
# timeout, so a spawn that failed (OS_REASON_CODESIGNING in O129) is named.
#
# Read-only against launchd: only `launchctl print`, never a kickstart or a
# bootout (a wedged job is the owner's to restart).
#
# LABEL is the launchd agent's label ("" = none installed: a plain 30 polls).
# Env: FATHOM_HEALTH_URL (default http://localhost:7575/healthz),
# FATHOM_DEPLOY_WAIT_S (override the number of polls), FATHOM_DEPLOY_POLL_S
# (seconds between polls, default 1; tests use 0).
#
# Exit: 0 healthy, 1 not healthy in time.

set -u

label="${1:-}"
url="${FATHOM_HEALTH_URL:-http://localhost:7575/healthz}"
poll="${FATHOM_DEPLOY_POLL_S:-1}"

# launchd's own defaults, used when the job cannot be read.
throttle=10
exit_timeout=20
margin=30

# job_field NAME TEXT: the value of the first "NAME = value" line in TEXT
# (the job's own line, which `launchctl print` prints before any nested one).
job_field() {
  printf '%s\n' "$2" | sed -n "s/^[[:space:]]*$1 = //p" | head -n 1
}

launchd_text=""
launchd_read=0
if [ -n "$label" ]; then
  if launchd_text="$(launchctl print "gui/$(id -u)/$label" 2>&1)"; then
    launchd_read=1
    t="$(job_field 'minimum runtime' "$launchd_text")"
    e="$(job_field 'exit timeout' "$launchd_text")"
    case "$t" in ''|*[!0-9]*) ;; *) throttle="$t" ;; esac
    case "$e" in ''|*[!0-9]*) ;; *) exit_timeout="$e" ;; esac
  else
    echo "deploy: could not read launchd for $label (${launchd_text:-no output}); waiting for the launchd defaults" >&2
  fi
fi

if [ -z "$label" ]; then
  tries=30
  basis="no launchd agent to consult"
else
  tries=$((throttle + exit_timeout + margin))
  basis="launchd ThrottleInterval ${throttle} s + exit timeout ${exit_timeout} s + ${margin} s startup margin"
  [ "$launchd_read" -eq 1 ] || basis="launchd unreadable, so its defaults: ThrottleInterval ${throttle} s + exit timeout ${exit_timeout} s + ${margin} s startup margin"
fi
case "${FATHOM_DEPLOY_WAIT_S:-}" in
  ''|*[!0-9]*) ;;
  *) tries="$FATHOM_DEPLOY_WAIT_S"; basis="FATHOM_DEPLOY_WAIT_S" ;;
esac

# launchd_report: the facts launchd holds about the job, one line.
launchd_report() {
  local text state reason runs
  if [ -z "$label" ]; then
    echo "  no launchd agent is installed to ask why (run: fathom daemon install)"
    return 0
  fi
  if ! text="$(launchctl print "gui/$(id -u)/$label" 2>&1)"; then
    echo "  could not read launchd for $label: ${text:-no output}"
    return 0
  fi
  state="$(job_field 'state' "$text")"
  reason="$(job_field 'last exit reason' "$text")"
  runs="$(job_field 'runs' "$text")"
  echo "  launchd $label: state = ${state:-unknown}; runs = ${runs:-unknown}; last exit reason = ${reason:-none recorded}"
}

n=0
while [ "$n" -lt "$tries" ]; do
  n=$((n + 1))
  if curl -s --connect-timeout 2 --max-time 5 "$url" 2>/dev/null | grep -q ok; then
    echo "deploy: healthz ok after $n poll(s)"
    if [ "$n" -gt 5 ]; then
      echo "deploy: the daemon took $n polls to come up (a first spawn that failed waits out launchd's throttle); launchd says:"
      launchd_report
    fi
    exit 0
  fi
  if [ $((n % 20)) -eq 0 ] && [ "$n" -lt "$tries" ]; then
    echo "deploy: still waiting for healthz ($n of $tries polls)" >&2
  fi
  [ "$n" -lt "$tries" ] && sleep "$poll"
done

echo "deploy FAILED: healthz at $url did not answer in $tries polls ($basis). The install itself is done; launchd says:" >&2
launchd_report >&2
exit 1
