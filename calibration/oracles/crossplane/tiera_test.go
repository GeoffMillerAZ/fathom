package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWriteTierAReportsFilteredWalkAsTierC(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "crossplane")
	script := `#!/bin/sh
if [ "$1" = version ]; then
  echo 'Crossplane CLI v2.5.0'
  exit 0
fi
case "${9##*/}" in
  accepted.yaml) printf '%s\n' '{"resources":[{"kind":"Composition","name":"accepted","status":"valid"}]}' ;;
  refused.yaml) printf '%s\n' '{"resources":[{"kind":"Composition","name":"refused","status":"invalid"}]}' ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	input := out{
		Tool: "yaml walk", Version: "walk version", Tier: "C",
		Definitions: []def{
			{Path: "accepted.yaml", Name: "Composition/accepted", Line: 2, Kind: "resource"},
			{Path: "accepted.yaml", Name: "step/compose", Line: 12, Kind: "step"},
			{Path: "accepted.yaml", Name: "element/composed", Line: 18, Kind: "element"},
			{Path: "refused.yaml", Name: "Composition/refused", Line: 2, Kind: "resource"},
		},
		References:      []ref{{Path: "accepted.yaml", From: "Composition/accepted", To: "Function/compose"}},
		CrossReferences: []xref{{Path: "accepted.yaml", From: "Composition/accepted", ToPath: "function.yaml", To: "Function/compose"}},
	}
	refsFile := filepath.Join(dir, "refs.json")
	filesFile := filepath.Join(dir, "files.txt")
	stdout, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = stdoutWriter
	t.Cleanup(func() { os.Stdout = oldStdout })
	err = writeTierA(input, []string{"accepted.yaml", "refused.yaml"}, dir, bin, dir, dir, refsFile, filesFile)
	if closeErr := stdoutWriter.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	var definitions out
	if err := json.Unmarshal(encoded, &definitions); err != nil {
		t.Fatal(err)
	}
	if definitions.Tier != "C" {
		t.Fatalf("native validation filters yaml-walk definitions, so tier = %q, want C", definitions.Tier)
	}
	wantDefs := input.Definitions[:2]
	if !reflect.DeepEqual(definitions.Definitions, wantDefs) {
		t.Fatalf("definitions = %#v, want %#v", definitions.Definitions, wantDefs)
	}
	if !strings.Contains(definitions.Tool, "crossplane resource validate") || definitions.Version != "Crossplane CLI v2.5.0" {
		t.Fatalf("validation provenance = %q / %q", definitions.Tool, definitions.Version)
	}
	refsJSON, err := os.ReadFile(refsFile)
	if err != nil {
		t.Fatal(err)
	}
	var refs out
	if err := json.Unmarshal(refsJSON, &refs); err != nil {
		t.Fatal(err)
	}
	if len(refs.Definitions) != 0 || !reflect.DeepEqual(refs.References, input.References) || !reflect.DeepEqual(refs.CrossReferences, input.CrossReferences) {
		t.Fatalf("refs output = %#v", refs)
	}
	acceptedFiles, err := os.ReadFile(filesFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(acceptedFiles) != "accepted.yaml\n" {
		t.Fatalf("accepted files = %q", acceptedFiles)
	}
}

// validate's JSON report decides a file: no resource invalid, and every
// Crossplane object of the file valid; a missing schema is tolerated only for
// what is not one of the file's objects (a function's input, whose schema
// needs a registry).
func TestValidateVerdict(t *testing.T) {
	report := []byte(`{"summary":{"total":2,"valid":1,"invalid":0,"missingSchemas":1},"resources":[
 {"apiVersion":"dummy.fn.crossplane.io/v1beta1","kind":"Response","name":"","status":"missingSchema"},
 {"apiVersion":"apiextensions.crossplane.io/v1","kind":"Composition","name":"c","status":"valid"}]}`)
	ok, why, inputs := verdict(report, []string{"Composition/c"})
	if !ok || inputs != 1 {
		t.Fatalf("a valid composition with an unvalidated function input: %v %q %d", ok, why, inputs)
	}
	if ok, _, _ := verdict(report, []string{"Composition/c", "Configuration/pkg"}); ok {
		t.Fatal("an object of the file with no valid result must refuse the file")
	}
	bad := []byte(`{"summary":{"total":1,"valid":0,"invalid":1,"missingSchemas":0},"resources":[
 {"apiVersion":"apiextensions.crossplane.io/v1","kind":"Composition","name":"c","status":"invalid"}]}`)
	if ok, _, _ := verdict(bad, []string{"Composition/c"}); ok {
		t.Fatal("an invalid resource must refuse the file")
	}
}

// The tool decides, offline, against crossplane's own CRDs: a Composition
// with an unknown field is refused, a valid one accepted.
func TestValidateOffline(t *testing.T) {
	bin, err := exec.LookPath("crossplane")
	if err != nil {
		t.Skip("crossplane not on PATH")
	}
	crds := os.Getenv("OX_CROSSPLANE_CRDS")
	if crds == "" {
		t.Skip("OX_CROSSPLANE_CRDS (crossplane's cluster/crds) not set")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	bad := filepath.Join(dir, "bad.yaml")
	comp := "apiVersion: apiextensions.crossplane.io/v1\nkind: Composition\nmetadata:\n  name: x\nspec:\n  compositeTypeRef:\n    apiVersion: example.org/v1\n    kind: X\n  mode: Pipeline\n  pipeline:\n  - step: a\n    functionRef:\n      name: f\n"
	if err := os.WriteFile(good, []byte(comp), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte(comp+"  bogus: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := validate(bin, crds, t.TempDir(), good, []string{"Composition/x"}); err != nil {
		t.Fatalf("valid composition refused: %v", err)
	}
	if _, err := validate(bin, crds, t.TempDir(), bad, []string{"Composition/x"}); err == nil {
		t.Fatal("a composition with an unknown field must be refused")
	}
}
