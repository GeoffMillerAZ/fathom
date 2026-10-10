# fathom's CLI on app-kit cli-kit (K11): the plan

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/cli-kit-adoption.md` proves it. Decision: owner K11, 2026-10-09.

Sequence, in prose. S1 fixes help on every verb without cobra or pflag and with no new dependency: every verb's help
exits 0 without running the verb, `fathom --help` and `--version` work, and `fathom help <verb>` is supported. S2
moves dispatch onto the kit's cobra-based cli-kit; its kill criterion is that the hook p50 regresses by more than 5 ms,
in which case the hooks stay on the hand-rolled path. S3 to S6 follow the owner decision and are not planned here. Only S1
rows are minted. Prefix CL: `CK-` collides with `third_party/app-kit/docs/plans/cli-kit.md`.

## S1: help on every verb

- [x] `fathom --help`, `-h` and `-help` print the verb list on stdout and exit 0, and `fathom --version` and `-version` print what `fathom version` prints and exit 0; a bare `fathom` and an unknown verb keep exit 1 (K11 S1) {id: CL-1, symbol: runWithStdin, package: cmd/fathom, reachable_from: TestRootHelpAndVersionExitZero}
- [x] Every registered verb, hidden ones included, answers a sole `-h`, `--help`, `-help` or `help`, and `fathom help <verb>`, with its usage on stdout (name, summary, its subverbs or flags) and exit 0 without running: no stdin read and no file written; one table test over verbs() pins it (K11 S1) {id: CL-2, symbol: printVerbHelp, package: cmd/fathom, reachable_from: TestEveryVerbHelpExitsZeroWithoutRunning}
- [x] Only a sole help token is intercepted: single-dash long flags and help words in any other position dispatch to the verb as before, and verify's and doctor's 0/1/2 exits are unchanged (compatibility surface) {id: CL-3, symbol: isHelpToken, package: cmd/fathom, reachable_from: TestSingleDashLongFlagsStillDispatch}
- [x] docs/usage.md states that help and `--version` exit 0 beside the verb exit codes {id: CL-4, oracle: lexical, path: docs/usage.md, pattern: "print to stdout and exit 0 without running the verb; `verify` and `doctor` keep 0/1/2"}
