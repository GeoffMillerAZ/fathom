# Unreleased: source-bound SCIP imports

Existing `scip-dotnet` imports now require a source receipt. An older index
without one is refused rather than reused as current typed evidence. Other
producers keep their existing behavior unless a sidecar opts the index in.

Before upgrading a service that imports C# SCIP, use a fresh store or explicitly
rebuild the affected service. Older stores did not retain each index's producer
identity, so missing pre-upgrade artifacts cannot retroactively become
source-bound. Update the local producer
runner to publish `INDEX.scip.receipt.json` using the
[source receipt contract](../guides/scip-source-receipts.md). Regenerate the
index against the actual project, source inventory, compiler configuration,
reference assemblies and pinned producer package. Do not manufacture fresh
hashes around an old index.

The runner must mark the receipt `started` before generation and publish
`complete` last. Ingest or refresh the service after generation. A refusal is
an error and clears that index's previous typed rows and graph memo; unrelated
typed indexes are preserved. Fix the named input or rerun the producer before
retrying. Syntax updates alone do not make a refused typed import successful.

Receipt checks run at import and refresh boundaries, including daemon and
file refresh. They do not continuously monitor disk or attest test execution.
