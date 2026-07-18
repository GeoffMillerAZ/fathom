# foundations/evidence

The evidence spine: stdlib-only packages for representing, grading, journaling,
and stress-testing claims. CUE owns shape (`schema.cue` per package); Go owns
behavior. Works with `foundations/provenance` (authorship, journal, anchoring)
and `foundations/reasoning` (effective-state resolution) — see their READMEs.

## Two Span types — read this first

There are **two** `Span` types in this tree. They are not interchangeable.

- **`evidence-span.Span` is CANONICAL** for all new work. It carries full
  provenance: `Grade` (`*evidencegrade.Grade` — explicit epistemic tier),
  `Author` (`aiprovenance.Author` — who/what produced it), `Locator`
  (structured pointer into the source), plus `SourceID`, `RecordID`,
  `RecordHash`, `OccurredAt`/`ObservedAt`. `ValidateAsEvidence()` enforces the
  invariant *"no provenance → not storable as evidence."*
- **`evidence-graph.Span` is the LEGACY shape**: bare
  `ID/EntityID/Field/Value/Confidence/Source/ExpiresAt` — no Grade, no Author,
  no Locator, no record hashes. `kits/investigation` still composes it (graph +
  TTL eviction + contradiction surfacing), so it stays. Do not build new
  pipelines on it; `conflict`'s package doc calls it "an impoverished span
  type" for a reason.

## Conflict join convention (prominent on purpose)

`conflict.Detect` joins assertions to spans with the key:

    Assertion.PropositionID == Span.EntityID

When that key matches, `Assertion.Content != Span.Value`, and the assertion is
not a retraction, a `KindClaimVsEvidence` conflict is emitted. There is no
`Field` on an Assertion; `PropositionID` is the stable subject anchor. If your
spans' `EntityID`s and your assertions' `PropositionID`s use different ID
schemes, claim-vs-evidence detection silently finds nothing.

## The canonical pipeline

document-parsing → evidence-span → evidence-grade → assertion → conflict.Detect
→ authenticity → provenance/anchor + provenance/event-store (the journal).

```go
segs := documentparsing.Parse(raw)           // foundations/knowledge/document-parsing

span := evidencespan.Span{
    ID: "span-1", EntityID: "prop-42", Field: "owner", Value: "alice",
    SourceID:   "src-wiki",                  // registered in source-registry
    RecordID:   "rec-9",
    RecordHash: recordfingerprint.HashContent(raw),
    OccurredAt: occurred, ObservedAt: observed,
    Locator:    evidencespan.Locator{Kind: "line", Value: "12"},
    Author:     aiprovenance.NewAI(modelID, modelVer, promptHash, paramsHash, inputDigest, at),
    Grade:      &evidencegrade.Grade{Tier: evidencegrade.TierDocumentary, Confidence: 0.9},
}
err := span.ValidateAsEvidence()             // no provenance → not evidence

claim, err := evidencegrade.GradeClaim(graded, rule) // weakest-link tier + confidence

a := assertion.Assertion{ID: "as-1", PropositionID: "prop-42",
    Declarant: "witness-7", Stance: assertion.StanceAffirms, Content: "bob"}
conflicts := conflict.Detect([]assertion.Assertion{a}, spans) // claim_vs_evidence: "bob" vs "alice"

assess, err := authenticity.Assess(signals)  // suspect record?
cap := authenticity.MaxEvidenceTier(assess.Overall) // caps tiers of derived spans

entry, err := journal.Append("evidence.span.recorded", payload) // eventstore.NewStore()
b := anchor.NewAnchorBuilder()               // proof-of-existence for a batch
b.AddLeaf([]byte(span.RecordHash))
root, proofs, err := b.Build()
```

## Package map

Concurrency notes are taken from each package's own docs; where a package
documents nothing, the entry says so — assume single-goroutine use.

| Package | Purpose | Concurrency contract |
|---|---|---|
| `assertion` | Speech-act primitive: `Proposition` (WHAT is claimed) vs `Assertion` (WHO asserted, `Declarant`/`Stance`/when) | Not documented — pure value types + `Validate`; assume single-goroutine |
| `authenticity` | Authenticity/integrity signals → `Assess` → `Assessment`; `MaxEvidenceTier` bridges verdicts to tier caps | Not documented — pure functions over caller-owned values; assume single-goroutine |
| `conflict` | Legal-grade conflict taxonomy over assertions + canonical spans; `Detect` is pure, deterministic, severity-scored | Not documented — `Detect` is a pure function; assume single-goroutine |
| `doc-provider` | Docs-content port (`DocProvider` + in-memory impls). **Not part of the spine** — see its package doc | Not documented — assume single-goroutine |
| `entity-crosswalk` | Append-only canonical-ID ↔ alias mapping across source families; transitive `Canonical()` | Documented goroutine-safe (internal RWMutex); zero value NOT usable — use `New(clock)` |
| `entity-resolution` | Similarity primitives + match candidates for **human review**; never auto-merges | Not documented — pure functions; assume single-goroutine |
| `evidence-grade` | Tier taxonomy (`direct` → `inferred`) + `GradeClaim` weakest-link promotion rules | Not documented — pure functions over value types; assume single-goroutine |
| `evidence-graph` | LEGACY in-memory graph: TTL eviction + contradiction surfacing over the legacy Span | Documented **NOT safe for concurrent use** — serialise access externally |
| `evidence-resolver` | Deterministic ref → `evidencespan.Span` registry; `ResolveAll` preserves input order | Not documented — assume single-goroutine |
| `evidence-span` | **Canonical Span** + `Locator`; `Validate` (lenient) vs `ValidateAsEvidence` (full provenance) | Not documented — plain value types; assume single-goroutine |
| `record-fingerprint` | Content-hash dedup `Store`: new / unchanged / changed per record ID; `HashContent` is the canonical hasher | Not documented — assume single-goroutine (methods lock an internal mutex, but no contract is stated) |
| `source-linker` | Bidirectional span ↔ source links; outputs sorted for determinism | Not documented — assume single-goroutine |
| `source-registry` | Governed registry of evidence sources (stable IDs referenced by spans, linker, crosswalk) | Not documented — assume single-goroutine |

Adjacent: `provenance/ai-provenance` (the `Author` embedded in spans),
`provenance/event-store` (append-only hash-chained journal),
`provenance/anchor` (Merkle proof-of-existence), `reasoning/defeasible`
(effective-state over provisions/modifiers, weakest-link confidence —
consistent with the evidence-grade ethos).
