# ADR-0052: Keep experimental CLI invocation origins default-off

- **Status:** Superseded by ADR-0070
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0048; candidate-only API, no dependency adoption

## Context

ADR-0048's compatibility trigger fired: the initial isolated Cobra/pflag candidate
passes its origin oracle but fails four unchanged kit assertions because returned
errors gain wrappers without command-level consent. Callback inputs, diagnostics
and causes survive, but that does not preserve direct error identity. The
[assessment](../assessments/2026-09-15-cli-invocation-origins.md) retains both results.

## Decision

The private Cobra candidate exposes `(*Command).SetInvocationErrorOrigins(bool)`.
Default is false. The nearest explicit setting on the command or its ancestors wins:
an explicit false overrides an enabled parent, and true overrides a disabled parent.
Resolve against the current tree, not a cached inherited value, so reparenting and
generated commands follow their actual ancestry. Configure before execution, as for
other command settings; this adds no concurrent-mutation guarantee or global toggle.

When disabled, return the original errors with their existing concrete types and
pointer identity. When enabled, wrap only ADR-0048's framework-owned refusals. Group
helpers without a command stay raw; the command's validation boundary applies policy.
Flag-error callbacks still receive the original error once; only an identity-preserving
return may gain a wrapper. Public `ParseFlags`, hook order, callback replacement/nil,
diagnostics, underlying causes and explicit exit policy retain ADR-0048's contracts.
The pflag built-in-origin capture remains separate; it does not enable Cobra wrapping.

Opting in changes returned error types/identity and is an explicit migration boundary.
It does not select exit codes. Root kit dependencies and `command.Execute` do not
enable or expose this experimental API. Automatic classification remains test-only.

## Alternatives rejected

- Always-on wrapping — breaks existing callers merely by replacing a dependency.
- Global switch — couples independent command trees and makes tests order-dependent.
- Copy inherited settings at attachment — stale after reparenting or later configuration.
- Message-based classification — cannot distinguish application and framework failures.

## Evidence and adoption boundary

Prepare two byte-identical candidates from verified pinned snapshots. Run the same
tagged oracle against baseline and candidate, retaining baseline's missing-feature
failures. Add default identity, explicit overrides, actual child execution, reparenting,
direct validation and generated-command tests; never relax the original kit canaries.
Run upstream Cobra/pflag suites and kit compatibility with race detection, passing
the temporary modfile through child builds. Rehash candidate/source manifests afterward.

This resolves a candidate design choice, not consumer readiness. Dependency adoption,
upstream maintenance, actual consumer exit migration and installed-host proof still
need their own review and authority. Historical Bosun/Surveyor usage failures remain.

## Revisit when

- A default-off call changes observable errors, diagnostics, output or exit behavior.
- An opted-in command loses its correct origin or a caller-owned error is misclassified.
- Upstream offers equivalent provenance, or a new pinned version invalidates the patch.
- An authorized consumer migration needs a public reset/inheritance policy or delivery.
