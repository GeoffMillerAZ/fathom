# ADR-0048: Prove CLI invocation origins before adopting classification

- **Status:** Superseded by ADR-0070
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0039; no dependency adoption or consumer migration

## Context

G5's reporting repair does not classify invocation failures. Cobra v1.10.2 constructs
untyped discovery, argument, required-flag and group errors. pflag v1.0.10 has typed
parse errors, but `InvalidValueError` can wrap arbitrary application callbacks.
An explicit `Exit(1, err)` is indistinguishable from an ordinary error through `Code`
alone. Those facts trigger ADR-0039's bounded typed-origin evaluation, not adoption.

## Decision

Add `command.ExplicitCode(error) (int, bool)` as an additive API exposing the same
first `errors.As` match already used by `Code`. Preserve explicit zero, one, two,
other codes, wrapping and ordered joins. `Code`, `Exit` and `Execute` retain their
existing results and Fang option precedence. No automatic classifier ships here.

Prepare a private, opt-in Cobra/pflag experiment under the existing root module's
assessment fixtures. Verify full pinned source sums before patching in-memory
snapshots, preserve upstream files/licenses, and emit exact candidate manifests into
fresh directories outside the checkout. Use temporary modfiles; never edit caches,
root pins, consumer dependencies or global settings. Repeated preparation must agree.

Tag only Cobra-owned constructors and parsed invocation refusals. Leave public
`ParseFlags` unchanged. A flag-error callback receives its original error exactly
once; tag only when it returns that same error. A replacement, contextual wrapper,
mixed join or nil return remains caller policy. Traverse retains its existing lack
of a flag-error callback. Do not reorder required/group checks ahead of pre-hooks.

The pflag candidate records whether the exact value about to run is one of its
concrete built-in converters. Exclude custom values, Func, BoolFunc, TextVar and Go
flag bridges. Capture before `Set`: public `Flag.Value` can change during or after
failure. Error text, underlying causes and existing accessors remain unchanged.

A test-only classifier gives explicit exits precedence, then assigns 2 only to
complete invocation-only error compositions. Mixed operational joins remain 1.
Never classify by message text, `Value.Type()` strings, callback replay or whether
RunE started. Generated help/completion protocol behavior remains unchanged.

## Consequences and adoption gate

This costs two maintained dependency patches, not a free wrapper around Execute.
Final execution errors may gain a wrapper: direct concrete-type/pointer expectations
can differ even when `errors.Is`/`As`, diagnostic text and callback input survive.
Keep those compatibility differences visible; do not weaken existing tests silently.

Run the same integration oracle on baseline and candidate, both full upstream test
suites, root compatibility checks, and parser/callback/composition canaries. Retain
failures. Synthetic proof is not installed-consumer delivery. Before adoption, review
patch/upstream cost, choose the public opt-in API and migration, and obtain authority
for actual dependency and consumer exit-code changes. Bosun/Surveyor stay unchanged.

## Revisit when

- The complete candidate matrix exposes an untyped origin or misclassified callback.
- A pinned upstream release provides equivalent typed provenance without these patches.
- An authorized consumer migration can exercise usage, operational and explicit exits.
- A source-sum or compatibility canary fails: repair or reject the candidate, never
  promote its current receipt as passing adoption evidence.
