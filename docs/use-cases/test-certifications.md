# Test Certifications

Say which test stands as evidence for which behaviour — an invariant, an incident, a spec, a governing rule, a port — and let fathom keep that claim honest: the test exists, reaches the code the target names, and passed in the last attested run. fathom asserts whether a certificate is **live**; a Review Kit requirement judges whether that is enough (vision 33, ADR-043).

---

## When to use this

- Your invariants or incident records say "a test enforces this", and you want that sentence checkable instead of prose.
- You want a plan row to be unprovable until a named test passes, the same way `fathom verify` already refuses an unreached symbol.
- You want a list of every declared target with no live certificate, each with the remedy on the line, for a review gate to consume.
- You want every implementation of a port listed with or without a conformance test.

Not for: a coverage percentage (see [test-coverage.md](test-coverage.md) for per-symbol coverage, which is a different fact), or running tests — fathom never runs them; it ingests test-kit's attested record.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `fathom tests ingest` (`--manifest`, `--results`) | CLI | Loads `tests/*.cue` suite manifests and attested result envelopes, joins each case to a `Test*` symbol by name, reads `// certifies:` markers, compiles the `certifications` layer |
| `fathom tests status` | CLI | Per suite: class, tier, last run's verdict and totals, `joined N of M cases` |
| `fathom certify status` | CLI | Targets, live certificates, gaps, join rate |
| `fathom certify gaps` (`--kind`, `--json`) | CLI | Every target with no live certificate and its remedy; `--json` is the versioned `fathom.certify.gaps/v1` shape |
| `fathom certify manifest` (`--out`, `--check`) | CLI | Regenerates `docs/certifications.md`; `--check` fails when the committed file was hand-edited |
| `fathom certify orphans` | CLI | `Test*` symbols that certify nothing, reach nothing changed in 180 days, and never failed |
| `fathom certify suggest` | CLI | Decision-port suggestions for unregistered tests (needs `[decide] provider`); never a claim |
| `fathom verify` (`certified_by:` plan key) | CLI | A row proves only on a live certificate from that test for the row's symbol |
| `review_diff` / `fathom review` | MCP tool / CLI | The `certifications` dimension: certificates per changed symbol, and targets in the changed files that lost a live one |
| `fathom receipt verify` (`--run-tests`) | CLI | Re-derives a `certification` receipt's reach from a bare checkout; reports `pass: unverified (no run)` unless you let it run that one test |

---

## 1. Declare what a test certifies

On the comment line above a Go test function:

```go
// certifies: invariant I35 (asserts), incident INC-21
func TestRefund_Idempotent(t *testing.T) { … }
```

Or per suite, in the test-kit manifest `tests/<component>.cue` (platform standard: test-kit `#Certifies`, ADR-0105 amending ADR-0042):

```cue
suites: "billing-invariants": certifies: [
	{kind: "invariant", id: "I35", strength: "asserts"},
	{kind: "port", id: "billing/ports.Ledger", strength: "conformance"},
]
```

Kinds: `invariant`, `incident`, `spec`, `rule`, `port`, `behaviour`. Strength ladder, ordered: `smoke` < `asserts` (default) < `golden` < `property` < `conformance` < `mutation-killed`. A malformed marker is an ingest warning naming the file and line, never a silent skip. Only the Go extractor reads markers today; other languages declare in the manifest.

## 2. Ingest the attested run

```sh
fathom tests ingest --service my-service --manifest . --results workspace/test-results/
fathom tests status --service my-service
```

Or set `[tests] results_dir` in `.fathom/config.toml` and every `ingest` picks the envelopes up (`docs/configuration.md`). Without an attested run, every certificate is `unproven` — a reached test is not a passing test.

## 3. Read the state, then the gaps

A certificate is `live` (reaches the target's symbols and the last attested run passed, no flake in the last three), `flaky` (served as suspect, never silently live), `broken`, `unproven` (no run), or `unreachable` (the test reaches nothing the target names). Only `live` satisfies a gate.

```sh
fathom certify status --service my-service
fathom certify gaps --service my-service --kind invariant
```

```
KIND       ID                       GAP   REMEDY
invariant  I35                      yes   add "// certifies: invariant I35 (asserts)" above a Test* function, or a #Certifies entry …
73 target(s), 13 live, 60 gap(s), joined 4762 of 4844 cases
```

(The counts are fathom's own, 2026-09-23.) An incident closes when its `Outcome:` line names `certificate <Test>` and that certificate is live. `--kind port` lists every exported interface with implementations and no conformance certificate, keeping each implementation's evidence tier.

## 4. Gate a plan row on it

```markdown
- [x] Refunds are idempotent {id: PAY-7, symbol: Refund, package: internal/billing, certified_by: TestRefund_Idempotent}
```

`fathom verify` returns `certified` only for a live certificate from that test for the row's symbol; `certificate-unproven`, `-broken`, `-unreachable` and `-absent` never prove the row.

## 5. Let the review kit judge

fathom states facts; adequacy is a Review Kit requirement (`testing.certified-targets`) whose `fathom-certify` evidence adapter runs `fathom certify gaps --json --kind <kind>` and decides with the organization's own remedy text and enforcement. The JSON schema string, field names and sort order are the wire contract.

---

## Setup & notes

- Test results come from test-kit's `#TestResult` envelope (plain JSON, NDJSON, or DSSE-wrapped; the digest is recorded as the run's identity). fathom verifies no signatures; that is test-kit's job.
- `docs/certifications.md` is generated. Commit it and run `fathom certify manifest --check` in CI; a hand edit is a defect.
- `certify suggest` is the only verb that needs a decision provider. With `provider = none` it prints one line and exits 0. Suggestions land in `workspace/artifacts/certify-suggestions.jsonl` and appear as hints in `certify gaps`; a human adds the marker.
- Open measurements (plan `docs/plans/test-certifications.md`): TC-M2 (half the deny-severity invariants live after two weeks), TC-M3 (the Review Kit requirement fires deny on a real change within a month), TC-M4 (half the first twenty suggestions accepted).

---

## Maintained

This doc covers: `fathom tests ingest` (`--manifest`, `--results`), `fathom tests status`, `fathom certify status`, `fathom certify gaps` (`--kind`, `--json`), `fathom certify manifest` (`--out`, `--check`), `fathom certify orphans`, `fathom certify suggest`, `fathom verify` (the certified_by plan key), `review_diff` (its certifications dimension), `fathom receipt verify` (`--run-tests`). Update it when these change.
