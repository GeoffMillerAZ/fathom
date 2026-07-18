# governed-docextract-hitl

**What it proves:** composing `kits/docextract` + `kits/agent-harness` +
`kits/hitl` into a governed document-analysis workflow is a matter of wiring
three independent kit APIs together. No single kit knows about the others; the
scenario is the only glue.

## The composition

```
CSV attachment (budget.csv)
       │
       ▼
kits/docextract.Extract(name, mediaType, data)
       │  returns prompt-ready text (dep-free, no subpackage needed for CSV)
       ▼
kits/agent-harness (MockClient, budget guard wired, injection screen on, journal on)
       │  the agent calls the propose_change tool
       ▼
kits/hitl Engine.Preview(snapshot, op)      ← dry-run (same Applier as Apply)
       │  returns Preview{Summary, Changes, OK}
       │
       ~  [scripted approver: a real system gates here on human action]
       │
       ▼
kits/hitl Engine.Apply(&live, op)           ← commits; caller owns lock+persist
       │
       ▼
Report (asserted in scenario_test.go)
```

## What each test assertion proves

| Test | Kit exercised | What it proves |
|------|--------------|----------------|
| `TestRun_DocextractProducedText` | `kits/docextract` | CSV bytes → prompt-ready text; "department" and "engineering" in output |
| `TestRun_HarnessJournalHasExpectedSteps` | `kits/agent-harness` | Harness ran, journalled `run-started`, `turn-completed`, `run-ended` |
| `TestRun_HarnessUsedTwoTurns` | `kits/agent-harness` | Tool-use turn + final-text turn = 2 model calls |
| `TestRun_BudgetWasCharged` | `kits/agent-harness` + `budgetguard` | Budget guard charged once per turn; `BudgetUsed == Turns` |
| `TestRun_HitlPreviewRan` | `kits/hitl` | `Engine.Preview` produced a non-empty Summary mentioning "status" |
| `TestRun_HitlApplyMutatedState` | `kits/hitl` | `Engine.Apply` set `status` to "approved"; untouched fields unchanged |
| `TestRun_HitlPreviewMatchesApply` | `kits/hitl` | Preview Summary references "approved" — same Applier drove both |
| `TestRun_RunReportNonEmpty` | `kits/agent-harness` | Harness `FinalText` / run-report is non-empty and mentions "run" |
| `TestRun_IsDeterministic` | all three kits | Identical runs produce identical Reports |
| `TestRun_DeterministicUnderRace` | all three kits | Race-detector clean under `t.Parallel()` |

## Why the same Applier for Preview and Apply matters

The hitl engine's load-bearing invariant is that **the human approves exactly
what commits**. Both `Preview` and `Apply` call the same `Applier` function:
`Preview` calls it on a clone (the live state is untouched); `Apply` calls it
on the live state. There is no serialisation/deserialisation step between the
preview and the commit, so there is no window where a mismatch could creep in.
`TestRun_HitlPreviewMatchesApply` asserts that the Preview Summary references
the same value ("approved") that `Apply` actually wrote.

## Why CSV (not xlsx)

`kits/docextract`'s core package handles CSV, TSV, plain text, JSON, and any
valid-UTF8 payload without external dependencies. xlsx support lives in the
`kits/docextract/xlsx` subpackage, which pulls in a third-party library. The
root module has **zero external dependencies** (`go.mod` has no `require`
block), so this scenario uses a CSV fixture. The seam is the point: replacing
`text/csv` with `xlsx.Handler` is one line.

## Governance defaults (all on by default)

- **Budget:** `budgetguard.Guard` is wired with generous per-minute / per-day
  limits. With a real model it would cap spend before any call exceeds the
  configured threshold.
- **Injection screen:** `InjectionAction` defaults to `InjectionBlock`. Every
  tool result is scanned before it re-enters the conversation; flagged content
  is replaced with an `is_error` block, never returned to the model.
- **Journal:** `investigationjournal.Journal` records `run-started`,
  `turn-completed` (one per model call), and `run-ended`. The test asserts all
  three are present.

## Running it

```sh
go test -race -count=1 ./scenarios/governed-docextract-hitl/
```

No API key required. The model side is fully scripted via `MockClient`.
