# `fathom certify suggest` live reading (TC-24, TC-M4's own baseline)

**Verdict (plain language).** It works, it is fast (well under Jev's own
sub-second design point even counting store I/O and two round trips), and
the five suggestions spot-checked by hand were all correctly classified.
The one finding worth carrying forward: on this sample, the strength
choice never left `asserts` — every single proceed-band suggestion picked
`asserts`, none of `smoke`/`golden`/`property`/`conformance`/
`mutation-killed`. That may be an accurate read of fathom's own test
suite (which genuinely has few goldens or property tests outside specific
packages) or it may mean the port's own instructions/criteria under-
distinguish the ladder — worth watching once TC-M4's twenty-suggestion
acceptance window is reviewed, not a blocker to shipping stage 3.

Measured 2026-09-23, tc-d built at commit `04d7c620` (`bin/fathom`),
against a scratch store ingesting this same tree (fresh `.fathom/`
scratch dir, own `[decide]` block: `provider = "typesafe"`, `endpoint =
"https://openrouter.ai/api"`, `model = "typesafe/jev-1.13"`, key via
`api_key_cmd`, never printed or copied here).

```
fathom ingest --store live.sqlite --service fathom --path <tc-d>
fathom certify suggest --service fathom --store live.sqlite --limit 60 --out live-suggestions.jsonl
```

## Numbers

| Metric | Value |
|---|---|
| Candidates considered (`--limit 60`) | 60 |
| Proceed-band suggestions written | 43 |
| Confirm-band ("unsure", not written) | 17 |
| Below both bands (silent) | 0 |
| Decide errors | 0 |
| Wall-clock, whole command (store open + 2 batched Decide calls + file write) | 1.29 s |
| Chunks (60 tests × 2 questions = 120 > the 64-question cap → 32/chunk) | 2 |
| Strength distribution across the 43 suggestions | `asserts`: 43, all others: 0 |
| Confidence range (43 suggestions) | 0.90 – 1.00 (all ≥ the 0.9 proceed floor, by construction) |
| `asserts_noul` range (43 suggestions) | 0.92 – 0.97 |

No errors, no timeouts, no chunk skipped for oversize state (every test's
capped-4 KiB source plus its two questions fit well inside the 96 KiB
budget).

## Spot-check (5 proceed-band suggestions, read by hand)

| Test | File:line | Suggested strength | Verdict |
|---|---|---|---|
| `TestCmdReport_AggregatesAndRenders` | `cmd/contextbenchscore/main_test.go:79` | asserts (0.96) | **Correct** — writes two arm-result fixtures, runs the report verb, asserts exit code and (further down) the rendered comparison content. |
| `TestHelp` | `cmd/fathom/app_test.go:38` | asserts (0.94) | **Correct** — asserts the help output contains `"ingest"`, a concrete expected substring, not a bare smoke/no-panic check. |
| `TestVersion` | `cmd/fathom/app_test.go:25` | asserts (0.90, the lowest of the five) | **Correct, and the low confidence is honest** — it only asserts stdout is non-empty, a weaker check than the other four; `asserts` is still the right ladder rung (it is a real, if thin, assertion), and 0.90 sitting right at the proceed floor reflects that thinness rather than a wrong call. |
| `TestIngestThenQuery` | `cmd/fathom/app_test.go:120` | asserts (0.99) | **Correct** — ingests a fixture then queries a known symbol, asserting the ingest produced output and (further down) that the query result names it. |
| `TestReadBeatLogTail_AbsentFile` | `cmd/fathom/beats_test.go:146` | asserts (0.94) | **Correct** — asserts `ok == false` for a missing file, one concrete expected outcome, single-purpose unit test. |

5 of 5 correct. None of the five was actually a `smoke` (no real
assertion) or a `golden`/`property`/`conformance` test, so this sample
does not by itself explain why the ladder never diversified — a larger
sample (or ones deliberately drawn from a golden-fixture-heavy package)
would be needed to tell "the port under-distinguishes the ladder" apart
from "this sample's tests are all genuinely `asserts`-strength."

## Verdict

Ship as designed: live, fast, cheap (well under a cent for 120 questions
across 2 calls), zero errors, and hand-accurate on every spot-check. Feed
this run's 43 suggestions into TC-M4's twenty-reviewed-suggestion window
before the acceptance-rate bar is judged; watch the strength distribution
on a bigger or more varied sample before concluding anything about the
ladder's own discriminating power.
