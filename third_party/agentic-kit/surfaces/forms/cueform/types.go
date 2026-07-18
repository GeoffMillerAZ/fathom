// Package cueform is the CUE-driven form runtime for agentic-kit.
//
// It provides two concerns:
//
//  1. Compose — compiles a FormDefinition into a ResolvedForm with stable
//     dotted field paths, validated field definitions, and resolved options.
//
//  2. Response — generates draft/sample responses and validates submitted
//     responses against a ResolvedForm, accumulating all errors into a
//     ValidationReport.
//
// cueform lives under surfaces/ because its role is human-facing form
// definition and validation — but surfaces/ packages are fully importable
// library code, exactly like foundations/. The tier in the import path
// expresses a package's ROLE in the layering, not its importability: import
// (and vendor) this package directly rather than forking or re-declaring the
// form types in your own module.
//
// No third-party dependencies are used; only stdlib.
package cueform

import "time"

// FieldKind is a typed string enumerating the supported field kinds.
type FieldKind string

const (
	// FieldKindText is a single-line string field.
	FieldKindText FieldKind = "text"
	// FieldKindLongText is a multi-line string field.
	FieldKindLongText FieldKind = "longtext"
	// FieldKindNumber is a floating-point numeric field.
	FieldKindNumber FieldKind = "number"
	// FieldKindInteger is a strict integer field.
	FieldKindInteger FieldKind = "integer"
	// FieldKindBoolean is a boolean (true/false) field.
	FieldKindBoolean FieldKind = "boolean"
	// FieldKindSelect is a single-choice enumeration field.
	FieldKindSelect FieldKind = "select"
	// FieldKindMultiSelect is a multi-choice enumeration field.
	FieldKindMultiSelect FieldKind = "multiselect"
	// FieldKindGroup is a container field whose value is its children.
	FieldKindGroup FieldKind = "group"
	// FieldKindList is a repeating field whose items are typed by Item.
	FieldKindList FieldKind = "list"
)

// FieldOption is a single selectable choice for select/multiselect fields.
type FieldOption struct {
	// Value is the machine-readable option key. Must be unique within the field.
	Value string
	// Label is the human-readable display string.
	Label string
}

// FieldValidation holds optional validation constraints for a field.
type FieldValidation struct {
	// MinLength constrains the minimum byte length of a string value.
	MinLength *int
	// MaxLength constrains the maximum byte length of a string value.
	MaxLength *int
	// Pattern is a Go regexp that a string value must fully match.
	Pattern string
	// MinValue constrains the minimum numeric value.
	MinValue *float64
	// MaxValue constrains the maximum numeric value.
	MaxValue *float64
	// MinItems constrains the minimum item count for list/multiselect fields.
	MinItems *int
	// MaxItems constrains the maximum item count for list/multiselect fields.
	MaxItems *int
}

// Condition makes a field conditionally visible.
type Condition struct {
	// FieldRef is the dotted path of the field to evaluate.
	// Bare keys (no dot) are qualified to the container path at compose time.
	FieldRef string
	// Operator is the comparison operator.
	Operator ConditionOperator
	// Value is the comparison value (unused for present/absent operators).
	Value any
}

// ConditionOperator enumerates the supported condition comparison operators.
type ConditionOperator string

const (
	// ConditionEquals requires the referenced field's value to equal Value.
	ConditionEquals ConditionOperator = "equals"
	// ConditionNotEquals requires the referenced field's value to differ
	// from Value.
	ConditionNotEquals ConditionOperator = "not_equals"
	// ConditionAnyOf requires the referenced field's value to match one of
	// the choices in Value.
	ConditionAnyOf ConditionOperator = "any_of"
	// ConditionNoneOf requires the referenced field's value to match none of
	// the choices in Value.
	ConditionNoneOf ConditionOperator = "none_of"
	// ConditionPresent requires the referenced field to have a value.
	ConditionPresent ConditionOperator = "present"
	// ConditionAbsent requires the referenced field to have no value.
	ConditionAbsent ConditionOperator = "absent"
)

// FieldDefinition is the input description of a single form field.
type FieldDefinition struct {
	// Key is the field identifier used in dotted paths. Must be non-empty.
	Key string
	// Kind is the field type. Must be a valid FieldKind.
	Kind FieldKind
	// Label is the human-readable display name. Must be non-empty.
	Label string
	// Description is optional explanatory text shown to the respondent.
	Description string
	// Required indicates the field must have a non-zero value in the response.
	Required bool
	// Options is the inline list of selectable choices (select/multiselect only).
	Options []FieldOption
	// Conditions makes the field conditionally visible.
	Conditions []Condition
	// Item describes the type of each list element (list fields only).
	Item *FieldDefinition
	// Fields holds nested field definitions (group fields only).
	Fields []FieldDefinition
	// Default is the value used when no other value is provided.
	Default any
	// Example is the value used in sample responses.
	Example any
	// Prefill is the value pre-populated in draft responses (highest priority after Example in sample mode).
	Prefill any
	// Validation holds optional field-level constraints.
	Validation *FieldValidation
}

// FormDefinition is the top-level input description of a form.
type FormDefinition struct {
	// ID uniquely identifies the form. Must be non-empty.
	ID string
	// Title is the human-readable form name. Must be non-empty.
	Title string
	// Description is optional explanatory text for the form.
	Description string
	// Fields is the flat list of top-level field definitions.
	// Must be non-empty.
	Fields []FieldDefinition
}

// ResolvedField is a compiled field with a stable dotted path and all
// references (options, conditions) resolved.
type ResolvedField struct {
	// Path is the stable dotted path for this field (e.g. "section.key").
	Path string
	// Key is the bare field key (last segment of Path).
	Key string
	// Kind is the resolved field kind.
	Kind FieldKind
	// Label is the human-readable display name.
	Label string
	// Description is the optional explanatory text.
	Description string
	// Required indicates the field must have a non-zero value.
	Required bool
	// Options is the resolved list of selectable choices.
	Options []FieldOption
	// Conditions are the resolved visibility conditions.
	Conditions []Condition
	// Item is the resolved list item definition (list fields only).
	Item *ResolvedField
	// Fields holds resolved nested fields (group fields only).
	Fields []ResolvedField
	// Default is the default value.
	Default any
	// Example is the example value used in sample responses.
	Example any
	// Prefill is the pre-populated value.
	Prefill any
	// Validation holds optional constraints.
	Validation *FieldValidation
}

// ResolvedForm is the compiled output of Compose. Every field has a stable
// dotted path and all references are resolved.
type ResolvedForm struct {
	// ID is the form identifier.
	ID string
	// Title is the human-readable form title.
	Title string
	// Description is the optional form description.
	Description string
	// Fields is the flat list of resolved top-level fields.
	Fields []ResolvedField
}

// Response is a map of dotted field path to submitted value.
type Response map[string]any

// ResponseStatus classifies the lifecycle state of a response.
type ResponseStatus string

const (
	// ResponseStatusDraft means the response has not yet been submitted as
	// complete.
	ResponseStatusDraft ResponseStatus = "draft"
	// ResponseStatusComplete means the response has been submitted as
	// complete.
	ResponseStatusComplete ResponseStatus = "complete"
)

// ValidationIssueCode classifies the kind of validation error.
type ValidationIssueCode string

const (
	// ValidationIssueRequired is used when a required field has no value.
	ValidationIssueRequired ValidationIssueCode = "required"
	// ValidationIssueType is used when a value has the wrong Go type.
	ValidationIssueType ValidationIssueCode = "type"
	// ValidationIssueConstraint is used when a value violates a constraint
	// (range, pattern, item count, option membership).
	ValidationIssueConstraint ValidationIssueCode = "constraint"
)

// ValidationIssue is a single validation problem for a specific field.
type ValidationIssue struct {
	// FieldPath is the dotted path of the offending field.
	FieldPath string
	// Code categorises the problem.
	Code ValidationIssueCode
	// Message is a human-readable description of the problem.
	Message string
}

// ValidationReport is the accumulated result of validating a Response.
type ValidationReport struct {
	// Valid is true when MissingRequired and Issues are both empty.
	Valid bool
	// MissingRequired lists the paths of required fields with no value.
	MissingRequired []string
	// Issues lists all non-required validation problems.
	Issues []ValidationIssue
	// VisibleFieldPaths lists the paths of fields that were evaluated
	// (conditions satisfied or absent).
	VisibleFieldPaths []string
}

// addIssue appends a ValidationIssue to the report.
func (r *ValidationReport) addIssue(fieldPath string, code ValidationIssueCode, message string) {
	r.Issues = append(r.Issues, ValidationIssue{
		FieldPath: fieldPath,
		Code:      code,
		Message:   message,
	})
}

// FormRecord is a stamped response record with metadata.
type FormRecord struct {
	// ResponseID is a unique identifier for this response record.
	ResponseID string
	// FormID is the ID of the form this response belongs to.
	FormID string
	// Status is the lifecycle state of the response.
	Status ResponseStatus
	// Values holds the submitted field values, keyed by dotted path.
	Values Response
	// CreatedAt is when the record was first created.
	CreatedAt time.Time
	// UpdatedAt is when the record was last modified.
	UpdatedAt time.Time
}
