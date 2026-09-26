#!/usr/bin/env bash
# Build/check fathom's standalone observatory CSS from the in-tree app-kit
# mirror. The explicit Tailwind source closure is part of the contract: a
# missing, unreadable, escaped, or concurrently changed input refuses before
# either committed generated artifact is replaced.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"
# shellcheck source=scripts/lib/build-cache.sh
source "${SCRIPT_DIR}/lib/build-cache.sh"
fathom_cache_configure
MIRROR_ROOT="${REPO_ROOT}/third_party/app-kit"
INPUT="${REPO_ROOT}/cmd/fathom/ui/input.css"
INPUT_DIR="${REPO_ROOT}/cmd/fathom/ui"
TOKENS="${REPO_ROOT}/cmd/fathom/ui/guide-baseline.cue"
THEME_OUT="${REPO_ROOT}/cmd/fathom/ui/tokens/theme.css"
CSS_OUT="${REPO_ROOT}/cmd/fathom/ui/app.css"

MODE="build"
case "${1:-}" in
  "") ;;
  --check) MODE="check" ;;
  --check-sources) MODE="sources" ;;
  *) echo "ui-css: usage: scripts/ui-css.sh [--check|--check-sources]" >&2; exit 2 ;;
esac

SOURCE_DECLARATIONS=(
  "../console_*.go"
  "../observatory.go"
  "../observatory_apply.go"
  "../observatory_brief.go"
  "../observatory_render.go"
  "../observatory_series.go"
  "../observatory_story.go"
  "../../../third_party/app-kit/appkit/ui/appearance/*.go"
  "../../../third_party/app-kit/appkit/ui/callout/*.go"
  "../../../third_party/app-kit/appkit/ui/chrome/*.go"
  "../../../third_party/app-kit/appkit/ui/dashboard/*.go"
  "../../../third_party/app-kit/appkit/ui/form/*.go"
  "../../../third_party/app-kit/appkit/ui/hitl/*.go"
  "../../../third_party/app-kit/appkit/ui/live/*.go"
  "../../../third_party/app-kit/appkit/ui/overlay/*.go"
  "../../../third_party/app-kit/appkit/ui/palette/*.go"
  "../../../third_party/app-kit/appkit/ui/stat/*.go"
  "../../../third_party/app-kit/appkit/ui/table/*.go"
  "../../../third_party/app-kit/appkit/ui/viz/*.go"
  "../../../third_party/app-kit/appkit/ui/vizframe/*.go"
)

render_expected_input() {
  cat <<'EOF'
/* fathom observatory CSS entry (vision 17; app-kit L3b consumption).
   Built by the checksum-pinned Tailwind binary via `task ui:css` — no Node.
   The explicit source list is load-bearing: Tailwind only emits utilities
   it can SEE, so every production renderer and consumed kit package is named. */
@import "tailwindcss" source(none);
@import "./tokens/theme.css";

/* fathom's complete production observatory source set; every console page
   and component lives in a console_*.go file, scanned by glob (vision 35) */
@source "../console_*.go";
@source "../observatory.go";
@source "../observatory_apply.go";
@source "../observatory_brief.go";
@source "../observatory_render.go";
@source "../observatory_series.go";
@source "../observatory_story.go";

/* mirrored kit packages imported by the observatory renderers */
@source "../../../third_party/app-kit/appkit/ui/appearance/*.go";
@source "../../../third_party/app-kit/appkit/ui/callout/*.go";
@source "../../../third_party/app-kit/appkit/ui/chrome/*.go";
@source "../../../third_party/app-kit/appkit/ui/dashboard/*.go";
@source "../../../third_party/app-kit/appkit/ui/form/*.go";
@source "../../../third_party/app-kit/appkit/ui/hitl/*.go";
@source "../../../third_party/app-kit/appkit/ui/live/*.go";
@source "../../../third_party/app-kit/appkit/ui/overlay/*.go";
@source "../../../third_party/app-kit/appkit/ui/palette/*.go";
@source "../../../third_party/app-kit/appkit/ui/stat/*.go";
@source "../../../third_party/app-kit/appkit/ui/table/*.go";
@source "../../../third_party/app-kit/appkit/ui/viz/*.go";
@source "../../../third_party/app-kit/appkit/ui/vizframe/*.go";
EOF
}

fail() {
  echo "ui-css: $*" >&2
  exit 1
}

inside_repo() {
  case "$1" in
    "${REPO_ROOT}"|"${REPO_ROOT}"/*) return 0 ;;
    *) fail "selected source resolves outside repository: $1" ;;
  esac
}

require_file() {
  local path="$1" physical shasum_stderr shasum_status
  [ -f "${path}" ] || fail "required source is missing: ${path#"${REPO_ROOT}/"}"
  [ ! -L "${path}" ] || fail "required source must not be a symlink: ${path#"${REPO_ROOT}/"}"
  physical="$(cd "$(dirname "${path}")" && pwd -P)/$(basename "${path}")"
  inside_repo "${physical}"
  # The content read is authoritative. Access predicates can disagree with a
  # subsequent open on external volumes, while this is the operation the
  # digest and build actually require.
  if shasum_stderr="$(shasum -a 256 "${path}" 2>&1 >/dev/null)"; then
    return
  else
    shasum_status=$?
  fi
  if [ -n "${shasum_stderr}" ]; then
    fail "required source is unreadable: ${path#"${REPO_ROOT}/"} (shasum exit ${shasum_status}: ${shasum_stderr})"
  fi
  fail "required source is unreadable: ${path#"${REPO_ROOT}/"} (shasum exit ${shasum_status})"
}

source_matches() {
  local declaration="$1" pattern matched=0 file
  pattern="${INPUT_DIR}/${declaration}"
  while IFS= read -r file; do
    [ -n "${file}" ] || continue
    matched=1
    require_file "${file}"
    printf '%s\n' "${file}"
  done < <(compgen -G "${pattern}" | LC_ALL=C sort || true)
  [ "${matched}" -eq 1 ] || fail "required source selection is empty: ${declaration}"
}

validate_source_contract() {
  local declaration stamp
  require_file "${INPUT}"
  require_file "${TOKENS}"
  require_file "${MIRROR_ROOT}/.vendor-commit"
  require_file "${MIRROR_ROOT}/go.mod"
  require_file "${MIRROR_ROOT}/go.sum"

  # Exact input, not presence checks: an additive import in a comment, on a
  # shared line, or after the audited list can otherwise reactivate Tailwind's
  # implicit repository scan while every expected directive remains present.
  cmp -s "${INPUT}" <(render_expected_input) \
    || fail "input.css differs from the exact audited import/source closure"

  stamp="$(tr -d '[:space:]' < "${MIRROR_ROOT}/.vendor-commit")"
  [[ "${stamp}" =~ ^[0-9a-f]{40}$ ]] || fail "app-kit mirror provenance stamp is malformed"
  grep -Fxq 'module github.com/geoffmilleraz/app-kit' "${MIRROR_ROOT}/go.mod" \
    || fail "app-kit mirror has the wrong module identity"
  if grep -Eq '^[[:space:]]*replace([[:space:]]|\()' "${MIRROR_ROOT}/go.mod"; then
    fail "app-kit mirror go.mod contains a replace directive; refusing external module resolution"
  fi
  if find "${MIRROR_ROOT}" -type l -print -quit | grep -q .; then
    fail "app-kit mirror contains a symlink; refusing an external compile dependency"
  fi

  for declaration in "${SOURCE_DECLARATIONS[@]}"; do
    source_matches "${declaration}" >/dev/null
  done
}

selected_files() {
  local declaration
  printf '%s\n' "${INPUT}"
  printf '%s\n' "${TOKENS}"
  # The CLI's transitive compile/embed closure is broader than cmd/appkit,
  # internal/tailwind, and ui/tokens. Hash the complete, small (about 3 MB)
  # in-tree mirror so no compiled or embedded input is silently omitted.
  find "${MIRROR_ROOT}" -type f -print
  for declaration in "${SOURCE_DECLARATIONS[@]}"; do
    source_matches "${declaration}"
  done
}

source_digest() {
  local file
  selected_files | LC_ALL=C sort -u | while IFS= read -r file; do
    require_file "${file}"
    printf '%s  %s\n' "$(shasum -a 256 "${file}" | awk '{print $1}')" "${file#"${REPO_ROOT}/"}"
  done | shasum -a 256 | awk '{print $1}'
}

stage_sources() {
  local stage_root="$1" declaration source relative destination
  mkdir -p "${stage_root}/cmd/fathom/ui/tokens"
  cp "${INPUT}" "${stage_root}/cmd/fathom/ui/input.css"
  for declaration in "${SOURCE_DECLARATIONS[@]}"; do
    while IFS= read -r source; do
      relative="${source#"${REPO_ROOT}/"}"
      destination="${stage_root}/${relative}"
      mkdir -p "$(dirname "${destination}")"
      cp "${source}" "${destination}"
    done < <(source_matches "${declaration}")
  done
}

publish() {
  local source="$1" destination="$2" adjacent
  adjacent="${destination}.ui-css.$$"
  cp "${source}" "${adjacent}"
  chmod 0644 "${adjacent}"
  mv -f "${adjacent}" "${destination}"
}

validate_source_contract
[ "${MODE}" = "sources" ] && { echo "ui-css: source closure valid"; exit 0; }

mkdir -p "${REPO_ROOT}/workspace/tmp"
LOCK="${REPO_ROOT}/workspace/tmp/ui-css.lock"
mkdir "${LOCK}" 2>/dev/null || fail "another ui-css build is active (${LOCK#"${REPO_ROOT}/"})"
WORK=""
cleanup() {
  [ -z "${WORK}" ] || rm -rf "${WORK}"
  rmdir "${LOCK}" 2>/dev/null || true
}
trap cleanup EXIT
WORK="$(mktemp -d "${REPO_ROOT}/workspace/tmp/ui-css.XXXXXX")"
APPKIT_BIN="${WORK}/appkit"
CANDIDATE_THEME="${WORK}/theme.css"
CANDIDATE_CSS="${WORK}/app.css"
STAGE_ROOT="${WORK}/stage"
BEFORE="$(source_digest)"

# Build in the mirrored module so the root module cannot hide an undeclared
# app-kit dependency. GOWORK=off forbids a sibling workspace from participating.
(
  cd "${MIRROR_ROOT}"
  GOWORK=off go build -o "${APPKIT_BIN}" ./cmd/appkit
)

# Token generation runs from fathom's root and reads its app-owned CUE source.
# Empty outputs are explicit; the kit's terminal palette belongs to the mirror.
(
  cd "${REPO_ROOT}"
  "${APPKIT_BIN}" gen \
    -tokens "${TOKENS}" \
    -css "${CANDIDATE_THEME}" -dtcg "" -go "" -terminal ""
)

stage_sources "${STAGE_ROOT}"
cp "${CANDIDATE_THEME}" "${STAGE_ROOT}/cmd/fathom/ui/tokens/theme.css"
"${APPKIT_BIN}" css \
  -i "${STAGE_ROOT}/cmd/fathom/ui/input.css" \
  -o "${CANDIDATE_CSS}"

AFTER="$(source_digest)"
[ "${BEFORE}" = "${AFTER}" ] || fail "selected sources changed during generation; refusing stale output"

if [ "${MODE}" = "check" ]; then
  require_file "${THEME_OUT}"
  require_file "${CSS_OUT}"
  cmp -s "${CANDIDATE_THEME}" "${THEME_OUT}" || fail "generated token theme is stale; run: task ui:css"
  cmp -s "${CANDIDATE_CSS}" "${CSS_OUT}" || fail "generated app.css is stale; run: task ui:css"
  echo "ui-css: generated CSS is current"
  exit 0
fi

# The lock prevents two builders from interleaving. Each replacement is
# atomic, but the pair cannot be one filesystem transaction because the theme
# lives in a child directory; an interruption between moves is deliberately
# detected by `--check` rather than described as pair-atomic.
publish "${CANDIDATE_THEME}" "${THEME_OUT}"
publish "${CANDIDATE_CSS}" "${CSS_OUT}"
echo "ui-css: wrote cmd/fathom/ui/tokens/theme.css and cmd/fathom/ui/app.css"
