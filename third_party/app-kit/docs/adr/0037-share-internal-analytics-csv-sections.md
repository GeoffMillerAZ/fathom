# ADR-0037: Share CSV structure checks, not report semantics

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-14
- **Decision drivers:** two concrete report compositions, stable Gallery bytes, one encoding policy

## Context

Gallery's reliability report combines the existing `analytics.WriteCSV` sections
into a rectangular export. The P11 analytical task now needs receipt/refund/net
evidence in one document. Both need exact shared context/header/row-shape checks;
their domain facts, selections, measures, comparisons and output columns differ.

## Decision

Move only the shared CSV key inventory and exact section-shape check into
`internal/analyticscsv`. Key accessors return copies. Section parsing accepts the
already generated shared-writer format, checks the expected bounded row count,
and refuses context/header/width drift. It is not a general CSV ingestion service.
The shared `analytics.WriteCSV` remains the sole numeric/missingness/formula-safe
cell encoder. Composers preserve those encoded cells, never decode them as raw labels
or apply a second round of spreadsheet escaping.

Keep Gallery's existing private wrapper and output contract. Freeze actual pre-change
overview/detail CSV bytes and prove equal output after extraction. P11 remains an
app-owned synthetic fixture, not a new public metrics API, reusable workflow promotion,
or fourth typed conversion. Its long-form receipt/refund/net export has explicit
measure-specific completeness, source/window context and selected product identity.
No combined-period total or missing-as-zero value follows from joining CSV sections.

## Alternatives considered

- Copy Gallery's parser into P11: repeated schema-drift fixes would be independent.
- Export a new public generic table/CSV framework: no demonstrated external contract
  needs it; report ownership and the existing writer already provide the boundary.
- Reimplement numeric/text escaping in each composer: risks divergence and double escaping.

## Consequences

Two internal callers inherit one format-drift check; public analytics APIs, Gallery
CSV bytes and source semantics remain unchanged. Changing the shared writer's format
must update this checker and each report's explicit schema under migration review.
New external formats are not accepted implicitly. Static CSV proof does not establish
download behavior, chart fidelity, authorization, consumer adoption or target certification.

## Revisit when

- A shared-writer change invalidates a report, or a malformed section passes a canary.
- A named external consumer needs a stable public composition contract.
- Another report needs a fundamentally different encoding or ingestion boundary.
