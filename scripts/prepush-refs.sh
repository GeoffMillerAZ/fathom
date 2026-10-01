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

decide() {
  rm -f "$marker"
  local lref lsha rref rsha refs=0 commit
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
        [ -n "$(git for-each-ref --count=1 --contains "$commit" refs/remotes 2>/dev/null)" ] || return 1
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

case "${1:-}" in
  decide)
    if [ -t 0 ]; then rm -f "$marker"; exit 1; fi
    decide
    ;;
  skipped) skipped ;;
  *)
    echo "usage: prepush-refs.sh decide < refs | skipped" >&2
    exit 2
    ;;
esac
