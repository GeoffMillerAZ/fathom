# dockerfile — provenance

- Grammar: `camdencheek/tree-sitter-dockerfile v0.2.0` (commit `868e44ce378deb68aac902a9db68ff82d2299dd0`),
  built into `internal/extractor/tsengine/grammars/dockerfile.wasm.gz`; pins and digests in
  `internal/extractor/tsengine/grammars/PROVENANCE.md`/`grammars.lock.json`. The queue entry's
  `licence` field says Apache-2.0; the repository's actual top-level `LICENSE` at the pinned
  commit is MIT (`build.sh`'s licence check reads and records what is actually there, not the
  queue's proposal) — MIT is on the §8.2 allowlist, so this is a discrepancy worth noting, not
  a blocker.
- `tags.scm`: written from scratch in this repository (design §13 step 11/12), never generated
  or copied. No source in the grammar's own `queries/` (only `highlights.scm`, no `tags.scm`),
  aider's query set, or nvim-treesitter carries a Dockerfile `tags.scm` to start from. Node and
  field shapes came only from `indexbench lang-probe` over `testdata/*.dockerfile` — never from
  reading `grammar.js`/`node-types.json`. See the file's own header for the deliberate limits:
  stage spans are single `from_instruction` lines only (the grammar keeps every instruction a
  flat `source_file` sibling, no stage subtree), so `COPY --from=<stage>` is captured nowhere —
  a copy_instruction several instructions after its stage's `from_instruction` is never
  contained by it, and `attachTSCalls`'s containment rule would silently drop such a capture.
- `testdata/`: fixtures written in this repository; goldens (`*.golden.json`) written by hand
  from reading each fixture, verified against `indexbench lang-probe` output, not generated
  from a working extraction.
- Port (`dockerextract.go` retired, epoch 16, `internal/fusion/pipeline.go`): the incumbent's
  only `SourceKind`, `DockerStage`, is kept as this dialect's `stage` kind's `source_kind`
  (compatibility, design §2.2's rule). `DockerfileArg`/`DockerfileEnv` (ARG/ENV key inventory)
  are new — the incumbent never extracted them. `Tier` moves from `TierPreScan` (a hand-written
  line scanner) to `TierVerified` (a real grammar parse). A stage's `Calls` now includes every
  FROM base reference unconditionally — external images too, not only a match against an
  earlier stage's alias — the tags engine's "collect every call site, unresolved" convention
  every other tags dialect already follows (ADR-021 §3); the incumbent only recorded a Calls
  entry when the image ref provably matched a prior stage name.
- Measurement (`docs/bench/readings/lang/dockerfile.json`, 2026-09-28): oracle A is
  `buildkit-dockerfile-parser` — `github.com/moby/buildkit/frontend/dockerfile/{parser,
  instructions}` v0.33.0, the same official parser family `docker buildx build --print=outline`
  itself is built on (`calibration/corpus.yaml`'s own pre-existing Dockerfile curation already
  names this package as the format's `go_parser_lib`). No Docker daemon is reachable in this
  sandboxed worker (Colima is not running; starting one is out of scope and disk-prohibitive
  for a single queue item), so `docker buildx build --print=outline` itself cannot run; the
  scratch tool that walked this same official AST offline (never committed; a throwaway,
  outside fathom's own module) is `/Volumes/m2ext/tmp/lang-dockerfile/oracle-gen/main.go` at
  measurement time — reproducible from `github.com/moby/buildkit/frontend/dockerfile` v0.33.0
  directly. `hadolint --format json` is a linter (parse-only signal), not a definitions source,
  so it named no oracle role here; universal-ctags 6.2.1 has no Dockerfile language at all
  (tier B and the tree-sitter-CLI tier-C fallback both unavailable — no upstream `tags.scm`
  exists to run through it either). Calibration subjects: `docker/cli` (d1c06ef, `dockerfiles/`
  + root `Dockerfile`), `docker/compose` (a32dc3d, `pkg/e2e/fixtures/**/Dockerfile`) — both
  already curated in `calibration/corpus.yaml`'s pre-existing Dockerfile entry, reused rather
  than re-pinned — and `docker-library/python` (8f2cb2e, no release tags; `master`'s HEAD),
  added as this queue entry's own `calibration/corpus.d/dockerfile.yaml` shard. 73 files, zero
  faults: `sym_recall` .9949, `sym_precision` .9975, `ref_recall` 1.0, `ref_precision` 1.0 — L1
  and L2 both excellent. L3 is `needs-strategy` (see the plan row's disposition,
  `LB-Q-dockerfile-rest`): no tags-engine import strategy maps a local image tag to the
  Dockerfile that built it, and `internal/langbench.recompute()` does not score
  `xref_precision`/`xref_recall` for any `kind: "tooling"` dialect yet.
