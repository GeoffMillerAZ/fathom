LL-M1 verdict: PASS

An outside module that imports `pkg/lang` links 79 non-standard packages (bar: at most 80), none of them CUE, and builds to 22.8 MiB stripped (bar: at most 30 MiB). The design predicted it passes only with CUE split out (`pkg/lang/cue`), and it does: before the cuts the same import read 227 packages and 30.0 MiB.

## Reading

| Item | Value |
|---|---|
| Commit | `b120f2dbb` (door, `pkg/lang/cue`, `fathom-lang` and the quality tests landed) |
| Consumer | `pkg/lang/testdata/consumer`: `lang.New`, then `Extract` of one Terraform file, printing the `Result` as JSON. Built in a module outside this repository with a `replace` onto this tree, proxy off, local module cache only (the build `TestOutsideModuleBuilds` runs) |
| Toolchain | go1.26.3, darwin/arm64, cgo at its default (on) |
| Non-standard packages | 79: the consumer's `main`, `pkg/lang`, 9 other fathom packages (`internal/core`, `tenancy`, `memo`, `extractor`, `extractor/tsengine`, `promql`, `langclass`, `langroute`, `langlink`) and 68 third-party. Standard library packages: 145 |
| CUE | 0 packages matching `cuelang`. No `internal/store`, `fusion`, `connector`, `mcp` or `net/http` in the closure |
| Stripped binary (`-ldflags="-s -w"`) | 23,878,130 bytes (22.8 MiB) on darwin/arm64. 24,395,938 bytes (23.3 MiB) on linux/amd64 with `CGO_ENABLED=0` |
| `fathom-lang`, same flags, `CGO_ENABLED=0` | 24,214,178 bytes (23.1 MiB) |

## Against the bar

| Bar | Read | Margin |
|---|---|---|
| At most 80 non-standard packages | 79 with the consumer's own `main`, 78 without | 1 to 2 packages |
| No CUE | 0 | |
| Stripped binary at most 30 MiB | 22.8 MiB | 7.2 MiB |

The package margin is thin. `TestLangClosure` holds `pkg/lang` itself to the same 80, so a new dependency on the door's side fails the gate before this reading moves. Most of the size is the embedded grammars and the wazero runtime.

## Method

```
go build -ldflags='-s -w' -o consumer .        # in the outside module
go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' .
```

with `GOWORK=off GOPROXY=off GOSUMDB=off GOFLAGS=-mod=mod GOTOOLCHAIN=local`. Counts are by `go list -deps`, so build tags and the host decide them; the figures here are the host's.
