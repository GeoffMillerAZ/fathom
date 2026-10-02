# Language breadth: footprint readings

Binary size, embedded grammar bytes and daemon memory per wave (LB-M3,
LB-M4, LB-M5, LB-M7; docs/design/language-breadth.md §8.3, §9). Each wave
appends its section; a verdict line is written only by the wave that
measures it.

## LB-M4 baseline (before wave 1), 2026-09-27

Measured on darwin/arm64, go 1.26.3, `CGO_ENABLED=1`, `go build ./cmd/fathom`
(no `-ldflags`), wazero v1.12.0, five embedded grammars.

| Build | `cmd/fathom` bytes | Embedded grammar bytes |
|---|--:|--:|
| acc362d0 (raw `.wasm`, before LB-F13) | 97,352,834 | 7,234,475 |
| d0caa732 (`.wasm.gz`, LB-F13) | 91,720,434 | 1,483,348 |

Compression saved 5,632,400 bytes of binary (the blobs shrank 5,751,127;
gzip -9 is 4.9×). The embedded `.wasm.gz` total is 3.5% of the 40 MiB cap
`TestEmbeddedGrammarBudget` enforces; at today's mean of 296,670 compressed
bytes a grammar, the cap admits about 140 grammars before the pack decision
(LB-F14's trigger is 36 MiB).

Compiled machine code per grammar (wazero compiler, arm64, measured by
`TestCompilationCacheUserScopeOnly`'s method — the compilation cache's file
size): javascript 844,395; kotlin 1,795,795; python 810,771; ruby 1,257,427;
typescript 943,867 bytes (0.46×–1.84× the module). The memo registry is
charged the module size (`blob_bytes`), since wazero does not report it.

## LB-M4 wave 1, 2026-09-28

Measured at a15b29a3 (lq-integ, wave 1 merged) on darwin/arm64, go 1.26.3,
`CGO_ENABLED=1`, `go build ./cmd/fathom` (no `-ldflags`), seven embedded
grammars (wave 1 added `css`, `html`; Terraform and the manifest formats are
pure Go and embed nothing).

| Build | `cmd/fathom` bytes | Embedded `.wasm.gz` bytes |
|---|--:|--:|
| d0caa732 (baseline, five grammars) | 91,720,434 | 1,483,348 |
| a15b29a3 (wave 1, seven grammars) | 91,975,346 | 1,618,705 |

Per grammar (`.wasm.gz`): css 74,133; html 61,224; javascript 114,833;
kotlin 510,036; python 123,581; ruby 311,992; typescript 422,906.

The wave grew the binary 254,912 bytes and the embedded grammars 135,357.
The embedded total is 3.9% of the 40 MiB (41,943,040-byte) cap and far
below LB-F14's 36 MiB pack trigger.

LB-M4 verdict: PASS (wave 1: 1,618,705 of 41,943,040 bytes)

## LB-M4 wave 1b, 2026-09-28

Measured at 20304d19 (lq-integ, wave 1b merged) on darwin/arm64, go 1.26.3,
`CGO_ENABLED=1`, `go build ./cmd/fathom` (no `-ldflags`), wazero v1.12.0,
thirteen embedded grammars (wave 1b added `bash`, `c`, `csharp`,
`dockerfile`, `java`, `sql`; GitHub Actions and Kubernetes are doc-engine
formats and embed nothing; `cpp` and the TypeScript rework did not merge).

| Build | `cmd/fathom` bytes | Embedded `.wasm.gz` bytes |
|---|--:|--:|
| a15b29a3 (wave 1, seven grammars) | 91,975,346 | 1,618,705 |
| 20304d19 (wave 1b, thirteen grammars) | 93,609,890 | 3,184,249 |

Per grammar added (`.wasm.gz`): bash 262,984; c 136,140; csharp 603,861;
dockerfile 69,325; java 113,109; sql 380,125.

The wave grew the binary 1,634,544 bytes and the embedded grammars
1,565,544. The embedded total is 7.6% of the 40 MiB (41,943,040-byte) cap
and far below LB-F14's 36 MiB pack trigger; the mean is now 244,942
compressed bytes a grammar.

LB-M4 verdict: PASS (wave 1b: 3,184,249 of 41,943,040 bytes)

## Ingest cost after wave 1b (coordinator, 2026-09-28)

The report: `scripts/stranger-check.sh` went from 110 s to 235 s after the language merge
(`1e87fffa`). Measured on darwin/arm64 (10 cores) against the same input — a `git archive`
of `88c961ed` (the tree after waves 1–1b and round 2's harness work) — with two binaries:
**pre** = `8a165059` (the first-parent commit just before the language merge) and **head** =
`88c961ed`. Every run held `<scratch>/lang-measure.lock`; sibling agents' gates
still ran (load average 9–16 on 10 cores throughout), which is the point below.

**Daemon ingest, stranger-shaped** (fresh `HOME`, so a cold Go build cache for the typed
pass; `serve-http` + `fathom init`, the step that was 125 s of the 164 s stranger-check):

| Run | init wall | Go typed-edge pass | daemon's own CPU |
|---|--:|--:|--:|
| pre (1) | 135.2 s | 2m02 | 21.2 s |
| head (1) | 160.4 s | 2m22 | 26.9 s |
| pre (2) | 185.3 s | 2m55 | — |
| head (2) | 95.2 s | 1m21 | — |
| pre (3) | 100.2 s | 1m29 | — |
| head (3) | 115.3 s | 1m41 | — |

The wall is the typed-edge pass: 85–95% of every run, and it is the `go` toolchain as a
child process (type-checking the module's 259 packages for export data from a cold build
cache, niced under RT-106's toolchain cap), not extraction — the daemon itself is 7–11%
busy in a 30 s CPU profile (`syscall.rawsyscalln`, SQLite, `go/parser`), and its own CPU
is 21–27 s of a 95–185 s run. The same binary on the same input swings 81 s → 175 s
with host load, so pre and head overlap completely: the 110 s → 235 s reading is host
contention on the typed pass, not the language merge.

**In-process ingest** (`cmdIngest` under `pprof`, warm Go build cache, fresh store and
compilation cache each run; the second of two runs each, the first pre run paid a cold
cache at 199 s user):

| Binary | wall | user + sys CPU | `fusion.(*Pipeline).Ingest` cum. |
|---|--:|--:|--:|
| pre | 44.4 s | 30.3 s | 13.3 s |
| head | 31.5 s | 29.2 s | 11.8 s |

Profile top entries (head): `EnrichTypedEdges` 5.9 s (of which `PutTypedEdges` 4.8 s,
SQLite), `extractSink.flush`/`PutObservations` 3.0 s, `resolveFileEdges` (Go) 2.2 s,
`EnrichCUEEdges` 1.3 s, `ScanCertifiesMarkers` 0.5 s, and the language program's only
new line: wazero `CompileModule` 0.44 s — each grammar compiled once per process (a cold
compilation cache; warm, it is a file read). Dialect classification (`ClassifyDialect`,
the per-file rule loop, and the generated/minified sniff) is below pprof's 0.01 s cut.

**Verdict.** No hot path to fix: grammar modules are compiled once per process and
reused across files (RT-107 holds), and dispatch cost does not register. The guard
`internal/fusion.TestIngestCostGuard` pins it machine-independently: an ingest of 2 or of
40 files per wasm dialect (TypeScript, JavaScript, Python, HTML, CSS, Dockerfile)
instantiates the same number of grammar modules (9 at concurrency 2; the ceiling is
grammars × concurrency, and the count may not grow with the file count). The stranger
check's wall belongs to the cold-cache Go typed pass under host load — a tenancy-lane
question (warm the typed pass's build cache, or measure that step on CPU like D1), not a
language one.

## LB-M4 wave 2, 2026-09-29

Measured at 88fe67a5 (lq-integ, wave 2 merged) on darwin/arm64, go 1.26.3,
`CGO_ENABLED=1`, `go build ./cmd/fathom` (no `-ldflags`), twenty-two
embedded grammars. Wave 2 added `cpp`, `dart`, `gotemplate` (Helm's
templates), `lua`, `makefile`, `php`, `powershell`, `rust`, `swift`, and
re-pinned `kotlin` (fwcd/tree-sitter-kotlin 0.3.8 to commit 1852ea17).
Docker Compose, GitLab CI, Maven, Cargo, Go modules and MSBuild are
doc-engine or native formats and embed nothing. The wave's base, eb96fcbc,
was built from a `git archive` (no VCS stamp, a few hundred bytes).

| Build | `cmd/fathom` bytes | Embedded `.wasm.gz` bytes |
|---|--:|--:|
| 20304d19 (wave 1b, thirteen grammars) | 93,609,890 | 3,184,249 |
| eb96fcbc (wave 2 base, round-2 harness work, thirteen grammars) | 94,639,090 | 3,184,249 |
| 88fe67a5 (wave 2, twenty-two grammars) | 96,980,578 | 5,122,633 |

Per grammar added (`.wasm.gz`): cpp 420,154; dart 205,433; gotemplate
68,230; lua 64,669; makefile 86,559; php 191,343; powershell 165,241;
rust 204,421; swift 475,154 (1,881,204 together). Kotlin's re-pin grew its
blob 57,180 bytes (510,036 to 567,216).

The wave grew the binary 2,341,488 bytes over its base (1,938,384 of it
grammar bytes, the rest the new dialects' Go code) and the embedded
grammars 1,938,384. The embedded total is 12.2% of the 40 MiB
(41,943,040-byte) cap and far below LB-F14's 36 MiB pack trigger; the mean
is now 232,847 compressed bytes a grammar, so the cap admits about 180
grammars at that mean.

LB-M4 verdict: PASS (wave 2: 5,122,633 of 41,943,040 bytes)

## LB-M5 ingest cost per byte on the polyglot set, 2026-09-29 (wave w6)

Measured on darwin/arm64 (10 cores), go 1.26.3, `GOWORK=off CGO_ENABLED=1 go build
./cmd/fathom` (no `-ldflags`) at two commits: **base** = 56c18502 (the build LB-M1's
baseline readings were taken with, from a `git worktree`) and **head** = 9ea6da5f. No
extraction change and no `ExtractorEpoch` bump.

**Input.** The polyglot set is LB-M1's 18 dialect subjects at the SHAs its readings record
(17 repositories; fmtlib/fmt serves CMake and C++), shallow-fetched by SHA and written with
`git archive` into one tree, one directory per subject, deleted afterwards: 6,548 files;
both builds' `ingest --dry-run` discover the same 6,524 files and 75,971,918 bytes, the
per-byte denominator.

**Method** (the wave-1b in-process method above). `fathom ingest --json --store <fresh
sqlite> --service poly --path <tree>`, niced, under `<scratch>/lang-measure.lock`,
timed by `/usr/bin/time -l` (user + sys includes any child); each run a fresh store and a
fresh compilation cache (a CLI ingest compiles grammars in memory; only the managed daemon
enables the on-disk cache). Two runs a build, interleaved base, head, base, head; the second
of each is the reading. The Go typed-edge pass does not run in either build: the tree's root
has no `go.mod` (`hasGoModule` checks the service root), so its share is zero and the Go
build cache does not enter. No LLM enrichment runs: nothing outside the pipeline wires
`WithEnricher`. Load average (1 min) 20.3 to 30.8 on 10 cores across the four runs (20.3 to
23.4 around the two readings), sibling workers' gates. An earlier set overlapped this
worker's own verify ingest and is discarded (it read 2.21× CPU).

| Build | bytes | wall | user + sys CPU | CPU ns/byte | wall ns/byte | instructions retired |
|---|--:|--:|--:|--:|--:|--:|
| base 56c18502 | 75,971,918 | 20.81 s | 17.29 s | 227.6 | 273.9 | 236.5 G |
| head 9ea6da5f | 75,971,918 | 36.95 s | 32.04 s | 421.7 | 486.4 | 523.3 G |
| head ÷ base | | 1.78× | 1.85× | **1.85×** | 1.78× | 2.21× |

The verdict uses D1's CPU basis (§1.5): 1.85×, over the 1.5× bound. Wall (1.78×) and the
load-independent instruction count (2.21×; first runs 2.23×) agree; the first runs read
2.24× CPU and 2.10× wall. `TestIngestCostGuard` (`internal/fusion/ingestcost_test.go`) pins
the six-module pool's invariant, grammar instantiations bounded by grammars × concurrency
and flat in the file count; the cost here is not per-file instantiation.

**Where it goes.** Head does more work on the same bytes: 81,676 symbols against 68,288,
5,742 file-resolved files against 973, and a doc index (3,226 units) base does not have.
Ingested one subject at a time (nested one level, so no subject's `go.mod` is a service
root; one run each; 2.04× CPU and 2.20× instructions in sum), the growth is +300.6 G
instructions:

| Subject | base G instr | head G instr | × | what head adds |
|---|--:|--:|--:|---|
| square/retrofit | 25.8 | 151.2 | 5.86 | its 3.6 MB of javadoc HTML and 0.4 MB of CSS, unsupported at base: 8,160 CSS and HTML symbols |
| App-vNext/Polly (C#) | 26.2 | 64.0 | 2.44 | a second, wasm parse of every `.cs` file for the file-resolved pass (it resolves none); 502 doc units |
| square/okhttp (Kotlin) | 51.5 | 87.2 | 1.69 | 1,422 file-resolved files; 3,495 more symbols |
| BurntSushi/ripgrep (Rust) | 11.2 | 32.3 | 2.89 | 674 file-resolved files |
| libuv/libuv (C) | 14.7 | 32.7 | 2.22 | 1,511 file-resolved files |
| fmtlib/fmt (C++, CMake) | 14.1 | 28.5 | 2.02 | 604 file-resolved files |
| the other eleven | 107.5 | 155.7 | 1.45 | |

Two causes carry it. **New coverage**: retrofit alone is 42% of the growth (the set without
it reads 1.78× on instructions, still over). **The file-resolved import pass** (L3): at base
it covered Python and JavaScript/TypeScript; at head it covers sixteen tags dialects, and
for C, C++, C#, Java, PHP, Protocol Buffers and Rust, whose live extractors are still the
incumbents, it parses every file a second time in the wasm tags engine. In an in-process
profile (`cmdIngest` under `pprof`, one run a build) code outside Go's symbol table (the
grammars' wasm under wazero and the CGO parsers, which pprof cannot split) is 11.7 s of
head's 33.7 s of samples against 2.9 s of base's 14.3 s; the scheduler's wakeups
(`pthread_cond_signal`) 5.8 s against 2.0 s; SQLite's `PutObservations` 4.8 s against 4.2 s;
wazero `CompileModule` 1.25 s against 0.27 s (each grammar the set uses compiled once in the
process; base embeds five); the file-resolved pass's Go side 1.6 s against 0.1 s;
`recordCentrality` 1.1 s.

LB-M5 verdict: FAIL (1.85× CPU per byte: 421.7 vs 227.6 ns over 75,971,918 bytes; wall 1.78×, instructions 2.21×; O208)

## LB-M7 answer payload with signatures and ranges, 2026-09-29 (wave w6)

Measured on darwin/arm64 (10 cores), go 1.26.3, `GOWORK=off CGO_ENABLED=1 go build
./cmd/fathom` (no `-ldflags`) at two commits: **head** = 9ea6da5f (signatures and end lines
on) and **base** = 56c18502 (the LB-M1 baseline build, the cross-check). No extraction
change and no `ExtractorEpoch` bump.

**Method.** The fixture `internal/connector/testdata/fauxtown/polyglot-service` (9 files,
1,465 bytes), copied to scratch, was ingested by each binary into its own private SQLite
store; head stores 27 symbols, base 26. A scratch probe drove each binary's stdio MCP
surface (`serve-mcp --store <scratch> --auto-refresh=false`, isolated `HOME`) with
`find_symbol(name)` and `symbol_context(symbol)` for each of the 26 distinct names head
defines (`Greeter` names the JavaScript and the Python class): 52 calls a build. The bytes
are each response's text payload (`content[0].text`, the JSON the agent reads), summed; the
whole JSON-RPC line, which carries the same object again as `structuredContent`, is shown
beside it. The counterfactual is head's own responses with every `signature` and `end_line`
key removed and re-encoded the way `encoding/json` encodes them (the re-encoder reproduces
all 104 real responses byte for byte). symbol_context's snippet needs no counterfactual: the
handler reads it from the definition's first line with no end line (`readSourceRange(...,
Line, 0, 40, ...)`, `internal/mcp/nav_tools.go`) in both builds, so an `EndLine` never
bounds it.

| Payload bytes, 52 calls | head | head without `signature`, `end_line` | growth | base (56c18502) |
|---|--:|--:|--:|--:|
| `find_symbol`, text | 6,689 | 5,575 | +20.0% | 5,970 |
| `symbol_context`, text | 14,950 | 14,169 | +5.5% | 13,943 |
| **both, text** | **21,639** | **19,744** | **+9.6%** | 19,913 |
| both, whole JSON-RPC line | 51,590 | 47,500 | +8.6% | 47,902 |

The L1 fields are 1,895 text bytes: 37 signatures (1,109 bytes; JavaScript functions 255,
Python 146, Terraform modules 136, JavaScript methods 104, doc chunks 102, Terraform
resources 100) and 59 end lines (786 bytes). The verdict sums both tools, as the row names
them together; per tool, `find_symbol` grows 20.0% because its rows are short (about 200
bytes) and a 20–60-byte signature is a large share of one, while `symbol_context`'s snippet
dilutes the same fields to 5.5%. The margin is 0.4 points: Java methods (7 rows here) are
the callables that still carry no signature, and a signature of about 40 bytes on each would
put the sum near 11% on this fixture (O209).

**Cross-check against base.** With both builds' `signature` and `end_line` keys removed,
head and base differ only by the `GoModule` symbol head extracts from `go.mod` (636 text
bytes: its `find_symbol` hit, the `symbol_context` neighbourhood base answers with
`def: null`, and the two `ModuleDep` rows' new caller); `index_commit` is identical. Base
already carried 805 bytes of the two fields (28 end lines from Java, CUE, Terraform and doc
chunks; 18 signatures from doc chunks, Kubernetes, `go.mod` requirements and Terraform).
Head over base is +8.7% of text: 1,090 bytes of added L1 fields and the 636-byte
`GoModule` symbol.

LB-M7 verdict: PASS (+9.6%: 21,639 vs 19,744 text-payload bytes over 52 find_symbol and symbol_context calls; find_symbol alone +20.0%)

## LB-M5 ingest cost per byte on the polyglot set, 2026-10-01 (wave cx, item calls: the C dispatch switch)

Measured on darwin/arm64 (10 cores), go 1.26.3, `GOWORK=off CGO_ENABLED=1 go build ./cmd/fathom`
(no `-ldflags`) at three commits: **base** = 56c18502 (LB-M1's build, from a `git worktree`),
**main** = 32afe369 (this wave's base, before the switch) and **head** = 5556eb84 (`.c` and C
headers served by the C tags dialect, LB-F119, `ExtractorEpoch` 45). C++ is not switched
(lang-parity refuses it, `docs/bench/readings/lang/parity/cpp.json`).

**Input.** The w6 polyglot set: LB-M1's 18 dialect subjects at the SHAs their readings record
(17 repositories), shallow-fetched by SHA and written with `git archive` into one scratch tree,
6,548 files; all three builds' `ingest --dry-run` discover the same 6,524 files and 75,971,918
bytes.

**Method** (w6's). `fathom ingest --json --store <fresh sqlite> --service poly --path <tree>`,
niced, timed by `/usr/bin/time -l`, each run with a fresh store, `FATHOM_HOME` and
`FATHOM_CACHE_ROOT`; runs interleaved base, main, head, the second valid run of each is the
reading. A run with a read failure or a failed doc unit is not valid: the external volume's TCC
denial (EPERM) failed 293 doc files in main's second run and two reads in head's third, both
discarded. Load average (1 min) 6.7 to 16.3, sibling waves' gates.

| Build | bytes | wall | user + sys CPU | CPU ns/byte | wall ns/byte | instructions retired |
|---|--:|--:|--:|--:|--:|--:|
| base 56c18502 | 75,971,918 | 20.32 s | 16.90 s | 222.4 | 267.5 | 238.1 G |
| main 32afe369 | 75,971,918 | 94.19 s | 83.28 s | 1,096.2 | 1,239.8 | 1,287.7 G |
| head 5556eb84 | 75,971,918 | 91.99 s | 81.78 s | 1,076.4 | 1,210.8 | 1,311.6 G |
| head ÷ base | | 4.53× | 4.84× | **4.84×** | 4.53× | 5.51× |

The first valid runs agree (base 17.97 s and 238.8 G, main 76.17 s and 1,283.5 G, head 77.08 s
and 1,312.2 G: head ÷ base 4.29× CPU, 5.49× instructions). The CPU difference between main and
head is load; the instruction count says the switch adds 23.9 G (+1.9%).

**The switch's own share.** Ingested one subject at a time (nested one level, one run each),
only libuv moves: 33.5 G at main, 42.1 G at head (+8.65 G, +26%); every other subject is within
±1%. C extraction moved from the CGO walker to the wasm tags engine, and the file-resolved pass
still parses each C file a second time (`ExtractFileSpecs` on the same `TSExtractor`, which
`fileResolveExtractor` now finds in the dispatch table): a C file is parsed twice in wasm where
it was parsed once natively and once in wasm. One parse giving both the symbols and the file
specs is O361; it lives in `internal/fusion/fileresolve.go` and the tags engine, outside this
item's files.

**Where the rest goes.** Summed over the subjects ingested alone: base 250.1 G, main 670.1 G,
head 680.6 G (head ÷ base 2.72×): Polly 3.23× and ripgrep 3.27× (C# and Rust still parsed by
their CGO walkers and again in wasm), retrofit 6.35× (its javadoc HTML and CSS), guzzle 4.71×,
chinook 4.47×, libuv 2.90×, fmt 2.52×. And the whole tree now costs 1.93× the sum of its
subjects (1,311.6 G against 680.6 G), where at w6 the two agreed (523.3 G whole against
551.6 G summed): a cost that grows with the service, not with its bytes, which appeared between
9ea6da5f and 32afe369 (O362). That component alone is past the bound.

LB-M5 verdict: FAIL (4.84× CPU per byte: 1,076.4 vs 222.4 ns over 75,971,918 bytes; wall 4.53×, instructions 5.51×; the C switch adds 1.9%; O361, O362)

## LB-M4 wave pc, 2026-10-02

Measured on darwin/arm64, go 1.26.3, `CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 GOWORK=off
go build ./cmd/fathom` (no `-ldflags`), thirty-one embedded grammars, item native of wave pc
(branch pc-native). The item adds no grammar: Rego is read by a reader in Go with no
dependency (LB-F131; ADR-048's wave-pc amendment), and Cedar is refused (LB-Q-cedar-rest).
The two libraries the readers were weighed against were linked into the same build by a probe
never committed (a file in `cmd/fathom` whose `init` calls the parser behind an environment
check, so the linker keeps it and its dependencies):

| Build | `cmd/fathom` bytes | over eb900e7a |
|---|--:|--:|
| eb900e7a (the wave's base) | 102,371,538 | |
| eb900e7a + OPA's parser (`github.com/open-policy-agent/opa/v1/ast` v1.18.2, the option not taken) | 108,821,826 | +6,450,288 |
| eb900e7a + cedar-go v1.8.0's policy and schema parsers (refused) | 103,239,218 | +867,680 |
| 0f82424c (the Rego reader, the rego-package strategy, the file-resolver hook) | 102,441,282 | +69,744 |

OPA's parser also raises `golang.org/x/net` (v0.54.0 to v0.55.0) and
`google.golang.org/protobuf` (v1.33.0 to v1.36.11) and adds gojsonschema, go-metrics,
xxhash and a second yaml module; cedar-go adds a 2022 `golang.org/x/exp`.

Parse robustness on the pinned subjects, the reference parser's acceptance as the
denominator: Rego, StyraInc/regal v2.2.3 `bundle/` and open-policy-agent/opa v1.18.2
(370 files OPA 1.18.2 accepts, 43 it rejects): the reader needs a repair on 0 of the 370
and faults on none; OPA's parser reads all 370 by construction (it is the oracle's).
Cedar, cedar-policy/cedar-examples c251f7d1 and cedar-integration-tests v4.13.0 (42 policy
and 17 schema files, all accepted by cedar-policy-cli 4.13.0): chrnorm/tree-sitter-cedar
5092204d leaves an ERROR node in 5 of the 42 policy files and has no schema grammar;
cedar-go rejects 6 of the 42 and gives no schema declaration a position.

The embedded `.wasm.gz` total is unchanged: 6,340,653 bytes (31 grammars), 15.1% of the
40 MiB (41,943,040-byte) cap and far below LB-F14's 36 MiB pack trigger.

LB-M4 verdict: PASS (wave pc: 6,340,653 of 41,943,040 bytes; the Rego reader +69,744 bytes against OPA's parser +6,450,288)

## LB-M5 ingest cost per byte on the polyglot set, 2026-10-02 (wave ic: items linear and oneparse merged)

Measured on darwin/arm64 (10 cores), go 1.26.3, `GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0
go build ./cmd/fathom` (no `-ldflags`) at two commits: **base** = 56c18502 (LB-M1's build, from
`git archive`) and **head** = 3d795e02 (wave ic's merged tree over origin/main 5a3ca691: the doc-link
pass reads symbols and open links once per pass, EW-49 and EW-50, and a file is parsed once for its
symbols and its file specs, EW-52 and EW-53; `ExtractorEpoch` 46).

**Input.** The w6 polyglot set (17 repositories at the SHAs in
`docs/bench/readings/efficiency-wave/one-parse.md`'s command sequence); both builds'
`ingest --dry-run` discover 6,524 files and 75,971,918 bytes, `stat_failed` 0.

**Method.** That sequence: `fathom ingest --json --store <fresh sqlite> --service poly --path <tree>`
under `nice -n 10 /usr/bin/time -l`, a fresh store, `FATHOM_HOME` and `FATHOM_CACHE_ROOT` every run;
the whole tree base, head, base, head, then every subject nested one level, twice per build; the
second valid run is the reading. A run is valid with `read_failed` 0, no doc failure, no unfinished
doc index, no EPERM in its timing, and `symbols`, `file_resolved`, `docs.units_written` and
`docs.links_written` equal to its twin's (O411's row check); six subject runs were rerun on those
rules, no whole-tree run. Load average (1 min) 4.8 to 6.8.

| Build | bytes | wall | user + sys CPU | CPU ns/byte | wall ns/byte | instructions retired |
|---|--:|--:|--:|--:|--:|--:|
| base 56c18502 | 75,971,918 | 20.51 s | 15.78 s | 207.7 | 270.0 | 235.7 G |
| head 3d795e02 | 75,971,918 | 33.31 s | 30.30 s | 398.8 | 438.5 | 485.8 G |
| head ÷ base | | 1.62× | 1.92× | **1.92×** | 1.62× | 2.06× |

The first valid runs agree (base 16.41 s and 236.1 G, head 30.95 s and 486.4 G: 1.89× CPU, 2.06×
instructions).

**What wave ic took out.** Against the 2026-10-01 head (5556eb84: 1,311.6 G, 81.78 s CPU, 4.84×) the
whole tree costs 63% fewer instructions. The superlinear term is gone: whole over the subjects summed
reads 0.93 at head (485.8 G against 519.7 G; first runs 0.94) and 0.94 at base (235.7 G against
249.8 G), where it read 1.93 on 2026-10-01 and 2.94 on item oneparse's branch alone (O362).

**Where the rest goes.** Summed over the subjects head costs 2.08× base (519.7 G against 249.8 G), the
whole tree's ratio: what is left is per byte, the work base never did (85,299 symbols against 68,288;
27,368 file-resolved edges against 973; 3,226 doc units and 1,880 doc-to-code links base does not
write). In the head whole run's `pass_times` `extract` is 22.9 s of 30.3 s CPU (its `store_writes`
5.4 s), `file_resolved` 3.3 s, `centrality` 1.4 s, `doc_index` 0.7 s, `policy_ref` 0.55 s and
`doc_xref` 0.5 s. By subject (head ÷ base, share of the 269.9 G added): retrofit 4.38× (+86.5 G,
32%; `extract` 5.6 of its 6.2 s, its javadoc HTML and CSS), fmt 5.01× (+55.8 G, 21%; `file_resolved`
1.8 of 4.1 s), libuv 2.66× (+24.2 G, 9%; C in wasm and its file-resolved pass), okhttp 1.37×
(+19.0 G), chinook 5.50× (+17.7 G; `doc_xref` and `policy_ref` 0.2 s each over its data files),
Polly 1.59× (+15.3 G), ripgrep 2.14× (+12.8 G); the other ten +38.6 G together.

LB-M5 verdict: FAIL (1.92× CPU per byte: 398.8 vs 207.7 ns over 75,971,918 bytes; wall 1.62×, instructions 2.06×; whole over summed 0.93, O362 fixed; the rest is per byte, led by retrofit's markup and fmt's file-resolved pass)

## LB-M5 profile, wave le item pycost (2026-10-02)

This section profiles the two causes the section above names, retrofit's markup and fmt's file-resolved pass,
fixes what is fixable without dropping a file type, and measures again. It writes no verdict line: the
integrator re-reads LB-M5 on the merged tree, by this file's convention, and flips it only on a PASS.

**Builds and method.** **Before** = 5f41217d (the base of wave le), **after** = 3cde84c9 plus the plan, observation
and Python test commits (the code is 3cde84c9's), each `GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0
go build ./cmd/fathom`. The method is the section above's: `fathom ingest --json --store <fresh sqlite> --service
poly --path <tree>` under `nice -n 10 /usr/bin/time -l`, a fresh store, `FATHOM_HOME` and `FATHOM_CACHE_ROOT`
every run, before and after interleaved, the second valid run the reading; a run is valid with `read_failed` 0, no
EPERM in its timing and `symbols`, `file_resolved`, `docs.units_written` and `docs.links_written` equal to its
twin's (O411's row check; three runs lost to the volume's EPERM wedge were discarded and rerun). The polyglot set
is `w-oneparse/poly`, the subjects the same trees as the 17-repository set (6,524 files, 75,971,918 bytes). Load
average (1 min) 8 to 19 during the readings, so CPU seconds are noisy (the first retrofit runs read 10.53 and 9.13
s, the second 9.98 and 8.24); **instructions retired are the reading**.

### Per subject, before and after

| Subject | instr before | instr after | delta | CPU before | CPU after | `extract` CPU | `file_resolved` CPU | symbols, file_resolved, units, links |
|---|--:|--:|--:|--:|--:|---|---|---|
| retrofit | 112.95 G | 93.85 G | -16.9% | 9.98 s | 8.24 s | 8.82 to 7.13 s | 0.35 to 0.35 s | 15,302, 1,458, 106, 102 (both) |
| fmt | 70.29 G | 50.80 G | -27.7% | 6.73 s | 5.22 s | 3.57 to 3.64 s | 2.82 to 1.29 s | 5,627, 1,686, 137, 149 (both) |
| whole tree | 492.63 G | 451.00 G | -8.5% | 31.74 s | 30.50 s | 23.89 to 23.61 s | 3.24 to 2.20 s | 85,299, 27,368, 3,226, 1,880 (both) |

The first valid runs agree (retrofit 113.49 G to 94.32 G, fmt 70.50 G to 50.20 G; whole tree 494.12 G to 452.10 G).
The section above read 485.8 G at 3d795e02; 5f41217d reads 492.6 G on this machine (what the merges between them
added), and 451.0 G after. The whole tree's instruction ratio to LB-M1's base (235.7 G)
is 1.91 after and 2.09 before; the bar is on CPU per byte, 1.5x of 15.78 s, 23.7 s, against 30.5 s here: **the
gap that is left is 6.8 s, and this item closes about 1.2 s of the 14.5 s the section above attributes to head**.

### retrofit: where the markup cost was, and the fix

`extract` was 5.6 of 6.2 s. Per extension, the 536 javadoc pages (3.6 MB) are about 40 of the subject's 94 G after
the fix and 59 before (the HTML dialect's `ExtractWithFileSpecs`, startup included); CSS (32 files, 388 KB) is 0.3 s
and the pages' 1,297 injected scripts 0.2 s of the pages' 3 s. Instructions retired for the html grammar alone, one
phase at a time:

| Phase over the 536 pages | before | after |
|---|--:|--:|
| parse only (3.2k instructions per byte) | 13.3 G | 13.3 G |
| tags and imports query | 36.4 G | 17.8 G |
| of which: a trivial `(document) @d` query, one match per file | 4.1 G | 4.1 G |
| `id` patterns (2) / custom element (2) / form and field (5) / imports (1), each group alone over the parse | 11.5 / 10.5 / 19 / 5.2 G | 8.8 (the start-tag pattern; the self-closing one is guarded) / about 0 / about 0 / 5.2 G |

The query is not slow in the host: 308,188 structural matches, 18,181 kept by the host-side text predicates, and
resolving the 963,425 captures of the dropped ones is 0.5% of the total (O453: a failing match dropped before its
captures are resolved read 49.96 G against 50.22 G, so the change was reverted). It is the guest: each pattern is
`(element (start_tag (tag_name) @_tag (#eq? ...) (attribute ...)))`, matching every tag or attribute of the page
structurally, and every pattern costs 3 to 9 G of the guest's work whether or not its predicate keeps a match. 0 of
the 536 pages hold a hyphenated tag or a form and 4 hold a field.

**The fix (LB-F180, LB-F181).** `(#requires-text! "re")` states what a source must contain for the pattern to match
at all; the engine runs a source the guard fails in a variant of the query compiled once without that pattern
(pattern indexes and capture names mapped back), so the guest does not run it. HTML's custom-element, form, field and
self-closing-`id` patterns carry guards that are necessary conditions (a hyphenated name after `<`, `<form` and
`action`, an input tag and `name`, `/>`); `TestHTMLGuardsAreNecessary` runs the whole query guarded and unguarded
over every HTML file of the repository and the synthetic cases, the dialect goldens are byte-identical, and the
counts are equal (above). The 36.4 G query is 17.8 G; retrofit is 16.9% fewer instructions and 1.2 s less `extract`.

What is left in retrofit (93.9 G): the html parse 13 G and the two patterns every page can satisfy (`id`, 8.8 G;
`src`/`href` imports, 5.2 G); the html injections (a second traversal, 4 G, and 1,297 script ranges parsed again);
the other grammars, java 306 files, kotlin 16, css 32 and javascript 33, 4 to 9 G each with their cold module
compiles; and the 536 pages' doc rows. An HTML parse costs 3.2k instructions a byte and a Kotlin parse 4.7k: the wasm
grammars themselves, which the engine's guest cannot change.

### fmt: where the file-resolved pass went, and the fix

`file_resolved` was 1.8 of 4.1 s (2.82 s here). A timer around each stage of `resolveLang` put 3.9 s wall of the
4.7 s pass in the C resolver's step over foreign headers: fmt has 26 `.h` files, 24 of them read as C++ by the router,
and the resolver for fmt's one C file (test/c-test.c) parsed every one with the C grammar to read their includes and
prototypes (O451). **The fix (LB-F179)**: a foreign header is parsed, and admitted to the include set, only when an own
file's import, or a reached header's text, names its base name. Every include resolves to a path that ends in its
operand, so no other header can be an include's target and the edges are the eager read's (the test builds a chain of
reached headers and an unreached one and compares edges, rows and the paths parsed with the eager shape's; the fmt
rows above are identical). fmt: `file_resolved` 2.82 to 1.29 s, 70.29 G to 50.80 G. What is left of the pass, from the
same stage timers: the three headers the C resolver does reach (core.h, format.h and format-inl.h, about 0.46 s), the
cold compile of the C++ wasm module for the two foreign C headers (about 0.34 s) and of the cmake module for two cmake
files (about 0.35 s), and `resolveImports` 0.25 s.

### The polyglot subjects after the fixes (instructions retired, one run each, standalone)

retrofit 94.6 G, okhttp 72.0, zod 57.3, fmt 50.4, Polly 41.9, libuv 36.4, chinook 21.9, ripgrep 21.0, protovalidate
17.7, sinatra 15.2, flask 12.6, express 11.0, terraform-aws-vpc 8.1, telescope 7.6, nvm 6.5, guzzle 5.1, cobra 4.9; sum
484.2 G (each pays its own module compiles, so the sum exceeds the whole tree's 451.0 G). okhttp's 72 G is 40 G of
Kotlin (parse 19 G for 4 MB, the tags and the references queries 8 G each); zod's is item tsjs's.

### What is not here

LB-M5 stays unmet on this item's evidence: the two named causes are cut (retrofit -16.9%, fmt -27.7%) and the whole tree
reads 1.91x instructions, 1.93x CPU against LB-M1's base. The rest is per byte in files the base never indexed: the
wasm parse of HTML, Kotlin, C# and the dialects (3 to 5k instructions a byte), the doc rows, and `store_writes` (5 s
of the whole tree's `extract`, not in this item's files). A guard (LB-F180) can be added to any dialect whose patterns
match broadly and are cut by predicates (gradle, cmake, commonlisp, helm, meson, conan and ruby carry them), each with
its own guarded-vs-unguarded test; none is added here. Observations O451 to O453.

## LB-M5 ingest cost per byte on the polyglot set, 2026-10-02 (wave le: items tsjs, go and pycost merged)

Measured on darwin/arm64 (10 cores), go 1.26.3, `GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0
go build ./cmd/fathom` (no `-ldflags`) at two commits: **base** = 56c18502 (LB-M1's build, from `git archive`) and
**head** = 8442dbaf (wave le's merged tree over origin/main 5f41217d: item pycost's foreign-header and
`#requires-text!` cuts, LB-F179..LB-F181, and item tsjs's TS/JS call sites, `ExtractorEpoch` 48).

**Input and method.** The w6 polyglot set as above (`w-oneparse/poly`, 17 repositories, 6,524 files, 75,971,918
bytes), the method of the wave ic section: `fathom ingest --json --store <fresh sqlite> --service poly --path <tree>`
under `nice -n 10 /usr/bin/time -l`, a fresh store, `FATHOM_HOME` and `FATHOM_CACHE_ROOT` every run, the whole tree
base, head, base, head, the second valid run the reading. Every run is valid by O411's row check: `read_failed` 0, no
EPERM, no doc failure, and `symbols`, `file_resolved`, `docs.units_written` and `docs.links_written` equal to its
twin's (base 68,288 and 973; head 85,299, 27,549, 3,226 and 1,880); one base run lost to the volume's EPERM wedge was
discarded and rerun. Load average (1 min) 6.5 to 9.1, so CPU seconds are noisy; whole tree only, no subject runs.

| Build | bytes | wall | user + sys CPU | CPU ns/byte | wall ns/byte | instructions retired |
|---|--:|--:|--:|--:|--:|--:|
| base 56c18502 | 75,971,918 | 23.08 s | 16.59 s | 218.4 | 303.8 | 238.4 G |
| head 8442dbaf | 75,971,918 | 40.19 s | 29.43 s | 387.4 | 529.0 | 451.8 G |
| head ÷ base | | 1.74× | 1.77× | **1.77×** | 1.74× | 1.90× |

The first valid runs agree (base 17.23 s and 237.8 G, head 29.36 s and 456.0 G: 1.70× CPU, 1.92× instructions).
Against wave ic's head (485.8 G) the merged tree costs 7.0% fewer instructions, item pycost's cut; head's
`file_resolved` count is 27,549 where wave ic's head read 27,368 (not attributed here). At the 1.5× bar head
would spend 24.9 s of CPU; it spends 29.4 s, a gap of 4.5 s, all of it per byte in files the base never indexed
(the section above).

LB-M5 verdict: FAIL (1.77× CPU per byte: 387.4 vs 218.4 ns over 75,971,918 bytes; wall 1.74×, instructions 1.90×; the second valid runs, load 6.5 to 9.1)

## LB-M5 ingest cost per byte on the polyglot set, 2026-10-02 (wave lg, item pycost)

Measured on darwin/arm64 (10 cores), go 1.26.3, `GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0
go build ./cmd/fathom` (no `-ldflags`) at two commits: **base** = 56c18502 (LB-M1's build, from `git archive`) and
**head** = b1a764b2 (wave le's merged tree 90f0a01f plus this item's three cuts, LB-F208 to LB-F210; no extraction
change, no `ExtractorEpoch` bump, the stored output equal).

**Input and method.** The w6 polyglot set rebuilt (`w-pycost/poly`: the 17 repositories at the SHAs the LB-M1
baseline readings record, `git archive` of a depth-1 fetch by SHA; `ingest --dry-run` reads 6,524 files and
75,971,918 bytes, the w6 denominator), `fathom ingest --json --store <fresh sqlite> --service poly --path <tree>`
under `nice -n 10 /usr/bin/time -l`, a fresh store, `FATHOM_HOME` and `FATHOM_CACHE_ROOT` every run, base, head,
base, head, the second valid run the reading. A run that read the volume's EPERM wedge (nonzero `read_failed`, outcome
not ok, or no output) was discarded and rerun, four to six times a protocol. Every run is valid by O411's row check:
`read_failed` 0, and `symbols`, `file_resolved`, `docs.units_written` and `docs.links_written` equal to its twin's
(base 68,288 and 973; head 85,299, 27,549, 3,226 and 1,880, wave le's head's counts).

| Build | bytes | wall | user + sys CPU | CPU ns/byte | wall ns/byte | instructions retired |
|---|--:|--:|--:|--:|--:|--:|
| base 56c18502 | 75,971,918 | 17.56 s | 14.67 s | 193.1 | 231.1 | 236.5 G |
| head b1a764b2 | 75,971,918 | 25.91 s | 23.47 s | 308.9 | 341.0 | 379.4 G |
| head ÷ base | | 1.48× | 1.60× | **1.60×** | 1.48× | 1.60× |

Load average (1 min) 3.5 to 5.0 for these second runs; the first runs read base 14.70 s and 236.8 G, head 23.46 s and
381.5 G (1.60× both). An earlier protocol of the same two binaries at load 9.5 to 12.4 read base 16.95 s and head
25.50 s CPU (**1.504×**) and 237.0 G and 379.8 G instructions (1.60×): the load inflates the cheaper, more
syscall-bound base more than the head, so the CPU ratio is only trustworthy at a low load, and the instruction
ratio, 1.60× in every run, is the figure to carry (O489). Against wave le's head (451.8 G) the head costs 16.0% fewer
instructions and the CPU ratio falls from 1.77×; at the 1.5× bar head would spend 22.0 s of CPU at this load,
it spends 23.5 s, a gap of 1.5 s or 24.6 G instructions (6.5%).

**Where the cuts came from.** A `runtime/pprof` CPU profile of the whole ingest (a hook patched into main for the
run, not committed), base against wave le's head (17.2 s against 27.5 s of samples):

| Cost (profile seconds) | base | head (le) | after this item |
|---|--:|--:|--:|
| `PutObservations`, of which the statement journal's temp-file writes | 4.9, 3.7 | 4.7, 3.5 | 2.9, memory 1.9 |
| `DetectsContent` (content-sniff NFAs, four passes) | 0 | 0.84 | about 0.1 |
| `connector.globMatch` (`strings.Split` per glob per file per pass) | small | 0.6 | about 0.1 |
| goroutine wake-ups (`pthread_cond_signal`) | 1.3 | 3.5 | 3.4 (open, below) |
| guest code (`runtime._ExternalCode`, the wasm modules) | 5.2 | 8.3 | 6.4 |

- **LB-F208 (O486).** An insert into `observations` fires the call-index trigger, so each row of the ingest
  transaction opens a statement journal of the pages it touches; a call-heavy row (hundreds of random
  `observation_calls` index pages) passed SQLite's 64 KiB spill and created, wrote and unlinked a temp file per row.
  The spill is raised to 4 MiB (`sqlite3_config`, before the first connection), the rows written are the rows
  written before. Interleaved on one binary at load 4 to 17: CPU 26.2 s mean against 29.0 s, 16 MiB and 64 MiB the same as
  4 MiB; instructions 443.2 G to 415.5 G.
- **LB-F209 (O487).** The three cloud-policy sniffs, CloudFormation's and OpenAPI's ran an unanchored NFA over every
  JSON and YAML file in each of four passes; chinook's 1.9 MB data JSON was most of its 21.9 G. `sniffLiterals` reads each
  pattern's syntax tree for the literals a match must contain and the NFA runs only on a source holding one: chinook
  6.0 G, the set 415.5 G to 396.2 G.
- **LB-F210.** `globMatch` checks the last pattern segment against the path's base name before splitting either;
  every pass classifies every file against every manifest glob. The profile read 0.6 s of the set in it.
- **Not a cut (O488).** Gradle's 31 scripts cost 16.7 G of the set; 73 `#requires-text!` guards on its 40-pattern
  query pruned patterns per script but compiled a query variant for each distinct subset (about 5 G each, 16 variants
  used up): 95.4 G, the symbols identical. Reverted. LB-F180's guards pay on a small query and few subsets (HTML),
  not here; ruby's predicated patterns are its cheap ones, and the other named candidates (cmake, commonlisp, helm,
  meson, conan) are not in the set's hot files.

**What the 24.6 G still owed is made of** (instructions, standalone subjects after the cuts: retrofit 86.5, okhttp
62.8, zod 51.6, fmt 47.6, Polly 37.1, libuv 33.9, ripgrep 18.7, sinatra 14.3, protovalidate 13.9, flask 11.6, express
10.9, terraform-aws-vpc 7.7, telescope 7.0, nvm 6.2, chinook 6.0, guzzle 4.7, cobra 4.5; sum 425 G, each paying its
own module compiles):

- **HTML** (retrofit's 536 pages, 42.0 G): no tags query 28.8, no imports query 40.5, no injections 39.55 (50 JavaScript
  symbols fewer), no query at all 23.3 (parse and startup). The tags query is 13.2 G, nearly all the `id` pattern
  matching every attribute; rooting it at the attribute alone (no `element`/`start_tag` chain) reads 38.0 G, 4.0 G
  less, but moves the definition's range from the element to the attribute, so it needs the host to walk to the
  parent, which changes goldens. Not done.
- **Goroutine wake-ups**, 2.2 to 2.6 s of CPU over base in `runtime.ready` from a channel send (the profile loses the
  sender's stack across `systemstack`); `tsengine.Parser.bounded` starts a goroutine and a channel per guest operation and
  is the candidate. Per-row driver goroutines were tried (`context.WithoutCancel` in the row loops, kept: no measured CPU, no
  harm) and are not it.
- **Centrality** 1.0 s over base (`computeCentrality`'s `callgraph.BuildBoth` is item go's), **wazero compile** 0.8 s
  (the CLI keeps no module cache), **doc rows** 0.5 s, the statement journal still 1.9 s in memory (fewer journalled
  pages per row, or a call index maintained after the bulk insert and not by trigger), and the Kotlin, C#, C++, Java and
  file-resolver work wave ld owns.

LB-M5 verdict: FAIL (1.60× CPU per byte: 308.9 vs 193.1 ns over 75,971,918 bytes; wall 1.48×, instructions 1.60×; the second valid runs, load 3.5 to 5.0; a protocol at load 9.5 to 12.4 read 1.504×)
