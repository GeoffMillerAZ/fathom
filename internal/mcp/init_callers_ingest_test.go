package mcp_test

// init_callers_ingest_test.go — O1124, end to end through a real ingest
// (the typed pass included): a call made inside a package-level
// function-literal variable is the variable's, never every init() of the
// package; and an init() that references a function only as a value is
// that one init(), never its siblings that share the "<pkg>.init" key.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func initCallersServer(t *testing.T) *mcp.Server {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module initfix\n\ngo 1.22\n",
		"p/target.go": "package p\n\n" +
			"// Target is called only inside seam's func literal.\n" +
			"func Target() int { return 1 }\n\n" +
			"// Registered is referenced only as a value, from a.go's init.\n" +
			"func Registered() int { return 2 }\n",
		"p/a.go": "package p\n\n" +
			"var handlers []func() int\n\n" +
			"func init() {\n\thandlers = append(handlers, Registered)\n}\n",
		"p/b.go": "package p\n\n" +
			"var countB int\n\n" +
			"func init() {\n\tcountB = 3\n}\n",
		"p/seam.go": "package p\n\n" +
			"// seam is a test seam: a func literal kept in a var.\n" +
			"var seam = func() int { return Target() }\n\n" +
			"// Use calls through the seam.\n" +
			"func Use() int { return seam() }\n",
	}
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "initcallers.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(t.Context(), client.ServiceRef{ID: "initfix", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	return mcp.NewServer(c)
}

// callerLocs is find_callers' direct rows as "file:line name", sorted.
func callerLocs(t *testing.T, got map[string]any) []string {
	t.Helper()
	rows, ok := got["callers"].([]any)
	if !ok {
		t.Fatalf("callers missing or wrong type: %v", got)
	}
	var out []string
	for _, r := range rows {
		out = append(out, r.(map[string]any)["loc"].(string))
	}
	sort.Strings(out)
	return out
}

func TestFindCallers_VarInitializerIsTheVariableNotEveryInit(t *testing.T) {
	srv := initCallersServer(t)

	got := callTool(t, srv, "find_callers", map[string]any{"service": "initfix", "symbol": "Target", "depth": 1})
	if locs := callerLocs(t, got); len(locs) != 1 || locs[0] != "p/seam.go:4 seam" {
		t.Errorf("find_callers(Target) = %v, want exactly [p/seam.go:4 seam]: the call sits in seam's initializer, not in any init()", locs)
	}

	got = callTool(t, srv, "find_callers", map[string]any{"service": "initfix", "symbol": "Registered", "depth": 1})
	if locs := callerLocs(t, got); len(locs) != 1 || locs[0] != "p/a.go:5 init" {
		t.Errorf("find_callers(Registered) = %v, want exactly [p/a.go:5 init]: b.go's init shares the key, not the reference", locs)
	}

	sc := callTool(t, srv, "symbol_context", map[string]any{"service": "initfix", "symbol": "Target", "snippet": false})
	callers, _ := sc["callers"].([]any)
	var keys []string
	for _, c := range callers {
		keys = append(keys, c.(string))
	}
	if len(keys) != 1 || !strings.HasSuffix(keys[0], "p.seam") {
		t.Errorf("symbol_context(Target).callers = %v, want the one key of seam, as find_callers names it", keys)
	}
}
