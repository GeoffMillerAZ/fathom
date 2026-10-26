package fusion_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_JinjaExplicitExtensionsDispatchAndResolve(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, ext := range []string{"j2", "jinja", "jinja2"} {
		write("templates/page."+ext, "{% macro card() %}ok{% endmacro %}\n{% include \"partials/local.j2\" %}\n")
	}
	write("templates/partials/local.j2", "{% macro local() %}local{% endmacro %}\n")
	write("plain.html", "<html><body><p>plain</p></body></html>\n")

	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(p.Close)
	ctx := context.Background()
	const service = "jinja-dispatch"
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: service, Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, service, "")
	if err != nil {
		t.Fatal(err)
	}
	macros := map[string]bool{}
	plainHTML := false
	for _, o := range obs {
		switch {
		case o.SourceKind == "JinjaMacro" && o.Symbol == "card":
			macros[strings.SplitN(o.SourceRef, ":", 2)[0]] = true
		case strings.HasPrefix(o.SourceRef, "plain.html") && o.SourceKind == "HTMLDocument":
			plainHTML = true
		}
	}
	for _, ext := range []string{"j2", "jinja", "jinja2"} {
		if !macros["templates/page."+ext] {
			t.Errorf("%s produced no JinjaMacro card; macros=%v", ext, macros)
		}
	}
	if !plainHTML {
		t.Error("plain.html did not remain an HTMLDocument")
	}
	edges, err := st.TypedEdges(ctx, service)
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"j2", "jinja", "jinja2"} {
		from := "templates/page." + ext
		resolved := false
		for _, e := range edges {
			if e.Kind == core.EdgeFileResolved && e.FromFile == from && strings.HasSuffix(e.ToSymbol, "templates/partials/local.<module>") {
				resolved = true
			}
		}
		if !resolved {
			t.Errorf("%s include has no file_resolved edge to local.j2; edges=%+v", from, edges)
		}
	}
}

func TestIngest_JinjaImportAliasesResolveQualifiedCalls(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("templates/page.j2", `{% import "forms.j2" as forms %}
{% import "other.j2" as other %}
{{ forms.input() }} {{ other.input() }} {{ input() }}
`)
	write("templates/bare.j2", `{% import "forms.j2" as forms %}
{% import "other.j2" as other %}
{{ input() }}
`)
	write("templates/forms.j2", `{% macro input() %}form{% endmacro %}
`)
	write("templates/other.j2", `{% macro input() %}other{% endmacro %}
`)

	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(p.Close)
	ctx := context.Background()
	const service = "jinja-aliases"
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: service, Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, service, "")
	if err != nil {
		t.Fatal(err)
	}
	var pageCalls []string
	for _, o := range obs {
		if strings.HasPrefix(o.SourceRef, "templates/page.j2") {
			pageCalls = append(pageCalls, o.Calls...)
		}
	}
	for _, want := range []string{"forms.input", "other.input", "input"} {
		found := false
		for _, got := range pageCalls {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("page stored calls %q; missing %q", pageCalls, want)
		}
	}
	edges, err := st.TypedEdges(ctx, service)
	if err != nil {
		t.Fatal(err)
	}
	wantTargets := map[string]string{
		"forms.input": "templates/forms.input",
		"other.input": "templates/other.input",
	}
	seenTargets := map[string]bool{}
	for _, e := range edges {
		if e.Kind != core.EdgeFileResolved || e.FromFile != "templates/page.j2" {
			continue
		}
		for call, target := range wantTargets {
			if strings.HasSuffix(e.ToSymbol, target) {
				seenTargets[call] = true
			}
		}
		if strings.HasSuffix(e.ToSymbol, "templates/forms.input") || strings.HasSuffix(e.ToSymbol, "templates/other.input") {
			if e.FromSymbol == "input" {
				t.Errorf("bare input() resolved to imported macro: %+v", e)
			}
		}
	}
	for call := range wantTargets {
		if !seenTargets[call] {
			t.Errorf("no file_resolved call edge for %s; edges=%+v", call, edges)
		}
	}
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved && e.FromFile == "templates/bare.j2" &&
			(strings.HasSuffix(e.ToSymbol, "templates/forms.input") || strings.HasSuffix(e.ToSymbol, "templates/other.input")) {
			t.Errorf("bare input() resolved to an imported macro: %+v", e)
		}
	}
}
