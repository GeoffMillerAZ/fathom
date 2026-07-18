// Package cueform — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// All examples below are commented out to prevent vet failures.
//
// To verify the schema rejects a given example, temporarily uncomment
// the block and run:
//   cue vet ./...   (expect errors)
//
// --- Rejected examples and expected errors ---
//
//   invalidForm_MissingID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidForm_MissingTitle:
//     Title: "" violates string & !="" — Title must not be empty
//
//   invalidForm_EmptyFields:
//     Fields: [] violates [#FieldDefinition, ...] — at least one field required
//
//   invalidField_MissingKey:
//     Key: "" violates string & !="" — Key must not be empty
//
//   invalidField_MissingLabel:
//     Label: "" violates string & !="" — Label must not be empty
//
//   invalidField_BadKind:
//     Kind: "unknown" violates #FieldKind enum
//
//   invalidOption_EmptyValue:
//     Value: "" violates string & !="" — option Value must not be empty
//
//   invalidOption_EmptyLabel:
//     Label: "" violates string & !="" — option Label must not be empty
//
//   invalidCondition_EmptyFieldRef:
//     FieldRef: "" violates string & !="" — FieldRef must not be empty
//
//   invalidCondition_BadOperator:
//     Operator: "like" violates #ConditionOperator enum

package cueform

// // invalidForm_MissingID — ID must not be empty
// invalidForm_MissingID: #FormDefinition & {
// 	ID:    ""
// 	Title: "Test"
// 	Fields: [{Key: "f", Kind: "text", Label: "F"}]
// }

// // invalidForm_MissingTitle — Title must not be empty
// invalidForm_MissingTitle: #FormDefinition & {
// 	ID:    "test"
// 	Title: ""
// 	Fields: [{Key: "f", Kind: "text", Label: "F"}]
// }

// // invalidForm_EmptyFields — at least one field is required
// invalidForm_EmptyFields: #FormDefinition & {
// 	ID:     "test"
// 	Title:  "Test"
// 	Fields: []
// }

// // invalidField_MissingKey — Key must not be empty
// invalidField_MissingKey: #FieldDefinition & {
// 	Key:   ""
// 	Kind:  "text"
// 	Label: "Field"
// }

// // invalidField_MissingLabel — Label must not be empty
// invalidField_MissingLabel: #FieldDefinition & {
// 	Key:   "field"
// 	Kind:  "text"
// 	Label: ""
// }

// // invalidField_BadKind — "unknown" is not a valid #FieldKind
// invalidField_BadKind: #FieldDefinition & {
// 	Key:   "field"
// 	Kind:  "unknown"
// 	Label: "Field"
// }

// // invalidOption_EmptyValue — option Value must not be empty
// invalidOption_EmptyValue: #FieldOption & {
// 	Value: ""
// 	Label: "Option A"
// }

// // invalidOption_EmptyLabel — option Label must not be empty
// invalidOption_EmptyLabel: #FieldOption & {
// 	Value: "a"
// 	Label: ""
// }

// // invalidCondition_EmptyFieldRef — FieldRef must not be empty
// invalidCondition_EmptyFieldRef: #Condition & {
// 	FieldRef: ""
// 	Operator: "equals"
// 	Value:    "yes"
// }

// // invalidCondition_BadOperator — "like" is not a valid #ConditionOperator
// invalidCondition_BadOperator: #Condition & {
// 	FieldRef: "some.field"
// 	Operator: "like"
// 	Value:    "yes"
// }
