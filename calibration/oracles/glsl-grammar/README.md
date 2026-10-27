# GLSL native grammar regression screen

This screen compares independently built native GLSL tree-sitter libraries over every eligible tracked regular file in one pinned, clean Git checkout. It applies the checked-in GLSL dialect's extensions and `.fs` sniff, records exact full-tree manifests before and after, and fails if the checkout changes. Symlinks are hashed without following them and are never parsed. Gitlinks record their index object ID and are never traversed.

The public `summary.json` contains only repository identity, commit, aggregate counts, byte counts and digests, each grammar binary hash, native `ERROR`/`MISSING` file counts, ordered passing-to-failing deltas, the legacy lexical macro-precision candidate cohort, and precision-statement subtree counts split between literal and macro qualifiers. Path-level manifests, the eligible population, every per-file result, statement byte spans, failure node locations, and untracked paths including ignored paths remain under `private/`. The screen copies no subject content and makes no correctness or throughput claim.

Run from the fathom repository root with the pinned workspace Python and an output directory outside the subject checkout:

```sh
nice -n 10 workspace/tools/python/bin/python -I -B \
  calibration/oracles/glsl-grammar/regression_screen.py \
  --subject workspace/calibration/<subject> \
  --commit <full-40-character-commit> \
  --grammar canonical=workspace/artifacts/glsl-grammar/canonical.dylib \
  --grammar candidate=workspace/artifacts/glsl-grammar/candidate.dylib \
  --dialect internal/extractor/dialects/glsl/dialect.json \
  --out workspace/artifacts/glsl-grammar/<run>
```

The existing candidate cohort and full-file ERROR/MISSING screen are unchanged. Cohort error counts cover whole files, including unrelated errors, and never attribute an error to a precision statement.

The additional statement screen tokenizes the original bytes, skipping line/block comments, strings and preprocessor lines. It recognizes literal and identifier-macro precision qualifiers across comments and multiline whitespace, then records the original `precision`-through-semicolon byte span. A statement passes only when the parse contains exactly one named `declaration` node with that exact span and its own subtree has no `ERROR` or `MISSING` node. No matching declaration is `unrecognized`, hence failing. This catches the withdrawn grammar's split macro parse, where an `ERROR` covers `precision` and the recovered declaration covers only the tail. A valid exact declaration can still pass beneath an unrelated enclosing error; neighboring and enclosing diagnostics are retained separately as overlap evidence and never establish a pass.

These are in-place subtree results from every recognized statement in the pinned holdout. They are separate from full-file results and are not synthetic isolated-snippet passes. Private `<grammar>-precision-statements.jsonl` rows retain path, qualifier class, original byte span, nearest containing node and overlapping diagnostics without copying statement text.

Grammar order is significant. Each grammar after the first is compared with both the first grammar and the immediately preceding grammar. The caller supplies the host load gate and process priority. It requires `tree_sitter==0.25.2`; each dylib is loaded locally through its `tree_sitter_glsl` symbol and wrapped as a `tree_sitter.Language` capsule without changing an installed grammar binding.

Run the synthetic tests without bytecode or external repositories:

```sh
mkdir -p workspace/tmp
nice -n 10 workspace/tools/python/bin/python -I -B \
  calibration/oracles/glsl-grammar/test_regression_screen.py
```
