#!/usr/bin/env bash
# Apply only the selected lock row's ordered, hash-pinned source patches.
set -euo pipefail
export LC_ALL=C
LOCK="${1:?usage: apply-patches.sh <lock> <row-json> <source-dir>}"
ROW="${2:?usage: apply-patches.sh <lock> <row-json> <source-dir>}"
SOURCE="${3:?usage: apply-patches.sh <lock> <row-json> <source-dir>}"
LOCKDIR="$(cd "$(dirname "$LOCK")" && pwd -P)"
SOURCE="$(cd "$SOURCE" && pwd -P)"

command -v jq >/dev/null || { echo "apply-patches.sh: jq is required" >&2; exit 2; }
# A tarball source must not supply a repository config that changes git apply.
if [ -e "$SOURCE/.git" ] || [ -L "$SOURCE/.git" ]; then
  echo "apply-patches.sh: source has a .git entry" >&2; exit 1
fi
jq -es 'length == 1 and (.[0] | type == "object" and (.licence | type == "string") and
  ((if has("patches") and .patches != null then .patches else [] end) as $p |
    ($p | type) == "array" and all($p[]; type == "object" and (.path | type == "string") and (.sha256 | type == "string"))))' <<<"$ROW" >/dev/null || {
  echo "apply-patches.sh: invalid selected row or patch entries" >&2; exit 1
}
if [ "$(jq -r '.licence' <<<"$ROW")" = "MPL-2.0" ] && [ "$(jq -r '(.patches // []) | length' <<<"$ROW")" != 0 ]; then
  echo "apply-patches.sh: MPL-2.0 sources must remain unmodified" >&2; exit 1
fi
GENERATE="$(jq -r '.generate // false' <<<"$ROW")"
if [ "$GENERATE" != true ] && [ "$GENERATE" != false ]; then
  echo "apply-patches.sh: generate must be boolean" >&2; exit 1
fi
SCANNER="$(jq -r '.scanner // false' <<<"$ROW")"
if [ "$SCANNER" != true ] && [ "$SCANNER" != false ]; then
  echo "apply-patches.sh: scanner must be boolean" >&2; exit 1
fi

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-grammar-patches.XXXXXXXX")"
trap 'rm -rf "$scratch"' EXIT

# Strip every ambient Git setting. The ceiling prevents discovering an
# ancestor checkout; the source itself cannot have a .git entry.
git_apply() {
  (cd "$SOURCE" && env -i PATH="$PATH" TMPDIR="$scratch" \
    GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null \
    GIT_CEILING_DIRECTORIES="$(dirname "$SOURCE")" \
    git -c core.autocrlf=false -c core.eol=lf -c apply.whitespace=nowarn \
    -c apply.ignoreWhitespace=false \
    apply --whitespace=nowarn "$@" "$staged")
}

check_target() {
  local target="$1" part current base upper lower subdir prefix relative allowed=0
  local -a parts
  if [[ ! "$target" =~ ^[A-Za-z0-9._/-]+$ ]] || [[ "$target" == /* || "$target" == */ || "$target" == *//* ]]; then
    echo "apply-patches.sh: invalid source target '$target'" >&2; return 1
  fi
  IFS=/ read -r -a parts <<<"$target"
  current="$SOURCE"
  for part in "${parts[@]}"; do
    if [ "$part" = "." ] || [ "$part" = ".." ] || [ -z "$part" ]; then
      echo "apply-patches.sh: noncanonical source target '$target'" >&2; return 1
    fi
    current="$current/$part"
    if [ -L "$current" ]; then
      echo "apply-patches.sh: symlink in source target '$target'" >&2; return 1
    fi
  done
  base="${target##*/}"
  upper="$(printf '%s' "$base" | tr '[:lower:]' '[:upper:]')"
  lower="$(printf '%s' "$target" | tr '[:upper:]' '[:lower:]')"
  case "$upper" in
    LICENSE*|LICENCE*|COPYING*|NOTICE*|COPYRIGHT*) echo "apply-patches.sh: licence/attribution target '$target' refused" >&2; return 1 ;;
  esac
  if [ "$GENERATE" = true ]; then
    case "$lower" in
      src/parser.c|*/src/parser.c|src/grammar.json|*/src/grammar.json|src/node-types.json|*/src/node-types.json)
        echo "apply-patches.sh: generated target '$target' refused" >&2; return 1 ;;
    esac
  else
    # Without generation, only inputs selected by build.sh's C compiler can
    # be patched. Headers may be included by the parser or scanner.
    while IFS= read -r subdir; do
      if [ "$subdir" = . ]; then prefix=src/; else prefix="$subdir/src/"; fi
      if [[ "$target" != "$prefix"* ]]; then continue; fi
      relative="${target#"$prefix"}"
      case "$relative" in
        parser.c) allowed=1 ;;
        scanner.c) if [ "$SCANNER" = true ]; then allowed=1; fi ;;
        *.h) allowed=1 ;;
      esac
    done < <(jq -r '(.subdirs // []) | if length == 0 then ["."] else . end | .[]' <<<"$ROW")
    if [ "$allowed" != 1 ]; then
      echo "apply-patches.sh: target '$target' is not compiled without generation" >&2; return 1
    fi
  fi
  if [ -e "$current" ] && [ ! -f "$current" ]; then
    echo "apply-patches.sh: nonregular source target '$target'" >&2; return 1
  fi
}

seen=$'\n'
index=0
while IFS= read -r entry; do
  path="$(jq -r .path <<<"$entry")"
  want="$(jq -r .sha256 <<<"$entry")"
  if [[ ! "$path" =~ ^patches/([A-Za-z0-9._-]+/)*[A-Za-z0-9._-]+\.patch$ ]] || [[ "$path" == *"/../"* || "$path" == *"/./"* || "$path" == *"//"* || "$path" == *"\\"* ]]; then
    echo "apply-patches.sh: invalid patch path '$path'" >&2; exit 1
  fi
  IFS=/ read -r -a components <<<"$path"
  candidate="$LOCKDIR"
  for component in "${components[@]}"; do
    if [ "$component" = "." ] || [ "$component" = ".." ]; then echo "apply-patches.sh: noncanonical patch path '$path'" >&2; exit 1; fi
    candidate="$candidate/$component"
    if [ -L "$candidate" ]; then echo "apply-patches.sh: symlink in patch path '$path'" >&2; exit 1; fi
  done
  [ -f "$candidate" ] || { echo "apply-patches.sh: patch '$path' missing" >&2; exit 1; }
  [[ "$want" =~ ^[0-9a-f]{64}$ ]] || { echo "apply-patches.sh: invalid sha256 for '$path'" >&2; exit 1; }
  case "$seen" in *$'\n'"$path"$'\n'*) echo "apply-patches.sh: duplicate patch '$path'" >&2; exit 1 ;; esac
  seen+="$path"$'\n'
  # One read of the original: hash, inspect, check and apply the private copy.
  staged="$scratch/$index.patch"
  cp "$candidate" "$staged"
  got="$(shasum -a 256 "$staged" | cut -c1-64)"
  [ "$got" = "$want" ] || { echo "apply-patches.sh: patch '$path' sha256 $got, lock $want" >&2; exit 1; }
  git_apply --numstat -z > "$scratch/numstat"
  git_apply --summary > "$scratch/summary"
  # Only ordinary regular-file additions may emit summary lines.
  while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*create[[:space:]]mode[[:space:]]100644[[:space:]][A-Za-z0-9._/-]+$ ]] || {
      echo "apply-patches.sh: patch '$path' changes file metadata" >&2; exit 1
    }
  done < "$scratch/summary"
  count=0
  : > "$scratch/before"
  while IFS= read -r -d '' record; do
    [[ "$record" == *$'\t'*$'\t'* ]] || { echo "apply-patches.sh: malformed patch numstat" >&2; exit 1; }
    added="${record%%$'\t'*}"
    rest="${record#*$'\t'}"
    removed="${rest%%$'\t'*}"
    target="${rest#*$'\t'}"
    [[ "$added" =~ ^[0-9]+$ && "$removed" =~ ^[0-9]+$ ]] || {
      echo "apply-patches.sh: binary patch refused" >&2; exit 1
    }
    check_target "$target"
    if [ -f "$SOURCE/$target" ]; then before="$(shasum -a 256 "$SOURCE/$target" | cut -c1-64)"; else before=absent; fi
    printf '%s\t%s\n' "$target" "$before" >> "$scratch/before"
    count=$((count + 1))
  done < "$scratch/numstat"
  [ "$count" -gt 0 ] || { echo "apply-patches.sh: empty patch '$path' refused" >&2; exit 1; }
  git_apply --check || { echo "apply-patches.sh: patch '$path' does not apply" >&2; exit 1; }
  git_apply
  changed=0
  while IFS=$'\t' read -r target before; do
    if [ -f "$SOURCE/$target" ]; then after="$(shasum -a 256 "$SOURCE/$target" | cut -c1-64)"; else after=absent; fi
    if [ "$before" != "$after" ]; then changed=1; fi
  done < "$scratch/before"
  [ "$changed" = 1 ] || { echo "apply-patches.sh: no-change patch '$path' refused" >&2; exit 1; }
  index=$((index + 1))
done < <(jq -c '.patches // [] | .[]' <<<"$ROW")
