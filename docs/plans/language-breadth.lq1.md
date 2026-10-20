# Language breadth, wave lq1: language-queue tooling

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/language-breadth.lq1.md` proves it. Rows here are the
queue-tooling rows wave lq1 files (`docs/plans/language-breadth.md` is edited only for a worker's own rows,
AP-14). Defect: `indexbench lang-next` printed `eligible` for scss, graphql, go and kotlin though each has a reading
and a deferred `-rest` row (O1394).

## Queue tooling

- [ ] `indexbench lang-next` separates startable from waiting: an item prints `eligible` only when its needs are met and no deferred `-rest` row holds it (no reading, `level_met` none, or a partial level with no `-rest` row, which prints `eligible (at L2 of L4)`); an item a deferred `-rest` row holds prints, with `-all` only, `waits: <rest id> (at <level_met> of <target>): <trigger>` {id: LB-F581, symbol: langNextLines, package: cmd/indexbench, reachable_from: TestLangNextSeparatesStartableFromWaiting}
- [ ] One loader reads `language-breadth.md` and its sorted `language-breadth.<wave>.md` siblings (the one sibling rule, shared with the console) into a single plan whose rows keep their source file; lang-next, the board and the queue test use it, a queue `depends_on` and a row's `needs` may name `language-breadth.<wave>.md#<id>`, and `LB-Q-graphql` names `language-breadth.lang5.md#LB-F580` again {id: LB-F582, symbol: LoadBreadthPlan, package: internal/langbench, reachable_from: TestQueueReadsSiblingPlans, needs: "LB-F581"}
