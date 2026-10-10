package fusion

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_HTMLIDReferences (wave lf5, item html; design §1.8, W1 and its
// K19 pair key): an ingest stores each HTML document's module symbol with the
// same-document ID references it makes (a label's for, an ARIA set split on
// whitespace, a fragment href), whatever element encloses them, and the file
// resolver joins each to the element carrying the id in the same document,
// never another page's element of that id; a link to another page is a file
// edge to that page's module symbol.
func TestIngest_HTMLIDReferences(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"site/index.html": `<!DOCTYPE html>
<html>
<body>
  <main id="page">
    <label for="email">Email</label>
    <input id="email" aria-describedby="hint missing">
    <p id="hint">We never share it.</p>
    <a href="#page">Top</a> <a href="about.html">About</a>
  </main>
</body>
</html>
`,
		"site/about.html": `<!DOCTYPE html>
<html>
<body>
  <p id="email">a different element named email</p>
</body>
</html>
`,
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "web", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "web", "")
	if err != nil {
		t.Fatal(err)
	}
	var module *core.Observation
	for i, o := range obs {
		if strings.HasPrefix(o.SourceRef, "site/index.html") && o.Symbol == core.ModuleSymbolName {
			module = &obs[i]
		}
	}
	if module == nil {
		t.Fatalf("site/index.html stored no module symbol")
	}
	if string(module.SourceKind) != "HTMLDocument" {
		t.Errorf("module kind = %q, want HTMLDocument", module.SourceKind)
	}
	if want := []string{"email", "hint", "missing", "page"}; !reflect.DeepEqual(module.Calls, want) {
		t.Errorf("module calls = %q, want %q (every W1 reference, whatever element encloses it)", module.Calls, want)
	}
	all, err := st.TypedEdges(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	want := []string{
		"site/index.html|web::site/index.<module>->web::site/index.email",
		"site/index.html|web::site/index.<module>->web::site/index.hint",
		"site/index.html|web::site/index.<module>->web::site/index.page",
		"site/index.html|web::site/index.page->web::site/about.<module>",
	}
	if !reflect.DeepEqual(edges, want) {
		t.Fatalf("file-resolved edges\n got: %q\nwant: %q", edges, want)
	}
}
