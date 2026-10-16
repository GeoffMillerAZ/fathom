package jsonschemagen_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/toolchains/jsonschema"
)

// skipIfNoCUE skips the test when the cue binary is not available.
// The repo gate has cue installed, so tests run in CI.
func skipIfNoCUE(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue binary not found on PATH — skipping (install cue to run these tests)")
	}
}

// testdataDir returns the absolute path to the testdata directory adjacent to
// this test file.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

// TestDeriveFromDir_CircuitBreaker is the end-to-end golden test.
// It derives JSON Schema from the testdata/circuitbreaker.cue fixture and
// compares the result byte-for-byte against circuitbreaker_golden.json.
//
// To regenerate the golden file after a schema change:
//
//	go run ./cmd/regen-jsonschemagen   # (if that helper exists)
//
// or manually: run the test with -update flag logic, or derive and write.
func TestDeriveFromDir_CircuitBreaker(t *testing.T) {
	skipIfNoCUE(t)

	td := testdataDir(t)
	got, err := jsonschemagen.DeriveFromDir(td, jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}

	goldenPath := filepath.Join(td, "circuitbreaker_golden.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("schema mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestDeriveFromDir_ValidJSON verifies that the output is always valid JSON.
func TestDeriveFromDir_ValidJSON(t *testing.T) {
	skipIfNoCUE(t)

	got, err := jsonschemagen.DeriveFromDir(testdataDir(t), jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}
	if !json.Valid(got) {
		t.Errorf("output is not valid JSON:\n%s", got)
	}
}

// TestDeriveFromDir_SchemaKeyPresent verifies the $schema key is present and
// set to the JSON Schema 2020-12 URI.
func TestDeriveFromDir_SchemaKeyPresent(t *testing.T) {
	skipIfNoCUE(t)

	got, err := jsonschemagen.DeriveFromDir(testdataDir(t), jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	schemaURI, _ := doc["$schema"].(string)
	if schemaURI != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("$schema = %q, want 2020-12 URI", schemaURI)
	}
}

// TestDeriveFromDir_DefsPresent verifies that $defs is present and non-empty.
func TestDeriveFromDir_DefsPresent(t *testing.T) {
	skipIfNoCUE(t)

	got, err := jsonschemagen.DeriveFromDir(testdataDir(t), jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	defs, ok := doc["$defs"].(map[string]any)
	if !ok || len(defs) == 0 {
		t.Errorf("$defs missing or empty; doc keys: %v", doc)
	}
}

// TestDeriveFromDir_NoComponentsSchemas verifies that the OpenAPI envelope
// (components.schemas) is not present in the output.
func TestDeriveFromDir_NoComponentsSchemas(t *testing.T) {
	skipIfNoCUE(t)

	got, err := jsonschemagen.DeriveFromDir(testdataDir(t), jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, has := doc["components"]; has {
		t.Error("output contains 'components' key from OpenAPI envelope — should be stripped")
	}
}

// TestDeriveFromDir_ExclusiveMinimumTransform verifies the OAS 3.0
// exclusiveMinimum:true boolean form is converted to the 2020-12 numeric form.
func TestDeriveFromDir_ExclusiveMinimumTransform(t *testing.T) {
	skipIfNoCUE(t)

	got, err := jsonschemagen.DeriveFromDir(testdataDir(t), jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	defs := doc["$defs"].(map[string]any)
	config := defs["Config"].(map[string]any)
	props := config["properties"].(map[string]any)
	openCooldown := props["OpenCooldown"].(map[string]any)

	// In JSON Schema 2020-12, exclusiveMinimum should be a number, not a bool.
	exMin := openCooldown["exclusiveMinimum"]
	switch exMin.(type) {
	case bool:
		t.Errorf("exclusiveMinimum is still a bool (OAS 3.0 form); want numeric 2020-12 form")
	case float64, json.Number:
		// Good — numeric form.
	default:
		t.Errorf("exclusiveMinimum has unexpected type %T: %v", exMin, exMin)
	}

	// minimum key should NOT be present for the OpenCooldown field since it was
	// absorbed into exclusiveMinimum.
	if _, has := openCooldown["minimum"]; has {
		t.Error("OpenCooldown still has 'minimum' key after exclusiveMinimum transform")
	}
}

// TestDeriveFromBytes_RoundTrip verifies DeriveFromBytes produces the same
// result as DeriveFromDir for the same CUE content.
func TestDeriveFromBytes_RoundTrip(t *testing.T) {
	skipIfNoCUE(t)

	td := testdataDir(t)
	cueFile := filepath.Join(td, "circuitbreaker.cue")
	cueData, err := os.ReadFile(cueFile)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	fromDir, err := jsonschemagen.DeriveFromDir(td, jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}
	fromBytes, err := jsonschemagen.DeriveFromBytes(cueData, jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromBytes: %v", err)
	}

	if !bytes.Equal(fromDir, fromBytes) {
		t.Errorf("DeriveFromDir and DeriveFromBytes differ:\ndir:\n%s\nbytes:\n%s", fromDir, fromBytes)
	}
}

// TestDeriveFromFile_RoundTrip verifies DeriveFromFile matches DeriveFromDir.
func TestDeriveFromFile_RoundTrip(t *testing.T) {
	skipIfNoCUE(t)

	td := testdataDir(t)
	fromDir, err := jsonschemagen.DeriveFromDir(td, jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}
	fromFile, err := jsonschemagen.DeriveFromFile(filepath.Join(td, "circuitbreaker.cue"), jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromFile: %v", err)
	}
	if !bytes.Equal(fromDir, fromFile) {
		t.Errorf("DeriveFromDir and DeriveFromFile differ:\ndir:\n%s\nfile:\n%s", fromDir, fromFile)
	}
}

// TestDeriveFromDir_Deterministic verifies that repeated calls produce
// byte-identical output (no map iteration non-determinism).
func TestDeriveFromDir_Deterministic(t *testing.T) {
	skipIfNoCUE(t)

	td := testdataDir(t)
	first, err := jsonschemagen.DeriveFromDir(td, jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("first DeriveFromDir: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := jsonschemagen.DeriveFromDir(td, jsonschemagen.Options{})
		if err != nil {
			t.Fatalf("DeriveFromDir iteration %d: %v", i, err)
		}
		if !bytes.Equal(first, again) {
			t.Errorf("iteration %d produced different output", i)
		}
	}
}

// TestDeriveFromDir_BadCUE verifies that an unsupported CUE construct (the
// != operator on strings) causes DeriveFromDir to return an error that names
// the unsupported construct, rather than emitting a silently looser schema.
func TestDeriveFromDir_BadCUE(t *testing.T) {
	skipIfNoCUE(t)

	tmp := t.TempDir()
	badCUE := []byte(`package test

// #Strict uses the !="" constraint, which cue def cannot emit as OpenAPI.
#Strict: {
    name: string & !=""
}
`)
	if err := os.WriteFile(filepath.Join(tmp, "schema.cue"), badCUE, 0o600); err != nil {
		t.Fatalf("write bad CUE: %v", err)
	}
	_, err := jsonschemagen.DeriveFromDir(tmp, jsonschemagen.Options{})
	if err == nil {
		t.Fatal("expected error for unsupported CUE construct (string & !=\"\"), got nil")
	}
	// The error message should reference the unsupported op.
	if !bytes.Contains([]byte(err.Error()), []byte("unsupported op")) {
		t.Errorf("error %q does not mention unsupported op", err.Error())
	}
}

// TestDeriveFromDir_MissingBinary verifies that a missing cue binary returns
// a clear error rather than a panic or empty result.
func TestDeriveFromDir_MissingBinary(t *testing.T) {
	_, err := jsonschemagen.DeriveFromDir(testdataDir(t), jsonschemagen.Options{
		CUEBinary: "/nonexistent/path/to/cue",
	})
	if err == nil {
		t.Fatal("expected error for missing cue binary, got nil")
	}
}

// TestCheckFresh_Passes verifies CheckFresh reports no error when the golden
// file matches the derived schema.
func TestCheckFresh_Passes(t *testing.T) {
	skipIfNoCUE(t)

	td := testdataDir(t)
	if err := jsonschemagen.CheckFresh(td, filepath.Join(td, "circuitbreaker_golden.json"), jsonschemagen.Options{}); err != nil {
		t.Fatalf("CheckFresh reported stale: %v", err)
	}
}

// TestCheckFresh_DetectsDrift verifies CheckFresh returns an error when the
// golden file does not match the derived schema.
func TestCheckFresh_DetectsDrift(t *testing.T) {
	skipIfNoCUE(t)

	tmp := t.TempDir()
	// Write a stale golden file with wrong content.
	stale := []byte(`{"$schema":"stale"}` + "\n")
	goldenPath := filepath.Join(tmp, "stale_golden.json")
	if err := os.WriteFile(goldenPath, stale, 0o600); err != nil {
		t.Fatalf("write stale golden: %v", err)
	}

	err := jsonschemagen.CheckFresh(testdataDir(t), goldenPath, jsonschemagen.Options{})
	if err == nil {
		t.Fatal("CheckFresh did not detect drift against stale golden file")
	}
}

// TestNullable_Transform verifies that nullable:true fields are transformed to
// the JSON Schema 2020-12 type-array form.
func TestNullable_Transform(t *testing.T) {
	skipIfNoCUE(t)

	tmp := t.TempDir()
	cueData := []byte(`package test

#WithNull: {
    name: string | null
}
`)
	if err := os.WriteFile(filepath.Join(tmp, "schema.cue"), cueData, 0o600); err != nil {
		t.Fatalf("write CUE: %v", err)
	}
	got, err := jsonschemagen.DeriveFromDir(tmp, jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	defs := doc["$defs"].(map[string]any)
	withNull := defs["WithNull"].(map[string]any)
	props := withNull["properties"].(map[string]any)
	nameProp := props["name"].(map[string]any)

	typeVal := nameProp["type"]
	typeArr, ok := typeVal.([]any)
	if !ok {
		t.Fatalf("name.type = %T (%v), want []any for nullable field", typeVal, typeVal)
	}
	has := func(needle string) bool {
		for _, v := range typeArr {
			if s, ok := v.(string); ok && s == needle {
				return true
			}
		}
		return false
	}
	if !has("string") {
		t.Errorf("name.type %v missing 'string'", typeArr)
	}
	if !has("null") {
		t.Errorf("name.type %v missing 'null'", typeArr)
	}
	if _, hasNullable := nameProp["nullable"]; hasNullable {
		t.Error("name property still contains 'nullable' key (OAS-ism should be removed)")
	}
}

// TestRefRewrite verifies that $ref values are rewritten from OpenAPI paths to
// JSON Schema $defs paths.
func TestRefRewrite(t *testing.T) {
	skipIfNoCUE(t)

	tmp := t.TempDir()
	cueData := []byte(`package test

#Base: {
    id: string
}

#Derived: {
    base: #Base
    name: string
}
`)
	if err := os.WriteFile(filepath.Join(tmp, "schema.cue"), cueData, 0o600); err != nil {
		t.Fatalf("write CUE: %v", err)
	}
	got, err := jsonschemagen.DeriveFromDir(tmp, jsonschemagen.Options{})
	if err != nil {
		t.Fatalf("DeriveFromDir: %v", err)
	}

	// Confirm no "#/components/schemas/" refs remain.
	if bytes.Contains(got, []byte("#/components/schemas/")) {
		t.Errorf("output still contains OpenAPI $ref path:\n%s", got)
	}
	// Confirm "#/$defs/" is present.
	if !bytes.Contains(got, []byte("#/$defs/")) {
		t.Errorf("output does not contain JSON Schema $defs $ref:\n%s", got)
	}
}
