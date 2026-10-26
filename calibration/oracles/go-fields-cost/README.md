# Go field cost and reading reproduction

`cost.py` measures a fixed source snapshot through the baseline and candidate CLI binaries. Supply full 40-character source and candidate commits. It computes actual binary SHA-256 digests and a path/content manifest of the source before the first run, checks the manifest before and after every run, runs three alternating baseline/candidate direct ingests into fresh stores at `nice -n 10`, waits for one-minute load at most 9, and stops a run after 20 minutes. It reads SQLite only after process exit and an empty or absent WAL. Its receipt keeps discovery, typed-work and persisted typed-edge-kind counts, allowing field symbol counts to grow; the arms must otherwise do equivalent work. The gate requires at most +15% median wall and +20% median store bytes.

From a committed fathom checkout, set the full commits and use scratch paths outside every canonical source checkout:

```sh
export REPO="$(pwd)"
export GO_FIELDS_SCRATCH="$(mktemp -d "${TMPDIR:-/tmp}/go-fields.XXXXXX")"
export FATHOM_HOME="$GO_FIELDS_SCRATCH/fathom-home" GOWORK=off GOMAXPROCS=2 GOFLAGS=-p=2 GOTOOLCHAIN=local FATHOM_DECIDE=off
export GOCACHE="$GO_FIELDS_SCRATCH/go-build" GOTMPDIR="$GO_FIELDS_SCRATCH/go-tmp" GOMODCACHE="$GO_FIELDS_SCRATCH/go-mod"
mkdir -p "$GOTMPDIR"
export SOURCE_COMMIT="$(git -C "$REPO" rev-parse 5b84aa5b4^{commit})"
export CANDIDATE_COMMIT="$(git -C "$REPO" rev-parse 4803e3d8b83b9ca132dcc75d5c27e12ff6250ea0^{commit})"
mkdir -p "$GO_FIELDS_SCRATCH/source" "$GO_FIELDS_SCRATCH/stores" "$GO_FIELDS_SCRATCH/logs"
git -C "$REPO" archive "$SOURCE_COMMIT" | tar -xf - -C "$GO_FIELDS_SCRATCH/source"
git -C "$REPO" worktree add --detach "$GO_FIELDS_SCRATCH/baseline-src" "$SOURCE_COMMIT"
git -C "$REPO" worktree add --detach "$GO_FIELDS_SCRATCH/candidate-src" "$CANDIDATE_COMMIT"
(cd "$GO_FIELDS_SCRATCH/baseline-src" && go build -trimpath -buildvcs=false -o "$GO_FIELDS_SCRATCH/fathom-baseline" ./cmd/fathom)
(cd "$GO_FIELDS_SCRATCH/candidate-src" && go build -trimpath -buildvcs=false -o "$GO_FIELDS_SCRATCH/fathom-candidate" ./cmd/fathom)
python3 calibration/oracles/go-fields-cost/cost.py \
  --baseline "$GO_FIELDS_SCRATCH/fathom-baseline" --candidate "$GO_FIELDS_SCRATCH/fathom-candidate" \
  --subject "$GO_FIELDS_SCRATCH/source" --stores "$GO_FIELDS_SCRATCH/stores" \
  --log-dir "$GO_FIELDS_SCRATCH/logs" --out "$GO_FIELDS_SCRATCH/go-cost.json" \
  --source-commit "$SOURCE_COMMIT" --candidate-commit "$CANDIDATE_COMMIT"
```

The candidate is the frozen experiment commit, not this branch HEAD after its runtime rollback. The CLI binaries must be built from the two named commits. Keep the generated receipt and logs together. Existing per-run store or log files cause a refusal so a rerun cannot mix evidence; existing empty directories are allowed.

The Go reading is a separate oracle measurement, using the committed `indexbench` implementation and independent `universal-ctags 6.2.1` and `scip-go 0.2.7` outputs. Clone the two pinned reading subjects into scratch, outside the canonical repository, and run `scip-go index ./...` in each checkout to produce its `index.scip`. The subject SHAs in the committed `docs/bench/readings/lang/go.json` are `45ffa3c668f7105e92089220d42aa64c9acbd11c` for `cli/cli` and `40b5bc1437a564fc795d388b23835e84f54cd1d1` for `spf13/cobra`.

```sh
git clone https://github.com/cli/cli.git "$GO_FIELDS_SCRATCH/cli"
git -C "$GO_FIELDS_SCRATCH/cli" checkout 45ffa3c668f7105e92089220d42aa64c9acbd11c
git clone https://github.com/spf13/cobra.git "$GO_FIELDS_SCRATCH/cobra"
git -C "$GO_FIELDS_SCRATCH/cobra" checkout 40b5bc1437a564fc795d388b23835e84f54cd1d1
(cd "$GO_FIELDS_SCRATCH/cli" && scip-go index ./...)
(cd "$GO_FIELDS_SCRATCH/cobra" && scip-go index ./...)
(cd "$GO_FIELDS_SCRATCH/candidate-src" && go run ./cmd/indexbench lang-oracle \
  -dialect go -subject "$GO_FIELDS_SCRATCH/cli" -subject-name cli/cli \
  -ctags "$(command -v ctags)" -scip "$GO_FIELDS_SCRATCH/cli/index.scip" \
  -commit "$CANDIDATE_COMMIT" -out "$GO_FIELDS_SCRATCH/go-reading.json")
(cd "$GO_FIELDS_SCRATCH/candidate-src" && go run ./cmd/indexbench lang-oracle \
  -dialect go -subject "$GO_FIELDS_SCRATCH/cobra" -subject-name spf13/cobra \
  -ctags "$(command -v ctags)" -scip "$GO_FIELDS_SCRATCH/cobra/index.scip" \
  -commit "$CANDIDATE_COMMIT" -out "$GO_FIELDS_SCRATCH/go-reading.json")
```

The reading commands write only to scratch. Review the generated level, grade, definition and call metrics against the committed reading before claiming any support level.

## Review corrections and source identity

The original collector receipt at commit 327002766 was assembled from an earlier
uncommitted revision. It is historical evidence, not output of this committed
collector. The binding rerun uses `cost.py` unchanged and preserves its emitted
schema/verdict. Any historical commentary belongs only under `annotations`.
The original corrected candidate had stale golden/count tests; the rerun freezes
expectation-only repairs after a full touched-package race suite passes.

Public original commits are b9a23a7dd (fields), 62383387b (protocol), and
68ef5163780197afde9346bddad720b6076677d9 (anchor repair). An unpublished measured
original was tree-identical to 68ef51637 over these paths. Reproduce that identity
hash on each available commit with:

```sh
git ls-tree -r 68ef5163780197afde9346bddad720b6076677d9 -- cmd internal calibration/oracles go.mod go.sum | shasum -a 256
```

The historical listing hash is
e619d6ebad42da55d2cb070ee173eea2b7ccb3b8d5365bd4c2fe680a2779c053.
This is a tree-listing identity, not a portable executable hash. Binary digests
are local identity only: host, toolchain, path and VCS stamping can change them.
For new controlled builds use the same flags for both arms (`-trimpath
-buildvcs=false`) and record the Go version/architecture and applied settings.
Keep the source snapshot fixed even when candidate test expectations change.

The strict rerun candidate is `4803e3d8b83b9ca132dcc75d5c27e12ff6250ea0`, frozen after the full
core/extractor/callgraph/fusion/langbench race suite passed. Relative to 68ef51637,
only test expectations and main's hook regression test differ in internal/cmd/pkg;
production code is unchanged. No historical red candidate is silently called green.
