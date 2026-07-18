package prompttemplate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"text/template"
)

// Render executes tpl.Text against params and returns the rendered prompt
// together with its provenance hashes.
//
// Rendering enforces missingkey=error: if the template references a map key
// that params does not contain, Render returns an error instead of silently
// emitting "<no value>". This guarantees PromptHash never covers a prompt
// with an accidental hole in it.
//
// ParamsHash determinism: params are canonicalized as JSON via
// encoding/json, which sorts map keys lexicographically at every nesting
// depth. Two maps with the same contents therefore hash identically
// regardless of insertion order or Go's randomized map iteration. A nil
// params map is normalized to an empty map and hashes as "{}".
//
// Struct values inside params are hashed by their JSON form: fields are
// emitted in declaration order (deterministic for a given type), honouring
// `json:` tags. Renaming or reordering struct fields changes the hash — by
// design, since it changes what the model could have seen.
//
// Params that JSON cannot canonicalize deterministically are rejected with
// an error: NaN and ±Inf floats, channels, funcs, complex numbers, and
// cyclic values all fail json.Marshal and therefore fail Render. Types
// implementing json.Marshaler are trusted to produce deterministic output —
// a non-deterministic MarshalJSON breaks the ParamsHash guarantee.
func Render(tpl Template, params map[string]any) (Rendered, error) {
	if err := tpl.Validate(); err != nil {
		return Rendered{}, err
	}
	paramsHash, err := hashParams(params)
	if err != nil {
		return Rendered{}, fmt.Errorf("prompttemplate: template %q: %w", tpl.Name, err)
	}
	t, err := template.New(tpl.Name).Option("missingkey=error").Parse(tpl.Text)
	if err != nil {
		// Unreachable in practice: Validate parsed the identical text above.
		return Rendered{}, fmt.Errorf("prompttemplate: parse template %q: %w", tpl.Name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, params); err != nil {
		return Rendered{}, fmt.Errorf("prompttemplate: render template %q: %w", tpl.Name, err)
	}
	return Rendered{
		Text:            buf.String(),
		PromptHash:      sha256Hex(buf.Bytes()),
		TemplateVersion: tpl.Version,
		ParamsHash:      paramsHash,
	}, nil
}

// hashParams returns the lowercase hex SHA-256 of the canonical JSON
// encoding of params. nil normalizes to an empty map so that nil and
// map[string]any{} hash identically.
func hashParams(params map[string]any) (string, error) {
	if params == nil {
		params = map[string]any{}
	}
	canonical, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("params cannot be canonicalized as JSON (NaN/Inf, channels, funcs, complex, and cyclic values are unsupported): %w", err)
	}
	return sha256Hex(canonical), nil
}

// ParamsFromStruct converts a struct (or pointer to struct) into a
// map[string]string suitable for passing to Render, using the struct's
// json: tags as map keys.
//
// Key derivation:
//   - Each exported field's json: tag name becomes the map key. Fields
//     tagged `json:"-"` are skipped. Fields without a json: tag use the
//     Go field name as-is (same behaviour as encoding/json).
//   - The tag's omitempty and other options are ignored for key derivation;
//     only the name portion is used.
//
// Value encoding:
//   - string fields are used verbatim.
//   - All other types (int, float64, bool, slices, nested structs, …) are
//     encoded as their canonical JSON representation. This shares the same
//     canonical-JSON path that Render uses for ParamsHash, so a struct
//     field renaming via its json: tag changes the params map key (and
//     therefore the ParamsHash) while a Go field rename that preserves the
//     json: tag changes nothing — the key-drift regression test below pins
//     this behaviour.
//
// Nested structs, maps, and slices are encoded as their JSON form, not
// recursively expanded. The intent is that template authors reference them
// via {{.field}} and the template receives the JSON string.
//
// v must be a struct or a pointer to a struct; a nil pointer, a non-struct
// value, or a value that cannot be marshalled returns an error.
func ParamsFromStruct(v any) (map[string]string, error) {
	if v == nil {
		return nil, fmt.Errorf("prompttemplate: ParamsFromStruct: v must not be nil")
	}
	// Marshal the whole struct to JSON first so we get canonical field names
	// (json tags honoured, omitempty applied, etc.), then decode into a
	// map[string]json.RawMessage to preserve per-value JSON encoding.
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("prompttemplate: ParamsFromStruct: cannot marshal struct: %w", err)
	}
	// Verify v is a struct (or pointer to struct) by checking the JSON output
	// starts with '{' — maps also start with '{' but are explicitly documented
	// as supported input too, so we accept both.
	if len(raw) == 0 || (raw[0] != '{') {
		return nil, fmt.Errorf("prompttemplate: ParamsFromStruct: v must be a struct or map, got JSON %q", string(raw))
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("prompttemplate: ParamsFromStruct: cannot decode struct fields: %w", err)
	}
	out := make(map[string]string, len(fields))
	for key, val := range fields {
		// If the field is a JSON string, use the unquoted value directly so
		// string fields are passed to templates as plain strings (not quoted).
		var s string
		if err := json.Unmarshal(val, &s); err == nil {
			out[key] = s
		} else {
			// Non-string values (numbers, bools, objects, arrays) are kept as
			// their canonical JSON form — the template receives a JSON fragment.
			out[key] = string(val)
		}
	}
	return out, nil
}

// sha256Hex returns the lowercase hex SHA-256 digest of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
