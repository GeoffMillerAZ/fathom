#!/usr/bin/env bash
# build.sh — reproduce the vendored *.wasm grammar blobs in this directory.
#
# Each blob is tree-sitter's pure-C core runtime (lib/src/lib.c, the official
# amalgamation) statically linked with ONE grammar's parser.c (+ scanner.c, if
# the grammar has an external scanner) into a single, self-contained
# wasm32-wasi "reactor" module — no dynamic linking, no Emscripten JS glue, no
# CGO. This is deliberately NOT the web-tree-sitter/tree-sitter-wasms shape
# (a "side module" needing Emscripten's dylink relocation + a JS-runtime
# callback shim to run) — that path was evaluated and found impractical to
# reimplement correctly in pure Go (see PROVENANCE.md's "path not taken").
# Building core+grammar together as one plain WASI module sidesteps dynamic
# linking entirely: wazero's stock `wasi_snapshot_preview1` package is the
# only host shim required.
#
# Requires (build-time only — NOT a runtime or `go build` dependency):
#   - zig (>= 0.16; ships a wasm32-wasi-musl libc + clang + wasm-ld)
#   - wasm-opt (binaryen; optional but shrinks the blobs ~25-30%)
#   - curl
#
# The COMMITTED blobs were produced with zig 0.16.0 (Homebrew, arm64 macOS)
# and wasm-opt version 129 (binaryen). Byte-identical reproduction across
# toolchain versions is NOT achievable — different zig/binaryen releases emit
# different (functionally equivalent) code. Reproduction therefore verifies
# the SOURCE pins (the tags/SHAs in PROVENANCE.md), not the output digests;
# the sha256s in PROVENANCE.md bind the committed bytes as-committed.
#
# Usage: ./build.sh [outdir]   (defaults to this directory)
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
OUT="${1:-.}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Pinned versions — keep these in lockstep with the CGO grammar versions in
# go.mod (github.com/tree-sitter/tree-sitter-<lang>) so the calibration
# comparison between the two engines is apples-to-apples. See PROVENANCE.md
# for the resolved commit SHAs and blob checksums this produced.
CORE_TAG="v0.26.11"
JS_TAG="v0.25.0"
TS_TAG="v0.23.2"
PY_TAG="v0.25.0"
RB_TAG="v0.23.1"

fetch() { # fetch <repo> <tag> <destdir>
  curl -sL --fail "https://github.com/$1/archive/refs/tags/$2.tar.gz" -o "$WORK/src.tar.gz"
  mkdir -p "$3"
  tar xzf "$WORK/src.tar.gz" -C "$3" --strip-components=1
}

echo "fetching tree-sitter core $CORE_TAG..."
fetch tree-sitter/tree-sitter "$CORE_TAG" "$WORK/core"
echo "fetching tree-sitter-javascript $JS_TAG..."
fetch tree-sitter/tree-sitter-javascript "$JS_TAG" "$WORK/javascript"
echo "fetching tree-sitter-typescript $TS_TAG..."
fetch tree-sitter/tree-sitter-typescript "$TS_TAG" "$WORK/typescript"
echo "fetching tree-sitter-python $PY_TAG..."
fetch tree-sitter/tree-sitter-python "$PY_TAG" "$WORK/python"
echo "fetching tree-sitter-ruby $RB_TAG..."
fetch tree-sitter/tree-sitter-ruby "$RB_TAG" "$WORK/ruby"

CORE_INC=(-I "$WORK/core/lib/include" -I "$WORK/core/lib/src")
# budget.c (this directory, checked in) adds fathom_parse_bounded — the
# deadline-bounded parse entry the Go engine calls (critic B1; see the
# shim's header comment for why guest-side cancellation beats wazero's
# WithCloseOnContextDone by ~3x corpus-wide).
CORE_SRC=("$WORK/core/lib/src/lib.c" "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/budget.c")

# The exported C API surface every extractor's marshaling layer calls
# (internal/extractor/tsengine). Adding a capability = add its export here.
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

build() { # build <name> <srcdir...> -- <extra include dirs> -- <extra export flags>
  local name="$1"; shift
  echo "=== $name ==="
  zig cc --target=wasm32-wasi-musl -mexec-model=reactor \
    "${CORE_INC[@]}" "${CORE_SRC[@]}" "$@" \
    -o "$WORK/$name.wasm" -Oz -Wl,--no-entry -Wl,--strip-debug \
    "${EXPORTS[@]}"
  if command -v wasm-opt >/dev/null 2>&1; then
    wasm-opt -Oz "$WORK/$name.wasm" -o "$WORK/$name.opt.wasm" --enable-bulk-memory
    mv "$WORK/$name.opt.wasm" "$WORK/$name.wasm"
  fi
  cp "$WORK/$name.wasm" "$OUT/$name.wasm"
  ls -la "$OUT/$name.wasm"
}

build javascript \
  -I "$WORK/javascript/src" "$WORK/javascript/src/parser.c" "$WORK/javascript/src/scanner.c" \
  -Wl,--export=tree_sitter_javascript

build typescript \
  -I "$WORK/typescript/typescript/src" -I "$WORK/typescript/tsx/src" \
  "$WORK/typescript/typescript/src/parser.c" "$WORK/typescript/typescript/src/scanner.c" \
  "$WORK/typescript/tsx/src/parser.c" "$WORK/typescript/tsx/src/scanner.c" \
  -Wl,--export=tree_sitter_typescript -Wl,--export=tree_sitter_tsx

build python \
  -I "$WORK/python/src" "$WORK/python/src/parser.c" "$WORK/python/src/scanner.c" \
  -Wl,--export=tree_sitter_python

build ruby \
  -I "$WORK/ruby/src" "$WORK/ruby/src/parser.c" "$WORK/ruby/src/scanner.c" \
  -Wl,--export=tree_sitter_ruby

echo "=== sha256 ==="
shasum -a 256 "$OUT"/*.wasm
