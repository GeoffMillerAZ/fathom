# Isolated consumer build rehearsal — 2026-09-13

Car Assistant's actual server compiled from two private module artifacts, with no
sibling-source access or network access. Two initially empty module/build cache pairs
produced byte-identical binaries. Generated/served CSS checks exposed two false-success
paths that compilation alone missed. This advances G4; it is not a release, consumer
migration, browser-quality result or G4 completion.

## Captured inputs and isolation

Private workspace: `/tmp/appkit-clean-build.LnJhBh`. Its `prepare.go` captured allowlisted
current Git-visible source, plus CA's ignored required `internal/web/static/tw.css`.
No original consumer files changed; all 84 selected source files and the original HEAD
matched their captured identities after testing. No consumer main or domain route ran.
Runtime data, secrets, deployments and paid asset roots were outside the selection.
One tracked search mock JSON was retained because production code embeds it; its data
was not inspected or used in a user journey. This is not a claim of all-synthetic source.

| Input | Captured HEAD | Selected files | Identity |
|---|---|---:|---|
| app-kit | d22871195401594f1083e2952d938ec5dfd655d3 | 432 | private prerelease named from selected-tree hash |
| agentic-kit root module | 233448fbc06b0825ae8726af7267d7cbe52e0a68 | 1,095 | same method; six nested-module files explicitly excluded |
| Car Assistant | 1f22fe59d795ea7b9f50856aaa4f4b99252f625c | 84 original / 85 staged | dirty working-tree bytes, not HEAD contents |

[Receipt](fixtures/clean-build/receipt.json) records exact candidate versions, ZIP SHA-256,
Go module sums, original file identities, staged changes and final executable metadata.
Tree identity is SHA-256 of Go `encoding/json.Marshal` of the sorted `{path,bytes,sha256}`
record array. Reconstruct staged records by replacing/adding its listed changes; no
records were deleted. This is not a full Git tree hash or an atomic concurrent snapshot.
The app-kit artifact was frozen **before** this assessment's CSS-guide fix; it is not
the final current release candidate. Nothing was tagged or published.

Module ZIP construction used `golang.org/x/mod/zip.CheckFiles/Create`, refused unexpected
omissions and produced identical ZIP bytes twice from the same in-memory snapshot.
That proves same-input archive repeatability, not independent re-snapshot consistency.
The one-off preparer has explicit file/total bounds and rejects source symlinks; it is
not a general-purpose, adversarially hardened archive or release tool.

Both builds used `env -i`, Go 1.26.3 darwin/arm64, `GOENV=off`, `GOWORK=off`,
`GOTOOLCHAIN=local`, `GOVCS=off`, `CGO_ENABLED=0`, separate fresh GOPATH/GOMODCACHE/GOCACHE,
and file-only GOPROXY inputs: the private artifacts plus existing public download
archives. No existing extracted module/build cache supplied compiled packages.
Public `GOSUMDB=sum.golang.org` remained enabled; only the two locally hash-bound private
candidates were exempted. The copied go.sum acquired their four exact checksum lines.

macOS sandbox-exec denied network access and reads below the original fleet root.
All three original go.mod reads failed with `Operation not permitted`; sandboxed access
to the live loopback Gallery failed while the same unrestricted request returned 200.
Go resolved both candidate dependencies under the isolated module cache, with no Replace.
These controls establish local isolation, not clean remote provisioning or another host.

## Commands and observed failures

From the copied consumer root, `run-isolated.sh` supplied the first isolation environment:

```sh
sh ../run-isolated.sh go mod download -json github.com/geoffmilleraz/app-kit@CANDIDATE github.com/geoffmilleraz/agentic-kit@CANDIDATE
sh ../run-isolated.sh go build -mod=readonly -trimpath -buildvcs=false -o ../car-assistant-final ./cmd/car-assistant
sh ../run-isolated-replay.sh go build -mod=readonly -trimpath -buildvcs=false -o ../car-assistant-replay ./cmd/car-assistant
```

`CANDIDATE` denotes each distinct exact receipt version, not a usable literal command.
The first readonly build correctly refused missing private go.sum entries. Explicit
download from the file proxy locked those sums in the copy, then both builds passed.
The 26,495,410-byte final/replay binaries share SHA-256
`a0924e40503c7bf6a9343509489bf20c7747168ba53286b29db6bef750fabd8c`.
They were inspected with `go version -m`, never executed.

An attempted `go run module@version` needed proxy `@v/list` metadata, then an unavailable
public checksum lookup. That path is **not** verified offline. Instead the CLI was built
readonly from the authenticated candidate module directory using its shipped go.sum.
No checksum bypass or runtime installation was used to turn the failure into a pass.

## Generated CSS is a separate build input

The frozen CLI successfully ran `gen -tokens web/css/tokens.cue -css web/css/theme.css
-dtcg '' -go '' -terminal ''`. Generated theme.css matched the original byte-for-byte:
SHA-256 `7e4918a1e17e780b631930d1910f1d8939b153babf9741ebfe6bf9ea5dfe9b87`.
The old MCP recipe's `gen ... -out ...` instead failed: `flag provided but not defined: -out`.

The local Tailwind v4.3.0 macOS ARM64 executable matched the candidate's CUE manifest:
SHA-256 `56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e`.
That verified executable ran directly under isolation; the `appkit css` wrapper itself
was not exercised there because the cleared environment supplied no user cache directory.

CA's 13 kit-source directives pointed at a sibling checkout unavailable in the copy.
Tailwind still exited 0, emitting 33,997 bytes without `.md\:flex` and
`.focus\:not-sr-only`. The independently read dashboard Shell uses those utilities for
its desktop sidebar and keyboard skip link. A CSS exit code cannot prove their presence.
Only the copy's directives were redirected to the exact resolved candidate source.
[Staging patch](fixtures/clean-build/staging.patch) preserves those module/sum/source
deltas; its absolute temporary paths are **evidence, not a portable consumer patch**.
Normal adoption should regenerate an ignored app-local source fragment from resolution.

The corrected compiler run into `internal/web/static/tw.css` replayed identically:
42,654 bytes, SHA-256 `1df389c0861e679eedb9c1c6d5c47fd86a356e3389ba372a0ec45f6bad345921`.
Earlier runs to an external output path retained additional theme declarations; output
location/scan inputs must stay fixed for byte comparisons. No visual equivalence to the
original 49,034-byte CSS is claimed.

The preserved [static-handler test](fixtures/clean-build/css_rehearsal_test.go.txt) requests
only `/static/tw.css` through the real `Server.Handler()` using httptest, no listening
socket or service dependencies. It checks 200, CSS content type, independently rebuilt
artifact identity and the two owned selectors. A test-only stub/Go overlay keeps its
assertion source outside Tailwind scanning; macOS overlay keys use canonical `/private/tmp`.
An earlier `/tmp` overlay selected no tests and was rejected as evidence, then corrected.

| Same focused test | Result |
|---|---|
| Rebuilt CSS | named test ran and passed |
| CSS removed by overlay | failed with HTTP 404; a separate no-CSS server build still compiled |
| Missing-sibling CSS substituted by overlay | failed artifact identity and both selector assertions despite HTTP 200 |

This binds build output to a real static handler, not a browser's computed geometry,
keyboard interaction, accessibility, dark/light appearance or consumer-domain behavior.
The CSS source issue is a delivery hazard, not proven cause of an existing visual defect.

## Durable kit outcome and remaining work

INC-9 records the stale MCP command/guidance. The corrected `get_kit_css_setup` now uses
real explicit output flags, fails before compiling stale tokens, checks selected source
directories, explains app-local motion vendoring and preserves original design/state
distinctions. Its actual CLI generation/negative canaries and exact MCP text/structured
parity run in `task check`; independent review also caught and closed the fail-fast gap.
Full `task check` passed, focused CSS-setup race tests passed, and the final MCP package
rerun passed. Intent checks covered 46 documents, 281 links and 25 ADRs with no failures;
17 inherited oversized ADRs remain documented debt. Independent evidence review
reconstructed both consumer tree hashes and verified all staged files, fixture/ZIP
hashes, reverse patch applicability and matching executable metadata.

G4 still needs exact delivered pins, reviewed consumer contracts and source-fragment
integration, fresh Claude/Codex host runs, real user-task/browser checks, and a reviewed
upgrade reaching a second consumer. The rehearsal artifacts are private, unpublished
and temporary; their receipt is evidence, not an enduring distribution channel.
Original and converted design catalogs remain independent choices, not a forced rewrite.
