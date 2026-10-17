EXP-M3 verdict: STOP

# E3 reading: do fathom's rows plus a fast model's labels answer real questions as well as reading the files (EXP-M3)

Run under `exp-preregistration.md` section 9 (EXP-1, fathom fd091c4f) as amended by `exp-preregistration-2026-10-17.md` section 2 (commit 7c688c49), 2026-10-17. Ten fresh agent sessions on the same model, one per question and arm. The table arm used only `bin/fathom` against the scratch store (`find_callers`, `blast_radius`, `test_impact` through `serve-mcp`, and `tests select`) and one live `TestDecideExpE3` run per question through `decide.Open` (typesafe over OpenRouter). The reading arm read the files of fathom at fd091c4f. A Claude reviewer graded both answers blind against the reference answers, which were written on 2026-10-16 before either arm ran, and recorded every grade before unblinding. Aggregates only; no question's answer, span text or row name is recorded here.

The bar: KEEP when the table is at least as correct as the reading arm on at least 4 of 5 questions at no more than half its context tokens, STOP otherwise. The table was at least as correct on 3 of 5 questions, and two of those three are ties where both arms were wrong. It used no more than half the reading arm's context on none of the 5. Both halves of the bar fail, so the verdict is STOP.

## Per question

Context tokens are the session's largest per-turn context (input plus cache-read plus cache-creation tokens) minus its first turn's, which is what the arm loaded beyond the shared prompt. The ratio is table over reading. Shares are of the rows selected.

| Question | Rows in | Rows selected | Computed share | Residue share | Judged rows (not consulted) | Table grade | Reading grade | Table context | Reading context | Ratio | Table USD | Reading USD |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Q1 | 51 | 42 | 0.00 | 1.00 | 42 (0) | wrong | partial | 19,082 | 26,854 | 0.71 | 0.0013 | 0 |
| Q2 | 1 | 1 | 0.00 | 1.00 | 1 (0) | correct | correct | 22,096 | 8,140 | 2.71 | 0.00003 | 0 |
| Q3 | 56 | 55 | 0.00 | 1.00 | 54 (1) | wrong | wrong | 44,647 | 44,535 | 1.00 | 0.0014 | 0 |
| Q4 | 32 | 29 | 0.00 | 1.00 | 29 (0) | wrong | wrong | 34,382 | 18,002 | 1.91 | 0.0008 | 0 |
| Q5 | 91 | 50 | 0.00 | 1.00 | 49 (1) | wrong | partial | 35,047 | 14,430 | 2.43 | 0.0012 | 0 |
| All | 231 | 177 | 0.00 | 1.00 | 175 (2) | 1 correct, 4 wrong | 1 correct, 2 partial, 2 wrong | 155,254 | 111,961 | 1.39 | 0.0046 | 0 |

At least as correct: Q2, Q3 and Q4 (3 of 5; Q3 and Q4 because the reading arm was also wrong). At no more than half the context: none. The 54 pruned rows: 22 were not a Go function or method, 3 were not in the index, and 29 (all Q5) were past `max_rows`. The two rows not consulted passed one request's 16 KiB-a-question bound.

Spend: 5 requests, all with provider-reported usage, US$0.0046 in all, each question under its US$0.10 cap. Latency 387 to 541 ms per request. The reading arm made no model call.

## In plain language

No. Fathom's rows plus Jev's labels answered one of five real questions correctly, the one whose answer was a single caller that fathom's caller list already named; reading the files answered that one too, and got two more half right. The table arm also cost more context, not less: 39 percent more over the five questions, and more than the reading arm on four of them (on Q3 by 112 tokens, about even). Fathom computed nothing: on all 177 selected rows no section-4 fact (an unchecked error, `%w` wrapping, a test that only executes, a locked write, a threaded context) answered the question, so every row went to Jev. That share is the measurement the `ask` spine's compute-first step (AQ-4) rests on, and on these questions it was zero.

Where the table arm failed:

- **Wrong judged labels (Q1, Q3, Q5).** On Q1 the rows held six of the seven reference paths under their own keys, and Jev labelled three of those six false. On Q3 it labelled 31 of 54 rows true, and the grader found none of them right. On Q5 all three reference functions were in the rows and labelled true, but three unrelated functions were labelled true beside them. Of Jev's 51 true labels over the five questions, 47 sat in the 0.5 to 0.9 band and 4 at 0.9 or above.
- **A missing row source (Q3, Q4).** On Q3 the table arm anchored on the obligations serve core, and the behaviour the question asks about lives in the MCP receipt that wraps it; with no `search` or `read_source`, nothing in the arm's tool set leads from a behaviour to the code that implements it, so the one reference test never entered the rows. On Q4 the row source was `blast_radius` with `kind: "delete"` on the shared type, which listed 32 referents and only one of the seven reference sites.
- **Fathom-tool gaps it hit.**
  - `find_callers` serves a call made inside a package-level function-literal variable (a test seam) as the package's shared `init` key, and lists every `init()` declaration of the package as a caller: 116 direct callers for a function with 6 real non-test callers. One Q1 reference path reached the judge step only as an unrelated `init` span. Filed as O1124.
  - `blast_radius` `kind: "delete"` on a Go type lists call-shaped uses of the type and misses functions that take it in a signature, under `checked_edges: typed` with 0 typed referents. That is where Q4's six missing sites went.
  - `blast_radius` `kind: "delete"` referents carry a file, a line and a bare name but no symbol key, so the agent had to rebuild keys by hand; its first two dry runs pruned all 32 and then 18 of the 32 rows as not in the index.
  - Probing after grading found a third limit, though the arm never hit it: `test_impact` on the Q3 receipt reports 1,186 reaching tests, prints 238 in the order returned and elides 948, and the one test that pins the behaviour is among the elided.
