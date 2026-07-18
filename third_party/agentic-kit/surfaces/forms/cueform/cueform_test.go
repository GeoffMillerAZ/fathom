package cueform_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/surfaces/forms/cueform"
)

// helpers

func intPtr(v int) *int         { return &v }
func f64Ptr(v float64) *float64 { return &v }

// yesNoOptions is a reusable select option list.
var yesNoOptions = []cueform.FieldOption{
	{Value: "yes", Label: "Yes"},
	{Value: "no", Label: "No"},
}

// colorOptions is a reusable multiselect option list.
var colorOptions = []cueform.FieldOption{
	{Value: "red", Label: "Red"},
	{Value: "green", Label: "Green"},
	{Value: "blue", Label: "Blue"},
}

// ── Compose tests ────────────────────────────────────────────────────────────

func TestCompose_ResolvesTopLevelPaths(t *testing.T) {
	form := cueform.FormDefinition{
		ID:    "f1",
		Title: "Form 1",
		Fields: []cueform.FieldDefinition{
			{Key: "name", Kind: cueform.FieldKindText, Label: "Name"},
			{Key: "age", Kind: cueform.FieldKindInteger, Label: "Age"},
		},
	}
	rf, err := cueform.Compose(form)
	if err != nil {
		t.Fatalf("Compose error: %v", err)
	}
	if len(rf.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(rf.Fields))
	}
	if rf.Fields[0].Path != "name" {
		t.Errorf("expected path %q, got %q", "name", rf.Fields[0].Path)
	}
	if rf.Fields[1].Path != "age" {
		t.Errorf("expected path %q, got %q", "age", rf.Fields[1].Path)
	}
}

func TestCompose_ResolvesGroupChildPaths(t *testing.T) {
	form := cueform.FormDefinition{
		ID:    "f2",
		Title: "Form 2",
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
		},
	}
	rf, err := cueform.Compose(form)
	if err != nil {
		t.Fatalf("Compose error: %v", err)
	}
	if len(rf.Fields) != 1 {
		t.Fatalf("expected 1 top-level field, got %d", len(rf.Fields))
	}
	group := rf.Fields[0]
	if group.Path != "address" {
		t.Errorf("expected group path %q, got %q", "address", group.Path)
	}
	if len(group.Fields) != 2 {
		t.Fatalf("expected 2 child fields, got %d", len(group.Fields))
	}
	if group.Fields[0].Path != "address.street" {
		t.Errorf("expected child path %q, got %q", "address.street", group.Fields[0].Path)
	}
	if group.Fields[1].Path != "address.city" {
		t.Errorf("expected child path %q, got %q", "address.city", group.Fields[1].Path)
	}
}

func TestCompose_ResolvesListItemPath(t *testing.T) {
	form := cueform.FormDefinition{
		ID:    "f3",
		Title: "Form 3",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "tags",
				Kind:  cueform.FieldKindList,
				Label: "Tags",
				Item:  &cueform.FieldDefinition{Key: "tag", Kind: cueform.FieldKindText, Label: "Tag"},
			},
		},
	}
	rf, err := cueform.Compose(form)
	if err != nil {
		t.Fatalf("Compose error: %v", err)
	}
	list := rf.Fields[0]
	if list.Item == nil {
		t.Fatal("expected Item to be resolved")
	}
	// list item path uses "[]" separator
	if !strings.Contains(list.Item.Path, "[]") {
		t.Errorf("expected list item path to contain '[]', got %q", list.Item.Path)
	}
}

func TestCompose_QualifiesBareconditionRef(t *testing.T) {
	form := cueform.FormDefinition{
		ID:    "f4",
		Title: "Form 4",
		Fields: []cueform.FieldDefinition{
			{Key: "flag", Kind: cueform.FieldKindBoolean, Label: "Flag"},
			{
				Key:   "detail",
				Kind:  cueform.FieldKindText,
				Label: "Detail",
				Conditions: []cueform.Condition{
					{FieldRef: "flag", Operator: cueform.ConditionEquals, Value: true},
				},
			},
		},
	}
	rf, err := cueform.Compose(form)
	if err != nil {
		t.Fatalf("Compose error: %v", err)
	}
	detail := rf.Fields[1]
	if len(detail.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(detail.Conditions))
	}
	// bare "flag" at top level has no container path, so should stay "flag"
	if detail.Conditions[0].FieldRef != "flag" {
		t.Errorf("expected condition FieldRef %q, got %q", "flag", detail.Conditions[0].FieldRef)
	}
}

func TestCompose_QualifiesBareconditionRefInGroup(t *testing.T) {
	form := cueform.FormDefinition{
		ID:    "f5",
		Title: "Form 5",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "section",
				Kind:  cueform.FieldKindGroup,
				Label: "Section",
				Fields: []cueform.FieldDefinition{
					{Key: "flag", Kind: cueform.FieldKindBoolean, Label: "Flag"},
					{
						Key:   "detail",
						Kind:  cueform.FieldKindText,
						Label: "Detail",
						Conditions: []cueform.Condition{
							{FieldRef: "flag", Operator: cueform.ConditionEquals, Value: true},
						},
					},
				},
			},
		},
	}
	rf, err := cueform.Compose(form)
	if err != nil {
		t.Fatalf("Compose error: %v", err)
	}
	detail := rf.Fields[0].Fields[1]
	if len(detail.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(detail.Conditions))
	}
	// bare "flag" inside group "section" should be qualified to "section.flag"
	if detail.Conditions[0].FieldRef != "section.flag" {
		t.Errorf("expected condition FieldRef %q, got %q", "section.flag", detail.Conditions[0].FieldRef)
	}
}

func TestCompose_RejectsDuplicatePaths(t *testing.T) {
	form := cueform.FormDefinition{
		ID:    "dup",
		Title: "Dup Form",
		Fields: []cueform.FieldDefinition{
			{Key: "name", Kind: cueform.FieldKindText, Label: "Name"},
			{Key: "name", Kind: cueform.FieldKindText, Label: "Name Again"},
		},
	}
	_, err := cueform.Compose(form)
	if err == nil {
		t.Fatal("expected error for duplicate path, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected 'duplicate' in error, got: %v", err)
	}
}

var composeRejectCases = []struct {
	name   string
	form   cueform.FormDefinition
	errSub string
}{
	{
		name: "missing form id",
		form: cueform.FormDefinition{
			Title:  "T",
			Fields: []cueform.FieldDefinition{{Key: "k", Kind: cueform.FieldKindText, Label: "L"}},
		},
		errSub: "form id",
	},
	{
		name: "missing form title",
		form: cueform.FormDefinition{
			ID:     "id",
			Fields: []cueform.FieldDefinition{{Key: "k", Kind: cueform.FieldKindText, Label: "L"}},
		},
		errSub: "title",
	},
	{
		name: "empty fields",
		form: cueform.FormDefinition{
			ID:    "id",
			Title: "T",
		},
		errSub: "field",
	},
	{
		name: "missing field key",
		form: cueform.FormDefinition{
			ID:    "id",
			Title: "T",
			Fields: []cueform.FieldDefinition{
				{Kind: cueform.FieldKindText, Label: "L"},
			},
		},
		errSub: "key",
	},
	{
		name: "missing field label",
		form: cueform.FormDefinition{
			ID:    "id",
			Title: "T",
			Fields: []cueform.FieldDefinition{
				{Key: "k", Kind: cueform.FieldKindText},
			},
		},
		errSub: "label",
	},
	{
		name: "unknown kind",
		form: cueform.FormDefinition{
			ID:    "id",
			Title: "T",
			Fields: []cueform.FieldDefinition{
				{Key: "k", Kind: "widget", Label: "L"},
			},
		},
		errSub: "unsupported kind",
	},
	{
		name: "list without item",
		form: cueform.FormDefinition{
			ID:    "id",
			Title: "T",
			Fields: []cueform.FieldDefinition{
				{Key: "lst", Kind: cueform.FieldKindList, Label: "L"},
			},
		},
		errSub: "must define an item",
	},
	{
		name: "group without children",
		form: cueform.FormDefinition{
			ID:    "id",
			Title: "T",
			Fields: []cueform.FieldDefinition{
				{Key: "grp", Kind: cueform.FieldKindGroup, Label: "L"},
			},
		},
		errSub: "must define nested fields",
	},
	{
		name: "select without options",
		form: cueform.FormDefinition{
			ID:    "id",
			Title: "T",
			Fields: []cueform.FieldDefinition{
				{Key: "sel", Kind: cueform.FieldKindSelect, Label: "L"},
			},
		},
		errSub: "option",
	},
	{
		name: "non-list with item",
		form: cueform.FormDefinition{
			ID:    "id",
			Title: "T",
			Fields: []cueform.FieldDefinition{
				{
					Key:   "txt",
					Kind:  cueform.FieldKindText,
					Label: "L",
					Item:  &cueform.FieldDefinition{Key: "x", Kind: cueform.FieldKindText, Label: "X"},
				},
			},
		},
		errSub: "kind=list",
	},
	{
		name: "non-group with children",
		form: cueform.FormDefinition{
			ID:    "id",
			Title: "T",
			Fields: []cueform.FieldDefinition{
				{
					Key:    "txt",
					Kind:   cueform.FieldKindText,
					Label:  "L",
					Fields: []cueform.FieldDefinition{{Key: "c", Kind: cueform.FieldKindText, Label: "C"}},
				},
			},
		},
		errSub: "kind=group",
	},
	{
		name: "condition with empty field_ref",
		form: cueform.FormDefinition{
			ID:    "id",
			Title: "T",
			Fields: []cueform.FieldDefinition{
				{
					Key:   "f",
					Kind:  cueform.FieldKindText,
					Label: "F",
					Conditions: []cueform.Condition{
						{FieldRef: "", Operator: cueform.ConditionEquals, Value: "x"},
					},
				},
			},
		},
		errSub: "field_ref",
	},
}

func TestCompose_RejectsBadDefinitions(t *testing.T) {
	for _, tc := range composeRejectCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cueform.Compose(tc.form)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errSub)
			}
			if !strings.Contains(err.Error(), tc.errSub) {
				t.Errorf("expected error containing %q, got: %v", tc.errSub, err)
			}
		})
	}
}

// ── Draft response tests ─────────────────────────────────────────────────────

func TestNewDraftResponse_SeedsRequiredFields(t *testing.T) {
	form := cueform.FormDefinition{
		ID:    "draft-test",
		Title: "Draft Test",
		Fields: []cueform.FieldDefinition{
			{Key: "name", Kind: cueform.FieldKindText, Label: "Name", Required: true},
			{Key: "bio", Kind: cueform.FieldKindLongText, Label: "Bio", Required: true},
			{Key: "age", Kind: cueform.FieldKindInteger, Label: "Age", Required: true},
			{Key: "score", Kind: cueform.FieldKindNumber, Label: "Score", Required: true},
			{Key: "active", Kind: cueform.FieldKindBoolean, Label: "Active", Required: true},
			{
				Key:      "status",
				Kind:     cueform.FieldKindSelect,
				Label:    "Status",
				Required: true,
				Options:  yesNoOptions,
			},
			{
				Key:      "tags",
				Kind:     cueform.FieldKindMultiSelect,
				Label:    "Tags",
				Required: true,
				Options:  colorOptions,
			},
			{
				Key:      "items",
				Kind:     cueform.FieldKindList,
				Label:    "Items",
				Required: true,
				Item:     &cueform.FieldDefinition{Key: "item", Kind: cueform.FieldKindText, Label: "Item"},
			},
		},
	}
	rf, err := cueform.Compose(form)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	rec := cueform.NewDraftResponse(rf)
	v := rec.Values
	if v["name"] != "" {
		t.Errorf("expected name=%q, got %v", "", v["name"])
	}
	if v["bio"] != "" {
		t.Errorf("expected bio=%q, got %v", "", v["bio"])
	}
	if v["age"] != 0 {
		t.Errorf("expected age=0, got %v", v["age"])
	}
	if v["score"] != 0.0 {
		t.Errorf("expected score=0.0, got %v", v["score"])
	}
	if v["active"] != false {
		t.Errorf("expected active=false, got %v", v["active"])
	}
	if v["status"] != "" {
		t.Errorf("expected status=%q, got %v", "", v["status"])
	}
	if _, ok := v["tags"]; !ok {
		t.Error("expected tags to be present")
	}
	if _, ok := v["items"]; !ok {
		t.Error("expected items to be present")
	}
}

func TestNewDraftResponse_OmitsOptionalFields(t *testing.T) {
	form := cueform.FormDefinition{
		ID:    "draft-optional",
		Title: "Draft Optional",
		Fields: []cueform.FieldDefinition{
			{Key: "required_name", Kind: cueform.FieldKindText, Label: "Name", Required: true},
			{Key: "optional_note", Kind: cueform.FieldKindText, Label: "Note"},
		},
	}
	rf, err := cueform.Compose(form)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	rec := cueform.NewDraftResponse(rf)
	if _, ok := rec.Values["optional_note"]; ok {
		t.Error("optional field without prefill/default should not appear in draft")
	}
	if _, ok := rec.Values["required_name"]; !ok {
		t.Error("required field should appear in draft")
	}
}

func TestNewDraftResponse_UsesPrefillOverDefault(t *testing.T) {
	form := cueform.FormDefinition{
		ID:    "prefill-test",
		Title: "Prefill Test",
		Fields: []cueform.FieldDefinition{
			{
				Key:     "city",
				Kind:    cueform.FieldKindText,
				Label:   "City",
				Default: "London",
				Prefill: "Paris",
			},
		},
	}
	rf, err := cueform.Compose(form)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	rec := cueform.NewDraftResponse(rf)
	if rec.Values["city"] != "Paris" {
		t.Errorf("expected city=Paris (prefill), got %v", rec.Values["city"])
	}
}

func TestNewDraftResponse_FormRecordMetadata(t *testing.T) {
	form := cueform.FormDefinition{
		ID:    "meta-form",
		Title: "Meta Form",
		Fields: []cueform.FieldDefinition{
			{Key: "f", Kind: cueform.FieldKindText, Label: "F"},
		},
	}
	rf, err := cueform.Compose(form)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	rec := cueform.NewDraftResponse(rf)
	if rec.FormID != "meta-form" {
		t.Errorf("expected FormID=meta-form, got %q", rec.FormID)
	}
	if rec.Status != cueform.ResponseStatusDraft {
		t.Errorf("expected Status=draft, got %q", rec.Status)
	}
	if !strings.Contains(rec.ResponseID, "draft") {
		t.Errorf("expected ResponseID to contain 'draft', got %q", rec.ResponseID)
	}
	if rec.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

// ── Validate tests ───────────────────────────────────────────────────────────

func mustCompose(t *testing.T, form cueform.FormDefinition) cueform.ResolvedForm {
	t.Helper()
	rf, err := cueform.Compose(form)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	return rf
}

func TestValidate_AcceptsValidResponse(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v1",
		Title: "V1",
		Fields: []cueform.FieldDefinition{
			{Key: "name", Kind: cueform.FieldKindText, Label: "Name", Required: true},
		},
	})
	report := cueform.Validate(rf, cueform.Response{"name": "Alice"})
	if !report.Valid {
		t.Errorf("expected Valid=true; missing=%v issues=%v", report.MissingRequired, report.Issues)
	}
}

func TestValidate_RejectsMissingRequired(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v2",
		Title: "V2",
		Fields: []cueform.FieldDefinition{
			{Key: "name", Kind: cueform.FieldKindText, Label: "Name", Required: true},
		},
	})
	report := cueform.Validate(rf, cueform.Response{})
	if report.Valid {
		t.Error("expected Valid=false for missing required field")
	}
	if len(report.MissingRequired) != 1 || report.MissingRequired[0] != "name" {
		t.Errorf("expected MissingRequired=[name], got %v", report.MissingRequired)
	}
}

func TestValidate_RejectsWrongTypeString(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v3",
		Title: "V3",
		Fields: []cueform.FieldDefinition{
			{Key: "name", Kind: cueform.FieldKindText, Label: "Name"},
		},
	})
	report := cueform.Validate(rf, cueform.Response{"name": 42})
	if report.Valid {
		t.Error("expected Valid=false for wrong type")
	}
	if len(report.Issues) == 0 {
		t.Error("expected type issue")
	}
	if report.Issues[0].Code != cueform.ValidationIssueType {
		t.Errorf("expected ValidationIssueType, got %q", report.Issues[0].Code)
	}
}

func TestValidate_RejectsWrongTypeBool(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-bool",
		Title: "Bool",
		Fields: []cueform.FieldDefinition{
			{Key: "active", Kind: cueform.FieldKindBoolean, Label: "Active"},
		},
	})
	report := cueform.Validate(rf, cueform.Response{"active": "yes"})
	if report.Valid {
		t.Error("expected Valid=false for string where bool expected")
	}
}

func TestValidate_RejectsFloatForInteger(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-int",
		Title: "Int",
		Fields: []cueform.FieldDefinition{
			{Key: "count", Kind: cueform.FieldKindInteger, Label: "Count"},
		},
	})
	report := cueform.Validate(rf, cueform.Response{"count": 3.14})
	if report.Valid {
		t.Error("expected Valid=false for float where integer expected")
	}
	if len(report.Issues) == 0 || report.Issues[0].Code != cueform.ValidationIssueType {
		t.Errorf("expected type issue, got %v", report.Issues)
	}
}

func TestValidate_AcceptsIntegerAsNumber(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-num",
		Title: "Num",
		Fields: []cueform.FieldDefinition{
			{Key: "score", Kind: cueform.FieldKindNumber, Label: "Score"},
		},
	})
	// int should be accepted as a number
	report := cueform.Validate(rf, cueform.Response{"score": 42})
	if !report.Valid {
		t.Errorf("expected Valid=true for int as number, got issues: %v", report.Issues)
	}
}

func TestValidate_RejectsInvalidSelectOption(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-sel",
		Title: "Sel",
		Fields: []cueform.FieldDefinition{
			{Key: "choice", Kind: cueform.FieldKindSelect, Label: "Choice", Options: yesNoOptions},
		},
	})
	report := cueform.Validate(rf, cueform.Response{"choice": "maybe"})
	if report.Valid {
		t.Error("expected Valid=false for invalid option")
	}
	if len(report.Issues) == 0 || report.Issues[0].Code != cueform.ValidationIssueConstraint {
		t.Errorf("expected constraint issue, got %v", report.Issues)
	}
}

func TestValidate_RejectsNonListForListField(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-list",
		Title: "List",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "tags",
				Kind:  cueform.FieldKindList,
				Label: "Tags",
				Item:  &cueform.FieldDefinition{Key: "tag", Kind: cueform.FieldKindText, Label: "Tag"},
			},
		},
	})
	report := cueform.Validate(rf, cueform.Response{"tags": "not-a-list"})
	if report.Valid {
		t.Error("expected Valid=false for non-list value on list field")
	}
	if len(report.Issues) == 0 || report.Issues[0].Code != cueform.ValidationIssueType {
		t.Errorf("expected type issue, got %v", report.Issues)
	}
}

func TestValidate_RejectsBadListItem(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-listitem",
		Title: "ListItem",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "counts",
				Kind:  cueform.FieldKindList,
				Label: "Counts",
				Item:  &cueform.FieldDefinition{Key: "count", Kind: cueform.FieldKindInteger, Label: "Count"},
			},
		},
	})
	// pass a float where integer is required
	report := cueform.Validate(rf, cueform.Response{
		"counts": []any{1, 2, 3.5},
	})
	if report.Valid {
		t.Error("expected Valid=false for float list item in integer list")
	}
}

func TestValidate_AcceptsValidListItems(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-listok",
		Title: "ListOK",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "names",
				Kind:  cueform.FieldKindList,
				Label: "Names",
				Item:  &cueform.FieldDefinition{Key: "name", Kind: cueform.FieldKindText, Label: "Name"},
			},
		},
	})
	report := cueform.Validate(rf, cueform.Response{
		"names": []any{"Alice", "Bob"},
	})
	if !report.Valid {
		t.Errorf("expected Valid=true; issues: %v", report.Issues)
	}
}

func TestValidate_SkipsInvisibleFields(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-cond",
		Title: "Conditional",
		Fields: []cueform.FieldDefinition{
			{Key: "show_detail", Kind: cueform.FieldKindBoolean, Label: "Show?"},
			{
				Key:      "detail",
				Kind:     cueform.FieldKindText,
				Label:    "Detail",
				Required: true,
				Conditions: []cueform.Condition{
					{FieldRef: "show_detail", Operator: cueform.ConditionEquals, Value: true},
				},
			},
		},
	})
	// show_detail is false (absent), so detail should be invisible and not required
	report := cueform.Validate(rf, cueform.Response{"show_detail": false})
	if !report.Valid {
		t.Errorf("expected Valid=true (invisible required field should be skipped); got: missing=%v issues=%v",
			report.MissingRequired, report.Issues)
	}
}

func TestValidate_ValidatesVisibleConditionalField(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-cond2",
		Title: "Conditional2",
		Fields: []cueform.FieldDefinition{
			{Key: "show_detail", Kind: cueform.FieldKindBoolean, Label: "Show?"},
			{
				Key:      "detail",
				Kind:     cueform.FieldKindText,
				Label:    "Detail",
				Required: true,
				Conditions: []cueform.Condition{
					{FieldRef: "show_detail", Operator: cueform.ConditionEquals, Value: true},
				},
			},
		},
	})
	// show_detail is true, so detail is visible and required
	report := cueform.Validate(rf, cueform.Response{"show_detail": true})
	if report.Valid {
		t.Error("expected Valid=false (visible required field is missing)")
	}
	found := false
	for _, p := range report.MissingRequired {
		if p == "detail" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'detail' in MissingRequired, got %v", report.MissingRequired)
	}
}

func TestValidate_StringLengthConstraints(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-strlen",
		Title: "StrLen",
		Fields: []cueform.FieldDefinition{
			{
				Key:        "code",
				Kind:       cueform.FieldKindText,
				Label:      "Code",
				Validation: &cueform.FieldValidation{MinLength: intPtr(3), MaxLength: intPtr(5)},
			},
		},
	})
	// too short
	r1 := cueform.Validate(rf, cueform.Response{"code": "ab"})
	if r1.Valid {
		t.Error("expected invalid for too-short string")
	}
	// too long
	r2 := cueform.Validate(rf, cueform.Response{"code": "abcdef"})
	if r2.Valid {
		t.Error("expected invalid for too-long string")
	}
	// just right
	r3 := cueform.Validate(rf, cueform.Response{"code": "abc"})
	if !r3.Valid {
		t.Errorf("expected valid for within-range string; issues: %v", r3.Issues)
	}
}

func TestValidate_NumberRangeConstraints(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-numrange",
		Title: "NumRange",
		Fields: []cueform.FieldDefinition{
			{
				Key:        "score",
				Kind:       cueform.FieldKindNumber,
				Label:      "Score",
				Validation: &cueform.FieldValidation{MinValue: f64Ptr(0.0), MaxValue: f64Ptr(100.0)},
			},
		},
	})
	r1 := cueform.Validate(rf, cueform.Response{"score": -1.0})
	if r1.Valid {
		t.Error("expected invalid for below-min number")
	}
	r2 := cueform.Validate(rf, cueform.Response{"score": 101.0})
	if r2.Valid {
		t.Error("expected invalid for above-max number")
	}
	r3 := cueform.Validate(rf, cueform.Response{"score": 50.0})
	if !r3.Valid {
		t.Errorf("expected valid for in-range number; issues: %v", r3.Issues)
	}
}

func TestValidate_GroupChildrenValidated(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-group",
		Title: "Group",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "addr",
				Kind:  cueform.FieldKindGroup,
				Label: "Address",
				Fields: []cueform.FieldDefinition{
					{Key: "street", Kind: cueform.FieldKindText, Label: "Street", Required: true},
					{Key: "city", Kind: cueform.FieldKindText, Label: "City", Required: true},
				},
			},
		},
	})
	// missing both group children
	r := cueform.Validate(rf, cueform.Response{})
	if r.Valid {
		t.Error("expected invalid: required group children missing")
	}
	hasStreet, hasCity := false, false
	for _, p := range r.MissingRequired {
		if p == "addr.street" {
			hasStreet = true
		}
		if p == "addr.city" {
			hasCity = true
		}
	}
	if !hasStreet || !hasCity {
		t.Errorf("expected both addr.street and addr.city missing, got %v", r.MissingRequired)
	}
}

func TestValidate_GroupListItemFields(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-grouplist",
		Title: "GroupList",
		Fields: []cueform.FieldDefinition{
			{
				Key:   "contacts",
				Kind:  cueform.FieldKindList,
				Label: "Contacts",
				Item: &cueform.FieldDefinition{
					Key:   "contact",
					Kind:  cueform.FieldKindGroup,
					Label: "Contact",
					Fields: []cueform.FieldDefinition{
						{Key: "name", Kind: cueform.FieldKindText, Label: "Name", Required: true},
						{Key: "email", Kind: cueform.FieldKindText, Label: "Email"},
					},
				},
			},
		},
	})
	// item missing required "name"
	r := cueform.Validate(rf, cueform.Response{
		"contacts": []any{
			map[string]any{"email": "alice@example.com"},
		},
	})
	if r.Valid {
		t.Error("expected invalid: required group list item field missing")
	}
}

func TestValidate_MinItemsConstraint(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-minitems",
		Title: "MinItems",
		Fields: []cueform.FieldDefinition{
			{
				Key:        "tags",
				Kind:       cueform.FieldKindList,
				Label:      "Tags",
				Item:       &cueform.FieldDefinition{Key: "tag", Kind: cueform.FieldKindText, Label: "Tag"},
				Validation: &cueform.FieldValidation{MinItems: intPtr(2)},
			},
		},
	})
	r1 := cueform.Validate(rf, cueform.Response{"tags": []any{"one"}})
	if r1.Valid {
		t.Error("expected invalid for fewer items than MinItems")
	}
	r2 := cueform.Validate(rf, cueform.Response{"tags": []any{"one", "two"}})
	if !r2.Valid {
		t.Errorf("expected valid for MinItems satisfied; issues: %v", r2.Issues)
	}
}

func TestValidate_VisibleFieldPathsPopulated(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-visible",
		Title: "Visible",
		Fields: []cueform.FieldDefinition{
			{Key: "a", Kind: cueform.FieldKindText, Label: "A"},
			{Key: "b", Kind: cueform.FieldKindText, Label: "B"},
		},
	})
	r := cueform.Validate(rf, cueform.Response{"a": "x", "b": "y"})
	found := map[string]bool{}
	for _, p := range r.VisibleFieldPaths {
		found[p] = true
	}
	if !found["a"] || !found["b"] {
		t.Errorf("expected both a and b in VisibleFieldPaths, got %v", r.VisibleFieldPaths)
	}
}

func TestValidate_PatternConstraint(t *testing.T) {
	rf := mustCompose(t, cueform.FormDefinition{
		ID:    "v-pattern",
		Title: "Pattern",
		Fields: []cueform.FieldDefinition{
			{
				Key:        "slug",
				Kind:       cueform.FieldKindText,
				Label:      "Slug",
				Validation: &cueform.FieldValidation{Pattern: "^[a-z0-9-]+$"},
			},
		},
	})
	r1 := cueform.Validate(rf, cueform.Response{"slug": "valid-slug-123"})
	if !r1.Valid {
		t.Errorf("expected valid slug, got issues: %v", r1.Issues)
	}
	r2 := cueform.Validate(rf, cueform.Response{"slug": "INVALID SLUG!"})
	if r2.Valid {
		t.Error("expected invalid slug with uppercase/spaces/punctuation")
	}
}
