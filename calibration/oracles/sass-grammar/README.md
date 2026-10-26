# Sass native grammar parse gate

These scripts reproduce the frozen 219-file gate for `bajrangCoder/tree-sitter-sass` at `fb280c41b070657e4ff4d4e5e6eea6cb19efd9b8`. The subjects are Bulma at `3e00a8e6d0d0e566d507328f0185ef84854effba` (all 91 tracked `.sass` files) and Vuetify at `b1cd40459b4a0801c021c0985ccd46fa65b9f579` (`packages/vuetify/src/`, 128 files). The scripts reject a different checkout or file count. A parse receives 2 seconds. The result is a native grammar refusal test, not a production WASM or language-support claim.

Run from the repository root, with Python and a C compiler available. Use a separate scratch directory for the candidate, subjects, virtual environment, npm packages, and outputs:

```sh
mkdir -p workspace/tmp
export SASS_SCRATCH="$(mktemp -d workspace/tmp/sass-gate.XXXXXX)"
SASS_SCRATCH="$(cd "$SASS_SCRATCH" && pwd)"
git clone https://github.com/bajrangCoder/tree-sitter-sass.git "$SASS_SCRATCH/tree-sitter-sass"
git -C "$SASS_SCRATCH/tree-sitter-sass" checkout fb280c41b070657e4ff4d4e5e6eea6cb19efd9b8
cat "$SASS_SCRATCH/tree-sitter-sass/LICENSE"
git clone https://github.com/jgthms/bulma.git "$SASS_SCRATCH/bulma"
git -C "$SASS_SCRATCH/bulma" checkout 3e00a8e6d0d0e566d507328f0185ef84854effba
git clone https://github.com/vuetifyjs/vuetify.git "$SASS_SCRATCH/vuetify"
git -C "$SASS_SCRATCH/vuetify" checkout b1cd40459b4a0801c021c0985ccd46fa65b9f579
python3 -m venv "$SASS_SCRATCH/venv"
"$SASS_SCRATCH/venv/bin/python" -m pip install 'tree-sitter==0.25.2'
npm install --prefix "$SASS_SCRATCH/npm" --no-save 'sass-parser@0.4.57'
export NODE_PATH="$SASS_SCRATCH/npm/node_modules"
cc -dynamiclib -fPIC -I"$SASS_SCRATCH/tree-sitter-sass/src" \
  "$SASS_SCRATCH/tree-sitter-sass/src/parser.c" \
  "$SASS_SCRATCH/tree-sitter-sass/src/scanner.c" \
  -o "$SASS_SCRATCH/sass-gate.dylib"
"$SASS_SCRATCH/venv/bin/python" calibration/oracles/sass-grammar/gate.py \
  --library "$SASS_SCRATCH/sass-gate.dylib" \
  --subject "jgthms/bulma=$SASS_SCRATCH/bulma" \
  --subject "vuetifyjs/vuetify=$SASS_SCRATCH/vuetify" \
  --out "$SASS_SCRATCH/gate.json"
node calibration/oracles/sass-grammar/reference.cjs "$SASS_SCRATCH/gate.json" "$SASS_SCRATCH/reference.json" \
  "jgthms/bulma=$SASS_SCRATCH/bulma" \
  "vuetifyjs/vuetify=$SASS_SCRATCH/vuetify"
```

On Linux, replace `cc -dynamiclib` with `cc -shared` and name the output `.so`. The `gate.json` records each file's error diagnostics, including a location fallback when `has_error` lacks an `ERROR` or missing descendant. The `reference.json` reports which grammar failures the independent `sass-parser` accepts. The pinned calibration had 123 grammar-error files (53 Bulma, 70 Vuetify), with 51 and 66 respectively on files accepted by `sass-parser`.

Before committing a reading, publish counts and hashes rather than the raw file
lists or compiler messages (X-R3):

```sh
python3 calibration/oracles/sass-grammar/publish.py --self-test
python3 calibration/oracles/sass-grammar/publish.py \
  --gate "$SASS_SCRATCH/gate.json" --reference "$SASS_SCRATCH/reference.json" \
  --out "$SASS_SCRATCH/public-summary.json"
```

Keep raw gate/reference outputs in scratch. `publish.py` can also sanitize the
historical combined receipt (reference_crosscheck embedded) without --reference.
The committed 2026-10-26 receipt was sanitized from that historical combined
receipt, preserving its provenance metadata and counts. No native measurement
was rerun during sanitization. Diagnostic/rejection lists are represented by
counts, kind histograms and SHA-256, never wholesale source file lists or messages.
