# The doc-xref pass reads code for Lambda only where one is declared (EW-57, EW-58)

**Verdict: keep.** In a repository with no Lambda function, a code edit no longer re-runs the doc-xref
pass. On zulip 12.3 (Python and TypeScript, no Lambda), each one-line Python edit had cost 1.06 to
1.20 s of doc-xref time per ingest since C3 (c68b5097). That cost is now 0: the pass is skipped, as it
was before C3 (70afc4a9). The edit hook's one-file refresh no longer invalidates the pass either, so
the ingest after it skips the pass too (0.86 to 1.20 s before, 0 after). The gate costs one store
read of the three declaring kinds' rows, timed in the ingest at 66 to 73 ms on zulip and 18 ms on
fathom.

fathom pays nothing as well. Its CloudFormation, SAM and Terraform Lambda fixtures sit under
`testdata/`, which discovery skips, and `calibration/` declares no function. The ingested fathom
tree therefore holds no declaring row (0 rows, checked by SQL below), and its 0.61 to 0.63 s pass on
a code edit is skipped. No sharper rule (scoping the input to a function's code directory) was built:
the numbers show no remaining cost to remove, and such a rule would need storage this wave freezes.

Stored output is identical. After the same edit and refresh sequence, the sorted `typed_edges` (doc-xref
included) and `observations` of c68b5097 and the after binary hash the same on both subjects. No
`ExtractorEpoch` bump is needed.

Where a function is declared, a code edit is still an input. Tests `TestReingestFile_LambdaHandlerParity`
(SAM), `TestReingestFile_TerraformLambdaHandlerParity` (Terraform) and
`TestIngest_LambdaTemplateAndHandlerBindTogether` cover that case.

EW-58 reading: zulip 12.3 (b3198225) Python edit: doc-xref pass 1.06-1.16 s at c68b5097, skipped at 70afc4a9 and after (gate read 66-73 ms); post-refresh ingest 0.86-1.20 s at c68b5097, skipped after; fathom (c68b5097 tree) 0.61-0.63 s at c68b5097, skipped after (gate read 18 ms); stored output identical.

## Subjects and binaries

| What | Identity |
|---|---|
| zulip | `zulip/zulip` tag `12.3`, `b31982251ff82d0b02e59f63ad21f19204e37aea`. 9,061 files: 2,005 `.py`, 464 `.ts`, 2,694 files of the nine handler-source extensions, 3,494 `.md`/`.yaml`/`.json`/`.toml`. No CloudFormation, SAM or Terraform Lambda declaration (grep over the tree: none) |
| fathom | this repository at `c68b5097d8a1fcde4940220be2299579a680eb87` (a clone). 10,184 files, 598 of the nine extensions; its Lambda fixtures are under `testdata/` (not discovered) |
| before C3 | `70afc4a929f3bad79919bbe7aad7a8b4097ccd91` |
| C3 | `c68b5097d8a1fcde4940220be2299579a680eb87` (origin/main at the wave's start) |
| after | `c37a693ad18067bd28e5702edb79ef15d00d53e4` (this branch: the gate, cbac3246, and its span, c37a693a) |
| machine | Apple M4, 10 cores, 32 GiB, go1.26.3. Swap was near full (14.3 of 15.4 GB) and other waves ran beside this one: each row carries its 1-minute load average |

Edited files: zulip `zerver/lib/utils.py`, fathom `calibration/adjudications/claims.py`. The edit is
one appended comment line.

## Numbers

Wall and CPU (user + sys) are `/usr/bin/time -l` over the whole `fathom ingest`. The doc-xref and
`lambda_gate` columns are the ingest's own `pass_times`, wall/CPU in seconds. "skip" means the pass did
not run. "refresh" is `POST /refresh-file` on a scratch `serve-http` daemon of the same binary and
store, the edit hook's one-file refresh path (ReingestFile). The post-refresh ingest follows it.

### zulip 12.3

| Binary (run) | Full ingest wall / CPU / peak RSS | Edit ingest wall / CPU | doc-xref on edit | `lambda_gate` | refresh | Post-refresh ingest wall / CPU | doc-xref after refresh | Load (edit) |
|---|---|---|---|---|---|---|---|---|
| 70afc4a9 | 45.96 s / 33.75 s / 1470 MiB | 8.67 s / 8.18 s | skip | — | 0.32 s | 3.12 s / 3.51 s | skip | 11.7 |
| c68b5097 | 39.14 s / 32.18 s / 1324 MiB | 8.67 s / 9.11 s | 1.160 / 1.096 | — | 0.31 s | 4.64 s / 4.92 s | 1.204 / 1.109 | 11.9 |
| c68b5097 (r2) | 32.87 s / 27.84 s / 1342 MiB | 6.59 s / 7.32 s | 1.060 / 0.952 | — | 0.23 s | 3.18 s / 3.56 s | 0.858 / 0.861 | 6.8 |
| after | 33.67 s / 28.07 s / 1338 MiB | 5.32 s / 6.33 s | skip | 0.066 / 0.067 | 0.31 s | 2.64 s / 3.10 s | skip | 5.0 |
| after (r2) | 39.75 s / 31.60 s / 1276 MiB | 6.13 s / 6.97 s | skip | 0.073 / 0.072 | 0.29 s | 2.26 s / 2.99 s | skip | 6.6 |

### fathom (the c68b5097 tree)

| Binary (run) | Full ingest wall / CPU / peak RSS | Edit ingest wall / CPU | doc-xref on edit | `lambda_gate` | refresh | Post-refresh ingest wall / CPU | doc-xref after refresh | Load (edit) |
|---|---|---|---|---|---|---|---|---|
| 70afc4a9 | 67.89 s / 58.29 s / 1831 MiB | 9.30 s / 9.08 s | skip | — | 0.30 s | 2.16 s / 2.48 s | skip | 7.7 |
| c68b5097 (cold Go cache) | 200.58 s / 321.13 s / 1765 MiB | 10.98 s / 10.43 s | 0.609 / 0.566 | — | 0.41 s | 2.95 s / 3.11 s | 0.599 / 0.546 | 11.4 |
| c68b5097 (warm) | 63.49 s / 56.27 s / 1853 MiB | 8.47 s / 8.82 s | 0.628 / 0.586 | — | 0.28 s | 2.59 s / 3.01 s | 0.595 / 0.555 | 6.4 |
| after | 53.95 s / 52.68 s / 1906 MiB | 6.65 s / 7.01 s | skip | 0.018 / 0.018 | 0.27 s | 1.92 s / 2.28 s | skip | 4.4 |

How to read them:

- The doc-xref span is the clean measure: the gate changes nothing else. The ingest totals move
  with load and with the centrality pass (3.3 to 5.3 s of each edit ingest), which this change does
  not touch.
- The first c68b5097 fathom full ingest ran with a cold shared Go build cache. Its typed Go pass took
  156.5 s wall, and its `go list` children took the CPU. The warm rerun is the comparable row. A full
  ingest runs the doc-xref pass in every binary (0.65 to 1.67 s): the gate is about incremental ingests.
- `lambda_gate` appears only where the gate read the store. That is the edit ingest, whose only
  changed file was a handler source. The post-refresh ingest has no changed file, and the refresh
  made its one read inside the daemon (ReingestFile records no pass times).
- The cost of the read itself, with sqlite3 on the same stores: `SELECT ... FROM observations WHERE
  service_id = ? AND source_kind IN ('CFNResource','TerraformResource','TerraformModule')` took 30 to
  40 ms warm over zulip's 222,448 rows. It took 0.91 s the first time, after the store had sat idle
  under the swap pressure above. On fathom (50,378 rows) it took 10 to 20 ms warm and 0.32 s cold. The
  read scans the service's rows through `idx_observations_service_id`, because no index has the kind,
  and a new one would break this wave's table freeze. It runs at most once per ingest or refresh, and
  only when a handler source changed while the pass's record still stood.

Stored output, after the same full, edit, refresh and ingest sequence (sha of the sorted rows):

| Subject | `typed_edges` rows | `typed_edges` c68b5097 / after | `observations` c68b5097 / after |
|---|---|---|---|
| zulip | 56,602 | 19d87004c21b / 19d87004c21b | f900f1728475 / f900f1728475 |
| fathom | 161,211 | 6ac21f3d5ca7 / 6ac21f3d5ca7 | 0accef385d08 / 0accef385d08 |

## What the gate is

A code file a handler binding may name (`.py`, `.js`, `.mjs`, `.cjs`, `.ts`, `.mts`, `.cts`, `.java`,
`.rb`) re-runs the doc-xref pass only when the service declares a Lambda function. A declaration is
a stored symbol `isLambdaDeclaration` reads as one:

- a `CFNResource` signed `AWS::Serverless::Function` or `AWS::Lambda::Function`;
- a `TerraformResource` named `aws_lambda_function.*`;
- a `TerraformModule` whose signature names `lambda`.

`ResolveLambdaHandlers` reads the same predicate. The gate applies in the same way to the one-file
refresh's invalidation (`passInvalidator.keysFor`).

The answer is one read (`ObservationsOfKinds`) memoized on the ingest's or the refresh's invalidator.
It is made only while the pass's record still stands. A document, an image-build input or a Jenkins
input changed in the same ingest invalidates the record first and needs no read.

Ordering: the invalidator reads mid-extract. When no document changed, no file a declaration can sit
in changed either, so that answer equals the store's after the extract phase. When a template and its
handler arrive together, the template is a document, so the pass runs and binds. A store without the
read, or a failed read, answers "declared" and the pass runs, as before.

Limits, disclosed (O766):

- A GitHub Actions run step or a Jenkins shell step can name a `.py`, `.js` or `.rb` script that
  defines functions (LB-F310, LB-F374). In a service with no Lambda function, an edit of that script
  re-runs the pass again only when a document changes. That is how 70afc4a9 behaved: C3 had covered
  this case by accident.
- The read scans the service's rows. It is cheap warm and slower cold, as measured above.

## Commands

Binaries (each from a clean checkout, in the wave's environment `genv-wave.sh h3-xref`):

```sh
git worktree add --detach $W/src-70afc4a9 70afc4a9 && (cd $W/src-70afc4a9 && go build -o $W/bin/fathom-70afc4a9 ./cmd/fathom)
git checkout c68b5097 && go build -o $W/bin/fathom-c68b5097 ./cmd/fathom
git checkout c37a693a && go build -o $W/bin/fathom-after ./cmd/fathom
git clone -q --depth 1 --branch 12.3 https://github.com/zulip/zulip.git $W/subjects/zulip
git clone -q <this repository> $W/subjects/fathom && git -C $W/subjects/fathom checkout --detach c68b5097
```

Each (binary, subject) pair, with `$B` the binary, `$ST` a fresh store, `$S` the subject and `$F` the
edited file. One ingest ran at a time across the wave (a `mkdir` lock).

```sh
/usr/bin/time -l $B ingest --store $ST --service $SVC --path $S --json > full.json    # full
/usr/bin/time -l $B ingest --store $ST --service $SVC --path $S --json > noop.json    # nothing changed
echo "# h3-xref edit 1" >> $S/$F
/usr/bin/time -l $B ingest --store $ST --service $SVC --path $S --json > edit.json    # one-line edit
echo "# h3-xref edit 2" >> $S/$F
$B serve-http --store $ST --addr 127.0.0.1:$PORT --call-log= --idle-exit 5m &         # scratch daemon
curl -s -X POST http://127.0.0.1:$PORT/refresh-file -d "{\"service\":\"$SVC\",\"file\":\"$F\"}"
kill %1
/usr/bin/time -l $B ingest --store $ST --service $SVC --path $S --json > post_refresh.json
git -C $S checkout -- $F
```

The doc-xref and `lambda_gate` figures are `.pass_times[] | select(.name == "doc_xref" or .name ==
"lambda_gate")` of each JSON. The no-declaration check on the fathom store:

```sh
sqlite3 $ST "SELECT count(*) FROM observations WHERE service_id='fathom-subj' AND ((source_kind='CFNResource' AND claim IN ('AWS::Serverless::Function','AWS::Lambda::Function')) OR (source_kind='TerraformResource' AND symbol GLOB 'aws_lambda_function.*') OR (source_kind='TerraformModule' AND instr(claim,'lambda')>0))"   # 0
```

The identity check: `sqlite3 $ST "SELECT * FROM typed_edges WHERE service_id='$SVC' ORDER BY 1,2,3,4,5,6" | shasum`,
and `SELECT source_ref, source_kind, symbol, package, claim, calls, line, end_line FROM observations WHERE service_id='$SVC' ORDER BY 1,2,3,4,5,6,7`.
