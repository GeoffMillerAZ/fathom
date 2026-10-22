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
