# Benchmark evidence: first consumer bridge

This overlay-only test runs inside resume_as_code's actual `cmd/redteam` package at
`7a6c56a26dd10a7cf80128d23ef13bab90377ae8`. It uses its existing scripted critics,
`Faithfulness`, provider-family checks and `Ladder`, then converts accepted, rejected
and unavailable results into the kit's experimental benchmark receipts.

No consumer source, vendor pin, model configuration or résumé data is changed. No paid
call occurs. All data is synthetic and zero durations are injected; unknown costs stay
unknown. The statistical comparator must refuse this bundle. This is a consumer-code
integration proof, **not adoption, a live benchmark or an effectiveness result**.

`resume_as_code_test.go.txt` is deliberately not a Go package in the kit: importing a
consumer would invert dependencies and introduce external requirements. Use a Go test
overlay mapping `<consumer>/cmd/redteam/kit_benchmark_pilot_test.go` to this file. Use a
temporary copy of that consumer's go.mod/go.sum with an explicit kit replacement and
the consumer's pinned app-kit replacement; never rewrite its actual module or vendor.

Run the overlay with `GOWORK=off GOPROXY=off go test -mod=readonly -modfile=<temp>/go.mod
-overlay=<temp>/overlay.json -race ./cmd/redteam -run TestKitBenchmarkReceiptPilot
-count=1 -v` from the consumer root. Resolve replacements and inspect `go list` before
claiming the proposed kit was tested. The paths are local caller choices, not defaults.

The example hashes the consumer code and synthetic configuration actually used. A
production collector must also account for the complete generation/revision/quality
workflow, hidden retries and all leaf calls; bind collector, artifact and dataset
provenance; record timing and price versions; and validate held-out acceptance. Reusing
this fixture as production performance evidence is invalid.

[Design and next integration](../../docs/designs/benchmark-evidence.md).

[Recorded verification and retained receipt](../../docs/evidence/benchmarks/2026-09-16/README.md)
include exact acceptance-artifact bytes. The kit's hermetic
`TestBenchmarkRetainedConsumerReceipt` checks reconstruction without requiring a consumer
checkout. Running the overlay remains a separate, pinned consumer-code integration test.
