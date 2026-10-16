package modelcatalog_test

import (
	"encoding/json"
	"sort"
	"testing"

	modelcatalog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/model-catalog"
)

// ---- helpers -----------------------------------------------------------------

// mustLoad is a test helper that calls Load and fails the test on error.
func mustLoad(t *testing.T, data []byte) modelcatalog.Catalog {
	t.Helper()
	c, err := modelcatalog.Load(data)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	return c
}

// catalogJSON builds the JSON representation of a simple catalog for tests.
// It marshals the provided map under the "models" key.
func catalogJSON(t *testing.T, models map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"models": models})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// entryJSON returns a map[string]any for a ModelEntry suitable for use in
// catalogJSON. Callers override fields as needed for their test case.
func entryJSON(id, family string, costTier int) map[string]any {
	return map[string]any{
		"id":      id,
		"family":  family,
		"lineage": []string{},
		"caps": map[string]any{
			"thinking":           false,
			"effort":             false,
			"temperature":        true,
			"seed":               false,
			"structured_outputs": false,
		},
		"cost_tier":  costTier,
		"deprecated": false,
	}
}

// ---- Load: happy path -------------------------------------------------------

func TestLoad_HappyPath(t *testing.T) {
	// Mirrors the content of testdata/valid.cue → validFullCatalog.
	data := catalogJSON(t, map[string]any{
		"example-fast-1": entryJSON("example-fast-1", "alpha", 0),
		"example-smart-1": map[string]any{
			"id":      "example-smart-1",
			"family":  "beta",
			"lineage": []string{"alpha"},
			"caps": map[string]any{
				"thinking":           true,
				"effort":             true,
				"temperature":        true,
				"seed":               false,
				"structured_outputs": true,
			},
			"cost_tier":  2,
			"deprecated": false,
		},
		"example-judge-1": entryJSON("example-judge-1", "gamma", 1),
	})

	c := mustLoad(t, data)

	// Entry lookup — present.
	e, ok := c.Entry("example-fast-1")
	if !ok {
		t.Fatal("Entry(\"example-fast-1\"): want ok=true, got false")
	}
	if e.ID != "example-fast-1" || e.Family != "alpha" || e.CostTier != 0 {
		t.Errorf("Entry(\"example-fast-1\") = %+v, want id=example-fast-1 family=alpha cost_tier=0", e)
	}
	if !e.Caps.Temperature {
		t.Error("Caps.Temperature should be true")
	}

	// Entry lookup — caps flags for thinking model.
	smart, ok := c.Entry("example-smart-1")
	if !ok {
		t.Fatal("Entry(\"example-smart-1\"): want ok=true, got false")
	}
	if !smart.Caps.Thinking || !smart.Caps.Effort || !smart.Caps.StructuredOutputs {
		t.Errorf("smart caps: thinking=%v effort=%v structured_outputs=%v, all want true",
			smart.Caps.Thinking, smart.Caps.Effort, smart.Caps.StructuredOutputs)
	}
	if len(smart.Lineage) != 1 || smart.Lineage[0] != "alpha" {
		t.Errorf("smart.Lineage = %v, want [alpha]", smart.Lineage)
	}

	// Entry lookup — absent.
	_, ok = c.Entry("no-such-model")
	if ok {
		t.Error("Entry(\"no-such-model\"): want ok=false, got true")
	}
}

func TestLoad_EmptyCatalog(t *testing.T) {
	data := catalogJSON(t, map[string]any{})
	c := mustLoad(t, data)
	_, ok := c.Entry("anything")
	if ok {
		t.Error("Entry on empty catalog: want ok=false")
	}
}

// ---- Load: rejection table --------------------------------------------------

func TestLoad_RejectionTable(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr string // non-empty substring expected in error
	}{
		{
			name:    "malformed JSON",
			data:    []byte(`{not valid json`),
			wantErr: "JSON unmarshal",
		},
		{
			name: "empty id field",
			data: catalogJSON(t, map[string]any{
				"example-fast-1": map[string]any{
					"id":      "", // empty — invalid
					"family":  "alpha",
					"lineage": []string{},
					"caps": map[string]any{
						"thinking": false, "effort": false,
						"temperature": true, "seed": false, "structured_outputs": false,
					},
					"cost_tier": 0, "deprecated": false,
				},
			}),
			wantErr: "empty id",
		},
		{
			name: "empty family field",
			data: catalogJSON(t, map[string]any{
				"example-fast-1": map[string]any{
					"id":      "example-fast-1",
					"family":  "", // empty — invalid
					"lineage": []string{},
					"caps": map[string]any{
						"thinking": false, "effort": false,
						"temperature": true, "seed": false, "structured_outputs": false,
					},
					"cost_tier": 0, "deprecated": false,
				},
			}),
			wantErr: "empty family",
		},
		{
			name: "negative cost_tier",
			data: catalogJSON(t, map[string]any{
				"example-fast-1": map[string]any{
					"id":      "example-fast-1",
					"family":  "alpha",
					"lineage": []string{},
					"caps": map[string]any{
						"thinking": false, "effort": false,
						"temperature": true, "seed": false, "structured_outputs": false,
					},
					"cost_tier":  -1, // invalid
					"deprecated": false,
				},
			}),
			wantErr: "negative cost_tier",
		},
		{
			name: "id does not match map key",
			data: catalogJSON(t, map[string]any{
				"example-fast-1": map[string]any{
					"id":      "completely-different-id", // mismatch
					"family":  "alpha",
					"lineage": []string{},
					"caps": map[string]any{
						"thinking": false, "effort": false,
						"temperature": true, "seed": false, "structured_outputs": false,
					},
					"cost_tier": 0, "deprecated": false,
				},
			}),
			wantErr: "does not match id field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := modelcatalog.Load(tt.data)
			if err == nil {
				t.Fatal("Load: want error, got nil")
			}
			if tt.wantErr != "" {
				if !containsStr(err.Error(), tt.wantErr) {
					t.Errorf("Load error = %q, want substring %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

// ---- Catalog.Validate -------------------------------------------------------

func TestValidate_NoProblemCleanCatalog(t *testing.T) {
	data := catalogJSON(t, map[string]any{
		"example-fast-1":  entryJSON("example-fast-1", "alpha", 0),
		"example-judge-1": entryJSON("example-judge-1", "gamma", 1),
	})
	c := mustLoad(t, data)
	if problems := c.Validate(); len(problems) != 0 {
		t.Errorf("Validate: want no problems, got %v", problems)
	}
}

func TestValidate_DanglingLineageRef(t *testing.T) {
	data := catalogJSON(t, map[string]any{
		"example-smart-1": map[string]any{
			"id":      "example-smart-1",
			"family":  "beta",
			"lineage": []string{"nonexistent-family"}, // no model in this family
			"caps": map[string]any{
				"thinking": false, "effort": false,
				"temperature": true, "seed": false, "structured_outputs": false,
			},
			"cost_tier": 1, "deprecated": false,
		},
	})
	c := mustLoad(t, data)
	problems := c.Validate()
	if !hasProblemKind(problems, "dangling_lineage_ref") {
		t.Errorf("Validate: want dangling_lineage_ref problem, got %v", problems)
	}
}

func TestValidate_AllDeprecatedFamily(t *testing.T) {
	data := catalogJSON(t, map[string]any{
		"example-old-1": map[string]any{
			"id":      "example-old-1",
			"family":  "epsilon",
			"lineage": []string{},
			"caps": map[string]any{
				"thinking": false, "effort": false,
				"temperature": true, "seed": false, "structured_outputs": false,
			},
			"cost_tier": 0, "deprecated": true,
		},
		"example-old-2": map[string]any{
			"id":      "example-old-2",
			"family":  "epsilon",
			"lineage": []string{},
			"caps": map[string]any{
				"thinking": false, "effort": false,
				"temperature": true, "seed": false, "structured_outputs": false,
			},
			"cost_tier": 0, "deprecated": true,
		},
		// A live model in a different family — must not produce a false positive.
		"example-fast-1": entryJSON("example-fast-1", "alpha", 0),
	})
	c := mustLoad(t, data)
	problems := c.Validate()
	if !hasProblemKind(problems, "all_deprecated_family") {
		t.Errorf("Validate: want all_deprecated_family problem, got %v", problems)
	}
	// The live alpha family must not be flagged.
	for _, p := range problems {
		if p.Kind == "all_deprecated_family" && containsStr(p.Detail, "alpha") {
			t.Errorf("Validate: falsely flagged alpha family: %v", p)
		}
	}
}

func TestValidate_PartiallyDeprecatedFamilyIsNotAProblem(t *testing.T) {
	data := catalogJSON(t, map[string]any{
		"example-fast-1": map[string]any{
			"id":      "example-fast-1",
			"family":  "alpha",
			"lineage": []string{},
			"caps": map[string]any{
				"thinking": false, "effort": false,
				"temperature": true, "seed": false, "structured_outputs": false,
			},
			"cost_tier": 0, "deprecated": false, // still active
		},
		"example-fast-1-legacy": map[string]any{
			"id":      "example-fast-1-legacy",
			"family":  "alpha",
			"lineage": []string{},
			"caps": map[string]any{
				"thinking": false, "effort": false,
				"temperature": true, "seed": false, "structured_outputs": false,
			},
			"cost_tier": 0, "deprecated": true, // retired
		},
	})
	c := mustLoad(t, data)
	if problems := c.Validate(); len(problems) != 0 {
		t.Errorf("Validate: partially-deprecated family must not produce problems, got %v", problems)
	}
}

func TestValidate_Deterministic(t *testing.T) {
	// A catalog with two problems to ensure sorting is stable across calls.
	data := catalogJSON(t, map[string]any{
		"example-smart-1": map[string]any{
			"id":      "example-smart-1",
			"family":  "beta",
			"lineage": []string{"ghost-family"}, // dangling ref
			"caps": map[string]any{
				"thinking": false, "effort": false,
				"temperature": true, "seed": false, "structured_outputs": false,
			},
			"cost_tier": 1, "deprecated": true, // all-deprecated beta
		},
	})
	c := mustLoad(t, data)
	got1 := c.Validate()
	got2 := c.Validate()
	if len(got1) != len(got2) {
		t.Fatalf("Validate not deterministic: len %d vs %d", len(got1), len(got2))
	}
	for i := range got1 {
		if got1[i] != got2[i] {
			t.Errorf("Validate not deterministic at index %d: %v vs %v", i, got1[i], got2[i])
		}
	}
	// Also verify they are sorted by Kind then Detail.
	sorted := make([]modelcatalog.Problem, len(got1))
	copy(sorted, got1)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Kind != sorted[j].Kind {
			return sorted[i].Kind < sorted[j].Kind
		}
		return sorted[i].Detail < sorted[j].Detail
	})
	for i := range got1 {
		if got1[i] != sorted[i] {
			t.Errorf("Validate output not sorted at index %d: got %v, want %v", i, got1[i], sorted[i])
		}
	}
}

// ---- ValidateIndependence ---------------------------------------------------

// baseIndependenceCatalog returns a Catalog with four models useful for
// independence tests:
//
//	example-fast-1  — family alpha, lineage []
//	example-fast-2  — family alpha, lineage []       (same family as fast-1)
//	example-smart-1 — family beta,  lineage [alpha]  (lineage overlaps alpha)
//	example-judge-1 — family gamma, lineage []       (independent)
func baseIndependenceCatalog(t *testing.T) modelcatalog.Catalog {
	t.Helper()
	data := catalogJSON(t, map[string]any{
		"example-fast-1": entryJSON("example-fast-1", "alpha", 0),
		"example-fast-2": entryJSON("example-fast-2", "alpha", 0),
		"example-smart-1": map[string]any{
			"id":      "example-smart-1",
			"family":  "beta",
			"lineage": []string{"alpha"},
			"caps": map[string]any{
				"thinking": false, "effort": false,
				"temperature": true, "seed": false, "structured_outputs": false,
			},
			"cost_tier": 2, "deprecated": false,
		},
		"example-judge-1": entryJSON("example-judge-1", "gamma", 1),
	})
	return mustLoad(t, data)
}

func TestValidateIndependence_SharedFamily(t *testing.T) {
	c := baseIndependenceCatalog(t)
	violations := modelcatalog.ValidateIndependence("example-fast-1", "example-fast-2", c, modelcatalog.IndependenceOpts{})
	if !hasViolationKind(violations, "shared_family") {
		t.Errorf("want shared_family violation, got %v", violations)
	}
}

func TestValidateIndependence_SharedLineageTransitive(t *testing.T) {
	// example-smart-1 (beta, lineage=[alpha]) vs example-fast-1 (alpha).
	// smart's lineage overlaps fast's family — shared lineage.
	c := baseIndependenceCatalog(t)
	violations := modelcatalog.ValidateIndependence("example-smart-1", "example-fast-1", c, modelcatalog.IndependenceOpts{})
	if !hasViolationKind(violations, "shared_lineage") {
		t.Errorf("want shared_lineage violation, got %v", violations)
	}
}

func TestValidateIndependence_UnknownJudgeID(t *testing.T) {
	c := baseIndependenceCatalog(t)
	violations := modelcatalog.ValidateIndependence("no-such-judge", "example-judge-1", c, modelcatalog.IndependenceOpts{})
	if !hasViolationKind(violations, "unknown_model") {
		t.Errorf("want unknown_model violation for unknown judge, got %v", violations)
	}
}

func TestValidateIndependence_UnknownProducerID(t *testing.T) {
	c := baseIndependenceCatalog(t)
	violations := modelcatalog.ValidateIndependence("example-judge-1", "no-such-producer", c, modelcatalog.IndependenceOpts{})
	if !hasViolationKind(violations, "unknown_model") {
		t.Errorf("want unknown_model violation for unknown producer, got %v", violations)
	}
}

func TestValidateIndependence_UnknownBothIDs(t *testing.T) {
	c := baseIndependenceCatalog(t)
	violations := modelcatalog.ValidateIndependence("ghost-judge", "ghost-producer", c, modelcatalog.IndependenceOpts{})
	count := 0
	for _, v := range violations {
		if v.Kind == "unknown_model" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("want 2 unknown_model violations (one per unknown id), got %d in %v", count, violations)
	}
}

func TestValidateIndependence_CleanPair(t *testing.T) {
	c := baseIndependenceCatalog(t)
	// example-judge-1 (gamma) vs example-fast-1 (alpha) — no shared family or lineage.
	violations := modelcatalog.ValidateIndependence("example-judge-1", "example-fast-1", c, modelcatalog.IndependenceOpts{})
	if len(violations) != 0 {
		t.Errorf("clean pair: want no violations, got %v", violations)
	}
}

func TestValidateIndependence_OptsAssertedSameSession(t *testing.T) {
	c := baseIndependenceCatalog(t)
	violations := modelcatalog.ValidateIndependence(
		"example-judge-1", "example-fast-1", c,
		modelcatalog.IndependenceOpts{SameSession: true},
	)
	if !hasViolationKind(violations, "same_session") {
		t.Errorf("want same_session violation from opts, got %v", violations)
	}
}

func TestValidateIndependence_OptsAssertedSharedHistory(t *testing.T) {
	c := baseIndependenceCatalog(t)
	violations := modelcatalog.ValidateIndependence(
		"example-judge-1", "example-fast-1", c,
		modelcatalog.IndependenceOpts{SharedHistory: true},
	)
	if !hasViolationKind(violations, "shared_history") {
		t.Errorf("want shared_history violation from opts, got %v", violations)
	}
}

func TestValidateIndependence_OptsDoNotAddViolationsWhenFalse(t *testing.T) {
	c := baseIndependenceCatalog(t)
	violations := modelcatalog.ValidateIndependence(
		"example-judge-1", "example-fast-1", c,
		modelcatalog.IndependenceOpts{SameSession: false, SharedHistory: false},
	)
	if len(violations) != 0 {
		t.Errorf("false opts must not add violations, got %v", violations)
	}
}

func TestValidateIndependence_Deterministic(t *testing.T) {
	c := baseIndependenceCatalog(t)
	// A pair with multiple violations: shared family + opts-asserted same_session
	// + opts-asserted shared_history.
	opts := modelcatalog.IndependenceOpts{SameSession: true, SharedHistory: true}
	got1 := modelcatalog.ValidateIndependence("example-fast-1", "example-fast-2", c, opts)
	got2 := modelcatalog.ValidateIndependence("example-fast-1", "example-fast-2", c, opts)
	if len(got1) != len(got2) {
		t.Fatalf("ValidateIndependence not deterministic: len %d vs %d", len(got1), len(got2))
	}
	for i := range got1 {
		if got1[i] != got2[i] {
			t.Errorf("ValidateIndependence not deterministic at index %d: %v vs %v", i, got1[i], got2[i])
		}
	}
	// Verify sorted by Kind then Detail.
	sorted := make([]modelcatalog.Violation, len(got1))
	copy(sorted, got1)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Kind != sorted[j].Kind {
			return sorted[i].Kind < sorted[j].Kind
		}
		return sorted[i].Detail < sorted[j].Detail
	})
	for i := range got1 {
		if got1[i] != sorted[i] {
			t.Errorf("ValidateIndependence output not sorted at index %d: got %v, want %v", i, got1[i], sorted[i])
		}
	}
}

func TestValidateIndependence_UnknownIDNeverSilentlyPasses(t *testing.T) {
	// Unknown model ID must always be a Violation; it is never silently passed.
	c := baseIndependenceCatalog(t)
	// Even with a clean opts, an unknown ID must produce at least one violation.
	violations := modelcatalog.ValidateIndependence("ghost", "example-judge-1", c, modelcatalog.IndependenceOpts{})
	if len(violations) == 0 {
		t.Error("unknown model ID must never silently pass; want at least one violation")
	}
}

// ---- small lineage edge cases -----------------------------------------------

func TestValidateIndependence_LineageOverlapBothSides(t *testing.T) {
	// model-a: family=X, lineage=[Z]
	// model-b: family=Y, lineage=[Z]
	// Both cite Z in lineage — shared lineage.
	data := catalogJSON(t, map[string]any{
		"example-model-a": map[string]any{
			"id": "example-model-a", "family": "X",
			"lineage": []string{"Z"},
			"caps": map[string]any{
				"thinking": false, "effort": false,
				"temperature": true, "seed": false, "structured_outputs": false,
			},
			"cost_tier": 0, "deprecated": false,
		},
		"example-model-b": map[string]any{
			"id": "example-model-b", "family": "Y",
			"lineage": []string{"Z"},
			"caps": map[string]any{
				"thinking": false, "effort": false,
				"temperature": true, "seed": false, "structured_outputs": false,
			},
			"cost_tier": 0, "deprecated": false,
		},
		// Z family must exist so there's no dangling lineage ref.
		"example-model-z": map[string]any{
			"id": "example-model-z", "family": "Z",
			"lineage": []string{},
			"caps": map[string]any{
				"thinking": false, "effort": false,
				"temperature": true, "seed": false, "structured_outputs": false,
			},
			"cost_tier": 0, "deprecated": false,
		},
	})
	c := mustLoad(t, data)
	violations := modelcatalog.ValidateIndependence("example-model-a", "example-model-b", c, modelcatalog.IndependenceOpts{})
	if !hasViolationKind(violations, "shared_lineage") {
		t.Errorf("models sharing a lineage entry must produce shared_lineage violation, got %v", violations)
	}
}

// ---- test helpers -----------------------------------------------------------

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

func hasProblemKind(problems []modelcatalog.Problem, kind string) bool {
	for _, p := range problems {
		if p.Kind == kind {
			return true
		}
	}
	return false
}

func hasViolationKind(violations []modelcatalog.Violation, kind string) bool {
	for _, v := range violations {
		if v.Kind == kind {
			return true
		}
	}
	return false
}
