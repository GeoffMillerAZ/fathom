# Expert Suite Phase O — platform modules in the coding loop + guarded edits

Design (normative — both adopted, owner-commissioned 2026-07-29; see
`docs/vision/13-expert-suite.md` §9's row **O** for the scope this plan
implements):

- `docs/design/platform-modules.md` — the platform stratum's actual
  distribution vehicle: a compiled-once expert module a consumer repo
  *binds* (`.fathom/modules.cue`, schema `schema/binding/`), served
  through the existing forced channels (edit-hook, `obligations` tool),
  module-tagged, never silent on an unmounted binding.
- `docs/design/guarded-edits.md` — pre-edit guidance for configured
  critical scopes: `.fathom/guard.cue` (schema `schema/guard/`), a
  PreToolUse hook (`fathom edit-hook --pre`) that is silent-by-default
  (local glob check, zero daemon calls on a miss), closing the intent of
  the long-deferred GI-8 row with a sharper mechanism.

**Anchor lessons from this program (binding on every row below).**
1. FUNCTIONS/METHODS only as `symbol:` anchors — never struct/type names
   (`test_reaches`/verify's own resolution is over callable symbols; a
   struct has no call edges to reach it by). Every `Obligation`/
   `Obligations` field addition below is anchored at the FUNCTION that
   sets or consumes it (`modserve.Bind`), never at `semantic.ModuleNote`
   or `semantic.Obligation` themselves.
2. Checked for symbol/type name collisions before anchoring (`grep -rn
   "^type <Name> "` and `"^func.*<Name>("` across `internal/` and
   `cmd/fathom/`) — `ValidateStrict` exists identically-named in THREE
   packages (`internal/modspec`, `internal/bindspec`, `internal/guardspec`)
   by design (mirrored dual-mode loaders); every row citing it pairs
   `symbol` with the correct `package` to disambiguate. No other
   collisions found among this phase's new symbols.
3. Doc-only rows (schema files, use-case docs, ADR amendment, skill
   syncs) carry `skip_test: true` with a `skip:` reason naming the actual
   acceptance evidence (a `cue vet` pass, a byte-diff, a cross-reference),
   never a fabricated `Test*` claim.

## Key engineering decisions this plan pins

- **The Displaced split (design deliverable 2).** `semantic.Obligations`
  gained `DisplacedRepo`/`DisplacedModule` (int, additive) and
  `ModuleNotes` ([]ModuleNote, additive); the pre-existing `Displaced`
  field is REPURPOSED as the back-compat TOTAL
  (`DisplacedRepo+DisplacedModule`) rather than left meaning only "repo
  displaced" — chosen because every pre-phase-O caller reading only
  `displaced` still sees a meaningful, correctly-summed number (when no
  binding exists, `DisplacedModule` is always 0 and `Displaced ==
  DisplacedRepo` exactly as before this field existed), which is a
  strictly compatible superset, not a silent meaning change for the
  common case.
- **Where the composition lives.** `internal/semantic` cannot import
  `internal/modulehost` (the reverse import already exists, for
  `ObligationsFor`'s own reuse) — the binding-aware merge therefore lives
  in a NEW package, `internal/modserve`, sitting above both, independently
  testable without an MCP server (`modserve.Bind`, exercised by
  `internal/modserve/bind_test.go`'s fakes, zero real store/daemon).
- **The stamp mechanism (design §2.3's open decision).** `os.TempDir()/
  fathom-guard-stamps/`, keyed by `sha256(session_id + service +
  repo-relative file)` — chosen over `workspace/tmp` because a stamp is
  per-edit bookkeeping outside any repo tree, never a build artifact or
  something that should ever be committed or indexed. Revised post-critic
  (finding 4, see Critic round below): dedup is skipped ENTIRELY when no
  `session_id` is available (rather than relying on `guardStampTTL` alone
  as the leakage backstop) — os.TempDir() is world-writable shared state,
  and a session-less key is fully predictable, so the safe answer is "no
  session, no dedup," never "dedup anyway, bounded by a clock." Stamps are
  additionally single-use (deleted once the post-hook consults them,
  finding 9) and hardened against a planted file on both write
  (`O_CREATE|O_EXCL|O_NOFOLLOW`) and read (ownership/permission/regular-
  file checks on the open FD, no TOCTOU gap).
- **`include_modules` gates DISPLAY, not computation.** The daemon always
  computes module rules once a binding exists (design doctrine: bindings
  are repo-wide, not per-guard); a guard's `include_modules` only filters
  what the PRE-edit preview shows. The post-hook is unaffected and always
  renders everything — so a rule hidden pre-edit by `include_modules:false`
  is deferred to the post-render, never lost. The stamp is built from what
  was ACTUALLY displayed, preserving this.
- **Command guards serve only their `note`.** No per-command obligations
  lookup exists (there is no file to scope one to) — a deliberate scope
  cut from design §2.5, not an oversight: it keeps the zero-daemon-call
  guarantee absolute for the Bash path and matches the design's own worked
  example (`note` only, no `scopes`).

## Critic round (MERGE-WITH-FIXES) — findings and fixes

Verdict: strong overall (trust boundary, budget priority, `modserve`
layering, zero-call claims, `Displaced` compatibility, and doctor-warn
semantics all survived attack with named mechanisms) but with real gaps.
Findings 1–7 and 10 applied; 8/9 folded into 5's fix; 11 not detailed in
the relayed review and not independently actioned.

1. **BLOCKING — scaffold silently widens the trust boundary.** The
   `ExampleTemplate()` worked examples (both `internal/bindspec` and
   `internal/guardspec`) wrote every optional field with a trailing `?:`
   — valid CUE SCHEMA syntax, but in a plain INSTANCE it declares the
   field optional-and-therefore-ABSENT: a human uncommenting
   `scopes?: [...]` gets a binding whose scope reads as unset (module's
   own scopes apply everywhere), the exact inverse of design §3.2's
   "intersects, never widens." Fixed: every instance example (both
   `ExampleTemplate()` bodies, `docs/adoption/claude-code.md` §7.5,
   `docs/use-cases/platform-modules.md`'s three worked examples) now
   writes concrete field names with no `?`, annotated "OPTIONAL" in a
   trailing comment instead. Hardened further: `LoadBindings`/`LoadGuards`
   now call `checkNoOptionalFields` (`bv.Fields(cue.Optional(true))` +
   `Iterator.IsOptional()`) and refuse loudly, naming the field, when ANY
   field is written with a trailing `?` in an authored instance — the one
   misconfiguration plain `LookupPath` absence can never distinguish from
   genuine omission. Tests: `TestLoadBindings_OptionalFieldSyntaxRejected`,
   `TestLoadGuards_OptionalFieldSyntaxRejected` (three cases each), plus
   `TestExampleTemplate_UncommentedRoundTrips` (both packages) — proves
   the FIXED scaffold, uncommented exactly as a human would, loads with
   every field actually SET, not silently absent.
2. **BLOCKING — two normative §3.3 requirements were neither implemented
   nor deferred.** (a) Staleness flags (design deliverable 2's own text)
   were never threaded: a detached/drifted module's rules rendered
   identically to a fresh module's. Fixed: `modserve.ModuleFederator`
   gains `ModuleStaleness` (the capability `*modulehost.Host` already
   exposed); `Bind` fetches it once per binding (never per rule),
   flags a rule's `Status` with the module's non-fresh verdict
   (`drifted`/`unknown-detached`) via the SAME field/rendering repo
   claims already use (`writeRuleBullet` already prints `r.Status`),
   and never clobbers a status the module's own store already set. A
   staleness-lookup failure fails open (unflagged, never blocks the
   payload). Tests: `TestBind_DriftedModule_FlagsRules`,
   `TestBind_UnknownDetachedModule_FlagsRules`, `TestBind_FreshModule_NotFlagged`,
   `TestBind_StalenessNeverClobbersExistingStatus`,
   `TestBind_StalenessLookupFailure_FailsOpen`. (b) The SessionStart brief
   line and module rules as evidence in `module_query`/`pack_context`/
   `brief`/`pkg/client.Client.Obligations` (design §3.3's other two
   bullets) were never implemented — moved to Deferred below with named
   triggers rather than left silently unclaimed.
3. **BLOCKING — phase O silently rebased the flywheel's headline gauge.**
   `edit-hook.seen`/`.answered` beats now fire for the `--pre` hook too
   (registered on BOTH `Edit|Write|MultiEdit` and `Bash`), but
   `cmd/fathom/instrument.go`'s `reportInstrument` (the `injection
   coverage (7d): N/M` line + its ≥90% gate) and `research_report.go`'s
   `InjectionSeen`/`InjectionAnswered`/`ObligationsServed` both summed
   ALL phases together — the moment a repo re-runs `fathom init`, the
   legacy denominator would silently absorb every `Bash` call too,
   making every phase-O-era number incomparable to every pre-phase-O
   number in `docs/plans/activation.md`. Fixed: both aggregators now
   skip `Meta["phase"] == "pre"` beats entirely when computing the
   legacy counters (a beat with no phase key at all predates this
   feature and is treated as post, preserving old log compatibility
   exactly). `GuardPreSeen`/`GuardPreServed` (research_report.go) report
   the pre-channel separately, as before. Test:
   `TestBuildResearchReport_GuardedEditsAndModules` (extended) pins that
   a pre beat does not move `InjectionSeen`; `TestReportInstrument_CoverageAndAlarms`
   (pre-existing, unaffected — its fixture beats carry no phase meta,
   proving the fix is backward-compatible by construction).
4. **Stamp file was unauthenticated shared-tmp state.** `os.TempDir()` is
   world-writable on Linux/CI; a predictable key (no `session_id`) let
   anyone who could write to the temp dir forge a stamp and make the
   post-hook silently suppress rules that were never actually served
   ("denial of guidance"); `os.WriteFile` followed a planted symlink.
   Fixed: `writeGuardStamp`/`readGuardStamp` are now a no-op whenever
   `sessionID == ""` (the dedup is a pure token optimization — losing it
   is free, so the safe answer is skipping it, not bounding it by a
   clock); writes go through `os.Remove` (never follows a symlink) then
   `os.OpenFile(O_CREATE|O_EXCL|O_WRONLY|O_NOFOLLOW, 0600)`; reads open
   with `O_NOFOLLOW` and check the OPEN FD's owner/permission bits/
   regular-file-ness (no TOCTOU gap between check and read) before
   trusting the content. Tests: `TestGuardStamp_SessionScoped` (extended
   coverage of the empty-session no-op path via the pre/post integration
   tests, which never set a session in the paths that must stay inert).
5. **Pre-hook worst case ~4s against the ≤2s design budget.**
   `resolveServiceAndRel` and `fetchObligations` were two sequential
   calls, each with an independent 2s client timeout. Fixed:
   `fetchObligations` now takes an explicit `timeout time.Duration`
   parameter (the post path passes its pre-existing `2*time.Second`,
   byte-identical to before); the `--pre` path shares ONE `preHookBudget`
   (2s) across both calls, giving the second call whatever remains
   (floored at 200ms so a fast first call still leaves a meaningful
   attempt) rather than a fresh 2s.
6. **`module_rules_served` counted SERVES, not RULES; `GuardDedupSuppressed`
   only counted full suppression.** Fixed (preferred option): both beats
   now carry their actual count in meta (`"rules"`/`"count"`) and
   `research_report.go` SUMS it — `reportModuleBeats` computes a per-
   module rule count for the call, `dedup-suppressed` fires (with its
   count) on ANY suppression, partial or full, not only the
   full-suppression collapse case. No field rename needed — summing
   actual rule counts makes `ModuleRulesServed`'s existing name and doc
   comment accurate. Test: `TestBuildResearchReport_GuardedEditsAndModules`
   asserts the summed (not counted) values.
7. **`include_modules:false` was the design's one silent suppression.**
   Module rules were fetched (consuming shared server-side budget) then
   dropped with no line. Fixed: one line —
   `"N module rule(s) hidden (include_modules:false)"` — appended to the
   pre-edit render whenever `hiddenModuleRules > 0`. Test:
   `TestEditHookPre_ModuleTaggedRule_IncludeModulesToggle`'s
   `include_modules false` subtest, renamed and extended to assert the
   line.
8. **Root-mismatch note (folded into finding 5's fix).** The LOCAL
   fast-path guard match and the daemon-resolved authoritative identity
   can in principle disagree (e.g. a dogfood-O15-style rename); documented
   as a comment at the point `resolveServiceAndRel`'s result supersedes
   the local `repoRelFile` in `edithook_pre.go` — note-only, per the
   critic's own "unless trivial" guidance; no behavioral branch added.
9. **Stamp replay (folded into finding 4's fix).** `deleteGuardStamp` now
   runs immediately after the post-hook consults a stamp (success or
   not), making a stamp single-use — it can never be replayed for a
   second, unrelated edit of the same file.
10. **Two tests wrote to the REAL machine temp dir.**
    `TestEditHookPre_MatchedGuard_FetchesAndRenders` (a real `session_id`,
    no override) and `TestEditHookPre_ModuleTaggedRule_IncludeModulesToggle`
    (defensive fix) now override `guardStampDir` via the same
    `t.Cleanup`-restore pattern the dedup-integration tests already used.
    Verified directly: `rm -rf` the real stamp dir, re-run the full
    guard/stamp test set, confirm zero files reappear.

## Tasks

- [x] `schema/binding/schema.cue`: `#Bindings`/`#Binding` CUE schema for `.fathom/modules.cue` (module name, optional consumer-side `scopes` intersection, `max_rules` default 2, optional `min_tier` enum) {id: O-1, symbol: LoadBindings, package: internal/bindspec, skip_test: true, skip: "schema.cue is data, not a callable symbol; its own negatives are Go-asserted by TestBindingSchema_Negatives (internal/bindspec/schema_test.go) and its positives by TestBindingSchema_ValidInstancesConcrete — anchored at the loader that is this schema's actual consumer (O-2) per house convention for New CUE schemas"}
  - `schema/binding/testdata/valid.cue` (three worked examples: scoped +
    tiered, defaults-only, multiple bindings) and `testdata/invalid.cue`
    (five commented negatives: empty list, bad module name, non-positive
    `max_rules`, unrecognized `min_tier`, empty `scopes` list) —
    negatives are asserted in Go per `TestBindingSchema_Negatives`, never
    live-vetted, per `CLAUDE.md`'s "New CUE schemas" convention.
  - `cue vet ./schema/binding/...` is clean.

- [x] `internal/bindspec`: the dual-mode loader for `.fathom/modules.cue` — `LoadBindings` (lenient reader, unknown-field-tolerant, collects every violation into one `*LoadError` naming file+field, rejects duplicate module names across bindings — a Go-enforced rule the CUE schema cannot express cleanly) and `ValidateStrict` (closed-schema authoring gate); `ExampleTemplate()` renders the commented, inert `fathom init` scaffold {id: O-2, symbol: LoadBindings, package: internal/bindspec}
  - Mirrors `internal/modspec`'s `LoadManifest`/`ValidateStrict` split
    exactly (embedded schema drift gate, `FieldError`/`LoadError`,
    presence-checked default lookups for `max_rules`).
  - Tests (`internal/bindspec/binding_test.go`): `TestBindingSchemaMatchesSource`,
    `TestLoadBindings_Valid`, `TestLoadBindings_DefaultsAppliedWhenFieldAbsent`,
    `TestLoadBindings_MultipleBindings`, `TestLoadBindings_UnknownFieldIgnored`,
    `TestLoadBindings_Negatives` (six cases including the duplicate-module
    Go-enforced rule), `TestLoadBindings_MissingFile`,
    `TestValidateStrict_ValidPasses`, `TestValidateStrict_EmptyBindingsFails`,
    `TestExampleTemplate_IsCommentedNoOp` (proves the `fathom init` scaffold
    is valid, inert CUE — a required-field error, never a parse error).

- [x] `schema/guard/schema.cue`: `#Guards`/`#Guard` CUE schema for `.fathom/guard.cue` (`scopes`, `note`, `max_rules` default 4, `include_modules` default false, `command_scopes` — design §2.1/§2.5) {id: O-3, symbol: LoadGuards, package: internal/guardspec, skip_test: true, skip: "schema.cue is data; negatives Go-asserted by TestGuardSchema_Negatives, positives by TestGuardSchema_ValidInstancesConcrete; anchored at the loader (O-4) per house convention"}
  - `schema/guard/testdata/valid.cue` (file-scoped + note + modules,
    command-scoped, defaults-only) and `testdata/invalid.cue` (four
    commented negatives). `cue vet ./schema/guard/...` is clean.

- [x] `internal/guardspec`: the dual-mode loader for `.fathom/guard.cue` — `LoadGuards` additionally rejects a guard naming NEITHER `scopes` NOR `command_scopes` (dead-by-construction, Go-enforced since the CUE schema's either/or cannot be expressed cleanly); `ExampleTemplate()` renders the commented, inert scaffold {id: O-4, symbol: LoadGuards, package: internal/guardspec}
  - Tests (`internal/guardspec/guard_test.go`): `TestLoadGuards_Valid`,
    `TestLoadGuards_DefaultsAppliedWhenFieldAbsent`,
    `TestLoadGuards_CommandScopes`, `TestLoadGuards_UnknownFieldIgnored`,
    `TestLoadGuards_Negatives` (five cases including the neither-scopes
    Go-enforced rule), `TestLoadGuards_MissingFile`,
    `TestValidateStrict_ValidPasses`, `TestValidateStrict_EmptyGuardsFails`,
    `TestExampleTemplate_IsCommentedNoOp`.

- [x] `internal/semantic`: `Obligation` gains `Module` (module-tagged rule provenance, omitted for every repo-local rule); `Obligations` gains `DisplacedRepo`/`DisplacedModule`/`ModuleNotes` (`ModuleNote{Module, Note}`) — all additive, `omitempty`, populated only by `modserve.Bind`, never by `ObligationsFor` itself {id: O-5, symbol: Bind, package: internal/modserve}
  - Anchored at `modserve.Bind` (the function that actually sets these
    fields) per anchor lesson 1 — `semantic.ModuleNote`/`Obligation` are
    struct types, not valid `symbol:` targets.
  - Test: `internal/semantic`'s existing suite (unchanged, still green —
    every new field is `omitempty` and untouched by `ObligationsFor`)
    plus O-6's `modserve` tests, which are the actual behavioral proof.

- [x] `internal/modserve`: `Bind(ctx, host, root, file, max, repoObl)` — the binding-aware composition. Reads `.fathom/modules.cue` at `root` (fails OPEN — absent or malformed returns `repoObl` UNCHANGED, the hot serving path never blocks on a bad file); for each binding, intersects consumer `scopes` (empty = module's own scopes apply as-is) BEFORE any federated lookup (so an out-of-scope binding costs nothing); calls `host.FederatedObligations` capped at the binding's own `max_rules`; filters by `min_tier`; tags each surviving rule with the binding's module name; flags a rule's `Status` with its (module,service)'s staleness verdict when not fresh (critic finding 2a — `ModuleStaleness`, fetched once per binding, never clobbering a status the module's own store already set); fills the SHARED budget after repo rules (repo priority); an unmounted-but-in-scope binding produces an explicit `ModuleNote`, never silence {id: O-6, symbol: Bind, package: internal/modserve}
  - `ModuleFederator` is a narrow interface (mirrors `internal/mcp`'s own
    `moduleBackend` pattern) satisfied structurally by `*modulehost.Host`
    — `internal/modserve` never imports `internal/mcp`. Extended
    post-critic (finding 2a) with `ModuleStaleness`, the exact method
    `*modulehost.Host` already exposed — closing design §3.3's
    "staleness honesty end-to-end" requirement, which the first pass had
    left unimplemented (see Critic round above).
  - Tests (`internal/modserve/bind_test.go`, a scripted `fakeFederator`,
    zero real store/daemon): `TestBind_NoBindingFile_ReturnsUnchanged`,
    `TestBind_NilHost_ReturnsUnchanged`,
    `TestBind_UnionsModuleRules_TaggedWithModuleName`,
    `TestBind_UnmountedModule_ProducesExplicitNote`,
    `TestBind_ScopeIntersection_ConsumerScopeExcludesFile` (proves ZERO
    federator calls when the binding's own scope excludes the file),
    `TestBind_MinTierFloor`,
    `TestBind_RepoRulesKeepPriorityWithinSharedBudget` (proves the shared
    budget is not additive — module rules count INSIDE it),
    `TestBind_MultipleBindings_Deterministic`,
    `TestBind_DriftedModule_FlagsRules`, `TestBind_UnknownDetachedModule_FlagsRules`,
    `TestBind_FreshModule_NotFlagged`, `TestBind_StalenessNeverClobbersExistingStatus`,
    `TestBind_StalenessLookupFailure_FailsOpen` (critic finding 2a),
    `TestBind_MalformedBindingFile_FailsOpen`.

- [x] `internal/mcp`: `registerSemanticTools` gains a `modules moduleBackend` parameter (threaded from `newServer`/`registerTools`, nil on every daemon started without `--module`); the `obligations` tool handler resolves the service's on-disk root via the EXISTING `ServiceMeta` (the O15 mechanism) and calls `modserve.Bind`; the wire response gains `displaced_repo`/`displaced_module`/`module_notes` (additive, present only when non-zero/non-empty) {id: O-7, symbol: registerSemanticTools, package: internal/mcp}
  - `modules == nil` (the pre-phase-O default, and every daemon with no
    `--module` flags) makes this a complete, byte-identical no-op —
    proven directly.
  - Tests (`internal/mcp/binding_obligations_test.go`, exercising the
    real wiring `NewServerWithModules` -> `registerTools` ->
    `registerSemanticTools` -> `modserve.Bind`, not just `modserve`'s own
    unit tests): `TestObligationsTool_BoundModule_UnionsAndTags`,
    `TestObligationsTool_NoModulesWired_Unchanged`,
    `TestObligationsTool_UnmountedBinding_ExplicitNote`. Full
    pre-existing `internal/mcp` suite (obligations, autorefresh
    forwarding, surfaces) stays green.

- [x] `cmd/fathom`: `edit-hook --pre` — the guard hook's file-scoped path (`Edit`/`Write`/`MultiEdit`). LOCAL fast path first: `serviceCandidates` (no daemon) resolves the repo-relative path, `loadGuardSet` reads `.fathom/guard.cue` from disk, `matchingFileGuard` glob-matches — a miss returns silently with ZERO HTTP calls. A match resolves authoritative service identity (`candidatesWithDaemon`, the ONE call this path ever makes on a hit), fetches obligations capped at the guard's `max_rules`, renders note-first + module-tag-aware bullets (`writeRuleBullet`, shared with the post path), filters module rules by `include_modules` for DISPLAY only, writes a guard stamp keyed on what was actually shown {id: O-8, symbol: cmdEditHookPreFile, package: cmd/fathom}
  - Critic round fixes (BLOCKING/cheap, see Critic round above):
    latency (finding 5) — `fetchObligations` now takes an explicit
    `timeout`; the two sequential calls this path makes share ONE
    `preHookBudget` (2s) instead of a fresh 2s each. Root-mismatch note
    (finding 8) — documented at the point the daemon-resolved identity
    supersedes the local guard-match root. Silent suppression (finding 7)
    — `include_modules:false` now appends "N module rule(s) hidden
    (include_modules:false)" instead of dropping them with no line.
  - Tests (`cmd/fathom/edithook_pre_test.go`, httptest counter pattern):
    `TestEditHookPre_NoGuardFile_ZeroDaemonCalls`,
    `TestEditHookPre_GuardFileButNoMatch_ZeroDaemonCalls` (the load-bearing
    proof: a guard.cue EXISTS but doesn't match — still zero calls),
    `TestEditHookPre_MatchedGuard_FetchesAndRenders` (asserts the exact
    `max_rules` sent, note-before-rule ordering; extended post-critic with
    a `guardStampDir` override, finding 10),
    `TestEditHookPre_ModuleTaggedRule_IncludeModulesToggle` (both states;
    the `include_modules:false` subtest extended to assert the hidden-count
    line, finding 7).

- [x] `cmd/fathom`: `edit-hook --pre` — the guard hook's command-scoped path (`Bash`, design §2.5). Same local-fast-path doctrine: `matchingCommandGuard` against `.fathom/guard.cue`'s `command_scopes` (a custom glob-to-regexp matcher, `commandMatches`/`globToRegexp` — `'*'` must cross `/` for a command string, unlike `filepath.Match`); a match injects ONLY the guard's `note` — no daemon call at all on this path (no file to scope an obligations lookup to) {id: O-9, symbol: cmdEditHookPreCommand, package: cmd/fathom}
  - Tests: `TestEditHookPre_CommandGuard_Matches` (asserts zero daemon
    calls even on a match), `TestEditHookPre_CommandGuard_NoMatch_Silent`,
    `TestCommandMatches_StarCrossesSlash` (`cmd/fathom/guard_test.go`),
    `TestMatchingCommandGuard`.

- [x] `cmd/fathom`: the local guard-matching layer (`guard.go`) — `loadGuardSet` (fails open on absent/malformed, doctor's job to be loud), `matchingFileGuard`/`matchingCommandGuard` (first-match-wins, file order, deterministic), `commandMatches`/`globToRegexp` {id: O-10, symbol: matchingFileGuard, package: cmd/fathom}
  - Tests (`cmd/fathom/guard_test.go`): `TestLoadGuardSet_Absent`,
    `TestLoadGuardSet_Malformed_FailsOpen`, `TestLoadGuardSet_Valid`,
    `TestMatchingFileGuard`, `TestMatchingCommandGuard`,
    `TestCommandMatches_StarCrossesSlash`.

- [x] `cmd/fathom`: the pre/post dedup stamp (`guardstamp.go`) — `writeGuardStamp`/`readGuardStamp` (session-scoped path under `os.TempDir()`; a no-op with no `session_id`, critic finding 4), `deleteGuardStamp` (single-use, finding 9), `dedupAgainstStamp` (computes the delta: rules newly matched since the pre-hook fired, plus any staleness change; module-tagged rules disambiguated by `(module, rule, src)` so a same-text rule from a different module is never wrongly suppressed) {id: O-11, symbol: dedupAgainstStamp, package: cmd/fathom}
  - Critic round fixes (finding 4, BLOCKING → hardening, see Critic round
    above): `session_id == ""` skips dedup ENTIRELY (never relies on
    `guardStampTTL` alone as the leakage backstop against a predictable
    key); writes go through `os.Remove` (never follows a symlink) then
    `os.OpenFile(O_CREATE|O_EXCL|O_WRONLY|O_NOFOLLOW, 0600)`; reads open
    with `O_NOFOLLOW` and check the OPEN FD's owner/permission-bits/
    regular-file-ness (no TOCTOU gap) before trusting the content.
  - Tests (`cmd/fathom/guardstamp_test.go`): `TestGuardStamp_WriteThenRead`,
    `TestGuardStamp_AbsentWhenNeverWritten`, `TestGuardStamp_ExpiresAfterTTL`,
    `TestGuardStamp_SessionScoped` (the leakage-bound proof: two sessions
    on the same file never share a stamp), `TestDedupAgainstStamp_*` (four
    cases: full suppression, delta-only, staleness-always-surfaced,
    module-tag disambiguation). No dedicated forged-stamp unit test was
    added beyond the ownership/O_NOFOLLOW code path itself — the
    session-required gate (finding 4's primary fix) removes the
    predictable-key attack surface the forgery scenario depended on.

- [x] `cmd/fathom`: `edit-hook` (default/post path) consults the stamp before rendering — a valid stamp collapses to a one-line "already served pre-edit, unchanged" marker when nothing changed, or renders only the delta plus a "N rule(s) already served" note otherwise; every `emitBeat` call in both paths gains a `phase: "pre"|"post"` meta key {id: O-12, symbol: cmdEditHook, package: cmd/fathom}
  - Critic round fixes (finding 6, cheap): `dedup-suppressed` now fires
    (with its actual count in meta) on ANY suppression — partial or
    full — not only the full-suppression collapse case, so
    `GuardDedupSuppressed` (O-22) counts real suppressed-rule totals.
    Finding 9: `deleteGuardStamp` runs immediately once a stamp is
    consulted, making it single-use.
  - Tests (`cmd/fathom/edithook_pre_test.go`):
    `TestEditHookPreThenPost_Dedup` (the end-to-end handoff: pre serves +
    stamps, post collapses to the marker, never repeats the rule),
    `TestEditHookPreThenPost_DifferentSession_NoDedup` (the leakage
    bound: a different `session_id` sees the full rule). Full
    pre-existing `cmd/fathom/edithook_precision_test.go` +
    `compile_edithook_test.go` suites stay green (the post path's
    no-stamp behavior is byte-identical to pre-phase-O).

- [x] `cmd/fathom/init.go`: registers `PreToolUse` under TWO matcher groups (`Edit|Write|MultiEdit` and `Bash`, same `edit-hook --pre` command) — required `ensureHookEntry`'s "already wired" marker search to be scoped to its OWN matcher group rather than the whole event array (a real bug this row's own test caught: the original event-wide search would see the first group's identical command and wrongly skip wiring the second) {id: O-13, symbol: wireHooks, package: cmd/fathom}
  - Test: `TestCmdInit_WiresRepoEndToEnd` (extended) asserts BOTH
    `PreToolUse` matcher groups are present with one hook each, not just
    that the marker string appears once.

- [x] `cmd/fathom/init.go`: scaffolds `.fathom/guard.cue` and `.fathom/modules.cue` — write-if-absent, ALWAYS the commented, entirely-inert `ExampleTemplate()` output (no active config by default, per both designs' deliverable 1) {id: O-14, symbol: writeFathomConfigScaffolds, package: cmd/fathom}
  - Tests: `TestCmdInit_WiresRepoEndToEnd` (extended: asserts every
    non-empty line in both scaffolded files is a `//` comment),
    `TestCmdInit_Idempotent` (extended: "already present" on re-run).

- [x] `cmd/fathom/doctor.go`: `doctorWarn` status — additive to `pass|fail|skipped`, visually and machine-distinguishable (`statusLabel`, JSON `summary.warn`), NEVER fails the exit code (a config-quality signal, not a wiring-broken verdict) {id: O-15, symbol: doctorExitCode, package: cmd/fathom}
  - Test: `TestDoctorExitCode_WarnDoesNotFailTheGate`
    (`cmd/fathom/doctor_phaseo_test.go`). Full pre-existing
    `TestCmdDoctor_AllPass`/`_DeadDaemon`/`_JSON` suite stays green
    (every new phase-O check reports `pass` on the common
    nothing-configured fixture, `0 fail` unchanged).

- [x] `cmd/fathom/doctor_phaseo.go`: `binding_file`/`guard_file` — loud (fail) on a malformed `.fathom/modules.cue`/`.fathom/guard.cue`; honest `pass` (never skipped) on absence, the common case {id: O-16, symbol: checkBindingFile, package: cmd/fathom}
  - Tests: `TestCheckBindingFile_Absent`, `TestCheckBindingFile_Valid`,
    `TestCheckBindingFile_Invalid_Loud`, `TestCheckGuardFile_Absent`,
    `TestCheckGuardFile_Invalid_Loud`.

- [x] `cmd/fathom/doctor_phaseo.go`: `binding_modules_mounted` — loud (fail) when a binding names a module `list_modules` does not report mounted on this daemon (the config-time catch complementing the serving-time explicit-note doctrine, O-6) {id: O-17, symbol: checkBindingModulesMounted, package: cmd/fathom}
  - Tests: `TestCheckBindingModulesMounted_NoBindings_Pass`,
    `_DaemonDown_Skipped`, `_AllMounted_Pass`, `_Unmounted_Loud`.

- [x] `cmd/fathom/doctor_phaseo.go`: `binding_module_readiness` — warn when a bound module's phase-J readiness ladder verdict (`runDoctor`, reused directly) is below `servable`, checked ONLY when the module's own manifest is locally resolvable (a `module.cue` next to the mounted store path, or inside it — the one convention `fathom module compile`'s own typical layout produces); honestly `skipped: "not checkable: module not local"` otherwise, never a guessed verdict {id: O-18, symbol: checkBindingModuleReadiness, package: cmd/fathom}
  - Tests: `TestCheckBindingModuleReadiness_NotLocal_Skipped`,
    `TestCheckBindingModuleReadiness_LocalBelowServable_Warn` (a real
    `runDoctor` call against a deliberately minimal local `module.cue`,
    verdict `none`, well below `servable`).

- [x] `cmd/fathom/doctor_phaseo.go`: `guard_dead_scopes` — warn when a file-scoped guard's globs match zero `list_sources` entries (a prune candidate, reported per law 8, never auto-removed); command-scoped guards are honestly noted as not checkable this way (no "indexed-commands" concept exists), never silently skipped {id: O-19, symbol: checkGuardDeadScopes, package: cmd/fathom}
  - Tests: `TestCheckGuardDeadScopes_NoGuards_Pass`,
    `_MatchesIndexedFile_Pass`, `_MatchesNothing_Warn`,
    `_CommandOnlyGuard_NotedNotFailed`.

- [x] `cmd/fathom/doctor_phaseo.go`: `guard_include_modules_unbound` — warn when a guard sets `include_modules:true` but the repo has no (or no valid) platform-module bindings to include {id: O-20, symbol: checkGuardIncludeModulesUnbound, package: cmd/fathom}
  - Tests: `TestCheckGuardIncludeModulesUnbound_NoIncludeModules_Pass`,
    `_WithBindings_Pass`, `_NoBindings_Warn`.

- [x] `cmd/fathom/edithook.go` + `research_report.go`: instrumentation gains a `phase: "pre"|"post"` beat dimension (every `emitBeat` call in both hook paths) and a `module-served` beat kind (`reportModuleBeats`, carrying each module's RULE count for this call in meta, critic finding 6 — not one beat per invocation) {id: O-21, symbol: reportModuleBeats, package: cmd/fathom}
  - Critic round fix (finding 6, cheap): `reportModuleBeats` originally
    emitted one beat per distinct module (a SERVE count), while the
    field it feeds (`ModuleRulesServed`) is named and documented as a
    RULE count (design §3.5). Fixed by carrying the actual per-module
    rule count in the beat's `"rules"` meta key, summed by O-22's
    aggregator — no field rename needed.
  - No dedicated test beyond O-8/O-9/O-12's own coverage (the beats these
    rows already assert fire correctly) — `reportModuleBeats` itself is
    exercised transitively by `TestEditHookPre_ModuleTaggedRule_IncludeModulesToggle`
    and `TestObligationsTool_BoundModule_UnionsAndTags`'s edit-hook-level
    twin; O-22's `TestBuildResearchReport_GuardedEditsAndModules` asserts
    the summed count reaches the report correctly.

- [x] `cmd/fathom/research_report.go`: `buildResearchReport` aggregates the new beat dimensions into `GuardPreSeen`/`GuardPreServed`/`GuardPostSeen`/`GuardPostServed`/`GuardDedupSuppressed`/`GuardCoverageNote` and `ModuleRulesServed`/`ModuleServingNote` — a phase-less beat (every log entry that predates this feature) defaults to `"post"` (exactly what it always was); an explicit `"not measurable: …"` note replaces every fabricated zero when the `--pre` hook has never fired, or no module rule was ever served, in the reporting window {id: O-22, symbol: buildResearchReport, package: cmd/fathom}
  - Critic round fix (finding 3, BLOCKING): the legacy `InjectionSeen`/
    `InjectionAnswered`/`ObligationsServed` counters (and
    `cmd/fathom/instrument.go`'s SEPARATE `reportInstrument` aggregator,
    its own `injection coverage (7d)` gauge + ≥90% gate) originally
    summed BOTH phases together — since `--pre` now fires on `Bash` too,
    a repo re-running `fathom init` would have silently rebased this
    gauge's denominator, making every phase-O-era number incomparable to
    every pre-phase-O number in `docs/plans/activation.md`. Both
    aggregators now skip `phase == "pre"` beats when computing the
    legacy counters — a beat with no phase key (predates this feature)
    is still counted, preserving old-log compatibility exactly. Finding
    6's second half: `dedup-suppressed` now sums its per-beat `"count"`
    meta (partial suppressions, not only full) rather than counting one
    per invocation.
  - Tests (`cmd/fathom/research_report_test.go`):
    `TestBuildResearchReport_GuardedEditsAndModules` (pre/post split,
    dedup-suppressed count, per-module tally, AND the pre-existing
    combined counters' backward compatibility all in one — extended
    post-critic to assert a `phase:"pre"` beat does NOT move
    `InjectionSeen`), `TestBuildResearchReport_GuardedEditsNeverFired`
    (the honest-absence note). Pre-existing `TestBuildResearchReport_Golden`
    (phase-less fixture beats) and `cmd/fathom/instrument_test.go`'s
    `TestReportInstrument_CoverageAndAlarms` (phase-less fixture beats,
    proving `reportInstrument`'s fix is backward-compatible too) stay
    green unchanged.

- [x] `cmd/fathom/research_report_render.go`: the text/markdown renderer gains a "guarded-edit coverage" and "platform-module serving" section — both print their honest note when unmeasured, a sorted per-module table otherwise {id: O-23, symbol: renderResearchReportText, package: cmd/fathom, skip_test: true, skip: "pure rendering over ResearchReport fields O-22's tests already populate and assert; renderResearchReportText's OWN pre-existing test coverage (renderResearchReportMarkdown wraps it verbatim) is unchanged by this addition, and this row adds no new field, only new print statements over fields already proven correct"}

- [x] Docs: `docs/use-cases/platform-modules.md` (house use-case format: When to use this / Tools / How it enhances Claude Code / Worked example / Setup & notes / Maintained — covers BOTH companion designs per the task's own instruction), linked from `docs/use-cases/README.md`'s guide table and `docs/use-cases/coding-agents.md`'s Maintained line {id: O-24, symbol: LoadBindings, package: internal/bindspec, skip_test: true, skip: "documentation-only row; the tools/fields it documents (obligations' module/displaced_repo/displaced_module/module_notes, edit-hook --pre) are proven by O-2/O-7/O-8/O-9's own tests"}

- [x] Docs: `docs/design/coding-agent-excellence.md` gains a channel-table row (M5b) and a §5.2 PreToolUse bullet; `docs/mcp-tools.md`'s `obligations` row documents the module-tagged/`displaced_repo`/`displaced_module`/`module_notes` additions; `ADR.md`'s ADR-024 gains a phase-O amendment paragraph pointing at `docs/design/platform-modules.md` {id: O-25, symbol: LoadBindings, package: internal/bindspec, skip_test: true, skip: "documentation-only row; cross-referenced by file:line in this plan's own prose above, not independently testable"}

- [x] Docs: `docs/adoption/claude-code.md` gains §7.5 (the PreToolUse guard hook, JSON snippet, dedup note); `docs/adoption/codex.md`'s capability table gains the guard-injection row ("No equivalent... a bound module's rules still reach Codex via `obligations`/`pack_context`") {id: O-26, symbol: LoadGuards, package: internal/guardspec, skip_test: true, skip: "documentation-only row"}

- [x] Skills: `.claude/skills/fathom-onboard/SKILL.md` and `.claude/skills/fathom-doctor/SKILL.md` updated (hook stack table gains the guard row, `.fathom/` section gains guard.cue/modules.cue, checklist table gains the six phase-O doctor checks, three->four statuses); `dist/agents/skills/{fathom-onboard,fathom-doctor}/SKILL.md` kept byte-identical (`diff` clean); both `codex-prompt.md` mirrors updated with the capability-gap row and check count {id: O-27, symbol: LoadGuards, package: internal/guardspec, skip_test: true, skip: "documentation-only row; byte-identity between .claude/skills and dist/agents/skills verified directly via `diff` (clean, both pairs) at authoring time — the flywheel worker's own precedent for this class of row"}

- [x] `docs/plans/gap1-instrumentation.md`: GI-8 ("PreToolUse grep-hint revival") closed — `disposition: refused`, reason pointing at `docs/design/guarded-edits.md` as the sharper mechanism that actually ships the PreToolUse channel GI-8 asked for (a different, narrower shape: configured critical scopes, not an evidence-gated grep-hint) {id: O-28, symbol: LoadGuards, package: internal/guardspec, skip_test: true, skip: "documentation-only row; the closed disposition line is the acceptance evidence itself"}

## Deferred (named triggers, not silently dropped — both designs' own §4/§3)

- **SessionStart brief line naming each bound module** (design §3.3's
  "Session brief" bullet: "the SessionStart payload names each bound
  module with its rule count and staleness verdict") — `sessionhook.go`
  is UNCHANGED this phase; critic finding 2b (phase-O merge) moved this
  here explicitly rather than leaving it silently unclaimed. **Trigger:**
  a real bound module in a real consumer repo, so the wiring is proven
  against actual usage before session-start real estate is spent naming
  a binding nobody has exercised yet.
- **Module rules as eligible evidence in `module_query`/`pack_context`/
  `brief`, and via `pkg/client.Client.Obligations`** (design §3.3's
  second bullet) — `modserve.Bind` has exactly ONE caller today
  (`internal/mcp/semantic_tools.go`'s obligations tool handler, over the
  MCP wire protocol); a caller using `pkg/client` directly (not through
  MCP) gets no bound-module evidence at all, and the three named tools
  never consult bindings. Critic finding 2b moved this here explicitly.
  **Trigger:** a documented consumer of `pkg/client` directly (bypassing
  the MCP wire protocol) that needs bound-module evidence, or a concrete
  `module_query`/`pack_context`/`brief` case where a bound module's
  evidence would have changed the answer.
- **Auto-binding / org-push of bindings** — no mechanism exists or is
  planned; binding is always the consumer repo's own committed opt-in
  (platform-modules.md §4).
- **Path remapping between a module's scope globs and a consumer's tree**
  — v1 requires the module's scopes to already be written
  consumer-shaped; a remap field is deferred to evidence, not
  speculation (platform-modules.md §3.2).
- **Module rules in `verify_plan` gating** — a plan row is proven by the
  repo's own evidence only; module rules stay advisory, never promoted
  (platform-modules.md §3.4).
- **A dedicated standards-conformance review dimension** (`review_diff`)
  — phase-O v1 serves module rules into the existing obligations
  dimension; deferred until the first documented case of a module rule
  review served but a reviewer needed machine-checked enforcement for
  (platform-modules.md §3.3).
- **Auto-derived guards** — an LLM proposing "this file looks important"
  is phase-L advisor territory, eval-gated, lands only as a *suggested*
  diff (guarded-edits.md §3).
- **Blocking guards / per-user guards** — inject-only is the doctrine;
  blocking stays L1's job, revisited only with a documented incident a
  warning demonstrably failed to prevent (guarded-edits.md §2.2/§3).
- **Bench honesty (both designs' own §6/adoption-claim deferral).** Phase
  O ships instrumented (O-21/O-22); its adoption claim — "is this
  actually reducing incidents / actually being read" — waits for
  qualifying traffic like every other law-8-gated channel in this
  program. No adoption claim is made by this plan.
