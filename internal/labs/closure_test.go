package labs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
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

func TestCheckLabClosureExclusiveOwnership(t *testing.T) {
	for _, kind := range []string{"cards", "exemption"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			writeLabJSON(t, dir, "lab-find-symbol.yaml", labFixture())
			if kind == "cards" {
				other := labFixture()
				other["id"] = "lab-search"
				other["helped"].(map[string]any)["arm"] = "plugin-minus-search"
				writeLabJSON(t, dir, "lab-search.yaml", other)
			} else {
				writeLabJSON(t, dir, "no-lab.yaml", map[string]any{"surfaces": []any{map[string]any{"surface": "mcp:find_symbol", "instrument": "none", "reason": "test conflict"}}})
			}
			if err := CheckLabClosure(dir, []string{"mcp:find_symbol"}); err == nil || !strings.Contains(err.Error(), "already claimed") {
				t.Fatalf("shared feature accepted: %v", err)
			}
		})
	}
}

// An exemption names an instrument that reads this surface, or an explicit
// reason measurement is absent. Plans for a future reader are not a reader.
func TestLabNoLabInstrumentEvidence(t *testing.T) {
	var data struct {
		Surfaces []NoLabSurface `json:"surfaces"`
	}
	dir := filepath.Join("..", "..", "calibration", "labs.d")
	if err := loadLabYAML(filepath.Join(dir, "no-lab.yaml"), cuecontext.New().CompileString("{...}"), &data); err != nil {
		t.Fatal(err)
	}
	readers := map[string]string{
		"indexbench L0": "internal/indexbench/oracle.go",
		"indexbench L1": "internal/indexbench/testdata/l1-replay.json",
	}
	for _, entry := range data.Surfaces {
		if strings.TrimSpace(entry.Reason) == "" {
			t.Errorf("%s has no reason", entry.Surface)
		}
		if strings.Contains(entry.Surface, "--harness ") && entry.Instrument != "none" {
			t.Errorf("%s is outside Claude-only measurement", entry.Surface)
		}
		if entry.Instrument == "none" {
			continue
		}
		reader, ok := readers[entry.Instrument]
		if !ok {
			t.Errorf("%s: unproven reader %s", entry.Surface, entry.Instrument)
			continue
		}
		source, err := os.ReadFile(filepath.Join("..", "..", reader))
		if err != nil {
			t.Fatal(err)
		}
		tool := strings.TrimPrefix(entry.Surface, "mcp:")
		if !strings.HasPrefix(entry.Surface, "mcp:") || !strings.Contains(string(source), `"`+tool+`"`) {
			t.Errorf("%s does not read %s", reader, entry.Surface)
		}
	}
}
