package labs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLabClosure(t *testing.T) {
	dir := t.TempDir()
	m := labFixture()
	m["features"] = []string{"unrelated-FM-1"}
	writeLabJSON(t, dir, "lab-find-symbol.yaml", m)
	writeLabJSON(t, dir, "no-lab.yaml", map[string]any{"surfaces": []any{map[string]any{"surface": "mcp:search", "instrument": "indexbench L0"}, map[string]any{"surface": "hook:silent", "instrument": "none", "reason": "observability only"}}})
	if err := CheckLabClosure(dir, []string{"mcp:find_symbol", "mcp:search", "hook:silent"}); err != nil {
		t.Fatal(err)
	}
	// The same feature ID cannot cover a missing shipped surface.
	err := CheckLabClosure(dir, []string{"mcp:z-new", "mcp:search", "hook:silent", "mcp:a-new", "mcp:find_symbol"})
	if err == nil || !strings.Contains(err.Error(), "mcp:a-new, mcp:z-new") {
		t.Fatalf("want sorted closure gaps, got %v", err)
	}
}

func TestCheckLabClosureRefuses(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries any
		want    string
	}{
		{"unknown", []any{map[string]any{"surface": "mcp:typo", "instrument": "reviewbench"}}, "unknown surface"},
		{"bare", []any{map[string]any{"surface": "mcp:search"}}, "instrument or a reason"},
		{"none-bare", []any{map[string]any{"surface": "mcp:search", "instrument": "none"}}, "instrument or a reason"},
		{"whitespace", []any{map[string]any{"surface": "mcp:search", "reason": "  "}}, "instrument or a reason"},
		{"duplicate", []any{map[string]any{"surface": "mcp:search", "reason": "not applicable"}, map[string]any{"surface": "mcp:search", "reason": "not applicable"}}, "duplicate"},
		{"typo-field", []any{map[string]any{"surface": "mcp:search", "instrument": "langbench", "typo": true}}, "field not allowed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeLabJSON(t, dir, "lab-find-symbol.yaml", labFixture())
			writeLabJSON(t, dir, "no-lab.yaml", map[string]any{"surfaces": tc.entries})
			err := CheckLabClosure(dir, []string{"mcp:find_symbol", "mcp:search"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
	dir := t.TempDir()
	writeLabJSON(t, dir, "lab-find-symbol.yaml", labFixture())
	if err := CheckLabClosure(dir, []string{"mcp:find_symbol"}); err != nil {
		t.Fatalf("no exclusion file needed when cards close inventory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "no-lab.yaml"), []byte("surfaces: [bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckLabClosure(dir, []string{"mcp:find_symbol"}); err == nil {
		t.Fatal("malformed exclusions accepted")
	}
	// Unknown card surfaces are refused by closure, too.
	writeLabJSON(t, dir, "no-lab.yaml", map[string]any{"surfaces": []any{}})
	if err := CheckLabClosure(dir, []string{"mcp:search"}); err == nil || !strings.Contains(err.Error(), "unknown surface") {
		t.Fatalf("unknown card surface: %v", err)
	}
}
