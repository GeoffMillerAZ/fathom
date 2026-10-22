#!/usr/bin/env bash
# prepush-refs.sh — the pre-push gate's "does this push carry a new commit?"
# decision, read ONCE from the ref lines git feeds the hook on stdin
# ("<local ref> <local sha> <remote ref> <remote sha>" per pushed ref).
#
# A push that carries no commit new to the remote needs neither validate nor
# verify: every pushed ref is a delete, or a tag whose commit a remote-tracking
# ref already holds (the tag after the release commit's own push). Anything
# else — a branch, a tag on a commit the remote lacks, an unreadable line, no
# stdin at all — keeps the full gate.
#
# Lefthook 1.13.6 hands the hook's stdin to ONE command: two `use_stdin: true`
# commands deadlock (measured). So the verify step (priority 1, the one stdin
# reader) runs `decide` first and leaves a marker; the validate step asks
# `skipped`. `decide` always rewrites the marker, so a full gate never reads a
# previous push's verdict; `skipped` also requires the marker to name the
# current HEAD and be under an hour old.
#
#   prepush-refs.sh decide    < ref lines   exit 0: skip the gate (marker written, reason printed); 1: full gate
#   prepush-refs.sh skipped                 exit 0: the marker says skip
#
# Runs under macOS bash 3.2. PREPUSH_SKIP_MARKER overrides the marker path.

set -u

marker="${PREPUSH_SKIP_MARKER:-workspace/tmp/prepush-skip}"
zero_re='^0+$'
refs_marker="${marker}.refs"

# decide REMOTE < refs. Only the remote-tracking refs of REMOTE (the name git
# hands pre-push as $1) vouch that it holds a commit: a stale ref of another
# remote proves nothing about this one. No name, an empty one or one with a
# slash (a URL) keeps the full gate.
decide() {
  rm -f "$marker"
  local remote="${1:-}" lref lsha rref rsha refs=0 commit
  case "$remote" in
    "" | */*) return 1 ;;
  esac
  while read -r lref lsha rref rsha; do
    [ -n "${lsha:-}" ] || continue
    refs=$((refs + 1))
    if [[ "$lsha" =~ $zero_re ]]; then
      continue # a delete carries no commit
    fi
    case "$lref" in
      refs/tags/*)
        commit=$(git rev-parse --verify -q "${lsha}^{commit}" 2>/dev/null) || return 1
        [ -n "$commit" ] || return 1
        [ -n "$(git for-each-ref --count=1 --contains "$commit" "refs/remotes/${remote}/" 2>/dev/null)" ] || return 1
        ;;
      *)
        return 1 # a branch (or anything else) is real work
        ;;
    esac
  done
  [ "$refs" -gt 0 ] || return 1 # no ref lines: stdin was a terminal or absent
  mkdir -p "$(dirname "$marker")" 2>/dev/null || return 1
  printf '%s %s\n' "$(git rev-parse HEAD 2>/dev/null)" "$(date +%s)" >"$marker" || return 1
  echo "pre-push: no commit is new to the remote (tag-only or delete push) — skipping validate and verify"
  return 0
}

skipped() {
  local head ts now
  [ -f "$marker" ] || return 1
  read -r head ts <"$marker" || return 1
  [ "$head" = "$(git rev-parse HEAD 2>/dev/null)" ] || return 1
  now=$(date +%s)
  [[ "$ts" =~ ^[0-9]+$ ]] || return 1
  [ $((now - ts)) -lt 3600 ] || return 1
  return 0
}

# Preserve every non-delete local SHA before decide's early full-gate return.
# A missing, malformed or stale handoff always keeps validate.
save_receipt_trees() {
  local lref lsha rref rsha extra count=0 trees="" head
  while read -r lref lsha rref rsha extra; do
    [ -n "${lref:-}${lsha:-}${rref:-}${rsha:-}" ] || continue
    [ -n "${lref:-}" ] && [ -n "${rref:-}" ] && [ -n "${rsha:-}" ] && [ -z "${extra:-}" ] || return 1
    [[ "$lsha" =~ ^[0-9a-f]{40}$ || "$lsha" =~ ^[0-9a-f]{64}$ ]] || return 1
    [[ "$rsha" =~ ^[0-9a-f]{40}$ || "$rsha" =~ ^[0-9a-f]{64}$ ]] || return 1
    [[ "$lsha" =~ $zero_re ]] && continue
    git rev-parse --verify -q "${lsha}^{commit}" >/dev/null 2>&1 || return 1
    trees="${trees}${lsha}
"
    count=$((count + 1))
  done
  [ "$count" -gt 0 ] || return 1
  head=$(git rev-parse HEAD 2>/dev/null) || return 1
  mkdir -p "$(dirname "$refs_marker")" || return 1
  { printf '%s %s\n' "$head" "$(date +%s)"; printf '%s' "$trees" | LC_ALL=C sort -u; } >"$refs_marker"
}

receipt_trees() {
  local head ts now sha trees=""
  [ -f "$refs_marker" ] || return 1
  {
    read -r head ts || return 1
    [ "$head" = "$(git rev-parse HEAD 2>/dev/null)" ] || return 1
    [[ "$ts" =~ ^[0-9]+$ ]] || return 1
    now=$(date +%s)
    [ $((now - ts)) -ge 0 ] && [ $((now - ts)) -lt 3600 ] || return 1
    while read -r sha; do
      [[ "$sha" =~ ^[0-9a-f]{40}$ || "$sha" =~ ^[0-9a-f]{64}$ ]] || return 1
      trees="${trees}${sha}
"
    done
  } <"$refs_marker"
  [ -n "$trees" ] || return 1
  printf '%s' "$trees"
}

case "${1:-}" in
  decide)
    rm -f "$marker" "$refs_marker"
    if [ -t 0 ]; then exit 1; fi
    refs=$(cat) || exit 1
    if ! printf '%s\n' "$refs" | save_receipt_trees; then rm -f "$refs_marker"; fi
    printf '%s\n' "$refs" | decide "${2:-}"
    ;;
  skipped) skipped ;;
  receipt-trees) receipt_trees ;;
  *)
    echo "usage: prepush-refs.sh decide REMOTE < refs | skipped | receipt-trees" >&2
    exit 2
    ;;
esac
