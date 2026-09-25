# ADR-0083: Render missing select values as unavailable prompts

- **Status:** Accepted
- **Type:** ux
- **Date:** 2026-09-23

## Context

`Select.Value` can outlive its option set. Previously, a nonempty value matching no
`Option.Value` emitted no selected option, allowing the browser to present a valid
first option that the application never chose. A placeholder plus an empty-valued
option could also emit two selected options. Both shapes obscure the actual state.

## Decision

When a nonempty `Value` matches no option, render one selected, disabled option with
an empty value. Use `Placeholder` as its escaped label when supplied; otherwise use
`Selection unavailable`. Do not reflect the missing raw value or select a valid option.

When `Value` is empty and `Placeholder` is set, the placeholder alone is selected,
including when the app supplies an empty-valued option. A matching nonempty value,
including a disabled match, retains existing output. Empty values without a placeholder
retain native first-option behavior and existing app-owned empty-option semantics.

The unavailable prompt participates only in native select semantics: an optional
missing value is omitted from `FormData`, while `Required` produces native missing-value
validation. The renderer does not add invalid/error state, descriptions, API fields,
classes or scripts; existing ID, name, required, disabled and ARIA attributes remain
unchanged. Apps validate and explain unavailable state and must set an explicit valid
fallback when substitution is intended.

## Consequences

This is a breaking rendered-contract correction for stale nonempty values and the
duplicate-empty selection shape. Rebuild consumers and review snapshots/selectors;
do not rely on accidental first-option substitution. Selected `interaction.forms`
metadata pins change with this correction. Matching disabled choices retain native
semantics: requiredness need not imply a submitted entry. Apps validate submitted values
and permissions; this is not a general disabled-choice policy. The
[assessment](../assessments/2026-09-23-select-state.md) keeps source, controlled-option
browser checks and failed native-key choice separate; no consumer acceptance is inferred.

## Alternatives rejected

- Select the first valid option: silently changes app state and submission meaning.
- Reflect the missing value as an option: exposes stale or untrusted state as a choice.
- Mark the control invalid automatically: validation messages and recovery belong to apps.

## Revisit when

- A named consumer needs a distinct unavailable-value representation.
- Browser form-data or required-validation behavior disagrees with this contract.
- App-owned empty-option semantics cannot coexist with exclusive placeholder selection.
