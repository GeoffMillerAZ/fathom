# P05 private email-field rehearsal

Task/state mapping: [contract](../../../design/form-validation-adapter.md).
Observed results and limits: [assessment](../../../assessments/2026-09-14-form-typed-adapter.md).

## Generate and adopt

Discover `reference.form-adapter` with the shared Gallery/terminal catalog or
`search_capabilities(query="email private adapter", profile="cli", kind="reference")`.
Use `get_capability` for its contract, then `get_kit_decision("form-adapter")` for this
guide. This is first-party tooling metadata, not a certified web-component entry;
discovery needs no private corpus, but generation requires verified original access.

```sh
go run ./cmd/appkit data assets-adapt-form
go run ./cmd/appkit data assets-adapt-form -out /existing/private/parent/new-field
go run ./cmd/appkit data assets-adapt-form -check /existing/private/parent/new-field
```

JSON is the read-only default. Explicit export/check requires the independent original
archive; no replacement, build, dependency install, public vendor write or catalog
certification occurs. Exit 0 exported/matching, 1 observed drift, 2 usage/could-not-check.
Copy the five-file delivery into an app-owned private package, compile form.templ with
templ v0.3.1020, and scan **form.go and form.templ** for Tailwind 4 class constants.
Use `NewEmailField(EmailField{...})`; handle errors before writing response bytes.
Error empty selects the authored neutral state. The app owns distinct IDs, reserved
ID-error anchors, native form action, validation, focus policy, auth/CSRF and persistence.
Neither the default invalid example nor an HTTP response proves an email was saved.

## Reproduce local evidence

```sh
go test ./internal/designadapt ./cmd/appkit -run '^(TestForm|TestAssetsAdaptForm)'
APPKIT_PROBE_CONVERSION_KIT_ROOT="$PWD" go test ./cmd/appkit -run '^TestAssetsAdaptFormLive$' -count=1
node docs/assessments/fixtures/form-adapter/prepare.mjs "$PWD"
node docs/assessments/fixtures/form-adapter/browser.mjs /printed/private/temporary/directory
```

Prepare uses a fresh 0700 temporary directory, immutable private export, a separate
consumer module with existing fixture go.mod/go.sum pins, and cached Go/templ/Tailwind
tools. No package installation occurs. It checks missing generated code fails, actual
source/default parsed DOM matches, the HTTP recovery/request/concurrency tests pass,
consumer dependencies exclude app-kit, and source/catalog/delivery remain unchanged.
Its compiler runs in the copied module, not under app-kit's module or private store.
These are cached dependency/compile proofs, not a sandbox, cold build or reproducibility
claim across toolchains. Inter v4.1 and Tailwind4.3.0 are chosen controlled conditions,
not recovered portal runtime pins. The embedded font retains its license.

Browser uses the previously approved isolated Playwright1.63.0 harness and installed
Chrome with fresh contexts, only ephemeral loopback URLs and synthetic addresses.
Application JavaScript is disabled. Original/default screenshot/DOM pairs are compared
at390/1440px in both themes;16 ordinary/long-label keyboard/pointer tasks exercise
native rejection,422,503,303,focus and refresh. Native autofocus waits at most1000ms
without setting focus; that is synchronization, not a performance guarantee.
Screenshots do not prove accessibility. Source44px failures stay recorded separately;
the fixture's44px controls and explicit links are not silently applied to the adapter.

Temporary private output is retained for inspection. Do not publish captures or paid
derivatives. Re-run into a new directory after any input change; never edit delivered
or captured source in place. Neither the full pilot nor any G1–G6 outcome is complete.
