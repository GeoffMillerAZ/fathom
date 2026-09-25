# Gallery-only Datastar pin

Unmodified `bundles/datastar.js` and `LICENSE.md` from Star Federation Datastar
v1.0.3, commit `73ab00e7c06d8c2bad030fdddafba800fcccbde2` (2026-08-27).

- Source: <https://github.com/starfederation/datastar/tree/73ab00e7c06d8c2bad030fdddafba800fcccbde2>
- Bundle: 33,538 bytes; SHA-256 `5d6b7794a50a83d82da962aec5e382f5ae83ac7afbc751f903f7a9c6bd433c65`.
- License: retain the adjacent upstream notice when copying the bundle.
- Scope: optional Gallery Activity observer and the private
  [P12 editor ownership probe](../../../../../docs/assessments/2026-09-16-client-editor-datastar.md),
  not a new required consumer dependency. P12 calls the exported GET action without
  declarative expressions; it does not establish expression-compiler CSP parity.
- Upgrade gate: review source/API/security changes; repeat asset hash, wire contract,
  lifecycle/redaction, payload budget and actual-browser CSP/interaction checks.
  Source review and Node/HTTP tests do not establish browser conformance.

The pin includes nonce-based expression compilation (`html[data-nonce]`). The
Gallery does not enable `unsafe-eval` or `unsafe-inline`. Do not remove its nonce
as a workaround for a failed browser check. Transport options intentionally use
an empty form, zero retries, explicit cancellation and app-owned visibility
handling; the upstream defaults do not meet this specimen's wire/lifetime contract.
