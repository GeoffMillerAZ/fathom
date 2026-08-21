# work frontier — packs, eligibility, leases, prime

> Governing source: `docs/vision/24-work-frontier.md` (this plan is its
> binding promotion). The dispatch boundary is normative: every row here
> serves facts; none of them watches, schedules, or launches anything.
>
> **Covenant (ADR-0070 class):** failing test first, `task validate`
> green before commit, dated CHANGELOG entry, boxes flip only on
> `fathom verify --service fathom --plan docs/plans/work-frontier.md`.
>
> Sequencing: WF-3/WF-4 want O4 (annotation-brace corruption) fixed
> first — a corrupted annotation silently drops the id a `needs:`
> reference points at. WF-1..2 (ctx packs) are independent and highest
> value. Every feature carries its pre-registered kill from vision 24.

## Packs — discovery attached to plan rows (vision 24 §2)

- [x] the plan grammar accepts a `ctx:` annotation key — space-separated pointers (symbol IDs, file paths, concern IDs, thread IDs) — parsed and carried on the row, ignored entirely by verify (non-gating by construction); lint stays offline and warns only on a malformed value (empty ctx), never errors — pointer RESOLUTION is WF-2's job, at render time, against the live index {id: WF-1, symbol: ParseMarkdownPlan, package: internal/verify, reachable_from: TestCtxAnnotationParsedNonGating}
- [x] `fathom plans context <plan> --row <id> [--budget N]` renders the row's pack fresh through the live index — symbol pointers: signature + file:line + direct callers (direct-store or HTTP, whichever answers); file pointers: existence + line count (v1 — the obligations-coverage enrichment via internal/semantic is a later delta, not claimed here); plus the row's own reason:/trigger: text — budget-capped with an elision block; an unresolvable pointer prints `unresolved (moved/renamed since planning)` and the command still exits 0 (a pull tool, not a gate) {id: WF-2, symbol: cmdPlansContext, package: cmd/fathom, reachable_from: TestPlansContextRendersFreshAndNamesStalePointers}

Kill (registered): if raw discovery does not drop on ctx-bearing rows
after ~20 of them (exploration ledger), WF-1 reverts — one line.

## Eligibility — the DAG is the plan file (vision 24 §3)

- [x] the plan grammar accepts `needs: <id>[, <id>…]` referencing stable row ids (multi-id values double-quoted — `needs: "A-1, A-2"` — because the annotation block comma-splits at top level, the same convention `reason:` already uses); lint errors on an unknown id, a self-reference, or a cycle {id: WF-3, symbol: LintPlan, package: internal/verify, reachable_from: TestNeedsLintCatchesUnknownSelfAndCycle}
- [x] frontier rows carry derived `eligible` and `blocked_on` — eligible iff every needs-target is claimed-and-verified or skipped-with-disposition; pure function of (plan, evidence), computed at query time, nothing stored; human output suffixes `(blocked on <id>)` and `--eligible-only` filters the JSON — the entire "bucket" surface, and the last one {id: WF-4, symbol: frontierForPlan, package: cmd/fathom, reachable_from: TestFrontierEligibilityDerivesFromNeeds}

Refused at the row level, per 09-deterministic-sdd out-of-scope-forever:
no watch/subscribe surface, no lifecycle enum, no event log, no spawn.

## Leases — declared intent, derived state (vision 24 §4)

- [x] `fathom plans lease <row-id> --plan <file> [--ttl 2h] [--release]` appends `{plan, row_id, holder, harness, filed, expires}` to `.fathom/leases.jsonl` under a file lock; holder defaults to the session id (Claude Code) or a supplied string (Codex); leased/available/expired are all derived (row exists ∧ now<expires), no status field exists anywhere, and nothing in verify or the store is touched {id: WF-5, symbol: cmdPlansLease, package: cmd/fathom, reachable_from: TestLeaseAppendsDerivedStateOnly}
- [x] `plans next` decorates frontier rows holding a fresh lease (`leased (holder, 43m left)`) and `--free` filters them; an expired lease is invisible — the row simply reappears {id: WF-6, symbol: frontierForPlan, package: cmd/fathom, reachable_from: TestFrontierDecoratesAndFreesLeases}
- [x] `fathom plans leases` lists active/expired leases and counts overlaps (two fresh leases on one row) — the collision instrument that is this feature's own evidence case {id: WF-7, symbol: cmdPlansLeases, package: cmd/fathom, reachable_from: TestLeasesReportCountsOverlaps}

Refused: heartbeats, supervisors, reminder hooks — TTL-only expiry is
the design, chosen because Codex has no lifecycle surface to fake.
Kill (registered): zero overlaps and zero decoration value after 6
weeks → delete the verbs, record the null.

## Prime and the opening moves — both harnesses (vision 24 §5–6)

- [x] `fathom prime` prints the full session primer as plain text — identity + freshness + live example + thread brief + custodian line, byte-equivalent in content to the SessionStart hook payload — so a hook-less harness pulls in one command what Claude Code gets pushed {id: WF-8, symbol: cmdPrime, package: cmd/fathom, reachable_from: TestPrimeMatchesSessionHookPayload}
- [x] the stanza template gains the task-shaped opening-moves table (debug/feature/review/refactor → first 2–3 calls) and its Codex line upgrades from `fathom thread brief` to `fathom prime`; the table lands verbatim in CLAUDE.md and AGENTS.md through the existing marker-replace path {id: WF-9, symbol: fathomStanza, package: cmd/fathom, reachable_from: TestStanzaCarriesOpeningMovesAndPrime}
- [x] `fathom init`'s manual-steps output prints the exact Codex MCP registration commands (`codex mcp add …` HTTP + stdio fallback) beside the `claude mcp add` line it already prints {id: WF-10, symbol: printManualSteps, package: cmd/fathom, reachable_from: TestInitPrintsCodexMcpAdd}
- [x] docs/adoption/codex.md §6 states the measurement boundary: Codex sessions are outside the impact observatory's instrumentation, adoption there is unmeasured by design, and law 12 forbids extrapolating Claude-arm numbers to Codex — asserted by a doc-content test so the honesty line cannot silently vanish {id: WF-11, symbol: TestCodexDocNamesMeasurementBoundary, package: cmd/fathom, skip_test: true, skip: "doc-content row — the acceptance evidence IS the test itself reading docs/adoption/codex.md; a test cannot be reached by a test"}

Kill (registered, I1): if the opening-moves table moves neither the
zero-fathom-call session rate (baseline 29/135) nor the raw:indexed
ratio (~5:1) over ~4 weeks of ON-arm sessions, revert WF-9's table
lines and record the null.

## Deferred to its own design pass

- The channel/vehicle seam (payload layer with per-vehicle emitters;
  role-variant orchestrator/subagent briefs) — registered direction in
  vision 24 §6, promoted when WF-8's duplication pressure (prime vs
  session hook composing the same payload twice) makes the refactor
  self-paying.
- Operator-contract custodian concerns (stanza-current, thread-hygiene,
  completion-cites-instruments) — graduate through vision 22's engine;
  thread-hygiene's mechanical half ships as IT-9.
