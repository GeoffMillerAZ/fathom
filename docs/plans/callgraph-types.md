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

- [x] core.Edge gains Kind (direct/iface-dispatch/unresolved); store round-trips it; counters in server_stats {id: CT-1, symbol: Edge, package: internal/core, skip_test: true, skip: "struct field addition pinned by store contract round-trip tests; type anchors are the documented residue"}
- [x] Typed enrichment pass: go/packages load per Go service, types.Selection resolution, implements-set computation, per-package content-hash cache, load-failure fallback with counter {id: CT-2, symbol: EnrichTypedEdges, package: internal/fusion}
- [x] Edge emission correctness fixtures: direct call, iface dispatch (small interface with 2 impls → 2 edges), func value (unresolved), method value, embedded promotion — table-driven against a fixture module {id: CT-3, symbol: EnrichTypedEdges, package: internal/fusion, skip_test: true, skip: "fixtures drive EnrichTypedEdges through the pass entry; the table tests call it directly — anchor shared with CT-2, this row pins the fixture matrix as its own claim"}
- [x] test_reaches consumes typed edges; dispatch-modality surfaced in verdict detail; three real O9-residue cases flip from refusal to typed reach (adapter Complete, Store contract-suite dispatch, Ping) {id: CT-4, symbol: TestReaches, package: internal/verify}
- [x] Performance gate met and recorded under the AMENDED criterion (owner ruling 2026-07-22, see vision 10): typed first run ≤1s absolute and ≤10× true syntax baseline, cached re-ingest delta-only — measured 0.42–0.67s warm plain (~3.3× a 0.12–0.18s baseline), 211/211 cache hits on rerun {id: CT-5, symbol: EnrichTypedEdges, package: internal/fusion, skip_test: true, skip: "measurement row; the recorded numbers + the cache tests are the evidence"}
- [x] blast_radius/find_callers/symbol_context surface typed edges (kind-labeled), docs updated {id: CT-6, symbol: TestReaches, package: internal/verify, skip_test: true, skip: "serving fan-out through MCP tools; pinned by tool-level tests with fixture stores"}

## CT-5 measured numbers (2026-07-22 fix round, this repo, self-hosted)

**Correction (critic F4).** The originally recorded numbers here (baseline
5.208s, ratio 0.33×) were DOUBLY contaminated: the "syntax-only baseline"
ran the then-unconditional typed hook inside `Ingest` (measuring typed vs
syntax+typed), and that first-ever run also paid a cold-GOCACHE `go list`.
`Config.DisableTypedEdges` now exists precisely so a true baseline is
measurable. Honest numbers, measured by
`internal/fusion.TestTypedEdges_PerformanceGate` (523 Go files / 4444
symbols, 211 go/packages entries incl. Tests:true variants, Apple Silicon
dev machine, warm caches, best of 2):

| measurement | -race (validate env) | plain (production-representative) |
|---|---|---|
| true syntax-only ingest baseline (MemoryStore) | 0.789–0.898s | 0.119–0.175s |
| typed first run (cold per-pkg cache) | 1.664–1.848s | 0.418–0.666s |
| typed fully-cached run | 0.887–0.948s | 0.376–0.392s |
| on-disk SQLite variant: baseline / typed first | — | 0.191–0.260s / 0.644–0.802s |

- **Cached re-ingest is delta-only: MET.** 211/211 cache-hit, 0 packages
  reloaded; touching one file reloads exactly its own package (+ its
  test-augmented variant). The cached floor (~0.38s plain) is the
  go/packages driver's own `go list` metadata pass, which re-runs on every
  Load regardless of our cache.
- **The literal ≤2× criterion: NOT MET — measured ~3.2–3.5× warm (2.0–2.1×
  under -race, which slows the parallel syntax pipeline more than the
  subprocess-dominated typed pass).** Structural finding, not a tuning gap:
  the fully-cached run — pure driver overhead, ZERO resolution work —
  already costs ≈2× the syntax baseline by itself, so no optimization of
  the resolution code can reach 2×; only replacing the `go list` driver
  with a custom loader could. The criterion was authored against a believed
  ~5s ingest baseline (the contaminated number above); the true baseline is
  ~0.15–0.25s, which the fixed driver cost cannot get under.
- **Absolute cost for perspective:** the typed pass adds ~0.4–0.8s to a
  first ingest and ~0.4s per drift-triggered re-ingest (production, warm) —
  the order-of-magnitude blowup vision 10's kill criterion feared (whole-
  program analysis, 10×+) is not present. The gate test enforces exactly
  that blowup guard (≤10×) and logs the literal 2×-criterion ratio on every
  run, so the number stays visible rather than asserted-red or fudged-green.
- **Pending owner decision (why this row is [~]):** either amend the
  criterion to the measured envelope (e.g. "typed first run ≤ 1s absolute
  and ≤ 10× baseline; cached delta-only") or fund a custom driver. All
  other CT-5 deliverables (the switch, the cache, the honest measurement)
  are in.

## Deferred

- [-] Cross-service typed resolution (fleet-level implements-sets) {id: CT-7, disposition: deferred, reason: "per-service scope covers the residue evidence; fleet-typed loading multiplies cost", trigger: "first cross-repo blast-radius miss attributable to per-service scoping"}
- [-] SSA/pointer-analysis precision for func values {id: CT-8, disposition: deferred, reason: "EdgeUnresolved is honest; pointer analysis is an order of magnitude in cost", trigger: "unresolved-edge counters showing a material share on real queries"}
