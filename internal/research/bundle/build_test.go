package bundle

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuild_Golden(t *testing.T) {
	p50, p95, rate := 12.5, 88.0, 0.9
	obs, open := 6, 2
	in := Input{
		WindowSince:   "2026-07-21T00:00:00Z",
		WindowUntil:   "2026-07-28T00:00:00Z",
		GeneratedAt:   "2026-07-28T00:00:00Z",
		FathomVersion: "fathom v0.1.0",
		CommitSHA:     "abc1234def5",
		CommitDirty:   true,

		CallLogRead:  true,
		HooksLogRead: true,

		TotalCalls: 42,
		SurfaceTraffic: []SurfaceCount{
			{Surface: "hook", Calls: 12},
			{Surface: "agent", Calls: 30},
		},
		ToolCalls: []ToolCount{
			{Tool: "verify_plan", Calls: 5},
			{Tool: "find_symbol", Calls: 20},
		},

		LatencyP50MS: &p50,
		LatencyP95MS: &p95,

		InjectionSeen:         10,
		InjectionAnswered:     9,
		InjectionCoverageRate: &rate,
		ObligationsServed:     8,

		VerifyClaimed:                 5,
		VerifyVerified:                4,
		VerifyFailing:                 1,
		VerifyInstrumentedCalls:       5,
		VerifyPredatesInstrumentation: 2,
		ReviewActiveFindings:          3,

		StalenessReportCalls: 2,

		TotalObservations: &obs,
		OpenObservations:  &open,

		ServicePlaintextCalls: map[string]int{
			"my-secret-repo-name": 30,
			"another-repo":        12,
		},
		Salt: "test-salt",

		BenchArms: []BenchArmCount{
			{Arm: "grep", TaskCount: 13, SuccessCount: 11, SuccessRate: 0.846, MeanTokens: 8800, MeanWallMS: 91000},
			{Arm: "fathom", TaskCount: 13, SuccessCount: 13, SuccessRate: 1.0, MeanTokens: 5000, MeanWallMS: 42000},
		},
	}

	b := Build(in)

	if b.SchemaVersion != SchemaVersionCurrent {
		t.Errorf("SchemaVersion = %q, want %q", b.SchemaVersion, SchemaVersionCurrent)
	}
	if b.CallTraffic == nil {
		t.Fatal("CallTraffic is nil, want present (CallLogRead was true)")
	}
	if b.CallTraffic.TotalCalls != 42 {
		t.Errorf("CallTraffic.TotalCalls = %d, want 42", b.CallTraffic.TotalCalls)
	}
	if b.VerifyGate == nil {
		t.Fatal("VerifyGate is nil, want present (CallLogRead was true)")
	}
	if b.Injection == nil {
		t.Fatal("Injection is nil, want present (HooksLogRead was true)")
	}

	// SurfaceTraffic/ToolCalls/BenchArms are sorted deterministically
	// regardless of Input order.
	st := b.CallTraffic.SurfaceTraffic
	if len(st) != 2 || st[0].Surface != "agent" || st[1].Surface != "hook" {
		t.Errorf("SurfaceTraffic not sorted: %+v", st)
	}
	tc := b.CallTraffic.ToolCalls
	if len(tc) != 2 || tc[0].Tool != "find_symbol" || tc[1].Tool != "verify_plan" {
		t.Errorf("ToolCalls not sorted: %+v", tc)
	}
	if len(b.BenchArms) != 2 || b.BenchArms[0].Arm != "fathom" || b.BenchArms[1].Arm != "grep" {
		t.Errorf("BenchArms not sorted: %+v", b.BenchArms)
	}

	// Service identities are hashed, never plaintext, and sorted by hash.
	if len(b.ServiceActivity) != 2 {
		t.Fatalf("ServiceActivity = %+v, want 2 rows", b.ServiceActivity)
	}
	wantHashA := HashService("test-salt", "my-secret-repo-name")
	wantHashB := HashService("test-salt", "another-repo")
	gotHashes := map[string]int{}
	for _, r := range b.ServiceActivity {
		gotHashes[r.ServiceHash] = r.Calls
	}
	if gotHashes[wantHashA] != 30 || gotHashes[wantHashB] != 12 {
		t.Errorf("ServiceActivity hashes/counts = %+v, want {%s:30, %s:12}", gotHashes, wantHashA, wantHashB)
	}

	// Byte-level: the plaintext service names must never appear anywhere in
	// the marshaled bundle.
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "my-secret-repo-name") || strings.Contains(string(raw), "another-repo") {
		t.Fatalf("bundle JSON leaks a plaintext service name: %s", raw)
	}
	if strings.Contains(string(raw), "test-salt") {
		t.Fatalf("bundle JSON leaks the salt: %s", raw)
	}

	if err := Validate(b); err != nil {
		t.Errorf("Validate(Build(in)) = %v, want nil", err)
	}
}

// TestBuild_Determinism proves Build is a pure, deterministic function of
// its input: two calls with the same Input (including an unordered map)
// produce byte-identical JSON.
func TestBuild_Determinism(t *testing.T) {
	in := Input{
		GeneratedAt: "2026-07-28T00:00:00Z",
		CallLogRead: true,
		TotalCalls:  3,
		ToolCalls: []ToolCount{
			{Tool: "b_tool", Calls: 1},
			{Tool: "a_tool", Calls: 2},
		},
		ServicePlaintextCalls: map[string]int{"svc-a": 1, "svc-b": 2, "svc-c": 3},
		Salt:                  "salt",
	}
	a, err := json.Marshal(Build(in))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(Build(in))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("Build is not deterministic:\n%s\nvs\n%s", a, b)
	}
}

// TestBuild_EmptyInput proves a zero-value Input still produces a
// schema-valid Bundle (the honest all-absent case, not an error): with
// CallLogRead/HooksLogRead both false, no group is fabricated.
func TestBuild_EmptyInput(t *testing.T) {
	b := Build(Input{})
	if err := Validate(b); err != nil {
		t.Fatalf("Validate(Build(Input{})) = %v, want nil", err)
	}
	if b.CallTraffic != nil {
		t.Errorf("CallTraffic = %+v, want nil (CallLogRead was false)", b.CallTraffic)
	}
	if b.VerifyGate != nil {
		t.Errorf("VerifyGate = %+v, want nil (CallLogRead was false)", b.VerifyGate)
	}
	if b.Injection != nil {
		t.Errorf("Injection = %+v, want nil (HooksLogRead was false)", b.Injection)
	}
	if b.ServiceActivity != nil || b.BenchArms != nil {
		t.Errorf("zero Input should produce nil optional slices, got %+v", b)
	}
}

// TestBuild_CallLogUnreadable_NoFabricatedZeros is critic Finding 3's own
// regression test: when CallLogRead is false, CallTraffic and VerifyGate
// must be nil even if the caller (by a hypothetical future bug) still
// populated their source scalar fields — the gate, not the values, decides
// whether the group is emitted.
func TestBuild_CallLogUnreadable_NoFabricatedZeros(t *testing.T) {
	in := Input{
		CallLogRead:  false,
		HooksLogRead: false,
		// Populated despite the false gates — Build must ignore these,
		// never emit a zero-valued group that looks like a real
		// measurement.
		TotalCalls:        999,
		VerifyClaimed:     5,
		InjectionSeen:     10,
		InjectionAnswered: 10,
	}
	b := Build(in)
	if b.CallTraffic != nil {
		t.Errorf("CallTraffic = %+v, want nil despite populated scalar fields (CallLogRead=false)", b.CallTraffic)
	}
	if b.VerifyGate != nil {
		t.Errorf("VerifyGate = %+v, want nil despite populated scalar fields (CallLogRead=false)", b.VerifyGate)
	}
	if b.Injection != nil {
		t.Errorf("Injection = %+v, want nil despite populated scalar fields (HooksLogRead=false)", b.Injection)
	}
}

// TestBuild_ReadButZero_IsARealMeasurement proves the flip side: when the
// instrument WAS read, a genuine zero is reported as a present group with
// zero values, not omitted.
func TestBuild_ReadButZero_IsARealMeasurement(t *testing.T) {
	in := Input{
		CallLogRead:  true,
		HooksLogRead: true,
		// Every scalar left at zero: a real "read, and found nothing."
	}
	b := Build(in)
	if b.CallTraffic == nil {
		t.Fatal("CallTraffic is nil, want present-with-zeros (CallLogRead=true)")
	}
	if b.CallTraffic.TotalCalls != 0 {
		t.Errorf("CallTraffic.TotalCalls = %d, want 0", b.CallTraffic.TotalCalls)
	}
	if b.VerifyGate == nil {
		t.Fatal("VerifyGate is nil, want present-with-zeros (CallLogRead=true)")
	}
	if b.Injection == nil {
		t.Fatal("Injection is nil, want present-with-zeros (HooksLogRead=true)")
	}
	if b.Injection.Seen != 0 {
		t.Errorf("Injection.Seen = %d, want 0", b.Injection.Seen)
	}
	if err := Validate(b); err != nil {
		t.Errorf("Validate(read-but-zero bundle) = %v, want nil", err)
	}
}
