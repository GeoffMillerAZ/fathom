package benchledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench"
	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

func inputs() CellInputs {
	return CellInputs{Base: contextbench.CellInputs{CorpusSHA256: strings.Repeat("a", 64), TaskID: "task", Arm: "treatment", ModelID: "model", HarnessVersion: "harness", Scoring: "oracle", PromptSubject: "subject", ArmSurface: strings.Repeat("b", 40)}, SubjectCommit: strings.Repeat("c", 40), ToolSchemaSHA256: strings.Repeat("d", 64), ProfileSHA256: strings.Repeat("e", 64), Family: FamilyFIX, CriterionID: "criterion", Trial: 1}
}
func receipt(id string) PreflightReceipt {
	return PreflightReceipt{SessionID: id, ProductCommit: strings.Repeat("b", 40), SubjectCommit: strings.Repeat("c", 40), ProfileSHA256: strings.Repeat("e", 64), BeforeAgent: true, W0: RungPass, W1: RungPass, W2: RungPass, W3: RungPass, W6: RungPass, EvidenceSHA256: strings.Repeat("f", 64)}
}
func pair(t *testing.T) PairPreflight {
	t.Helper()
	p, e := AdmitPair("pair", receipt("t"), receipt("c"))
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func cell(arm string) scenariobench.CellResult {
	return scenariobench.CellResult{TaskID: "task", Arm: arm, Trial: 1}
}
func refusal(t *testing.T, e error) {
	t.Helper()
	var r *RefusalError
	if !errors.As(e, &r) {
		t.Fatalf("want typed refusal, got %v", e)
	}
}
func TestLaneBenchCells(t *testing.T) {
	in := inputs()
	k, e := CellKey(in)
	if e != nil {
		t.Fatal(e)
	}
	if len(k) != 64 {
		t.Fatal(k)
	}
	again, _ := CellKey(in)
	if again != k {
		t.Fatal("nondeterministic")
	}
	canonical := `{"base_key":"` + contextbench.CellKey(in.Base) + `","subject_commit":"` + in.SubjectCommit + `","tool_schema_sha256":"` + in.ToolSchemaSHA256 + `","profile_sha256":"` + in.ProfileSHA256 + `","family":"FIX","criterion_id":"criterion","trial":1}`
	sum := sha256.Sum256([]byte(canonical))
	if k != hex.EncodeToString(sum[:]) {
		t.Fatal("key is not SHA256 of contract fixed-order JSON")
	}
	// Every base invariant and every additional actual input binds identity.
	changes := []func(*CellInputs){func(x *CellInputs) { x.Base.CorpusSHA256 = strings.Repeat("1", 64) }, func(x *CellInputs) { x.Base.TaskID += "2" }, func(x *CellInputs) { x.Base.Arm += "2" }, func(x *CellInputs) { x.Base.ModelID += "2" }, func(x *CellInputs) { x.Base.HarnessVersion += "2" }, func(x *CellInputs) { x.Base.Scoring += "2" }, func(x *CellInputs) { x.Base.PromptSubject += "2" }, func(x *CellInputs) { x.Base.ArmSurface = strings.Repeat("2", 40) }, func(x *CellInputs) { x.SubjectCommit = strings.Repeat("3", 40) }, func(x *CellInputs) { x.ToolSchemaSHA256 = strings.Repeat("4", 64) }, func(x *CellInputs) { x.ProfileSHA256 = strings.Repeat("5", 64) }, func(x *CellInputs) { x.Family = FamilyLOC }, func(x *CellInputs) { x.CriterionID += "2" }, func(x *CellInputs) { x.Trial++ }}
	for i, change := range changes {
		x := in
		change(&x)
		got, e := CellKey(x)
		if e != nil || got == k {
			t.Fatalf("input %d did not bind: %v", i, e)
		}
	}
	for _, family := range []TaskFamily{FamilyPLAN, FamilyHAZ, FamilyFIX, FamilyLOC, FamilyREV, FamilyLANE, FamilyASK} {
		x := in
		x.Family = family
		if _, e := CellKey(x); e != nil {
			t.Fatal(e)
		}
	}
	for _, field := range []string{"CorpusSHA256", "TaskID", "Arm", "ModelID", "HarnessVersion", "Scoring", "PromptSubject", "ArmSurface"} {
		for _, breaks := range []string{"\r", "\n"} {
			x := in
			v := reflect.ValueOf(&x.Base).Elem().FieldByName(field)
			v.SetString(v.String() + breaks)
			_, e := CellKey(x)
			refusal(t, e)
		}
	}
	for _, change := range []func(*CellInputs){func(x *CellInputs) { x.Trial = 0 }, func(x *CellInputs) { x.Family = "OTHER" }, func(x *CellInputs) { x.SubjectCommit = "abc" }, func(x *CellInputs) { x.ToolSchemaSHA256 = "bad" }, func(x *CellInputs) { x.CriterionID = "" }, func(x *CellInputs) { x.CriterionID = "a\nb" }, func(x *CellInputs) { x.Base.TaskID = "" }, func(x *CellInputs) { x.ProfileSHA256 = "" }} {
		x := in
		change(&x)
		_, e := CellKey(x)
		refusal(t, e)
	}
	control := in
	control.Base.Arm = "control"
	control.Base.ArmSurface = ""
	control.ToolSchemaSHA256 = ""
	control.ProfileSHA256 = ""
	before, e := CellKey(control)
	if e != nil {
		t.Fatal(e)
	}
	treatmentBefore, _ := CellKey(in)
	in.Base.ArmSurface = strings.Repeat("9", 40)
	treatmentAfter, _ := CellKey(in)
	if treatmentBefore == treatmentAfter {
		t.Fatal("treatment commit not bound")
	}
	after, _ := CellKey(control)
	if before != after {
		t.Fatal("treatment-only change invalidated control")
	}
	badControl := control
	badControl.ProfileSHA256 = strings.Repeat("e", 64)
	_, e = CellKey(badControl)
	refusal(t, e)
	row := rowFixture(t)
	row.Key = "wrong"
	refusal(t, AppendCell(&strings.Builder{}, nil, row))
	row = rowFixture(t)
	refusal(t, AppendCell(&strings.Builder{}, []CellRow{row}, row))
}
func TestPairPreflightAssignedArms(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*PreflightReceipt)
		want   PreflightDisposition
	}{
		{"healthy", func(r *PreflightReceipt) {}, PreflightAdmitted},
		{"fail", func(r *PreflightReceipt) { r.W2 = RungFail }, PreflightVoid},
		{"W0 fail", func(r *PreflightReceipt) { r.W0 = RungFail }, PreflightVoid},
		{"W1 fail", func(r *PreflightReceipt) { r.W1 = RungFail }, PreflightVoid},
		{"W3 fail", func(r *PreflightReceipt) { r.W3 = RungFail }, PreflightVoid},
		{"W6 fail", func(r *PreflightReceipt) { r.W6 = RungFail }, PreflightVoid},
		{"missing digest beats fail", func(r *PreflightReceipt) { r.EvidenceSHA256 = ""; r.W0 = RungFail }, PreflightUnknown},
		{"unknown", func(r *PreflightReceipt) { r.W2 = RungUnknown }, PreflightUnknown},
		{"missing", func(r *PreflightReceipt) { *r = PreflightReceipt{} }, PreflightUnknown},
		{"late fail", func(r *PreflightReceipt) { r.BeforeAgent = false; r.W0 = RungFail }, PreflightUnknown},
		{"off with adapter-verified absence", func(r *PreflightReceipt) { r.W2 = RungNotApplicable }, PreflightAdmitted},
		{"off missing evidence", func(r *PreflightReceipt) { r.W2 = RungNotApplicable; r.EvidenceSHA256 = "" }, PreflightUnknown},
		{"active failure", func(r *PreflightReceipt) { r.W2 = RungNotApplicable; r.W3 = RungFail }, PreflightVoid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, which := range []int{0, 1} {
				a, b := receipt("t"), receipt("c")
				if which == 0 {
					tc.change(&a)
				} else {
					tc.change(&b)
				}
				p, e := AdmitPair("pair", a, b)
				if e != nil {
					t.Fatal(e)
				}
				if p.Disposition != tc.want || p.Valid != (tc.want == PreflightAdmitted) {
					t.Fatalf("%+v", p)
				}
				for _, arm := range []string{"treatment", "control"} {
					got, e := ProjectCell(cell(arm), arm, p)
					if e != nil {
						t.Fatal(e)
					}
					want := map[PreflightDisposition]CellState{PreflightAdmitted: CellScored, PreflightVoid: CellVoid, PreflightUnknown: CellUnknown}[tc.want]
					if got.State != want || got.Success || got.SurfaceCalls != 0 {
						t.Fatalf("%+v", got)
					}
				}
			}
		})
	}
	// A daemon-free control still supplies subject/startup/budget evidence.
	a, b := receipt("t"), receipt("c")
	b.ProductCommit = ""
	b.ProfileSHA256 = ""
	b.W1 = RungNotApplicable
	b.W2 = RungNotApplicable
	b.W3 = RungNotApplicable
	p, e := AdmitPair("baseline", a, b)
	if e != nil || p.Disposition != PreflightAdmitted {
		t.Fatalf("baseline: %+v %v", p, e)
	}
	b.W0 = RungFail
	p, e = AdmitPair("baseline", a, b)
	if e != nil || p.Disposition != PreflightVoid {
		t.Fatalf("baseline W0: %+v %v", p, e)
	}
	b = receipt("c")
	b.SubjectCommit = strings.Repeat("1", 40)
	_, e = AdmitPair("baseline", a, b)
	refusal(t, e)
	for _, change := range []func(*PreflightReceipt){func(r *PreflightReceipt) { r.W0 = RungNotApplicable }, func(r *PreflightReceipt) { r.W6 = RungNotApplicable }, func(r *PreflightReceipt) { r.W1 = "bogus" }, func(r *PreflightReceipt) { r.EvidenceSHA256 = "bad" }} {
		a := receipt("t")
		change(&a)
		_, e := AdmitPair("pair", a, receipt("c"))
		refusal(t, e)
	}
}
func TestProjectCellPreflightDisposition(t *testing.T) {
	for _, d := range []PreflightDisposition{PreflightAdmitted, PreflightVoid, PreflightUnknown} {
		p := pair(t)
		p.Disposition = d
		p.Valid = d == PreflightAdmitted
		for _, reason := range []string{"fail", "all pass", "late unknown", ""} {
			p.Reason = reason
			for _, arm := range []string{"treatment", "control"} {
				got, e := ProjectCell(cell(arm), arm, p)
				if e != nil {
					t.Fatal(e)
				}
				want := map[PreflightDisposition]CellState{PreflightAdmitted: CellScored, PreflightVoid: CellVoid, PreflightUnknown: CellUnknown}[d]
				if got.State != want {
					t.Fatal(got)
				} // Delivery consumers use this identical typed mapping.
				if (got.State == CellVoid) != (p.Disposition == PreflightVoid) {
					t.Fatal("delivery mapping differs")
				}
			}
		}
	}
	got, e := ProjectCell(cell("control"), "control", PairPreflight{})
	if e != nil || got.State != CellUnknown {
		t.Fatalf("missing pair: %+v %v", got, e)
	}
	for _, change := range []func(*PairPreflight){func(p *PairPreflight) { p.Valid = false }, func(p *PairPreflight) { p.BeforeAgent = false }, func(p *PairPreflight) { p.Disposition = "bad" }} {
		p := pair(t)
		change(&p)
		_, e := ProjectCell(cell("treatment"), "treatment", p)
		refusal(t, e)
	}
	_, e = ProjectCell(cell("control"), "treatment", pair(t))
	refusal(t, e)
}
func TestScoredCellSafeProjection(t *testing.T) {
	const canary = "PRIVATE_CANARY"
	in := cell("treatment")
	in.WorktreePath = canary
	in.PlanPath = canary
	in.AgentDiffPath = canary
	in.SubjectClone = canary
	in.Gold = scenariobench.Gold{Files: []string{canary}, Functions: []string{canary}, TestPatch: canary}
	in.Oracle.TestOutputRef = canary
	in.Oracle.CoverProfile = canary
	in.Sessions = []scenariobench.SessionResult{{RawPath: canary, SessionStartContext: canary, Error: canary, InstrumentError: canary, FinalAnswer: canary, FilesRead: []string{canary}, FilesEdited: []string{canary}, TotalTokens: 7, ToolCalls: map[string]int{canary: 1}}, {TotalTokens: 11}}
	for _, success := range []bool{false, true} {
		if success {
			in.Readings.SurfaceCalls = 3
		}
		in.Success = success
		in.Oracle.Passed = success
		in.Oracle.FailToPassOK = success
		in.Oracle.PassToPassOK = true
		got, e := ProjectCell(in, "treatment", pair(t))
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := json.Marshal(got)
		if strings.Contains(string(raw), canary) {
			t.Fatal(string(raw))
		}
		var keys map[string]any
		_ = json.Unmarshal(raw, &keys)
		want := []string{"task_id", "assigned_arm", "trial", "oracle_passed", "fail_to_pass_ok", "pass_to_pass_ok", "success", "surface_calls", "total_tokens", "state"}
		if len(keys) != len(want) {
			t.Fatal(keys)
		}
		for _, k := range want {
			if _, ok := keys[k]; !ok {
				t.Fatal(k)
			}
		}
		if got.Success != success || got.OraclePassed != success || got.TotalTokens != 18 || got.SurfaceCalls != in.Readings.SurfaceCalls || got.State != CellScored || got.AssignedArm != "treatment" {
			t.Fatal(got)
		}
	}
}

func TestBenchContractsAPIHouseRules(t *testing.T) {
	check := func(value any, keys string) {
		t.Helper()
		raw, e := json.Marshal(value)
		if e != nil {
			t.Fatal(e)
		}
		var m map[string]json.RawMessage
		if e = json.Unmarshal(raw, &m); e != nil {
			t.Fatal(e)
		}
		want := strings.Fields(keys)
		if len(m) != len(want) {
			t.Fatalf("wire keys: %s", raw)
		}
		for _, k := range want {
			if _, ok := m[k]; !ok {
				t.Fatalf("missing %s in %s", k, raw)
			}
		}
	}
	check(inputs(), "base subject_commit tool_schema_sha256 profile_sha256 family criterion_id trial")
	raw, _ := json.Marshal(inputs())
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	var base map[string]string
	_ = json.Unmarshal(m["base"], &base)
	if len(base) != 8 || base["arm_surface"] != inputs().Base.ArmSurface {
		t.Fatal(base)
	}
	check(receipt("t"), "session_id product_commit subject_commit profile_sha256 before_agent w0 w1 w2 w3 w6 reason evidence_sha256")
	check(pair(t), "pair_id treatment_session control_session before_agent valid disposition reason")
	empty, e := ReadCells(strings.NewReader(""))
	if e != nil {
		t.Fatal(e)
	}
	raw, _ = json.Marshal(empty)
	if string(raw) != "[]" {
		t.Fatal(string(raw))
	}
	in := cell("treatment")
	in.Sessions = []scenariobench.SessionResult{{TotalTokens: math.MaxInt64}, {TotalTokens: 1}}
	_, e = ProjectCell(in, "treatment", pair(t))
	var limit *LimitError
	if !errors.As(e, &limit) {
		t.Fatalf("want typed limit: %v", e)
	}
	// Computation functions accept no context; I/O remains at the reader/writer.
	for _, fn := range []any{CellKey, AdmitPair, ProjectCell} {
		typ := reflect.TypeOf(fn)
		for i := 0; i < typ.NumIn(); i++ {
			if typ.In(i).String() == "context.Context" {
				t.Fatal("pure function takes context")
			}
		}
	}
}
