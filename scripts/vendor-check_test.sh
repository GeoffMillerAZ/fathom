#!/usr/bin/env bash
#
# vendor-check_test.sh — shell-level test for scripts/vendor-check.sh and
# scripts/sync-vendor.sh (the .vendor-commit pin and the ADR-070 rewrite).
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
#   5. an unrewritten (upstream-form) copy   -> vendor-check fails.
#   6. a closure mirror (<name>.keep)         -> only the manifest's paths are
#      compared; a stray file or a path the pin lacks fails.
#
# sync-vendor rewrites the copy into the consumer module (go.mod kept as
# go.mod.upstream, imports under <consumer>/third_party/agentic-kit), and
# vendor-check reverses that before comparing — both asserted below.
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
mkdir -p "${SIBLING}/foundations" "${SIBLING}/cmd/tool"
(
  cd "${SIBLING}"
  git init -q
  git config user.email test@example.com
  git config user.name test
  printf 'module github.com/geoffmilleraz/agentic-kit\n\ngo 1.26\n' > go.mod
  echo "package foundations" > foundations/x.go
  printf 'package main\n\nimport _ "github.com/geoffmilleraz/agentic-kit/foundations"\n\nfunc main() {}\n' > cmd/tool/main.go
  echo "# agentic-kit fixture" > README.md
  git add -A
  git commit -q -m "fixture commit"
)
COMMIT=$(git -C "${SIBLING}" rev-parse HEAD)

# The rewriter both scripts run, built once from this repository.
KITREWRITE="${TMP}/kitrewrite"
( cd "${REPO_ROOT}" && GOWORK=off go build -o "${KITREWRITE}" ./cmd/kitrewrite )
export KITREWRITE

# ─── build a throwaway "consumer" repo (plays the role of fathom) ──────────
CONSUMER="${TMP}/consumer"
mkdir -p "${CONSUMER}/third_party"
cat > "${CONSUMER}/go.mod" <<EOF
module example.com/consumer

go 1.26
EOF

# Install the real scripts under test into the consumer so relative paths
# (third_party/agentic-kit, ../agentic-kit) resolve exactly like they do in
# the real repo.
mkdir -p "${CONSUMER}/scripts"
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
[ "$(cat "${CONSUMER}/third_party/agentic-kit/.vendor-commit")" = "${COMMIT}" ] || fail "sync-kit did not stamp the sibling HEAD into .vendor-commit"
[ -f "${CONSUMER}/third_party/agentic-kit/go.mod.upstream" ] || fail "sync-kit did not keep the mirror's go.mod as go.mod.upstream"
[ ! -e "${CONSUMER}/third_party/agentic-kit/go.mod" ] || fail "sync-kit left the mirror a nested module (go.mod)"
grep -q '"example.com/consumer/third_party/agentic-kit/foundations"' "${CONSUMER}/third_party/agentic-kit/cmd/tool/main.go" \
  || fail "sync-kit did not rewrite the kit's own import into the consumer module"

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
# Repoint the pin (the mirror's .vendor-commit) at a commit the sibling has never seen.
printf 'deadbeef00000000000000000000000000000000\n' > "${CONSUMER}/third_party/agentic-kit/.vendor-commit"
set +e
run_vendor_check
rc=$?
set -e
check "sibling missing the pinned commit skips gracefully" "${rc}" "0"
grep -qi "skipping" "${TMP}/out.log" || fail "missing-commit run should say it's skipping"

# ─── case 5: an upstream-form copy (no rewrite) -> vendor-check fails ───────
rm -rf "${CONSUMER}/third_party/agentic-kit"
mkdir -p "${CONSUMER}/third_party/agentic-kit"
git -C "${SIBLING}" archive "${COMMIT}" | tar -x -C "${CONSUMER}/third_party/agentic-kit"
printf '%s\n' "${COMMIT}" > "${CONSUMER}/third_party/agentic-kit/.vendor-commit"
set +e
run_vendor_check
rc=$?
set -e
check "unrewritten upstream-form copy fails" "${rc}" "1"
grep -q "go.mod" "${TMP}/out.log" || fail "upstream-form message should name the mirror's go.mod"

# ─── case 6: a closure mirror (third_party/<name>.keep, DS-4) ───────────────
# vendor-check compares only the manifest's paths, in upstream form (the
# go.mod.upstream rename undone); README.md is upstream but not kept.
( cd "${CONSUMER}" && bash scripts/sync-vendor.sh agentic-kit > /dev/null 2>&1 )
KEEP="${CONSUMER}/third_party/agentic-kit.keep"
( cd "${CONSUMER}" && find third_party/agentic-kit -type f ! -name README.md | LC_ALL=C sort ) > "${KEEP}"
rm "${CONSUMER}/third_party/agentic-kit/README.md"
set +e
run_vendor_check
rc=$?
set -e
check "a mirror holding exactly its manifest's paths passes" "${rc}" "0"
grep -q "agentic-kit.keep" "${TMP}/out.log" || fail "a manifest compare should name the manifest"

echo "stray" > "${CONSUMER}/third_party/agentic-kit/stray.txt"
set +e
run_vendor_check
rc=$?
set -e
check "a file outside the manifest fails" "${rc}" "1"
rm "${CONSUMER}/third_party/agentic-kit/stray.txt"

printf 'third_party/agentic-kit/not-upstream.txt\n' >> "${KEEP}"
set +e
run_vendor_check
rc=$?
set -e
check "a manifest path the pinned commit lacks fails" "${rc}" "1"
grep -q "not-upstream.txt" "${TMP}/out.log" || fail "the missing manifest path should be named"

echo ""
echo "vendor-check_test.sh: ${pass}/${pass} checks passed"
