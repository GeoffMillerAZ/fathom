# COV-M3 · Go initial dependency admission

Status: preregistered, provisional. COV-4 remains unchecked until an independent
reading review. No admission or reader change is authorized by this record.

## Preregistration

COV-D10 and ADR-114 govern this first admission. The immutable population is
[the Go pin manifest](../../../../calibration/oracles/dep-coverage/cov_go_pins.json).
In corpus-list order: Cobra `40b5bc1437a564fc795d388b23835e84f54cd1d1`
(36 files), GitHub CLI `45ffa3c668f7105e92089220d42aa64c9acbd11c`
(737), Prometheus `9ec59baffb547e24f1468a53eb82901e58feabd8` (618).
Total 1,391. Select every tracked regular case-sensitive `.go` file, including
tests and build-excluded files. Exclude symlinks and gitlinks from selection,
retain their metadata in input receipts, and resolve against the full tracked
repository. Selection SHA-256 hashes sorted UTF-8 paths, each followed by NUL.
The manifest freezes each selection digest. Foreman path audit: zero rejections.
Any later rejected input stays in all denominators, incomplete, with the same
rejected set required in B3 (ESCALATE-19).

Native oracle: Go `go1.27.4`, executable SHA-256
`05f88ad8cc1e19f455104bd936e255e9ac9acf7b7725df96fb59e7260b425252`.
Run `go list -e -deps -test -json ./...` in every tracked module root for
membership, resolution and build exclusions. Run named-file `go list -e -json`
for file-specific import and embed evidence. Package import unions alone never
supply a file's imports. Both contexts are `GOOS=darwin GOARCH=arm64`, first
`CGO_ENABLED=0`, then `CGO_ENABLED=1`. No custom tags. Tests remain selected;
native ordinary, internal-test and external-test membership are kept separate.
Ignored or unlisted files have no native truth, rather than empty truth.
The cgo context is supplementary: cgo files need explicit native C input
attribution or remain gaps. No `-compiled`, build, generate or test execution
on subject code. Native synthetic test-package executables are never run.

Native environment: `GOENV=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local
GOPROXY=off GOSUMDB=off GOWORK=off`, scrub inherited Go build settings.
Use the existing module cache read-only in intent, record unavailable module
or generated inputs as gaps, never install, fetch or use the network. Check
tracked manifests and untracked/ignored inputs before and after. Preserve raw
native JSON and stderr privately, including errors from `-e`; normalize checkout
paths only in adapted evidence. Hash raw and normalized captures separately.

Grade the unchanged built-in reader with explicit `TrustUnmeasured("go")`,
which exposes candidate completeness for measurement only. Establish fixture
mutation RED for new coverage/scoring behavior before implementing it. Gather
native truth before reader observations. B1: zero missed native local direct
edges and zero inventory-refuted external claims on complete files. B2: zero
wrong bindings and zero inconclusive bindings on complete files. A complete file
without unique native truth makes both inconclusive. B3: identical reader JSON
for repeat (reversed input order) and a physically separate checkout, including
rejections. B4: per-subject and pooled complete share, unknown evidence shares
by code, extra edges over comparable local edges, and every coverage denominator.
Use tracked module/package names for the COV-D6 inventory. Config and sibling
inputs require native-supported attribution; ambiguous embed patterns remain
gaps. Preserve separate context coverage and disagreements. Apply the primary
context's score, with no unexplained supplementary disagreement on a complete
file. B5: any unresolved B1/B2 failure prevents admission. Below 0.80 complete
is sound, low coverage only if all soundness bars pass. A smaller or ungraded
population is inconclusive. No substitution or threshold change after a run.

At most two correction rounds exist for first admission, but this lane performs
none: report any needed correction or admission flag for a subsequent stage.
Test-only helpers must remain unreachable from product builds. The full gate,
independent reading review and final admission disposition belong to the foreman.
