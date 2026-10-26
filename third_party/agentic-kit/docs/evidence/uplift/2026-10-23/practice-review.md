# Practice-review initial review (2026-10-23)

Scope: read-only review of the uncommitted `examples/practice-review/**` patch against main `1aca786`, UP-3 through UP-6, and decision 0032. Findings describe the initial state; dispositions describe the revised patch verified later the same day. This is an offline synthetic fixture review, not a claim about source authenticity, actual effect authorization, external adoption, or completed AE rows.

## Blocking findings

**F1 - The trusted pin did not bind the check's meaning.** The initial code pinned only `Practice.Definition` and chose behavior from unpinned `Practice.Kind`. Changing `bounded-work` to `evidence-integrity` under the same version and definition, with `steps=4,max_steps=3`, made the CLI exit 0. **Resolved:** `main.go:227-249,343-348` now hashes a canonical typed definition containing kind, the trusted work limit, and meaning-bearing metadata, excluding only catalog path. A structurally valid kind switch and a changed trusted limit each produce exit 2 with a changed-definition gap. The path-move test still passes.

**F2 - An effect request on another check was ignored.** Adding `requested_actions:["publish"]` to `review-alpha`'s `bounded-work` check made the initial CLI exit 0. **Resolved:** `main.go:430-442` rejects a kind-incompatible action or work field. The same CLI mutation now exits 2; the supported effect-scope check still compares every requested action with the independent allowed set at `main.go:479-489`.

**F3 - Subject observations were detached from evidence identity.** Copying `review-alpha` evidence onto `review-beta`'s evidence-integrity check made the initial CLI exit 0. **Resolved:** `main.go:251-280,413-417,448-465` hashes the canonical full subject bytes, seals a subject-specific check and independent allowed-action scope in each prepared packet, and compares evidence `subject_id` and revision to the expected subject. The same CLI mutation now exits 1 with beta failed. The baseline receipt has six subject-specific bindings. These checks do not authenticate an external source.

**F4 - Go accepted subject IDs that CUE rejected.** Replacing `review-alpha` with `review alpha` in both scope and subjects made the initial CLI exit 0 while CUE vet rejected both files. **Resolved:** `main.go:29,205,285,367-382,403-405` now checks keys, bounded counts, and required collection presence. The same CLI mutation exits 2. Revised CUE constraints at `schema.cue:10,27-63` cover the admitted counts and payloads.

## Evidence and limits

The baseline CLI exits 0. All four original counterexamples were run against initial and revised locally built binaries with copied fixtures outside the repository. A delta review also found two CUE-invalid payloads: negative work steps initially exited 1, and a fractional-second `observed_at` initially exited 0. `main.go:216-218,430-432` now rejects both as invalid; direct CLI runs exit 2 and CUE vet rejects the same files. The revised `TestCLIRejectsCueInvalidValues` covers both. The three shipped positive CUE fixtures pass `cue vet`; `go test -race ./examples/practice-review` passes on the revised patch. No blocking finding remains in this bounded review.

`fathom review_diff` timed out and obligations returned raw, incomplete guidance. Findings and dispositions rest on working-tree source, tests, and executable probes, not a clean Fathom verdict. Full `task validate`, external adoption, and the full AE acceptance exits were outside this review.
