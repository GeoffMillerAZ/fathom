RO-M0 verdict: CUT. Compacting Bash output is not worth its risk on this machine's agent work. Over 30 days and 137,875 Bash calls, grep-group is the one built-in safe enough to serve compact, and it would remove 0.53% of Bash bytes. Every built-in served together would remove 3.1%. Both are under the 4% bar. An rtk-style lossy filter would save little here and would hide lines the agent goes on to use: the long-file outline, the largest candidate, dropped a line the agent referenced within five calls in 44% of its views. So the lossy track stops at lossless stages and rich enrichment. Readouts keep the parts that are seams rather than byte savings (observe, replay, the claim check, the gate preflight and the config audit), and the Claude Code mods program becomes the main line.

# RO-M0: the corpus reading

Taken 2026-10-18 with `fathom readout replay --days 30 --json` on the deployed binary (v0.15.6 at 08e28994), over
every project's local Claude Code transcripts on the owner's machine, deduplicated by tool-use id, in 35 s. The
output is aggregates only: command heads, counts and bytes, with no command text, output text, path, session or
project name. It is not committed, because it reads the owner's private sessions. The bars were fixed in
`docs/plans/readouts.md` RO-M0 before any rule figure was computed, including the 2026-10-18 amendment that reads
bars (a) to (c) on the projection.

## What it means

1. **Lossy lean is cut.** Bar (c) needs the eligible classes to remove at least 4% of Bash bytes. They remove 0.535%.
   CUT does not depend on bar (b): with every built-in counted, eligible or not, the total is 3.11%.
2. **Agents read whole files to use them.** Bar (a) keeps the outline class (whole-file source reads over 200
   lines are 3.94% of Bash bytes, over the 3% floor), so `read-outline` (RO-8) is not refused. Bar (b) refuses it
   for lean, though. In 280 of its 634 views, a dropped line was referenced within five calls, a 95% Wilson lower
   bound of 0.52 against the 0.90 bar. Design §1's 12.3% ceiling rested 69% on the assumption that a long read
   needs only 60 lines. This reading refutes that assumption.
3. **The rules are mild where they fire.** grep-group matches 18.1% of Bash bytes but drops only 3.0% of what it
   sees. git-diff drops 8.2% and go-test 9.0%. read-outline drops 33.7%, the one class with real removal, and the
   one agents need most.
4. **As matched today, the hook sees almost nothing.** Every rule together would remove 0.09% of Bash bytes as
   matched, against 3.11% projected. The calls are compound:
   - 58% of `sed -n` calls,
   - 92% of `grep`,
   - 97% of `ls`,
   - 99.9% of `go test`.

   RO-28 (quiet prefixes) is what lets any rule see the call, and rich enrichment needs it as much as lean did.
5. **The dispositions in this change.**
   - Lean emission (RO-20), saved raw output (RO-22), the channel reading (RO-M2) and lean per rule (RO-M3) are
     deferred.
   - They revive when RO-M19 reads KEEP, because H7's dedupe view needs the view channel. They also revive if a
     later RO-M0 reading over a new 30-day window reads KEEP.
   - Seam H1 is re-rated Low, with RO-M0 as its sizing reading (G41).
   - RO-21 (the test-run hook folded into the readout hook) is not lossy, and it stays.
   - RO-M1 (14 days of observe) stays, because it gates phase 2's rich enrichment.
6. **Mods phase 1 starts.** Readouts phase 0 has landed: RO-3 to RO-18 are verified and this reading is filed
   (roadmap §9.7, ADR-072 decision 12).

## The corpus

| Figure | Value |
|---|---|
| Window | 2026-09-18T17:30Z to 2026-10-18T14:17Z |
| Projects, sessions, transcript files | 90, 408, 3,982 |
| Bash calls | 137,875 |
| Bash result bytes | 366,574,285 |
| Duplicates skipped (resumed sessions) | 978 |
| Unpaired calls (no result) | 419 |
| Failures (never replaced, ADR-071 decision 2) | 3,402 |
| Persisted outputs, read from their files | 137 of 137 |
| Unparsed or oversize lines | 0 |

## Bar (a): whole-file source reads over 200 lines

918 calls and 14,458,321 bytes are whole-file reads of a source file over 200 lines (`cat`, `sed -n 1,$p`, never
`head` or `tail`). That is 3.94% of Bash bytes, over the 3% floor, so the outline class stays as a built-in in
observe.

## Bar (b): per built-in

These figures are projected: each call is read as its last segment after its leading quiet segments. A view is a
hit the rule would serve, meaning it dropped something and came out smaller than the raw. The reference rate's
denominator is views, never raw hits, because a hit served raw drops no line. As-matched figures sit in the last two
columns.

| Rule | Lossiness | Hits | Views | Would remove (share of Bash bytes) | Diagnostic trips | Unscoped trips | Referenced / views | Wilson lower | Eligible | As matched: hits | As matched: would remove |
|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|
| grep-group@1 | lines | 29,093 | 4,356 | 0.535% | 0 | 0 | 163 / 4,356 | 0.957 | yes | 287 | 0.003% |
| git-diff@1 | lines | 2,075 | 616 | 0.256% | 0 | 147 | 64 / 616 | 0.870 | no | 11 | 0.000% |
| go-test@1 | whole | 6,231 | 1,271 | 0.101% | 0 | 0 | 739 / 1,271 | 0.392 | no | 15 | 0.000% |
| git-show@1 | lines | 749 | 134 | 0.054% | 0 | 23 | 10 / 134 | 0.868 | no | 2 | 0.000% |
| ls-long@1 | lines | 1,085 | 31 | 0.020% | 0 | 0 | 0 / 31 | 0.890 | no (under 200) | 86 | 0.007% |
| git-grep-group@1 | lines | 649 | 188 | 0.015% | 0 | 1 | 2 / 188 | 0.962 | no (under 200) | 8 | 0.000% |
| git-log@1 | lines | 3,317 | 0 | 0 | 0 | 0 | 0 / 0 | n/a | no | 0 | 0 |
| read-outline@1 | whole | 3,214 | 634 | 2.130% | 0 | 574 | 280 / 634 | 0.520 | no | 145 | 0.081% |

Notes on the table:
- **Protected trips.** Diagnostic-stream trips are 0 for every rule, both as matched and projected, so no defect
  blocks. Content-stream trips are 0 as well.
- **Unscoped trips.** These are recorded without a bar. They count the lines an unscoped protected-line rule would
  have tripped on, which is the defect that the stream scoping fixed. As matched, the only one is read-outline, with
  17.
- **Raw reasons.** Most hits are served raw by the never-worse guard: 24,737 for grep-group and 4,960 for go-test.
  Below-floor accounts for 2,526 of read-outline's raw hits and 744 of ls-long's. read-outline also keeps 54 reads
  raw because the session had edited the file.
- **git-log.** git-log serves no view, because these commits carry no trailers for it to fold.

## Bar (c): the program

The only eligible class is grep-group@1, which would remove 1,960,345 bytes, 0.535% of Bash bytes, against the 4%
bar. As matched, no class is eligible and the total is 0. The verdict is CUT.

## (d) Recorded without a bar

The effective shapes classify each call by its last segment after its leading quiet segments, and read a pipeline by
its producer. The quiet list is `cd`, `export`, `mkdir -p`, `sleep N`, an assignment alone, `set`, `source`, `.`
and `tccutil reset`.

| Shape | Calls | Share of Bash bytes | Compound share of its calls |
|---|---:|---:|---:|
| `sed -n` | 16,552 | 21.4% | 57.6% |
| `grep` | 34,739 | 20.5% | 91.9% |
| `cat` | 7,436 | 12.8% | 75.0% |
| (unreadable) | 7,036 | 7.2% | n/a |
| `ls` | 5,843 | 5.4% | 97.3% |
| `git diff` | 3,888 | 3.7% | 86.5% |
| `bash -c` | 5,839 | 3.4% | 21.6% |
| `python3` | 4,777 | 2.9% | 48.6% |
| `go test` | 6,267 | 1.3% | 99.9% |
| `git show` | 1,070 | 1.2% | 95.3% |
| `git log` | 3,542 | 1.1% | 98.7% |

Other figures recorded without a bar:
- **First-unit shapes.** Classified by its first unit instead, a call reads as `cd` for 57.9% of Bash bytes and as
  an assignment for 15.2%. The 2026-10-18 amendment exists to correct that shape.
- **`tccutil reset`.** 5,111 calls, 96.5% of them compound.
- **Re-read proxy.** A whole-file read followed by a `sed -n` of the same file within five calls: 15 of 2,521
  whole reads, and 2 of the 918 long ones. This is not the reference rate. The proxy counts a second read of the same
  file, while a reference also counts a path, `path:line` or identifier from a dropped line, such as an edit's text.
- **Unmatched calls.**
  - No rule: 82,057.
  - Machine-bound, kept raw by ADR-071 decision 3(e): 45,246.
  - Unreadable: 6,616. The top reason class is a command name that is not literal, at 5,107, and then the same
    inside `shell -c`, at 500.
  - Failures: 3,402.
- **Joins.**
  - Declared test commands: matched from the call's directory 8,383 times, and from the built-in list alone 410
    times.
  - `head_stamp`: missed on every past call, by design.
  - Outline: computed 448 times, not computed 186 times.
  - Persisted text: missed once.
