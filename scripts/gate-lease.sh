#!/usr/bin/env bash
# gate-lease.sh — run a gate command under the machine-wide gate lease
# (docs/design/responsible-tenant.md §12.3, RT-129).
#
#   bash scripts/gate-lease.sh <command> [args...]
#
# Concurrent `task validate` / pre-push runs on one machine — several
# repositories, several agents — queue on one lease instead of running
# their race-enabled test suites side by side. The lease is an flock on
# ${FATHOM_GATE_LOCK:-~/.fathom/locks/gate.lock}: the kernel drops it when
# its holder dies, so there is no stale lease to break. A waiter says who
# holds it ("waiting for gate lease held by pid N (repo X)").
#
# The command runs niced (`nice -n 10`): it yields the CPU to interactive
# work but is not clamped. FATHOM_GATE_NICE=off runs it at normal priority;
# FATHOM_GATE_NICE=background runs it under macOS's `taskpolicy -b` instead.
# Not the default: background QoS confines the gate to the efficiency cores
# and throttles its I/O, and measured 2026-09-27 (M-series, two agents
# busy) `go test -race -p=1 ./cmd/fathom` took 441 s under it against 174 s
# at normal priority and 168 s under `nice -n 10`; every package slowed
# ~3x alike (fusion 74 s -> 438 s in the gate), pushing cmd/fathom past
# go test's 10 min default timeout.
#
# macOS ships no flock(1); perl's flock(2) is the lock (perl is on every
# macOS and nearly every Linux). The perl process holds the lock and waits
# for the command, so the lease lasts exactly as long as the command.

set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: gate-lease.sh <command> [args...]" >&2
  exit 2
fi

lock="${FATHOM_GATE_LOCK:-${HOME}/.fathom/locks/gate.lock}"
mkdir -p "$(dirname "$lock")"
repo="$(git rev-parse --show-toplevel 2>/dev/null || pwd -P)"

prio=()
case "${FATHOM_GATE_NICE:-on}" in
  off) ;;
  background)
    if command -v taskpolicy >/dev/null 2>&1; then
      prio=(taskpolicy -b)
    elif command -v nice >/dev/null 2>&1; then
      prio=(nice -n 10)
    fi
    ;;
  *)
    if command -v nice >/dev/null 2>&1; then
      prio=(nice -n 10)
    fi
    ;;
esac

command -v perl >/dev/null 2>&1 || { echo "gate-lease: perl is required for the gate lease (flock)" >&2; exit 2; }

exec perl -MFcntl=:flock -e '
  my ($lock, $repo, @cmd) = @ARGV;
  open(my $fh, ">>", $lock) or die "gate-lease: cannot open $lock: $!\n";
  unless (flock($fh, LOCK_EX | LOCK_NB)) {
    my $holder = "an unknown holder";
    if (open(my $r, "<", "$lock.holder")) { local $/; $holder = <$r> // $holder; chomp $holder; close $r }
    print STDERR "gate-lease: waiting for gate lease held by $holder\n";
    flock($fh, LOCK_EX) or die "gate-lease: flock $lock: $!\n";
  }
  if (open(my $w, ">", "$lock.holder.$$")) { print $w "pid $$ (repo $repo)\n"; close $w; rename("$lock.holder.$$", "$lock.holder") }
  my $rc = system(@cmd);
  exit 127 if $rc == -1;
  exit(128 + ($rc & 127)) if $rc & 127;
  exit($rc >> 8);
' "$lock" "$repo" "${prio[@]+"${prio[@]}"}" "$@"
