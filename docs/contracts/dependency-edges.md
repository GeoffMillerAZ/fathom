# Dependency edges contract

Status: accepted design, 2026-11-18 (ADR-114). The Go and JSON surfaces ship after
review of the public door. See the [design](../design/dependency-edges.md) for the
preregistered admission bars.

## Read and consume

`pkg/lang/deps` reads one file from bytes or a set of paths from a caller-supplied,
read-only repository `fs.FS`. A file record names its repository-relative path,
dialect, measurement and completeness, then its direct edges and unknowns.
`fathom-lang deps` serializes the same records as JSON. Edges and unknowns are
sorted arrays, including when empty. Identical repository inputs produce identical
bytes across runs and checkout paths.

A file is complete only if the reader accounts for every dependency construct in
its dialect and resolves every one. Any unknown leaves the file incomplete. A
reader without a passing admission is unmeasured. Its files remain incomplete
unless the consumer explicitly trusts the dialect by name and the result records
that trust. A dialect with a failed soundness reading is detect-only. A consumer
must widen or run whenever the files in its closure include incomplete evidence.

A reader may reuse `pkg/lang` reference and unresolved evidence. It does no
network reads, starts no toolchain and computes no closure or hash. It resolves
using the supplied repository view and keeps absolute paths out of every output,
including lexical diagnostics and untrusted extension returns. Raw dependency
operands are kept only when safe to disclose. Values covered by the library's
redaction rule are withheld.

## Serialized facts

An edge names the importing path, kind, safe raw operand, target, resolution,
version if pinned, one-based source line and provenance. Paths are logical
repository-relative slash paths. An external ID is `ecosystem:name`. A version
comes only from a lockfile or manifest pin. Unknown targets carry a typed reason.
An unknown names its file, code, safe raw operand, line and possible evidence
sources. An empty `decidable_by` grants no trust.

| Field | Closed values |
|---|---|
| Kind | `import`, `include`, `embed`, `build-input`, `exec`, `config-ref` |
| Resolution | `file`, `directory`, `external`, `unknown` |
| Provenance | `builtin`, `pack`, `extension`, `declared` |
| Unknown code | `dynamic-import`, `computed-path`, `unresolved-alias`, `missing-config`, `unbounded-glob`, `unsupported-construct`, `parse-error`, `unmeasured-reader`, `external-unpinned`, `read-limit` |
| Decidable by | `toolchain`, `lockfile`, `config-file`, `observation`, `declaration` |

Extending a closed set requires architect review. The public Go API snapshot
and the CUE schema with valid and invalid fixtures hold the reviewed shape.

## Declared-edge exchange

The export identifies a source unit and target unit, each by ecosystem and logical
name, in the declared-edge shape consumed by the CI kit's dependency graph.
It exports direct edges and preserves the caller's responsibility to check
completeness before using any result as a skip key. This exchange is data only.
A pack, registered Go resolver or caller-declared edge list can supply facts.
Every edge discloses its provenance. Consumers can grade their extensions with
the conformance helper.
