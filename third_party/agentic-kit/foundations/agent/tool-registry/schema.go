package toolregistry

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Prop describes one property of a flat object schema built by ObjectSchema.
type Prop struct {
	// Name is the property name. Must be non-empty and unique per schema.
	Name string

	// Type is the JSON Schema type: "string", "number", "integer",
	// "boolean", or "array" (arrays of scalars only; set Items).
	Type string

	// Description tells the model what the property means. Omitted from the
	// schema when empty.
	Description string

	// Required marks the property as required in the emitted schema.
	Required bool

	// Enum restricts a "string" property to the given values. Only valid
	// when Type is "string". Omitted from the schema when empty.
	Enum []string

	// Items is the scalar element type ("string", "number", "integer",
	// "boolean") when Type is "array"; it must be empty otherwise.
	Items string
}

// scalarTypes are the JSON Schema types ObjectSchema accepts for scalar
// properties and array elements.
var scalarTypes = map[string]bool{
	"string":  true,
	"number":  true,
	"integer": true,
	"boolean": true,
}

// ObjectSchema builds a flat JSON Schema object — type "object" with the
// given properties — for ToolDef.InputSchema. It is DELIBERATELY MINIMAL:
// scalar properties, string enums, and arrays of scalars. Anything richer
// (nested objects, oneOf, formats, numeric ranges) is out of scope: supply
// raw JSON to ToolDef.InputSchema directly — json.RawMessage IS the escape
// hatch.
//
// Properties and the required list are emitted in declaration order, so the
// output is deterministic and golden-testable.
//
// ObjectSchema is a definition-time constructor in the spirit of
// regexp.MustCompile: it PANICS on malformed props (empty or duplicate Name,
// unsupported Type, Enum on a non-string, Items on a non-array, or an array
// without a scalar Items), because a bad tool schema is a programming bug
// that should fail at startup, not at dispatch.
func ObjectSchema(props ...Prop) json.RawMessage {
	seen := make(map[string]bool, len(props))
	var required []string

	var buf bytes.Buffer
	buf.WriteString(`{"type":"object","properties":{`)
	for i, p := range props {
		validateProp(p, seen)
		seen[p.Name] = true

		if i > 0 {
			buf.WriteByte(',')
		}
		writeJSONString(&buf, p.Name)
		buf.WriteString(`:{"type":`)
		writeJSONString(&buf, p.Type)
		if p.Type == "array" {
			buf.WriteString(`,"items":{"type":`)
			writeJSONString(&buf, p.Items)
			buf.WriteString(`}`)
		}
		if p.Description != "" {
			buf.WriteString(`,"description":`)
			writeJSONString(&buf, p.Description)
		}
		if len(p.Enum) > 0 {
			buf.WriteString(`,"enum":`)
			writeJSONValue(&buf, p.Enum)
		}
		buf.WriteByte('}')

		if p.Required {
			required = append(required, p.Name)
		}
	}
	buf.WriteByte('}')
	if len(required) > 0 {
		buf.WriteString(`,"required":`)
		writeJSONValue(&buf, required)
	}
	buf.WriteByte('}')

	return json.RawMessage(buf.Bytes())
}

// validateProp panics with a descriptive message when p is malformed. seen
// holds the names already emitted in this schema.
func validateProp(p Prop, seen map[string]bool) {
	if p.Name == "" {
		panic("toolregistry: ObjectSchema: prop has empty Name")
	}
	if seen[p.Name] {
		panic(fmt.Sprintf("toolregistry: ObjectSchema: duplicate prop %q", p.Name))
	}
	switch {
	case scalarTypes[p.Type]:
		if p.Items != "" {
			panic(fmt.Sprintf("toolregistry: ObjectSchema: prop %q has Items but Type %q is not \"array\"", p.Name, p.Type))
		}
	case p.Type == "array":
		if !scalarTypes[p.Items] {
			panic(fmt.Sprintf("toolregistry: ObjectSchema: array prop %q needs a scalar Items type, got %q", p.Name, p.Items))
		}
	default:
		panic(fmt.Sprintf("toolregistry: ObjectSchema: prop %q has unsupported Type %q", p.Name, p.Type))
	}
	if len(p.Enum) > 0 && p.Type != "string" {
		panic(fmt.Sprintf("toolregistry: ObjectSchema: prop %q has Enum but Type %q is not \"string\"", p.Name, p.Type))
	}
}

// writeJSONString writes s to buf as a JSON string literal.
func writeJSONString(buf *bytes.Buffer, s string) {
	b, err := json.Marshal(s)
	if err != nil {
		// json.Marshal of a string cannot fail.
		panic(fmt.Sprintf("toolregistry: marshal string: %v", err))
	}
	buf.Write(b)
}

// writeJSONValue writes v (a []string here) to buf as JSON.
func writeJSONValue(buf *bytes.Buffer, v []string) {
	b, err := json.Marshal(v)
	if err != nil {
		// json.Marshal of a []string cannot fail.
		panic(fmt.Sprintf("toolregistry: marshal value: %v", err))
	}
	buf.Write(b)
}
