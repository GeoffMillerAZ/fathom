# ADR-0068: Prepare private unsigned macOS delivery first

- **Status:** Accepted
- **Type:** platform
- **Date:** 2026-09-16
- **Extends:** ADR-0021, ADR-0027, ADR-0033 and ADR-0047

## Context

The owner selected a private unsigned app bundle as the first macOS delivery path.
This settles the design direction, not installation or use of signing credentials.
Existing cold server/native builds and synthetic recovery evidence do not establish
bundle launch, packaging/update recovery, Gatekeeper acceptance or a released product.

## Decision

Prepare one private macOS bundle for the existing Gallery workflow, using the pinned
HTTP-owning process and native companion topology. Preserve the independently usable
browser entrypoint, current native security profile, app-owned state and exact build
provenance. Keep Wails isolated; no consumer migration or new shell/runtime is implied.

Unsigned means no Developer ID/notarization distribution workflow. Record any existing
linker/ad-hoc signature truthfully; do not call it trusted or signed release delivery.
Checksums detect artifact disagreement, not publisher identity. Keep the bundle and
evidence private, with no purchased corpus, personal data or source-checkout dependency
required for its representative task. Do not silently strip quarantine or relax OS policy.

The acceptance contract is [private unsigned delivery](../design/web-desktop-parity.md#private-unsigned-macos-delivery).
Use fresh private staging, explicit artifact inventory and retained prior/candidate
revisions. Prove missing/changed executable refusal, relocated launch, shared workflow,
shutdown, failed-start recovery and manual replacement/rollback. Preserve a known-good
bundle until the candidate passes. Do not add a background updater, installer service,
signing abstraction or automatic migration merely to rehearse private delivery.

This record specifies work; it does not claim bundle implementation or acceptance.
Installation, signing credentials, publication and consumer deployment need explicit
authority. A packaging test must name its authorized execution scope and actual host;
successful source builds or existing native probes do not grant inherited passes.

## Alternatives and deferred work

- Signed/notarized delivery first: unnecessary for the selected private rehearsal.
  Wins when the owner selects distribution requiring a trust/signing workflow.
- Browser-only delivery: remains independently useful, but does not satisfy the
  selected native packaging and recovery outcome.
- Automatic updates now: no demonstrated private-use requirement. Manual exact-version
  replacement and rollback are the present acceptance path.

Windows remains owner-deferred; macOS success does not establish Windows or mobile
support. Native input/media failures remain open independently of packaging progress.

## Revisit when

- The owner selects distribution to another user/device or signed/notarized delivery.
- An actual supported host refuses the unsigned bundle under normal OS policy.
- Manual replacement cannot preserve the selected app's state or recover a failed start.
- A second app demonstrates a common packaging/update seam worth promoting into the kit.
