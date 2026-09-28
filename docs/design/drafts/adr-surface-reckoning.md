# Draft ADR text — the surface reckoning and the support matrix

Draft for the orchestrator to number and merge into `ADR.md` (this clone may not edit it).
Two records: A (the surface reckoning) and B (the support matrix). Designs:
`docs/design/surface-reckoning.md`, `docs/design/support-matrix.md`. Plans:
`docs/plans/surface-reckoning.md` (SR), `docs/plans/support-matrix.md` (SM).

---

## ADR-0XX: The surface reckoning — eight tools by default, zero-use tools deleted, every removal proven absent (2026-09-27)

**Status.** Proposed 2026-09-27 from the owner-accepted feature scorecard (86 features:
30 keep, 24 improve, 12 merge, 10 ditch, 10 new). Amends ADR-014 (surfaces), ADR-035 (the
promoted verbs), ADR-046 (frozen). Design: `docs/design/surface-reckoning.md`. Plan:
`docs/plans/surface-reckoning.md`.

**Context.** The registry serves 59 tools on bare `/rpc` (63 with the module host's four)
and 25 on the agent surface, 18,310 bytes of schema before an agent asks anything. The call
log (97,898 calls) shows the load concentrated on a dozen tools; 30-odd registrations have
0–5 lifetime calls, and the most accurate impact verbs (blast_radius: precision 1.00,
0.006 s) are among the least called. Law 4 forbids net growth; it has never forced a
shrink.

**Decision.**
1. *The default agent surface is eight tools*: `find_symbol`, `search`, `read_source`,
   `symbol_context`, `obligations`, `verify_plan`, `blast_radius`, `review_diff`, at
   `/rpc/agent`. A path-scoped `extended` surface (21 tools) is the opt-in; `/rpc/companion`
   aliases it. Surfaces stay execute-and-flag, so nothing an agent calls stops working;
   off-surface demand is read from the call log after fourteen days (SR-M3).
2. *Deleted*: `cross_repo_blast_radius`, `dep_drift`, `http_client_calls`, `fleet_drift`,
   `reachable`, `traverse`, `history`, `memory_search` with all of anchored memory, the
   MCP `brief` tool, the `narrate` and `brief` CLI verbs, the four module MCP tools with the
   `/rpc/modules` surface and hosted-modules mode, twelve legacy tools (`complexity`,
   `god_object`, `community`, `dead_code`, `cloc`, `secret_scan`, `shell_out`,
   `proto_conformance`, `traceability`, `evidence_summary`, `contradiction_scan`, `batch`)
   and the four canned prompts.
3. *Merged*: `find_callees` into `symbol_context`; `entrypoints` into `repo_map`'s header;
   `capabilities`, `server_stats`, `list_sources` into one `status` tool; `rename_impact` and
   `safe_delete_check` into `blast_radius` (`kind: change|rename|delete`); `diff_impact`
   into `review_diff`; the `api_diff` tool into `conserve`; `arch_conformance`,
   `spec_coverage`, `spec_refs` into review internals (spec references ride the
   `obligations` answer the edit hook already fetches); `stale_docs` into
   `staleness_report`'s `doc_drift`; the `discoveries` tool into the session brief. CLI:
   the metrics verbs under one `metrics`, `providers`/`smoke` as a doctor row, the CUE
   shadow verbs out of help.
4. *Frozen, not deleted*: `fathom research` (ADR-046 decision 6 routes the retro corpus
   through its bundle) and `fathom assess` (this record marks ADR-046 frozen) leave help and
   stay callable until an owner pull or a superseding record.
5. *Every removal is proven absent.* A `gone:` contract on the registration literal (or the
   package import), a pinned registry test, callers evidence in the commit, the exemption
   check re-run, dependent claimed plan rows re-anchored (capability survives) or retired
   to `[-] refused` (capability gone), and a `### Breaking` line with a migration note.
6. *Counts after the reckoning*: 25 tools on bare `/rpc` (from 59), 8 on the default
   surface (from 25), 0 prompts (from 4); the default's schema at most 9,000 bytes.

**Alternatives rejected.**
- *Keep the tools, hide them from the default surface only.* Zero-call tools still cost
  schema bytes on `/rpc`, docs, tests and review; law 4 applies to the registry.
- *A hard gate on the default surface.* An error teaches an agent to abandon the surface
  and looks like no demand (ADR-014 amendment 2026-07-16).
- *Deleting the Expert Module system with its tools.* The manifest, `modules.cue` and the
  pack format are on the compatibility surface and used by layers; only the MCP tools had
  zero use.
- *Deleting `research` and `assess` outright.* Both sit under an accepted ADR with claimed
  rows; a freeze keeps the record honest until it is superseded.

**Consequences.**
- Breaking: every removed tool name, the `module_query` wire contract v0, the default
  surface's membership, the stanza (it no longer names `ingest`). One migration note,
  `docs/upgrades/0.9.0-surface-reckoning.md`.
- The compatibility surface line "the 19-tool agent surface allowlist" is replaced (it was
  already stale at 25).
- ADR-035's promoted verbs lose `rename_impact` (folded into `blast_radius`); the five-verb
  lead becomes two on the default surface (`blast_radius`, which now also answers rename
  and delete, and `review_diff`), with `test_impact` and `localize` on `extended`.
- ADR-046: status becomes *Accepted — frozen 2026-09-27* (verb out of help, capability off
  by default, no new rows) until an owner pull.

**Revisit when.**
- SR-M3 shows a tool left off the default called off-surface by agents on at least 5% of
  sessions: move it to the default in place of the least-called default tool (the count
  stays at eight).
- SR-M2 shows agents on the default surface take more turns to a first answer than on the
  25-tool surface.
- A removed tool's replacement cannot answer a question the removed tool answered (a
  filed observation with the question): restore the capability inside the replacement,
  never as a new tool.

---

## ADR-0XY: The support matrix is a projection — declared targets, computed states, evidence links; no status is typed (2026-09-27)

**Status.** Proposed 2026-09-27. Owner decision (binding): track implementation quality
and progress for languages, tools and every other supported kind as a projection, never a
hand-maintained status list. Extends ADR-048 decision 2 from languages to every kind.
Design: `docs/design/support-matrix.md`. Plan: `docs/plans/support-matrix.md`. Schema:
`schema/support/`.

**Context.** Fathom now declares support along eight axes — languages, tool formats,
agent clients, spec dialects (N4), rules files (N8.3), coverage formats (T3), contract-key
kinds (N6), dependency ecosystems (N1). Each has, or is getting, a registry the code loads.
Hand-kept status tables in this repo have drifted within weeks (the scope file's "19-tool"
surface; `package_deps` still listed a month after deletion).

**Decision.**
1. *One projection over registries.* Each kind's registry exposes `(kind, id,
   capabilities)`; targets come from the language queue (languages, tool formats) or
   `docs/plans/support.targets.json` (the other six kinds); state is computed from
   committed readings and `fathom verify` of the owning plan row; every cell links its
   evidence.
2. *No reading, no level.* A registered entry without a reading is `tolerated`; a target
   without a registry entry is `declared`; a level is shown only when a reading computed it.
   The state vocabulary is closed and has no member a person sets.
3. *Closure is tested.* A registry entry without a target fails; a target without a
   registry entry fails once its plan row is claimed; a reading naming an unknown entry
   fails.
4. *History is a ledger.* Every reading write appends to
   `docs/bench/readings/support-ledger.jsonl`; burn-up and `regressed` are computed from it.
5. *Surfaces render the one projection*: `fathom support [--kind …] [--json]`
   (`fathom.support/v1`), a console Support page (heatmap, dialects, progress, what's next),
   the generated `docs/support.md` (LB-F17's tracker becomes its language and tool
   sections — one generator), and the roadmap artifact's data. No MCP tool (law 4).
6. *CUE validates, never authors* (ADR-018). `schema/support/schema.cue` vets targets,
   readings, ledger rows and the output; the entries stay JSON and Go data.
7. *Dialect packs are deferred* (owner, 2026-09-27) until embedded grammars near the
   40 MiB cap; the Support page's dialects view shows the running total, which is the
   trigger. This amends ADR-048 decision 5's "pinned packs for the long tail" from
   "planned" to "deferred with a measured trigger".

**Alternatives rejected.**
- *A hand-kept catalog (markdown or JSON statuses).* It rots, and a typed level with no
  reading is the claimed-unproven row `verify` exists to refuse.
- *CUE-authored entries.* ADR-018 rejected CUE as an authoring surface; the registries
  already live in Go and JSON where their owners work.
- *An MCP tool for support queries.* Law 4; the CLI and the generated doc answer it.
- *Client support from the descriptors' `VerifiedAgainst` string alone.* It is authored;
  a local live check (`fathom harness verify`) writes the reading it must agree with.

**Consequences.**
- New: `internal/support`, `fathom support`, `fathom harness verify`, `schema/support/`,
  `docs/plans/support.targets.json`, `docs/bench/readings/support/`, the readings ledger,
  `docs/support.md`, the `/support` console page.
- Changed: LB-F17 and LB-M6-M1..M3 point at `docs/support.md`; `docs/languages.md` is
  removed; `indexbench lang-oracle` appends to the ledger.
- Claude Code and Codex get client readings from existing live tests; Cursor, Copilot and
  Gemini render `tolerated` until verified.
- Compatibility surface grows by `fathom.support/v1` and the targets file shape.

**Revisit when.**
- A kind needs a state the closed vocabulary cannot express (add it to the schema with a
  derivation rule, never a hand-set status).
- Embedded grammars reach 36 MiB (90% of the cap): the packs deferral's trigger fires.
- The closure test is bypassed or loosened to land a registry entry without a target.
