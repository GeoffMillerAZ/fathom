# Changelog

All notable changes to fathom. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); dates are ISO-8601.

## [Unreleased]

### 2026-08-07 — compile --dry-run: cost prints as an honest range, never a confident zero (O8, plan IO-8)

The dry run once reported input-only tokens and `cost_usd: 0.000000` for
a run whose real cost was $0.64 and 21 minutes (output was 87% of the
tokens). A dry run now carries an estimate band on the run
(EstTokensOutLow/High = input × [2, 8] — the observed obligations layer
ran ~6.7×; the ratios are revisable data) and the human rendering prints
tokens-in flagged as the bytes/4 heuristic it is, output as
unknown-with-a-band, cost as a range labeled "never a point — O8", and a
duration band at the observed ~100 output tok/s. Real runs print exactly
as before. Pinned by TestDryRunEstimateRange (positive ascending cost
band, no LLM configured — a dry run must never need one).

### 2026-08-07 — one vocabulary for "none configured": explicit-empty bindings/guards are valid (O7 completed, plan IO-7)

O22's fix accepted the ABSENT field but kept `bindings: []` a loud error
("closer to a mistake than an intentional omission") — and the fleet
promptly proved the opposite: the activate skill, the kit packs, and the
review fix-pass all standardize the explicit literal, so doctor FAILed an
adopter file fathom's own tooling prescribes. Two sanctioned vocabularies
is the drift bug class. Now one: `bindings: []` and `guards: []` load
clean and pass ValidateStrict (schema `[_, ...]` constraints dropped in
schema/binding + schema/guard, embedded mirrors re-synced); only a
present-and-malformed entry fails. Doctor's existing scaffold-awaiting-
opt-in PASS wording covers both forms. Live proof: platform-engineering's
standing `binding_file FAIL` (its modules.cue carries the prescribed
`bindings: []`) is now PASS. Pinned by TestExplicitEmptyBindings /
TestExplicitEmptyGuards; the schema/loader negatives that asserted the
rejection are inverted.

### 2026-08-07 — review: --budget-seconds bounds the advisory channel (O9 root-caused, plan IO-9)

O9's root cause corrected: nothing prompts and nothing hangs — a six-file
pre-push review measured 22.6s, the verify loop adds more, and lefthook
buffers each step's output until completion, so scripted pushes read the
silence as a stall and killed it at their caps. `fathom review` gains
`--budget-seconds N`: the whole body races the deadline and on excess
prints a named advisory skip and exits 0. The flag REQUIRES --no-persist
— a run that can be cut off mid-flight must never be a store writer —
and the pre-push step now passes --budget-seconds 20 (bounding the
advisory channel, not weakening a gate: that step always exited 0).
Pinned by TestReviewNonInteractive (budget bounds elapsed time; the
no-persist guard refuses; fast runs unaffected). Follow-up filed in O9's
fix note: why a six-file review costs 22.6s at all.

### 2026-08-07 — doctor: lefthook gates recognized through variable indirection (plan IO-5)

`checkLefthookGates` no longer requires the literal substring "fathom
verify": `lefthookInvokes` matches any per-line spelling of the binary —
literal, path-suffixed, or a shell variable (`"$FATHOM" verify`,
`${FATHOM} verify`) — closing the audit's live false negative where a
correctly-gated repo (platform-engineering's pre-push loop) was told to
add the gate it already had. Honest note: the audit's other two doctor
findings (binary_fresh comparing the adopter's HEAD, hooks_firing reading
a repo-relative call log) were ALREADY fixed at HEAD by the O23 wave —
the audit ran the stale repo binary, which is the deploy-skew lesson the
reconcile design exists to close.

### 2026-08-07 — plan grammar: braces in quoted values no longer void the row; unparseable blocks are loud (O4, plan IO-6)

`parseAnnotation`'s brace pairing is now quote-aware: a quoted value
containing the notation's own vocabulary — the exact string
`{symbol, package}` an author writes when *explaining the grammar* — no
longer terminates the block, so the row keeps its author-owned id and its
whole contract (`TestParseAnnotation_BracesInQuotedValues` pins both the
id and every key). GI-7's trailing-block rule is preserved: pairing runs
over outside-quote braces only and the block whose close is last wins
(the existing prose-example and Beat-literal tests stay green). Second
half of O4: a brace block that parses to zero keys but contains contract
keywords surfaces as `Task.AnnotationSuspect`, and `LintPlan` says so by
name — "the row's contract and author-owned id are VOID" — instead of the
silent md-NNN downgrade (`TestLintPlan_UnparseableAnnotationIsLoud`).

### 2026-08-07 — session-hook: enumeration failure is not "not indexed" (O2, plan IO-4)

`fetchServices` now returns an error alongside the map, with one retry on
its own 4s timeout, and every failure mode (transport, non-2xx,
unmarshal) is an error instead of an indistinguishable empty map. The
session hook grew the third branch O2 asked for: a healthy daemon whose
list_services fails renders a degraded notice naming the failure — never
the confident false "this repo is not an indexed service" instruction
that taught agents to abandon index-first discovery for indexed repos.
A genuinely empty index now says "none yet" instead of the dangling
"Indexed services: ." join artifact, and the hook's inline candidate
dedup now uses mergeCandidates (the O1-c fix), closing the last copy of
the collapsing dedup. Pinned by TestSessionHookDegraded and
TestSessionHookZeroServices.

### 2026-08-07 — observe: the initial read now waits out a concurrent creator's write window

`fathom observe`'s foreign-file refusal fired on a file that was simply
mid-creation: the initial `readObservations` can land in a concurrent
winner's O_EXCL-create→header-write window and see a markerless file —
indistinguishable, at that instant, from a foreign file — so the loser
refused and the entry was lost (exit 1). The refusal path now waits the
same 200ms `reReadAfterLostCreateRace` window the lost-O_EXCL path already
used before concluding the file is genuinely foreign.
`TestObserve_ConcurrentFirstCreationLosesNothing` was failing
deterministically in the full suite; now green at `-count=10 -race`.
Found by the impact-observatory activation gate run (plan §A), not by a
user losing an observation — which is the order these should be found in.

### 2026-08-07 — deterministic service resolution; dedup never collapses (O1 a–c, plan IO-1)

`resolveFromServices` no longer resolves duplicate-corpus-root conflicts by
Go map iteration order (the coin flip that zeroed obligations on 18/50
measured edits): all matches are collected and the winner is chosen by
richness (symbol count), then basename affinity, then lexicographic order —
pinned by `TestResolveFromServicesDeterministic` (200-iteration determinism
asserts per case). `candidatesWithDaemon`'s dedup is extracted to
`mergeCandidates`, which drops only exact name+root duplicates — a
name-equal candidate at a different root now survives, so a wrong daemon
answer degrades instead of totalling the fallback list
(`TestMergeCandidatesNeverCollapses`). O1 fix (d) — daemon-side duplicate
refusal — lands with plan row IO-2.

### Measured — T5-8 second swing: fathom WINS its home turf; the lookup ×10 retraction stands (docs/plans/trust-and-tokenomics.md, 2026-07-24)
- **Home bench (pre-registered H2-1 criterion, unchanged): PASS — both
  halves met.** Success 13/13 vs grep 11/13 (strictly greater); tokens
  1,594,762 vs 2,795,948 = 0.570× (≤1× required); turns 47 vs 72. First
  swing (same corpus, 14-tool surface): 12/13 vs 12/13 at 1.071× — both
  halves failed. The fix wave drove it: read_source (T5-4) collapsed
  H1-T13 from 601,132 tokens/16 turns to 106,954/4; DocChunk `end_line`
  (T5-5) flipped fathom's only home-turf failure (H1-T08) to a pass;
  H1-T12 fell 299,830/8 → 108,046/4. Anomalies audited before scoring:
  grep's two failures are legitimate misses (H1-T13's is the identical
  miss as the first swing; H1-T08's is n=1 variance the verdict does not
  rest on — granting it back still leaves 13 > 12), and the win survives
  scoring against grep's better first-swing numbers (0.735×, 13 > 12).
  Artifacts: `rerun-home-results-20260724T170152Z.json` + summary.
- **Lookup bench (B1's unchanged <5× bar): the retraction STANDS,
  recorded either way.** 15/15 vs 14/15 success (fathom strictly better
  this run), tokens 0.511× = a 1.96× reduction (was 1.38×) — still under
  the pre-registered 5× bar, so the ×10 claim stays retracted. B1-T09
  (O12's 6.6× loss) collapsed 504,785/20 turns → 107,268/3 (0.21×).
  Artifacts: `rerun-results-20260724T164907Z.json` + summary.
- **Pre-run corpus revision committed FIRST (8a847d3, the registration
  proof):** home corpus re-verified at HEAD bfb8010 (zero ground-truth
  changes needed; one new sweep-missed +6-line citation drift on
  backend.go's obligations recorded as a capability finding); lookup
  corpus re-swept for three days of drift (B1-T01/T04/T14/T15 gained the
  real new callers cmd/fathom/plans.go, plans_archive.go, doctor.go —
  all-of tightenings); the bench runner's fathom-arm allowlist gained
  `read_source` to match the live 16-tool `/rpc/agent` surface (ingest
  still denied). Criteria untouched on both benches.
- **`list_services` now returns each service's recorded corpus `path`**
  (additive; from ServiceMeta, absolute post-O10), and the session/edit/stop
  hooks resolve their service identity by matching the project directory
  (symlink-resolved) against those paths — the daemon's own repo↔service
  mapping is authoritative, with the old basename guess as fallback and
  fail-open on any error. Root cause of O15: the sextant→fathom rename left
  this repo's dir basename naming a nonexistent service, and the back-compat
  purge removed the alias shim — obligations injection was silently dead on
  the dogfood repo for six days while probe traffic looked healthy. The
  edit-hook fetches the resolution only after a candidate root contains the
  edited file, preserving the out-of-repo total-silence invariant.

### Fixed — sweep re-anchors citation drift instead of only detecting it; obligations eviction is now visible (T5-1/T5-2, docs/plans/trust-and-tokenomics.md, 2026-07-24)
- **O13 (citation drift, the home bench's top product finding) — fixed in code.** `core.ClaimLink.AnchorText` (the literal cited text, `core.LinesText`) is now persisted alongside the hash by both live "cites"-link writers (`internal/semantic/compile.go`'s `verifyDraft`, the review findings bridge's `AttachClaims`). `semantic.Sweep` re-anchors a drifted citation when its `AnchorText` resolves to exactly one new location in the cited file (`store.Store.UpdateClaimLink`, new — SQLite + memory, with a migration for pre-existing `claim_links` rows), appending a `ClaimEvent{Verdict:"re-anchored"}` audit entry (mirrors ADR-017's remap); zero or multiple matches still fall through to stale, unchanged. A claim compiled before this change has no stored `AnchorText` and can only ever go stale on drift, never heal — an honest degradation. **Fleet re-sweep (T5-1b), run against a read-only copy of the live store:** 340 claims checked, 73 wrong citations found (all fathom-service), 0 healed / 73 staled — every existing claim predates this fix (0/548 fathom `cites` links carry `AnchorText` yet), so healing activates on the NEXT fleet recompile. The exact O13 reproduction claim is now correctly flagged stale in the copy (previously served with no status flag at all).
- **O14 (eviction visibility) — fixed in code.** `obligations` reports `displaced: N` when the `MaxObligations` cap drops served-eligible claims, and every served rule carries a `layer` label naming which manifest layer compiled it — surfaced on the MCP tool response and the edit-hook payload (one integer + short per-rule labels; the payload budget is unaffected). The scope-mapping half of O14 (a mis-scoped rule crowding out unrelated files) is a compile-prompt fix, filed — not compiled — as `schema/semantic/layers/obligations/prompt.PENDING-scope-precision.md`: `internal/semantic/prompts.go` content-addresses `PromptVersion` off `prompt.md`'s bytes, so editing the live prompt (even a marked-pending block) would silently trigger a flag-day as a side effect of this commit. Lands at the next deliberate flag-day alongside OP-6.
- **Operator-visible UX event on activation (expected honesty, not a regression):** once the rebuilt daemon binary is deployed and a live sweep runs, ~73 fathom obligations that today serve unflagged begin serving `[stale]`-labeled fleet-wide in the edit-hook payload — the sweep finally telling the truth about citations that were already wrong, not new breakage. They clear via re-anchor/recompile as `AnchorText` populates.
- Tests: `TestSweep_ReanchorsOnUpstreamLineShift` (internal/semantic, 6 subtests incl. heal-then-resweep idempotency and multi-line double-heal across successive shifts), `TestSweepClaims_ReanchorsOnUpstreamLineShift` (pkg/client), `claims_link_update_reanchors` + `TestSQLiteStore_LegacySchemaGainsAnchorTextColumn` (internal/store), `TestObligationsFor_DisplacedCountAndLayerLabels` (internal/semantic), `TestEditHookRendersDisplacedAndNonDefaultLayer` (cmd/fathom).

### Added — read_source + DocChunk line offsets + symbol_context snippet (T5-4/T5-5/T5-6, docs/plans/trust-and-tokenomics.md, 2026-07-24)
- **`read_source`**: a new bounded, line-numbered, read-only raw-text tool
  (`{service, file, start_line?, end_line?}`, `internal/mcp/readsource.go`),
  added to the agent/companion surface allowlist (now 16 tools — the
  program's second net-new tool since the founding 15, docs/mcp-tools.md).
  Capped at 200 lines / 16000 bytes per call; refuses an absolute path,
  path traversal, a file outside the service corpus, a binary file, and an
  explicit range wider than the cap; an out-of-range EOF boundary is
  clamped and noted, never refused. Directly answers the H3-1 home-bench
  finding: the agent surface had no raw-text read, so recovering current
  text at a cited line cost 16 turns / 601k tokens (T13) where a plain
  `sed -n` would do it in one.
- **DocChunk line offsets**: `core.SymbolNode`/`core.Observation` gained an
  additive `EndLine` field, threaded through `internal/fusion`'s
  SymbolNode→Observation mapping, `internal/store`'s SQLite schema
  (new `end_line` column + `ALTER TABLE` migration for pre-existing
  stores, including the live fleet store), and every serving path via
  `core.Observation.ToSymbolNode`. `internal/extractor/mdextract.go`'s
  `MarkdownExtractor.Extract` now computes each heading's real section
  extent (the line before the next heading, or EOF for the last one) —
  closing the H3-1-identified T08 class, where a DocChunk's single heading
  line forced downstream tools to GUESS a cited line by arithmetic (a
  reported "line 20" whose real text was at line 29). Paired with
  `read_source`, a doc-cited answer now pins the section's real lines.
- **`symbol_context` inline snippet**: an additive `snippet` field carries
  the definition's source, inline, line-numbered, capped at 40 lines, read
  via the same safe path as `read_source` — a "what is X, show it to me"
  question now completes in one turn instead of two. Honest-absence:
  omitted (never faked) when the source isn't retrievable.
  `snippet_truncated:true` marks a snippet that stopped at the cap with
  more file below it (critic finding D — a 100-line function must never
  silently look 40 lines long), and `snippet:false` opts out entirely.
- **read_source hardening (critic round, three proven holes closed)**:
  (A) symlink containment — the lexical-only path check let file and
  directory symlinks inside a corpus read files OUTSIDE it on the shared
  daemon; both the corpus root and the candidate are now
  `filepath.EvalSymlinks`-resolved and the resolved path must sit inside
  the resolved root (same-corpus symlink targets still read fine).
  (B) a 2MB `os.Stat` hard ceiling refuses oversized files BEFORE any read
  (pinned equal to `fusion.MaxExtractSize` — such files aren't in the
  index either), and a single line longer than the 16000-byte cap comes
  back truncated to it with a note, never whole (the minified-JS case).
  (C) "indexed corpus" is enforced, not aspirational — `.fathomignore`'d
  paths (the connector's own compiled matcher, file + ancestor dirs), the
  always-skipped directories, and `.env`-pattern files all refuse with a
  reason (indexed dot-paths like `.github/` stay readable). An empty file
  now reports `total_lines: 0`. Every "15-tool" surface reference across
  the living docs updated to 16 with `read_source` named; ADR-014's
  amendment and docs/vision/05-surfaces.md carry dated amendment notes.

### Measured — home-turf bench run: fathom does NOT win its own ground (H3-1, docs/plans/home-bench.md, 2026-07-24)
- **Product findings from the run filed as dogfood O13 (sweep-missed citation drift — claims served with silently wrong CLAUDE.md citations, confirmed live) and O14 (the top-6 serving cap silently evicts previously-served claims after a recompile)** — the bench's most valuable outputs beyond the verdict itself.
- **The Wave-3b home-turf context-bench ran for real** (13 index-native
  tasks × 2 arms, all 26 `claude -p` runs live against the recompiled
  daemon; artifacts `home-results-20260724T033131Z.json` +
  `home-summary-20260724T033131Z.md` under
  `workspace/artifacts/context-bench/`). Against the pre-registered H2-1
  criterion, **both halves failed**: success 12/13 vs 12/13 (equal, not
  strictly greater) and tokens 1.071× grep's (a 7% premium where ≤1× was
  required). Per the plan's own consequence line, the pillar's pulled-tool
  story is refuted on its own ground — recorded verbatim in the H3-1 row.
  Failures offset: fathom alone failed H1-T08 (DocChunk granularity cannot
  pin a sentence-start line number), grep alone failed H1-T13 (quoted an
  ADR identifier from outside the required line range). Fathom was cheaper
  on 8/13 tasks; of the 5 expensive ones (incl. T06 1.91×, a plain lookup grep won on cost), 4 sit exactly where the 14-tool read-only
  surface lacks a direct evidence path (exact line numbers, current doc
  text at a cited range) — erased the aggregate.
- **Mandatory pre-run re-derivation executed and committed first**
  (corpus revision 1d685a3, per `post_recompile_rederivation`): the
  recompile's three-layer blend (obligations+adr-rules+spec-bindings)
  silently drifted T01/T02's citations (sweep gap — content moved, no
  `status` flag), crowded T12's target claim out of the served top-6
  (the predicted fresh-re-cite inversion did NOT occur), and SS-8's
  DuckDB deletion broke T06/T07/T13's targets outright; T11's
  'single-writer' scoring token collided with new CLAUDE.md prose.
  Ground truths hand-re-verified at the run commit; the H2-1 criterion
  untouched.

### Removed — DuckDB backend + CGO deleted outright (ADR-025, SS-8, docs/plans/store-sqlite.md, 2026-07-23)
- **SS-8's deferred trigger fired**: the operator ran `migrate-store` on the
  fleet DB and the daemon ran a full day green on SQLite
  (`workspace/logs/store-migration-stamp.txt`: `2026-07-23T01:56:17Z`). With
  that, `internal/store/duckdb.go` (`DuckStore`, `github.com/marcboeker/
  go-duckdb/v2`) and its test file are deleted outright — no DuckDB code
  remains anywhere in this repo.
- **`migrate-store` removed** (`internal/store/migrate.go` +
  `cmd/fathom/migratestore.go`), not kept: it existed solely to read DuckDB
  as a migration source, so keeping it would have kept a CGO island alive
  for no live purpose — the fleet migrated, and reading a pre-SS-8 store
  now needs a pre-SS-8 build of fathom. `scanObservations`/`scanClaims`
  (shared, backend-agnostic row scanners `migrate.go` and `duckdb.go` both
  used) moved into `sqlite.go`, their sole remaining consumer.
- **`store.BackendForPath` now refuses `.duckdb` loudly**: the extension is
  still *recognized* (so a leftover pre-SS-8 file still participates in
  `PlatformStore`'s same-stem collision detection instead of vanishing
  silently) but opening one returns a typed error naming the file and
  pointing at a pre-SS-8 build — never a silent fallback. The empty-path
  in-memory convention (`""`) now opens an in-memory `SQLiteStore` instead
  of an in-memory `DuckStore`.
- **`go.mod`/`go.sum` cleaned by `go mod tidy`**: `github.com/marcboeker/
  go-duckdb/v2` and its bindings, plus every indirect dependency that
  existed only to support DuckDB/Arrow (`apache/arrow-go`, `klauspost/
  compress`, `klauspost/cpuid/v2`, `pierrec/lz4/v4`, `zeebo/xxh3`,
  `google/flatbuffers`, `go-viper/mapstructure/v2`, `goccy/go-json`,
  `golang.org/x/exp`, `golang.org/x/telemetry`, `golang.org/x/xerrors`),
  are gone.
- **`internal/store` is now pure Go**: `go build`/`go test -race` pass with
  `CGO_ENABLED=0` for the store package with zero workarounds — proven, not
  asserted. `CGO_ENABLED=1`/`MACOSX_DEPLOYMENT_TARGET=14.0` remain required
  for the full `fathom` binary, but now solely because of the tree-sitter
  extractor bindings (ADR-013) — a separate, not-yet-triggered removal
  (XT-6b). Taskfile/CLAUDE.md/scripts/docs updated to state the CGO
  requirement's real, current reason instead of the retired one.
- **Docs**: `docs/configuration.md`'s DuckDB→SQLite runbook is now marked
  historical; `docs/usage.md`'s `migrate-store` section is now a removal
  notice; `docs/why-duckdb.md`'s superseded banner now also records the
  removal; `docs/architecture.md`/`docs/distribution.md`/README.md updated
  to the SQLite-only, tree-sitter-is-the-CGO-reason reality;
  `scripts/migrate-fleet-store.sh` is marked historical (completed
  2026-07-23, retained for reference; step 4/8 can no longer run against a
  post-SS-8 binary); `scripts/reingest-fleet.sh` no longer calls the store
  lock "DuckDB's". Vision/design docs describing future DuckDB-specific
  mechanisms (VSS vectors, `ATTACH` federation) got engine-assumption notes
  flagging a technical re-check is needed before any of them fire — not a
  redesign of the mechanisms themselves.
- **Remaining manual step**: the on-disk rollback file
  `workspace/db/fleet.duckdb` on the fleet checkout is untouched by this
  removal (deliberately — SS-8 never touches the live fleet store) and is
  the operator's to delete once satisfied it's no longer needed.

### Added — the review verb: `review_diff` + `fathom review` (Phase C, 2026-07-23, docs/plans/review-verb.md C1-1/C2-1/C3-1)
- **C1-1 — `review_diff`, the program's one new MCP tool:** a new pure-Go
  `internal/analysis/review` package composes, in-process, diff impact
  (blast radius + typed edges), obligations/findings scoped to the changed
  files, architecture conformance (the service's own `.fathom/rules/
  archconformance.cue` project rule pack — `review_diff` never accepts
  inline rules), and spec coverage into ONE cited verdict
  (`Verdict.Dimensions[].Findings[]`, each carrying an evidence citation).
  Honesty rules apply per dimension: a dimension that cannot run reports
  `checked:false` + `reason`, never a clean-shaped pass. New MCP tool
  `review_diff` (`GroupInvestigation`, `internal/mcp/review_tool.go`); CLI
  mirror `fathom review --store DB --service S --since <ref>`
  (`cmd/fathom/review.go`, mirrors `cmdReport`'s store-direct structure —
  resolves changed files via `git diff --name-only <ref>`, then runs the
  identical engine). Refactor along the way: the arch-conformance edge
  normalization logic (method-origin/method-target package resolution,
  P1-4) moved from `internal/mcp/extra_tools.go`'s unexported
  `archConformanceEdges`/`archResolver` into
  `internal/analysis/archconformance.EdgesFromNodes` — one implementation
  the standalone `arch_conformance` tool and `review_diff` both call, never
  two that could drift. Likewise the inline/project rule-pack resolution
  (C4-2/C4-3's honesty split) is now `internal/projectrules.ResolveRules`,
  shared the same way.
- **C2-1 — the findings bridge (fires FT-7's pre-armed trigger,
  docs/plans/findings-tracer.md):** review findings are persisted as
  content-addressed claims on the `findings` layer (`ClaimID` = sha256 of
  layer+kind+subject+reproKey, tier `verified` — a deterministic pass, no
  LLM) via `review.AttachClaims`. Durability rides the store's own
  `PutClaims` upsert (the pre-existing "status stays refuted" CASE WHEN,
  FT-4) — no re-derived logic in this row: an unchanged finding re-emitted
  by a later review reproduces the identical `ClaimID`, so a finding
  refuted via `fathom claim refute` / the `obligations` tool's feedback
  action carries `status:"refuted"` (excluded from its dimension's active
  `count`, but still listed — never silently dropped) across every later
  re-review. Proven at three layers: `TestFindingsBridge_
  RefutationDurability` (pure, `internal/analysis/review`),
  `TestReviewDiff_FindingsBridge_RefutationSticksAcrossReReview` (MCP
  wiring, fake claims backend), `TestReview_FindingsBridge_
  RefutationDurability` (CLI, end-to-end through a real store).
- **C2-1 write-path soundness (critic round — five proven defects fixed
  before any release carried the bridge):**
  1. *Sweep contract:* persisted cites links now carry the REAL cited-line
     hash resolved at emit time, and a bare-path/unhashable citation gets
     no cites link at all — previously hashless bare-path links made the
     daemon's post-refresh `SweepClaims` flip EVERY review claim to
     stale/contradicted with fabricated "anchor no longer resolves" events
     (measured: 57 claims flipped, 26→77 lying events over two sweeps).
     Arch findings now cite the violating symbol's real `file#Lline` (new
     `archconformance.KeyIndex`); spec findings resolve their ref's line.
     `TestReview_SweepZeroFlips` pins zero flips/zero events.
  2. *Idempotency + serving purity:* only the deterministic-oracle
     dimensions persist (`arch_conformance`, `spec_coverage`) — never
     `obligations` (re-minting re-served claims as wrappers grew the store
     57→82 on an IDENTICAL re-run and displaced real obligations from
     `ObligationsFor`'s budget via exact-path scope ranking) and never
     `impact` info-noise. `TestReview_Idempotent_DoubleRun`,
     `TestReview_ObligationsForUnpolluted`.
  3. *Deliberate persistence:* `fathom review --no-persist` added; the
     pre-push hook uses it (a hook is never an undisclosed writer to the
     shared store); persist behavior disclosed in the tool description,
     change-review.md, and the plan. `TestReview_NoPersist_WritesNothing`.
  4. *Reconcile (no claim rot):* a persisting run over changed-file set F
     supersedes review-minted claims on files in F its CHECKED dimensions
     no longer emit — absent = fixed; unchecked dimensions never supersede.
     Every persisted claim also carries a scope-fp drift anchor (store
     fingerprint, FT-6) so `DriftedFindings` works for review claims; a
     finding whose file has no fingerprint is not persisted (upholding the
     zero-links invariant, whose sweep.go comment now names this bridge as
     the second legitimate writer). `review.BuildPersist` is the one
     shared MCP/CLI write composition.
     `TestReview_Reconcile_FixedFindingSuperseded`,
     `TestStaleReviewClaims_*`, `TestAttachClaims_LinkSoundness`,
     `TestAttachClaims_NoFingerprintMeansNoPersist`.
  5. *Tier admission:* the findings layer.cue (both copies) now records
     the verified-tier oracle admission (ADR-018 oracle classes / ADR-016
     M1 — deterministic, Model-empty findings) beside its sme-stated
     corpus tier; ADR.md's "no layer emits verified yet" updated;
     `TestClaimsVocabularyMatchesSemanticLayer` pins tier and admission
     text, not just kind.
  Plus: the obligations dimension now carries a served claim's
  stale/contradicted status label through to the verdict
  (served-with-label, `TestRun_Obligations_ServedStatusLabelCarriedThrough`)
  instead of mapping it away to severity only.
- **C3-1 — the forced channel:** `lefthook.yml`'s pre-push gains a
  non-blocking `fathom review --service fathom --since origin/main
  --no-persist` summary step — always exits 0 regardless of `bin/fathom
  review`'s own result, degrading operational errors to one line
  (fail-open, same doctrine as the `verify` step's exit 2); measured ~12s
  worst-case on a pathological 1777-file diff. The `review_change` MCP
  prompt now calls `review_diff` instead of narrating an 8-step manual
  tool sequence; that sequence is preserved as `docs/use-cases/
  change-review.md`'s appendix for tool-by-tool users, whose worked example
  now leads with the one-call form.

### Fixed — serving-layer hygiene (Wave 3a P1-6/P1-9, 2026-07-23)
- **`migration_targets` merge semantics (dogfood O11, closed):** a caller-
  supplied `deprecated_pkgs` list now **merges with** the built-in default
  list (deduplicated, deterministic order) instead of silently replacing it
  — matching the tool's own parameter description ("default list is always
  included"). Externally visible: callers passing a custom list now also
  get default-list findings.
- **Sweep event vocabulary:** the post-compile sweep's `claim_events`
  verdict for anchor-dead claims is now `"contradicted"`, matching the
  `ClaimContradicted` status it sets (previously wrote `"refuted"` — the
  word reserved for deliberate human/agent judgment, which is never served
  again; contradicted claims stay served WITH a label pending recompile).
  The store contract test had encoded the old conflation as its own fixture
  and now models the real contract.

### Fixed — Wave 3a pulled-tools excellence: O12's three product gaps + the C4-4 method-edge gap (docs/plans/pulled-tools-excellence.md P1-1/P1-2/P1-3/P1-4/P1-5/P1-8)
- **P1-1 — parameter-name inconsistency (closes O12(b)):** `find_symbol`,
  `query_symbols`, and `references` now additionally accept `symbol` as an
  additive alias for their native argument (`name`/`concern`/`query`
  respectively — `symbol_context` already used `symbol` natively). Native
  name stays primary; a call supplying both only errors (`ErrCodeParams`,
  naming both parameters) when the values disagree. New
  `internal/mcp/paramalias.go`.
- **P1-2 — cross-repo call edges missing (closes O12(a)):** `symbol_context`
  and `find_callers` now consult the fleet backend for cross-service call
  edges when one is wired (fleet stores) — new `cross_service_callers`/
  `cross_service_callees` result fields, each entry scope-labeled
  `"cross-service"`, additive and never merged into the existing
  same-service `callers`/`callees` fields. Reuses `cross_repo_callers`'s
  own fleet-wide matching (`fleetCallers`, extracted so both tools share
  one implementation) plus a new symbol-precise cross-service callee
  resolver (mirrors `internal/platformlink`'s qualified-match rule at
  single-symbol granularity, since `CrossRepoEdge` tracks the calling FILE,
  not the calling symbol). New `internal/mcp/crossservice.go`.
- **P1-3 — case-insensitive tier steers to the wrong package (closes
  O12(c)):** `find_symbol`'s case-insensitive tier now returns a
  package-match group alongside the pre-existing name-match group, so a
  query naming an entire package (e.g. "heartbeat") is no longer hidden
  behind a same-named type/symbol in an unrelated package — both groups
  return together, ranked, never first-hit steering. `matched_via` stays
  `"case_insensitive"`. `caseFoldMatches` (internal/mcp/findsymbol.go).
- **P1-4 — arch_conformance's disclosed method-edge gap, closed (C4-4):**
  both edge endpoints are now normalized to their declaring Go package
  before matching rules, so package-named rules catch method-origin AND
  method-target edges, not only function-to-function ones. New
  `archConformanceEdges`/`archResolver` (internal/mcp/extra_tools.go); the
  "DISCLOSED GAP" comment in `schema/rules/archconformance/schema.cue`
  (and its byte-identical embed,
  `internal/projectrules/archconformance_schema.cue`) is now "CLOSED".
- **P1-5 — the proof re-run:** re-ran the three cross_repo_callers bench
  tasks (B1-T09/T10/T11) against a freshly-ingested scratch fleet store
  (never the live daemon). B1-T09 (the O12 flagship case) went from
  504,785 tokens / 20 turns to 321,100 tokens / 9 turns (−36% / −55%) —
  improving 6.6×→4.2× vs. grep's answer, which is **still 4.2× more
  tokens than grep: a non-flip on the ratio, residual tracked** in
  dogfood O12 (kept under Open). Part of the residual is real code
  structure: a live probe found `cmdInstrument` delegates two of the
  three heartbeat calls through `reportInstrument`, a one-hop indirection
  the original symptom's prose had collapsed. B1-T10/T11 were already
  near the cheap 3-turn floor pre-fix and show ~1% token noise — also
  non-flips, recorded as such. See dogfood O12 and the plan's P1-5 row
  for the full numbers.
- **P1-8 — dispatch now enforces declared `required` fields
  (`internal/mcp/dispatch.go`'s new `validateRequired`):** previously only
  individual handlers enforced their own required parameters. A P1-1 alias
  satisfies its primary's requirement at this layer too
  (`toolDescriptor.aliases`), so an alias-only call is never rejected for
  "missing" the native name. Batch sub-calls get the same gate (the batch
  loop previously invoked handlers directly, bypassing it); a failing
  sub-call errors into its own result, never the whole batch. Three tools
  change observable behavior — each declared a required field its handler
  never checked, and each now refuses with `-32602` where it used to run:
  - `license_policy` without `allowed` (C4-5's original find; it used to
    report against an empty allowlist).
  - `spec_coverage` without `spec_ids` (used to return an empty coverage
    matrix — the "orphan-listing" use C4-5 classified as legitimate; the
    declared contract now wins, and restoring orphan-listing would be a
    schema change making `spec_ids` optional, not a validation hole).
  - `spec_status` without `spec_ids` (same shape as spec_coverage).

### Added — spec-driven development adoption guide (AD-3, docs/plans/adoption-docs.md)
- **`docs/adoption/spec-driven-development.md`**: the third and final definitive
  adoption guide (alongside `claude-code.md`/`codex.md`) — the full SDD loop
  (author a plan in the `{id, symbol, package, ...}` grammar → implement →
  `fathom verify` proves rows → the pre-push gate blocks over-claiming →
  computed progress → archival lifecycle → `spec-bindings` layer binding →
  oracle classes), written against this repo's own live adoption program
  (`docs/plans/activation.md`, `docs/plans/adoption-docs.md`) as the worked
  example, with real `fathom verify`/`gen-plan-cue` output captured from this
  worktree's build. `fathom plans status`/`next`/`archive`/`restore` (E3/E4,
  `docs/plans/new-use-cases.md`) are documented from the committed design and
  marked "(landing this wave)" — not yet built in this worktree.

### Fixed — Wave-1 adoption-friction findings (dogfood)
- **`task build` silently diverged from every other build-path convention**:
  it wrote to `workspace/tmp/fathom` while the post-commit rebuild hook,
  `fathom init`'s baked-in hook commands, and the daemon-install pattern all
  expect `bin/fathom` — `docs/adoption/claude-code.md` §1 had to warn
  adopters away from the task runner's own default build target. `Taskfile.yml`'s
  `build` task now emits `bin/fathom`; the fast inner-dev-loop variant the
  calibration harness depends on (`calibration/baseline.sh`'s `BIN` default)
  moved to a differently-named `build-dev` task, still emitting
  `workspace/tmp/fathom`. Verified: `task build` → `bin/fathom` exists and
  runs; `task build-dev` → `workspace/tmp/fathom` unaffected;
  `task scripts-check` green.
- **`docs/use-cases/getting-started.md` documented the removed `set_profile`
  tool**: it narrated a stdio-only per-session profile switch (Step 5, the
  Tools table, the "Profiles keep the tool surface focused" bullet, the
  `serve-mcp`/`serve-http` table, and the `## Maintained` line) that ADR-014's
  surfaces-week1 amendment retired entirely. Rewritten against
  `docs/mcp-tools.md` (the truth source): path-scoped HTTP surfaces
  (`/rpc/agent` aka `/rpc/companion`, group surfaces, bare `/rpc`),
  execute-and-flag gate semantics (`off_surface:true`, never a hard reject),
  and a real `capabilities` response shape (`active_surface`/`active_profile`).
- **`docs/usage.md`'s CLI reference stopped at `session-hook`**: thirteen
  shipped verbs had no reference entry — `init`, `compile`, `layers`,
  `verify`, `config`, `providers`, `smoke`, `instrument`, `edit-hook`,
  `stop-hook`, `claim`, `remap`, `report`. Added one section each, flags
  verified against `cmd/fathom/*.go`. (`fathom plans` is not yet present in
  this worktree, so it's not documented here — see the SDD guide above for
  its committed design.)

### Added — `fathom plans` (E3/E4, docs/plans/new-use-cases.md), reverse-drift report (E6-1), verify_plan latency SLO (B3-1, docs/plans/falsifiability.md)
- **`fathom plans status`** (E3-1): a per-plan rollup — proven, claimed-
  unproven, open, deferred-with-trigger, skipped-other — computed fresh by
  running the existing verify engine over one or more plan files (or every
  active `docs/plans/*.md` by default); never an asserted state. `--json` +
  a human table; deterministic, sorted by plan file. `cmd/fathom/plans.go`.
- **`fathom plans next`** (E3-2): the ordered work frontier for an agent —
  open/in-progress rows plus claimed-but-unproven rows (verify Fail as
  `claimed-failing`, verify Unknown as `claimed-unknown` — the same set CI's
  `--strict` blocks on, so the frontier never reports "no open work" on a
  plan CI rejects), each with its `{id, symbol, package}` anchors and
  originating plan file. Ordering is (plan file, then DOCUMENT order) —
  never TaskID-lexical order, since an author-owned `id:` need not sort the
  way it was written.
- **`fathom plans archive <plan>` / `fathom plans restore <plan>`** (E4-1):
  the governed artifact lifecycle. Archive refuses unless (a) every row is
  `[x]`-claimed-and-currently-verifying-Pass or `[-]` skipped, (b) every
  row's task ID is lexically absorbed into CHANGELOG.md or ADR.md at a
  token boundary — "E3-10" never absolves "E3-1" (this entry is that
  absorption for E3-1, E3-2, E4-1, E4-2, E6-1, B3-1), (c) the engine is
  re-run fresh, not trusted from a prior pass — and additionally preflights
  that the plan file itself has no uncommitted changes, so the tombstone's
  HEAD sha always names the exact content archived. On success the plan is
  `git mv`d to `docs/plans/archive/` and a tombstone (date, HEAD sha,
  verify summary) is written into its header and re-staged (a staging
  failure is surfaced, never swallowed); `restore` reverses the move,
  refusing if an active plan of the same name exists, and leaves the
  tombstone in place as the historical record. `cmd/fathom/plans_archive.go`.
- **Pre-push/CI gate scoped to active plans** (E4-2): both `lefthook.yml`'s
  `verify` command and `scripts/ci.sh` already only walk the top level of
  `docs/plans/` (a non-recursive glob / `find -maxdepth 1`), so
  `docs/plans/archive/` was structurally out of gate scope the moment E4-1
  introduced it — documented explicitly in both files and
  `docs/plans/README.md` rather than left as an accident: each archived plan
  now removes exactly one `fathom verify` invocation per push/CI run.
- **`fathom report --analyzer reverse-drift`** (E6-1): public API surface
  (`internal/analysis/apisurface`, unchanged; `_test.go` symbols excluded —
  test helpers were 68% of the raw exported surface on the fleet index)
  minus spec-bindings coverage (served `spec-bindings`-layer claims, matched
  by file-path glob against each claim's `Scope` — the same mechanism
  `internal/semantic/serve.go` already uses) = the ungoverned public
  surface, per service. When zero spec-bindings claims are compiled the
  report banners that the listing is the whole surface, not measured drift.
  Read-only, one markdown doc, deterministic.
  `cmd/fathom/report_reversedrift.go`.
- **verify_plan p95 ≤ 5s SLO + Stop-hook fail-open timeout** (B3-1,
  docs/plans/falsifiability.md): the measured p50 2.2s / p95 4.5s latency
  and an explicit p95 ≤ 5s budget are now recorded in
  `docs/design/coding-agent-excellence.md` §2's ledger. The Stop-hook's
  verify caller (`cmd/fathom/stophook.go`) times each `verify_plan` call at
  that same 5s ceiling (`stopHookVerifyTimeout`); a call that exceeds it
  degrades to an advisory warning ("verify timed out — treat as
  unconfirmed") for that plan and keeps checking the others, never a
  blocked session end and never silent — a timeout stays advisory even under
  `--block`, since it is an operational condition, not a proven-false claim.

### Added — context-bench + the ×10 retraction (falsifiability B1/B2-1, 2026-07-23)
- **context-bench built and run** (docs/plans/falsifiability.md B1-1/B1-2/B1-3):
  15-task committed corpus (`testdata/contextbench/tasks.json`, ground truth
  grep-derived at a recorded commit, three scoring rules), tested scorer
  (`internal/contextbench`, 16 tests; `cmd/contextbenchscore` CLI, 7 tests),
  two-arm `claude -p` runner (`scripts/context-bench.sh`: fathom-MCP-only arm
  vs grep/find/cat-only arm, `--strict-mcp-config` isolation, honest
  degradation when the daemon is unreachable).
- **The ×10 tokenomics claim is RETRACTED** (B1-3/B1-4): first full run — all
  30 arm-runs real — measured equal task success (15/15 both arms) at 1.38×
  aggregate / 1.79× mean per-task end-to-end token reduction and 2.81× mean
  wall-time reduction. The pre-registered "<5× → retract" criterion fired;
  every doc stating ×10 / ≤1/10th as a claim now states the measured
  magnitudes (coding-agent-excellence §0/§2/§3/§7, product-vision §3,
  use-cases/coding-agents §5, vision 04 §1, vision 11 §3-B1).
- **B2-1 existence-check wiring**: `scripts/check-surface-traffic_test.sh`
  proves the three verdict classes (ok / zero-traffic / missing-dependency)
  against a local fixture HTTP server, never the live daemon; the script
  header now distinguishes per-surface `workspace/logs/<surface>.stamp` files
  from the program-wide `workspace/logs/adoption-clock-t0.txt` (law-8 clock —
  explicitly NOT started; the file does not exist).

### Fixed — relative corpus paths at the ingest boundary (dogfood O10, 2026-07-22)
- **Typed edges silently absent for relative `--path`** (the CLI default
  `.`): the connector-scope filter rel-mapped absolute go/packages file
  positions against a relative `repoPath`, so every package was excluded —
  zero edges, zero failures, zero output. `EnrichTypedEdges` now normalizes
  `repoPath` to absolute at entry; a never-silent stderr guard reports the
  "packages seen but all excluded" condition; `cmd/fathom ingest` prints the
  typed-edge counters (their absence was invisible precisely because it
  didn't).
- **`ServiceMeta.Path` recorded verbatim-relative**: `--path .` from a
  sibling repo's post-commit hook would be resolved from the DAEMON's
  working directory on auto-refresh — refreshing the wrong corpus under that
  service ID. The pipeline now records the corpus root absolute. Both repos'
  post-commit hooks pass `--path "$PWD"` (agentic-kit 55ef7cf).
- **Operational**: `scripts/migrate-fleet-store.sh` — the executable form of
  the ADR-020 DuckDB→SQLite runbook (stop → quarantine the mismatched
  same-stem file → migrate → full re-ingest → install the daemon binary to
  `~/.local/libexec/fathom/` → repoint the plist, with `go` on the daemon
  PATH for typed enrichment → restart → verify → stamp the SS-8/XT-6b bake
  clocks). The daemon now runs an installed copy, decoupled from post-commit
  rebuilds of `bin/fathom`.

### Added — tree-sitter extraction engine: wazero/WASM (ADR-021 §4, docs/plans/extraction-treesitter.md)
- **`internal/extractor/tsengine`**: a pure-Go tree-sitter execution engine —
  tree-sitter's own C core runtime statically linked with one grammar's
  `parser.c`/`scanner.c` per language (plus the checked-in `budget.c` shim),
  compiled to a self-contained `wasm32-wasi` module (`zig cc
  --target=wasm32-wasi-musl -mexec-model=reactor` + `wasm-opt -Oz`) and
  executed by `github.com/tetratelabs/wazero` (now a direct dependency) — no
  CGO in the engine, no Docker at runtime, no Emscripten dynamic linking.
  The grammar repos' own released `tree-sitter-<lang>.wasm` artifacts (and
  the matching `web-tree-sitter` core) were evaluated first and found to
  require reimplementing Emscripten's dylink relocation + a JS-runtime
  callback shim to run under bare wazero — confirmed empirically, not just
  cited from ADR-013; see `internal/extractor/tsengine/grammars/
  PROVENANCE.md`'s "path not taken" for the concrete import/export dumps.
  Blobs (exact bytes: JS 458,741; TS+TSX 2,037,040; Python 539,848; Ruby
  1,710,411) are rebuilt by `grammars/build.sh`; provenance (upstream repo/
  version/sha, full sha256, toolchain versions) in `grammars/PROVENANCE.md`.
  **The binary-wide CGO win is PENDING, not achieved here** (owner ruling,
  plan M2): the CGO tree-sitter deps remain in go.mod as non-load-bearing
  in-tree incumbents until the fleet bakes N=7 days green on tree-sitter
  primary, then get deleted (plan row XT-6b); DuckDB's CGO is a separate,
  ADR-020-tracked surface.
- **`internal/extractor.NewTSExtractor`** (XT-1): one `TSExtractor` type
  parameterized by (grammar, `.scm` query set) implements `extractor.
  Extractor` for `"javascript"` (dispatching `.js`/`.jsx`/`.ts`/`.tsx` — the
  extensions fusion registers), `"python"`, and `"ruby"`. Query files
  (`internal/extractor/tsqueries/*.scm`) are real tree-sitter queries
  (method-name alternations include `private_property_identifier` and
  `computed_property_name`, so `#private` and `[Symbol.iterator]` methods
  index — critic B2) compiled and matched via tree-sitter's own query
  engine, not a hand-rolled tree walk. Symbol qualification ("Class.method",
  Ruby's "Outer::Inner::Widget") is one generic containment algorithm over
  the flat match list. A Parser+Query is created once per (grammar, entry
  point) and reused for the process's lifetime (mutex-guarded).
- **Pathological-input bounds (critic B1)**: `({[` x 50,000 (150KB, a
  truncated minified bundle's shape) ran 52.9s unbounded (larger inputs 11+
  min) where the CGO incumbent takes ~49ms — measured root cause: the parse
  finishes in ~70ms, the QUERY cursor's error-tree scan is the 45s+. Now
  bounded three ways: guest-side budgets via tree-sitter's own progress
  callbacks (`grammars/budget.c`, `tsengine.ParseTimeout` default 10s; the
  wazero `WithCloseOnContextDone` alternative measured ~3x slower corpus-
  wide and was rejected), a host-side hard backstop that abandons any guest
  work the callbacks fail to stop (`tsengine.ParseHardBackstop`, default 2x
  budget; goroutine+reaper, module memory freed when the runaway finishes),
  and parser re-creation on any engine fault. Faults are counted per-ingest
  on **`fusion.Report.TSEngineFaults`** with one stderr line each (critic
  M1 — previously a silent nil,nil poisoned the parser for the daemon's
  lifetime); the faulted file is fingerprinted, and the next file of that
  language extracts on a fresh parser (tested at engine/extractor/fusion
  levels).
- **Calibration (XT-2/XT-3/XT-4, `calibration/baseline.sh` before/after on
  the 12-repo corpus)** — the harness's "recall" column is honestly a
  **coverage ratio** (extracted code symbols ÷ grep-proxy definitions), not
  true recall: coverage never regressed on any repo; JavaScript/TypeScript
  show the largest gains (lodash 0.41→1.26; nest/typeorm/zod/axios all up,
  including reclaimed `#private`/computed methods after B2). Python/Ruby
  were already tree-sitter via CGO bindings (ADR-013) — not "heuristic" —
  and tie their incumbents; the owner ratified ship-on-tie conditional on
  the XT-6b CGO-deletion path (recorded verbatim in the plan). Corpus
  blindness recorded: zero `#private` methods exist in the six JS/TS corpus
  repos, so the gate could not catch B2 — a modern-syntax repo is flagged
  for the next corpus revision. Performance: strict interleaved medians put
  the corpus ingest at **1.69x the incumbent (exceeds the plan's 1.5x
  line — flagged in the plan for the owner's gate call, not silently
  claimed as met)**; the first commit's 1.41x was measured against a
  slower incumbent run. Scores + both protocols recorded in
  `docs/plans/extraction-treesitter.md`.
- **`fusion.ExtractorEpoch` bumped 4→5** (Python/JS/TS/JSX/TSX/Ruby now
  produced by the new engine — every already-indexed file's stored
  observations were produced by the CGO extractor, so re-extraction is
  forced) with a new epoch-invalidation test seeded at the true prior epoch.
- **`capabilities` MCP tool** gains `extraction_engines` (XT-5): per-extension
  engine provenance (`"tree-sitter-wasm"` for `.py`/`.js`/`.jsx`/`.ts`/
  `.tsx`/`.rb` — exactly the extensions the connector discovers and fusion
  registers; `.pyi`/`.mjs`/`.cjs` are parseable but not wired, so not
  claimed) via `extractor.EngineProvenance`.

### Added — typed Go call edges dissolve the O9 ambiguity residue (ADR-021 §3, docs/plans/callgraph-types.md)
- **`internal/fusion/typedcalls.go`**: `EnrichTypedEdges`, an additive Go-only
  pass (`golang.org/x/tools/go/packages`, promoted to a direct dependency)
  that runs in `Ingest` after syntax extraction and resolves call sites via
  `go/types`: `EdgeDirect` (statically resolved target — embedded-struct
  promotion and generic instantiation resolve for free), `EdgeIfaceDispatch`
  (one edge per in-scope implementer of an interface-typed call), and
  `EdgeUnresolved` (func/method values — explicit, never silent). The
  syntax-derived `Calls` contract is untouched. Scoped to the connector's
  Discover set (`.fathomignore` respected — an ignored file's colliding
  package/func names can never fabricate edges onto live symbol keys), with a
  per-package content-hash cache, a post-load reconcile that purges rows for
  deleted/moved packages (a `git mv`-ed package keeps its SymbolKeys, so a
  stale row would otherwise persist as fabricated reachability evidence
  forever), and per-package failure counters threaded into `fusion.Report`
  (`TypedPackages`, `TypedPackagesFailed`, edge counts by kind) plus a stderr
  line on failure — a daemon whose typed pass always fails (no `go` on PATH)
  is distinguishable from a non-Go service. `Config.DisableTypedEdges` turns
  the pass off (degraded environments; true syntax-only baseline measurement).
  A struct that satisfies an interface only via an embedded interface field is
  never offered as a dispatch implementer, and a promoted call through one
  fans out to the real implementers — never to a phantom interface-method key
  `goextract` doesn't create.
- **`core.Edge` gains `Kind`**; new store surface (`PutTypedEdges`,
  `TypedEdges`, `ListTypedEdgePackages`, `DeleteTypedEdgePackage`, per-package
  fingerprints) round-trips it through SQLite, DuckDB, and the memory fake,
  contract-tested; `PurgeService` clears typed-edge rows too.
- **verify (CT-4)**: `test_reaches` consumes typed edges via
  `verify.DetailedEvidence` — a path crossing an interface-dispatch edge (the
  old unconditional refusal for a method name shared by multiple receivers)
  now verifies, labeled `"reached via interface dispatch (N implementer(s))"`
  in the criterion detail. Three real O9 residue shapes (adapter `Complete`,
  Store contract-suite `PutClaims`, `Ping`) are proven end-to-end flipping
  from refusal to typed reach against a real fixture module.
- **Serving (CT-6)**: `blast_radius`/`diff_impact` traverse typed edges;
  `find_callers` adds `typed_callers`, `symbol_context` adds `typed_edges`
  (kind-labeled); `server_stats` takes an optional `service` and reports
  per-kind typed-edge counts (all-zero = the pass hasn't run for that store).
  Note: universe/stdlib interface dispatch (`error.Error`) fans out to every
  first-party implementer and dominates dispatch counts by design.
- **Performance gate (CT-5)**: first-run typed load measured against a TRUE
  syntax-only baseline (`DisableTypedEdges`) on this repo — see
  docs/plans/callgraph-types.md for the recorded numbers; cached re-ingest is
  delta-only (only changed packages reload).

### Fixed — tool identity critic round (ADR-021 §2, critic F1–F6 on e500b28)
- **ToolKind/Generated now actually persist** (F1, HIGH): both fields were
  write-only against the real stores — the sqlite and duckdb column lists
  dropped them while the memory fake (whole-struct append) round-tripped
  them, so every asserting test passed against a lie. Both columns now
  exist in both backends (sqlite: fresh DDL + a PRAGMA table_info
  conditional-ALTER for pre-existing scratch stores; duckdb: the existing
  ALTER-IF-NOT-EXISTS legacy path), are written and scanned, and the
  round-trip assertion lives in the shared store CONTRACT suite so the
  memory fake and the real backends can never diverge on these fields
  again.
- **CRD tagging is per-document** (F1e/P15): a multi-doc YAML stream
  holding a CRD and a Deployment tagged EVERY observation "k8s-crd"
  (file-wide classifier verdict). `core.SymbolNode.ToolKind` lets the k8s
  extractor — which sees document boundaries — tag each node; the pipeline
  prefers the per-node verdict over the per-file one.
- **Guard-wrapped Helm templates extract** (F2): `{{- if ... -}}` guards
  and whole-line control flow became placeholder scalars that broke the
  YAML decode — half of every `helm create` chart
  (ingress/serviceaccount/hpa) still emitted zero symbols. Whole-line
  actions are now dropped, not substituted. Measured on the 10-file demo
  corpus (helm chart incl. a guard-wrapped ingress template + compose
  override + kustomize + k8s + root-pattern Dockerfile + OpenAPI doc):
  7 symbols at the pre-toolident baseline → 23 after this round; the
  guard-wrapped ingress contributes its Ingress workload node, which the
  pre-critic build could not decode at all (it saw only the values refs).
- **image↔Dockerfile keying fixed** (F3): `build/myapp.dockerfile` was
  keyed by DIRECTORY name, giving an image literally named "build" a false
  reference and the actual "myapp" none — `*.dockerfile` files now key by
  STEM. A repo-root `Dockerfile` (the most common layout) keyed "." and
  never matched; when the repo has exactly one Dockerfile and it sits at
  the root, image refs now fall back to it (repos with 2+ Dockerfiles
  never guess). Documented residual: in a lone-root-Dockerfile repo a
  public image (postgres) also matches the fallback — TierPreScan and the
  `dockerfile:` signature flag it as the heuristic it is.
- **IsGenerated honors the real conventions** (F4): for Go, the official
  `// Code generated ... DO NOT EDIT.` line is now accepted anywhere
  before the package clause (bounded 4KB) — the zz_generated ecosystem's
  license-header-first files (marker at line ~17) were all missed by the
  old 3-line window. For other languages the marker line must begin with
  that language's comment token, so markdown or YAML values QUOTING the
  marker no longer vanish from cloc's code counts (markdown's only honored
  form is an HTML comment `<!-- Code generated ... -->`). Residual
  documented in the func doc: unrecognized-language files use a permissive
  token set and can still false-positive on a verbatim quote.
- **Compose signal + classified-but-zero visibility** (F5):
  `services: # comment` (valid compose) no longer defeats the top-level
  signal. And a file that classifies but then extracts zero symbols (a
  GitLab-CI config's top-level `services:` LIST matching the compose
  signal) — the last surviving silent-zero path — is now counted: per-tool
  `classified_zero_files` beside the unclassified/oversize counters in
  `fusion.Report`, `Pipeline.ClassifiedZeroStats()`, CLI ingest output,
  and `server_stats`.
- **Docs** (F6): `server_stats` counters documented as EVENT counters
  since process start (reset on daemon restart), not index properties; the
  two distinct "unclassified" meanings (cloc's language bucket vs the
  classifier's event counter) cross-referenced in both rows; the `.tpl`
  values-ref scope-out stated in the helm paragraph below.

### Added — tool identity: files understood as what they are (ADR-021 §2, docs/plans/toolident.md)
- **Pass-3 tool-identity classifier** (`internal/fusion/classify.go`,
  `fusion.Classify`): between the hash-gate and extractor dispatch, yaml/cue
  files whose extension is ambiguous are content-classified into a `ToolKind`
  (k8s manifest, CRD, compose, helm-template-yaml, kustomize, openapi,
  cue-module, timoni, or unclassified) instead of being dispatched purely by
  path/extension. A per-ingest **sibling index** (`buildSiblingIndex`), built
  once from the Discover list, answers "does an ancestor dir contain
  Chart.yaml / cue.mod?" cheaply.
- **Helm templates/*.yaml no longer a blind spot**: `HelmTemplateYAMLExtractor`
  substitutes every `{{ ... }}` action span with a scalar placeholder
  (balanced on the `{{`/`}}` delimiter pair) before YAML decode, recovering
  the k8s document shape from the majority of a real chart's content that
  previously failed yaml.v3 outright and silently emitted nothing. Lines
  consisting solely of template actions (guards and control flow —
  `{{- if ... -}}` / `{{- range }}` / `{{- end }}`) are dropped rather than
  placeholder-substituted, so the guard-wrapped templates `helm create`
  emits (ingress/serviceaccount/hpa) decode too. Templated
  `metadata.name` falls back to the file stem with a `templated:true` marker;
  `.Values.x.y` references are captured as `KindHelmValueRef` observations.
  `HelmValuesExtractor` now recurses into nested values.yaml keys (dotted
  names), giving values-key ↔ template-usage an exact join via the
  `references` tool. Scope note: `.Values` references are extracted from
  `templates/*.yaml`/`*.yml` only — usage inside `.tpl` partials
  (`_helpers.tpl`) is NOT captured, so a values key referenced exclusively
  from a partial is invisible to that join (the `.tpl` extractor still
  emits the partial's named templates themselves).
- **Kustomization understood as what it is**: `KustomizeExtractor`
  (`internal/extractor/kustomizeextract.go`) emits `KindKustomizeResource`/
  `KindKustomizePatch`/`KindKustomizeImage` reference observations from
  `resources:`/`patches:`/`images:` — previously parsed-then-discarded by the
  generic k8s extractor, which only emits a workload node when
  `metadata.name` is present (kustomization.yaml never has one, so it silently
  produced zero symbols).
- **Compose files recognized by content, any filename**: a top-level
  `services:` mapping key now classifies as compose regardless of basename
  (`docker-compose.override.yml` and friends), not just the four hardcoded
  canonical names.
- **OpenAPI/Swagger documents** (`internal/extractor/openapiextract.go`):
  detected by a top-level `openapi:`/`swagger:` key, emitting
  `KindOpenAPIOperation`/`KindOpenAPISchema` instead of falling through to the
  k8s extractor for silent zero output. CRDs (`apiVersion:
  apiextensions.k8s.io`) are now tagged distinctly from generic k8s manifests.
- **CUE module / timoni awareness**: `.cue` files under a `cue.mod/` ancestor
  or matching timoni's `bundle:`/`timoni.cue` markers are tagged
  `cue-module`/`timoni` on their observations (`core.Observation.ToolKind`),
  layered on the existing LT-4 CUE extractor without duplicating it.
- **Generated-file detection** (`connector.IsGenerated`): the "Code generated
  ... DO NOT EDIT" convention (Go's own, shared by protoc-gen-go/mockgen/
  stringer) tags observations (`core.Observation.Generated`) and
  `core.SourceFile.Generated`; `cloc` reports a `generated` files/bytes
  bucket (same mechanics as the `binary` bucket), excluded from `by_lang`
  and `total`.
- **Unclassified/oversize visibility**: yaml/cue files whose content matches
  no known tool are counted (`fusion.Report.Unclassified`), never silently
  routed into the k8s extractor for a zero-symbol result. Files over 2MB
  (`fusion.MaxExtractSize`) are skipped from extraction and counted
  (`fusion.Report.Oversize`) rather than parsed. Both counters also
  accumulate process-lifetime on the `Pipeline` (`Stats()` /
  `Client.IngestStats()`) and surface via the `server_stats` tool and as a
  per-call delta on the `ingest` tool, through the optional `statsBackend`
  capability (forwarded through the auto-refresh wrapper).
- **Cross-refs, cheap tier**: a k8s/compose container image whose base name
  matches a Dockerfile found in the same ingest (by parent-directory name)
  gets a synthesized `KindImageDockerfileRef` reference observation
  (`TierPreScan` — a same-name match, not a resolved build dependency).
  Selector↔Service and cross-repo image resolution remain deferred (TI-9).
- **`fusion.ExtractorEpoch` bumped "3"→"4"**: the tool-identity classifier
  changed yaml-family extraction semantics for every already-indexed
  yaml/cue file, so the salt bump forces re-extraction rather than leaving
  stale pre-classifier observations in place.
- Deferred (see docs/plans/toolident.md): Selector↔Service and cross-repo
  image resolution, Helm render-time evaluation (TI-9); jsonnet/.ipynb
  extraction (TI-10) — no fleet corpus contains them yet.

### Added — pure-Go SQLite (WAL) store backend replaces DuckDB as the default (ADR-020, docs/plans/store-sqlite.md)
- **`internal/store/sqlite.go`**: `SQLiteStore` (`modernc.org/sqlite`, pure Go)
  implements the full 26-method `Store` interface with a fresh, final-column
  schema (no DuckDB-style `ALTER TABLE IF NOT EXISTS` migration cruft — every
  SQLite store is created with the final columns from day one). Per-connection
  DSN pragmas (`journal_mode=WAL`, `busy_timeout=5000`, `synchronous=NORMAL`)
  apply to every physical connection the driver opens, not just the first —
  verified live by `TestSQLite_WALActuallyEngages` (asserts `PRAGMA
  journal_mode` really reports `"wal"`, not just "no error"). The read pool is
  deliberately left uncapped for file stores — DuckStore's
  `SetMaxOpenConns(1)` write-serialization is DuckDB-specific and does NOT
  carry over; SQLite's own single-writer lock already serializes writes. The
  sole exception is the in-memory store (`path == ""`), whose pool is capped
  at one connection: every physical modernc connection to `":memory:"` opens
  its OWN private database, so an uncapped pool made concurrent readers fail
  with "no such table" (`TestSQLiteStore_ParallelReaders` covers both modes:
  memory proves serialized correctness, file proves true reader concurrency).
  Fresh DDL also carries an `observations(service_id)` index — every
  observations query filters on it.
- **Cross-process concurrency proof** (`TestSQLite_ReaderDuringWriter`): two
  independent `*sql.DB` handles against the same on-disk file — the honest
  in-process approximation of two OS processes — prove a reader completes a
  `SELECT` while a separate connection holds an open, uncommitted write
  transaction. This is the property DuckDB's cross-process single-writer lock
  never gave fathom (daemon writes forced CLI `ingest`/`claim`/`report` to fail
  with a lock error) and the reason for the swap.
- **`store.BackendForPath(path)`** dispatches `.duckdb` → `DuckStore`,
  `.sqlite`/`.db` → `SQLiteStore`, replacing four independent `.duckdb`
  hardcodes (`store.Open`, `PlatformStore.load`, `gen-platform-manifest`,
  `platform-ingest`). `PlatformStore` now loads a directory of mixed-backend
  store files (proven by `TestOpenPlatform_MixedBackends`) — a fleet mid-
  migration, one service moved to SQLite at a time, loads correctly
  throughout. Same-stem duplicates (svc.duckdb beside svc.sqlite — exactly
  the state `migrate-store` leaves) resolve via `store.ResolveStoreFiles`:
  the `.sqlite` file wins with a loud stderr warning naming both files,
  never a silent name-order first-wins that would serve stale
  pre-migration data; `gen-platform-manifest` dedupes identically (one
  entry per service). Store-extension matching is case-insensitive.
- **`fathom migrate-store --from X.duckdb --to Y.sqlite`** ports the eight
  non-derivable tables (`semantic_claims`, `claim_links`, `claim_events`,
  `compile_runs`, `service_meta`, `commits`, `knowledge_cards`,
  `doc_summaries`) store-direct into a freshly created SQLite store —
  cards/summaries are LLM-derived enrichment a re-ingest does NOT rebuild
  (enrichment is opt-in and unwired in production; `PurgeService`
  deliberately retains cards for the same reason), so treating them as
  derivable would have silently lost them. The source is opened READ-ONLY
  (DuckDB `access_mode=read_only`, no schema DDL) and left byte-identical.
  Prints a per-table row count + content spot-hash, each hash VERIFIED by
  re-reading the ported rows from the target and comparing (mismatch fails
  the migration), and the exact re-ingest command for the derived remainder
  (`observations`, `fingerprints`). Orphan `claim_links` rows (claim_id
  with no claim — unreachable by any query) are skipped with a counted
  warning rather than ported as dead rows. Refuses to write into an
  existing target that already has rows in any of its tables; validates
  `--from` ends in `.duckdb` and `--to` ends in `.sqlite` before touching
  either file. Operator runbook (stop daemon → migrate → repoint →
  restart → re-ingest → delete old store): docs/configuration.md.
- **Doc-comment truth fix**: `Store.PutObservations`'s comment implied
  DB-level dedup/upsert ("must not duplicate rows on repeated calls"). Neither
  backend has ever done this — it is a bare `INSERT`/append; the
  purge-then-insert replace semantics that make re-ingest idempotent are
  enforced entirely by the caller (`internal/fusion` calls `PurgeSource`
  immediately before `PutObservations` on every changed file). The comment now
  states the real contract.
- **Defaults flip to SQLite**: the CLI `--store` flag default across
  `ingest`/`query`/`inspect`/`gen-claude-md`/`gen-status-cue`/`ingest-diff`/
  `watch`/`platform-callers` moves from `workspace/db/fathom.duckdb` to
  `workspace/db/fathom.sqlite`; `platform-ingest` now writes
  `<service>.sqlite` per service; `fathom init`'s starter `config.toml`
  scaffold and daemon-down manual instructions follow suit. `docs/
  configuration.md`, `docs/architecture.md`, `docs/usage.md`, `docs/
  claude-code-integration.md`, `docs/distribution.md`, and `docs/why-duckdb.md`
  (marked superseded, kept as historical rationale) updated accordingly.
  Repo tooling followed: `lefthook.yml`'s post-commit refresh,
  `scripts/reingest-fleet.sh`, `scripts/ci.sh`'s verify scratch store,
  `calibration/baseline.sh`, and `CLAUDE.md`'s store-path references all
  moved off `.duckdb` paths so nothing keeps regenerating the same-stem
  collision state. `docs/usage.md` gained a `migrate-store` catalog entry
  and `docs/configuration.md` the full ADR-020 operator runbook (stop
  daemon → migrate → repoint `[daemon].store`/plist → restart →
  re-ingest → delete the old `.duckdb`), including the sibling-repo
  post-commit-hook caveat. DuckDB stays compiled in for this slice —
  `migrate-store`'s source side needs it — removed once the operator's
  fleet store has migrated (SS-8, deferred, tracked in
  `docs/plans/store-sqlite.md`).
- Added dependency: `modernc.org/sqlite v1.52.0` (pure Go; pin matches
  throughline's known-good version). `go.mod`/`go.sum` updated; the
  `third_party/agentic-kit` cold-clone property (`GOWORK=off go build ./...`)
  and `task vendor-check` are unaffected (the kit mirror itself didn't
  change).

### Fixed — language truth: one registry, binary detection, honest CUE (ADR-021 §1, docs/plans/lang-truth.md)
- **One ext→language registry.** `pkg/client.langFromPath` — a stale
  10-language table maintained separately from the extractor registry (25
  distinct language tags across 45 extension/basename keys) — is deleted. `internal/connector.extClassMap` now pairs each registered
  extension/basename with both its `SourceKind` and its language tag in one
  place; `connector.LanguageForPath` is the single source of truth
  `Client.Sources` consumes, and `internal/fusion`'s new
  `TestLanguageRegistryParity` asserts a bijection against `fusion.New`'s
  extractor registrations — a new extractor literally cannot be added
  without also declaring its language (and vice versa). `cloc` now reports
  all registered languages plus an explicit `unclassified` bucket for
  anything with no registered extractor — never a silent `other`. (Three
  lockfile heuristics with no extractor behind them — `go.sum`,
  `Gemfile.lock`, `Cargo.lock`, `requirements.txt` — now honestly report
  `unclassified` instead of a hand-guessed language;
  `package-lock.json` still resolves via its registered `.json` extension.)
- **Binary detection.** `connector.IsBinary` sniffs for a NUL byte in a
  file's first 8000 bytes (precedent: agentic-kit's `constitution-lint`
  `looksBinary`). Binary content is excluded from extraction (`fusion`'s
  `extractAndStore`) and from cloc's line counts; the `cloc` MCP tool
  reports binaries in a separate `binary: {files, bytes}` bucket. Fixes the
  423K-line "other" bucket that was actually an untracked 103MB `fathom`
  build binary at repo root; `.fathomignore` now also excludes `/fathom`.
- **CUE extraction v2.** `internal/extractor/cueextract.go` now recurses
  struct literals to emit nested fields with dotted names (e.g.
  `#Layer.budgets.max_usd_per_run`, depth-capped at 6), reads `Package`
  from the file's actual `package` clause (`ast.File.PackageName`, filename
  fallback only when absent — fixing `internal/config/config_schema.cue`
  and `schema/config/schema.cue`, both `package config`, previously landing
  as two different wrong packages), and captures imports as
  `core.KindCUEImport` reference observations. Golden-tested against the
  real `schema/semantic/schema.cue` `#Layer` field tree.
  `fusion.ExtractorEpoch` is bumped `"2"`→`"3"` so already-indexed `.cue`
  files are re-extracted under the new semantics. **Blast radius: the epoch
  salt is global** — it invalidates every stored fingerprint of every file
  in every service, so the FIRST ingest of each service after this change
  re-extracts the entire corpus (a one-time full re-index per service, on
  the order of a cold ingest), not the usual ~0.2s hash-gated refresh.
  Subsequent ingests return to normal speed.
- **Root-anchored ignore patterns.** `internal/filter` previously treated a
  leading `/` as literal pattern text that could never match a normalised
  path — every leading-slash `.fathomignore` entry was a silent no-op. A
  leading `/` now strips and root-anchors the pattern (gitignore
  semantics): `/fathom` excludes the root-level build artifact from
  discovery entirely without swallowing `cmd/fathom/`.
- **Scan tools skip binary files.** The source scanners (`secret_scan`,
  `shell_out`, `http_client_calls`, `antipatterns`, `deprecation`,
  `env_keys`, `markers`/`test_debt`) no longer regex-scan
  binary-classified content.
- **CUE malformed-import crash.** A mid-edit `.cue` save containing a
  malformed import (e.g. `import 42`) produced a partial AST whose empty
  import path made `literal.Unquote` panic, crashing `fathom ingest` and
  the stdio server; import capture now guards nil/empty/non-string paths.

### Added — build fathom without agentic-kit access (ADR-019, docs/distribution.md)
- **`third_party/agentic-kit/`** is a committed, full mirror of the private
  `github.com/geoffmilleraz/agentic-kit` tree at exactly the commit
  `go.mod`'s pseudo-version pins (`git -C ../agentic-kit archive <commit> |
  tar -x`, whole tree minus `.git`). `go.mod`'s `replace` now points there
  instead of `../agentic-kit`; `go.work`'s `use ../agentic-kit` still wins
  for local co-development (unchanged dev workflow), and `GOWORK=off` falls
  through to the in-tree mirror — proven both ways with `go list -m -f
  '{{.Dir}}' github.com/geoffmilleraz/agentic-kit`, and end-to-end with
  `GOWORK=off go build ./...` and `GOWORK=off go test ./internal/config/
  ./internal/verify/`, all green from a working tree with no sibling
  checkout involved in the resolution.
- **`task sync-kit`** (`scripts/sync-kit.sh`) re-archives `../agentic-kit`
  at the pinned commit into `third_party/agentic-kit`; refuses if the
  sibling lacks that commit. Run it (and commit the result) on every kit
  pin bump.
- **`task vendor-check`** (`scripts/vendor-check.sh`), now a `task
  validate` dependency and therefore part of the pre-push lefthook gate: a
  drift guard that content-hashes (sha256 per file, path-keyed — not raw
  tar bytes, which `git archive`'s embedded commit timestamp would make
  disagree even for identical content) `third_party/agentic-kit` against a
  fresh archive of the pinned commit, and fails if they differ. Skips
  gracefully (exit 0, warning) when `../agentic-kit` isn't present or
  lacks the pinned commit — the expected case for a kit-free consumer.
  Covered by `scripts/vendor-check_test.sh` (5 cases: missing dir, fresh
  match, tampered mismatch, absent sibling, sibling-missing-commit — all
  in a throwaway temp layout, no mutation of the real repo or sibling).
- **`third_party/`** is excluded from `task fmt-check` / `task cue-vet`'s
  file scans (it's upstream's code, upstream's formatting — `vendor-check`
  owns drift detection there, not gofmt).
- **Air-gapped builds:** `go mod vendor` now succeeds standalone (the one
  module needing private-repo access resolves in-tree); the resulting
  `vendor/` (~565MB, mostly DuckDB static libs) is gitignored — a
  consumer's local artifact, never committed.
- Docs: new `docs/distribution.md` (how to build without kit access, the
  sync-kit ritual, the air-gapped path); README gets a Quickstart note +
  a docs-table pointer; ADR-019 records the decision.
- **Size note:** the ground assumption going in was ~716KB for the kit's
  tree; the pinned commit actually archives to ~13MB (1,153 files),
  including one ~4.1MB Mach-O binary (`kit-new`) that appears to be an
  accidental commit upstream (a `docs:` commit, unrelated to the binary).
  Archived faithfully per "exclude nothing else" — exactness is what makes
  the content-hash drift guard meaningful — but flagged here for the
  agentic-kit maintainer to prune in a future commit.

### Removed — sextant back-compat surface (owner decision 2026-07-17: no back-compat wanted)
- **Pre-rename back-compat is gone, not deprecated.** The rename shipped
  2026-07-07 (ADR-015) with several shims so a mid-migration fleet wouldn't
  break; the owner overrode that "kept by design" decision and asked for all
  of it out. Removed:
  - `cmd/fathom/serviceident.go`'s `withNameAliases` — the `sextant`↔`fathom`
    service-name aliasing that `session-hook`, `edit-hook`, and `stop-hook`
    used to resolve either brand era against the daemon's index. Each hook
    now looks up only the candidate's own (lower-cased) name; the double
    loop in all three hooks collapses to a single lookup.
  - `internal/connector.LegacyIgnoreFileName` (`.sextantignore`) and its
    fallback read in `loadIgnoreFile` — only `.fathomignore` is honoured
    now. `TestLocalConnector_Discover_LegacyIgnoreFallback` flips to
    `TestLocalConnector_Discover_NoLegacyIgnoreFallback`, asserting a
    `.sextantignore`-only repo now gets zero excludes from it (the file is
    just an ordinary discovered file to `Discover`).
  - Migration prose pointing users from `sextant session-hook` to `fathom
    session-hook` (`docs/use-cases/getting-started.md`).
  - `docs/rename-migration.md` is marked historical/superseded at the top
    (its back-compat bullet list is no longer accurate) rather than
    rewritten — it's a record of what shipped 2026-07-07, not living
    guidance.
  **Behavioral consequence:** stores ingested under the service name
  `"sextant"` and repos relying on `.sextantignore` are no longer
  recognized — re-ingest under the real service name, or rename the ignore
  file to `.fathomignore`. The fleet's own store already uses `"fathom"`
  post `reingest-fleet.sh`, so the dogfood machine is unaffected.
  `scripts/fathom-adoption.sh`'s `mcp__sextant__`-prefix counting is
  intentionally untouched — it is retrospective measurement over pre-rename
  session transcripts that still exist on disk under the old directory
  name, not a runtime back-compat code path.

### Added — layer catalog: prompt-as-data, pack shape, adr-rules + spec-bindings, `fathom layers` (docs/plans/layer-catalog.md)
- **Prompts are data, not Go (LC-1).** Every `#Layer` instance now carries
  `prompt_file`; `promptForLayer` is generic over `LayerDef` instead of a
  Go switch statement per layer name — the silent obligations fallback
  for an unwired-but-manifest-valid layer name (a correctness bug) is
  gone. Zero identity rotation: `obligations` stays `obligations/c02b1aa1`,
  pinned by test.
- **`serving.drift_tracked` replaces the `kind=="finding"` hardcode
  (LC-2)** at all three remediation-drift sites (compile-side scope-fp
  writing, and both serve-side `code_drifted` sites) — a schema field, not
  a Go type check, so a future layer opts into drift tracking by setting
  one CUE field. `findings` sets it `true`; `obligations` stays at the
  schema default (`false`).
- **Built-ins migrate to the vision's pack shape (LC-4):**
  `schema/semantic/layers/<name>/{layer.cue,prompt.md,eval/,KILL.md}`,
  mirrored (`layer.cue`+`prompt.md` only) into `internal/semantic/layers/`
  and `go:embed`'d — `schema.cue` now holds only the `#Layer` definition.
- **The pack loader unions embedded packs with a project's own
  `.fathom/layers/` (LC-3)** — presence = enabled (ADR-018 reconciler
  doctrine, no separate enabled/disabled state anywhere), a project pack
  overriding an embedded one of the same name. `pkg/client.CompileLayer`
  resolves through this union rooted at the service's own ingested path,
  so a project-local pack compiles and serves exactly like a built-in one
  (`TestClient_CompileLayer_ProjectLocalPack`).
- **`fathom layers list|show|install`** (LC-7): store-direct like `fathom
  claim`'s v0 convention. `install` validates a candidate pack against
  `#Layer` (missing `prompt.md`/empty `eval/` warn, don't fail) and copies
  it into `.fathom/layers/<name>/` — users never hand-author CUE.
- **Two new embedded packs, both cleared the precision/recall shipping
  gate LIVE against `FATHOM_LLM_PROVIDER=claude-cli`** (docs/design/
  semantic-layers.md §4.6; no API key needed): `adr-rules` (the layer-2
  proof — architectural constraints from `ADR.md`/`docs/design/*.md`,
  precision 1.00 / recall 1.00) and `spec-bindings` (the SDD pack —
  gated-plan task bindings from `docs/plans/**` excluding `README.md`,
  scoped to each task's `package:` annotation, precision 1.00 / recall
  1.00 on the first attempt). Results in each pack's `eval/RESULTS.md`.
- **Third-layer end-to-end proof (LC-8):** a fixture pack nobody wrote any
  Go for compiles and serves — including remediation-drift tracking for a
  PROJECT-local pack, which surfaced and fixed a real gap: drift-tracking
  resolution now consults the same embedded ∪ project union a compile saw
  (via `ServiceMeta.Path`), not an embedded-only lookup.
- **Fixed:** `#Layer.corpus.exclude` was decoded from the manifest but
  never wired into any `SelectCorpus` call — `spec-bindings`' corpus is
  the first to need it. `SelectCorpus` now takes an independent exclude
  matcher.
- Docs: `docs/layers.md` is the new catalog reference (per-pack when-to-use,
  corpus, cost, gate metrics); `docs/mcp-tools.md`'s `compile_layer` row and
  `docs/use-cases/coding-agents.md`/`change-review.md` updated for the new
  verb and packs.
- **Pre-merge hardening (critic review of the slice):**
  - A broken project pack no longer takes the whole union down: a
    half-authored `.fathom/layers/<name>/` is skipped (embedded packs —
    including one it would have overridden — keep serving), warned once on
    stderr, and surfaced by `fathom layers list` via `LayerInfo.LoadError`;
    only requesting it by name errors. Drift tracking for embedded
    `findings` provably survives an invalid sibling pack
    (`TestDriftedFindings_SurvivesBrokenProjectPack`).
  - Honest uninstall semantics: pack presence governs COMPILATION; serving
    derives from the STORE. `layers list --store --service` gains an
    orphaned-layers section (claims served with no defining pack); a real
    `layers uninstall` that retires claims is deferred as LC-13.
  - `fathom compile`'s flag-parse-time layer check is advisory-only now
    (warn + pass through): a hard CWD-rooted rejection false-negatived
    cross-repo project layers (`--service B --layer <B's pack>` from repo
    A) — the backend's service-rooted resolution is the only rejection
    (`TestCompile_AcceptsCrossRepoProjectLayer`).
  - Install hygiene: `layers install` stages into a hidden temp sibling and
    atomically renames (a mid-copy failure leaves no half-written pack);
    reinstalling refuses without `--force`; `--force` replaces the whole
    directory, never merges.
  - Eval evidence hardening: fixture sets are explicit
    (`labels.json`.`docs`, never a glob — `RESULTS.md` can't leak into the
    corpus); both new packs' fixtures expanded to the obligations
    precedent (adr-rules 7 rules + 2 traps, spec-bindings 6 rules + 2
    traps) and RE-GATED at 1.00/1.00 twice each under the PINNED production
    compile model (`claude-haiku-4-5-20251001`, claude-cli lane), recorded
    with model+lane+date+command in each `eval/RESULTS.md`. The obligations
    layer re-run under that same pinned model recorded precision 1.00 but
    recall straddling its bar (0.71/0.86) — a pre-OP-6 signal, recorded in
    `schema/semantic/layers/obligations/eval/RESULTS.md` and escalated.
  - `layerIsDriftTracked` memoized per `ObligationsFor` call (one union
    load per distinct layer, not per served rule, on the edit-hook path).

### Added — configuration experience: profiles, ~/.fathom/config.toml, health, smoke (docs/plans/config-experience.md)
- **`internal/config`**: TOML-authored, CUE-validated (`schema/config/schema.cue`,
  embedded byte-identical with a drift test — the `internal/semantic/manifest.go`
  precedent) runtime configuration. `Load` merges a user layer
  (`~/.fathom/config.toml`) and a project layer (`.fathom/config.toml`)
  field-by-field — a partial project-layer override never blanks out fields
  it didn't mention — and tracks per-field `Provenance` (which layer/env-var
  supplied every value). `ResolveProfile` selects a named profile
  (`--profile`, else `default_profile`) and layers the **env-overlay**
  (`FATHOM_LLM_PROVIDER`, `FATHOM_COMPILE_MODEL`, …) on top, so fathom's
  pre-profile env-only behavior is reproduced byte-for-byte when no config
  file exists. **Raw secrets are unrepresentable**: `#Profile` is a closed
  CUE struct with no `api_key` field — `api_key_env` (read at LLM-client
  construction time, never at load) or `api_key_cmd` (argv-exec, no shell,
  10s bound, trimmed stdout, output never logged or embedded in errors) are
  the only credential paths.
- **`compile --profile`**: a new `cmd/fathom/llmprofile.go` construction
  path (`buildLLMClientForProfile` + six `build*ClientProfile` functions)
  sits ALONGSIDE `compile.go`'s original env-only `buildLLMClient` and its
  six `build*Client` functions, which are byte-for-byte untouched — all 15
  pre-existing `compile_test.go` provider tests pass unmodified, source and
  expectations both. `compileModelForProfile` adds the profile as a
  precedence tier between `FATHOM_COMPILE_MODEL` (still wins) and the
  cheap-tier default; `compileModel()` itself is unchanged.
- **`fathom config [--explain] [--json] [--profile X]`**: prints the
  effective configuration. `--explain` is the flagship feature — every key
  annotated with its source (`flag`/`env:VAR`/`project:path`/`user:path`/
  `default`).
- **`fathom providers [--profile X] [--json]`**: auth-health probe —
  `map[string]llmclient.Pinger` built from every configured profile (or just
  one), one `llmclient.ProbeAll` call (zero/near-zero tokens). A profile
  whose adapter doesn't implement `Pinger` (a type-assertion, not a
  provider-name allowlist) is reported `not probeable`, never silently
  dropped. Non-zero exit on any probed or construction failure.
- **`fathom smoke --profile X|--all [--json]`**: one tiny real completion
  per selected profile — model, wall-clock latency, tokens, and an estimated
  cost (the same flat per-KTok rates every `compile` run's `cost_usd`
  already uses). Spends real tokens, so `--profile`/`--all` is required
  explicitly; no implicit "smoke everything" default. No live calls in
  tests — fake shims / `httptest`, mirroring `compile_test.go`'s patterns.
- **`serve-http`** loads the same config at startup: `[daemon]`
  (`store`/`addr`/`call_log`/`auto_refresh`) feeds flag *defaults* (a flag
  on the invocation still wins), and `[daemon].default_profile` (falling
  back to the top-level `default_profile`) selects `compile_layer`'s
  provider — closing the gap where the launchd plist carries no env at all
  and `compile_layer` was silently disabled on the always-on daemon.
  Read-once: a config change reaches the daemon only after a restart, like
  every other daemon knob.
- **`fathom init`** gains a write-if-absent step scaffolding
  `~/.fathom/config.toml` with every profile shape (subscription, API-key,
  AWS GovCloud, Azure Government) commented out and the secrets doctrine
  stated inline; never overwrites an existing file.
- **AWS GovCloud / Azure Government** are profiles, not modes: `region =
  "us-gov-west-1"` or an explicit FIPS `endpoint` for Bedrock; `endpoint =
  "https://<resource>.openai.azure.us"` for Azure OpenAI. Hermetic
  construct-level proofs (no network) that a profile's region/endpoint
  actually reaches the kit adapter's `Config`, not just that fathom's own
  pre-checks pass.
- Docs: `docs/configuration.md` is the new canonical guide (precedence,
  profiles, GovCloud/Government recipes, `config`/`providers`/`smoke`,
  secrets doctrine, daemon restart-to-apply); pointers from `README.md` and
  `docs/use-cases/coding-agents.md` §5 (the provider-matrix table stays the
  single source, linked rather than duplicated).
- Repurposed the pre-existing (unused, zero-importer) `internal/config`
  package rather than colliding names elsewhere — its old JSON
  query-profile API (`coding`/`investigation`/`ci`/`platform`) predated this
  feature and had no callers anywhere in the tree.
- Review hardening (coordinator critique of the first landing): the two
  pre-existing serve-http tests that reach the new config load now isolate
  HOME/cwd (they would have started binding real `[daemon].addr` ports the
  day an operator adopts `[daemon]`); a modelless bedrock **profile**
  fails fast naming the requirement instead of silently constructing
  around the Anthropic-API default model id (which passes `providers`'
  Ping but breaks the first compile opaquely) — plus a table-driven
  equivalence test (`llmprofile_equivalence_test.go`) that drives all six
  lanes through BOTH the frozen env-only builders and the profile builders
  from the same environment and fails on any construct-vs-error or routing
  divergence; `compile --profile` without `--store` is now an error naming
  the constraint (HTTP mode compiles with the daemon's own configured
  profile) instead of silently ignoring the flag; TOML parse errors carry
  line/column; CUE validation errors print each offending key once
  (grouped per path); the daemon's fail-to-start on a malformed config is
  now tested and documented (pre-flight with `fathom config`); the
  `providers` "not probeable" branch is tested end-to-end through a
  builder seam (`providerClientBuilder`).

### Added — provider matrix: openai, azure-openai, bedrock, codex-cli (docs/plans/provider-matrix.md)
- **Four more `FATHOM_LLM_PROVIDER` lanes**, rounding `buildLLMClient`
  (`cmd/fathom/compile.go`) out to six: `openai` (`buildOpenAIClient` —
  vanilla OpenAI Chat Completions via `agentic-kit/foundations/llmclient/openai`,
  `OPENAI_API_KEY` optional for local/compatible hosts, `FATHOM_OPENAI_BASE_URL`
  override for Ollama/vLLM), `azure-openai` (`buildAzureOpenAIClient` — the
  same adapter's Azure mode; `AZURE_OPENAI_ENDPOINT` and
  `AZURE_OPENAI_DEPLOYMENT` are checked by fathom itself, before calling the
  adapter, so a missing one names the env var an operator actually sets
  rather than the adapter's Go `Config` field), `bedrock`
  (`buildBedrockClient` — AWS Bedrock via `agentic-kit/foundations/llmclient/bedrock`'s
  full credential-chain resolver: explicit → env → shared files including
  `aws sso login` sessions and `credential_process` → IMDSv2, expiry-aware
  refresh, no static keys required from fathom; `FATHOM_COMPILE_MODEL` is
  fathom's own requirement — Bedrock has no meaningful default model, so a
  missing value fails construction naming the variable), and `codex-cli`
  (`buildCodexCLIClient` — mirrors `claude-cli` via
  `agentic-kit/foundations/llmclient/codexcli`, `FATHOM_CODEX_BIN` override;
  **also Pings the constructed client** before returning it, since
  codexcli's `Request.System` mapping and ambient-context suppression ride
  undocumented `codex-cli` config keys a future CLI upgrade could silently
  drop — a failed Ping is a construction error naming the silent-degradation
  risk, so fathom never hands back a client that could silently lose its
  system prompt and revert to the CLI's ~13k-token built-in persona).
  The unknown-provider error now lists all six lanes.
  Follow-up in the same landing: CLI `--store` mode no longer hardcodes the
  Anthropic model string into `WithLLM` — both construction sites share
  `compileModel()` (`FATHOM_COMPILE_MODEL`, cheap-tier default), because
  `Request.Model` wins over any adapter's `DefaultModel` when non-empty and
  the hardcoded string would have silently overridden every non-Anthropic
  lane in store-direct compiles.
  Docs: `docs/use-cases/coding-agents.md` §5 gets a provider matrix table
  (lane, auth mode, env vars, marginal-cost story) including the honest
  `codex-cli` cost line — roughly 10k input tokens of ambient overhead per
  call (~300k for a 31-section compile) against ChatGPT plan limits, ~60%
  cache-discounted steady-state, versus `claude-cli`'s near-zero ambient
  overhead under `--safe-mode`.
  go.mod's `agentic-kit` require bumped to the pseudo-version for the
  commit landing all three kit slices (aws-credchain, azure-openai,
  codexcli-adapter). Tests: `cmd/fathom/compile_test.go` — construct-level
  coverage for `openai`/`azure-openai`/`bedrock` (env fixtures via
  `t.Setenv`, including a base-URL/Azure-deployment routing round trip
  against a local `httptest` server), and hermetic fake-shim coverage for
  `codex-cli` covering both the healthy and degraded Ping outcomes. No real
  network/API/CLI-login calls in any of it.

### Added — subscription-backed compile provider (claudecli)
- **`FATHOM_LLM_PROVIDER=claude-cli`.** `cmd/fathom/compile.go`'s single
  construction site (`buildAnthropicClient`, called by both `compile
  --dry-run=false` and `serve-http`'s `compile_layer`) is now
  `buildLLMClient`, which switches on `FATHOM_LLM_PROVIDER`: unset or
  `"anthropic"` keeps today's behavior (needs `ANTHROPIC_API_KEY`);
  `"claude-cli"` shells out to a locally logged-in Claude Code CLI instead
  (via `github.com/geoffmilleraz/agentic-kit/foundations/llmclient/claudecli`,
  new in agentic-kit), so obligations/findings compiles ride a Claude Pro/Max
  subscription's OAuth login instead of a metered key.
  `FATHOM_CLAUDE_BIN` optionally overrides the resolved `claude` binary path
  (e.g. for the `serve-http` launchd daemon, whose PATH may not include it).
  The no-provider-available error now names both options. Reported
  `cost_usd` is the same API-equivalent accounting under either provider;
  actual marginal spend under `claude-cli` is $0 (subscription plan limits,
  not a metered key) — documented in `docs/use-cases/coding-agents.md` §5.
  go.mod's `agentic-kit` require bumped to the pseudo-version pinning the
  commit that added `claudecli` (kit `go.work`+`replace` still couples the
  local sibling checkout for day-to-day dev; the pin matters for a cold
  clone). Tests: `cmd/fathom/compile_test.go` covers `buildLLMClient`'s
  switch (default/explicit anthropic without a key, claude-cli construct +
  one real `Complete` through a hermetic fake-shim, unresolvable
  `FATHOM_CLAUDE_BIN`, an unknown provider value) — the anthropic path's
  (nonexistent) tests are untouched.

### Added — findings-as-claims tracer bullet (docs/plans/findings-tracer.md D3, FT-1..FT-6)
- **Kind:"finding" unlocked end-to-end.** A `findings` `#Layer` instance
  (`schema/semantic/schema.cue`: kind `finding`, corpus `findings/**`, tier
  `sme-stated`) joins `obligations` in the manifest. `internal/semantic.Compile`
  is now layer-aware — it resolves name/kind/tier/tier_profile/prompt from
  `Compiler.Layer` (falling back to the pre-M1 obligations identity when no
  layer is loaded, so every existing direct-Compiler test and the
  expert-module non-git-corpus test are unaffected) — instead of hardcoding
  the obligations identity everywhere. A new findings system prompt
  (`internal/semantic/prompts.go`) shares the obligations claim envelope and
  JSON schema; only the compile instructions differ (FT-1).
- **`pkg/client.CompileLayer(ctx, service, layer, dryRun)`** generalizes
  `CompileObligations` (now a thin wrapper). Fixes finding F7: `cmd/fathom
  compile --layer` and the `compile_layer` MCP tool validated the requested
  layer name via `LoadLayer` and then silently discarded it, always compiling
  "obligations" regardless of what was asked for — both now route the
  validated name all the way through.
- **Kind-aware serving.** `semantic.ObligationsFor` now reads claims across
  every compiled layer (not just `obligations`) and the served `Obligation`
  gains a `kind` field (`json:"kind,omitempty"`, omitted for the obligations
  layer — byte-compatible with every existing consumer) so a finding claim
  scoped to a file is labeled, not silently indistinguishable from an
  obligation (FT-2).
- **`fathom claim refute|corroborate --store DB --service S --claim ID --note
  "..."`** wires the previously-dead `claim_events` machinery
  (`AppendClaimEvent` had zero non-test callers before this): `refute` appends
  a `refuted` event and flips claim status to `ClaimRefuted`; `corroborate`
  appends a `corroborated` event and never changes status or tier —
  corroboration is provenance, not evidence-laundering (FT-3).
- **Refutation durability.** `PutClaims` (both `DuckStore` and `MemoryStore`)
  now retains `ClaimRefuted` status across an upsert of the same `claim_id` —
  a recompile that re-emits an unchanged rule can no longer silently
  resurrect a suppressed finding to the compiler's default fresh status
  (FT-4).
- **`fathom report --store DB --service S --analyzer security_rollup --out
  findings/`** is the deterministic bridge: writes ONE markdown finding
  document (per-category sections — hardcoded secrets, unchecked shell-outs,
  unsafe HTTP client usage — each with `path:line` lists), mirroring
  `internal/mcp/extra_tools.go`'s `security_rollup` shape, so deterministic
  analyzer output flows through the normal ingest→compile→claims pipeline
  instead of a parallel persistence path. Output is a pure function of the
  corpus working tree at run time (`Sources()` re-reads disk, not the index;
  no clock/random ordering in the body), verified by a double-run
  byte-compare — including with `--out` INSIDE the corpus, the intended
  deployment: source files under the resolved `--out` directory are excluded
  from the scan, closing the self-scan feedback loop where run N+1 would
  find shell-out/http snippets embedded in run N's own doc and mutate it
  every run. Secret-category findings are rendered redacted (`path:line` +
  rule only — the scanner messages embed the matched source line, and a
  committable document must never exfiltrate the credential it reports)
  (FT-5).
- **Remediation drift (FT-6).** The compiler now persists, for finding-kind
  claims only, a `ClaimLink{Rel:"scope-fp"}` per scope-matched file with its
  compile-time store fingerprint. `semantic.DriftedFindings` is a new, pure
  read that re-compares those fingerprints against the CURRENT store state;
  `ObligationsFor` calls it and sets `code_drifted:true` on the served
  payload when the scoped code changed since compile. Chosen design: a live
  re-read at serve time rather than a persisted flag — keeps `Sweep` and
  `ObligationsFor` both pure functions of current store state, with no third
  claim-status axis, and matches the vision's explicit rule that code drift
  is a serving annotation, never a status flip (a finding whose code changed
  might be FIXED, not wrong). Anchor persistence is loud, never best-effort:
  a fingerprint read failure (or a fingerprint-less indexed file) at compile
  time aborts the run (`errDriftAnchor`) instead of silently omitting the
  link — the digest gate would never recompile the unchanged section, so a
  missing anchor would permanently blind drift for that file. This upholds
  the invariant DriftedFindings relies on (every compiled finding claim
  carries ≥1 scope-fp link), and serving is bounded: a per-call fingerprint
  cache queries each distinct anchored file at most once per
  `ObligationsFor`, regardless of how many claims share it (the edit-hook
  hot path). Post-merge follow-up closed FT-2's last hop: the edit-hook's
  rendered bullet now labels non-obligation kinds after the tier
  (`[…, sme-stated, finding]`) and appends `code drifted since compile`
  when `code_drifted` is set — obligation bullets are byte-identical to
  before.
- **Findings prompt: no bare `**` scopes.** The findings compile prompt
  instructs the narrowest affected-path globs the document justifies; a
  genuinely repo-wide finding scopes to its affected file class (e.g.
  `**/*.go`), never a bare `**` — aligned with the O8 precision work's
  compile guard. The prompt identity is content-addressed, so this rotates
  only `findings/<hash8>`; no compiled priors existed.

### Added — O8 precision: serve-time trust (docs/plans/o8-precision-and-speclines.md OP-1..OP-5)
- **Prompt v3** (`internal/semantic/prompts.go`): the obligations compile system
  prompt now instructs file-class scoping — a rule binds only the file class its
  governing document addresses (code rules → source globs like `**/*.go`, docs
  rules → markdown globs like `**/*.md`) and a bare `"**"`/repo-wide catch-all
  scope is never allowed. All v2 instructions (binding-rules-only, narrowest
  scope, 28-word rule cap, the `{"claims":[...]}` envelope) are kept. The
  content-addressed `PromptVersion` (TextHash-derived) rotated to
  `obligations/c02b1aa1`, forcing every section to recompile under the new
  prompt rather than silently keeping v2 claims (the v1→v2 hand-bumped-const
  bug class this mechanism exists to prevent).
- **Over-broad-scope guard** (`internal/semantic/compile.go` `verifyDraft`): a
  claim whose scope glob matches more than half of `IndexedFiles` is rejected
  and counted in `ClaimsRejected`, exactly like a failed citation — the
  compile-time backstop for prompt v3's "never `**`" instruction. This makes
  `"**"` always over-broad in a non-empty corpus, so every prior test scope of
  `["**"]` was rewritten to a real narrow glob (`internal/mcp/**` against the
  shared 3-file test fixture). **Exemption (review fix):** single-file-class
  extension globs — a glob whose final segment is `*.<ext>` with one literal
  extension (`**/*.go`, `docs/**/*.md`, `*.pb.go`) — are exempt from the
  breadth check (never from zero-match rejection): they are class-scoped by
  construction, exactly the shape prompt v3 mandates for legitimately
  fleet-wide rules, and in a go-majority corpus `**/*.go` necessarily exceeds
  50% — without the exemption the guard would reject every prompt-compliant
  repo-wide code rule at the OP-6 recompile. Bare `**`, directory globs
  (`internal/**`), and multi-extension patterns matching >50% stay rejected.
- **Edit-hook root refusal** (`cmd/fathom/edithook.go` `cmdEditHook`): a file
  that does not relativize under ANY resolved service-candidate root now skips
  that candidate outright instead of falling back to querying the daemon with
  the raw out-of-repo path — the O8 addendum (obligations fired for a file
  outside every indexed repo) goes silent. **Review fixes:** the check is not
  purely lexical — `relativeToRoot` retries with both sides
  `filepath.EvalSymlinks`-resolved (falling back to the lexical path on error),
  so symlinked layouts (`~/code` → `/Volumes/…/code`, macOS `/var` →
  `/private/var`) where `gitToplevel` returns the physical root while the
  session addresses files through the symlink don't false-refuse every in-repo
  file; the `..`-escape test is `rel == ".." || strings.HasPrefix(rel, "../")`
  (a root-level `..foo` entry is not an escape); and a relative `file_path` is
  absolutized before the candidate loop so it cannot bypass the refusal.
- **Spec-line injection** (`cmd/fathom/edithook.go`): when the edited file cites
  bracketed spec IDs (`internal/sdd.ExtractRefsFromText`, the same extraction
  `spec_refs` uses), the edit-hook looks up each ID (capped at 3) via the
  `spec_refs` MCP tool and appends `"▸ implements [ID] — defined at
  <file:line>"` for the location OUTSIDE the edited file — literally tying the
  cited spec to where it's defined, in the channel that already works
  (docs/vision/04 §4 item 3). **Review fix:** spec lines fire independently of
  obligations — a file citing a spec ID with zero matching rules (the common
  case post-recompile, with prompt v3's narrower scopes) gets a
  spec-lines-only payload under its own `⚖️ fathom spec refs` header;
  zero-output-on-clean is preserved (no rules AND no spec IDs → silent).
  Shares the obligations payload's ≤150-token budget; spec lines are dropped
  first (never truncating obligations), and when obligations alone already
  spend the budget every spec_refs RPC is skipped up front. Silent on any
  error (unreadable file, daemon down, etc.) per hook doctrine.
- **`plan` project skill** (`.claude/skills/plan/SKILL.md`): scaffolds a
  gate-ready grammar-v2 plan (stable `id`s, `{symbol, package}` contracts
  grounded via `pack_context`/`test_impact`, typed `disposition`s for skips,
  `skip_test` always paired with `skip:`, spec-ID brackets, the `fathom verify`
  command to run before checking any box) so new plans are born gate-ready
  instead of needing an honesty rewrite after the fact.

### Added — coding-agents canonical doc + staleness sweep (IA-4/IA-5)
- docs/use-cases/coding-agents.md is THE adoption guide (the loop, hooks,
  gates, CLAUDE.md/AGENTS.md doctrine, agent-surface registration, the honest
  cost story); usage.md set_profile guidance retired; the June
  claude-code-integration.md banner-superseded (assessment content kept);
  getting-started/coding-context scoped and pointed at the canonical doc.

### Added — fathom init: one-command coding-agent wiring (docs/plans/init-and-adoption.md IA-1/IA-2/IA-3)
- `fathom init` wires a repo end-to-end, idempotently and additive-only: creates
  `.fathomignore` if absent, calls the daemon's `ingest` MCP tool and confirms
  the service is indexed (fail-soft exit 2 with startup instructions when the
  daemon is unreachable or the service still has no symbols), JSON-merges
  project-level `.claude/settings.json` hooks (SessionStart `session-hook`,
  PostToolUse `edit-hook`, Stop `stop-hook`), writes the canonical fathom
  operating-contract stanza into both CLAUDE.md and AGENTS.md between
  `<!-- fathom:begin/end -->` markers, and prints the manual steps it cannot
  safely automate (MCP registration one-liner, lefthook pre-push verify
  snippet). Every mutation is create-if-absent or marker-replace, so a second
  run changes zero bytes and reports "already present" throughout.
- `mergeHookSettings` preserves every unrelated top-level key, hook event, and
  sibling hook entry in an existing `settings.json`; detection is by command
  substring (`"fathom session-hook"` etc.), so re-running never duplicates an
  entry regardless of the exact binary path baked in.

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

### Added — M1: the manifest goes live (semantic layers, ADR-016 amendment)
- **`schema/semantic/schema.cue` is now a runtime artifact, not documentation.**
  `internal/semantic.LoadLayer` embeds a package-local, byte-identity-tested
  copy of the schema (`internal/semantic/manifest_schema.cue`,
  `TestManifestSchemaMatchesSource`) and decodes `#Layer` instances (corpus
  globs, tier, budgets, serving caps) via `cuelang.org/go`. The obligations
  layer's corpus include list and per-claim rule-length budget now come from
  the manifest instead of the hand-synced `ObligationsInclude` Go var and a
  bare `maxRuleWords` const — both retired (grep-clean under
  `internal/semantic`). `cmd/fathom compile` and the `compile_layer` MCP tool
  accept any manifest-listed layer (today still exactly `obligations`) and
  reject unknown names with the manifest's live layer list instead of a
  hardcoded string check.
- **Content-addressed prompt identity** (agentic-kit v0.7.0
  `prompttemplate`). The obligations system prompt moved into a
  `prompttemplate.Template` (`internal/semantic/prompts.go`); `PromptVersion`
  is no longer a hand-bumped const — `obligationsPromptVersion()` derives it
  as `obligations/<hash8>` from `Template.TextHash()`. The digest gate
  (`allDigestsMatch`) keys on this derived identity, so any edit to the
  prompt text is now automatically a new compile identity — the exact bug
  class that silently left 27/32 sections on stale v1 claims after the
  v1→v2 bump cannot recur.
  **This is a flag-day recompile.** The prompt text did not change in this
  slice, but its identity's FORM did (hash-derived vs. the retired
  `obligations/v2` const), so the very next real compile will see every
  existing claim's `PromptVersion` mismatch and recompile the whole corpus
  once. No compile was run as part of this change (no API keys, no LLM
  calls — mocks only); sequence the actual fleet recompile deliberately —
  ideally folded with the O8 serve-time precision prompt revision so the
  fleet pays one recompile, not two — and never mid-measurement-window.
- **Typed compile failures** (agentic-kit `llmclient/structured`). Section
  completion now goes through `structured.Complete`, which sets
  `Request.OutputSchema` to the claim-draft JSON Schema and classifies
  truncation/malformed model output as `structured.ErrTruncated` /
  `structured.ErrMalformed` — the hand-rolled `errBadResponse` classifier and
  its fenced-code-block-stripping retire. Skip-section policy
  (`sections_failed++`) is unchanged; transport errors still abort the run.
- **Run-level budget enforcement** (`enforceBudgets`, ADR-016 §4.1). A real
  compile now refuses to start when its pre-flight, digest-gate-aware cost
  estimate exceeds the layer's `max_usd_per_run` (typed refusal, still
  ledgered), and stops cleanly mid-corpus when the running token total
  crosses `max_tokens_per_run` — sections not yet reached are left untouched
  and resume via the ordinary digest gate on the next invocation. Both cases
  are recorded on the existing `compile_runs` ledger, additively
  (`core.CompileRun.Refused` / `RefusalReason` / `BudgetStopped`; new
  columns on the DuckDB backend, migrated idempotently).
- **`#Layer.tier` widens to admit `verified`** for deterministic,
  Model-empty compile paths (ADR-018 oracle classes: `index`/`cmd`/`lexical`)
  — amendment recorded under ADR-016 in `ADR.md`. No layer emits it yet;
  `ai-derived` stays unadmitted, still awaiting its own revival evidence.
- Spec: `docs/plans/m1-manifest.md` (M1-1..M1-8, all landed).
- **Adversarial-review fixes (same slice, pre-recompile):**
  (F1) the structured-outputs schema is now OBJECT-rooted
  (`{"claims":[...]}`) with `additionalProperties:false` on every object —
  Anthropic structured outputs reject array roots / open objects with a 400,
  which is transport-class and would have aborted every real compile at
  section 1; the system prompt and decode target moved to the envelope shape
  (`TestClaimDraftSchema_AnthropicStructuredOutputCompatible` pins both wire
  constraints). This prompt/schema change MUST land before the flag-day
  recompile — the TextHash identity covers the prompt text, not the schema,
  so sequencing is the protection.
  (F2) superseded/refuted claims no longer feed the digest gate: one
  superseded row (old digest or old PromptVersion) previously forced its
  subject to re-LLM on every future run, forever
  (`TestCompile_SupersededClaimsDoNotDefeatDigestGate`).
  (F5) a budget-stopped run is no longer indistinguishable from success at
  the operator surfaces: `compile_layer` returns
  `budget_stopped`/`refusal_reason` (and `refused` symmetrically) and the
  CLI printers render them.

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
