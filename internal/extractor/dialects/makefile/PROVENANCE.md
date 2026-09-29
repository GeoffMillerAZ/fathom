# makefile — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-make` at commit `70613f3d812cbabbd7f38d104d60a409c4008b43`
  (`main`, 2026-02-26; MIT), built into `internal/extractor/tsengine/grammars/makefile.wasm.gz`
  (86,559 bytes gzip, 265,371 raw, ABI 15, no scanner); pins and digests in
  `internal/extractor/tsengine/grammars/PROVENANCE.md`/`grammars.lock.json`. The lock's `tag` field
  carries the commit id, the mechanism `gotemplate` already uses for a repository with no pinnable
  release. The latest release tag, `v1.1.1` (commit `5e9e8f8ff3387b0edcaa90f46ddf3629f4cfeb1d`, 2024-12-21),
  was built and measured first and is not the pin: nine commits later the grammar parses unescaped
  parentheses in text and `$(shell ...)`, trailing comments in assignments and continued recipes
  (`2ddf542`, `dbb2b04`, `0ee5851`), and on the same subjects, queries and oracle the release reads
  valkey's Makefiles at 467 definitions where the pinned commit reads 588 (sym_recall .956 against
  .981; both are L2 excellent on references). `main`'s HEAD ships its generated `src/parser.c`, so
  `"generate": false` holds. Move to the next release tag when one ships and re-measure.
- `tags.scm`: written from scratch in this repository (design §13 step 11), never generated or
  copied. The grammar's `queries/` holds `highlights.scm`, `folds.scm` and `injections.scm` (no
  `tags.scm`, read for node vocabulary only); aider and nvim-treesitter carry no make tags or
  locals query. Node and field shapes came from `indexbench lang-probe` over `testdata/*.mk`,
  never from `grammar.js`/`node-types.json`. The file's header states what is and is not a
  definition or a reference, and the grammar and engine limits behind the output.
- `testdata/`: fixtures written in this repository (none copied from a subject); goldens written
  by hand from reading each fixture before any query existed (red first), then checked against
  `indexbench lang-probe`. Two classes of value differed from the by-reading expectation and were
  corrected, each verified against the fixture rather than accepted from the engine: an end line
  runs past the construct's last line over the blank lines tree-sitter-make's node absorbs
  (`all: prog docs` on line 15 ends at 16), and an inline recipe (`inline: dep ; echo inline`)
  starts at the `;`, so the signature is `inline: dep`.
- Kinds: `MakeTarget` (a rule's target word, or a target a target-specific assignment names),
  `MakePatternRule` (a target containing `%`), `MakeVariable` (`=`, `:=`, `::=`, `?=`, `!=`; not
  `+=`), `MakeDefine` (`define ... endef`). Targets and pattern rules are callable: their
  literal prerequisites are their `Calls`. No incumbent exists (a new language), so no
  `ExtractorEpoch` bump and no parity run.
- Measurement (`docs/bench/readings/lang/makefile.json`, 2026-09-29): `jart/cosmopolitan` 4.0.2,
  `prometheus/prometheus` v3.9.1 and `valkey-io/valkey` 9.1.2 (`calibration/corpus.d/makefile.yaml`;
  177 files read, zero faults). L1 against universal-ctags 6.2.1 (tier B) with two disclosed
  corrections, L2 against GNU make 4.4.1's own database (tier A); both in
  `calibration/oracles/make/`, whose README is the record of what each oracle does and does not
  claim. `calibration/oracles/make/` is bench-only: never a product dependency.
