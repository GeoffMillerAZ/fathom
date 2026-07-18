// Package cueform — valid test data that must unify with schema.cue.
// Run: cue vet ./... (from the cueform directory or repo root)
package cueform

import "github.com/geoffmilleraz/agentic-kit/surfaces/forms/cueform"

// validSimpleForm demonstrates a minimal form with a single text field.
validSimpleForm: cueform.#FormDefinition & {
	ID:    "simple-form"
	Title: "Simple Form"
	Fields: [
		{
			Key:      "name"
			Kind:     "text"
			Label:    "Full Name"
			Required: true
		},
	]
}

// validFormWithAllKinds demonstrates a form with every supported field kind.
validFormWithAllKinds: cueform.#FormDefinition & {
	ID:          "all-kinds"
	Title:       "All Field Kinds"
	Description: "Tests every supported field kind"
	Fields: [
		{Key: "txt", Kind: "text", Label: "Text"},
		{Key: "ltxt", Kind: "longtext", Label: "Long Text"},
		{Key: "num", Kind: "number", Label: "Number"},
		{Key: "int", Kind: "integer", Label: "Integer"},
		{Key: "bool", Kind: "boolean", Label: "Boolean"},
		{
			Key:     "sel"
			Kind:    "select"
			Label:   "Select"
			Options: [{Value: "a", Label: "A"}, {Value: "b", Label: "B"}]
		},
		{
			Key:     "msel"
			Kind:    "multiselect"
			Label:   "Multi Select"
			Options: [{Value: "x", Label: "X"}, {Value: "y", Label: "Y"}]
		},
		{
			Key:   "grp"
			Kind:  "group"
			Label: "Group"
			Fields: [
				{Key: "child", Kind: "text", Label: "Child"},
			]
		},
		{
			Key:   "lst"
			Kind:  "list"
			Label: "List"
			Item: {Key: "item", Kind: "text", Label: "Item"}
		},
	]
}

// validFormWithConditions demonstrates conditions on a field.
validFormWithConditions: cueform.#FormDefinition & {
	ID:    "conditional-form"
	Title: "Conditional Form"
	Fields: [
		{
			Key:     "has_address"
			Kind:    "boolean"
			Label:   "Has Address?"
			Required: true
		},
		{
			Key:   "address"
			Kind:  "text"
			Label: "Address"
			Conditions: [
				{FieldRef: "has_address", Operator: "equals", Value: true},
			]
		},
	]
}

// validFormWithValidation demonstrates all validation constraint types.
validFormWithValidation: cueform.#FormDefinition & {
	ID:    "validation-form"
	Title: "Validation Form"
	Fields: [
		{
			Key:   "short_text"
			Kind:  "text"
			Label: "Short Text"
			Validation: {MinLength: 2, MaxLength: 50}
		},
		{
			Key:   "tagged"
			Kind:  "text"
			Label: "Tagged"
			Validation: {Pattern: "^[a-z]+$"}
		},
		{
			Key:   "score"
			Kind:  "number"
			Label: "Score"
			Validation: {MinValue: 0.0, MaxValue: 100.0}
		},
		{
			Key:   "count"
			Kind:  "integer"
			Label: "Count"
			Validation: {MinValue: 1, MaxValue: 10}
		},
		{
			Key:   "tags"
			Kind:  "list"
			Label: "Tags"
			Item: {Key: "tag", Kind: "text", Label: "Tag"}
			Validation: {MinItems: 1, MaxItems: 5}
		},
	]
}

// validFormWithGroupListItem demonstrates a list whose item is a group.
validFormWithGroupListItem: cueform.#FormDefinition & {
	ID:    "group-list-form"
	Title: "Group List Form"
	Fields: [
		{
			Key:   "contacts"
			Kind:  "list"
			Label: "Contacts"
			Item: {
				Key:   "contact"
				Kind:  "group"
				Label: "Contact"
				Fields: [
					{Key: "first_name", Kind: "text", Label: "First Name", Required: true},
					{Key: "last_name", Kind: "text", Label: "Last Name", Required: true},
					{Key: "email", Kind: "text", Label: "Email"},
				]
			}
		},
	]
}

// validFieldKinds verifies every #FieldKind enum value is accepted.
_fkText:        cueform.#FieldKind & "text"
_fkLongText:    cueform.#FieldKind & "longtext"
_fkNumber:      cueform.#FieldKind & "number"
_fkInteger:     cueform.#FieldKind & "integer"
_fkBoolean:     cueform.#FieldKind & "boolean"
_fkSelect:      cueform.#FieldKind & "select"
_fkMultiSelect: cueform.#FieldKind & "multiselect"
_fkGroup:       cueform.#FieldKind & "group"
_fkList:        cueform.#FieldKind & "list"

// validConditionOperators verifies every #ConditionOperator enum value is accepted.
_coEquals:    cueform.#ConditionOperator & "equals"
_coNotEquals: cueform.#ConditionOperator & "not_equals"
_coAnyOf:     cueform.#ConditionOperator & "any_of"
_coNoneOf:    cueform.#ConditionOperator & "none_of"
_coPresent:   cueform.#ConditionOperator & "present"
_coAbsent:    cueform.#ConditionOperator & "absent"
