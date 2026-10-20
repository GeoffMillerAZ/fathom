package main

import (
	"bytes"
	"os"
	"testing"
)

func TestCmdProvenance(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cmdProvenance([]string{"-root", "../.."}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	want, err := os.ReadFile("../../" + provenanceDoc)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatal("provenance output differs from PROVENANCE.md; run task regen")
	}
	var o2, e2 bytes.Buffer
	if code := cmdProvenance([]string{"-root", t.TempDir()}, &o2, &e2); code == 0 {
		t.Fatal("a root without the document must fail")
	}
}
