package labs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func labFixture() map[string]any {
	return map[string]any{
		"id": "lab-find-symbol", "features": []string{"F-01"}, "surface": []string{"mcp:find_symbol"},
		"firing":  map[string]any{"should": "definition lookup", "should_not": "literal text", "grader": "tool_used"},
		"helped":  map[string]any{"metric": "calls to definition", "arm": "plugin-minus-find-symbol", "direction": "decrease", "minimum_effect_of_interest": 1},
		"quality": []any{}, "fixtures": []string{"internal/connector/testdata/fauxtown"}, "harness": []string{"fork"},
		"kill":  []any{map[string]any{"criterion": "K4", "reading": "docs/bench/readings/labs/lab-find-symbol.md"}},
		"knobs": []any{}, "owners": []string{},
	}
}

func writeLabJSON(t *testing.T, dir, name string, data any) {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLabs(t *testing.T) {
	dir := t.TempDir()
	writeLabJSON(t, dir, "lab-find-symbol.yaml", labFixture())
	// Features are cross-references, so an FM-1 identifier is valid too.
	other := labFixture()
	other["id"] = "lab-search"
	other["surface"] = []string{"mcp:search"}
	other["features"] = []string{"FM-1-future-id"}
	other["helped"].(map[string]any)["arm"] = "plugin-minus-search"
	writeLabJSON(t, dir, "lab-search.yaml", other)
	writeLabJSON(t, dir, "no-lab.yaml", map[string]any{"not": "a lab card"})
	cards, err := LoadLabs(dir, []string{"mcp:find_symbol", "mcp:search"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 || cards[0].ID != "lab-find-symbol" || cards[1].ID != "lab-search" {
		t.Fatalf("unsorted or missing cards: %+v", cards)
	}
}

func TestLoadLabsRefuses(t *testing.T) {
	for _, tc := range []struct {
		name, file, want string
		mutate           func(map[string]any)
	}{
		{"filename", "other.yaml", "file name", func(m map[string]any) {}},
		{"surface", "lab-find-symbol.yaml", "unknown surface", func(m map[string]any) { m["surface"] = []string{"mcp:typo"} }},
		{"should-not", "lab-find-symbol.yaml", "should_not", func(m map[string]any) { delete(m["firing"].(map[string]any), "should_not") }},
		{"broad-arm", "lab-find-symbol.yaml", "helped arm", func(m map[string]any) { m["helped"].(map[string]any)["arm"] = "plugin-minus-find-symbol-and-search" }},
		{"whole-plugin", "lab-find-symbol.yaml", "helped arm", func(m map[string]any) { m["helped"].(map[string]any)["arm"] = "no-plugin" }},
		{"kill", "lab-find-symbol.yaml", "kill", func(m map[string]any) { m["kill"] = []any{} }},
		{"unknown-field", "lab-find-symbol.yaml", "field not allowed", func(m map[string]any) { m["typo"] = true }},
		{"na-reason", "lab-find-symbol.yaml", "reason", func(m map[string]any) { m["firing"] = map[string]any{"mode": "n/a"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			m := labFixture()
			tc.mutate(m)
			writeLabJSON(t, dir, tc.file, m)
			_, err := LoadLabs(dir, []string{"mcp:find_symbol"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestLoadLabsNAAndInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	m := labFixture()
	m["firing"] = map[string]any{"mode": "n/a", "reason": "always loaded"}
	writeLabJSON(t, dir, "lab-find-symbol.yaml", m)
	if _, err := LoadLabs(dir, []string{"mcp:find_symbol"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lab-find-symbol.yaml"), []byte("id: [broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLabs(dir, []string{"mcp:find_symbol"}); err == nil {
		t.Fatal("accepted broken YAML")
	}
	if _, err := LoadLabs(filepath.Join(dir, "missing"), nil); err == nil {
		t.Fatal("accepted missing directory")
	}
}

func TestLoadLabsSingleMemberArms(t *testing.T) {
	for _, tc := range []struct {
		id, arm  string
		surfaces []string
	}{
		{"lab-skills", "plugin-minus-skill:{skill}", []string{"skill:fathom-plan", "skill:fathom-verify"}},
		{"lab-decision-port", "plugin-minus-decide:{site}", []string{"decide:route", "decide:assess"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			dir := t.TempDir()
			m := labFixture()
			m["id"] = tc.id
			m["surface"] = tc.surfaces
			m["helped"].(map[string]any)["arm"] = tc.arm
			writeLabJSON(t, dir, tc.id+".yaml", m)
			if _, err := LoadLabs(dir, tc.surfaces); err != nil {
				t.Fatal(err)
			}
			for _, arm := range []string{"plugin-minus-" + strings.TrimPrefix(tc.id, "lab-"), tc.arm + "-and-another"} {
				m["helped"].(map[string]any)["arm"] = arm
				writeLabJSON(t, dir, tc.id+".yaml", m)
				if _, err := LoadLabs(dir, tc.surfaces); err == nil {
					t.Errorf("broad arm %q accepted", arm)
				}
			}
		})
	}
}
