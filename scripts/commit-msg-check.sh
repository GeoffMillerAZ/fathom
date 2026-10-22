#!/usr/bin/env bash
# BE-2: the same message policy for lefthook and each commit in a CI push range.
set -euo pipefail
[ "$#" -eq 1 ] || { echo 'usage: commit-msg-check.sh MESSAGE_FILE' >&2; exit 2; }
merge=0
merge_head=$(git rev-parse --git-path MERGE_HEAD 2>/dev/null) || merge_head=""
[ -z "$merge_head" ] || [ ! -f "$merge_head" ] || merge=1
LC_ALL=C awk -v merge="$merge" '
  { sub(/\r$/, "") }
  $0 == "# ------------------------ >8 ------------------------" { exit }
  /^#/ { next }
  !subject {
    subject=1
    special=(merge || $0 ~ /^([Mm]erge[: ]|Revert |fixup! |squash! )[[:blank:]]*[^[:space:]]/)
    exempt=(special || $0 ~ /^revert(\([^()]+\))?!?: [^[:space:]]/)
    if (!special && $0 !~ /^(bench|build|calibration|chore|ci|docs|feat|fix|perf|refactor|release|revert|style|test)(\([^()]+\))?!?: [^[:space:]].*$/) {
      print "commit-msg: subject must use a conventional commit type" > "/dev/stderr"
      bad=1
    }
  }
  NR > 1 && /^Verified:[[:blank:]]*[^[:space:]]/ { verified=1 }
  tolower($0) ~ /^[[:blank:]]*(co-authored-by|claude-session):/ {
    print "commit-msg: forbidden provenance trailer at line " NR > "/dev/stderr"
    bad=1
  }
  END {
    if (!subject) { print "commit-msg: empty message" > "/dev/stderr"; bad=1 }
    if (!exempt && !verified) {
      print "commit-msg: body requires a nonempty Verified: line" > "/dev/stderr"
      bad=1
    }
    exit bad
  }
' "$1"
