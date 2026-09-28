package langbench

import (
	"encoding/json"
	"strings"
	"testing"
)

func f(v float64) *float64 { return &v }

func l1Metrics(v float64) map[string]*float64 {
	return map[string]*float64{
		MSymRecall: f(v), MSymPrecision: f(v), MKindAgreement: f(v),
		MRangeAgreement: f(v), MSignatureCoverage: f(v),
	}
}

func tiers(tier string, metrics map[string]*float64) map[string]string {
	out := map[string]string{}
	for k, v := range metrics {
		if v != nil {
			out[k] = tier
		}
	}
	return out
}

func okTenancy() Tenancy {
	return Tenancy{ThroughputMBs: 4, MaxFileMS: 100, PeakModulePages: 300}
}

func baseReading(kind string) Reading {
	return Reading{
		Dialect: "x", Kind: kind, TargetLevel: "L2",
		Subjects:         []Subject{{Name: "o/r", Files: 1000, Bytes: 1 << 20}},
		RequiredKinds:    []string{"function", "class"},
		KindIntersection: []string{"class", "function", "method"},
		Tenancy:          okTenancy(),
	}
}

// TestLevelMetFromMetricsAndTiers (LB-F16, design §1.2–§1.4): level_met and
// grade are computed from metrics, thresholds and tiers only.
func TestLevelMetFromMetricsAndTiers(t *testing.T) {
	type want struct {
		level string
		grade Grade
	}
	cases := []struct {
		name  string
		setup func(r *Reading)
		want  want
	}{
		{"tier B excellent L1", func(r *Reading) {
			r.Metrics = l1Metrics(0.97)
			r.MetricTiers = tiers("B", r.Metrics)
		}, want{"L1", Grade{L1: GradeExcellent, L2: GradeNotAttempted, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"tier C caps at good", func(r *Reading) {
			r.Metrics = l1Metrics(0.99)
			r.MetricTiers = tiers("C", r.Metrics)
		}, want{"L1", Grade{L1: GradeGood, L2: GradeNotAttempted, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"tier D never proves", func(r *Reading) {
			r.Metrics = l1Metrics(0.99)
			r.MetricTiers = tiers("D", r.Metrics)
		}, want{LevelNone, Grade{L1: GradeTolerated, L2: GradeNotAttempted, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"good threshold", func(r *Reading) {
			r.Metrics = l1Metrics(0.91)
			r.MetricTiers = tiers("B", r.Metrics)
		}, want{"L1", Grade{L1: GradeGood, L2: GradeNotAttempted, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"one metric misses", func(r *Reading) {
			r.Metrics = l1Metrics(0.97)
			r.Metrics[MSymPrecision] = f(0.5)
			r.MetricTiers = tiers("B", r.Metrics)
		}, want{LevelNone, Grade{L1: GradeUnmet, L2: GradeNotAttempted, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"unmeasured L1 metric", func(r *Reading) {
			r.Metrics = l1Metrics(0.97)
			r.Metrics[MRangeAgreement] = nil
			r.MetricTiers = tiers("B", r.Metrics)
		}, want{LevelNone, Grade{L1: GradeUnmet, L2: GradeNotAttempted, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"no callables", func(r *Reading) {
			r.Metrics = l1Metrics(0.97)
			r.Metrics[MSignatureCoverage] = nil
			r.NotApplicable = []string{MSignatureCoverage}
			r.MetricTiers = tiers("B", r.Metrics)
		}, want{"L1", Grade{L1: GradeExcellent, L2: GradeNotAttempted, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"L2 over tier A, grade is the minimum", func(r *Reading) {
			r.Metrics = l1Metrics(0.97)
			r.Metrics[MCallRecall] = f(0.80)
			r.Metrics[MCallPrecision] = f(0.95)
			r.MetricTiers = tiers("B", r.Metrics)
			r.MetricTiers[MCallRecall] = "A"
			r.MetricTiers[MCallPrecision] = "A"
		}, want{"L2", Grade{L1: GradeExcellent, L2: GradeGood, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"L2 cannot stand on an unmet L1", func(r *Reading) {
			r.Metrics = l1Metrics(0.5)
			r.Metrics[MCallRecall] = f(0.95)
			r.Metrics[MCallPrecision] = f(0.95)
			r.MetricTiers = tiers("A", r.Metrics)
		}, want{LevelNone, Grade{L1: GradeUnmet, L2: GradeExcellent, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"tier B does not prove L2 excellent", func(r *Reading) {
			r.Metrics = l1Metrics(0.97)
			r.Metrics[MCallRecall] = f(0.95)
			r.Metrics[MCallPrecision] = f(0.95)
			r.MetricTiers = tiers("B", r.Metrics)
		}, want{"L2", Grade{L1: GradeExcellent, L2: GradeGood, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"tenancy floor fails every level", func(r *Reading) {
			r.Metrics = l1Metrics(0.97)
			r.MetricTiers = tiers("B", r.Metrics)
			r.Tenancy.MaxFileMS = 5000
		}, want{LevelNone, Grade{L1: GradeExcellent, L2: GradeNotAttempted, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
		{"faults over 0.1%", func(r *Reading) {
			r.Metrics = l1Metrics(0.97)
			r.MetricTiers = tiers("B", r.Metrics)
			r.Tenancy.Faults = 2
			r.Tenancy.FaultingFiles = []string{"a", "b"}
		}, want{LevelNone, Grade{L1: GradeExcellent, L2: GradeNotAttempted, L3: GradeNotAttempted, L4: GradeNotAttempted}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := baseReading("language")
			tc.setup(&r)
			level, grade := LevelMet(r)
			if level != tc.want.level || grade != tc.want.grade {
				t.Errorf("LevelMet = %s %+v, want %s %+v", level, grade, tc.want.level, tc.want.grade)
			}
		})
	}
}

func TestLevelMetToolFormats(t *testing.T) {
	r := baseReading("tooling")
	r.Metrics = map[string]*float64{
		MSymRecall: f(0.99), MSymPrecision: f(0.96),
		MRefRecall: f(0.96), MRefPrecision: f(0.97),
		MXrefPrecision: f(0.96), MXrefRecall: f(0.85),
	}
	r.MetricTiers = tiers("A", r.Metrics)
	level, grade := LevelMet(r)
	want := Grade{L1: GradeGood, L2: GradeExcellent, L3: GradeExcellent, L4: GradeNotApplicable}
	if level != "L3" || grade != want {
		t.Fatalf("tool LevelMet = %s %+v, want L3 %+v", level, grade, want)
	}
}

func TestValidateReading(t *testing.T) {
	good := baseReading("language")
	good.Metrics = l1Metrics(0.97)
	good.MetricTiers = tiers("B", good.Metrics)
	good.LevelMet, good.Grade = LevelMet(good)
	if err := good.Validate(); err != nil {
		t.Fatalf("Validate(good) = %v", err)
	}

	cases := map[string]func(r *Reading){
		"required kind excluded": func(r *Reading) { r.RequiredKinds = append(r.RequiredKinds, "struct") },
		"typed level":            func(r *Reading) { r.LevelMet = "L2" },
		"typed grade":            func(r *Reading) { r.Grade.L1 = GradeGood },
		"metric without tier":    func(r *Reading) { delete(r.MetricTiers, MSymRecall) },
		"unknown tier":           func(r *Reading) { r.MetricTiers[MSymRecall] = "E" },
		"unknown metric":         func(r *Reading) { r.Metrics["vibes"] = f(1); r.MetricTiers["vibes"] = "A" },
		"out of range":           func(r *Reading) { r.Metrics[MSymRecall] = f(1.5) },
		"bad n/a":                func(r *Reading) { r.NotApplicable = []string{MSymRecall} },
		"faults unnamed":         func(r *Reading) { r.Tenancy.Faults = 1 },
		"no subject":             func(r *Reading) { r.Subjects = nil },
		"bad kind":               func(r *Reading) { r.Kind = "prose" },
	}
	for name, mutate := range cases {
		r := good
		r.Metrics = cloneMetrics(good.Metrics)
		r.MetricTiers = map[string]string{}
		for k, v := range good.MetricTiers {
			r.MetricTiers[k] = v
		}
		r.RequiredKinds = append([]string(nil), good.RequiredKinds...)
		mutate(&r)
		if err := r.Validate(); err == nil {
			t.Errorf("%s: Validate accepted the reading", name)
		}
	}
}

func cloneMetrics(m map[string]*float64) map[string]*float64 {
	out := map[string]*float64{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// The committed shape: two-space indent, level_met on one line, null
// metrics printed (§10).
func TestReadingMarshalShape(t *testing.T) {
	r := baseReading("language")
	r.Metrics = l1Metrics(0.97)
	r.Metrics[MCallRecall] = nil
	r.MetricTiers = tiers("B", r.Metrics)
	r.Finalize()
	b, err := MarshalReading(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"\n  \"level_met\": \"L1\",\n", "\"call_recall\": null", "\"L1\": \"excellent\""} {
		if !strings.Contains(s, want) {
			t.Errorf("marshal lacks %q:\n%s", want, s)
		}
	}
	var back Reading
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	b2, _ := MarshalReading(back)
	if string(b2) != s {
		t.Errorf("round trip not byte-identical")
	}
}
