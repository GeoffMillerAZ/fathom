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
# A condition is a paragraph: its label line through the line before the next blank line,
# heading, table row, fence, other condition line, or list item at the same or a shallower
# indent. A label split across two lines is not seen. A condition inside a superseded
# record (its first `Status` line, `**Status.**`, `Status:` or `*Status: ...*`, starts with
# "superseded"; a record is a `## ` section, or the whole file when `Status` precedes its
# first `## `) is closed and not listed; rejected and deferred records are kept. Lines inside code fences, labels quoted in backticks or double quotes (a
# mention, not a condition) and table rows are skipped.
#
# Age is whole days from the author time of the newest commit that changed any line of the
# paragraph (`git blame --line-porcelain`), so a later disposition sentence resets it, to now (INTENT_STALE_NOW, epoch seconds, when set). git
# dates changes: a judged condition keeps its age until an edit (its reading, a sharper
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

# condition_lines <file>: one `<first> <last>` line-number pair per condition, one per line:
# the label line through the line before the next blank line, heading, table row, fence,
# other condition line, or list item at the same or a shallower indent. A condition inside
# a superseded record is not listed. A record is an `# ` or `## ` section whose first
# `Status` line (`**Status.** ...`, `**Status:** ...`, `Status: ...`, `*Status: ...*`)
# starts with "superseded"; a Status line ahead of the first `## ` heading under a `# `
# heading belongs to the whole `# ` section (a per-file record), any other to its `## `.
condition_lines() {
  awk '
    function indent(s) { match(s, /^[ \t]*/); return RLENGTH }
    function close_open() { if (open) { print open, FNR - 1; open = 0 } }
    BEGIN { label = "(revisit when|promote when|drop if|wins if)"; blank = 1 }
    FNR == 1 { pass = (pass == "") ? 1 : 2; fence = 0; blank = 1; section = 0; h1 = 0; h2 = 0; seen1 = 0; seen2 = 0 }
    pass == 1 {
      line = tolower($0)
      if (line ~ /^[ \t]*(```|~~~)/) { fence = !fence; next }
      if (fence) next
      if (line ~ /^#[ \t]/) { h1++; h2 = 0; cur1 = h1; cur2 = ""; seen1 = 0; seen2 = 0; inh1 = 1; next }
      if (line ~ /^##[ \t]/) { h2++; cur2 = h1 "." h2; seen2 = 0; inh1 = 0; next }
      if (line ~ /^[ \t>*_-]*status[.:]?[*_]*[ \t:.]/) {
        sup = (line ~ /^[ \t>*_-]*status[.:]?[*_]*[ \t:.]*superseded/)
        if (inh1 || cur2 == "") { if (!seen1) { seen1 = 1; if (sup) dead1[cur1] = 1 } }
        else if (!seen2) { seen2 = 1; if (sup) dead2[cur2] = 1 }
      }
      lh1[FNR] = cur1; lh2[FNR] = cur2
      next
    }
    {
      line = tolower($0)
      if (line ~ /^[ \t]*(```|~~~)/) { close_open(); fence = !fence; blank = 1; next }
      if (fence) next
      if (line ~ /^[ \t]*$/) { close_open(); blank = 1; next }
      start = blank; blank = 0
      if (line ~ /^[ \t]*#/) {
        close_open()
        section = (line ~ ("^[ \t]*#+[ \t]+(\\*\\*)?" label "([^a-z]|$)"))
        blank = 1
        next
      }
      if (line ~ /^[ \t]*\|/) { close_open(); next }
      item = (line ~ /^[ \t]*([-*+]|[0-9]+[.)])[ \t]/)
      iscond = 0
      if (section) { if (start || item) iscond = 1 }
      else if (line ~ ("^[ \t]*(>[ \t]*)?(([-*+]|[0-9]+[.)])[ \t]+)?(\\*\\*?|__)?[ \t]*" label "([^a-z]|$)")) iscond = 1
      else if (line ~ ("\\*\\*[^*]+\\*\\*(.*[^a-z`])?" label "([^a-z`]|$)")) iscond = 1
      else if (line ~ ("(^|[^a-z`])" label "(\\*\\*)?[ \t]*:")) iscond = 1
      else if (line ~ ("(^|[^`*_])(\\*\\*|__)[ \t]*" label "([^a-z`]|$)")) iscond = 1
      else if (line ~ ("(\\.|;|—|–)[ \t]+" label "([^a-z`]|$)")) iscond = 1
      if (open && (iscond || (item && indent($0) <= openind))) close_open()
      if (iscond) {
        if (!(lh1[FNR] in dead1) && !(lh2[FNR] != "" && (lh2[FNR] in dead2))) { open = FNR; openind = indent($0) }
      }
    }
    END { if (pass == 2 && open) print open, FNR }
  ' "$1" "$1"
}

TAB=$(printf '\t')
rows=$(
  git ls-files -z -- "${roots[@]}" | tr '\0' '\n' | { grep -E '\.md$' || true; } | while IFS= read -r f; do
    [ -f "$f" ] || continue
    ranges=$(condition_lines "$f")
    [ -n "$ranges" ] || continue
    args=()
    while read -r first last; do args+=(-L "$first,$last"); done <<<"$ranges"
    git blame --line-porcelain "${args[@]}" -- "$f" | awk -v now="$now" -v path="$f" -v ranges="${ranges//$'\n'/;}" '
      BEGIN {
        n = split(ranges, rl, ";")
        for (i = 1; i <= n; i++) { split(rl[i], p, " "); lo[i] = p[1]; hi[i] = p[2] }
      }
      /^[0-9a-f]+ [0-9]+ [0-9]+/ { lineno = $3; next }
      /^author-time / { t = $2; next }
      /^\t/ {
        for (i = 1; i <= n; i++) if (lineno >= lo[i] && lineno <= hi[i]) {
          if (t > newest[i]) newest[i] = t
          if (lineno == lo[i]) {
            text = substr($0, 2)
            sub(/^[ \t]+/, "", text); sub(/[ \t\r]+$/, "", text)
            gsub(/\t/, " ", text)
            first[i] = text
          }
        }
      }
      END {
        for (i = 1; i <= n; i++) {
          age = int((now - newest[i]) / 86400); if (age < 0) age = 0
          printf "%d\t%s\t%d\t%s\n", age, path, lo[i], first[i]
        }
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
