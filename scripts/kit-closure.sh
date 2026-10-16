#!/usr/bin/env bash
#
# kit-closure.sh — the app-kit mirror holds only fathom's build closure
# (ADR-070, docs/plans/distribution.md DS-4). Computes the keep list, never
# hand-lists it, and writes it to the committed manifest third_party/app-kit.keep
# (one repository-relative path per line, byte-sorted). Since ADR-070 the
# mirror is packages of fathom's module, so whatever it holds ships in every
# module zip the Go proxy caches; the closure keeps the kit's design-asset
# data, docs, spikes and other apps out of it.
#
# The closure, from the tree as it stands:
#   (a) every app-kit package in `go list -deps -test` of fathom's own
#       packages (third_party/ excluded from the roots);
#   (b) every app-kit package in `go list -deps` of the mirror's cmd/appkit,
#       which scripts/ui-css.sh builds to regenerate the observatory CSS;
#       (a) and (b) are taken for five fixed GOOS/GOARCH pairs, and each
#       package contributes its non-test Go files (constraint-ignored ones
#       too, so no platform loses a file), its other compiled files and its
#       go:embed targets;
#   (c) every mirror path a non-Go file outside third_party/ names (docs,
#       scripts, appkit.adoption.json, scripts/ui-css.sh's closed Tailwind
#       source set): a file keeps itself, a glob keeps what it matches, a
#       directory must already hold a kept file; a name the mirror does not
#       hold fails, so a new reference to a pruned path cannot pass unseen;
#   (d) the copy's provenance and identity files and fathom-pack/.
#
# Usage:
#   bash scripts/kit-closure.sh            # print the computed closure
#   bash scripts/kit-closure.sh --write    # write third_party/app-kit.keep
#   bash scripts/kit-closure.sh --check    # exit 1 when the manifest differs from the closure,
#                                          # or the mirror holds a file outside it / lacks one
#   bash scripts/kit-closure.sh --prune    # delete mirror files outside the manifest (sync-kit)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"
cd "${REPO_ROOT}"

MIRROR="third_party/app-kit"
MANIFEST="third_party/app-kit.keep"
# Names the repository's prose cites that ADR-070 renamed (the copy's
# go.mod/go.sum are kept as *.upstream); they name history, not a file.
RENAMED=("${MIRROR}/go.mod" "${MIRROR}/go.sum")
ROOT_FILES=(AGENTS.md CLAUDE.md LICENSE.md README.md .vendor-commit go.mod.upstream go.sum.upstream)
# Fixed platforms, so the closure is the same on every host: no platform's
# build may lose a package.
PLATFORMS=(darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64)

fail() {
  echo "kit-closure: $*" >&2
  exit 1
}

mode="print"
case "${1:-}" in
  "") ;;
  --write) mode="write" ;;
  --check) mode="check" ;;
  --prune) mode="prune" ;;
  *) echo "kit-closure: usage: kit-closure.sh [--write|--check|--prune]" >&2; exit 2 ;;
esac

[ -d "${MIRROR}" ] || fail "${MIRROR} does not exist — run: task sync-kit"
# Scratch lives under the gitignored workspace/ (AGENTS.md), so the reference
# scan below never reads it.
mkdir -p workspace/tmp
WORK="$(mktemp -d workspace/tmp/kit-closure.XXXXXX)"
trap 'rm -rf "${WORK}"' EXIT

# Files a package contributes, absolute; test files never.
FILES_TMPL='{{$d := .Dir}}{{range .GoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .CgoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .IgnoredGoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .CFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .CXXFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .MFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .HFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .FFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .SFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .SwigFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .SwigCXXFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .SysoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .IgnoredOtherFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .EmbedFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}'

golist() {
  # One go list, its failure fatal (a process substitution would swallow it).
  local platform="$1" out="$2"
  shift 2
  GOWORK=off GOFLAGS=-mod=readonly CGO_ENABLED=1 GOOS="${platform%/*}" GOARCH="${platform#*/}" go list "$@" > "${out}" 2> "${out}.err" \
    || fail "${platform} go list $1 failed: $(head -n 3 "${out}.err")"
}

# go_closure writes the app-kit files of (a) and (b) to ${WORK}/go.
go_closure() {
  local mod kit platform pkgs=()
  mod="$(GOWORK=off go list -m -f '{{.Path}}')" || fail "go list -m failed"
  kit="${mod}/${MIRROR}"
  : > "${WORK}/deps"
  for platform in "${PLATFORMS[@]}"; do
    local roots=()
    golist "${platform}" "${WORK}/roots" ./...
    while IFS= read -r p; do roots+=("${p}"); done < <(grep -v "^${mod}/third_party/" "${WORK}/roots")
    [ "${#roots[@]}" -gt 0 ] || fail "go list found no packages of ${mod}"
    golist "${platform}" "${WORK}/d" -deps -test -f '{{.ImportPath}}' "${roots[@]}"
    cat "${WORK}/d" >> "${WORK}/deps"
    golist "${platform}" "${WORK}/d" -deps -f '{{.ImportPath}}' "./${MIRROR}/cmd/appkit"
    cat "${WORK}/d" >> "${WORK}/deps"
  done
  while IFS= read -r p; do pkgs+=("${p}"); done < <(sed 's/ \[.*\]$//' "${WORK}/deps" | grep "^${kit}/" | LC_ALL=C sort -u)
  [ "${#pkgs[@]}" -gt 0 ] || fail "no app-kit package in fathom's build closure"
  : > "${WORK}/go"
  for platform in "${PLATFORMS[@]}"; do
    golist "${platform}" "${WORK}/f" -f "${FILES_TMPL}" "${pkgs[@]}"
    cat "${WORK}/f" >> "${WORK}/go"
  done
}

is_renamed() {
  local r
  for r in "${RENAMED[@]}"; do [ "$1" = "${r}" ] && return 0; done
  return 1
}

# Mirror paths named by the repository's own non-Go files, normalized.
references() {
  git grep --untracked -hoE "${MIRROR}/[^][[:space:]\"'\`():,;<>|#{}\$\\]+" -- . ':!third_party' ':!*.go' \
    | sed -E 's/[.]+$//; s|/+$||' | LC_ALL=C sort -u
}

# Resolve the references against the mirror; directories are checked last,
# against the finished closure, by the caller.
resolve_references() {
  local ref match found
  while IFS= read -r ref; do
    if [[ "${ref}" == *'*'* ]]; then
      found=0
      while IFS= read -r match; do
        [ -f "${match}" ] && { printf 'F %s\n' "${match}"; found=1; }
      done < <(compgen -G "${ref}" || true)
      [ "${found}" -eq 1 ] || fail "${ref} (named outside third_party/) matches nothing in the mirror — run: task sync-kit, or fix the reference"
    elif [ -f "${ref}" ]; then
      printf 'F %s\n' "${ref}"
    elif [ -d "${ref}" ]; then
      printf 'D %s\n' "${ref}"
    elif is_renamed "${ref}"; then
      continue
    else
      fail "${ref} is named outside third_party/ but the mirror does not hold it — run: task sync-kit, or fix the reference"
    fi
  done < <(references)
}

# compute writes the closure to ${WORK}/closure (a file, so a failure exits
# here instead of vanishing inside a process substitution).
compute() {
  local work="${WORK}" f d
  go_closure
  { grep -v '_test\.go$' "${work}/go" || true; } | sed "s|^${REPO_ROOT}/||" | { grep "^${MIRROR}/" || true; } > "${work}/files"
  resolve_references > "${work}/refs"
  grep '^F ' "${work}/refs" | cut -c3- >> "${work}/files" || true
  for f in "${ROOT_FILES[@]}"; do
    [ -f "${MIRROR}/${f}" ] || fail "${MIRROR}/${f} is missing — run: task sync-kit"
    printf '%s\n' "${MIRROR}/${f}" >> "${work}/files"
  done
  [ -d "${MIRROR}/fathom-pack" ] || fail "${MIRROR}/fathom-pack is missing — run: task sync-kit"
  find "${MIRROR}/fathom-pack" -type f >> "${work}/files"
  LC_ALL=C sort -u "${work}/files" > "${work}/closure"
  while IFS= read -r d; do
    grep -q "^${d}/" "${work}/closure" \
      || fail "${d} is named outside third_party/ but no file under it is in the closure — name its files, or fix the reference"
  done < <(grep '^D ' "${work}/refs" | cut -c3-)
}

mirror_files() {
  find "${MIRROR}" -type f | LC_ALL=C sort
}

case "${mode}" in
  print)
    compute
    cat "${WORK}/closure"
    ;;
  write)
    compute
    cp "${WORK}/closure" "${MANIFEST}.tmp"
    mv -f "${MANIFEST}.tmp" "${MANIFEST}"
    echo "kit-closure: wrote ${MANIFEST} ($(wc -l < "${MANIFEST}" | tr -d ' ') paths)"
    ;;
  check)
    [ -f "${MANIFEST}" ] || fail "${MANIFEST} is missing — run: bash scripts/kit-closure.sh --write"
    compute
    rc=0
    if ! diff "${WORK}/closure" "${MANIFEST}" > "${WORK}/diff"; then
      echo "kit-closure: ${MANIFEST} differs from the computed closure (< computed, > manifest):" >&2
      head -n 40 "${WORK}/diff" >&2
      echo "kit-closure: run: bash scripts/kit-closure.sh --write (after task sync-kit when the closure grew)" >&2
      rc=1
    fi
    extra="$(LC_ALL=C comm -23 <(mirror_files) "${MANIFEST}")"
    missing="$(LC_ALL=C comm -13 <(mirror_files) "${MANIFEST}")"
    if [ -n "${extra}" ]; then
      echo "kit-closure: ${MIRROR} holds $(printf '%s\n' "${extra}" | wc -l | tr -d ' ') file(s) outside ${MANIFEST}, first: $(printf '%s\n' "${extra}" | head -n 1) — run: bash scripts/kit-closure.sh --prune" >&2
      rc=1
    fi
    if [ -n "${missing}" ]; then
      echo "kit-closure: ${MIRROR} lacks $(printf '%s\n' "${missing}" | wc -l | tr -d ' ') manifest file(s), first: $(printf '%s\n' "${missing}" | head -n 1) — run: task sync-kit" >&2
      rc=1
    fi
    [ "${rc}" -eq 0 ] && echo "kit-closure: ${MANIFEST} is the computed closure ($(wc -l < "${MANIFEST}" | tr -d ' ') paths) and the mirror holds exactly it"
    exit "${rc}"
    ;;
  prune)
    [ -f "${MANIFEST}" ] || fail "${MANIFEST} is missing — run: bash scripts/kit-closure.sh --write"
    n=0
    while IFS= read -r f; do
      rm -f "${f}"
      n=$((n + 1))
    done < <(LC_ALL=C comm -23 <(mirror_files) "${MANIFEST}")
    find "${MIRROR}" -type d -empty -delete
    echo "kit-closure: removed ${n} file(s) outside ${MANIFEST}"
    ;;
esac
