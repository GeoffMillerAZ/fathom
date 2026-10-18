#!/usr/bin/env bash
#
# intent-stale.sh — the bounded intent review (decision 0021): list the condition lines
# (Revisit when, Promote when, Drop if, wins if) whose last change is oldest, so a weekly
# review judges ten conditions instead of every one. git only; needs no fathom. Ships
# inside the intent-audit skill, so every install of the skill carries it.
#
# Usage:
#   bash <intent-audit skill dir>/scripts/intent-stale.sh [-n N] [path ...]
#
#   -n N    how many lines to print (default 10; 0 prints all)
#   path    roots to scan instead of the defaults: docs/intent, docs/adr and ADR.md,
#           whichever exist. Tracked *.md files under the roots are read.
#
# A condition line carries a label, case-insensitive, in one of these places:
#   - at the line start, after an optional quote, list marker, bold or italic:
#     `- **Drop if:**`, `- *Revisit when (added).*`;
#   - after a bold span: `- **Option.** Lost because ... Wins if: ...`;
#   - anywhere, followed by a colon: `... holds for now. Revisit when: ...`;
#   - anywhere in bold: `... lost on cost. **Revisit when.** ...`;
#   - opening a sentence or after a dash: `... I14. Revisit when ...`, `— wins if ...`;
#   - or the line starts a bullet or paragraph in a section headed by a label
#     (`## Revisit when`).
# A wrapped condition is dated by its first line, and a label split across two lines is
# not seen. Lines inside code fences, labels quoted in backticks or double quotes (a
# mention, not a condition) and table rows are skipped.
#
# Age is whole days from the author time of the commit that last changed the line
# (`git blame --line-porcelain`) to now (INTENT_STALE_NOW, epoch seconds, when set). git
# dates changes: a judged line keeps its age until an edit (its reading, a sharper
# wording) lands in a commit. Output, one row per line: `<age>d  <path>:<line>  <text>`,
# the text trimmed and cut to 120 characters, ordered by age descending, then path,
# then line. Exit 0 (a review aid, never a gate), 2 on a usage error.

set -euo pipefail
export LC_ALL=C

usage() {
  sed -n '8,13p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
}

limit=10
while [ $# -gt 0 ]; do
  case "$1" in
    -n)
      [ $# -ge 2 ] || usage
      case "$2" in '' | *[!0-9]*) usage ;; esac
      limit=$2
      shift 2
      ;;
    -h | --help) usage ;;
    --)
      shift
      break
      ;;
    -*) usage ;;
    *) break ;;
  esac
done

top=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "intent-stale: not a git repository; nothing to list" >&2
  exit 0
}

# Roots are repository-relative when given from a subdirectory.
prefix=$(git rev-parse --show-prefix)
roots=()
if [ $# -gt 0 ]; then
  for r in "$@"; do
    case "$r" in /*) roots+=("$r") ;; *) roots+=("${prefix}${r}") ;; esac
  done
else
  for r in docs/intent docs/adr ADR.md; do
    [ -e "$top/$r" ] && roots+=("$r")
  done
fi
cd "$top"
[ "${#roots[@]}" -gt 0 ] || exit 0

now=${INTENT_STALE_NOW:-$(date +%s)}
case "$now" in '' | *[!0-9]*)
  echo "intent-stale: INTENT_STALE_NOW must be epoch seconds" >&2
  exit 2
  ;;
esac

# condition_lines <file>: the line numbers of the file's condition lines, one per line.
condition_lines() {
  awk '
    BEGIN { label = "(revisit when|promote when|drop if|wins if)"; blank = 1 }
    {
      line = tolower($0)
      if (line ~ /^[ \t]*(```|~~~)/) { fence = !fence; blank = 1; next }
      if (fence) next
      if (line ~ /^[ \t]*$/) { blank = 1; next }
      start = blank; blank = 0
      if (line ~ /^[ \t]*#/) {
        section = (line ~ ("^[ \t]*#+[ \t]+(\\*\\*)?" label "([^a-z]|$)"))
        blank = 1
        next
      }
      if (line ~ /^[ \t]*\|/) next
      if (section) {
        if (start || line ~ /^[ \t]*([-*+]|[0-9]+[.)])[ \t]/) print NR
        next
      }
      if (line ~ ("^[ \t]*(>[ \t]*)?(([-*+]|[0-9]+[.)])[ \t]+)?(\\*\\*?|__)?[ \t]*" label "([^a-z]|$)")) { print NR; next }
      if (line ~ ("\\*\\*[^*]+\\*\\*(.*[^a-z`])?" label "([^a-z`]|$)")) { print NR; next }
      if (line ~ ("(^|[^a-z`])" label "(\\*\\*)?[ \t]*:")) { print NR; next }
      if (line ~ ("(^|[^`*_])(\\*\\*|__)[ \t]*" label "([^a-z`]|$)")) { print NR; next }
      if (line ~ ("(\\.|;|—|–)[ \t]+" label "([^a-z`]|$)")) { print NR; next }
    }
  ' "$1"
}

TAB=$(printf '\t')
rows=$(
  git ls-files -z -- "${roots[@]}" | tr '\0' '\n' | { grep -E '\.md$' || true; } | while IFS= read -r f; do
    [ -f "$f" ] || continue
    nums=$(condition_lines "$f")
    [ -n "$nums" ] || continue
    args=()
    for n in $nums; do args+=(-L "$n,$n"); done
    git blame --line-porcelain "${args[@]}" -- "$f" | awk -v now="$now" -v path="$f" '
      /^[0-9a-f]+ [0-9]+ [0-9]+/ { lineno = $3; next }
      /^author-time / { t = $2; next }
      /^\t/ {
        text = substr($0, 2)
        sub(/^[ \t]+/, "", text); sub(/[ \t\r]+$/, "", text)
        gsub(/\t/, " ", text)
        age = int((now - t) / 86400); if (age < 0) age = 0
        printf "%d\t%s\t%d\t%s\n", age, path, lineno, text
      }
    '
  done | sort -t "$TAB" -k1,1nr -k2,2 -k3,3n
)
[ -n "$rows" ] || exit 0

# cut120 <text>: the first 120 characters of UTF-8 text, never splitting one (bytes in
# 0x80..0xBF continue a character; every other byte starts one).
cut120() {
  local s=$1 out="" n=0 i c
  for ((i = 0; i < ${#s}; i++)); do
    c=${s:i:1}
    if [[ "$c" < $'\x80' || "$c" > $'\xbf' ]]; then
      n=$((n + 1))
      [ "$n" -le 120 ] || break
    fi
    out+=$c
  done
  printf '%s' "$out"
}

count=0
while IFS="$TAB" read -r age path lineno text; do
  if [ "$limit" -gt 0 ] && [ "$count" -ge "$limit" ]; then break; fi
  printf '%4sd  %s:%s  %s\n' "$age" "$path" "$lineno" "$(cut120 "$text")"
  count=$((count + 1))
done <<<"$rows"
exit 0
