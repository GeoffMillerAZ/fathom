# Normal-opening bundle lifecycle probe

Opt-in local G6 verification under [ADR-0068](../../docs/adr/0068-prepare-private-unsigned-macos-delivery-first.md).
This exercises a reviewed private bundle through macOS LaunchServices, separately
from the [sandboxed direct-executable probe](NATIVE-DELIVERY-PROBE.md).
Implementation and current run results are recorded in the
[runtime assessment](../../docs/assessments/2026-09-16-private-bundle-runtime.md).

## Authority and inputs

Use only an explicitly authorized synthetic Gallery bundle and an existing reviewed
AX driver. This is a real local app launch: the existing native-failure recovery can
open the synthetic loopback Gallery in the default browser. Unlike the direct probe,
this path does not inherit the test sandbox's browser-opener denial. Obtain execution
authority that includes this recovery; do not assume direct-probe approval covers it.

No installation, signing, quarantine removal, OS-policy change, dependency build,
real data or consumer migration is part of this probe. It may focus two test windows;
run native probes serially and do not interact with them during the test.
Only existing Accessibility permission is used, with exact-child AX inspect/New/close
actions. No keyboard events, browser-menu action or permission grant is tested.

Supply the same server/native/driver hashes, private existing output parent, and
all-or-none bundle quartet documented in [NATIVE-DELIVERY-PROBE.md](NATIVE-DELIVERY-PROBE.md#run).
Here the bundle quartet is required: server/native paths must be that bundle's fixed
resources. Checksums identify reviewed bytes; they do not authenticate a publisher.
The bundle and driver must have independently retained source/build provenance.

## Execution contract

The maintained entrypoints are `launchservices_probe.mjs` and its synthetic
`launchservices_probe_test.mjs`. Run the synthetic suite first. Use the same explicit
input flags as the direct bundle probe, replacing its script with
`spikes/wails-gallery/launchservices_probe.mjs`.

Before launching, retain exact harness/helper sources, arguments, input pins and
host identity in exclusive private output. The preflight checks fixed bundle resources,
bounded metadata, private modes and quarantine, and verifies `gallery-launcher -check`.
Recheck selected identities immediately before requesting the launch.

Request only `open -n BUNDLE`: no output redirection, app arguments or environment
override. Discover the new exact HTTP parent against a pre-launch process baseline.
Bind its direct native child by PID ancestry, private file/directory ownership and
modes, and the pinned native digest. Retain both observed and resolved path spellings:
the verified macOS `/var` to `/private/var` alias is allowed, arbitrary symlinks are not.
Discover the parent's single explicit IPv4 loopback listener, not a wildcard listener.

## Acceptance and failure

Apply the unchanged lifecycle oracle: one ready window/New enabled, two/New disabled,
close to one, reopen to two, then close both. Initial readiness must precede any New
action; creating a replacement window cannot rescue missing initial readiness.
The parent and verified child must disappear and the listener must refuse connections
before emergency cleanup. Keep asynchronous intermediate observations, not just final
states. Timeout, fallback or cleanup cannot turn a failed lifecycle into a pass.

LaunchServices owns the parent: successful `open` means only request acceptance;
this harness cannot reap or claim the application's exit status. Process absence and
listener refusal are narrower evidence than the direct probe's observed zero exit.

Cleanup has its own bound and must not depend on successful native discovery. Signal
only identity-bound, launch-owned processes. If the child cannot be bound, retain
unknown native absence explicitly rather than claiming complete cleanup from parent
absence alone. Preserve failed receipts and diagnose the specific refusal.

Record strict signature-verification output separately. A failed resource-envelope
check is not automatically the cause of a LaunchServices failure; the retained
[mixed controls](../../docs/assessments/2026-09-16-private-bundle-runtime.md#normal-launchservices-controls)
show why those claims must remain separate. Never strip attributes or sign a candidate
to make this gate pass without the applicable authorization.

This is not a Finder click test, full keyboard/accessibility or rendered-content proof,
shared DOM-task acceptance, browser recovery validation, manual update/rollback,
Gatekeeper assurance or support on an untested host. The
[full delivery matrix](../../docs/design/web-desktop-parity.md#private-unsigned-macos-delivery)
remains the acceptance contract. Windows/mobile remain deferred.
