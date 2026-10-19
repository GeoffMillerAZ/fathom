# Native CUE registry-denial policy

Native CUE typed enrichment supports the admitted local module and builtin
packages. It now supplies an explicit refusal registry; `CUE_REGISTRY=inline:{}`
was not an offline switch in the pinned CUE loader. External modules are refused,
including modules already present in an operator's persistent cache. No registry,
credential reader, transport or persistent module-cache client is constructed by
this boundary. This does not sandbox local filesystem reads.

The CUE pass now has an independent policy epoch. A completed/partial state from
before this policy is rechecked on the next ordinary ingest even when no source
changed. Successful current-policy passes resume normal unchanged-input gating.
No extractor epoch, database schema or other producer namespace changes.

CUE can collapse a failed multi-package load into a single synthetic instance
and discard the registry error's type. When the explicit registry observes a
denial, enrichment therefore performs at most one native retry for each stored
CUE package identity. Only identities whose own retry invokes that refusal
registry lose prior typed rows and fingerprints. Transitive dependents are
identified by their own native loads, not inferred from error text. Healthy local
packages, other producers, and the existing ordinary compile-failure retention
policy remain unchanged. Absolute-directory fallback identities must resolve
physically inside the admitted root before a relative package selection is used.

An incomplete, cancelled or failed inspection names the failure and leaves no
completed CUE pass stamp, so unchanged input is retried. Some old rows may remain
after such an incomplete inspection; do not interpret it as a complete invalidation
or a current typed proof. The native loader itself has no cancellation API;
cancellation is checked between its finite per-package invocations. Ordinary full
ingest updates the store watermark, and MCP ingest/refresh evicts the existing
service graph memos. Direct calls to `EnrichCUEEdges` write the store only; callers
that hold their own graph snapshots must refresh those snapshots themselves.
