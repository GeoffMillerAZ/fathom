// Package editor is the editor-facing surface layer for agentic-kit.
//
// It converts a compiled [cueform.ResolvedForm] into a flat, ordered slice of
// [FieldModel] values that an IDE or editor extension can render without
// needing to understand CUE or the form composition rules.
//
// The output is deterministic: identical [cueform.ResolvedForm] inputs always
// produce the same [FieldModel] slice in the same order.
package editor

import "github.com/geoffmilleraz/agentic-kit/surfaces/forms/cueform"

// FieldKind mirrors the field kinds from cueform, represented as a plain
// string so that editor consumers do not need to import the cueform package.
type FieldKind string

const (
	// FieldKindText identifies a single-line text field.
	FieldKindText FieldKind = "text"
	// FieldKindLongText identifies a multi-line text field.
	FieldKindLongText FieldKind = "longtext"
	// FieldKindNumber identifies a numeric (floating-point) field.
	FieldKindNumber FieldKind = "number"
	// FieldKindInteger identifies an integer field.
	FieldKindInteger FieldKind = "integer"
	// FieldKindBoolean identifies a boolean field.
	FieldKindBoolean FieldKind = "boolean"
	// FieldKindSelect identifies a single-choice selection field.
	FieldKindSelect FieldKind = "select"
	// FieldKindMultiSelect identifies a multiple-choice selection field.
	FieldKindMultiSelect FieldKind = "multiselect"
	// FieldKindGroup identifies a nested group of fields.
	FieldKindGroup FieldKind = "group"
	// FieldKindList identifies a repeatable list field.
	FieldKindList FieldKind = "list"
)

// FieldOption is a single selectable choice for select/multiselect models.
// It is a plain copy of the cueform option so callers need not import cueform.
type FieldOption struct {
	// Value is the machine-readable option key.
	Value string
	// Label is the human-readable display string.
	Label string
}

// FieldModel is the editor-facing representation of a single resolved form
// field. It carries every property an editor needs to render and validate a
// field without requiring access to the original FormDefinition.
type FieldModel struct {
	// Path is the stable dotted path (e.g. "section.fieldname").
	Path string

	// Key is the bare field identifier (last path segment).
	Key string

	// Kind is the rendered field type.
	Kind FieldKind

	// Label is the human-readable display name.
	Label string

	// Description is the optional hint text shown below the label.
	Description string

	// Required indicates the field must carry a non-zero value.
	Required bool

	// Options is the ordered list of selectable values (select/multiselect).
	Options []FieldOption

	// Depth is the nesting depth of this field: 0 for top-level fields,
	// 1 for direct group children, 2 for nested group children, etc.
	// List item templates have the same depth as their parent list field + 1.
	Depth int

	// IsListItem is true when this FieldModel represents a list item template
	// rather than a regular resolved field.
	IsListItem bool

	// DefaultValue is the field's configured default, or nil if absent.
	DefaultValue any

	// Placeholder is the example value that editors may display as placeholder
	// text. Derived from the field's Example value when present, falling back
	// to Prefill, then nil.
	Placeholder any

	// HasConditions is true when the field has at least one visibility
	// condition. Editors can use this to add visual affordances.
	HasConditions bool

	// HasValidation is true when the field carries a non-nil Validation block.
	HasValidation bool
}

// resolvedFieldKind maps a cueform.FieldKind to the local FieldKind.
// Any unrecognised kind falls back to FieldKindText.
func resolvedFieldKind(k cueform.FieldKind) FieldKind {
	switch k {
	case cueform.FieldKindText:
		return FieldKindText
	case cueform.FieldKindLongText:
		return FieldKindLongText
	case cueform.FieldKindNumber:
		return FieldKindNumber
	case cueform.FieldKindInteger:
		return FieldKindInteger
	case cueform.FieldKindBoolean:
		return FieldKindBoolean
	case cueform.FieldKindSelect:
		return FieldKindSelect
	case cueform.FieldKindMultiSelect:
		return FieldKindMultiSelect
	case cueform.FieldKindGroup:
		return FieldKindGroup
	case cueform.FieldKindList:
		return FieldKindList
	default:
		return FieldKindText
	}
}
