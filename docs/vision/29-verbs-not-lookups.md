# 29 — Verbs, not lookups: compete on the answer grep cannot give, and make every answer falsifiable

Status: ACCEPTED 2026-09-14 (ADR-035). Plan: `docs/plans/verbs.md`.
Builds on 28 (receipts), 27 (the strengths bench), 23 (the doctrine plane),
02 §4.7 (edge kinds grow one at a time), ADR-028 (memory recalls, the index
asserts), ADR-034.

## 0. The verdict in one paragraph

Three independent sources now say the same thing. The home bench with the
tools present: given everything, the agent greps first and fathom pays
1.6–2.7× the payload for lookups it does not win. The lookup pilot with the
competitors' tools present: five arms indistinguishable on accuracy, grep
cheapest, fathom ahead only on blast radius, concern search and verify-shaped
questions, and weak on its own governance lookups. The literature
(arXiv 2608.13568): LSP-shaped tools cost capable models more than grep on
localization while winning on completeness. So the product is not a better
lookup. It is the answer grep cannot produce — what breaks, who calls,
which tests reach, what governs, whether it is done — served as **verbs**
with a receipt a stranger can check, from an index a type checker has
graded. This vision promotes those verbs to the agent's hot loop, repairs
the one differentiated tool the bench caught being wrong (obligations),
turns governance from advice into a test, makes every answer falsifiable
without fathom, and takes fathom onto a corpus it did not write.

## 1. What the research and the readings found (the facts)

| # | Fact | Source |
|---|---|---|
| R1 | The agent surface is nineteen lookups; `blast_radius`, `review_diff`, `test_impact`, `rename_impact`, `reachable`, `diff_impact` are registered but not on it | `internal/mcp/surface.go` |
| R2 | On the home bench the gold token came from Grep 0.23, Bash 0.18, obligations 0.15; on the verify-reachability task every grep arm failed and the fathom arm passed 3/3 | `docs/bench/README.md` (run 20260913T154217Z, M6 row) |
| R3 | `obligations` for `internal/extractor/goextract.go` served six plan-row bindings from other languages and displaced 65; the governing `internal/CLAUDE.md` rule was not served | O23, E1 in `docs/design/bench-pyramid.md` |
| R4 | M1: 64 cells, two corpora, zero violations on either arm — advice on a careful agent is unfalsifiable | `docs/bench/README.md` (voided rows, but the null is the same with tools present so far) |
| R5 | Index accuracy against go/types: callers 1.00/1.00; references 1.00/0.997; gopls 1.00/0.96; Serena 1.00/0.79; graphify 0.95/0.93 — measured once, by hand, for $0 | `docs/bench/competitors-ledger.jsonl` |
| R6 | Typed edges are Go-only; twelve tree-sitter languages carry syntax-only edges with no visible resolution tier | ADR-021, `internal/extractor` |
| R7 | M8 (fleet blast radius) is unrunnable: no edge crosses a process boundary | `docs/vision/10-rearchitecture.md` |
| R8 | The harness's own auto-memory carried a code claim as settled fact; the custodian audits CLAUDE.md prose, not memory files | this session's memory index; `fathom config audit` |
| R9 | SCIP is the settled interchange format (Sourcegraph's public index, scip-go/-typescript/-python maintained by others); Kythe's team was cut; Bloop shut down — the index is a commodity, the layer above it is not | sweep: code-intel-infra |
| R10 | Localization research (LocAgent 92.7 % file-level on a syntax-only graph; SHERLOC: agents spend about half their tool budget locating) — fathom has a richer graph and no localize verb | sweep: agent-tooling |
| R11 | Greptile sells a graph-grounded PR review; fathom's `review_diff` is deterministic and cited and reaches no human | sweep: code-intel-infra |
| R12 | Long Code Arena's bug-localization track and Terminal-Bench's SWE subset are corpora fathom does not own; BC-15 is still open | sweep: governance-verification |

## 2. The reframes (three theses)

1. **Stop competing on lookup.** A new agent-surface tool must claim
   precision, completeness, freshness, governance or falsifiability — never
   token savings versus grep. The lookups stay for the shapes grep cannot
   answer; the verbs lead.
2. **Governance moves from advice to enforcement.** A rule that can be
   compiled to a failing test is served at the edit *and* fails the gate;
   the advisory channel stays for prose that cannot compile.
3. **fathom's memory product is the adjudicator of claims it did not
   make.** The custodian audits every auto-loaded file — CLAUDE.md,
   AGENTS.md, harness memory — for code-fact sentences and marks each
   anchored-fresh, drifted (with the receipt), or unanchorable.

## 3. The program

| Wave | What | Row | Effort |
|---|---|---|---|
| A | Promote `blast_radius`, `review_diff`, `test_impact`, `rename_impact` to the agent surface, add `safe_delete_check`; opening moves name a verb first; `gold_source_tool` on every bench cell; the L0 index oracle and an L1 payload replay in `task validate` | M11, instrument | S |
| B | `obligations` ranks by governing scope, splits rules from plan-row claims, inlines the cited lines, reports displaced tiers | M10 | M |
| C | A `layer-rule` obligation kind compiled to a generated test (`fathom rules gen-test`); a Ruleset template | M13 | M |
| D | The freshness probe: scripted edits at HEAD, L0 continuously, graphify `--watch` once per release; SB-51 finished | M7 | S |
| E | The receipt envelope: resolution tier on every edge and answer; `fathom receipt verify` re-derives a claim from a bare checkout with go/types and git alone | M11b | L |
| F | PR-native receipted review: `fathom review --format github` and one deterministic comment per PR | M15 | L |
| G | A corpus fathom does not own: Long Code Arena bug localization with and without the surface; a `localize` verb measured on it | M12, M16 | L |
| H | The custodian over harness memory: an unanchored-code-fact detector across auto-loaded files, behind a flag until precision ≥ 0.8 | M14 | M |
| I | `service_ref` edges (HTTP routes first) so blast radius crosses a process; `fathom export scip` | M8, M9 | L |
| J | `fathom tests select` from the coverage join; mutation reports as a gate input | M12b, M2 | L |

Each row carries its versus, its oracle, its predicted win and no-effect,
and a pre-registered kill criterion in `docs/plans/verbs.md`. Paid rows
stay inside the pilot tier; the free gates run on every commit.

## 4. What fathom refuses (from the do-not list, now doctrine)

Symbol-mutating tools (the index asserts, the agent edits). A general
cross-reference ontology or query DSL. Embeddings as a primary retrieval
path (a nearest neighbour carries no receipt). A dataflow or taint engine
(ingest Semgrep's and CodeQL's findings as governed claims instead).
Language count as a headline. Another raw lookup pitched on token savings.
Another payload-trimming pass on the lookups. Any citation of a voided
reading. Token or cost claims before a two-arm payload reading. A passing
task standing in for a passing tool. An agent-facing memory write that
commits. Self-attestation checkpoints. Runtime guardrails, IDE plugins,
debugger attach. A human-browsable graph as the adoption wedge.

## 5. What would falsify this vision

- The promoted verbs are the gold source on fewer than one in five of the
  tasks they were promoted for: the promotion reverts and the verdict says
  "can answer blast radius when told to, and is not told to".
- The repaired `obligations` does not put the governing rule in the top
  three for 80 % of forty hand-labelled files: it serves only exact scope
  bindings and stays silent otherwise.
- A compiled layer rule catches nothing in the last 200 commits and
  nothing in thirty days of dogfood: governance stays advisory.
- Independent re-derivation reproduces fewer than 95 % of sampled
  receipts: the envelope is decoration and is removed.
- The external corpus shows no lift with a confidence interval that
  includes zero: the verdict publishes the null and BC-15 closes on it.

**Status, 2026-09-17 evening.** Fired: M7's kill clause (VB-18) — freshness
leaves positioning. Closed on evidence: M11b held (50 of 50 receipts
re-derived, VB-21); M8 held after the AST extractor (VB-31); M12b and M14
did not meet their bars and stay advisory-only (VB-35, VB-29). Stay open:
M13's gated-versus-advisory governance pilot (VB-16 — the 200-commit
replay caught nothing and the pilot itself has not run) and M15's
historical-PR review (VB-24 — 5/42 (11.9%) strict-new hits against a 30%
floor, unflipped); the M10 obligations claim (VB-12) is on hold until
ADR-037's raw-guide fallback is re-measured (guards-pilot-6 second
reading). Two product moves shipped in response this round: ADR-037
(`obligations` degrades to a raw guide section instead of silence, plus
the cross-tier relevance-ranking fix that stopped a bare ancestor-token
hit from blocking a farther, strongly relevant rule) and ADR-038 (a
compiled guard or layer rule denies a violating PreToolUse edit outright
instead of only advising after it lands). Next measurement: the guards
corpus, not yet run.
