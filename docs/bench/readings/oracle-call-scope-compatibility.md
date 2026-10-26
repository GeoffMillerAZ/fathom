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

## One open regrade: HLSL

HLSL requires fresh DXC scope and pair output; stored counts cannot recover newly scored zero-call files. Run the exact replay with pinned DXC `1.9.0.5184`:

```sh
bash calibration/oracles/dxc-ast/reading.sh docs/bench/readings/lang/hlsl.json
```

This ENGINE change must not land before the HLSL result returns. No HLSL metric or level is asserted by this report.
