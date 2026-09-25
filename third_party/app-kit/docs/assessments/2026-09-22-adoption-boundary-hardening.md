# Adoption boundary hardening

## Bounded maintenance batch

- Packet: owner-resumed R1/R4 adoption and analytical correctness, base `4b4a06d`.
  Two source-reviewed boundary fixes plus a full-gate fixture repair; no new consumer.
- Needs: `get_capability` returns its current revision but cannot require the revision
  from a prior search; callers must detect a changed selection themselves. Analytics
  promises an explicit IANA zone or UTC but admits Go's host-dependent `Local` sentinel.
  The full local gate also fails when a positive host-snapshot test supplies macOS's
  `/var` alias instead of its production caller's canonical root.
- Deliverables: an optional exact MCP revision precondition; explicit-zone refusal
  before analytics output; a canonical positive snapshot fixture with a real source-file
  symlink refusal control. Preserve default MCP output and valid explicit-zone output.
- Authority: kit source/tests/docs, scoped commits/push only. No installed-host/global
  config, consumer changes, dependencies, paid calls, native runtime or deployment.
- Intent: I11 explicit analytics meaning, I15/I17 evidence limits, I18 honest selection;
  ADR-0020/0065 metadata identity and ADR-0069/0072 owner-funded bounded maintenance.
  No new catalog truth, promotion, future or runtime is introduced.
- Compatibility: MCP adds an optional input only. Analytics now refuses an input that
  violates its documented explicit-zone contract; callers using `Local` must select
  their real IANA zone or UTC. No automatic timezone substitution or consumer repinning.
- Acceptance: actual MCP roundtrips for omitted/current/stale/malformed preconditions,
  schema and post-error recovery; analytics range/snapshot validation and no-output
  refusal plus valid-zone controls; snapshot positive/negative controls; scoped races
  and complete `task check`, retaining any failed attempts. No browser/host-quality claim.
- Cost/stop: two bounded source/test agents, one independent diff review; no browser
  acquisition. One baseline full gate and one final gate, with a further run only for
  a diagnosed correction. Stop rather than add unrelated feature or consumer work.
- Owner: app-kit maintainer. Evidence packet:
  `/Volumes/m2ext/appkit-evidence/appkit-adoption-hardening.qEIxhM`; base archive and executed red/green
  source/logs remain private. Return the shared refusals, tests and migration guidance.

## Results

### Explicit analytical timezones

`Window.validate` now rejects the reserved `Local` value before calling
`time.LoadLocation`. Go's own API documents that this value returns process-local
configuration, unlike the package's promised explicit reporting zone. No zone is
guessed or substituted. Both public renderers and CSV already share this validator.

The retained RED test shows range and snapshot datasets accepted `Local`. The same
test now checks validation, both renderers and CSV refusal without output; positive
UTC-range and America/Phoenix-snapshot controls assert independently authored HTML/CSV
labels. Two tests/four subtests and the complete analytics race suite pass. The test
bytes did not change between RED/GREEN. No actual multi-host acquisition or timezone-
database pinning is claimed; explicit IANA locations still use Go's available zone data.

### Full-gate snapshot fixture

Baseline `task check` passed root vet and the other root Go packages but stopped at
`TestHostBundleSourceSnapshotRetainsExactGitVisibleBytes`: the fixture's `/var` temp
root compared unequal to its `/private/var` resolution. `rehearseHost` already
canonicalizes its source root before calling the unchanged `readHostSource` helper.
The positive fixture now follows that same precondition. Its exact file-count/body/hash
and unexpected-path assertions remain; a new negative control requires a real
source-file symlink to fail with no returned snapshot. Both focused tests pass.
This is fixture repair, not a native runtime change or evidence of a new host build.

### MCP exact lookup and final batch gate

`get_capability` now accepts optional `expected_revision`. A supplied value must be
64 lowercase hexadecimal characters and match the server's embedded catalog before
lookup or metadata-pin generation. A mismatch directs the caller to restart search.
Omitted/empty values retain current-snapshot behavior and the existing output shape.
This is metadata consistency across discovery/reconnect, not source or host freshness.

The actual in-memory MCP protocol test covers schema optionality, current/omitted
output equivalence, short/uppercase/nonhex/whitespace/stale refusal, no structured or
text-content entry/pin leakage, and successful reuse of the same session afterward.
The stale fixture changes a nibble of the real revision, guaranteeing a mismatch.
The pre-review test and original RED remain retained separately. Focused tests and
existing catalog parity/refusal suites pass; independent cross-review found no blocking
issues. No installed MCP binary or host registration was refreshed.

Final `task check` passes: root Go vet/tests, declared CUE and Node unit suites,
all shipped theme generation and generated-output cleanliness. Default opt-in skips
remain; this command does not run every browser/native acquisition. Scoped race suites
pass for analytics, MCP, qualityplan and the releasecandidate rehearsal. No generated
CSS/theme bytes changed. The baseline failure remains in `task-check-baseline.log`.

## Reproduction and retained identities

Run `task check`; focused entrypoints are
`TestGetCapabilityExpectedRevisionGuardsExactReview`,
`TestLocalTimeZoneIsRejectedBeforeOutput`, `TestExplicitTimeZonesRetainExpectedLabels`,
`TestHostBundleSourceSnapshotRetainsExactGitVisibleBytes` and
`TestHostSourceSnapshotRejectsFileSymlink` in their source packages. The source archive,
original/intermediate/final test bytes and separate logs in the private packet bind
the results; no failed artifact was overwritten. The direct patch tool twice refused
one external-volume read, while ordinary reads and the local `apply_patch` entrypoint
worked; no filesystem permissions or host policy were changed.

| Final source | SHA-256 |
|---|---|
| analytics.go | `eb7e6b5685604a55edea9b13b51d74014d19faae6dcf53b737ea9ee23424d181` |
| timezone_test.go | `5bd572c7a0f7c4612929e3afe2acfae2b41a9020aa3d06eaf18ecc94ca29038d` |
| catalog_tools.go | `aeaa75259b27726f0703ae000e80407733a69b8410c22930414b178b663f1525` |
| catalog_tools_test.go | `4351ce3bb87b784f82934b9296266a5060412ededa3a966eda5b3b9db740c464` |
| host_test.go | `5c80446816adcd6423e05d25ced5fceb8ac7d363b8ee47b6f809d173feb72873` |

Contribution disposition: returned to the kit's shared validation/protocol and test
seams, with adoption/analytics migration guidance and a CUE evidence reference.
Originating and second-consumer integration are not claimed. Catalog IDs, metadata
pins, maturity and authored partial gates remain unchanged; quality-inventory revision
changes because its evidence reference changed. No host, browser, human or release
acceptance is inferred from these source checks.
