Synthetic format controls; actual source extraction and scratch-store symbol joins; runtime coverage and oracle accuracy unmeasured.

# FCT-10 labelled feasibility reading

The two fresh-store trials pass the approved **feasibility structural controls: PASS**.
This establishes deterministic structural feasibility for these five public fixture controls only.
The original population is **Go plus four T3 formats**, at least five languages with
blocks joined to symbols; it was not read or replaced. **FCT-10 and TQ-M3 remain open**.
No plan box is changed, and no lexical verdict match authorizes completion.

Authority: architect ruling T4-2 and the approved
[immutable contract](../../../contracts/track-free-measurements.md), source
`19c068e82`, approval receipt `df5cc06c5`, independent LAND `02fcdcd6b`,
local contract commit `0444f502a908a1263ddece18e3aa9e9f71bd5d5e`.
The governing design is `docs/design/test-intelligence.md:168` (TQ-M3),
with `docs/plans/realignment.md` FCT-10 and TQ-9..14w. I38 and P7/P22
require complete inputs and earned conclusions; principles T5 keeps criteria
unchanged after data. ADR-051 decisions 1–2 require existing assets and joins.
No applicable intent trigger is established. There is no new method, bar or
population, and no product implementation change.

## Frozen method and provenance

Production/fixture base: `83226aece9b0f421381373f8dafc4a912d3888e5`.
Execution HEAD: `0444f502a908a1263ddece18e3aa9e9f71bd5d5e`;
tree: `df5f3dd0fe6403d2cdd120ad6f4ef2fe4d4e0b44`.
Toolchain: `go version go1.27.4 darwin/arm64`; OS/architecture: `Darwin arm64`.
All 21 original input hashes and five availability-example hashes below matched
both pinned-base and execution bytes before ingestion and were rechecked before
each trial. The five copied sources were byte-identical; only those five files
were ingested. Goldens supplied expectations only, never database nodes.

The reviewed scratch runner was reused; its sole source adjustment was manifest
cardinality 21 → 26 to include the approved amendment's five availability hashes.
No execution, expectation, transformation, parser, join or control logic changed.
Named raw Go literals were extracted through `go/parser` and `strconv.Unquote`,
requiring exactly one scoped match. Report bytes and SHA-256 were serialized and
frozen before the first ingest; actual symbol ranges never moved report spans.
Each trial opened its own nonexistent SQLite path, called production
`client.NewConstantClient`, `Client.Ingest`, and `Client.ListAllSymbols`, and
passed the full actual indexed node set to the production coverage parsers and
`JoinForeignBlocks`. No positive node was constructed or injected.

Scratch artifacts are relative to `workspace/tmp/free-measurements/`:
`trial-1.sqlite` and `trial-2.sqlite` are distinct new stores;
`trial-1.json` and `trial-2.json` are fresh raw receipts, not prior critic probes.
The runner refuses existing stores. `corpus/`, `seeds/`, `reports/`,
`manifest.json`, `report-hashes.json`, `frozen-inputs.json`, `canonical-1.json`,
`canonical-2.json`, `refusal-receipts.json`, and `audit-results.json` retain the
local reproducibility material. Raw trial rows and exact report/runner bytes are
published below; scratch databases and logs are not bundled.

All heavy commands ran serially under the existing machine-wide gate lease.
The prescribed lane environment was sourced in every shell. The scratch wrapper
set the following exports before the commands below and retained the gate lock:

```sh
export TMPDIR="$PWD/workspace/tmp/free-measurements/tmp"
export GOTMPDIR="$PWD/workspace/tmp/free-measurements/gotmp"
export FATHOM_HOME="$PWD/workspace/tmp/free-measurements/home"
export FATHOM_STATE_DIR="$PWD/workspace/tmp/free-measurements/state"
unset FATHOM_VERIFY_LEDGER FATHOM_INTENT_LEDGER FATHOM_SESSION_LEDGER FATHOM_HOOKS_LOG
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
```

Home/state directories were fresh. Overrides were unset without reading values.
The five source programs were extracted, never executed; imports were not resolved
by running native programs. No network fetch, dependency change, daemon, shared
private-store ingest, model call or full gate was used.

## Results for both trials

Each trial returned nil ingest/list error: Discovered=5, Extracted=5, Symbols=79,
Skipped=0, ReadFailed=0, Oversize=0, SkippedGenerated=0; unsupported languages
absent. All five distinct source paths were accounted for and every callable had
EndLine > 0. Full ingest reports, including all other counters and raw pass
timings, are published below. Timings are incidental; this is not a latency reading.

| Language / format | Actual target / kind | Range | Indexed nodes | Blocks / statements / covered | Percent | Symbol joins | File fallback / unmatched | Assertions, trial 1 / 2 |
|---|---|---|---:|---|---:|---:|---|---|
| python / coverage.py | `top` / PythonFunc | 13–17 | 22 | 3 / 3 / 2 | 66.6666666667 | 1 | 0 / 0 | PASS / PASS |
| javascript / Istanbul | `create` / JSFunc | 17–21 | 17 | 2 / 2 / 1 | 50 | 1 | 0 / 0 | PASS / PASS |
| java / JaCoCo | `Widget.sum` / JavaMethod | 13–21 | 15 | 2 / 2 / 1 | 50 | 1 | 0 / 0 | PASS / PASS |
| ruby / lcov | `Pipeline.run` / RubyMethod | 6–16 | 5 | 3 / 3 / 2 | 66.6666666667 | 1 | 0 / 0 | PASS / PASS |
| c / llvm-cov | `add` / CFunc | 36–41 | 20 | 2 / 2 / 2 | 100 | 1 | 0 / 0 | PASS / PASS |

`InputsFrozen`, `SourcesReallyIngested`, `FormatsParsed`,
`FiveActualSymbolJoins`, `UnmatchedNeverGuessed`, `UnrangedDemotesFile`,
`OutOfRangeDoesNotBecomeCoverage` and `Repeatable` all pass in both trials.
The extra ingest/path assertions also pass. Tests arrays are empty/nil: hit counts
provide no per-test attribution. There is no mean hiding a missing language.

Every language/trial has one positive symbol join with its actual indexed Key,
real target range and raw table totals, with no positive ByFile or Unmatched.
Separate unknown-file controls yield no coverage and exactly
`absent/<language>.missing`; separate target-EndLine=0 mutations yield one file
fallback with unchanged totals and no symbol join/unmatched. These deliberately
mutated copies do not describe an extractor range failure or enter the positive
denominator. Outside-range controls yield no BySymbol, ByFile or Unmatched.
Their omitted spans are explicitly retained below; they are never subtracted
from positive denominators:

- python: [[74, 74], [74, 74], [74, 74]] in each trial.
- javascript: [[32, 32], [32, 34]] in each trial.
- java: [[47, 47], [47, 47]] in each trial.
- ruby: [[38, 38], [38, 38], [38, 38]] in each trial.
- c: [[61, 61], [61, 61]] in each trial.

Canonical projections sort nodes by file/line/kind/name/key, blocks by
file/start/end/statements/covered, and join results by file/line/key; all control
results are retained. Raw ingest timing fields are excluded. Both complete
canonical byte strings are identical, SHA-256 `ebae585ca7ed15b9038e9752af13f037edb2f621cea893bf3c726e39dd0cda7a`.

## Independent validation and limits

The contract-specific checker first failed on the literal missing report
(`report-red.log`, exit 1), before this document existed. The independent Python
audit rehashed all 26 base/current/copy inputs, inverted all five location-only
transformations, recalculated every block/range/count/percent and actual symbol
membership from raw nodes, recomputed every negative control, and checked both
canonical projections and byte equality. It did not accept runner boolean flags
as sufficient evidence. Scratch-only source drift, report drift, missing report,
and existing-but-unreadable report (directory in file slot) were each refused
before ingest with exit 1; restoration passed with exit 0. Raw receipts follow.
The completed report checker passed and rejected meaningful altered-count,
cohort-substitution and completion mutants (each exit 1).

Commands and exits (all heavy commands under the lease, serially):

```sh
bash scripts/gate-lease.sh go run -race ./workspace/tmp/free-measurements/runner
# exit 0: Synthetic structural assertions PASS; runtime coverage and oracle accuracy unmeasured
bash scripts/gate-lease.sh python3 workspace/tmp/free-measurements/audit.py
# exit 0: independent recomputation PASS; fresh check-only refusal runs use go run -race
bash scripts/gate-lease.sh go test -race ./internal/analysis/coverage -run 'Test(ParseLcov|ParseCoberturaJaCoCo|ParseCoveragePyIstanbul|ParseLLVMCov|JoinForeignBlocks)' -count=1
# exit 0
bash scripts/gate-lease.sh go test -race ./cmd/fathom -run '^Test(CmdCoverageIngest.*|BoundedReader_CapEnforced|TestsCoverageJoinsForeignBlocks)$' -count=1
# exit 0
python3 workspace/tmp/free-measurements/check-report.py
# exit 0 after report; missing-report RED and three report mutants exit 1
```

Package regressions establish their named parser/join and CLI size-cap/binding
behavior only; they are not the five-language measurement or a native producer
trial. The API trial does not itself test CLI arguments/size caps. Locale warnings
came from the lease wrapper; no race diagnostic or trial error occurred.

Synthetic hits and fixture summary metadata are not execution truth. This reading
measures neither runtime coverage, native producer interoperability, independent
or outside-oracle accuracy, whole-language accuracy, temporal behavior,
default-on/support behavior, benefit, deployment nor fleet health. It supplies no
new native-producer/oracle accuracy bar. No original input has been fabricated
from feasibility success, and no Go control has been substituted after data.
A separate fresh critic is required after this report lane; this document does
not claim its own approval or any plan completion. CHG-M2 remains deferred under
T4-3; no ENV work or other reading is claimed.

## Original cohort input inventory

Inventory basis: the pinned base above, `docs/design/test-intelligence.md` T3 and
section 5 line 168, `docs/plans/test-intelligence.md` TQ-9..14w/TQ-M3, and
`docs/plans/realignment.md` FCT-10. The design supplies the population but does
not choose the four formats or their language/source/report pairs. Searches of
`docs/bench/` and `docs/contracts/` for TQ-M3, FCT-10 and coverage-languages
found no original-cohort preregistration beyond these governing requirements;
the present contract registers the separate feasibility controls only.
`docs/bench/readings/tests/coverage-languages.md` is absent at base, so there is
no original reading there to reuse. This is not evidence that parser or Go
examples are absent.

| Required original input | Available record or fixture | Exact remaining gap |
|---|---|---|
| Frozen original cohort registration | Design line 168: Go plus four T3 formats, at least five languages; TQ-M3 and FCT-10 remain unchecked | No frozen original roster selecting the four formats, their distinct languages, source/report paths, target symbols/ranges, byte hashes and report-to-source/commit provenance. The five controls above register a different feasibility population. No new four-format selection is made here. |
| Go source/profile pair | **available-as-example**: `cmd/fathom/review_test.go` `reviewFixtureSrc` defines `pkg/a.go`, function `A` at lines 3–5. `cmd/fathom/tests_coverage_test.go` `testsCoverageFixture` ingests it; `writeCoverProfile` constructs `cover.out` with a `mode: set` block using the extracted range; `TestTestsCoverage_JSONReportsJoinedSymbol_BarePathDefaultsUnit` joins it. `internal/analysis/coverage/profile_test.go` `TestParseProfile_Set` is a parser example. | No Go pair selected and frozen for the original reading, with exact source/profile bytes, hashes and binding/provenance. The CLI example generates the profile from extracted ranges; it is not a registered, frozen original measurement pair. |
| Four T3 format/language/source-report pairs | All six parser seeds exist (listed next); the five source fixtures and goldens in the immutable manifest exist | No governing selection of which four formats and languages, nor frozen corresponding source/report pairs and their source locations, hashes, provenance and expected joins. Existing seeds alone do not identify the original four; synthetic path/line adaptation does not supply that registration. |
| Commit association and reading receipts | T3 and TQ-9 require a report commit or command-line binding; an unbound report never gates. `cmd/fathom/coverage_ingest_test.go` `TestCmdCoverageIngestBindsCommit` supplies regression evidence | No original-cohort report binding/provenance or original trials/results is registered in the examined records. A source commit alone does not prove a report was produced from that source. |

The available T3 seeds are in `internal/analysis/coverage/` at the base and
hashes in the immutable manifest:

| Format | Exact seed symbol/local and original source location |
|---|---|
| lcov | `lcov_test.go`, `TestParseLcov` local `in`: `src/widget.py` |
| Cobertura | `cobertura_jacoco_test.go`, `coberturaSample`: `src/widget.py` |
| JaCoCo | `cobertura_jacoco_test.go`, `jacocoSample`: package `com/widget`, sourcefile `Widget.java` |
| coverage.py | `coveragepy_istanbul_test.go`, `coveragePySample`: `src/widget.py` |
| Istanbul | `coveragepy_istanbul_test.go`, `istanbulSample`: `/repo/src/widget.js` (fixture report path) |
| llvm-cov | `llvmcov_test.go`, `llvmCovSample`: `src/widget.c` |

`cmd/fathom/tests_coverage_foreign_test.go` `foreignCoverageFixture` and
`TestTestsCoverageJoinsForeignBlocks` are also available-as-example: lcov
`cov.info` joins `pkg/a.go` function `A`; coverage.py `cov.json` names
`pkg/consts.go` and demonstrates file fallback; the unmatched report entry is
preserved. These are reports over Go fixtures, not four other languages. The
additional Go/example files are pinned to the same base for this inventory:

| Path | SHA-256 |
|---|---|
| `cmd/fathom/review_test.go` | `59d1b4f8e4fe97ad81ca9c623fd81d272707039604cafc58b379f7fb520b1252` |
| `cmd/fathom/tests_coverage_test.go` | `61cec388d37bbb9558566d4c26a83a0c6a3911cb5a48814fa80a09bbcc7bd689` |
| `cmd/fathom/tests_coverage_foreign_test.go` | `e183b416022d0ccc9ae96d98095103bfc55e2448e1477983a60bd3ace86fd05c` |
| `cmd/fathom/coverage_ingest_test.go` | `aae2c71b10a0f1614c3b415a6d83e61b7dba9b99d32d6e403f34beaea6eae781` |
| `internal/analysis/coverage/profile_test.go` | `a8af73df666626ebf7630dc611e09377722e6309d690c69a29c93985efc823a2` |

These are literal missing registration/input/receipt items, not a new demand
for native producers or independent oracles. T3/TQ-M3 do not prescribe a new
native-producer/oracle accuracy bar. Native producer interoperability and runtime
accuracy remain **unmeasured**, separate from the missing original inputs.
Nothing here promotes synthetic hits to execution truth or changes the original
cohort or its bar after probes exist.


## Frozen original 21 hashes

| Path | SHA-256 |
|---|---|
| `docs/plans/realignment.md` | `2392810232371350c4319a06a28fc9d0a814e00a90e3b0a55d8f3fea3f0cda54` |
| `docs/plans/test-intelligence.md` | `dc841711976406fe12a40077bf573e79814eeda68b3bd4a0693a8abba16791c8` |
| `docs/design/test-intelligence.md` | `13c91bd4ce4472df2813f8da4c10a701b59981555952ed49ff02439995b9d10d` |
| `pkg/client/client.go` | `ef58f04c1e631d3690ea658aa7ccc2242dfc4e19c19870cd24ef92ca30bd6b74` |
| `internal/analysis/coverage/join_foreign.go` | `50eb5134fa326913b15bf2c8e1a3c3efce82e2ae9ba4ba9405e6e45fdcca092b` |
| `cmd/fathom/coverage_ingest.go` | `9f9415d829ce1824c786b06209be231df432e77aaa66b13a1535d004c7b745cd` |
| `cmd/fathom/tests_coverage.go` | `3efa20f074e0b91f292a226aac56db99e5451cbc778d9f507ab3a6d2901915bb` |
| `internal/analysis/coverage/coveragepy_istanbul_test.go` | `945b00c686662c024e865121b44d4518dda402dc0e72b701dc100c71de21193b` |
| `internal/analysis/coverage/cobertura_jacoco_test.go` | `9fc485e927793eb12f0491c98a9756acaffa78c48790424981dd4acdee5fbcd2` |
| `internal/analysis/coverage/lcov_test.go` | `e7edd75d601c2a614ce2f50beb257edd3d2a3a2f8d4de8768280b8911e9849bf` |
| `internal/analysis/coverage/llvmcov_test.go` | `6a88bd985308fd3c049f68aa24b234f200a67723826c0a299ecd3f8e508ea606` |
| `internal/extractor/dialects/python/testdata/shapes.py` | `a2a6243afe55d7d89bba43418061e31ef1a1f76af730721ecfa7fcc529134308` |
| `internal/extractor/dialects/python/testdata/shapes.py.golden.json` | `f15ac834ee65ec34bbba64fcbf986b46512536c03be98bc72f9363add390f90d` |
| `internal/extractor/dialects/javascript/testdata/bindings.js` | `05732e3c4dce18143c3537fb550ca7cb7ee4bbba8db59b54540528420626026e` |
| `internal/extractor/dialects/javascript/testdata/bindings.js.golden.json` | `b2d156efbe8b2d6e82dfc31373b2d524aa10428a87c536fcb7619df7dfd632a7` |
| `internal/extractor/dialects/java/testdata/shapes.java` | `07f549be82864141ac5b106e1e7035e30574a1393d576946c4538c104d934ea9` |
| `internal/extractor/dialects/java/testdata/shapes.java.golden.json` | `513e002582ae9a2d6725920783c5ac64154593fdcade3d2405b0344117a8478b` |
| `internal/extractor/dialects/ruby/testdata/calls.rb` | `6282795432999e81b7d87c60dc8cc81e084aa316c8cc88070be2367e3c2943de` |
| `internal/extractor/dialects/ruby/testdata/calls.rb.golden.json` | `67ab82888eac9f2510f421a93b85821b491325df01d062e4353e9d7293a6dc59` |
| `internal/extractor/dialects/c/testdata/shapes.c` | `589187d168b9975ee4763a439ce769c432cfd378cadf68595058dbd9e63b4656` |
| `internal/extractor/dialects/c/testdata/shapes.c.golden.json` | `e057a842141be1dee76bee7c23a35fa13a59605921a110af23c85701fa3c6056` |

## Location transformations frozen before ingestion

Source copies come from `internal/extractor/dialects/<language>/testdata/`;
the exact source and golden hashes appear in the manifests. The following named
raw literals come from `internal/analysis/coverage/`:

| Language | Seed file / scoped literal | Frozen source path | Line shift |
|---|---|---|---:|
| Python | coveragepy_istanbul_test.go / coveragePySample | python/shapes.py | +12 |
| JavaScript | coveragepy_istanbul_test.go / istanbulSample | javascript/bindings.js | +16 |
| Java | cobertura_jacoco_test.go / jacocoSample | java/shapes.java | +12 |
| Ruby | lcov_test.go / TestParseLcov local in | ruby/calls.rb | +5 |
| C | llvmcov_test.go / llvmCovSample | c/shapes.c | +35 |

Python changes the files key and executed/missing lines; JavaScript changes its
key/path and every statement start/end line; Java changes only sourcefile name
and line nr, retaining package/counters; Ruby changes SF and DA/FN line numbers;
C changes filename and every segment line, including skipped/gap/no-count
segments. Hits, IDs, columns, flags, names and summary metadata are retained.
C line 37 OR-merges counted segments; shifted lines 38/39 emit no blocks.

## Exact transformed report bytes

### python

SHA-256: `881aa2b7d535b2a75802ddb908056c36b17ac1be53db39652e8c01968576d2ef`. Only the contract's path and line offsets were changed;
JSON uses sorted keys/two-space indent/final newline; XML retains fixed UTF-8
seed serialization and final newline; lcov retains record order and final newline.

```json
{
  "files": {
    "python/shapes.py": {
      "executed_lines": [
        13,
        15
      ],
      "missing_lines": [
        14
      ]
    }
  },
  "meta": {
    "format": 3,
    "version": "7.4.0"
  },
  "totals": {
    "covered_lines": 2
  }
}
```

### javascript

SHA-256: `b76309faaf10811cd1039a7b61f4ba9f041204a078d20d1e26b8f112601db2a8`. Only the contract's path and line offsets were changed;
JSON uses sorted keys/two-space indent/final newline; XML retains fixed UTF-8
seed serialization and final newline; lcov retains record order and final newline.

```json
{
  "javascript/bindings.js": {
    "path": "javascript/bindings.js",
    "s": {
      "0": 1,
      "1": 0
    },
    "statementMap": {
      "0": {
        "end": {
          "column": 10,
          "line": 17
        },
        "start": {
          "column": 0,
          "line": 17
        }
      },
      "1": {
        "end": {
          "column": 1,
          "line": 21
        },
        "start": {
          "column": 0,
          "line": 19
        }
      }
    }
  }
}
```

### java

SHA-256: `f38bc1202b3600abd2de16dde0f67a6360c683f4041ec2156055ab0069654b5d`. Only the contract's path and line offsets were changed;
JSON uses sorted keys/two-space indent/final newline; XML retains fixed UTF-8
seed serialization and final newline; lcov retains record order and final newline.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<report name="widget">
  <package name="com/widget">
    <sourcefile name="java/shapes.java">
      <line nr="13" mi="0" ci="2" mb="0" cb="0"/>
      <line nr="14" mi="1" ci="0" mb="0" cb="0"/>
    </sourcefile>
  </package>
</report>
```

### ruby

SHA-256: `5671796a8a781fde96e5dd5aebaffbfcd5fcb215343386f378614a9cf147ac98`. Only the contract's path and line offsets were changed;
JSON uses sorted keys/two-space indent/final newline; XML retains fixed UTF-8
seed serialization and final newline; lcov retains record order and final newline.

```text
TN:
SF:ruby/calls.rb
DA:6,1
DA:7,0
DA:10,3
FN:6,widget
FNDA:1,widget
FNF:1
FNH:1
LF:3
LH:2
end_of_record
```

### c

SHA-256: `e3b2622ce867f96f061b84f389a5700c1693220cd7222d7641c64e24db3c4a24`. Only the contract's path and line offsets were changed;
JSON uses sorted keys/two-space indent/final newline; XML retains fixed UTF-8
seed serialization and final newline; lcov retains record order and final newline.

```json
{
  "data": [
    {
      "files": [
        {
          "filename": "c/shapes.c",
          "segments": [
            [
              36,
              1,
              3,
              true,
              true,
              false
            ],
            [
              37,
              1,
              0,
              true,
              true,
              false
            ],
            [
              37,
              5,
              3,
              true,
              false,
              false
            ],
            [
              38,
              1,
              0,
              false,
              false,
              false
            ],
            [
              39,
              1,
              0,
              true,
              true,
              true
            ]
          ]
        }
      ]
    }
  ],
  "type": "llvm.coverage.json.export",
  "version": "2.0.1"
}
```

## Reused scratch runner

SHA-256: `b0f93208c586d46c134d408803e71d96cfbd19d0d4fe77f0923b957b5954829b`.

```go
// Independent scratch conformance runner. Assertions follow track-free-measurements.md.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	cov "github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const root = "workspace/tmp/free-measurements"

type caseDef struct {
	Lang, Source, Target, Kind, SeedFile, Seed, Func string
	Start, End, Shift, Count, Covered                int
	Lines, Ends, Hits                                []int
}

var cases = []caseDef{
	{"python", "shapes.py", "top", "PythonFunc", "coveragepy_istanbul_test.go", "coveragePySample", "", 13, 17, 12, 3, 2, []int{13, 14, 15}, []int{13, 14, 15}, []int{1, 0, 1}},
	{"javascript", "bindings.js", "create", "JSFunc", "coveragepy_istanbul_test.go", "istanbulSample", "", 17, 21, 16, 2, 1, []int{17, 19}, []int{17, 21}, []int{1, 0}},
	{"java", "shapes.java", "Widget.sum", "JavaMethod", "cobertura_jacoco_test.go", "jacocoSample", "", 13, 21, 12, 2, 1, []int{13, 14}, []int{13, 14}, []int{1, 0}},
	{"ruby", "calls.rb", "Pipeline.run", "RubyMethod", "lcov_test.go", "in", "TestParseLcov", 6, 16, 5, 3, 2, []int{6, 7, 10}, []int{6, 7, 10}, []int{1, 0, 1}},
	{"c", "shapes.c", "add", "CFunc", "llvmcov_test.go", "llvmCovSample", "", 36, 41, 35, 2, 2, []int{36, 37}, []int{36, 37}, []int{1, 1}},
}

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func read(p string) []byte { b, e := os.ReadFile(p); must(e); return b }
func write(p string, b []byte) {
	must(os.MkdirAll(filepath.Dir(p), 0755))
	must(os.WriteFile(p, b, 0644))
}
func enc(v any) []byte     { b, e := json.MarshalIndent(v, "", "  "); must(e); return append(b, '\n') }
func hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func require(ok bool, s string) {
	if !ok {
		panic(s)
	}
}
func source(c caseDef) string {
	return "internal/extractor/dialects/" + c.Lang + "/testdata/" + c.Source
}
func rel(c caseDef) string { return c.Lang + "/" + c.Source }
func manifest() map[string]string {
	re := regexp.MustCompile("(?m)^\\| `([^`]+)` \\| `([0-9a-f]{64})` \\|$")
	m := map[string]string{}
	for _, v := range re.FindAllSubmatch(read("docs/contracts/track-free-measurements.md"), -1) {
		m[string(v[1])] = string(v[2])
	}
	require(len(m) == 26, "manifest count")
	return m
}
func checkInputs(m map[string]string, reports map[string]string, corpus bool) error {
	for p, w := range m {
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		if hash(b) != w {
			return fmt.Errorf("InputsFrozen: drift %s", p)
		}
	}
	if corpus {
		for _, c := range cases {
			b, e := os.ReadFile(root + "/corpus/" + rel(c))
			if e != nil {
				return e
			}
			if hash(b) != m[source(c)] {
				return fmt.Errorf("InputsFrozen: copied source drift %s", rel(c))
			}
		}
	}
	for p, w := range reports {
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		if len(b) >= 256*1024*1024 || hash(b) != w {
			return fmt.Errorf("InputsFrozen: report drift %s", p)
		}
	}
	return nil
}
func seed(c caseDef) string {
	f, e := parser.ParseFile(token.NewFileSet(), "internal/analysis/coverage/"+c.SeedFile, nil, 0)
	must(e)
	var found []string
	for _, d := range f.Decls {
		if c.Func != "" {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name.Name != c.Func {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				a, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for i, l := range a.Lhs {
					if id, ok := l.(*ast.Ident); ok && id.Name == c.Seed {
						v, ok := a.Rhs[i].(*ast.BasicLit)
						require(ok && v.Kind == token.STRING, "seed literal")
						s, e := strconv.Unquote(v.Value)
						must(e)
						found = append(found, s)
					}
				}
				return true
			})
		} else {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, sp := range gd.Specs {
				v, ok := sp.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, n := range v.Names {
					if n.Name == c.Seed {
						lit, ok := v.Values[i].(*ast.BasicLit)
						require(ok && lit.Kind == token.STRING, "seed literal")
						s, e := strconv.Unquote(lit.Value)
						must(e)
						found = append(found, s)
					}
				}
			}
		}
	}
	require(len(found) == 1, "seed unique "+c.Seed)
	return found[0]
}
func obj(v any) map[string]any { return v.(map[string]any) }
func arr(v any) []any          { return v.([]any) }
func transform(c caseDef, s string) []byte {
	if c.Lang == "java" {
		s = strings.ReplaceAll(s, `name="Widget.java"`, `name="java/shapes.java"`)
		re := regexp.MustCompile(`nr="([0-9]+)"`)
		s = re.ReplaceAllStringFunc(s, func(v string) string {
			n, e := strconv.Atoi(re.FindStringSubmatch(v)[1])
			must(e)
			return fmt.Sprintf(`nr="%d"`, n+c.Shift)
		})
		return []byte(s)
	}
	if c.Lang == "ruby" {
		lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
		for i, v := range lines {
			if strings.HasPrefix(v, "SF:") {
				lines[i] = "SF:" + rel(c)
			}
			if strings.HasPrefix(v, "DA:") || strings.HasPrefix(v, "FN:") {
				p := strings.SplitN(v[3:], ",", 2)
				n, e := strconv.Atoi(p[0])
				must(e)
				lines[i] = fmt.Sprintf("%s%d,%s", v[:3], n+c.Shift, p[1])
			}
		}
		return []byte(strings.Join(lines, "\n") + "\n")
	}
	var v map[string]any
	must(json.Unmarshal([]byte(s), &v))
	switch c.Lang {
	case "python":
		files := obj(v["files"])
		entry := obj(files["src/widget.py"])
		delete(files, "src/widget.py")
		files[rel(c)] = entry
		for _, key := range []string{"executed_lines", "missing_lines"} {
			a := arr(entry[key])
			for i, x := range a {
				a[i] = x.(float64) + float64(c.Shift)
			}
		}
	case "javascript":
		entry := obj(v["/repo/src/widget.js"])
		delete(v, "/repo/src/widget.js")
		v[rel(c)] = entry
		entry["path"] = rel(c)
		for _, st := range obj(entry["statementMap"]) {
			for _, p := range []string{"start", "end"} {
				loc := obj(obj(st)[p])
				loc["line"] = loc["line"].(float64) + float64(c.Shift)
			}
		}
	case "c":
		for _, d := range arr(v["data"]) {
			for _, f := range arr(obj(d)["files"]) {
				obj(f)["filename"] = rel(c)
				for _, segment := range arr(obj(f)["segments"]) {
					a := arr(segment)
					a[0] = a[0].(float64) + float64(c.Shift)
				}
			}
		}
	}
	return enc(v)
}

type projection struct {
	File, Name, Kind, Key string
	Line, EndLine         int
}

func project(nodes []client.SymbolNode) []projection {
	out := make([]projection, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, projection{n.FilePath, n.Name, string(n.Kind), n.Key(), n.Line, n.EndLine})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Key < b.Key
	})
	return out
}
func callable(k string) bool {
	for _, s := range []string{"Func", "Method", "Fn", "Function"} {
		if strings.HasSuffix(k, s) {
			return true
		}
	}
	return false
}

type row struct {
	Language                          string
	Blocks                            []cov.ForeignBlock
	Join                              cov.ForeignJoinResult
	UnmatchedBlocks, OutOfRangeBlocks []cov.ForeignBlock
	Unmatched, Unranged, OutOfRange   cov.ForeignJoinResult
	Checks                            map[string]bool
}
type trial struct {
	Report client.Report
	Nodes  []projection
	Rows   []row
	Checks map[string]bool
	Errors []string
}

func runTrial(i int, m, reports map[string]string) trial {
	must(checkInputs(m, reports, true))
	db := fmt.Sprintf("%s/trial-%d.sqlite", root, i)
	_, e := os.Stat(db)
	require(os.IsNotExist(e), "fresh DB required")
	cli, e := client.NewConstantClient(client.ModeLocal, db)
	must(e)
	defer cli.Close()
	corpus, e := filepath.Abs(root + "/corpus")
	must(e)
	report, e := cli.Ingest(context.Background(), client.ServiceRef{ID: "free-coverage", Path: corpus})
	t := trial{Report: report, Checks: map[string]bool{"InputsFrozen": true}}
	if e != nil {
		t.Errors = append(t.Errors, e.Error())
	}
	nodes, ne := cli.ListAllSymbols(context.Background(), "free-coverage")
	if ne != nil {
		t.Errors = append(t.Errors, ne.Error())
	}
	t.Nodes = project(nodes)
	t.Checks["IngestReport"] = e == nil && ne == nil && report.Discovered == 5 && report.Extracted == 5 && report.Skipped == 0 && report.ReadFailed == 0 && report.Oversize == 0 && report.SkippedGenerated == 0 && len(report.UnsupportedLanguages) == 0
	paths := map[string]bool{}
	for _, n := range nodes {
		paths[n.FilePath] = true
	}
	t.Checks["FiveSourcePaths"] = len(paths) == 5
	for _, c := range cases {
		t.Checks["FiveSourcePaths"] = t.Checks["FiveSourcePaths"] && paths[rel(c)]
	}
	for _, c := range cases {
		r := row{Language: c.Lang, Checks: map[string]bool{}}
		data := read(root + "/reports/" + c.Lang + ".report")
		var pe error
		switch c.Lang {
		case "python", "javascript":
			r.Blocks, pe = cov.ParseCoveragePyIstanbul(bytes.NewReader(data))
		case "java":
			r.Blocks, pe = cov.ParseCoberturaJaCoCo(bytes.NewReader(data))
		case "ruby":
			r.Blocks, pe = cov.ParseLcov(bytes.NewReader(data))
		case "c":
			r.Blocks, pe = cov.ParseLLVMCov(bytes.NewReader(data))
		}
		if pe != nil {
			t.Errors = append(t.Errors, c.Lang+": "+pe.Error())
		}
		sort.Slice(r.Blocks, func(i, j int) bool {
			a, b := r.Blocks[i], r.Blocks[j]
			if a.File != b.File {
				return a.File < b.File
			}
			if a.StartLine != b.StartLine {
				return a.StartLine < b.StartLine
			}
			if a.EndLine != b.EndLine {
				return a.EndLine < b.EndLine
			}
			if a.Statements != b.Statements {
				return a.Statements < b.Statements
			}
			return a.Covered < b.Covered
		})
		good := pe == nil && len(r.Blocks) == c.Count
		if good {
			for i, b := range r.Blocks {
				good = good && b.File == rel(c) && b.StartLine == c.Lines[i] && b.EndLine == c.Ends[i] && b.Statements == 1 && b.Covered == c.Hits[i] && len(b.Tests) == 0
			}
		}
		r.Checks["FormatsParsed"] = good
		idx := -1
		targets := 0
		ranged := true
		maxEnd := 0
		for j, n := range nodes {
			if n.FilePath != rel(c) {
				continue
			}
			if callable(string(n.Kind)) {
				ranged = ranged && n.EndLine > 0
				if n.EndLine > maxEnd {
					maxEnd = n.EndLine
				}
			}
			if n.Name == c.Target && string(n.Kind) == c.Kind && n.Line == c.Start && n.EndLine == c.End {
				idx = j
				targets++
			}
		}
		r.Checks["SourcesReallyIngested"] = targets == 1 && ranged
		r.Join = cov.JoinForeignBlocks(r.Blocks, nodes)
		good = idx >= 0 && len(r.Join.BySymbol) == 1 && len(r.Join.ByFile) == 0 && len(r.Join.Unmatched) == 0
		if good {
			s := r.Join.BySymbol[0]
			good = s.Symbol == nodes[idx].Key() && s.File == rel(c) && s.Line == c.Start && s.EndLine == c.End && s.Statements == c.Count && s.Covered == c.Covered && math.Abs(s.Percent-100*float64(c.Covered)/float64(c.Count)) < 1e-9 && len(s.Tests) == 0
		}
		r.Checks["FiveActualSymbolJoins"] = good
		r.UnmatchedBlocks = append([]cov.ForeignBlock(nil), r.Blocks...)
		for j := range r.UnmatchedBlocks {
			r.UnmatchedBlocks[j].File = "absent/" + c.Lang + ".missing"
		}
		r.Unmatched = cov.JoinForeignBlocks(r.UnmatchedBlocks, nodes)
		r.Checks["UnmatchedNeverGuessed"] = len(r.Unmatched.BySymbol) == 0 && len(r.Unmatched.ByFile) == 0 && len(r.Unmatched.Unmatched) == 1 && r.Unmatched.Unmatched[0] == "absent/"+c.Lang+".missing"
		altered := append([]client.SymbolNode(nil), nodes...)
		if idx >= 0 {
			altered[idx].EndLine = 0
		}
		r.Unranged = cov.JoinForeignBlocks(r.Blocks, altered)
		good = idx >= 0 && len(r.Unranged.BySymbol) == 0 && len(r.Unranged.ByFile) == 1 && len(r.Unranged.Unmatched) == 0
		if good {
			s := r.Unranged.ByFile[0]
			good = s.File == rel(c) && s.Statements == c.Count && s.Covered == c.Covered && math.Abs(s.Percent-100*float64(c.Covered)/float64(c.Count)) < 1e-9 && len(s.Tests) == 0
		}
		r.Checks["UnrangedDemotesFile"] = good
		r.OutOfRangeBlocks = append([]cov.ForeignBlock(nil), r.Blocks...)
		for j := range r.OutOfRangeBlocks {
			b := &r.OutOfRangeBlocks[j]
			width := b.EndLine - b.StartLine
			b.StartLine = maxEnd + 10
			b.EndLine = b.StartLine + width
		}
		r.OutOfRange = cov.JoinForeignBlocks(r.OutOfRangeBlocks, nodes)
		r.Checks["OutOfRangeDoesNotBecomeCoverage"] = len(r.OutOfRange.BySymbol) == 0 && len(r.OutOfRange.ByFile) == 0 && len(r.OutOfRange.Unmatched) == 0
		t.Rows = append(t.Rows, r)
	}
	must(checkInputs(m, reports, true))
	write(fmt.Sprintf("%s/trial-%d.json", root, i), enc(t))
	return t
}
func main() {
	m := manifest()
	must(checkInputs(m, nil, false))
	write(root+"/manifest.json", enc(m))
	if len(os.Args) > 1 && os.Args[1] == "check-only" {
		var reports map[string]string
		must(json.Unmarshal(read(root+"/report-hashes.json"), &reports))
		must(checkInputs(m, reports, true))
		fmt.Println("InputsFrozen PASS")
		return
	}
	reports := map[string]string{}
	for _, c := range cases {
		write(root+"/corpus/"+rel(c), read(source(c)))
		s := seed(c)
		write(root+"/seeds/"+c.Lang+".seed", []byte(s))
		data := transform(c, s)
		p := root + "/reports/" + c.Lang + ".report"
		write(p, data)
		reports[p] = hash(data)
		var golden struct {
			Symbols []struct {
				Name, Kind string
				Line       int
				EndLine    int `json:"end_line"`
			}
		}
		must(json.Unmarshal(read(source(c)+".golden.json"), &golden))
		hits := 0
		for _, n := range golden.Symbols {
			if n.Name == c.Target && n.Kind == c.Kind && n.Line == c.Start && n.EndLine == c.End {
				hits++
			}
		}
		require(hits == 1, "golden target "+c.Lang)
	}
	write(root+"/report-hashes.json", enc(reports))
	a := runTrial(1, m, reports)
	b := runTrial(2, m, reports)
	canonical := func(t trial) []byte {
		return enc(struct {
			Nodes []projection
			Rows  []row
		}{t.Nodes, t.Rows})
	}
	ca, cb := canonical(a), canonical(b)
	write(root+"/canonical-1.json", ca)
	write(root+"/canonical-2.json", cb)
	repeat := bytes.Equal(ca, cb)
	ok := repeat
	for i, t := range []trial{a, b} {
		for k, v := range t.Checks {
			fmt.Printf("trial=%d %s=%t\n", i+1, k, v)
			ok = ok && v
		}
		for _, r := range t.Rows {
			fmt.Printf("trial=%d language=%s blocks=%d joins=%d checks=%s", i+1, r.Language, len(r.Blocks), len(r.Join.BySymbol), enc(r.Checks))
			for _, v := range r.Checks {
				ok = ok && v
			}
		}
		ok = ok && len(t.Errors) == 0
	}
	fmt.Printf("Repeatable=%t canonical_sha256=%s\n", repeat, hash(ca))
	if !ok {
		fmt.Println("Synthetic structural assertions FAIL")
		os.Exit(1)
	}
	fmt.Println("Synthetic structural assertions PASS; runtime coverage and oracle accuracy unmeasured")
}
```

## Complete raw trials and independent receipts

Null/empty arrays below are retained as returned; no result rows are removed.

### trial-1.json

SHA-256: `c6a313ff6646fd56e64751a9f190a7114e6174dacf4a331495e4eccce79a7061`.

<!-- data:trial-1.json -->
```json
{
  "Report": {
    "Discovered": 5,
    "Extracted": 5,
    "Skipped": 0,
    "StatSkipped": 0,
    "Deleted": 0,
    "PeakInflightBytes": 3805,
    "ReadFailed": 0,
    "ReadFailures": null,
    "Symbols": 79,
    "PurgedLegacyAnchors": false,
    "Unclassified": 0,
    "UnsupportedLanguages": null,
    "DefaultIgnoredDirs": null,
    "SkippedGenerated": 0,
    "AmbiguousUnclassified": 0,
    "ClassifiedBySniff": 0,
    "Oversize": 0,
    "ClassifiedZero": 0,
    "TypedPackages": 0,
    "TypedPackagesFailed": 0,
    "TypedEdgesDirect": 0,
    "TypedEdgesIfaceDispatch": 0,
    "TypedEdgesUnresolved": 0,
    "TypedPackagesSeen": 0,
    "TypedFailed": false,
    "TypedFailReason": "",
    "TypedFallback": "",
    "TypedSkipped": "",
    "TypedPackagesSkipped": 0,
    "TypedSkips": null,
    "TypedNotLoaded": null,
    "CUEPackages": 0,
    "CUEPackagesFailed": 0,
    "CUEEdgesDirect": 0,
    "CUEEdgesUnresolved": 0,
    "CUESkipped": "",
    "FileResolvedEdges": 6,
    "DocXrefEdges": 0,
    "DocXrefSkipped": "",
    "RouteRefEdges": 0,
    "RouteRefUnresolved": null,
    "RouteRefSkipped": "",
    "PolicyRefEdges": 0,
    "PolicyRefUnresolved": null,
    "PolicyRefSkipped": "",
    "FileResolveReused": 0,
    "FileResolveFaults": 0,
    "FileResolveFaultFiles": null,
    "FileResolveSkipped": "",
    "TSEngineFaults": 0,
    "ModelsRefused": 0,
    "DocIndex": {
      "FilesScanned": 0,
      "FilesChanged": 0,
      "FilesUnchanged": 0,
      "FilesDeleted": 0,
      "UnitsWritten": 0,
      "UnitsElided": 0,
      "ElidedBytes": 0,
      "Elided": null,
      "Failures": null,
      "LinksWritten": 0
    },
    "DocIndexSkipped": "",
    "PassTimes": [
      {
        "Name": "discover",
        "Parent": "",
        "WallNS": 2515542,
        "CPUNS": 1900000
      },
      {
        "Name": "extract",
        "Parent": "",
        "WallNS": 3561374875,
        "CPUNS": 3625442000
      },
      {
        "Name": "lambda_gate",
        "Parent": "extract",
        "WallNS": 527875,
        "CPUNS": 528000
      },
      {
        "Name": "store_writes",
        "Parent": "extract",
        "WallNS": 18533250,
        "CPUNS": 18563000
      },
      {
        "Name": "typed_go",
        "Parent": "",
        "WallNS": 11792,
        "CPUNS": 12000
      },
      {
        "Name": "cue",
        "Parent": "",
        "WallNS": 312250,
        "CPUNS": 312000
      },
      {
        "Name": "file_resolved",
        "Parent": "",
        "WallNS": 15226583,
        "CPUNS": 15330000
      },
      {
        "Name": "resolve",
        "Parent": "file_resolved",
        "WallNS": 1628458,
        "CPUNS": 1627000
      },
      {
        "Name": "doc_xref",
        "Parent": "",
        "WallNS": 1085792,
        "CPUNS": 1101000
      },
      {
        "Name": "image_ref",
        "Parent": "doc_xref",
        "WallNS": 1500,
        "CPUNS": 1000
      },
      {
        "Name": "route_ref",
        "Parent": "",
        "WallNS": 1194875,
        "CPUNS": 1220000
      },
      {
        "Name": "policy_ref",
        "Parent": "",
        "WallNS": 1134916,
        "CPUNS": 1151000
      },
      {
        "Name": "doc_index",
        "Parent": "",
        "WallNS": 440667,
        "CPUNS": 439000
      },
      {
        "Name": "centrality",
        "Parent": "",
        "WallNS": 9683291,
        "CPUNS": 9709000
      },
      {
        "Name": "commit",
        "Parent": "",
        "WallNS": 371714667,
        "CPUNS": 285597000
      }
    ]
  },
  "Nodes": [
    {
      "File": "c/shapes.c",
      "Name": "MAX_SIZE",
      "Kind": "CMacro",
      "Key": "free-coverage::c/shapes.MAX_SIZE",
      "Line": 4,
      "EndLine": 4
    },
    {
      "File": "c/shapes.c",
      "Name": "SQUARE",
      "Kind": "CMacro",
      "Key": "free-coverage::c/shapes.SQUARE",
      "Line": 6,
      "EndLine": 6
    },
    {
      "File": "c/shapes.c",
      "Name": "Color",
      "Kind": "CEnum",
      "Key": "free-coverage::c/shapes.Color",
      "Line": 8,
      "EndLine": 12
    },
    {
      "File": "c/shapes.c",
      "Name": "COLOR_RED",
      "Kind": "CEnumerator",
      "Key": "free-coverage::c/shapes.COLOR_RED",
      "Line": 9,
      "EndLine": 9
    },
    {
      "File": "c/shapes.c",
      "Name": "COLOR_GREEN",
      "Kind": "CEnumerator",
      "Key": "free-coverage::c/shapes.COLOR_GREEN",
      "Line": 10,
      "EndLine": 10
    },
    {
      "File": "c/shapes.c",
      "Name": "COLOR_BLUE",
      "Kind": "CEnumerator",
      "Key": "free-coverage::c/shapes.COLOR_BLUE",
      "Line": 11,
      "EndLine": 11
    },
    {
      "File": "c/shapes.c",
      "Name": "Point",
      "Kind": "CStruct",
      "Key": "free-coverage::c/shapes.Point",
      "Line": 14,
      "EndLine": 17
    },
    {
      "File": "c/shapes.c",
      "Name": "Point.x",
      "Kind": "CField",
      "Key": "free-coverage::c/shapes.Point.x",
      "Line": 15,
      "EndLine": 15
    },
    {
      "File": "c/shapes.c",
      "Name": "Point.y",
      "Kind": "CField",
      "Key": "free-coverage::c/shapes.Point.y",
      "Line": 16,
      "EndLine": 16
    },
    {
      "File": "c/shapes.c",
      "Name": "Value",
      "Kind": "CUnion",
      "Key": "free-coverage::c/shapes.Value",
      "Line": 19,
      "EndLine": 22
    },
    {
      "File": "c/shapes.c",
      "Name": "Value.as_int",
      "Kind": "CField",
      "Key": "free-coverage::c/shapes.Value.as_int",
      "Line": 20,
      "EndLine": 20
    },
    {
      "File": "c/shapes.c",
      "Name": "Value.as_float",
      "Kind": "CField",
      "Key": "free-coverage::c/shapes.Value.as_float",
      "Line": 21,
      "EndLine": 21
    },
    {
      "File": "c/shapes.c",
      "Name": "PointAlias",
      "Kind": "CTypedef",
      "Key": "free-coverage::c/shapes.PointAlias",
      "Line": 24,
      "EndLine": 24
    },
    {
      "File": "c/shapes.c",
      "Name": "Callback",
      "Kind": "CTypedef",
      "Key": "free-coverage::c/shapes.Callback",
      "Line": 26,
      "EndLine": 26
    },
    {
      "File": "c/shapes.c",
      "Name": "Handler",
      "Kind": "CStruct",
      "Key": "free-coverage::c/shapes.Handler",
      "Line": 28,
      "EndLine": 30
    },
    {
      "File": "c/shapes.c",
      "Name": "Handler.run",
      "Kind": "CField",
      "Key": "free-coverage::c/shapes.Handler.run",
      "Line": 29,
      "EndLine": 29
    },
    {
      "File": "c/shapes.c",
      "Name": "helper",
      "Kind": "CFunc",
      "Key": "free-coverage::c/shapes.helper",
      "Line": 32,
      "EndLine": 34
    },
    {
      "File": "c/shapes.c",
      "Name": "add",
      "Kind": "CFunc",
      "Key": "free-coverage::c/shapes.add",
      "Line": 36,
      "EndLine": 41
    },
    {
      "File": "c/shapes.c",
      "Name": "make_point",
      "Kind": "CFunc",
      "Key": "free-coverage::c/shapes.make_point",
      "Line": 43,
      "EndLine": 47
    },
    {
      "File": "c/shapes.c",
      "Name": "dispatch",
      "Kind": "CFunc",
      "Key": "free-coverage::c/shapes.dispatch",
      "Line": 49,
      "EndLine": 51
    },
    {
      "File": "java/shapes.java",
      "Name": "com.example.util.Helper",
      "Kind": "JavaImport",
      "Key": "free-coverage::java/com.example.shapes.com.example.util.Helper",
      "Line": 3,
      "EndLine": 3
    },
    {
      "File": "java/shapes.java",
      "Name": "Widget",
      "Kind": "JavaClass",
      "Key": "free-coverage::java/com.example.shapes.Widget",
      "Line": 5,
      "EndLine": 26
    },
    {
      "File": "java/shapes.java",
      "Name": "count",
      "Kind": "JavaField",
      "Key": "free-coverage::java/com.example.shapes.count",
      "Line": 6,
      "EndLine": 6
    },
    {
      "File": "java/shapes.java",
      "Name": "label",
      "Kind": "JavaField",
      "Key": "free-coverage::java/com.example.shapes.label",
      "Line": 7,
      "EndLine": 7
    },
    {
      "File": "java/shapes.java",
      "Name": "name",
      "Kind": "JavaField",
      "Key": "free-coverage::java/com.example.shapes.name",
      "Line": 7,
      "EndLine": 7
    },
    {
      "File": "java/shapes.java",
      "Name": "Widget.Widget",
      "Kind": "JavaMethod",
      "Key": "free-coverage::java/com.example.shapes.Widget.Widget",
      "Line": 9,
      "EndLine": 11
    },
    {
      "File": "java/shapes.java",
      "Name": "Widget.sum",
      "Kind": "JavaMethod",
      "Key": "free-coverage::java/com.example.shapes.Widget.sum",
      "Line": 13,
      "EndLine": 21
    },
    {
      "File": "java/shapes.java",
      "Name": "Widget.Inner",
      "Kind": "JavaClass",
      "Key": "free-coverage::java/com.example.shapes.Widget.Inner",
      "Line": 23,
      "EndLine": 25
    },
    {
      "File": "java/shapes.java",
      "Name": "Widget.Inner.innerMethod",
      "Kind": "JavaMethod",
      "Key": "free-coverage::java/com.example.shapes.Widget.Inner.innerMethod",
      "Line": 24,
      "EndLine": 24
    },
    {
      "File": "java/shapes.java",
      "Name": "Shape",
      "Kind": "JavaInterface",
      "Key": "free-coverage::java/com.example.shapes.Shape",
      "Line": 28,
      "EndLine": 30
    },
    {
      "File": "java/shapes.java",
      "Name": "Shape.area",
      "Kind": "JavaMethod",
      "Key": "free-coverage::java/com.example.shapes.Shape.area",
      "Line": 29,
      "EndLine": 29
    },
    {
      "File": "java/shapes.java",
      "Name": "Color",
      "Kind": "JavaEnum",
      "Key": "free-coverage::java/com.example.shapes.Color",
      "Line": 32,
      "EndLine": 34
    },
    {
      "File": "java/shapes.java",
      "Name": "Point",
      "Kind": "JavaRecord",
      "Key": "free-coverage::java/com.example.shapes.Point",
      "Line": 36,
      "EndLine": 38
    },
    {
      "File": "java/shapes.java",
      "Name": "Point.sum",
      "Kind": "JavaMethod",
      "Key": "free-coverage::java/com.example.shapes.Point.sum",
      "Line": 37,
      "EndLine": 37
    },
    {
      "File": "java/shapes.java",
      "Name": "MyAnno",
      "Kind": "JavaAnnotation",
      "Key": "free-coverage::java/com.example.shapes.MyAnno",
      "Line": 40,
      "EndLine": 42
    },
    {
      "File": "javascript/bindings.js",
      "Name": "\u003cmodule\u003e",
      "Kind": "JSModule",
      "Key": "free-coverage::javascript/bindings.\u003cmodule\u003e",
      "Line": 1,
      "EndLine": 29
    },
    {
      "File": "javascript/bindings.js",
      "Name": "express",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.express",
      "Line": 4,
      "EndLine": 4
    },
    {
      "File": "javascript/bindings.js",
      "Name": "app",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.app",
      "Line": 5,
      "EndLine": 5
    },
    {
      "File": "javascript/bindings.js",
      "Name": "count",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.count",
      "Line": 6,
      "EndLine": 6
    },
    {
      "File": "javascript/bindings.js",
      "Name": "a",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.a",
      "Line": 7,
      "EndLine": 7
    },
    {
      "File": "javascript/bindings.js",
      "Name": "b",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.b",
      "Line": 7,
      "EndLine": 7
    },
    {
      "File": "javascript/bindings.js",
      "Name": "join",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.join",
      "Line": 8,
      "EndLine": 8
    },
    {
      "File": "javascript/bindings.js",
      "Name": "res",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.res",
      "Line": 8,
      "EndLine": 8
    },
    {
      "File": "javascript/bindings.js",
      "Name": "first",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.first",
      "Line": 9,
      "EndLine": 9
    },
    {
      "File": "javascript/bindings.js",
      "Name": "second",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.second",
      "Line": 9,
      "EndLine": 9
    },
    {
      "File": "javascript/bindings.js",
      "Name": "pending",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.pending",
      "Line": 10,
      "EndLine": 10
    },
    {
      "File": "javascript/bindings.js",
      "Name": "handler",
      "Kind": "JSFunc",
      "Key": "free-coverage::javascript/bindings.handler",
      "Line": 11,
      "EndLine": 13
    },
    {
      "File": "javascript/bindings.js",
      "Name": "version",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.version",
      "Line": 14,
      "EndLine": 14
    },
    {
      "File": "javascript/bindings.js",
      "Name": "router",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.router",
      "Line": 15,
      "EndLine": 15
    },
    {
      "File": "javascript/bindings.js",
      "Name": "config",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.config",
      "Line": 16,
      "EndLine": 16
    },
    {
      "File": "javascript/bindings.js",
      "Name": "create",
      "Kind": "JSFunc",
      "Key": "free-coverage::javascript/bindings.create",
      "Line": 17,
      "EndLine": 21
    },
    {
      "File": "javascript/bindings.js",
      "Name": "wrapped",
      "Kind": "JSFunc",
      "Key": "free-coverage::javascript/bindings.wrapped",
      "Line": 22,
      "EndLine": 22
    },
    {
      "File": "python/shapes.py",
      "Name": "\u003cmodule\u003e",
      "Kind": "PythonModule",
      "Key": "free-coverage::python/shapes.\u003cmodule\u003e",
      "Line": 1,
      "EndLine": 64
    },
    {
      "File": "python/shapes.py",
      "Name": "LIMIT",
      "Kind": "PythonVar",
      "Key": "free-coverage::python/shapes.LIMIT",
      "Line": 8,
      "EndLine": 8
    },
    {
      "File": "python/shapes.py",
      "Name": "name",
      "Kind": "PythonVar",
      "Key": "free-coverage::python/shapes.name",
      "Line": 9,
      "EndLine": 9
    },
    {
      "File": "python/shapes.py",
      "Name": "other_name",
      "Kind": "PythonVar",
      "Key": "free-coverage::python/shapes.other_name",
      "Line": 9,
      "EndLine": 9
    },
    {
      "File": "python/shapes.py",
      "Name": "counter",
      "Kind": "PythonVar",
      "Key": "free-coverage::python/shapes.counter",
      "Line": 10,
      "EndLine": 10
    },
    {
      "File": "python/shapes.py",
      "Name": "top",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.top",
      "Line": 13,
      "EndLine": 17
    },
    {
      "File": "python/shapes.py",
      "Name": "fetch",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.fetch",
      "Line": 20,
      "EndLine": 21
    },
    {
      "File": "python/shapes.py",
      "Name": "decorated",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.decorated",
      "Line": 25,
      "EndLine": 29
    },
    {
      "File": "python/shapes.py",
      "Name": "Base",
      "Kind": "PythonClass",
      "Key": "free-coverage::python/shapes.Base",
      "Line": 32,
      "EndLine": 51
    },
    {
      "File": "python/shapes.py",
      "Name": "Base.attr",
      "Kind": "PythonVar",
      "Key": "free-coverage::python/shapes.Base.attr",
      "Line": 33,
      "EndLine": 33
    },
    {
      "File": "python/shapes.py",
      "Name": "Base.__init__",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Base.__init__",
      "Line": 35,
      "EndLine": 37
    },
    {
      "File": "python/shapes.py",
      "Name": "Base.value",
      "Kind": "PythonField",
      "Key": "free-coverage::python/shapes.Base.value",
      "Line": 36,
      "EndLine": 36
    },
    {
      "File": "python/shapes.py",
      "Name": "Base.size",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Base.size",
      "Line": 40,
      "EndLine": 41
    },
    {
      "File": "python/shapes.py",
      "Name": "Base.build",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Base.build",
      "Line": 44,
      "EndLine": 45
    },
    {
      "File": "python/shapes.py",
      "Name": "Inner",
      "Kind": "PythonClass",
      "Key": "free-coverage::python/shapes.Inner",
      "Line": 47,
      "EndLine": 51
    },
    {
      "File": "python/shapes.py",
      "Name": "Inner.deep",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Inner.deep",
      "Line": 48,
      "EndLine": 51
    },
    {
      "File": "python/shapes.py",
      "Name": "local",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.local",
      "Line": 49,
      "EndLine": 50
    },
    {
      "File": "python/shapes.py",
      "Name": "Child",
      "Kind": "PythonClass",
      "Key": "free-coverage::python/shapes.Child",
      "Line": 54,
      "EndLine": 57
    },
    {
      "File": "python/shapes.py",
      "Name": "Child.method",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Child.method",
      "Line": 55,
      "EndLine": 57
    },
    {
      "File": "python/shapes.py",
      "Name": "outer",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.outer",
      "Line": 60,
      "EndLine": 64
    },
    {
      "File": "python/shapes.py",
      "Name": "Local",
      "Kind": "PythonClass",
      "Key": "free-coverage::python/shapes.Local",
      "Line": 61,
      "EndLine": 63
    },
    {
      "File": "python/shapes.py",
      "Name": "Local.m",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Local.m",
      "Line": 62,
      "EndLine": 63
    },
    {
      "File": "ruby/calls.rb",
      "Name": "Pipeline",
      "Kind": "RubyClass",
      "Key": "free-coverage::ruby/calls.Pipeline",
      "Line": 3,
      "EndLine": 29
    },
    {
      "File": "ruby/calls.rb",
      "Name": "Pipeline::STAGES",
      "Kind": "RubyConstant",
      "Key": "free-coverage::ruby/calls.Pipeline::STAGES",
      "Line": 4,
      "EndLine": 4
    },
    {
      "File": "ruby/calls.rb",
      "Name": "Pipeline.run",
      "Kind": "RubyMethod",
      "Key": "free-coverage::ruby/calls.Pipeline.run",
      "Line": 6,
      "EndLine": 16
    },
    {
      "File": "ruby/calls.rb",
      "Name": "Pipeline.dispatch",
      "Kind": "RubyMethod",
      "Key": "free-coverage::ruby/calls.Pipeline.dispatch",
      "Line": 18,
      "EndLine": 22
    },
    {
      "File": "ruby/calls.rb",
      "Name": "Pipeline.handlers",
      "Kind": "RubyMethod",
      "Key": "free-coverage::ruby/calls.Pipeline.handlers",
      "Line": 26,
      "EndLine": 28
    }
  ],
  "Rows": [
    {
      "Language": "python",
      "Blocks": [
        {
          "File": "python/shapes.py",
          "StartLine": 13,
          "EndLine": 13,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "python/shapes.py",
          "StartLine": 14,
          "EndLine": 14,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "python/shapes.py",
          "StartLine": 15,
          "EndLine": 15,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Join": {
        "BySymbol": [
          {
            "Symbol": "free-coverage::python/shapes.top",
            "File": "python/shapes.py",
            "Line": 13,
            "EndLine": 17,
            "Statements": 3,
            "Covered": 2,
            "Percent": 66.66666666666666,
            "Tests": null
          }
        ],
        "ByFile": null,
        "Unmatched": null
      },
      "UnmatchedBlocks": [
        {
          "File": "absent/python.missing",
          "StartLine": 13,
          "EndLine": 13,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "absent/python.missing",
          "StartLine": 14,
          "EndLine": 14,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "absent/python.missing",
          "StartLine": 15,
          "EndLine": 15,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "OutOfRangeBlocks": [
        {
          "File": "python/shapes.py",
          "StartLine": 74,
          "EndLine": 74,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "python/shapes.py",
          "StartLine": 74,
          "EndLine": 74,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "python/shapes.py",
          "StartLine": 74,
          "EndLine": 74,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Unmatched": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": [
          "absent/python.missing"
        ]
      },
      "Unranged": {
        "BySymbol": null,
        "ByFile": [
          {
            "File": "python/shapes.py",
            "Statements": 3,
            "Covered": 2,
            "Percent": 66.66666666666666,
            "Tests": null
          }
        ],
        "Unmatched": null
      },
      "OutOfRange": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": null
      },
      "Checks": {
        "FiveActualSymbolJoins": true,
        "FormatsParsed": true,
        "OutOfRangeDoesNotBecomeCoverage": true,
        "SourcesReallyIngested": true,
        "UnmatchedNeverGuessed": true,
        "UnrangedDemotesFile": true
      }
    },
    {
      "Language": "javascript",
      "Blocks": [
        {
          "File": "javascript/bindings.js",
          "StartLine": 17,
          "EndLine": 17,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "javascript/bindings.js",
          "StartLine": 19,
          "EndLine": 21,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "Join": {
        "BySymbol": [
          {
            "Symbol": "free-coverage::javascript/bindings.create",
            "File": "javascript/bindings.js",
            "Line": 17,
            "EndLine": 21,
            "Statements": 2,
            "Covered": 1,
            "Percent": 50,
            "Tests": null
          }
        ],
        "ByFile": null,
        "Unmatched": null
      },
      "UnmatchedBlocks": [
        {
          "File": "absent/javascript.missing",
          "StartLine": 17,
          "EndLine": 17,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "absent/javascript.missing",
          "StartLine": 19,
          "EndLine": 21,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "OutOfRangeBlocks": [
        {
          "File": "javascript/bindings.js",
          "StartLine": 32,
          "EndLine": 32,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "javascript/bindings.js",
          "StartLine": 32,
          "EndLine": 34,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "Unmatched": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": [
          "absent/javascript.missing"
        ]
      },
      "Unranged": {
        "BySymbol": null,
        "ByFile": [
          {
            "File": "javascript/bindings.js",
            "Statements": 2,
            "Covered": 1,
            "Percent": 50,
            "Tests": null
          }
        ],
        "Unmatched": null
      },
      "OutOfRange": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": null
      },
      "Checks": {
        "FiveActualSymbolJoins": true,
        "FormatsParsed": true,
        "OutOfRangeDoesNotBecomeCoverage": true,
        "SourcesReallyIngested": true,
        "UnmatchedNeverGuessed": true,
        "UnrangedDemotesFile": true
      }
    },
    {
      "Language": "java",
      "Blocks": [
        {
          "File": "java/shapes.java",
          "StartLine": 13,
          "EndLine": 13,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "java/shapes.java",
          "StartLine": 14,
          "EndLine": 14,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "Join": {
        "BySymbol": [
          {
            "Symbol": "free-coverage::java/com.example.shapes.Widget.sum",
            "File": "java/shapes.java",
            "Line": 13,
            "EndLine": 21,
            "Statements": 2,
            "Covered": 1,
            "Percent": 50,
            "Tests": null
          }
        ],
        "ByFile": null,
        "Unmatched": null
      },
      "UnmatchedBlocks": [
        {
          "File": "absent/java.missing",
          "StartLine": 13,
          "EndLine": 13,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "absent/java.missing",
          "StartLine": 14,
          "EndLine": 14,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "OutOfRangeBlocks": [
        {
          "File": "java/shapes.java",
          "StartLine": 47,
          "EndLine": 47,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "java/shapes.java",
          "StartLine": 47,
          "EndLine": 47,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "Unmatched": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": [
          "absent/java.missing"
        ]
      },
      "Unranged": {
        "BySymbol": null,
        "ByFile": [
          {
            "File": "java/shapes.java",
            "Statements": 2,
            "Covered": 1,
            "Percent": 50,
            "Tests": null
          }
        ],
        "Unmatched": null
      },
      "OutOfRange": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": null
      },
      "Checks": {
        "FiveActualSymbolJoins": true,
        "FormatsParsed": true,
        "OutOfRangeDoesNotBecomeCoverage": true,
        "SourcesReallyIngested": true,
        "UnmatchedNeverGuessed": true,
        "UnrangedDemotesFile": true
      }
    },
    {
      "Language": "ruby",
      "Blocks": [
        {
          "File": "ruby/calls.rb",
          "StartLine": 6,
          "EndLine": 6,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "ruby/calls.rb",
          "StartLine": 7,
          "EndLine": 7,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "ruby/calls.rb",
          "StartLine": 10,
          "EndLine": 10,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Join": {
        "BySymbol": [
          {
            "Symbol": "free-coverage::ruby/calls.Pipeline.run",
            "File": "ruby/calls.rb",
            "Line": 6,
            "EndLine": 16,
            "Statements": 3,
            "Covered": 2,
            "Percent": 66.66666666666666,
            "Tests": null
          }
        ],
        "ByFile": null,
        "Unmatched": null
      },
      "UnmatchedBlocks": [
        {
          "File": "absent/ruby.missing",
          "StartLine": 6,
          "EndLine": 6,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "absent/ruby.missing",
          "StartLine": 7,
          "EndLine": 7,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "absent/ruby.missing",
          "StartLine": 10,
          "EndLine": 10,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "OutOfRangeBlocks": [
        {
          "File": "ruby/calls.rb",
          "StartLine": 38,
          "EndLine": 38,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "ruby/calls.rb",
          "StartLine": 38,
          "EndLine": 38,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "ruby/calls.rb",
          "StartLine": 38,
          "EndLine": 38,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Unmatched": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": [
          "absent/ruby.missing"
        ]
      },
      "Unranged": {
        "BySymbol": null,
        "ByFile": [
          {
            "File": "ruby/calls.rb",
            "Statements": 3,
            "Covered": 2,
            "Percent": 66.66666666666666,
            "Tests": null
          }
        ],
        "Unmatched": null
      },
      "OutOfRange": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": null
      },
      "Checks": {
        "FiveActualSymbolJoins": true,
        "FormatsParsed": true,
        "OutOfRangeDoesNotBecomeCoverage": true,
        "SourcesReallyIngested": true,
        "UnmatchedNeverGuessed": true,
        "UnrangedDemotesFile": true
      }
    },
    {
      "Language": "c",
      "Blocks": [
        {
          "File": "c/shapes.c",
          "StartLine": 36,
          "EndLine": 36,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "c/shapes.c",
          "StartLine": 37,
          "EndLine": 37,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Join": {
        "BySymbol": [
          {
            "Symbol": "free-coverage::c/shapes.add",
            "File": "c/shapes.c",
            "Line": 36,
            "EndLine": 41,
            "Statements": 2,
            "Covered": 2,
            "Percent": 100,
            "Tests": null
          }
        ],
        "ByFile": null,
        "Unmatched": null
      },
      "UnmatchedBlocks": [
        {
          "File": "absent/c.missing",
          "StartLine": 36,
          "EndLine": 36,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "absent/c.missing",
          "StartLine": 37,
          "EndLine": 37,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "OutOfRangeBlocks": [
        {
          "File": "c/shapes.c",
          "StartLine": 61,
          "EndLine": 61,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "c/shapes.c",
          "StartLine": 61,
          "EndLine": 61,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Unmatched": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": [
          "absent/c.missing"
        ]
      },
      "Unranged": {
        "BySymbol": null,
        "ByFile": [
          {
            "File": "c/shapes.c",
            "Statements": 2,
            "Covered": 2,
            "Percent": 100,
            "Tests": null
          }
        ],
        "Unmatched": null
      },
      "OutOfRange": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": null
      },
      "Checks": {
        "FiveActualSymbolJoins": true,
        "FormatsParsed": true,
        "OutOfRangeDoesNotBecomeCoverage": true,
        "SourcesReallyIngested": true,
        "UnmatchedNeverGuessed": true,
        "UnrangedDemotesFile": true
      }
    }
  ],
  "Checks": {
    "FiveSourcePaths": true,
    "IngestReport": true,
    "InputsFrozen": true
  },
  "Errors": null
}
```

### trial-2.json

SHA-256: `7c387be39827d49073d0a88cb6ef29e8c4a3039a9e47d2af1d030e9defebf5bd`.

<!-- data:trial-2.json -->
```json
{
  "Report": {
    "Discovered": 5,
    "Extracted": 5,
    "Skipped": 0,
    "StatSkipped": 0,
    "Deleted": 0,
    "PeakInflightBytes": 3805,
    "ReadFailed": 0,
    "ReadFailures": null,
    "Symbols": 79,
    "PurgedLegacyAnchors": false,
    "Unclassified": 0,
    "UnsupportedLanguages": null,
    "DefaultIgnoredDirs": null,
    "SkippedGenerated": 0,
    "AmbiguousUnclassified": 0,
    "ClassifiedBySniff": 0,
    "Oversize": 0,
    "ClassifiedZero": 0,
    "TypedPackages": 0,
    "TypedPackagesFailed": 0,
    "TypedEdgesDirect": 0,
    "TypedEdgesIfaceDispatch": 0,
    "TypedEdgesUnresolved": 0,
    "TypedPackagesSeen": 0,
    "TypedFailed": false,
    "TypedFailReason": "",
    "TypedFallback": "",
    "TypedSkipped": "",
    "TypedPackagesSkipped": 0,
    "TypedSkips": null,
    "TypedNotLoaded": null,
    "CUEPackages": 0,
    "CUEPackagesFailed": 0,
    "CUEEdgesDirect": 0,
    "CUEEdgesUnresolved": 0,
    "CUESkipped": "",
    "FileResolvedEdges": 6,
    "DocXrefEdges": 0,
    "DocXrefSkipped": "",
    "RouteRefEdges": 0,
    "RouteRefUnresolved": null,
    "RouteRefSkipped": "",
    "PolicyRefEdges": 0,
    "PolicyRefUnresolved": null,
    "PolicyRefSkipped": "",
    "FileResolveReused": 0,
    "FileResolveFaults": 0,
    "FileResolveFaultFiles": null,
    "FileResolveSkipped": "",
    "TSEngineFaults": 0,
    "ModelsRefused": 0,
    "DocIndex": {
      "FilesScanned": 0,
      "FilesChanged": 0,
      "FilesUnchanged": 0,
      "FilesDeleted": 0,
      "UnitsWritten": 0,
      "UnitsElided": 0,
      "ElidedBytes": 0,
      "Elided": null,
      "Failures": null,
      "LinksWritten": 0
    },
    "DocIndexSkipped": "",
    "PassTimes": [
      {
        "Name": "discover",
        "Parent": "",
        "WallNS": 1800708,
        "CPUNS": 1801000
      },
      {
        "Name": "extract",
        "Parent": "",
        "WallNS": 531445708,
        "CPUNS": 558973000
      },
      {
        "Name": "lambda_gate",
        "Parent": "extract",
        "WallNS": 540667,
        "CPUNS": 533000
      },
      {
        "Name": "store_writes",
        "Parent": "extract",
        "WallNS": 18028167,
        "CPUNS": 18046000
      },
      {
        "Name": "typed_go",
        "Parent": "",
        "WallNS": 3125,
        "CPUNS": 3000
      },
      {
        "Name": "cue",
        "Parent": "",
        "WallNS": 284209,
        "CPUNS": 284000
      },
      {
        "Name": "file_resolved",
        "Parent": "",
        "WallNS": 14787125,
        "CPUNS": 14908000
      },
      {
        "Name": "resolve",
        "Parent": "file_resolved",
        "WallNS": 1299959,
        "CPUNS": 1302000
      },
      {
        "Name": "doc_xref",
        "Parent": "",
        "WallNS": 1033916,
        "CPUNS": 1053000
      },
      {
        "Name": "image_ref",
        "Parent": "doc_xref",
        "WallNS": 1542,
        "CPUNS": 2000
      },
      {
        "Name": "route_ref",
        "Parent": "",
        "WallNS": 1147209,
        "CPUNS": 1169000
      },
      {
        "Name": "policy_ref",
        "Parent": "",
        "WallNS": 1124250,
        "CPUNS": 1150000
      },
      {
        "Name": "doc_index",
        "Parent": "",
        "WallNS": 429083,
        "CPUNS": 429000
      },
      {
        "Name": "centrality",
        "Parent": "",
        "WallNS": 9675625,
        "CPUNS": 9705000
      },
      {
        "Name": "commit",
        "Parent": "",
        "WallNS": 308488000,
        "CPUNS": 285172000
      }
    ]
  },
  "Nodes": [
    {
      "File": "c/shapes.c",
      "Name": "MAX_SIZE",
      "Kind": "CMacro",
      "Key": "free-coverage::c/shapes.MAX_SIZE",
      "Line": 4,
      "EndLine": 4
    },
    {
      "File": "c/shapes.c",
      "Name": "SQUARE",
      "Kind": "CMacro",
      "Key": "free-coverage::c/shapes.SQUARE",
      "Line": 6,
      "EndLine": 6
    },
    {
      "File": "c/shapes.c",
      "Name": "Color",
      "Kind": "CEnum",
      "Key": "free-coverage::c/shapes.Color",
      "Line": 8,
      "EndLine": 12
    },
    {
      "File": "c/shapes.c",
      "Name": "COLOR_RED",
      "Kind": "CEnumerator",
      "Key": "free-coverage::c/shapes.COLOR_RED",
      "Line": 9,
      "EndLine": 9
    },
    {
      "File": "c/shapes.c",
      "Name": "COLOR_GREEN",
      "Kind": "CEnumerator",
      "Key": "free-coverage::c/shapes.COLOR_GREEN",
      "Line": 10,
      "EndLine": 10
    },
    {
      "File": "c/shapes.c",
      "Name": "COLOR_BLUE",
      "Kind": "CEnumerator",
      "Key": "free-coverage::c/shapes.COLOR_BLUE",
      "Line": 11,
      "EndLine": 11
    },
    {
      "File": "c/shapes.c",
      "Name": "Point",
      "Kind": "CStruct",
      "Key": "free-coverage::c/shapes.Point",
      "Line": 14,
      "EndLine": 17
    },
    {
      "File": "c/shapes.c",
      "Name": "Point.x",
      "Kind": "CField",
      "Key": "free-coverage::c/shapes.Point.x",
      "Line": 15,
      "EndLine": 15
    },
    {
      "File": "c/shapes.c",
      "Name": "Point.y",
      "Kind": "CField",
      "Key": "free-coverage::c/shapes.Point.y",
      "Line": 16,
      "EndLine": 16
    },
    {
      "File": "c/shapes.c",
      "Name": "Value",
      "Kind": "CUnion",
      "Key": "free-coverage::c/shapes.Value",
      "Line": 19,
      "EndLine": 22
    },
    {
      "File": "c/shapes.c",
      "Name": "Value.as_int",
      "Kind": "CField",
      "Key": "free-coverage::c/shapes.Value.as_int",
      "Line": 20,
      "EndLine": 20
    },
    {
      "File": "c/shapes.c",
      "Name": "Value.as_float",
      "Kind": "CField",
      "Key": "free-coverage::c/shapes.Value.as_float",
      "Line": 21,
      "EndLine": 21
    },
    {
      "File": "c/shapes.c",
      "Name": "PointAlias",
      "Kind": "CTypedef",
      "Key": "free-coverage::c/shapes.PointAlias",
      "Line": 24,
      "EndLine": 24
    },
    {
      "File": "c/shapes.c",
      "Name": "Callback",
      "Kind": "CTypedef",
      "Key": "free-coverage::c/shapes.Callback",
      "Line": 26,
      "EndLine": 26
    },
    {
      "File": "c/shapes.c",
      "Name": "Handler",
      "Kind": "CStruct",
      "Key": "free-coverage::c/shapes.Handler",
      "Line": 28,
      "EndLine": 30
    },
    {
      "File": "c/shapes.c",
      "Name": "Handler.run",
      "Kind": "CField",
      "Key": "free-coverage::c/shapes.Handler.run",
      "Line": 29,
      "EndLine": 29
    },
    {
      "File": "c/shapes.c",
      "Name": "helper",
      "Kind": "CFunc",
      "Key": "free-coverage::c/shapes.helper",
      "Line": 32,
      "EndLine": 34
    },
    {
      "File": "c/shapes.c",
      "Name": "add",
      "Kind": "CFunc",
      "Key": "free-coverage::c/shapes.add",
      "Line": 36,
      "EndLine": 41
    },
    {
      "File": "c/shapes.c",
      "Name": "make_point",
      "Kind": "CFunc",
      "Key": "free-coverage::c/shapes.make_point",
      "Line": 43,
      "EndLine": 47
    },
    {
      "File": "c/shapes.c",
      "Name": "dispatch",
      "Kind": "CFunc",
      "Key": "free-coverage::c/shapes.dispatch",
      "Line": 49,
      "EndLine": 51
    },
    {
      "File": "java/shapes.java",
      "Name": "com.example.util.Helper",
      "Kind": "JavaImport",
      "Key": "free-coverage::java/com.example.shapes.com.example.util.Helper",
      "Line": 3,
      "EndLine": 3
    },
    {
      "File": "java/shapes.java",
      "Name": "Widget",
      "Kind": "JavaClass",
      "Key": "free-coverage::java/com.example.shapes.Widget",
      "Line": 5,
      "EndLine": 26
    },
    {
      "File": "java/shapes.java",
      "Name": "count",
      "Kind": "JavaField",
      "Key": "free-coverage::java/com.example.shapes.count",
      "Line": 6,
      "EndLine": 6
    },
    {
      "File": "java/shapes.java",
      "Name": "label",
      "Kind": "JavaField",
      "Key": "free-coverage::java/com.example.shapes.label",
      "Line": 7,
      "EndLine": 7
    },
    {
      "File": "java/shapes.java",
      "Name": "name",
      "Kind": "JavaField",
      "Key": "free-coverage::java/com.example.shapes.name",
      "Line": 7,
      "EndLine": 7
    },
    {
      "File": "java/shapes.java",
      "Name": "Widget.Widget",
      "Kind": "JavaMethod",
      "Key": "free-coverage::java/com.example.shapes.Widget.Widget",
      "Line": 9,
      "EndLine": 11
    },
    {
      "File": "java/shapes.java",
      "Name": "Widget.sum",
      "Kind": "JavaMethod",
      "Key": "free-coverage::java/com.example.shapes.Widget.sum",
      "Line": 13,
      "EndLine": 21
    },
    {
      "File": "java/shapes.java",
      "Name": "Widget.Inner",
      "Kind": "JavaClass",
      "Key": "free-coverage::java/com.example.shapes.Widget.Inner",
      "Line": 23,
      "EndLine": 25
    },
    {
      "File": "java/shapes.java",
      "Name": "Widget.Inner.innerMethod",
      "Kind": "JavaMethod",
      "Key": "free-coverage::java/com.example.shapes.Widget.Inner.innerMethod",
      "Line": 24,
      "EndLine": 24
    },
    {
      "File": "java/shapes.java",
      "Name": "Shape",
      "Kind": "JavaInterface",
      "Key": "free-coverage::java/com.example.shapes.Shape",
      "Line": 28,
      "EndLine": 30
    },
    {
      "File": "java/shapes.java",
      "Name": "Shape.area",
      "Kind": "JavaMethod",
      "Key": "free-coverage::java/com.example.shapes.Shape.area",
      "Line": 29,
      "EndLine": 29
    },
    {
      "File": "java/shapes.java",
      "Name": "Color",
      "Kind": "JavaEnum",
      "Key": "free-coverage::java/com.example.shapes.Color",
      "Line": 32,
      "EndLine": 34
    },
    {
      "File": "java/shapes.java",
      "Name": "Point",
      "Kind": "JavaRecord",
      "Key": "free-coverage::java/com.example.shapes.Point",
      "Line": 36,
      "EndLine": 38
    },
    {
      "File": "java/shapes.java",
      "Name": "Point.sum",
      "Kind": "JavaMethod",
      "Key": "free-coverage::java/com.example.shapes.Point.sum",
      "Line": 37,
      "EndLine": 37
    },
    {
      "File": "java/shapes.java",
      "Name": "MyAnno",
      "Kind": "JavaAnnotation",
      "Key": "free-coverage::java/com.example.shapes.MyAnno",
      "Line": 40,
      "EndLine": 42
    },
    {
      "File": "javascript/bindings.js",
      "Name": "\u003cmodule\u003e",
      "Kind": "JSModule",
      "Key": "free-coverage::javascript/bindings.\u003cmodule\u003e",
      "Line": 1,
      "EndLine": 29
    },
    {
      "File": "javascript/bindings.js",
      "Name": "express",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.express",
      "Line": 4,
      "EndLine": 4
    },
    {
      "File": "javascript/bindings.js",
      "Name": "app",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.app",
      "Line": 5,
      "EndLine": 5
    },
    {
      "File": "javascript/bindings.js",
      "Name": "count",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.count",
      "Line": 6,
      "EndLine": 6
    },
    {
      "File": "javascript/bindings.js",
      "Name": "a",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.a",
      "Line": 7,
      "EndLine": 7
    },
    {
      "File": "javascript/bindings.js",
      "Name": "b",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.b",
      "Line": 7,
      "EndLine": 7
    },
    {
      "File": "javascript/bindings.js",
      "Name": "join",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.join",
      "Line": 8,
      "EndLine": 8
    },
    {
      "File": "javascript/bindings.js",
      "Name": "res",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.res",
      "Line": 8,
      "EndLine": 8
    },
    {
      "File": "javascript/bindings.js",
      "Name": "first",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.first",
      "Line": 9,
      "EndLine": 9
    },
    {
      "File": "javascript/bindings.js",
      "Name": "second",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.second",
      "Line": 9,
      "EndLine": 9
    },
    {
      "File": "javascript/bindings.js",
      "Name": "pending",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.pending",
      "Line": 10,
      "EndLine": 10
    },
    {
      "File": "javascript/bindings.js",
      "Name": "handler",
      "Kind": "JSFunc",
      "Key": "free-coverage::javascript/bindings.handler",
      "Line": 11,
      "EndLine": 13
    },
    {
      "File": "javascript/bindings.js",
      "Name": "version",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.version",
      "Line": 14,
      "EndLine": 14
    },
    {
      "File": "javascript/bindings.js",
      "Name": "router",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.router",
      "Line": 15,
      "EndLine": 15
    },
    {
      "File": "javascript/bindings.js",
      "Name": "config",
      "Kind": "JSConst",
      "Key": "free-coverage::javascript/bindings.config",
      "Line": 16,
      "EndLine": 16
    },
    {
      "File": "javascript/bindings.js",
      "Name": "create",
      "Kind": "JSFunc",
      "Key": "free-coverage::javascript/bindings.create",
      "Line": 17,
      "EndLine": 21
    },
    {
      "File": "javascript/bindings.js",
      "Name": "wrapped",
      "Kind": "JSFunc",
      "Key": "free-coverage::javascript/bindings.wrapped",
      "Line": 22,
      "EndLine": 22
    },
    {
      "File": "python/shapes.py",
      "Name": "\u003cmodule\u003e",
      "Kind": "PythonModule",
      "Key": "free-coverage::python/shapes.\u003cmodule\u003e",
      "Line": 1,
      "EndLine": 64
    },
    {
      "File": "python/shapes.py",
      "Name": "LIMIT",
      "Kind": "PythonVar",
      "Key": "free-coverage::python/shapes.LIMIT",
      "Line": 8,
      "EndLine": 8
    },
    {
      "File": "python/shapes.py",
      "Name": "name",
      "Kind": "PythonVar",
      "Key": "free-coverage::python/shapes.name",
      "Line": 9,
      "EndLine": 9
    },
    {
      "File": "python/shapes.py",
      "Name": "other_name",
      "Kind": "PythonVar",
      "Key": "free-coverage::python/shapes.other_name",
      "Line": 9,
      "EndLine": 9
    },
    {
      "File": "python/shapes.py",
      "Name": "counter",
      "Kind": "PythonVar",
      "Key": "free-coverage::python/shapes.counter",
      "Line": 10,
      "EndLine": 10
    },
    {
      "File": "python/shapes.py",
      "Name": "top",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.top",
      "Line": 13,
      "EndLine": 17
    },
    {
      "File": "python/shapes.py",
      "Name": "fetch",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.fetch",
      "Line": 20,
      "EndLine": 21
    },
    {
      "File": "python/shapes.py",
      "Name": "decorated",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.decorated",
      "Line": 25,
      "EndLine": 29
    },
    {
      "File": "python/shapes.py",
      "Name": "Base",
      "Kind": "PythonClass",
      "Key": "free-coverage::python/shapes.Base",
      "Line": 32,
      "EndLine": 51
    },
    {
      "File": "python/shapes.py",
      "Name": "Base.attr",
      "Kind": "PythonVar",
      "Key": "free-coverage::python/shapes.Base.attr",
      "Line": 33,
      "EndLine": 33
    },
    {
      "File": "python/shapes.py",
      "Name": "Base.__init__",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Base.__init__",
      "Line": 35,
      "EndLine": 37
    },
    {
      "File": "python/shapes.py",
      "Name": "Base.value",
      "Kind": "PythonField",
      "Key": "free-coverage::python/shapes.Base.value",
      "Line": 36,
      "EndLine": 36
    },
    {
      "File": "python/shapes.py",
      "Name": "Base.size",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Base.size",
      "Line": 40,
      "EndLine": 41
    },
    {
      "File": "python/shapes.py",
      "Name": "Base.build",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Base.build",
      "Line": 44,
      "EndLine": 45
    },
    {
      "File": "python/shapes.py",
      "Name": "Inner",
      "Kind": "PythonClass",
      "Key": "free-coverage::python/shapes.Inner",
      "Line": 47,
      "EndLine": 51
    },
    {
      "File": "python/shapes.py",
      "Name": "Inner.deep",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Inner.deep",
      "Line": 48,
      "EndLine": 51
    },
    {
      "File": "python/shapes.py",
      "Name": "local",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.local",
      "Line": 49,
      "EndLine": 50
    },
    {
      "File": "python/shapes.py",
      "Name": "Child",
      "Kind": "PythonClass",
      "Key": "free-coverage::python/shapes.Child",
      "Line": 54,
      "EndLine": 57
    },
    {
      "File": "python/shapes.py",
      "Name": "Child.method",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Child.method",
      "Line": 55,
      "EndLine": 57
    },
    {
      "File": "python/shapes.py",
      "Name": "outer",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.outer",
      "Line": 60,
      "EndLine": 64
    },
    {
      "File": "python/shapes.py",
      "Name": "Local",
      "Kind": "PythonClass",
      "Key": "free-coverage::python/shapes.Local",
      "Line": 61,
      "EndLine": 63
    },
    {
      "File": "python/shapes.py",
      "Name": "Local.m",
      "Kind": "PythonFunc",
      "Key": "free-coverage::python/shapes.Local.m",
      "Line": 62,
      "EndLine": 63
    },
    {
      "File": "ruby/calls.rb",
      "Name": "Pipeline",
      "Kind": "RubyClass",
      "Key": "free-coverage::ruby/calls.Pipeline",
      "Line": 3,
      "EndLine": 29
    },
    {
      "File": "ruby/calls.rb",
      "Name": "Pipeline::STAGES",
      "Kind": "RubyConstant",
      "Key": "free-coverage::ruby/calls.Pipeline::STAGES",
      "Line": 4,
      "EndLine": 4
    },
    {
      "File": "ruby/calls.rb",
      "Name": "Pipeline.run",
      "Kind": "RubyMethod",
      "Key": "free-coverage::ruby/calls.Pipeline.run",
      "Line": 6,
      "EndLine": 16
    },
    {
      "File": "ruby/calls.rb",
      "Name": "Pipeline.dispatch",
      "Kind": "RubyMethod",
      "Key": "free-coverage::ruby/calls.Pipeline.dispatch",
      "Line": 18,
      "EndLine": 22
    },
    {
      "File": "ruby/calls.rb",
      "Name": "Pipeline.handlers",
      "Kind": "RubyMethod",
      "Key": "free-coverage::ruby/calls.Pipeline.handlers",
      "Line": 26,
      "EndLine": 28
    }
  ],
  "Rows": [
    {
      "Language": "python",
      "Blocks": [
        {
          "File": "python/shapes.py",
          "StartLine": 13,
          "EndLine": 13,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "python/shapes.py",
          "StartLine": 14,
          "EndLine": 14,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "python/shapes.py",
          "StartLine": 15,
          "EndLine": 15,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Join": {
        "BySymbol": [
          {
            "Symbol": "free-coverage::python/shapes.top",
            "File": "python/shapes.py",
            "Line": 13,
            "EndLine": 17,
            "Statements": 3,
            "Covered": 2,
            "Percent": 66.66666666666666,
            "Tests": null
          }
        ],
        "ByFile": null,
        "Unmatched": null
      },
      "UnmatchedBlocks": [
        {
          "File": "absent/python.missing",
          "StartLine": 13,
          "EndLine": 13,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "absent/python.missing",
          "StartLine": 14,
          "EndLine": 14,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "absent/python.missing",
          "StartLine": 15,
          "EndLine": 15,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "OutOfRangeBlocks": [
        {
          "File": "python/shapes.py",
          "StartLine": 74,
          "EndLine": 74,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "python/shapes.py",
          "StartLine": 74,
          "EndLine": 74,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "python/shapes.py",
          "StartLine": 74,
          "EndLine": 74,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Unmatched": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": [
          "absent/python.missing"
        ]
      },
      "Unranged": {
        "BySymbol": null,
        "ByFile": [
          {
            "File": "python/shapes.py",
            "Statements": 3,
            "Covered": 2,
            "Percent": 66.66666666666666,
            "Tests": null
          }
        ],
        "Unmatched": null
      },
      "OutOfRange": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": null
      },
      "Checks": {
        "FiveActualSymbolJoins": true,
        "FormatsParsed": true,
        "OutOfRangeDoesNotBecomeCoverage": true,
        "SourcesReallyIngested": true,
        "UnmatchedNeverGuessed": true,
        "UnrangedDemotesFile": true
      }
    },
    {
      "Language": "javascript",
      "Blocks": [
        {
          "File": "javascript/bindings.js",
          "StartLine": 17,
          "EndLine": 17,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "javascript/bindings.js",
          "StartLine": 19,
          "EndLine": 21,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "Join": {
        "BySymbol": [
          {
            "Symbol": "free-coverage::javascript/bindings.create",
            "File": "javascript/bindings.js",
            "Line": 17,
            "EndLine": 21,
            "Statements": 2,
            "Covered": 1,
            "Percent": 50,
            "Tests": null
          }
        ],
        "ByFile": null,
        "Unmatched": null
      },
      "UnmatchedBlocks": [
        {
          "File": "absent/javascript.missing",
          "StartLine": 17,
          "EndLine": 17,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "absent/javascript.missing",
          "StartLine": 19,
          "EndLine": 21,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "OutOfRangeBlocks": [
        {
          "File": "javascript/bindings.js",
          "StartLine": 32,
          "EndLine": 32,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "javascript/bindings.js",
          "StartLine": 32,
          "EndLine": 34,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "Unmatched": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": [
          "absent/javascript.missing"
        ]
      },
      "Unranged": {
        "BySymbol": null,
        "ByFile": [
          {
            "File": "javascript/bindings.js",
            "Statements": 2,
            "Covered": 1,
            "Percent": 50,
            "Tests": null
          }
        ],
        "Unmatched": null
      },
      "OutOfRange": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": null
      },
      "Checks": {
        "FiveActualSymbolJoins": true,
        "FormatsParsed": true,
        "OutOfRangeDoesNotBecomeCoverage": true,
        "SourcesReallyIngested": true,
        "UnmatchedNeverGuessed": true,
        "UnrangedDemotesFile": true
      }
    },
    {
      "Language": "java",
      "Blocks": [
        {
          "File": "java/shapes.java",
          "StartLine": 13,
          "EndLine": 13,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "java/shapes.java",
          "StartLine": 14,
          "EndLine": 14,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "Join": {
        "BySymbol": [
          {
            "Symbol": "free-coverage::java/com.example.shapes.Widget.sum",
            "File": "java/shapes.java",
            "Line": 13,
            "EndLine": 21,
            "Statements": 2,
            "Covered": 1,
            "Percent": 50,
            "Tests": null
          }
        ],
        "ByFile": null,
        "Unmatched": null
      },
      "UnmatchedBlocks": [
        {
          "File": "absent/java.missing",
          "StartLine": 13,
          "EndLine": 13,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "absent/java.missing",
          "StartLine": 14,
          "EndLine": 14,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "OutOfRangeBlocks": [
        {
          "File": "java/shapes.java",
          "StartLine": 47,
          "EndLine": 47,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "java/shapes.java",
          "StartLine": 47,
          "EndLine": 47,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        }
      ],
      "Unmatched": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": [
          "absent/java.missing"
        ]
      },
      "Unranged": {
        "BySymbol": null,
        "ByFile": [
          {
            "File": "java/shapes.java",
            "Statements": 2,
            "Covered": 1,
            "Percent": 50,
            "Tests": null
          }
        ],
        "Unmatched": null
      },
      "OutOfRange": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": null
      },
      "Checks": {
        "FiveActualSymbolJoins": true,
        "FormatsParsed": true,
        "OutOfRangeDoesNotBecomeCoverage": true,
        "SourcesReallyIngested": true,
        "UnmatchedNeverGuessed": true,
        "UnrangedDemotesFile": true
      }
    },
    {
      "Language": "ruby",
      "Blocks": [
        {
          "File": "ruby/calls.rb",
          "StartLine": 6,
          "EndLine": 6,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "ruby/calls.rb",
          "StartLine": 7,
          "EndLine": 7,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "ruby/calls.rb",
          "StartLine": 10,
          "EndLine": 10,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Join": {
        "BySymbol": [
          {
            "Symbol": "free-coverage::ruby/calls.Pipeline.run",
            "File": "ruby/calls.rb",
            "Line": 6,
            "EndLine": 16,
            "Statements": 3,
            "Covered": 2,
            "Percent": 66.66666666666666,
            "Tests": null
          }
        ],
        "ByFile": null,
        "Unmatched": null
      },
      "UnmatchedBlocks": [
        {
          "File": "absent/ruby.missing",
          "StartLine": 6,
          "EndLine": 6,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "absent/ruby.missing",
          "StartLine": 7,
          "EndLine": 7,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "absent/ruby.missing",
          "StartLine": 10,
          "EndLine": 10,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "OutOfRangeBlocks": [
        {
          "File": "ruby/calls.rb",
          "StartLine": 38,
          "EndLine": 38,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "ruby/calls.rb",
          "StartLine": 38,
          "EndLine": 38,
          "Statements": 1,
          "Covered": 0,
          "Tests": null
        },
        {
          "File": "ruby/calls.rb",
          "StartLine": 38,
          "EndLine": 38,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Unmatched": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": [
          "absent/ruby.missing"
        ]
      },
      "Unranged": {
        "BySymbol": null,
        "ByFile": [
          {
            "File": "ruby/calls.rb",
            "Statements": 3,
            "Covered": 2,
            "Percent": 66.66666666666666,
            "Tests": null
          }
        ],
        "Unmatched": null
      },
      "OutOfRange": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": null
      },
      "Checks": {
        "FiveActualSymbolJoins": true,
        "FormatsParsed": true,
        "OutOfRangeDoesNotBecomeCoverage": true,
        "SourcesReallyIngested": true,
        "UnmatchedNeverGuessed": true,
        "UnrangedDemotesFile": true
      }
    },
    {
      "Language": "c",
      "Blocks": [
        {
          "File": "c/shapes.c",
          "StartLine": 36,
          "EndLine": 36,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "c/shapes.c",
          "StartLine": 37,
          "EndLine": 37,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Join": {
        "BySymbol": [
          {
            "Symbol": "free-coverage::c/shapes.add",
            "File": "c/shapes.c",
            "Line": 36,
            "EndLine": 41,
            "Statements": 2,
            "Covered": 2,
            "Percent": 100,
            "Tests": null
          }
        ],
        "ByFile": null,
        "Unmatched": null
      },
      "UnmatchedBlocks": [
        {
          "File": "absent/c.missing",
          "StartLine": 36,
          "EndLine": 36,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "absent/c.missing",
          "StartLine": 37,
          "EndLine": 37,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "OutOfRangeBlocks": [
        {
          "File": "c/shapes.c",
          "StartLine": 61,
          "EndLine": 61,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        },
        {
          "File": "c/shapes.c",
          "StartLine": 61,
          "EndLine": 61,
          "Statements": 1,
          "Covered": 1,
          "Tests": null
        }
      ],
      "Unmatched": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": [
          "absent/c.missing"
        ]
      },
      "Unranged": {
        "BySymbol": null,
        "ByFile": [
          {
            "File": "c/shapes.c",
            "Statements": 2,
            "Covered": 2,
            "Percent": 100,
            "Tests": null
          }
        ],
        "Unmatched": null
      },
      "OutOfRange": {
        "BySymbol": null,
        "ByFile": null,
        "Unmatched": null
      },
      "Checks": {
        "FiveActualSymbolJoins": true,
        "FormatsParsed": true,
        "OutOfRangeDoesNotBecomeCoverage": true,
        "SourcesReallyIngested": true,
        "UnmatchedNeverGuessed": true,
        "UnrangedDemotesFile": true
      }
    }
  ],
  "Checks": {
    "FiveSourcePaths": true,
    "IngestReport": true,
    "InputsFrozen": true
  },
  "Errors": null
}
```

### manifest.json

SHA-256: `6c21ca5a61b430f86f02a0fecd0f19b77bc3196b9231665e3320db49b0871dc0`.

<!-- data:manifest.json -->
```json
{
  "cmd/fathom/coverage_ingest.go": "9f9415d829ce1824c786b06209be231df432e77aaa66b13a1535d004c7b745cd",
  "cmd/fathom/coverage_ingest_test.go": "aae2c71b10a0f1614c3b415a6d83e61b7dba9b99d32d6e403f34beaea6eae781",
  "cmd/fathom/review_test.go": "59d1b4f8e4fe97ad81ca9c623fd81d272707039604cafc58b379f7fb520b1252",
  "cmd/fathom/tests_coverage.go": "3efa20f074e0b91f292a226aac56db99e5451cbc778d9f507ab3a6d2901915bb",
  "cmd/fathom/tests_coverage_foreign_test.go": "e183b416022d0ccc9ae96d98095103bfc55e2448e1477983a60bd3ace86fd05c",
  "cmd/fathom/tests_coverage_test.go": "61cec388d37bbb9558566d4c26a83a0c6a3911cb5a48814fa80a09bbcc7bd689",
  "docs/design/test-intelligence.md": "13c91bd4ce4472df2813f8da4c10a701b59981555952ed49ff02439995b9d10d",
  "docs/plans/realignment.md": "2392810232371350c4319a06a28fc9d0a814e00a90e3b0a55d8f3fea3f0cda54",
  "docs/plans/test-intelligence.md": "dc841711976406fe12a40077bf573e79814eeda68b3bd4a0693a8abba16791c8",
  "internal/analysis/coverage/cobertura_jacoco_test.go": "9fc485e927793eb12f0491c98a9756acaffa78c48790424981dd4acdee5fbcd2",
  "internal/analysis/coverage/coveragepy_istanbul_test.go": "945b00c686662c024e865121b44d4518dda402dc0e72b701dc100c71de21193b",
  "internal/analysis/coverage/join_foreign.go": "50eb5134fa326913b15bf2c8e1a3c3efce82e2ae9ba4ba9405e6e45fdcca092b",
  "internal/analysis/coverage/lcov_test.go": "e7edd75d601c2a614ce2f50beb257edd3d2a3a2f8d4de8768280b8911e9849bf",
  "internal/analysis/coverage/llvmcov_test.go": "6a88bd985308fd3c049f68aa24b234f200a67723826c0a299ecd3f8e508ea606",
  "internal/analysis/coverage/profile_test.go": "a8af73df666626ebf7630dc611e09377722e6309d690c69a29c93985efc823a2",
  "internal/extractor/dialects/c/testdata/shapes.c": "589187d168b9975ee4763a439ce769c432cfd378cadf68595058dbd9e63b4656",
  "internal/extractor/dialects/c/testdata/shapes.c.golden.json": "e057a842141be1dee76bee7c23a35fa13a59605921a110af23c85701fa3c6056",
  "internal/extractor/dialects/java/testdata/shapes.java": "07f549be82864141ac5b106e1e7035e30574a1393d576946c4538c104d934ea9",
  "internal/extractor/dialects/java/testdata/shapes.java.golden.json": "513e002582ae9a2d6725920783c5ac64154593fdcade3d2405b0344117a8478b",
  "internal/extractor/dialects/javascript/testdata/bindings.js": "05732e3c4dce18143c3537fb550ca7cb7ee4bbba8db59b54540528420626026e",
  "internal/extractor/dialects/javascript/testdata/bindings.js.golden.json": "b2d156efbe8b2d6e82dfc31373b2d524aa10428a87c536fcb7619df7dfd632a7",
  "internal/extractor/dialects/python/testdata/shapes.py": "a2a6243afe55d7d89bba43418061e31ef1a1f76af730721ecfa7fcc529134308",
  "internal/extractor/dialects/python/testdata/shapes.py.golden.json": "f15ac834ee65ec34bbba64fcbf986b46512536c03be98bc72f9363add390f90d",
  "internal/extractor/dialects/ruby/testdata/calls.rb": "6282795432999e81b7d87c60dc8cc81e084aa316c8cc88070be2367e3c2943de",
  "internal/extractor/dialects/ruby/testdata/calls.rb.golden.json": "67ab82888eac9f2510f421a93b85821b491325df01d062e4353e9d7293a6dc59",
  "pkg/client/client.go": "ef58f04c1e631d3690ea658aa7ccc2242dfc4e19c19870cd24ef92ca30bd6b74"
}
```

### report-hashes.json

SHA-256: `6c086b24c27cf63a6245d5343203527c37f450f6702be4a4b3df9a6dedb989e6`.

<!-- data:report-hashes.json -->
```json
{
  "workspace/tmp/free-measurements/reports/c.report": "e3b2622ce867f96f061b84f389a5700c1693220cd7222d7641c64e24db3c4a24",
  "workspace/tmp/free-measurements/reports/java.report": "f38bc1202b3600abd2de16dde0f67a6360c683f4041ec2156055ab0069654b5d",
  "workspace/tmp/free-measurements/reports/javascript.report": "b76309faaf10811cd1039a7b61f4ba9f041204a078d20d1e26b8f112601db2a8",
  "workspace/tmp/free-measurements/reports/python.report": "881aa2b7d535b2a75802ddb908056c36b17ac1be53db39652e8c01968576d2ef",
  "workspace/tmp/free-measurements/reports/ruby.report": "5671796a8a781fde96e5dd5aebaffbfcd5fcb215343386f378614a9cf147ac98"
}
```

### refusal-receipts.json

SHA-256: `91f873e10e8f7748c4bfbd676fd5ae2edc66c2254b8093d093ac9440f90d6ba5`.

<!-- data:refusal-receipts.json -->
```json
[
  {
    "case": "source-drift",
    "exit": 1,
    "expected_refusal": true,
    "pass": true
  },
  {
    "case": "report-drift",
    "exit": 1,
    "expected_refusal": true,
    "pass": true
  },
  {
    "case": "missing-report",
    "exit": 1,
    "expected_refusal": true,
    "pass": true
  },
  {
    "case": "unreadable-report",
    "exit": 1,
    "expected_refusal": true,
    "pass": true
  },
  {
    "case": "restored-inputs",
    "exit": 0,
    "expected_refusal": false,
    "pass": true
  }
]
```

### audit-results.json

SHA-256: `9d467cc77d3581c873a8fe15d231a511d598e917da1380630c6086d20d9d7ac3`.

<!-- data:audit-results.json -->
```json
{
  "input_hashes": 26,
  "original_hashes": 21,
  "availability_hashes": 5,
  "location_transforms": "PASS",
  "trial_results": [
    {
      "trial": 1,
      "ingest": "PASS",
      "symbols": 79,
      "rows": [
        {
          "language": "python",
          "nodes": 22,
          "target": {
            "File": "python/shapes.py",
            "Name": "top",
            "Kind": "PythonFunc",
            "Key": "free-coverage::python/shapes.top",
            "Line": 13,
            "EndLine": 17
          },
          "blocks": 3,
          "statements": 3,
          "covered": 2,
          "percent": 66.66666666666667,
          "outside_spans": [
            [
              74,
              74
            ],
            [
              74,
              74
            ],
            [
              74,
              74
            ]
          ],
          "checks": "PASS"
        },
        {
          "language": "javascript",
          "nodes": 17,
          "target": {
            "File": "javascript/bindings.js",
            "Name": "create",
            "Kind": "JSFunc",
            "Key": "free-coverage::javascript/bindings.create",
            "Line": 17,
            "EndLine": 21
          },
          "blocks": 2,
          "statements": 2,
          "covered": 1,
          "percent": 50.0,
          "outside_spans": [
            [
              32,
              32
            ],
            [
              32,
              34
            ]
          ],
          "checks": "PASS"
        },
        {
          "language": "java",
          "nodes": 15,
          "target": {
            "File": "java/shapes.java",
            "Name": "Widget.sum",
            "Kind": "JavaMethod",
            "Key": "free-coverage::java/com.example.shapes.Widget.sum",
            "Line": 13,
            "EndLine": 21
          },
          "blocks": 2,
          "statements": 2,
          "covered": 1,
          "percent": 50.0,
          "outside_spans": [
            [
              47,
              47
            ],
            [
              47,
              47
            ]
          ],
          "checks": "PASS"
        },
        {
          "language": "ruby",
          "nodes": 5,
          "target": {
            "File": "ruby/calls.rb",
            "Name": "Pipeline.run",
            "Kind": "RubyMethod",
            "Key": "free-coverage::ruby/calls.Pipeline.run",
            "Line": 6,
            "EndLine": 16
          },
          "blocks": 3,
          "statements": 3,
          "covered": 2,
          "percent": 66.66666666666667,
          "outside_spans": [
            [
              38,
              38
            ],
            [
              38,
              38
            ],
            [
              38,
              38
            ]
          ],
          "checks": "PASS"
        },
        {
          "language": "c",
          "nodes": 20,
          "target": {
            "File": "c/shapes.c",
            "Name": "add",
            "Kind": "CFunc",
            "Key": "free-coverage::c/shapes.add",
            "Line": 36,
            "EndLine": 41
          },
          "blocks": 2,
          "statements": 2,
          "covered": 2,
          "percent": 100.0,
          "outside_spans": [
            [
              61,
              61
            ],
            [
              61,
              61
            ]
          ],
          "checks": "PASS"
        }
      ],
      "all_assertions": "PASS"
    },
    {
      "trial": 2,
      "ingest": "PASS",
      "symbols": 79,
      "rows": [
        {
          "language": "python",
          "nodes": 22,
          "target": {
            "File": "python/shapes.py",
            "Name": "top",
            "Kind": "PythonFunc",
            "Key": "free-coverage::python/shapes.top",
            "Line": 13,
            "EndLine": 17
          },
          "blocks": 3,
          "statements": 3,
          "covered": 2,
          "percent": 66.66666666666667,
          "outside_spans": [
            [
              74,
              74
            ],
            [
              74,
              74
            ],
            [
              74,
              74
            ]
          ],
          "checks": "PASS"
        },
        {
          "language": "javascript",
          "nodes": 17,
          "target": {
            "File": "javascript/bindings.js",
            "Name": "create",
            "Kind": "JSFunc",
            "Key": "free-coverage::javascript/bindings.create",
            "Line": 17,
            "EndLine": 21
          },
          "blocks": 2,
          "statements": 2,
          "covered": 1,
          "percent": 50.0,
          "outside_spans": [
            [
              32,
              32
            ],
            [
              32,
              34
            ]
          ],
          "checks": "PASS"
        },
        {
          "language": "java",
          "nodes": 15,
          "target": {
            "File": "java/shapes.java",
            "Name": "Widget.sum",
            "Kind": "JavaMethod",
            "Key": "free-coverage::java/com.example.shapes.Widget.sum",
            "Line": 13,
            "EndLine": 21
          },
          "blocks": 2,
          "statements": 2,
          "covered": 1,
          "percent": 50.0,
          "outside_spans": [
            [
              47,
              47
            ],
            [
              47,
              47
            ]
          ],
          "checks": "PASS"
        },
        {
          "language": "ruby",
          "nodes": 5,
          "target": {
            "File": "ruby/calls.rb",
            "Name": "Pipeline.run",
            "Kind": "RubyMethod",
            "Key": "free-coverage::ruby/calls.Pipeline.run",
            "Line": 6,
            "EndLine": 16
          },
          "blocks": 3,
          "statements": 3,
          "covered": 2,
          "percent": 66.66666666666667,
          "outside_spans": [
            [
              38,
              38
            ],
            [
              38,
              38
            ],
            [
              38,
              38
            ]
          ],
          "checks": "PASS"
        },
        {
          "language": "c",
          "nodes": 20,
          "target": {
            "File": "c/shapes.c",
            "Name": "add",
            "Kind": "CFunc",
            "Key": "free-coverage::c/shapes.add",
            "Line": 36,
            "EndLine": 41
          },
          "blocks": 2,
          "statements": 2,
          "covered": 2,
          "percent": 100.0,
          "outside_spans": [
            [
              61,
              61
            ],
            [
              61,
              61
            ]
          ],
          "checks": "PASS"
        }
      ],
      "all_assertions": "PASS"
    }
  ],
  "Repeatable": true,
  "canonical_sha256": "ebae585ca7ed15b9038e9752af13f037edb2f621cea893bf3c726e39dd0cda7a",
  "feasibility": "PASS",
  "original_cohort": "unread",
  "open_rows": [
    "FCT-10",
    "TQ-M3"
  ],
  "verdict": "INCONCLUSIVE (original cohort unread)"
}
```

FCT-10 verdict: INCONCLUSIVE (original cohort unread)
