# Fixed native command profiles, 2026-10-24

Status: GT4A qualified within the fixed profiles and observed hosts below.
Broad GT4, other scanner/PII profiles, native Git/auth/hooks,
installation and production economics remain open. Contract: [NQ](../../../designs/command-native-qualification.md).

## Actual observations

- Final owned helper-Mac runs: [18 core cases](native-mac-core.json) and
  [27 scanner-profile cases](native-mac-scanner.json), both all pass, darwin/arm64.
  Each scanner profile includes the core roster. No cases were skipped or marked N/A.
- [Hosted native observations](hosted/summary.json), source `996cb5e403595b5ff6a3088f11f9d1e310726b0e`:
  Linux/amd64, Windows/amd64 and macOS/arm64 each pass all 18 core cases. Actual
  receipts and source/runner byte pins are retained per host. Both CUE versions
  validate all three receipts. [Workflow run](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40132941681).
  Publication normalizes CRLF to LF without changing decoded JSON. The summary
  separates original artifact hashes from published-file hashes; original bytes
  remain in the private backup and the hosted artifacts during their retention.
- [Source/binary pins](source-pins.json) bind the archived base, exact new Go/CUE
  files, trusted Gitleaks binary and actual qualified executables. The helper used
  Go 1.27.3. This was a working candidate built from a source archive, not a clean
  Git build assertion. Documentation corrections do not change those Go/CUE pins.
- CUE 0.16.1 and 0.17.1 each accept 39 actual candidate reports across ordinary JSON,
  indented JSON, real compact tuples and successful ordinary compact fallback, plus
  four actual runner JSON/text receipts. Six tiny-cap calls correctly emit no report.
  Default emitted-receipt tests also cover pass/fail/unknown/preflight and reject
  extra fields, invalid counts/status and missing cases. CUE shape is not ordered
  roster execution proof; Go additionally validates IDs/order and case histograms.
- Helper `go test -race`, `go vet` and Linux/Windows amd64 cross-builds pass.
  Cross-builds are not native host evidence. The controller's full `task validate`
  passes with CUE 0.17.1. No five-root library/exported API changed.
  [Implementation CI](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40132941627)
  also passes the full gate, WASI governance build and WASM example at `996cb5e`.
- Independent security/source review and guide-only use found and repaired the
  defects below. The guide trial recomputed binary pins, passed both profiles on
  the controller Mac, observed wrong-pin exit 2/all cases unknown, and rejected a
  case-skip flag with exit 2/no output. A final fixed-help test proves no candidate
  execution and correct writer-failure handling.
- The adoption skill explicitly routes requested native qualification and excludes
  ordinary read/Git use. Python's skill validator was unavailable because PyYAML is
  absent; Ruby YAML frontmatter checks and independent behavioral guide use passed.
  No dependency was installed to make a validation tool available.

## Retained failures and remedies

1. [Attempt 1](attempt-1-core.json): 15 false failures/3 passes. The runner wrongly
   required optional `externalScanner` in ordinary core reports. An explicit required
   field list plus real-report regression fixes the oracle; the guard was correct.
2. Real stdout/stderr subprocess tests caught a cap bypass in the new runner:
   embedding `bytes.Buffer` promoted `ReadFrom`, allowing `io.Copy` to bypass capped
   `Write`. Both channels accepted 1,048,577 bytes in the red test. A private buffer
   field removes that alternate write path; actual overflow tests now hold. This
   pre-publication defect was not evidence of a deployed guard bypass.
3. [Attempt 2](attempt-2-scanner.json): core passed; A/B/A scanner cases falsely
   failed because the oracle omitted B's selected line. Actual owned captures and
   independent source review confirmed five rows. The corrected exact oracle accepts
   that row and rejects its omission. Scanner actual/reused counters were unchanged.
   Captured candidate files also needed `.json` extensions for CUE encoding inference.
4. Attempt 3: both runtime profiles passed, and CUE 0.16.1 passed, but CUE 0.17.1
   rejected actual runner receipts. The redundant outer `close` interacting with
   conditionals caused top-level `field not allowed` failures. Removing only that
   wrapper leaves the definition closed. Both versions still reject added fields,
   bad counts, inconsistent status and a wrong profile. A default non-native
   emitted-report test now detects the wire failure without opt-in native pins.
5. Review strengthened the roster descriptor, successful compact fallback, finite
   unknown-code validation, direct duplicate-argument test, and transitive Go workflow
   triggers. The descriptor is not a hash of the whole implementation: external
   source/runner identity remains required.
6. Initial full validation correctly rejected stale generated catalog/package count.
   Regenerating the catalog and updating README to 220 packages repaired the docs;
   no gate was changed or bypassed.

Original source snapshots, failed/green logs, binaries, hashes, CUE reduction probes,
review and guide receipts are retained in the private dated command-native backup.
Published receipts contain no source bodies, subprocess diagnostics or host paths.
They are observational records, not signed attestations or authority to disclose data.

The additive hosted workflow builds at its recorded source revision, observes the
fixed core roster and retains sanitized receipts even on failure. Those build hashes
identify bytes, not independent trust. Existing CI, Taskfile, Lefthook and machine
configuration are unchanged. Fresh [isolated-source proof](proof.txt) verifies all nine claimed
command rows; this is static symbol/test reachability, separate from the native
observations above. Broader GT4 requires its separate native surface matrix.
