# W7-1 P1/P2: final FIX2 coherence correction

Final code-fix round: local proofs pass, but **architect-class HOLD** remains for
independent re-review and the architect's diff read before any integration. If the
final review rejects this source, hold it rather than starting FIX3. No issue 251
dispatch, full integration gate, deployed ENG-1 proof or plan-row completion is
claimed. W12's chosen RT-197 option three remains undispatched behind its freeze;
its known defects and their separately owned witnesses are untouched.

Latest admitted snapshot+syntax+BOTH median: **1.496902958 s → 1.250227709 s**,
**16.5% lower (1.197x)** than the unchanged indexed baseline. Every changed trial
still exceeds one second. Both older source cohorts below are preserved as
historical measurements, not acceptance evidence for this final source.

## F2 and the coherence check

FIX1 closed sequential F1 but missed publication interleaving. `PatchPackage`
publishes the updated Snapshot before replacing its syntax graph. At that point a
first BOTH load can see current nodes containing new `B.Run` and old unmarked
syntax which still reaches `A.Run` from `h.Run`. Same watermark is not snapshot
identity. Copying that old graph changed a formerly fresh BOTH answer.

FIX2 leaves the existing non-atomic package publication and approximation entirely
unchanged. It refines the existing internal `CanReuseSyntax` predicate to take the
intended snapshot nodes. Reuse now requires both an unpatched memo syntax graph
and exact equality with the snapshot's deduplicated node values. A reverse scan
implements Build's last-observation-wins rule. Every SymbolNode field, including
Calls order and nil/empty distinction, is compared. An added, removed or changed
node declines reuse and takes the existing fresh `BuildBoth(snapshot.Nodes, typed)`
path. Equivalent immutable node content can safely be reused even when produced
by different publications; no epoch guess is needed.

The predicate uses a transient seen-key set, not new resident state. No new
exported symbol, graph/snapshot field, cache/group, lock, store/config structure,
handler read or dependency is added. P1, FIX1's package-patched marker,
`WithPackagePatch`, `ReplacePackage`, the clone helpers and typed augmentation are
unchanged. This repairs only P2's additional reuse boundary; it does not make
package patches sound or propose a fifth RT-197 scheme.

## Actual controlled-publication RED before production edit

The independent review witness was replayed against actual `PatchPackage` before
any FIX2 production edit. A test-only Go overlay inserts only a callback variable
and a scheduling barrier after the existing snapshot publication. Removing those
two insertions yields byte-identical production source, asserted by retained
`red-overlay-proof.json`. Actual first `GraphFor(BOTH)` runs while refresh is
paused, then refresh resumes. No fake graph is injected.

```text
go test -race -overlay=workspace/tmp/w7-fix2/red-overlay.json ./internal/graphmemo -run '^TestCriticFix1ConcurrentPublication$' -count=1
current B.Run=true old syntax reusable=true candidate A.Run=true fresh A.Run=false
first BOTH reused previous-generation unmarked syntax after current snapshot was published
FAIL github.com/geoffmilleraz/fathom/internal/graphmemo 0.293s
```

This is deterministic scheduled preemption, not an uninstrumented stress-run
claim. The historical `2c495d35` source and proof remain intact. The durable owned
test `TestCloneBothRejectsMixedSnapshotPublication` separately constructs exactly
that snapshot-published/old-syntax-resident memo boundary without a production
hook. It was also RED before the correction in four input orders with nil, empty
and populated typed edges. Its oracle is still fresh `BuildBoth` over the actual
published nodes, with all Graph fields, served output, typed/provenance metadata,
registry bytes, memo identity, held graphs and snapshot immutability asserted.
An initial accounting assertion forgot that earlier subtest scopes remain resident;
it was corrected to use the pre-load registry-byte delta before production edits.
The corrected test remained RED on the same stale-edge/full-field mismatch.

## GREEN and coverage

The same actual-production barrier replay passes with FIX2 and the same fresh
full-field oracle:

```text
go test -race -overlay=workspace/tmp/w7-fix2/green-overlay.json ./internal/graphmemo -run '^TestCriticFix1ConcurrentPublication$' -count=1
ok github.com/geoffmilleraz/fathom/internal/graphmemo 1.381s

go test -race ./internal/analysis/callgraph ./internal/graphmemo -count=1
ok github.com/geoffmilleraz/fathom/internal/analysis/callgraph 12.055s
ok github.com/geoffmilleraz/fathom/internal/graphmemo 3.162s
```

Direct Go tests used the verified SH package pool; no EX wrapper was nested inside
SH. The owned publication-state test covers all four orders and three typed-edge
inputs. Existing sequential F1, generated/real-fixture/four-order equality,
pruning, clone isolation, cold-only retention, failure, admission, singleflight,
cancellation, watermark and byte-accounting tests remain enabled.

New field-completeness tests mutate every SymbolNode field through reflection and
require reuse refusal, so adding a field without updating the comparison fails.
They cover duplicate last-wins order, extra/missing nodes and nil/empty Calls.
Every generated/fixture syntax graph must also accept its own original input,
including duplicate observations in all four orders.

The original route probe intentionally seeded syntax from different snapshot node
values. That no longer meets the corrected eligibility contract. The probe now
uses identical snapshot data and a three-edge adjacency's spare-capacity
fingerprint: fresh resolution and cloning have identical full Graph values but
predictably different retained capacity/EstimatedBytes. It asserts the fresh
full-field answer and cloned capacity, proving the production route still reuses
matching syntax without admitting a mismatched graph. The fixture asserts that
this fingerprint actually differs, rather than silently passing if it disappears.
No output oracle is relaxed.

Overlay receipts record HEAD at capture and the exact worktree source SHA256.
The GREEN overlay source hash and all measured candidate production hashes are
checked against final owned source; `proof-source.json` binds the final commit,
tree, test and reading identities. Overlay callback/barrier files are ignored
evidence only and never enter product code.

## Latest five baseline and five final-source trials, all EX

Baseline remains unchanged indexed source
`d9eef4794afdbc200d1b8697ce121d9d688fedaf`. The candidate is that exact archive with
only the three owned production files overlaid from FIX2. Each trial is a fresh
process with a separate byte-identical copy of the stable original W5 input,
SHA256 `e22f40d4dd467b0201badae263901741db23a81864c4f545c111b5c469ecd704`.
Every copy and pinned input hash matches before/after. The original W5 corpus
archive is byte-identical. Archived evidence is read-only; no ingest ran.

The common W5 serving-load harness, phase marks, boundary memory/memo sampling and
full-field serializer are identical in both source arms and unchanged from FIX1.
All original Graph fields plus package-patched provenance are serialized; the
old baseline has normalized false provenance and candidate fresh graphs emit
actual false. All ten latest canonical outputs match. No refusal, omission,
retry, selected timing, warm result or fast-decline substitute is included.

Actual successful admission, one symbol read and one typed read occur per trial.
At primary completion, snapshot and typed each have 1 miss/flight/entry and
graphstore 2; after resolved graphstore has 3. Every group has zero declined,
abandoned, refuse-cancelled or uncached builds. The enforced gate and managed
budget remain unchanged except the harness's ten-minute tree deadline for a
complete timed load. Environment: Go 1.27.4 darwin/arm64, cgo, 32 GiB/12-core Apple
M2 Pro, GOMAXPROCS 6, 4 GiB managed budget, 1 GiB memo budget, 8 GiB GC hint, normal
level. Admission/scope/snapshot/syntax/BOTH including typed loading are primary;
resolved and common memory sampling are outside primary and inside full-load time.
No OS page-cache flush is claimed.

| Trial | Primary ns | Full-load ns | Snapshot s | Syntax s | BOTH s | Resolved extra s | Boundary allocated MiB | Boundary resident MiB | Process peak RSS MiB |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 01-baseline | 1442907459 | 1730747292 | 0.342789 | 0.299193 | 0.800926 | 0.287839 | 806.058 | 419.480 | 1482.000 |
| 02-p1p2 | 1250227709 | 1542809667 | 0.343016 | 0.307660 | 0.599552 | 0.292581 | 727.229 | 418.980 | 1657.328 |
| 03-p1p2 | 1240608500 | 1555657042 | 0.342547 | 0.298618 | 0.599443 | 0.315048 | 727.382 | 418.980 | 1563.438 |
| 04-baseline | 1440765750 | 1722939750 | 0.340736 | 0.300675 | 0.799354 | 0.282173 | 805.972 | 419.480 | 1509.656 |
| 05-baseline | 1509837375 | 1795541458 | 0.354254 | 0.321306 | 0.834277 | 0.285703 | 806.034 | 419.479 | 1694.406 |
| 06-p1p2 | 1287802792 | 1601237584 | 0.336404 | 0.321458 | 0.629941 | 0.313434 | 727.555 | 418.978 | 1660.219 |
| 07-p1p2 | 1276316583 | 1567742792 | 0.348992 | 0.331041 | 0.596284 | 0.291426 | 726.691 | 418.981 | 1660.500 |
| 08-baseline | 1496902958 | 1775724458 | 0.360444 | 0.324817 | 0.811642 | 0.278821 | 805.567 | 419.481 | 1673.484 |
| 09-baseline | 1534256791 | 1818163708 | 0.337482 | 0.338877 | 0.857898 | 0.283906 | 805.718 | 419.480 | 1671.109 |
| 10-p1p2 | 1237251292 | 1524217125 | 0.336950 | 0.299731 | 0.600570 | 0.286965 | 727.753 | 418.980 | 1542.641 |

| Latest median | Baseline | FIX2 P1/P2 | Reduction |
|---|---:|---:|---:|
| Primary seconds | 1.496903 | 1.250228 | 16.48% |
| Full-load seconds | 1.775724 | 1.555657 | 12.39% |
| Boundary allocated MiB | 805.972198 | 727.381592 | 9.75% |
| Boundary live heap MiB | 495.443993 | 401.578583 | 18.95% |
| Boundary memo estimate MiB | 419.479829 | 418.980210 | 0.12% |
| Process peak RSS MiB | 1671.109375 | 1657.328125 | 0.82% |

Primary and full-load timer columns are exact nanoseconds; phase/memory displays
are rounded. Every raw value and counter is retained. The transient eligibility
set and generated keys cost allocations, fully included here: the latest net
allocation reduction is smaller than the historical FIX1 cohort's. This is not
a paired incremental measurement of the validator alone. No new resident memo
structure is hidden from accounting. Peak RSS includes resolved and output
serialization, not just the primary boundary. Five-per-arm medians are descriptive,
not confidence intervals or separate P1/P2 effect estimates. The one-second bar
still fails. Timings concern fresh loads; actual publication correctness is the
separate controlled replay and owned regression proof above.

## Final-source identities and raw evidence

Original corpus archive SHA256: `0d2836139e55958e8d7f4ab3ef9198034cbe70bc1e98df7c9b43f723be57d082`.
`identity.json` records every overlaid product/harness hash; `fix-source.diff`
records the correction against reviewed parent `2c495d35`.

| Latest binary | SHA256 |
|---|---|
| baseline | `15539c6ab3d1cb9522c15c44728700934d498e472a24df24387b0095c9dbb662` |
| p1p2 | `ee507db81ceb43a86a4451c255c1ae24a162d9b34c1729476d48180b63af1c85` |

| Latest output, equal in all ten trials | SHA256 |
|---|---|
| both | `e7a4a4a7764051b2c2462dcb938be9ad4928a765caf6a6ce08d604ef1f3088ac` |
| resolved | `cafaba53c5b445595ace544567269223e0734b475998057dab8e4072676b21d7` |
| snapshot | `106d44446d04dc54781a110e5e24f42a15bc9055e56346426e0121dbc3fab202` |
| syntax | `cf89006dad2a148dc3b384167850a2966f2938aa317ce97eb13a27474678afca` |
| typed | `1c07d1627ad3ac3f96ba8ecad7db5b2e2b97223f40de0c785a0a9719bf6510bf` |

Final raw evidence is isolated under `workspace/tmp/w7-fix2`: RED/GREEN overlays
and restoration/source receipts, actual-publication and durable-test RED logs,
full-package GREEN, source archive/arm copies/diffs, identity manifests, binaries
and Go build metadata, every result/exit/time/RSS/ambient/full-output receipt,
input hashes, `all-trials.json`, `summary.json`, `SHA256SUMS` and final
`proof-source.json`. Measured source and GREEN overlay source match the final
owned production files. Historical `workspace/tmp/w7` and `w7-fix1` evidence is
untouched. Original W5/W6 evidence remains read-only at
`../out/evidence-bundle4/measurement-251/workspace` with suffixes intact.

Reproduce with retained FIX2 scripts and a fresh output area after preserving
existing proofs, sourced lane environment, shared `FATHOM_GATE_LOCK` and clear
`GATE-RUNNING`:

```sh
bash scripts/gate-lease.sh bash workspace/tmp/w7-fix2/run.sh
bash scripts/gate-lease.sh python3 workspace/tmp/w7-fix2/analyze.py
```

Binaries use `-trimpath -buildvcs=false`. Package tests go directly through the SH
pool; build/timing/analysis/hash work is EX. Scripts retain every trial and reject
changed input, unequal full outputs, incomplete misses or refusals/abandonments.
No full gate, native plan verification, push, deployment or restart ran here.
The integration owner retains any performance changelog fragment outside this
lane's allowlist. Corrected critic LAND, if obtained, still leaves architect HOLD
before integration. This is the second and final fix round; no FIX3 is authorized.

---

The entire older FIX1 record follows unchanged apart from its heading, including
its earlier W7 historical section. F1's sequential fix remains, but F2 invalidated
its publication-coherence acceptance. Those old trials and hashes describe exactly
their old sources and are not proof of this final correction.

## Historical FIX1 source 2c495d35 (superseded by FIX2)

FIX1 is implemented with local production-refresh and package proofs. **Architect
class HOLD** remains pending independent re-review and the architect's diff read.
No normal integration, issue 251 dispatch, RT-197 redesign, deployed ENG-1 proof or
plan-row claim is authorized or implied.

The latest admitted cache-miss snapshot+syntax+BOTH median is **1.506906375 s →
1.264825542 s**, a **16.1% reduction (1.191x)** over the unchanged indexed baseline.
The one-second bar still fails in every changed trial. The earlier W7 results are
preserved below as historical data for the known-regressing source `fe3aa2234`.
They are not evidence for the corrected source.

## F1 and the minimal eligibility correction

Independent review of `fe3aa2234` found a real regression: a completed resident
syntax graph can be a package-patch approximation. In valid Go, `A.Run` initially
makes `c.Call(h a.A) { h.Run() }` resolve uniquely. A refresh adds distinct `B.Run`
with all full keys still unique. The existing syntax package patch may retain
`c.Call → A.Run`; a first cold BOTH build previously resolved the current nodes
fresh and refused that now-ambiguous receiver guess. P2 incorrectly cloned the
approximate graph into that formerly fresh answer.

The memo now applies its existing package patch through `WithPackagePatch`, which
marks the returned graph as package-patched before publication. `GraphFor(BOTH)`
reuses syntax only when its exact-scope resident graph passes `CanReuseSyntax`.
Marked syntax takes the unchanged fresh `BuildBoth(current snapshot nodes, typed)`
path. Full builds have an unmarked provenance bit; `WithTypedEdges` copies the bit
and cannot promote a marked graph. Eligibility here is provenance for the memo's
syntax entry, not a universal soundness claim about arbitrary caller-built graphs.

This adds one boolean to the existing Graph value, no retained cache or memo group.
`ReplacePackage` and its clone helpers are unchanged. Approximate syntax and already
resident BOTH patch behavior remain as before, including their inherited limits.
Cold combined-only retention, admission, singleflight, cancellation, typed-read
failure disclosure, watermark and byte-accounting laws stay in force. No collision
guard, snapshot reorder, wider refresh repair, store/handler read, schema/config
change or RT-197 work is included. P1's implementation is unchanged by FIX1.

## Actual production-refresh RED, then GREEN

Before any FIX1 production edit, the new test exercised the actual
`SnapshotFor → GraphFor(Syntax) → PatchPackage → PeekSnapshot → first GraphFor(BOTH)`
route. The extracted `h.Run` fixture has no duplicate full key. It runs forward,
reverse, sorted and seeded-shuffled inputs with nil, empty and populated typed
edges. Every one of the twelve cases was RED on the reviewed source:

```text
go test -race ./internal/graphmemo -run '^TestCloneBothFirstLoadAfterProductionRefresh$' -count=1
first BOTH after production refresh differs from fresh BuildBoth (all Graph fields); inherited A.Run=true
served receiver-call answer differs from fresh oracle
FAIL github.com/geoffmilleraz/fathom/internal/graphmemo 0.368s
```

The oracle remains a fresh `BuildBoth` over the actual refreshed snapshot, never a
patched graph or relaxed edge comparison. It asserts every Graph field, the served
callee output and immutability of both the pre-refresh and held patched graph plus
the refreshed snapshot. It explicitly verifies that the inherited approximate
syntax result remains present while the first BOTH answer refuses its stale guess.
A separate clone test checks that fresh provenance survives cloning, package-patch
provenance stays ineligible through cloning, and the held original is unchanged.
All earlier generated/fixture/four-order equality, pruning, isolation and memo-law
tests remain enabled.

After the correction, direct package tests under the verified three-slot SH pool:

```text
go test -race ./internal/analysis/callgraph ./internal/graphmemo -count=1
ok github.com/geoffmilleraz/fathom/internal/analysis/callgraph 12.526s
ok github.com/geoffmilleraz/fathom/internal/graphmemo 3.080s
```

No EX wrapper was nested inside the package-test SH lease. Both timing binaries,
the ten trials, analysis and evidence hashing use EX. The measured production
files are checked against their recorded candidate hashes after the run.

## Latest 5+5 exclusive timing: all trials

The baseline is the original unchanged indexed source at
`d9eef4794afdbc200d1b8697ce121d9d688fedaf`, not the regressing W7 commit. The candidate
uses that exact archive with only the three owned production files overlaid from
FIX1. Each fresh process reads a separate byte-identical copy of the stable W5
input SHA256 `e22f40d4dd467b0201badae263901741db23a81864c4f545c111b5c469ecd704`.
Copies and pinned input match before/after. The original corpus archive matches
byte-for-byte. Archived W5/W6 evidence is read-only; no ingest is run.

Same W5 admitted boundary and phase instrumentation as the historical run, with
identical boundary memory/memo sampling in both arms. The common full-field
serializer additionally emits the new package-patched provenance bit: false for
the old baseline, which lacks this field, and its actual false value for the
candidate's full builds. Every original field is still serialized unchanged.
This occurs after the timed load. New output digests therefore need not equal the
historical serialization; all ten latest outputs must and do match each other.

Every trial has one actual successful admitted ticket, one symbol read, one typed
read and zero refusal, stopped/context-error or incomplete-miss outcomes. At BOTH,
snapshot and typed groups each have 1 miss/flight/entry; graphstore has 2 of each,
then 3 after resolved. All groups have zero declined, abandoned, refuse-cancelled
and uncached builds. No retry, omission, warm substitute or fast refusal is selected.

Environment remains Go 1.27.4 darwin/arm64, cgo enabled, 32 GiB/12-core Apple M2 Pro,
GOMAXPROCS 6, 4 GiB managed budget, 1 GiB memo budget, 8 GiB GC hint and normal level.
The constructed gate uses `ModeEnforce`; the tree deadline alone is extended to
ten minutes to obtain a complete timed load. This is a cold memo reading with no
OS page-cache flush. The primary timer includes admission, scope setup, snapshot,
syntax and BOTH including typed reads. Resolved and common memory-sampling overhead
are outside that primary boundary and remain in the full-load time.

Primary/full-load timers below are exact nanoseconds; displayed phases/memory are
rounded. All raw values/counters/output files are retained in
`workspace/tmp/w7-fix1/results` and `all-trials.json`.

| Trial | Primary ns | Full-load ns | Snapshot s | Syntax s | BOTH s | Resolved extra s | Boundary allocated MiB | Boundary resident MiB | Process peak RSS MiB |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 01-baseline | 1527890000 | 1838141333 | 0.352660 | 0.323977 | 0.851253 | 0.310251 | 806.040 | 419.480 | 1791.938 |
| 02-p1p2 | 1262763459 | 1563083042 | 0.356543 | 0.315222 | 0.590998 | 0.300319 | 707.658 | 418.980 | 1584.516 |
| 03-p1p2 | 1264825542 | 1553005000 | 0.358576 | 0.318350 | 0.587899 | 0.288179 | 708.220 | 418.980 | 1329.672 |
| 04-baseline | 1505867125 | 1795935542 | 0.359971 | 0.318858 | 0.827039 | 0.290068 | 806.372 | 419.479 | 1619.609 |
| 05-baseline | 1563715458 | 1855670083 | 0.354695 | 0.314108 | 0.894913 | 0.291954 | 806.521 | 419.479 | 1781.250 |
| 06-p1p2 | 1285233458 | 1601879417 | 0.357517 | 0.324431 | 0.603286 | 0.316645 | 707.499 | 418.979 | 1741.047 |
| 07-p1p2 | 1272466041 | 1575350166 | 0.361299 | 0.316083 | 0.595083 | 0.302883 | 708.174 | 418.981 | 1534.844 |
| 08-baseline | 1476450209 | 1774823875 | 0.351891 | 0.312096 | 0.812464 | 0.298373 | 805.966 | 419.481 | 1646.516 |
| 09-baseline | 1506906375 | 1800981167 | 0.375601 | 0.309043 | 0.822262 | 0.294074 | 805.850 | 419.479 | 1481.828 |
| 10-p1p2 | 1242951208 | 1535138958 | 0.356937 | 0.317082 | 0.568932 | 0.292166 | 708.159 | 418.981 | 1387.344 |

| Latest median | Baseline | FIX1 P1/P2 | Reduction |
|---|---:|---:|---:|
| Primary seconds | 1.506906 | 1.264826 | 16.06% |
| Full-load seconds | 1.800981 | 1.563083 | 13.21% |
| Boundary allocated MiB | 806.040031 | 708.158607 | 12.14% |
| Boundary live heap MiB | 480.208382 | 401.411743 | 16.41% |
| Boundary memo estimate MiB | 419.479203 | 418.980317 | 0.12% |
| Process peak RSS MiB | 1646.515625 | 1534.843750 | 6.78% |

These are descriptive medians, not confidence intervals or separate P1/P2 causal
estimates. Process peak RSS includes resolved building and canonical output
serialization; it is not a primary-boundary peak. Current timings exercise a fresh
load; the separate production-refresh test establishes the F1 fallback. No deployed
or broad refresh soundness claim follows from either proof.

## Latest source, binary, output and raw-proof identities

The original corpus archive SHA256 is `0d2836139e55958e8d7f4ab3ef9198034cbe70bc1e98df7c9b43f723be57d082`.
`identity.json` records the SHA256 of every overlaid production file and every
common harness file in both source arms. `fix-source.diff` records the product
correction against reviewed parent `fe3aa2234`.

| Latest binary | SHA256 |
|---|---|
| baseline | `15539c6ab3d1cb9522c15c44728700934d498e472a24df24387b0095c9dbb662` |
| p1p2 | `08bc8260aad84a9d437441ae3ad61ac1af214cfa3ae07c76fa29858c2302ddd0` |

| Latest output, same in every trial | SHA256 |
|---|---|
| both | `e7a4a4a7764051b2c2462dcb938be9ad4928a765caf6a6ce08d604ef1f3088ac` |
| resolved | `cafaba53c5b445595ace544567269223e0734b475998057dab8e4072676b21d7` |
| snapshot | `106d44446d04dc54781a110e5e24f42a15bc9055e56346426e0121dbc3fab202` |
| syntax | `cf89006dad2a148dc3b384167850a2966f2938aa317ce97eb13a27474678afca` |
| typed | `1c07d1627ad3ac3f96ba8ecad7db5b2e2b97223f40de0c785a0a9719bf6510bf` |

Latest raw proofs are isolated under `workspace/tmp/w7-fix1`: production RED,
full-package GREEN, `source.tar`, per-arm source copies and diffs, `identity.json`,
`fix-source.diff`, binaries/Go build metadata, ten result/exit/time/RSS/ambient and
full-output receipts, before/after store hashes, `all-trials.json`, `summary.json`
and `SHA256SUMS`. Hashing verifies that measured candidate files still equal the
owned production source. Historical W7 raw proofs remain untouched under
`workspace/tmp/w7`. Original W5/W6 evidence now resides read-only under
`../out/evidence-bundle4/measurement-251/workspace`, with original suffixes intact.

Reproduce using retained `w7-fix1` scripts, a fresh output area with existing
proofs preserved, the stable pinned input, sourced lane environment and the shared
`FATHOM_GATE_LOCK`. Wait for `GATE-RUNNING` before heavy work:

```sh
bash scripts/gate-lease.sh bash workspace/tmp/w7-fix1/run.sh
bash scripts/gate-lease.sh python3 workspace/tmp/w7-fix1/analyze.py
```

Both binaries use `-trimpath -buildvcs=false`. The scripts copy from the archived
harness/corpus as read-only inputs, preserve every trial, and reject mismatched
input/output hashes, incomplete misses, declines or abandonments. Build/ingest/
timing remain EX. No full integration gate or native plan verification ran in this
lane. The integration owner still owns any performance changelog fragment outside
the six-path allowlist. FIX1 requires a second independent review before the
architect reads this held diff.

---

The following section is the unmodified historical W7 record apart from its
heading. Its source was rejected for F1 and is superseded for acceptance by FIX1
above. Old timings, hashes and test outputs remain evidence of exactly what ran,
not evidence that the old source handled production refresh correctly.

## Historical W7 source fe3aa2234 (superseded by FIX1)

P1/P2 reduce the admitted cache-miss snapshot+syntax+BOTH median from **1.500602458 s
to 1.266565417 s** on the same pinned W5 ingest: **15.6% less time, 1.185x speedup**.
The one-second bar still fails. This is local helper evidence, not a deployed
ENG-1 proof. Architect-class **HOLD** pending separate critic and architect review.
No plan row is claimed. Issue 251 is not dispatched.

## Change and preserved laws

P1 compacts the graph's owned typed-pruning adjacency slice in place instead of
copying survivors into a capacity-zero buffer that was discarded when nothing was
removed. P2 adds `Graph.WithTypedEdges`, reusing the unchanged clone helpers in
`replace_package.go`, and routes `GraphFor(GraphBoth)` through it only when the
exact scope's completed syntax graph is already resident. An absent or in-flight
syntax graph keeps the existing full-build path. Combined-only loads retain no
additional syntax entry.

Every graph map and mutable adjacency slice is copied before typed augmentation.
Immutable node payloads retain their existing sharing contract. Optional nil maps
remain nil until populated. The existing combined memo admission, singleflight,
context cancellation, typed-read failure disclosure, watermark eviction and byte
accounting remain in force (I21, I38, I49, I51). No store, schema, configuration,
registry or handler change, new memo group, retained cache or dependency is added.
P3/P4, worker reload and RT-197 are outside this change.

## RED before implementation, GREEN after

The exclusive lease reproduced RED before production edits:

```text
go test -race ./internal/analysis/callgraph ./internal/graphmemo -run '^TestCloneBoth' -count=1
WithTypedEdges undefined
TestCloneBothProductionReusesCompletedSyntax: production GraphBoth re-resolved snapshot instead of cloning completed syntax
```

The production-entry marker graph has an edge absent from snapshot nodes. Its full
graph comparison therefore detects re-resolution, beyond helper-only correctness.
After P2, full equality passed and P1 remained RED against the frozen old pruning
algorithm: candidate 3 allocations, baseline 3. Only then did P1 change.

After both changes, under the exclusive lease:

```text
go test -race ./internal/analysis/callgraph ./internal/graphmemo -count=1
ok github.com/geoffmilleraz/fathom/internal/analysis/callgraph 12.055s
ok github.com/geoffmilleraz/fathom/internal/graphmemo 2.947s
```

The clone equality oracle independently resolves syntax a second time. It checks
all Graph fields with `reflect.DeepEqual`, plus served caller/callee/test-impact,
candidate and product-reach outputs. Inputs include nil/empty sets, generated
ambiguous/duplicate-key/method/field sets, and Go extraction of actual servedcalls
and fauxtown fixtures, each in four input orders and with nil/empty/populated typed
edges. Tests cover typed pruning, dispatch, instantiation, missing endpoints,
synthetic package init and mutation isolation in both directions. The frozen
pruning oracle also covers all 16 typed-edge keep/remove masks and idempotence.

Clone helpers trim spare adjacency capacity. Tests account for the exact
capacity-byte difference in `EstimatedBytes`, rather than demanding identical heap
cost for identical graph values. Production-entry tests check exact registry byte
accounting, concurrent singleflight identity, memo hits, watermark changes,
empty-watermark behavior, combined-only retention, cancellation, typed failures,
recovery and cold deadline admission. An initial admission test incorrectly reused
a scope whose syntax build had already measured its service cost; it was corrected
to assert the existing cold-admission rule on a distinct cold scope. Product
admission logic was not changed.

Two initial tests used a temporarily authorized shared package pool before its
self-check authorization was withdrawn. Both had finished before the correction.
All decisive RED/GREEN and all builds/timings here subsequently ran under the
exclusive lease. No further shared-pool job ran. Evidence logs preserve both
attempts. Harness preparation also hit an older Python tarfile API before any
build/trial; its compatibility fix and failure log are retained. No trial was
omitted, retried, selected or replaced.

## Controlled measurement

Ten fresh processes: five unchanged indexed-baseline and five P1/P2 indexed
trials, alternating which arm leads each pair. Each receives a separate
byte-identical copy of the original W5 scratch ingest. The pinned source is only
copied, never opened by the client or written. Every copy's before/after hash and
the pinned source's before/after hash match:

`e22f40d4dd467b0201badae263901741db23a81864c4f545c111b5c469ecd704`.

The unchanged W5 indexed serving-load harness is reused in both arms with identical
phase marks, full-field canonical graph serialization and additional common
boundary memory/memo sampling. The primary timer starts before actual enforced
admission and ends after snapshot, syntax and BOTH complete. Typed loading is
included in BOTH. Resolved builds occur afterward and are shown separately.
Full-load time includes resolved and common measurement overhead. Snapshot phase
also includes admission/scope setup. Memory samples at BOTH are outside its timer;
process peak RSS includes later resolved construction and output serialization.
This is a cold memo reading, not an OS page-cache flush.

Environment: Go 1.27.4, darwin/arm64, cgo enabled, 32 GiB RAM, 12-core Apple M2 Pro,
GOMAXPROCS 6, 4 GiB managed budget, 1 GiB memo budget, 8 GiB GC hint, normal level.
The harness uses actual `ModeEnforce` admission, with only the tree deadline
extended to ten minutes so a timing trial measures a complete load rather than a
short-deadline refusal. Budget's informational Admission field remains its default;
the constructed gate's mode is explicitly enforce. No paid API or benchmark service was
used. Builds and the complete trial sequence held the exclusive shared gate lease.

Every trial had one admitted successful ticket, one symbol read and one typed read.
At the primary boundary the snapshot and typed groups each had one miss, flight and
entry; graphstore had two misses, flights and entries. After resolved, graphstore
had three of each. All groups had zero declined, abandoned, refuse-cancelled and
uncached builds. No warm/cached substitute or fast refusal is included. Each arm
loaded 104183 snapshot nodes and 256033 typed edges; syntax had 103099 nodes/92411
edges, BOTH 103118/162606, and resolved 103118/120260. Constructor references: 4216.

## All trials

Primary and full-load timers below are exact nanoseconds. Phase and memory columns
are rounded only for display. Every raw result, phase nanosecond, counter and
memory value is retained in `workspace/tmp/w7/all-trials.json` and the ten trial
directories. The required primary boundary includes admission, snapshot, syntax
and BOTH; the original W5 scan trials remain independent historical evidence.

| Trial | Primary ns | Full-load ns | Snapshot s | Syntax s | BOTH s | Resolved extra s | Boundary allocated MiB | Boundary resident MiB | Process peak RSS MiB |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 01-baseline | 1457749750 | 1748081334 | 0.347254 | 0.300609 | 0.809887 | 0.290331 | 805.973 | 419.480 | 1782.516 |
| 02-p1p2 | 1289623250 | 1603740375 | 0.357020 | 0.325471 | 0.607132 | 0.314116 | 707.475 | 418.981 | 1446.250 |
| 03-p1p2 | 1266565417 | 1542403417 | 0.365570 | 0.309721 | 0.591274 | 0.275838 | 708.476 | 418.981 | 1342.234 |
| 04-baseline | 1500602458 | 1788748083 | 0.353245 | 0.330020 | 0.817337 | 0.288145 | 805.871 | 419.481 | 1533.516 |
| 05-baseline | 1527452958 | 1847544333 | 0.356559 | 0.322939 | 0.847955 | 0.320091 | 805.586 | 419.479 | 1716.422 |
| 06-p1p2 | 1284700666 | 1587694666 | 0.357571 | 0.328648 | 0.598482 | 0.302993 | 708.004 | 418.981 | 1524.656 |
| 07-p1p2 | 1235570875 | 1544945333 | 0.336146 | 0.303954 | 0.595471 | 0.309374 | 708.194 | 418.980 | 1627.406 |
| 08-baseline | 1486191375 | 1765184625 | 0.338095 | 0.315985 | 0.832111 | 0.278993 | 805.930 | 419.481 | 1678.062 |
| 09-baseline | 1566325625 | 1853985750 | 0.366572 | 0.333849 | 0.865905 | 0.287659 | 805.620 | 419.480 | 1618.516 |
| 10-p1p2 | 1250867958 | 1550767833 | 0.363169 | 0.312543 | 0.575156 | 0.299899 | 707.906 | 418.980 | 1549.609 |

## Medians and limits

| Measurement | Indexed baseline | P1/P2 | Reduction |
|---|---:|---:|---:|
| Primary boundary seconds | 1.500602 | 1.266565 | 15.60% |
| Full-load seconds | 1.788748 | 1.550768 | 13.30% |
| Boundary allocated MiB | 805.871155 | 708.003838 | 12.14% |
| Boundary live heap MiB | 495.294670 | 400.795525 | 19.08% |
| Boundary memo estimate MiB | 419.479890 | 418.980622 | 0.12% |
| Process peak RSS MiB | 1678.062500 | 1524.656250 | 9.14% |

These five-per-arm descriptive medians do not establish a confidence interval or
separate P1's effect from P2's. Peak RSS is the whole process maximum, not a primary
boundary peak. The memo estimate decreases only about 0.50 MiB because both graph
values are still retained; allocation and live-heap reductions are larger. The
one-second primary bar fails in every P1/P2 trial. Deployed behavior, RT-195's
original scan ratio and ENG-1 completion are not established by this reading.

## Identity and reproduction

Baseline/corpus commit: `d9eef4794afdbc200d1b8697ce121d9d688fedaf`. The candidate is that exact
archive with only `graph.go`, `cache.go`, and `graphstore.go` overlaid. Source archive
SHA256: `0d2836139e55958e8d7f4ab3ef9198034cbe70bc1e98df7c9b43f723be57d082`.

| Binary | SHA256 |
|---|---|
| baseline | `92a1585ea57c8c5fced0d366ca2895919ca2966db5cdaa110c5589e4bd7f5317` |
| p1p2 | `e29cf36ea32f85b602e650370a9f1e5b8593c28d5fa9074a4272e201451f55a7` |

The following output SHA256 values matched across all ten trials, including every
private Graph field and suffix-index value (not just node/edge counts):

| Output | SHA256 |
|---|---|
| both | `48860bbf28bb6948682035b351de339e5cd6175290ad54f551f2ac547f55d692` |
| resolved | `b087384be66589ee51a2f08db4d382b3b3dbf88000869ca6adb9792dbdbafa85` |
| snapshot | `106d44446d04dc54781a110e5e24f42a15bc9055e56346426e0121dbc3fab202` |
| syntax | `ce43dbb85e06842ab965604eba4f5b6ae527cfabeef793c37778e2e867fefced` |
| typed | `1c07d1627ad3ac3f96ba8ecad7db5b2e2b97223f40de0c785a0a9719bf6510bf` |

Retained evidence is under `workspace/tmp/w7`: `source.tar`, `identity.json`
(per-file baseline/candidate and identical harness hashes), source diffs, binaries
and Go build metadata, environment before/after, raw per-trial canonical outputs,
result JSON, time/RSS stderr, exit codes, input hashes, `all-trials.json`,
`summary.json`, RED/GREEN logs and `SHA256SUMS`. The source and candidate diffs
allow a critic to verify that no product change outside P1/P2 enters either arm.

Reproduction uses the retained ignored scripts and the original W5 indexed harness:
source the lane environment, set the shared `FATHOM_GATE_LOCK`, wait for
`GATE-RUNNING` to clear, then run these commands from the lane root with a fresh
`workspace/tmp/w7` output area (preserve the existing evidence first):

```sh
bash scripts/gate-lease.sh bash workspace/tmp/w7/run.sh
bash scripts/gate-lease.sh python3 workspace/tmp/w7/analyze.py
```

`run.sh` builds both source arms, preserves the original pinned input, copies it
per trial and runs all ten in their published order. Both use `-trimpath
-buildvcs=false`. The scripts preserve all outputs and reject changed input,
incomplete memo misses, refusals and graph/snapshot/typed output differences.
They run no ingest or deployment. Per-file source identities and commands are
retained so the same baseline can be reconstructed without reinvention.

Separate critic and architect reads are still required. No full gate, native plan
verification, push, deployment, restart, release or plan-row completion is claimed.
The integration owner owns any observable-performance changelog fragment because
this lane's path allowlist excludes the changelog. This diff must remain outside
normal land bundles until the architect reviews it.
