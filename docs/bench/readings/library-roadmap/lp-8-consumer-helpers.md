LP-8 state: three helpers are proven (span lookups, row export and schema, agent cookbook). The outline is delivered with its size bar missed for two dialects (Python and TypeScript). HLP-2 and LP-8 stay open.

# LP-8: consumer helpers, where the program stands

This is the program reading of LP-8. It carries no verdict line, because LP-8's done row waits for HLP-2 (ESCALATE-28 R7). Every measured number below is copied from the committed reading it cites, and none is new.

## The rows

| Row | What | State | Reading |
|---|---|---|---|
| HLP-1b | `fathom-lang at` reaches `Index.EnclosingRange` | proven by its plan row (symbol and package contract) | none, the contract is the proof |
| HLP-1 | Span lookups: `view.NewIndex`, `Enclosing`, `EnclosingRange`, `Touched`, `Slice`, the `at` verb | PASS | [hlp-1-span-lookups.md](hlp-1-span-lookups.md) |
| HLP-3b | `fathom-lang extract -rows` reaches `view.Rows` | proven by its plan row | none |
| HLP-3 | Row export and schema: `extract -rows`, `schema`, `view.Rows` | PASS | [hlp-3-row-export.md](hlp-3-row-export.md) |
| HLP-2b | `fathom-lang outline` reaches `view.Outline` | proven by its plan row | none |
| HLP-2 | File outline: `view.Outline`, the `outline` verb | FAIL (python, typescript), open | [hlp-2-file-outline.md](hlp-2-file-outline.md) |
| HLP-4 | Agent cookbook and tool definitions | PASS | [hlp-4-agent-cookbook.md](hlp-4-agent-cookbook.md) |

## The HLP-2 numbers, as published

The bar, fixed before the reading: median of outline bytes over file bytes at or under 20 percent for each of the four dialects. From [the HLP-2 reading](hlp-2-file-outline.md):

| Dialect | Files | Median | 90th percentile | Bar |
|---|---|---|---|---|
| go | 1386 | 18.50% | 36.77% | PASS |
| javascript | 1508 | 13.14% | 33.43% | PASS |
| python | 1314 | 58.31% | 123.96% | FAIL |
| typescript | 1224 | 24.17% | 75.19% | FAIL |

The bar did not move (R7). The reading measures size, not whether an agent does better with an outline. By file size the median ratio falls as files grow in every dialect (the "By file size" table of the same reading), and the cookbook's recipe 1 says to read a small file whole and to use the outline for a large one.

The other measured results: the HLP-1 reading reports 100.00% of symbols with a known span in all four dialects, and `Enclosing` `found` on 89.13% of Go lines and 100.00% of Python, JavaScript and TypeScript lines. The HLP-3 reading rebuilt the nested document from the rows for 5432 of 5432 measured files, with 0 lossy.

## Rulings

- ESCALATE-26 R1: the package budget is the door's own closure (`pkg/lang` at 80) and does not grow. `pkg/lang/view` is an opt-in package like `pkg/lang/cue`.
- ESCALATE-26 R2: `EnclosingRange(start, end)` is an additive function with `Enclosing(line)` equal to `EnclosingRange(line, line)`, and the unknown-span test uses START.
- ESCALATE-27 R3: `found` needs a complete read, a known span holding the whole range, and no unknown-span symbol with `Line` below 1 or at or after the candidate's `Line` and at or before START.
- ESCALATE-27 R4: the last tie-break is the largest `Key`, so the innermost symbol is last in the one shared order and HLP-2's "first as parent" stands.
- ESCALATE-28 R5: `Text` writes each whitespace run that holds a newline or carriage return as one space and drops it at either end of a string.
- ESCALATE-28 R6: the shared order is made total after `Key` by `Kind`, `Canonical`, `Name`, `Signature`, `Package`, `Tier`, with the innermost rule its exact reverse.
- ESCALATE-28 R7: the HLP-2 bar is a miss and stays one. No pooled bar, no outline without signatures and no new bar. The size-bucket table and the fixed header and footer bytes are description. HLP-4 needs HLP-1, HLP-2b and HLP-3. LP-8's done row stays open while HLP-2 is open.

## Register entries

- HLP-15, row export for dependency edges (`deps -rows`), rating 4 / 2 / 2: selected, scheduled after HLP-3 is proven, an open row outside this build ([library roadmap section 7](../../../design/library-roadmap.md)).
- HLP-16, an outline size bar by file size, rating 3 / 2 / 2: deferred. It would be preregistered and read on a corpus not used for HLP-2. Trigger: the bench measures agent benefit from outlines, or a consumer asks.

## What stays open

- HLP-2: its box stays unchecked. Its reading says FAIL for Python and TypeScript. R7 adopts no new bar, and closing it is not scheduled. HLP-16 is the deferred route.
- LP-8: its done row (`LP-8 done: HLP-1 through HLP-4 proven`) stays unchecked while HLP-2 is.
- HLP-15: open, after LP-8, outside this build.
- The helper surfaces (`pkg/lang/view`, `at`, `outline`, `extract -rows`, `schema`, the cookbook and `tools.json`) stay provisional until the LP-8 readings pass and one release has shipped.
