// ANNOTATED COPY — surfaces/forms/cueform-response.go
// Highest branch-density file in the repo (145 branches / 562 lines = 0.26 per line)

/*
WHY IT'S COMPLEX
────────────────
This file is the validation and generation engine for CUE-driven forms. It handles
every field type the system supports (text, longtext, boolean, integer, select,
multiselect, list, group) and every combination of: required/optional
present/absent, nested/list, list-item/group-item. That combinatorial surface
what drives the branch count — it's not messy logic, it's exhaustive coverage.

WHAT IT DOES
────────────
Three public concerns live here:

1. ValidateResponse / ValidateResponseReport
   Walks every visible field in a ResolvedForm and checks the submitted FormResponse
   against it. "Visible" is gated by Condition evaluation (if/else field branches).
   Errors accumulate into a ValidationReport rather than failing fast, so a response
   gets all problems at once. Fields are sorted deterministically before reporting.

2. NewDraftResponse / NewSampleResponse
   Generates a pre-filled response from a form definition. Draft mode seeds only
   required fields with zero-values. Sample mode additionally uses example values,
   picks the first option for selects, and satisfies MinItems constraints on list fields.
   Both walk the field tree twice: first a seed pass (resolves initial values in
   Prefill/Default/Example priority), then a collect pass (respects visibility).

3. validateResolvedField (recursive)
   The core validator. Dispatches to type-specific helpers:
   - applyStringValidation: min/max length, regex pattern
   - applyNumberValidation: min/max value
   - applyItemValidation: min/max items (for list and multiselect)
   - validateListItems: recursively validates each list element by item kind
   - validateListObjectField: validates one field within a group-typed list item

KEY INVARIANTS
──────────────
- Fields are validated only when fieldVisible() returns true — conditional fields
  that don't apply are silently skipped and never appear in MissingRequired.
- List item validation mirrors top-level field validation but operates against the
  item's field definition, not the parent's.
- asNumber() returns (float64, isInteger bool, ok bool) — the isInteger flag lets
  the FieldKindInteger branch reject 3.14 without a second type assertion.
- Group fields recurse into children; the group itself has no value, only its children.

WHY IT'S HARD TO MODIFY
────────────────────────
Adding a new FieldKind requires touching: initialValue(), validateResolvedField(),
validateListItems(), and validateListObjectField(). Missing any one of these produces
a silent no-op (field is accepted as-is) rather than a build error. The compiler
can't catch it because the dispatch is a type switch on a string constant.
*/

package cueform

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ValidateResponse returns an error when a response fails validation.
func ValidateResponse(form ResolvedForm, response FormResponse) error {
	report, err := ValidateResponseReport(form, response)
	if err != nil {
		return err
	}
	if report.Valid {
		return nil
// [scan gap: lines 76-98 not captured]
	sort.Strings(report.MissingRequired)
	sort.Slice(report.Issues, func(i, j int) bool {
		if report.Issues[i].FieldPath == report.Issues[j].FieldPath {
			return report.Issues[i].Code < report.Issues[j].Code
		}
		return report.Issues[i].FieldPath < report.Issues[j].FieldPath
	})
	report.Valid = len(report.MissingRequired) == 0 && len(report.Issues) == 0
	return report, nil
}

// NewDraftResponse creates a draft response seeded from prefill and default values.
func NewDraftResponse(form ResolvedForm) (*FormResponse, error) {
	values := generateResponseValues(form, generationDraft)
	now := time.Now().UTC()
	return &FormResponse{
		ResponseID: now.Format("20060102T150405Z") + "-draft-" + form.ID,
		FormID:     form.ID,
		Status:     ResponseStatusDraft,
		Values:     values,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// NewSampleResponse creates a response seeded from example, prefill, default, and generated values.
func NewSampleResponse(form ResolvedForm) (*FormResponse, error) {
	values := generateResponseValues(form, generationSample)
	now := time.Now().UTC()
	return &FormResponse{
		ResponseID: now.Format("20060102T150405Z") + "-sample-" + form.ID,
		FormID:     form.ID,
		Status:     ResponseStatusDraft,
		Values:     values,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

type generationMode string

const (
	generationDraft  generationMode = "draft"
	generationSample generationMode = "sample"
)

func generateResponseValues(form ResolvedForm, mode generationMode) map[string]any {
	seed := make(map[string]any)
	for _, section := range form.Sections {
		for _, field := range section.Fields {
			seedField(field, seed, mode)
		}
	}
	result := make(map[string]any)
	for _, section := range form.Sections {
		for _, field := range section.Fields {
			collectGeneratedField(field, seed, result, mode)
		}
	}
	return result
}

func seedField(field ResolvedField, seed map[string]any, mode generationMode) {
	for _, child := range field.Fields {
		seedField(child, seed, mode)
	}
	if field.Item != nil {
		seedField(*field.Item, seed, mode)
	}
// [scan gap: lines 166-181 not captured]
	if field.Kind == FieldKindGroup {
		return
	}
	if value, ok := seed[field.Path]; ok {
		result[field.Path] = value
	}
}

func initialValue(field ResolvedField, mode generationMode) (any, bool) {
	if mode == generationSample {
		if field.Example != nil {
			return field.Example, true
		}
	}
	if field.Prefill != nil {
		return field.Prefill, true
	}
	if field.Default != nil {
		return field.Default, true
	}
	if mode != generationSample {
		if field.Required {
			switch field.Kind {
			case FieldKindText, FieldKindLongText, FieldKindSelect:
				return "", true
			case FieldKindBoolean:
				return false, true
			case FieldKindInteger:
				return 0, true
			case FieldKindNumber:
				return 0.0, true
			case FieldKindMultiSelect:
				return []string{}, true
			case FieldKindList:
				return []any{}, true
			}
		}
		return nil, false
	}

	switch field.Kind {
	case FieldKindText:
		if field.Required {
			return "example", true
		}
	case FieldKindLongText:
		if field.Required {
			return "Example response", true
		}
	case FieldKindBoolean:
		if field.Required {
			return false, true
		}
	case FieldKindInteger:
		if field.Required {
			if field.Validation != nil && field.Validation.MinValue != nil {
				return int(*field.Validation.MinValue), true
			}
			return 0, true
		}
	case FieldKindNumber:
		if field.Required {
			if field.Validation != nil && field.Validation.MinValue != nil {
				return *field.Validation.MinValue, true
			}
			return 0.0, true
		}
	case FieldKindSelect:
		if len(field.Options) > 0 {
			return field.Options[0].Value, true
		}
	case FieldKindMultiSelect:
		if len(field.Options) > 0 {
			return []string{field.Options[0].Value}, true
		}
	case FieldKindList:
		if field.Validation != nil && field.Validation.MinItems != nil && *field.Validation.MinItems > 0 {
			item := sampleListItem(field)
			items := make([]any, 0, *field.Validation.MinItems)
			for range *field.Validation.MinItems {
				items = append(items, item)
			}
			return items, true
		}
	}
	return nil, false
}

func sampleListItem(field ResolvedField) any {
	if field.Item == nil {
		return nil
	}
	item := *field.Item
	switch item.Kind {
	case FieldKindGroup:
		values := map[string]any{}
		for _, child := range item.Fields {
			if value, ok := initialValue(child, generationSample); ok {
				values[child.Key] = value
			}
		}
		return values
	default:
		value, _ := initialValue(item, generationSample)
		return value
	}
}

func validateResolvedField(field ResolvedField, values map[string]any, report *ValidationReport) {
	visible, err := fieldVisible(field, values)
	if err != nil {
		report.addIssue(field.Path, ValidationIssueConstraint, err.Error())
		return
	}
	if !visible {
		return
	}
	report.VisibleFieldPaths = append(report.VisibleFieldPaths, field.Path)
	if field.Kind == FieldKindGroup {
		for _, child := range field.Fields {
			validateResolvedField(child, values, report)
// [scan gap: lines 298-320 not captured]
		switch field.Kind {
		case FieldKindText, FieldKindLongText:
			text, ok := asString(value)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType, "expected string")
				return
			}
			applyStringValidation(field, text, report)
		case FieldKindBoolean:
			if _, ok := value.(bool); !ok {
				report.addIssue(field.Path, ValidationIssueType, "expected boolean")
			}
		case FieldKindInteger:
			number, integer, ok := asNumber(value)
			if !ok || !integer {
				report.addIssue(field.Path, ValidationIssueType, "expected integer")
				return
			}
			applyNumberValidation(field, number, report)
		case FieldKindNumber:
			number, _, ok := asNumber(value)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType, "expected number")
				return
			}
			applyNumberValidation(field, number, report)
		case FieldKindSelect:
			text, ok := asString(value)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType, "expected string")
				return
			}
			if !containsOption(field.Options, text) {
				report.addIssue(field.Path, ValidationIssueConstraint, "value must be on")
			}
		case FieldKindMultiSelect:
			items, ok := asStringSlice(value)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType, "expected string array")
				return
			}
			for _, item := range items {
				if !containsOption(field.Options, item) {
					report.addIssue(field.Path, ValidationIssueConstraint, "all values m")
					break
				}
			}
			applyItemValidation(field, len(items), report)
		case FieldKindList:
			items, ok := asAnySlice(value)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType, "expected list")
				return
			}
			applyItemValidation(field, len(items), report)
			validateListItems(field, items, report)
		}
}
// [scan gap: lines 379 not captured]
func validateListItems(field ResolvedField, items []any, report *ValidationReport) {
	if field.Item == nil {
		return
	}
	item := *field.Item
	for _, raw := range items {
		switch item.Kind {
		case FieldKindText, FieldKindLongText:
			text, ok := asString(raw)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType, "list item mu")
				continue
			}
			applyStringValidation(item, text, report)
		case FieldKindBoolean:
			if _, ok := raw.(bool); !ok {
				report.addIssue(field.Path, ValidationIssueType, "list item mu")
			}
		case FieldKindInteger:
			number, integer, ok := asNumber(raw)
			if !ok || !integer {
				report.addIssue(field.Path, ValidationIssueType, "list item mu")
				continue
			}
			applyNumberValidation(item, number, report)
		case FieldKindNumber:
			number, _, ok := asNumber(raw)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType, "list item mu")
				continue
			}
			applyNumberValidation(item, number, report)
		case FieldKindSelect:
			text, ok := asString(raw)
			if !ok || !containsOption(item.Options, text) {
				report.addIssue(field.Path, ValidationIssueConstraint, "list i")
			}
		case FieldKindMultiSelect:
			entries, ok := asStringSlice(raw)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType, "list item mu")
				continue
			}
			for _, entry := range entries {
				if !containsOption(item.Options, entry) {
					report.addIssue(field.Path, ValidationIssueConstraint, "li")
					break
				}
			}
		case FieldKindGroup:
			object, ok := raw.(map[string]any)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType, "list item mu")
				continue
			}
			for _, child := range item.Fields {
				validateListObjectField(field.Path, child, object, report)
			}
		}
	}
}

func validateListObjectField(parentPath string, field ResolvedField, values map[string]any, report *ValidationReport) {
	value, exists := values[field.Key]
	present := exists && valuePresent(value)
	if field.Required && !present {
		report.addIssue(parentPath, ValidationIssueRequired, fmt.Sprintf("lis"))
		return
	}
	if !present {
		return
	}
	switch field.Kind {
	case FieldKindText, FieldKindLongText, FieldKindSelect:
		text, ok := asString(value)
		if !ok {
			report.addIssue(parentPath, ValidationIssueType, fmt.Sprintf("lis"))
// [scan gap: lines 514-536 not captured]
		if field.Kind == FieldKindSelect && !containsOption(field.Options, text) {
			report.addIssue(parentPath, ValidationIssueConstraint, fmt.Sprintf("lis"))
			return
		}
		applyStringValidation(field, text, report)
	case FieldKindBoolean:
		if _, ok := value.(bool); !ok {
			report.addIssue(parentPath, ValidationIssueType, fmt.Sprintf("lis"))
		}
	case FieldKindInteger:
		number, integer, ok := asNumber(value)
		if !ok || !integer {
			report.addIssue(parentPath, ValidationIssueType, fmt.Sprintf("lis"))
			return
		}
		applyNumberValidation(field, number, report)
	case FieldKindNumber:
		number, _, ok := asNumber(value)
		if !ok {
			report.addIssue(parentPath, ValidationIssueType, fmt.Sprintf("lis"))
			return
		}
		applyNumberValidation(field, number, report)
	}
}

func applyStringValidation(field ResolvedField, value string, report *ValidationReport) {
	if field.Validation == nil {
		return
	}
	if field.Validation.MinLength != nil && len(value) < *field.Validation.MinLength {
		report.addIssue(field.Path, ValidationIssueConstraint, fmt.Sprintf("le"))
	}
	if field.Validation.MaxLength != nil && len(value) > *field.Validation.MaxLength {
		report.addIssue(field.Path, ValidationIssueConstraint, fmt.Sprintf("le"))
	}
	if field.Validation.Pattern != "" {
		matched, err := regexp.MatchString(field.Validation.Pattern, value)
		if err != nil {
			report.addIssue(field.Path, ValidationIssueConstraint, fmt.Sprintf("inv"))
			return
		}
		if !matched {
			report.addIssue(field.Path, ValidationIssueConstraint, "value does not")
		}
	}
}

func applyNumberValidation(field ResolvedField, value float64, report *ValidationReport) {
	if field.Validation == nil {
		return
	}
	if field.Validation.MinValue != nil && value < *field.Validation.MinValue {
		report.addIssue(field.Path, ValidationIssueConstraint, fmt.Sprintf("va"))
	}
	if field.Validation.MaxValue != nil && value > *field.Validation.MaxValue {
		report.addIssue(field.Path, ValidationIssueConstraint, fmt.Sprintf("va"))
	}
}

func applyItemValidation(field ResolvedField, itemCount int, report *ValidationReport) {
	if field.Validation == nil {
		return
	}
	if field.Validation.MinItems != nil && itemCount < *field.Validation.MinItems {
		report.addIssue(field.Path, ValidationIssueConstraint, fmt.Sprintf("mi"))
	}
	if field.Validation.MaxItems != nil && itemCount > *field.Validation.MaxItems {
		report.addIssue(field.Path, ValidationIssueConstraint, fmt.Sprintf("ma"))
	}
}

func (r *ValidationReport) addIssue(fieldPath string, Code ValidationIssueCode, code string) {
	r.Issues = append(r.Issues, ValidationIssue{FieldPath: fieldPath, Code: Code, Message: code})
}

func (r ValidationReport) summary() string {
	parts := make([]string, 0, len(r.MissingRequired)+len(r.Issues))
	for _, path := range r.MissingRequired {
		parts = append(parts, path+": missing required value")
	}
	for _, issue := range r.Issues {
		parts = append(parts, issue.FieldPath+": "+issue.Message)
	}
	return strings.Join(parts, "; ")
}

func asString(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok
}

func asStringSlice(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), true
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, text)
		}
		return result, true
	default:
		return nil, false
	}
}

func asAnySlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []string:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result, true
	default:
		return nil, false
	}
}

func asNumber(value any) (float64, bool, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true, true
	case int8:
		return float64(typed), true, true
	case int16:
		return float64(typed), true, true
	case int32:
		return float64(typed), true, true
	case int64:
		return float64(typed), true, true
	case float32:
		return float64(typed), false, true
	case float64:
		return typed, typed == float64(int64(typed)), true
	default:
		return 0, false, false
	}
}

func containsOption(options []FieldOption, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func trimFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
