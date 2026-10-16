# Distribution: the plan (owner decision Q1 of 2026-10-16, ADR-070)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/distribution.md` proves it. Decision: ADR-070 (fathom is one
self-contained module; the kit mirrors are packages of it by a reversible import-path rewrite), amending ADR-019.
Owner and maintainer guide: `docs/distribution.md`.

Goal: a stranger runs `go install github.com/geoffmilleraz/fathom/cmd/fathom@<tag>` and a Go consumer runs
`go get github.com/geoffmilleraz/fathom/pkg/client@<tag>`, with no credentials, no clone and no `replace`.

## Module

- [x] The module is self-contained: go.mod carries no replace or exclude and requires no module under fathom's owner path or a mirror's upstream path, no kit mirror holds its own go.mod, and no Go or templ file outside testdata and nested modules imports a kit by its upstream path; `TestRepositoryIsSelfContained` holds it in the gate and `go run ./cmd/kitrewrite -check` prints it {id: DS-1, symbol: Check, package: internal/kitrewrite}
- [x] The rewrite round trip: forward then `-reverse` is byte-identical on a fixture tree (both mirrors and the consumer's own files, and one kit copy alone), idempotent, and never touches testdata, a nested module, a dot or scratch directory, a non-import string or a lookalike path; `task sync-kit` and `task vendor-check` run it {id: DS-2, symbol: RewriteRepo, package: internal/kitrewrite}
- [ ] (op) After the first release carrying ADR-070: from an empty GOPATH and module cache with no credentials, `go install github.com/geoffmilleraz/fathom/cmd/fathom@<tag>` and `go get github.com/geoffmilleraz/fathom/pkg/client@<tag>` in a scratch module both succeed through the public proxy, recorded with the tag, Go version and wall time (the release item records it) {id: DS-M1, oracle: lexical, path: docs/bench/readings/distribution/install-paths.md, pattern: "^DS-M1 reading: go install .* PASS"}

## Release binaries
