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

### O1 — Answer.Noul and Record.Noul still carry the vendor term after DP-12 made the kind generic (KindPredicate)

- **Symptom:** Answer.Noul and Record.Noul still carry the vendor term after DP-12 made the kind generic (KindPredicate)
- **Evidence:**
  - **Tool:** foundations/decision (DP-12 review)
  - **Expected:** a generic kind's answer field has a generic name (for example Probability or Truth), so a consumer of a non-TypeSafe provider never reads one vendor's vocabulary in the core types
  - **Actual:** DP-12 renamed only the Kind; Answer.Noul, Record.Noul and consumer rows derived from them (fathom internal/decide decisionRow json:"noul") keep the vendor name; renaming them is another exported-field break, out of DP-12's scope
  - **Repro:** grep -n 'Noul float64' foundations/decision/types.go foundations/decision/record.go
- **Root cause:** DP-12's brief scoped the rename to the Kind; the answer fields were named after the vendor primitive when the port was written as a 1:1 mirror (2026-09-22).
- **Fix:** proposed, not done: rename the fields generically in DP-14's breaking landing (or its own plan row), keeping the old names as deprecated aliases for one release, and give fathom's JF-24 the matching row migration.
- **Status:** open
- **Impact:** two exported fields in the core types still name one vendor's primitive; no runtime effect.

### O3 — decisiontest conformance: TypeSafe carries a provider-echoed API key into TypeSafeError, and its score Confidence departs from the documented contract

- **Symptom:** the DP-16 conformance suite passes TypeSafe only with two harness declarations that name TypeSafe-specific behavior: `RedactsEchoedKey: false` and `ScoreConfidenceFromLevels: true`.
- **Evidence:**
  - **Tool:** foundations/decision/decisiontest (DP-16, wave wk2 item conform)
  - **Expected:** no error a Decider returns contains its API key, even when the provider's own failure body echoes the key (the openai adapter redacts exactly this case, `TestAzureAPIKeyNeverInErrors`); and a KindScore answer's Confidence is `Confidence` over a degenerate input, 1, as the package doc's "Confidence formula" section and the `Answer` doc comment in types.go say.
  - **Actual:** with `RedactsEchoedKey: true` the suite fails: `key leak: an error contains the API key: "typesafe: status 500: decisiontest: credential decisiontest-fake-credential-not-a-key was rejected"`. `TypeSafeError.Message` is the response body verbatim (capped at 512 bytes); its doc relies on the API never echoing the key. Separately, `typesafeParseScoreAnswer` computes Confidence over the wire's per-level `probabilities` (and a score reply without them reads as Confidence 0, not 1), while LLM and the docs give a score Confidence 1.
  - **Repro:** set `RedactsEchoedKey: true` in `TestTypeSafePassesConformance` (foundations/decision/conformance_test.go) and run `go test -run TypeSafePassesConformance ./foundations/decision/`; for the score case, compare `typesafe_wire.go` `typesafeParseScoreAnswer` with the `Answer` doc in types.go.
- **Root cause:** TypeSafe's error path trusts the provider not to echo credentials; the score Confidence was written against the vendor's level distribution after the core docs fixed the degenerate rule, and nothing compared the adapters until a shared suite existed.
- **Fix:** done 2026-10-09 (DP-27, decision 0018): `TypeSafeError.Message` (and the new `OpenAIError.Message`) is the body with the key, any echoed Authorization header or bearer token and any "sk-" key shape redacted before truncation, and `TestTypeSafePassesConformance` runs with `RedactsEchoedKey: true`. The score rule is decided once and documented in `Answer` and the package doc: Confidence over the level distribution when the wire carries one (typesafe, openai, which also report it in `Answer.Probabilities`), the degenerate rule (1) when it carries none (llm); `ScoreConfidenceFromLevels` stays because the wires differ, not the rule. A typesafe score reply without a distribution still reads as Confidence 0; openai refuses such an answer into Invalid.
- **Status:** fixed
- **Impact:** a provider or proxy that echoes the bearer token in an error body would put the key into logs through `TypeSafeError`; score Confidence values are not comparable across LLM and TypeSafe, which matters for per-provider calibration readings (DP-22, DP-23).

### O4 — tests select --safe picks 1157 tests for a one-package change because a new error type's Is method fans out to every errors.Is call

- **Symptom:** tests select --safe picks 1157 tests for a one-package change because a new error type's Is method fans out to every errors.Is call
- **Evidence:**
  - **Tool:** fathom tests select --since <base> [--safe] (scratch ingest, --service agentic-kit)
  - **Expected:** a change confined to foundations/decision (four new decorator files, doc.go) plus two docs selects the decision package's tests, about 80
  - **Actual:** 1157 tests ranked; cmd/kit-new and cmd/endpoint-guard tests "reach agentic-kit::foundations/decision/RecordedError.RecordedError.Is", although nothing outside foundations/decision imports the package. Before the commit, the same command selected 0 tests and reported 0 changed symbols: the new files were untracked, so `--since` saw no change and said nothing about them.
  - **Repro:** at agentic-kit ddcf01f (branch wk2-decor), fresh scratch ingest, then `fathom tests select --since 38d6e63 --store <scratch> --service agentic-kit` and look for RecordedError.Is in the reasons
- **Root cause:** suspected: `errors.Is` dispatches to any `Is(error) bool` method, and the call graph links every errors.Is call site to every implementer by method name, not by the error values that can reach it. Untracked files are invisible to a git-range diff.
- **Fix:** proposed, not done: do not count errors.Is/As dispatch to an `Is`/`As` method as reaching it unless the implementer's package is imported by the test's package; have `--since` warn when the tree has untracked source files.
- **Status:** open
- **Impact:** with --safe, adding one error type with an Is method selects nearly every test in the repo, so the selection saves nothing. Before a commit it silently selects none.

### O5 — a bare {symbol} anchor re-binds silently when a second package adds a same-named symbol, and verify then fails test_reaches on a symbol tests do reach

- **Symptom:** llm-surface md-002 (`{symbol: RunConformance}`, no package) passed before wave wk2 and fails default `fathom verify` after it, although no line of llm-surface.md or llm-test changed.
- **Evidence:**
  - **Tool:** fathom verify (default and --strict), fresh scratch ingest, --service agentic-kit
  - **Expected:** the anchor keeps meaning `llmtest.RunConformance` (foundations/llmclient/llm-test), or verify reports the name as ambiguous and names both candidates.
  - **Actual:** wave wk2 added `decisiontest.RunConformance` (DP-16). symbol_present now reads "RunConformance at foundations/decision/decisiontest/conformance.go:171" and test_reaches fails "no Test* statically reaches it", while 6 tests reach that function (DP-16's own edge_wired, `TestLLMPassesConformance` -> `decisiontest.RunConformance`, passes on the same store).
  - **Repro:** agentic-kit at the wk2 merge tip (59d3a38), fresh scratch ingest, `fathom verify --store <scratch> --service agentic-kit --plan docs/plans/llm-surface.md` with md-002 unpinned.
- **Root cause:** suspected: bare-name resolution picks one of several same-named symbols with no ambiguity signal, and test_reaches looks the name up by a different key than symbol_present and edge_wired.
- **Fix:** workaround done in the wk2 integration: md-002 pins `package: foundations/llmclient/llm-test` (the symbol the row always meant); verify exits 0. Proposed, not done: make a bare anchor that matches more than one symbol an explicit ambiguity (listing the packages) instead of a silent pick, and make test_reaches use the resolved symbol.
- **Status:** open
- **Impact:** any new same-named exported function anywhere in the repo can flip an unrelated plan's gate red, with a failure message that points at test coverage instead of the anchor.

### O6 — two parallel items that each add exported API to foundations/decision must both edit the same one-line compat.md row

- **Symptom:** wave jk's items move (DP-14) and compose (DP-21) both change foundations/decision's exported API, and the kit's rule is that new exported API is recorded in `docs/compat.md`. The package's whole record is one table row on one physical line (line 145, several thousand characters), so both items rewrite the same line.
- **Evidence:**
  - **Tool:** git merge of parallel wave items; docs/compat.md
  - **Expected:** two items that touch different parts of one package's record merge without a textual conflict, as their code (separate files) does.
  - **Actual:** move appends a DP-14 sentence to the `foundations/decision` row and rewrites its DP-12 sentence; compose records `ComposeFromConfig` in the same row. Any two such edits are one conflicting hunk. The CHANGELOG has the same shape at a smaller scale (the brief had to assign compose "the hunk after L21").
  - **Repro:** branch jk-move (docs/compat.md line 145) against compose's branch; `git merge` either into the other.
- **Root cause:** compat rows carry each package's whole change history inline, one line per package, so a package under active work becomes a single-line hot spot.
- **Fix:** proposed, not done: keep the compat row to classification plus a one-sentence rationale and move the dated change history into the package's migration notes or a per-package section below the table, so parallel items add lines instead of rewriting one; until then, a wave brief names one owner per compat row.
- **Status:** open
- **Impact:** a guaranteed manual merge on every wave that runs two items in one package; the merger resolves prose by hand, where a dropped sentence loses a compatibility record.

### O7 — compat.md's classification-counts table drifts silently: no check recomputes it

- **Symptom:** `docs/compat.md` "Classification counts (for sign-off)" read 36 experimental and 184 total while the rows below held 37 experimental and 185 packages; DP-16's `foundations/decision/decisiontest` row (wave wk2) was never counted.
- **Evidence:**
  - **Tool:** constitution-lint compat-coverage (task validate)
  - **Expected:** the summary table either matches the classified rows or the gate says it does not; the prose under it already says the counts are "recomputed from the rows below, not carried forward".
  - **Actual:** compat-coverage checks that every package has a row with a valid class, and never reads the summary table; the drift passed every gate. DP-14 (wave jk item move) recomputed it to 124/39/24 = 187 after adding its two rows.
  - **Repro:** at agentic-kit b6208bd, `grep -c '^| \`[a-z]*/.*| experimental |' docs/compat.md` gives 37 against the table's 36; `go run ./cmd/constitution-lint` passes.
- **Root cause:** the counts are hand-maintained, and the lint parses the rows (`parseCompatRows`, which skips the summary table on purpose) but not the summary.
- **Fix:** proposed, not done: have compat-coverage count the rows per class and fail when the summary table disagrees (or generate the table, as kit-catalog generates the README's package count).
- **Status:** open
- **Impact:** the sign-off distribution a reader quotes is wrong by however many rows landed since the last hand recount; small today, unbounded over waves.

### O8 — a lexical oracle reads the working tree, not the ingested store, so a fresh scratch ingest does not bind a lexical row to the commit it ingested

- **Symptom:** `fathom verify --strict --store <base store>` passes DP-24's lexical oracle (`pattern: "foundations/decision/typesafe"` in docs/compat.md) against a store ingested at b6208bd, whose docs/compat.md has 0 matches.
- **Evidence:**
  - **Tool:** fathom verify --strict --row DP-24 --json (scratch stores, --service agentic-kit)
  - **Expected:** a row verified against a store ingested at commit C is judged on C's content, so the wave contract's "verify on a fresh scratch ingest of HEAD" pins a lexical row to HEAD, as it pins symbol rows; or verify says the oracle read the working tree.
  - **Actual:** against the base store the oracle reports "pattern matches at docs/compat.md:145, 2 match(es) total (51932 bytes, non-empty)": 51932 bytes is the working tree's compat.md (wave jk item move), and `git show b6208bd:docs/compat.md` is 49755 bytes with 0 matches.
  - **Repro:** agentic-kit branch jk-move: ingest b6208bd into a scratch store, check out jk-move's tree, then `fathom verify --strict --store <that store> --service agentic-kit --plan docs/plans/decision-port.md --row DP-24 --json`.
- **Root cause:** suspected: the lexical oracle opens `path` relative to the repo root on disk at verify time instead of reading the ingested document, so symbol and lexical rows in one report are judged against different trees.
- **Fix:** proposed, not done: read the oracle's path from the store's ingested copy (or record the ingested commit and refuse when the tree differs), and name the source in the detail line. Workaround used here: the claim commit was verified after the tree was committed and re-ingested, and the pattern's absence at the base was checked with `git show`.
- **Status:** open
- **Impact:** an uncommitted edit can prove a lexical row on a store that never saw it; a tightened pattern cannot be shown to fail on an older store, so "tightening proves something" is checkable only by hand.
