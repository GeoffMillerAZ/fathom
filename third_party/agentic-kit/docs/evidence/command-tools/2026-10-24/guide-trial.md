# GT-5 guide-only adopter trial, 2026-10-24

Source: synthetic files and copied controls under `/Volumes/m2ext/tmp/agentic-kit-gt5-guide-trial/`. Executables already present; no install, download, global config, private history, or model call. The scanner executable SHA-256 matched its local source receipt before use.

Command shape: `/Volumes/m2ext/tmp/command-guard-gt5 inspect --root /Volumes/m2ext/tmp/agentic-kit-gt5-guide-trial --scope <scratch>/scope.json --policy <scratch>/policy.json --request <scratch>/request-CASE.json --format json`. Scope required scanner ID `gitleaks`; policy used fixed `gitleaks-stdin-8.30.1`, absolute executable, verified hash, 5000 ms, 1048576-byte report cap. All controls and files were synthetic; this shared scratch directory is not a protected production policy source.

| Case | Exit | Sanitized result |
|---|---:|---|
| Guide's built-in read fixture | 0 | 25 bytes scanned, two rows |
| Own public read | 0 | 23 bytes read/scanned/submitted/observed, 1 invoked/completed, 0 findings, one row |
| Initial fictitious password string | 0 | 38 bytes observed, 0 findings, one row; not a useful scanner canary |
| Own deterministic fictitious ghp-shaped canary | 1 | scanner_finding, 51 bytes observed, 1 finding, 0 rows |
| Wrong executable hash | 2 | scanner_incomplete/hash_mismatch, 0 invoked/completed, 0 rows |

Guide and skill correctly forbid implicit download, hook changes, scanner waiver, and raw report exposure. The missing adoption aid is a bundled, explicitly fictitious positive scanner canary and a note that plausible password strings may not match embedded default rules. A clean report is limited to selected raw text and the configured regex plus Gitleaks default-rule profile; it does not prove secret or PII absence, organizational authorization, or native Windows/Linux behavior.
