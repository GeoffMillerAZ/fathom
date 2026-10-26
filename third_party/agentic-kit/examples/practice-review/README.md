# Offline practice review example

Run from the repository root. The fixture contains two synthetic review workflows and three required properties.

```sh
go run ./examples/practice-review \
  --catalog examples/practice-review/fixtures/catalog.json \
  --scope examples/practice-review/fixtures/scope.json \
  --subjects examples/practice-review/fixtures/subjects.json \
  --at 2026-10-23T00:00:00Z \
  --format json
```

Exit 0 means all independently expected subjects and criteria are present and their deterministic fixture properties pass. Exit 1 means a property fails. Exit 2 means invalid input or unknown coverage, including a missing subject, criterion, changed definition pin or expired evidence. The receipt has exact input file SHA-256 digests, catalog and six subject-specific prepared binding digests, sorted coverage gaps and per-subject assessment traces. Reordering collections preserves the logical result and trace order. It changes raw input file digests if the JSON bytes change. No overall score is emitted.

The `scope.json` file is the separately authored trust input. It pins exact criterion stable keys, versions and semantic definition digests, plus every expected subject revision, required criterion list and allowed actions. Keep it independent of the catalog and subjects. The semantic digest covers every practice meaning and behavior field, including check kind, trusted work limit, remedy and sources; it excludes only the movable catalog path. Catalog Kit resolves stable keys and seals each subject check against its exact definition and independent allowed actions. The subject file supplies evidence bytes, recorded steps and requested actions. Requested actions are checked against **only** the allowed actions in scope.

The checks evaluate supplied configuration and evidence bytes. Each evidence item declares a subject ID and revision, which are checked against the independent scope. These are caller assertions: a digest match does not authenticate the source, prove retention, show that a source claim is true, or authorize an actual effect. This example does not exchange findings with Review Kit, attest production readiness, or complete the full AE-1/AE-2 program.

Plant an established failure by copying the subjects fixture and changing `checks[1].work.steps` for `review-alpha` from `2` to `4`; the pinned catalog definition sets `max_steps` to `3`. The CLI exits 1. Delete that check entirely and it exits 2 with a coverage gap and an unknown assessment trace. Adding `max_steps` to a subject check is rejected as an unknown field.

Validate the typed fixtures and tests:

```sh
cue vet examples/practice-review/schema.cue examples/practice-review/fixtures/catalog.json -d '#Catalog'
cue vet examples/practice-review/schema.cue examples/practice-review/fixtures/scope.json -d '#Scope'
cue vet examples/practice-review/schema.cue examples/practice-review/fixtures/subjects.json -d '#Subjects'
go test -race ./examples/practice-review
```

Adoption requires an independently maintained expected scope, real subject identities and revisions, source-authenticated evidence, and an effect enforcement boundary. None is inferred from this fixture.
