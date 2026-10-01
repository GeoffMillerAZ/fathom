#!/usr/bin/env bash
# lines-replay.sh: SV-M1's $0 replay (docs/plans/stewardship.md). Rebuilds the
# repositories of the 2026-09-29 consolidation backup from its bundles, stash
# bundles and worktree patches, then runs `fathom plans lines --json` on each.
#
#   lines-replay.sh BACKUP_DIR OUT_DIR FATHOM_BIN [FATHOM_CLONE]
#
# BACKUP_DIR is a read-only COPY of workspace/durable-2026-09-29/cleanup/ (never
# the owner's tree); OUT_DIR is scratch. FATHOM_CLONE (a clone of fathom with
# origin/main) supplies the main the unfinished CSS worker branched from.
# Clean worktrees are checked out sparse (nothing but their index), so 46
# worktrees cost megabytes; a dirty one is checked out whole and its saved
# patch and untracked files are put back.
set -euo pipefail
B=$(cd "$1" && pwd)
O=$2
F=$3
CLONE=${4:-}
mkdir -p "$O"
O=$(cd "$O" && pwd)
export GIT_AUTHOR_NAME=replay GIT_AUTHOR_EMAIL=replay@example.invalid GIT_COMMITTER_NAME=replay GIT_COMMITTER_EMAIL=replay@example.invalid
g() { git -c commit.gpgsign=false -c core.hooksPath=/dev/null -c advice.detachedHead=false "$@"; }
log() { printf '[replay] %s\n' "$*" >&2; }

# rebuild NAME BUNDLE: a non-bare repository holding every ref of the bundle.
rebuild() {
  local r=$O/$1
  rm -rf "$r" "$O/$1-wt"
  g init -q -b replay-unborn "$r"
  g -C "$r" fetch -q --update-head-ok "$2" 'refs/*:refs/*'
  g -C "$r" checkout -q -f main
}

# stashes NAME: re-create the stash list (oldest first) from stashes.tsv;
# objects come from the all-refs bundle or the per-stash bundle.
stashes() {
  local name=$1 r=$O/$1 rows
  rows=$(awk -F'\t' -v n="$name" '$6==n {print $1"\t"$2}' "$B/manifest.parts/stashes.tsv" | sort -t'{' -k2 -rn)
  [ -n "$rows" ] || return 0
  g -C "$r" update-ref -d refs/stash 2>/dev/null || true
  while IFS=$'\t' read -r sel sha; do
    local idx=${sel#stash@\{}; idx=${idx%\}}
    if ! g -C "$r" cat-file -e "$sha^{commit}" 2>/dev/null; then
      g -C "$r" fetch -q "$B/$name-stash-$idx.bundle" "refs/stash-$idx:refs/replay/stash-$idx"
    fi
    local msg
    msg=$(sed -n 's/^full: //p' "$B/$name-stash-$idx.msg")
    g -C "$r" stash store -m "$msg" "$sha"
  done <<<"$rows"
  g -C "$r" for-each-ref --format='%(refname)' refs/replay/ | while read -r ref; do g -C "$r" update-ref -d "$ref"; done
}

# worktrees NAME: re-create every linked worktree of worktrees.tsv (its first
# row for a repository is the main worktree, put back in place).
worktrees() {
  local name=$1 r=$O/$1 main
  main=$(awk -F'\t' -v n="$name" '$1==n {print $2; exit}' "$B/manifest.parts/worktrees.tsv")
  awk -F'\t' -v n="$name" '$1==n' "$B/manifest.parts/worktrees.tsv" | while IFS=$'\t' read -r _ path branch head status saved; do
    local dirty=${status%% *} base wt
    base=$(basename "$path")
    if [ "$path" = "$main" ]; then
      wt=$r
    else
      wt=$O/$name-wt/$base
      mkdir -p "$O/$name-wt"
      if [ -z "$branch" ] || [ "$branch" = " detached" ] || [ "$branch" = "detached" ]; then
        g -C "$r" worktree add -q --no-checkout --detach "$wt" "$(g -C "$r" rev-parse "$head^{commit}")"
      else
        g -C "$r" worktree add -q --no-checkout "$wt" "$branch"
      fi
      if [ "$dirty" = "0" ]; then
        g -C "$wt" sparse-checkout set --no-cone '/.replay-nothing' >/dev/null 2>&1 || true
      fi
      g -C "$wt" reset -q --hard
    fi
    if [ "$dirty" != "0" ]; then
      local stem=$name-wt-$base
      [ "$path" = "$main" ] && stem=$name-wt-main
      if [ -s "$B/$stem.patch" ]; then
        g -C "$wt" apply --whitespace=nowarn "$B/$stem.patch" || log "patch did not apply: $stem"
      fi
      if [ -s "$B/$stem.tgz" ]; then
        tar -xzf "$B/$stem.tgz" -C "$wt" || log "untracked files did not extract: $stem"
      fi
    fi
  done
}

# held NAME BUNDLE [BASE]: one held item's scratch clone. Its main and
# origin/main are the bundle's main, or BASE when the bundle carries none.
held() {
  local r=$O/held-$1
  rm -rf "$r"
  g init -q -b replay-unborn "$r"
  g -C "$r" fetch -q --update-head-ok "$2" 'refs/heads/*:refs/heads/*'
  if [ -n "${3:-}" ]; then
    g -C "$r" update-ref refs/heads/main "$3"
  fi
  g -C "$r" update-ref refs/remotes/origin/main refs/heads/main
  g -C "$r" checkout -q -f main
}

log "sextant"
rebuild sextant "$B/sextant-all-refs.bundle"
stashes sextant
worktrees sextant
log "agentic-kit"
rebuild agentic-kit "$B/agentic-kit-all-refs.bundle"
stashes agentic-kit
worktrees agentic-kit

log "held items"
held wa_kit "$B/scratch-bundles/held-wa_kit.bundle"
held agentsmd_kit "$B/scratch-bundles/held-agentsmd_kit.bundle"
r=$O/held-lq-wip-css
rm -rf "$r"
g init -q -b replay-unborn "$r"
g -C "$r" fetch -q "$B/scratch-bundles/held-lq-wip-css.bundle" 'refs/heads/*:refs/heads/*'
if [ -n "$CLONE" ]; then
  g -C "$r" fetch -q "$CLONE" "refs/remotes/origin/main:refs/replay/gh-main"
  mb=$(g -C "$r" merge-base refs/replay/gh-main 'refs/heads/wip-css/lang/css')
  g -C "$r" update-ref -d refs/replay/gh-main
  g -C "$r" update-ref refs/heads/main "$mb"
  g -C "$r" update-ref refs/remotes/origin/main "$mb"
  g -C "$r" checkout -q -f main
fi

for repo in sextant agentic-kit held-wa_kit held-agentsmd_kit held-lq-wip-css; do
  [ -d "$O/$repo/.git" ] || continue
  "$F" plans lines --root "$O/$repo" --json >"$O/$repo.lines.json"
  "$F" plans lines --root "$O/$repo" >"$O/$repo.lines.txt"
  log "$repo: $(wc -l <"$O/$repo.lines.txt") report lines"
done
