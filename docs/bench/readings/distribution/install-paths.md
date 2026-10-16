DS-M1 reading: go install PASS, go get pkg/client PASS, release archive PASS (v0.13.0, 2026-10-16)

# Install paths from a published tag (DS-M1)

The operational row of `docs/plans/distribution.md`: after the first release
carrying ADR-070, a stranger installs fathom and a Go consumer fetches
`pkg/client` through the public proxy, with no credentials, no clone and no
`replace`. One reading on the owner's Mac (darwin/arm64), go1.26.3, against
tag `v0.13.0` (commit `84290c34a6d3`), within an hour of the tag.

## Environment

Every Go command ran under `env -i` with a fresh `GOPATH`, `GOMODCACHE` and
`GOCACHE` in a new scratch directory, `GOWORK=off`, `GOFLAGS=` (empty) and
`GOENV=off`, so no user `go env -w` setting applied. `go env` read
`GOPROXY=https://proxy.golang.org,direct` and `GOSUMDB=sum.golang.org`, with
`GONOSUMDB` and `GOPRIVATE` empty: the defaults. No git or GitHub credentials
were involved in the Go paths.

## 1. `go install` (PASS)

```sh
go install github.com/geoffmilleraz/fathom/cmd/fathom@v0.13.0   # exit 0, 60 s wall, 39 modules downloaded
$GOPATH/bin/fathom version
# fathom v0.13.0 (revision unknown)
```

`revision unknown` is expected: a binary built by `go install pkg@version` has
no VCS stamp, and `fathom version` prints the release from
`internal/buildinfo`.

## 2. `go get` of `pkg/client` (PASS)

A scratch module `example.com/consumer` with this `main.go`:

```go
package main

import "github.com/geoffmilleraz/fathom/pkg/client"

func main() { _ = client.New }
```

```sh
go mod init example.com/consumer
go get github.com/geoffmilleraz/fathom/pkg/client@v0.13.0   # exit 0
go build -o consumer .                                         # exit 0; 9 s wall for get and build
```

The consumer's `go.mod` requires `github.com/geoffmilleraz/fathom v0.13.0` and
carries no `replace` and no kit module.

## 3. Release archive (PASS)

```sh
gh release download v0.13.0 -R GeoffMillerAZ/fathom -p fathom_0.13.0_darwin_arm64.tar.gz -p checksums.txt
grep ' fathom_0.13.0_darwin_arm64.tar.gz$' checksums.txt | shasum -a 256 -c -
# fathom_0.13.0_darwin_arm64.tar.gz: OK
tar -xzf fathom_0.13.0_darwin_arm64.tar.gz      # fathom, LICENSE.md, README.md
file fathom                                      # Mach-O 64-bit executable arm64
./fathom version
# fathom v0.13.0 @ 84290c34a6d3 [2026-10-16T19:39:08Z]
```

The README's download block, run as written with its install directory moved
to scratch, also resolved `V=0.13.0`, passed the checksum and printed the same
version. The release workflow attached four archives (darwin and linux, arm64
and amd64) and `checksums.txt`.

## What this reading found besides the PASS

- GoReleaser overwrote the prerelease flag the ship step set, so `v0.13.0`
  first published as the repository's Latest release. It was set back to a
  prerelease, and `.goreleaser.yaml` now says `prerelease: true`.
- With every release a prerelease, `/releases/latest` has nothing to redirect
  to, so the install docs' old `releases/latest` lookup produced no version.
  The docs now ask the releases API for the newest release, prereleases
  included.

## Limits

One machine, one architecture (darwin/arm64), one tag. The linux and
darwin/amd64 archives were attached but not run here. The Go paths ran
through the public proxy and checksum database as a stranger would.
