# DEP-M3 Dockerfile dependency edges

**Disposition.** Detect-only. The pinned population has 86 files in three repositories. BuildKit parsed 84 files and all 84 had gaps in native evidence. The candidate marked zero files complete. B1 and B2 are inconclusive, and B3 passed. The product reports direct edges and typed unknowns, with completeness disabled even under explicit trust.

Measured on 2026-11-18 using Go 1.27.4 on darwin/arm64. Candidate commit `e58849393691c69d0ad73655df5290180d8e888e` uses the accepted dependency door at `eb14872f759a12d18721ddc23f5023bda4d94df3`. The published harness can replay the same candidate behind a test-only override. There were no reader correction rounds after the first corpus run.

## Results

| Subject | Files | Native parsed | Full oracle truth | Candidate complete | Complete with truth | Complete without truth | Expected | Complete local edges | Missing | Wrong | Extra | B2 inconclusive bindings |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Docker CLI | 14 | 13 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| Docker Compose | 30 | 29 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| Docker Library Python | 42 | 42 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| Pooled | 86 | 84 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

| Candidate evidence | Docker CLI | Docker Compose | Docker Library Python | Pooled |
| --- | ---: | ---: | ---: | ---: |
| Edges | 22 | 2,606 | 42 | 2,670 |
| `computed-path` | 29 | 12 | 0 | 41 |
| `external-unpinned` | 18 | 39 | 42 | 99 |
| `missing-config` | 36 | 6 | 0 | 42 |
| `parse-error` | 13 | 0 | 0 | 13 |
| `read-limit` | 1 | 1 | 0 | 2 |
| `unresolved-alias` | 1 | 1 | 0 | 2 |
| `unsupported-construct` | 344 | 44 | 42 | 430 |
| Total unknowns | 442 | 103 | 84 | 629 |
| All records | 464 | 2,709 | 126 | 3,299 |
| Unknown share | 95.26% | 3.80% | 66.67% | 19.07% |

All other unknown codes had zero candidate records. The product's detect-only gate also emits `unmeasured-reader`.

| Native coverage and gaps | Docker CLI | Docker Compose | Docker Library Python | Pooled |
| --- | ---: | ---: | ---: | ---: |
| Parsed with partial truth | 13 | 29 | 42 | 84 |
| Observed edges on partial rows | 19,916 | 6,413 | 0 | 26,329 |
| Instruction parsing failed | 1 | 1 | 0 | 2 |
| Base-image configuration | 13 | 28 | 42 | 83 |
| Computed image or platform | 10 | 6 | 0 | 16 |
| Unknown local context | 11 | 6 | 0 | 17 |
| Explicit frontend | 9 | 5 | 0 | 14 |
| External mount | 0 | 3 | 0 | 3 |
| Platform | 4 | 5 | 0 | 9 |
| Stage, image or named context | 1 | 1 | 0 | 2 |

Gap counts overlap. The native adapter built successfully, so tool build loss was zero. A CLI non-instruction document and one Compose input failed native instruction parsing and supplied no truth. Two partial rows supplied the observed edge counts, largely from copying broad context trees. Those counts describe what the native parser and Linux context projection could attest and stay outside the scored denominators.

| Bar | Outcome |
| --- | --- |
| B1 | Inconclusive for every subject and pooled. Zero complete files with oracle coverage. The zero missing count has a zero expected-edge denominator. |
| B2 | Inconclusive for every subject and pooled. Zero complete attributable local edges. Wrong and ambiguous binding counts are both zero. |
| B3 | Pass for all subjects and pooled. Native and candidate output were byte-identical across the repeated reads and independent checkout paths. A second invocation reproduced all evidence bytes. |
| B4 | Completeness was 0/14, 0/30, 0/42 and 0/86 pooled. The extra-edge rate is undefined at 0/0 comparable edges for every subject and pooled. Coverage is below 80%, with soundness unevaluated. |
| B5 | Detect-only. The frozen sample supplies no evidence for admission. |

## Method

The [preregistration](../../../../calibration/oracles/deps-docker/pins.json) freezes three Git commits, the tracked regular-file basename predicate and the SHA-256 of each sorted, NUL-terminated selection. Docker CLI is pinned by the peeled commit. Docker Compose and Docker Library Python use their existing corpus commits. The population includes regular Dockerfile and Containerfile variants in all directories. No selected file drops out of the denominator after parsing. The checkouts contain no symlinks, gitlinks or untracked files. Both paths match their Git blob bytes and the tracked-set receipts before and after measurement.

The manifest's phrase “No generated files” is imprecise. The registered Git predicate excludes untracked worktree output and includes matching files committed by upstream generators. All 42 Docker Library Python files have upstream generation notices and are members of the original hashed selection. The frozen manifest, corpus and bars are preserved.

| Subject | Selected | Tracked files | Tracked bytes | Tracked-set SHA-256 |
| --- | ---: | ---: | ---: | --- |
| Docker CLI | 14 | 3,983 | 34,014,918 | `70fcc9b43aa74242d293dab80209a994ca1117579f43988849b8e1c5ddd914cf` |
| Docker Compose | 30 | 641 | 1,834,341 | `2a41cd4f41c109acc3cc500cbc81f24c4446ebb359a1822f191fa16ed26872c0` |
| Docker Library Python | 42 | 55 | 196,407 | `200029d7d4f0af2b358f4687add9a8a07c4411aee091a71c913cdb162b9c715c` |

The test-only oracle links BuildKit v0.33.1 (commit `8c91502cf280bd70a0c50912ce251c46a8881d9f`), Moby patternmatcher v0.6.1 and BuildKit's pinned fsutil. The adapter uses BuildKit's AST, instruction parser, shell lexer, mount expansion and Linux path normalization. Native ignore parsing, parent matching and wildcard resolution supply context membership. An independent file walk projects surviving directory members to repository targets. A Dockerfile-specific ignore file takes precedence over the repository root's default, even when empty. File-wide configuration edges carry line zero. The repository root is the context for top-level Dockerfiles in this reading. A nested file with a local operand needs a build invocation or a declaration to establish its context.

An oracle row supplies full truth only when native parsing and local binding both finish without gaps. Partial rows preserve observed targets and explain absent coverage. External image metadata and inherited ONBUILD, custom frontends, named contexts, computed operands, remote ADD, mounted secrets and runtime inputs require evidence beyond this read. Known stages account for intra-file references. External image tags do not establish immutable versions. The adapter executes no Dockerfile instructions and makes no build, daemon or registry calls.

The product reader handles repository paths in pure Go and applies the shared result sanitizer. The existing image reference join operates in the ingestion fusion pass and depends on stored stage nodes and a repository-wide image index. This reader uses the standalone dependency door and the supplied repository view.

The shared scorer checks B1 and B2 on complete files with full oracle truth. Expected counts are oracle repository edges for that population. B2 counts wrong bindings and tracks inconclusive operand pairing separately. B4 uses all files for completeness and all reported edge and unknown records for unknown shares. Its extra-edge denominator counts observed repository edges on full-truth rows. Repeated runs include a reversed product batch, and a second checkout path is read independently by the product and oracle. The harness compares their ordered JSON bytes.

## Reproduce

Use Go 1.27.4, Python 3 and Git. Run the following from a clean checkout containing this reading. The two opted-in oracle tests replay the candidate while the ordinary suite checks the published detect-only result. Set `DEP_PACKAGE_TEST_LEASE` to the shared package-test lease script if the machine uses a gate lock, and export that machine's `FATHOM_GATE_LOCK` and `FATHOM_GATE_FLAG`. Modules can be fetched in the tool cache on the first setup. The race tests and oracle can then run with module fetching disabled.

```sh
set -eu
scratch="$PWD/workspace/tmp/dep-m3"
tool="$PWD/workspace/tools/deps/buildkit-0.33.1"
oracle_src="$PWD/calibration/oracles/deps-docker"
mkdir -p "$scratch/tmp" "$scratch/gotmp" "$tool/adapter" "$tool/modcache" "$tool/gocache"
export TMPDIR="$scratch/tmp" GOTMPDIR="$scratch/gotmp"

cp "$oracle_src/main.go" "$oracle_src/go.mod" "$oracle_src/go.sum" "$tool/adapter/"
(
    export GOMODCACHE="$tool/modcache" GOCACHE="$tool/gocache" GOTOOLCHAIN=local GOWORK=off
    go -C "$tool/adapter" build -mod=readonly -trimpath -o "$tool/deps-docker-oracle" .
)
export DEP_DOCKER_ORACLE="$tool/deps-docker-oracle"
"$DEP_DOCKER_ORACLE" -version
export GOPROXY=off

run_race() {
    if [ -n "${DEP_PACKAGE_TEST_LEASE:-}" ]; then
        python3 "$DEP_PACKAGE_TEST_LEASE" go test -race "$@"
    else
        go test -race "$@"
    fi
}

run_race -count=1 ./pkg/lang/deps/...
DEP_DOCKER_NATIVE_FIXTURE=1 run_race -count=1 -v -run '^TestDockerfileNativeFixture$' ./pkg/lang/deps
python3 "$oracle_src/corpus.py" "$scratch/corpus" prepare
python3 "$oracle_src/corpus.py" "$scratch/corpus" snapshot "$scratch/before"
for pass in one two; do
    mkdir "$scratch/$pass"
    DEP_DOCKER_READING=1 DEP_DOCKER_CORPUS="$scratch/corpus" DEP_DOCKER_RECEIPTS="$scratch/$pass" \
        run_race -count=1 -v -run '^TestDockerfileAdmissionReading$' ./pkg/lang/deps >"$scratch/$pass.log"
done
diff -r "$scratch/one" "$scratch/two"
python3 "$oracle_src/corpus.py" "$scratch/corpus" snapshot "$scratch/after"
diff -r "$scratch/before" "$scratch/after"
```

The `prepare` command also accepts `--cache <directory>` containing the three same-named bare Git repositories. The reading refuses an absent binary, mismatched linked pin, absent corpus, dirty checkout or changed selection when explicitly requested. Full manifests, selected paths and product and oracle JSON remain in ignored scratch.

## Fixture proof

- The leased `go test -race -count=1 -run '^TestDockerfile' ./pkg/lang/deps` failed before the first implementation and passed with the reader wired. The final suite also covers the typed resolver request, file-wide line zero and detect-only behavior under explicit trust.
- Mutation of `nativeExcluded` in the isolated scratch copy of `calibration/oracles/deps-docker/main.go` replaced `return matcher.MatchesOrParentMatches(name)` with `return false, nil`. After building that adapter with the same pins and supplying it as `DEP_DOCKER_ORACLE`, the exact synthetic test above returned `FAIL: TestDockerfileNativeFixture` with two missing targets. The original adapter passed. The same fixture removes five product edges without altering native evidence, then changes a native source operand and observes a different native target.
- `TestDockerfileCaps` in `pkg/lang/deps/dockerfile_test.go` supplies 2 MiB plus one byte, 4,097 ignore rules and 4,097 context members. `go test -race -count=1 -run '^TestDockerfileCaps$' ./pkg/lang/deps` exercises the same assertions as the Dockerfile fixture run. Each case refuses completeness and an over-limit context operand contributes no partial edges.
- `go test -race -count=1 -run '^Test(LangClosure|Deps)' ./pkg/lang ./cmd/fathom-lang` passed. The pure-Go package closure read counted 80 non-standard packages for `pkg/lang` and five for `pkg/lang/deps`, with the BuildKit adapter outside both.
