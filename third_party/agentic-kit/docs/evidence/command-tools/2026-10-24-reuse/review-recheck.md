# GT13b F1/F2 recheck

Reviewed the integrated root scanner, CUE schema and new negative/worst-corpus tests against `/Volumes/m2ext/tmp/command-reuse-integrated-pins.json`. No builds or native runs performed here.

**F1 closed in source.** `#ScannerCompleteCoverage` now has two mutually exclusive branches: absent `reuse` requires `completedFiles == expectedFiles`; present `reuse` requires `completedFiles + reuse.files == expectedFiles`. This is applied to successful ordinary/compact file and Git reports, including scanner-finding reports. `#ExternalScanner` and `#GitExternalScanner` retain the partial-coverage upper bound on incomplete reports. The new tests remove `reuse` from undercovered file/Git output and report CUE 0.16/0.17 rejection. I did not independently rerun CUE.

**F2 closed in source.** The scanner now hashes each selected input initially, before and after its own runner call, and once across the corpus before successful return. Cache hits check their current input; the final pass catches persistent mutation of a prior input. Cost is linear in selected bytes, including with reuse disabled. The worker's 32 x 2 MiB test reports 128 witness hashes and 32 real calls, matching the source's four passes per input. Deadline is checked after the final pass and wins over mutation errors where time has expired.

No new blocker found in this narrow reread. Private copies, byte equality after hash, cap fallback, actual/reuse counter separation, final coverage equation and deferred cleanup hold remain intact. The production transport does not mutate caller slices; an in-process injected runner is trusted and can fabricate evidence. Pinned native agreement, paired overhead measurement, full gate and publication are still pending root receipts.
