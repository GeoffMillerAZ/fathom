// Package datacontract — invalid examples.
// ALL blocks below are commented out so that `cue vet ./...` passes.
// Uncomment individual blocks to observe the validation error described.
package datacontract

// import "github.com/geoffmilleraz/agentic-kit/foundations/data/data-contract:datacontract"

// -- Example 1: unknown field type ------------------------------------------
// BadFieldType would fail because "uuid" is not a member of #FieldType.
//
// badFieldType: datacontract.#DataContract & {
// 	Name: "bad-field-type"
// 	Fields: [
// 		{Name: "id", Type: "uuid", Required: true}, // ERROR: not a valid #FieldType
// 	]
// }

// -- Example 2: empty contract name -----------------------------------------
// EmptyName would fail because Name must be a non-empty string.
//
// emptyName: datacontract.#DataContract & {
// 	Name: ""  // ERROR: violates & !=""
// 	Fields: [
// 		{Name: "x", Type: "string", Required: true},
// 	]
// }

// -- Example 3: empty field name --------------------------------------------
// EmptyFieldName would fail because FieldDef.Name must be a non-empty string.
//
// emptyFieldName: datacontract.#DataContract & {
// 	Name: "ok"
// 	Fields: [
// 		{Name: "", Type: "string", Required: true}, // ERROR: violates & !=""
// 	]
// }
