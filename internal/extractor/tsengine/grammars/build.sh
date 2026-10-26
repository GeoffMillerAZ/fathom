#!/usr/bin/env bash
# build.sh — build ONE grammar blob from grammars.lock.json (LB-F28,
# docs/design/language-breadth.md §8.1).
#
# Each blob is tree-sitter's pure-C core runtime (lib/src/lib.c, the official
# amalgamation) plus budget.c (this directory) statically linked with ONE
# grammar repository's parser.c (+ scanner.c) into a single, self-contained
# wasm32-wasi "reactor" module — no dynamic linking, no Emscripten JS glue,
# no CGO. This is deliberately NOT the web-tree-sitter/tree-sitter-wasms
# "side module" shape (ADR-048 LB-X1; PROVENANCE.md's "path not taken").
#
# Everything the build needs comes from the lock: the core and grammar repo,
# tag, commit and tarball sha256, the licence, the entry points, the grammar
# subdirectories, whether a scanner exists and whether the grammar must be
# generated first. The script refuses a tarball whose sha256 or embedded
# commit differs from the lock and a licence that is not the lock's or not
# on the ADR-048 allowlist, then prints the three numbers the lock row
# records (blob_sha256, blob_bytes, gz_bytes) for pasting back.
#
# Requires (build-time only — NOT a runtime or `go build` dependency):
#   zig (the lock's toolchain.zig), wasm-opt (toolchain.wasm_opt), jq, curl,
#   git (get-tar-commit-id), gzip; npx only for a "generate": true grammar.
# Byte-identical output across other zig/binaryen versions is not claimed;
# the lock's digests bind the committed bytes, its pins bind the sources.
#
# Usage: ./build.sh <blob> [outdir]      (outdir defaults to this directory)
# Writes <outdir>/<blob>.wasm.gz. Needs network access to fetch the pinned
# tarballs; never part of `task validate`. After a rebuild, update the lock
# row, then regenerate PROVENANCE.md's tables:
#   go test ./internal/extractor/tsengine -run TestProvenanceRenderedFromLock -update-provenance
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCK="$HERE/grammars.lock.json"
BLOB="${1:?usage: build.sh <blob> [outdir]}"
OUT="$(cd "${2:-$HERE}" && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

command -v jq >/dev/null || { echo "build.sh: jq is required" >&2; exit 2; }
ROW="$(jq -e --arg b "$BLOB" '.grammars[] | select(.blob == $b)' "$LOCK")" ||
  { echo "build.sh: no grammars.lock.json row for blob '$BLOB'" >&2; exit 2; }
field() { jq -r "$1" <<<"$ROW"; }

# The ADR-048 licence allowlist (design §8.2), as SPDX ids.
ALLOWED=" MIT Apache-2.0 BSD-2-Clause BSD-3-Clause ISC Unlicense CC0-1.0 Zlib 0BSD MPL-2.0 "

# spdx_of <file>: the SPDX id a licence file's text states, or "unclear".
spdx_of() {
  local f="$1"
  if grep -qiE 'GNU (AFFERO|LESSER|LIBRARY)? ?GENERAL PUBLIC' "$f"; then echo "GPL-family"
  elif grep -qiE 'Server Side Public License|Business Source License|Functional Source License|PolyForm|Commons Clause|European Union Public' "$f"; then echo "refused-family"
  elif grep -qiE 'Mozilla Public License,? (Version )?2\.0' "$f"; then echo "MPL-2.0"
  elif grep -qiE 'Apache License' "$f" && grep -qE 'Version 2\.0' "$f"; then echo "Apache-2.0"
  elif grep -qiE 'Permission is hereby granted, free of charge' "$f"; then echo "MIT"
  elif grep -qiE 'Permission to use, copy, modify, and/or distribute' "$f"; then
    if grep -qiE 'copyright notice and this permission notice appear' "$f"; then echo "ISC"; else echo "0BSD"; fi
  elif grep -qiE 'Redistribution and use in source and binary forms' "$f"; then
    if grep -qiE 'endorse or promote' "$f"; then echo "BSD-3-Clause"; else echo "BSD-2-Clause"; fi
  elif grep -qiE 'This is free and unencumbered software released into the public domain' "$f"; then echo "Unlicense"
  elif grep -qiE 'CC0 1\.0 Universal' "$f"; then echo "CC0-1.0"
  elif grep -qiE "This software is provided 'as-is'" "$f"; then echo "Zlib"
  else echo "unclear"
  fi
}

# check_licence <srcdir> <spdx>: the repository's top-level licence AND any
# licence file under queries/ and src/ must state the lock's SPDX id, and it
# must be on the allowlist.
check_licence() {
  local dir="$1" want="$2" f got found=0
  case "$ALLOWED" in *" $want "*) ;; *) echo "build.sh: licence '$want' is not on the ADR-048 allowlist" >&2; exit 1 ;; esac
  while IFS= read -r f; do
    found=1
    got="$(spdx_of "$f")"
    if [ "$got" != "$want" ]; then
      echo "build.sh: ${f#"$dir"/} reads as '$got', the lock says '$want'; refused" >&2
      exit 1
    fi
  done < <( { find "$dir" -maxdepth 1 -type f \( -iname 'LICENSE*' -o -iname 'LICENCE*' -o -iname 'COPYING*' \);
              find "$dir" -mindepth 2 -type f \( -path '*/queries/*' -o -path '*/src/*' \) \( -iname 'LICENSE*' -o -iname 'LICENCE*' -o -iname 'COPYING*' \); } | sort)
  [ "$found" = 1 ] || { echo "build.sh: $dir has no top-level licence file; 'no licence' is refused" >&2; exit 1; }
}

# fetch <repo> <tag> <tarball_sha256> <commit> <destdir>
# A grammar repository that publishes no release tags (ngalaiko/tree-sitter-
# go-template) is pinned by its 40-hex commit id, which the lock's "tag" field
# then carries; the tarball is that commit's archive.
fetch() {
  local tgz="$WORK/$(basename "$1").tar.gz" got commit ref="refs/tags/$2"
  if [[ "$2" =~ ^[0-9a-f]{40}$ ]]; then ref="$2"; fi
  curl -sL --fail "https://github.com/$1/archive/$ref.tar.gz" -o "$tgz"
  got="$(shasum -a 256 "$tgz" | cut -c1-64)"
  [ "$got" = "$3" ] || { echo "build.sh: $1 $2 tarball sha256 $got, lock $3; refused" >&2; exit 1; }
  commit="$(gzip -dc "$tgz" | git get-tar-commit-id || true)"
  [ "$commit" = "$4" ] || { echo "build.sh: $1 $2 tarball is commit '$commit', lock $4; refused" >&2; exit 1; }
  mkdir -p "$5"
  tar xzf "$tgz" -C "$5" --strip-components=1
}

want_tool() { # want_tool <name> <have> <want>
  [ "$2" = "$3" ] || echo "build.sh: warning: $1 $2, the lock's toolchain says $3; output will not match the committed digests" >&2
}
want_tool zig "$(zig version 2>/dev/null || echo missing)" "$(jq -r .toolchain.zig "$LOCK")"
if command -v wasm-opt >/dev/null 2>&1; then
  want_tool wasm-opt "$(wasm-opt --version | awk '{print $3}')" "$(jq -r .toolchain.wasm_opt "$LOCK")"
fi

echo "fetching core $(jq -r .core.repo "$LOCK") $(jq -r .core.tag "$LOCK")..."
fetch "$(jq -r .core.repo "$LOCK")" "$(jq -r .core.tag "$LOCK")" \
  "$(jq -r .core.tarball_sha256 "$LOCK")" "$(jq -r .core.commit "$LOCK")" "$WORK/core"
check_licence "$WORK/core" "$(jq -r .core.licence "$LOCK")"

REPO="$(field .repo)"; TAG="$(field .tag)"
echo "fetching $REPO $TAG..."
fetch "$REPO" "$TAG" "$(field .tarball_sha256)" "$(field .commit)" "$WORK/grammar"
check_licence "$WORK/grammar" "$(field .licence)"

# Apply only lock-declared, hash-verified source changes after upstream
# archive/commit/licence checks and before optional parser generation.
bash "$HERE/apply-patches.sh" "$LOCK" "$BLOB" "$WORK/grammar"
check_licence "$WORK/grammar" "$(field .licence)"

SUBDIRS=()
while IFS= read -r d; do SUBDIRS+=("$d"); done < <(field '(.subdirs // []) | if length == 0 then ["."] else . end | .[]')

if [ "$(field .generate)" = "true" ]; then
  CLI="$(jq -r '.toolchain.tree_sitter_cli // empty' "$LOCK")"
  [ -n "$CLI" ] || { echo "build.sh: generate: true needs toolchain.tree_sitter_cli pinned in the lock" >&2; exit 2; }
  for d in "${SUBDIRS[@]}"; do
    (cd "$WORK/grammar/$d" && npx --yes "tree-sitter-cli@$CLI" generate)
  done
fi

GRAMMAR_ARGS=()
for d in "${SUBDIRS[@]}"; do
  src="$WORK/grammar/$d/src"
  GRAMMAR_ARGS+=(-I "$src" "$src/parser.c")
  if [ "$(field .scanner)" = "true" ]; then GRAMMAR_ARGS+=("$src/scanner.c"); fi
done
while IFS= read -r ep; do GRAMMAR_ARGS+=("-Wl,--export=$ep"); done < <(field '.entry_points[]')

# The exported C API surface internal/extractor/tsengine calls. Adding a
# capability = add its export here (and a lock bump for every blob).
EXPORTS=(
  -Wl,--export=malloc -Wl,--export=free -Wl,--export=calloc -Wl,--export=realloc
  -Wl,--export=ts_parser_new -Wl,--export=ts_parser_delete -Wl,--export=ts_parser_set_language
  -Wl,--export=ts_parser_parse_string -Wl,--export=ts_parser_reset
  -Wl,--export=fathom_parse_bounded
  -Wl,--export=fathom_query_cursor_exec_bounded
  -Wl,--export=fathom_take_cancelled
  -Wl,--export=fathom_node_end_byte
  -Wl,--export=ts_tree_root_node -Wl,--export=ts_tree_delete
  -Wl,--export=ts_node_child_count -Wl,--export=ts_node_named_child_count
  -Wl,--export=ts_node_child -Wl,--export=ts_node_named_child
  -Wl,--export=ts_node_type -Wl,--export=ts_node_symbol
  -Wl,--export=ts_node_start_byte -Wl,--export=ts_node_end_byte
  -Wl,--export=ts_node_start_point -Wl,--export=ts_node_end_point
  -Wl,--export=ts_node_is_named -Wl,--export=ts_node_is_null
  -Wl,--export=ts_node_field_name_for_child
  -Wl,--export=ts_language_symbol_name -Wl,--export=ts_language_abi_version
  -Wl,--export=ts_query_new -Wl,--export=ts_query_delete
  -Wl,--export=ts_query_cursor_new -Wl,--export=ts_query_cursor_delete
  -Wl,--export=ts_query_cursor_exec -Wl,--export=ts_query_cursor_next_match
  -Wl,--export=ts_query_capture_name_for_id
)
# The predicate export (§8.1), per lock row: the engine reads predicates
# host-side from the query source (predicates.go) and never requires it, so
# the blobs built before it keep "predicate_export": false and stay
# reproducible byte for byte; a new or rebuilt blob sets it true.
if [ "$(field '.predicate_export // false')" = "true" ]; then
  EXPORTS+=(-Wl,--export=ts_query_predicates_for_pattern)
fi

echo "=== $BLOB ==="
zig cc --target=wasm32-wasi-musl -mexec-model=reactor \
  -I "$WORK/core/lib/include" -I "$WORK/core/lib/src" \
  "$WORK/core/lib/src/lib.c" "$HERE/budget.c" \
  "${GRAMMAR_ARGS[@]}" \
  -o "$WORK/$BLOB.wasm" -Oz -Wl,--no-entry -Wl,--strip-debug \
  "${EXPORTS[@]}"
if command -v wasm-opt >/dev/null 2>&1; then
  wasm-opt -Oz "$WORK/$BLOB.wasm" -o "$WORK/$BLOB.opt.wasm" --enable-bulk-memory
  mv "$WORK/$BLOB.opt.wasm" "$WORK/$BLOB.wasm"
else
  echo "build.sh: warning: wasm-opt missing; the blob is ~25-30% larger than a locked build" >&2
fi
gzip -9 -n -c "$WORK/$BLOB.wasm" > "$OUT/$BLOB.wasm.gz"

echo "=== lock numbers for $BLOB (paste into grammars.lock.json) ==="
echo "\"blob_sha256\": \"$(shasum -a 256 "$WORK/$BLOB.wasm" | cut -c1-64)\","
echo "\"blob_bytes\": $(stat -f%z "$WORK/$BLOB.wasm" 2>/dev/null || stat -c%s "$WORK/$BLOB.wasm"),"
echo "\"gz_bytes\": $(stat -f%z "$OUT/$BLOB.wasm.gz" 2>/dev/null || stat -c%s "$OUT/$BLOB.wasm.gz")"
