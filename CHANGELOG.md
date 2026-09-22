# Changelog

All notable changes to fathom. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); dates are ISO-8601.

**How to read this file.** Each release opens with what it was for, then
one block per day: a bold topic naming the program and its plan rows
(`SH-1..6`, `O16`, `ADR-025` are durable reference points -- `fathom plans
archive` checks that every archived row id is absorbed here, in `ADR.md`, or
in the per-release narrative), followed by what changed. The full narrative
record -- root causes, measurements, rationale as originally written -- lives
per release under `docs/changelog/` and is never normative; plans, ADRs, and
observations are.
From 2026-08-25 the `[Unreleased]` block follows the change-records rules
(`agentic-kit/docs/project-intent/changes.md`): a `Release impact:` line first,
one line per observable change at most two sentences, `### Breaking` first
when present, the delta not the journey; released blocks stay as written.
Compatibility surface: `docs/intent/scope.md`.

## [Unreleased]

Release impact: minor (pre-1.0: breaking bumps minor)

**The agent-surface `tools/list` schema tax is cut 62.7% (O101, TD-2).** Bench evidence (lookups-ts/-py) measured a median 15,219 cache-read tokens/turn on the wired arm vs 5,165 for grep — a ~10k-token fixed tax paid every turn, traced to the 48,341-byte `tools/list` payload served on `POST /rpc/agent`. `outputSchema` (RP-11, 18,827 B, 39% of the payload) is now off `tools/list` by default — still generated from the Go result-type registry, still returned as `structuredContent` on every `tools/call`, and still requestable on `tools/list` via `?contracts=true` on the HTTP surface (`internal/mcp`'s `WithContracts`) for a client that validates envelopes. Every agent-surface tool `Description` is capped at 260 bytes and every `inputSchema` property description at 120 bytes, flat (no more 700/1000-byte tiers); alias/history prose moved to `docs/mcp-tools.md` where it was not already documented there (the aliases themselves are unchanged). Measured: 48,341 → 18,008 bytes. Docs: `docs/mcp-tools.md`.

**Bug reports route to `localize` (O102).** The SessionStart brief and the stanza's opening moves now send a bug report, stack trace, error string or failing test to `localize` first — one bounded, logged walk of the index before any grep; the first LCA round showed the verb was never reached for from an issue-shaped prompt.

**The decision port, off by default (vision 32, ADR-042).** `[decide] provider = none | llm | typesafe` in `.fathom/config.toml`; with `none` nothing is called and nothing changes. With `llm`, the daemon asks one batched typed question per site — is this guide section a binding rule (the compile pre-filter, skipped sections counted, disclosed and still served raw), does this rule govern this edit (obligations re-ranked inside a governing-scope tier only; envelope `ranked_by`, per-rule `relevance`), which verb does this prompt want (`POST /route`; the prompt hook injects one firm route line at the proceed band, one hedged line at confirm, nothing below that or on any failure — the port never asserts a code fact, only the prompt's shape). Every answered decision is logged to `decisions.jsonl` beside the configured call log; a doctor row names the provider. `typesafe` (2026-09-22, ADR-042 amendment) routes the same three question sites through the kit's TypeSafe (Jev) adapter directly — `endpoint`/`model`/`api_key_env`/`api_key_cmd` configure it, the key resolved at call time (cmd wins over env, unset defers to `TYPESAFE_API_KEY`), never a literal in config; both `llm` and `typesafe` fail open to `none` with one warn line on a missing key, an unreachable endpoint, or any other open failure (I14), and `fathom doctor`'s `decision_provider` row names what failed. Docs: `docs/mcp-tools.md`, `docs/configuration.md`.

**A refreshed file reaches the next graph query (SU-4, O99).** POST /refresh-file now patches the daemon's memoized syntax+typed call graph for the file's package in place (the memo is keyed by a watermark a single-file re-ingest never moves, so it kept serving the pre-refresh graph) and reports `graph_patched`; the resolved-only graph rebuilds on its next use.

**The push gate verifies the tree it pushes (INC-21).** The pre-push `verify` step ingests HEAD into a scratch store under `workspace/tmp` and proves every claimed plan row against that index, instead of against the daemon's index of the operator's checkout; a failed scratch ingest is a named fail-open line. Running it hermetically exposed that `fathom verify --store` walked a syntax-only call graph while the daemon's `verify_plan` walked syntax plus typed edges, so rows whose reachability crosses a method value, an interface dispatch, or a bare call on a receiver sharing a method name with another type verified on one surface and failed on the other (O40, O98). One graph now: the CLI verify and plans-context paths add the typed edges, and `reachable`, `test_impact`, `dead_code` and `find_callees` walk the same watermark-cached graph `verify_plan` and the nav tools use. A bare `symbol:` that collides with a Markdown heading of the same name is resolved to its code definition rather than handed to the graph as an ambiguous name. `task push` now sends the pack over smart HTTP derived from the push remote (INC-22, I41): git opens the transport before the pre-push hook, and an ssh session does not survive this repository's five-to-nine-minute gate. Both push-gate scripts moved upstream into agentic-kit's fathom-pack (operating.md §5, kit 89fca073f70b) and are consumed from the vendored kit; the pin moved with them.

**Claims gain bi-temporal validity (ADR-041 decision 7, SU-19).** `semantic_claims`
gains additive `valid_from`/`valid_to`/`asserted_by` columns (`claim_events` gains
`asserted_by` too), stamped at mint and closed at every supersede/refute path;
`semantic.ObligationsAsOf` and the `obligations` MCP tool's new `as_of` argument (RFC3339
timestamp or commit sha) answer "what governed this file AT that instant" — including a
claim since superseded or refuted, if `as_of` predates that event — labeled `as_of` in
the response. `fathom findings near --as-of` mirrors it on the CLI. Default (`as_of`
omitted) serving is byte-identical to before this row. Docs: `docs/mcp-tools.md`,
`docs/layers.md`, `docs/usage.md`.

**Claims gain an explicit supersedes chain (ADR-041 decision 7, SU-20).**
`semantic.WriteSupersedes` appends a `supersedes` claim link (never `PutClaims`'
wholesale link-set replace — the new `store.Store.AppendClaimLink` appends one row in
place) from a successor claim to the predecessor it replaces; `semantic.SupersessionChain`
walks it back through however many times a subject has been re-superseded.
Wired at native review reconciliation (`fathom review` and the `review_diff` MCP tool),
via `review.SupersedeSuccessor`'s reverse lookup of this run's OWN re-emitted finding for
the predecessor's subject — the genuine successor case, as opposed to a finding that
simply stopped firing (fixed, no replacement), which still supersedes but links to
nothing. External closure/revocation and `fathom claim refute` close `valid_to` (closing
a gap left in SU-19) but never call `WriteSupersedes`: both retire a claim in place with
no distinct successor row to link to.

**Claims gain symbol-bound anchors (ADR-041 decision 7, SU-21).** When a claim's
citation resolves to exactly one symbol definition in the index, it also gets an
`anchors` claim link (`internal/semantic.AnchorSymbol`, wired into the compiler's own
citation step) — the symbol's `core.SymbolKey` plus its own current content
fingerprint. `Sweep` reads it as a SECOND healing path, only when the existing
literal-text re-anchor (T5-1/O13) is unavailable or fails: the anchored symbol's
CURRENT body hash must still match the one stored at anchor time — proof of a pure
line-range move, never a content edit passed off as one — before the citation is
re-pointed at its new range instead of flipping stale. `rename_impact` gains
`claims_anchored`/`claims_anchored_ids`, so a rename is never silent for governance.

**Claims record who asserted them (ADR-041 decision 7, SU-22).** `core.AssertedBy(kind,
id)` builds the canonical `"kind:id"` form every writer now records at mint: a compile
run's own `run.RunID` (`"compile:<run_id>"`), a review run's `--idempotency-key` or its
corpus watermark (`"review:<key-or-watermark>"`), an import's `producer@version`
(`"import:<producer>@<version>"`), and `fathom claim add`'s harness hook session id
(`"claim-add:<session>"`) via a new `AddClaimRequest.SessionID`. Served as `asserted_by`
on each obligations rule, omitted when empty.

**Vectors removed, not parked (ADR-041 decision 2, SU-8).** `internal/vector`, the
query engine's `WithVector` option, the `SymbolIndexer` interface, `hybridRank`'s
vector branch, and `core.Query.SemanticWt` (its only reader) are deleted, along with
fathom's own import of agentic-kit's embedding client (`embedding-client`,
`vector-store`) — the keyword+tier ranking (`scoreAndRank`) is unchanged, pinned by
the existing tests plus a new ranking-order golden. Not breaking: `WithVector` had no
production caller (`internal/modulehost/federated.go` always called `query.New` bare)
and `SemanticWt` did nothing outside the deleted path. `TestQueryEngineHasNoVectorOption`
refuses any future `*Vector*`/`*Embedding*` declaration in the query engine by AST scan;
revisit only through vision 31 §5's kill criterion. Docs scrubbed so "semantic" means
compiled claims, never vectors: `docs/architecture.md` (Hybrid semantic search section
rewritten), `docs/design/scope-and-purge.md`, `docs/design/semantic-layers.md`,
`docs/intent/futures.md` (F13 superseded). Evidence: `docs/vision/31-substrate.md`.

**`localize` walks the graph (ADR-041 D4, SU-14).** Every seed `localize` resolves now
expands through the typed/heuristic call graph (`expandSeedsByGraph`) to depth 2, ranked
by seed evidence, graph distance, resolved-over-heuristic, then how many distinct seeds
converge on a candidate; when seed extraction finds nothing at all, seeds are reformulated
from identifier splitting, quoted literals and file basenames (never a model call).
Candidates cap at 25 (`candidates_total` plus the standard elision block) and each
candidate's `why` caps at 3 citations, closing the 882,577-byte single payload lca-run-1
hit. Deterministic on identical input. The LCA corpus's `max_turns` (30) is confirmed
current; the Kotlin near-empty-index question from lca-run-1 is recorded as an open note
on the corpus rather than silently dropped.

**Typed edges beyond Go by import (ADR-041 decision 5, SU-9/SU-12/SU-13).** `fathom
ingest --scip PATH` (repeatable, or auto-detected `index.scip` / `.fathom/scip/*.scip`)
imports an external indexer's SCIP output as `typed_edges` — `internal/scip.ImportIndex`
maps definition/reference occurrences using the same package/qualifier convention the
language's own extractor already uses, replaced per fathom package; the ingest summary
names the producer (e.g. `via scip-typescript 0.3.16`). `indexbench scip-oracle`
(`task bench-l0-scip`) is the free-tier L0 oracle: precision/recall/unresolved-rate for
fathom's edges against a SCIP truth, per language and edge kind. `docs/languages.md` is
now kept true by `TestLanguagesTrackerMatchesRegistry` (`internal/connector`), and
`fathom capabilities --json` gains `typed_edge_producers` (native/scip:\<tool\>/file/
syntax per language). No new module dependency: SCIP's wire types come from the
already-vendored `third_party/scip` bindings `fathom export scip` uses.

**Language lookups corpora and push gate (ADR-041 decisions 9, 10; SU-23, SU-24, SU-26).**
`testdata/scenariobench/lookups-ts.json` (nest) and `lookups-py.json` (fastapi) mirror
lookups-vs.json's task classes on non-Go calibration subjects, committed and not run;
`TestLanguageLookupsCorporaParse` checks arms, criterion, and that no prompt names its
own gold file. Every committed corpus (and the LCA external corpus) now passes a pure
`PlanCells` enumeration (`TestAllCorporaPlan`); `docs/bench/README.md` gains a "Next
round" section ordering guards-fathom, findings-loop, lookups-vs, lookups-ts,
lookups-py, and LCA with cell counts and ledger-derived cost estimates. `task push`
refuses to push (`scripts/push-check.sh`, exit 3) unless the lefthook pre-push hook is
present, closing INC-20 (invariant I40).

**Findings-loop learning telemetry (vision 31 §3 D8, ADR-041 decision 8, plan `docs/plans/substrate.md` §D).** `findings candidates` now scores each rule candidate by sites × mean CODEOWNERS-ownership weight of its member files, decayed once per disputed (refuted) member, and caps emission at a configurable cadence with the rest counted `deferred`; every candidate's drafted rule now carries a `promoted_from: <pattern id>, members: N, producers: [...]` provenance tag (`internal/analysis/findings.CandidatePolicy`/`RuleCandidatesWithPolicy`). `findings status` gains `rules_never_fired_30d` and `top_fired`, computed from the edit hook's own beat log (`internal/analysis/findings.RuleTelemetry`) — which currently-served rules have gone quiet and which fire most. New `fathom claim add --store --service --scope GLOB [--scope GLOB ...] --rule TEXT [--remedy TEXT] [--cite path#Lstart-Lend] [--json]` mints one human-authored, content-addressed claim (producer `human.review`, tier capped at sme-stated regardless of any `--tier` value, its own `human-authored/v1` reconciliation domain) served by the pre-edit hook like any finding. `fathom claim refute` gains `--dry-run`: previews `would withhold N served row(s) across N file(s): <first 5 files>` (and the equivalent `{claim_id, would_withhold, files, truncated}` JSON) by walking the claim's own scope globs against the indexed corpus — proven to write nothing.

**Typed edges beyond Go: CUE native, TypeScript/JavaScript/Python file-resolved (ADR-041 decision 5, `docs/plans/substrate.md` §B).** `EnrichCUEEdges` (SU-10, `internal/fusion`) loads every CUE package under a repo with `cue/load`, resolving same-file, cross-file and cross-package `#A: #B & {...}`/import references to typed `core.EdgeDirect` edges (an unsettled reference is `core.EdgeUnresolved`, never dropped) — wired into ingest beside the Go pass, package-granular, under its own `cue:`-prefixed store-key namespace so it can never collide with a Go package's typed-edge rows; `ingest` gains a `cue: N pkg(s) processed (…), N edge(s)` line (`--json`: `cue_edges`). `ResolveFileEdges` (SU-11, `internal/fusion`) adds a per-file import/module-scope resolver for TypeScript/JavaScript and Python when no SCIP index is present for the service, emitting the new `core.EdgeFileResolved` kind — tier pre-scan, always heuristic (`c: "h"`) in `find_callers`/`resolved_nav`/every nav consumer, never presented as typed; `ingest` reports `file_resolved: N` alongside it. `docs/languages.md`'s CUE row moves to native (done); its TypeScript/JavaScript/Python rows mark the file fallback done.

**Substrate hygiene and freshness (ADR-041 decisions 3/4, wave 1 of the substrate round).**
`EnsureIndexes` adds the secondary indexes the obligations claim query and the
find_callers/find_callees typed-edge queries actually use, applied idempotently at every
store Open. `Store.ClaimsForServe` prefilters claims by layer, served status and the edited
file's directory prefixes in the backend (SQL `WHERE` for SQLite, a Go loop for the memory
store) before `obligationsCore` scores them in Go, byte-identical to the prior full-scan
path on every existing golden. Every `tools/call` result now passes through a
dispatch-level byte budget (`budgetedHandler`) so a handler that forgets to budget itself
still cannot exceed it. `CachedCallGraph.InvalidatePackage` patches one package's nodes and
edges into a warm call graph in place instead of forcing a full rebuild (production wiring
at the typed-edge and per-file re-ingest call sites is tracked as follow-up).
`fusion.Pipeline.ReingestFile` re-extracts one file's observations/syntax edges and
fingerprint on request, leaving typed edges package-granular; the daemon's new loopback
`POST /refresh-file` route serves it. The pre-edit hook now refreshes a stale file before
answering obligations (`ensureFileFresh`) instead of only reporting staleness, and the
prompt hook ignores harness-injected system notifications (`isSystemNotification`) so their
prose can no longer be mis-read as repo paths. Evidence: `docs/plans/substrate.md` SU-1..3,
SU-5..7.

**Portable bounded build cache (ADR-039).** `FATHOM_CACHE_ROOT` now relocates the
supported Taskfile/script build caches without embedding a workstation path; configured
missing roots refuse, and `FATHOM_CACHE_MAX_MIB` bounds the regenerable Go build cache.

**App-kit adoption (shell/nav/stat/theme, CSS pipeline).** Observatory pages now render
through the vendored app-kit dashboard shell, pinned to mirror `bcfeb8e`: shared
sidebar/mobile navigation with current-page state, a focusable main landmark and skip
link, kit stat tiles for metric display, kit-native action buttons with 44px touch
targets, and the kit's theme tokens for light/dark, all inside the existing bounded
content width. `task ui:css` (`scripts/ui-css.sh`) builds the CSS from that exact
mirror with `GOWORK=off`, needs no sibling app-kit checkout, and refuses unaudited,
missing, or changed inputs; `bash scripts/ui-css.sh --check` regenerates and compares
the committed output. `appkit.adoption.json` pins the reviewed `shell.layout` metadata,
generated CSS and selected source bytes the adoption checker verifies; the routed
contribution contract in `docs/app-kit.md` keeps fathom-owned display code separate
from reusable app-kit fixes, which flow back to the kit as their own reviewed,
versioned adoption. Evidence: `docs/app-kit.md`.

**Observatory display truth.** Freshness states reserve positive language for complete
evidence, render partial coverage as an explicit lower bound with its reason, and wrap
long diagnostics instead of clipping them. Impact no longer shows an unreadable
response cost as zero or carries over a stale per-tool ratio: call-log read failures
report their exact (escaped) reason, `classifyCallLogRead` preserves the underlying
scanner-error cause and discards incomplete prefix counts, and independent schema-tax
and measured-zero/positive cost figures stay distinct. Fleet, impact, story and home
now distinguish an unavailable observation count from a partial lower bound and from a
complete measured total (including zero); story labels the fleet snapshot separately
from the week, and home still preserves unavailable call-log/catch evidence rather than
filling it in. Missing configuration and unavailable comparable data now show a named
explanation and a link to the evidence behind it, and service share keeps its full
explanation, denominator and snapshot source. `fathom capabilities` and the observatory
now count `.fathom/guard.cue` entries through the guard schema — quoted labels, compact
CUE, comments and note-less guards all report correctly — while a malformed or
unreadable guard file stays an explicit broken input rather than reading as absent.
Evidence: `docs/app-kit.md`.

### Added

- `search` — new agent-surface tool: grep with a receipt (enclosing symbol/kind, test/generated/vendored flags, ident/comment/string classification); excludes vendored and `workspace/` trees by default (RC-24, 19-tool agent surface).
- `fathom discoveries`/`discoveries` MCP tool and `fathom routes` — recent call-log pointers (symbol ids, file paths), resolved fresh against the live index, handed to subagents and the next session (RC-25..33).
- Kotlin extraction (`.kt`/`.kts`): classes/objects/interfaces, functions, properties, imports, same-file call edges.
- `summary` field (a deterministic ≤200-char answer sentence) on nine agent-surface tools: `find_callers`, `references`, `blast_radius`, `rename_impact`, `safe_delete_check`, `test_impact`, `localize`, `search`, `obligations`.
- `#Guard`/`#LayerRule` gain `enforce: bool | *false` (ADR-038): an enforced rule denies a violating Edit/Write/MultiEdit/Bash call outright instead of only advising after the fact; escape hatch `FATHOM_ENFORCE=off` or `.fathom/config.toml`'s `[guards] enforce = false`.
- Codex verified against codex-cli 0.153.1 (hooks and MCP surface confirmed live); `fathom harness hooks --harness ID` prints paste-ready hook JSON for a harness (like Codex) that only loads a user-global hook file.
- Seven advisory hook events (`SubagentStart`, `PostToolUseFailure`, `FileChanged`, `WorktreeCreate`, `CwdChanged`, `DirectoryAdded`, `Setup`), all fail-open; `doctor hooks_coverage` reports wiring against what the detected harness offers.
- `fathom intent render --playbook` writes a root `INTENT.md` inside fathom markers, held to existence by `config audit`'s `pointers_resolve`.
- `fathom compile --max-tokens-per-run`/`--max-usd-per-run` overrides a resolved layer's budget for one compile call (a positive number or `"all"` for unlimited).
- `review_diff`/`fathom review` and `fathom tests coverage` accept one Go coverage profile per test layer (`coverage_profiles: [{layer, path}]` / repeatable `--coverage-profile [layer=]<file>`), reporting per-changed-symbol coverage broken out by layer instead of one blended number.
- `read_source` gains `max_bytes` (default 8KB); `find_symbol` on a Go interface carries `implementations`.
- **The findings loop's governed import lands** (ADR-040, `docs/plans/findings-loop.md` FL-1..FL-14): `fathom findings import --store --service --subject --producer --file [--dry-run] [--json]` (and `pkg/client.ImportFindings`, the identical code path) accepts the review kit's projection under a provisioned `[findings] subject`, deriving one content-addressed claim per finding and mapping its lifecycle onto the existing status/event vocabulary instead of inventing a new one. A producer catalog (`schema/semantic/producers.cue`) caps served tier at `min(event tier, producer cap)` and refuses a `fathom`-origin producer outright, closing the circular-proof loophole. Ordering and idempotency are enforced per claim via a new `external_findings` cursor table (additive on both backends), and native review reconciliation (`StaleReviewClaims`) and the external importer stay provably separate reconciliation domains, discriminated by `PromptVersion`.
- The findings loop (`docs/vision/30-findings-loop.md`, ADR-040): `fathom init` scaffolds a commented `[findings]` section in `.fathom/config.toml`; a new `fathom-findings` skill (import/inspect/refute) and a `fathom-onboard` step document it end to end (`docs/adoption/findings-loop.md`, `docs/configuration.md`). The `fathom findings import`/`fathom findings candidates` CLI verbs it documents are other waves' work (`docs/plans/findings-loop.md` sections A-D) and are not yet present.
- Measurement corpus `testdata/scenariobench/findings-loop.json` (FL-24, `task bench-scenario-findings`): four tasks, each a real gap this repo's own tree carries, anchored to a review-kit finding served (full arm) or withheld (control arm) — committed only, not run.
- Findings served at the point of work (ADR-040, docs/vision/30-findings-loop.md §3.4): the pre/post-edit hook's "open findings on this file" advisory block (capped at 3 findings/1,200 bytes, drifted anchors flagged, superseded/refuted never shown, post-edit gated on the edit actually touching the finding's cited lines); the prompt hook's `open findings: N across the files this prompt names`; the session brief's `open findings: N (oldest …)` (a new plain `GET /findings/summary` daemon route, never an MCP tool); the session hook's outbox sync (`[findings] outbox` in `.fathom/config.toml`, a sha256 digest cursor, one attempt per session); `fathom findings candidates [--min-sites 3] [--json]` (rule candidates derived from `DerivePatterns`, native and external findings alike, with a drafted rule sentence and its ADR-038 check family — fathom never writes the rule); `findings status` gains `candidates` and `disagreements_by_rule`.

### Changed

- Agent-surface tool descriptions now carry only what a caller needs to call correctly — one opening sentence, at most two sentences on the result shape (budget/elision, tiers, checked:false), the "Reach for it when …" routing sentence, and "Required: …"; plan-row ids, ADR cross-references, and migration notes moved to `docs/mcp-tools.md` (nothing lost, address only), and the five tools whose result can be definitive (`find_symbol`, `find_callers`, `symbol_context`, `verify_plan`, `obligations`) keep a short "this result IS the answer" stop sentence (SB-4). Argument schemas are unchanged, pinned by a pre-diet golden. Measured `tools/list` schema bytes: 60,918 → 47,480 (still 25 tools); `agentSurfaceSchemaByteCeiling` re-pinned to 49,480.
- `find_callers`/`blast_radius` return only edges the Go typed pass resolved by default (heuristic edges behind `include_heuristic: true`); `references`' definition half gets the same treatment; every row carries `edge_confidence` and each response names `edge_filter`.
- `blast_radius` defaults to a depth cap of 2 and a 500-symbol listing cap (both overridable, reported alongside `total_symbols`/`truncated`/`depth_capped`); `diff_impact` stays the unbounded both-kinds walk for PR review.
- `references` defaults to `exact: true` where the service has typed edges (text mentions behind `include_heuristic`), renders one occurrence under 100 bytes (default cap 25), and refuses a query that is not an identifier, a dotted path, or a declared regex.
- `find_callers` renders one caller under 100 bytes by default (`with_signatures: true` restores them) and now unions every candidate a bare name shared by two packages could mean, instead of silently falling back to the syntax answer.
- `find_callers`, `outline`, `symbol_context`, `staleness_report`, `repo_map` are now budgeted: a truncated answer keeps whole rows plus a one-line `elision`; `find_callers`/`references` add a `scope_required` escape hatch for a fan-in too large to page through at all.
- `obligations` orders served rules fresh-before-stale, then by layer role, then specificity; `#Layer` gains `role: "governs" | "claims"` (default `governs`) so external packs fill the serving cap first.
- A claims-layer rule is now scoped at serve time to its own row's `{package, symbol}` contract instead of binding by prose keyword; the tool inlines each rule's cited lines as `snippet` (`include_snippets`) and, called with no `file`, answers "what governs this repository" from the root scope.
- `references` on a query naming a method exactly one indexed interface declares now carries `implementations` first, one row per implementing type.
- `fathom init --hooks-scope project` (or `FATHOM_HOOKS_SCOPE=project`) forces every fathom hook to project scope regardless of Claude Code user-scope settings, so a clean-room or CI harness that loads only project settings still gets them wired.
- `fathom init --harness=codex` no longer writes `.codex/hooks.json` — codex-cli 0.153.1 never loads a project hook file — and prints the entries instead.
- `core.SymbolNode.EndLine` is now set by the Go extractor and most tree-sitter/AST-backed extractors, not only Markdown; `outline`/`query_symbols`/`symbol_context` render `end_line` for these symbols.
- Every Claude Code MCP entry fathom writes now sets `alwaysLoad: true`, avoiding a wasted `ToolSearch` turn per session on a headless run.
- The CGO Java extractor now also emits field and import declarations; `ExtractorEpoch` bumps 6→7, forcing re-extraction of every already-indexed `.java` file on next ingest.
- `fathom review --format github`'s "Changed symbols" caller listing now walks to depth 2 (direct callers plus callers-of-callers), labeled `[tier, depth N]`, instead of stopping at direct callers only; call-site completeness counts stay direct-callers-only. Moved the M15/VB-24 historical-PR recall bench from 5/42 (11.9%) to 6/42 (14.3%) with 0 regressions and 0 false findings — still well short of the row's 30% floor (`docs/bench/readings/review-recall-loop-2026-09-18.md`).

### Fixed

- `fathom daemon install` now writes `GOPATH`/`GOCACHE`/`GOMODCACHE` into the launchd plist and the typed-edge loader repairs them when unset, empty, or relative — previously a Go service could silently fall back to syntax-only call edges while `fathom doctor` reported a healthy index. `doctor` gains a `typed_edges` row, `daemon status` names any missing variable, and `/statusz` (schema_version 3) and `staleness_report` carry a per-service `typed_edges` block.
- The `WorktreeCreate` hook entry is withdrawn: Claude Code read its stdout as the path of the worktree it was meant to create, blocking every worktree-isolated agent spawn. `fathom init`/`fleet reconcile` no longer write it (reconcile removes it as drift); the worktree note now rides on `SubagentStart`. Advisory events with no `additionalContext` (`CwdChanged`, `Setup`, `FileChanged`, `ConfigChange`, `InstructionsLoaded`) emit the top-level `systemMessage` instead; Codex Stop-hook advisories now emit the valid common `systemMessage` field too.
- `obligations` now compiles every `**/CLAUDE.md`, not only the repo root's, floored to its own subtree at compile time — a rule about one subtree can no longer widen scope to a sibling directory.
- `ingest` (CLI and MCP) and `list_services` report `typed_edges: {packages_seen, packages_processed, edges, failed, error?, fallback?}`; `failed: true` now flips the outcome to `degraded`/exit 1 instead of a silent clean success when packages were seen but none processed.
- The daemon `ingest` tool refuses a `path` that differs from a service's registered root unless `remap: true` is passed; `post-commit-hook` checks the registered root first and skips on a mismatch instead of silently re-pointing the service.
- Service-scoped `search` now loads the same `.fathomignore` `read_source`/ingest use and fail-closes rather than serve a file either would refuse; both also exclude `.env`-pattern files and never follow a symlink.
- `obligations` accepts `path` as an alias for `file`; its governing-scope ranking no longer lets a nearer-tier candidate's bare ancestor-directory-name match block a farther-tier, more relevant rule.
- `fathom compile`'s `max_usd_per_run <= 0` now means unlimited, matching `max_tokens_per_run`'s own convention.
- `internal/guardspec.LoadGuards` no longer rejects a `forbidden_paths`-only guard as "governing nothing".

`fathom review` gains an explicit `--mode report|persist` (`--no-persist` is now an alias for `--mode report`), `--manifest FILE` for an explicit changed-path digest set (mutually exclusive with `--since`; a current-content mismatch refuses with `manifest mismatch: <path>` and exit 2), `--idempotency-key` (a repeated key with the same request replays the recorded response from the new additive `review_runs` store table and writes nothing else; a different request under the same key refuses), and `--expect-index-watermark` (a mismatch sets `stale_index: true` and refuses persist while still running every dimension); `--json` now emits a versioned `ReviewResponse` envelope (`schema_version`, `producer`, `inputs` fingerprints, `coverage{complete,unknown}`, `truncated`/`stale_index`/`cancelled`, `unanchored_paths` for a changed process-subject path the index has no symbol for) around the unchanged `Verdict`; `review_diff` (MCP) is untouched beyond an additive `schema_version`. New `fathom review-key --service S (--since REF | --manifest FILE) [--prior FILE] [--json]` and `pkg/client.ReviewKey` compute one sha256 token (`internal/analysis/reviewkey`) over a changed-path manifest, the typed dependency closure (reusing the existing typed call graph, never a new import parser), the project's rule pack/`guard.cue`/layer manifests, `docs/plans/**`, tooling identity, and index watermarks — `complete: false` plus a named `unknown[]` entry whenever any of those inputs could not be verified, and `changed_since_prior[]` against a supplied prior key.

- A stale/contradicted obligation rule no longer renders as if binding: every hook (`edit-hook`, its `--pre` guard, the prompt hook) moves it into a trailing "stale rules — not binding until recompiled" block with one `fathom compile` remedy line, and its inlined snippet is the rule's stored compile-time text, never a live re-read of the current file at the old citation range (which can now hold unrelated content); the `obligations` MCP tool gains `stale_count`/`remedy`.
- `.mjs`/`.cjs`/`.mts`/`.cts` now extract: the "javascript" tree-sitter engine already spanned the JS pair, but neither the connector's language classification nor fusion's extractor dispatch table ever named any of the four, so files with these extensions were silently dropped before ingest reached an extractor.
- `symbol_context` on a bare name matching more than one definition, or resolving to a single definition with more than 50 callers, returns `{ambiguous|scope_required, candidates, count, hint}` and no neighbourhood, instead of a silent empty answer or an unbounded (observed ~63k-token) cross-service caller dump; `find_symbol`'s case-insensitive tier now caps at 20 rows with the same elision disclosure the substring tier already had; `pack_context` refuses a `concern` argument containing whitespace instead of silently running a futile query and returning an empty pack.
- `fathom instrument --adoption`, `fathom research report` and `fathom flywheel` (which gains a `--service` flag) now resolve a named service's call log the same way — its recorded corpus path, then that root's own `.fathom/config.toml`, then the daemon's default for that root — instead of three divergent, CWD-relative rules that could each read a different log for the same repo; `doctor` gains an `instrument_identity` row.
- `fathom layers install <dir> --root DIR` (the documented flag order) is no longer rejected as too many positional arguments; flags are now accepted in either position relative to the pack directory.

- agentic-kit's `evidence-producer` catalog (`foundations/evidence/evidence-producer`) is vendored into `third_party/agentic-kit` (pin `2639212`); `internal/semantic.CatalogFromKit` builds an `evidenceproducer.Catalog` from `schema/semantic/producers.cue` under the shared engineering tier ladder, mapping fathom's `fathom`/`external` origin vocabulary to the kit's `self`/`external`. `ProducerCatalog.Lookup`, `CapTier`, the importer's fathom-origin refusal and a new `Importer.IndependentProducers` corroboration check now delegate to the kit's `Lookup`/`Cap`/`Admit`/`Independent` instead of a private `tierRank` map; `TestProducerCatalogsAgree` cross-checks fathom's catalog against the kit's own `#Catalog` CUE schema (SU-25, ADR-041 decision 8).

<!-- su-wave-s11 -->
- `fathom verify`'s CLI store path (`verifyAllAgainstStore`, `cmd/fathom/verify.go`) built its reachability call graph with `callgraph.Build` alone, never merging in typed Go call edges — unlike the MCP `verify_plan` tool, which already did. On a syntax-only graph, a direct call on a concrete receiver whose method name is shared by another receiver of the same interface (e.g. `SQLiteStore.EnsureIndexes` vs `MemoryStore.EnsureIndexes`) is refused by the ambiguous-name fallback, so `edge_wired`/`test_reaches` falsely reported "does not reach" (O40; SU-1/SU-2 were unprovable by the CLI gate even though the typed pass had already resolved the edge). Fixed by wiring `TypedEdges` + `callgraph.BuildBoth` into the CLI path, matching the MCP tool; `fathom verify --plan docs/plans/substrate.md --strict` now reports `25 claimed: 25 verified, 0 failing`. O40's own working theory (a malformed/duplicated method key) did not hold up under a fixture reproduction and is closed as investigated-and-ruled-out, not as the fix; O39 (references exact mode) did not reproduce against this repo and stays open. O98 (new) names three other `callgraph.Build` call sites — `rename_impact`/`reachable`/`test_impact`/`dead_code`'s shared `buildGraph`, the `callees` tool, and `cmd/fathom`'s plan-context resolver — that look like the same class of gap, unfixed this wave.
<!-- /su-wave-s11 -->

## [0.8.0] -- 2026-09-02

**What this release is.** fathom stopped being a Claude Code program and
started being a harness-agnostic one: the harness is now data behind one seam
(ADR-029) -- Claude Code byte-identical by golden, Codex and Gemini CLI
shipping with real codecs and an honest `unverified` stamp -- and the shipped
skills moved to the `.agents/skills` path 13 of 15 surveyed harnesses read.
Three ways the gate could report a clean result over a broken reading were
found and closed: a verdict that recorded nothing about which binary built the
store it read (O20), a third `package main` that collapsed a symbol key and
silently refuted five untouched rows (O21), and a worktree that re-rooted the
daemon's service while every ingest answered `ok` (O22). The verify gate
gained a lexical oracle (ADR-030) so a documentation or measurement row proves
itself instead of carrying a skip note, and the bench gained the two things it
needed to be believable by someone who is not its author -- a third-party
derived public corpus and reliability statistics on every ledger row
(ADR-031). Nothing here licenses a savings claim; the A/B is still one-armed.
Detail: `docs/changelog/0.8.0.md`.

### Breaking
- **`fathom init --skills=codex` writes `<repo>/.agents/skills/<name>/SKILL.md`
  (the path Codex documents) and no longer `~/.codex/prompts/`; bare
  `--skills` (= `all`) writes `.agents/skills` + `.claude/skills`, both
  repo-scoped.** The deprecated prompts twin is `--skills=codex-prompts`, the
  only user-global write (RP-1; migration:
  `docs/upgrades/0.8.0-skills-on-the-standard.md`).
- **A plan row's `package:` names ONE directory and no longer matches
  sub-packages by prefix** (PC-32), so a row anchored at a parent directory
  stops matching. Migration: name the symbol's own package directory
  (`internal/contextbench/publicloc`, not `internal/contextbench`).
- **`fathom intent audit --json` nests the read outcome in
  `coverage.unreadable[]`** -- `{path, outcome: {kind, detail, bytes}}` where
  `outcome` was a bare string (PC-34, ADR-027). Migration: read
  `outcome.kind` for the old value.

### Added
- **The harness is data behind one seam (ADR-029).** `internal/harness` holds a
  descriptor registry (Claude Code, Codex, Gemini CLI, the generic `agents`
  pull tier) plus per-harness codecs, and Claude Code's hook table, stanza and
  hook envelopes are derived from it byte-identically (HP-1, HP-2, HP-3, HP-9,
  HP-10, HP-16); `schema/harness/schema.cue` validates the exported registry
  (HP-11).
- **`fathom harness list | capabilities | matrix | ladder`** prints the
  registry, each harness's per-capability vehicle table (HP-4), the adoption
  matrix `docs/adoption/other-harnesses.md` pins by test (RP-18), and the tier
  B/C/D rows -- Windsurf, OpenCode, Amp, Zed/ACP, Junie, Warp, Continue, Aider
  -- each with its source (HP-25). Every row of an unverified descriptor says
  `unverified on this machine`, and `docs/adoption/codex.md` §6 is now that
  table, test-pinned (HP-24).
- **`--harness <id>` on `session-hook`, `edit-hook`, `stop-hook`,
  `session-end-hook`, `pre-compact-hook`** (HP-5, HP-6, HP-7, HP-8): absent ⇒
  `claude-code`, and an unknown id or a harness with no codec exits 0 with one
  stderr line. Hooks never detect their caller -- the wired command line
  spells it.
- **Codex and Gemini CLI codecs** (HP-17, HP-19, HP-22, HP-23). Each speaks its
  harness's documented envelope (Codex: `apply_patch` → the patch's first file,
  `additionalContext` cut at 8,000 bytes with a named cut; Gemini CLI:
  `systemMessage` where `additionalContext` is unshown, flat decision never
  emitted), round-tripped against fixtures that cite their source; both
  descriptors ship `unverified` with the 60-day demotion clock started
  2026-09-02.
- **`fathom init --harness=claude-code|codex|gemini-cli|agents|cursor|copilot|none`**
  writes only that descriptor's instruction files and `.fathom/`, generates the
  stanza's hook sentences from the descriptor, and prints the harness's own
  `mcp add` line (HP-12, HP-18). Codex and Gemini CLI hook entries go into
  `.codex/hooks.json` and the `hooks` key of `.gemini/settings.json` through
  the wiring engine with a receipt `fleet undo` reverses; `cursor` and
  `copilot` write stanza and skills only and report `hooks: not wired`
  (HP-20).
- **`fathom doctor` accepts every registry id**, resolves `auto` through the
  registry's presence markers, adds a `gate_posture` row from the descriptor's
  fail mode (HP-13, HP-14), and reads MCP registration or a non-Claude
  harness's hook files as `skipped: unverified`, never a guessed PASS (HP-15,
  HP-21).
- **Store provenance on gate verdicts (O20).** `/statusz` (schema 2) carries
  the daemon's own `binary_commit`, `binary_dirty` and `built_at`, every
  verify-ledger row carries `daemon_commit`, `store_commit` and
  `binary_commit`, and `fathom verify` prints one stderr `provenance:` line
  when the daemon's build is not this binary's -- never compared to the cwd's
  HEAD (PC-1, PC-2, PC-3). Doctor gains `daemon_fresh` and `binary_fresh` says
  it knows the local binary only (PC-4, PC-6); `plans status`, `intent audit`
  and doctor's `unproven_claims` mark a stale daemon's readings
  `measured-stale` -- a coverage gap, never a failure (PC-5).
- **Claim-moment verify (PC-22).** The PostToolUse `edit-hook` runs the verify
  engine for a depth-1 `docs/plans/*.md` the moment an edit adds a `[x]`
  relative to git HEAD and injects the refusal as `additionalContext`, silent
  on a clean claim, an edit that claims nothing new, or any operational
  failure. `~/.fathom/verify-ledger.jsonl` rows gain an additive `origin`
  (`cli` | `stop` | `claim-moment`), and the Stop hook now leaves its own rows.
- **Two more hook rows.** `InstructionsLoaded` → `fathom session-hook --event
  instructions-loaded` writes one silent `instructions-loaded/loaded` beat
  `{file_path, memory_type, load_reason}`, the first harness-side evidence the
  stanza reached a session (PC-23); `ConfigChange` → `fathom config audit
  --hook` runs the custodian audit on the changed repo and writes one
  `custodian/config-change` beat, never a block (PC-24). Doctor's
  `stanza_loaded` PASSes when a beat names this repo's `CLAUDE.md`/`AGENTS.md`
  and SKIPs -- never FAILs -- naming the wiring date otherwise (PC-25).
- **Wiring drift over MCP entries and owned skills.** The law-15 ledger
  baselines each harness descriptor's MCP server entry (argv and env/header
  NAMES, never values) and every `fathom:owned` skill file; `fleet reconcile`
  reports drift by kind and path and never merges (`--apply` records missing
  baselines, `--apply --rebaseline` re-records a drifted one) (PC-28, PC-29).
  Doctor gains `wiring_drift` (PASS / FAIL naming each / SKIP on an unreadable
  file), the custodian a `wiring_current` kind, and the session attention line
  a `wiring drift N` fragment (PC-30).
- **The lexical oracle (ADR-030).** A plan row may carry `{oracle: lexical,
  path: <repo path>, pattern: <regexp>}` and `fathom verify` proves it from the
  working tree -- path exists, non-empty, pattern matches -- so a measurement,
  changelog or design row verifies and leaves U2 (PC-10, PC-18, PC-19). Lint
  and the gate refuse a markdown path without a pattern and a row carrying both
  a symbol and an oracle (PC-12, PC-20).
- **`needs: <sibling>.md#<id>`** -- a row may wait on a row in another plan;
  `fathom verify` lints a missing sibling or row and `plans next` reads the
  sibling's authored state (PC-17).
- **`--format sarif`** on `fathom verify`, `fathom intent audit` and `fathom
  config audit`: SARIF 2.1.0 of the refusals, one rule per criterion / claim
  kind / concern, located at the plan line or the cited file:line (PC-21).
- **One verify-gate figure (PC-13).** `fathom research report` prints a `verify
  gate: figure:` sentence computed from the verify ledger with its window and
  denominators inline (`verifyFigure`), and the adoption gauge reads the same
  computation.
- **`task config-audit`** (`fathom config audit` against this repo) joins `task
  validate`; `TestNoOrgSpecificNames` (ADR-011, with a local
  `FATHOM_ORG_DENYLIST`) and `TestMaintainedLinesParse` lift invariants I19 and
  I36 from `stated` to tested (PC-14, PC-27).
- **`--json` on `ingest`, `init`, `thread brief|list`, `daemon status`, `layers
  list`** (RP-14, RP-15, RP-16, RP-17): ingest carries counts and
  `read_failures` with an errno class and `outcome: ok|degraded|refused` so CI
  can assert `read_failed == 0`; init carries every write, the receipt id and
  the manual steps as data. Envelopes: `docs/usage.md`.
- **`fathom plugin render` emits the Agent Plugins 1.0 `plugin.json` and
  `mcp.json` beside `.claude-plugin/plugin.json` over one `skills/` tree**
  (RP-6). Both manifests vet against the vendored `schema/plugin/schema.cue`
  under `task cue-vet` and the pair is golden-pinned (RP-7); `scripts/ci.sh`
  runs `claude plugin validate --strict` over the rendered tree when a `claude`
  binary is on PATH and prints a named skip otherwise (RP-8).
- **`schema/skills/schema.cue`** vendors the Agent Skills frontmatter spec, and
  every shipped `SKILL.md` vets against it under `task cue-vet` (RP-2).
- **`task stranger-check`**, inside `task validate`: the tracked tree, built
  with `GOWORK=off`, runs `fathom init --harness=none` and `fathom doctor`
  against a throwaway daemon and fails on any FAIL row (RP-5, RP-23).
- **Blast depth.** `find_callers` and `references` rows carry `depth` and
  `edge_confidence` (`resolved` = typed pass, `heuristic` = syntax-only;
  `core.EdgeConfidenceOf`) with a `by_depth` summary, `find_callers` takes an
  optional `depth` that adds `transitive_callers`, and
  `blast_radius`/`diff_impact` add `by_depth` (BD-1, BD-2, BD-3, BD-6).
  `review_diff` and `fathom review` compose `test_impact` -- `affected_tests`
  (cited file:line) and `test_gap`, the changed symbols no test reaches (BD-4,
  BD-5); all fields additive.
- **Served MCP contract.** Every agent-surface tool description names its own
  required arguments and a "Reach for it when" sentence, the 17 tools serve
  `annotations` and a generated `outputSchema`, `tools/call` returns
  `structuredContent`, `tools/list` is sorted by name, `capabilities` reports
  `schema_bytes`, and `initialize` echoes the client's protocol version from a
  supported table (`2025-11-25` … `2024-11-05`) with the
  `MCP-Protocol-Version` header tolerated and echoed (RP-9, RP-10, RP-11,
  RP-12). The per-session schema budget grows from ~12.4 KB to 28.7 KB (16.0
  KB of it the description sentences) and
  `TestServeHTTPNegotiatesWithKitClient` proves `/rpc/agent` against
  agentic-kit's Streamable-HTTP client (RP-13).
- **`contextbenchscore import`** derives the public Go localization corpus
  (`testdata/contextbench/tasks-public-go.json`) from pinned local
  Multi-SWE-bench (CC0-1.0) and SWE-Explore-Bench (MIT) snapshots with no LLM
  and no network, under a pre-registered selection rule, emitting per-task
  `derivation`, `requires`, `subject` and a `corpus_sha256` that reproduces
  bit-for-bit (PB-14, PB-15, PB-17). `cmd/loctruth` computes the `find_callers`
  ground truth with `go/types` -- never fathom's own index (PB-16) -- and
  `docs/bench/public-loc.md` is the runbook with the cost statement and
  criterion H2-2 registered before run 1 (PB-18, PB-20).
- **`docs/bench/ledger.jsonl`** is the per-run bench history, one line per run,
  created by the first run and appended never rewritten; it holds one
  backfilled n=1 row (the 2026-07-24 second swing, indeterminate under the
  floor) and `task bench-ledger` re-renders `docs/bench/README.md` from it for
  free (PB-2, PB-3).
- **Reliability figures on every ledger row (PB-10).** `ComputeReliability`
  (agentic-kit `reliability-stats`) records `ci_success_diff` -- a seeded,
  task-clustered paired bootstrap 95% CI on the per-task success difference --
  plus `wilcoxon_p` on per-task mean tokens, `a12`, and per-arm `pass_at_1` /
  `pass_pow_3`, rendered as a `Δsuccess 95% CI` column. Reported, never
  gating; additive, so older rows render `n/r`.
- **`contextbenchscore regress` / `task bench-regress`** compare the newest
  ledger row against the previous same-corpus row task by task and exit
  0 / 1 / 3 (no regression / a task fell / not comparable); flaky tasks are
  listed and added tasks are not regressions (PB-4, PB-6, PB-13, BC-16).
- **Criterion H2-2** -- precision-aware: a fenced answer block within
  `max_predicted`, precision and recall recorded -- is registered beside H2-1
  in the home corpus, whose verdicts stand (PB-8, PB-11). The corpus also
  commits a leak rule: a task the new no-tools `none` arm passes in 2 of 3
  trials is excluded from the verdict and named on the row (PB-9).
- **Runner flags** `--resume <run-dir>` (score only the missing cells),
  `--subject-root` / `--service` / `--fathom-url` / `--prompt-subject` (bench a
  repo other than this one), `--scratch-daemon` (ingest the subject into a
  store under `workspace/`, never the fleet store), `--env`, `--scoring
  precision`, `--criterion` and `--stream` (PB-5, PB-7, PB-12). `memory_search`
  is excluded from the fathom arm by design and the README says why.
- **`task bench-home` / `bench-lookup`** run three trials per (task, arm) cell
  instead of one and score the run against the criterion committed in the
  corpus (BC-14). Both spend real LLM tokens and are deliberately absent from
  `validate`, hooks and CI.
- **The home corpus is true at HEAD again** (`truth_at_head_revision`, PB-1):
  the moved workspace rule, the inverted ADR-010 staleness verdict, three moved
  premises and the bare `29` token are re-derived, and every ground-truth token
  now carries a `truth_at_head` anchor that `TestHomeCorpusTruthAtHead`
  re-checks in `task validate`, so corpus drift fails the gate.
- **`docs/guides/openviking-side-by-side.md`** -- how to run OpenViking
  alongside fathom without the two fighting, and the authority rule that keeps
  conversational memory from asserting facts about code (ADR-028, untested as
  written).
- **ADR-016 carries a PROPOSED amendment (Q21, PC-16)**: a measured obligation
  ceiling (~320 tokens, the post-edit p90) in place of the unmeasured 150 --
  recorded in `ADR.md` as proposed; nothing changes until the owner decides.

### Changed
- **One read manifest (PC-34, ADR-027).** `internal/readset` is deleted onto
  agentic-kit's `foundations/telemetry/read-manifest`, which the kit
  generalised from it; fathom keeps the rendering adapter
  `internal/readcoverage` and the pinned lower-bound wording is unchanged.
- **`fleet reconcile` plans the shipped skills through the wiring engine**
  (RP-4): both repo-scoped shapes, rewritten only under the `fathom:owned`
  marker and receipted; a hand-authored file by a skill's name or a hand edit
  under the marker is BLOCKED by name. Doctor's `skills_installed` reads
  `.agents/skills` for every non-Claude harness and names each missing skill
  (RP-3).
- **Adoption docs match the binary** (RP-19, RP-20, RP-21, RP-22): the
  `--skills` shapes and `fathom plugin render` in the Claude Code and Codex
  guides, a dated *Verified against* line and the resolved Codex-hooks
  contradiction in `docs/adoption/codex.md` §6, the nine shipped skills
  re-rendered, and `TestNoStaleHelpPhrasesInDocs` forbidding the two stale
  sentences under `docs/`.
- **`contextbenchscore ledger`** pairs the arms, evaluates the corpus's
  `machine_criterion` and exits 0 pass / 1 fail / 3 indeterminate (BC-15). A
  run where a task ran in only one arm, or where the thinnest cell fell below
  the registered replication floor, is indeterminate -- never a win (ADR-031).
- **The home corpus registers a replication floor of 3 trials** it did not
  previously have. The two n=1 runs of 2026-07-24 are indeterminate under it
  going forward; their recorded verdicts stand as history of an n=1 instrument
  and are not rewritten.
- **Bench results report two token ratios**: the aggregate the criterion reads,
  and the per-task median an outlier cannot move. The median is interpretation
  only and cannot become the gate.
- Ledger rows carry `per_task`, `corpus_sha256`, `model_id`, `cost_usd`,
  `dirty_tree`, `cells_lost` and `leaked_tasks` -- additive, so older rows
  parse and read as not-recorded (PB-3, PB-19).
- The U2 remedy in `fathom intent audit` now says what the counter does: a
  `skip:` reason does not lift U2, a symbol or a lexical oracle does
  (`TestU2RemedyMatchesCounter`, PC-7, PC-8, PC-9).

### Fixed
- **A third `package main` silently refuted five untouched plan rows (O21,
  PC-32).** `core.SymbolNode.Key()` collapsed every `func run` in every
  `package main` into one node, so `callgraph.Build` kept whichever was
  ingested last and the verdict depended on ingest order; a symbol under a
  directory is now keyed `service::<dir>/<package>.Name` and `TypedEdgeEpoch`
  2 rebuilds stored typed edges on the next ingest (the old
  `service::package.Name` form still resolves as an input wherever it names
  exactly one symbol).
- **A worktree's post-commit hook re-rooted the daemon's service while every
  ingest answered `ok` (O22, PC-33, I30).** `ingest` -- the MCP tool and
  `fathom ingest` -- now refuses to re-root an existing service at a git
  worktree of its recorded root or any `.claude/worktrees/` path, naming both
  paths, and its `ok` envelope carries `root`; the post-commit hook run inside
  such a worktree refreshes nothing and says which service and root it left
  alone.
- **`-store` / `-call-log` defaults resolve under the enclosing repository's
  `workspace/` from any subdirectory, or `~/.fathom/` outside a repository --
  never the bare cwd** (invariant I31, O11/O12, PC-15). Explicit flags,
  `[daemon].store` / `[daemon].call_log` and `FATHOM_CALL_LOG` still win.
- Doctor's `binding_modules_mounted` / `binding_module_readiness` rows name the
  real reason `list_modules` is absent -- no `--module` store was mounted --
  instead of "tool not found" (PC-11).
- `fathom init`'s manual steps no longer claim Codex has no PreToolUse hook;
  the sentence predated the Codex codec, and the replacement says what is wired
  and that it stays unverified until a Codex session writes beats.

## [0.7.0] -- 2026-09-01

**What this release is.** fathom became something a second person can adopt.
A fresh clone did not build at all — `go.work` was committed pointing at two
private sibling checkouts, and one of those siblings had no in-tree mirror —
and every "the daemon is down" message named one developer's launchd label.
Both are fixed, both are now gated, and the evidence for what fathom does and
does not prove is written down for a reviewer to argue with.
Detail: `docs/changelog/0.7.0.md`.

### 2026-09-01

### Breaking
- **`go.work` is no longer committed** and is gitignored. It listed
  `../agentic-kit` and `../app-kit`, so any clone without those siblings failed
  before compiling a line — and it masked missing `go.sum` entries underneath.
  Contributors who want a workspace now create their own; nothing else changes.

### Added
- **`fathom daemon install | status | uninstall`** (macOS/launchd). `install`
  writes the agent with your own resolved binary, store, call log and a PATH
  carrying your Go toolchain, then bootstraps it — replacing a documented
  copy-paste plist with four paths to hand-edit. It refuses to stack a second
  agent on the same port, and says so out loud when no Go toolchain is on the
  PATH it wrote (without one, auto-refresh silently degrades Go services to
  syntax-only edges).
- **`third_party/app-kit`** — the second private module is now mirrored in-tree
  on ADR-019's terms, so a clone with no private access builds. Each mirror
  records the commit it was cut from in `.vendor-commit`; `task sync-kit`
  refreshes both; `vendor-check` asserts pin and stamp agree.
- **`task clean-clone-check`**, in `task validate`: copies the tracked tree,
  drops the workspace, and builds with `GOWORK=off`. The adopter build broke
  silently once and stayed broken because every machine that ran the gate had
  the siblings.
- **`docs/adoption/README.md`** — a start-here that leads with what is proven
  (the verify gate has caught 540 claimed-but-unproven rows across 2,494 runs
  over 167 plans) and what is not (no savings claim is admissible; the A/B is
  one-armed).

### Changed
- Every daemon-restart hint — `fathom doctor`'s remedy, the SessionStart line
  when the daemon is down, `fathom init`'s closing note, `task deploy` —
  resolves **this machine's** launchd label by reading `~/Library/LaunchAgents`
  instead of naming a fixed one. A machine with no agent is told to install one;
  off macOS, to run `fathom serve-http` under its own supervisor.
- agentic-kit pin advanced 25 commits (2026-07-23 → 9a44f9b).
- The `fathom-doctor` and `fathom-onboard` skills cover the daemon verb; the
  doctor skill's stale "the remedy spells a raw copy" note is corrected.

### Fixed
- **An unreadable input no longer makes a repository look healthier than it is** (ADR-027). `fathom intent audit` reported 0 invariants instead of 37, and half the unproven claims, when one corpus file could not be read — naming nothing. Counts over incomplete coverage now render as named lower bounds (`≥ 85 (1 source(s) unreadable: …: permission denied)`) and blindness leads the NOT-COMPUTED list.
- `fathom doctor`'s `unproven_claims` row and `fathom intent audit` reported different totals for the same tree (12 against 88). The row built a partial audit and printed the result as a total; U12 and U13 need no index and are now included.
- `binary_fresh` reported "go.mod does not declare `github.com/geoffmilleraz/fathom`" against the repo whose go.mod declares exactly that, when the file was merely unreadable. Unreadable is now its own message and a WARN.
- `guard_dead_scopes` called a guard dead when its files exist but are not INDEXED, sending the reader to a remedy that deletes working configuration. It now looks at the disk first.

### Added
- Claim kind **U14**: a sensitive-surface row whose globs match no file has silently stopped guarding. It found one on this repo — `internal/instrument*` guarding "the numbers the program steers by", pointing at a path that does not exist.

## [0.6.0] -- 2026-08-27

**What this release is.** The repository learned to audit what it declares about
itself. `fathom intent audit` reads a Project Intent corpus -- invariants and their
enforcement rungs, conditions, sensitive surfaces, decision records -- checks it
against the index and the gates, and lists every claim nothing proves, with a remedy
per line and no score. Two doctor rows carry it into a session through ONE shared
attention line rather than a second always-loaded one. Two checks were narrowed
because running them against this repo showed they fired on healthy rows, and the
narrowings are the findings. The obligation ceiling was finally MEASURED (missed on
47% of payloads) and its enforcement deferred to an owner decision rather than
shipped blind. Detail: `docs/changelog/0.6.0.md`.

### 2026-08-27

### Added
- `fathom intent audit` reads a repo's Project Intent corpus and reports what it declares, what an instrument can show, and what nothing shows — unproven claims by kind, each with a remedy, and every source it could not compute named. `fathom intent export --guard` proposes `.fathom/guard.cue` entries from the router's sensitive surfaces; fathom never writes that file (ADR-026, `docs/use-cases/intent-corpus.md`).
- `fathom doctor` gains `intent` and `unproven_claims` rows. Neither can FAIL: not adopting the practice is a choice, and honest debt is not a broken repo. Both are silenced by the `intent-corpus` capability, which never silences the verb.
- The SessionStart primer's custodian line became one shared attention line — `⚠️ attention: config N · unproven M (+k) · fired F — fathom doctor` — rather than a second always-loaded line. It reads recorded ledger rows only and goes silent when the last audit is over 14 days old.
- `fathom post-commit-hook` refreshes this repo's index through the daemon instead of a shell ingest against the shared store (INC-3).
- Custodian check kind `adr_references_resolve`: every path a decision record cites must exist, scoped to this repo's own record-id shape so a sibling repo's records are named rather than reported missing.

### Changed
- **`fathom fleet report` `schema_version` 1 → 2**: an `unproven` column read from the intent ledger. A repo with no recorded audit renders `n/a: no audit recorded`, never a `0`.
- The four MCP group surfaces (`coding`, `investigation`, `shared`, `platform`) are retired; `agent`, `companion`, `modules` and bare `/rpc` remain (ADR-014 amendment 2, on law 4).
- The whole SessionStart primer is capped at 3,200 bytes with any cut named; the thread brief keeps its own 2,400-byte budget.
- `fathom help` renders from the dispatch table, so a dispatched verb can no longer be missing from it.

### Fixed
- `fathom plans archive` reads the git history of commits that touched non-plan paths, so a proven plan whose row ids were absorbed by code commits can finally be archived; range and slash-list ids (`SH-1..6`, `IT-14/15`) expand.

## [0.5.0] -- 2026-08-25

**What this release is.** The frontier closed and the instrument told the truth
about itself. Reachability became real (function values are edges, closures
count, discovery gaps are named), ingest became honest (unreadable files are
counted and a wedged reader can no longer replace a good index with an empty
one), law 15 reached the production write path (init and reconcile write only
through the wiring engine, with receipts and undo, and the observatory can
preview and apply behind a local-only gate), threads scope to the repo you are
in, the custodian polices the stanza against the daemon and handoff notes
against their evidence, memories reach the edit loop and the agent surface,
budgets are project-settable, and the observatory renders absence as absence.
The repository also adopted Project Intent (`docs/intent/`), archived its first
proven plans, and re-swept every guide and shipped skill against the binary.
Detail: `docs/changelog/0.5.0.md`.

### 2026-08-25

**Frontier closed: ingest honesty, project budgets, law 15 on the production path, repo-scoped threads, custodian concerns, memory surfacing, observatory series (IT-14/15, BA-9, SH-7/8, TH-6, CC-14/15, AM-6/8, RG-6..8, INC-12)**
- IT-14/IT-15 (O16 fixed): `fathom ingest` counts and names unreadable files in a `read-failed:` column with their errno (plus the external-volume TCC hint), still indexes what it could read, and exits 1; a run that could read nothing refuses to replace a populated index, and the daemon's `ingest` tool returns the same cause as a JSON-RPC error.
- BA-9: a `budgets` block in `.fathom/capabilities.cue` or `~/.fathom/capabilities.cue` sets per-class answer budgets (`list`, `composite`, `map`; tokens; project → user → built-in); `capabilities set` preserves it.
- INC-12, SH-7, SH-8: `fathom init` and `fleet reconcile --apply` write only through the wiring engine — v3 digest markers, receipts, `fleet undo`; a hand-edited tracked region is refused and named while a v1 region is adopted. The observatory gains preview → typed diff → apply with a plan digest (409 on drift) behind `requireLocalMutation` (loopback, same-origin, registered services only).
- TH-6: `thread note|point` default to the current repo's thread; `thread list|brief` show this repo's items and count the rest (`--all-repos`, `--all`).
- CC-14, CC-15: custodian kinds `stanza_tools_served` (every tool the stanza names is served by the daemon; daemon down reads unable-to-check) and `handoff_cites_instruments` (a handoff note claiming shipped work must cite a commit, plan row, verify run, or retro).
- AM-6, AM-8: the edit-hook appends a labeled, budget-respecting memory block for memories anchored to the edited file; `memory_search` joins the agent surface (17 tools), every result carries tier and state, retired memories never surface.
- RG-6..8: the service page renders 30-day per-dimension series and the fleet page a grade distribution from real call-log rows; an absent day renders `stat.Absent` with its reason, never a zero or a line across the gap, and unreadable sources render Absent.
- `task store-purego` joins `task validate` (the pure-Go store is proven on every push); `fathom plans archive` also scans `docs/changelog/*.md` for absorbed row ids and reads the changelog's range and slash-list reference points (`SH-1..6`, `IT-14/15`) as every id they denote; sibling-project names left the fixtures (ADR-011).
- Operator rows recorded with evidence: GKE-W1-14 (live-store format markers), SW-7 (registration at `/rpc/agent`), H1-17 (injection coverage 99% over 168 h).
- Project Intent adopted: `docs/intent/` (router, living docs, decisions index over `ADR.md`) and the stanza in `CLAUDE.md`/`AGENTS.md`.

**Reachability truth: function values are edges, the gate goes quiet (VS-5..10, O18, O19)**
- VS-6: a function referenced as a value is an edge in both passes (`core.EdgeFuncValue`): `mux.HandleFunc(p, h)`, route-table literals, `http.HandlerFunc(h)`, middleware, method values. An httptest that builds a server now statically reaches every handler it registers; webui rows no longer need direct-call tests (consumer finding G15). Building it caught a phantom-edge bug: a selector's trailing ident bypassed the interface check.
- VS-7: the "call it at the test's top level" folk rule is retired with evidence -- both passes already descend into `t.Run`/`defer`/goroutine closures; pinned by `TestClosureCallsReachFromEnclosingTest`.
- VS-5: the syntax resolver already refuses ambiguous same-named methods (never first-wins), proven stable across node orderings; the 08-23 C++ flip's cause was VS-9's gap, not the resolver.
- VS-9: the typed pass counts edges dropped because their target file was outside the discovered set and, when that file exists on disk, names it as a discovery gap on stderr -- a partially wedged discovery can no longer void reachability rows as a clean "0 failed".
- VS-10: `internal/fusion/testdata/reachfix`, the end-to-end reachability harness (mux, route table, HandlerFunc, middleware, method value, closure-only caller, same-named methods) every future reachability claim runs against.
- VS-8: `fathom verify --quiet` prints failing rows and the summary only; the pre-push gate, the init guidance for consumer repos, and the fathom-verify skill carry it (consumer finding G14).

## [0.4.0] -- 2026-08-24

**What this release is.** Ambient fathom, measured. The capability
registry, self-healing fleet, anchored memory, and the flywheel check
(phases T-Y) made governance a delivered configuration; the impact
observatory, exploration ledger, and continuity threads made the program
answer for itself in instruments; the config custodian made "instructed
means true" mechanical. The 08-21 evidence audit then turned the
instruments on the instruments (the verify gate proven, the A/B one-armed,
the gauges caught lying under the TCC wedge and the OFF arm) and the
closing waves shipped instrument truth, the work frontier (ctx packs,
eligibility, leases, `fathom prime`), the verify spine (CUE joins the call
graph), the vision-05 registry prune (34 tools deleted on evidence), and
budgeted answers. Detail: `docs/changelog/0.4.0.md`.

### 2026-08-23

**Verify spine, surface prune, budgeted answers (VS-1..4, SP-B1..3, BA-1..8)**
- verify-spine (VS-1..4): O4 annotation tokenizer is quote-aware with a loud lint on malformed blocks; CUE joins the call graph via `cueReferenceEdges` (unification/field-type/list-element/import edges), fixing O3 and ending the skip_test-only exit for CUE rows.
- surface-prune (SP-B1..3): vision-05 kill clock executed, 34 of 59 zero-call registry tools deleted, 25 exempt with per-tool evidence; `docs/mcp-tools.md` synced with a registry-doc bidirectional test; `instrument --adoption` gains IT-1 read-error classification, fixing O17 (no more "88 of 88 zero" from a wedged reader).
- ambient-fathom (BA-1..8): budgeted answers ship, `elisionFor` with a registry-iteration disclosure test (caught `repo_map` disclosing nothing on truncation), `applyBudget` with strict-prefix and never-refuses rules, per-tool defaults, per-call size accounting, and the observatory Cost panel.
- CUE extractor migrated off deprecated `f.Imports` ahead of its mid-2026 removal; a fourth reachability silent-void confirmed (same-named methods do not wire, the Extract collision), worked around with direct wire tests.

### 2026-08-21

**Evidence audit, work frontier, instrument truth (IT-1..13, WF-1..11)**
- 12-agent audit of every feedback channel produced the first full evidence verdict: verify gate PROVEN (180 of 436 gate runs/5d caught claimed>verified), thread bet COLLECTING at 4/20, A/B NOT ADMISSIBLE (ON arm: 0 admissible units); instruments themselves lied under the TCC wedge and the OFF arm.
- instrument-truth (IT-1..13), 24/24 rows gate-verified: flywheel read-errors OPEN never IDLE with wedge hint; `ArmStateForWindow` arm-aware; counting beats survive `FATHOM_CHANNELS=off`; doctor surface_traffic falls back to persistent call log; session-hook fetch failure never "not indexed" (O2); observation ids mint under flock; adoption verify figure derives from verify ledger, retiring the 215/4940 double-count (O6).
- work-frontier (WF-1..11): `ctx:` pointers + `plans context` materializer, `needs:` + derived eligible/blocked_on + `--eligible-only`, TTL-only advisory leases (.fathom/leases.jsonl), `fathom prime` for Codex, dispatch boundary registered in vision 24.
- Found while shipping: a wedged ingest writes a zero-symbol/zero-edge index while reporting "0 failed", a fourth false-signal shape; measurement arm flips ON now that IT-1..5 are live, OFF floor exceeded 5x.

### 2026-08-14

**Completion pass: recommendations shipped (O29, SD-1, CC-8..10, DO-1)**
- O29 interim fix: `plans status` fans per-plan verifies out 8-wide (was serial at ~4s x ~40 plans = 2m35s measured).
- SD-1: the verify ledger, every gate run appends per-plan rows to `~/.fathom/verify-ledger.jsonl`, closing O14/O30 (flywheel verify-gate loop read only session-scoped runs; a repo gating on every push read "never ran").
- CC-8: the custodian flywheel loop (IDLE/OPEN/CLOSED), fed by a new custodian audit beat; CC-10: `fathom init` scaffolds a commented concerns.cue and fleet reconcile reports zero-coverage repos informationally.
- DO-1/2/4: A/B criterion's outcome half now derives from disk (median commits per unit beat-window); `fathom rate` demoted to optional human override.
- DP-1: the doctrine catalog design doc. Spillover named: SP-10, SH-7..9, AM-6, AM-8, IR-3..5, DO-3, CC-9/11/12/13, SD-2..4, DP-2..4.

### 2026-08-13

**Adoption surfaces catch up to shipped fathom**
- Refreshed continuity threads, lifecycle hooks, capability registry, scope/purge, config custodian, and plans status/next/archive across all adoption surfaces, every claim checked against the binary.
- Shipped skills updated: onboard teaches the full hook stack including SessionEnd/PreCompact and the nine-skill set; doctor's row table gains capabilities + concerns; a new ninth skill `fathom-custodian` added.
- Docs synced: Claude Code guide (7.1 brief+attention injection, new 7.7 continuity threads, 8 concerns.cue + capabilities.cue, 11 nine skills), Codex guide, usage.md, mcp-tools.md.
- Install demo caught a real defect before commit: `skillsdist.Names` is the canonical list and the embed directive alone does not ship a skill, nine skills now proven installed by demonstration.

**Config custodian ships its audit (vision 22, CC-0..7)**
- "Instructed means true": agent configuration joins the evidence plane via `internal/custodian`, loading falsifiable wants from CUE at two law-13 scopes (`~/.fathom/concerns.cue` user defaults, `.fathom/concerns.cue` governing).
- Four mechanical check kinds run from file reads only, no daemon, no LLM: forbidden_pattern, command_documented_and_exists, max_lines, pointers_resolve.
- Surfaces: `fathom config audit` (text/JSON, exit 1 iff violated, zero concerns exits 0 with absence named), a doctor row, and a session-brief attention line that exists only while violations exist.
- Falsification runs confirmed red for project-governs and file:line-evidence mutations; vision 23 and vision 22 section 7 land as specs with plan rows CC-8..13, DP-1..4, SD-1..4 open.

**Brief learns from its first reader (O28)**
- First real-world contact: a fresh session reported the brief was just "session ended with N dirty files", no substantive handoff.
- Deposits supersede per repo (stable `se-<repo>` id): the fact closes itself when the tree comes clean (`repo_clean`).
- Brief now leads with the spec plane (local plans summary, checked vs open) before this repo's items, then a collapsed foreign-telemetry line.
- A thin brief teaches: when no handoff notes exist, it says exactly what to run before `/clear`. Supersede confirmed RED under mutation.

**Reconcile detects stanza-content drift, fleet refreshed**
- Stanza template gained the continuity line, but `fathom fleet reconcile` reported "wiring current" for all 17 repos, it checked stanza PRESENCE, never content against the current template.
- `staleStanzaFiles` now compares each marker-owned region against the template rendered with the registry's service id, dirname only as a daemon-unreachable fallback.
- First apply using dirname mis-rendered exactly one repo, sextant (registry id `fathom`); the corrected binary flagged and repaired its own mistake.
- Applied fleet-wide: 17 repos, 0 drifted, every stanza now carries the continuity instructions.

**Phase V lands: scope hides, purge proves (SP-3)**
- All six rework defects re-reproduced against the new code and confirmed dead: fleet-wipe purge, inert suppression, VERIFIED-without-probe, constructor-blind audit, fleet-wide exclusions, home-reading gate.
- `read_source` served purged content from disk after a purge printed VERIFIED; fixed so `read_source` and `symbol_context`'s snippet path load service-scoped exclusions and consult suppressions per call, refusing with the suppression named.
- The disclosure channel (SP-3) was inert in deployment: `ApplySuppressions` produced disclosures no shipped surface collected; fixed by wrapping MCP dispatch's ctx with the collector, rendering a `suppressed` block into the envelope.

**TH-5: continuity thread promote and handoff**
- `fathom thread promote <thread>` scaffolds a gate-ready plan from a thread's open items with stable uppercase ids and origin session pointers; no row born checked, no contracts claimed.
- Promoted items gain `plan_row_checked` conditions on their own rows so checking a row closes its source item; promote refuses to overwrite an existing plan.
- `fathom thread handoff` renders open work as a harness-neutral markdown artifact with full session ids, grouped by thread, unbudgeted.
- Stanza template gains the Codex path (SP-1, SP-9, SP-11, SP-12): harnesses without SessionStart are instructed to run `fathom thread brief` at start; note/point/close accept the natural `"text" --flags` order.

**Phase V rebuilt: scope/purge defects made structural (V1-V6, AM-12, SP-10)**
- `fathom scope list|suppress|unsuppress|preview` (reversible, disclosed suppression) and `fathom purge ... | history` (irreversible, verified removal) rebuilt so each of six verified defects (V1-V6) is impossible by construction.
- V1: `PurgeReport` carries a tri-state `PurgeVerdict` (verified/bytes-unchecked/failed) computed in one place (`purgeVerdict`), mapped to distinct exit codes.
- V2/V4: ingest-time exclusions are service-scoped by construction via `~/.fathom/ignore.d/<service>`; V3: chokepoint audit derives constructor list from the store package's own source, catching `MountReadOnly`.
- V5: suppression applies at the query layer, re-read per call via `scope.Source` stat-based reload; V6: no init()-armed globals, resolved only through `$FATHOM_SUPPRESSIONS`.
- Everything deleted and proven per location, closing AM-12 (`.fathom/memories.jsonl` purge for quoting memories); SP-10 (observatory Scope panel) stays open by instruction.

**Continuity threads live (vision 21, TH-1..4)**
- The thread, one strand of intent spanning sessions, is now first-class: `fathom thread brief|note|point|list|close`, an append-only store, items that close themselves when their awaited artifact materializes (`plan_row_checked` / `file_exists`).
- SessionEnd hook deposits one mechanical item only when work is actually in flight; PreCompact files a pointer before context dies; neither reads transcript content.
- The brief is capped at 2,400 bytes (~600 tokens), injected only on startup/clear; `resume` and `compact` print nothing, both properties confirmed RED under mutation.
- Measurement loop: every ledger row carries cohort labels, `threadBetCheck` evaluates the registered bet, flywheel gains a continuity loop. TH-5 (promote/handoff, Codex stanza) is next.

### 2026-08-12

**Arm-covered windows count as measurement unit (MI-5)**
- Owner pushed back on a 40-fresh-session baseline: his practice is one session resumed across days, so session-level units produced a control arm that could never admit anything.
- Amendment registered before any data existed under it: the unit becomes an arm-homogeneous unit, a fully-covered session or the arm-covered window of a longer one, restricted to lines timestamped after the arm's `set_at`.
- Compared quantity becomes median real tokens per assistant turn, with a >=10-turn floor per unit and >=20 units per arm; window read is tail-bounded, a truncated window is excluded and named.
- Both leak paths (since-filter removed, whole-session figures admitted) confirmed RED under mutation; flywheel's arm loop now admits windowed units.

**Daemon self-updates its own binary (AU-1..3)**
- launchd `KeepAlive` means the daemon exiting IS a restart; it now watches its own executable and steps aside for a verified newer binary instead of waiting for a human to run `launchctl kickstart`.
- AU-1 adds four safety properties: verify the candidate executes and reports a version before yielding, require the same bytes across two consecutive polls (settle), rate-limit to one adoption per cooldown, and log every adoption/refusal loudly.
- Guards against a real incident: an unsigned binary once installed over the daemon's path and macOS refused to run it (`OS_REASON_CODESIGNING`); exiting on file-change alone would have looped forever.
- `FATHOM_SELF_RESTART=off` pins the binary, and the choice is announced rather than silently honoured.

**Dropped instrument fields caused zero admissible A/B sessions (MI-4)**
- `transcriptSummaryFrom` hand-copied 3 of 12 parser fields; every field added later (including `FirstTS`) was computed then discarded, so `arm_coverage` read `unknown` and not one OFF-arm session was admissible (EX-1, IQ-2).
- Consequences: no arm ever had a spend-shaped median (in/out/cache split dropped), warm-up (IQ-2) forever reported 'rows predate the instrument', and exploration-ledger per-question cost (EX-1) never filled in.
- Fix: whole-struct assignment so new fields carry by construction; `TestTranscriptSummaryCarriesEveryField` asserts via reflection so reverting the fix requires naming all twelve fields.
- Also fixed: `TestFlywheelSurvivesWithoutALocalCallLog` used process-global `os.Chdir`, breaking later tests resolving relative paths; replaced with a `FATHOM_CALL_LOG` override.

### 2026-08-09

**Session-count measurement bug inflated denominators 25x (MI-1..3)**
- Claude Code's `Stop` event fires per turn, not per session close, so 200 ledger rows were only 8 distinct session ids; one resume_as_code session wrote 74 rows alone, inflating 'of 188 sessions' denominators roughly 25x.
- `discovery_actions_total` is a per-session cumulative, so summing rows double-counted: 17,727 collapsed to 1,038 after fix; the pre-registered '>=20 sessions per arm' floor would have been met by about one real session.
- Fixed by deduping at read time, keeping the LAST row per session id, while still reporting the raw stop count alongside for auditability (MI-1).
- New `fathom rate <1-5> [--note]` command instruments the self-rated outcome half of the criterion, joined to the ledger by session id at read time; an unjoinable rating is refused rather than stored.
- `evaluateABCriterion` now states in words which parts are unmet, e.g. 'the OFF arm has 0 of the 20 sessions registered as the minimum; the ON arm has 5 of 20'.

### 2026-08-08

**Realization grade replaces wiring grade, ladder of 5 rungs (RG-1..4)**
- `wiringGrade` could score a repo A from configuration alone (one call ever, no obligations firing, no gate running); `realizationGrade` (RG-1) instead folds a 5-dimension ladder: wired, fresh, queried, preventing, gating, each a precondition for the next.
- 'Queried' now counts calls in the current window rather than ever, so the signal can actually go down; 'preventing' and 'gating' are new and are the only dimensions measuring fathom doing something rather than being available.
- The letter grade names the first broken rung (e.g. 'B -- not preventing') instead of a bare letter.
- A repo with no plans grades gating not-applicable rather than failing; a repo with no sessions in the window grades idle, never F, avoiding false alarms that bury the real one.
- Fleet page now sorts by unrealized value (unmet rungs weighted by sessions) instead of call count, ties breaking on sessions then name.
- Refs: SH-1, SH-6.

**Self-healing fleet engine, fathom edits repos under law 15 (SH-1..6)**
- `fathom fleet reconcile --apply` sat unrun beside 9 of 17 repos for weeks since it required a human to act; `internal/wiring` (SH-1..6) makes it a safe button via three ownership tiers.
- Marker format v3 (`<!-- fathom:begin v3 sha256:... -->`) hashes the region body as fathom last wrote it, so a hand edit inside fathom's region is detectable with no history; pre-v3 markers are recorded as unknown provenance, distinct from drift (law 3).
- `.fathom/owned.json` ledgers keys fathom added to files it doesn't own; a losing ledger makes fathom own nothing, never a duplicate.
- Plan -> apply -> receipt: the plan is always computed before mutation, and one BLOCKED item does not block others in the same run; law 15 is one-sided -- a drifted region is never merged, only reported BLOCKED with both versions attached.
- New verbs `fathom fleet receipts`, `fathom fleet undo --last`, `fathom fleet undo <receipt-id>`; undo restores from receipt (never inverts) and refuses if the file changed since, itself receipted.
- Also fixed: `fetchServicesOnce` dereferenced a nil `*http.Client`, so `fathom flywheel` panicked outright when the local call log was absent.
- Refs: AM-1, AM-5, AM-7, SH-6.

**Anchored memory, refusal as the product (AM-1..5, AM-7)**
- Phase W of `docs/plans/ambient-fathom.md`: a memory with no anchor is refused outright (not stored at low tier), with the reason and destination stated, e.g. 'put it in CLAUDE.md'; the store stays empty on refusal by test.
- Tier is derived structurally from evidence kind, never asserted by the caller; `WriteMemory` routes claimed confidence through agentic-kit's `scoring.ApplyCeiling` plus `TierCeiling` (AM-1's first caller), proven against a lying caller claiming verified/5 that lands capped at assumed/1.
- Drift is evaluated at read time and cached per index generation, no sweep or timer, so it can't be missed while the daemon is down.
- Lifecycle: fresh -> suspect (anchor content changed, tier drops one step) -> orphaned (anchor disappears, stops surfacing, never deleted); `fathom memory review` resolves as still-true/no-longer-true/rewrite, with model-confirmed landing strictly below human-confirmed automatically.
- New `internal/memory` package and `fathom memory add|list|review|show`, stored at `<root>/.fathom/memories.jsonl`, project-scoped, loopback only (law 13); AM-6 (obligations injection) and AM-8 (`memory_search` tool) deliberately deferred.
- Refs: AM-5.

**`fathom flywheel` verifies feedback loops are actually closed (FW-1..3)**
- New command checks 7 loops (session ledger, counterfactual arm, call log, obligations, verify gate, exploration ledger, observations) and reports each CLOSED (produced, arriving, and consumed), OPEN (a defect, exit code 1), or IDLE (nothing to measure yet, never fails the exit code).
- Motivated by two same-day silent measurement failures: the OFF arm burned 14 hours recording nothing, then produced rows that were all inadmissible.
- The arm loop distinguishes 'armed but recording nothing' from 'rows arriving, none admissible' -- the latter is subtler because the producing side looks alive.
- Caught a bug in itself on first run: the obligations loop read `ObligationCalls` off the impact model where it is always zero instead of the call log via `buildObservatoryModel`, wrongly reporting IDLE on a loop carrying 6,994 injections.
- Live fleet result at time of writing: 4 closed, 1 open (counterfactual arm), 2 idle.

**Arm coverage labeling fixed after OFF-arm rows leaked pre-arm activity**
- First two OFF-arm rows after the earlier fix were corrupt but unlabeled: their transcripts began 10 days before the arm was set (duration_ms 243h and 256h), one carrying tokens_total 933,232,702, yet stamped arm: off.
- Such rows would drag the OFF arm toward the ON arm in a paired comparison, manufacturing a false null result ('fathom saves nothing') from mislabeled evidence.
- Producer side: `sessionLedgerEntry` gains `arm_coverage` (+note) via `armCoverage`, valued full (session began at/after arm set, the only value admissible), partial (transcript predates the arm), unknown (no timestamped line, not assumed clean per law 3), or empty (no arm active).
- Consumer side: `buildImpactModel` excludes partial/unknown rows from both arms and names the excluded count with reason rather than silently dropping them; a row stamped off with no coverage field is treated as unknown.
- Also fixed: the brief promised a median comparison whenever both arms had any sessions, even without tokens on both sides; every new test was confirmed RED against the ungated fold first, without which the 933M row leaks into the OFF median.

**Capability registry reported switches it never enforced (CR-7)**
- `fathom capabilities set guards off` wrote config and the table reported 'switched off by this repository' as fact while guards kept firing on every edit -- a config read rendered as behavioral fact, violating law 14 inside the package built to enforce it.
- Fix distinguishes `Capability.Switchable` (obligations, guards, verify-gate: actually gated in `edithook.go`, `edithook_pre.go`, `stophook.go` before daemon traffic) from Status capabilities (index, layers, modules, agent-surface, session-ledger, narrate), which `set` now refuses and names the real control instead.
- `verify-gate` off still records the ledger row with `verify_catches` as nil (not a fabricated zero), since the gate did not run and its result is unknown (law 3).
- `session-ledger` is deliberately status, not switchable, so measurement cannot be silently disabled from a config file; `FATHOM_CHANNELS=off` remains the documented kill switch.
- `TestEverySwitchableCapabilityHasAnEnforcementCase` fails the build if a capability is marked switchable without a test proving observed behavior changes; all enforcement tests were confirmed RED against the shipped defect first.

**URGENT: OFF arm was silencing its own measurement, zero ledger rows**
- `fathomChannelsDisabled()` returned `env == "off" || armOffActive()` and every hook called it first, so an active OFF arm made the session hook exit before its announcement/decrement and the stop hook exit before `recordSessionLedgerEntry`, leaving zero ledger rows.
- Measured cost: an arm set 2026-08-08T07:11:47Z produced no ledger rows for the rest of that coding day, with `sessions_remaining` stuck at 10 of 10 since the decrement never ran.
- `TokensOffSessions` could never rise, so the counterfactual pilot could never leave 'collecting' -- the one instrument able to license a savings claim (law 12) was structurally unable to produce one.
- Fix separates measurement from assistance: `fathomChannelsDisabled()` is now the env kill switch alone; new `assistiveChannelsDisabled()` = env or active OFF arm, used only by edit hooks (obligations, guards, pre-edit channel); session and stop hooks use the env gate so they always record, and the stop hook returns before the verify pass (an assistive gate) leaving `verify_catches` nil rather than 0.
- A test, `TestArmStamping/channels_kill_switch_honors_the_arm`, had asserted the buggy conflation as required behavior for weeks; it and a new `armmeasurement_test.go` now pin the correct contract, confirmed RED against the old gate first.

**Capability registry, one typed on/off surface (CR-1..6)**
- `internal/capability` is a typed registry over four extension seams; `fathom capabilities` is the new verb answering "why isn't X happening here?"
- Detection is observation, never configuration: `guards: on` with no rules compiled reports OFF with the reason (law 3 applied to config).
- Every registry entry must state why it is off, enforced by a test iterating the registry against a ZERO probe; absence and breakage are kept distinct so only breakage fails doctor.
- Law 13 enforced loudly: a user-scope file setting a project-only key is refused with the key named, unknown ids get nearest-match suggestions.
- Fixed three defects found by tests: unbound module read as breakage, duplicated `daemon_reachable` diagnosis, and index freshness collapsed from three states (unknown/stale/fresh) to two; also renamed ambiguous anchor `Validate` to `ValidateSettings` (O22, O23).

**Value brief rewritten as a readable story (BR-1..4)**
- The brief now opens with a plain-language short version (what fathom is, its scale, what it has and has not shown) plus five numbered questions each with an explicit Reading line.
- All counts route through the observatory's existing `fmtThousands` (`17,727` not `17727`); a duplicate helper added in a first draft was deleted in favor of the incumbent.
- The closing section became a work order: each entry names what's switched off, what enabling it would let a future brief say, and the exact command or a reason none exists.
- Exposure is not outcome: the prevention section now states that 6,992 rule injections and 17,727 discovery actions count things fathom did, not mistakes avoided.
- Brief page rendering escapes markup before tag emission so an instrument note can never inject markup.

**Observatory readable pass: token medians, chart values, index-only honesty**
- The 596M-token finding: `tokens_total` summed all flow including cache reads, inflating a single row's "median session" to 596M tokens; the ledger now splits in/out/cache at instrument, model, and render layers.
- Rows predating the split count as unknown rather than feeding the median an inflated total; OFF arm now reads "median real tokens --" instead of "no token totals".
- The tool-load chart now renders comma-grouped value labels (`ShowValue: true`) instead of relying on bar-width eyeballing.
- Every non-A wiring card now shows a remedy via fallback to `GradeRaise` when `FreshnessRemedy`/`WiringRemedy` don't cover the "wired but unused" case.
- A C/D-graded service with zero ledger sessions and zero agent calls now renders a neutral "index-only" chip instead of a scary amber letter; learn-tooltips added for "(N agent)" traffic figures.

**Exploration ledger: the 90x measured, dashboard readable (vision 19, EX/FR/TD)**
- Every session transcript now records both raw exploration (Grep/Glob/Read sizes) and indexed discovery (fathom answers), measuring per-question token cost and ratio, folded fleet-wide (EX-1/2/3); impact page leads with this story (EX-4).
- Dashboard readability: every page opens with a plain-English lead and no-JS explainer; `/observatory/brief` renders a management-ready summary with markdown download (EX-5/6); service page shows layers, guards, bindings, and obligations-compile condition (EX-7/FR-5).
- Field report fixes: plans status/next now discover plans in the service's own repo, not blindly cwd, fixing a cross-repo wrong-verdict bug (FR-1); the 264,487-catches artifact retired in favor of per-session verify_catches (FR-2).
- Fleet reconcile detects and `--apply` installs missing shipped skills (FR-3); session hook now teaches the first query with a live symbol from the repo's own index so the first query lands (FR-4); O13 filed: CompileLayer ignores the provider registry, deferred to FR-6.
- Tool diet (TD): agent surface pinned at <=16 tools (~12.3 KB schemas, ~3.1k tokens/session) by test; the alarming 88-tool count is the full /rpc surface agents never load; wiring grades stop reading invisible-from-here as absent (EX-8).

**Observatory v2: the four questions leaders ask (vision 18, IQ-1..11 + IQ-15)**
- Q1 (what did the agent not have to do?) stays DARK until OFF-arm sessions exist, law 12 rendered as product; Q2 (what did fathom prevent?) shows verify catches plus 6,985 obligations served live from the call log, with conformity checking deferred to IQ-13.
- Q3 (where does the agent flounder?): call-log rows carry result_count and matched_via (IQ-1) so hits/misses/did_you_mean are distinguishable per tool; Q4 (is knowledge compounding?): ledger learns first_edit_turn (IQ-2).
- New story layer: `/observatory/session?id=S` replays one session as a Timeline with a ledger verdict card (IQ-4); `/observatory/story` composes the trailing week with session-replay links (IQ-5); landing page leads with the four questions (IQ-6).
- Fleet tiles gain 14-day sparklines, top-tools bar chart, and a WIRING GRADE chip (A-F) with the one command to raise it (owner ask); brief reorganizes around the four questions (IQ-11).
- `--for 14d` parses (IQ-9); `fathom init` auto-arms a 10-session narrate window, self-expiring (IQ-10), ON fleet-wide until 2026-08-22; deferred: session-start context pack (IQ-12), obligation conformity (IQ-13), LLM weekly (IQ-14).

**Observatory: value realization at a glance, built on app-kit (vision 17, OB-1..12)**
- Daemon grows a full loopback front door: GET /, `/observatory` fleet tiles, `/observatory/service?id=S` detail, `/observatory/impact` verify catches/discovery actions with denominators and a counterfactual pilot state machine (law 12); `?format=json` everywhere.
- `fathom brief` renders the impact model to a dated local markdown value brief with an explicit "Not measured" section, keeping sharing a human verb (law 13).
- Built entirely on app-kit per owner directive: dashboard.RenderWidget/Grid, stat.RenderBand, viz.Sparkline/Gauge replace hand-rolled tiles deleted the same day; the honest-absence tile was promoted up into the kit as `stat.Absent` (app-kit af47262, D13).
- CSS builds via `task ui:css` from `cmd/fathom/ui/input.css` using the kit's checksum-pinned Tailwind binary, no Node; UI doctrine wired into CLAUDE.md/AGENTS.md and .fathom/guard.cue.
- Every page verified by Playwright screenshots in both themes; fixes included absence-note duplication, missing thousands separators, and a duplicated law-14 line.

**Full C++/CMake/clang/LLVM/Ninja support (cpp-toolchain, IO-27 trigger fired)**
- Symbols get one spelling: out-of-class definitions ("Widget::render") normalize "::" to "." so call edges join inline-method spelling (CT-1); templates, operators, destructors, and C++17 nested namespaces pinned by tests (CT-2).
- Polyglot `.h` extension now routes per-file through a C++-signal sniffer so LLVM-style headers keep classes/namespaces/methods instead of being flattened by the C grammar (CT-3).
- CMake becomes a dialect: CMakeLists.txt/*.cmake extract projects, targets, functions, options, find_package deps, with target_link_libraries as target-to-target edges (CT-4/5), via TierPreScan hand scanner (no variable-indirection guessing).
- languages field stops lying for C-family services: cpp/c/cmake/rust/csharp/cue (CT-6); doctor's `cpp_toolchain` check warns (never fails) when compile_commands.json is missing, naming the Ninja+clang export command (CT-7); init excludes build/, out/, cmake-build-*/ (CT-8); build.ninja refused as indexable input (CT-9).
- ExtractorEpoch 5->6 forces re-extraction of all stored C/C++ observations; calibrated live against fmtlib/fmt: 5,470 symbols (424 classes + 181 namespaces from .h headers), 24 CMake targets, zero "::" contamination (CT-10). Precise clang mode, WASM engine migration, Conan deferred (CT-11/12/13); O12 filed: ingest CLI's cwd-relative -store default indexes into the void.

**Activation plan closes: exit proofs run live (IO-30/31/32)**
- IO-30: `fathom instrument --adoption --service app-kit` from app-kit cwd resolved the daemon call log (69 calls, bare=12/agent=57 split), proving the gauge is cwd-independent.
- IO-31: doctor on app-kit ran 24 checks, 20 pass/3 fail/1 skipped, and every FAIL was true (lefthook_gates, skills_installed, surface_traffic) -- no false doctor row survives on a consumer repo.
- IO-32: the counterfactual pilot was pre-registered before any OFF session, N=10 matched sessions per arm, primary metric tokens_total from the ledger, medians reported as a range with denominators, kill criteria filed per law 12.
- O9 closed (fixed by the IO-9 budget fix); O10 filed: a redeploy rolls the TCC dice, new binary hash under launchd silently loses the removable-volume grant; O11 filed: CLI call log resolves cwd-relative.
- Final sweep `fathom verify --service fathom --plan docs/plans/impact-observatory-activation.md`: 32 rows, 29 claimed, every contract-bearing row verified, three deferred rows carry triggers.

**Skills pack: eight skills, three renderings, one plugin (vision 16, IO-23/24/25/26)**
- fathom-rewire and fathom-activate join the embedded skills pack, promoted from the reconcile pass and activation catalog prompts.
- `--skills=agents` renders the same SKILL.md set to the shared `.agents/skills` path Codex and Gemini CLI both walk natively, one spec-conformant write for multiple harnesses.
- `fathom plugin render` generates the Claude Code plugin from the same embedded sources: plugin.json, all eight skills, hooks/hooks.json spelled via the shim-stable `fathom`, .mcp.json, and optionally a marketplace manifest.
- IO-26 integration proof run live: both kit packs (app-kit ui-appkit, agentic-kit agentic-kit-standards) install via `fathom layers install` and list with honest not-yet-eval-gated warnings.

**Narrate v0: enhanced-understanding dial, story rows only (vision 14 section 6, IO-21/22)**
- `fathom narrate on --for D|--sessions N` is bounded, auto-expiring, and off by default; an unbounded narrate call refuses.
- While active, hooks append version-stamped story rows per intervention (obligation-served, verify-catch, guard-served) to `<repo>/workspace/artifacts/narrate/story-<date>.jsonl`.
- Story rows are per-repo, local, never bundled, and a lens over mechanism-labeled events, never a source of truth (law 14 candidate) -- nothing downstream may consume them as evidence.
- Pinned by TestNarrateExpiry and TestStoryRows, confirming inactive narrate writes nothing.

**Fleet reconcile and session tripwire: wiring is a diff (vision 14 section 7, IO-17/18/19)**
- `fathom fleet reconcile` diffs every repo's installed hooks against the canonical template (fathomHookSpecs), naming missing entries, stale absolute binary paths, and pre-rename stanza branding per repo.
- `--apply` executes only the SAFE subset: repointing stale commands to the shim-stable `fathom` in the project file, merging missing entries through init's idempotent machinery, one scope, never both.
- The session hook is now the tripwire: a drifted repo's session opens by naming the drift and the exact remedy.
- Pinned by TestReconcileDiff/TestReconcileApply (drift -> apply -> clean) and TestSessionTripwire.

**PATH shim: hooks spell `fathom`, never a checkout path (IO-20)**
- `fathom init` installs `~/.local/bin/fathom` pointing to the deployed libexec binary, falling back to the running executable, refusing to clobber a foreign real file, and idempotent on rerun.
- doctor gains a `binary_on_path` check after a 2026-07-31 case where `fathom verify ...` printed command-not-found as one token and the pipeline exited 0, a verification failure wearing a pass.
- One stable name on PATH ends the class of bugs where hook behavior depends on when a checkout was last rebuilt.

**Fleet report: the audit as a verb (IO-16)**
- `fathom fleet report` folds daemon registry, per-service call-log traffic, local wiring state, open observations, ledger session counts, and three-state freshness (indexed commit vs HEAD via statusz) into `~/.fathom/fleet/report-<date>.{json,md}`.
- Local files only, no daemon writes, no network beyond loopback; collection stays per-repo, consumption is explicit and human-run, not silent aggregation.
- Every unmeasured cell is a dash, never a fabricated zero.
- First live run folded the real fleet's 18 services.

### 2026-08-07

**Ledger attribution: verify catches ride the session row (IO-14, IO-15)**
- Stop hook now runs its verify pass first; the session-ledger row carries `verify_catches`, the proven-failing count for newly-claimed tasks, as a nullable field (nil = UNKNOWN when verify never ran, not a fabricated zero).
- Together with `arm` (IO-14) the ledger now holds the counterfactual program's join keys per session.
- The vision's other two attribution fields, `injected_rules_used` and `discovery_substitutions`, need obligation-file tracking and call-log/beat correlation that don't exist yet; deferred, not dropped.

**Measurement arm: FATHOM_CHANNELS=off with a memory (IO-14, O15)**
- `fathom arm off --for D --sessions N [--note]` is the counterfactual program's OFF switch: STAMPED (schema+fathom version, set-at, note), BOUNDED (unbounded OFF refuses), ANNOUNCED (doctor's `channels_env` and the session hook declare it), RECORDED (session-ledger `arm: on|off` field).
- Addresses the O15 lesson that a silent kill switch is a measurement bug.
- `fathomChannelsDisabled()` composes both switches; hooks treat them identically. State at `~/.fathom/arm.json` (0600); `fathom arm on` ends it early.
- Pinned by TestArmStamping (bound refusal, session countdown to expiry, wall-clock expiry, clear, kill-switch composition).

**Version-stamped telemetry: every record names its producer (IO-11)**
- New `internal/buildinfo` package (VCS revision + -dirty, "unknown" floor, never empty) stamps call-log rows (`schema_version 2` + `fathom_version`), hook-beat meta, session-ledger rows, and daily rollups.
- Hook-beat meta also gains `project_dir`, the per-repo attribution the user-global beat log lacked; caller keys win on collision, caller maps never mutated.
- Legacy rows without the fields read as version-unknown, never backfilled or guessed.
- The 251,834-catch-headline class of cross-semantics aggregation becomes a filterable query instead of an investigation.

**Registry surgery: two verbs, one-root-one-service enforced (O1, IO-1, IO-2, IO-3)**
- `remove_service` (idempotent every-row deletion, claim_events cascaded through claims) and `rename_service` (atomic re-key across eleven service_id tables, typed refusals, claim ids stay minted as historical evidence) both delegate store to client to AutoRefresh to tool and bust the identity cache on success.
- The ingest handler now enforces ValidServiceID at the daemon (the geoffmiller-cloud twin was re-fed for seven weeks past the client fix), plus EvalSymlinks canonicalization, duplicate-root and nested-root refusal.
- With IO-1's deterministic resolution, every O1 fix (a)-(d) is now landed.

**Adoption gauge: scoped to any repo, agent-surface aware (O6, IO-10)**
- `instrument --adoption` gains `--service` (case-insensitive filter; unattributed rows excluded AND counted beside the filter, never silently dropped) and cwd-independence, reusing doctor's call-log resolution when the relative default log is absent.
- O6's headline defect closes: the zero-call list is partitioned into "zero-call AGENT-surface tools (actionable)" vs "registered but not exposed" (different remedy).
- New `mcp.AgentSurfaceTools()` exports the allowlist the partition scopes against.
- Pinned by TestAdoptionServiceScoped.

**compile --dry-run: cost as an honest range (O8, IO-8)**
- Dry run once reported input-only tokens and `cost_usd: 0.000000` for a run that actually cost $0.64 and 21 minutes (output was 87% of tokens).
- Now carries an estimate band (EstTokensOutLow/High = input x [2, 8], observed obligations layer ran ~6.7x); human rendering prints tokens-in as the bytes/4 heuristic it is, output as unknown-with-a-band, cost as a labeled range, duration band at ~100 output tok/s.
- Real runs print exactly as before.
- Pinned by TestDryRunEstimateRange (positive ascending cost band, no LLM configured needed).

**Explicit-empty bindings/guards accepted as valid (O7, O22, IO-7)**
- O22's fix accepted the ABSENT field but kept `bindings: []` a loud error; the fleet's activate skill, kit packs, and review fix-pass all standardize the explicit literal, so doctor FAILed an adopter file fathom's own tooling prescribes.
- Now one vocabulary: `bindings: []` and `guards: []` load clean and pass ValidateStrict (schema `[_, ...]` constraints dropped in schema/binding + schema/guard); only present-and-malformed entries fail.
- Live proof: platform-engineering's standing `binding_file FAIL` is now PASS.
- Pinned by TestExplicitEmptyBindings / TestExplicitEmptyGuards; prior rejection-asserting negatives inverted.

**review --budget-seconds bounds the advisory channel (O9, IO-9)**
- Root cause: nothing prompts and nothing hangs, a six-file pre-push review measured 22.6s and lefthook buffers output until completion, so scripted pushes read the silence as a stall and killed it at their caps.
- `fathom review` gains `--budget-seconds N`: the whole body races the deadline and on excess prints a named advisory skip, exits 0.
- Flag requires `--no-persist` since a run that can be cut off must never be a store writer; pre-push step now passes `--budget-seconds 20`.
- Pinned by TestReviewNonInteractive. Follow-up filed in O9's fix note: why a six-file review costs 22.6s at all.

**doctor: lefthook gates recognized through variable indirection (IO-5, O23)**
- `checkLefthookGates` no longer requires the literal substring "fathom verify"; `lefthookInvokes` matches literal, path-suffixed, or shell-variable spellings (`"$FATHOM" verify`, `${FATHOM} verify`).
- Closes the audit's live false negative where a correctly-gated repo (platform-engineering's pre-push loop) was told to add a gate it already had.
- The audit's other two doctor findings (`binary_fresh` HEAD comparison, `hooks_firing` repo-relative call log) were already fixed at HEAD by the O23 wave; the audit ran a stale repo binary, the deploy-skew lesson the reconcile design exists to close.

**Plan grammar: quoted braces preserved, unparseable blocks loud (O4, GI-7, IO-6)**
- `parseAnnotation`'s brace pairing is now quote-aware: a quoted value containing the notation's own vocabulary `{symbol, package}` no longer terminates the block, so the row keeps its author-owned id and full contract (TestParseAnnotation_BracesInQuotedValues).
- GI-7's trailing-block rule is preserved: pairing runs over outside-quote braces only, last close wins.
- A brace block that parses to zero keys but contains contract keywords now surfaces as `Task.AnnotationSuspect`, and LintPlan names it "VOID" instead of silent md-NNN downgrade (TestLintPlan_UnparseableAnnotationIsLoud).

**Session hook: enumeration failure is not "not indexed" (O1, O2, IO-4)**
- `fetchServices` now returns an error alongside the map, with one retry on its own 4s timeout; every failure mode (transport, non-2xx, unmarshal) becomes an error instead of an indistinguishable empty map.
- Session hook grew the third branch O2 asked for: a healthy daemon whose `list_services` fails renders a degraded notice naming the failure, never the confident false "not an indexed service" instruction.
- A genuinely empty index now says "none yet" instead of the dangling "Indexed services: ." join artifact.
- Hook's inline candidate dedup now uses mergeCandidates (the O1-c fix). Pinned by TestSessionHookDegraded and TestSessionHookZeroServices.

**observe race window before foreign-file refusal**
- `fathom observe`'s foreign-file refusal now waits the same 200ms `reReadAfterLostCreateRace` window before concluding a file is genuinely foreign, instead of misreading a concurrent creator's O_EXCL-create-to-header-write window as a foreign file.
- Previously the loser refused and lost the entry (exit 1); `TestObserve_ConcurrentFirstCreationLosesNothing` now green at `-count=10 -race`.
- Found by the impact-observatory activation gate run, not by a user losing an observation.

**Deterministic service resolution and dedup (O1 a-c, IO-1, IO-2)**
- `resolveFromServices` no longer picks duplicate-corpus-root winners by Go map iteration order, which had zeroed obligations on 18/50 measured edits; winner is now chosen by richness (symbol count), then basename affinity, then lexicographic order, pinned by `TestResolveFromServicesDeterministic` (200-iteration determinism per case).
- `candidatesWithDaemon`'s dedup is extracted to `mergeCandidates`, which drops only exact name+root duplicates so a name-equal candidate at a different root survives (`TestMergeCandidatesNeverCollapses`).
- O1 fix (d), daemon-side duplicate refusal, lands separately with plan row IO-2.

## [0.3.0] -- 2026-07-30

**What this release is.** The rearchitecture and the proof. The store
went pure-Go SQLite and DuckDB was deleted outright (ADR-025); the review
verb, findings-as-claims, semantic layers, expert modules and the layer
catalog landed; the MCP surface split into a curated 16-tool agent surface
over an expert registry (ADR-014); the deterministic SDD reconciler,
`fathom plans`, and the done-means-proven gate went live in the repo's own
hooks; and the benches were run honestly -- the lookup x10 claim retracted
under its pre-registered bar, then fathom winning its home turf 13/13 vs
11/13 at 0.570x tokens. Detail: `docs/changelog/0.3.0.md`.

### 2026-07-24

**T5-8 second bench swing, fathom wins home turf (O10, O12, O15)**
- Home bench (H2-1 criterion): PASS, success 13/13 vs grep 11/13, tokens 1,594,762 vs 2,795,948 = 0.570x, turns 47 vs 72; fix wave drove it, `read_source` (T5-4) cut H1-T13 from 601,132 tokens/16 turns to 106,954/4, DocChunk `end_line` (T5-5) flipped H1-T08 to a pass.
- Lookup bench (B1's <5x bar): the x10 claim retraction STANDS; 15/15 vs 14/15 success, tokens 0.511x = 1.96x reduction (was 1.38x), still under the pre-registered 5x bar.
- Pre-run corpus revision committed first (8a847d3); bench runner's fathom-arm allowlist gained `read_source` to match the live 16-tool `/rpc/agent` surface.
- `list_services` now returns each service's recorded corpus `path`; session/edit/stop hooks resolve service identity by matching project directory against those paths, fixing O15 where obligations injection was silently dead on the dogfood repo for six days.

**Citation drift re-anchoring and eviction visibility (T5-1, T5-2, O13, O14, ADR-017, OP-6)**
- O13 fixed: `core.ClaimLink.AnchorText` is now persisted alongside the hash by both live "cites"-link writers; `semantic.Sweep` re-anchors a drifted citation when its `AnchorText` resolves to exactly one new location, appending a `ClaimEvent{Verdict:"re-anchored"}` audit entry.
- Fleet re-sweep (T5-1b): 340 claims checked, 73 wrong citations found (all fathom-service), 0 healed / 73 staled since no existing claim carries `AnchorText` yet; healing activates on the next fleet recompile.
- O14 fixed: `obligations` reports `displaced: N` when `MaxObligations` drops served-eligible claims, and every served rule carries a `layer` label; the scope-mapping half is filed as `prompt.PENDING-scope-precision.md` pending a deliberate flag-day alongside OP-6.
- Once deployed, ~73 fathom obligations begin serving `[stale]`-labeled fleet-wide, an expected honesty event, not new breakage.

**read_source tool, DocChunk line offsets, symbol_context snippet (T5-4, T5-5, T5-6, ADR-014)**
- New `read_source` MCP tool (`{service, file, start_line?, end_line?}`), bounded/line-numbered/read-only, added to the 16-tool agent surface; capped at 200 lines / 16000 bytes, refuses absolute paths, traversal, out-of-corpus files, and binaries; answers H3-1 where recovering cited text cost 16 turns / 601k tokens.
- DocChunk gains an additive `EndLine` field threaded through fusion, SQLite schema (`end_line` column + migration), and serving paths; `MarkdownExtractor.Extract` now computes each heading's real section extent, closing the H3-1 T08 class of guessed cited lines.
- `symbol_context` gains an inline `snippet` field (source, line-numbered, capped at 40 lines), with `snippet_truncated:true` when capped and `snippet:false` to opt out; honest-absence when source isn't retrievable.
- Hardening round closed three proven holes: symlink containment via `filepath.EvalSymlinks`, a 2MB `os.Stat` ceiling plus 16000-byte line truncation, and enforced `.fathomignore`/skip-dir/`.env` refusal reasons; all "15-tool" doc references updated to 16 with `read_source` named.

**Home-turf bench run: fathom does not win its own ground (H3-1, O13, O14, SS-8)**
- Wave-3b home-turf context-bench (13 index-native tasks x 2 arms, 26 live `claude -p` runs) failed both halves of the H2-1 criterion: success 12/13 vs 12/13 (equal, not strictly greater), tokens 1.071x grep's (7% premium where <=1x was required).
- Fathom alone failed H1-T08 (DocChunk granularity can't pin a sentence-start line); grep alone failed H1-T13 (quoted an ADR identifier outside the required line range); fathom cheaper on 8/13 tasks.
- Run's product findings filed as dogfood O13 (sweep-missed citation drift) and O14 (top-6 serving cap silently evicts previously-served claims after recompile).
- Pre-run re-derivation found the recompile's three-layer blend silently drifted T01/T02's citations and SS-8's DuckDB deletion broke T06/T07/T13's targets outright.

### 2026-07-23

**DuckDB backend and CGO deleted outright (ADR-025, SS-8, XT-6b, ADR-013)**
- SS-8's deferred trigger fired after a full green day on SQLite; `internal/store/duckdb.go` (`DuckStore`, `go-duckdb/v2`) and its test file are deleted outright, no DuckDB code remains in the repo.
- `migrate-store` removed rather than kept, since reading a pre-SS-8 store now needs a pre-SS-8 build; shared row scanners moved into `sqlite.go`.
- `store.BackendForPath` now refuses `.duckdb` loudly with a typed error naming the file, instead of a silent fallback; empty-path in-memory convention now opens `SQLiteStore`.
- `go mod tidy` removed `go-duckdb/v2` and its Arrow-only indirect deps; `internal/store` now builds/tests with `CGO_ENABLED=0`, while the full binary still needs CGO solely for tree-sitter (ADR-013), a separate not-yet-triggered removal (XT-6b).

**The review verb: review_diff and fathom review (Phase C, ADR-016, ADR-018, FT-4, FT-6, FT-7)**
- C1-1: new `review_diff` MCP tool composes diff impact, scoped obligations/findings, architecture conformance, and spec coverage into one cited verdict; CLI mirror `fathom review --store DB --service S --since <ref>`; arch-conformance edge normalization and rule-pack resolution shared via new `internal/analysis/archconformance` and `internal/projectrules` packages.
- C2-1: review findings persisted as content-addressed claims on the `findings` layer (verified tier, deterministic, fires FT-7's pre-armed trigger); refutation durability proven at three layers of tests.
- C2-1 write-path fixes before release: real cited-line hashes prevent sweep from falsely flipping 57 review claims to stale/contradicted; only deterministic-oracle dimensions persist (never `obligations` or `impact`); `--no-persist` flag added and used by the pre-push hook; reconcile supersedes fixed findings; verified-tier admission recorded in layer.cue (ADR-018/ADR-016 M1).
- C3-1: `lefthook.yml` pre-push gains a non-blocking `fathom review --no-persist` summary step (~12s worst case on a 1777-file diff); `review_change` MCP prompt now calls `review_diff` directly instead of an 8-step manual sequence.

**Serving-layer hygiene fixes (Wave 3a P1-6, P1-9, O11)**
- O11 closed: `migration_targets`'s caller-supplied `deprecated_pkgs` list now merges with the built-in default list (deduplicated, deterministic order) instead of silently replacing it, matching the tool's documented behavior.
- Sweep event vocabulary fixed: the post-compile sweep's `claim_events` verdict for anchor-dead claims is now `"contradicted"` (matching `ClaimContradicted` status) instead of `"refuted"`, reserving "refuted" for deliberate human/agent judgment.

**Pulled-tools excellence: O12's product gaps and C4-4 method-edge gap (P1-1..P1-5, P1-8)**
- P1-1 closes O12(b): `find_symbol`, `query_symbols`, and `references` now accept `symbol` as an additive parameter alias, erroring only when both native and alias values disagree.
- P1-2 closes O12(a): `symbol_context` and `find_callers` gain `cross_service_callers`/`cross_service_callees` fields when a fleet backend is wired, additive and never merged into same-service results.
- P1-3 closes O12(c): `find_symbol`'s case-insensitive tier now returns a package-match group alongside the name-match group so a package-naming query is no longer hidden behind an unrelated same-named symbol.
- P1-4 closes C4-4: `arch_conformance` edge endpoints are normalized to their declaring Go package so package rules catch method-origin and method-target edges, not just function-to-function ones.
- P1-5 re-run: B1-T09 (the O12 flagship case) improved from 504,785 tokens/20 turns to 321,100/9 turns (6.6x to 4.2x vs grep), still 4.2x more than grep, tracked as residual under O12. P1-8: dispatch now enforces declared `required` fields, so `license_policy`, `spec_coverage`, and `spec_status` refuse with -32602 when a required field is missing instead of silently running empty.

**Spec-driven development adoption guide (AD-3)**
- Added `docs/adoption/spec-driven-development.md`, the third and final adoption guide alongside `claude-code.md`/`codex.md`.
- Documents the full SDD loop: author a plan in `{id, symbol, package, ...}` grammar, implement, `fathom verify` proves rows, pre-push gate blocks over-claiming, computed progress, archival lifecycle, `spec-bindings` layer binding, oracle classes.
- Written against this repo's own live adoption program (`docs/plans/activation.md`, `docs/plans/adoption-docs.md`) with real `fathom verify`/`gen-plan-cue` output.
- `fathom plans status`/`next`/`archive`/`restore` documented from committed design and marked "(landing this wave)", not yet built in this worktree.

**Wave-1 adoption-friction fixes (ADR-014)**
- `task build` wrote to `workspace/tmp/fathom` while hooks and daemon-install expected `bin/fathom`; `Taskfile.yml`'s `build` now emits `bin/fathom`, with the fast inner-loop variant renamed `build-dev` (still `workspace/tmp/fathom`).
- `docs/use-cases/getting-started.md` documented the removed `set_profile` tool, retired by ADR-014's surfaces-week1 amendment; rewritten against `docs/mcp-tools.md` covering path-scoped HTTP surfaces, `off_surface:true` gate semantics, and the real `capabilities` response shape.
- `docs/usage.md`'s CLI reference stopped at `session-hook`; added entries for thirteen shipped verbs (`init`, `compile`, `layers`, `verify`, `config`, `providers`, `smoke`, `instrument`, `edit-hook`, `stop-hook`, `claim`, `remap`, `report`), flags verified against `cmd/fathom/*.go`.

**`fathom plans` (E3/E4), reverse-drift report (E6-1), verify_plan SLO (B3-1)**
- `fathom plans status` (E3-1): per-plan rollup (proven, claimed-unproven, open, deferred-with-trigger, skipped-other) computed fresh from the verify engine, `--json` plus a human table.
- `fathom plans next` (E3-2): ordered work frontier including claimed-but-unproven rows as `claimed-failing`/`claimed-unknown`, ordered by plan file then document order.
- `fathom plans archive`/`restore` (E4-1): governed lifecycle requiring all rows proven or skipped, task IDs absorbed into CHANGELOG.md/ADR.md, and a fresh re-run; writes a tombstone and `git mv`s the plan to `docs/plans/archive/`.
- Pre-push/CI gate scoped to active plans (E4-2): `lefthook.yml` and `scripts/ci.sh` walk only the top level of `docs/plans/`, so archived plans drop out of gate scope.
- `fathom report --analyzer reverse-drift` (E6-1) computes ungoverned public API surface per service; `verify_plan` p95 SLO (B3-1) set at 5s with a Stop-hook timeout degrading to advisory, never a hard block.

**context-bench and the x10 tokenomics retraction (B1/B2-1)**
- context-bench built (B1-1/B1-2/B1-3): 15-task committed corpus with grep-derived ground truth, tested scorer (`internal/contextbench`, 16 tests; `cmd/contextbenchscore`, 7 tests), two-arm `claude -p` runner comparing fathom-MCP-only vs grep/find/cat-only.
- The x10 tokenomics claim is RETRACTED (B1-3/B1-4): first full run measured equal task success (15/15 both arms) at 1.38x aggregate / 1.79x mean per-task token reduction and 2.81x mean wall-time reduction, below the pre-registered <5x retract criterion.
- All docs stating x10/<=1/10th updated to measured magnitudes.
- B2-1 existence-check wiring: `scripts/check-surface-traffic_test.sh` proves ok/zero-traffic/missing-dependency verdicts against a local fixture server; law-8 adoption clock explicitly not started.

### 2026-07-22

**Relative corpus paths at ingest boundary (dogfood O10, ADR-020)**
- Typed edges were silently absent for relative `--path` (CLI default `.`): the connector-scope filter rel-mapped absolute file positions against a relative `repoPath`, excluding every package with zero edges and zero failures.
- `EnrichTypedEdges` now normalizes `repoPath` to absolute at entry; added a never-silent stderr guard and typed-edge counters in `cmd/fathom ingest` output.
- `ServiceMeta.Path` was recorded verbatim-relative, causing wrong-corpus refresh from the daemon's working directory; now recorded absolute, with both repos' post-commit hooks passing `--path "$PWD"`.
- Added `scripts/migrate-fleet-store.sh`, the executable ADR-020 DuckDB-to-SQLite runbook, decoupling the daemon from post-commit rebuilds and stamping the SS-8/XT-6b bake clocks.

**Tree-sitter extraction engine via wazero/WASM (ADR-021 s4, XT-1..6b)**
- `internal/extractor/tsengine`: a pure-Go tree-sitter execution engine, C core + one grammar's parser.c/scanner.c compiled to `wasm32-wasi` and run via `wazero`, no CGO in the engine and no Docker at runtime.
- `NewTSExtractor` (XT-1) implements `extractor.Extractor` for javascript/python/ruby via real tree-sitter `.scm` queries; qualification handles "Class.method" and Ruby's "Outer::Inner::Widget".
- Pathological-input bounds (critic B1): `({[` x 50,000 previously ran 52.9s unbounded vs ~49ms on the CGO incumbent; now bounded via guest-side parse budgets, a host-side hard backstop, and parser re-creation on fault, counted in `fusion.Report.TSEngineFaults`.
- Calibration (XT-2/XT-3/XT-4) on the 12-repo corpus: coverage never regressed; JS/TS gains largest (lodash 0.41 to 1.26); Python/Ruby tie their CGO incumbents; corpus ingest measured at 1.69x the incumbent, exceeding the plan's 1.5x line, flagged for owner gate call.
- `fusion.ExtractorEpoch` bumped 4 to 5, forcing re-extraction; `capabilities` MCP tool gains `extraction_engines` (XT-5); binary-wide CGO deletion (XT-6b) remains PENDING on a 7-day green bake, per ADR-013.
- Refs: ADR-020.

**Typed Go call edges resolve O9 ambiguity residue (ADR-021 s3, CT-4..6)**
- `internal/fusion/typedcalls.go`'s `EnrichTypedEdges`: additive Go-only pass via `go/types` producing `EdgeDirect`, `EdgeIfaceDispatch` (one edge per in-scope implementer), and `EdgeUnresolved`, scoped to `.fathomignore`-respecting Discover set with per-package caching and reconcile.
- `core.Edge` gains `Kind`; new store surface round-trips typed edges through SQLite, DuckDB, and the memory fake, contract-tested.
- verify (CT-4): `test_reaches` now verifies paths crossing interface-dispatch edges, labeled "reached via interface dispatch (N implementer(s))"; three real O9 residue shapes proven flipping from refusal to typed reach.
- Serving (CT-6): `blast_radius`/`diff_impact` traverse typed edges; `find_callers` adds `typed_callers`, `symbol_context` adds `typed_edges`, `server_stats` reports per-kind typed-edge counts.
- Performance gate (CT-5): first-run typed load measured against a true syntax-only baseline; cached re-ingest is delta-only.

**Tool identity critic round fixes (ADR-021 s2, F1-F6)**
- ToolKind/Generated were write-only against real stores (F1, HIGH): sqlite/duckdb column lists dropped them while the memory fake round-tripped them, so asserting tests passed against a lie; both columns now persist in both backends and are covered by the shared store CONTRACT suite.
- CRD tagging is now per-document (F1e/P15) via `core.SymbolNode.ToolKind`, fixing a multi-doc YAML stream that tagged every observation "k8s-crd" file-wide.
- Guard-wrapped Helm templates extract (F2): whole-line `{{- if ... -}}` actions are dropped rather than substituted; measured 7 to 23 symbols on the 10-file demo corpus.
- image-Dockerfile keying fixed (F3): `*.dockerfile` files now key by stem instead of directory name; root-level lone Dockerfile falls back correctly.
- IsGenerated (F4) accepts Go's "Code generated ... DO NOT EDIT." marker anywhere before the package clause (bounded 4KB); compose signal and classified-but-zero visibility fixed (F5) with new `classified_zero_files` counters; docs updated (F6) to clarify `server_stats` counters are event counters since process start.

**Tool identity classifier for yaml/cue tooling (ADR-021 s2, LT-4, TI-9/10)**
- Pass-3 tool-identity classifier (`fusion.Classify`) content-classifies ambiguous yaml/cue files into a `ToolKind` (k8s, CRD, compose, helm-template-yaml, kustomize, openapi, cue-module, timoni, unclassified) using a per-ingest sibling index.
- `HelmTemplateYAMLExtractor` substitutes `{{ ... }}` action spans with placeholder scalars before YAML decode, recovering document shape from previously-failing helm charts; `KustomizeExtractor` and OpenAPI/Swagger extractor added, previously silently zero-output.
- Compose files recognized by content (top-level `services:` key) regardless of filename, not just four hardcoded names.
- CUE module/timoni awareness tags `.cue` files under `cue.mod/` ancestors, layered on the existing LT-4 CUE extractor; generated-file detection adds `Generated` flags and a `cloc` bucket.
- `fusion.ExtractorEpoch` bumped 3 to 4 forcing re-extraction; Selector-Service and cross-repo image resolution deferred (TI-9), jsonnet/.ipynb extraction deferred (TI-10).

**Pure-Go SQLite store backend replaces DuckDB (ADR-020)**
- Added `internal/store/sqlite.go`: `SQLiteStore` (`modernc.org/sqlite`) implements the full 26-method `Store` interface with fresh final-column schema, WAL pragmas verified per-connection by `TestSQLite_WALActuallyEngages`.
- `TestSQLite_ReaderDuringWriter` proves cross-process concurrency, a reader completes a `SELECT` while another connection holds an open write transaction, unlike DuckDB's cross-process lock.
- `store.BackendForPath` dispatches `.duckdb`/`.sqlite`/`.db` replacing four hardcodes; `PlatformStore` loads mixed-backend fleets; same-stem duplicates resolve to `.sqlite` with a loud warning.
- `fathom migrate-store --from X.duckdb --to Y.sqlite` ports the eight non-derivable tables read-only with per-table row count + verified content spot-hash; deferred DuckDB removal tracked as SS-8.
- CLI `--store` defaults flip from `workspace/db/fathom.duckdb` to `workspace/db/fathom.sqlite` across all commands; added dependency `modernc.org/sqlite v1.52.0`.

**Language truth: one registry, binary detection, honest CUE (ADR-021 §1)**
- Deleted `pkg/client.langFromPath`'s stale 10-language table; `internal/connector.extClassMap` is now the single ext-to-language source, parity-tested against `fusion.New`'s extractor registrations via `TestLanguageRegistryParity`.
- `cloc` reports an explicit `unclassified` bucket instead of a silent `other`; lockfile heuristics with no extractor (`go.sum`, `Gemfile.lock`, `Cargo.lock`, `requirements.txt`) now honestly report `unclassified`.
- `connector.IsBinary` sniffs for a NUL byte in the first 8000 bytes; fixes a 423K-line "other" bucket that was actually an untracked 103MB `fathom` binary.
- CUE extraction v2 recurses struct literals for nested field names, reads `Package` from the actual `package` clause, and captures imports as `core.KindCUEImport`; `fusion.ExtractorEpoch` bumped 2 to 3 triggers a one-time full re-index per service.
- Fixed root-anchored `.fathomignore` patterns (leading `/` was a silent no-op) and a CUE malformed-import crash in `literal.Unquote`.

**Build fathom without agentic-kit access (ADR-019)**
- Added `third_party/agentic-kit/`, a committed full mirror of the private kit tree at the pinned commit; `go.mod`'s `replace` points there while `go.work` still wins for local co-development.
- `task sync-kit` (`scripts/sync-kit.sh`) re-archives the sibling checkout at the pinned commit; `task vendor-check` (`scripts/vendor-check.sh`) content-hashes the mirror against a fresh archive and is now part of `task validate`.
- Air-gapped `go mod vendor` now succeeds standalone; new `docs/distribution.md` documents the build path.
- Size note: the pinned commit archives to ~13MB (1,153 files) including an accidental ~4.1MB Mach-O binary (`kit-new`), archived faithfully and flagged for the kit maintainer to prune.

### 2026-07-17

**Sextant back-compat surface removed (ADR-015)**
- Owner overrode the original 2026-07-07 "kept by design" back-compat decision and asked for all of it removed, not deprecated.
- Removed `withNameAliases` sextant/fathom service-name aliasing from `session-hook`, `edit-hook`, `stop-hook`, collapsing each hook's double lookup to a single lookup.
- Removed `internal/connector.LegacyIgnoreFileName` (`.sextantignore`) fallback; only `.fathomignore` is honoured now, covered by `TestLocalConnector_Discover_NoLegacyIgnoreFallback`.
- Behavioral consequence: stores ingested under service name "sextant" and repos relying on `.sextantignore` are no longer recognized; re-ingest or rename required.

**Layer catalog: prompt-as-data, pack shape, adr-rules + spec-bindings (LC-1..8,13, OP-6)**
- LC-1: every `#Layer` carries `prompt_file`; `promptForLayer` is generic over `LayerDef`, removing the silent obligations fallback for an unwired layer name.
- LC-2: `serving.drift_tracked` schema field replaces the `kind=="finding"` Go hardcode at all three remediation-drift sites; LC-3 unions embedded packs with a project's `.fathom/layers/`, LC-4 migrates built-ins to `layer.cue`/`prompt.md`/`eval/`/`KILL.md` pack shape.
- LC-7 adds `fathom layers list|show|install`; two new embedded packs `adr-rules` and `spec-bindings` each cleared precision 1.00 / recall 1.00 live against `claude-cli`.
- LC-8: a third-layer end-to-end proof surfaced and fixed a real drift-tracking resolution gap for project-local packs.
- Hardening: a broken project pack no longer breaks the union (warned, surfaced via `LoadError`); LC-13 (real `layers uninstall`) deferred; obligations layer re-run recorded recall straddling its bar (0.71/0.86), escalated as OP-6.
- Refs: ADR-018.

**Configuration experience: profiles, ~/.fathom/config.toml, health, smoke**
- Added `internal/config`: TOML-authored, CUE-validated runtime config merging user (`~/.fathom/config.toml`) and project (`.fathom/config.toml`) layers field-by-field with per-field `Provenance` tracking.
- `#Profile` is a closed CUE struct with no raw `api_key` field; only `api_key_env` or `api_key_cmd` (argv-exec, no shell, 10s bound) supply credentials.
- Added `fathom config [--explain]`, `fathom providers [--profile X]` (auth-health probe via `llmclient.Pinger`), and `fathom smoke --profile X|--all` (one real completion, cost estimate, requires explicit selection).
- `serve-http` now loads `[daemon]` config at startup, closing the gap where `compile_layer` was silently disabled on the launchd daemon with no env; AWS GovCloud and Azure Government ship as profiles.
- Review hardening: modelless bedrock profile fails fast, `compile --profile` without `--store` errors naming the constraint, TOML/CUE errors carry line/column and grouped keys.

**Provider matrix: openai, azure-openai, bedrock, codex-cli**
- Rounds `buildLLMClient` out to six `FATHOM_LLM_PROVIDER` lanes, adding `openai`, `azure-openai`, `bedrock` (full AWS credential-chain resolver, no static keys required), and `codex-cli`.
- `codex-cli` lane also Pings the constructed client before returning it, since undocumented config keys could silently drop system-prompt mapping and revert to the CLI's ~13k-token built-in persona.
- CLI `--store` mode no longer hardcodes the Anthropic model string into `WithLLM`; both construction sites now share `compileModel()`.
- Docs add a provider matrix table including the `codex-cli` cost line, roughly 10k input tokens of ambient overhead per call (~300k for a 31-section compile), ~60% cache-discounted steady-state.

**Subscription-backed compile provider via Claude CLI**
- `FATHOM_LLM_PROVIDER=claude-cli` shells out to a locally logged-in Claude Code CLI instead of `ANTHROPIC_API_KEY`, via new `agentic-kit` `foundations/llmclient/claudecli` package.
- `cmd/fathom/compile.go`'s single construction site renamed `buildLLMClient`, switching on the provider; `FATHOM_CLAUDE_BIN` overrides the resolved `claude` binary path (e.g. for `serve-http`'s launchd daemon).
- Reported `cost_usd` is the same API-equivalent accounting under either provider, but actual marginal spend under `claude-cli` is $0 (subscription plan limits, not a metered key).
- `cmd/fathom/compile_test.go` covers `buildLLMClient`'s switch: default/explicit anthropic without a key, claude-cli construct plus one real `Complete` through a hermetic fake-shim, unresolvable `FATHOM_CLAUDE_BIN`, unknown provider value.

**Findings-as-claims tracer bullet (D3, FT-1..FT-6)**
- Kind:"finding" `#Layer` instance added alongside `obligations`; `internal/semantic.Compile` becomes layer-aware, resolving name/kind/tier/tier_profile/prompt from `Compiler.Layer` (FT-1).
- `pkg/client.CompileLayer(ctx, service, layer, dryRun)` generalizes `CompileObligations`; fixes finding F7 where `cmd/fathom compile --layer` and the `compile_layer` MCP tool silently discarded the requested layer name and always compiled "obligations".
- `semantic.ObligationsFor` now reads claims across every compiled layer, and served `Obligation` gains a `kind` field (FT-2).
- `fathom claim refute|corroborate --store DB --service S --claim ID --note "..."` wires the previously-dead `claim_events` machinery; `refute` flips status to `ClaimRefuted`, `corroborate` is provenance-only (FT-3). `PutClaims` retains `ClaimRefuted` status across upsert of the same `claim_id` (FT-4).
- `fathom report --store DB --service S --analyzer security_rollup --out findings/` writes deterministic markdown findings, verified by double-run byte-compare, excluding its own `--out` directory from the scan (FT-5); remediation drift tracked via `ClaimLink{Rel:"scope-fp"}` fingerprints and `semantic.DriftedFindings` sets `code_drifted:true` at serve time (FT-6), tied to O8 precision work's compile guard on bare `**` scopes.

**O8 precision: serve-time trust (OP-1..OP-5)**
- Prompt v3 forbids bare `**`/repo-wide catch-all scopes, requiring file-class scoping; `PromptVersion` rotates to `obligations/c02b1aa1`, forcing full recompile under O8.
- Over-broad-scope guard in `internal/semantic/compile.go` `verifyDraft` rejects a claim whose scope glob matches more than half of `IndexedFiles`, counted in `ClaimsRejected`; single-file-class extension globs (`**/*.go`, `docs/**/*.md`) are exempted from the breadth check as a review fix.
- Edit-hook root refusal (`cmd/fathom/edithook.go` `cmdEditHook`): a file not relativizing under any resolved service-candidate root skips that candidate instead of querying the daemon with a raw out-of-repo path, fixing an O8 addendum; symlink resolution and `..`-escape checks added as review fixes.
- Spec-line injection appends `"▸ implements [ID] — defined at <file:line>"` for cited spec IDs via `spec_refs`, capped at 3, firing independently of obligations under a shared ≤150-token budget (OP-1..OP-5, OP-6 recompile).
- New `plan` project skill (`.claude/skills/plan/SKILL.md`) scaffolds gate-ready grammar-v2 plans with stable ids, `{symbol, package}` contracts, typed dispositions, and the `fathom verify` command.

**Coding-agents canonical doc and staleness sweep (IA-4/IA-5)**
- `docs/use-cases/coding-agents.md` becomes THE adoption guide covering the loop, hooks, gates, CLAUDE.md/AGENTS.md doctrine, agent-surface registration, and the honest cost story.
- `usage.md` `set_profile` guidance retired; June `claude-code-integration.md` banner-superseded (assessment content kept); `getting-started`/`coding-context` scoped and pointed at the canonical doc.

**`fathom init`: one-command coding-agent wiring (IA-1/IA-2/IA-3)**
- `fathom init` wires a repo end-to-end, idempotently and additive-only: creates `.fathomignore` if absent, calls the daemon's `ingest` MCP tool and confirms the service is indexed, fail-soft exit 2 when the daemon is unreachable.
- JSON-merges project-level `.claude/settings.json` hooks (SessionStart `session-hook`, PostToolUse `edit-hook`, Stop `stop-hook`), writes the fathom operating-contract stanza into CLAUDE.md and AGENTS.md between `<!-- fathom:begin/end -->` markers.
- Every mutation is create-if-absent or marker-replace, so a second run changes zero bytes and reports "already present".
- `mergeHookSettings` preserves every unrelated top-level key, hook event, and sibling hook entry; detection is by command substring so re-running never duplicates an entry.

**MCP surfaces week-1: execute-and-flag, agent surface (amends ADR-014)**
- Gate semantics change from hard-gate to execute-and-flag: an off-surface `tools/call` or `batch` sub-call now executes instead of erroring, logged with `off_surface:true` in the JSONL call log; old hard gate stays behind `Server.strictSurfaces`.
- The `companion` allowlist becomes the 15-tool **agent** surface (minus `batch`/`brief`, plus `verify_plan`/`capabilities`), served at `POST /rpc/agent` with `POST /rpc/companion` kept as a compatible alias.
- `set_profile` removed entirely, amending ADR-010/014's stdio clause, since stdio now always serves the full registry.
- Four zero-call alias tools deleted (`platform_callers`, `test_debt`, `api_routes`, `deps`) in favor of canonical `cross_repo_callers`, `markers`, `routes`, `dependencies`; `serve-http`'s default listen address changes from `:7575` to `127.0.0.1:7575` (loopback).

**Gap-1 channel instrumentation: denominator, liveness, origin**
- Hooks beat a central log (`FATHOM_HOOKS_LOG`) via agentic-kit v0.7.0 telemetry/heartbeat: edit-hook seen/answered/error, session-hook fired, giving injection coverage a denominator.
- `fathom instrument` computes coverage (answered/seen vs the ≥90% gate), per-day rollup, and liveness alarms (sessions without edits in 7d; edits unanswered in 1d); advisory, exit 0 always.
- `X-Fathom-Origin` header lets every hook/CLI HTTP call declare its channel, recorded in the daemon call log as `origin` to separate plumbing from agent choice.
- Grammar hardening (GI-7): the plan-annotation parser takes the trailing brace block, so prose braces and code literals in titles no longer hijack contracts.

**M1: the manifest goes live (semantic layers, ADR-016 amendment)**
- `schema/semantic/schema.cue` becomes a runtime artifact: `internal/semantic.LoadLayer` embeds a byte-identity-tested copy and decodes `#Layer` instances via `cuelang.org/go`, retiring the hand-synced `ObligationsInclude` var and `maxRuleWords` const.
- Content-addressed prompt identity via agentic-kit v0.7.0 `prompttemplate`: `PromptVersion` derives as `obligations/<hash8>` from `Template.TextHash()`, so any prompt edit is automatically a new compile identity, preventing the bug class that left 27/32 sections stale on v1 claims after the v1→v2 bump.
- Typed compile failures via agentic-kit `llmclient/structured`, classifying truncation/malformed output as `structured.ErrTruncated`/`structured.ErrMalformed`, retiring the hand-rolled `errBadResponse` classifier.
- `enforceBudgets` (ADR-016 §4.1) refuses to start a compile whose pre-flight cost estimate exceeds `max_usd_per_run`, and stops mid-corpus when the running token total crosses `max_tokens_per_run`, recorded on the `compile_runs` ledger.
- Adversarial-review fixes: (F1) structured-outputs schema made object-rooted (`{"claims":[...]}`) for Anthropic compatibility; (F2) superseded/refuted claims no longer feed the digest gate; (F5) budget-stopped runs surface `budget_stopped`/`refusal_reason` at `compile_layer` and CLI printers. Spec `docs/plans/m1-manifest.md` (M1-1..M1-8, all landed); `#Layer.tier` widens to admit `verified` (ADR-018).
- Refs: O8.

**Extractor-epoch fingerprint salt fixes stale observations (O9)**
- The pass-3 hash-gate keyed fingerprints on file content only, so a binary with improved extraction (e.g. H1's aliased-selector canonicalization and method-edge forms) silently kept unchanged files' old observations, breaking `test_reaches` on genuinely-tested symbols (dogfood O9).
- Fingerprints now salted with `fusion.ExtractorEpoch`, bumped on any extraction-semantics change to force full re-extract on next ingest.
- `ObligationsIndexStale` adopts the shared `fusion.FingerprintHash` so the formulas cannot diverge; regression covered by `TestIngest_ExtractorEpochInvalidatesOldFingerprints`.

**SDD reconciler v0: plan grammar v2, wave gate, CUE shadow (ADR-018)**
- Plan grammar v2 (`internal/verify`, additive): `[~]` in-progress checkbox, author-owned stable `id:` keys, typed dispositions on `[-]` (`disposition: refused|deferred` + `reason:` + `trigger:`), `skip:` justification for `skip_test`.
- Wave gate `fathom verify --require-all` (`verify.GateRequireAll`) fails on open/in-progress tasks so a wave advances only when every task is claimed-and-verified or explicitly skipped.
- `fathom gen-plan-cue` emits a generated-only `#Plan` document and proves lossless round-trip against the Markdown parse; hand-authored plan CUE is deprecated.
- `fathom stop-hook` checks tasks newly claimed `[x]` this session against verification at session end; advisory by default, `--block` refuses session end with the failing citation.

**Obligations v2: first-fleet-compile hardening (ADR-016)**
- Prompt v1 to v2 and corpus narrowed to binding docs only (`CLAUDE.md`, `ADR.md`, `docs/plans/README.md`), after the first fleet compile turned `docs/design/**` essays into noisy `**`-scoped aspirational rules.
- Digest gate now includes `PromptVersion` after a prompt bump previously failed to recompile 27/32 sections still on v1 claims; covered by `TestCompile_PromptBumpDefeatsDigestGate`.
- Per-section failure resilience: a malformed/truncated model response now fails only its own section (tracked in `compile_runs.sections_failed`) instead of aborting an 88-section fleet corpus; `MaxTokens` raised 1200 to 2400.
- Claims whose subject left the corpus selector are now superseded on next compile rather than served forever.
- First full fleet compile under v2: 31/32 sections, 72 claims (9 rejected by compile-time verification), $0.047.

**Semantic layers v1: obligations tracer bullet (ADR-016)**
- Governed-claims substrate adds four additive tables: `semantic_claims` (content-addressed, tiered, status-tracked), `claim_links` (cites/scopes), `compile_runs` (token/USD ledger), append-only `claim_events` refute channel.
- Obligations compiler pipeline: governing docs to markdown sections, digest gate (unchanged sections cost $0), DLP redaction, injected LLM (strict JSON), compile-time anchor and scope verification, transactional store with supersede; rules word-budgeted at ≤28 words so a 6-rule payload stays ≤150 tokens.
- Citation sweep (post-ingest, token-free) re-hashes cited lines: drift flips status to `stale`, unresolvable citations to `contradicted`, restored docs heal to `fresh`; wired into `IngestService`.
- `obligations` MCP tool on the 15-tool companion surface serves rules ordered by scope specificity, capped at 6, with zero-output-on-clean; `compile_layer` tool and `fathom compile` CLI added, `dry_run` defaults true.
- PostToolUse injection via `fathom edit-hook` delivers ≤150-token obligations after every Edit/Write, silent on clean or error; precision eval harness gates the layer at precision ≥0.90 / recall ≥0.75, opt-in via `FATHOM_EVAL_LLM=1`.

**Fleet onboarding ergonomics (O7)**
- `ingest` tool onboards new services via optional `path` (absolute corpus root), ingesting a repo the index has never seen through the live daemon, no single-writer lock dance.
- Session-hook's "add this repo" guidance now names the exact call; capability forwarded through the auto-refresh wrapper (the O7 lesson) with a through-the-wrapper test.
- Subdirectory-rooted projects resolve their service: both hooks try the project dir's basename, then the enclosing git repo's basename, so `<repo>/deploy` no longer resolves to service "deploy".
- Edit-hook resolves file paths against whichever root matched; `reingest-fleet.sh` gains nested-path support plus three fleet repos (growing-gardens-bi, car-assistant, crossplane-platform).

**Done-means-proven verify gate (H1.2, O6)**
- Checkbox semantics: `[x]` in a plan is a claim; `verify.Gate` fails any claimed task whose contract fails (`--strict` also gates Unknown); `[ ]` open and `[-]` skipped never gate.
- `fathom verify` CLI is HTTP-first against the daemon, `--store` for CI; gate always computed locally so a stale daemon can't launder a claim. Exit 0/1/2, fail-open pre-push, fail-closed strict CI.
- Wired into lefthook pre-push and `scripts/ci.sh` here and in agentic-kit; first gated run caught 3 over-claims by the plan's own author (recall gap filed as dogfood O6).
- Gate honesty fix: an empty index now reads "cannot verify", never "all claims false" -- `fathom verify` probes the index first (`list_services` or symbol count) and exits 2 with a fix-it message when the service isn't indexed.

**Born-portable stores (ADR-017)**
- Repo-relative anchors from every connector, including LocalConnector; absolute refs from legacy stores still read.
- `fathom remap` repoints a service root in one metadata update.
- Watermark SPI (git HEAD or content-addressed `manifest:` hash) makes freshness domain-neutral, so non-git corpora (docs folders, findings exports) auto-refresh and sweep honestly.
- Ingest self-heal purges pre-ADR-017 absolute-anchored content wholesale so no symbol duplicates across anchor styles; contract test `TestStorePortability_RoundTrip` covers move corpus + remap.

**Adoption metrics fixed after rename**
- `fathom-adoption.sh` measured nothing for this repo after the rename because transcript dirs are path-keyed; fix adds name-era aliasing, per-ISO-week rollup, and a daemon call-log summary section.
- Measured baseline: 0% fathom share (0 vs 193 grep calls), the number the forced channels exist to move.

**Rename sextant to fathom (ADR-015)**
- Module path becomes `github.com/geoffmilleraz/fathom`, binary `cmd/fathom` moves to `bin/fathom`, MCP serverInfo/resource URIs/metrics prefixes, docs and scripts all renamed.
- Back-compat kept where the old name is data in the field: `.sextantignore` still honoured as legacy fallback behind `.fathomignore`, adoption metrics count both `mcp__sextant__`/`mcp__fathom__` prefixes, session-hook treats the two service/dir names as one identity, `reingest-fleet.sh` finds the repo under either directory name.
- Historical records (changelog, USAGE_JOURNAL.md, dogfood-observations.md) intentionally keep "sextant"; operator steps in `docs/rename-migration.md`.
- Motivated by fleet usage mining: sextant works when called but is dramatically under-used, agents reach for shell grep 10x+ more even where the index exists, because "index first" lived only in passive docs.

**Path-scoped tool surfaces on shared daemon (ADR-014)**
- `POST /rpc/{surface}` serves an immutable per-request view of the registry: `companion` is a curated 14-tool coding-agent hot loop; `coding`/`investigation`/`shared`/`platform` mirror the group profiles; bare `/rpc` remains full. Scoped endpoints cut tools/list schema payload ~80% for non-deferring clients (motivated by review of 86 tools ≈ 8.6k schema tokens).
- Surfaces hard-gate `tools/call`, closing the profile-bypass hole where a tool hidden from `tools/list` was still callable, including `batch` sub-calls; off-surface calls now error naming the surface and pointing at `POST /rpc`. Canned prompts scoped the same way.
- `set_profile` rejected over HTTP since the profile is process-global on the shared daemon; stdio keeps `set_profile` and gains `companion` as a valid value. `capabilities` and the manifest resource report `active_surface` alongside `active_profile`.
- Tool-call telemetry wired into the daemon via `serve-http --call-log` (default `workspace/logs/mcp-calls.jsonl`), recording request `surface`, individually logged `batch` sub-calls (`via:"batch"`), and atomic `O_APPEND` writes so a daemon kill can't merge lines.
- `session-hook` (Claude Code SessionStart) pings the daemon, maps project dir to service, and injects an active "resolve via sextant BEFORE Grep/Read" reminder, or an honest signal when the daemon is down or repo isn't indexed.

**Query staleness compaction, find_symbol fallback, schema slim**
- Query-side staleness is constant-size now (USAGE_JOURNAL.md ask #4): `staleness` block carries at most 3 example rows plus `stale_sources`, `total_sources`, `fresh_sources`, and a new `oldest_ingested_at` low-water mark; full list stays in `staleness_report`. Regression: `TestQuery_StalenessReport_CompactWhenManyStale`.
- `find_symbol` no longer punishes not knowing the exact identifier (USAGE_JOURNAL.md ask #3): matching degrades exact/key-suffix -> case-insensitive -> substring (capped at 20, `truncated` flag), with up to 5 `did_you_mean` suggestions on a full miss; response reports `matched_via`. Regressions: `internal/mcp/findsymbol_test.go`.
- `verify_plan`'s tool schema slimmed ~27% (~150 tokens), from ~554 tokens (~6% of the whole registry); `min_tier`'s valid values moved from prose into a JSON-schema `enum`. Behavior unchanged; full verdict reference in `docs/mcp-tools.md`.

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
