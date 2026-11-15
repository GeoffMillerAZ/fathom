LIB-11 verdict: DECIDED 80 for the door, 81 for a consumer (architect ruling 2026-11-15)

Measured on 2026-11-15 at frozen base `a6c67b22038f6f056d267305b830ed5becc74403`.
The original reading failed the authored 80-package consumer bar. W2-1 now
raises that bar by one to 81 while retaining 80 for the door, as recorded in
ADR-092's 2026-11-15 amendment. This is a bar increase, not a change to the
counting population. Raw counts and the original failure remain below.

## Reading

| Population or check | Result | Original authored bar |
|---|---|---|
| Door, `go list -deps ./pkg/lang`, unique non-standard import paths including `pkg/lang` | 80 | At most 80: PASS |
| Committed outside consumer, unique non-standard import paths including its own `main` | 81 | LL-M1: at most 80: FAIL |
| Consumer minus door | Only `example.com/consumer` | Populations differ by the consumer's own package |
| Door minus consumer | Empty | Both use the same door closure |
| CUE, store, fusion, connector, MCP and `net/http` in the door closure | None | None: PASS |
| Outside consumer stripped binary | 24,215,618 bytes, 23.0938 MiB | At most 30 MiB: PASS |
| `TestLangClosure` and `TestOutsideModuleBuilds`, under `-race` | PASS, package result `ok` in 2.300s | Prescribed baseline proof |

Toolchain: `go1.27.4`, `darwin/arm64`, `CGO_ENABLED=1`. Counts are build- and
toolchain-specific. They count packages, not modules or the lines of a test's
augmented dependency graph.

## What holds the closure and which trigger fired

The two guard packages are `internal/readlimit` and `internal/hclbound`. Both
are present in the measured door closure. The historical LL-M1 reading reported
78 packages for the door and 79 including the consumer. The read guarantee added
two leaves, as documented in `docs/design/lang-library.md` section 8 and the
comment above `packageBudget` in `pkg/lang/closure_test.go`.

That comment calls 81 the consumer bar. The authored LL-M1 acceptance in
`docs/design/lang-library.md` section 10 still says 80 including the consumer.
Passing the door's 80-package test does not prove that separate acceptance.
Dropping the consumer's own package from the count would change the population
used by the historical reading.

**ADR-092's trigger "LL-M1 reads FAIL with CUE split" has fired:** the outside
consumer reads 81 against 80 while the measured closure contains no CUE. The
size bar still holds. This calls for the recorded reconsideration of the cuts,
not an automatic increase of either budget or removal of the read guarantees.
The architect's subsequent W2-1 ruling answers this fired trigger by explicitly
raising the consumer bar to 81; the door remains at 80 with zero headroom.
Under that ruling, both measured package counts and the binary size pass.
The two guards remain. HZ-4 and HZ-6 may proceed only without adding a door
package; tagging still waits on LIB-12 and a green wave.

## Method

The committed `pkg/lang/testdata/consumer/{go.mod,go.sum,main.go}` were copied
into a scratch module under `workspace/`. Only its existing relative `replace`
target was rewritten to this checkout, following `TestOutsideModuleBuilds`.
No requirements or sums were regenerated. Both list calls and the consumer
build used:

```sh
GOWORK=off GOPROXY=off GOSUMDB=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local
```

For the door, from the repository root:

```sh
go list -deps ./pkg/lang
go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./pkg/lang
```

For the copied consumer, from its module directory:

```sh
go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' .
go build -ldflags='-s -w' -o consumer .
./consumer
```

Nonempty import paths were deduplicated before counting and comparing the two
sets. Binary size is the resulting file's byte count divided by 1,048,576 for
MiB, using the LL-M1 stripped-build method. The consumer executed successfully.
The prescribed existing tests ran unchanged:

```sh
go test -race ./pkg/lang -run '^(TestLangClosure|TestOutsideModuleBuilds)$' -count=1
```

All builds and tests held the shared gate lease. These are measurements and
baseline tests, not a new implementation's RED/GREEN proof. No full gate ran.

## Ruling and proof boundary

The original blocked handback did not run scratch verification and did not
claim LIB-11. W2-1 authorizes the literal ADR-092 amendment, the section 8
quality-bar sentence, this reading, the retained diagnosis and the LIB-11 box
only after strict scratch verification proves it. Section 10's original 80
and the historical LL-M1 PASS reading remain historical context; the later
ADR-092 amendment governs the current bar. No counting population is redefined.

The [diagnosis](lib-11-consumer-diagnosis.md) preserves the package history and
the earlier unadopted alternative. The selected ruling raises the consumer bar
instead of excluding its root. No code, packages or release tag changes.

## W2 proof before claiming LIB-11

At candidate commit `097226e88`, a source-built binary ingested this checkout
into a fresh scratch store with the 4 GiB tenancy budget. Strict verification
of LIB-11 reported `verified` (one passing row, zero failures); strict
verification of `library-roadmap.md` and `lang-library.md` together exited 0:
92 tasks total, 39 claimed, 39 verified, zero failing. LL-M1 was among the
verified claims. Open release rows remain open. Both prescribed `pkg/lang`
race tests also passed again in 2.405s. All heavy commands held the shared lease
after the controller's gate marker cleared.

LIB-11's lexical proof establishes the reading contract, not runtime behavior;
the separate race test run supplies the runtime evidence stated here. LIB-11
is claimed only after that pre-claim proof. The consolidated final commit is
subject to a fresh scratch ingest and strict verification, recorded in the
lane handback rather than predicted by this reading.
