# Product reach across nested Go modules (EW-59, EW-60, EW-61)

**Verdict: keep the fix, and an owner ruling is needed on its cost.** The typed pass now loads a
repository's nested Go modules. In platform-engineering, every env-kit and image-kit plan row that
names a symbol now reads wired: 22 of 22 env-kit rows (0 before) and 3 of 3 image-kit rows (0
before).

On fathom, the cost is over the K26 bar. Loading every nested module took the ingest from 70.1 s to
89.0 s (+27%). fathom's 42 calibration oracles each load a third-party dependency graph. The first
ingest compiled those graphs, and the typed pass took 1008 s. The fix was then narrowed: the pass
loads only the nested modules that belong to the repository. With the narrowing, fathom's warm ingest
grows from 53.3 s to 62.5 s (median of five interleaved pairs, +17%, bar +15%). The typed pass grows
from 22.0 s to 29.5 s. In isolation, fathom's four own nested modules cost 4.6 to 4.8 s of the pass.
Store size grows by 0.26%, peak RSS is unchanged, and no verdict on fathom's 66 plans changes.

The rows are built and covered by tests. EW-59 and EW-60 stay open until the owner rules on the
2-point overrun.

EW-61 reading: platform-engineering 70d88ddb, fathom fb7d9f3f -> e0da3f75: env-kit 22/22 rows wired (was 0/22), `verify --strict` 3 claimed verified (was 0); image-kit 3/3 wired (was 0/3), 2 verified (was 0); PE typed_edges 2,819 -> 33,394, warm ingest 12.2 -> 67.7 s; fathom warm ingest 53.3 -> 62.5 s (+17%), typed pass 22.0 -> 29.5 s, store +0.26%, 0 verdict changes over 66 plans.

## The cause

The typed pass was one `packages.Load(cfg, "./...")` with `Dir` at the repository root
(`internal/fusion/typedcalls.go`). `./...` never enters a directory that holds its own go.mod.
`loadableByRootPattern` and `zeroPackagesDisclosed` excluded nested modules by design.

The syntax pass could not make up for it. `bosun`'s main calls `cmd.NewRoot`, and four kits (docs,
env, image, test) each declare `internal/cmd.NewRoot`. `callgraph.resolve`'s `pkg.Name`, `dir.Name`
and suffix lookups are all ambiguous there, and ambiguity binds nothing. So each of those mains is a
product entry that reaches itself and its own `execute`, and nothing else. review-kit's main calls
uniquely named functions (`cli.RunWithContext`, `cli.NewGitHost`), so its syntax edges resolved. That
is why review-kit and ci-kit had edges and env-kit did not.

Ruled out:

- Entry detection: `bosun`'s main is one of the 16 entries.
- The replace directives: no nested module was ever attempted.
- The go directive: 1.26.3 against a local go 1.26.3.
- The syntax pass's import mapping: no ExtractorEpoch bump is needed for this fix.

## platform-engineering, per nested module

Ingested with fathom fb7d9f3f (before) and e0da3f75 (after), warm GOCACHE. Go functions and methods,
non-test files. "Syntax-only" means an out-edge the typed pass did not confirm.

| Module | Go funcs | typed out-edges before → after | syntax-only out-edges before → after | product-reached before → after |
|---|---:|---:|---:|---:|
| `.` | 258 | 337 → 356 | 60 → 46 | 211 → 211 |
| `capstan` | 417 | 0 → 509 | 338 → 60 | 9 → 47 |
| `capstan/witness` | 35 | 0 → 9 | 10 → 0 | 1 → 18 |
| `kits/ci-kit` | 430 | 0 → 561 | 426 → 40 | 59 → 118 |
| `kits/ci-kit/apiclient` | 13 | 0 → 11 | 5 → 0 | 0 → 0 |
| `kits/ci-kit/depgraph` | 62 | 0 → 61 | 53 → 11 | 2 → 11 |
| `kits/ci-kit/lint` | 9 | 0 → 6 | 3 → 2 | 0 → 2 |
| `kits/docs-kit` | 166 | 0 → 240 | 220 → 26 | 3 → 138 |
| `kits/env-kit` | 436 | 0 → 890 | 625 → 50 | 2 → 392 |
| `kits/env-kit/doctorreport` | 9 | 0 → 2 | 2 → 0 | 6 → 9 |
| `kits/env-kit/fingerprint` | 6 | 0 → 6 | 2 → 0 | 0 → 6 |
| `kits/env-kit/policyengine` | 27 | 0 → 30 | 22 → 0 | 1 → 27 |
| `kits/env-kit/scaffold` | 8 | 0 → 0 | 3 → 0 | 2 → 5 |
| `kits/image-kit` | 199 | 0 → 342 | 235 → 21 | 2 → 145 |
| `kits/review-kit` | 695 | 0 → 1472 | 1463 → 157 | 516 → 687 |
| `kits/test-kit` | 187 | 0 → 309 | 250 → 44 | 2 → 156 |
| `kits/test-kit/examples/runnable` | 3 | 0 → 0 | 0 → 0 | 0 → 0 |
| `label` | 10 | 0 → 4 | 3 → 0 | 2 → 9 |
| `pdk/gen/addons` | 19 | 0 → 18 | 14 → 0 | 12 → 18 |
| `pdk/substrates/functions/fn-platform-defaults` | 9 | 0 → 8 | 8 → 0 | 1 → 1 |
| `pdk/substrates/functions/fn-secure-service` | 5 | 0 → 3 | 3 → 0 | 1 → 1 |
| `platform` | 56 | 0 → 48 | 34 → 0 | 3 → 6 |
| `portal` | 1464 | 0 → 779 | 2557 → 2073 | 179 → 178 |

Product entries: 16 → 47. Functions reached: 1,225 → 2,599.

`kits/test-kit/examples/runnable` (`example.com/cart`) is not loaded. Its module path is not the
repository's, and the pass discloses it in one line. The cobra and pflag copies under
`kits/*/third_party/` fall under the same rule, and discovery ignores them anyway.

`portal` is not a clean comparison in a clone. Its go.mod replaces app-kit with `../../app-kit`, a
sibling checkout outside the repository, so 10 portal packages fail typed loading in a clone and the
ingest reads degraded. With the caller packages failing, typed edges cover only part of the module.
The resulting change is 7 functions gained and 14 lost (GitCatalog's walk and its domain types),
which reaches 179 → 178. Under the owner's checkout, where `../../app-kit` exists, those packages
load.

## platform-engineering, verify

`verify --strict` run before and after on the same tree (`docs/plans/env-kit.md`,
`docs/plans/image-kit.md`):

| Plan | Summary before | Summary after | rows reading wired, before → after |
|---|---|---|---|
| env-kit | 80 tasks total; 46 claimed: 0 verified, 46 failing | 80 tasks total; 46 claimed: 3 verified, 43 failing | 0 → 22 (22 library-only → 0) |
| image-kit | 47 tasks total; 22 claimed: 0 verified, 22 failing | 47 tasks total; 22 claimed: 2 verified, 20 failing | 0 → 3 (3 library-only → 0) |

EK-121, EK-124, EK-125 and EK-126 read `wired` and `pass` under `--require-all`. The plans'
remaining unknowns are their own: `skip_test` waivers read no-contract, and runtime_pass has no
CI evidence ingested. Those are platform-engineering's proof items.

## Cost

Interleaved scratch ingests under the wave's ingest lock, warm GOCACHE, go 1.26.3, on a machine with
a load average of 4 to 8 from other waves. Values are medians.

| Subject | Build | typed pass wall | typed pass CPU (in process) | ingest wall | peak RSS | store | typed_edges |
|---|---|---:|---:|---:|---:|---:|---:|
| fathom | fb7d9f3f (rounds 7–11) | 22.0 s | 21.2 s | 53.3 s | 1.95–2.06 GB | 151.24 MB | 161,028 |
| fathom | e0da3f75, own modules only (rounds 7–11) | 29.5 s | 22.5 s | 62.5 s (+17%) | 1.97–2.05 GB | 151.63 MB (+0.26%) | 161,876 |
| fathom | 4bf63e3a, every nested module (rounds 3–5) | 56.2 s (vs 22.8 s) | 26.4 s | 89.0 s (vs 70.1 s, +27%) | 2.00–2.05 GB | 153.06 MB (+1.2%) | 165,360 |
| platform-engineering | fb7d9f3f | 1.2–3.5 s | 0.13–0.17 s | 10.8–12.2 s | 0.48–0.50 GB | 41.7 MB | 2,819 |
| platform-engineering | e0da3f75 | 58.2 s | 4.1 s | 67.7 s | 0.48 GB | 62.9 MB | 33,394 |

- Per-pair ingest ratios, own modules only: 1.113, 1.172, 1.186, 1.219 and 1.138.
- Attribution with the pass alone (a scratch SQLite store, three interleaved pairs):
  - The root module's share is unchanged: 23.0 / 31.8 / 27.5 s before, 23.3 / 31.5 / 23.6 s after.
  - The four own nested modules add 4.6 to 4.8 s: alertmanager and grafana about 1.4 s, prometheus
    3.1 s, telemetry-census 0.3 s.
  - Their go/packages loads alone take 2.6 to 3.5 s.
  - fathom's other 39 nested modules (`fathom.bench/oracles/*`) are not loaded.
- First ingest on a GOCACHE without the nested modules' dependency graphs:
  - fathom with every nested module: typed pass 1008 s, against 128 s for fb7d9f3f's first run.
    The cause is kyverno's, prometheus's and helm's trees.
  - platform-engineering: typed pass 636 s.
  - After that, the build cache answers.
- The typed pass runs only when a Go input changed (RT-104), so the warm figure is what an edit
  pays. The pass still reloads every module on any Go change. Gating each module on its own inputs
  would need a per-module state row.
- Not loaded under the narrowing: fathom's argo oracle. Its go directive (1.26.5) is newer than the
  local toolchain. Loading it is a disclosed failure, never a toolchain fetch (K22), but it makes
  fathom's own ingest read degraded, and the pre-push verify gate reads a degraded ingest as
  fail-open (O780).

## fathom's own verdicts

`verify --strict --require-all` over all 66 plans (2,736 tasks), on a scratch ingest of each build,
each run from its own tree:

- 1,887 claimed rows: 1,887 verified before and after.
- 0 rows change status.
- 0 rows change their wired criterion.

Under the narrowing, alertmanager, grafana, prometheus and telemetry-census gain typed edges. Their
module paths are the repository's. Their mains were already entries and were already reached through
syntax edges.

## Subjects and binaries

| What | Identity |
|---|---|
| fathom before | fb7d9f3f (origin/main at the start of wave h4) |
| fathom after | 4bf63e3a (every nested module), e0da3f75 (own modules only) |
| platform-engineering | 70d88ddb, an https clone in the wave's scratch directory, deleted after |
| Go | go1.26.3 darwin/arm64, GOTOOLCHAIN=local, GOWORK=off, GOFLAGS -p=2 |

## Commands

```sh
lockf -k <ingest.lock> <fathom> ingest --store <scratch>.sqlite --service fathom --path <tree>
lockf -k <ingest.lock> <fathom> ingest --store <scratch>.sqlite --service platform-engineering --path <pe-clone>
<fathom> verify --strict [--require-all --json] --store <scratch>.sqlite --service platform-engineering --plan docs/plans/env-kit.md
<fathom> verify --strict --require-all --json --store <scratch>.sqlite --service fathom --plan docs/plans/<each>.md ...
sqlite3 <scratch>.sqlite "select count(*) from typed_edges where pkg_path like 'gomod:%'"
```

The per-module table comes from a scratch probe outside the tracked tree. It builds
`callgraph.BuildBoth` and `ProductEntryReach` over the store, as `fathom verify` does, and counts
each Go function's out-edges by kind and its reach, grouped by the nearest go.mod.
