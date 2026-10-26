# Round-1 raw trial logs

`bench-subjects-r1-logs.tar.gz` is committed at `docs/bench/readings/bench-prep/raw/bench-subjects-r1-logs.tar.gz`.
SHA-256: `f737d9bb3f9b1100bcb7cda969084a9fae8ff88c4f3da4d32920d4039a70b844`.

The archive contains exactly 30 raw logs, one for each `log_sha256` in the ten subjects' historical `bench.evidence.suite.runs` receipts in `receipts-r1.json`. Each member is named `<subject-id>-run<N>.log`; its bytes match that trial’s committed hash. No log was rewritten or sanitized. Tar metadata omits account names and uses zero timestamps. Identical log contents retain separate members for each trial. Setup failures and warm-ups are not these thirty claimed green trials.

Instruments are committed in `calibration/bench/tools/`; see its README for inputs, commands, limitations and the historical mutable-module deviation. Verify the archive with `python3 calibration/bench/tools/verify.py --archive "$job/bench-subjects-r1-logs.tar.gz"`.
