#!/usr/bin/env bash
# fetch.sh DEST -- check out every subject of subjects.tsv at its pinned commit under DEST, sparse (only the subject's
# directory), one clone per repository. Idempotent. Needs git only.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
dest=${1:?usage: fetch.sh DEST}
mkdir -p "$dest"
dest=$(cd "$dest" && pwd)
grep -v '^#' "$here/subjects.tsv" | while IFS=$'\t' read -r id tool kind repo sha path; do
  d="$dest/${repo//\//_}"
  if [ ! -d "$d/.git" ]; then
    git init -q "$d"
    git -C "$d" remote add origin "https://github.com/$repo.git"
    git -C "$d" sparse-checkout init --cone
  fi
  if ! git -C "$d" cat-file -e "$sha^{commit}" 2>/dev/null; then
    git -C "$d" fetch -q --depth 1 --filter=blob:none origin "$sha"
  fi
  dir=$path
  [ "$tool" = terraform ] || dir=$(dirname "$path")
  if [ "$dir" = . ]; then git -C "$d" sparse-checkout disable; else git -C "$d" sparse-checkout add "$dir"; fi
  git -C "$d" checkout -q --detach "$sha"
done
