# Tool-oracle call-scope compatibility

Measured against `285002448`: an unscoped empty `calls` list contributes no calls family. A nonempty explicit `scope` asserts every named file, including zero-call files. This preserves existing producer contracts while fixing precision on scoped zero-call files.

## Fan-out and stored-count regrades

At this commit the tracker loads 118 flat `docs/bench/readings/lang/*.json` files. Nested historical/data files are excluded. The only committed call-oracle producer with a top-level array `scope` is `calibration/oracles/dxc-ast`; only the existing HLSL reading carries its scope disclosure. SQL, PHP, CSS, HTML and Tailwind do not gain calls families from unscoped empty lists.

Scratch copies of those five committed readings were run through `lang-regrade -commit 285002448`. Every decoded field except the added regrade note matches exactly: grades, level, metrics, counts, subjects, families and tenancy are unchanged. Canonical readings were not rewritten. This is a stored-count check, not a fresh extraction or producer run.

| Reading | Stored-count regrade |
|---|---|
| SQL | L2 -> L2 |
| PHP | L4 -> L4 |
| CSS | L3 -> L3 |
| HTML | L3 -> L3 |
| Tailwind | L3 -> L3 |

Reproduce on scratch copies, never the canonical files:

```sh
mkdir -p workspace/tmp/call-scope-regrade
for id in sql php css html tailwind; do
  cp "docs/bench/readings/lang/$id.json" "workspace/tmp/call-scope-regrade/$id.json"
done
go run ./cmd/indexbench lang-regrade -commit 285002448 workspace/tmp/call-scope-regrade/{sql,php,css,html,tailwind}.json
```

## Producer compatibility

Pinned sqlglot 27.8.0 was run on Pagila `9baf49c4149e43229f6021e6218d6b2ac8ef4f34`, using the existing measured set `pagila-schema.sql,pagila-schema-jsonb.sql,pagila-temporal-setup.sql`. It emits zero calls without a scope, including its unsupported-statement fallback. The corrected harness reads all three measured files and records no calls family or call counts for that oracle. The empty list is not scored as zero-call truth. This replay checks calls only; L1 and range oracles were not rerun.

```sh
python3 -I calibration/oracles/sqlglot/sql_refs_oracle.py workspace/calibration/scope-compat/sql/pagila pagila-schema.sql pagila-schema-jsonb.sql pagila-temporal-setup.sql > workspace/tmp/sql-pagila-calls.json
go run ./cmd/indexbench lang-oracle -dialect sql -subject workspace/calibration/scope-compat/sql/pagila -subject-name devrimgunduz/pagila -only pagila-schema.sql,pagila-schema-jsonb.sql,pagila-temporal-setup.sql -ctags workspace/tmp/absent-ctags -tool-oracle workspace/tmp/sql-pagila-calls.json -out workspace/tmp/sql-pagila-compatibility.json
```

A second real sqlglot replay used Sqitch `ce0550c45ade3527d0c7f981c934e1ea4bdd6c19` and the existing 28-file SQLite measurement set. It preserves the nonempty calls family: 14 truth pairs and 14 Fathom pairs, all matched, at tier C. The harness discloses 22 oracle entries outside the measured files. Together the two subjects reproduce the existing SQL call counts without treating Pagila’s empty list as an assertion. This is a call-only control, not a full SQL remeasurement.

```sh
python3 -I calibration/oracles/sqlglot/sql_refs_oracle.py --dialect '*sqlite*=sqlite' workspace/calibration/scope-compat/sql/sqitch > workspace/tmp/sql-sqitch-calls.json
go run ./cmd/indexbench lang-oracle -dialect sql -subject workspace/calibration/scope-compat/sql/sqitch -subject-name sqitchers/sqitch -only 'lib/App/Sqitch/Engine/sqlite.sql,lib/App/Sqitch/Engine/Upgrade/sqlite-*.sql,t/lib/upgradable_registries/sqlite.sql,t/engine/**/*.sql,t/sql/**/*.sql' -ctags workspace/tmp/absent-ctags -tool-oracle workspace/tmp/sql-sqitch-calls.json -out workspace/tmp/sql-pagila-compatibility.json
```

The command-level regressions also pass a nonempty calls oracle followed by a PHP-style definitions-only `calls: []` file. The second file neither triggers the duplicate-family refusal nor changes call counts or metrics. The current PHP reading uses nonempty php-parser calls at tier B, whose unscoped coverage remains calls-derived. No PHP executable, Composer environment or pinned SCIP indexes were available here, so no complete PHP producer/extraction replay is claimed.

Runnable regression and stored-count proof:

```sh
go test -race ./cmd/indexbench -run 'TestScoreToolOracle(CallScope|DefinitionsOnlyBesideCalls)|TestCmdLangOracle(UnscopedEmptyCallsIgnored|DefinitionsOnlyBesideCalls)|TestCmdLangRegradeCallScopeUnaffectedReadings' -count=1
```

## HLSL fresh replay

The [public Linux replay](https://github.com/GeoffMillerAZ/fathom/actions/runs/40407275332) ran on Ubuntu 24.04 with DXC v1.9.2602 across the pinned three-subject corpus. The #45-disabled control reproduced the committed semantic counters, metrics, scope and grades. The main harness measured the same 543 DXC-scoped files of 644 measured files. The only semantic metric change from control was call precision, 0.9656 to 0.956 (delta -0.0096): 39 additional false positives in explicitly scoped files with no oracle calls, 9 in directx-graphics-samples and 30 in kajiya. Fathom call pairs rose 3920 to 3959; matched pairs stayed 3785 and truth pairs stayed 3839, so call recall stayed 0.9859. L1 excellent, L2 good and L3 good remain unchanged. The [canonical HLSL reading](lang/hlsl.json) now contains the main-run values and its Linux timing, with a note linking the public run. Historical-to-fresh timing and note differences are environmental or producer-description drift, not effects attributed to #45.

## R-D19 integration regrade

Main `5d41806d0` was merged once into this branch. The conflicting reading
was regenerated from the complete frozen [public replay](https://github.com/GeoffMillerAZ/fathom/actions/runs/40407275332)
reading with main's committed `indexbench lang-regrade`, rather than merging
reading fields between parents. The same harness regraded scratch copies of
both parents as controls.

D19 changes no HLSL values: import-resolution, file-edge-precision and
file-edge-recall denominators are 998, 597 and 1,222, all above 20. HLSL
declares no extra manifest-specific evidence bar. L3 good and every other
grade remain unchanged. The scope replay still contributes only the 39 extra
Fathom calls to semantic counters: +9 directx-graphics-samples, +30 kajiya,
3,920 to 3,959 pooled; precision .9656 to .956, recall .9859 unchanged.

The frozen replay retains its Linux timing and provenance, including
1.30 MB/CPU-s and maximum 25 ms. Main's separately recorded Apple M4 retime
is 1.92 MB/CPU-s and maximum 19 ms. These existing G-R9 source differences
and producer-note differences are not attributed to D19 or the scope rule.
No producer or extractor was rerun here.

```sh
bash scripts/fathom-go.sh run ./cmd/indexbench lang-regrade -commit 5d41806d0 docs/bench/readings/lang/hlsl.json
bash scripts/fathom-go.sh test -race -count=1 -run 'Test(LevelMetL3Evidence|L3DenominatorCoversEveryL3Metric|LevelMetManifestL3Evidence|CommittedReadingsRoundTrip|CommittedReadingsAreCanonical|CmdLangRegrade|ScoreToolOracleCallScope|CmdLangOracleEmptyCallsScopeBlocksL2|CmdLangOracleScopeReportsZeroTruthFile)' ./internal/langbench ./cmd/indexbench
```
