# XM-M1: sub-agent re-derivation over 14 days

XM-M1 verdict: BUILD

Reading of the row in docs/plans/exploration-maps.md (XM-M1, bar from docs/design/exploration-maps.md section 5 and 8): the share of sub-agent file reads that hit files the parent read or edited before spawning, split into followed by an edit of that file (legitimate) and not (re-derivation); below 15 percent of sub-agent reads kills handoff maps. Scan as of 2026-09-28T18:08:47 local, window 14 days, script `<scratch>/efficiency/scans/xm_m1.py`, raw numbers `<scratch>/efficiency/scans/xm-m1.json` (neither committed). Statistics only, no transcript text was printed or stored.

## Verdict

On the row as written (parent history unbounded, repo-relative file identity, Read plus file-reading Bash) re-derivation is 36.1% of sub-agent file reads (95% cluster bootstrap 32.4% to 39.7%), against a kill line of 15%. The kill does not fire, so the literal verdict is BUILD. The verdict is fragile and the reason matters more than the token: the pass is carried by stale parent knowledge, not by what a handoff map would hand over. Only 2.8% of reads hit a file the parent touched in the hour before spawning, and 15.1% hit a file the parent read since its last compaction (interval 11.2% to 19.5%, straddling the bar). Every sub-agent in the window belongs to one long-running parent session (27 days, 12 compactions), so this is one orchestrator pattern, not a population.

The larger finding for session efficiency is sideways, not downward: re-derivation against files any earlier agent in the same session (parent or sibling) had already read is 69.9% of reads and 15.90M tokens, 32.0% of all sub-agent tool_result volume. That is fleet-level re-reading of a hot core (ADR.md, Taskfile.yml, CHANGELOG.md, go.mod and a few design docs), which a parent-to-child map does not address but a shared or persisted map, or a trimmed onboarding set, would.

## Method

- Corpus: all main and sub-agent transcripts under the project directory. Agents counted: 998 spawned inside the window (823 workflow agents under subagents/workflows/, 175 Agent-tool agents, of which 45 nested at depth 2 or 3), all under 1 parent session. Orphans (parent transcript missing): 0. Agents with at least one resolved file read: 846.
- Parent and spawn time: an Agent-tool child uses the meta toolUseId to find the parent tool_use timestamp; a workflow child has no toolUseId, so its first record timestamp is the spawn time and the parent is the session that ran the Workflow. A nested child takes its parentAgentId transcript as parent (the ancestors variant unions the whole chain). "Parent knew a file" means a successful Read, mcp read_source, file-reading Bash, or an edit (Edit, Write, NotebookEdit, sed -i, redirect, tee, cp or mv target) dated before spawn.
- A read event is one (call, file) pair: a Read call, a read_source call, or a Bash simple command from cat, head, tail, sed, awk, grep, egrep, fgrep, rg, less, nl, jq, cut, sort, diff, git show rev:path, git blame, or a `<` redirect, with an explicit file operand. Heredocs are stripped, cd is tracked, and directory, glob, variable and pipe-only operands are skipped. Errored calls are skipped.
- File identity: sub-agents work in per-wave clones (scratchpad clones and .claude/worktrees), so raw absolute paths almost never match the parent. Paths under the main checkout, a worktree or a scratchpad clone fold to a repo-relative key when that relative path is tracked in fathom (git ls-files plus 45 days of history). Fold precision proxy: 93.7% of Bash repo-class reads and 99.8% of Read repo-class reads land on tracked files. A strict-path variant is reported as the floor.
- Split: a hit is legitimate when the same sub-agent later edits that file (by folded key), and re-derivation when it does not. Percentages are over all resolved repo-file read events of the sub-agents. Tokens are tool_result text bytes divided by 4 (Bash output split evenly across the files of one call); 248 reads were persisted-output stubs, so volumes are a lower bound.
- Uncertainty: 95% intervals come from a cluster bootstrap (2,000 draws, seed fixed) over workflows and individual Agent-tool agents (about 192 clusters).

## Headline numbers (row as written)

| Quantity | Value |
|---|---|
| Resolved repo-file read events by sub-agents | 28,362 (16,165 unique agent-file pairs) |
| Hit a file the parent read or edited before spawning | 49.5% (14,039 events) |
| Legitimate (followed by an edit of that file) | 13.4% (3,794 events) |
| Re-derivation (not followed by an edit) | 36.1% (10,245 events), interval 32.4% to 39.7% |
| Of the re-derivation, the parent had read it / only edited it | 9,299 / 946 |
| Re-derivation over unique agent-file pairs | 36.3% |
| Tokens read by sub-agents (repo files) | 22.68M |
| Tokens of re-derivation reads | 7.58M (33.4% of repo-file read tokens, 15.2% of all 49.69M sub-agent tool_result tokens), about 8,955 per reading agent |
| Tokens of legitimate hits | 3.59M |
| Agents with 5 or more reads at or above 15% re-derivation | 652 of 736 (median agent 44.0%) |
| Parent known repo files at spawn (median, p90, max) | 621, 711, 715 of 5,899 tracked, yet 49% of child reads hit them: reads are concentrated on a hot core |

## Sensitivity: what the verdict depends on

Re-derivation share of sub-agent reads (kill line 15%). Rows above the line pass the bar; rows below would kill.

| Variant | Reads | Re-derivation | 95% interval | Re-derivation tokens | Bar |
|---|---|---|---|---|---|
| Headline: parent history unbounded, folded repo files, Read + Bash + read_source | 28,362 | 36.1% | 32.4% to 39.7% | 7.58M | pass |
| Same, every file class (adds scratch, tmp, .claude paths) | 40,148 | 26.2% | 23.1% to 29.2% | 7.96M | pass |
| Same, tracked files only | 26,857 | 37.5% | 33.7% to 41.3% | 7.51M | pass |
| Same, minus CLAUDE.md, AGENTS.md, ADR.md, plans and intent READMEs | 27,050 | 34.6% | 30.9% to 38.1% | 6.65M | pass |
| Bash reads only | 24,425 | 39.7% | 36.0% to 43.8% | 6.24M | pass |
| Ancestor chain instead of immediate parent | 28,362 | 36.9% | 33.3% to 40.6% | 8.15M | pass |
| Fleet: parent plus every earlier agent of the session (not the row; context) | 28,362 | 69.9% | 63.3% to 74.9% | 15.90M | pass |
| Parent touched it within 24 hours before spawn | 28,362 | 17.5% | 13.6% to 20.7% | 4.05M | pass (interval straddles) |
| Parent touched it since its last compaction (files still in its context) | 28,362 | 15.1% | 11.2% to 19.5% | 3.53M | pass (interval straddles) |
| Read tool only (the row read narrowly) | 3,937 | 14.2% | 11.5% to 16.9% | 1.33M | below (interval straddles) |
| Parent touched it within 6 hours before spawn | 28,362 | 9.1% | 5.7% to 12.9% | 2.17M | below |
| Since last compaction, Read tool only | 3,937 | 7.3% | 4.7% to 9.6% | 0.75M | below |
| Parent touched it within 1 hour before spawn | 28,362 | 2.8% | 1.7% to 4.3% | 0.74M | below |
| Within 1 hour, Read tool only | 3,937 | 2.6% | 1.4% to 4.1% | 0.26M | below |
| Parent touched it within 15 minutes before spawn | 28,362 | 1.3% | 0.7% to 2.1% | 0.37M | below |
| Raw absolute paths (clones never match), repo files | 28,362 | 7.7% | 5.3% to 11.1% | 1.47M | below |
| Raw absolute paths, every file class | 40,148 | 6.1% | 4.4% to 8.5% | 1.86M | below |

Reading the table: the row passes wherever the parent is credited with everything it ever read in a 27-day, 12-compaction session, and on the fleet view. It fails when identity is raw paths (an artifact of clones, not a real absence), when the count is narrowed to the Read tool (14.2%, interval 11.5% to 16.9%), and whenever the parent must have touched the file recently enough for a handoff map to plausibly carry it (within 6 hours or less). Since-last-compaction sits on the bar (15.1%, interval 11.2% to 19.5%).

## Strata (parent, folded, repo files)

| Stratum | Agents | Reads | Hit | Legitimate | Re-derivation |
|---|---|---|---|---|---|
| Workflow agents | 823 | 22,271 | 49.1% | 9.6% | 39.5% |
| Agent-tool agents | 175 | 6,091 | 51.0% | 27.4% | 23.7% |
| Agents that edited at least one file | 629 | 24,266 | 49.2% | 15.6% | 33.6% |
| Read-only agents (legit is 0 by construction) | 369 | 4,096 | 51.1% | 0.0% | 51.1% |
| Nested agents (depth 2 or 3) | 45 | 356 | 0.0% | 0.0% | 0.0% |

Nested agents show no hits against their own parent's before-spawn reads (356 reads), so the ancestor chain adds only 0.8 points.

## Top re-read files (literal row: parent history unbounded, re-derivation events)

| File | Re-derivation reads | Agents | Tokens |
|---|---|---|---|
| ADR.md | 565 | 257 | 515,096 |
| Taskfile.yml | 502 | 241 | 215,416 |
| CHANGELOG.md | 371 | 182 | 318,095 |
| docs/design/observatory-console.md | 316 | 149 | 276,594 |
| docs/usage.md | 273 | 119 | 132,355 |
| cmd/fathom/servehttp.go | 264 | 120 | 214,511 |
| docs/vision/35-observatory-console.md | 247 | 137 | 143,193 |
| docs/intent/invariants.md | 225 | 159 | 241,693 |
| go.mod | 216 | 163 | 69,668 |
| docs/plans/observatory-console.md | 200 | 138 | 170,622 |
| docs/mcp-tools.md | 180 | 91 | 131,136 |
| docs/design/refactor-conservation.md | 175 | 109 | 208,851 |
| internal/store/store.go | 148 | 66 | 63,454 |
| pkg/client/client.go | 146 | 77 | 88,032 |
| docs/intent/scope.md | 138 | 74 | 97,115 |
| internal/store/sqlite.go | 129 | 67 | 56,560 |
| lefthook.yml | 124 | 88 | 63,112 |
| .fathom/observations.md | 121 | 67 | 122,509 |
| docs/plans/refactor-conservation.md | 106 | 75 | 104,525 |
| docs/app-kit.md | 105 | 48 | 67,881 |

Largest by tokens: ADR.md (515,096), CHANGELOG.md (318,095), docs/design/observatory-console.md (276,594), docs/intent/invariants.md (241,693), Taskfile.yml (215,416), cmd/fathom/servehttp.go (214,511).

Top files re-read against the fleet view (parent or any earlier agent): ADR.md (585 reads, 267 agents), Taskfile.yml (508 reads, 244 agents), CHANGELOG.md (373 reads, 183 agents), docs/design/observatory-console.md (316 reads, 149 agents), docs/usage.md (291 reads, 128 agents), cmd/fathom/servehttp.go (265 reads, 121 agents), docs/vision/35-observatory-console.md (247 reads, 137 agents), docs/intent/invariants.md (226 reads, 160 agents), go.mod (216 reads, 163 agents), docs/mcp-tools.md (212 reads, 96 agents).

Top since-last-compaction re-reads (the context-live reading): ADR.md (489 reads), Taskfile.yml (415 reads), CHANGELOG.md (255 reads), cmd/fathom/servehttp.go (232 reads), docs/intent/invariants.md (174 reads), docs/design/refactor-conservation.md (168 reads), go.mod (146 reads), docs/intent/scope.md (118 reads), docs/design/observatory-console.md (112 reads), docs/plans/refactor-conservation.md (103 reads).

## Limits

- One parent session: every sub-agent in the window descends from a single long-running orchestrator session, whose per-wave clone and workflow-script pattern is not what an ordinary session does. Nothing here estimates a population of parents.
- "Before spawning" is taken literally. The parent read set at spawn is a median of 621 repo files gathered over up to 27 days; a map built by the planner for the task would carry a far smaller set. The recency rows are the sensitivity, not a replacement for the row.
- The denominator excludes 25,848 read-type Bash commands with no resolvable file operand (pipes, directories, globs, variables) and 13 unparseable Bash calls out of 62,294. Directory-wide greps and finds are discovery work a map could replace but the row does not count; reads done through python or other interpreters, and reads whose content came via git diff or log, are not seen. Parent knowledge is therefore undercounted and edits by interpreters are missed, which biases re-derivation in opposite directions.
- Followed-by-an-edit is judged only from the sub-agent's own later edits of the same repo-relative file. A read that informed an edit of a different file counts as re-derivation.
- Token volumes use tool_result size only, not the cost of the model re-reading carried context.
- RC-28 SubagentStart pointer injection was live over this period per the design doc (584 events in 4 days), so these shares are the residual after it; that was not separately verified here.

## What this means for the plan

- Literal row: not killed (36.1% vs 15%). Under this reading handoff maps stay alive, gated by XM-M2 (the A/B with its own 25% token bar).
- The addressable pool for a parent-to-child handoff is small: reads of files the parent touched in the last hour are 2.8% of reads (0.74M tokens over 14 days). The pool a persisted or shared map could reach is much larger: files the parent touched more than an hour before spawning add 33.4 points and sibling re-reads reach 69.9% of reads.
- If the aim is fewer tokens per session, the measured waste is in fleet onboarding reads of the same hot files across sibling agents (top files above), so a shared brief, a preloaded hot-file digest, or live maps that sub-agents also append to (XM-14, F50) looks like a better first target than the parent-only handoff of XM-1 to XM-8. This reading is as strong as its single parent session and should be repeated once another orchestrator session has spawned enough agents.
