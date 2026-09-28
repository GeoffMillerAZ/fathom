package main

import (
	"bytes"
	"os"
	"testing"
)

func TestCmdLangTracker(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cmdLangTracker([]string{"-root", "../.."}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	want, err := os.ReadFile("../../docs/languages.md")
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatal("lang-tracker output differs from docs/languages.md; regenerate it")
	}
}
