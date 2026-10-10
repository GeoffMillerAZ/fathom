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

// TestIngest_SCSSDialect (LB-F465): ingest reads .scss with the scss tags
// dialect: mixins, functions, variables, class names and placeholders are
// stored without their sigils, an @include is a call, and each @use, @forward
// and @import resolves by Sass's partial rule to a file edge between module
// symbols; a legacy @import also binds the partial's names, so the @include
// of a mixin it declares is a file-resolved edge to that mixin.
func TestIngest_SCSSDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"scss/_variables.scss": "$brand: #336699 !default;\n",
		"scss/mixins/_index.scss": `@mixin button-variant($bg) {
  background: $bg;
}
`,
		"scss/main.scss": `@use "variables" as v;
@import "mixins";

%reset {
  margin: 0;
}

.btn {
  @include button-variant(v.$brand);
}
`,
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "scssx", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "scssx", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[string(o.SourceKind)+"|"+o.Symbol] = o
	}
	for _, want := range []string{"SCSSVariable|brand", "SCSSMixin|button-variant", "SCSSPlaceholder|reset", "SCSSClass|btn", "SCSSModule|<module>"} {
		if _, ok := got[want]; !ok {
			var have []string
			for k := range got {
				have = append(have, k)
			}
			sort.Strings(have)
			t.Errorf("%s not stored; have %q", want, have)
		}
	}
	edges, err := st.TypedEdges(ctx, "scssx")
	if err != nil {
		t.Fatal(err)
	}
	var fromMain []string
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved && e.FromFile == "scss/main.scss" {
			fromMain = append(fromMain, e.ToSymbol)
		}
	}
	sort.Strings(fromMain)
	for _, want := range []string{"_variables." + core.ModuleSymbolName, "_index." + core.ModuleSymbolName, "button-variant"} {
		found := false
		for _, to := range fromMain {
			if strings.HasSuffix(to, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no file-resolved edge from scss/main.scss to %s; edges from it: %q", want, fromMain)
		}
	}
}
