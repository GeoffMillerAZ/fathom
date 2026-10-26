# nginx native upstream population oracle

Bench-only; no product dependency, server, backend request or Docker.
The completed r3 diagnostic measures L2 excellent: 1/1 references at A.
Definitions remain C (71/71 scored); L1 good. The canonical L3 reading is
unchanged and awaits the controller's D19 gate.

| Field | Authority | Source |
|---|---|---|
| Same-file literal upstream bindings | A | nginx 1.31.6 native HTTP/stream registration object identities, full-input parsing and target-only negative controls |
| Definition names/positions and enclosing owner labels | C | independent crossplane 0.5.8 walk; no native position claim |
| Configuration acceptance | A | native `nginx -T` on an isolated profile |
| Cross-file/include/route grading | unchanged | legacy canonical reading; not regraded by this experiment |

The observer wraps `ngx_http_upstream_add` and `ngx_stream_upstream_add`
without changing their original bodies or return values. It captures every
registration during successful configuration testing, including unused and
forward declarations. Native returned-object identity selects bindings before
comparison with scanner references. The walk labels lexical owners only;
native-only pairs are retained. Unknown events or ambiguous ownership refuse
the whole profile. This enumerates literal configuration bindings, not visited
request traces. Each local pair must survive the original profile and fail a
target-only declaration rename at the original source location. Native
zero-default-port passes report `no port in upstream`; ordinary HTTP passes
report `host not found in upstream`. Both must name the original target.

## Population and exclusions

All 47 historically disclosed native inputs load. The unchanged ingest gates
score 35 files: 12 inputs and their 12 raw definition entries are refused as
ambiguous content. These are existing exclusions, not a new scope reduction.
Native enumeration covers them too; none contains a local upstream reference.
The raw independent definition output has 83 entries; 71 are scored. The
original 71-definition gate remains unchanged. Full inventories stay scratch.

| Cohort | Native inputs | Scored files | Scored definitions found/truth | Native/scored local references |
|---|---:|---:|---:|---:|
| awesome-compose |1|1|4/4|0/0, no reference evidence|
| nginx |2|2|5/5|0/0, no reference evidence|
| nginx-demos |11|11|30/30|1/1 A|
| server-configs-nginx |33|21|32/32|0/0, no reference evidence|

The diagnostic keeps the union of conflicting historical scope disclosures;
selecting the narrower one previously hit 61 definitions and failed the gate.
`native-population-r1.json` records all per-subject counts and exact normative
tracked-set before/after hashes. They agree, with zero generated/untracked
inputs, symlinks or gitlinks. The tracked approximation includes other formats
and files outside the historical native scope; it is not a measured-file list.
Three warm timings are 9.58, 9.64, 9.86 MB/CPU-s (median 9.64), zero faults.
One genuine binding and about 34 KB support no broad operational coverage claim.

## Reproduce

Set private TMPDIR/GOTMPDIR/GOCACHE/GOMODCACHE, keep GOWORK=off, check
`df -h ~` and `uptime`, require 20 GiB free and load<=6, and use one serial
nice -n 10 run. Install requirements only in a private venv. The committed build
driver pins nginx 1.31.6, OpenSSL3.6.2 and PCRE2-10.47 archives/checksums.

```sh
python3 -m venv workspace/oracle/nginx-venv
workspace/oracle/nginx-venv/bin/pip install --only-binary=:all: -r calibration/oracles/nginx/requirements.txt
nice -n 10 python3 -B calibration/oracles/nginx/build_native.py -cache workspace/oracle/nginx-build -observe-upstreams
install -m700 workspace/oracle/nginx-build/nginx-1.31.6/objs/nginx workspace/oracle/nginx-observer
bash scripts/fathom-go.sh build -o workspace/tmp/indexbench ./cmd/indexbench
workspace/oracle/nginx-venv/bin/python -B calibration/oracles/nginx/grade_native_l2.py -native-population -indexbench workspace/tmp/indexbench -python workspace/oracle/nginx-venv/bin/python -nginx workspace/oracle/nginx-observer -build-receipt workspace/oracle/nginx-build/build-receipt.json -subjects workspace/calibration/nginx-native -out workspace/tmp/nginx-population
workspace/oracle/nginx-venv/bin/python -B calibration/oracles/nginx/probe_controls.py -nginx workspace/oracle/nginx-observer -build-receipt workspace/oracle/nginx-build/build-receipt.json -out workspace/artifacts/lang-nginx/native-controls
```

Keep the receipt, then remove source builds/archives after packaging and delete
subjects once measured. This lane already pruned both. A rebuilt binary gets
its own measured checksum; `native-observer-build-r1.json` records this run's
source, shim and binary identities. No system installation or daemon is used.

## Control evidence and remaining work

All 42 authored tests passed with native tests enabled, zero skips. HTTP and
stream, unused/forward declarations, scanner omission, exact identity and
scratch cleanup are covered. `probe_controls.py` runs the actual fathom
extractor under `-race`; deleting all 5 emitted nodes leaves raw native truth
unchanged, while changing the input binding changes truth. Filtering native
pairs through scanner refs makes the focused omission test FAIL; restored
code passes. `native-controls-r1.json` records hashes and the FAIL line.

Caps: `test_population.py` refuses the whole profile at 10,001 events against
10,000 and 8,388,609 stderr bytes against 8,388,608. The native CLI always runs
fresh authored controls on its exact checksum-verified binary before A output.

Source positions are still an open native-parser instrument, not an
availability ceiling. Remaining X-R4 candidates for a future L1 ceiling need
complete versioned search receipts. The native L2 availability trigger has
fired; L3/include work remains held until D19. No product, epoch, generated
tracker or checkbox change is part of this diagnostic.


## Open L1 native lexer follow-up

The separate L1 branch adds `-observe-tokens` to the pinned build driver,
alongside `-observe-upstreams`. `native-token-build-r1.json` freezes the measured
binary and native lexer/shim identities before configuration queries.
The observer reads the native first-word start line and block/file end counters;
stripping its read-only hook and wrapper exactly restores the original parser.
No independent parser chooses the token population. `native_tokens.py` refuses
unknown/incomplete events, missing boundaries and cap overflows.

Authored controls cover multiline/inline ranges, data blocks, unused entities,
quotes, non-ASCII arguments and final-newline differences. Native definitions
and owners survive omission by the independent walk. Caps are 10,000 tokens,
64 arguments including the directive, 512 bytes per argument/path, and 8 MiB
stderr; exact cap-plus-one controls refuse the entire profile. Native definition
names changed by scratch-path rewriting also refuse instead of guessing an
inverse. The canonical full-input measurement remains pending at this commit.

```sh
nice -n 10 python3 -B calibration/oracles/nginx/build_native.py -cache workspace/oracle/nginx-token-build -observe-upstreams -observe-tokens
install -m700 workspace/oracle/nginx-token-build/nginx-1.31.6/objs/nginx workspace/oracle/nginx-tokens
NGINX_TOKENS="$PWD/workspace/oracle/nginx-tokens" python3 -B -m unittest discover -s calibration/oracles/nginx -p 'test_native_token*.py'
workspace/oracle/nginx-venv/bin/python -B calibration/oracles/nginx/grade_native_l2.py -native-population -native-definitions -indexbench workspace/tmp/indexbench -python workspace/oracle/nginx-venv/bin/python -nginx workspace/oracle/nginx-tokens -build-receipt workspace/oracle/nginx-token-build/build-receipt.json -subjects workspace/calibration/nginx-native -out workspace/tmp/nginx-native-positions
```

Use the same disk/load/serial guards and private caches as above. Package the
build receipt and small binary, then prune completed source builds/downloads.
No L3/canonical reading, extractor, engine, epoch or checkbox change follows.
