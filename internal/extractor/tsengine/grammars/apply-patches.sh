#!/usr/bin/env bash
# Apply only the lock row's ordered, hash-pinned in-tree source patches.
set -euo pipefail
LOCK="${1:?usage: apply-patches.sh <lock> <blob> <source-dir>}"
BLOB="${2:?usage: apply-patches.sh <lock> <blob> <source-dir>}"
SOURCE="${3:?usage: apply-patches.sh <lock> <blob> <source-dir>}"
LOCKDIR="$(cd "$(dirname "$LOCK")" && pwd -P)"
SOURCE="$(cd "$SOURCE" && pwd -P)"

command -v jq >/dev/null || { echo "apply-patches.sh: jq is required" >&2; exit 2; }
ROW="$(jq -ec --arg b "$BLOB" '[.grammars[] | select(.blob == $b)] | if length == 1 then .[0] else error("expected exactly one grammar row") end' "$LOCK")" || { echo "apply-patches.sh: expected exactly one row for $BLOB" >&2; exit 2; }
jq -e '(if has("patches") and .patches != null then .patches else [] end) as $p | (($p | type) == "array") and (if ($p | type) == "array" then all($p[]; (type == "object") and (.path | type == "string") and (.sha256 | type == "string")) else false end)' <<<"$ROW" >/dev/null || {
  echo "apply-patches.sh: patches must be an array of path/sha256 string objects" >&2; exit 1
}
if [ "$(jq -r '.licence' <<<"$ROW")" = "MPL-2.0" ] && [ "$(jq -r '(.patches // []) | length' <<<"$ROW")" != 0 ]; then
  echo "apply-patches.sh: MPL-2.0 sources must remain unmodified" >&2; exit 1
fi

seen=$'\n'
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
  got="$(shasum -a 256 "$candidate" | cut -c1-64)"
  [ "$got" = "$want" ] || { echo "apply-patches.sh: patch '$path' sha256 $got, lock $want" >&2; exit 1; }
  # Stop repository discovery at the source's parent. Otherwise git apply
  # interprets paths at an ancestor checkout root and edits the wrong tree.
  (cd "$SOURCE" && env -u GIT_DIR -u GIT_WORK_TREE GIT_CEILING_DIRECTORIES="$(dirname "$SOURCE")" git apply --check "$candidate") || { echo "apply-patches.sh: patch '$path' does not apply" >&2; exit 1; }
  (cd "$SOURCE" && env -u GIT_DIR -u GIT_WORK_TREE GIT_CEILING_DIRECTORIES="$(dirname "$SOURCE")" git apply "$candidate")
done < <(jq -c '.patches // [] | .[]' <<<"$ROW")
