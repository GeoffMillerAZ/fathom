# Round-3 archive verification and D24 reconciliation

The committed tools were run from the repository root against the committed archives:

```sh
python3 calibration/bench/tools/verify.py --archive docs/bench/readings/bench-prep/raw/bench-subjects-r1-logs.tar.gz
python3 calibration/bench/tools/verify_r2.py --archive docs/bench/readings/bench-prep/raw/bench-subjects-r2-offline-logs.tar.gz
```

Both exited 0. Round 1: all 30 historical trial logs match their committed hashes.
Round 2: archive checksum and all 73 raw offline logs match; 42 timed trials,
39 green and three linux/arm64 Flow platform failures. No trials were rerun for
this verification; these are historical readings, not the revised harness.

`c6-d24-sha256.json`, beside `c6-r2.json`, commits SHA-256 digests of all twenty
complete D24 recount outputs, including their full site lists. Python outputs
were regenerated with the committed `c6_python.py` and `recount.py`, using
CPython 3.12.15; all pins were checked locally. Attrs gives **143, 55, 34** sites
for `attr._next_gen.field`, `attr._next_gen.define`, `attr._make.Factory`.
The **reading**, rather than the tool, is corrected: the old 138-site field
count was stale. CPython 3.9.6 skips the pattern-matching test file and gives
138/54/34. Pinned CPython 3.12.15 includes five more field calls and a define
call at `tests/test_pattern_matching.py:82:10`, giving 143/55/34. The review
reported 143/54/34; the additional define site is directly resolved by this run. The D24 qualifying-symbol count remains three. D26 product-only
counts supersede these historical admission claims in `c6-r2.md`.
