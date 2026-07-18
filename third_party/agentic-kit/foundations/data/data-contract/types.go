// Package datacontract provides low-level data-layer contracts validated by CUE.
// The Go types in this file mirror the CUE definitions in schema.cue.
// CUE is authoritative for data shape; Go is authoritative for behaviour.
//
// Concurrency contract: DataContract and FieldDef are plain value types with
// no mutable state; values are safe to read concurrently. Validate is a pure
// function and may be called from multiple goroutines simultaneously.
package datacontract

import "fmt"

// FieldType is the set of recognised primitive field types.
// Values must match the #FieldType enum in schema.cue.
type FieldType string

const (
	// FieldTypeString identifies a string-valued field.
	FieldTypeString FieldType = "string"
	// FieldTypeInt identifies an integer-valued field.
	FieldTypeInt FieldType = "int"
	// FieldTypeFloat identifies a floating-point-valued field.
	FieldTypeFloat FieldType = "float"
	// FieldTypeBool identifies a boolean-valued field.
	FieldTypeBool FieldType = "bool"
	// FieldTypeBytes identifies a raw-bytes-valued field.
	FieldTypeBytes FieldType = "bytes"
)

// FieldDef describes a single field within a DataContract.
// It mirrors #FieldDef in schema.cue.
type FieldDef struct {
	// Name is the non-empty field identifier.
	Name string

	// Type is the primitive type expected for this field.
	Type FieldType

	// Required indicates whether the field must be present in every record.
	Required bool
}

// DataContract is a named description of a data record shape.
// It mirrors #DataContract in schema.cue.
type DataContract struct {
	// Name is a non-empty identifier for this contract (e.g. "user-event").
	Name string

	// Fields is the ordered list of field definitions.
	Fields []FieldDef
}

// Validate checks that record satisfies the contract:
//   - All Required fields are present.
//   - Every present field matches the declared FieldType.
//
// Type checking follows Go's standard JSON-like type rules:
//
//   - "string"  → Go string
//   - "int"     → Go int, int8, int16, int32, int64, uint and variants, or
//     float64/float32 with a zero fractional part (common in JSON unmarshalling)
//   - "float"   → Go float32 or float64
//   - "bool"    → Go bool
//   - "bytes"   → Go []byte
func (dc DataContract) Validate(record map[string]any) error {
	for _, f := range dc.Fields {
		val, present := record[f.Name]
		if !present {
			if f.Required {
				return fmt.Errorf("datacontract %q: required field %q is missing", dc.Name, f.Name)
			}
			continue
		}
		if err := checkType(dc.Name, f.Name, f.Type, val); err != nil {
			return err
		}
	}
	return nil
}

// checkType verifies that val matches the expected FieldType.
func checkType(contractName, fieldName string, ft FieldType, val any) error {
	ok := false
	switch ft {
	case FieldTypeString:
		_, ok = val.(string)
	case FieldTypeInt:
		ok = isIntLike(val)
	case FieldTypeFloat:
		switch val.(type) {
		case float32, float64:
			ok = true
		}
	case FieldTypeBool:
		_, ok = val.(bool)
	case FieldTypeBytes:
		_, ok = val.([]byte)
	default:
		return fmt.Errorf("datacontract %q: field %q has unknown type %q", contractName, fieldName, ft)
	}
	if !ok {
		return fmt.Errorf("datacontract %q: field %q expected type %q, got %T", contractName, fieldName, ft, val)
	}
	return nil
}

// isIntLike returns true when v is any Go integer type, or a float64/float32
// whose fractional part is zero (as produced by encoding/json unmarshalling).
func isIntLike(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		uintptr:
		return true
	case float64:
		f := v.(float64)
		return f == float64(int64(f))
	case float32:
		f := v.(float32)
		return f == float32(int32(f))
	}
	return false
}
