# GT13b CUE fix recheck

**No blocker found in the two-file correction.** `schema.cue:52-59` uses conditional constraints in one open helper: absent `reuse` requires completed=expected; present `reuse` requires completed+reused=expected. The trailing `...` permits the rest of the closed scanner receipt without allowing unknown public fields through `#ExternalScanner`/`#GitExternalScanner`. The helper is applied to complete and scanner-finding file/Git ordinary and compact reports. Partial reports retain the upper-bound law.

`scanner_reuse_test.go` now locates `cue` through PATH, avoiding personal binary paths. If unavailable, the test explicitly skips schema qualification; CI's separate CUE setup/gate remains necessary. Added disabled complete/finding file/Git cases in JSON, text and compact, plus missing-reuse negatives. No hidden runtime contract or production Go change in this correction. Source hashes match `/Volumes/m2ext/tmp/command-reuse-cue-fix-receipt.md`.

The worker reports old-schema red and CUE 0.16/0.17 focused race green. I did not rerun builds. Native attempt 1 remains failed; attempt 2 and its timing checkpoint have no verdict yet.
