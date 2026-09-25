# ADR-0045: Prepare private content-identified candidates; upgrade by reviewed exact pins

- **Status:** Accepted
- **Type:** process
- **Date:** 2026-09-15
- **Supersedes:** ADR-0013; preserves ADR-0026's unpublished native-module exception

## Context

G4 requires a prepared candidate and independent consumer build, not merely a local
replace or passing root tests. The [first rehearsal](../assessments/2026-09-13-clean-consumer-build.md)
used a temporary preparer and an older working-tree snapshot. It proved a scoped
build, not a reusable preparation path or released adoption. ADR-0013's unbounded
`go get -u` guidance conflicts with current exact-pin and selected-artifact checks.

## Decision

Keep one root Go module (`github.com/geoffmilleraz/app-kit`) and one semver release
stream. Preserve layered adoption: import Go packages, copy selected portable assets
through the existing vendor contract, and keep documentation available with the kit.
ADR-0026's unpublished Wails experiment stays outside this module's dependency graph.

Upgrades name an exact reviewed version (`go get github.com/geoffmilleraz/app-kit@VERSION`)
and review the resulting module graph, migration notes and selected artifact changes.
Repeat the consumer's build/CSS/runtime checks; reconcile its explicit adoption record.
Local replaces remain development conveniences. Neither `go get -u`, a file declaration,
nor successful dependency resolution proves an upgrade or propagation outcome.
Keep pre-1.0 incompatible changes explicit, with affected consumers and migration steps;
release versioning and authorization remain governed by scope/autonomy and the changelog.

Provide `go run ./tools/releasecandidate` as a maintainer-only private preparer.
It selects bounded Git-visible regular files under a fixed source allowlist, refuses
unsafe paths and local module replacements, and compares two path/byte snapshots plus
HEAD before writing. Drift fails; this is not an atomic capture of adversarial writers.
Keep paid `data/`, native spikes and global configuration outside selection. The
allowlist is not a secret scanner or a license audit; outputs remain private.

Derive an unpublished prerelease version from the full selected-tree SHA-256. Build
the module ZIP twice from the same retained bytes, compare bytes, and record file
hashes, ZIP SHA-256, Go module sums, HEAD, exclusions and limitations. Emit an ordinary
file GOPROXY plus receipt into a fresh explicit private directory outside the source.
An existing output is an error; failed writes retain incomplete output for inspection.
Proxy metadata uses a fixed Unix epoch, not an invented source or release timestamp.

Candidate identity is selected working-tree content, not a clean commit, tag or
certificate. Review the inventory before reuse. Rehearsal separately proves a pinned
consumer with empty caches, no sibling source, no network fallback, required CSS
selectors and meaningful handler assertions. It does not prove browser UX or adoption.
Do not change consumer pins, install tools, publish or tag as a preparation side effect.

## Alternatives rejected

- Reuse the temporary preparer indefinitely: no maintained refusal/replay contract.
- Require a root commit before any rehearsal: conflates a private reviewed snapshot
  with a release and pressures unrelated dirty work into commits.
- Create a release service or new public CLI family: no present consumer requirement.
- Claim source selection establishes redistribution rights: paid-source rights require
  their own review even when files are not in the original archive directory.

## Revisit when

- A selected package cannot build from the prepared ZIP or a required document is omitted.
- A forbidden/private path enters an artifact, or concurrent drift escapes the bounded check.
- A native adapter earns promotion or two consumers require independent release cadence.
- The owner authorizes a release: add clean-commit provenance and applicable delivery gates
  before tagging; do not relabel a rehearsal receipt as release evidence.
