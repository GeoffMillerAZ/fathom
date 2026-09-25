# ADR-0081: Derive one consistent action column for the whole table

- **Status:** Accepted
- **Type:** ux
- **Date:** 2026-09-23
- **Extends:** ADR-0080's managed-link repair; changes its preserved empty-Href layout

## Context

`Row.Href` renders a trailing cell independently of `TableParams.RowHrefs`, which
controls only the header. Forgetting that flag produces an unlabeled action column;
mixed linked/unlinked rows have different cell counts even when it is set. The
previous safety repair deliberately left this layout behavior unchanged.

## Decision

Derive the presence of one action column once per render: a custom `RowAction`, an
explicit `RowHrefs: true`, or any nonempty row Href requires it. Scan rows at most
once using a render-local parameter copy, without mutating caller rows, parameters
or captured component state when one component is rendered concurrently.
The header and every body row agree on that decision. An unlinked row gets an empty
cell, not a disabled control or a fabricated action. A refused nonempty destination
still gets ADR-0080's unavailable label. Custom slots retain precedence.

Keep `RowHrefs` source-compatible as an explicit reservation for empty or currently
unlinked results; it is no longer required to describe actual linked rows. No public
field, JavaScript, route, sorting engine or footer policy is added. Apps still own
footer colspans, trusted slots, selection, activation and route authorization.

## Consequences

Rendered structure changes for previously inconsistent inputs. Rebuild Go/templ,
review snapshots and selectors, and make app-authored footer colspans include the
action column. Existing CSS classes suffice; standard source-scanning requirements
remain. Review selected `data.table` metadata pins; no consumer upgrade is automatic.
This is a breaking rendered-contract correction, not full table accessibility or
starter certification. [Evidence](../assessments/2026-09-23-table-action-column.md)
keeps structural, browser and consumer acceptance distinct.

## Alternatives rejected

- Require callers to duplicate the row scan: leaves a redundant flag and the mixed-
  row defect; useful only if column omission has an independently justified meaning.
- Infer separately in every row: unnecessary quadratic work and divergent decisions.
- Remove `RowHrefs`: loses a useful reservation for empty states and breaks source.

## Revisit when

- A named consumer needs a deliberately different action-column shape.
- Inference changes row values, custom actions or caller-owned state.
- App-authored footer or header associations disagree with the derived shape.
