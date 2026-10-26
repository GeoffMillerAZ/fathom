# Catalog Kit verification

Date: 2026-10-20, America/Phoenix. Candidate based on agentic-kit
`5d4c989ce18df24f91f479b7e11ab1e2b3b386de`.

Toolchain used: Go 1.26.0, CUE 0.16.1, Task 3.44.1. The optional format adapter was
tested with Python 3.12 and PyArrow 20.0.0 in a separate temporary dependency path.
No external dependency was added to the root Go module.

| Check | Observed result |
|---|---|
| `task validate` after final Go/CUE changes | Pass, exit 0. Includes format, vet, all CUE packages/positive fixtures, shell checks, constitution lint, root race tests, intent-updater race tests and both nested modules. |
| Constitution linter | Pass, 903 findings checked; generated package catalog and README agree at 210 source packages. |
| `go test -race ./kits/catalog-kit ./cmd/catalogctl ./cmd/internal/catalog ./foundations/knowledge/catalog` | Pass within the repository gate; focused runs also passed. |
| CUE export of the shipped spec/policy | Pass; output resolves through the actual CLI. |
| CLI evidence preparation | One criterion prepared successfully without a provider call. An empty witness list refused with no stdout request bytes. |
| Real-source `scan-go` and package-facet query | Pass, exactly 210 package entries from the existing collector, plus distinct intermediate groups. Working-tree provenance was labelled `working-tree-candidate`, not an uncommitted tree falsely labelled as a published revision. |
| Go inventory order permutation | Same resolved bundle digest; duplicate package paths refuse. |
| Parquet adapter tests | Two test methods pass, including nonempty/empty typed roundtrips, lineage metadata and preserving prior output on malformed/duplicate input. |
| Actual CLI NDJSON to Parquet path | Pass through the optional adapter. Logical rows/metadata checked; cross-version Parquet byte identity is not claimed. |
| Four skill structural validations | Pass. |
| Independent read-only catalog-doctor trial | Correctly diagnosed a real resolver refusal without granting permissions or editing inputs; see `skill-trial.md`. |
| `git diff --check` | Pass. |
| `task downstream-check` | Command exits 0 after explicitly skipping all four absent checkouts: fathom/sextant, consumer-a, consumer-b and consumer-c. All four are untested. |
| Fathom plan verification | Unavailable: no Fathom MCP tools or CLI in this environment. Plan rows remain unchecked. |
| Live model quality / paid provider calls | Not run. No provider enabled. |
| Review Kit / Env Kit production migrations | Not run; work order CK-9 onward remains open. |

The first full run exposed a local CUE import-name issue in the new testdata; the
import now names its package explicitly. The final full run includes that repair.
No existing gate was weakened. The existing Markdown catalog format and provider
contracts retain their behavior.

This record establishes offline implementation and the stated real-source integration.
It does not establish consumer adoption, evidence-source truth, calibration, native
provider wire compatibility for the new workflow or hosted CI status.
