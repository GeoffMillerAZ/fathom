package cueform

import (
	"fmt"
	"strings"
)

// Compose compiles a FormDefinition into a ResolvedForm with stable dotted
// field paths. Every field is validated and paths are checked for uniqueness
// across the entire form. The returned ResolvedForm is immutable — callers
// should not modify it after creation.
//
// Errors are returned for:
//   - Invalid form structure (missing ID, title, no fields).
//   - Invalid field definitions (missing key/label, unknown kind, etc.).
//   - Duplicate field paths anywhere in the form.
func Compose(form FormDefinition) (ResolvedForm, error) {
	if err := validateFormDefinition(form); err != nil {
		return ResolvedForm{}, err
	}

	resolved := ResolvedForm{
		ID:          form.ID,
		Title:       form.Title,
		Description: form.Description,
		Fields:      make([]ResolvedField, 0, len(form.Fields)),
	}

	seenPaths := map[string]struct{}{}
	for _, field := range form.Fields {
		resolvedField, err := resolveField(field, "", seenPaths)
		if err != nil {
			return ResolvedForm{}, err
		}
		resolved.Fields = append(resolved.Fields, resolvedField)
	}

	return resolved, nil
}

// resolveField recursively resolves a FieldDefinition into a ResolvedField.
// containerPath is the dotted prefix for the current nesting level (empty for
// top-level fields). seenPaths is threaded through all recursive calls to
// detect duplicate paths across the entire form.
func resolveField(field FieldDefinition, containerPath string, seenPaths map[string]struct{}) (ResolvedField, error) {
	if err := validateFieldDefinition(field); err != nil {
		return ResolvedField{}, err
	}

	path := joinPath(containerPath, field.Key)
	if _, exists := seenPaths[path]; exists {
		return ResolvedField{}, fmt.Errorf("duplicate field path %q", path)
	}
	seenPaths[path] = struct{}{}

	resolved := ResolvedField{
		Path:        path,
		Key:         field.Key,
		Kind:        field.Kind,
		Label:       field.Label,
		Description: field.Description,
		Required:    field.Required,
		Options:     append([]FieldOption(nil), field.Options...),
		Conditions:  resolveConditions(field.Conditions, containerPath),
		Default:     field.Default,
		Example:     field.Example,
		Prefill:     field.Prefill,
		Validation:  field.Validation,
	}

	if field.Kind == FieldKindGroup {
		for _, child := range field.Fields {
			resolvedChild, err := resolveField(child, path, seenPaths)
			if err != nil {
				return ResolvedField{}, err
			}
			resolved.Fields = append(resolved.Fields, resolvedChild)
		}
	}

	if field.Kind == FieldKindList && field.Item != nil {
		itemContainerPath := path + "[]"
		item, err := resolveListItem(*field.Item, itemContainerPath, seenPaths)
		if err != nil {
			return ResolvedField{}, err
		}
		resolved.Item = &item
	}

	return resolved, nil
}

// resolveListItem resolves a FieldDefinition used as a list item template.
// List item paths use the form "parent[].<key>" to distinguish them from
// regular field paths. List items are NOT added to seenPaths because the same
// item definition is reused for every element of the list.
func resolveListItem(field FieldDefinition, containerPath string, seenPaths map[string]struct{}) (ResolvedField, error) {
	if err := validateFieldDefinition(field); err != nil {
		return ResolvedField{}, err
	}

	path := joinPath(containerPath, field.Key)

	resolved := ResolvedField{
		Path:        path,
		Key:         field.Key,
		Kind:        field.Kind,
		Label:       field.Label,
		Description: field.Description,
		Required:    field.Required,
		Options:     append([]FieldOption(nil), field.Options...),
		Conditions:  resolveConditions(field.Conditions, containerPath),
		Default:     field.Default,
		Example:     field.Example,
		Prefill:     field.Prefill,
		Validation:  field.Validation,
	}

	if field.Kind == FieldKindGroup {
		for _, child := range field.Fields {
			resolvedChild, err := resolveListItem(child, path, seenPaths)
			if err != nil {
				return ResolvedField{}, err
			}
			resolved.Fields = append(resolved.Fields, resolvedChild)
		}
	}

	if field.Kind == FieldKindList && field.Item != nil {
		nestedContainerPath := path + "[]"
		item, err := resolveListItem(*field.Item, nestedContainerPath, seenPaths)
		if err != nil {
			return ResolvedField{}, err
		}
		resolved.Item = &item
	}

	return resolved, nil
}

// resolveConditions qualifies bare FieldRef values (no dot) to the
// containerPath, enabling partial fields to reference siblings without
// knowing the full path. Already-qualified refs (containing a dot) are
// left unchanged.
func resolveConditions(conditions []Condition, containerPath string) []Condition {
	if len(conditions) == 0 {
		return nil
	}
	resolved := make([]Condition, 0, len(conditions))
	for _, c := range conditions {
		updated := c
		if c.FieldRef != "" && !strings.Contains(c.FieldRef, ".") && containerPath != "" {
			updated.FieldRef = joinPath(containerPath, c.FieldRef)
		}
		resolved = append(resolved, updated)
	}
	return resolved
}

// validateFormDefinition checks that a FormDefinition is structurally valid
// before Compose begins resolving fields.
func validateFormDefinition(form FormDefinition) error {
	if form.ID == "" {
		return fmt.Errorf("form id is required")
	}
	if form.Title == "" {
		return fmt.Errorf("form title is required")
	}
	if len(form.Fields) == 0 {
		return fmt.Errorf("form %q must contain at least one field", form.ID)
	}
	return nil
}

// validateFieldDefinition checks that a FieldDefinition is structurally valid.
// This is called for every field (including nested group children and list
// items) before the field is resolved.
func validateFieldDefinition(field FieldDefinition) error {
	if field.Key == "" {
		return fmt.Errorf("field key is required")
	}
	if field.Label == "" {
		return fmt.Errorf("field %q label is required", field.Key)
	}
	switch field.Kind {
	case FieldKindText, FieldKindLongText, FieldKindBoolean, FieldKindInteger,
		FieldKindNumber, FieldKindSelect, FieldKindMultiSelect, FieldKindGroup, FieldKindList:
		// valid
	case "":
		return fmt.Errorf("field %q kind is required", field.Key)
	default:
		return fmt.Errorf("field %q has unsupported kind %q", field.Key, field.Kind)
	}
	if field.Kind == FieldKindGroup && len(field.Fields) == 0 {
		return fmt.Errorf("group field %q must define nested fields", field.Key)
	}
	if field.Kind != FieldKindGroup && len(field.Fields) > 0 {
		return fmt.Errorf("field %q may only set nested fields when kind=group", field.Key)
	}
	if field.Kind == FieldKindList && field.Item == nil {
		return fmt.Errorf("list field %q must define an item", field.Key)
	}
	if field.Kind != FieldKindList && field.Item != nil {
		return fmt.Errorf("field %q may only set item when kind=list", field.Key)
	}
	if (field.Kind == FieldKindSelect || field.Kind == FieldKindMultiSelect) && len(field.Options) == 0 {
		return fmt.Errorf("field %q requires at least one option", field.Key)
	}
	for _, c := range field.Conditions {
		if c.FieldRef == "" {
			return fmt.Errorf("field %q contains a condition with empty field_ref", field.Key)
		}
		switch c.Operator {
		case ConditionEquals, ConditionNotEquals, ConditionAnyOf, ConditionNoneOf,
			ConditionPresent, ConditionAbsent:
			// valid
		case "":
			return fmt.Errorf("field %q contains a condition with empty operator", field.Key)
		default:
			return fmt.Errorf("field %q contains unsupported condition operator %q", field.Key, c.Operator)
		}
	}
	return nil
}

// joinPath builds a dotted path from non-empty segments.
func joinPath(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, ".")
}
