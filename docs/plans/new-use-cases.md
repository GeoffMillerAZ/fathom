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

- [ ] Commit ingestion captures full message body; store round-trip in the contract suite; backfill on re-ingest proven {id: E2-1, symbol: parseGitLog, package: internal/fusion}
- [ ] Authorship trailer oracle → verified-tier claims; deterministic; golden-tested on fixture history {id: E2-2, symbol: parseGitLog, package: internal/fusion, skip_test: true, skip: "anchor shared with E2-1 until the oracle package lands; its own golden tests are the coverage"}
- [ ] authorship report joining share × staleness × complexity; fleet numbers recorded here on first run {id: E2-3, symbol: cmdReport, package: cmd/fathom}
- [ ] plans status: computed rollup via the verify engine; golden-tested against a fixture plan set {id: E3-1, symbol: cmdPlans, package: cmd/fathom}
- [ ] plans next: ordered unproven-row frontier with anchors; deterministic ordering test {id: E3-2, symbol: cmdPlans, package: cmd/fathom, skip_test: true, skip: "anchor shared with E3-1 (same new command family); frontier ordering has its own test"}
- [ ] plans archive with the three refusal criteria + tombstone + restore; refusal cases golden-tested {id: E4-1, symbol: cmdPlans, package: cmd/fathom, skip_test: true, skip: "anchor shared with E3-1 pending the command family landing; archive/restore tests are the coverage"}
- [ ] Pre-push verify gate scopes to active plans (archive/ excluded); gate-cost delta recorded {id: E4-2, symbol: cmdVerify, package: cmd/fathom}
- [ ] Reverse-drift report: api_surface minus bindings coverage {id: E6-1, symbol: cmdReport, package: cmd/fathom, skip_test: true, skip: "report mode over two existing tested analyzers; anchored to the report command that hosts it"}
