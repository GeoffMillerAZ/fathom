# Offline uplift delivery evidence

Scope: UP-1..6, decision 0032, command-local diagnostics and a synthetic W1 reference.
The containing commit owns the implementation. Pre-commit tests/builds used `1aca786`
plus the reviewed working changes; this is not a claim that those binaries were clean
committed builds. No exported library API or external Go dependency changed.

| Instrument | Observed result |
|---|---|
| Final `task validate`, external `TMPDIR` | Exit 0: formatting, vet, CUE, shell/lifecycle, constitution and repository race suites, including nested modules |
| `go test -race ./cmd/quality-gate -count=1 -timeout=180s` | Exit 0 (57.090s): diagnostics, model aliases, eligible bands, one injected clock, usable Git, secret omission and bounded quoted paths, plus existing enforcement regressions |
| `go test -race ./examples/practice-review -count=1` | Exit 0 (2.855s): independent coverage, semantic definition/subject/check binding, trusted limits, wrong-subject evidence, data bounds and malformed inputs, expiry and writer failure |
| `scripts/quality-gate-smoke.sh` | Exit 0: installed Lefthook jobs, staged-invalid/worktree-valid refusal, empty commit, full-tree and multi-ref rejection, valid raw-OID push, observation and malformed inputs |
| CUE fixtures and actual doctor JSON | Three practice input fixtures, doctor fixture and a real local-bootstrap doctor receipt passed their selected definitions |
| Direct compiled example | Exit 0, two subjects, six subject-specific bound packets, zero coverage gaps |
| Skill/guide trials | Skill validator exit 0; guide-only practice trial first useful result in 6s, complete 25s; configuration trial 21s; documented 0/1/2 cases reproduced |
| Documentation checks | 258 changed-file relative link targets resolved; root router 80 lines, scope 145, decision 56; generated catalog/README count is 213 |
| Independent review | [Doctor](doctor-review.md), [practice](practice-review.md), [final review](final-review.md): reproduced findings repaired; no remaining blocking finding in the bounded final review |

The first full gate refused stale generated catalog/README counts after adding the
example. Regeneration repaired that drift; no check or gate was weakened. The two
completed worker worktrees were removed after their complete diffs were preserved
and byte-compared with the integrated patches. Only the integration worktree remained.
The initial CUE walk included attached worktrees; future waves can close completed,
preserved worktrees before the final gate to avoid that extra scan. No gate edit.

See the [guide trial](guide-trial.md) for exact times and working-build identity.
These are agent-assisted synthetic runs on this host, not fresh-machine or human
usability measurements. Live provider/model quality, savings, Review Kit exchange,
external consumer adoption, source authenticity, actual effect authorization and
protected CI integration remain unverified. Whole AE-1/AE-2 and later waves stay open.
Fathom's partial/stale review and incomplete obligations remain O11, not a clean static
proof. Downstream consumer checks were not required or claimed for this command/example
change outside the five library roots. Review admission by 2026-11-21 or next release.
