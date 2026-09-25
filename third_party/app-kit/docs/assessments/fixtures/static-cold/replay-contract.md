# Static inventory correction and cold replay — prospective contract

New explicit goal continuation resumes the exact defect documented in app-kit
d75ff9b. The previous packet remains failed and is not repaired or relabeled.
This is one R1 maintainer batch under ADR-0072, not a new general build framework.

Need: remove enumeration-order dependence without accepting different source bytes,
then prove the existing ADR-0071 exporter builds from frozen offline inputs in two
independently empty cache/output roots. No application, dependency, consumer, installed
tool, global configuration, live-data, browser, deployment or release changes.

Inputs reuse the reviewed clean76d3ca6 private candidate and 133-file public dependency
closure under /Volumes/m2ext/appkit-evidence/appkit-static-cold.7qApnn7w. Their original receipts,
all path/size/hash facts, Go module sums and exact proxy inventories must still match.
Retain their original identity; do not claim they were prepared again. The successful
warm collector's post-run snapshot and missing failed-v1 source remain INC-63 limits.
Cold proof stands independently on the fully frozen selected inputs.

Code correction: compare copied inventories canonically by(scope,path), reject
duplicates and retain exact byte/hash/membership checks. Pure tests must accept the
observed golang/glog versus golang-jwt prefix collision, expose the old false mismatch,
and reject changed hash/size, duplicates, extra/missing and wrong-scope records.
Freeze exact helper/tests before running. Freeze final runner/policies/all inputs
before cold execution; admission independently pins the entire manifest hash.

Keep Go1.26.3 darwin/arm64, CGO_ENABLED=0, GOTOOLCHAIN=local, GOWORK=off, GOENV=off,
-mod=readonly, trimpath/buildvcs=false, original go.sum checks and pinned Tailwind4.3.
Each run starts with distinct empty GOCACHE/GOMODCACHE/GOPATH/temp/output directories.
Selected file GOPROXY only; no network fallback. Sandbox denies network, both fleet
roots, original/cache aliases, old preparation and run roots, and the other new run.
Writes stay within its owned run root plus /dev/null. Ordinary OS/toolchain reads
remain allowed: denylist proof, not complete hermeticity or malicious-build security.
Require existing-file source/cache/peer denials plus actual network PermissionError,
not connection refusal. Bound child exit, pipe draining and owned group cleanup;
unknown/remaining processes or forced cleanup after normal exit fail.

Require identical exporter binaries, CSS and all11 site files across the pair.
After initial cold build, require seven exact exporter tests; skips are not passes.
Use the existing independent six-input checker and three detached manifest poison
controls. Historical HTML/CSS equality is informational, never fresh browser evidence.
No consumer/host/CGO-enabled/cross-platform/accessibility/full-quality claim follows.

Budget:30minutes from02:21UTC, two cold runs, each command at most5minutes plus bounded
cleanup. One separately frozen correction only for a demonstrated harness issue;
no unexplained product/dependency/sandbox retry. At failure or budget cap preserve
evidence, update the narrow next action, land useful work, and stop this packet.
