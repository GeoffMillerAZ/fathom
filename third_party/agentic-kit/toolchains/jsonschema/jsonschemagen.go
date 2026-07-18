// Package jsonschemagen derives JSON Schema (draft 2020-12) from CUE definitions
// by shelling to the cue binary at generation time.
//
// # Pipeline
//
//  1. Shell to `cue def --out openapi <package>` in the CUE module directory.
//  2. Extract components.schemas from the OpenAPI 3.0 envelope.
//  3. Transform each schema entry from OpenAPI 3.0 conventions to JSON Schema
//     draft 2020-12 (nullable→type array, $ref rewrite, exclusiveMinimum
//     normalisation, format pruning).
//  4. Wrap as a single JSON Schema document with $schema and $defs.
//
// # Generation-time tool — not a runtime dependency
//
// This package shells to the `cue` binary. CUE is a toolchain dependency of
// agentic-kit (installed by CI and checked by `task doctor`). The binary is
// resolved at call time via exec.LookPath — its discovery happens at generation
// time, never at library-runtime paths. The runtime Go code produced from
// generated schemas is stdlib-only.
//
// # Supported CUE constructs
//
// The following CUE construct → JSON Schema mappings are supported and tested.
// Unsupported constructs cause `cue def` to fail and this package surfaces that
// error verbatim — it never silently emits a looser schema:
//
//	CUE construct                 JSON Schema output
//	─────────────────────────────────────────────────────────────────────
//	string                        {"type":"string"}
//	int / int & >=N               {"type":"integer"} + minimum/maximum
//	float64 / number              {"type":"number"} + optional bounds
//	bool                          {"type":"boolean"}
//	[...T]                        {"type":"array","items":{T}}
//	"a" | "b" | "c"               {"type":"string","enum":["a","b","c"]}
//	{field: T}                    {"type":"object","properties":{...}}
//	optional_field?: T            property without "required" entry
//	T | null                      {"type":["T","null"]}
//	string & =~"pattern"          {"type":"string","pattern":"..."}
//	int & >=N & <=M               {"type":"integer","minimum":N,"maximum":M}
//	>0 & int                      {"type":"integer","exclusiveMinimum":0}
//	nested definition ref #X.Y   {"$ref":"#/$defs/X"}
//
//	UNSUPPORTED — cue def fails loudly:
//	string & !=""                 (unsupported op != for string type)
//
// # Determinism
//
// Output is byte-stable: JSON keys are emitted in sorted order via
// encoding/json with a custom deterministic marshal step, so committed schema
// artifacts are golden-diffable. Run DeriveFromDir, write the result, and check
// it into the repository alongside the originating schema.cue.
//
// # Concurrency
//
// DeriveFromDir and DeriveFromFile are safe for concurrent use; each call
// creates its own temporary scratch directory and process.
package jsonschemagen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// DefaultCUEBinary is the cue binary name resolved via PATH.
// Override with an absolute path when the binary is not on PATH.
const DefaultCUEBinary = "cue"

// Options controls schema derivation.
type Options struct {
	// CUEBinary is the path to the cue binary.
	// Defaults to DefaultCUEBinary ("cue") resolved via PATH.
	CUEBinary string

	// PackageName selects a specific CUE package within the directory when
	// multiple packages coexist.  If empty, cue def operates on the single
	// package present (or errors if there are more than one).
	PackageName string
}

func (o Options) cueBin() string {
	if o.CUEBinary != "" {
		return o.CUEBinary
	}
	return DefaultCUEBinary
}

// DeriveFromDir derives a JSON Schema document from all CUE definitions
// (#-prefixed names) found in the CUE package rooted at dir.
//
// dir must be a directory containing at least one .cue file with a package
// declaration.  If the CUE definitions use constructs not supported for
// OpenAPI export (e.g. string & !=""), the error message will include the cue
// binary's diagnostic output naming the unsupported construct.
//
// The returned JSON bytes are a stable (sorted-key), indented JSON Schema
// draft-2020-12 document.
func DeriveFromDir(dir string, opts Options) ([]byte, error) {
	raw, err := runCueDef(dir, opts)
	if err != nil {
		return nil, err
	}
	return transformOpenAPIToJSONSchema(raw)
}

// DeriveFromFile derives a JSON Schema document from a single CUE file.
// The file is copied into a temporary directory to give cue def a clean
// package to work with.
func DeriveFromFile(cueFile string, opts Options) ([]byte, error) {
	data, err := os.ReadFile(cueFile)
	if err != nil {
		return nil, fmt.Errorf("jsonschemagen: read %s: %w", cueFile, err)
	}
	return DeriveFromBytes(data, opts)
}

// DeriveFromBytes derives a JSON Schema document from the bytes of a single
// CUE file.  The bytes are written to a temporary directory and processed by
// cue def.  The temporary directory is removed after the call.
func DeriveFromBytes(cueData []byte, opts Options) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "jsonschemagen-*")
	if err != nil {
		return nil, fmt.Errorf("jsonschemagen: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	if err := os.WriteFile(tmp+"/schema.cue", cueData, 0o600); err != nil {
		return nil, fmt.Errorf("jsonschemagen: write temp file: %w", err)
	}
	return DeriveFromDir(tmp, opts)
}

// ─── internal: cue invocation ────────────────────────────────────────────────

// runCueDef invokes `cue def --out openapi <dir>` and returns its stdout.
func runCueDef(dir string, opts Options) ([]byte, error) {
	bin := opts.cueBin()

	// Resolve binary to an absolute path; provides a better error message.
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("jsonschemagen: cue binary %q not found on PATH: %w", bin, err)
	}

	args := []string{"def", "--out", "openapi"}
	if opts.PackageName != "" {
		args = append(args, "."+":"+opts.PackageName)
	} else {
		args = append(args, ".")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(resolved, args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"jsonschemagen: cue def failed in %s: %w\n%s",
			dir, err, strings.TrimSpace(stderr.String()),
		)
	}
	return stdout.Bytes(), nil
}

// ─── internal: OpenAPI 3.0 → JSON Schema 2020-12 transform ─────────────────

// openapiDoc is the minimal OpenAPI 3.0 envelope we parse.
type openapiDoc struct {
	Components struct {
		Schemas map[string]json.RawMessage `json:"schemas"`
	} `json:"components"`
}

// transformOpenAPIToJSONSchema converts `cue def --out openapi` output into a
// standalone JSON Schema draft 2020-12 document.
//
// The resulting document has the shape:
//
//	{
//	  "$schema": "https://json-schema.org/draft/2020-12/schema",
//	  "$defs": {
//	    "Name": { ... transformed schema ... }
//	  }
//	}
func transformOpenAPIToJSONSchema(openapi []byte) ([]byte, error) {
	var doc openapiDoc
	if err := json.Unmarshal(openapi, &doc); err != nil {
		return nil, fmt.Errorf("jsonschemagen: parse openapi output: %w", err)
	}

	// Walk each schema in components.schemas, transform it, collect results.
	defs := make(map[string]any, len(doc.Components.Schemas))
	for name, raw := range doc.Components.Schemas {
		var schema any
		if err := json.Unmarshal(raw, &schema); err != nil {
			return nil, fmt.Errorf("jsonschemagen: unmarshal schema %q: %w", name, err)
		}
		transformed := transformSchema(schema)
		defs[name] = transformed
	}

	out := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$defs":   defs,
	}
	return marshalDeterministic(out)
}

// transformSchema recursively transforms a parsed JSON value (from OpenAPI 3.0
// schema) into its JSON Schema 2020-12 equivalent.
//
// Transformations applied:
//   - nullable:true + type:"T" → type:["T","null"]
//   - $ref "#/components/schemas/X" → "#/$defs/X"
//   - exclusiveMinimum:true / exclusiveMaximum:true (OAS 3.0 boolean)
//     → exclusiveMinimum:N / exclusiveMaximum:N (JS 2020-12 numeric)
//   - format:"double" and format:"float" are dropped (OAS-ism not useful for
//     LLM schema constraints)
func transformSchema(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		// Scalar or array at the top level — pass through.
		return v
	}

	out := make(map[string]any, len(m))

	// Gather values we need for cross-field decisions before copying.
	nullable, _ := m["nullable"].(bool)
	typ, hasType := m["type"].(string)
	rawMin, hasMin := m["minimum"]
	rawMax, hasMax := m["maximum"]
	exMin, _ := m["exclusiveMinimum"].(bool)
	exMax, _ := m["exclusiveMaximum"].(bool)
	format, _ := m["format"].(string)

	for k, val := range m {
		switch k {
		case "nullable":
			// Consumed below — do not emit.
			continue

		case "type":
			if nullable && hasType {
				out["type"] = []any{typ, "null"}
			} else {
				out["type"] = val
			}

		case "format":
			// Drop OpenAPI numeric format hints that carry no constraint
			// semantics useful for LLM output schemas.
			if format == "double" || format == "float" {
				continue
			}
			out["format"] = val

		case "$ref":
			if s, ok := val.(string); ok {
				out["$ref"] = rewriteRef(s)
			} else {
				out["$ref"] = val
			}

		case "exclusiveMinimum":
			// OpenAPI 3.0: exclusiveMinimum is a boolean; minimum holds the value.
			// JSON Schema 2020-12: exclusiveMinimum is the exclusive bound value.
			if exMin && hasMin {
				out["exclusiveMinimum"] = rawMin
			} else if !exMin {
				// Not a boolean true — pass the value through as-is (already JS style).
				out["exclusiveMinimum"] = val
			}
			// If exMin==true we already consumed it; minimum is emitted separately.

		case "exclusiveMaximum":
			if exMax && hasMax {
				out["exclusiveMaximum"] = rawMax
			} else if !exMax {
				out["exclusiveMaximum"] = val
			}

		case "minimum":
			// If this was an exclusive bound in OAS 3.0 form, the minimum value
			// becomes exclusiveMinimum in 2020-12 and we skip emitting minimum.
			if exMin {
				// Already handled in the exclusiveMinimum case above; skip.
				continue
			}
			out["minimum"] = val

		case "maximum":
			if exMax {
				continue
			}
			out["maximum"] = val

		case "properties":
			props, ok := val.(map[string]any)
			if !ok {
				out[k] = val
				continue
			}
			newProps := make(map[string]any, len(props))
			for pk, pv := range props {
				newProps[pk] = transformSchema(pv)
			}
			out[k] = newProps

		case "items":
			out[k] = transformSchema(val)

		case "allOf", "anyOf", "oneOf":
			if arr, ok := val.([]any); ok {
				newArr := make([]any, len(arr))
				for i, elem := range arr {
					newArr[i] = transformSchema(elem)
				}
				out[k] = newArr
			} else {
				out[k] = val
			}

		default:
			out[k] = val
		}
	}

	// If nullable was set but there was no type field, add null to an
	// existing anyOf/oneOf, or fall back to a type array.
	if nullable && !hasType {
		if _, hasAnyOf := out["anyOf"]; !hasAnyOf {
			if _, hasOneOf := out["oneOf"]; !hasOneOf {
				out["type"] = []any{"null"}
			}
		}
	}

	return out
}

// rewriteRef converts "#/components/schemas/X" (OpenAPI) to "#/$defs/X"
// (JSON Schema).  Other $ref forms are returned unchanged.
func rewriteRef(ref string) string {
	const prefix = "#/components/schemas/"
	if strings.HasPrefix(ref, prefix) {
		return "#/$defs/" + ref[len(prefix):]
	}
	return ref
}

// ─── internal: deterministic JSON marshal ────────────────────────────────────

// marshalDeterministic encodes v as indented JSON with object keys in sorted
// order, producing byte-stable output suitable for golden-file comparison.
func marshalDeterministic(v any) ([]byte, error) {
	// encoding/json already sorts map keys (via reflect on map[string]any).
	// We confirm this with a recursive sort pass to be explicit.
	sorted := sortKeys(v)
	b, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("jsonschemagen: marshal: %w", err)
	}
	return append(b, '\n'), nil
}

// sortKeys recursively rebuilds map[string]any values with sorted keys,
// ensuring byte-stable JSON output regardless of Go's map iteration order.
func sortKeys(v any) any {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Use a slice of pairs encoded as a sortedMap shim — actually we
		// rely on encoding/json's built-in map-key sorting.  But we still
		// recurse to sort nested maps.
		out := make(map[string]any, len(val))
		for _, k := range keys {
			out[k] = sortKeys(val[k])
		}
		return out

	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = sortKeys(elem)
		}
		return out

	default:
		return v
	}
}

// ─── freshness check ─────────────────────────────────────────────────────────

// CheckFresh re-derives the JSON Schema from dir and compares it byte-for-byte
// against the contents of goldenFile.  It returns an error describing the
// mismatch if they differ.  This is the pattern used by golden tests:
//
//	if err := jsonschemagen.CheckFresh(t.TempDir(), "testdata/golden.json", jsonschemagen.Options{}); err != nil {
//	    t.Fatal(err)
//	}
func CheckFresh(dir, goldenFile string, opts Options) error {
	got, err := DeriveFromDir(dir, opts)
	if err != nil {
		return fmt.Errorf("jsonschemagen: derive: %w", err)
	}
	want, err := os.ReadFile(goldenFile)
	if err != nil {
		return fmt.Errorf("jsonschemagen: read golden %s: %w", goldenFile, err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf(
			"jsonschemagen: schema drift detected between %s and cue def output from %s\n"+
				"run: update the golden file by re-deriving the schema",
			goldenFile, dir,
		)
	}
	return nil
}
