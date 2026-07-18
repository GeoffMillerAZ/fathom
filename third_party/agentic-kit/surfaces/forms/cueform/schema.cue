// Package cueform defines the CUE schema for the cueform runtime.
// These definitions are the authoritative source of truth for the value
// spaces that the Go types in types.go mirror. CUE is authoritative for
// the valid value space; Go is authoritative for runtime behaviour.
//
// Evaluate with: cue vet ./...
package cueform

// #FieldKind enumerates the valid field kinds.
#FieldKind: "text" | "longtext" | "number" | "integer" | "boolean" | "select" | "multiselect" | "group" | "list"

// #ConditionOperator enumerates the valid condition comparison operators.
#ConditionOperator: "equals" | "not_equals" | "any_of" | "none_of" | "present" | "absent"

// #FieldOption is a single selectable choice for select/multiselect fields.
#FieldOption: {
	// Value is the machine-readable option key.
	Value: string & !=""
	// Label is the human-readable display string.
	Label: string & !=""
}

// #FieldValidation holds optional validation constraints.
#FieldValidation: {
	MinLength?: int & >=0
	MaxLength?: int & >=0
	Pattern?:   string
	MinValue?:  number
	MaxValue?:  number
	MinItems?:  int & >=0
	MaxItems?:  int & >=0
}

// #Condition makes a field conditionally visible.
#Condition: {
	// FieldRef is the dotted path of the field to evaluate.
	FieldRef: string & !=""
	// Operator is the comparison operator.
	Operator: #ConditionOperator
	// Value is the comparison value (may be omitted for present/absent).
	Value?: _
}

// #FieldDefinition is the input description of a single form field.
#FieldDefinition: {
	// Key is the field identifier. Must be non-empty.
	Key: string & !=""
	// Kind is the field type.
	Kind: #FieldKind
	// Label is the human-readable display name. Must be non-empty.
	Label: string & !=""
	// Description is optional explanatory text.
	Description?: string
	// Required indicates the field must have a value in the response.
	Required?: bool
	// Options is the inline list of choices (select/multiselect only).
	Options?: [...#FieldOption]
	// Conditions makes the field conditionally visible.
	Conditions?: [...#Condition]
	// Item describes each list element (list fields only).
	Item?: #FieldDefinition
	// Fields holds nested definitions (group fields only).
	Fields?: [...#FieldDefinition]
	// Default is the value used when no other value is provided.
	Default?: _
	// Example is the value used in sample responses.
	Example?: _
	// Prefill is the value pre-populated in draft responses.
	Prefill?: _
	// Validation holds optional field-level constraints.
	Validation?: #FieldValidation
}

// #FormDefinition is the top-level input description of a form.
#FormDefinition: {
	// ID uniquely identifies the form. Must be non-empty.
	ID: string & !=""
	// Title is the human-readable form name. Must be non-empty.
	Title: string & !=""
	// Description is optional explanatory text.
	Description?: string
	// Fields is the list of top-level field definitions. At least one required.
	Fields: [#FieldDefinition, ...#FieldDefinition]
}
