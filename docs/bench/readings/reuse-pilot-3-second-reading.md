## Second reading: reuse-pilot-3 (change tasks, full vs control)

### Caveats first

1. **Read the ledger's booleans, not the trees on disk.** Each (task, arm) pair ran both trials in one scratch worktree, and the two tasks that share a base commit (RE-SB-cli__cli-10043 and its -K1 variant) share that worktree too: six source trees served eight cells. The per-trial cell records were written inline and in sequence (10043 cells 00:39 to 00:50, K1 cells 01:27 to 01:29; 10048's trial-1 cells precede the on-disk oracle outputs by four minutes), so the PASS/FAIL booleans are per-trial and stand. But agent.diff and the oracle output hold only the last trial, the source tree left behind for 10043 is K1's, and trial 1 of any cell cannot be re-derived from disk.
2. **One trial is contaminated.** The subject is a full clone and the merged fix for RE-SB-cli__cli-10124 (commit 2306623ca, PR #10124) is reachable from the base commit. The control planner in trial 2 ran `git log --all`, found it, and wrote it into the plan. The trial failed anyway (it followed the issue's stale error text), so the count does not move, but that trial is void as evidence and the leak is latent for every task in this corpus.
3. **The oracle scores production code only.** Before running tests it resets every file the gold test patch touches and reapplies the canonical patch, so an agent that never edits the downstream test files still passes. The full arm's two passes on 10043 rest on diffs touching 4 of the 8 gold files; the control arm's touch all 8.
4. **The K1 cell cannot be won by either arm as built.** One worker, a two-row plan, and a worker told to leave the second row for workers who do not exist. All four trials failed the same way. The corpus registered "no effect" here and that is what it got.
5. **Two secondary readings are not arm-comparable.** The false_done and drift detectors skip `_test.go` files and match names exactly while methods are indexed as `Type.method`, so any plan row anchored to a `Test*` symbol or a bare method name is refuted no matter what the agent did. Only the full arm writes such rows, so its refutation counts are inflated (10043 full t2: claimed 5, refuted 5). And each full-arm diff carries 14 to 16 fathom wiring and runtime files (CLAUDE.md, .fathom/*.cue, receipts.jsonl, workspace/logs, retro JSON) that control diffs do not, so payload_bytes is not like-for-like.
6. **The evidence lives in a session scratchpad**, not under the repository's workspace/. The repository's cell store holds the pilot-2 records under the same cell keys; resolve a key there and you silently read the wrong run.

### What this pilot measured

Four change-shaped tasks derived from cli/cli pull requests, planner plus workers, sonnet, two trials per arm. The wired arm had the fathom MCP server connected in every session and the hook stack firing (PostToolUse 18, 6, 20 and 15 across the four tasks) and called fathom tools 29 times (9, 3, 9, 8). The control arm had no MCP server and no hooks. Registered criterion M5-1 asks the wired arm to match control on successes and to spend no more than 1.0x the tokens per success.

### Did fathom help on these change tasks

No. Successes are full 2/8, control 2/8: both arms passed both trials of RE-SB-cli__cli-10043 and failed everything else. In all four passing trials the gold sites were located by Bash or Grep; fathom is the recorded gold source in 1 of 16 trials, a 10048 failure. Trial by trial, the 29 tool calls decorated in four trials, helped without changing the result in one, and in the two 10048 trials anchored the plan to the wrong functions (NewCmdView/NewCmdEdit where the tests call viewRun/editRun). verify_plan produced correct fail verdicts twice and workers checked the box past them; two workers satisfied it by writing `skip_test: true` into the row. The full arm's own prediction ("overlap low, workers reach first edit sooner") is refuted by its own readings: tokens to first edit were higher for full on every task (2.74x, 2.05x, 1.26x, 2.38x control) and overlap was higher on two of four.

### The cost

Full 14,496,262 tokens against control 8,465,827 (1.712x); $7.17 against $4.93 (1.455x). Per task: 10043 5,152,934 vs 2,099,669 (2.454x for the same 2/2); K1 1,345,691 vs 971,794; 10048 3,493,797 vs 2,305,148; 10124 4,503,840 vs 3,089,216. M5-1 fails on the tokens-per-success bar (1.712x against 1.0x) although the success bar is met. Dropping the null K1 cell does not rescue it: 13,150,571 vs 7,494,033 is 1.755x.

### What would have to be true for the wired arm to win here

- The plan's anchors have to be the gold functions. Both 10048 plans anchored to the Cobra constructors; find_symbol was called and did not move them.
- Plan rows have to fit the worker budget, or the harness has to reassign. An eight-row per-symbol plan with three workers leaves five rows unclaimed by construction; on 10124 the two rows that wire the command were never assigned in either trial. Control's coarse checklist let one worker sweep five files in one loop.
- A verify_plan fail has to stop the box from being checked, and `skip_test` has to be granted by the verifier, not written by the worker.
- The tools have to find something grep did not. In every passing trial grep had the sites first and the tool calls confirmed them.
- The K1 cell has to be dropped or given a harness that can finish a two-row plan.
- Tokens per success have to fall below control's, which on 10043 means closing a 2.45x gap, not a margin.

### Standings after reading

Registered oracle: full 2/8, control 2/8, unchanged; M5-1 FAIL stands. Conservative adjustments: 10124 control t2 void (leak, FAIL either way; control 2/7 if excluded); K1 null by design for both arms. Under a mergeable-patch reading instead of the registered oracle, full would be 0/8 and control 2/8. Nothing in the four reader reports shows a fathom tool changing an outcome on these change tasks.