# Kustomize native calibration

This lane measures the unchanged Kustomize document dialect against Kustomize
v5.8.1's own typed API and CLI. The fixed five-subject population is in
`pins.json`. The original scopes remain `kubernetes-sigs/kustomize`'s
`examples/` and `argoproj/argocd-example-apps`'s root. The three expanded
subjects exercise Flux Helm, Flux multi-tenancy, and GCP microservices.
Subject commits and membership were frozen before this canonical run.
Earlier private exploratory probes were used to select expanded subjects;
they are not blind holdout evidence.

Run these commands from the repository root. Set `KUST_CACHE` to a caller-owned
directory under `workspace/` or an allowed external cache; raw truth, logs,
files and unmatched diagnostics stay there. The commands deliberately do not
change the product engine. The baseline `indexbench` binary
must identify itself as a clean build from
`6db920e72898abb20bf86c22652b401902e34598` through `go version -m`.

```sh
KUST_CACHE="${KUST_CACHE:-$PWD/workspace/tmp/kustomize-cache}"
BASE_CHECKOUT="$PWD/workspace/tmp/kustomize-base-checkout"
mkdir -p "$KUST_CACHE"/go-build "$KUST_CACHE"/go-tmp "$KUST_CACHE"/go-mod "$KUST_CACHE"/tmp "$KUST_CACHE"/tools
export GOCACHE="$KUST_CACHE/go-build" GOTMPDIR="$KUST_CACHE/go-tmp" GOMODCACHE="$KUST_CACHE/go-mod" TMPDIR="$KUST_CACHE/tmp"
export GOWORK=off GOMAXPROCS=2 GOFLAGS=-p=2 GOTOOLCHAIN=go1.27.3 PYTHONDONTWRITEBYTECODE=1
git worktree add --detach "$BASE_CHECKOUT" 6db920e72898abb20bf86c22652b401902e34598
nice -n 10 go -C "$BASE_CHECKOUT" build -o "$KUST_CACHE/tools/indexbench-kustomize-base" ./cmd/indexbench
git -C "$BASE_CHECKOUT" rev-parse HEAD
git -C "$BASE_CHECKOUT" status --porcelain --untracked-files=all
go version -m "$KUST_CACHE/tools/indexbench-kustomize-base"
cd calibration/oracles/kustomize
nice -n 10 go mod download
nice -n 10 go build -trimpath -buildvcs=false -o "$KUST_CACHE/tools/kustomize-oracle" .
cd -
python3 calibration/oracles/kustomize/manifests.py --cache "$KUST_CACHE" --phase before
nice -n 10 python3 calibration/oracles/kustomize/run_native.py --cache "$KUST_CACHE" --oracle-bin "$KUST_CACHE/tools/kustomize-oracle" --phase before
nice -n 10 python3 calibration/oracles/kustomize/grade.py --cache "$KUST_CACHE" --indexbench "$KUST_CACHE/tools/indexbench-kustomize-base" --checkout "$BASE_CHECKOUT"
python3 calibration/oracles/kustomize/manifests.py --cache "$KUST_CACHE" --phase after
nice -n 10 python3 calibration/oracles/kustomize/run_native.py --cache "$KUST_CACHE" --oracle-bin "$KUST_CACHE/tools/kustomize-oracle" --phase after
python3 calibration/oracles/kustomize/publish.py --cache "$KUST_CACHE" --out docs/assessments/data/2026-10-26-kustomize-r1.json
python3 calibration/oracles/kustomize/publish_reading.py --cache "$KUST_CACHE" --public docs/assessments/data/2026-10-26-kustomize-r1.json --reading docs/bench/readings/lang/kustomize.json
```

The `rev-parse` command must print the fixed full commit; `status` must be
empty. `go version -m` must report that same `vcs.revision` and
`vcs.modified=false`. The public `indexbench_local_artifact_sha256` records the
binary used in this run, not a portable source identity: this build omits
`-trimpath`, so the checkout's absolute path can change its hash. A rebuild
must match the `go version -m` revision and clean state; its binary hash need
not match the recorded local artifact. For a fresh cache, run `prepare.py --cache "$KUST_CACHE"
--step tool --step subjects` before the shown sequence. Use one heavy job at a
time and niceness 10. Both
drivers wait and recheck while one-minute load exceeds 9 before each subject.
`prepare.py` is the only subject/tool acquisition step: it fetches fixed
subject commits and the pinned native archive and verifies its SHA256. The
same official v5.8.1 `checksums.txt` pins darwin_arm64
(`8886f8a7...`), linux_amd64 (`029a7f0f...`) and linux_arm64
(`0953ea3e...`) archives in `pins.json`; `prepare.py` selects by host.
The oracle recipe uses `-trimpath -buildvcs=false`, and the receipt records
the resulting oracle binary hash. The
native oracle runs with scrubbed environment and dead proxies; it does not
contact cloud services, enable plugins, or invoke Helm.

The oracle enumerates every filename recognized by native Kustomize, then
decodes every candidate through `types.Kustomization.Unmarshal` and obtains
positions from native kyaml. Each typed entry is either reconciled with its
build result or disclosed in the private receipt, including failed builds,
unsupported fields, no-ops and unsupported filenames. For Components, a
build of the Component alone cannot establish behavior. The oracle evaluates
every actual non-Component parent closure that reaches it and discloses context
counts. This is the P-T1 full-population rule applied to native evaluation.

`run_native.py` records raw JSON, a built-file list, receipt and log for each
subject and phase. Before and after raw JSON, list and receipt must match byte
for byte; tracked-set manifests must also agree. `grade.py` grades every
supported built file in each of three complete pooled runs. It checks each
subject's exact measured file set and bytes, keeps unsupported native
`Kustomization` basename files in a separate count, and asserts zero faults.
Unmatched files remain private. It keeps the median CPU throughput run only
after counts, metrics and grades match across all three. The original subject
SHA and cross-reference floors (12 and 1), and pooled floor (13), are conserved
as explicit gates; no outcome-based file removal is permitted.

Run the driver identity tests and native oracle tests before measurement:

```sh
python3 -m unittest calibration/oracles/kustomize/test_grade.py
mkdir -p workspace/logs
nice -n 10 go test -race ./internal/fusion -run '^TestKustomizeBaseDirectoryNeedsTheOneFileRule$' -count=1 > workspace/logs/kustomize-r1-directory-preference.log 2>&1
KUSTOMIZE_BIN="$KUST_CACHE/tools/kustomize-5.8.1" nice -n 10 go -C calibration/oracles/kustomize test -race -count=1 ./...
```

The CLI tests check native-invalid configuration accounting, a changed source
fact, real Component content and whole-unit admission refusals with no partial
truth or receipt. Binary identity accepts only the exact clean source revision.

The product-side independence control runs the actual document extractor, then
removes only its resource selectors in memory. It requires changed extractor
output and unchanged pinned source bytes. The native after phase must still
match before exactly. From the repository root:

```sh
mkdir -p workspace/tmp/kustomize-proof
cat calibration/oracles/kustomize/product_ablation.go.txt > workspace/tmp/kustomize-proof/product_ablation.go
nice -n 10 go run ./workspace/tmp/kustomize-proof/product_ablation.go -subject-file "$KUST_CACHE/git-subjects/argoproj--argocd-example-apps--8088f4c0d970/kustomize-guestbook/kustomization.yaml" > "$KUST_CACHE/kustomize-r1/product-control.json"
```

`publish.py` writes only org/repository IDs, fixed commit IDs, counts, metrics
and hashes. It excludes subject source, file lists, raw oracle rows, log lines,
unmatched details and any personal namespace. The public record states each
L3 cross-reference metric's own denominator and whether D19's minimum 20
holds. A thin result is published with `D19_xref_thin=true` and a held reading
note; publication does not select on the grade or denominator.
`python3 calibration/oracles/kustomize/publish.py --self-test` runs synthetic
fixtures for that disclosure. Native build
truth is limited to the fixed Kustomize version, exact selected commits and
successfully built contexts; external resources, Helm rendering and controller
execution remain outside this oracle.

After the public receipt passes, `publish_reading.py` checks its unannotated
median SHA256, all five pins and every pooled and per-subject measured value.
It preserves all measured reading fields and the existing `required_kinds`,
merging the median's scope notes with receipt-derived counts, provenance and
limits. Add `--check`
to validate and print its receipt without writing the reading. The directory
preference test log must be green; the writer does not infer that product
behavior from the native oracle.
