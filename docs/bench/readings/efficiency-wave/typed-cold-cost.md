# The typed pass's cold cost (EW-71; owner rulings C1 and K30)

**Verdict: PASS. With fathom's measurement-only nested modules skipped, a stranger's first `fathom
init` of fathom on a cold build cache takes 220 s, under the 306 s bar (255 s + 20%). At 50783574
it took 1698 s.** The typed pass no longer compiles the prometheus, alertmanager and grafana
oracles' third-party trees, which wave h5's own-module rule (EW-62) made fathom's own by module
path. fathom's cold first ingest drops from 1782 s to 299 s, and its warm ingest from 113 s to 76 s
(medians of three interleaved rounds). Cold peak RSS drops 16%; warm peak RSS and store size do not
move. The stored output loses
only those four modules' Go typed edges; no verify verdict on fathom's plans changes (STEP 3, below).
Keep the change: it removes the regression the K26 ruling's warm-only reading did not see, and
`fathom init` now stops at once when the daemon fails or refuses the ingest instead of waiting 30
minutes.

EW-71 verdict: PASS (stranger-check init 220.3 s on a cold GOCACHE at b50b5f1f, bar 306 s; 1697.6 s at 50783574)

## What changed (wave rf1, branch rf1-typedcost)

- `[typed] skip_modules` (EW-68): a repository's own `.fathom/config.toml` names nested Go modules
  the typed pass does not load, as repo-relative directory globs. fathom's own config skips
  `calibration/oracles/**` and `calibration/telemetry-census` (b50b5f1f).
- The typed pass's one stderr line (EW-69) also names the three slowest nested module loads and the
  knob. On a scratch ingest of b326e2a5 (no skip yet, warm cache) it read: `slowest nested module
  loads: calibration/oracles/prometheus 8.5s, calibration/oracles/alertmanager 5.2s,
  calibration/oracles/grafana 1.6s; skip a module with [typed] skip_modules in .fathom/config.toml`.
- `fathom init` (EW-70) polls the run its ingest was answered with and stops when the run reads
  done with ok=false. No ExtractorEpoch or TypedEdgeEpoch change.

## Method

- Machine: the shared wave host (go 1.26.3, darwin/arm64), wave rf1 running beside it: load
  average 6 to 17, swap 17.4 of 18 GiB used. Every heavy step ran under the wave's ingest lock
  (`lockf -k <wave scratch>/ingest.lock`), one at a time.
- Cold means a private, empty `GOCACHE`, deleted after the run. The module cache (`GOMODCACHE`)
  was the wave's shared, already-populated one, so no step downloads modules; a stranger who has
  never built these modules also pays the downloads.
- Base is 50783574 (origin/main). After is b50b5f1f (the code of e2b5acde and 0351207f plus the
  skip in fathom's config); later commits on the branch change only docs.

### (a) Per-module cold typed loads

`loadprobe` (a scratch program, outside the module) calls `packages.Load` exactly as
`loadTypedModule` does at 50783574: pattern `./...`, `Tests: true`, the same `Need*` mode, `Dir`
the module root. One private cold `GOCACHE` is shared across the sequence and deleted after it, in
the pass's own order: the root module first (`GOFLAGS=-p=2 GOMAXPROCS=2`, the pass's root caps),
then the four modules the pass loads at base, then every other nested module, each under
`nestedModuleEnv`'s caps (`GOFLAGS=-p=1 GOMAXPROCS=1 GOMEMLIMIT=1GiB GOTOOLCHAIN=local`, set on the
go children only, as in the daemon). Each figure is that module's marginal cold cost after the
modules above it. Peak RSS is `/usr/bin/time -l`'s maximum resident set size of the probe and its
go children (the largest single process).

```sh
# per module, under the ingest lock:
/usr/bin/time -l loadprobe <module dir> GOFLAGS=-p=1 GOMAXPROCS=1 GOMEMLIMIT=1GiB GOTOOLCHAIN=local
```

fathom has 43 nested modules: 42 under `calibration/oracles/` (41 at depth one, plus
`scip-clang/preproc`) and `calibration/telemetry-census`. At base the pass loaded four of them (module
path fathom's plus the directory); the other 39 (`fathom.bench/oracles/*`) were not loaded.

| Module | loaded at base | cold load wall | packages | peak RSS |
|---|---|---:|---:|---:|
| root module (`.`) | yes | 86.5 s | 427 | 1.71 GB |
| `calibration/oracles/alertmanager` | yes | 43.1 s | 3 | 0.52 GB |
| `calibration/oracles/grafana` | yes | 7.9 s | 1 | 0.16 GB |
| `calibration/oracles/prometheus` | yes | 366.8 s | 3 | 2.16 GB |
| `calibration/telemetry-census` | yes | 0.6 s | 3 | 0.04 GB |
| `calibration/oracles/ansible` | no | 0.5 s | 1 | 0.05 GB |
| `calibration/oracles/argo` | no | 0.02 s (load failure: its go.mod needs go 1.26.5, the local toolchain is 1.26.3, GOTOOLCHAIN=local; K22) | 0 | 0.01 GB |
| `calibration/oracles/caddyfile` | no | 13.4 s | 3 | 0.19 GB |
| `calibration/oracles/cargo` | no | 0.6 s | 1 | 0.04 GB |
| `calibration/oracles/cedar` | no | 0.4 s | 1 | 0.04 GB |
| `calibration/oracles/clang-ast-defs` | no | 0.7 s | 3 | 0.04 GB |
| `calibration/oracles/clang-calls` | no | 0.6 s | 3 | 0.05 GB |
| `calibration/oracles/clang-includes` | no | 0.8 s | 3 | 0.03 GB |
| `calibration/oracles/crossplane` | no | 0.6 s | 3 | 0.05 GB |
| `calibration/oracles/cue` | no | 0.5 s | 3 | 0.05 GB |
| `calibration/oracles/docker-compose` | no | 0.2 s | 1 | 0.04 GB |
| `calibration/oracles/envoy` | no | 15.0 s | 3 | 0.19 GB |
| `calibration/oracles/flux` | no | 8.0 s | 3 | 0.13 GB |
| `calibration/oracles/gator` | no | 65.5 s | 3 | 0.47 GB |
| `calibration/oracles/github-actions` | no | 3.3 s | 1 | 0.33 GB |
| `calibration/oracles/gitlab-ci` | no | 0.5 s | 3 | 0.05 GB |
| `calibration/oracles/go-mod` | no | 0.3 s | 3 | 0.04 GB |
| `calibration/oracles/gradle` | no | 0.4 s | 3 | 0.05 GB |
| `calibration/oracles/helm` | no | 52.9 s | 3 | 0.47 GB |
| `calibration/oracles/jaxrs` | no | 0.6 s | 3 | 0.03 GB |
| `calibration/oracles/kong` | no | 0.6 s | 3 | 0.06 GB |
| `calibration/oracles/kubernetes` | no | 1.4 s | 3 | 0.08 GB |
| `calibration/oracles/kustomize` | no | 0.3 s | 3 | 0.04 GB |
| `calibration/oracles/kyverno` | no | 250.9 s | 3 | 0.56 GB |
| `calibration/oracles/lefthook-dump` | no | 0.5 s | 3 | 0.04 GB |
| `calibration/oracles/maven` | no | 0.4 s | 3 | 0.05 GB |
| `calibration/oracles/msbuild` | no | 0.2 s | 1 | 0.04 GB |
| `calibration/oracles/npm` | no | 0.6 s | 3 | 0.06 GB |
| `calibration/oracles/opa-parse` | no | 0.2 s | 1 | 0.03 GB |
| `calibration/oracles/otelcol` | no | 3.5 s | 3 | 0.17 GB |
| `calibration/oracles/protoc-descriptors` | no | 0.6 s | 1 | 0.05 GB |
| `calibration/oracles/pulumi-yaml-go` | no | 46.9 s | 3 | 1.09 GB |
| `calibration/oracles/scip-clang/preproc` | no | 0.2 s | 3 | 0.03 GB |
| `calibration/oracles/spring` | no | 0.2 s | 3 | 0.03 GB |
| `calibration/oracles/taskfile` | no | 0.4 s | 3 | 0.05 GB |
| `calibration/oracles/terraform` | no | 0.5 s | 3 | 0.04 GB |
| `calibration/oracles/timoni` | no | 0.5 s | 3 | 0.05 GB |
| `calibration/oracles/traefik` | no | 1.2 s | 3 | 0.08 GB |
| `calibration/oracles/vcpkg-depend` | no | 0.2 s | 1 | 0.04 GB |

- The four modules loaded at base cost 418.3 s cold after the root module (86.5 s). prometheus
  alone is 366.8 s at 2.16 GB: the aws-sdk-go-v2 ec2 compile O812 found.
- The other 39 would cost 474.0 s more if they were loaded, mostly kyverno (250.9 s), gator
  (65.5 s), helm (52.9 s) and pulumi-yaml-go (46.9 s).
- `[typed] skip_modules = ["calibration/oracles/**", "calibration/telemetry-census"]` skips all
  43, so a module that a future path change makes fathom's own stays skipped too.

### (b) stranger-check init, cold

`scripts/stranger-check.sh` from a clean worktree at each SHA, with `FATHOM_CACHE_ROOT` unset and
`GOCACHE` a private empty directory (deleted after), the shared module cache, under the ingest
lock. The script builds the tracked tree, starts a throwaway daemon (4 GiB budget) and times
`fathom init`.

```sh
unset FATHOM_CACHE_ROOT; export GOCACHE=<private, empty>
lockf -k <ingest.lock> /usr/bin/time -l bash scripts/stranger-check.sh
```

| SHA | init | build | first proven answer | peak RSS (tree) | cold GOCACHE after |
|---|---:|---:|---:|---:|---:|
| 50783574 (base) | **1697.6 s** | 86.8 s | 29:45 | 2.15 GB | 3.1 GB |
| b50b5f1f (after) | **220.3 s** | 63.3 s | 4:44 | 1.89 GB | 1.1 GB |

The bar is 306 s (255 s, the pre-h5 base, + 20%): PASS. Wave h5 measured 1218 to 1413 s for base
on a quieter host; this host's 1698 s says the same thing louder. One run per SHA: each base run
holds the wave's ingest lock for 30 minutes.

### (c) fathom ingest: cold first ingest (K30) and warm

Fresh-store scratch ingests of a `git archive` of each SHA, each SHA's own binary, under the ingest
lock. Cold: a private empty `GOCACHE`, deleted after. Warm: the wave's shared `GOCACHE`, after one
uncounted warm-up ingest per binary, then three interleaved rounds (base, after; after, base;
base, after).

```sh
lockf -k <ingest.lock> /usr/bin/time -l <bin>/fathom ingest --store <fresh>.sqlite --service fathom --path <archive>
```

| | base 50783574 | after b50b5f1f | change |
|---|---:|---:|---:|
| cold first ingest wall | 1781.8 s | 299.3 s | -83% |
| cold typed pass wall | 1726.2 s | 251.4 s | -85% |
| cold peak RSS | 2.15 GB | 1.81 GB | -16% |
| warm ingest wall, median of 3 | 113.1 s | 76.1 s | -33% |
| warm typed pass wall, median of 3 | 70.7 s | 32.0 s | -55% |
| warm typed pass CPU (in process), median of 3 | 26.9 s | 23.1 s | -14% |
| warm peak RSS, median of 3 | 2.06 GB | 2.09 GB | +1.6% |
| store | 154,791,936 B | 154,804,224 B | +0.01% |
| typed edges | 163,564 | 163,278 | -286 |

Per round (warm ingest wall / typed pass wall): base 97.4 / 54.3 s, 113.1 / 70.7 s, 148.0 /
102.6 s; after 66.6 / 26.5 s, 76.1 / 32.0 s, 90.8 / 47.8 s. Every after round is below every
base round. The warm-ups (uncounted) read base 203.4 s and after 501.0 s; the after warm-up
rebuilt the export data of the packages its own commits changed while the host was busy.

K26 read warm wall against the ruled +17% baseline: the after wall is 33% below base, so the ruled
+17% is gone rather than grown. The after typed pass loads the root module alone, which is the
pre-EW-59 shape fb7d9f3f measured, plus the skip line. K30 (cold first ingest) reads -83%.

## What the stored output loses

Only the four modules' Go typed edges. Typed edges from Go files under `calibration/` drop from
576 to 0: grafana/main.go 360, alertmanager 88, prometheus 82 and telemetry-census 46 (cold stores
of base and after). Typed edges from all files under `calibration/` drop from 1,338 to 762; the 762
that remain come from non-Go files (the Python and JavaScript oracles, `calibration/incidents`' Java
and JavaScript), which the typed Go pass never wrote. The skipped modules keep their syntax-pass
edges. Whole-store typed edges: 163,564 to 163,278 (the after tree also carries this branch's new
code). No ExtractorEpoch or TypedEdgeEpoch change: a store whose typed pass ran without the skip
re-runs it once at the next ingest (the state row's skip stamp changed) and purges those 576.

## Verify verdicts before and after the skip (STEP 3, K27)

`fathom verify --strict --json` over every `docs/plans/*.md`, one `--plan` each, against two fresh
scratch ingests (warm cache, b50b5f1f's binary) of two clean worktrees whose only difference is
fathom's `.fathom/config.toml`: b326e2a5 (the code, no skip) and b50b5f1f (the code and the skip).
Each verify ran in its own worktree, at the ingested commit, so `gone:` rows read their real verdict.

```sh
lockf -k <ingest.lock> bin/fathom ingest --store <fresh>.sqlite --service fathom --path .   # in each worktree
bin/fathom verify --strict --json --store <fresh>.sqlite --service fathom --plan docs/plans/<plan>.md
```

| | b326e2a5 (no skip) | b50b5f1f (skip) |
|---|---:|---:|
| plans verified | 69 | 69 |
| plans exiting 0 | 69 | 69 |
| rows: pass / fail / unknown / total | 1951 / 447 / 486 / 2884 | 1951 / 447 / 486 / 2884 |
| rows whose verdict changed | | 0 |
| rows with any criterion changed | | 0 |

No row's verdict or criterion changes: no plan row names a symbol in the skipped modules, and the
rows that name their files are lexical (LB-F42 and LB-F85 read `calibration/oracles/*/main.go`
directly). The 447 failing and 486 unknown rows are the same rows on both sides, and every plan
exits 0 on both, so every claimed row verifies with and without the skip.

## Caveats

- One stranger-check run per SHA. Base's 1698 s is consistent with wave h5's 1218 to 1413 s; the
  after run is 86 s under the bar.
- The host was not quiet. Lock waits are excluded from every wall above (each step's clock starts
  when it holds the lock), but CPU contention from other waves is not.
- (a) is marginal cost in one order on one shared cold cache; a module measured alone on an empty
  cache would also pay the standard library and the dependencies an earlier module compiled.
