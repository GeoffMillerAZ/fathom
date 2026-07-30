# Expert Suite Phase P — production tokenomics + scoped config

Design (normative — both adopted, owner-commissioned 2026-07-30; see
`docs/vision/13-expert-suite.md` §9's row **P** for the scope this plan
implements):

- `docs/design/production-tokenomics.md` — payload cost accounting
  ("what fathom SPENT," law 12), the discovery-share denominator (the
  counting-only PreToolUse observer), the session token ledger,
  `FATHOM_CHANNELS=off`, service-identity caching, and the pre-registered
  A/B protocol (§5 below — text only, not run by this plan).
- `docs/design/scoped-config.md` — `~/.fathom/ignore` (additive-only
  union) and `~/.fathom/guard.cue` (note/command_scopes only, law 13),
  disclosure everywhere, `init --seed-user-ignore`.
- Doctrine: `docs/vision/13-expert-suite.md` §8 laws 12 and 13 (already
  landed in that doc; this plan is the implementation, not the doctrine
  change).

**Anchor lessons carried from phase O (binding on every row below).**
1. FUNCTIONS/METHODS only as `symbol:` anchors — never struct/type names.
2. Checked for symbol/type name collisions before anchoring (`grep -rn
   "^type <Name> "` and `"^func.*<Name>("` across `internal/` and
   `cmd/fathom/`, this plan's own P-*/F-* symbol list) — zero collisions
   found for every symbol anchored below.
3. Doc-only rows carry `skip_test: true` with a `skip:` reason naming the
   actual acceptance evidence, never a fabricated `Test*` claim.

## Key engineering decisions this plan pins

- **Instrument A's beat shape.** Every injected payload (session brief,
  post-edit, pre-edit guard — file and command — and the module-tagged
  subset of either) calls the shared `emitPayloadCost(channel, payload,
  ruleCount, phase, sessionID)` (`cmd/fathom/payloadcost.go`), a beat on a
  NEW channel (`"payload-cost"`) whose `Kind` names the channel
  (`session-brief`/`post-edit`/`pre-edit-guard`/`module-rules`) and whose
  `Meta` carries `bytes`, `tokens_estimate` (via the pre-existing
  `internal/cost.Estimate`, bytes/4 — reused, not reinvented), `rules`,
  and (when known) `phase`/`session`. `module-rules` is computed by
  re-rendering ONLY the module-tagged bullets through the SAME
  `writeRuleBullet` the real payload used (`reportModuleBeats`), so its
  byte count is exact, never an approximation.
- **Instrument B reuses the phase-O PreToolUse channel, not a new hook.**
  `cmdEditHookPre` (`edithook_pre.go`) gains a THIRD dispatch branch for
  `Grep`/`Glob`/`Read` (`cmdEditHookPreDiscovery`, `discovery.go`) plus a
  parallel check inside the existing `Bash` branch
  (`emitDiscoveryBeatIfShellSearch`) that runs ALONGSIDE the pre-existing
  command-guard match, never instead of it. Both paths make the daemon
  ZERO calls, proven directly (`TestCmdEditHookPre_DiscoveryTools_ZeroDaemonCalls`,
  the same httptest-counter pattern O-8/O-9 established). `fathom init`
  registers a THIRD `PreToolUse` matcher group (`Grep|Glob|Read`) — the
  existing `ensureHookEntry` per-group marker scoping (O-13's fix) means
  this wires independently of the other two groups.
- **The join key.** Instrument A's `emitPayloadCost` and instrument B's
  discovery beats both carry a `"session"` meta key when a session id is
  available; instrument C's session ledger (`sessionledger.go`) sums
  exactly those beats for its own `session_id` (`sumSessionBeats`). Three
  independent instruments, one join key, per design §6's own framing.
- **Where `session_id` had to be threaded from.** `cmdEditHookPreCommand`
  and `cmdEditHookPreDiscovery`/`emitDiscoveryBeatIfShellSearch` gained a
  `sessionID` parameter (previously command guards never needed one — no
  per-file dedup stamp on that path). `cmdSessionHook` gained a `stdin
  io.Reader` parameter (it never read stdin before this phase) to recover
  `session_id` from the SessionStart hook envelope — `main.go`'s dispatch
  updated to pass it through, mirroring `serve-mcp`'s existing pattern.
- **The session ledger's transcript parsing is best-effort by
  construction.** `transcriptSummaryFrom` (`sessionledger.go`) reads
  Claude Code's own `transcript_path` JSONL — turn count (`type ==
  "assistant"` lines), harness-reported tokens (summed `message.usage.*`
  fields), and duration (last-minus-first parseable timestamp). ANY
  unrecognized shape, absent field, or unreadable file degrades that ONE
  field to an honest `*_note`, never a fabricated zero — malformed lines
  are skipped individually, never aborting the whole file
  (`TestTranscriptSummaryFrom_MalformedLinesSkipped`).
- **`FATHOM_CHANNELS=off` is checked FIRST, before any other work**, in
  every hook entrypoint (`cmdEditHook`, `cmdSessionHook`, `cmdStopHook`) —
  proven by dedicated `*_ChannelsOff_TotalSilence` tests asserting BOTH
  zero daemon calls and zero stdout. `fathom doctor`'s `channels_env`
  check (`doctor_phasep.go`) is the loud, independent surface that
  detects the switch being set — doctor is never itself silenced by the
  var it's reporting on.
- **Instrument E's cache is per-(repo root, daemon URL), not per-PID.**
  `identcache.go`'s `resolveIndexedCandidateCached` wraps the existing
  `resolveIndexedCandidate` daemon round trip with a `guardstamp.go`-style
  hardened local stamp (0700 dir, 0600 file, `O_EXCL|O_NOFOLLOW` write,
  owner/permission-checked read, no TOCTOU gap) keyed by
  `sha256(projDir + baseURL)`, TTL 5 minutes, escape hatch
  `FATHOM_IDENT_NO_CACHE`. Every outcome emits an `ident-cache` beat
  (`hit`/`miss`) rather than an in-process counter — a hook is a
  one-shot process making at most a couple of these calls, so a
  process-local counter would be nearly meaningless; the beat log lets
  `research report` aggregate a real hit rate across sessions.
  `candidatesWithDaemon` (the function EVERY hook/doctor check already
  called for identity resolution) now goes through the cache
  transparently — no caller-side change needed anywhere else.
- **Scoped-config's safety property is directional, not merely
  additive-only in isolation.** `internal/connector.LoadIgnoreScoped`
  unions user-then-project (in that literal slice order) so
  `filter.Matcher`'s existing last-match-wins semantics make a PROJECT
  negation always beat a USER-scope exclusion on conflict — proven
  directly (`TestLoadIgnoreScoped_ProjectWinsOnConflict`). The loud
  refusal (`*connector.ErrUserScopeReinclude`) fires at `Discover` (the
  one place that actually governs what enters the index); the read-time
  convenience `LoadIgnore` (`read_source`'s dependency) degrades
  gracefully instead — project-scope-only fallback, never a broken hot
  path — with `fathom doctor`'s `user_scope_ignore` check as the loud
  surface for that condition outside of ingest.
- **`guardspec.LoadUserGuards` tags its own output, no separate "source"
  flag needed anywhere downstream.** Every decoded user-scope guard's
  `Note` is prefixed `"[user] "` AT LOAD TIME — every existing renderer
  (`cmdEditHookPreCommand`'s command-guard path) already prints `Note`
  verbatim, so tagging happens for free without touching render code or
  widening `guardspec.Guard`'s shape with a provenance field.

## §5 — the A/B protocol, pre-registered (PLAN TEXT ONLY — NOT RUN by this plan)

Per the falsifiability precedent this program has held to since
`docs/plans/falsifiability.md`: the criterion and kill clause are
committed here, in this exact form, BEFORE any run. This section is not
executed as part of phase P — it is filed as the pre-registration a
future measurement run must be scored against unmodified. Verbatim from
`docs/design/production-tokenomics.md` §5:

> - **Unit:** a working session in an indexed repo.
> - **Arms:** obligations injection + index tools ON (control = today) vs
>   a `FATHOM_CHANNELS=off` session (hooks silent, MCP surface
>   unregistered).
> - **Measure:** end-of-session harness token total (recorded by the Stop
>   hook, §6), turns, and a subjective 1–5 "did I get where I wanted"
>   note.
> - **Criterion, registered before any run:** ON must show lower median
>   tokens-per-session at equal-or-better self-rated outcome across ≥20
>   sessions per arm. Recorded either way, per house record.
> - **Kill:** if ON is not better after 40 sessions, the production
>   tokenomics claim is retracted in every doc that makes it (same
>   mechanism as B1-4).

The OFF arm's mechanism is `FATHOM_CHANNELS=off` (P-4 below) plus a
manual MCP-surface unregistration the owner performs by hand (no code
mechanism exists or is needed for that half — it is a one-line removal
from the harness's own MCP config). The ON arm's token total is whatever
the harness itself reports; when a harness exposes none, the Stop hook's
`tokens_note` (P-3) records `"unavailable"` rather than a fabricated
number for that session, and that session is excluded from the median,
not counted as zero.

**No run has occurred. No results exist. This section will be superseded
by a results row in a later plan the day the coordinator actually runs
it — never edited in place, per the B1-4/H3-1 precedent this program
follows for every falsifiable claim.**

## Tasks

### A — payload cost accounting (`docs/design/production-tokenomics.md` §3)

- [x] Every injection beat records rendered bytes, an explicit token estimate (bytes/4), rule count, and phase — field names and report lines always say "estimate," never "measured" {id: P-1, symbol: emitPayloadCost, package: cmd/fathom}
  - `cmd/fathom/payloadcost.go`: `emitPayloadCost(channel, payload,
    ruleCount, phase, sessionID)` — a no-op on an empty payload (never a
    fabricated zero-cost beat for "nothing was injected," which already
    has its own `answered:false` beat).
  - Wired at all four channels: `sessionhook.go` (session-brief, after
    `buildSessionContext`), `edithook.go` (post-edit, before the final
    JSON marshal), `edithook_pre.go` (pre-edit-guard, both the file path
    and the command-note path), and `reportModuleBeats` (module-rules,
    re-rendering only the module-tagged bullets via the same
    `writeRuleBullet` the real payload used).
  - Tests: `cmd/fathom/channels_test.go`,
    `cmd/fathom/research_report_phasep_test.go`'s
    `TestBuildResearchReport_PayloadCost_PerChannelTotals` exercises the
    beats end-to-end through the aggregator; `TestBuildSessionLedgerEntry_JoinsInstrumentAAndB`
    proves the `session` meta tag reaches the join. Full `cmd/fathom`
    suite green (`go test -race ./cmd/fathom/...`).

- [x] `research report` gains a section titled EXACTLY "what fathom SPENT": per-channel totals and per-edit p50/p95, honest-absence note when never fired {id: P-2, symbol: buildPayloadCostSection, package: cmd/fathom}
  - `cmd/fathom/research_report.go`: `PayloadCostByChannel`
    (`map[string]PayloadCostChannelStat`), `PayloadCostPerEditP50Estimate`/
    `P95Estimate` (computed over post-edit + pre-edit-guard beats only —
    each IS one edit's injected governance), `PayloadCostNote` (always
    present; cites law 12 and "estimate" explicitly when populated,
    `"not measurable: …"` when the channel never fired).
  - `cmd/fathom/research_report_render.go`: the exact string
    `"what fathom SPENT"` as a section header — pinned directly
    (`TestRenderResearchReportText_TitleIsExactlyWhatFathomSpent`).
  - Tests: `TestBuildResearchReport_PayloadCost_PerChannelTotals`,
    `TestBuildResearchReport_PayloadCost_NeverFired_HonestNote`,
    `TestRenderResearchReportText_TitleIsExactlyWhatFathomSpent`
    (`cmd/fathom/research_report_phasep_test.go`).

### B — the discovery-share denominator (`docs/design/production-tokenomics.md` §4)

- [x] The counting-only PreToolUse observer: zero daemon calls, zero injection, one beat per discovery action with a coarse kind {id: P-3, symbol: cmdEditHookPreDiscovery, package: cmd/fathom}
  - `cmd/fathom/discovery.go`: `cmdEditHookPreDiscovery(toolName,
    sessionID)` (the `Grep`/`Glob`/`Read` branch) and
    `emitDiscoveryBeatIfShellSearch(command, sessionID)` (the `Bash`
    branch's classifier — `firstCommandToken` conservatively resolves
    argv[0] past env-assignment/`command`/`env` wrappers and a path
    prefix; an unrecognized shape counts nothing, never mis-counts).
  - `cmd/fathom/edithook_pre.go`'s `cmdEditHookPre` dispatch extended to
    a three-way switch (`Bash` / `Grep|Glob|Read` / default file guard).
  - `cmd/fathom/init.go`'s `fathomHookSpecs` gains a THIRD `PreToolUse`
    matcher group (`Grep|Glob|Read`, same `edit-hook --pre` command).
  - Tests (`cmd/fathom/discovery_test.go`): `TestFirstCommandToken`,
    `TestEmitDiscoveryBeatIfShellSearch`,
    `TestCmdEditHookPre_DiscoveryTools_ZeroDaemonCalls` (the load-bearing
    zero-daemon-call proof, httptest counter, all three tool names),
    `TestCmdEditHookPre_DiscoveryTools_EmitsBeat` (Read stays its own
    bucket), `TestCmdEditHookPre_BashSearchCommand_EmitsShellSearchAndStillGuards`
    (proves the classifier runs ALONGSIDE, not instead of, command-guard
    matching). `cmd/fathom/init_test.go`'s
    `TestCmdInit_WiresRepoEndToEnd`/`TestMergeHookSettings_SkipsUserScopeWiring`
    extended to assert THREE `PreToolUse` groups, not two.

- [x] `research report` gains a discovery-share section: fathom's own index-tool calls vs observed discovery-action buckets, `read` its own bucket, whole section labeled correlational {id: P-4, symbol: buildDiscoverySection, package: cmd/fathom}
  - `cmd/fathom/research_report.go`: `discoveryIndexTools` (the explicit,
    commented 10-tool set: `find_symbol`, `query_symbols`, `references`,
    `find_callers`, `symbol_context`, `outline`, `repo_map`,
    `pack_context`, `read_source`, `entrypoints` — deliberately excludes
    `capabilities`/`ingest`/`list_services`/`obligations`/
    `staleness_report`/`verify_plan` per the design's own instruction);
    `DiscoveryActionsTotal`/`ByKind`/`DiscoveryIndexToolCalls`/
    `DiscoverySharePercent`; `DiscoveryShareNote` rewritten to describe
    the REAL mechanism (was: "not measurable... no denominator") and
    explicitly says CORRELATIONAL every time, including with zero
    traffic.
  - Test-suite fix: `TestBuildResearchReport_Golden`
    (`research_report_test.go`) previously pinned `DiscoveryShareNote` to
    the generic "not measurable" bucket — updated to assert the note is
    always present and always says "CORRELATIONAL" (the field's honest
    meaning legitimately changed from structurally-impossible to
    real-but-not-causal).
  - Tests: `TestBuildResearchReport_DiscoveryShare` (index-tool
    numerator excludes plumbing, share percentage arithmetic),
    `TestBuildResearchReport_DiscoveryShare_ReadNeverFoldedIntoGrep`,
    `TestBuildResearchReport_DiscoveryShareNote_AlwaysCorrelational`.

### C — the session token ledger (`docs/design/production-tokenomics.md` §6)

- [x] The Stop hook appends one line per session to a local ledger: session id, repo, duration/turns/tokens IF available (explicit `unavailable`, never a fabricated zero), plus instrument A/B counts as the join {id: P-5, symbol: buildSessionLedgerEntry, package: cmd/fathom}
  - `cmd/fathom/sessionledger.go`: `sessionLedgerPath()`
    (`$FATHOM_SESSION_LEDGER`, default `~/.fathom/session-ledger.jsonl`);
    `appendSessionLedger` (best-effort, every failure swallowed — mirrors
    `emitBeat`'s own doctrine); `buildSessionLedgerEntry` (the pure,
    testable core — injected clock, no `time.Now()`); `transcriptSummaryFrom`/
    `summarizeTranscriptBytes` (best-effort Claude Code transcript JSONL
    parsing — turn count from `type=="assistant"` lines, tokens from
    summed `message.usage.*` fields, duration from first/last parseable
    timestamp; any unrecognized shape degrades that field alone, never
    the whole entry); `sumSessionBeats` (the A/B join over the beat log,
    filtered by `Meta["session"]`).
  - `cmd/fathom/stophook.go`: `recordSessionLedgerEntry` called from
    `cmdStopHook` immediately after parsing `session_id`/`transcript_path`
    from the Stop hook JSON, BEFORE the plan-verification flow —
    read-only w.r.t. everything else the hook does, a ledger-write
    failure can never affect that flow (swallowed inside
    `appendSessionLedger`).
  - `cmd/fathom/sessionhook.go`/`main.go`: `cmdSessionHook` gained a
    `stdin io.Reader` parameter (it never read stdin before) to recover
    `session_id` for instrument A's session-brief tag —
    `main.go`'s dispatch updated to pass it through.
  - Tests (`cmd/fathom/sessionledger_test.go`):
    `TestTranscriptSummaryFrom_NoPath_AllUnavailable`,
    `_UnreadableFile_AllUnavailable`, `_ValidTranscript` (full round-trip:
    turn count, summed tokens, duration all correct against a hand-built
    fixture), `_NoUsageData_TokensUnavailableNeverZero` (the law-12-style
    honesty proof for this instrument), `_MalformedLinesSkipped`;
    `TestBuildSessionLedgerEntry_JoinsInstrumentAAndB` (the A/B join,
    proven against a mixed-session beat fixture — a different
    `session_id`'s beats must never leak in), `_NoSessionID_JoinNoted`;
    `TestAppendSessionLedger_WritesOneJSONLine`,
    `TestSessionLedgerPath_EnvOverride`; end-to-end:
    `TestStopHook_AppendsSessionLedgerEntry`,
    `TestStopHook_ChannelsOff_NoSessionLedgerEntry`.

### D — `FATHOM_CHANNELS=off` kill switch (`docs/design/production-tokenomics.md` §9.4 item 4)

- [x] One env var silences every fathom hook completely, checked FIRST in each entrypoint; `fathom doctor` reports loudly when it's set {id: P-6, symbol: fathomChannelsDisabled, package: cmd/fathom}
  - `cmd/fathom/channels.go`: `fathomChannelsDisabled()` — `os.Getenv("FATHOM_CHANNELS") == "off"`,
    exact-match only (any other value is a no-op, tested).
  - Wired as the FIRST statement in `cmdEditHook` (`edithook.go`),
    `cmdSessionHook` (`sessionhook.go`), `cmdStopHook` (`stophook.go`) —
    before stdin read, before any HTTP client construction, before any
    beat emission.
  - `cmd/fathom/doctor_phasep.go`: `checkChannelsEnv()` — `doctorWarn`
    (never fail — a deliberate debug/A-B state, not a wiring break) when
    set, naming the remedy; doctor itself reads the SAME env var
    independently and is never silenced by it.
  - Tests (`cmd/fathom/channels_test.go`):
    `TestFathomChannelsDisabled` (unset/off/other-value-does-not-disable);
    `TestEditHook_ChannelsOff_TotalSilence`,
    `TestSessionHook_ChannelsOff_TotalSilence`,
    `TestStopHook_ChannelsOff_TotalSilence` (all three: zero daemon
    calls via `countingServer`, zero stdout); `cmd/fathom/doctor_phasep_test.go`'s
    `TestCheckChannelsEnv_Unset`/`_Off`.

### E — service-identity caching (`docs/design/production-tokenomics.md` §7)

- [x] `candidatesWithDaemon`'s daemon round trip cached per (repo root, daemon URL), short TTL, `--no-cache` escape, hit-rate reported as a latency/load signal {id: P-7, symbol: resolveIndexedCandidateCached, package: cmd/fathom}
  - `cmd/fathom/identcache.go`: `resolveIndexedCandidateCached` wraps
    `resolveIndexedCandidate` with a `guardstamp.go`-hardened local stamp
    (`identCacheDir`, TTL `identCacheTTL` = 5 min, keyed by
    `sha256(projDir + baseURL)`); a NEGATIVE resolution (repo not yet
    indexed) is cached too, so an unindexed repo doesn't re-probe on
    every single edit either. Escape hatch: `FATHOM_IDENT_NO_CACHE`
    (env var, not a per-command flag — the dominant callers are hooks
    invoked via a fixed JSON command line, not an interactive session).
    Every outcome emits an `ident-cache` beat (`hit`/`miss`).
  - `cmd/fathom/serviceident.go`'s `candidatesWithDaemon` now calls the
    cached wrapper instead of `resolveIndexedCandidate` directly — every
    existing caller (edit-hook pre/post, stop-hook, doctor checks) gets
    the cache transparently, no signature change needed anywhere else.
  - `cmd/fathom/research_report.go`: `buildIdentCacheSection` —
    `IdentCacheHits`/`Misses`/`HitRate`/`Note` (the note explicitly
    disclaims being a token figure — "latency/load signal only").
  - `cmd/fathom/doctor_phasep.go`: `checkIdentCacheEnv()` — `doctorWarn`
    when the escape hatch is set.
  - Tests (`cmd/fathom/identcache_test.go`): `TestIdentCache_WriteThenRead`,
    `_NotFoundCachedToo`, `_AbsentWhenNeverWritten`, `_ExpiresAfterTTL`,
    `_KeyedByBothRootAndURL`; the load-bearing proof:
    `TestCandidatesWithDaemon_CachesAcrossCalls` (two calls, ONE daemon
    round trip, via `countingServer`) and
    `TestCandidatesWithDaemon_NoCacheEscape` (env var set, TWO round
    trips); `cmd/fathom/research_report_phasep_test.go`'s
    `TestBuildResearchReport_IdentCacheHitRate`,
    `_IdentCache_NeverFired_HonestNote`.

### F — scoped config (`docs/design/scoped-config.md`)

- [x] `~/.fathom/ignore` UNIONed into every `connector.LoadIgnore` caller; additive-only enforced in code — a user-scope re-include is refused loudly {id: P-8, symbol: LoadIgnoreScoped, package: internal/connector}
  - `internal/connector/userignore.go`: `UserIgnorePath()` (`~/.fathom/ignore`);
    `ErrUserScopeReinclude` (names the file, 1-indexed line, and the
    exact pattern); `LoadIgnoreScoped(root)` — unions user-then-project
    (project wins on conflict, `filter.Matcher`'s last-match-wins);
    `IgnoreCounts{Project, User}` for disclosure; `ProjectOnlyIgnore`
    (doctor's isolation helper); `DefaultUserIgnoreSeed` (the
    machine-junk defaults `init --seed-user-ignore` offers).
  - `internal/connector/local.go`: `Discover` now calls
    `LoadIgnoreScoped` and REFUSES outright (returns the error) on a
    violation — the loud enforcement point, since `Discover` is what
    actually governs what enters the index. The exported `LoadIgnore`
    (read_source's dependency) degrades gracefully on the SAME
    violation instead — project-scope-only fallback, never a broken hot
    serving path.
  - Tests (`internal/connector/userignore_test.go`):
    `TestLoadIgnoreScoped_NeitherFile_NilMatcher`, `_UnionsProjectAndUser`,
    `_UserReinclude_RefusedLoudly` (the load-bearing loud-refusal proof,
    asserts the exact line number and pattern via `errors.As`),
    `_ProjectReinclude_StillAllowed` (project's OWN `!pattern` stays
    legal), `_ProjectWinsOnConflict` (law 13's directional proof —
    project overrides a user-scope default on the SAME path),
    `TestLocalConnector_Discover_UserReinclude_RefusesIngest` (end-to-end
    through the real `Discover`), `TestLoadIgnore_DegradesGracefullyOnUserViolation`,
    `TestUserIgnorePath_UnresolvableHome`. Full pre-existing
    `internal/connector` suite green (`Discover`'s existing ignore
    behavior unchanged when no user-scope file exists).

- [x] `~/.fathom/guard.cue` loaded by `guardspec`, restricted to `note`/`command_scopes` — any governance field is a strict validation error naming the field; notes tagged `[user]` {id: P-9, symbol: LoadUserGuards, package: internal/guardspec}
  - `internal/guardspec/guard.go`: `userScopeForbiddenFields` (`scopes`,
    `max_rules`, `include_modules`, `min_tier` — the last checked
    defensively even though no `#Guard` field of that name exists today,
    closing the same silent-unknown-field risk `checkNoOptionalFields`
    already closes for the `?:` syntax trap); `LoadUserGuards(path)` —
    per-guard forbidden-field scan BEFORE decode, `"[user] "`-prefixes
    every non-empty `Note` at load time (no separate provenance field
    needed anywhere downstream), requires `command_scopes` non-empty
    (a user-scope guard can never set `scopes`, so it can only ever be a
    command guard).
  - `cmd/fathom/guard.go`: `loadUserGuardSet()` — fail-open on
    absence/malformed (mirrors `loadGuardSet`'s own discipline exactly);
    `cmd/fathom/edithook_pre.go`'s `cmdEditHookPreCommand` consults it
    ONLY when no PROJECT guard already matched — project always wins,
    proven directly.
  - Tests (`internal/guardspec/userguard_test.go`): `TestLoadUserGuards_Valid`
    (tag applied, `Scopes` never set), `_NoteOnlyStillTagged`,
    `_ScopesForbidden`, `_MaxRulesForbidden`, `_IncludeModulesForbidden`,
    `_MinTierForbidden` (the defensive-field proof), `_NoCommandScopes_Rejected`,
    `_MissingFile`. `cmd/fathom/userguard_test.go`:
    `TestLoadUserGuardSet_Absent`, `_Valid`, `_Malformed_FailsOpen`,
    `TestEditHookPre_UserScopeCommandGuard_Matches` (end-to-end through
    the real hook, `[user] ` tag visible in the rendered payload),
    `TestEditHookPre_ProjectGuardWinsOverUserScope` (the law-13 proof at
    the hook level — user note never appears when a project guard
    already matched the same command).

- [x] Disclosure: ignore counts split project/user in `fathom ingest`/`fathom module sources`; a doctor check naming each user-scope file in effect; the "why can't fathom see my code" warning {id: P-10, symbol: checkUserScopeIgnore, package: cmd/fathom, skip_test: true, skip: "the checks THIS row documents (checkUserScopeIgnore/checkUserScopeGuard) are anchored and tested at P-8/P-9's own rows via doctor_phasep_test.go; this row's own acceptance evidence is the disclosure lines' presence in ingest.go/module_sources.go, verified by direct code read (grep for the 'excluded:' format string) and by the real scratch ingest run at plan-verification time, which printed 'excluded: 6 pattern(s) (6 project, 0 user-scope ~/.fathom/ignore)' — since neither command's own pre-existing test suite asserts stdout byte-for-byte"}
  - `cmd/fathom/ingest.go`: `cmdIngest` prints `excluded: N pattern(s)
    (P project, U user-scope ~/.fathom/ignore)` after a successful
    ingest, via a cheap, display-only re-read of `LoadIgnoreScoped`
    (`Discover` already ran it as the real enforcement gate — a
    violation would have failed ingest before this line is ever
    reached).
  - `cmd/fathom/module_sources.go`: `cmdModuleSources` gains the same
    disclosure line in its human-text output (`root:`/`excluded:` rows).
  - `cmd/fathom/doctor_phasep.go`: `checkUserScopeIgnore(root)` — names
    `~/.fathom/ignore` when in effect, `doctorWarn` on a violation, and —
    the "why can't fathom see my code" pre-empt —
    `walkForUserHiddenFiles` samples the tree (capped
    `userScopeIgnoreWalkCap` entries) for a file excluded by the UNION
    but NOT by project-scope-alone, warning with a concrete example path
    when found; `checkUserScopeGuard()` — names `~/.fathom/guard.cue`
    when in effect, `doctorWarn` on a governance-field violation.
  - Tests (`cmd/fathom/doctor_phasep_test.go`):
    `TestCheckUserScopeIgnore_NoHomeFile_Pass`,
    `_ConfiguredNoHiddenFiles_Pass`, `_HidingIndexableFile_Warn` (the
    "why can't fathom see my code" proof), `_ProjectAlreadyExcludes_NotBlamedOnUser`
    (the isolation proof — `ProjectOnlyIgnore` correctly excludes the
    project's own contribution from blame), `_Reinclude_Warn`;
    `TestCheckUserScopeGuard_Absent_Pass`, `_Valid_Pass`, `_Invalid_Warn`.

- [x] `init --seed-user-ignore`: prints exactly what it would write and requires the flag; never writes silently {id: P-11, symbol: seedUserIgnoreFile, package: cmd/fathom}
  - `cmd/fathom/init.go`: `seedUserIgnoreFile(stdout)` — write-if-absent
    (mirrors `writeFathomIgnore`'s own discipline), prints the exact
    content BEFORE writing it, never overwrites an existing file. Called
    from `cmdInit` ONLY when `--seed-user-ignore` is explicitly passed —
    every other run of `fathom init` never touches `~/.fathom/ignore` at
    all, proven directly.
  - Tests (`cmd/fathom/seeduserignore_test.go`):
    `TestSeedUserIgnoreFile_WritesDefaults` (every default pattern
    present in BOTH the written file and the printed output),
    `_NeverOverwrites`; `TestCmdInit_SeedUserIgnore_RequiresFlag` (the
    load-bearing proof: a full `fathom init` run with NO flag leaves
    `~/.fathom/ignore` absent), `_WritesWhenFlagged`.

### Test hygiene (package-wide, cuts across every row above)

- [x] `cmd/fathom` package gains a `TestMain` hermetic-$HOME safety net — required once user-scope code paths started defaulting to the real `$HOME` {id: P-12, symbol: fathomChannelsDisabled, package: cmd/fathom, skip_test: true, skip: "the safety net itself (cmd/fathom/testmain_test.go's TestMain) is test-harness infrastructure, not a callable production symbol — anchored to P-6's own symbol since fathomChannelsDisabled's tests are among the many that now depend on this hermetic HOME being in place; verified directly by re-running the full cmd/fathom suite with -race after adding it (zero failures, zero writes to the real machine $HOME confirmed by inspection of the TestMain-set path)"}

## Docs

- [x] `docs/use-cases/research-flywheel.md`: tools-table row extended, discovery-blind-spot claim updated, new "Production tokenomics (phase P)" section {id: P-13, symbol: buildPayloadCostSection, package: cmd/fathom, skip_test: true, skip: "documentation-only row; the sections/fields it documents are proven by P-1 through P-7's own tests"}

- [x] `docs/adoption/research-flywheel.md`: Step 4 gains two bullets — discovery share real-but-correlational, payload cost a cost figure never savings {id: P-14, symbol: buildDiscoverySection, package: cmd/fathom, skip_test: true, skip: "documentation-only row"}

- [x] `docs/configuration.md`: new "Scoped config" section — additive-only enforcement, exact refusal text, disclosure, `init --seed-user-ignore`, guard restriction and `[user]` tagging {id: P-15, symbol: LoadIgnoreScoped, package: internal/connector, skip_test: true, skip: "documentation-only row; the mechanisms it documents are proven by P-8/P-9's own tests"}

- [x] `docs/adoption/claude-code.md`: §7.5's JSON snippet gains the third `PreToolUse` matcher group; new §7.6 documents the discovery observer and `FATHOM_CHANNELS=off` {id: P-16, symbol: cmdEditHookPreDiscovery, package: cmd/fathom, skip_test: true, skip: "documentation-only row; the wiring it documents is proven by P-3/P-6's own tests (TestCmdInit_WiresRepoEndToEnd's three-group assertion, the ChannelsOff_TotalSilence suite)"}

- [x] `docs/design/guarded-edits.md` §3's non-goal line already points at `docs/design/scoped-config.md` law 13 — verified unchanged, no edit needed this phase {id: P-17, symbol: LoadUserGuards, package: internal/guardspec, skip_test: true, skip: "verification-only row; confirmed by direct read of docs/design/guarded-edits.md §3 at authoring time — the line already reads 'per-user guards ONLY as narrowly amended by docs/design/scoped-config.md law 13'"}

## Deferred (named triggers, not silently dropped)

- **`~/.fathom/ignore`/`~/.fathom/guard.cue` disclosure in `repo_map`/
  `list_sources`'s MCP wire response** — P-10 wires disclosure into the
  two CLI surfaces (`ingest`, `module sources`) that already print a
  human-text summary; the MCP tool responses (`internal/mcp`'s
  `list_sources`/`repo_map` handlers) would need their own JSON field
  addition, a materially bigger surface-compatibility change than this
  phase's other additive-JSON rows. **Trigger:** a documented case where
  an MCP caller needed the split and had no CLI fallback available.
- **Bundle sanitization for instruments A/B/E.** `fathom research
  bundle`'s allowlist (`internal/research/bundle`) is a SEPARATE,
  independently-validated struct from `ResearchReport` by design (its own
  CUE schema, its own field allowlist) — none of phase P's new
  `ResearchReport` fields reach the bundle automatically, which is the
  SAFE default (no accidental leak), not an oversight. **Trigger:** a
  documented request to share cost/discovery/cache figures externally via
  the sanitized bundle path.
- **The A/B protocol itself.** Pre-registered in §5 above, text only.
  **Trigger:** the coordinator running it — an owner action, not
  something this plan (or any future code-only plan) can complete.
- **Per-session MCP-surface unregistration tooling for the OFF arm.**
  §5's OFF arm needs the harness's MCP config edited by hand; no fathom
  command automates it (it is a harness-side action, not a fathom one).
  **Trigger:** the A/B running often enough that the manual step becomes
  the friction point — not yet observed.
