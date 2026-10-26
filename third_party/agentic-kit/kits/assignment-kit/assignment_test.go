package assignmentkit

import (
	"bytes"
	"encoding/json"
	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const d1 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const d2 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const d3 = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

var now = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func base() (Assignment, Scope, Input) {
	a := Assignment{Version: 1, ID: "task", Title: "Check work", PolicyDigest: d1, SubjectDigest: d2, Goals: []Goal{{ID: "main", Title: "Evidence passes", Kind: "required", Profile: fitness.Profile{Version: 1, ID: "profile", Domain: "domain", Criteria: []fitness.Criterion{{ID: "check", DefinitionDigest: d3, Kind: fitness.CriterionRequired, AllowedMethods: []fitness.EvidenceMethod{fitness.EvidenceMethodDeterministic, fitness.EvidenceMethodHeuristic}}}}}}, Budgets: []Budget{{"calls", "unit", 2}}, Deadline: now.Add(time.Hour)}
	fp, e := Fingerprint(a)
	if e != nil {
		panic(e)
	}
	s := Scope{fp, d1, []string{"main"}}
	in := Input{AssignmentDigest: fp, PolicyDigest: d1, Goals: []GoalInput{{GoalID: "main", Observations: []fitness.Observation{{CriterionID: "check", DefinitionDigest: d3, SubjectDigest: d2, Outcome: fitness.FindingPass, Method: fitness.EvidenceMethodDeterministic, Reason: "verified", ArtifactDigests: []string{d1}, ObservedAt: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)}}}}, Usage: []Usage{{"calls", 0}}, Progress: &Progress{}}
	return a, s, in
}
func eval(t *testing.T, a Assignment, s Scope, in Input, at time.Time) Result {
	t.Helper()
	r, e := Evaluate(a, s, in, at)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestAcceptanceAndPins(t *testing.T) {
	a, s, in := base()
	if got := eval(t, a, s, in, now).Status; got != "complete" {
		t.Fatal(got)
	}
	a.Goals = append(a.Goals, Goal{ID: "omitted", Title: "Must pass", Kind: "required", Profile: a.Goals[0].Profile})
	fp, _ := Fingerprint(a)
	s.AssignmentDigest = fp
	in.AssignmentDigest = fp
	if _, e := Evaluate(a, s, in, now); e == nil {
		t.Fatal("planted omission accepted")
	}
	a, s, in = base()
	in.Goals = append(in.Goals, in.Goals[0])
	if _, e := Evaluate(a, s, in, now); e == nil {
		t.Fatal("duplicate input accepted")
	}
	a, s, in = base()
	in.Goals[0].GoalID = "foreign"
	if _, e := Evaluate(a, s, in, now); e == nil {
		t.Fatal("foreign goal accepted")
	}
	a, s, in = base()
	a.Title = "changed"
	if _, e := Evaluate(a, s, in, now); e == nil {
		t.Fatal("changed definition accepted")
	}
}
func TestEvidenceRules(t *testing.T) {
	cases := []struct {
		name  string
		alter func(*Input)
	}{{"missing", func(x *Input) { x.Goals = nil }}, {"stale", func(x *Input) { x.Goals[0].Observations[0].ValidUntil = now }}, {"future", func(x *Input) {
		o := &x.Goals[0].Observations[0]
		o.ObservedAt = now.Add(time.Second)
		o.ValidUntil = now.Add(time.Minute)
	}}, {"subject", func(x *Input) { x.Goals[0].Observations[0].SubjectDigest = d1 }}, {"definition", func(x *Input) { x.Goals[0].Observations[0].DefinitionDigest = d1 }}, {"method", func(x *Input) { x.Goals[0].Observations[0].Method = fitness.EvidenceMethodHuman }}, {"na", func(x *Input) { x.Goals[0].Observations[0].Outcome = fitness.FindingNotApplicable }}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, s, in := base()
			tc.alter(&in)
			r := eval(t, a, s, in, now)
			if r.Status == "complete" {
				t.Fatal("false completion")
			}
		})
	}
}
func TestDependenciesStretchAndScored(t *testing.T) {
	a, s, in := base()
	g := a.Goals[0]
	g.ID = "stretch"
	g.Kind = "stretch"
	a.Goals = append(a.Goals, g)
	fp, _ := Fingerprint(a)
	s.AssignmentDigest = fp
	in.AssignmentDigest = fp
	if got := eval(t, a, s, in, now).Status; got != "complete" {
		t.Fatal(got)
	}
	a, s, in = base()
	g = a.Goals[0]
	g.ID = "dep"
	a.Goals = append(a.Goals, g)
	a.Goals[0].DependsOn = []string{"dep"}
	fp, _ = Fingerprint(a)
	s = Scope{fp, d1, []string{"main", "dep"}}
	in.AssignmentDigest = fp
	if got := eval(t, a, s, in, now).Status; got == "complete" {
		t.Fatal("failed dependency completed")
	}
	a, s, in = base()
	a.Goals[0].Profile.Criteria = append(a.Goals[0].Profile.Criteria, fitness.Criterion{ID: "soft", DefinitionDigest: d1, Kind: fitness.CriterionScored, AllowedMethods: []fitness.EvidenceMethod{fitness.EvidenceMethodDeterministic}, Weight: 1})
	in.Goals[0].Observations[0].Outcome = fitness.FindingFail
	fp, _ = Fingerprint(a)
	s.AssignmentDigest = fp
	in.AssignmentDigest = fp
	if got := eval(t, a, s, in, now).Status; got == "complete" {
		t.Fatal("scored bypass")
	}
}
func TestConfidenceBindings(t *testing.T) {
	a, s, in := base()
	a.Goals[0].Confidence = []ConfidenceBar{{"check", 0.8, d1, d2}}
	o := &in.Goals[0].Observations[0]
	o.Method = fitness.EvidenceMethodHeuristic
	od, _ := ObservationFingerprint(*o)
	fp, _ := Fingerprint(a)
	s.AssignmentDigest = fp
	in.AssignmentDigest = fp
	if got := eval(t, a, s, in, now).Status; got == "complete" {
		t.Fatal("missing reading")
	}
	in.Goals[0].Readings = []ConfidenceReading{{"check", od, d1, d2, 0.8}}
	if got := eval(t, a, s, in, now).Status; got != "complete" {
		t.Fatal(got)
	}
	for _, mutate := range []func(*ConfidenceReading){func(r *ConfidenceReading) { r.Value = 0.79 }, func(r *ConfidenceReading) { r.BasisDigest = d3 }, func(r *ConfidenceReading) { r.SiteDigest = d3 }, func(r *ConfidenceReading) { r.ObservationDigest = d3 }} {
		r := in.Goals[0].Readings[0]
		mutate(&r)
		in.Goals[0].Readings[0] = r
		if got := eval(t, a, s, in, now).Status; got == "complete" {
			t.Fatal("invalid confidence completed")
		}
	}
	in.Goals[0].Readings[0].Value = math.NaN()
	if _, e := Evaluate(a, s, in, now); e == nil {
		t.Fatal("nonfinite reading accepted")
	}
}
func TestConcernsBudgetsAndRules(t *testing.T) {
	a, s, in := base()
	a.Concerns = []Concern{{"risk", "Unresolved issue", "blocking"}, {"warning", "Advisory issue", "advisory"}}
	a.Rules = []Rule{{"escalate", "attempts", 1, "escalate", "review", "ask"}, {"pivot", "attempts", 1, "pivot", "plan", "change"}, {"stop", "attempts", 1, "stop", "halt", "end"}}
	fp, _ := Fingerprint(a)
	s.AssignmentDigest = fp
	in.AssignmentDigest = fp
	r := eval(t, a, s, in, now)
	if r.Status != "blocked" || len(r.Concerns) != 2 {
		t.Fatal(r.Status)
	}
	in.Concerns = []ConcernInput{{ID: "risk", State: "resolved", Reason: "fixed", ObservedAt: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)}}
	r = eval(t, a, s, in, now)
	if r.Status != "blocked" {
		t.Fatal("unsubstantiated resolution")
	}
	in.Concerns[0].ArtifactDigests = []string{d3}
	r = eval(t, a, s, in, now)
	if r.Status != "complete" {
		t.Fatal(r.Status)
	}
	in.Progress.Attempts = 1
	r = eval(t, a, s, in, now)
	if r.Status != "stopped" || len(r.MatchedRules) != 3 {
		t.Fatal(r)
	}
	in.Progress.Attempts = 0
	in.Usage[0].Spent = 2
	if got := eval(t, a, s, in, now).Status; got != "stopped" {
		t.Fatal(got)
	}
	in.Usage[0].Spent = 0
	if got := eval(t, a, s, in, a.Deadline).Status; got != "stopped" {
		t.Fatal(got)
	}
	in.Usage = nil
	if got := eval(t, a, s, in, now).Status; got != "blocked" {
		t.Fatal(got)
	}
	in.Usage = []Usage{{"calls", 0}}
	in.Progress = nil
	if got := eval(t, a, s, in, now).Status; got != "blocked" {
		t.Fatal(got)
	}
}
func TestPermutationDetachmentRaceAndBounds(t *testing.T) {
	a, s, in := base()
	a.IntentRefs = []Reference{{"b", d1}, {"a", d2}}
	f1, _ := Fingerprint(a)
	a.IntentRefs = []Reference{{"a", d2}, {"b", d1}}
	f2, _ := Fingerprint(a)
	if f1 != f2 {
		t.Fatal("assignment order changed identity")
	}
	s.AssignmentDigest = f2
	in.AssignmentDigest = f2
	before, _ := json.Marshal(in)
	r := eval(t, a, s, in, now)
	r.Goals[0].Assessment.Trace[0].ArtifactDigests[0] = "changed"
	after, _ := json.Marshal(in)
	if !bytes.Equal(before, after) {
		t.Fatal("input mutated")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				if _, e := Evaluate(a, s, in, now); e != nil {
					t.Error(e)
				}
			}
		}()
	}
	wg.Wait()
	a.Goals = append(a.Goals, make([]Goal, 64)...)
	if e := Validate(a); e == nil {
		t.Fatal("oversize accepted")
	}
	a, _, _ = base()
	a.Goals[0].DependsOn = []string{"main"}
	if e := Validate(a); e == nil {
		t.Fatal("cycle accepted")
	}
}
func TestMarshaledGoCUE(t *testing.T) {
	if _, e := exec.LookPath("cue"); e != nil {
		t.Skip("cue unavailable")
	}
	a, s, in := base()
	result := eval(t, a, s, in, now)
	for _, tc := range []struct {
		name, def string
		v         any
	}{{"assignment", "#Assignment", a}, {"scope", "#Scope", s}, {"input", "#Input", in}, {"result", "#Result", result}} {
		b, e := json.Marshal(tc.v)
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(t.TempDir(), tc.name+".json")
		if e := os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		cmd := exec.Command("cue", "vet", "schema.cue", p, "-d", tc.def)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("%s: %v %s", tc.name, e, out)
		}
	}
	a.Goals[0].Kind = "bogus"
	b, _ := json.Marshal(a)
	p := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(p, b, 0600)
	out, e := exec.Command("cue", "vet", "schema.cue", p, "-d", "#Assignment").CombinedOutput()
	if e == nil {
		t.Fatalf("invalid CUE accepted: %s", out)
	}
	if !strings.Contains(string(out), "kind") {
		t.Fatal(string(out))
	}
}
func TestOutputStable(t *testing.T) {
	a, s, in := base()
	r1 := eval(t, a, s, in, now)
	r2 := eval(t, a, s, in, now)
	if !reflect.DeepEqual(r1, r2) {
		t.Fatal("nondeterministic output")
	}
}

func TestObservationFingerprintAndInputPermutation(t *testing.T) {
	a, s, in := base()
	o := in.Goals[0].Observations[0]
	o.ArtifactDigests = []string{d1, d3}
	f1, e := ObservationFingerprint(o)
	if e != nil {
		t.Fatal(e)
	}
	o.ArtifactDigests = []string{d3, d1}
	f2, e := ObservationFingerprint(o)
	if e != nil || f1 != f2 {
		t.Fatal("artifact order changed observation identity", e)
	}
	o.Reason = "changed"
	f3, e := ObservationFingerprint(o)
	if e != nil || f3 == f1 {
		t.Fatal("changed observation reused identity", e)
	}
	o.Reason = "verified"
	o.ValidUntil = o.ObservedAt
	if _, e := ObservationFingerprint(o); e == nil {
		t.Fatal("invalid observation hashed")
	}
	a.Concerns = []Concern{{ID: "a", Description: "Advisory", Severity: "advisory"}, {ID: "b", Description: "Other", Severity: "advisory"}}
	fp, _ := Fingerprint(a)
	s.AssignmentDigest = fp
	in.AssignmentDigest = fp
	in.Concerns = []ConcernInput{{ID: "a", State: "open", Reason: "pending", ObservedAt: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)}, {ID: "b", State: "open", Reason: "pending", ObservedAt: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)}}
	r1 := eval(t, a, s, in, now)
	in.Concerns[0], in.Concerns[1] = in.Concerns[1], in.Concerns[0]
	r2 := eval(t, a, s, in, now)
	if !reflect.DeepEqual(r1, r2) {
		t.Fatal("input order changed result")
	}
}
func TestMalformedAndBoundaries(t *testing.T) {
	a, s, in := base()
	in.Usage = []Usage{{BudgetID: "foreign", Spent: 0}}
	if _, e := Evaluate(a, s, in, now); e == nil {
		t.Fatal("foreign usage accepted")
	}
	a, s, in = base()
	in.Goals[0].Readings = []ConfidenceReading{{CriterionID: "check", ObservationDigest: d1, BasisDigest: d1, SiteDigest: d1, Value: 0.8}}
	if _, e := Evaluate(a, s, in, now); e == nil {
		t.Fatal("unrequested reading accepted")
	}
	a, s, in = base()
	a.Goals[0].Title = strings.Repeat("x", 2049)
	if e := Validate(a); e == nil {
		t.Fatal("oversize text accepted")
	}
	a, s, in = base()
	a.Goals[0].Profile.Criteria[0].Weight = math.NaN()
	if e := Validate(a); e == nil {
		t.Fatal("nonfinite criterion accepted")
	}
	a, s, in = base()
	a.Deadline = a.Deadline.In(time.FixedZone("offset", 3600))
	if e := Validate(a); e == nil {
		t.Fatal("noncanonical time accepted")
	}
	a, s, in = base()
	a.Budgets[0].Limit = 0
	if e := Validate(a); e == nil {
		t.Fatal("zero budget accepted")
	}
}

func TestCUEBoundariesAndGoParity(t *testing.T) {
	if _, e := exec.LookPath("cue"); e != nil {
		t.Skip("cue unavailable")
	}
	vet := func(t *testing.T, v any, def string) bool {
		t.Helper()
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(t.TempDir(), "value.json")
		if e := os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		return exec.Command("cue", "vet", "schema.cue", p, "-d", def).Run() == nil
	}
	a, s, _ := base()
	a.Title = strings.Repeat("😀", 513) // 2052 UTF-8 bytes
	if Validate(a) == nil || vet(t, a, "#Assignment") {
		t.Fatal("overlong UTF-8 text accepted")
	}
	a, s, _ = base()
	a.IntentRefs = []Reference{{ID: "intent", RevisionDigest: d1}, {ID: "intent", RevisionDigest: d2}}
	if Validate(a) == nil {
		t.Fatal("duplicate intent label accepted by Go")
	}
	a, s, _ = base()
	a.Goals = append(a.Goals, make([]Goal, 64)...)
	if Validate(a) == nil || vet(t, a, "#Assignment") {
		t.Fatal("oversize goals accepted")
	}
	a, s, _ = base()
	s.RequiredGoalIDs = []string{"main", "main"}
	if vet(t, s, "#Scope") {
		t.Fatal("duplicate expected IDs accepted by CUE")
	}
	a, s, _ = base()
	a.Budgets[0].Limit = 0
	if Validate(a) == nil || vet(t, a, "#Assignment") {
		t.Fatal("zero limit accepted")
	}
}
func TestConfidencePreservesFoundationAssessment(t *testing.T) {
	a, s, in := base()
	a.Goals[0].Confidence = []ConfidenceBar{{CriterionID: "check", Minimum: 0.8, BasisDigest: d1, SiteDigest: d2}}
	in.Goals[0].Observations[0].Method = fitness.EvidenceMethodHeuristic
	fp, _ := Fingerprint(a)
	s.AssignmentDigest = fp
	in.AssignmentDigest = fp
	raw, e := fitness.AssessProfile(a.Goals[0].Profile, a.SubjectDigest, in.Goals[0].Observations, now)
	if e != nil {
		t.Fatal(e)
	}
	r := eval(t, a, s, in, now)
	g := r.Goals[0]
	if r.Status == "complete" || g.Satisfied || g.Assessment.EvidenceDigest != raw.EvidenceDigest || !reflect.DeepEqual(g.Assessment.Trace, raw.Trace) || g.Confidence[0].Status != "unknown" {
		t.Fatal("raw assessment was altered or confidence hold omitted")
	}
	in.Goals[0].Readings = []ConfidenceReading{{CriterionID: "check", ObservationDigest: d1, BasisDigest: d1, SiteDigest: d2, Value: 0.9}}
	r = eval(t, a, s, in, now)
	if r.Status == "complete" {
		t.Fatal("wrong observation digest accepted")
	}
}
func TestNestedOversizeRefuses(t *testing.T) {
	a, s, in := base()
	in.Goals[0].Observations = make([]fitness.Observation, 257)
	if _, e := Evaluate(a, s, in, now); e == nil {
		t.Fatal("oversize observations accepted")
	}
	a, s, in = base()
	in.Goals[0].Observations[0].Reason = strings.Repeat("x", 2049)
	if _, e := Evaluate(a, s, in, now); e == nil {
		t.Fatal("oversize observation reason accepted")
	}
}

func TestCUEComputableUniquenessAndAggregate(t *testing.T) {
	if _, e := exec.LookPath("cue"); e != nil {
		t.Skip("cue unavailable")
	}
	vet := func(v any, def string) bool {
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(t.TempDir(), "value.json")
		if e := os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		return exec.Command("cue", "vet", "schema.cue", p, "-d", def).Run() == nil
	}
	cases := []struct {
		name   string
		mutate func(*Assignment)
	}{
		{"intent", func(a *Assignment) { a.IntentRefs = []Reference{{"parent", d1}, {"parent", d2}} }},
		{"goal", func(a *Assignment) { g := a.Goals[0]; a.Goals = append(a.Goals, g) }},
		{"budget", func(a *Assignment) { a.Budgets = append(a.Budgets, Budget{"calls", "other", 5}) }},
		{"rule", func(a *Assignment) {
			r := Rule{"threshold", "attempts", 1, "stop", "halt", "stop"}
			a.Rules = []Rule{r, r}
		}},
		{"criterion", func(a *Assignment) {
			c := a.Goals[0].Profile.Criteria[0]
			a.Goals[0].Profile.Criteria = append(a.Goals[0].Profile.Criteria, c)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, _, _ := base()
			tc.mutate(&a)
			if Validate(a) == nil || vet(a, "#Assignment") {
				t.Fatal("duplicate accepted")
			}
		})
	}
	t.Run("aggregate-criteria", func(t *testing.T) {
		a, _, _ := base()
		template := a.Goals[0]
		a.Goals = []Goal{}
		for i := 0; i < 2; i++ {
			g := template
			g.ID = "goal" + string(rune('a'+i))
			g.Profile.Criteria = []fitness.Criterion{}
			for j := 0; j < 129; j++ {
				c := template.Profile.Criteria[0]
				c.ID = "crit" + strings.Repeat("x", i) + strings.Repeat("a", j/26) + string(rune('a'+j%26))
				g.Profile.Criteria = append(g.Profile.Criteria, c)
			}
			a.Goals = append(a.Goals, g)
		}
		if Validate(a) == nil || vet(a, "#Assignment") {
			t.Fatal("aggregate criteria bound absent")
		}
	})
	t.Run("input-identities", func(t *testing.T) {
		a, s, in := base()
		in.Goals = append(in.Goals, in.Goals[0])
		if _, e := Evaluate(a, s, in, now); e == nil || vet(in, "#Input") {
			t.Fatal("duplicate goal input accepted")
		}
		a, s, in = base()
		in.Goals[0].Observations = append(in.Goals[0].Observations, in.Goals[0].Observations[0])
		if _, e := Evaluate(a, s, in, now); e == nil || vet(in, "#Input") {
			t.Fatal("duplicate observation accepted")
		}
		a, s, in = base()
		in.Usage = append(in.Usage, in.Usage[0])
		if _, e := Evaluate(a, s, in, now); e == nil || vet(in, "#Input") {
			t.Fatal("duplicate usage accepted")
		}
	})
}

func TestCUECalendarAndInt64Parity(t *testing.T) {
	if _, e := exec.LookPath("cue"); e != nil {
		t.Skip("cue unavailable")
	}
	a, _, _ := base()
	b, e := json.Marshal(a)
	if e != nil {
		t.Fatal(e)
	}
	check := func(name, stringJSON string) {
		t.Helper()
		p := filepath.Join(t.TempDir(), name+".json")
		if e := os.WriteFile(p, []byte(stringJSON), 0600); e != nil {
			t.Fatal(e)
		}
		if out, e := exec.Command("cue", "vet", "schema.cue", p, "-d", "#Assignment").CombinedOutput(); e == nil {
			t.Fatalf("CUE accepted %s: %s", name, out)
		}
		var decoded Assignment
		if e := json.Unmarshal([]byte(stringJSON), &decoded); e == nil {
			t.Fatalf("Go accepted %s", name)
		}
	}
	badDate := strings.Replace(string(b), "2026-10-23T13:00:00Z", "2026-99-99T13:00:00Z", 1)
	check("calendar", badDate)
	badInt := strings.Replace(string(b), `"limit":2`, `"limit":9223372036854775808`, 1)
	check("int64", badInt)
}

func TestScopePreflightBoundedAllocations(t *testing.T) {
	a, scope, in := base()
	large := scope
	large.RequiredGoalIDs = make([]string, 20000)
	// Use valid unique ASCII identifiers: validation must refuse cardinality before
	// allocating a uniqueness map proportional to the untrusted roster.
	for i := range large.RequiredGoalIDs {
		large.RequiredGoalIDs[i] = "goal" + strconv.Itoa(i)
	}
	small := large
	small.RequiredGoalIDs = large.RequiredGoalIDs[:65]
	allocs := func(s Scope) float64 {
		// Fingerprint marshals JSON before rejecting the scope. Race builds randomly
		// drop sync.Pool entries, so average that churn rather than comparing three
		// noisy samples. Keep the same tolerance for input-sized uniqueness work.
		return testing.AllocsPerRun(1000, func() {
			if _, err := Evaluate(a, s, in, now); err == nil {
				t.Fatal("oversized scope accepted")
			}
		})
	}
	if got, baseline := allocs(large), allocs(small); got > baseline+2 {
		t.Fatalf("oversized roster allocation grows with input: large=%g small=%g", got, baseline)
	}
}

func TestCUEEmptyTextAndZeroTimeParity(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue unavailable")
	}
	for _, tc := range []struct {
		name string
		edit func(*Assignment)
	}{
		{"blank-title", func(a *Assignment) { a.Title = " \t\n\u00a0" }},
		{"zero-time", func(a *Assignment) { a.Deadline = time.Time{} }},
		{"year-zero", func(a *Assignment) { a.Deadline = time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _, _ := base()
			tc.edit(&a)
			if Validate(a) == nil {
				t.Fatal("Go accepted invalid value")
			}
			body, err := json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "value.json")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			if err := exec.Command("cue", "vet", "schema.cue", path, "-d", "#Assignment").Run(); err == nil {
				t.Fatal("CUE accepted Go-invalid value")
			}
		})
	}
}

func TestCanonicalAssessmentConstraints(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue unavailable")
	}
	for _, name := range []string{"required-weight", "all-scored", "unbacked-pass"} {
		t.Run(name, func(t *testing.T) {
			a, scope, in := base()
			var value any
			definition := "#Assignment"
			switch name {
			case "required-weight":
				a.Goals[0].Profile.Criteria[0].Weight = 1
				value = a
			case "all-scored":
				a.Goals[0].Profile.Criteria[0].Kind = fitness.CriterionScored
				a.Goals[0].Profile.Criteria[0].Weight = 1
				value = a
			case "unbacked-pass":
				in.Goals[0].Observations[0].ArtifactDigests = nil
				if _, err := Evaluate(a, scope, in, now); err == nil {
					t.Fatal("Go accepted unbacked pass")
				}
				value, definition = in, "#Input"
			}
			if definition == "#Assignment" && Validate(a) == nil {
				t.Fatal("Go accepted invalid profile")
			}
			body, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "value.json")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			if err := exec.Command("cue", "vet", "schema.cue", path, "-d", definition).Run(); err == nil {
				t.Fatal("CUE lost canonical assessment constraint")
			}
		})
	}
}
