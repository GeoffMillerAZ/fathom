# Cloud sessions: readings

Readings for `docs/plans/cloud-sessions.md`. Each reading is one line that begins with its row id, which the
row's lexical oracle matches; the sections under it say what was run. The owner's real-session readings (CS-M1,
CS-M2) are not here yet.

## CS-8: `task test` on Linux as root

Taken 2026-10-03 on branch `cs8` off `4d2005f0`, with no model spend: the fix is `9514bf98`, and `b0bdc41b`
was the same branch before its last test-only changes (below).

CS-8 reading: task test green on Linux as root (ubuntu:24.04, 4 vCPU, aarch64, root, PID 1 docker-init). At `9514bf98`, `task validate` exited 0 in 7 min 6 s with 8 GB of RAM and 12 GB of guest swap; that run included `task test`, every package and 1,933 `cmd/fathom` tests in four shards. At `b0bdc41b`, `task test` exited 0 in 9 min 21 s with 16 GB, the VM's documented size. Before the fix, the same suite failed 18 tests in 8 packages as root.

### The machine

- **VM.** A Colima VM on the owner's Mac: aarch64, 4 vCPU, 16 GB, later 8 GB with a 12 GB guest swapfile on the
  VM's disk. The documented cloud VM is x86_64 with 4 vCPU and 16 GB (`docs/intent/domain.md`). The architecture differs because the host is Apple silicon. Nothing in
  the failures below depended on it.
- **Container.** `ubuntu:24.04` plus apt's `golang-go git build-essential curl ca-certificates python3 perl jq`,
  run as root with `--init`. Ubuntu's Go 1.22 means the switch to go.mod's toolchain was exercised. The tree was
  streamed in with `.git` and owned by root.
- **Provisioning.** `FATHOM_CLOUD_SKIP_INGEST=1 bash scripts/cloud-setup.sh` took 92 s and set up go1.26.2,
  `bin/fathom`, task v3.52.0, cue v0.16.1 and lefthook v1.13.6. The suite ran in the environment
  `cloud-session.sh` exports: go.mod's toolchain first on PATH and `TMPDIR=/root/.fathom-cloud/tmp`.

### Before: what failed at `4d2005f0`

Measured in the same image on an 8 GB VM, with no init (`sleep infinity` as PID 1):

| Package | Failing tests | Cause |
|---|---|---|
| `cmd/fathom` | TestFlywheelReadErrorIsOpenNeverIdle, TestCmdRefactorPlan_UnreadableSourceFailsLoud, TestServeHTTPWaitsForStoreWithoutExiting | root reads a mode-000 file or directory |
| `cmd/fathom` | TestDaemonStatusReportsSigningIdentity, TestRestartHintNamesDaemonStartWhenKillSwitchSet | darwin-only behavior asserted on Linux |
| `cmd/fathom` | TestServeHTTPHeldPortExits75AndRecordsHolder | no lsof in the image, so no holder named (product) |
| `cmd/fathom` | TestGitToplevelCachedPerCwd | ext4/overlayfs reused the removed `.git`'s inode for the re-init |
| `cmd/fathom` | TestLefthook_PrePushVerifyReceivesStdinFromLefthook | lefthook's `sh` is dash, and the verify body was bash (product) |
| `internal/conserve` | TestGitShowFile_UnreadableBlobIsAnError | root reads a mode-000 git object |
| `internal/custodian` | TestADRReferencesNamesUnreadableCitingFiles | root reads a mode-000 file |
| `internal/docindex` | TestDocCorpusUnreadableDirErrors | root lists a mode-000 directory |
| `internal/readcoverage` | four tests | root reads a mode-000 file |
| `internal/scenariobench` | TestClaudeHarness_RunSession_StreamWritePermissionDeniedIsAnInstrumentError | root writes a mode-0444 file |
| `internal/semantic` | TestInstallPack_MidCopyFailureLeavesNoDebris | root reads a mode-000 file |
| `internal/fusion` | TestCommandTreeKillsGrandchildOnCancel | with no init, the killed grandchild stays a zombie, and a zombie answers signal 0 |
| `internal/extractor` | none (binary killed) | 8 GB is too little at the default concurrency (passes alone in 59 s) |

Later full runs added one more failure. In 2 of 3 runs, TestPolyglotIngestHoldsModulePool
(`internal/extractor/tsengine`) read "-1 module(s) left open". An earlier test's abandoned runaway was reaped
mid-test once load stretched it past two minutes.

### What changed

- **Injected refusals.** Product readers go through `internal/fsfault`, and the test calls `fsfault.Deny`. Six
  readers: the call-log opens, the connector walk, adrrefs, the doc corpus walk, pack copy, and the
  scenariobench transcript write.
- **Real refusals root also gets.** These cover reads that are not fathom's own: a corrupt loose object for git,
  a directory in the file's place for the kit's read manifest, and a file in the store directory's place
  (ENOTDIR) for the serve-http child. Each test keeps its chmod variant for non-root users.
- **Linux equivalents.** On Linux, status neither reads nor prints a signature, and the restart hint is the
  foreground `serve-http` line.
- **Product fixes**, each red before its fix:
  - The pre-push verify body is POSIX sh. The gate body tests also run it under `sh`.
  - ListenerPID falls back to procfs's socket tables.
  - The toplevel cache checks `<top>/.git` itself.
  - The module-leak check counts owned modules, not live ones.
- **The grandchild test** asserts that no process in the group is still running. It reads `/proc` on Linux, where
  a zombie counts as gone.

### After

- **16 GB, `--init`, `b0bdc41b`.** `task test` exit 0 in 9 min 21 s. Shard walls were 282, 373, 269 and 362 s.
  An earlier run on the same machine, before the module-leak fix, peaked at 14.9 GB of cgroup memory (page cache
  included).
- **Two `task validate` runs at that stage.** All twelve dependency steps passed, including clean-clone-check
  and stranger-check, and so did store-purego (without `-race`) and scripts-check. In the test step, a
  different load flake fired each time. Each flake was fixed with a test-only change and stress-run in the
  container: the ETXTBSY write (at least 10 of 40 loaded runs before, 290 after with none), the console
  fixture's clock race and lost seed (2 of 30 before, 0 of 100 after), and a SIGPIPE in
  `scripts/cloud-setup_test.sh` (here-strings).
- **8 GB plus 12 GB guest swap, `--init`, `9514bf98` (the fix as committed).** `task validate` exit 0 in
  7 min 6 s, `cmd/fathom` shards 130, 154, 131 and 159 s. Guest swap held 53 MB at the end. The 16 GB VM was
  not run again: alongside the default 8 GB Colima VM it pushed the host's macOS swap to 24 GB and filled the
  Mac's internal disk.
- **8 GB, no init**, at an earlier stage of the fix: all four `cmd/fathom` shards and `internal/fusion` (the
  grandchild test) were green.
- **macOS**, with `-race`: every changed package passed in full, as did the affected `cmd/fathom` tests. The full
  macOS gate runs on the push.

### What this reading does not say

- **8 GB without swap is not enough.** On an 8 GB machine with no swap, the default concurrency gets
  `internal/extractor`'s test binary killed. With guest swap the suite passed and barely used it.
- **Some checks are still skipped for root.** 41 tests skip their unreadable-file check as root. They pass and
  cover nothing for root. They predate CS-8, and routing each through `fsfault` is follow-up work.
  - `cmd/fathom`, 21: console_learn ×2, console_plansdata, console_project_tabs, console_projects,
    console_search, console_truth, console_wavef_truth ×3, coverage ×2, doctor_intent, doctor_packrows,
    doctor_serviceroots, doctor_wiring, instrument_adoption, intentdoctrine, killswitch_review,
    lifecycle_findings, plans_active.
  - `internal`, 20: certify gaps and gaps_unread; console knowledge; custodian codefacts ×3, commands and corpus;
    docindex ingest; fusion docindexpass and servicerefs ×2; hookguard killswitch; intent bets, coverage and
    visions ×4; wiring.
- **Not the real VM.** This is not an x86_64 run and not the platform's own VM. CS-M2 is the reading from a real
  cloud session.
