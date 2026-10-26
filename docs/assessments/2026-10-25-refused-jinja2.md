# Jinja2: grammar replacement and full oracle reading

**Measured:** Jinja2 reaches L3 with excellent L1, L2, and L3 grades on the
520-file calibration corpus. Three full oracle runs gave a median 5.17 MB per
CPU-second, above the 1.0 floor, with three files in parse error and zero
calibration faults. The `.j2`, `.jinja`, and `.jinja2` extensions are dispatched.
`templates/*.html` remains outside this coverage.

## Why the previous dialect was refused

The former `cathaysia/tree-sitter-jinja` v0.5.0 grammar (Apache-2.0) lexed text
one character at a time. After a pre-parse repair it still had 21 of 520 files
in error. Its 2026-10-25 local full-oracle timing retake measured 0.93 MB/CPU-s
across three runs (790.031, 791.365, 791.658 CPU ms); the historical reading
was 0.34. It could not meet the 1.0 tenancy floor despite excellent accuracy.
The retimer could not accept a shard subdirectory as a subject root: it accepted
`git rev-parse HEAD` initially, then `subjectSHA` required the repository top
level and falsely reported a SHA change. Full independent oracle reruns with
explicit subject SHAs verified the same corpus and accuracy. This retimer
friction is unresolved and did not change the measurement scope.

## Replacement and production build

The replacement is
[`bennypowers/tree-sitter-jinja-dialects`](https://github.com/bennypowers/tree-sitter-jinja-dialects/tree/4f832fe6feeae8c9e5963d835bec0272a8331b47)
commit `4f832fe6feeae8c9e5963d835bec0272a8331b47` (MIT), with the
in-tree `optional-else.patch` at SHA-256
`ac3e0825cd149da56ed02cac14ea2e7f96b92d8b8f10c1c0a3cc9e9f4410be86`.
It uses the shared grammar-patch mechanism from [P-R1 PR #36](https://github.com/GeoffMillerAZ/fathom/pull/36) (`gpu-sci/grammar-patches-r1`). The patch
allows a conditional without else while preserving loop-filter precedence.
The committed production-build proof reproduces WASM ERROR-or-MISSING files
27/520 unpatched to 3/520 patched: 24 repaired, no new errors and zero faults.
`calibration/oracles/jinja2-meta/proof/parse-gate.json` records both blob digests,
all file/byte totals and content/error-set hashes, without subject file lists.
The driver checks pinned HEADs and clean full shard subtrees, then builds both
variants with unmodified production `build.sh`. Its source commit is `85b73ee11`.

The 22 authored controls are independently graded by Jinja 3.1.6. The shipped
WASM agrees on 21; one pre-existing loop over-acceptance is explicitly recorded:
Jinja rejects an unparenthesized conditional iterable that the grammar accepts.
The Go test pins that divergence and the legal loop filter's AST, so an AST
regression or a future correction is noticed. The three authored upstream
corpus additions are committed beside the patch. The prepared upstream PR's
separate suite claim is 108 passing cases (105 upstream plus these three).

Production `build.sh` ran unmodified twice and produced the same raw WASM
SHA-256 `71680f5fead32c2881a5e486e8dc79a71656e3be60fff7b714667673b5f30e25`
(212061 raw bytes, 71302 compressed bytes). The dialect parses input as written
without the old preprocessor. Its tags/import queries were authored from owned
fixture probes using keyword positions, the header's `@end`, and the macro
signature lead. No upstream query or calibration source was copied.

## Full oracle evidence

The three raw receipts `calibration/oracles/jinja2-meta/proof/full-oracle-run{1,2,3}.json` ran on
2026-10-26 UTC (2026-10-25 local), darwin/arm64 Apple silicon, nice 10,
GOMAXPROCS=2, starting one-minute load below 9, one heavy job at a time. Every
run used the exact two existing corpus shards, with no file exclusion:

| Subject and shard | Commit | Files | Bytes |
|---|---|---:|---:|
| openstack/kolla, kolla/docker | `27e42412114c29669f70a298f89c54abc6a7f4ac` | 189 | 192662 |
| openstack/kolla-ansible, ansible/roles | `4b493ef36ace2b90257ecf94feb00e2c6e8014c9` | 331 | 541914 |

The independent tier-A oracle used Jinja 3.1.6's parser, AST, and
`meta.find_referenced_templates`. All three runs agree on 1124 definitions,
687 qualified calls, and 189 resolved imports/file edges. All measured accuracy metrics
are 1.0 and L1–L3 grade excellent. L4 was not attempted. This is a full
extraction and resolution grade, beyond the native parse-coverage screen.
The measured checkout, freshly built standalone Go harness and independent AST
adapter are all `85b73ee11`. No older binary was reused. Namespace
receiver names add four distinct call pairs formerly merged by bare-name truth.
No threshold or name-match mode changed. The adapter fails on an unsupported
receiver or direct dynamic callee rather than excluding its file.

| Run | Extraction CPU ms | MB/CPU-s | ERROR or MISSING files | Faults |
|---|---:|---:|---:|---:|
| 1 | 142.144 | 5.17 | 3 | 0 |
| 2 | 149.184 | 4.92 | 3 | 0 |
| 3 | 141.760 | 5.18 | 3 | 0 |

Run 1 is the median by unrounded CPU throughput (734576 / 142.144 ms).
The canonical `docs/bench/readings/lang/jinja2.json` is byte-identical to that
harness receipt, including its five driver-written notes. No notes were appended;
source, environment and independent ingest-test disclosures belong here. The error rate is 3/520
(0.58%); maximum measured file time is 2 ms and peak module memory is 279
pages. Remaining parser misses involve splatted arguments, extra filter
arguments, and a numeric dotted attribute.

A separate committed stress receipt (`calibration/oracles/jinja2-meta/proof/stress.json`)
records a 2 MiB text input at 58.581 ms, 256 nested blocks at 1.426 ms, and
one malformed 57344-byte repeated `{% if (` input that hit the 10-second budget
(10000.640 ms, one fault). The next 55-byte macro with optional else recovered
in 7.214 ms with two symbols and zero faults. The calibration's zero-fault
result does not cover arbitrary malformed inputs.

## Coverage and disposition

The three explicit Jinja extensions are dispatched and covered by end-to-end
tests, including an import alias collision with two same-named macros and a
plain HTML control. These tests are independent of the reading harness. The `.html` route is an honest ceiling: extension dispatch and the file
resolver select HTML before a Jinja sniff. That needs an engine and HTML regrade
before claiming template HTML support. The five-source route audit is
`docs/assessments/2026-10-25-refused-jinja2-sources.md`; working LSP/runtime
oracles do not change product dispatch. The upstream PR is prepared only under
`workspace/upstream/jinja-optional-else/`; no fork or PR has been opened. This
assessment changes no plan checkbox. Its governing record is LB-Q-jinja2; the
former refusal was LB-Q-jinja2-rest, and P-R1 supplied the shared patch path.

## Reproduction entrypoints

Install the pinned Jinja 3.1.6 oracle dependency as documented in
`calibration/oracles/jinja2-meta/README.md`, then use the committed entrypoints
below. The subject roots and SHAs are the table above. Grammar controls, production
patched/unpatched proof and stress commands are in
`calibration/oracles/jinja2-meta/PATCH-PROOF.md`. Run one heavy job at a
time with nice 10, after one-minute load falls below 9. Repeat the two
`lang-oracle` commands into separate run files three times; select the median
by pooled extraction CPU time. Do not substitute whole-repository roots.

```sh
python3 -I calibration/oracles/jinja2-meta/jinja2_meta_dump.py \
  -subject workspace/calibration/kolla/kolla/docker \
  -pythonpath workspace/tools/jinja2pkg -out workspace/artifacts/jinja-oracle-kolla.json
python3 -I calibration/oracles/jinja2-meta/jinja2_meta_dump.py \
  -subject workspace/calibration/kolla-ansible/ansible/roles \
  -pythonpath workspace/tools/jinja2pkg -out workspace/artifacts/jinja-oracle-kolla-ansible.json
go run ./cmd/indexbench lang-oracle -dialect jinja2 \
  -subject workspace/calibration/kolla/kolla/docker -subject-name openstack/kolla \
  -subject-sha 27e42412114c29669f70a298f89c54abc6a7f4ac \
  -tool-oracle workspace/artifacts/jinja-oracle-kolla.json -out workspace/artifacts/jinja-review.json
go run ./cmd/indexbench lang-oracle -dialect jinja2 \
  -subject workspace/calibration/kolla-ansible/ansible/roles -subject-name openstack/kolla-ansible \
  -subject-sha 4b493ef36ace2b90257ecf94feb00e2c6e8014c9 \
  -tool-oracle workspace/artifacts/jinja-oracle-kolla-ansible.json -out workspace/artifacts/jinja-review.json
```
