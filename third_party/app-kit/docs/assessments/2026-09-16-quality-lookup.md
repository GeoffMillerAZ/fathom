# Bounded authored-quality lookup

G1/G4 implementation of ADR-0020/0069. The existing MCP server now offers
`list_quality_work` and `get_quality_work`; no additional service, inventory,
dependency or public UI component was introduced. Installed servers are unchanged.

## Behavior and limits

The reader embeds and validates the existing CUE inventory and schema, joins exact
capability IDs/profiles to the embedded catalog, and returns detached typed values.
Compact discovery defaults to the five core areas; `on-demand` selects recorded
non-core work, not every commercial component. Exact detail preserves all authored
standard/gate statuses, reasons, next steps, evidence, reviews, pins, selection and
consumer integration fields. Standard definitions and backlog policy travel with it.

Canonical content revisions ignore formatting/item ordering but change with metadata.
An optional expected revision refuses stale selection. Unknown IDs/profiles/scopes,
duplicate work IDs and oversized results fail; there is no silent truncation or
readiness percentage. Bounds:32 items,64KiB list and128KiB detail per representation.
MCP provides equivalent text and structured JSON, so total wire size includes both.

Every result says `authored-planning-not-certification`. Runtime lookup does not open
referenced files, recheck their hashes, execute gates, prove adoption or promote a
component. The existing inventory tests retain their separate file/pin validation.
Raw document resources can come from a newer kit-root checkout than the embedded
binary snapshot; rebuild/review before upgrading a host. The later [Gallery quality
view](2026-09-16-quality-gallery.md) is verified separately; current catalog schemas
and tools retain their existing contracts.

## Verification

- Reader tests cover faithful typed projection, canonical revisions, detached nested
  values, scope/capability joins, empty known matches, stale/malformed selection,
  invalid schema/duplicates/profile fit, retained optional fields and oversized
  list/detail refusal. A synthetic nonexistent receipt remains an authored reference,
  demonstrating that lookup does not pretend to authenticate it.
- In-memory MCP tests cover advertised typed/local/read-only tools, all item/scope
  projections, all capability joins, text/structured equality, exact integration
  statuses, input refusals and startup/guidance routing.
- Full qualityplan/MCP race suites:381 passing test events, zero failures, four
  pre-existing opt-in private/live probes skipped. This is not a full root test run.
- A private standalone build passes. Its actual stdio process initializes and serves
  five core records plus exact analytics detail, rejects a stale revision and keeps
  the deferred candidate separate. A sandbox denies network and both owner code
  roots; a source-read refusal canary passes before startup. Empty kit-root guidance
  does not prevent embedded lookup. This is not installed Claude/Codex adoption.
- That process returns1936-byte list/8270-byte detail JSON, exits0 without a signal,
  and leaves its executable unchanged. These are payload observations, not latency,
  general performance, native UI or consumer-quality certification.

## Retained runs

Initial root `/Volumes/m2ext/appkit-evidence/appkit-quality-lookup.NzILgyxn/` retains selected source,
commands and outputs. Its first race run failed an existing CSS test because the
minimal environment omitted the normal home/cache location (380 passes/1 failure/4
skips). The source stayed unchanged. Its initial stdio preparation also stopped
before launching the server: `/usr/bin/test` does not exist on this Mac. The diagnostic
and original harness remain; a fresh harness uses `/bin/test`, corrected JavaScript
budget parentheses and UTF-8 stream decoding. No task expectation was weakened.

The corrected race root `/Volumes/m2ext/appkit-evidence/appkit-quality-lookup-env.uQ9hE0MY/` inherits
the unchanged normal home location and checks the already-cached Tailwind4.3.0 binary
against its existing pin before running. No compiler download/upgrade was needed.
Actual stdio proof: `/Volumes/m2ext/appkit-evidence/appkit-quality-stdio.qXZ9W51E/stdio/`.
Receipts retain selected source identities, not a full clean-build dependency closure.

| Artifact | SHA-256 |
|---|---|
| Corrected race receipt | `a0da8d4ebdf47ab61ecf55d10e45ddf2e8cb07d88a93a8dee07b578bb4852e37` |
| Standalone build receipt | `34bb25bc2dca835c242364fdbb87d2ff3c8a696ed96c4f9c940fd1ab9baf58af` |
| Tested MCP executable | `4e61444528dc3bea4d8df483214bd251be346d7c1aba38561258df85df014aaa` |
| Actual stdio receipt | `5252c193a646d340495462f42994244054b09d92dca12f8bc4f115a88fc994c2` |
| Authored inventory content revision | `5e352c7e04db5a10267b6fee8a2c93a871efa2119ae04eb5ce299e884afef1be` |
