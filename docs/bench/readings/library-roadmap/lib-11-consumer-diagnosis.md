LIB-11 diagnosis: the extra package is the consumer root; LL-M1 remains FAIL at 81 against 80 (historical pre-W2 verdict)

Reading date: 2026-11-15. Source/history inspected at `468dc705813382d584926887b451cbea145c76ac`.
The retained measurement is of frozen base `a6c67b22038f6f056d267305b830ed5becc74403`,
reported by reading commit `9e879b59e`. This is a diagnosis of that measurement,
not a fresh closure measurement or a plan claim. At diagnosis time W1-2 kept
the bar at 80. The W2-1 disposition below supersedes that freeze.

## LIB-11 evidence: the counting population

Let D be the unique non-standard import paths in the retained `door.txt`, and C
those in `consumer.txt`. Rechecking the files without invoking Go gives:

| Population | Result |
|---|---|
| D, including `pkg/lang` | 80 |
| C, including the outside module's `main` | 81 |
| C minus D | Exactly `example.com/consumer` |
| D minus C | Empty |
| C minus its root `example.com/consumer` | 80 |

The retained script runs `go list -deps` with `{{if not .Standard}}{{.ImportPath}}{{end}}`,
then removes blank lines and deduplicates. It includes the named root, not just
its imports. These are package counts, not module counts or test dependencies.
The outside root is non-standard and therefore counted. "81st" denotes the set
difference, not its position in dependency traversal or sorted output.

The root's import chain points **outward**:

```text
example.com/consumer (package main, the measured root)
  -> github.com/geoffmilleraz/fathom/pkg/lang
     -> the rest of the door closure
```

`pkg/lang/testdata/consumer/main.go:5-13` imports four standard packages
(`context`, `encoding/json`, `fmt`, `os`) and `pkg/lang`. There is no dependency
chain importing `example.com/consumer`: it is selected by `go list ... .` in
its own module. Its module path is committed in the fixture's `go.mod:1`.
The retained set difference proves that this consumer adds no non-standard
dependency beyond itself to D.

## LIB-11 history: entry versus crossing the bar

| Event | Source/history evidence |
|---|---|
| Consumer root introduced | `8c231272676170669c17bd0bdadbbad287b5689c` adds `pkg/lang/testdata/consumer/main.go` and the outside-build test. Its main already imports `pkg/lang`. |
| Fixture module path committed | `19f5d826c3d849cc775786490130302dfbcd7f9a` adds the tidied fixture `go.mod` declaring `module example.com/consumer`. |
| Historical total includes root | `docs/bench/readings/lang-library/m1-closure.md` records 79 with the consumer's main, 78 without, at `b120f2dbb`. Excluding main now would change that population. |
| Shared HCL guard enters door path | `c87dfaea149e2d5ad1f091e2aef847eac7d57662` extracts `internal/hclbound`; `internal/extractor/nesting.go` imports it. Chain: consumer -> `pkg/lang` -> `internal/extractor` -> `internal/hclbound`. |
| Recorded total becomes 81 | `fc42d0cb5b887bb2cf401bf17d7b80d3c40db896` adds `pkg/lang/reader.go`'s direct `internal/readlimit` import and deadline wiring. Its message explicitly records one over LL-M1 with main counted; its closure-test diff changes the door budget from 79 to 80 and describes 80 door / 81 consumer. |

Thus the root was not a newly introduced transitive dependency at the breach.
The read-guarantee work added two library leaves relative to the original 78:
`hclbound` and `readlimit`. The latter was created in `4863fc323` and wired into
the public Reader in `fc42d0cb5`. This history is source evidence, not a new
execution of historical builds under today's toolchain.

## LIB-11 escalation: smallest sound correction requires approval

No source cut is warranted merely to remove the consumer-only entry: deleting
the consumer removes the subject, and removing its library import removes the
outside-use proof. Removing either guard would weaken ADR-092's read guarantee.
The retained delta identifies no accidental transitive package to delete.

The smallest accounting correction to consider is to define the budgeted
population as **unique non-standard packages reachable from the consumer,
excluding only its own root import path and including `pkg/lang`**. Always
report the raw total alongside it. That population reads 80 here, with raw
total 81. It must not exclude all main packages, all consumer-module packages,
or library guard packages. A consumer adding another non-standard dependency
would still consume budget.

This is a proposed acceptance-definition amendment, not an arithmetic fix to
the existing acceptance: section 10 of `docs/design/lang-library.md` budgets
the outside consumer, and the historical reading explicitly includes main.
It requires architect approval under W1-2 and reconciliation of ADR-092,
the design, acceptance/checker coverage and readings before any PASS claim.
The closure-test comment calling 81 the consumer bar supplies no authority to
change the authored 80. Nothing is amended by this reading.

If the architect retains the historical population, the required source cut
is at least one actual package from D: door at most 79, consumer at most 80.
These artifacts do not prove a safe implementation of that cut. Moving or
merging guard code solely to erase a package boundary is not established as
safe or warranted by the root-count discrepancy. Escalate that choice rather
than weaken the guarantee or disguise 81 as a pass. ADR-092's "LL-M1 reads FAIL
with CUE split" revisit remains fired. Library tagging and HZ-4/HZ-6 additions
remain held under W1-2.

## LIB-11 proof and limits

Retained evidence supplied by lane 253: `workspace/tmp/lib-11/{measure.sh,door.txt,consumer.txt}`
and `workspace/logs/lib-11-measure.log`. SHA-256 identities:

```text
door.txt     208903ea8a4c8e00826fdb74d1d4b29668d70cbf627f593b9458610570a66565
consumer.txt cd649d7f870bc38697aa5a6be8f7da14f719c00a892cdde1802dca0deb88d607
measure.sh   00b6f7ae8ae96b9e2f5b3ad6de28402e2d0a99beb004a2204310f7b4fe652b76
```

A Python check asserted unique counts 80/81, both set differences above,
the root-excluded count 80, and the retained binary's 24,215,618-byte size.
The retained log records Go 1.27.4, darwin/arm64, cgo enabled, no forbidden
packages, both guards present, and prescribed race tests passing in 2.300s.
Those tests were not rerun here. No build, test, ingest, verify or full gate
ran in this diagnosis; the shared gate was running at the final evidence check.
Only retained artifacts and source/history were used. No threshold, package,
code, plan claim or library tag changes.

## LIB-11 W2-1 disposition (2026-11-15)

The architect selected a consumer-bar increase from 80 to 81, retaining the
original population including its own main package and a door bar of 80.
The root-excluded alternative above was not adopted. Raw counts remain 80
and 81. ADR-092's amendment answers the fired LL-M1 revisit trigger; the door
has zero package headroom. A change adding a door package must remove one or
return for a ruling. W1-2's freeze is replaced: HZ-4 and HZ-6 may proceed only
without adding a door package. The tag still waits on LIB-12 and a green wave.
The historical diagnosis and its limits above are retained as evidence of
what was known before the ruling, not as current holds or a current FAIL.
