# ADR-0070: Adopt invocation-origin classification explicitly per CLI

- **Status:** Accepted
- **Type:** dependency
- **Date:** 2026-09-16
- **Supersedes:** ADR-0048 and ADR-0052; their origin and inheritance contracts remain

## Context

The owner authorized the four Bosun/Surveyor unknown-flag/unknown-command failures
to migrate from exit1 to exit2, including reviewed APIs, dependencies and synthetic
verification. The [private adapter](../assessments/2026-09-16-cli-private-adapter.md)
already demonstrates that policy, while retaining the old consumers' compatibility
failures. This fires the authorized-migration triggers of ADR-0039/0048/0052.
Authorization is not evidence that the active migration has passed.

## Decision

Add `command.ExecuteWithClassifier` and `command.InvocationOriginCode`. Existing
`Execute` and `Code` keep their results and Fang option precedence. Success remains0;
an explicit `Exit` wins, including0,1 and ordered joins. Only a non-nil error without
an explicit exit reaches the supplied classifier, once. A nil or declining classifier
uses `Code`. The invocation classifier selects2 only for complete recognized
framework-origin compositions; mixed/unknown errors remain unclassified, ordinary
errors fall back to1, and explicit operational codes remain unchanged.

Use the reviewed Cobra v1.10.2 / pflag v1.0.10 source patches. Origins stay default-off,
with the nearest explicit ancestor setting winning. Consumer entrypoints directly
call `root.SetInvocationErrorOrigins(true)` before the selected execution policy;
a missing patched dependency must fail compilation, not silently retain exit1.
Callback identity, custom converters, diagnostics, hook order, explicit exits and
default-off error identity keep ADR-0048/0052's contracts. Do not classify by text.

Keep app-kit's root dependency graph upstream-compatible. Each selected consumer
owns complete generated `third_party/cli-origin/{cobra,pflag}` snapshots, licenses,
source manifests and explicit local module replacements. Go does not inherit a
dependency's replacements. Cobra's copied module also selects its sibling patched
pflag so independent module tests use the same provenance implementation. That
module-only wiring is recorded separately from the original patch manifest.

The existing hash-verifying [preparer](../assessments/fixtures/cli-consumers/cobra-origin/README.md)
remains the patch source; do not edit generated consumer copies. Delivery must verify
every file, refuse missing/extra/drifted sources, and regenerate identically before
an upgrade is accepted. No published fork, module-cache edit or installed binary is
needed. Local app-kit replacements remain pre-existing development debt, not release
or standalone distribution proof.

## Proof and maintenance cost

Retain the original four runtime failures and the old exit1 consumer tests. Record
the authorized expectation changes explicitly. Prove ordinary/explicit/curated errors
unchanged, patched upstream suites, the invocation-origin oracle, unchanged kit
compatibility tests and the fixed sandboxed help/version/invalid-input matrix.
No operational commands, real data, signing, commits or publishing are admitted.

Accept two maintained patches and duplicated consumer source snapshots. Platform's
existing recursive module gates will discover the added modules; do not hide them
or relax those gates. Test them explicitly in the migration. Compilation alone does
not verify source identity; the manifest check is a separate delivery requirement.

## Alternatives rejected

- Always-on kit classification: changes unselected consumers' compatibility contracts.
- Runtime optional setter fallback: can silently retain the defect with wrong dependencies.
- Replaying parsing or matching messages: cannot preserve caller-owned failure policy.
- A shared external source path: adds a sibling dependency to independently useful kits.

## Revisit when

- Upstream supplies equivalent provenance, or a version invalidates either patch.
- Default-off identity, callback policy, explicit exit or mixed-error canaries fail.
- A selected copy drifts or a build fails without the source checkout: repair delivery
  before claiming adoption; publishing still needs separate authorization.
- A third CLI requests this policy: evaluate its semantics and maintainers' patch cost
  before selecting it; this record does not opt in the fleet.
