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

## LB-M5 ingest cost per byte on the polyglot set, 2026-10-02 (wave ld, item dispatch: the PHP dispatch switch)

Measured on darwin/arm64 (10 cores), go 1.26.3, `GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0
go build ./cmd/fathom` (no `-ldflags`) at three commits: **base** = 56c18502 (LB-M1's build, a `git worktree`),
**main** = 5c891a06 (origin/main, this wave's base) and **head** = c34559d4 (the wave ld dispatch branch: `.php`
served by the PHP tags dialect, LB-F183, `ExtractorEpoch` 48). Rust, C#, Java and C++ are not switched
(lang-parity refuses each, `docs/bench/readings/lang/parity/`), so PHP is the only switch this reading prices.

**Input.** The w6 polyglot set (17 repositories at the SHAs in
`docs/bench/readings/efficiency-wave/one-parse.md`'s command sequence); all three builds' `ingest --dry-run`
discover 6,524 files and 75,971,918 bytes, `stat_failed` 0. Its one PHP subject is guzzle/guzzle (70 files,
745,912 bytes).

**Method.** That sequence: `fathom ingest --json --store <fresh sqlite> --service poly --path <tree>` under
`nice -n 10 /usr/bin/time -l`, a fresh store, `FATHOM_HOME` and `FATHOM_CACHE_ROOT` every run, the builds
interleaved; the whole tree, then guzzle nested one level. Validity as O411's rows: the external volume's TCC
denial invalidated five whole-tree runs (head 1,196 reads failed; head twice and base once a store or a run
directory refused; head once `file_resolved` 28,349 against its twins' 28,383, a silently gated resolve read)
and one guzzle run; each was rerun. The reading is the second valid run of each build. Load average (1 min)
13.4 to 21.0 throughout, sibling waves' gates: CPU and wall are inflated against wave ic's reading at load
5; instructions retired are the load-independent figure.

| Build | bytes | wall | user + sys CPU | CPU ns/byte | wall ns/byte | instructions retired |
|---|--:|--:|--:|--:|--:|--:|
| base 56c18502 | 75,971,918 | 25.17 s | 20.56 s | 270.6 | 331.3 | 236.2 G |
| main 5c891a06 | 75,971,918 | 47.27 s | 40.30 s | 530.5 | 622.2 | 491.7 G |
| head c34559d4 | 75,971,918 | 60.30 s | 46.55 s | 612.7 | 793.7 | 501.5 G |
| head ÷ base | | 2.40× | 2.26× | **2.26×** | 2.40× | 2.12× |

Main reads 1.96× CPU and 2.08× instructions against base, wave ic's 1.92× and 2.06× within the load's
noise. The valid runs agree on instructions (base 236.2 to 238.2 G, main 491.7 to 494.2 G, head 501.3 and
501.5 G); CPU spreads 38.7 to 42.2 s at main and 46.6 to 47.9 s at head.

**The PHP switch's share.** Whole tree, head over main: +9.8 G instructions (+2.0%; +8.6 G, +1.7%, between
the adjacent main and head runs) and +6.3 s CPU (+15.5%), the `extract` pass 31.6 to 35.8 s CPU and
`store_writes` 7.5 to 8.9 s between adjacent runs; 70 more symbols (a `PhpNamespace` per namespaced guzzle
file) and 1,015 more file-resolved edges (27,368 to 28,383: the dialect's qualified `Foo::bar` calls bind
through a `use`, which the CGO walker's calls never did). guzzle alone: base 2.8 G and 0.29 s, main 5.0 G
and 0.54 s, head 9.1 G and 0.83 s (head ÷ main 1.82× instructions, 1.54× CPU; `extract` 0.34 to 0.65 s,
file-resolved edges 266 to 1,281, peak RSS 67 to 143 MB): PHP moved from one native parse to one wasm parse
and the process compiles the PHP grammar module once. The whole tree's +9.8 G is more than guzzle's own
+4.1 G; the rest was not isolated (O459). D1 does not gate on cost; this is disclosed, not a hold.

LB-M5 verdict: FAIL (2.26× CPU per byte: 612.7 vs 270.6 ns over 75,971,918 bytes; wall 2.40×, instructions 2.12×; the PHP switch adds 2.0% instructions and 15.5% CPU under load 13 to 21, O459; main alone reads 1.96×)
