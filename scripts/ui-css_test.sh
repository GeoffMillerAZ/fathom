#!/usr/bin/env bash
# Synthetic refusal tests for scripts/ui-css.sh. Everything lives below one
# throwaway directory; the repository's real input and generated CSS are read
# only to seed the fixture and are never modified.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"
REAL_GO="$(command -v go)"
REAL_SHASUM="$(command -v shasum)"
mkdir -p "${ROOT}/workspace/tmp"
TMP="$(mktemp -d "${ROOT}/workspace/tmp/ui-css-test.XXXXXX")"
TMP="$(cd "${TMP}" && pwd -P)"
trap 'rm -rf "${TMP}"' EXIT
FIXTURE="${TMP}/fixture"
TRACE="${TMP}/trace.log"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass=0
ok() { pass=$((pass + 1)); echo "ok - $1"; }
expect_fail() {
  local description="$1" needle="$2"; shift 2
  if "$@" >"${TMP}/out.log" 2>&1; then
    fail "${description}: unexpectedly passed"
  fi
  grep -Fq "${needle}" "${TMP}/out.log" || { cat "${TMP}/out.log" >&2; fail "${description}: missing refusal text ${needle}"; }
  ok "${description}"
}
expect_pass() {
  local description="$1"; shift
  if ! "$@" >"${TMP}/out.log" 2>&1; then
    cat "${TMP}/out.log" >&2
    fail "${description}: unexpectedly failed"
  fi
  ok "${description}"
}
assert_outputs_unchanged() {
  local description="$1" theme_before="$2" css_before="$3"
  [ "${theme_before}" = "$(shasum -a 256 "${FIXTURE}/cmd/fathom/ui/tokens/theme.css")" ] \
    || fail "${description}: changed theme.css"
  [ "${css_before}" = "$(shasum -a 256 "${FIXTURE}/cmd/fathom/ui/app.css")" ] \
    || fail "${description}: changed app.css"
}

mkdir -p "${FIXTURE}/scripts/lib" "${FIXTURE}/cmd/fathom/ui/tokens" \
  "${FIXTURE}/third_party/app-kit/cmd/appkit" \
  "${FIXTURE}/third_party/app-kit/cue/tokens" \
  "${FIXTURE}/third_party/app-kit/internal/tailwind" \
  "${FIXTURE}/third_party/app-kit/appkit/ui/tokens" \
  "${FIXTURE}/workspace/tmp" "${TMP}/bin"
cp "${SCRIPT_DIR}/ui-css.sh" "${FIXTURE}/scripts/ui-css.sh"
cp "${SCRIPT_DIR}/lib/build-cache.sh" "${FIXTURE}/scripts/lib/build-cache.sh"
cp "${ROOT}/cmd/fathom/ui/input.css" "${FIXTURE}/cmd/fathom/ui/input.css"
cp "${ROOT}/cmd/fathom/ui/guide-baseline.cue" "${FIXTURE}/cmd/fathom/ui/guide-baseline.cue"
printf '0000000000000000000000000000000000000000\n' > "${FIXTURE}/third_party/app-kit/.vendor-commit"
printf 'tokens: {}\n' > "${FIXTURE}/third_party/app-kit/cue/tokens/kit.cue"
printf 'module example.test/fixture\n\ngo 1.26.2\n' > "${FIXTURE}/go.mod"
printf 'module github.com/geoffmilleraz/app-kit\n\ngo 1.26.2\n' > "${FIXTURE}/third_party/app-kit/go.mod.upstream"
: > "${FIXTURE}/third_party/app-kit/go.sum.upstream"
printf 'package tailwind\n' > "${FIXTURE}/third_party/app-kit/internal/tailwind/tailwind.go"
printf 'package tokens\n' > "${FIXTURE}/third_party/app-kit/appkit/ui/tokens/tokens.go"

# Materialize one readable file for every audited declaration in input.css.
while IFS= read -r declaration; do
  target="${FIXTURE}/cmd/fathom/ui/${declaration}"
  if [[ "${target}" == *'*'* ]]; then
    target="${target%\*}fixture.go"
  fi
  mkdir -p "$(dirname "${target}")"
  printf 'package fixture\n' > "${target}"
done < <(sed -n 's/^[[:space:]]*@source "\([^"]*\)";[[:space:]]*$/\1/p' "${FIXTURE}/cmd/fathom/ui/input.css")
# ...and for every kit stylesheet it imports (the docs prose, L1).
while IFS= read -r declaration; do
  target="${FIXTURE}/cmd/fathom/ui/${declaration}"
  mkdir -p "$(dirname "${target}")"
  printf '/* fixture */\n' > "${target}"
done < <(sed -n 's/^[[:space:]]*@import "\(\.\.\/[^"]*\.css\)";[[:space:]]*$/\1/p' "${FIXTURE}/cmd/fathom/ui/input.css")

# A tiny appkit command lets the real script exercise its build/stage/check
# flow without a network request or the real generated outputs.
cat > "${FIXTURE}/third_party/app-kit/cmd/appkit/main.go" <<'GO'
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
)

func flag(name string) string {
	for i := 2; i+1 < len(os.Args); i++ {
		if os.Args[i] == name { return os.Args[i+1] }
	}
	return ""
}

func main() {
	wd, _ := os.Getwd()
	if trace := os.Getenv("UI_CSS_TEST_TRACE"); trace != "" {
		f, _ := os.OpenFile(trace, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		fmt.Fprintf(f, "appkit|%s|%s\n", wd, strings.Join(os.Args[1:], " "))
		_ = f.Close()
	}
	if len(os.Args) < 2 { os.Exit(2) }
	switch os.Args[1] {
	case "gen":
		source, err := os.ReadFile(flag("-tokens"))
		if err != nil { os.Exit(3) }
		_ = os.WriteFile(flag("-css"), []byte(fmt.Sprintf("/* generated theme from %x */\n", sha256.Sum256(source))), 0o644)
	case "css":
		_ = os.WriteFile(flag("-o"), []byte("/* generated app */\n"), 0o644)
		if mutate := os.Getenv("UI_CSS_TEST_MUTATE"); mutate != "" {
			f, _ := os.OpenFile(mutate, os.O_APPEND|os.O_WRONLY, 0o644)
			_, _ = f.WriteString("// changed during build\n")
			_ = f.Close()
		}
	default:
		os.Exit(2)
	}
}
GO

# The wrapper proves the production builder invokes Go from the root module
# (ADR-070: the mirror is its packages) with GOWORK=off; it delegates to the
# actual Go tool for the synthetic build.
cat > "${TMP}/bin/go" <<EOF
#!/usr/bin/env bash
if [ "\${1:-}" = build ]; then
  printf 'go|%s|GOWORK=%s\n' "\${PWD}" "\${GOWORK:-}" >> "${TRACE}"
  [ "\${PWD}" = "${FIXTURE}" ] || exit 91
  [ "\${GOWORK:-}" = off ] || exit 92
fi
exec "${REAL_GO}" "\$@"
EOF
chmod +x "${TMP}/bin/go"
cat > "${TMP}/bin/shasum" <<EOF
#!/usr/bin/env bash
for arg in "\$@"; do
  if [ -n "\${UI_CSS_TEST_UNREADABLE:-}" ] && [ -e "\${arg}" ]; then
    physical="\$(cd "\$(dirname "\${arg}")" && pwd -P)/\$(basename "\${arg}")"
    if [ "\${physical}" = "\${UI_CSS_TEST_UNREADABLE}" ]; then
      echo "synthetic shasum read denied: \${physical}" >&2
      exit 77
    fi
  fi
done
exec "${REAL_SHASUM}" "\$@"
EOF
chmod +x "${TMP}/bin/shasum"
cat > "${TMP}/bin/[" <<'EOF'
#!/bin/sh
case "${1:-}" in
  -r) exit 1 ;;
esac
exec /bin/[ "$@"
EOF
chmod +x "${TMP}/bin/["
printf '%s\n' "enable -n '['" > "${TMP}/disable-test-r.bash"
export PATH="${TMP}/bin:${PATH}"
export UI_CSS_TEST_TRACE="${TRACE}"

(cd "${FIXTURE}" && bash scripts/ui-css.sh >/dev/null)
grep -Fq "go|${FIXTURE}|GOWORK=off" "${TRACE}" || fail "root-module GOWORK=off build was not observed"
grep -Fq "appkit|${FIXTURE}|gen -tokens ${FIXTURE}/cmd/fathom/ui/guide-baseline.cue" "${TRACE}" || fail "root-local app-owned token generation was not observed"
grep -Fq -- '-terminal ' "${TRACE}" || fail "token generation did not pass the explicit empty terminal output"
ok "mirrored builder uses the app-owned token source"

THEME_BEFORE="$(shasum -a 256 "${FIXTURE}/cmd/fathom/ui/tokens/theme.css")"
CSS_BEFORE="$(shasum -a 256 "${FIXTURE}/cmd/fathom/ui/app.css")"
expect_pass "false access predicate cannot override successful content reads" \
  env BASH_ENV="${TMP}/disable-test-r.bash" bash "${FIXTURE}/scripts/ui-css.sh"
assert_outputs_unchanged "false-access-predicate build" "${THEME_BEFORE}" "${CSS_BEFORE}"

mv "${FIXTURE}/cmd/fathom/observatory_render.go" "${TMP}/observatory_render.go"
expect_fail "missing selected source refuses before writing" "required source selection is empty" \
  bash "${FIXTURE}/scripts/ui-css.sh"
assert_outputs_unchanged "missing-source refusal" "${THEME_BEFORE}" "${CSS_BEFORE}"
mv "${TMP}/observatory_render.go" "${FIXTURE}/cmd/fathom/observatory_render.go"

cp "${FIXTURE}/cmd/fathom/ui/input.css" "${TMP}/input.css"
sed 's/@import "tailwindcss" source(none);/@import "tailwindcss";/' "${TMP}/input.css" > "${FIXTURE}/cmd/fathom/ui/input.css"
expect_fail "implicit Tailwind scanning is refused" "exact audited import/source closure" \
  bash "${FIXTURE}/scripts/ui-css.sh"
assert_outputs_unchanged "implicit-scan refusal" "${THEME_BEFORE}" "${CSS_BEFORE}"
cp "${TMP}/input.css" "${FIXTURE}/cmd/fathom/ui/input.css"

printf '\n/* hidden additive directive: @import "tailwindcss"; */\n' >> "${FIXTURE}/cmd/fathom/ui/input.css"
expect_fail "embedded additive import is refused" "exact audited import/source closure" \
  bash "${FIXTURE}/scripts/ui-css.sh"
assert_outputs_unchanged "additive-import refusal" "${THEME_BEFORE}" "${CSS_BEFORE}"
cp "${TMP}/input.css" "${FIXTURE}/cmd/fathom/ui/input.css"

printf '\n/* hidden additive directive: @source "../../../../outside/*.go"; */\n' >> "${FIXTURE}/cmd/fathom/ui/input.css"
expect_fail "embedded out-of-repo source is refused" "exact audited import/source closure" \
  bash "${FIXTURE}/scripts/ui-css.sh"
assert_outputs_unchanged "additive-source refusal" "${THEME_BEFORE}" "${CSS_BEFORE}"
cp "${TMP}/input.css" "${FIXTURE}/cmd/fathom/ui/input.css"

export UI_CSS_TEST_UNREADABLE="${FIXTURE}/cmd/fathom/observatory_apply.go"
expect_fail "unreadable selected source refuses before writing" "shasum exit 77: synthetic shasum read denied: ${FIXTURE}/cmd/fathom/observatory_apply.go" \
  bash "${FIXTURE}/scripts/ui-css.sh"
grep -Fq "required source is unreadable" "${TMP}/out.log" \
  || fail "unreadable selected source: missing generic refusal text"
unset UI_CSS_TEST_UNREADABLE
assert_outputs_unchanged "unreadable-source refusal" "${THEME_BEFORE}" "${CSS_BEFORE}"

cp "${FIXTURE}/cmd/fathom/observatory_series.go" "${TMP}/observatory_series.go"
export UI_CSS_TEST_MUTATE="${FIXTURE}/cmd/fathom/observatory_series.go"
expect_fail "source changed during build refuses publication" "selected sources changed during generation" \
  bash "${FIXTURE}/scripts/ui-css.sh"
unset UI_CSS_TEST_MUTATE
assert_outputs_unchanged "concurrent-source refusal" "${THEME_BEFORE}" "${CSS_BEFORE}"
cp "${TMP}/observatory_series.go" "${FIXTURE}/cmd/fathom/observatory_series.go"

cp "${FIXTURE}/cmd/fathom/ui/guide-baseline.cue" "${TMP}/guide-baseline.cue"
export UI_CSS_TEST_MUTATE="${FIXTURE}/cmd/fathom/ui/guide-baseline.cue"
expect_fail "token source changed during build refuses publication" "selected sources changed during generation" \
  bash "${FIXTURE}/scripts/ui-css.sh"
unset UI_CSS_TEST_MUTATE
assert_outputs_unchanged "concurrent-token refusal" "${THEME_BEFORE}" "${CSS_BEFORE}"
cp "${TMP}/guide-baseline.cue" "${FIXTURE}/cmd/fathom/ui/guide-baseline.cue"

printf '// changed after generation\n' >> "${FIXTURE}/cmd/fathom/ui/guide-baseline.cue"
expect_fail "changed token source makes generated theme stale" "generated token theme is stale" \
  bash "${FIXTURE}/scripts/ui-css.sh" --check
assert_outputs_unchanged "token-source stale check" "${THEME_BEFORE}" "${CSS_BEFORE}"
cp "${TMP}/guide-baseline.cue" "${FIXTURE}/cmd/fathom/ui/guide-baseline.cue"

printf '/* stale */\n' >> "${FIXTURE}/cmd/fathom/ui/app.css"
STALE_BEFORE="$(shasum -a 256 "${FIXTURE}/cmd/fathom/ui/app.css")"
expect_fail "stale generated output is refused by check mode" "generated app.css is stale" \
  bash "${FIXTURE}/scripts/ui-css.sh" --check
[ "${STALE_BEFORE}" = "$(shasum -a 256 "${FIXTURE}/cmd/fathom/ui/app.css")" ] || fail "stale-output check rewrote app.css"

mv "${FIXTURE}/cmd/fathom/observatory_story.go" "${TMP}/observatory_story.go"
ln -s "${TMP}/observatory_story.go" "${FIXTURE}/cmd/fathom/observatory_story.go"
expect_fail "out-of-repository source resolution is refused" "must not be a symlink" \
  bash "${FIXTURE}/scripts/ui-css.sh"
assert_outputs_unchanged "symlink-source refusal" "${THEME_BEFORE}" "${STALE_BEFORE}"

echo "ui-css_test.sh: ${pass}/${pass} checks passed"
