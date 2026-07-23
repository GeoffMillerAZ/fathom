# Phase E — New use cases (vision 11 §3.E)

Tracer bullets only; CLI/gate/ingest surfaces, zero new MCP tools; every
adoption kill gated on law 8's workload precondition.

Design (normative):
- **E2 authorship ledger.** Commit ingestion captures the full message body
  (subject-only today; verified 0/763 stored messages contain a body).
  A deterministic Model-empty pass (ADR-018 oracle class: lexical) parses
  Co-Authored-By trailers into `kind:"authorship"` claims at tier
  `verified`, subject = commit sha, evidence = the trailer line. One CLI
  report (`fathom report --analyzer authorship`) joins authorship share ×
  staleness × complexity per service. Kill: fleet backfill shows no usable
  signal in the join → keep the ledger, retract the risk-correlation pitch.
- **E3 SDD computed progress.** `fathom plans status` — per-plan rollup
  {proven, claimed-unproven, open, deferred-with-trigger} computed by the
  existing verify engine, never asserted; `fathom plans next` — the ordered
  work frontier (open/unproven rows with their {symbol, package} anchors)
  for agents. CLI verbs only. Regression timelines depend on D2's
  claim_events and are deferred to its landing.
- **E4 artifact lifecycle.** `fathom plans archive <plan>`: refuses unless
  (a) every row proven or dispositioned, (b) plan ids appear in CHANGELOG
  or ADR (lexical absorption check), (c) anchors resolve. On success: file
  moves to docs/plans/archive/, a tombstone records the evidence, and the
  pre-push verify gate scopes to active plans only (bounds the
  O(all-plans-forever) gate cost — verify p50 2.2s and growing).
  `fathom plans restore` must exist before the first real archive. Claims
  persistence beyond archival depends on A2 producing production claims.
- **E6 reverse drift.** One report mode: public API surface minus
  spec-bindings coverage = ungoverned surface, per service.

## Tasks

- [x] Commit ingestion captures full message body; store round-trip in the contract suite; backfill on re-ingest proven {id: E2-1, symbol: parseGitLog, package: internal/fusion}
- [x] Authorship trailer oracle → verified-tier claims; deterministic; golden-tested on fixture history {id: E2-2, symbol: CompileClaims, package: internal/authorship}
- [x] authorship report joining share × staleness × complexity; fleet numbers recorded here on first run (see "Authorship report — fleet numbers" below) {id: E2-3, symbol: cmdReport, package: cmd/fathom}
- [ ] plans status: computed rollup via the verify engine; golden-tested against a fixture plan set {id: E3-1, symbol: cmdPlans, package: cmd/fathom}
- [ ] plans next: ordered unproven-row frontier with anchors; deterministic ordering test {id: E3-2, symbol: cmdPlans, package: cmd/fathom, skip_test: true, skip: "anchor shared with E3-1 (same new command family); frontier ordering has its own test"}
- [ ] plans archive with the three refusal criteria + tombstone + restore; refusal cases golden-tested {id: E4-1, symbol: cmdPlans, package: cmd/fathom, skip_test: true, skip: "anchor shared with E3-1 pending the command family landing; archive/restore tests are the coverage"}
- [ ] Pre-push verify gate scopes to active plans (archive/ excluded); gate-cost delta recorded {id: E4-2, symbol: cmdVerify, package: cmd/fathom}
- [ ] Reverse-drift report: api_surface minus bindings coverage {id: E6-1, symbol: cmdReport, package: cmd/fathom, skip_test: true, skip: "report mode over two existing tested analyzers; anchored to the report command that hosts it"}

### Authorship report — fleet numbers (2026-07-23, first run)

`fathom report --analyzer authorship` run against **this repo's own history**
(153 commits, its full log — under the 500-commit `git log` cap), ingested
into a throwaway scratch store under `workspace/tmp/` (never the live fleet
store or daemon):

| metric | value |
|---|---|
| commits (total) | 153 |
| commits co-authored (AI trailer) | 152 |
| co-authored share | 99.3% |
| authorship claims emitted | 152 |
| sources (total) | 514 |
| sources stale (>24h since watermark) | 0 |
| avg cyclomatic complexity (func/method) | 4.78 |
| max cyclomatic complexity | 328 (`internal/store/contract_test.go:16 storeContract`) |

Reading: on this repo, the co-authored share is saturated (99.3%) — nearly
every commit already carries the fleet's `Co-Authored-By: Claude Fable 5
<noreply@anthropic.com>` convention (CLAUDE.md's own documented practice), so
a single-repo run cannot show variance between co-authored and
human-only commits to correlate against staleness/complexity — that
comparison needs the E2 design's "fleet backfill" (multiple services with
materially different co-authorship rates), not yet run. The ledger and
report both work end-to-end (152 claims persisted, evidence = the verbatim
trailer line, join computed and rendered); the risk-correlation question
itself is open pending that fleet-wide run, exactly the kill/keep gate the
design section names.
