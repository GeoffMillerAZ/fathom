# Private macOS bundle preparation

Experimental implementation of [ADR-0068](../../docs/adr/0068-prepare-private-unsigned-macos-delivery-first.md).
This prepares and checks private artifacts. It does not establish Finder/Gatekeeper
acceptance, native workflow, rollback, signing, installation or a supported release.
The [delivery matrix](../../docs/design/web-desktop-parity.md#private-unsigned-macos-delivery)
remains the acceptance contract. Windows is deferred.

## Fixed topology

```text
Gallery.app/Contents/
  Info.plist
  MacOS/gallery-launcher
  Resources/bundle.json
  Resources/gallery-server
  Resources/gallery-native
```

The native-free launcher resolves its own executable location, checks the fixed
layout, canonical metadata, file modes/sizes and exact SHA-256 values, then replaces
itself with the HTTP-owning server. Only the paired `-native-host` path and its
`-native-host-sha256` are passed. No user-supplied flags, direct-native fallback or
extra supervisor process is introduced. The existing parent stages its native
companion, owns signals/HTTP state and applies ADR-0047's recovery policy.

The old experimental `Info.plist` names `gallery-host` and does not select this
topology. Preparation generates a separate fixed plist naming `gallery-launcher`.
The existing direct-native, standalone browser and parent commands are unchanged.

## Preparation and read-only check

Run from this isolated module. Build the helper commands without native dependencies:

```sh
CGO_ENABLED=0 go build -mod=readonly -trimpath -buildvcs=false -o /ABS/PRIVATE/prepare-bundle ./cmd/prepare-bundle
CGO_ENABLED=0 go build -mod=readonly -trimpath -buildvcs=false -o /ABS/PRIVATE/gallery-launcher ./cmd/bundle-launcher
```

Select reviewed server/native builds and their exact hashes from retained build
evidence. Do not rebuild them implicitly or use a mutable sibling checkout as a
release identity. These flags are all mandatory; placeholders are not runnable inputs:

```sh
/ABS/PRIVATE/prepare-bundle \
  -out /ABS/PRIVATE/Gallery.app \
  -launcher /ABS/PRIVATE/gallery-launcher -launcher-sha256 LAUNCHER_SHA256 \
  -server /ABS/PRIVATE/gallery-server -server-sha256 SERVER_SHA256 \
  -native /ABS/PRIVATE/gallery-native -native-sha256 NATIVE_SHA256 \
  -kit-revision FULL_40_OR_64_HEX_REVISION -min-macos REVIEWED_MAJOR.MINOR
/ABS/PRIVATE/Gallery.app/Contents/MacOS/gallery-launcher -check
```

`-check` validates and prints a plan without executing the server or native companion.
Preparation never launches, signs, installs, downloads, updates or removes anything.
The destination must be a new absolute `.app` path under an existing real directory.
Resolve macOS path aliases explicitly; symlinked source/output paths are refused.
Never put a trial in `/Applications` or overwrite a prior/running bundle.

Inputs require regular executable files (at most256MiB), exact expected hashes and
no group/world write access. Bundle directories, their immediate staging parent and
metadata also refuse group/world write. Quarantine on executable leaves, ancestors or
the bundle is refused; nothing strips attributes. Other operating systems fail closed
because this helper cannot inspect macOS quarantine there. Assembly hashes and copies from the same opened source descriptor into
exclusive private outputs, rechecking copied bytes. A failure after creation leaves
the partial output for inspection, never a successful receipt. Choose a fresh path
for another attempt; do not turn a refusal into an automatic overwrite or trust bypass.

Manifest/plist identity and minimum OS are declarations, not proof of linked Mach-O
targets, executable roles or publisher trust. They must be reconciled with actual
build receipts and runtime tests before use. No defense against a hostile process
running as the same user, or mutation between final verification and `exec`, is claimed.
The bundle must remain immutable while checked/running; manual replacement is a
separate authorized rehearsal, not an updater implemented by these tools.

## Local verification

```sh
go test -mod=readonly -race ./internal/macbundle ./cmd/prepare-bundle ./cmd/bundle-launcher -count=1
go vet -mod=readonly ./internal/macbundle ./cmd/prepare-bundle ./cmd/bundle-launcher
node --test bundle_check_probe_test.mjs
```

Tests use synthetic files and injected execution; no real native app is started.
They cover deterministic assembly, relocation, metadata/byte/path refusals, output
collisions, argument boundaries, check-only behavior and native-free launcher imports.
As elsewhere in this spike, private helper flags use the existing stdlib preparation
style; this is not a new fleet CLI or a change to the Charm CLI-kit contract.

For the compiled-command boundary, use already-built local helper binaries and a
new, existing empty0700 directory. The maintained synthetic rehearsal retains its
exact harness/configuration and checks helper/ancestor quarantine before copying or
executing helpers. It creates inert server/native fixtures and invokes the launcher
only with `-check`, retaining deterministic pairs, relocation and all refusal copies:

```sh
node bundle_check_probe.mjs /ABS/PRIVATE/prepare-bundle /ABS/PRIVATE/gallery-launcher /ABS/PRIVATE/EMPTY_OUTPUT
```

Its synthetic revision/minimum OS are labeled declarations, not build provenance.
The [2026-09-16 assessment](../../docs/assessments/2026-09-16-private-bundle-seam.md)
separately records assembly/check-only evidence using the actual retained Gallery
server/native pair. Those bytes predate later working-tree UI changes.

An authorized direct-executable lifecycle rehearsal uses the existing
[`native_delivery_probe.mjs`](NATIVE-DELIVERY-PROBE.md) flags and adds the all-or-none
bundle launcher, launcher SHA-256, manifest SHA-256 and plist SHA-256 quartet. Its
server/native paths and pins must name the fixed resources in that same bundle. The
probe runs `gallery-launcher -check`, freezes the checked identities, then invokes
that in-place launcher with no arguments and applies the unchanged AX lifecycle
oracle. It never extracts or rebuilds the bundle executables.

Before a real bundle launch, retain exact sources, binaries, commands and host details,
then obtain the applicable execution authority. Run the existing shared workflow and
failure/rollback matrix against the exact staged bytes. Static checks do not resolve
the separately open native input/media failures or prove browser recovery through
the OS-selected opener. Direct executable launch is not Finder/LaunchServices launch,
shared DOM-task evidence or manual rollback. A launched process alone is not acceptance.

Before normal-opening acceptance, retain strict `codesign --verify --strict --verbose=4`
output independently of launch results. The [normal-opening controls](../../docs/assessments/2026-09-16-private-bundle-runtime.md#normal-launchservices-controls)
retain an instrumented refusal, minimal-request startup, failed signature verification
and a harness path-alias failure with subsequent owned cleanup. Do not reuse that frozen
one-off normal-launch harness before repairing its alias, listener and cleanup checks.
The separate [maintained probe](LAUNCHSERVICES-PROBE.md) now has12 synthetic regressions
and exact-source normal-opening AX lifecycle evidence; it does not promote the old
one-off runs, signature verification, shared tasks or rollback into passes.
