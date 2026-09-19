package fitnessfunctionregistry

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	recordfingerprint "github.com/geoffmilleraz/agentic-kit/foundations/evidence/record-fingerprint"
	evalgate "github.com/geoffmilleraz/agentic-kit/foundations/quality/eval-gate"
)

const (
	// ProfileVersion is the supported assessment-profile contract version.
	ProfileVersion = 1

	// AssessmentTrustCallerDeclared labels envelopes whose evidence references
	// were supplied by the caller. Hashing binds bytes; it does not authenticate
	// observations, artifacts, or their retention.
	AssessmentTrustCallerDeclared = "caller-declared"

	maxProfileCriteria  = 256
	maxProfileMethods   = 4
	maxArtifactDigests  = 16
	maxProfileTextBytes = 2048
	maxProfileIDBytes   = 128
	maxCriterionWeight  = 1_000_000
)

var (
	// ErrInvalidProfile reports a malformed assessment profile.
	ErrInvalidProfile = errors.New("fitnessfunctionregistry: invalid assessment profile")
	// ErrInvalidObservation reports a malformed or foreign observation.
	ErrInvalidObservation = errors.New("fitnessfunctionregistry: invalid assessment observation")
	// ErrInvalidAssessment reports malformed subject identity or evaluation time.
	ErrInvalidAssessment = errors.New("fitnessfunctionregistry: invalid assessment input")

	profileDigestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
	profileIDRE     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._/-]{0,127}$`)
)

// EvidenceMethod distinguishes how a collector produced an observation. The
// profile, rather than this package, decides which methods are admissible.
type EvidenceMethod string

const (
	// EvidenceMethodDeterministic is a reproducible computed check.
	EvidenceMethodDeterministic EvidenceMethod = "deterministic"
	// EvidenceMethodEmpirical is a measurement from a bounded experiment.
	EvidenceMethodEmpirical EvidenceMethod = "empirical"
	// EvidenceMethodHeuristic is fallible automated evidence.
	EvidenceMethodHeuristic EvidenceMethod = "heuristic"
	// EvidenceMethodHuman is an explicitly recorded human review.
	EvidenceMethodHuman EvidenceMethod = "human"
)

// FindingOutcome is the distinct state of an observation or projected rule.
type FindingOutcome string

const (
	// FindingPass means applicable evidence supports the criterion.
	FindingPass FindingOutcome = "pass"
	// FindingFail means applicable evidence does not support the criterion.
	FindingFail FindingOutcome = "fail"
	// FindingUnknown means the collector could not establish an outcome.
	FindingUnknown FindingOutcome = "unknown"
	// FindingNotApplicable means permitted evidence says the criterion does not apply.
	FindingNotApplicable FindingOutcome = "not-applicable"
)

// CriterionKind distinguishes hard readiness criteria from soft-score criteria.
type CriterionKind string

const (
	// CriterionRequired is a criterion that decides readiness but never contributes
	// to the soft score.
	CriterionRequired CriterionKind = "required"
	// CriterionScored is a criterion that contributes to the soft score only after
	// every required criterion is known and passing.
	CriterionScored CriterionKind = "scored"
)

// RequiredStatus is the aggregate result of required criteria. Unknown means
// readiness is blocked by missing, stale, mismatched, or all-N/A evidence.
type RequiredStatus string

const (
	// RequiredPass means at least one required criterion passed and none failed or
	// remained unknown.
	RequiredPass RequiredStatus = "pass"
	// RequiredFail means a known required criterion failed.
	RequiredFail RequiredStatus = "fail"
	// RequiredUnknown means required evidence is missing, unknown, stale, or all N/A.
	RequiredUnknown RequiredStatus = "unknown"
)

// AssessmentRule states why a trace has its outcome.
type AssessmentRule string

const (
	// RuleObserved records an accepted applicable observation.
	RuleObserved AssessmentRule = "observed"
	// RuleObservedUnknown records a collector-declared unknown observation.
	RuleObservedUnknown AssessmentRule = "observed-unknown"
	// RuleObservedNotApplicable records a permitted N/A observation.
	RuleObservedNotApplicable AssessmentRule = "observed-not-applicable"
	// RuleMissingObservation records no finding for a profile criterion.
	RuleMissingObservation AssessmentRule = "missing-observation"
	// RuleDefinitionMismatch records a finding made against another definition.
	RuleDefinitionMismatch AssessmentRule = "definition-mismatch"
	// RuleSubjectMismatch records a finding made against another subject.
	RuleSubjectMismatch AssessmentRule = "subject-mismatch"
	// RuleMethodDisallowed records a method the profile does not admit.
	RuleMethodDisallowed AssessmentRule = "method-disallowed"
	// RuleObservedInFuture records evidence observed after the supplied evaluation time.
	RuleObservedInFuture AssessmentRule = "observed-in-future"
	// RuleObservationStale records evidence that expired at or before evaluation time.
	RuleObservationStale AssessmentRule = "observation-stale"
	// RuleNotApplicableDisallowed records an N/A claim forbidden by the profile.
	RuleNotApplicableDisallowed AssessmentRule = "not-applicable-disallowed"
)

// Criterion is one domain-owned assessment requirement. Kind selects either a
// required readiness criterion or a scored soft-optimization criterion.
type Criterion struct {
	ID                 string           `json:"id"`
	DefinitionDigest   string           `json:"definition_digest"`
	Kind               CriterionKind    `json:"kind"`
	AllowedMethods     []EvidenceMethod `json:"allowed_methods"`
	AllowNotApplicable bool             `json:"allow_not_applicable"`
	Weight             float64          `json:"weight"`
}

// Profile is a versioned, domain-scoped set of criteria. It does not own a
// catalog, execute collectors, retain artifacts, or authorize an action.
type Profile struct {
	Version  int         `json:"version"`
	ID       string      `json:"id"`
	Domain   string      `json:"domain"`
	Criteria []Criterion `json:"criteria"`
}

// Observation is caller-declared evidence for exactly one profile criterion.
// ArtifactDigests identify consumer-retained material; this package never
// resolves them and makes no authentication claim.
type Observation struct {
	CriterionID      string         `json:"criterion_id"`
	DefinitionDigest string         `json:"definition_digest"`
	SubjectDigest    string         `json:"subject_digest"`
	Outcome          FindingOutcome `json:"outcome"`
	Method           EvidenceMethod `json:"method"`
	Reason           string         `json:"reason"`
	ArtifactDigests  []string       `json:"artifact_digests"`
	// ObservedAt and ValidUntil must be canonical UTC values, so their JSON
	// representation is a four-digit-year RFC3339 timestamp ending in Z.
	ObservedAt time.Time `json:"observed_at"`
	ValidUntil time.Time `json:"valid_until"`
}

// AssessmentTrace is a sorted, reconstructable projection of one profile
// criterion. Unknown rules are explicit rather than silently omitted.
type AssessmentTrace struct {
	CriterionID       string         `json:"criterion_id"`
	DefinitionDigest  string         `json:"definition_digest"`
	Outcome           FindingOutcome `json:"outcome"`
	Rule              AssessmentRule `json:"rule"`
	Reason            string         `json:"reason"`
	ArtifactDigests   []string       `json:"artifact_digests"`
	ObservationDigest *string        `json:"observation_digest"`
}

// Assessment is the pure result of applying one Profile to caller-declared
// observations at a caller-injected time. SoftScore is nil when readiness did
// not pass, a scored criterion is unknown, or no scored criterion applies.
type Assessment struct {
	Trust          string            `json:"trust"`
	ProfileDigest  string            `json:"profile_digest"`
	SubjectDigest  string            `json:"subject_digest"`
	EvidenceDigest string            `json:"evidence_digest"`
	EvaluatedAt    time.Time         `json:"evaluated_at"`
	RequiredStatus RequiredStatus    `json:"required_status"`
	SoftScore      *float64          `json:"soft_score"`
	Trace          []AssessmentTrace `json:"trace"`
}

// Validate verifies the bounded profile contract without mutating it.
func (p Profile) Validate() error {
	if p.Version != ProfileVersion {
		return profileInvalid("version must be %d", ProfileVersion)
	}
	if !validProfileID(p.ID) || !validProfileID(p.Domain) {
		return profileInvalid("id and domain must be bounded ASCII identifiers")
	}
	if len(p.Criteria) == 0 || len(p.Criteria) > maxProfileCriteria {
		return profileInvalid("criteria must contain 1 through %d entries", maxProfileCriteria)
	}
	seen := make(map[string]struct{}, len(p.Criteria))
	required := 0
	for i, criterion := range p.Criteria {
		if err := criterion.validate(); err != nil {
			return profileInvalid("criteria[%d]: %v", i, err)
		}
		if _, duplicate := seen[criterion.ID]; duplicate {
			return profileInvalid("criteria contains duplicate id %q", criterion.ID)
		}
		seen[criterion.ID] = struct{}{}
		if criterion.Kind == CriterionRequired {
			required++
		}
	}
	if required == 0 {
		return profileInvalid("at least one required criterion is needed")
	}
	return nil
}

func (c Criterion) validate() error {
	if !validProfileID(c.ID) {
		return errors.New("id must be a bounded ASCII identifier")
	}
	if !validProfileDigest(c.DefinitionDigest) {
		return errors.New("definition_digest must be a lowercase SHA-256 digest")
	}
	if !c.Kind.Valid() {
		return errors.New("kind must be required or scored")
	}
	if len(c.AllowedMethods) == 0 || len(c.AllowedMethods) > maxProfileMethods {
		return fmt.Errorf("allowed_methods must contain 1 through %d entries", maxProfileMethods)
	}
	methods := make(map[EvidenceMethod]struct{}, len(c.AllowedMethods))
	for _, method := range c.AllowedMethods {
		if !method.Valid() {
			return errors.New("allowed method is invalid")
		}
		if _, duplicate := methods[method]; duplicate {
			return fmt.Errorf("allowed_methods contains duplicate %q", method)
		}
		methods[method] = struct{}{}
	}
	if c.Kind == CriterionRequired {
		if c.Weight != 0 {
			return errors.New("required criterion weight must be zero")
		}
		return nil
	}
	if math.IsNaN(c.Weight) || math.IsInf(c.Weight, 0) || c.Weight <= 0 || c.Weight > maxCriterionWeight {
		return fmt.Errorf("scored criterion weight must be finite and in (0,%d]", maxCriterionWeight)
	}
	return nil
}

// Valid reports whether kind is a recognized assessment criterion kind.
func (kind CriterionKind) Valid() bool {
	return kind == CriterionRequired || kind == CriterionScored
}

// Valid reports whether m is a recognized evidence method.
func (m EvidenceMethod) Valid() bool {
	switch m {
	case EvidenceMethodDeterministic, EvidenceMethodEmpirical, EvidenceMethodHeuristic, EvidenceMethodHuman:
		return true
	default:
		return false
	}
}

// Validate verifies the intrinsic bounded observation shape. Binding it to a
// particular Profile and subject is intentionally handled by AssessProfile as
// an explicit unknown rule rather than a malformed-envelope rejection.
func (o Observation) Validate() error {
	if !validProfileID(o.CriterionID) {
		return observationInvalid("criterion_id must be a bounded ASCII identifier")
	}
	if !validProfileDigest(o.DefinitionDigest) || !validProfileDigest(o.SubjectDigest) {
		return observationInvalid("definition_digest and subject_digest must be lowercase SHA-256 digests")
	}
	if !o.Outcome.Valid() || !o.Method.Valid() {
		return observationInvalid("outcome or method is invalid")
	}
	if !validProfileText(o.Reason) {
		return observationInvalid("reason must be non-empty valid UTF-8 and at most %d bytes", maxProfileTextBytes)
	}
	if len(o.ArtifactDigests) > maxArtifactDigests {
		return observationInvalid("artifact_digests exceeds %d entries", maxArtifactDigests)
	}
	seen := make(map[string]struct{}, len(o.ArtifactDigests))
	for _, digest := range o.ArtifactDigests {
		if !validProfileDigest(digest) {
			return observationInvalid("artifact digest must be a lowercase SHA-256 digest")
		}
		if _, duplicate := seen[digest]; duplicate {
			return observationInvalid("artifact_digests contains duplicate digest")
		}
		seen[digest] = struct{}{}
	}
	if (o.Outcome == FindingPass || o.Outcome == FindingFail || o.Outcome == FindingNotApplicable) && len(o.ArtifactDigests) == 0 {
		return observationInvalid("pass, fail, and not-applicable observations require artifact evidence")
	}
	if err := validateProfileTime("observed_at", o.ObservedAt); err != nil {
		return observationInvalid("%v", err)
	}
	if err := validateProfileTime("valid_until", o.ValidUntil); err != nil {
		return observationInvalid("%v", err)
	}
	if !o.ValidUntil.After(o.ObservedAt) {
		return observationInvalid("valid_until must be after observed_at")
	}
	return nil
}

// Valid reports whether outcome is part of the assessment contract.
func (outcome FindingOutcome) Valid() bool {
	switch outcome {
	case FindingPass, FindingFail, FindingUnknown, FindingNotApplicable:
		return true
	default:
		return false
	}
}

// Fingerprint returns a domain-separated SHA-256 profile identity. It uses
// canonical typed Go JSON with sorted criteria/methods, not JCS.
func (p Profile) Fingerprint() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	body, err := json.Marshal(canonicalProfile{Domain: "agentic-kit/assessment-profile/v1", Version: ProfileVersion, Profile: canonicalizeProfile(p)})
	if err != nil {
		return "", profileInvalid("canonical profile: %v", err)
	}
	return recordfingerprint.HashContent(body), nil
}

// AssessProfile projects profile-bound observations into required readiness and
// an optional soft score. It is pure: now is injected, inputs are never
// mutated, and referenced artifacts stay caller-owned.
func AssessProfile(profile Profile, subjectDigest string, observations []Observation, now time.Time) (Assessment, error) {
	if err := profile.Validate(); err != nil {
		return Assessment{}, err
	}
	if !validProfileDigest(subjectDigest) {
		return Assessment{}, assessmentInvalid("subject_digest must be a lowercase SHA-256 digest")
	}
	if err := validateProfileTime("now", now); err != nil {
		return Assessment{}, assessmentInvalid("%v", err)
	}
	if len(observations) > maxProfileCriteria {
		return Assessment{}, observationInvalid("observations exceeds %d entries", maxProfileCriteria)
	}
	byID := make(map[string]Observation, len(observations))
	criteria := make(map[string]Criterion, len(profile.Criteria))
	for _, criterion := range profile.Criteria {
		criteria[criterion.ID] = criterion
	}
	for i, observation := range observations {
		if err := observation.Validate(); err != nil {
			return Assessment{}, observationInvalid("observations[%d]: %v", i, err)
		}
		if _, known := criteria[observation.CriterionID]; !known {
			return Assessment{}, observationInvalid("observations[%d]: foreign criterion_id %q", i, observation.CriterionID)
		}
		if _, duplicate := byID[observation.CriterionID]; duplicate {
			return Assessment{}, observationInvalid("duplicate observation for criterion_id %q", observation.CriterionID)
		}
		byID[observation.CriterionID] = observation
	}
	profileDigest, err := profile.Fingerprint()
	if err != nil {
		return Assessment{}, err
	}
	evidenceDigest, err := assessmentEvidenceDigest(profileDigest, subjectDigest, observations)
	if err != nil {
		return Assessment{}, err
	}
	canonicalCriteria := sortedProfileCriteria(profile.Criteria)
	trace := make([]AssessmentTrace, 0, len(canonicalCriteria))
	requiredFailure := false
	requiredUnknown := false
	requiredPass := false
	scoredKnown := true
	scoreChecks := make([]evalgate.Check, 0, len(canonicalCriteria))
	for _, criterion := range canonicalCriteria {
		entry := assessCriterion(criterion, byID[criterion.ID], byIDHas(byID, criterion.ID), subjectDigest, now)
		trace = append(trace, entry)
		if criterion.Kind == CriterionRequired {
			switch entry.Outcome {
			case FindingPass:
				requiredPass = true
			case FindingFail:
				requiredFailure = true
			case FindingUnknown:
				requiredUnknown = true
			}
			continue
		}
		switch entry.Outcome {
		case FindingPass:
			scoreChecks = append(scoreChecks, evalgate.Check{Name: criterion.ID, Passed: true, Weight: criterion.Weight})
		case FindingFail:
			scoreChecks = append(scoreChecks, evalgate.Check{Name: criterion.ID, Passed: false, Weight: criterion.Weight})
		case FindingUnknown:
			scoredKnown = false
		}
	}
	result := Assessment{
		Trust:          AssessmentTrustCallerDeclared,
		ProfileDigest:  profileDigest,
		SubjectDigest:  subjectDigest,
		EvidenceDigest: evidenceDigest,
		EvaluatedAt:    canonicalProfileTime(now),
		Trace:          trace,
	}
	switch {
	case requiredFailure:
		result.RequiredStatus = RequiredFail
	case requiredUnknown || !requiredPass:
		result.RequiredStatus = RequiredUnknown
	default:
		result.RequiredStatus = RequiredPass
	}
	if result.RequiredStatus == RequiredPass && scoredKnown && len(scoreChecks) > 0 {
		execution := Execute(FitnessFunction{Name: profile.ID, Checks: scoreChecks}, "assessment-profile", result.EvaluatedAt)
		score := execution.Score
		result.SoftScore = &score
	}
	return result, nil
}

func byIDHas(values map[string]Observation, id string) bool {
	_, ok := values[id]
	return ok
}

func assessCriterion(criterion Criterion, observation Observation, present bool, subjectDigest string, now time.Time) AssessmentTrace {
	trace := AssessmentTrace{
		CriterionID: criterion.ID, DefinitionDigest: criterion.DefinitionDigest,
		Outcome: FindingUnknown, ArtifactDigests: make([]string, 0),
	}
	if !present {
		trace.Rule, trace.Reason = RuleMissingObservation, "no observation supplied"
		return trace
	}
	observationDigest := mustObservationDigest(observation)
	trace.ObservationDigest = &observationDigest
	trace.ArtifactDigests = append(make([]string, 0, len(observation.ArtifactDigests)), observation.ArtifactDigests...)
	sort.Strings(trace.ArtifactDigests)
	if observation.DefinitionDigest != criterion.DefinitionDigest {
		trace.Rule, trace.Reason = RuleDefinitionMismatch, "observation definition digest does not match criterion"
		return trace
	}
	if observation.SubjectDigest != subjectDigest {
		trace.Rule, trace.Reason = RuleSubjectMismatch, "observation subject digest does not match assessment subject"
		return trace
	}
	if !criterionAllowsMethod(criterion, observation.Method) {
		trace.Rule, trace.Reason = RuleMethodDisallowed, "observation method is not allowed by criterion"
		return trace
	}
	if observation.ObservedAt.After(now) {
		trace.Rule, trace.Reason = RuleObservedInFuture, "observation was recorded after evaluation time"
		return trace
	}
	if !now.Before(observation.ValidUntil) {
		trace.Rule, trace.Reason = RuleObservationStale, "observation validity ended at or before evaluation time"
		return trace
	}
	if observation.Outcome == FindingNotApplicable {
		if !criterion.AllowNotApplicable {
			trace.Rule, trace.Reason = RuleNotApplicableDisallowed, "criterion does not permit not-applicable evidence"
			return trace
		}
		trace.Outcome, trace.Rule, trace.Reason = FindingNotApplicable, RuleObservedNotApplicable, observation.Reason
		return trace
	}
	if observation.Outcome == FindingUnknown {
		trace.Rule, trace.Reason = RuleObservedUnknown, observation.Reason
		return trace
	}
	trace.Outcome, trace.Rule, trace.Reason = observation.Outcome, RuleObserved, observation.Reason
	return trace
}

func criterionAllowsMethod(criterion Criterion, method EvidenceMethod) bool {
	for _, allowed := range criterion.AllowedMethods {
		if allowed == method {
			return true
		}
	}
	return false
}

func assessmentEvidenceDigest(profileDigest, subjectDigest string, observations []Observation) (string, error) {
	canonical := canonicalAssessmentEvidence{
		Domain:        "agentic-kit/assessment-evidence/v1",
		Version:       ProfileVersion,
		ProfileDigest: profileDigest,
		SubjectDigest: subjectDigest,
		Observations:  canonicalizeObservations(observations),
	}
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", assessmentInvalid("canonical evidence: %v", err)
	}
	return recordfingerprint.HashContent(body), nil
}

func mustObservationDigest(observation Observation) string {
	body, err := json.Marshal(canonicalObservation{Domain: "agentic-kit/assessment-observation/v1", Version: ProfileVersion, Observation: canonicalizeObservation(observation)})
	if err != nil {
		panic("fitnessfunctionregistry: validated observation cannot canonicalize: " + err.Error())
	}
	return recordfingerprint.HashContent(body)
}

type canonicalProfile struct {
	Domain  string           `json:"domain"`
	Version int              `json:"version"`
	Profile profileCanonical `json:"profile"`
}

type profileCanonical struct {
	Version  int                  `json:"version"`
	ID       string               `json:"id"`
	Domain   string               `json:"domain"`
	Criteria []criterionCanonical `json:"criteria"`
}

type criterionCanonical struct {
	ID                 string           `json:"id"`
	DefinitionDigest   string           `json:"definition_digest"`
	Kind               CriterionKind    `json:"kind"`
	AllowedMethods     []EvidenceMethod `json:"allowed_methods"`
	AllowNotApplicable bool             `json:"allow_not_applicable"`
	Weight             float64          `json:"weight"`
}

type canonicalAssessmentEvidence struct {
	Domain        string                 `json:"domain"`
	Version       int                    `json:"version"`
	ProfileDigest string                 `json:"profile_digest"`
	SubjectDigest string                 `json:"subject_digest"`
	Observations  []observationCanonical `json:"observations"`
}

type canonicalObservation struct {
	Domain      string               `json:"domain"`
	Version     int                  `json:"version"`
	Observation observationCanonical `json:"observation"`
}

type observationCanonical struct {
	CriterionID      string         `json:"criterion_id"`
	DefinitionDigest string         `json:"definition_digest"`
	SubjectDigest    string         `json:"subject_digest"`
	Outcome          FindingOutcome `json:"outcome"`
	Method           EvidenceMethod `json:"method"`
	Reason           string         `json:"reason"`
	ArtifactDigests  []string       `json:"artifact_digests"`
	ObservedAt       string         `json:"observed_at"`
	ValidUntil       string         `json:"valid_until"`
}

func canonicalizeProfile(profile Profile) profileCanonical {
	criteria := make([]criterionCanonical, 0, len(profile.Criteria))
	for _, criterion := range profile.Criteria {
		methods := append([]EvidenceMethod(nil), criterion.AllowedMethods...)
		sort.Slice(methods, func(i, j int) bool { return methods[i] < methods[j] })
		criteria = append(criteria, criterionCanonical{
			ID: criterion.ID, DefinitionDigest: criterion.DefinitionDigest, Kind: criterion.Kind,
			AllowedMethods:     methods,
			AllowNotApplicable: criterion.AllowNotApplicable, Weight: criterion.Weight,
		})
	}
	sort.Slice(criteria, func(i, j int) bool { return criteria[i].ID < criteria[j].ID })
	return profileCanonical{Version: profile.Version, ID: profile.ID, Domain: profile.Domain, Criteria: criteria}
}

func canonicalizeObservations(observations []Observation) []observationCanonical {
	canonical := make([]observationCanonical, 0, len(observations))
	for _, observation := range observations {
		canonical = append(canonical, canonicalizeObservation(observation))
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].CriterionID < canonical[j].CriterionID })
	return canonical
}

func canonicalizeObservation(observation Observation) observationCanonical {
	artifacts := append(make([]string, 0, len(observation.ArtifactDigests)), observation.ArtifactDigests...)
	sort.Strings(artifacts)
	return observationCanonical{
		CriterionID: observation.CriterionID, DefinitionDigest: observation.DefinitionDigest,
		SubjectDigest: observation.SubjectDigest, Outcome: observation.Outcome, Method: observation.Method,
		Reason: observation.Reason, ArtifactDigests: artifacts,
		ObservedAt: canonicalProfileTime(observation.ObservedAt).Format(time.RFC3339Nano), ValidUntil: canonicalProfileTime(observation.ValidUntil).Format(time.RFC3339Nano),
	}
}

func sortedProfileCriteria(criteria []Criterion) []Criterion {
	sorted := append([]Criterion(nil), criteria...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	return sorted
}

func validProfileID(value string) bool {
	return len(value) <= maxProfileIDBytes && profileIDRE.MatchString(value)
}

func validProfileDigest(value string) bool { return profileDigestRE.MatchString(value) }

func validProfileText(value string) bool {
	return len(value) <= maxProfileTextBytes && utf8.ValidString(value) && strings.TrimSpace(value) != ""
}

func validateProfileTime(name string, value time.Time) error {
	if value.IsZero() || value.Location() != time.UTC || value != value.Round(0) || value.Year() < 0 || value.Year() > 9999 {
		return fmt.Errorf("%s must be a non-zero canonical UTC time representable in JSON", name)
	}
	_, offset := value.Zone()
	if offset != 0 || offset%60 != 0 {
		return fmt.Errorf("%s must not carry an offset or offset seconds", name)
	}
	return nil
}

func canonicalProfileTime(value time.Time) time.Time { return value.UTC().Round(0) }

func profileInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidProfile}, args...)...)
}

func observationInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidObservation}, args...)...)
}

func assessmentInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidAssessment}, args...)...)
}
