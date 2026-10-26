# Subject preparation: current D26 reading

D26 retains **Go 0, Python 1, TypeScript 2** preparation candidates. No BX-44
subject is admitted. Class-validator is retained for BX-M0 only because C5
fails. C4 is unrun; C5 has zero fail-before/pass-after replays and its post-cutoff
half awaits Q2. No plan box changed and no paid API call was made.

| Language | Kept after the D26 preparation screen | Refused, with criterion | Dev-forced by prior read |
|---|---|---|---|
| Go | none | chi, urfave/cli, Testify: C6; goose: C3/C6; cobra: C6/C7; go-git: C3 in this isolation setup | cobra (also refused; own corpus) |
| Python | pytest | Starlette, marshmallow, Click, attrs, Rich: C6 | marshmallow and Rich (both now refused) |
| TypeScript | redux-toolkit; class-validator for BX-M0 only (C5 fails for BX-44) | Immer: C6 and C3 scoped to linux/arm64 Flow; tsyringe: C2/C6; Redux, Remeda, class-transformer: C6 (class-transformer also C5 for BX-44); trpc: C3 in this isolation setup | none |

Every candidate, kept or refused, has a `product_only` count and witnesses in
[c6-r2.json](c6-r2.json). Product-only qualifying counts are pytest 4,
class-validator 4 and redux-toolkit 4; trpc 11 and go-git 45 pass C6 but fail C3.
All other candidates have zero. The full method and twenty-row table are in
[c6-r2.md](c6-r2.md). Historical D24 outputs are bound by
[c6-d24-sha256.json](c6-d24-sha256.json); [verification-r3.md](verification-r3.md)
corrects the attrs reading using recorded CPython 3.12.15 (143/55/34 sites).

Cobra fails C7 because `calibration/corpus.yaml:268` already includes it;
[c7-r3.json](c7-r3.json) records both failure and prior read. The live fleet
exclusion remains unknown for the other candidates. Rich/marshmallow's prior
reads remain recorded even though D26 now refuses them. There is **no retained
dev-forced candidate**; no held-out split was assigned.

[D23 draft parity](draft-status-r3.json) records the identical admission-manifest
deferral reason for all twelve historical/new drafts, including pytest and
redux-toolkit: star/age metadata must be observed by the controller at vetting.
The ten existing preparation files are historical drafts and receive no
admission exception. Missing metadata was not invented or fetched under this
round's package-fetch-only network allowance. Their setup/test environment and
Rich export staging are explicit under design section 5's mount contract.

[C5 recount](c5-r3.json) covers all twenty pins using committer dates for both
filtering and recording and `\b(fix(es|ed)?|bugs?|regressions?)\b`. It supersedes
all earlier fix-message estimates; they are surrogates, not replayed fixes.
Class-transformer has zero qualifying commits since 2024-10-25, and
class-validator six; both remain ineligible for BX-44's C5 requirement.

## Section-5 shortfalls

The target is six subjects per language, three dev and three held-out, across
three size bands. Against even the preparation-candidate target, Go is short
**6**, Python **5**, TypeScript **4** (or **5** excluding the C5-failing
BX-M0-only class-validator). No subjects have completed all C1–C7 vetting, so
formal admission remains short six in every language. No retained split is
assigned: every language is short **3 dev and 3 held-out**. Historical prior
reads among refused candidates do not fill those slots.

Go has no retained size band: small, middle and large are all empty. Python
retains only large (pytest, 117,244 lines), missing small and middle. TypeScript
retains small (class-validator, 14,455; BX-M0 only) and large (redux-toolkit,
86,935), missing middle; for BX-44 it also lacks small. The round-2 search
vetted six Go, six Python and eight TypeScript candidates, so the approximate
eight-candidate search target remains short two for Go and Python. No
replacement candidates were searched or fetched in this round.

## Proof and remaining measurements

Both committed archives were verified in place: round 1, 30 matching trial
logs; round 2, 73 matching logs, 42 timed trials, 39 green and three linux/arm64
Flow failures. The committed verification commands/results are in
[verification-r3.md](verification-r3.md).

[Testify readonly remeasurement](testify-r3.md) has one green warm-up and three
green offline trials, with committed [receipts](testify-r3.json) and six raw logs.
The instrument copies are explicitly [labeled as edited](instruments-r3.json)
where they changed after the round-2 runs. C3 remains incomplete for the design:
native macOS and the exact twelve-core condition were not measured. Trpc's
three timed trials and go-git's full warm-up/three trials remain unrun after
witnessed failures in the measured setup. Immer's Flow failure proves only the
linux/arm64 refusal, not another architecture or macOS.

`go test -race ./schema/bench`, `task cue-vet`, manifest shell/environment
checks, C5 committer-date checks, D26 policy/count/digest checks, Testify log
verification, and `command gofmt -l` pass. `task validate` was attempted and is
**blocked before its checks by the sandbox-denied shared gate lease**; the full
gate remains unverified and was not bypassed. C4, formal graph certification,
actual C5 fix replay, model-cutoff yield after Q2, controller split assignment,
D23 admission observations and the other live C7 fleet checks remain unmeasured.
