RO-M11 verdict: STOP. A post-compaction working-set injection would recover almost nothing. Over the latest 14 days, sessions compacted 20 times, 1.43 a day against a bar of 2. After the median compaction, the agent re-read 1 of the 15 files it had read or edited in the 40 turns before, against a bar of 2. This matches CA-M1's STOP of 2026-09-29: carry-across CA-1 to CA-4 stay deferred, seam H3 (symbol spans after a compaction) is re-rated Low, and RO-42 and RO-M12 are deferred.

# RO-M11: H3's re-scan

Taken 2026-10-18 with `fathom readout replay --days 30 --json` on the deployed binary (v0.15.6 at 518af7f6). This is
CA-M1's method in its Bash-inclusive variant: 40 assistant turns before each compaction and 20 after. It prints
aggregates only and is not committed. Events are the `compact_boundary` lines in the latest 14 days of the window,
from 2026-10-09T17:00Z to 2026-10-18T14:01Z, each counted once by its uuid.

| Figure | Value | Bar |
|---|---:|---|
| Compaction events | 20 (all in main sessions, all with the full 20 turns after) | none |
| Compactions per day | 1.43 | at least 2 |
| Median pre set (files read or edited in the 40 turns before) | 15 | none |
| Median distinct pre-set files re-read in the 20 turns after | 1 | at least 2 |
| Events with at least 2 re-reads | 9 of 20 | none |
| Events with no re-read | 4 of 20 | none |

Both clauses read STOP. A read here is a Read, `read_source`, or a Bash read through the lexer. A re-read also counts
a Grep on a path. Each window is cut at the next boundary.
