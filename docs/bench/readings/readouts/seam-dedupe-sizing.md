RO-M19 verdict: STOP. Over 30 days, results that repeat an earlier one are too small to serve as pointers. Exact repeats of Bash and Read results in the same session, plus Read spans wholly inside an earlier span of an unchanged file, are 0.40% of Bash and Read bytes, against a 1% bar. Most repeats are short outputs, such as the line a compound `cd` command prints. Seam H7 (the dedupe view) is re-rated Low and moved to a future (F87). RO-47 and RO-M20 are deferred. With this STOP, a later RO-M0 reading that reads KEEP is the only trigger left for the view channel (RO-20, RO-M2).

# RO-M19: H7's sizing

Taken 2026-10-18 with `fathom readout replay --days 30 --json` on the deployed binary (v0.15.6 at 518af7f6), over
every project's local Claude Code transcripts on the owner's machine. It prints aggregates only and is not committed.
The window runs from 2026-09-18T17:30Z to 2026-10-18T15:30Z. The bar was fixed in `docs/plans/readouts.md` RO-M19
before the run.

| Figure | Results | Bytes |
|---|---:|---:|
| Bash results (the dedupe section's base: transcript text, failures and empty results never repeat) | 138,283 | 322,932,633 |
| Read results | 5,930 | 41,653,992 |
| Bash and Read, the denominator | | 364,586,625 |
| Bash exact repeats inside one parallel batch | 16 | 1,048 |
| Bash exact repeats across turns | 3,432 | 162,961 |
| Read exact repeats across turns | 41 | 48,601 |
| Read spans inside an earlier span or whole-file shell read, across turns | 152 | 1,247,141 |
| All repeats | 3,641 | 1,459,751 |
| Share of Bash and Read bytes | | 0.40% (bar: at least 1%) |
| Recovery repeats, excluded (the same command within five Bash calls) | 340 | 28,640 |

Notes:
- A repeat counts only after the session's last `compact_boundary`. A compaction drops the earlier result from
  context, so a repeat across one is not waste.
- Most repeated Bash results come from `cd` compounds: 1,873 calls and 88,702 bytes. Their output is a short
  path or status line.
- Contained Read spans are the one real class, at 1.25 MB. Even served whole, they are 0.34%.
- The denominator is the dedupe section's own Bash base. Replay's Bash total, which adds persisted-output text, is
  367.3 MB. Against that figure the share is lower still.
- **Not taken:** the row's PostToolBatch capture (what one input carries, and whether its output can replace a
  result). A capture needs a live session, and the byte bar already reads STOP, so no build row would use the
  answer. F87 names the capture as the first step if it is ever promoted.
