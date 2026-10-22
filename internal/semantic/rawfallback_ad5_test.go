package semantic

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestAD5RawRowsInForce(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	for _, tt := range []struct {
		name, path, body string
		want             int
	}{
		{"accepted", "ADR.md", "## ADR-001: Current\n\n**Decision.** Always sort outputs.\n", 1},
		{"proposed", "ADR.md", "## ADR-001: Proposal\n\n**Status.** Proposed.\n\n**Decision.** Always sort outputs.\n", 0},
		{"rejected", "ADR.md", "## ADR-001: Rejected\n\n**Status.** Rejected.\n\n**Decision.** Always sort outputs.\n", 0},
		{"deferred", "ADR.md", "## ADR-001: Deferred\n\n**Status.** Deferred.\n\n**Decision.** Always sort outputs.\n", 0},
		{"superseded", "ADR.md", "## ADR-001: Old\n\n**Status.** Superseded by ADR-002.\n\n**Decision.** Always sort outputs.\n", 0},
		{"partly superseded", "ADR.md", "## ADR-001: Partial\n\n**Status.** Superseded in part by ADR-002.\n\n**Decision.** Always sort outputs.\n", 1},
		{"archive", "docs/plans/archive/old.md", "# Plan\n\n- [ ] Always sort outputs.\n", 0},
		{"checked plan", "docs/plans/current.md", "# Plan\n\n- [x] Always sort outputs.\n", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			corpus := []core.SourceFile{{Path: tt.path, Content: []byte(tt.body)}}
			rows, err := RawFallbackRules(nil, layer, corpus, "pkg/a.go", 2, "coverage gap", nil, nil)
			if err != nil || len(rows) != tt.want {
				t.Fatalf("rows = %+v, err = %v, want %d", rows, err, tt.want)
			}
			for _, row := range rows {
				if !strings.HasPrefix(row.Src, tt.path+"#L") {
					t.Errorf("citation = %q, want source document", row.Src)
				}
			}
		})
	}
}

func TestAD5RawInstructionExtraction(t *testing.T) {
	for _, tt := range []struct {
		name, path, body, want string
	}{
		{"imperatives", "AGENTS.md", "# Rules\n- You **must** sort `outputs`.\n- Never use [randomness](https://example.invalid).\n- Always inject clocks.\n", "You must sort outputs. Never use randomness."},
		{"only", "AGENTS.md", "# Rules\nOnly serve current rules.\n", "Only serve current rules."},
		{"do not", "AGENTS.md", "# Rules\nDo not fetch dependencies.\n", "Do not fetch dependencies."},
		{"don't", "AGENTS.md", "# Rules\nDon't fetch dependencies.\n", "Don't fetch dependencies."},
		{"required", "AGENTS.md", "# Rules\nStable ordering is required.\n", "Stable ordering is required."},
		{"decision", "ADR.md", "## ADR-001: Storage\n**Decision.** Use the existing store.\n**Decision:** Keep one writer.\n**Decision** Retain portable files.\n", "Use the existing store. Keep one writer."},
		{"heading only", "AGENTS.md", "# Always sort outputs\n", ""},
		{"no instruction", "AGENTS.md", "# Rules\nSome background about the project.\n", ""},
		{"code is not instruction", "AGENTS.md", "# Rules\n```text\nAlways do the wrong thing.\n```\nNever execute examples.\n", "Never execute examples."},
		{"character cap", "AGENTS.md", "# Rules\nMust " + strings.Repeat("x", 300) + "\n", "Must " + strings.Repeat("x", 232) + "..."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			secs := SplitMarkdownSections(tt.path, []byte(tt.body))
			if got := rawRuleText(secs[0]); got != tt.want {
				t.Errorf("instruction = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAD5RawRowsWithholdUnextractableSections(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	corpus := []core.SourceFile{{Path: "AGENTS.md", Content: []byte("# Heading only\n\n# Current instructions\n\nAlways sort outputs.\n")}}
	for _, root := range []bool{false, true} {
		var rows []Obligation
		var err error
		if root {
			rows, err = RootRawFallbackRules(nil, layer, corpus, 1, "coverage gap")
		} else {
			rows, err = RawFallbackRules(nil, layer, corpus, "pkg/a.go", 1, "coverage gap", nil, nil)
		}
		if err != nil || len(rows) != 1 || rows[0].Rule != "Always sort outputs." {
			t.Errorf("root=%v: rows = %+v, err = %v, want an instruction after withholding the heading", root, rows, err)
		}
	}
}
