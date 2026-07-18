package datacontract_test

import (
	"testing"

	datacontract "github.com/geoffmilleraz/agentic-kit/foundations/data/data-contract"
)

// --- helpers ----------------------------------------------------------------

func makeContract() datacontract.DataContract {
	return datacontract.DataContract{
		Name: "test-contract",
		Fields: []datacontract.FieldDef{
			{Name: "id", Type: datacontract.FieldTypeString, Required: true},
			{Name: "count", Type: datacontract.FieldTypeInt, Required: true},
			{Name: "ratio", Type: datacontract.FieldTypeFloat, Required: false},
			{Name: "active", Type: datacontract.FieldTypeBool, Required: false},
			{Name: "blob", Type: datacontract.FieldTypeBytes, Required: false},
		},
	}
}

// --- Validate: happy path ---------------------------------------------------

func TestValidate_AllFieldsPresent(t *testing.T) {
	t.Parallel()
	dc := makeContract()
	record := map[string]any{
		"id":     "abc",
		"count":  42,
		"ratio":  3.14,
		"active": true,
		"blob":   []byte{0x01},
	}
	if err := dc.Validate(record); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_OptionalFieldsMissing(t *testing.T) {
	t.Parallel()
	dc := makeContract()
	record := map[string]any{
		"id":    "abc",
		"count": 1,
	}
	if err := dc.Validate(record); err != nil {
		t.Fatalf("unexpected error when optional fields absent: %v", err)
	}
}

func TestValidate_IntFromFloat64(t *testing.T) {
	// encoding/json unmarshals numbers as float64; whole float64 should be
	// accepted as int.
	t.Parallel()
	dc := datacontract.DataContract{
		Name: "json-compat",
		Fields: []datacontract.FieldDef{
			{Name: "count", Type: datacontract.FieldTypeInt, Required: true},
		},
	}
	record := map[string]any{"count": float64(7)}
	if err := dc.Validate(record); err != nil {
		t.Fatalf("whole float64 should satisfy int field: %v", err)
	}
}

func TestValidate_IntFromFloat64Fractional(t *testing.T) {
	// A fractional float64 must NOT satisfy an int field.
	t.Parallel()
	dc := datacontract.DataContract{
		Name: "json-compat",
		Fields: []datacontract.FieldDef{
			{Name: "count", Type: datacontract.FieldTypeInt, Required: true},
		},
	}
	record := map[string]any{"count": float64(7.5)}
	if err := dc.Validate(record); err == nil {
		t.Fatal("fractional float64 should fail int type check")
	}
}

// --- Validate: missing required field ---------------------------------------

func TestValidate_MissingRequired(t *testing.T) {
	t.Parallel()
	dc := makeContract()
	record := map[string]any{
		// "id" is absent — required field
		"count": 1,
	}
	err := dc.Validate(record)
	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}
}

// --- Validate: wrong type ---------------------------------------------------

func TestValidate_WrongTypeString(t *testing.T) {
	t.Parallel()
	dc := makeContract()
	record := map[string]any{
		"id":    123, // should be string
		"count": 1,
	}
	if err := dc.Validate(record); err == nil {
		t.Fatal("expected type error for int value in string field")
	}
}

func TestValidate_WrongTypeFloat(t *testing.T) {
	t.Parallel()
	dc := makeContract()
	record := map[string]any{
		"id":    "ok",
		"count": 1,
		"ratio": "not-a-float",
	}
	if err := dc.Validate(record); err == nil {
		t.Fatal("expected type error for string value in float field")
	}
}

func TestValidate_WrongTypeBool(t *testing.T) {
	t.Parallel()
	dc := makeContract()
	record := map[string]any{
		"id":     "ok",
		"count":  1,
		"active": "yes", // should be bool
	}
	if err := dc.Validate(record); err == nil {
		t.Fatal("expected type error for string value in bool field")
	}
}

func TestValidate_WrongTypeBytes(t *testing.T) {
	t.Parallel()
	dc := makeContract()
	record := map[string]any{
		"id":    "ok",
		"count": 1,
		"blob":  "not-bytes", // should be []byte
	}
	if err := dc.Validate(record); err == nil {
		t.Fatal("expected type error for string value in bytes field")
	}
}

// --- DataContract name / field name validation logic -----------------------

func TestValidate_EmptyRecord(t *testing.T) {
	t.Parallel()
	dc := datacontract.DataContract{
		Name: "no-required",
		Fields: []datacontract.FieldDef{
			{Name: "opt", Type: datacontract.FieldTypeString, Required: false},
		},
	}
	if err := dc.Validate(map[string]any{}); err != nil {
		t.Fatalf("empty record with no required fields should pass: %v", err)
	}
}

func TestValidate_ExtraFieldsIgnored(t *testing.T) {
	t.Parallel()
	dc := makeContract()
	record := map[string]any{
		"id":        "ok",
		"count":     5,
		"extra_key": "value", // not in contract — should be silently ignored
		"another":   99,
	}
	if err := dc.Validate(record); err != nil {
		t.Fatalf("extra fields should be ignored: %v", err)
	}
}

// --- All integer types accepted for FieldTypeInt ---------------------------

func TestValidate_AllIntTypes(t *testing.T) {
	t.Parallel()
	dc := datacontract.DataContract{
		Name: "int-types",
		Fields: []datacontract.FieldDef{
			{Name: "v", Type: datacontract.FieldTypeInt, Required: true},
		},
	}
	variants := []any{
		int(1), int8(1), int16(1), int32(1), int64(1),
		uint(1), uint8(1), uint16(1), uint32(1), uint64(1),
	}
	for _, v := range variants {
		if err := dc.Validate(map[string]any{"v": v}); err != nil {
			t.Errorf("expected %T to satisfy int field: %v", v, err)
		}
	}
}
