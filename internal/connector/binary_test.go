package connector_test

// binary_test.go — unit coverage for connector.IsBinary (ADR-021 §1 / LT-2):
// the NUL-byte-in-first-8000-bytes heuristic that lets fathom exclude binary
// content from extraction and from cloc's line counts. Precedent:
// agentic-kit's cmd/constitution-lint looksBinary.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

func TestIsBinary_PlainTextIsNotBinary(t *testing.T) {
	if connector.IsBinary([]byte("package main\n\nfunc main() {}\n")) {
		t.Error("plain Go source misclassified as binary")
	}
}

func TestIsBinary_Empty(t *testing.T) {
	if connector.IsBinary(nil) {
		t.Error("empty content misclassified as binary")
	}
	if connector.IsBinary([]byte{}) {
		t.Error("empty slice misclassified as binary")
	}
}

func TestIsBinary_NULInFirstBytes(t *testing.T) {
	data := []byte("MZ\x00\x00this looks like a binary header")
	if !connector.IsBinary(data) {
		t.Error("NUL byte near the start was not detected as binary")
	}
}

func TestIsBinary_NULFarBeyondSniffWindow(t *testing.T) {
	// A NUL byte after the first 8000 bytes is out of the sniff window — the
	// heuristic is a cheap prefix check, not a full-file scan.
	text := strings.Repeat("a", 8500)
	data := []byte(text)
	data = append(data, 0x00)
	if connector.IsBinary(data) {
		t.Error("NUL byte beyond the sniff window should not flip the classification")
	}
}

func TestIsBinary_NULAtWindowBoundary(t *testing.T) {
	// A NUL byte at exactly the 8000th byte (index 7999) is inside the window.
	data := bytes.Repeat([]byte("a"), 8000)
	data[7999] = 0x00
	if !connector.IsBinary(data) {
		t.Error("NUL byte at the last in-window index should be detected")
	}
}

func TestIsBinary_ShortFileNoNUL(t *testing.T) {
	if connector.IsBinary([]byte("hi")) {
		t.Error("short NUL-free content misclassified as binary")
	}
}
