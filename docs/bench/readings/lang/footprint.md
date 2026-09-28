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
