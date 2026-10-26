# decision-lab

Offline authoring and diagnosis for bounded text-only string-choice sites. It prepares a canonical decision-port request, checks a captured port result, and studies independently labeled cases. It does not call a provider, load keys, read source references, mutate a cache, run a subprocess, or authorize an action. The shared decision port still supports predicate, score, and images; this utility intentionally does not.

The recipes use the existing [decision-site guide](../../docs/decision-sites.md), [model authority boundary](../../docs/intent/decisions/0026-reject-model-only-authority-and-completeness.md), and [reading method](../../docs/fast-judgment.md). The three example packs are `fixtures/review-triage.pack.json`, `fixtures/retrieval-support.pack.json`, and `fixtures/clarification.pack.json`. Each declares observable instructions and option criteria, fallback, baseline, kill criterion, provider, resolved model, owner, policy epoch, unknown option, and independently owner-authored diagnostic favorable options. These are site recipes, not permission policies.

## Independent offline trial

From the repository root:

```sh
trial_dir=$(mktemp -d)
go run ./cmd/decision-lab prepare --pack cmd/decision-lab/fixtures/review-triage.pack.json --input cmd/decision-lab/fixtures/case-1.input.json --out "$trial_dir/prepared.json" --format json
go run ./cmd/decision-lab check --pack cmd/decision-lab/fixtures/review-triage.pack.json --input cmd/decision-lab/fixtures/case-1.input.json --capture cmd/decision-lab/fixtures/case-1.capture.json --format json
go run ./cmd/decision-lab study --pack cmd/decision-lab/fixtures/review-triage.pack.json --manifest cmd/decision-lab/fixtures/review-triage.manifest.json --captures cmd/decision-lab/fixtures/review-triage.captures.json --format json
go run ./cmd/decision-lab prepare --pack cmd/decision-lab/fixtures/retrieval-support.pack.json --input cmd/decision-lab/fixtures/retrieval-1.input.json --out "$trial_dir/retrieval-prepared.json" --format json
go run ./cmd/decision-lab check --pack cmd/decision-lab/fixtures/retrieval-support.pack.json --input cmd/decision-lab/fixtures/retrieval-1.input.json --capture cmd/decision-lab/fixtures/retrieval-1.capture.json --format json
go run ./cmd/decision-lab study --pack cmd/decision-lab/fixtures/retrieval-support.pack.json --manifest cmd/decision-lab/fixtures/retrieval-support.manifest.json --captures cmd/decision-lab/fixtures/retrieval-support.captures.json --format json
```

The `check` command needs **one** capture, while the supplied captures fixture is an array for `study`. Review triage has three cases, including a false favorable answer and missing-context `unknown`. Retrieval has seven cases spanning `supported`, `contradicted`, `mixed`, `unsupported`, and `unknown`, plus a false favorable answer and a supported case predicted `unknown`. The retrieval recipe distinguishes support from contradiction and mixed evidence; it does not replace an existing source-support reviewer. Both complete studies exit 0 despite diagnostic errors. Inspect `falseFavorable`, `abstainedExpectedFavorable`, `classes`, and each reading's `n`. These synthetic semantic labels were AI-authored, so provenance is `frontier`; they are not independently adjudicated acceptance evidence.

`prepare` defaults to metadata only. `--out PATH` creates a new file exclusively with mode `0600`; it fails if the path exists. `--out -` deliberately sends the full prepared envelope, including `Request.State`, as JSON to stdout for a pipe. Keep captured state and envelopes in an appropriate private location. The CLI never echoes captured state, provider refusal text, invalid-answer text, or transport error bodies in its normal diagnostic output. Its error reasons are fixed codes with next steps.

## JSON contract

Owner-authored pack, input, capture, and manifest fields use lower camel case. `prepared.request` is the **decision port** request with Go-capitalized `State`, `Questions`, `ID`, `Kind`, `Instructions`, `Options`, `Name`, and `Criteria`. A captured `response` is the **decision port** response with Go-capitalized `Provider`, `Model`, `Answers`, `Invalid`, `Declined`, `Usage`, and `Latency`. Each `Answers` entry has `ID`, `Kind`, `Choice`, `Probabilities`, and derived `Confidence`. `Latency` is an integer nanosecond duration as encoded by Go's `time.Duration`; `Usage` has `InputTokens` and `OutputTokens`. Raw OpenAI or other native provider JSON is unsupported. Translate it into a port response outside this utility, then retain the original capture separately if needed.

The capture pins `caseId`, `packDigest`, `inputDigest`, and `requestKey` from `prepare`. The pack explicitly names the supported `decision.Confidence/1` and `decision.NewReading/1` recipes; those names enter its digest and every reading key. The golden example is a three-option distribution of 0.8, 0.1, 0.1, which produces confidence 0.7. A changed core formula requires a new recipe identity and requalification. `answerCacheHits` is a required list of question IDs whose exact valid answers the client reused; only those question IDs are excluded from fresh calibration N. `attemptCount` is a required declaration of 0..32 attempts for the capture, including retries, and is reported without billing inference. This is distinct from a provider's prompt-prefix cache, which can still produce a fresh answer; the core `Usage` has no cache-read/write split, so the study does not sum token usage or claim savings. `usageUnreportedCaptures` counts captures with declared attempts but no response usage or zero input and output tokens; it marks unknown usage, not zero spend. Record either one typed `cause` (`no-provider`, `timeout`, `not-authorized`, `budget-exhausted`, `transport-error`) or a `response`, never both. Preflight causes require zero attempts; timeout and transport error require at least one. Reasons inside `Invalid` and `Declined` are stored as nonempty strings but not echoed.

For `study`, write the expected manifest independently of captures. Every case embeds its actual `input`, an independently declared `independentUnitId`, and a `labels` map with an exact expected option and `human`, `computed`, or `frontier` provenance for every question. Distinct cases must also have distinct recomputed `RequestKey` exposures; renamed case, subject, revision, or unit IDs cannot count the same request twice. Duplicate exposure exits 2 with `duplicate_exposure` and the validated case IDs. The unique unit IDs and request keys express study design and byte identity; they cannot authenticate empirical independence or truth. The manifest pins `datasetKind` (`synthetic` or `observed`) and the trusted pack digest. The entire manifest, pack, policy, question order, provider, resolved model, and reading recipe enter each reading key. Captures cannot reduce the expected cohort; omitted captures exit 2 with `missing_cases` and the validated missing case IDs. Source references are never opened.

All JSON inputs are capped at 1 MiB and depth 24, and reject duplicate, unknown or noncanonical fields, malformed UTF-8, trailing data, and nulls. IDs use 1..64 ASCII letters, digits, period, underscore, or hyphen, beginning with a letter or digit. The semantic pack/input digests and exact raw file digests are reported separately. Hashes establish identity, not producer authenticity or source truth. Use trusted inputs and independent labels before interpreting a study.

Exit 0 from `check` means every expected question has a known usable answer; 2 includes any unknown, refusal, invalid answer, missing outcome, or other unusable result. The report retains each validated answer's option, recomputed confidence, and answer reuse flag, including valid `unknown` answers; it never prints source or provider reasons. Valid known answers in a partial batch remain visible by question. Exit 0 from `study` means complete valid diagnostics even when predictions are poor or all attempts failed; 2 means malformed or incomplete data. The study reports all five cause counts, declared and failed attempts, cache hits, and captures with unreported usage. An unknown prediction against an expected favorable label increments `abstainedExpectedFavorable`, not `falseUnfavorable`. Neither exit status gives action authority, validates a threshold, promotes a model, proves cost savings, or replaces the baseline/kill review. No overall quality score is produced. The site owner must declare and compare its acceptance bars, including false favorable errors, abstentions, sample independence, and the baseline before adoption. A failed bar calls for the pack's fallback or kill criterion.

To verify the CUE shapes and Go behavior:

```sh
cd cmd/decision-lab
cue vet ./...
cue vet -c schema.cue -d '#Pack' fixtures/review-triage.pack.json
cue vet -c schema.cue -d '#Manifest' fixtures/review-triage.manifest.json
cue vet -c schema.cue -d '#Pack' fixtures/retrieval-support.pack.json
cue vet -c schema.cue -d '#Manifest' fixtures/retrieval-support.manifest.json
cue vet -c schema.cue -d '#Captures' fixtures/retrieval-support.captures.json
cd ../..
go test -race ./cmd/decision-lab
go vet ./cmd/decision-lab
```
