# Carry-across Reading 0 (CA-M1): transcript scan

CA-M1 verdict: STOP

Plain summary: over the last 14 days the median compaction was followed by 0.5 re-reads of the files touched in the 40 turns before it, against a bar of 2. Re-reads cost about 7k tokens in total across the 8 complete events, and a control at ordinary points in the same session re-reads about as often as the compaction points do. There is no measurable waste for an injection to recover, so thread.go's silence on compact stands. The sample is small (8 complete events, one main session plus one workflow agent), which limits how far the finding generalizes, and the reasons the A/B could not run anyway are listed below.

Scan date: 2026-09-29 01:03 UTC (window 2026-09-15 to 2026-09-29). Bar and stop rule: `docs/design/carry-across.md` section 3 and plan row CA-M1 in `docs/plans/carry-across.md`.

## Headline numbers (design parameters: 40 turns before, 20 turns after, Bash-inclusive reads)

| Measure | Value | Bar |
|---|---|---|
| Compaction events in the 14 days | 9 (8 main session, 1 workflow agent) | design assumed about 551 per 4 days |
| Events with the full 20 post turns (used for medians) | 8 | none |
| Median pre set (files read or edited in the 40 turns before) | 13.5 files | none |
| Median distinct pre-set files re-read in the 20 turns after | 0.5 | at least 2, else STOP |
| Events with at least 2 re-reads | 2 of 8 | none |
| Re-read calls as a share of all reads after compaction | 17.0 percent pooled (9 of 53), 6.3 percent median per event | at least 10 percent, else STOP |
| Re-read then edited within 3 turns (legitimate) | 1 call | none |
| Re-read only | 8 calls | none |
| Re-read token volume (estimate, chars divided by 4) | 6.9k total, 0.87k per event on average | none |
| All read token volume in the same post windows | 49k total | none |
| Median turns and tokens to the next proof event | 99 turns, about 101k tokens (proof found in 8 of 8) | none |

Stop rule: it fires if the median compaction has fewer than 2 re-reads OR re-reads are under 10 percent of post-compaction reads. The first clause fires (0.5 is under 2). The second clause does not fire on the pooled share (17.0 percent) and would fire on the median per-event share (6.3 percent). STOP either way.

## Method

Script: `<scratch>/efficiency/scans/ca_scan.py` (no dependencies beyond the Python standard library; statistics only, no transcript text or file path is printed or stored). Re-run with `python3 ca_scan.py`; outputs `ca-scan.json` and `ca-scan.tables.txt` beside it (none committed). Sensitivity sweep: `sens.sh`. Counterfactual: `ca_control.py`.

- Corpus: every `*.jsonl` under `~/.claude/projects/<encoded-root>`, 1,635 files (233 top-level sessions, 1,402 subagent and workflow transcripts). The window is applied by boundary timestamp (at or after now minus 14 days); by file mtime the same two top-level sessions are active.
- Compaction event: a `system` record with `subtype: compact_boundary` (it carries trigger, preTokens, postTokens). No older summary-record format exists in this corpus (zero `summary` records).
- Turn: one assistant API response, deduplicated by message id, in file order. Pre window is the 40 turns before the boundary line, post window the 20 turns after (cut at the next boundary if one comes first).
- Pre set: files targeted by Read, Edit, Write, MultiEdit, NotebookEdit or the fathom read_source tool, plus, in the Bash-inclusive variant, files named by Bash read commands (cat, head, tail, sed, nl, grep, rg, awk, wc, jq, diff and similar) or written by Bash (redirects, sed -i, tee, cp, mv, gofmt -w and similar).
- Re-read: a post-window Read, Grep on a file path, Glob whose pattern matches a pre-set file, read_source call, or (Bash-inclusive) a Bash read command that names a pre-set file. Counted as distinct files and as calls.
- Reads after compaction (the denominator for the 10 percent bar): all Read, Grep, Glob and read_source calls, plus (Bash-inclusive) Bash calls with at least one read command on a path.
- Split: a re-read call is "then edited" when an edit of the same file occurs in the same turn or the next 3 turns, otherwise "re-read only".
- Tokens: the size of the matching tool result divided by 4. For Bash calls the whole command output is counted, so Bash figures are upper bounds.
- Proof event: the first later Bash command whose verb position is `go test`, `task validate|test|verify|gate`, `fathom verify`, `pytest`, `npm|pnpm test`, `cargo test` or `make test`, with a non-error result and no FAIL or panic marker, or an Edit flipping a plan checkbox from `[ ]` to `[x]`. Turns and tokens are counted from the boundary (assistant output tokens plus tool-result estimate). The test-run hook sensor was not found among the hook records in the transcripts, so the fallback the design allows was used.
- Variants: D0 is the literal design (Read, Grep, Glob, read_source and edit tools only). D1 adds Bash reads and writes. D2 is D1 without log-like targets (.log, .out, .err, background task output) so that polling a log does not count as a re-read. D3 matches files by basename, an upper bound that is robust to `cd` and relative paths but can over-count common names.
- Counterfactual (P12): the same measure applied at control points in the same session, every 20 turns, at least 100 turns from any boundary, inside the window (94 points at the design parameters). Compaction events are placed against a 20,000-draw bootstrap of the mean over 8 random control points.

## Corpus facts

- Compaction events in all transcripts of this project: 14 (12 auto, 2 manual), in 3 files: the main session (12), one earlier session (1), one workflow agent (1). Across all 394 project directories under `~/.claude/projects` (6,306 files) there are 18 events, and the same 9 fall in the 14-day window, all in this project.
- The design's "about 551 pre-compact events per 4 days" is not reproducible from transcripts (9 in 14 days here). The fathom hook log agrees with the transcripts: `~/.fathom/hooks.jsonl` (and its rotated copy) holds 11 InstructionsLoaded records with load reason `compact`, from 2026-09-14 to 2026-09-29. The source of 551 is unverified.
- Sessions active in the window by content timestamps: 2 top-level (this main session and one 2.5 MB session with no compaction) and 995 subagent or workflow transcripts, of which 1 compacted. Compactions per active top-level session in the window: 4.0 mean (8 and 0). The main session started 2026-09-02, has 4,465 turns and has compacted 12 times, one about every 370 turns (roughly every two days), at 925k to 1,000k tokens of context down to 13k to 30k (about 21k on average).
- The main session works through Bash. In the window it made 3,003 tool calls: 83.2 percent Bash, 4.4 percent Agent, 2.1 percent Workflow, 0.9 percent Read (26), 0.2 percent Edit (5), 1.5 percent Write (45), and no Grep or Glob calls. About a third of Bash calls (790 of 2,498) are file reads. The literal design measure (D0) sees zero reads after any compaction in the main session, so it cannot measure this workflow; that is why D1 is the reported variant.
- Claude Code attached 1 to 5 file references after each compaction (36 in total, 2.6 on average). None of them is in the 40-turn Bash-inclusive pre set. Inference, not verified here: the harness re-attach follows Read-tool state, which this session seldom uses.

## Per event (D1, 40 before, 20 after)

Rows before 2026-09-15 are outside the window and shown for context. The last row is this session's own compaction, about two minutes old at scan time, so it has 19 of 20 post turns and no proof event yet; it is excluded from the medians.

| date (UTC) | in 14d | source | trigger | context before / after | pre set (files) | re-read files | re-read calls | reads after | share | re-read tokens | then edited / only | turns, tokens to proof |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 2026-08-27 | no | main session | auto | 1000k / 13k | 19 | 3 | 6 | 9 | 0.67 | 1351 | 2 / 4 | 23, 23k |
| 2026-09-13 | no | main session | auto | 968k / 19k | 19 | 2 | 4 | 26 | 0.15 | 2681 | 2 / 2 | 18, 70k |
| 2026-09-13 | no | main session | manual | 926k / 19k | 11 | 4 | 3 | 9 | 0.33 | 3250 | 0 / 3 | 28, 114k |
| 2026-09-14 | no | main session | auto | 967k / 23k | 20 | 0 | 0 | 7 | 0.00 | 0 | 0 / 0 | 31, 30k |
| 2026-09-14 | no | main session | auto | 968k / 24k | 17 | 0 | 0 | 3 | 0.00 | 0 | 0 / 0 | 129, 172k |
| 2026-09-18 | yes | main session | manual | 925k / 18k | 12 | 0 | 0 | 4 | 0.00 | 0 | 0 / 0 | 117, 256k |
| 2026-09-19 | yes | main session | auto | 969k / 22k | 16 | 2 | 2 | 11 | 0.18 | 1005 | 1 / 1 | 1, 1k |
| 2026-09-22 | yes | main session | auto | 968k / 21k | 24 | 3 | 3 | 9 | 0.33 | 3724 | 0 / 3 | 89, 177k |
| 2026-09-24 | yes | main session | auto | 966k / 29k | 2 | 0 | 0 | 6 | 0.00 | 0 | 0 / 0 | 14, 15k |
| 2026-09-25 | yes | main session | auto | 970k / 22k | 14 | 0 | 0 | 4 | 0.00 | 0 | 0 / 0 | 109, 146k |
| 2026-09-26 | yes | main session | auto | 967k / 23k | 11 | 0 | 0 | 2 | 0.00 | 0 | 0 / 0 | 48, 55k |
| 2026-09-26 | yes | workflow agent | auto | 967k / 13k | 13 | 1 | 3 | 9 | 0.33 | 1660 | 0 / 3 | 135, 52k |
| 2026-09-28 | yes | main session | auto | 969k / 21k | 14 | 1 | 1 | 8 | 0.12 | 548 | 0 / 1 | 116, 178k |
| 2026-09-29 | yes | main session | auto | 969k / 26k | 5 | 0 | 0 | 7 | 0.00 | 0 | 0 / 0 | none (19 post turns) |

Re-read files can exceed re-read calls because one `cat a b c` call reads several files.

## Variants at the design parameters (window, 8 complete events)

| Variant | Median re-read files | Events with 2 or more | Pooled share | Median share | Verdict |
|---|---|---|---|---|---|
| D0 literal (Read, Grep, Glob) | 0 | 0 | undefined (0 reads) | undefined | STOP |
| D1 Bash-inclusive | 0.5 | 2 | 17.0 percent | 6.3 percent | STOP |
| D2 D1 without log-like targets | 0.5 | 1 | 15.1 percent | 4.5 percent | STOP |
| D3 D1 by basename (upper bound) | 0.5 | 2 | 18.9 percent | 9.1 percent | STOP |

Over all 14 events in the corpus (13 complete) D1 gives a median of 1 re-read file, 5 events with 2 or more, pooled share 20.6 percent, and still STOP.

## Counterfactual: is the excess caused by compaction?

Mean distinct pre-set files re-read in the 20 turns after a point, D1, 40 before:

| | Compaction events (n 8) | Control points (n 94) |
|---|---|---|
| Mean re-read files | 0.875 | 0.745 (bootstrap 95 percent for the mean of 8: 0.125 to 1.5) |
| Median re-read files | 0.5 | 0 |
| Mean re-read tokens | 867 | 380 |
| Share of points with 2 or more re-reads | 25 percent | 18 percent |

Probability that 8 random control points average at least as many re-reads as the compaction events: 0.39 (D1), 0.50 (D2), 0.64 (D3). Compaction adds roughly 0.13 re-read files and about 490 tokens per event over the ordinary baseline, and the difference is not distinguishable from noise. At a 100-turn pre window: compaction 1.375 files against control 0.865 (p 0.13, D1); D3 1.75 against 1.19 (p 0.12).

## Sensitivity to the window sizes (window events, complete only)

| Pre / post turns | D1 verdict (median files, pooled share) | D3 upper bound verdict |
|---|---|---|
| 40 / 20 (design) | STOP (0.5, 17.0 percent) | STOP (0.5, 18.9 percent) |
| 40 / 40 | STOP (1.0, 12.0 percent) | STOP (1.0, 14.8 percent) |
| 20 / 10 | STOP (0, 21.4 percent) | STOP (0, 25.0 percent) |
| 100 / 20 | STOP (1.5, 24.5 percent) | BUILD (2.0, 32.1 percent) |
| 200 / 20 | STOP (1.5, 26.4 percent) | BUILD (2.0, 39.6 percent) |
| 1000 / 20 | BUILD (2.5, 45.3 percent) | BUILD (2.5, 54.7 percent) |

The verdict flips only when the pre window is widened well beyond the design's 40 turns and, at 100 to 200 turns, only under the basename upper bound. A larger pre set overlaps a later read by chance, and the control shows the same growth at non-compaction points, so the flip is not evidence of compaction waste. At 200 and 1000 turns no control points remain to test (the pre window spans the window's turns).

## What this means for the plan

- CA-M1 reads STOP. CA-1 through CA-5 (the injection, the switch, the arm assignment, the A/B, the I12 amendment) should not start; thread.go's deliberate silence on compact stands and is recorded as confirmed, citing this reading.
- CA-M2 could not have run in any case. It needs at least 150 compactions per arm (300 in total). The rate here is 9 in 14 days (0.64 per day), about 470 days for 300. And the arm is assigned by session id hash, but 8 of the 9 events come from one session, so both arms cannot be populated.
- If the owner wants to keep the question open cheaply, the stated stop rule already handles it: re-run this scan when compaction volume changes (for example more concurrent long sessions) with `python3 ca_scan.py`. That needs no product change.

## Limits

- Small sample: 8 complete in-window events from one long session and one workflow agent. The bootstrap intervals are wide, and the verdict is a decision on the pre-registered rule for this owner's traffic, not an estimate for other users.
- Bash parsing is heuristic (verbs, redirects, path-like tokens, no `cd` tracking); D3 bounds the miss rate from above and does not change the design-parameter verdict.
- Token figures are estimates from result length, not model-side accounting.
- Subagent transcripts almost never compact (1 of 1,402), so their reads are not part of the post-compaction measure by construction.
- The live session file was read as a snapshot; its newest event is incomplete as noted.
