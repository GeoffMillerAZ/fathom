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

## Open

### O1 — Obligations injection silently dropped out on ~36% of edits (measured 18/50 empty), nondeterministically. Root cause: two services ('fathom' and 'sextant') recorded the SAME corpus root in service_meta. resolveFromServices ranges over a Go map, whose iteration order is random, so it resolved to whichever matched first. When it picked 'sextant', candidatesWithDaemon filtered the identical basename candidate out of the fallback list, leaving [sextant] alone — and 'sextant' was a crippled shadow with 5862 observations but 0 typed_edges and 0 semantic_claims, so it served zero rules and an empty callgraph.

- **Symptom:** Obligations injection silently dropped out on ~36% of edits (measured 18/50 empty), nondeterministically. Root cause: two services ('fathom' and 'sextant') recorded the SAME corpus root in service_meta. resolveFromServices ranges over a Go map, whose iteration order is random, so it resolved to whichever matched first. When it picked 'sextant', candidatesWithDaemon filtered the identical basename candidate out of the fallback list, leaving [sextant] alone — and 'sextant' was a crippled shadow with 5862 observations but 0 typed_edges and 0 semantic_claims, so it served zero rules and an empty callgraph.
- **Evidence:**
  - **Tool:** edit-hook / serviceident.resolveFromServices
  - **Expected:** Deterministic service resolution: one corpus root resolves to exactly one service, or resolution is stably ordered when several match.
  - **Actual:** Coin-flip resolution. 18/50 edit-hook runs on internal/mcp/server.go injected nothing; the other 32 injected 79 eligible rules. research report --since 168h corroborates over a week of real use: obligations_served=2010 of answered=4023 (50%).
  - **Repro:** for i in $(seq 1 50); do echo '{"tool_name":"Edit","tool_input":{"file_path":"<repo>/internal/mcp/server.go"},"cwd":"<repo>"}' | bin/fathom edit-hook; done | grep -c obligations   # before dedup: ~32/50; after: 50/50
- **Root cause:** Two distinct failures compounding.
  1. *Data:* the 2026-07-07 sextant→fathom rename (ADR-015) left the directory
     named `sextant`. Something later ingested the repo under its basename, so
     `service_meta` held two rows — `fathom` and `sextant` — with byte-identical
     corpus roots. The shadow carried only what a bare `ingest` produces
     (observations 5862, fingerprints 697, commits 268) and none of what the
     enrichment and compile passes produce (typed_edges 0 vs 11218,
     semantic_claims 0 vs 628, claim_links 0 vs 1293, compile_runs 0 vs 14).
  2. *Code:* `resolveFromServices` (cmd/fathom/serviceident.go:66-85) returns on
     the first match while ranging over a `map[string]svcInfo`. Go randomizes map
     iteration order, so with two rows matching the same resolved path the winner
     is a coin flip. `candidatesWithDaemon` (:91-104) then *removes* the basename
     candidate when it equals the resolved name — so the `sextant` branch produces
     the single-element list `[sextant]` with no fallback to `fathom`, converting a
     tie into a total loss rather than a degraded-but-working answer.
  Note the irony: `resolveIndexedCandidate`'s own doc comment cites dogfood O15,
  where this same rename killed injection for six days. That fix made resolution
  path-based instead of name-based, which is right — but it assumed a path maps to
  at most one service, and nothing enforces that invariant.
- **Fix:** Data side (applied 2026-07-29): deleted the shadow's rows from
  observations/fingerprints/commits/service_meta and VACUUMed — provably loss-free,
  since every table where `sextant` had rows `fathom` had an equal count, and every
  table where `fathom` was rich `sextant` was empty. Store 29M→26M; edit-hook went
  18/50 empty → 0/50 empty. Code side (NOT yet done, the durable fix): make
  resolution deterministic and total —
  (a) sort the candidate service names before matching so a tie resolves stably;
  (b) prefer the richer service on a tie (the one with typed_edges/semantic_claims)
      rather than an arbitrary one;
  (c) never let the dedup in `candidatesWithDaemon` shrink the candidate list to a
      single entry — a resolved name should be *prepended*, never allowed to
      *displace* the fallback;
  (d) reject an ingest that would record a corpus root already claimed by a
      different service, the same way the ingest guard already rejects path-like
      service IDs — that is what would have prevented this outright.
- **Status:** closed 2026-08-07 — all fixes (a)-(d) landed (plan IO-1 + IO-2/IO-3): deterministic resolution, never-collapsing dedup, daemon-side one-root-one-service refusal, and remove_service/rename_service for the surviving twins
- **Impact:** Highest-severity class fathom has: silent, nondeterministic loss of
  its single most valuable agentic feature. ~36% of edits in this repo received no
  governing rules, and the affected edits also saw an empty typed-edge callgraph
  (`find_callers`/`symbol_context` degraded), with no error, no warning, and no log
  line — indistinguishable from "this file has no rules". A week of real usage read
  `obligations_served=2010 / answered=4023` (50%). Because it presents as flakiness
  rather than breakage, it survived a `fathom doctor` that reported 10/11 PASS:
  doctor probed CLAUDE.md, won the coin flip, and reported the feature healthy.
  Corollary finding for doctor: a single probe cannot detect a nondeterministic
  fault — `obligations_served` should probe N times and assert consistency.

### O2 — SessionStart hook emitted a false "this repo is not an indexed service" for a repo that WAS indexed, instructing the agent to abandon index-first discovery for the whole session. Root cause: fetchServices collapses every failure mode (transport error, non-2xx, unmarshal failure, shared 2s timeout) into an empty map, which buildSessionContext cannot distinguish from "daemon healthy, zero services".

- **Symptom:** A session opening in `platform-engineering` received:
  *"ℹ️ fathom code-intelligence is live, but this repo ("platform-engineering")
  is not an indexed service — its symbols aren't in the index, so Grep/Read
  discovery is expected here. Indexed services: ."*
  The service was indexed the whole time (256 symbols, correct corpus root,
  `reingest_recommended:false`). Re-running the identical command minutes
  later returned the correct ✅ branch.
- **Evidence:**
  - **Tool:** session-hook / `cmd/fathom/sessionhook.go:buildSessionContext`,`fetchServices`
  - **Expected:** ✅ branch naming the service, or — if the service list
    can't be read — an honest *degraded* signal.
  - **Actual:** the not-indexed branch, asserting a negative it had no
    evidence for. `Indexed services: .` is `strings.Join(nil, ", ")` plus
    the format string's trailing period (`sessionhook.go:118`) — i.e. the
    known-list was EMPTY, not a service literally named ".".
  - **Repro:** force `fetchServices` to fail (point `--url` at a live
    `/healthz` whose `tools/call` errors or exceeds the 2s client timeout):
    `bin/fathom session-hook --dir <indexed-repo>` → not-indexed branch.
- **Root cause:** `fetchServices` (`sessionhook.go:123-167`) is best-effort
  by construction: `originHeader` error, `hc.Do` error, `io.ReadAll` error,
  and `json.Unmarshal` error each `return out` with an empty map, and the
  single `&http.Client{Timeout: 2 * time.Second}` (`:77`) is shared with the
  `/healthz` probe, so a healthy-but-loaded daemon can pass health and then
  time out enumeration. `buildSessionContext` treats the empty map as
  ground truth: the candidate loop (`:99-110`) matches nothing and control
  falls through to the not-indexed return (`:118`). Nothing distinguishes
  "we failed to ask" from "the answer is no".
  The irony mirrors O1: the hook was built *because* telemetry showed
  fathom is under-used and "index first" in passive docs doesn't work. Its
  own degraded path emits an active, confident instruction to grep instead
  — the exact behaviour it exists to prevent — and does so for the entire
  session, with no error, no warning, and no retry.
- **Fix:** (NOT yet done)
  (a) `fetchServices` returns `(map[string]svcInfo, error)` so callers can
      tell failure from emptiness;
  (b) `buildSessionContext` gains a third branch for enumeration failure:
      "daemon is up but the service list could not be read — call
      `list_services` before assuming this repo is unindexed", never the
      discouraging assertion;
  (c) give the `list_services` call its own timeout separate from
      `/healthz`, plus one retry — the same call succeeded seconds later,
      so a single retry would have prevented this outright;
  (d) guard `len(known) == 0` at `:118` so an empty list can never be
      formatted into a sentence as `Indexed services: .`;
  (e) `sessionhook_test.go` currently asserts only that the string
      "not an indexed service" appears — add a case asserting it does NOT
      appear when enumeration fails for a repo that is in fact indexed.
- **Status:** open
- **Impact:** Same severity class as O1 — silent, transient loss of
  fathom's agentic value — but broader blast radius per occurrence: O1 lost
  rules on individual edits, O2 mis-primes an entire session's discovery
  strategy from its first token. It is also self-concealing: an agent told
  "Grep/Read discovery is expected here" has no reason to call
  `list_services` and disprove it, so the session never notices, and the
  telemetry that motivated this hook (fathom under-use vs grep) is
  partially *caused* by the hook's own failure path. Cheap to hit: any
  daemon slow enough to exceed 2s on one call.

### O3 — CUE symbols are indexed at `verified` tier but participate in NO call graph (cueextract emits zero edges), so `test_reaches` and `reachable_from` are unsatisfiable by construction for every CUE definition. In a CUE-first consumer repo the verify gate silently degrades to `symbol_present` — "the definition exists" — and every plan row must take `skip_test: true`.

- **Symptom:** Authoring the first symbol-contract plan in `platform-engineering`
  (a repo whose ADR-0002 makes CUE the language of intent), 12 of 36 rows claim
  CUE definitions. Every one of them had to carry `skip_test: true` with a prose
  `skip:` justification, because no contract naming a CUE symbol can ever satisfy
  `test_reaches`/`reachable_from`.
- **Evidence:**
  - **Tool:** verify / `internal/extractor/cueextract.go`
  - **Expected:** a CUE definition can be reached by *something* the index knows
    about — at minimum by the definitions that unify it.
  - **Actual:** `grep -n "Edge\|edges\|Calls" internal/extractor/cueextract.go`
    returns **nothing** across all 157 lines. Symbols are emitted
    (`CUEDefinition`, `CUEField`, `verified` tier, correct file:line —
    `query_symbols --service planning --concern pdk` returns `#Block` and its
    twelve fields); edges are not emitted at all.
  - **Consequence:** `find_callers` / `references` on a CUE definition return
    nothing structural, and `verify` can only ever assert existence.
- **Root cause:** The CUE extractor was built as a symbol extractor only. Unlike
  the Go/JS extractors it has no edge-emitting pass, so CUE symbols enter the
  store as isolated nodes. Nothing is wrong at the verify layer — `test_reaches`
  correctly reports "no path" — but the contract vocabulary offers a CUE row no
  way to prove anything beyond presence, and `skip_test` is the only exit.
- **Fix:** (NOT yet done) CUE has a natural edge relation the extractor discards:
  (a) **Emit reference edges.** `#Kit: #Block & {...}` is an edge `#Kit → #Block`.
      So are field type references (`profile: #Profile`), list element types
      (`uses: [...#Block]`), and cross-package imports. This is the highest-value
      piece on its own: it restores `find_callers`/`references` for CUE and makes
      "what composes this contract?" answerable — for a PDK-style ladder that is
      the central question.
  (b) **Treat `testdata/*.cue` unification as the CUE analogue of test-reaches.**
      A fixture that unifies `#Effects` and is expected to fail `cue vet` *is* a
      test reaching a symbol. Mapping it onto the existing edge kind makes
      `reachable_from` satisfiable for CUE with no new plan grammar.
  (c) **Add a CUE evidence tier** (`vet_clean` / `export_golden`) so a row can
      prove behaviour, not just presence. Today that proof is prose inside
      `skip:`, which no machine reads.
- **Status:** open
- **Impact:** Scoped but sharp, and it will grow. It does not produce false
  passes — the honesty boundary holds, and fathom makes the author write an
  explicit escape rather than silently passing (this is the design working).
  The cost is erosion: a gate escaped 12 times in one plan trains its authors to
  reach for `skip_test` reflexively, and the escapes are indistinguishable at
  review time from the legitimate non-code ones. It also removes fathom's
  discovery value for CUE-heavy corpora exactly where the fleet is heading —
  `platform-engineering` intends CUE for contracts, schemas, profiles, policy,
  pipeline definitions and flavor tables, so the majority of its authored intent
  will be invisible to the callgraph tools. Related to O1's theme: the failure is
  quiet and reads as "this symbol has no callers" rather than "this language has
  no edges".

### O4 — A `{`/`}` inside a double-quoted annotation value silently destroys the whole annotation block: the row loses its author-owned `id` (falls back to positional `md-NNN`, which the Stop hook matches on), loses every other key, and leaks the raw annotation into its title — with no warning, no lint, and no exit-code change.

- **Symptom:** A plan row whose `skip:` justification mentions the contract
  notation `{symbol, package}` parses as `md-009` instead of its declared
  `P0-9`, renders its title as
  `Phase-1 plan drafted with symbol contracts (first code phase) {id: P0-9, skip_test: true, skip: "docs/plans/phase-1-pdk-day0.md: 36 rows,`
  and carries none of its keys. Quoted **commas** are handled correctly; quoted
  **braces** are not.
- **Evidence:**
  - **Tool:** verify / `internal/verify/markdown.go` (`ParseMarkdownPlan`)
  - **Repro** (two lines, control + test):
    ```
    - [x] control row no braces {id: R-1, skip_test: true, skip: "alpha, beta, gamma"}
    - [x] test row with braces in quoted value {id: R-2, skip_test: true, skip: "alpha, {symbol, package} beta"}
    ```
    `fathom verify --service <any> --plan braces.md` →
    ```
    ? [R-1] control row no braces
    ? [md-002] test row with braces in quoted value {id: R-2, skip_test: true, skip: "alpha,
    ```
  - **Expected:** `[R-2]` with a clean title and its keys, exactly as `[R-1]`.
  - **Actual:** positional id, polluted title, all keys dropped. Exit code
    unchanged (0); `LintPlan` emits no warning.
- **Root cause:** the annotation-block scanner locates the `{…}` block without
  respecting quoting, so the `}` of an inner `{symbol, package}` terminates the
  block early (the same class of bug the documented comma rule already fixed for
  values — quoting is honored for commas but not for braces). Everything after
  the premature terminator is treated as title text.
- **Fix:** (NOT yet done)
  (a) make the block scanner quote-aware for `{`/`}` exactly as it already is for
      `,` — one shared tokenizer for the annotation block rather than two
      independent splitting rules;
  (b) **fail loudly on a malformed block.** A row that contains a `{…}` block but
      cannot parse it should be a `LintPlan` warning at minimum — silently
      degrading to `no-contract` is the one outcome that hides the error, because
      an Unknown row looks identical to an intentionally-uncontracted one;
  (c) add the two-line repro above as a parser test case.
- **Status:** open
- **Impact:** Low-frequency trigger, high consequence, and self-concealing —
  the combination this log keeps finding (cf. O1, O2). Three compounding effects:
  1. **The id is lost.** The README states author-owned `id` should be used "in
     every gated plan that will be diffed across edits — the Stop hook matches
     tasks by ID." A silently-positional row is matched by `md-NNN`, and
     positional ids renumber whenever a line is inserted above them, so the Stop
     hook starts matching the wrong task.
  2. **The contract is lost.** `symbol`, `package`, `skip_test` all vanish, so a
     row intended to carry a real symbol contract degrades to Unknown and proves
     nothing — while still counting as claimed.
  3. **The trigger is idiomatic.** fathom's own docs describe the work frontier
     as carrying "their `{id, symbol, package}` anchors"; an author explaining a
     `skip:`/`reason:` in those terms writes braces naturally. The feature's own
     vocabulary is the landmine.

### O5 — `doctor`'s binary_fresh check compares the fathom binary's build stamp against the CONSUMER repo's git HEAD, so it reports an unclearable stale-binary failure in every repo that is not sextant. hooks_firing looks for the call log under the consumer repo and fails the same way.

- **Symptom:** `fathom doctor --service platform-engineering` (run from the
  platform-engineering working tree) reports:
  `binary_fresh FAIL — binary @ 8af274bfa435 does not match HEAD 7e0835e921e0`.
  `7e0835e921e0` is platform-engineering's HEAD commit, not sextant's. No
  rebuild of fathom can ever satisfy this, because the check is asking the
  binary's build stamp to equal a different repository's HEAD.
- **Evidence:**
  - **Tool:** doctor / binary_fresh (and, separately, hooks_firing)
  - **Repro:** in any consumer repo with commits,
    `fathom doctor --service <that-service>` → binary_fresh FAIL naming that
    repo's HEAD. Confirmed by `git -C <consumer> rev-parse HEAD` matching the
    "HEAD" the check printed.
  - **Remedy printed:** `cp bin/fathom ~/.local/libexec/fathom/fathom &&
    launchctl kickstart …` — correct advice for a genuinely stale daemon,
    inert for this failure.
  - **hooks_firing:** FAIL — "call log
    `<consumer-repo>/workspace/logs/mcp-calls.jsonl` not found — no hook has
    ever written to it". The daemon is started with
    `--call-log <sextant>/workspace/logs/mcp-calls.jsonl`, so the log exists,
    just not where the check looks. Hooks demonstrably fire (the edit-hook
    injected on every edit in that session, and `surface_traffic` PASSes in the
    same run — the two checks disagree about the same fact).
- **Root cause:** both checks resolve a path/ref relative to the doctored
  repo when the fact they are asserting belongs to the fathom installation.
  `surface_traffic` gets this right by asking the daemon; `hooks_firing`
  asks the filesystem of the wrong tree.
- **Fix:** (NOT yet done)
  (a) binary_fresh: compare the build stamp against the fathom SOURCE repo
      (the stamp already identifies it), or skip with an explicit "n/a —
      doctoring a consumer repo" when `--service` is not fathom's own;
  (b) hooks_firing: resolve the call-log path from the daemon's actual
      configuration rather than assuming `<repo>/workspace/logs/`, or defer to
      `surface_traffic`, which already answers the question correctly;
  (c) whichever way, a check the operator cannot clear must not be a FAIL.
- **Status:** open
- **Impact:** Reputational, and therefore worse than it looks. `doctor` is the
  most valuable thing in the tool for a new consumer — in this session it
  correctly caught four real, fixable adoption gaps (obligations never
  compiled, no lefthook verify gate, no CLAUDE.md stanza, zero skills
  installed) that had left roughly two thirds of fathom unused. Two permanent
  unclearable FAILs alongside them train the reader to skim the summary line
  ("6 fail") and stop reading, which is exactly the audience — a first-time
  consumer — whose adoption the tool most needs to win.

### O6 — `instrument --adoption` reports "zero-call registered tools (62 of 85 registered)" against surfaces that do not expose them. The agent surface offers 16 tools; the report counts all 85, overstating under-adoption ~4x and pointing the operator at the wrong remedy. Its verify-gate figure (251,834 catches across 920 calls) also reads as a double-count.

- **Symptom:** `fathom instrument --adoption --call-log <log> --root <consumer>`
  prints a 62-name zero-call list including `arch_conformance`,
  `contradiction_scan`, `dead_code`, `call_path`, `rename_impact`,
  `stale_docs`, `traceability`. Read plainly this says the agent ignored three
  quarters of fathom. But `capabilities` on the same daemon returns
  `active_surface: "agent", tool_count: 16` — the great majority of those 62
  are not reachable from an agent session, so a zero there is structural, not
  behavioural.
- **Evidence:**
  - **Tool:** instrument --adoption
  - **Expected:** a zero-call list scoped to what the measured surface exposes,
    or explicitly partitioned.
  - **Actual:** one undifferentiated list against the full 85-tool registry.
  - **Consequence:** "use more tools" and "expose more tools on this surface"
    are opposite remedies, and the report cannot distinguish them. In this
    session the genuinely actionable finding — of the 16 agent-surface tools,
    about half were ever called, and the unused half is exactly the navigation
    half (`find_symbol`, `find_callers`, `symbol_context`, `outline`,
    `entrypoints`, `read_source`, `repo_map`, `pack_context`) — had to be
    derived by hand from `capabilities`, because the report does not print it.
- **Second, separable issue in the same report:**
  `verify-gate catch count: 251834 catch(es) across 920 instrumented call(s)`.
  273 catches per instrumented call is not a credible per-run figure; it reads
  as summing every claimed-but-failing task across every plan on every call
  rather than counting distinct catches. A single implausible number costs the
  whole report its credibility with the first reader who checks one.
- **Root cause:** the adoption report is built from the tool REGISTRY
  (`registeredToolNames`, which the flag docs note is backend-independent)
  rather than from the surface/profile the call log belongs to. Surface is
  already a first-class concept — the same report's "surface traffic" section
  breaks calls down by `agent`/`cli`/`companion`/bare — so the data to scope it
  is present and unused.
- **Fix:** (NOT yet done)
  (a) scope the zero-call list to the tools the measured surface exposes, or
      partition into "unused on this surface (n of 16)" and "registered but not
      exposed here (n)";
  (b) print the surface-scoped adoption ratio as the headline number, since
      that is the one an operator can act on;
  (c) re-derive the verify-gate catch count as distinct catches and sanity-cap
      the per-call ratio, or label precisely what is being summed.
- **Status:** open
- **Impact:** This is the instrument the flywheel is steered by — it is what
  answers "is fathom helping?", which is the question that decides whether a
  team keeps investing in it. Overstating under-adoption makes the tool look
  ignored when it is partly just unexposed, and an implausible headline
  statistic invites the reader to discount the credible findings printed beside
  it. In this repo the real answer was nuanced and useful (verify_plan is
  load-bearing and has caught three real defects; navigation tools are unused;
  obligations are dark because they were never compiled) — and none of those
  three conclusions came from the report's own summary.

### O7 — `fathom init` scaffolds a `.fathom/modules.cue` that `fathom doctor` immediately rejects: the fully-commented file has no `bindings` field, which the binding schema marks required. Its sibling `guard.cue`, written by the same command and equally commented, passes.

- **Symptom:** on a fresh repo, `fathom init` → `fathom doctor` reports
  `binding_file FAIL — .fathom/modules.cue is invalid: bindspec: field
  "bindings": required, MUST be present`, plus two SKIPs that cascade from it
  (`binding_modules_mounted`, `binding_module_readiness`). The operator has
  taken no action other than running the two commands the adoption guide
  prescribes, in order.
- **Evidence:**
  - **Repro:** `fathom init --path <fresh repo>` then
    `fathom doctor --service <svc>`.
  - The scaffold is entirely comments — every line of the example binding is
    prefixed `//` — so the file parses to an empty struct, and the schema
    requires `bindings`.
  - **The inconsistency is the tell:** `.fathom/guard.cue` is written by the
    same `init`, is equally all-comments, and reports
    `guard_file PASS — .fathom/guard.cue valid`. Two sibling scaffolds, one
    command, opposite outcomes.
  - **Fix that works:** appending `bindings: []` clears it.
- **Root cause:** the binding schema makes `bindings` required where the guard
  schema tolerates its list being absent; the scaffold writer emits a
  comments-only file for both without emitting the empty list the stricter
  schema needs.
- **Fix:** (NOT yet done) either (a) have `init` emit `bindings: []` beneath
  the commented example (one line, matches what an operator must add anyway),
  or (b) relax the schema so an absent `bindings` means "none bound", which is
  what guard already does and what the file's own comment already says ("No org
  rule ever reaches this repo until a binding below is uncommented").
- **Status:** open
- **Impact:** First-run experience, on the exact path the adoption guide walks a
  new consumer down. It is cosmetic in effect — nothing is broken, no rule is
  missed — but it costs `doctor` credibility at the worst possible moment,
  because it lands in the same summary line as the findings that DO matter. In
  this session `doctor` legitimately caught four real adoption gaps; a new
  consumer counting failures cannot tell those apart from a scaffold that was
  born failing.

### O8 — `compile --dry-run` estimates ONLY input tokens and prints `cost_usd: 0.000000`, but output is ~87% of tokens and ~97% of cost. The dry run under-reported this compile by 25x on tokens and reported its cost as zero. A flag documented as "estimate cost without calling the LLM" cannot estimate cost.

- **Symptom:** for `--service platform-engineering --layer obligations`:

  | | dry run | actual |
  |---|---|---|
  | tokens_in | 5,547 | 18,531 |
  | tokens_out | **0** | **124,722** |
  | cost_usd | **0.000000** | **0.642141** |
  | duration | 1 ms | 1,265,127 ms (21 min) |

  The dry run's own flag help reads "estimate cost without calling the LLM
  (default true; use --dry-run=false to spend)". It estimated 4% of the tokens
  and 0% of the cost.
- **Evidence:**
  - **Tool:** `fathom compile --dry-run` / `--dry-run=false`
  - Same service, same layer, same store, minutes apart; only the flag differed.
  - Input alone was also under-estimated 3.3x (5,547 → 18,531), so the gap is
    not solely the missing output term.
  - Model was `claude-haiku-4-5`; 124,722 output tokens at Haiku output pricing
    accounts for essentially all of the $0.64, which is why omitting the output
    term zeroes the estimate.
- **Root cause:** the dry-run path measures the assembled prompt and stops. It
  has no output-token model, so `cost_usd` is computed from a term that is
  structurally zero — and prints as a confident `0.000000` rather than as
  "unknown".
- **Fix:** (NOT yet done)
  (a) estimate output as a ratio of input per layer (obligations emitted ~6.7x
      its input here) and print a RANGE, not a point;
  (b) failing that, print `cost_usd: unknown (output not estimated)` — an
      honest unknown is far better than a precise zero, which reads as free;
  (c) include the input-side under-estimate in the same fix — whatever the dry
      run measures is not what the real run sends;
  (d) surface expected duration. 21 minutes for a 10-section layer is the kind
      of thing an operator wants to know before starting, not after.
- **Status:** open
- **Impact:** This is the number a user decides on. In this session the owner
  asked, in as many words, "is it worth using and how much will it cost me?" —
  I quoted the dry run (5,547 tokens, effectively free) and was wrong by 25x on
  tokens, by $0.64 on the printed cost, and by 21 minutes on time. On a
  subscription lane the dollar figure may be notional, but the token and time
  costs are real and the operator was given neither. A cost estimator that
  reports zero for a non-zero job trains users to stop reading it, and the flag
  defaults to `true` precisely so it is the first thing anyone runs.
- **Adjacent, worth checking while in there:** 19 of 42 candidate claims were
  rejected (45%). If that is normal it should be explained in the output; if it
  is not, the emitted 23 cost 42 claims' worth of generation.

### O9 — lefthook pre-push review hook hangs indefinitely in non-interactive shells — pushes from agents/scripts never complete

- **Symptom:** lefthook pre-push review hook hangs indefinitely in non-interactive shells — pushes from agents/scripts never complete
- **Evidence:**
  - **Tool:** review
  - **Expected:** pre-push review completes and the push proceeds (or blocks with a named verdict) in a non-interactive shell
  - **Actual:** review prints 4/4 dimensions checked, 8 active findings all info/warn none blocking, then stalls at a review-prompt line; three scripted pushes hung until killed; git count-objects rules out transfer size
  - **Repro:** non-interactive git push origin HEAD from feat/mcp-scoped-surfaces, 2026-08-07; workaround once: LEFTHOOK=0 push after review had already reported non-blocking; fix: review must detect non-TTY and never prompt
- **Root cause:** CORRECTED 2026-08-07 — nothing prompts and nothing hangs: `review` took 22.6s on a six-file diff, the verify loop adds more, and lefthook buffers each step's output until completion, so the push looked stalled and scripted wrappers killed it at their 60–90s caps. The `review >` line was lefthook's section header, not a prompt.
- **Fix:** IO-9 (2026-08-08): `review --budget-seconds N` (requires `--no-persist`) races the
  review body against a deadline and exits with a NAMED verdict
  (`review: budget exhausted after Ns — advisory, not blocking`) instead of
  silently running long; the lefthook review step passes `--budget-seconds 20`,
  so a pre-push now completes inside scripted wrappers' caps. Pinned by
  TestReviewNonInteractive.
- **Status:** closed 2026-08-08
- **Impact:** scripted/agent pushes completed with hooks live from IO-9 onward;
  the two LEFTHOOK=0 pushes that preceded the fix are the measure of the cost.


### O10 — every redeploy rolls the TCC dice: a new binary hash under launchd silently loses the volume grant and the daemon crash-loops

- **Symptom:** `task deploy` build+install+kickstart succeed, healthz assert fails; `launchctl list` shows last-exit -9 with a fresh near-zero-CPU process (KeepAlive crash-loop); the same binary run from an interactive shell serves fine on a spare port.
- **Evidence (2026-08-08):** deployed fathom @ 1030ad4085b3 matched HEAD; 127.0.0.1:7575 connection-refused across two kickstarts; the identical invocation from the user session answered `ok` on :7576 in 15s. Store and corpus live on /Volumes/m2ext (macOS 26 removable-volume TCC class); the PRIOR binary hash served for days under the same plist.
- **Root cause:** the TCC removable-volumes grant is per-code-identity; an ad-hoc-signed Go binary changes cdhash every build, and a launchd job can never present the consent prompt — so first volume access after a redeploy is a silent kernel deny and the store open dies.
- **Fix:** needs a durable, hash-independent grant for `~/.local/libexec/fathom/fathom` (Full Disk Access is the known-good class — Ghostty's access flows through it across updates). Candidate hardening: `task deploy` should end by asserting healthz THROUGH launchd (it does) and, on refusal, print this diagnosis instead of a bare exit 1.
- **Status:** CLOSED 2026-08-08 (second closure, this time structural) — the operator created a self-signed `fathom-dev` codesigning identity, trusted it for Code Signing, and re-toggled FDA once. `task deploy` now signs every install (verified: real signature, not adhoc), and a post-toggle `launchctl kickstart` brought the daemon up clean on the first try. The grant now follows the IDENTITY, not the cdhash. Final confirmation arrives free at the next source-changing deploy; if that one also comes up clean, this observation is done forever. Previously: REOPENED 2026-08-08 (same day) — the aa66a1f redeploy succeeding was luck of timing, not durability: the FDA row for an AD-HOC-signed binary records the cdhash at grant time, so the aceb09d redeploy produced a NEW failure mode — not a crash-loop but a HANG (main thread parked in getcwd → open$NOCANCEL forever: an unpromptable tccd consult from launchd context; `sample` transcript in the 2026-08-08 session). Zero-CPU process, no listener, no deny logged.
- **Durable fix (two steps, then dead forever):** (1) create a self-signed CODE-SIGNING identity named "fathom-dev" (Keychain Access → Certificate Assistant → Create a Certificate → Code Signing, self-signed) — `task deploy` already signs with it when present; (2) after the FIRST signed deploy, toggle fathom's Full Disk Access off/on once so the grant records the identity-based requirement. Every later redeploy keeps the grant.
- **Interim:** the launchd job is booted out; a session-hosted daemon serves 7575 (dies with the session). Re-bootstrap after the grant: `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.geoffmilleraz.fathom-http.plist`.
- **Impact:** every unsigned deploy re-rolls the TCC dice; two deploys and two different failure signatures (deny crash-loop, consult hang) both trace to the same root.

### O11 — CLI runs write a relative `workspace/logs/mcp-calls.jsonl` under whatever cwd they run in

- **Symptom:** a stray `cmd/fathom/workspace/logs/mcp-calls.jsonl` appeared in the repo after `review` ran with cwd=cmd/fathom (budget-race test runs, 2026-08-08); content was review verdict rows.
- **Root cause (suspected):** the call-log default path resolves relative to cwd instead of the repo/service root or an absolute user-scope path.
- **Fix:** TODO — default the CLI call log to an absolute path (service workspace root or ~/.fathom), never cwd-relative.
- **Status:** open
- **Impact:** cosmetic here (deleted), but a cwd-relative log means CLI surface traffic silently fragments across directories and undercounts in the adoption gauge.


### O12 — ingest CLI -store default is cwd-relative: running from any directory silently creates a junk workspace/db/ store there

- **Symptom:** `fathom ingest --service calib-fmt` run from a corpus checkout printed a full success summary while writing to `<cwd>/workspace/db/fathom.sqlite` — a store nothing will ever read — and FATHOM_HTTP was silently ignored (ingest is store-direct, not daemon-routed).
- **Evidence (2026-08-08):** the CT-10 calibration first-run: scratch daemon's store stayed at 0 services while ingest reported 5,470 symbols; the data was in a fresh `workspace/db/` inside the clone.
- **Root cause:** the `-store` flag defaults to the relative path "workspace/db/fathom.sqlite" (same class as O11's cwd-relative call log); nothing warns when the default resolves outside any known service root.
- **Fix:** TODO — resolve the default against the repo/service root (or ~/.fathom), print the absolute store path in the summary line, and say loudly when a fresh store file is being created.
- **Status:** open
- **Impact:** a successful-looking ingest that indexes into the void; cost one calibration re-run here, would cost a new adopter their first hour.


### O13 — CompileLayer ignores the provider registry: `fathom providers` says ok while compile refuses with "no LLM configured"

- **Symptom:** `fathom providers` reports profile personal/claude-cli ok, but `fathom compile --service X --dry-run=false` errors "no LLM configured for CompileLayer (set ANTHROPIC_API_KEY…)" — the two surfaces disagree about whether an LLM exists.
- **Evidence (2026-08-08):** three compiles refused in a row; meanwhile 6,985 obligations calls served staleness warnings instead of compiled rules (the resume_as_code field report's top finding).
- **Root cause:** CompileLayer is wired only to a direct Anthropic key (WithLLM), not to the provider profiles the rest of the CLI resolves.
- **Fix:** bridge CompileLayer to the provider registry so claude-cli profiles compile too; until then the unblock is an ANTHROPIC_API_KEY in the daemon's launchd EnvironmentVariables.
- **Status:** open
- **Impact:** the second-busiest tool in the fleet serves warnings, not rules, on every wired repo.

### O14 — flywheel verify-gate loop reads IDLE ('gate has never run in a recorded session') while lefthook pre-push runs bin/fathom verify on every push in this repo

- **Symptom:** flywheel verify-gate loop reads IDLE ('gate has never run in a recorded session') while lefthook pre-push runs bin/fathom verify on every push in this repo
- **Evidence:**
  - **Tool:** flywheel
  - **Expected:** verify-gate loop CLOSED, or a reason naming why pre-push runs are invisible to it
  - **Actual:** IDLE with remedy suggesting a lefthook step that already exists
  - **Repro:** fathom flywheel in sextant @ HEAD; lefthook.yml pre-push has the verify step; multiple pushes today
- **Root cause:** the loop read only session-scoped instrumentation (stop-hook verify outcomes); git-native pre-push runs never enter that path, so a repo gating on every push still read "never ran".
- **Fix:** SD-1's verify ledger — every `fathom verify` run appends per-plan rows to ~/.fathom/verify-ledger.jsonl, and the loop reads it before declaring idle. Proven by demonstration same day: one gated run flipped the loop to CLOSED.
- **Status:** resolved (2026-08-14)
- **Impact:** TODO

### O15 — realization 'wired' rung fails geoffmiller-cloud for a project settings.json without fathom hooks, while the hooks ARE wired at user scope (init's user-scope dedup put them there on purpose)

- **Symptom:** realization 'wired' rung fails geoffmiller-cloud for a project settings.json without fathom hooks, while the hooks ARE wired at user scope (init's user-scope dedup put them there on purpose)
- **Evidence:**
  - **Tool:** fleet report
  - **Expected:** wired MET with a scope note, or a distinct partial-visibility verdict
  - **Actual:** grade B — not wired, remedy suggests reconcile, which correctly refuses to double-wire
  - **Repro:** fathom fleet report; geoffmiller-cloud hooks_note='settings.json present, no fathom hook references'; ~/.claude/settings.json carries the fathom hooks
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O16 — a wedged reader produces a plausible empty index: ingest wrote 0 symbols / 0 typed edges while printing '188 pkg(s) (0 failed)' and exit 0 — the fourth false-signal shape of the TCC EPERM wedge

- **Symptom:** a wedged reader produces a plausible empty index: ingest wrote 0 symbols / 0 typed edges while printing '188 pkg(s) (0 failed)' and exit 0 — the fourth false-signal shape of the TCC EPERM wedge
- **Evidence:**
  - **Tool:** ingest / extractor read-error handling
  - **Expected:** a file or package the extractor cannot READ is counted in the failed column and named; an empty result from read errors must be loud, never a clean-looking zero
  - **Actual:** five consecutive 'bin/fathom ingest --service fathom' runs under the recurring wedge each wrote a zero-symbol index reporting '(0 failed)'; the empty index then made 'fathom verify' refute all 24 freshly-proven plan rows via symbol_present-not-found. Daemon-side MCP ingest (its own TCC grant) produced 8703 symbols from the same tree seconds later
  - **Repro:** with the ghostty process tree wedged (recurs on /Volumes/m2ext), run bin/fathom ingest --service fathom → symbols: 0, typed-edges (0 failed), exit 0; heal via daemon-side mcp ingest tool
- **Root cause:** the fetch pass aborted on the FIRST read error with no per-file read-failed counter, and nothing stopped a run that read nothing from advancing the watermark and replacing the service's observations — so a wedged process tree produced a plausible empty index that printed `(0 failed)`.
- **Fix:** IT-14/IT-15 (docs/plans/instrument-truth.md, `internal/fusion/readfailures.go`): per-file read failures are recorded and named in a `read-failed:` column with errno (+ the external-volume TCC hint on EPERM), the readable files are still indexed, the run exits 1; a run that could read NOTHING refuses to touch an index that already holds observations for the service (no watermark advance, no purge, no write) and the daemon's `ingest` tool returns the same cause as a JSON-RPC error.
- **Status:** fixed (2026-08-25) — IT-14/IT-15; the ingest-side refusal covers the whole-tree wedge shape O16 recorded (zero readable files); a partially wedged tree now indexes what it can and exits 1 naming the rest.
- **Impact:** the fourth false-signal shape of the wedge is closed at the writer: a good index can no longer be replaced by an empty one, and an ingest under the wedge is loud.

### O17 — instrument --adoption under a wedged call-log read reports 'zero-call registered tools (88 of 88)' — every tool falsely zero — instead of naming the read error; the IT-1 read-error classification landed in flywheel but not here

- **Symptom:** instrument --adoption under a wedged call-log read reports 'zero-call registered tools (88 of 88)' — every tool falsely zero — instead of naming the read error; the IT-1 read-error classification landed in flywheel but not here
- **Evidence:**
  - **Tool:** instrument --adoption / readCallLog consumer
  - **Expected:** an unreadable call log is reported as a read error (errno named, TCC hint for external volumes), never rendered as universal zero adoption
  - **Actual:** with the TCC wedge active, the command printed 'zero-call registered tools (88 of 88 registered)' listing every tool including find_symbol/verify_plan which have thousands of calls; after tccutil reset the same command reads 31034 calls
  - **Repro:** wedge the process tree, run fathom instrument --adoption, observe 88/88 zero; reset and re-run to see the true counts
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** fixed (2026-08-23) — SP-B3, docs/plans/surface-prune.md: classifyCallLogRead in instrument_adoption.go renders READ ERROR + NOT COMPUTED, never universal zero
- **Impact:** TODO

### O18 — verify's test_reaches cannot see HTTP handlers wired through a mux: every webui handler row needs a direct-call test to prove (G15, reported by a consumer-repo session 2026-08-25)

- **Symptom:** verify's test_reaches cannot see HTTP handlers wired through a mux: every webui handler row needs a direct-call test to prove (G15, reported by a consumer-repo session 2026-08-25)
- **Evidence:**
  - **Tool:** verify / internal/fusion/typedcalls.go + internal/extractor/goextract.go collectCalls
  - **Expected:** an httptest-driven test that builds the server (which registers handlers via mux.HandleFunc / route tables / http.HandlerFunc) statically reaches every handler it registers
  - **Actual:** both edge passes only record *ast.CallExpr targets; a function passed as a VALUE (mux.HandleFunc(p, h), []route{{p, h}}, http.HandlerFunc(h), middleware(h)) is never an edge -- typedcalls.go's header explicitly excludes func values -- so the handler is unreachable and the row must take skip_test
  - **Repro:** plan row {symbol: handleX, reachable_from: TestServerServesX} where TestServerServesX calls newServer() and hits httptest; fathom verify -> edge_wired fails
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** fixed (2026-08-25) — VS-6, docs/plans/verify-spine.md: function values are edges in both passes (core.EdgeFuncValue); a test that builds the server reaches every registered handler; harness VS-10 proves mux/route-table/HandlerFunc/middleware/method-value forms
- **Impact:** TODO

### O19 — fathom verify prints every plan row on every run, so the pre-commit/pre-push gate output is a wall of ✓ rows per plan per commit (G14, consumer-repo session 2026-08-25)

- **Symptom:** fathom verify prints every plan row on every run, so the pre-commit/pre-push gate output is a wall of ✓ rows per plan per commit (G14, consumer-repo session 2026-08-25)
- **Evidence:**
  - **Tool:** verify CLI output
  - **Expected:** a --quiet (failing-only) mode: print ✗ rows and the summary line, suppress ✓ rows; the gate scripts use it
  - **Actual:** no such flag; every row prints
  - **Repro:** run the lefthook pre-push verify loop over ~40 plans and read the output
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** fixed (2026-08-25) — VS-8: `fathom verify --quiet` (failing-only + summary); lefthook and the init guidance carry it
- **Impact:** TODO

### O20 — A daemon predating HEAD turned the pre-push verify gate into a FALSE FAILURE: docs/plans/ambient-fathom.md blocked the push with edge_wired misses (TestCmdScopePreviewMatchesEffect does not reach main.cmdScope; same for cmdPurge/cmdThread rows) while doctor reported binary_fresh PASS.

- **Symptom:** A daemon predating HEAD turned the pre-push verify gate into a FALSE FAILURE: docs/plans/ambient-fathom.md blocked the push with edge_wired misses (TestCmdScopePreviewMatchesEffect does not reach main.cmdScope; same for cmdPurge/cmdThread rows) while doctor reported binary_fresh PASS.
- **Evidence:**
  - **Tool:** verify (pre-push) + mcp ingest
  - **Expected:** verify to gate on what the tree actually proves — the symbols and tests both exist and the chain Test -> run -> runWithStdin -> cmdScope is intact in source.
  - **Actual:** Exit 1 against the daemon's store, exit 0 against a HEAD-built store. Same plan, same tree, two stores. The daemon process had ~9h uptime (older binary) and it is what serviced the MCP ingest refresh, so the shared index was rebuilt by pre-HEAD extraction code. doctor's binary_fresh note already says 'local binary only, the daemon installed copy is unverified' but the row still reads PASS, so the diagnostic said fine while the gate failed loudly. Candidate remedies: (1) verify/doctor compares the DAEMON's binary stamp to HEAD, not only the local one, and annotates a gate failure whose store an older binary built; (2) the MCP ingest tool refreshing a shared store from a stale daemon binary is the hazard itself — O25's sibling.
  - **Repro:** bin/fathom verify --service fathom --plan docs/plans/ambient-fathom.md  # exit 1 (daemon store)   vs   bin/fathom ingest -service fathom -path . -store workspace/tmp/verifyprobe/probe.sqlite && bin/fathom verify --service fathom --plan docs/plans/ambient-fathom.md --store workspace/tmp/verifyprobe/probe.sqlite  # exit 0, 77 claimed / 75 verified / 0 failing
- **Root cause:** A gate verdict is a function of the store, and the store is a function of the binary that built it — and nothing recorded which binary that was. The launchd daemon (6c6f437) serviced the MCP `ingest` refresh, so the shared index was extracted by pre-HEAD code, while `fathom verify` (the same installed binary) judged HEAD's plan against it; doctor's `binary_fresh` compared only the LOCAL binary to HEAD and passed. Two instruments, neither able to say "this verdict came from a stale extractor", and one of them coloured red.
- **Fix:** PC-1..5 (`docs/plans/proof-credibility.md`): `/statusz` schema 2 carries the daemon's `binary_commit`/`binary_dirty`/`built_at` (`statuszBuildInfo`); every verify-ledger row carries `daemon_commit`, `store_commit`, `binary_commit` (`verifyLedgerProvenance`); `fathom verify` prints one `provenance:` line when the daemon's build is not this binary's (`provenanceNotice`, never vs the cwd's HEAD — O5); doctor `daemon_fresh` compares the daemon's stamp to this binary and `binary_fresh` says it knows the local binary only (`checkDaemonFresh`); `plans status`, `intent audit` and doctor's `unproven_claims` count a stale daemon's readings as `measured-stale` — a coverage gap, never a failure (`markMeasuredStale`, ADR-027). The operator remedy stays `task deploy`.
- **Status:** fixed (2026-09-02; the sibling hazard — an MCP `ingest` refreshing the shared store from a stale daemon — is O25's and is now at least VISIBLE through `daemon_fresh` and the provenance line)
- **Impact:** one false red push gate on 2026-09-01 (five rows), ~40 minutes of reproduction; before this fix every such failure looked like a code defect.

### O21 — Adding a third package main binary (cmd/loctruth) silently REFUTED five previously-verified ambient-fathom rows (SP-4, SP-7, SP-8, SP-9, TH-1) that had nothing to do with it.

- **Symptom:** Adding a third package main binary (cmd/loctruth) silently REFUTED five previously-verified ambient-fathom rows (SP-4, SP-7, SP-8, SP-9, TH-1) that had nothing to do with it.
- **Evidence:**
  - **Tool:** fathom verify (callgraph)
  - **Expected:** A new binary in its own directory cannot change whether cmd/fathom's TestCmdScopePreviewMatchesEffect reaches cmd/fathom's cmdScope.
  - **Actual:** core.SymbolNode.Key() is service::package.Name, so every 'func run' in every package main collapses to one key fathom::main.run, and callgraph.Build does g.nodes[n.Key()] = n — last-writer-wins. With two main packages cmd/fathom's run happened to win; the third made cmd/loctruth's run win, deleting the run->runWithStdin hop for the whole graph. The verdict therefore depended on ingest order, not on code.
  - **Repro:** fathom ingest -service fathom -path . -store probe.sqlite && fathom verify --store probe.sqlite --service fathom --plan docs/plans/ambient-fathom.md  # before/after adding any second func run to a package main
- **Root cause:** `core.SymbolNode.Key()` was `service::package.Name` with the package's declared NAME, so every `package main` shared the qualifier `main`; `callgraph.Build` deduplicates nodes by key, last writer wins, and the typed pass keyed its edges the same way.
- **Fix:** PC-32 — the key carries the package DIRECTORY (`core.QualifiedPackage`: `service::<dir>/<package>.Name`); `callgraph` binds same-package and package-name-qualified callees to the caller's directory; the typed pass derives the same key from the declaring file (`TypedEdgeEpoch` 2 rebuilds stored typed edges); verify's `package:` filter is a one-directory match. `TestTwoMainsSameFuncNameNeverCollapse` (reachfix `cmd/a`, `cmd/b`) pins it; the integrator's `runLocTruth` rename is no longer load-bearing.
- **Status:** fixed (CHANGELOG [Unreleased], PC-32)
- **Impact:** every verdict over a fleet with two or more `package main` binaries — five ambient-fathom rows refuted by an unrelated commit, and any `cmd/*` symbol's callers/tests reported for the wrong binary.

### O22 — The MCP ingest tool returned {ok:true} four times while the daemon's service 'fathom' was rooted at a stale agent worktree, so every 'refreshed' daemon verify read a tree that was not merged main (0-2 verified where the probe store read 16/5/5/9/13).

- **Symptom:** The MCP ingest tool returned {ok:true} four times while the daemon's service 'fathom' was rooted at a stale agent worktree, so every 'refreshed' daemon verify read a tree that was not merged main (0-2 verified where the probe store read 16/5/5/9/13).
- **Evidence:**
  - **Tool:** MCP ingest / serve-http daemon
  - **Expected:** ingest {service: fathom} refreshes the index for the repo I am working in, or says it cannot.
  - **Actual:** staleness_report showed current_commit == indexed_commit == c9f2ca3c (a commit only on branch worktree-wf_58a5b5d5-a58-1) with reingest_recommended:false — the daemon was honestly fresh about the wrong tree, and nothing in the ok:true envelope named the root it ingested.
  - **Repro:** mcp ingest {service: fathom}; mcp staleness_report {service: fathom}  # compare current_commit against git rev-parse HEAD in your checkout
- **Root cause:** a worktree's post-commit hook called `ingest {service, path: <worktree>}`; the tool's root-hygiene guard only compared the path against OTHER services' roots, so it re-rooted the existing service at the worktree and answered `ok:true` without naming any root.
- **Fix:** PC-33 — `internal/worktree.CheckReroot` (shared by the MCP tool and `pkg/client`) refuses to re-root an existing service at a git worktree of its recorded root or any `.claude/worktrees/` path, naming both paths; the `ok` envelope carries `root`; the post-commit hook detects a worktree and refreshes nothing, saying which service and root it left alone. `TestIngestRefusesWorktreeReroot`, `TestPostCommitHookLeavesWorktreeAlone`, `TestCheckReroot`.
- **Status:** fixed (CHANGELOG [Unreleased], PC-33; I30 extended)
- **Impact:** four daemon verifies read a tree that was not main (0-2 verified where the probe store read 16/5/5/9/13) with no signal that the root had moved.

### O23 — Bench run-20260902T211103Z H1-T01: obligations for internal/extractor/goextract.go served 6 rules, displaced 65 — all six are spec-bindings from cpp-toolchain.md (CT-1..4), lang-truth.md (CUE) and extraction-treesitter.md; the internal/CLAUDE.md extractor-wiring rule (fusion.New + golden test) the file is actually governed by was NOT served. 71 rules matched one Go file; plan-row bindings keyed on the word 'extractor' crowded out the human-written CLAUDE.md rule. H1-T02 (model.go): 64 matched, the right rule ranked 1 but flagged stale, beside the O14 mis-scoped 'ext→kind mapping' rule and a process rule ('Advance waves only when…') bound to a Go file.

- **Symptom:** Bench run-20260902T211103Z H1-T01: obligations for internal/extractor/goextract.go served 6 rules, displaced 65 — all six are spec-bindings from cpp-toolchain.md (CT-1..4), lang-truth.md (CUE) and extraction-treesitter.md; the internal/CLAUDE.md extractor-wiring rule (fusion.New + golden test) the file is actually governed by was NOT served. 71 rules matched one Go file; plan-row bindings keyed on the word 'extractor' crowded out the human-written CLAUDE.md rule. H1-T02 (model.go): 64 matched, the right rule ranked 1 but flagged stale, beside the O14 mis-scoped 'ext→kind mapping' rule and a process rule ('Advance waves only when…') bound to a Go file.
- **Evidence:**
  - **Tool:** obligations
  - **Expected:** For 'what governs this file', the CLAUDE.md obligations layer outranks spec-bindings (plan rows are 'what claims this file', a different question), and a plan row scopes by its {package} contract, not by prose keyword.
  - **Actual:** Fathom arm passed T01 only by falling back to read_source internal/CLAUDE.md after obligations returned irrelevant rules — the cell PASSes while the tool it was designed to exercise FAILED. Task-level success hides tool-level failure.
  - **Repro:** obligations {service: fathom, file: internal/extractor/goextract.go} against the daemon at 138e465; transcript workspace/artifacts/context-bench/run-20260902T211103Z/raw/H1-T01-fathom-t1.stream.jsonl
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O24 — Every fathom-arm cell in run-20260902T211103Z opens with 1–2 ToolSearch turns to load deferred MCP tool schemas (Claude Code 2.1.258 defers MCP tools; Bash is core and never deferred). July's run-20260724T033131Z has zero ToolSearch calls. On H1-T01, 2 of the fathom arm's 4 tool calls are ToolSearch; each is a full turn at ~28k cache-read tokens. The grep arm pays none of this.

- **Symptom:** Every fathom-arm cell in run-20260902T211103Z opens with 1–2 ToolSearch turns to load deferred MCP tool schemas (Claude Code 2.1.258 defers MCP tools; Bash is core and never deferred). July's run-20260724T033131Z has zero ToolSearch calls. On H1-T01, 2 of the fathom arm's 4 tool calls are ToolSearch; each is a full turn at ~28k cache-read tokens. The grep arm pays none of this.
- **Evidence:**
  - **Tool:** context-bench
  - **Expected:** Both arms pay the same harness overhead, or the bench reports ToolSearch turns as a named, excluded line item.
  - **Actual:** An asymmetric harness tax of 1–2 turns per cell lands entirely on the treatment arm and is counted against it by H2-1's token half. Also a daily user-facing cost: every session's first fathom call pays it.
  - **Repro:** grep -c ToolSearch workspace/artifacts/context-bench/run-20260902T211103Z/raw/H1-T01-fathom-t*.stream.jsonl vs the July raw dir
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O25 — H1-T02's ground truth ('internal/core', 'pkg/client') is verbatim in the root CLAUDE.md, which every claude -p session auto-loads. The none arm (no tools) passed it 2/2 at ~28k tokens in 5.5s by quoting the system prompt. Under the committed leak_rule the task is leaked and leaves the verdict — the mechanism worked, but only after spending 6 tool-arm cells on it.

- **Symptom:** H1-T02's ground truth ('internal/core', 'pkg/client') is verbatim in the root CLAUDE.md, which every claude -p session auto-loads. The none arm (no tools) passed it 2/2 at ~28k tokens in 5.5s by quoting the system prompt. Under the committed leak_rule the task is leaked and leaves the verdict — the mechanism worked, but only after spending 6 tool-arm cells on it.
- **Evidence:**
  - **Tool:** context-bench
  - **Expected:** A static gate: no ground-truth token may appear verbatim in any file the harness auto-loads (root CLAUDE.md, AGENTS.md, user memory). TestHomeCorpusTruthAtHead already walks every token; the same walk can assert absence from the baseline set for free.
  - **Actual:** Leaks are only discovered by spending LLM tokens on the none arm.
  - **Repro:** grep -n 'pkg/client' CLAUDE.md; results/H1-T02-none-t1.json raw_result
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O26 — Per-cell token breakdown in run-20260902T211103Z: input 4–12, output 400–1,600, cache_creation 1–30k, cache_read 28k–180k. total_tokens is cache_read × turns — a turn counter wearing a token costume. The 3.1× spread on identical H1-T02 fathom trials (59k/181k/148k) is 3 vs 6 vs 5 turns; the discovery payload fathom claims to shrink is invisible under a ~28k/turn baseline both arms re-read.

- **Symptom:** Per-cell token breakdown in run-20260902T211103Z: input 4–12, output 400–1,600, cache_creation 1–30k, cache_read 28k–180k. total_tokens is cache_read × turns — a turn counter wearing a token costume. The 3.1× spread on identical H1-T02 fathom trials (59k/181k/148k) is 3 vs 6 vs 5 turns; the discovery payload fathom claims to shrink is invisible under a ~28k/turn baseline both arms re-read.
- **Evidence:**
  - **Tool:** context-bench
  - **Expected:** A payload-scoped metric registered before the next run: tool-result bytes per cell, turns excluding ToolSearch, and cost_usd — H2-1's total-token half kept as history, never re-scored.
  - **Actual:** H2-1's token criterion rewards fewer turns, not smaller payloads; the July record already said a payload-scoped claim needs a payload-scoped instrument, and none exists.
  - **Repro:** results/H1-T02-fathom-t{1,2,3}.json fields input_tokens/cache_read_tokens/num_turns
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O27 — (interactive mode only — see O32 for the headless measurement) O24's deferral tax does not reproduce on Claude Code 2.1.270: the whole 17-tool, 28,715-byte agent surface loads eagerly and a session spends ZERO ToolSearch turns, with and without `alwaysLoad: true`. ADR-032's own revisit trigger ("a harness release removes MCP tool deferral") has fired.

- **Symptom:** O24's deferral tax does not reproduce on Claude Code 2.1.270: the whole 17-tool, 28,715-byte agent surface loads eagerly and a session spends ZERO ToolSearch turns, with and without `alwaysLoad: true`. ADR-032's own revisit trigger ("a harness release removes MCP tool deferral") has fired.
- **Evidence:**
  - **Tool:** mcp-surface
  - **Expected:** ADR-032 §5 asked for the agent-surface schema budget to be held under the harness's deferral threshold, measured per release when undocumented (BP-22). Claude Code documents no threshold, so it was measured.
  - **Actual:** Two `claude -p --output-format stream-json` runs against the live daemon's `/rpc/agent` surface (28,715 bytes / 17 tools, `capabilities.schema_bytes`), one with `alwaysLoad: true` and one without: BOTH reported all 17 `mcp__fathom__*` tools in the `system/init` tool list and zero `ToolSearch` tool_use blocks in the stream. O24 (claude 2.1.258) measured 1–2 ToolSearch turns per cell at ~28k cache-read tokens each. `alwaysLoad` is a startup-ordering flag per Claude Code's MCP docs (startup waits for the server), NOT a deferral control, and the measurement is consistent with that. Consequence: shrinking the surface to the ~16kB July size would have to come out of the generated `outputSchema`s (10,693 bytes, the RP-11 machine-checkable contract) or the per-tool "Reach for it when …" sentences, to chase a threshold that is measurably not binding. `$ref`/`$defs` compaction was evaluated and saves exactly 0 bytes: every repeated sub-schema repeats ACROSS tools, and tools/list gives each tool an independent schema document.
  - **Repro:** claude -p "list the fathom MCP tools you can call, names only" --output-format stream-json --verbose --mcp-config <{"mcpServers":{"fathom":{"type":"http","url":"http://127.0.0.1:7575/rpc/agent","alwaysLoad":true}}}> --strict-mcp-config --allowedTools "mcp__fathom__*" --model sonnet, then count tool_use blocks named ToolSearch
- **Root cause:** A harness behavior change between claude 2.1.258 and 2.1.270, outside fathom.
- **Fix:** `TestNarrowProfileSchemaBytes` re-pinned at the same 26,000–30,000 band with this measurement as its stated reason (a budget, not a threshold); `alwaysLoad: true` added to the Claude Code plugin's `.mcp.json` and to the claude-code descriptor's MCP note for the startup-ordering benefit it does have; `toolsearch_turns` shipped on the session/exploration ledger (BP-23) so a future harness release that re-introduces deferral shows up as a number instead of a surprise.
- **Status:** open
- **Impact:** BP-22 cannot be checked off as written — the plan row asks for the surface to be held under a threshold, and the honest finding is that no threshold is binding. The row needs re-wording against this measurement before it is claimed.

### O28 — init wrote .codex/hooks.json for two waves, and codex-cli never read it

- **Symptom:** init wrote .codex/hooks.json for two waves, and codex-cli never read it
- **Evidence:**
  - **Tool:** fathom init --harness=codex
  - **Expected:** the documented project hook layer (<repo>/.codex/hooks.json) loads under directory trust, as Codex's hooks reference describes
  - **Actual:** on codex-cli 0.153.1, touch hooks placed in <repo>/.codex/hooks.json on SessionStart, Stop, SessionEnd and UserPromptSubmit fired ZERO times in a headless codex exec run under a trusted parent project with [features] hooks = true and --dangerously-bypass-hook-trust; the identical entries under $CODEX_HOME/hooks.json fired every time. Every fathom Codex adopter since 2026-09-02 has been given a hook file the harness ignores, and doctor's harness_hooks row read it back as evidence of wiring. Fixed: the descriptor records the project file as documented-but-not-loaded with no wiring kind, init writes nothing and prints the entries (fathom harness hooks --harness codex), and the descriptor is now verified against a real session instead of a fetched doc page. The general lesson: a documented extension point is a claim, and the only thing that settles it is a smoke test that would have failed.
  - **Repro:** place a touch hook in <repo>/.codex/hooks.json, run: codex exec --json --sandbox read-only --dangerously-bypass-hook-trust 'reply ok'
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O29 — The launchd daemon hung for minutes inside open() of the SQLite store on /Volumes/m2ext (goroutine 1 [syscall] in modernc sqlite _posixOpen, SIGQUIT dump) after 'fathom daemon uninstall' + 'install': install's defaults chose a new label (dev.fathom.serve-http), a fresh ~/.fathom/db store and dropped WorkingDirectory and the call-log arg; even after reinstalling with the fleet store, the original label, the fathom-dev-signed libexec binary and WorkingDirectory restored, the process bootstrapped from this shell inherited a wedged removable-volume TCC verdict and never bound the port. The same signed binary started in the foreground from the shell opened the store in 6 s.

- **Symptom:** The launchd daemon hung for minutes inside open() of the SQLite store on /Volumes/m2ext (goroutine 1 [syscall] in modernc sqlite _posixOpen, SIGQUIT dump) after 'fathom daemon uninstall' + 'install': install's defaults chose a new label (dev.fathom.serve-http), a fresh ~/.fathom/db store and dropped WorkingDirectory and the call-log arg; even after reinstalling with the fleet store, the original label, the fathom-dev-signed libexec binary and WorkingDirectory restored, the process bootstrapped from this shell inherited a wedged removable-volume TCC verdict and never bound the port. The same signed binary started in the foreground from the shell opened the store in 6 s.
- **Evidence:**
  - **Tool:** daemon install / launchd
  - **Expected:** 'fathom daemon install' on a machine that already has an agent should refuse or reuse the existing label/store/call-log/WorkingDirectory rather than silently create a second, default-configured agent; 'daemon status' should name a hung-in-open() process as a TCC consult (pid alive, no listen socket, no 'listening' log line for this run) with the Ghostty-relaunch remedy from the playbook; the foreground fallback should be a documented verb.
  - **Actual:** A second agent with the wrong store came up; the restored agent hung; doctor said only 'NOT reachable'; the fix was manual: bootout, foreground serve-http via nohup, GOPATH exported by hand.
  - **Repro:** fathom daemon uninstall && fathom daemon install  # then launchctl print gui/$(id -u)/com.geoffmilleraz.fathom-http shows pid alive, lsof -iTCP:7575 empty; kill -QUIT <pid>; grep -n 'goroutine 1 \[syscall' ~/.fathom/logs/fathom-http.err
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O30 — doctor's typed_edges row SKIPs with 'this daemon process has not ingested "sextant"' while the daemon's own staleness_report for service fathom says typed_edges packages=211 failed=false — the row derives the service id from the directory name (sextant) instead of resolving it the way obligations_served does (fathom).

- **Symptom:** doctor's typed_edges row SKIPs with 'this daemon process has not ingested "sextant"' while the daemon's own staleness_report for service fathom says typed_edges packages=211 failed=false — the row derives the service id from the directory name (sextant) instead of resolving it the way obligations_served does (fathom).
- **Evidence:**
  - **Tool:** doctor typed_edges
  - **Expected:** PASS with the package count for service fathom, resolved like every other daemon-backed row.
  - **Actual:** SKIP naming a service that does not exist.
  - **Repro:** bin/fathom doctor | grep typed_edges   # vs   curl -s -X POST http://127.0.0.1:7575/rpc -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"staleness_report","arguments":{"service":"fathom"}}}' | grep -o '"typed_edges":{[^}]*}'
- **Root cause:** `checkTypedEdges` (cmd/fathom/doctor_typed.go) resolved its `svc` from bare `serviceCandidates(root)` — the directory-basename guess only — instead of `candidatesWithDaemon(hc, baseURL, root)`, the daemon-resolved-path-first resolution `checkObligationsServed` already uses; a repo whose dir basename does not name (or misnames) the indexed service always asked `staleness_report` for the wrong id and got `known:false`.
- **Fix:** `checkTypedEdges` now resolves `svc` via `candidatesWithDaemon` when no `--service` is given, same as `checkObligationsServed`. `TestCheckTypedEdges_ResolvesServiceLikeObligationsServed` (cmd/fathom/doctor_typed_test.go) pins it: dir named "sextant", daemon indexes it as "fathom" at the matching path — row now PASSes naming "fathom" and its package count instead of SKIPping "sextant".
- **Status:** fixed
- **Impact:** every doctor run on this repo (and any adopter with the same dir-basename/service-id split) reported typed_edges SKIP instead of the daemon's real PASS/FAIL verdict.

### O31 — The first replicated home run (run-20260913T002832Z, 3 arms × 13 tasks × n=3, $7.90) refutes the cost half of both criteria: fathom passed 31/33 to grep's 28/33 (p=0.042, CI −0.06..+0.24) but returned 1.90× grep's tool-result bytes and cost 1.46× as much; `read_source` was the largest single payload (38 calls, 123 KB of 419 KB) and the worst task (H1-T06, payload 18.6×) was a two-line grep that the fathom arm answered in 492 bytes with `find_symbol` and then kept exploring with `references` calls that returned 6–7 KB each, one of them for the junk query `func (s *`.

- **Symptom:** H2-1 FAIL (tokens 1.444×), H2-3 FAIL (payload 1.901×, cost 1.465×); leaked H1-T04, H1-T12 (the none arm passed both 3/3); flaky in one arm or the other H1-T02, T07, T08, T11, T13. fathom won outright on H1-T07 (3/3 vs 1/3, 0.41× tokens, 0.21× payload) and H1-T13; lost cost on every greppable lookup (T06 18.6×, T11 6.5×, T08 4.5×, T03 3.5×, T10 2.7× tokens).
- **Evidence:**
  - **Tool:** references, read_source, query_symbols, outline (payload per call 2.4–3.2 KB; grep's Bash 1.6 KB)
  - **Expected:** fathom's own payload ≤ grep's on its home corpus (H2-3, ADR-032 §3).
  - **Actual:** fathom arm 209 tool calls / 419 KB over 33 cells; grep arm 138 calls / 222 KB. gold_source_rate[fathom]: obligations 0.18, references 0.18, read_source 0.15, find_callers 0.12, query_symbols 0.12, answer 0.09.
  - **Repro:** docs/bench/ledger.jsonl rows run_id 20260913T002832Z; per-tool bytes from workspace/artifacts/context-bench/run-20260913T002832Z/raw/*.stream.jsonl
- **Root cause:** three, each measured: (1) `references` renders ~220 bytes per occurrence (seven JSON keys and a text line) and answers a textual junk query with 27 heuristic lines instead of a refusal; (2) the fathom arm has no `cat`, so `read_source` is used as one and its bytes count against fathom while the grep arm's `cat` bytes count against grep equally — a wash in bytes but not in turns; (3) the model keeps exploring after the index has answered (T06: the answer was in the first 492-byte result), which the tool descriptions do not stop.
- **Fix:** pre-registered as BP-40..43 in docs/plans/bench-pyramid.md; the targeted L3 run over T06/T10/T11 must show payload ≤ 1.0× before the next L4 spend.
- **Status:** open
- **Impact:** the home bench is honest and cheap to re-run (grep and none cells borrow by key; only the 39 fathom cells re-run, ~$2.60); the win claim is success rate, not cost, until BP-40..43 land.

### O32 — O27 was measured in the wrong harness mode: in headless `claude -p` (the bench runner) every one of the 39 fathom cells spent at least one `ToolSearch` turn (histogram 1:31, 2:7, 3:1; 48 turns in the 33 scored cells), and those turns carried 11.4% of the fathom arm's assistant-turn tokens (1.41M of 12.35M); the grep arm spent zero.

- **Symptom:** the stream-json `init` event lists all 17 `mcp__fathom__*` tools, yet the model's first fathom call is preceded by `ToolSearch {"query":"select:mcp__fathom__find_symbol,..."}` — the schemas are deferred in headless mode at 28.7 KB even though an interactive session on the same binary (O27) loaded them eagerly.
- **Evidence:**
  - **Tool:** ToolSearch (harness), toolsearch_turns on the ledger row (treatment_toolsearch_turns_mean = 1.27)
  - **Expected:** zero ToolSearch turns, as O27 measured interactively on 2.1.270.
  - **Actual:** ≥1 per headless cell, 11.4% of the arm's tokens, ~2% of its payload bytes.
  - **Repro:** python tally over workspace/artifacts/context-bench/run-20260913T002832Z/raw/*-fathom-t*.stream.jsonl counting tool_use blocks named ToolSearch
- **Root cause:** the deferral rule differs between interactive and `-p` sessions (or by tool-count/config path); O27 generalised one mode to the harness. Whether a smaller schema would avoid it in headless mode is unmeasured.
- **Fix:** BP-22 re-dispositioned deferred (not refused) with the headless measurement; ADR-032 amended; H2-3 already reports the turns without gating them (it is a harness cost, not a payload cost — and it is real money for a headless adopter).
- **Status:** open
- **Impact:** ~11% of the fathom arm's token cost in headless runs is the harness loading fathom's schemas; the 1.444× token ratio is ~1.28× without it, still over 1.0.

### O33 — verify_plan's daemon latency doubled between 2026-09-09 (median ~4.5 s over the day's calls) and 2026-09-12 (10.7–14.8 s, every call), and `fathom verify`'s 10 s client timeout then turned every verification into exit 2 (operational, fail-open): the pre-push gate looked green while proving nothing.

- **Symptom:** `fathom verify --plan docs/plans/bench-pyramid.md` → `error: POST http://localhost:7575/rpc: context deadline exceeded`, exit 2, while the daemon's call log shows the same verify_plan completing in 11.6 s with claimed 39 / failing 0.
- **Evidence:**
  - **Tool:** verify_plan (daemon), fathom verify (CLI client, cmd/fathom/verify.go)
  - **Expected:** a verify call completes inside the client budget, or the budget is set from the measured call.
  - **Actual:** workspace/logs/mcp-calls.jsonl: verify_plan latency_ms 3906–5447 on 2026-09-09, 10029–14795 on 2026-09-12/13 (same service, 10,985 symbols); client timeout 10 s.
  - **Repro:** grep '"tool":"verify_plan"' workspace/logs/mcp-calls.jsonl | tail -20   # latency_ms
- **Root cause:** two, both found by profiling a scratch daemon (this repo, 11,532 symbols): (1) `registerVerifyPlan` (internal/mcp/verify_plan_tool.go) built its call graph from scratch on EVERY call — `callgraph.Build(nodes)` alone measured 13.4–25.4s in isolation (every other stage — ListAllSymbols, TypedEdges, AddTypedEdges, QuerySymbols — under 200ms combined) — never once calling into the watermark-keyed memo (BP-28/I21) that `resolved_nav.go`'s nav tools already use; (2) even for callers that DID use the memo, `serveHTTP`'s default config (`daemonAutoRefreshDefault` true) wraps the backend in `autoRefreshBackend`, which embeds `QueryInterface` but never forwarded the optional `StoreWatermark` capability — the exact "interface embedding does not forward" O7 class this file already names for `IngestStats`/`TypedEdgeStatus` — so `serviceWatermark`'s type assertion always failed, the watermark was always `""`, and the memo silently cached nothing at all on the real daemon, for nav tools and verify_plan alike, regardless of how their callers were wired.
- **Fix:** `registerVerifyPlan` now builds its graph via `s.graphCache.Graph(service+"\x00both", mark, …)` — the same cache key `loadNavGraphs` uses, so verify_plan and the nav tools share one built graph per watermark; `autoRefreshBackend` gained a `StoreWatermark` passthrough. `TestVerifyPlan_ReusesWatermarkedCallGraphMemo` (verify_plan_tool_test.go) and `TestNavGraphMemoSurvivesAutoRefreshWrapper` (resolved_nav_test.go) pin both fixes via the new `Server.GraphCacheStatsForTest` accessor; `TestVerifyReachabilityUsesBothEdgeKinds` pins that no verdict moved.
- **Fix (SB-50, docs/plans/strengths-bench.md):** `resolveAmbiguousCallees` now resolves through a component-boundary suffix index (`bySuffix`/`byDotSuffix`) `Build` computes once — O(nodes) — turning the fallback into O(calls) amortized. Same repo, same store: `callgraph.Build` dropped from 12.2 s best / 13.4 s avg (old) to 0.18 s best / 0.25 s avg (new), ~65-70x; edge count unchanged (17,141 both). The client budget stays 60 s (still the honest number until verify_plan's own cold latency is remeasured end-to-end) — the gap: does the daemon's per-call cost now clear the old 10 s window, and does the watermark memo actually serve repeated verify_plan calls at one watermark. Neither remeasured yet.
- **Status:** fixed — the memo now serves verify_plan and every nav tool (autoRefreshBackend forwards StoreWatermark), and the cold build is O(nodes) (SB-50): warm 0.36 s, cold 1.84 s on this repo
- **Impact:** measured on a scratch daemon (this repo, `docs/plans/bench-pyramid.md`, `tools/call verify_plan` x3): before the fix, repeated calls at the SAME watermark cost 12.9s / 23.7s / 15.2s (exactly O33's symptom — the memo never engaged). After: first call after a fresh ingest still pays the build (13.4–54s, load-dependent — a cold-cache cost the nav tools already accept under BP-28) but every repeated call at the same watermark dropped to 0.36s / 0.41s, a 30–60x drop for the common case (a session's second-and-later verify_plan call). The client's 60s budget (above) is no longer routinely needed for a warm watermark.

### O34 — The WorktreeCreate hook (HP-30) emitted hookSpecificOutput.additionalContext, which Claude Code 2.1.270 rejects on that event ("hookSpecificOutput.worktreePath: Invalid input"), and the rejection blocked EVERY worktree-isolated agent spawn from this repo — five workers refused at once on 2026-09-13.

- **Symptom:** `Agent(isolation: worktree)` → "WorktreeCreate hook failed: … Hook JSON output validation failed — hookSpecificOutput.worktreePath: Invalid input"; the hook's output was the fathom worktree advisory as additionalContext.
- **Evidence:**
  - **Tool:** fathom session-hook --event worktree-create (harness codec claudeCodec.EncodeOutput)
  - **Expected:** a hook-silent vehicle (the descriptor row says VehicleHookSilent) emits nothing the harness can reject; an advisory reaches the user.
  - **Actual:** the codec encoded every non-empty Context as hookSpecificOutput.additionalContext regardless of the event; Claude Code validates that field per event and only accepts it on SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, PostToolBatch, PostToolUseFailure, SubagentStart, Stop/SubagentStop.
  - **Repro:** echo '{"session_id":"x","cwd":"/tmp","hook_event_name":"WorktreeCreate","worktree_path":"/tmp/w"}' | bin/fathom session-hook --event worktree-create   # before: hookSpecificOutput; after: {"systemMessage": …}
- **Root cause (final):** WorktreeCreate is a CREATION hook — Claude Code reads a registered hook's stdout as the path of the worktree the hook was supposed to create, so no advisory shape can ride on it; the hook row is withdrawn (HP-29 refused) and the line moves to SubagentStart. Earlier reading: the codec had one output shape for all events; the descriptor's Vehicle was never consulted at encode time and the wave-3 events were verified against the docs' input schemas, not their output schemas (O27's sibling: a probe that exercised the wrong half).
- **Fix:** `claudeContextEvents` in internal/harness/codec.go — additionalContext only where documented, `systemMessage` (top-level, accepted on every event, shown to the user) elsewhere; `TestClaudeCodecUsesSystemMessageWhereAdditionalContextIsUndocumented`.
- **Status:** fixed (second form found on relaunch: the harness reads WorktreeCreate stdout as the worktree path, so even systemMessage broke it; the codec now emits nothing on that event and the advisory goes to stderr)
- **Impact:** every agent spawn with worktree isolation from a fathom-wired repo was blocked between the wave-3 rollout (2026-09-12) and this fix; the same shape would have hit Setup, CwdChanged, DirectoryAdded, FileChanged and ConfigChange advisories the moment they had something to say.

### O35 — The scenario driver's wired arm needed five fixes before its first real cell, none of which a unit test could have caught: a relative root landed the worktree inside the clone; the scratch daemon outlived a failed cell; a seven-minute obligations compile aborted on one section of ~130 (a malformed claude CLI envelope, a different section each run); every cell re-paid the compile; and a linked worktree of a repo with a go.work could not build (`[setup failed]`), so every detector read "violated" before the agent had done anything.

- **Symptom:** dry cells 1–4 on 2026-09-13 (fake harness, real subject, real scratch daemon, real compile, real detectors) each failed one layer deeper than the last.
- **Evidence:**
  - **Tool:** cmd/scenariobench run (PrepareSubject, wireScratchDaemon, compileLayers, RunOracle)
  - **Expected:** a wired cell prepares, compiles once per (subject, commit, arm), runs, and its detectors judge the agent's tree.
  - **Actual:** chdir on a relative path; two `serve-http` processes left running; `fathom compile: exit status 1 (claudecli: malformed response envelope)` at ADR-010 then ADR-005; `go test` `[setup failed]` under the subject's go.work; detectors violated=true on an untouched tree.
  - **Repro:** the fixes' commits name each: 0524756 (SubjectEnv), 670dc0f (retry + count), 79d35b9 (shared worktree + compiled.ok), and the abs-root fix in eab006d.
- **Root cause:** the fake-harness tests exercise the loop with a fake worktree; the subject-level plumbing (git worktree add from the clone's cwd, a subprocess's cwd, the compile's LLM transport, a sibling-module go.work) only fails against a real subject. The driver had no "real subject, fake model" tier.
- **Fix:** the five commits; the dry cell IS that tier now (`-harness fake` against the committed guards corpus) and belongs in the runbook as the step before any spend.
- **Status:** fixed
- **Impact:** ~$0.40 of compile spend on aborted cells; no pilot cell was scored on the broken oracle.

### O36 — `references` without a `services` filter fails the WHOLE call when one indexed service's root has vanished ("references: sources demo: client.Sources discover \"demo\": lstat …/scratchpad/adopt2/demo: no such file or directory") — a stale service from an earlier session's scratch adoption makes a fleet-wide references query unusable for every live service.

- **Symptom:** tools/call references {"service":"fathom","query":"HooksWired"} on the fleet daemon → -32603 naming service "demo".
- **Evidence:**
  - **Tool:** references (internal/mcp), the sources discovery it runs per service
  - **Expected:** a service whose root is gone is skipped and named in `degraded`; the live services answer.
  - **Actual:** the first unreadable service aborts the call.
  - **Repro:** curl -s -X POST http://127.0.0.1:7575/rpc/agent -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"references","arguments":{"service":"fathom","query":"HooksWired","max":3}}}'
- **Root cause:** the per-service loop returned on the first `ListAllSymbols`/`Sources` error instead of collecting it — confirmed: both discovery calls in `registerReferences` (internal/mcp/references_tool.go) used `return nil, &rpcError{...}` on a per-service error.
- **Fix:** both calls now append a `{service, reason}` row to a new `degraded` field and `continue` to the next service instead of aborting (execute-and-flag, ADR-014) — `TestReferencesDegradesUnreadableServiceRoot` and `TestReferencesDegradesUnreadableSourcesOnly` (internal/mcp/references_tool_test.go) pin a fake backend reproducing the exact repro shape.
- **Status:** fixed
- **Impact:** every fleet-wide references call on this store since the `demo` service was registered; the bench's fathom arm always passes `services: ["fathom"]` and was not affected.

### O37 — Guards pilot 1 (2026-09-13) scored 16 cells that never ran: `claude -p` died at startup ("MCP config file not found" — the driver passed --mcp-config relative to ITS cwd while the session runs in the worktree), the dead sessions were stored as cells with zero tokens, the untouched tree passed every detector so the full arm read PASS, and the re-run borrowed the dead cells as data. The first public dry cell's oracle also never ran: a subtest name with an apostrophe broke the `sh -c` test command.

- **Symptom:** `guards-i31-workspace-artifact/full/t1: PASS tokens=0 cost=0.0000`; control arm `borrowed` ×16 on the second launch; `sh: -c: line 1: syntax error near unexpected token 'ghe.com'` in the cli/cli oracle output.
- **Evidence:**
  - **Tool:** cmd/scenariobench run (ClaudeHarness, runOneCell, runOracleTests)
  - **Expected:** a session that never started is LOST, not a cell; success needs an edited tree; test names are data, not shell.
  - **Actual:** as above; raw stderr under workspace/artifacts/scenariobench/raw/*/session-0-single.stream.jsonl.stderr.
  - **Repro:** the three fixes' commit names the paths.
- **Root cause:** the harness was only ever driven by the fake transcript in tests; the fake never touches --mcp-config, never errors, always "edits" /repo/foo.go, and the guards oracle has no positive check, so a no-op tree passes.
- **Fix:** absolute raw dir; `CellLost` (never stored, counted); `Success = oracle.Passed && AnyEdits`; a `change-present` detector on every guards task; `go test` exec'd as argv with regex-quoted, anchored names.
- **Status:** fixed
- **Impact:** no scored row was published; ~$0.50 of dead-session spend; the 21 junk cells were deleted from the store.

### O38 — Public pilot 1 (cli/cli, 17 cells, ~$25): zero successes on both arms, for two reasons the cells make separable. The agents finished by writing their OWN tests and reporting "done, verified green" while the hidden upstream tests still failed (the M2 false-done shape, before the SDD readings were wired into the driver); and in 8 of 17 cells the hidden test patch could not be applied because the agent had edited the same test files ("does not match index") — SWE-bench resets those files to base before applying, the driver did not.

- **Symptom:** every public cell FAIL; `not_measurable: "test patch did not apply: …download_test.go: does not match index"` on the 10009/10154/10043-t2 cells; a control NOTES.md reading "State: done, verified green … all pass, including the new test" beside `f2p pass 0 fail 1 of 1`.
- **Evidence:**
  - **Tool:** cmd/scenariobench run (ApplyTestPatch, RunOracle), public-go.json tasks 10009, 10016, 10043, 10048, 10072, 10154
  - **Expected:** the hidden test patch applies over the agent's tree; a session's "done" is judged by the upstream tests, and the false-done reading is on the cell.
  - **Actual:** 8 cells not measurable; 9 measurable cells all failed the upstream tests (one partial: 10048 control t2 passed 2 of 4); readings carried tokens/turns/cost only — PlanCoverage/FalseDone/Resumption existed as functions but runOneCell never called them, and single-session tasks were never asked to plan.
  - **Repro:** workspace/artifacts/scenariobench/cells/*/t*.json with run_id public-pilot-1
- **Root cause:** the driver's oracle followed the patch, not the protocol (reset test files, then apply); the wave-3 readings were proven by symbol, not by a cell that carried them (SB-20..23 anchored functions, no row demanded the wiring).
- **Fix:** W7: reset-then-apply; plan-first prompt in single mode; readings wired into runOneCell with plan.md and agent.diff captured per cell for offline re-scoring; paths sanitized. The pilot re-runs on four tasks.
- **Status:** open (fix in progress)
- **Impact:** ~$25 of spend produced transcripts but no ledger row; the false-done pattern in the notes is the first direct sighting of what M2 exists to measure.

### O39 — references exact results omit known callers while verify_plan sees their test reachability

- **Symptom:** references exact results omit known callers while verify_plan sees their test reachability
- **Evidence:**
  - **Tool:** references
  - **Expected:** Known in-repo call occurrences or an explicit unsupported-typed-scope reason
  - **Actual:** references query runReview and SyncCatalogs each returned count 0 with edge_filter resolved; verify_plan reports 27 and 23 tests reaching them after the same fresh MCP ingest
  - **Repro:** MCP references service platform-engineering query runReview max 12 after MCP ingest; compare verify_plan symbol runReview package kits/review-kit/internal/cli on 2026-09-13. Six reference queries also took about a minute together.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O40 — Selector-method call edges disappear and receiver names duplicate in canonical verification targets

- **Symptom:** Selector-method call edges disappear and receiver names duplicate in canonical verification targets
- **Evidence:**
  - **Tool:** verify_plan
  - **Expected:** Direct TestFileContextIsScopedBoundedAndVersionAware calls to s.Context and s.Append are present in the call graph
  - **Actual:** symbol_context lists Open/draft/event/subject but omits direct methods; verification target becomes Store.Store.Context and reports no test reachability. Interface a.Collect similarly disappears with fathomAdapter.fathomAdapter.Collect.
  - **Repro:** Fresh MCP ingest platform-engineering, verify_plan symbol Store.Context package kits/review-kit/internal/findings; inspect TestFileContextIsScopedBoundedAndVersionAware. Public runFindingsQuery and its native runtime tests pass independently.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O40 — guards-adr-pilot-1 (32 cells, $31.45) read 30 cells as "partial": every task's change-present detector diffed ONE named file against the index, so a change the agent made in a sibling file (or committed) counted as absent. The rule detectors were clean on every cell — the second corpus in a row with zero rule violations on either arm.

- **Symptom:** `change-present (exit 1, violated)` beside a non-empty agent.diff (3–13 KB) on 30 of 32 cells; `rule` detectors exit 0 everywhere.
- **Evidence:**
  - **Tool:** cmd/scenariobench run (RunOracle detectors), testdata/scenariobench/guards-adr.json
  - **Expected:** a positive check that the requested change exists wherever the agent put it.
  - **Actual:** `test -n "$(git diff --name-only cmd/fathom/query.go)"` — one file, working tree vs index.
  - **Repro:** cells with run_id guards-adr-pilot-1; `git apply` any agent.diff onto base and run the old detector.
- **Root cause:** the detector guessed the file the change would land in; agents put it elsewhere, and a committed change has no working-tree diff.
- **Fix:** detectors receive SCENARIO_BASE_COMMIT and diff the package directory against base (SB-54); the run is recorded as violations-measurable, success-not-measurable.
- **Status:** fixed (the corpus and driver); the pilot itself is not re-run — its violation reading stands
- **Impact:** $31 bought the violation reading and not the success reading; the change-present pattern is now the same on both guards corpora.

### O41 — Every wired scenariobench cell of the 2026-09-13 round ran without fathom's MCP tools: 72/72 full sessions show mcp_servers [] in the stream-json init event; graphify's server failed 12/12 (graphify --mcp is not a command); gopls and Serena connected and were never called (0/24 sessions); every session inherited the operator's global hooks (fathom's own SessionStart primer on the control arm) and the thread brief crossed tasks sharing a worktree basename

- **Symptom:** Every wired scenariobench cell of the 2026-09-13 round ran without fathom's MCP tools: 72/72 full sessions show mcp_servers [] in the stream-json init event; graphify's server failed 12/12 (graphify --mcp is not a command); gopls and Serena connected and were never called (0/24 sessions); every session inherited the operator's global hooks (fathom's own SessionStart primer on the control arm) and the thread brief crossed tasks sharing a worktree basename
- **Evidence:**
  - **Tool:** cmd/scenariobench run (ClaudeHarness.RunSession)
  - **Expected:** The rendered arm (fathom entry merged by wireScratchDaemon, written by RenderArm) reaches claude -p; a cell whose server is absent is LOST; the control arm sees nothing of fathom
  - **Actual:** PrepareSubject rendered the arm but main.go passed the original arm to RunSessions; newSessionRequest set Arm: arm (MCP nil); the harness wrote {mcpServers:{}} and passed --strict-mcp-config, which also hid the worktree's .mcp.json. M1–M5 and the paid half of M6/M7 measured prompt text against prompt text.
  - **Repro:** python3 over workspace/artifacts/scenariobench/raw/*/session-0-*.stream.jsonl: the init event's mcp_servers is [] for every *-full-* dir; grep -c mcp__ over the assistant tool_use names is 0
- **Root cause:** `runOneCell` passed the corpus arm, not the rendered one, into `RunSessions`; the harness built its own MCP config from `Arm.MCP` (nil) and `--strict-mcp-config` hid the worktree `.mcp.json`. graphify: wrong CLI (`--mcp`). gopls/Serena: present but un-named by any routing text, so the agent grepped. Hooks: `SubjectEnv` inherits `os.Environ()` and the operator's `~/.claude`. Threads: `threadScope{repo: filepath.Base(repoRoot)}`.
- **Fix:** fd49bb7 (rendered config reaches the harness; `alwaysLoad`; allow-lists cover `mcp__<server>`; init event parsed; absent server ⇒ LOST; `tool_calls`/`surface_calls`/`surface_unused` on every row). The clean room, competitor stanzas, graphify command, thread scope and the voiding of the 74 rows are `docs/plans/receipts.md` wave 0–3 (vision 28, ADR-034).
- **Status:** fixed (driver); reruns pending (receipts wave 5)
- **Addendum (2026-09-13, review of the merged fix):** the wired arm had no fathom HOOKS either. `PrepareSubject` ran `fathom init` (real hook stack into `.claude/settings.json`) and then `RenderArm` truncated that file to a single PostToolUse running `fathom obligations-hook`, a subcommand that does not exist. The "hook" text the first-round readers saw in wired transcripts was the operator's global SessionStart hook, present on both arms. Fixed on branch fix-a (RenderArm never replaces an existing hooks key; hook commands name the cell's binary) with a new instrument-truth gate: a wired session with no SessionStart hook response is LOST.
- **Impact:** ≈ $108 of scenario spend measured prompt text against prompt text; M1–M5 and the paid M6/M7 readings are voided and restated as unmeasured. The home bench (tools present) stands.

### O42 — fathom init writes no project-scope hooks when the same hooks are already wired at user scope (~/.claude/settings.json): the note says 'not created — every fathom hook already wired at user scope'. Under a clean-room harness (claude -p --restricted --settings <project>/.claude/settings.json, which ignores user settings) or in CI, the repo then has no fathom hooks at all. The scenario bench's wired arm hit this in the first dry cell after the review fixes (assertWiredHooksPresent refused the worktree).

- **Symptom:** fathom init writes no project-scope hooks when the same hooks are already wired at user scope (~/.claude/settings.json): the note says 'not created — every fathom hook already wired at user scope'. Under a clean-room harness (claude -p --restricted --settings <project>/.claude/settings.json, which ignores user settings) or in CI, the repo then has no fathom hooks at all. The scenario bench's wired arm hit this in the first dry cell after the review fixes (assertWiredHooksPresent refused the worktree).
- **Evidence:**
  - **Tool:** fathom init --harness claude-code (cmd/fathom/init.go hook writer)
  - **Expected:** A repo wired by fathom init carries its hook stack wherever a harness that loads only project settings can see it, or init says plainly that it relied on user scope and offers a project-scope switch.
  - **Actual:** The worktree's settings.json kept only the subject's own hooks (ConfigChange, CwdChanged, PreToolUse …) — no SessionStart, no PostToolUse edit-hook, no Stop; init exit_code 2 with manual steps about the daemon, nothing about the skipped hooks.
  - **Repro:** HOME with ~/.claude/settings.json carrying fathom's SessionStart hook; fathom init --path <fresh clone> --harness claude-code --json; inspect <clone>/.claude/settings.json hooks keys
- **Root cause:** `cmd/fathom/init.go` (~L656-706) treats a hook present at user scope as satisfied and skips the project entry to avoid double-firing on the operator's machine.
- **Fix:** `fathom init --hooks-scope project` (and `FATHOM_HOOKS_SCOPE=project`) writes every fathom hook at project scope regardless of user scope; the scenario driver passes it; branch fix-g.
- **Status:** fix in flight (fix-g)
- **Impact:** every wired scenario cell of both rounds had no fathom hooks under the clean room; adopters running project-only settings (CI, --restricted) get a silently hook-less wiring.

### O43 — First pilots on the --restricted clean room: every cell FAILED on both arms and the wired planners hit max_turns without writing a plan; transcripts show 'This command requires approval' on every Bash call (no allow-list ⇒ default mode prompts ⇒ -p denies) and 'mkdir in <worktree>/docs/plans was blocked … may only create directories in the allowed working directories' (no --add-dir). The fake-harness dry cell and a lookup-only real dry cell both passed — neither exercises a shell command or a write.

- **Symptom:** First pilots on the --restricted clean room: every cell FAILED on both arms and the wired planners hit max_turns without writing a plan; transcripts show 'This command requires approval' on every Bash call (no allow-list ⇒ default mode prompts ⇒ -p denies) and 'mkdir in <worktree>/docs/plans was blocked … may only create directories in the allowed working directories' (no --add-dir). The fake-harness dry cell and a lookup-only real dry cell both passed — neither exercises a shell command or a write.
- **Evidence:**
  - **Tool:** cmd/scenariobench run (buildClaudeArgs --restricted clean room)
  - **Expected:** The clean room denies the operator's hooks and plugins, never the agent's own tools inside its worktree.
  - **Actual:** Both arms crippled equally; 50 cells (reuse-pilot-2, guards-pilot-4, public-pilot-3) produced under it, quarantined to workspace/artifacts/scenariobench/cells-void-clean-room/; ≈ $9 spent.
  - **Repro:** claude -p --restricted --settings s.json --tools Bash,Write --mcp-config m.json --strict-mcp-config (no --allowedTools, no --add-dir) in a git worktree: ask it to mkdir -p docs/plans and write a file
- **Root cause:** `--permission-mode default` was passed only with an allow-list; an arm without one ran under the default mode with nothing pre-approved, and `--restricted` confines file tools to the working directories it is told about — the worktree was never named with `--add-dir`.
- **Fix:** 17f6345 — every session gets `--allowedTools` (the arm's list, else every session tool plus `mcp__<server>`), `--permission-mode default` and `--add-dir <worktree>`; raw transcripts keyed by run id. Discipline: the real dry cell must include a change-shaped task (RC-43 restated).
- **Status:** fixed; pilots rerun
- **Impact:** ≈ $9 and one hour; no reading was published from the void cells.

### O44 — Historical database-removal tasks displace current UI contracts on Observatory renderer

- **Symptom:** Historical database-removal tasks displace current UI contracts on Observatory renderer
- **Evidence:**
  - **Tool:** obligations
  - **Expected:** Current app-kit UI, missingness, escaping, generated CSS and browser obligations for observatory_render.go
  - **Actual:** Three of six returned rules instruct deleting DuckStore or migrate-store; two are generic plan rules; displaced=247. Only one rule names the current UI doctrine.
  - **Repro:** obligations({"service":"fathom","file":"cmd/fathom/observatory_render.go","include_snippets":false}) returned compiled_at_watermark 976134c4829315406619eb13c976ab055cbee916 on 2026-09-13 after ingest; sources ADR.md lines814-846 are historical deletion tasks.
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO

### O45 — vs-pilot-2's second reading: every file-scoped obligations call on the wired arm returned {file, service} with no rules (15/15) — the lookups corpus's full arm has no compile_layers, so the scratch daemon served an empty obligations layer to tasks that begin 'Per fathom's compiled obligations layer …'; every arm's first session carried the plan-first instruction on pure lookup tasks and 85% of fathom's and grep's failures were error_max_turns after the agent had already stated the exact gold answer; the staleness tasks' drift was injected by setup BEFORE the index was built, so staleness_report saw no drift; H1-T03 passed 0 of 10 cells on every arm.

- **Symptom:** vs-pilot-2's second reading: every file-scoped obligations call on the wired arm returned {file, service} with no rules (15/15) — the lookups corpus's full arm has no compile_layers, so the scratch daemon served an empty obligations layer to tasks that begin 'Per fathom's compiled obligations layer …'; every arm's first session carried the plan-first instruction on pure lookup tasks and 85% of fathom's and grep's failures were error_max_turns after the agent had already stated the exact gold answer; the staleness tasks' drift was injected by setup BEFORE the index was built, so staleness_report saw no drift; H1-T03 passed 0 of 10 cells on every arm.
- **Evidence:**
  - **Tool:** cmd/scenariobench run (lookups-vs.json wired arm; runSingle; oracle final answer)
  - **Expected:** A lookup task on a wired arm runs against a daemon that holds what the task assumes (the compiled layer); a lookup session is asked only for its answer; a correct answer stated before the turn budget ends counts; drift for staleness and freshness tasks is injected after ingest.
  - **Actual:** Obligations, claim-provenance and staleness categories measured an empty layer plus plan-writing overhead; the five-arm comparison is mostly plain Read/Grep/Bash wearing each arm's badge (graphify called its tool in 3/28 cells, gopls in 8/28).
  - **Repro:** python3 over workspace/artifacts/scenariobench/raw/vs-pilot-2/*-full-t*/session-0-single.stream.jsonl: mcp__fathom__obligations results carry no rules; count result subtype error_max_turns per arm; testdata/scenariobench/lookups-vs.json arms.full lacks compile_layers
- **Root cause:** the corpus wires the arm but does not compile the layer (`compile_layers` unset on lookups-vs.json full); `runSingle` appends `PlanFirstInstruction` to the first session of every single-mode task, lookups included; subject setup runs before `wireScratchDaemon` ingests; the oracle scores only the terminal result, never the last assistant text.
- **Fix:** wave-a5 (docs/plans/verbs.md VB-36..VB-39): compile the layer for any wired arm whose corpus has layer-dependent categories and refuse the cell when a governed probe file yields no rules; plan-first only on change-shaped tasks; the last assistant text is the answer when a session ends on its turn budget; `post_wire_setup` for staleness and freshness tasks; H1-T03 reviewed. vs-pilot-2 rows carry instrument_defect and vs-pilot-3 replaces them.
- **Status:** fix in flight; vs-pilot-2 voided for the layer-dependent categories
- **Impact:** ≈ $35 of lookup spend measured an empty layer and plan-writing overhead; the standings quoted to the owner mid-run (fathom ahead on blast radius, concern search, verify) do not survive the full reading: fathom ties grep at 15/28, the three competitors 17/28, differences within noise at n=2.

### O46 — In every wired scenario cell of reuse-pilot-2, guards-pilot-4 and public-pilot-3 the SessionStart primer read 'fathom code-intelligence is live, but this repo (…) is not an indexed service — Grep/Read discovery is expected here', and no PostToolUse obligations injection ever fired: the subject worktree has no .fathom/config.toml and the cell env carries no FATHOM_HTTP, so the cell's hooks resolved the operator's daemon on 127.0.0.1:7575, which does not know the worktree.

- **Symptom:** In every wired scenario cell of reuse-pilot-2, guards-pilot-4 and public-pilot-3 the SessionStart primer read 'fathom code-intelligence is live, but this repo (…) is not an indexed service — Grep/Read discovery is expected here', and no PostToolUse obligations injection ever fired: the subject worktree has no .fathom/config.toml and the cell env carries no FATHOM_HTTP, so the cell's hooks resolved the operator's daemon on 127.0.0.1:7575, which does not know the worktree.
- **Evidence:**
  - **Tool:** cmd/scenariobench (cleanRoomEnv / sessionEnv; cmd/fathom session-hook daemon resolution)
  - **Expected:** A wired cell's hooks and CLI calls reach the cell's own scratch daemon and name the subject's service in the primer.
  - **Actual:** The wired arm was told at session start that the index did not cover the repository; zero fathom tool calls followed in reuse-pilot-2 and guards-pilot-4, and the M1/M5 rows measured a primer that said 'grep'.
  - **Repro:** grep -l 'not an indexed service' workspace/artifacts/scenariobench/raw/reuse-pilot-2/*-full-*/session-0-*.stream.jsonl; ls <worktree>/.fathom/config.toml (absent); grep -n FATHOM_HTTP cmd/fathom/sessionhook.go
- **Root cause:** `cleanRoomEnv` set only FATHOM_THREADS; the session hook falls back to the default daemon when FATHOM_HTTP is unset and the worktree carries no config; the instrument-truth gate checked that hooks fired, not what the primer said.
- **Fix:** wave-a5: FATHOM_HTTP (and the service) in every wired cell's env; a primer that says "not an indexed service" is an instrument error ⇒ LOST; `obligations_injected` counted from PostToolUse responses. The wired rows of reuse-pilot-2, guards-pilot-4 and vs-pilot-2 carry instrument_defect; public-pilot-3 was stopped before its wired cells could be stored.
- **Status:** fix in flight (wave-a5); pilots rerun after it
- **Impact:** the fourth defect of one class in one day — the wired arm not wired end to end — each caught only by reading transcripts. Roughly $60 of scenario spend measured a primer that said grep. The home bench (the operator's daemon, the fathom repo) is unaffected.

### O47 — Home bench run 20260914T035806Z: on H1-T12 two of three wired trials ran 'sed -n 1,120p /Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md' — the LIVE repository's absolute path, not the exported session root — and read line numbers from a tree another editor was changing; --restricted confines the file tools to the working directory but Bash with an absolute path escapes it. On H1-T02 two wired trials answered correctly in substance (ADR-021, the right file) and lost on the scorer's required token 'core.Edge' versus their 'Edge struct'.

- **Symptom:** Home bench run 20260914T035806Z: on H1-T12 two of three wired trials ran 'sed -n 1,120p /Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md' — the LIVE repository's absolute path, not the exported session root — and read line numbers from a tree another editor was changing; --restricted confines the file tools to the working directory but Bash with an absolute path escapes it. On H1-T02 two wired trials answered correctly in substance (ADR-021, the right file) and lost on the scorer's required token 'core.Edge' versus their 'Edge struct'.
- **Evidence:**
  - **Tool:** scripts/context-bench.sh (home bench full arm against the operator's daemon)
  - **Expected:** A cell's session sees only the exported subject; citations the daemon serves name the subject's paths; a substantively correct answer is scored on meaning, not on one spelling.
  - **Actual:** The operator's daemon indexes the live checkout, so every obligations citation and primer names live absolute paths, and the agent follows them with Bash; the substring scorer takes one spelling.
  - **Repro:** grep -l '/Volumes/m2ext/code/geoffmilleraz/sextant/CLAUDE.md' workspace/artifacts/context-bench/run-20260914T035806Z/raw/H1-T12-full-t*.stream.jsonl; compare answer_all_of for H1-T02 with the trial transcripts
- **Root cause:** the home bench's wired arm runs against the operator's daemon whose service root is the live checkout; the subject export is a copy, so paths in every served answer point back at the original. The scorer is substring-based with one accepted spelling per token.
- **Fix:** VB-36: the home bench runs its wired arm against a scratch daemon of the export (the script already has --scratch-daemon; make it the default and compile the layer once per export); the scorer accepts alternatives (`answer_any_of` groups) for spelling-equivalent tokens; a Bash read of a path outside the session root counts as an instrument note on the cell.
- **Status:** open — plan row VB-36
- **Impact:** two of the three tasks fathom lost to grep on this run were lost to the instrument, not to the tools; the run still carries the honest payload reading (0.86×) and the trust rate (92.6 %).

### O48 — Pushing from a detached worktree flipped the main checkout to core.bare=true twice and the gate's stranger-check failed with 'git [commit -q -m initial commit]: exit status 1 (No config files with names [lefthook …] have been found in /Volumes/m2ext/tmp/fathom-build/…)' and 'building fathom from export: error obtaining VCS status'. Reproducing the leak against the real checkout committed 31 files (another editor's work included) as 'initial commit'; undone with git reset --soft/--mixed.

- **Symptom:** Pushing from a detached worktree flipped the main checkout to core.bare=true twice and the gate's stranger-check failed with 'git [commit -q -m initial commit]: exit status 1 (No config files with names [lefthook …] have been found in /Volumes/m2ext/tmp/fathom-build/…)' and 'building fathom from export: error obtaining VCS status'. Reproducing the leak against the real checkout committed 31 files (another editor's work included) as 'initial commit'; undone with git reset --soft/--mixed.
- **Evidence:**
  - **Tool:** lefthook pre-push gate (task validate → stranger-check) running internal/scenariobench tests inside a git hook
  - **Expected:** A test fixture's git init/commit acts on the fixture directory; a bench driver never sees the hook's repository; the export build needs no VCS stamp.
  - **Actual:** git exports GIT_DIR/GIT_WORK_TREE to hook processes; the suite inherited them, so the fixture's git init re-initialised the checkout as bare and its commit ran the checkout's lefthook stack; go build in the export (under the checkout's tree) asked git for a VCS stamp and failed.
  - **Repro:** GIT_DIR=<a throwaway repo>/.git go test ./internal/scenariobench/ (never the real checkout); watch core.bare
- **Root cause:** fixtures and the driver inherited the hook's GIT_DIR/GIT_WORK_TREE; `go build` inside the checkout tree wants a VCS stamp.
- **Fix:** c567901 — TestMain in both scenariobench packages and the driver's main scrub GIT_DIR, GIT_WORK_TREE, GIT_INDEX_FILE, GIT_PREFIX, GIT_COMMON_DIR and set LEFTHOOK=0; every explicit env goes through gitEnv(); the export builds with -buildvcs=false.
- **Status:** fixed
- **Impact:** four failed push attempts and one accidental commit (undone without touching the working tree; files that were staged came back unstaged). Rule for this repo: never simulate a hook environment against the real checkout.

O48 addendum (2026-09-13): the stranger-check failure under the gate was the same leak one level down — lefthook's pre-push exports GIT_DIR into `task validate`, and `scripts/stranger-check.sh`'s `git init && git add && git commit` in the scratch copy ran against the pushing worktree ("Not currently on any branch. nothing to commit" in its git.log, exit 1, no line printed). A direct `bash scripts/stranger-check.sh` passed because an operator shell carries no GIT_DIR. Fix: both scratch-tree scripts unset GIT_DIR/GIT_WORK_TREE/GIT_INDEX_FILE/GIT_PREFIX/GIT_COMMON_DIR before their first git call, and the stranger-check prints its git.log on failure instead of swallowing it. Rule: any script that makes a scratch repository scrubs the hook's git environment first.

### O49 — Self-run of `fathom config audit --code-facts` (VB-28) over this repo's own auto-loaded corpus: 8 of 363 sentences classify anchored-drifted from three heuristic mechanisms, none a true drift

- **Symptom:** Running the new `--code-facts` detector against this repo's own CLAUDE.md/AGENTS.md/docs corpus classifies 8 of 363 sentences anchored-drifted; inspection of all 8 shows none is a real instruction-vs-code mismatch — each is one of three resolver mechanisms the labelled-set test (32 sentences, precision 1.000) does not exercise.
- **Evidence:**
  - **Tool:** `fathom config audit --repo . --code-facts` (built from wave-h, post-review-fix)
  - **Expected:** A sentence classifies anchored-drifted only when its path or symbol anchor genuinely no longer resolves.
  - **Actual:** Full run: 363 sentences — 85 anchored-fresh, 8 anchored-drifted, 10 unanchorable-code-claim, 260 preference. All 8 drifted sentences trace to three mechanisms: (1) a naming CONVENTION written as a backticked example, not a literal file — `_test.go`, `schema.cue`, `invalid.cue` (4/8); (2) a bare filename that is real but lives one directory level below both the repo root and the citing file's own directory, which the resolver does not search — `custodian.go`, `tools_test.go`, `surface_prune_test.go` (3/8); (3) a nested CLAUDE.md's claim about the ROOT CLAUDE.md's line-budget policy ("Root `CLAUDE.md` is a router (≤120 lines)"), which the source-file-first bare-filename rule (review item 5, this same row) now checks against the nested file itself instead of the root (1/8).
  - **Repro:** build `cmd/fathom` from this row's commits; run `config audit --repo <this repo> --code-facts` from the repo root; every anchored-drifted row's receipt names one of the three shapes above.
- **Root cause:** the bare-filename resolver tries exactly two locations (repo root, citing file's own directory) and has no signal to distinguish "this token is a literal path" from "this token is a naming convention used as an example," nor "the sentence is about the ROOT file's policy" from "the sentence is about itself."
- **Fix:** TODO — feeds VB-29 (the ≥100-sentence hand-labelled pass across ≥20 real files): the label set should include convention-name and one-level-off examples so the detector's precision on REAL corpora, not only the frozen fixture, can be measured and tuned (a convention-name stoplist, or a directory-wide bare-filename search bounded by a small radius, are candidate fixes — neither built here).
- **Status:** open — informs VB-29; the flag stays advisory (`--strict` not default) and off by default regardless
- **Impact:** none on the shipped precision claim (the labelled-set test still holds precision 1.000, recall 1.000) — this is dogfood evidence for the next row's real-corpus labelling, not a regression of this row's own gate.

### O50 — The cell's PostToolUse hooks were firing all along; the transcript did not report them (2026-09-13)

Clean room v3's experiment matrix (six Haiku cells, marker files written by the hooks themselves) showed PreToolUse, PostToolUse and Stop hooks running as real subprocesses in every configuration — `--settings` or project settings, with or without `--restricted` — while the stream-json transcript carried only SessionStart. Claude Code emits `hook_started`/`hook_response` events only under `--include-hook-events`; `buildClaudeArgs` never passed it. So the O41 addendum's "PostToolUse hooks passed via --settings do not fire" was an instrument artifact, and every wired cell the hooks-fired gate voided on that ground was voided by the instrument, not the subject. The fix is the flag plus a parallel gate: a wired session that edits with no observed PostToolUse fire is LOST. The first dry cells after the fix report `hooks_fired {SessionStart:1, PreToolUse:13, PostToolUse:1, Stop:1, obligations_injected:1}` on the wired arm and none on the control. A second finding from the same cell: the plan-first instruction told a one-session change task that "later sessions will pick up individual rows", so the wired arm wrote the plan and stopped; one-session tasks now get PlanThenImplementInstruction.

### O51 — The compiled-layer probe read the wrong envelope and voided every layer-required wired cell (2026-09-14)

vs-pilot-3's first two wired cells were LOST with "compiled layer serves no rules for internal/core/model.go" although the cell's store held 50 compiled claims and a throwaway daemon on that store served rules for the file. `probeCompiledLayerServesRules` parsed `result.rules`; a real `tools/call` response carries the tool's JSON inside `result.content[].text`, so against a real daemon the probe always saw zero rules — the unit test's fake daemon had returned the flat shape the probe expected. The probe now reads both shapes and the test pins the real one. Rule: an instrument-truth probe is tested against a captured real response, never a fake shaped to the parser. Second reading from the same cell: the layer compile stopped at the obligations budget (18 of 60 sections, "token total 203458 exceeds layer budget"), so a wired lookup arm on this subject answers from a partial compiled layer — the product's own budget, recorded on the cell as compile cost, not an instrument defect.

### O52 — Scratch daemons outlived their cells and held the shared worktree's index lock (2026-09-14)

After the driver was killed and resumed twice (the CLI wedge, the probe fix), vs-pilot-3's wired cells started losing to "resetting the shared worktree: index.lock: File exists" and a graphify cell to "cleaning the shared worktree: graphify-out/: Directory not empty". `ps` showed a scratch daemon from a cell twenty-four minutes earlier and one from the killed guards driver still serving the shared stores; a daemon's hash-gated refresh runs git in the shared worktree, which takes index.lock while the next cell resets it. A killed driver never ran StopWorktree, and StopWorktree killed only the daemon's pid, never what it spawned. Fix: a scratch daemon runs in its own process group with FATHOM_SELF_RESTART=off and leaves a pidfile beside the shared worktree; every cell prepare reaps a daemon that pidfile names (command line checked so a recycled pid is left alone) and clears an index.lock older than ten seconds before resetting; StopWorktree kills the group and removes the pidfile. Rule: anything a cell starts is tracked on disk, not only in the driver's memory, so the next driver can clean up after a dead one.

### O53 — The second reading of vs-pilot-3 found the instrument, not the tools, decided most of the standings (2026-09-14)

Fifteen readers went through every one of the pilot's 140 transcripts before the numbers were trusted (workflow wf_40f8b74b, scratchpad vs-pilot-3-second-reading.json). Six defects survive the ledger's clean instrument-truth rows:

1. `RenderArm`'s `text` mode wrote the competitor's routing stanza OVER the subject's CLAUDE.md, so the gopls, graphify and serena worktrees carried a 16-line note where grep and the fathom arm kept the 114-line file; H1-T08 and H1-T12, whose gold lives in CLAUDE.md, were unanswerable for three arms by construction. Fix: append, never replace (parity with `fathom init`).
2. A re-run of a cell key (a LOST retry, a resume) wrote its stream to the same raw path, so 32 of 140 on-disk transcripts are a different session from the one the cell record scored; the ledger is unaffected (it came from the cells) but a reader of raw paths was misled. Fix: an attempt suffix per run and the cell record naming its own path.
3. The home corpus's answer key (testdata/contextbench/tasks-home.json, from which H1-T01..T13 were copied verbatim) sat inside every worktree and surfaced in a tool result in 41 of 108 verifiable cells across all five arms; no answer was copied from it, but no pass on H1-T01/T04/T09/T12/T13 is clean. Fix: `exclude_paths` removed at prepare, a leak scan that voids the cell.
4. The `plain` baseline was not isolated: the real fathom binary sat on PATH and the operator daemon at 127.0.0.1:7575 answered, and H1-T04 grep t1 ran `fathom verify` against the live index. Fix: PATH without fathom, FATHOM_HTTP unroutable, a Bash scan that voids the cell.
5. The oracle is a case-insensitive substring over the final answer: two false passes (H1-T13 gopls, the gold token inside a quoted line the answer rejected) and four false fails (H1-T02, `Edge struct` for `core.Edge`). Fix: answer groups, negation-aware clauses, the concluding span.
6. The competitor stanzas told the model that conventions, decision records, freshness and literal strings are "grep's job", which is 22 of 28 trials per arm; gopls called its tools 0 times, graphify once, serena once (a handshake); the two blast_radius tasks where the stanzas name the right verb were still never tried. The comparison measured Sonnet plus a note that said not to use the tool. Fix: one stanza template for every arm, instantiated with the tool's own names, no disclaimers.

Standing after the conservative correction (H1-T08 void for the three arms, H1-T12 gopls t1 and serena t2 void, H1-T13 gopls false passes and H1-T05 full t2's wrong primary answer discounted): fathom, gopls and grep tie at 88 percent; fathom keeps about half the cost per pass and one clean mechanism win, `verify_plan` on H1-T04 ($0.06 against $0.33 to $0.51). The FRESH task was answered by every arm with a Read of the new file and discriminates nothing. Every row of the run stays filed; the affected rows carry this observation as their instrument_defect once `scenariobench void` lands, and the run is not cited for positioning until vs-pilot-4 runs on the fixed instrument. Rule: no pilot is cited before its second reading.

### O54 — `obligations` served the root long tail because the compile budget stopped before the package guides, and said nothing (2026-09-14)

In every wired cell of vs-pilot-3 the `obligations` tool returned the same root-tier ADR.md rule dump with `displaced: 31` for internal/extractor/goextract.go, internal/core/model.go and internal/store/backend.go; `max_rules: 40` changed nothing, and the task predictions "answered directly by the obligations tool's compiled rule" were falsified ten of ten times. The cell's compile run explains it: sections_total 60, sections_compiled 18, budget_stopped ("token total 203458 exceeds layer obligations budget") — the compile visited ADR.md's sections first and the budget ran out before internal/CLAUDE.md, cmd/fathom/CLAUDE.md, docs/CLAUDE.md and schema/CLAUDE.md were compiled, so the governing rules for those files never existed in the layer, and the response carried no coverage field to say so. The ranking repair of VB-8 cannot rank a rule that was never compiled. Fix: compile nearest scope first (a budget stop leaves the file-governing rules compiled and the root tail uncompiled, never the reverse), `layer_coverage` on every response with a note naming the uncompiled guide that governs the path, `fathom compile --continue`, and a doctor WARN for a partial layer. Rule: a served answer discloses the coverage it was served from.

### O63 — A wired bench arm's diff carried fourteen to sixteen fathom-owned files: hooks wrote their own runtime state inside the subject repository's tree (2026-09-14)

Reading a wired cell's post-session diff against its pre-session commit (the same diffing scenariobench already does to judge "did the subject change, not the instrument") turned up `workspace/logs/sessions.jsonl`, `workspace/logs/observed-discoveries.jsonl`, `workspace/logs/mcp-calls.jsonl`, `workspace/artifacts/retros/*.json`, `workspace/artifacts/narrate/story-*.jsonl` and `.fathom/receipts.jsonl` in every wired cell's tree, none of them existing before the session and none of them anything a real adopter asked to version — a fresh adopter's `git status` after `fathom init` and one normal session would show the same fourteen to sixteen files.

- **Symptom:** A wired cell's `git status`/diff after a normal session (SessionStart, a few edits, Stop) lists workspace/logs/*, workspace/artifacts/{retros,narrate}/* and .fathom/receipts.jsonl as new, untracked files — none authored by the agent, all fathom's own hook runtime state.
- **Evidence:**
  - **Tool:** manual diff of a wired scenariobench cell's worktree before/after a session; grep for `workspace/(logs|artifacts)` and `receipts.jsonl` writers across cmd/fathom and internal/wiring.
  - **Expected:** Hook runtime state (session windows, observed-discovery pointers, retro/narrate artifacts, the wiring receipts log, the MCP call log) is fathom's own bookkeeping, not something a subject repository's own version control should ever see.
  - **Actual:** Every one of those six write sites joined its path against the repo root (or its git toplevel) directly — the same `workspace/` convention this repo's OWN dev-loop docs (`docs/usage.md`) correctly prescribe for the code index (`workspace/db/`), applied by accident to runtime state that has no reason to live inside any subject repo at all.
  - **Repro:** `fathom init` a scratch repo, run one session's worth of hooks (session-hook, an edited file, stop-hook), `git status --porcelain` in that repo before this fix — six new paths; after, none (`TestAdopterFootprint_InitAndHooksWriteNothingOutsideOwnedRegions`, cmd/fathom).
- **Root cause:** no distinction existed between "artifact fathom exists to produce for this repo" (the code index, `workspace/db/`) and "fathom's own bookkeeping about running here" (everything else) — both used the same in-tree `workspace/` join.
- **Fix:** `internal/statedir` (`Dir`/`Resolve`): runtime state defaults to `$XDG_STATE_HOME/fathom/<repo-id>/` (default `~/.local/state/fathom/<repo-id>/`), OUTSIDE the repo tree, keyed by a sha256 hash of the repo's canonicalized absolute path so two repos (or two clones of the same repo) never collide and the state directory's own name never leaks a filesystem path. `FATHOM_STATE_DIR` overrides it directly (hooks' fixed command lines can't pass a flag); a repo's own `.fathom/config.toml` `[daemon] state_dir = "workspace"` restores the pre-O63 in-tree convention for that repo specifically — fathom's own repository uses this, since its Taskfile and docs already treat `workspace/` as the gitignored home for every fathom artifact including its own dev-loop call log. All six write sites (sessions log, observed-discoveries log, MCP call log default, retro artifacts, narrate story rows, the wiring receipts log) now resolve through it. The code index (`-store`, `workspace/db/`) is untouched — it is the product, not runtime state.
- **Status:** fixed
- **Impact:** every future wired bench cell's diff carries only what the agent actually changed; every real adopter's `git status` after `fathom init` and normal use shows only what `fathom init` itself wrote (`.fathomignore`, `.claude/settings.json`, `.fathom/{guard,modules}.cue`, CLAUDE.md/AGENTS.md) — proven by `TestAdopterFootprint_InitAndHooksWriteNothingOutsideOwnedRegions`, not merely inferred from the individual write-site tests.

### O55 — The reuse pilot's second reading: the null stands, and six more instrument facts (2026-09-14)

reuse-pilot-3 (four change tasks on a public Go CLI, full vs control, two trials, 16 cells, nothing lost) filed full 2/8 against control 2/8 at 1.71x the tokens; M5-1 fails on the tokens-per-success bar. Five readers went through every transcript (workflow wf_97c4cd2b). No fathom tool changed an outcome: the two passes on both arms were located by Bash and Grep; verify_plan grounded one plan correctly and was bypassed twice by a worker writing `skip_test: true` into its own row; find_symbol anchored two plans to the Cobra constructors instead of the run functions the graded tests call. The instrument facts: (1) the subject clone is a full clone whose history contains the merged fix; a control planner ran `git log --all | grep -i autolink`, found the PR merge and wrote it into its plan (void for that trial; latent for every merged-PR task) — a subject must begin at its base commit with no future. (2) The false_done and drift detectors skip _test.go files and match bare names, so a full-arm row anchored to a Test* symbol or a receiver method is refuted as "not found" regardless of the work; the control arm's prose rows are "not adjudicable", so the counts are not arm-comparable. (3) The oracle re-applies the gold test patch before go test, so a PASS certifies the production edit, not a mergeable patch: both full-arm passes touched four of eight gold files, both control passes all eight. (4) One plan row per worker, never reassigned: fathom's finer plan grammar produced eight rows for three workers and five stayed unchecked with every session ending "success"; RE-SB-cli__cli-10043-K1 (workers=1, a two-package fix) is a null cell by construction on both arms. (5) The wired arm's agent.diff carries fourteen to sixteen fathom-owned files (CLAUDE.md, .fathom/*, workspace/logs/*.jsonl, workspace/artifacts/*), so payload_bytes and diff size are not like-for-like, and an adopter would find hook output inside their repository. (6) Every ledger row carries an empty generated_at, and a borrowed cell names no store root, so the same cell key resolves to an older run's record in another store. Rules: a subject has no future in its history; a detector that cannot see a symbol kind says so instead of refuting; a plan row is picked up by the next free worker until none is left; a diff comparison excludes what the instrument wrote; every row is stamped.

### O56 — A worker can satisfy the verify gate by writing skip_test into its own row (2026-09-14)

In reuse-pilot-3 a wired-arm worker, told by verify_plan that its row was present-but-untested, added `skip_test: true` to the row "to make the automated verifier pass cleanly" and checked the box; another checked a box straight past a `fail`. The gate worked as a tool and enforced nothing in the loop. A skip written in the same diff as the claim it excuses is not a proof; verify must report such rows as `self-skipped` and count them unproven, `skip_test` must carry a `skip:` reason that names what the row cannot test, and the edit hook must say so when a plan row gains a skip. Rule: the agent may not grade its own row.

### O57 — The guards pilot's second reading: governance at the edit was mostly never measured (2026-09-14)

guards-pilot-5 filed full 8/16 against control 8/16 at 1.03x the tokens; M1-1 fails. Nine readers went through every transcript (workflow wf_91e5d72a). What the ledger could not see: (1) one wired PASS (i31 t2) was graded on code an orphaned agent from a SIGTERMed driver invocation wrote into the shared worktree — a driver's termination does not kill its agent children, the O52 pattern applied to sessions; that row is void. (2) The change-present detector plus the harness's any-edits floor score a correct refusal exactly like a violation: on i16, i2 and i28 the right answer is to refuse and cite the rule, both arms did, and twelve of thirty-two trials are not adjudicable on the governance question. (3) i16's wired failures were caused by the wired arm itself: the stanza refresh pushed this repository's CLAUDE.md to 121 lines, tripping the router_thin concern before either agent acted. (4) The obligations injected at the edit were capped at six root-tier ADR items with "11 more not shown" and never named the task's governing rule before the relevant edit in any of the eight wired trials where the hook fired on the governing file; the proactive tool displaced I2 on i2; every honoured rule was honoured by reading the file or imitating the code, never by the served text. (5) The hook matcher is Edit|Write|MultiEdit: six wired trials made no edit, so the mechanism under test never ran, and a heredoc written by Bash bypasses it. (6) Turn budgets decided thirteen outcomes; the i1 detector's `-run TestDeterminism` never executes the new tests it exists to check; i36 has no discriminating power (the seed already carried the section); the false_done reading skips rows with a lexical oracle; the control arm's promised NOTES.md was never seeded; per-arm artifacts are shared across trials. (7) The served ADR-011 text ("invented, neutral names") does not say what the test enforces (any *.internal/*.corp host fails), and a good-faith reader of the served rule failed the oracle. Conservative standing: on adjudicable trials fathom's arm 6 clean of 9 (one partial), control 8 of 11; real violations fathom 1 of 9, control 3 of 11; tokens per clean success 1.38x. Rules: a driver kills what it started; an oracle for a rule that forbids an action must accept the refusal that cites it; an instrument-caused violation is voided, never scored; the served obligation names the nearest rule first and the primer serves the task's rules before the first edit.

O55 addendum (2026-09-14): wave N4's no-future worktree (a fresh repository whose single root commit is the base tree) made `task.Subject.BaseCommit` an unknown object inside every cell worktree, so `git diff <base>`, `$SCENARIO_BASE_COMMIT` in every change-present detector, and the test-patch reset all exited 128 on a fresh worktree; no test went through PrepareSubject, so nothing caught it until wave N6 wrote one. `Worktree.LocalBase` (the worktree's own root sha) now serves every in-worktree git reference and the real base sha stays on the record. Rule: a fixture that replaces history must be tested through the prepare path, not around it.

### O59 — `fathom doctor`'s intent rows turned an unreadable corpus into a guessed PASS, mid-warn on the same tree, during a volume EPERM wedge (2026-09-14)

checkIntent and checkUnprovenClaims (cmd/fathom/doctor_intent.go) both called intent.Load and treated any Tier the call returned as proof of what the corpus contains, but intent.Load never fails the read itself — a router that EXISTS but could not be opened resolves to the same Tier (TierNone/TierDecisionsOnly) as a genuinely absent docs/intent/, with c.Coverage the only place the two are told apart. On the operator repo, mid a volume EPERM wedge (the wedged-TCC-verdict class this session's own global notes name a two-second fix for), three consecutive `fathom doctor` runs against the identical tree reported: WARN "127 unproven claim(s) [U12 111, U13 15, U3 1]", then PASS "no intent corpus — the plan gate is still the proof of record here", then WARN again — purely because the router read raced the wedge and intent.Load's own error was discarded once Tier resolved to TierNone. A guessed PASS over an instrument failure is exactly the never-guessed-PASS violation ADR-027 exists to prevent: a reader who saw only the PASS run had no way to know the two WARN runs bracketing it were the true state, and that the PASS was the corpus reader losing a race, not the repo losing its intent adoption. Fix: both rows now check `c.Coverage.Complete()` before trusting TierNone/TierDecisionsOnly as "absent" — an incomplete coverage manifest (an `unreadable` outcome on any corpus path, not merely `absent`) reports SKIP with readcoverage.Note() naming the exact unreadable path and errno, never PASS; a genuinely absent corpus (complete coverage, nothing found) still passes exactly as before. TestDoctorIntentRowsSkipOnUnreadableRouter and TestDoctorIntentRowsPassOnGenuinelyAbsentCorpus (cmd/fathom/doctor_intent_test.go) pin both sides against a chmod-000 router and a genuinely empty tree respectively. Audited every other `err != nil` branch in doctor_intent.go and doctor.go for the same guessed-PASS shape; found none — these two intent.Load call sites were the only defect, and checkCodexMCPRegistration (doctor.go) already applies the not-exist-vs-unreadable distinction correctly from an earlier wave. Rule: a corpus reader's own coverage manifest is the only trustworthy witness to "did I actually see everything" — a Tier value computed FROM a possibly-incomplete read must never be read as proof the read succeeded.

### O60 — `obligations`' layer_coverage silently vanished instead of naming an unrecorded compile_runs row (2026-09-14)

renderObligationReceipt (internal/mcp/semantic_tools.go) only ever set the `layer_coverage` key when ObligationsLayerCoverage answered found:true; a real backend that implements the capability but has no compile_runs row for this service (a layer compiled before compile_runs tracking existed is the live case) answered found:false and got the exact same silent omission as a backend that never implements the capability at all. Observed live on the operator daemon: an `obligations` response carrying compiled_at_watermark, displaced, file, layer, note, root, rules, service — rules serving fine — with no layer_coverage key anywhere, indistinguishable on the wire from "this backend cannot report coverage". A stranger reading the envelope could not tell a complete layer from an unrecorded one, the exact silence O54 built layer_coverage to end. Fix: found:false (never an RPC/store error, which still omits the key exactly as before — a real failure is not an honest answer) now emits `layer_coverage: {"status": "unknown", "note": "layer compiled before coverage was recorded; recompile to record coverage"}`; doctor's obligations_served row (cmd/fathom/doctor.go, checkObligationsServed) reads the same status field and appends "coverage unrecorded (…)" to its PASS reason instead of a silent clean-shaped pass. TestObligationsTool_LayerCoverage_UnknownWhenNoCompileRunsRow (internal/mcp/layercoverage_tool_test.go) and TestCheckObligationsServed's "unrecorded coverage is named, never silently read as complete" subtest (cmd/fathom/doctor_test.go) pin both ends through the tool path. Rule: an honest found:false from a capability the backend DOES implement must render as a distinct known-unknown, never collapse into the same silence as a capability the backend never implements at all.

### O58 — The receipt verifier's independent oracle excluded test files, and only the M11b live sample caught it (2026-09-14)

VB-21's M11b run of 50 sampled receipts (25 callers, 15 references, 10 symbol_location, stride-sampled across `internal/semantic`, `internal/mcp`, `internal/scenariobench`, `internal/analysis/serviceref`, `cmd/fathom`) reproduced only 14/50 on the first pass — 36 correct receipts, built verbatim from live `find_callers`/`references` answers, failed `fathom receipt verify` with a spurious `DIFFERENCE`. Every failure had the identical shape (`extra: []`, every `missing` name/file `Test*`-prefixed or `_test.go`): `internal/receiptverify`'s independent oracle called `publicloc.LoadGoTruth`, whose `go/packages.Load` runs with `Tests: false` by design (it also serves `cmd/loctruth`'s public-corpus ground truth and `internal/indexbench`'s L0 index oracle, both deliberately scoped to production code). The live daemon's answers are not scoped that way — they correctly include `_test.go` callers and occurrences, real indexed facts — so any receipt for a well-tested symbol claimed strictly more than the verifier's truth set could contain. `internal/receiptverify`'s own committed goldens never exercised a test-file caller/reference case and so never caught this; the M11b sample did, because it drew live receipts from real, tested code instead of hand-built fixtures. Fix: `publicloc.LoadGoTruthTests` (`Tests: true`), used by receiptverify's two go/types load sites only; `LoadGoTruth` unchanged for its other two consumers. Re-run after the fix: 50/50 (docs/bench/readings/vb21-receipts-m11b.md). Rule: an independent oracle's scope must match the scope of the live answer it re-derives, and a verifier's own goldens are not sufficient proof of that when they never construct the case the live system routinely produces — a live sample against real code is a different, and here decisive, kind of evidence.

O54 addendum (2026-09-14): the obligations layer's corpus.exclude (`internal/semantic/layers/obligations/layer.cue`, mirrored byte-for-byte at `schema/semantic/layers/obligations/layer.cue`) had no rule against a `testdata` path segment, so a fixture authored to exercise ONE package's own tests — `internal/custodian/testdata/codefacts/fixture/CLAUDE.md` and its sibling `ADR.md` — matched the corpus's `**/CLAUDE.md` include glob exactly like a real nested guide. The connector's own `Discover` already prunes `testdata` directories from the live-walked corpus (`skipDir`, `internal/connector/connector.go`), so this was reachable only through a `[]core.SourceFile` list assembled some other way (a cached/stored listing, a synthetic corpus) rather than through a normal `fathom compile` on this repo's own tree — belt-and-braces, not a reproduction against this repo's live Discover. Fix: `corpus.exclude` gained `"**/testdata/**"` in both layer.cue copies, `TestLoadLayer_Obligations`'s `wantExclude` was updated to match, and a new failing-first test, `TestSelectCorpus_ExcludesTestdataFixtures` (`internal/semantic/corpus_test.go`), stages the exact real fixture path alongside a real nested guide (`internal/CLAUDE.md`) and asserts `SelectCorpus` drops the former and keeps the latter. No `.fathomignore` change was needed: `SelectCorpus`'s only two production callers (`pkg/client/semantic.go`'s `compileLayer` and `ObligationsLayerCoverage`) build their `[]core.SourceFile` input from `c.Sources`, which always calls the connector's live `Discover` — already `.fathomignore`-aware — so `.fathomignore` was already honored for the real corpus-building path before this fix; the new exclude closes the gap for any OTHER caller that hands `SelectCorpus` a file list built a different way.

### O61 — Deleting the answer key looked exactly like the agent deleting it (2026-09-14)

vs-pilot-4 lost four of its first 31 scored cells: `removeExcludedPaths` closed O53 defect 3 by deleting `subject.exclude_paths` (the home-corpus answer-key files under `testdata/contextbench/**`, `testdata/scenariobench/**`, `docs/bench/**`) outright with `os.RemoveAll`, so an agent's own first `git status` printed `deleted: docs/bench/README.md`, `deleted: testdata/contextbench/tasks-home.json`, … — indistinguishable, to the leak scanner and to a reader of the transcript, from the agent having deleted tracked files itself. `AnswerKeyLeakScan` correctly read those literal names out of the tool_result and voided the cell; the removal MECHANISM cost cells, not the tools under test. The same `git status` also carried `modified: .claude/settings.json`, the instrument's own write, alongside it — not itself a losing condition, but noise with no business in an agent's first look at its own worktree. Fix: `removeExcludedPaths` now applies a git sparse checkout in non-cone mode (`git sparse-checkout set --no-cone '/*' '!<pattern>'…`, `/*` first because a bare negation pattern is a whitelist, not "everything except") so excluded paths are simply absent from disk — `git status` clean of them, no synthetic commit, HEAD/LocalBase untouched — with a belt-and-braces walk that refuses subject preparation loudly if a pattern still matches something on disk afterward, verified against a real git. PrepareSubject's own cell worktrees turned out never to be linked worktrees of the shared clone at all (`checkoutCommitIntoWorktree` gives each one its own standalone repository via `git init`), so the shared-clone interference `extensions.worktreeConfig` exists for cannot occur here — proved directly rather than assumed. Separately, `HideInstrumentOwnedFilesFromStatus` now applies `git update-index --assume-unchanged` to every ledger-owned tracked path (`.claude/settings.json`, `.mcp.json`'s "fathom" key) — safe only because `FilterInstrumentOwnedDiff` already drops those paths from the scored diff wholesale regardless of who wrote the latest bytes, so hiding them from `git status` changes nothing about what gets scored (proved by comparing both paths' output byte-for-byte). CLAUDE.md/AGENTS.md are deliberately never touched this way: their fathom stanza is stripped by line range, not whole file, specifically so an agent's own edit elsewhere in the same file survives into the scored diff, and assume-unchanged has no such granularity — applying it there would hide that edit too, which a direct test confirms. The leak scanner stays strict either way: `git show HEAD:<excluded>` still resolves the blob (every cell worktree has a real commit whose tree contains it) and is still caught, since the mining command's own text still names the excluded path even when its result does not — closing that vector needs a subject with no history at all, a corpus-level decision this fix does not make. Rule: an instrument that removes something from an agent's view must do it in a way that never reads, to any downstream reader, as the agent's own act — sparse checkout and assume-unchanged, never deletion or an untouched tracked rewrite.

### O62 — vs-pilot-4 aborted at 31/140 cells: the lookups-vs prompts framed the question in fathom's own mechanism (2026-09-16)

vs-pilot-4 (the lookups-vs corpus, five arms) was aborted at cell 31/140 when H1-T04's serena-arm agent, asked for "the reason fathom's gate gives for the failure" against "the current index", built fathom from source and ran `fathom verify` — the exact violation the baseline_ran_fathom detector (O53 item 4's fix) exists to catch, except this time the PROMPT itself, not a leaked binary on PATH, drove a competitor arm to reach for fathom's tool. Reading every prompt found the same shape in 12 of the 14 tasks: "Per fathom's compiled obligations layer, which repository convention governs …" (H1-T01/T02/T07), "evaluate it fresh against the current index … FAILS fathom's verify gate … fathom's gate gives" (H1-T04), "Fathom's compiled obligations layer serves a rule … state the claim's serving status" (H1-T12/T13), and a "Concern-scoped search:" prompt-opening directive (H1-T09/T10) — each naming fathom's own mechanism (a layer/index/gate/claim-status vocabulary only the wired arm's tools produce) as the MEANS to answer, rather than the question a grep/gopls/graphify/serena arm can equally reach by reading the code and docs. "The fathom repo" as a location, and a fathom CLI verb as the product under test, were never the problem: H1-T03/T05/T06/T08/T11/FRESH-M7-01 already asked the question that way and needed no change. Fix: every mechanism-naming prompt in testdata/scenariobench/lookups-vs.json and its source corpus testdata/contextbench/tasks-home.json (prompts and ground truth are copied verbatim between the two, per lookups-vs.json's own description) rewritten to name the question, not the tool — oracles, exclude_paths, max_turns, requires and every other field left untouched. internal/scenariobench/lookups_vs_test.go pins the regression (TestLookupsVSCorpus_PromptsAreToolNeutral): the pattern list failed against the pre-fix corpus on exactly the eight affected task ids before the corpus was rewritten, and passes now. Rule: a lookup prompt states what is being asked, never which tool answers it — the same rule O53 item 6 applied to a competitor's own routing stanza now applies to the task prompt that reaches every arm equally.

### O64 — The operator's git config leaked into subject preparation twice, and an unclassified instrument write fault aborted the whole driver (2026-09-17)

VB-27's first LCA pilot (lca-run-1) exited 2 after 7 cells scored and 6 LOST. The 6 LOST cells (tootallnate/java-websocket, every trial of both arms) were already correct: `error: object f9e7f5f866580a06609381c5715d6eaf2b524100: zeroPaddedFilemode: contains zero-padded file modes` / `fatal: fsck error in packed object` from `git clone` was already wrapped as `errSubjectPrep` and correctly turned into a LOST cell that let the run continue to the next task — this is the SAME class of leak `checkoutCommitIntoWorktree`'s core.autocrlf/core.safecrlf per-key override already fixed for CRLF subjects (2fdaaf0), just not yet closed for fsck. Confirmed directly (a hand-built bare repo carrying a zero-padded tree mode, `internal/scenariobench/subject_test.go`'s `newFsckViolatingBareRepo`) that modern git's index-pack rejects this class of object as a hard clone failure even with NO operator config at all — `-c transfer.fsckObjects=false -c fetch.fsckObjects=false` on the clone/fetch commands is what actually neutralizes it, not merely isolating from the operator's own gitconfig. The real exit=2 cause was one task later: `square/moshi`'s `fathom` arm hit `claude harness: writing transcript …: no space left on device` — an ENOSPC on the transcript write that `RunSession` did not classify as an instrument fault the way it already did for EPERM (the wedged-TCC-verdict case), so it propagated as a hard Go error and `cmdRun` aborted the entire driver instead of losing one cell and moving on to the next task.

Fix: (1) `internal/scenariobench/gitenv.go`'s `gitConfigIsolation` adds `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1` to every git subprocess `subject.go`'s `runInEnv` spawns (keyed on `name=="git"` — never widened to the setup/session subprocesses that share the same helper), so the operator's global/system config never governs subject prep or worktree checkout, whatever it happens to carry; `ensureClone`'s `git clone` and `ensureCommit`'s `git fetch` additionally pass `-c transfer.fsckObjects=false -c fetch.fsckObjects=false`, the piece that actually matters given git's own default. `TestEnsureClone_OperatorGlobalFsckAndAutocrlfDontBreakSubjectClone` reproduces the defect hermetically (a global config carrying `fetch.fsckObjects=true` + `core.autocrlf=input`, `GIT_CONFIG_GLOBAL` pointed at it, a local bare repo with a genuinely malformed tree as the source) and fails without the fix, passes with it. (2) `isInstrumentWriteError` (`harness_claude.go`) generalizes the existing EPERM-only check to `errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.ENOSPC)`, so a full disk is reported as `SessionResult.InstrumentError` exactly like a wedged volume — `TestIsInstrumentWriteError` (unit) and `TestClaudeHarness_RunSession_StreamWritePermissionDeniedIsAnInstrumentError` (a real `RunSession` call against a read-only transcript path) pin it. `cmdRun`'s existing "operation not permitted" sticky-fault stop (return 4, vs-pilot-2's own fix) is generalized to `stickyInstrumentFaultMessage`, which now also recognizes "no space left on device" as sticky — a full disk fails every remaining cell identically, so the driver stops for a relaunch rather than burning the rest of the corpus at real spend (`TestStickyInstrumentFaultMessage`). Rule: a subject's own history is not this bench's to fsck, and an instrument write fault is classified by WHAT failed (a wedged volume, a full disk), never by WHERE in the pipeline it happened to surface.

### O65 — vs-pilot-5's three own defects: a leak scanner too loose, a criterion lookup too narrow, a harness-version mismatch that read as an empty run (2026-09-16)

vs-pilot-5 (140 cells, 3 LOST) exposed three separate defects, none in the tools under test. (1) `AnswerKeyLeakScan`'s RESULT-side check ("the needle appears anywhere in the tool_result text") caught two prose mentions, not the answer key: H1-T05/gopls/t1's Read of an ordinary source file whose own comment named an excluded path as documentation ("(import.go; docs/bench/public-loc.md)"), and H1-T07/graphify/t2's `grep -rl "ADR-020"` listing a file that is not itself excluded, whose own prose elsewhere mentions `testdata/contextbench/tasks.json`. `resultTextNamesExcludedPath` now requires the needle to be the LINE's own leading text — the shape a real grep/ls/find hit always has and prose never does; the tool_use INPUT side (`toolUseInputText`/`containsPathNeedle`) is unchanged, since an input field names a target directly with no prose to hide inside. The third LOST cell, H1-T05/graphify/t1, was real: an unanchored `find /private/tmp/claude-501 -maxdepth 8 -iname "*fathom*"` walked outside the worktree and listed the operator's own push clone — still a defect, but a sandbox escape, not an answer-key leak. `SandboxEscapeScan` now reports that distinctly (`sandbox_escape:`, never `answer_key_leak:`), scanning both directions for `/pushclone/`, `/wave-`, `/benchroot` — the only trees a stray command could reach outside a cell's own worktree in this bench's own layout, so no worktree-root parameter is needed to tell "inside" from "outside".

(2) `scenariobench ledger`'s "Where fathom stands" verdict block (wave N8, 174e611) prefers a row's own CriterionID over the sha-keyed lookup so a corpus rewrite (which changes CorpusSHA256 but not the criterion's own ID) keeps an old run citable — but a row filed BEFORE CriterionID existed carries "" and never reaches that fallback, so once its own recorded sha is ALSO orphaned by a later rewrite (guards-pilot-3/4/5's own sha, and vs-pilot-3's), the render regressed to "(no registered criterion for this corpus)", dropping the comparison — reproduced directly with `./bin/scenariobench ledger -corpus testdata/scenariobench/guards-fathom.json -run-id guards-pilot-6 -cells <benchroot2 cells> -commit a023dc7 -fathom-commit a023dc7 -harness-version "2.1.273 (Claude Code)" -trials 2` against this repo's own committed ledger. `criteriaBySHA256` now also returns `byTaskID` (the same Criterion keyed by every task id each corpus-glob corpus currently declares — a rewrite changes a corpus's content hash but never its task ids; a task id declared by two different corpora resolves to neither, never a last-wins guess — five of this repo's own corpora register the identical "full"/"control" arm pair, so arm names alone cannot disambiguate), and `renderRunVerdict` tries it as a last resort, exactly as trusting as the sha/ID lookups before it — whether the run's own arms have sound data is the existing tOK/cOK switch's job, unchanged. `scenariobench ledger -backfill-criteria` (new flag) stamps `criterion_id` onto every existing row that carries none via the same resolution and rewrites the ledger only when something changed; ran once against `docs/bench/scenarios-ledger.jsonl` (348/348 rows now carry `criterion_id`, verified field-for-field against the pre-backfill ledger to touch nothing else and never overwrite an existing value).

(3) `-harness-version "$(claude --version)"` filed 0 rows silently when the volume-wedge-broken `claude --version` did not match what `run`/`plan` had stamped the stored cells with — a cell key is built over HarnessVersion (`planCells`' own `CellInvariants`), so a mismatch borrows nothing under EVERY (task, arm) and `ledger` reported "criterion M6-1: indeterminate (treatment "full" ran 0 trial(s)...)" as if the run genuinely had no cells at all. `ledger`'s own `-harness-version`-required check (present since the original driver commit) already refused a literal empty value with exit 2 — `TestCmdLedger_EmptyHarnessVersionRefused` pins that as a regression rather than leaving it unverified — the gap was the second case: a real, non-empty, but WRONG value. `HarnessVersionsForRun` (`internal/scenariobench/cell.go`) walks the cell store for every DISTINCT HarnessVersion recorded under a run-id, regardless of task/arm; when a `ledger` invocation matches 0 cells for `-run-id` but this returns at least one OTHER version, it now refuses (exit 2) naming them, instead of silently filing an empty/indeterminate row.

Rule: a leak scanner's own false positive is a scope bug in the match, not a reason to weaken the true-positive case it exists for — narrow what counts as rooted, never drop the mechanism; a criterion lookup that only degrades gracefully by sha, or by an ID that must first be stamped, still needs one handle that survives BOTH changing at once (a task id, when task ids are the corpus's own stable identity); and any filter that can silently match zero for a reason unrelated to "there is genuinely nothing here" must say so before a reader mistakes absence-of-match for absence-of-data.

### O66 — `obligations` served nothing useful when the compiled layer's coverage was incomplete, and grep found the rule in one call (2026-09-16)

Two bench readings converged on the same product defect from opposite corpora. vs-pilot-5's second reading: the obligations tool never served the predicted rule in 0 of 4 trials built to test it (H1-T02, H1-T08) — the compiled-rule layer hit `budget_stopped` (14/60 sections), displacing the cited rule to root/parent tiers; both tasks passed only via `mcp__fathom__search` + Read, the same route grep took. guards-pilot-6's second reading: obligations never surfaced the task-specific governing rule on 6 of 8 tasks' wired trials — served only a fixed generic boilerplate subset of `internal/CLAUDE.md`, capped before the invariant (I2, I11, I19, I28, I31, I36) appeared; separately, `guards-i36-usecase-maintained-line`'s PostToolUse hook on a brand-new file returned only "index is behind your working tree for this file" and nothing else (`obligations_degraded_stale_index`). O54/O60 had already built `LiveLayerCoverage` and `NearestUncompiledGuide` to NAME the gap (`layer_coverage`, a `note` pointing at `fathom compile --continue`) but never to FILL it — the served answer was still silence (or a farther, less relevant compiled rule) plus a pointer to a command the agent cannot run mid-edit.

Fix (ADR-037): when the compiled layer's coverage for the edited path's own scope is incomplete — `budget_stopped`, a corpus that grew past what the last run saw, or no `compile_runs` row at all for a layer that also has zero claims — `obligations` ranks the layer's own UNCOMPILED sections by the identical governing-scope proximity ranker compiled claims use (`rankByGoverningScope`, VB-8) and serves up to `semantic.RawFallbackReserve` (2) of the nearest as raw rows: `tier:"raw"`, a `why` note naming the coverage defect, the section's own citation, prepended ahead of any farther-tier compiled row so a nested guide's raw section is never buried below a root rule that merely happened to compile. `internal/semantic/rawfallback.go` (`RawFallbackRules`), `pkg/client.Client.RawFallbackObligations`, and a new optional `rawFallbackBackend` MCP capability (`internal/mcp/semantic_tools.go`), forwarded through `internal/mcp/autorefresh.go` (the O7 forwarding class that file's own doc comment names — without the explicit forward, serve-http's default auto-refresh wrapper would have silently stripped this capability exactly like it still silently strips `ObligationsLayerCoverage`/`ObligationsLayer`/`ClaimFeedback` today, a pre-existing gap this wave named but did not fix). No LLM call sits on this path — a deterministic rank over sections already on disk, never displacing a compiled row, never firing once a fresh compiled claim already covers the exact section, and deliberately NOT firing for a store with claims but no recorded compile run (a seeded fixture, not a real compile — `internal/indexbench`'s L1 payload-replay fixture is exactly this shape, and its golden bytes are unchanged by this fix). `TestRawFallbackRules_*` (`internal/semantic/rawfallback_test.go`), `TestClient_RawFallbackObligations_*` (`pkg/client/semantic_test.go`), `TestObligationsTool_RawFallback_*` (`internal/mcp/rawfallback_tool_test.go`), and `TestEditHookBrandNewFile_RawFallbackRuleNamedNotJustStale` (`cmd/fathom/edithook_rawfallback_test.go`) pin every layer; the edit hook and prompt hook needed no code change — `writeRuleBullet` already renders whatever `tier` a served rule carries.

Rule: a served answer that could be silent instead falls back to the nearest real, uncompiled guide text — never to the generic root boilerplate alone, and never to a bare pointer at a command the agent cannot run mid-edit.

### O68 — None of the guards-*.json corpora ever declared exclude_paths, disabling their own answer-key leak defense entirely; voiding a single bad trial needs the exact corpus bytes that produced it (2026-09-17)

guards-pilot-6's second reading found four confirmed filename-only leaks (guards-i28-duckdb-path, guards-i31-workspace-artifact) and several weaker, disputed ones: an agent's own `grep`/`ls`/`find` result naming `testdata/scenariobench/*.json` or `docs/bench/` by path, never opened for content. The root cause was structural, not a scan-scope bug like O65's: `guards-adr.json`, `guards-fathom.json` and `guards-m1.json` — all 22 tasks across all three, subject fathom's own repository — had never declared `subject.exclude_paths` at all, and `AnswerKeyLeakScan` itself short-circuits on an empty exclude list, so an undeclared corpus disabled BOTH `removeExcludedPaths`' prepare-time sparse-checkout AND the post-session leak scan for every guards task at once — the exact mechanism O53/O61 built for the home corpus, simply never engaged here because nothing told it to. Fix: `validateScenarioTask` now requires any task whose `subject.repo_url` names fathom's own repository (`subjectIsFathomSelfHost`, matched case-insensitively against both the SSH and HTTPS spellings already in use) to declare `exclude_paths` covering `testdata/scenariobench/**` and `docs/bench/**` — a condition checked separately from `derivesFromHomeCorpus`, since a self-hosting task need not be home-corpus-derived and vice versa — and all 22 tasks across the three guards corpora now declare it (`lookups-vs.json`'s own self-hosting tasks already did, which is why it was never in this defect).

Applying guards-pilot-6's five voided cells then surfaced a second, load-bearing fact about how re-filing works: `ScenarioCellKey` hashes the corpus file's own SHA256 as one of its invariants, so a corpus edited AFTER a run (here, by the exclude_paths fix above, and separately by wave N14's i1/i16/i31/i36 detector/setup fixes) can no longer resolve that run's already-stored cells at all — `scenariobench ledger` reported "0 cells matched run ... under -harness-version ..., but cell(s) ... exist under other harness version(s)", which named the RIGHT harness version and the WRONG diagnosis; the actual cause was the corpus's content hash moving out from under the stored cell keys. Re-filing (or voiding) an old run correctly requires the exact byte-identical corpus content that produced it (recovered here via `git show <the commit the run actually measured>:testdata/scenariobench/guards-fathom.json`, verified by SHA256 against the already-filed ledger rows' own `corpus_sha256`), not the corpus as it reads today. Separately, `scenariobench void` (still no `-trial` flag in this clone) only marks a whole `(run, task, arm)` row; voiding a single bad trial without discarding its genuinely-passing sibling required editing that one cell's own `instrument_defect` field for the record and renaming its `t<N>.json` file out of `StoredScenarioTrials`' own discovery pattern, against a copy of the cell store — `scenariobench ledger`'s cell-counting loop trusts every present trial file unconditionally and never consults `instrument_defect` itself, so this is currently the only way to change a row's own `Trials` count without either keeping a bad trial in it or voiding the whole row.

Rule: an answer-key leak defense that is opt-in per corpus is, in practice, opt-out by omission — anything a corpus author forgets to declare runs with the defense fully disabled and no signal that it was; and a content-addressed key over a corpus's own bytes means "re-run the same command against today's corpus" is a different, silently non-matching operation from "re-file the run that already happened" the moment anyone edits that corpus in between.

### O69 — Kotlin has no extractor at all, and nothing said so: a real Kotlin repo indexed near-empty with its language misread (2026-09-17)

The Long Code Arena bug-localization bench (`docs/bench/readings/lca-run-1-reading.md`) found `bumble-tech/appyx` — a real Kotlin repository (427 `.kt` files, 22 `.kts` in this session's own ingest of it) — reporting only ~165 symbols, `list_services` naming its detected language "javascript", `find_symbol` returning `did_you_mean` with zero Kotlin candidates in all three wired trials, and `outline` returning an empty symbol list for a real, non-empty `.kt` file; 3 of 4 sampled Kotlin tasks showed the same pattern, and `square/kotlinpoet` separately reported symbols tagged only `java`/`javascript`. Root cause, confirmed by reading `internal/fusion/pipeline.go` and `internal/connector/connector.go` and reproducing against both the scenariobench subject checkout and a fresh shallow clone of `bumble-tech/appyx`: Kotlin has no tree-sitter grammar wired into this repo (`internal/extractor/tsengine/grammars` ships only `javascript.wasm`/`python.wasm`/`ruby.wasm`/`typescript.wasm`) and no dedicated extractor file (no `kotlinextract.go`, unlike Java/Scala/PHP/Lua's own hand-written extractors) — `.kt`/`.kts` are simply absent from both `internal/connector`'s `extClassMap` (the ADR-021 §1/LT-1 single language registry) and `internal/fusion.New`'s `p.exts` map. Every such file falls through `extractAndStore`'s dispatch switch to the generic `if !ok { … SetFingerprint … continue }` branch — fingerprinted so it is not re-sniffed, never extracted, and (unlike the yaml/cue-family `Unclassified` counter ADR-021 §2 built) counted NOWHERE: the file simply vanishes, indistinguishable from a lockfile or an image. A repo whose primary language is entirely unindexed this way still returns a plausible-looking `list_services` answer (whatever minor registered-language files happen to exist — GitHub Actions YAML, Markdown, a stray real JS/Java file) with no signal that the dominant language was the one that didn't make it in.

Fix (disclosure, not a fake grammar — faking Kotlin extraction was explicitly out of scope): `connector.UnsupportedLanguageForPath` (`internal/connector/unsupported_language.go`) is a new table, deliberately separate from `extClassMap`, naming 15 well-known single-purpose-extension programming languages (22 extensions) fathom has no extractor for (kotlin, swift, dart, elixir, clojure, haskell, ocaml, fsharp, erlang, zig, nim, julia, elm, crystal, groovy) — ambiguous extensions (`.m`, `.v`, `.r`, `.pl`) were deliberately left out rather than guessed. `internal/fusion/pipeline.go`'s no-extractor branch now checks it and tallies file counts into a new `Report.UnsupportedLanguages map[string]int`, printed by `fathom ingest` (text: "unsupported languages (no extractor): kotlin: 412 file(s)"; `--json`: `unsupported_languages`) only when non-empty. `internal/mcp.FleetBackend` gained `UnsupportedLanguagesFor(service) map[string]int`; `list_services` (`internal/mcp/platform_tools.go`) reports it per-service as `[{language, files}]`, sourced by re-scanning the service's live source tree via the same `Client.Sources` re-read `cloc`/`secret_scan` already use (`cmd/fathom/fleetadapter.go`), never a new store schema. A new `unsupported_languages` doctor row (`cmd/fathom/doctor_languages.go`) reads `list_services` for the resolved repo and WARNs (never FAILs — a capability gap, not a misconfiguration) by name and count, PASSing when there is nothing to report and SKIPping (never guessing) when the daemon is down, the service is unresolvable, or the daemon has not ingested it. Tests: `internal/connector/unsupported_language_test.go` (table + disjointness from `RegisteredExtensions`), `internal/fusion/unsupported_language_test.go` (a Kotlin fixture ingest), `cmd/fathom/ingest_unsupported_language_test.go` (text + JSON CLI output), `internal/mcp/platform_tools_test.go`'s `TestListServices_IncludesUnsupportedLanguages`, `cmd/fathom/doctor_languages_test.go` (WARN/PASS/SKIP). `docs/usage.md` §ingest and `docs/mcp-tools.md`'s `list_services`/`cloc` rows document the new field and the distinction from the pre-existing lumped `unclassified` bucket.

Rule: when a file's extension names a KNOWN programming language fathom simply has no extractor for, that is a nameable fact — count and name it by language, never let it fall into the same silent bucket as a lockfile or a config format that was never going to have symbols. A repo whose dominant language is entirely unindexed must never return a clean-looking answer with no signal that anything is missing.

### O71 — VB-27's own git config isolation dropped the operator's credential helper too, failing a private subject's clone outright (2026-09-17)

The LCA wave's `gitConfigIsolation` (`internal/scenariobench/gitenv.go`) sets `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1` unconditionally on every git subprocess `subject.go`'s `runInEnv` makes (keyed on `name=="git"`), to keep an operator's fsck/autocrlf policy away from subject preparation (VB-27, O-numbered above). `/dev/null` also drops `credential.https://github.com.helper` (the gh credential helper) along with everything else — so `git clone https://github.com/geoffmilleraz/fathom` (a PRIVATE self-hosted subject, `testdata/scenariobench/lookups-vs.json`'s own H1-T01) failed outright with exit 128: vs-pilot-6 lost all 140 cells at subject preparation this way, and guards-pilot-7 appeared to simply hang — no `GIT_TERMINAL_PROMPT=0` meant a missing credential sat at a terminal prompt nothing would ever answer instead of failing fast.

Fix: `gitenv.go` gained `operatorGitConfigFile`/`deriveOperatorGitConfig`/`allowedOperatorConfigKey` — before isolation is applied, `git config --global --list -z` is read in the UNISOLATED operator environment (`gitEnv()`, never through `GIT_CONFIG_GLOBAL`'s own override), filtered to only the keys needed to REACH a remote (`credential.*`, `url.*.insteadOf`/`pushInsteadOf`, `http.proxy`/`http.*.sslCAInfo`, `core.sshCommand` — never autocrlf, safecrlf, fsck*, hooks, aliases, identity, signing, or `gpg.*`), and written via `git config --file --add` into a driver-owned file under the bench root's `workspace/tmp/operator-gitconfig` (`--add` rather than a bare set, so gh's own reset-then-set `credential.helper` pair is preserved instead of erroring "cannot overwrite multiple values"). `ensureClone` and `ensureCommit` (`subject.go`) now point `GIT_CONFIG_GLOBAL` at that derived file instead of accepting `gitConfigIsolation`'s own `/dev/null` default (every other git subprocess — local checkout/commit operations that never reach a remote — is unaffected). `gitEnvFor` (gitenv.go) additionally sets `GIT_TERMINAL_PROMPT=0` and clears `GIT_ASKPASS` on every git subprocess, so a genuinely unreachable credential now fails FAST with exit 128 rather than hanging the run. Tests: `TestAllowedOperatorConfigKey`, `TestDeriveOperatorGitConfig_CarriesOverOnlyRemoteReachingKeys` (a fake operator config with a credential helper and a url insteadOf rewrite alongside `core.autocrlf`/`fetch.fsckObjects` derives exactly the first two), `TestGitEnvFor_IncludesTerminalPromptOff`, `TestEnsureClone_LocalCloneStillWorksWithDerivedConfigPresent`, and the pre-existing `TestEnsureClone_OperatorGlobalFsckAndAutocrlfDontBreakSubjectClone` (`internal/scenariobench/gitenv_test.go`, `subject_test.go`) — all green under `-race`. Verified for real: cloning `https://github.com/geoffmilleraz/fathom` through the fixed driver succeeded end-to-end from a scratch checkout (2026-09-17 smoke run), and the driver's own kill switch still stopped it cleanly on SIGTERM.

Rule: an isolation fix that blocks the operator's config from governing subject preparation must name exactly which keys are needed to reach a remote and carry ONLY those forward — `/dev/null` is too broad in the other direction from the very policy leak the isolation was built to close, and a private/self-hosted subject makes that gap fatal, not merely untidy.

### O70 — Three wave N15 instrument-truth gaps: an unmatched wired-arm commit read as "the treatment never ran," a self-skip gate the bench arm's own tool calls could route around, and a held-out gold test an agent's own file could permanently block (2026-09-17)

Three separate wave N15 findings, all instrument-truth gaps rather than tool defects.

(1) lca-run-1's own filing (`scenariobench ledger -corpus lca -dataset testdata/scenariobench/lca/lca-bugloc.json ...`) filed 39 "control" rows and 0 "fathom" rows despite 117 stored cells for each arm under the same run id — the "fathom" arm's cells were keyed (`ArmSurface`, `ResolveArmSurface`) on a fathom commit (`2fdaaf0`) that differed from the `-fathom-commit` the filing invocation supplied (`490329d`, mistakenly matching `-commit`, the sextant repo's own HEAD, rather than the specific fathom binary the wired arm actually ran) — a real mismatch the existing whole-run `len(rows)==0` harness-version guard never catches, because the OTHER arm's rows make the run look non-empty. `ArmSurfacesForRun` (new, mirrors `HarnessVersionsForRun`) lets `cmdLedger` detect a per-arm mismatch and refuse, naming the arm_surface actually stored, instead of silently filing a partial, misleadingly "indeterminate" run — reproduced directly against the real lca-run-1 cell store before and after the fix. Separately, the render's own "Where fathom stands" block said "(no registered criterion for this corpus)" even once both arms filed correctly, because `criteriaBySHA256` only globs COMMITTED corpus JSON files and an external (LCA) corpus's criterion exists only in memory — `ExternalCorpusCriteria` (new) registers it directly so `criterionByID`'s scan can resolve it. The verdict block also now renders the bench-confidence layer's own paired bootstrap statistics (`ComputePairedRunStats`/`RenderPairedRunStats`, reusing agentic-kit's `reliabilitystats.PairedBootstrapCI`) for any run with a resolved treatment/control — lca-run-1's own reading (39 paired tasks): success-rate diff +0.009 [-0.077, +0.094], token ratio 2.00x [1.71x, 2.33x], matching the operator's independent bootstrap ([-0.077, +0.094] / [1.71, 2.34]) within rounding.

(2) reuse-pilot-4 10048/full/t2: a worker wrote `skip_test: true` into row GIST-TTY-3 (already correctly refused by `verify_plan` for a failing `test_reaches`) and the row came back claimed — AFTER wave N5's self-skip gate (`internal/verify/selfskip.go`) was merged. Root cause: that gate's own git-based comparison (`AnnotateSelfSkip`) only runs inside `verify_plan`'s MCP handler when a call supplies BOTH `plan_markdown` AND `plan_path` (`internal/mcp/verify_plan_tool.go`) — a call using the structured `tasks` array skips it outright (the annotate step is wired into the `plan_markdown` branch only), and a `plan_markdown` call with no `plan_path` never fetches a prior git version to diff against. The worker's own raw transcript (`session-3-worker.stream.jsonl`, benchroot2) shows exactly the second shape: its skip-adding call carried `plan_markdown` with no `plan_path`. Fix: the scenariobench DRIVER now catches this independent of what the agent passes the MCP tool — `SelfSkipRows` (new, `internal/scenariobench/selfskip.go`) scans every session's `BoxChecks` (recorded from real Edit/Write/MultiEdit tool calls, deduped by exact line text, never by row id) for any claimed ("- [x]") line whose own annotation carries `skip_test: true`, catching both the simultaneous claim+skip and the "refusal reversed" pattern (an already-claimed row re-edited later to add skip_test produces a second, textually distinct BoxCheck). `SuccessFor`, the one formula every Success-setting caller already shares, now checks this first and force-fails regardless of oracle shape, including the refusal-aware exemption; `CellResult` gained a `self_skip` field so a reader sees WHY a cell failed, not just that it did.

(3) reuse-pilot-4 10124: the held-out gold test_patch creates a NEW file (`pkg/cmd/repo/autolink/list/list_test.go`) at the exact path every agent independently wrote its own test file at, and STAGED (`git add`) — reproduced directly: `git add` a colliding file, `rm -f` it (the existing `resetTestPatchTargets` reset step), then `git apply` a new-file patch at that path still fails both ways ("already exists in working directory" plain, a conflict under `--3way`) because a working-tree removal does not clear the stale INDEX entry `git apply` also checks. All 4 cells were unscoreable — `RunOracle`'s own early-return on a failed `ApplyTestPatch` means fail_to_pass_ok/pass_to_pass_ok never actually ran. Fix: `ApplyTestPatch` now stages the patch in a driver-owned, agent-untouched worktree (`applyTestPatchViaStaging`, reusing `CalibrateSubject`'s own `prepareCalibrationWorktree`) and copies the resulting files into the agent's worktree by a plain `os.WriteFile`, which has no "already exists" failure mode and never touches the agent worktree's own index — the original reset-then-`git apply` path stays as a fallback for a Worktree with no real object store to stage from.

Rule: an instrument-truth check that only fires when the caller happens to pass the right combination of optional arguments, or that only clears a working-tree file without clearing the index entry a downstream git command also consults, is not a check the codebase can rely on — the DRIVER's own independent read (the session's real edit history, a staged worktree instead of a shared one) is what actually closes the gap, and a filing command whose invariants silently mismatch for one arm while its sibling arm files fine needs the SAME "name the mismatch and refuse" treatment the harness-version guard already gives a mismatch that empties the whole run.

### O72 — SandboxEscapeScan's own bench-root substring collided with the cell's own worktree, LOSing 105 of 140 vs-pilot-6 cells; 91 of them recovered from raw without re-spending (2026-09-17)

vs-pilot-6 (`lookups-vs.json`, 14 tasks × 5 arms × 2 trials) LOST 105 of 140 cells, all to `sandbox_escape`. Root cause: every run's bench ROOT is `<scratchpad>/benchroot2`, and `SandboxEscapeScan`'s own `sandboxEscapeMarkers` (O65, wave N12) includes the bare substring `"/benchroot"` — which `"/benchroot2"` also contains. The cell's OWN, entirely legitimate directories — its subject-worktree (`.../benchroot2/workspace/scenariobench/subjects/<sha>-worktrees/<cell>`) and its per-cell artifact dir (`.../benchroot2/workspace/artifacts/scenariobench/worktrees/<cell>`) — both live inside that root, so the very FIRST `cd` or tool call naming either one tripped the marker; the scan had no notion of "the cell's own directories" to exempt them against at all. Fix (`internal/scenariobench/answerkey.go`): `SandboxEscapeScan`/`textNamesSandboxEscape` now take the caller's own safe directories (`SandboxEscapeScan(session, safeDirs...)`); a marker match is an escape only when the FULL absolute path token it sits in (`enclosingPathToken` — bounded by whitespace/quote/bracket/`:` characters, never the bare marker substring) is neither a relative path nor nested under any given safe dir (`pathUnderAnyDir`). `runOneSession` (sessions.go) now passes `req.Worktree` (the subject worktree), the newly-threaded `cellDir` (`wt.CellDir`, added to `runOneSession`'s own signature and all three call sites), and `req.RawDir` (the raw transcript dir) — the same three directories `CellLost`'s own reason string already names. Tests: `TestSandboxEscapeScan_InWorktreeCdIsNotEscape` (a `cd` into the exact colliding shape above, real path strings, must NOT flag), `TestSandboxEscapeScan_RelativePathIsNeverAnEscape`, and the existing `TestSandboxEscapeScan_FindOutsideWorktreeIsCaught` strengthened to pass an UNRELATED cell's own safe dirs and still catch the genuine escape (`answerkey_test.go`).

vs-pilot-6 had already spent real tokens producing all 140 raw transcripts before this defect was found, so the fix alone does not recover the 105 already-LOST cells — nothing re-scores a stored session automatically. `scenariobench run -rescore-lost [-rescore-reason <text>]` (new; `cmd/scenariobench/rescore.go`, `internal/scenariobench/rescore.go`) does that from the raw streams already on disk, at zero spend: for each selected cell with no stored record (or, with `-rescore-reason`, a stored record whose own `instrument_defect` matches), it reconstructs `[]SessionResult` from the newest COMPLETE raw attempt (`LatestCompleteAttempt`, `IsCompleteSessionStream` — a stream a killed driver cut off mid-session is never picked) via the same `ParseSessionStream` the live harness uses, applies the EXACT post-session gate `runOneSession` applies to a session it just ran (answer-key leak, sandbox escape — now benefiting from the fix above via `HistoricalWorktreePath`/`HistoricalCellDir`, computed with no I/O from the task/arm alone, exactly reproducing what `PrepareSubject` would have named — the four instrument-truth checks, each session's own `wantServers` read from its captured `session-<n>-mcp.json` rather than re-deriving arm rendering), then runs the SAME oracle (`RunOracle`) with a zero `Worktree{}` — safe only because `NeedsLiveWorktree(task)` (test patch, named tests, a detector, or refusal scoring — none of which `lookups-vs.json`'s answer-only tasks carry) is checked first and refused as "not rescorable" rather than guessed. This file NEVER imports a `Harness` and never calls `PrepareSubject`/`RunSessions` — it cannot spend real tokens by construction, not merely by convention. Stored cells carry a new `rescored_from_raw: true` field, and never claim the readings that genuinely need a live worktree (`PlanCoverage`, `FalseDone`, `Resumption`, `ContractDrift`, `Overlap`, `RuleServed`) — left at their own zero value rather than guessed. Tests: `internal/scenariobench/rescore_test.go` (the pure helpers) and `cmd/scenariobench/rescore_test.go` (`TestCmdRun_RescoreLost_ReconstructsCellFromRaw`, `TestCmdRun_RescoreLost_GenuineLeakStaysLost` — a real escape in the raw stream stays LOST even under rescore — `TestCmdRun_RescoreLost_NeverInvokesHarness`, `TestCmdRun_RescoreLost_ReasonOverwritesMatchingStoredCell`).

Run for real against vs-pilot-6 (`./bin/scenariobench run -corpus testdata/scenariobench/lookups-vs.json -trials 2 -run-id vs-pilot-6 -root <scratchpad>/benchroot2 -cells <scratchpad>/benchroot2/workspace/artifacts/scenariobench/cells -raw-dir <scratchpad>/benchroot2/workspace/artifacts/scenariobench/raw -model-id sonnet -harness-version "2.1.274 (Claude Code)" -fathom-commit bcb62ac -rescore-lost -rescore-reason sandbox_escape`, invariants confirmed against the 35 already-stored cells and a `plan` dry-run first): of the 105 LOST cells, 91 rescored (74 PASS, 17 FAIL) and 14 stayed genuinely LOST (still real `sandbox_escape`, unrelated to this defect — `gopls` and `serena` 5 each, `graphify` and `grep` 2 each, `full` 0). Final tally: 126 of 140 cells scored (35 original + 91 rescored), 14 lost. Read-only on the raw store throughout (0 `.pid` files present, so the pre-existing orphan reaper touched nothing); the only write was new/overwritten cell files under `benchroot2/workspace/artifacts/scenariobench/cells`, which the concurrently-running guards pilot (a disjoint `guards-pilot-6` run id, 34 cells, untouched) never reads.

Rule: a literal-substring marker meant to name "outside the sandbox" must be checked against the sandbox's OWN known-legitimate directories before it fires, not merely against a fixed list of other trees' name fragments — a bench root's own directory name can, and here did, collide with the very marker meant to catch escapes FROM it. And a scoring defect discovered after a real run already spent tokens is not a reason to re-spend them: whatever the live path would have computed from a session's transcript, a re-parse of that same stored transcript can compute identically, for cells whose oracle never needed the worktree beyond what already got captured.

### O73 — `obligations` ranked same-tier rules by document order, not topical relevance; the raw fallback served decoys on 0 of 10 H1 governance trials (2026-09-17)

vs-pilot-6's second reading (`docs/bench/readings/vs-pilot-6-second-reading.md` §5): ADR-037's raw-guide fallback fired on 8 of 10 "what governs this" trials (H1-T01/T02/T08/T12/T13, new mechanism, up from never existing) but never once served the rule the oracle wanted — every fired trial served an unrelated decoy (internal/CLAUDE.md's "Store purity" section, or a root CLAUDE.md header) instead of the section that actually names the edited file. Root cause: within a governing-scope TIER (VB-8's `rankByGoverningScope` — exact_file/package/parent_dir/root), both `obligationsCore`'s `less()` (compiled claims, `internal/semantic/serve.go`) and `RawFallbackRules`'s candidate sort (`internal/semantic/rawfallback.go`) fell straight to document order (a raw section's own `StartLine`) or a near-alphabetical tie-break (a compiled claim's `specificity`/`Rule` text) — a nested guide's several same-tier sections, or a crowded root-tier bucket, were served in the order a human happened to write them, never by which one actually reads back the file's own name. `internal/CLAUDE.md`'s "Store purity" section (line 20) always won over its own "Adding things" section (line 33, the one naming `fusion.New`/golden-test) for `internal/extractor/goextract.go`, by document order alone.

Fix (`internal/semantic/relevance.go`, new): a deterministic, no-LLM lexical relevance score — tokens from the edited path (every directory segment, weighted ancestor < package < stem; the bare extension, weakest) plus optional caller-supplied symbol/import names at stem weight — scored against a candidate's heading (2×) and body, hit count capped at 3 per token (repetition doesn't compound), no division by section length (an early length-normalised version scored the "Adding things" section BELOW "Store purity" purely because its other three recipes diluted the ratio — exactly backwards for the fixture this exists to fix). Wired as a new tie-break inserted strictly INSIDE a governing-scope tier — after VB-8's tier+depth comparison, before the pre-existing specificity/rule-text/document-order tie-breaks — in both `obligationsCore`'s `less()` (path tokens only, no store I/O, keeping the edit-hook hot path cheap) and `RawFallbackRules` (path tokens plus `extraTokens`, resolved by `pkg/client.RawFallbackObligations` via new `semantic.SymbolNamesForFile` — one `QueryObservations` scan, since that call path already does several). The budget cap needed no separate "guarantee" mechanism: `matched`/`cands` are fully relevance-sorted before either cap-slicing block runs, so a rule naming the file's own package/stem now survives the cap by construction (`TestObligationsFor_RelevanceSurvivesCap`: the one rule of twenty naming `cmd/x/emit.go`'s own stem, alphabetically 15th, now sorts inside the 6-rule cap). Item 4: a long raw section's citation (`Src`) now narrows to the lines around its own first relevance hit (`citeWindow`, `rawSnippetWindowLines=8`) instead of always spanning the whole section — the MCP snippet renderer (`internal/mcp/obligationsnippet.go`) truncates a citation's TEXT from its first byte at the 240-byte cap, so an unnarrowed long section showed its opening lines, never the bullet a reader wanted.

Five-task offline reproduction (`internal/semantic/h1governance_test.go`, `TestH1GovernanceTasks_RawFallbackOffline` — a DIAGNOSTIC, not a hard gate, same status as VB-12's own free-oracle reading): runs `RawFallbackRules` over THIS repo's own real guides at HEAD with an EMPTY compiled layer (claims=nil), scoring whether the expected section's own heading is in the top 3 served rows. H1-T01 (`internal/extractor/goextract.go` → "Adding things") **PASS** — the flagship fix. H1-T12 (`internal/core/model.go` → "The domain model is the keystone") **PASS**. H1-T02 (→ ADR-021), H1-T13 (→ ADR-014) **MISS** in a strict top-3 window — not a relevance-ranking defect: internal/CLAUDE.md's own five sections are a NEARER tier (nested guide) than any ADR.md section (root) for this file, and VB-8's tier primacy (by design, unchanged by this fix) fills the 3-slot window with all-nearer-tier candidates before a farther-tier one gets to compete; a wider window (`max=40`) shows ADR-021 at position 7 of 40 — ahead of ~30 other ADR sections, a real ranking improvement, just not inside 3 slots already claimed by a closer tier. H1-T08 (a root-wide convention with no lexical tie to any one file's own name) **MISS** — this task needs a different mechanism (the file wasn't the signal; the topic was), out of this wave's scope.

VB-11 oracle (`go test ./internal/semantic -run GoverningRuleOracle`, hermetic 3-label fixture — the real 40-label corpus reading needs a compiled store this offline wave never builds) and the L1 payload replay (`go test ./internal/indexbench -run TestPayloadReplay`) both byte-identical before/after — no regression, no golden update needed.

Rule: a proximity ranker (VB-8) answers WHICH TIER a rule belongs to; it was never designed to answer WHICH RULE WITHIN a tier, and nothing else was answering that question either, so ties fell to accidents of authoring order or alphabetization. A tier's own membership is a geometry question (closest scope wins); which member of that tier actually concerns the file being edited is a text question, and needs its own (small, deterministic, bounded) answer — never silence, and never "whatever a human happened to write first."

### O74 — O72's own 14 "genuinely lost" cells were four MORE safe-dir/token-boundary gaps in `SandboxEscapeScan`, not a race in the rescore batch; 10 of 14 recovered, cost was never actually missing (2026-09-17)

The vs-pilot-6 second reading (`docs/bench/readings/vs-pilot-6-second-reading.md` §6) re-read all 14 cells O72 left LOST and found every one had a complete raw session (`is_error: false`, a real `result` event) with a correct final answer, and guessed the rescore-from-raw BATCH itself raced or partially wrote (pointing at one task's 6 survivors sharing a single 03:01:54 timestamp). Direct reproduction (rebuilding `./bin/scenariobench` and re-running `-rescore-lost -rescore-reason sandbox_escape` against the real raw store) found no race at all: 13 of the 14 were still `sandbox_escape`, one was a genuine `instrument: mcp server gopls failed`, and every `sandbox_escape` traced to ONE of four boundary/safe-dir gaps in `SandboxEscapeScan`/`pathUnderAnyDir` (`internal/scenariobench/answerkey.go`), none touched by O72's fix: (1) `enclosingPathToken` treats no character as ending a token except whitespace/quote/bracket/`:`/`;` — a trailing `.` is never one, so Serena's own `<active-project>` startup message ("...activated a new project with name 'X' at <path>.\nActive language servers: go.") glues its own sentence period onto an otherwise-EXACT safe-dir token, failing `pathUnderAnyDir`'s prefix check by one byte, on literally every serena-arm session; (2) this harness's own colorized `ls -la` (OSC-8 terminal hyperlinks, `ESC]8;;file:///abs/path ESC\ display-text ESC]8;;ESC\`) embeds the FULL absolute path inside an escape sequence whose own `:`/`;` bytes corrupt the boundary walk into a token that starts or ends mid-string; (3) the cell's own safe-dir list (`req.Worktree`, `cellDir`, `req.RawDir`) never included the shared `subjects/` pool ONE level above the cell's own worktree — a bare `ls`/directory-listing `Grep` there (an agent orienting itself after a Read/Bash miss) trips the marker even though the pool holds nothing but sha256-hash-named directories, no task-specific content; (4) macOS resolves `/tmp` to `/private/tmp` — a tool call spelled with the bare `/tmp/…` form names the identical file as this fleet's own `-root` convention (always the resolved `/private/tmp/…` spelling), but `pathUnderAnyDir`'s bare string-prefix comparison saw two different trees.

Fix, all four narrowly scoped and none loosening a genuine catch: `enclosingPathToken` trims a trailing run of `.` from its own returned token (can only SHRINK it, never move where it starts, so a real escape's own marker position is untouched); `stripTerminalEscapes` (new, `ansiOSCRE`/`ansiCSIRE`) strips OSC-8/CSI escape sequences from text before `textNamesSandboxEscape` scans it — a decorated path's own href always duplicates a path the tool's structured input already carries unescaped, so this loses no detection power; `HistoricalSubjectsDir(root)` (rescore.go) and `SubjectsDirFromClone(wt.SubjectClone)` (subject.go, `filepath.Dir` of `PrepareSubject`'s own `cloneDir`) compute the shared pool root with no I/O and both `runOneSession` (sessions.go, threaded through all three `RunSessions` call sites) and `rescoreOneCell` (cmd/scenariobench/rescore.go) now pass it as a FOURTH safe dir — deliberately narrow: reading or `cd`-ing INTO one specific OTHER cell's own worktree, nested one level deeper, stays caught, since only THIS cell's own four dirs are ever safe; `pathUnderAnyDir` canonicalizes both sides through `macTmpCanonical` (`strings.TrimPrefix(path, "/private")`) before comparing. Two cells were investigated and deliberately left LOST rather than forced through: `H1-T03/graphify/t1`'s one remaining `sandbox_escape` traces to a tool_result the HARNESS ITSELF truncated mid-path (a `"... [20012 characters truncated] ..."` cut landing inside a hash string, and a second occurrence ending at the literal end-of-file) — the safety-confirming suffix is genuinely gone, not merely corrupted, and "never guessed at" means this stays LOST rather than trusting a truncated fragment; `H1-T11/serena/t1` (a recursive `Grep -i fathom` over the WHOLE `scenariobench/` dir, not just `subjects/`) and `H1-T11/gopls/t2` (`ls .../workspace/` — subjects/artifacts/logs/tmp all at once — plus an unrestricted `find /private/tmp/claude-501 -iname "*fathom*"`) both name real paths broader than the pool boundary defect (3) fixes; `workspace/scenariobench` itself also holds `calibration/` and `stores/` (a wired arm's own scratch daemon index, keyed per worktree, genuinely more revealing than a bare hash name), so widening safety that far was rejected as a real weakening, not a narrow fix, and both cells stay LOST. `H1-T04/gopls/t1`'s `instrument: mcp server gopls failed` is a real, correct catch (the reading's own §7 independently names it `mcp_not_connected`) — the same instrument-truth gate a live run applies, "never a looser bar," so it is not this defect's to fix either.

Separately, and despite the reading's own claim ("91 of 126 scored cells carry no dollar figure") — direct inspection of the raw store found `total_cost_usd` present and correctly parsed (`ps.result.TotalCostUSD`, stream.go) in EVERY session checked, live-filed and rescored alike, both already sharing the one `ParseSessionStream`; the reading's own gap was almost certainly its input materials (a stdout log covering only 35 of 140 cells), not the code. `stream.go` still gained a defensive fallback per the reading's own ask ("derive cost... from the stream's usage/model fields"): `ModelUsage map[string]{CostUSD}` (the result event's own `modelUsage` field) and `sumModelUsageCost`, applied only when `TotalCostUSD == 0` — an explicit nonzero total is never overridden.

Re-running `-rescore-lost -rescore-reason sandbox_escape` against benchroot2 with the fixed driver recovered 10 of the 14: **serena 26/27** (4 of 5 recovered — `H1-T11/serena/t1` stays lost), **gopls 21/26** (3 of 5 — `H1-T04/gopls/t1` and `H1-T11/gopls/t2` stay lost), **graphify 22/27** (1 of 2 pre-void — `H1-T03/graphify/t1` stays lost — plus a scorer_false_fail void below), **grep 20/27** (both of 2 recovered, before a separate contamination void dropped its own denominator by one), **full 24/28** (0 lost this wave). Applying the reading's five §3 voids (`vs-pilot-6-voids.json`) needed extending `scenariobench void -trial N`, which only ever set `Success = true` ("scorer_false_fail" was its only shape): `-verdict pass|fail` (new, default `pass`, fully backward compatible) lets a `scorer_false_pass` correct to `false`, and `-exclude` (new) `DeleteScenarioCell`s the one named trial entirely — for an instrument-caused trial (worktree-reuse contamination) that must count toward NEITHER pass nor fail, distinct from the whole-row void form (every trial equally untrustworthy) and from a proven single-direction correction. Applied: H1-T01/full/t1 and H1-T02/graphify/t1 (`scorer_false_fail`, `-verdict pass`), H1-T05/full/t1 (`scorer_false_pass`, `-verdict fail`), H1-T04/grep/t1 (cross-cell worktree contamination, `-exclude`), FRESH-M7-01/grep/t2 (`baseline_ran_fathom`, `-verdict pass` — a no-op flip that still records the defect; the pass verdict stands, its cost/token figures are informationally flagged only, since no aggregation path yet filters a per-cell `InstrumentDefect` out of `AggregateRowReadings`'s cost sum — a known, undeferred gap, not silently claimed fixed). Re-filed: `docs/bench/README.md`'s vs-pilot-6 line now reads **`full vs grep: success 24/28 vs 20/27, tokens 0.760×, cost 0.995×`** (full net-unchanged, one false fail up and one false pass down cancelling exactly as the reading itself predicted; grep's denominator down one from the exclusion). Final scored/lost, all arms: full 24/28, gopls 21/26, graphify 22/27, grep 20/27, serena 26/27 — 135 of 140 cells scored, 4 genuinely lost (down from 14), 1 excluded.

Tests: `TestSandboxEscapeScan_TrailingSentencePunctuationNotEscape`, `TestSandboxEscapeScan_HyperlinkedLsListingNotEscape`, `TestSandboxEscapeScan_SubjectsPoolListingNotEscape` (plus its own converse — a DIFFERENT cell's worktree nested under the pool stays caught), `TestSandboxEscapeScan_MacTmpSymlinkSpellingNotEscape` (`answerkey_test.go`); `TestRescoreSessionsFromRaw_SerenaActiveProjectMessageStaysScorable`, built from the real H1-T04/serena/t1 raw-dir shape (`attempt-1/session-0-single.stream.jsonl` + sibling `session-0-mcp.json`, structure copied not content), `TestHistoricalWorktreePathAndCellDir_MatchPrepareSubjectFormula` extended for `HistoricalSubjectsDir`, `TestSubjectsDirFromClone_MatchesHistoricalSubjectsDir` (`rescore_test.go`); `TestCmdRun_RescoreLost_SubjectsPoolListingRecoveredEndToEnd` — deliberately hands the driver NO safe dir naming the pool, proving the WIRING computes it, not just that `pathUnderAnyDir` can accept one (`cmd/scenariobench/rescore_test.go`); `TestParseSessionStream_CostFallsBackToModelUsageWhenTotalCostUSDIsZero`, `TestParseSessionStream_ExplicitTotalCostUSDWinsOverModelUsage` (`stream_test.go`); `TestCmdVoid_TrialFlagVerdictFailCorrectsScorerFalsePass`, `TestCmdVoid_TrialFlagExcludeDeletesOnlyThatCell` (`main_test.go`) — all confirmed to fail on the pre-fix code (built, ran red, then restored) before the fix made them green.

Rule: when a "genuinely lost, complete, correctly-answered" claim disagrees with a scorer that says LOST, re-run the scorer against the real raw data before trusting either a race-condition guess or the reading's own count of what's missing — both turned out wrong here (no race; cost was already present), and the actual defect was a token-boundary/safe-dir gap class distinct from, but siblings of, O72's own fix. And a void tool that can only correct in one direction (false_fail) will eventually meet a false_pass or a true per-trial exclusion it cannot express — extend it the same way `-trial N` itself was once added, rather than mis-applying the existing flag and silently filing the wrong verdict.

### O75 — Cross-tier promotion and behaviour-token relevance close zero of O73's three remaining H1 misses on the real corpus, but both mechanisms are proven correct on synthetic fixtures — the misses are now KNOWN structural limits, not open questions (2026-09-17)

O73's addendum left three of five H1 "what governs this" tasks unsolved and named two candidate mechanisms to try next: (1) let a farther-tier rule that strongly names the file leapfrog a nearer-tier rule that names nothing about it at all, and (2) give a root-wide convention with no file/package/symbol of its own (the workspace/ row) a way to tie to a file through the file's own BEHAVIOUR instead of its name. This wave built both, deterministic and no-LLM throughout, and then measured them honestly against the real corpus rather than declaring victory on the synthetic fixtures alone.

(1) `crossTierCloser` (`internal/semantic/governingscope.go`) replaces the bare `a.governing.closerThan(b.governing)` call in both rankers (`obligationsCore`'s `less()`, `RawFallbackRules`' candidate sort). It reproduces plain `closerThan` in every case except one: a farther-tier candidate whose relevance is STRONG — a hit on a packageWeight-or-above token (the file's own package path, stem, or a caller-resolved symbol/behaviour name; `relevanceStrength`, `internal/semantic/relevance.go`, `relevanceScore`'s own refactor to also report this) — is promoted ahead of a nearer-tier candidate that is a TOTAL lexical blank (relevance == 0, "names nothing about the file at all"). The cap is deliberately narrow: the moment the nearer candidate has ANY hit, however weak (an ancestor directory name mentioned once, in passing), proximity wins outright and no promotion happens.

(2) `BehaviorTokens` (`internal/semantic/relevance.go`) is a small fixed table of source-text signals — `os.WriteFile(`, `os.Create(`, `os.MkdirAll(`, … → `"workspace"`; `exec.Command("go"/"sh"/"bash", …)` → `"gate"`/`"validate"` — scanned as plain substrings against the edited file's own real source text. `pkg/client.RawFallbackObligations` already fetches every source file's content (`Sources`) to build the layer's corpus; it now looks the edited file up in that same list and folds `BehaviorTokens`' verdict into the pre-existing `extraTokens` path a resolved symbol name already used (wave O2 item 1) — no new store call, no new I/O.

Re-running `TestH1GovernanceTasks_RawFallbackOffline` against this repo's own real corpus at HEAD with both mechanisms wired: **the pass/miss split is unchanged from O73** (H1-T01/T12 pass, H1-T02/T08/T13 still miss) — but each miss now has a PROVEN reason instead of a plausible-sounding one, and two of the three are proven not by inspection but by a synthetic fixture that isolates exactly the mechanism in question and shows it working:

- **H1-T02** (ADR-021, strong on caller-supplied `Edge`/`EdgeKind`) stays crowded out because internal/CLAUDE.md's five same-tier sections each carry at least a weak hit (mostly the bare ancestor token "internal", mentioned in passing across the guide) — never a true zero — so the cap correctly declines to promote. `TestCrossTierCloser_AnyHitOnNearerBlocksPromotion` and `TestRawFallbackRules_CrossTierPromotion_AnyHitOnNestedGuideBlocksPromotion` pin this exact behaviour directly: a synthetic sibling of the same shape but with a genuinely BLANK nearer-tier section (`TestRawFallbackRules_CrossTierPromotion_StrongRootBeatsBlankNestedGuide`) DOES get promoted correctly — the mechanism works; the real corpus simply never hands it a true blank to promote past.
- **H1-T13** (ADR-014) stays a miss because ADR-014's own section is itself the FARTHER-tier candidate with ZERO relevance (no hit on "model"/"internal"/"core", no caller-supplied symbol either) — promotion only ever promotes a farther side that is ITSELF strong, and there is nothing here to promote. A rule whose content has drifted to name nothing lexically about the file it governs needs a semantic mechanism, not a lexical one — explicitly out of this wave's deterministic scope.
- **H1-T08** (the workspace/ convention) stays a miss for a narrower reason than either of the above: `cmd/fathom/emit.go` is a synthetic scenariobench path with no real file on disk in this repo, so `BehaviorTokens` has no source text to scan at all. The mechanism itself is proven to close exactly this shape of gap given a real file: `TestRawFallbackRules_BehaviorTokenPromotesUntiedRootConvention` (a synthetic corpus fixture: without a behaviour token a lexically-louder decoy wins the single slot; with the `"workspace"` token folded in, the untied convention wins instead) and `TestClient_RawFallbackObligations_BehaviorTokenFromFileContent` (the full client-level path: a real `cmd/emit.go` containing an actual `os.WriteFile` call, read off disk by `RawFallbackObligations` itself, promotes the same convention over the same decoy) both demonstrate it working end to end.

`h1governance_test.go` itself changes status (wave O3 item 3): H1-T01/T12 are now a HARD gate — `t.Errorf` on a MISS, failing the build, not a diagnostic `t.Logf` — since wave O2 already established they pin a real regression surface. H1-T02/T08/T13 stay diagnostic (`t.Logf`), but each subtest now carries its own `missWhy` string inline explaining exactly which of the two mechanisms was tried and why it doesn't close that particular gap, so a MISS never reads as a silent skip or an open question. VB-11 oracle (`go test ./internal/indexbench -run TestGoverningRuleOracle_Deterministic`) and the L1 payload replay (`go test ./internal/indexbench -run TestPayloadReplay`) both stayed byte-identical — no regression, no golden update.

Rule: a mechanism that is CORRECT (proven on a fixture built to isolate it) is not the same claim as a mechanism that CLOSES a specific known gap (proven against the real corpus that motivated it) — both are worth stating, and conflating them is how a "we fixed it" report quietly becomes untrue the next time someone reads the real numbers. The honest form here is two proofs side by side: the mechanism works (synthetic fixture, unconditional), and it did not move this particular needle (real corpus, with the specific reason why) — never one substituted for the other.

### O76 — `-rescore-lost` can now RE-MATERIALIZE a live worktree for a task whose oracle needs one, but guards-pilot-7's own raw dirs never captured the one artifact that makes it possible (2026-09-17)

O72/O74 rescore a LOST cell from raw only when `NeedsLiveWorktree(task)` is false — a detector, named tests, or refusal scoring get refused outright as "not rescorable", because `RunOracle` against a zero `Worktree{}` cannot run `go test`, a forbidden-path diff, or a shell detector at all. guards-pilot-7 (`guards-fathom.json`, 8 tasks × 2 arms × 2 trials, run at commit `bcb62ac`) hits exactly this: every task carries `oracle.detectors`, so all 32 cells `NeedsLiveWorktree`. The run LOST 31 of 32 to the sandbox_escape false positive O72/O74 already fixed; only `guards-i28-duckdb-path/control/t1` was scored live. Inspecting every raw dir under `workspace/artifacts/scenariobench/raw/guards-pilot-7/` (`attempt-1/session-0-single.stream.jsonl` [+ an occasional sibling `.stream.jsonl.stderr`] plus `session-0-mcp.json`) found NO `agent.diff`, NO `NOTES.md`/plan file, and NO oracle artifact anywhere — the driver never captured one for this run, and the worktree that WOULD have held `wt.CellDir/agent.diff` (`workspace/artifacts/scenariobench/worktrees/`) plus the shared subjects clone (`workspace/scenariobench/`) were both deleted by the operator after the run to reclaim disk.

Built the re-materialization capability anyway, proven against a fixture rather than guessed at (`internal/scenariobench/rematerialize.go`, new): `RematerializeCellWorktree(ctx, root, task, arm, attemptDir)` calls the SAME `PrepareSubject` a live cell used (identical sparse exclusions, instrument writes, `LocalBase` semantics — this is the one place in the rescore path that now calls `PrepareSubject`, never `RunSessions`/a `Harness`, so it still cannot spend real tokens by construction) against `attemptDir`'s own stored `CellRawAgentDiffPath` (`agent.diff`, captureAgentDiff's own `git add -N .` + `git diff LocalBase` shape) and `CellRawUntrackedSnapshotDir` (`untracked-snapshot/`, a new convention for a byte-identical fallback a diff alone can never carry — a binary file, since `captureAgentDiff` never passes `--binary`). `ApplyStoredAgentDiff` re-filters the stored diff through the SAME instrument-owned-path exclusion `captureAgentDiff` applies (widened, `rematerializeInstrumentOwnedPaths`, to also cover `wt.Arm`'s own rendered `SettingsPath`/`MCPConfigPath` — RenderArm writes both for EVERY arm, wired or not, but the wiring ledger `InstrumentOwnedPaths` reads only exists after a WIRED arm's `fathom init`, so an unwired arm's own rendered files were otherwise un-owned and a stored diff naming them as "new" would collide with THIS worktree's own freshly-rendered copies — "already exists in working directory", caught building the fixture test itself). `AgentDiffBinaryFilesMissingSnapshot` refuses (never guesses) whenever the diff names a binary create/modify with no matching snapshot file. `cmd/scenariobench/rescore.go`'s `rescoreOneCellRematerialized` then runs the SAME baseline detectors + `RunOracle` + `VoidBaselineRedDetectors` a live cell runs (O57 finding 3), against the REAL `wt`, and the returned `cleanup` always stops any scratch daemon and `os.RemoveAll`s the worktree — disk-small, one re-materialized worktree at a time, exactly as `PrepareSubject` treats it on the next cell for the same `(base_commit, arm)`. Stored cells carry `rescored_from_raw: true` AND the new `rematerialized: true` (`CellResult.Rematerialized`, `types.go`) — still never claiming the SDD readings a plan.md this reconstruction has no sound way to date (PlanCoverage, FalseDone, Resumption, ContractDrift, Overlap, RuleServed), left at zero exactly like the non-rematerialized rescore path.

Deliberately NOT built: replaying the raw session stream's own Edit/Write/MultiEdit `tool_use` payloads (full content is right there — `old_string`/`new_string`/`content`) as a diff substitute when no `agent.diff` was ever stored. It is tempting — guards-pilot-7's own streams carry every edit verbatim — but a Bash call in the same session could also have mutated a tracked file (`sed -i`, `mv`, a redirect, `gofmt -w`) with no structured tool_use event to show it, so replaying only the Edit/Write calls can never be PROVEN byte-identical to what the driver actually scored. `RematerializeCellWorktree` refuses instead, naming the missing `agent.diff` explicitly, never silently trusting a partial reconstruction — the same "never guess" bar O72/O74's own rescore-from-raw path already held.

Failing-first test (`cmd/scenariobench/rescore_test.go`, `TestCmdRun_RescoreLost_RematerializesLiveWorktreeOracleAndScores`): a fixture raw dir (one `Write` session stream + a real captured `agent.diff`) for a tiny, entirely local git fixture subject and a task whose oracle carries one real detector — pre-wave-N20 code refused this outright ("oracle needs a live worktree"); it now re-materializes, applies the diff, runs the detector against the rematerialized tree, stores `Success: true, rescored_from_raw: true, rematerialized: true`, and deletes the worktree. Converse (`TestCmdRun_RescoreLost_LiveWorktreeOracleWithNoStoredDiffStaysNotRescorable`): the same shape with no `agent.diff` stays LOST, names the missing diff, and never even attempts `PrepareSubject`. Plus `internal/scenariobench/rematerialize_test.go`: `TestRematerializeCellWorktree_AppliesStoredDiffOntoFreshPrepareSubject` (the live-then-delete-then-rematerialize round trip, byte for byte), `TestRematerializeCellWorktree_NoStoredDiffIsNotOK`, `TestRematerializeCellWorktree_EmptyDiffIsAZeroEditCell` (a zero-byte diff is a real zero-edit outcome, never confused with "nothing captured"), `TestAgentDiffBinaryFilesMissingSnapshot_FlagsAndClearsOnSnapshot`, `TestAgentDiffBinaryFilesMissingSnapshot_DeletionNeedsNoSnapshot`, `TestApplyStoredAgentDiff_OverlaysUntrackedSnapshotOnTopOfDiff` — all confirmed to fail on the pre-fix code before the fix made them green.

Re-ran `-rescore-lost -rescore-reason sandbox_escape` for real against guards-pilot-7 (`./bin/scenariobench run -corpus testdata/scenariobench/guards-fathom.json -trials 2 -run-id guards-pilot-7 -root <scratchpad>/benchroot2 -cells .../cells -raw-dir .../raw -model-id sonnet -harness-version "2.1.274 (Claude Code)" -fathom-commit bcb62ac -rescore-lost -rescore-reason sandbox_escape`): the new machinery correctly found NO `agent.diff` under any of the 31 LOST raw dirs and reported every one "not rescorable: oracle needs a live worktree and no agent.diff was captured under …", never attempting `PrepareSubject` (confirmed: `workspace/scenariobench/` and `workspace/artifacts/scenariobench/worktrees/` stayed absent throughout, `benchroot2`'s own git status shows zero writes outside the pre-existing `cells/` store). Final tally, unchanged from before this wave because nothing WAS rescorable: **control 1/1** (`guards-i28-duckdb-path`, the one cell scored live), **full 0/1** (`M1-1: indeterminate, below the registered trial floor`), 31 lost. Filed into the clone's ledger (`ledger -corpus testdata/scenariobench/guards-fathom.json -run-id guards-pilot-7 -cells <scratchpad>/benchroot2/workspace/artifacts/scenariobench/cells -commit bcb62ac -fathom-commit bcb62ac -harness-version "2.1.274 (Claude Code)" -trials 2`, corpus bytes confirmed byte-identical to `git show bcb62ac:testdata/scenariobench/guards-fathom.json` before filing since cell keys hash the corpus): `docs/bench/README.md` gains a **guards-pilot-7** line (`control 1/1 (surface calls 0)`) and one ledger row.

Rule: build the general capability the moment it is needed and prove it against a fixture, but never spend a re-materialized worktree's cost (a real `PrepareSubject`, a real wired arm's scratch daemon and `fathom compile`) chasing a cell whose raw dir was checked first and found to hold nothing the new machinery can act on — "not rescorable, and here is exactly why" is itself the honest, complete answer for this run, not a placeholder for a future one. And a diff-reconstruction convention that re-derives instrument-owned exclusion from the wiring ledger alone will miss whatever an UNWIRED arm's own renderer writes unconditionally — widen the exclusion to the renderer's own manifest (`ArmFiles`), not just what got ledger-tracked.

### O77 — `agent.diff` lived only under `wt.CellDir`, a directory the NEXT cell resets; now every cell persists it into its own raw attempt dir, unconditionally, before the LOST decision (2026-09-17)

O76 built `RematerializeCellWorktree` to rescore a LOST cell whose oracle needs a live worktree, but guards-pilot-7's own 31 raw dirs held no `agent.diff` for it to consume at all — because `captureAgentDiff` (`cmd/scenariobench/main.go`) only ever wrote one to `wt.CellDir/agent.diff`, under `workspace/artifacts/scenariobench/worktrees/…`. That directory is SHARED across every trial of one `(subject, base_commit, arm)` (O57 finding 6's own per-trial reassignment lives one level below it) and is exactly what the next cell sharing that key, or an operator's own disk-reclaiming cleanup, resets or removes — so once a cell turned out LOST (sandbox_escape, an instrument error, anything `CellLost` catches AFTER real sessions already ran and really edited files), its `agent.diff` had no second home and was gone for good. The raw attempt dir (`workspace/artifacts/scenariobench/raw/<run-id>/<task>-<arm>-t<n>/attempt-<k>/`, `allocateRawAttemptDir`) is the one directory this driver never reuses and never resets — exactly the durable record O76's own machinery needs.

Fix: `persistRawCellArtifacts` (`cmd/scenariobench/raw_artifacts.go`, new) writes, into that same raw attempt dir, a byte-for-byte copy of `captureAgentDiff`'s own output at `scenariobench.CellRawAgentDiffPath(attemptDir)`; a byte-identical `untracked-snapshot/` copy of every binary file the diff names as created or modified (`scenariobench.AgentDiffBinaryFilesMissingSnapshot` against the not-yet-existing snapshot dir names the full set to try), each capped at `maxRawUntrackedSnapshotFileBytes` (10 MiB, a `var` so a test can shrink it) — what was captured and what was skipped (and why, and its size) goes into `untracked-snapshot.json` alongside it, so an absence is never ambiguous between "nothing to capture" and "capture refused"; and `cell-artifacts.json` naming the task's base commit, `wt.LocalBase`, and the instrument-owned paths `captureAgentDiff` excluded (not consumed by `RematerializeCellWorktree` today, the smallest useful record of what the copy leaves out). `runOneCell` calls this right after `captureAgentDiff`, unconditionally and BEFORE `RunOracle` — well before the caller's own `scenariobench.CellLost` gate (`runCellForReport`) decides whether the cell is a reading at all — so it runs for every cell regardless of outcome, exactly the property O76's own gap needed. Best-effort throughout (a write failure lands on stderr, never fails the cell), matching `captureAgentDiff`/`captureAgentPlan`'s own existing philosophy that a captured artifact is a convenience for later re-scoring, never a precondition for the readings already computed from the live worktree.

Locating the attempt dir from `runOneCell` needed its own small lookup rather than reusing `SessionResult.RawPath`: `ClaudeHarness` writes a session's raw stream to `req.RawDir` and stamps that path onto `RawPath`, but `FakeHarness` (`harness_fake.go`) never touches `req.RawDir` at all — it reads straight from its own committed `StreamsDir` and stamps THAT path onto `RawPath` instead, so deriving the attempt dir from a session's own `RawPath` would have pointed persistence at committed testdata fixtures for every `-harness fake` cell (test and dry-cell runs alike) rather than the real raw dir. `allocateRawAttemptDir` creates the attempt-`<k>` directory unconditionally, before dispatching to either harness, so `scenariobench.LatestAttemptDir(cellRawBase)` (new, `rescore.go`, alongside `LatestCompleteAttempt`) — the highest-numbered `attempt-<k>` subdirectory currently on disk, regardless of whether anything was ever written into it — names the right directory for both harnesses with no change to `RunSessions`' own signature (which ~25 call sites across this package's tests share) and no reliance on stream-file content at all.

Failing-first test (`cmd/scenariobench/raw_artifacts_test.go`, `TestCmdRun_PersistsRawAgentDiffEvenWhenCellIsLost`): a real local git fixture subject, `-harness fake` replaying a stream whose own tool_result names a path outside every safe dir (the same `/pushclone/` shape `TestCmdRun_RescoreLost_GenuineLeakStaysLost` uses) — the LIVE `run` path, never `-rescore-lost` — produces a LOST cell (`sandbox_escape`, exit 3, nothing stored), and `agent.diff` plus `cell-artifacts.json` are still found under the raw attempt dir; confirmed to fail on the pre-fix code (`no such file or directory`) before the fix made it pass. `TestPersistRawCellArtifacts_RematerializeAcceptsTheRawCopyUnchanged` is the round-trip proof for O76's own consumer: a live `PrepareSubject`, a real edit, `captureAgentDiff` + `persistRawCellArtifacts` exactly as `runOneCell` calls them, the worktree destroyed (simulating the cleanup that lost guards-pilot-7's own cells), then `scenariobench.RematerializeCellWorktree` reproduces the tree byte for byte — with NO change to `RematerializeCellWorktree`/`ApplyStoredAgentDiff`/`CellRawAgentDiffPath`/`CellRawUntrackedSnapshotDir` at all, since this fix targets those exact existing paths. Plus `TestPersistRawCellArtifacts_CopiesAgentDiffAndCellArtifacts`, `TestPersistRawCellArtifacts_CapturesUntrackedBinaryFile`, `TestPersistRawCellArtifacts_SkipsOversizedBinaryFile` (the documented cap, and its `untracked-snapshot.json` skip entry), `TestPersistRawCellArtifacts_EmptyAttemptDirIsANoOp`; `TestLatestAttemptDir_PicksHighestNumbered`, `TestLatestAttemptDir_EmptyAttemptDirStillCounts` (the FakeHarness shape — an attempt dir with nothing written into it still counts), `TestLatestAttemptDir_MissingDirIsFalse` (`internal/scenariobench/rescore_test.go`) — all confirmed red before the fix.

Rule: an artifact meant to survive a shared, reset-prone directory needs its OWN durable home from the moment it is first written, not a "capture it again once a rescue tool needs it" plan — O76 built the rescue tool and only THEN discovered, against real production data, that nothing had ever been left for it to rescue. And when a per-session path (`RawPath`) is stamped differently by different `Harness` implementations for reasons specific to each (a real harness's own written artifact vs. a fake harness's own fixture source), do not read a directory out of it for a purpose neither implementation's own doc comment promises — recompute the shared, harness-independent fact (the attempt dir every `RunSessions` call allocates up front, regardless of harness) instead.

### O78 — The evidence was already in the payload; two bench readings found the model reasoning past it anyway, so wave L1 made the direct answer the first thing on the page (2026-09-17)

`docs/bench/readings/vs-pilot-6-second-reading.md` and `vs-pilot-5` name the same shape twice. H1-T05: `find_callers(RenderMarkdown, depth=2)` returned the direct caller correctly — first in `callers`, depth 1, unambiguous — and the model's final answer named a caller three hops out instead; the correct name appeared only in an intermediate reasoning aside, never the stated conclusion, and the lexical scorer matched the aside (`vs-pilot-6`'s own `scorer_false_pass` void). H1-T07: `search` hit `ADR.md:591` — the right line — and the answer cited a different document. Both tools were correct. Both payloads were complete, budgeted, and depth-labeled. The agent read past the right row anyway, because nothing in the response said "this one" — a JSON object built for completeness has no notion of what a reader scans first.

Fix, deterministic and additive (`internal/mcp/summary.go`, new): every promoted-verb response now leads with a `summary` field — one sentence, ≤200 chars, naming the direct answer in the question's own terms, built from the SAME sorted/post-budget data the rest of the body carries (never a wider pre-elision set, so it can never claim a row the body does not actually return). `find_callers`, `references`, `blast_radius`, `rename_impact`, `safe_delete_check`, `test_impact`, `localize`, `search`, and `obligations` all gained one. The mechanical problem BA-1/RC-19's existing `elided`/`elision` fields never had to solve: every handler here returns `map[string]any`, and `encoding/json` always sorts a map's keys alphabetically — adding a `"summary"` entry to the map would not make it FIRST, it would sort wherever `"s"` falls (after `resolution`, before `top_packages`, for find_callers). `summaryEnvelope.MarshalJSON` splices `summary` in ahead of the body's own encoding instead of trying to reorder the map, so the fix needed no handler rewritten to a struct and no existing field touched. find_callers' own summary is built from `callers[:len(wireCallers)]` — the depth-1 prefix the wire actually returns, post-budget — never from `transitive_callers`, so a depth-2+ caller can never be named "direct": the exact defect H1-T05 found.

Tests: one summary-content assertion per verb (`internal/mcp/summary_test.go`), five against the real fauxtown fixture fleet ingested through the connector+extractor+fusion pipeline (`indexbench.NewFixtureServer` — the same construction `internal/indexbench/testdata/l1-replay.json`'s golden entries replay against), four against this package's existing lightweight fakes (`richFakeBackend`, the fauxtown `safe_delete_check` fixture, `newSearchServer`). `TestFindCallersSummary_NamesDirectNeverDepth2` reproduces H1-T05's exact shape using the existing `graphBackend` fixture (Target's direct callers Direct1/Direct2; Deep calls Direct1 at depth 2; TestDeep calls Deep at depth 3) and asserts the summary's direct-callers clause never contains "Deep". `TestPayloadReplay` (`internal/indexbench`) regenerated the L1 golden: median payload growth 6.8%, but three tiny `obligations` fixtures (547-885 bytes) and one `find_callers` fixture (389→470 bytes) grew 21-28% — the fixture baseline is small enough that even a minimal one-sentence direct answer cannot stay under a 10% guideline built for a realistic multi-KB payload; `references` (+8.7%), `localize` (+9.9%) and `blast_radius` (+4.8%) all clear it. Shortened what was reasonably compressible (`obligations`' rule-head cap 100→70 chars) and left the rest as specified rather than gut the summary's content to chase the last few points on an unusually small fixture.

Rule: a correct, complete, well-labeled payload is not the same claim as "the agent will read the right part of it" — when a bench reading shows the model finding the right evidence and stating the wrong conclusion anyway, the fix is not more data or a stronger label buried at position N, it is putting the direct answer where a reader who stops after the first sentence still gets it right. And a growth-budget guideline stated as a percentage of payload size is a fleet-realistic number, not a promise that holds against a fixture built to be as small as possible on purpose.

### O79 — Java was already extracted, not merely classified; Kotlin genuinely had nothing — wave P2 closed Kotlin from scratch and closed Java's field/import gap without moving it off CGO (2026-09-17)

The Long Code Arena bench run (`docs/bench/readings/lca-run-1-reading.md`) had 12 Kotlin and 8 Java tasks of 40; O69 (wave P1) disclosed Kotlin's missing extractor but left Java's status unverified — only four tree-sitter grammar `.wasm` blobs shipped (`internal/extractor/tsengine/grammars`: javascript/python/ruby/typescript), yet `.java` was mapped to `KindJavaClass` in `internal/connector`, so it was unclear whether that was a real extractor or just a classification stub. Investigated first, per the assignment: reading `internal/extractor/javaextract.go` and `internal/fusion/pipeline.go` showed Java was already fully wired — `extractor.NewJavaExtractor()` (CGO tree-sitter bindings, `github.com/tree-sitter/go-tree-sitter` + `tree-sitter-java/bindings/go`, ADR-013) has been registered in `fusion.New` at `p.exts[".java"]` since before this wave, with its own golden-fixture tests (`javaextract_test.go`) covering classes/interfaces/enums/records/methods/constructors and same-file call edges. It was never "only classified" — but it WAS incomplete: no field declarations, no import declarations, the exact two constructs the task's own scope named for both languages. Kotlin, by contrast, had genuinely nothing: no grammar blob, no extractor file, no registry entry anywhere.

Given that split, wave P2 deliberately did NOT build `java.wasm` or a second, wazero-based Java extractor even though the task brief asked for one "the same way" as Kotlin's — ADR-021 §4 already names moving the CGO incumbents (Java among the nine) to the wazero engine as XT-6b, "a separate, not-yet-triggered slice" (ADR.md, `docs/intent/futures.md` F1), and building it now would pre-build a documented future the project-intent rules say not to pre-build. Instead: (1) **Kotlin**, genuinely new, went through the wazero path ADR-021 §4 reserves for new-language breadth — `internal/extractor/tsengine/grammars/kotlin.wasm` built by the existing `build.sh` recipe (extended, not replaced) from `fwcd/tree-sitter-kotlin` tag `0.3.8` (commit `e1a2d5ad1f61f5740677183cd4125bb071cd2f30`, not an official tree-sitter-org grammar), same `zig 0.16.0`/`wasm-opt 129` toolchain that produced the original four blobs, sha256 `750a499adfc0a1339abebe5da80154eb6680c5732a3a5fdceaaace5053def3c3` (2,488,435 bytes — the largest blob of the five). fwcd/tree-sitter-kotlin uses NO `field()` names anywhere in its grammar (verified against `grammar.js` directly, not assumed), so `internal/extractor/tsqueries/kotlin.scm` locates every name by node kind and anonymous-keyword position instead (`(class_declaration "interface" (type_identifier) @name)` distinguishes interfaces from classes by which literal token is present; `(source_file (property_declaration …))` restricts properties to top-level by requiring a direct-child structural path, mirroring `python.scm`'s module-var constraint) — this was worked out by fetching the grammar source and reading `grammar.js` line by line before writing a single query pattern, then verified against a real parse via a throwaway debug test (deleted before the final commit) rather than guessed and iterated blind. `tsRegistry["kotlin"]` (`internal/extractor/tsextract.go`) is pure data (kinds map + variant table), no new Go walking logic — exactly ADR-021 §4's "adding a language = grammar blob + queries + fixtures" promise, now proven true for a language with zero incumbent to crib from. (2) **Java** stayed on CGO and gained `KindJavaField`/`KindJavaImport`: `field_declaration`'s `_variable_declarator_list` repeats the SAME field name (`declarator`) across multiple `variable_declarator` children when a line declares more than one field (`int a, b;`), so `ChildByFieldName` — which only ever returns the first match for a repeated field — silently would have dropped every field after the first; fixed by walking named children by KIND instead. `import_declaration`'s `$._name` is unfielded (unlike everywhere else in this grammar), so the import path is read as `NamedChild(0)` directly — a wildcard import's `*` is a separate later sibling (`asterisk` node) and is dropped, matching Kotlin's own wildcard-import handling for consistency across the two languages this wave touched. `ExtractorEpoch` bumped 6→7 (both changes: a brand-new extractor for `.kt`/`.kts`, and a coverage change to what the existing `.java` extractor emits) so every already-indexed Java file's stored observations — missing fields and imports — get pulled onto epoch 7 on the next ingest rather than staying silently incomplete.

Proven live on two real subjects (shallow clones into scratch, `bin/fathom ingest --store <scratch>.sqlite`, deleted after — never the shared daemon/store): `bumble-tech/appyx` (Kotlin, the same repo O69's finding cited, 549 files, 4.4MB) — 6,725 symbols (4,129 imports, 1,259 functions, 578 properties, 464 classes, 122 objects, 62 interfaces) where O69 measured ~165; `jhy/jsoup` (Java, 313 files, 4.7MB, picked over `junit-team/junit5` for size) — 7,559 symbols including 806 fields and 1,912 imports that did not exist under epoch 6, alongside 4,418 methods/314 classes/14 interfaces/12 enums carried over from the pre-existing extractor. Tests: `internal/extractor/tsextract_test.go` (`TestTSExtractor_Kotlin*` — basic construct coverage, top-level-only property scoping, nested-class chaining via `.`, the `.kts` extension sharing the same grammar, determinism); `internal/extractor/javaextract_test.go` (`TestJavaExtractor_FieldsAndImports` — plain/wildcard/static imports, a single field, and the multi-declarator "two fields from one declaration" regression); `internal/extractor/tsengine/tsengine_test.go`'s `TestGrammars`; the connector/fusion/CLI-level `TestUnsupportedLanguageForPath_*`/`TestLanguageForPath_*`/`TestIngest_*UnsupportedLanguage*` suites all updated to use Swift (still genuinely unsupported) as their disclosure fixture instead of Kotlin, plus new `TestIngest_KotlinExtractsRealSymbols`/`TestIngestReportShowsKotlinSymbols` proving the closure at the fusion and CLI layers respectively. `go build ./...`, `gofmt`, `go vet`, and `go test -race` all green across `internal/extractor/...`, `internal/connector/...`, `internal/fusion/...`, and the `cmd/fathom` `Ingest|Plans|NoOrg` subset (the one pre-existing failure, `TestTypedEdges_PerformanceGate`, reproduces identically on the unmodified base commit — a machine-load-sensitive perf gate, unrelated to this wave).

Rule: "no extractor" and "no COMPLETE extractor" are different findings and need different investigations before either is assumed — Java's own connector entry looked exactly like Kotlin's from the outside (an extension mapped to a `SourceKind`), but reading the actual extractor file settled which one it was in minutes, and conflating them would have meant either building a redundant second Java extractor or, worse, skipping Java's real gap (fields, imports) because "it already has an extractor" sounded like "it's done". And a documented not-yet-triggered future (XT-6b: migrate every CGO incumbent to wazero) is a reason to LEAVE a working extractor exactly where it is, layer-appropriately extended, not a reason to invent work matching the shape of a task brief written before the investigation had an answer.

### O82 — ingest's own response now discloses a typed pass with nothing to trust, a hook can no longer re-point a service, and `obligations` accepts `path` as `file`'s alias (2026-09-17, wave P3)

Four defects filed from the 2026-09-16 push-clone incident (see this file's earlier note and `docs/guides/fleet-index.md`): the daemon answered `{"ok":true}` for a push clone whose typed-edge enrichment produced ZERO typed edges (go.work named sibling checkouts that existed only beside the operator's own checkout), so `test_impact`/`verify test_reaches` silently read "no test reaches it" and a push was refused with nothing in the ingest that ran it to explain why; the same clone's lefthook post-commit `fathom-refresh` step re-registered service "fathom" at the clone's own path.

(1) **Ingest honesty.** `worktree.CheckReroot`/BP-35's own `TypedEdgeStatus` machinery (Sep 12) already computed the right verdict for a "packages seen but 0 processed" typed pass, but nothing on the `ingest` tool's OWN response ever read it — a caller had to make a SEPARATE `staleness_report`/`doctor` call to discover the gap, and the incident shows an agent does not always make that call before pushing. `fusion.Report` (`internal/fusion/pipeline.go`) now carries `TypedPackagesSeen`/`TypedFailed`/`TypedFailReason`/`TypedFallback`, populated from the SAME `Pipeline.TypedEdgeStatus(svc.ID)` staleness_report/doctor already read — one source of truth. `fathom ingest` (text and `--json`, `cmd/fathom/ingest.go` + `ingest_json.go`) and the MCP `ingest` tool (`internal/mcp/extra_tools.go`) now print/carry `typed_edges:{packages_seen, packages_processed, edges, failed, error?, fallback?}` whenever a typed pass was ATTEMPTED (not the pre-O82 "non-zero packages/failures only" condition, which is exactly what let a Go tree with zero typed edges print/carry nothing at all) — `failed:true` flips `--json`'s `outcome` to `degraded`/exit 1, the text path's own exit code, and the MCP tool's top-level `ok` to **false**, while the syntax index itself is kept (never rolled back — "say it plainly", never "refuse"). `list_services` gained the same per-service `typed_edges` row (previously absent entirely) so `known:false`/`failed:true` reads identically everywhere. `fathom doctor`'s existing `typed_edges` row (BP-36) needed no change: it already reads `staleness_report`, which reads the same underlying status map.

(2) **GOWORK fallback.** `TypedEdgeEnv` (BP-35, Sep 12) already forces `GOWORK=off` unconditionally on every typed-edge subprocess, so a go.work naming a missing/foreign sibling cannot break enrichment for a tree ingested elsewhere — confirmed with a regression test (a temp module under a go.work whose second `use` names a path that does not exist: `TestEnrichTypedEdges_GoWorkMissingSiblingSurvives`). Added the disclosure half: `TypedEdgeReport.Fallback`/`TypedEdgeStatus.Fallback` name `"GOWORK=off"` whenever an ambient workspace was discoverable and therefore overridden (`detectWorkspaceOverride`, `internal/fusion/typedenv.go` — a best-effort `go env GOWORK` probe run WITHOUT the override, from the repo's own directory, so upward auto-discovery or a global `go env -w GOWORK=…` pin both surface). The probe needed its own environment repair (`repairedGoEnv`, factored out of `TypedEdgeEnv`): `go env GOWORK` itself fails outright under the same broken-GOPATH shape BP-35 exists to fix (`go: GOPATH entry is relative; must be absolute path: "$HOME/go"`), which would have made detection silently report "no override" on exactly the daemon environments most likely to need one. Writing the test surfaced a wave-setup artifact worth recording: this repo's own gate commands `export GOWORK=off` around every `go` invocation, which — since `os.Environ()` is inherited wholesale — makes every fixture's go.work invisible to `detectWorkspaceOverride` unless the test explicitly `t.Setenv("GOWORK", "")` first; a persistent `go env -w GOWORK=…` pin is a DIFFERENT thing from a shell-exported one (neither was present as a stray global on the machine this wave ran on — checked `~/Library/Application Support/go/env` directly rather than assumed).

(3) **A hook must never re-point a service.** `worktree.CheckReroot` (O22/PC-33) only refuses re-rooting a service onto a LINKED git worktree (`git worktree add`) of its own recorded root; a plain `git clone` elsewhere — a separate physical checkout, exactly the push-clone shape — sails straight through it, since it shares no git-common-dir and carries no `.claude/worktrees/` marker. The daemon-side `ingest` handler (`internal/mcp/extra_tools.go`) now refuses ANY `path` that differs (canonicalized) from an EXISTING service's recorded root unless `remap:true` is also passed — the deliberate-move escape hatch, parallel to the store-direct `fathom remap` verb, which never touches the daemon. The `post-commit-hook` verb (`cmd/fathom/postcommithook.go`) now asks `list_services` for the registered root BEFORE calling `ingest` and skips with a plain message on a mismatch, instead of stumbling into the daemon's own refusal and printing a confusing "daemon refresh failed … falling back" line for what is really "this hook is running inside the wrong checkout" — fail-open on enumeration failure (daemon down or too old), leaving the `ingest` call's own refusal as the backstop. `TestPostCommitHookPrefersDaemon`'s call-count assertion moved from exactly-1 to exactly-2 (the new `list_services` pre-check, then `ingest`) — a deliberate, documented behavior change, not a regression.

(4) **`obligations` `path`/`file` alias.** Calling the `obligations` MCP tool with `path` instead of `file` silently fell back to root-scope rules only (`rootScope := p.File == ""` read true because the argument landed in a field the handler never read) — hit live by the operator today. `internal/mcp/semantic_tools.go` now accepts `path` as an alias, `file` winning when both are given; documented in the tool schema/description, `docs/mcp-tools.md`, `docs/guides/fleet-index.md`.

Failing-first tests throughout, one commit per defect: `internal/mcp/semantic_tools_test.go` (`TestObligationsTool_PathAliasForFile`, `TestObligationsTool_FileWinsOverPath`); `internal/mcp/ingest_remap_test.go` (`TestIngestRefusesPlainCloneReroot` + 3 subtests), `cmd/fathom/postcommithook_remap_test.go` (`TestPostCommitHookSkipsWhenRootIsNotRegistered`, `TestPostCommitHookRefreshesWhenRootMatchesRegistered`); `internal/fusion/typedcalls_goworkfallback_test.go` (`TestEnrichTypedEdges_GoWorkMissingSiblingSurvives`, `TestEnrichTypedEdges_GoWorkFallbackDisclosed`, `TestEnrichTypedEdges_NoGoWorkNoFallback`); `internal/fusion/typedenv_test.go` (`TestIngestReport_CarriesTypedEdgeDisclosure`, `TestIngestReport_DisclosesTypedFailure`), `internal/mcp/platform_tools_test.go` (`TestListServices_IncludesTypedEdgesOutcome`, `TestListServices_OmitsTypedEdgesWhenUnknown`), `internal/mcp/ingest_typed_disclosure_test.go` (`TestIngestTool_TypedEdgesDisclosedOnSuccess`, `TestIngestTool_TypedEdgesFailureMarksOkFalse`, `TestIngestTool_TypedEdgesOmittedWhenUnknown`), `cmd/fathom/json_envelopes_test.go` + `cmd/fathom/ingest_typed_disclosure_test.go` (CLI text/`--json` disclosure and degraded-outcome cases).

Rule: a fact an instrument already computes correctly in-process (BP-35's `TypedEdgeStatus`, Sep 12) is not "surfaced" until the SPECIFIC call site that would have prevented the incident carries it — `staleness_report`/`doctor` answering the question honestly for five days did not stop the push-clone incident, because nothing made an agent ask them before pushing; the fix was never a new fact, only wiring the existing one onto the response an agent was already going to read. And a hook's own refusal-avoidance check (list_services before ingest) is not redundant with the daemon's own refusal (remap:true) even though both enforce the same invariant — the hook's version produces a clear, on-topic message at the point of the mistake; the daemon's is the backstop for every OTHER caller.

### O80 — Wave O4: an ancestor-only hit no longer blocks cross-tier promotion (T02 now passes; T13's own farther-side blank stays a proven miss), the bench's compile step was silently accepting whatever the layer's default budget stopped at, and doctor dropped corpus_grew off the wire (2026-09-17)

Three follow-ups from the readings behind ADR-037's wave O2/O3 addenda, worked as one wave.

**(1) Ranking.** O75 left H1-T02 and H1-T13 as proven, structural misses. Re-reading the actual mechanism: `crossTierCloser`'s promotion cap (`internal/semantic/governingscope.go`) required the nearer-tier candidate to be a TOTAL lexical blank (`relevance == 0`) before a farther, strong candidate could leapfrog it — but a bare ancestor-directory token ("internal", "cmd", "docs") registers as a nonzero hit under the existing scoring (`ancestorWeight`, `internal/semantic/relevance.go`) purely because every sibling file under that guide shares it, never because the guide actually names the edited file. H1-T02 (`internal/core/model.go` vs. ADR-021) was exactly this: internal/CLAUDE.md's other same-tier sections each carried only a weak "internal" mention, which the old `relevance == 0` test read as "not blank" and refused to promote past. Fixed by narrowing the cap's OWN bar from "the nearer candidate has ANY hit" to "the nearer candidate is itself STRONG" (`relevanceStrength`'s own packageWeight-or-above bit — package path, stem, or a caller-resolved symbol/behaviour token; never ancestor or extension weight) — `crossTierCloser`'s signature dropped the now-unused relevance floats entirely, taking only the two `strong` bools. Re-running `TestH1GovernanceTasks_RawFallbackOffline`: H1-T02 now passes as a hard gate. H1-T13 does not, and the reason is different from H1-T02's — ADR-014's own section is the FARTHER-tier candidate here and carries ZERO relevance itself (no hit on "model"/"internal"/"core", no caller-supplied symbol); promotion only ever promotes a farther side that is ITSELF strong, and this narrowing does nothing for a farther side with nothing to promote. Confirmed a real, separate finding along the way: re-running the fix against the REAL corpus surfaced that ADR-037's own multi-wave addendum (which documents this very ranking mechanism using its target tokens — "Edge/EdgeKind", "core", "model" — verbatim) also crosses the "strong" bar on ordinary domain vocabulary and, being the longest, most discursive entry in ADR.md, out-scores genuinely on-topic sections on raw hit-weighted magnitude within the same tier — a same-tier (wave O2) relevance-ranking limitation, not this wave's cross-tier one, and one this repo's own corpus growth keeps widening every time a wave documents a gap using the gap's own tokens. Proven in isolation, free of that real-corpus confound: `TestCrossTierCloser_AncestorOnlyHitOnNearerDoesNotBlockPromotion` / `TestCrossTierCloser_StrongHitOnNearerBlocksPromotion` (unit level) and `TestRawFallbackRules_CrossTierPromotion_AncestorOnlyHitDoesNotBlockPromotion` / `TestRawFallbackRules_CrossTierPromotion_StrongHitOnNestedGuideBlocksPromotion` (`RawFallbackRules` level, synthetic fixtures) all pass, and `TestGoverningRuleOracle_Deterministic`/`TestPayloadReplay` (`internal/indexbench`) stayed byte-identical.

**(2) Compile budget as instrument truth.** vs-pilot-5's second reading found the bench's own subject-preparation compile step (`internal/scenariobench/subject.go`'s `compileLayers`, `fathom compile -layer obligations -dry-run=false`) accepting whatever the layer's DEFAULT `max_tokens_per_run` (layer.cue, 200000) stopped at — 14/60 sections on the reading that motivated ADR-037 — with no flag to ask for the whole corpus in one run and nothing on the cell recording that the layer was incomplete at all. `fathom compile` gained `--max-tokens-per-run`/`--max-usd-per-run` (`cmd/fathom/compile.go`, direct `--store` mode only, mirroring `--force`/`--continue`'s own guard-flag pattern): empty leaves the layer's own budget untouched, `"all"` maps to the unlimited sentinel (0, or 0.0 — `enforceBudgets`' `MaxUSDPerRun` check gained the same `>0`-means-bounded convention `MaxTokensPerRun` already had; previously an explicit 0 there meant "refuse everything", the opposite of every other budget field in that function), a plain number overrides directly. Threaded via a new `semantic.BudgetOverride` (`Apply` returns a new `LayerDef`, never mutating the caller's own) and `pkg/client.Client.CompileLayerWithBudget`, additive beside the existing `CompileLayer`/`CompileLayerWithEscape`/`CompileLayerResume`. `compileLayers` now always passes `-max-tokens-per-run=all`. `CompileCost` (subject.go) and `CompileCostReading` (types.go) both gained `SectionsTotal`/`SectionsCompiled`/`BudgetStopped`, parsed off `fathom compile`'s own prose stdout (pulled into a new pure `parseCompileOutput` for testability) exactly like the existing `tokens_in`/`cost_usd` lines already were. `AggregateRowReadings` (cell.go) now actually sets the row-level `r.CompileCost` (previously never assigned at all, despite `CompileCostUSD` being summed) — the first NON-cached cell's reading, by ascending trial, falling back to the earliest cached one only when every cell in the row was cached, so a row never surfaces a legitimately-all-zero cached reading ahead of a real, populated one sitting right next to it. `RenderScenarioLedger` (ledger.go) folds `"layer N/M compiled"` (plus `" (budget_stopped)"` when applicable) into the notes column for a wired row that compiled a layer — no new table column, so every existing snapshot expectation of the header/column count survives untouched.

**(3) Doctor's own corpus_grew.** `fathom doctor`'s `obligations_served` check already printed a `PARTIALLY compiled (%d/%d sections, budget_stopped=%v)` WARN (O54 deliverable 5) — but its own wire-payload mirror, `doctorLayerCoverage` (cmd/fathom/doctor.go), had no field for `corpus_grew` at all, so `semantic.LayerCoverage`'s own `corpus_grew,omitempty` (already on the wire — `internal/mcp/semantic_tools.go` serializes the whole `LayerCoverage` struct verbatim) silently vanished on decode into an unknown JSON field. The operator's own daemon was observed at 42/66 compiled with a real `corpus_grew: 14` and the WARN said nothing about it — understating how much of the gap is "the corpus outgrew the last compile" (14 sections no compile run has EVER seen) versus "this run's own budget stopped it early" (a stale digest on an already-seen section, a different failure mode with a different remedy). Fixed: `CorpusGrew int` added to the mirror struct, and the WARN reason now appends `, corpus_grew=%d` whenever it is nonzero (omitted entirely when zero, so a budget-stopped run with NO corpus growth is never misdescribed as having outgrown anything) — the remedy command (`fathom compile --service %s --layer obligations --dry-run=false --continue`) was already correct and unchanged. Pinned with the operator's own exact numbers (`TestCheckObligationsServed`'s new "partially compiled layer names corpus_grew when the corpus has outgrown the last compile" / "...omits corpus_grew when the corpus has not grown" subtests).

Rule: a wave that documents its own ranking bug using the exact tokens that bug is about will make the DOCUMENTING text itself the loudest false-positive candidate the next time someone re-runs the same reproduction — corpus self-reference is a real, measurable cost of this repo's own "narrate the finding in ADR.md" discipline, not a hypothetical one, and it compounds with each addendum. And a wire field that exists, serializes, and gets silently dropped by ONE reader's own hand-copied mirror struct is a shape worth grepping for on principle whenever a new field is added anywhere near an MCP payload doctor.go also decodes — the type system gives no warning for an extra JSON key nobody asked to keep.

### O83 — Governance now bites at the edit for a checkable rule: `enforce: true` denies a PreToolUse violation instead of only advising it (ADR-038, wave G1, 2026-09-17)

guards-pilot-8's own reading (`docs/bench/readings/guards-pilot-8-second-reading.md`) found the
governing rule now reaches the wired agent before the relevant edit on 4 of 8 tasks — ADR-036/037's
own fixes working — and the wired arm violated the rule anyway on two of those four (i19, an
org-specific name landed in a new fixture; i28, a lenient DuckDB path shipped over ADR-025's loud
refusal), with one session going on to rewrite its own pinned regression test rather than honor
the rule it had just been served. Violations tied the control arm 4-for-4. `docs/design/
guarded-edits.md` §2.2 had named exactly this as its own escalation trigger since the design's
first version: "revisit only with a documented incident a warning demonstrably failed to prevent."

Fix (ADR-038): `#Guard`/`#LayerRule` gain `enforce: bool | *false`; `internal/guardspec.LoadGuards`/
`LoadLayerRules` refuse to load an enforced rule with no deterministic check behind it, naming it.
`internal/guardspec/enforce.go` adds five checks, each a pure function over one proposed edit's own
path/text: `forbidden_paths`, `checks: ["org_token","secret_literal","workspace_output",
"usecase_maintained"]` (guard), `cgo_purity` + `import_direction`/`boundary` content scans (layer
rule, over the proposed text's textually-present imports). `internal/harness.HookOutput` gains
`Deny`/`DenyReason`; Claude Code and Codex render the documented `permissionDecision:"deny"` shape
(their existing "the shape fathom never emits" golden fixture is now the shape it emits, narrowly);
Gemini refuses it. `cmd/fathom/enforce.go` wires this into `edithook_pre.go`, before the advisory
daemon round-trip, with `FATHOM_ENFORCE=off` / `[guards] enforce = false` as the escape hatch. This
repo's own `.fathom/guard.cue` turns `enforce: true` on the rules mapping to four of the eight
guards-fathom.json tasks (i2, i19, i31, i36) plus the actual raw-cp-onto-daemon-path command guard
(i16's Bash half); i1/i11 have no pre-write signal and stay gate-only, i28 is a control-flow
decision no structural check can see, i16's CLAUDE.md-prose half stays gate-only via
`task config-audit` (a content-pattern check for "advises a raw cp" risks a false positive on the
Taskfile's own legitimate `cp -f bin/fathom … .new` line).

Along the way: `LoadGuards` rejected a `forbidden_paths`-only guard as "governing nothing" — only
`scopes`/`command_scopes` were recognized governing fields — a real bug this wave's own first
`forbidden_paths` guard tripped over immediately (fixed: `forbidden_paths` now also counts).
`testdata/scenariobench/guards-m1.json`'s `.fathom/guard.cue#L43` citation for `core-no-mcp-or-cmd`
had drifted to line 93 once this wave's own new guard entries grew the file above it — the same
class of line-citation maintenance the two immediately-preceding commits on `main` already needed
(H1-T07/guards-m1's ADR-011/ADR-036 citations) and evidently a recurring cost of citing a bare line
number into a file that keeps growing above the cited line, worth a future revisit toward
heading-anchored citations into `.fathom/guard.cue` the way `ResolveCitationLine` already does for
ADR.md.

Rule: an "obligation the agent can read and still contradict" is not proof of anything until a
check exists that the tool call itself cannot get past — a served rule and an enforced rule are
different instruments, and thesis 2 (vision 29 §2) says the compiled-test half of governance was
always meant to reach the edit moment too, not just the gate.
