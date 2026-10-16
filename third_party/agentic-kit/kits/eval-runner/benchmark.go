package evalrunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	recordfingerprint "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/record-fingerprint"
	reliabilitystats "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
)

const (
	// BenchmarkManifestVersion is the only manifest version supported by this
	// experimental evidence format.
	BenchmarkManifestVersion = 1

	// BenchmarkNotApplicableMarker is hashed by NotApplicableDigest when a
	// required dependency has no applicable implementation (for example, a
	// no-LLM workload's prompt component). The marker itself is never accepted
	// as a component digest.
	BenchmarkNotApplicableMarker = "not-applicable"

	// BenchmarkEvidenceTrustCallerDeclared says validation has checked only
	// structure and caller attestations. It is not a claim that calls, holdouts,
	// or grader independence were observed by this package.
	BenchmarkEvidenceTrustCallerDeclared = "caller-declared"

	maxBenchmarkComponents = 64
	maxBenchmarkTrials     = 10000

	benchmarkEvidenceFingerprintVersion = 1
)

var (
	// ErrInvalidBenchmark reports malformed experimental benchmark evidence.
	ErrInvalidBenchmark = errors.New("evalrunner: invalid benchmark evidence")
	// ErrBenchmarkComparisonRefused reports evidence that cannot support a
	// paired live-quality comparison.
	ErrBenchmarkComparisonRefused = errors.New("evalrunner: benchmark comparison refused")

	benchmarkDigestRE        = regexp.MustCompile(`^[0-9a-f]{64}$`)
	benchmarkComponentNameRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	benchmarkIDRE            = regexp.MustCompile(`^[ -~]{1,256}$`)
)

var requiredSolutionComponents = []string{
	"code", "prompt", "model", "parameters", "tools", "workflow", "retrieval", "context",
}

var requiredEvaluationComponents = []string{
	"dataset", "acceptance", "grader", "protocol", "environment", "pricing", "useful_result",
}

// BenchmarkManifest describes the caller-attested dependencies of one
// benchmark run. It is deliberately not a universal manifest: callers choose
// the complete dependency set for their workload, adding components as needed.
// Component values are SHA-256 digests, never secrets or provider credentials.
type BenchmarkManifest struct {
	Version    int               `json:"version"`
	Workload   string            `json:"workload"`
	Solution   map[string]string `json:"solution"`
	Evaluation map[string]string `json:"evaluation"`
}

// NotApplicableDigest returns the digest required for a required component
// that genuinely does not apply to a workload, such as RAG retrieval in a
// no-RAG workload.
func NotApplicableDigest() string {
	return recordfingerprint.HashContent([]byte(BenchmarkNotApplicableMarker))
}

// Validate verifies the bounded, caller-supplied benchmark identity.
func (m BenchmarkManifest) Validate() error {
	if m.Version != BenchmarkManifestVersion {
		return benchmarkInvalid("manifest version must be %d", BenchmarkManifestVersion)
	}
	if !validBenchmarkID(m.Workload) {
		return benchmarkInvalid("workload must be printable ASCII, non-empty, and at most 256 bytes")
	}
	if err := validateBenchmarkComponents("solution", m.Solution, requiredSolutionComponents); err != nil {
		return err
	}
	return validateBenchmarkComponents("evaluation", m.Evaluation, requiredEvaluationComponents)
}

// Fingerprint returns a versioned, domain-separated SHA-256 identity. It uses
// canonical Go JSON with sorted component slices, not JCS. A component digest
// remains distinct from any external freshness policy carried by the caller.
func (m BenchmarkManifest) Fingerprint() (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}
	canonical := struct {
		Domain     string               `json:"domain"`
		Version    int                  `json:"version"`
		Workload   string               `json:"workload"`
		Solution   []benchmarkComponent `json:"solution"`
		Evaluation []benchmarkComponent `json:"evaluation"`
	}{
		Domain:     "agentic-kit/eval-runner/benchmark-manifest/v1",
		Version:    m.Version,
		Workload:   m.Workload,
		Solution:   canonicalBenchmarkComponents(m.Solution),
		Evaluation: canonicalBenchmarkComponents(m.Evaluation),
	}
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: canonical manifest: %v", ErrInvalidBenchmark, err)
	}
	return recordfingerprint.HashContent(body), nil
}

type benchmarkComponent struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

func canonicalBenchmarkComponents(components map[string]string) []benchmarkComponent {
	canonical := make([]benchmarkComponent, 0, len(components))
	for name, digest := range components {
		canonical = append(canonical, benchmarkComponent{Name: name, Digest: digest})
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].Name < canonical[j].Name })
	return canonical
}

func validateBenchmarkComponents(kind string, components map[string]string, required []string) error {
	if len(components) == 0 || len(components) > maxBenchmarkComponents {
		return benchmarkInvalid("%s must contain 1 through %d components", kind, maxBenchmarkComponents)
	}
	for name, digest := range components {
		if !benchmarkComponentNameRE.MatchString(name) {
			return benchmarkInvalid("%s component name %q is invalid", kind, name)
		}
		if !benchmarkDigestRE.MatchString(digest) {
			return benchmarkInvalid("%s component %q must be a lowercase SHA-256 digest", kind, name)
		}
	}
	for _, name := range required {
		if _, ok := components[name]; !ok {
			return benchmarkInvalid("%s is missing required component %q", kind, name)
		}
	}
	return nil
}

// BenchmarkOutcome is the terminal outcome of an attempted benchmark task.
type BenchmarkOutcome string

const (
	// BenchmarkOutcomeAccepted means the output satisfied acceptance checks.
	BenchmarkOutcomeAccepted BenchmarkOutcome = "accepted"
	// BenchmarkOutcomeRejected means the output was produced but rejected.
	BenchmarkOutcomeRejected BenchmarkOutcome = "rejected"
	// BenchmarkOutcomeError means execution ended in an error.
	BenchmarkOutcomeError BenchmarkOutcome = "error"
	// BenchmarkOutcomeTimeout means execution exceeded its time limit.
	BenchmarkOutcomeTimeout BenchmarkOutcome = "timeout"
	// BenchmarkOutcomeCanceled means execution was intentionally canceled.
	BenchmarkOutcomeCanceled BenchmarkOutcome = "canceled"
	// BenchmarkOutcomeAbandoned means execution stopped without a conclusion.
	BenchmarkOutcomeAbandoned BenchmarkOutcome = "abandoned"
	// BenchmarkOutcomeUnknown means the terminal outcome could not be classified.
	BenchmarkOutcomeUnknown BenchmarkOutcome = "unknown"
)

// BenchmarkCostBasis identifies how the reported aggregate cost was obtained.
type BenchmarkCostBasis string

const (
	// BenchmarkCostObserved is a cost observed from authoritative billing data.
	BenchmarkCostObserved BenchmarkCostBasis = "observed"
	// BenchmarkCostUsage is a cost computed from recorded usage and pricing.
	BenchmarkCostUsage BenchmarkCostBasis = "usage"
	// BenchmarkCostEstimated is a cost calculated from an estimate.
	BenchmarkCostEstimated BenchmarkCostBasis = "estimated"
	// BenchmarkCostUnknown records that no aggregate cost is available.
	BenchmarkCostUnknown BenchmarkCostBasis = "unknown"
)

// BenchmarkMode is caller-declared run provenance. This package does not
// observe calls or independently establish the declaration. Replay and
// synthetic evidence can be summarized for bookkeeping but cannot establish
// comparative quality here.
type BenchmarkMode string

const (
	// BenchmarkModeLive is caller-declared live execution evidence.
	BenchmarkModeLive BenchmarkMode = "live"
	// BenchmarkModeReplay is caller-declared replay evidence.
	BenchmarkModeReplay BenchmarkMode = "replay"
	// BenchmarkModeSynthetic is caller-declared synthetic evidence.
	BenchmarkModeSynthetic BenchmarkMode = "synthetic"
)

// BenchmarkTrial is one terminal task/replicate observation. TrialSpecDigest
// identifies the caller-retained input, replicate assignment, and controlled
// conditions for exact pairing, excluding intentionally varied solution components.
// AcceptanceDigest identifies the caller-retained
// acceptance-check result for an accepted or rejected trial. CostUSD covers all
// work for the trial: producers, routers, reviewers, failed attempts, retries,
// cancellation, and probes—not only a final model response. CostComplete is the
// caller's attestation that this total has that coverage; the library cannot
// discover hidden calls.
type BenchmarkTrial struct {
	TaskID           string             `json:"task_id"`
	ReplicateID      string             `json:"replicate_id"`
	Outcome          BenchmarkOutcome   `json:"outcome"`
	AttemptCount     int                `json:"attempt_count"`
	TrialSpecDigest  string             `json:"trial_spec_digest"`
	OutputDigest     *string            `json:"output_digest"`
	AcceptanceDigest *string            `json:"acceptance_digest"`
	CostUSD          *float64           `json:"cost_usd"`
	CostBasis        BenchmarkCostBasis `json:"cost_basis"`
	PriceDigest      *string            `json:"price_digest"`
	CostComplete     bool               `json:"cost_complete"`
	DurationMS       float64            `json:"duration_ms"`
	AcceptedMS       *float64           `json:"accepted_ms"`
	FirstVisibleMS   *float64           `json:"first_visible_ms"`
	FirstUsefulMS    *float64           `json:"first_useful_ms"`
}

// Validate verifies a terminal trial and its aggregate cost coverage facts.
func (t BenchmarkTrial) Validate() error {
	if !validBenchmarkID(t.TaskID) {
		return benchmarkInvalid("task_id must be printable ASCII, non-empty, and at most 256 bytes")
	}
	if !validBenchmarkID(t.ReplicateID) {
		return benchmarkInvalid("replicate_id must be printable ASCII, non-empty, and at most 256 bytes")
	}
	if !validBenchmarkOutcome(t.Outcome) {
		return benchmarkInvalid("outcome %q is invalid", t.Outcome)
	}
	if t.AttemptCount < 1 || t.AttemptCount > maxBenchmarkTrials {
		return benchmarkInvalid("attempt_count must be between one and %d", maxBenchmarkTrials)
	}
	if !benchmarkDigestRE.MatchString(t.TrialSpecDigest) {
		return benchmarkInvalid("trial_spec_digest must be a lowercase SHA-256 digest")
	}
	if !finiteNonNegative(t.DurationMS) {
		return benchmarkInvalid("duration_ms must be finite and non-negative")
	}
	if err := validateOptionalDigest("output_digest", t.OutputDigest); err != nil {
		return err
	}
	if t.Outcome == BenchmarkOutcomeAccepted {
		if t.OutputDigest == nil {
			return benchmarkInvalid("accepted trial requires output_digest")
		}
		if t.AcceptedMS == nil || !finiteNonNegative(*t.AcceptedMS) || *t.AcceptedMS < t.DurationMS {
			return benchmarkInvalid("accepted trial requires finite accepted_ms at or after duration_ms")
		}
	} else if t.AcceptedMS != nil {
		return benchmarkInvalid("non-accepted trial must not set accepted_ms")
	}
	if t.Outcome == BenchmarkOutcomeAccepted || t.Outcome == BenchmarkOutcomeRejected {
		if err := validateRequiredDigest("acceptance_digest", t.AcceptanceDigest); err != nil {
			return err
		}
	} else if t.AcceptanceDigest != nil {
		return benchmarkInvalid("no-verdict trial must not set acceptance_digest")
	}
	if err := validateOptionalTime("first_visible_ms", t.FirstVisibleMS, t.completionBound()); err != nil {
		return err
	}
	if err := validateOptionalTime("first_useful_ms", t.FirstUsefulMS, t.completionBound()); err != nil {
		return err
	}
	return t.validateCost()
}

func (t BenchmarkTrial) completionBound() float64 {
	if t.Outcome == BenchmarkOutcomeAccepted && t.AcceptedMS != nil {
		return *t.AcceptedMS
	}
	return t.DurationMS
}

func (t BenchmarkTrial) validateCost() error {
	if !validBenchmarkCostBasis(t.CostBasis) {
		return benchmarkInvalid("cost_basis %q is invalid", t.CostBasis)
	}
	if t.CostBasis == BenchmarkCostUnknown {
		if t.CostUSD != nil || t.PriceDigest != nil || t.CostComplete {
			return benchmarkInvalid("unknown cost must omit cost_usd and price_digest and cannot be complete")
		}
		return nil
	}
	if t.CostUSD == nil || !finiteNonNegative(*t.CostUSD) {
		return benchmarkInvalid("known cost requires finite non-negative cost_usd")
	}
	return validateRequiredDigest("price_digest", t.PriceDigest)
}

func validBenchmarkID(value string) bool {
	return benchmarkIDRE.MatchString(value) && strings.TrimSpace(value) != ""
}

func validateOptionalDigest(name string, digest *string) error {
	if digest != nil && !benchmarkDigestRE.MatchString(*digest) {
		return benchmarkInvalid("%s must be a lowercase SHA-256 digest when present", name)
	}
	return nil
}

func validateRequiredDigest(name string, digest *string) error {
	if digest == nil || !benchmarkDigestRE.MatchString(*digest) {
		return benchmarkInvalid("%s must be a lowercase SHA-256 digest", name)
	}
	return nil
}

func validateOptionalTime(name string, value *float64, upper float64) error {
	if value != nil && (!finiteNonNegative(*value) || *value > upper) {
		return benchmarkInvalid("%s must be finite, non-negative, and no later than completion", name)
	}
	return nil
}

func validBenchmarkOutcome(outcome BenchmarkOutcome) bool {
	switch outcome {
	case BenchmarkOutcomeAccepted, BenchmarkOutcomeRejected, BenchmarkOutcomeError, BenchmarkOutcomeTimeout,
		BenchmarkOutcomeCanceled, BenchmarkOutcomeAbandoned, BenchmarkOutcomeUnknown:
		return true
	default:
		return false
	}
}

func validBenchmarkCostBasis(basis BenchmarkCostBasis) bool {
	switch basis {
	case BenchmarkCostObserved, BenchmarkCostUsage, BenchmarkCostEstimated, BenchmarkCostUnknown:
		return true
	default:
		return false
	}
}

func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

// BenchmarkEvidence is a bounded, time-scoped collection of terminal trials.
// ValidUntil gives callers—not this package's clock—the evidence freshness
// policy used by comparisons.
type BenchmarkEvidence struct {
	Manifest   BenchmarkManifest `json:"manifest"`
	Mode       BenchmarkMode     `json:"mode"`
	ObservedAt time.Time         `json:"observed_at"`
	ValidUntil time.Time         `json:"valid_until"`
	Trials     []BenchmarkTrial  `json:"trials"`
}

// Validate verifies evidence without consulting a clock.
func (e BenchmarkEvidence) Validate() error {
	if err := e.Manifest.Validate(); err != nil {
		return err
	}
	if e.Mode != BenchmarkModeLive && e.Mode != BenchmarkModeReplay && e.Mode != BenchmarkModeSynthetic {
		return benchmarkInvalid("mode %q is invalid", e.Mode)
	}
	if !validBenchmarkTime(e.ObservedAt) || !validBenchmarkTime(e.ValidUntil) || !e.ValidUntil.After(e.ObservedAt) {
		return benchmarkInvalid("timestamps must round-trip through RFC3339 JSON, with valid_until after observed_at")
	}
	if len(e.Trials) > maxBenchmarkTrials {
		return benchmarkInvalid("trials exceeds %d", maxBenchmarkTrials)
	}
	seen := make(map[benchmarkTrialPair]struct{}, len(e.Trials))
	for _, trial := range e.Trials {
		if err := trial.Validate(); err != nil {
			return err
		}
		if trial.CostUSD != nil && *trial.PriceDigest != e.Manifest.Evaluation["pricing"] {
			return benchmarkInvalid("price_digest must equal evaluation pricing component")
		}
		key := benchmarkTrialKey(trial)
		if _, ok := seen[key]; ok {
			return benchmarkInvalid("duplicate task_id and replicate_id %q/%q", trial.TaskID, trial.ReplicateID)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// JSON timestamp encoding cannot represent offset seconds or out-of-range
// years. Reject these before hashing/reporting can change or lose an instant.
func validBenchmarkTime(value time.Time) bool {
	if value.IsZero() || value.UTC().Year() < 0 || value.UTC().Year() > 9999 {
		return false
	}
	_, offset := value.Zone()
	if offset%60 != 0 {
		return false
	}
	_, err := value.MarshalJSON()
	return err == nil
}

// Fingerprint returns a versioned, domain-separated SHA-256 identity for all
// caller-declared evidence. It canonicalizes timestamps in UTC and trial order
// by task/replicate identity, but does not authenticate any retained receipt.
func (e BenchmarkEvidence) Fingerprint() (string, error) {
	if err := e.Validate(); err != nil {
		return "", err
	}
	manifestFingerprint, err := e.Manifest.Fingerprint()
	if err != nil {
		return "", err
	}
	canonical := struct {
		Domain              string                    `json:"domain"`
		Version             int                       `json:"version"`
		ManifestFingerprint string                    `json:"manifest_fingerprint"`
		Mode                BenchmarkMode             `json:"mode"`
		ObservedAt          string                    `json:"observed_at"`
		ValidUntil          string                    `json:"valid_until"`
		Trials              []canonicalBenchmarkTrial `json:"trials"`
	}{
		Domain:              "agentic-kit/eval-runner/benchmark-evidence/v1",
		Version:             benchmarkEvidenceFingerprintVersion,
		ManifestFingerprint: manifestFingerprint,
		Mode:                e.Mode,
		ObservedAt:          e.ObservedAt.UTC().Format(time.RFC3339Nano),
		ValidUntil:          e.ValidUntil.UTC().Format(time.RFC3339Nano),
		Trials:              canonicalBenchmarkTrials(e.Trials),
	}
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: canonical evidence: %v", ErrInvalidBenchmark, err)
	}
	return recordfingerprint.HashContent(body), nil
}

type canonicalBenchmarkTrial struct {
	TaskID           string             `json:"task_id"`
	ReplicateID      string             `json:"replicate_id"`
	Outcome          BenchmarkOutcome   `json:"outcome"`
	AttemptCount     int                `json:"attempt_count"`
	TrialSpecDigest  string             `json:"trial_spec_digest"`
	OutputDigest     *string            `json:"output_digest"`
	AcceptanceDigest *string            `json:"acceptance_digest"`
	CostUSD          *float64           `json:"cost_usd"`
	CostBasis        BenchmarkCostBasis `json:"cost_basis"`
	PriceDigest      *string            `json:"price_digest"`
	CostComplete     bool               `json:"cost_complete"`
	DurationMS       float64            `json:"duration_ms"`
	AcceptedMS       *float64           `json:"accepted_ms"`
	FirstVisibleMS   *float64           `json:"first_visible_ms"`
	FirstUsefulMS    *float64           `json:"first_useful_ms"`
}

func canonicalBenchmarkTrials(trials []BenchmarkTrial) []canonicalBenchmarkTrial {
	canonical := make([]canonicalBenchmarkTrial, 0, len(trials))
	for _, trial := range trials {
		canonical = append(canonical, canonicalBenchmarkTrial{
			TaskID:           trial.TaskID,
			ReplicateID:      trial.ReplicateID,
			Outcome:          trial.Outcome,
			AttemptCount:     trial.AttemptCount,
			TrialSpecDigest:  trial.TrialSpecDigest,
			OutputDigest:     trial.OutputDigest,
			AcceptanceDigest: trial.AcceptanceDigest,
			CostUSD:          trial.CostUSD,
			CostBasis:        trial.CostBasis,
			PriceDigest:      trial.PriceDigest,
			CostComplete:     trial.CostComplete,
			DurationMS:       trial.DurationMS,
			AcceptedMS:       trial.AcceptedMS,
			FirstVisibleMS:   trial.FirstVisibleMS,
			FirstUsefulMS:    trial.FirstUsefulMS,
		})
	}
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].TaskID == canonical[j].TaskID {
			return canonical[i].ReplicateID < canonical[j].ReplicateID
		}
		return canonical[i].TaskID < canonical[j].TaskID
	})
	return canonical
}

// BenchmarkSummary keeps absent metrics nil rather than presenting them as
// zero. Cost totals include accepted and non-accepted trials alike. First
// visible/useful timings remain raw trial facts in v1; this summary does not
// standardize or aggregate their semantics across workload collectors.
type BenchmarkSummary struct {
	EvidenceTrust           string                         `json:"evidence_trust"`
	Mode                    BenchmarkMode                  `json:"mode"`
	ObservedAt              time.Time                      `json:"observed_at"`
	ValidUntil              time.Time                      `json:"valid_until"`
	ManifestFingerprint     string                         `json:"manifest_fingerprint"`
	EvidenceFingerprint     string                         `json:"evidence_fingerprint"`
	Trials                  int                            `json:"trials"`
	Attempts                int                            `json:"attempts"`
	Outcomes                map[BenchmarkOutcome]int       `json:"outcomes"`
	FirstAttemptAccepted    int                            `json:"first_attempt_accepted"`
	EventualAccepted        int                            `json:"eventual_accepted"`
	FirstAttemptSuccessRate *float64                       `json:"first_attempt_success_rate"`
	EventualSuccessRate     *float64                       `json:"eventual_success_rate"`
	KnownCostTrials         int                            `json:"known_cost_trials"`
	MissingCostTrials       int                            `json:"missing_cost_trials"`
	TotalKnownCostUSD       float64                        `json:"total_known_cost_usd"`
	CostBasisCounts         map[BenchmarkCostBasis]int     `json:"cost_basis_counts"`
	TotalCostByBasisUSD     map[BenchmarkCostBasis]float64 `json:"total_cost_by_basis_usd"`
	CostCoverageComplete    bool                           `json:"cost_coverage_complete"`
	CostLowerBound          bool                           `json:"cost_lower_bound"`
	CostWarning             string                         `json:"cost_warning"`
	CostPerAttemptedTaskUSD *float64                       `json:"cost_per_attempted_task_usd"`
	CostPerAcceptedUSD      *float64                       `json:"cost_per_accepted_usd"`
	P50DurationMS           *float64                       `json:"p50_duration_ms"`
	P95DurationMS           *float64                       `json:"p95_duration_ms"`
	AcceptedTrials          int                            `json:"accepted_trials"`
	P50AcceptedMS           *float64                       `json:"p50_accepted_ms"`
	P95AcceptedMS           *float64                       `json:"p95_accepted_ms"`
}

// SummarizeBenchmark computes bookkeeping statistics. It does not infer hidden
// cost coverage or use a clock, and it is valid for live, replay, and
// synthetic evidence.
func SummarizeBenchmark(evidence BenchmarkEvidence) (BenchmarkSummary, error) {
	if err := evidence.Validate(); err != nil {
		return BenchmarkSummary{}, err
	}
	summary := BenchmarkSummary{
		EvidenceTrust:   BenchmarkEvidenceTrustCallerDeclared,
		Mode:            evidence.Mode,
		ObservedAt:      evidence.ObservedAt,
		ValidUntil:      evidence.ValidUntil,
		Trials:          len(evidence.Trials),
		Outcomes:        make(map[BenchmarkOutcome]int),
		CostBasisCounts: make(map[BenchmarkCostBasis]int),
		TotalCostByBasisUSD: map[BenchmarkCostBasis]float64{
			BenchmarkCostObserved:  0,
			BenchmarkCostUsage:     0,
			BenchmarkCostEstimated: 0,
		},
	}
	fingerprint, err := evidence.Manifest.Fingerprint()
	if err != nil {
		return BenchmarkSummary{}, err
	}
	summary.ManifestFingerprint = fingerprint
	evidenceFingerprint, err := evidence.Fingerprint()
	if err != nil {
		return BenchmarkSummary{}, err
	}
	summary.EvidenceFingerprint = evidenceFingerprint
	durations := make([]float64, 0, len(evidence.Trials))
	accepted := make([]float64, 0, len(evidence.Trials))
	completeCosts := len(evidence.Trials) > 0
	allKnownCostsExact := true
	for _, trial := range evidence.Trials {
		summary.Attempts += trial.AttemptCount
		summary.Outcomes[trial.Outcome]++
		summary.CostBasisCounts[trial.CostBasis]++
		if trial.CostBasis == BenchmarkCostEstimated {
			allKnownCostsExact = false
		}
		durations = append(durations, trial.DurationMS)
		if trial.Outcome == BenchmarkOutcomeAccepted && trial.AttemptCount == 1 {
			summary.FirstAttemptAccepted++
		}
		if trial.Outcome == BenchmarkOutcomeAccepted {
			summary.EventualAccepted++
			summary.AcceptedTrials++
			accepted = append(accepted, *trial.AcceptedMS)
		}
		if trial.CostUSD != nil {
			summary.KnownCostTrials++
			summary.TotalKnownCostUSD += *trial.CostUSD
			if !finiteNonNegative(summary.TotalKnownCostUSD) {
				return BenchmarkSummary{}, benchmarkInvalid("total known cost overflows finite range")
			}
			summary.TotalCostByBasisUSD[trial.CostBasis] += *trial.CostUSD
			if !finiteNonNegative(summary.TotalCostByBasisUSD[trial.CostBasis]) {
				return BenchmarkSummary{}, benchmarkInvalid("total cost for basis %q overflows finite range", trial.CostBasis)
			}
		} else {
			summary.MissingCostTrials++
		}
		if trial.CostUSD == nil || !trial.CostComplete {
			completeCosts = false
		}
	}
	if summary.Trials > 0 {
		first := float64(summary.FirstAttemptAccepted) / float64(summary.Trials)
		eventual := float64(summary.EventualAccepted) / float64(summary.Trials)
		summary.FirstAttemptSuccessRate = &first
		summary.EventualSuccessRate = &eventual
	}
	summary.CostCoverageComplete = completeCosts
	// A mixed estimated sum is never presented as a mathematical lower bound.
	// Known observed/usage non-negative costs remain a lower bound when caller
	// coverage is incomplete, even if other trials have unknown costs.
	summary.CostLowerBound = summary.KnownCostTrials > 0 && !completeCosts && allKnownCostsExact
	summary.CostWarning = benchmarkCostWarning(summary)
	if completeCosts && summary.Trials > 0 {
		value := summary.TotalKnownCostUSD / float64(summary.Trials)
		summary.CostPerAttemptedTaskUSD = &value
	}
	if completeCosts && summary.EventualAccepted > 0 {
		value := summary.TotalKnownCostUSD / float64(summary.EventualAccepted)
		summary.CostPerAcceptedUSD = &value
	}
	summary.P50DurationMS = benchmarkQuantile(durations, 0.50)
	summary.P95DurationMS = benchmarkQuantile(durations, 0.95)
	summary.P50AcceptedMS = benchmarkQuantile(accepted, 0.50)
	summary.P95AcceptedMS = benchmarkQuantile(accepted, 0.95)
	return summary, nil
}

func benchmarkCostWarning(summary BenchmarkSummary) string {
	parts := make([]string, 0, 2)
	if summary.CostBasisCounts[BenchmarkCostUnknown] > 0 || !summary.CostCoverageComplete {
		parts = append(parts, "partial cost coverage: caller attestation does not establish hidden-call completeness")
	}
	if summary.CostBasisCounts[BenchmarkCostEstimated] > 0 {
		parts = append(parts, "estimated cost present: aggregate and derived metrics include caller-declared estimates and are not an observed-cost claim or mathematical lower bound")
	}
	return strings.Join(parts, "; ")
}

func benchmarkQuantile(values []float64, p float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	position := p * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	value := sorted[lower] + (position-float64(lower))*(sorted[upper]-sorted[lower])
	return &value
}

// BenchmarkComparison is an experimental paired terminal-acceptance comparison.
// Difference is candidate accepted outcome minus baseline accepted outcome.
// Rejected, error, timeout, canceled, and abandoned terminal outcomes count as
// non-accepted; unknown outcomes are refused. Its interval is not a claim of
// universal superiority.
type BenchmarkComparison struct {
	EvidenceTrust                string                    `json:"evidence_trust"`
	BaselineMode                 BenchmarkMode             `json:"baseline_mode"`
	CandidateMode                BenchmarkMode             `json:"candidate_mode"`
	BaselineObservedAt           time.Time                 `json:"baseline_observed_at"`
	CandidateObservedAt          time.Time                 `json:"candidate_observed_at"`
	BaselineValidUntil           time.Time                 `json:"baseline_valid_until"`
	CandidateValidUntil          time.Time                 `json:"candidate_valid_until"`
	BaselineFingerprint          string                    `json:"baseline_fingerprint"`
	CandidateFingerprint         string                    `json:"candidate_fingerprint"`
	BaselineEvidenceFingerprint  string                    `json:"baseline_evidence_fingerprint"`
	CandidateEvidenceFingerprint string                    `json:"candidate_evidence_fingerprint"`
	AsOf                         time.Time                 `json:"as_of"`
	Seed                         int64                     `json:"seed"`
	Resamples                    int                       `json:"resamples"`
	Level                        float64                   `json:"level"`
	Difference                   reliabilitystats.Interval `json:"difference"`
	Pairs                        int                       `json:"pairs"`
	BaselineAccepted             int                       `json:"baseline_accepted"`
	CandidateAccepted            int                       `json:"candidate_accepted"`
	Warning                      string                    `json:"warning"`
}

// CompareBenchmarks compares paired, fresh, live evidence with a task-clustered
// bootstrap. Replays and synthetic records remain useful for SummarizeBenchmark
// but are refused here as quality evidence.
func CompareBenchmarks(baseline, candidate BenchmarkEvidence, now time.Time, seed int64, resamples int, level float64) (BenchmarkComparison, error) {
	if err := baseline.Validate(); err != nil {
		return BenchmarkComparison{}, benchmarkComparisonRefused("baseline: %v", err)
	}
	if err := candidate.Validate(); err != nil {
		return BenchmarkComparison{}, benchmarkComparisonRefused("candidate: %v", err)
	}
	if !validBenchmarkTime(now) {
		return BenchmarkComparison{}, benchmarkComparisonRefused("now must be supplied and round-trip through RFC3339 JSON")
	}
	if baseline.Mode != BenchmarkModeLive || candidate.Mode != BenchmarkModeLive {
		return BenchmarkComparison{}, benchmarkComparisonRefused("only live evidence supports quality comparison")
	}
	if !benchmarkFresh(baseline, now) || !benchmarkFresh(candidate, now) {
		return BenchmarkComparison{}, benchmarkComparisonRefused("evidence is not fresh at supplied now")
	}
	if resamples < 100 || resamples > 10000 {
		return BenchmarkComparison{}, benchmarkComparisonRefused("resamples must be between 100 and 10000")
	}
	if math.IsNaN(level) || math.IsInf(level, 0) || level <= 0 || level >= 1 {
		return BenchmarkComparison{}, benchmarkComparisonRefused("level must be finite and between zero and one")
	}
	if baseline.Manifest.Workload != candidate.Manifest.Workload ||
		!equalBenchmarkComponents(baseline.Manifest.Evaluation, candidate.Manifest.Evaluation) {
		return BenchmarkComparison{}, benchmarkComparisonRefused("workload and evaluation manifest must match")
	}
	baselineFingerprint, err := baseline.Manifest.Fingerprint()
	if err != nil {
		return BenchmarkComparison{}, benchmarkComparisonRefused("baseline manifest: %v", err)
	}
	candidateFingerprint, err := candidate.Manifest.Fingerprint()
	if err != nil {
		return BenchmarkComparison{}, benchmarkComparisonRefused("candidate manifest: %v", err)
	}
	if baselineFingerprint == candidateFingerprint {
		return BenchmarkComparison{}, benchmarkComparisonRefused("solution fingerprints must differ")
	}
	baselineEvidenceFingerprint, err := baseline.Fingerprint()
	if err != nil {
		return BenchmarkComparison{}, benchmarkComparisonRefused("baseline evidence: %v", err)
	}
	candidateEvidenceFingerprint, err := candidate.Fingerprint()
	if err != nil {
		return BenchmarkComparison{}, benchmarkComparisonRefused("candidate evidence: %v", err)
	}
	if len(baseline.Trials) == 0 || len(candidate.Trials) == 0 {
		return BenchmarkComparison{}, benchmarkComparisonRefused("empty trial sets cannot be compared")
	}

	candidateTrials := make(map[benchmarkTrialPair]BenchmarkTrial, len(candidate.Trials))
	for _, trial := range candidate.Trials {
		candidateTrials[benchmarkTrialKey(trial)] = trial
	}
	if len(baseline.Trials) != len(candidateTrials) {
		return BenchmarkComparison{}, benchmarkComparisonRefused("trial sets are not exactly paired")
	}
	observations := make([]reliabilitystats.Observation, 0, len(baseline.Trials))
	comparison := BenchmarkComparison{
		EvidenceTrust:                BenchmarkEvidenceTrustCallerDeclared,
		BaselineMode:                 baseline.Mode,
		CandidateMode:                candidate.Mode,
		BaselineObservedAt:           baseline.ObservedAt,
		CandidateObservedAt:          candidate.ObservedAt,
		BaselineValidUntil:           baseline.ValidUntil,
		CandidateValidUntil:          candidate.ValidUntil,
		BaselineFingerprint:          baselineFingerprint,
		CandidateFingerprint:         candidateFingerprint,
		BaselineEvidenceFingerprint:  baselineEvidenceFingerprint,
		CandidateEvidenceFingerprint: candidateEvidenceFingerprint,
		AsOf:                         now,
		Seed:                         seed,
		Resamples:                    resamples,
		Level:                        level,
		Pairs:                        len(baseline.Trials),
	}
	for _, baselineTrial := range baseline.Trials {
		candidateTrial, ok := candidateTrials[benchmarkTrialKey(baselineTrial)]
		if !ok {
			return BenchmarkComparison{}, benchmarkComparisonRefused("trial sets are not exactly paired")
		}
		if baselineTrial.TrialSpecDigest != candidateTrial.TrialSpecDigest {
			return BenchmarkComparison{}, benchmarkComparisonRefused("paired trials have different trial_spec_digest")
		}
		if baselineTrial.Outcome == BenchmarkOutcomeUnknown || candidateTrial.Outcome == BenchmarkOutcomeUnknown {
			return BenchmarkComparison{}, benchmarkComparisonRefused("unknown terminal outcomes cannot be compared")
		}
		baselineAccepted := acceptedOutcome(baselineTrial)
		candidateAccepted := acceptedOutcome(candidateTrial)
		comparison.BaselineAccepted += baselineAccepted
		comparison.CandidateAccepted += candidateAccepted
		observations = append(observations, reliabilitystats.Observation{
			Task: baselineTrial.TaskID,
			Diff: float64(candidateAccepted - baselineAccepted),
		})
	}
	interval, err := reliabilitystats.PairedBootstrapCI(observations, seed, resamples, level)
	if err != nil {
		return BenchmarkComparison{}, benchmarkComparisonRefused("paired bootstrap: %v", err)
	}
	comparison.Difference = interval
	comparison.Warning = "caller-declared evidence: comparator is conditional arithmetic, not observation or independence attestation; rejected, error, timeout, canceled, and abandoned terminal outcomes count as non-accepted"
	if interval.Tasks < 10 {
		comparison.Warning += "; fewer than 10 task clusters, independence unverified"
	}
	return comparison, nil
}

func benchmarkFresh(evidence BenchmarkEvidence, now time.Time) bool {
	return !now.Before(evidence.ObservedAt) && now.Before(evidence.ValidUntil)
}

func equalBenchmarkComponents(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for name, digest := range left {
		if right[name] != digest {
			return false
		}
	}
	return true
}

type benchmarkTrialPair struct {
	Task      string
	Replicate string
}

func benchmarkTrialKey(trial BenchmarkTrial) benchmarkTrialPair {
	return benchmarkTrialPair{Task: trial.TaskID, Replicate: trial.ReplicateID}
}

func acceptedOutcome(trial BenchmarkTrial) int {
	if trial.Outcome == BenchmarkOutcomeAccepted {
		return 1
	}
	return 0
}

func benchmarkInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidBenchmark}, args...)...)
}

func benchmarkComparisonRefused(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrBenchmarkComparisonRefused}, args...)...)
}
