package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestEveryToolDeclaresCostClass (RT-55, design §3.1): every registered
// tool carries a cost class, registration without one panics, and
// docs/mcp-tools.md's cost-class table matches the registry (I11).
func TestEveryToolDeclaresCostClass(t *testing.T) {
	srv, _ := newModuleTestServer()
	classes := srv.ToolCostClassesForTest()
	if len(classes) < 58 {
		t.Fatalf("only %d tools registered — the registry did not build", len(classes))
	}
	valid := map[string]bool{"point": true, "index": true, "tree": true, "fleet": true, "ingest": true, "egress": true}
	for tool, c := range classes {
		if !valid[c] {
			t.Errorf("%s: cost class %q", tool, c)
		}
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Error("registering a tool with no cost class did not fail")
			}
		}()
		srv.RegisterUndeclaredToolForTest("zz_undeclared_tool")
	}()

	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "mcp-tools.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	start := strings.Index(doc, "## Cost classes")
	end := strings.Index(doc, "## Shared tools")
	if start < 0 || end <= start {
		t.Fatal(`docs/mcp-tools.md has no "## Cost classes" section before "## Shared tools"`)
	}
	row := regexp.MustCompile("(?m)^\\|\\s*`([a-z_]+)`\\s*\\|\\s*`([a-z]+)`\\s*\\|")
	documented := map[string]string{}
	for _, m := range row.FindAllStringSubmatch(doc[start:end], -1) {
		documented[m[1]] = m[2]
	}
	var drift []string
	for tool, c := range classes {
		if documented[tool] != c {
			drift = append(drift, tool+": registry "+c+", doc "+documented[tool])
		}
	}
	for tool := range documented {
		if _, ok := classes[tool]; !ok {
			drift = append(drift, tool+": documented, not registered")
		}
	}
	sort.Strings(drift)
	if len(drift) > 0 {
		t.Fatalf("docs/mcp-tools.md cost classes drifted from the registry:\n%s", strings.Join(drift, "\n"))
	}
}

// TestCostClassMatchesMeasuredCost (RT-152, design §3.1): a tool's
// declared class is at least the class of its measured cost in the
// committed cost table (internal/mcp/testdata/toolcost.json), and the
// table covers exactly the registry. A class moves down only in the commit
// that rewrites the tool, which rewrites its row and carries its cost
// assertion.
func TestCostClassMatchesMeasuredCost(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "toolcost.json"))
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Tools map[string]struct {
			Measured string `json:"measured"`
			Basis    string `json:"basis"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	rank := map[string]int{"point": 0, "index": 1, "tree": 2, "fleet": 3}
	srv, _ := newModuleTestServer()
	declared := srv.ToolCostClassesForTest()
	var bad []string
	for tool, c := range declared {
		row, ok := table.Tools[tool]
		if !ok {
			bad = append(bad, tool+": no row in the cost table")
			continue
		}
		if row.Basis == "" {
			bad = append(bad, tool+": row has no basis")
		}
		dr, dok := rank[c]
		mr, mok := rank[row.Measured]
		switch {
		case !dok || !mok:
			if c != row.Measured {
				bad = append(bad, tool+": declared "+c+", measured "+row.Measured+" (ingest and egress match exactly)")
			}
		case dr < mr:
			bad = append(bad, tool+": declared "+c+" is cheaper than its measured "+row.Measured)
		}
	}
	for tool := range table.Tools {
		if _, ok := declared[tool]; !ok {
			bad = append(bad, tool+": cost row for an unregistered tool")
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("cost classes vs the committed cost table:\n%s", strings.Join(bad, "\n"))
	}

	// §3.1's argument-dependent class: references over every service is fleet.
	if got := srv.CostClassOfForTest("references", `{"query":"x","services":["*"]}`); got != "fleet" {
		t.Fatalf(`references services ["*"] = %s, want fleet`, got)
	}
	if got := srv.CostClassOfForTest("references", `{"query":"x"}`); got != "tree" {
		t.Fatalf("references (one service) = %s, want tree", got)
	}
	// An explicit list naming several services does the same fleet work as
	// ["*"] (each one's symbols, typed edges and text): it takes the fleet
	// slot too; one named service (even repeated) stays tree.
	if got := srv.CostClassOfForTest("references", `{"query":"x","services":["a","b"]}`); got != "fleet" {
		t.Fatalf(`references services ["a","b"] = %s, want fleet`, got)
	}
	if got := srv.CostClassOfForTest("references", `{"query":"x","services":["a","a"]}`); got != "tree" {
		t.Fatalf(`references services ["a","a"] = %s, want tree`, got)
	}
}
