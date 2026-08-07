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
- **Status:** open — data repaired, code fix (a)-(d) outstanding
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
- **Root cause:** TODO
- **Fix:** TODO
- **Status:** open
- **Impact:** TODO
