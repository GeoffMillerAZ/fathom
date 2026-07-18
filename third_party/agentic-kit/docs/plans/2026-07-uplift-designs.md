# 2026-07 uplift — design specs (Update-2 absorptions & key merges)

Companion to [2026-07-uplift.md](2026-07-uplift.md) §7. Normative design
for the net-new packages and the merge-into-existing-seam changes. Every
spec here was **re-derived from the reconstructed Update-2 notes**
(`tmp/agent_kit__update_2/`, photo-reconstructed — intent, not spec) through
the 2026-07-08 design critiques (`tmp/audit-2026-07-08/update2/`); garbled
fragments were re-decided on merits, not ported. All packages follow the
standard shape (schema.cue · types.go · impl · mock.go · tests · testdata)
unless waived, stdlib-only, injected clocks, org-agnostic: **no model names,
no family names, no rule catalogs shipped as code — all such facts are CUE
data supplied by the caller.**

Dependency spine: Wave 0 (replay hash) → Wave 1 (`model-catalog`, `Probe`,
structured outputs) → Wave 3 eval slice (needs model-catalog + OutputSchema)
and evidence ladder (needs evidence-anchor, feeds hitl). Nothing here blocks
Wave 2.

---

## 1. Wave 0 merge — canonical request hash in `llmclient/replay`

**Problem it fixes:** additive contract growth (e.g. ContentBlock gaining
`MediaType`/`Data`) changes serialized JSON and silently invalidates every
recorded cassette's `RequestHash` — the current red gate.

**Design (credit: Update-2 `llm-cassette` Key):** `hashRequest` v2
canonicalizes before hashing — sorted keys (existing), **drop zero-value
fields** (empty strings, zero numbers, nil/empty slices and maps), stable
encoding of `json.RawMessage` payloads. Cassette entries record
`hash_version: 2`; the reader accepts v1 entries by re-canonicalizing with
the v1 rule (no drop) so existing cassettes stay readable, and `-record`
always writes v2. `MatchByRequestHash` semantics unchanged (ordinal
verification, not keyed lookup — documented; keyed lookup is NOT a goal).

**Tests:** golden hash fixtures; property test: adding a zero-valued field to
a struct does not change its v2 hash; v1 cassette replays green.

---

## 2. Wave 1 — `foundations/llmclient/model-catalog` (package `modelcatalog`)

**Purpose:** one CUE-contracted home for per-model facts (capabilities,
family, lineage, cost tier, deprecation) and the pure checks that consume
them. The kit ships the schema, loader, validator, and *example* testdata —
never authoritative rows (cost-tracker PriceTable precedent; the Update-2
notes' own hardcoded examples were stale at writing time, which is the
argument).

**CUE sketch:**

```cue
#ModelEntry: {
  id:      string & !=""
  family:  string & !=""          // opaque org-defined tag, never an enum
  lineage: [...string]            // style-correlated families (distilled models)
  caps: {
    thinking:           bool | *false
    effort:             bool | *false
    temperature:        bool | *true   // accepted on the wire?
    seed:               bool | *false
    structured_outputs: bool | *false
  }
  cost_tier:  int & >=0            // ordinal within the caller's catalog
  deprecated: bool | *false
}
#ModelCatalog: { models: [ID=string]: #ModelEntry & {id: ID} }
```

**Go surface:** `Load([]byte) (Catalog, error)` (JSON mirroring the CUE,
`cue vet`-checked in tests) · `Catalog.Entry(id) (ModelEntry, bool)` ·
`Catalog.Validate() []Problem` (unknown lineage refs, deprecated-only
families) · **`ValidateIndependence(judgeID, producerID string, opts
IndependenceOpts) []Violation`** — judge and producer must not share family
or lineage; opts add session/history-sharing assertions supplied by the
caller. Violations are construction-time fatal in any consumer (fail before
tokens are spent). Unknown model ID = a Violation, not a silent pass.

**Composition:** Wave 1 adapters consult `caps` before mapping
Thinking/Effort/Temperature (never send what the catalog says the model
rejects); Wave 3 eval slice requires `ValidateIndependence` to pass before
any verdict is `decisive`; Ladder/failover rungs may cite catalog ids.

**Non-guarantees (doc'd):** the kit cannot know whether a caller's table is
current; `deprecated`/caps are caller assertions. Family separation
mitigates self-preference bias only — position/verbosity/style biases
persist cross-family (cite the literature in the package doc).

**Concurrency:** immutable after Load; safe to share.

---

## 3. Wave 1 merge — `Probe`/`Pinger` in `llmclient`

`type Pinger interface { Ping(ctx context.Context) error }` — anthropic
already satisfies it; **bedrock gains Ping** (signed zero-cost request,
typed-error mapped — its docs already promise one). Helper
`ProbeAll(ctx, map[string]Pinger) map[string]error` fails fast with every
lane's typed error (never just the first). Eval-runner and failover both
call it at startup. Hermetic tests via httptest; no separate package.

---

## 4. Wave 3 — the eval slice (`foundations/quality/*`)

One record vocabulary, four small packages + one check. Everything consumes
`#EvalRecord`; nothing re-implements journals, ledgers, judging, or scoring
engines that already exist (jsonl-log, correction-ledger, qa-harness,
eval-gate).

### 4.1 `eval-journal` (package `evaljournal`)

**Purpose:** the append-only truth of every judge call. A record *type* +
persistence, NOT a journal engine.

```cue
#EvalRecord: {
  run_id:      string & !=""
  scenario_id: string & !=""
  producer: { model: string, prompt_hash?: string, params_hash?: string }
  judge:    { model: string, family: string }
  verdict:  "pass" | "fail" | "absent_proven" | "abstain" | "error"
  decisive: bool                       // per §4.2 provenance + §5 discipline
  score?:   number
  rationale: string                    // empty rationale ⇒ decisive MUST be false
  evidence_refs: [...string]
  trace: { run_id?: string, span_id?: string }
  occurred_at: string                  // time.RFC3339, injected clock
}
```

**Go:** `Append(w, rec) error` (validates first — the CUE rules mirrored in
Go), `Load(r) ([]Record, error)` via `persistence/jsonl-log` exactly as
`investigation-journal/persistence.go` does; deterministic sorted marshal so
baselines are byte-stable and diffable. Corrections (mark INVALID, annotate
baselines) are **not** re-implemented: when first needed, apply
`correction-ledger` over `run_id` (merge-map).

### 4.2 `scenario-bank` (package `scenariobank`)

**Purpose:** versioned eval scenarios as CUE **data** with provenance
discipline (replaces the notes' absolute "production data only" rule, which
is a PII hazard and forbids canaries).

```cue
#Provenance: "production-captured" | "production-derived-redacted" |
             "synthetic-adversarial"
#Scenario: {
  id: string & !="", version: int & >=1
  provenance: #Provenance
  decisive_eligible: bool & (provenance != "synthetic-adversarial" | false)
  input: {...}
  expected?: {...}                     // gold, optional
  rubric?: #Rubric
}
#Rubric: { criteria: [...{ name: string, weight: number & >0,
  predicate: { kind: "contains" | "regex" | "json_path_eq" | "nonempty",
               arg: string, value?: string } }] }
```

**Go:** `Load`, `Filter(by provenance/id)`, `RubricJudgeFrom(#Rubric)
qaharness.Judge` — the interpretable predicate vocabulary compiles into the
*existing* RubricJudge; rubrics are never Go closures in data. Score history
is a projection over eval-journal records (`HistoryFor(scenarioID, recs)`),
not stored state. Anything production-derived MUST pass a redaction pass
before commit (compose pre-llm-dlp-gate / tokenization; enforced by a
`Validate` that rejects `production-captured` scenarios in committed
testdata).

### 4.3 `diff-reporter` (package `diffreporter`)

**One primitive, two roles** (roadmap #6 + Update-2 diff-reporter):
`Diff(base, cand []evaljournal.Record) Delta` — per-scenario/per-concern
score deltas, verdict flips, improvement/regression counts (a canonical
verdict flip is always a regression; score-drop significance is delegated to
the parked conviction-metrics — until then the threshold is caller config
with explicit units) — and `TrajectoryDiff(base, cand []llmclient.Message)
TrajDelta` — ordered tool-call sequence (name + args-hash), stop causes,
turn counts, golden-diffable with byte-stable marshal. No corpus needed for
the trajectory role: cassettes + RunResult.Messages suffice.

### 4.4 `determinism-gate` (package `determinismgate`)

**Layers INVERTED from the notes** (their layer 1 is unsendable to current
frontier models): **primary and always-on = structural** — schema validation
of judge output (OutputSchema/CUE), K-run variance probe (`Probe(ctx, judge,
scenario, K)` → agreement rate + per-field entropy + verdict-flip count);
**opportunistic = sampling** — pin temperature/seed only when
`modelcatalog` caps say the target accepts them; never construct a request
with a knob the catalog rejects. Verdict: `Stable(band)` / `Unstable(band)`
— regression detection consumes the measured band, not a constant.
**Non-guarantee (doc'd):** cannot make hosted models deterministic; hard
determinism lives in replay cassettes only.

### 4.5 Loop-honesty checks — no new packages

Config certification = FitnessFunctions registered in the already-landed
`fitness-function-registry` (independence validated · provenance gates on ·
journal attached · determinism probed), first dogfooded as
`task repo-fitness`. `integrity-evaluator` gains
`Options.ResolveRef func(ref string) bool` (nil ⇒ current in-memory set) so
"the cited path exists" is one injected resolver, not a new
mitigation-manifest package — foundations stay I/O-free.

---

## 5. Wave 3 — the evidence-acquisition ladder (`foundations/evidence/*`)

Investigate-until-confident with an **earned** ABSENT_PROVEN. Composes the
spine (element-coverage, coverage, evidence-grade, evidence-span,
evidence-anchor); re-derives nothing.

### 5.1 `evidence-signature` (package `evidencesignature`)

The keystone: "enough evidence" as a declarative, diffable, testable
per-concern contract, decided at authoring time, not inference time.

```cue
#Archetype: "existence" | "absence" | "threshold" | "contradicts"
#Scope: { globs?: [...string], source_ids?: [...string] }  // exactly one set
#ConcernSignature: {
  id: string & !=""
  archetype: #Archetype
  scope: #Scope                        // repo globs OR registered sources
  constraints?: [...string]            // paths/sources search is valid over
  predicates: [...{ kind: string, arg: string }]  // proof predicates
  threshold?: { op: ">=" | "<=" | "==", value: number }
  retriever_hints?: [...string]
  // absence archetypes MUST ship a planted-positive canary fixture
  canary?: { path: string, pattern: string }
}
```

**Go:** `PreconditionSatisfied(sig, findings, cov CoverageManifest) bool` —
existence: ≥1 decisive hit; absence: zero hits AND `cov` complete over
`scope` with no unexplained skips; threshold: arithmetic over findings.
**Honest restriction (doc'd + validated):** archetypes are
lexically-decidable claims only — reachability/cross-service are NOT
representable; decompose them into lexical sub-obligations or route to a
structured analyzer (the fleet's code-intelligence index is that class).
Signatures are owned, tested artifacts: glob-liveness test (every glob
matches ≥1 file or is annotated `expected_empty`), canary contract test
(Tier-2 must find the planted positive), CI set-difference alarm (live tree
minus union of scopes) — the maintenance burden is the system's dominant
operating cost and is budgeted, not wished away.

### 5.2 `evidence-ladder` (package `evidenceladder`)

```go
type Tier1 interface { Lookup(ctx, ConcernID) ([]Signal, error) }   // cache: signal only, never verdicts
type Tier2 interface { Search(ctx, Signature) (Findings, CoverageManifest, error) }
type Tier3 interface { Escalate(ctx, EscalationRecord) (HumanVerdict, error) }
```

Deterministic progression: applicability → Tier-1 → (miss or insufficient ⇒
Tier-2; **an empty Tier-1 result is a cache miss, never a conclusion**) →
(`PreconditionSatisfied` false after exhaustive Tier-2 ⇒ Tier-3). Verdicts:
`FOUND(refs)` · `ABSENT_PROVEN(manifest)` · `ABSENT_UNVERIFIED(manifest,
reason)` · `UNRESOLVED`. Per plan D10: ABSENT_PROVEN is scoped ("absent
within declared scope at snapshot <hash>") and requires a complete
`CoverageManifest{files, tree_hash, tool, flags, skipped: {ignored, binary,
oversize, symlink, lfs_stub, non_utf8}}` — any unexplained skip downgrades
to ABSENT_UNVERIFIED, fail-closed. Every Tier-2 escalation emits a
GapRecord (§5.4). **Tier-1 is an interface** — the fleet's sextant/fathom
index is a downstream adapter; the kit never imports it. Tier-3 timeout is
fail-closed to UNRESOLVED, never to absent.

**Tier-3 contract (first real consumer for kits/hitl +
operator-elicitation):** `EscalationRecord{concern, signature, manifest,
candidates, question}` (CUE-contracted) proposed through hitl's
propose→preview→approve engine; `HumanVerdict{who, when, grade
(evidence-grade tier), rationale, feedback}` — `feedback` is mandatory and
materializes as a signature amendment (an SME answer improves the contract
or the escalation recurs forever); the verdict enters the spine as a graded,
provenanced assertion, not a side channel.

### 5.3 `direct-investigator` (package `directinvestigator`)

Deterministic Tier-2 over an on-disk tree. **Pure-Go is the contract**:
`filepath.WalkDir` in sorted order + `regexp`, bounded file sizes, explicit
skip accounting into the CoverageManifest (that accounting is the
guarantee). Optional accelerator: an `Engine` interface whose rg
implementation shells through `bounded-exec` **only when the caller
explicitly provides the binary path** (never PATH-sniffed), equivalence-
tested against the pure engine when present and absent in CI. No LLM
anywhere — "searched exhaustively and found nothing" must be a provable,
reproducible claim. Refs minted as `file://<relpath>#L<start>-L<end>`
through `evidencespan.Locator` (+ optional `evidence-anchor` SpanProof) so
ladder output enters the spine graded and anchorable. For absence
archetypes, prefer broad weak patterns with downstream triage over narrow
"decisive" patterns with nothing to triage.

### 5.4 `extraction-gap-log` (package `extractiongaplog`)

The flywheel store: `#GapRecord{concern_id, tier1_key_missed,
scope_searched, outcome, found_refs, occurred_at}` appended via jsonl-log
(injected clock). Projections: per-concern Tier-1 hit-rate;
`WarmList(recs)` → the patterns Tier-1 repeatedly missed (the fleet feeds
this to sextant ingest); `Signals(recs)` → `improvementflywheel.Signal` so
gap accumulation rides the existing proposal loop. **Claim, honestly
scoped:** correctness is lower-bounded by Tier-2 (whose coverage §5.1–5.3
continuously verify); warmed entries are Tier-1 signal only, keyed to
content hashes, never satisfy `PreconditionSatisfied`, and absence-shaped
cache entries invalidate on any change under the concern's constraints.

### 5.5 Riding roadmap #24 — `retrieval-metrics` (+ ranker seam)

Pure functions beside the RAG truth-layer bridge: `Precision`, `Recall`,
`MRR` over expected-vs-found finding sets **plus the absence-quality metrics
the notes lack** — `FalseAbsentRate` (measured deterministically via the
canary fixtures; in production via Tier-3 overturn rate) and
`CoverageFraction` (live tree under some signature's scope). Per-archetype
metric applicability declared in the signature schema (an absence concern is
never scored by MRR — it is undefined on empty gold sets). The reranker is a
deterministic feature ranker over signature-derived signals (pattern
strength, path specificity, hit density, ref stability), LLM-free; a
similarity-backed mode exists only where an Embedder is actually wired,
marked non-deterministic, and excluded from ABSENT_PROVEN flows. The notes'
"BackedFabric/BRP/Provisional" modes are scan debris — not implemented.

---

## 6. Removals & refusals

Enumerated with rationale in the plan's §7 merge-map (agent-trace,
capture-typeset, conditions-DPA, slux-qa-harness refused; judge stack,
conviction-metrics statistics, leak-detector, improvement-loop,
document-domain Tier-2 parked with triggers). The Wave-4 kill list (§6 of
the plan) is unchanged by Update 2 — nothing in the notes resurrects a
condemned package; wave-manager explicitly does **not** revive the
coordination museum (policy = data here; execution = D7's worker-pool, if
ever).

## 7. Test doctrine for everything above

Deterministic, `-race`-clean, no network (httptest only), injected clocks,
CUE fixtures vetted by the gate. Three signature test classes are
mandatory, not optional: **canary** (planted positive found), **liveness**
(globs match the live tree or are annotated), **manifest** (skip accounting
sums to the walk). The eval slice's own records are byte-stable
(golden-diffable); the determinism gate is itself probed against MockClient
scripts with seeded variance. Every package doc states its concurrency
contract and its non-guarantees as plainly as its guarantees — the honest-
contract rule is what makes an *evals* layer trustworthy at all.
