# Frozen Fathom candidate: scoped regression checks

## Prospective contract

R4/R5's [frozen review packet](fixtures/adoption-analytics-2026-09-21/fathom/README.md)
has focused guide/traffic tests and an ordinary build, but no retained race/vet or
existing Observatory regression result. This check closes that prerequisite only.

- Subject: the retained `appkit-r5-candidate.7QEdhc/src` candidate based on Fathom
  `c58adaa2`, with exact app-kit `f7ab0fbd3496e9dc3da8c97b6c4290f09a71ec50`.
  Archive a fresh copy outside both active repositories, omitting only the generated
  `fathom` executable and disposable `workspace/`. Freeze file identities and commands
  before execution. Do not incorporate the separate CSS experiment's output.
- Run existing synthetic tests matching
  `^(TestAppkitTraffic|TestObservatoryThemeGuide|TestThemeGuideRejectsNonLoopbackBeforeListening|TestObservatory|TestBuildObservatoryModel)`
  in `./cmd/fathom` under `-race -count=1 -timeout=120s`, then package `go vet`.
  No new fixtures, assertions, source fixes or broad-suite execution by implication.
- Require readonly manifests, local Go toolchain and existing offline module cache.
  Use isolated writable build/temp directories and process-local sandbox protection
  against active-repository reads/writes and external network. Review selected tests'
  data/listener boundaries before execution; no real logs, daemon or user data.
- Retain full output and process exit results. Budget: one run per command, at most
  eight minutes for test compilation/execution and four for vet. A failed command
  is a result, not automatic permission to retry or weaken assertions. Stop only
  owned processes if the cap is reached; wait for confirmed exit before cleanup.
- Verify the archived original and copied source/manifests remain unchanged. Report
  exact test count, skipped tests and static-vet scope; no full-suite, active consumer,
  browser, human usability, guide promotion or G3/G4 completion claim.
- Authority: isolated synthetic candidate checks only. Active Fathom and Conjure,
  consumer pins/dependencies, installed binaries, deployments and global config stay
  untouched. Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-fathom-regressions.EvmvBu`.

## Result

Both commands passed on their first execution. The race run reports **36 top-level
tests, 104 test/subtest passes, zero failures and zero skips**. Package execution
took 2.105 seconds; compilation plus execution took about 60 seconds. Vet exited 0
with empty stdout/stderr. These are check durations, not dashboard performance.

The selected tests cover existing Observatory model/rendering/source-validity/error
regressions plus the prepared guide/traffic changes. They use temporary synthetic
files and in-process HTTP recorders, with one loopback `httptest.NewServer`. The
existing `TestMain` creates temporary home/cache roots; the harness clears inherited
`FATHOM_*` overrides. No live daemon or user log was selected. Source and module-cache
writes were denied during the commands, as were active-repository access and external
networking; local loopback was allowed for the synthetic server/refusal tests.

Pre/post manifests are byte-identical: **4,882 entries**, including eight unchanged
internal agentic-kit skill symlinks (not followed during inventory). Every original
selected entry also still matches. Independent review checked all 1,473 non-stamp
app-kit mirror files against the exact f7ab archive reference: no missing or differing
files. This checks the mirror body, not only its stamp. Both owned process groups
are absent after exit; the private temporary directory is empty. Source, manifests,
existing CSS, active consumers and installed binaries remain unchanged.

This closes the frozen candidate's scoped race/vet/regression prerequisite, not
active adoption, a full Fathom suite, browser equivalence, human usability or G3/G4.
Reuse it only for these exact sources. A later integration still requires rebase/
pin review and checks against the actual applied consumer source.

## Inputs, commands and retained diagnostics

Local toolchain: `/etc/profiles/per-user/geoff/bin/go`, Go 1.26.3 darwin/arm64.
From the copied `src/`, commands under the retained process-local sandbox were:

```sh
go test -race -json -mod=readonly ./cmd/fathom \
  -run '^(TestAppkitTraffic|TestObservatoryThemeGuide|TestThemeGuideRejectsNonLoopbackBeforeListening|TestObservatory|TestBuildObservatoryModel)' \
  -count=1 -timeout=120s
go vet -mod=readonly ./cmd/fathom
```

Both use `GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
GOFLAGS=-mod=readonly GOMAXPROCS=2 CGO_ENABLED=1`, existing
`GOMODCACHE=/Users/geoff/.dev-cache/go/modules`, and the evidence root's isolated
`GOCACHE=…/go-cache TMPDIR=…/tmp GOTMPDIR=…/tmp`. The build cache was privately cloned
from the retained candidate's cache; this was not a cold build or dependency download.
The frozen runner applies 480/240-second outer limits, retains direct file output and
records exit/timeout/escalation; neither timeout nor escalation occurred.

Before either Go command, packaging/preflight caught harness errors: BSD tar's exclude
pattern omitted `cmd/fathom` as well as the root executable; the first source verifier
rejected legitimate internal skill links; and Seatbelt required `localhost`, not a
literal IPv4 host, in its network rule. The rejected archive/copy and preflight inputs
remain retained. The corrected archive enumerates exact top-level entries; allowed
skill symlinks must resolve inside the mirrored skill tree. Source verification and
sandbox parsing passed before the single race/vet acquisitions. These preflight errors
are not candidate test failures or evidence of an application fix.

Paths below are relative to `/Volumes/m2ext/appkit-evidence/appkit-fathom-regressions.EvmvBu`.

| Artifact | SHA-256 |
|---|---|
| `candidate-inputs-02.tar` | `e88645036255f4f12beae04b261b8b8a36ba9ca316e6c32b784ba032a6ca58e3` |
| `executed-inputs.tar` (runner, verifier, sandbox, prospective contract, pre-run manifest) | `ad14616116870e2bfb35bf7e2346139869e3b44e664921e280dba032672e362d` |
| `source-before-02.json` and `source-after.json` | `b6c5651d3102c7c609063d23eab15d6f096286dda91594b5fbbb8185776314cc` |
| `race-01.stdout` (JSON events) | `240efb5ac54816c068c84ef25ede2d4466bf45b2d1f70591b42130531ff9d5a6` |
| `race-01.exit.json` | `e610f52bf96d042dcbe73c0c34ab35735eab0346030211b81a683d6850fc7165` |
| `vet-01.exit.json` | `08ba171c85ceb78f7d217c292132f548f37467f26a836be78aea315837498353` |
| `race-01.stderr`, `vet-01.stdout`, `vet-01.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `preflight-inputs-01.tar` (rejected verifier/sandbox) | `f6097396ce0958092d13cf972ebcc8a5f501904d6d3899068588f9a8b8b8611c` |

The first rejected archive is `candidate-inputs.tar`, SHA-256
`02a4e0a2bc65e8f8733684fc82edc8294c36e8fd4803fcc601b1aa418c330137`;
its extraction remains `rejected-source-01/`. Neither was used for Go execution.
