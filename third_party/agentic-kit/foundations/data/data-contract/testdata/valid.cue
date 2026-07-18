// Package datacontract — valid test data that must unify with schema.cue.
package datacontract

import "github.com/geoffmilleraz/agentic-kit/foundations/data/data-contract:datacontract"

// userEvent is a well-formed contract with required and optional fields.
userEvent: datacontract.#DataContract & {
	Name: "user-event"
	Fields: [
		{Name: "user_id", Type: "string", Required: true},
		{Name: "action", Type: "string", Required: true},
		{Name: "score", Type: "float", Required: false},
	]
}

// metricsRecord is a contract using int, float, and bool types.
metricsRecord: datacontract.#DataContract & {
	Name: "metrics-record"
	Fields: [
		{Name: "timestamp", Type: "int", Required: true},
		{Name: "value", Type: "float", Required: true},
		{Name: "is_anomaly", Type: "bool", Required: false},
	]
}

// blobRecord is a contract with a bytes field.
blobRecord: datacontract.#DataContract & {
	Name: "blob-record"
	Fields: [
		{Name: "payload", Type: "bytes", Required: true},
	]
}

// allFieldTypes exercises every #FieldType value in one contract.
allFieldTypes: datacontract.#DataContract & {
	Name: "all-types"
	Fields: [
		{Name: "a", Type: "string", Required: true},
		{Name: "b", Type: "int", Required: true},
		{Name: "c", Type: "float", Required: true},
		{Name: "d", Type: "bool", Required: true},
		{Name: "e", Type: "bytes", Required: false},
	]
}

// Verify all #FieldType enum values are accepted.
_ft1: datacontract.#FieldType & "string"
_ft2: datacontract.#FieldType & "int"
_ft3: datacontract.#FieldType & "float"
_ft4: datacontract.#FieldType & "bool"
_ft5: datacontract.#FieldType & "bytes"
