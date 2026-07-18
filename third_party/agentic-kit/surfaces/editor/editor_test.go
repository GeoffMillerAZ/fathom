package editor_test

import (
	"testing"

	"github.com/geoffmilleraz/agentic-kit/surfaces/editor"
	"github.com/geoffmilleraz/agentic-kit/surfaces/forms/cueform"
)

// mustCompose is a test helper that compiles a FormDefinition or fails the test.
func mustCompose(t *testing.T, fd cueform.FormDefinition) cueform.ResolvedForm {
	t.Helper()
	rf, err := cueform.Compose(fd)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	return rf
}

// ── BuildFieldModel: flat form ────────────────────────────────────────────────

func TestBuildFieldModel_FlatForm(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "flat",
		Title: "Flat Form",
		Fields: []cueform.FieldDefinition{
			{Key: "name", Kind: cueform.FieldKindText, Label: "Name", Required: true},
			{Key: "age", Kind: cueform.FieldKindInteger, Label: "Age"},
			{Key: "bio", Kind: cueform.FieldKindLongText, Label: "Bio"},
		},
	})

	models := editor.BuildFieldModel(rf)

	if len(models) != 3 {
		t.Fatalf("expected 3 field models, got %d", len(models))
	}

	// first field
	m := models[0]
	if m.Path != "name" {
		t.Errorf("expected path=name, got %q", m.Path)
	}
	if m.Key != "name" {
		t.Errorf("expected key=name, got %q", m.Key)
	}
	if m.Kind != editor.FieldKindText {
		t.Errorf("expected kind=text, got %q", m.Kind)
	}
	if m.Label != "Name" {
		t.Errorf("expected label=Name, got %q", m.Label)
	}
	if !m.Required {
		t.Error("expected required=true for name field")
	}
	if m.Depth != 0 {
		t.Errorf("expected depth=0 for top-level field, got %d", m.Depth)
	}

	// second field
	if models[1].Kind != editor.FieldKindInteger {
		t.Errorf("expected kind=integer, got %q", models[1].Kind)
	}
	if models[2].Kind != editor.FieldKindLongText {
		t.Errorf("expected kind=longtext, got %q", models[2].Kind)
	}
}

// ── BuildFieldModel: group children ──────────────────────────────────────────

func TestBuildFieldModel_GroupChildren(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "grouped",
		Title: "Grouped Form",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "address",
				Kind:  cueform.FieldKindGroup,
				Label: "Address",
				Fields: []cueform.FieldDefinition{
					{Key: "street", Kind: cueform.FieldKindText, Label: "Street"},
					{Key: "city", Kind: cueform.FieldKindText, Label: "City"},
				},
			},
			{Key: "note", Kind: cueform.FieldKindText, Label: "Note"},
		},
	})

	models := editor.BuildFieldModel(rf)

	// Expect: address(0), street(1), city(1), note(0)
	if len(models) != 4 {
		t.Fatalf("expected 4 models, got %d", len(models))
	}

	paths := make([]string, len(models))
	depths := make([]int, len(models))
	for i, m := range models {
		paths[i] = m.Path
		depths[i] = m.Depth
	}

	wantPaths := []string{"address", "address.street", "address.city", "note"}
	for i, want := range wantPaths {
		if paths[i] != want {
			t.Errorf("models[%d]: expected path=%q, got %q", i, want, paths[i])
		}
	}

	wantDepths := []int{0, 1, 1, 0}
	for i, want := range wantDepths {
		if depths[i] != want {
			t.Errorf("models[%d]: expected depth=%d, got %d", i, want, depths[i])
		}
	}
}

// ── BuildFieldModel: list item template ──────────────────────────────────────

func TestBuildFieldModel_ListItem(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "listed",
		Title: "Listed Form",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "tags",
				Kind:  cueform.FieldKindList,
				Label: "Tags",
				Item:  &cueform.FieldDefinition{Key: "tag", Kind: cueform.FieldKindText, Label: "Tag"},
			},
		},
	})

	models := editor.BuildFieldModel(rf)

	// Expect: tags(depth=0, isListItem=false), tag(depth=1, isListItem=true)
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	list := models[0]
	if list.Kind != editor.FieldKindList {
		t.Errorf("expected kind=list, got %q", list.Kind)
	}
	if list.IsListItem {
		t.Error("expected IsListItem=false for list field itself")
	}
	if list.Depth != 0 {
		t.Errorf("expected depth=0 for list, got %d", list.Depth)
	}

	item := models[1]
	if item.Key != "tag" {
		t.Errorf("expected item key=tag, got %q", item.Key)
	}
	if !item.IsListItem {
		t.Error("expected IsListItem=true for list item template")
	}
	if item.Depth != 1 {
		t.Errorf("expected item depth=1, got %d", item.Depth)
	}
}

// ── BuildFieldModel: select options ──────────────────────────────────────────

func TestBuildFieldModel_SelectOptions(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "select-form",
		Title: "Select Form",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "priority",
				Kind:  cueform.FieldKindSelect,
				Label: "Priority",
				Options: []cueform.FieldOption{
					{Value: "low", Label: "Low"},
					{Value: "high", Label: "High"},
				},
			},
		},
	})

	models := editor.BuildFieldModel(rf)

	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	m := models[0]
	if len(m.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(m.Options))
	}
	if m.Options[0].Value != "low" {
		t.Errorf("expected option[0].Value=low, got %q", m.Options[0].Value)
	}
	if m.Options[1].Value != "high" {
		t.Errorf("expected option[1].Value=high, got %q", m.Options[1].Value)
	}
}

// ── BuildFieldModel: placeholder precedence ───────────────────────────────────

func TestBuildFieldModel_PlaceholderPrecedence(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "placeholder-form",
		Title: "Placeholder Form",
		Fields: []cueform.FieldDefinition{
			{Key: "with_example", Kind: cueform.FieldKindText, Label: "Ex", Example: "ex-val", Prefill: "pre-val"},
			{Key: "with_prefill", Kind: cueform.FieldKindText, Label: "Pre", Prefill: "pre-val"},
			{Key: "no_hint", Kind: cueform.FieldKindText, Label: "None"},
		},
	})

	models := editor.BuildFieldModel(rf)

	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}
	if models[0].Placeholder != "ex-val" {
		t.Errorf("expected Placeholder=ex-val (example wins), got %v", models[0].Placeholder)
	}
	if models[1].Placeholder != "pre-val" {
		t.Errorf("expected Placeholder=pre-val (prefill fallback), got %v", models[1].Placeholder)
	}
	if models[2].Placeholder != nil {
		t.Errorf("expected Placeholder=nil, got %v", models[2].Placeholder)
	}
}

// ── BuildFieldModel: HasConditions and HasValidation flags ───────────────────

func TestBuildFieldModel_MetaFlags(t *testing.T) {
	min3 := 3
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "meta-form",
		Title: "Meta Form",
		Fields: []cueform.FieldDefinition{
			{Key: "trigger", Kind: cueform.FieldKindBoolean, Label: "Trigger"},
			{
				Key:   "conditional",
				Kind:  cueform.FieldKindText,
				Label: "Conditional",
				Conditions: []cueform.Condition{
					{FieldRef: "trigger", Operator: cueform.ConditionEquals, Value: true},
				},
			},
			{
				Key:        "validated",
				Kind:       cueform.FieldKindText,
				Label:      "Validated",
				Validation: &cueform.FieldValidation{MinLength: &min3},
			},
			{Key: "plain", Kind: cueform.FieldKindText, Label: "Plain"},
		},
	})

	models := editor.BuildFieldModel(rf)

	if len(models) != 4 {
		t.Fatalf("expected 4 models, got %d", len(models))
	}

	// trigger: no conditions, no validation
	if models[0].HasConditions {
		t.Error("expected HasConditions=false for trigger")
	}
	if models[0].HasValidation {
		t.Error("expected HasValidation=false for trigger")
	}

	// conditional: has conditions
	if !models[1].HasConditions {
		t.Error("expected HasConditions=true for conditional field")
	}

	// validated: has validation
	if !models[2].HasValidation {
		t.Error("expected HasValidation=true for validated field")
	}

	// plain: neither
	if models[3].HasConditions || models[3].HasValidation {
		t.Error("expected both flags false for plain field")
	}
}

// ── BuildFieldModel: default value carried through ───────────────────────────

func TestBuildFieldModel_DefaultValue(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "default-form",
		Title: "Default Form",
		Fields: []cueform.FieldDefinition{
			{Key: "env", Kind: cueform.FieldKindText, Label: "Env", Default: "production"},
			{Key: "count", Kind: cueform.FieldKindInteger, Label: "Count"},
		},
	})

	models := editor.BuildFieldModel(rf)

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].DefaultValue != "production" {
		t.Errorf("expected DefaultValue=production, got %v", models[0].DefaultValue)
	}
	if models[1].DefaultValue != nil {
		t.Errorf("expected DefaultValue=nil, got %v", models[1].DefaultValue)
	}
}

// ── BuildFieldModel: determinism ─────────────────────────────────────────────

func TestBuildFieldModel_Determinism(t *testing.T) {
	fd := cueform.FormDefinition{
		ID:    "det-form",
		Title: "Determinism Form",
		Fields: []cueform.FieldDefinition{
			{Key: "alpha", Kind: cueform.FieldKindText, Label: "Alpha"},
			{
				Key:   "group",
				Kind:  cueform.FieldKindGroup,
				Label: "Group",
				Fields: []cueform.FieldDefinition{
					{Key: "beta", Kind: cueform.FieldKindText, Label: "Beta"},
					{Key: "gamma", Kind: cueform.FieldKindText, Label: "Gamma"},
				},
			},
			{
				Key:   "items",
				Kind:  cueform.FieldKindList,
				Label: "Items",
				Item:  &cueform.FieldDefinition{Key: "item", Kind: cueform.FieldKindText, Label: "Item"},
			},
		},
	}

	rf, err := cueform.Compose(fd)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	first := editor.BuildFieldModel(rf)
	second := editor.BuildFieldModel(rf)

	if len(first) != len(second) {
		t.Fatalf("non-deterministic: first=%d models, second=%d models", len(first), len(second))
	}
	for i := range first {
		if first[i].Path != second[i].Path {
			t.Errorf("models[%d] path mismatch: %q vs %q", i, first[i].Path, second[i].Path)
		}
		if first[i].Depth != second[i].Depth {
			t.Errorf("models[%d] depth mismatch: %d vs %d", i, first[i].Depth, second[i].Depth)
		}
		if first[i].IsListItem != second[i].IsListItem {
			t.Errorf("models[%d] IsListItem mismatch", i)
		}
	}
}

// ── BuildFieldModel: nested group children have correct depth ─────────────────

func TestBuildFieldModel_NestedGroupDepth(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "nested",
		Title: "Nested Groups",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "outer",
				Kind:  cueform.FieldKindGroup,
				Label: "Outer",
				Fields: []cueform.FieldDefinition{
					{
						Key:   "inner",
						Kind:  cueform.FieldKindGroup,
						Label: "Inner",
						Fields: []cueform.FieldDefinition{
							{Key: "leaf", Kind: cueform.FieldKindText, Label: "Leaf"},
						},
					},
				},
			},
		},
	})

	models := editor.BuildFieldModel(rf)

	// outer(0), inner(1), leaf(2)
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}
	if models[0].Depth != 0 {
		t.Errorf("outer: expected depth=0, got %d", models[0].Depth)
	}
	if models[1].Depth != 1 {
		t.Errorf("inner: expected depth=1, got %d", models[1].Depth)
	}
	if models[2].Depth != 2 {
		t.Errorf("leaf: expected depth=2, got %d", models[2].Depth)
	}
}

// ── BuildFieldModel: empty form returns empty slice ───────────────────────────

func TestBuildFieldModel_EmptyFields(t *testing.T) {
	// Compose a minimal valid form with a single field, then pass an empty
	// ResolvedForm directly to confirm no panic and empty result.
	rf := cueform.ResolvedForm{
		ID:     "empty",
		Title:  "Empty",
		Fields: nil,
	}
	models := editor.BuildFieldModel(rf)
	if len(models) != 0 {
		t.Errorf("expected 0 models for empty form, got %d", len(models))
	}
}
