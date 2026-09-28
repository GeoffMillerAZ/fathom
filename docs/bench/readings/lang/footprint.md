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
