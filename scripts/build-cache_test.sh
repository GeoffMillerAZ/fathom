#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"
# shellcheck source=scripts/lib/build-cache.sh
source "${SCRIPT_DIR}/lib/build-cache.sh"

fail() {
  echo "build-cache test: $*" >&2
  exit 1
}

assert_eq() {
  local want="$1" got="$2" label="$3"
  [ "$got" = "$want" ] || fail "${label}: got '$got', want '$want'"
}

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-build-cache-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

# With no FATHOM_CACHE_ROOT, fathom must preserve the caller's ordinary Go
# environment. Other machines should not gain a new cache tree merely by
# cloning the repository.
(
  unset FATHOM_CACHE_ROOT FATHOM_CACHE_MAX_MIB FATHOM_CACHE_CONFIGURED
  export GOCACHE="${scratch}/caller-go-cache"
  fathom_cache_configure
  assert_eq "${scratch}/caller-go-cache" "$GOCACHE" "unset root preserves GOCACHE"
)

# A configured removable/external root must already exist. This prevents a
# missing mount from being recreated as an ordinary directory on the internal
# disk and silently defeating the operator's placement decision.
if (
  export FATHOM_CACHE_ROOT="${scratch}/missing"
  unset FATHOM_CACHE_CONFIGURED
  fathom_cache_configure
) 2>/dev/null; then
  fail "missing configured root was accepted"
fi
[ ! -e "${scratch}/missing" ] || fail "missing configured root was created"

if (
  export FATHOM_CACHE_ROOT="relative/cache"
  unset FATHOM_CACHE_CONFIGURED
  fathom_cache_configure
) 2>/dev/null; then
  fail "relative configured root was accepted"
fi

root="${scratch}/cache root"
mkdir -p "$root"
(
  export FATHOM_CACHE_ROOT="$root" FATHOM_CACHE_MAX_MIB=0
  unset FATHOM_CACHE_CONFIGURED
  fathom_cache_configure
  assert_eq "$root/build" "$GOCACHE" "GOCACHE"
  assert_eq "$root/mod" "$GOMODCACHE" "GOMODCACHE"
  assert_eq "$root/tmp" "$GOTMPDIR" "GOTMPDIR"
  assert_eq "$root/tmp" "$TMPDIR" "TMPDIR"
  assert_eq "$root/uv" "$UV_CACHE_DIR" "UV_CACHE_DIR"
  for dir in "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR" "$UV_CACHE_DIR"; do
    [ -d "$dir" ] || fail "cache subdirectory was not created: $dir"
  done
)

# The size guard owns only Fathom's dedicated Go build subdirectory. A fake
# du/go pair makes the threshold and exact cleanup command deterministic.
fake_bin="${scratch}/fake-bin"
mkdir -p "$fake_bin"
cat >"${fake_bin}/du" <<'EOF'
#!/usr/bin/env bash
echo "9217 $2"
EOF
cat >"${fake_bin}/go" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_GO_LOG"
EOF
chmod +x "${fake_bin}/du" "${fake_bin}/go"

go_log="${scratch}/go.log"
(
  export PATH="${fake_bin}:$PATH"
  export FAKE_GO_LOG="$go_log"
  export FATHOM_CACHE_ROOT="$root" FATHOM_CACHE_MAX_MIB=8
  unset FATHOM_CACHE_CONFIGURED
  fathom_cache_configure 2>/dev/null
)
assert_eq "clean -cache" "$(cat "$go_log")" "over-limit cleanup"

: >"$go_log"
(
  export PATH="${fake_bin}:$PATH"
  export FAKE_GO_LOG="$go_log"
  export FATHOM_CACHE_ROOT="$root" FATHOM_CACHE_MAX_MIB=0
  unset FATHOM_CACHE_CONFIGURED
  fathom_cache_configure
)
[ ! -s "$go_log" ] || fail "disabled size guard invoked go clean"

grep -Fq 'GO: bash scripts/fathom-go.sh' "${REPO_ROOT}/Taskfile.yml" \
  || fail "Taskfile GO commands do not use the cache wrapper"
if grep -Fq '/Volumes/' "${REPO_ROOT}/Taskfile.yml"; then
  fail "Taskfile contains a machine-specific volume path"
fi
for file in \
  module-cold-run.sh ci.sh clean-clone-check.sh stranger-check.sh \
  bench-ledger-render.sh ui-css.sh context-bench.sh; do
  grep -Fq 'fathom_cache_configure' "${SCRIPT_DIR}/${file}" \
    || fail "${file} bypasses the build-cache policy"
done

echo "build-cache test: PASS"
