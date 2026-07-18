// Package datacontract defines CUE contracts for data-layer data contracts.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package datacontract

// #FieldType enumerates the recognised primitive field types.
#FieldType: "string" | "int" | "float" | "bool" | "bytes"

// #FieldDef describes a single field within a data contract.
#FieldDef: {
	// Name is the non-empty field identifier.
	Name: string & !=""

	// Type is the primitive type expected for this field.
	Type: #FieldType

	// Required indicates whether the field must be present in every record.
	Required: bool
}

// #DataContract is a named, versioned description of a data record shape.
#DataContract: {
	// Name is a non-empty identifier for the contract (e.g. "user-event").
	Name: string & !=""

	// Fields is the ordered list of field definitions. At least one field
	// is required so that an empty contract cannot be accidentally created.
	Fields: [#FieldDef, ...#FieldDef]
}
