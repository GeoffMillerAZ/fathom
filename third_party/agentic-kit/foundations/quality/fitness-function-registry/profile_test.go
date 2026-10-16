package fitnessfunctionregistry_test

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

	recordfingerprint "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/record-fingerprint"
	ffr "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
)

var profileNow = time.Date(2026, time.September, 16, 12, 0, 0, 0, time.UTC)

func TestProfileValidateBoundaries(t *testing.T) {
	base := testProfile()
	tests := []struct {
		name string
		edit func(*ffr.Profile)
	}{
		{"version", func(p *ffr.Profile) { p.Version = 2 }},
		{"missing required", func(p *ffr.Profile) { p.Criteria[0].Kind = ffr.CriterionScored; p.Criteria[0].Weight = 1 }},
		{"required weight", func(p *ffr.Profile) { p.Criteria[0].Weight = 1 }},
		{"scored zero", func(p *ffr.Profile) { p.Criteria[1].Weight = 0 }},
		{"nonfinite weight", func(p *ffr.Profile) { p.Criteria[1].Weight = math.Inf(1) }},
		{"duplicate criterion", func(p *ffr.Profile) { p.Criteria = append(p.Criteria, p.Criteria[0]) }},
		{"duplicate method", func(p *ffr.Profile) {
			p.Criteria[0].AllowedMethods = append(p.Criteria[0].AllowedMethods, ffr.EvidenceMethodDeterministic)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cloneProfile(base)
			tc.edit(&got)
			if err := got.Validate(); !errors.Is(err, ffr.ErrInvalidProfile) {
				t.Fatalf("Validate error = %v, want ErrInvalidProfile", err)
			}
		})
	}
}

func TestProfileErrorsDoNotEchoUnboundedInput(t *testing.T) {
	large := strings.Repeat("untrusted", 8192)
	for _, field := range []string{"kind", "allowed-method", "outcome", "method"} {
		t.Run(field, func(t *testing.T) {
			p := testProfile()
			o := testObservation(p.Criteria[0], digest("subject"), ffr.FindingPass, ffr.EvidenceMethodDeterministic)
			var err error
			switch field {
			case "kind":
				p.Criteria[0].Kind = ffr.CriterionKind(large)
				err = p.Validate()
			case "allowed-method":
				p.Criteria[0].AllowedMethods[0] = ffr.EvidenceMethod(large)
				err = p.Validate()
			case "outcome":
				o.Outcome = ffr.FindingOutcome(large)
				err = o.Validate()
			case "method":
				o.Method = ffr.EvidenceMethod(large)
				err = o.Validate()
			}
			if err == nil || len(err.Error()) > 512 || strings.Contains(err.Error(), large) {
				t.Fatal("invalid input did not produce a bounded diagnostic")
			}
		})
	}
}

func TestAssessProfileRequiredAndScoreRules(t *testing.T) {
	profile := testProfile()
	subject := digest("subject")
	tests := []struct {
		name         string
		observations []ffr.Observation
		wantStatus   ffr.RequiredStatus
		wantScore    *float64
		wantOutcomes []ffr.FindingOutcome
	}{
		{
			name: "all pass",
			observations: []ffr.Observation{
				testObservation(profile.Criteria[0], subject, ffr.FindingPass, ffr.EvidenceMethodDeterministic),
				testObservation(profile.Criteria[1], subject, ffr.FindingPass, ffr.EvidenceMethodEmpirical),
			},
			wantStatus: ffr.RequiredPass, wantScore: floatPtr(1), wantOutcomes: []ffr.FindingOutcome{ffr.FindingPass, ffr.FindingPass},
		},
		{
			name: "required fail wins unknown",
			observations: []ffr.Observation{
				testObservation(profile.Criteria[0], subject, ffr.FindingFail, ffr.EvidenceMethodDeterministic),
				testObservation(profile.Criteria[1], subject, ffr.FindingUnknown, ffr.EvidenceMethodEmpirical),
			},
			wantStatus: ffr.RequiredFail, wantOutcomes: []ffr.FindingOutcome{ffr.FindingUnknown, ffr.FindingFail},
		},
		{
			name: "missing required is unknown",
			observations: []ffr.Observation{
				testObservation(profile.Criteria[1], subject, ffr.FindingPass, ffr.EvidenceMethodEmpirical),
			},
			wantStatus: ffr.RequiredUnknown, wantOutcomes: []ffr.FindingOutcome{ffr.FindingPass, ffr.FindingUnknown},
		},
		{
			name: "permitted all required na cannot pass",
			observations: []ffr.Observation{
				testObservation(profile.Criteria[0], subject, ffr.FindingNotApplicable, ffr.EvidenceMethodDeterministic),
				testObservation(profile.Criteria[1], subject, ffr.FindingPass, ffr.EvidenceMethodEmpirical),
			},
			wantStatus: ffr.RequiredUnknown, wantOutcomes: []ffr.FindingOutcome{ffr.FindingPass, ffr.FindingNotApplicable},
		},
		{
			name: "scored na excluded visibly",
			observations: []ffr.Observation{
				testObservation(profile.Criteria[0], subject, ffr.FindingPass, ffr.EvidenceMethodDeterministic),
				testObservation(profile.Criteria[1], subject, ffr.FindingNotApplicable, ffr.EvidenceMethodEmpirical),
			},
			wantStatus: ffr.RequiredPass, wantOutcomes: []ffr.FindingOutcome{ffr.FindingNotApplicable, ffr.FindingPass},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ffr.AssessProfile(profile, subject, tc.observations, profileNow)
			if err != nil {
				t.Fatal(err)
			}
			if got.RequiredStatus != tc.wantStatus {
				t.Fatalf("RequiredStatus = %q, want %q", got.RequiredStatus, tc.wantStatus)
			}
			if !sameFloatPtr(got.SoftScore, tc.wantScore) {
				t.Fatalf("SoftScore = %v, want %v", got.SoftScore, tc.wantScore)
			}
			outcomes := make([]ffr.FindingOutcome, len(got.Trace))
			for i, trace := range got.Trace {
				outcomes[i] = trace.Outcome
			}
			if !reflect.DeepEqual(outcomes, tc.wantOutcomes) {
				t.Fatalf("trace outcomes = %v, want %v", outcomes, tc.wantOutcomes)
			}
		})
	}
}

func TestAssessProfileUnknownRulesAndInputErrors(t *testing.T) {
	profile := testProfile()
	profile.Criteria[0].AllowNotApplicable = false
	subject := digest("subject")
	base := testObservation(profile.Criteria[0], subject, ffr.FindingPass, ffr.EvidenceMethodDeterministic)
	tests := []struct {
		name string
		edit func(*ffr.Observation)
		rule ffr.AssessmentRule
	}{
		{"definition mismatch", func(o *ffr.Observation) { o.DefinitionDigest = digest("other") }, ffr.RuleDefinitionMismatch},
		{"subject mismatch", func(o *ffr.Observation) { o.SubjectDigest = digest("other-subject") }, ffr.RuleSubjectMismatch},
		{"method disallowed", func(o *ffr.Observation) { o.Method = ffr.EvidenceMethodHuman }, ffr.RuleMethodDisallowed},
		{"future", func(o *ffr.Observation) {
			o.ObservedAt = profileNow.Add(time.Second)
			o.ValidUntil = profileNow.Add(time.Hour)
		}, ffr.RuleObservedInFuture},
		{"stale", func(o *ffr.Observation) { o.ObservedAt = profileNow.Add(-time.Hour); o.ValidUntil = profileNow }, ffr.RuleObservationStale},
		{"na disallowed", func(o *ffr.Observation) {
			o.Outcome = ffr.FindingNotApplicable
			o.ArtifactDigests = []string{digest("na")}
		}, ffr.RuleNotApplicableDisallowed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			observation := base
			tc.edit(&observation)
			got, err := ffr.AssessProfile(profile, subject, []ffr.Observation{observation}, profileNow)
			if err != nil {
				t.Fatal(err)
			}
			trace := traceFor(t, got.Trace, profile.Criteria[0].ID)
			if trace.Outcome != ffr.FindingUnknown || trace.Rule != tc.rule {
				t.Fatalf("trace = %+v, want unknown/%q", trace, tc.rule)
			}
		})
	}
	foreign := base
	foreign.CriterionID = "foreign"
	if _, err := ffr.AssessProfile(profile, subject, []ffr.Observation{foreign}, profileNow); !errors.Is(err, ffr.ErrInvalidObservation) {
		t.Fatalf("foreign criterion error = %v", err)
	}
	if _, err := ffr.AssessProfile(profile, subject, []ffr.Observation{base, base}, profileNow); !errors.Is(err, ffr.ErrInvalidObservation) {
		t.Fatalf("duplicate observation error = %v", err)
	}
	badTime := base
	badTime.ObservedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	badTime.ValidUntil = time.Date(10000, 1, 1, 1, 0, 0, 0, time.UTC)
	if err := badTime.Validate(); !errors.Is(err, ffr.ErrInvalidObservation) {
		t.Fatalf("JSON-lossy time error = %v", err)
	}
	offsetTime := base
	offsetTime.ObservedAt = time.Date(2026, 9, 16, 11, 0, 0, 0, time.FixedZone("offset", 3600))
	offsetTime.ValidUntil = time.Date(2026, 9, 16, 13, 0, 0, 0, time.FixedZone("offset", 3600))
	if err := offsetTime.Validate(); !errors.Is(err, ffr.ErrInvalidObservation) {
		t.Fatalf("offset time error = %v", err)
	}
	noEvidence := base
	noEvidence.Outcome = ffr.FindingNotApplicable
	noEvidence.ArtifactDigests = nil
	if err := noEvidence.Validate(); !errors.Is(err, ffr.ErrInvalidObservation) {
		t.Fatalf("not-applicable without evidence error = %v", err)
	}
	for _, outcome := range []ffr.FindingOutcome{ffr.FindingPass, ffr.FindingFail} {
		noEvidence.Outcome = outcome
		if err := noEvidence.Validate(); !errors.Is(err, ffr.ErrInvalidObservation) {
			t.Fatalf("%s without evidence error = %v", outcome, err)
		}
	}
	oversized := make([]ffr.Observation, 257)
	if _, err := ffr.AssessProfile(profile, subject, oversized, profileNow); !errors.Is(err, ffr.ErrInvalidObservation) {
		t.Fatalf("oversized observations error = %v", err)
	}
}

func TestAssessProfileCanonicalIdentityAndImmutability(t *testing.T) {
	profile := testProfile()
	subject := digest("subject")
	observations := []ffr.Observation{
		testObservation(profile.Criteria[0], subject, ffr.FindingPass, ffr.EvidenceMethodDeterministic),
		testObservation(profile.Criteria[1], subject, ffr.FindingFail, ffr.EvidenceMethodEmpirical),
	}
	observations[0].ArtifactDigests = []string{digest("a"), digest("b")}
	beforeProfile, beforeObservations := cloneProfile(profile), cloneObservations(observations)
	first, err := ffr.AssessProfile(profile, subject, observations, profileNow)
	if err != nil {
		t.Fatal(err)
	}
	permutedProfile := cloneProfile(profile)
	permutedProfile.Criteria = []ffr.Criterion{permutedProfile.Criteria[1], permutedProfile.Criteria[0]}
	permutedProfile.Criteria[0].AllowedMethods = []ffr.EvidenceMethod{ffr.EvidenceMethodHuman, ffr.EvidenceMethodEmpirical}
	permutedObservations := []ffr.Observation{observations[1], observations[0]}
	permutedObservations[1].ArtifactDigests = []string{digest("b"), digest("a")}
	second, err := ffr.AssessProfile(permutedProfile, subject, permutedObservations, profileNow)
	if err != nil {
		t.Fatal(err)
	}
	if first.ProfileDigest != second.ProfileDigest || first.EvidenceDigest != second.EvidenceDigest || !reflect.DeepEqual(first.Trace, second.Trace) {
		t.Fatalf("permutation changed identity: first=%+v second=%+v", first, second)
	}
	if !reflect.DeepEqual(profile, beforeProfile) || !reflect.DeepEqual(observations, beforeObservations) {
		t.Fatal("AssessProfile mutated caller input")
	}
	tampered := cloneObservations(observations)
	tampered[0].ArtifactDigests[0] = digest("changed")
	changed, err := ffr.AssessProfile(profile, subject, tampered, profileNow)
	if err != nil {
		t.Fatal(err)
	}
	if changed.EvidenceDigest == first.EvidenceDigest {
		t.Fatal("artifact change did not change evidence digest")
	}
}

func TestAssessmentJSONProjectsToCUE(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue not installed")
	}
	profile := testProfile()
	subject := digest("subject")
	assessment, err := ffr.AssessProfile(profile, subject, []ffr.Observation{
		testObservation(profile.Criteria[0], subject, ffr.FindingPass, ffr.EvidenceMethodDeterministic),
		testObservation(profile.Criteria[1], subject, ffr.FindingPass, ffr.EvidenceMethodEmpirical),
	}, profileNow)
	if err != nil {
		t.Fatal(err)
	}
	for _, wire := range []struct {
		definition string
		value      any
	}{{"#Profile", profile}, {"#Assessment", assessment}} {
		body, err := json.Marshal(wire.value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "wire.json")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("cue", "vet", "-d", wire.definition, "profile.cue", path)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s did not project to CUE: %v\n%s", wire.definition, err, output)
		}
	}
	missingScored, err := ffr.AssessProfile(profile, subject, []ffr.Observation{
		testObservation(profile.Criteria[0], subject, ffr.FindingPass, ffr.EvidenceMethodDeterministic),
	}, profileNow)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(missingScored)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "missing-trace-artifacts.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cue", "vet", "-d", "#Assessment", "profile.cue", path).CombinedOutput(); err != nil {
		t.Fatalf("empty output trace artifacts did not project to CUE: %v\n%s", err, output)
	}
	unknown := testObservation(profile.Criteria[0], subject, ffr.FindingUnknown, ffr.EvidenceMethodDeterministic)
	unknown.ArtifactDigests = nil
	body, err = json.Marshal(unknown)
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(t.TempDir(), "unknown-observation.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cue", "vet", "-d", "#Observation", "profile.cue", path).CombinedOutput(); err != nil {
		t.Fatalf("nil unknown artifacts did not project to CUE: %v\n%s", err, output)
	}
	for _, outcome := range []ffr.FindingOutcome{ffr.FindingPass, ffr.FindingFail, ffr.FindingNotApplicable} {
		terminal := unknown
		terminal.Outcome = outcome
		if err := terminal.Validate(); err == nil {
			t.Fatalf("%s without artifacts passed Go", outcome)
		}
		body, err := json.Marshal(terminal)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command("cue", "vet", "-d", "#Observation", "profile.cue", path).CombinedOutput(); err == nil {
			t.Fatalf("%s without artifacts passed CUE: %s", outcome, output)
		}
	}
	longReason := unknown
	longReason.Reason = strings.Repeat("x", 2049)
	body, err = json.Marshal(longReason)
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(t.TempDir(), "oversized-reason.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cue", "vet", "-d", "#Observation", "profile.cue", path).CombinedOutput(); err == nil {
		t.Fatalf("over-byte-limit reason unexpectedly passed CUE: %s", output)
	}
	offset := testObservation(profile.Criteria[0], subject, ffr.FindingPass, ffr.EvidenceMethodDeterministic)
	offset.ObservedAt = time.Date(2026, 9, 16, 11, 0, 0, 0, time.FixedZone("offset", 3600))
	offset.ValidUntil = time.Date(2026, 9, 16, 13, 0, 0, 0, time.FixedZone("offset", 3600))
	body, err = json.Marshal(offset)
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(t.TempDir(), "offset-observation.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cue", "vet", "-d", "#Observation", "profile.cue", path).CombinedOutput(); err == nil {
		t.Fatalf("offset observation unexpectedly passed UTC CUE contract: %s", output)
	}
}

func testProfile() ffr.Profile {
	return ffr.Profile{
		Version: ffr.ProfileVersion, ID: "redteam-effect", Domain: "security",
		Criteria: []ffr.Criterion{
			{ID: "no-unauthorized-effect", DefinitionDigest: digest("no-effect"), Kind: ffr.CriterionRequired, AllowedMethods: []ffr.EvidenceMethod{ffr.EvidenceMethodDeterministic}, AllowNotApplicable: true},
			{ID: "benign-utility", DefinitionDigest: digest("utility"), Kind: ffr.CriterionScored, AllowedMethods: []ffr.EvidenceMethod{ffr.EvidenceMethodEmpirical, ffr.EvidenceMethodHuman}, AllowNotApplicable: true, Weight: 2},
		},
	}
}

func testObservation(criterion ffr.Criterion, subject string, outcome ffr.FindingOutcome, method ffr.EvidenceMethod) ffr.Observation {
	return ffr.Observation{
		CriterionID: criterion.ID, DefinitionDigest: criterion.DefinitionDigest, SubjectDigest: subject,
		Outcome: outcome, Method: method, Reason: "captured outcome", ArtifactDigests: []string{digest(criterion.ID + "artifact")},
		ObservedAt: profileNow.Add(-time.Hour), ValidUntil: profileNow.Add(time.Hour),
	}
}

func digest(value string) string      { return recordfingerprint.HashContent([]byte(value)) }
func floatPtr(value float64) *float64 { return &value }
func sameFloatPtr(got, want *float64) bool {
	return (got == nil && want == nil) || (got != nil && want != nil && *got == *want)
}

func cloneProfile(value ffr.Profile) ffr.Profile {
	copy := value
	copy.Criteria = append([]ffr.Criterion(nil), value.Criteria...)
	for i := range copy.Criteria {
		copy.Criteria[i].AllowedMethods = append([]ffr.EvidenceMethod(nil), value.Criteria[i].AllowedMethods...)
	}
	return copy
}

func cloneObservations(values []ffr.Observation) []ffr.Observation {
	copy := append([]ffr.Observation(nil), values...)
	for i := range copy {
		copy[i].ArtifactDigests = append([]string(nil), values[i].ArtifactDigests...)
	}
	return copy
}

func traceFor(t *testing.T, traces []ffr.AssessmentTrace, id string) ffr.AssessmentTrace {
	t.Helper()
	for _, trace := range traces {
		if trace.CriterionID == id {
			return trace
		}
	}
	t.Fatalf("trace for %q not found", id)
	return ffr.AssessmentTrace{}
}
