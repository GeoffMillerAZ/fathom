# Guide-only scanner reuse trial

**Scope:** prebuilt inspector only; no source edits/builds/providers/network. Scanner `/Volumes/m2ext/tmp/agentic-kit-gitleaks-8.30.1/gitleaks`, SHA-256 `ba52fb1bfabbcde42f032afad3d6e0b19dff8ed105229a16e7caa338bbc0e84f`, version `8.30.1`. `timeoutMillis=10000`, `maxReportBytes=65536`; controls outside selected root. Synthetic text only. Required scanner scope restored to `gitleaks`.

Initial request repeated path `a.txt` for A/B/A. That returned exit 2 `invalid` before reads. Preserved original at `/Volumes/m2ext/tmp/command-reuse-guide-20261024-a1-before-distinct-paths`. Corrected to three distinct paths `a.txt`, `b.txt`, `c.txt`, where `c.txt` exactly copies A. Independent contract: selected paths must be distinct.

Command for valid cases: `python3 /Volumes/m2ext/tmp/command-reuse-guide-20261024-a1/rerun.py` (each call invokes `/Volumes/m2ext/tmp/command-guard-reuse inspect --root ... --scope ... --policy ... --request ... --format ...`; `TMPDIR=/Volumes/m2ext/tmp/agentic-kit-build`).

- Enabled JSON, text, compact, and second invocation: exit 0; each reported `expectedFiles=3`, `attempted/invoked/completed=2`, `bytesSubmitted/Observed=95`, `reuse.files=1`, `reuse.bytes=48`; all report selected logical bytes 143. Second invocation repeated two scans, confirming invocation-local reuse. Text and compact retained reuse metadata.
- Disabled: exit 0; 3 attempted/invoked/completed, 143 submitted/observed; no `reuse` field.
- Late regex blocker: exit 1 `rule`; scanner `not_invoked`, zero subprocesses, zero rows.
- Wrong scanner hash: exit 2 `scanner_incomplete/hash_mismatch`; zero subprocesses and rows.
- Missing required scanner scope: exit 2 `invalid`, zero reads. This is expected control mismatch, not a security bypass.

Friction/fix: initial A/B/A repeated a path despite the distinct-selected-path contract; use distinct paths with equal bytes. One early missing-scope control was accidentally left active; restored independently from the documented scope shape. Default Gitleaks coverage is profile-specific, not universal PII detection. No latency or broader host/production/authority claims.

Scratch and scripts retained: `/Volumes/m2ext/tmp/command-reuse-guide-20261024-a1`.
