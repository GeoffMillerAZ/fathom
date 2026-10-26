# nginx native L1/L2 diagnostics

Bench only; no product dependency, server, backend request or Docker.
The completed native lexer/upstream diagnostic measures L1/L2 excellent:
71/71 scored definitions, 71/71 exact ranges and 1/1 references, all A.
The canonical L3 reading is unchanged and awaits the controller's D19 gate.
The earlier r3 upstream diagnostic supplied C definitions; its evidence stays
historical, never relabelled.

| Field | Authority | Native source |
|---|---|---|
| Definition names and start/end ranges | A | complete ngx_conf_read_token arguments, evaluated first-word start_line, block-close and file-end counters |
| Reference owners | A | enclosing entities from the complete native token tree |
| Same-file literal upstream bindings | A | native HTTP/stream registration returned-object identities, followed by target-only ablation |
| Configuration acceptance | A | successful isolated nginx -T |
| Cross-file/include/route grading | unchanged | canonical L3 not regraded |

The lexer wrapper preserves native decisions and return values. Stripping its
read-only first-word observation and wrapper rename exactly restores the
checksum-pinned original parser. The upstream observer likewise preserves both
original registration function bodies and results. Native object identities
select reference targets; no scanner chooses the definition or pair population.
The independent crossplane walk is compared only afterwards. Authored omission
controls retain native definitions and owners even when that walk is empty.
Unknown events, missing metadata/boundaries, failed native configs, ambiguous
owners or scratch rewriting of a graded entity name refuse the entire family.

## Full original scope and ranges

All 47 originally disclosed inputs load. Existing ingest gates score 35 files;
12 native-valid inputs and their 12 definitions remain excluded as ambiguous
content. All 83 raw native definitions are retained in evidence; 71 are scored.
No reference is excluded. The diagnostic preserves the union of historical
scope disclosures and the original 71-definition count gate.

| Cohort | Native/scored files | Scored definitions and exact ranges found/truth | References |
|---|---:|---:|---:|
| awesome-compose | 1/1 | 4/4 A | 0/0, no reference evidence |
| nginx | 2/2 | 5/5 A | 0/0, no reference evidence |
| nginx-demos | 11/11 | 30/30 A | 1/1 A |
| server-configs-nginx | 33/21 | 32/32 A | 0/0, no reference evidence |

`native-position-population-r1.json` records counts, tier caps, exact normative
tracked-set receipts and native identities. All before/after hashes agree,
with zero generated/untracked files, symlinks or gitlinks. The tracked
approximation includes other formats and does not replace the measured scope.
Across the profiles there are 216 native source evaluations versus 210 sources
summed distinctly within each profile. Six repeated included-source evaluations
have exactly identical complete native trees/ranges; all are counted. Every
one of the 47 target sources is evaluated once. Differing repeats refuse.
No reference or target definition disappears through include coalescing.

Zero faults; warm throughput runs are 9.58, 7.74, 10.29 MB/CPU-s (median 9.58).
One binding and about 34 KB are thin evidence, not broad operational coverage.
Full raw responses, inventories and manifests remain private. Measured clones
and completed native source builds/downloads have already been removed.

## Reproduce

Use private TMPDIR/GOTMPDIR/GOCACHE/GOMODCACHE and GOWORK=off. Check `df -h ~`
and `uptime` before heavy steps: require at least 20 GiB free, load at most 6,
one serial nice -n 10 run, and a private venv. The committed build driver pins
nginx 1.31.6, OpenSSL 3.6.2 and PCRE2 10.47 by archive checksum.

```sh
python3 -m venv workspace/oracle/nginx-venv
workspace/oracle/nginx-venv/bin/pip install --only-binary=:all: -r calibration/oracles/nginx/requirements.txt
nice -n 10 python3 -B calibration/oracles/nginx/build_native.py -cache workspace/oracle/nginx-token-build -observe-upstreams -observe-tokens
install -m700 workspace/oracle/nginx-token-build/nginx-1.31.6/objs/nginx workspace/oracle/nginx-tokens
bash scripts/fathom-go.sh build -o workspace/tmp/indexbench ./cmd/indexbench
workspace/oracle/nginx-venv/bin/python -B calibration/oracles/nginx/grade_native_l2.py -native-population -native-definitions -indexbench workspace/tmp/indexbench -python workspace/oracle/nginx-venv/bin/python -nginx workspace/oracle/nginx-tokens -build-receipt workspace/oracle/nginx-token-build/build-receipt.json -subjects workspace/calibration/nginx-native-positions -out workspace/artifacts/nginx-native-positions
workspace/oracle/nginx-venv/bin/python -B calibration/oracles/nginx/probe_controls.py -native-definitions -nginx workspace/oracle/nginx-tokens -build-receipt workspace/oracle/nginx-token-build/build-receipt.json -out workspace/artifacts/nginx-native-position-controls
```

Keep the receipt and small binary, then prune source builds/downloads and
subjects after measurement. `native-token-build-r1.json` pins this measured
build; a rebuild freezes its own checksum before any configuration query.
The measurement CLI requires fresh native population, lexer/range and scanner
omission controls before A output. No system install or daemon is used.

## Control evidence

Authored tests cover HTTP/stream, unused/forward declarations, multiline/inline
ranges, custom data blocks, quoted/non-ASCII arguments, repeated identical
includes, differing-repeat refusal, final-newline differences and identity
failures. The control driver executes the actual Fathom extractor with -race:
deleting all five nodes leaves raw native truth unchanged; changing an input
binding changes truth. Filtering native definitions by independent scanner
candidates makes the omission test FAIL, then restored green.
`native-position-controls-r1.json` records the digests and FAIL line.

Caps refuse the whole profile at 10,001 tokens, 65 arguments including the
directive, 513 bytes per argument/path, and 8,388,609 stderr bytes. Native cap
controls prove refusal even when nginx accepts the authored configuration.
No truncated, dropped or independently supplied field becomes native truth.

There is no remaining native L1/L2 availability ceiling. Canonical integration
and L3/include work remain held until D19. No product, engine, epoch, generated
tracker or checkbox change is part of this diagnostic.

Final verification: all 63 authored tests pass with native controls enabled and zero skips. Focused extractor races, release-notes, merge-debris, ADR-011 and strict reading round-trip pass. X-R5 matches are format/template tokens, decorators and Authority/Authored prose; no subject people or attribution lists are committed.
