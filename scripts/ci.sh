#!/usr/bin/env bash
# ci.sh — fathom CI helper
#
# Runs the full validation gate (task validate) and prints a compact summary.
# Exits 0 on success, 1 on failure.
#
# Usage:
#   ./scripts/ci.sh
#   TASK_FLAGS=--verbose ./scripts/ci.sh
#
# Environment:
#   TASK_FLAGS   Extra flags forwarded to the `task` invocation (default: empty).
#   CGO_ENABLED  Defaults to 1 (required by the DuckDB driver).

set -euo pipefail

# ─── defaults ─────────────────────────────────────────────────────────────────

: "${CGO_ENABLED:=1}"
export CGO_ENABLED

TASK_FLAGS="${TASK_FLAGS:-}"

# ─── helpers ──────────────────────────────────────────────────────────────────

print_header() {
    echo "======================================================================"
    echo "  fathom CI — $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    echo "======================================================================"
}

print_summary() {
    local status="$1"
    echo ""
    echo "======================================================================"
    if [ "$status" = "ok" ]; then
        echo "  RESULT: PASS"
    else
        echo "  RESULT: FAIL"
    fi
    echo "  elapsed: ${SECONDS}s"
    echo "======================================================================"
}

require_task() {
    if ! command -v task >/dev/null 2>&1; then
        echo "error: 'task' (Task runner) is not installed or not in PATH." >&2
        echo "       Install it: https://taskfile.dev/installation/" >&2
        exit 1
    fi
}

# ─── main ─────────────────────────────────────────────────────────────────────

print_header
require_task

# Move to the repository root (the directory containing Taskfile.yml),
# which is the parent of the scripts/ directory.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

echo "repo:     ${REPO_ROOT}"
echo "CGO:      ${CGO_ENABLED}"
echo "task:     $(task --version 2>/dev/null || echo unknown)"
echo ""

PASS=0
# Run the full gate; capture exit code without letting set -e abort the script.
# shellcheck disable=SC2086
if task ${TASK_FLAGS} validate; then
    PASS=1
fi

if [ "${PASS}" -eq 1 ]; then
    print_summary ok
    exit 0
else
    print_summary fail
    exit 1
fi
