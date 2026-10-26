# nginx: native L2 population measured, L3 unchanged

The r3 diagnostic scores 71/71 definitions at C and 1/1 references at A,
zero faults, L1 good/L2 excellent. Native nginx 1.31.6 enumerates upstream
object identities over all 47 original disclosed inputs before comparison;
there is no scanner-selected reference denominator. The one local pair passes
target-only declaration ablation. The canonical L3 reading is unchanged.

Native source bodies/returns are preserved by the HTTP/stream observer.
The build, original/instrumented source and shim hashes are recorded in
`calibration/oracles/nginx/native-observer-build-r1.json`. All 42 authored tests
passed with every native test enabled. Stream's target-only mutation produces
`no port in upstream`, unlike HTTP's host-not-found diagnostic; both require
the original target/location and a successful unchanged profile.

Scope is the union of all historical disclosures. The actual harness scores 35
files; 12 native-valid inputs and 12 raw independent definitions are excluded
by unchanged ambiguous-content ingest gates. None has a local reference, so
all native reference truth remains scored. Native raw definitions total 83;
scored definitions stay 71. This is disclosed, not presented as perfect file
coverage. No exclusion or golden is weakened. The earlier narrower-scope
attempt reproduced 61 definitions and was refused by the unchanged count gate.

`native-population-r1.json` records per-subject counts and normative tracked-set
receipts. The exact recipe from design section 13.3.1 was run before/after;
all hashes agree and generated/untracked, symlink and gitlink counts are zero.
Tracked-set counts exceed the reading because they include other formats and
files outside the historical scoped inputs. Three warm timings are 9.58, 9.64,
9.86 MB/CPU-s (median 9.64). One reference and about 34 KB are thin evidence,
not broad operational coverage. Measured clones and native source builds have
been removed; raw responses and manifests remain private scratch.

Independence: run the committed control driver:

```sh
python3 -B calibration/oracles/nginx/probe_controls.py -nginx workspace/oracle/nginx-observer -build-receipt calibration/oracles/nginx/native-observer-build-r1.json -out workspace/artifacts/lang-nginx/native-controls
```

The helper executes the
actual fathom extractor with `go run -race`; removing its 5 nodes preserves
raw native truth, and changing the input binding changes native truth.

Mutation: `native_population.py`, filter native refs by the scanner's refs;
the focused `test_native_pair_survives_an_independent_scanner_omission` reports
FAIL, then restored green. The command above reproduces both runs; its
`mutation.log` and `native-controls-r1.json` retain the result/digests.

Caps: `test_population.py` tests whole-profile refusal at 10,001 events and
8,388,609 stderr bytes, each cap + 1. The same committed command reproduces
these named controls and writes `caps.log`.

The native L2 availability trigger has fired. Native definition positions
remain open work, not an oracle-absence ceiling. Canonical integration and
L3 work await D19/controller gating. No product or architecture decision,
extractor epoch, generated tracker or plan checkbox changes. No model decision
applies.

After merging Wave B, all 42 authored tests pass with native controls enabled
and zero skips. The release-notes, merge-debris, ADR-011 and strict reading
round-trip checks pass. The X-R5 scan of branch-added evidence found no subject
people, emails or attribution lists; receipts contain counts and hashes.
