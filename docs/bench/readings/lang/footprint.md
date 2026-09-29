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
`88c961ed`. Every run held `/Volumes/m2ext/tmp/lang-measure.lock`; sibling agents' gates
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
