package fusion

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_CSSNameReferences (LB-F459, design 1.8 W2): an ingest stores a
// stylesheet's var() reads and animation names as its rules' references, and
// a reference to a custom property or an @keyframes rule another stylesheet
// declares, through an @import, is a file-resolved edge to that definition.
// Each stylesheet is a module (K18), so the @import itself is an edge from the
// importing sheet's module symbol to the imported sheet's, and a reference no
// rule owns (an @font-face descriptor) is its module's.
func TestIngest_CSSNameReferences(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"css/base.css": `:root {
  --brand: #336699;
  --gap: var(--space);
}

@keyframes pulse {
  from { opacity: var(--dim); }
}
`,
		"css/main.css": `@import "base.css";

.btn, .link {
  color: var(--brand);
  animation: pulse 1s ease-in infinite;
}

@font-face {
  font-display: var(--display);
}
`,
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cssx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cssx", "")
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]string{}
	for _, o := range obs {
		file, _, _ := strings.Cut(o.SourceRef, ":")
		calls[file+"|"+o.Symbol] = strings.Join(o.Calls, ",")
	}
	for key, want := range map[string]string{
		"css/main.css|.btn":     "--brand,pulse",
		"css/main.css|.link":    "",
		"css/base.css|--gap":    "--space",
		"css/base.css|pulse":    "--dim",
		"css/base.css|:root":    "",
		"css/base.css|--brand":  "",
		"css/main.css|<module>": "--display",
	} {
		got, ok := calls[key]
		if !ok {
			var have []string
			for k := range calls {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", key, have)
			continue
		}
		if got != want {
			t.Errorf("%s calls = %q, want %q", key, got, want)
		}
	}
	edges, err := st.TypedEdges(ctx, "cssx")
	if err != nil {
		t.Fatal(err)
	}
	var resolved []string
	moduleEdge := false
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved && e.FromFile == "css/main.css" {
			resolved = append(resolved, e.ToSymbol)
			if strings.HasSuffix(e.FromSymbol, core.ModuleSymbolName) && strings.HasSuffix(e.ToSymbol, "base."+core.ModuleSymbolName) {
				moduleEdge = true
			}
		}
	}
	sort.Strings(resolved)
	if !moduleEdge {
		t.Errorf("no file-resolved edge from the module of css/main.css to the module of css/base.css (K18: an @import is an edge between the sheets); edges from it: %q", resolved)
	}
	for _, want := range []string{"--brand", "pulse"} {
		found := false
		for _, to := range resolved {
			if strings.HasSuffix(to, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no file-resolved edge from css/main.css to %s (its @import of base.css); file-resolved edges from it: %q", want, resolved)
		}
	}
}
