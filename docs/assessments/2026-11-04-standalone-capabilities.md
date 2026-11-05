# Which of fathom's capabilities could stand alone, and how the repository should be organized

Measured 2026-11-04 at `c26ced479` from `go list` over `./internal/... ./cmd/... ./pkg/...` (157 packages, 473,461 non-test lines).

## Verdict

Only the language library should become public now, because only it has a named consumer. Several others could stand alone, and each gets a stated trigger. The repository needs no mass
reorganization. Its one real structural problem is `cmd/fathom`, which is its own program and not part of this change.

## How to organize the repository

| Option | Verdict | Why |
|---|---|---|
| Move `internal/extractor` under a new `lang/` root | No | 144 plan rows anchor to `internal/extractor` and 155 to `internal/fusion`, and language lanes work there. Moving breaks proofs and every open lane, and buys only tidiness. |
| Group the 146 `internal/` packages into families | No | Cosmetic, with the same cost. |
| Add a public door and enforce its closure | Yes | The boundary is enforced by a test rather than implied by a directory. |
| Pull dispatch out of `internal/fusion` | Yes | `fusion` is 32,036 lines mixing ingest, resolution and routing, and routing is the piece others need. |
| Split `cmd/fathom` into packages | Later | See below. |

### `cmd/fathom`

143,462 of 473,461 lines (30%) sit in one `package main`, across 1,184 files, importing 102 internal packages. Its tests are the slowest in the suite, and anything in it can reach anything else in
it. Verbs could move to `internal/cli/<verb>` with `main` as a thin table.

It is worth doing, but not now: it touches files every lane edits, and it has no user-visible result. F98 records it with its trigger.

## Candidates

Closure counts the fathom packages a package reaches. Fan-in counts the fathom packages that import it.

| Capability | Packages | Lines | Closure | Why it could be valuable | Verdict and trigger |
|---|---|---|---|---|---|
| Language understanding | `extractor`, `tsengine`, `promql` | 24,589 for `extractor` | 12 today, 4 after the cuts | Facts from 100+ languages and formats, graded against real oracles | **Build now** (ADR-092) |
| Route inventory | `analysis/routes` | 10,914 | 2 | API inventory and attack surface from source across frameworks | Next door, under `pkg/lang`, when a consumer asks |
| Incident to code | `logtemplate`, `stackframe` | 2,776 | 1 and 0 | Map a log line or a stack trace to a source site, which observability tools lack | A named observability consumer |
| Agent command guards | `shellcmd`, `hookguard` | 2,937 | 0 and 2 | Understand shell commands for agent safety hooks | Better as a kit piece, if a second agent tool wants it |
| Test selection by observed reads | `analysis/testinputs` | 446 | 0 | Pick Go tests by the files they read. TQ-M8 read 4 of 6 against a bar of 5, so it is not proven yet. | The CI kit, once TQ-M7 reads (ADR-063 O1) |
| Multi-harness setup | `harness` | 3,799 | 0 | Hook and instruction setup across Claude Code, Codex and Gemini | A second agent tool needs it |
| Budgets and shedding | `tenancy` | 5,033 | 0 | Memory accounting and shed levels for long-lived Go daemons | A second daemon needs it |
| Read-only git | `gitro` | 706 | 0 | A small and safe git reader | Too small to matter alone, and fine to copy |
| Parser grading harness | `langbench`, `calibration/` | 10,483 and more | 42 | Grade a parser against an oracle by tier. The grading method is what sets fathom apart. | Pack authors ask for more than `langtest` gives |
| SCIP import and export | `scip`, `analysis/scipexport` | 1,859 | 1 and 2 | Bridge to other code-intelligence tools | A named consumer |

### Why not more of them now

P20 says to build for a named consumer and to generalize from two. A public package is a promise: it joins the compatibility surface and slows every later change to it. Most of these are one
door away, because their closures are already small, so waiting costs almost nothing. That is the point of measuring closure.

The repository-strategy test also applies (`docs/handoff/repo-strategy.md`): could this ship on a different day than fathom, and would anyone notice? For all of these the answer is not yet.

## What it drove

- ADR-092, G50 and `docs/plans/lang-library.md`.
- F98 (`cmd/fathom` split) and F99 (further doors) in `docs/intent/futures/f98-f99.md`.
