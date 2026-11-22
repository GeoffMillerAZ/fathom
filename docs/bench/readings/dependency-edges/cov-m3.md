# COV-M3 · Go initial dependency admission

**Independent admission result.** Go remains unmeasured. The 1,391-file
pool contains 1,000 files with native truth and zero candidate-complete
files. B1 and B2 are inconclusive in the empty complete slice. B3 passed
in both build contexts.

Preregistration commit: `cea1adaeb`. Measurement harness:
`d9d7faf2b9943a798bf9e537b190f95b65acbbc4`. Product reader bytes are unchanged
from `7f70e23d7feb3bd272fdf80cc1300035c48fb93c`. No reader correction or
admission-flag change was made.

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

## Reading and denominators

Both preregistered contexts produced identical adapted truth and gap sets.
All 1,000 files covered by both contexts agree. The tables below apply to
each context separately. The pool contains three repositories and 1,391 files;
no selected file was removed. Harness-only rejected inputs: zero.

| Subject | Selected | Native covered | Native gaps | Candidate complete | Complete with / without truth |
|---|---:|---:|---:|---:|---:|
| Cobra | 36 | 35 | 1 | 0 | 0 / 0 |
| GitHub CLI | 737 | 727 | 10 | 0 | 0 / 0 |
| Prometheus | 618 | 238 | 380 | 0 | 0 / 0 |
| **Pooled** | **1391** | **1000** | **391** | **0** | **0 / 0** |

B1 and B2 are **inconclusive**, with zero complete files and zero expected
local edges in that graded population. Missing edges, wrong targets, inconclusive
bindings, complete-file extra edges and COV-D6 external-as-local claims are all
zero per subject and pooled. These zero counters are not soundness passes.
The scorer deliberately refuses vacuous admission. B3 passes per subject and
pooled, including the empty rejected set. B5 therefore permits no admission.
No complete file lacks truth, because no file is complete.

B4 complete share is **0 / 1,391 = 0%**, and 0% for every subject.
Unknown share uses all evidence (edges plus unknowns). The extra-edge rate uses
local observed edges on native-covered files, including incomplete files.

| Subject | Edges | Unknowns | Evidence | Unknown share | Comparable local edges | Extra | Extra rate |
|---|---:|---:|---:|---:|---:|---:|---:|
| Cobra | 594 | 756 | 1350 | 56.00% | 402 | 0 | 0.00% |
| GitHub CLI | 10391 | 13879 | 24270 | 57.19% | 6376 | 0 | 0.00% |
| Prometheus | 10275 | 15761 | 26036 | 60.54% | 2179 | 0 | 0.00% |
| **Pooled** | 21260 | 30396 | 51656 | 58.84% | 8957 | 0 | 0.00% |

| Unknown code | Cobra | GitHub CLI | Prometheus | Pooled | Share of pooled evidence |
|---|---:|---:|---:|---:|---:|
| `external-unpinned` | 178 | 3932 | 4196 | 8306 | 16.08% |
| `unbounded-glob` | 0 | 0 | 1 | 1 | 0.00% |
| `unresolved-alias` | 1 | 11 | 18 | 30 | 0.06% |
| `unsupported-construct` | 577 | 9936 | 11546 | 22059 | 42.70% |

## Native coverage limits

| Gap | Cobra | GitHub CLI | Prometheus | Pooled |
|---|---:|---:|---:|---:|
| `native-build-excluded` | 1 | 8 | 40 | 49 |
| `native-embed-attribution` | 0 | 1 | 0 | 1 |
| `native-import-unavailable` | 0 | 0 | 326 | 326 |
| `native-unlisted` | 0 | 1 | 14 | 15 |

The native evidence covers 71.89% of the selection. Unlisted and
build-excluded files remain in the denominator, rather than receiving empty
truth. Unavailable direct-import evidence accounts for 326 files. Missing
module or generated inputs were preserved as gaps, with raw native diagnostics,
and no dependency installation or network remedy was attempted. One named-file
embed listing has multiple patterns without defensible per-pattern file
attribution. Its package union is not promoted to file truth. Cgo/native
companions require separate attribution; fixtures exercise that refusal.
No result claims another OS, architecture, tag set or installed dependency state.

## Findings and disposition

- **COV-M3-F1.** Every selected file has an `unsupported-construct` unknown
  attributed to a tracked `go.mod`. The reading measures 30,396 unknowns,
  including 22,059 unsupported constructs. Configuration uncertainty blocks
  all candidate completeness, so this first reading cannot establish B1/B2.
- **COV-M3-F2.** The native oracle leaves 391 selected files uncovered.
  This lane cannot grade the full population under the preregistered offline
  constraints. A future candidate-complete file in that set would make B1/B2
  inconclusive until native evidence covers it.

**Correction rounds.** Zero reader rounds were used. No complete file
exposed a wrong or missed binding. The existing conservative go.mod
disclosure blocks admission in this population. The Go reading supports
no change to explicit trust or to the reader's admission flag.

## Receipts

Each primary and relocated checkout had matching before/after tracked manifests
and clean tracked, untracked and ignored status. The full tracked repository
supplied resolution inputs. There were no tracked symlinks or gitlinks.

| Subject | Regular tracked files | Input manifest SHA-256 |
|---|---:|---|
| Cobra | 66 | `200b657265beec4fbc3b288b452d20010d918ba2c5159aba3df0430355b46a92` |
| GitHub CLI | 979 | `9872138945a62464124d40eb3dc6b1d7463d8e82a0149c3bea71ac340b6f51ea` |
| Prometheus | 1542 | `614776300e5f81ada1ca0034437994bbfac76126fe44fa0d51ced462176d4eab` |

The adapted digest includes truth and gap classifications. Raw native
captures include physical checkout paths and stay private; their hashes are
run receipts, not a promise of equality across machines.

| Subject | Adapted truth/gaps SHA-256, both contexts | Raw cgo=0 SHA-256 | Raw cgo=1 SHA-256 |
|---|---|---|---|
| Cobra | `b5cdfedcbacf0cac54da507942178845f3233a41b1c48f158e4895d6ce66a679` | `37c7f061699740edfee663444b85bf667575bfa67546a4783fd744dac78267ea` | `c6c3ffd1c5df9f2cdc6123dccf257c9c2bfc5d629899f232ff43b6c894fd7a50` |
| GitHub CLI | `b240520d09a279ef9b6d2059682828b62e8ee576df82e5787ac1b24a564fc110` | `c856954141f09fc823995d68c017d27c90a1f95c63562a0f7ca6ff26fdc6ef9f` | `08be13e4fe150f24888807f84e54c85a90924b155603c5a2209555b9e6873992` |
| Prometheus | `3325e24d2f410f9d96fa30df03ac01462e2f266cba369eaf71605e7902c315d8` | `c5e52b96ce506a610fbdcd80fce95435688e232fa9873a8f808033c5d6359a0e` | `2df239b6a636b4f4c6724275143b54a0e56fd991e454a4369cd40eabd1c5f97c` |

Primary, reversed-input repeat and physical-relocation reader JSON are
byte-identical. The common SHA-256 per subject is:

| Subject | Reader JSON SHA-256 |
|---|---|
| Cobra | `5289c092a68e869dbe1979698b4c76fd2f90999bd5268b0b6139b7caa462ccaf` |
| GitHub CLI | `be595ee0de740910a41158faae731076758a037499f591dd242f4c12dd4e6da8` |
| Prometheus | `003462b3124f392d99a39346f3de21c638906e3340c85312c94fea019ef233d2` |

The independently reproduced aggregate matches byte for byte after removing
only the six run-specific `Subjects[].Receipt.captures[].raw_sha256` values
and encoding with sorted keys, compact separators, and a trailing newline.
The normalized aggregate SHA-256 is
`b086210afab74c97312ce129f3bfeb1b4cd98beeb9bf1054df678a64e340c18c`.
Every field used in the verdict and all 1,391 reader rows agree.

Original run aggregate receipt SHA-256:
`20a8dcae1f9f440cf5f4acbe8a1f12b036ac807e54080d69360559360d02ff96`.

## Proof and reproduction

The native adapter is test-only and independent of product output. Fixture
controls cover ordinary/internal-test/external-test membership, nested modules,
build exclusions, unavailable imports, cgo companions and embeds. The initial
coverage stub was RED. Replacing named-file imports with the package import
union made the attribution control fail. Disabling Go module-prefix inventory
checks made `TestCOVGoModulePrefix` fail. Both mutants were restored before
grading. `TestCOVGoNativeScoring` drops actual product imports (B1 refuses),
checks raw native bytes remain unchanged, then changes source and observes
changed raw truth. Existing `TestGoDependencyOracle/B2` rejects a wrong binding.
The Go inventory also checks nested-module shadowing and component boundaries.

`go list -deps ./pkg/lang/deps` excludes `depbench`, and the product-reader diff
against the pinned base is empty. The complete package suite passed under
`-race` with native fixture controls enabled. Native capture passed in 197.11 s;
reader measurement passed in 164.28 s. A setup attempt failed before invoking
any corpus oracle because a shared path helper required an existing output
directory. The capture entry point now accepts a fresh directory. That attempt
is retained privately and is not a reader correction round.

Source the lane environment and use the machine package-test lease when working
in a shared lane. `$lease` below is that lease wrapper; `$cov_sources` contains
local Git sources for the pinned subjects. `$cov_run` must be a fresh scratch
path outside Git and module roots. No preparation step fetches dependencies.

```sh
source "$lane_env"
umask 077
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
python3 -B -m unittest discover -s calibration/oracles/dep-coverage -p cov_go_test.py
FATHOM_COV_GO_CONTROL=1 python3 "$lease" go test -race -count=1 ./pkg/lang/deps
FATHOM_COV_GO_CAPTURE=1 FATHOM_COV_GO_SOURCES="$cov_sources" FATHOM_COV_GO_ROOT="$cov_run" python3 "$lease" go test -race -count=1 -timeout=60m -run '^TestCOVGoCapture$' -v ./pkg/lang/deps
FATHOM_COV_GO_MEASURE=1 FATHOM_COV_GO_ROOT="$cov_run" python3 "$lease" go test -race -count=1 -timeout=30m -run '^TestCOVGoAdmission$' -v ./pkg/lang/deps
```

Capture verifies the pinned native version/digest, commits, selection digests,
input manifests and clean status. Measurement verifies native capture digests
and the selected-file truth/gap partition before constructing the reader.
It refuses a previously measured campaign root. Preserve both failed setup
logs and successful raw artifacts. A fresh reviewer must rerun from a clean
clone and confirm the counts. This lane performed a separate arithmetic replay
of raw observations; it is not that independent review.
