#!/usr/bin/env bash
#
# vendor-check_test.sh — shell-level test for scripts/vendor-check.sh (and,
# incidentally, scripts/sync-vendor.sh — they share scripts/lib/kit-pin.sh).
#
# Builds a throwaway sibling git repo + a throwaway "consumer" repo (with its
# own go.mod and third_party/agentic-kit, laid out exactly like the real repo
# so ../agentic-kit resolves correctly) under a temp dir. Never touches the
# real ../agentic-kit checkout or this repo's third_party/agentic-kit.
#
# Cases:
#   1. freshly synced third_party/ matches -> vendor-check passes.
#   2. a corrupted file in third_party/     -> vendor-check fails.
#   3. sibling checkout absent               -> vendor-check skips (exit 0).
#   4. sibling checkout present but lacks
#      the pinned commit                     -> vendor-check skips (exit 0).
#
# Usage:
#   bash scripts/vendor-check_test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

pass=0
check() {
  local desc="$1" got="$2" want="$3"
  if [ "${got}" != "${want}" ]; then
    fail "${desc}: expected exit ${want}, got ${got}"
  fi
  pass=$((pass + 1))
  echo "ok — ${desc}"
}

TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

# ─── build a throwaway sibling "agentic-kit" repo ──────────────────────────
SIBLING="${TMP}/agentic-kit"
mkdir -p "${SIBLING}/foundations"
(
  cd "${SIBLING}"
  git init -q
  git config user.email test@example.com
  git config user.name test
  echo "package foundations" > foundations/x.go
  echo "# agentic-kit fixture" > README.md
  git add -A
  git commit -q -m "fixture commit"
)
COMMIT=$(git -C "${SIBLING}" rev-parse HEAD)
SHORT=${COMMIT:0:12}

# ─── build a throwaway "consumer" repo (plays the role of fathom) ──────────
CONSUMER="${TMP}/consumer"
mkdir -p "${CONSUMER}/third_party"
cat > "${CONSUMER}/go.mod" <<EOF
module example.com/consumer

go 1.26

require github.com/geoffmilleraz/agentic-kit v0.0.0-0.20260717061147-${SHORT}
EOF

# Install the real scripts under test + their shared lib into the consumer
# so relative paths (scripts/lib/kit-pin.sh, third_party/agentic-kit,
# ../agentic-kit) resolve exactly like they do in the real repo.
mkdir -p "${CONSUMER}/scripts/lib"
cp "${SCRIPT_DIR}/lib/kit-pin.sh" "${CONSUMER}/scripts/lib/kit-pin.sh"
cp "${SCRIPT_DIR}/sync-vendor.sh" "${CONSUMER}/scripts/sync-vendor.sh"
cp "${SCRIPT_DIR}/vendor-check.sh" "${CONSUMER}/scripts/vendor-check.sh"

run_vendor_check() {
  ( cd "${CONSUMER}" && bash scripts/vendor-check.sh > "${TMP}/out.log" 2>&1 )
}

# ─── case 1: no third_party/ yet -> fail with "run: task sync-kit" ─────────
set +e
run_vendor_check
rc=$?
set -e
check "missing third_party/agentic-kit fails" "${rc}" "1"
grep -q "task sync-kit" "${TMP}/out.log" || fail "missing-dir message should mention task sync-kit"

# ─── sync via the real sync-kit.sh, then case: fresh sync matches ──────────
( cd "${CONSUMER}" && bash scripts/sync-vendor.sh agentic-kit > "${TMP}/sync.log" 2>&1 )
[ -f "${CONSUMER}/third_party/agentic-kit/foundations/x.go" ] || fail "sync-kit did not populate third_party/agentic-kit"

set +e
run_vendor_check
rc=$?
set -e
check "freshly synced third_party/agentic-kit passes" "${rc}" "0"

# ─── case 2: corrupt a synced file -> vendor-check fails ───────────────────
echo "package foundations // tampered" > "${CONSUMER}/third_party/agentic-kit/foundations/x.go"
set +e
run_vendor_check
rc=$?
set -e
check "tampered third_party/agentic-kit fails" "${rc}" "1"
grep -q "task sync-kit" "${TMP}/out.log" || fail "drift message should mention task sync-kit"

# restore before the remaining cases
( cd "${CONSUMER}" && bash scripts/sync-vendor.sh agentic-kit > /dev/null 2>&1 )

# ─── case 3: sibling absent -> skip (exit 0) ────────────────────────────────
mv "${SIBLING}" "${SIBLING}.hidden"
set +e
run_vendor_check
rc=$?
set -e
check "absent sibling skips gracefully" "${rc}" "0"
grep -qi "skipping" "${TMP}/out.log" || fail "absent-sibling run should say it's skipping"
mv "${SIBLING}.hidden" "${SIBLING}"

# ─── case 4: sibling present but lacks the pinned commit -> skip (exit 0) ──
( cd "${SIBLING}" && git commit -q --allow-empty -m "unrelated later commit" )
# Repoint the pin at a commit the sibling has never seen.
cat > "${CONSUMER}/go.mod" <<EOF
module example.com/consumer

go 1.26

require github.com/geoffmilleraz/agentic-kit v0.0.0-0.20260717061147-deadbeef0000
EOF
set +e
run_vendor_check
rc=$?
set -e
check "sibling missing the pinned commit skips gracefully" "${rc}" "0"
grep -qi "skipping" "${TMP}/out.log" || fail "missing-commit run should say it's skipping"

echo ""
echo "vendor-check_test.sh: ${pass}/${pass} checks passed"
