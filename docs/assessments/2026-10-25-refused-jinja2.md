# Jinja2: throughput retake and grammar candidate

The dispatched state stays refused. The original cathaysia v0.5.0 dialect still
reads **none** at **0.93 MB/CPU-s**, below the unchanged 1.0 floor. Three full oracle
runs reproduce every accuracy count and grade, with 21/520 files in error, zero
faults, and L1/L2/L3 excellent accuracy. The historical throughput was 0.34.

## Original-dialect timing evidence

Measured locally on darwin/arm64 Apple silicon at fathom 3b77ae9c4, with nice 10,
GOMAXPROCS=2, GOFLAGS=-p=2 and starting one-minute load below 9. One heavy lane job
at a time. Warm-up compilation is outside per-file timing. All 520 files and
734576 bytes of the two calibration/corpus.d/jinja2.yaml shard scopes remain in
scope, with no -only exclusion. Subject commits are unchanged:

| Subject and shard | Commit | Files | Bytes | Files with error |
|---|---|---:|---:|---:|
| openstack/kolla, kolla/docker | 27e42412114c29669f70a298f89c54abc6a7f4ac | 189 | 192662 | 2 |
| openstack/kolla-ansible, ansible/roles | 4b493ef36ace2b90257ecf94feb00e2c6e8014c9 | 331 | 541914 | 19 |

Jinja 3.1.6's parser, AST and meta.find_referenced_templates ran independently
for both subjects (0 unparsed; 1021/103 definitions; 188/1 resolved imports).
Each of three lang-oracle runs used those neutral oracle files and explicit
-subject-sha values, since the shard root is a subdirectory. Every subject identity,
accuracy counter, metric and tier equals the original reading.

| Run | Pooled extraction CPU ms | MB/CPU-s | Max-file ms |
|---|---:|---:|---:|
| 1 | 790.031 | 0.93 | 36 |
| 2 | 791.365 | 0.93 | 37 |
| 3 | 791.658 | 0.93 | 35 |

Run 2 is the median by unrounded CPU throughput. Only the reading's timing
fields are refreshed, then langbench.Regrade recomputes the same accuracy grades
and level none. Accuracy measured_at and fathom_commit remain the original
measurement, as for lang-retime; a note dates and identifies this timing retake.

## Retimer friction, draft only

lang-retime first accepts git rev-parse HEAD from a shard subdirectory, then
measureSubject calls subjectSHA, which requires the root to be its repository's
top level. It returns no SHA, and the retimer says "sha changed during measurement"
although no file or commit changed. Whole-repository roots instead correctly fail
scope identity (199 vs 189 Kolla files). No shared-engine fix is included. The safe
fallback above fully reran the independent oracle and verified exact identities.

## Candidate source

The existing-work search found
[bennypowers/tree-sitter-jinja-dialects](https://github.com/bennypowers/tree-sitter-jinja-dialects/tree/4f832fe6feeae8c9e5963d835bec0272a8331b47)
(MIT). Unlike the current grammar's one-character text tokens, its text rule
consumes a run. Its structured expressions and statements allow definition and
import queries, so this fires the written grammar retry trigger. Cathaysia HEAD
c213d3745ccdcaaa858869181c7b1bf9557a025f still uses character tokens.

Native screening of the alternative gives 27/520 files with ERROR or MISSING,
just over 5%. Twenty-six files fail on a legal conditional without else, which
the grammar requires. A minimal local optional-else patch gives 3/520 (0.58%),
with legal/invalid controls checked independently by Jinja. An initial bounded
WASM screen agrees across three runs, with zero faults. Those initial WASM numbers
are provisional: upstream self-review found a loop-filter AST regression. A
for_condition precedence adjustment preserves that AST and passes all 105
original upstream cases, three new corpus cases and eight independent Jinja
syntax controls; native errors remain 3/520. The final patch awaits a WASM
rebuild through P-R1. No candidate accuracy or full extraction-throughput grade
is claimed from a parse-only screen.

## P-R1 and next work

P-R1 assigns the shared grammar-patch mechanism to gpu-sci/grammar-patches-r1.
Reuse it after its READY receipt, and base this item on that branch. Do not add a
second mechanism. Prepare the optional-else patch in its selected layout, adapt
Jinja's dialect queries, rerun full oracle measurements and median-three timings,
and prove dispatch/classification before claiming support. No candidate blob,
patch pin, dispatch, epoch bump or plan checkbox is shipped by this assessment.

The upstream PR is prepared only under workspace/upstream/jinja-optional-else/.
No upstream PR or fork has been created. Remaining parser misses, after the
initial optional-else fix, are splatted arguments, a filter's extra arguments,
and a numeric dotted attribute. No calibration source is copied into the patch.
