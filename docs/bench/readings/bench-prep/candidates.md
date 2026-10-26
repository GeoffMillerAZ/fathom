# Benchmark preparation: measured candidate subjects

Measured on 2026-10-25 (America/Phoenix). Scope: issue #18 subject-preparation lane, the design’s subject criteria and manifest fields, and BX-1 through BX-5. This is a **draft candidate reading**, not BX-3/BX-4 vetting or BX-5 index acceptance. No plan boxes are checked.

Drafts kept: Go 3 kept / 2 refused, Python 5 kept / 0 refused, TypeScript 2 kept / 3 refused. “Kept” means retained for later vetting after this preparation pass; it does not mean all seven design criteria pass.

C4 was deliberately not run. C5 below is an estimate from history, with zero fail-before/pass-after replays and the post-cutoff half pending Q2. The fleet registry is unreachable, so the design’s additional C7 fleet-index exclusion is unknown. C7’s requested checks (not fathom; at most two per organization) pass for all 15 candidates. No agent or paid API was invoked.

## Measurement contract and commands

Full clones are under `workspace/bench/subjects/`; each was detached at its recorded SHA. `git rev-parse --is-shallow-repository` returned false for all 15. Tracked subject files remained unchanged after setup and tests. Logs/builds were written to the job directory; copies of raw receipts, logs and the measurement scripts are under gitignored `workspace/bench/receipts/`, `workspace/bench/logs/` and `workspace/bench/prep-tools/`. No subject source or patch is committed.

Commands below use `$job`, `$subject`, `$venv`, `$pytest_cache`, `$pytest_temp`, `$coverage_dir`, `$resolved_requirements`, `$module_cache` and `$build_cache` as location placeholders. Container mount targets are likewise represented by variables to keep paths portable. The setup and test strings below are the commands executed with those location substitutions; package flags and test selection are preserved.

```sh
git clone "https://github.com/$repository.git" "$subject"
git -C "$subject" rev-parse HEAD
git -C "$subject" symbolic-ref --short HEAD
git -C "$subject" checkout --detach "$sha"
git -C "$subject" rev-parse --is-shallow-repository
git -C "$subject" ls-files -z
git -C "$subject" show "$sha:$file"
git -C "$subject" log "$sha" --since=2024-10-25T00:00:00Z --until=2026-10-26T00:00:00Z --no-merges --format="@@%H%x09%aI%x09%s" --name-only
git -C "$subject" show -s --format=%B "$fix_sha"
curl -L -sS "https://api.github.com/repos/$repository"
python3 "$job/prepare.py"
python3 "$job/static.py"
python3 "$job/issue_links.py"
python3 "$job/c6_python.py"
node "$job/c6_ts.cjs" "$subject"
GOPROXY=off GOFLAGS=-mod=mod GOMODCACHE="$module_cache" "$job/builds/c6-typed" "$subject"
python3 "$job/run_suites.py" "$subject_id"
```

The initial Click clone failed with an HTTP/2 stream cancellation; a full `git -c http.version=HTTP/1.1 clone` succeeded. The other clones used the first command above. Public repository metadata was read from the REST representation of each repository page (source links below), not guessed from popularity. The observer fetched metadata without authentication.

C1: read each root licence blob with `git show SHA:path`; the table names its SPDX identifier and path. C2: select tracked suffixes `.go`, `.py`, or `.ts/.tsx/.mts/.cts`, then sum LF bytes plus one for an unterminated nonempty final line from the pinned blobs. Counts include tests, generated files, declarations, fixtures and examples; no dependency install output is counted. Per-file counts are in the raw `*-lines.tsv` receipts. Bands here are descriptive: 5k–25k, 25k–75k, 75k–150k. No candidate reaches the largest band, so the design’s three-band spread remains unfilled.

C5: count non-merge commits in the stated two-year UTC window touching both any non-test file and a test path. A test path contains a `test`, `tests`, `spec`, `specs` or `__tests__` directory, or ends in `_test.go`, `test_*.py`, `*_test.py`, or `.test/.spec.[cm]?[jt]sx?`. The second count is the subset whose subject matches `fix|bug|regression`, case-insensitively. A separate, narrower count requires a non-test file in the primary language. Documentation/build-only co-changes can inflate the broad count; neither count proves a bug fix. Commits are counted once, irrespective of file count. Counts below 30 are a yield warning; they do not prove that older history cannot supply C5 tasks. Issue references are recovered from commit bodies and linked to the same commits’ test/source paths; PR references are not asserted to be issue closures.

C6: Go uses `go/types` via `golang.org/x/tools/go/packages` v0.52.0, loading `./...` with tests. Python uses `ast`, resolving direct imports, aliases and re-exports; dynamic method dispatch is not counted. TypeScript uses compiler 5.9.3 `getResolvedSignature`, excluding interface method and call signatures. Five or more static call sites across at least three caller packages/modules qualifies a symbol in this draft; counts include test callers and are not counts of distinct caller functions. The Python analysis is static import evidence, not a SCIP truth-graph certification. The formal design C6 graph gate remains for vetting. The Go AST-only exploratory counts were superseded by the typed counts.

The TypeScript oracle was installed separately with `npm install --prefix "$oracle_tools" --save-exact typescript@5.9.3`; subject dependencies were installed only by the setup commands below. Scratch Go measurement programs were formatted and `command gofmt -l` returned no paths. No Go file in this repository changed.

## C3 isolation, platform and timing

Native `sandbox-exec` returned `sandbox_apply: Operation not permitted`. Instead, every warm-up and timed run used an operator-run disposable Linux arm64 container with `--network none --cpus 4`. This is preparation evidence, not a new product container runner or a BX-55 decision. Docker exposed four CPUs. Native macOS and the design’s exact 12-core condition were not measured; no extrapolated timing is reported.

Dependency setup used network-enabled containers. Each successful setup was followed by one warm-up and exactly three fresh test-process runs with networking denied. Go also used `GOFLAGS=-mod=mod GOPROXY=off GOTOOLCHAIN=local GOMAXPROCS=4`, a warmed module cache, and `-count=1` to disable result caching. Python ran from a preinstalled local venv with `PIP_NO_INDEX=1`; TypeScript used `npm_config_offline=true` after `npm ci`. Timings use `time.monotonic()` around `docker run`, include startup overhead, exclude dependency setup and the warm-up, and ran one suite at a time. The harness timeout was 900 seconds; no run reached it.

```sh
docker run --rm --network none --cpus 4 "$image" sh -c 'curl --noproxy "*" -I --connect-timeout 2 --max-time 3 https://1.1.1.1; cat "$route_file"'
docker run --rm --network none --cpus 4 -v "$subject:$container_subject" -v "$work:$container_work" -w "$container_subject" "$image" sh -c "$test_command"
```

For each language image the direct HTTPS probe exited 7 immediately, and the network-route file contained only its header (no external route). Loopback remains available for in-process HTTP tests. Proxy variables were cleared in the test containers. The full executed wrapper also supplied the cache and venv mounts, temporary-directory variable, language-specific offline environment, and process-local Git `safe.directory`; see `run_suites.py` in the raw tools.

Pinned image digests and observed tools:

- `golang@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5` (golang:1.27-bookworm).
- `python@sha256:14a4933e6cef421424f7a66d89a3f4eafcbd0d90e0a822251f298e05132dc735` (python:3.12-bookworm).
- `node@sha256:63538dfe497cc29dfafdda25ada09b46db68a3f4cc205abf1f5d6bf0913f3384` (node:22-bookworm).

Go: `go1.27.3`; Python: `3.12.15`, pip `25.0.1`; Node: `22.23.4`, npm `10.9.9`. Per-subject lockfile hashes and Python resolved dependency versions are recorded in the kept drafts. An upstream lockfile hash identifies a file; it does not imply every install used that lockfile. Python setup used the recorded venv dependencies; Rich’s runtime versions were taken from its Poetry lock.

## Candidates

Each row is one candidate, including refusals. C3 triples are wall seconds followed by exit codes; `setup` means no valid suite measurement was possible. C5 is broad co-change / fix-message subset (primary-language source co-change in parentheses). C6 is the number of qualifying symbols, capped at three in this table; the evidence below names them. Stars are observed counts, with the requested band.

| Language | Candidate / immutable pin | C1 | C2 lines / band | C3 seconds (exit codes) | C5 estimate | C6 | C7 owner/count | Created / stars | Disposition and evidence |
|---|---|---|---|---|---|---|---|---|---|
| Go | [go-chi/chi](https://github.com/go-chi/chi/tree/9f8adb1aabaa8a91180a821111d3f8e668d1c0bf)<br>`9f8adb1aabaa8a91180a821111d3f8e668d1c0bf` | MIT `LICENSE` | 13,152 / 5k-25k | 28.652, 28.98, 29.098 (0,0,0) | 42 / 8 (41) | 3 | go-chi / 1 | 2015-10-15 / 22,997 (over 10k) | Kept draft |
| Go | [pressly/goose](https://github.com/pressly/goose/tree/2d285d2249855c959765409c68b17a6634f0acb4)<br>`2d285d2249855c959765409c68b17a6634f0acb4` | MIT `LICENSE` | 17,226 / 5k-25k | 15.508, 14.895, 15.144 (1,1,1) | 21 / 6 (20) | 3 | pressly / 1 | 2016-02-25 / 11,623 (over 10k) | Refused — C3: full suite did not pass three isolated trials |
| Go | [spf13/cobra](https://github.com/spf13/cobra/tree/adbc8813901bba65827259daa8e22ff94ec1f30e)<br>`adbc8813901bba65827259daa8e22ff94ec1f30e` | Apache-2.0 `LICENSE.txt` | 16,765 / 5k-25k | 3.09, 2.254, 2.981 (0,0,0) | 15 / 3 (13) | 0 | spf13 / 1 | 2013-09-03 / 44,724 (over 10k) | Refused — C6: fewer than three qualifying symbols in measured call graph |
| Go | [stretchr/testify](https://github.com/stretchr/testify/tree/87a7b9d57689f6579db2da795ab8deeab29cb724)<br>`87a7b9d57689f6579db2da795ab8deeab29cb724` | MIT `LICENSE` | 29,801 / 25k-75k | 10.73, 11.392, 10.568 (0,0,0) | 30 / 6 (29) | 3 | stretchr / 1 | 2012-10-16 / 26,233 (over 10k) | Kept draft |
| Go | [urfave/cli](https://github.com/urfave/cli/tree/89ad11673146784b702fb14ce1e29f2b864e51c2)<br>`89ad11673146784b702fb14ce1e29f2b864e51c2` | MIT `LICENSE` | 30,858 / 25k-75k | 4.869, 3.722, 3.598 (0,0,0) | 204 / 135 (196) | 3 | urfave / 1 | 2013-07-13 / 24,306 (over 10k) | Kept draft |
| Python | [encode/starlette](https://github.com/encode/starlette/tree/8aa828ceaff764a2e57932fb42fdf105a47c4c83)<br>`8aa828ceaff764a2e57932fb42fdf105a47c4c83` | BSD-3-Clause `LICENSE.md` | 22,681 / 5k-25k | 15.016, 14.93, 15.263 (0,0,0) | 135 / 10 (130) | 3 | encode / 1 | 2018-06-25 / 12,684 (over 10k) | Kept draft |
| Python | [marshmallow-code/marshmallow](https://github.com/marshmallow-code/marshmallow/tree/b762ea2d59ca52fe5df76b90de14bed7e5ec5607)<br>`b762ea2d59ca52fe5df76b90de14bed7e5ec5607` | MIT `LICENSE` | 15,722 / 5k-25k | 4.554, 4.518, 4.54 (0,0,0) | 68 / 17 (65) | 3 | marshmallow-code / 1 | 2013-11-10 / 7,246 (1k-10k) | Kept draft |
| Python | [pallets/click](https://github.com/pallets/click/tree/2247b35ea1c47c727d7a06e51fa280e12a863ff6)<br>`2247b35ea1c47c727d7a06e51fa280e12a863ff6` | BSD-3-Clause `LICENSE.txt` | 30,378 / 25k-75k | 11.442, 11.386, 11.375 (0,0,0) | 135 / 36 (132) | 3 | pallets / 1 | 2014-04-24 / 17,801 (over 10k) | Kept draft |
| Python | [python-attrs/attrs](https://github.com/python-attrs/attrs/tree/c8bb9bfaa2843e7844b5bb13bed4370b51feec4f)<br>`c8bb9bfaa2843e7844b5bb13bed4370b51feec4f` | MIT `LICENSE` | 21,026 / 5k-25k | 9.023, 9.021, 8.85 (0,0,0) | 48 / 9 (36) | 3 | python-attrs / 1 | 2015-01-27 / 5,861 (1k-10k) | Kept draft |
| Python | [Textualize/rich](https://github.com/Textualize/rich/tree/9d8f9a372cc5916fd4781fec207ced7ddac2f08f)<br>`9d8f9a372cc5916fd4781fec207ced7ddac2f08f` | MIT `LICENSE` | 51,866 / 25k-75k | 8.077, 8.073, 7.887 (0,0,0) | 31 / 12 (31) | 3 | textualize / 1 | 2019-11-10 / 57,641 (over 10k) | Kept draft |
| TypeScript | [immerjs/immer](https://github.com/immerjs/immer/tree/75a290e44ba14bd723c8ef8546b6559c8ef3ae88)<br>`75a290e44ba14bd723c8ef8546b6559c8ef3ae88` | MIT `LICENSE` | 5,422 / 5k-25k | setup 1.412s (exit 1) | 37 / 30 (36) | 3 | immerjs / 1 | 2017-12-29 / 28,994 (over 10k) | Refused — C3: dependency setup failed; no valid timed suite trials |
| TypeScript | [microsoft/tsyringe](https://github.com/microsoft/tsyringe/tree/78222334f49265ea2874fac2c73284345c1124d9)<br>`78222334f49265ea2874fac2c73284345c1124d9` | MIT `LICENSE` | 3,809 / below 5k | setup 1.421s (exit 1) | 3 / 0 (3) | 3 | microsoft / 1 | 2017-11-17 / 6,014 (1k-10k) | Refused — C2: 3809 primary-language lines; below 5,000; C3: dependency setup failed; no valid timed suite trials |
| TypeScript | [reduxjs/redux](https://github.com/reduxjs/redux/tree/61ebf8596654cf3c4b197e426e302fcb3c2bc13e)<br>`61ebf8596654cf3c4b197e426e302fcb3c2bc13e` | MIT `LICENSE.md` | 8,303 / 5k-25k | setup 1.467s (exit 1) | 5 / 3 (4) | 2 | reduxjs / 1 | 2015-05-29 / 61,446 (over 10k) | Refused — C3: dependency setup failed; no valid timed suite trials; C6: fewer than three qualifying symbols in measured call graph |
| TypeScript | [typestack/class-transformer](https://github.com/typestack/class-transformer/tree/a2734a5d154c9b0beb9a10555a04416bc371de06)<br>`a2734a5d154c9b0beb9a10555a04416bc371de06` | MIT `LICENSE` | 8,125 / 5k-25k | 11.4, 11.449, 11.542 (0,0,0) | 0 / 0 (0) | 3 | typestack / 2 | 2016-02-09 / 7,339 (1k-10k) | Kept draft |
| TypeScript | [typestack/class-validator](https://github.com/typestack/class-validator/tree/2e1a5c27dbd65b80e27fe96b49bd6e6641fa3603)<br>`2e1a5c27dbd65b80e27fe96b49bd6e6641fa3603` | MIT `LICENSE` | 14,455 / 5k-25k | 13.223, 13.158, 13.044 (0,0,0) | 6 / 1 (6) | 3 | typestack / 2 | 2015-10-03 / 11,850 (over 10k) | Kept draft |

## Per-candidate commands and fan-in evidence

### go-chi/chi

Pin `9f8adb1aabaa8a91180a821111d3f8e668d1c0bf` (observed ref `master`). [Licence at pin](https://github.com/go-chi/chi/blob/9f8adb1aabaa8a91180a821111d3f8e668d1c0bf/LICENSE), SHA-256 `a2d51b7515acfaff2f7a88688650f2fc4fd99561383e72bba2305e3db59a1647`. [Public metadata](https://api.github.com/repos/go-chi/chi).

C2 extension receipt: `.go`: 89 files / 13152 lines.

Setup: `go version && go mod download`. Test: `go test -count=1 -timeout=14m ./...`.

Warm-up: 40.844s, exit 0. Timed trial log SHA-256: run 1 `0b050eeae1933bb07bff68e68347286a1e0f453b2641896e3ec5ea19d180a7ca`; run 2 `a06623cbbbdd6d715485b61da45f1956da5560cf94dfba1fe1543e85bee99979`; run 3 `89dafd6fd4af13c5356d4fbbdf79536179a31ea2c2861cfcbc49ff2dea981a08`.

C6 method: go/types through x/tools/go/packages v0.52.0 (Tests=true, ./...). Qualifying symbols:

- `github.com/go-chi/chi/v5.NewRouter`: 114 call sites across 3 caller packages/modules; examples: `github.com/go-chi/chi/v5`, `github.com/go-chi/chi/v5/middleware`, `github.com/go-chi/chi/v5/middleware_test`. Sites: `middleware/basic_auth_test.go:17:7`, `middleware/clean_path_test.go:12:7`, `middleware/client_ip_example_test.go:41:7`.
- `github.com/go-chi/chi/v5.*Mux.Get`: 106 call sites across 3 caller packages/modules; examples: `github.com/go-chi/chi/v5`, `github.com/go-chi/chi/v5/middleware`, `github.com/go-chi/chi/v5/middleware_test`. Sites: `middleware/basic_auth_test.go:19:2`, `middleware/clean_path_test.go:14:2`, `middleware/client_ip_example_test.go:74:2`.
- `github.com/go-chi/chi/v5.*Mux.Use`: 58 call sites across 3 caller packages/modules; examples: `github.com/go-chi/chi/v5`, `github.com/go-chi/chi/v5/middleware`, `github.com/go-chi/chi/v5/middleware_test`. Sites: `middleware/basic_auth_test.go:18:2`, `middleware/clean_path_test.go:13:2`, `middleware/client_ip_example_test.go:50:2`.

Test-linked history examples (not replayed):

- [aba86186a274](https://github.com/go-chi/chi/commit/aba86186a27484f3cadc55b8d6448d2049af71bf): explicit closing references #843, touching `mux_test.go` and `mux.go`.
- [38939062c5df](https://github.com/go-chi/chi/commit/38939062c5df4d3e8814aad1a488983112627ced): references (may be PRs) #1156, touching `middleware/compress_test.go` and `middleware/compress.go`.

### pressly/goose

Pin `2d285d2249855c959765409c68b17a6634f0acb4` (observed ref `main`). [Licence at pin](https://github.com/pressly/goose/blob/2d285d2249855c959765409c68b17a6634f0acb4/LICENSE), SHA-256 `f23985556a879c0e68bb4e136ee4b88a550d26fa33db77a65d76b721c93093ef`. [Public metadata](https://api.github.com/repos/pressly/goose).

C2 extension receipt: `.go`: 146 files / 17226 lines.

Setup: `go version && go mod download`. Test: `go test -count=1 -timeout=14m ./...`.

Warm-up: 77.55s, exit 1. Timed trial log SHA-256: run 1 `af7492d7f51c378502dd9cbf02ddc65eaf13f3c8ac03f72c84ab6b1d8a7a177f`; run 2 `24c665b5f910b1d379de908b8faac813c0042f80c85d7dd150b9bf008c066fd0`; run 3 `1a19c9f0e116214ea681c7d7fc6b552762e34733631d19310544c15ebe6741ba`.

C6 method: go/types through x/tools/go/packages v0.52.0 (Tests=true, ./...). Qualifying symbols:

- `github.com/pressly/goose/v3/internal/sqlparser.ParseSQLMigration`: 13 call sites across 3 caller packages/modules; examples: `github.com/pressly/goose/v3`, `github.com/pressly/goose/v3/internal/migrationstats`, `github.com/pressly/goose/v3/internal/sqlparser`. Sites: `internal/migrationstats/migration_sql.go:21:29`, `internal/migrationstats/migration_sql.go:29:33`, `internal/sqlparser/parse.go:54:23`.
- `github.com/pressly/goose/v3.GetDBVersion`: 10 call sites across 3 caller packages/modules; examples: `github.com/pressly/goose/v3/tests/gomigrations/error`, `github.com/pressly/goose/v3/tests/gomigrations/success_test`, `github.com/pressly/goose/v3_test`. Sites: `goose_embed_test.go:42:15`, `goose_embed_test.go:47:14`, `tests/gomigrations/error/gomigrations_error_test.go:33:18`.
- `github.com/pressly/goose/v3.CollectMigrations`: 9 call sites across 3 caller packages/modules; examples: `github.com/pressly/goose/v3`, `github.com/pressly/goose/v3/tests/gomigrations/error`, `github.com/pressly/goose/v3/tests/gomigrations/register_test`. Sites: `down.go:22:21`, `down.go:57:21`, `redo.go:20:21`.

Test-linked history examples (not replayed):

- [ed3decdf4841](https://github.com/pressly/goose/commit/ed3decdf4841f57ecfabe51d863e22b13f975df7): references (may be PRs) #1110, touching `internal/migrationstats/migrationstats_test.go` and `internal/migrationstats/migration_go.go`.
- [6d6e4a4b52fe](https://github.com/pressly/goose/commit/6d6e4a4b52fe15a6ae40540afae81258487d5afe): references (may be PRs) #1124, touching `provider_test.go` and `provider_collect.go`.

C3 refusal evidence: `pkg/dockermanage/dockerpostgres` tests fail because the Docker service socket is absent; supplying that host service would bypass the cell’s isolation. Temporary-directory cleanup failures also occurred. The whole suite failed all three trials; no passing subset was substituted.

### spf13/cobra

Pin `adbc8813901bba65827259daa8e22ff94ec1f30e` (observed ref `main`). [Licence at pin](https://github.com/spf13/cobra/blob/adbc8813901bba65827259daa8e22ff94ec1f30e/LICENSE.txt), SHA-256 `5e3400b93bbb099e83e52bab885e7441750673c21f97988ca3f1240639b63283`. [Public metadata](https://api.github.com/repos/spf13/cobra).

C2 extension receipt: `.go`: 36 files / 16765 lines.

Setup: `go version && go mod download`. Test: `go test -count=1 -timeout=14m ./...`.

Warm-up: 5.584s, exit 0. Timed trial log SHA-256: run 1 `22d20a7e592f86ec621fac13bf7c065a4e199f4f07b5f9b129cfb581f3864094`; run 2 `c2c2ebb59b9041ac3c1ef8208fa105e834234971726e31f23da82c4730c7bf45`; run 3 `a15038769b63acbd0c6308919320ef9d6feae9523bedc42bc113e4a57667e518`.

C6 method: go/types through x/tools/go/packages v0.52.0 (Tests=true, ./...). No qualifying triple was established; strongest measured entries:

- `github.com/spf13/cobra.*Command.Flags`: 197 call sites across 2 caller packages/modules; examples: `github.com/spf13/cobra`, `github.com/spf13/cobra/doc`. Sites: `active_help_test.go:237:2`, `active_help_test.go:302:2`, `bash_completions_test.go:104:2`.
- `github.com/spf13/cobra.*Command.AddCommand`: 157 call sites across 2 caller packages/modules; examples: `github.com/spf13/cobra`, `github.com/spf13/cobra/doc`. Sites: `active_help_test.go:180:2`, `active_help_test.go:277:2`, `active_help_test.go:330:2`.
- `github.com/spf13/cobra.*Command.Name`: 120 call sites across 2 caller packages/modules; examples: `github.com/spf13/cobra`, `github.com/spf13/cobra/doc`. Sites: `active_help.go:50:46`, `active_help_test.go:281:29`, `active_help_test.go:299:29`.

Test-linked history examples (not replayed):

- [746ef0715872](https://github.com/spf13/cobra/commit/746ef07158728502482cea9f880a6f4b21ef29a9): explicit closing references #2257, touching `completions_test.go` and `completions.go`.
- [a97f9fd47b29](https://github.com/spf13/cobra/commit/a97f9fd47b290016526c8af2dac0531fea5cd773): references (may be PRs) #2234, touching `completions_test.go` and `completions.go`.

C6 refusal evidence: the typed `./...` graph, including tests, has at most two caller packages per measured symbol; e.g. `Command.Flags`, `Command.AddCommand`, and `Command.Name`. This is scoped to that loaded program, not external consumers.

### stretchr/testify

Pin `87a7b9d57689f6579db2da795ab8deeab29cb724` (observed ref `master`). [Licence at pin](https://github.com/stretchr/testify/blob/87a7b9d57689f6579db2da795ab8deeab29cb724/LICENSE), SHA-256 `f8e536c1c7b695810427095dc85f5f80d44ff7c10535e8a9486cf393e2599189`. [Public metadata](https://api.github.com/repos/stretchr/testify).

C2 extension receipt: `.go`: 62 files / 29801 lines.

Setup: `go version && go mod download`. Test: `go test -count=1 -timeout=14m ./...`.

Warm-up: 35.504s, exit 0. Timed trial log SHA-256: run 1 `55cf7d9559a022f478c9e5bf66d5b3f35e6c2a4b413e527501bd95a6da168c26`; run 2 `6fc74e5d213dc0fd2c3926dce136356aa8a081216eb5e8ceea02dcca5877c2ca`; run 3 `0b6df26710abc11a8394e3a6e96288ee62096b527ce8a3e1317dfd10b7d4f827`.

C6 method: go/types through x/tools/go/packages v0.52.0 (Tests=true, ./...). Qualifying symbols:

- `github.com/stretchr/testify/assert.Equal`: 281 call sites across 5 caller packages/modules; examples: `github.com/stretchr/testify`, `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/mock`. Sites: `assert/assertion_compare_test.go:474:3`, `assert/assertion_format.go:84:9`, `assert/assertion_forward.go:146:9`.
- `github.com/stretchr/testify/assert.False`: 209 call sites across 5 caller packages/modules; examples: `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/assert/internal/unsafetests_test`, `github.com/stretchr/testify/mock`. Sites: `assert/assertion_compare_test.go:171:3`, `assert/assertion_compare_test.go:217:3`, `assert/assertion_compare_test.go:263:3`.
- `github.com/stretchr/testify/assert.True`: 209 call sites across 5 caller packages/modules; examples: `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/assert/internal/unsafetests_test`, `github.com/stretchr/testify/mock`. Sites: `assert/assertion_compare_test.go:456:3`, `assert/assertion_format.go:829:9`, `assert/assertion_forward.go:1640:9`.

Test-linked history examples (not replayed):

- [15f6826fe461](https://github.com/stretchr/testify/commit/15f6826fe4614cc225c16903d2e9a1c966fb2332): explicit closing references #1800, touching `assert/assertions_test.go` and `assert/assertions.go`.
- [0bf6b946d985](https://github.com/stretchr/testify/commit/0bf6b946d985309f37a4364b0b1f01a92698730e): references (may be PRs) #1212, touching `mock/mock_test.go` and `mock/mock.go`.

### urfave/cli

Pin `89ad11673146784b702fb14ce1e29f2b864e51c2` (observed ref `main`). [Licence at pin](https://github.com/urfave/cli/blob/89ad11673146784b702fb14ce1e29f2b864e51c2/LICENSE), SHA-256 `85695dfe027cabaf97c7fea77c45a01a36c0815e52df9219894b1ba031ee72f0`. [Public metadata](https://api.github.com/repos/urfave/cli).

C2 extension receipt: `.go`: 79 files / 30858 lines.

Setup: `go version && go mod download`. Test: `go test -count=1 -timeout=14m ./...`.

Warm-up: 9.734s, exit 0. Timed trial log SHA-256: run 1 `c253c810009aa0ba461e325ecf65626919c679fac565eee5b1e3a36a4d1a7006`; run 2 `6372c456de424faf376d41630c9f85c3e96d456922a88dd3da6b4354a2e18add`; run 3 `59d05ade4a9af7aab9d5ca21a19d721a4519415951b13b40347770c080a530dc`.

C6 method: go/types through x/tools/go/packages v0.52.0 (Tests=true, ./...). Qualifying symbols:

- `github.com/urfave/cli/v3.*Command.Run`: 486 call sites across 4 caller packages/modules; examples: `github.com/urfave/cli/v3`, `github.com/urfave/cli/v3/examples/example-cli`, `github.com/urfave/cli/v3/scripts`. Sites: `args_test.go:108:9`, `args_test.go:121:10`, `args_test.go:136:9`.
- `github.com/urfave/cli/v3.*Command.String`: 58 call sites across 3 caller packages/modules; examples: `github.com/urfave/cli/v3`, `github.com/urfave/cli/v3/scripts`, `github.com/urfave/cli/v3_test`. Sites: `command_test.go:1136:21`, `command_test.go:1223:23`, `command_test.go:1327:12`.
- `github.com/urfave/cli/v3.*Command.Bool`: 50 call sites across 3 caller packages/modules; examples: `github.com/urfave/cli/v3`, `github.com/urfave/cli/v3/scripts`, `github.com/urfave/cli/v3_test`. Sites: `command_test.go:1397:13`, `command_test.go:1398:12`, `command_test.go:1637:9`.

Test-linked history examples (not replayed):

- [89fb848895a5](https://github.com/urfave/cli/commit/89fb848895a5a2687f70d1c40f0634f9daff3db1): references (may be PRs) #1540, touching `flag_ext_test.go` and `command_setup.go`.
- [bd73abc74526](https://github.com/urfave/cli/commit/bd73abc74526e2f10eac2c64743f8dad85915e46): references (may be PRs) #1540, touching `flag_ext_test.go` and `flag_ext.go`.

### encode/starlette

Pin `8aa828ceaff764a2e57932fb42fdf105a47c4c83` (observed ref `main`). [Licence at pin](https://github.com/encode/starlette/blob/8aa828ceaff764a2e57932fb42fdf105a47c4c83/LICENSE.md), SHA-256 `dcb95677a02240243187e964f941847d19b17821cf99e5afae684fab328c19bf`. [Public metadata](https://api.github.com/repos/encode/starlette).

C2 extension receipt: `.py`: 84 files / 22681 lines.

Setup: `python --version && python -m venv ${venv} && ${venv}/bin/pip install -e . pytest pytest-codspeed anyio[trio] httpx httpx2[zstd] blockbuster opentelemetry-sdk importlib-metadata python-multipart jinja2 itsdangerous pyyaml typing_extensions && ${venv}/bin/pip freeze > ${resolved_requirements}`. Test: `${venv}/bin/python -m pytest -q -o cache_dir=${pytest_cache} --basetemp=${pytest_temp}`.

Warm-up: 18.004s, exit 0. Timed trial log SHA-256: run 1 `cd6875def3a3a30688d2f128cd86dbfde033e95fdc0e1ebed63dbcf2712e145d`; run 2 `89f33732e000a01b332615e62aeec5aad79d9a561a8b1b3835bb5e1f4c046fde`; run 3 `b9ffd2408f39223519ec8a4730c4dd9064e1f1427c86d68c336de7990c120c21`.

C6 method: Python ast import and re-export resolution; direct calls only. Qualifying symbols:

- `starlette.responses.PlainTextResponse`: 103 call sites across 23 caller packages/modules; examples: `benchmarks.routing_benchmark`, `starlette.endpoints`, `starlette.middleware.authentication`. Sites: `benchmarks/routing_benchmark.py:13:12`, `starlette/endpoints.py:56:16`, `starlette/middleware/authentication.py:52:16`.
- `starlette.routing.Route`: 258 call sites across 22 caller packages/modules; examples: `benchmarks.response_benchmark`, `benchmarks.routing_benchmark`, `starlette.routing`. Sites: `benchmarks/response_benchmark.py:44:28`, `benchmarks/response_benchmark.py:45:50`, `benchmarks/routing_benchmark.py:21:17`.
- `starlette.applications.Starlette`: 184 call sites across 19 caller packages/modules; examples: `benchmarks.response_benchmark`, `tests.middleware.test_base`, `tests.middleware.test_body_limit`. Sites: `benchmarks/response_benchmark.py:45:32`, `tests/middleware/test_base.py:1049:20`, `tests/middleware/test_base.py:1096:11`.

Test-linked history examples (not replayed):

- [9eea41ad3c26](https://github.com/encode/starlette/commit/9eea41ad3c26ad21b9bdfe4578c1cdad6c9b9ac2): references (may be PRs) #3130, touching `tests/test_testclient.py` and `starlette/testclient.py`.
- [37309255b4c1](https://github.com/encode/starlette/commit/37309255b4c1b9c381a2d24a1eaf83100984a16a): references (may be PRs) #3193, touching `tests/test_staticfiles.py` and `starlette/staticfiles.py`.

### marshmallow-code/marshmallow

Pin `b762ea2d59ca52fe5df76b90de14bed7e5ec5607` (observed ref `dev`). [Licence at pin](https://github.com/marshmallow-code/marshmallow/blob/b762ea2d59ca52fe5df76b90de14bed7e5ec5607/LICENSE), SHA-256 `906b5d9051e426144cb173ad911667b8ebd05a9c584c2c26c135b32a3ed12001`. [Public metadata](https://api.github.com/repos/marshmallow-code/marshmallow).

C2 extension receipt: `.py`: 38 files / 15722 lines.

Setup: `python --version && python -m venv ${venv} && ${venv}/bin/pip install -e . pytest simplejson && ${venv}/bin/pip freeze > ${resolved_requirements}`. Test: `${venv}/bin/python -m pytest -q -o cache_dir=${pytest_cache}`.

Warm-up: 5.156s, exit 0. Timed trial log SHA-256: run 1 `c1d7c688498b2391cf3033fca6f5fe5d1d4a2445948d92f825db19660d207260`; run 2 `c1d7c688498b2391cf3033fca6f5fe5d1d4a2445948d92f825db19660d207260`; run 3 `fcc3793bfc78504e1d4499872b84387e56028e91ce6d90c7de149c594b4e20fe`.

C6 method: Python ast import and re-export resolution; direct calls only. Qualifying symbols:

- `marshmallow.exceptions.ValidationError`: 104 call sites across 15 caller packages/modules; examples: `examples.flask_example`, `examples.package_json_example`, `marshmallow.fields`. Sites: `examples/flask_example.py:64:15`, `examples/package_json_example.py:24:19`, `performance/benchmark.py:19:15`.
- `marshmallow.fields.String`: 211 call sites across 13 caller packages/modules; examples: `examples.flask_example`, `examples.inflection_example`, `examples.package_json_example`. Sites: `examples/flask_example.py:53:13`, `examples/flask_example.py:54:12`, `examples/flask_example.py:70:15`.
- `marshmallow.fields.Integer`: 158 call sites across 13 caller packages/modules; examples: `examples.flask_example`, `performance.benchmark`, `tests.base`. Sites: `examples/flask_example.py:52:10`, `examples/flask_example.py:68:10`, `performance/benchmark.py:23:10`.

Test-linked history examples (not replayed):

- [902f99c4151d](https://github.com/marshmallow-code/marshmallow/commit/902f99c4151d1f0c4c3b8e8dfbafc7b6a76aeaa6): references (may be PRs) #3016, touching `tests/test_validate.py` and `src/marshmallow/validate.py`.
- [f07eadc87dfa](https://github.com/marshmallow-code/marshmallow/commit/f07eadc87dfac25ed505d5cd9d186920f2682733): references (may be PRs) #2937, touching `tests/test_validate.py` and `src/marshmallow/validate.py`.

### pallets/click

Pin `2247b35ea1c47c727d7a06e51fa280e12a863ff6` (observed ref `main`). [Licence at pin](https://github.com/pallets/click/blob/2247b35ea1c47c727d7a06e51fa280e12a863ff6/LICENSE.txt), SHA-256 `9a8ad106a394e853bfe21f42f4e72d592819a22805d991b5f3275029292b658d`. [Public metadata](https://api.github.com/repos/pallets/click).

C2 extension receipt: `.py`: 91 files / 30378 lines.

Setup: `apt-get update && (cd ${work} && apt-get download less) && mkdir -p ${work}/os-tools && dpkg-deb --fsys-tarfile ${work}/less_*.deb | tar --no-same-owner -x -C ${work}/os-tools && ${work}/os-tools/usr/bin/less --version && python --version && python -m venv ${venv} && ${venv}/bin/pip install -e . pytest && ${venv}/bin/pip freeze > ${resolved_requirements}`. Test: `PATH=${work}/os-tools/usr/bin:$PATH ${venv}/bin/python -m pytest -q -o cache_dir=${pytest_cache} --basetemp=${pytest_temp}`.

Warm-up: 12.354s, exit 0. Timed trial log SHA-256: run 1 `73a32d9736df3fe2fd3413fd3d67ac1a1d70af29e07efc96b5b74853c7594ba9`; run 2 `82f4807ac51d69740a98c13dc73a23088e6e3f379f6dd8e485566165f54366b4`; run 3 `3f90cdf820a5e1b958bcdf9e4e864fea431ca2ccfb7f9121ac8f7ff745b65c86`.

C6 method: Python ast import and re-export resolution; direct calls only. Qualifying symbols:

- `click.decorators.command`: 348 call sites across 33 caller packages/modules; examples: `click.decorators`, `examples.aliases.aliases`, `examples.colors.colors`. Sites: `examples/aliases/aliases.py:89:2`, `examples/colors/colors.py:24:2`, `examples/complex/complex/cli.py:48:2`.
- `click.decorators.option`: 280 call sites across 25 caller packages/modules; examples: `click.decorators`, `examples.aliases.aliases`, `examples.completion.completion`. Sites: `examples/aliases/aliases.py:136:2`, `examples/aliases/aliases.py:90:2`, `examples/completion/completion.py:13:2`.
- `click.decorators.argument`: 145 call sites across 21 caller packages/modules; examples: `examples.aliases.aliases`, `examples.completion.completion`, `examples.complex.complex.commands.cmd_init`. Sites: `examples/aliases/aliases.py:134:2`, `examples/aliases/aliases.py:135:2`, `examples/completion/completion.py:25:2`.

Test-linked history examples (not replayed):

- [fc518e41218b](https://github.com/pallets/click/commit/fc518e41218b447e875fe27244ae692c269bcb77): explicit closing references #3802, touching `tests/test_abort_interrupt.py` and `src/click/core.py`.
- [bec59289d8cf](https://github.com/pallets/click/commit/bec59289d8cf9b9b4010642b2fee483e5f8eeefc): references (may be PRs) #1884, #2331, #3125, touching `tests/test_basic.py` and `src/click/decorators.py`.

### python-attrs/attrs

Pin `c8bb9bfaa2843e7844b5bb13bed4370b51feec4f` (observed ref `main`). [Licence at pin](https://github.com/python-attrs/attrs/blob/c8bb9bfaa2843e7844b5bb13bed4370b51feec4f/LICENSE), SHA-256 `882115c95dfc2af1eeb6714f8ec6d5cbcabf667caff8729f42420da63f714e9f`. [Public metadata](https://api.github.com/repos/python-attrs/attrs).

C2 extension receipt: `.py`: 56 files / 21026 lines.

Setup: `python --version && python -m venv ${venv} && ${venv}/bin/pip install -e . pytest hypothesis pympler cloudpickle pytest-xdist && ${venv}/bin/pip freeze > ${resolved_requirements}`. Test: `${venv}/bin/python -m pytest -q -o cache_dir=${pytest_cache}`.

Warm-up: 10.617s, exit 0. Timed trial log SHA-256: run 1 `d23184af3286dd3dced4bd07f188cb525fa04cc402a70834c50ea1131333684a`; run 2 `3245a1ba970b59cb51028933bdb1b82a18ec341cfcf63bd7d7f158a9dab2336c`; run 3 `a69b12cbca9b9742afbd52eb54445bbdfdc494613eee10a2d0a8e3682d412c55`.

C6 method: Python ast import and re-export resolution; direct calls only. Qualifying symbols:

- `attr._make.attrib`: 522 call sites across 17 caller packages/modules; examples: `attr._make`, `attr._next_gen`, `attr._version_info`. Sites: `src/attr/_make.py:3314:24`, `src/attr/_make.py:3374:19`, `src/attr/_make.py:419:21`.
- `attr._make.fields`: 137 call sites across 17 caller packages/modules; examples: `attr._funcs`, `attr._make`, `tests.test_annotations`. Sites: `src/attr/_funcs.py:272:13`, `src/attr/_funcs.py:415:13`, `src/attr/_funcs.py:486:22`.
- `attr._make.attrs`: 244 call sites across 14 caller packages/modules; examples: `attr._make`, `attr._next_gen`, `attr._version_info`. Sites: `src/attr/_make.py:3356:11`, `src/attr/_make.py:3368:2`, `src/attr/_next_gen.py:363:16`.

Test-linked history examples (not replayed):

- [97f8d175656b](https://github.com/python-attrs/attrs/commit/97f8d175656bc03c373a1c9038048a4d312c307c): references (may be PRs) #1593, touching `tests/test_annotations.py` and `src/attr/_make.py`.
- [af9c510912ce](https://github.com/python-attrs/attrs/commit/af9c510912ce604a94896cc35c27368d5baf6ae6): references (may be PRs) #1513, touching `tests/test_validators.py` and `src/attr/validators.py`.

### Textualize/rich

Pin `9d8f9a372cc5916fd4781fec207ced7ddac2f08f` (observed ref `main`). [Licence at pin](https://github.com/Textualize/rich/blob/9d8f9a372cc5916fd4781fec207ced7ddac2f08f/LICENSE), SHA-256 `deed7c17a4318158190a3ea239cc879a5a50271cebb98ae7025f48fbe58dca15`. [Public metadata](https://api.github.com/repos/Textualize/rich).

C2 extension receipt: `.py`: 213 files / 51866 lines.

Setup: `python --version && python -m venv ${venv} && ${venv}/bin/pip install -e . pytest==7.4.4 pytest-cov==3.0.0 attrs==21.4.0 pygments==2.19.2 markdown-it-py==3.0.0 && ${venv}/bin/pip freeze > ${resolved_requirements}`. Test: `${venv}/bin/python -m pytest -q -o cache_dir=${pytest_cache} --basetemp=${pytest_temp}`.

Warm-up: 9.757s, exit 0. Timed trial log SHA-256: run 1 `4ac1c351265f7472182b532e076157b560e3bfa923525ce9ab43f3b76a591855`; run 2 `4ac1c351265f7472182b532e076157b560e3bfa923525ce9ab43f3b76a591855`; run 3 `97eb8a35015bb0c92f6419888a55cf2c7d0bd1dd656771aaa75c1f84f7d9aca1`.

C6 method: Python ast import and re-export resolution; direct calls only. Qualifying symbols:

- `rich.console.Console`: 362 call sites across 75 caller packages/modules; examples: `benchmarks.benchmarks`, `examples.columns`, `examples.exception`. Sites: `benchmarks/benchmarks.py:106:19`, `benchmarks/benchmarks.py:131:24`, `benchmarks/benchmarks.py:150:24`.
- `rich.text.Text`: 302 call sites across 42 caller packages/modules; examples: `benchmarks.benchmarks`, `examples.layout`, `examples.listdir`. Sites: `benchmarks/benchmarks.py:26:9`, `benchmarks/benchmarks.py:29:9`, `benchmarks/benchmarks.py:35:9`.
- `rich.segment.Segment`: 299 call sites across 23 caller packages/modules; examples: `benchmarks.benchmarks`, `rich.__main__`, `rich.align`. Sites: `benchmarks/benchmarks.py:210:13`, `benchmarks/benchmarks.py:211:13`, `benchmarks/benchmarks.py:212:13`.

Test-linked history examples (not replayed):

- [95fe8ff59249](https://github.com/Textualize/rich/commit/95fe8ff59249d9ac0534a82af7c746826a5999f4): explicit closing references #3881, touching `tests/test_prompt.py` and `rich/prompt.py`.
- [30e5ed61a606](https://github.com/Textualize/rich/commit/30e5ed61a6064220fe2ff40f4713463488ee6d07): explicit closing references #3569, touching `tests/test_panel.py` and `rich/panel.py`.

### immerjs/immer

Pin `75a290e44ba14bd723c8ef8546b6559c8ef3ae88` (observed ref `main`). [Licence at pin](https://github.com/immerjs/immer/blob/75a290e44ba14bd723c8ef8546b6559c8ef3ae88/LICENSE), SHA-256 `99cf22f6960a6fe228ec84ea9aefb0d75b27999ece83a3a9801e1c2081fa270b`. [Public metadata](https://api.github.com/repos/immerjs/immer).

C2 extension receipt: `.ts`: 29 files / 5422 lines; `.tsx`: 0 files / 0 lines; `.mts`: 0 files / 0 lines; `.cts`: 0 files / 0 lines.

Setup: `node --version && npm --version && npm ci`. Test: `npm run test:ci -- --coverageDirectory=${coverage_dir}`.

No warm-up or valid timed suite runs: dependency setup failed. The pinned tree has no `package-lock.json` or `npm-shrinkwrap.json`; `npm ci` returned EUSAGE.

C6 method: TypeScript 5.9.3 getResolvedSignature; interfaces and call signatures excluded. Qualifying symbols:

- `src/utils/errors.ts#die`: 20 call sites across 8 caller packages/modules; examples: `src/core/current.ts`, `src/core/finalize.ts`, `src/core/immerClass.ts`. Sites: `src/core/current.ts:18:23`, `src/core/finalize.ts:35:4`, `src/core/immerClass.ts:100:66`.
- `src/utils/common.ts#isDraftable`: 13 call sites across 7 caller packages/modules; examples: `src/core/current.ts`, `src/core/finalize.ts`, `src/core/immerClass.ts`. Sites: `src/core/current.ts:23:7`, `src/core/finalize.ts:238:13`, `src/core/finalize.ts:290:4`.
- `src/utils/common.ts#isDraft`: 11 call sites across 5 caller packages/modules; examples: `src/core/current.ts`, `src/core/finalize.ts`, `src/core/immerClass.ts`. Sites: `src/core/current.ts:18:7`, `src/core/finalize.ts:148:8`, `src/core/finalize.ts:224:6`.

Test-linked history examples (not replayed):

- [13f88d850b3b](https://github.com/immerjs/immer/commit/13f88d850b3ba6dc95dfab4a6cccbdc6550be08c): references (may be PRs) #1345, touching `__tests__/patch.js` and `src/plugins/patches.ts`.
- [b671cdac2af2](https://github.com/immerjs/immer/commit/b671cdac2af23e486edd38c040065a228eb0e651): references (may be PRs) #1341, touching `__tests__/array-methods-filter-length.js` and `src/plugins/arrayMethods.ts`.

### microsoft/tsyringe

Pin `78222334f49265ea2874fac2c73284345c1124d9` (observed ref `master`). [Licence at pin](https://github.com/microsoft/tsyringe/blob/78222334f49265ea2874fac2c73284345c1124d9/LICENSE), SHA-256 `27ebda9d51f0a56b7e281ccd8230a27236dcb51c05f64b07869ecf6e965d68b0`. [Public metadata](https://api.github.com/repos/microsoft/tsyringe).

C2 extension receipt: `.ts`: 62 files / 3809 lines; `.tsx`: 0 files / 0 lines; `.mts`: 0 files / 0 lines; `.cts`: 0 files / 0 lines.

Setup: `node --version && npm --version && npm ci`. Test: `npm run test:ci -- --coverageDirectory=${coverage_dir}`.

No warm-up or valid timed suite runs: dependency setup failed. The pinned tree has no `package-lock.json` or `npm-shrinkwrap.json`; `npm ci` returned EUSAGE.

C6 method: TypeScript 5.9.3 getResolvedSignature; interfaces and call signatures excluded. Qualifying symbols:

- `src/decorators/injectable.ts#injectable`: 68 call sites across 12 caller packages/modules; examples: `src/__tests__/auto-injectable.test.ts`, `src/__tests__/errors.test.ts`, `src/__tests__/fixtures/01-test-case-A01-injects-B01.ts`. Sites: `src/__tests__/auto-injectable.test.ts:102:4`, `src/__tests__/auto-injectable.test.ts:150:4`, `src/__tests__/errors.test.ts:12:4`.
- `src/decorators/inject.ts#inject`: 9 call sites across 6 caller packages/modules; examples: `src/__tests__/errors.test.ts`, `src/__tests__/fixtures/02-test-case-A02-lazy-injects-B02.ts`, `src/__tests__/fixtures/02-test-case-B02-lazy-injects-A02.ts`. Sites: `src/__tests__/errors.test.ts:42:18`, `src/__tests__/fixtures/02-test-case-A02-lazy-injects-B02.ts:7:16`, `src/__tests__/fixtures/02-test-case-B02-lazy-injects-A02.ts:11:16`.
- `src/lazy-helpers.ts#delay`: 5 call sites across 5 caller packages/modules; examples: `src/__tests__/fixtures/02-test-case-A02-lazy-injects-B02.ts`, `src/__tests__/fixtures/02-test-case-B02-lazy-injects-A02.ts`, `src/__tests__/fixtures/03-test-case-A03-lazy-injects-B03-interface.ts`. Sites: `src/__tests__/fixtures/02-test-case-A02-lazy-injects-B02.ts:7:23`, `src/__tests__/fixtures/02-test-case-B02-lazy-injects-A02.ts:11:23`, `src/__tests__/fixtures/03-test-case-A03-lazy-injects-B03-interface.ts:13:15`.

No issue-referencing test/source fix-message example found in the measured two-year window.

### reduxjs/redux

Pin `61ebf8596654cf3c4b197e426e302fcb3c2bc13e` (observed ref `master`). [Licence at pin](https://github.com/reduxjs/redux/blob/61ebf8596654cf3c4b197e426e302fcb3c2bc13e/LICENSE.md), SHA-256 `f2da73c752c6b87624755edacf927cbd915fa76555d383e74494bad4a5155ab3`. [Public metadata](https://api.github.com/repos/reduxjs/redux).

C2 extension receipt: `.ts`: 56 files / 6886 lines; `.tsx`: 15 files / 802 lines; `.mts`: 6 files / 615 lines; `.cts`: 0 files / 0 lines.

Setup: `node --version && npm --version && npm ci`. Test: `npm run test:ci -- --coverageDirectory=${coverage_dir}`.

No warm-up or valid timed suite runs: dependency setup failed. The pinned tree has no `package-lock.json` or `npm-shrinkwrap.json`; `npm ci` returned EUSAGE.

C6 method: TypeScript 5.9.3 getResolvedSignature; interfaces and call signatures excluded. No qualifying triple was established; strongest measured entries:

- `test/helpers/actionCreators.ts#addTodo`: 21 call sites across 4 caller packages/modules; examples: `test/applyMiddleware.spec.ts`, `test/bindActionCreators.spec.ts`, `test/createStore.spec.ts`. Sites: `test/applyMiddleware.spec.ts:11:22`, `test/applyMiddleware.spec.ts:33:20`, `test/applyMiddleware.spec.ts:34:20`.
- `src/utils/kindOf.ts#kindOf`: 9 call sites across 3 caller packages/modules; examples: `src/bindActionCreators.ts`, `src/combineReducers.ts`, `src/createStore.ts`. Sites: `src/bindActionCreators.ts:68:86`, `src/combineReducers.ts:35:54`, `src/createStore.ts:124:72`.
- `src/utils/isPlainObject.ts#isPlainObject`: 3 call sites across 3 caller packages/modules; examples: `src/combineReducers.ts`, `src/createStore.ts`, `src/utils/isAction.ts`. Sites: `src/combineReducers.ts:33:8`, `src/createStore.ts:271:10`, `src/utils/isAction.ts:6:5`.

Test-linked history examples (not replayed):

- [9b21de4947de](https://github.com/reduxjs/redux/commit/9b21de4947de93dcc7a8234b243aa263b9e8fd22): explicit closing references #4890, touching `test/createStore.spec.ts` and `src/createStore.ts`.
- [28d433b76273](https://github.com/reduxjs/redux/commit/28d433b762735c870a073e3a3c8a27a33ca8df6f): references (may be PRs) #4888, touching `test/createStore.spec.ts` and `src/createStore.ts`.

### typestack/class-transformer

Pin `a2734a5d154c9b0beb9a10555a04416bc371de06` (observed ref `develop`). [Licence at pin](https://github.com/typestack/class-transformer/blob/a2734a5d154c9b0beb9a10555a04416bc371de06/LICENSE), SHA-256 `40fb2182a242493ce32f6211c3400f2dcf6787c9c68cd261bfb61461d69780ef`. [Public metadata](https://api.github.com/repos/typestack/class-transformer).

C2 extension receipt: `.ts`: 69 files / 8125 lines; `.tsx`: 0 files / 0 lines; `.mts`: 0 files / 0 lines; `.cts`: 0 files / 0 lines.

Setup: `node --version && npm --version && npm ci --legacy-peer-deps --ignore-scripts`. Test: `npm run test:ci -- --coverageDirectory=${coverage_dir}`.

Warm-up: 13.291s, exit 0. Timed trial log SHA-256: run 1 `25302cf2cedef883765caeee3a4e2fb943b8f6acb3006d93b2bf2c19bd4bf4f8`; run 2 `cdd8ea59156bc3d487427088a97bd2ae5de1f3230ee9ce9bb1a49c150006bd5f`; run 3 `4826a3201ec1ed3f2eba039a6ed865beeb7aae1ab612d0263fd05b5f0b308207`.

C6 method: TypeScript 5.9.3 getResolvedSignature; interfaces and call signatures excluded. Qualifying symbols:

- `src/decorators/type.decorator.ts#Type`: 79 call sites across 17 caller packages/modules; examples: `sample/sample1-simple-usage/Album.ts`, `sample/sample1-simple-usage/Photo.ts`, `sample/sample1-simple-usage/User.ts`. Sites: `sample/sample1-simple-usage/Album.ts:10:4`, `sample/sample1-simple-usage/Photo.ts:14:4`, `sample/sample1-simple-usage/Photo.ts:17:4`.
- `src/index.ts#plainToInstance`: 93 call sites across 12 caller packages/modules; examples: `test/functional/basic-functionality.spec.ts`, `test/functional/circular-reference-problem.spec.ts`, `test/functional/custom-transform.spec.ts`. Sites: `test/functional/basic-functionality.spec.ts:1085:30`, `test/functional/basic-functionality.spec.ts:111:29`, `test/functional/basic-functionality.spec.ts:1159:30`.
- `src/decorators/exclude.decorator.ts#Exclude`: 53 call sites across 12 caller packages/modules; examples: `sample/sample1-simple-usage/Album.ts`, `sample/sample2-iheritance/Album.ts`, `sample/sample2-iheritance/Authorable.ts`. Sites: `sample/sample1-simple-usage/Album.ts:7:4`, `sample/sample2-iheritance/Album.ts:8:4`, `sample/sample2-iheritance/Authorable.ts:7:4`.

No issue-referencing test/source fix-message example found in the measured two-year window.

### typestack/class-validator

Pin `2e1a5c27dbd65b80e27fe96b49bd6e6641fa3603` (observed ref `develop`). [Licence at pin](https://github.com/typestack/class-validator/blob/2e1a5c27dbd65b80e27fe96b49bd6e6641fa3603/LICENSE), SHA-256 `ef7c4bc854ad6f0ee0ea4bdae8dee790ed65edd22713d2806d423b8e0727328a`. [Public metadata](https://api.github.com/repos/typestack/class-validator).

C2 extension receipt: `.ts`: 176 files / 14455 lines; `.tsx`: 0 files / 0 lines; `.mts`: 0 files / 0 lines; `.cts`: 0 files / 0 lines.

Setup: `node --version && npm --version && npm ci`. Test: `npm run test:ci -- --coverageDirectory=${coverage_dir}`.

Warm-up: 15.393s, exit 0. Timed trial log SHA-256: run 1 `e891f5540e6bd4d38ae36c1ec1463044a0408b0c6a4d358bb5c7126a53df657e`; run 2 `45319904cedc9a094d459e4d81f8f4fc26998fd6aab3b93698599f0c1302ea91`; run 3 `e2e1d1a79dd2b43b7f010e6fd9f9229f5ac7279e07099d71b795e0266c11bc5b`.

C6 method: TypeScript 5.9.3 getResolvedSignature; interfaces and call signatures excluded. Qualifying symbols:

- `src/decorator/common/ValidateBy.ts#buildMessage`: 107 call sites across 107 caller packages/modules; examples: `src/decorator/array/ArrayContains.ts`, `src/decorator/array/ArrayMaxSize.ts`, `src/decorator/array/ArrayMinSize.ts`. Sites: `src/decorator/array/ArrayContains.ts:27:25`, `src/decorator/array/ArrayMaxSize.ts:25:25`, `src/decorator/array/ArrayMinSize.ts:25:25`.
- `src/decorator/common/ValidateBy.ts#ValidateBy`: 106 call sites across 106 caller packages/modules; examples: `src/decorator/array/ArrayContains.ts`, `src/decorator/array/ArrayMaxSize.ts`, `src/decorator/array/ArrayMinSize.ts`. Sites: `src/decorator/array/ArrayContains.ts:21:10`, `src/decorator/array/ArrayMaxSize.ts:19:10`, `src/decorator/array/ArrayMinSize.ts:19:10`.
- `src/validation/Validator.ts#validate`: 143 call sites across 17 caller packages/modules; examples: `sample/sample2-using-groups/app.ts`, `sample/sample3-nested-objects/app.ts`, `sample/sample4-custom-validator/app.ts`. Sites: `sample/sample2-using-groups/app.ts:14:1`, `sample/sample2-using-groups/app.ts:18:1`, `sample/sample2-using-groups/app.ts:30:1`.

Test-linked history examples (not replayed):

- [0c87bebe8b29](https://github.com/typestack/class-validator/commit/0c87bebe8b290a2527b8390c4a4877d7412bc2d9): references (may be PRs) #2549, touching `test/functional/validation-functions-and-decorators.spec.ts` and `src/decorator/string/IsBase64.ts`.

## Initial attempts and preparation corrections

These failures remain in the raw `*-initial-*` receipts. They are not silently removed or counted as evidence of green suites. Corrections changed the setup/environment; they did not patch the subjects or rerun a failed trial unchanged until it passed.

- **urfave/cli**: 6.09s/exit 1; 2.646s/exit 1; 3.094s/exit 1. The nested `go build` test failed Git VCS stamping because the mounted checkout was owned by a different numeric account. The corrected wrapper passed process-local GIT_CONFIG_COUNT/GIT_CONFIG_KEY_0/GIT_CONFIG_VALUE_0 for safe.directory. No global Git configuration changed.

- **encode/starlette**: 32.164s/exit 1; 32.144s/exit 1; 32.023s/exit 1. pytest rejected ownership of its default temporary directory on the shared filesystem. The final command supplies an explicit --basetemp.

- **pallets/click**: 18.051s/exit 1; 17.006s/exit 1; 17.197s/exit 1. pytest rejected ownership of its default temporary directory on the shared filesystem. A second preparation attempt supplied --basetemp and exposed a missing external pager. The final setup installs the Debian less archive into the job-owned tool directory, adds it to PATH, and supplies --basetemp. Upstream pytest defaults deselect 31,000 stress cases; that default was preserved.

- **Textualize/rich**: 7.921s/exit 1; 7.908s/exit 1; 7.84s/exit 1. Unconstrained allowed dependencies installed Pygments 2.21.0 and produced eight rendering/syntax snapshot failures. The corrected setup used the pin’s Poetry lock versions: Pygments 2.19.2, markdown-it-py 3.0.0, attrs 21.4.0, pytest 7.4.4 and pytest-cov 3.0.0.

- **typestack/class-transformer**: setup 2.589s/exit 1. npm 10 rejected eslint-plugin-jest’s optional peer dependency on typescript-eslint 5 against the lock’s version 4 (ERESOLVE). The final setup uses npm ci --legacy-peer-deps --ignore-scripts, preserving the lock and the upstream CI’s script suppression. This compatibility flag is explicit in the draft.

Click’s intermediate --basetemp attempt still failed 24 pager cases because the base image lacked less: 11.386s/exit 1, 11.418s/exit 1, 11.366s/exit 1. Its logs and receipts remain under the tempfix prefix. The final system-package archive and checksum are in the Click draft.

Click’s first less extraction returned exit 2 because archive extraction attempted to change ownership on the shared mount. The final extraction uses tar --no-same-owner. This was a setup failure, with no timed trials; the systemprep receipts are retained.

## Limits and next evidence

The drafted files contain the design’s repo-entry fields and `bench` block. `schema/bench` and `#Subject` are absent at this revision, so CUE validation and the future loader were not claimed. Drafts use null for the unmeasured L0 reading and `fresh_after`, `unassigned` for the split, and an empty task-source list. No held-out split or bank provenance was invented.

Before admission: run the implemented vetter; C4 accuracy and refusal rate; C5 fail-before/pass-after replay and post-cutoff count after Q2; formal C6 truth graphs; C7 live fleet exclusion; native macOS and the specified CPU condition. C4 refusal rate is unmeasured, not zero. Python-version skips and upstream default test selection remain part of each command’s scope; a green run is not every optional platform/type-check matrix.

Slowest final timed suite attempt: go-chi/chi, 29.098s, exit 0. Slowest kept draft: go-chi/chi, 29.098s. No final run exceeded 15 minutes. Across all initial and final environments, the slowest timed attempt was Starlette’s initial run 1 at 32.164s (exit 1, temporary-directory ownership errors).

Validation: see the verification receipt appended below. No public runtime behavior changed, so no release fragment was added.

## Verification receipt

2026-10-25:

- Ruby `YAML.safe_load` parsed all 10 drafts; each filename equals its subject id.
- `python3 "$job/verify.py"` passed: pinned SHAs and licence hashes, measured line counts, upstream lock hashes, 30 green isolated trials and their log hashes, three named fan-in examples per draft, organization cap, null/unassigned pending fields, 15 candidate rows, portable paths and whitespace.
- All 15 clones report full history and no tracked-file difference from the measured SHA after preparation.
- `task fmt-check` passed; `command gofmt -l` on the scratch Go measurement programs returned no paths. No repository Go source changed.
- `GOMODCACHE="$module_cache" task validate` was attempted and blocked before any gate check: the sandbox denied opening the shared machine-wide gate lease. The lease was not bypassed and no global configuration changed. Full validation is **unverified**, not green.
- The absent `schema/bench` package prevents CUE `#Subject` validation; these are explicitly drafts for that future contract. No C4 run, plan verification or claimed plan completion was substituted.
