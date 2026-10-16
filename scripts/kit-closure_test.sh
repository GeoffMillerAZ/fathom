#!/usr/bin/env bash
#
# kit-closure_test.sh — shell-level test for scripts/kit-closure.sh (DS-4).
#
# Builds a throwaway consumer module (a git repo laid out like fathom, with a
# small third_party/app-kit mirror) under a temp dir and asserts the computed
# closure: the packages the consumer and the mirror's cmd/appkit reach, their
# go:embed targets, a test-only import, the non-Go paths the consumer names,
# the root files and fathom-pack/; never an unused package, a test file or a
# data directory nobody names. Then the modes: --write/--prune/--check agree,
# and --check fails on a stray file, a stale manifest and a dangling reference.
#
# Usage:
#   bash scripts/kit-closure_test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

pass=0
ok() {
  pass=$((pass + 1))
  echo "ok — $1"
}

TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

C="${TMP}/consumer"
# The mirror prefix is spelled through a variable: kit-closure.sh reads every
# non-Go file of the real repository, this one included, for mirror paths.
KP="third_party/app-kit"
K="${C}/${KP}"
M=example.com/consumer
mkdir -p "${C}/scripts" "${C}/cmd/app" "${C}/docs" \
  "${K}/ui/card" "${K}/ui/unused" "${K}/ui/helper" "${K}/internal/tw" "${K}/cmd/appkit" \
  "${K}/data/assets" "${K}/docs" "${K}/fathom-pack/layers" "${K}/cue/tokens"
cp "${SCRIPT_DIR}/kit-closure.sh" "${C}/scripts/kit-closure.sh"
printf 'module %s\n\ngo 1.26\n' "${M}" > "${C}/go.mod"

# The consumer imports ui/card; its test imports ui/helper.
printf 'package main\n\nimport _ "%s/%s/ui/card"\n\nfunc main() {}\n' "${M}" "${KP}" > "${C}/cmd/app/main.go"
printf 'package main\n\nimport (\n\t"testing"\n\n\t_ "%s/%s/ui/helper"\n)\n\nfunc TestX(t *testing.T) {}\n' "${M}" "${KP}" > "${C}/cmd/app/main_test.go"
# A doc names one non-Go mirror file, a glob and a renamed name.
printf 'Tokens: %s/cue/tokens/kit.cue; sources %s/ui/card/*.go.\nOld pin: %s/go.mod.\n' "${KP}" "${KP}" "${KP}" > "${C}/docs/notes.md"

# ui/card embeds a stylesheet; its test file must not be kept.
printf 'package card\n\nimport _ "embed"\n\n//go:embed card.css\nvar CSS string\n' > "${K}/ui/card/card.go"
printf '.card{}\n' > "${K}/ui/card/card.css"
printf 'package card\n\nimport "testing"\n\nfunc TestCard(t *testing.T) {}\n' > "${K}/ui/card/card_test.go"
printf 'package card\n' > "${K}/ui/card/card_windows.go"
printf 'package helper\n' > "${K}/ui/helper/helper.go"
printf 'package unused\n' > "${K}/ui/unused/unused.go"
printf 'package tw\n' > "${K}/internal/tw/tw.go"
printf 'package main\n\nimport _ "%s/%s/internal/tw"\n\nfunc main() {}\n' "${M}" "${KP}" > "${K}/cmd/appkit/main.go"
printf '{}\n' > "${K}/data/assets/manifest.json"
printf '# docs\n' > "${K}/docs/guide.md"
printf 'tokens: {}\n' > "${K}/cue/tokens/kit.cue"
printf 'pack\n' > "${K}/fathom-pack/layers/a.cue"
for f in AGENTS.md CLAUDE.md LICENSE.md README.md ROADMAP.md; do printf '%s\n' "${f}" > "${K}/${f}"; done
printf 'module github.com/geoffmilleraz/app-kit\n' > "${K}/go.mod.upstream"
printf '\n' > "${K}/go.sum.upstream"
printf '0123456789abcdef0123456789abcdef01234567\n' > "${K}/.vendor-commit"

printf 'workspace/\n' > "${C}/.gitignore"
( cd "${C}" && git init -q && git add -A )

run() { ( cd "${C}" && GOWORK=off bash scripts/kit-closure.sh "$@" ) > "${TMP}/out" 2> "${TMP}/err"; }

run || fail "print mode failed: $(cat "${TMP}/err")"
for f in .vendor-commit AGENTS.md CLAUDE.md LICENSE.md README.md cmd/appkit/main.go cue/tokens/kit.cue fathom-pack/layers/a.cue go.mod.upstream go.sum.upstream internal/tw/tw.go ui/card/card.css ui/card/card.go ui/card/card_test.go ui/card/card_windows.go ui/helper/helper.go; do
  printf '%s/%s\n' "${KP}" "${f}"
done > "${TMP}/want"
diff "${TMP}/want" "${TMP}/out" > "${TMP}/d" || fail "computed closure differs (< want, > got): $(cat "${TMP}/d")"
ok "closure: imported, test-imported and cmd/appkit packages, embeds, every-GOOS files, named paths, root files; a glob keeps the test file it names"

# card_test.go is kept only because the doc's glob names it; without the glob it goes.
printf 'Tokens: %s/cue/tokens/kit.cue.\n' "${KP}" > "${C}/docs/notes.md"
run || fail "print mode failed: $(cat "${TMP}/err")"
grep -q 'card_test.go' "${TMP}/out" && fail "a test file nobody names is in the closure"
ok "a package's test files are not its build closure"

run --check && fail "--check passed with no manifest"
grep -q 'missing' "${TMP}/err" || fail "--check without a manifest should say it is missing"
ok "--check without a manifest fails"

run --write || fail "--write failed: $(cat "${TMP}/err")"
run --check && fail "--check passed while the mirror holds files outside the manifest"
grep -q -- '--prune' "${TMP}/err" || fail "a stray-file failure should say run --prune"
ok "--check fails while the mirror holds unlisted files"

run --prune || fail "--prune failed: $(cat "${TMP}/err")"
[ ! -e "${K}/data" ] && [ ! -e "${K}/docs" ] && [ ! -e "${K}/ui/unused" ] && [ ! -e "${K}/ROADMAP.md" ] \
  || fail "--prune left an unlisted path"
run --check || fail "--check failed after --write and --prune: $(cat "${TMP}/err")"
ok "--prune removes data, docs, an unused package and narrative files; --check then passes"

printf 'x\n' > "${K}/ui/card/stray.txt"
run --check && fail "--check passed with a stray file in the mirror"
rm "${K}/ui/card/stray.txt"
ok "--check fails on a stray mirror file"

printf '%s/ui/card/card.go\n' "${KP}" >> "${C}/third_party/app-kit.keep"
LC_ALL=C sort -o "${C}/third_party/app-kit.keep" "${C}/third_party/app-kit.keep"
printf '%s/zzz.txt\n' "${KP}" >> "${C}/third_party/app-kit.keep"
run --check && fail "--check passed with a stale manifest"
grep -q 'differs from the computed closure' "${TMP}/err" || fail "a stale manifest should say it differs"
run --write || fail "--write failed"
ok "--check fails on a stale manifest"

printf 'See %s/docs/guide.md.\n' "${KP}" >> "${C}/docs/notes.md"
run --check && fail "--check passed while a doc names a pruned path"
grep -q 'docs/guide.md' "${TMP}/err" || fail "a dangling reference should be named"
ok "--check fails when a doc names a path the mirror does not hold"

echo ""
echo "kit-closure_test.sh: ${pass}/${pass} checks passed"
