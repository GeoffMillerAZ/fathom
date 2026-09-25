# ADR-0025: Chart composition checks prove scoped static output, not chart correctness

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-13

## Context

G3 requires reusable Resume-as-Code chart checks with adversarial fixtures and a
coverage canary. RAC's `tests/lint/charts_test.go` flags absent bar values, legends
and last-value use, but function-wide identifiers cannot prove those facts reached
the output. An unused legend or discarded last value can satisfy that source lint.
RAC's own source-to-data timeline test is a separate, stronger derivation oracle.

## Decision

Provide an opt-in Go test helper, `ui/vizcheck.CheckHTML`, for an explicitly selected
static composition. Its caller supplies an independent expected output contract:
ordered SVG text, name declarations, scoped annotations/legend when selected,
captioned table cells and source text. Every SVG in the selected root must have one
declared chart owner or an explicitly justified decorative exclusion. Zero charts
is valid for a declared missing/empty/table-only state, not an implicit skip.

Invalid/ambiguous input and resource limits return an error; output mismatches return
findings. Neither is a pass. Fixed IDs, bounded HTML and concrete expectations keep
the helper deterministic, without a selector language, filesystem walker, renderer
registry, test-runner dependency or application runtime.

Direct labels may replace a legend. `aria-hidden` is not a decorative classification.
Do not move editorial policy into low-level `viz` constructors or silently suppress
their output. This implements ADR-0017/0019's composition seam; it supersedes neither.

## Evidence boundaries

There are distinct proofs: declared metric contract; app-owned source/derivation;
static text/table/export agreement; actual browser interaction, visibility and geometry.
This helper contributes only to the third and checks narrow name declarations.
It cannot establish CSS visibility, accessible-name computation, chart geometry,
contrast, keyboard use, dynamic DOM behavior, source relevance or formula correctness.

Maintain positive and planted-negative fixtures, including wrong-scope text, extra
or dropped SVG coverage and mismatched chart/table values. Consumer expectations
must not be extracted from the renderer output they claim to check. Review oracle
changes independently of renderer repairs; a copied output is not a truth fixture.

## Alternatives rejected

- Copy RAC's AST linter as a conformance gate: lexical presence can be unrelated to
  chart output. It can still be useful advisory detection in its owning app.
- Infer every numeric-looking visual's obligations: diagrams, ordinal assessments
  and direct labels have different legitimate contracts; the app declares meaning.
- Add a browser engine to this helper: conflates deterministic local checks with
  target-specific proof. Browser checks remain necessary and separate.

## Revisit when

- A planted missing/wrong-output case passes, or a new chart escapes declared coverage.
- Two consumers need the same unsupported table/annotation shape; add only the
  narrow extension demonstrated by their fixtures.
- A consumer presents static findings as runtime accessibility, fidelity or adoption
  proof; correct that integration and its evidence labels before promotion.
