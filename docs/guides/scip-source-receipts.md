# Source-bound SCIP imports

SCIP supplies typed references. A source receipt binds an index to the actual
source, generated projects, compiler configuration and reference assemblies
used by its external producer. It is a deterministic freshness declaration,
not a signature, a compiler execution attestation or a language excellence grade.
Fathom does not execute or install the producer.

`scip-dotnet` indexes require a sidecar at `INDEX.scip.receipt.json`. Other
producers retain legacy behavior without a sidecar; providing one opts that
index into the same contract. Do not copy a receipt from another index or
refresh its hashes without rerunning the producer against those inputs.

The closed JSON schema is `fathom.scip.source/v1`:

```json
{
  "schema": "fathom.scip.source/v1",
  "status": "complete",
  "root": "/absolute/local/repository",
  "index_sha256": "64 lowercase hexadecimal characters",
  "producer": {
    "name": "scip-dotnet",
    "version": "the index metadata's actual version",
    "distribution_version": "the installed distribution's version",
    "package_path": "/absolute/path/to/pinned/producer.nupkg",
    "package_sha256": "64 lowercase hexadecimal characters"
  },
  "inputs": [
    {"path": "src/Worker.cs", "sha256": "64 lowercase hexadecimal characters"},
    {"path": "Actual.csproj", "sha256": "64 lowercase hexadecimal characters"},
    {"path": "/absolute/path/to/reference.dll", "sha256": "64 lowercase hexadecimal characters"},
    {"path": "/absolute/path/to/pinned/producer.nupkg", "sha256": "64 lowercase hexadecimal characters"}
  ],
  "inventories": [
    {"root": "src", "pattern": "*.cs", "paths": ["src/Worker.cs"]}
  ]
}
```

The example hashes are placeholders, so this example deliberately cannot pass.
Paths in `inputs` may be repository-relative or absolute local files. External
SDK/reference files must be named individually; inventory roots must stay
inside the repository. Inventories recurse and accept a simple `*.extension`
pattern. Their paths are repository-relative or absolute. Add separate
inventories for assembly declarations or other producer inputs as appropriate.
The limits are 64 MiB per source-bound index, 8 MiB of receipt JSON, 20,000
input files totaling at most 1 GiB, and 100,000 walked entries per inventory; exceeding a limit refuses the typed import.

The root must match both the service and SCIP metadata. The producer name and
metadata version must match the index. Every SCIP document must have a current
source hash and belong to an unchanged inventory. Every inventory member also
requires an input hash, including files with no SCIP definitions. Every declared input is
hashed, including generated `.csproj` configuration/defines and referenced DLLs.
For scip-dotnet, a project hash, reference assembly hash and a bound distribution
package identity are mandatory. The producer must enumerate its real input
closure; Fathom checks that declaration, it does not implement MSBuild or infer
undeclared compiler dependencies.

A producer writes `status: "started"` **before** attempting regeneration, then
atomically publishes `complete` only after the producer succeeds, diagnostics
and document coverage pass, and all input hashes still match. A failed attempt
must leave its sidecar incomplete, even when an older index remains on disk.
Publish the completed receipt last so partial writes cannot look current.

CLI ingest, direct client import, MCP ingest and daemon refresh share the same
validation. Root `index.scip`, selected state-directory `scip/*.scip` indexes
(with `.fathom/scip/*.scip` legacy fallback), and previously registered
source-bound indexes are reconciled after ingestion. A missing/stale/mixed or
incomplete receipt refuses typed enrichment, clears only that index's previous
typed rows and evicts its served graph memo. Other producers' rows remain.
A refused source-bound import makes ingest fail even when syntax was updated;
read the diagnostic rather than interpreting the syntax update as a full pass.

State-directory selection uses the same `statedir.SCIPIndexes` precedence as
CLI ingest. A preferred state-directory file shadows the same legacy basename;
remembered typed rows for that shadowed legacy index are cleared, while its
registration and both artifacts remain. A malformed preferred index does not
fall back to the legacy copy. Root `index.scip` stays independent. This does not
move old files, rewrite receipts or claim a source binding for an older store.

Freshness is checked at import/refresh boundaries, including file refresh.
This does **not** continuously hash the filesystem on every query. Before a
refresh completes, the normal index-staleness/basis rules still apply. It does
not prove callback execution, serialized scene references, remote behavior or
successful tests. Raw local producers and source receipts stay distinct from
runtime test evidence.

Stores created before this contract did not retain the producer identity of
each imported index. If both an old artifact and its receipt are already gone,
that identity cannot be reconstructed from its edges. Upgrade with a fresh
store or explicitly rebuild the affected service; do not present an unknown
pre-upgrade typed store as source-bound. Once registered under this contract,
an index remains remembered even if both files disappear.

## Decision models

None applies. Content hashes, inventories, identity matching and namespace
invalidation are deterministic local checks; no provider or model egress is used.
