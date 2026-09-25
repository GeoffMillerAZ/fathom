# RAC channel-yield review packet (private, 2026-09-21)

This is a **review-only patch**, not app-kit adoption, a consumer landing, a
release, or permission to change the active RAC checkout. The consumer is
`/Volumes/m2ext/code/geoffmilleraz/resume_as_code`, branch
`kit-governed-adoption`, HEAD
`15f73064e6eec69d9246df8eb1d0bc6336146ecb`. Its worktree is dirty and
independently owned. The ignored overlay source is
`workspace/artifacts/rac-analytics-candidate/`; the patch is a portable
unified diff from the selected active preimage, not from clean HEAD.

## Identity and scope

`channel-yield.patch` (SHA-256
`1059fe4fe98e7b70770fbd67124e3fc856db64bc8976e73d7b945d62780a9169`)
modifies only `cmd/serve/pipeline.go` and `cmd/serve/settings.go`, and adds
`cmd/serve/analytics_export.go` and
`cmd/serve/analytics_acceptance_test.go`. It adds an opt-in aggregate CSV
link/endpoint over canonical `ops.ChannelYield` and synthetic acceptance
tests. It includes no vendor files, generated CSS, broad worktree changes,
user registry, evaluation data, licensed asset, binary, or provider call.

The selected active preimages were checked before packaging:

| RAC path | SHA-256 |
|---|---|
| `cmd/serve/pipeline.go` | `bdcd6cefb5436ccf1c91d598b5b5210dc0b73a3d3ef3c3b144c35d4f8cb74b1d` |
| `cmd/serve/settings.go` | `d58136bd73bbdc297dfd4fd68ae495a97fae9be6d205a37d90322102017134c4` |
| `cmd/serve/assessment.go` | `8aacd1f63d3be83c9f58f36d990abf62d5c8a96fff44b772d88abe8000b76012` |
| `cmd/ops/channels.go` | `cc8848f64243bc9f79e6a0e1913049de0d64030669ba78a69510b8252c1bd2ab` |
| `cmd/serve/pipeline_clarity_test.go` | `143d2c1a8bbfcecab9c0db7396cd0f37d44edfe73649ededc922791cac0f78f6` |
| `cmd/serve/assessment_evidence_test.go` | `ac55fe3cedfe8713865300708c5d380d2ab19c6806bc986bef30e51995dc5fca` |
| `vendor/github.com/geoffmilleraz/app-kit/appkit/ui/viz/bars.go` | `903f35cf1b01ea60ba015fe8787ef629977f2656a9bea11b9e7d1e8fd029efe6` |
| `cmd/serve/web/app.css` | `7a8e0ad74402b2c32bc97409e71477e1af758840f8aa5f6684ae132b457b8aba` |

The four candidate file SHA-256 values, respectively, are
`ef820d6b6158924f5ee173c712c4f6b533288b58f998fe91a7ddd27abcbdfe0b`,
`7010e2775f0212834c85a0c72ad8c4d866943a607f5a9ed5eed854152f366a72`,
`0dbe304e3f6fde551ccad7f966e451b73789be6eb2a017c37d4ff206caa4f16e`,
and `24d63b35ec6bac5206084908551687d928e34021902146fe26d51eb90802d8b0`.

## Synthetic evidence and budget

The empty-state link assertion was red on the original overlay candidate:
`TestCandidateChannelYieldZeroIsNotMissingAndLegacyJSONRemains/no-records`
reported “owning widget must expose the aggregate export in every state.”
The source of that failed test remains in the ignored candidate workspace
outside this patch. The follow-up correction uses one shared link for empty
and populated widget states; the focused replay passed.

Before measurement, the synthetic contract fixed four canonical channel rows,
20 fictional applications (five per channel), widget HTML ≤16KiB, aggregate
CSV ≤8KiB, and combined pure widget+CSV render ≤5ms/op and ≤512KiB
allocated/op on the local benchmark. The test checks exact chart/table/CSV
values and all four CSV rows' units, source, scope, freshness and rule fields.
The benchmark enforces the local time/allocation ceilings.

Reproduction from the RAC root, without applying this patch:

```sh
devbox run -- go test -overlay workspace/artifacts/rac-analytics-candidate/overlay.json -mod=vendor ./cmd/serve -run '^(TestCandidateChannelYield|TestPipeline|TestPeopleReusesChannelEvidenceWithoutNarrowGrid|TestAppPageAssessmentEvidence)' -bench '^BenchmarkCandidateChannelYieldWidgetCSV20$' -benchmem -count=1
```

Observed `PASS`: widget 5,365 bytes; CSV 1,870 bytes; 22,189 ns/op;
35,496 B/op; 285 allocs/op. Environment: Apple M4, darwin/arm64,
Go 1.26.2 via devbox. This is not p95, browser/full-page, update, or
human task-time evidence. The test harness invokes `s.handleExport`
directly: auth invariance is source-level only. The original handler's
other-method behavior is unchanged, not hardened. Separate page/export
loads have no shared snapshot; source freshness is explicitly unknown.

`git apply --check /Volumes/m2ext/code/geoffmilleraz/app-kit/docs/assessments/fixtures/adoption-analytics-2026-09-21/rac/channel-yield.patch`
returned 0 against the active RAC worktree from its root on 2026-09-21.
No patch was applied. Any later preimage drift requires re-review. Applying
or committing the patch, changing consumer guidance/pins, browser validation,
and broader G3/G4 acceptance all await separate active-consumer authority.
