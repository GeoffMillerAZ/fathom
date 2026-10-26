# Uplift evidence, 2026-10-22

This directory separates external design inputs, a repository contribution audit,
and an actual offline assessment of the existing source. None establishes new
runtime features, production adoption, model calibration or saved human time.

- [Research](research.md): 31 dated primary-source entries and limits.
- [Contributions](contributions.md): inspected refs, PRs, source deltas and cleanup coverage.
- [Offline report](offline/REPORT.md): actual `AssessProfile` run, 18 definitions,
  source/evidence pins, five uncached local test groups and CUE checks.
- [Assessment](offline/assessment.json): caller-declared required `fail`, null soft
  score, 11 scoped component passes, 2 missing targets, 5 unknown composition gates.
- [Bundle manifest](offline/bundle-sha256.json): exact retained artifact hashes/bytes.
- [Artifact map](offline/artifact-map.json): historical absolute receipt paths to
  retained relative artifacts, with the original content hashes unchanged.

## Reproduce the historical assessment projection

From the root checkout, copy `offline/assess.go.txt` to a scratch directory outside
any module, then run it with the checked-in input and an output in that scratch
directory. The input is self-contained; the historical observation/evaluation time
is injected. For example, with an existing absolute `$audit_scratch` outside the repo:

```sh
cp docs/evidence/agentic-engineering/2026-10-22/offline/assess.go.txt "$audit_scratch/assess.go"
go run "$audit_scratch/assess.go" docs/evidence/agentic-engineering/2026-10-22/offline/input.json "$audit_scratch/assessment.json"
cmp docs/evidence/agentic-engineering/2026-10-22/offline/assessment.json "$audit_scratch/assessment.json"
cue vet foundations/quality/fitness-function-registry/profile.cue docs/evidence/agentic-engineering/2026-10-22/offline/profile.json -d '#Profile'
cue vet foundations/quality/fitness-function-registry/profile.cue "$audit_scratch/assessment.json" -d '#Assessment'
```

This recomputes a projection of retained observations, not their truth or current
freshness. To collect new evidence, rerun the exact command groups in `commands.json`
against a new source manifest, adjudicate every criterion and preserve a new dated
receipt. Do not reuse old validity or fabricate new independent trials. Historical
absolute paths in raw receipts name the actual run; all referenced test/JSON logs
are retained here by basename. The driver is evidence text, not a new public CLI.

## Documentation delivery verification

Checks passed before publication: full `task validate`; local links and all 36
request mappings; decision/index/work-order reconciliation; all 55 offline
artifact hashes and their staged membership; byte-identical offline assessment
replay with network module resolution disabled; Profile/Assessment CUE checks;
and `git diff --cached --check`. Independent review found and resolved the raw-log
publication gap and the undeclared-dependency overclaim; final recheck was clear.

Fathom's strict check of the new work order exited 0 because it contains **zero
checked claims**. Existing symbol anchors do not prove new behavior. The first
anchor report contained one failed and nine unknown static anchors (including
deferred rows); a type-only workbench anchor was corrected to the existing commit
method. No runtime task was closed from that report. The final normal push runs
the repository's existing source-backed proof and validation hooks.

Runtime library source is unchanged; downstream consumers are not mutated or
re-certified by this documentation change. Final commands/results are recorded
in the commit's `Verified:` line. No new implementation, live quality, security
certification or consumer adoption is implied by documentation delivery.
