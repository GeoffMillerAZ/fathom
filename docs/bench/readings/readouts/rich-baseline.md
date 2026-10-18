RO-M7 baseline: 46.8% (95% Wilson interval 42.3% to 51.3%) of the test-run hook's failing-run rows are followed within three calls by a touch of a file or test the row names, over 470 located rows between 2026-10-01 and 2026-10-18. Agents already act on a failing run about half the time with no enrichment. Rich mode's failure lines therefore need at least 56.8% on RO-M4's identical count to earn their place (10 points of lift per record kind). If RO-M4 counts rows a different way from this baseline, it is not comparable.

# RO-M7: RO-M4's baseline

Taken 2026-10-18 with `fathom readout report --baseline --json` on the deployed binary (v0.15.6 at 518af7f6), over
both generations of the owner's hook log and the local transcripts. It prints aggregates only and is not committed.
This reading is filed now, before RO-23 lands, because the hook log rotates and these rows would leave it.

| Figure | Value |
|---|---:|
| Window (the first row's `at` to the last) | 2026-10-01T06:51Z to 2026-10-18T11:26Z, 17.2 days |
| Failing-run rows | 739 |
| Located in a transcript | 470 |
| Touched within three calls (a Read, Edit, Write or MultiEdit of a named file, a shell read of it, or a command naming a failing test) | 220 |
| Share, with its 95% Wilson interval | 46.8% (42.3% to 51.3%) |
| Not computable, counted apart and never as untouched (I38) | 269: 217 rows name no file or test, 52 have no matching tool call, 0 transcripts gone |

Notes:
- **How rows are located.** Test-run rows carry no `tool_use_id` (O1210). A row is located by its session, its
  invocation and its scope, at the nearest result within 30 s. An ambiguous match counts as having no tool call.
  RO-M4 must locate rows the same way, or the test-run row must gain `tool_use_id` before RO-23 starts logging
  enriched rows, so that both counts read the same key.
- **Rows that name nothing.** These 217 rows are mostly failures with no failing test id, such as build errors, which
  name no file the baseline could match.
