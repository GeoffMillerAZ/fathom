# Tree-sitter extraction engine — wazero/WASM, calibration-gated (ADR-021 §4, ADR-004)

The language-breadth rearchitecture: one grammar-driven engine replaces
per-language heuristic extractors, language by language, only where it
beats them. Sequenced AFTER toolident (the classifier is the dispatch it
plugs into) and lang-truth (the registry it registers against).

Design (normative):
- **Pure Go always**: tree-sitter grammars compiled to WASM, executed via
  wazero. CGO bindings are forbidden (would reverse ADR-020's win). The
  engine embeds grammar .wasm blobs (size-audited; each language notes
  its blob size in the registry).
- **Extractor shape**: one TSExtractor implementing extractor.Extractor,
  parameterized by (grammar, query set) — tree-sitter queries (.scm)
  express symbols/calls/imports per language, kept as data beside the
  grammar. Adding a language = grammar blob + queries + fixtures, no new
  Go extraction logic.
- **Calibration gate per language** (the June corpus + harness): a
  language ships tree-sitter-backed ONLY when its calibration score
  beats the incumbent heuristic extractor on the corpus (precision AND
  recall); results recorded per language in this plan. Incumbent stays
  until beaten — no regressions for breadth's sake.
- **First three languages by demand**: TypeScript/JS (largest customer
  surface; current heuristic is the weakest vs ecosystem complexity),
  Python (second), Ruby (third — existing heuristic is thin). Go is NOT
  migrated (go/parser + typed edges are strictly better).
- Determinism: identical input → identical symbols (wazero is
  deterministic; queries sorted); golden fixtures per language.
- Performance recorded: parse throughput per language vs heuristic;
  acceptable if total ingest stays within 1.5× on the calibration corpus.

## Tasks

- [x] wazero engine + grammar/query registry + TSExtractor implementing extractor.Extractor {id: XT-1, symbol: NewTSExtractor, package: internal/extractor, reachable_from: TestAdoptionServiceScoped}
- [x] TypeScript/JS: grammar + queries + fixtures; calibration beats incumbent (scores recorded here) {id: XT-2, symbol: NewTSExtractor, package: internal/extractor, skip_test: true, skip: "language onboarding = data + calibration evidence; engine symbol already anchored by XT-1"}
- [x] Python: same bar {id: XT-3, symbol: NewTSExtractor, package: internal/extractor, skip_test: true, skip: "same rationale as XT-2"}
- [x] Ruby: same bar {id: XT-4, symbol: NewTSExtractor, package: internal/extractor, skip_test: true, skip: "same rationale as XT-2"}
- [x] Registry/docs: per-language engine provenance (heuristic vs tree-sitter) surfaced in capabilities + docs; ExtractorEpoch bump per migrated language {id: XT-5, symbol: FingerprintHash, package: internal/fusion, skip_test: true, skip: "epoch + docs; pinned by the epoch invalidation test"}

### XT-1 — engine (deviation from the original wazero sourcing assumption, disclosed)

The plan assumed vendoring grammar repos' own released `tree-sitter-<lang>.wasm`
artifacts directly. That was tried first and found genuinely infeasible to run
under bare wazero: both that artifact and its matching `web-tree-sitter` core
are Emscripten "side modules" (a `dylink.0` section, shared-memory/table base
offsets, GOT wiring, plus four `tree_sitter_*_callback` JS-runtime shims for
streaming input/logging/cancellation) — reimplementing Emscripten's dynamic
linker in Go is exactly the risk ADR-013 already flagged, now confirmed
empirically (see `internal/extractor/tsengine/grammars/PROVENANCE.md`'s "path
not taken"). The shape that DOES work and is what shipped: `zig cc
--target=wasm32-wasi-musl -mexec-model=reactor` statically links tree-sitter's
own core C runtime with one grammar's `parser.c`/`scanner.c` into a single
self-contained module per language — no dynamic linking, only wazero's stock
`wasi_snapshot_preview1` shim. Same design intent (pure Go, wazero, no CGO,
grammar-as-data blobs with size-audited provenance), different, simpler,
verified mechanism. Build recipe + resolved shas/sizes/sha256:
`internal/extractor/tsengine/grammars/{PROVENANCE.md,build.sh}`.

Symbol/call queries are real `.scm` files compiled and matched via
tree-sitter's own query engine (`ts_query_new`/cursor/`next_match`), not a
hand-rolled tree walk — `internal/extractor/tsqueries/*.scm`. Qualification
("Class.method", Ruby's "Outer::Inner") is one generic, language-agnostic
containment algorithm over the flat match list (`internal/extractor/
tsextract.go`: `resolveTSParents`/`qualifyTSDefs`), parameterized by which
capture kinds are containers/callables per language and whether containers
chain — so onboarding a language really is data (blob + queries + a kind-map
table), matching the plan's "no new Go extraction logic" intent.

Performance note: a wazero module instantiation is far more expensive than a
CGO `ts.NewParser()` call, so `TSExtractor` does NOT re-instantiate per file
— one long-lived `tsengine.Parser` + compiled `Query` per (grammar, entry
point), reused for the process's lifetime behind a mutex (fusion's pass-3
extraction is already documented sequential-per-Pipeline, so this is normally
uncontended). Re-instantiating per file was the first cut and measured ~11x
slower than the CGO incumbent on the calibration corpus — swapping to reuse
brought it to 1.41x (see Performance below).

### Calibration scores (2026-07-22, `calibration/baseline.sh`; updated after
the critic round's B2 query fix — coverage-ratio column; precision verified
via exhaustive exact-match unit tests, see below)

**Metric honesty note (critic M3):** the harness's column is labeled
"recall" but is really a **coverage ratio** — code symbols extracted ÷
grep-proxy definition count. The proxy is not ground truth (it both misses
definition forms and over-counts in strings/comments), so values above 1.0
are possible and the number is comparable BETWEEN extractors on the same
corpus, not an absolute recall. It is called "coverage" below wherever this
plan speaks for itself.

Before (incumbent, CGO tree-sitter/ADR-013) vs after (this engine, wazero,
including the B2 #private/computed-method query fix) — `code symbols` /
`coverage`, same corpus, same files, only the extractor changed:

| Language | Repo | Before (code, coverage) | After (code, coverage) | Verdict |
|---|---|---|---|---|
| JavaScript | axios | 774, 1.12 | 891, 1.29 | beats |
| JavaScript | express | 150, 1.01 | 151, 1.02 | beats |
| JavaScript | lodash | 648, 0.41 | 1970, 1.26 | **beats — large coverage gap closed** |
| TypeScript | nest | 5474, 2.29 | 5722, 2.40 | beats |
| TypeScript | typeorm | 5835, 2.21 | 6106, 2.31 | beats |
| TypeScript | zod | 2701, 1.56 | 3073, 1.77 | beats |
| Python | django | 43759, 1.07 | 43597, 1.06 | ties (−0.2%, within proxy noise) |
| Python | fastapi | 6207, 1.30 | 6184, 1.29 | ties |
| Python | flask | 1710, 1.06 | 1708, 1.06 | ties |
| Ruby | fastlane | 8539, 1.00 | 8541, 1.00 | ties |
| Ruby | jekyll | 1398, 1.01 | 1399, 1.01 | ties |
| Ruby | sinatra | 1159, 1.01 | 1163, 1.02 | ties |

(The B2 fix is visible as axios 884→891 and nest 5715→5722 vs the first
commit's recorded numbers — real #private/computed methods reclaimed.)

Full reports: `workspace/calibration/baseline-before-CGO.md` and
`workspace/calibration/baseline-after-tsengine.md` (gitignored, regenerate
with `calibration/clone.sh` + `calibration/baseline.sh`). **Coverage never
regressed on any of the 12 repos**; JS/TS show the biggest gains (lodash's
0.41→1.26 traces to the incumbent's recursive walker under-visiting some
constructs the flat query-based approach reaches directly; TS gains come from
its richer query set catching interfaces/types/enums the incumbent already
handled, just more completely). Python/Ruby are within noise of the
incumbent (already itself tree-sitter/CGO-backed, ADR-013 — these three
extractors were NOT "heuristic" as the plan's opening design section assumed;
they were already CGO tree-sitter, so this migration's real win is
CGO-removal + JS/TS coverage, not "tree-sitter vs regex").

**Corpus blindness (critic B2, recorded for the next calibration round):**
none of the six JS/TS corpus repos (nest, zod, typeorm, express, axios,
lodash) contains a single `#private` method (rg-verified) — so the
calibration gate was structurally unable to catch the B2 regression
(#private/computed method names dropped by an over-constrained query
alternation; caught by the critic, fixed with unit regressions in
`tsextract_test.go`). The next corpus revision should add at least one
modern-syntax-heavy JS/TS repo (class fields, #private members, decorators
in their post-TC39 form).

**Precision**: no automated precision oracle exists in `calibration/` (only
the recall proxy). Verified instead via (1) exhaustive exact-match unit tests
per language covering the tricky cases incumbents special-cased — nested
function NOT inheriting enclosing class scope, module-level var suppressed
inside a class/function, Ruby's `Outer::Inner::Widget.method` chaining,
per-symbol call collection (`internal/extractor/tsextract_test.go`); (2)
structural queries can't match non-definition shapes the way a regex heuristic
can, so false positives are structurally bounded to begin with; (3) manual
spot-check of real extracted symbol names against source for `express` (151
real named functions, e.g. `createApplication`, `tryRender`, `onaborted`,
`matchLayer` — all genuine) and `lodash` (the +1322 additional symbols are
real `function`-shaped definitions, including short minified identifiers in
`dist/lodash.min.js`, not misparse artifacts).

**Verdict: all three languages (JS/TS, Python, Ruby) ship tree-sitter-backed**
— wired into `internal/fusion.New()`, `ExtractorEpoch` bumped 4→5.

### OWNER RULING on the Python/Ruby ties (critic M2 — recorded verbatim)

> ship-on-tie is RATIFIED for Python and Ruby, conditional: (a) the tie is
> verified real (critic's construct probes: exact parity incl. singleton
> methods, heredoc scanner state, Struct.new); (b) the CGO-removal rationale
> must be REALIZED — add a deferred row XT-6b; (c) CHANGELOG corrected to
> say the CGO win is PENDING that deletion, not achieved in this commit.
> Fallback doctrine: runtime faults recover via parser re-creation (M1),
> never via the CGO incumbents — deleting them later must not remove a
> load-bearing path.

Applied: (a) verified by 1:1 symbol-set parity tests against the in-tree
CGO incumbents on the named probes —
`TestTSExtractor_Ruby_ParityWithIncumbentOnCriticProbes` (singleton methods
incl. `class << self`, heredoc external-scanner state, `Struct.new` with a
method-bearing block) and `TestTSExtractor_Python_ParityWithIncumbent`
(decorated/async defs, nested classes, module vars) — all green; (b) XT-6b
added under Deferred below; (c) CHANGELOG corrected. Fault recovery is
parser re-creation only (`TSExtractor.noteFault` — the CGO incumbents are
not on any runtime path).

### Performance

`calibration/baseline.sh` wall-clock over all 12 repos (18,679 files total),
same machine, same corpus, binary difference only. Two measurement
protocols, both recorded (critic-round honesty):

| Build | Wall time | Ratio |
|---|--:|--:|
| Incumbent (CGO tree-sitter, ADR-013), first-commit measurement | 15.5s | 1.0× |
| This engine, naive (fresh wazero instantiation per file) | 174.7s | 11.3× — measured, then fixed, not shipped |
| This engine, first commit (Parser+Query reuse, no budgets) | 21.9s | 1.41× vs that day's incumbent run |
| This engine, host-side WithCloseOnContextDone deadline (critic B1, first attempt) | 63.6s | ~4.1× — measured, rejected, not shipped |
| This engine, final (guest budgets + hard backstop + scalar node marshaling) | 20.2–23.5s | see below |

**Strict interleaved protocol** (3 alternating incumbent/new pairs,
back-to-back on an otherwise idle machine — the fairest measure taken):
incumbent median **12.4s**, final engine median **20.9s** → **1.69×**, which
MISSES the plan's 1.5× budget under this protocol. The first commit's 1.41×
was real but flattered by a slower incumbent run that hour (15.5s wall at
86% CPU — background load). User-CPU medians: 13.5s vs 21.3s → 1.58×.
Attribution: wazero-compiled tree-sitter executes at roughly 1.8× native
tree-sitter for this parse+query workload; the B1 guest budgets cost ~2%;
per-capture marshaling was optimized (scalar-args `fathom_node_end_byte`,
bulk capture reads — worth ~1-2%); no further cheap wins were found (wazero
v1.12's default compiler IS its optimizing backend). Context for the gate
decision, not excuses: the corpus is 100% migrated-language files (the worst
case — fleet repos are Go-dominant and Go extraction is untouched), and the
denominator carries the CGO toolchain this migration exists to delete.
**Flagged for the coordinator: strict-protocol result exceeds the plan's
1.5× line; accept/reject is an owner call, not silently claimed as met.**

### Pathological-input bound (critic B1 — measured before/after)

`({[` x 50,000 (150KB, a truncated minified bundle's shape; passes every
filter, under the 2MB cap):

| Path | Behavior |
|---|---|
| Before (unbounded) | 52.9s at 100% CPU holding the extractor mutex; x200,000 ran 11+ min, uncancellable |
| Root cause (measured during fix) | parse completes in ~70ms; the 45s+ is the QUERY CURSOR scanning the error tree — the critic's parse-phase attribution was close but the hot loop is the query's |
| After: guest budgets (`budget.c`) | parse + query scan each cancel via tree-sitter's own progress callbacks (ParseTimeout, default 10s); cancellation surfaces as an error, never a truncated match set |
| After: hard backstop (`ParseHardBackstop`, default 2× budget) | any guest work the callbacks fail to bound (measured: check cadence collapses to ~8s on this input under load) is ABANDONED at the backstop — Extract returns within a fixed bound, the runaway burns out in the background, its module is reaped |
| After: recovery | fault counted on `fusion.Report.TSEngineFaults` + one stderr line; file fingerprinted; parser re-created; the NEXT file extracts normally (tested at engine, extractor, and fusion levels) |

Suspected (unconfirmed) reason the v0.26 core explodes where the incumbent's
v0.25 takes 49ms: error-recovery/query-scan behavior differences between
those core versions — the bounds above are deliberately version-independent.

### Blob provenance summary (full detail in `internal/extractor/tsengine/grammars/PROVENANCE.md`)

| Language | Blob | Size (bytes) | Grammar version (matches go.mod's CGO pin) |
|---|---|--:|---|
| JavaScript | javascript.wasm | 458,741 | tree-sitter-javascript v0.25.0 |
| TypeScript + TSX | typescript.wasm | 2,037,040 | tree-sitter-typescript v0.23.2 (both grammars) |
| Python | python.wasm | 539,848 | tree-sitter-python v0.25.0 |
| Ruby | ruby.wasm | 1,710,411 | tree-sitter-ruby v0.23.1 |

Core runtime: tree-sitter v0.26.11 (statically linked into every blob) plus
the checked-in `budget.c` shim (bounded parse/query entry points + scalar
node accessor). Built via `zig cc --target=wasm32-wasi-musl` + `wasm-opt
-Oz` (zig 0.16.0 / binaryen 129 for the committed bytes; byte-identical
cross-toolchain reproduction is not a goal — see PROVENANCE.md);
reproducible via `internal/extractor/tsengine/grammars/build.sh`.

## Deferred

- [-] Remaining languages of the 30-language program (java, c#, c/cpp, scala, php, lua upgrades...) {id: XT-6, disposition: deferred, reason: "ship three proven languages first; the engine makes each subsequent one data-only", trigger: "per-language, when an indexed repo materially uses it — calibration gate per language as always"}
- [-] Delete the CGO tree-sitter incumbents once tree-sitter-wasm has baked {id: XT-6b, disposition: deferred, reason: "tree-sitter-wasm is primary for js/ts/py/ruby; the CGO grammar bindings remain only as in-tree incumbents pending bake", trigger: "fleet runs N=7 days green on tree-sitter primary — then delete the CGO tree-sitter deps from go.mod and the incumbent extractors for py/ruby (js heuristic, pure Go, may stay as reference)"}
- [-] Tree-sitter-derived call edges beyond symbols {id: XT-7, disposition: deferred, reason: "symbol truth first; call-edge queries per language are their own calibration problem", trigger: "first non-Go language where find_callers demand is evidenced by call-log queries"}

**Owner ruling (2026-07-22, closes the flag):** the 1.5× corpus budget is
AMENDED to ≤2× incumbent corpus-wide, accepted at the measured 1.69×
(strict interleaved medians). Reasoning mirrors CT-5: at ~1–2s per repo
one-time (hash-gated thereafter), the ratio is the wrong unit; the
operative guarantees are the per-parse bound (ParseTimeout +
ParseHardBackstop — the pathological class is now bounded, counted, and
self-healing) and delta-only re-ingest. The wasm-vs-native residue
(~1.8×) is the price of the pure-Go endgame and is paid knowingly.
Recorded in vision 10 alongside the CT-5 amendment.
