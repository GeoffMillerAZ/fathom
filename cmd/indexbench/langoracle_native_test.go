package main

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestNativeFilesWithError_RegoRepairs: the Rego reader's repair count is
// the reading's files_with_error (ADR-048's wave-pc amendment: its revisit
// trigger is a repair rate past 5% on a new subject, so lang-oracle measures
// it); a file read as written counts nothing, one the reader read only by a
// repair counts once, and every other dialect reports none.
func TestNativeFilesWithError_RegoRepairs(t *testing.T) {
	files := []core.SourceFile{
		{Path: "ok.rego", Content: []byte("package p\n\nallow if {\n\tinput.x\n}\n")},
		{Path: "open.rego", Content: []byte("package p\n\nallow if {\n\tinput.x\n\nafter := 1\n")},
		{Path: "str.rego", Content: []byte("package p\n\nmsg := \"open\n")},
	}
	n, ok := nativeFilesWithError("rego", files)
	if !ok || n != 2 {
		t.Fatalf("nativeFilesWithError(rego) = %d, %v; want 2, true", n, ok)
	}
	if n, ok := nativeFilesWithError("rego", files[:1]); !ok || n != 0 {
		t.Fatalf("nativeFilesWithError(rego, clean) = %d, %v; want 0, true", n, ok)
	}
	if _, ok := nativeFilesWithError("nginx", files); ok {
		t.Fatal("a native dialect with no repair count reports files_with_error")
	}
}
