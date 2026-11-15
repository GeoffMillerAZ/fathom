ENG-1 verdict: INCONCLUSIVE

RT-195 implementation and equality proof, 2026-11-15. Base snapshot:
`a6c67b22038f6f056d267305b830ed5becc74403`. The lane's implementation commit
contains this reading and the three callgraph source/test files.

`Build` now constructs a directory → dot-suffix → candidate-keys index once,
uses it for constructor-scoped calls, and releases it when the build returns.
The graph retains no constructor index. `resolveCtorScoped` is unchanged:
the scan remains the independent test oracle and the package-patch resolver.

## Evidence

The prescribed test was written and executed before implementation:

```text
ctor_scope_index_test.go:95:11: undefined: buildCtorScopeIndex
FAIL github.com/geoffmilleraz/fathom/internal/analysis/callgraph [build failed]
```

After implementation, under the gate lease:

```text
go test -race ./internal/analysis/callgraph -run '^TestCtorScopeIndexMatchesScan$' -count=1
ok github.com/geoffmilleraz/fathom/internal/analysis/callgraph 1.899s
go test -race ./internal/analysis/callgraph -count=1
ok github.com/geoffmilleraz/fathom/internal/analysis/callgraph 9.902s
```

The equality test checks constructor resolutions and complete sorted syntax edge
sets against the unchanged scan over 30 seeded generated sets, explicit corner
cases, and Go extraction of the existing fauxtown fixture. It checks four input
permutations per set, colliding directory suffixes, nested import paths,
same-named methods, absent methods, empty/root file paths and Go fields.
The existing served-call golden test passed in the package run.

## Unproven acceptance

No cold-build timing was taken. Test durations above are test process durations,
not cold graph build measurements. Neither the RT-195 ratio of at most 0.1 nor
the ENG-1 deployed build below one second is established.

Deployment preflight found zero valid code-signing identities. `task deploy`
requires the stable `fathom-dev` identity and refuses an ad-hoc replacement.
The reachable daemon reported an existing ad-hoc binary. No deploy was attempted,
no serving HEAD assertion was obtained, and no deployed binary identity or timing
is claimed. Creating a signing identity changes global configuration and is
outside this lane's owned paths.

The integration owner must supply the signing identity, measure indexed and scan
cold builds over the same pinned snapshot, deploy with `task deploy`, retain its
serving HEAD assertion, and read the deployed cold build. RT-195 and ENG-1 remain
unchecked. No scratch-ingest verification or full integration gate is claimed.
The integration owner also owns the observable-performance change note because
this lane's path allowlist contains no changelog fragment.
