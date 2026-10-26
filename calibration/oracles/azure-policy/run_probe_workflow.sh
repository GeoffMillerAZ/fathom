#!/usr/bin/env bash
# T-R11: run only on the disposable GitHub-hosted Ubuntu 22.04 x64 job.
set -euo pipefail
# Parse the whole launcher before any vendor process can start.
main() {
  cd "${GITHUB_WORKSPACE:?}"
  private="${RUNNER_TEMP:?}/azure-policy-private"
  public="${RUNNER_TEMP:?}/azure-policy-public"
  readonly private public
  mkdir -m 700 "$private" "$public"
  trap 'rm -rf -- "$private"' EXIT
  # Never stream a native response, raw exception or private log into job logs.
  sha256sum -c calibration/oracles/azure-policy/probe-linux-checksums.sha256 >"$private/checksums.log" 2>&1
  python3 -B -m unittest discover -s calibration/oracles/azure-policy -p 'test_*.py' >"$private/tests.log" 2>&1
  python3 -B calibration/oracles/azure-policy/probe_filesystem_controls.py -out "$private/controls" >"$private/controls.log" 2>&1
  set +e
  nice -n 10 python3 -B calibration/oracles/azure-policy/probe_linux.py \
    -cache "$private/cache" -out "$private/out" -publish "$public/receipt.json" \
    -min-free-gib 1 -max-load 6 >"$private/launcher.log" 2>&1
  probe_code=$?
  set -e
  case "$probe_code" in
    0|2) test -f "$public/receipt.json" ;;
    *) exit 1 ;;
  esac
  # The outer launcher validates and writes this separate file only after the
  # complete PID namespace has exited. This step never copies private output.
  printf '%s\n' 'validated=true' >>"${GITHUB_OUTPUT:?}"
}
main "$@"
exit
