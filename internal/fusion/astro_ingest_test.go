package fusion

import (
	"context"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// astroCorpus is a small Astro project: a page that imports a component, a
// TypeScript helper beside the component under the same stem, and a library
// module the page's frontmatter and its processed <script> both call.
var astroCorpus = map[string]string{
	"src/components/Card.astro": `---
interface Props {
  title: string;
}
const { title } = Astro.props;
---
<article><h2>{title}</h2><slot /></article>
`,
	"src/components/Card.ts": `export function cardTitle(s: string): string {
  return s.trim();
}
`,
	"src/lib/format.ts": `export function format(n: number): string {
  return String(n);
}
`,
	"src/pages/index.astro": `---
import Card from '../components/Card.astro';
import { format } from '../lib/format';
const n = format(3);
---
<Card title={n} />
<script>
  import { format } from '../lib/format';
  format(1);
</script>
`,
}

// TestIngest_AstroDialect (LB-F544, LB-F545; design 1.8's Astro paragraph):
// ingest reads .astro with the astro dialect. A component is its file's module
// symbol, keyed by the base name with its extension (F1), so Card.astro and the
// Card.ts beside it never share a key; the frontmatter's TypeScript definitions
// are the file's (F2); and the page reaches the component it imports and
// renders: the file resolver runs for the host (its ranges' imports are its
// own), and the frontmatter's import of Card.astro is a file-resolved edge
// from the page's module symbol to the component's (K18's module-import edge).
func TestIngest_AstroDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, astroCorpus)
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "astrox", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "astrox", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	var have []string
	for _, o := range obs {
		k := strings.SplitN(o.SourceRef, ":", 2)[0] + "|" + string(o.SourceKind) + "|" + o.Package + "|" + o.Symbol
		got[k] = true
		have = append(have, k)
	}
	sort.Strings(have)
	for _, want := range []string{
		"src/components/Card.astro|AstroComponent|Card.astro|" + core.ModuleSymbolName,
		"src/components/Card.astro|TSInterface|Card.astro|Props",
		"src/components/Card.astro|JSConst|Card.astro|title",
		"src/components/Card.ts|JSFunc|Card|cardTitle",
		"src/pages/index.astro|AstroComponent|index.astro|" + core.ModuleSymbolName,
		"src/pages/index.astro|JSConst|index.astro|n",
	} {
		if !got[want] {
			t.Errorf("%s not stored; have %q", want, have)
		}
	}
	edges, err := st.TypedEdges(ctx, "astrox")
	if err != nil {
		t.Fatal(err)
	}
	var fromPage []string
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved && e.FromFile == "src/pages/index.astro" {
			fromPage = append(fromPage, e.FromSymbol+" -> "+e.ToSymbol)
		}
	}
	sort.Strings(fromPage)
	want := "astrox::src/pages/index.astro." + core.ModuleSymbolName + " -> astrox::src/components/Card.astro." + core.ModuleSymbolName // <Card /> renders the component
	if !slices.Contains(fromPage, want) {
		t.Errorf("no file-resolved edge %s; edges from the page: %q", want, fromPage)
	}
}
