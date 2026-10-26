# jinja2 — provenance and divergences

LB-Q-jinja2, design `docs/design/language-breadth.md` §13. A tags-engine dialect for
`.j2`, `.jinja`, and `.jinja2`, using
[`bennypowers/tree-sitter-jinja-dialects`](https://github.com/bennypowers/tree-sitter-jinja-dialects/tree/4f832fe6feeae8c9e5963d835bec0272a8331b47)
at commit `4f832fe6feeae8c9e5963d835bec0272a8331b47` (MIT).

## Where each piece comes from

- **Grammar.** Built from that upstream commit with the in-tree
  `internal/extractor/tsengine/grammars/patches/jinja2/optional-else.patch`
  (SHA-256 `ac3e0825cd149da56ed02cac14ea2e7f96b92d8b8f10c1c0a3cc9e9f4410be86`).
  The patch allows a conditional without an else and preserves loop-filter precedence.
  The shared patch mechanism is P-R1 (`50675f456`). The unmodified production
  `build.sh` repeatedly built the same patched source with matching raw WASM SHA-256
  `71680f5fead32c2881a5e486e8dc79a71656e3be60fff7b714667673b5f30e25`
  (212061 bytes raw, 71302 bytes compressed).
- **Queries.** `tags.scm` and `imports.scm` were written here from owned fixture
  probes. They use keyword positions for statement names, the header's `@end`
  for definition ranges, and the macro signature lead. No upstream query is copied.
- **Imports.** `include-path` resolves a template name through the importing
  file's directory and ancestor directories, matching the measured template trees.
- **Fixtures.** Written for this dialect; no upstream file is copied.
- **Oracle.** `calibration/oracles/jinja2-meta` uses Jinja 3.1.6's parser, AST,
  and `meta.find_referenced_templates` independently (tier A).

The old `cathaysia/tree-sitter-jinja` v0.5.0 (Apache-2.0) needed a pre-parse
repair and left 21 of 520 files with errors. It measured 0.93 MB/CPU-s in the
2026-10-25 timing retake (historically 0.34). The new grammar parses files as
written; no `jinja-templates` preprocessing is used. Native screening moved
from 27 errors unpatched to 3 patched, repairing 24 files with no new failures.
All 108 upstream corpus cases (105 existing, 3 added) and eight independent
Jinja 3.1.6 legal/invalid controls pass.

Three full-oracle receipts are in `workspace/artifacts/jinja-final-run{1,2,3}.json`:
520 files, 734576 bytes, 1124 definitions, 687 qualified calls, 189 resolved imports/file
edges, L1–L3 excellent, 3 files with ERROR or MISSING (0.58%), zero faults, and
4.77/4.93/5.08 MB/CPU-s. The median is 4.93. These receipts measure full
extraction accuracy and throughput; the native error count alone measures only
parse coverage. Three remaining parse errors involve splatted arguments, extra
filter arguments, and a numeric dotted attribute.

A separate bounded stress probe (`workspace/artifacts/jinja-stress.json`) aborted
one 57344-byte malformed repeated `{% if (` input at the 10-second budget; that
input counted one fault. The next 55-byte macro with optional else recovered in
6.383 ms with two symbols and zero faults. Thus the zero-fault claim applies to
the 520-file calibration, not to arbitrary malformed input.

## Coverage ceiling

`templates/*.html` is outside this dialect's dispatch. Extension dispatch and
the file resolver select HTML before Jinja sniffing. Adding that coverage needs
an engine and HTML regrade; it is not implemented here.

An upstream PR is prepared only under `workspace/upstream/jinja-optional-else/`.
It has not been opened.

Namespace imports opt into `imports.whole_bindings: "namespace"`; qualified
call captures preserve the receiver. Includes and extends emit side-effect-only
import specs. Two imported templates defining the same macro do not collide or
leak that macro into unqualified scope (ingest collision regression). Other
dialects keep the existing whole-import binding defaults.
