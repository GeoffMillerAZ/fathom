package cueform

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// NewDraftResponse creates a FormRecord seeded with zero values for all
// required fields (using Prefill > Default priority). Optional fields with
// no Prefill or Default are omitted.
func NewDraftResponse(form ResolvedForm) FormRecord {
	values := generateResponseValues(form, generationModeDraft)
	now := time.Now().UTC()
	return FormRecord{
		ResponseID: now.Format("20060102T150405Z") + "-draft-" + form.ID,
		FormID:     form.ID,
		Status:     ResponseStatusDraft,
		Values:     values,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// NewSampleResponse creates a FormRecord seeded from Example > Prefill >
// Default priority, picking first options for select fields and satisfying
// MinItems constraints for list fields.
func NewSampleResponse(form ResolvedForm) FormRecord {
	values := generateResponseValues(form, generationModeSample)
	now := time.Now().UTC()
	return FormRecord{
		ResponseID: now.Format("20060102T150405Z") + "-sample-" + form.ID,
		FormID:     form.ID,
		Status:     ResponseStatusDraft,
		Values:     values,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// Validate walks every visible field in form and checks response against it.
// Visibility is gated by Condition evaluation; invisible fields are silently
// skipped. All errors accumulate into the returned ValidationReport.
func Validate(form ResolvedForm, response Response) ValidationReport {
	report := ValidationReport{}
	for _, field := range form.Fields {
		validateResolvedField(field, response, &report)
	}
	sort.Strings(report.MissingRequired)
	sort.Strings(report.VisibleFieldPaths)
	sort.Slice(report.Issues, func(i, j int) bool {
		pi, pj := report.Issues[i].FieldPath, report.Issues[j].FieldPath
		if pi != pj {
			return pi < pj
		}
		return report.Issues[i].Code < report.Issues[j].Code
	})
	report.Valid = len(report.MissingRequired) == 0 && len(report.Issues) == 0
	return report
}

// generationMode controls how generateResponseValues seeds initial values.
type generationMode string

const (
	generationModeDraft  generationMode = "draft"
	generationModeSample generationMode = "sample"
)

// generateResponseValues produces a Response map by seeding initial values
// for all fields in the form and then collecting the ones that are visible.
func generateResponseValues(form ResolvedForm, mode generationMode) Response {
	seed := make(map[string]any)
	for _, field := range form.Fields {
		seedField(field, seed, mode)
	}
	result := make(Response)
	for _, field := range form.Fields {
		collectGeneratedField(field, seed, result, mode)
	}
	return result
}

// seedField recursively computes initial values for a field and its children,
// storing them in seed keyed by dotted path.
func seedField(field ResolvedField, seed map[string]any, mode generationMode) {
	// Recurse into group children and list item first so nested paths are seeded.
	for _, child := range field.Fields {
		seedField(child, seed, mode)
	}
	if field.Item != nil {
		seedField(*field.Item, seed, mode)
	}
	if field.Kind == FieldKindGroup {
		// Groups have no value of their own; only their children have values.
		return
	}
	if value, ok := initialValue(field, mode); ok {
		seed[field.Path] = value
	}
}

// collectGeneratedField copies seeded values into result, respecting visibility
// conditions evaluated against the already-seeded values.
func collectGeneratedField(field ResolvedField, seed map[string]any, result Response, mode generationMode) {
	// Evaluate visibility using seeded values so that conditional fields are
	// correctly included or excluded in the generated response.
	visible, err := fieldVisible(field, seed)
	if err != nil || !visible {
		return
	}
	if field.Kind == FieldKindGroup {
		for _, child := range field.Fields {
			collectGeneratedField(child, seed, result, mode)
		}
		return
	}
	if value, ok := seed[field.Path]; ok {
		result[field.Path] = value
	}
}

// initialValue returns the value that should be seeded for field in the given
// generation mode. Returns (value, true) when a value should be seeded,
// (nil, false) when the field should be omitted.
//
// Priority:
//   - sample mode: Example > Prefill > Default > generated zero/first-option
//   - draft mode:  Prefill > Default > zero (required only)
func initialValue(field ResolvedField, mode generationMode) (any, bool) {
	if mode == generationModeSample {
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

	if mode == generationModeDraft {
		// Draft mode: seed zero values for required fields only.
		if !field.Required {
			return nil, false
		}
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
		return nil, false
	}

	// Sample mode: generate values even for optional fields.
	switch field.Kind {
	case FieldKindText:
		return "example", true
	case FieldKindLongText:
		return "Example response.", true
	case FieldKindBoolean:
		return false, true
	case FieldKindInteger:
		if field.Validation != nil && field.Validation.MinValue != nil {
			return int(*field.Validation.MinValue), true
		}
		return 0, true
	case FieldKindNumber:
		if field.Validation != nil && field.Validation.MinValue != nil {
			return *field.Validation.MinValue, true
		}
		return 0.0, true
	case FieldKindSelect:
		if len(field.Options) > 0 {
			return field.Options[0].Value, true
		}
	case FieldKindMultiSelect:
		if len(field.Options) > 0 {
			return []string{field.Options[0].Value}, true
		}
	case FieldKindList:
		minItems := 0
		if field.Validation != nil && field.Validation.MinItems != nil {
			minItems = *field.Validation.MinItems
		}
		if minItems <= 0 {
			minItems = 1
		}
		item := sampleListItem(field)
		items := make([]any, 0, minItems)
		for range minItems {
			items = append(items, item)
		}
		return items, true
	}
	return nil, false
}

// sampleListItem returns a single representative item for a list field in
// sample mode.
func sampleListItem(field ResolvedField) any {
	if field.Item == nil {
		return nil
	}
	item := *field.Item
	if item.Kind == FieldKindGroup {
		values := map[string]any{}
		for _, child := range item.Fields {
			if value, ok := initialValue(child, generationModeSample); ok {
				values[child.Key] = value
			}
		}
		return values
	}
	value, _ := initialValue(item, generationModeSample)
	return value
}

// validateResolvedField validates a single resolved field against the response.
// Invisible fields are silently skipped (their paths never appear in the report).
func validateResolvedField(field ResolvedField, values Response, report *ValidationReport) {
	visible, err := fieldVisible(field, values)
	if err != nil {
		report.addIssue(field.Path, ValidationIssueConstraint, "condition error: "+err.Error())
		return
	}
	if !visible {
		return
	}
	report.VisibleFieldPaths = append(report.VisibleFieldPaths, field.Path)

	// Group fields have no value of their own; recurse into children.
	if field.Kind == FieldKindGroup {
		for _, child := range field.Fields {
			validateResolvedField(child, values, report)
		}
		return
	}

	value, present := values[field.Path]
	if !present || !valuePresent(value) {
		if field.Required {
			report.MissingRequired = append(report.MissingRequired, field.Path)
		}
		return
	}

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
		num, isInt, ok := asNumber(value)
		if !ok || !isInt {
			report.addIssue(field.Path, ValidationIssueType, "expected integer")
			return
		}
		applyNumberValidation(field, num, report)

	case FieldKindNumber:
		num, _, ok := asNumber(value)
		if !ok {
			report.addIssue(field.Path, ValidationIssueType, "expected number")
			return
		}
		applyNumberValidation(field, num, report)

	case FieldKindSelect:
		text, ok := asString(value)
		if !ok {
			report.addIssue(field.Path, ValidationIssueType, "expected string")
			return
		}
		if !containsOption(field.Options, text) {
			report.addIssue(field.Path, ValidationIssueConstraint,
				fmt.Sprintf("value %q is not a valid option", text))
		}

	case FieldKindMultiSelect:
		items, ok := asStringSlice(value)
		if !ok {
			report.addIssue(field.Path, ValidationIssueType, "expected string array")
			return
		}
		for _, item := range items {
			if !containsOption(field.Options, item) {
				report.addIssue(field.Path, ValidationIssueConstraint,
					fmt.Sprintf("value %q is not a valid option", item))
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

// validateListItems validates each element of a list field against the item
// definition. The item kind determines what type and constraints are expected.
func validateListItems(field ResolvedField, items []any, report *ValidationReport) {
	if field.Item == nil {
		return
	}
	item := *field.Item
	for i, raw := range items {
		switch item.Kind {
		case FieldKindText, FieldKindLongText:
			text, ok := asString(raw)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType,
					fmt.Sprintf("list item[%d] must be a string", i))
				continue
			}
			applyStringValidation(item, text, report)

		case FieldKindBoolean:
			if _, ok := raw.(bool); !ok {
				report.addIssue(field.Path, ValidationIssueType,
					fmt.Sprintf("list item[%d] must be a boolean", i))
			}

		case FieldKindInteger:
			num, isInt, ok := asNumber(raw)
			if !ok || !isInt {
				report.addIssue(field.Path, ValidationIssueType,
					fmt.Sprintf("list item[%d] must be an integer", i))
				continue
			}
			applyNumberValidation(item, num, report)

		case FieldKindNumber:
			num, _, ok := asNumber(raw)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType,
					fmt.Sprintf("list item[%d] must be a number", i))
				continue
			}
			applyNumberValidation(item, num, report)

		case FieldKindSelect:
			text, ok := asString(raw)
			if !ok || !containsOption(item.Options, text) {
				report.addIssue(field.Path, ValidationIssueConstraint,
					fmt.Sprintf("list item[%d] must be a valid option", i))
			}

		case FieldKindMultiSelect:
			entries, ok := asStringSlice(raw)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType,
					fmt.Sprintf("list item[%d] must be a string array", i))
				continue
			}
			for _, entry := range entries {
				if !containsOption(item.Options, entry) {
					report.addIssue(field.Path, ValidationIssueConstraint,
						fmt.Sprintf("list item[%d] value %q is not a valid option", i, entry))
					break
				}
			}

		case FieldKindGroup:
			object, ok := raw.(map[string]any)
			if !ok {
				report.addIssue(field.Path, ValidationIssueType,
					fmt.Sprintf("list item[%d] must be an object", i))
				continue
			}
			for _, child := range item.Fields {
				validateListObjectField(field.Path, child, object, report)
			}
		}
	}
}

// validateListObjectField validates a single named field within a group-typed
// list item. parentPath is the path of the parent list field (used for issue
// reporting since list item children don't have independent top-level paths).
func validateListObjectField(parentPath string, field ResolvedField, values map[string]any, report *ValidationReport) {
	value, exists := values[field.Key]
	present := exists && valuePresent(value)
	if field.Required && !present {
		report.addIssue(parentPath, ValidationIssueRequired,
			fmt.Sprintf("list item field %q is required", field.Key))
		return
	}
	if !present {
		return
	}
	switch field.Kind {
	case FieldKindText, FieldKindLongText, FieldKindSelect:
		text, ok := asString(value)
		if !ok {
			report.addIssue(parentPath, ValidationIssueType,
				fmt.Sprintf("list item field %q must be a string", field.Key))
			return
		}
		if field.Kind == FieldKindSelect && !containsOption(field.Options, text) {
			report.addIssue(parentPath, ValidationIssueConstraint,
				fmt.Sprintf("list item field %q value %q is not a valid option", field.Key, text))
			return
		}
		applyStringValidation(field, text, report)

	case FieldKindBoolean:
		if _, ok := value.(bool); !ok {
			report.addIssue(parentPath, ValidationIssueType,
				fmt.Sprintf("list item field %q must be a boolean", field.Key))
		}

	case FieldKindInteger:
		num, isInt, ok := asNumber(value)
		if !ok || !isInt {
			report.addIssue(parentPath, ValidationIssueType,
				fmt.Sprintf("list item field %q must be an integer", field.Key))
			return
		}
		applyNumberValidation(field, num, report)

	case FieldKindNumber:
		num, _, ok := asNumber(value)
		if !ok {
			report.addIssue(parentPath, ValidationIssueType,
				fmt.Sprintf("list item field %q must be a number", field.Key))
			return
		}
		applyNumberValidation(field, num, report)

	case FieldKindMultiSelect:
		entries, ok := asStringSlice(value)
		if !ok {
			report.addIssue(parentPath, ValidationIssueType,
				fmt.Sprintf("list item field %q must be a string array", field.Key))
			return
		}
		for _, entry := range entries {
			if !containsOption(field.Options, entry) {
				report.addIssue(parentPath, ValidationIssueConstraint,
					fmt.Sprintf("list item field %q value %q is not a valid option", field.Key, entry))
				break
			}
		}
	}
}

// fieldVisible evaluates whether a field is visible given the current response
// values. A field is visible when it has no conditions, or when all conditions
// are satisfied. Returns an error only when a condition operator is unrecognised.
func fieldVisible(field ResolvedField, values map[string]any) (bool, error) {
	for _, c := range field.Conditions {
		val, exists := values[c.FieldRef]
		present := exists && valuePresent(val)
		switch c.Operator {
		case ConditionPresent:
			if !present {
				return false, nil
			}
		case ConditionAbsent:
			if present {
				return false, nil
			}
		case ConditionEquals:
			if !present || !valuesEqual(val, c.Value) {
				return false, nil
			}
		case ConditionNotEquals:
			if present && valuesEqual(val, c.Value) {
				return false, nil
			}
		case ConditionAnyOf:
			anyMatch := false
			if present {
				choices, _ := toStringSlice(c.Value)
				vs, ok := asString(val)
				if ok {
					for _, choice := range choices {
						if vs == choice {
							anyMatch = true
							break
						}
					}
				}
			}
			if !anyMatch {
				return false, nil
			}
		case ConditionNoneOf:
			if present {
				choices, _ := toStringSlice(c.Value)
				vs, ok := asString(val)
				if ok {
					for _, choice := range choices {
						if vs == choice {
							return false, nil
						}
					}
				}
			}
		default:
			return false, fmt.Errorf("unsupported condition operator %q", c.Operator)
		}
	}
	return true, nil
}

// valuesEqual compares two arbitrary values for equality using string
// representation for cross-type comparisons (e.g. int vs float64).
func valuesEqual(a, b any) bool {
	if a == b {
		return true
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// toStringSlice coerces a value to []string for any_of / none_of condition
// value lists.
func toStringSlice(v any) ([]string, bool) {
	switch typed := v.(type) {
	case []string:
		return typed, true
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, s)
		}
		return result, true
	case string:
		return []string{typed}, true
	default:
		return nil, false
	}
}

// valuePresent returns true when v is not the zero value for its type.
func valuePresent(v any) bool {
	if v == nil {
		return false
	}
	switch typed := v.(type) {
	case string:
		return typed != ""
	case bool:
		return true
	case int:
		return true
	case int8, int16, int32, int64:
		return true
	case float32, float64:
		return true
	case []string:
		return len(typed) >= 0 // empty slice is still "present"
	case []any:
		return len(typed) >= 0
	default:
		return true
	}
}

// applyStringValidation applies MinLength, MaxLength, and Pattern constraints
// to a string value.
func applyStringValidation(field ResolvedField, value string, report *ValidationReport) {
	if field.Validation == nil {
		return
	}
	if field.Validation.MinLength != nil && len(value) < *field.Validation.MinLength {
		report.addIssue(field.Path, ValidationIssueConstraint,
			fmt.Sprintf("length %d is less than minimum %d", len(value), *field.Validation.MinLength))
	}
	if field.Validation.MaxLength != nil && len(value) > *field.Validation.MaxLength {
		report.addIssue(field.Path, ValidationIssueConstraint,
			fmt.Sprintf("length %d exceeds maximum %d", len(value), *field.Validation.MaxLength))
	}
	if field.Validation.Pattern != "" {
		matched, err := regexp.MatchString(field.Validation.Pattern, value)
		if err != nil {
			report.addIssue(field.Path, ValidationIssueConstraint,
				fmt.Sprintf("invalid pattern %q: %v", field.Validation.Pattern, err))
			return
		}
		if !matched {
			report.addIssue(field.Path, ValidationIssueConstraint,
				fmt.Sprintf("value does not match pattern %q", field.Validation.Pattern))
		}
	}
}

// applyNumberValidation applies MinValue and MaxValue constraints to a numeric value.
func applyNumberValidation(field ResolvedField, value float64, report *ValidationReport) {
	if field.Validation == nil {
		return
	}
	if field.Validation.MinValue != nil && value < *field.Validation.MinValue {
		report.addIssue(field.Path, ValidationIssueConstraint,
			fmt.Sprintf("value %s is less than minimum %s",
				trimFloat(value), trimFloat(*field.Validation.MinValue)))
	}
	if field.Validation.MaxValue != nil && value > *field.Validation.MaxValue {
		report.addIssue(field.Path, ValidationIssueConstraint,
			fmt.Sprintf("value %s exceeds maximum %s",
				trimFloat(value), trimFloat(*field.Validation.MaxValue)))
	}
}

// applyItemValidation applies MinItems and MaxItems constraints to a count.
func applyItemValidation(field ResolvedField, itemCount int, report *ValidationReport) {
	if field.Validation == nil {
		return
	}
	if field.Validation.MinItems != nil && itemCount < *field.Validation.MinItems {
		report.addIssue(field.Path, ValidationIssueConstraint,
			fmt.Sprintf("item count %d is less than minimum %d", itemCount, *field.Validation.MinItems))
	}
	if field.Validation.MaxItems != nil && itemCount > *field.Validation.MaxItems {
		report.addIssue(field.Path, ValidationIssueConstraint,
			fmt.Sprintf("item count %d exceeds maximum %d", itemCount, *field.Validation.MaxItems))
	}
}

// containsOption returns true when any option in the slice has the given value.
func containsOption(options []FieldOption, value string) bool {
	for _, opt := range options {
		if opt.Value == value {
			return true
		}
	}
	return false
}

// trimFloat formats a float64 without unnecessary trailing zeros.
func trimFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// asString coerces value to string.
func asString(value any) (string, bool) {
	s, ok := value.(string)
	return s, ok
}

// asNumber coerces value to float64, returning (float64, isInteger, ok).
// The isInteger flag is true when the original value was an integer type or
// when the float64 value is exactly representable as an integer.
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

// asStringSlice coerces value to []string, accepting both []string and []any
// (where each element must be a string).
func asStringSlice(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), true
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, s)
		}
		return result, true
	default:
		return nil, false
	}
}

// asAnySlice coerces value to []any, accepting both []any and []string.
func asAnySlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []string:
		result := make([]any, 0, len(typed))
		for _, s := range typed {
			result = append(result, s)
		}
		return result, true
	default:
		return nil, false
	}
}
