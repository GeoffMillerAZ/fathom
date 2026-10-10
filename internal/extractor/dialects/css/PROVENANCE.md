# css — provenance

- Grammar: `tree-sitter/tree-sitter-css v0.25.0` (MIT), built into
  `internal/extractor/tsengine/grammars/css.wasm.gz`; pin and digests in
  `internal/extractor/tsengine/grammars/PROVENANCE.md`.
- `tags.scm`: written in this repository (LB-Q-css). `tree-sitter/tree-sitter-css` ships no
  `queries/tags.scm` of its own (only `highlights.scm`), and neither aider
  (`aider/queries/**/*-tags.scm`) nor nvim-treesitter carries a css-tags.scm/locals.scm to
  start from (checked 2026-09-27 and again 2026-09-28) — never GitNexus. Every capture shape
  comes from `indexbench lang-probe` against real source (this file, testdata, and the
  calibration subjects), never from `grammar.js`/`node-types.json` (design §13.2).
  - The selector pattern matches an explicit alternation of the thirteen selector-node types
    observed via `lang-probe` (`tag_name`, `universal_selector`, `class_selector`,
    `id_selector`, `attribute_selector`, `pseudo_class_selector`, `pseudo_element_selector`,
    `namespace_selector`, `nesting_selector`, and the four combinators —
    `descendant_selector`, `child_selector`, `sibling_selector`, `adjacent_sibling_selector`),
    not a bare `(_)` wildcard: `comment` is also a direct child of `selectors` (CSS allows a
    trailing `/* note */` after a comma inside a multi-line selector group — a real shape,
    e.g. `necolas/normalize.css` v2.1.3 line 324, `html input[type="button"], /* 1 */`), and
    `(_)` matched it as a spurious "/* 1 */" definition. Found by running the harness against
    normalize.css (§13 step 21); fixed by the alternation and locked in by
    `testdata/comments.css`.
  - Custom properties (`--name: value;`) are syntactically an ordinary `declaration`;
    `#match?` (LB-F2) singles them out by name. universal-ctags 6.2.1 has no CSS kind for
    these at all (checked directly: `ctags --list-kinds-full=CSS` lists only `class`, `id`,
    `selector`) — deliberately absent from `oracle_kinds`, the same convention as
    `PythonTypeAlias` (see `python/PROVENANCE.md`).
  - `@keyframes` blocks are one definition per animation; the individual `from`/`to`/`N%`
    steps inside are not captured — ctags has no equivalent and the vision's fleet-gap note
    names "keyframes" as a whole, not its steps.
- `imports.scm`: written in this repository. `@import "file.css";` and `@import url("file.css")
  screen;` both resolve via the generic `@import.stmt`/`@import.module` captures (LB-F7) and
  the `relative-path` strategy (design §6 table: "CSS/SCSS/LESS `@import`").
- `testdata/`: fixtures written in this repository; goldens generated with `-update` and
  reviewed by hand against each fixture's source and against `indexbench lang-probe`'s tree.

## Engine changes this dialect required (LB-Q-css, 2026-09-28)

CSS is the first tags-engine dialect added after the five LB-F1 migrated languages, and it
exposed three gaps nothing before it had hit — none are new strategies or predicates (design
§13.2's "do not extend the engine" is about those); each is recorded here plus in its own
file's log/commit for LB-F1's broader audience:

- `internal/fusion.New` wired only the five migrated languages' extensions into `p.exts` by
  hand; a dialect added after them (this one) had no path into the real pipeline at all —
  `no-extractor` for every `.css` file, both at real ingest and in `indexbench lang-oracle`.
  Fixed with a generic loop over `extractor.LoadDialects()` for every tags-engine dialect not
  already wired by hand (`internal/fusion/pipeline.go`).
- `internal/connector.extClassMap`, `unsupported_language_gen.go` (regenerated via
  `indexbench lang-disclosure`), and `internal/connector.typedEdgeProducers` (`"css": "file"` —
  LB-F11's generic `FileResolveLangs()` already covers CSS's `relative-path` `@import`
  strategy, no maintained SCIP producer exists for CSS) all needed a CSS entry alongside the
  registry fix — the bijection `TestLanguageRegistryParity` holds.
- `tsextract.go`'s `Line`/`EndLine` computation assumed one `@name` per `@definition.<kind>`
  (true for every migrated language). CSS's grouped selector list is N `@name`s sharing one
  `@definition.selector` (the `rule_set`, needed whole for a correct `EndLine` — the rule's
  closing brace). `Line` used to take the shared definition node's own row for every match,
  so every selector past the first in a multi-line group got the WRONG line — both in the
  store (a wrong `find_symbol`/`references` location for that symbol, not just a benchmark
  artifact) and against ctags, which reports each selector's real line (§1.4's matching
  tolerance is only `|Δline| ≤ 2`, exceeded by up to ~20 lines on `necolas/normalize.css`).
  Fixed by taking each match's own `@name` row for `Line` (a new `tsDef.defLine` field keeps
  the shared definition's own row for `EndLine`'s within-range newline count, unaffected).
  `TestTagsDialectParity`'s five migrated-language goldens are byte-identical before and
  after (name and definition rows coincide for every one of their fixtures).
- `internal/extractor.Dialect` gained an optional `ctags` field
  (`extractor.DialectCtags`/`langbench.CtagsSpec`, mirrored in `schema/dialect/dialect.cue`):
  the langbench harness's tier-B invocation for the five migrated languages lives in the
  embedded `internal/langbench/incumbents.json` only; a dialect added after them had no way
  to say `ctags --languages=CSS` at all (`indexbench lang-oracle` reported ctags
  "unavailable: no parser for this dialect" even though ctags DOES know CSS). `css/dialect.json`
  sets `"ctags": {"languages": ["CSS"], "args": ["--sort=no"]}` (the args since K19, below).

## Oracle finding: universal-ctags 6.2.1 mistokenizes modern selector lists (LB-Q-css)

`docs/bench/readings/lang/css.json`'s low combined `sym_recall`/`sym_precision` (0.05/0.13) is
an ORACLE limitation, not an extraction defect — verified, not assumed:

- `necolas/normalize.css` v2.1.3 alone (no `:has()`/`:is()`/`:not()`): fathom and ctags both
  find 70 selector-kind definitions; 58 match (`sym_recall`/`sym_precision` 0.83/0.83 — short
  of L1 `good` .85/.90, the rest is ctags' own text-normalization quirks: it inserts a space
  before `:not(...)`'s closing paren, strips the brackets from a bare `[hidden]` attribute
  selector to `hidden`, and inserts a space before a chained pseudo-element
  `[type="search"]::-webkit-search-cancel-button`).
- `picocss/pico` v2.1.1 (`css/`, 119 non-minified files, heavy `:has()`/`:is()`/`:not()` with
  multi-line, comma-separated, paren-nested arguments — e.g. `pico.css:190-207`'s
  `[role=search]:has(button.contrast:focus,\n[type=submit].contrast:focus, ...)`): ctags
  emits 194,677 selector-kind tags against fathom's 78,191 (tree-sitter-parsed, verified
  correct against the source). Inspecting ctags' raw JSON output directly shows the cause: a
  stray split at an inner `)` produces malformed tags like `"').contrast'"` — ctags' CSS
  parser does not track paren nesting across a multi-line functional-pseudo-class argument
  list and treats each inner comma as a new top-level selector.
- Not query-fixable (§13.4 step 21's "queries can fix" bar): the mismatch is entirely in what
  the ORACLE reports, not in what fathom extracts (fathom's own 78,191 tags were spot-checked
  against `pico.css`'s source and against `indexbench lang-probe`'s tree — correct).
- Recorded in the reading's `disposition` (LB-Q-css stays `[ ]`; a sibling `LB-Q-css-rest`
  row carries the disposition per design §14.2). Trigger: a ctags release that tracks paren
  nesting across a multi-line selector argument list, or an Opus-approved substitution for
  the queue entry's second `calibration_subjects` slot.

## K19: ctags compared in its own rendition (wave lf4, LB-F423, O798)

The finding above was right about the oracle; the harness now scores against it honestly
instead of the plan waiting for a ctags release. At fb7d9f3f every ctags-only tag on
normalize.css and pico is explained by replaying `parsers/css.c` (v6.2.1, 9463f1c6) over the
dialect's selector text, and none of the causes the plan had named (BareName's separators,
per-selector lines, a `:not(A, B)` misparse) costs a match (O798's table). The rule, scoped to
the `css` lang tag and the ctags oracle (`langbench.cssCtagsRendition`, `ScoreL1`): a selector
is compared in the tags ctags writes for it, the first being the match key at its own line and
the rest the same definition, consumed and tallied `ctags:selector-fragment`; `dialect.json`
runs ctags with `--sort=no`, since its sort drops a tag two selectors share on one line. The
dialect itself is unchanged. Readings: `css.json` L1 good (sym_recall 1, sym_precision .9925 on
normalize.css and pico; the 590 dialect-only selectors are inside `@-moz-document`, a block
ctags skips), and the holdout read once, `holdout/css.json`, L1 good at .999/.999 on
github-markdown-css v5.9.0 and pure v3.0.0, pinned before the rule ran on them.
