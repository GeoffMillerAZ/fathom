# Jinja optional-else source patch

Source: bennypowers/tree-sitter-jinja-dialects at
4f832fe6feeae8c9e5963d835bec0272a8331b47, MIT.

The patch admits Jinja conditionals without an else clause and preserves the
existing for-loop filter AST by giving for_condition higher precedence.
No template text is blanked or rewritten. The shared P-R1 mechanism, published
as PR #36 (`gpu-sci/grammar-patches-r1`), verifies archive identity and patch
bytes before generation and confines the patch to the verified source tree.

## Committed proof

- `internal/extractor/dialects/jinja2/testdata/syntax-controls.json` contains 22
  authored syntax controls graded independently by Jinja 3.1.6. Run
  `python3 -I calibration/oracles/jinja2-meta/syntax_controls.py -pythonpath workspace/tools/jinja2pkg`
  to check the oracle verdicts (`-write` regenerates them). Run
  `go test -race -count=1 -run TestJinja2SyntaxControls ./internal/extractor`
  to check the shipped WASM with the harness's ERROR-or-MISSING query.
- Legal loop filters and parenthesized conditional iterables cover the
  precedence hunk. One explicitly marked, pre-existing over-acceptance is
  preserved: Jinja rejects an unparenthesized conditional iterable that both
  unpatched and patched grammar accept. The test notices a future correction.
- `conditional_optional_else.txt` contains the three authored upstream corpus
  additions. The prepared upstream PR reports 108 passing cases (105 upstream
  plus these three); that upstream-suite claim is separate from the committed
  WASM controls and full-corpus parse-gate proof.
- `calibration/oracles/jinja2-meta/PATCH-PROOF.md` documents the committed
  driver for rebuilding patched and unpatched blobs with production `build.sh`,
  comparing all pinned corpus files, and recording counts and hashes.

The upstream PR is prepared only under `workspace/upstream/jinja-optional-else/`.
No PR or fork has been published.
