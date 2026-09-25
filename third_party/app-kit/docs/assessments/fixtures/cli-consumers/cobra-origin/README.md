# Private CLI invocation-origin experiment

ADR-0070 now admits [explicit consumer adoption](../../../../adr/0070-opt-in-cli-invocation-classification.md).
The preparation and historical experiment below retain their original boundaries;
app-kit's root dependency pins remain upstream. This tool is also the maintained
source of the selected consumer dependency snapshots, not permission to update them.

## Reviewed consumer delivery

`deliver.mjs stage <absolute-candidate-root> <fresh-absolute-target>` verifies the
reviewed full source receipt and every file before creating a new delivery. It keeps
both upstream licenses and all source/tests. The only added wiring upgrades the copied
Cobra module's pflag requirement to1.0.10 and selects `../pflag`, so independent module
tests use the reviewed pair. Source receipt and delivered-file manifests remain separate.

`deliver.mjs check <absolute-delivered-root>` checks the pinned source receipt, exact
file inventory and bytes, deterministic module wiring and exact delivery manifest.
Missing, extra, symlinked or changed sources fail; changing a source and its authored
delivery digest together still fails. The checker is an explicit maintainer command,
not an installed fleet hook or runtime dependency. Node is only maintainer tooling.
The tool never overwrites or deletes; a failed stage is retained for diagnosis and
requires a new fresh target. Caller-selected real consumer updates need authority.

Each selected consumer adds module replacements for both copied modules and directly
enables Cobra's origin setter before `ExecuteWithClassifier(..., InvocationOriginCode)`.
Do not use optional interface detection as a fallback. Upgrades must rerun source checks,
upstream suites, origin/default compatibility and consumer usage/operational regressions.
Regenerate the same candidate twice and compare receipts; never edit generated copies.
When pins/patches change, review the preparer, the delivery tool's receipt identity and
module wiring together. No cache mutation, public fork or installed-binary upgrade occurs.

## Original experiment and oracle

[Decision](../../../../adr/0048-prove-cli-invocation-origins-before-adoption.md),
[default-off policy](../../../../adr/0052-default-off-cli-invocation-origins.md) and
[results](../../../2026-09-15-cli-invocation-origins.md). No root dependency adoption.

`go run ./docs/assessments/fixtures/cli-consumers/cobra-origin` prepares the exact
cached Cobra v1.10.2 and pflag v1.0.10 sources into a fresh absolute output outside
the working checkout. Full source module sums are mandatory. It copies only verified
regular-file snapshots, applies bounded constructor changes and added `.go.txt` files,
and retains upstream licenses. It makes no network request or consumer mutation.

Run from the app-kit root with explicit reviewed paths:

```sh
go run ./docs/assessments/fixtures/cli-consumers/cobra-origin \
  --cobra /absolute/module-cache/github.com/spf13/cobra@v1.10.2 \
  --pflag /absolute/module-cache/github.com/spf13/pflag@v1.0.10 \
  --out /absolute/fresh-parent/candidate
```

Copy the current root `go.mod` and `go.sum` into a private temporary `probe.mod` and
`probe.sum`, then use `go mod edit -modfile=/absolute/probe.mod` with these two replaces:

```text
github.com/spf13/cobra=/absolute/fresh-parent/candidate/cobra
github.com/spf13/pflag=/absolute/fresh-parent/candidate/pflag
```

Do not edit root pins or the cache. Inherit the temporary modfile through GOFLAGS
so upstream tests' child Go builds use the same exact graph:

```sh
GOWORK=off GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
GOFLAGS=-modfile=/absolute/probe.mod \
go test -race -count=1 github.com/spf13/cobra github.com/spf13/pflag

GOWORK=off GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
GOFLAGS=-modfile=/absolute/probe.mod \
go test -race -tags cobraoriginprobe -count=1 \
  ./docs/assessments/fixtures/cli-consumers/cobra-origin
```

Repeat the same tagged oracle with `GOFLAGS=` on the unchanged baseline; typed-origin
assertions must fail. A compile-only pass is not oracle execution. The tagged helper
enables origins only when the candidate API exists; absence never skips assertions.
Default-off compatibility tests execute without that helper. Run the unchanged kit
command tests under both graphs: the initial candidate's four identity failures stay
historical evidence; the default-off candidate must pass them without test changes.

The isolated candidate adds `(*cobra.Command).SetInvocationErrorOrigins(bool)`.
The nearest explicit ancestor setting wins; no setting means false. Configure before
execution. Reparented/generated commands consult their actual current ancestry.
When enabled, returned errors may gain wrappers; callback input, diagnostics and
causes remain intact. No classifier or automatic exit-code change is introduced.
This setter does **not** exist in the kit's unchanged root dependency graph.

Normal untagged tests exercise only preparation safety/determinism. The historical
experiment's classifier is test-only; ADR-0070 separately promotes the reviewed policy
into an opt-in kit API, with explicit exits first and mixed/unknown errors unclassified.
The directory is part of the existing root module, not a new nested module or release.
The preparer protects against accidental drift, not malicious concurrent filesystem
writers; receipts are selected working-tree evidence, not signed releases.
