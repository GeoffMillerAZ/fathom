package investigationtemplate_test

import (
	"testing"

	it "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/investigation-template"
)

func validTemplate() it.Template {
	return it.Template{
		Name:  "gap-analysis",
		Steps: []string{"scope-definition", "evidence-collection", "gap-identification", "remediation-proposal"},
	}
}

// TestRegisterAndGetHappyPath ensures a registered template can be retrieved.
func TestRegisterAndGetHappyPath(t *testing.T) {
	var r it.Registry
	tmpl := validTemplate()
	if err := r.Register(tmpl); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}
	got, ok := r.Get(tmpl.Name)
	if !ok {
		t.Fatal("expected to find template, got false")
	}
	if got.Name != tmpl.Name {
		t.Errorf("Name mismatch: want %q, got %q", tmpl.Name, got.Name)
	}
	if len(got.Steps) != len(tmpl.Steps) {
		t.Errorf("Steps length mismatch: want %d, got %d", len(tmpl.Steps), len(got.Steps))
	}
}

// TestGetMissing ensures Get returns false for an unregistered name.
func TestGetMissing(t *testing.T) {
	var r it.Registry
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected false for missing template, got true")
	}
}

// TestRegisterEmptyName ensures an empty Name is rejected.
func TestRegisterEmptyName(t *testing.T) {
	var r it.Registry
	tmpl := validTemplate()
	tmpl.Name = ""
	if err := r.Register(tmpl); err == nil {
		t.Fatal("expected error for empty Name, got nil")
	}
}

// TestRegisterEmptySteps ensures a template with no steps is rejected.
func TestRegisterEmptySteps(t *testing.T) {
	var r it.Registry
	tmpl := validTemplate()
	tmpl.Steps = nil
	if err := r.Register(tmpl); err == nil {
		t.Fatal("expected error for nil Steps, got nil")
	}
	tmpl.Steps = []string{}
	if err := r.Register(tmpl); err == nil {
		t.Fatal("expected error for empty Steps slice, got nil")
	}
}

// TestRegisterEmptyStepLabel ensures a blank step label within Steps is rejected.
func TestRegisterEmptyStepLabel(t *testing.T) {
	var r it.Registry
	tmpl := validTemplate()
	tmpl.Steps = []string{"step-one", "", "step-three"}
	if err := r.Register(tmpl); err == nil {
		t.Fatal("expected error for empty step label, got nil")
	}
}

// TestRegisterOverwrite ensures registering the same name twice overwrites.
func TestRegisterOverwrite(t *testing.T) {
	var r it.Registry
	tmpl1 := validTemplate()
	tmpl2 := it.Template{Name: "gap-analysis", Steps: []string{"new-step-a", "new-step-b"}}
	if err := r.Register(tmpl1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.Register(tmpl2); err != nil {
		t.Fatalf("unexpected error on overwrite: %v", err)
	}
	got, ok := r.Get("gap-analysis")
	if !ok {
		t.Fatal("template not found after overwrite")
	}
	if len(got.Steps) != 2 || got.Steps[0] != "new-step-a" {
		t.Errorf("expected overwritten steps, got %v", got.Steps)
	}
}

// TestRegisterDefensiveCopy ensures mutating the original Steps after
// registration does not affect the stored template.
func TestRegisterDefensiveCopy(t *testing.T) {
	var r it.Registry
	tmpl := validTemplate()
	if err := r.Register(tmpl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Mutate the original steps.
	tmpl.Steps[0] = "mutated"
	got, _ := r.Get(tmpl.Name)
	if got.Steps[0] == "mutated" {
		t.Error("Registry stored a reference to the input slice instead of a copy")
	}
}
