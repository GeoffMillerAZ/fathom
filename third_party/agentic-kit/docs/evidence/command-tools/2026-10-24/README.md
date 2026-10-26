# Gitleaks stdin profile qualification

Scope: GT5A under [0039](../../../intent/decisions/0039-governed-command-tools-preserve-evidence-and-authority.md)
and [GT-C3a](../../../designs/governed-command-tools.md#gt-c3a-gitleaks-stdin-qualification-profile).
This qualifies one pinned raw-text/default-rule profile on macOS arm64. Broader GT5
scanner/PII/format coverage, GT4 native platforms and the complete command/hook goal
remain open. No scanner, hook or machine-wide configuration was installed or changed.

## Native evidence

[Native trial](native-trial.json) binds the final guard binary and pinned Gitleaks
executable to 14 synthetic cases. [Source pins](source-pins.json) bind the command,
schema, tests and fixtures. [Scanner provenance](scanner-source.json) records the
official asset hash and HTTPS identity limit, not independent signature attestation.

| Cases | Observed result |
|---|---|
| Clean, empty, 4608-byte text and 1 MiB multifragment text | Exit 0, exact selected byte accounting, complete required profile witness |
| Independent fictitious credential, later-file finding and bundled canary | Exit 1, scanner finding, every content row withheld |
| Inline `gitleaks:allow` comment | Still detected and withheld on the controlled recipe |
| Hostile ambient config/environment and ignore file | Ignored by the private working directory/environment; finding still blocks |
| Text-only PDF with upstream exit 0 and empty report | Adapter exit 2 with `coverage_mismatch`; upstream observed zero of 40 supplied bytes |
| Wrong executable hash, report overflow, deadline and omitted scanner scope | Exit 2, no content rows; appropriate fixed diagnostic categories |

The profile checks process completion, whole scanner budget, strict version/log/JSON/
exit agreement and exact observed byte count together. A byte count alone does not
attest semantic rule evaluation, sensitivity or absence of secrets. Native allowlists
remain part of the embedded defaults; decoding, archive and arbitrary config profiles
are not qualified here. Raw reports/errors never enter the public receipts.

## Other instruments and review

- Full `task validate` passed. Focused fresh race run passed with 81.1% statement
  coverage. Tests use injected runners and the Go test executable, not Gitleaks,
  network, keys or downloaded dependencies.
- Windows amd64 and Linux amd64 cross-builds passed. Those prove compilation only;
  no native subprocess, path, hook or isolation claim follows.
- Independent security differential review found and resolved two low-severity issues:
  attempted calls were conflated with started calls, and cleanup wording was absolute.
  [Retained review](security-review.md) names its source and static-review limits.
- Independent [guide/skill trial](guide-trial.md) proved public/finding/hash-failure
  paths. Its plausible password sample missed default rules. The guide now supplies
  a deterministic fictitious canary and the native trial verifies that case. A passing
  canary proves one known detection, not broad accuracy or production suitability.
- First strict plan candidate correctly failed an unused private wrapper anchor.
  The wrapper was removed and the contract now names `scanSelectedWithRunner`, the
  actual product/test path. No verifier waiver or artificial test was added.
- Fresh isolated-store strict verification passed all four claimed GT rows, including
  GT5A, before that row was checked. GT5 remains in progress, not silently narrowed to
  this profile. Runtime passing evidence remains the separate tests/native trial.

The shared Fathom daemon remained stale/unavailable to bounded review calls. A separate
fresh scratch store supplies plan proof; that does not repair or qualify the shared
daemon. Gate/plan/native/review logs are retained privately under
`/Volumes/m2ext/backups/agentic-kit/design-reviews/2026-10-24-command-scanner/`.
The normal push receipt is retained after delivery and must be inspected separately.

## Limits and adoption

Use the [command guide](../../../../cmd/command-guard/README.md) and existing audit
skill. Independently require the scanner in protected scope/policy. Hash/version and
executable snapshots bind trusted bytes, not organizational authorization. The child
retains host filesystem/network authority; no hard memory ceiling or hostile-filesystem
wall-time guarantee is claimed. Cleanup failure holds and can leave private artifacts.

No five-root library or exported API changed; downstream consumer checks were not
required for this command-local extension. app-kit's existing stdlib CLI divergence
remains documented. Installation, authenticated operator policy, global hook control,
local/advanced classifiers, optional Fathom and measured rollout remain separate work.
