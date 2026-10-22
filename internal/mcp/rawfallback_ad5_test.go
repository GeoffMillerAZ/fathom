package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAD5RawFreshnessRefindsLiveHeading(t *testing.T) {
	for _, snippets := range []bool{false, true} {
		t.Run(map[bool]string{false: "no snippets", true: "snippets"}[snippets], func(t *testing.T) {
			c, root := citationFixture(t)
			path := filepath.Join(root, "AGENTS.md")
			before := "# Background\nContext.\n\n# Rules\nAlways sort outputs.\n\n"
			after := "# Rules\nAlways sort outputs.\n\n# Background\nContext.\n\n"
			if err := os.WriteFile(path, []byte(before), 0600); err != nil {
				t.Fatal(err)
			}
			citationGit(t, root, "add", "AGENTS.md")
			citationGit(t, root, "commit", "-qm", "guide")
			if err := c.IngestServiceAt(t.Context(), "sample", root); err != nil {
				t.Fatal(err)
			}
			args := map[string]any{"service": "sample", "file": "sample.go", "include_snippets": snippets}
			citationCall(t, c, "obligations", args) // populate the existing corpus memo
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(after), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
				t.Fatal(err)
			}
			p, _ := citationCall(t, c, "obligations", args)
			rows, _ := p["rules"].([]any)
			if len(rows) != 1 {
				t.Fatalf("rules = %v, want one instruction", p)
			}
			row := rows[0].(map[string]any)
			if row["rule"] != "Always sort outputs." || row["src"] != "AGENTS.md#L1-L3" {
				t.Errorf("live row = %v", row)
			}
			requireCitationState(t, row, "stale")
			if snippets && !strings.Contains(row["snippet"].(string), "Always sort outputs.") {
				t.Errorf("snippet = %v", row)
			}
		})
	}
}

func TestAD5RawWithheldCountOnReceipt(t *testing.T) {
	c, root := citationFixture(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Heading only\n\n# Rules\nAlways sort outputs.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p, _ := citationCall(t, c, "obligations", map[string]any{"service": "sample", "file": "sample.go", "include_snippets": false})
	if got := p["raw_withheld"]; got == nil || got.(json.Number).String() != "1" {
		t.Errorf("withheld count = %v, receipt = %v", got, p)
	}
	rows, _ := p["rules"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["rule"] != "Always sort outputs." {
		t.Errorf("rules = %v", rows)
	}
}
