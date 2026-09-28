# Doc-questions scenario (KR-10)

The knowledge-retrieval bench slice (vision 38, ADR-049 decision 8; design
`docs/design/knowledge-retrieval.md` §9, plan `docs/plans/knowledge-retrieval.md`
KR-M1). Thirty questions whose answers live in this repository's prose — ADRs,
plans, design docs, intent, usage docs — not in code. Every gold answer below
is a short, verifiable quote or fact, each with the doc and section it lives
in, re-derivable by hand at the commit this file was written against
(`b59bcff3`, 2026-09-28) the same way the home corpus's `truth_at_head`
anchors are (`docs/bench/README.md`).

This file is the scenario only: questions, gold answers, sources. `KR-M1` (op,
gated on this row) is the run itself — two arms (fathom, grep) answer these
against the `search kind: docs` surface versus plain grep, scored against the
gold below, written to `docs/bench/readings/knowledge/doc-questions.md`.

## Criterion registered

Criterion registered here, before any run scores against it — the same
pre-registration discipline `docs/bench/public-loc.md`'s "Pre-registered
criterion for run 1 (H2-2)" uses, so a result cannot be graded by a bar
chosen after seeing it. This is design §9's KR-M1 threshold, stated
operationally:

1. **Success per cell:** the arm's answer contains, as a verbatim substring
   (case-insensitive, whitespace-normalized), the task's `gold` string below.
   Precision/recall are not applicable (single gold string per task); this is
   a pass/fail bar, mirroring the bench's existing `machine_criterion`
   substring convention (`docs/bench/README.md` "Two criteria, one bar").
2. **Pass rate:** over the non-leaked tasks (a task the `none` arm passes in
   at least 2 of 3 trials is `leaked` and excluded, `docs/bench/README.md`'s
   own leak rule), fathom's pass count is **at least** grep's.
3. **Bytes:** fathom's aggregate response payload bytes across the run is **at
   most 1.0×** grep's aggregate bytes.
4. Both bars must hold. Replication floor: 3 trials per cell, matching the
   bench's existing convention (`docs/bench/README.md`).

A run's verdict line: `KR-M1 verdict: PASS` (or `FAIL`, with the failing bar
named), written to `docs/bench/readings/knowledge/doc-questions.md` — the
`docs/plans/knowledge-retrieval.md` KR-M1 row's own `pattern` anchor.

## Questions

| id | Question | Gold | Source |
|---|---|---|---|
| DQ-1 | What table format does fathom use for the doc index's full-text search? | FTS5 | `docs/design/knowledge-retrieval.md` §2 decision 1 |
| DQ-2 | Which SQLite driver does the doc index rely on to avoid CGO? | modernc.org/sqlite | `docs/design/knowledge-retrieval.md` §2 decision 1 |
| DQ-3 | How is BM25 weighted between heading and body text in doc ranking? | 3:1 | `docs/design/knowledge-retrieval.md` §5 |
| DQ-4 | What is the k constant used for reciprocal rank fusion in doc ranking? | 60 | `docs/design/knowledge-retrieval.md` §5 |
| DQ-5 | What flag on a doc unit marks it as living under `docs/plans/archive/` or `tmp/`? | stale_by_location | `docs/design/knowledge-retrieval.md` §3 |
| DQ-6 | Does knowledge retrieval add embeddings or a vector index? | No | `docs/design/knowledge-retrieval.md` §11 |
| DQ-7 | What three store tables does knowledge retrieval add, additive to store format v0? | docs_fts, doc_units, doc_links | `docs/design/knowledge-retrieval.md` §4 |
| DQ-8 | Under backticks, which four target kinds does a doc-to-code link resolve to, per §6's table? | symbol, path, contract key, bare word | `docs/design/knowledge-retrieval.md` §6 |
| DQ-9 | What does `staleness_report` gain to report a doc span whose linked target changed? | doc_drift | `docs/design/knowledge-retrieval.md` §6 |
| DQ-10 | Per §11, does doc drift ever mean a doc is wrong? | No, only that the target moved | `docs/design/knowledge-retrieval.md` §11 |
| DQ-11 | What ADR accepts the knowledge-retrieval design? | ADR-049 | `docs/design/knowledge-retrieval.md` header |
| DQ-12 | Per ADR-049 decision 3, how many tools does the default agent surface hold at most? | eight | `ADR.md` ADR-049 decision 3 |
| DQ-13 | Per ADR-049's kitchen-sink gate, what happens to an item with a letter but no instrument? | It goes to futures.md with a Promote-when | `ADR.md` ADR-049 decision 1 |
| DQ-14 | Per ADR-049 decision 12, is a vector database refused or accepted? | Refused | `ADR.md` ADR-049 decision 12 |
| DQ-15 | Per ADR-050, how many verbs does the system graph's contract-key link model expose? | three | `ADR.md` ADR-050 title |
| DQ-16 | Per ADR-011, what invariant number covers the org-agnostic naming rule? | I19 | `ADR.md` ADR-011 |
| DQ-17 | Per ADR-011, which test mechanically enforces no org-specific names? | TestNoOrgSpecificNames | `ADR.md` ADR-011 |
| DQ-18 | Per ADR-044 decision 1, does fathom ever write the change it is proving? | No, it never writes the change | `ADR.md` ADR-044 decision 1 |
| DQ-19 | Per ADR-044 decision 5, what verdict does a conservation plane with a missing input report? | unproven | `ADR.md` ADR-044 decision 5 |
| DQ-20 | Per ADR-041, what was removed from guidance because a feature must earn its place? | Embeddings | `ADR.md` ADR-041 context |
| DQ-21 | In the plan grammar, what does a `[-]` checkbox mean? | Skipped or abandoned | `docs/plans/README.md` Checkbox semantics |
| DQ-22 | In the plan grammar, which flag makes `fathom verify` the wave gate? | --require-all | `docs/plans/README.md` |
| DQ-23 | Where do all build artifacts, logs, temp files and local DBs belong, per the workspace convention? | workspace/ | `docs/usage.md` §workspace/ convention |
| DQ-24 | Per CLAUDE.md's non-negotiables, what command must stay green as the gate? | task validate | `CLAUDE.md` Non-negotiables table |
| DQ-25 | Per CLAUDE.md, what is the correct way to deploy, never a raw `cp`? | task deploy | `CLAUDE.md` Non-negotiables table |
| DQ-26 | Per CLAUDE.md, a `docs/plans/*.md` box flips to `[x]` only when what proves it? | fathom verify | `CLAUDE.md` Non-negotiables table |
| DQ-27 | Per CLAUDE.md's UI doctrine line, what governs the observatory's markup? | app-kit doctrine | `CLAUDE.md` Non-negotiables table |
| DQ-28 | Per ADR-049 decision 8, what fuses BM25 with in doc ranking? | concern ranking | `ADR.md` ADR-049 decision 8 |
| DQ-29 | Per ADR-049's revisit triggers, what happens if the FTS5 slice misses grep's pass rate after one tuning round? | Re-weigh vision 31's bench before any other retrieval work | `ADR.md` ADR-049 Revisit when |
| DQ-30 | Per §7 of the knowledge-retrieval design, are dependency docs ever fetched from the network? | Never (I56) | `docs/design/knowledge-retrieval.md` §7 |
| DQ-31 | Per §8's tenancy table, what re-indexes on a docs-only change? | Only changed units | `docs/design/knowledge-retrieval.md` §8 |
| DQ-32 | Per §10, what is the cost of giving Jev a role in doc ranking or linking, and is it given one today? | A model call per query on the hottest read path; no role today | `docs/design/knowledge-retrieval.md` §10 |
