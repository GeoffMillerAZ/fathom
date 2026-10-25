package fusion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// TestBinaryGateExemptsClaimedContainers: the one binary rule (ADR-021 (1), amended
// 2026-10-24) excludes NUL-sniffed content unless a dialect claims its extension as an
// OPC zip model container and the bytes open as a zip package.
func TestBinaryGateExemptsClaimedContainers(t *testing.T) {
	rules := []connector.DialectRule{{ID: "m", Extensions: []string{".slx", ".mdl"}, Containers: []string{".slx"}}}
	zipBytes := []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00")
	cases := []struct {
		path string
		data []byte
		want bool
	}{
		{"m/plant.slx", zipBytes, false},
		{"m/plant.mdl", zipBytes, true},
		{"m/tool.bin", []byte("\x7fELF\x00\x00"), true},
		{"m/plant.slx", []byte("\x7fELF\x00\x00"), true},
		{"m/plant.slx", []byte("Model {\n}\n"), false}, // text is never binary
	}
	for _, c := range cases {
		if got := binaryExcludedWith(rules, c.path, c.data); got != c.want {
			t.Errorf("binaryExcludedWith(%s) = %v, want %v", c.path, got, c.want)
		}
	}
	// No embedded manifest claims a container today, so the shipped gate excludes a .slx.
	if !binaryExcluded("m/plant.slx", zipBytes) && !claimsContainer(".slx") {
		t.Error("an unclaimed .slx passed the shipped gate")
	}
}

func claimsContainer(ext string) bool {
	for _, r := range dialectRules() {
		for _, e := range r.Containers {
			if strings.EqualFold(e, ext) {
				return true
			}
		}
	}
	return false
}

// TestBinaryGateIsTheOnlySniff: every fusion read path asks binaryExcluded, never
// connector.IsBinary directly, so the container exemption is one rule, not nineteen.
func TestBinaryGateIsTheOnlySniff(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "binarygate.go" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "connector.IsBinary(") {
			t.Errorf("%s calls connector.IsBinary; use binaryExcluded (binarygate.go)", f)
		}
	}
}
