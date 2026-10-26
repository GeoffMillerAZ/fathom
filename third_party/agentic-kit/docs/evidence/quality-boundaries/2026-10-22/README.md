# Quality boundary delivery evidence

Scope: experimental `cmd/quality-gate`, decision 0030 and QB-1 through QB-8. The command composes existing decision foundations and adds no library API or external Go dependency. The tracked implementation is the containing commit; local build/smoke receipts identify the earlier HEAD plus working changes, without claiming a clean committed build.

| Instrument | Observed result |
|---|---|
| `task validate` with external `TMPDIR` | Exit 0: formatting, vet, constitution/CUE checks, shell checks and repository race suites, including nested modules |
| `go test -race ./cmd/quality-gate -count=1` | Exit 0: strict policy, exact Git snapshots/drift/partial clone, hard rules, model/HTTP doubles, calibration, cache and CLI negatives |
| `scripts/quality-gate-smoke.sh` | Exit 0: actual installed commit-msg/pre-push in disposable Git repo and bare remote, preserved existing jobs, staged-invalid/worktree-valid refusal, empty commit invocation and whole multi-ref push rejection |
| Independent review and follow-up | Four initial findings repaired and regression-tested; final review found no additional concrete bypass in the scoped command |
| Skill forward task and `quick_validate.py` | Independent fixture adoption refused invalid and missing-policy commits; root validator exited 0 using isolated PyYAML tooling |
| Fathom `review_diff` | Partial 5/8 dimensions with stale/incomplete obligations; not a clean review or runtime proof |
| `push-check.sh` | Exit 0: existing Lefthook pre-push wiring present; actual publication hook result is checked during push |

Details: [installed hook receipt](lefthook.md), [security review and dispositions](review.md), [independent skill task](skill-forward-test.md). Tests use local Git fixtures, HTTP doubles and injected decisions, never paid providers. CUE parity has targeted positive/negative fixtures, not exhaustive equivalence. The tested CUE vet permits some omitted required collections; the Go decoder checks their presence independently.

No compatibility surface API changed, so downstream consumer checks were not required or claimed. No real consumer/global hook or protected-branch configuration was modified. Live provider calibration, production adoption, false-clear/false-hold rates, cost savings and review effort remain unverified. This closes the bounded offline reference, not those adoption or measurement goals. Review decision 0030 by 2026-11-21 or the next release.

## Excellence pass

Decision 0031 and QB-9 through QB-14 refine the same experimental command. The
[independent review](excellence-review.md) records four additional findings and their
resolution, including the accepted distinction between required and advisory inputs.

| Instrument | Observed result |
|---|---|
| `task validate` with external `TMPDIR` | Exit 0 after the final code delta: repository formatting, vet, CUE and race gates, including nested modules |
| `go test -race ./cmd/quality-gate -count=1 -timeout=90s` | Exit 0 (13.115s): Git shim refusal, raw-OID parsing, full recipe/cohort binding, HTTP bounds, shared dispatch budget, required input preflight and receipt coverage, shadow observation and deterministic ordering |
| `scripts/quality-gate-smoke.sh` | Exit 0 with actual Lefthook hooks and a local bare remote; adds a real Git raw-OID push to the prior negative and multi-ref cases |
| Independent final review | No remaining blocking finding within the declared source/design/skill scope; no live-provider or consumer CI proof |
| Updated skill validation | `quick_validate.py` exit 0; narrow adoption trigger retained, no global installation |

The smoke binary came from `d9191ec` plus these working changes. Tested versions:
Git 2.54.0, Go 1.26.3, CUE 0.16.1 and Lefthook 1.13.6. No external Go dependency or
exported library API changed; downstream consumer checks were not required or claimed.
Fathom's incomplete obligation compilation remains O11, not a verified review pass.

[DW-1 through DW-9](../../../designs/deterministic-workflows.md) are durable composition
designs. Their runtime and adoption work remains in the existing AE work order. The
original live-quality, cost, adoption and review-date limits above still apply. Old model
calibration requires genuine re-evaluation under the new recipe, not replacement hashes.
