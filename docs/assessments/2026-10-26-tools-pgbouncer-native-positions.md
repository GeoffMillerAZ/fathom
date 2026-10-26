# PgBouncer: native definitions and ranges reach excellent

The unchanged six-file canonical population now measures 161/161 definitions,
161/161 exact ranges and 1/1 references at tier A. Both L1 and target L2 are
excellent, with zero faults and median 43.48 MB/CPU-s (47.07, 17.71, 43.48).
There is one subject, so its numerators, denominators, ratios (all 1) and tier
caps equal the pooled figures. Required kinds and the target remain unchanged.
One binding in 18,826 bytes is the complete thin population, not broad coverage.

The stock PgBouncer 1.26.1 parser enumerates every explicit section/key callback
before comparison. A terminal invalid section proves all earlier tokens were
parsed before listener/backend startup. For callback i at proposed line L,
a before-token mutation must report error L
and exactly native events[:i]; a new invalid line after L must report error
L+1 and exactly events[:i+1]. All 161 before and 161 after prefixes match.
The prior line-only syntax-error check was vacuous and did not support A; its
receipt is superseded. These native prefix brackets tie a particular token
to its source line rather than echoing a locator-chosen position.
Single-line values end on that line; native token order supplies first-header
to last-entry section ranges and last-key duplicate semantics. An independent
locator supplies mutation sites only. Missing/extra callbacks refuse the input.

The first whole-input attempt failed because the native callback opens the
shipped absolute logfile while parsing. Protocol r3 redirects the five existing
logfile values to private scratch, preserving every name and physical line.
No key is removed/inserted. Active syslog refuses the family. Original numeric
and verbose values must still pass the baseline; only afterwards may verbose
be set to 2 for observation. Canonical inputs required no verbose changes.
The authored control proves an invalid original value is never rescued.

`native-positions-summary-r4.json` carries six per-input counts (2, 9, 28, 28,
71, 23), source hashes, native identity and normative tracked-set receipts.
The exact design section 13.3.1 command ran before/after: 440 tracked files,
3,339,789 bytes, equal digest, zero generated/untracked files, symlinks or
gitlinks. Other tracked formats are outside the six-file reading, not silently
included in its denominator. No submodule content is graded. The native L2
reference instrument was rerun on this same build: 52 native sources, with
49 without auth user, two without an external auth database and one binding.
All native/scanner pairs agree only after native enumeration.

Independence: `probe_position_controls.py` executes actual Fathom output under
`go run -race`. Removing its two nodes leaves native truth unchanged; changing
an authored input key changes truth. Mutation: filtering native callbacks by
independent locator sites makes
`test_native_population_cannot_be_selected_by_locator` FAIL; restoration passes.
The committed `native-position-controls-r4.json` preserves hashes and FAIL line.
Caps: `test_all_caps_refuse_whole_input` rejects source 1,048,577 bytes, native
log 8,388,609 bytes and 10,001 callbacks, each cap + 1. Unknown syntax/includes,
identity drift, quoted/unused keys, repeated keys/sections, logging suppression
and invalid original values have authored controls. Native controls run before
any A output, with no skip allowed.

Reproduction commands and native build pins are in
`calibration/oracles/pgbouncer/README.md`. Full native logs, source inventories
and manifests stay private; committed evidence contains counts and hashes.
Measured subjects and source builds are removed after packaging. This closes
the L1 follow-up rather than retaining an absent-instrument ceiling. No engine,
epoch, generated tracker, checkbox, L3 or model decision change applies.

Binding review correction: the committed line-shift control rejects +1, +2
and reversed locator positions. Disabling native prefix requirements produces
the focused FAIL, then restoration passes. The r4 receipt records that FAIL,
the +1 refusal reason and unchanged native input/population hashes. All six
canonical inputs were rerun with both brackets; the L2 52-source population
and one reference were also rerun. No source or required kind was trimmed.

Conditions: one-minute load 1.79 at measurement start, nice 10, under the lane
serial guard on a shared machine; other-lane activity is uncontrolled. These
sub-millisecond, tiny-file timings vary, so the median verifies the floor and
is not a comparative benchmark. The subject was pruned after measurement.
Binary SHA is observed per build: both position drivers accept an explicit
-build-receipt, validating pinned archives/native sources, runtime version and
that build's binary SHA. Historical receipts are not edited to admit a new
build. The X-R4 tracing row and both assessment index links are corrected.

Final review-delta checks: 22 native tests, zero skips; focused extractor races, strict reading round-trip, release-notes, merge-debris and ADR-011 pass. X-R5 scan finds only the test decorator, no subject people or attribution lists.
