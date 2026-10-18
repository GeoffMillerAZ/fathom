#!/usr/bin/env bash
#
# intent-stale_test.sh — proves the intent-audit skill's scripts/intent-stale.sh lists
# the condition lines whose last change is oldest, over a disposable git repository
# whose commits are backdated with GIT_AUTHOR_DATE — never the real checkout. Covers
# every label form the script documents, the lines it must skip (prose, code fences,
# backticked or quoted labels, index tables, files outside the roots), the age reset
# when a judged line is edited, the deterministic order (age, then path, then line),
# superseded records skipped (rejected ones kept), a later line of a condition's paragraph
# resetting its age, the 120-character cut on a multi-byte line, -n, explicit roots, and exit 0 outside a
# repository.
# Wired into the kit's `task scripts-check`.
#
# Usage:
#   bash scripts/intent-stale_test.sh

set -euo pipefail

# A git hook exports GIT_DIR (and, from a linked worktree, more GIT_* paths); inherited here, every
# git call below would act on the real repository, not the disposable one.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES
# The operator's git config (signing, hooks paths, blame ignore lists) must not reach the fixture.
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STALE="${SCRIPT_DIR}/../.claude/skills/intent-audit/scripts/intent-stale.sh"

fail() {
  echo "intent-stale test: $*" >&2
  exit 1
}

[ -f "$STALE" ] || fail "missing $STALE"

scratch="$(mktemp -d "${TMPDIR:-/tmp}/intent-stale-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
repo="$scratch/repo"
mkdir -p "$repo"
git -C "$repo" init -q
git -C "$repo" config user.name "Fixture"
git -C "$repo" config user.email "fixture@example.invalid"
git -C "$repo" config commit.gpgsign false

# commit_at <ISO date> <message>: commit everything with that author and committer date.
commit_at() {
  git -C "$repo" add -A
  GIT_AUTHOR_DATE="$1" GIT_COMMITTER_DATE="$1" git -C "$repo" commit -q -m "$2"
}

put() {
  mkdir -p "$(dirname "$repo/$1")"
  printf '%s\n' "$2" >"$repo/$1"
}

# futures <Drop if line>: the futures file, with list-marker labels, prose that must not
# match, and a fenced block.
futures() {
  put docs/intent/futures.md "# Futures

## F1. Alpha

- Confidence: plausible. Horizon: later.
- Promote when: a second consumer asks for alpha.
$1

We may revisit when the time is right, which is prose, not a condition.

\`\`\`
Revisit when: inside a fence, never listed.
\`\`\`"
}

# 2026-01-01: the futures file.
futures "- Drop if: alpha's sponsor leaves."
commit_at "2026-01-01T12:00:00Z" "futures"

# 2026-02-01: a decision record with a heading section, a bold label, a mid-line label
# after a bold span, and an index table whose column header names a label.
put docs/intent/decisions/0001-x.md "# 0001: X

**Status:** accepted

## Alternatives rejected

- **Keep two copies.** They drift. Wins if: the import line stops resolving.

## Revisit when

- a second repository adopts X.
- never for the stdlib rule.

## Notes

Not a condition."
put docs/intent/decisions/README.md "| Id | Status | Revisit when |
|---|---|---|
| 0001 | accepted | a second repository adopts X |"
commit_at "2026-02-01T12:00:00Z" "record"

# 2026-03-01: a root ADR file (a repository's decision records), a backticked label,
# a mid-sentence label with a colon, a bold label, a multi-byte line over 120 chars, a
# bold label after other text, an italic one opening a bullet, labels opening a sentence
# or following a dash without a colon, and quoted, backticked or mid-line italic labels
# that name a condition rather than state one.
long="Revisit when: the café's naïve résumé — measured, not argued — exceeds the threshold that the owner set on the first gated plan, twice in a row, across two repositories at once."
put ADR.md "# ADR-001: Y

The choice holds for now. Revisit when: a consumer pins a tag.
**Revisit when** the owner rules.
List them with \`Revisit when:\` in a grep, not a condition.
$long
Lost on cost. **Revisit when.** a second evaluator exists.
- *Revisit when (added).* the daemon sheds load.
A CUE catalog — wins if a second tool consumes it.
Held against I14. Revisit when the harness draws apps.
The first \"Revisit when\" bullet fired, which is a disposition, not a condition.
A reading: *Drop if* did not hold, an italic mention mid-line.
Each record's \`**Revisit when.**\` paragraph holds it, a backticked mention."
# Outside the roots: never listed by default.
put docs/other.md "Revisit when: outside the roots."
commit_at "2026-03-01T12:00:00Z" "adr"

# 2026-04-01: the judged line is edited (its reading added), so its age resets.
futures "- Drop if: alpha's sponsor leaves (2026-04-01: sponsor still active)."
commit_at "2026-04-01T12:00:00Z" "judge F1 Drop if"

# 2026-04-15: a superseded per-file record and a record log. A superseded record's
# conditions are closed and never listed; accepted and rejected ones are.
put docs/adr/0009-z.md "# 0009: Z

**Status:** Superseded by 0010

## Revisit when

- dead, the record is superseded."
put docs/adr/log.md "# Log

## ADR-1 — A

**Status.** superseded by ADR-2.

**Revisit when.** dead section.

## ADR-2 — B

**Status.** Accepted.

**Revisit when.** live section.

## ADR-3 — C

**Status:** Rejected.

- **Alt.** Lost. Wins if: still listed.

## ADR-4 — D

**Status.** Accepted.

**Revisit when.** judged paragraph condition
continues here."
commit_at "2026-04-15T12:00:00Z" "superseded records"

# 2026-04-20: a disposition sentence lands on a later line of ADR-4's paragraph, which
# resets that condition's age.
printf '%s\n' "Fired 2026-04-20: judged, stays." >>"$repo/docs/adr/log.md"
commit_at "2026-04-20T12:00:00Z" "judge ADR-4"

# The clock is fixed: 2026-05-01T12:00:00Z.
now=1777636800

run() {
  (cd "$repo" && INTENT_STALE_NOW="$now" bash "$STALE" "$@")
}

# The 120-character cut counts characters, not bytes: the long line keeps its accents whole.
cut120="Revisit when: the café's naïve résumé — measured, not argued — exceeds the threshold that the owner set on the first gat"

# Built by a function: bash 3.2 misparses a quote in a heredoc inside $(...).
want_rows() {
  cat <<EOF
 120d  docs/intent/futures.md:6  - Promote when: a second consumer asks for alpha.
  89d  docs/intent/decisions/0001-x.md:7  - **Keep two copies.** They drift. Wins if: the import line stops resolving.
  89d  docs/intent/decisions/0001-x.md:11  - a second repository adopts X.
  89d  docs/intent/decisions/0001-x.md:12  - never for the stdlib rule.
  61d  ADR.md:3  The choice holds for now. Revisit when: a consumer pins a tag.
  61d  ADR.md:4  **Revisit when** the owner rules.
  61d  ADR.md:6  $cut120
  61d  ADR.md:7  Lost on cost. **Revisit when.** a second evaluator exists.
  61d  ADR.md:8  - *Revisit when (added).* the daemon sheds load.
  61d  ADR.md:9  A CUE catalog — wins if a second tool consumes it.
  61d  ADR.md:10  Held against I14. Revisit when the harness draws apps.
  30d  docs/intent/futures.md:7  - Drop if: alpha's sponsor leaves (2026-04-01: sponsor still active).
  16d  docs/adr/log.md:13  **Revisit when.** live section.
  16d  docs/adr/log.md:19  - **Alt.** Lost. Wins if: still listed.
  11d  docs/adr/log.md:25  **Revisit when.** judged paragraph condition
EOF
}
want="$(want_rows)"

# -n 0 lists every condition line.
set +e
got="$(run -n 0 2>"$scratch/err")"
rc=$?
set -e
[ "$rc" -eq 0 ] || fail "-n 0: exit $rc, want 0 ($(cat "$scratch/err"))"
if [ "$got" != "$want" ]; then
  diff <(printf '%s\n' "$want") <(printf '%s\n' "$got") >&2 || true
  fail "-n 0: output differs (want left, got right)"
fi

# The default is the ten oldest, in the same order.
gotdefault="$(run)"
[ "$gotdefault" = "$(printf '%s\n' "$want" | head -n 10)" ] || fail "default run: got: $gotdefault"

# -n bounds the list to the oldest N, in the same order.
got3="$(run -n 3)"
[ "$got3" = "$(printf '%s\n' "$want" | head -n 3)" ] || fail "-n 3: got: $got3"

# Explicit roots replace the defaults: docs/other.md is listed only when named.
gotother="$(run docs/other.md)"
[ "$gotother" = "  61d  docs/other.md:1  Revisit when: outside the roots." ] || fail "explicit root: got: $gotother"

# Same tree, same clock, same bytes.
[ "$(run -n 0)" = "$got" ] || fail "second run differs"

# A usage error exits 2; outside a repository the script says so and exits 0.
set +e
(cd "$repo" && bash "$STALE" -n x) >/dev/null 2>&1
rc=$?
set -e
[ "$rc" -eq 2 ] || fail "-n x: exit $rc, want 2"
mkdir -p "$scratch/plain"
set +e
outplain="$(cd "$scratch/plain" && GIT_CEILING_DIRECTORIES="$scratch" bash "$STALE" 2>&1)"
rc=$?
set -e
[ "$rc" -eq 0 ] || fail "outside a repository: exit $rc, want 0"
echo "$outplain" | grep -q "not a git repository" || fail "outside a repository: output: $outplain"

echo "intent-stale test: ok"
