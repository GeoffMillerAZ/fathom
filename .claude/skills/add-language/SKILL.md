---
name: add-language
description: Use for any work on how deeply fathom understands a programming language or file format. That covers adding a dialect (tree-sitter tags.scm or a doc engine), pinning or re-pinning a grammar, building support for a format such as justfiles or systemd units, measuring or regrading a reading against an oracle (ctags, a SCIP indexer, the tool's own evaluation), explaining why a language sits below its target level (L1 to L4) or was refused (parse errors over the 5% line), choosing the next item from the language-breadth queue, and flipping an LB-Q plan row. Trigger whenever someone in the fathom repository asks to support, add, finish, fix, re-pin or measure a language or format, or asks why one is below target or refused, even if they never say "dialect" or "skill". Not for using fathom on another repository, fathom's MCP tools, or the daemon's memory.
---

# Adding or finishing a language

A level is proven, never typed: a committed reading under `docs/bench/readings/lang/<id>.json` measures fathom against an independent oracle, and the generated tracker in `docs/languages.md` follows from it. Your job is to move one queue item's reading to its target, or to record honestly why it cannot get there.

The full worker recipe is `docs/design/language-breadth.md` section 13. Read the step you are on, not the whole design. This skill tells you which tool to reach for at each step.

## 1. Pick the work

- `go run ./cmd/indexbench lang-next` names the next ready queue item (its `depends_on` rows are done).
- `go run ./cmd/indexbench lang-next --near-miss` lists refused or below-target items whose blocking metric sits just past its bar (for example a parse-error rate of 5.6% against the 5% line). Try these first: a newer grammar pin can flip them.
- `go run ./cmd/indexbench lang-gap -id <id>` explains a below-target item: the miss classes keeping it down, with counts, examples where the reading recorded locations, and the recipe step to apply. Pass `-fresh` to measure again instead of reading the committed file.

The queue is `docs/plans/language-breadth.queue.json`; each item's plan row is `LB-Q-<id>` in `docs/plans/language-breadth.md`.

## 2. The levels and what proves them

| Level | Means | Typical proof |
|---|---|---|
| L1 | every definition, right name and range | ctags (tier B) or the language's indexer |
| L2 | call edges by syntax | a tier A or C oracle |
| L3 | imports and file-resolved edges | the language's own resolution or an import dump |
| L4 | typed edges | a SCIP index from the maintained indexer (Go and CUE are native) |

Oracle tiers: A is the language's own tools (SCIP indexer, type checker, the tool's own evaluation); B is universal-ctags or a compiler syntax dump; C is an independent parser or a grammar's tag queries; D is a grep proxy and proves nothing beyond "tolerated". Every level also needs at most 0.1% faulting files, at least 1 MB/s per core and no file over 2 s. A field the oracle cannot report caps the level at good.

## 3. The loop

1. Grammar (tags-engine items): `go run ./cmd/indexbench lang-pin -id <id>` drafts the lock row; check the licence at the pinned commit; build with `internal/extractor/tsengine/grammars/build.sh`.
2. Dialect: manifest, 3-6 fixtures under 200 lines each, `tags.scm` (and `imports.scm` for L3). Iterate with `go run ./cmd/indexbench lang-probe -dialect <id> <fixture>` until every required kind is captured. Write the failing test first.
3. Measure: `task lang:loop -- <id>` runs probe, oracle, regrade and tracker for the item and prints one bounded summary: level met with grade, the delta against the committed reading, and the top mismatch classes. It never writes the committed reading unless you pass `WRITE=1`, and it exits non-zero when any level regressed. It never installs or fetches an oracle: when one is missing it says which.
   If cloud throughput falls below 1 MB/CPU-s, run `indexbench lang-retime -reading <reading.json> -subject <name>=<dir> [-subject ...] [-runs 3] [-write]` on Apple silicon before citing a throughput refusal (design §1.9 T2).
4. Iterate on the top mismatch class, re-running the loop, until the reading meets the target or a written ceiling explains why it cannot.
5. Commit the reading with `WRITE=1 task lang:loop -- <id>`, then regenerate the tracker: `go run ./cmd/indexbench lang-tracker > docs/languages.md`. Never hand-edit the generated sections of `docs/languages.md`; `TestLanguagesTrackerIsGenerated` fails when you do.

## 4. Refusing honestly

When no grammar parses real code with under 5% of files in error, or no oracle can measure the target, the item is refused, not forced: its `LB-Q-<id>` row takes a `[-]` disposition with the measurement and a trigger to retry (for example a newer grammar release). A refusal claims no level and stays disclosed. Do not lower a bar, trim a calibration corpus or hand-label results to pass.

## 5. Proving the row

Only `fathom verify --service fathom --plan docs/plans/language-breadth.md` flips the `LB-Q-<id>` box. Run the gate's language tests before pushing: `go test ./internal/extractor ./internal/connector ./internal/fusion -run '<id>'` plus the tracker test above.

## 6. Many items at once

Each queue item is independent, so breadth work parallelizes well: one helper lane per item, each in its own checkout, following the `fathom-offload` skill for checkouts, checksums and returning results. Keep heavy oracle runs to one per machine at a time, and keep review and the gate on the controller.
