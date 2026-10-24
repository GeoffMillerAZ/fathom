# jinja2 — provenance and divergences

LB-Q-jinja2, design `docs/design/language-breadth.md` §13. A tags-engine dialect over
`cathaysia/tree-sitter-jinja` v0.5.0 (commit `687b1ff1d6a7eaae26df84560bb1c049f4f1b4c1`,
Apache-2.0, the `tree-sitter-jinja` grammar; blob `jinja2` in `grammars.lock.json`). Built and
measured, **not dispatched**: see the plan row `LB-Q-jinja2-rest` and `refusedDialects`.

## Where each piece comes from

- **Grammar.** The upstream repository ships `queries/highlights.scm` and `injections.scm`, no tags
  query; neither aider nor nvim-treesitter has a Jinja tags or locals query. `dbt-labs/tree-sitter-jinja2`
  v0.2.0 (Apache-2.0) reads a `{% … %}` statement as one opaque regex token, so a macro or block name
  is not a node: not usable.
- **`tags.scm` / `imports.scm`.** Written in this repository from `indexbench lang-probe` output
  over `testdata/`. The grammar is flat (a `block` and its `endblock` are sibling nodes), so a
  definition's range is its opening tag and nothing nests.
- **Preprocess.** `jinja-templates` (`tsextract_jinja.go`): the grammar reads a text line opening with
  `# ` as a line statement and `in` as an operator only in a for header; Jinja has line statements off
  by default and `in` everywhere. Taken only when the file does not parse as written.
- **Imports.** `include-path`: Jinja looks a template name up in the loader's search path; the
  importing file's directory, then each ancestor directory, is the layout of a Flask `templates/`
  tree, a Kolla `docker/` tree and an Ansible role.
- **Fixtures.** Written for this dialect; no upstream file is copied.
- **Oracle.** `calibration/oracles/jinja2-meta` (jinja2 3.1.6, tier A).

## Not built

- `templates/*.html` by content sniff: the pipeline dispatches `.html` by extension before any sniff
  (`p.exts`), and a manifest sniff on `.html` would make every other `.html` file ambiguous and
  excluded unless the html dialect also sniffs. It waits for dispatch (see the plan row's trigger).
- Dispatch itself: throughput .34 MB per CPU-second, under the 1.0 floor, from the grammar's
  one-token-per-character text lexing.
