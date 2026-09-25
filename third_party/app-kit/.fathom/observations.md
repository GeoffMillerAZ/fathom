<!-- fathom-observations:v1 -->
# Fathom observations

A project-local friction log, fed by `fathom observe` — the flywheel's
intake (agents are the primary users, so agent-filed observations are the
flywheel's fuel; docs/plans/pack-taxonomy-adoption.md). Entries graduate
into fixes (and a CHANGELOG entry) or are closed with a recorded reason;
`fathom instrument --adoption` reports how many are still open.

Entry shape: **symptom → evidence → root cause → fix → status → impact**
(docs/dogfood-observations.md's own convention, scoped to this repo).
Append-only: `fathom observe` only ever adds a new entry at the end —
correct an entry by hand (root cause / fix / status) once it's understood,
never by re-running the command over it.

Entry ids (O1, O2, …) are **advisory under concurrency**: two sessions
filing at once can mint the same O<N> — no entry is ever lost, and file
order is chronological (append-only), so disambiguate colliding ids by
their order in this file (later entry = later filing) and renumber at
triage if it matters.

## Observations (status recorded per entry)

### O1 — MCP references rejects literal documentation paths although routing needs documentation discovery

- **Symptom:** MCP references rejects literal documentation paths although routing needs documentation discovery
- **Evidence:**
  - **Tool:** references
  - **Expected:** Clear literal path lookup or instruction to use regex for docs/adr
  - **Actual:** MCP -32602 rejected docs/adr; file inventory used after index routing
  - **Repro:** references({query:docs/adr,services:[app-kit],max:30}); identifier-only contract requires regex:true for slash paths
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O2 — references includes binary executable noise in occurrence results

- **Symptom:** references includes binary executable noise in occurrence results
- **Evidence:**
  - **Tool:** references
  - **Expected:** Source and documentation occurrences for registerComponentTools
  - **Actual:** Results include appkit-mcp executable lines with null and nontext bytes alongside tools.go and main.go
  - **Repro:** references({services:["app-kit"],query:"registerComponentTools",max:30}) reports appkit-mcp:34435 and appkit-mcp:196012
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O3 — symbol_context untyped callees include an unrelated interface implementation

- **Symptom:** symbol_context untyped callees include an unrelated interface implementation
- **Evidence:**
  - **Tool:** symbol_context
  - **Expected:** Importer call context without unrelated analytics test writer methods
  - **Actual:** designassets.Import untyped callees includes appkit/ui/analytics/failingWriter.Write; typed_edges.callees correctly omits it; reported by CLI test agent
  - **Repro:** symbol_context({"service":"app-kit","symbol":"designassets.Import"}) at internal/designassets/store.go:110 in current working tree
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O4 — Exact module-path reference lookup misses known package imports

- **Symptom:** Exact module-path reference lookup misses known package imports
- **Evidence:**
  - **Tool:** references
  - **Expected:** A module-path search returns the real package import occurrences, or clearly identifies exact-definition versus prefix matching
  - **Actual:** Agent observed exact_available:true and go.mod definition with occurrences:[] for full module path; shorter agentic-kit/ query found CA imports and also GGB matches despite services filter
  - **Repro:** references({services:["car-assistant"],query:"github.com/geoffmilleraz/agentic-kit",regex:true,max:40}) versus query:"agentic-kit/"; bounded G4 source scout on 2026-09-13. Root cause unconfirmed.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O5 — Reference search emits generated executable binary lines

- **Symptom:** Reference search emits generated executable binary lines
- **Evidence:**
  - **Tool:** references
  - **Expected:** Source/config/doc occurrences without ignored compiled appkit-mcp executable matches
  - **Actual:** get_kit_css_setup query returned appkit-mcp:32762 and appkit-mcp:32940 string-table fragments alongside real source, wasting bounded results
  - **Repro:** references({services:["app-kit"],query:"get_kit_css_setup",max:12}) on 2026-09-13; ignored root appkit-mcp is generated build output
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O6 — Stale obligation source lines no longer support returned rule

- **Symptom:** Stale obligation source lines no longer support returned rule
- **Evidence:**
  - **Tool:** obligations
  - **Expected:** Rule citation and snippet support the served rule, or stale rule not served as governing evidence
  - **Actual:** Use go tool templ and go tool cue rule cites CLAUDE.md#L46-L47 whose snippet is For material UI work, choose a complete visual reference separately from the renderer; status stale
  - **Repro:** obligations({"service":"app-kit","file":"apps/gallery/shell_demo_render.go"}) also reproduced by asset subagent on shell_live_assets.go and shell_live_assets_test.go
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O7 — Shipped app-kit Fathom pack contradicts accepted runtime and design-fidelity decisions

- **Symptom:** Shipped app-kit Fathom pack contradicts accepted runtime and design-fidelity decisions
- **Evidence:**
  - **Tool:** layers
  - **Expected:** Guard and semantic prompt preserve originals and follow ADR-0022/0023 consumer profiles
  - **Actual:** v0.1 guard-pack and v1 prompt instruct dark-variant deletion and prohibit client tooling for all Go hosts
  - **Repro:** Read fathom-pack/guard-pack.cue and fathom-pack/layers/ui-appkit/prompt.md alongside docs/adr/0022-private-design-references-and-fidelity.md and 0023-profile-specific-conversion-and-client-compute.md
- **Root cause:** Distributed v0.1 guidance was not updated with the accepted fidelity and profile decisions.
- **Fix:** v0.2 guards and v2 semantic prompt preserve originals and select profiles; seven `TestFathom*` checks include retired-instruction and deletion canaries. Config audit refused four seeded violations, then passed after repair.
- **Status:** closed for authored/staged guidance; evaluation and already-served stale obligations remain separate open work (O6).
- **Impact:** The distributed source no longer recommends the demonstrated destructive or universal-runtime choices. This is not semantic precision/recall evidence.

### O8 — layers install usage documents flags after a positional path but parser rejects that order

- **Symptom:** layers install usage documents flags after a positional path but parser rejects that order
- **Evidence:**
  - **Tool:** layers install
  - **Expected:** Documented install <dir> --root DIR invocation accepts its root flag
  - **Actual:** requires exactly one pack directory; flags-before-path succeeds
  - **Repro:** fathom layers install fathom-pack/layers/ui-appkit --root . versus fathom layers install --root . fathom-pack/layers/ui-appkit
- **Root cause:** Observed parser stops flag processing at the positional path; upstream implementation not changed here.
- **Fix:** Document and use `fathom layers install --root . fathom-pack/layers/ui-appkit` (flags first).
- **Status:** open upstream; local workaround verified by successful staging.
- **Impact:** No repeated install retries or forced evaluation bypass needed.

### O9 — Reconcile records pre-refresh skill digests and its own refreshed files immediately fail doctor wiring_drift

- **Symptom:** Reconcile records pre-refresh skill digests and its own refreshed files immediately fail doctor wiring_drift
- **Evidence:**
  - **Tool:** fleet reconcile
  - **Expected:** Successful owned-skill refresh records the bytes it installed and remains idempotent
  - **Actual:** 17 stale digest warnings immediately after receipt 7fa69ddc8d69 although all nine skills are reported byte-current
  - **Repro:** fathom fleet reconcile --repo . --apply; fathom doctor --harness codex --service app-kit --json; skills_installed passes while wiring_drift rejects refreshed owned skills
- **Root cause:** Ownership manifest retained pre-refresh digests for 17 tool-refreshed skill files.
- **Fix:** Reviewed owned-file diff, then repo-local `--rebaseline`; doctor matches all 41 owned entries and reconcile dry-run reports zero drift. No global configuration change.
- **Status:** open upstream; local recovery complete. Rebaseline is not an automatic response to unexplained drift.
- **Impact:** Current host wiring is clean without discarding user-owned edits; future refreshes may reproduce the tool defect.

### O10 — Local executable exclusion accidentally hid the appkit source directory

- **Symptom:** Local executable exclusion accidentally hid the appkit source directory
- **Evidence:**
  - **Tool:** ingest
  - **Expected:** Kit source and TestVizEmitsNoClassAttribute remain indexed while root executables are ignored
  - **Actual:** Root /appkit pattern hid the appkit directory; 3117 symbols and exact test miss; removing it restored 4547 symbols and the exact test hit
  - **Repro:** Compare .fathomignore /appkit with the appkit/ source directory; ingest app-kit then find_symbol TestVizEmitsNoClassAttribute before and after removal
- **Root cause:** `/appkit` matched the source directory, not an executable at that name.
- **Fix:** Keep only `/appkit-mcp` and `/gallery` root-executable exclusions; add `TestFathomIgnoreKeepsSourceRoots` and re-ingest.
- **Status:** closed; exact `TestVizEmitsNoClassAttribute` lookup and its runtime test pass after repair (INC-12).
- **Impact:** Restored indexed kit source. The bounded `get_kit_css_setup` reference sample also contains no root executable; it is not an exhaustive search-engine certification.

### O11 — Capabilities says guards are off although doctor and live hooks parse six valid CUE guards

- **Symptom:** Capabilities says guards are off although doctor and live hooks parse six valid CUE guards
- **Evidence:**
  - **Tool:** capabilities
  - **Expected:** Count guards using the same semantic CUE loader as doctor and preserve invalid or unavailable states
  - **Actual:** Config panel and local fallback count literal note: text and miss quoted labels; app-kit has six quoted note fields and gets zero
  - **Repro:** cmd/fathom/configpanel.go buildConfigPanel and capabilityprobe.go countGuardRules use textual counts; .fathom/guard.cue quoted labels reproduce capabilities off versus doctor six guards
- **Root cause:** Textual `note:` counting disagreed with the semantic guard loader, including quoted labels and rules with no note.
- **Fix:** Generic Fathom source contribution uses one read-manifest-backed CUE parse for both surfaces, preserving missing, empty, malformed and unreadable states. Positive and negative CLI/config-panel integration tests pass under race detection.
- **Status:** source repair verified; not installed or deployed. The isolated candidate reports `6 guard rules active` against this repo; the existing installed binary still carries the defect.
- **Impact:** Candidate diagnostics agree with the six parsed guards. This does not certify semantic obligations, host coverage, or UI outcomes.

### O12 — An unresolved bare symbol_context Run query returned a very large cross-service caller set

- **Symptom:** An unresolved bare symbol_context Run query returned a very large cross-service caller set
- **Evidence:**
  - **Tool:** symbol_context
  - **Expected:** An unresolved or ambiguous bare name should produce bounded scoped guidance or candidates
  - **Actual:** service app-kit, symbol Run returned def:null and unrelated test Run callers across many services; combined output exceeded 63000 tokens. Caller should use exact symbol key, but unresolved result should still be bounded.
  - **Repro:** symbol_context({service:app-kit,symbol:Run,snippet:true}); compare exact RenderStatusTable query, which returns its definition and relevant callers
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O13 — references advertised fleet-wide occurrence search but an identifier type query returned no occurrences with an undocumented resolved-only filter

- **Symptom:** references advertised fleet-wide occurrence search but an identifier type query returned no occurrences with an undocumented resolved-only filter
- **Evidence:**
  - **Tool:** references
  - **Expected:** ChannelFunnel type mentions in cmd/ops/channels.go and channels_test.go, or explicit routing/filter guidance in the advertised tool contract
  - **Actual:** references query ChannelFunnel with services resume_as_code word true returned count 0, one verified definition, edge_filter resolved; source has return type, map and literal references. ChannelYield query returned seven call occurrences as a positive control.
  - **Repro:** After successful ingest(service=resume_as_code), call references({query:ChannelFunnel,services:[resume_as_code],word:true,max:20}); compare known cmd/ops/channels.go and channels_test.go mentions and references query ChannelYield. Repo HEAD 7a6c56a26dd10a7cf80128d23ef13bab90377ae8 plus documented Pipeline working-tree changes. Root cause and remedy not yet established.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O14 — Tailwind topic search omitted the actual CSS build implementation and returned stale unrelated records after service refresh

- **Symptom:** Tailwind topic search omitted the actual CSS build implementation and returned stale unrelated records after service refresh
- **Evidence:**
  - **Tool:** query_symbols
  - **Expected:** Relevant internal/tailwind implementation or its CLI callers
  - **Actual:** Top four: cli-kit doc, twplus normalize.Build, adoption.BuildEvidence and twplus index.Builder; reingest recommended with 49 stale sources
  - **Repro:** app-kit HEAD 63b3a4f; query_symbols service=app-kit concern=tailwind keywords=[build,CLI] top=4; fallback rg --files internal/tailwind found tailwind.go
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O15 — Fathom navigation handoff checkout is not the recorded fathom service root

- **Symptom:** Fathom navigation handoff checkout is not the recorded fathom service root
- **Evidence:**
  - **Tool:** list_services
  - **Expected:** Index identity matching the consumer checkout before navigation source discovery
  - **Actual:** list_services records fathom at a scratchpad pushclone; prime in sextant reports checkout unindexed; no-path ingest refreshed the recorded clone
  - **Repro:** On 2026-09-14 list_services({}) reports fathom path /private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946-4037-4a56-8b48-3336a3490e6f/scratchpad/pushclone while the authorized checkout is /Volumes/m2ext/code/geoffmilleraz/sextant. app-kit service still matches its canonical checkout. Root cause beyond recorded mapping not established; no shared reroot attempted.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O16 — obligations returns stale governing-rule source snippets after ingest

- **Symptom:** obligations returns stale governing-rule source snippets after ingest
- **Evidence:**
  - **Tool:** obligations
  - **Expected:** Current governing rule source and current watermark after app-kit ingest, or explicit actionable refresh path
  - **Actual:** file apps/receipts/render.go returns a stale tool-directive rule at CLAUDE.md L46-L47 with snippet about complete visual references; compiled watermark6bf5e4b differs from HEAD63b3a4f; read current intent instead
  - **Repro:** ingest({"service":"app-kit"}); obligations({"service":"app-kit","file":"apps/receipts/render.go"}) at app-kit HEAD63b3a4f01168b184f91d1e701414445820e24fdc
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O17 — Fathom service refresh resolves scratch checkout during authorized Observatory upgrade

- **Symptom:** Fathom service refresh resolves scratch checkout during authorized Observatory upgrade
- **Evidence:**
  - **Tool:** ingest
  - **Expected:** Current consumer source at sextant is the indexed Fathom repository or tool clearly signals the mismatch
  - **Actual:** ingest service fathom returns scratchpad/pushclone root; fathom prime in sextant reports this repo unindexed; source inspection fallback required
  - **Repro:** 2026-09-14: ingest({service:fathom}) returned /private/tmp/claude-501/.../scratchpad/pushclone; authoritative sextant HEAD e1841718 and mirror bcfeb8e verified directly. Existing continuity notes report the same drift; no re-root performed.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O18 — Current .mjs fixture functions absent from refreshed symbol index

- **Symptom:** Current .mjs fixture functions absent from refreshed symbol index
- **Evidence:**
  - **Tool:** outline
  - **Expected:** outline includes verify and exported verifyHardenedReference from the selected source file
  - **Actual:** outline returned an empty symbols array; find_symbol verifyHardenedReference also returned no match after ingest, while the source defines them
  - **Repro:** ingest service app-kit; outline service app-kit file docs/assessments/fixtures/catalyst-hardened/reference.mjs; find_symbol service app-kit name verifyHardenedReference. Exact file read used for this private-source verifier; no index-enforcement claim.
  - **Recurrence 2026-09-16:** after canonical app-kit ingest, outline of `spikes/wails-gallery/native_delivery_probe.mjs` returns `symbols:[]` despite exported `parseArgs`, `assertStrictState` and `bundleLayoutForLauncher`. Same-session Go outline of `companion_input_probe_test.go` returns its functions. Exact known-file source reads remain necessary; no daemon/config changes attempted.
  - **Recurrence 2026-09-17 submission controls:** after successful ingest, outline for `apps/styleguide/browser/cancellation.mjs` and `cdp_guard.mjs` returns empty symbols; the latter directly defines exported async `connectGuard`. Exact known-file reads locate the diagnostic seams. This remains the existing .mjs coverage issue; no parser root cause or daemon/config repair is inferred.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O19 — Fathom service index resolves to a temporary clone instead of the authorized paused checkout

- **Symptom:** Fathom service index resolves to a temporary clone instead of the authorized paused checkout
- **Evidence:**
  - **Tool:** ingest
  - **Expected:** Source identity for the active sextant checkout before a scoped reader repair
  - **Actual:** ingest service fathom returned /private/tmp/claude-501/.../scratchpad/pushclone; approved checkout is /Volumes/m2ext/code/geoffmilleraz/sextant at e1841718
  - **Repro:** ingest({service:fathom}) on 2026-09-15; compare root with retained actual-handler run-report consumer path
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O20 — NewThemeGuide uncommitted source absent until explicit consumer ingest

- **Symptom:** NewThemeGuide uncommitted source absent until explicit consumer ingest
- **Evidence:**
  - **Tool:** find_symbol
  - **Expected:** Exact Go function or stale-index indication
  - **Actual:** Initial lookup returned no match and unrelated CUE suggestions; ingest then returned exact cmd/serve/themeguide.go:30
  - **Repro:** find_symbol service resume_as_code name NewThemeGuide before and after ingest on 2026-09-15; existing working-tree implementation, unchanged HEAD
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O21 — Symbol lookup fails while internal Data volume is exhausted

- **Symptom:** Symbol lookup fails while internal Data volume is exhausted
- **Evidence:**
  - **Tool:** find_symbol
  - **Expected:** Resolve existing AdaptMenu after successful app-kit ingest
  - **Actual:** MCP -32603 QueryObservations query: unable to open database file (14)
  - **Repro:** 2026-09-15: ingest app-kit reports ok; find_symbol service app-kit names AdaptMenu and menuArtifact both fail; df reports internal Data101MiB while external workspace has146GiB. No global repair attempted.
  - **Recovery:** Later 2026-09-15, internal free space was already about15GiB before an owner-authorized verified cache relocation. Fresh ingest/find_symbol resolves AdaptMenu and VerifyMenuRepairReplay exactly; thread notes write successfully. Cache receipt: /Volumes/m2ext/appkit-cache-relocation.1AhLVW/RELOCATION.md.
- **Root cause:** Unproven; database failure coincided with internal disk exhaustion. No claim that the cache relocation alone explains recovery.
- **Fix:** Operational recovery observed; no Fathom code, global configuration or hook change was made.
- **Status:** resolved (observed lookup/write recovery, not a Fathom code fix)
- **Impact:** Source navigation and continuity recording are usable again; unrelated wiring warnings remain separate.

### O22 — find_symbol package-like query floods context without result budget

- **Symptom:** find_symbol package-like query floods context without result budget
- **Evidence:**
  - **Tool:** find_symbol
  - **Expected:** bounded exact-name matches or a clarification to use repo_map/outline
  - **Actual:** name gallery returned roughly 53k tokens of package-contained symbols with matched_via case_insensitive; exact RenderControl returns one symbol
  - **Repro:** MCP find_symbol service=app-kit name=gallery versus name=RenderControl on 2026-09-16 working tree
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O23 — Concern-scoped query for intent documentation checks returns unrelated component fields

- **Symptom:** Concern-scoped query for intent documentation checks returns unrelated component fields
- **Evidence:**
  - **Tool:** query_symbols
  - **Expected:** Relevant intent/link/budget checker symbols or an explicit no-match result
  - **Actual:** Top four results were DrawerArtifact.defaults.MenuLinks, MenuArtifact.defaults.Links, NavbarArtifact.defaults.Links and TestAnalyticsReferencePayloadBudgetAndNoBrowserRuntime; none identified intent documentation checks. Fell back to bounded file listing.
  - **Repro:** query_symbols({service:app-kit,concern:intent,keywords:[links,budget,doctor],top:4}) on 2026-09-16 after successful ingest; result also reported stale sources. No known missing checker is claimed.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O24 — Root obligations return stale rules with unrelated current source snippets immediately after clean ingest

- **Symptom:** Root obligations return stale rules with unrelated current source snippets immediately after clean ingest
- **Evidence:**
  - **Tool:** obligations
  - **Expected:** Current source-grounded rules, or explicit withholding of stale citations
  - **Actual:** Five rules marked stale; generated-output rule cites commercial source guidance and checksum rule cites autonomy prose
  - **Repro:** mcp ingest service app-kit returned ok true then obligations service app-kit without file returned compiled_at_watermark 6bf5e4b8005f66caa7a31a120fc0e45574265a3a and stale CLAUDE.md line snippets
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O25 — Impact commands select a stale scratch-clone log and disagree about this repository observation intake

- **Symptom:** Impact commands select a stale scratch-clone log and disagree about this repository observation intake
- **Evidence:**
  - **Tool:** flywheel / instrument --adoption
  - **Expected:** Identify the app-kit service and current call stream; distinguish stale or mismatched measurement inputs and read the same observation file
  - **Actual:** flywheel reports call-log OPEN with last traffic about 45 hours old and observations IDLE; instrument reads a sextant scratch pushclone log, reports find_symbol/repo_map zero despite current successful calls, and reads 20 open app-kit observations
  - **Repro:** From /Volumes/m2ext/code/geoffmilleraz/app-kit on 2026-09-16: fathom flywheel; fathom instrument --adoption --json; compare call_log_path, timestamps and observations_path with current MCP find_symbol service app-kit calls and .fathom/observations.md. Cause of routing mismatch not established; no reroot or global repair performed.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O26 — Repeated stale obligation reference after successful app-kit ingest during native bundle verification

- **Symptom:** Repeated stale obligation reference after successful app-kit ingest during native bundle verification
- **Evidence:**
  - **Tool:** obligations
  - **Expected:** Current file-scoped guidance, or actionable stale-rule refresh instructions
  - **Actual:** With file=docs/intent/scope.md, status=stale; rule text about documentation claims points to CLAUDE.md line87 whose snippet is about index-before-grep. No runtime authority inferred.
  - **Repro:** MCP ingest service app-kit returned ok canonical root; obligations service app-kit file docs/intent/scope.md returned compiled_at_watermark 6bf5e4b8005f66caa7a31a120fc0e45574265a3a with stale rule. Initial caller used incorrect path key; repeated with correct file key and reproduced.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O27 — App-kit ingest call did not return before caller cancellation during native workflow verification

- **Symptom:** App-kit ingest call did not return before caller cancellation during native workflow verification
- **Evidence:**
  - **Tool:** ingest
  - **Expected:** Refresh acknowledgement or an explicit bounded error while preparing local evidence documentation
  - **Actual:** Initial functions cell yielded after 31 seconds; subsequent waits produced no MCP result. Caller terminated the orchestration wait, not the daemon. Refresh completion and cause are unknown; native test is independent.
  - **Repro:** mcp__fathom__ingest({"service":"app-kit"}) on 2026-09-16 after obligations reported index_stale; no commit or global configuration mutation. Retain as operational interruption, not an index correctness finding.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O28 — pack_context silently returns an empty pack for a sentence supplied instead of a concern ID

- **Symptom:** pack_context silently returns an empty pack for a sentence supplied instead of a concern ID
- **Evidence:**
  - **Tool:** pack_context
  - **Expected:** A recognized concern ID or explicit unknown-concern diagnostic; do not treat this as free-text search
  - **Actual:** Caller supplied a natural-language G4 sentence; result budget4000 dropped0 included[] tokens0. Tool schema says concern ID. This is observed caller misuse and a possible diagnostic improvement, not evidence of missing indexed source.
  - **Repro:** mcp__fathom__pack_context({"service":"app-kit","concern":"G4 adoption improvement propagation second consumer reviewed upgrade reusable feedback release candidate JIT"}); observed by bounded propagation audit 2026-09-16. No daemon, index or source repair attempted.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O29 — Root obligations returned stale rules with unrelated current snippets after successful ingest

- **Symptom:** Root obligations returned stale rules with unrelated current snippets after successful ingest
- **Evidence:**
  - **Tool:** obligations
  - **Expected:** Root conventions with accurate source attribution, or explicit unusable stale evidence
  - **Actual:** Five rules labeled stale; CLAUDE.md lines 44-45 quote commercial source retrieval for generated-output rule; root guidance read directly instead
  - **Repro:** ingest service app-kit then obligations service app-kit max_rules 6; compiled watermark 6bf5e4b8005f66caa7a31a120fc0e45574265a3a on 2026-09-16
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O30 — File obligations fallback returned stale root snippets after ingest

- **Symptom:** File obligations fallback returned stale root snippets after ingest
- **Evidence:**
  - **Tool:** obligations
  - **Expected:** Current relevant source snippets, or explicit refusal to treat stale snippets as current guidance
  - **Actual:** app-kit ingest succeeded; obligations root fallback reported compiled_at_watermark 6bf5e4b8005f66caa7a31a120fc0e45574265a3a with stale doc-claimed rules whose quoted lines now contain unrelated routing prose. File-scoped query returned no rules; current source was read directly.
  - **Repro:** MCP ingest service app-kit, then obligations service app-kit (root) and file docs/assessments/fixtures/analytics-usability/verify-pair.mjs during 2026-09-16 CLI migration handoff; this records stale-source usability friction, not evidence that edits were governed by those snippets.
  - **Recurrence:** After the CLI milestone, a fresh successful ingest and root obligations query returned the same stale watermark/snippets during static-starter selection. Same issue as O16/O30, not a new defect; current CLAUDE.md was read directly and no stale source claim accepted.
  - **Recurrence (2026-09-17 theme controls):** After successful ingest, obligations for internal/qualityplan/inventory.cue and docs/intent/scope.md again returned watermark6bf5e4b and stale citations: the tool-directive rule quoted commercial-source prose and the documentation rule quoted index-routing prose. Raw root guidance was also available. Current CLAUDE.md/intent governed the change; no stale rule was accepted and no global reconciliation was attempted.
  - **Recurrence (2026-09-17 JIT rehearsal):** The file-scoped query for internal/qualityplan/quality_test.go returned current raw root sections alongside the same stale tool-directive rule at CLAUDE.md#L46-L47. It explicitly reported 0/5 compiled sections; current source governed review. This is the existing stale-obligation issue, not another defect or proof of hook enforcement.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O31 — Continuity brief returned old handoffs during intermittent workspace EPERM

- **Symptom:** Continuity brief returned old handoffs during intermittent workspace EPERM
- **Evidence:**
  - **Tool:** thread brief
  - **Expected:** Latest app-kit handoff th-873f3f2b, or an explicit inability to resolve the current continuity store
  - **Actual:** At continuation start, brief printed 2026-09-14 navigation and comparison handoffs plus 30 more; previous turn returned th-873f3f2b after recording completed Fathom repair and extended G4 audit. Following source reads failed EPERM. No causal or store-identity claim established.
  - **Repro:** Run fathom thread brief from /Volumes/m2ext/code/geoffmilleraz/app-kit while file reads intermittently fail Operation not permitted; compare named latest thread note th-873f3f2b. Root lookup and fallback behavior need investigation.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O32 — find_symbol misses exported async createMemoryMeasurements in a new .mjs probe

- **Symptom:** find_symbol misses exported async createMemoryMeasurements in a new .mjs probe
- **Evidence:**
  - **Tool:** find_symbol
  - **Expected:** Hit apps/styleguide/browser/memory.mjs exported function
  - **Actual:** matched_via none with unrelated Go suggestions; direct source read confirms function. NewThemeLab Go lookup succeeds.
  - **Repro:** Service app-kit after session ingest: find_symbol name=createMemoryMeasurements returns none; source contains export async function createMemoryMeasurements. Parser support or index coverage cause unproven.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O33 — find_symbol misses exported trackOwnedChild in new .mjs helper after ingest

- **Symptom:** find_symbol misses exported trackOwnedChild in new .mjs helper after ingest
- **Evidence:**
  - **Tool:** find_symbol
  - **Expected:** Exact symbol hit at apps/styleguide/browser/owned_process.mjs
  - **Actual:** matched_via none; unrelated Go and Python suggestions; direct read confirms export
  - **Repro:** Service app-kit: ingest succeeded, then find_symbol name=trackOwnedChild returned none. Source apps/styleguide/browser/owned_process.mjs contains export function trackOwnedChild. Same .mjs coverage family as O32; parser cause not established.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O34 — Known O30 stale compiled obligation recurs during bounded core uplift

- **Symptom:** Known O30 stale compiled obligation recurs during bounded core uplift
- **Evidence:**
  - **Tool:** obligations
  - **Expected:** Current governing source only, or clearly quarantined stale claims
  - **Actual:** Fresh ingest still returns watermark 6bf5e4b and a stale CLAUDE.md L46-L47 tool directive alongside current raw sections; current source consulted instead
  - **Repro:** 2026-09-21 service app-kit, main 0fe6292: obligations file appkit/ui/analytics/render.go, include_snippets false, max_rules 4; current directive is in Maintainer-only workflow, not cited lines. Recurrence of existing O30, no new compilation authorized.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O35 — thread brief plan summary disagrees with current app-kit plan verification

- **Symptom:** thread brief plan summary disagrees with current app-kit plan verification
- **Evidence:**
  - **Tool:** thread brief
  - **Expected:** Identify the plan source and current app-kit counts, or mark stale totals unknown
  - **Actual:** At dc4bd2f, brief says 3 plans/45 gated rows/45 checked/0 open; immediately preceding pre-push verified four current plan files,56 total/40 claimed/40 verified
  - **Repro:** Run fathom thread brief in app-kit at dc4bd2f and compare docs/plans/{cli-kit,experience-platform,motion-provenance,multi-target-kit}.md plus git pre-push output. No cause proven; may be stale ledger or scope mismatch.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O36 — find_symbol misses exported async sampleOwnedProcesses in indexed JavaScript

- **Symptom:** find_symbol misses exported async sampleOwnedProcesses in indexed JavaScript
- **Evidence:**
  - **Tool:** find_symbol
  - **Expected:** Exact definition at apps/styleguide/browser/process_rss.mjs:29
  - **Actual:** matched_via none and unrelated suggestions after successful app-kit ingest; source defines the exported async function
  - **Repro:** find_symbol({"service":"app-kit","name":"sampleOwnedProcesses"}) at HEAD445fe69; inspect apps/styleguide/browser/process_rss.mjs, unchanged in this task. Root cause unconfirmed.
  - **Control:** `parseProcessRows` in the same file also misses; `evaluateMemoryV2Metrics` in the fixture directory resolves exactly. This does not isolate an async-parser defect.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O37 — Reference lookup parameter mismatch caused accidental fleet expansion during a bounded app-kit lookup

- **Symptom:** Reference lookup parameter mismatch caused accidental fleet expansion during a bounded app-kit lookup
- **Evidence:**
  - **Tool:** references
  - **Expected:** A service-scoped lookup or explicit refusal of an unsupported service argument
  - **Actual:** Initial name argument refused because query is required; then query plus service=app-kit returned occurrences from app-kit, Conjure and platform-engineering plus a missing demo root. This is caller/tool-contract friction, not a claim that valid services scoping is broken.
  - **Repro:** MCP references({service:"app-kit",name:"RenderField"}) then references({service:"app-kit",query:"RenderField"}) at app-kit34a0cc5. Compare find_symbol({service:"app-kit",name:"RenderField"}) style; references response recommends narrowing services instead. Known-good references services argument not probed in this task.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O38 — query_symbols recommends re-ingest immediately after a successful hash-gated ingest

- **Symptom:** query_symbols recommends re-ingest immediately after a successful hash-gated ingest
- **Evidence:**
  - **Tool:** query_symbols
  - **Expected:** Freshness scoped to returned code evidence, or an actionable distinction between source index and stale context documents
  - **Actual:** At app-kit 4a3cdd4, ingest returned ok; query_symbols returned relevant static-content Go symbols but also 65 stale sources of82 and reingest_recommended true, citing old skill/handoff documents. Source definitions inspected agree with returned paths. Related freshness friction O14; no claim of incorrect symbols.
  - **Repro:** ingest service app-kit; query_symbols service app-kit concern ui-experience keywords static,content,reading top10 on 2026-09-23; successful outline of apps/gallery/static_content.go provides known-good comparison
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO
