ENG-1 verdict: 17-fold gain and a miss of the one-second bar on the helper. The deployed half of ENG-1 stays open.

RT-195 implementation and equality proof, 2026-11-15. Base snapshot:
`a6c67b22038f6f056d267305b830ed5becc74403`. The lane's implementation commit
contains this reading and the three callgraph source/test files.

`Build` now constructs a directory → dot-suffix → candidate-keys index once,
uses it for constructor-scoped calls, and releases it when the build returns.
The graph retains no constructor index. `resolveCtorScoped` is unchanged:
the scan remains the independent test oracle and the package-patch resolver.

## Evidence

The prescribed test was written and executed before implementation:

```text
ctor_scope_index_test.go:95:11: undefined: buildCtorScopeIndex
FAIL github.com/geoffmilleraz/fathom/internal/analysis/callgraph [build failed]
```

After implementation, under the gate lease:

```text
go test -race ./internal/analysis/callgraph -run '^TestCtorScopeIndexMatchesScan$' -count=1
ok github.com/geoffmilleraz/fathom/internal/analysis/callgraph 1.899s
go test -race ./internal/analysis/callgraph -count=1
ok github.com/geoffmilleraz/fathom/internal/analysis/callgraph 9.902s
```

The equality test checks constructor resolutions and complete sorted syntax edge
sets against the unchanged scan over 30 seeded generated sets, explicit corner
cases, and Go extraction of the existing fauxtown fixture. It checks four input
permutations per set, colliding directory suffixes, nested import paths,
same-named methods, absent methods, empty/root file paths and Go fields.
The existing served-call golden test passed in the package run.

## Unproven acceptance

No cold-build timing was taken. Test durations above are test process durations,
not cold graph build measurements. Neither the RT-195 ratio of at most 0.1 nor
the ENG-1 deployed build below one second is established.

Deployment preflight found zero valid code-signing identities. `task deploy`
requires the stable `fathom-dev` identity and refuses an ad-hoc replacement.
The reachable daemon reported an existing ad-hoc binary. No deploy was attempted,
no serving HEAD assertion was obtained, and no deployed binary identity or timing
is claimed. Creating a signing identity changes global configuration and is
outside this lane's owned paths.

The integration owner must supply the signing identity, measure indexed and scan
cold builds over the same pinned snapshot, deploy with `task deploy`, retain its
serving HEAD assertion, and read the deployed cold build. RT-195 and ENG-1 remain
unchecked. No scratch-ingest verification or full integration gate is claimed.
The integration owner also owns the observable-performance change note because
this lane's path allowlist contains no changelog fragment.

## W5-2 full admitted cache-miss reading, 2026-11-15

**W5-2 verdict: CONDITION NOT MET. Do not dispatch #251.** The raw median
index load is **1,529,603,250 ns**. W5-2 requires strictly
less than 1,000,000,000 ns. The scan median is
26,485,509,250 ns. The index/scan median ratio is
0.057752457601. Every one of the five index trials exceeds one second.

This append supplies the previously missing source-built scratch timing. It does
not establish deployed ENG-1, check a plan box, amend RT-197, or implement #251.
No deploy, restart, push, tag, model call, or full integration gate was run.

### Source, corpus and store

- Source commit: `d9eef4794afdbc200d1b8697ce121d9d688fedaf`.
- Source tree: `4090b63cfd5bb2688d476bc54386a2f23fa1e080`.
- RT-195 source change at this tip: `f1bad259cb2d389718657afa9a401139e7f3175b`.
- An unmodified `git archive` of that commit supplied the corpus and both build
  copies, entirely under ignored `workspace/tmp/w5-2/`. The corpus contains no
  measurement instrumentation. An archive-to-files comparison after the run
  proved every archived regular file in the corpus unchanged.
- A source-built, uninstrumented CLI ingested the corpus once into a new SQLite
  store, service `w5-2`. It reported `outcome=ok`, `exit_code=0`, `read_failed=0`,
  8,672 discovered files, 8,668 extracted files and 104,068 ingest-report symbols.
  Actual snapshot retrieval subsequently returned 104,183 nodes. These are
  different pipeline counters, not interchangeable population estimates.
- Default corpus exclusions remained in force: four oversize files, 185
  unclassified files, 30 generated-file skips, 175 typed package skips, and 52
  nested modules not loaded (51 configured skips and one foreign module path).
  Unsupported-language counts were two Groovy and 128 Scheme/Racket files.
  The typed Go pass processed 401 packages with zero failed packages. The
  complete stored typed-edge population read by the graph path was 256,033 rows.
  These exclusions bound this reading; no claim of complete language coverage
  or successful analysis of excluded files is made.
- All trials read the same store and watermark
  `d9eef4794afdbc200d1b8697ce121d9d688fedaf@1794782482463676000`.
  The main SQLite file SHA-256 was `e22f40d4dd467b0201badae263901741db23a81864c4f545c111b5c469ecd704`
  both before and after all ten trials. No re-ingest occurred between trials.

### Actual boundary and timing configuration

The harness calls the real `internal/mcp.loadNavGraphsErr`, rather than a
constructor benchmark, fixture graph builder, or cached tool response. The
monotonic timer starts immediately before enforced admission, before
`navScope`, snapshot retrieval, snapshot indexing, syntax or graph work. Four
identical phase marks were added only in the ignored serving-source copies:

```text
start -> Gate.Admit -> loadNavGraphsErr -> navScope -> loadNavGraphsAt
  serviceSnapshotAt: SnapshotForExtra -> Client.ListAllSymbols -> newSnapshot
  mark snapshot
  GraphFor(GraphSyntax): callgraph.Build
  mark syntax
  GraphForTyped(GraphBoth): TypedFor -> Client.TypedEdges -> callgraph.BuildBoth
  mark both       <- required full-load timer endpoint
  GraphForTyped(GraphResolved)
  mark resolved -> return -> serving-boundary timer endpoint
```

The W5-2 decision uses the cumulative `both` mark, after the snapshot and both
required complete graphs exist. The full serving-boundary wall time, which also
includes the additional resolved graph, is published separately. Its index
median is 1,841,375,875 ns and its scan median is
26,770,054,833 ns. Neither measurement includes
compilation, ingest, client/store construction, JSON serialization, hashing,
or output comparison. Scope/watermark retrieval is inside both timings.

- Go `go1.27.4`, `darwin/arm64`, `CGO_ENABLED=1`,
  `MACOSX_DEPLOYMENT_TARGET=14.0`; optimized builds used
  `go build -trimpath -buildvcs=false -o <binary> <package>` with empty
  `GOFLAGS`. There was no race instrumentation, `-N`, `-l`, PGO profile, or
  timing-specific compiler optimization. Packages were `./cmd/fathom` for the
  ingest CLI and `./cmd/w5read` for both timing variants.
- Apple M2 Pro, 12 CPUs, 32 GiB RAM. The harness explicitly resolves the default
  managed budget from those facts: 4 GiB budget, 3 GiB shed mark, 4 GiB refuse
  mark, 8 GiB exit/GC hint and 1 GiB shared memo cache. `GOMAXPROCS=6`,
  `GOMEMLIMIT=8GiB`, `GOGC=100`. No forced GC preceded a trial.
- Actual admission is `Gate.Admit`, enforce mode, tree class, CLI origin, two
  tree slots and a 268,435,456-byte per-call ceiling. The only admission-budget
  override is a ten-minute tree deadline instead of the default one minute,
  declared before the run so a slow scan can complete. No trial approached
  either deadline. The resolved budget JSON retains its default `Admission`
  field of `observe`; the actual gate constructor explicitly selects enforce.
  The process publishes the default normal level; no live host governor sampler
  runs in this scratch harness. The current direct loaders charge zero ticket
  bytes, reported as zero rather than invented metering. No ticket stopped.
- Each trial is a fresh process with one client and initially empty application
  memo groups. This is an application cache miss, not an OS-cache-cold reading.
  Ingest and prior trials can warm OS/SQLite pages. Global caches were neither
  purged nor controlled. The existing compiler/module caches were reused only
  during preparation; builds and ingest were never timed as loads.
- One shared gate lease covered all builds, the ingest and the entire scheduled
  ten-trial run without release, with normal priority (`FATHOM_GATE_NICE=off`).
  The shared lease was `FATHOM_GATE_LOCK` at the wave-configured shared gate
  location. The gate-running marker was absent when heavy work began. No lane
  tests ran as part of this measurement. Other background activity was not
  stopped. Per-trial load averages appear below; this is not an idle-host claim.
- The schedule was fixed before execution: index, scan, scan, index, index,
  scan, scan, index, index, scan. No warmup, discarded measurement, invalid
  load, retry or selective rerun occurred. One source-preparation attempt failed
  before any build or timing because the installed Python lacked tarfile's
  `filter` argument; extraction compatibility was corrected. All builds and
  the single ingest succeeded on their first execution.

### Exact old-scan variant

The existing production `Graph.resolveCtorScoped` is the independent scan
oracle. The two source copies differ in exactly one file and these two hunks:

```diff
--- index/internal/analysis/callgraph/graph.go
+++ scan/internal/analysis/callgraph/graph.go
@@
-	ctorScopes := buildCtorScopeIndex(g.nodes)
@@
-				calleeKey = ctorScopes.resolve(raw[:i], raw[i+1:])
+				calleeKey = g.resolveCtorScoped(raw[:i], raw[i+1:])
```

All other production code is identical. Compared with the source archive, the
index copy changes only the four phase marks in `internal/mcp/resolved_nav.go`;
the scan copy additionally changes the two hunks above. Both add the same
`internal/mcp/w5_reading.go`, `internal/analysis/callgraph/w5_export.go` and
`cmd/w5read/main.go`. The wrapper records counters, takes timestamps and calls
the real serving entry. The graph exporter runs after timing and serializes
all 14 graph fields, including private suffix candidate keys/counts, both
adjacency directions, node metadata, resolver indexes, typed-edge kinds,
instantiations, typed files/directories and dispatch counts. JSON map keys are
canonicalized by the standard encoder. These additions are measurement-only;
no tracked production file was edited, even temporarily.

### Every raw full-load trial

All values below are integer nanoseconds. Snapshot includes admission and scope
work. Syntax and BOTH are incremental phase durations; BOTH includes its cold
typed-edge retrieval. Required total is snapshot + syntax + BOTH, without
rounding. Serving total includes the additional resolved graph and return.

| Order / variant | Snapshot ns | Syntax ns | BOTH ns | Required total ns | Serving total ns |
|---|---:|---:|---:|---:|---:|
| 01-index | 357650708 | 318913458 | 825643584 | 1502207750 | 1795751708 |
| 02-scan | 339099791 | 12788956292 | 13357453167 | 26485509250 | 26770054833 |
| 03-scan | 349913083 | 12805863167 | 13275399541 | 26431175791 | 26734034583 |
| 04-index | 357487167 | 328663292 | 843452791 | 1529603250 | 1853183125 |
| 05-index | 357179542 | 321182541 | 879928125 | 1558290208 | 1841375875 |
| 06-scan | 357231583 | 12937211584 | 13728124583 | 27022567750 | 27317464167 |
| 07-scan | 364362042 | 13105888000 | 13620856875 | 27091106917 | 27393357084 |
| 08-index | 350212250 | 318415166 | 811676209 | 1480303625 | 1767705625 |
| 09-index | 350941875 | 335455333 | 870789375 | 1557186583 | 1863766000 |
| 10-scan | 350343334 | 12777678833 | 13164777917 | 26292800084 | 26589802875 |

Every trial exited zero and reported `valid=true`. Additional raw measurements:

| Trial | UTC start (2026-11-15) | Symbol store read ns | Typed store read ns | Allocated bytes in load | GC cycles |
|---|---|---:|---:|---:|---:|
| 01-index | 22:41:31.230622Z | 249417500 | 345389875 | 1048422392 | 11 |
| 02-scan | 22:41:35.392103Z | 238863917 | 343680666 | 1044769448 | 11 |
| 03-scan | 22:42:03.928721Z | 243548708 | 340140042 | 1043366984 | 11 |
| 04-index | 22:42:32.454542Z | 249261208 | 345965542 | 1048061576 | 11 |
| 05-index | 22:42:36.124045Z | 245357916 | 346253250 | 1046745264 | 11 |
| 06-scan | 22:42:39.744939Z | 243424167 | 347477125 | 1043629608 | 11 |
| 07-scan | 22:43:08.881792Z | 247785708 | 337966750 | 1043138848 | 11 |
| 08-index | 22:43:38.029737Z | 243379750 | 341024583 | 1048415296 | 11 |
| 09-index | 22:43:41.565032Z | 242818375 | 341080833 | 1047759632 | 11 |
| 10-scan | 22:43:45.203099Z | 245221917 | 344697666 | 1043045904 | 11 |

The following RSS and process real times cover the entire process, including
after-timer artifact serialization/hashing. They are not graph-load cost.

| Trial | Load-end heap bytes | Process maximum RSS bytes | Process real seconds | Host load average 1 / 5 / 15 min |
|---|---:|---:|---:|---|
| 01-index | 591568112 | 1503576064 | 4.20 | 16.76 / 8.57 / 5.27 |
| 02-scan | 591294752 | 1502773248 | 29.10 | 17.02 / 8.76 / 5.36 |
| 03-scan | 573878952 | 1750581248 | 28.49 | 16.50 / 9.44 / 5.73 |
| 04-index | 592071216 | 1554202624 | 3.64 | 14.49 / 9.52 / 5.86 |
| 05-index | 642651736 | 1343913984 | 3.59 | 14.69 / 9.65 / 5.93 |
| 06-scan | 599779072 | 1725710336 | 29.11 | 15.44 / 9.88 / 6.03 |
| 07-scan | 574483664 | 1526398976 | 29.12 | 17.82 / 10.89 / 6.51 |
| 08-index | 600712448 | 1608974336 | 3.51 | 18.83 / 11.79 / 6.98 |
| 09-index | 588707856 | 1552056320 | 3.60 | 17.80 / 11.69 / 6.98 |
| 10-scan | 590420616 | 1553334272 | 28.39 | 16.85 / 11.60 / 6.97 |

No thermal or performance warning had been recorded in the captured
`pmset -g therm` readings. Host swap-in/out counters did not change across the
run. These are observed ambient conditions, not isolation guarantees.

### Completeness, misses and equivalence

The following exact values held separately in **each of all ten trials**:

| Evidence | Value per trial |
|---|---|
| Initial memo resident bytes | 0 |
| Snapshot backend loads / memo misses / flights / final entries | 1 / 1 / 1 / 1 |
| Typed backend loads / memo misses / flights / final entries | 1 / 1 / 1 / 1 |
| Graph memo misses / flights / final entries | 3 / 3 / 3 (syntax, BOTH, resolved) |
| Snapshot and graph memo hits | 0 |
| Typed memo hits | 3, internal reuse after the one cold typed load |
| Declines / abandonments / refuse cancellations / uncached builds / evictions | 0 / 0 / 0 / 0 / 0 |
| Refusal / context error / typed read error / stopped ticket | none / none / none / false |
| Snapshot nodes / typed-edge rows / constructor-reference occurrences | 104183 / 256033 / 4216 |
| Syntax nodes / directed edges | 103099 / 92411 |
| BOTH nodes / directed edges | 103118 / 162606 |
| Resolved nodes / directed edges | 103118 / 120260 |
| Final snapshot / typed memo bytes | 67552976 / 59800143 |

The snapshot includes duplicate symbol keys that graph construction deduplicates;
adding typed endpoints also changes the graph node population. Counts above
are actual separate populations and are identical between variants.

Graph byte estimates vary with slice capacity even when canonical outputs are
byte-identical. Each raw receipt reports its actual final graph and total bytes:

| Trial | Graph memo bytes | Total memo bytes |
|---|---:|---:|
| 01-index | 434284413 | 561637532 |
| 02-scan | 434285069 | 561638188 |
| 03-scan | 434285053 | 561638172 |
| 04-index | 434284525 | 561637644 |
| 05-index | 434284157 | 561637276 |
| 06-scan | 434283869 | 561636988 |
| 07-scan | 434284909 | 561638028 |
| 08-index | 434284909 | 561638028 |
| 09-index | 434285165 | 561638284 |
| 10-scan | 434284573 | 561637692 |

Every trial saved its complete snapshot, typed-edge slice and three complete
canonical graph outputs outside the timer. Independent exact byte comparisons
passed for all ten trials against the first trial, for all five populations.
The common SHA-256 values below are repeated in every raw result receipt:

| Complete output | SHA-256 in every trial |
|---|---|
| both | `48860bbf28bb6948682035b351de339e5cd6175290ad54f551f2ac547f55d692` |
| resolved | `b087384be66589ee51a2f08db4d382b3b3dbf88000869ca6adb9792dbdbafa85` |
| snapshot | `106d44446d04dc54781a110e5e24f42a15bc9055e56346426e0121dbc3fab202` |
| syntax | `ce43dbb85e06842ab965604eba4f5b6ae527cfabeef793c37778e2e867fefced` |
| typed | `1c07d1627ad3ac3f96ba8ecad7db5b2e2b97223f40de0c785a0a9719bf6510bf` |

### Raw receipts and reproduction provenance

Raw JSON, stdout, stderr, exit codes, per-trial ambient captures, complete
comparison inputs, source archive, source copies, scripts, binaries, ingest
output and build metadata remain under ignored `workspace/tmp/w5-2/` for lane
review. They are not deployed artifacts. These published hashes pin those
local receipts; the tables above preserve every raw timing in the commit.

| Raw result receipt (`results/<trial>/result.json`) | SHA-256 |
|---|---|
| 01-index | `4650d29e18ccef34847130a521f75cee538bf5c5ec51f39edfa90d8f8612e08c` |
| 02-scan | `0737407e1d451a9af530c50a9266636dd70e934ec45a06f4b71eca1cd1b69af6` |
| 03-scan | `e74b855dccfb889e89ad7ea8f688a061dc69ea94b66c0025036dc8c5fd170716` |
| 04-index | `4e56dc2cd8118d46e1c4b66d3aa4774e9e44141ac529a5e1e9c6e763e09e5ffd` |
| 05-index | `3adc4044a2b265820c401fe3ec2cc49b19103d1f98a84c171ba3859e4019dc82` |
| 06-scan | `64a25343249910970df375706cb40ca5c3270bc84dd8b8bb91957022e54b0e46` |
| 07-scan | `917ab8c6cdaaf9738cc98e958707f37ede7bb0f92876a64730bbcdcf162445c0` |
| 08-index | `34b1b9ab7f721be1495eb2e635bda07bb497609f49cc706a13674bccbaf88917` |
| 09-index | `e6b0b18d275f29d59039be39bf13969794724a8a89b92c6ffb47c22f3a402f83` |
| 10-scan | `3871bbc48b274e14d88f376f22449b8fa207222955171b3282d268c2dd0429eb` |

| Provenance artifact | SHA-256 |
|---|---|
| `source.tar` | `0d2836139e55958e8d7f4ab3ef9198034cbe70bc1e98df7c9b43f723be57d082` |
| `base.json` | `5a888518a5d5c07a10c063638c31ca4e795dc4005c7e5c1ddb156828ba7f0bf9` |
| `bin/fathom-ingest` | `44f3f3256a6045ffa084670ca7838cf6c05f1bde0c4e6823e047fc2944e439a2` |
| `bin/index` | `6f0cdc6d33f3622af7d77f4f6da87023cf64340333a7f8ef9f7f03fe98ef24da` |
| `bin/scan` | `7320998d799e2bcdeac5e10c8d353660262e59f1cba23f94d8858e2ef27b2d31` |
| `prepare.py` | `65b9e51ba8ccefee816accc31bb7565c34499a8c381cfc55ca812ab26a9b1d41` |
| `run.sh` | `05904e39ab6c20bb95d463de0ba975b773b114e6cae11fcf20a1847b038a9a10` |
| `analyze.py` | `abae9a44e33154764235828827f3cf65221f0230b02729ea6f949508eef4b152` |
| `proof.py` | `c2152f6ec15985c421947926110491240b49d30a15752673fdac428a8bf76fae` |
| `proof.log` | `51e6557b0b56497cd38f4c9ca0416eb8f09c5d85fce777df5c5193904a112284` |
| `common.diff` | `c7a876f27d31221732b1664a68d02b6493b384579d17f1378d767b0762d0d0fd` |
| `variant.diff` | `1ae29ac37cfb873ef9be821452c0b73799ef521698b812489e50edfa846fef45` |
| `index/internal/mcp/w5_reading.go` | `7b5c2ed20610993326f69ea4f2494246acf2e8d3729f0c0cad9d609de88f6e74` |
| `index/internal/analysis/callgraph/w5_export.go` | `fa8e7bac8de39a453a0087eaacd597154f4b4fa82c92d92f877590770eef59ac` |
| `index/cmd/w5read/main.go` | `ff253640acc90a3072e32bd2d9b6542446eaa71ccd3072e72343859619c83b5e` |
| `ingest.json` | `fdfaf95a2587d93be8e6c0af055ab9154a7bc2dcdde59dcbe502a59ed3893a71` |
| `ingest.stderr` | `47bb31f32c5244145a31d974f23d24e5b572a6ae414d7e0c3685de4d45969c50` |
| `build-info.txt` | `9437fd751aee897e4648264d0c3c76bd228eeef74c0f3587fafd14e3c125c44c` |
| `all-trials.json` | `03dfbe190cdf4109e490becd184cb9790b5ded18229ef8cb16cd5fdb57c27fb7` |
| `summary.json` | `31d390d16534191acb655fef22842a2a4f670b516b61ef575142e7e2d4a14e8b` |
| `lease-receipt.txt` | `04510971365aa0c2bea26c03852f52de47b3da63c6dccd485f61035ecdd6d9bd` |
| `run.log` | `27d0816b17bee5fd7bce65812c4626b2fe4cb16293e92a0632196dc68705a809` |
| `environment-before.txt` | `7e5d07c75737482c61174301822ffd01fb9607bd972290cf62576a816b40f953` |
| `environment-after.txt` | `a06d1ed2af592a42f76d8d90bb7d03f2eb26e29888ad02b64ce1118e54fbc548` |
| `SHA256SUMS` | `b90eb63316ad17ee7a4c42f7496d854eabb62832b5dbf974f3a4894b5aa2b3fd` |

The retained `run.sh` contains the exact commands. In outline, after sourcing
the lane environment and acquiring the shared lease, it sets all temporary
paths and `FATHOM_HOME` inside the ignored job directory, builds the three
optimized binaries, and runs:

```sh
bin/fathom-ingest ingest --store scratch.sqlite --service w5-2 --path corpus --json
# Separate process for each scheduled variant; each writes one raw receipt.
/usr/bin/time -lp bin/<index-or-scan> scratch.sqlite results/<trial> <variant>
python3 analyze.py
python3 proof.py
```

The analyzer recomputes every output digest and the exact five-trial medians.
The independent proof compares source archive bytes, the two variant trees,
all complete output files and the before/after store identities. Both passed:

```text
source_copy corpus tracked_changes []
source_copy index tracked_changes ['internal/mcp/resolved_nav.go']
source_copy scan tracked_changes ['internal/analysis/callgraph/graph.go', 'internal/mcp/resolved_nav.go']
variant_difference ['internal/analysis/callgraph/graph.go']
exact_byte_comparisons: PASS (10 trials x 5 complete populations)
admitted_complete_trials: PASS (5 index + 5 scan, no invalid trials)
store_unchanged: PASS
decision: NO DISPATCH (index median 1529603250ns >= 1000000000ns)
```

Limits: this is a single helper and pinned corpus, with five application-cold
loads per variant and warm/uncontrolled OS caches under recorded ambient load.
It does not measure a real hook deadline, first answer after a one-file refresh,
abandonment under traffic, deployment identity, or a deployed build. Those
acceptances remain open. W5-2 therefore requires escalation with these numbers
and no #251 dispatch.

## W6-1c indexed full-load CPU profile, 2026-11-15

**W6-1a verdict: a 17-fold gain and a miss of the one-second bar on the helper.**
The deployed half of ENG-1 stays open. The W5 medians remain exactly
1,529,603,250 ns indexed and 26,485,509,250 ns scanned (17.3153-fold).
W6-1b does not dispatch #251 as invalidation followed by reload on the next
query. The following CPU captures diagnose that existing result; they are not
new acceptance trials and do not replace, rerun or average into any W5 trial.

### Boundary, admission and identities

Five profile captures were declared before execution. Each ran in a fresh
process against the **same original W5 scratch store**, with no re-ingest and
no changed input. Source code remains pinned to
`d9eef4794afdbc200d1b8697ce121d9d688fedaf`; the reading branch resumed from
`e70154fbe8e2018259a39c8dc2224794a89979c6`. The original store SHA-256 remained
`e22f40d4dd467b0201badae263901741db23a81864c4f545c111b5c469ecd704` before
and after profiling. The watermark, snapshot and typed-edge populations are
exactly those published in W5. Verification recomputed and passed all **137**
entries in the retained W5 manifest, preserving its source, binaries, raw
trials and output artifacts.

A new ignored `workspace/tmp/w6-profile/source/` copies the frozen W5 index
source. It differs from that copy in only two files: the already measurement-
only `internal/mcp/w5_reading.go` and `internal/mcp/resolved_nav.go`. The diff
adds profiling control, labels, a snapshot-start label and a boundary counter
capture. It changes no resolver, store query, graph algorithm or production
behavior. All tracked production files remain untouched.

`runtime/pprof.StartCPUProfile` starts immediately before the monotonic timer
and enforced admission. The real `loadNavGraphsErr` then takes the scope, loads
and indexes the snapshot, builds syntax, reads typed edges, and builds BOTH.
The existing `both` phase mark records the required endpoint and immediately
calls `StopCPUProfile`. Only after it stops does the serving entry continue
with the resolved graph. Serialization, output hashing, comparisons, client
construction, compilation and ingest are outside CPU profiling. No resolved-
graph build or output-exporter frame occurs in any captured profile.

Phase labels (`snapshot`, `syntax`, `both`) and nested read labels (`symbols`,
`typed`) are inherited by memo build goroutines. These labels attribute
allocation samples even when `runtime.systemstack` cuts off the original
calling frame. Background GC/scheduler samples without a label remain
unassigned; they are not silently allocated to a preferred phase.

Builds use the same optimized non-race flags and runtime settings as W5:
Go 1.27.4 on darwin/arm64, cgo enabled, deployment target 14.0,
`go build -trimpath -buildvcs=false`, empty `GOFLAGS`, `GOMAXPROCS=6`,
8 GiB Go memory limit, `GOGC=100`, default managed 4 GiB budget and 1 GiB memo
cache. Actual admission remains enforced tree/CLI with the declared ten-minute
deadline. No live host governor sampler was added. All profiles ran at normal
priority under one continuous acquisition of the wave shared heavy-work lease,
covering source preparation, build, all five captures and pprof report
generation. The gate-running marker was absent before heavy work. OS/SQLite
cache warmth remains uncontrolled; there was no cache purge or warmup.

At the required BOTH endpoint, **every capture** had one snapshot miss/build
and entry, two graph misses/builds and entries (syntax and BOTH), and one typed
miss/build and entry with one internal typed hit. Snapshot/graph hits were
zero. Initial resident bytes were zero. All declines, abandonments, uncached
builds, evictions and refuse cancellations were zero. Each backend actually
loaded symbols once and typed edges once. Every capture was admitted, complete,
`valid=true`, exited zero and had no stopped ticket, context error or typed
read error. No profile attempt was discarded, invalidated or repeated.

After the profile stops, resolved adds the third graph miss/entry as in W5.
Exact byte comparisons passed for every complete snapshot, typed-edge slice,
syntax graph, BOTH graph and resolved graph against the original W5 outputs.
All five output SHA-256 values are the identical values already published
above. This is not a refused load, a cached response, a constructor-only
profile or a graph-builder-only profile.

### Raw capture boundaries

All phase values are integer nanoseconds. They are profiling-instrumented
diagnostics, not substitute acceptance readings. Resolved duration excludes
CPU-profile stop/flush overhead and is outside the CPU capture. The serving
wall time includes that overhead, so it must not be compared directly with
unprofiled W5 serving latency.

| Capture | Snapshot ns | Syntax ns | BOTH incl. typed read ns | Required total ns | Stop/flush ns | Resolved outside profile ns | Serving wall ns |
|---|---:|---:|---:|---:|---:|---:|---:|
| 01-index-profile | 357021958 | 315821542 | 837266000 | 1510109500 | 118053500 | 309597750 | 1937761417 |
| 02-index-profile | 354359708 | 342059584 | 850534916 | 1546954208 | 185914000 | 321398459 | 2054267375 |
| 03-index-profile | 335405667 | 305657917 | 838794125 | 1479857709 | 153104750 | 296545666 | 1929508667 |
| 04-index-profile | 360904584 | 338100333 | 844527125 | 1543532042 | 184095459 | 317001999 | 2044630375 |
| 05-index-profile | 348585334 | 304243875 | 818466666 | 1471295875 | 160881167 | 305772250 | 1937950084 |

| Capture | UTC start (2026-11-15) | Symbol store wall ns | Typed store wall ns | CPU samples | CPU sample ns |
|---|---|---:|---:|---:|---:|
| 01-index-profile | 23:04:30.215981Z | 240740084 | 345886375 | 160 | 1600000000 |
| 02-index-profile | 23:04:33.944416Z | 246778292 | 351294875 | 164 | 1640000000 |
| 03-index-profile | 23:04:37.794522Z | 233898333 | 348164791 | 156 | 1560000000 |
| 04-index-profile | 23:04:41.548086Z | 234889166 | 347424750 | 160 | 1600000000 |
| 05-index-profile | 23:04:45.332367Z | 244807292 | 338824125 | 153 | 1530000000 |

The merged CPU profile contains **793 samples at a 10,000,000 ns period**,
7.93 CPU-sample seconds. Its displayed 8.35-second duration includes profile
stop/flush accounting; it is not the sum of required-load wall work. Individual
capture granularity is 10 ms, and one merged sample is about 0.126% of the
denominator. These are sampled observations, not exact per-function clocks.
Percentages are rounded separately; the integer sample partition sums exactly.

### CPU shares without double counting

The following phase/read partition is disjoint and sums to 100%. The
BOTH graph row excludes its separately attributed typed store read. The
snapshot indexing row excludes its symbol store read. The unlabelled row
retains background work; the tiny admission/scope phase had no observed samples
and is still inside the measured boundary.

| Cost | CPU sample ns | Share of all 793 samples | Interpretation |
|---|---:|---:|---|
| C1 Symbol store read, including row scan/decode | 1110000000 | 14.00% | Full `ListAllSymbols` loader, including allocations |
| C2 Snapshot indexing after store read | 550000000 | 6.94% | `newSnapshot` and labelled allocation/runtime work |
| C3 Typed store read, including row scan | 1680000000 | 21.19% | Full `TypedEdges` loader, including allocations |
| C4 Syntax graph | 1390000000 | 17.53% | One `GraphFor(GraphSyntax)` build, including allocations |
| C5 BOTH graph after typed read | 2150000000 | 27.11% | `BuildBoth`, its repeated `Build`, typed augmentation and allocations |
| C6 Unlabelled background/runtime | 1050000000 | 13.24% | GC and runtime work left unattributed |

C1 + C3 is **35.18%** of CPU samples in the store loaders. C2 + C4 +
C5 accounts for 51.58%; C6 is the remaining 13.24%. These ownership shares
include runtime work and are not sums of function cumulative percentages.

The following **overlapping** stack-based views identify operations within
those buckets. They must not be added to the preceding partition or to each
other. A caller's cumulative share includes its callees. `systemstack` allocation
samples can retain a phase label while losing that caller in the stack.

| Stack view | CPU sample ns | Share of full profile | Overlap / limit |
|---|---:|---:|---|
| runtime.madvise | 2400000000 | 30.26% | Heap/page commitment beneath allocations, not an independent added phase |
| SQLite `Pread` path | 2000000000 | 25.22% | Within C1/C3; not measured disk-wait time |
| database/sql Rows.Next | 2110000000 | 26.61% | Includes SQLite stepping and the Pread path |
| database/sql Rows.Scan | 10000000 | 0.13% | Row assignment subset within store reads |
| encoding/json decode, any sampled frame | 0 | 0.00% | Zero observed samples; not zero elapsed decoding cost |
| callgraph.Build, both invocations combined | 1660000000 | 20.93% | Includes the Build inside BuildBoth |
| callgraph.BuildBoth | 1120000000 | 14.12% | Includes its nested Build and AddTypedEdges |
| callgraph.AddTypedEdges | 250000000 | 3.15% | Subset of BuildBoth |
| graphmemo.newSnapshot | 170000000 | 2.14% | Subset of C2; systemstack allocations can be outside this ancestry |
| callgraph.buildIndexes | 520000000 | 6.56% | Subset of Build |
| Graph.buildSuffixIndex | 340000000 | 4.29% | Subset of Build |
| buildCtorScopeIndex | 80000000 | 1.01% | Subset of Build; constructor indexing is no longer dominant |
| Graph.dropTypedFileGuesses | 20000000 | 0.25% | Subset of typed augmentation |
| runtime.growslice | 290000000 | 3.66% | Overlaps allocations in graph/read work |
| Graph.EstimatedBytes | 30000000 | 0.38% | Byte accounting within graph phases |
| runtime.gcBgMarkWorker | 560000000 | 7.06% | Subset of C6; not all GC costs |

`runtime.madvise` alone has 240 samples (30.26%). Its ownership split is
220 ms in C1, 380 ms in C2, 270 ms in C3, 550 ms in C4 and 980 ms in C5.
These are already included in the phase/read totals. The syntax-phase
`Build` ancestry is 790 ms (9.96%); the second `Build` inside BOTH is 870 ms
(10.97%). `BuildBoth` ancestry is 1,120 ms (14.12%), including 250 ms of
`AddTypedEdges` (3.15%). This is direct evidence of duplicated construction,
not a claim that all of C5 can be eliminated.

**Decode limitation.** There are zero observed `encoding/json` frames and only
one observed `Rows.Scan` sample (0.13%). The source still decodes `callsJSON`
for every symbol in `scanSymbolNodesWithHint`; that work did execute. Driver
value conversion can also occur under `Rows.Next`, so these samples do not
provide a separately identifiable percentage for all decoding combined. A
decoder rewrite is not supported by this profile.

**CPU versus wait.** Store wall intervals above include SQLite reads, scans,
JSON decoding, allocations, scheduling and waits. CPU samples in `Pread` or
`madvise` identify sampled stacks, not a disk-I/O timer or an off-CPU wait
profile. No disk-wait percentage was measured. Subtracting sampled CPU from
wall time would mix different instruments and unassigned background work.
No claim of disk saturation or guaranteed removable wall time is made.

### Smallest source-grounded removal proposals, unapproved

Intent consulted before these proposals: I21/I51 require watermark-scoped,
byte-accounted derived state; I38 preserves complete reads and named failures;
I49 requires admission. ADR-049 decision 2 retains the table/cache/job freeze.
ADR-095 remains proposed and conditional on OQ5 and a fresh tenancy reading.
RT-195 keeps its constructor index local to a build. W6-1b rejects the
next-query reload option at these numbers. No intent rule or future was
promoted or amended here, and no new revisit trigger is established by a CPU
profile. F27's ingest-cache future is unrelated to avoiding this full-load work.

Every proposal below removes only the identified redundant component, not
necessarily its entire cost bucket. Required cold reads, initial syntax
construction and snapshot indexes cannot simply be deleted while preserving
this full-load contract. No proposal is implemented or independently approved.

| Proposal | Source evidence at the pinned commit | Smallest removal and what remains |
|---|---|---|
| P1 Remove avoidable typed-pruning copies (C5 allocation pressure) | `internal/analysis/callgraph/graph.go:790`, especially `kept := cs[:0:0]` at line 805; callers in `graph.go:772` and `replace_package.go:267` | Compact adjacency in place on the exclusively owned new graph, or allocate only after the first actual removal. Currently every nonempty retained list allocates afresh because its initial capacity is zero, even when no edge is removed and the replacement slice is discarded. Prove ownership/aliasing first. This targets needless copies, not all 30.26% madvise or all C5. The observed pruning ancestry itself is only 0.25%. |
| P2 Remove the second syntax-resolution pass (C4/C5) | `internal/mcp/resolved_nav.go:204-210` requests syntax then BOTH; `internal/analysis/callgraph/cache.go:204` implements BOTH as `Build(nodes)` then `AddTypedEdges`; full clone helpers already exist in `replace_package.go:272-346` | Seed BOTH from an isolated clone of the complete resident syntax graph rather than invoking Build again. Reuse existing clone helpers and deep-copy every mutable map/slice before typed augmentation. This removes repeated resolver/index construction and sorting, while retaining the mandatory first syntax build, cloning, typed augmentation and both complete immutable outputs. No additional retained cache is needed. Benefit and memory cost remain unmeasured. |
| P3 Remove non-covering typed-row lookup/sort work (C3) | `internal/store/sqlite_typededges.go:63-87` reads five fields ordered by from/to/kind; the current from-symbol index in `sqlite_schema.go:34` is only `(service_id, from_symbol)`; sampled stacks include SQLite table-moveto/B-tree traversal | A replacement covering ordered index `(service_id, from_symbol, to_symbol, kind, from_service, from_file)` is the smallest source-grounded storage proposal for eliminating that table-lookup/order component if selected by the planner. It does not eliminate reading 256,033 rows or all C1/C3/Pread cost. It adds index bytes and ingest work and needs a separate cost/read-law review and actual query-plan/latency proof. This lane obtained no EXPLAIN result and claims no tested saving. Do not enlarge the bounded SQLite page cache implicitly. |
| P4 Remove only the empty-array JSON decoding branch (C1 decode) | `internal/store/sqlite_symbolnodes.go:91` calls `json.Unmarshal` for calls; line 83 explicitly preserves every original row/JSON failure | An exact-byte `callsJSON == "[]"` fast path could return the same non-nil empty slice; all other values, including null and malformed data, must keep existing decoding/errors. That would remove only decoding of that canonical empty value. With zero observed JSON samples there is no measured reason to prioritize it, and no new decoder or dependency is justified. |

C1's complete observation read and C2's shared snapshot indexing remain required
on a genuine cold miss. Removing them entirely requires a different
representation or still-valid resident state, not an honest local shortcut.
P1/P2 reduce allocation inputs to the runtime and may also reduce C6 GC work;
profiling alone does not quantify that reduction. P1 is the narrowest local
removal, P2 targets a visibly repeated pass, and P3 is a storage tradeoff rather
than a free speedup. None proves the one-second bar. The separate W6-1d
contract author owns the refresh-worker alternative and its freeze question.

### Reproduction and raw artifacts

All new raw artifacts remain in ignored `workspace/tmp/w6-profile/`; every W5
artifact remains where it was. The retained `prepare.py` produces the exact
profile-only diff; `run.sh` builds and runs the five predetermined captures.
The invoking shell sourced the lane environment and ran
`scripts/gate-lease.sh bash workspace/tmp/w6-profile/run.sh` with the shared
wave `FATHOM_GATE_LOCK` and `FATHOM_GATE_NICE=off`. The script retained the
active lease-holder receipt. Heavy verification reacquired that same lease.

```sh
# Within ignored source/, with the same compiler/runtime environment as W5:
go build -trimpath -buildvcs=false -o ../bin/index-profile ./cmd/w5read
# In a new process per capture, original-store is the unchanged W5 SQLite file:
/usr/bin/time -lp bin/index-profile original-store results/<capture> index-profile
go tool pprof -proto bin/index-profile results/*/cpu.pprof > merged.pprof
go tool pprof -raw bin/index-profile merged.pprof > reports/raw.txt
go tool pprof -tags bin/index-profile merged.pprof > reports/tags.txt
go tool pprof -top -cum -nodecount=60 bin/index-profile merged.pprof
go tool pprof -top -cum -nodecount=40 -tagfocus=phase=syntax bin/index-profile merged.pprof
go tool pprof -top -cum -nodecount=40 -tagfocus=read=typed bin/index-profile merged.pprof
python3 parse-profile.py
python3 verify.py
```

`parse-profile.py` parses every raw sample, stack and label, derives the
disjoint partition and separately derives cumulative function unions without
double counting recursive/inlined frames. `verify.py` independently compares
all complete output files, validates admission/miss counters, checks source
differences and verifies the original W5 manifest and store. Its output:

```text
W5 preserved: PASS (137 manifest entries)
profile-only source difference: PASS ['internal/mcp/resolved_nav.go', 'internal/mcp/w5_reading.go']
profiles: PASS (5 admitted complete cache misses, no invalid/retried/discarded captures)
required boundary: PASS (snapshot 1 miss, syntax+BOTH 2 misses, typed 1 miss; all complete)
output equality: PASS (5 captures x 5 complete populations; exact bytes equal W5)
store identity: PASS (original W5 store unchanged)
CPU samples: PASS (793 samples x 10000000ns; resolved and serialization excluded)
```

| Capture | Original CPU profile SHA-256 | Raw result JSON SHA-256 |
|---|---|---|
| 01-index-profile | `48e0629b5c6e2e5db7b4397e272016cd82cf95d7c1a0a26bfdaaeaa5f279ba54` | `fd9ba5cd9ef2e7bed1940146a67f58910a81191c284c384d553e897e90ca4637` |
| 02-index-profile | `855b04bc964e74f689f74708e65623a75bf1355a50ef6553c2f6f64d6032ebb7` | `1aaf198541dffd6001991faba84e70c80a2b47c4d0a84fa1b33ebc5736e2ca30` |
| 03-index-profile | `9b078ee7209a7268a676a0f0f9f7ae9ebfdc95a836d1e8bd53e9ae25a49e6059` | `935ebe3b96bba4c1afb6008585f3a6a03ea4751f59e8a070b376405dd034b8f0` |
| 04-index-profile | `d49c2d984e70f9a85f593de0fb5a7c9d6d2b79228bf3c256eec5bae0555e1d74` | `f3128ae8ea9c20a47943d9dbff8e5549ffbe07706df5aab359aeaf6b8d48d5c6` |
| 05-index-profile | `7f260f9745f8a81355a2bf01194e457cb50702d23f8baf6e117070e5da7fe31d` | `200d681c005a7ff081c8d77fbee94a3806ffbf56c69f51f89227153df1b07e27` |

| Artifact | SHA-256 |
|---|---|
| `identity.json` | `f91eba861eb56aa64359867a6da7e45f085d4e6e9627a5820033510fc1152619` |
| `bin/index-profile` | `e13dd26a5b354e0709fbbfd1e536c8db52dda6148eb5f523cc52d5ad446b085a` |
| `merged.pprof` | `8a4e036a7c08be043a2b0293b93e848076bc4f9b6e5187b607e61c989085d7ad` |
| `profile-only.diff` | `efaac630b52db308704dd7587ea506c4d2b3b645104598ff14e33ad5f2afbaf8` |
| `source/internal/mcp/w5_reading.go` | `3687d88c03fba5b2d615c18aa873b67c5d6e6b8ae0d4af296266bf06835e2c75` |
| `source/internal/mcp/resolved_nav.go` | `382bd6b2ba37f6baec4296ead9087ef8b59188ecee0640c01e4c3ee74c5927d6` |
| `prepare.py` | `35efd4fe3e396db27ae1bccb046492d8ffa2cf3ae88957fd8045b3b95f8b1d47` |
| `run.sh` | `248f33c4a52a47d8bc6794af3c7486285fa3b5afc17f7183fb8224dd92c5bcae` |
| `parse-profile.py` | `edbbc4744f1762a7eba76ad9956624ff511efca2729bfb62ae2b43b062be8e79` |
| `verify.py` | `6410ab0bb1a6627ff8f0479b3972039e10271e8776c6dc740244a8f53c4f417c` |
| `verify.log` | `021e5f6a8f4000e8f19e1afd883b92c900ecc56ee27023b8abbd765128b8aa50` |
| `captures.json` | `c1ae3bbd966a75ae6a90a1692a8138cb9d3d144b7347e20a4b45b8c2a1e4630f` |
| `sample-analysis.json` | `9a846f155c16190287bf4fb3e0fa61f0a7a109077958810428dbce7d33307832` |
| `reports/raw.txt` | `6995e82c5c2bfd7a8486f89568c7a50a79039ff9bc129fe88c3a6237eb7f9120` |
| `reports/tags.txt` | `6def1171d29df1df943c6290ec7b89b047bd698435139e67398f018542ed48c8` |
| `reports/top-flat.txt` | `873befe6e7b352d5f570053e895bf85d8b10089735c08a555ef03bd81c766aa5` |
| `reports/top-cum.txt` | `858bb412b9e04c2fb444c8b41edc6adc3dc00955c0356f4b905effdf633bad88` |
| `build-info.txt` | `002d41d948cef33dafb32eab915b78b15cd0733e7328b06bbb94f09ee136b2c7` |
| `lease-holder.txt` | `781c931b44805597e26b980447d7745a54e3e84b611d2804041857da216740fa` |
| `inspection-attempts.txt` | `3cd57d5ff196fc186d20fd0a24bfeb0556be5bee17fb8e5f441fad16838dea07` |
| `environment-before.txt` | `aff823978ebd58c1f7ac1179ad025e12841f07e312b28c7ff4b0d70a36a44237` |
| `environment-after.txt` | `127c1b87edb6e9c5e9fa386bec5dcb9f58a83738b6b11c864441a2ab4e8affe5` |
| `SHA256SUMS` | `6924f425ad488b9aa43fe5370c6f92065fd0aa56cdb6f139d3506e8d325d26cd` |

Ambient snapshots reported no recorded thermal/performance warning and
unchanged host swap-in/out counters. One-minute host load average was 10.45
before preparation and 14.05 after the captures; background activity was not
stopped. One optional read-only Python SQLite EXPLAIN attempt failed with
`unable to open database file` before producing a plan. It did not change the
input or trigger a repeated profile and supplies no evidence for P3. The scope
tag-focus report matched zero samples, disclosed above. There were no failed
builds or failed profile captures.

**Escalate these measured costs.** No #251 implementation or dispatch, deploy,
restart, push, tag, model spend, new table/cache/job, plan claim or full gate
was performed. Deployed ENG-1 remains open. Separate critic review and any
implementation decision remain with the controller.
