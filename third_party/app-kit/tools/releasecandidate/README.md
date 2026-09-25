# Private module candidate preparation

Maintainer-only G4 tool under [ADR-0045](../../docs/adr/0045-exact-pins-and-private-candidate-preparation.md).
No publishing, tagging, consumer mutation, dependency installation or source writes.
This is not a new `appkit` command or an automatic release workflow.

## Prepare

Run from the maintained app-kit checkout with its existing Go dependencies available:

```sh
go test ./tools/releasecandidate
go run ./tools/releasecandidate -root /absolute/app-kit -out /fresh/private/candidate
```

The output's parent must exist; the output itself must not exist and must be outside
the source. Preparation creates private directories/files and emits a JSON receipt to
stdout and `receipt.json`. Diagnostics go to stderr; exits are 0 success/help, 2 usage,
1 failed preparation/output. On any write failure, retain and inspect the incomplete
output; a retry needs a fresh path. It never overwrites or deletes existing output.

## Identity and source selection

The selected source consists of tracked plus untracked nonignored files under
`appkit`, `apps`, `cmd`, `internal`, `cue`, `tools`, `docs`, `fathom-pack`, plus explicit
root module/license/document/task files (the authoritative list is in `candidate.go`).
Git-visible tracked files remain selected even if an ignore rule would now match them.
Paid `data/`, native `spikes/`, `.git` and global agent configuration are not selected.
Stable Git-reported deletions and nested Go modules are excluded with recorded reasons;
other ZIP omissions fail. A file disappearing during its selected read still fails.
Symlinks, special files, unsafe paths, `.env`-prefixed leaves and local module
replacements are refused. Limits: 4,000 selected files, 8 MiB/file, 64 MiB total.

Two whole selected path/byte snapshots and source HEAD must agree. This detects
observed drift, not arbitrary concurrent writers or an atomic filesystem snapshot.
The full selected-tree digest identifies `v0.0.0-rehearsal.h<DIGEST>`; the receipt binds
sorted paths, byte counts and SHA-256 hashes, HEAD, ZIP SHA-256 and Go module/`go.mod` sums.
The ZIP is generated twice from the same retained bytes and must match byte-for-byte.
Proxy `.info` uses the fixed Unix epoch, deliberately not a source/release timestamp.

Review the receipt's file inventory before reuse. The allowlist and `.env` refusal
are not a secret or license scan. First-party adaptations may still have commercial
provenance; **keep the complete candidate private**. Untracked selected source is
intentional and does not become clean-commit or release evidence.

## Consume for a separate rehearsal

The output contains a standard file proxy:
`proxy/github.com/geoffmilleraz/app-kit/@v/<VERSION>.{zip,mod,info}` and `list`.
Use the receipt's exact version in a fresh synthetic consumer, without any replacement.
Keep public module checksums enabled and private exceptions specific to app-kit.
Use only explicit local dependency proxies, empty build/module caches and source/network
denial for the build proof. Assert selected CSS selectors and meaningful rendered/export
content separately; a compiler exit alone does not show a useful styled application.

On macOS, the bounded synthetic harness performs two empty-cache builds and handler/CSS
checks. Supply an existing public module-download cache; no network fallback is allowed:

```sh
go run ./tools/releasecandidate/rehearsal \
  -receipt /fresh/private/candidate/receipt.json \
  -candidate-proxy /fresh/private/candidate/proxy \
  -tailwind /absolute/cached/tailwindcss-macos-arm64 \
  -public-proxy /absolute/go-module-cache/cache/download \
  -out /fresh/private/rehearsal
```

It verifies the embedded platform Tailwind pin, candidate ZIP/inventory and both Go
sums; embeds/fingerprints its fixture; denies network and the owner's two fleet source
roots with `sandbox-exec`; and retains command hashes, module resolution and output hashes
in `rehearsal-receipt.json`. Commands have a five-minute limit. Scope: synthetic shell,
navigation, bar/table fallback and CSV; explicit runtime-only CSS sources and a poison
selector exclusion. The fixed source-denial roots make this a local maintainer harness,
not a portable sandbox or production acceptance suite.

The embedded `fixture/main.go` is also compiled/executed by root Go tests. Its CSV
reader requires the exact ten-column ADR-0056 schema and declared row meaning;
eight-column, header/order, meaning, missing-value and row-count drift are refused.
Only the three deployment inputs are embedded; fixture tests stay outside the bundle.

Preparation does not run a consumer, promote a catalog record, or certify browser/native
behavior. Rehearsal receipts, reviewed migration notes and any authorized real-consumer
upgrade remain separate evidence. Tags/clean-commit releases require separate authority.

### Opt-in real Gallery host rehearsal

Add `-host-source /absolute/app-kit/spikes/wails-gallery` to the rehearsal command
to select the actual experimental host instead of the three-file synthetic consumer.
All existing candidate/proxy/pin validation still applies. The Tailwind executable is
validated but not invoked in this mode: Gallery serves CSS embedded in the exact candidate.

The harness retains Git-visible spike source bytes and its executed binary before any
build. Its exact directory allowlist includes the private bundle launcher/preparer and
`internal/macbundle`; unknown directories still fail before building. Retaining those
sources does not mean the server/native build commands compile or test the packaging
commands. Root-level evidence helpers are retained, not executed by this rehearsal.
Only the known development app-kit replacement is removed in fresh staged files;
the source checkout and its module files remain unchanged. Each run starts with empty
Go caches, resolves the exact private kit artifact, and invokes the spike's explicit
`prepare-wails -appkit-version VERSION` mode to verify/copy/patch the existing Wails pin.
It prefetches graph metadata with `go mod graph`, not an all-module archive download.
The original prepared stage stays intact; a directory-hash-checked copy and separate
native module file use a stable relative Wails path so build metadata can reproduce.
Builds deny network and both fleet source roots. Selected existing POST/SSE/shutdown
tests run under a separate loopback-only policy; the server dependency test still refuses
Wails/cgo imports. No native window or default browser is opened.

`host-receipt.json` records source inventory, command output files/hashes, selected
module, prepared module identity, failures, and server/native binary comparisons.
Every retry needs a new output directory. Passing requires two identical server and
native binaries and an unchanged source snapshot, not merely successful compilation.
The native linker explicitly requests macOS14.0 and its Mach-O `LC_BUILD_VERSION`
must agree; environment variables alone do not establish that binary contract.
This records a build minimum, not runtime acceptance on macOS14.
This is private experimental delivery evidence, not a signed bundle, installer, updater,
native input/media acceptance, browser visual review, or real-consumer adoption.
See the [two-cold-root result](../../docs/assessments/2026-09-15-independent-gallery-host.md)
for actual measured scope, artifact identities and retained failed attempts.
The [current-source refresh](../../docs/assessments/2026-09-16-current-gallery-candidate.md)
records later source selection, a distinct candidate and remaining execution gates.

### Exact-output browser tasks

The opt-in [browser harness](browser/run.mjs) uses an already-installed Playwright
and Chrome on macOS. It installs nothing, changes no default browser, and never uses
a signed-in profile. Supply an independently reviewed host receipt hash and a fresh
private output directory outside both fleet source roots:

```sh
node --test tools/releasecandidate/browser/*_test.mjs
node tools/releasecandidate/browser/run.mjs \
  --receipt /absolute/host-rehearsal/host-receipt.json \
  --receipt-sha256 EXACT_SHA256 \
  --playwright /absolute/existing/node_modules/playwright/index.mjs \
  --out /fresh/private/browser-rehearsal
```

Before execution it retains the selected server, parent receipt and complete first-party
harness/test closure. Both parent server artifacts must match. It runs those retained
bytes under source-denied, localhost-only server policy with actual refusal canaries.
The existing host's `-browser` path invokes a private PATH opener, which atomically
hands off the one-use launch URL; this does not verify LaunchServices. Browser application
requests are separately allowlisted; the Chrome process is not network-sandboxed.

Four fresh synthetic sessions exercise desktop/light and narrow/dark workflows with
and without page JavaScript. Tasks include keyboard saves, invalid input, a second
window's conflicting save, explicit review, failure/retry, Settings persistence, and
SIGINT shutdown. Script-enabled sessions additionally exercise opt-in snapshots,
cross-window updates, stop/reconnect and stale-state disclosure after shutdown.
Form navigation is armed before submission. Every served CSS response must match
the candidate. Context teardown, browser closure, profile cleanup and handoff residue
are success gates; cleanup uncertainty stays a failure.

Receipts retain fixed failure categories, hashes and full-document/task-viewport PNGs,
not raw capability URLs. Chrome can temporarily persist a capability in its private
profile. Failed artifacts remain private; retries never overwrite earlier evidence.
Geometry checks do not certify screenshots: inspect the task-viewport images, since
fixed chrome can cover content in full-document captures. This is synthetic browser
acceptance, not native input/media, OS-wide browser isolation, performance, full
accessibility, real-consumer adoption or release certification.

## Migration review for an authorized consumer upgrade

A prepared private candidate, reviewed migration notes and a released pin are distinct.
Before repeating preparation or a rehearsal, inspect the existing receipt and compare
the exact selected source and required task coverage. Lack of a tag alone does not
mean preparation is missing. Later source changes never inherit an older receipt's
results; build or exercise the changed boundary when new evidence is actually needed.

The artifact captures the selected working tree, not only this preparer's new files.
Review its inventory and the complete [Unreleased changes](../../CHANGELOG.md) first.
Select an exact candidate/release, remove development replacements for the isolated
build, and review the resulting module graph/sums plus portable `kit.lock` changes.
Regenerate selected tokens/CSS with explicit module-resolved scanner inputs. Recheck
font/radius controls, stat/nav layout and the analytics renderer's table-only fallback.
Account for the existing breaking notes: custom `appkit gen -tokens` inputs require
explicit `-terminal` output when needed; MCP theming returns review guidance, not
automatic palette replacement. Preserve app identity, auth and business semantics.
For selected analytics exports, migrate shared eight-column readers to ten, Gallery31
to33 and Receipts33 to35 under ADR-0056; retain exact-header refusal and row meaning.
Review catalog search/cursor changes and selected metadata: v1 still binds the whole
catalog; opt-in v2 needs a rebuilt checker and explicit selected-record review under
ADR-0065. Missing historical bytes mean prospective selection, not a verified delta.
For private evidence readers, review ADR-0061's32/64 capture boundary without rewriting
old packet identities. For CLI consumers, replay help/version/invalid-input diagnostics
and exact exit policy; default-off origin experiments are not automatically adopted.
Refresh the consumer's reviewed adoption contract and applicable agent/MCP document root;
rebuild affected processes before verifying exact guidance bytes. Run the consumer's
own task, accessibility, export and recovery tests before describing it as upgraded.
No consumer upgrade is performed by either preparation or a synthetic rehearsal.
