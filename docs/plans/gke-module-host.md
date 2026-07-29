# GKE Module Host — read-only federation over N conforming stores

Design (normative — see `docs/vision/12-governed-knowledge-engine.md` §3 for
the architecture decision this plan implements, and §1 for the owner
decision that pulls this forward ahead of `docs/design/product-vision.md`
§4 H3's external-adoption entry condition; that entry condition is
**unmet** and this plan does not claim otherwise):

- **What "module host" means.** `fathom serve-http` mounting N conforming
  module stores (`docs/spec/store-format-v0.md` + `docs/spec/
  expert-module-v0.md`) **read-only**, as one surface. Not a new binary,
  not a second engine — `docs/design/expert-modules.md` §1's "there is no
  second engine, no second schema" extended across store boundaries. Every
  answer carries per-module identity + evidence tier + staleness
  (`store-format-v0.md` §3 Server conformance, generalized to N stores
  instead of one).
- **Mounting mechanism.** A multi-store query layer behind `pkg/platform`'s
  existing cross-service graph machinery (`pkg/platform.Build`,
  `pkg/platform.BuildFromStore`, the primitive
  `pkg/platform.CrossRepoBlastRadius` already generalizes) — this is
  `ATTACH`-shaped in spirit (`store-format-v0.md` §11 names exactly this
  shape as living "at a layer *above* this format"), not a rewrite of the
  store engine and not the DuckDB-specific `ATTACH` mechanism
  `product-vision.md` §4 H3 describes (that description predates ADR-025's
  SQLite-only reality — this plan targets SQLite multi-store query
  composition, the technical re-check `product-vision.md`'s engine-
  assumption note already calls for).
- **Conformance at mount, not at query time.** A store is checked against
  `store-format-v0.md`'s file-identity markers (`application_id =
  0x46415448`) and the conformance level the host requires (at minimum L0 +
  Server) the moment it is mounted — never lazily discovered mid-query. A
  store that fails this check is refused loudly; it is never silently
  served as if it were empty or partially trusted (doctrine 3, "never
  silent").
- **Surface exposure — the doctrine-4 argument (decided here, not
  deferred).** Two options were on the table: (a) extend the existing
  `/rpc/agent` 16-tool allowlist (`docs/mcp-tools.md`) with module-scoped
  variants of `obligations`/`staleness_report`, or (b) a new scoped surface,
  `/rpc/modules`, serving a small, module-host-specific tool set. **This
  plan picks (b), a new scoped surface, not new tools on `/rpc/agent`.**
  Reasoning: `/rpc/agent`'s allowlist is a single-store-per-call contract
  today (every tool takes one `service`) — bending it to mean "one module,
  chosen from a federated set" would be a silent behavior change on an
  already-registered, already-in-use surface (the same class of risk the
  `read_source` addition (T5-4, `docs/plans/trust-and-tokenomics.md`)
  deliberately avoided by adding a new tool rather than overloading an
  existing one). A new surface, gated behind its own explicit opt-in
  (nobody's `~/.claude.json` points at `/rpc/modules` until they choose to),
  costs zero regression risk to `/rpc/agent`'s existing 16-tool contract and
  keeps doctrine 4's "no net tool growth" honest: `/rpc/modules` reuses the
  *same* tool shapes (`obligations`-pattern scope lookup, staleness report)
  federation-wrapped, not new tool verbs — the net-new surface is a scoping
  boundary, the same kind of move `/rpc/agent` itself already was
  (ADR-014), not a new capability contract. No tool ships under this plan
  without this paragraph's reasoning holding; if a reviewer finds it doesn't
  hold, `/rpc/agent` extension is the fallback, not silently taken as the
  default.
- **Auth + read-only hardening are launch requirements for hosted, not
  later hardening** (`docs/vision/12-governed-knowledge-engine.md` §2 law
  9). The local, loopback-only, unauthenticated daemon (`docs/mcp-tools.md`)
  stays exactly as it is — this plan's hardening rows apply only to a
  network-reachable deployment, which does not exist until they are proven.

## Tasks

- [ ] Mount N conforming module stores read-only; each mount conformance-checked at mount time against the store-format-v0 markers (`application_id`/`user_version`) and required-schema tables for the level the host requires {id: GKE-MH-1, symbol: BuildFromStore, package: pkg/platform}
- [ ] Unmount a module cleanly (host continues serving the remaining set; no stale reference to the unmounted store's connection survives the call) {id: GKE-MH-2, symbol: BuildFromStore, package: pkg/platform}
- [ ] Federated query layer: a scope-lookup query spanning mounted modules returns claims tagged with their owning module's identity, never merged into an ambiguous unattributed set {id: GKE-MH-3, symbol: Graph, package: pkg/platform}
- [ ] Per-module provenance on every served payload: module identity, evidence tier, and status (fresh/stale/contradicted) present on every claim, mirroring store-format-v0.md §3 Server conformance generalized to N stores {id: GKE-MH-4, symbol: Graph, package: pkg/platform}
- [ ] `/rpc/modules` scoped surface registered, serving the module-host-specific tool set decided in this plan's design section (obligations-pattern scope lookup + staleness, federation-wrapped) {id: GKE-MH-5, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "surface-registration row; anchored to the existing surface-registration precedent (A1-1, docs/plans/activation.md) pending the module-host's own registration function, which does not exist yet"}
- [ ] Staleness honesty per module: a federated answer's reported freshness is never fresher than its least-fresh contributing module; each module's own watermark drift is independently detectable from stored data alone (store-format-v0.md §6, L1 conformance, generalized) {id: GKE-MH-6, symbol: Graph, package: pkg/platform}
- [ ] Auth for the hosted deployment mode: a network-reachable module host refuses an unauthenticated request, mirroring the local daemon's loopback-only trust boundary being replaced with an explicit one {id: GKE-MH-7, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "hardening row; no auth code path exists yet — anchored to the surface layer it will gate, same as GKE-MH-5"}
- [ ] Read-only mode enforced at the store-open layer for the hosted deployment: a mounted module store cannot be written by the hosting process under any code path reachable from the module-host surface (law 9) {id: GKE-MH-8, symbol: Open, package: internal/store, skip_test: true, skip: "hardening row; store.Open today always runs createSchema (store-format-v0.md §1.3's documented honest gap) — this row depends on that gap closing first, tracked here rather than assumed"}
- [ ] Conformance refusal tests: a non-conforming store (wrong `application_id`, missing required table, `.duckdb` extension) is refused loudly at mount — never silently degraded to an empty or partial module {id: GKE-MH-9, symbol: BackendForPath, package: internal/store}
- [ ] Conformance refusal tests: a conforming-but-insufficient-level store (e.g. L0 only, mounted where the host requires Server conformance) is refused with a named reason, not silently served at a lower honesty bar than the host's contract promises {id: GKE-MH-10, symbol: BackendForPath, package: internal/store}
