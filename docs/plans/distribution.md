# Distribution: the plan

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/distribution.md` proves it. This file carries the
`## Release binaries` section (item release, wave dist1); item kitmod adds the `go install` rows.

## Release binaries

- [x] GoReleaser builds the darwin targets with cgo and `MACOSX_DEPLOYMENT_TARGET=14.0` (ADR-066), proven by `goreleaser release --snapshot` on a Mac: four archives, `checksums.txt`, each binary the right architecture, `fathom version` correct {id: DS-3, oracle: lexical, path: .goreleaser.yaml, pattern: "MACOSX_DEPLOYMENT_TARGET=14\.0"}
