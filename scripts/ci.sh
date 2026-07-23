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

# ─── fathom verify (fail-closed, §5.3 docs/design/coding-agent-excellence.md) ─
# Build bin/fathom, ingest the repo into a temp store, then for each
# docs/plans/*.md run fathom verify --strict. Skip cleanly if no plan files.
#
# Scope (E4-2, docs/plans/new-use-cases.md): ACTIVE plans only. -maxdepth 1
# never descends into docs/plans/archive/, so an archived plan (via `fathom
# plans archive`) costs one fewer `fathom verify` invocation on every CI run,
# with no separate exclusion needed.
VERIFY_PASS=1
FATHOM_PLANS=()
while IFS= read -r -d $'\0' f; do
    FATHOM_PLANS+=("$f")
done < <(find docs/plans -maxdepth 1 -name '*.md' ! -name 'README.md' -print0 2>/dev/null || true)

if [ ${#FATHOM_PLANS[@]} -gt 0 ]; then
    echo ""
    echo "step: fathom verify (${#FATHOM_PLANS[@]} plan file(s))"
    echo "  building bin/fathom …"
    CGO_ENABLED=${CGO_ENABLED} MACOSX_DEPLOYMENT_TARGET=14.0 \
        go build -o bin/fathom ./cmd/fathom

    FATHOM_TMPDIR=$(mktemp -d)
    FATHOM_TMPSTORE="${FATHOM_TMPDIR}/verify.sqlite"
    echo "  ingesting into ${FATHOM_TMPSTORE} …"
    bin/fathom ingest --service fathom --path . --store "${FATHOM_TMPSTORE}"

    for f in "${FATHOM_PLANS[@]}"; do
        echo "  verifying ${f} …"
        if ! bin/fathom verify --strict \
                --store "${FATHOM_TMPSTORE}" \
                --service fathom \
                --plan "${f}"; then
            VERIFY_PASS=0
        fi
    done

    rm -rf "${FATHOM_TMPDIR}"
else
    echo ""
    echo "step: fathom verify — no docs/plans/*.md, skipping"
fi

if [ "${PASS}" -eq 1 ] && [ "${VERIFY_PASS}" -eq 1 ]; then
    print_summary ok
    exit 0
else
    print_summary fail
    exit 1
fi
