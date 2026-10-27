### Security

- The go commands fathom runs on its own run with `GOPROXY=off`, `GOSUMDB=off`, `GONOPROXY=,`, `GOTELEMETRY=off` and `GOTOOLCHAIN=local`, and `-mod=mod` is dropped from `GOFLAGS`, so `go list` never rewrites a repository's `go.mod` or `go.sum` (ADR-083).
- `fathom receipt verify` runs under the same offline environment whatever `go_network` says, with `GOWORK=off` and `-mod=readonly`: a `GOPRIVATE` dependency no longer goes to its version control, and a `toolchain` line is never fetched.
