# CSS native grammar census

`gate.py` compares a native tree-sitter CSS candidate's parse errors against the
already frozen LightningCSS source sidecars. It parses every raw CSS file in
each sidecar, including minified/generated files. It reports a second count
excluding only `*.min.css`, matching the existing production scope. The census
does not read Fathom output, measure definition quality, or assign a grade.

Build each candidate from a separately pinned grammar checkout. Record its
source commit and the resulting library SHA-256 beside the census. The example
uses macOS; replace `-dynamiclib` with the host's shared-library flag elsewhere.

```sh
mkdir -p <scratch>/css-grammar
cc -O2 -fPIC -dynamiclib -I <grammar-checkout>/src \
  <grammar-checkout>/src/parser.c <grammar-checkout>/src/scanner.c \
  -o <scratch>/css-grammar/css-candidate.dylib
shasum -a 256 <scratch>/css-grammar/css-candidate.dylib
```

Use Python with `tree-sitter==0.25.2`. The grammar library exports
`tree_sitter_css`, which `gate.py` wraps as a `tree_sitter.Language` PyCapsule.
The parser must expose `timeout_micros`; every file and both instrument controls
have a two-second parse limit.

```sh
<venv>/bin/python calibration/oracles/css-grammar/gate.py \
  --library <scratch>/css-grammar/css-candidate.dylib \
  --symbol tree_sitter_css \
  --subject normalize=<normalize-clone>=<scratch>/normalize.source.json \
  --subject pico=<pico-clone>=<scratch>/pico.source.json \
  --subject gitea=<gitea-clone>=<scratch>/gitea.source.json \
  --out <scratch>/css-grammar/candidate-census.json
```

The sidecar must come from the committed Chromium CSS `primary.cjs` wrapper.
For each subject, the gate requires a clean Git root at the sidecar's commit,
the complete CSS list in the manifest's scope, and matching raw file hashes
before and after parsing. The output records library hash, runtime version,
grammar ABI, source commit/digest, raw and production-eligible error counts,
and up to three source-relative error locations. A valid style rule and an
unclosed block are controls for the instrument only. Do not use the Tachyons
holdout while selecting a grammar candidate.
