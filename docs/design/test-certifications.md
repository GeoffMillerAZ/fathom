# Test certifications — mechanics

Design for vision 33 (ADR-043). This is the contract the plan
(`docs/plans/test-certifications.md`) is gated on and the three repositories
build against: test-kit (the vocabulary), fathom (the facts), Review Kit (the
judgment). Every section names what is deterministic, what a declaration is,
and what tier each fact earns.

**Status (2026-09-23).** Stages 0-3 landed in fathom: the `certifications`
layer (§4, compile/resolve/sweep), `fathom verify certified_by` (I43), the
`review_diff`/`fathom review` `certifications` dimension, `fathom certify
status|gaps|manifest|orphans|suggest`, a `certification` receipt kind, the
`fathom doctor` `certifications` row, and fathom's own repo declaring
certificates for its deny-severity invariants and open incidents' tests
(`tests/fathom.cue`). §7's test-kit and Review Kit halves (TC-17, TC-18)
landed in platform-engineering, proven by that repository's own gate. Open:
TC-M2/TC-M3/TC-M4 (the three two-week/one-month/twenty-suggestion operator
measurements) and fathom's own live-certificate count against its
deny-severity invariants — see `docs/certifications.md` for the current
counts.

## 1. Vocabulary (owned by the platform standard, test-kit ADR-0042)

A certification names a target, a strength and, optionally, the tests that
carry it. Two declaration surfaces, one grammar.

**Per suite**, in `tests/<component>.cue` (the test-kit manifest):

```cue
#Certifies: {
	kind:     "invariant" | "spec" | "rule" | "incident" | "port" | "behaviour"
	id:       string & !=""       // I35 · US-12 · CLAUDE.md#deploy · INC-21 · billing/ports.Ledger · a free id
	strength: *"asserts" | "smoke" | "property" | "golden" | "conformance" | "mutation-killed"
	tests?:   [...string]         // Test* names inside the suite; absent = every case in the suite
}
#Suite: { …existing fields…, certifies?: [...#Certifies] }
```

**Per test**, in the source, on the comment line(s) immediately above a test
function, language-neutral:

```
// certifies: invariant I35 (asserts), incident INC-21
```

Grammar: `certifies:` then a comma-separated list of `<kind> <id>` with an
optional `(<strength>)`; strength defaults to `asserts`. A malformed marker is
an ingest warning naming the file and line, never a silent skip. The Go
extractor reads it in this round; the marker grammar is comment-based so every
other extractor can add it (a plan row per language, deferred with a trigger).

The strength ladder is closed and ordered: `smoke` < `asserts` < `golden` <
`property` < `conformance` < `mutation-killed`. Only `mutation-killed` is
corroborated mechanically; the others are declared and then checked for the
minimum they imply (a `conformance` certificate must sit in a suite of class
`conformance` naming a port; `mutation-killed` requires a mutation join).

Targets resolve deterministically, or the certificate is `contradicted`:

| kind | resolves to | via |
|---|---|---|
| `invariant` | a row in `docs/intent/invariants.md` | the intent corpus reader |
| `incident` | a record in `docs/intent/incidents.md` | the intent corpus reader |
| `spec` | the symbols `spec_coverage` binds to the id | existing spec tools |
| `rule` | a governing claim subject (`file#heading`) | the obligations layer |
| `port` | an interface symbol and its implementations | `find_symbol` implementations, `via` tier kept |
| `behaviour` | nothing; declared only | — (tier never above doc-claimed) |

## 2. Evidence producers (fathom ingests, never runs)

- **Manifests**: `tests/*.cue` under a service's root, loaded with `cue/load`
  against test-kit's schema mirrored in `schema/testkit/` (drift-gated against
  the pinned test-kit commit the same way `third_party/` is).
- **Attested result envelopes** (`#TestResult`): read from `--results
  <file|dir>` on `fathom tests ingest`, or from `[tests] results_dir` in
  `.fathom/config.toml` on every `ingest`. Per case: `id`, `name`, `status`
  (pass | fail | skip | flake), `attempts`, `duration`, `failure`; per suite:
  `verdict`, `totals`, `coverage`, artifact references. The DSSE envelope's
  digest is recorded as the run's identity; signature verification is
  test-kit's job and its result, when present, is carried as a fact.
- **Coverage profiles and mutation reports**: as today (`--profile`,
  `--mutation`), plus artifact references from the envelope resolved against
  `[tests] artifacts_dir` when the digest matches.

Store (additive): `test_suites` (service, component, suite, class, tier, port,
declared certifies), `test_cases` (service, suite, case id, test symbol key when
joined, last status, attempts, run digest, observed at), `test_runs` (digest,
started at, verdict, totals, coverage). Every row carries the service watermark
it was observed under.

**Case-to-symbol join**: a case's `name` is joined to a `Test*` symbol in the
same service by exact name (Go: `TestX`, `TestX/sub` joins to `TestX`). The
join rate is a reported fact (`fathom tests status` prints `joined N of M
cases`) and stage 0's kill criterion.

## 3. The certifications layer (fathom, deterministic)

Layer `certifications`, role `claims`, kind `certification`, one claim per
(target, test). It is compiled like the findings layer — `PutClaims` with
content-addressed ids, no model — from declarations × joins:

| field | value |
|---|---|
| `Subject` | `<kind>:<id>` (the target) |
| `Rule` | the served line: `TestRefund_Idempotent certifies invariant I35 (asserts) — live` |
| `Scope` | the test file plus the files of the symbols the test reaches that the target names |
| `Tier` | declared only → `doc-claimed`; test exists and reaches a target symbol → `pre-scan`; reach plus a passing attested run → `verified` |
| `Status` | `fresh`; `stale` when the test or a reached symbol moved since the last run; `contradicted` when the test is gone or the target does not resolve; `refuted` by a human |
| provenance | `{test, suite, class, strength, state, run_digest, attempts, coverage_pct?, mutation_score?}` |

**State** (a fact beside the tier): `live` (reach + last run passed, no flake
in the last three runs), `flaky` (a flake in the last three), `broken` (last
run failed), `unproven` (no attested run ingested), `unreachable` (declared,
but the test reaches nothing the target names). Only `live` satisfies a gate.

**Sweep**: on every ingest, like every other claim: a renamed test →
`stale`; a deleted test → `contradicted`; a newly ingested run updates state.

## 4. Where it is served

- **verify** (`internal/verify/markdown.go` grammar): row key `certified_by:
  <Test>` (repeatable; optional `strength: <level>`). Verdicts: `certified`
  (live certificate from that test for the row's symbol), `certificate-unproven`
  (no attested run), `certificate-broken`, `certificate-unreachable`
  (the test does not reach the symbol), `certificate-absent` (no such test).
  Only `certified` proves the row. The self-skip rule applies unchanged.
- **review** (`internal/analysis/review`, new dimension `certifications`): per
  changed symbol, its certificates with strength and state; targets whose scope
  intersects the changed files and now have no live certificate; `checked:
  false, reason: "no certifications declared"` when the layer is empty.
- **CLI verbs**, verb-first (P10), no new MCP tool: `fathom tests ingest
  --manifest <dir> --results <file|dir>`; `fathom tests status`; `fathom certify
  status [--service]`; `fathom certify gaps [--kind invariant|incident|spec|port]
  [--json]`; `fathom certify manifest [--out docs/certifications.md] [--check]`;
  `fathom certify orphans`; `fathom certify suggest` (decision port required).
- **Receipts**: kind `certification` `{test, target, strength, index_commit,
  reached: [symbols], run_digest}`; `fathom receipt verify` re-derives reach
  with go/types and reports `pass: unverified (no run)` unless `--run-tests`
  is given, in which case it runs that one test and reports the result.
- **Doctor**: row `certifications` — declared count, live count, join rate,
  and "no manifests found" as a skip, never a pass.

## 5. Gaps, incidents, ports, orphans, ordering

- `certify gaps` sources of targets: every invariant row; every incident
  record; every spec id declared in plans; every exported interface with at
  least one implementation (kind `port`), fleet-wide when `--fleet`. Each gap
  line carries the remedy: the marker to add, or the manifest field.
- **Incidents**: an `Outcome:` line may name `certificate <Test>`; the gap
  list treats an incident with such a line whose certificate is `live` as
  closed, and everything else as a gap. The kit's operating doc gains the
  sentence in its own repository (deferred row).
- **Flakiness**: state `flaky` from the last three attested runs; served as
  `suspect` tier in the envelope, never silently live.
- **Orphans**: `Test*` symbols that certify nothing, reach no symbol changed in
  the last 180 days, and never failed in any ingested run. A list, no action:
  test-kit's rule is that a test you stop running is deleted, not hidden.
- **Ordering**: `fathom tests select` ranks tests that certify a target in
  the changed files first, then the existing reach ranking. Advisory; the
  bench row for selection stays open.

## 6. The decision port (Jev), optional and bounded

`fathom certify suggest` asks, for every `Test*` symbol with no declaration,
one batched request: a `choice` over the strength ladder and a `noul` "this
test asserts a behaviour rather than merely executing code". Answers in the
proceed band (≥ 0.9) become suggestions written to
`workspace/artifacts/certify-suggestions.jsonl` and rendered in `certify gaps`
as `suggested: TestX may certify …`; confirm-band answers render as
`unsure`; nothing below. A suggestion is never a claim; a human adds the
marker. With `[decide] provider = none` the verb prints one line saying so and
exits 0. Jev is the design point (sub-second, calibrated); an LLM lane works
offline within the port's existing bounds.

## 7. The judge (Review Kit) and the standard (test-kit)

- test-kit: `#Certifies` and `#Suite.certifies` (ADR-0042 amendment), `surveyor
  validate` accepts it, one example manifest carries it, the docs page names
  the marker grammar as fathom's.
- Review Kit: requirement `testing.certified-targets` with check `{kind:
  "evidence", adapter: "fathom-certify", binding: {kind: "invariant",
  min_strength: "asserts"}, min_tier: "verified"}`, a finding rule for a
  missing certificate with a remedy, and the adapter in `internal/evidence`
  running `fathom certify gaps --json --kind <kind>` through the existing
  fathom bridge runner. Enforcement severity is the organization's.

## 8. Measurements (op rows, never skipped silently)

- TC-M1: join rate ≥ 80 % on fathom's own repo and on test-kit's `examples/`
  after stage 0, else the envelope needs a case-to-symbol contract first.
- TC-M2: ≥ 50 % of fathom's deny-severity invariants carry a live certificate
  two weeks after stage 1, else the declaration path changes.
- TC-M3: the Review Kit requirement fires `deny` on a real change within a
  month of stage 2, else it is theatre.
- TC-M4: suggestions accepted ≥ 50 % of the time, else `certify suggest` is
  removed.
