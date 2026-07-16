# Changelog

All notable changes to sextant. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); dates are ISO-8601.

## [Unreleased]

### Changed — MCP surfaces week-1: execute-and-flag, agent surface, loopback default (docs/plans/surfaces-week1.md, amends ADR-014)
- Gate semantics change from hard-gate to **execute-and-flag**: an off-surface
  `tools/call` or `batch` sub-call now executes instead of erroring, logged
  with `off_surface:true` in the JSONL call log — an error taught abandonment,
  which looked identical to no-demand in 8 days of dogfood evidence. The
  pre-amendment hard gate stays available behind `Server.strictSurfaces` for
  any future multi-tenant shape.
- The `companion` allowlist becomes the 15-tool **agent** surface (minus
  `batch`/`brief`, plus `verify_plan`/`capabilities` — the top non-hook tool
  and the discovery escape hatch), served at the new primary path
  `POST /rpc/agent` with `POST /rpc/companion` kept as a compatible alias.
- `set_profile` is removed entirely (amends ADR-010/014's stdio clause): no
  production caller reaches it any more, so stdio now always serves the full
  registry.
- Four zero-call alias tool registrations deleted: `platform_callers`,
  `test_debt`, `api_routes`, `deps` — use the canonical `cross_repo_callers`,
  `markers`, `routes`, `dependencies`.
- `serve-http`'s default listen address changes from `:7575` (all interfaces)
  to `127.0.0.1:7575` (loopback) — surfaces were never a security boundary,
  so the default must not listen on all interfaces regardless.

### Added — Gap-1 channel instrumentation: denominator, liveness, origin (docs/plans/gap1-instrumentation.md)
- Hooks beat a central log (FATHOM_HOOKS_LOG, default ~/.fathom/hooks.jsonl)
  via agentic-kit v0.7.0 telemetry/heartbeat: edit-hook seen/answered/error,
  session-hook fired — injection coverage finally has a denominator, and beat
  failures are swallowed (hooks never break their silent contract).
- `fathom instrument`: coverage (answered/seen vs the ≥90%% gate), per-day
  rollup, and liveness alarms from the standing expectations (sessions
  without edits in 7d; edits unanswered in 1d — the four-dark-days failure
  made visible). Advisory: exit 0 always.
- X-Fathom-Origin: every hook/CLI HTTP call declares its channel; the daemon
  call log records `origin` so demand numbers and kill clocks separate
  plumbing from agent choice.
- Grammar hardening (GI-7): the plan-annotation parser takes the TRAILING
  brace block — prose braces and code literals in titles no longer hijack
  contracts (caught live by the first non-vacuous gate run over the fleet).

### Fixed — extractor-epoch fingerprint salt: extractor upgrades now reach unchanged files (O9)
- The pass-3 hash-gate keyed fingerprints on file content only, so a binary
  with improved extraction (e.g. H1's aliased-selector canonicalization and
  method-edge forms) silently kept every unchanged file's OLD observations —
  the fleet store served pre-H1 `Calls` forms and `test_reaches` failed on
  genuinely-tested symbols (dogfood O9, caught by the first HTTP-mode run of
  the re-armed agentic-kit gate). Fingerprints are now salted with
  `fusion.ExtractorEpoch` (bump on any extraction-semantics change → full
  re-extract on next ingest); `ObligationsIndexStale` adopts the shared
  `fusion.FingerprintHash` so the formulas cannot diverge. Regression:
  `TestIngest_ExtractorEpochInvalidatesOldFingerprints`.

### Added — SDD reconciler v0: plan grammar v2, wave gate, CUE shadow, Stop hook (ADR-018)
- **Plan grammar v2** (`internal/verify`, additive — v1 plans parse unchanged):
  `[~]` in-progress checkbox; author-owned stable `id:` keys (positional
  `md-NNN` renumbering was fatal for cross-edit diffs); typed dispositions on
  `[-]` (`disposition: refused|deferred` + `reason:` + recorded-only
  `trigger:`); `skip:` justification for `skip_test`; double-quoted annotation
  values may contain commas. `verify.LintPlan` reports convention violations
  as warnings (never exit-code changes in v0).
- **Wave gate**: `fathom verify --require-all` (`verify.GateRequireAll`) —
  fails on open/in-progress tasks so a wave advances only when every task is
  claimed-and-verified or explicitly skipped. The recorded exit-0 run is the
  transition record; there is no transition engine (ADR-018: state is
  derived, never driven).
- **CUE shadow**: `fathom gen-plan-cue` emits a generated-only `#Plan`
  document (schema/plan gains state/disposition/reason/trigger +
  expect.package/skip_reason) and proves **lossless round-trip** against the
  Markdown parse. CUE is output, never an authoring surface (the Dagger
  lesson); hand-authored plan CUE is deprecated.
- **Stop hook**: `fathom stop-hook` — done-means-proven at session
  granularity: tasks newly claimed `[x]` this session (working tree vs HEAD,
  matched by stable ID) that FAIL verification surface at session end.
  Advisory by default; `--block` refuses session end with the failing
  citation. Silent exit 0 on every operational failure (edit-hook doctrine);
  Unknown never fires (fail-open honesty).
- Specs: `docs/design/sdd-reconciler.md` (normative), ADR-018, gated plan
  `docs/plans/sdd-reconciler-v0.md`; grammar reference updated in
  `docs/plans/README.md`. Vision record: `docs/vision/09-deterministic-sdd.md`.

### Changed — obligations v2: first-fleet-compile hardening
- **Prompt v1 → v2 + corpus narrowed to binding docs** (`CLAUDE.md`, `ADR.md`,
  `docs/plans/README.md`). The first fleet compile turned `docs/design/**` essays into
  noisy `**`-scoped aspirational rules ("achieve precision ≥0.90") injected on every
  edit — the trust-fatal class ADR-016 §4.6 warns about. v2 compiles binding rules only
  and prefers the narrowest scope the source supports.
- **Digest gate now includes `PromptVersion`.** A prompt bump previously did not
  recompile unchanged sections — the v1→v2 recompile silently kept v1 claims on 27/32
  sections. Covered by `TestCompile_PromptBumpDefeatsDigestGate`.
- **Per-section failure resilience.** A malformed or truncated model response
  (strict-JSON parse failure, `StopMaxTokens`) now fails only its own section —
  counted in the new `compile_runs.sections_failed` — instead of aborting an
  88-section fleet corpus; transport/auth errors still abort. `MaxTokens` 1200→2400
  (rule-dense sections were truncating mid-JSON).
- **Corpus-departure retirement.** Claims whose subject left the corpus selector are
  superseded on the next compile, not served forever.
- First full fleet compile under v2: 31/32 sections, 72 claims (9 rejected by
  compile-time verification), $0.047.

### Added — semantic layers v1: the obligations tracer bullet (ADR-016)
- **Governed-claims substrate.** Four additive tables beside observations —
  `semantic_claims` (content-addressed `claim_id`; tier + `tier_profile`; status
  fresh/stale/contradicted/superseded/refuted; flattened provenance: model, prompt
  version, repro key, input digest, compile watermark), `claim_links` (`cites` with
  line-range hash, `scopes` globs), `compile_runs` (real token/USD ledger), and
  append-only `claim_events` (the refute channel). Superseded claims are status-flipped,
  never deleted. Both store implementations + contract tests.
- **Obligations compiler** (`internal/semantic`): governing docs → markdown sections
  (fence-aware, stable subjects) → digest gate (unchanged sections cost $0) → DLP
  redaction → injected LLM (strict JSON; mock/replay in tests) → **compile-time anchor
  and scope verification** (a claim whose cited lines don't resolve or whose scope globs
  match no indexed file is rejected, counted per run) → transactional store + supersede.
  Rules are word-budgeted at compile (≤28 words) so a 6-rule payload stays ≤150 tokens.
  Dry-run estimation is the default spending gate. Layer definition schema:
  `schema/semantic/schema.cue` (+ Go-asserted negatives).
- **Citation sweep** (post-ingest, token-free): re-hashes every served claim's cited
  lines; drift flips status to `stale`, unresolvable citations to `contradicted`,
  restored docs heal back to `fresh`. Wired into `IngestService` so the daemon's
  auto-refresh keeps claims honest.
- **Decision-moment serving.** `obligations` MCP tool (on the companion surface, now 15
  tools): rules governing a file, ordered by scope specificity, capped at 6, each with
  `src` citation + tier, drifted rules served *flagged*; **zero-output-on-clean**.
  `compile_layer` tool (full surface only; `dry_run` defaults true) + `fathom compile`
  CLI. `staleness_report` gains a per-layer claims summary.
- **PostToolUse injection** (`fathom edit-hook`): after every Edit/Write, Claude Code
  gets the edited file's obligations injected (≤150 tokens) plus an index-behind-your-
  working-tree warning; silent on clean, silent on any error (hooks never block —
  the coding-agent-excellence §5.3 failure policy). This is the configuration-forced
  delivery channel the 0–7% pulled-tool share demanded.
- **Precision eval harness** (`internal/semantic/eval_test.go` + labeled fixture corpus
  with deliberate scope traps): gates the layer at precision ≥0.90 / recall ≥0.75
  against a real model; opt-in (`FATHOM_EVAL_LLM=1`), deterministic suite stays free.

### Added — fleet onboarding ergonomics
- **`ingest` tool onboards new services**: optional `path` (absolute corpus root)
  ingests a repo the index has never seen — through the live daemon, no
  single-writer lock dance. The session-hook's "add this repo" guidance now names
  the exact call. Capability forwarded through the auto-refresh wrapper (the O7
  lesson) with a through-the-wrapper test.
- **Subdirectory-rooted projects resolve their service**: both hooks now try the
  project dir's basename, then the enclosing git repo's basename (a Claude
  project rooted at `<repo>/deploy` no longer resolves to service "deploy");
  the edit-hook resolves file paths against whichever root matched.
  `reingest-fleet.sh` gains nested-path support + three fleet repos
  (growing-gardens-bi, car-assistant, crossplane-platform); the adoption
  script's default names follow.

### Added — done-means-proven gate (H1.2)
- **Checkbox semantics**: `[x]` in a plan is a *claim*; `verify.Gate` fails any claimed
  task whose contract fails (`--strict` also gates Unknown). `[ ]` open and `[-]`
  skipped never gate.
- **`fathom verify` CLI**: HTTP-first against the daemon, `--store` for CI; the gate is
  always computed locally so a stale daemon can't launder a claim. Exit 0/1/2 with
  fail-open pre-push (exit 2 warns and allows) and fail-closed strict CI. Wired into
  lefthook pre-push + `scripts/ci.sh` here and in agentic-kit (`docs/plans/` in both).
  First gated run caught 3 over-claims by the plan's own author (demo:
  `docs/plans/README.md`; recall gap filed as dogfood O6).
- **Gate honesty: an empty index is "cannot verify", never "all claims false".**
  Caught live against the pre-migration daemon: verifying a service with zero
  indexed symbols made every claim read `symbol not found` — evidence-shaped
  output for an operational problem, which would have blocked every push (the
  §5.3 trust-fatal mode). `fathom verify` now probes the index first (HTTP:
  `list_services`; store: symbol count) and exits 2 (fail-open) with a
  fix-it message when the service isn't indexed.

### Added — born-portable stores (ADR-017)
- **Repo-relative anchors** from every connector (LocalConnector included — absolute
  refs from legacy stores still read); **`fathom remap`** repoints a service root in one
  metadata update; **Watermark SPI** (git HEAD or content-addressed `manifest:` hash)
  makes freshness domain-neutral — non-git corpora (docs folders, findings exports)
  auto-refresh and sweep honestly; **ingest self-heal** purges pre-ADR-017
  absolute-anchored content wholesale so no symbol duplicates across anchor styles.
  Contract: `TestStorePortability_RoundTrip` (move corpus + remap → citations, queries,
  freshness, and hash-gating all survive).

### Fixed
- `fathom-adoption.sh` measured nothing for this repo after the rename (transcript dirs
  are path-keyed): name-era aliasing, per-ISO-week rollup, and a daemon call-log summary
  section. Measured baseline: **0% fathom share (0 vs 193 grep calls)** — the number the
  forced channels exist to move.


### Renamed
- **sextant → fathom** (ADR-015). Module path `github.com/geoffmilleraz/fathom`,
  binary `cmd/fathom` → `bin/fathom`, MCP serverInfo/resource URIs/metrics prefixes,
  docs and scripts. Back-compat kept where the old name is data in the field:
  `.sextantignore` still honoured (legacy fallback behind `.fathomignore`),
  adoption metrics count both `mcp__sextant__`/`mcp__fathom__` prefixes,
  `session-hook` treats the two service/dir names as one identity, and
  `reingest-fleet.sh` finds the repo under either directory name. Historical
  records (this changelog below, USAGE_JOURNAL.md, dogfood-observations.md)
  intentionally keep "sextant". Operator steps: `docs/rename-migration.md`.

Uplifts from mining local Claude Code usage across the fleet. The finding: sextant works
when called, but is dramatically under-used — agents reach for shell grep 10×+ more than
sextant even where the index exists, because "index first" lived only in passive docs.

### Added
- **Path-scoped tool surfaces on the shared HTTP daemon.** `POST /rpc/{surface}` serves an
  immutable per-request view of the registry: `companion` is a curated 14-tool coding-agent
  hot loop (`find_symbol`, `query_symbols`, `references`, `outline`, `repo_map`,
  `find_callers`, `symbol_context`, `pack_context`, `brief`, `entrypoints`, `batch`,
  `ingest`, `staleness_report`, `list_services`); `coding`/`investigation`/`shared`/
  `platform` mirror the group profiles. Bare `/rpc` remains the full surface (the right
  registration for schema-deferring clients like Claude Code); scoped endpoints cut the
  tools/list schema payload ~80% for clients that load every schema up front. One daemon,
  one `mcp.Server`, one store — a surface is context carried per request
  (`mcp.WithRequestScope`), not a second server. Motivated by the tool-surface strategy
  review (86 tools ≈ 8.6k schema tokens for non-deferring clients).
- **Surfaces hard-gate `tools/call`, closing the profile-bypass hole.** Previously a tool
  hidden from `tools/list` by the active profile was still callable (`findTool` scanned the
  full registry) — and `batch` dispatched sub-call handlers directly, bypassing any filter.
  On a named HTTP surface, off-surface calls (including `batch` sub-calls) now return an
  error naming the surface and pointing at `POST /rpc`. Canned prompts are scoped the same
  way: a surface hides prompts whose workflow references tools it gates (resources stay
  available everywhere — data, not capability). The stdio profile stays advisory (a single
  client owns that process), pinned by regression tests on the non-gating side.
- **`set_profile` is rejected over HTTP.** The profile is process-global on the shared
  daemon: any client flipping it changed `tools/list` for *every* session, and
  `listChanged:false` meant nobody re-fetched. The error points at path-scoped surfaces;
  stdio keeps `set_profile` and gains `companion` as a valid value. Since every HTTP call
  to it is dead, `set_profile` is also dropped from every HTTP `tools/list` view — scoped
  endpoints advertise only what they serve. `capabilities` and the manifest resource now
  report `active_surface` alongside `active_profile`.
- **Tool-call telemetry wired into the daemon.** `serve-http --call-log` (default
  `workspace/logs/mcp-calls.jsonl`, empty disables) appends the existing JSONL CallLogger
  records — now with the request `surface`, with `batch` sub-calls logged individually
  (`via:"batch"`) so bundled hot-loop lookups are counted per tool, and with each record
  written as a single atomic `O_APPEND` write so a daemon kill can't merge lines — making
  which-tools-does-the-hot-loop-actually-use measurable instead of argued (feeds the
  sextant-adoption metric and any future surface curation). Decision record: ADR-014.
- **`session-hook` — a Claude Code SessionStart adoption hook.** `sextant session-hook`
  pings the shared serve-http daemon, maps the project dir to a service, and injects an
  active reminder ("this repo is service X (N symbols); resolve symbols/callers/structure
  via sextant BEFORE Grep/Read") — or an honest signal when the daemon is down or the repo
  isn't indexed. Short HTTP timeout, always exits 0, hides malformed services from the list.

### Changed
- **Query-side staleness is constant-size now** (USAGE_JOURNAL.md ask #4: an 8-node
  `query_symbols` answer arrived buried under ~117 lines of per-source staleness
  metadata). The `staleness` block on query responses now carries at most 3 example rows
  (the oldest stale sources) plus `stale_sources`, `total_sources`, `fresh_sources`, and
  a new `oldest_ingested_at` low-water mark; `reingest_recommended` unchanged. The full
  source list and commit drift stay available via the `staleness_report` tool.
  Regression: `TestQuery_StalenessReport_CompactWhenManyStale`.
- **`find_symbol` no longer punishes not knowing the exact identifier**
  (USAGE_JOURNAL.md ask #3: `find_symbol("serveHTTP")` → `[]` because that wasn't the
  exact spelling). Matching now degrades deterministically — exact/key-suffix →
  case-insensitive (same equality rules, case-folded) → substring (capped at 20,
  tightest names first, explicit `truncated`) — and a full miss returns up to 5
  `did_you_mean` suggestions when any exist: edit-distance typo candidates from the
  symbol list first ("authentiction" → `Authentication`), else concern-ranked. The
  response reports `matched_via` (`exact`/`case_insensitive`/`substring`/`none`);
  results are existence proof only for the first two tiers. Fallback tiers run only on
  an exact miss, in the MCP layer, so no backend interface changed. Regressions:
  `internal/mcp/findsymbol_test.go`.
- **`verify_plan`'s tool schema slimmed ~27% (~150 tokens; description nearly halved)**
  (it was the heaviest schema on the surface at ~554 tokens, ~6% of the whole registry
  for schema-loading clients). `min_tier`'s valid values moved from prose into a proper
  JSON-schema `enum`. Behavior is unchanged; the full verdict reference lives in
  `docs/mcp-tools.md`.

### Fixed
- **Ingest no longer aborts on a symlink-to-directory (Nix/devbox trees).** `WalkDir`
  does not follow symlinks, so a link like `.devbox/nix/profile/default-9-link`
  surfaced as a non-dir entry; `Fetch`'s `os.ReadFile` then failed with "is a
  directory", and because the fusion fetch pass treats the first fetch error as
  fatal, **one stray link aborted the entire ingest**. `LocalConnector.Discover` now
  emits only regular files (symlinks/devices/sockets/pipes skipped). Found
  dogfooding `resume_as_code`, which went from a hard abort to 1,160 clean symbols
  (the Nix store behind the link is correctly excluded, not indexed as noise).
  Regression: `TestLocalConnector_Discover_SkipsSymlinkToDir`.
- **`IngestService` re-ingests from the recorded path, not the service ID.** It used
  `ServiceRef{ID: service, Path: service}` — re-walking the bare ID as a directory, which
  ingested nothing and **clobbered `ServiceMeta.path` to empty**. Since auto-refresh,
  `staleness_report`, and the `ingest` MCP tool all key off that path, every refresh
  silently froze the shared daemon's index. Now it uses the stored `meta.Path` (mirroring
  `Sources`), so refreshes preserve the path and actually pick up new commits. Regression:
  `TestIngestService_UsesAndPreservesStoredPath`. Also serves `serve-http --auto-refresh`
  (added so the always-on daemon self-refreshes) and `scripts/reingest-fleet.sh` (rebuild
  the fleet index cleanly with path metadata + no malformed/bloated services).
- **Reject path-like service IDs (fleet hygiene).** `ingest --service ../app-kit` had
  polluted the fleet with duplicate, confusingly-named services. `core.ValidServiceID`
  rejects path separators / `..` / whitespace at the `client.Ingest` chokepoint (CLI + the
  ingest MCP tool), suggesting a clean name. Prevents recurrence.
- **Bound list-tool output to a char budget.** A real `query_symbols` result hit ~57KB and
  blew the caller's token budget. `query_symbols` and `references` now keep the longest
  prefix of records that fits ~16KB and flag `truncated`, so one result can't blow context.

## [0.2.0] — 2026-06-16

Enhancements driven by mining throughline's real Claude Code sessions (841 sextant calls,
1666 shell greps, a heavy manual freshness loop).

### Added
- **`references` MCP tool** — evidence-tiered, fleet-wide occurrence search (substring /
  whole-word / regex). Returns text matches at the `pre-scan` tier plus symbol definitions
  cross-referenced from the index at their real tier. Turns the ~658 repo-by-repo greps a
  session does into one structured call.
- **`verify_plan` accepts a Markdown checklist** (`plan_markdown`), and `gen-status-cue`
  accepts a `.md` plan file: each `- [ ]/[x] title [SPEC-ID] {symbol: Foo}` line becomes a
  verifiable task. A `/verify` slash command runs the loop — surfacing `[x]`-but-not-verified
  items before "done" is trusted. (throughline's sessions were hand-rolling this.)

### Changed
- **`serve-mcp --auto-refresh` (default on)** — a query for a service whose git HEAD has
  drifted from the indexed commit transparently re-ingests (hash-gated, debounced per
  commit) before answering. Removes the manual staleness→ingest loop (49 ingests / 19
  staleness checks per 3 sessions). A second `serve-mcp` on the same store now refuses with
  actionable guidance instead of DuckDB's raw single-writer lock error.

## [0.1.0] — 2026-06-15

First tagged milestone: a multi-language code-intelligence + evidence engine with an MCP
server, a language-agnostic cross-repo graph, and an evidence-backed verification layer.

### Added
- **Multi-language extraction (~25 languages/tools).** Tree-sitter via CGO (ADR-013) for
  general languages — Python, JS/TS, Java, Ruby, Rust, C#, C, C++, Scala, PHP, Lua — and
  pure-Go parsers for config/IaC/tooling: Terraform/HCL, Kubernetes, Helm, Dockerfile,
  docker-compose, GitHub Actions, Protocol Buffers, GraphQL, CUE, Markdown, SQL, TOML,
  JSON. 93 `SourceKind`s. Interim intra-file call edges for the tree-sitter languages.
  Calibration corpus + harness.
- **Language-agnostic cross-repo blast radius.** `internal/platformlink` builds the
  cross-repo graph from stored multi-language observations instead of a Go-only filesystem
  walk; `pkg/platform.BuildFromStore`; the `platform-callers` CLI defaults to the store
  path (`--repos` is a labelled legacy Go-only mode). Cross-repo edges now resolve for
  every language whose extractor populates call edges.
- **Evidence-backed definition-of-done.** `internal/verify` engine + the `verify_plan` MCP
  tool: a per-task verdict computed from code evidence (the claimed symbol exists at a
  sufficient tier ∧ a test statically reaches it ∧ a required call-edge is wired ∧ no
  contradiction ∧ declared specs are covered), each with cited evidence. The
  `gen-status-cue` CLI renders a deterministic, one-directional `STATUS.cue` from a
  human-owned plan file (`schema/plan` CUE schema). **Static by construction**:
  `runtime_pass` is always `unknown` — a static check never claims a test passes.
- **Use-case adoption docs** under `docs/use-cases/` — a getting-started guide plus six
  how-tos (coding-context, cross-repo blast radius, spec verification, incident
  investigation, change review, fleet health) for wiring sextant into Claude Code, with a
  one-doc-per-use-case maintenance convention.
- **Build-identity / staleness signal.** `sextant version` reports the embedded VCS
  revision, commit time, and dirty flag (Go `-buildvcs`, no ldflags); `serve-mcp` logs it
  at startup, so a long-running server that has outlived commits is self-evident — compare
  against `git rev-parse HEAD`. (O3.)
- Earlier foundation in this release: typed edge-graph `traverse` + commit `history`
  ingestion; `spec_refs` and `brief` tools; git-HEAD auto-staleness; line numbers carried
  through to MCP results; `.sextantignore` repo-specific excludes; the Claude Code
  integration playbook and assessment.

### Fixed
- **`SymbolNode.Package` persists through the store.** It was silently dropped in the
  `SymbolNode → Observation` mapping, degrading `SymbolKey` reconstruction to `"svc::.Name"`
  and breaking precise cross-repo / call-graph matching off stored evidence. Added a
  `package` column with an idempotent migration.
- **The call graph resolves cross-package selector calls.** A callee recorded as
  `"pkg.Name"` (e.g. an external `*_test` package calling the symbol under test) never
  matched the node key `"svc::pkg.Name"`, so `test_impact` / `find_callers` /
  `rename_impact` under-counted in the common Go external-test-package pattern. Fixed with
  a precise `package.Name` index (a receiver-method call is never mistaken for a package
  selector).
- **`outline` honors its repo-relative contract.** `symbolnav.Outline` matched the stored
  absolute `FilePath` against the `file` arg by exact string, so a repo-relative path —
  the documented contract — returned `{"symbols":[]}`. Now resolves a relative query by
  path-segment suffix (absolute still exact). Found via fleet dogfooding (throughline).
  (O1.)

### Changed
- **The `post-commit` hook rebuilds `bin/sextant` on Go changes** before re-indexing, so
  the on-disk binary tracks HEAD. A running `serve-mcp` still needs a restart to load a new
  binary. (O3.)
