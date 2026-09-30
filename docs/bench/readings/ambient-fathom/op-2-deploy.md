# OP-2: the deployed daemon serves main's HEAD after a deliberate deploy

Row states and readings only. The row's lexical oracle matches the one line that begins with the row id
(`docs/plans/ambient-fathom.md`, OP-2); the sections under it say what was read.

OP-2 reading: the managed daemon serves main's HEAD after a deliberate deploy, read twice on 2026-09-29: at stop 1 of the efficiency wave `task deploy` exited 0 and its assert read the served binary as 087fb5ab, equal to HEAD after push 81, and at 22:27Z, read-only, the lifecycle ledger's last `deploy` row (by `task deploy`, 21:27:30Z) is followed by the start of pid 57698 on commit 9ea6da5f, which is `origin/main`, and `fathom doctor --service fathom` reads `binary_fresh` and `daemon_fresh` PASS at that commit; no daemon was deployed or restarted for this reading.

## What was read

- **Stop 1 (EW-S1, `docs/bench/readings/efficiency-wave/stops.md`).** Push 81 landed as 087fb5ab. `task deploy`
  from a clean clone exited 0 in 77 s; after 60 health polls its assert read the served binary as 087fb5abb033,
  equal to HEAD. `fathom doctor --service fathom` on the main checkout then read `binary_fresh` and
  `daemon_fresh` PASS, local and daemon binary both at 087fb5abb033.
- **Today, read-only (2026-09-29T22:27Z).** `git ls-remote origin refs/heads/main` names 9ea6da5f. The daemon
  ledger (`~/.fathom/daemon/<label>/lifecycle.jsonl`, the managed job's launchd label) ends with a `deploy`
  row written by `task deploy` at 21:27:30Z (binary hash e8e7c035671b, replacing pid 2642), that pid's `exit` row (reason
  `drain`, not counted by the breaker), and a `start` row at 21:28:31Z for pid 57698, managed, `binary_commit`
  9ea6da5fec78. `pgrep` shows pid 57698 is the running `serve-http --managed` on 127.0.0.1:7575.
- **Doctor at main's tip.** `fathom doctor --service fathom` from a detached checkout of 9ea6da5f, with the
  installed binary: `binary_fresh` PASS ("this binary @ 9ea6da5fec78 matches HEAD 9ea6da5fec78 of its own source
  tree"), `daemon_fresh` PASS ("daemon binary @ 9ea6da5fec78 matches this binary @ 9ea6da5fec78 (daemon built
  2026-09-29T20:47:39Z)"); 53 checks, 40 pass, 3 warn, 0 fail, 10 skipped.
- **The same doctor from a worker clone** two commits ahead of main reads `binary_fresh` FAIL ("2 commit(s)
  behind HEAD") and `daemon_fresh` PASS. That row compares the local binary to the checkout it runs in, so an
  unpushed branch reads behind by construction; it says nothing about the deployed daemon against main.

## What this reading does not say

- It reads one moment. The row's condition holds only until main moves: the next push is live in the daemon
  only after the next `task deploy` (its assert is the check), and `daemon_fresh` is the row to re-read.
- It does not judge any other row in the plan live; it says the binary those rows are judged against is main's.
