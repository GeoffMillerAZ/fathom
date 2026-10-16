// surface_glance_doc_test.go: docs/mcp-tools.md opens with a table of the
// three surfaces, their tool counts and the tools each one adds. A reader
// takes those numbers as fact, so this test holds every row to the code:
// the default and extended allowlists (internal/mcp/surface.go) and the
// full registry tools/list serves.
package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// glanceRow is one row of the at-a-glance table: `| surface | N | tools |`.
var glanceRow = regexp.MustCompile("(?m)^\\| `(/rpc[a-z/]*)`[^|]*\\| (\\d+) \\| (.*) \\|$")

// glanceTool is a backticked tool name in a row's last cell.
var glanceTool = regexp.MustCompile("`([a-z][a-z0-9_]*)`")

func TestMcpToolsDocSurfaceGlanceMatchesRegistry(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "mcp-tools.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	start := strings.Index(doc, "## The surfaces at a glance")
	if start < 0 {
		t.Fatal("docs/mcp-tools.md has no \"## The surfaces at a glance\" section")
	}
	end := strings.Index(doc[start+1:], "\n## ")
	section := doc[start:]
	if end >= 0 {
		section = doc[start : start+1+end]
	}

	srv, _ := newModuleTestServer()
	resp := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{},
	})
	want := map[string][]string{
		"/rpc/agent":    mcp.DefaultSurfaceTools(),
		"/rpc/extended": mcp.ExtendedSurfaceTools(),
		"/rpc":          toolNames(t, resp),
	}

	rows := glanceRow.FindAllStringSubmatch(section, -1)
	if len(rows) != len(want) {
		t.Fatalf("parsed %d surface rows, want %d (one per surface):\n%s", len(rows), len(want), section)
	}
	// Each row lists the tools it ADDS to the row above it ("the eight,
	// plus ..."), so the running union is what the surface serves.
	var union []string
	for _, row := range rows {
		path, countText, cell := row[1], row[2], row[3]
		names, ok := want[path]
		if !ok {
			t.Errorf("row names surface %s, which the table should not carry", path)
			continue
		}
		for _, m := range glanceTool.FindAllStringSubmatch(cell, -1) {
			union = append(union, m[1])
		}
		got := append([]string(nil), union...)
		sort.Strings(got)
		exp := append([]string(nil), names...)
		sort.Strings(exp)
		if strings.Join(got, ",") != strings.Join(exp, ",") {
			t.Errorf("%s: the table lists %v, the code serves %v", path, got, exp)
		}
		if n, _ := strconv.Atoi(countText); n != len(names) {
			t.Errorf("%s: the table says %s tools, the code serves %d", path, countText, len(names))
		}
	}
}
