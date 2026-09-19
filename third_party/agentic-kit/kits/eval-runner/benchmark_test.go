package evalrunner

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	recordfingerprint "github.com/geoffmilleraz/agentic-kit/foundations/evidence/record-fingerprint"
)

func TestBenchmarkManifestFingerprintCanonicalAndComplete(t *testing.T) {
	manifest := testManifest()
	first, err := manifest.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	permuted := BenchmarkManifest{
		Version:  manifest.Version,
		Workload: manifest.Workload,
		Solution: map[string]string{
			"context":    manifest.Solution["context"],
			"retrieval":  manifest.Solution["retrieval"],
			"workflow":   manifest.Solution["workflow"],
			"tools":      manifest.Solution["tools"],
			"parameters": manifest.Solution["parameters"],
			"model":      manifest.Solution["model"],
			"prompt":     manifest.Solution["prompt"],
			"code":       manifest.Solution["code"],
		},
		Evaluation: reverseComponents(manifest.Evaluation),
	}
	second, err := permuted.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("permuted maps changed fingerprint: %s != %s", first, second)
	}
	permuted.Solution["code"] = digest("different-code")
	changed, err := permuted.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if first == changed {
		t.Fatal("dependency change did not change fingerprint")
	}
	delete(permuted.Solution, "prompt")
	if err := permuted.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("missing required component error = %v", err)
	}

	noLLM := testManifest()
	noLLM.Solution["prompt"] = NotApplicableDigest()
	noLLM.Solution["model"] = NotApplicableDigest()
	noLLM.Solution["retrieval"] = NotApplicableDigest()
	if err := noLLM.Validate(); err != nil {
		t.Fatalf("not-applicable hashes must remain valid caller evidence: %v", err)
	}
}

func TestBenchmarkEvidenceTimestampRoundTrip(t *testing.T) {
	base := testEvidence(testManifest(), BenchmarkModeLive, testTrial("task", "one", BenchmarkOutcomeAccepted))
	for name, stamp := range map[string]time.Time{
		"year outside JSON range":     time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		"offset outside JSON range":   base.ObservedAt.In(time.FixedZone("wide", 25*60*60)),
		"offset seconds lost by JSON": base.ObservedAt.In(time.FixedZone("seconds", 1)),
	} {
		t.Run(name, func(t *testing.T) {
			evidence := base
			evidence.ObservedAt, evidence.ValidUntil = stamp, stamp.Add(time.Hour)
			if _, err := SummarizeBenchmark(evidence); !errors.Is(err, ErrInvalidBenchmark) {
				t.Fatalf("unrepresentable timestamp summarized: %v", err)
			}
			candidate := base
			candidate.Manifest = candidateManifest()
			if _, err := CompareBenchmarks(base, candidate, stamp, 17, 100, .95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
				t.Fatalf("unrepresentable comparison time accepted: %v", err)
			}
		})
	}
	base.ObservedAt = base.ObservedAt.In(time.FixedZone("half-hour", 30*60))
	wire, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip BenchmarkEvidence
	if err := json.Unmarshal(wire, &roundTrip); err != nil {
		t.Fatal(err)
	}
	before, err := base.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	after, err := roundTrip.Fingerprint()
	if err != nil || before != after {
		t.Fatalf("valid timezone changed evidence identity through JSON: %s vs %s (%v)", before, after, err)
	}
}

func TestBenchmarkRetainedConsumerReceipt(t *testing.T) {
	wire, err := os.ReadFile("../../docs/evidence/benchmarks/2026-09-16/consumer-receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	var retained struct {
		Evidence  BenchmarkEvidence `json:"evidence"`
		Summary   BenchmarkSummary  `json:"summary"`
		Artifacts map[string]string `json:"artifacts"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(wire)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&retained); err != nil {
		t.Fatal(err)
	}
	actual, err := SummarizeBenchmark(retained.Evidence)
	if err != nil || !reflect.DeepEqual(actual, retained.Summary) {
		t.Fatalf("retained arithmetic no longer reconstructs: %v\n%+v", err, actual)
	}
	if retained.Evidence.Mode != BenchmarkModeSynthetic || actual.Trials != 3 || actual.EventualAccepted != 1 || actual.MissingCostTrials != 3 {
		t.Fatal("synthetic consumer integration was relabeled or changed")
	}
	for digest, body := range retained.Artifacts {
		if recordfingerprint.HashContent([]byte(body)) != digest {
			t.Fatalf("retained artifact bytes changed: %s", digest)
		}
	}
	for _, trial := range retained.Evidence.Trials {
		digests := []string{trial.TrialSpecDigest}
		for _, ref := range []*string{trial.AcceptanceDigest, trial.OutputDigest} {
			if ref != nil {
				digests = append(digests, *ref)
			}
		}
		for _, digest := range digests {
			if _, ok := retained.Artifacts[digest]; !ok {
				t.Fatalf("trial %s lost required artifact %s", trial.TaskID, digest)
			}
		}
	}
	candidate := retained.Evidence
	candidate.Manifest = candidateManifest()
	if _, err := CompareBenchmarks(retained.Evidence, candidate, retained.Evidence.ObservedAt, 17, 100, .95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("synthetic evidence entered comparison: %v", err)
	}
}

func TestBenchmarkTrialValidation(t *testing.T) {
	trial := testTrial("task-1", "r-1", BenchmarkOutcomeAccepted)
	if err := trial.Validate(); err != nil {
		t.Fatal(err)
	}
	trial.OutputDigest = nil
	if err := trial.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("accepted without output digest error = %v", err)
	}
	trial = testTrial("task-1", "r-1", BenchmarkOutcomeAccepted)
	trial.AcceptanceDigest = nil
	if err := trial.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("accepted without acceptance digest error = %v", err)
	}
	rejected := testTrial("task-1", "r-1", BenchmarkOutcomeRejected)
	rejected.AcceptanceDigest = nil
	if err := rejected.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("rejected without acceptance digest error = %v", err)
	}
	noVerdict := testTrial("task-1", "r-1", BenchmarkOutcomeError)
	noVerdict.AcceptanceDigest = stringPtr(digest("unexpected-acceptance"))
	if err := noVerdict.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("no-verdict acceptance digest error = %v", err)
	}
	trial = testTrial("task-1", "r-1", BenchmarkOutcomeAccepted)
	trial.TrialSpecDigest = ""
	if err := trial.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("missing trial spec digest error = %v", err)
	}
	trial = testTrial("task-1", "r-1", BenchmarkOutcomeAccepted)
	negative := -1.0
	trial.FirstUsefulMS = &negative
	if err := trial.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("negative useful time error = %v", err)
	}
	trial = testTrial("task-1", "r-1", BenchmarkOutcomeAccepted)
	infinite := math.Inf(1)
	trial.DurationMS = infinite
	if err := trial.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("infinite duration error = %v", err)
	}

	zero := 0.0
	observedZero := testTrial("task-2", "r-1", BenchmarkOutcomeRejected)
	observedZero.CostUSD = &zero
	observedZero.CostBasis = BenchmarkCostObserved
	if err := observedZero.Validate(); err != nil {
		t.Fatalf("observed zero is a real known cost: %v", err)
	}
	unknown := observedZero
	unknown.CostUSD = nil
	unknown.PriceDigest = nil
	unknown.CostBasis = BenchmarkCostUnknown
	unknown.CostComplete = false
	if err := unknown.Validate(); err != nil {
		t.Fatalf("unknown cost should remain distinct from observed zero: %v", err)
	}
	unknown.CostUSD = &zero
	if err := unknown.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("unknown cost with zero value error = %v", err)
	}
}

func TestBenchmarkEvidenceDuplicateTrials(t *testing.T) {
	evidence := testEvidence(testManifest(), BenchmarkModeLive,
		testTrial("task-1", "r-1", BenchmarkOutcomeAccepted),
		testTrial("task-1", "r-1", BenchmarkOutcomeRejected),
	)
	if err := evidence.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("duplicate trial error = %v", err)
	}
}

func TestBenchmarkEvidenceRejectsUnsafeIdentityPricesAndNonFiniteCost(t *testing.T) {
	manifest := testManifest()
	manifest.Version = 0
	if err := manifest.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("zero manifest version error = %v", err)
	}
	evidence := testEvidence(testManifest(), BenchmarkModeLive, testTrial("task-1", "r-1", BenchmarkOutcomeAccepted))
	evidence.ObservedAt = time.Time{}
	if err := evidence.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("zero evidence time error = %v", err)
	}
	evidence = testEvidence(testManifest(), BenchmarkModeLive, testTrial("bad\x00task", "r-1", BenchmarkOutcomeAccepted))
	if err := evidence.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("control character task id error = %v", err)
	}
	evidence = testEvidence(testManifest(), BenchmarkModeLive, testTrial("task-1", "r-1", BenchmarkOutcomeAccepted))
	evidence.ValidUntil = evidence.ObservedAt
	if err := evidence.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("zero validity window error = %v", err)
	}
	evidence = testEvidence(testManifest(), BenchmarkModeLive, testTrial("task-1", "r-1", BenchmarkOutcomeAccepted))
	evidence.Trials[0].PriceDigest = stringPtr(digest("wrong-pricing"))
	if err := evidence.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("price manifest mismatch error = %v", err)
	}
	evidence = testEvidence(testManifest(), BenchmarkModeLive, testTrial("task-1", "r-1", BenchmarkOutcomeAccepted))
	nan := math.NaN()
	evidence.Trials[0].CostUSD = &nan
	if err := evidence.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("NaN cost error = %v", err)
	}
	evidence = testEvidence(testManifest(), BenchmarkModeLive, testTrial("task-1", "r-1", BenchmarkOutcomeAccepted))
	evidence.Trials[0].AttemptCount = maxBenchmarkTrials + 1
	if err := evidence.Validate(); !errors.Is(err, ErrInvalidBenchmark) {
		t.Fatalf("attempt bound error = %v", err)
	}
}

func TestBenchmarkEvidenceFingerprintCanonicalAndSensitive(t *testing.T) {
	evidence := testEvidence(testManifest(), BenchmarkModeLive,
		testTrial("task-b", "r-1", BenchmarkOutcomeRejected),
		testTrial("task-a", "r-1", BenchmarkOutcomeAccepted),
	)
	first, err := evidence.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	permuted := evidence
	permuted.Trials = []BenchmarkTrial{evidence.Trials[1], evidence.Trials[0]}
	second, err := permuted.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("trial order changed evidence fingerprint: %s != %s", first, second)
	}
	tampered := evidence
	tampered.Trials = append([]BenchmarkTrial(nil), evidence.Trials...)
	tampered.Trials[0].TrialSpecDigest = digest("other-conditions")
	changed, err := tampered.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if first == changed {
		t.Fatal("trial receipt change did not change evidence fingerprint")
	}
	tampered = evidence
	tampered.Trials = append([]BenchmarkTrial(nil), evidence.Trials...)
	tampered.Trials[0].DurationMS++
	changed, err = tampered.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if first == changed {
		t.Fatal("trial numeric change did not change evidence fingerprint")
	}
}

func TestSummarizeBenchmarkCostCoverageAndFailures(t *testing.T) {
	accepted := testTrial("task-1", "r-1", BenchmarkOutcomeAccepted)
	accepted.CostUSD = floatPtr(2)
	accepted.DurationMS = 10
	accepted.AcceptedMS = floatPtr(12)
	failed := testTrial("task-2", "r-1", BenchmarkOutcomeRejected)
	failed.CostUSD = floatPtr(3)
	failed.DurationMS = 20
	timedOut := testTrial("task-3", "r-1", BenchmarkOutcomeTimeout)
	timedOut.CostUSD = nil
	timedOut.PriceDigest = nil
	timedOut.CostBasis = BenchmarkCostUnknown
	timedOut.CostComplete = false
	timedOut.DurationMS = 30

	summary, err := SummarizeBenchmark(testEvidence(testManifest(), BenchmarkModeReplay, accepted, failed, timedOut))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Outcomes[BenchmarkOutcomeAccepted] != 1 || summary.Outcomes[BenchmarkOutcomeTimeout] != 1 {
		t.Fatalf("outcome counts = %#v", summary.Outcomes)
	}
	if summary.TotalKnownCostUSD != 5 || summary.KnownCostTrials != 2 {
		t.Fatalf("known cost omitted failed work: %#v", summary)
	}
	if summary.EvidenceFingerprint == "" || summary.TotalCostByBasisUSD[BenchmarkCostObserved] != 5 ||
		summary.TotalCostByBasisUSD[BenchmarkCostUsage] != 0 || summary.TotalCostByBasisUSD[BenchmarkCostEstimated] != 0 {
		t.Fatalf("evidence fingerprint or cost basis totals = %#v", summary)
	}
	if !summary.CostLowerBound || summary.CostCoverageComplete || summary.CostPerAttemptedTaskUSD != nil || summary.CostPerAcceptedUSD != nil {
		t.Fatalf("partial/unknown cost presented as complete: %#v", summary)
	}
	if summary.EventualSuccessRate == nil || math.Abs(*summary.EventualSuccessRate-1.0/3.0) > 1e-12 {
		t.Fatalf("eventual rate = %v", summary.EventualSuccessRate)
	}
	if summary.P50DurationMS == nil || *summary.P50DurationMS != 20 || summary.P50AcceptedMS == nil || *summary.P50AcceptedMS != 12 {
		t.Fatalf("duration/accepted quantiles = %#v", summary)
	}
	estimated := testTrial("task-estimated", "r-1", BenchmarkOutcomeRejected)
	estimated.CostBasis = BenchmarkCostEstimated
	estimated.CostComplete = false
	summary, err = SummarizeBenchmark(testEvidence(testManifest(), BenchmarkModeLive, estimated))
	if err != nil {
		t.Fatal(err)
	}
	if summary.CostLowerBound || !strings.Contains(summary.CostWarning, "estimated") {
		t.Fatalf("estimated partial cost claimed a lower bound: %#v", summary)
	}
	allFailure := testTrial("task-4", "r-1", BenchmarkOutcomeError)
	allFailure.CostUSD = floatPtr(0)
	summary, err = SummarizeBenchmark(testEvidence(testManifest(), BenchmarkModeLive, allFailure))
	if err != nil {
		t.Fatal(err)
	}
	if summary.EventualSuccessRate == nil || *summary.EventualSuccessRate != 0 || summary.P50AcceptedMS != nil || summary.CostPerAcceptedUSD != nil {
		t.Fatalf("all-failure denominators/absent accepted metrics = %#v", summary)
	}
}

func TestCompareBenchmarksPairingFreshnessAndClusteredCI(t *testing.T) {
	baselineManifest := testManifest()
	candidateManifest := testManifest()
	candidateManifest.Solution["code"] = digest("candidate")
	baseline := testEvidence(baselineManifest, BenchmarkModeLive,
		testTrial("task-a", "r-1", BenchmarkOutcomeRejected),
		testTrial("task-a", "r-2", BenchmarkOutcomeRejected),
		testTrial("task-b", "r-1", BenchmarkOutcomeAccepted),
	)
	candidate := testEvidence(candidateManifest, BenchmarkModeLive,
		testTrial("task-a", "r-1", BenchmarkOutcomeAccepted),
		testTrial("task-a", "r-2", BenchmarkOutcomeAccepted),
		testTrial("task-b", "r-1", BenchmarkOutcomeAccepted),
	)
	now := baseline.ObservedAt.Add(time.Hour)
	first, err := CompareBenchmarks(baseline, candidate, now, 17, 200, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CompareBenchmarks(baseline, candidate, now, 17, 200, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("seeded comparison changed: %#v != %#v", first, second)
	}
	if first.Pairs != 3 || first.Difference.Tasks != 2 || first.Warning == "" {
		t.Fatalf("replicates treated as independent tasks: %#v", first)
	}
	if first.BaselineEvidenceFingerprint == "" || first.CandidateEvidenceFingerprint == "" {
		t.Fatalf("comparison missing evidence fingerprints: %#v", first)
	}
	knownError := candidate
	knownError.Trials = append([]BenchmarkTrial(nil), candidate.Trials...)
	knownError.Trials[0].Outcome = BenchmarkOutcomeError
	knownError.Trials[0].AcceptedMS = nil
	knownError.Trials[0].AcceptanceDigest = nil
	withError, err := CompareBenchmarks(baseline, knownError, now, 17, 200, 0.95)
	if err != nil || withError.CandidateAccepted != 2 {
		t.Fatalf("known terminal error must remain a non-accepted pair: comparison=%#v error=%v", withError, err)
	}

	missing := candidate
	missing.Trials = missing.Trials[:2]
	if _, err := CompareBenchmarks(baseline, missing, now, 17, 200, 0.95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("missing pair error = %v", err)
	}
	if _, err := CompareBenchmarks(baseline, candidate, baseline.ValidUntil, 17, 200, 0.95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("expiry boundary error = %v", err)
	}
	if _, err := CompareBenchmarks(baseline, candidate, baseline.ObservedAt.Add(-time.Nanosecond), 17, 200, 0.95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("future-observed error = %v", err)
	}
	replay := candidate
	replay.Mode = BenchmarkModeReplay
	if _, err := CompareBenchmarks(baseline, replay, now, 17, 200, 0.95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("replay comparison error = %v", err)
	}
	synthetic := candidate
	synthetic.Mode = BenchmarkModeSynthetic
	if _, err := CompareBenchmarks(baseline, synthetic, now, 17, 200, 0.95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("synthetic comparison error = %v", err)
	}
	mismatchedEvaluation := candidate
	mismatchedEvaluation.Manifest = testManifest()
	mismatchedEvaluation.Manifest.Solution["code"] = digest("candidate")
	mismatchedEvaluation.Manifest.Evaluation["grader"] = digest("other-grader")
	if _, err := CompareBenchmarks(baseline, mismatchedEvaluation, now, 17, 200, 0.95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("evaluation mismatch error = %v", err)
	}
	if _, err := CompareBenchmarks(baseline, baseline, now, 17, 200, 0.95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("same manifest error = %v", err)
	}
	differentTasks := candidate
	differentTasks.Trials[2].TaskID = "task-c"
	if _, err := CompareBenchmarks(baseline, differentTasks, now, 17, 200, 0.95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("same-length different task set error = %v", err)
	}
	mismatchedSpec := candidate
	mismatchedSpec.Trials = append([]BenchmarkTrial(nil), candidate.Trials...)
	mismatchedSpec.Trials[0].TrialSpecDigest = digest("different-input-or-schedule")
	if _, err := CompareBenchmarks(baseline, mismatchedSpec, now, 17, 200, 0.95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("different trial spec error = %v", err)
	}
	unknown := candidate
	unknown.Trials = append([]BenchmarkTrial(nil), candidate.Trials...)
	unknown.Trials[0].Outcome = BenchmarkOutcomeUnknown
	unknown.Trials[0].AcceptedMS = nil
	unknown.Trials[0].AcceptanceDigest = nil
	if _, err := CompareBenchmarks(baseline, unknown, now, 17, 200, 0.95); !errors.Is(err, ErrBenchmarkComparisonRefused) {
		t.Fatalf("unknown outcome comparison error = %v", err)
	}
}

func TestBenchmarkJSONPointerShape(t *testing.T) {
	trial := testTrial("task-json", "r-1", BenchmarkOutcomeError)
	trial.OutputDigest = nil
	trial.AcceptanceDigest = nil
	trial.AcceptedMS = nil
	trial.FirstVisibleMS = nil
	trial.FirstUsefulMS = nil
	encoded, err := json.Marshal(trial)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"\"output_digest\":null", "\"acceptance_digest\":null", "\"accepted_ms\":null", "\"first_visible_ms\":null", "\"first_useful_ms\":null"} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("serialized shape missing %s: %s", field, encoded)
		}
	}
}

func TestBenchmarkJSONProjectsToCUE(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue not installed")
	}
	evidence := testEvidence(testManifest(), BenchmarkModeLive, testTrial("task-cue", "r-1", BenchmarkOutcomeAccepted))
	summary, err := SummarizeBenchmark(evidence)
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := CompareBenchmarks(evidence, testEvidence(candidateManifest(), BenchmarkModeLive, testTrial("task-cue", "r-1", BenchmarkOutcomeAccepted)), evidence.ObservedAt.Add(time.Hour), 3, 100, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	for name, definition := range map[string]string{
		"evidence":   "#BenchmarkEvidence",
		"summary":    "#BenchmarkSummary",
		"comparison": "#BenchmarkComparison",
	} {
		var payload any
		switch name {
		case "evidence":
			payload = evidence
		case "summary":
			payload = summary
		default:
			payload = comparison
		}
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), name+".json")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("cue", "vet", "-d", definition, "benchmark.cue", path)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s JSON did not project to CUE: %v\n%s", name, err, output)
		}
	}
}

func testManifest() BenchmarkManifest {
	return BenchmarkManifest{
		Version:  BenchmarkManifestVersion,
		Workload: "fixture-workload",
		Solution: map[string]string{
			"code":       digest("code"),
			"prompt":     digest("prompt"),
			"model":      digest("model"),
			"parameters": digest("parameters"),
			"tools":      digest("tools"),
			"workflow":   digest("workflow"),
			"retrieval":  digest("retrieval"),
			"context":    digest("context"),
		},
		Evaluation: map[string]string{
			"dataset":       digest("dataset"),
			"acceptance":    digest("acceptance"),
			"grader":        digest("grader"),
			"protocol":      digest("protocol"),
			"environment":   digest("environment"),
			"pricing":       digest("pricing"),
			"useful_result": digest("useful-result"),
		},
	}
}

func testEvidence(manifest BenchmarkManifest, mode BenchmarkMode, trials ...BenchmarkTrial) BenchmarkEvidence {
	observed := time.Date(2026, time.September, 16, 0, 0, 0, 0, time.UTC)
	return BenchmarkEvidence{
		Manifest:   manifest,
		Mode:       mode,
		ObservedAt: observed,
		ValidUntil: observed.Add(24 * time.Hour),
		Trials:     trials,
	}
}

func testTrial(task, replicate string, outcome BenchmarkOutcome) BenchmarkTrial {
	output := digest(task + replicate + "output")
	price := digest("pricing")
	accepted := 20.0
	visible := 5.0
	useful := 10.0
	trial := BenchmarkTrial{
		TaskID:          task,
		ReplicateID:     replicate,
		Outcome:         outcome,
		AttemptCount:    1,
		TrialSpecDigest: digest(task + replicate + "input-schedule-conditions"),
		CostUSD:         floatPtr(1),
		CostBasis:       BenchmarkCostObserved,
		PriceDigest:     &price,
		CostComplete:    true,
		DurationMS:      15,
		FirstVisibleMS:  &visible,
		FirstUsefulMS:   &useful,
	}
	if outcome == BenchmarkOutcomeAccepted {
		trial.OutputDigest = &output
		trial.AcceptedMS = &accepted
	}
	if outcome == BenchmarkOutcomeAccepted || outcome == BenchmarkOutcomeRejected {
		acceptance := digest(task + replicate + "acceptance-result")
		trial.AcceptanceDigest = &acceptance
	}
	return trial
}

func floatPtr(value float64) *float64 { return &value }

func stringPtr(value string) *string { return &value }

func candidateManifest() BenchmarkManifest {
	manifest := testManifest()
	manifest.Solution["code"] = digest("candidate-cue")
	return manifest
}

func digest(value string) string {
	return recordfingerprint.HashContent([]byte(value))
}

func reverseComponents(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
