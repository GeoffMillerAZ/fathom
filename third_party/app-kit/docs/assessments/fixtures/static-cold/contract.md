# Static exporter cache-cold delivery contract

R1 owner-funded maintenance for core.static-content, following the explicit
continuation of the app-kit uplift. Prior turn landed76d3ca6b908de01d7946f7ef0845cadb6a3d8a48.
Need: current static reading code must build without the active checkout or shared
Go caches, produce identical binary/CSS/site bytes in two independent cold roots,
and retain enough exact inputs for another local replay. This is not a new generator.

Scope: reuse ADR-0045's private candidate/proxy contract and ADR-0071's fixed export.
Keep source/module/CSS pins, content, APIs and all exporter refusal semantics.
No consumer edit, dependency upgrade, network download, installed binary/configuration,
deployment, browser matrix, real data, paid corpus, release or quality promotion.

Inputs must be frozen and reviewed before cold execution:
- Exact clean app-kit76d3ca6 selected by the maintained candidate preparer, with
  inventory/ZIP/Go sums and excluded paths. Do not include data/, spikes/, secrets,
  symlinks or global configuration. Keep the bundle private.
- Selected already-local public module archive/metadata closure, with paths,
  byte counts and SHA256. No live fallback to a user's module-download cache during
  either cold run. Committed go.sum checks remain mandatory; no missing checksums
  are silently accepted. Freeze graph metadata needed for lazy module resolution.
- Go1.26.3 darwin/arm64, fixed GOROOT and executable/tool identities; pure-Go
  CGO_ENABLED=0 build profile, GOTOOLCHAIN=local, GOWORK=off and GOENV=off.
  Go executable SHA256 c3c8820e5edea04144cd3b6cf504aadd050bd2c7e4e63a69a6da17aed1661163.
  No claim about CGO-enabled, another toolchain/platform or independent-host results.
- Copied Tailwind4.3.0 macOS-arm64 executable matching internal/tailwind/manifest.cue:
  56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e.
- Exact executed harness, source bundle, environment, sandbox and all command logs;
  freeze pre-run bytes before any executed revision is changed.

Two roots must start with independently empty build/module caches and independent
work/output/temp paths. Offline filesystem policy denies network, both source fleet
roots, original Go caches, preparation caches and the other run's working/cache/output
directories. Go resolves modules only through the two frozen file proxies; other
ordinary OS/toolchain reads remain allowed by this explicit filesystem denylist.
This is not a complete read allowlist or protection from a malicious build. Limit writes to the owned
run root and harmless device sinks. Demonstrate source/cache/other-root refusals and
network EPERM, not merely connection-refused; retain the exact policy and canaries.
This is filesystem-constrained local proof, not complete OS hermeticity.

Build the exact export command with trimpath and explicit VCS metadata policy;
source identity is the reviewed candidate receipt, not an invented clean-binary stamp.
Build each CSS from its explicit selected sources, then export under /reading/.
Require identical exporter binaries, CSS and all eleven site files, with independently
recomputed sorted names/hashes, required native HTML/content/CSS selectors and
manifest validation. Run the selected export unit/refusal tests after the cold build
so tests do not warm its initial build cache. Record all skips as skips.
Compare current site bytes with prepared-4wDE2G historical artifacts; matching bytes
may reference historical UI evidence, never count as a fresh browser run. Differences
are reported for later review and do not justify an unplanned browser acquisition.

Budget: at most60minutes of this batch, two cold runs, each command capped5minutes,
one separately retained harness repair only for a demonstrated preparation/collector
defect. No retry of an unexplained product/sandbox/dependency failure. Source selection
must stay within the preparer's existing bounds; dependency input copying is capped
512MiB, all new outputs stay under this private external-volume root. Missing inputs
fail with an exact next action; no network fallback or broader cache copy is admitted.

Owner: app-kit maintainer. Evidence root: /Volumes/m2ext/appkit-evidence/appkit-static-cold.7qApnn7w.
If proven, return the smallest maintained replay fixture, documentation and inventory
evidence. Keep standards/gates partial where consumer/host/other quality requirements
remain. At cap or failure preserve artifacts and hand off; no automatic second packet.
