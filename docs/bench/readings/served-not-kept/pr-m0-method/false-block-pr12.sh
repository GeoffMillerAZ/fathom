#!/usr/bin/env bash
# PR-12's false-block check (ADR-057 revisit clause 2), CV-M2's method: run the
# pre-push rule, `fathom conserve --since C^ --strict-plane integrity`, over
# single commits of this repository's own history and list every pin line.
#
# Two commit sets, each capped at 50, most recent first:
#   guarded      CV-M2's set: commits touching a guarded test file or a pin's
#                naming source. Guarded test files: every _test.go carrying a
#                `// certifies:` marker, plus the file defining each Test*
#                named in docs/intent/invariants.md, docs/intent/scope.md,
#                .fathom/guard.cue or tests/*.cue (resolved at $FB_REF).
#   firstparent  the last 50 first-parent commits of $FB_REF (merges included:
#                a merge's first-parent range is a wave's pushed range).
#
# Inputs (environment):
#   FB_REPO    a clone of fathom (a worktree is added under FB_DIR, then removed)
#   FB_REF     the tip to walk back from (the reading used origin/main)
#   FB_DIR     work directory
#   FB_FATHOM  the fathom binary under test
#   FB_SETS    optional; the sets to run (default "guarded firstparent")
#   FB_PRE     optional shell command run before every commit (the reading's host
#              cleared a stale file-access verdict on its external volume with it)
# Output: $FB_DIR/<set>.tsv (sha, exit, summary line) and $FB_DIR/<set>.pins
# (every pin line, prefixed with its sha). No model is called anywhere.
set -uo pipefail
: "${FB_REPO:?}" "${FB_REF:?}" "${FB_DIR:?}" "${FB_FATHOM:?}"
mkdir -p "$FB_DIR"
WT="$FB_DIR/wt"
git -C "$FB_REPO" worktree remove --force "$WT" 2>/dev/null
git -C "$FB_REPO" worktree add -q --detach "$WT" "$FB_REF" || exit 2
trap 'git -C "$FB_REPO" worktree remove --force "$WT" 2>/dev/null' EXIT

# The guarded file set, resolved at FB_REF.
names=$(cd "$WT" && cat docs/intent/invariants.md docs/intent/scope.md .fathom/guard.cue tests/*.cue 2>/dev/null |
	grep -oE '\bTest[A-Za-z0-9_]+' | sort -u)
files=$(cd "$WT" && {
	git grep -l '// certifies:' -- '*_test.go' ':!third_party'
	for n in $names; do git grep -l "^func $n(" -- '*_test.go' ':!third_party'; done
} | sort -u)
sources="docs/intent/invariants.md docs/intent/scope.md .fathom/guard.cue tests"
echo "$files" >"$FB_DIR/guarded-files.txt"

git -C "$FB_REPO" log -n 50 --format=%H "$FB_REF" -- $files $sources >"$FB_DIR/guarded.commits"
git -C "$FB_REPO" rev-list --first-parent -n 50 "$FB_REF" >"$FB_DIR/firstparent.commits"

for set in ${FB_SETS:-guarded firstparent}; do
	: >"$FB_DIR/$set.tsv"
	: >"$FB_DIR/$set.pins"
	while read -r c; do
		[ -n "${FB_PRE:-}" ] && { bash -c "$FB_PRE" >/dev/null 2>&1 || true; }
		parent=$(git -C "$FB_REPO" rev-parse -q --verify "$c^1") || continue
		git -C "$WT" checkout -q -f --detach "$c" || exit 2
		git -C "$WT" clean -q -fdx
		db="$FB_DIR/store.sqlite"
		rm -f "$db" "$db-wal" "$db-shm"
		out=$(cd "$WT" && "$FB_FATHOM" conserve --since "$parent" --service fb --store "$db" --strict-plane integrity 2>&1)
		rc=$?
		printf '%s\t%s\t%s\n' "${c:0:10}" "$rc" "$(grep '^integrity pins:' <<<"$out")" >>"$FB_DIR/$set.tsv"
		{ grep '^pin ' <<<"$out" || true; } | sed "s/^/${c:0:10} /" >>"$FB_DIR/$set.pins"
	done <"$FB_DIR/$set.commits"
done
