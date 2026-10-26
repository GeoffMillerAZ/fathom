# Contribution inventory and disposition

Historical preservation state. Subsequent cleanup and recovery artifacts are
recorded in [the follow-up receipt](../../kit-interoperability/2026-10-21/cleanup.md).

Date: 2026-10-21. Pulled origin/main first: 6187f8a -> 5d4c989, three commits.
Initial dirty Fathom files were stashed under
`integration-2026-10-21-preserve-fathom-wiring`, then restored before integration.
No worktree, stash, branch or user edit was deleted.

| Source | Disposition and evidence |
|---|---|
| origin/codex/catalog-kit-2026-10-20 / GitHub draft PR #11, 89fbe33 | Integrated as the catalog workflow starting point; strengthened by the current specification and tests. |
| origin/agent-tokenomics, 8af2939 | Ancestor of main, already landed. |
| docs/intent-excellence-assessment, fac9fe7 | September content landed 2026-09-25 and was hardened; decision 0016 records the subsequent audit. |
| operational-reuse, a72939b; evidence-producer, 2639212 | Landed through the September integration under different commits, including d7e71e3 and f7e2673. |
| worktree-evidence-reuse, 3b5b3c8 | Four dirty paths: two equal main, two retain weaker historical reuse behavior. Main 605730a preserves historical regrade/reprice. Preserved, not ported. |
| worktree-gw-cli-wire, f8ec57f | Dirty generated README/catalog variants only; CLI work landed through 0ca8823 and later fixes. Preserved. |
| worktree-gw-sourced-scenario, 5256950 | Dirty generated README/catalog variants only; sourced workflow landed through aa7bf96. Preserved. |
| worktree-gw-workflow-cli, 1b29451 | Eleven dirty sourced-prose paths: seven byte-identical to main, four older/weaker validation. Main includes strict Unicode hardening. Preserved. |
| Pre-existing current Fathom wiring edits | Retained and reconciled through the installed generator, receipt 6a9d12c5dfbe; hooks, instruction routing and skills integrated. Machine ownership/receipt state stays ignored. |
| Doctor-trust stash, base f4f0076 | Doctor deduplication already landed as 4f6b0be; remaining kit-new changes are formatting only. Preserved. |
| Temporary prose/HITL overlays stash | Superseded by aa7bf96 and 1c2c7f9; preserved. |
| Untracked workflow-CLI draft stash | Superseded by 0ca8823; preserved. |
| GitHub PR inventory | All eleven PRs checked. PR #11 is the catalog source above; the other open count is zero. Closed unmerged #3/#4/#5 were integrated under different commits; decision 0016 records check-catalog, capped-confidence and structural-refusal dispositions. Independent inspection found no missing useful delta. All remaining PRs are merged. |
| Bounded reflog/unreachable-object audit | 84 unreachable commits: stash snapshots and earlier drafts. No further useful delta identified. This is a bounded audit, not proof about every historical object. |

Branch comparisons used main history, semantic changes and decision 0016, not only
patch IDs: squash integration and later hardening give old patches distinct IDs.
The four additional worktree locations remain available in `git worktree list`.
The read-only audit and final code review ran independently of implementation.

Prepublication re-fetch and PR inventory retained the same contribution heads:
main 5d4c989, catalog 89fbe33 and already-landed agent-tokenomics 8af2939.
All four preserved worktree heads remain unchanged: evidence-reuse has 4 dirty
records, gw-cli-wire 2, gw-sourced-scenario 2 and gw-workflow-cli 11. All four
stashes remain present. Original
consumer-a/consumer-b/consumer-c retain 29/30/141 dirty records at
their recorded revisions. Fathom is the intentional exception: its necessary
upstream repairs were separately validated, pushed and deployed at 07b249ca5d78.
