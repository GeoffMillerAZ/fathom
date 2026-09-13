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

### O27 — O24's deferral tax does not reproduce on Claude Code 2.1.270: the whole 17-tool, 28,715-byte agent surface loads eagerly and a session spends ZERO ToolSearch turns, with and without `alwaysLoad: true`. ADR-032's own revisit trigger ("a harness release removes MCP tool deferral") has fired.

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
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO
