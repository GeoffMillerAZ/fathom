# Typed Go call edges — dissolving the O9 residue (ADR-021 §3)

Grounded by wf_d0588a59: emission is syntax-only (goextract collectCalls,
single-file, no go/types); resolution (callgraph/graph.go:156-242)
correctly refuses ambiguity — any method name with >1 receiver in the
node set yields no edge (graph.go:218-240). That honesty invariant stays;
this spec raises the ceiling so ambiguity becomes rare instead of
constant (every adapter's Complete/Stream/Ping; every Store-interface
call site).

Design (normative):
- **Enrichment, not replacement.** collectCalls and the existing
  resolution stay the universal baseline (all languages, degraded
  environments). A Go-only typed pass (golang.org/x/tools/go/packages,
  NeedTypes|NeedSyntax|NeedTypesInfo; promote to direct dependency) runs
  in fusion after extraction for Go services, resolving call sites via
  types.Selection and emitting **core.Edge records with kinds**:
  EdgeDirect (static target), EdgeIfaceDispatch (one edge per in-scope
  implementer, computed via types.Implements over the loaded package
  set), EdgeUnresolved (func values / higher-order — explicit, never
  silent). The syntax-derived Calls field is untouched — additive.
- **Verify honesty upgrade**: test_reaches consumes typed edges when
  present. A path that crosses an EdgeIfaceDispatch edge reports
  "reached via interface dispatch (N implementers)" — counts as reached
  for the gate (it is strictly better evidence than today's refusal),
  labeled distinctly in verdict detail so a reader sees the modality.
  The O9-residue cited-skip class becomes retirable: the spec's proof
  row re-verifies three real residue cases (adapter Complete;
  Store.PutClaims via contract suite; kit-style Ping) flipping from
  refusal to typed edges.
- **Performance gate** (kill criterion from vision 10): typed load of
  this repo ≤ 2× current ingest wall-clock on first run; per-package
  cache keyed by the package file-set content hash makes re-ingest
  incremental (only changed packages reload). Measured numbers recorded
  in the plan on completion.
- **Failure honesty**: a package that fails typed loading (build errors,
  missing deps) falls back to baseline edges for that package with a
  per-ingest counter — never blocks ingest, never silently pretends.
- ExtractorEpoch bump not required (Calls unchanged); typed edges are
  NEW observations — their absence on stale stores is visible via the
  edge-kind counters in server_stats.

## Tasks

- [ ] core.Edge gains Kind (direct/iface-dispatch/unresolved); store round-trips it; counters in server_stats {id: CT-1, symbol: Edge, package: internal/core, skip_test: true, skip: "struct field addition pinned by store contract round-trip tests; type anchors are the documented residue"}
- [ ] Typed enrichment pass: go/packages load per Go service, types.Selection resolution, implements-set computation, per-package content-hash cache, load-failure fallback with counter {id: CT-2, symbol: EnrichTypedEdges, package: internal/fusion}
- [ ] Edge emission correctness fixtures: direct call, iface dispatch (small interface with 2 impls → 2 edges), func value (unresolved), method value, embedded promotion — table-driven against a fixture module {id: CT-3, symbol: EnrichTypedEdges, package: internal/fusion, skip_test: true, skip: "fixtures drive EnrichTypedEdges through the pass entry; the table tests call it directly — anchor shared with CT-2, this row pins the fixture matrix as its own claim"}
- [ ] test_reaches consumes typed edges; dispatch-modality surfaced in verdict detail; three real O9-residue cases flip from refusal to typed reach (adapter Complete, Store contract-suite dispatch, Ping) {id: CT-4, symbol: TestReaches, package: internal/verify}
- [ ] Performance gate met and recorded: first-load ≤2× ingest baseline; cached re-ingest delta-only (measured, numbers in this file) {id: CT-5, symbol: EnrichTypedEdges, package: internal/fusion, skip_test: true, skip: "measurement row; the recorded numbers + the cache tests are the evidence"}
- [ ] blast_radius/find_callers/symbol_context surface typed edges (kind-labeled), docs updated {id: CT-6, symbol: TestReaches, package: internal/verify, skip_test: true, skip: "serving fan-out through MCP tools; pinned by tool-level tests with fixture stores"}

## Deferred

- [-] Cross-service typed resolution (fleet-level implements-sets) {id: CT-7, disposition: deferred, reason: "per-service scope covers the residue evidence; fleet-typed loading multiplies cost", trigger: "first cross-repo blast-radius miss attributable to per-service scoping"}
- [-] SSA/pointer-analysis precision for func values {id: CT-8, disposition: deferred, reason: "EdgeUnresolved is honest; pointer analysis is an order of magnitude in cost", trigger: "unresolved-edge counters showing a material share on real queries"}
