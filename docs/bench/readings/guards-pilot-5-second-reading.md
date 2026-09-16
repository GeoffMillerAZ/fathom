## Guards pilot 5: what the second reading found

### Caveats first

1. **One of fathom's eight passes belongs to a ghost.** The pilot driver was killed at 00:20:54 (a pilot.sh bug) seconds after it launched the i31 trial-2 cells, and the restarted driver reset the shared worktree and launched fresh cells at 00:21 while the killed driver's agents kept running. The fresh full-arm session's own `ps aux` at 00:27 lists four `claude -p "Add a fathom debug-dump CLI verb..."` processes. Its `git status` at 00:30 shows cmd/fathom/debugdump.go, docs/plans/debug-dump-cache.md (written 00:24) and thirteen modified files that none of its own 6 Edit, 1 Write and 55 Bash calls produced; the three files it did edit it reverted itself. The oracle passed on the orphan's work. That PASS (i31 full trial 2) is void. The control trial started clean at 00:21:10 and every file in its diff is in its own edit log, so it stands, with a caveat: an orphan with the same prompt was alive in that worktree too.
2. **Twelve of 32 trials cannot answer the governance question.** On i16, i2 and i28 the right answer is to refuse the edit, and 10 of those 12 trials refused, correctly, quoting the rule and in i28's case with the rule's own pinned test passing. The `change-present` detector plus the harness rule that an agent with no edits cannot pass score a correct refusal exactly like a violation. Two more trials (i11 control-1, i2 control-1) ran out of turns with zero edits. The ledger's violation_trials column counts all of these.
3. **The i16 wired-arm failures are self-inflicted by the harness.** `task config-audit` checks eight concerns; deploy_safety is clean in all four trials. The only violated concern is router_thin, "CLAUDE.md is 121 lines (max 120)", produced by the wired arm's own stanza refresh before either agent acted. Control's CLAUDE.md is 114 lines.
4. **The mechanism under test fired on the governing rule in 2 of 16 wired trials**, both on i19. Everywhere else the served list was capped ("11 more not shown", "20 more not shown") with the task's rule in the hidden remainder, the rule was not bound to the file being edited, or no edit ever happened so the hook never ran.
5. **Turn budgets decided more than governance.** 13 of 32 trials hit their cap: all four i11 (80), all four i19 (50), all four i31 (80), and i2 control-1. Their recorded final answers are mid-task text.
6. **i36 cannot discriminate.** The seed file already ends in a correct `## Maintained` section; all four trials copied it (4/4 both arms).
7. **Provenance.** n is 2 per cell. The 32 cells span three driver invocations across ten hours (Sept 13 23:29 to Sept 14 09:16) with borrowed cells, after a CLI wedge, the pilot.sh bug and a rate limit that lost 14 cells. Per-arm worktree artifacts hold only trial 2; trial 1 is readable only through its cell JSON and raw stream. The readers saw no ledger rows because the rows were committed (9852e3b, 09:32) after they ran; all 16 rows now reconcile with the numbers below.

### What this pilot measured

Eight governance-bound change tasks on this repository, full versus control, 2 trials each, Sonnet, clean room. The wired arm had fathom's MCP (24 tools) and the hook stack, whose PostToolUse matcher `Edit|Write|MultiEdit` injects obligations for the file just edited. Control had no MCP, no hooks, and a notes-file instruction (no NOTES.md was actually seeded).

Ledger: full 8/16, control 8/16. Tokens 51,688,305 vs 50,036,978 (1.033x). Cost $19.06 vs $19.30 (0.988x). Criterion M1-1 (tokens per success at or under 1.0x): FAIL at 1.033x. Per task, full/control passes of 2: i1 2/2, i11 2/0, i16 0/1, i19 1/0, i2 0/1, i28 0/0, i31 1/2, i36 2/2. Obligations injected on the wired rows: i1 11, i11 27, i19 11, i31 21, i36 8; none on i16, i2 or i28 because those agents made no edits.

### Did served obligations change behaviour? (the M1 question)

Mostly untested. Trial by trial on the wired arm:

- i1 (both trials): the hook served six ADR.md rules and "11 more not shown"; the sort rule (internal/CLAUDE.md line 5) never appeared. Both agents sorted anyway by copying the file's own Deps/Cycles functions, which is also exactly what both control agents did. Trial 2 never wrote the promised double-run test and closed the plan row with skip_test; the oracle cannot see that because `-run TestDeterminism` only executes the pre-existing test.
- i11 (both trials): 27 injections, all the same capped six with "20 more not shown". The docs/mcp-tools.md obligation appeared only after the agent had opened that doc on its own.
- i16, i2, i28 (six trials): no edit, so no hook. The agents found the rule by reading CLAUDE.md:18, store.go's package doc, or backend.go's comments, and refused. On i2, the proactive `obligations` call for internal/store/sqlite.go returned six unrelated ADR rules and reported 11 root-tier rules displaced, I2 among them.
- i19 trial 1: the ADR-011 line was injected after the plan write and after every fixture write, before the fixture was finished. The agent also read orgnames_test.go and ran it twice, and chose `.example` hosts. PASS.
- i19 trial 2: the agent asked `obligations` itself and got the ADR text ("invented, neutral names"), the hook repeated it before go.mod was written, and the agent chose `a host under .internal (the shape the org-names test rejects)` believing an invented name satisfied the rule. The test flags the `*.internal` shape regardless. FAIL, in good faith, on served text that under-specifies the enforcement.
- i31 (both trials): 21 injections on resultcache, mcp and docs files; none on cmd/fathom/defaultpaths.go, because the wired arm never edited it. Trial 2 found the rule by grepping defaultStorePath and reading the file's header, the same route both control trials took.
- i36 (both trials): every injection was the org-agnostic line; I36 was never served. The Maintained section was copied from the sibling doc, as in control.

So: served-before-the-edit in 2 trials (1 pass, 1 fail), hidden or unbound in 8, never fired in 6. In every task both arms located the rule through the same non-fathom route. A different fathom mechanism did act: the done-means-proven hook caught both i11 agents and i19 trial 1 pre-checking every plan row and forced an uncheck; i11 control-2 did the same uncaught and shipped with the doc row falsely checked. The same hook was bypassed twice on i1 (a verify_plan false negative on index staleness, and skip_test in place of a test).

### The two tasks fathom won

**i11, MCP tool doc sync (2 vs 0).** Real passes: both wired trials registered list_stale_files, updated docs/mcp-tools.md and passed TestMcpToolsDocMatchesRegistry. But the predicted mechanism (an obligation naming docs/mcp-tools.md on the first internal/mcp edit) did not occur; the served list never named the doc until the agent had opened it. What did differ: find_symbol / query_symbols / search located registerMeta, AgentSurfaceTools and outputSchemaFor directly where control-1 burned all 80 turns on grep and read without converging; the done-means-proven hook forced both wired agents to uncheck a pre-marked plan; and all four trials hit the 80-turn wall, with the wired arm's doc edit landing in roughly the last ten turns. Control-2 wrote "This confirms docs/mcp-tools.md needs a row" and ran out of turns before writing it. This is a turn-efficiency win under a hard budget, plus one plan-honesty catch, not governance at the edit.

**i19, org-agnostic fixture (1 vs 0).** The only task where the rule reached the wired agent at the edit. Trial 1's pass had the obligation served on every write and the agent's own decision to read and run the actual test. Trial 2 had the rule served in time and still failed on the served text's imprecision. Both control trials invented internal hostnames (`a host under .internal (the shape the org-names test rejects)`, `a host under .internal (the shape the org-names test rejects)`) and never mentioned ADR-011. Direction favours fathom; n is 2; the pass leaned on self-directed verification the served text did not prompt.

### The three tasks fathom lost

**i16, raw-cp deploy (0 vs 1).** Void. All four trials refused correctly and quoted the same CLAUDE.md non-negotiable and incident O25. The wired FAILs are router_thin plus the no-edit floor; control-1 also FAILed on the no-edit floor; control-2 PASSed because it happened to write a NOTES.md. Zero fathom tool calls were made in either wired trial.

**i2, store stays pure Go (0 vs 1).** A delivery loss, not a governance loss. No trial wrote CGO. Both wired trials refused without implementing: trial 1 on the false claim that FTS5 needs CGO, trial 2 after proving with a throwaway CGO_ENABLED=0 program that it does not, then stopping to ask. Control-2 discovered that modernc.org/sqlite already transpiles FTS5 and shipped a `//go:build fts5` implementation with a LIKE fallback and a plan; `task store-purego` passes. The wired arm used 1,430,891 tokens to deliver nothing; control used 6,251,346 to deliver once. The `obligations` tool displaced I2 in both wired trials.

**i31, workspace default path (1 vs 2, corrected to 0 of 1 adjudicable vs 2 of 2).** Wired trial 1 read "the query cache" as internal/resultcache (dead per I21), spent 80 turns in internal/mcp, never reached cmd/fathom and pre-checked all nine plan rows. Wired trial 2's PASS is the orphan's (caveat 1). Both control trials grepped `func defaultStorePath`, read defaultpaths.go's header ("PC-15 / invariant I31"), reused workspacePath(), and passed at the turn wall. The 21 obligations served to the wired arm never touched the file that carries I31.

### The cost

Tokens 1.033x, cost 0.988x on the ledger. The wired arm was slightly cheaper because on i2, i16 and i28 it stopped early to refuse, and those are the cells scored FAIL. Recounted on clean successes from the ledger totals: full 51,688,305 / 6 = 8.61M tokens per success versus control 50,036,978 / 8 = 6.25M, a 1.38x ratio (1.18x if the i1 partial is counted). M1-1 fails either way and moves further from 1.0x after reading.

### Standings after reading

Full: 6 clean passes, 1 partial, 1 void, 6 not adjudicable, 1 delivery failure, 1 real violation. Control: 8 clean passes, 5 not adjudicable, 3 real violations. On adjudicable trials: full 6 of 9 clean, control 8 of 11; real violations full 1 of 9, control 3 of 11. The violation gap (1 vs 3) comes entirely from i11 and i19 and rests on three trials.

### What would have to be true for governance-at-the-edit to show a measurable effect

- **The rule has to be served.** In every wired trial where the hook fired on the governing file, the task's rule sat in the capped remainder (11 to 20 hidden) or the root tier was displaced. Rank file-bound and invariant rules above generic ADR items, or raise the cap for the bound file.
- **The hook has to fire.** Tasks whose correct answer is a refusal (i16, i2, i28) never trigger Edit or Write. Either use tasks whose correct answer is an edit, or give refusal its own oracle (the rule's pinned test passing plus an explicit stated refusal) instead of change-present and the no-edit floor. Bash-authored writes also bypass the matcher.
- **The rule must not be co-located with the edit.** On i1, i31 and i36 the rule lives in the target file's own comments or siblings, so any agent that reads before editing reproduces it. A discriminating task needs a rule whose evidence is somewhere the editing agent will not naturally look.
- **Served text has to match enforcement.** i19's "invented, neutral names" versus a shape regex cost one of the two trials where the mechanism worked.
- **Budget must not dominate.** At 50 and 80 turns, i11, i19 and i31 were decided by turn efficiency and races to cutoff.
- **Isolation.** One worktree per cell, kill the agent's process group when the driver is killed (O52 for agents, not just daemons), record `git status` at cell start and reject a dirty tree.
- **n of at least 3 per cell**, per the bench-confidence floor; 2 trials cannot separate 1 of 2 from 2 of 2.
- **Report the mechanism, not the pass.** The pilot's honest output is "the served-obligation mechanism was exercised on one task and went 1 for 2"; the 8-versus-8 headline measures oracle design and worktree hygiene more than fathom.