package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
